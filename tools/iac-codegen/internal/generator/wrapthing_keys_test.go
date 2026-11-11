package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// generateWrapThingWith generates the wrapthing contract with its overrides file changed: the first
// from in it becomes to. It returns the error of the generator.
func generateWrapThingWith(t *testing.T, from, to string) error {
	t.Helper()
	base, err := os.ReadFile(filepath.Join("testdata", "wrapthing-overrides.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	spec, err := os.ReadFile(filepath.Join("testdata", "wrapthing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	changed := strings.Replace(string(base), from, to, 1)
	if changed == string(base) {
		t.Fatalf("the overrides file has no %q", from)
	}
	dir := t.TempDir()
	overrides := filepath.Join(dir, "wrapthing-overrides.yaml")
	if err := os.WriteFile(overrides, []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}
	input, loadDir := syntheticInput(t)
	input.OpenAPI = spec
	return generateFromInput(Options{Resource: "WrapThing", OutputDir: filepath.Join(dir, "out"), OverridesPath: overrides}, input, loadDir)
}

// clientID: uuid needs a string, states computed itself, and takes only the value uuid.
func TestClientIDKeyIsChecked(t *testing.T) {
	tests := map[string]struct{ from, to, want string }{
		"another kind of id":        {"id: {clientID: uuid}", "id: {clientID: guid}", `want "uuid"`},
		"a line that says computed": {"id: {clientID: uuid}", "id: {clientID: uuid, computed: true}", "clientID makes the attribute computed"},
		"a field that is no string": {"      span:\n        terraformType: Int64\n", "      span:\n        terraformType: Int64\n      limit: {clientID: uuid}\n", "clientID: uuid needs a string attribute"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if err := generateWrapThingWith(t, test.from, test.to); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want it to contain %q", err, test.want)
			}
		})
	}
}
