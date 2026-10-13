package customthing

import (
	"context"
	"testing"

	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"example.com/iac-test-sdk/go/openapi/gen/custom_things_service"
)

// thingValue is the resource with the id and the name, and every other attribute null.
func thingValue(t *testing.T, root tftypes.Type, id, name tftypes.Value) tftypes.Value {
	t.Helper()
	object, ok := root.(tftypes.Object)
	if !ok {
		t.Fatalf("schema type %T, want an object", root)
	}
	values := map[string]tftypes.Value{}
	for attr, attrType := range object.AttributeTypes {
		values[attr] = tftypes.NewValue(attrType, nil)
	}
	values["id"], values["name"] = id, name
	return tftypes.NewValue(root, values)
}

// The id of this resource is server-set and its Update has the id in the body, so no Update
// field sends it. The generated Update copies the state id into the body.
func TestUpdateSendsTheStateIDInTheBody(t *testing.T) {
	ctx := context.Background()
	custom_things_service.Replaced = nil
	r := &Resource{client: &custom_things_service.CustomThingsServiceAPIService{}}
	var schemaResp frameworkresource.SchemaResponse
	r.Schema(ctx, frameworkresource.SchemaRequest{}, &schemaResp)
	root := schemaResp.Schema.Type().TerraformType(ctx)
	id := tftypes.NewValue(tftypes.String, "thing-1")
	state := thingValue(t, root, id, tftypes.NewValue(tftypes.String, "old"))
	plan := thingValue(t, root, id, tftypes.NewValue(tftypes.String, "new"))
	config := thingValue(t, root, tftypes.NewValue(tftypes.String, nil), tftypes.NewValue(tftypes.String, "new"))
	resp := &frameworkresource.UpdateResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: plan}}
	r.Update(ctx, frameworkresource.UpdateRequest{
		Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: config},
		Plan:   tfsdk.Plan{Schema: schemaResp.Schema, Raw: plan},
		State:  tfsdk.State{Schema: schemaResp.Schema, Raw: state},
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if len(custom_things_service.Replaced) != 1 {
		t.Fatalf("replace calls = %d, want 1", len(custom_things_service.Replaced))
	}
	thing := custom_things_service.Replaced[0].Thing
	if thing == nil || thing.Id == nil || *thing.Id != "thing-1" {
		t.Fatalf("replace body thing = %+v, want id thing-1", thing)
	}
	if thing.Name == nil || *thing.Name != "new" {
		t.Fatalf("replace body name = %v, want new", thing.Name)
	}
}
