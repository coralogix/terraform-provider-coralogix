package generator

import (
	"os"
	"path/filepath"
	"testing"
)

// The golden output of a released resource whose list items are each one arm of a oneOf, as the
// filters of a dashboard widget are: an arm with a nested object, an object, a string, and an
// arm with no fields shown as a bool (asBool).
func TestGoldenOutputFilterThing(t *testing.T) {
	out := generateFilterThing(t)
	golden := filepath.Join("testdata", "golden", "filterthing")
	if *updateGolden {
		if err := os.RemoveAll(golden); err != nil {
			t.Fatal(err)
		}
		if err := copyDirectory(out, golden); err != nil {
			t.Fatal(err)
		}
	}
	if diff := compareDirectories(golden, out); diff != "" {
		t.Fatalf("golden output differs; run go test ./internal/generator -run TestGoldenOutputFilterThing -update:\n%s", diff)
	}
}

// The generated resource runs: see testdata/filterthing_semantics_test.go.
func TestFilterThingRuntimeSemantics(t *testing.T) {
	input, _ := syntheticInput(t)
	out := generateFilterThing(t)
	semantics, err := os.ReadFile(filepath.Join("testdata", "filterthing_semantics_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "semantics_test.go"), semantics, 0o644); err != nil {
		t.Fatal(err)
	}
	compileGenerated(t, out, input)
}

func generateFilterThing(t *testing.T) string {
	t.Helper()
	spec, err := os.ReadFile(filepath.Join("testdata", "filterthing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	input, loadDir := syntheticInput(t)
	input.OpenAPI = spec
	out := filepath.Join(t.TempDir(), "filterthing")
	options := Options{Resource: "FilterThing", OutputDir: out, OverridesPath: filepath.Join("testdata", "filterthing-overrides.yaml")}
	if err := generateFromInput(options, input, loadDir); err != nil {
		t.Fatal(err)
	}
	return out
}
