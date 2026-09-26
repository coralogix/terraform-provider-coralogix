package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/internal/model"
)

// testSDKModule is the test SDK module (testsdk/generate.sh).
const testSDKModule = "github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/testsdk"

// generatedCases are the generated resources and their inputs.
var generatedCases = []struct{ dir, spec, resource, sdk, acc string }{
	{"../../generated/aievaluation", patchedSpec, "AiEvaluation", realSDK, "../../spec/acc/AiEvaluation.yaml"},
	{"../../generated/fakeboard", "../../spec/fake/openapi.yaml", "FakeBoard", testSDKModule, ""},
	{"../../generated/fakesettings", "../../spec/fake/settings.yaml", "FakeSettings", testSDKModule, ""},
	{"../../generated/fakerule", "../../spec/fake/rules.yaml", "FakeRule", testSDKModule, ""},
	{"../../generated/fakeview", "../../spec/fake/views.yaml", "FakeView", testSDKModule, ""},
}

// TestGeneratedUpToDate checks that each generated directory is the output
// of the generator for its spec. To rewrite it, run the --out command in
// README.md, "Commands".
func TestGeneratedUpToDate(t *testing.T) {
	for _, c := range generatedCases {
		t.Run(c.resource, func(t *testing.T) {
			r, refs, err := checkedSDKNames(c.spec, c.resource, c.sdk)
			if err != nil {
				t.Fatal(err)
			}
			var acc *accValues
			if c.acc != "" {
				if acc, err = loadAccValues(c.acc); err != nil {
					t.Fatal(err)
				}
			}
			files, err := generate(r, refs, filepath.Base(c.dir), acc)
			if err != nil {
				t.Fatal(err)
			}
			for name, got := range files {
				want, err := os.ReadFile(filepath.Join(c.dir, name))
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(got, want) {
					t.Errorf("%s is not up to date. Run go run ./cmd/tfgen ... --out %s", name, c.dir)
				}
			}
			again, err := generate(r, refs, filepath.Base(c.dir), acc)
			if err != nil {
				t.Fatal(err)
			}
			for name := range files {
				if !bytes.Equal(files[name], again[name]) {
					t.Errorf("%s: two runs give different output", name)
				}
			}
		})
	}
}

