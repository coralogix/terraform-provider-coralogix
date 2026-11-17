package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func generateFlatThing(t *testing.T) string {
	t.Helper()
	out, err := generateFlatThingWith(t, nil)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// generateFlatThingWith generates flatthing with the overrides after the replacements, a list of
// old and new text pairs, and returns the output directory or the error of the generator.
func generateFlatThingWith(t *testing.T, replacements []string) (string, error) {
	t.Helper()
	spec, err := os.ReadFile(filepath.Join("testdata", "flatthing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	overrides, err := os.ReadFile(filepath.Join("testdata", "flatthing-overrides.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(overrides)
	for i := 0; i+1 < len(replacements); i += 2 {
		if !strings.Contains(text, replacements[i]) {
			t.Fatalf("the overrides do not contain %q", replacements[i])
		}
		text = strings.Replace(text, replacements[i], replacements[i+1], 1)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "overrides.yaml")
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	input, loadDir := syntheticInput(t)
	input.OpenAPI = spec
	out := filepath.Join(dir, "flatthing")
	options := Options{Resource: "FlatThing", OutputDir: out, OverridesPath: path}
	return out, generateFromInput(options, input, loadDir)
}

// The golden output of a released resource whose Create and Update bodies take the settings flat,
// while the resource holds them in an inlined object, and whose Update is a PATCH with a mask. It
// has a set of objects and an object with static defaults as extra attributes, the validators
// lengthBetween, sizeAtMost, and valuesAre on them, and an extra attribute that puts three fields
// in the update mask (maskPaths).
func TestGoldenOutputFlatThing(t *testing.T) {
	out := generateFlatThing(t)
	golden := filepath.Join("testdata", "golden", "flatthing")
	if *updateGolden {
		if err := os.RemoveAll(golden); err != nil {
			t.Fatal(err)
		}
		if err := copyDirectory(out, golden); err != nil {
			t.Fatal(err)
		}
	}
	if diff := compareDirectories(golden, out); diff != "" {
		t.Fatalf("golden output differs; run go test ./internal/generator -run TestGoldenOutputFlatThing -update:\n%s", diff)
	}
}

// The generated code runs: a change of criteria names the three fields that it stands for in the
// update mask, and a change of the name alone names only the name.
func TestFlatThingRuntimeSemantics(t *testing.T) {
	input, _ := syntheticInput(t)
	out := generateFlatThing(t)
	semantics, err := os.ReadFile(filepath.Join("testdata", "flatthing_semantics_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "semantics_test.go"), semantics, 0o644); err != nil {
		t.Fatal(err)
	}
	compileGenerated(t, out, input)
}

// The files of the overrides that the new keys reject. Each one is an error of the reader or of the
// generator, not a silent line.
func TestFlatThingNewKeysAreChecked(t *testing.T) {
	for _, c := range []struct {
		name         string
		replacements []string
		want         string
	}{
		{"a mask path that an attribute has already", []string{"maskPaths: [allowed, denied, samples]", "maskPaths: [name]"}, "needs a line with skip: true"},
		{"a mask path that is not a field of the resource", []string{"maskPaths: [allowed, denied, samples]", "maskPaths: [allowed, denied, sampless]"}, `names "sampless"`},
		{"a mask path that is no plain field name", []string{"maskPaths: [allowed, denied, samples]", "maskPaths: [allowed.x]"}, "no plain field name"},
		{"a mask path with no update mask", []string{"afterFlatten: true\n", "afterFlatten: true\nupdateMask: omit\n"}, "updateMask: omit sends none"},
		{"a set of objects on a nested type", []string{"types:\n  FlatThing:\n", "types:\n  FlatThingSample:\n    extraAttributes:\n      more:\n        type: set\n        optional: true\n        computed: true\n        default: []\n        markdownDescription: x\n        attributes:\n          n: {type: string, required: true, markdownDescription: x}\n  FlatThing:\n"}, "applies to an attribute of the resource"},
		{"a set of objects inside an object", []string{"          accepted:\n            type: object\n", "          inner:\n            type: set\n            optional: true\n            computed: true\n            default: []\n            markdownDescription: x\n            attributes:\n              n: {type: string, required: true, markdownDescription: x}\n          accepted:\n            type: object\n"}, "cannot be a set of objects"},
		{"a set default with an element", []string{"        default: []\n        validators:\n          - sizeAtMost: 1024", "        default: [x]\n        validators:\n          - sizeAtMost: 1024"}, "must be empty"},
		{"a default outside lengthBetween", []string{"lengthBetween: [0, 65536]", "lengthBetween: [1, 65536]"}, "lengthBetween"},
		{"a repeated mask path", []string{"maskPaths: [allowed, denied, samples]", "maskPaths: [allowed, allowed]"}, "empty or repeated path"},
		{"a mask path with inUpdate", []string{"maskPaths: [allowed, denied, samples]", "maskPaths: [allowed]\n        inUpdate: true"}, "cannot be combined with inUpdate"},
		{"a mask path inside an object", []string{"            markdownDescription: \"Accepted criteria.\"", "            maskPaths: [allowed]\n            markdownDescription: \"Accepted criteria.\""}, "cannot be dynamic, writeOnly, inUpdate, or have maskPaths"},
		{"a set of objects with an element type", []string{"        type: set\n        optional: true\n        computed: true\n        default: []\n        validators:\n          - sizeAtMost: 1024", "        type: set\n        elementType: string\n        optional: true\n        computed: true\n        default: []\n        validators:\n          - sizeAtMost: 1024"}, "has attributes and no elementType"},
		{"a length range in the wrong order", []string{"lengthBetween: [1, 256]", "lengthBetween: [256, 1]"}, "lengthBetween needs a minimum and a maximum length"},
		{"an element range in the wrong order", []string{"valuesAre: {lengthBetween: [1, 65536]}", "valuesAre: {lengthBetween: [65536]}"}, "valuesAre.lengthBetween needs a minimum and a maximum length"},
		{"a set size limit on a string", []string{"lengthBetween: [0, 65536]", "sizeAtMost: 5"}, "the validator does not fit a String attribute"},
		{"an object default with an unknown attribute", []string{"          accepted: {flags: \"\", texts: []}\n          refused: {flags: \"\", texts: []}", "          accepted: {flags: \"\", texts: [], nope: 1}\n          refused: {flags: \"\", texts: []}"}, "nope"},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, err := generateFlatThingWith(t, c.replacements)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("error = %v, want one that contains %q", err, c.want)
			}
		})
	}
}
