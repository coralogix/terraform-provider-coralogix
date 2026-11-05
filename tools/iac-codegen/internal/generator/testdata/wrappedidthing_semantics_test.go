package wrappedidthing

import (
	"context"
	"errors"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"example.com/iac-test-sdk/go/openapi/gen/wrapped_things_service"
)

func thingSchema(t *testing.T) (*Resource, frameworkresource.SchemaResponse, tftypes.Type) {
	t.Helper()
	ctx := context.Background()
	r := &Resource{client: &wrapped_things_service.WrappedThingsServiceAPIService{}}
	var schemaResp frameworkresource.SchemaResponse
	r.Schema(ctx, frameworkresource.SchemaRequest{}, &schemaResp)
	return r, schemaResp, schemaResp.Schema.Type().TerraformType(ctx)
}

func thingValue(root tftypes.Type, id, name, quota tftypes.Value) tftypes.Value {
	return tftypes.NewValue(root, map[string]tftypes.Value{"id": id, "name": name, "quota": quota})
}

func stored(id int64, name string, quota float64) wrapped_things_service.GetThingResponse {
	return wrapped_things_service.GetThingResponse{ThingId: wrapped_things_service.ThingId{Id: &id}, Name: name, Quota: quota}
}

var (
	unknownID = tftypes.NewValue(tftypes.String, tftypes.UnknownValue)
	nullID    = tftypes.NewValue(tftypes.String, nil)
)

// Create reads the id in the wrapper of the Create response, reads the thing with Get under that
// id, and keeps the id as its decimal text. The quota that the API stores as a float32 reads
// back as the shortest decimal of that float32.
func TestCreateReadsTheWrappedID(t *testing.T) {
	ctx := context.Background()
	wrapped_things_service.Created, wrapped_things_service.GetIDs = nil, nil
	wrapped_things_service.Stored = stored(wrapped_things_service.CreatedID, "a", float64(float32(0.3)))
	r, schemaResp, root := thingSchema(t)
	quota := tftypes.NewValue(tftypes.Number, 0.3)
	plan := thingValue(root, unknownID, tftypes.NewValue(tftypes.String, "a"), quota)
	resp := &frameworkresource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.NewValue(root, nil)}}
	r.Create(ctx, frameworkresource.CreateRequest{
		Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: thingValue(root, nullID, tftypes.NewValue(tftypes.String, "a"), quota)},
		Plan:   tfsdk.Plan{Schema: schemaResp.Schema, Raw: plan},
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if len(wrapped_things_service.GetIDs) != 1 || wrapped_things_service.GetIDs[0] != wrapped_things_service.CreatedID {
		t.Fatalf("get ids = %v, want the id that Create returned", wrapped_things_service.GetIDs)
	}
	var id types.String
	var gotQuota types.Float64
	resp.Diagnostics.Append(resp.State.GetAttribute(ctx, path.Root("id"), &id)...)
	resp.Diagnostics.Append(resp.State.GetAttribute(ctx, path.Root("quota"), &gotQuota)...)
	if resp.Diagnostics.HasError() || id.ValueString() != "1002" || gotQuota.ValueFloat64() != 0.3 {
		t.Fatalf("state id = %v, quota = %v (%v), want \"1002\" and 0.3", id, gotQuota, resp.Diagnostics)
	}
}

// A Get that fails after Create made the thing keeps the decimal text of the id in the state.
func TestCreateKeepsTheWrappedIDWhenTheReadFails(t *testing.T) {
	ctx := context.Background()
	wrapped_things_service.GetErr = errors.New("read failed")
	defer func() { wrapped_things_service.GetErr = nil }()
	r, schemaResp, root := thingSchema(t)
	quota := tftypes.NewValue(tftypes.Number, nil)
	plan := thingValue(root, unknownID, tftypes.NewValue(tftypes.String, "a"), tftypes.NewValue(tftypes.Number, tftypes.UnknownValue))
	resp := &frameworkresource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.NewValue(root, nil)}}
	r.Create(ctx, frameworkresource.CreateRequest{
		Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: thingValue(root, nullID, tftypes.NewValue(tftypes.String, "a"), quota)},
		Plan:   tfsdk.Plan{Schema: schemaResp.Schema, Raw: plan},
	}, resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("create with a failing read: want an error")
	}
	var id types.String
	resp.State.GetAttribute(ctx, path.Root("id"), &id)
	if id.ValueString() != "1002" {
		t.Fatalf("state id = %q, want \"1002\"", id.ValueString())
	}
}

// Update parses the id of the state for the path, and reads the thing with Get after the empty
// Update response.
func TestUpdateSendsTheParsedID(t *testing.T) {
	ctx := context.Background()
	wrapped_things_service.Updated, wrapped_things_service.UpdatedIDs, wrapped_things_service.GetIDs = nil, nil, nil
	wrapped_things_service.Stored = stored(7, "new", 0.5)
	r, schemaResp, root := thingSchema(t)
	idValue := tftypes.NewValue(tftypes.String, "7")
	quota := tftypes.NewValue(tftypes.Number, 0.5)
	state := thingValue(root, idValue, tftypes.NewValue(tftypes.String, "old"), quota)
	plan := thingValue(root, idValue, tftypes.NewValue(tftypes.String, "new"), quota)
	resp := &frameworkresource.UpdateResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: state}}
	r.Update(ctx, frameworkresource.UpdateRequest{
		Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: thingValue(root, nullID, tftypes.NewValue(tftypes.String, "new"), quota)},
		Plan:   tfsdk.Plan{Schema: schemaResp.Schema, Raw: plan},
		State:  tfsdk.State{Schema: schemaResp.Schema, Raw: state},
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if len(wrapped_things_service.UpdatedIDs) != 1 || wrapped_things_service.UpdatedIDs[0] != 7 || len(wrapped_things_service.GetIDs) != 1 || wrapped_things_service.GetIDs[0] != 7 {
		t.Fatalf("update ids = %v, get ids = %v, want 7", wrapped_things_service.UpdatedIDs, wrapped_things_service.GetIDs)
	}
	var id types.String
	resp.State.GetAttribute(ctx, path.Root("id"), &id)
	if id.ValueString() != "7" {
		t.Fatalf("state id = %q, want \"7\"", id.ValueString())
	}
}

// An import id that is not an integer is an error at the import.
func TestImportRejectsANonIntegerID(t *testing.T) {
	ctx := context.Background()
	r, schemaResp, root := thingSchema(t)
	resp := &frameworkresource.ImportStateResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.NewValue(root, nil)}}
	r.ImportState(ctx, frameworkresource.ImportStateRequest{ID: "abc"}, resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("import of abc: want an error")
	}
	resp = &frameworkresource.ImportStateResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.NewValue(root, nil)}}
	r.ImportState(ctx, frameworkresource.ImportStateRequest{ID: "42"}, resp)
	var id types.String
	resp.Diagnostics.Append(resp.State.GetAttribute(ctx, path.Root("id"), &id)...)
	if resp.Diagnostics.HasError() || id.ValueString() != "42" {
		t.Fatalf("import of 42: id = %v (%v)", id, resp.Diagnostics)
	}
}