func TestTFName(t *testing.T) {
	cases := map[string]string{
		"id":                "id",
		"isEnabled":         "is_enabled",
		"sqlReadOnly":       "sql_read_only",
		"allowRecursiveCte": "allow_recursive_cte",
		"HTTPServer":        "http_server",
	}
	for in, want := range cases {
		if got := tfName(in); got != want {
			t.Errorf("tfName(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestTopLevelMaskWithGroups renders the fake resource with a mask pattern
// that accepts only top-level names, and type-checks the result. The fake
// resource has oneOf groups at the root, so this covers the top-level mask
// code for groups, which no generated resource uses yet.
func TestTopLevelMaskWithGroups(t *testing.T) {
	r, refs, err := checkedSDKNames("../../spec/fake/openapi.yaml", "FakeBoard", testSDKModule)
	if err != nil {
		t.Fatal(err)
	}
	r.UpdateMaskPattern = `^[a-zA-Z_][a-zA-Z0-9_]*(,[a-zA-Z_][a-zA-Z0-9_]*)*$`
	dir, err := os.MkdirTemp("../../generated", "zz_toplevel_")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	files, err := generate(r, refs, filepath.Base(dir), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(files["mask.go"], []byte("dropSwitchedArms(mask, \"\", maskFields, maskGroups")) ||
		bytes.Contains(files["mask.go"], []byte("maskPaths")) {
		t.Error("mask.go is not the top-level mask with groups")
	}
	for name, src := range files {
		if err := os.WriteFile(filepath.Join(dir, name), src, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfg := &packages.Config{Mode: packages.NeedTypes | packages.NeedSyntax | packages.NeedTypesInfo, Dir: dir}
	pkgs, err := packages.Load(cfg, ".")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range pkgs {
		for _, e := range p.Errors {
			t.Error(e)
		}
	}
}

// generatedTypeCases are the generated type packages (D20) and their inputs.
var generatedTypeCases = []struct {
	dir, spec, tag, sdk string
	roots, enums        []string
	overrides           string
}{
	{"../../generated/fakepanel", "../../spec/fake/openapi.yaml", "Fake Boards Service", testSDKModule, []string{"Panel", "Header", "Interval", "AbsoluteTime"},
		[]string{"Color", "Unit", "Orientation", "Comparison", "Delivery"}, ""},
	{"../../generated/dashboardwidgets", patchedSpec, "Dashboard service", realSDK, []string{"Widget.Definition"}, nil, ""},
	{"../../generated/fakerouting", "../../spec/fake/openapi.yaml", "Fake Boards Service", testSDKModule, []string{"Routing"}, nil,
		"../../spec/fake/routing.overrides.yaml"},
	{"../../generated/fakewrap", "../../spec/fake/openapi.yaml", "Fake Boards Service", testSDKModule, []string{"Layout"}, nil,
		"../../spec/fake/layout.overrides.yaml"},
	{"../../generated/fakeunwrap", "../../spec/fake/openapi.yaml", "Fake Boards Service", testSDKModule, []string{"Query"}, nil,
		"../../spec/fake/query.overrides.yaml"},
	{"../../generated/fakeinline", "../../spec/fake/openapi.yaml", "Fake Boards Service", testSDKModule, []string{"Key"}, nil,
		"../../spec/fake/key.overrides.yaml"},
	{"../../generated/fakealarm", "../../spec/fake/openapi.yaml", "Fake Boards Service", testSDKModule, []string{"Alarm"}, nil,
		"../../spec/fake/alarm.overrides.yaml"},
}

// TestGeneratedTypesUpToDate checks that each generated type package is the
// output of the generator, and that a second run gives the same files.
func TestGeneratedTypesUpToDate(t *testing.T) {
	for _, c := range generatedTypeCases {
		t.Run(filepath.Base(c.dir), func(t *testing.T) {
			in := typeInputs{roots: c.roots, enums: c.enums, overrides: c.overrides}
			types, enums, refs, err := checkedTypeNames(c.spec, in, c.tag, c.sdk)
			if err != nil {
				t.Fatal(err)
			}
			var ov *overrides
			if c.overrides != "" {
				if ov, err = loadOverrides(c.overrides); err != nil {
					t.Fatal(err)
				}
			}
			cmd := typesCommand(in, c.tag)
			files, err := generateTypes(types, enums, refs, ov, filepath.Base(c.dir), cmd)
			if err != nil {
				t.Fatal(err)
			}
			again, err := generateTypes(types, enums, refs, ov, filepath.Base(c.dir), cmd)
			if err != nil {
				t.Fatal(err)
			}
			for name, got := range files {
				want, err := os.ReadFile(filepath.Join(c.dir, name))
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(got, want) {
					t.Errorf("%s is not up to date. Run go run ./cmd/tfgen --types ... --out %s", name, c.dir)
				}
				if !bytes.Equal(got, again[name]) {
					t.Errorf("%s: two runs give different output", name)
				}
			}
		})
	}
}

// TestTypesRejects checks the inputs that the type mode rejects.
func TestTypesRejects(t *testing.T) {
	const spec = "../../spec/fake/openapi.yaml"
	for _, c := range []struct {
		name  string
		roots []string
		tag   string
		want  string
	}{
		{"no tag", []string{"Panel"}, "", "--tag is required with --types and --enums"},
		{"unknown type", []string{"Nope"}, "Fake Boards Service", "component Nope not found"},
		{"an enum", []string{"Unit"}, "Fake Boards Service", "want an object or a oneOf with fields"},
		{"no fields", []string{"BoldStyle"}, "Fake Boards Service", "want an object or a oneOf with fields"},
		{"listed twice", []string{"Panel", "Panel"}, "Fake Boards Service", "listed twice"},
		{"another SDK package", []string{"Panel"}, "Fake Settings Service", "fake_settings_service"},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, _, _, err := checkedTypeNames(spec, typeInputs{roots: c.roots}, c.tag, testSDKModule)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("error = %v, want it to contain %q", err, c.want)
			}
		})
	}
}

// TestEnumNames checks the Terraform names of the fake enums: the three zero
// value styles of the real API, and a name that two values share.
func TestEnumNames(t *testing.T) {
	_, enums, refs, err := checkedTypeNames("../../spec/fake/openapi.yaml",
		typeInputs{enums: []string{"Orientation", "Comparison", "Delivery", "Color"}}, "Fake Boards Service", testSDKModule)
	if err != nil {
		t.Fatal(err)
	}
	ix, err := indexRefs(refs)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"Orientation": "vertical=ORIENTATION_VERTICAL_UNSPECIFIED horizontal=ORIENTATION_HORIZONTAL",
		"Comparison":  "more_than=COMPARISON_MORE_THAN_OR_UNSPECIFIED less_than=COMPARISON_LESS_THAN",
		"Delivery":    "disabled=DISABLED errors_only=ERRORS_ONLY",
		"Color":       "red=RED green=GREEN blue=BLUE",
	}
	for _, e := range enums {
		got, err := enumNames(e, ix, nil)
		if err != nil {
			t.Fatal(err)
		}
		var items []string
		for _, it := range got.Items {
			items = append(items, it.TFName+"="+it.Value)
		}
		if s := strings.Join(items, " "); s != want[e.Schema] {
			t.Errorf("%s: %s, want %s", e.Schema, s, want[e.Schema])
		}
	}
	clash := *enums[0]
	clash.Values = []string{"ORIENTATION_VERTICAL", "ORIENTATION_VERTICAL_UNSPECIFIED"}
	if _, err := enumNames(&clash, ix, nil); err == nil || !strings.Contains(err.Error(), `both have the Terraform name "vertical"`) {
		t.Errorf("two values with one name: error %v", err)
	}
	if _, _, _, err := checkedTypeNames("../../spec/fake/openapi.yaml", typeInputs{enums: []string{"Panel"}}, "Fake Boards Service", testSDKModule); err == nil {
		t.Error("an object in --enums: no error")
	}
}

// TestOverridesRejects checks the overrides files that the type mode
// rejects (D21). A stale or wrong override is an error, not ignored.
func TestOverridesRejects(t *testing.T) {
	const spec = "../../spec/fake/openapi.yaml"
	in := typeInputs{roots: []string{"Routing"}}
	types, enums, refs, err := checkedTypeNames(spec, in, "Fake Boards Service", testSDKModule)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ name, yaml, want string }{
		{"unknown key", "types: {Routing: {routingName: {rename: x}}}", "field rename not found"},
		{"unknown component", "types: {Panel: {query: {skip: true}}}", "types.Panel: no such object"},
		{"unknown field", "types: {Routing: {nope: {skip: true}}}", "types.Routing.nope: no such field"},
		{"skip with more", "types: {Routing: {weight: {skip: true, name: w}}}", "skip cannot be combined"},
		{"bad name", "types: {Routing: {weight: {name: Weight}}}", `"Weight" is not a valid Terraform attribute name`},
		{"name clash", "types: {Routing: {weight: {name: priority}}}", `weight both have the Terraform name "priority"`},
		{"set on a scalar", "types: {Routing: {weight: {set: true}}}", "set: the field is a number, not a list"},
		{"required and computed", "types: {Routing: {weight: {required: true, computed: true}}}", "cannot be optional or computed"},
		{"no flag left", "types: {Routing: {weight: {optional: false}}}", "must be required, optional, or computed"},
		{"default not computed", `types: {Routing: {weight: {default: "1", computed: false}}}`, "a default needs a computed attribute"},
		{"every field skipped", "types: {Target: {connectorId: {skip: true}, tags: {skip: true}}}", "types.Target: every field is skipped"},
		{"read only and required", "types: {Routing: {weight: {readOnly: true, required: true}}}", "readOnly cannot be combined"},
		{"zero and null", "types: {Routing: {channels: {missingAsZero: true, emptyAsNull: true}}}", "cannot be combined"},
		{"null of a scalar", "types: {Routing: {weight: {emptyAsNull: true}}}", "emptyAsNull: the field is a number"},
		{"zero of a time", "types: {Routing: {createTime: {missingAsZero: true}}}", "missingAsZero is not supported for time"},
		{"flags of a spec readOnly field", "types: {Routing: {createTime: {computed: true}}}", "the spec marks the field readOnly"},
		{"unknown enum", "enums: {Color: {terraformNames: true}}", "enums.Color: no such enum"},
		{"values without names", "enums: {Delivery: {values: {DISABLED: off}}}", "need terraformNames: true"},
		{"unknown enum value", "enums: {Delivery: {terraformNames: true, values: {NOPE: x}}}", "values.NOPE: no such value"},
		{"enum name clash", "enums: {Delivery: {terraformNames: true, values: {DISABLED: errors_only}}}", `both have the Terraform name "errors_only"`},
		{"enum default", "types: {Routing: {delivery: {default: ERRORS_ONLY}}}\nenums: {Delivery: {terraformNames: true}}", `default "ERRORS_ONLY" is not one of [disabled errors_only]`},
	} {
		t.Run(c.name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "overrides.yaml")
			if err := os.WriteFile(p, []byte(c.yaml), 0o644); err != nil {
				t.Fatal(err)
			}
			ov, err := loadOverrides(p)
			if err == nil {
				_, err = generateTypes(types, enums, refs, ov, "fakerouting", "")
			}
			switch {
			case err == nil:
				t.Fatalf("no error, want %q", c.want)
			case !strings.Contains(err.Error(), c.want):
				t.Errorf("error:\n%v\nwant it to contain:\n%s", err, c.want)
			}
		})
	}
}

