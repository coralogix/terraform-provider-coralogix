package generator

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/coralogix/coralogix-management-sdk/iac/model"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/overrides"
)

func legacySpec(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "legacy.yaml"))
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
      name: {description: The name., required: true}
      labels: {readEmptyAs: "null"}
      rules: {computed: true}
  LegacyRule:
    fields:
      name: {description: The rule name., required: true}
      kind: {computed: true, default: alpha}
      targets: {computed: true}
  LegacyLabels:
    fields:
      env: {computed: true}
  LegacyTarget:
    fields:
      connectorId: {description: The connector., required: true}
      id: {skip: true}
enums:
  legacy.Kind: {zero: unspecified, values: [ALPHA, BETA], rejected: [GAMMA]}
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

// A name line that gives the Terraform name that the field has anyway changes nothing.
func TestNameLineWithTheDefaultNameIsStale(t *testing.T) {
	file := mustParse(t, "resource: LegacyThing\nmode: existing\nvalidators:\n  inferred: false\ntypes:\n  LegacyThing:\n    fields:\n      name: {name: name}\n")
	_, err := validateOpenAPIWith(legacySpec(t), "LegacyThing", model.OperationIDs{}, "sdk", "provider", file)
	if err == nil || !strings.Contains(err.Error(), "OVERRIDE_UNUSED") || !strings.Contains(err.Error(), `"name" is already the Terraform name of "name"`) {
		t.Fatalf("err = %v, want OVERRIDE_UNUSED for the name line", err)
	}
}

