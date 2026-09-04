// Copyright 2026 Coralogix Ltd.
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
	"slices"
	"strconv"
	"strings"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	teamGroups "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/team_groups_management_service"
	users "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/users_management_service"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const (
	// userPageSize is the page size for every paginated users or groups read.
	userPageSize = int64(100)
	// userEmailType is the email type SCIM returned for every user.
	userEmailType = "work"
)

// createLoginModes is the login mode list a new user gets. The Users API rejects a
// create whose template has no login mode, and the resource exposes no attribute for
// it, so the provider has to choose one value for every user it creates.
func createLoginModes() []users.AllowedLoginMode {
	return []users.AllowedLoginMode{
		users.ALLOWEDLOGINMODE_ALLOWED_LOGIN_MODE_SSO,
		users.ALLOWEDLOGINMODE_ALLOWED_LOGIN_MODE_LOCAL,
	}
}

// userNotFoundError marks a user that the backend no longer has, so read, update and
// delete can each choose their own reaction to it.
type userNotFoundError struct {
	id string
}

func (e *userNotFoundError) Error() string {
	return fmt.Sprintf("user %q not found", e.id)
}

func isUserNotFoundErr(err error) bool {
	var notFound *userNotFoundError
	return errors.As(err, &notFound)
}

// userClients holds what a user read or write needs: the two services it calls, and
// the team id every Users path contains. teamID is the client set's own resolver, so
// the resource and the data source share one cached lookup without either of them
// holding the whole client set.
type userClients struct {
	users      *users.UsersManagementServiceAPIService
	teamGroups *teamGroups.TeamGroupsManagementServiceAPIService
	teamID     func(ctx context.Context) (int64, error)
}

func newUserClients(clientSet *clientset.ClientSet) *userClients {
	return &userClients{
		users:      clientSet.Users(),
		teamGroups: clientSet.TeamGroups(),
		teamID:     clientSet.TeamID,
	}
}

// searchUsers pages SearchUsers and returns every user it lists. An empty username
// lists the whole team; a non-empty one asks the backend to narrow the result, which
// it may do partially, so callers still have to match the username themselves.
func searchUsers(ctx context.Context, client *users.UsersManagementServiceAPIService, teamID int64, username string) ([]users.RbacV2User, error) {
	var found []users.RbacV2User
	var pageToken int64

	for {
		req := client.UsersMgmtServiceSearchUsers(ctx, teamID).PageSize(userPageSize)
		if username != "" {
			req = req.Username(username)
		}
		if pageToken != 0 {
			req = req.PageToken(pageToken)
		}

		resp, httpResp, err := req.Execute()
		if err != nil {
			return nil, cxsdkOpenapi.NewAPIError(httpResp, err)
		}
		if resp == nil || len(resp.Users) == 0 {
			return found, nil
		}
		found = append(found, resp.Users...)

		// The token is an offset into the result set. A missing token, or one that does
		// not move forward, means this was the last page.
		next := resp.GetNextPageToken()
		if next <= pageToken {
			return found, nil
		}
		pageToken = next
	}
}

// findUserByID resolves a Terraform id, which is the stable userId, to a user. The
// Users API cannot read by userId, so this pages the team and matches. When state also
// holds the username, hintUsername narrows the first attempt to a single page.
func findUserByID(ctx context.Context, client *users.UsersManagementServiceAPIService, teamID int64, userID, hintUsername string) (*users.RbacV2User, error) {
	if hintUsername != "" {
		candidates, err := searchUsers(ctx, client, teamID, hintUsername)
		if err != nil {
			return nil, err
		}
		if user := matchUserByID(candidates, userID); user != nil {
			return user, nil
		}
	}

	candidates, err := searchUsers(ctx, client, teamID, "")
	if err != nil {
		return nil, err
	}
	if user := matchUserByID(candidates, userID); user != nil {
		return user, nil
	}

	return nil, &userNotFoundError{id: userID}
}

// findUsersByUsername returns every user whose username equals the given one. The
// server-side filter can match partially, so the exact comparison happens here.
func findUsersByUsername(ctx context.Context, client *users.UsersManagementServiceAPIService, teamID int64, username string) ([]users.RbacV2User, error) {
	candidates, err := searchUsers(ctx, client, teamID, username)
	if err != nil {
		return nil, err
	}
	return matchUsersByUsername(candidates, username), nil
}

func matchUserByID(candidates []users.RbacV2User, userID string) *users.RbacV2User {
	for i := range candidates {
		if candidates[i].GetUserId() == userID {
			return &candidates[i]
		}
	}
	return nil
}

// matchUsersByUsername compares case-insensitively, because SSO login can normalize
// letter case in the backend.
func matchUsersByUsername(candidates []users.RbacV2User, username string) []users.RbacV2User {
	matches := make([]users.RbacV2User, 0, len(candidates))
	for _, candidate := range candidates {
		if strings.EqualFold(candidate.GetUsername(), username) {
			matches = append(matches, candidate)
		}
	}
	return matches
}

