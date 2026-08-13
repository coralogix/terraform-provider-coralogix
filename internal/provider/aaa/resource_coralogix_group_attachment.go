package aaa

import (
	"context"
	"fmt"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	teamGroups "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/team_groups_management_service"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

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
	client *teamGroups.TeamGroupsManagementServiceAPIService
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

	r.client = clientSet.TeamGroups()
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
		MarkdownDescription: "Coralogix group attachment. Attaches a set of users to a Coralogix group. For more info please review - https://coralogix.com/docs/user-guides/account-management/user-management/assign-user-roles-and-scopes-via-groups/.",
	}
}

func (r *GroupAttachmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan *GroupAttachmentResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	groupID, diags := parseGroupID(plan.GroupID)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	existing, err := r.listMembers(ctx, groupID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get group", err.Error())
		return
	}

	toAdd := userIDsToAdd(plan.UserIDs, existing)
	if err := r.applyUserOp(ctx, groupID, "add", toAdd); err != nil {
		resp.Diagnostics.AddError("Failed to attach users to group", err.Error())
		return
	}

	state := &GroupAttachmentResourceModel{
		GroupID: plan.GroupID,
		UserIDs: plan.UserIDs,
	}
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

	groupID, diags := parseGroupID(state.GroupID)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	existing, err := r.listMembers(ctx, groupID)
	if err != nil {
		if isGroupNotFoundErr(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to get group", err.Error())
		return
	}

	existingSet := userIDSet(existing)
	userIds := make([]string, 0)
	for _, userId := range confUserIds {
		if _, ok := existingSet[userId]; ok {
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

	groupID, diags := parseGroupID(plan.GroupID)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	existing, err := r.listMembers(ctx, groupID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get group", err.Error())
		return
	}

	diffAdd, diffRemove := userIDDiff(plan.UserIDs, state.UserIDs)
	if err := r.applyUserOp(ctx, groupID, "remove", userIDsToRemove(diffRemove, existing)); err != nil {
		resp.Diagnostics.AddError("Failed to attach users to group", err.Error())
		return
	}
	if err := r.applyUserOp(ctx, groupID, "add", userIDsToAdd(diffAdd, existing)); err != nil {
		resp.Diagnostics.AddError("Failed to attach users to group", err.Error())
		return
	}

	state = &GroupAttachmentResourceModel{
		GroupID: plan.GroupID,
		UserIDs: plan.UserIDs,
	}
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

	groupID, diags := parseGroupID(state.GroupID)
	response.Diagnostics.Append(diags...)
	if response.Diagnostics.HasError() {
		return
	}

	existing, err := r.listMembers(ctx, groupID)
	if err != nil {
		if isGroupNotFoundErr(err) {
			return
		}
		response.Diagnostics.AddError("Failed to get group", err.Error())
		return
	}

	if err := r.applyUserOp(ctx, groupID, "remove", userIDsToRemove(state.UserIDs, existing)); err != nil {
		response.Diagnostics.AddError("Failed to attach users to group", err.Error())
		return
	}
}

func (r *GroupAttachmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group_attachment"
}

func (r *GroupAttachmentResource) listMembers(ctx context.Context, groupID int64) ([]string, error) {
	ids, httpResp, err := listGroupUserIDs(ctx, r.client, groupID)
	if err != nil {
		apiErr := cxsdkOpenapi.NewAPIError(httpResp, err)
		if cxsdkOpenapi.IsNotFound(apiErr) {
			return nil, &groupNotFoundError{id: groupID}
		}
		return nil, fmt.Errorf("%s", utils.FormatOpenAPIErrors(apiErr, "GetGroupUsers", groupID))
	}
	return ids, nil
}

func (r *GroupAttachmentResource) applyUserOp(ctx context.Context, groupID int64, operationType string, userIDs []string) error {
	httpResp, err := applyGroupUserOperation(ctx, r.client, groupID, operationType, userIDs)
	if err != nil {
		return fmt.Errorf("%s", utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResp, err), "Update", groupID))
	}
	return nil
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

	return result, diags
}
