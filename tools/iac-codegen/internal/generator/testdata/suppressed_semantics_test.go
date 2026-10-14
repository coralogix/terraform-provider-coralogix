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
	return &SuppressedThingModel{UniqueIdentifier: id, Name: types.StringValue("rule"), Window: &WindowModel{Timezone: types.StringValue("UTC+2"), DaysOfWeek: types.SetNull(types.StringType), Hours: types.SetNull(types.Int64Type)}, ScheduleOperation: types.StringValue("active")}
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
	if m.UniqueIdentifier.ValueString() != "rule-1" || m.Window.Timezone.ValueString() != "UTC+2" || m.ScheduleOperation.ValueString() != "mute" {
		t.Fatalf("flatten = id %v, time_zone %v, operation %v; want rule-1, UTC+2, and mute", m.UniqueIdentifier, m.Window.Timezone, m.ScheduleOperation)
	}
	if _, diags := flatten(context.Background(), &suppressed_things_service.SuppressedThing{Id: ptr("version-7"), Name: "rule"}); !diags.HasError() {
		t.Fatal("a response without uniqueIdentifier must be an error: Read, Update, and Delete need the id")
	}
}

// days shows the weekday numbers 1 to 7 of the API as Sunday to Saturday.
func TestDaysAreNamed(t *testing.T) {
	ctx := context.Background()
	m := model(types.StringValue("rule-1"))
	m.Window.DaysOfWeek = types.SetValueMust(types.StringType, []attr.Value{types.StringValue("Sunday"), types.StringValue("Saturday")})
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
	if want := types.SetValueMust(types.StringType, []attr.Value{types.StringValue("Monday")}); !flat.Window.DaysOfWeek.Equal(want) {
		t.Fatalf("days [2] read back as %v, want [Monday]", flat.Window.DaysOfWeek)
	}
}

