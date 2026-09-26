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
	roots               []string
}{
	{"../../generated/fakepanel", "../../spec/fake/openapi.yaml", "Fake Boards Service", fakeSDK, []string{"Panel", "Header", "Interval", "AbsoluteTime"}},
	{"../../generated/dashboardwidgets", patchedSpec, "Dashboard service", realSDK, []string{"Widget.Definition"}},
}

// TestGeneratedTypesUpToDate checks that each generated type package is the
// output of the generator, and that a second run gives the same files.
func TestGeneratedTypesUpToDate(t *testing.T) {
	for _, c := range generatedTypeCases {
		t.Run(filepath.Base(c.dir), func(t *testing.T) {
			types, refs, err := checkedTypeNames(c.spec, c.roots, c.tag, c.sdk)
			if err != nil {
				t.Fatal(err)
			}
			cmd := typesCommand(c.roots, c.tag)
			files, err := generateTypes(types, refs, filepath.Base(c.dir), cmd)
			if err != nil {
				t.Fatal(err)
			}
			again, err := generateTypes(types, refs, filepath.Base(c.dir), cmd)
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
		{"no tag", []string{"Panel"}, "", "--tag is required with --types"},
		{"unknown type", []string{"Nope"}, "Fake Boards Service", "component Nope not found"},
		{"an enum", []string{"Unit"}, "Fake Boards Service", "want an object or a oneOf with fields"},
		{"no fields", []string{"BoldStyle"}, "Fake Boards Service", "want an object or a oneOf with fields"},
		{"listed twice", []string{"Panel", "Panel"}, "Fake Boards Service", "listed twice"},
		{"another SDK package", []string{"Panel"}, "Fake Settings Service", "fake_settings_service"},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, _, err := checkedTypeNames(spec, c.roots, c.tag, fakeSDK)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("error = %v, want it to contain %q", err, c.want)
			}
		})
	}
}
