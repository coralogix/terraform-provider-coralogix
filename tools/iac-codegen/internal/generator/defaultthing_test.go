package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func generateDefaultThing(t *testing.T) string {
	t.Helper()
	spec, err := os.ReadFile(filepath.Join("testdata", "defaultthing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	input, loadDir := syntheticInput(t)
	input.OpenAPI = spec
	out := filepath.Join(t.TempDir(), "defaultthing")
	options := Options{Resource: "DefaultThing", OutputDir: out, OverridesPath: filepath.Join("testdata", "defaultthing-overrides.yaml")}
	if err := generateFromInput(options, input, loadDir); err != nil {
		t.Fatal(err)
	}
	return out
}

// The golden output of a released resource whose optional objects and lists plan a static default.
func TestGoldenOutputDefaultThing(t *testing.T) {
	out := generateDefaultThing(t)
	golden := filepath.Join("testdata", "golden", "defaultthing")
	if *updateGolden {
		if err := os.RemoveAll(golden); err != nil {
			t.Fatal(err)
		}
		if err := copyDirectory(out, golden); err != nil {
			t.Fatal(err)
		}
	}
	if diff := compareDirectories(golden, out); diff != "" {
		t.Fatalf("golden output differs; run go test ./internal/generator -run TestGoldenOutputDefaultThing -update:\n%s", diff)
	}
}

// The generated resource, run with real Terraform against a fake service that returns an omitted
// object with empty lists and an omitted list as an empty one.
func TestDefaultThingRuntimeSemantics(t *testing.T) {
	input, _ := syntheticInput(t)
	out := generateDefaultThing(t)
	semantics, err := os.ReadFile(filepath.Join("testdata", "defaultthing_semantics_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "semantics_test.go"), semantics, 0o644); err != nil {
		t.Fatal(err)
	}
	compileGenerated(t, out, input)
}

// generateDefaultThingWith generates the fixture with one change to its overrides file and returns
// the error.
func generateDefaultThingWith(t *testing.T, old, replacement string) error {
	t.Helper()
	spec, err := os.ReadFile(filepath.Join("testdata", "defaultthing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	overrides, err := os.ReadFile(filepath.Join("testdata", "defaultthing-overrides.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Replace(string(overrides), old, replacement, 1)
	if text == string(overrides) {
		t.Fatalf("the overrides file has no %q", old)
	}
	path := filepath.Join(t.TempDir(), "overrides.yaml")
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	input, loadDir := syntheticInput(t)
	input.OpenAPI = spec
	options := Options{Resource: "DefaultThing", OutputDir: filepath.Join(t.TempDir(), "out"), OverridesPath: path}
	return generateFromInput(options, input, loadDir)
}

// Each default that does not fit the attribute, or that the lines of its fields contradict, stops the
// generator with a message that names the place. No such default is left out or changed silently.
func TestObjectDefaultErrors(t *testing.T) {
	tests := map[string]struct{ old, new, want string }{
		"object default without computed": {
			"routing: {computed: true, default: {}}", "routing: {default: {}}", "not computed",
		},
		"unknown attribute": {
			"default: {unit: never, enabled: false, window: {days: 7}}", "default: {unit: never, enabled: false, window: {days: 7}, colour: red}", "colour is no attribute of the object",
		},
		"unknown nested attribute": {
			"window: {days: 7}}}", "window: {day: 7}}}", "day is no attribute of the object",
		},
		"string for a bool": {
			"enabled: false, window", "enabled: \"no\", window", "a default of type string does not fit a Bool attribute",
		},
		"number for a string": {
			"{unit: never, enabled", "{unit: 3, enabled", "a default of type int does not fit a String attribute",
		},
		"value outside the enum": {
			"{unit: never, enabled", "{unit: weeks, enabled", `"weeks" is not a Terraform value of the enum DefaultUnit`,
		},
		"the own default of an attribute differs": {
			"unit: {computed: true, default: never}", "unit: {computed: true, default: hours}", "has its own default",
		},
		"the own default of an attribute, which the object leaves null": {
			"limits: {computed: true, default: {unit: never, enabled: false, window: {days: 7}}}", "limits: {computed: true, default: {enabled: false, window: {days: 7}}}", "has its own default",
		},
		"the own default of a nested object differs": {
			"window: {computed: true, default: {days: 7}}", "window: {computed: true, default: {days: 8}}", "has its own default",
		},
		"object default on a list": {
			"rules: {computed: true, default: []}", "rules: {computed: true, default: {name: x}}", "an object default fits an object attribute, not a ListNested",
		},
		"empty list on an object": {
			"routing: {computed: true, default: {}}", "routing: {computed: true, default: []}", "[] fits a list or set attribute, not a SingleNested",
		},
		"null on a scalar": {
			"enabled: {computed: true, default: false}", "enabled: {computed: true, default: null}", "null fits a list, set, map, or object attribute, not a Bool",
		},
		"a list that is not empty inside an object": {
			"routing: {computed: true, default: {}}", "routing: {computed: true, default: {targets: [a]}}", "a list default must be empty",
		},
		"null without computed": {
			"overrides: {computed: true, default: null,", "overrides: {default: null,", "not computed",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			err := generateDefaultThingWith(t, test.old, test.new)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err = %v, want it to contain %q", err, test.want)
			}
		})
	}
}

// A default object checks the values of its attributes against the validators of their lines, and a
// required attribute needs a value.
func TestObjectDefaultChecksTheValidatorsAndRequiredAttributes(t *testing.T) {
	tests := map[string]struct{ old, new, want string }{
		"between": {
			"      enabled: {computed: true, default: false}\n", "      enabled: {computed: true, default: false}\n      count: {computed: true, default: 5, validators: [{between: [10, 20]}]}\n", "outside the between validator",
		},
		"required attribute left null": {
			"      unit: {computed: true, default: never}\n", "      unit: {computed: true, default: never}\n      count: {required: true}\n", "is required: the default must give it a value",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			err := generateDefaultThingWith(t, test.old, test.new)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err = %v, want it to contain %q", err, test.want)
			}
		})
	}
}
