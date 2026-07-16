// Copyright 2024 Coralogix Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package aaa

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"

	roless "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/role_management_service"
)

func NewCustomRoleSource() resource.Resource {
	return &CustomRoleSource{}
}

type CustomRoleSource struct {
	client      *roless.RoleManagementServiceAPIService
	permissions *cxsdk.PermissionsClient
	// aliasMap maps lowercase deprecated permission expressions to lowercase canonical expressions.
	// Built once from ListAllPermissions. Empty map (nil) means no alias resolution is available.
	aliasMap map[string]string
}

// Ensure CustomRoleSource implements resource.ResourceWithModifyPlan.
var _ resource.ResourceWithModifyPlan = &CustomRoleSource{}

func (r *CustomRoleSource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_custom_role"
}

type RolesModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	ParentRole  types.String `tfsdk:"parent_role"`
	Permissions types.Set    `tfsdk:"permissions"`
}

func (r *CustomRoleSource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	clientSet, ok := req.ProviderData.(*clientset.ClientSet)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *clientset.ClientSet, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = clientSet.CustomRoles()
	r.permissions = clientSet.Permissions()

	// Eagerly fetch the permission alias map so flattenCustomRole and ModifyPlan can use it.
	// This call is non-fatal: if the endpoint is unavailable (e.g., not yet deployed), we fall
	// back to an empty alias map which preserves the pre-existing exact-case-insensitive comparison.
	if r.permissions != nil {
		aliasResp, err := r.permissions.ListAll(ctx, &cxsdk.ListAllPermissionsRequest{})
		if err != nil {
			resp.Diagnostics.AddWarning(
				"Could not fetch permission aliases",
				fmt.Sprintf("ListAllPermissions failed: %s. Deprecated permission expression aliases will not be resolved; existing permissions will still be compared case-insensitively.", err),
			)
			r.aliasMap = map[string]string{}
		} else {
			r.aliasMap = buildPermissionAliasMap(aliasResp.GetPermissions())
		}
	} else {
		r.aliasMap = map[string]string{}
	}
}

// buildPermissionAliasMap converts the ListAllPermissions response into a lookup map.
// Keys are lowercase deprecated expression forms; values are lowercase canonical expressions.
func buildPermissionAliasMap(perms []*cxsdk.RbacPermission) map[string]string {
	aliases := make(map[string]string, len(perms))
	for _, p := range perms {
		canonical := strings.ToLower(p.GetExpression())
		if canonical == "" {
			continue
		}
		for _, dep := range p.GetDeprecatedExpressions() {
			lower := strings.ToLower(dep)
			if lower != "" && lower != canonical {
				aliases[lower] = canonical
			}
		}
	}
	return aliases
}

// normalizePermission returns the lowercase canonical form of a permission expression.
// If the expression is a known deprecated alias, it returns its canonical replacement.
func normalizePermission(expr string, aliases map[string]string) string {
	lower := strings.ToLower(expr)
	if canonical, ok := aliases[lower]; ok {
		return canonical
	}
	return lower
}

func (r *CustomRoleSource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version: 0,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "Custom Role ID.",
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Custom Role name.",
			},
			"description": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Custom Role description.",
			},
			"parent_role": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Parent role name",
			},
			"permissions": schema.SetAttribute{
				Required:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Custom role permissions. Deprecated expression forms (e.g. `alerts-map:Read`) are accepted and treated as equivalent to their canonical replacement (e.g. `alerts:MapRead`); no drift will appear unless you change the actual set of permissions.",
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
				},
			},
		},
		MarkdownDescription: "Coralogix Custom Role. For more info please review - https://coralogix.com/docs/user-guides/account-management/user-management/create-roles-and-permissions/.",
	}
}

