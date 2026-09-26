// This file is handwritten. It runs the generated CRUD code of a full-replace
// resource (E11) against a fake HTTP server: PUT on the collection path, the
// id in the body, no update mask, and readOnly server fields that are never
// sent.

package fakerule

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/fakesdk/go/openapi/cxsdk"
)

const rulesPath = "/fake/rules/v1"

// fakeAPI answers every request with status and body, and records the
// requests as "METHOD path" and their bodies.
type fakeAPI struct {
	status   int
	body     string
	requests []string
	bodies   []string
}

func (f *fakeAPI) resource(t *testing.T) *Resource {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		f.requests = append(f.requests, r.Method+" "+r.URL.Path)
		f.bodies = append(f.bodies, string(b))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(f.status)
		_, _ = io.WriteString(w, f.body)
	}))
	t.Cleanup(srv.Close)
	r := &Resource{}
	var resp resource.ConfigureResponse
	r.Configure(context.Background(), resource.ConfigureRequest{ProviderData: cxsdk.NewClientSet(srv.URL)}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("configure: %v", resp.Diagnostics)
	}
	return r
}

func (f *fakeAPI) wantRequests(t *testing.T, want ...string) {
	t.Helper()
	if len(f.requests) != len(want) {
		t.Fatalf("requests = %v, want %v", f.requests, want)
	}
	for i := range want {
		if f.requests[i] != want[i] {
			t.Fatalf("requests = %v, want %v", f.requests, want)
		}
	}
}

// sent returns the body of request i as a JSON object.
func (f *fakeAPI) sent(t *testing.T, i int) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(f.bodies[i]), &m); err != nil {
		t.Fatalf("body %d: %v: %s", i, err, f.bodies[i])
	}
	return m
}

const stored = `{"rule":{"id":"r1","name":"errors","kind":"RULE_KIND_LOGS","description":"old","enabled":true,` +
	`"priority":3,"tags":["a","b"],"condition":{"query":"level:error","threshold":2},"createTime":"2026-09-26T10:00:00Z"}}`

func model() *FakeRuleModel {
	return &FakeRuleModel{
		Id:          types.StringValue("r1"),
		Name:        types.StringValue("errors"),
		Kind:        types.StringValue("RULE_KIND_LOGS"),
		Description: types.StringValue("old"),
		Enabled:     types.BoolValue(true),
		Priority:    types.Int32Value(3),
		Tags:        types.ListValueMust(types.StringType, []attr.Value{types.StringValue("a"), types.StringValue("b")}),
		Condition:   &RuleConditionModel{Query: types.StringValue("level:error"), Threshold: types.Float64Value(2)},
		CreateTime:  types.StringValue("2026-09-26T10:00:00Z"),
	}
}

func emptyState() tfsdk.State {
	return tfsdk.State{Schema: Schema(), Raw: tftypes.NewValue(Schema().Type().TerraformType(context.Background()), nil)}
}

func toState(t *testing.T, m *FakeRuleModel) tfsdk.State {
	t.Helper()
	s := emptyState()
	if diags := s.Set(context.Background(), m); diags.HasError() {
		t.Fatalf("set state: %v", diags)
	}
	return s
}

func toPlan(t *testing.T, m *FakeRuleModel) tfsdk.Plan {
	t.Helper()
	s := toState(t, m)
	return tfsdk.Plan(s)
}

func idOf(t *testing.T, s tfsdk.State) string {
	t.Helper()
	var id types.String
	if diags := s.GetAttribute(context.Background(), path.Root("id"), &id); diags.HasError() {
		t.Fatalf("id: %v", diags)
	}
	return id.ValueString()
}

// TestCreate checks that Create sends no id and no server field.
func TestCreate(t *testing.T) {
	api := &fakeAPI{status: 200, body: stored}
	r := api.resource(t)
	plan := model()
	plan.Id, plan.CreateTime = types.StringUnknown(), types.StringUnknown()
	resp := resource.CreateResponse{State: emptyState()}
	r.Create(context.Background(), resource.CreateRequest{Plan: toPlan(t, plan)}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("create: %v", resp.Diagnostics)
	}
	api.wantRequests(t, "POST "+rulesPath)
	body := api.sent(t, 0)
	for _, k := range []string{"id", "createTime"} {
		if _, ok := body[k]; ok {
			t.Errorf("Create body has %s: %s", k, api.bodies[0])
		}
	}
	if body["kind"] != "RULE_KIND_LOGS" {
		t.Errorf("Create body kind = %v, want RULE_KIND_LOGS", body["kind"])
	}
	if got := idOf(t, resp.State); got != "r1" {
		t.Errorf("id = %q, want r1", got)
	}
}

