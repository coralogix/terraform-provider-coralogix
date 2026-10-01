package generator

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/model"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/overrides"
)

func legacySpec(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "model", "testdata", "legacy.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

const legacyOverrides = `
resource: LegacyThing
mode: existing
validators:
  inferred: false
api:
  requestWrapper: thing
  updateIDInBody: true
  clientSetID: true
types:
  LegacyThing:
    fields:
      id: {description: The id.}
      name: {description: The name.}
      labels: {readEmptyAs: "null"}
      rules: {computed: true}
  LegacyRule:
    fields:
      name: {description: The rule name.}
      kind: {computed: true, default: alpha}
      targets: {computed: true}
  LegacyLabels:
    required: []
    fields:
      env: {computed: true}
  LegacyTarget:
    fields:
      connectorId: {description: The connector.}
      id: {skip: true}
enums:
  legacy.Kind: {zero: unspecified, values: [ALPHA, BETA]}
`

func mustParse(t *testing.T, text string) *overrides.File {
	t.Helper()
	f, err := overrides.Parse([]byte(text))
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func eligibilityCodes(t *testing.T, err error) []string {
	t.Helper()
	if err == nil {
		return nil
	}
	var eligibility *EligibilityError
	if !errors.As(err, &eligibility) {
		t.Fatalf("err = %v, want an EligibilityError", err)
	}
	var codes []string
	for _, item := range eligibility.Report {
		codes = append(codes, item.Code)
	}
	return slices.Compact(codes)
}

// The contract of the legacy resource is fine. Only the renderer features are missing.
func TestLegacyResourceLeavesOnlyRendererFeatures(t *testing.T) {
	_, err := validateOpenAPIWith(legacySpec(t), "LegacyThing", model.OperationIDs{}, "sdk", "provider", mustParse(t, legacyOverrides))
	got := eligibilityCodes(t, err)
	if len(got) == 0 {
		return // every feature is rendered
	}
	if !slices.Equal(got, []string{"RENDERER_POLICY_UNSUPPORTED"}) {
		t.Fatalf("codes = %v, want only RENDERER_POLICY_UNSUPPORTED", got)
	}
}

// A line that matches nothing is an error, so a stale line cannot stay in the file.
func TestStaleOverrideLinesAreErrors(t *testing.T) {
	tests := map[string]struct {
		text string
		line string
	}{
		"component gone":  {"types:\n  Gone:\n    fields:\n      id: {skip: true}\n", "types.Gone.fields.id"},
		"field gone":      {"types:\n  LegacyTarget:\n    fields:\n      gone: {skip: true}\n", "types.LegacyTarget.fields.gone"},
		"required is set": {"types:\n  LegacyRule:\n    required: []\n", "types.LegacyRule.required"},
		"not an enum":     {"enums:\n  LegacyRule: {zero: unspecified}\n", "enums.LegacyRule"},
		"enum component":  {"enums:\n  gone.Kind: {zero: unspecified}\n", "enums.gone.Kind"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			file := mustParse(t, "resource: LegacyThing\nmode: existing\nvalidators:\n  inferred: false\n"+test.text)
			_, err := validateOpenAPIWith(legacySpec(t), "LegacyThing", model.OperationIDs{}, "sdk", "provider", file)
			if err == nil || !strings.Contains(err.Error(), "OVERRIDE_UNUSED") || !strings.Contains(err.Error(), test.line) {
				t.Fatalf("err = %v, want OVERRIDE_UNUSED for %s", err, test.line)
			}
		})
	}
}

// A field line can set several keys, and the contract can state only some of them. The message
// names the stale key and the keys to keep, so that a user does not delete a key that is still needed.
func TestStaleKeyIsNamedNotTheLine(t *testing.T) {
	spec := strings.Replace(string(legacySpec(t)), "      required: []\n      properties:\n        id:", "      required: [name]\n      properties:\n        id:", 1)
	if spec == string(legacySpec(t)) {
		t.Fatal("the test contract did not change: update the replaced text")
	}
	tests := map[string]struct {
		field string
		want  []string
	}{
		"required next to a description": {
			"name: {markdownDescription: The name., required: true}",
			[]string{"types.LegacyThing.fields.name", `the contract already states that "name" is required`, "Delete only the key required. Keep markdownDescription"},
		},
		"readOnly next to useStateForUnknown": {
			"createTime: {readOnly: true, useStateForUnknown: true}",
			[]string{"types.LegacyThing.fields.createTime", `the contract already states that "createTime" is readOnly`, "Delete only the key readOnly. Keep useStateForUnknown"},
		},
		"the only key": {
			"name: {required: true}",
			[]string{"types.LegacyThing.fields.name", "Delete the line: the API contract now states everything that it sets."},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			file := mustParse(t, "resource: LegacyThing\nmode: existing\nvalidators:\n  inferred: false\ntypes:\n  LegacyThing:\n    fields:\n      "+test.field+"\n")
			_, err := validateOpenAPIWith([]byte(spec), "LegacyThing", model.OperationIDs{}, "sdk", "provider", file)
			if err == nil || !strings.Contains(err.Error(), "OVERRIDE_UNUSED") {
				t.Fatalf("err = %v, want OVERRIDE_UNUSED", err)
			}
			for _, want := range test.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("err = %v\nwant it to contain %q", err, want)
				}
			}
		})
	}
}

