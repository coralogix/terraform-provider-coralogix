package keything

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

	"example.com/iac-test-sdk/go/openapi/gen/key_things_service"
)

func keySchema(t *testing.T) (*Resource, frameworkresource.SchemaResponse, tftypes.Type) {
	t.Helper()
	ctx := context.Background()
	r := &Resource{client: &key_things_service.KeyThingsServiceAPIService{}}
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

// keyValue is a planned key with one preset and one permission.
func keyValue(root tftypes.Type, id tftypes.Value, name string, enabled bool, permissions ...string) tftypes.Value {
	values := map[string]tftypes.Value{}
	for attrName, attrType := range root.(tftypes.Object).AttributeTypes {
		values[attrName] = tftypes.NewValue(attrType, nil)
	}
	values["id"], values["name"] = id, tftypes.NewValue(tftypes.String, name)
	ownerType := root.(tftypes.Object).AttributeTypes["owner"]
	owner := map[string]tftypes.Value{}
	for attrName, attrType := range ownerType.(tftypes.Object).AttributeTypes {
		owner[attrName] = tftypes.NewValue(attrType, nil)
	}
	owner["team_id"] = tftypes.NewValue(tftypes.String, "42")
	values["owner"] = tftypes.NewValue(ownerType, owner)
	values["enabled"] = tftypes.NewValue(tftypes.Bool, enabled)
	values["hashed"] = tftypes.NewValue(tftypes.Bool, false)
	values["presets"] = stringSet("APM")
	values["permissions"] = stringSet(permissions...)
	return tftypes.NewValue(root, values)
}

func stored(id, name string, enabled bool, permissions ...string) key_things_service.KeyThing {
	team := int64(42)
	return key_things_service.KeyThing{
		Id: id, Name: name, Enabled: enabled, Owner: key_things_service.KeyOwner{TeamId: &team},
		KeyPermissions: key_things_service.KeyThingKeyPermissions{
			Presets:     []key_things_service.PresetThing{{Name: "APM", Permissions: []string{"serverless:Read"}}},
			Permissions: permissions,
		},
	}
}

func setOf(t *testing.T, ctx context.Context, state tfsdk.State, name string) []string {
	t.Helper()
	var set types.Set
	if diags := state.GetAttribute(ctx, path.Root(name), &set); diags.HasError() {
		t.Fatal(diags)
	}
	var out []string
	for _, e := range set.Elements() {
		out = append(out, e.(attr.Value).(types.String).ValueString())
	}
	slices.Sort(out)
	return out
}

// Create sends the name and the names of the presets in the key permissions, under the field names
// of Create. It reads the key with Get, whose presets are objects: the state holds their names.
func TestCreateSendsNamesAndReadsPresetNames(t *testing.T) {
	ctx := context.Background()
	key_things_service.Created = nil
	key_things_service.Stored = stored(key_things_service.CreatedID, "k", true, "livetail:Read")
	r, schemaResp, root := keySchema(t)
	plan := keyValue(root, tftypes.NewValue(tftypes.String, tftypes.UnknownValue), "k", true, "livetail:Read")
	resp := &frameworkresource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.NewValue(root, nil)}}
	r.Create(ctx, frameworkresource.CreateRequest{
		Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: keyValue(root, tftypes.NewValue(tftypes.String, nil), "k", true, "livetail:Read")},
		Plan:   tfsdk.Plan{Schema: schemaResp.Schema, Raw: plan},
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if len(key_things_service.Created) != 1 {
		t.Fatalf("create bodies = %+v, want one", key_things_service.Created)
	}
	body := key_things_service.Created[0]
	if body.Name != "k" || !slices.Equal(body.KeyPermissions.Presets, []string{"APM"}) || !slices.Equal(body.KeyPermissions.Permissions, []string{"livetail:Read"}) {
		t.Fatalf("create body = %+v, want the name and the lists in the key permissions", body)
	}
	if body.Owner.TeamId == nil || *body.Owner.TeamId != 42 || body.Owner.UserId != nil {
		t.Errorf("create owner = %+v, want the team id 42 as a number", body.Owner)
	}
	var owner types.Object
	if diags := resp.State.GetAttribute(ctx, path.Root("owner"), &owner); diags.HasError() {
		t.Fatal(diags)
	}
	if teamID := owner.Attributes()["team_id"].(types.String); teamID.ValueString() != "42" {
		t.Errorf("state owner.team_id = %v, want the text 42", teamID)
	}
	if body.Hashed != nil {
		t.Errorf("create sent hashed = %v: the attribute is read-only", *body.Hashed)
	}
	if got := setOf(t, ctx, resp.State, "presets"); !slices.Equal(got, []string{"APM"}) {
		t.Errorf("state presets = %v, want the names of the objects that Get returned", got)
	}
	if len(key_things_service.Updated) != 0 {
		t.Errorf("create called Update %d times, want none for the default value of enabled", len(key_things_service.Updated))
	}
}

