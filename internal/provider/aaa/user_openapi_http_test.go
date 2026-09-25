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
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	users "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/users_management_service"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// newUsersClient points a generated Users client at a local test server, so the
// pagination and matching loops run against real HTTP without touching a backend.
func newUsersClient(t *testing.T, handler http.HandlerFunc) *users.UsersManagementServiceAPIService {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	cfg := users.NewConfiguration()
	cfg.Servers = users.ServerConfigurations{{URL: server.URL}}
	return users.NewAPIClient(cfg).UsersManagementServiceAPI
}

func writeJSON(t *testing.T, w http.ResponseWriter, body any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(body); err != nil {
		t.Errorf("encoding response: %v", err)
	}
}

func searchPage(t *testing.T, w http.ResponseWriter, userIDs []string, nextPageToken int64) {
	t.Helper()
	page := map[string]any{"users": []map[string]any{}}
	list := make([]map[string]any, 0, len(userIDs))
	for _, id := range userIDs {
		list = append(list, map[string]any{
			"userId":        id,
			"userAccountId": 1,
			"username":      fmt.Sprintf("%s@coralogix.com", id),
			"status":        "USER_STATUS_ACTIVE",
		})
	}
	page["users"] = list
	if nextPageToken != 0 {
		page["nextPageToken"] = nextPageToken
	}
	writeJSON(t, w, page)
}

// SearchUsers pages by offset, so every page after the first has to carry the token the
// previous page returned. A user on the last page must still be found.
func TestSearchUsersFollowsEveryPage(t *testing.T) {
	t.Parallel()

	var seenTokens []string
	client := newUsersClient(t, func(w http.ResponseWriter, r *http.Request) {
		// The team comes from the API key, so no team id may reach the request.
		if got := r.URL.Path; got != "/aaa/users/v2" {
			t.Errorf("path = %q", got)
		}
		if r.URL.Query().Has("team_id") {
			t.Errorf("team_id = %q, want absent", r.URL.Query().Get("team_id"))
		}
		if got := r.URL.Query().Get("page_size"); got != "100" {
			t.Errorf("page_size = %q, want 100", got)
		}
		token := r.URL.Query().Get("page_token")
		seenTokens = append(seenTokens, token)

		switch token {
		case "":
			searchPage(t, w, []string{"id-1", "id-2"}, 2)
		case "2":
			searchPage(t, w, []string{"id-3", "id-4"}, 4)
		case "4":
			searchPage(t, w, []string{"id-5"}, 0)
		default:
			t.Errorf("unexpected page_token %q", token)
		}
	})

	found, err := searchUsers(context.Background(), client, "")
	if err != nil {
		t.Fatalf("searchUsers error: %v", err)
	}
	if got := userIDs(found); strings.Join(got, ",") != "id-1,id-2,id-3,id-4,id-5" {
		t.Errorf("collected ids = %v, want every page", got)
	}
	if len(seenTokens) != 3 {
		t.Errorf("requests = %v, want three pages", seenTokens)
	}
}

// A backend that keeps returning the same offset must not spin forever.
func TestSearchUsersStopsWhenTheTokenDoesNotAdvance(t *testing.T) {
	t.Parallel()

	requests := 0
	client := newUsersClient(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests > 5 {
			t.Fatal("searchUsers did not stop on a page token that repeats")
		}
		// The first page advances the offset to 2. Every page after that answers with
		// the same offset, which points back at the page just served.
		if r.URL.Query().Get("page_token") == "" {
			searchPage(t, w, []string{"id-1"}, 2)
			return
		}
		searchPage(t, w, []string{"id-2"}, 2)
	})

	found, err := searchUsers(context.Background(), client, "")
	if err != nil {
		t.Fatalf("searchUsers error: %v", err)
	}
	if requests != 2 {
		t.Errorf("requests = %d, want the loop to stop after the repeat", requests)
	}
	if got := userIDs(found); strings.Join(got, ",") != "id-1,id-2" {
		t.Errorf("collected ids = %v", got)
	}
}

