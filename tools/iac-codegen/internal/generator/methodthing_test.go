package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/overrides"
)

func generateMethodThing(t *testing.T) string {
	t.Helper()
	spec, err := os.ReadFile(filepath.Join("testdata", "methodthing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	input, loadDir := syntheticInput(t)
	input.OpenAPI = spec
	out := filepath.Join(t.TempDir(), "methodthing")
	options := Options{Resource: "MethodThing", OutputDir: out, OverridesPath: filepath.Join("testdata", "methodthing-overrides.yaml")}
	if err := generateFromInput(options, input, loadDir); err != nil {
		t.Fatal(err)
	}
	return out
}

// api.methods (a PUT Create and a POST Update), a Create response that holds the id as an
// integer, a oneOf of the Create request with one arm skipped, and equality: caseInsensitive on a
// set of strings that the Update body wraps.
func TestGoldenOutputMethodThing(t *testing.T) {
	out := generateMethodThing(t)
	golden := filepath.Join("testdata", "golden", "methodthing")
	if *updateGolden {
		if err := os.RemoveAll(golden); err != nil {
			t.Fatal(err)
		}
		if err := copyDirectory(out, golden); err != nil {
			t.Fatal(err)
		}
	}
	if diff := compareDirectories(golden, out); diff != "" {
		t.Fatalf("golden output differs; run go test ./internal/generator -run TestGoldenOutputMethodThing -update:\n%s", diff)
	}
}

// The generated code runs: Create reads the integer id, Update wraps the permissions, and the
// read keeps the case of the configured permissions.
func TestMethodThingRuntimeSemantics(t *testing.T) {
	input, _ := syntheticInput(t)
	out := generateMethodThing(t)
	semantics, err := os.ReadFile(filepath.Join("testdata", "methodthing_semantics_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "semantics_test.go"), semantics, 0o644); err != nil {
		t.Fatal(err)
	}
	compileGenerated(t, out, input)
}

// equality: caseInsensitive keeps the spelling of the elements of a list or set of strings. On a
// plain string, or on a list of objects, there is nothing to keep, and the generator says so.
func TestCaseInsensitiveEqualityNeedsAListOfStrings(t *testing.T) {
	spec, err := os.ReadFile(filepath.Join("testdata", "methodthing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	overrides, err := os.ReadFile(filepath.Join("testdata", "methodthing-overrides.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "overrides.yaml")
	changed := strings.Replace(string(overrides), "      name: {updateAs: newName, required: true}", "      name: {updateAs: newName, required: true, equality: caseInsensitive}", 1)
	if changed == string(overrides) {
		t.Fatal("the overrides do not have the name line")
	}
	if err := os.WriteFile(path, []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}
	input, loadDir := syntheticInput(t)
	input.OpenAPI = spec
	err = generateFromInput(Options{Resource: "MethodThing", OutputDir: filepath.Join(dir, "methodthing"), OverridesPath: path}, input, loadDir)
	if err == nil || !strings.Contains(err.Error(), "OVERRIDE_EQUALITY_NOT_STRING") {
		t.Fatalf("err = %v, want OVERRIDE_EQUALITY_NOT_STRING for a plain string", err)
	}
}

// An Update by POST has no update mask, and the model leaves the choice of fields to the tool: the
// overrides must state updateMask: omit, or the generator refuses the resource.
func TestUpdateByPOSTNeedsTheOmitDecision(t *testing.T) {
	spec, err := os.ReadFile(filepath.Join("testdata", "methodthing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	overrides, err := os.ReadFile(filepath.Join("testdata", "methodthing-overrides.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	changed := strings.Replace(string(overrides), "\nupdateMask: omit\n", "\n", 1)
	if changed == string(overrides) {
		t.Fatal("the overrides do not have the updateMask line")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "overrides.yaml")
	if err := os.WriteFile(path, []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}
	input, loadDir := syntheticInput(t)
	input.OpenAPI = spec
	err = generateFromInput(Options{Resource: "MethodThing", OutputDir: filepath.Join(dir, "methodthing"), OverridesPath: path}, input, loadDir)
	if err == nil || !strings.Contains(err.Error(), "an Update by POST has no update mask") {
		t.Fatalf("err = %v, want an error that asks for updateMask: omit", err)
	}
}

// The generated Update of a POST sends no mask and is no full replace: the package has the mask
// helpers and no replace.go.
func TestUpdateByPOSTIsNoReplace(t *testing.T) {
	out := generateMethodThing(t)
	if _, err := os.Stat(filepath.Join(out, "replace.go")); err == nil {
		t.Error("replace.go exists: an Update by POST is no full replace")
	}
	if _, err := os.Stat(filepath.Join(out, "mask.go")); err != nil {
		t.Errorf("mask.go: %v, want the change detection of Update", err)
	}
	resource, err := os.ReadFile(filepath.Join(out, "resource.go"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(resource), "mask string") || strings.Contains(string(resource), ".UpdateMask(") {
		t.Error("resource.go sends an update mask, but an Update by POST has none")
	}
}

// An object that the package compares, such as the item of a set, compares a list or set of strings
// with equality: caseInsensitive without the case, so the prior item pairs with an API item that has
// an element in another case.
func TestSameCheckIgnoresTheCaseOfCaseInsensitiveStrings(t *testing.T) {
	list := &convField{Conv: convStrings, Collection: "List", SDK: "Names", Equality: overrides.EqualityCaseInsensitive}
	got, err := sameCheck(list)
	if err != nil || got != "slices.EqualFunc(a.Names, b.Names, strings.EqualFold)" {
		t.Errorf("list = %q (%v), want a comparison with strings.EqualFold", got, err)
	}
	set := &convField{Conv: convStrings, Collection: "Set", SDK: "Names", Equality: overrides.EqualityCaseInsensitive}
	got, err = sameCheck(set)
	if err != nil || got != "sameUnordered(a.Names, b.Names, sameFoldedString)" {
		t.Errorf("set = %q (%v), want an unordered comparison in any case", got, err)
	}
	plain := &convField{Conv: convStrings, Collection: "List", SDK: "Names"}
	if got, _ = sameCheck(plain); got != "slices.Equal(a.Names, b.Names)" {
		t.Errorf("plain list = %q, want the exact comparison", got)
	}
}
