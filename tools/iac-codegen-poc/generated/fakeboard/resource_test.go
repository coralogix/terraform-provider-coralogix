// This file is handwritten. It runs the generated CRUD code against a fake
// HTTP server through the fake SDK.

package fakeboard

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/fakesdk/go/openapi/cxsdk"
)

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

func toState(t *testing.T, m *FakeBoardModel) tfsdk.State {
	t.Helper()
	ctx := context.Background()
	s := tfsdk.State{Schema: Schema(), Raw: tftypes.NewValue(Schema().Type().TerraformType(ctx), nil)}
	if diags := s.Set(ctx, m); diags.HasError() {
		t.Fatalf("set state: %v", diags)
	}
	return s
}

func toPlan(t *testing.T, m *FakeBoardModel) tfsdk.Plan {
	t.Helper()
	s := toState(t, m)
	return tfsdk.Plan{Schema: s.Schema, Raw: s.Raw}
}

// response returns the API JSON of the board m, with id b1. The fake API
// returns the board itself, not inside a wrapper field.
func response(t *testing.T, m *FakeBoardModel) string {
	t.Helper()
	body, diags := expandCreate(context.Background(), m)
	assertNoDiags(t, diags)
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	return `{"id":"b1",` + string(b[1:])
}

func TestCreate(t *testing.T) {
	plan := full(t)
	api := &fakeAPI{status: 200, body: response(t, plan)}
	r := api.resource(t)
	plan.Id, plan.UpdatedAt = types.StringUnknown(), types.StringUnknown()
	empty := tfsdk.State{Schema: Schema(), Raw: tftypes.NewValue(Schema().Type().TerraformType(context.Background()), nil)}
	resp := resource.CreateResponse{State: empty}
	r.Create(context.Background(), resource.CreateRequest{Plan: toPlan(t, plan)}, &resp)
	assertNoDiags(t, resp.Diagnostics)
	if want := "POST /fake/boards/v1"; len(api.requests) != 1 || api.requests[0] != want {
		t.Fatalf("requests = %v, want [%s]", api.requests, want)
	}
	assertJSON(t, json.RawMessage(api.bodies[0]), fullJSON)
	var got FakeBoardModel
	assertNoDiags(t, resp.State.Get(context.Background(), &got))
	if got.Id.ValueString() != "b1" || got.Layout.Section.Header.Style.Font.Size.ValueInt64() != 12 {
		t.Errorf("state = %+v", got)
	}
}

// TestUpdateNested changes one value three levels deep. The spec pattern
// accepts nested paths, so the mask names the leaf. The body has the whole
// layout; the server changes only the masked path.
func TestUpdateNested(t *testing.T) {
	state := full(t)
	state.Id = types.StringValue("b1")
	plan := full(t)
	plan.Id = types.StringValue("b1")
	plan.Layout.Section.Header.Text = types.StringValue("Memory")

	api := &fakeAPI{status: 200, body: response(t, plan)}
	r := api.resource(t)
	resp := resource.UpdateResponse{State: toState(t, plan)}
	r.Update(context.Background(), resource.UpdateRequest{Plan: toPlan(t, plan), State: toState(t, state)}, &resp)
	assertNoDiags(t, resp.Diagnostics)
	if want := "PATCH /fake/boards/v1/b1"; len(api.requests) != 1 || api.requests[0] != want {
		t.Fatalf("requests = %v, want [%s]", api.requests, want)
	}
	var body struct {
		UpdateMask string
		Layout     json.RawMessage
	}
	if err := json.Unmarshal([]byte(api.bodies[0]), &body); err != nil {
		t.Fatal(err)
	}
	if want := "layout.section.header.text"; body.UpdateMask != want {
		t.Errorf("updateMask = %q, want %s", body.UpdateMask, want)
	}
	want, diags := expandUpdate(context.Background(), plan)
	assertNoDiags(t, diags)
	b, err := json.Marshal(want.Layout)
	if err != nil {
		t.Fatal(err)
	}
	assertJSON(t, body.Layout, string(b))
}