// TestWideNumbersRejectsLists checks that wideNumbers stops on a list of
// int32 (Section.columns), which it does not convert yet.
func TestWideNumbersRejectsLists(t *testing.T) {
	types, enums, refs, err := checkedTypeNames("../../spec/fake/openapi.yaml", typeInputs{roots: []string{"Section"}}, "Fake Boards Service", testSDKModule)
	if err != nil {
		t.Fatal(err)
	}
	_, err = generateTypes(types, enums, refs, &overrides{WideNumbers: true}, "fake", "")
	if err == nil || !strings.Contains(err.Error(), "wideNumbers: a list of integer int32 is not supported") {
		t.Errorf("error = %v", err)
	}
}

// TestWrapperRejects checks the wrapper overrides that the type mode rejects
// (D21).
func TestWrapperRejects(t *testing.T) {
	const spec = "../../spec/fake/openapi.yaml"
	types, enums, refs, err := checkedTypeNames(spec, typeInputs{roots: []string{"Layout"}}, "Fake Boards Service", testSDKModule)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ name, yaml, want string }{
		{"wrap on an API field", "types: {Layout: {title: {wrap: [section]}}}", "the key must be a new attribute name"},
		{"unknown field", "types: {Layout: {w: {wrap: [nope]}}}", "wrap: no field nope"},
		{"skipped field", "types: {Layout: {w: {wrap: [section]}, section: {skip: true}}}", "wrap: section is skipped"},
		{"two wrappers", "types: {Layout: {a: {wrap: [section]}, b: {wrap: [section]}}}", "wrap: section is also in a"},
		{"split oneOf", "types: {Layout: {w: {wrap: [refreshOff]}}}", "must be in one wrapper or in none"},
		{"bad name", "types: {Layout: {Wrap: {wrap: [section]}}}", `"Wrap" is not a valid Terraform attribute name`},
		{"default", "types: {Layout: {w: {wrap: [section], default: x}}}", "a wrapper can only set wrap, required, optional, computed, and useStateForUnknown"},
		{"name clash", "types: {Layout: {w: {wrap: [section]}, title: {name: w}}}", `both have the Terraform name "w"`},
		{"on a oneOf", "types: {TextStyle: {w: {wrap: [bold, font]}}}", "wrap needs an object, TextStyle is a oneOf"},
	} {
		t.Run(c.name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "overrides.yaml")
			if err := os.WriteFile(p, []byte(c.yaml), 0o644); err != nil {
				t.Fatal(err)
			}
			ov, err := loadOverrides(p)
			if err == nil {
				_, err = generateTypes(types, enums, refs, ov, "fakewrap", "")
			}
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("error:\n%v\nwant it to contain:\n%s", err, c.want)
			}
		})
	}
}

