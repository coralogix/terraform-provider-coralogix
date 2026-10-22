package folderthing

import (
	"context"
	"errors"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"example.com/iac-test-sdk/go/openapi/gen/folder_things_service"
)

func thingSchema(t *testing.T) (*Resource, frameworkresource.SchemaResponse, tftypes.Type) {
	t.Helper()
	ctx := context.Background()
	r := &Resource{client: &folder_things_service.FolderThingsServiceAPIService{}}
	var schemaResp frameworkresource.SchemaResponse
	r.Schema(ctx, frameworkresource.SchemaRequest{}, &schemaResp)
	return r, schemaResp, schemaResp.Schema.Type().TerraformType(ctx)
}

func thingValue(root tftypes.Type, id, name tftypes.Value) tftypes.Value {
	values := map[string]tftypes.Value{}
	for attr, attrType := range root.(tftypes.Object).AttributeTypes {
		values[attr] = tftypes.NewValue(attrType, nil)
	}
	values["id"], values["name"] = id, name
	return tftypes.NewValue(root, values)
}

// Create sends the thing in the wrapper and no id, because the server sets it. The response holds
// only the id, so Create reads the thing with Get, and the state is what Get returns.
func TestCreateReadsTheThingWithGet(t *testing.T) {
	ctx := context.Background()
	folder_things_service.Created, folder_things_service.GetIDs = nil, nil
	id, name := folder_things_service.CreatedID, "stored"
	folder_things_service.Stored = folder_things_service.FolderThing{Id: &id, Name: &name}
	r, schemaResp, root := thingSchema(t)
	plan := thingValue(root, tftypes.NewValue(tftypes.String, tftypes.UnknownValue), tftypes.NewValue(tftypes.String, "a"))
	config := thingValue(root, tftypes.NewValue(tftypes.String, nil), tftypes.NewValue(tftypes.String, "a"))
	resp := &frameworkresource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.NewValue(root, nil)}}
	r.Create(ctx, frameworkresource.CreateRequest{
		Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: config},
		Plan:   tfsdk.Plan{Schema: schemaResp.Schema, Raw: plan},
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if len(folder_things_service.Created) != 1 || folder_things_service.Created[0].Thing == nil ||
		folder_things_service.Created[0].Thing.Id != nil || folder_things_service.Created[0].RequestId != nil {
		t.Fatalf("create bodies = %+v, want one thing without an id and no request id", folder_things_service.Created)
	}
	if len(folder_things_service.GetIDs) != 1 || folder_things_service.GetIDs[0] != folder_things_service.CreatedID {
		t.Fatalf("get ids = %v, want the id that Create returned", folder_things_service.GetIDs)
	}
	var gotID, gotName types.String
	resp.Diagnostics.Append(resp.State.GetAttribute(ctx, path.Root("id"), &gotID)...)
	resp.Diagnostics.Append(resp.State.GetAttribute(ctx, path.Root("name"), &gotName)...)
	if resp.Diagnostics.HasError() || gotID.ValueString() != id || gotName.ValueString() != "stored" {
		t.Fatalf("state id = %v, name = %v (%v), want %s and stored", gotID, gotName, resp.Diagnostics, id)
	}
}

// Replace has the id in the body and returns no fields, so Update reads the thing with Get.
func TestUpdateReadsTheThingWithGet(t *testing.T) {
	ctx := context.Background()
	folder_things_service.Replaced, folder_things_service.GetIDs = nil, nil
	id, name := "folder-1", "new"
	folder_things_service.Stored = folder_things_service.FolderThing{Id: &id, Name: &name}
	r, schemaResp, root := thingSchema(t)
	idValue := tftypes.NewValue(tftypes.String, id)
	state := thingValue(root, idValue, tftypes.NewValue(tftypes.String, "old"))
	plan := thingValue(root, idValue, tftypes.NewValue(tftypes.String, "new"))
	resp := &frameworkresource.UpdateResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: plan}}
	r.Update(ctx, frameworkresource.UpdateRequest{
		Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: thingValue(root, tftypes.NewValue(tftypes.String, nil), tftypes.NewValue(tftypes.String, "new"))},
		Plan:   tfsdk.Plan{Schema: schemaResp.Schema, Raw: plan},
		State:  tfsdk.State{Schema: schemaResp.Schema, Raw: state},
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if len(folder_things_service.Replaced) != 1 || folder_things_service.Replaced[0].Thing == nil ||
		folder_things_service.Replaced[0].Thing.Id == nil || *folder_things_service.Replaced[0].Thing.Id != id {
		t.Fatalf("replace bodies = %+v, want one thing with the state id", folder_things_service.Replaced)
	}
	if len(folder_things_service.GetIDs) != 1 || folder_things_service.GetIDs[0] != id {
		t.Fatalf("get ids = %v, want the id of the thing", folder_things_service.GetIDs)
	}
}

