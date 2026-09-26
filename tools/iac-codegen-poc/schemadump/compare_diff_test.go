package schemadump_test

import (
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/schemadump"
)

// TestCompare checks the kind of each difference, and which ones break
// users (D21).
func TestCompare(t *testing.T) {
	oneOf := func(v ...string) []validator.String { return []validator.String{stringvalidator.OneOf(v...)} }
	hw := map[string]schema.Attribute{
		"same":      schema.StringAttribute{Optional: true},
		"gone":      schema.SingleNestedAttribute{Optional: true, Attributes: map[string]schema.Attribute{"x": schema.StringAttribute{Optional: true}}},
		"typed":     schema.ListAttribute{Optional: true, ElementType: types.StringType},
		"loose":     schema.StringAttribute{Required: true},
		"computed":  schema.StringAttribute{Optional: true, Computed: true},
		"defaulted": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false)},
		"kept": schema.StringAttribute{Computed: true,
			PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
		"gained": schema.StringAttribute{Computed: true},
		"replaced": schema.StringAttribute{Optional: true,
			PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
		"limited": schema.StringAttribute{Optional: true},
		"enum":    schema.StringAttribute{Optional: true, Validators: oneOf("a", "b")},
		"grown":   schema.StringAttribute{Optional: true, Validators: oneOf("a")},
		"nulled": schema.ListAttribute{Optional: true, Computed: true, ElementType: types.StringType,
			Default: listdefault.StaticValue(types.ListNull(types.StringType))},
	}
	gen := map[string]schema.Attribute{
		"same":      schema.StringAttribute{Optional: true},
		"typed":     schema.SetAttribute{Optional: true, ElementType: types.StringType},
		"loose":     schema.StringAttribute{Optional: true},
		"computed":  schema.StringAttribute{Optional: true},
		"defaulted": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true)},
		"kept":      schema.StringAttribute{Computed: true},
		"gained": schema.StringAttribute{Computed: true,
			PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
		"replaced": schema.StringAttribute{Optional: true},
		"limited":  schema.StringAttribute{Optional: true, Validators: []validator.String{stringvalidator.LengthAtMost(5)}},
		"enum":     schema.StringAttribute{Optional: true, Validators: oneOf("A", "B")},
		"grown":    schema.StringAttribute{Optional: true, Validators: oneOf("a", "c")},
		"new":      schema.StringAttribute{Optional: true},
		"needed":   schema.StringAttribute{Required: true},
		"nulled":   schema.ListAttribute{Optional: true, ElementType: types.StringType},
	}
	dump := func(a map[string]schema.Attribute) []schemadump.Entry {
		return schemadump.Dump(schema.Schema{Attributes: a}, nil)
	}
	got := map[string]bool{} // "path kind" → breaking
	for _, d := range schemadump.Compare(dump(hw), dump(gen)) {
		key := d.Path + " " + d.Kind
		if d.Kind == schemadump.KindValidator {
			key += " " + strings.Fields(d.Detail)[0]
		}
		got[key] = d.Breaking
	}
	want := map[string]bool{
		"gone only in handwritten": true, // gone.x is not listed on its own
		"typed type":               true,
		"loose flags":              false,
		"computed flags":           true,
		"defaulted default":        true,
		"kept plan modifier":       true, // a lost UseStateForUnknown shows "known after apply"
		"gained plan modifier":     false,
		"replaced plan modifier":   true,
		"limited validator +":      false,
		"enum validator values":    true,
		"enum validator new":       false,
		"grown validator new":      false,
		"new only in generated":    false,
		"needed only in generated": true,
		"nulled default":           false, // a null default: the same as optional
	}
	for k, b := range want {
		if g, ok := got[k]; !ok || g != b {
			t.Errorf("%s: breaking %v (found %v), want %v", k, g, ok, b)
		}
	}
	for k := range got {
		if _, ok := want[k]; !ok {
			t.Errorf("unexpected difference %q", k)
		}
	}
}
