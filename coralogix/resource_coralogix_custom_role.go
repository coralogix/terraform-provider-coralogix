package coralogix

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"log"
	"reflect"
	"slices"
	"strings"
	"terraform-provider-coralogix/coralogix/clientset"
	roles "terraform-provider-coralogix/coralogix/clientset/grpc/roles"
)

var (
	getRolePath          = "com.coralogixapis.aaa.rbac.v2.RoleManagementService/GetCustomRole"
	createRolePath       = "com.coralogixapis.aaa.rbac.v2.RoleManagementService/CreateRole"
	deleteRolePath       = "com.coralogixapis.aaa.rbac.v2.RoleManagementService/DeleteRole"
	updateRolePath       = "com.coralogixapis.aaa.rbac.v2.RoleManagementService/UpdateRole"
	getSystemRolesPath   = "com.coralogixapis.aaa.rbac.v2.RoleManagementService/UpdateRole"
	getAllPermissionPath = "com.coralogixapis.aaa.rbac.v2.RoleManagementService/UpdateRole"
)

func NewCustomRoleSource() resource.Resource {
	return &CustomRoleSource{}
}

type CustomRoleSource struct {
	client               *clientset.RolesClient
	parentRolesMapping   map[string]*roles.SystemRole
	supportedPermissions []*roles.Permission
}

func (c *CustomRoleSource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_custom_role"
}

