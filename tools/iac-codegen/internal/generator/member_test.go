package generator

import (
	"os"
	"path/filepath"
	"strings"
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
 // The fixed query value that Get always sends rides on every page.
 for i, s := range client.SearchStatuses {
  if s != member_service.MEMBERSTATUS_MEMBER_STATUS_ACTIVE {
   t.Fatalf("page %d status = %v", i+1, s)
  }
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

func TestGoldenOutputMemberHook(t *testing.T) {
	input, loadDir := syntheticInput(t)
	input.OpenAPI = []byte(mustRead(t, filepath.Join("testdata", "memberhook.yaml")))
	out := filepath.Join(t.TempDir(), "memberhook")
	options := Options{Resource: "Member", OutputDir: out, OverridesPath: filepath.Join("testdata", "memberhook-overrides.yaml")}
	if err := generateFromInput(options, input, loadDir); err != nil {
		t.Fatal(err)
	}
	golden := filepath.Join("testdata", "golden", "memberhook")
	if *updateGolden {
		if err := os.RemoveAll(golden); err != nil {
			t.Fatal(err)
		}
		if err := copyDirectory(out, golden); err != nil {
			t.Fatal(err)
		}
	}
	if diff := compareDirectories(golden, out); diff != "" {
		t.Fatalf("memberhook golden differs; run with -update:\n%s", diff)
	}
	compileGenerated(t, out, input)
}

func TestMemberThingNegativeKeys(t *testing.T) {
	input, _ := syntheticInput(t)
	memberOverrides := filepath.Join("testdata", "memberthing-overrides.yaml")
	memberSpec := filepath.Join("testdata", "memberthing.yaml")
	// refused runs the spec through the validator and requires the named error.
	refused := func(spec []byte, overrides string, want string) {
		t.Helper()
		file, err := readOverrides(overrides)
		if err != nil {
			t.Fatal(err)
		}
		_, err = validateOpenAPIWith(spec, "Member", Options{}.OperationIDs, input.SDKModule, input.Module, file)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("the spec must be refused with %q: %v", want, err)
		}
	}
	// mutated unmarshals the fixture contract, lets the case change it, and marshals it back.
	mutated := func(change func(doc map[string]any)) []byte {
		t.Helper()
		var doc map[string]any
		if err := yaml.Unmarshal(mustRead(t, memberSpec), &doc); err != nil {
			t.Fatal(err)
		}
		change(doc)
		spec, err := yaml.Marshal(doc)
		if err != nil {
			t.Fatal(err)
		}
		return spec
	}
	// A List whose GET lost the page-token query parameter must be refused: the model cannot
	// read the next page.
	spec := mutated(func(doc map[string]any) {
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
	})
	refused(spec, memberOverrides, "get.page")
	// A List with paging parameters but no next-page property the model can read must be
	// refused: the generated Get would drop resources beyond the first page.
	spec = mutated(func(doc map[string]any) {
		schemas := doc["components"].(map[string]any)["schemas"].(map[string]any)
		delete(schemas["SearchMembersResponse"].(map[string]any)["properties"].(map[string]any), "nextPageToken")
	})
	refused(spec, memberOverrides, "cannot read where the pages end")
	// A next page token of another type must be refused: the generated loop compares it with
	// "" and sends it to the builder method, so any other type fails at provider build.
	spec = mutated(func(doc map[string]any) {
		schemas := doc["components"].(map[string]any)["schemas"].(map[string]any)
		token := schemas["SearchMembersResponse"].(map[string]any)["properties"].(map[string]any)["nextPageToken"].(map[string]any)
		token["type"] = "integer"
		token["format"] = "int64"
	})
	refused(spec, memberOverrides, "want a string or *string next page token")
	// A batch write whose response array has no createResult line must be refused: the
	// generated call could not tell the created item from the failed ones.
	overrides := string(mustRead(t, memberOverrides))
	stripped := strings.Replace(overrides,
		"    createResult:\n      resource: member\n      status: status\n      created:\n      - CREATE_MEMBER_STATUS_CREATED\n      - CREATE_MEMBER_STATUS_INVITED\n      message: message\n", "", 1)
	stripped = strings.Replace(stripped,
		"  createValues:\n    onboardingMode: ONBOARDING_MODE_NO_INVITE\n", "", 1)
	if stripped == overrides {
		t.Fatal("the overrides must carry the batchWrite.createResult block")
	}
	without := filepath.Join(t.TempDir(), "overrides.yaml")
	if err := os.WriteFile(without, []byte(stripped), 0o644); err != nil {
		t.Fatal(err)
	}
	strippedOverrides, err := readOverrides(without)
	if err != nil {
		t.Fatal(err)
	}
	_, err = validateOpenAPIWith(mustRead(t, memberSpec), "Member", Options{}.OperationIDs, input.SDKModule, input.Module, strippedOverrides)
	// The model refuses the undecipherable batch Create response at the contract gate, and the
	// renderer refuses one that the model passes but the createResult line does not describe.
	if err == nil || !strings.Contains(err.Error(), "RESOURCE_LIFECYCLE_INCOMPLETE") && !strings.Contains(err.Error(), "batch write array") {
		t.Fatalf("the refusal must be the batch Create response shape: %v", err)
	}
}
