package sharedthing

import (
	"context"
	"math/big"
	"slices"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"example.com/iac-test-sdk/go/openapi/gen/shared_things_service"
)

func thingSchema(t *testing.T) (*Resource, frameworkresource.SchemaResponse, tftypes.Type) {
	t.Helper()
	ctx := context.Background()
	r := &Resource{client: &shared_things_service.SharedThingsServiceAPIService{}}
	var schemaResp frameworkresource.SchemaResponse
	r.Schema(ctx, frameworkresource.SchemaRequest{}, &schemaResp)
	return r, schemaResp, schemaResp.Schema.Type().TerraformType(ctx)
}

// thingValue is the resource with the id and the name, a one_day window, and a ratio of 0.5 as
// the measure. Every other attribute is null.
func thingValue(root tftypes.Type, id, name tftypes.Value) tftypes.Value {
	object := root.(tftypes.Object)
	values := map[string]tftypes.Value{}
	for attr, attrType := range object.AttributeTypes {
		values[attr] = tftypes.NewValue(attrType, nil)
	}
	values["id"], values["name"] = id, name
	window := object.AttributeTypes["window"].(tftypes.Object)
	values["window"] = tftypes.NewValue(window, map[string]tftypes.Value{"frame": tftypes.NewValue(tftypes.String, "one_day")})
	measure := object.AttributeTypes["measure"].(tftypes.Object)
	ratio := measure.AttributeTypes["by_ratio"].(tftypes.Object)
	values["measure"] = tftypes.NewValue(measure, map[string]tftypes.Value{
		"by_count": tftypes.NewValue(measure.AttributeTypes["by_count"], nil),
		"by_ratio": tftypes.NewValue(ratio, map[string]tftypes.Value{"ratio": tftypes.NewValue(tftypes.Number, big.NewFloat(0.5))}),
	})
	return tftypes.NewValue(root, values)
}

// Create sends the resource as the body and reads the resource from the response, next to the
// ids of other objects. The server sets the id.
func TestCreateReadsTheResourceNextToOtherValues(t *testing.T) {
	ctx := context.Background()
	shared_things_service.Created = nil
	r, schemaResp, root := thingSchema(t)
	unknownID := tftypes.NewValue(tftypes.String, tftypes.UnknownValue)
	name := tftypes.NewValue(tftypes.String, "a")
	plan := thingValue(root, unknownID, name)
	config := thingValue(root, tftypes.NewValue(tftypes.String, nil), name)
	resp := &frameworkresource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.NewValue(root, nil)}}
	r.Create(ctx, frameworkresource.CreateRequest{
		Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: config},
		Plan:   tfsdk.Plan{Schema: schemaResp.Schema, Raw: plan},
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if len(shared_things_service.Created) != 1 || shared_things_service.Created[0].Id != nil {
		t.Fatalf("create bodies = %+v, want one without an id", shared_things_service.Created)
	}
	// The groups are Terraform objects. The API holds their fields in the resource itself.
	body := shared_things_service.Created[0]
	if body.Frame == nil || *body.Frame != shared_things_service.SHAREDFRAME_SHARED_FRAME_ONE_DAY {
		t.Fatalf("create body frame = %v, want SHARED_FRAME_ONE_DAY", body.Frame)
	}
	if body.ByCount != nil || body.ByRatio == nil || body.ByRatio.Ratio == nil || *body.ByRatio.Ratio != 0.5 {
		t.Fatalf("create body measure = %+v, %+v, want only a ratio of 0.5", body.ByCount, body.ByRatio)
	}
	var frame types.String
	resp.Diagnostics.Append(resp.State.GetAttribute(ctx, path.Root("window").AtName("frame"), &frame)...)
	var ratio types.Float32
	resp.Diagnostics.Append(resp.State.GetAttribute(ctx, path.Root("measure").AtName("by_ratio").AtName("ratio"), &ratio)...)
	if resp.Diagnostics.HasError() || frame.ValueString() != "one_day" || ratio.ValueFloat32() != 0.5 {
		t.Fatalf("state window.frame = %v, measure.by_ratio.ratio = %v (%v), want one_day and 0.5", frame, ratio, resp.Diagnostics)
	}
	var id types.String
	resp.Diagnostics.Append(resp.State.GetAttribute(ctx, path.Root("id"), &id)...)
	if resp.Diagnostics.HasError() || id.ValueString() != shared_things_service.CreatedID {
		t.Fatalf("state id = %v (%v), want %s", id, resp.Diagnostics, shared_things_service.CreatedID)
	}
}

