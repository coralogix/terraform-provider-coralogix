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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	users "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/users_management_service"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const (
	// userPageSize is the page size for every paginated users read.
	userPageSize = int64(100)
	// userEmailType is the email type SCIM returned for every user.
	userEmailType = "work"
	// userEchoPrivateKey is the private state key that holds the fields an update has
	// to send back unchanged. See userEcho.
	userEchoPrivateKey = "users_api_echo"
)

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

// userEcho holds the user fields that the resource does not manage but an update has to
// send. PUT replaces the whole template: an omitted login mode list is written as none,
// and an omitted status as inactive. So every update sends back what the last read
// returned. The values live in private state, which keeps them out of the schema.
type userEcho struct {
	AllowedLoginMode []users.AllowedLoginMode `json:"allowedLoginMode"`
	AccessType       *users.AccessType        `json:"accessType,omitempty"`
}

func userEchoFrom(user *users.RbacV2User) userEcho {
	return userEcho{
		AllowedLoginMode: user.AllowedLoginMode,
		AccessType:       user.AccessType,
	}
}

func encodeUserEcho(user *users.RbacV2User) ([]byte, error) {
	return json.Marshal(userEchoFrom(user))
}

// decodeUserEcho returns nil when private state has no echo, which happens after an
// upgrade from the SCIM provider when no read has run yet.
func decodeUserEcho(raw []byte) (*userEcho, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var echo userEcho
	if err := json.Unmarshal(raw, &echo); err != nil {
		return nil, fmt.Errorf("decoding the stored user fields: %w", err)
	}
	return &echo, nil
}

