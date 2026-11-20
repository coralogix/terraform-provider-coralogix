package maskthing

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"example.com/iac-test-sdk/go/openapi/gen/mask_things_service"
)

func rawOf(t *testing.T, m MaskThingModel) tftypes.Value {
	t.Helper()
	ctx := context.Background()
	s := Schema()
	plan := tfsdk.Plan{Schema: s, Raw: tftypes.NewValue(s.Type().TerraformType(ctx), nil)}
	if diags := plan.Set(ctx, &m); diags.HasError() {
		t.Fatal(diags)
	}
	return plan.Raw
}

// update runs the generated updateRequest for a change from state to plan. The configuration is the
// plan with no value for the attribute that only the server sets.
func update(t *testing.T, state, plan MaskThingModel) (*mask_things_service.UpdateMaskThingRequest, string) {
	t.Helper()
	s := Schema()
	config := plan
	if plan.Limits != nil {
		limits := *plan.Limits
		limits.Applied = types.Int32Null()
		config.Limits = &limits
	}
	planRaw := rawOf(t, plan)
	body, mask, diags := updateRequest(context.Background(),
		tfsdk.Config{Schema: s, Raw: rawOf(t, config)},
		tfsdk.Plan{Schema: s, Raw: planRaw},
		tfsdk.State{Schema: s, Raw: rawOf(t, state)})
	if diags.HasError() {
		t.Fatal(diags)
	}
	return body, mask
}

func fixedRange(end string) *MaskThingRangeModel {
	return &MaskThingRangeModel{Fixed: &MaskThingFixedModel{Start: types.StringValue("a"), End: types.StringValue(end)}}
}

func filterSet(t *testing.T, filters ...MaskThingFilterModel) types.Set {
	t.Helper()
	elemType := types.ObjectType{AttrTypes: maskThingFilterAttrTypes()}
	if len(filters) == 0 {
		return types.SetNull(elemType)
	}
	var elems []attr.Value
	for _, f := range filters {
		obj, diags := types.ObjectValueFrom(context.Background(), maskThingFilterAttrTypes(), f)
		if diags.HasError() {
			t.Fatal(diags)
		}
		elems = append(elems, obj)
	}
	set, diags := types.SetValue(elemType, elems)
	if diags.HasError() {
		t.Fatal(diags)
	}
	return set
}

func baseThing() MaskThingModel {
	return MaskThingModel{
		CreateTime:  types.StringValue("2030-01-01T00:00:00Z"),
		Owner:       types.StringNull(),
		Filters:     types.SetNull(types.ObjectType{AttrTypes: maskThingFilterAttrTypes()}),
		Id:          types.StringValue("1"),
		Name:        types.StringValue("n"),
		Description: types.StringValue("d"),
		Range:       fixedRange("b"),
		Limits:      &MaskThingLimitsModel{Max: types.Int32Value(5), Min: types.Int32Null(), Applied: types.Int32Value(7)},
	}
}

// The API replaces the whole value of each field that the mask names, so a switch of the arm names
// the field "range", not an arm path. The body holds the new arm and not the old one.
func TestArmSwitchNamesTheTopLevelField(t *testing.T) {
	plan := baseThing()
	plan.Range = &MaskThingRangeModel{Relative: &MaskThingRelativeModel{Minutes: types.Int32Value(15)}}
	body, mask := update(t, baseThing(), plan)
	if mask != "range" {
		t.Fatalf("mask = %q, want range", mask)
	}
	if body.Range == nil || body.Range.Relative == nil || body.Range.Relative.Minutes != 15 {
		t.Errorf("body.Range = %+v, want the relative arm with 15 minutes", body.Range)
	}
	if body.Range.Fixed != nil {
		t.Errorf("body.Range.Fixed = %+v, want nil: the old arm leaves with the old value", body.Range.Fixed)
	}
}

// A change inside an arm names the field too. The body holds the whole value of the field,
// including the values that did not change.
func TestChangeInsideAnArmSendsTheWholeField(t *testing.T) {
	plan := baseThing()
	plan.Range = fixedRange("c")
	body, mask := update(t, baseThing(), plan)
	if mask != "range" {
		t.Fatalf("mask = %q, want range", mask)
	}
	if body.Range == nil || body.Range.Fixed == nil || body.Range.Fixed.Start != "a" || body.Range.Fixed.End != "c" {
		t.Errorf("body.Range = %+v, want fixed a..c", body.Range)
	}
}

// Removing an optional object names the field and sends no value.
func TestRemovedObjectNamesTheFieldWithNoValue(t *testing.T) {
	plan := baseThing()
	plan.Limits = nil
	body, mask := update(t, baseThing(), plan)
	if mask != "limits" {
		t.Fatalf("mask = %q, want limits", mask)
	}
	if body.Limits != nil {
		t.Errorf("body.Limits = %+v, want nil", body.Limits)
	}
}

// Removing an optional scalar names the field and sends no value.
func TestRemovedScalarNamesTheFieldWithNoValue(t *testing.T) {
	plan := baseThing()
	plan.Description = types.StringNull()
	body, mask := update(t, baseThing(), plan)
	if mask != "description" {
		t.Fatalf("mask = %q, want description", mask)
	}
	if body.Description != nil {
		t.Errorf("body.Description = %q, want nil", *body.Description)
	}
}

// A value that only the server sets is unknown in the plan when another field changes. It is no
// part of the request, so it does not put the object in the mask.
func TestServerValueInsideTheObjectIsNotAChange(t *testing.T) {
	plan := baseThing()
	plan.Name = types.StringValue("renamed")
	plan.Limits = &MaskThingLimitsModel{Max: types.Int32Value(5), Min: types.Int32Null(), Applied: types.Int32Unknown()}
	_, mask := update(t, baseThing(), plan)
	if mask != "name" {
		t.Errorf("mask = %q, want name", mask)
	}
}

// A set of objects is replaced whole too. A new element names the field "filters", and the body holds
// every element, the old ones and the new one.
func TestAddedSetElementSendsTheWholeSet(t *testing.T) {
	stringFilter := MaskThingFilterModel{StringFilter: &MaskThingStringFilterModel{Field: types.StringValue("platform"), Value: types.StringValue("web")}}
	boolFilter := MaskThingFilterModel{BoolFilter: &MaskThingBoolFilterModel{Value: types.BoolValue(true)}}
	state, plan := baseThing(), baseThing()
	state.Filters = filterSet(t, stringFilter)
	plan.Filters = filterSet(t, stringFilter, boolFilter)
	body, mask := update(t, state, plan)
	if mask != "filters" {
		t.Fatalf("mask = %q, want filters", mask)
	}
	if len(body.Filters) != 2 {
		t.Errorf("body.Filters has %d elements, want 2", len(body.Filters))
	}
}

// Nothing changed: no mask, so Update sends no request.
func TestNoChangeHasNoMask(t *testing.T) {
	body, mask := update(t, baseThing(), baseThing())
	if body != nil || mask != "" {
		t.Errorf("body = %+v, mask = %q, want none", body, mask)
	}
}
