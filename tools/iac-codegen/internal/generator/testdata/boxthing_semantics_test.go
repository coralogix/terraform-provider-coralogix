package boxthing

import (
	"context"
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"example.com/iac-test-sdk/go/openapi/gen/box_things_service"
)

// routeError is the 404 of a route that does not exist: the List has no resource that can be
// gone, so it is an error and never a removal.
type routeError struct{}

func (routeError) Error() string   { return "Not Found: Not Found" }
func (routeError) StatusCode() int { return http.StatusNotFound }
func (routeError) Body() []byte    { return []byte("Not Found: Not Found") }

func reset() {
	box_things_service.Listed, box_things_service.ListErr = 0, nil
	box_things_service.Stored = []box_things_service.BoxThing{{Id: "box-1", Name: "one"}, {Id: "box-2", Name: "two"}}
	box_things_service.Created, box_things_service.Updated, box_things_service.UpdatedIDs = nil, nil, nil
}

func boxSchema(t *testing.T) (*Resource, frameworkresource.SchemaResponse, tftypes.Type) {
	t.Helper()
	ctx := context.Background()
	r := &Resource{client: &box_things_service.BoxThingsServiceAPIService{}}
	var schemaResp frameworkresource.SchemaResponse
	r.Schema(ctx, frameworkresource.SchemaRequest{}, &schemaResp)
	return r, schemaResp, schemaResp.Schema.Type().TerraformType(ctx)
}

func boxValue(root tftypes.Type, id, name string) tftypes.Value {
	values := map[string]tftypes.Value{}
	for attr, attrType := range root.(tftypes.Object).AttributeTypes {
		values[attr] = tftypes.NewValue(attrType, nil)
	}
	values["id"] = tftypes.NewValue(tftypes.String, id)
	values["name"] = tftypes.NewValue(tftypes.String, name)
	values["description"] = tftypes.NewValue(tftypes.String, "")
	return tftypes.NewValue(root, values)
}

func readBox(t *testing.T, id string) (*frameworkresource.ReadResponse, frameworkresource.SchemaResponse) {
	t.Helper()
	r, schemaResp, root := boxSchema(t)
	state := boxValue(root, id, "old")
	resp := &frameworkresource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: state}}
	r.Read(context.Background(), frameworkresource.ReadRequest{State: tfsdk.State{Schema: schemaResp.Schema, Raw: state}}, resp)
	return resp, schemaResp
}

// Read lists the box things once and takes the one with the id of the state.
func TestReadSelectsTheItemByItsID(t *testing.T) {
	reset()
	resp, _ := readBox(t, "box-2")
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	var name string
	state := resp.State
	if diags := state.GetAttribute(context.Background(), path.Root("name"), &name); diags.HasError() {
		t.Fatal(diags)
	}
	if name != "two" || box_things_service.Listed != 1 {
		t.Fatalf("name = %q after %d calls of the List, want two after one", name, box_things_service.Listed)
	}
}

// No item with the id removes the resource from the state, with a warning.
func TestReadRemovesAResourceThatTheListLacks(t *testing.T) {
	reset()
	resp, _ := readBox(t, "box-9")
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if !resp.State.Raw.IsNull() || len(resp.Diagnostics.Warnings()) != 1 {
		t.Fatalf("state null = %t, warnings = %d, want a removed resource and one warning", resp.State.Raw.IsNull(), len(resp.Diagnostics.Warnings()))
	}
}

// A 404 of the List is a missing route. It is an error, and the state stays.
func TestReadKeepsTheStateOnA404OfTheList(t *testing.T) {
	reset()
	box_things_service.ListErr = routeError{}
	resp, _ := readBox(t, "box-2")
	if !resp.Diagnostics.HasError() || resp.State.Raw.IsNull() {
		t.Fatalf("diagnostics = %v, state null = %t, want an error and the state kept", resp.Diagnostics, resp.State.Raw.IsNull())
	}
}

// Update puts the id in its path, and the mask names the changed field.
func TestUpdateTakesTheIDInThePath(t *testing.T) {
	reset()
	r, schemaResp, root := boxSchema(t)
	state := boxValue(root, "box-2", "two")
	plan := boxValue(root, "box-2", "renamed")
	resp := &frameworkresource.UpdateResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: state}}
	r.Update(context.Background(), frameworkresource.UpdateRequest{
		Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: plan},
		Plan:   tfsdk.Plan{Schema: schemaResp.Schema, Raw: plan},
		State:  tfsdk.State{Schema: schemaResp.Schema, Raw: state},
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if len(box_things_service.UpdatedIDs) != 1 || box_things_service.UpdatedIDs[0] != "box-2" {
		t.Fatalf("update ids = %v, want [box-2]", box_things_service.UpdatedIDs)
	}
	if mask := box_things_service.Updated[0].UpdateMask; mask == nil || *mask != "name" {
		t.Fatalf("mask = %v, want name", mask)
	}
}
