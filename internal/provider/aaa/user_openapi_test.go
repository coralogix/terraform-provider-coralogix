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
	"net/http"
	"strings"
	"testing"

	users "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/users_management_service"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
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

	// The API returns empty strings, not absent fields, for a user without a name.
	// That has to read as null too, or every nameless user would drift.
	blank := ""
	blankNamed := testUser("id-c", "c@coralogix.com", "", "", users.USERSTATUS_USER_STATUS_ACTIVE)
	blankNamed.FirstName = &blank
	blankNamed.LastName = &blank
	name, diags = flattenUserName(&blankNamed)
	if diags.HasError() {
		t.Fatalf("flattenUserName diagnostics: %v", diags)
	}
	if !name.IsNull() {
		t.Errorf("name = %#v, want null for empty names", name)
	}
}

func TestFlattenUser(t *testing.T) {
	t.Parallel()

	user := testUser("id-a", "a@coralogix.com", "Given", "Family", users.USERSTATUS_USER_STATUS_ACTIVE)
	user.GroupIds = []int64{145282, 7}
	state, diags := flattenUser(context.Background(), &user)
	if diags.HasError() {
		t.Fatalf("flattenUser diagnostics: %v", diags)
	}
	if state.ID.ValueString() != "id-a" {
		t.Errorf("id = %q, want the stable userId", state.ID.ValueString())
	}
	if !state.Active.ValueBool() {
		t.Error("active = false, want true for an ACTIVE user")
	}
	// SCIM returned groups[].value as decimal strings of the numeric group ids.
	want := types.SetValueMust(types.StringType, []attr.Value{types.StringValue("145282"), types.StringValue("7")})
	if !state.Groups.Equal(want) {
		t.Errorf("groups = %s, want %s", state.Groups, want)
	}

	// A user with no group memberships gets a known empty set, never a null one.
	user.GroupIds = nil
	state, diags = flattenUser(context.Background(), &user)
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
	if _, diags := flattenUser(context.Background(), &user); !diags.HasError() {
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

// PUT replaces rather than merges. The template therefore always carries both names,
// the status and whatever login modes and access type the last read returned, even
// when those are empty.
func TestUpdateUserTemplate(t *testing.T) {
	t.Parallel()

	sso := userEcho{AllowedLoginMode: []users.AllowedLoginMode{users.ALLOWEDLOGINMODE_ALLOWED_LOGIN_MODE_SSO}, AccessType: users.NewAccessType("permanent")}
	template := updateUserTemplate(&UserNameModel{
		GivenName:  types.StringValue("Given"),
		FamilyName: types.StringValue("Family"),
	}, true, sso)

	if template.GetFirstName() != "Given" || template.GetLastName() != "Family" {
		t.Errorf("name = %q %q", template.GetFirstName(), template.GetLastName())
	}
	if template.GetStatus() != users.USERSTATUS_USER_STATUS_ACTIVE {
		t.Errorf("status = %s", template.GetStatus())
	}
	if len(template.AllowedLoginMode) != 1 || template.AllowedLoginMode[0] != users.ALLOWEDLOGINMODE_ALLOWED_LOGIN_MODE_SSO {
		t.Errorf("allowedLoginMode = %v, want the echoed SSO", template.AllowedLoginMode)
	}
	if template.AccessType.GetAccessType() != "permanent" {
		t.Errorf("accessType = %#v, want the echoed value", template.AccessType)
	}
	if template.Username != nil {
		t.Errorf("username = %q, want absent: the API ignores it on update", template.GetUsername())
	}

	// No name block still sends both names, as empty strings, so the request never
	// depends on how the backend treats an omitted field.
	template = updateUserTemplate(nil, false, userEcho{})
	if template.FirstName == nil || template.LastName == nil || template.GetFirstName() != "" || template.GetLastName() != "" {
		t.Errorf("name = %#v %#v, want two empty strings", template.FirstName, template.LastName)
	}
	if template.GetStatus() != users.USERSTATUS_USER_STATUS_INACTIVE {
		t.Errorf("status = %s", template.GetStatus())
	}
	if len(template.AllowedLoginMode) != 0 {
		t.Errorf("allowedLoginMode = %v, want empty when the read returned none", template.AllowedLoginMode)
	}
}

// Create sends no login mode. SCIM created users without one, and the resource has no
// attribute to choose one.
func TestCreateUserTemplate(t *testing.T) {
	t.Parallel()

	template := createUserTemplate("user@coralogix.com", nil, false)
	if template.GetUsername() != "user@coralogix.com" {
		t.Errorf("username = %q", template.GetUsername())
	}
	if template.AllowedLoginMode != nil {
		t.Errorf("allowedLoginMode = %v, want absent", template.AllowedLoginMode)
	}
	if template.AccessType != nil {
		t.Errorf("accessType = %#v, want absent", template.AccessType)
	}
	if template.GetStatus() != users.USERSTATUS_USER_STATUS_INACTIVE {
		t.Errorf("status = %s, want the planned INACTIVE", template.GetStatus())
	}
	if template.FirstName != nil || template.LastName != nil {
		t.Errorf("name = %#v %#v, want absent without a name block", template.FirstName, template.LastName)
	}
}

// The echo round-trips through private state as JSON. An empty login mode list must stay
// empty, never turn into a default.
func TestUserEchoRoundTrip(t *testing.T) {
	t.Parallel()

	for name, user := range map[string]users.RbacV2User{
		"sso and local": {
			AllowedLoginMode: []users.AllowedLoginMode{users.ALLOWEDLOGINMODE_ALLOWED_LOGIN_MODE_LOCAL, users.ALLOWEDLOGINMODE_ALLOWED_LOGIN_MODE_SSO},
			AccessType:       &users.AccessType{AccessType: "permanent", PermanentAccess: map[string]any{}},
		},
		"none": {AllowedLoginMode: []users.AllowedLoginMode{}, AccessType: users.NewAccessType("permanent")},
	} {
		raw, err := encodeUserEcho(&user)
		if err != nil {
			t.Fatalf("%s: encode: %v", name, err)
		}
		echo, err := decodeUserEcho(raw)
		if err != nil {
			t.Fatalf("%s: decode: %v", name, err)
		}
		if len(echo.AllowedLoginMode) != len(user.AllowedLoginMode) {
			t.Errorf("%s: allowedLoginMode = %v, want %v", name, echo.AllowedLoginMode, user.AllowedLoginMode)
		}
		if echo.AccessType.GetAccessType() != "permanent" {
			t.Errorf("%s: accessType = %#v", name, echo.AccessType)
		}
	}

	if echo, err := decodeUserEcho(nil); err != nil || echo != nil {
		t.Errorf("decodeUserEcho(nil) = %v, %v, want nil, nil", echo, err)
	}
}

// fakePrivateState stands in for the framework's private state in unit tests.
type fakePrivateState map[string][]byte

func (f fakePrivateState) GetKey(_ context.Context, key string) ([]byte, diag.Diagnostics) {
	return f[key], nil
}

func (f fakePrivateState) SetKey(_ context.Context, key string, value []byte) diag.Diagnostics {
	f[key] = value
	return nil
}

// Update and Delete echo what private state holds and make no read. Only state written
// by the SCIM provider, with no refresh since the upgrade, has nothing stored; that one
// case searches for the user first.
func TestUserResourceEchoSource(t *testing.T) {
	t.Parallel()

	var searches int
	client := newUsersClient(t, func(w http.ResponseWriter, r *http.Request) {
		searches++
		writeJSON(t, w, map[string]any{"users": []map[string]any{{
			"userId": "id-1", "username": "a@coralogix.com", "allowedLoginMode": []string{"ALLOWED_LOGIN_MODE_SSO"},
		}}})
	})
	r := &UserResource{client: client}

	stored := fakePrivateState{}
	local := users.RbacV2User{AllowedLoginMode: []users.AllowedLoginMode{users.ALLOWEDLOGINMODE_ALLOWED_LOGIN_MODE_LOCAL}}
	if diags := setUserEcho(context.Background(), stored, &local); diags.HasError() {
		t.Fatalf("setUserEcho: %v", diags)
	}
	echo, err := r.userEcho(context.Background(), stored, "id-1", "a@coralogix.com")
	if err != nil {
		t.Fatalf("userEcho: %v", err)
	}
	if searches != 0 {
		t.Errorf("searches = %d, want none when private state holds the echo", searches)
	}
	if len(echo.AllowedLoginMode) != 1 || echo.AllowedLoginMode[0] != users.ALLOWEDLOGINMODE_ALLOWED_LOGIN_MODE_LOCAL {
		t.Errorf("allowedLoginMode = %v, want the stored LOCAL", echo.AllowedLoginMode)
	}

	echo, err = r.userEcho(context.Background(), fakePrivateState{}, "id-1", "a@coralogix.com")
	if err != nil {
		t.Fatalf("userEcho: %v", err)
	}
	if searches != 1 {
		t.Errorf("searches = %d, want one when private state is empty", searches)
	}
	if len(echo.AllowedLoginMode) != 1 || echo.AllowedLoginMode[0] != users.ALLOWEDLOGINMODE_ALLOWED_LOGIN_MODE_SSO {
		t.Errorf("allowedLoginMode = %v, want the SSO the search returned", echo.AllowedLoginMode)
	}
}

// The API cannot tell a null name from an empty one. A prior value that says the same
// thing as the API is kept, so partial or empty name blocks reach an empty plan.
func TestPreserveUserName(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	nameObject := func(given, family types.String) types.Object {
		return types.ObjectValueMust(userNameAttr(), map[string]attr.Value{"given_name": given, "family_name": family})
	}
	nullName := types.ObjectNull(userNameAttr())

	for name, tc := range map[string]struct {
		prior, fromAPI, want types.Object
	}{
		"only given name configured": {
			prior:   nameObject(types.StringValue("Given"), types.StringNull()),
			fromAPI: nameObject(types.StringValue("Given"), types.StringValue("")),
			want:    nameObject(types.StringValue("Given"), types.StringNull()),
		},
		"empty names configured": {
			prior:   nameObject(types.StringValue(""), types.StringValue("")),
			fromAPI: nullName,
			want:    nameObject(types.StringValue(""), types.StringValue("")),
		},
		"no name block, no name": {prior: nullName, fromAPI: nullName, want: nullName},
		"changed out of band": {
			prior:   nameObject(types.StringValue("Old"), types.StringValue("Name")),
			fromAPI: nameObject(types.StringValue("New"), types.StringValue("Name")),
			want:    nameObject(types.StringValue("New"), types.StringValue("Name")),
		},
		"import": {
			prior:   types.ObjectUnknown(userNameAttr()),
			fromAPI: nameObject(types.StringValue("A"), types.StringValue("B")),
			want:    nameObject(types.StringValue("A"), types.StringValue("B")),
		},
	} {
		if got := preserveUserName(ctx, tc.prior, tc.fromAPI); !got.Equal(tc.want) {
			t.Errorf("%s: got %s, want %s", name, got, tc.want)
		}
	}
}

// HTTP success alone does not mean the user exists. The per-user status decides, and
// only CREATED with a user yields state.
func TestCreatedUser(t *testing.T) {
	t.Parallel()

	status := users.CREATEUSERSTATUS_CREATE_USER_STATUS_CREATED
	user := testUser("id-a", "a@coralogix.com", "", "", users.USERSTATUS_USER_STATUS_INACTIVE)
	created := &users.CreateUserResult{Username: "a@coralogix.com", Status: &status, User: &user}

	got, err := createdUser(created)
	if err != nil {
		t.Fatalf("createdUser error: %v", err)
	}
	if got.GetUserId() != "id-a" || got.GetStatus() != users.USERSTATUS_USER_STATUS_INACTIVE {
		t.Errorf("user = %#v, want the user from the result", got)
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
		_, err := createdUser(result)
		if err == nil {
			t.Errorf("%s: createdUser accepted status %s", name, tc.status)
			continue
		}
		if !strings.Contains(err.Error(), tc.wantWord) {
			t.Errorf("%s: error = %q, want it to mention %q", name, err, tc.wantWord)
		}
	}

	// A CREATED result without a user is still a failure, because Terraform cannot store
	// an empty id.
	noUser := &users.CreateUserResult{Username: "a@coralogix.com", Status: &status}
	if _, err := createdUser(noUser); err == nil {
		t.Error("createdUser accepted a CREATED result without a user")
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