// TestUnwrapRejects checks the errors of the unwrap overrides (D21).
func TestUnwrapRejects(t *testing.T) {
	types, enums, refs, err := checkedTypeNames("../../spec/fake/openapi.yaml", typeInputs{roots: []string{"Query"}}, "Fake Boards Service", testSDKModule)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ name, yaml, want string }{
		{"unknown object", "unwrap: [Nope]", "unwrap.Nope: no such object in the generated types"},
		{"listed twice", "unwrap: [LuceneQuery, LuceneQuery]", "unwrap.LuceneQuery: listed twice"},
		{"two fields", "unwrap: [ObservationField]", "the object has 2 fields, unwrap needs exactly one"},
		{"a oneOf", "unwrap: [QuerySource]", "a oneOf cannot be unwrapped"},
		{"a root", "unwrap: [Query]", "a root type cannot be unwrapped"},
		{"flags on the value", "unwrap: [LuceneQuery]\ntypes: {LuceneQuery: {value: {required: true}}}",
			"the field of an unwrapped object can only have set, missingAsZero, and emptyAsNull"},
		{"read rule on the user", "unwrap: [LuceneQuery]\ntypes: {QuerySource: {luceneQuery: {missingAsZero: true}}}",
			"put missingAsZero and emptyAsNull on types.LuceneQuery.value"},
		{"computed object value", "unwrap: [FieldSource]\ntypes: {Query: {field: {required: false, computed: true}}}",
			"a computed field of an unwrapped object whose value is an object is not supported"},
	} {
		t.Run(c.name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "overrides.yaml")
			if err := os.WriteFile(p, []byte(c.yaml), 0o644); err != nil {
				t.Fatal(err)
			}
			ov, err := loadOverrides(p)
			if err == nil {
				_, err = generateTypes(types, enums, refs, ov, "fakeunwrap", "")
			}
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("error:\n%v\nwant it to contain:\n%s", err, c.want)
			}
		})
	}
}

