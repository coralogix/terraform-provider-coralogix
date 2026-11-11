package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The golden output of a released resource that shows oneOf fields as a type attribute plus the
// merged fields of their arms (typedArms): an arm with no fields, arms that share a field, an arm
// with a nested object, and scalar arms, in a single field and in the items of a list.
func TestGoldenOutputTypedThing(t *testing.T) {
	out := generateTypedThing(t)
	golden := filepath.Join("testdata", "golden", "typedthing")
	if *updateGolden {
		if err := os.RemoveAll(golden); err != nil {
			t.Fatal(err)
		}
		if err := copyDirectory(out, golden); err != nil {
			t.Fatal(err)
		}
	}
	if diff := compareDirectories(golden, out); diff != "" {
		t.Fatalf("golden output differs; run go test ./internal/generator -run TestGoldenOutputTypedThing -update:\n%s", diff)
	}
}

// The generated resource runs: see testdata/typedthing_semantics_test.go.
func TestTypedThingRuntimeSemantics(t *testing.T) {
	input, _ := syntheticInput(t)
	out := generateTypedThing(t)
	semantics, err := os.ReadFile(filepath.Join("testdata", "typedthing_semantics_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "semantics_test.go"), semantics, 0o644); err != nil {
		t.Fatal(err)
	}
	compileGenerated(t, out, input)
}

func generateTypedThing(t *testing.T) string {
	t.Helper()
	spec, err := os.ReadFile(filepath.Join("testdata", "typedthing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	input, loadDir := syntheticInput(t)
	input.OpenAPI = spec
	out := filepath.Join(t.TempDir(), "typedthing")
	options := Options{Resource: "TypedThing", OutputDir: out, OverridesPath: filepath.Join("testdata", "typedthing-overrides.yaml")}
	if err := generateFromInput(options, input, loadDir); err != nil {
		t.Fatal(err)
	}
	return out
}

// A typedArms line that cannot hold the oneOf stops generation with a message that names it.
func TestTypedThingKeysRefuseOtherShapes(t *testing.T) {
	base, err := os.ReadFile(filepath.Join("testdata", "typedthing-overrides.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	spec, err := os.ReadFile(filepath.Join("testdata", "typedthing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]struct{ from, to, want string }{
		"a name that is no arm": {
			"names: {average: avg}", "names: {average: avg, median: med}", "typedArms.names.median is no arm"},
		"two arms with one value": {
			"names: {average: avg}", "names: {average: count}", "both have the value"},
		"an arm that is a list": {
			"  TypedList:\n", "  TypedList:\n    typedArms: {}\n", "typedArms needs arms that are objects or scalars"},
		"a type field named like an arm field": {
			"names: {average: avg}", "field: field\n      names: {average: avg}", "is also a field of the arm"},
		"the same name for the type and the value": {
			"names: {metadataField: metadata, tagField: tag, processTagField: process_tag}",
			"field: value\n      names: {metadataField: metadata, tagField: tag, processTagField: process_tag}", "field and value are both"},
	}
	cases["asList with a field"] = struct{ from, to, want string }{"typedArms: {asList: true}", "typedArms: {asList: true, field: kind}", "takes no field"}
	cases["asList on a list arm with other fields"] = struct{ from, to, want string }{"      labels: {skip: true}\n", "      labels: {readEmptyAs: \"null\"}\n", "skip the other fields of the list arm"}
	cases["a rule that names no attribute of the arm"] = struct{ from, to, want string }{"exactlyOneOf: [unit]", "exactlyOneOf: [nothing]", "is not another attribute of the arm"}
	cases["a validators line on a number field of an arm"] = struct{ from, to, want string }{"      percentileUnit: {name: unit}\n", "      percentileUnit: {name: unit}\n      percent: {validators: [{between: [0, 1]}]}\n", "takes a rule over other attributes, or a validator of a text field"}
	cases["two arm fields with one Go name and two Terraform names"] = struct{ from, to, want string }{
		"      field: {readEmptyAs: \"null\", validators: [{exactlyOneOf: [unit]}]}\n",
		"      field: {readEmptyAs: \"null\", name: other_field}\n", "have one Go name"}
	// A shared field that is not text has one attribute, so its checks must be the same in every arm.
	t.Run("a shared number with other limits", func(t *testing.T) {
		limited := strings.Replace(string(spec), "        weight: {type: number, format: double, x-coralogix-presence: true}\n", "        weight: {type: number, format: double, minimum: 0, x-coralogix-presence: true}\n", 1)
		limited = strings.Replace(limited, "        weight: {type: number, format: double, x-coralogix-presence: true}\n", "        weight: {type: number, format: double, maximum: 5, x-coralogix-presence: true}\n", 1)
		kept := strings.Replace(string(base), "  inferred: false", "  inferred: true", 1)
		if limited == string(spec) || kept == string(base) {
			t.Fatal("the contract or the overrides did not change: update the replaced text")
		}
		dir := t.TempDir()
		overrides := filepath.Join(dir, "typedthing-overrides.yaml")
		if err := os.WriteFile(overrides, []byte(kept), 0o644); err != nil {
			t.Fatal(err)
		}
		input, loadDir := syntheticInput(t)
		input.OpenAPI = []byte(limited)
		err := generateFromInput(Options{Resource: "TypedThing", OutputDir: filepath.Join(dir, "typedthing"), OverridesPath: overrides}, input, loadDir)
		if err == nil || !strings.Contains(err.Error(), "only a text field gets checks for each arm") {
			t.Fatalf("error = %v, want it to say that only a text field gets checks for each arm", err)
		}
	})
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			changed := strings.Replace(string(base), c.from, c.to, 1)
			if changed == string(base) {
				t.Fatal("the overrides did not change: update the replaced text")
			}
			dir := t.TempDir()
			overrides := filepath.Join(dir, "typedthing-overrides.yaml")
			if err := os.WriteFile(overrides, []byte(changed), 0o644); err != nil {
				t.Fatal(err)
			}
			input, loadDir := syntheticInput(t)
			input.OpenAPI = spec
			options := Options{Resource: "TypedThing", OutputDir: filepath.Join(dir, "typedthing"), OverridesPath: overrides}
			err := generateFromInput(options, input, loadDir)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("error = %v, want it to contain %q", err, c.want)
			}
		})
	}
}