// TestUpdate checks the full replace: PUT on the collection path, the id from
// the state in the body, every Update field, also the unchanged ones, and no
// mask. The immutable kind and the readOnly createTime are not sent.
func TestUpdate(t *testing.T) {
	api := &fakeAPI{status: 200, body: stored}
	r := api.resource(t)
	plan := model()
	plan.Priority = types.Int32Value(5)
	plan.CreateTime = types.StringUnknown()
	resp := resource.UpdateResponse{State: toState(t, model())}
	r.Update(context.Background(), resource.UpdateRequest{Plan: toPlan(t, plan), State: toState(t, model())}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("update: %v", resp.Diagnostics)
	}
	api.wantRequests(t, "PUT "+rulesPath)
	body := api.sent(t, 0)
	if body["id"] != "r1" {
		t.Errorf("body id = %v, want r1 from the state", body["id"])
	}
	if body["priority"] != float64(5) {
		t.Errorf("body priority = %v, want 5", body["priority"])
	}
	for _, k := range []string{"name", "description", "enabled", "tags", "condition"} {
		if _, ok := body[k]; !ok {
			t.Errorf("body has no unchanged field %s: a full replace sends all of them: %s", k, api.bodies[0])
		}
	}
	for _, k := range []string{"updateMask", "kind", "createTime"} {
		if _, ok := body[k]; ok {
			t.Errorf("body has %s: %s", k, api.bodies[0])
		}
	}
}

// TestUpdateClear checks that a value that the plan removes is not in the
// body: the server clears it.
func TestUpdateClear(t *testing.T) {
	api := &fakeAPI{status: 200, body: stored}
	r := api.resource(t)
	plan := model()
	plan.Description = types.StringNull()
	plan.Tags = types.ListNull(types.StringType)
	plan.Condition.Threshold = types.Float64Null()
	resp := resource.UpdateResponse{State: toState(t, model())}
	r.Update(context.Background(), resource.UpdateRequest{Plan: toPlan(t, plan), State: toState(t, model())}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("update: %v", resp.Diagnostics)
	}
	body := api.sent(t, 0)
	for _, k := range []string{"description", "tags"} {
		if _, ok := body[k]; ok {
			t.Errorf("body has cleared field %s: %s", k, api.bodies[0])
		}
	}
	if cond, _ := body["condition"].(map[string]any); cond == nil || cond["threshold"] != nil || cond["query"] != "level:error" {
		t.Errorf("body condition = %v, want the query and no threshold", body["condition"])
	}
}

// TestUpdateNoChange checks that Update sends no PUT when no Update field
// changed, and reads the resource instead (D14).
func TestUpdateNoChange(t *testing.T) {
	api := &fakeAPI{status: 200, body: stored}
	r := api.resource(t)
	plan := model()
	plan.CreateTime = types.StringUnknown() // computed, not an Update field
	resp := resource.UpdateResponse{State: toState(t, model())}
	r.Update(context.Background(), resource.UpdateRequest{Plan: toPlan(t, plan), State: toState(t, model())}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("update: %v", resp.Diagnostics)
	}
	api.wantRequests(t, "GET "+rulesPath+"/r1")
}

func TestReadAndDelete(t *testing.T) {
	api := &fakeAPI{status: 200, body: stored}
	r := api.resource(t)
	read := resource.ReadResponse{State: toState(t, model())}
	r.Read(context.Background(), resource.ReadRequest{State: toState(t, model())}, &read)
	if read.Diagnostics.HasError() {
		t.Fatalf("read: %v", read.Diagnostics)
	}
	var del resource.DeleteResponse
	r.Delete(context.Background(), resource.DeleteRequest{State: toState(t, model())}, &del)
	if del.Diagnostics.HasError() {
		t.Fatalf("delete: %v", del.Diagnostics)
	}
	api.wantRequests(t, "GET "+rulesPath+"/r1", "DELETE "+rulesPath+"/r1")
}
