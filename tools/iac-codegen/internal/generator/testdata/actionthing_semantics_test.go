package actionthing

import (
	"context"
	"testing"

	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"example.com/iac-test-sdk/go/openapi/gen/action_things_service"
)

func thingSchema(t *testing.T) (*Resource, frameworkresource.SchemaResponse, tftypes.Object) {
	t.Helper()
	r := &Resource{client: &action_things_service.ActionThingsServiceAPIService{}}
	var schemaResp frameworkresource.SchemaResponse
	r.Schema(context.Background(), frameworkresource.SchemaRequest{}, &schemaResp)
	return r, schemaResp, schemaResp.Schema.Type().TerraformType(context.Background()).(tftypes.Object)
}

// thingValue holds the attributes that the test sets, and null for the others.
func thingValue(root tftypes.Object, set map[string]tftypes.Value) tftypes.Value {
	values := map[string]tftypes.Value{}
	for attr, attrType := range root.AttributeTypes {
		values[attr] = tftypes.NewValue(attrType, nil)
	}
	for attr, v := range set {
		values[attr] = v
	}
	return tftypes.NewValue(root, values)
}

func str(s string) tftypes.Value { return tftypes.NewValue(tftypes.String, s) }

func create(t *testing.T, hidden tftypes.Value) {
	t.Helper()
	action_things_service.Created, action_things_service.Replaced = nil, nil
	r, schemaResp, root := thingSchema(t)
	set := func(id tftypes.Value) map[string]tftypes.Value {
		return map[string]tftypes.Value{"id": id, "name": str("a"), "hidden": hidden}
	}
	resp := &frameworkresource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.NewValue(root, nil)}}
	r.Create(context.Background(), frameworkresource.CreateRequest{
		Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: thingValue(root, set(tftypes.NewValue(tftypes.String, nil)))},
		Plan:   tfsdk.Plan{Schema: schemaResp.Schema, Raw: thingValue(root, set(tftypes.NewValue(tftypes.String, tftypes.UnknownValue)))},
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
}

// A hidden value other than the server default is set by a Replace after Create, which sends the
// id that Create returned. The Create body has no such field.
func TestCreateFollowsWithAReplaceForTheUpdateOnlyField(t *testing.T) {
	create(t, tftypes.NewValue(tftypes.Bool, true))
	if len(action_things_service.Created) != 1 || action_things_service.Created[0].Name == nil {
		t.Fatalf("create bodies = %+v, want one flat body", action_things_service.Created)
	}
	if len(action_things_service.Replaced) != 1 {
		t.Fatalf("replace bodies = %+v, want one", action_things_service.Replaced)
	}
	thing := action_things_service.Replaced[0].Thing
	if thing == nil || thing.Id == nil || *thing.Id != "thing-1" || thing.Hidden == nil || !*thing.Hidden {
		t.Fatalf("replace thing = %+v, want the id of the new thing and hidden true", thing)
	}
}

// The server default needs no second call.
func TestCreateWithTheServerDefaultNeedsNoReplace(t *testing.T) {
	create(t, tftypes.NewValue(tftypes.Bool, false))
	if len(action_things_service.Created) != 1 || len(action_things_service.Replaced) != 0 {
		t.Fatalf("created = %+v, replaced = %+v, want one create and no replace", action_things_service.Created, action_things_service.Replaced)
	}
}

// A removed note is sent empty, because the API keeps what a request omits.
func TestUpdateSendsEmptyForRemovedNote(t *testing.T) {
	action_things_service.Replaced = nil
	id, note := "thing-1", "old"
	action_things_service.Stored = action_things_service.ActionThing{Id: &id, Name: &note, Note: &note}
	r, schemaResp, root := thingSchema(t)
	state := thingValue(root, map[string]tftypes.Value{
		"id": str(id), "name": str("a"), "hidden": tftypes.NewValue(tftypes.Bool, false), "note": str("old"),
	})
	plan := thingValue(root, map[string]tftypes.Value{"id": str(id), "name": str("a"), "hidden": tftypes.NewValue(tftypes.Bool, false)})
	resp := &frameworkresource.UpdateResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: plan}}
	r.Update(context.Background(), frameworkresource.UpdateRequest{
		Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: plan},
		Plan:   tfsdk.Plan{Schema: schemaResp.Schema, Raw: plan},
		State:  tfsdk.State{Schema: schemaResp.Schema, Raw: state},
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if len(action_things_service.Replaced) != 1 {
		t.Fatalf("replace bodies = %+v, want one", action_things_service.Replaced)
	}
	thing := action_things_service.Replaced[0].Thing
	if thing.Note == nil || *thing.Note != "" {
		t.Fatalf("replace thing = %+v, want note \"\"", thing)
	}
}
