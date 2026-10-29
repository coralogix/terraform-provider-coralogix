package renamething

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"example.com/iac-test-sdk/go/openapi/gen/rename_things_service"
)

func thingSchema(t *testing.T) (*Resource, frameworkresource.SchemaResponse, tftypes.Type) {
	t.Helper()
	ctx := context.Background()
	r := &Resource{client: &rename_things_service.RenameThingsServiceAPIService{}}
	var schemaResp frameworkresource.SchemaResponse
	r.Schema(ctx, frameworkresource.SchemaRequest{}, &schemaResp)
	return r, schemaResp, schemaResp.Schema.Type().TerraformType(ctx)
}

func nullValues(typ tftypes.Type) map[string]tftypes.Value {
	values := map[string]tftypes.Value{}
	for attr, attrType := range typ.(tftypes.Object).AttributeTypes {
		values[attr] = tftypes.NewValue(attrType, nil)
	}
	return values
}

// thingValue is a planned thing with one parameter.
func thingValue(root tftypes.Type, id tftypes.Value, kind string) tftypes.Value {
	values := nullValues(root)
	values["id"], values["kind"] = id, tftypes.NewValue(tftypes.String, kind)
	values["version"] = tftypes.NewValue(tftypes.String, "v2")
	list := root.(tftypes.Object).AttributeTypes["parameters"].(tftypes.List)
	param := nullValues(list.ElementType)
	param["name"] = tftypes.NewValue(tftypes.String, "p")
	param["value"] = tftypes.NewValue(tftypes.String, "x")
	values["parameters"] = tftypes.NewValue(list, []tftypes.Value{tftypes.NewValue(list.ElementType, param)})
	return tftypes.NewValue(root, values)
}

func checkSpec(t *testing.T, spec *rename_things_service.RenameSpec, kind string) {
	t.Helper()
	if spec == nil || spec.SpecKind == nil || *spec.SpecKind != kind || spec.SpecVersion == nil || *spec.SpecVersion != "v2" {
		t.Fatalf("spec = %+v, want kind %s and version v2 under the request names", spec, kind)
	}
	if spec.Holder == nil || len(spec.Holder.Items) != 1 || spec.Holder.Items[0].Name == nil || *spec.Holder.Items[0].Name != "p" {
		t.Fatalf("spec holder = %+v, want the parameter one object deeper", spec.Holder)
	}
}

// Create sends the kind, the version, and the parameters in the request component, under the names
// of their request paths, and no id. The response holds only the id, so Create reads the thing.
func TestCreateWritesTheRequestPaths(t *testing.T) {
	ctx := context.Background()
	rename_things_service.Saved, rename_things_service.GetIDs = nil, nil
	id, kind := rename_things_service.SavedID, "k"
	rename_things_service.Stored = rename_things_service.RenameThing{Id: &id, Kind: &kind}
	r, schemaResp, root := thingSchema(t)
	plan := thingValue(root, tftypes.NewValue(tftypes.String, tftypes.UnknownValue), "k")
	resp := &frameworkresource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.NewValue(root, nil)}}
	r.Create(ctx, frameworkresource.CreateRequest{
		Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: thingValue(root, tftypes.NewValue(tftypes.String, nil), "k")},
		Plan:   tfsdk.Plan{Schema: schemaResp.Schema, Raw: plan},
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if len(rename_things_service.Saved) != 1 {
		t.Fatalf("create bodies = %+v, want one", rename_things_service.Saved)
	}
	checkSpec(t, rename_things_service.Saved[0].Spec, "k")
	if len(rename_things_service.GetIDs) != 1 || rename_things_service.GetIDs[0] != id {
		t.Fatalf("get ids = %v, want the id that Create returned", rename_things_service.GetIDs)
	}
}

