package thing

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestScalarPresence(t *testing.T) {
	if expandBool(types.BoolNull()) != nil || expandBool(types.BoolUnknown()) != nil {
		t.Fatal("null and unknown bool values must be omitted")
	}
	if got := expandBool(types.BoolValue(false)); got == nil || *got {
		t.Fatalf("explicit false = %v", got)
	}
	if got := expandInt64(types.Int64Value(0)); got == nil || *got != 0 {
		t.Fatalf("explicit zero = %v", got)
	}
	if got := expandString(types.StringValue("")); got == nil || *got != "" {
		t.Fatalf("explicit empty string = %v", got)
	}
}

func TestCollectionPresenceAndStability(t *testing.T) {
	t.Run("list presence and order", testListPresenceAndOrder)
	t.Run("set stability", testSetStability)
	t.Run("map and flatten", testMapAndFlatten)
}

func testListPresenceAndOrder(t *testing.T) {
	ctx := context.Background()
	var diags diag.Diagnostics
	if got := expandStrings[string](ctx, path.Root("ordered"), types.ListNull(types.StringType), &diags); got != nil {
		t.Fatalf("null list = %#v", got)
	}
	if got := expandStrings[string](ctx, path.Root("ordered"), types.ListUnknown(types.StringType), &diags); got != nil {
		t.Fatalf("unknown list = %#v", got)
	}
	empty := types.ListValueMust(types.StringType, nil)
	if got := expandStrings[string](ctx, path.Root("ordered"), empty, &diags); got == nil || len(got) != 0 {
		t.Fatalf("empty list = %#v", got)
	}
	ordered := types.ListValueMust(types.StringType, []attr.Value{types.StringValue("b"), types.StringValue("a")})
	if got := expandStrings[string](ctx, path.Root("ordered"), ordered, &diags); len(got) != 2 || got[0] != "b" || got[1] != "a" {
		t.Fatalf("ordered list = %#v", got)
	}
	if diags.HasError() {
		t.Fatal(diags)
	}
}

func testSetStability(t *testing.T) {
	ctx := context.Background()
	var diags diag.Diagnostics
	first := types.SetValueMust(types.StringType, []attr.Value{types.StringValue("a"), types.StringValue("b")})
	second := types.SetValueMust(types.StringType, []attr.Value{types.StringValue("b"), types.StringValue("a")})
	if !first.Equal(second) {
		t.Fatal("set order changed equality")
	}
	duplicate, duplicateDiags := types.SetValue(types.StringType, []attr.Value{types.StringValue("a"), types.StringValue("a")})
	if duplicateDiags.HasError() {
		t.Fatalf("duplicate set = %#v, %v", duplicate, duplicateDiags)
	}
	if got := expandStringsSet[string](ctx, path.Root("unordered"), duplicate, &diags); len(got) != 1 || got[0] != "a" {
		t.Fatalf("expanded duplicate set = %#v", got)
	}
	if diags.HasError() {
		t.Fatal(diags)
	}
}

func testMapAndFlatten(t *testing.T) {
	ctx := context.Background()
	var diags diag.Diagnostics
	emptyMap := types.MapValueMust(types.StringType, nil)
	if got := expandStringMap[string](ctx, path.Root("labels"), emptyMap, &diags); got == nil || len(got) != 0 {
		t.Fatalf("empty map = %#v", got)
	}
	if !flattenStringsList[string](ctx, nil, &diags).IsNull() || flattenStringsList(ctx, []string{}, &diags).IsNull() {
		t.Fatal("list flatten lost the null and empty distinction")
	}
	if diags.HasError() {
		t.Fatal(diags)
	}
}

func TestUpdateClearArmSwitchAndNoOp(t *testing.T) {
	scalar := &maskNode{attr: "enabled", api: "enabled"}
	if got := maskPaths(nil, "", scalar, types.BoolValue(false), types.BoolNull()); len(got) != 1 || got[0] != "enabled" {
		t.Fatalf("set false mask = %v", got)
	}
	if got := maskPaths(nil, "", scalar, types.BoolNull(), types.BoolValue(false)); len(got) != 1 || got[0] != "enabled" {
		t.Fatalf("clear mask = %v", got)
	}
	if got := maskPaths(nil, "", scalar, types.BoolValue(false), types.BoolValue(false)); len(got) != 0 {
		t.Fatalf("unchanged mask = %v", got)
	}

	armA := &maskNode{attr: "a", api: "a"}
	armB := &maskNode{attr: "b", api: "b"}
	union := &maskNode{attr: "choice", api: "choice", oneOf: true, children: []*maskNode{armA, armB}}
	typesByName := map[string]attr.Type{"a": types.StringType, "b": types.StringType}
	plan := types.ObjectValueMust(typesByName, map[string]attr.Value{"a": types.StringNull(), "b": types.StringValue("new")})
	state := types.ObjectValueMust(typesByName, map[string]attr.Value{"a": types.StringValue("old"), "b": types.StringNull()})
	if got := maskPaths(nil, "", union, plan, state); len(got) != 1 || got[0] != "choice.b" {
		t.Fatalf("arm-switch mask = %v", got)
	}

	values := map[string]attr.Value{
		"name":      types.StringNull(),
		"enabled":   types.BoolNull(),
		"count":     types.Int64Null(),
		"ordered":   types.ListNull(types.StringType),
		"unordered": types.SetNull(types.StringType),
		"labels":    types.MapNull(types.StringType),
	}
	body, diags := updateRequest(context.Background(), fakeData{values: values}, fakeData{values: values})
	if diags.HasError() || body != nil {
		t.Fatalf("unchanged update = %#v, %v", body, diags)
	}
}

type fakeData struct {
	values map[string]attr.Value
}

func (fakeData) Get(context.Context, interface{}) diag.Diagnostics {
	panic("Get must not run for an unchanged plan")
}

func (d fakeData) GetAttribute(_ context.Context, p path.Path, target interface{}) diag.Diagnostics {
	value := target.(*attr.Value)
	*value = d.values[p.String()]
	return nil
}