// ModifyPlan suppresses spurious diffs for the permissions set when every element in the
// plan normalizes to the same canonical expression as its counterpart in state. This handles
// the case where state has been updated to a canonical form (e.g. "alerts:MapRead") but the
// user's config still contains the deprecated alias (e.g. "alerts-map:Read").
func (r *CustomRoleSource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	// Only act during update plans: both state and plan must be non-null.
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}

	var planPerms, statePerms types.Set
	if diags := req.Plan.GetAttribute(ctx, path.Root("permissions"), &planPerms); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	if diags := req.State.GetAttribute(ctx, path.Root("permissions"), &statePerms); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	if planPerms.IsNull() || planPerms.IsUnknown() || statePerms.IsNull() || statePerms.IsUnknown() {
		return
	}
	if len(planPerms.Elements()) != len(statePerms.Elements()) {
		return
	}

	planList := utils.TypeStringSetToStringSlice(ctx, planPerms)
	stateList := utils.TypeStringSetToStringSlice(ctx, statePerms)

	// Build sets of normalized canonicals for both sides.
	planNorm := make(map[string]bool, len(planList))
	for _, p := range planList {
		planNorm[normalizePermission(p, r.aliasMap)] = true
	}
	stateNorm := make(map[string]bool, len(stateList))
	for _, p := range stateList {
		stateNorm[normalizePermission(p, r.aliasMap)] = true
	}

	// If every canonical in the plan is covered by state canonicals (and vice-versa), the
	// effective permission set is identical — suppress the diff by keeping the state value.
	for k := range planNorm {
		if !stateNorm[k] {
			return
		}
	}
	for k := range stateNorm {
		if !planNorm[k] {
			return
		}
	}

	if diags := resp.Plan.SetAttribute(ctx, path.Root("permissions"), statePerms); diags.HasError() {
		resp.Diagnostics.Append(diags...)
	}
}

