package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The golden output of a released resource like the parsing rule groups: a list of oneOf
// matchers shown as one set per arm (splitArms), and scalars that the released resource sent as
// their zero value when the plan was unknown (sendUnknownAsZero).
func TestGoldenOutputParseThing(t *testing.T) {
	out := generateParseThing(t)
	golden := filepath.Join("testdata", "golden", "parsething")
	if *updateGolden {
		if err := os.RemoveAll(golden); err != nil {
			t.Fatal(err)
		}
		if err := copyDirectory(out, golden); err != nil {
			t.Fatal(err)
		}
	}
	if diff := compareDirectories(golden, out); diff != "" {
		t.Fatalf("golden output differs; run go test ./internal/generator -run TestGoldenOutputParseThing -update:\n%s", diff)
	}
}

// The generated resource runs: see testdata/parsething_semantics_test.go.
func TestParseThingRuntimeSemantics(t *testing.T) {
	input, _ := syntheticInput(t)
	out := generateParseThing(t)
	semantics, err := os.ReadFile(filepath.Join("testdata", "parsething_semantics_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "semantics_test.go"), semantics, 0o644); err != nil {
		t.Fatal(err)
	}
	compileGenerated(t, out, input)
}

func generateParseThing(t *testing.T) string {
	t.Helper()
	spec, err := os.ReadFile(filepath.Join("testdata", "parsething.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	input, loadDir := syntheticInput(t)
	input.OpenAPI = spec
	out := filepath.Join(t.TempDir(), "parsething")
	options := Options{Resource: "ParseThing", OutputDir: out, OverridesPath: filepath.Join("testdata", "parsething-overrides.yaml")}
	if err := generateFromInput(options, input, loadDir); err != nil {
		t.Fatal(err)
	}
	return out
}

// Each new key of the parsing rules refuses a field that it cannot convert, with a message that
// names the field.
func TestParseThingKeysRefuseOtherShapes(t *testing.T) {
	base, err := os.ReadFile(filepath.Join("testdata", "parsething-overrides.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	spec, err := os.ReadFile(filepath.Join("testdata", "parsething.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]struct{ from, to, want string }{
		"splitArms on a string": {
			"      note: {computed: true}\n", "      note: {splitArms: true}\n",
			"splitArms needs a list or set whose items are a oneOf"},
		"negate on a string": {
			"      escaped: {requestValue: \"true\"}\n", "      escaped: {requestValue: \"true\"}\n      target: {negate: true}\n",
			"negate needs a bool field"},
		"position on a string": {
			"      order: {position: true}\n      enabled: {name: active", "      order: {position: true}\n      source: {position: true}\n      enabled: {name: active",
			"position needs an integer attribute"},
		"sendUnknownAsZero on a list": {
			"      matchers: {splitArms: true, collection: set, computed: true, default: []}\n", "      matchers: {splitArms: true, collection: set, computed: true, default: []}\n      groups: {computed: true, sendUnknownAsZero: true}\n",
			"sendUnknownAsZero needs a string, bool, or number field"},
		"requestValue that is no bool": {
			"      escaped: {requestValue: \"true\"}\n", "      escaped: {requestValue: \"yes\"}\n",
			"needs an enum, a string, or a bool field (true or false)"},
		"carry request value for no arm": {
			"requestValues: {pickParams: {source: text}}", "requestValues: {otherParams: {source: text}}",
			"otherParams, which is no arm"},
		"carry request value for no sibling": {
			"requestValues: {pickParams: {source: text}}", "requestValues: {pickParams: {origin: text}}",
			"origin, which is no field of ParseStep"},
		"position outside a list item": {
			"      order: {computed: true, sendUnknownAsZero: true}\n", "      order: {position: true}\n",
			"position needs a field of the items of a list"},
		"caseInsensitive on an enum set": {
			"    trimPrefix: SEVERITY_\n", "    trimPrefix: SEVERITY_\n    caseInsensitive: true\n",
			"is for a scalar field of the enum"},
		"fold of arms with other fields": {
			"dropParams: {into: keepParams,", "copyParams: {into: keepParams,",
			"the arms copyParams and keepParams need the same fields"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if !strings.Contains(string(base), c.from) {
				t.Fatalf("the overrides do not contain %q", c.from)
			}
			dir := t.TempDir()
			path := filepath.Join(dir, "overrides.yaml")
			if err := os.WriteFile(path, []byte(strings.Replace(string(base), c.from, c.to, 1)), 0o644); err != nil {
				t.Fatal(err)
			}
			input, loadDir := syntheticInput(t)
			input.OpenAPI = spec
			err := generateFromInput(Options{Resource: "ParseThing", OutputDir: filepath.Join(dir, "out"), OverridesPath: path}, input, loadDir)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("generate = %v, want an error that contains %q", err, c.want)
			}
		})
	}
}

// The acceptance test makes up an element of an enum set from the Terraform values of the enums
// line, also when validators.inferred is false and an enum: true line adds the validator.
func TestParseThingAcceptanceUsesTerraformEnumValues(t *testing.T) {
	spec, err := os.ReadFile(filepath.Join("testdata", "parsething.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	acceptance := filepath.Join(dir, "acceptance.yaml")
	if err := os.WriteFile(acceptance, []byte("resource: ParseThing\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	input, loadDir := syntheticInput(t)
	input.OpenAPI = spec
	out := filepath.Join(dir, "parsething")
	options := Options{Resource: "ParseThing", OutputDir: out, OverridesPath: filepath.Join("testdata", "parsething-overrides.yaml"), AcceptancePath: acceptance}
	if err := generateFromInput(options, input, loadDir); err != nil {
		t.Fatal(err)
	}
	test, err := os.ReadFile(filepath.Join(out, "acceptance_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(test), `severities = ["debug"]`) || strings.Contains(string(test), "SEVERITY_") {
		t.Fatalf("the acceptance configs do not set severities to a Terraform value:\n%s", test)
	}
}
