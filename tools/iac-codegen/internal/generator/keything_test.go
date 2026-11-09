package generator

import (
	"os"
	"path/filepath"
	"testing"
)

func generateKeyThing(t *testing.T) string {
	t.Helper()
	spec, err := os.ReadFile(filepath.Join("testdata", "keything.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	input, loadDir := syntheticInput(t)
	input.OpenAPI = spec
	out := filepath.Join(t.TempDir(), "keything")
	options := Options{Resource: "KeyThing", OutputDir: out, OverridesPath: filepath.Join("testdata", "keything-overrides.yaml")}
	if err := generateFromInput(options, input, loadDir); err != nil {
		t.Fatal(err)
	}
	return out
}

// updateAs, an unwrap entry for a wrapper that only Update holds, and itemName: Update names and
// wraps values differently from Create and Get, and Get holds each preset as an object.
func TestGoldenOutputKeyThing(t *testing.T) {
	out := generateKeyThing(t)
	golden := filepath.Join("testdata", "golden", "keything")
	if *updateGolden {
		if err := os.RemoveAll(golden); err != nil {
			t.Fatal(err)
		}
		if err := copyDirectory(out, golden); err != nil {
			t.Fatal(err)
		}
	}
	if diff := compareDirectories(golden, out); diff != "" {
		t.Fatalf("golden output differs; run go test ./internal/generator -run TestGoldenOutputKeyThing -update:\n%s", diff)
	}
}

// The generated code runs: Create and Update send the names and the wrappers of their own bodies,
// and flatten takes the names out of the preset objects.
func TestKeyThingRuntimeSemantics(t *testing.T) {
	input, _ := syntheticInput(t)
	out := generateKeyThing(t)
	semantics, err := os.ReadFile(filepath.Join("testdata", "keything_semantics_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "semantics_test.go"), semantics, 0o644); err != nil {
		t.Fatal(err)
	}
	compileGenerated(t, out, input)
}
