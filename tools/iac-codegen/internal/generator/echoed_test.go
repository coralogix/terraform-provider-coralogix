package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/issue"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/model"
)

func echoedSpec(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "echoed.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func echoedOverrides(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "echoed-overrides.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func replaceOnce(t *testing.T, s, old, replacement string) string {
	t.Helper()
	if !strings.Contains(s, old) {
		t.Fatalf("the test input did not change: %q is missing", old)
	}
	return strings.Replace(s, old, replacement, 1)
}

func echoedErr(t *testing.T, spec, overrides string) error {
	t.Helper()
	_, err := validateOpenAPIWith([]byte(spec), "EchoThing", model.OperationIDs{}, "sdk", "provider", mustParse(t, overrides))
	return err
}

// Each part of the echoed contract needs its rule: without it, the generator refuses the contract.
func TestEchoedContractNeedsEachRule(t *testing.T) {
	tests := map[string]struct {
		spec, overrides func(t *testing.T, spec, overrides string) (string, string)
		code, location  string
	}{
		"skip of a field that Replace echoes": {
			overrides: func(t *testing.T, spec, o string) (string, string) {
				return spec, replaceOnce(t, o, "      isInternal: {skip: true}\n", "")
			},
			code: "FIELD_LIFECYCLE_UNSUPPORTED", location: "components.schemas.EchoThing.isInternal",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			spec, overrides := test.overrides(t, echoedSpec(t), echoedOverrides(t))
			err := echoedErr(t, spec, overrides)
			if err == nil {
				t.Fatal("the contract is eligible, want an issue")
			}
			if report := eligibilityReport(t, err); !hasIssue(report, test.code, test.location) {
				t.Fatalf("issues = %v, want %s at %s", report, test.code, test.location)
			}
		})
	}
}

// The responses wrap the resource as {"thing": {"allOf": [$ref]}}. The contract check reads the
// fields of the $ref; it once read the allOf schema, found no Get field, and refused every field.
func TestEchoedContractIsEligible(t *testing.T) {
	if err := echoedErr(t, echoedSpec(t), echoedOverrides(t)); err != nil {
		t.Fatal(err)
	}
}

// The generated requests never send a skipped field, so a request that requires it is refused.
func TestSkipOfARequiredRequestFieldIsRefused(t *testing.T) {
	spec := replaceOnce(t, echoedSpec(t), "              title: Replace Echo Thing Request\n              type: object\n              required: [id, name, kind]\n",
		"              title: Replace Echo Thing Request\n              type: object\n              required: [id, name, kind, isInternal]\n")
	err := echoedErr(t, spec, echoedOverrides(t))
	if err == nil {
		t.Fatal("the contract is eligible, want SKIP_REQUIRED_FIELD")
	}
	if report := eligibilityReport(t, err); !hasIssue(report, "SKIP_REQUIRED_FIELD", "components.schemas.EchoThing.isInternal") {
		t.Fatalf("issues = %v, want SKIP_REQUIRED_FIELD", report)
	}
}

func hasIssueAt(report issue.Report, code, location string) bool {
	for _, item := range report {
		if item.Code == code && strings.Contains(item.Location+" "+item.Message, location) {
			return true
		}
	}
	return false
}

// Each line for the older shapes of the echoed contract is needed: without it, the generator
// refuses the contract with the issue that the line answers.
func TestEchoedContractNeedsEachLine(t *testing.T) {
	tests := map[string]struct {
		line, code, location string
		without              string // the line without the key, when the line has other keys
	}{
		"createAs folds the flat Create field":               {"      quota: {createAs: {limit: limitValue}, computed: true}\n", "FIELD_LIFECYCLE_UNSUPPORTED", "EchoThing.quota", "      quota: {computed: true}\n"},
		"readOnly on a server flag of a shared component":    {"      exceeded: {readOnly: true, name: has_exceeded}\n", "FIELD_LIFECYCLE_UNSUPPORTED", "quota.exceeded", "      exceeded: {name: has_exceeded}\n"},
		"computed keeps a value Update requires":             {"      limit: {computed: true, validators: [{atLeast: 0}]}\n", "FIELD_REQUIREDNESS_UNSUPPORTED", "quota.limit", "      limit: {validators: [{atLeast: 0}]}\n"},
		"optional relaxes a required list that may be empty": {"      values: {optional: true, collection: set, name: tags}\n", "REQUIRED_VALUE_CAN_BE_EMPTY", "values", "      values: {collection: set, name: tags}\n"},
		"computed states the presence of a plain bool":       {"      enabled: {computed: true, name: enable}\n", "FIELD_PRESENCE_UNKNOWN", "enabled", "      enabled: {name: enable}\n"},
		"skip of an empty oneOf marker":                      {"      none: {skip: true}\n", "RENDERER_SHAPE_UNSUPPORTED", "none", ""},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			err := echoedErr(t, echoedSpec(t), replaceOnce(t, echoedOverrides(t), test.line, test.without))
			if err == nil {
				t.Fatal("the contract is eligible, want an issue")
			}
			if report := eligibilityReport(t, err); !hasIssueAt(report, test.code, test.location) {
				t.Fatalf("issues = %v, want %s at %s", report, test.code, test.location)
			}
		})
	}
}

