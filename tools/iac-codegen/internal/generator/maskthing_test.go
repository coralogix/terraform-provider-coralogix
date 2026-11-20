package generator

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func maskThingSpec(t *testing.T) []byte {
	t.Helper()
	spec, err := os.ReadFile(filepath.Join("testdata", "maskthing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return spec
}

func generateMaskThing(t *testing.T, spec []byte) (string, error) {
	t.Helper()
	input, loadDir := syntheticInput(t)
	input.OpenAPI = spec
	out := filepath.Join(t.TempDir(), "maskthing")
	return out, generateFromInput(Options{Resource: "MaskThing", OutputDir: out}, input, loadDir)
}

// The golden output of a new resource whose update mask pattern rejects dotted paths. The API takes
// only top-level names and replaces the whole value of each field, the arms of a oneOf included.
// The mask has no nested path, and no oneOf arm path.
func TestGoldenOutputMaskThing(t *testing.T) {
	out, err := generateMaskThing(t, maskThingSpec(t))
	if err != nil {
		t.Fatal(err)
	}
	golden := filepath.Join("testdata", "golden", "maskthing")
	if *updateGolden {
		if err := os.RemoveAll(golden); err != nil {
			t.Fatal(err)
		}
		if err := copyDirectory(out, golden); err != nil {
			t.Fatal(err)
		}
	}
	if diff := compareDirectories(golden, out); diff != "" {
		t.Fatalf("golden output differs; run go test ./internal/generator -run TestGoldenOutputMaskThing -update:\n%s", diff)
	}
	acceptance := readGenerated(t, out, "acceptance_test.go")
	for _, want := range []string{
		// A computed-only attribute that the response lists as required has a value, and the data
		// source holds the same value. The attribute that the response does not list (owner) has no check.
		`resource.TestCheckResourceAttrSet(resourceAddress, "create_time")`,
		`resource.TestCheckResourceAttrPair(dataSourceAddress, "create_time", resourceAddress, "create_time")`,
		// A step of the sweep checks each set element that it configures: a scalar leaf and the
		// leaves of a oneOf arm.
		`"bool_filter.value": "true"`,
		`"string_filter.field": "MASK_THING_STRING_FIELD_COUNTRY"`,
	} {
		if !strings.Contains(acceptance, want) {
			t.Errorf("acceptance_test.go does not contain %q", want)
		}
	}
	if strings.Contains(acceptance, `"owner"`) {
		t.Error("acceptance_test.go checks owner, which the response does not list as required")
	}
	if strings.Contains(acceptance, "The resource always exists") {
		t.Error("acceptance_test.go ends the sweep with the full config, but the API has a Delete")
	}
	mask := readGenerated(t, out, "mask.go")
	if strings.Contains(mask, "maskNode") || strings.Contains(mask, "oneOfPaths") {
		t.Error("mask.go has a mask tree, but the API takes only top-level names")
	}
}

// A top-level mask pattern and a oneOf in an Update field are an eligible contract for a new
// resource: the check passes and the generator makes the resource.
func TestMaskThingCheckPasses(t *testing.T) {
	candidate := filepath.Join(t.TempDir(), "candidate.yaml")
	if err := os.WriteFile(candidate, maskThingSpec(t), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Check(CheckOptions{Resource: "MaskThing", OpenAPIPath: candidate}); err != nil {
		t.Fatalf("check: %v", err)
	}
}

// A pattern that accepts dotted paths still gives leaf paths: the same contract with that pattern
// has a mask tree, and the oneOf arm is a path.
func TestMaskThingWithDottedPatternHasLeafMask(t *testing.T) {
	spec := strings.Replace(string(maskThingSpec(t)),
		"'^[A-Za-z_][A-Za-z0-9_]*(,[A-Za-z_][A-Za-z0-9_]*)*$'",
		"'^[A-Za-z_][A-Za-z0-9_.]*(,[A-Za-z_][A-Za-z0-9_.]*)*$'", 1)
	out, err := generateMaskThing(t, []byte(spec))
	if err != nil {
		t.Fatal(err)
	}
	if mask := readGenerated(t, out, "mask.go"); !strings.Contains(mask, "oneOfPaths") {
		t.Error("mask.go has no oneOf paths, but the pattern accepts dotted paths")
	}
}

// The mask pattern states the rule, so a contract with no pattern is still refused, and an
// existing resource keeps its issue (see TestReleasedShapeErrors, the body mask case).
func TestMaskThingWithoutPatternIsRefused(t *testing.T) {
	spec := strings.Replace(string(maskThingSpec(t)),
		", pattern: '^[A-Za-z_][A-Za-z0-9_]*(,[A-Za-z_][A-Za-z0-9_]*)*$'", "", 1)
	_, err := generateMaskThing(t, []byte(spec))
	var eligibility *EligibilityError
	if !errors.As(err, &eligibility) || !hasReportCode(eligibility.Report, "UPDATE_MASK_CONTRACT_MISSING") {
		t.Fatalf("error = %v, want UPDATE_MASK_CONTRACT_MISSING", err)
	}
}

// The generated code runs: an arm switch and a removal name the top-level field, and the body holds
// the whole value of the field.
func TestMaskThingRuntimeSemantics(t *testing.T) {
	input, _ := syntheticInput(t)
	out, err := generateMaskThing(t, maskThingSpec(t))
	if err != nil {
		t.Fatal(err)
	}
	semantics, err := os.ReadFile(filepath.Join("testdata", "maskthing_semantics_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "semantics_test.go"), semantics, 0o644); err != nil {
		t.Fatal(err)
	}
	compileGenerated(t, out, input)
}
