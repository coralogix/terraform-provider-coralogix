// This file is handwritten. It tests the generated CRUD code in resource.go
// against a fake API server. It needs no network access.

package aievaluation

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var idPath = path.Root("id")

// testID is a UUID. The SDK rejects an id that is not 36 characters long.
const testID = "11111111-2222-3333-4444-555555555555"

// apiEvaluation is the JSON of stored() with testID, as the API sends it.
const apiEvaluation = `{"id":"` + testID + `","application":"app","subsystem":"sub","target":"PROMPT",
	"isEnabled":true,"threshold":0.5,"config":{"allowedTopics":{"topics":["a","b"]}},
	"updatedAt":"2026-09-25T10:00:00Z"}`

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
		io.WriteString(w, f.body)
	}))
	t.Cleanup(srv.Close)
	cs := cxsdk.NewClientSet(cxsdk.NewConfigBuilder().WithAPIKey("test").WithURL(srv.URL).Build())
	r := &Resource{}
	var resp resource.ConfigureResponse
	r.Configure(context.Background(), resource.ConfigureRequest{ProviderData: cs}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("configure: %v", resp.Diagnostics)
	}
	return r
}

func storedWithID() *AiEvaluationModel {
	m := stored()
	m.Id = types.StringValue(testID)
	return m
}

func emptyState() tfsdk.State {
	return tfsdk.State{Schema: Schema(), Raw: tftypes.NewValue(Schema().Type().TerraformType(context.Background()), nil)}
}

func TestCreate(t *testing.T) {
	api := &fakeAPI{status: 200, body: `{"aiEvaluation":` + apiEvaluation + `}`}
	r := api.resource(t)
	plan := storedWithID()
	plan.Id = types.StringUnknown()
	resp := resource.CreateResponse{State: emptyState()}
	r.Create(context.Background(), resource.CreateRequest{Plan: toPlan(t, plan)}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("create: %v", resp.Diagnostics)
	}
	if want := []string{"POST /ai/evaluations/v3"}; !equal(api.requests, want) {
		t.Errorf("requests = %v, want %v", api.requests, want)
	}
	var got AiEvaluationModel
	resp.State.Get(context.Background(), &got)
	if got.Id.ValueString() != testID || got.Threshold.ValueFloat64() != 0.5 {
		t.Errorf("state = %+v", got)
	}
}

func TestCreateAPIError(t *testing.T) {
	api := &fakeAPI{status: 409, body: `{"code":409,"message":"Already Exists: An evaluation with these parameters already exists"}`}
	r := api.resource(t)
	plan := storedWithID()
	plan.Id = types.StringUnknown()
	resp := resource.CreateResponse{State: emptyState()}
	r.Create(context.Background(), resource.CreateRequest{Plan: toPlan(t, plan)}, &resp)
	if len(resp.Diagnostics) != 1 {
		t.Fatalf("diagnostics = %v, want 1 error", resp.Diagnostics)
	}
	d := resp.Diagnostics[0]
	if d.Summary() != "Unable to create ai_evaluation" || !strings.Contains(d.Detail(), "Already Exists") {
		t.Errorf("diagnostic = %q: %q", d.Summary(), d.Detail())
	}
	if !resp.State.Raw.IsNull() {
		t.Errorf("state is set after a failed create")
	}
}

// TestCreatePartial: the API creates the resource, but flatten fails. The
// state keeps the id, so Terraform can delete the resource later.
func TestCreatePartial(t *testing.T) {
	bad := strings.Replace(apiEvaluation, `"allowedTopics":{"topics":["a","b"]}`, `"sqlLoad":{"joinLimit":"-1"}`, 1)
	api := &fakeAPI{status: 200, body: `{"aiEvaluation":` + bad + `}`}
	r := api.resource(t)
	plan := storedWithID()
	plan.Id = types.StringUnknown()
	resp := resource.CreateResponse{State: emptyState()}
	r.Create(context.Background(), resource.CreateRequest{Plan: toPlan(t, plan)}, &resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("create: no error")
	}
	var id types.String
	resp.State.GetAttribute(context.Background(), idPath, &id)
	if id.ValueString() != testID {
		t.Errorf("id in state = %s, want %s", id, testID)
	}
}

