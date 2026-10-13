package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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
	spec := replaceOnce(t, echoedSpec(t), "              title: Replace Echo Thing Request\n              type: object\n              required: [id, name]\n",
		"              title: Replace Echo Thing Request\n              type: object\n              required: [id, name, isInternal]\n")
	err := echoedErr(t, spec, echoedOverrides(t))
	if err == nil {
		t.Fatal("the contract is eligible, want SKIP_REQUIRED_FIELD")
	}
	if report := eligibilityReport(t, err); !hasIssue(report, "SKIP_REQUIRED_FIELD", "components.schemas.EchoThing.isInternal") {
		t.Fatalf("issues = %v, want SKIP_REQUIRED_FIELD", report)
	}
}
