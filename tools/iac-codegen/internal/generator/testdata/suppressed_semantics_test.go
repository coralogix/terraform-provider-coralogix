package suppressedthing

import (
	"context"
	"slices"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"example.com/iac-test-sdk/go/openapi/gen/suppressed_things_service"
)

func ptr[T any](v T) *T { return &v }

func model(id types.String) *SuppressedThingModel {
	return &SuppressedThingModel{Id: id, Name: types.StringValue("rule"), TimeFrame: &WindowModel{TimeZone: types.StringValue("UTC+2"), Days: types.SetNull(types.StringType)}, Operation: types.StringValue("active")}
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
	value := func(id any, name string) tftypes.Value {
		// Every other attribute is null.
		attrs := map[string]tftypes.Value{}
		for attr, typ := range root.(tftypes.Object).AttributeTypes {
			attrs[attr] = tftypes.NewValue(typ, nil)
		}
		attrs["id"] = tftypes.NewValue(tftypes.String, id)
		attrs["name"] = tftypes.NewValue(tftypes.String, name)
		attrs["operation"] = tftypes.NewValue(tftypes.String, "active")
		return tftypes.NewValue(root, attrs)
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

// days shows the weekday numbers 1 to 7 of the API as Sunday to Saturday.
func TestDaysAreNamed(t *testing.T) {
	ctx := context.Background()
	m := model(types.StringValue("rule-1"))
	m.TimeFrame.Days = types.SetValueMust(types.StringType, []attr.Value{types.StringValue("Sunday"), types.StringValue("Saturday")})
	body, diags := expandCreate(ctx, m)
	if diags.HasError() {
		t.Fatal(diags)
	}
	got := slices.Sorted(slices.Values(body.SuppressedThing.Window.DaysOfWeek))
	if !slices.Equal(got, []int32{1, 7}) {
		t.Fatalf("days Sunday and Saturday were sent as %v, want [1 7]", got)
	}

	flat, diags := flatten(ctx, &suppressed_things_service.SuppressedThing{
		UniqueIdentifier: ptr("rule-1"), Name: "rule", ScheduleOperation: suppressed_things_service.THINGOPERATION_THING_OPERATION_MUTE,
		Window: &suppressed_things_service.Window{Timezone: "UTC", DaysOfWeek: []int32{2}},
	})
	if diags.HasError() {
		t.Fatal(diags)
	}
	if want := types.SetValueMust(types.StringType, []attr.Value{types.StringValue("Monday")}); !flat.TimeFrame.Days.Equal(want) {
		t.Fatalf("days [2] read back as %v, want [Monday]", flat.TimeFrame.Days)
	}
}

// A name or a number outside the valueNames line is an error, not a silent zero or an empty name.
func TestUnknownDaysAreErrors(t *testing.T) {
	ctx := context.Background()
	m := model(types.StringValue("rule-1"))
	m.TimeFrame.Days = types.SetValueMust(types.StringType, []attr.Value{types.StringValue("Funday")})
	if _, diags := expandCreate(ctx, m); !diags.HasError() {
		t.Fatal("the name Funday must be an error")
	}
	_, diags := flatten(ctx, &suppressed_things_service.SuppressedThing{
		UniqueIdentifier: ptr("rule-1"), Name: "rule", ScheduleOperation: suppressed_things_service.THINGOPERATION_THING_OPERATION_MUTE,
		Window: &suppressed_things_service.Window{Timezone: "UTC", DaysOfWeek: []int32{9}},
	})
	if !diags.HasError() {
		t.Fatal("the API value 9 must be an error")
	}
}

// always_active shows the empty arm alwaysActive as a bool: true sends the empty object, and a
// response that has it reads as true; one without it reads as null.
func TestAlwaysActiveIsABool(t *testing.T) {
	ctx := context.Background()
	m := model(types.StringValue("rule-1"))
	m.Repeat = &RepeatModel{AlwaysActive: types.BoolValue(true)}
	body, diags := expandCreate(ctx, m)
	if diags.HasError() {
		t.Fatal(diags)
	}
	if got := body.SuppressedThing.Repeat; got == nil || got.AlwaysActive == nil || len(got.AlwaysActive) != 0 || got.Dynamic != nil {
		t.Fatalf("always_active = true sent %#v, want the empty alwaysActive arm alone", got)
	}

	m.Repeat = &RepeatModel{AlwaysActive: types.BoolValue(false)}
	if _, diags := expandCreate(ctx, m); !diags.HasError() {
		t.Fatal("always_active = false must be an error, as in the released resource")
	}

	thing := &suppressed_things_service.SuppressedThing{
		UniqueIdentifier: ptr("rule-1"), Name: "rule", ScheduleOperation: suppressed_things_service.THINGOPERATION_THING_OPERATION_MUTE,
		Repeat: &suppressed_things_service.Repeat{AlwaysActive: map[string]interface{}{}},
	}
	flat, diags := flatten(ctx, thing)
	if diags.HasError() {
		t.Fatal(diags)
	}
	if !flat.Repeat.AlwaysActive.Equal(types.BoolValue(true)) || flat.Repeat.Dynamic != nil {
		t.Fatalf("the alwaysActive arm read back as %v, want always_active = true", flat.Repeat)
	}

	thing.Repeat = &suppressed_things_service.Repeat{Dynamic: &suppressed_things_service.Dynamic{Every: "day"}}
	flat, diags = flatten(ctx, thing)
	if diags.HasError() {
		t.Fatal(diags)
	}
	if !flat.Repeat.AlwaysActive.IsNull() {
		t.Fatalf("always_active read back as %v without the arm, want null", flat.Repeat.AlwaysActive)
	}
}
