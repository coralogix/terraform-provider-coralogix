package sharedthing

import (
	"context"
	"testing"

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

func thingValue(root tftypes.Type, id, name tftypes.Value) tftypes.Value {
	return tftypes.NewValue(root, map[string]tftypes.Value{"id": id, "name": name})
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
}

// Get does not require the id. A response without one fails the read: without the id,
// Terraform cannot read, update, or delete the resource.
func TestReadWithoutAnIDFails(t *testing.T) {
	ctx := context.Background()
	name := "a"
	shared_things_service.Stored = shared_things_service.SharedThing{Name: &name}
	r, schemaResp, root := thingSchema(t)
	state := thingValue(root, tftypes.NewValue(tftypes.String, "thing-1"), tftypes.NewValue(tftypes.String, "a"))
	resp := &frameworkresource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: state}}
	r.Read(ctx, frameworkresource.ReadRequest{State: tfsdk.State{Schema: schemaResp.Schema, Raw: state}}, resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("read of a response without an id: want an error")
	}
}
