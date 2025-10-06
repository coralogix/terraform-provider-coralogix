package provider

import (
	"context"
	"fmt"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func NewGroupAttachmentResource() resource.Resource {
	return &GroupAttachmentResource{}
}

type GroupAttachmentResource struct {
	cxClientsSet *clientset.ClientSet
}

type GroupAttachmentResourceModel struct {
	GroupID string   `tfsdk:"group_id"`
	UserIDs []string `tfsdk:"user_ids"`
}

func (r *GroupAttachmentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	r.cxClientsSet = clientSet
}

func (r *GroupAttachmentResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version: 0,
		Attributes: map[string]schema.Attribute{
			"group_id": schema.StringAttribute{
				Description: "The ID of the group to attach the users to",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"user_ids": schema.SetAttribute{
				Description: "The IDs of the users to attach to the group",
				Required:    true,
				ElementType: types.StringType,
			},
		},
	}
}

func (r *GroupAttachmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan *GroupAttachmentResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	groupId := plan.GroupID
	getGroupResp, err := r.cxClientsSet.Groups().GetGroup(ctx, groupId)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get group", err.Error())
		return
	}
	if getGroupResp == nil {
		resp.Diagnostics.AddError("Group not found", "Group not found")
		return
	}

	existingMembers := make(map[string]bool)
	for _, member := range getGroupResp.Members {
		existingMembers[member.Value] = true
	}

	newMembers := make([]clientset.SCIMGroupMember, 0)
	userIds := make([]string, 0)
	for _, memberId := range plan.UserIDs {
		if _, ok := existingMembers[memberId]; !ok {
			newMembers = append(newMembers, clientset.SCIMGroupMember{Value: memberId})
		}
		userIds = append(userIds, memberId)
	}

	groupAttachmentReq := &clientset.SCIMGroup{
		ID:          groupId,
		DisplayName: getGroupResp.DisplayName,
		Members:     append(getGroupResp.Members, newMembers...),
		Role:        getGroupResp.Role,
		ScopeID:     getGroupResp.ScopeID,
	}

	_, err = r.cxClientsSet.Groups().UpdateGroup(ctx, groupId, groupAttachmentReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to attach users to group", err.Error())
		return
	}

	state := &GroupAttachmentResourceModel{
		GroupID: getGroupResp.ID,
		UserIDs: userIds,
	}
	// Set state to fully populated data
	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
}

func (r *GroupAttachmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state *GroupAttachmentResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var userIdsConf types.Set
	if diags = req.State.GetAttribute(ctx, path.Root("user_ids"), &userIdsConf); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	confUserIds, diags := extractGroupMembersIds(ctx, userIdsConf)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	getGroupResp, err := r.cxClientsSet.Groups().GetGroup(ctx, state.GroupID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get group", err.Error())
		return
	}
	if getGroupResp == nil {
		resp.Diagnostics.AddError("Group not found", "Group not found")
		return
	}

	existingUserIds := make(map[string]bool)
	for _, member := range getGroupResp.Members {
		existingUserIds[member.Value] = true
	}

	userIds := make([]string, 0)
	for _, userId := range confUserIds {
		if _, ok := existingUserIds[userId]; ok {
			userIds = append(userIds, userId)
		}
	}

	state.UserIDs = userIds
	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
}

func (r *GroupAttachmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan *GroupAttachmentResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state *GroupAttachmentResourceModel
	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	groupId := plan.GroupID
	getGroupResp, err := r.cxClientsSet.Groups().GetGroup(ctx, groupId)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get group", err.Error())
		return
	}
	if getGroupResp == nil {
		resp.Diagnostics.AddError("Group not found", "Group not found")
		return
	}

	existingMembers := make(map[string]bool)
	for _, member := range getGroupResp.Members {
		existingMembers[member.Value] = true
	}

	membersInPlan := make(map[string]bool)
	for _, userId := range plan.UserIDs {
		membersInPlan[userId] = true
	}

	membersInState := make(map[string]bool)
	for _, userId := range state.UserIDs {
		membersInState[userId] = true
	}

	membersToApply := make([]clientset.SCIMGroupMember, 0)
	for userId := range membersInPlan {
		membersToApply = append(membersToApply, clientset.SCIMGroupMember{Value: userId})
	}
	for userId := range existingMembers {
		if _, ok := membersInState[userId]; ok {
			if _, ok := membersInPlan[userId]; !ok {
				// user is in state but not in plan
				continue
			}
		}
		membersToApply = append(membersToApply, clientset.SCIMGroupMember{Value: userId})
	}

	groupAttachmentReq := &clientset.SCIMGroup{
		ID:          groupId,
		DisplayName: getGroupResp.DisplayName,
		Members:     membersToApply,
		Role:        getGroupResp.Role,
		ScopeID:     getGroupResp.ScopeID,
	}

	_, err = r.cxClientsSet.Groups().UpdateGroup(ctx, groupId, groupAttachmentReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to attach users to group", err.Error())
		return
	}

	state = &GroupAttachmentResourceModel{
		GroupID: groupId,
		UserIDs: plan.UserIDs,
	}
	// Set state to fully populated data
	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
}

func (r *GroupAttachmentResource) Delete(ctx context.Context, request resource.DeleteRequest, response *resource.DeleteResponse) {
	var state *GroupAttachmentResourceModel
	diags := request.State.Get(ctx, &state)
	response.Diagnostics.Append(diags...)
	if response.Diagnostics.HasError() {
		return
	}

	groupId := state.GroupID
	getGroupResp, err := r.cxClientsSet.Groups().GetGroup(ctx, groupId)
	if err != nil {
		response.Diagnostics.AddError("Failed to get group", err.Error())
		return
	}
	if getGroupResp == nil {
		response.Diagnostics.AddError("Group not found", "Group not found")
		return
	}

	membersToRemove := make(map[string]bool)
	for _, userId := range state.UserIDs {
		membersToRemove[userId] = true
	}

	remainMembers := make([]clientset.SCIMGroupMember, 0)
	for _, member := range getGroupResp.Members {
		if _, ok := membersToRemove[member.Value]; !ok {
			remainMembers = append(remainMembers, member)
		}
	}

	groupAttachmentReq := &clientset.SCIMGroup{
		ID:          getGroupResp.ID,
		DisplayName: getGroupResp.DisplayName,
		Members:     remainMembers,
		Role:        getGroupResp.Role,
		ScopeID:     getGroupResp.ScopeID,
	}

	_, err = r.cxClientsSet.Groups().UpdateGroup(ctx, groupId, groupAttachmentReq)
	if err != nil {
		response.Diagnostics.AddError("Failed to attach users to group", err.Error())
		return
	}
}

func (r *GroupAttachmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group_attachment"
}

func extractGroupMembersIds(ctx context.Context, set types.Set) ([]string, diag.Diagnostics) {
	result := make([]string, 0)
	diags := diag.Diagnostics{}
	for _, v := range set.Elements() {
		val, err := v.ToTerraformValue(ctx)
		if err != nil {
			diags.AddError("Failed to convert value to Terraform", err.Error())
			continue
		}
		var str string

		if err = val.As(&str); err != nil {
			diags.AddError("Failed to convert value to string", err.Error())
			continue
		}
		result = append(result, str)
	}

	return result, nil
}
