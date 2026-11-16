package generator

import (
	"os"
	"path/filepath"
	"testing"

	"go.yaml.in/yaml/v4"
)

func TestGoldenOutputFromListBatch(t *testing.T) {
	input, loadDir := syntheticInput(t)
	input.OpenAPI = []byte(mustRead(t, filepath.Join("testdata", "memberthing.yaml")))
	out := filepath.Join(t.TempDir(), "memberthing")
	options := Options{Resource: "Member", OutputDir: out, OverridesPath: filepath.Join("testdata", "memberthing-overrides.yaml")}
	if err := generateFromInput(options, input, loadDir); err != nil {
		t.Fatal(err)
	}
	golden := filepath.Join("testdata", "golden", "memberthing")
	if *updateGolden {
		if err := os.RemoveAll(golden); err != nil {
			t.Fatal(err)
		}
		if err := copyDirectory(out, golden); err != nil {
			t.Fatal(err)
		}
	}
	if diff := compareDirectories(golden, out); diff != "" {
		t.Fatalf("memberthing golden differs; run with -update:\n%s", diff)
	}
	compileGenerated(t, out, input)
}

func TestFromListPagingRuntime(t *testing.T) {
	input, loadDir := syntheticInput(t)
	input.OpenAPI = []byte(mustRead(t, filepath.Join("testdata", "memberthing.yaml")))
	out := filepath.Join(t.TempDir(), "memberthing")
	options := Options{Resource: "Member", OutputDir: out, OverridesPath: filepath.Join("testdata", "memberthing-overrides.yaml")}
	if err := generateFromInput(options, input, loadDir); err != nil {
		t.Fatal(err)
	}
	const semantics = `package memberthing
import (
 "context"
 "strings"
 "testing"
 "example.com/iac-test-sdk/go/openapi/gen/member_service"
)
func ptr(s string) *string { return &s }
func TestReadPagesUntilTheItemIsFound(t *testing.T) {
 var token string = "page-2"
 client := &member_service.MemberServiceAPIService{
  SearchResponses: []*member_service.SearchMembersResponse{
   {NextPageToken:&token, Members: []member_service.Member{{MemberId:ptr("first")}}},
   {Members: []member_service.Member{{MemberId:ptr("third")}}},
  },
 }
 r := &Resource{client:client}
 got, err := r.get(context.Background(), "third")
 if err != nil || got == nil || *got.MemberId != "third" { t.Fatalf("paged read = %v, %v", got, err) }
 if client.SearchPageCalls != 2 { t.Fatalf("page calls = %d", client.SearchPageCalls) }
 if len(client.SearchPageTokens) != 1 || client.SearchPageTokens[0] != token {
  t.Fatalf("page tokens = %v", client.SearchPageTokens)
 }
}
func TestReadOfAnAbsentMemberReturnsNil(t *testing.T) {
 client := &member_service.MemberServiceAPIService{
  SearchResponses: []*member_service.SearchMembersResponse{{Members: []member_service.Member{{MemberId:ptr("first")}}}},
 }
 r := &Resource{client:client}
 got, err := r.get(context.Background(), "absent")
 if err != nil || got != nil { t.Fatalf("absent read = %v, %v", got, err) }
}
func TestDeleteSendsTheIdsAndTheStatus(t *testing.T) {
 client := &member_service.MemberServiceAPIService{}
 r := &Resource{client:client}
 if err := r.delete(context.Background(), "gone-id"); err != nil { t.Fatal(err) }
 if client.DeleteBody == nil { t.Fatal("delete sent no body") }
 if client.DeleteBody.Status == nil || *client.DeleteBody.Status != member_service.MEMBERSTATUS_MEMBER_STATUS_INACTIVE {
  t.Fatalf("delete status = %v", client.DeleteBody.Status)
 }
 if len(client.DeleteBody.MemberIds) != 1 || client.DeleteBody.MemberIds[0] != "gone-id" {
  t.Fatalf("delete ids = %v", client.DeleteBody.MemberIds)
 }
}
func TestCreateReadsTheFirstResultAndReportsTheMessage(t *testing.T) {
 created := member_service.CREATEMEMBERSTATUS_CREATE_MEMBER_STATUS_CREATED
 client := &member_service.MemberServiceAPIService{
  CreateResults: &member_service.CreateMembersResponse{Results: []member_service.CreateMemberResult{{
   Status: &created, Member: &member_service.Member{Login: ptr("a@b.c")},
  }}},
 }
 r := &Resource{client:client}
 got, err := r.create(context.Background(), &member_service.CreateMemberRequest{})
 if err != nil || got == nil || *got.Login != "a@b.c" { t.Fatalf("create = %v, %v", got, err) }
 failed := member_service.CREATEMEMBERSTATUS_CREATE_MEMBER_STATUS_FAILED
 message := "already exists"
 client.CreateResults = &member_service.CreateMembersResponse{Results: []member_service.CreateMemberResult{{Status:&failed, Message:&message}}}
 _, err = r.create(context.Background(), &member_service.CreateMemberRequest{})
 if err == nil || !strings.Contains(err.Error(), message) { t.Fatalf("failed create error = %v", err) }
}
`
	if err := os.WriteFile(filepath.Join(out, "semantics_test.go"), []byte(semantics), 0o644); err != nil {
		t.Fatal(err)
	}
	compileGenerated(t, out, input)
}

func TestMemberThingNegativeKeys(t *testing.T) {
	input, _ := syntheticInput(t)
	var doc map[string]any
	data := mustRead(t, filepath.Join("testdata", "memberthing.yaml"))
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	// A List whose GET lost the page-token query parameter must be refused: the model cannot
	// read the next page.
	paths := doc["paths"].(map[string]any)
	get := paths["/members/v1"].(map[string]any)["get"].(map[string]any)
	params := get["parameters"].([]any)
	kept := params[:0]
	for _, p := range params {
		if p.(map[string]any)["name"] != "page_token" {
			kept = append(kept, p)
		}
	}
	get["parameters"] = kept
	spec, err := yaml.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	file, err := readOverrides(filepath.Join("testdata", "memberthing-overrides.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := validateOpenAPIWith(spec, "Member", Options{}.OperationIDs, input.SDKModule, input.Module, file); err == nil {
		t.Fatal("a paged List without a page-token parameter must be refused")
	}
	// A delete without its status value must be refused: the model requires the fixed values
	// beside the ids.
	data = mustRead(t, filepath.Join("testdata", "memberthing.yaml"))
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	file, err = readOverrides(filepath.Join("testdata", "memberthing-overrides.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := validateOpenAPIWith(data, "Member", Options{}.OperationIDs, input.SDKModule, input.Module, file); err != nil {
		t.Fatalf("the memberthing contract must be eligible: %v", err)
	}
}