// A username in state narrows the first attempt to a filtered request. The full scan
// only happens when the filter does not produce the user.
func TestFindUserByIDUsesTheUsernameHintFirst(t *testing.T) {
	t.Parallel()

	var filters []string
	client := newUsersClient(t, func(w http.ResponseWriter, r *http.Request) {
		filters = append(filters, r.URL.Query().Get("username"))
		searchPage(t, w, []string{"id-1"}, 0)
	})

	user, err := findUserByID(context.Background(), client, "id-1", "id-1@coralogix.com")
	if err != nil {
		t.Fatalf("findUserByID error: %v", err)
	}
	if user.GetUserId() != "id-1" {
		t.Errorf("userId = %q", user.GetUserId())
	}
	if len(filters) != 1 || filters[0] != "id-1@coralogix.com" {
		t.Errorf("requests = %v, want one filtered request", filters)
	}
}

// Import has no username, and a stale username in state must not hide the user. Both
// cases fall back to a full scan.
func TestFindUserByIDFallsBackToAFullScan(t *testing.T) {
	t.Parallel()

	var filters []string
	client := newUsersClient(t, func(w http.ResponseWriter, r *http.Request) {
		filter := r.URL.Query().Get("username")
		filters = append(filters, filter)
		if filter != "" {
			// The filtered request answers with a different user, as a renamed account
			// would.
			searchPage(t, w, []string{"id-other"}, 0)
			return
		}
		searchPage(t, w, []string{"id-1"}, 0)
	})

	user, err := findUserByID(context.Background(), client, "id-1", "stale@coralogix.com")
	if err != nil {
		t.Fatalf("findUserByID error: %v", err)
	}
	if user.GetUserId() != "id-1" {
		t.Errorf("userId = %q", user.GetUserId())
	}
	if len(filters) != 2 || filters[0] == "" || filters[1] != "" {
		t.Errorf("requests = %v, want the filtered attempt then the full scan", filters)
	}
}

// A user the backend no longer has must produce the sentinel, so Read removes the
// resource instead of failing the apply.
func TestFindUserByIDReportsNotFound(t *testing.T) {
	t.Parallel()

	client := newUsersClient(t, func(w http.ResponseWriter, r *http.Request) {
		searchPage(t, w, []string{"id-other"}, 0)
	})

	_, err := findUserByID(context.Background(), client, "id-gone", "")
	if !isUserNotFoundErr(err) {
		t.Errorf("error = %v, want the not-found sentinel", err)
	}
}

// A backend failure must surface as an error, never as an empty result that would look
// like a deleted user and drop the resource from state.
func TestSearchUsersSurfacesBackendErrors(t *testing.T) {
	t.Parallel()

	client := newUsersClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(t, w, map[string]any{"code": 500, "message": "boom"})
	})

	_, err := findUserByID(context.Background(), client, "id-1", "")
	if err == nil {
		t.Fatal("findUserByID returned no error on a 500")
	}
	if isUserNotFoundErr(err) {
		t.Errorf("error = %v, a 500 must not be treated as not found", err)
	}
}

// decodeBody reads a JSON request body into a generic value, so the tests see exactly
// what went over the wire, including fields the SDK leaves out.
func decodeBody(t *testing.T, r *http.Request) []map[string]any {
	t.Helper()
	var body []map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		t.Fatalf("decoding request body: %v", err)
	}
	return body
}

