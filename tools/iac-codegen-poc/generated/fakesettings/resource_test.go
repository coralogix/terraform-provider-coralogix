// This file is handwritten. It runs the generated CRUD code of a singleton
// (D18) against a fake HTTP server: no id in the path, a fixed id attribute,
// and an empty Delete response.

package fakesettings

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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/testsdk/go/openapi/cxsdk"
)

const settingsPath = "/fake/settings/v1"

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

const stored = `{"settings":{"enabled":true,"allowList":["10.0.0.1"],"mode":"STRICT","updatedAt":"2026-09-25T10:00:00Z"}}`

func model(enabled bool) *FakeSettingsModel {
	return &FakeSettingsModel{
		Id:        types.StringValue(TypeName),
		Enabled:   types.BoolValue(enabled),
		AllowList: types.ListValueMust(types.StringType, []attr.Value{types.StringValue("10.0.0.1")}),
		Mode:      types.StringValue("STRICT"),
		UpdatedAt: types.StringValue("2026-09-25T10:00:00Z"),
	}
}

func emptyState() tfsdk.State {
	return tfsdk.State{Schema: Schema(), Raw: tftypes.NewValue(Schema().Type().TerraformType(context.Background()), nil)}
}

func toState(t *testing.T, m *FakeSettingsModel) tfsdk.State {
	t.Helper()
	s := emptyState()
	if diags := s.Set(context.Background(), m); diags.HasError() {
		t.Fatalf("set state: %v", diags)
	}
	return s
}

func toPlan(t *testing.T, m *FakeSettingsModel) tfsdk.Plan {
	t.Helper()
	s := toState(t, m)
	return tfsdk.Plan{Schema: s.Schema, Raw: s.Raw}
}

func idOf(t *testing.T, s tfsdk.State) string {
	t.Helper()
	var id types.String
	if diags := s.GetAttribute(context.Background(), path.Root("id"), &id); diags.HasError() {
		t.Fatalf("id: %v", diags)
	}
	return id.ValueString()
}

// TestSchemaID checks the fixed, read-only id attribute of a singleton.
func TestSchemaID(t *testing.T) {
	id, ok := Schema().Attributes["id"].(schema.StringAttribute)
	if !ok || !id.Computed || id.Optional || id.Required {
		t.Errorf("id = %+v, want a computed string", Schema().Attributes["id"])
	}
}

func TestCreate(t *testing.T) {
	api := &fakeAPI{status: 200, body: stored}
	r := api.resource(t)
	plan := model(true)
	plan.Id, plan.UpdatedAt = types.StringUnknown(), types.StringUnknown()
	resp := resource.CreateResponse{State: emptyState()}
	r.Create(context.Background(), resource.CreateRequest{Plan: toPlan(t, plan)}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("create: %v", resp.Diagnostics)
	}
	api.wantRequests(t, "POST "+settingsPath)
	if got := idOf(t, resp.State); got != TypeName {
		t.Errorf("id = %q, want the fixed %q", got, TypeName)
	}
}

func TestRead(t *testing.T) {
	api := &fakeAPI{status: 200, body: stored}
	r := api.resource(t)
	resp := resource.ReadResponse{State: toState(t, model(false))}
	r.Read(context.Background(), resource.ReadRequest{State: toState(t, model(false))}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("read: %v", resp.Diagnostics)
	}
	api.wantRequests(t, "GET "+settingsPath) // no id in the path
	var got FakeSettingsModel
	resp.State.Get(context.Background(), &got)
	if !got.Enabled.ValueBool() {
		t.Errorf("enabled = %s, want true from the API", got.Enabled)
	}
}

// TestReadNotCreated checks that a 404 (the singleton was not created, or was
// deleted) removes it from the state.
func TestReadNotCreated(t *testing.T) {
	api := &fakeAPI{status: 404, body: `{"code":404,"message":"Not Found: no settings"}`}
	r := api.resource(t)
	resp := resource.ReadResponse{State: toState(t, model(true))}
	r.Read(context.Background(), resource.ReadRequest{State: toState(t, model(true))}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("read: %v", resp.Diagnostics)
	}
	if !resp.State.Raw.IsNull() {
		t.Error("the state is not removed")
	}
}

// TestImport checks that any import id works: the Read after the import sets
// the fixed id.
func TestImport(t *testing.T) {
	r := (&fakeAPI{status: 200, body: stored}).resource(t)
	resp := resource.ImportStateResponse{State: emptyState()}
	r.ImportState(context.Background(), resource.ImportStateRequest{ID: "anything"}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("import: %v", resp.Diagnostics)
	}
	read := resource.ReadResponse{State: resp.State}
	r.Read(context.Background(), resource.ReadRequest{State: resp.State}, &read)
	if read.Diagnostics.HasError() {
		t.Fatalf("read: %v", read.Diagnostics)
	}
	if got := idOf(t, read.State); got != TypeName {
		t.Errorf("id after the read = %q, want %q", got, TypeName)
	}
}

func TestUpdate(t *testing.T) {
	api := &fakeAPI{status: 200, body: stored}
	r := api.resource(t)
	resp := resource.UpdateResponse{State: toState(t, model(true))}
	r.Update(context.Background(), resource.UpdateRequest{Plan: toPlan(t, model(true)), State: toState(t, model(false))}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("update: %v", resp.Diagnostics)
	}
	api.wantRequests(t, "PATCH "+settingsPath)
	var body struct{ UpdateMask string }
	if err := json.Unmarshal([]byte(api.bodies[0]), &body); err != nil {
		t.Fatal(err)
	}
	if body.UpdateMask != "enabled" {
		t.Errorf("updateMask = %q, want enabled", body.UpdateMask)
	}
}

// TestDelete checks the empty Delete response ({}), and that 404 is not an
// error.
func TestDelete(t *testing.T) {
	for _, c := range []struct {
		status  int
		body    string
		wantErr bool
	}{
		{200, `{}`, false},
		{404, `{"code":404,"message":"Not Found: no settings"}`, false},
		{500, `{"code":500,"message":"boom"}`, true},
	} {
		api := &fakeAPI{status: c.status, body: c.body}
		r := api.resource(t)
		var resp resource.DeleteResponse
		r.Delete(context.Background(), resource.DeleteRequest{State: toState(t, model(true))}, &resp)
		if resp.Diagnostics.HasError() != c.wantErr {
			t.Errorf("status %d: error = %v, want error %t", c.status, resp.Diagnostics, c.wantErr)
		}
		api.wantRequests(t, "DELETE "+settingsPath)
	}
}
