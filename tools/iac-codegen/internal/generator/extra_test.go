package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/overrides"
)

// An empty API object still has a model. Its component must identify the Terraform object that
// receives an extra attribute; it has no child field that can identify it.
func TestExtraAttributeOfEmptyNestedObject(t *testing.T) {
	out := &tfResource{
		Attributes: []*tfAttr{{Name: "empty", Kind: "SingleNested", ValueKind: "Object", ObjectComponent: "EmptyConfig"}},
		Models:     []*tfModel{{Name: "EmptyConfigModel"}},
	}
	file := &overrides.File{Resource: "Thing", Types: map[string]overrides.Type{
		"EmptyConfig": {ExtraAttributes: map[string]overrides.ExtraAttribute{
			"url": {Type: "string", MarkdownDescription: "URL."},
		}},
	}}
	if err := attachExtraAttributes(out, file); err != nil {
		t.Fatal(err)
	}
	if len(out.Attributes[0].Attributes) != 1 || out.Attributes[0].Attributes[0].Name != "url" {
		t.Fatalf("empty object attributes = %#v, want url", out.Attributes[0].Attributes)
	}
	if len(out.Models[0].Fields) != 1 || out.Models[0].Fields[0].TFName != "url" {
		t.Fatalf("empty object model fields = %#v, want url", out.Models[0].Fields)
	}
}

func TestExtraAttributeOfEmptyAPIObjectCompiles(t *testing.T) {
	spec, err := os.ReadFile(filepath.Join("testdata", "evalthing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	base, err := os.ReadFile(filepath.Join("testdata", "evalthing-overrides.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	lines := string(base)
	lines = strings.Replace(lines, "mode: existing\n", "mode: existing\nafterFlatten: true\n", 1)
	lines = strings.Replace(lines, "  TopicsEvalConfig:\n", "  ToxicityEvalConfig:\n    extraAttributes:\n      url: {type: string, markdownDescription: URL.}\n  TopicsEvalConfig:\n", 1)
	dir := t.TempDir()
	overridesPath := filepath.Join(dir, "overrides.yaml")
	if err := os.WriteFile(overridesPath, []byte(lines), 0o644); err != nil {
		t.Fatal(err)
	}
	input, loadDir := syntheticInput(t)
	input.OpenAPI = spec
	out := filepath.Join(dir, "evalthing")
	if err := generateFromInput(Options{Resource: "EvalThing", OutputDir: out, OverridesPath: overridesPath}, input, loadDir); err != nil {
		t.Fatal(err)
	}
	generated, err := os.ReadFile(filepath.Join(out, "convert.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(generated), "out.Toxicity.Url = types.StringNull()") {
		t.Fatal("flatten did not initialize the extra URL of the empty API object")
	}
	compileGenerated(t, out, input)
}