// Update sends the id of the state beside the wrapper, not inside it, and the same request paths.
func TestUpdateSendsTheIDBesideTheWrapper(t *testing.T) {
	ctx := context.Background()
	rename_things_service.Updated, rename_things_service.GetIDs = nil, nil
	id, kind := "thing-9", "k2"
	rename_things_service.Stored = rename_things_service.RenameThing{Id: &id, Kind: &kind}
	r, schemaResp, root := thingSchema(t)
	idValue := tftypes.NewValue(tftypes.String, id)
	state, plan := thingValue(root, idValue, "k"), thingValue(root, idValue, "k2")
	resp := &frameworkresource.UpdateResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: state}}
	r.Update(ctx, frameworkresource.UpdateRequest{
		Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: thingValue(root, tftypes.NewValue(tftypes.String, nil), "k2")},
		Plan:   tfsdk.Plan{Schema: schemaResp.Schema, Raw: plan},
		State:  tfsdk.State{Schema: schemaResp.Schema, Raw: state},
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if len(rename_things_service.Updated) != 1 {
		t.Fatalf("update bodies = %+v, want one", rename_things_service.Updated)
	}
	body := rename_things_service.Updated[0]
	if body.Id == nil || *body.Id != id {
		t.Fatalf("update body id = %v, want %s beside the wrapper", body.Id, id)
	}
	checkSpec(t, body.Spec, "k2")
	if len(rename_things_service.GetIDs) != 1 || rename_things_service.GetIDs[0] != id {
		t.Fatalf("get ids = %v, want the id of the thing", rename_things_service.GetIDs)
	}
}

// withSettings returns the thing value with a settings object.
func withSettings(root tftypes.Type, v tftypes.Value, setting string) tftypes.Value {
	var values map[string]tftypes.Value
	if err := v.As(&values); err != nil {
		panic(err)
	}
	obj := tftypes.Object{AttributeTypes: map[string]tftypes.Type{"mode": tftypes.String}}
	values["settings"] = tftypes.NewValue(obj, map[string]tftypes.Value{"mode": tftypes.NewValue(tftypes.String, setting)})
	return tftypes.NewValue(root, values)
}

// A change of only the dynamic attribute with inUpdate sends an Update, and BeforeWrite runs before
// the call.
func TestUpdateWhenOnlyTheDynamicAttributeChanges(t *testing.T) {
	ctx := context.Background()
	rename_things_service.Updated, rename_things_service.GetIDs = nil, nil
	id, kind := "thing-9", "k"
	rename_things_service.Stored = rename_things_service.RenameThing{Id: &id, Kind: &kind}
	r, schemaResp, root := thingSchema(t)
	var order []string
	r.hooks.BeforeWrite = func(context.Context, tfsdk.Config, any) diag.Diagnostics {
		order = append(order, "BeforeWrite")
		return nil
	}
	idValue := tftypes.NewValue(tftypes.String, id)
	state := withSettings(root, thingValue(root, idValue, "k"), "a")
	plan := withSettings(root, thingValue(root, idValue, "k"), "b")
	resp := &frameworkresource.UpdateResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: state}}
	r.Update(ctx, frameworkresource.UpdateRequest{
		Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: plan},
		Plan:   tfsdk.Plan{Schema: schemaResp.Schema, Raw: plan},
		State:  tfsdk.State{Schema: schemaResp.Schema, Raw: state},
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if len(rename_things_service.Updated) != 1 {
		t.Fatalf("update bodies = %+v, want one for the change of the dynamic attribute", rename_things_service.Updated)
	}
	if len(order) != 1 || order[0] != "BeforeWrite" {
		t.Fatalf("hook calls = %v, want one BeforeWrite", order)
	}
	// The API holds no value of the dynamic attribute, so the state keeps the planned one with no hook.
	var planned, got map[string]tftypes.Value
	if err := plan.As(&planned); err != nil {
		t.Fatal(err)
	}
	if err := resp.State.Raw.As(&got); err != nil {
		t.Fatal(err)
	}
	if !got["settings"].Equal(planned["settings"]) {
		t.Fatalf("state settings = %v, want the planned %v", got["settings"], planned["settings"])
	}
}
