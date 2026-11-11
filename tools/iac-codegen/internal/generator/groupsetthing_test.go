package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The golden output of a released resource whose groups are a set with nested server defaults that
// the response requires, a validator line that keeps such a default, a uint64 limit shown as Int64, a
// skipped uint64 that may not fit Int64, a required line on a component that only the response
// has, and a string attribute that the API does not hold.
func TestGoldenOutputGroupSetThing(t *testing.T) {
	out := generateGroupSetThing(t)
	golden := filepath.Join("testdata", "golden", "groupsetthing")
	if *updateGolden {
		if err := os.RemoveAll(golden); err != nil {
			t.Fatal(err)
		}
		if err := copyDirectory(out, golden); err != nil {
			t.Fatal(err)
		}
	}
	if diff := compareDirectories(golden, out); diff != "" {
		t.Fatalf("golden output differs; run go test ./internal/generator -run TestGoldenOutputGroupSetThing -update:\n%s", diff)
	}
}

// Create sends the limit as a string and leaves out the unset interval; a read flattens the stored
// limit to Int64, and keeps the document of the plan. A state upgrade keeps the prior document.
func TestGroupSetThingRuntimeSemantics(t *testing.T) {
	input, _ := syntheticInput(t)
	out := generateGroupSetThing(t)
	semantics, err := os.ReadFile(filepath.Join("testdata", "groupsetthing_semantics_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "semantics_test.go"), semantics, 0o644); err != nil {
		t.Fatal(err)
	}
	compileGenerated(t, out, input)
}

func generateGroupSetThing(t *testing.T) string {
	t.Helper()
	spec, err := os.ReadFile(filepath.Join("testdata", "groupsetthing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	input, loadDir := syntheticInput(t)
	input.OpenAPI = spec
	out := filepath.Join(t.TempDir(), "groupsetthing")
	options := Options{Resource: "GroupSetThing", OutputDir: out, OverridesPath: filepath.Join("testdata", "groupsetthing-overrides.yaml")}
	if err := generateFromInput(options, input, loadDir); err != nil {
		t.Fatal(err)
	}
	return out
}

// Another resource's operation that sends OutGroupThing in its request does not make the required
// line of OutGroupThing stale: only the Create and Update of the selected resource count.
func TestRequiredLineIgnoresAnotherResourcesRequest(t *testing.T) {
	spec, err := os.ReadFile(filepath.Join("testdata", "groupsetthing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	const other = `  /groupset/v1/imports:
    post:
      tags: [Group Import Things Service]
      operationId: GroupImports_Create
      requestBody:
        required: true
        content:
          application/json:
            schema: {$ref: '#/components/schemas/OutGroupThing'}
      responses:
        '200':
          description: ok
          content:
            application/json:
              schema: {type: object}
components:
`
	text := strings.Replace(string(spec), "components:\n", other, 1)
	if text == string(spec) {
		t.Fatal("fixture text not found")
	}
	input, loadDir := syntheticInput(t)
	input.OpenAPI = []byte(text)
	options := Options{Resource: "GroupSetThing", OutputDir: filepath.Join(t.TempDir(), "out"), OverridesPath: filepath.Join("testdata", "groupsetthing-overrides.yaml")}
	if err := generateFromInput(options, input, loadDir); err != nil {
		t.Fatalf("generate: %v", err)
	}
}

// A keep-state line on a nested field with a server default keeps the released behavior: the plan
// keeps the state, and no server-default modifier changes it back to unknown.
func TestKeepStateLineDropsTheNestedServerDefault(t *testing.T) {
	for _, key := range []string{"useStateForUnknown", "useNonNullStateForUnknown"} {
		t.Run(key, func(t *testing.T) {
			overrides, err := os.ReadFile(filepath.Join("testdata", "groupsetthing-overrides.yaml"))
			if err != nil {
				t.Fatal(err)
			}
			const line = "      interval:\n        validators: [{atLeast: 60}]\n"
			text := strings.Replace(string(overrides), line, line+"        "+key+": true\n", 1)
			if text == string(overrides) {
				t.Fatal("fixture text not found")
			}
			dir := t.TempDir()
			path := filepath.Join(dir, "overrides.yaml")
			if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
				t.Fatal(err)
			}
			spec, err := os.ReadFile(filepath.Join("testdata", "groupsetthing.yaml"))
			if err != nil {
				t.Fatal(err)
			}
			input, loadDir := syntheticInput(t)
			input.OpenAPI = spec
			out := filepath.Join(dir, "out")
			if err := generateFromInput(Options{Resource: "GroupSetThing", OutputDir: out, OverridesPath: path}, input, loadDir); err != nil {
				t.Fatalf("generate: %v", err)
			}
			schema := string(mustRead(t, filepath.Join(out, "schema.go")))
			start := strings.Index(schema, `"interval": schema.Int64Attribute{`)
			if start < 0 {
				t.Fatalf("schema.go has no interval attribute:\n%s", schema)
			}
			attr := schema[start : start+strings.Index(schema[start:], "Description:")]
			if strings.Contains(attr, "serverDefaultModifier") {
				t.Fatalf("interval keeps the server-default modifier with %s:\n%s", key, attr)
			}
		})
	}
}

// A server default inside a set within another set is refused: the modifier can ask the
// configuration only about the outermost set.
func TestServerDefaultInsideANestedSetIsRefused(t *testing.T) {
	spec, err := os.ReadFile(filepath.Join("testdata", "groupsetthing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(spec)
	for _, r := range [][2]string{
		{"items: {type: array, items: {$ref: '#/components/schemas/InItemThing'}}", "items: {type: array, x-coralogix-collection: set, items: {$ref: '#/components/schemas/InItemThing'}}"},
		{"items: {type: array, items: {$ref: '#/components/schemas/OutItemThing'}}", "items: {type: array, x-coralogix-collection: set, items: {$ref: '#/components/schemas/OutItemThing'}}"},
		{"      required: [key]\n      properties:\n        key: {type: string, minLength: 1}\n", "      required: [key]\n      properties:\n        key: {type: string, minLength: 1}\n        weight: {type: integer, format: int64, default: 1, x-coralogix-presence: true}\n"},
		{"      required: [key, labels]\n      properties:\n        key: {type: string, minLength: 1}\n", "      required: [key, labels, weight]\n      properties:\n        key: {type: string, minLength: 1}\n        weight: {type: integer, format: int64, default: 1, x-coralogix-presence: true}\n"},
	} {
		if !strings.Contains(text, r[0]) {
			t.Fatalf("fixture text not found: %s", r[0])
		}
		text = strings.Replace(text, r[0], r[1], 1)
	}
	input, loadDir := syntheticInput(t)
	input.OpenAPI = []byte(text)
	options := Options{Resource: "GroupSetThing", OutputDir: filepath.Join(t.TempDir(), "out"), OverridesPath: filepath.Join("testdata", "groupsetthing-overrides.yaml")}
	err = generateFromInput(options, input, loadDir)
	if err == nil || !strings.Contains(err.Error(), "inside a set within another set is unsupported") {
		t.Fatalf("generate = %v, want the nested-set server default refused", err)
	}
}

// An optional group needs fields whose absence the response can show, and a useStateForUnknown line
// of a type needs a list or set that holds the type.
func TestOptionalGroupKeysAreChecked(t *testing.T) {
	spec, err := os.ReadFile(filepath.Join("testdata", "groupsetthing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	base, err := os.ReadFile(filepath.Join("testdata", "groupsetthing-overrides.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	tests := map[string]struct{ old, new, want string }{
		"a field that the SDK holds as a value": {
			"        fields: [periodMinutes]\n", "        fields: [periodMinutes, name]\n", "an optional group needs fields that the SDK holds by pointer",
		},
		"a type that no list or set holds": {
			"  OutGroupThing:\n    # The period", "  InItemThing:\n    useStateForUnknown: true\n  OutGroupThing:\n    # The period", "no list or set of the resource holds objects of InItemThing",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if !strings.Contains(string(base), test.old) {
				t.Fatalf("the overrides file has no %q", test.old)
			}
			path := filepath.Join(t.TempDir(), "overrides.yaml")
			if err := os.WriteFile(path, []byte(strings.Replace(string(base), test.old, test.new, 1)), 0o644); err != nil {
				t.Fatal(err)
			}
			input, loadDir := syntheticInput(t)
			input.OpenAPI = spec
			options := Options{Resource: "GroupSetThing", OutputDir: filepath.Join(t.TempDir(), "out"), OverridesPath: path}
			if err := generateFromInput(options, input, loadDir); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err = %v, want %q", err, test.want)
			}
		})
	}
}

// A group inside a group of the resource stands for one arm of a oneOf whose other arms are fields
// of the group. The generator reports a group that is not optional, one that holds more than that
// arm, a field that the resource does not have, and a name that another attribute of the group has.
func TestGroupInsideAGroupIsChecked(t *testing.T) {
	spec, err := os.ReadFile(filepath.Join("testdata", "groupsetthing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	base, err := os.ReadFile(filepath.Join("testdata", "groupsetthing-overrides.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	const inner = "      relative: {fields: [relativeWindow], optional: true}\n"
	tests := map[string]struct{ new, want string }{
		"not optional":                          {"      relative: {fields: [relativeWindow]}\n", "groups.window.groups.relative: the group holds an arm of a oneOf, so the other arms can be set instead: add optional: true"},
		"a field that is no arm":                {"      relative: {fields: [relativeWindow, name], optional: true}\n", "groups.window.groups.relative: a group inside a group holds one arm of a oneOf and no other field"},
		"a field that is no top-level field":    {inner + "      ghost: {fields: [missing], optional: true}\n", "groups.window.groups.ghost: missing is no top-level field of GroupSetThing"},
		"the name of an attribute of the group": {inner + "      absolute: {fields: [name], optional: true}\n", "groups.window.groups.absolute: the name is also the attribute absolute of window"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if !strings.Contains(string(base), inner) {
				t.Fatalf("the overrides file has no %q", inner)
			}
			path := filepath.Join(t.TempDir(), "overrides.yaml")
			if err := os.WriteFile(path, []byte(strings.Replace(string(base), inner, test.new, 1)), 0o644); err != nil {
				t.Fatal(err)
			}
			input, loadDir := syntheticInput(t)
			input.OpenAPI = spec
			options := Options{Resource: "GroupSetThing", OutputDir: filepath.Join(t.TempDir(), "out"), OverridesPath: path}
			if err := generateFromInput(options, input, loadDir); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err = %v, want %q", err, test.want)
			}
		})
	}
}
