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
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"google.golang.org/protobuf/encoding/protojson"
	"terraform-provider-coralogix/coralogix/clientset"
	"terraform-provider-coralogix/coralogix/utils"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/grpc/codes"
)

var (
	FilterTypeSchemaToProto = map[string]cxsdk.FilterType{
		"starts_with": cxsdk.FilterTypeStartsWith,
		"ends_with":   cxsdk.FilterTypeEndsWith,
		"contains":    cxsdk.FilterTypeContains,
		"exact":       cxsdk.FilterTypeExact,
	}
	FilterTypeProtoToSchema = utils.ReverseMap(FilterTypeSchemaToProto)
	validFilterTypes        = utils.GetKeys(FilterTypeSchemaToProto)
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
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Group name.",
			},
			"team_id": schema.StringAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "Team ID to which the group belongs.",
			},
			"description": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(""),
				MarkdownDescription: "Group description.",
			},
			"external_id": schema.StringAttribute{
				Optional: true,
			},
			"roles": schema.SetNestedAttribute{
				Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":          schema.StringAttribute{Required: true},
						"name":        schema.StringAttribute{Computed: true},
						"description": schema.StringAttribute{Computed: true},
					},
				},
			},
			"users": schema.SetNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":              schema.StringAttribute{Computed: true},
						"user_name":       schema.StringAttribute{Computed: true},
						"user_account_id": schema.StringAttribute{Computed: true},
						"first_name":      schema.StringAttribute{Computed: true},
						"last_name":       schema.StringAttribute{Computed: true},
					},
				},
				MarkdownDescription: "Set of users in the group. This is a computed attribute and will be populated after the group is created or updated. For managing the group's users use the `coralogix_group_attachment` resource.",
			},
			"scope": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"id": schema.StringAttribute{
						Computed: true,
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.UseStateForUnknown(),
						},
					},
					"filters": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"subsystems":   ScopeFilterSchema(),
							"applications": ScopeFilterSchema(),
						},
					},
				},
				MarkdownDescription: "Group scope. If not provided, the group will not have a scope.",
			},
			"next_gen_scope_id": schema.StringAttribute{
				Optional: true,
				Computed: true,
			},
		},
		MarkdownDescription: "Coralogix group.",
	}
}

func ScopeFilterSchema() schema.SetNestedAttribute {
	return schema.SetNestedAttribute{
		Optional: true,
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"term": schema.StringAttribute{
					Required: true,
				},
				"filter_type": schema.StringAttribute{
					Required: true,
					Validators: []validator.String{
						stringvalidator.OneOf(validFilterTypes...),
					},
					MarkdownDescription: "Filter type for the scope filter. Valid values are: " + strings.Join(validFilterTypes, ", "),
				},
			},
		},
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

	createGroupRequest, diags := extractCreateGroupV2(ctx, plan)
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

	users, diag := getGroupUsers(ctx, r.client, createResp.GroupId)
	if diag != nil {
		resp.Diagnostics.Append(diag)
		return
	}

	state, diags := flattenGroupV2(ctx, getResp.Group, users)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// Set state to fully populated data
	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
}