// PUT replaces the whole template, so an update has to carry the names, the status and
// the echoed login modes and access type, and address the user by userId only.
func TestPutUserSendsTheFullTemplate(t *testing.T) {
	t.Parallel()

	accessType := users.NewAccessType("permanent")
	accessType.PermanentAccess = map[string]any{}
	echo := userEcho{
		AllowedLoginMode: []users.AllowedLoginMode{users.ALLOWEDLOGINMODE_ALLOWED_LOGIN_MODE_SSO},
		AccessType:       accessType,
	}

	var sent []map[string]any
	client := newUsersClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/aaa/users/v2" {
			t.Errorf("request = %s %s, want PUT /aaa/users/v2", r.Method, r.URL.Path)
		}
		sent = decodeBody(t, r)
		writeJSON(t, w, map[string]any{"users": []map[string]any{{
			"userId": "id-1", "username": "a@coralogix.com", "status": "USER_STATUS_INACTIVE",
		}}})
	})

	name := &UserNameModel{GivenName: types.StringValue("Given"), FamilyName: types.StringNull()}
	user, err := putUser(context.Background(), client, "id-1", updateUserTemplate(name, false, echo))
	if err != nil {
		t.Fatalf("putUser error: %v", err)
	}
	if user.GetUserId() != "id-1" {
		t.Errorf("userId = %q", user.GetUserId())
	}

	if len(sent) != 1 {
		t.Fatalf("body = %v, want one update", sent)
	}
	if sent[0]["userId"] != "id-1" {
		t.Errorf("userId = %v", sent[0]["userId"])
	}
	if _, ok := sent[0]["userAccountId"]; ok {
		t.Error("userAccountId was sent, but it is mutually exclusive with userId")
	}
	template := sent[0]["userTemplate"].(map[string]any)
	want := map[string]any{
		"firstName":        "Given",
		"lastName":         "",
		"status":           "USER_STATUS_INACTIVE",
		"allowedLoginMode": []any{"ALLOWED_LOGIN_MODE_SSO"},
		"accessType":       map[string]any{"accessType": "permanent", "permanentAccess": map[string]any{}},
	}
	for key, value := range want {
		if got, _ := json.Marshal(template[key]); string(got) != mustJSON(t, value) {
			t.Errorf("userTemplate.%s = %s, want %s", key, got, mustJSON(t, value))
		}
	}
	if _, ok := template["username"]; ok {
		t.Error("username was sent, but the API ignores it on update")
	}
}

// An unknown userId is a 404, which has to become the not-found sentinel so Update
// drops the resource and Delete treats it as done.
func TestPutUserMapsNotFound(t *testing.T) {
	t.Parallel()

	client := newUsersClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		// The exact body EU2 returned for an unknown userId.
		writeJSON(t, w, map[string]any{"code": 404, "message": "Not Found: user id-gone not found in team 1"})
	})

	_, err := putUser(context.Background(), client, "id-gone", updateUserTemplate(nil, false, userEcho{}))
	if !isUserNotFoundErr(err) {
		t.Errorf("error = %v, want the not-found sentinel", err)
	}
}

// Create sends no login mode, so a new user gets none, and it sends the planned status
// so `active = false` needs no second call.
func TestCreateUsersRequestBody(t *testing.T) {
	t.Parallel()

	var sent []map[string]any
	client := newUsersClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/aaa/users/v2" || r.URL.Query().Has("team_id") {
			t.Errorf("request = %s %s, want POST /aaa/users/v2 without team_id", r.Method, r.URL)
		}
		sent = decodeBody(t, r)
		writeJSON(t, w, map[string]any{"results": []map[string]any{}})
	})

	_, _, err := client.UsersMgmtServiceCreateUsers(context.Background()).
		CreateUserRequest([]users.CreateUserRequest{{
			OnboardingMode: ptrTo(users.ONBOARDINGMODE_ONBOARDING_MODE_NO_INVITE),
			UserTemplate:   createUserTemplate("a@coralogix.com", nil, false),
		}}).Execute()
	if err != nil {
		t.Fatalf("create error: %v", err)
	}

	if sent[0]["onboardingMode"] != "ONBOARDING_MODE_NO_INVITE" {
		t.Errorf("onboardingMode = %v", sent[0]["onboardingMode"])
	}
	template := sent[0]["userTemplate"].(map[string]any)
	if _, ok := template["allowedLoginMode"]; ok {
		t.Errorf("allowedLoginMode = %v, want absent", template["allowedLoginMode"])
	}
	if template["status"] != "USER_STATUS_INACTIVE" {
		t.Errorf("status = %v, want USER_STATUS_INACTIVE", template["status"])
	}
}

