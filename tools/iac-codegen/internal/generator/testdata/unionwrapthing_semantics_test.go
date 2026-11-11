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

// The variant of the API follows the arm that is set, in both requests: the arms are in a group of the
// resource, so the line reaches them through it. The response variant is ignored on read.
func TestVariantFollowsTheArmThatIsSet(t *testing.T) {
	ctx := context.Background()
	for name, test := range map[string]struct {
		kind *UnionWrapThingKindModel
		want union_wrap_things_service.ThingVariant
	}{
		"arm a": {&UnionWrapThingKindModel{ArmA: &ArmAModel{Value: types.StringValue("v")}}, union_wrap_things_service.THINGVARIANT_THING_VARIANT_A_OR_UNSPECIFIED},
		"arm b": {&UnionWrapThingKindModel{ArmB: &ArmBModel{Count: types.Int32Value(1)}}, union_wrap_things_service.THINGVARIANT_THING_VARIANT_B},
		"arm c": {&UnionWrapThingKindModel{ArmC: &ArmCModel{Label: types.StringValue("l")}}, union_wrap_things_service.THINGVARIANT_THING_VARIANT_C},
	} {
		t.Run(name, func(t *testing.T) {
			m := &UnionWrapThingModel{Name: types.StringValue("n"), Description: types.StringNull(), Kind: test.kind}
			create, diags := expandCreate(ctx, m)
			if diags.HasError() {
				t.Fatal(diags)
			}
			update, diags := expandUpdate(ctx, m)
			if diags.HasError() {
				t.Fatal(diags)
			}
			for request, thing := range map[string]*union_wrap_things_service.Thing{"create": create.Thing, "update": update.Thing} {
				if thing.Variant == nil || *thing.Variant != test.want {
					t.Errorf("%s variant = %v, want %s", request, thing.Variant, test.want)
				}
			}
		})
	}
	// No arm sends no variant, and a response with another variant reads the same.
	none, diags := expandCreate(ctx, &UnionWrapThingModel{Name: types.StringValue("n"), Description: types.StringNull(), Kind: &UnionWrapThingKindModel{}})
	if diags.HasError() || none.Thing.Variant != nil {
		t.Fatalf("variant without an arm = %v (%v), want none", none.Thing.Variant, diags)
	}
	variant := union_wrap_things_service.THINGVARIANT_THING_VARIANT_C
	id := "thing-1"
	got, diags := flatten(ctx, &union_wrap_things_service.UnionWrapThing{Id: &id, Thing: &union_wrap_things_service.Thing{Name: "n", Variant: &variant, ArmA: &union_wrap_things_service.ArmA{Value: "v"}}})
	if diags.HasError() || got.Kind == nil || got.Kind.ArmA == nil {
		t.Fatalf("flatten = %+v (%v), want the arm of the response, not the variant", got, diags)
	}
}
