package configurationgroup

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// A rename leaves integration_version unknown, so the update mask can reset it.
// The mask must name only name. A chart_version change still plans the field unknown.
func TestRenameKeepsIntegrationVersionOutOfTheMask(t *testing.T) {
	ctx := context.Background()
	state := presetResource(t, "a", "0.0.353", types.StringValue("0.9.0"))
	config := presetResource(t, "b", "0.0.353", types.StringNull())
	plan := presetResource(t, "b", "0.0.353", types.StringUnknown())
	plan = withIntegrationVersion(t, ctx, state, config, plan, "b", "0.0.353")

	mask, diags := updateMask(ctx,
		tfsdk.Config{Schema: Schema(), Raw: config},
		tfsdk.Plan{Schema: Schema(), Raw: plan},
		tfsdk.State{Schema: Schema(), Raw: state},
	)
	if diags.HasError() {
		t.Fatal(diags)
	}
	if len(mask) != 1 || mask[0] != "name" {
		t.Fatalf("mask = %v, want [name]", mask)
	}

	config = presetResource(t, "a", "0.0.354", types.StringNull())
	plan = presetResource(t, "a", "0.0.354", types.StringUnknown())
	plan = withIntegrationVersion(t, ctx, state, config, plan, "a", "0.0.354")
	mask, diags = updateMask(ctx,
		tfsdk.Config{Schema: Schema(), Raw: config},
		tfsdk.Plan{Schema: Schema(), Raw: plan},
		tfsdk.State{Schema: Schema(), Raw: state},
	)
	if diags.HasError() {
		t.Fatal(diags)
	}
	found := false
	for _, path := range mask {
		if path == "family.preset.integrationVersion" {
			found = true
		}
	}
	if !found {
		t.Fatalf("mask = %v, want family.preset.integrationVersion", mask)
	}
}

func withIntegrationVersion(t *testing.T, ctx context.Context, state, config, plan tftypes.Value, name, chartVersion string) tftypes.Value {
	t.Helper()
	family := Schema().Attributes["family"].(schema.SingleNestedAttribute)
	preset := family.Attributes["preset"].(schema.SingleNestedAttribute)
	attr := preset.Attributes["integration_version"].(schema.StringAttribute)
	req := planmodifier.StringRequest{
		Path:        path.Root("family").AtName("preset").AtName("integration_version"),
		ConfigValue: types.StringNull(),
		PlanValue:   types.StringUnknown(),
		StateValue:  types.StringValue("0.9.0"),
		Config:      tfsdk.Config{Schema: Schema(), Raw: config},
		Plan:        tfsdk.Plan{Schema: Schema(), Raw: plan},
		State:       tfsdk.State{Schema: Schema(), Raw: state},
	}
	for _, modifier := range attr.PlanModifiers {
		resp := &planmodifier.StringResponse{PlanValue: req.PlanValue}
		modifier.PlanModifyString(ctx, req, resp)
		if resp.Diagnostics.HasError() {
			t.Fatal(resp.Diagnostics)
		}
		req.PlanValue = resp.PlanValue
	}
	return presetResource(t, name, chartVersion, req.PlanValue)
}

func presetResource(t *testing.T, name, chartVersion string, integration types.String) tftypes.Value {
	t.Helper()
	root := Schema().Type().TerraformType(context.Background()).(tftypes.Object)
	familyType := root.AttributeTypes["family"].(tftypes.Object)
	presetType := familyType.AttributeTypes["preset"].(tftypes.Object)
	integrationValue := tftypes.NewValue(presetType.AttributeTypes["integration_version"], nil)
	if integration.IsUnknown() {
		integrationValue = tftypes.NewValue(presetType.AttributeTypes["integration_version"], tftypes.UnknownValue)
	} else if !integration.IsNull() {
		integrationValue = tftypes.NewValue(presetType.AttributeTypes["integration_version"], integration.ValueString())
	}
	preset := objectWith(presetType, map[string]tftypes.Value{
		"chart_name":             tftypes.NewValue(presetType.AttributeTypes["chart_name"], "chart_name_otel_integration"),
		"chart_version":          tftypes.NewValue(presetType.AttributeTypes["chart_version"], chartVersion),
		"integration_version":    integrationValue,
		"observability_features": tftypes.NewValue(presetType.AttributeTypes["observability_features"], "{}"),
	})
	family := objectWith(familyType, map[string]tftypes.Value{"preset": preset})
	return objectWith(root, map[string]tftypes.Value{
		"name":   tftypes.NewValue(root.AttributeTypes["name"], name),
		"family": family,
	})
}

func objectWith(typ tftypes.Object, set map[string]tftypes.Value) tftypes.Value {
	values := map[string]tftypes.Value{}
	for name, attrType := range typ.AttributeTypes {
		if value, ok := set[name]; ok {
			values[name] = value
			continue
		}
		values[name] = tftypes.NewValue(attrType, nil)
	}
	return tftypes.NewValue(typ, values)
}
