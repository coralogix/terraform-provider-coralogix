package renamething

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"example.com/iac-test-sdk/go/openapi/gen/rename_things_service"
)

func TestAfterFlattenReplacesPriorDynamicExtra(t *testing.T) {
	ctx := context.Background()
	r, schemaResp, root := thingSchema(t)
	id, kind := "thing-9", "k"
	priorValue := withSettings(root, thingValue(root, tftypes.NewValue(tftypes.String, id), kind), "prior")
	prior := tfsdk.State{Schema: schemaResp.Schema, Raw: priorValue}
	r.hooks.AfterFlatten = func(_ context.Context, m *RenameThingModel, _ *rename_things_service.RenameThing) diag.Diagnostics {
		m.Settings = types.DynamicValue(types.StringValue("from-api"))
		return nil
	}
	state := tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.NewValue(root, nil)}
	if diags := r.setState(ctx, &rename_things_service.RenameThing{Id: &id, Kind: &kind}, prior, &state); diags.HasError() {
		t.Fatal(diags)
	}
	var got types.Dynamic
	if diags := state.GetAttribute(ctx, path.Root("settings"), &got); diags.HasError() {
		t.Fatal(diags)
	}
	value, ok := got.UnderlyingValue().(types.String)
	if !ok || value.ValueString() != "from-api" {
		t.Fatalf("settings = %v, want the hook value", got)
	}
}
