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

	teamGroups "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/team_groups_management_service"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
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
		Role:        types.StringValue(apiRole),
		ScopeID:     scopeID,
	}, nil
}

func flattenMemberIDs(memberIDs []string) (types.Set, diag.Diagnostics) {
	values := make([]attr.Value, 0, len(memberIDs))
	for _, id := range memberIDs {
		values = append(values, types.StringValue(id))
	}
	return types.SetValue(types.StringType, values)
}

// membersForState keeps the members written to state consistent with the plan, which the
// framework requires whenever the planned value is known. An unknown plan accepts the
// group's actual membership, which is what a create or an unmanaged member list produces.
func membersForState(planned, flattened types.Set) types.Set {
	if planned.IsUnknown() {
		return flattened
	}
	return planned
}

// membersUnmanaged reports whether the configuration leaves membership to be maintained
// elsewhere. Ownership is decided by the configuration alone - prior state carries members
// this resource merely read, so it cannot say whether the configuration manages them.
// Only a null value means the argument was omitted: an unknown one was configured from a
// value Terraform has still to resolve, and it resolves before the member list is applied.
func membersUnmanaged(configuredMembers types.Set) bool {
	return configuredMembers.IsNull()
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

func teamGroupRoleUpdateByName(name string) *teamGroups.RoleUpdate {
	return &teamGroups.RoleUpdate{
		Action: &teamGroups.RoleUpdateAction{
			ActionType:    "set_role_by_name",
			SetRoleByName: teamGroups.NewSetRoleByName(name),
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
