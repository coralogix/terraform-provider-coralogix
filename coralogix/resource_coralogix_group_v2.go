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

package coralogix

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"

	"terraform-provider-coralogix/coralogix/clientset"
	"terraform-provider-coralogix/coralogix/utils"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/grpc/codes"
)

func NewGroupV2Resource() resource.Resource {
	return &GroupV2Resource{}
}

type GroupV2Resource struct {
	client *cxsdk.GroupsClient
}

func (r *GroupV2Resource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group_v2"
}

func (r *GroupV2Resource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	r.client = clientSet.GroupGrpc()
}

func (r *GroupV2Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version: 0,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "Group ID.",
			},
			"display_name": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				MarkdownDescription: "Group display name.",
			},
			"members": schema.SetAttribute{
				Optional:    true,
				ElementType: types.StringType,
			},
			"role": schema.StringAttribute{
				Required: true,
			},
			"scope_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Scope attached to the group.",
				Computed:            true,
			},
		},
		MarkdownDescription: "Coralogix group.",
	}
}

func (r *GroupV2Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *GroupV2Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan *GroupV2ResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	createGroupRequest, diags := extractGroupV2(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	groupStr, _ := json.Marshal(createGroupRequest)
	log.Printf("[INFO] Creating new group: %s", string(groupStr))
	createResp, err := r.client.Create(ctx, createGroupRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error creating Group",
			utils.FormatRpcErrors(err, cxsdk.CreateTeamGroupRPC, string(groupStr)),
		)
		return
	}
	getResp, err := r.client.Get(ctx, &cxsdk.GetTeamGroupRequest{GroupId: createResp.GroupId})
	groupStr, _ = json.Marshal(getResp)
	log.Printf("[INFO] Getting group: %s", groupStr)
	state, diags := flattenGroup(getResp)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// Set state to fully populated data
	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
}

func flattenGroup(group *cxsdk.GetTeamGroupResponse) (*GroupV2ResourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	state := &GroupV2ResourceModel{
		TeamId:         types.StringValue(group),
		Name:           types.StringValue(group.Name),
		Description:    types.StringValue(group.Description),
		ExternalId:     types.StringValue(group.ExternalId),
		NextGenScopeId: types.StringValue(group.NextGenScopeId),
	}

	if group.TeamId != nil {
		state.TeamId = types.StringValue(strconv.Itoa(int(group.TeamId.Id)))
	} else {
		state.TeamId = types.StringNull()
	}

	if group.RoleIds != nil {
		state.RoleIds, diags = flattenSCIMGroupMembers(group.RoleIds)
		if diags.HasError() {
			return nil, diags
		}
	} else {
		state.RoleIds = types.SetNull(types.StringType)
	}

	if group.Members != nil {
		state.UserIds, diags = flattenSCIMGroupMembers(group.Members)
		if diags.HasError() {
			return nil, diags
		}
	} else {
		state.UserIds = types.SetNull(types.StringType)
	}

	return state, diags
}

func (r *GroupV2Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state *GroupV2ResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	//Get refreshed Group value from Coralogix
	id := state.ID.ValueString()
	log.Printf("[INFO] Reading Group: %s", id)
	getGroupResp, err := r.client.GetGroup(ctx, id)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if cxsdk.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("Group %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError(
				"Error reading Group",
				utils.FormatRpcErrors(err, fmt.Sprintf("%s/%s", r.client.TargetUrl, id), ""),
			)
		}
		return
	}
	respStr, _ := json.Marshal(getGroupResp)
	log.Printf("[INFO] Received Group: %s", string(respStr))

	state, diags = flattenSCIMGroup(getGroupResp)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// Set state to fully populated data
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *GroupV2Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var plan *GroupV2ResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	groupUpdateReq, diags := extractGroup(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	groupStr, _ := json.Marshal(groupUpdateReq)
	log.Printf("[INFO] Updating Group: %s", string(groupStr))
	groupID := plan.ID.ValueString()
	groupUpdateResp, err := r.client.UpdateGroup(ctx, groupID, groupUpdateReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error updating Group",
			utils.FormatRpcErrors(err, fmt.Sprintf("%s/%s", r.client.TargetUrl, groupUpdateReq.ID), string(groupStr)),
		)
		return
	}
	groupStr, _ = json.Marshal(groupUpdateResp)
	log.Printf("[INFO] Submitted updated Group: %s", string(groupStr))

	// Get refreshed Group value from Coralogix
	id := plan.ID.ValueString()
	getGroupResp, err := r.client.GetGroup(ctx, id)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if cxsdk.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("Group %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError(
				"Error reading Group",
				utils.FormatRpcErrors(err, fmt.Sprintf("%s/%s", r.client.TargetUrl, id), string(groupStr)),
			)
		}
		return
	}
	groupStr, _ = json.Marshal(getGroupResp)
	log.Printf("[INFO] Received Group: %s", string(groupStr))

	state, diags := flattenSCIMGroup(getGroupResp)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// Set state to fully populated data
	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
}

