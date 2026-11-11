package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// generateConfigThingWith generates the configthing contract with its overrides file changed: the
// first from in it becomes to. It returns the error of the generator.
func generateConfigThingWith(t *testing.T, from, to string) error {
	t.Helper()
	base, err := os.ReadFile(filepath.Join("testdata", "configthing-overrides.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	spec, err := os.ReadFile(filepath.Join("testdata", "configthing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	changed := strings.Replace(string(base), from, to, 1)
	if changed == string(base) {
		t.Fatalf("the overrides file has no %q", from)
	}
	dir := t.TempDir()
	overrides := filepath.Join(dir, "configthing-overrides.yaml")
	if err := os.WriteFile(overrides, []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}
	input, loadDir := syntheticInput(t)
	input.OpenAPI = spec
	return generateFromInput(Options{Resource: "ConfigThing", OutputDir: filepath.Join(dir, "configthing"), OverridesPath: overrides}, input, loadDir)
}

// terraformType Float64 needs a float32 field and Number needs an int32 or a double field.
func TestFloatAndNumberKeysRefuseOtherFields(t *testing.T) {
	tests := map[string]struct{ from, to, want string }{
		"Float64 on a double":  {"scale: {terraformType: Number}", "scale: {terraformType: Float64}", "terraformType Float64 needs a float32"},
		"Float64 on a string":  {"ratio: {terraformType: Float64}", "ratio: {terraformType: Float64}\n      name: {terraformType: Float64}", "terraformType Float64 needs a float32"},
		"Number on a string":   {"ratio: {terraformType: Float64}", "ratio: {terraformType: Float64}\n      name: {terraformType: Number}", "terraformType Number needs an int32 or a double"},
		"Number on a float32":  {"ratio: {terraformType: Float64}", "ratio: {terraformType: Number}", "terraformType Number needs an int32 or a double"},
		"an unknown type name": {"ratio: {terraformType: Float64}", "ratio: {terraformType: Float32}", "terraformType is"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if err := generateConfigThingWith(t, test.from, test.to); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want it to contain %q", err, test.want)
			}
		})
	}
}

// A validator target must exist, and equals is true only.
func TestValidatorKeysRefuseBadTargets(t *testing.T) {
	tests := map[string]struct{ from, to, want string }{
		"a target that goes too far up":                            {`alsoRequires: ["../name"]`, `alsoRequires: ["../../../name"]`, "goes up 3 levels"},
		"a target that is not above":                               {`alsoRequires: ["../name"]`, `alsoRequires: ["../nothing"]`, "is not another attribute"},
		"a target at the level of the attribute that has the line": {`alsoRequires: ["../name"]`, `alsoRequires: ["name"]`, "is not another attribute"},
		"equals false": {"- equals: true", "- equals: false", "equals is true only"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if err := generateConfigThingWith(t, test.from, test.to); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want it to contain %q", err, test.want)
			}
		})
	}
}