// Two fields that a name line makes share a Terraform name collide, as two API names would.
func TestNameLineCollision(t *testing.T) {
	file := mustParse(t, "resource: SuppressedThing\nmode: existing\nvalidators:\n  inferred: false\napi:\n  requestWrapper: suppressedThing\n  updateIDInBody: true\n  idProperty: uniqueIdentifier\ntypes:\n  SuppressedThing:\n    fields:\n      uniqueIdentifier: {name: id}\n")
	spec, err := os.ReadFile(filepath.Join("testdata", "suppressed.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = validateOpenAPIWith(spec, "SuppressedThing", model.OperationIDs{}, "sdk", "provider", file)
	if err == nil || !strings.Contains(err.Error(), `name "id" is the Terraform name of field "id"`) {
		t.Fatalf("err = %v, want the name id rejected: the version id is not skipped", err)
	}
}

// The id property must exist and is needed in the Update body; the version id field of the same
// object does not stand in for it.
func TestIDPropertyMustBeAField(t *testing.T) {
	file := mustParse(t, "resource: SuppressedThing\nmode: existing\nvalidators:\n  inferred: false\napi:\n  requestWrapper: suppressedThing\n  updateIDInBody: true\n  idProperty: ruleId\ntypes:\n  SuppressedThing:\n    fields:\n      id: {skip: true}\n")
	spec, err := os.ReadFile(filepath.Join("testdata", "suppressed.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = validateOpenAPIWith(spec, "SuppressedThing", model.OperationIDs{}, "sdk", "provider", file)
	if err == nil || !strings.Contains(err.Error(), "UPDATE_ID_NOT_IN_BODY") || !strings.Contains(err.Error(), `"ruleId"`) {
		t.Fatalf("err = %v, want UPDATE_ID_NOT_IN_BODY for ruleId", err)
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
	spec := strings.Replace(string(legacySpec(t)), "        id:\n          type: string\n          pattern: '^[a-z][a-z0-9_-]*$'\n          x-coralogix-presence: true\n", "", 1)
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

func TestLengthAtLeastOverride(t *testing.T) {
	a := &tfAttr{Name: "name", Kind: "String", ValueKind: "String", Required: true, Component: "T", Property: "name"}
	out := &tfResource{Attributes: []*tfAttr{a}}
	file := mustParse(t, "resource: LegacyThing\nmode: existing\nvalidators:\n  inferred: false\ntypes:\n  T:\n    fields:\n      name: {validators: [{lengthAtLeast: 1}]}\n")
	if err := applyOverrides(out, file); err != nil {
		t.Fatal(err)
	}
	if got := a.Validators; !slices.Equal(got, []string{"stringvalidator.LengthAtLeast(1)"}) {
		t.Fatalf("validators = %v, want LengthAtLeast(1)", got)
	}
}

func thingSpec(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// The generator gives a client-set id UseStateForUnknown, so the key on its line changes nothing.
// A key that changes nothing is reported, so a stale line cannot stay in the file.
func TestUseStateForUnknownOnClientSetIDIsRejected(t *testing.T) {
	text := strings.Replace(legacyOverrides, "id: {description: The id.}", "id: {description: The id., useStateForUnknown: true}", 1)
	if text == legacyOverrides {
		t.Fatal("the test overrides did not change: update the replaced text")
	}
	_, err := validateOpenAPIWith(legacySpec(t), "LegacyThing", model.OperationIDs{}, "sdk", "provider", mustParse(t, text))
	if !slices.Contains(eligibilityCodes(t, err), "OVERRIDE_UNUSED") || !strings.Contains(err.Error(), "useStateForUnknown") {
		t.Fatalf("err = %v, want OVERRIDE_UNUSED for useStateForUnknown", err)
	}
}

// Every call after Create names the resource by its id. A resource without the id attribute
// cannot read, update, delete, or import, so the generator rejects the skip with a clear issue.
func TestSkippingTheResourceIDIsRejected(t *testing.T) {
	text := strings.Replace(legacyOverrides, "id: {description: The id.}", "id: {skip: true}", 1)
	if text == legacyOverrides {
		t.Fatal("the test overrides did not change: update the replaced text")
	}
	_, err := validateOpenAPIWith(legacySpec(t), "LegacyThing", model.OperationIDs{}, "sdk", "provider", mustParse(t, text))
	if !slices.Contains(eligibilityCodes(t, err), "RESOURCE_ID_SKIPPED") {
		t.Fatalf("err = %v, want RESOURCE_ID_SKIPPED", err)
	}
}

// Skipping an arm of a oneOf group changes what the group means. The generator rejects it. A group
// of arms only used to give a one-arm group that required the arm. A group next to normal fields
// used to stop the generator with a nil pointer.
func TestSkippingAOneOfArmIsRejected(t *testing.T) {
	const skip = "resource: Thing\nmode: existing\nvalidators:\n  inferred: false\ntypes:\n  ThingConfig:\n    fields:\n      queue: {skip: true}\n"
	mixed := strings.Replace(string(thingSpec(t)), "      required: []\n      properties:\n        http:", "      required: []\n      properties:\n        label: {type: string, x-coralogix-presence: true}\n        http:", 1)
	if mixed == string(thingSpec(t)) {
		t.Fatal("the test contract did not change: update the replaced text")
	}
	tests := map[string][]byte{"only arms": thingSpec(t), "arms next to a field": []byte(mixed)}
	for name, spec := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := validateOpenAPIWith(spec, "Thing", model.OperationIDs{}, "sdk", "provider", mustParse(t, skip))
			if !slices.Contains(eligibilityCodes(t, err), "ONEOF_ARM_SKIPPED") {
				t.Fatalf("err = %v, want ONEOF_ARM_SKIPPED", err)
			}
		})
	}
}

// readEmptyAs is rendered for an object and for a list or set of objects. On another kind it
// would do nothing, so the generator rejects it.
func TestReadEmptyAsOnOtherKindsIsRejected(t *testing.T) {
	for _, field := range []string{"labels", "enabled"} {
		t.Run(field, func(t *testing.T) {
			text := "resource: Thing\nmode: existing\nvalidators:\n  inferred: false\ntypes:\n  Thing:\n    fields:\n      " + field + ": {readEmptyAs: \"null\"}\n"
			_, err := validateOpenAPIWith(thingSpec(t), "Thing", model.OperationIDs{}, "sdk", "provider", mustParse(t, text))
			if err == nil || !strings.Contains(err.Error(), "readEmptyAs") {
				t.Fatalf("err = %v, want an issue about readEmptyAs", err)
			}
		})
	}
}

// readEmptyAs: "null" works on a list of strings: the API returns an empty list for a list that
// the request did not send.
func TestReadEmptyAsOnAStringList(t *testing.T) {
	text := "resource: Thing\nmode: existing\nvalidators:\n  inferred: false\ntypes:\n  Thing:\n    fields:\n      destinations: {readEmptyAs: \"null\"}\n"
	validated, err := validateOpenAPIWith(thingSpec(t), "Thing", model.OperationIDs{}, "sdk", "provider", mustParse(t, text))
	if err != nil {
		t.Fatal(err)
	}
	data, err := buildConvWith(validated.resource, validated.refs, validated.overrides)
	if err != nil {
		t.Fatal(err)
	}
	for _, obj := range data.Objects {
		for _, f := range obj.Fields {
			if f.TFName == "destinations" && obj.Flatten && !f.ReadEmptyAsNull {
				t.Fatalf("%s.destinations does not read an empty list as null", obj.Model)
			}
		}
	}
}

// A value of the contract must be accepted or rejected. A new API value stops the generator until
// someone decides, so it cannot be silently refused or silently accepted.
func TestEnumValueOfTheContractNeedsADecision(t *testing.T) {
	const head = "resource: LegacyThing\nmode: existing\nvalidators:\n  inferred: false\nenums:\n  legacy.Kind:\n    zero: unspecified\n"
	tests := map[string]struct {
		lines string
		want  string
	}{
		"value without a decision": {"    values: [ALPHA, BETA]\n", "ENUM_VALUE_UNDECIDED"},
		"rejected value is gone":   {"    values: [ALPHA, BETA]\n    rejected: [GAMMA, DELTA]\n", "OVERRIDE_UNUSED"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := validateOpenAPIWith(legacySpec(t), "LegacyThing", model.OperationIDs{}, "sdk", "provider", mustParse(t, head+test.lines))
			if !slices.Contains(eligibilityCodes(t, err), test.want) {
				t.Fatalf("err = %v, want %s", err, test.want)
			}
		})
	}
	// With a decision for every value, the line is accepted.
	file := mustParse(t, head+"    values: [ALPHA, BETA]\n    rejected: [GAMMA]\n")
	_, err := validateOpenAPIWith(legacySpec(t), "LegacyThing", model.OperationIDs{}, "sdk", "provider", file)
	for _, code := range eligibilityCodes(t, err) {
		if code == "ENUM_VALUE_UNDECIDED" || code == "OVERRIDE_UNUSED" {
			t.Fatalf("err = %v, want no %s", err, code)
		}
	}
}

// The enum validator takes its values from the enums line, in sorted order. Adding a value to the
// line changes the validator too, so the two lists cannot drift.
func TestEnumValidatorFollowsTheEnumsLine(t *testing.T) {
	attr := &tfAttr{Name: "kind", Kind: "String", ValueKind: "String", EnumSchema: "legacy.Kind"}
	tests := map[string]struct {
		file string
		want string
	}{
		"two values and the zero": {"enums:\n  legacy.Kind: {zero: unspecified, values: [BETA, ALPHA]}\n", `stringvalidator.OneOf("alpha", "beta", "unspecified")`},
		"a new value":             {"enums:\n  legacy.Kind: {zero: unspecified, values: [ALPHA, BETA, GAMMA]}\n", `stringvalidator.OneOf("alpha", "beta", "gamma", "unspecified")`},
		"no zero value":           {"enums:\n  legacy.Kind: {values: [ALPHA]}\n", `stringvalidator.OneOf("alpha")`},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			file := mustParse(t, "resource: LegacyThing\nmode: existing\nvalidators:\n  inferred: false\n"+test.file)
			got, err := enumValidatorExpr(attr, file, false)
			if err != nil || got != test.want {
				t.Fatalf("got %q, %v, want %q", got, err, test.want)
			}
		})
	}
	file := mustParse(t, "resource: LegacyThing\nmode: existing\nvalidators:\n  inferred: false\n")
	if _, err := enumValidatorExpr(&tfAttr{Name: "name", Kind: "String", ValueKind: "String"}, file, false); err == nil {
		t.Fatal("want an error for a field that is not an enum")
	}
}

// enumExceptZero leaves the zero value out of the validator. The conversion map still has it,
// so a response that contains the zero value still converts.
func TestEnumExceptZeroKeepsTheZeroValueInTheMap(t *testing.T) {
	attr := &tfAttr{Name: "chart_name", Kind: "String", ValueKind: "String", EnumSchema: "ChartName"}
	file := mustParse(t, "resource: Thing\nmode: existing\nvalidators:\n  inferred: false\nenums:\n  ChartName: {zero: CHART_NAME_UNSPECIFIED, values: [CHART_NAME_OTEL_INTEGRATION, CHART_NAME_OTEL_ECS_EC2]}\n")
	got, err := enumValidatorExpr(attr, file, true)
	want := `stringvalidator.OneOf("chart_name_otel_ecs_ec2", "chart_name_otel_integration")`
	if err != nil || got != want {
		t.Fatalf("validator = %q, %v, want %q", got, err, want)
	}
	b := &convBuilder{file: file}
	m, err := b.enumMapFor("ChartName", "ChartName", &model.Type{EnumZero: "CHART_NAME_UNSPECIFIED"})
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Values) == 0 || m.Values[0].TF != "CHART_NAME_UNSPECIFIED" || m.Values[0].API != "CHART_NAME_UNSPECIFIED" {
		t.Fatalf("map values = %+v, want the zero value first", m.Values)
	}
}

