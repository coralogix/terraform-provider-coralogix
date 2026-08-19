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
	"errors"
	"fmt"
	"net/http"
	"strconv"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	roless "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/role_management_service"
	teamGroups "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/team_groups_management_service"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/coralogix/terraform-provider-coralogix/internal/utils"
)

func parseGroupID(id string) (int64, diag.Diagnostics) {
	parsed, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return 0, diag.Diagnostics{diag.NewErrorDiagnostic("Invalid group ID", err.Error())}
	}
	return parsed, nil
}

type groupNotFoundError struct {
	id int64
}

func (e *groupNotFoundError) Error() string {
	return fmt.Sprintf("group %d not found", e.id)
}

func isGroupNotFoundErr(err error) bool {
	var notFound *groupNotFoundError
	return errors.As(err, &notFound)
}

func listGroupUserIDs(ctx context.Context, client *teamGroups.TeamGroupsManagementServiceAPIService, groupID int64) ([]string, *http.Response, error) {
	var ids []string
	var pageToken string
	for {
		req := client.GroupsMgmtServiceGetGroupUsers(ctx, groupID).PageSize(100)
		if pageToken != "" {
			req = req.PageToken(pageToken)
		}
		resp, httpResp, err := req.Execute()
		if err != nil {
			return nil, httpResp, err
		}
		if resp == nil {
			return ids, httpResp, nil
		}
		for _, user := range resp.GetUsers() {
			if user.UserId != nil && *user.UserId != "" {
				ids = append(ids, *user.UserId)
			}
		}
		if resp.NextPageToken == nil || *resp.NextPageToken == "" || *resp.NextPageToken == pageToken {
			return ids, httpResp, nil
		}
		pageToken = *resp.NextPageToken
	}
}

func flattenTeamGroup(group *teamGroups.TeamGroup, memberIDs []string) (*GroupResourceModel, diag.Diagnostics) {
	return flattenTeamGroupWithPreferredRole(group, memberIDs, "")
}

func flattenTeamGroupWithPreferredRole(group *teamGroups.TeamGroup, memberIDs []string, preferredRole string) (*GroupResourceModel, diag.Diagnostics) {
	if group == nil || group.GroupId == nil {
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Invalid group", "API returned an empty group")}
	}

	members, diags := flattenMemberIDs(memberIDs)
	if diags.HasError() {
		return nil, diags
	}

	scopeID := types.StringNull()
	if group.Scope != nil && group.Scope.ScopeId != nil && *group.Scope.ScopeId != "" {
		scopeID = types.StringValue(*group.Scope.ScopeId)
	}

	apiRole := ""
	if group.Role != nil {
		apiRole = group.Role.GetName()
	}

	return &GroupResourceModel{
		ID:          types.StringValue(strconv.FormatInt(*group.GroupId, 10)),
		DisplayName: types.StringValue(group.GetName()),
		Members:     members,
		Role:        types.StringValue(roleNameForState(apiRole, preferredRole)),
		ScopeID:     scopeID,
	}, nil
}

func flattenMemberIDs(memberIDs []string) (types.Set, diag.Diagnostics) {
	if len(memberIDs) == 0 {
		return types.SetNull(types.StringType), nil
	}
	values := make([]attr.Value, 0, len(memberIDs))
	for _, id := range memberIDs {
		values = append(values, types.StringValue(id))
	}
	return types.SetValue(types.StringType, values)
}

func extractMemberIDs(ctx context.Context, members types.Set) ([]string, diag.Diagnostics) {
	if members.IsNull() || members.IsUnknown() {
		return []string{}, nil
	}
	var ids []string
	diags := members.ElementsAs(ctx, &ids, false)
	if ids == nil {
		ids = []string{}
	}
	return ids, diags
}

// roleAliases maps historical SCIM role names to OpenAPI system role names.
// SCIM accepted "Read Only"; ListSystemRoles returns "Legacy Read Only".
var roleAliases = map[string]string{
	"Read Only": "Legacy Read Only",
}

func roleLookupNames(name string) []string {
	names := []string{name}
	if alias, ok := roleAliases[name]; ok {
		names = append(names, alias)
	}
	return names
}

func roleNameMatches(apiName, requested string) bool {
	if apiName == requested {
		return true
	}
	if alias, ok := roleAliases[requested]; ok && apiName == alias {
		return true
	}
	return false
}

func roleNameForState(apiName, preferred string) string {
	if preferred != "" && roleNameMatches(apiName, preferred) {
		return preferred
	}
	if preferred == "" {
		for alias, target := range roleAliases {
			if apiName == target {
				return alias
			}
		}
	}
	return apiName
}

func lookupRoleID(ctx context.Context, roles *roless.RoleManagementServiceAPIService, name string) (int64, diag.Diagnostics) {
	var diags diag.Diagnostics
	candidates := roleLookupNames(name)

	systemResp, httpResp, err := roles.RoleManagementServiceListSystemRoles(ctx).Execute()
	if err != nil {
		diags.AddError("Error listing system roles", utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResp, err), "ListSystemRoles", name))
		return 0, diags
	}
	if id, ok := firstMatchingRoleID(systemRoleIDs(systemResp), candidates); ok {
		return id, diags
	}

	customResp, httpResp, err := roles.RoleManagementServiceListCustomRoles(ctx).Execute()
	if err != nil {
		diags.AddError("Error listing custom roles", utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResp, err), "ListCustomRoles", name))
		return 0, diags
	}
	if id, ok := firstMatchingRoleID(customRoleIDs(customResp), candidates); ok {
		return id, diags
	}

	diags.AddError("Role not found", fmt.Sprintf("no system or custom role named %q", name))
	return 0, diags
}

