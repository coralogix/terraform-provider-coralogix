package generator

import (
	"os"
	"path/filepath"
	"testing"
)

// An empty set inside a chosen oneOf arm takes the same form as the null that the API returns, so
// a set element with an explicit empty set in its arm still pairs with its configured twin.
func TestEmptySetInsideOneOfArm(t *testing.T) {
	out, err := generateMaskThing(t, maskThingSpec(t))
	if err != nil {
		t.Fatal(err)
	}
	semantics := `package maskthing

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestEmptySetInsideOneOfArm(t *testing.T) {
	oneOfArms["zz.items.view"] = true
	defer delete(oneOfArms, "zz.items.view")
	filters := types.SetType{ElemType: types.StringType}
	arm := types.ObjectType{AttrTypes: map[string]attr.Type{"filters": filters}}
	elem := types.ObjectType{AttrTypes: map[string]attr.Type{"view": arm}}
	build := func(f types.Set) types.Set {
		view := types.ObjectValueMust(arm.AttrTypes, map[string]attr.Value{"filters": f})
		return types.SetValueMust(elem, []attr.Value{types.ObjectValueMust(elem.AttrTypes, map[string]attr.Value{"view": view})})
	}
	prior := build(types.SetValueMust(types.StringType, []attr.Value{})) // configured: filters = []
	api := build(types.SetNull(types.StringType))                        // the API answers no filters
	var d diag.Diagnostics
	got := priorEmptySet(context.Background(), "zz.items", api, prior, &d).(types.Set)
	view := got.Elements()[0].(types.Object).Attributes()["view"].(types.Object)
	if view.IsNull() {
		t.Fatal("the arm lost its presence")
	}
	if f := view.Attributes()["filters"]; f.IsNull() {
		t.Fatal("the explicit empty set came back as null")
	}
	// An arm with no configured filters stays present and empty.
	only := build(types.SetNull(types.StringType))
	if v := emptyNormal("zz.items", only.Elements()[0]); v.IsNull() {
		t.Fatal("a chosen arm with nothing inside became null")
	}
}
`
	if err := os.WriteFile(filepath.Join(out, "emptyarm_test.go"), []byte(semantics), 0o644); err != nil {
		t.Fatal(err)
	}
	input, _ := syntheticInput(t)
	compileGenerated(t, out, input)
}