func (r *GroupV2Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state *GroupV2ResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	//Get refreshed Group value from Coralogix
	id, err := strconv.Atoi(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid Group ID",
			fmt.Sprintf("Failed to convert group ID %s to integer: %s", state.ID.ValueString(), err.Error()),
		)
		return
	}

	teamGroupId := &cxsdk.TeamGroupID{
		Id: uint32(id),
	}
	getGroupReq := &cxsdk.GetTeamGroupRequest{
		GroupId: teamGroupId,
	}
	log.Printf("[INFO] Reading Group: %d", id)
	getGroupResp, err := r.client.Get(ctx, getGroupReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if cxsdk.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("Group %d is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("Group %d will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError(
				"Error reading Group",
				utils.FormatRpcErrors(err, cxsdk.GetTeamGroupRPC, protojson.Format(getGroupReq)),
			)
		}
		return
	}

	respStr, _ := json.Marshal(getGroupResp)
	log.Printf("[INFO] Received Group: %s", string(respStr))

	users, diag := getGroupUsers(ctx, r.client, getGroupResp.Group.GroupId)
	if diag != nil {
		resp.Diagnostics.Append(diag)
		return
	}

	state, diags = flattenGroupV2(ctx, getGroupResp.Group, users)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// Set state to fully populated data
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func getGroupUsers(ctx context.Context, groupsClient *cxsdk.GroupsClient, teamGroupId *cxsdk.TeamGroupID) ([]*cxsdk.GroupsUser, diag.Diagnostic) {
	var users []*cxsdk.GroupsUser
	getGroupUsersReq := &cxsdk.GetGroupUsersRequest{
		GroupId: teamGroupId,
	}

	for {
		getUsersResp, err := groupsClient.GetUsers(ctx, getGroupUsersReq)
		if err != nil {
			log.Printf("[ERROR] Received error: %s", err.Error())
			return nil, diag.NewErrorDiagnostic(
				"Error getting group users",
				utils.FormatRpcErrors(err, cxsdk.GetGroupUsersRPC, protojson.Format(getGroupUsersReq)),
			)
		}
		users = append(users, getUsersResp.GetUsers()...)
		switch nextPage := getUsersResp.GetNextPage().(type) {
		case *cxsdk.GetGroupUsersResponseNoMorePages:
			return users, nil
		case *cxsdk.GetGroupUsersResponseToken:
			getGroupUsersReq.PageToken = &nextPage.Token.NextPageToken
			// continue loop to fetch next page
			continue
		}
	}
}

func (r *GroupV2Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var plan *GroupV2ResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	groupUpdateReq, diags := extractUpdateGroupV2(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	groupStr, _ := json.Marshal(groupUpdateReq)
	log.Printf("[INFO] Updating Group: %s", string(groupStr))
	// Set the ID for the update request

	_, err := r.client.Update(ctx, groupUpdateReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if cxsdk.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(
				err.Error(),
				fmt.Sprintf("Group %q is in state, but no longer exists in Coralogix backend", plan.ID.ValueString()),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError(
				"Error updating Group",
				utils.FormatRpcErrors(err, cxsdk.UpdateTeamGroupRPC, string(groupStr)),
			)
		}
		return
	}

	getResp, err := r.client.Get(ctx, &cxsdk.GetTeamGroupRequest{GroupId: groupUpdateReq.GroupId})
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if cxsdk.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(
				err.Error(),
				fmt.Sprintf("Group %q is in state, but no longer exists in Coralogix backend", plan.ID.ValueString()),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError(
				"Error reading Group",
				utils.FormatRpcErrors(err, "Get", fmt.Sprintf("Group ID: %s", plan.ID.ValueString())),
			)
			resp.Diagnostics.AddError("Failed to get group", err.Error())
		}
		return
	}

	groupStr, _ = json.Marshal(getResp)
	log.Printf("[INFO] Getting group: %s", groupStr)

	users, diag := getGroupUsers(ctx, r.client, groupUpdateReq.GroupId)
	if diag != nil {
		resp.Diagnostics.Append(diag)
		return
	}

	state, diags := flattenGroupV2(ctx, getResp.Group, users)
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

	id, err := strconv.Atoi(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid Group ID",
			fmt.Sprintf("Failed to convert group ID %s to integer: %s", state.ID.ValueString(), err.Error()),
		)
		return
	}
	log.Printf("[INFO] Deleting Group %d", id)
	if _, err = r.client.Delete(ctx, &cxsdk.DeleteTeamGroupRequest{GroupId: &cxsdk.TeamGroupID{Id: uint32(id)}}); err != nil {
		if cxsdk.Code(err) == codes.NotFound {
			log.Printf("[WARN] Group %d not found, assuming it has already been deleted", id)
			return
		}
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error deleting Group",
			utils.FormatRpcErrors(err, cxsdk.DeleteTeamGroupRPC, fmt.Sprintf("Group ID: %d", id)),
		)
	}
	log.Printf("[INFO] Group %d deleted", id)
}