// computed: false makes a field optional. On a field that the contract makes server-owned, nothing
// would be left: the schema would have no mode, and the Framework cannot build it.
func TestComputedFalseOnAServerOwnedFieldIsRejected(t *testing.T) {
	text := strings.Replace(legacyOverrides, "      id: {description: The id.}\n", "      id: {description: The id.}\n      createTime: {computed: false}\n", 1)
	if text == legacyOverrides {
		t.Fatal("the test overrides did not change: update the replaced text")
	}
	_, err := validateOpenAPIWith(legacySpec(t), "LegacyThing", model.OperationIDs{}, "sdk", "provider", mustParse(t, text))
	if err == nil || !strings.Contains(err.Error(), "without a mode") {
		t.Fatalf("err = %v, want an error about an attribute without a mode", err)
	}
}

// An enums line maps Terraform values to API values for a single enum field. A list, set, or map of
// the enum casts the strings as they are, so the generator rejects the combination.
func TestEnumLineOnAMapIsRejected(t *testing.T) {
	spec := strings.Replace(string(thingSpec(t)), "        tags:\n          type: array\n          uniqueItems: true",
		"        kinds:\n          type: object\n          additionalProperties: {$ref: '#/components/schemas/ThingKind'}\n          x-coralogix-presence: true\n        tags:\n          type: array\n          uniqueItems: true", 1)
	if spec == string(thingSpec(t)) {
		t.Fatal("the test contract did not change: update the replaced text")
	}
	file := mustParse(t, "resource: Thing\nmode: existing\nvalidators:\n  inferred: false\nenums:\n  ThingKind:\n    zero: unspecified\n    values: [THING_KIND_STANDARD, THING_KIND_ADVANCED]\n    rejected: [THING_KIND_P5_OR_UNSPECIFIED]\n")
	_, err := validateOpenAPIWith([]byte(spec), "Thing", model.OperationIDs{}, "sdk", "provider", file)
	if err == nil || !strings.Contains(err.Error(), "not to a map of it") {
		t.Fatalf("err = %v, want an error about an enums line on a map", err)
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

// A line that only changes a text does not say how an omitted value behaves. The generator still
// stops when the contract does not state presence for a field that needs it: env has a default
// that is not "".
func TestWordingOnlyLineDoesNotWaivePresence(t *testing.T) {
	spec := strings.Replace(string(legacySpec(t)), "        env: {type: string}\n", "        env: {type: string, default: prod}\n", 1)
	if spec == string(legacySpec(t)) {
		t.Fatal("the test contract did not change: update the replaced text")
	}
	tests := map[string]struct {
		line      string
		wantIssue bool
	}{
		"wording only":       {"{description: The env.}", true},
		"states a lifecycle": {"{description: The env., computed: true}", false},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			text := strings.Replace(legacyOverrides, "env: {computed: true}", "env: "+test.line, 1)
			_, err := validateOpenAPIWith([]byte(spec), "LegacyThing", model.OperationIDs{}, "sdk", "provider", mustParse(t, text))
			if got := slices.Contains(eligibilityCodes(t, err), "FIELD_PRESENCE_UNKNOWN"); got != test.wantIssue {
				t.Fatalf("FIELD_PRESENCE_UNKNOWN = %v, want %v (err = %v)", got, test.wantIssue, err)
			}
		})
	}
}

// An enum with a zero value (KIND_UNSPECIFIED) needs a zero key. Without it the conversion map has
// no row for the value, and a response that has it fails with Unsupported API value.
func TestEnumZeroValueNeedsAZeroKey(t *testing.T) {
	const head = "resource: LegacyThing\nmode: existing\nvalidators:\n  inferred: false\nenums:\n  legacy.Kind:\n"
	const values = "    values: [ALPHA, BETA]\n    rejected: [GAMMA]\n"
	_, err := validateOpenAPIWith(legacySpec(t), "LegacyThing", model.OperationIDs{}, "sdk", "provider", mustParse(t, head+values))
	if !slices.Contains(eligibilityCodes(t, err), "ENUM_ZERO_UNDECIDED") {
		t.Fatalf("err = %v, want ENUM_ZERO_UNDECIDED", err)
	}
	_, err = validateOpenAPIWith(legacySpec(t), "LegacyThing", model.OperationIDs{}, "sdk", "provider", mustParse(t, head+"    zero: unspecified\n"+values))
	if slices.Contains(eligibilityCodes(t, err), "ENUM_ZERO_UNDECIDED") {
		t.Fatalf("err = %v, want no ENUM_ZERO_UNDECIDED", err)
	}
}

// A missing required list means that no field is required, so a required: []
// line changes nothing and is stale.
func TestEmptyRequiredLineIsUnused(t *testing.T) {
	text := strings.Replace(legacyOverrides, "  LegacyLabels:\n    fields:", "  LegacyLabels:\n    required: []\n    fields:", 1)
	doc, err := model.Load(legacySpec(t))
	if err != nil {
		t.Fatal(err)
	}
	component, report := resolveResource(doc, "LegacyThing")
	if len(report) != 0 {
		t.Fatal(report)
	}
	report = overrideIssues(doc, component, mustParse(t, text))
	if len(report) != 1 || report[0].Code != "OVERRIDE_UNUSED" || report[0].Location != "behavior-overrides.yaml:types.LegacyLabels.required" {
		t.Fatalf("report = %v, want OVERRIDE_UNUSED for types.LegacyLabels.required", report)
	}
}

func TestNumericOverrideDefault(t *testing.T) {
	tests := []struct {
		kind  string
		value any
		want  string
	}{
		{"Int64", 0, "int64default.StaticInt64(0)"},
		{"Int64", -7, "int64default.StaticInt64(-7)"},
		{"Int32", 2147483647, "int32default.StaticInt32(2147483647)"},
		{"Float64", 0.5, "float64default.StaticFloat64(0.5)"},
		{"Float64", 3, "float64default.StaticFloat64(3)"},
		{"Float64", 1e6, "float64default.StaticFloat64(1000000)"},
		{"Float32", 0.1, "float32default.StaticFloat32(0.1)"},
	}
	for _, test := range tests {
		t.Run(test.want, func(t *testing.T) {
			a := &tfAttr{Name: "n", Kind: test.kind, ValueKind: test.kind, Optional: true}
			if err := applyField(a, overrides.Field{Default: test.value}, nil); err != nil {
				t.Fatal(err)
			}
			if a.Default != test.want {
				t.Fatalf("default = %s, want %s", a.Default, test.want)
			}
			if want := []string{strings.ToLower(test.kind) + "default"}; !slices.Equal(defaultPackages([]*tfAttr{a}), want) {
				t.Fatalf("packages = %v, want %v", defaultPackages([]*tfAttr{a}), want)
			}
			if _, ok := defaultState(a); !ok {
				t.Fatal("the acceptance test does not see the default")
			}
		})
	}
}