// A name or a number outside the valueNames line is an error, not a silent zero or an empty name.
func TestUnknownDaysAreErrors(t *testing.T) {
	ctx := context.Background()
	m := model(types.StringValue("rule-1"))
	m.Window.DaysOfWeek = types.SetValueMust(types.StringType, []attr.Value{types.StringValue("Funday")})
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

// The API requires an arm of the filter. A filter without one sends the empty ids arm, which
// the API reads as every alert; a filter with an arm sends only that arm.
func TestFilterWithoutArmSendsTheDefaultArm(t *testing.T) {
	ctx := context.Background()
	m := model(types.StringValue("rule-1"))
	m.Filter = &FilterModel{Ids: types.SetNull(types.StringType)}
	body, diags := expandCreate(ctx, m)
	if diags.HasError() {
		t.Fatal(diags)
	}
	if got := body.SuppressedThing.Filter; got == nil || got.Ids == nil || len(got.Ids.Value) != 0 || got.Labels != nil {
		t.Fatalf("a filter without an arm sent %#v, want the empty ids arm alone", got)
	}

	m.Filter = &FilterModel{Ids: types.SetNull(types.StringType), Labels: &LabelsModel{Key: types.StringValue("team")}}
	body, diags = expandCreate(ctx, m)
	if diags.HasError() {
		t.Fatal(diags)
	}
	if got := body.SuppressedThing.Filter; got == nil || got.Ids != nil || got.Labels == nil {
		t.Fatalf("a filter with labels sent %#v, want the labels arm alone", got)
	}

	m.Filter = &FilterModel{Ids: types.SetValueMust(types.StringType, []attr.Value{types.StringValue("a")})}
	body, diags = expandCreate(ctx, m)
	if diags.HasError() {
		t.Fatal(diags)
	}
	if got := body.SuppressedThing.Filter; got == nil || got.Ids == nil || !slices.Equal(got.Ids.Value, []string{"a"}) {
		t.Fatalf("ids [a] sent %#v, want [a]", got)
	}
}

// frequency is a Terraform-only object that holds the daily and weekly arms, which the API holds in
// the dynamic schedule itself.
func TestFrequencyGroupHoldsTheArms(t *testing.T) {
	ctx := context.Background()
	m := model(types.StringValue("rule-1"))
	m.Repeat = &RepeatModel{AlwaysActive: types.BoolNull(), Dynamic: &DynamicModel{Every: types.StringValue("day"), Frequency: &DynamicFrequencyModel{Daily: &DailyModel{}}}}
	body, diags := expandCreate(ctx, m)
	if diags.HasError() {
		t.Fatal(diags)
	}
	got := body.SuppressedThing.Repeat.Dynamic
	if got == nil || got.Daily == nil || got.Weekly != nil || got.Every != "day" {
		t.Fatalf("frequency.daily sent %#v, want the daily arm of the dynamic schedule", got)
	}

	flat, diags := flatten(ctx, &suppressed_things_service.SuppressedThing{
		UniqueIdentifier: ptr("rule-1"), Name: "rule", ScheduleOperation: suppressed_things_service.THINGOPERATION_THING_OPERATION_MUTE,
		Repeat: &suppressed_things_service.Repeat{Dynamic: &suppressed_things_service.Dynamic{Every: "week", Weekly: &suppressed_things_service.Weekly{Note: ptr("mondays")}}},
	})
	if diags.HasError() {
		t.Fatal(diags)
	}
	f := flat.Repeat.Dynamic.Frequency
	if f == nil || f.Daily != nil || f.Weekly == nil || f.Weekly.Note.ValueString() != "mondays" {
		t.Fatalf("the weekly arm read back as %+v, want frequency.weekly", f)
	}

	flat, diags = flatten(ctx, &suppressed_things_service.SuppressedThing{
		UniqueIdentifier: ptr("rule-1"), Name: "rule", ScheduleOperation: suppressed_things_service.THINGOPERATION_THING_OPERATION_MUTE,
		Repeat: &suppressed_things_service.Repeat{Dynamic: &suppressed_things_service.Dynamic{Every: "day", Daily: map[string]interface{}{}}},
	})
	if diags.HasError() {
		t.Fatal(diags)
	}
	if f := flat.Repeat.Dynamic.Frequency; f == nil || f.Daily == nil {
		t.Fatalf("daily: {} read back as %+v, want frequency.daily set", f)
	}
}

// The API reads the empty ids arm back as an empty list. readEmptyAs makes it null, as a filter
// without an arm has no ids; the state keeps the configured [] through keepPriorEmpty.
func TestEmptyWrappedIDsReadAsNull(t *testing.T) {
	flat, diags := flatten(context.Background(), &suppressed_things_service.SuppressedThing{
		UniqueIdentifier: ptr("rule-1"), Name: "rule", ScheduleOperation: suppressed_things_service.THINGOPERATION_THING_OPERATION_MUTE,
		Filter: &suppressed_things_service.Filter{Ids: &suppressed_things_service.Ids{Value: []string{}}},
	})
	if diags.HasError() {
		t.Fatal(diags)
	}
	if !flat.Filter.Ids.IsNull() {
		t.Fatalf("ids {value: []} read back as %v, want null", flat.Filter.Ids)
	}
}

// An unknown ids arm is no missing arm: expand does not send the default arm, which would select
// every alert.
func TestUnknownArmSendsNoDefaultArm(t *testing.T) {
	m := model(types.StringValue("rule-1"))
	m.Filter = &FilterModel{Ids: types.SetUnknown(types.StringType)}
	body, _ := expandCreate(context.Background(), m)
	if body != nil && body.SuppressedThing.Filter != nil && body.SuppressedThing.Filter.Ids != nil {
		t.Fatalf("an unknown ids arm sent %#v, want no default arm", body.SuppressedThing.Filter.Ids)
	}
}

// priority and hours keep the released Int64 type, while the API holds int32. A value outside the
// int32 range is an error, not a wrapped number.
func TestInt64AttributesSendInt32(t *testing.T) {
	ctx := context.Background()
	m := model(types.StringValue("rule-1"))
	m.Priority = types.Int64Value(7)
	m.Window.Hours = types.SetValueMust(types.Int64Type, []attr.Value{types.Int64Value(8), types.Int64Value(20)})
	body, diags := expandCreate(ctx, m)
	if diags.HasError() {
		t.Fatal(diags)
	}
	thing := body.SuppressedThing
	if thing.Priority == nil || *thing.Priority != 7 {
		t.Fatalf("priority 7 was sent as %v, want 7", thing.Priority)
	}
	if got := slices.Sorted(slices.Values(thing.Window.Hours)); !slices.Equal(got, []int32{8, 20}) {
		t.Fatalf("hours [8 20] were sent as %v", got)
	}

	m.Priority = types.Int64Value(1 << 31)
	if _, diags := expandCreate(ctx, m); !diags.HasError() {
		t.Fatal("priority 2^31 must be an error: the API holds int32")
	}
	m.Priority = types.Int64Value(7)
	m.Window.Hours = types.SetValueMust(types.Int64Type, []attr.Value{types.Int64Value(-(1 << 31) - 1)})
	if _, diags := expandCreate(ctx, m); !diags.HasError() {
		t.Fatal("an hour below the int32 range must be an error")
	}

	flat, diags := flatten(ctx, &suppressed_things_service.SuppressedThing{
		UniqueIdentifier: ptr("rule-1"), Name: "rule", ScheduleOperation: suppressed_things_service.THINGOPERATION_THING_OPERATION_MUTE,
		Priority: ptr(int32(3)), Window: &suppressed_things_service.Window{Timezone: "UTC", Hours: []int32{6}},
	})
	if diags.HasError() {
		t.Fatal(diags)
	}
	if !flat.Priority.Equal(types.Int64Value(3)) {
		t.Fatalf("priority 3 read back as %v", flat.Priority)
	}
	if want := types.SetValueMust(types.Int64Type, []attr.Value{types.Int64Value(6)}); !flat.Window.Hours.Equal(want) {
		t.Fatalf("hours [6] read back as %v", flat.Window.Hours)
	}
}
