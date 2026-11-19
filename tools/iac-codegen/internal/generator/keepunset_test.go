package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// generateFixtureWith generates the resource of a released fixture, with the overrides of the
// fixture and the extra lines, which come right after the mode line.
func generateFixtureWith(t *testing.T, resource, fixture, extra string) (string, error) {
	t.Helper()
	spec, err := os.ReadFile(filepath.Join("testdata", fixture+".yaml"))
	if err != nil {
		t.Fatal(err)
	}
	overrides, err := os.ReadFile(filepath.Join("testdata", fixture+"-overrides.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(overrides), "mode: existing\n") {
		t.Fatalf("%s has no mode line to put the extra lines after", fixture)
	}
	text := strings.Replace(string(overrides), "mode: existing\n", "mode: existing\n"+extra, 1)
	path := filepath.Join(t.TempDir(), fixture+"-overrides.yaml")
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	input, loadDir := syntheticInput(t)
	input.OpenAPI = spec
	out := filepath.Join(t.TempDir(), fixture)
	options := Options{Resource: resource, OutputDir: out, OverridesPath: path}
	return out, generateFromInput(options, input, loadDir)
}

// keepUnsetComputedState gives a resource with no equality line the generated ModifyPlan step that
// plans no change when only unset unknown values differ, and the generated code runs it.
func TestKeepUnsetComputedStateEmitsTheStep(t *testing.T) {
	without, err := generateFixtureWith(t, "FilterThing", "filterthing", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(without, "equality.go")); err == nil {
		t.Fatal("a resource with no equality line and no key must not have equality.go")
	}

	out, err := generateFixtureWith(t, "FilterThing", "filterthing", "keepUnsetComputedState: true\n")
	if err != nil {
		t.Fatal(err)
	}
	equality, err := os.ReadFile(filepath.Join(out, "equality.go"))
	if err != nil {
		t.Fatalf("the key must emit equality.go: %v", err)
	}
	if !strings.Contains(string(equality), "onlyUnsetUnknowns(plan, req.Config.Raw, req.State.Raw)") {
		t.Fatal("equality.go has no onlyUnsetUnknowns step")
	}
	if strings.Contains(string(equality), "resetsToDefault") {
		t.Fatal("a resource with no server default must not have the resetsToDefault map")
	}

	semantics, err := os.ReadFile(filepath.Join("testdata", "keepunset_semantics_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "semantics_test.go"), semantics, 0o644); err != nil {
		t.Fatal(err)
	}
	input, _ := syntheticInput(t)
	compileGenerated(t, out, input)
}

// An attribute with a server default plans unknown when the configuration drops its value, so the
// update resets it. The step must not read that unknown value as an unset computed value.
func TestKeepUnsetComputedStateSkipsAttributesWithAServerDefault(t *testing.T) {
	out, err := generateFixtureWith(t, "ActionThing", "actionthing", "keepUnsetComputedState: true\n")
	if err != nil {
		t.Fatal(err)
	}
	equality, err := os.ReadFile(filepath.Join(out, "equality.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"var resetsToDefault = map[string]bool{", `"hidden": true`, "if resetsToDefault[attributeKey(p)] {"} {
		if !strings.Contains(string(equality), want) {
			t.Errorf("equality.go has no %q", want)
		}
	}
	input, _ := syntheticInput(t)
	compileGenerated(t, out, input)
}

// A server default inside a set item has no path that the step could skip, so the key refuses it.
func TestKeepUnsetComputedStateRefusesAServerDefaultInASet(t *testing.T) {
	_, err := generateFixtureWith(t, "GroupSetThing", "groupsetthing", "keepUnsetComputedState: true\n")
	if err == nil || !strings.Contains(err.Error(), "inside a set of objects") {
		t.Fatalf("err = %v, want the set server default error", err)
	}
}
