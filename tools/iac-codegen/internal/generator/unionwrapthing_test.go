package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coralogix/coralogix-management-sdk/iac/model"
)

// The golden output of a released resource whose API holds every field in one object, with the
// arms of a oneOf next to common fields, and does not require that object in the requests or the
// response. The object is inlined, so its fields and its arms are top-level attributes, and the
// arms form one group.
func TestGoldenOutputUnionWrapThing(t *testing.T) {
	out := generateUnionWrapThing(t)
	golden := filepath.Join("testdata", "golden", "unionwrapthing")
	if *updateGolden {
		if err := os.RemoveAll(golden); err != nil {
			t.Fatal(err)
		}
		if err := copyDirectory(out, golden); err != nil {
			t.Fatal(err)
		}
	}
	if diff := compareDirectories(golden, out); diff != "" {
		t.Fatalf("golden output differs; run go test ./internal/generator -run TestGoldenOutputUnionWrapThing -update:\n%s", diff)
	}
}

// Create and Update send the inlined object with its fields and the selected arm, and Create reads
// the object back. A response without the object has no resource.
func TestUnionWrapThingRuntimeSemantics(t *testing.T) {
	input, _ := syntheticInput(t)
	out := generateUnionWrapThing(t)
	semantics, err := os.ReadFile(filepath.Join("testdata", "unionwrapthing_semantics_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "semantics_test.go"), semantics, 0o644); err != nil {
		t.Fatal(err)
	}
	compileGenerated(t, out, input)
}

func generateUnionWrapThing(t *testing.T) string {
	t.Helper()
	spec, err := os.ReadFile(filepath.Join("testdata", "unionwrapthing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	input, loadDir := syntheticInput(t)
	input.OpenAPI = spec
	out := filepath.Join(t.TempDir(), "unionwrapthing")
	options := Options{Resource: "UnionWrapThing", OutputDir: out, OverridesPath: filepath.Join("testdata", "unionwrapthing-overrides.yaml")}
	if err := generateFromInput(options, input, loadDir); err != nil {
		t.Fatal(err)
	}
	return out
}

func unionWrapThingInputs(t *testing.T) (spec, overrides string) {
	t.Helper()
	specData, err := os.ReadFile(filepath.Join("testdata", "unionwrapthing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	overridesData, err := os.ReadFile(filepath.Join("testdata", "unionwrapthing-overrides.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return string(specData), string(overridesData)
}

// Each of the schemas that the inline line covers may or may not require the object. The line
// states required: true for the ones that do not, and is stale when every one of them does. A
// oneOf joins the group of the schema that receives its fields only when that schema has none.
func TestInlineOfAnObjectWithAOneOf(t *testing.T) {
	const createTitle = "title: Create Union Wrap Thing Request\n              type: object\n"
	const replaceTitle = "title: Replace Union Wrap Thing Request\n              type: object\n"
	const resource = "    UnionWrapThing:\n      type: object\n"
	requireIn := func(old string) func(t *testing.T, s, o string) (string, string) {
		return func(t *testing.T, s, o string) (string, string) {
			return replaceOnce(t, s, old, old+"              required: [thing]\n"), o
		}
	}
	requireAll := func(t *testing.T, s, o string) (string, string) {
		s, o = requireIn(createTitle)(t, s, o)
		s, o = requireIn(replaceTitle)(t, s, o)
		return replaceOnce(t, s, resource, resource+"      required: [thing]\n"), o
	}
	tests := map[string]struct {
		change         func(t *testing.T, spec, overrides string) (string, string)
		code, location string // empty when the contract is eligible
		message        string
	}{
		"the line states the object as required": {},
		"no required line": {
			change: func(t *testing.T, s, o string) (string, string) {
				return s, replaceOnce(t, o, "{inline: true, required: true}", "{inline: true}")
			},
			code: "INLINE_UNSUPPORTED", location: "components.schemas.UnionWrapThing", message: "add required: true to the inline line of thing",
		},
		"the contract requires the object everywhere": {
			change: requireAll,
			code:   "OVERRIDE_UNUSED", location: "behavior-overrides.yaml:types.UnionWrapThing.fields.thing",
		},
		"the contract requires the object in the response only": {
			change: func(t *testing.T, s, o string) (string, string) {
				return replaceOnce(t, s, resource, resource+"      required: [thing]\n"), o
			},
		},
		"the contract requires the object in the requests only": {
			change: func(t *testing.T, s, o string) (string, string) {
				s, o = requireIn(createTitle)(t, s, o)
				return requireIn(replaceTitle)(t, s, o)
			},
		},
		"the schema that receives the fields has a oneOf": {
			change: func(t *testing.T, s, o string) (string, string) {
				return replaceOnce(t, s, resource, resource+"      oneOf: [{required: [id]}]\n"), o
			},
			code: "INLINE_UNSUPPORTED", location: "components.schemas.UnionWrapThing", message: "has a group already",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			spec, overrides := unionWrapThingInputs(t)
			if test.change != nil {
				spec, overrides = test.change(t, spec, overrides)
			}
			_, err := validateOpenAPIWith([]byte(spec), "UnionWrapThing", model.OperationIDs{}, "sdk", "provider", mustParse(t, overrides))
			if test.code == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			report := eligibilityReport(t, err)
			if !hasIssue(report, test.code, test.location) {
				t.Fatalf("report = %v, want %s at %s", report, test.code, test.location)
			}
			if !strings.Contains(fmt.Sprint(report), test.message) {
				t.Fatalf("report = %v, want the message %q", report, test.message)
			}
		})
	}
}

// requestValueBy reaches an object of a group of the resource through the group. An object of a group
// of types.<Type>.groups is in the object of its component, not in the model of the request, so the
// line is refused there.
func TestRequestValueByIsRefusedForAnObjectInANestedGroup(t *testing.T) {
	cf := &convField{TFName: "variant", RequestValueBy: []fixedBy{{Model: "ArmA", Value: "X", NestedGroup: true}}}
	err := (&convBuilder{}).applyFixedRequest(cf)
	if err == nil || !strings.Contains(err.Error(), "which is in a group of types.<Type>.groups") {
		t.Fatalf("err = %v, want requestValueBy refused for a field in a nested group", err)
	}
}

// A nested component can inline an object that its schema does not require, an object that has a
// field with its own name, and an arm that stays beside another arm. An optional object cannot have
// a required field: Terraform could not leave it out with the object.
func TestInlineOfAnOptionalObjectAndAKeptArm(t *testing.T) {
	tests := map[string]struct {
		change  func(t *testing.T, spec, overrides string) (string, string)
		message string // empty when the contract is eligible
	}{
		"the contract as it is": {},
		"the line states the object as required": {
			change: func(t *testing.T, s, o string) (string, string) {
				return s, replaceOnce(t, o, "extra: {inline: true}", "extra: {inline: true, required: true}")
			},
		},
		"an optional object with a required field": {
			change: func(t *testing.T, s, o string) (string, string) {
				return replaceOnce(t, s, "    ItemExtra:\n      type: object\n", "    ItemExtra:\n      type: object\n      required: [note]\n"), o
			},
			message: "so its fields must be optional too, but it requires note",
		},
		"an arm beside another arm with a required field": {
			change: func(t *testing.T, s, o string) (string, string) {
				return replaceOnce(t, s, "    CustomOptions:\n      type: object\n", "    CustomOptions:\n      type: object\n      required: [title]\n"), o
			},
			message: "so its fields must be optional too, but it requires title",
		},
		// With the other arm skipped, the arm is the object as before, and a request does not always send it.
		"an arm beside a skipped arm in a request": {
			change: func(t *testing.T, s, o string) (string, string) {
				return s, replaceOnce(t, o, "  ItemOptions:\n    fields:\n      custom: {inline: true}\n", "  ItemOptions:\n    fields:\n      custom: {inline: true}\n      internal: {skip: true}\n")
			},
			message: "is an arm of a oneOf group of a request",
		},
		// required: true says that the API always has the arm, so a request always sends it.
		"an arm beside a skipped arm in a request, with required": {
			change: func(t *testing.T, s, o string) (string, string) {
				return s, replaceOnce(t, o, "  ItemOptions:\n    fields:\n      custom: {inline: true}\n", "  ItemOptions:\n    fields:\n      custom: {inline: true, required: true}\n      internal: {skip: true}\n")
			},
		},
		"an arm beside a skipped arm in a request, with no line that requires it": {
			change: func(t *testing.T, s, o string) (string, string) {
				return s, replaceOnce(t, o, "      link: {inline: true, required: true}\n", "      link: {inline: true}\n")
			},
			message: "is an arm of a oneOf group of a request, so the request does not always send it; if the API always has it, add required: true to the inline line of link",
		},
		// A oneOf without the branch that allows no arm requires one. The inline removes the group, so
		// the rule would be lost.
		"an arm beside a kept arm in a oneOf that requires an arm": {
			change: func(t *testing.T, s, o string) (string, string) {
				return replaceOnce(t, s, "        - required: [custom]\n        - required: [internal]\n        - not:\n            anyOf:\n              - required: [custom]\n              - required: [internal]\n      properties:\n        custom: {allOf: [{$ref: '#/components/schemas/CustomOptions'}]}", "        - required: [custom]\n        - required: [internal]\n      properties:\n        custom: {allOf: [{$ref: '#/components/schemas/CustomOptions'}]}"), o
			},
			message: "is an arm of a oneOf group that requires one arm",
		},
		"an arm beside a kept arm with required": {
			change: func(t *testing.T, s, o string) (string, string) {
				return s, replaceOnce(t, o, "      custom: {inline: true}\n", "      custom: {inline: true, required: true}\n")
			},
			message: "no schema needs required: true on the inline line of",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			spec, overrides := unionWrapThingInputs(t)
			if test.change != nil {
				spec, overrides = test.change(t, spec, overrides)
			}
			_, err := validateOpenAPIWith([]byte(spec), "UnionWrapThing", model.OperationIDs{}, "sdk", "provider", mustParse(t, overrides))
			if test.message == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if report := eligibilityReport(t, err); !strings.Contains(fmt.Sprint(report), test.message) {
				t.Fatalf("report = %v, want the message %q", report, test.message)
			}
		})
	}
}

// A component whose object has inline lines too gets the fields of the object as the object shows
// them, whatever the names of the two components are: the object is done first. The line of an
// outer component that sorts before its object must not see the oneOf of the object.
func TestChainedInlineLinesDoNotDependOnTheNamesOfTheComponents(t *testing.T) {
	spec, overrides := unionWrapThingInputs(t)
	spec, overrides = strings.ReplaceAll(spec, "ItemAction", "AItemAction"), strings.ReplaceAll(overrides, "ItemAction", "AItemAction")
	if _, err := validateOpenAPIWith([]byte(spec), "UnionWrapThing", model.OperationIDs{}, "sdk", "provider", mustParse(t, overrides)); err != nil {
		t.Fatal(err)
	}
}

// The keep path of a distributeTo line is in API names. It goes through the objects that inline
// lines moved out of the item, and the generator reports a path that leaves them.
func TestKeepPathGoesThroughInlinedObjects(t *testing.T) {
	tests := map[string]struct{ keep, want string }{
		"a step that no line moved":      {"definition.nope", "keep: ActionDefinition has no field nope, and no inline line moved it out of its object"},
		"a step after the innermost arm": {"definition.link.url", "keep: ActionLink has no field url, and no inline line moved it out of its object"},
		"a skipped arm":                  {"definition.target", "keep: ActionDefinition has no field target, and no inline line moved it out of its object"},
		"the field that names the item":  {"itemName", "keep: UnionReminder has no field itemName"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			spec, overrides := unionWrapThingInputs(t)
			dir := t.TempDir()
			path := filepath.Join(dir, "unionwrapthing-overrides.yaml")
			if err := os.WriteFile(path, []byte(replaceOnce(t, overrides, "keep: definition.link", "keep: "+test.keep)), 0o644); err != nil {
				t.Fatal(err)
			}
			input, loadDir := syntheticInput(t)
			input.OpenAPI = []byte(spec)
			err := generateFromInput(Options{Resource: "UnionWrapThing", OutputDir: filepath.Join(dir, "out"), OverridesPath: path}, input, loadDir)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want it to contain %q", err, test.want)
			}
		})
	}
}

// A skip line of the component of an inlined object is carried to the component that receives the
// fields of the object: the target is no attribute of the action or the reminder, although only the
// definition skips it. A line of the receiving component for the field takes precedence.
func TestSkipOfAnInlinedObjectIsCarried(t *testing.T) {
	schema, err := os.ReadFile(filepath.Join(generateUnionWrapThing(t), "schema.go"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(schema), `"target"`) || strings.Contains(string(schema), `"page_id"`) {
		t.Fatal("the schema has the skipped target")
	}
	spec, overrides := unionWrapThingInputs(t)
	overrides = replaceOnce(t, overrides, "  ItemAction:\n    fields:\n      definition: {inline: true}\n", "  ItemAction:\n    fields:\n      definition: {inline: true}\n      target: {description: The target of the action}\n")
	dir := t.TempDir()
	path := filepath.Join(dir, "unionwrapthing-overrides.yaml")
	if err := os.WriteFile(path, []byte(overrides), 0o644); err != nil {
		t.Fatal(err)
	}
	input, loadDir := syntheticInput(t)
	input.OpenAPI = []byte(spec)
	out := filepath.Join(dir, "out")
	if err := generateFromInput(Options{Resource: "UnionWrapThing", OutputDir: out, OverridesPath: path}, input, loadDir); err != nil {
		t.Fatal(err)
	}
	if schema, err = os.ReadFile(filepath.Join(out, "schema.go")); err != nil || !strings.Contains(string(schema), "The target of the action") {
		t.Fatalf("the schema has no target attribute with the line of the action (%v)", err)
	}
}

// A types.<Type>.at line names the holder of the component and the fields with another name there.
// A line that matches no place, or no field, is an error: its names would be dropped without a word.
func TestPlaceLinesAreChecked(t *testing.T) {
	tests := map[string]struct{ from, to, want string }{
		"a holder that the resource does not have": {"UnionItem.first:", "UnionItem.nothing:", "no field of that holder holds Tag"},
		"a field that the component does not have": {"fields: {label: {name: title}}", "fields: {other: {name: title}}", "Tag has no field other"},
		"a key that is no holder path":             {"UnionItem.first:", "first:", "the key is the API path of the holder"},
		"a name that is no Terraform name":         {"name: title", "name: Title", "must be lower case letters"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			spec, overrides := unionWrapThingInputs(t)
			dir := t.TempDir()
			path := filepath.Join(dir, "unionwrapthing-overrides.yaml")
			if err := os.WriteFile(path, []byte(replaceOnce(t, overrides, test.from, test.to)), 0o644); err != nil {
				t.Fatal(err)
			}
			input, loadDir := syntheticInput(t)
			input.OpenAPI = []byte(spec)
			err := generateFromInput(Options{Resource: "UnionWrapThing", OutputDir: filepath.Join(dir, "out"), OverridesPath: path}, input, loadDir)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want it to contain %q", err, test.want)
			}
		})
	}
}

