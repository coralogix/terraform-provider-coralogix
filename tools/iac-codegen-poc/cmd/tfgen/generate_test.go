package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// fakeSDK is the fake SDK module (fakesdk/generate.sh).
const fakeSDK = "github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/fakesdk"

// generatedCases are the generated resources and their inputs.
var generatedCases = []struct{ dir, spec, resource, sdk string }{
	{"../../generated/aievaluation", patchedSpec, "AiEvaluation", realSDK},
	{"../../generated/fakeboard", "../../spec/fake/openapi.yaml", "FakeBoard", fakeSDK},
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
			files, err := generate(r, refs, filepath.Base(c.dir))
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
			again, err := generate(r, refs, filepath.Base(c.dir))
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
