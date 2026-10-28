package generator

import (
	"os"
	"path/filepath"
	"testing"
)

// The golden output of a released resource whose Create body is the resource, whose Replace wraps
// it and has its id in the body, whose API refuses the field hidden in Create (a default line
// gives its server default), and which keeps a note and tags that a request omits.
func TestGoldenOutputActionThing(t *testing.T) {
	out := generateActionThing(t)
	golden := filepath.Join("testdata", "golden", "actionthing")
	if *updateGolden {
		if err := os.RemoveAll(golden); err != nil {
			t.Fatal(err)
		}
		if err := copyDirectory(out, golden); err != nil {
			t.Fatal(err)
		}
	}
	if diff := compareDirectories(golden, out); diff != "" {
		t.Fatalf("golden output differs; run go test ./internal/generator -run TestGoldenOutputActionThing -update:\n%s", diff)
	}
}

// Create sends the flat body and follows with a Replace for the field that it cannot send. Update
// sends an empty note and empty tags for removed ones, because the API keeps what a request omits.
func TestActionThingRuntimeSemantics(t *testing.T) {
	input, _ := syntheticInput(t)
	out := generateActionThing(t)
	semantics, err := os.ReadFile(filepath.Join("testdata", "actionthing_semantics_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "semantics_test.go"), semantics, 0o644); err != nil {
		t.Fatal(err)
	}
	compileGenerated(t, out, input)
}

func generateActionThing(t *testing.T) string {
	t.Helper()
	spec, err := os.ReadFile(filepath.Join("testdata", "actionthing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	input, loadDir := syntheticInput(t)
	input.OpenAPI = spec
	out := filepath.Join(t.TempDir(), "actionthing")
	options := Options{Resource: "ActionThing", OutputDir: out, OverridesPath: filepath.Join("testdata", "actionthing-overrides.yaml")}
	if err := generateFromInput(options, input, loadDir); err != nil {
		t.Fatal(err)
	}
	return out
}