func TestNumericOverrideDefaultMustFit(t *testing.T) {
	tests := []struct {
		kind  string
		value any
	}{
		{"Int32", 2147483648},
		{"Int64", 0.5},
		{"String", 0},
		{"Bool", 1},
		{"Float32", 1e39},
		{"Float64", math.Inf(1)},
		{"Float64", math.NaN()},
	}
	for _, test := range tests {
		a := &tfAttr{Name: "n", Kind: test.kind, ValueKind: test.kind, Optional: true}
		if err := applyField(a, overrides.Field{Default: test.value}, nil); err == nil {
			t.Errorf("%s default %v: no error, want one", test.kind, test.value)
		}
	}
}

// In existing mode, a default line replaces the declared server default. Without one, the
// declared default applies, also when the field has another line.
func TestExistingModeUsesTheDeclaredDefaultWithoutADefaultLine(t *testing.T) {
	declared := "0"
	field := &model.ResourceField{Name: "priorityOrder", Create: &model.Attrs{Default: &declared}, Get: &model.Attrs{Required: true}}
	tests := map[string]struct {
		policy model.Policy
		want   bool
	}{
		"new resource":         {model.Policy{}, true},
		"no line":              {model.Policy{Existing: true}, true},
		"line without default": {model.Policy{Existing: true, Released: []string{"Group.priorityOrder"}}, true},
		"default line":         {model.Policy{Existing: true, Released: []string{"Group.priorityOrder"}, Defaults: []string{"Group.priorityOrder"}}, false},
	}
	for name, test := range tests {
		r := &model.Resource{Name: "Group", Policy: test.policy}
		if got := serverDefault(r, field); got != test.want {
			t.Errorf("%s: serverDefault = %t, want %t", name, got, test.want)
		}
	}
}

// A computed: false line states a mode that a server default cannot have, so it replaces the
// server default of the contract, as a default line does. Without either, the contract default
// applies. A required: true line on such a field is unused: the contract already requires it in Get.
func TestModeLinesReplaceTheContractDefault(t *testing.T) {
	spec := archivedSpec(t)
	request := "                name: {type: string, minLength: 1}\n"
	spec = strings.ReplaceAll(spec, request, request+"                priority: {type: integer, format: int64, default: 0, x-coralogix-presence: true}\n")
	spec = strings.Replace(spec, "      required: [id, name]\n", "      required: [id, name, priority]\n", 1)
	response := "        id: {type: string, readOnly: true}\n        name: {type: string, minLength: 1}\n"
	spec = strings.Replace(spec, response, response+"        priority: {type: integer, format: int64}\n", 1)
	if strings.Count(spec, "priority:") != 3 {
		t.Fatal("the test contract did not change: update the replaced text")
	}
	head, err := os.ReadFile(filepath.Join("testdata", "archived-overrides.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	lineOf := func(text string) *overrides.File {
		if text == "" {
			return mustParse(t, string(head))
		}
		return mustParse(t, string(head)+"    fields:\n      priority: "+text+"\n")
	}
	tests := map[string]struct {
		line          string
		computed      bool
		serverDefault bool
		staticDefault string
	}{
		"no line":        {"", true, true, ""},
		"other line":     {"{description: The priority.}", true, true, ""},
		"not computed":   {"{computed: false}", false, false, ""},
		"static default": {"{computed: true, default: 0}", true, false, "int64default.StaticInt64(0)"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			validated, err := validateOpenAPIWith([]byte(spec), "ArchivedThing", model.OperationIDs{}, "candidate.invalid/coralogix-management-sdk", providerModulePath, lineOf(test.line))
			if err != nil {
				t.Fatal(reportOf(t, err))
			}
			out, err := buildTFResourceWith(validated.resource, "generated", validated.overrides)
			if err != nil {
				t.Fatal(err)
			}
			i := slices.IndexFunc(out.Attributes, func(a *tfAttr) bool { return a.Name == "priority" })
			if i < 0 {
				t.Fatal("no priority attribute")
			}
			a := out.Attributes[i]
			hasServerDefault := slices.ContainsFunc(a.Modifiers, func(m string) bool { return strings.HasPrefix(m, "serverDefaultModifier") })
			if a.Computed != test.computed || hasServerDefault != test.serverDefault || a.Default != test.staticDefault {
				t.Fatalf("computed = %t, server default = %t, static default = %q; want %t, %t, %q",
					a.Computed, hasServerDefault, a.Default, test.computed, test.serverDefault, test.staticDefault)
			}
		})
	}
	_, err = validateOpenAPIWith([]byte(spec), "ArchivedThing", model.OperationIDs{}, "candidate.invalid/coralogix-management-sdk", providerModulePath, lineOf("{required: true}"))
	if !reportHas(requireReport(t, err), "OVERRIDE_UNUSED", "behavior-overrides.yaml:types.ArchivedThing.fields.priority") {
		t.Fatalf("required line: err = %v, want OVERRIDE_UNUSED", err)
	}
}

// legacySpecWithReadOnlyCopy adds a read-only field to the legacy resource that holds the
// component of a field that the user sets, as an API returns a server copy of a request value.
func legacySpecWithReadOnlyCopy(t *testing.T, component string) []byte {
	t.Helper()
	old := "        createTime: {type: string, format: date-time, readOnly: true}\n"
	spec := string(legacySpec(t))
	if !strings.Contains(spec, old) {
		t.Fatal("testdata/legacy.yaml has no createTime")
	}
	return []byte(strings.Replace(spec, old, old+"        lastRule: {allOf: [{$ref: '#/components/schemas/"+component+"'}], readOnly: true}\n", 1))
}

// required: true states the mode of the field that the user sets. The read-only copy of the
// same component stays computed.
func TestRequiredLineKeepsAReadOnlyCopyComputed(t *testing.T) {
	spec := legacySpecWithReadOnlyCopy(t, "LegacyRule")
	validated, err := validateOpenAPIWith(spec, "LegacyThing", model.OperationIDs{}, "sdk", "provider", mustParse(t, legacyOverrides))
	if err != nil {
		t.Fatal(err)
	}
	out, err := buildTFResourceWith(validated.resource, "generated", validated.overrides)
	if err != nil {
		t.Fatal(err)
	}
	managed := attrAt(t, out.Attributes, "rules", "name")
	copied := attrAt(t, out.Attributes, "last_rule", "name")
	if !managed.Required || managed.Computed {
		t.Errorf("rules.name required=%t computed=%t, want required", managed.Required, managed.Computed)
	}
	if copied.Required || copied.Optional || !copied.Computed || copied.Description != "The rule name." {
		t.Errorf("last_rule.name required=%t optional=%t computed=%t description=%q, want computed with the line's text", copied.Required, copied.Optional, copied.Computed, copied.Description)
	}
	// kind: {computed: true, default: alpha}. A default on the read-only copy would plan a value
	// that the server does not set.
	if kind := attrAt(t, out.Attributes, "rules", "kind"); kind.Default == "" {
		t.Errorf("rules.kind has no default, want the line's default")
	}
	if kind := attrAt(t, out.Attributes, "last_rule", "kind"); kind.Default != "" || len(kind.Modifiers) != 0 {
		t.Errorf("last_rule.kind default=%q modifiers=%v, want neither", kind.Default, kind.Modifiers)
	}
}

