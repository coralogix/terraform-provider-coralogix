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

func TestFlattenCustomRole_ApplyKeepsPlannedPermissionsWhenAPIAddsSome(t *testing.T) {
	role := apiRole(123, "view_alerts", "view_dashboards", "view_logs")
	plan := roleModel("123", stringSet("view_alerts", "view_dashboards"))

	result, diags := flattenCustomRole(role, plan, permissionsFromPlan)
	if diags.HasError() {
		t.Fatalf("expected no error diagnostics, got: %v", diags)
	}
	if !result.Permissions.Equal(plan.Permissions) {
		t.Errorf("expected planned permissions %v, got: %v", plan.Permissions, result.Permissions)
	}
}

func TestFlattenCustomRole_ApplyKeepsPlannedPermissionsWhenAPIDropsSome(t *testing.T) {
	role := apiRole(456, "view_alerts", "view_dashboards")
	plan := roleModel("456", stringSet("view_alerts", "view_dashboards", "view_logs"))

	result, diags := flattenCustomRole(role, plan, permissionsFromPlan)
	if diags.HasError() {
		t.Fatalf("expected no error diagnostics, got: %v", diags)
	}
	if !result.Permissions.Equal(plan.Permissions) {
		t.Errorf("expected planned permissions %v, got: %v", plan.Permissions, result.Permissions)
	}
}

func TestFlattenCustomRole_ApplyRejectsNullPermissions(t *testing.T) {
	role := apiRole(321, "view_alerts")
	plan := roleModel("321", types.SetNull(types.StringType))

	if _, diags := flattenCustomRole(role, plan, permissionsFromPlan); !diags.HasError() {
		t.Fatal("expected an error for a null permissions set on the apply path")
	}
}

func TestFlattenCustomRole_RefreshSurfacesExtraAPIPermissions(t *testing.T) {
	role := apiRole(123, "view_alerts", "view_dashboards", "view_logs")
	state := roleModel("123", stringSet("view_alerts", "view_dashboards"))

	result, diags := flattenCustomRole(role, state, permissionsFromAPI)
	if diags.HasError() {
		t.Fatalf("expected no error diagnostics, got: %v", diags)
	}

	expected := stringSet("view_alerts", "view_dashboards", "view_logs")
	if !result.Permissions.Equal(expected) {
		t.Errorf("expected permissions to reflect API response %v, got: %v", expected, result.Permissions)
	}
}

func TestFlattenCustomRole_RefreshSurfacesMissingAPIPermissions(t *testing.T) {
	role := apiRole(456, "view_alerts", "view_dashboards")
	state := roleModel("456", stringSet("view_alerts", "view_dashboards", "view_logs"))

	result, diags := flattenCustomRole(role, state, permissionsFromAPI)
	if diags.HasError() {
		t.Fatalf("expected no error diagnostics, got: %v", diags)
	}

	expected := stringSet("view_alerts", "view_dashboards")
	if !result.Permissions.Equal(expected) {
		t.Errorf("expected permissions to reflect API response %v, got: %v", expected, result.Permissions)
	}
}

func TestFlattenCustomRole_RefreshPreservesStateCasing(t *testing.T) {
	// The API may echo a different casing than the configuration used. Keeping the
	// stored casing avoids a diff that no apply could ever settle.
	role := apiRole(555, "view_alerts", "view_dashboards")
	state := roleModel("555", stringSet("VIEW_ALERTS", "VIEW_DASHBOARDS"))

	result, diags := flattenCustomRole(role, state, permissionsFromAPI)
	if diags.HasError() {
		t.Fatalf("expected no error diagnostics, got: %v", diags)
	}

	expected := stringSet("VIEW_ALERTS", "VIEW_DASHBOARDS")
	if !result.Permissions.Equal(expected) {
		t.Errorf("expected permissions with state casing %v, got: %v", expected, result.Permissions)
	}
}

func TestFlattenCustomRole_Import(t *testing.T) {
	// Import passes through the ID only, so Read starts with a null permissions
	// set and takes the API's values.
	role := apiRole(101, "view_alerts", "view_dashboards", "view_logs")
	state := &RolesModel{
		ID:          types.StringValue("101"),
		Name:        types.StringNull(),
		Description: types.StringNull(),
		ParentRole:  types.StringNull(),
		Permissions: types.SetNull(types.StringType),
	}

	result, diags := flattenCustomRole(role, state, permissionsFromAPI)
	if diags.HasError() {
		t.Fatalf("expected no error diagnostics, got: %v", diags)
	}

	expected := stringSet("view_alerts", "view_dashboards", "view_logs")
	if !result.Permissions.Equal(expected) {
		t.Errorf("expected all API permissions on import %v, got: %v", expected, result.Permissions)
	}
}
