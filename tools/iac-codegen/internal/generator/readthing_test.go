package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// generateReadThing generates legacything, whose writes return the resource, with the getHooks and
// readAfterWrite lines, or with the overrides of the caller.
func generateReadThing(t *testing.T, overridesPath string) (string, error) {
	t.Helper()
	spec, err := os.ReadFile(filepath.Join("testdata", "legacy.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	input, loadDir := syntheticInput(t)
	input.OpenAPI = spec
	out := filepath.Join(t.TempDir(), "readthing")
	options := Options{Resource: "LegacyThing", OutputDir: out, OverridesPath: overridesPath}
	return out, generateFromInput(options, input, loadDir)
}

// readThingOverrides returns the overrides of the fixture, with the first occurrence of old
// replaced by replacement, in a file of its own.
func readThingOverrides(t *testing.T, old, replacement string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "readthing-overrides.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	changed := strings.Replace(string(data), old, replacement, 1)
	if changed == string(data) && old != replacement {
		t.Fatalf("the overrides have no %q: update the replaced text", old)
	}
	path := filepath.Join(t.TempDir(), "behavior-overrides.yaml")
	if err := os.WriteFile(path, []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// The golden output of a released resource whose writes return the resource and that reads it back
// after a write.
func TestGoldenOutputReadAfterWrite(t *testing.T) {
	out, err := generateReadThing(t, filepath.Join("testdata", "readthing-overrides.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	golden := filepath.Join("testdata", "golden", "readthing")
	if *updateGolden {
		if err := os.RemoveAll(golden); err != nil {
			t.Fatal(err)
		}
		if err := copyDirectory(out, golden); err != nil {
			t.Fatal(err)
		}
	}
	if diff := compareDirectories(golden, out); diff != "" {
		t.Fatalf("golden output differs; run go test ./internal/generator -run TestGoldenOutputReadAfterWrite -update:\n%s", diff)
	}
}

// With the line, the generated Create and Update hand the written resource to readAfterWrite, which
// reads it with Get so that AfterGet runs, and keeps the write response when the read fails.
// Without the line, they return the write response and the package has no such helper.
func TestReadAfterWriteInTheGeneratedCalls(t *testing.T) {
	with, err := generateReadThing(t, filepath.Join("testdata", "readthing-overrides.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(with, "resource.go"))
	if err != nil {
		t.Fatal(err)
	}
	resource := string(data)
	if got := strings.Count(resource, "r.readAfterWrite(ctx, "); got != 2 {
		t.Fatalf("calls of readAfterWrite = %d, want one in create and one in update", got)
	}
	for _, want := range []string{"func (r *Resource) readAfterWrite(", "v, err := r.get(ctx, id)", "if err != nil || v == nil {\n\t\treturn written\n\t}"} {
		if !strings.Contains(resource, want) {
			t.Fatalf("the generated resource lacks %q", want)
		}
	}

	path := readThingOverrides(t, "readAfterWrite: true\n", "")
	without, err := generateReadThing(t, path)
	if err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(filepath.Join(without, "resource.go"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "readAfterWrite") {
		t.Fatal("the generated resource of a file without the line has readAfterWrite")
	}
}

// The line feeds the AfterGet hook, so it needs getHooks.
func TestReadAfterWriteNeedsGetHooks(t *testing.T) {
	path := readThingOverrides(t, "getHooks: true\n", "")
	_, err := generateReadThing(t, path)
	if err == nil || !strings.Contains(err.Error(), "readAfterWrite needs getHooks") {
		t.Fatalf("error = %v, want readAfterWrite to need getHooks", err)
	}
}

// A resource whose Create and Update already read the resource with Get has nothing to change, so
// the line is an error and does not hide where the behavior comes from.
func TestReadAfterWriteRejectsWritesThatAlreadyReadBack(t *testing.T) {
	spec, err := os.ReadFile(filepath.Join("testdata", "folderthing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	overrides, err := os.ReadFile(filepath.Join("testdata", "getthing-overrides.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "behavior-overrides.yaml")
	if err := os.WriteFile(path, []byte(strings.Replace(string(overrides), "getHooks: true\n", "getHooks: true\nreadAfterWrite: true\n", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	input, loadDir := syntheticInput(t)
	input.OpenAPI = spec
	options := Options{Resource: "FolderThing", OutputDir: filepath.Join(t.TempDir(), "getthing"), OverridesPath: path}
	err = generateFromInput(options, input, loadDir)
	if err == nil || !strings.Contains(err.Error(), "readAfterWrite changes nothing") {
		t.Fatalf("error = %v, want readAfterWrite to change nothing", err)
	}
}
