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

package provider

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
)

const (
	userIDFormat      = "00000000-0000-4000-8000-%012d"
	userAccountIDBase = 1000
	userNoInvite      = "ONBOARDING_MODE_NO_INVITE"
)

// userFake models the public Users REST contract and the behavior that the handwritten
// resource relies on. Every id, account id, and email is synthetic; no tenant or member data
// is used. It follows the contract of the pinned SDK:
//   - GET searches, with a partial and case-insensitive username filter and an offset page token
//   - POST creates a batch and answers one result per requested user, with a per-user status
//   - PUT replaces the whole template of each user: an omitted login mode list is written as none,
//     and an omitted status as inactive
//   - there is no DELETE and no read by userId
type userFake struct {
	mu       sync.Mutex
	t        *testing.T
	users    []map[string]any
	seq      int
	requests []grRequest

	// searchStatus, createStatus, and updateStatus make that call answer with the status. 0 means normal.
	searchStatus int
	createStatus int
	updateStatus int
	// pageLimit caps the users of a search page below the requested page size. 0 means no cap.
	pageLimit int
	// stuckPageToken is the next page token of every page that has users. 0 means the offset.
	stuckPageToken int
	// foldCase stores every username in lower case, as an SSO login can.
	foldCase bool
	// createResult is the status that every create answers with, without creating the user.
	createResult  string
	createMessage string
	// createMode shapes a create that succeeds. "no-user-id" answers without the userId, "no-result"
	// answers with no result, and "other-username" answers with the result of another username.
	createMode string
	// updateOmitUsers makes a successful update answer without the users.
	updateOmitUsers bool
	// deactivateWithoutResponse deactivates users, then answers without the users field.
	deactivateWithoutResponse bool
}

func newUserFake(t *testing.T) *userFake {
	t.Helper()
	f := &userFake{t: t}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)

	target, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	original := http.DefaultTransport
	http.DefaultTransport = redirectTransport{target: target, next: original}
	t.Cleanup(func() { http.DefaultTransport = original })
	return f
}

// newUserFidelityFake returns a fake with no server, for the fidelity tests that call handle directly.
func newUserFidelityFake() *userFake {
	return &userFake{}
}

func (h *grHarness) users() *userFake {
	h.t.Helper()
	f, ok := h.api.(*userFake)
	if !ok {
		h.t.Fatal("expected *userFake")
	}
	return f
}

// ForgetPrivateState drops the private state of the resource, as after an upgrade from a provider
// version that never stored it. The next write has to search for the login modes.
func (h *grHarness) ForgetPrivateState() { h.private = nil }

// seed stores a user as if it was made outside Terraform. It returns the userId.
func (f *userFake) seed(user map[string]any) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	stored := f.newUser(user)
	return stored["userId"].(string)
}

// omitID removes the id from a stored user, so a scenario can record how callers handle a
// malformed SearchUsers result. It returns false when the fake does not have the user.
func (f *userFake) omitID(id string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	user := f.find(id)
	if user == nil {
		return false
	}
	delete(user, "userId")
	return true
}

// forget removes a user without a request, as if it was removed outside Terraform.
func (f *userFake) forget(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, user := range f.users {
		if user["userId"] == id {
			f.users = append(f.users[:i], f.users[i+1:]...)
			return
		}
	}
}

// change sets fields of a stored user without a request, as if a person changed it elsewhere.
func (f *userFake) change(id string, fields map[string]any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	user := f.find(id)
	if user == nil {
		panic("change of a user the fake does not have: " + id)
	}
	for key, value := range fields {
		user[key] = value
	}
}

func (f *userFake) takeRequests() []grRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.requests
	f.requests = nil
	return out
}

func (f *userFake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	raw, _ := io.ReadAll(r.Body)
	rec := grRequest{Method: r.Method, Path: r.URL.Path}
	if q := r.URL.RawQuery; q != "" {
		rec.Path += "?" + q
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &rec.Body)
	}
	f.checkRequest(r.URL.Query(), rec.Body)
	status, body := f.handle(r.Method, r.URL.Path, r.URL.Query(), rec.Body)
	rec.Status = status
	f.requests = append(f.requests, rec)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// checkRequest fails the test when the provider sends what it must never send: the team comes
