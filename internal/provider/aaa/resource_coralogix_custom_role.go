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
	"log"
	"net/http"

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

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"

	roless "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/role_management_service"
)

func NewCustomRoleSource() resource.Resource {
	return &CustomRoleSource{}
}

type CustomRoleSource struct {
	client *roless.RoleManagementServiceAPIService
}

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
				MarkdownDescription: "Custom role permissions",
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
				},
			},
		},
		MarkdownDescription: "Coralogix Custom Role.",
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

	log.Printf("[INFO] Creating new coralogix_custom_role: %s", utils.FormatJSON(rq))
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
	log.Printf("[INFO] Created new coralogix_custom_role: %s", utils.FormatJSON(result))

	state := flattenCustomRole(result.Role)

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

	log.Printf("[INFO] Reading coralogix_custom_role: %s", utils.FormatJSON(rq))
	result, httpResponse, err := rq.
		Execute()

	if err != nil {
		if httpResponse.StatusCode == http.StatusNotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("coralogix_custom_role %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError("Error reading coralogix_custom_role", utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Read", nil))
		}
		return
	}
	log.Printf("[INFO] Replaced new coralogix_custom_role: %s", utils.FormatJSON(result))
	state = flattenCustomRole(result.Role)

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

	log.Printf("[INFO] Replacing new coralogix_custom_role: %s", utils.FormatJSON(rq))
	_, httpResponse, err := r.client.
		RoleManagementServiceUpdateRole(ctx, *id).
		RoleManagementServiceUpdateRoleRequest(*rq).
		Execute()

	if err != nil {
		if httpResponse.StatusCode == http.StatusNotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("coralogix_custom_role %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError("Error updating coralogix_custom_role", utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Update", nil))
		}
		return
	}
	result, httpResponse, err := r.client.
		RoleManagementServiceGetCustomRole(ctx, *id).
		Execute()
	log.Printf("[INFO] Replaced new coralogix_custom_role: %s", utils.FormatJSON(result))

	state := flattenCustomRole(result.Role)

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

	log.Printf("[INFO] Deleting coralogix_custom_role")

	result, httpResponse, err := r.client.
		RoleManagementServiceDeleteRole(ctx, *id).
		Execute()

	if err != nil {
		resp.Diagnostics.AddError("Error deleting coralogix_custom_role",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Delete", nil),
		)
		return
	}
	log.Printf("[INFO] Deleted coralogix_custom_role: %s", utils.FormatJSON(result))
}

func extractCreateCustomRoleRequest(ctx context.Context, roleModel *RolesModel) (*roless.RoleManagementServiceCreateRoleRequest, diag.Diagnostics) {
	permissions, diags := utils.TypeStringSliceToStringSlice(ctx, roleModel.Permissions.Elements())
	if diags.HasError() {
		return nil, diags
	}

	return &roless.RoleManagementServiceCreateRoleRequest{

		CreateRoleRequestParentRoleName: &roless.CreateRoleRequestParentRoleName{
			ParentRoleName: roleModel.ParentRole.ValueStringPointer(),
			Name:           roleModel.Name.ValueStringPointer(),
			Description:    roleModel.Description.ValueStringPointer(),
			Permissions:    permissions,
		},
	}, nil
}

func flattenCustomRole(customRole *roless.V2CustomRole) *RolesModel {

	return &RolesModel{
		ID:          utils.Int64ToStringValue(customRole.RoleId),
		ParentRole:  types.StringPointerValue(customRole.ParentRoleName),
		Permissions: utils.StringSliceToTypeStringSet(customRole.Permissions),
		Description: types.StringPointerValue(customRole.Description),
		Name:        types.StringPointerValue(customRole.Name),
	}
}

func extractUpdateCustomRoleRequest(ctx context.Context, model *RolesModel) (*roless.RoleManagementServiceUpdateRoleRequest, diag.Diagnostics) {
	permissions, diags := utils.TypeStringSliceToStringSlice(ctx, model.Permissions.Elements())
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
