package suppressedthing

import (
	"context"
	"testing"

	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"example.com/iac-test-sdk/go/openapi/gen/suppressed_things_service"
)

func ptr[T any](v T) *T { return &v }

func model(id types.String) *SuppressedThingModel {
	return &SuppressedThingModel{Id: id, Name: types.StringValue("rule"), TimeFrame: &WindowModel{TimeZone: types.StringValue("UTC+2")}, Operation: types.StringValue("active")}
}

// The server sets the id: Create sends no id, also when the plan has one.
func TestCreateSendsNoID(t *testing.T) {
	for _, id := range []types.String{types.StringUnknown(), types.StringNull(), types.StringValue("set")} {
		body, diags := expandCreate(context.Background(), model(id))
		if diags.HasError() {
			t.Fatal(diags)
		}
		if body.SuppressedThing.UniqueIdentifier != nil || body.SuppressedThing.Id != nil {
			t.Fatalf("id %v: Create sent uniqueIdentifier %v and id %v, want neither", id, body.SuppressedThing.UniqueIdentifier, body.SuppressedThing.Id)
		}
	}
}

// Update names the resource by the id property in its body, with the id of the state. The version
// id is not sent.
func TestUpdateSendsTheIDProperty(t *testing.T) {
	ctx := context.Background()
	suppressed_things_service.Stored = &suppressed_things_service.SuppressedThing{UniqueIdentifier: ptr("rule-1"), Name: "renamed", ScheduleOperation: suppressed_things_service.THINGOPERATION_THING_OPERATION_ACTIVATE}
	suppressed_things_service.Sent = nil
	r := &Resource{client: &suppressed_things_service.SuppressedThingsServiceAPIService{}}
	var schemaResp frameworkresource.SchemaResponse
	r.Schema(ctx, frameworkresource.SchemaRequest{}, &schemaResp)
	root := schemaResp.Schema.Type().TerraformType(ctx)
	window := root.(tftypes.Object).AttributeTypes["time_frame"]
	value := func(id any, name string) tftypes.Value {
		return tftypes.NewValue(root, map[string]tftypes.Value{
			"id":         tftypes.NewValue(tftypes.String, id),
			"name":       tftypes.NewValue(tftypes.String, name),
			"time_frame": tftypes.NewValue(window, nil),
			"operation":  tftypes.NewValue(tftypes.String, "active"),
		})
	}
	state := tfsdk.State{Schema: schemaResp.Schema, Raw: value("rule-1", "rule")}
	plan := tfsdk.Plan{Schema: schemaResp.Schema, Raw: value("rule-1", "renamed")}
	config := tfsdk.Config{Schema: schemaResp.Schema, Raw: value(nil, "renamed")}
	resp := &frameworkresource.UpdateResponse{State: state}
	r.Update(ctx, frameworkresource.UpdateRequest{Config: config, Plan: plan, State: state}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	got := suppressed_things_service.Sent
	if got == nil || got.UniqueIdentifier == nil || *got.UniqueIdentifier != "rule-1" || got.Id != nil || got.Name != "renamed" {
		t.Fatalf("Update sent %+v, want uniqueIdentifier rule-1, no version id, and name renamed", got)
	}
	if got.ScheduleOperation != suppressed_things_service.THINGOPERATION_THING_OPERATION_ACTIVATE {
		t.Fatalf("operation active was sent as %q, want THING_OPERATION_ACTIVATE", got.ScheduleOperation)
	}
}

// The id attribute is the id property. The version id, which every Update replaces, is not
// stored, and a response without the id property is an error.
func TestFlattenReadsTheIDProperty(t *testing.T) {
	m, diags := flatten(context.Background(), &suppressed_things_service.SuppressedThing{
		UniqueIdentifier: ptr("rule-1"), Id: ptr("version-7"), Name: "rule", ScheduleOperation: suppressed_things_service.THINGOPERATION_THING_OPERATION_MUTE,
		Window: &suppressed_things_service.Window{Timezone: "UTC+2"},
	})
	if diags.HasError() {
		t.Fatal(diags)
	}
	if m.Id.ValueString() != "rule-1" || m.TimeFrame.TimeZone.ValueString() != "UTC+2" || m.Operation.ValueString() != "mute" {
		t.Fatalf("flatten = id %v, time_zone %v, operation %v; want rule-1, UTC+2, and mute", m.Id, m.TimeFrame.TimeZone, m.Operation)
	}
	if _, diags := flatten(context.Background(), &suppressed_things_service.SuppressedThing{Id: ptr("version-7"), Name: "rule"}); !diags.HasError() {
		t.Fatal("a response without uniqueIdentifier must be an error: Read, Update, and Delete need the id")
	}
}
