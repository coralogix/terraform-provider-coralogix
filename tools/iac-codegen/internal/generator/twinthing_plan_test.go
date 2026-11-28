package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withFoldDefaults gives one field of each twin of testdata/twinthing.yaml a contract default: label
// belongs to the shown twin only, and threshold to the folded twin only. Neither default is a zero
// value, so each field is Optional and Computed with the server-default plan modifier.
func withFoldDefaults(spec string) string {
	spec = strings.Replace(spec, "label: {type: string,", "label: {type: string, default: standard,", 1)
	return strings.Replace(spec, "threshold: {type: integer, format: int64,", "threshold: {type: integer, format: int64, default: 5,", 1)
}

// A computed field that only one twin of a fold has is planned null while the flag selects the other
// twin: the state or the server default would else reach the expand as a field of the twin that the
// request does not send. The generated plan runs: see testdata/twinthing_plan_semantics_test.go.
func TestTwinThingFoldPlansTheFieldsOfTheOtherTwinNull(t *testing.T) {
	input, loadDir := twinThingInput(t, withFoldDefaults)
	out, err := generateTwinThing(t, input, loadDir)
	if err != nil {
		t.Fatal(err)
	}
	schema := readGenerated(t, out, "schema.go")
	for _, want := range []string{
		`serverDefaultModifier{value: types.StringValue("standard")}, foldSideModifier{flag: "keep_matching", when: true}`,
		`serverDefaultModifier{value: types.Int64Value(5)}, foldSideModifier{flag: "keep_matching", when: false}`,
	} {
		if !strings.Contains(strings.Join(strings.Fields(schema), " "), strings.Join(strings.Fields(want), " ")) {
			t.Errorf("the schema lacks %s", want)
		}
	}
	semantics, err := os.ReadFile(filepath.Join("testdata", "twinthing_plan_semantics_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "plan_semantics_test.go"), semantics, 0o644); err != nil {
		t.Fatal(err)
	}
	compileGenerated(t, out, input)
}

// Plain optional fields of one twin keep no value in state, so they take no modifier, and a fold of
// twins that have the same fields has no field to plan.
func TestTwinThingFoldWithoutComputedFieldsHasNoSideModifier(t *testing.T) {
	input, loadDir := twinThingInput(t, keep)
	out, err := generateTwinThing(t, input, loadDir)
	if err != nil {
		t.Fatal(err)
	}
	if schema := readGenerated(t, out, "schema.go"); strings.Contains(schema, "foldSideModifier") {
		t.Error("the schema has a foldSideModifier, but no field of one twin is computed")
	}
}

// The same plan inside the items of a set: the steps are a set, and the fields of each twin are
// computed by an overrides line. The modifier finds the flag of its own item by the planned item
// in the path, not by the index.
func TestTwinThingFoldPlansTheFieldsOfTheOtherTwinNullInSetItems(t *testing.T) {
	input, loadDir := twinThingInput(t, keep)
	out := filepath.Join(t.TempDir(), "parsething")
	options := Options{Resource: "ParseThing", OutputDir: out, OverridesPath: filepath.Join("testdata", "twinthing-set-overrides.yaml")}
	if err := generateFromInput(options, input, loadDir); err != nil {
		t.Fatal(err)
	}
	schema := readGenerated(t, out, "schema.go")
	if !strings.Contains(schema, `"steps": schema.SetNestedAttribute{`) || !strings.Contains(schema, "foldSideModifier{") {
		t.Fatalf("the schema has no set of steps with a foldSideModifier:\n%s", schema)
	}
	semantics, err := os.ReadFile(filepath.Join("testdata", "twinthing_plan_semantics_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "plan_semantics_test.go"), semantics, 0o644); err != nil {
		t.Fatal(err)
	}
	compileGenerated(t, out, input)
}
