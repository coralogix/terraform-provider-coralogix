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

package provider

import (
	"context"
	"fmt"
	"log"
	"strconv"

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
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/encoding/protojson"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
)

func NewCustomRoleSource() resource.Resource {
	return &CustomRoleSource{}
}

type CustomRoleSource struct {
	client *cxsdk.RolesClient
}

func (c *CustomRoleSource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_custom_role"
}

type RolesModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	ParentRole  types.String `tfsdk:"parent_role"`
	Permissions types.Set    `tfsdk:"permissions"`
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
}

func (c *CustomRoleSource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
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
				MarkdownDescription: "Custom role permissions",
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
				},
			},
		},
		MarkdownDescription: "Coralogix Custom Role.",
	}
}

func (c *CustomRoleSource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var desiredState *RolesModel
	diags := req.Plan.Get(ctx, &desiredState)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	createCustomRoleRequest, diags := makeCreateCustomRoleRequest(ctx, desiredState)
	if diags.HasError() {
		resp.Diagnostics = diags
		return
	}

	log.Printf("[INFO] Creating Custom Role: %s", protojson.Format(createCustomRoleRequest))
	createCustomRoleResponse, err := c.client.Create(ctx, createCustomRoleRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error creating Custom Role",
			utils.FormatRpcErrors(err, cxsdk.RolesCreateRoleRPC, protojson.Format(createCustomRoleRequest)),
		)
		return
	}
	log.Printf("[INFO] Created custom role with ID: %v", createCustomRoleResponse.Id)

	desiredState.ID = types.StringValue(strconv.FormatInt(int64(createCustomRoleResponse.Id), 10))

	// Set state to fully populated data
	diags = resp.State.Set(ctx, desiredState)
	resp.Diagnostics.Append(diags...)
}

func (c *CustomRoleSource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var currentState *RolesModel
	resp.Diagnostics.Append(req.State.Get(ctx, &currentState)...)
	if resp.Diagnostics.HasError() {
		return
	}
	roleId, err := strconv.Atoi(currentState.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Id", "Custom role id must be an int")
		return
	}

	readReq := &cxsdk.GetCustomRoleRequest{RoleId: uint32(roleId)}
	log.Printf("[INFO] Reading Custom Role: %s", protojson.Format(readReq))
	role, err := c.client.Get(ctx, readReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if cxsdk.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("Custom Role %q is in state, but no longer exists in Coralogix backend", roleId),
				fmt.Sprintf("%d will be recreated when you apply", roleId),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError(
				"Error reading Custom Role",
				utils.FormatRpcErrors(err, cxsdk.RolesGetCustomRoleRPC, protojson.Format(readReq)),
			)
		}
		return
	}
	flattenedRule, diags := flattenCustomRole(ctx, role.GetRole())
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, flattenedRule)...)
}

func (c *CustomRoleSource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var currentState, desiredState *RolesModel
	resp.Diagnostics.Append(req.State.Get(ctx, &currentState)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(req.Plan.Get(ctx, &desiredState)...)
	if resp.Diagnostics.HasError() {
		return
	}

	roleId, err := strconv.Atoi(currentState.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Id", "Custom role id must be an int")
		return
	}
	var updateRoleRequest = cxsdk.UpdateRoleRequest{
		RoleId: uint32(roleId),
	}

	if currentState.ParentRole != desiredState.ParentRole {
		resp.Diagnostics.AddError("Custom role update error", "ParentRole can not be updated!")
		return
	}

	if currentState.Name != desiredState.Name {
		updateRoleRequest.NewName = desiredState.Name.ValueStringPointer()
	}
	if currentState.Description != desiredState.Description {
		updateRoleRequest.NewDescription = desiredState.Description.ValueStringPointer()
	}

	if !currentState.Permissions.Equal(desiredState.Permissions) {
		permissions, diags := utils.TypeStringSliceToStringSlice(ctx, desiredState.Permissions.Elements())
		if diags.HasError() {
			diags.AddError("Custom role update error", "Error extracting permissions")
			resp.Diagnostics.Append(diags...)
			return
		}
		updateRoleRequest.NewPermissions = &cxsdk.RolePermissions{
			Permissions: permissions,
		}
	}

	_, err = c.client.Update(ctx, &updateRoleRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error updating custom role",
			utils.FormatRpcErrors(err, cxsdk.RolesUpdateRoleRPC, protojson.Format(&updateRoleRequest)),
		)
		return
	}

	log.Printf("[INFO] Custom Role %v updated", roleId)

	// Set state to fully populated data
	resp.Diagnostics.Append(resp.State.Set(ctx, desiredState)...)
}

func (c *CustomRoleSource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (c *CustomRoleSource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var currentState *RolesModel
	diags := req.State.Get(ctx, &currentState)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := strconv.Atoi(currentState.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Id", "Custom role id must be an int")
		return
	}
	deleteRoleRequest := cxsdk.DeleteRoleRequest{
		RoleId: uint32(id),
	}

	_, err = c.client.Delete(ctx, &deleteRoleRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error deleting Custom Role",
			utils.FormatRpcErrors(err, cxsdk.RolesDeleteRoleRPC, protojson.Format(&deleteRoleRequest)),
		)
		return
	}

	log.Printf("[INFO] Custom Role %v deleted", id)
}

func makeCreateCustomRoleRequest(ctx context.Context, roleModel *RolesModel) (*cxsdk.CreateRoleRequest, diag.Diagnostics) {
	permissions, diags := utils.TypeStringSliceToStringSlice(ctx, roleModel.Permissions.Elements())
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.CreateRoleRequest{
		Name:        roleModel.Name.ValueString(),
		Description: roleModel.Description.ValueString(),
		Permissions: permissions,
		ParentRole:  &cxsdk.CreateRoleRequestParentRoleName{ParentRoleName: roleModel.ParentRole.ValueString()},
	}, nil
}

func flattenCustomRole(ctx context.Context, customRole *cxsdk.CustomRole) (*RolesModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	permissions, diags := types.SetValueFrom(ctx, types.StringType, customRole.Permissions)
	if diags.HasError() {
		return nil, diags
	}

	model := RolesModel{
		ID:          types.StringValue(strconv.Itoa(int(customRole.RoleId))),
		ParentRole:  types.StringValue(customRole.ParentRoleName),
		Permissions: permissions,
		Description: types.StringValue(customRole.Description),
		Name:        types.StringValue(customRole.Name),
	}

	return &model, nil
}
