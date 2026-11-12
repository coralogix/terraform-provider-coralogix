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

// customRoleFirstID is the first id that the fake gives a custom role. Ids are synthetic.
const customRoleFirstID = 5001

// customRoleFakeTeamID is the team of every role in the fake. The real API puts it in each role.
const customRoleFakeTeamID = 700001

// customRoleFake models the custom roles REST API as recorded on a real tenant
// (fidelity/recorded.json):
//   - Create is a PUT of the collection, and Update is a POST of the role. Create returns only
//     {"id":N}, Update and Delete return {}, and Get wraps the role in {"role":{...}}.
//   - A parent role is required, by name (case-sensitive) or by id, and not both.
//   - Every permission must be a known one, in any case. The API keeps the case that it was given
//     and one of each permission, whatever the case.
//   - Names are unique in a team, and the empty name is a name: Create and Update accept it, and
//     a second role with it answers 409. A request without a name has the empty name. Update to the
//     name that the role has is no conflict.
//   - Get returns the permissions of the role. The list returns them together with the permissions
//     of the parent role.
//   - An id that does not exist is 404 on Get, Update, and Delete.
type customRoleFake struct {
	mu       sync.Mutex
	roles    map[int64]*customRoleRecord
	nextID   int64
	requests []grRequest
	// createStatus, getStatus, listStatus, updateStatus, and deleteStatus force an answer of the
	// next calls.
	createStatus, getStatus, listStatus, updateStatus, deleteStatus int
}

type customRoleRecord struct {
	name        string
	description string
	parentID    int64
	permissions []string
}

// customRoleSystemRoles are the parent roles that the fake knows, by name. The ids are those of
// the system roles of the real API.
var customRoleSystemRoles = map[string]int64{
	"Standard User":  103,
	"Read-Only User": 104,
	"No-Access User": 17,
}

// customRoleKnownPermissions are the permissions that the fake accepts, in any case.
var customRoleKnownPermissions = map[string]bool{
	"spans.events2metrics:updateconfig": true,
	"spans.events2metrics:readconfig":   true,
	"alerts:readconfig":                 true,
	"alerts:updateconfig":               true,
}

// customRoleParentPermissions are the permissions of the parent roles. The real list adds them to
// the permissions of each role; the fake has twenty-two of them, as many as the recording needs to
// tell them apart.
func customRoleParentPermissions() []string {
	out := make([]string, 0, 22)
	for i := 1; i <= 22; i++ {
		out = append(out, fmt.Sprintf("parent-area-%d:Read", i))
	}
	return out
}

func newCustomRoleFidelityFake() *customRoleFake {
	return &customRoleFake{roles: map[int64]*customRoleRecord{}, nextID: customRoleFirstID}
}

func newCustomRoleFake(t *testing.T) *customRoleFake {
	t.Helper()
	f := newCustomRoleFidelityFake()
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

func (h *grHarness) customRoles() *customRoleFake {
	h.t.Helper()
	f, ok := h.api.(*customRoleFake)
	if !ok {
		h.t.Fatal("expected *customRoleFake")
	}
	return f
}

func (f *customRoleFake) takeRequests() []grRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.requests
	f.requests = nil
	return out
}

// seed adds a role of the parent "Standard User" as if it was created outside Terraform and
// returns its id.
func (f *customRoleFake) seed(name string, permissions []string) int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := f.nextID
	f.nextID++
	f.roles[id] = &customRoleRecord{name: name, description: "seeded", parentID: customRoleSystemRoles["Standard User"], permissions: permissions}
	return id
}

// seedDuplicate adds a second role with the name of another one. The real API refuses it, but a
// role can have the name of one that was renamed on another team.
func (f *customRoleFake) seedDuplicate(name string) int64 {
	return f.seed(name, nil)
}

// forgetOutside removes a role, so the API answers 404 for it.
func (f *customRoleFake) forgetOutside(id int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.roles, id)
}

