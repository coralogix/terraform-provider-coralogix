package generator

import (
	"os"
	"path/filepath"
	"testing"
)

// The golden output of a released collection resource: one write replaces the things of a kind,
// Get lists them with a fixed query, the written item wraps the common fields and sends disabled
// for enabled, and a rule shows its comma-separated values as a set.
func TestGoldenOutputOverwriteThing(t *testing.T) {
	out := generateOverwriteThing(t)
	golden := filepath.Join("testdata", "golden", "overwritething")
	if *updateGolden {
		if err := os.RemoveAll(golden); err != nil {
			t.Fatal(err)
		}
		if err := copyDirectory(out, golden); err != nil {
			t.Fatal(err)
		}
	}
	if diff := compareDirectories(golden, out); diff != "" {
		t.Fatalf("golden output differs; run go test ./internal/generator -run TestGoldenOutputOverwriteThing -update:\n%s", diff)
	}
}

// Create and Update send the write and read the things back with the query of the kind. Delete
// sends the write with no things. The request holds the fields in the wrapper and negates enabled.
func TestOverwriteThingRuntimeSemantics(t *testing.T) {
	input, _ := syntheticInput(t)
	out := generateOverwriteThing(t)
	semantics, err := os.ReadFile(filepath.Join("testdata", "overwritething_semantics_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "semantics_test.go"), semantics, 0o644); err != nil {
		t.Fatal(err)
	}
	compileGenerated(t, out, input)
}

func generateOverwriteThing(t *testing.T) string {
	t.Helper()
	spec, err := os.ReadFile(filepath.Join("testdata", "overwritething.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	input, loadDir := syntheticInput(t)
	input.OpenAPI = spec
	out := filepath.Join(t.TempDir(), "overwritething")
	options := Options{Resource: "ThingList", OutputDir: out, OverridesPath: filepath.Join("testdata", "overwritething-overrides.yaml")}
	if err := generateFromInput(options, input, loadDir); err != nil {
		t.Fatal(err)
	}
	return out
}
