package aaa

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"

	roless "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/role_management_service"
)

func ptr[T any](v T) *T { return &v }

func stringSet(vals ...string) types.Set {
	elements := make([]attr.Value, len(vals))
	for i, v := range vals {
		elements[i] = types.StringValue(v)
	}
	return types.SetValueMust(types.StringType, elements)
}

func apiRole(id int64, permissions ...string) *roless.CustomRole {
	return &roless.CustomRole{
		RoleId:         ptr(id),
		Name:           ptr("Test Role"),
		Description:    ptr("desc"),
		ParentRoleName: ptr("Standard User"),
		Permissions:    permissions,
	}
}

func roleModel(id string, permissions types.Set) *RolesModel {
	return &RolesModel{
		ID:          types.StringValue(id),
		Name:        types.StringValue("Test Role"),
		Description: types.StringValue("desc"),
		ParentRole:  types.StringValue("Standard User"),
		Permissions: permissions,
	}
}

func TestFlattenCustomRole_TakesExtraAPIPermissions(t *testing.T) {
	role := apiRole(123, "view_alerts", "view_dashboards", "view_logs")
	tfModel := roleModel("123", stringSet("view_alerts", "view_dashboards"))

	result, diags := flattenCustomRole(role, tfModel)
	if diags.HasError() {
		t.Fatalf("expected no error diagnostics, got: %v", diags)
	}

	expected := stringSet("view_alerts", "view_dashboards", "view_logs")
	if !result.Permissions.Equal(expected) {
		t.Errorf("expected permissions to reflect API response %v, got: %v", expected, result.Permissions)
	}
}

func TestFlattenCustomRole_DropsPermissionsTheAPIDoesNotHold(t *testing.T) {
	role := apiRole(456, "view_alerts", "view_dashboards")
	tfModel := roleModel("456", stringSet("view_alerts", "view_dashboards", "view_logs"))

	result, diags := flattenCustomRole(role, tfModel)
	if diags.HasError() {
		t.Fatalf("expected no error diagnostics, got: %v", diags)
	}

	expected := stringSet("view_alerts", "view_dashboards")
	if !result.Permissions.Equal(expected) {
		t.Errorf("expected permissions to reflect API response %v, got: %v", expected, result.Permissions)
	}
}

func TestFlattenCustomRole_KeepsTerraformCasing(t *testing.T) {
	// The API may echo a permission spelled differently than the configuration.
	// Keeping Terraform's spelling avoids a diff that no apply could settle.
	role := apiRole(555, "view_alerts", "SendData")
	tfModel := roleModel("555", stringSet("VIEW_ALERTS", "senddata"))

	result, diags := flattenCustomRole(role, tfModel)
	if diags.HasError() {
		t.Fatalf("expected no error diagnostics, got: %v", diags)
	}

	expected := stringSet("VIEW_ALERTS", "senddata")
	if !result.Permissions.Equal(expected) {
		t.Errorf("expected permissions with Terraform's casing %v, got: %v", expected, result.Permissions)
	}
}

func TestFlattenCustomRole_ImportTakesAPIPermissions(t *testing.T) {
	// Import passes through the ID only, so there is no spelling to preserve and
	// the API's values are taken verbatim.
	role := apiRole(101, "view_alerts", "SendData")
	tfModel := &RolesModel{
		ID:          types.StringValue("101"),
		Name:        types.StringNull(),
		Description: types.StringNull(),
		ParentRole:  types.StringNull(),
		Permissions: types.SetNull(types.StringType),
	}

	result, diags := flattenCustomRole(role, tfModel)
	if diags.HasError() {
		t.Fatalf("expected no error diagnostics, got: %v", diags)
	}

	expected := stringSet("view_alerts", "SendData")
	if !result.Permissions.Equal(expected) {
		t.Errorf("expected all API permissions on import %v, got: %v", expected, result.Permissions)
	}
}