// A Get that fails after Create made the thing keeps the id in the state, so Terraform marks the
// resource as tainted instead of leaving it behind.
func TestCreateKeepsTheIDWhenTheReadFails(t *testing.T) {
	ctx := context.Background()
	folder_things_service.GetErr = errors.New("read failed")
	defer func() { folder_things_service.GetErr = nil }()
	r, schemaResp, root := thingSchema(t)
	plan := thingValue(root, tftypes.NewValue(tftypes.String, tftypes.UnknownValue), tftypes.NewValue(tftypes.String, "a"))
	config := thingValue(root, tftypes.NewValue(tftypes.String, nil), tftypes.NewValue(tftypes.String, "a"))
	resp := &frameworkresource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.NewValue(root, nil)}}
	r.Create(ctx, frameworkresource.CreateRequest{
		Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: config},
		Plan:   tfsdk.Plan{Schema: schemaResp.Schema, Raw: plan},
	}, resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("create with a failing read: want an error")
	}
	var id types.String
	resp.State.GetAttribute(ctx, path.Root("id"), &id)
	if id.ValueString() != folder_things_service.CreatedID {
		t.Fatalf("state id = %q, want %q", id.ValueString(), folder_things_service.CreatedID)
	}
}

func deleteRequest(t *testing.T, r *Resource, schemaResp frameworkresource.SchemaResponse, root tftypes.Type) *frameworkresource.DeleteResponse {
	t.Helper()
	state := thingValue(root, tftypes.NewValue(tftypes.String, "folder-1"), tftypes.NewValue(tftypes.String, "a"))
	resp := &frameworkresource.DeleteResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: state}}
	r.Delete(context.Background(), frameworkresource.DeleteRequest{State: tfsdk.State{Schema: schemaResp.Schema, Raw: state}}, resp)
	return resp
}

// An API that answers a delete of a missing thing with another status than 404 is a failure,
// unless the DeleteGone hook says that the thing is gone. The hook gets the id.
func TestDeleteGoneHook(t *testing.T) {
	folder_things_service.DeleteErr = errors.New("permission denied")
	defer func() { folder_things_service.DeleteErr = nil }()
	r, schemaResp, root := thingSchema(t)
	if resp := deleteRequest(t, r, schemaResp, root); !resp.Diagnostics.HasError() {
		t.Fatal("delete that fails with no hook: want an error")
	}
	var gotID string
	r.hooks.DeleteGone = func(_ context.Context, _ *folder_things_service.FolderThingsServiceAPIService, id string, _ error) bool {
		gotID = id
		return true
	}
	if resp := deleteRequest(t, r, schemaResp, root); resp.Diagnostics.HasError() || gotID != "folder-1" {
		t.Fatalf("delete with a hook that says gone: diagnostics = %v, id = %q, want none and folder-1", resp.Diagnostics, gotID)
	}
	r.hooks.DeleteGone = func(context.Context, *folder_things_service.FolderThingsServiceAPIService, string, error) bool {
		return false
	}
	if resp := deleteRequest(t, r, schemaResp, root); !resp.Diagnostics.HasError() {
		t.Fatal("delete with a hook that says not gone: want an error")
	}
}