// A required or readOnly key is needed while the contract does not state the fact.
func TestKeysThatTheContractDoesNotStateAreNotStale(t *testing.T) {
	file := mustParse(t, "resource: LegacyThing\nmode: existing\nvalidators:\n  inferred: false\ntypes:\n  LegacyThing:\n    fields:\n      name: {required: true}\n      labels: {readOnly: true}\n")
	_, err := validateOpenAPIWith(legacySpec(t), "LegacyThing", model.OperationIDs{}, "sdk", "provider", file)
	if err != nil && strings.Contains(err.Error(), "OVERRIDE_UNUSED") {
		t.Fatalf("err = %v, want no OVERRIDE_UNUSED", err)
	}
}

// An override can say that an id travels in a request body that has no id. The generated call
// could not name the resource, and the override would have no effect. The generator fails closed.
func TestBodyIDOverridesNeedTheIDInTheBody(t *testing.T) {
	spec := strings.Replace(string(legacySpec(t)), "        id:\n          type: string\n          pattern: '^[a-z][a-z0-9_-]*$'\n", "", 1)
	if spec == string(legacySpec(t)) {
		t.Fatal("the test contract did not change: update the replaced text")
	}
	const head = "resource: LegacyThing\nmode: existing\nvalidators:\n  inferred: false\napi:\n  requestWrapper: thing\n"
	tests := map[string]struct {
		api  string
		want string
	}{
		"client-set id":  {"  clientSetID: true\n", "CLIENT_SET_ID_NOT_IN_CREATE"},
		"update id body": {"  updateIDInBody: true\n", "UPDATE_ID_NOT_IN_BODY"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := validateOpenAPIWith([]byte(spec), "LegacyThing", model.OperationIDs{}, "sdk", "provider", mustParse(t, head+test.api))
			if !slices.Contains(eligibilityCodes(t, err), test.want) {
				t.Fatalf("err = %v, want %s", err, test.want)
			}
		})
	}
	// With the id in the bodies, the same overrides are accepted.
	file := mustParse(t, head+"  clientSetID: true\n  updateIDInBody: true\n")
	_, err := validateOpenAPIWith(legacySpec(t), "LegacyThing", model.OperationIDs{}, "sdk", "provider", file)
	for _, code := range eligibilityCodes(t, err) {
		if code == "CLIENT_SET_ID_NOT_IN_CREATE" || code == "UPDATE_ID_NOT_IN_BODY" {
			t.Fatalf("err = %v, want no %s", err, code)
		}
	}
}

// validators.inferred: false removes the limits of the contract. A oneOf group validator states
// the structure of the request, so it stays.
func TestInferredFalseKeepsOneOfGroupValidators(t *testing.T) {
	limit := "stringvalidator.LengthAtLeast(3)"
	group := `stringvalidator.ExactlyOneOf(path.MatchRelative().AtParent().AtName("b"))`
	out := &tfResource{Attributes: []*tfAttr{{
		Name: "a", Kind: "String", ValueKind: "String",
		Validators: []string{limit, group}, GroupValidators: []string{group},
	}}}
	file := mustParse(t, "resource: LegacyThing\nmode: existing\nvalidators:\n  inferred: false\n")
	if err := applyOverrides(out, file); err != nil {
		t.Fatal(err)
	}
	if got := out.Attributes[0].Validators; !slices.Equal(got, []string{group}) {
		t.Fatalf("validators = %v, want only the group validator", got)
	}
}

func TestOverridesMustMatchTheSelectedResource(t *testing.T) {
	file := mustParse(t, strings.Replace(legacyOverrides, "resource: LegacyThing", "resource: OtherThing", 1))
	_, err := validateOpenAPIWith(legacySpec(t), "LegacyThing", model.OperationIDs{}, "sdk", "provider", file)
	if err == nil || !strings.Contains(err.Error(), "OVERRIDE_RESOURCE_MISMATCH") {
		t.Fatalf("err = %v, want OVERRIDE_RESOURCE_MISMATCH", err)
	}
}

// Publish replaces the whole output directory. The overrides file must survive that, and
// a directory that has only this file is a directory that the generator may fill.
func TestPublishKeepsTheOverridesFile(t *testing.T) {
	out := filepath.Join(t.TempDir(), "thing")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, overrides.FileName), []byte("resource: Thing\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	header := "// Code generated by coralogix-iac-codegen v0.0.0. DO NOT EDIT.\n"
	files := map[string][]byte{"a.go": []byte(header + "package thing\n")}
	if err := keepOverrides(Options{OutputDir: out}, files); err != nil {
		t.Fatal(err)
	}
	if string(files[overrides.FileName]) != "resource: Thing\n" {
		t.Fatalf("files = %v", files)
	}
	for round := 0; round < 2; round++ { // the second round replaces a non-empty output
		if err := publish(out, files); err != nil {
			t.Fatalf("round %d: %v", round, err)
		}
		entries, err := os.ReadDir(out)
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		if !slices.Equal(names, []string{"a.go", overrides.FileName}) {
			t.Fatalf("round %d: entries = %v", round, names)
		}
	}
}

func TestPublishRefusesOtherFiles(t *testing.T) {
	out := filepath.Join(t.TempDir(), "thing")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "notes.yaml"), []byte("x: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := publish(out, map[string][]byte{"a.go": []byte("// Code generated by coralogix-iac-codegen v0.0.0. DO NOT EDIT.\npackage thing\n")}); err == nil {
		t.Fatal("publish replaced a directory that has another file")
	}
}
