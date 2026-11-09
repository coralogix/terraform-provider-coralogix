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
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"
)

const (
	apiKeyBasePath = "/aaa/api-keys/v3"
	// apiKeyFakeTeamID is the team that owns the key of the fake. Ids are synthetic.
	apiKeyFakeTeamID = 4000001
)

var apiKeyIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-?[0-9a-f]{4}-?[0-9a-f]{4}-?[0-9a-f]{4}-?[0-9a-f]{12}$`)

// apiKeyPresets are the presets of the fake, with the permissions that the API shows for each.
var apiKeyPresets = map[string][]string{
	"APM": {
		"serverless:Read", "service-catalog:ReadDimensionsConfig", "service-catalog:ReadSLIConfig",
		"service-catalog:UpdateDimensionsConfig", "service-catalog:UpdateSLIConfig", "service-map:Read",
		"service-catalog:Read", "service-catalog:ReadApdexConfig", "service-catalog:UpdateApdexConfig",
		"service-catalog:Update",
	},
}

// apiKeyPermissions are the permissions that the fake accepts.
var apiKeyPermissions = map[string]bool{
	"livetail:Read":   true,
	"serverless:Read": true,
}

// apiKeyTeamPolicyPermissions are the permissions that the API adds to the policy of a key that a
// team owns, as "deny", unless the policy sets them.
var apiKeyTeamPolicyPermissions = []string{
	"team-custom-api-keys:ReadConfig",
	"team-custom-api-keys:Manage",
	"team-custom-api-keys:ReadAccessPolicy",
	"team-custom-api-keys:UpdateAccessPolicy",
}

// apiKeyFake models the API keys REST API as recorded on a real tenant (fidelity/recorded.json):
//   - Create returns {"keyId", "name", "value"}. Get wraps the key in {"keyInfo": ...} and shows the
//     value of every key that is not hashed. Update and Delete return {}.
//   - Names are not unique and have no length limit. An empty name is rejected.
//   - A key needs a preset or a permission. An Update that sends a preset or permission wrapper
//     replaces that list, and the wrapper without items empties it, also on both lists.
//   - The API accepts only team-owned keys of the team of the caller.
//   - A policy is validated, and the API adds the team permissions that it does not set, as "deny".
//   - A key that never existed, and a deleted key, answer 404 on Get, Update, and Delete.
type apiKeyFake struct {
	mu       sync.Mutex
	keys     map[string]*apiKeyRecord
	order    []string
	nextID   int
	requests []grRequest
	// createStatus, getStatus, updateStatus, and deleteStatus force an answer of the next calls.
	createStatus, getStatus, updateStatus, deleteStatus int
}

type apiKeyRecord struct {
	id           string
	name         string
	active       bool
	hashed       bool
	presets      []string
	permissions  []string
	accessPolicy string
	value        string
}

func newAPIKeyFidelityFake() *apiKeyFake {
	return &apiKeyFake{keys: map[string]*apiKeyRecord{}, nextID: 1}
}

func newAPIKeyFake(t *testing.T) *apiKeyFake {
	t.Helper()
	f := newAPIKeyFidelityFake()
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

func (h *grHarness) apiKeys() *apiKeyFake {
	h.t.Helper()
	f, ok := h.api.(*apiKeyFake)
	if !ok {
		h.t.Fatal("expected *apiKeyFake")
	}
	return f
}

func (f *apiKeyFake) takeRequests() []grRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.requests
	f.requests = nil
	return out
}

// apiKeySeed is a key that was made outside Terraform.
type apiKeySeed struct {
	name         string
	hashed       bool
	presets      []string
	permissions  []string
	accessPolicy string
}

// seed adds a key as if it was created outside Terraform and returns its id.
func (f *apiKeyFake) seed(s apiKeySeed) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	rec := f.newRecord(s.name, s.presets, s.permissions)
	rec.hashed = s.hashed
	rec.accessPolicy = s.accessPolicy
	return rec.id
}

// edit changes a key as the UI would.
func (f *apiKeyFake) edit(id string, change func(*apiKeyRecord)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	change(f.keys[id])
}

// forgetOutside deletes a key as the UI would.
func (f *apiKeyFake) forgetOutside(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.keys, id)
}

// apiKeyFakeID is the id that the fake gives the n-th key.
func apiKeyFakeID(n int) string {
	return fmt.Sprintf("00000000-0000-4000-8000-%012d", n)
}

func (f *apiKeyFake) newRecord(name string, presets, permissions []string) *apiKeyRecord {
	id := apiKeyFakeID(f.nextID)
	f.nextID++
	rec := &apiKeyRecord{
		id: id, name: name, active: true, presets: presets, permissions: permissions,
		value: fmt.Sprintf("fake-secret-%d", f.nextID-1),
	}
	f.keys[id] = rec
	f.order = append(f.order, id)
	return rec
}

func (f *apiKeyFake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

func (f *apiKeyFake) handle(method, path string, body any) (int, any) {
	i := strings.Index(path, apiKeyBasePath)
	if i < 0 {
		return apiKeyError(http.StatusNotFound, "Not Found")
	}
	rest := strings.TrimPrefix(path[i+len(apiKeyBasePath):], "/")
	switch {
	case rest == "" && method == http.MethodPost:
		return f.create(asObject(body))
	case rest == "list/all" && method == http.MethodGet:
		return f.list()
	case rest == "":
		return apiKeyError(http.StatusNotFound, "Not Found")
	}
	if !apiKeyIDPattern.MatchString(rest) {
		return apiKeyError(http.StatusBadRequest, fmt.Sprintf("Bad Request: key_id invalid length: expected length 32 for simple format, found %d", len(rest)))
	}
	switch method {
	case http.MethodGet:
		return f.get(rest)
	case http.MethodPut:
		return f.update(rest, asObject(body))
	case http.MethodDelete:
		return f.delete(rest)
	default:
		return apiKeyError(http.StatusNotFound, "Not Found")
	}
}

func (f *apiKeyFake) create(request map[string]any) (int, any) {
	if f.createStatus != 0 {
		return apiKeyError(f.createStatus, http.StatusText(f.createStatus))
	}
	name, _ := request["name"].(string)
	if name == "" {
		return apiKeyError(http.StatusBadRequest, "Bad Request: name must be not empty")
	}
	owner := asObject(request["owner"])
	if len(owner) == 0 {
		return apiKeyError(http.StatusBadRequest, "Bad Request: Missing Owner.")
	}
	permissions, hasPermissions := request["keyPermissions"].(map[string]any)
	if !hasPermissions {
		return apiKeyError(http.StatusBadRequest, "Bad Request: Missing Key Permissions.")
	}
	presets := apiKeyStrings(permissions["presets"])
	direct := apiKeyStrings(permissions["permissions"])
	if len(presets) == 0 && len(direct) == 0 {
		return apiKeyError(http.StatusBadRequest, "Bad Request: preset_ids or permission_ids must be provided")
	}
	if status, answer := apiKeyCheckOwner(owner); status != 0 {
		return status, answer
	}
	if status, answer := apiKeyCheckLists(presets, direct); status != 0 {
		return status, answer
	}
	policy, status, answer := apiKeyPolicy(request["accessPolicy"])
	if status != 0 {
		return status, answer
	}
	rec := f.newRecord(name, presets, direct)
	rec.hashed, _ = request["hashed"].(bool)
	rec.accessPolicy = policy
	return http.StatusOK, map[string]any{"keyId": rec.id, "name": rec.name, "value": rec.value}
}

// apiKeyCheckOwner accepts the team of the caller and rejects every other owner, as the API does
// for a key whose owner is a team.
func apiKeyCheckOwner(owner map[string]any) (int, any) {
	if user, ok := owner["userId"].(string); ok {
		return apiKeyError(http.StatusForbidden, fmt.Sprintf("Permission Denied: Permission denied \"The user specified as the API key owner, '%s', either does not exist or is not a member of the current team.\"", user))
	}
	if _, ok := owner["organisationId"]; ok {
		return apiKeyError(http.StatusBadRequest, "Bad Request: Unsupported combination of key owner and permissions")
	}
	if team, _ := owner["teamId"].(float64); int64(team) != apiKeyFakeTeamID {
		return apiKeyError(http.StatusForbidden, "Permission Denied: Permission denied \"Caller is not authorize to perform this action\"")
	}
	return 0, nil
}

// apiKeyCheckLists rejects an unknown preset or permission, and a repeated one (500).
func apiKeyCheckLists(presets, permissions []string) (int, any) {
	for _, p := range presets {
		if _, ok := apiKeyPresetName(p); !ok {
			return apiKeyError(http.StatusBadRequest, fmt.Sprintf("Bad Request: Preset %s does not exist or is not a valid preset for the key being created.", p))
		}
	}
	for _, p := range permissions {
		if !apiKeyPermissions[p] {
			return apiKeyError(http.StatusBadRequest, fmt.Sprintf("Bad Request: BadRequest Permission %s not found", p))
		}
	}
	if apiKeyRepeats(presets) || apiKeyRepeats(permissions) {
		return apiKeyError(http.StatusInternalServerError, "Internal Server Error: Internal Error - service error")
	}
	return 0, nil
}

// apiKeyPresetName returns the name that the API gives a preset, which it matches in any case.
func apiKeyPresetName(name string) (string, bool) {
	for known := range apiKeyPresets {
		if strings.EqualFold(known, name) {
			return known, true
		}
	}
	return "", false
}

func apiKeyRepeats(items []string) bool {
	seen := map[string]bool{}
	for _, item := range items {
		if seen[item] {
			return true
		}
		seen[item] = true
	}
	return false
}

// apiKeyPolicy validates an access policy and returns the text that the API stores: compact JSON
// that also has the team permissions that the policy leaves out, set to "deny". An empty policy
// is no policy.
func apiKeyPolicy(raw any) (string, int, any) {
	text, set := raw.(string)
	if !set || text == "" {
		return "", 0, nil
	}
	var doc any
	if err := json.Unmarshal([]byte(text), &doc); err != nil {
		status, answer := apiKeyError(http.StatusBadRequest, "Bad Request: invalid argument: provided access_policy_raw_json is not a valid json: expected ident at line 1 column 2")
		return "", status, answer
	}
	object, ok := doc.(map[string]any)
	if !ok {
		status, answer := apiKeyError(http.StatusBadRequest, "Bad Request: invalid argument: json schema validation error: "+text+" is not of type \"object\"")
		return "", status, answer
	}
	for key := range object {
		if key != "version" && key != "default" && key != "rules" {
			status, answer := apiKeyError(http.StatusBadRequest, fmt.Sprintf("Bad Request: invalid argument: json schema validation error: additional properties are not allowed ('%s' was unexpected)", key))
			return "", status, answer
		}
	}
	permissions := asObject(asObject(object["default"])["permissions"])
	var foreign []string
	for key := range permissions {
		if !strings.HasPrefix(key, "team-custom-api-keys:") {
			foreign = append(foreign, key)
		}
	}
	sort.Strings(foreign)
	if len(foreign) > 0 {
		status, answer := apiKeyError(http.StatusBadRequest, "Bad Request: invalid argument: access policy contains permissions [[\n"+apiKeyPolicyPermissionList(foreign)+"]] which are not valid for resource type: resource_team_custom_api_keys")
		return "", status, answer
	}
	merged := map[string]any{}
	for _, key := range apiKeyTeamPolicyPermissions {
		merged[key] = "deny"
	}
	for key, value := range permissions {
		merged[key] = value
	}
	object["default"] = map[string]any{"permissions": merged}
	if _, ok := object["rules"]; !ok {
		object["rules"] = []any{}
	}
	stored, _ := json.Marshal(object)
	return string(stored), 0, nil
}

// apiKeyPolicyPermissionList names the rejected permissions as the API does: one quoted
// identifier per line.
func apiKeyPolicyPermissionList(keys []string) string {
	var b strings.Builder
	for _, key := range keys {
		b.WriteString(fmt.Sprintf("    \"permission_%s\",\n", apiKeyPermissionIdentifier(key)))
	}
	return b.String()
}

// apiKeyPermissionIdentifier is the name that the API gives a permission in an error: the service
// and the action in snake case, such as permission_data_ingest_api_keys_read_access_policy.
func apiKeyPermissionIdentifier(permission string) string {
	service, action, _ := strings.Cut(permission, ":")
	var snake strings.Builder
	for i, r := range action {
		if i > 0 && r >= 'A' && r <= 'Z' {
			snake.WriteByte('_')
		}
		snake.WriteRune(r)
	}
	return strings.ToLower(strings.ReplaceAll(service, "-", "_") + "_" + snake.String())
}

func (f *apiKeyFake) get(id string) (int, any) {
	if f.getStatus != 0 {
		return apiKeyError(f.getStatus, http.StatusText(f.getStatus))
	}
	rec, ok := f.keys[id]
	if !ok {
		return apiKeyError(http.StatusNotFound, fmt.Sprintf("Not Found: Key with id KeyId(%s) does not exist", id))
	}
	return http.StatusOK, map[string]any{"keyInfo": f.info(rec)}
}

// info is the key as Get and the list show it.
func (f *apiKeyFake) info(rec *apiKeyRecord) map[string]any {
	presets := []any{}
	for _, name := range rec.presets {
		canonical, _ := apiKeyPresetName(name)
		permissions := []any{}
		for _, p := range apiKeyPresets[canonical] {
			permissions = append(permissions, p)
		}
		presets = append(presets, map[string]any{"name": canonical, "permissions": permissions})
	}
	permissions := []any{}
	for _, p := range rec.permissions {
		permissions = append(permissions, p)
	}
	info := map[string]any{
		"id":     rec.id,
		"name":   rec.name,
		"owner":  map[string]any{"teamId": apiKeyFakeTeamID},
		"active": rec.active,
		"hashed": rec.hashed,
		"keyPermissions": map[string]any{
			"presets":     presets,
			"permissions": permissions,
		},
	}
	if !rec.hashed {
		info["value"] = rec.value
	}
	if rec.accessPolicy != "" {
		info["accessPolicy"] = rec.accessPolicy
	}
	return info
}

func (f *apiKeyFake) list() (int, any) {
	keys := []any{}
	for _, id := range f.order {
		if rec, ok := f.keys[id]; ok {
			keys = append(keys, f.info(rec))
		}
	}
	return http.StatusOK, map[string]any{"keys": keys}
}

func (f *apiKeyFake) update(id string, request map[string]any) (int, any) {
	if f.updateStatus != 0 {
		return apiKeyError(f.updateStatus, http.StatusText(f.updateStatus))
	}
	rec, ok := f.keys[id]
	if !ok {
		return apiKeyError(http.StatusNotFound, fmt.Sprintf("Not Found: Key %s not found", id))
	}
	if name, set := request["newName"].(string); set && name == "" {
		return apiKeyError(http.StatusBadRequest, "Bad Request: new_name must be not empty")
	}
	presets, setPresets := request["presets"].(map[string]any)
	permissions, setPermissions := request["permissions"].(map[string]any)
	newPresets, newPermissions := rec.presets, rec.permissions
	if setPresets {
		newPresets = apiKeyStrings(presets["presets"])
	}
	if setPermissions {
		newPermissions = apiKeyStrings(permissions["permissions"])
	}
	if status, answer := apiKeyCheckLists(newPresets, newPermissions); status != 0 && (setPresets || setPermissions) {
		return status, answer
	}
	policy, status, answer := apiKeyPolicy(request["accessPolicy"])
	if status != 0 {
		return status, answer
	}
	if name, set := request["newName"].(string); set {
		rec.name = name
	}
	if active, set := request["isActive"].(bool); set {
		rec.active = active
	}
	rec.presets, rec.permissions = newPresets, newPermissions
	if _, set := request["accessPolicy"].(string); set {
		rec.accessPolicy = policy
	}
	return http.StatusOK, map[string]any{}
}

func (f *apiKeyFake) delete(id string) (int, any) {
	if f.deleteStatus != 0 {
		return apiKeyError(f.deleteStatus, http.StatusText(f.deleteStatus))
	}
	if _, ok := f.keys[id]; !ok {
		return apiKeyError(http.StatusNotFound, fmt.Sprintf("Not Found: Key with id KeyId(%s) does not exist", id))
	}
	delete(f.keys, id)
	return http.StatusOK, map[string]any{}
}

func apiKeyStrings(value any) []string {
	var out []string
	for _, item := range asSlice(value) {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func apiKeyError(status int, message string) (int, any) {
	return status, map[string]any{"code": status, "message": message}
}
