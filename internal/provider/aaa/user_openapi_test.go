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
	"strings"
	"testing"

	users "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/users_management_service"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func testUser(userID, username, firstName, lastName string, status users.UserStatus) users.RbacV2User {
	user := users.RbacV2User{
		UserId:   &userID,
		Username: &username,
		Status:   &status,
	}
	if firstName != "" || lastName != "" {
		user.FirstName = &firstName
		user.LastName = &lastName
	}
	return user
}

func TestMatchUserByID(t *testing.T) {
	t.Parallel()

	candidates := []users.RbacV2User{
		testUser("id-a", "a@coralogix.com", "A", "One", users.USERSTATUS_USER_STATUS_ACTIVE),
		testUser("id-b", "b@coralogix.com", "B", "Two", users.USERSTATUS_USER_STATUS_ACTIVE),
	}

	if got := matchUserByID(candidates, "id-b"); got == nil || got.GetUsername() != "b@coralogix.com" {
		t.Errorf("matchUserByID(id-b) = %#v", got)
	}
	if got := matchUserByID(candidates, "id-missing"); got != nil {
		t.Errorf("matchUserByID(id-missing) = %#v, want nil", got)
	}
}

// SCIM returned exactly one primary work email whose value was the username. Anything
// else is a state contract change for configuration that reads `emails`.
func TestDeriveUserEmails(t *testing.T) {
	t.Parallel()

	emails, diags := deriveUserEmails(context.Background(), "user@coralogix.com")
	if diags.HasError() {
		t.Fatalf("deriveUserEmails diagnostics: %v", diags)
	}
	if len(emails.Elements()) != 1 {
		t.Fatalf("emails = %#v, want exactly one entry", emails)
	}

	var models []UserEmailModel
	if diags := emails.ElementsAs(context.Background(), &models, false); diags.HasError() {
		t.Fatalf("ElementsAs diagnostics: %v", diags)
	}
	if !models[0].Primary.ValueBool() {
		t.Error("primary = false, want true")
	}
	if models[0].Type.ValueString() != "work" {
		t.Errorf("type = %q, want work", models[0].Type.ValueString())
	}
	if models[0].Value.ValueString() != "user@coralogix.com" {
		t.Errorf("value = %q", models[0].Value.ValueString())
	}
}

func TestFlattenUserName(t *testing.T) {
	t.Parallel()

	named := testUser("id-a", "a@coralogix.com", "Given", "Family", users.USERSTATUS_USER_STATUS_ACTIVE)
	name, diags := flattenUserName(&named)
	if diags.HasError() {
		t.Fatalf("flattenUserName diagnostics: %v", diags)
	}
	if name.IsNull() {
		t.Fatal("name is null, want an object")
	}

	// A user the API never gave a name stays a null object, which is what the SCIM read
	// produced when it omitted the name entirely.
	nameless := testUser("id-b", "b@coralogix.com", "", "", users.USERSTATUS_USER_STATUS_ACTIVE)
	name, diags = flattenUserName(&nameless)
	if diags.HasError() {
		t.Fatalf("flattenUserName diagnostics: %v", diags)
	}
	if !name.IsNull() {
		t.Errorf("name = %#v, want null", name)
	}

	// A user the API reports with blank names becomes an object with empty strings, not
	// a null object, so the value round-trips.
	blank := ""
	blankNamed := testUser("id-c", "c@coralogix.com", "", "", users.USERSTATUS_USER_STATUS_ACTIVE)
	blankNamed.FirstName = &blank
	blankNamed.LastName = &blank
	name, diags = flattenUserName(&blankNamed)
	if diags.HasError() {
		t.Fatalf("flattenUserName diagnostics: %v", diags)
	}
	if name.IsNull() {
		t.Error("name is null, want an object with empty strings")
	}
}

func TestFlattenUser(t *testing.T) {
	t.Parallel()

	user := testUser("id-a", "a@coralogix.com", "Given", "Family", users.USERSTATUS_USER_STATUS_ACTIVE)
	state, diags := flattenUser(context.Background(), &user, []string{"1", "2"})
	if diags.HasError() {
		t.Fatalf("flattenUser diagnostics: %v", diags)
	}
	if state.ID.ValueString() != "id-a" {
		t.Errorf("id = %q, want the stable userId", state.ID.ValueString())
	}
	if !state.Active.ValueBool() {
		t.Error("active = false, want true for an ACTIVE user")
	}
	if len(state.Groups.Elements()) != 2 {
		t.Errorf("groups = %#v, want two ids", state.Groups)
	}

	// A user with no group memberships gets a known empty set, never a null one.
	state, diags = flattenUser(context.Background(), &user, nil)
	if diags.HasError() {
		t.Fatalf("flattenUser diagnostics: %v", diags)
	}
	if state.Groups.IsNull() {
		t.Error("groups is null, want an empty set")
	}
	if len(state.Groups.Elements()) != 0 {
		t.Errorf("groups = %#v, want an empty set", state.Groups)
	}
}

