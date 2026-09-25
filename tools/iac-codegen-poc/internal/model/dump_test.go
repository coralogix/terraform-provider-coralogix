package model_test

import (
	"flag"
	"os"
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/internal/model"
)

var update = flag.Bool("update", false, "rewrite the golden files")

const (
	patchedSpec = "../../spec/openapi.patched.yaml"
	golden      = "testdata/ai_evaluation.golden"
)

// TestDump compares the AiEvaluation model with the golden file. To rewrite
// the file, run: go test ./internal/model -run TestDump -update
func TestDump(t *testing.T) {
	data, err := os.ReadFile(patchedSpec)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := model.Load(data)
	if err != nil {
		t.Fatal(err)
	}
	r, err := model.Build(doc, "AiEvaluation")
	if err != nil {
		t.Fatal(err)
	}
	got := model.Dump(r)
	if *update {
		if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Errorf("dump differs from %s. Run with -update and check the diff.\ngot:\n%s", golden, got)
	}
}