// An API that answers an update of a missing thing with another status than 404 is a failure,
// unless the UpdateGone hook says that the thing is gone: Update then says that the resource no
// longer exists, as for a 404.
func TestUpdateGoneHook(t *testing.T) {
	ctx := context.Background()
	folder_things_service.ReplaceErr = errors.New("does not exist")
	defer func() { folder_things_service.ReplaceErr = nil }()
	r, schemaResp, root := thingSchema(t)
	idValue := tftypes.NewValue(tftypes.String, "folder-1")
	update := func() *frameworkresource.UpdateResponse {
		state := thingValue(root, idValue, tftypes.NewValue(tftypes.String, "old"))
		plan := thingValue(root, idValue, tftypes.NewValue(tftypes.String, "new"))
		resp := &frameworkresource.UpdateResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: state}}
		r.Update(ctx, frameworkresource.UpdateRequest{
			Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: thingValue(root, tftypes.NewValue(tftypes.String, nil), tftypes.NewValue(tftypes.String, "new"))},
			Plan:   tfsdk.Plan{Schema: schemaResp.Schema, Raw: plan},
			State:  tfsdk.State{Schema: schemaResp.Schema, Raw: state},
		}, resp)
		return resp
	}
	summary := func(resp *frameworkresource.UpdateResponse) string {
		if !resp.Diagnostics.HasError() {
			return ""
		}
		return resp.Diagnostics.Errors()[0].Summary()
	}
	if got := summary(update()); got != "Unable to update "+TypeName {
		t.Fatalf("update that fails with no hook: summary = %q, want Unable to update", got)
	}
	r.hooks.UpdateGone = func(_ context.Context, _ *folder_things_service.FolderThingsServiceAPIService, id string, _ error) bool {
		return id == "folder-1"
	}
	if got := summary(update()); got != "Resource no longer exists" {
		t.Fatalf("update with a hook that says gone: summary = %q, want Resource no longer exists", got)
	}
}

// Create returns the id, and Get leaves it out: the state still has the id that Create returned.
func TestCreateKeepsTheReturnedIDWhenGetOmitsIt(t *testing.T) {
	ctx := context.Background()
	name := "stored"
	folder_things_service.Stored = folder_things_service.FolderThing{Name: &name}
	r, schemaResp, root := thingSchema(t)
	plan := thingValue(root, tftypes.NewValue(tftypes.String, tftypes.UnknownValue), tftypes.NewValue(tftypes.String, "a"))
	config := thingValue(root, tftypes.NewValue(tftypes.String, nil), tftypes.NewValue(tftypes.String, "a"))
	resp := &frameworkresource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.NewValue(root, nil)}}
	r.Create(ctx, frameworkresource.CreateRequest{
		Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: config},
		Plan:   tfsdk.Plan{Schema: schemaResp.Schema, Raw: plan},
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	var id types.String
	resp.State.GetAttribute(ctx, path.Root("id"), &id)
	if id.ValueString() != folder_things_service.CreatedID {
		t.Fatalf("state id = %q, want %q", id.ValueString(), folder_things_service.CreatedID)
	}
}

// Update reads the thing with Get, and Get leaves the id out: the state keeps the id of the state.
func TestUpdateKeepsTheIDWhenGetOmitsIt(t *testing.T) {
	ctx := context.Background()
	name := "new"
	folder_things_service.Stored = folder_things_service.FolderThing{Name: &name}
	r, schemaResp, root := thingSchema(t)
	idValue := tftypes.NewValue(tftypes.String, "folder-1")
	state := thingValue(root, idValue, tftypes.NewValue(tftypes.String, "old"))
	plan := thingValue(root, idValue, tftypes.NewValue(tftypes.String, "new"))
	resp := &frameworkresource.UpdateResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: state}}
	r.Update(ctx, frameworkresource.UpdateRequest{
		Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: thingValue(root, tftypes.NewValue(tftypes.String, nil), tftypes.NewValue(tftypes.String, "new"))},
		Plan:   tfsdk.Plan{Schema: schemaResp.Schema, Raw: plan},
		State:  tfsdk.State{Schema: schemaResp.Schema, Raw: state},
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	var id types.String
	resp.State.GetAttribute(ctx, path.Root("id"), &id)
	if id.ValueString() != "folder-1" {
		t.Fatalf("state id = %q, want folder-1", id.ValueString())
	}
}