// importUser runs ImportState on an empty state, the way Terraform starts an import.
func importUser(t *testing.T, r *UserResource, id string) *resource.ImportStateResponse {
	t.Helper()
	ctx := context.Background()

	var schemaResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)
	resp := &resource.ImportStateResponse{State: tfsdk.State{
		Schema: schemaResp.Schema,
		Raw:    tftypes.NewValue(schemaResp.Schema.Type().TerraformType(ctx), nil),
	}}
	r.ImportState(ctx, resource.ImportStateRequest{ID: id}, resp)
	return resp
}

func importedString(t *testing.T, resp *resource.ImportStateResponse, attribute string) string {
	t.Helper()
	var value types.String
	if diags := resp.State.GetAttribute(context.Background(), path.Root(attribute), &value); diags.HasError() {
		t.Fatalf("reading %s: %v", attribute, diags)
	}
	return value.ValueString()
}

// An email is resolved with one filtered search, and state gets the UUID as id, the
// same as an import by id.
func TestUserImportStateByEmail(t *testing.T) {
	t.Parallel()

	var filters []string
	client := newUsersClient(t, func(w http.ResponseWriter, r *http.Request) {
		filters = append(filters, r.URL.Query().Get("username"))
		writeJSON(t, w, map[string]any{"users": []map[string]any{{
			"userId": "476b96bc-0f2e-42dd-b038-186bc1121b73", "username": "A@coralogix.com",
		}}})
	})

	resp := importUser(t, &UserResource{client: client}, "a@coralogix.com")
	if resp.Diagnostics.HasError() {
		t.Fatalf("ImportState diagnostics: %v", resp.Diagnostics)
	}
	if len(filters) != 1 || filters[0] != "a@coralogix.com" {
		t.Errorf("requests = %v, want one search filtered by the email", filters)
	}
	if got := importedString(t, resp, "id"); got != "476b96bc-0f2e-42dd-b038-186bc1121b73" {
		t.Errorf("id = %q, want the UUID", got)
	}
	if got := importedString(t, resp, "user_name"); got != "A@coralogix.com" {
		t.Errorf("user_name = %q, want the backend spelling", got)
	}
}

// A UUID is passed through unchanged and makes no call. Read resolves it later.
func TestUserImportStateByID(t *testing.T) {
	t.Parallel()

	client := newUsersClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request %s %s", r.Method, r.URL)
	})

	resp := importUser(t, &UserResource{client: client}, "476b96bc-0f2e-42dd-b038-186bc1121b73")
	if resp.Diagnostics.HasError() {
		t.Fatalf("ImportState diagnostics: %v", resp.Diagnostics)
	}
	if got := importedString(t, resp, "id"); got != "476b96bc-0f2e-42dd-b038-186bc1121b73" {
		t.Errorf("id = %q", got)
	}
}

// An email that matches nobody fails the import instead of writing an empty id.
func TestUserImportStateByEmailNotFound(t *testing.T) {
	t.Parallel()

	client := newUsersClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]any{"users": []map[string]any{}})
	})

	resp := importUser(t, &UserResource{client: client}, "gone@coralogix.com")
	if !resp.Diagnostics.HasError() {
		t.Fatal("ImportState accepted an email that matches no user")
	}
	if got := resp.Diagnostics.Errors()[0].Summary(); !strings.Contains(got, "not found") {
		t.Errorf("error = %q, want not found", got)
	}
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("encoding %v: %v", value, err)
	}
	return string(raw)
}

func ptrTo[T any](v T) *T { return &v }
