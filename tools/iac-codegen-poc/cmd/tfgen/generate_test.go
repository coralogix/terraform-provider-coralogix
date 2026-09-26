package main

import (
	"bytes"
	"os"
	"path/filepath"
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
