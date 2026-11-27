package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coralogix/coralogix-management-sdk/iac/source"
)

// twinThingInput is the synthetic input with the SDK package of testdata/twinthing_sdk.go.txt in
// place of the parsing SDK, and a spec in which the twin arms of a fold have other fields.
func twinThingInput(t *testing.T, edit func(spec string) string) (source.Input, string) {
	t.Helper()
	input, loadDir := copiedSyntheticInput(t)
	sdk, err := os.ReadFile(filepath.Join("testdata", "twinthing_sdk.go.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(input.SDKDir, "go", "openapi", "gen", "parse_things_service", "parse.go"), sdk, 0o644); err != nil {
		t.Fatal(err)
	}
	spec, err := os.ReadFile(filepath.Join("testdata", "twinthing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	input.OpenAPI = []byte(edit(string(spec)))
	return input, loadDir
}

func generateTwinThing(t *testing.T, input source.Input, loadDir string) (string, error) {
	t.Helper()
	out := filepath.Join(t.TempDir(), "parsething")
	options := Options{Resource: "ParseThing", OutputDir: out, OverridesPath: filepath.Join("testdata", "twinthing-overrides.yaml")}
	return out, generateFromInput(options, input, loadDir)
}

func keep(spec string) string { return spec }

// A fold of twin arms with other fields shows the union of the fields on the arm that holds the
// fold: a field that only the shown arm has, and a field that only the folded arm has. The
// generated resource runs: see testdata/twinthing_semantics_test.go.
func TestTwinThingFoldShowsTheUnionOfTheFields(t *testing.T) {
	input, loadDir := twinThingInput(t, keep)
	out, err := generateTwinThing(t, input, loadDir)
	if err != nil {
		t.Fatal(err)
	}
	schema := readGenerated(t, out, "schema.go")
	for _, want := range []string{`"label"`, `"limits"`, `"reason"`, `"threshold"`, `"extra"`, `"keep_matching"`} {
		if !strings.Contains(schema, want) {
			t.Errorf("the schema lacks %s", want)
		}
	}
	semantics, err := os.ReadFile(filepath.Join("testdata", "twinthing_semantics_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "semantics_test.go"), semantics, 0o644); err != nil {
		t.Fatal(err)
	}
	compileGenerated(t, out, input)
}

// Twin arms that cannot fold are refused: a field with two types is a real conflict, and arms
// without a common field are no twins.
func TestTwinThingFoldRefusesArmsThatAreNoTwins(t *testing.T) {
	cases := map[string]func(string) string{
		"a field with two types": func(spec string) string {
			return strings.Replace(spec, "    ParseDropParams:\n      type: object\n      properties:\n        rule: {type: string,", "    ParseDropParams:\n      type: object\n      properties:\n        rule: {type: string, format: byte,", 1)
		},
		"no common field": func(spec string) string {
			spec = strings.Replace(spec, "        rule: {type: string, x-coralogix-presence: true}\n        keepLogs: {type: boolean, x-coralogix-presence: true}\n        reason:", "        reason:", 1)
			return spec
		},
	}
	for name, edit := range cases {
		t.Run(name, func(t *testing.T) {
			input, loadDir := twinThingInput(t, edit)
			_, err := generateTwinThing(t, input, loadDir)
			if err == nil || !strings.Contains(err.Error(), "the arms dropParams and keepParams need the same fields") {
				t.Fatalf("got %v, want the refusal of the fold", err)
			}
		})
	}
}

// A field that only one twin has is optional, also when the contract requires it: the flag may
// select the twin that lacks it.
func TestTwinThingFoldMakesAFieldOfOneTwinOptional(t *testing.T) {
	input, loadDir := twinThingInput(t, func(spec string) string {
		spec = strings.Replace(spec, "    ParseKeepParams:\n      type: object\n", "    ParseKeepParams:\n      type: object\n      required: [label]\n", 1)
		spec = strings.Replace(spec, "    ParseDropParams:\n      type: object\n", "    ParseDropParams:\n      type: object\n      required: [reason]\n", 1)
		spec = strings.Replace(spec, "label: {type: string,", "label: {type: string, minLength: 1,", 1)
		return strings.Replace(spec, "reason: {type: string,", "reason: {type: string, minLength: 1,", 1)
	})
	out, err := generateTwinThing(t, input, loadDir)
	if err != nil {
		t.Fatal(err)
	}
	schema := readGenerated(t, out, "schema.go")
	for _, name := range []string{"label", "reason"} {
		i := strings.Index(schema, `"`+name+`": schema.StringAttribute{`)
		if i < 0 || !strings.Contains(schema[i:i+200], "Optional:") || strings.Contains(schema[i:i+200], "Required:") {
			t.Errorf("%s is not optional: %.200s", name, schema[max(i, 0):])
		}
	}
}