func flattenGroupV2(ctx context.Context, group *cxsdk.TeamGroup, users []*cxsdk.GroupsUser) (*GroupV2ResourceModel, diag.Diagnostics) {
	roles, diags := flattenRoles(ctx, group.Roles)
	if diags.HasError() {
		return nil, diags
	}

	scopes, diags := flattenScopes(ctx, group.Scope)
	if diags.HasError() {
		return nil, diags
	}

	usersSet, diags := flattenGroupUsers(ctx, users)
	if diags.HasError() {
		return nil, diags
	}

	state := &GroupV2ResourceModel{
		ID:             flattenGroupId(group.GroupId),
		Name:           types.StringValue(group.Name),
		TeamId:         flattenTeamId(group.TeamId),
		Description:    utils.StringPointerToTypeString(group.Description),
		ExternalId:     utils.StringPointerToTypeString(group.ExternalId),
		Roles:          roles,
		Users:          usersSet,
		Scope:          scopes,
		NextGenScopeId: utils.StringPointerToTypeString(group.NextGenScopeId),
	}

	return state, nil
}

func flattenGroupUsers(ctx context.Context, users []*cxsdk.GroupsUser) (types.Set, diag.Diagnostics) {
	if users == nil {
		return types.SetNull(types.ObjectType{AttrTypes: UserAttrs()}), nil
	}
	// Flatten the users into a slice of UserModel
	flattenedUsers := make([]*UserModel, 0, len(users))
	for _, user := range users {
		flattenedUsers = append(flattenedUsers, &UserModel{
			ID:            types.StringValue(user.UserId.Id),
			Username:      types.StringValue(user.Username),
			UserAccountId: types.StringValue(strconv.Itoa(int(user.UserAccountId.Id))),
			FirstName:     types.StringValue(user.FirstName),
			LastName:      types.StringValue(user.LastName),
		})
	}

	return types.SetValueFrom(ctx, types.ObjectType{AttrTypes: UserAttrs()}, flattenedUsers)
}

func flattenRoles(ctx context.Context, roles []*cxsdk.Role) (types.Set, diag.Diagnostics) {
	if roles == nil {
		return types.SetNull(types.ObjectType{AttrTypes: RolesAttrs()}), nil
	}

	flattenedRoles := make([]*GroupRolesModel, 0, len(roles))
	for _, role := range roles {
		flattenedRoles = append(flattenedRoles, &GroupRolesModel{
			ID:          types.StringValue(strconv.Itoa(int(role.RoleId.Id))),
			Name:        types.StringValue(role.Name),
			Description: types.StringValue(role.Description),
		})
	}

	return types.SetValueFrom(ctx, types.ObjectType{AttrTypes: RolesAttrs()}, flattenedRoles)
}

func flattenScopes(ctx context.Context, scope *cxsdk.GroupScope) (types.Object, diag.Diagnostics) {
	if scope == nil {
		return types.ObjectNull(ScopeAttrs()), nil
	}

	filters, diags := flattenGroupScopeFilters(ctx, scope.Filters)
	if diags.HasError() {
		return types.ObjectNull(ScopeAttrs()), diags
	}

	scopeModel := ScopeModel{
		ID:      flattenScopeId(scope.Id),
		Filters: filters,
	}

	return types.ObjectValueFrom(ctx, ScopeAttrs(), scopeModel)
}

func flattenGroupScopeFilters(ctx context.Context, filters *cxsdk.ScopeFilters) (types.Object, diag.Diagnostics) {
	if filters == nil {
		return types.ObjectNull(ScopeFiltersAttrs()), nil
	}

	subsystems, diags := flattenGroupScopeFiltersList(ctx, filters.Subsystems)
	if diags.HasError() {
		return types.ObjectNull(ScopeFiltersAttrs()), diags
	}

	applications, diags := flattenGroupScopeFiltersList(ctx, filters.Applications)
	if diags.HasError() {
		return types.ObjectNull(ScopeFiltersAttrs()), diags
	}

	scopeFiltersModel := ScopeFiltersModel{
		Subsystems:   subsystems,
		Applications: applications,
	}

	return types.ObjectValueFrom(ctx, ScopeFiltersAttrs(), scopeFiltersModel)
}