func TestReadNotFound(t *testing.T) {
	api := &fakeAPI{status: 404, body: `{"code":404,"message":"Not Found: evaluation not found"}`}
	r := api.resource(t)
	resp := resource.ReadResponse{State: toState(t, storedWithID())}
	r.Read(context.Background(), resource.ReadRequest{State: toState(t, storedWithID())}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("read: %v", resp.Diagnostics)
	}
	if !resp.State.Raw.IsNull() {
		t.Error("the resource is still in the state")
	}
}

// TestReadImport: after an import, the state has only the id. Read fills the rest.
func TestReadImport(t *testing.T) {
	api := &fakeAPI{status: 200, body: `{"aiEvaluation":` + apiEvaluation + `}`}
	r := api.resource(t)
	state := emptyState()
	state.SetAttribute(context.Background(), idPath, testID)
	resp := resource.ReadResponse{State: state}
	r.Read(context.Background(), resource.ReadRequest{State: state}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("read: %v", resp.Diagnostics)
	}
	if want := []string{"GET /ai/evaluations/v3/" + testID}; !equal(api.requests, want) {
		t.Errorf("requests = %v, want %v", api.requests, want)
	}
	var got AiEvaluationModel
	resp.State.Get(context.Background(), &got)
	if got.Application.ValueString() != "app" || got.Config == nil || got.Config.AllowedTopics == nil {
		t.Errorf("state = %+v", got)
	}
}

func TestUpdate(t *testing.T) {
	cases := []struct {
		name   string
		change func(m *AiEvaluationModel)
		want   string // the request
		mask   string // the update mask, "" when there is no PATCH
	}{
		{"no change sends no PATCH (D14)", func(m *AiEvaluationModel) {}, "GET /ai/evaluations/v3/" + testID, ""},
		{"change", func(m *AiEvaluationModel) { m.Threshold = types.Float64Null() }, "PATCH /ai/evaluations/v3/" + testID, "threshold"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			api := &fakeAPI{status: 200, body: `{"aiEvaluation":` + apiEvaluation + `}`}
			r := api.resource(t)
			plan := storedWithID()
			c.change(plan)
			state := toState(t, storedWithID())
			resp := resource.UpdateResponse{State: toState(t, plan)}
			r.Update(context.Background(), resource.UpdateRequest{Plan: toPlan(t, plan), State: state}, &resp)
			if resp.Diagnostics.HasError() {
				t.Fatalf("update: %v", resp.Diagnostics)
			}
			if want := []string{c.want}; !equal(api.requests, want) {
				t.Fatalf("requests = %v, want %v", api.requests, want)
			}
			if c.mask != "" {
				var body struct{ UpdateMask string }
				json.Unmarshal([]byte(api.bodies[0]), &body)
				if body.UpdateMask != c.mask {
					t.Errorf("updateMask = %q, want %q", body.UpdateMask, c.mask)
				}
			}
		})
	}
}

func TestDelete(t *testing.T) {
	cases := []struct {
		status int
		ok     bool
	}{
		{200, true},
		{404, true}, // already deleted
		{500, false},
	}
	for _, c := range cases {
		api := &fakeAPI{status: c.status, body: `{}`}
		r := api.resource(t)
		var resp resource.DeleteResponse
		r.Delete(context.Background(), resource.DeleteRequest{State: toState(t, storedWithID())}, &resp)
		if resp.Diagnostics.HasError() == c.ok {
			t.Errorf("status %d: diagnostics = %v", c.status, resp.Diagnostics)
		}
		if want := []string{"DELETE /ai/evaluations/v3/" + testID}; !equal(api.requests, want) {
			t.Errorf("status %d: requests = %v, want %v", c.status, api.requests, want)
		}
	}
}

func equal(a, b []string) bool { return strings.Join(a, "\n") == strings.Join(b, "\n") }