// TestInlineRejects checks the errors of the inline, int64, and string
// overrides (D21, F67, F68).
func TestInlineRejects(t *testing.T) {
	in := typeInputs{roots: []string{"Key", "Layout", "Query", "Routing"}}
	types, enums, refs, err := checkedTypeNames("../../spec/fake/openapi.yaml", in, "Fake Boards Service", testSDKModule)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ name, yaml, want string }{
		{"stale", "types: {Key: {nope: {inline: true}}}", "types.Key.nope: no such field"},
		{"with flags", "types: {Key: {keyPermissions: {inline: true, required: true}}}", "inline cannot be combined with other overrides"},
		{"on a scalar", "types: {Key: {name: {inline: true}}}", "inline needs an object, the field is a string"},
		{"on a list", "types: {Routing: {targets: {inline: true}}}", "inline needs an object, the field is a list"},
		{"on a oneOf arm", "types: {TextStyle: {font: {inline: true}}}", "a oneOf arm cannot be inlined"},
		{"on a wrapped field", "types: {Key: {w: {wrap: [keyPermissions]}, keyPermissions: {inline: true}}}", "a wrapped field cannot be inlined"},
		{"an unwrapped object", "unwrap: [FieldSource]\ntypes: {Query: {field: {inline: true}}}", "FieldSource is unwrapped"},
		{"groups inside", "types: {Section: {interval: {inline: true}}}", "Interval has oneOf groups, which is not supported"},
		{"inline inside", "types: {Key: {rotation: {inline: true}}, KeyRotation: {permissions: {inline: true}}}",
			"KeyRotation has an inlined field, which is not supported"},
		{"name clash", "types: {Key: {keyPermissions: {inline: true}}, Key.Permissions: {presets: {name: name}}}",
			`name and keyPermissions.presets both have the Terraform name "name"`},
		{"int64 without the pattern", "types: {Key: {name: {int64: true}}}", `int64: the field is a string with the pattern "", it must be a string with the pattern ^-?[0-9]+$ or ^[0-9]+$`},
		{"int64 on a number", "types: {Key: {ownerTeamId: {int64: true}}}", "int64: the field is a integer int64"},
		{"string on a string", "types: {Key: {name: {string: true}}}", "string: the field is a string, it must be an int64 JSON number"},
		{"string on an int32", "types: {KeyLimits: {perMinute: {string: true}}}", "string: the field is a integer int32"},
		{"int64 and string", "types: {Key: {maxCount: {int64: true, string: true}}}", "int64 and string cannot be combined"},
		{"wide on an int64", "types: {Key: {ownerTeamId: {wide: true}}}", "wide: the field is a integer int64, not an int32 or a float"},
		{"defaultObject on a string", "types: {Key: {name: {defaultObject: true}}}", "defaultObject needs an object with no default, the field is a string"},
		{"defaultObject not computed", "types: {Key: {rotation: {defaultObject: true, computed: false}}}", "a default needs a computed attribute"},
	} {
		t.Run(c.name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "overrides.yaml")
			if err := os.WriteFile(p, []byte(c.yaml), 0o644); err != nil {
				t.Fatal(err)
			}
			ov, err := loadOverrides(p)
			if err == nil {
				_, err = generateTypes(types, enums, refs, ov, "fakeinline", "")
			}
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("error:\n%v\nwant it to contain:\n%s", err, c.want)
			}
		})
	}
}