// from the API key, and a created user must never get an invitation email.
func (f *userFake) checkRequest(query url.Values, body any) {
	if f.t == nil {
		return
	}
	f.t.Helper()
	if query.Has("team_id") {
		f.t.Errorf("the provider sent team_id %q", query.Get("team_id"))
	}
	var walk func(value any)
	walk = func(value any) {
		switch value := value.(type) {
		case map[string]any:
			for key, item := range value {
				if key == "teamId" {
					f.t.Errorf("the provider sent teamId %v", item)
				}
				walk(item)
			}
		case []any:
			for _, item := range value {
				walk(item)
			}
		}
	}
	walk(body)
}

func (f *userFake) handle(method, path string, query url.Values, body any) (int, any) {
	if !strings.HasSuffix(path, userBasePath) {
		return userFakeError(http.StatusNotFound)
	}
	switch method {
	case http.MethodGet:
		return f.search(query)
	case http.MethodPost:
		return f.create(body)
	case http.MethodPut:
		return f.update(body)
	default:
		return userFakeError(http.StatusNotFound)
	}
}

// userFakeError answers as the API facade does. A 404 from a route that does not exist has the generic
// message "Not Found: Not Found", which says nothing about a user; a missing user has its own message.
func userFakeError(status int) (int, any) {
	message := http.StatusText(status)
	if status == http.StatusNotFound {
		message = "Not Found: Not Found"
	}
	return status, map[string]any{"code": status, "message": message}
}

// search matches the username as a case-insensitive part of the stored username, and pages with
// an offset token. The last page has no next page token.
func (f *userFake) search(query url.Values) (int, any) {
	if f.searchStatus != 0 {
		return userFakeError(f.searchStatus)
	}
	filter := strings.ToLower(query.Get("username"))
	matches := make([]map[string]any, 0, len(f.users))
	for _, user := range f.users {
		if strings.Contains(strings.ToLower(user["username"].(string)), filter) {
			matches = append(matches, user)
		}
	}

	size := len(matches)
	if requested, err := strconv.Atoi(query.Get("page_size")); err == nil && requested > 0 && requested < size {
		size = requested
	}
	if f.pageLimit > 0 && f.pageLimit < size {
		size = f.pageLimit
	}
	offset, _ := strconv.Atoi(query.Get("page_token"))
	if offset > len(matches) {
		offset = len(matches)
	}
	end := min(offset+size, len(matches))

	page := make([]any, 0, end-offset)
	for _, user := range matches[offset:end] {
		page = append(page, copyUser(user))
	}
	response := map[string]any{"users": page, "totalCount": len(matches)}
	if end < len(matches) {
		response["nextPageToken"] = end
	}
	if f.stuckPageToken != 0 && len(page) > 0 {
		response["nextPageToken"] = f.stuckPageToken
	}
	return http.StatusOK, response
}

func (f *userFake) create(body any) (int, any) {
	if f.createStatus != 0 {
		return userFakeError(f.createStatus)
	}
	requests, _ := body.([]any)
	results := make([]any, 0, len(requests))
	for _, raw := range requests {
		request := asObject(raw)
		template := asObject(request["userTemplate"])
		username, _ := template["username"].(string)
		if f.t != nil && request["onboardingMode"] != userNoInvite {
			f.t.Errorf("the provider created %q without ONBOARDING_MODE_NO_INVITE: %v", username, request["onboardingMode"])
		}
		if result := f.createResultFor(username, template); result != nil {
			results = append(results, result)
		}
	}
	return http.StatusOK, map[string]any{"results": results}
}

