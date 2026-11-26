package generator

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGoldenOutputEchoResultJoin(t *testing.T) {
	input, loadDir := syntheticInput(t)
	input.OpenAPI = []byte(mustRead(t, filepath.Join("testdata", "echoresult.yaml")))
	out := filepath.Join(t.TempDir(), "echoresult")
	options := Options{Resource: "Member", OutputDir: out, OverridesPath: filepath.Join("testdata", "echoresult-overrides.yaml")}
	if err := generateFromInput(options, input, loadDir); err != nil {
		t.Fatal(err)
	}
	golden := filepath.Join("testdata", "golden", "echoresult")
	if *updateGolden {
		if err := os.RemoveAll(golden); err != nil {
			t.Fatal(err)
		}
		if err := copyDirectory(out, golden); err != nil {
			t.Fatal(err)
		}
	}
	if diff := compareDirectories(golden, out); diff != "" {
		t.Fatalf("echoresult golden differs; run with -update:\n%s", diff)
	}
	const semantics = `package echoresult
import (
 "context"
 "strings"
 "testing"

 "example.com/iac-test-sdk/go/openapi/gen/member_service"
)
func ptr(s string) *string { return &s }
func TestCreateJoinsTheResultOnTheEcho(t *testing.T) {
 created := member_service.CREATEMEMBERSTATUS_CREATE_MEMBER_STATUS_CREATED
 // The first result echoes nothing: the join skips it without a panic and the
 // second result belongs to the request.
 client := &member_service.MemberServiceAPIService{
  CreateResults: &member_service.CreateMembersResponse{Results: []member_service.CreateMemberResult{
   {Status: &created},
   {Status: &created, Username: ptr("a@b.c"), Member: &member_service.Member{Login: ptr("a@b.c"), Username: ptr("a@b.c")}},
  }},
 }
 r := &Resource{client: client}
 got, err := r.create(context.Background(), &member_service.CreateMemberRequest{Template: &member_service.MemberTemplate{Login: ptr("a@b.c"), Username: ptr("a@b.c")}})
 if err != nil || got == nil || got.Username == nil || *got.Username != "a@b.c" {
  t.Fatalf("create = %v, %v", got, err)
 }
 // No result echoes the request: the released refusal, no panic on the nil echo.
 client.CreateResults = &member_service.CreateMembersResponse{Results: []member_service.CreateMemberResult{{Status: &created}}}
 _, err = r.create(context.Background(), &member_service.CreateMemberRequest{Template: &member_service.MemberTemplate{Login: ptr("a@b.c"), Username: ptr("a@b.c")}})
 if err == nil || !strings.Contains(err.Error(), "create returned no result for \"a@b.c\"") {
  t.Fatalf("nil-echo create error = %v", err)
 }
}
`
	if err := os.WriteFile(filepath.Join(out, "semantics_test.go"), []byte(semantics), 0o644); err != nil {
		t.Fatal(err)
	}
	compileGenerated(t, out, input)
}