func (r *GroupV2Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state *GroupV2ResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	log.Printf("[INFO] Deleting Group %s", id)
	if err := r.client.DeleteGroup(ctx, id); err != nil {
		resp.Diagnostics.AddError(
			fmt.Sprintf("Error Deleting Group %s", id),
			utils.FormatRpcErrors(err, fmt.Sprintf("%s/%s", r.client.TargetUrl, id), ""),
		)
		return
	}
	log.Printf("[INFO] Group %s deleted", id)
}

type GroupV2ResourceModel struct {
	Name           types.String `tfsdk:"name"`
	TeamId         types.String `tfsdk:"team_id"`
	Description    types.String `tfsdk:"description"`
	ExternalId     types.String `tfsdk:"external_id"`
	RoleIds        types.Set    `tfsdk:"role_ids"`
	UserIds        types.Set    `tfsdk:"user_ids"`
	ScopeFilters   types.Object `tfsdk:"scope_filters"` // ScopeFiltersModel
	NextGenScopeId types.String `tfsdk:"next_gen_scope_id"`
}

type ScopeFiltersModel struct {
}

func extractGroupV2(ctx context.Context, plan *GroupV2ResourceModel) (*cxsdk.CreateTeamGroupRequest, diag.Diagnostics) {
	teamID, err := extractTeamId(plan.TeamId)
	if err != nil {
		return nil, diag.Diagnostics{
			diag.NewErrorDiagnostic(
				"Invalid Team ID",
				fmt.Sprintf("Failed to extract team ID from %s: %s", plan.TeamId.ValueString(), err.Error()),
			),
		}
	}

	return &cxsdk.CreateTeamGroupRequest{
		Name:   plan.Name.ValueString(),
		TeamId: teamID,
	}, nil
}

func extractTeamId(teamId types.String) (*cxsdk.GroupsTeamID, error) {
	if teamId.IsNull() || teamId.IsUnknown() {
		return nil, nil
	}

	id, err := strconv.Atoi(teamId.ValueString())
	if err != nil {
		return nil, fmt.Errorf("invalid team ID: %s", teamId.ValueString())
	}

	return &cxsdk.GroupsTeamID{
		Id: uint32(id),
	}, nil
}

func extractGroupMembers(ctx context.Context, members types.Set) ([]clientset.SCIMGroupMember, diag.Diagnostics) {
	membersElements := members.Elements()
	groupMembers := make([]clientset.SCIMGroupMember, 0, len(membersElements))
	var diags diag.Diagnostics
	for _, member := range membersElements {
		val, err := member.ToTerraformValue(ctx)
		if err != nil {
			diags.AddError("Failed to convert value to Terraform", err.Error())
			continue
		}
		var str string

		if err = val.As(&str); err != nil {
			diags.AddError("Failed to convert value to string", err.Error())
			continue
		}
		groupMembers = append(groupMembers, clientset.SCIMGroupMember{Value: str})
	}
	if diags.HasError() {
		return nil, diags
	}
	return groupMembers, nil
}