// Update sends the new name as newName and each list in its wrapper object, and the value of
// enabled as isEnabled.
func TestUpdateSendsTheUpdateNamesAndWrappers(t *testing.T) {
	ctx := context.Background()
	key_things_service.Updated, key_things_service.UpdatedID = nil, nil
	id := "key-9"
	key_things_service.Stored = stored(id, "k2", false, "livetail:Read", "serverless:Read")
	r, schemaResp, root := keySchema(t)
	idValue := tftypes.NewValue(tftypes.String, id)
	state := keyValue(root, idValue, "k", true, "livetail:Read")
	plan := keyValue(root, idValue, "k2", false, "livetail:Read", "serverless:Read")
	resp := &frameworkresource.UpdateResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: state}}
	r.Update(ctx, frameworkresource.UpdateRequest{
		Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: keyValue(root, tftypes.NewValue(tftypes.String, nil), "k2", false, "livetail:Read", "serverless:Read")},
		Plan:   tfsdk.Plan{Schema: schemaResp.Schema, Raw: plan},
		State:  tfsdk.State{Schema: schemaResp.Schema, Raw: state},
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if len(key_things_service.Updated) != 1 || key_things_service.UpdatedID[0] != id {
		t.Fatalf("update bodies = %+v for %v, want one for %s", key_things_service.Updated, key_things_service.UpdatedID, id)
	}
	body := key_things_service.Updated[0]
	if body.NewName == nil || *body.NewName != "k2" || body.IsEnabled == nil || *body.IsEnabled {
		t.Fatalf("update body = %+v, want newName k2 and isEnabled false", body)
	}
	if body.Presets == nil || !slices.Equal(body.Presets.Presets, []string{"APM"}) {
		t.Errorf("update presets = %+v, want the names in their wrapper", body.Presets)
	}
	if body.Permissions == nil || !slices.Equal(body.Permissions.Permissions, []string{"livetail:Read", "serverless:Read"}) {
		t.Errorf("update permissions = %+v, want the permissions in their wrapper", body.Permissions)
	}
}

// Create cannot set enabled, so it follows with an Update when the configuration sets another
// value than the default.
func TestCreateFollowsWithAnUpdateForAnotherEnabled(t *testing.T) {
	ctx := context.Background()
	key_things_service.Created, key_things_service.Updated, key_things_service.UpdatedID = nil, nil, nil
	// A new key is active, whatever the configuration says.
	key_things_service.Stored = stored(key_things_service.CreatedID, "k", true, "livetail:Read")
	r, schemaResp, root := keySchema(t)
	plan := keyValue(root, tftypes.NewValue(tftypes.String, tftypes.UnknownValue), "k", false, "livetail:Read")
	resp := &frameworkresource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.NewValue(root, nil)}}
	r.Create(ctx, frameworkresource.CreateRequest{
		Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: keyValue(root, tftypes.NewValue(tftypes.String, nil), "k", false, "livetail:Read")},
		Plan:   tfsdk.Plan{Schema: schemaResp.Schema, Raw: plan},
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if len(key_things_service.Updated) != 1 || key_things_service.Updated[0].IsEnabled == nil || *key_things_service.Updated[0].IsEnabled {
		t.Fatalf("update bodies after create = %+v, want one that sends isEnabled false", key_things_service.Updated)
	}
}