func flattenGroupScopeFiltersList(ctx context.Context, filters []*cxsdk.GroupScopeFilter) (types.Set, diag.Diagnostics) {
	if filters == nil {
		return types.SetNull(types.ObjectType{AttrTypes: GroupScopeFilterAttrs()}), nil
	}

	flattenedFilters := make([]GroupScopeFilterModel, 0, len(filters))
	for _, filter := range filters {
		flattenedFilters = append(flattenedFilters, GroupScopeFilterModel{
			Term:       types.StringValue(filter.Term),
			FilterType: types.StringValue(FilterTypeProtoToSchema[filter.FilterType]),
		})
	}

	return types.SetValueFrom(ctx, types.ObjectType{AttrTypes: GroupScopeFilterAttrs()}, flattenedFilters)
}

func flattenGroupId(id *cxsdk.TeamGroupID) types.String {
	if id == nil {
		return types.StringNull()
	}

	return types.StringValue(strconv.Itoa(int(id.Id)))
}

func flattenTeamId(id *cxsdk.GroupsTeamID) types.String {
	if id == nil {
		return types.StringNull()
	}

	return types.StringValue(strconv.Itoa(int(id.Id)))
}

func flattenScopeId(id *cxsdk.ScopeID) types.String {
	if id == nil {
		return types.StringNull()
	}

	return types.StringValue(strconv.Itoa(int(id.Id)))

}

type GroupV2ResourceModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	TeamId         types.String `tfsdk:"team_id"`
	Description    types.String `tfsdk:"description"`
	ExternalId     types.String `tfsdk:"external_id"`
	Roles          types.Set    `tfsdk:"roles"` // GroupRolesModel
	Users          types.Set    `tfsdk:"users"` // UserModel
	Scope          types.Object `tfsdk:"scope"` // ScopeModel
	NextGenScopeId types.String `tfsdk:"next_gen_scope_id"`
}

type GroupRolesModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
}

func RolesAttrs() map[string]attr.Type {
	return map[string]attr.Type{
		"id":          types.StringType,
		"name":        types.StringType,
		"description": types.StringType,
	}
}

type ScopeModel struct {
	ID      types.String `tfsdk:"id"`
	Filters types.Object `tfsdk:"filters"` // ScopeFiltersModel
}

func ScopeAttrs() map[string]attr.Type {
	return map[string]attr.Type{
		"id": types.StringType,
		"filters": types.ObjectType{
			AttrTypes: ScopeFiltersAttrs(),
		},
	}
}

func ScopeFiltersAttrs() map[string]attr.Type {
	return map[string]attr.Type{
		"subsystems":   types.SetType{ElemType: types.ObjectType{AttrTypes: GroupScopeFilterAttrs()}},
		"applications": types.SetType{ElemType: types.ObjectType{AttrTypes: GroupScopeFilterAttrs()}},
	}
}

type ScopeFiltersModel struct {
	Subsystems   types.Set `tfsdk:"subsystems"`   // GroupScopeFilterModel
	Applications types.Set `tfsdk:"applications"` // GroupScopeFilterModel
}

type GroupScopeFilterModel struct {
	Term       types.String `tfsdk:"term"`
	FilterType types.String `tfsdk:"filter_type"`
}

func GroupScopeFilterAttrs() map[string]attr.Type {
	return map[string]attr.Type{
		"term":        types.StringType,
		"filter_type": types.StringType,
	}
}

type UserModel struct {
	ID            types.String `tfsdk:"id"`              // User ID
	Username      types.String `tfsdk:"user_name"`       // User name
	UserAccountId types.String `tfsdk:"user_account_id"` // User account ID
	FirstName     types.String `tfsdk:"first_name"`      // User first name
	LastName      types.String `tfsdk:"last_name"`       // User last name
}

