package flatthing

import (
	"context"
	"slices"
	"testing"

	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"example.com/iac-test-sdk/go/openapi/gen/flat_things_service"
)

func flatSchema(t *testing.T) (frameworkresource.SchemaResponse, tftypes.Type) {
	t.Helper()
	ctx := context.Background()
	r := &Resource{client: &flat_things_service.FlatThingsServiceAPIService{}}
	var schemaResp frameworkresource.SchemaResponse
	r.Schema(ctx, frameworkresource.SchemaRequest{}, &schemaResp)
	return schemaResp, schemaResp.Schema.Type().TerraformType(ctx)
}

func nullObject(typ tftypes.Type) map[string]tftypes.Value {
	values := map[string]tftypes.Value{}
	for name, attrType := range typ.(tftypes.Object).AttributeTypes {
		values[name] = tftypes.NewValue(attrType, nil)
	}
	return values
}

// criteriaValue is the criteria attribute with the flags of the accepted criteria.
func criteriaValue(root tftypes.Type, flags string) tftypes.Value {
	criteriaType := root.(tftypes.Object).AttributeTypes["criteria"]
	criterionType := criteriaType.(tftypes.Object).AttributeTypes["accepted"]
	criterion := func(flags string) tftypes.Value {
		values := nullObject(criterionType)
		values["flags"] = tftypes.NewValue(tftypes.String, flags)
		values["texts"] = tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, []tftypes.Value{})
		return tftypes.NewValue(criterionType, values)
	}
	return tftypes.NewValue(criteriaType, map[string]tftypes.Value{"accepted": criterion(flags), "refused": criterion("")})
}

func flatValue(root tftypes.Type, name, flags string) tftypes.Value {
	values := nullObject(root)
	values["id"] = tftypes.NewValue(tftypes.String, "1")
	values["name"] = tftypes.NewValue(tftypes.String, name)
	values["kind"] = tftypes.NewValue(tftypes.String, "small")
	values["instructions"] = tftypes.NewValue(tftypes.String, "i")
	values["description"] = tftypes.NewValue(tftypes.String, "")
	values["verbose"] = tftypes.NewValue(tftypes.Bool, false)
	values["criteria"] = criteriaValue(root, flags)
	return tftypes.NewValue(root, values)
}

func maskOf(t *testing.T, state, plan tftypes.Value) []string {
	t.Helper()
	schemaResp, _ := flatSchema(t)
	mask, diags := updateMask(context.Background(),
		tfsdk.Config{Schema: schemaResp.Schema, Raw: plan},
		tfsdk.Plan{Schema: schemaResp.Schema, Raw: plan},
		tfsdk.State{Schema: schemaResp.Schema, Raw: state})
	if diags.HasError() {
		t.Fatal(diags)
	}
	return mask
}

// A change of criteria names the three fields that it stands for, which no attribute shows.
func TestCriteriaChangeNamesItsMaskPaths(t *testing.T) {
	_, root := flatSchema(t)
	got := maskOf(t, flatValue(root, "a", ""), flatValue(root, "a", "polite"))
	if want := []string{"allowed", "denied", "samples"}; !slices.Equal(got, want) {
		t.Errorf("mask = %v, want %v", got, want)
	}
}

// A change of the name alone names only the name: the hook still sends the criteria in the body,
// and the server leaves the fields that the mask does not name.
func TestNameChangeLeavesCriteriaOutOfTheMask(t *testing.T) {
	_, root := flatSchema(t)
	got := maskOf(t, flatValue(root, "a", "polite"), flatValue(root, "b", "polite"))
	if want := []string{"name"}; !slices.Equal(got, want) {
		t.Errorf("mask = %v, want %v", got, want)
	}
}

// Nothing changed: no mask, so Update sends no request.
func TestNoChangeHasNoMask(t *testing.T) {
	_, root := flatSchema(t)
	if got := maskOf(t, flatValue(root, "a", "polite"), flatValue(root, "a", "polite")); len(got) != 0 {
		t.Errorf("mask = %v, want none", got)
	}
}
