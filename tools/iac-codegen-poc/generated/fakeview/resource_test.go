// This file is handwritten. It runs the generated CRUD code of a full-replace
// resource (E11) against a fake HTTP server: PUT on the item path, with the
// id in the path and not in the body, and the view itself in each response.

package fakeview

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/testsdk/go/openapi/cxsdk"
)

const viewsPath = "/fake/views/v1"

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

const stored = `{"id":"v1","name":"errors","query":"level:error","compact":true,"columns":["time","text"]}`

func model() *FakeViewModel {
	return &FakeViewModel{
		Id:      types.StringValue("v1"),
		Name:    types.StringValue("errors"),
		Query:   types.StringValue("level:error"),
		Compact: types.BoolValue(true),
		Columns: types.ListValueMust(types.StringType, []attr.Value{types.StringValue("time"), types.StringValue("text")}),
	}
}

func emptyState() tfsdk.State {
	return tfsdk.State{Schema: Schema(), Raw: tftypes.NewValue(Schema().Type().TerraformType(context.Background()), nil)}
}

func toState(t *testing.T, m *FakeViewModel) tfsdk.State {
	t.Helper()
	s := emptyState()
	if diags := s.Set(context.Background(), m); diags.HasError() {
		t.Fatalf("set state: %v", diags)
	}
	return s
}

func toPlan(t *testing.T, m *FakeViewModel) tfsdk.Plan {
	t.Helper()
	s := toState(t, m)
	return tfsdk.Plan(s)
}

// TestUpdate checks the full replace on the item path: the id is in the path
// and not in the body, every Update field is sent, a removed value is not,
// and there is no mask.
func TestUpdate(t *testing.T) {
	api := &fakeAPI{status: 200, body: stored}
	r := api.resource(t)
	plan := model()
	plan.Name = types.StringValue("all errors")
	plan.Query = types.StringNull()
	resp := resource.UpdateResponse{State: toState(t, model())}
	r.Update(context.Background(), resource.UpdateRequest{Plan: toPlan(t, plan), State: toState(t, model())}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("update: %v", resp.Diagnostics)
	}
	api.wantRequests(t, "PUT "+viewsPath+"/v1")
	body := api.sent(t, 0)
	if body["name"] != "all errors" || body["compact"] != true {
		t.Errorf("body = %s, want the new name and the unchanged compact", api.bodies[0])
	}
	if cols, _ := body["columns"].([]any); len(cols) != 2 {
		t.Errorf("body columns = %v, want the unchanged 2 columns", body["columns"])
	}
	for _, k := range []string{"id", "query", "updateMask"} {
		if _, ok := body[k]; ok {
			t.Errorf("body has %s: %s", k, api.bodies[0])
		}
	}
}

// TestCreateReadDelete checks the other operations: the response is the view
// itself, and Delete returns {}.
func TestCreateReadDelete(t *testing.T) {
	api := &fakeAPI{status: 200, body: stored}
	r := api.resource(t)
	plan := model()
	plan.Id = types.StringUnknown()
	created := resource.CreateResponse{State: emptyState()}
	r.Create(context.Background(), resource.CreateRequest{Plan: toPlan(t, plan)}, &created)
	if created.Diagnostics.HasError() {
		t.Fatalf("create: %v", created.Diagnostics)
	}
	read := resource.ReadResponse{State: created.State}
	r.Read(context.Background(), resource.ReadRequest{State: created.State}, &read)
	if read.Diagnostics.HasError() {
		t.Fatalf("read: %v", read.Diagnostics)
	}
	api.body = `{}`
	var del resource.DeleteResponse
	r.Delete(context.Background(), resource.DeleteRequest{State: read.State}, &del)
	if del.Diagnostics.HasError() {
		t.Fatalf("delete: %v", del.Diagnostics)
	}
	api.wantRequests(t, "POST "+viewsPath, "GET "+viewsPath+"/v1", "DELETE "+viewsPath+"/v1")
}
