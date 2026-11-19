package filterthing

import (
	"context"
	"testing"

	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"example.com/iac-test-sdk/go/openapi/gen/filter_things_service"
)

func keepUnsetType(t *testing.T) tftypes.Object {
	t.Helper()
	r := &Resource{client: &filter_things_service.FilterThingsServiceAPIService{}}
	var resp frameworkresource.SchemaResponse
	r.Schema(context.Background(), frameworkresource.SchemaRequest{}, &resp)
	return resp.Schema.Type().TerraformType(context.Background()).(tftypes.Object)
}

func keepUnsetValue(typ tftypes.Object, set map[string]tftypes.Value) tftypes.Value {
	values := map[string]tftypes.Value{}
	for name, attr := range typ.AttributeTypes {
		values[name] = tftypes.NewValue(attr, nil)
	}
	for name, v := range set {
		values[name] = v
	}
	return tftypes.NewValue(typ, values)
}

// With the keepUnsetComputedState line, the generated ModifyPlan plans no change when the plan
// differs from the state only in unknown values that the configuration does not set.
func TestOnlyUnsetUnknowns(t *testing.T) {
	typ := keepUnsetType(t)
	str := func(s string) tftypes.Value { return tftypes.NewValue(tftypes.String, s) }
	unknown := tftypes.NewValue(tftypes.String, tftypes.UnknownValue)
	state := keepUnsetValue(typ, map[string]tftypes.Value{"id": str("1"), "name": str("a")})

	tests := []struct {
		name               string
		plan, config, want any
	}{
		{"an unset computed value is the only difference",
			keepUnsetValue(typ, map[string]tftypes.Value{"id": unknown, "name": str("a")}),
			keepUnsetValue(typ, map[string]tftypes.Value{"name": str("a")}), true},
		{"a configured value changed",
			keepUnsetValue(typ, map[string]tftypes.Value{"id": unknown, "name": str("b")}),
			keepUnsetValue(typ, map[string]tftypes.Value{"name": str("b")}), false},
		{"an unknown value that the configuration sets stays a change",
			keepUnsetValue(typ, map[string]tftypes.Value{"id": str("1"), "name": unknown}),
			keepUnsetValue(typ, map[string]tftypes.Value{"name": unknown}), false},
	}
	for _, test := range tests {
		got := onlyUnsetUnknowns(test.plan.(tftypes.Value), test.config.(tftypes.Value), state)
		if got != test.want {
			t.Errorf("%s: onlyUnsetUnknowns = %v, want %v", test.name, got, test.want)
		}
	}
}