// The server sets the id, so no Update field sends it. Update copies the state id into the body,
// which is the resource itself.
func TestUpdateSendsTheStateIDInTheBody(t *testing.T) {
	ctx := context.Background()
	shared_things_service.Replaced = nil
	r, schemaResp, root := thingSchema(t)
	id := tftypes.NewValue(tftypes.String, "thing-1")
	state := thingValue(root, id, tftypes.NewValue(tftypes.String, "old"))
	plan := thingValue(root, id, tftypes.NewValue(tftypes.String, "new"))
	config := thingValue(root, tftypes.NewValue(tftypes.String, nil), tftypes.NewValue(tftypes.String, "new"))
	resp := &frameworkresource.UpdateResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: plan}}
	r.Update(ctx, frameworkresource.UpdateRequest{
		Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: config},
		Plan:   tfsdk.Plan{Schema: schemaResp.Schema, Raw: plan},
		State:  tfsdk.State{Schema: schemaResp.Schema, Raw: state},
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if len(shared_things_service.Replaced) != 1 {
		t.Fatalf("replace calls = %d, want 1", len(shared_things_service.Replaced))
	}
	body := shared_things_service.Replaced[0]
	if body.Id == nil || *body.Id != "thing-1" || body.Name == nil || *body.Name != "new" {
		t.Fatalf("replace body = %+v, want id thing-1 and name new", body)
	}
	// The API keeps a stored note that the request omits, so a null note is sent as "", and the
	// stored "" reads back as null.
	if body.Note == nil || *body.Note != "" {
		t.Fatalf("replace body note = %v, want \"\"", body.Note)
	}
	var note types.String
	resp.Diagnostics.Append(resp.State.GetAttribute(ctx, path.Root("note"), &note)...)
	if resp.Diagnostics.HasError() || !note.IsNull() {
		t.Fatalf("state note = %v, %v, want null", note, resp.Diagnostics)
	}
}

// Get does not require the id. A response without one fails the read: without the id,
// Terraform cannot read, update, or delete the resource.
func TestReadWithoutAnIDFails(t *testing.T) {
	ctx := context.Background()
	name, frame := "a", shared_things_service.SHAREDFRAME_SHARED_FRAME_ONE_DAY
	shared_things_service.Stored = shared_things_service.SharedThing{Name: &name, Frame: &frame}
	r, schemaResp, root := thingSchema(t)
	state := thingValue(root, tftypes.NewValue(tftypes.String, "thing-1"), tftypes.NewValue(tftypes.String, "a"))
	resp := &frameworkresource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: state}}
	r.Read(ctx, frameworkresource.ReadRequest{State: tfsdk.State{Schema: schemaResp.Schema, Raw: state}}, resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("read of a response without an id: want an error")
	}
}

// A response with none, or more than one, of the arms of a group that needs one fails the read:
// the state would break the ExactlyOneOf of the arms.
func TestReadWithoutAnArmFails(t *testing.T) {
	ctx := context.Background()
	id, name, frame := "thing-1", "a", shared_things_service.SHAREDFRAME_SHARED_FRAME_ONE_DAY
	shared_things_service.Stored = shared_things_service.SharedThing{Id: &id, Name: &name, Frame: &frame}
	r, schemaResp, root := thingSchema(t)
	state := thingValue(root, tftypes.NewValue(tftypes.String, "thing-1"), tftypes.NewValue(tftypes.String, "a"))
	resp := &frameworkresource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: state}}
	r.Read(ctx, frameworkresource.ReadRequest{State: tfsdk.State{Schema: schemaResp.Schema, Raw: state}}, resp)
	found := false
	for _, d := range resp.Diagnostics.Errors() {
		found = found || d.Summary() == "Invalid response from server"
	}
	if !found {
		t.Fatalf("read of a response without an arm: want Invalid response from server, got %v", resp.Diagnostics)
	}
	ratio := float32(0.5)
	shared_things_service.Stored.ByRatio = &shared_things_service.SharedRatio{Ratio: &ratio}
	resp = &frameworkresource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: state}}
	r.Read(ctx, frameworkresource.ReadRequest{State: tfsdk.State{Schema: schemaResp.Schema, Raw: state}}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("read of a response with an arm: %v", resp.Diagnostics)
	}
	limit := int32(3)
	shared_things_service.Stored.ByCount = &shared_things_service.SharedCount{Limit: &limit}
	resp = &frameworkresource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: state}}
	r.Read(ctx, frameworkresource.ReadRequest{State: tfsdk.State{Schema: schemaResp.Schema, Raw: state}}, resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("read of a response with two arms: want an error")
	}
}

