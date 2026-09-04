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
	"time"

	"github.com/cenkalti/backoff/v5"
	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	teamGroups "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/team_groups_management_service"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func NewGroupResource() resource.Resource {
	return &GroupResource{}
}

type GroupResource struct {
	client *teamGroups.TeamGroupsManagementServiceAPIService
}

func (r *GroupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group"
}

func (r *GroupResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	r.client = clientSet.TeamGroups()
}

func (r *GroupResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
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
				Computed:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.Set{
					setplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "IDs of the users that make up the group, as the complete member list. Omit the argument to leave membership unmanaged by this resource - Terraform then reads and stores the group's current members without changing them, which is what to do when membership is maintained in the Coralogix UI or by `coralogix_group_attachment`. Set `members = []` to remove every member. A single group's membership must be managed either here or by `coralogix_group_attachment`, never by both.",
			},
			"role": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Role assigned to the group. Create and update send this name. Read stores the name the API returns.",
			},
			"scope_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Scope attached to the group.",
				Computed:            true,
			},
		},
		MarkdownDescription: "Coralogix group. Groups bind users to roles and scopes. For more info please review - https://coralogix.com/docs/user-guides/account-management/user-management/assign-user-roles-and-scopes-via-groups/.",
	}
}

func (r *GroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *GroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan *GroupResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq, diags := r.extractCreateTeamGroupRequest(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	log.Printf("[INFO] Creating new group: %s", plan.DisplayName.ValueString())
	createResp, httpResp, err := r.client.
		GroupsMgmtServiceCreateTeamGroup(ctx).
		CreateTeamGroupRequest(*createReq).
		Execute()
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating Group",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResp, err), "Create", nil),
		)
		return
	}
	if createResp == nil || createResp.Group == nil || createResp.Group.GroupId == nil {
		resp.Diagnostics.AddError("Error creating Group", "API returned an empty group")
		return
	}

	state, diags := r.readFlattenedGroupToState(ctx, *createResp.Group.GroupId, plan.ScopeID.ValueString())
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.Members = membersForState(plan.Members, state.Members)

	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
}

func (r *GroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state *GroupResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	groupID, diags := parseGroupID(state.ID.ValueString())
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	log.Printf("[INFO] Reading Group: %d", groupID)
	flattened, err := r.readFlattenedGroup(ctx, groupID, "")
	if err != nil {
		if isGroupNotFoundErr(err) {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("Group %q is in state, but no longer exists in Coralogix backend", state.ID.ValueString()),
				fmt.Sprintf("%s will be recreated when you apply", state.ID.ValueString()),
			)
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading Group", err.Error())
		return
	}

	diags = resp.State.Set(ctx, flattened)
	resp.Diagnostics.Append(diags...)
}

func (r *GroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan *GroupResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var config *GroupResourceModel
	diags = req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	groupID, diags := parseGroupID(plan.ID.ValueString())
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq, diags := r.extractUpdateTeamGroupRequest(ctx, plan, config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	log.Printf("[INFO] Updating Group: %d", groupID)
	_, httpResp, err := r.client.
		GroupsMgmtServiceUpdateTeamGroup(ctx, groupID).
		UpdateTeamGroupRequest(*updateReq).
		Execute()
	if err != nil {
		apiErr := cxsdkOpenapi.NewAPIError(httpResp, err)
		if cxsdkOpenapi.IsNotFound(apiErr) {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("Group %q is in state, but no longer exists in Coralogix backend", plan.ID.ValueString()),
				fmt.Sprintf("%s will be recreated when you apply", plan.ID.ValueString()),
			)
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error updating Group",
			utils.FormatOpenAPIErrors(apiErr, "Update", nil),
		)
		return
	}

	state, err := r.readFlattenedGroup(ctx, groupID, plan.ScopeID.ValueString())
	if err != nil {
		if isGroupNotFoundErr(err) {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("Group %q is in state, but no longer exists in Coralogix backend", plan.ID.ValueString()),
				fmt.Sprintf("%s will be recreated when you apply", plan.ID.ValueString()),
			)
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading Group", err.Error())
		return
	}
	state.Members = membersForState(plan.Members, state.Members)

	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
}

func (r *GroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state *GroupResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	groupID, diags := parseGroupID(state.ID.ValueString())
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	log.Printf("[INFO] Deleting Group %d", groupID)
	_, httpResp, err := r.client.GroupsMgmtServiceDeleteTeamGroup(ctx, groupID).Execute()
	if err != nil {
		apiErr := cxsdkOpenapi.NewAPIError(httpResp, err)
		if cxsdkOpenapi.IsNotFound(apiErr) {
			return
		}
		resp.Diagnostics.AddError(
			fmt.Sprintf("Error Deleting Group %s", state.ID.ValueString()),
			utils.FormatOpenAPIErrors(apiErr, "Delete", nil),
		)
		return
	}
}

