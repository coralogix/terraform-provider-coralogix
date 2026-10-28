// Copyright 2024 Coralogix Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	https://www.apache.org/licenses/LICENSE-2.0
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
	"strings"
	"sync"
	"testing"
)

const (
	actionBasePath  = "/actions/actions/v2"
	actionGoldenDir = "testdata/action_baseline"

	actionValidation = "Bad Request: validation failed for method /com.coralogixapis.actions.v2.ActionsService/"
)

var (
	actionUUID    = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
	actionURLForm = regexp.MustCompile(`^http[s]?://.*`)
)

// actionFake follows the actions service as recorded in fidelity/recorded.json, not the Go
// client:
//   - Create takes the action at the top of the body, Replace takes it in {action: ...}, and both
//     answer {action: {...}}. Delete answers {}.
//   - A private action needs a user, which an API key does not have, so a create with isPrivate
//     true or omitted is 400.
//   - Replace needs the id and isHidden, and ignores the isHidden value. It keeps an omitted
//     scalar and replaces a list. It cannot change isPrivate (412).
//   - A dpxlFilter without the version prefix is stored with `<v1> `. An omitted description and
//     dpxlFilter are left out of the answer, and an empty one is returned as "".
//   - The name is unique (409), at most 50 characters, and the url must start with http(s)://.
//   - A missing or malformed id is 404 on Get, Replace, and Delete.
//
// The fake does not check the order of validations, and it keeps a list that Replace omits as it
// was (not recorded).
type actionFake struct {
	mu       sync.Mutex
	actions  map[string]map[string]any
	seq      int
	requests []grRequest
}

func newActionFakeState() *actionFake { return &actionFake{actions: map[string]map[string]any{}} }

func newActionFake(t *testing.T) *actionFake {
	t.Helper()
	f := newActionFakeState()
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

// seed stores an action as if it was made outside Terraform. It returns the id.
func (f *actionFake) seed(action map[string]any) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	status, body := f.create(action)
	if status != http.StatusOK {
		panic(fmt.Sprintf("seed failed: %d %v", status, body))
	}
	return actionIDOf(body)
}

// forget deletes an action without a request, as if it was deleted outside Terraform.
func (f *actionFake) forget(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.actions, id)
}

func (f *actionFake) takeRequests() []grRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.requests
	f.requests = nil
	return out
}

func (f *actionFake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(r.Body)
	rec := grRequest{Method: r.Method, Path: r.URL.Path}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &rec.Body)
	}
	f.mu.Lock()
	status, body := f.handle(r.Method, r.URL.Path, rec.Body)
	rec.Body = dropEmptyCollections(rec.Body)
	rec.Status = status
	f.requests = append(f.requests, rec)
	f.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func actionFail(status int, reason string) (int, any) {
	prefix := map[int]string{
		http.StatusBadRequest:          "Bad Request",
		http.StatusNotFound:            "Not Found",
		http.StatusConflict:            "Already Exists",
		http.StatusPreconditionFailed:  "Failed Precondition",
		http.StatusNotImplemented:      "Not Implemented",
		http.StatusInternalServerError: "Internal",
	}[status]
	return status, map[string]any{"code": status, "message": prefix + ": " + reason}
}

func actionInvalid(method, reason string) (int, any) {
	return http.StatusBadRequest, map[string]any{"code": http.StatusBadRequest, "message": actionValidation + method + ": ValidationError: " + reason}
}

func actionMissing(id string) (int, any) {
	return actionFail(http.StatusNotFound, "Action not found, id "+id+", companyId 1")
}

func (f *actionFake) handle(method, path string, body any) (int, any) {
	_, rest, ok := strings.Cut(path, actionBasePath)
	if !ok {
		return actionFail(http.StatusNotFound, "unknown path "+path)
	}
	id := strings.TrimPrefix(rest, "/")
	in, _ := body.(map[string]any)
	switch {
	case method == http.MethodPost && rest == "":
		return f.create(in)
	case method == http.MethodPut && rest == "":
		return f.replace(in)
	case method == http.MethodGet && rest == "":
		list := []any{}
		for _, a := range f.actions {
			list = append(list, a)
		}
		return http.StatusOK, map[string]any{"actions": list}
	case method == http.MethodGet && id != "":
		a, ok := f.actions[id]
		if !ok {
			return actionMissing(id)
		}
		return http.StatusOK, map[string]any{"action": a}
	case method == http.MethodDelete && id != "":
		if _, ok := f.actions[id]; !ok {
			return actionMissing(id)
		}
		delete(f.actions, id)
		return http.StatusOK, map[string]any{}
	}
	return actionFail(http.StatusNotFound, "unknown route "+method+" "+path)
}