func TestFlattenUserRejectsMissingUserID(t *testing.T) {
	t.Parallel()

	user := testUser("", "a@coralogix.com", "", "", users.USERSTATUS_USER_STATUS_ACTIVE)
	if _, diags := flattenUser(context.Background(), &user, nil); !diags.HasError() {
		t.Error("flattenUser accepted a user without a userId")
	}
}

// Every status other than ACTIVE has to read as inactive, so an unspecified status is
// never reported as a usable user.
func TestIsUserActive(t *testing.T) {
	t.Parallel()

	for status, want := range map[users.UserStatus]bool{
		users.USERSTATUS_USER_STATUS_ACTIVE:      true,
		users.USERSTATUS_USER_STATUS_INACTIVE:    false,
		users.USERSTATUS_USER_STATUS_UNSPECIFIED: false,
	} {
		user := testUser("id-a", "a@coralogix.com", "", "", status)
		if got := isUserActive(&user); got != want {
			t.Errorf("isUserActive(%s) = %t, want %t", status, got, want)
		}
	}

	if got := userStatusFromActive(true); got != users.USERSTATUS_USER_STATUS_ACTIVE {
		t.Errorf("userStatusFromActive(true) = %s", got)
	}
	if got := userStatusFromActive(false); got != users.USERSTATUS_USER_STATUS_INACTIVE {
		t.Errorf("userStatusFromActive(false) = %s", got)
	}
}

func TestUpdateUserTemplate(t *testing.T) {
	t.Parallel()

	template := updateUserTemplate("user@coralogix.com", &UserNameModel{
		GivenName:  types.StringValue("Given"),
		FamilyName: types.StringValue("Family"),
	}, true)

	if template.GetUsername() != "user@coralogix.com" {
		t.Errorf("username = %q", template.GetUsername())
	}
	if template.GetFirstName() != "Given" || template.GetLastName() != "Family" {
		t.Errorf("name = %q %q", template.GetFirstName(), template.GetLastName())
	}
	if template.GetStatus() != users.USERSTATUS_USER_STATUS_ACTIVE {
		t.Errorf("status = %s", template.GetStatus())
	}

	// An update must not send a login mode or an access type. Both are writable but
	// absent from every read, so a value the provider invents would overwrite whatever
	// the user already has.
	if template.AllowedLoginMode != nil {
		t.Errorf("allowedLoginMode = %#v, want absent", template.AllowedLoginMode)
	}
	if template.AccessType != nil {
		t.Errorf("accessType = %#v, want absent", template.AccessType)
	}

	// A configuration with no name block must not send empty names, which would clear
	// a name the user already has.
	template = updateUserTemplate("user@coralogix.com", nil, false)
	if template.FirstName != nil || template.LastName != nil {
		t.Errorf("name = %#v %#v, want both absent", template.FirstName, template.LastName)
	}
	if template.GetStatus() != users.USERSTATUS_USER_STATUS_INACTIVE {
		t.Errorf("status = %s", template.GetStatus())
	}
}

// The create payload has to carry a login mode, because the API rejects a create
// without one.
func TestCreateUserTemplate(t *testing.T) {
	t.Parallel()

	template := createUserTemplate("user@coralogix.com", nil, true)
	if len(template.AllowedLoginMode) == 0 {
		t.Error("allowedLoginMode is empty, which the API rejects on create")
	}
	if template.AccessType != nil {
		t.Errorf("accessType = %#v, want absent", template.AccessType)
	}
}

func TestUserNameChanged(t *testing.T) {
	t.Parallel()

	user := testUser("id-a", "a@coralogix.com", "Given", "Family", users.USERSTATUS_USER_STATUS_ACTIVE)

	if userNameChanged(&user, nil) {
		t.Error("a configuration with no name block asks for no change")
	}
	if userNameChanged(&user, &UserNameModel{
		GivenName:  types.StringValue("Given"),
		FamilyName: types.StringValue("Family"),
	}) {
		t.Error("an unchanged name asks for no change")
	}
	if !userNameChanged(&user, &UserNameModel{
		GivenName:  types.StringValue("Other"),
		FamilyName: types.StringValue("Family"),
	}) {
		t.Error("a changed given name asks for a change")
	}
}

