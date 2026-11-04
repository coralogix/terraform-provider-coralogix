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
	cases := map[string]struct{ from, to, want, specFrom, specTo string }{
		"splitArms on a string": {
			"      note: {computed: true}\n", "      note: {splitArms: true}\n",
			"splitArms needs a list or set whose items are a oneOf", "", ""},
		"invert on a string": {
			"      escaped: {requestValue: \"true\"}\n", "      escaped: {requestValue: \"true\"}\n      target: {invert: true}\n",
			"invert needs a bool field", "", ""},
		"position on a string": {
			"      order: {position: true}\n      enabled: {name: active", "      order: {position: true}\n      source: {position: true}\n      enabled: {name: active",
			"position needs an integer attribute", "", ""},
		"sendUnknownAsZero on a list": {
			"      matchers: {splitArms: true, collection: set, computed: true, default: []}\n", "      matchers: {splitArms: true, collection: set, computed: true, default: []}\n      groups: {computed: true, sendUnknownAsZero: true}\n",
			"sendUnknownAsZero needs a string, bool, or number field", "", ""},
		"requestValue that is no bool": {
			"      escaped: {requestValue: \"true\"}\n", "      escaped: {requestValue: \"yes\"}\n",
			"needs an enum, a string, or a bool field (true or false)", "", ""},
		"carry request value for no arm": {
			"requestValues: {pickParams: {source: text}}", "requestValues: {otherParams: {source: text}}",
			"otherParams, which is no arm", "", ""},
		"carry request value for no sibling": {
			"requestValues: {pickParams: {source: text}}", "requestValues: {pickParams: {origin: text}}",
			"origin, which is no field of ParseStep", "", ""},
		"position outside a list item": {
			"      order: {computed: true, sendUnknownAsZero: true}\n", "      order: {position: true}\n",
			"position needs a field of the items of a list", "", ""},
		"caseInsensitive on an enum set": {
			"    trimPrefix: SEVERITY_\n", "    trimPrefix: SEVERITY_\n    caseInsensitive: true\n",
			"is for a scalar field of the enum", "", ""},
		"fold of arms with other fields": {
			"dropParams: {into: keepParams,", "copyParams: {into: keepParams,",
			"the arms copyParams and keepParams need the same fields", "", ""},
		"two position fields in an item": {
			"      order: {position: true}\n      enabled: {name: active", "      order: {position: true}\n      rank: {position: true}\n      enabled: {name: active",
			"a list item has one position field",
			"        source: {type: string, x-coralogix-presence: true}\n        params:", "        source: {type: string, x-coralogix-presence: true}\n        rank: {type: integer, format: int64, x-coralogix-presence: true}\n        params:"},
		"position on a read-only field": {
			"      order: {position: true}\n      enabled: {name: active", "      order: {computed: true, sendUnknownAsZero: true}\n      rank: {position: true}\n      enabled: {name: active",
			"position needs a field that Create and Update send",
			"        id: {type: string, x-coralogix-presence: true}\n        name: {type: string, x-coralogix-presence: true}\n        enabled: {type: boolean, x-coralogix-presence: true}\n        order: {type: integer, format: int64, x-coralogix-presence: true}\n        source:", "        id: {type: string, x-coralogix-presence: true}\n        name: {type: string, x-coralogix-presence: true}\n        enabled: {type: boolean, x-coralogix-presence: true}\n        order: {type: integer, format: int64, x-coralogix-presence: true}\n        rank: {type: integer, format: int64, readOnly: true}\n        source:"},
		"caseInsensitive values that differ only in case": {
			"    names: {PICK_FIELD_THREADID: threadID}\n", "    names: {PICK_FIELD_THREADID: threadID, PICK_FIELD_TEXT: ThreadId}\n",
			"caseInsensitive needs values that differ in more than case", "", ""},
		"requestValue that is no enum value": {
			"      field: {validators: [{enum: true}]}\n", "      field: {requestValue: PICK_FIELD_BOGUS}\n",
			"is no accepted value of the enum", "", ""},
		"two folds into one arm": {
			"            dropParams: {into: keepParams, flag: keep_matching,", "            pickParams: {into: keepParams, flag: other, markdownDescription: \"x\"}\n            dropParams: {into: keepParams, flag: keep_matching,",
			"both fold into keepParams", "", ""},
		"carry request value for the carried field": {
			"requestValues: {pickParams: {source: text}}", "requestValues: {pickParams: {source: text, params: x}}",
			"the field that holds the oneOf", "", ""},
		"carry request value for a read-only sibling": {
			"requestValues: {pickParams: {source: text}}", "requestValues: {pickParams: {source: text, id: x}}",
			"which Create and Update do not send", "", ""},
		"carried names that collide": {
			"      pickParams: {name: pick}\n", "      pickParams: {name: pick}\n  ParseKeepParams:\n    fields:\n      rule: {name: source}\n",
			"two fields named source in Terraform", "", ""},
		"two carrySiblings fields": {
			"      order: {position: true}\n      enabled: {name: active", "      order: {position: true}\n      name: {carrySiblings: {}}\n      enabled: {name: active",
			"an object shows one oneOf as its arms", "", ""},
		"fold of arms with other field types": {
			"dropParams: {into: keepParams,", "dropParams: {into: keepParams,",
			"the arms dropParams and keepParams need the same fields",
			"    ParseDropParams:\n      type: object\n      properties:\n        rule: {type: string, x-coralogix-presence: true}", "    ParseDropParams:\n      type: object\n      properties:\n        rule: {type: string, format: byte, x-coralogix-presence: true}"},
		"splitArms on a list": {
			"      matchers: {splitArms: true, collection: set, computed: true, default: []}\n", "      matchers: {splitArms: true, computed: true, default: []}\n",
			"splitArms needs a set", "", ""},
		"splitArms without computed": {
			"      matchers: {splitArms: true, collection: set, computed: true, default: []}\n", "      matchers: {splitArms: true, collection: set}\n",
			"splitArms needs computed: true", "", ""},
		"position on a component that is also outside a list": {
			"      order: {position: true}\n      enabled: {name: active", "      order: {position: true}\n      enabled: {name: active",
			"is also held outside a list",
			"                teamId: {$ref: '#/components/schemas/ParseTeamId'}\n|        groups:\n          type: array\n          items: {$ref: '#/components/schemas/ParseGroup'}\n",
			"                teamId: {$ref: '#/components/schemas/ParseTeamId'}\n                lastGroup: {$ref: '#/components/schemas/CreateParseGroup'}\n|        groups:\n          type: array\n          items: {$ref: '#/components/schemas/ParseGroup'}\n        lastGroup: {$ref: '#/components/schemas/ParseGroup'}\n"},
		"carrySiblings on a read-only oneOf": {
			"requestValues: {pickParams: {source: text}}", "requestValues: {}",
			"carrySiblings needs a oneOf that Create and Update send",
			"        source: {type: string, x-coralogix-presence: true}\n        params: {$ref: '#/components/schemas/ParseParams'}\n    ParseStep:",
			"        source: {type: string, x-coralogix-presence: true}\n    ParseStep:"},
		"splitArms with item limits": {
			"validators:\n  inferred: false\n", "validators:\n  inferred: true\n",
			"splitArms cannot keep the minItems or maxItems",
			"                matchers:\n                  type: array\n|        matchers:\n          type: array\n",
			"                matchers:\n                  type: array\n                  minItems: 1\n|        matchers:\n          type: array\n          minItems: 1\n"},
		"carried arm component held elsewhere": {
			"      keepParams: {name: keep}\n", "      keepParams: {name: keep}\n",
			"component ParseKeepParams needs another model struct in each place: one place has the field",
			"                teamId: {$ref: '#/components/schemas/ParseTeamId'}\n|        groups:\n          type: array\n          items: {$ref: '#/components/schemas/ParseGroup'}\n",
			"                teamId: {$ref: '#/components/schemas/ParseTeamId'}\n                defaultKeep: {$ref: '#/components/schemas/ParseKeepParams'}\n|        groups:\n          type: array\n          items: {$ref: '#/components/schemas/ParseGroup'}\n        defaultKeep: {$ref: '#/components/schemas/ParseKeepParams'}\n"},
		"split arm with the name of a root attribute": {from: "appName: {name: apps}", to: "appName: {name: note}", want: "the split arm note has the name of another top-level attribute"},
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
			// specFrom and specTo hold one or more replacements, separated by "|".
			contract := string(spec)
			if c.specFrom != "" {
				froms, tos := strings.Split(c.specFrom, "|"), strings.Split(c.specTo, "|")
				for i := range froms {
					if !strings.Contains(contract, froms[i]) {
						t.Fatalf("the contract does not contain %q", froms[i])
					}
					contract = strings.ReplaceAll(contract, froms[i], tos[i])
				}
			}
			input.OpenAPI = []byte(contract)
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

// The lines of carrySiblings name an arm as the API does, also when the name starts with an
// acronym, so lowerFirst of its Go name is another spelling.
func TestParseThingCarryFindsAcronymArms(t *testing.T) {
	input, loadDir := copiedSyntheticInput(t)
	rename := strings.NewReplacer("keepParams", "KEEPParams", "dropParams", "DROPParams", "pickParams", "PICKParams")
	spec, err := os.ReadFile(filepath.Join("testdata", "parsething.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	input.OpenAPI = []byte(rename.Replace(string(spec)))
	sdkFile := filepath.Join(input.SDKDir, "go", "openapi", "gen", "parse_things_service", "parse.go")
	sdk, err := os.ReadFile(sdkFile)
	if err != nil {
		t.Fatal(err)
	}
	sdkRename := strings.NewReplacer("\tKeepParams *", "\tKEEPParams *", "\tDropParams *", "\tDROPParams *", "\tPickParams *", "\tPICKParams *")
	if err := os.WriteFile(sdkFile, []byte(sdkRename.Replace(string(sdk))), 0o644); err != nil {
		t.Fatal(err)
	}
	base, err := os.ReadFile(filepath.Join("testdata", "parsething-overrides.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "overrides.yaml")
	if err := os.WriteFile(path, []byte(rename.Replace(string(base))), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out")
	if err := generateFromInput(Options{Resource: "ParseThing", OutputDir: out, OverridesPath: path}, input, loadDir); err != nil {
		t.Fatal(err)
	}
	convert, err := os.ReadFile(filepath.Join(out, "convert.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`fixed := "text"`, "oneOf.DROPParams = &", "arm.KeepMatching = types.BoolValue(false)"} {
		if !strings.Contains(string(convert), want) {
			t.Errorf("convert.go does not contain %q", want)
		}
	}
}
