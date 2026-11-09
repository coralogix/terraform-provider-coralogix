package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coralogix/coralogix-management-sdk/iac/model"
)

// The golden output of a released resource whose API holds every field in one object, with the
// arms of a oneOf next to common fields, and does not require that object in the requests or the
// response. The object is inlined, so its fields and its arms are top-level attributes, and the
// arms form one group.
func TestGoldenOutputUnionWrapThing(t *testing.T) {
	out := generateUnionWrapThing(t)
	golden := filepath.Join("testdata", "golden", "unionwrapthing")
	if *updateGolden {
		if err := os.RemoveAll(golden); err != nil {
			t.Fatal(err)
		}
		if err := copyDirectory(out, golden); err != nil {
			t.Fatal(err)
		}
	}
	if diff := compareDirectories(golden, out); diff != "" {
		t.Fatalf("golden output differs; run go test ./internal/generator -run TestGoldenOutputUnionWrapThing -update:\n%s", diff)
	}
}

// Create and Update send the inlined object with its fields and the selected arm, and Create reads
// the object back. A response without the object has no resource.
func TestUnionWrapThingRuntimeSemantics(t *testing.T) {
	input, _ := syntheticInput(t)
	out := generateUnionWrapThing(t)
	semantics, err := os.ReadFile(filepath.Join("testdata", "unionwrapthing_semantics_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "semantics_test.go"), semantics, 0o644); err != nil {
		t.Fatal(err)
	}
	compileGenerated(t, out, input)
}

func generateUnionWrapThing(t *testing.T) string {
	t.Helper()
	spec, err := os.ReadFile(filepath.Join("testdata", "unionwrapthing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	input, loadDir := syntheticInput(t)
	input.OpenAPI = spec
	out := filepath.Join(t.TempDir(), "unionwrapthing")
	options := Options{Resource: "UnionWrapThing", OutputDir: out, OverridesPath: filepath.Join("testdata", "unionwrapthing-overrides.yaml")}
	if err := generateFromInput(options, input, loadDir); err != nil {
		t.Fatal(err)
	}
	return out
}

func unionWrapThingInputs(t *testing.T) (spec, overrides string) {
	t.Helper()
	specData, err := os.ReadFile(filepath.Join("testdata", "unionwrapthing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	overridesData, err := os.ReadFile(filepath.Join("testdata", "unionwrapthing-overrides.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return string(specData), string(overridesData)
}

// Each of the schemas that the inline line covers may or may not require the object. The line
// states required: true for the ones that do not, and is stale when every one of them does. A
// oneOf joins the group of the schema that receives its fields only when that schema has none.
func TestInlineOfAnObjectWithAOneOf(t *testing.T) {
	const createTitle = "title: Create Union Wrap Thing Request\n              type: object\n"
	const replaceTitle = "title: Replace Union Wrap Thing Request\n              type: object\n"
	const resource = "    UnionWrapThing:\n      type: object\n"
	requireIn := func(old string) func(t *testing.T, s, o string) (string, string) {
		return func(t *testing.T, s, o string) (string, string) {
			return replaceOnce(t, s, old, old+"              required: [thing]\n"), o
		}
	}
	requireAll := func(t *testing.T, s, o string) (string, string) {
		s, o = requireIn(createTitle)(t, s, o)
		s, o = requireIn(replaceTitle)(t, s, o)
		return replaceOnce(t, s, resource, resource+"      required: [thing]\n"), o
	}
	tests := map[string]struct {
		change         func(t *testing.T, spec, overrides string) (string, string)
		code, location string // empty when the contract is eligible
		message        string
	}{
		"the line states the object as required": {},
		"no required line": {
			change: func(t *testing.T, s, o string) (string, string) {
				return s, replaceOnce(t, o, "{inline: true, required: true}", "{inline: true}")
			},
			code: "INLINE_UNSUPPORTED", location: "components.schemas.UnionWrapThing", message: "add required: true to the inline line of thing",
		},
		"the contract requires the object everywhere": {
			change: requireAll,
			code:   "OVERRIDE_UNUSED", location: "behavior-overrides.yaml:types.UnionWrapThing.fields.thing",
		},
		"the contract requires the object in the response only": {
			change: func(t *testing.T, s, o string) (string, string) {
				return replaceOnce(t, s, resource, resource+"      required: [thing]\n"), o
			},
		},
		"the contract requires the object in the requests only": {
			change: func(t *testing.T, s, o string) (string, string) {
				s, o = requireIn(createTitle)(t, s, o)
				return requireIn(replaceTitle)(t, s, o)
			},
		},
		"the schema that receives the fields has a oneOf": {
			change: func(t *testing.T, s, o string) (string, string) {
				return replaceOnce(t, s, resource, resource+"      oneOf: [{required: [id]}]\n"), o
			},
			code: "INLINE_UNSUPPORTED", location: "components.schemas.UnionWrapThing", message: "has a group already",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			spec, overrides := unionWrapThingInputs(t)
			if test.change != nil {
				spec, overrides = test.change(t, spec, overrides)
			}
			_, err := validateOpenAPIWith([]byte(spec), "UnionWrapThing", model.OperationIDs{}, "sdk", "provider", mustParse(t, overrides))
			if test.code == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			report := eligibilityReport(t, err)
			if !hasIssue(report, test.code, test.location) {
				t.Fatalf("report = %v, want %s at %s", report, test.code, test.location)
			}
			if !strings.Contains(fmt.Sprint(report), test.message) {
				t.Fatalf("report = %v, want the message %q", report, test.message)
			}
		})
	}
}

// requestValueBy reaches an object of a group of the resource through the group. An object of a group
// of types.<Type>.groups is in the object of its component, not in the model of the request, so the
// line is refused there.
func TestRequestValueByIsRefusedForAnObjectInANestedGroup(t *testing.T) {
	cf := &convField{TFName: "variant", RequestValueBy: []fixedBy{{Model: "ArmA", Value: "X", NestedGroup: true}}}
	err := (&convBuilder{}).applyFixedRequest(cf)
	if err == nil || !strings.Contains(err.Error(), "which is in a group of types.<Type>.groups") {
		t.Fatalf("err = %v, want requestValueBy refused for a field in a nested group", err)
	}
}