// TestCustomRejects checks the errors of the custom and namesArm overrides
// (D21).
func TestCustomRejects(t *testing.T) {
	in := typeInputs{roots: []string{"Alarm", "Routing"}}
	types, enums, refs, err := checkedTypeNames("../../spec/fake/openapi.yaml", in, "Fake Boards Service", testSDKModule)
	if err != nil {
		t.Fatal(err)
	}
	const pkg = "customPackage: example.com/custom\n"
	const window = "{type: String, shape: fd12ebc42bf2}"
	for _, c := range []struct{ name, yaml, want string }{
		{"custom with flags", pkg + "types: {MetricRule: {ofTheLast: {custom: " + window + ", required: true}}}", "custom can only be combined with name"},
		{"custom type", pkg + "types: {MetricRule: {ofTheLast: {custom: {type: Text, shape: fd12ebc42bf2}}}}", `custom: type "Text" is not one of`},
		{"stale shape", pkg + "types: {MetricRule: {ofTheLast: {custom: {type: String, shape: abc}}}}",
			"the API type is not the one that the handwritten converter was written for (shape fd12ebc42bf2"},
		{"custom arm", pkg + "types: {Alarm: {metricRule: {custom: " + window + "}}}", "a oneOf arm cannot be custom"},
		{"no package", "types: {MetricRule: {ofTheLast: {custom: " + window + "}}}", "a custom field needs customPackage"},
		{"package without custom", pkg, "customPackage is set, but no field is custom"},
		{"namesArm with flags", "types: {Alarm: {type: {namesArm: true, required: true}}}", "namesArm cannot be combined with other overrides"},
		{"namesArm on a string", "types: {Alarm: {name: {namesArm: true}}}", "namesArm needs an enum, the field is a string"},
		{"namesArm without a group", "types: {Routing: {delivery: {namesArm: true}}}", "namesArm needs exactly one oneOf group in Routing, it has 0"},
	} {
		t.Run(c.name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "overrides.yaml")
			if err := os.WriteFile(p, []byte(c.yaml), 0o644); err != nil {
				t.Fatal(err)
			}
			ov, err := loadOverrides(p)
			if err == nil {
				_, err = generateTypes(types, enums, refs, ov, "fakealarm", "")
			}
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("error:\n%v\nwant it to contain:\n%s", err, c.want)
			}
		})
	}
}

