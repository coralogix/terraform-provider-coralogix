package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// A rename keeps integration_version. A chart_version change still plans it unknown.
// Plan modifiers do not call the API.
func TestPlanRenameKeepsIntegrationVersion(t *testing.T) {
	ctx := context.Background()
	object := configurationGroupObject(ctx, t)
	state := configurationGroupValue(t, object, "a", "0.0.353", "0.9.0")
	renamed := configurationGroupValue(t, object, "b", "0.0.353", nil)
	planned := planConfigurationGroup(ctx, t, object, state, renamed, configurationGroupValue(t, object, "b", "0.0.353", tftypes.UnknownValue))
	if got := stringValue(t, planned, "family", "preset", "integration_version"); got != "0.9.0" {
		t.Fatalf("integration_version = %q, want 0.9.0", got)
	}

	changed := configurationGroupValue(t, object, "a", "0.0.354", nil)
	planned = planConfigurationGroup(ctx, t, object, state, changed, configurationGroupValue(t, object, "a", "0.0.354", tftypes.UnknownValue))
	value := stepValue(t, planned, "family", "preset", "integration_version")
	if value.IsKnown() {
		t.Fatalf("integration_version = %s, want unknown", value)
	}
}

func configurationGroupObject(ctx context.Context, t *testing.T) tftypes.Object {
	t.Helper()
	server, err := testAccProtoV6ProviderFactories["coralogix"]()
	if err != nil {
		t.Fatal(err)
	}
	schemaResp, err := server.GetProviderSchema(ctx, &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatal(err)
	}
	resourceSchema, ok := schemaResp.ResourceSchemas["coralogix_configuration_group"]
	if !ok {
		t.Fatal("coralogix_configuration_group is missing from the provider schema")
	}
	object, ok := resourceSchema.ValueType().(tftypes.Object)
	if !ok {
		t.Fatalf("schema type = %T", resourceSchema.ValueType())
	}
	return object
}

func planConfigurationGroup(ctx context.Context, t *testing.T, object tftypes.Object, prior, config, proposed tftypes.Value) tftypes.Value {
	t.Helper()
	server, err := testAccProtoV6ProviderFactories["coralogix"]()
	if err != nil {
		t.Fatal(err)
	}
	encode := func(v tftypes.Value) *tfprotov6.DynamicValue {
		t.Helper()
		dv, err := tfprotov6.NewDynamicValue(object, v)
		if err != nil {
			t.Fatal(err)
		}
		return &dv
	}
	resp, err := server.PlanResourceChange(ctx, &tfprotov6.PlanResourceChangeRequest{
		TypeName:         "coralogix_configuration_group",
		PriorState:       encode(prior),
		Config:           encode(config),
		ProposedNewState: encode(proposed),
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range resp.Diagnostics {
		if d.Severity == tfprotov6.DiagnosticSeverityError {
			t.Fatalf("%s — %s", d.Summary, d.Detail)
		}
	}
	planned, err := resp.PlannedState.Unmarshal(object)
	if err != nil {
		t.Fatal(err)
	}
	return planned
}

func configurationGroupValue(t *testing.T, root tftypes.Object, name, chartVersion string, integration any) tftypes.Value {
	t.Helper()
	familyType := root.AttributeTypes["family"].(tftypes.Object)
	presetType := familyType.AttributeTypes["preset"].(tftypes.Object)
	preset := fillObject(presetType, map[string]tftypes.Value{
		"chart_name":             tftypes.NewValue(presetType.AttributeTypes["chart_name"], "chart_name_otel_integration"),
		"chart_version":          tftypes.NewValue(presetType.AttributeTypes["chart_version"], chartVersion),
		"integration_version":    tftypes.NewValue(presetType.AttributeTypes["integration_version"], integration),
		"observability_features": tftypes.NewValue(presetType.AttributeTypes["observability_features"], "{}"),
	})
	family := fillObject(familyType, map[string]tftypes.Value{"preset": preset})
	return fillObject(root, map[string]tftypes.Value{
		"name":   tftypes.NewValue(root.AttributeTypes["name"], name),
		"family": family,
	})
}

func fillObject(typ tftypes.Object, set map[string]tftypes.Value) tftypes.Value {
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

func stringValue(t *testing.T, value tftypes.Value, steps ...string) string {
	t.Helper()
	var out string
	if err := stepValue(t, value, steps...).As(&out); err != nil {
		t.Fatal(err)
	}
	return out
}

func stepValue(t *testing.T, value tftypes.Value, steps ...string) tftypes.Value {
	t.Helper()
	for _, step := range steps {
		next, err := value.ApplyTerraform5AttributePathStep(tftypes.AttributeName(step))
		if err != nil {
			t.Fatal(err)
		}
		value = next.(tftypes.Value)
	}
	return value
}
