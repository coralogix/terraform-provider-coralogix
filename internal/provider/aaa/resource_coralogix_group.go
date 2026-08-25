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
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/cenkalti/backoff/v5"
	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/terraform-plugin-framework/attr"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func NewGroupResource() resource.Resource {
	return &GroupResource{}
}

type GroupResource struct {
	client *clientset.GroupsClient
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

	r.client = clientSet.Groups()
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
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "IDs of the users that make up the group. Omit the argument to leave membership unmanaged by this resource - for example when it is maintained in the Coralogix UI or by `coralogix_group_attachment`. Set `members = []` to remove every member. A single group's membership must be managed either here or by `coralogix_group_attachment`, never by both.",
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

	createGroupRequest, diags := extractGroup(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	groupStr, _ := json.Marshal(createGroupRequest)
	log.Printf("[INFO] Creating new group: %s", string(groupStr))
	createResp, err := r.client.CreateGroup(ctx, createGroupRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error creating Group",
			utils.FormatRpcErrors(err, r.client.TargetUrl, string(groupStr)),
		)
		return
	}

	// Retry GetGroup with backoff when we set scope_id: API may have eventual consistency
	// and not return nextGenScopeId on the first read after create.
	getResp, err := r.getGroupWithScopeRetry(ctx, createResp.ID, plan.ScopeID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading group",
			utils.FormatRpcErrors(err, r.client.TargetUrl, createResp.ID),
		)
		return
	}

	groupStr, _ = json.Marshal(getResp)
	log.Printf("[INFO] Getting group: %s", groupStr)
	state, diags := flattenSCIMGroup(getResp)

	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	state.Members = membersMatchingPrior(plan.Members, state.Members)

	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
}

// getGroupWithScopeRetry fetches the group and, when expectedScopeID is set,
// retries with backoff until the API returns the scope (handles eventual consistency).
func (r *GroupResource) getGroupWithScopeRetry(ctx context.Context, groupID, expectedScopeID string) (*clientset.SCIMGroup, error) {
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = time.Second
	b.MaxInterval = 3 * time.Second

	op := func() (*clientset.SCIMGroup, error) {
		resp, err := r.client.GetGroup(ctx, groupID)
		if err != nil {
			return nil, err
		}
		if expectedScopeID != "" && resp.ScopeID == "" {
			log.Printf("[INFO] Group %s scope_id not yet visible (eventual consistency), retrying", groupID)
			return nil, fmt.Errorf("scope_id not yet visible")
		}
		return resp, nil
	}

	return backoff.Retry(ctx, op,
		backoff.WithBackOff(b),
		backoff.WithMaxTries(5),
		backoff.WithMaxElapsedTime(10*time.Second),
	)
}

func flattenSCIMGroup(group *clientset.SCIMGroup) (*GroupResourceModel, diag.Diagnostics) {
	members, diags := flattenSCIMGroupMembers(group.Members)
	if diags.HasError() {
		return nil, diags
	}

	scopeId := types.StringNull()
	if group.ScopeID != "" {
		scopeId = types.StringValue(group.ScopeID)
	}

	return &GroupResourceModel{
		ID:          types.StringValue(group.ID),
		DisplayName: types.StringValue(group.DisplayName),
		Members:     members,
		Role:        types.StringValue(group.Role),
		ScopeID:     scopeId,
	}, nil
}

func flattenSCIMGroupMembers(members []clientset.SCIMGroupMember) (types.Set, diag.Diagnostics) {
	if len(members) == 0 {
		return types.SetNull(types.StringType), nil
	}
	var diags diag.Diagnostics
	membersIDs := make([]attr.Value, 0, len(members))
	for _, member := range members {
		membersIDs = append(membersIDs, types.StringValue(member.Value))
	}
	if diags.HasError() {
		return types.SetNull(types.StringType), diags
	}

	return types.SetValue(types.StringType, membersIDs)
}

// membersMatchingPrior keeps the members written to state null-consistent with the prior
// value it is reconciled against - the plan in Create and Update, the previous state in Read.
// A null prior means membership is not managed here; an empty one must not collapse to null.
func membersMatchingPrior(prior, flattened types.Set) types.Set {
	if prior.IsNull() {
		return types.SetNull(types.StringType)
	}
	if flattened.IsNull() {
		return types.SetValueMust(types.StringType, []attr.Value{})
	}
	return flattened
}

func (r *GroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state *GroupResourceModel
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
		if status.Code(err) == codes.NotFound {
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

	newState, diags := flattenSCIMGroup(getGroupResp)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// A state without a display name comes from an import and is hydrated as-is; otherwise membership
	// absent from state is managed elsewhere and is not adopted on refresh.
	if !state.DisplayName.IsNull() {
		newState.Members = membersMatchingPrior(state.Members, newState.Members)
	}

	diags = resp.State.Set(ctx, newState)
	resp.Diagnostics.Append(diags...)
}

func (r *GroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var plan *GroupResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state *GroupResourceModel
	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	groupUpdateReq, diags := extractGroup(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	groupID := plan.ID.ValueString()

	// Membership absent from both configuration and state is managed elsewhere, and the SCIM update is a full replace.
	if plan.Members.IsNull() && state.Members.IsNull() {
		currentGroup, err := r.client.GetGroup(ctx, groupID)
		if err != nil {
			log.Printf("[ERROR] Received error: %s", err.Error())
			resp.Diagnostics.AddError(
				"Error reading Group",
				utils.FormatRpcErrors(err, fmt.Sprintf("%s/%s", r.client.TargetUrl, groupID), ""),
			)
			return
		}
		groupUpdateReq.Members = currentGroup.Members
	}

	groupStr, _ := json.Marshal(groupUpdateReq)
	log.Printf("[INFO] Updating Group: %s", string(groupStr))
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
		if status.Code(err) == codes.NotFound {
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

	newState, diags := flattenSCIMGroup(getGroupResp)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	newState.Members = membersMatchingPrior(plan.Members, newState.Members)

	diags = resp.State.Set(ctx, newState)
	resp.Diagnostics.Append(diags...)
}

func (r *GroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state *GroupResourceModel
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

type GroupResourceModel struct {
	ID          types.String `tfsdk:"id"`
	DisplayName types.String `tfsdk:"display_name"`
	Members     types.Set    `tfsdk:"members"` // Set of strings
	Role        types.String `tfsdk:"role"`
	ScopeID     types.String `tfsdk:"scope_id"`
}

func extractGroup(ctx context.Context, plan *GroupResourceModel) (*clientset.SCIMGroup, diag.Diagnostics) {
	members, diags := extractGroupMembers(ctx, plan.Members)
	if diags.HasError() {
		return nil, diags
	}

	return &clientset.SCIMGroup{
		DisplayName: plan.DisplayName.ValueString(),
		Members:     members,
		Role:        plan.Role.ValueString(),
		ScopeID:     plan.ScopeID.ValueString(),
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
