package generator

import (
	"os"
	"path/filepath"
	"strings"
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

// A terraformType String line keeps the validators over other attributes, and refuses a validator
// over the value: the contract limit of the number has no string counterpart.
func TestTerraformTypeStringRefusesAValueValidator(t *testing.T) {
	spec, err := os.ReadFile(filepath.Join("testdata", "keything.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	overrides, err := os.ReadFile(filepath.Join("testdata", "keything-overrides.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "overrides.yaml")
	inferred := strings.Replace(string(overrides), "inferred: false", "inferred: true", 1)
	if inferred == string(overrides) {
		t.Fatal("the overrides do not have validators.inferred: false")
	}
	if err := os.WriteFile(path, []byte(inferred), 0o644); err != nil {
		t.Fatal(err)
	}
	input, loadDir := syntheticInput(t)
	input.OpenAPI = spec
	err = generateFromInput(Options{Resource: "KeyThing", OutputDir: filepath.Join(dir, "keything"), OverridesPath: path}, input, loadDir)
	if err == nil || !strings.Contains(err.Error(), "terraformType String cannot keep the validator") {
		t.Fatalf("err = %v, want a refusal of the number limit on the team id", err)
	}
}
