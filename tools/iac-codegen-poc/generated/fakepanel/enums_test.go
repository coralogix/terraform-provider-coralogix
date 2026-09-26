// This file is handwritten. It uses the generated enum names (F54) as a
// handwritten resource would: a validator from the names, and the maps in
// both directions.

package fakepanel_test

import (
	"context"
	"slices"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/generated/fakepanel"
	sdk "github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/testsdk/go/openapi/gen/fake_boards_service"
)

func TestEnumMaps(t *testing.T) {
	if !slices.Equal(fakepanel.OrientationNames, []string{"vertical", "horizontal"}) {
		t.Errorf("names = %v", fakepanel.OrientationNames)
	}
	for _, name := range fakepanel.OrientationNames {
		v, ok := fakepanel.OrientationByName[name]
		if !ok {
			t.Fatalf("%s: not in ByName", name)
		}
		if back, ok := fakepanel.OrientationName(v); !ok || back != name {
			t.Errorf("%s → %s → %q, %t", name, v, back, ok)
		}
	}
	if v := fakepanel.ComparisonByName["more_than"]; v != sdk.COMPARISON_COMPARISON_MORE_THAN_OR_UNSPECIFIED {
		t.Errorf("more_than = %s", v)
	}
	if _, ok := fakepanel.DeliveryName(sdk.DELIVERY_DELIVERY_UNSPECIFIED); ok {
		t.Error("the value that only means \"not set\" has a name")
	}
	if _, ok := fakepanel.DeliveryName(sdk.Delivery("NEW_VALUE")); ok {
		t.Error("an unknown value has a name")
	}
}

// TestEnumValidator checks a handwritten validator made from the names.
func TestEnumValidator(t *testing.T) {
	v := stringvalidator.OneOf(fakepanel.OrientationNames...)
	for value, wantErr := range map[string]bool{"vertical": false, "horizontal": false, "VERTICAL": true, "diagonal": true} {
		req := validator.StringRequest{Path: path.Root("orientation"), ConfigValue: types.StringValue(value)}
		var resp validator.StringResponse
		v.ValidateString(context.Background(), req, &resp)
		if resp.Diagnostics.HasError() != wantErr {
			t.Errorf("%s: error %t, want %t", value, resp.Diagnostics.HasError(), wantErr)
		}
	}
}
