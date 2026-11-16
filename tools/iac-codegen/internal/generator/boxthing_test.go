package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func generateBoxThingWith(t *testing.T, replacements []string, specEdits ...string) (string, error) {
	t.Helper()
	spec, err := os.ReadFile(filepath.Join("testdata", "boxthing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i+1 < len(specEdits); i += 2 {
		if !strings.Contains(string(spec), specEdits[i]) {
			t.Fatalf("the spec does not contain %q", specEdits[i])
		}
		spec = []byte(strings.Replace(string(spec), specEdits[i], specEdits[i+1], 1))
	}
	overrides, err := os.ReadFile(filepath.Join("testdata", "boxthing-overrides.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(overrides)
	for i := 0; i+1 < len(replacements); i += 2 {
		if !strings.Contains(text, replacements[i]) {
			t.Fatalf("the overrides do not contain %q", replacements[i])
		}
		text = strings.Replace(text, replacements[i], replacements[i+1], 1)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "overrides.yaml")
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	input, loadDir := syntheticInput(t)
	input.OpenAPI = spec
	out := filepath.Join(dir, "boxthing")
	options := Options{Resource: "BoxThing", OutputDir: out, OverridesPath: path}
	return out, generateFromInput(options, input, loadDir)
}

func generateBoxThing(t *testing.T) string {
	t.Helper()
	out, err := generateBoxThingWith(t, nil)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// The golden output of a released resource that is read from its List: the generated Get lists the
// box things and selects the one with the id of the resource.
func TestGoldenOutputBoxThing(t *testing.T) {
	out := generateBoxThing(t)
	golden := filepath.Join("testdata", "golden", "boxthing")
	if *updateGolden {
		if err := os.RemoveAll(golden); err != nil {
			t.Fatal(err)
		}
		if err := copyDirectory(out, golden); err != nil {
			t.Fatal(err)
		}
	}
	if diff := compareDirectories(golden, out); diff != "" {
		t.Fatalf("golden output differs; run go test ./internal/generator -run TestGoldenOutputBoxThing -update:\n%s", diff)
	}
}

// The generated code runs: Read selects the item by its id from the List, a missing item removes
// the resource from the state, and Update puts the id in its path.
func TestBoxThingRuntimeSemantics(t *testing.T) {
	input, _ := syntheticInput(t)
	out := generateBoxThing(t)
	semantics, err := os.ReadFile(filepath.Join("testdata", "boxthing_semantics_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "semantics_test.go"), semantics, 0o644); err != nil {
		t.Fatal(err)
	}
	compileGenerated(t, out, input)
}

// A List that pages its result is refused: the generated Get reads one call of the List, and a tool
// that stopped at the first page would call a resource on a later page gone.
func TestBoxThingPagedListIsRefused(t *testing.T) {
	_, err := generateBoxThingWith(t, nil,
		"      operationId: BoxThingsService_ListBoxThings\n",
		"      operationId: BoxThingsService_ListBoxThings\n      parameters:\n        - {name: page_size, in: query, required: false, schema: {type: integer, format: int32}}\n")
	if err == nil || !strings.Contains(err.Error(), "pages its result") {
		t.Fatalf("error = %v, want one that says the List pages its result", err)
	}
}
