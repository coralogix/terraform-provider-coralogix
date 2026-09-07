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

	teamGroups "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/team_groups_management_service"
	users "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/users_management_service"
)

// testTeamID is an arbitrary team id. The tests only check that it reaches the URL.
const testTeamID = int64(1)

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

func newTeamGroupsClient(t *testing.T, handler http.HandlerFunc) *teamGroups.TeamGroupsManagementServiceAPIService {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	cfg := teamGroups.NewConfiguration()
	cfg.Servers = teamGroups.ServerConfigurations{{URL: server.URL}}
	return teamGroups.NewAPIClient(cfg).TeamGroupsManagementServiceAPI
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
		if got := r.URL.Path; got != fmt.Sprintf("/aaa/teams/v2/%d/search", testTeamID) {
			t.Errorf("path = %q", got)
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

	found, err := searchUsers(context.Background(), client, testTeamID, "")
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

	found, err := searchUsers(context.Background(), client, testTeamID, "")
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

	user, err := findUserByID(context.Background(), client, testTeamID, "id-1", "id-1@coralogix.com")
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

	user, err := findUserByID(context.Background(), client, testTeamID, "id-1", "stale@coralogix.com")
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

	_, err := findUserByID(context.Background(), client, testTeamID, "id-gone", "")
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

	_, err := findUserByID(context.Background(), client, testTeamID, "id-1", "")
	if err == nil {
		t.Fatal("findUserByID returned no error on a 500")
	}
	if isUserNotFoundErr(err) {
		t.Errorf("error = %v, a 500 must not be treated as not found", err)
	}
}

// The Users API does not report memberships, so `groups` is rebuilt by reading every
// group's member list. Both the group list and each member list are paginated.
func TestListUserGroupIDsPagesGroupsAndMembers(t *testing.T) {
	t.Parallel()

	memberRequests := map[string]int{}
	client := newTeamGroupsClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/aaa/team-groups/v2":
			if got := r.URL.Query().Get("team_id"); got != fmt.Sprintf("%d", testTeamID) {
				t.Errorf("team_id = %q", got)
			}
			if r.URL.Query().Get("page_token") == "" {
				writeJSON(t, w, map[string]any{
					"groups":        []map[string]any{{"groupId": 1}, {"groupId": 2}},
					"nextPageToken": "second",
				})
				return
			}
			writeJSON(t, w, map[string]any{"groups": []map[string]any{{"groupId": 3}}})

		case strings.HasSuffix(r.URL.Path, "/users/list"):
			groupID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/aaa/team-groups/v2/"), "/users/list")
			memberRequests[groupID]++
			switch groupID {
			case "1":
				// Group 1 holds the user on its second member page.
				if r.URL.Query().Get("page_token") == "" {
					writeJSON(t, w, map[string]any{
						"users":         []map[string]any{{"userId": "someone-else"}},
						"nextPageToken": "more",
					})
					return
				}
				writeJSON(t, w, map[string]any{"users": []map[string]any{{"userId": "id-1"}}})
			case "2":
				writeJSON(t, w, map[string]any{"users": []map[string]any{{"userId": "someone-else"}}})
			case "3":
				writeJSON(t, w, map[string]any{"users": []map[string]any{{"userId": "id-1"}}})
			default:
				t.Errorf("unexpected group %q", groupID)
			}

		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	})

	groupIDs, err := listUserGroupIDs(context.Background(), client, testTeamID, "id-1")
	if err != nil {
		t.Fatalf("listUserGroupIDs error: %v", err)
	}
	if strings.Join(groupIDs, ",") != "1,3" {
		t.Errorf("groups = %v, want the two groups holding the user", groupIDs)
	}
	// One request per group, plus the extra page group 1 needed. The cost must not
	// grow with the number of users in the team.
	if memberRequests["1"] != 2 || memberRequests["2"] != 1 || memberRequests["3"] != 1 {
		t.Errorf("member requests = %v", memberRequests)
	}
}

// A user in no group gets an empty result, which flattenUser turns into a known empty
// set rather than a null one.
func TestListUserGroupIDsWithNoMemberships(t *testing.T) {
	t.Parallel()

	client := newTeamGroupsClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/aaa/team-groups/v2" {
			writeJSON(t, w, map[string]any{"groups": []map[string]any{{"groupId": 1}}})
			return
		}
		writeJSON(t, w, map[string]any{"users": []map[string]any{{"userId": "someone-else"}}})
	})

	groupIDs, err := listUserGroupIDs(context.Background(), client, testTeamID, "id-1")
	if err != nil {
		t.Fatalf("listUserGroupIDs error: %v", err)
	}
	if len(groupIDs) != 0 {
		t.Errorf("groups = %v, want none", groupIDs)
	}

	state, diags := flattenUser(context.Background(), &users.RbacV2User{
		UserId:   ptrTo("id-1"),
		Username: ptrTo("id-1@coralogix.com"),
		Status:   ptrTo(users.USERSTATUS_USER_STATUS_ACTIVE),
	}, groupIDs)
	if diags.HasError() {
		t.Fatalf("flattenUser diagnostics: %v", diags)
	}
	if state.Groups.IsNull() {
		t.Error("groups is null, want a known empty set")
	}
}

// A failure while reading one group's members must not silently produce a short group
// list, which would look like the user was removed from a group.
func TestListUserGroupIDsSurfacesMemberReadErrors(t *testing.T) {
	t.Parallel()

	client := newTeamGroupsClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/aaa/team-groups/v2" {
			writeJSON(t, w, map[string]any{"groups": []map[string]any{{"groupId": 1}}})
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(t, w, map[string]any{"code": 500, "message": "boom"})
	})

	if _, err := listUserGroupIDs(context.Background(), client, testTeamID, "id-1"); err == nil {
		t.Error("listUserGroupIDs returned no error on a 500")
	}
}

func ptrTo[T any](v T) *T { return &v }