// TestArmValues checks that namesArm pairs each arm with one enum value by
// the name rule, and fails for an arm or a value without a pair.
func TestArmValues(t *testing.T) {
	enum := &model.Type{Kind: model.Enum, Schema: "RuleType", EnumPrefix: "RULE_TYPE_",
		Values: []string{"RULE_TYPE_LOGS_RULE_OR_UNSPECIFIED", "RULE_TYPE_METRIC_RULE", "RULE_TYPE_SPAN_RULE"}}
	obj := &model.Type{Kind: model.Object, Schema: "Rule", Groups: []model.OneOfGroup{{Arms: []string{"logsRule", "metricRule"}}}}
	f := &model.Field{Name: "type", Type: enum}
	_, err := armValues(obj, f)
	if err == nil || err.Error() != "namesArm: the value RULE_TYPE_SPAN_RULE has no arm" {
		t.Errorf("a value without an arm: %v", err)
	}
	obj.Groups[0].Arms = append(obj.Groups[0].Arms, "spanRule", "traceRule")
	if _, err := armValues(obj, f); err == nil || err.Error() != "namesArm: the arm traceRule has no value" {
		t.Errorf("an arm without a value: %v", err)
	}
	obj.Groups[0].Arms = obj.Groups[0].Arms[:3]
	got, err := armValues(obj, f)
	if err != nil || got["logsRule"] != "RULE_TYPE_LOGS_RULE_OR_UNSPECIFIED" || got["spanRule"] != "RULE_TYPE_SPAN_RULE" {
		t.Errorf("pairs = %v, %v", got, err)
	}
}

// TestEnumAcceptZero checks that acceptZero: false keeps the zero value's
// name for reading (ByName, Name) but not in the names a configuration may
// use (Names).
func TestEnumAcceptZero(t *testing.T) {
	types, enums, refs, err := checkedTypeNames("../../spec/fake/openapi.yaml", typeInputs{roots: []string{"Routing"}}, "Fake Boards Service", testSDKModule)
	if err != nil {
		t.Fatal(err)
	}
	f := false
	ov := &overrides{Enums: map[string]enumOverride{"Delivery": {TerraformNames: true, Zero: "unspecified", AcceptZero: &f}}}
	files, err := generateTypes(types, enums, refs, ov, "fakerouting", "")
	if err != nil {
		t.Fatal(err)
	}
	src := string(files["enums.go"])
	names := src[strings.Index(src, "var DeliveryNames"):]
	names = names[:strings.Index(names, "}")]
	if strings.Contains(names, "unspecified") || !strings.Contains(names, "errors_only") {
		t.Errorf("DeliveryNames:\n%s\nwant errors_only and no unspecified", names)
	}
	if !strings.Contains(src, `"unspecified": fake_boards_service.DELIVERY_DELIVERY_UNSPECIFIED`) {
		t.Error("DeliveryByName has no unspecified")
	}
	bad := &overrides{Enums: map[string]enumOverride{"Delivery": {TerraformNames: true, AcceptZero: &f}}}
	if _, err := generateTypes(types, enums, refs, bad, "fakerouting", ""); err == nil || !strings.Contains(err.Error(), "acceptZero needs zero") {
		t.Errorf("acceptZero without zero: error = %v", err)
	}
}