// An at line that inlines a field gives one place a copy of the component. The generator checks the
// holder, the field of the holder that holds the component, and the fields that the line names, and
// refuses a line that the places or the other lines of the file make unclear.
func TestPlaceInlineIsChecked(t *testing.T) {
	const extra = "    ItemExtra:\n      type: object\n"
	tests := map[string]struct {
		spec, overrides [2]string
		want            string
	}{
		"a holder that the contract does not have": {
			overrides: [2]string{"UnionItem.choice:", "NoSuchHolder.choice:"}, want: "the contract has no component NoSuchHolder"},
		"a field that the holder does not have": {
			overrides: [2]string{"UnionItem.choice:", "UnionItem.nothing:"}, want: "UnionItem has no field nothing"},
		"a field of the holder that holds another component": {
			overrides: [2]string{"UnionItem.choice:", "UnionItem.first:"}, want: "the field first of UnionItem does not hold Pick"},
		"a field that the component does not have": {
			overrides: [2]string{"value: {inline: true}\n          note:", "nothing: {inline: true}\n          note:"}, want: "Pick has no field nothing"},
		"a field that is no object": {
			overrides: [2]string{"note: {name: remark}", "note: {inline: true}"}, want: "note must be a $ref to an object component"},
		"a holder that the resource does not hold": {
			spec:      [2]string{extra, "    Orphan:\n      type: object\n      properties:\n        pick: {$ref: '#/components/schemas/Pick'}\n" + extra},
			overrides: [2]string{"UnionItem.choice:", "Orphan.pick:"}, want: "the resource does not hold Orphan.pick, so the at line of Pick changes nothing"},
		"the same field inlined in every place by the component": {
			overrides: [2]string{"  Pick:\n    at:", "  Pick:\n    fields:\n      value: {inline: true}\n    at:"}, want: "the line of Pick inlines the field in every place already"},
		"neither a name nor inline": {
			overrides: [2]string{"note: {name: remark}", "note: {}"}, want: "sets nothing"},
		"a key of a field line that the field cannot take": {
			overrides: [2]string{"note: {name: remark}", "note: {name: remark, terraformType: Int64}"}, want: "terraformType"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			spec, overrides := unionWrapThingInputs(t)
			if test.spec[0] != "" {
				spec = replaceOnce(t, spec, test.spec[0], test.spec[1])
			}
			dir := t.TempDir()
			path := filepath.Join(dir, "unionwrapthing-overrides.yaml")
			if err := os.WriteFile(path, []byte(replaceOnce(t, overrides, test.overrides[0], test.overrides[1])), 0o644); err != nil {
				t.Fatal(err)
			}
			input, loadDir := syntheticInput(t)
			input.OpenAPI = []byte(spec)
			err := generateFromInput(Options{Resource: "UnionWrapThing", OutputDir: filepath.Join(dir, "out"), OverridesPath: path}, input, loadDir)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want it to contain %q", err, test.want)
			}
		})
	}
}