type namedRoleID struct {
	name string
	id   int64
}

func systemRoleIDs(resp *roless.ListSystemRolesResponse) []namedRoleID {
	if resp == nil {
		return nil
	}
	out := make([]namedRoleID, 0, len(resp.GetRoles()))
	for _, role := range resp.GetRoles() {
		if role.RoleId != nil && role.GetName() != "" {
			out = append(out, namedRoleID{name: role.GetName(), id: *role.RoleId})
		}
	}
	return out
}

func customRoleIDs(resp *roless.ListCustomRolesResponse) []namedRoleID {
	if resp == nil {
		return nil
	}
	out := make([]namedRoleID, 0, len(resp.GetRoles()))
	for _, role := range resp.GetRoles() {
		if role.RoleId != nil && role.GetName() != "" {
			out = append(out, namedRoleID{name: role.GetName(), id: *role.RoleId})
		}
	}
	return out
}

func firstMatchingRoleID(roles []namedRoleID, names []string) (int64, bool) {
	for _, name := range names {
		for _, role := range roles {
			if role.name == name {
				return role.id, true
			}
		}
	}
	return 0, false
}

func teamGroupRoleUpdate(roleID int64) *teamGroups.RoleUpdate {
	return &teamGroups.RoleUpdate{
		Action: &teamGroups.RoleUpdateAction{
			ActionType: "set_role_id",
			SetRoleId: &teamGroups.SetRoleId{
				Value: teamGroups.PtrInt64(roleID),
			},
		},
	}
}

func teamGroupUserUpdates(operationType string, userIDs []string) *teamGroups.UserUpdates {
	list := &teamGroups.UserIdList{UserIds: userIDs}
	op := teamGroups.NewUserUpdatesOperation(operationType)
	switch operationType {
	case "add":
		op.SetAdd(*list)
	case "remove":
		op.SetRemove(*list)
	default:
		op.SetSet(*list)
	}
	return &teamGroups.UserUpdates{Operation: op}
}

func teamGroupScopeSet(scopeID string) *teamGroups.ScopeUpdate {
	return &teamGroups.ScopeUpdate{
		Action: &teamGroups.ScopeUpdateAction{
			ActionType: "set_scope_id",
			SetScopeId: &teamGroups.SetScopeId{
				Value: teamGroups.PtrString(scopeID),
			},
		},
	}
}

func userIDSet(ids []string) map[string]struct{} {
	out := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		out[id] = struct{}{}
	}
	return out
}

func userIDsToAdd(planIDs, existingIDs []string) []string {
	existing := userIDSet(existingIDs)
	var add []string
	for _, id := range planIDs {
		if _, ok := existing[id]; !ok {
			add = append(add, id)
		}
	}
	return add
}

func userIDsToRemove(stateIDs, existingIDs []string) []string {
	existing := userIDSet(existingIDs)
	var remove []string
	for _, id := range stateIDs {
		if _, ok := existing[id]; ok {
			remove = append(remove, id)
		}
	}
	return remove
}

func applyGroupUserOperation(ctx context.Context, client *teamGroups.TeamGroupsManagementServiceAPIService, groupID int64, operationType string, userIDs []string) (*http.Response, error) {
	// add/remove of an empty list is a no-op. set of an empty list replaces all members.
	if len(userIDs) == 0 && operationType != "set" {
		return nil, nil
	}
	if userIDs == nil {
		userIDs = []string{}
	}
	_, httpResp, err := client.
		GroupsMgmtServiceUpdateTeamGroup(ctx, groupID).
		UpdateTeamGroupRequest(teamGroups.UpdateTeamGroupRequest{
			UserUpdates: teamGroupUserUpdates(operationType, userIDs),
		}).
		Execute()
	return httpResp, err
}

// desiredAttachmentGroupUserIDs is the full member list for one attachment Update set.
// Keep users this attachment does not own. Drop users in state but not in plan. Add plan users.
func desiredAttachmentGroupUserIDs(existing, stateIDs, planIDs []string) []string {
	state := userIDSet(stateIDs)
	plan := userIDSet(planIDs)
	seen := make(map[string]struct{}, len(existing)+len(planIDs))
	out := make([]string, 0, len(existing)+len(planIDs))

	for _, id := range existing {
		if _, inState := state[id]; inState {
			if _, inPlan := plan[id]; !inPlan {
				continue
			}
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	for _, id := range planIDs {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func userIDDiff(planIDs, stateIDs []string) (add, remove []string) {
	plan := userIDSet(planIDs)
	state := userIDSet(stateIDs)
	for id := range plan {
		if _, ok := state[id]; !ok {
			add = append(add, id)
		}
	}
	for id := range state {
		if _, ok := plan[id]; !ok {
			remove = append(remove, id)
		}
	}
	return add, remove
}