func UserAttrs() map[string]attr.Type {
	return map[string]attr.Type{
		"id":              types.StringType,
		"user_name":       types.StringType,
		"user_account_id": types.StringType,
		"first_name":      types.StringType,
		"last_name":       types.StringType,
	}
}

func extractCreateGroupV2(ctx context.Context, plan *GroupV2ResourceModel) (*cxsdk.CreateTeamGroupRequest, diag.Diagnostics) {
	teamID, err := extractTeamId(plan.TeamId)
	if err != nil {
		return nil, diag.Diagnostics{
			diag.NewErrorDiagnostic(
				"Invalid Team ID",
				fmt.Sprintf("Failed to extract team ID from %s: %s", plan.TeamId.ValueString(), err.Error()),
			),
		}
	}

	roleIds, diags := extractRoleIds(ctx, plan.Roles)
	if diags.HasError() {
		return nil, diags
	}

	//userIds, diags := extractUserIds(plan.Users)
	//if diags.HasError() {
	//	return nil, diags
	//}

	scopeFilters, diags := extractScope(ctx, plan.Scope)

	return &cxsdk.CreateTeamGroupRequest{
		Name:        plan.Name.ValueString(),
		TeamId:      teamID,
		Description: utils.TypeStringToStringPointer(plan.Description),
		ExternalId:  utils.TypeStringToStringPointer(plan.ExternalId),
		RoleIds:     roleIds,
		//UserIds:      userIds,
		ScopeFilters:   scopeFilters,
		NextGenScopeId: utils.TypeStringToStringPointer(plan.NextGenScopeId),
	}, nil
}

