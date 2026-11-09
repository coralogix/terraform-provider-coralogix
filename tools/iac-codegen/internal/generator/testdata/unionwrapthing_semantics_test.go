package unionwrapthing

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"

	"example.com/iac-test-sdk/go/openapi/gen/union_wrap_things_service"
)

// Create and Update send the inlined object, with the common fields and the selected arm in it,
// whether or not the SDK holds the object in a pointer.
func TestRequestsSendTheInlinedObject(t *testing.T) {
	ctx := context.Background()
	m := &UnionWrapThingModel{
		Name:        types.StringValue("n"),
		Description: types.StringNull(),
		Kind:        &UnionWrapThingKindModel{ArmB: &ArmBModel{Count: types.Int32Value(3)}},
	}
	create, diags := expandCreate(ctx, m)
	if diags.HasError() {
		t.Fatal(diags)
	}
	update, diags := expandUpdate(ctx, m)
	if diags.HasError() {
		t.Fatal(diags)
	}
	for name, thing := range map[string]*union_wrap_things_service.Thing{"create": create.Thing, "update": update.Thing} {
		if thing == nil || thing.Name != "n" {
			t.Fatalf("%s thing = %+v, want name n", name, thing)
		}
		if thing.Description != nil {
			t.Fatalf("%s description = %v, want nil", name, *thing.Description)
		}
		if thing.ArmB == nil || thing.ArmB.Count == nil || *thing.ArmB.Count != 3 || thing.ArmA != nil || thing.ArmC != nil {
			t.Fatalf("%s arms = %+v %+v %+v, want only arm b with count 3", name, thing.ArmA, thing.ArmB, thing.ArmC)
		}
	}
}

// A response reads the fields and the arm of the object as top-level attributes.
func TestResponseFlattensTheInlinedObject(t *testing.T) {
	id, description := "thing-1", "d"
	got, diags := flatten(context.Background(), &union_wrap_things_service.UnionWrapThing{
		Id:    &id,
		Thing: &union_wrap_things_service.Thing{Name: "n", Description: &description, ArmA: &union_wrap_things_service.ArmA{Value: "v"}},
	})
	if diags.HasError() {
		t.Fatal(diags)
	}
	if got.Id.ValueString() != "thing-1" || got.Name.ValueString() != "n" || got.Description.ValueString() != "d" {
		t.Fatalf("model = %+v, want id thing-1, name n, description d", got)
	}
	if got.Kind == nil || got.Kind.ArmA == nil || got.Kind.ArmA.Value.ValueString() != "v" || got.Kind.ArmB != nil || got.Kind.ArmC != nil {
		t.Fatalf("kind = %+v, want only arm a with value v", got.Kind)
	}
}

// The API always has the object, so a response without it has no resource.
func TestResponseWithoutTheInlinedObjectIsAnError(t *testing.T) {
	id := "thing-1"
	_, diags := flatten(context.Background(), &union_wrap_things_service.UnionWrapThing{Id: &id})
	if !diags.HasError() || !strings.Contains(diags[0].Detail(), "no thing") {
		t.Fatalf("diagnostics = %v, want an error about the missing thing", diags)
	}
}