// A component that only a read-only field holds has no field that the user sets, so
// required: true on it is still an error.
func TestRequiredLineOnlyOnAReadOnlyCopyIsAnError(t *testing.T) {
	spec := legacySpecWithReadOnlyCopy(t, "LegacyLabels")
	spec = []byte(strings.Replace(string(spec), "        labels: {$ref: '#/components/schemas/LegacyLabels'}\n", "", 1))
	file := mustParse(t, strings.Replace(legacyOverrides, "      labels: {readEmptyAs: \"null\"}\n", "", 1))
	file.Types["LegacyLabels"].Fields["env"] = overrides.Field{Required: true}
	_, err := validateOpenAPIWith(spec, "LegacyThing", model.OperationIDs{}, "sdk", "provider", file)
	if err == nil || !strings.Contains(err.Error(), "required: true needs a field that the user sets") {
		t.Fatalf("err = %v, want an error about a required and computed field", err)
	}
}

// attrAt returns the attribute at the Terraform path names.
func attrAt(t *testing.T, attrs []*tfAttr, names ...string) *tfAttr {
	t.Helper()
	var a *tfAttr
	for _, name := range names {
		i := slices.IndexFunc(attrs, func(a *tfAttr) bool { return a.Name == name })
		if i < 0 {
			t.Fatalf("no attribute %q in %v", name, names)
		}
		a = attrs[i]
		attrs = a.Attributes
	}
	return a
}

