package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

const generatedDir = "../../generated/aievaluation"

// TestGeneratedUpToDate checks that generated/aievaluation is the output of
// the generator for the patched spec. To rewrite it, run the --out command in
// README.md, "Commands".
func TestGeneratedUpToDate(t *testing.T) {
	r, refs, err := checkedSDKNames(patchedSpec, "AiEvaluation")
	if err != nil {
		t.Fatal(err)
	}
	files, err := generate(r, refs, filepath.Base(generatedDir))
	if err != nil {
		t.Fatal(err)
	}
	for name, got := range files {
		want, err := os.ReadFile(filepath.Join(generatedDir, name))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s is not up to date. Run go run ./cmd/tfgen ... --out generated/aievaluation", name)
		}
	}
	again, err := generate(r, refs, filepath.Base(generatedDir))
	if err != nil {
		t.Fatal(err)
	}
	for name := range files {
		if !bytes.Equal(files[name], again[name]) {
			t.Errorf("%s: two runs give different output", name)
		}
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
