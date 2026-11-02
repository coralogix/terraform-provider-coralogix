package sharedthing

import (
	"context"
	"testing"

	"example.com/iac-test-sdk/go/openapi/gen/shared_things_service"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestUnsupportedRootArmFailsRead(t *testing.T) {
	value := &shared_things_service.SharedThing{ByRatio: &shared_things_service.SharedRatio{}}
	_, diags := flatten(context.Background(), value, nil)
	if !diags.HasError() {
		t.Fatal("a response with the unsupported root arm must fail")
	}
}

func TestAfterFlattenReceivesAPIAndModel(t *testing.T) {
	id, name := "thing-1", "from-api"
	frame := shared_things_service.SHAREDFRAME_SHARED_FRAME_ONE_DAY
	limit := int32(1)
	value := &shared_things_service.SharedThing{
		Id: &id, Name: &name, Frame: &frame,
		ByCount: &shared_things_service.SharedCount{Limit: &limit},
	}
	called := false
	r := &Resource{hooks: Hooks{AfterFlatten: func(_ context.Context, m *SharedThingModel, api *shared_things_service.SharedThing) diag.Diagnostics {
		called = true
		if api != value {
			t.Fatal("hook received a different API value")
		}
		m.Name = types.StringValue("from-hook")
		return nil
	}}}
	state := tfsdk.State{Schema: Schema()}
	if diags := r.setState(context.Background(), value, nil, &state); diags.HasError() {
		t.Fatalf("set state: %v", diags)
	}
	if !called {
		t.Fatal("AfterFlatten was not called")
	}
	var got types.String
	if diags := state.GetAttribute(context.Background(), path.Root("name"), &got); diags.HasError() || got.ValueString() != "from-hook" {
		t.Fatalf("name = %q, diagnostics = %v", got.ValueString(), diags)
	}
}