// serverSetID: a released API returns a server-set id that its contract does not require. The
// generated flatten then checks the id, as it does for a client-set one. An id that Get requires
// needs no check, and serverSetID on it is an error.
func TestExistingResourceChecksAnOptionalServerSetID(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "legacy-overrides.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Replace(string(data), "  clientSetID: true\n", "  serverSetID: true\n", 1)
	text = strings.Replace(text, "      id:\n", "      id:\n        readOnly: true\n", 1)
	if strings.Count(text, "serverSetID")+strings.Count(text, "readOnly: true") != 2 {
		t.Fatal("testdata/legacy-overrides.yaml has no clientSetID line or no id line")
	}
	overridesPath := filepath.Join(t.TempDir(), "behavior-overrides.yaml")
	if err := os.WriteFile(overridesPath, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	input, loadDir := syntheticInput(t)
	input.OpenAPI = legacySpec(t)
	out := filepath.Join(t.TempDir(), "legacything")
	if err := generateFromInput(Options{Resource: "LegacyThing", OutputDir: out, OverridesPath: overridesPath}, input, loadDir); err != nil {
		t.Fatal(err)
	}
	if convert, err := os.ReadFile(filepath.Join(out, "convert.go")); err != nil || !strings.Contains(string(convert), `"Missing id"`) {
		t.Fatalf("convert.go has no id check (err %v)", err)
	}
	if convert, err := os.ReadFile(filepath.Join(generateCustomThing(t), "convert.go")); err != nil || strings.Contains(string(convert), `"Missing id"`) {
		t.Fatalf("customthing convert.go checks an id that Get requires (err %v)", err)
	}
}

func TestServerSetIDOnARequiredIDIsAnError(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "customthing-overrides.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	spec, err := os.ReadFile(filepath.Join("testdata", "customthing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Replace(string(data), "  updateIDInBody: true\n", "  updateIDInBody: true\n  serverSetID: true\n", 1)
	_, err = validateOpenAPIWith(spec, "CustomThing", model.OperationIDs{}, "sdk", "provider", mustParse(t, text))
	if got := eligibilityCodes(t, err); !slices.Contains(got, "SERVER_SET_ID_UNNEEDED") {
		t.Fatalf("codes = %v, want SERVER_SET_ID_UNNEEDED", got)
	}
}

func TestClientAndServerSetIDAreExclusive(t *testing.T) {
	_, err := overrides.Parse([]byte("resource: Thing\nmode: existing\nvalidators:\n  inferred: false\napi:\n  clientSetID: true\n  serverSetID: true\n"))
	if err == nil || !strings.Contains(err.Error(), "serverSetID") {
		t.Fatalf("err = %v, want an error about clientSetID and serverSetID", err)
	}
}

// A group holds top-level fields that the user sets and that a full replace sends. The generator
// reports any other use, because the renderer has no plan logic for it.
func TestGroupsRejectFieldsThatTheyCannotHold(t *testing.T) {
	sharedSpec, err := os.ReadFile(filepath.Join("testdata", "sharedthing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	const shared = "resource: SharedThing\nmode: existing\nvalidators:\n  inferred: false\napi:\n  updateIDInBody: true\n  serverSetID: true\ntypes:\n  SharedThing:\n    fields:\n      id: {readOnly: true}\n      name: {required: true}\n      frame: {required: true, validators: [{enum: true}]}\nenums:\n  SharedFrame: {zero: unspecified, values: [SHARED_FRAME_ONE_DAY, SHARED_FRAME_SEVEN_DAYS]}\n"
	tests := map[string]struct {
		spec     []byte
		resource string
		text     string
		want     string
	}{
		"patch update":                  {thingSpec(t), "Thing", "resource: Thing\nmode: existing\nvalidators:\n  inferred: false\ngroups:\n  meta: {fields: [description]}\n", "full-replace Update"},
		"no such field":                 {sharedSpec, "SharedThing", shared + "groups:\n  meta: {fields: [missing]}\n", "no top-level field"},
		"split oneOf":                   {sharedSpec, "SharedThing", shared + "groups:\n  a: {fields: [byCount]}\n  b: {fields: [byRatio]}\n", "must all be in one group"},
		"requireOne without arms":       {sharedSpec, "SharedThing", shared + "groups:\n  meta:\n    fields: [name]\n    requireOne: true\n", "requireOne needs the arms"},
		"group named as a field":        {sharedSpec, "SharedThing", shared + "groups:\n  name: {fields: [frame]}\n", "also the top-level attribute name"},
		"requireOne with another field": {sharedSpec, "SharedThing", shared + "groups:\n  measure:\n    fields: [byCount, byRatio, name]\n    requireOne: true\n", "and no other field"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			validated, err := validateOpenAPIWith(test.spec, test.resource, model.OperationIDs{}, "sdk", "provider", mustParse(t, test.text))
			if err == nil {
				_, err = buildTFResourceWith(validated.resource, "generated", validated.overrides)
			}
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err = %v, want it to contain %q", err, test.want)
			}
		})
	}
}

// A between bound of an integer attribute must be one the validator states exactly. A fraction
// would be cut, so [0.5, 10] would accept 0, and a number past 2^53 has lost its last digits.
func TestBetweenOnIntegersNeedsExactBounds(t *testing.T) {
	tests := []struct {
		kind    string
		between []float64
		want    string
	}{
		{"Int32", []float64{0.5, 10}, ""},
		{"Int32", []float64{0, 2147483648}, ""},
		{"Int64", []float64{0, 1 << 54}, ""},
		{"Int64", []float64{-1.5, 3}, ""},
		{"Int64", []float64{-(1 << 53), 1 << 53}, "int64validator.Between(-9007199254740992, 9007199254740992)"},
		{"Int32", []float64{1, 10}, "int32validator.Between(1, 10)"},
		{"Float32", []float64{0, 0.5}, "float32validator.Between(0.0, 0.5)"},
	}
	for _, test := range tests {
		a := &tfAttr{Name: "n", Kind: test.kind, ValueKind: test.kind, Optional: true}
		got, err := validatorExpr(a, overrides.Validator{Between: test.between}, nil)
		if test.want == "" {
			if err == nil {
				t.Errorf("%s between %v = %q, want an error", test.kind, test.between, got)
			}
			continue
		}
		if err != nil || got != test.want {
			t.Errorf("%s between %v = %q, %v, want %q", test.kind, test.between, got, err, test.want)
		}
	}
}

// An exactlyOneOf or atLeastOneOf name must be another attribute of the same object. The
// validator renders it into a path, so a typo compiles and fails only on a configuration.
func TestSiblingValidatorTargetsMustExist(t *testing.T) {
	a := &tfAttr{Name: "static_values"}
	siblings := []*tfAttr{a, {Name: "label_keys"}}
	tests := []struct {
		v    overrides.Validator
		fail bool
	}{
		{overrides.Validator{ExactlyOneOf: []string{"label_keys"}}, false},
		{overrides.Validator{AtLeastOneOf: []string{"label_keys"}}, false},
		{overrides.Validator{ExactlyOneOf: []string{"labelKeys"}}, true},
		{overrides.Validator{AtLeastOneOf: []string{"missing"}}, true},
		{overrides.Validator{ExactlyOneOf: []string{"static_values"}}, true},
	}
	for _, test := range tests {
		err := checkSiblingTargets(a, siblings, []overrides.Validator{test.v})
		if (err != nil) != test.fail {
			t.Errorf("checkSiblingTargets(%+v) = %v, want error %v", test.v, err, test.fail)
		}
	}
}

// A requestValueBy value is not an attribute, so an update mask cannot hold it: switching the
// object that sets it would send the new value in the body and leave it out of the mask.
// The generator refuses the line on a resource that Update patches with a mask.
func TestRequestValueByIsRefusedWithAnUpdateMask(t *testing.T) {
	text := "resource: Thing\nmode: existing\nvalidators:\n  inferred: false\ntypes:\n  Thing:\n    fields:\n      kind: {requestValueBy: {config: THING_KIND_STANDARD, spec: THING_KIND_ADVANCED}}\n"
	// The update body also takes kind, so Update sends the derived value.
	spec := strings.Replace(string(thingSpec(t)), "              title: UpdateThingRequest\n              type: object\n              required: []\n              properties:\n",
		"              title: UpdateThingRequest\n              type: object\n              required: []\n              properties:\n                kind:\n                  $ref: '#/components/schemas/ThingKind'\n", 1)
	if spec == string(thingSpec(t)) {
		t.Fatal("the test spec did not change: update the replaced text")
	}
	_, err := validateOpenAPIWith([]byte(spec), "Thing", model.OperationIDs{}, "sdk", "provider", mustParse(t, text))
	if err == nil || !strings.Contains(err.Error(), "requestValueBy needs an Update that replaces the resource") {
		t.Fatalf("err = %v, want requestValueBy refused for a masked Update", err)
	}
}

// valueNames names the numbers of an integer list or set. On any other field it is an error.
func TestValueNamesNeedIntegerCollection(t *testing.T) {
	err := generateSuppressedWith(t, "timezone: {name: time_zone}", "timezone: {name: time_zone, valueNames: {1: UTC}}")
	if err == nil || !strings.Contains(err.Error(), "valueNames needs a list or set of integers") {
		t.Fatalf("err = %v, want valueNames needs a list or set of integers", err)
	}
}

// asBool shows an object without fields. An object with fields is an error.
func TestAsBoolNeedsAnEmptyObject(t *testing.T) {
	spec, err := os.ReadFile(filepath.Join("testdata", "suppressed.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	file := mustParse(t, "resource: SuppressedThing\nmode: existing\nvalidators:\n  inferred: false\napi:\n  requestWrapper: suppressedThing\n  updateIDInBody: true\n  serverSetID: true\n  idProperty: uniqueIdentifier\ntypes:\n  SuppressedThing:\n    fields:\n      uniqueIdentifier: {readOnly: true, name: id}\n      id: {skip: true}\n  Repeat:\n    fields:\n      dynamic: {asBool: true}\n")
	_, err = validateOpenAPIWith(spec, "SuppressedThing", model.OperationIDs{}, "sdk", "provider", file)
	if err == nil || !strings.Contains(err.Error(), "asBool needs an object without fields") {
		t.Fatalf("err = %v, want asBool needs an object without fields", err)
	}
}

// defaultArm marks one arm of a oneOf group in a nested object: a wrapped list of strings that
// reads empty as null. An object arm would read back as an object of nulls where the plan has null.
func TestDefaultArmErrors(t *testing.T) {
	tests := map[string]struct{ from, to, want string }{
		"two arms of one group": {"ids: {defaultArm: true, readEmptyAs: \"null\"}", "ids: {defaultArm: true, readEmptyAs: \"null\"}\n      labels: {defaultArm: true}", "ids and labels are arms of one group"},
		"no arm":                {"timezone: {name: time_zone}", "timezone: {name: time_zone, defaultArm: true}", "timezone is no oneOf arm"},
		"top level":             {"window: {name: time_frame}", "window: {name: time_frame, defaultArm: true}", "nested object only"},
		"object arm":            {"ids: {defaultArm: true, readEmptyAs: \"null\"}", "ids: {readEmptyAs: \"null\"}\n      labels: {defaultArm: true}", "defaultArm needs a list of strings in wrapper objects"},
		"empty list not null":   {"ids: {defaultArm: true, readEmptyAs: \"null\"}", "ids: {defaultArm: true}", "defaultArm needs a list of strings in wrapper objects"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			err := generateSuppressedWith(t, test.from, test.to)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err = %v, want %q", err, test.want)
			}
		})
	}
}

// generateSuppressedWith generates the suppressedthing resource with one change of its
// behavior-overrides file.
func generateSuppressedWith(t *testing.T, from, to string) error {
	t.Helper()
	spec, err := os.ReadFile(filepath.Join("testdata", "suppressed.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	text, err := os.ReadFile(filepath.Join("testdata", "suppressed-overrides.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	changed := strings.Replace(string(text), from, to, 1)
	if changed == string(text) {
		t.Fatalf("%q is not in suppressed-overrides.yaml", from)
	}
	overridesPath := filepath.Join(t.TempDir(), "overrides.yaml")
	if err := os.WriteFile(overridesPath, []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}
	input, loadDir := syntheticInput(t)
	input.OpenAPI = spec
	options := Options{Resource: "SuppressedThing", OutputDir: filepath.Join(t.TempDir(), "out"), OverridesPath: overridesPath}
	return generateFromInput(options, input, loadDir)
}

// The name collision check leaves out a fixed request value, which has no attribute.
func TestNameLineCollisionsFollowTheRenderedFields(t *testing.T) {
	spec, err := os.ReadFile(filepath.Join("testdata", "suppressed.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	const head = "resource: SuppressedThing\nmode: existing\nvalidators:\n  inferred: false\napi:\n  requestWrapper: suppressedThing\n  updateIDInBody: true\n  serverSetID: true\n  idProperty: uniqueIdentifier\ntypes:\n  SuppressedThing:\n    fields:\n      uniqueIdentifier: {readOnly: true, name: id}\n      id: {skip: true}\n"
	fixed := head + "      scheduleOperation: {requestValue: THING_OPERATION_MUTE}\n      name: {name: schedule_operation}\n"
	_, err = validateOpenAPIWith(spec, "SuppressedThing", model.OperationIDs{}, "sdk", "provider", mustParse(t, fixed))
	if err != nil && (strings.Contains(err.Error(), "NAME_COLLISION") || strings.Contains(err.Error(), "is the Terraform name of field")) {
		t.Fatalf("a fixed request value has no attribute, so it collides with nothing: %v", err)
	}
}

// An idProperty that names the Get path parameter changes nothing.
func TestIDPropertyThatIsThePathParameterIsUnneeded(t *testing.T) {
	spec, err := os.ReadFile(filepath.Join("testdata", "suppressed.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	file := mustParse(t, "resource: SuppressedThing\nmode: existing\nvalidators:\n  inferred: false\napi:\n  requestWrapper: suppressedThing\n  updateIDInBody: true\n  idProperty: thing_id\n")
	_, err = validateOpenAPIWith(spec, "SuppressedThing", model.OperationIDs{}, "sdk", "provider", file)
	if err == nil || !strings.Contains(err.Error(), "ID_PROPERTY_UNNEEDED") {
		t.Fatalf("err = %v, want ID_PROPERTY_UNNEEDED", err)
	}
}

// An id property that Create does not send, and that Update and Get have, is the server-set id:
// computed, as for an id named like the path parameter.
func TestIDPropertyMissingFromCreateIsComputed(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "suppressed.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	spec := string(data)
	createRef := "                suppressedThing: {$ref: '#/components/schemas/SuppressedThing'}\n"
	if i := strings.Index(spec, createRef); i < 0 {
		t.Fatal("no Create body ref in suppressed.yaml")
	} else {
		spec = spec[:i] + strings.Replace(createRef, "SuppressedThing'", "SuppressedThingDraft'", 1) + spec[i+len(createRef):]
	}
	// Without serverSetID, Get must require the id.
	spec = strings.Replace(spec, "      required: [name, scheduleOperation]\n      properties:\n        uniqueIdentifier:", "      required: [uniqueIdentifier, name, scheduleOperation]\n      properties:\n        uniqueIdentifier:", 1)
	spec = strings.Replace(spec, "uniqueIdentifier: {type: string, x-coralogix-presence: true,", "uniqueIdentifier: {type: string, minLength: 1, x-coralogix-presence: true,", 1)
	spec += `    SuppressedThingDraft:
      type: object
      required: [name, scheduleOperation]
      properties:
        id: {type: string, x-coralogix-presence: true, description: The version id of the thing.}
        name: {type: string, minLength: 1}
        window: {$ref: '#/components/schemas/Window'}
        repeat: {$ref: '#/components/schemas/Repeat'}
        filter: {$ref: '#/components/schemas/Filter'}
        scheduleOperation: {$ref: '#/components/schemas/ThingOperation'}
        priority: {type: integer, format: int32, x-coralogix-presence: true}
`
	file := mustParse(t, "resource: SuppressedThing\nmode: existing\nvalidators:\n  inferred: false\napi:\n  requestWrapper: suppressedThing\n  updateIDInBody: true\n  idProperty: uniqueIdentifier\nunwrap: [Ids]\ntypes:\n  SuppressedThing:\n    fields:\n      uniqueIdentifier: {name: id}\n      id: {skip: true}\n  SuppressedThingDraft:\n    fields:\n      id: {skip: true}\n")
	validated, err := validateOpenAPIWith([]byte(spec), "SuppressedThing", model.OperationIDs{}, "sdk", "provider", file)
	if err != nil {
		t.Fatalf("an id property that only Update and Get have must be computed: %v", err)
	}
	for _, f := range validated.resource.Fields {
		if f.Name == "uniqueIdentifier" && f.Behavior != model.Computed {
			t.Fatalf("uniqueIdentifier is %s, want computed", f.Behavior)
		}
	}
}

// The update mask of a PATCH names the arm that the plan selects, not the arm that expand adds, so a
// default arm needs a full replace.
func TestDefaultArmNeedsAFullReplace(t *testing.T) {
	overridesPath := filepath.Join(t.TempDir(), "overrides.yaml")
	text := "resource: Thing\nmode: existing\nvalidators:\n  inferred: false\ntypes:\n  ThingConfig:\n    fields:\n      http: {defaultArm: true}\n"
	if err := os.WriteFile(overridesPath, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	input, loadDir := syntheticInput(t)
	err := generateFromInput(Options{Resource: "Thing", OutputDir: filepath.Join(t.TempDir(), "out"), OverridesPath: overridesPath}, input, loadDir)
	if err == nil || !strings.Contains(err.Error(), "needs a full-replace Update") {
		t.Fatalf("err = %v, want defaultArm needs a full-replace Update", err)
	}
}

// A group of a nested component holds fields of that component, every arm of a oneOf or none, and
// needs a full replace, as a top-level group does.
func TestNestedGroupErrors(t *testing.T) {
	const from = "        fields: [daily, weekly]\n"
	tests := map[string]struct{ to, want string }{
		"no such field":      {"        fields: [daily, weekly, monthly]\n", `component "Dynamic" has no field "monthly"`},
		"split oneOf":        {"        fields: [daily]\n", "must all be in one group, or in none"},
		"requireOne non-arm": {"        fields: [every, daily, weekly]\n        requireOne: true\n", "types.Dynamic.groups.frequency: requireOne needs the arms of one oneOf of Dynamic"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			err := generateSuppressedWith(t, from, test.to)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err = %v, want %q", err, test.want)
			}
		})
	}
	t.Run("misspelled type", func(t *testing.T) {
		err := generateSuppressedWith(t, "  Dynamic:\n    groups:\n", "  Dyanmic:\n    groups:\n")
		if err == nil || !strings.Contains(err.Error(), `no component "Dyanmic"`) {
			t.Fatalf("err = %v, want no component Dyanmic", err)
		}
	})
	t.Run("type of no object of the resource", func(t *testing.T) {
		// The component is in the contract, so only the builder sees that the groups go nowhere.
		data, err := os.ReadFile(filepath.Join("testdata", "suppressed.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		spec := strings.Replace(string(data), "    Daily:\n", "    Unused:\n      type: object\n      properties:\n        a: {type: string}\n    Daily:\n", 1)
		text, err := os.ReadFile(filepath.Join("testdata", "suppressed-overrides.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		changed := strings.Replace(string(text), "  Dynamic:\n    groups:\n", "  Unused:\n    groups:\n      g: {fields: [a]}\n  Dynamic:\n    groups:\n", 1)
		validated, err := validateOpenAPIWith([]byte(spec), "SuppressedThing", model.OperationIDs{}, "sdk", "provider", mustParse(t, changed))
		if err == nil {
			_, err = buildTFResourceWith(validated.resource, "generated", validated.overrides)
		}
		if err == nil || !strings.Contains(err.Error(), "types.Unused.groups: no object of the resource is Unused") {
			t.Fatalf("err = %v, want the unused groups reported", err)
		}
	})
	t.Run("named as a field", func(t *testing.T) {
		err := generateSuppressedWith(t, "      frequency:\n", "      every:\n")
		if err == nil || !strings.Contains(err.Error(), "the name is also the attribute every") {
			t.Fatalf("err = %v, want the name is also the attribute every", err)
		}
	})
	t.Run("patch update", func(t *testing.T) {
		overridesPath := filepath.Join(t.TempDir(), "overrides.yaml")
		text := "resource: Thing\nmode: existing\nvalidators:\n  inferred: false\ntypes:\n  ThingConfig:\n    groups:\n      kind: {fields: [http, queue]}\n"
		if err := os.WriteFile(overridesPath, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		input, loadDir := syntheticInput(t)
		err := generateFromInput(Options{Resource: "Thing", OutputDir: filepath.Join(t.TempDir(), "out"), OverridesPath: overridesPath}, input, loadDir)
		if err == nil || !strings.Contains(err.Error(), "a group needs a full-replace Update") {
			t.Fatalf("err = %v, want a group needs a full-replace Update", err)
		}
	})
}

// A group model struct takes the name of its component and the group. When another model has that
// name, the generator reports it instead of sharing one struct.
func TestNestedGroupModelNameClash(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "suppressed.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	// The weekly arm's component becomes DynamicFrequency, whose model is the model name of the
	// frequency group of Dynamic.
	spec := strings.ReplaceAll(string(data), "schemas/Weekly'", "schemas/DynamicFrequency'")
	spec = strings.Replace(spec, "    Weekly:\n", "    DynamicFrequency:\n", 1)
	overrides, err := os.ReadFile(filepath.Join("testdata", "suppressed-overrides.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	validated, err := validateOpenAPIWith([]byte(spec), "SuppressedThing", model.OperationIDs{}, "sdk", "provider", mustParse(t, string(overrides)))
	if err == nil {
		_, err = buildTFResourceWith(validated.resource, "generated", validated.overrides)
	}
	if err == nil || !strings.Contains(err.Error(), "the model struct DynamicFrequencyModel is made by") ||
		!strings.Contains(err.Error(), "the component DynamicFrequency") || !strings.Contains(err.Error(), "types.Dynamic.groups.frequency") {
		t.Fatalf("err = %v, want a model struct clash", err)
	}
}

// terraformType keeps a released Int64 on an int32 field, or a list or set of int32 only.
func TestTerraformTypeNeedsAnInt32Field(t *testing.T) {
	err := generateSuppressedWith(t, "timezone: {name: time_zone}", "timezone: {name: time_zone, terraformType: Int64}")
	if err == nil || !strings.Contains(err.Error(), "terraformType Int64 needs an int32") {
		t.Fatalf("err = %v, want terraformType Int64 needs an int32 field", err)
	}
}

// A Create response that holds only the id, and an id path parameter with another name than the
// id field, need the idField and createResponseID lines. A line that names nothing is an error.
func TestFolderBodyIDOverrides(t *testing.T) {
	spec, err := os.ReadFile(filepath.Join("testdata", "folderthing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	const head = "resource: FolderThing\nmode: existing\nvalidators:\n  inferred: false\ntypes:\n  FolderThing:\n    fields:\n      id: {readOnly: true}\napi:\n  requestWrapper: thing\n  updateIDInBody: true\n  serverSetID: true\n"
	tests := map[string]struct {
		api  string
		want string
	}{
		"no lines":            {"", "RESOURCE_LIFECYCLE_INCOMPLETE"},
		"no id property":      {"  createResponseID: thingId\n", "UPDATE_ID_NOT_IN_BODY"},
		"unknown id property": {"  idProperty: key\n  createResponseID: thingId\n", "RESOURCE_LIFECYCLE_INCOMPLETE"},
		"unknown response id": {"  idProperty: id\n  createResponseID: key\n", "RESOURCE_LIFECYCLE_INCOMPLETE"},
		"valid lines":         {"  idProperty: id\n  createResponseID: thingId\n", ""},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := validateOpenAPIWith(spec, "FolderThing", model.OperationIDs{}, "sdk", "provider", mustParse(t, head+test.api))
			if test.want == "" {
				if err != nil && !strings.Contains(err.Error(), "SDK_") {
					t.Fatalf("err = %v, want none", err)
				}
				return
			}
			if !slices.Contains(eligibilityCodes(t, err), test.want) {
				t.Fatalf("err = %v, want %s", err, test.want)
			}
		})
	}
}