// searchUsers pages SearchUsers and returns every user it lists. An empty username
// lists the whole team. A non-empty one is an exact, case-insensitive filter.
func searchUsers(ctx context.Context, client *users.UsersManagementServiceAPIService, username string) ([]users.RbacV2User, error) {
	var found []users.RbacV2User
	var pageToken int64

	for {
		req := client.UsersMgmtServiceSearchUsers(ctx).PageSize(userPageSize)
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
// Users API cannot read by userId, so this searches and matches. When state holds the
// username, hintUsername narrows the first attempt to one request. Import has no
// username, so it pages the whole team.
func findUserByID(ctx context.Context, client *users.UsersManagementServiceAPIService, userID, hintUsername string) (*users.RbacV2User, error) {
	if hintUsername != "" {
		candidates, err := searchUsers(ctx, client, hintUsername)
		if err != nil {
			return nil, err
		}
		if user := matchUserByID(candidates, userID); user != nil {
			return user, nil
		}
	}

	candidates, err := searchUsers(ctx, client, "")
	if err != nil {
		return nil, err
	}
	if user := matchUserByID(candidates, userID); user != nil {
		return user, nil
	}

	return nil, &userNotFoundError{id: userID}
}

// findUsersByUsername returns every user whose username equals the given one.
func findUsersByUsername(ctx context.Context, client *users.UsersManagementServiceAPIService, username string) ([]users.RbacV2User, error) {
	candidates, err := searchUsers(ctx, client, username)
	if err != nil {
		return nil, err
	}
	return matchUsersByUsername(candidates, username), nil
}

// singleUserByUsername picks the one user an email lookup must resolve to. The data
// source and import by email share it, so both reject the same ambiguous results.
func singleUserByUsername(matches []users.RbacV2User, username string) (*users.RbacV2User, diag.Diagnostics) {
	var diags diag.Diagnostics
	switch len(matches) {
	case 0:
		diags.AddError(fmt.Sprintf("User with user_name %q not found", username), "")
	case 1:
		if matches[0].GetUserId() == "" {
			diags.AddError(
				fmt.Sprintf("User with user_name %q was returned without an id", username),
				"Use the user id instead, or report this to the provider developers.",
			)
			break
		}
		return &matches[0], nil
	default:
		diags.AddError(
			fmt.Sprintf("Multiple Users found with user_name %q", username),
			fmt.Sprintf("Matched user ids: %s. Use the user id instead.", strings.Join(userIDs(matches), ", ")),
		)
	}
	return nil, diags
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

// flattenUserToState flattens a user and turns the diagnostics into an error, for the
// callers that report errors rather than diagnostics.
func flattenUserToState(ctx context.Context, user *users.RbacV2User) (*UserResourceModel, error) {
	state, diags := flattenUser(ctx, user)
	if diags.HasError() {
		first := diags.Errors()[0]
		return nil, fmt.Errorf("%s: %s", first.Summary(), first.Detail())
	}
	return state, nil
}

func flattenUser(ctx context.Context, user *users.RbacV2User) (*UserResourceModel, diag.Diagnostics) {
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

	groups, diags := flattenUserGroups(ctx, user.GroupIds)
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

// flattenUserGroups writes group ids as decimal strings, the same values SCIM returned
// in groups[].value. A user with no memberships gets a known empty set, never a null
// one, so other configuration can index into `groups` the way it could under SCIM.
func flattenUserGroups(ctx context.Context, groupIDs []int64) (types.Set, diag.Diagnostics) {
	ids := make([]string, 0, len(groupIDs))
	for _, id := range groupIDs {
		ids = append(ids, strconv.FormatInt(id, 10))
	}
	return types.SetValueFrom(ctx, types.StringType, ids)
}

// flattenUserName returns a null object for a user without a name. The API returns an
// empty string, not an absent field, for a name that was never set.
func flattenUserName(user *users.RbacV2User) (types.Object, diag.Diagnostics) {
	if user.GetFirstName() == "" && user.GetLastName() == "" {
		return types.ObjectNull(userNameAttr()), nil
	}
	return types.ObjectValue(userNameAttr(), map[string]attr.Value{
		"given_name":  types.StringValue(user.GetFirstName()),
		"family_name": types.StringValue(user.GetLastName()),
	})
}

// preserveUserName keeps the prior name when it says the same thing as the API. The API
// cannot tell a null name from an empty one, so without this a configuration with
// `given_name = ""`, or with only one of the two names, would never reach an empty plan.
func preserveUserName(ctx context.Context, prior, fromAPI types.Object) types.Object {
	if prior.IsUnknown() {
		return fromAPI
	}
	priorName, diags := extractUserName(ctx, prior)
	if diags.HasError() {
		return fromAPI
	}
	apiName, diags := extractUserName(ctx, fromAPI)
	if diags.HasError() {
		return fromAPI
	}
	if userNameFirst(priorName) == userNameFirst(apiName) && userNameLast(priorName) == userNameLast(apiName) {
		return prior
	}
	return fromAPI
}

func userNameFirst(name *UserNameModel) string {
	if name == nil {
		return ""
	}
	return name.GivenName.ValueString()
}

func userNameLast(name *UserNameModel) string {
	if name == nil {
		return ""
	}
	return name.FamilyName.ValueString()
}

func userNameAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"given_name":  types.StringType,
		"family_name": types.StringType,
	}
}

// deriveUserEmails rebuilds the computed emails set from the username. The Users API
// has no email collection. The username is always the email, and the SCIM read always
// returned exactly one primary work email whose value was the username.
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

// createUserTemplate builds the create payload. It sends no login mode, so a new user
// gets none, which is what SCIM gave it.
func createUserTemplate(username string, name *UserNameModel, active bool) *users.UserTemplate {
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

// updateUserTemplate builds the full PUT payload. PUT replaces rather than merges, so it
// always carries both names, the status, and the echoed login modes and access type.
func updateUserTemplate(name *UserNameModel, active bool, echo userEcho) *users.UserTemplate {
	status := userStatusFromActive(active)
	givenName := userNameFirst(name)
	familyName := userNameLast(name)
	return &users.UserTemplate{
		FirstName:        &givenName,
		LastName:         &familyName,
		Status:           &status,
		AllowedLoginMode: echo.AllowedLoginMode,
		AccessType:       echo.AccessType,
	}
}

// putUser sends one PUT by userId and returns the user the response reports.
func putUser(ctx context.Context, client *users.UsersManagementServiceAPIService, userID string, template *users.UserTemplate) (*users.RbacV2User, error) {
	updateReq := []users.UpdateUserRequest{{
		UserId:       &userID,
		UserTemplate: template,
	}}
	resp, httpResp, err := client.UsersMgmtServiceUpdateUsers(ctx).UpdateUserRequest(updateReq).Execute()
	if err != nil {
		return nil, formatUserAPIError(httpResp, err, "UpdateUsers", userID)
	}
	if resp == nil {
		return nil, fmt.Errorf("update of user %q returned an empty response", userID)
	}
	if user := matchUserByID(resp.Users, userID); user != nil {
		return user, nil
	}
	return nil, fmt.Errorf("update of user %q returned no user", userID)
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

// createdUser turns one create result into the user it created. HTTP success alone
// does not mean the user exists, so the per-user status decides.
func createdUser(result *users.CreateUserResult) (*users.RbacV2User, error) {
	switch status := result.GetStatus(); status {
	case users.CREATEUSERSTATUS_CREATE_USER_STATUS_CREATED:
		if result.User == nil || result.User.GetUserId() == "" {
			return nil, fmt.Errorf("user %q was created, but the response carries no user%s", result.Username, createResultMessage(result))
		}
		return result.User, nil
	case users.CREATEUSERSTATUS_CREATE_USER_STATUS_ALREADY_EXISTS:
		return nil, fmt.Errorf(
			"user %q already exists. Import it with `terraform import` instead of creating it%s",
			result.Username, createResultMessage(result),
		)
	case users.CREATEUSERSTATUS_CREATE_USER_STATUS_INVITED,
		users.CREATEUSERSTATUS_CREATE_USER_STATUS_ALREADY_INVITED:
		return nil, fmt.Errorf(
			"user %q was invited rather than created, so it has no id until the invitation is accepted%s",
			result.Username, createResultMessage(result),
		)
	default:
		return nil, fmt.Errorf("user %q was not created (status %s)%s", result.Username, status, createResultMessage(result))
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
