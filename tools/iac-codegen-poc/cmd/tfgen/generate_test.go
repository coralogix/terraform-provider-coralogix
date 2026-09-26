package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

// fakeSDK is the fake SDK module (fakesdk/generate.sh).
const fakeSDK = "github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/fakesdk"

// generatedCases are the generated resources and their inputs.
var generatedCases = []struct{ dir, spec, resource, sdk, acc string }{
	{"../../generated/aievaluation", patchedSpec, "AiEvaluation", realSDK, "../../spec/acc/AiEvaluation.yaml"},
	{"../../generated/fakeboard", "../../spec/fake/openapi.yaml", "FakeBoard", fakeSDK, ""},
	{"../../generated/fakesettings", "../../spec/fake/settings.yaml", "FakeSettings", fakeSDK, ""},
	{"../../generated/fakerule", "../../spec/fake/rules.yaml", "FakeRule", fakeSDK, ""},
	{"../../generated/fakeview", "../../spec/fake/views.yaml", "FakeView", fakeSDK, ""},
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
	r, refs, err := checkedSDKNames("../../spec/fake/openapi.yaml", "FakeBoard", fakeSDK)
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
	{"../../generated/fakepanel", "../../spec/fake/openapi.yaml", "Fake Boards Service", fakeSDK, []string{"Panel", "Header", "Interval", "AbsoluteTime"},
		[]string{"Color", "Unit", "Orientation", "Comparison", "Delivery"}, ""},
	{"../../generated/dashboardwidgets", patchedSpec, "Dashboard service", realSDK, []string{"Widget.Definition"}, nil, ""},
	{"../../generated/fakerouting", "../../spec/fake/openapi.yaml", "Fake Boards Service", fakeSDK, []string{"Routing"}, nil,
		"../../spec/fake/routing.overrides.yaml"},
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
			_, _, _, err := checkedTypeNames(spec, typeInputs{roots: c.roots}, c.tag, fakeSDK)
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
		typeInputs{enums: []string{"Orientation", "Comparison", "Delivery", "Color"}}, "Fake Boards Service", fakeSDK)
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
	if _, _, _, err := checkedTypeNames("../../spec/fake/openapi.yaml", typeInputs{enums: []string{"Panel"}}, "Fake Boards Service", fakeSDK); err == nil {
		t.Error("an object in --enums: no error")
	}
}

// TestOverridesRejects checks the overrides files that the type mode
// rejects (D21). A stale or wrong override is an error, not ignored.
func TestOverridesRejects(t *testing.T) {
	const spec = "../../spec/fake/openapi.yaml"
	in := typeInputs{roots: []string{"Routing"}}
	types, enums, refs, err := checkedTypeNames(spec, in, "Fake Boards Service", fakeSDK)
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
		{"default not computed", `types: {Routing: {weight: {default: "1", computed: false}}}`, "a default needs an optional and computed attribute"},
		{"every field skipped", "types: {Target: {connectorId: {skip: true}, tags: {skip: true}}}", "types.Target: every field is skipped"},
		{"read only and required", "types: {Routing: {weight: {readOnly: true, required: true}}}", "readOnly cannot be combined"},
		{"zero and null", "types: {Routing: {channels: {missingAsZero: true, emptyAsNull: true}}}", "cannot be combined"},
		{"null of a scalar", "types: {Routing: {weight: {emptyAsNull: true}}}", "emptyAsNull: the field is a number"},
		{"zero of an enum", "types: {Routing: {delivery: {missingAsZero: true}}}", "missingAsZero is not supported for enum"},
		{"zero of a time", "types: {Routing: {createTime: {missingAsZero: true}}}", "missingAsZero is not supported for time"},
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
	types, enums, refs, err := checkedTypeNames("../../spec/fake/openapi.yaml", typeInputs{roots: []string{"Section"}}, "Fake Boards Service", fakeSDK)
	if err != nil {
		t.Fatal(err)
	}
	_, err = generateTypes(types, enums, refs, &overrides{WideNumbers: true}, "fake", "")
	if err == nil || !strings.Contains(err.Error(), "wideNumbers: a list of integer int32 is not supported") {
		t.Errorf("error = %v", err)
	}
}