func extractUpdateGroupV2(ctx context.Context, plan *GroupV2ResourceModel) (*cxsdk.UpdateTeamGroupRequest, diag.Diagnostics) {
	groupId, err := extractGroupId(plan.ID)
	if err != nil {
		return nil, diag.Diagnostics{
			diag.NewErrorDiagnostic(
				"Invalid Team ID",
				fmt.Sprintf("Failed to extract team ID from %s: %s", plan.TeamId.ValueString(), err.Error()),
			),
		}
	}

	roleIds, diags := extractRoleIds(ctx, plan.Roles)
	if diags.HasError() {
		return nil, diags
	}

	//userIds, diags := extractUserIds(plan.Users)
	//if diags.HasError() {
	//	return nil, diags
	//}

	scopeFilters, diags := extractScope(ctx, plan.Scope)

	return &cxsdk.UpdateTeamGroupRequest{
		GroupId:     groupId,
		Name:        plan.Name.ValueString(),
		Description: utils.TypeStringToStringPointer(plan.Description),
		ExternalId:  utils.TypeStringToStringPointer(plan.ExternalId),
		RoleUpdates: &cxsdk.UpdateTeamGroupRequestRoleUpdates{RoleIds: roleIds},
		//UserUpdates:  &cxsdk.UpdateTeamGroupRequestUserUpdates{UserIds: userIds},
		ScopeFilters:   scopeFilters,
		NextGenScopeId: utils.TypeStringToStringPointer(plan.NextGenScopeId),
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

func extractGroupId(groupId types.String) (*cxsdk.TeamGroupID, error) {
	if groupId.IsNull() || groupId.IsUnknown() {
		return nil, nil
	}

	id, err := strconv.Atoi(groupId.ValueString())
	if err != nil {
		return nil, fmt.Errorf("invalid group ID: %s", groupId.ValueString())
	}

	return &cxsdk.TeamGroupID{
		Id: uint32(id),
	}, nil
}

func extractRoleIds(ctx context.Context, rolesIds types.Set) ([]*cxsdk.RoleID, diag.Diagnostics) {
	var diags diag.Diagnostics
	if rolesIds.IsNull() || rolesIds.IsUnknown() {
		return nil, diags
	}

	roleElements := rolesIds.Elements()
	roleSet := make([]*cxsdk.RoleID, 0, len(roleElements))
	for _, roleElement := range roleElements {
		role, ok := roleElement.(types.Object)
		if !ok {
			diags.AddError(
				"Invalid Role ID",
				fmt.Sprintf("Expected role ID to be of type object, got: %T", roleElement),
			)
			return nil, diags
		}

		var roleModel GroupRolesModel
		if roleDiags := role.As(ctx, &roleModel, basetypes.ObjectAsOptions{}); roleDiags.HasError() {
			diags.Append(roleDiags...)
			return nil, diags
		}

		id, err := strconv.Atoi(roleModel.ID.ValueString())
		if err != nil {
			diags.AddError(
				"Invalid Role ID",
				fmt.Sprintf("Failed to convert role ID %s to integer: %s", roleModel.ID.ValueString(), err.Error()),
			)
			return nil, diags
		}

		roleSet = append(roleSet, &cxsdk.RoleID{Id: uint32(id)})
	}

	return roleSet, diags
}

//func extractUserIds(usersIds types.Set) ([]*cxsdk.UserID, diag.Diagnostics) {
//	var diags diag.Diagnostics
//	if usersIds.IsNull() || usersIds.IsUnknown() {
//		return nil, diags
//	}
//
//	userElements := usersIds.Elements()
//	userSet := make([]*cxsdk.UserID, 0, len(userElements))
//	for _, userElement := range userElements {
//		user, ok := userElement.(types.String)
//		if !ok {
//			diags.AddError(
//				"Invalid User ID",
//				fmt.Sprintf("Expected user ID to be of type string, got: %T", userElement),
//			)
//			return nil, diags
//		}
//
//		userSet = append(userSet, &cxsdk.UserID{Id: user.ValueString()})
//	}
//
//	return userSet, diags
//}

func extractScope(ctx context.Context, scope types.Object) (*cxsdk.ScopeFilters, diag.Diagnostics) {
	if scope.IsNull() || scope.IsUnknown() {
		return nil, nil
	}

	var scopeModel ScopeModel
	if diags := scope.As(ctx, &scopeModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	return extractScopeFilters(ctx, scopeModel.Filters)
}

func extractScopeFilters(ctx context.Context, scopeFilters types.Object) (*cxsdk.ScopeFilters, diag.Diagnostics) {
	if scopeFilters.IsNull() || scopeFilters.IsUnknown() {
		return nil, nil
	}

	var scopeFilterModel ScopeFiltersModel
	if diags := scopeFilters.As(ctx, &scopeFilterModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	subsystems, diags := extractScopeFilterList(ctx, scopeFilterModel.Subsystems)
	if diags.HasError() {
		return nil, diags
	}

	applications, diags := extractScopeFilterList(ctx, scopeFilterModel.Applications)
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.ScopeFilters{
		Subsystems:   subsystems,
		Applications: applications,
	}, diags
}

func extractScopeFilterList(ctx context.Context, filters types.Set) ([]*cxsdk.GroupScopeFilter, diag.Diagnostics) {
	var diags diag.Diagnostics
	if filters.IsNull() || filters.IsUnknown() {
		return nil, diags
	}

	filterElements := filters.Elements()
	filterSet := make([]*cxsdk.GroupScopeFilter, 0, len(filterElements))
	for _, filterElement := range filterElements {
		filter, ok := filterElement.(types.Object)
		if !ok {
			diags.AddError(
				"Invalid Group Scope Filter",
				fmt.Sprintf("Expected group scope filter to be of type object, got: %T", filterElement),
			)
			return nil, diags
		}

		var filterModel GroupScopeFilterModel
		if filterDiags := filter.As(ctx, &filterModel, basetypes.ObjectAsOptions{}); filterDiags.HasError() {
			diags.Append(filterDiags...)
			return nil, diags
		}

		filterSet = append(filterSet, &cxsdk.GroupScopeFilter{
			Term:       filterModel.Term.ValueString(),
			FilterType: FilterTypeSchemaToProto[filterModel.FilterType.ValueString()],
		})
	}

	return filterSet, diags
}
