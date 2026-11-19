package generator

import (
	"testing"

	"github.com/coralogix/coralogix-management-sdk/iac/model"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/overrides"
)

// The plan modifier of a server default compares the default with the state, and the state holds
// the Terraform spelling of an enum that the overrides map. The default must use that spelling.
func TestServerDefaultOfAMappedEnumUsesTheTerraformSpelling(t *testing.T) {
	enum := &model.Type{
		Kind:     model.Enum,
		Schema:   "Strategy",
		Values:   []string{"STRATEGY_UNSPECIFIED", "STRATEGY_GOOD", "STRATEGY_UNCOUNTED"},
		EnumZero: "STRATEGY_UNSPECIFIED",
	}
	file := &overrides.File{Enums: map[string]overrides.Enum{
		"Strategy": {Zero: "unspecified", TrimPrefix: "STRATEGY_"},
	}}
	for _, test := range []struct{ name, api, want string }{
		{"a value with a trimmed prefix", "STRATEGY_UNCOUNTED", `types.StringValue("uncounted")`},
		{"the zero value", "STRATEGY_UNSPECIFIED", `types.StringValue("unspecified")`},
	} {
		got, err := serverDefaultValueFor(enum, test.api, file, "Thing", "strategy")
		if err != nil || got != test.want {
			t.Errorf("%s: got %q, %v, want %q", test.name, got, err, test.want)
		}
	}
	if _, err := serverDefaultValueFor(enum, "STRATEGY_OTHER", file, "Thing", "strategy"); err == nil {
		t.Error("a default that is not a value of the enum must be an error")
	}

	// With no line under enums the API spelling stays, as the state holds it.
	got, err := serverDefaultValueFor(enum, "STRATEGY_UNCOUNTED", &overrides.File{}, "Thing", "strategy")
	if err != nil || got != `types.StringValue("STRATEGY_UNCOUNTED")` {
		t.Errorf("an enum without a mapping: got %q, %v", got, err)
	}
	got, err = serverDefaultValueFor(enum, "STRATEGY_UNCOUNTED", nil, "Thing", "strategy")
	if err != nil || got != `types.StringValue("STRATEGY_UNCOUNTED")` {
		t.Errorf("a new resource: got %q, %v", got, err)
	}
}