func userIDs(candidates []users.RbacV2User) []string {
	ids := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if id := candidate.GetUserId(); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

// listUserGroupIDs returns the ids of the groups the user belongs to. The Users API
// does not report memberships, so this reads each group's member list once. The cost
// is one request per group, not one per user.
func listUserGroupIDs(ctx context.Context, client *teamGroups.TeamGroupsManagementServiceAPIService, teamID int64, userID string) ([]string, error) {
	var groupIDs []string
	var pageToken string

	for {
		req := client.GroupsMgmtServiceGetTeamGroups(ctx).TeamId(teamID).PageSize(userPageSize)
		if pageToken != "" {
			req = req.PageToken(pageToken)
		}

		resp, httpResp, err := req.Execute()
		if err != nil {
			return nil, cxsdkOpenapi.NewAPIError(httpResp, err)
		}
		if resp == nil {
			return groupIDs, nil
		}

		for _, group := range resp.Groups {
			if group.GroupId == nil {
				continue
			}
			memberIDs, err := listTeamGroupUserIDs(ctx, client, *group.GroupId)
			if err != nil {
				return nil, err
			}
			if slices.Contains(memberIDs, userID) {
				groupIDs = append(groupIDs, strconv.FormatInt(*group.GroupId, 10))
			}
		}

		next := resp.GetNextPageToken()
		if next == "" || next == pageToken {
			return groupIDs, nil
		}
		pageToken = next
	}
}

func listTeamGroupUserIDs(ctx context.Context, client *teamGroups.TeamGroupsManagementServiceAPIService, groupID int64) ([]string, error) {
	var ids []string
	var pageToken string

	for {
		req := client.GroupsMgmtServiceGetGroupUsers(ctx, groupID).PageSize(userPageSize)
		if pageToken != "" {
			req = req.PageToken(pageToken)
		}

		resp, httpResp, err := req.Execute()
		if err != nil {
			return nil, cxsdkOpenapi.NewAPIError(httpResp, err)
		}
		if resp == nil {
			return ids, nil
		}
		for _, member := range resp.Users {
			if id := member.GetUserId(); id != "" {
				ids = append(ids, id)
			}
		}

		next := resp.GetNextPageToken()
		if next == "" || next == pageToken {
			return ids, nil
		}
		pageToken = next
	}
}

// readUser resolves the user and its group memberships, then flattens both into state.
// The team id is a parameter rather than a lookup, so a caller that already has it
// does not pay for a second WhoAmI.
func readUser(ctx context.Context, clients *userClients, teamID int64, userID, hintUsername string) (*UserResourceModel, error) {
	user, err := findUserByID(ctx, clients.users, teamID, userID, hintUsername)
	if err != nil {
		return nil, err
	}

	return flattenUserWithGroups(ctx, clients, teamID, user)
}

func flattenUserWithGroups(ctx context.Context, clients *userClients, teamID int64, user *users.RbacV2User) (*UserResourceModel, error) {
	groupIDs, err := listUserGroupIDs(ctx, clients.teamGroups, teamID, user.GetUserId())
	if err != nil {
		return nil, err
	}

	state, diags := flattenUser(ctx, user, groupIDs)
	if diags.HasError() {
		first := diags.Errors()[0]
		return nil, fmt.Errorf("%s: %s", first.Summary(), first.Detail())
	}
	return state, nil
}

func flattenUser(ctx context.Context, user *users.RbacV2User, groupIDs []string) (*UserResourceModel, diag.Diagnostics) {
	if user.GetUserId() == "" {
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic(
			"Invalid user",
			"The API returned a user without a userId. Report this to the provider developers.",
		)}
	}

	name, diags := flattenUserName(user)
	if diags.HasError() {
		return nil, diags
	}

	emails, diags := deriveUserEmails(ctx, user.GetUsername())
	if diags.HasError() {
		return nil, diags
	}

	// A user with no memberships gets a known empty set, never a null one, so other
	// configuration can index into `groups` the way it could under SCIM.
	if groupIDs == nil {
		groupIDs = []string{}
	}
	groups, diags := types.SetValueFrom(ctx, types.StringType, groupIDs)
	if diags.HasError() {
		return nil, diags
	}

	return &UserResourceModel{
		ID:       types.StringValue(user.GetUserId()),
		UserName: types.StringValue(user.GetUsername()),
		Name:     name,
		Active:   types.BoolValue(isUserActive(user)),
		Emails:   emails,
		Groups:   groups,
	}, nil
}

// flattenUserName keeps a user that has never had a name as a null object, which is
// what the SCIM read produced when it omitted the name entirely.
func flattenUserName(user *users.RbacV2User) (types.Object, diag.Diagnostics) {
	if user.FirstName == nil && user.LastName == nil {
		return types.ObjectNull(userNameAttr()), nil
	}
	return types.ObjectValue(userNameAttr(), map[string]attr.Value{
		"given_name":  types.StringValue(user.GetFirstName()),
		"family_name": types.StringValue(user.GetLastName()),
	})
}

func userNameAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"given_name":  types.StringType,
		"family_name": types.StringType,
	}
}

// deriveUserEmails rebuilds the computed emails set from the username. The Users API
// has no email collection, and the SCIM read always returned exactly one primary work
// email whose value was the username.
func deriveUserEmails(ctx context.Context, username string) (types.Set, diag.Diagnostics) {
	return types.SetValueFrom(ctx, types.ObjectType{AttrTypes: userEmailAttr()}, []UserEmailModel{{
		Primary: types.BoolValue(true),
		Value:   types.StringValue(username),
		Type:    types.StringValue(userEmailType),
	}})
}

func userEmailAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"primary": types.BoolType,
		"value":   types.StringType,
		"type":    types.StringType,
	}
}

// isUserActive treats every status other than ACTIVE as inactive, so an unknown or
// unspecified status never reports a user as usable.
func isUserActive(user *users.RbacV2User) bool {
	return user.GetStatus() == users.USERSTATUS_USER_STATUS_ACTIVE
}

func userStatusFromActive(active bool) users.UserStatus {
	if active {
		return users.USERSTATUS_USER_STATUS_ACTIVE
	}
	return users.USERSTATUS_USER_STATUS_INACTIVE
}

// createUserTemplate builds the create payload, which has to carry a login mode.
func createUserTemplate(username string, name *UserNameModel, active bool) *users.UserTemplate {
	template := updateUserTemplate(username, name, active)
	template.AllowedLoginMode = createLoginModes()
	return template
}

// updateUserTemplate builds the update payload. It sends only the fields the resource
// manages, so the login modes and the access type a user already has are left alone.
// The provider cannot preserve either one itself: both are writable in the template but
// absent from every read, so there is nothing to read back and resend.
func updateUserTemplate(username string, name *UserNameModel, active bool) *users.UserTemplate {
	status := userStatusFromActive(active)
	template := &users.UserTemplate{
		Username: &username,
		Status:   &status,
	}
	if name != nil {
		givenName := name.GivenName.ValueString()
		familyName := name.FamilyName.ValueString()
		template.FirstName = &givenName
		template.LastName = &familyName
	}
	return template
}

// createUserResultFor picks the result that belongs to the requested username. The
// response carries one entry per requested user and echoes the username, so a caller
// can join on that instead of on position.
func createUserResultFor(resp *users.CreateUsersResponse, username string) (*users.CreateUserResult, error) {
	if resp == nil {
		return nil, errors.New("create returned an empty response")
	}
	for i := range resp.Results {
		if strings.EqualFold(resp.Results[i].Username, username) {
			return &resp.Results[i], nil
		}
	}
	return nil, fmt.Errorf("create returned no result for %q", username)
}

// createdUserIDs turns one create result into the ids Terraform needs. HTTP success
// alone does not mean the user exists, so the per-user status decides.
func createdUserIDs(result *users.CreateUserResult) (userID string, userAccountID int64, err error) {
	switch status := result.GetStatus(); status {
	case users.CREATEUSERSTATUS_CREATE_USER_STATUS_CREATED:
		if result.GetUserId() == "" || result.GetUserAccountId() == 0 {
			return "", 0, fmt.Errorf("user %q was created without ids%s", result.Username, createResultMessage(result))
		}
		return result.GetUserId(), result.GetUserAccountId(), nil
	case users.CREATEUSERSTATUS_CREATE_USER_STATUS_ALREADY_EXISTS:
		return "", 0, fmt.Errorf(
			"user %q already exists. Import it with `terraform import` instead of creating it%s",
			result.Username, createResultMessage(result),
		)
	case users.CREATEUSERSTATUS_CREATE_USER_STATUS_INVITED,
		users.CREATEUSERSTATUS_CREATE_USER_STATUS_ALREADY_INVITED:
		return "", 0, fmt.Errorf(
			"user %q was invited rather than created, so it has no id until the invitation is accepted%s",
			result.Username, createResultMessage(result),
		)
	default:
		return "", 0, fmt.Errorf("user %q was not created (status %s)%s", result.Username, status, createResultMessage(result))
	}
}

func createResultMessage(result *users.CreateUserResult) string {
	if message := result.GetMessage(); message != "" {
		return ": " + message
	}
	return ""
}

// formatUserAPIError maps a 404 to the not-found sentinel and formats everything else.
func formatUserAPIError(httpResp *http.Response, err error, operation, userID string) error {
	apiErr := cxsdkOpenapi.NewAPIError(httpResp, err)
	if cxsdkOpenapi.IsNotFound(apiErr) {
		return &userNotFoundError{id: userID}
	}
	return fmt.Errorf("%s", utils.FormatOpenAPIErrors(apiErr, operation, userID))
}