// The data source shows the note that the API stores, also "", which the resource reads as null.
// It runs the BeforeFlatten hook of the resource, and takes the id or the name.
func TestDataSourceShowsTheStoredNote(t *testing.T) {
	ctx := context.Background()
	id, name, frame, note, ratio := "thing-1", "a", shared_things_service.SHAREDFRAME_SHARED_FRAME_ONE_DAY, "", float32(0.5)
	shared_things_service.Stored = shared_things_service.SharedThing{
		Id: &id, Name: &name, Frame: &frame, Note: &note, ByRatio: &shared_things_service.SharedRatio{Ratio: &ratio},
	}
	flattened := false
	d := &DataSource{resource: Resource{
		client: &shared_things_service.SharedThingsServiceAPIService{},
		hooks: Hooks{BeforeFlatten: func(context.Context, *shared_things_service.SharedThing) diag.Diagnostics {
			flattened = true
			return nil
		}},
	}}
	var schemaResp datasource.SchemaResponse
	d.Schema(ctx, datasource.SchemaRequest{}, &schemaResp)
	if a := schemaResp.Schema.Attributes["id"]; !a.IsOptional() || a.IsRequired() {
		t.Fatalf("data source id = %#v, want optional", a)
	}
	if a := schemaResp.Schema.Attributes["name"]; !a.IsOptional() || !a.IsComputed() {
		t.Fatalf("data source name = %#v, want optional and computed", a)
	}
	if a := schemaResp.Schema.Attributes["note"]; !a.IsComputed() || a.IsOptional() || a.IsRequired() {
		t.Fatalf("data source note = %#v, want computed-only", a)
	}
	root := schemaResp.Schema.Type().TerraformType(ctx).(tftypes.Object)
	values := map[string]tftypes.Value{}
	for attr, attrType := range root.AttributeTypes {
		values[attr] = tftypes.NewValue(attrType, nil)
	}
	values["id"] = tftypes.NewValue(tftypes.String, id)
	config := tftypes.NewValue(root, values)
	resp := &datasource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: config}}
	d.Read(ctx, datasource.ReadRequest{Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: config}}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if !flattened {
		t.Fatal("the data source did not run BeforeFlatten")
	}
	var got types.String
	resp.Diagnostics.Append(resp.State.GetAttribute(ctx, path.Root("note"), &got)...)
	if resp.Diagnostics.HasError() || got.IsNull() || got.ValueString() != "" {
		t.Fatalf("data source note = %v (%v), want \"\"", got, resp.Diagnostics)
	}
}

// With duplicateNames: first, the data source takes the first summary of the List with the name,
// in the order of the API, and reads that id with Get. A name that no summary has is an error.
func TestDataSourceTakesTheFirstNameMatch(t *testing.T) {
	ctx := context.Background()
	id, name, frame, ratio := "thing-2", "a", shared_things_service.SHAREDFRAME_SHARED_FRAME_ONE_DAY, float32(0.5)
	shared_things_service.Stored = shared_things_service.SharedThing{
		Id: &id, Name: &name, Frame: &frame, ByRatio: &shared_things_service.SharedRatio{Ratio: &ratio},
	}
	summary := func(id, name string) shared_things_service.SharedThingSummary {
		return shared_things_service.SharedThingSummary{Id: &id, Name: &name}
	}
	shared_things_service.Summaries = []shared_things_service.SharedThingSummary{
		summary("thing-1", "b"), summary("thing-2", "a"), summary("thing-3", "a"),
	}
	t.Cleanup(func() { shared_things_service.Summaries, shared_things_service.GotIDs = nil, nil })
	d := &DataSource{resource: Resource{client: &shared_things_service.SharedThingsServiceAPIService{}}}
	var schemaResp datasource.SchemaResponse
	d.Schema(ctx, datasource.SchemaRequest{}, &schemaResp)
	read := func(name string) *datasource.ReadResponse {
		root := schemaResp.Schema.Type().TerraformType(ctx).(tftypes.Object)
		values := map[string]tftypes.Value{}
		for attr, attrType := range root.AttributeTypes {
			values[attr] = tftypes.NewValue(attrType, nil)
		}
		values["name"] = tftypes.NewValue(tftypes.String, name)
		config := tftypes.NewValue(root, values)
		shared_things_service.GotIDs = nil
		resp := &datasource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: config}}
		d.Read(ctx, datasource.ReadRequest{Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: config}}, resp)
		return resp
	}

	resp := read("a")
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if !slices.Equal(shared_things_service.GotIDs, []string{"thing-2"}) {
		t.Fatalf("Get ids = %v, want [thing-2]", shared_things_service.GotIDs)
	}
	var got types.String
	resp.Diagnostics.Append(resp.State.GetAttribute(ctx, path.Root("id"), &got)...)
	if got.ValueString() != "thing-2" {
		t.Fatalf("data source id = %v (%v), want thing-2", got, resp.Diagnostics)
	}

	resp = read("missing")
	if !resp.Diagnostics.HasError() || resp.Diagnostics[0].Summary() != `coralogix_shared_thing_v2 with name "missing" not found` || len(shared_things_service.GotIDs) != 0 {
		t.Fatalf("missing name: Get ids = %v, diagnostics = %v", shared_things_service.GotIDs, resp.Diagnostics)
	}
}