func (r *CustomRoleSource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan *RolesModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	rq, diags := extractCreateCustomRoleRequest(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	createResult, httpResponse, err := r.client.
		RoleManagementServiceCreateRole(ctx).
		RoleManagementServiceCreateRoleRequest(*rq).
		Execute()

	if err != nil {
		resp.Diagnostics.AddError("Error creating coralogix_custom_role",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Create", rq),
		)
		return
	}

	result, httpResponse, err := r.client.
		RoleManagementServiceGetCustomRole(ctx, *createResult.Id).
		Execute()
	if err != nil {
		resp.Diagnostics.AddError("Error refreshing updated coralogix_custom_role. State was not updated", utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Read", nil))
		return
	}

	state, err := flattenCustomRole(result.Role, plan, r.aliasMap)
	if err != nil {
		resp.Diagnostics.AddError("Error flattening coralogix_custom_role after creation", err.Error())
		return
	}

	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
}

func (r *CustomRoleSource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state *RolesModel

	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, diags := utils.TypeStringToInt64Pointer(state.ID)
	if diags.HasError() {
		return
	}

	rq := r.client.
		RoleManagementServiceGetCustomRole(ctx, *id)

	result, httpResponse, err := rq.
		Execute()

	if err != nil {
		if httpResponse.StatusCode == http.StatusNotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("coralogix_custom_role %v is in state, but no longer exists in Coralogix backend", *id),
				fmt.Sprintf("%v will be recreated when you apply", *id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError("Error reading coralogix_custom_role", utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Read", nil))
		}
		return
	}
	state, err = flattenCustomRole(result.Role, state, r.aliasMap)
	if err != nil {
		resp.Diagnostics.AddError("Error flattening coralogix_custom_role after read", err.Error())
		return
	}

	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
}

func (r *CustomRoleSource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan *RolesModel

	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	id, diags := utils.TypeStringToInt64Pointer(plan.ID)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	rq, diags := extractUpdateCustomRoleRequest(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, httpResponse, err := r.client.
		RoleManagementServiceUpdateRole(ctx, *id).
		RoleManagementServiceUpdateRoleRequest(*rq).
		Execute()

	if err != nil {
		if httpResponse.StatusCode == http.StatusNotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("coralogix_custom_role %v is in state, but no longer exists in Coralogix backend", *id),
				fmt.Sprintf("%v will be recreated when you apply", *id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError("Error updating coralogix_custom_role", utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Update", rq))
		}
		return
	}
	result, httpResponse, err := r.client.
		RoleManagementServiceGetCustomRole(ctx, *id).
		Execute()

	if err != nil {
		resp.Diagnostics.AddError("Error refreshing updated coralogix_custom_role. State was not updated", utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Read", nil))
		return
	}

	state, err := flattenCustomRole(result.Role, plan, r.aliasMap)
	if err != nil {
		resp.Diagnostics.AddError("Error flattening coralogix_custom_role after update", err.Error())
		return
	}

	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
}

func (r *CustomRoleSource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *CustomRoleSource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state *RolesModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, diags := utils.TypeStringToInt64Pointer(state.ID)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	rq := r.client.
		RoleManagementServiceDeleteRole(ctx, *id)

	_, httpResponse, err := rq.
		Execute()

	if err != nil {
		resp.Diagnostics.AddError("Error deleting coralogix_custom_role",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Delete", nil),
		)
		return
	}
}

func extractCreateCustomRoleRequest(ctx context.Context, roleModel *RolesModel) (*roless.RoleManagementServiceCreateRoleRequest, diag.Diagnostics) {
	permissions, diags := utils.TypeStringElementsToStringSlice(ctx, roleModel.Permissions.Elements())
	if diags.HasError() {
		return nil, diags
	}

	return &roless.RoleManagementServiceCreateRoleRequest{
		ParentRoleName: roleModel.ParentRole.ValueStringPointer(),
		Name:           roleModel.Name.ValueStringPointer(),
		Description:    roleModel.Description.ValueStringPointer(),
		Permissions:    permissions,
	}, nil
}

// flattenCustomRole converts an API CustomRole into a RolesModel.
//
// When plan permissions are provided (i.e. not an import), it verifies that
// every plan permission is present in the API response — using normalizePermission
// so that deprecated expression aliases (e.g. "alerts-map:Read") are treated as
// equivalent to their canonical form (e.g. "alerts:MapRead"). On a match the plan's
// original form is kept in state, preventing spurious diffs on the next plan.
func flattenCustomRole(customRole *roless.CustomRole, plan *RolesModel, aliasMap map[string]string) (*RolesModel, error) {
	permissionsFromPlan := utils.TypeStringSetToStringSlice(context.Background(), plan.Permissions)
	permissionsFromAPI := customRole.Permissions
	// permissions are required, so if plan.Permissions is null, it must mean that we're importing
	isImport := plan.Permissions.IsNull()

	if isImport {
		// Just take what the API gives us and return, because we're importing
		apiPermsSet, diags := types.SetValueFrom(context.Background(), types.StringType, customRole.Permissions)
		if diags.HasError() {
			return nil, fmt.Errorf("failed to convert API permissions to set")
		}

		return &RolesModel{
			ID:          utils.Int64ToStringValue(customRole.RoleId),
			ParentRole:  types.StringPointerValue(customRole.ParentRoleName),
			Permissions: apiPermsSet,
			Description: types.StringPointerValue(customRole.Description),
			Name:        types.StringPointerValue(customRole.Name),
		}, nil
	}
	if len(permissionsFromAPI) != len(permissionsFromPlan) {
		return nil, fmt.Errorf("the number of permissions specified in the plan (%d) does not match the number of permissions returned from the Coralogix API (%d).", len(permissionsFromPlan), len(permissionsFromAPI))
	}
	for _, perm := range permissionsFromPlan {
		normalizedPlanPerm := normalizePermission(perm, aliasMap)
		permissionWasReturnedFromAPI := false
		for _, apiPerm := range permissionsFromAPI {
			if normalizedPlanPerm == normalizePermission(apiPerm, aliasMap) {
				permissionWasReturnedFromAPI = true
				break
			}
		}
		if !permissionWasReturnedFromAPI {
			return nil, fmt.Errorf("permission %s was specified in the plan but was not returned from the Coralogix API.", perm)
		}
	}

	return &RolesModel{
		ID:         utils.Int64ToStringValue(customRole.RoleId),
		ParentRole: types.StringPointerValue(customRole.ParentRoleName),
		// Keep the plan's original permission forms in state (preserving the user's spelling,
		// whether canonical or deprecated), so that the next plan sees no diff as long as the
		// effective permission set is unchanged.
		Permissions: plan.Permissions,
		Description: types.StringPointerValue(customRole.Description),
		Name:        types.StringPointerValue(customRole.Name),
	}, nil
}

func extractUpdateCustomRoleRequest(ctx context.Context, model *RolesModel) (*roless.RoleManagementServiceUpdateRoleRequest, diag.Diagnostics) {
	permissions, diags := utils.TypeStringElementsToStringSlice(ctx, model.Permissions.Elements())
	if diags.HasError() {
		return nil, diags
	}
	return &roless.RoleManagementServiceUpdateRoleRequest{
		NewDescription: model.Description.ValueStringPointer(),
		NewName:        model.Name.ValueStringPointer(),
		NewPermissions: &roless.V2Permissions{
			Permissions: permissions,
		},
	}, nil
}
