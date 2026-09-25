package model_test

import (
	"flag"
	"os"
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/internal/model"
)

var update = flag.Bool("update", false, "rewrite the golden files")

// dumpCases are the resources with a golden dump.
var dumpCases = []struct{ spec, resource, golden string }{
	{"../../spec/openapi.patched.yaml", "AiEvaluation", "testdata/ai_evaluation.golden"},
	{"../../spec/fake/openapi.yaml", "FakeBoard", "testdata/fake_board.golden"},
}

// TestDump compares each model with its golden file. To rewrite the files,
// run: go test ./internal/model -run TestDump -update
func TestDump(t *testing.T) {
	for _, c := range dumpCases {
		t.Run(c.resource, func(t *testing.T) {
			data, err := os.ReadFile(c.spec)
			if err != nil {
				t.Fatal(err)
			}
			doc, err := model.Load(data)
			if err != nil {
				t.Fatal(err)
			}
			r, err := model.Build(doc, c.resource)
			if err != nil {
				t.Fatal(err)
			}
			got := model.Dump(r)
			if *update {
				if err := os.WriteFile(c.golden, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(c.golden)
			if err != nil {
				t.Fatal(err)
			}
			if got != string(want) {
				t.Errorf("dump differs from %s. Run with -update and check the diff.\ngot:\n%s", c.golden, got)
			}
		})
	}
}
