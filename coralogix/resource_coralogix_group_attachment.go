package coralogix

import (
	"context"
	"fmt"
	"log"
	"strconv"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"terraform-provider-coralogix/coralogix/clientset"
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

	id, err := strconv.Atoi(plan.GroupID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Group ID", fmt.Sprintf("Group ID must be an integer, got: %s", plan.GroupID))
		return
	}
	groupId := &cxsdk.TeamGroupID{
		Id: uint32(id),
	}

	userIds := make([]*cxsdk.UserID, 0)
	for _, userId := range plan.UserIDs {
		userIds = append(userIds, &cxsdk.UserID{Id: userId})
	}

	groupAttachmentReq := &cxsdk.AddUsersToTeamGroupRequest{
		GroupId: groupId,
		UserIds: userIds,
	}

	_, err = r.cxClientsSet.GroupGrpc().AddUsers(ctx, groupAttachmentReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to attach users to group", err.Error())
		return
	}

	state := &GroupAttachmentResourceModel{
		GroupID: plan.GroupID,
		UserIDs: plan.UserIDs,
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

	id, err := strconv.Atoi(state.GroupID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Group ID", fmt.Sprintf("Group ID must be an integer, got: %s", state.GroupID))
		return
	}

	users, diag := getGroupUsers(ctx, r.cxClientsSet.GroupGrpc(), &cxsdk.TeamGroupID{Id: uint32(id)})
	if diag != nil {
		resp.Diagnostics.Append(diag)
		return
	}

	existingUserIds := make(map[string]bool)
	for _, user := range users {
		existingUserIds[user.UserId.Id] = true
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

	membersToAdd := make([]*cxsdk.UserID, 0)
	membersToRemove := make([]*cxsdk.UserID, 0)

	for userId := range membersInPlan {
		if !existingMembers[userId] {
			membersToAdd = append(membersToAdd, &cxsdk.UserID{
				Id: userId,
			})
		}
	}
	for userId := range membersInState {
		if !membersInPlan[userId] && existingMembers[userId] {
			membersToRemove = append(membersToRemove, &cxsdk.UserID{
				Id: userId,
			})
		}
	}

	id, err := strconv.Atoi(groupId)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Group ID", fmt.Sprintf("Group ID must be an integer, got: %s", groupId))
		return
	}
	teamGroupId := &cxsdk.TeamGroupID{Id: uint32(id)}

	if len(membersToAdd) > 0 {
		addUsersToTeamGroupReq := &cxsdk.AddUsersToTeamGroupRequest{
			GroupId: teamGroupId,
			UserIds: membersToAdd,
		}
		log.Printf("[INFO] Adding users to group: %v", addUsersToTeamGroupReq.UserIds)

		_, err = r.cxClientsSet.GroupGrpc().AddUsers(ctx, addUsersToTeamGroupReq)
		if err != nil {
			resp.Diagnostics.AddError("Failed to attach users to group", err.Error())
			return
		}
	}

	if len(membersToRemove) > 0 {
		log.Printf("[INFO] Removing users from group: %v", membersToRemove)
		removeUsersFromTeamGroupReq := &cxsdk.RemoveUsersFromTeamGroupRequest{
			GroupId: teamGroupId,
			UserIds: membersToRemove,
		}
		_, err = r.cxClientsSet.GroupGrpc().RemoveUsers(ctx, removeUsersFromTeamGroupReq)
		if err != nil {
			resp.Diagnostics.AddError("Failed to detach users from group", err.Error())
			return
		}
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

	usersToRemove := make([]*cxsdk.UserID, 0)
	for _, userId := range state.UserIDs {
		usersToRemove = append(usersToRemove, &cxsdk.UserID{
			Id: userId,
		})
	}

	groupId, err := strconv.Atoi(state.GroupID)
	if err != nil {
		response.Diagnostics.AddError("Invalid Group ID", fmt.Sprintf("Group ID must be an integer, got: %s", state.GroupID))
		return
	}

	_, err = r.cxClientsSet.GroupGrpc().RemoveUsers(ctx, &cxsdk.RemoveUsersFromTeamGroupRequest{
		GroupId: &cxsdk.TeamGroupID{
			Id: uint32(groupId),
		},
		UserIds: usersToRemove,
	})
	if err != nil {
		response.Diagnostics.AddError("Failed to detach users from group", err.Error())
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