type GroupResourceModel struct {
	ID          types.String `tfsdk:"id"`
	DisplayName types.String `tfsdk:"display_name"`
	Members     types.Set    `tfsdk:"members"`
	Role        types.String `tfsdk:"role"`
	ScopeID     types.String `tfsdk:"scope_id"`
}

func (r *GroupResource) extractCreateTeamGroupRequest(ctx context.Context, plan *GroupResourceModel) (*teamGroups.CreateTeamGroupRequest, diag.Diagnostics) {
	var diags diag.Diagnostics

	userIDs, memberDiags := extractMemberIDs(ctx, plan.Members)
	diags.Append(memberDiags...)
	if diags.HasError() {
		return nil, diags
	}

	createReq := &teamGroups.CreateTeamGroupRequest{
		Name:     teamGroups.PtrString(plan.DisplayName.ValueString()),
		RoleName: teamGroups.PtrString(plan.Role.ValueString()),
		UserIds:  userIDs,
	}
	if !plan.ScopeID.IsNull() && !plan.ScopeID.IsUnknown() && plan.ScopeID.ValueString() != "" {
		createReq.Scope = &teamGroups.V2Scope{ScopeId: teamGroups.PtrString(plan.ScopeID.ValueString())}
	}
	return createReq, diags
}

func (r *GroupResource) extractUpdateTeamGroupRequest(ctx context.Context, plan, config *GroupResourceModel) (*teamGroups.UpdateTeamGroupRequest, diag.Diagnostics) {
	var diags diag.Diagnostics

	updateReq := &teamGroups.UpdateTeamGroupRequest{
		Name:       teamGroups.PtrString(plan.DisplayName.ValueString()),
		RoleUpdate: teamGroupRoleUpdateByName(plan.Role.ValueString()),
	}
	if !membersUnmanaged(config.Members) {
		userIDs, memberDiags := extractMemberIDs(ctx, plan.Members)
		diags.Append(memberDiags...)
		if diags.HasError() {
			return nil, diags
		}
		updateReq.UserUpdates = teamGroupUserUpdates("set", userIDs)
	}
	updateReq.ScopeUpdate = scopeUpdateFromPlan(plan)
	return updateReq, diags
}

func scopeUpdateFromPlan(plan *GroupResourceModel) *teamGroups.ScopeUpdate {
	if plan.ScopeID.IsUnknown() || plan.ScopeID.IsNull() || plan.ScopeID.ValueString() == "" {
		return nil
	}
	return teamGroupScopeSet(plan.ScopeID.ValueString())
}

func (r *GroupResource) readFlattenedGroupToState(ctx context.Context, groupID int64, expectedScopeID string) (*GroupResourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	state, err := r.readFlattenedGroup(ctx, groupID, expectedScopeID)
	if err != nil {
		diags.AddError("Error reading Group", err.Error())
		return nil, diags
	}
	return state, diags
}

func (r *GroupResource) readFlattenedGroup(ctx context.Context, groupID int64, expectedScopeID string) (*GroupResourceModel, error) {
	group, err := r.getGroupWithScopeRetry(ctx, groupID, expectedScopeID)
	if err != nil {
		return nil, err
	}

	memberIDs, httpResp, err := listGroupUserIDs(ctx, r.client, groupID)
	if err != nil {
		return nil, formatGroupReadError(httpResp, err, groupID)
	}

	state, diags := flattenTeamGroup(group, memberIDs)
	if diags.HasError() {
		return nil, fmt.Errorf("%s", diags[0].Detail())
	}
	return state, nil
}

func (r *GroupResource) getGroupWithScopeRetry(ctx context.Context, groupID int64, expectedScopeID string) (*teamGroups.TeamGroup, error) {
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = time.Second
	b.MaxInterval = 3 * time.Second

	op := func() (*teamGroups.TeamGroup, error) {
		resp, httpResp, err := r.client.GroupsMgmtServiceGetTeamGroup(ctx, groupID).Execute()
		if err != nil {
			return nil, backoff.Permanent(formatGroupReadError(httpResp, err, groupID))
		}
		if resp == nil || resp.Group == nil {
			return nil, fmt.Errorf("API returned an empty group")
		}
		if expectedScopeID != "" && (resp.Group.Scope == nil || resp.Group.Scope.GetScopeId() == "") {
			log.Printf("[INFO] Group %d scope_id not yet visible (eventual consistency), retrying", groupID)
			return nil, fmt.Errorf("scope_id not yet visible")
		}
		return resp.Group, nil
	}

	return backoff.Retry(ctx, op,
		backoff.WithBackOff(b),
		backoff.WithMaxTries(5),
		backoff.WithMaxElapsedTime(10*time.Second),
	)
}

func formatGroupReadError(httpResp *http.Response, err error, groupID int64) error {
	apiErr := cxsdkOpenapi.NewAPIError(httpResp, err)
	if cxsdkOpenapi.IsNotFound(apiErr) {
		return &groupNotFoundError{id: groupID}
	}
	return fmt.Errorf("%s", utils.FormatOpenAPIErrors(apiErr, "Read", groupID))
}
