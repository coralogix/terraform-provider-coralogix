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

func TestExtraAttributeOfReusedNestedObject(t *testing.T) {
	first := &tfAttr{Name: "first", ObjectComponent: "SharedConfig", Attributes: []*tfAttr{{Name: "first_field"}}}
	second := &tfAttr{Name: "second", ObjectComponent: "SharedConfig", Attributes: []*tfAttr{{Name: "second_field"}}}
	out := &tfResource{
		Attributes: []*tfAttr{first, second},
		Models:     []*tfModel{{Name: "SharedConfigModel"}},
	}
	file := &overrides.File{Resource: "Thing", Types: map[string]overrides.Type{
		"SharedConfig": {ExtraAttributes: map[string]overrides.ExtraAttribute{
			"url": {Type: "string", MarkdownDescription: "URL."},
		}},
	}}
	if err := attachExtraAttributes(out, file); err != nil {
		t.Fatal(err)
	}
	for _, parent := range []*tfAttr{first, second} {
		if len(parent.Attributes) != 2 || parent.Attributes[1].Name != "url" {
			t.Errorf("%s attributes = %#v, want its API field and url", parent.Name, parent.Attributes)
		}
	}
	if first.Attributes[0].Name != "first_field" || second.Attributes[0].Name != "second_field" {
		t.Fatal("attaching the extra attribute replaced existing fields")
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

func TestComputedEmptyAPIObjectWithExtraCompiles(t *testing.T) {
	spec, err := os.ReadFile(filepath.Join("testdata", "evalthing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	spec = []byte(strings.Replace(string(spec),
		"        enabled: {type: boolean}\n    EvalConfig:",
		"        enabled: {type: boolean}\n        emptyConfig: {$ref: '#/components/schemas/ToxicityEvalConfig', readOnly: true}\n    EvalConfig:", 1))
	base, err := os.ReadFile(filepath.Join("testdata", "evalthing-overrides.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Replace(string(base), "  TopicsEvalConfig:\n",
		"  ToxicityEvalConfig:\n    extraAttributes:\n      url: {type: string, computed: true, markdownDescription: URL.}\n  TopicsEvalConfig:\n", 1)
	dir := t.TempDir()
	overridesPath := filepath.Join(dir, "overrides.yaml")
	if err := os.WriteFile(overridesPath, []byte(lines), 0o644); err != nil {
		t.Fatal(err)
	}
	input, loadDir := copiedSyntheticInput(t)
	input.OpenAPI = spec
	sdkFile := filepath.Join(input.SDKDir, "go", "openapi", "gen", "eval_things_service", "eval.go")
	sdk, err := os.ReadFile(sdkFile)
	if err != nil {
		t.Fatal(err)
	}
	sdk = []byte(strings.Replace(string(sdk),
		"type EvalThing struct {\n", "type EvalThing struct {\n\tEmptyConfig map[string]interface{}\n", 1))
	if err := os.WriteFile(sdkFile, sdk, 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "evalthing")
	if err := generateFromInput(Options{Resource: "EvalThing", OutputDir: out, OverridesPath: overridesPath}, input, loadDir); err != nil {
		t.Fatal(err)
	}
	generated, err := os.ReadFile(filepath.Join(out, "convert.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(generated), `"url": types.StringNull()`) {
		t.Fatal("computed empty object has no typed null for its extra URL")
	}
	const runtimeTest = `package evalthing

import (
	"context"
	"testing"

	"example.com/iac-test-sdk/go/openapi/gen/eval_things_service"
)

func TestComputedEmptyExtraFlatten(t *testing.T) {
	ctx := context.Background()
	value, diags := flatten(ctx, &eval_things_service.EvalThing{EmptyConfig: map[string]interface{}{}})
	if diags.HasError() || value.EmptyConfig.IsNull() {
		t.Fatalf("present empty object: value=%v, diagnostics=%v", value.EmptyConfig, diags)
	}
	attrs := value.EmptyConfig.Attributes()
	if len(attrs) != 1 || !attrs["url"].IsNull() {
		t.Fatalf("extra attributes = %v, want one null URL", attrs)
	}
	nullValue, diags := flatten(ctx, &eval_things_service.EvalThing{})
	if diags.HasError() || !nullValue.EmptyConfig.IsNull() || len(nullValue.EmptyConfig.AttributeTypes(ctx)) != 1 {
		t.Fatalf("missing empty object: value=%v, diagnostics=%v", nullValue.EmptyConfig, diags)
	}
}
`
	if err := os.WriteFile(filepath.Join(out, "extra_computed_test.go"), []byte(runtimeTest), 0o644); err != nil {
		t.Fatal(err)
	}
	compileGenerated(t, out, input)
}