func (f *customRoleFake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	raw, _ := io.ReadAll(r.Body)
	rec := grRequest{Method: r.Method, Path: r.URL.Path}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &rec.Body)
	}
	status, body := f.handle(r.Method, r.URL.Path, rec.Body)
	rec.Status = status
	f.requests = append(f.requests, rec)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func (f *customRoleFake) handle(method, path string, body any) (int, any) {
	i := strings.Index(path, customRoleBasePath)
	if i < 0 {
		return customRoleError(http.StatusNotFound, "Not Found")
	}
	rest := strings.TrimPrefix(path[i+len(customRoleBasePath):], "/")
	if rest == "" {
		switch method {
		case http.MethodPut:
			return f.create(asObject(body))
		case http.MethodGet:
			return f.list()
		default:
			return customRoleError(http.StatusNotFound, "Not Found")
		}
	}
	id, err := strconv.ParseUint(rest, 10, 32)
	if err != nil {
		return customRoleError(http.StatusBadRequest, fmt.Sprintf("Bad Request: type mismatch, parameter: role_id, error: strconv.ParseUint: parsing %q: invalid syntax", rest))
	}
	switch method {
	case http.MethodGet:
		return f.get(int64(id))
	case http.MethodPost:
		return f.update(int64(id), asObject(body))
	case http.MethodDelete:
		return f.delete(int64(id))
	default:
		return customRoleError(http.StatusNotFound, "Not Found")
	}
}

func (f *customRoleFake) create(request map[string]any) (int, any) {
	if f.createStatus != 0 {
		return customRoleError(f.createStatus, http.StatusText(f.createStatus))
	}
	name, _ := request["name"].(string)
	var parent int64
	switch byID, hasID := request["parentRoleId"]; {
	case hasID && request["parentRoleName"] != nil:
		return customRoleError(http.StatusBadRequest, fmt.Sprintf("Bad Request: proto: (line 1:%d): error parsing \"parentRoleName\", oneof com.coralogixapis.aaa.rbac.v2.CreateRoleRequest.parent_role is already set", customRoleKeyColumn(request, "parentRoleName")))
	case hasID:
		parent = int64(byID.(float64))
		if !customRoleSystemRoleExists(parent) {
			return customRoleError(http.StatusBadRequest, "Bad Request: Parent role not found")
		}
	case request["parentRoleName"] != nil:
		parentName, _ := request["parentRoleName"].(string)
		id, ok := customRoleSystemRoles[parentName]
		if !ok {
			return customRoleError(http.StatusBadRequest, "Bad Request: Parent role not found")
		}
		parent = id
	default:
		return customRoleError(http.StatusBadRequest, "Bad Request: Parent role is required")
	}
	permissions, bad, ok := customRolePermissions(request["permissions"])
	if !ok {
		return customRoleError(http.StatusBadRequest, "Bad Request: Expression is not valid: "+bad)
	}
	if f.nameTaken(name, 0) {
		return customRoleError(http.StatusConflict, fmt.Sprintf("Already Exists: Could not create custom role %s for team %d as it already exists.", name, customRoleFakeTeamID))
	}
	description, _ := request["description"].(string)
	id := f.nextID
	f.nextID++
	f.roles[id] = &customRoleRecord{name: name, description: description, parentID: parent, permissions: permissions}
	return http.StatusOK, map[string]any{"id": id}
}

func (f *customRoleFake) get(id int64) (int, any) {
	if f.getStatus != 0 {
		return customRoleError(f.getStatus, http.StatusText(f.getStatus))
	}
	role, ok := f.roles[id]
	if !ok {
		return customRoleError(http.StatusNotFound, "Not Found: Role not found")
	}
	return http.StatusOK, map[string]any{"role": f.render(id, role, false)}
}

func (f *customRoleFake) list() (int, any) {
	if f.listStatus != 0 {
		return customRoleError(f.listStatus, http.StatusText(f.listStatus))
	}
	roles := []any{}
	for id := int64(customRoleFirstID); id < f.nextID; id++ {
		if role, ok := f.roles[id]; ok {
			roles = append(roles, f.render(id, role, true))
		}
	}
	return http.StatusOK, map[string]any{"roles": roles}
}

