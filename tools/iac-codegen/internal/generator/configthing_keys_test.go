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
	_, err := generateConfigThingSchemaWith(t, from, to)
	return err
}

// generateConfigThingSchemaWith is generateConfigThingWith that also returns the generated
// schema.go.
func generateConfigThingSchemaWith(t *testing.T, from, to string) (string, error) {
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
	out := filepath.Join(dir, "configthing")
	if err := generateFromInput(Options{Resource: "ConfigThing", OutputDir: out, OverridesPath: overrides}, input, loadDir); err != nil {
		return "", err
	}
	schema, err := os.ReadFile(filepath.Join(out, "schema.go"))
	if err != nil {
		t.Fatal(err)
	}
	return string(schema), nil
}

// A "../" in a validator target goes up one object level, and the level of an item of a list is two
// path steps (the element and the name of the list). The attribute is a map inside the selector
// object of an item of the list remotes: "../name" is the name of the item (two steps up from the
// attribute: its name, then the selector), and "../../name" is the name of the resource, which is
// also past the element and the name of the list (four steps).
func TestAncestorTargetCountsTheStepsOfAList(t *testing.T) {
	tests := map[string]struct{ target, want string }{
		"an attribute of the item":     {"../name", `path.MatchRelative().AtParent().AtParent().AtName("name")`},
		"an attribute of the resource": {"../../name", `path.MatchRelative().AtParent().AtParent().AtParent().AtParent().AtName("name")`},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// The overrides file has "../name" already, so its golden holds the first case.
			schema, err := os.ReadFile(filepath.Join("testdata", "golden", "configthing", "schema.go"))
			if err != nil {
				t.Fatal(err)
			}
			if test.target != "../name" {
				generated, err := generateConfigThingSchemaWith(t, `alsoRequires: ["../name"]`, `alsoRequires: ["`+test.target+`"]`)
				if err != nil {
					t.Fatal(err)
				}
				schema = []byte(generated)
			}
			if !strings.Contains(string(schema), "mapvalidator.AlsoRequires("+test.want+")") {
				t.Errorf("the schema has no mapvalidator.AlsoRequires(%s)", test.want)
			}
		})
	}
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