// actionCheck is the schema validation that runs before the business rules. prefix is "" for a
// create and "action." for a replace.
func actionCheck(method, prefix string, in map[string]any) (int, any, bool) {
	field := func(name string) string { return `"` + prefix + name + `"` }
	name, hasName := in["name"].(string)
	switch {
	case !hasName:
		s, b := actionInvalid(method, field("name")+" is required")
		return s, b, false
	case name == "":
		s, b := actionInvalid(method, field("name")+" is not allowed to be empty")
		return s, b, false
	case len([]rune(name)) > 50:
		s, b := actionInvalid(method, field("name")+" length must be less than or equal to 50 characters long")
		return s, b, false
	}
	u, hasURL := in["url"].(string)
	switch {
	case !hasURL:
		s, b := actionInvalid(method, field("url")+" is required")
		return s, b, false
	case u == "":
		s, b := actionInvalid(method, field("url")+" is not allowed to be empty")
		return s, b, false
	case !actionURLForm.MatchString(u):
		s, b := actionInvalid(method, fmt.Sprintf(`%s with value "%s" fails to match the required pattern: /^http[s]?:\/\/.*/`, field("url"), u))
		return s, b, false
	}
	seen := map[string]bool{}
	fields, _ := in["urlFields"].([]any)
	for i, raw := range fields {
		uf, _ := raw.(map[string]any)
		n, _ := uf["name"].(string)
		if n == "" {
			s, b := actionInvalid(method, fmt.Sprintf(`"%surlFields[%d].name" is not allowed to be empty`, prefix, i))
			return s, b, false
		}
		if seen[n] {
			s, b := actionInvalid(method, fmt.Sprintf(`"%surlFields[%d]" contains a duplicate value`, prefix, i))
			return s, b, false
		}
		seen[n] = true
	}
	return 0, nil, true
}

// actionCheckSourceType refuses an omitted or unspecified type ("Invalid source type") and a name
// that is not in the enum (a proto error, whose line and column the recorder masks).
func actionCheckSourceType(st string) (int, any, bool) {
	switch st {
	case "SOURCE_TYPE_LOG", "SOURCE_TYPE_DATA_MAP":
		return 0, nil, true
	case "", "SOURCE_TYPE_UNSPECIFIED":
		s, b := actionFail(http.StatusBadRequest, "Invalid source type")
		return s, b, false
	}
	s, b := actionFail(http.StatusBadRequest, fmt.Sprintf("proto:\u00a0(line L:C): invalid value for enum field sourceType: %q", st))
	return s, b, false
}

func actionFilter(v any) any {
	s, ok := v.(string)
	if !ok || s == "" || strings.HasPrefix(s, "<v1>") {
		return v
	}
	return "<v1> " + s
}

func (f *actionFake) nameTaken(name, except string) bool {
	for id, a := range f.actions {
		if id != except && a["name"] == name {
			return true
		}
	}
	return false
}

func (f *actionFake) create(in map[string]any) (int, any) {
	if in == nil {
		in = map[string]any{}
	}
	if status, body, ok := actionCheck("CreateAction", "", in); !ok {
		return status, body
	}
	if private, _ := in["isPrivate"].(bool); private || in["isPrivate"] == nil {
		return actionFail(http.StatusBadRequest, "Private actions need to be linked to a user")
	}
	st, _ := in["sourceType"].(string)
	if status, body, ok := actionCheckSourceType(st); !ok {
		return status, body
	}
	name, _ := in["name"].(string)
	if f.nameTaken(name, "") {
		return actionFail(http.StatusConflict, "Action with name "+name+" already exists")
	}
	f.seq++
	id := fmt.Sprintf("00000000-0000-4000-9000-%012d", f.seq)
	a := map[string]any{
		"id": id, "name": name, "url": in["url"], "sourceType": st,
		"isPrivate": false, "isHidden": false,
		"applicationNames": listOrEmpty(in["applicationNames"]),
		"subsystemNames":   listOrEmpty(in["subsystemNames"]),
		"urlFields":        listOrEmpty(in["urlFields"]),
	}
	if v, ok := in["description"]; ok {
		a["description"] = v
	}
	if v, ok := in["dpxlFilter"]; ok {
		a["dpxlFilter"] = actionFilter(v)
	}
	f.actions[id] = a
	return http.StatusOK, map[string]any{"action": a}
}

func listOrEmpty(v any) any {
	if l, ok := v.([]any); ok {
		return l
	}
	return []any{}
}

func (f *actionFake) replace(req map[string]any) (int, any) {
	in, _ := req["action"].(map[string]any)
	if in == nil {
		in = map[string]any{}
	}
	if _, ok := in["id"].(string); !ok {
		return actionInvalid("ReplaceAction", `"action.id" is required`)
	}
	if _, ok := in["isHidden"].(bool); !ok {
		return actionInvalid("ReplaceAction", `"action.isHidden" is required`)
	}
	if status, body, ok := actionCheck("ReplaceAction", "action.", in); !ok {
		return status, body
	}
	id := in["id"].(string)
	a, ok := f.actions[id]
	if !ok {
		return actionMissing(id)
	}
	if private, _ := in["isPrivate"].(bool); private != a["isPrivate"] {
		return actionFail(http.StatusPreconditionFailed, "Can not change visibility of shared actions, delete it instead")
	}
	st, _ := in["sourceType"].(string)
	if status, body, ok := actionCheckSourceType(st); !ok {
		return status, body
	}
	name, _ := in["name"].(string)
	if f.nameTaken(name, id) {
		return actionFail(http.StatusConflict, "Action with name "+name+" already exists")
	}
	a["name"], a["url"], a["sourceType"] = name, in["url"], st
	if v, ok := in["description"]; ok {
		a["description"] = v
	}
	if v, ok := in["dpxlFilter"]; ok {
		a["dpxlFilter"] = actionFilter(v)
	}
	for _, k := range []string{"applicationNames", "subsystemNames", "urlFields"} {
		if v, ok := in[k]; ok {
			a[k] = listOrEmpty(v)
		}
	}
	return http.StatusOK, map[string]any{"action": a}
}

// actionIDOf returns the id of the action in a response, or "".
func actionIDOf(resp any) string {
	m, _ := resp.(map[string]any)
	a, _ := m["action"].(map[string]any)
	id, _ := a["id"].(string)
	return id
}