// render is a role as the API shows it. The list adds the permissions of the parent role.
func (f *customRoleFake) render(id int64, role *customRoleRecord, inherit bool) map[string]any {
	permissions := append([]string{}, role.permissions...)
	if inherit {
		permissions = append(permissions, customRoleParentPermissions()...)
	}
	parentName := ""
	for name, parentID := range customRoleSystemRoles {
		if parentID == role.parentID {
			parentName = name
		}
	}
	return map[string]any{
		"roleId":         id,
		"teamId":         customRoleFakeTeamID,
		"name":           role.name,
		"description":    role.description,
		"parentRoleId":   role.parentID,
		"permissions":    permissions,
		"parentRoleName": parentName,
	}
}

func (f *customRoleFake) update(id int64, request map[string]any) (int, any) {
	if f.updateStatus != 0 {
		return customRoleError(f.updateStatus, http.StatusText(f.updateStatus))
	}
	role, ok := f.roles[id]
	if !ok {
		return customRoleError(http.StatusNotFound, "Not Found: Role not found")
	}
	name, setName := request["newName"].(string)
	if setName && f.nameTaken(name, id) {
		return customRoleError(http.StatusConflict, fmt.Sprintf("Already Exists: Could not update name of custom role %s to %s for team %d as it already exists.", role.name, name, customRoleFakeTeamID))
	}
	var permissions []string
	wrapper, setPermissions := request["newPermissions"].(map[string]any)
	if setPermissions {
		var bad string
		var valid bool
		if permissions, bad, valid = customRolePermissions(wrapper["permissions"]); !valid {
			return customRoleError(http.StatusBadRequest, "Bad Request: Expression is not valid: "+bad)
		}
	}
	if setName {
		role.name = name
	}
	if description, ok := request["newDescription"].(string); ok {
		role.description = description
	}
	if setPermissions {
		role.permissions = permissions
	}
	return http.StatusOK, map[string]any{}
}

func (f *customRoleFake) delete(id int64) (int, any) {
	if f.deleteStatus != 0 {
		return customRoleError(f.deleteStatus, http.StatusText(f.deleteStatus))
	}
	if _, ok := f.roles[id]; !ok {
		return customRoleError(http.StatusNotFound, "Not Found: Role not found")
	}
	delete(f.roles, id)
	return http.StatusOK, map[string]any{}
}

func (f *customRoleFake) nameTaken(name string, except int64) bool {
	for id, role := range f.roles {
		if id != except && role.name == name {
			return true
		}
	}
	return false
}

// customRolePermissions reads the permissions of a request and returns the first one that the API
// does not know, and false when there is one. The API keeps the case that it was given.
func customRolePermissions(value any) ([]string, string, bool) {
	out := []string{}
	seen := map[string]int{}
	for _, item := range asSlice(value) {
		permission, _ := item.(string)
		if !customRoleKnownPermissions[strings.ToLower(permission)] {
			return nil, permission, false
		}
		// The API keeps one of each permission, whatever the case: of the twins it keeps the
		// first in ASCII order (an upper case letter comes before a lower case one).
		key := strings.ToLower(permission)
		if at, twin := seen[key]; twin {
			if permission < out[at] {
				out[at] = permission
			}
			continue
		}
		seen[key] = len(out)
		out = append(out, permission)
	}
	return out, "", true
}

// customRoleKeyColumn is the column at which the real API reports a key of the request: the
// position of the key in the JSON that the client sends, with the keys sorted, as Go writes a map.
func customRoleKeyColumn(request map[string]any, key string) int {
	raw, _ := json.Marshal(request)
	return strings.Index(string(raw), `"`+key+`"`) + 1
}

func customRoleSystemRoleExists(id int64) bool {
	for _, known := range customRoleSystemRoles {
		if known == id {
			return true
		}
	}
	return false
}

func customRoleError(status int, message string) (int, any) {
	return status, map[string]any{"code": status, "message": message}
}
