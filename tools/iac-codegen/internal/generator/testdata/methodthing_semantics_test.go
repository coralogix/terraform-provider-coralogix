package methodthing

import (
	"context"
	"slices"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"example.com/iac-test-sdk/go/openapi/gen/method_things_service"
)

func methodSchema(t *testing.T) (*Resource, frameworkresource.SchemaResponse, tftypes.Type) {
	t.Helper()
	ctx := context.Background()
	r := &Resource{client: &method_things_service.MethodThingsServiceAPIService{}}
	var schemaResp frameworkresource.SchemaResponse
	r.Schema(ctx, frameworkresource.SchemaRequest{}, &schemaResp)
	return r, schemaResp, schemaResp.Schema.Type().TerraformType(ctx)
}

func stringSet(values ...string) tftypes.Value {
	items := make([]tftypes.Value, len(values))
	for i, v := range values {
		items[i] = tftypes.NewValue(tftypes.String, v)
	}
	return tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, items)
}

func thingValue(root tftypes.Type, id tftypes.Value, name string, permissions ...string) tftypes.Value {
	values := map[string]tftypes.Value{}
	for attrName, attrType := range root.(tftypes.Object).AttributeTypes {
		values[attrName] = tftypes.NewValue(attrType, nil)
	}
	values["id"], values["name"] = id, tftypes.NewValue(tftypes.String, name)
	values["description"] = tftypes.NewValue(tftypes.String, "d")
	values["parent_name"] = tftypes.NewValue(tftypes.String, "Standard")
	values["permissions"] = stringSet(permissions...)
	return tftypes.NewValue(root, values)
}

func permissionsOf(t *testing.T, ctx context.Context, state tfsdk.State) []string {
	t.Helper()
	var set types.Set
	if diags := state.GetAttribute(ctx, path.Root("permissions"), &set); diags.HasError() {
		t.Fatal(diags)
	}
	var out []string
	for _, e := range set.Elements() {
		out = append(out, e.(attr.Value).(types.String).ValueString())
	}
	slices.Sort(out)
	return out
}

// Create sends the parent by name and the permissions in their own list, reads the integer id of
// its response, and reads the thing with Get. The state keeps the case of the configuration where
// Get returns the same permission in another case.
func TestCreateReadsTheIntegerIDAndKeepsTheConfiguredCase(t *testing.T) {
	ctx := context.Background()
	method_things_service.Created = nil
	method_things_service.Stored = method_things_service.MethodThing{
		ThingId: method_things_service.CreatedID, Name: "t", Description: "d", ParentId: 7, ParentName: "Standard",
		Permissions: []string{"A:READ", "b:write"},
	}
	r, schemaResp, root := methodSchema(t)
	plan := thingValue(root, tftypes.NewValue(tftypes.String, tftypes.UnknownValue), "t", "a:read", "b:write")
	resp := &frameworkresource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.NewValue(root, nil)}}
	r.Create(ctx, frameworkresource.CreateRequest{
		Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: thingValue(root, tftypes.NewValue(tftypes.String, nil), "t", "a:read", "b:write")},
		Plan:   tfsdk.Plan{Schema: schemaResp.Schema, Raw: plan},
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if len(method_things_service.Created) != 1 {
		t.Fatalf("create bodies = %+v, want one", method_things_service.Created)
	}
	body := method_things_service.Created[0]
	if body.ParentName == nil || *body.ParentName != "Standard" || body.ParentId != nil {
		t.Errorf("create parent = name %v, id %v, want the name only", body.ParentName, body.ParentId)
	}
	if !slices.Equal(body.Permissions, []string{"a:read", "b:write"}) && !slices.Equal(body.Permissions, []string{"b:write", "a:read"}) {
		t.Errorf("create permissions = %v", body.Permissions)
	}
	var id types.String
	if diags := resp.State.GetAttribute(ctx, path.Root("id"), &id); diags.HasError() || id.ValueString() != "41" {
		t.Errorf("state id = %v (%v), want the text of the integer id 41", id, diags)
	}
	if got := permissionsOf(t, ctx, resp.State); !slices.Equal(got, []string{"a:read", "b:write"}) {
		t.Errorf("state permissions = %v, want the spelling of the plan, not the one of Get", got)
	}
}

// Update sends the new name, the new description, and the permissions inside their wrapper, to
// the id in the path.
func TestUpdateSendsTheNewNamesAndTheWrapper(t *testing.T) {
	ctx := context.Background()
	method_things_service.Updated, method_things_service.UpdatedID = nil, nil
	method_things_service.Stored = method_things_service.MethodThing{
		ThingId: 9, Name: "t2", Description: "d", ParentId: 7, ParentName: "Standard", Permissions: []string{"a:read", "c:read"},
	}
	r, schemaResp, root := methodSchema(t)
	idValue := tftypes.NewValue(tftypes.String, "9")
	state := thingValue(root, idValue, "t", "a:read")
	plan := thingValue(root, idValue, "t2", "a:read", "c:read")
	resp := &frameworkresource.UpdateResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: state}}
	r.Update(ctx, frameworkresource.UpdateRequest{
		Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: thingValue(root, tftypes.NewValue(tftypes.String, nil), "t2", "a:read", "c:read")},
		Plan:   tfsdk.Plan{Schema: schemaResp.Schema, Raw: plan},
		State:  tfsdk.State{Schema: schemaResp.Schema, Raw: state},
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if len(method_things_service.Updated) != 1 || method_things_service.UpdatedID[0] != 9 {
		t.Fatalf("update bodies = %+v for %v, want one for 9", method_things_service.Updated, method_things_service.UpdatedID)
	}
	body := method_things_service.Updated[0]
	if body.NewName == nil || *body.NewName != "t2" {
		t.Errorf("update name = %v, want t2", body.NewName)
	}
	if body.NewPermissions == nil || len(body.NewPermissions.Permissions) != 2 {
		t.Errorf("update permissions = %+v, want the two permissions in their wrapper", body.NewPermissions)
	}
}

// Two permissions that differ only in case keep their own spellings when the API keeps both: a
// prior spelling serves one API element, an exact match first.
func TestReadKeepsTwoSpellingsThatDifferOnlyInCase(t *testing.T) {
	ctx := context.Background()
	method_things_service.Created = nil
	method_things_service.Stored = method_things_service.MethodThing{
		ThingId: method_things_service.CreatedID, Name: "t", Description: "d", ParentId: 7, ParentName: "Standard",
		Permissions: []string{"A:READ", "a:read", "b:write"},
	}
	r, schemaResp, root := methodSchema(t)
	resp := &frameworkresource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.NewValue(root, nil)}}
	r.Create(ctx, frameworkresource.CreateRequest{
		Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: thingValue(root, tftypes.NewValue(tftypes.String, nil), "t", "a:read", "A:READ", "b:write")},
		Plan:   tfsdk.Plan{Schema: schemaResp.Schema, Raw: thingValue(root, tftypes.NewValue(tftypes.String, tftypes.UnknownValue), "t", "a:read", "A:READ", "b:write")},
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if got := permissionsOf(t, ctx, resp.State); !slices.Equal(got, []string{"A:READ", "a:read", "b:write"}) {
		t.Errorf("state permissions = %v, want both spellings of a:read", got)
	}
}
