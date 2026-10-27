package upsertthing

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"example.com/iac-test-sdk/go/openapi/gen/upsert_service"
)

// values is one value of each attribute. nil is null, and unknown is a value that Terraform plans.
type values map[string]any

var unknown = struct{}{}

func raw(t *testing.T, s frameworkresource.SchemaResponse, v values) tftypes.Value {
	t.Helper()
	ctx := context.Background()
	root := s.Schema.Type().TerraformType(ctx).(tftypes.Object)
	attrs := map[string]tftypes.Value{}
	for name, typ := range root.AttributeTypes {
		switch value := v[name]; value {
		case unknown:
			attrs[name] = tftypes.NewValue(typ, tftypes.UnknownValue)
		default:
			attrs[name] = tftypes.NewValue(typ, value)
		}
	}
	return tftypes.NewValue(root, attrs)
}

func setup(t *testing.T, stored *upsert_service.Target) (*Resource, frameworkresource.SchemaResponse) {
	t.Helper()
	upsert_service.Stored, upsert_service.Sets = stored, nil
	r := &Resource{client: &upsert_service.UpsertServiceAPIService{}}
	var s frameworkresource.SchemaResponse
	r.Schema(context.Background(), frameworkresource.SchemaRequest{}, &s)
	return r, s
}

func get(t *testing.T, state tfsdk.State, name string) any {
	t.Helper()
	var out TargetModel
	if diags := state.Get(context.Background(), &out); diags.HasError() {
		t.Fatal(diags)
	}
	return map[string]any{
		"id": out.Id.ValueString(), "active": out.Enabled.ValueBool(), "version": out.Version.ValueString(),
		"name": out.Name.ValueString(), "zone": out.Zone.ValueString(), "token": out.Token.ValueString(),
	}[name]
}

func noErrors(t *testing.T, diags diag.Diagnostics) {
	t.Helper()
	if diags.HasError() {
		t.Fatal(diags)
	}
}

// Create sends the inlined fields in their objects with the Set call, and the state has the
// fields of the response objects and the fixed id.
func TestCreateSetsTheSingleton(t *testing.T) {
	r, s := setup(t, nil)
	config := raw(t, s, values{"name": "n", "active": true})
	plan := raw(t, s, values{"id": "upsert-thing-settings", "name": "n", "active": true, "version": unknown, "zone": unknown, "token": unknown})
	resp := &frameworkresource.CreateResponse{State: tfsdk.State{Schema: s.Schema}}
	r.Create(context.Background(), frameworkresource.CreateRequest{Config: tfsdk.Config{Schema: s.Schema, Raw: config}, Plan: tfsdk.Plan{Schema: s.Schema, Raw: plan}}, resp)
	noErrors(t, resp.Diagnostics)
	if len(upsert_service.Sets) != 1 {
		t.Fatalf("Set calls = %d, want 1", len(upsert_service.Sets))
	}
	sent := upsert_service.Sets[0]
	if !sent.Enabled || sent.Spec.Name != "n" || sent.Spec.Zone != nil || sent.Spec.Token != nil {
		t.Fatalf("Set body = %+v, want enabled, the name, and no zone or token", sent)
	}
	for name, want := range map[string]any{"id": "upsert-thing-settings", "active": true, "version": "v1", "name": "n", "zone": "server-zone", "token": "derived"} {
		if got := get(t, resp.State, name); got != want {
			t.Errorf("%s = %v, want %v", name, got, want)
		}
	}
}

// Update sends the whole resource with the Set call.
func TestUpdateSetsTheWholeResource(t *testing.T) {
	zone, version, token, enabled := "eu", "v1", "derived", true
	r, s := setup(t, &upsert_service.Target{Meta: upsert_service.Meta{Enabled: &enabled, Version: &version}, Spec: &upsert_service.Spec{Name: "n", Zone: &zone, Token: &token}})
	state := raw(t, s, values{"id": "upsert-thing-settings", "name": "n", "active": true, "version": "v1", "zone": "eu", "token": "derived"})
	config := raw(t, s, values{"name": "n", "active": false})
	plan := raw(t, s, values{"id": "upsert-thing-settings", "name": "n", "active": false, "version": "v1", "zone": "eu", "token": "derived"})
	resp := &frameworkresource.UpdateResponse{State: tfsdk.State{Schema: s.Schema}}
	r.Update(context.Background(), frameworkresource.UpdateRequest{
		Config: tfsdk.Config{Schema: s.Schema, Raw: config}, Plan: tfsdk.Plan{Schema: s.Schema, Raw: plan}, State: tfsdk.State{Schema: s.Schema, Raw: state},
	}, resp)
	noErrors(t, resp.Diagnostics)
	if len(upsert_service.Sets) != 1 {
		t.Fatalf("Set calls = %d, want 1", len(upsert_service.Sets))
	}
	if sent := upsert_service.Sets[0]; sent.Enabled || sent.Spec.Name != "n" || sent.Spec.Zone == nil || *sent.Spec.Zone != "eu" {
		t.Fatalf("Set body = %+v, want the whole resource with enabled false", sent)
	}
	if got := get(t, resp.State, "active"); got != false {
		t.Fatalf("active = %v, want false", got)
	}
}

// Read removes the resource when the API returns another arm: the company has no such target.
func TestReadWithoutTheInlinedArmRemovesTheResource(t *testing.T) {
	r, s := setup(t, nil)
	state := raw(t, s, values{"id": "upsert-thing-settings", "name": "n", "active": true, "version": "v1", "zone": "eu", "token": "derived"})
	resp := &frameworkresource.ReadResponse{State: tfsdk.State{Schema: s.Schema, Raw: state}}
	r.Read(context.Background(), frameworkresource.ReadRequest{State: tfsdk.State{Schema: s.Schema, Raw: state}}, resp)
	noErrors(t, resp.Diagnostics)
	if !resp.State.Raw.IsNull() {
		t.Fatal("the state still has the resource")
	}
	if resp.Diagnostics.WarningsCount() != 1 {
		t.Fatalf("diagnostics = %v, want one warning", resp.Diagnostics)
	}
}

// Delete makes no call: the API cannot remove the singleton.
func TestDeleteMakesNoCall(t *testing.T) {
	zone, version, token, enabled := "eu", "v1", "derived", true
	stored := &upsert_service.Target{Meta: upsert_service.Meta{Enabled: &enabled, Version: &version}, Spec: &upsert_service.Spec{Name: "n", Zone: &zone, Token: &token}}
	r, s := setup(t, stored)
	state := raw(t, s, values{"id": "upsert-thing-settings", "name": "n", "active": true, "version": "v1", "zone": "eu", "token": "derived"})
	resp := &frameworkresource.DeleteResponse{State: tfsdk.State{Schema: s.Schema, Raw: state}}
	r.Delete(context.Background(), frameworkresource.DeleteRequest{State: tfsdk.State{Schema: s.Schema, Raw: state}}, resp)
	noErrors(t, resp.Diagnostics)
	if len(upsert_service.Sets) != 0 || upsert_service.Stored != stored {
		t.Fatal("Delete called the API")
	}
	if resp.Diagnostics.WarningsCount() != 1 {
		t.Fatalf("diagnostics = %v, want one warning that the API keeps the resource", resp.Diagnostics)
	}
}