// A line that names what the contract does not have, or that would change what a oneOf means,
// is refused.
func TestEchoedContractRefusesWrongLines(t *testing.T) {
	tests := map[string]struct {
		spec      func(t *testing.T, spec string) string
		overrides func(t *testing.T, o string) string
		code      string
		location  string
	}{
		"createAs names a field that Create does not have": {
			overrides: func(t *testing.T, o string) string {
				return replaceOnce(t, o, "{createAs: {limit: limitValue}, computed: true}", "{createAs: {limit: maxValue}, computed: true}")
			},
			code: "CREATE_AS_UNSUPPORTED", location: "maxValue",
		},
		"optional on a field that the contract does not require": {
			overrides: func(t *testing.T, o string) string {
				return replaceOnce(t, o, "{computed: true, name: enable}", "{computed: true, name: enable, optional: true}")
			},
			code: "OVERRIDE_UNUSED", location: "EchoItem",
		},
		"name that the JSON name already gives": {
			overrides: func(t *testing.T, o string) string {
				return replaceOnce(t, o, "name: enable}", "name: enabled}")
			},
			code: "OVERRIDE_UNUSED", location: "EchoItem",
		},
		"name of another field": {
			overrides: func(t *testing.T, o string) string {
				return replaceOnce(t, o, "name: enable}", "name: key}")
			},
			code: "OVERRIDE_UNUSED", location: "EchoItem",
		},
		"requestValueBy names a field that the component does not have": {
			overrides: func(t *testing.T, o string) string {
				return replaceOnce(t, o, "{requestValueBy: {quota: KIND_LIMITED}}", "{requestValueBy: {limits: KIND_LIMITED}}")
			},
			code: "OVERRIDE_UNUSED", location: "limits",
		},
		"requestValueBy names a field that is not an object": {
			overrides: func(t *testing.T, o string) string {
				return replaceOnce(t, o, "{requestValueBy: {quota: KIND_LIMITED}}", "{requestValueBy: {name: KIND_LIMITED}}")
			},
			code: "OVERRIDE_UNUSED", location: "not an object",
		},
		"requestValueBy sends a value that the enum does not have": {
			overrides: func(t *testing.T, o string) string {
				return replaceOnce(t, o, "{requestValueBy: {quota: KIND_LIMITED}}", "{requestValueBy: {quota: KIND_HUGE}}")
			},
			code: "OVERRIDE_UNUSED", location: "KIND_HUGE",
		},
		"keyedBy on a field that is not a list": {
			overrides: func(t *testing.T, o string) string {
				return replaceOnce(t, o, "{requestValueBy: {quota: KIND_LIMITED}}", "{requestValueBy: {quota: KIND_LIMITED}, keyedBy: {key: key}}")
			},
			code: "OVERRIDE_UNUSED", location: "list of objects",
		},
		"keyedBy with a key that is not a required string": {
			overrides: func(t *testing.T, o string) string {
				return replaceOnce(t, o, "keyedBy: {key: key}", "keyedBy: {key: enabled}")
			},
			code: "OVERRIDE_UNUSED", location: "required string",
		},
		"keyedBy with a value next to other properties": {
			overrides: func(t *testing.T, o string) string {
				return replaceOnce(t, o, "keyedBy: {key: key}", "keyedBy: {key: key, value: values}")
			},
			code: "OVERRIDE_UNUSED", location: "only the key and the value",
		},
		"atLeast on a string": {
			overrides: func(t *testing.T, o string) string {
				return replaceOnce(t, o, "          - lengthAtLeast: 1\n", "          - atLeast: 1\n")
			},
			code: "RENDERER_SHAPE_UNSUPPORTED", location: "name",
		},
		"collection: set on a field that is not a list": {
			overrides: func(t *testing.T, o string) string {
				return replaceOnce(t, o, "{computed: true, name: enable}", "{computed: true, name: enable, collection: set}")
			},
			code: "OVERRIDE_UNUSED", location: "EchoItem",
		},
		"skip of an arm that holds a value": {
			overrides: func(t *testing.T, o string) string {
				return replaceOnce(t, o, "      none: {skip: true}\n", "      none: {skip: true}\n      detail: {skip: true}\n")
			},
			code: "ONEOF_ARM_SKIPPED", location: "EchoItem.detail",
		},
		"skip of an empty arm when the group needs an arm": {
			spec: func(t *testing.T, s string) string {
				return replaceOnce(t, s, "        - {type: object, not: {anyOf: [{required: [none], type: object}, {required: [detail], type: object}]}}\n", "")
			},
			code: "ONEOF_ARM_SKIPPED", location: "EchoItem.none",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			spec, overrides := echoedSpec(t), echoedOverrides(t)
			if test.spec != nil {
				spec = test.spec(t, spec)
			}
			if test.overrides != nil {
				overrides = test.overrides(t, overrides)
			}
			err := echoedErr(t, spec, overrides)
			if err == nil {
				t.Fatal("the contract is eligible, want an issue")
			}
			if report := eligibilityReport(t, err); !hasIssueAt(report, test.code, test.location) {
				t.Fatalf("issues = %v, want %s at %s", report, test.code, test.location)
			}
		})
	}
}
