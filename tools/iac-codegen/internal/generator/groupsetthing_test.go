package generator

import (
	"os"
	"path/filepath"
	"testing"
)

// The golden output of a released resource whose groups are a set with nested server defaults that
// the response requires, a validator line that keeps such a default, a uint64 limit shown as Int64, a
// skipped uint64 that may not fit Int64, a required line on a component that only the response
// has, and a string attribute that the API does not hold.
func TestGoldenOutputGroupSetThing(t *testing.T) {
	out := generateGroupSetThing(t)
	golden := filepath.Join("testdata", "golden", "groupsetthing")
	if *updateGolden {
		if err := os.RemoveAll(golden); err != nil {
			t.Fatal(err)
		}
		if err := copyDirectory(out, golden); err != nil {
			t.Fatal(err)
		}
	}
	if diff := compareDirectories(golden, out); diff != "" {
		t.Fatalf("golden output differs; run go test ./internal/generator -run TestGoldenOutputGroupSetThing -update:\n%s", diff)
	}
}

// Create sends the limit as a string and leaves out the unset interval; a read flattens the stored
// limit to Int64, and keeps the document of the plan. A state upgrade keeps the prior document.
func TestGroupSetThingRuntimeSemantics(t *testing.T) {
	input, _ := syntheticInput(t)
	out := generateGroupSetThing(t)
	semantics, err := os.ReadFile(filepath.Join("testdata", "groupsetthing_semantics_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "semantics_test.go"), semantics, 0o644); err != nil {
		t.Fatal(err)
	}
	compileGenerated(t, out, input)
}

func generateGroupSetThing(t *testing.T) string {
	t.Helper()
	spec, err := os.ReadFile(filepath.Join("testdata", "groupsetthing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	input, loadDir := syntheticInput(t)
	input.OpenAPI = spec
	out := filepath.Join(t.TempDir(), "groupsetthing")
	options := Options{Resource: "GroupSetThing", OutputDir: out, OverridesPath: filepath.Join("testdata", "groupsetthing-overrides.yaml")}
	if err := generateFromInput(options, input, loadDir); err != nil {
		t.Fatal(err)
	}
	return out
}