type RolesModel struct {
	ID          types.Int64  `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	ParentRole  types.String `tfsdk:"parent_role"`
	Permissions types.Set    `tfsdk:"permissions"`
	TeamId      types.Int64  `tfsdk:"team_id"`
}

func (c *CustomRoleSource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	c.client = clientSet.CustomRoles()

	systemRoles, diags := c.fetchAllSystemRoles(ctx)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	c.parentRolesMapping = systemRoles

	allPermissions, diags := c.fetchAllPermissions(ctx)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	c.supportedPermissions = allPermissions

}

func (c *CustomRoleSource) Schema(ctx context.Context, request resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version: 0,
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
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
			"team_id": schema.Int64Attribute{
				Required:            true,
				MarkdownDescription: "Custom Role teamId.",
			},
			"parent_role": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Parent roles name",
			},
			"permissions": schema.SetAttribute{
				Required:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Custom role permissions",
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
				},
			},
		},
		MarkdownDescription: "Coralogix Api keys.",
	}
}

func (c *CustomRoleSource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var desiredState *RolesModel
	diags := req.Plan.Get(ctx, &desiredState)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	createCustomRoleRequest, diags := c.makeCreateCustomRoleRequest(ctx, desiredState)
	if diags.HasError() {
		resp.Diagnostics = diags
		return
	}

	log.Printf("[INFO] Creating Custom Role: %s", protojson.Format(createCustomRoleRequest))
	createCustomRoleResponse, err := c.client.CreateRole(ctx, createCustomRoleRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if status.Code(err) == codes.PermissionDenied || status.Code(err) == codes.Unauthenticated {
			resp.Diagnostics.AddError(
				"Error creating Custom Role",
				fmt.Sprintf("permission denied for url - %s\ncheck your org-key and permissions", createRolePath),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error creating Custom Role",
				formatRpcErrors(err, createApiKeyPath, protojson.Format(createCustomRoleRequest)),
			)
		}
		return
	}
	log.Printf("[INFO] Created custom role with ID: %v", createCustomRoleResponse.Id)

	desiredState.ID = types.Int64Value(int64(createCustomRoleResponse.Id))
	// Set state to fully populated data
	diags = resp.State.Set(ctx, desiredState)
	resp.Diagnostics.Append(diags...)
}

func (c *CustomRoleSource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var currentState *RolesModel
	diags := req.State.Get(ctx, &currentState)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	model, done := c.getRoleById(ctx, uint32(currentState.ID.ValueInt64()))
	if done.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	diags = resp.State.Set(ctx, model)
	resp.Diagnostics.Append(diags...)
}

func (c *CustomRoleSource) getRoleById(ctx context.Context, roleId uint32) (*RolesModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	getCustomRoleRequest := roles.GetCustomRoleRequest{
		RoleId: roleId,
	}

	createCustomRoleResponse, err := c.client.GetRole(ctx, &getCustomRoleRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if status.Code(err) == codes.PermissionDenied || status.Code(err) == codes.Unauthenticated {
			diags.AddError(
				"Error creating Custom Role",
				fmt.Sprintf("permission denied for url - %s\ncheck your org-key and permissions", getRolePath),
			)
		} else {
			diags.AddError(
				"Error creating Custom Role",
				formatRpcErrors(err, createApiKeyPath, protojson.Format(&getCustomRoleRequest)),
			)
		}
		return nil, diags
	}

	model, diags := c.flatterCustomRole(ctx, createCustomRoleResponse.GetRole())
	if diags.HasError() {
		return nil, diags
	}

	model.Permissions, diags = types.SetValueFrom(ctx, types.StringType, model.Permissions.Elements())
	if diags.HasError() {
		return nil, diags
	}

	return model, nil
}

func (c *CustomRoleSource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var currentState, desiredState *RolesModel
	diags := req.State.Get(ctx, &currentState)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}
	diags = req.Plan.Get(ctx, &desiredState)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := currentState.ID.ValueInt64()

	var updateRoleRequest = roles.UpdateRoleRequest{
		RoleId: uint32(id),
	}

	if currentState.TeamId != desiredState.TeamId {
		diags.AddError("Custom role update error", "TeamId can not be updated!")
		resp.Diagnostics.Append(diags...)
		return
	}
	if currentState.ParentRole != desiredState.ParentRole {
		diags.AddError("Custom role update error", "ParentRole can not be updated!")
		resp.Diagnostics.Append(diags...)
		return
	}

	if currentState.Name != desiredState.Name {
		updateRoleRequest.NewName = desiredState.Name.ValueStringPointer()
	}
	if currentState.Description != desiredState.Description {
		updateRoleRequest.NewDescription = desiredState.Description.ValueStringPointer()
	}

	if !reflect.DeepEqual(currentState.Permissions.Elements(), desiredState.Permissions.Elements()) {
		permissions, diags := typeStringSliceToStringSlice(ctx, desiredState.Permissions.Elements())
		if diags.HasError() {
			diags.AddError("Custom role update error", "Error extracting permissions")
			resp.Diagnostics.Append(diags...)
			return
		}
		systemRole, diags := c.getIdFromSystemRoleName(currentState.ParentRole.ValueStringPointer())
		if diags.HasError() {
			diags.AddError("Invalid ParentRole", "Error extracting parent role")
			resp.Diagnostics.Append(diags...)
			return
		}

		_, diags = c.validatePermissions(&permissions, systemRole)
		if diags.HasError() {
			diags.AddError("Custom role update error", "Error extracting permissions")
			resp.Diagnostics.Append(diags...)
			return
		}
		updateRoleRequest.NewPermissions = &roles.Permissions{
			Permissions: permissions,
		}
	}

	_, err := c.client.UpdateRole(ctx, &updateRoleRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if status.Code(err) == codes.PermissionDenied || status.Code(err) == codes.Unauthenticated {
			resp.Diagnostics.AddError(
				"Error deleting Custom Role",
				fmt.Sprintf("permission denied for url - %s\ncheck your org-key and permissions", updateRolePath),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error updating custom role",
				formatRpcErrors(err, deleteApiKeyPath, protojson.Format(&updateRoleRequest)),
			)
		}
		return
	}

	log.Printf("[INFO] Custom Role %s updated", id)

	if diags.HasError() {
		return
	}

	// Set state to fully populated data
	resp.Diagnostics.Append(resp.State.Set(ctx, desiredState)...)
}

func (c *CustomRoleSource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var currentState *RolesModel
	diags := req.State.Get(ctx, &currentState)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := currentState.ID.ValueInt64()

	deleteRoleRequest := roles.DeleteRoleRequest{
		RoleId: uint32(currentState.ID.ValueInt64()),
	}

	_, err := c.client.DeleteRole(ctx, &deleteRoleRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if status.Code(err) == codes.PermissionDenied || status.Code(err) == codes.Unauthenticated {
			resp.Diagnostics.AddError(
				"Error deleting Custom Role",
				fmt.Sprintf("permission denied for url - %s\ncheck your org-key and permissions", deleteApiKeyPath),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error getting Custom Role",
				formatRpcErrors(err, deleteApiKeyPath, protojson.Format(&deleteRoleRequest)),
			)
		}
		return
	}

	log.Printf("[INFO] Custom Role %s deleted", id)
}

func (c *CustomRoleSource) fetchAllSystemRoles(ctx context.Context) (map[string]*roles.SystemRole, diag.Diagnostics) {
	var diags diag.Diagnostics
	role, err := c.client.ListSystemRole(ctx, &roles.ListSystemRolesRequest{})
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if status.Code(err) == codes.PermissionDenied || status.Code(err) == codes.Unauthenticated {
			diags.AddError(
				"Error getting System Role",
				fmt.Sprintf("permission denied for url - %s\ncheck your org-key and permissions", getSystemRolesPath),
			)
		} else {
			diags.AddError(
				"Error getting System Role",
				formatRpcErrors(err, getApiKeyPath, protojson.Format(&roles.ListSystemRolesRequest{})),
			)
		}
		return nil, diags
	}

	thisMap := make(map[string]*roles.SystemRole)
	for _, elem := range role.GetRoles() {
		thisMap[elem.Name] = elem
	}

	return thisMap, nil
}

func (c *CustomRoleSource) makeCreateCustomRoleRequest(ctx context.Context, roleModel *RolesModel) (*roles.CreateRoleRequest, diag.Diagnostics) {
	permissions, diags := typeStringSliceToStringSlice(ctx, roleModel.Permissions.Elements())
	if diags.HasError() {
		return nil, diags
	}
	systemRole, diags := c.getIdFromSystemRoleName(roleModel.ParentRole.ValueStringPointer())
	if diags.HasError() {
		return nil, diags
	}

	_, diags = c.validatePermissions(&permissions, systemRole)
	if diags.HasError() {
		return nil, diags
	}

	return &roles.CreateRoleRequest{
		Name:         roleModel.Name.ValueString(),
		Description:  roleModel.Description.ValueString(),
		ParentRoleId: systemRole.RoleId,
		Permissions:  permissions,
		TeamId:       uint32(roleModel.TeamId.ValueInt64()),
	}, nil
}

func (c *CustomRoleSource) getIdFromSystemRoleName(parentRole *string) (*roles.SystemRole, diag.Diagnostics) {
	var diags diag.Diagnostics
	roleId, ok := c.parentRolesMapping[*parentRole]

	if !ok {
		diags.AddError("Invalid parent role!", c.logAvailableParentRoles())
		return nil, diags
	}
	return roleId, nil
}

func (c *CustomRoleSource) fetchAllPermissions(ctx context.Context) ([]*roles.Permission, diag.Diagnostics) {
	var diags diag.Diagnostics
	existingPermissions, err := c.client.ListPermissions(ctx, &roles.ListAllPermissionsRequest{})
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if status.Code(err) == codes.PermissionDenied || status.Code(err) == codes.Unauthenticated {
			diags.AddError(
				"Error getting All permissions",
				fmt.Sprintf("permission denied for url - %s\ncheck your org-key and permissions", getAllPermissionPath),
			)
		} else {
			diags.AddError(
				"Error getting supported permissions",
				formatRpcErrors(err, getApiKeyPath, protojson.Format(&roles.ListAllPermissionsRequest{})),
			)
		}
		return nil, diags
	}
	return existingPermissions.GetPermissions(), nil
}

func (c *CustomRoleSource) validatePermissions(permissions *[]string, parentRole *roles.SystemRole) ([]string, diag.Diagnostics) {
	var diags diag.Diagnostics

	for _, elem := range *permissions {
		exist := slices.ContainsFunc(c.supportedPermissions, func(e *roles.Permission) bool {
			return *e.Expression == elem
		})
		if !exist {
			diags.AddError(fmt.Sprintf("Invalid permission %s", elem), c.logAvailablePermissions())
			return nil, diags
		}
		exist = slices.ContainsFunc(parentRole.Permissions, func(parentRolePermissions string) bool {
			return parentRolePermissions == elem
		})
		if exist {
			diags.AddError("Permission already assigned from parent role; permission %s", elem)
			return nil, diags
		}
	}

	return *permissions, nil
}

func (c *CustomRoleSource) logAvailablePermissions() string {

	var message []string
	for _, p := range c.supportedPermissions {
		message = append(message, fmt.Sprintf("%s |  %s  | %s  | %s  |", *p.Expression, *p.Description, *p.Explanation, *p.DocLink))
	}

	return strings.Join(message, "\n")
}

func (c *CustomRoleSource) logAvailableParentRoles() string {
	var message []string
	for _, p := range c.parentRolesMapping {
		message = append(message, fmt.Sprintf("%s |  %s  |", p.Name, p.Description))
	}

	return strings.Join(message, "\n")
}

func (c *CustomRoleSource) flatterCustomRole(ctx context.Context, customRole *roles.CustomRole) (*RolesModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	conditionSatisfied := ""
	for key, val := range c.parentRolesMapping {
		if val.RoleId == customRole.ParentRoleId {
			conditionSatisfied = key
			break
		}
	}
	if len(conditionSatisfied) == 0 {
		diags.AddError("Invalid parent role id", "Parent role not found!")
		return nil, diags
	}

	permissions, diags := types.SetValueFrom(ctx, types.StringType, customRole.Permissions)
	if diags.HasError() {
		return nil, diags
	}

	model := RolesModel{
		ID:          types.Int64Value(int64(customRole.RoleId)),
		TeamId:      types.Int64Value(int64(customRole.TeamId)),
		ParentRole:  types.StringValue(conditionSatisfied),
		Permissions: permissions,
		Description: types.StringValue(customRole.Description),
		Name:        types.StringValue(customRole.Name),
	}

	return &model, nil
}