// An inline line on a field whose object has a typedArms line shows the arms of the object as the
// attributes of the component that holds it. The generator refuses the shapes where the line would
// make an arm of something that is no arm, or where no component could take it.
func TestInlineOfATypedObjectIsChecked(t *testing.T) {
	spanValue := "    TypedSpanValue:\n      type: object\n      properties:\n        value: {$ref: '#/components/schemas/TypedSpanField'}\n"
	tests := map[string]struct {
		spec, overrides [2]string
		want            string
	}{
		"an object that shows a list": {
			overrides: [2]string{"    typedArms:\n      names: {metadataField: metadata, tagField: tag, processTagField: process_tag}", "    typedArms: {asList: true}"},
			want:      "has typedArms.asList, which shows a list",
		},
		"another field beside the object": {
			spec: [2]string{spanValue, spanValue + "        note: {type: string, x-coralogix-presence: true}\n"},
			want: "the fields of an object with a typedArms line are arms in TypedSpanValue, but TypedSpanValue has other fields",
		},
		"two objects with a typedArms line": {
			spec:      [2]string{spanValue, spanValue + "        notSelection: {$ref: '#/components/schemas/TypedNotSelection'}\n"},
			overrides: [2]string{"      value: {inline: true}\n", "      value: {inline: true}\n      notSelection: {inline: true}\n"},
			want:      "more than one inlined object has a typedArms line",
		},
		"the resource itself": {
			overrides: [2]string{"      id: {readOnly: true}\n", "      id: {readOnly: true}\n      aggregation: {inline: true, required: true}\n"},
			want:      "only a nested component takes with its fields",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			spec, err := os.ReadFile(filepath.Join("testdata", "typedthing.yaml"))
			if err != nil {
				t.Fatal(err)
			}
			overrides, err := os.ReadFile(filepath.Join("testdata", "typedthing-overrides.yaml"))
			if err != nil {
				t.Fatal(err)
			}
			if test.spec[0] != "" {
				spec = []byte(replaceOnce(t, string(spec), test.spec[0], test.spec[1]))
			}
			if test.overrides[0] != "" {
				overrides = []byte(replaceOnce(t, string(overrides), test.overrides[0], test.overrides[1]))
			}
			path := filepath.Join(t.TempDir(), "typedthing-overrides.yaml")
			if err := os.WriteFile(path, overrides, 0o644); err != nil {
				t.Fatal(err)
			}
			input, loadDir := syntheticInput(t)
			input.OpenAPI = spec
			err = generateFromInput(Options{Resource: "TypedThing", OutputDir: filepath.Join(t.TempDir(), "typedthing"), OverridesPath: path}, input, loadDir)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want it to contain %q", err, test.want)
			}
		})
	}
}