// HTTP success alone does not mean the user exists. The per-user status decides, and
// only CREATED yields the ids Terraform stores.
func TestCreatedUserIDs(t *testing.T) {
	t.Parallel()

	userID := "id-a"
	accountID := int64(7)
	created := &users.CreateUserResult{Username: "a@coralogix.com", UserId: &userID, UserAccountId: &accountID}
	status := users.CREATEUSERSTATUS_CREATE_USER_STATUS_CREATED
	created.Status = &status

	gotID, gotAccountID, err := createdUserIDs(created)
	if err != nil {
		t.Fatalf("createdUserIDs error: %v", err)
	}
	if gotID != userID || gotAccountID != accountID {
		t.Errorf("ids = %q %d", gotID, gotAccountID)
	}

	for name, tc := range map[string]struct {
		status   users.CreateUserStatus
		wantWord string
	}{
		"already exists": {users.CREATEUSERSTATUS_CREATE_USER_STATUS_ALREADY_EXISTS, "already exists"},
		"invited":        {users.CREATEUSERSTATUS_CREATE_USER_STATUS_INVITED, "invited"},
		"domain":         {users.CREATEUSERSTATUS_CREATE_USER_STATUS_DOMAIN_NOT_ALLOWED, "not created"},
		"failed":         {users.CREATEUSERSTATUS_CREATE_USER_STATUS_FAILED, "not created"},
		"unspecified":    {users.CREATEUSERSTATUS_CREATE_USER_STATUS_UNSPECIFIED, "not created"},
	} {
		result := &users.CreateUserResult{Username: "a@coralogix.com", Status: &tc.status}
		_, _, err := createdUserIDs(result)
		if err == nil {
			t.Errorf("%s: createdUserIDs accepted status %s", name, tc.status)
			continue
		}
		if !strings.Contains(err.Error(), tc.wantWord) {
			t.Errorf("%s: error = %q, want it to mention %q", name, err, tc.wantWord)
		}
	}

	// A CREATED result without ids is still a failure, because Terraform cannot store
	// an empty id.
	noIDs := &users.CreateUserResult{Username: "a@coralogix.com", Status: &status}
	if _, _, err := createdUserIDs(noIDs); err == nil {
		t.Error("createdUserIDs accepted a CREATED result without ids")
	}
}

// The create response carries one entry per requested user and echoes the username, so
// the result is joined on the username rather than on position.
func TestCreateUserResultFor(t *testing.T) {
	t.Parallel()

	resp := &users.CreateUsersResponse{Results: []users.CreateUserResult{
		{Username: "other@coralogix.com"},
		{Username: "A@Coralogix.com"},
	}}

	result, err := createUserResultFor(resp, "a@coralogix.com")
	if err != nil {
		t.Fatalf("createUserResultFor error: %v", err)
	}
	if result.Username != "A@Coralogix.com" {
		t.Errorf("result = %q, want the case-insensitive match", result.Username)
	}

	if _, err := createUserResultFor(resp, "missing@coralogix.com"); err == nil {
		t.Error("createUserResultFor accepted a username with no result")
	}
	if _, err := createUserResultFor(nil, "a@coralogix.com"); err == nil {
		t.Error("createUserResultFor accepted an empty response")
	}
}

// A backend that normalizes letter case must not produce an inconsistent-result error.
func TestPreserveUserNameCase(t *testing.T) {
	t.Parallel()

	configured := types.StringValue("User@Coralogix.com")
	fromAPI := types.StringValue("user@coralogix.com")

	if got := preserveUserNameCase(configured, fromAPI); got.ValueString() != "User@Coralogix.com" {
		t.Errorf("got %q, want the configured case", got.ValueString())
	}

	// A genuinely different username is the backend's, not the configuration's.
	if got := preserveUserNameCase(configured, types.StringValue("other@coralogix.com")); got.ValueString() != "other@coralogix.com" {
		t.Errorf("got %q, want the API value", got.ValueString())
	}

	if got := preserveUserNameCase(types.StringNull(), fromAPI); got.ValueString() != "user@coralogix.com" {
		t.Errorf("got %q, want the API value", got.ValueString())
	}
}

func TestIsUserNotFoundErr(t *testing.T) {
	t.Parallel()

	if !isUserNotFoundErr(&userNotFoundError{id: "id-a"}) {
		t.Error("isUserNotFoundErr did not recognize its own sentinel")
	}
	if isUserNotFoundErr(context.Canceled) {
		t.Error("isUserNotFoundErr matched an unrelated error")
	}
}