func (f *userFake) createResultFor(username string, template map[string]any) any {
	if f.createResult != "" {
		result := map[string]any{"status": f.createResult, "username": username}
		if f.createMessage != "" {
			result["message"] = f.createMessage
		}
		return result
	}
	for _, user := range f.users {
		if strings.EqualFold(user["username"].(string), username) {
			return map[string]any{
				"status":   "CREATE_USER_STATUS_ALREADY_EXISTS",
				"username": username,
				"message":  "user already exists",
			}
		}
	}

	user := f.newUser(templateUser(template, username))
	switch f.createMode {
	case "no-user-id":
		return map[string]any{"status": "CREATE_USER_STATUS_CREATED", "username": username, "user": map[string]any{"username": username}}
	case "no-result":
		return nil
	case "other-username":
		return map[string]any{"status": "CREATE_USER_STATUS_CREATED", "username": "tf-acc-user-other@coralogix.com", "user": copyUser(user)}
	}
	return map[string]any{
		"status":        "CREATE_USER_STATUS_CREATED",
		"username":      username,
		"userId":        user["userId"],
		"userAccountId": user["userAccountId"],
		"user":          copyUser(user),
	}
}

// templateUser turns a create template into the fields of a user.
func templateUser(template map[string]any, username string) map[string]any {
	user := map[string]any{"username": username}
	for _, key := range []string{"firstName", "lastName", "status", "allowedLoginMode", "accessType"} {
		if value, ok := template[key]; ok {
			user[key] = value
		}
	}
	return user
}

// update replaces the whole template of each user, as the real PUT does: what the template leaves out
// is cleared, and an omitted status is inactive. The username does not change.
func (f *userFake) update(body any) (int, any) {
	if f.updateStatus != 0 {
		return userFakeError(f.updateStatus)
	}
	requests, _ := body.([]any)
	var accountIDs []any
	var updated []any
	for _, raw := range requests {
		request := asObject(raw)
		user := f.findRequested(request)
		if user == nil {
			return http.StatusNotFound, map[string]any{"code": http.StatusNotFound, "message": "Not Found: user not found"}
		}
		template := asObject(request["userTemplate"])
		user["firstName"], _ = template["firstName"].(string)
		user["lastName"], _ = template["lastName"].(string)
		user["status"] = "USER_STATUS_INACTIVE"
		if status, ok := template["status"]; ok {
			user["status"] = status
		}
		user["allowedLoginMode"] = []any{}
		if modes, ok := template["allowedLoginMode"]; ok {
			user["allowedLoginMode"] = modes
		}
		delete(user, "accessType")
		if access, ok := template["accessType"]; ok {
			user["accessType"] = access
		}
		accountIDs = append(accountIDs, user["userAccountId"])
		updated = append(updated, copyUser(user))
	}
	if f.deactivateWithoutResponse {
		return http.StatusOK, map[string]any{"userAccountIds": accountIDs}
	}
	if f.updateOmitUsers {
		return http.StatusOK, map[string]any{}
	}
	return http.StatusOK, map[string]any{"userAccountIds": accountIDs, "users": updated}
}

func (f *userFake) findRequested(request map[string]any) map[string]any {
	if id, ok := request["userId"].(string); ok {
		return f.find(id)
	}
	if account, ok := request["userAccountId"].(float64); ok {
		for _, user := range f.users {
			if float64(user["userAccountId"].(int)) == account {
				return user
			}
		}
	}
	return nil
}

func (f *userFake) find(id string) map[string]any {
	for _, user := range f.users {
		if user["userId"] == id {
			return user
		}
	}
	return nil
}

// newUser stores a user with the fields of input and the defaults the API gives: an empty name, an
// active status, and no login modes, access type, or groups.
func (f *userFake) newUser(input map[string]any) map[string]any {
	f.seq++
	user := map[string]any{
		"userId":           fmt.Sprintf(userIDFormat, f.seq),
		"userAccountId":    userAccountIDBase + f.seq,
		"firstName":        "",
		"lastName":         "",
		"status":           "USER_STATUS_ACTIVE",
		"allowedLoginMode": []any{},
		"groupIds":         []any{},
	}
	for key, value := range input {
		user[key] = value
	}
	if username, ok := user["username"].(string); ok && f.foldCase {
		user["username"] = strings.ToLower(username)
	}
	f.users = append(f.users, user)
	return user
}

func copyUser(user map[string]any) map[string]any {
	encoded, _ := json.Marshal(user)
	var out map[string]any
	_ = json.Unmarshal(encoded, &out)
	return out
}
