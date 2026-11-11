package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// generateGetThing generates folderthing with the getHooks line: Hooks have Get and AfterGet.
func generateGetThing(t *testing.T) string {
	t.Helper()
	spec, err := os.ReadFile(filepath.Join("testdata", "folderthing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	input, loadDir := syntheticInput(t)
	input.OpenAPI = spec
	out := filepath.Join(t.TempDir(), "getthing")
	options := Options{Resource: "FolderThing", OutputDir: out, OverridesPath: filepath.Join("testdata", "getthing-overrides.yaml")}
	if err := generateFromInput(options, input, loadDir); err != nil {
		t.Fatal(err)
	}
	return out
}

// The golden output of a released resource whose Get call has hooks, with a data source.
func TestGoldenOutputGetHooks(t *testing.T) {
	out := generateGetThing(t)
	golden := filepath.Join("testdata", "golden", "getthing")
	if *updateGolden {
		if err := os.RemoveAll(golden); err != nil {
			t.Fatal(err)
		}
		if err := copyDirectory(out, golden); err != nil {
			t.Fatal(err)
		}
	}
	if diff := compareDirectories(golden, out); diff != "" {
		t.Fatalf("golden output differs; run go test ./internal/generator -run TestGoldenOutputGetHooks -update:\n%s", diff)
	}
}

// The default Get call runs without hooks, the Get hook replaces it with its classification of
// errors, and AfterGet sees the whole response after Read, Create, Update, and in the data source.
func TestGetHooksRuntimeSemantics(t *testing.T) {
	input, _ := syntheticInput(t)
	out := generateGetThing(t)
	semantics, err := os.ReadFile(filepath.Join("testdata", "getthing_semantics_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "semantics_test.go"), semantics, 0o644); err != nil {
		t.Fatal(err)
	}
	compileGenerated(t, out, input)
}

// A resource without the line has no Get and AfterGet hooks, no getResult in its context, and a
// data source that does not run them.
func TestNoGetHooksWithoutTheLine(t *testing.T) {
	spec, err := os.ReadFile(filepath.Join("testdata", "folderthing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	overrides, err := os.ReadFile(filepath.Join("testdata", "getthing-overrides.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	without := strings.Replace(string(overrides), "getHooks: true\n", "", 1)
	if without == string(overrides) {
		t.Fatal("the overrides have no getHooks line: update the replaced text")
	}
	path := filepath.Join(t.TempDir(), "behavior-overrides.yaml")
	if err := os.WriteFile(path, []byte(without), 0o644); err != nil {
		t.Fatal(err)
	}
	input, loadDir := syntheticInput(t)
	input.OpenAPI = spec
	out := filepath.Join(t.TempDir(), "getthing")
	if err := generateFromInput(Options{Resource: "FolderThing", OutputDir: out, OverridesPath: path}, input, loadDir); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"resource.go", "data_source.go"} {
		data, err := os.ReadFile(filepath.Join(out, name))
		if err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{"AfterGet", "getResult", "net/http", "hooks.Get"} {
			if strings.Contains(string(data), forbidden) {
				t.Fatalf("%s of a resource without getHooks has %q", name, forbidden)
			}
		}
	}
}

// A singleton has no id in its Get call, and its data source can pass a query id instead: it has
// no Get hooks.
func TestGetHooksRejectASingleton(t *testing.T) {
	spec, err := os.ReadFile(filepath.Join("testdata", "singleton-id.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	overrides, err := os.ReadFile(filepath.Join("testdata", "singleton-id-overrides.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	err = generateSingleton(t, spec, string(overrides)+"\ngetHooks: true\n")
	if err == nil || !strings.Contains(err.Error(), "getHooks is for a resource whose Get call takes an id") {
		t.Fatalf("error = %v, want getHooks to need an id in the Get call", err)
	}
}
