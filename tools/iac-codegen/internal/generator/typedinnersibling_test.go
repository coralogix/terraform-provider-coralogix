package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// innerSiblingTest reads the notEquals arm of the operator. The inner selection of the other arm
// (equals) has a collection beside its arms that notEquals lacks, so the merged operator holds the
// collection for the equals arm only. The read of notEquals must still give it a typed null, or
// the model cannot be written to state.
const innerSiblingTest = `package typedthing

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"example.com/iac-test-sdk/go/openapi/gen/typed_things_service"
)

func TestReadingTheOtherArmGivesTheInnerSiblingATypedNull(t *testing.T) {
	ctx := context.Background()
	var diags diag.Diagnostics
	notEquals := &typed_things_service.TypedOperator{NotEquals: &typed_things_service.TypedNotEquals{
		Selection: &typed_things_service.TypedNotSelection{List: &typed_things_service.TypedNotList{Values: []string{"a"}}},
	}}
	model := flattenTypedOperator(ctx, path.Empty(), notEquals, nil, &diags)
	if diags.HasError() {
		t.Fatal(diags)
	}
	if !model.Extra.IsNull() {
		t.Fatalf("extra = %v, want null", model.Extra)
	}
	var resp frameworkresource.SchemaResponse
	(&Resource{}).Schema(ctx, frameworkresource.SchemaRequest{}, &resp)
	attrTypes := resp.Schema.Attributes["operator"].(schema.SingleNestedAttribute).GetType().(types.ObjectType).AttrTypes
	if _, d := types.ObjectValueFrom(ctx, attrTypes, model); d.HasError() {
		t.Fatalf("the model cannot be written to state: %v", d)
	}
}
`

// An inner typedArms object (the selection) gains a collection beside its arms. The released shape
// stays: the collection is one more attribute of the operator. The arm that has no such collection
// reads it as a typed null.
func TestAnInnerSiblingCollectionIsInitializedInTheOtherArm(t *testing.T) {
	for _, c := range []struct {
		name, property, goType, attrType string
	}{
		{"list", "{type: array, items: {type: string}}", "[]string", "ListType"},
		{"set", "{type: array, items: {type: string}, x-coralogix-collection: set}", "[]string", "SetType"},
		{"map", "{type: object, additionalProperties: {type: string}}", "map[string]string", "MapType"},
	} {
		t.Run(c.name, func(t *testing.T) {
			input, loadDir := copiedSyntheticInput(t)
			spec, err := os.ReadFile(filepath.Join("testdata", "typedthing.yaml"))
			if err != nil {
				t.Fatal(err)
			}
			anchor := "        list: {$ref: '#/components/schemas/TypedList'}\n"
			input.OpenAPI = []byte(replaceOnce(t, string(spec), anchor, anchor+"        extra: "+c.property+"\n"))
			sdkFile := filepath.Join(input.SDKDir, "go", "openapi", "gen", "typed_things_service", "typed.go")
			sdk, err := os.ReadFile(sdkFile)
			if err != nil {
				t.Fatal(err)
			}
			field := "type TypedSelection struct {\n\tAll  map[string]interface{}\n\tList *TypedList\n"
			patched := replaceOnce(t, string(sdk), field, field+"\tExtra "+c.goType+"\n")
			if err := os.WriteFile(sdkFile, []byte(patched), 0o644); err != nil {
				t.Fatal(err)
			}
			out := filepath.Join(t.TempDir(), "typedthing")
			options := Options{Resource: "TypedThing", OutputDir: out, OverridesPath: filepath.Join("testdata", "typedthing-overrides.yaml")}
			if err := generateFromInput(options, input, loadDir); err != nil {
				t.Fatal(err)
			}
			if schema := readGenerated(t, out, "schema.go"); !strings.Contains(schema, `"extra"`) {
				t.Fatal("the operator has no extra attribute")
			}
			if err := os.WriteFile(filepath.Join(out, "inner_sibling_test.go"), []byte(innerSiblingTest), 0o644); err != nil {
				t.Fatal(err)
			}
			compileGenerated(t, out, input)
		})
	}
}
