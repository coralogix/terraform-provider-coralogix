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
	"strconv"
	"strings"
	"sync"
	"testing"
)

// aiceBasePrefix is the part of the path that comes before the AI routes.
const aiceBasePrefix = "/ai/"

// aiceFakeCompanyID is the company of every application in the fake. The real API puts it in each
// application.
const aiceFakeCompanyID = "700001"

// aiceUUIDPattern is the form that the real API checks for an id. A different form is an error
// that depends on the route (see the notes of the fake).
var aiceUUIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// aiceMaskPaths are the field paths that an update mask may name, as the service lists them (proto
// names). The REST gateway takes the names of the JSON body, in lower camel case, and turns them into
// these: "policyType" is "policy_type". A name with an underscore is refused by the gateway.
var aiceMaskPaths = map[string]string{
	"name":                         "name",
	"description":                  "description",
	"instructions":                 "instructions",
	"violates":                     "violates",
	"safe":                         "safe",
	"policy_type":                  "policyType",
	"should_include_system_prompt": "shouldIncludeSystemPrompt",
	"examples":                     "examples",
}

// aiceSnakeCase turns a lower camel case path of the JSON body into the proto name.
func aiceSnakeCase(path string) string {
	var out []rune
	for _, r := range path {
		if r >= 'A' && r <= 'Z' {
			out = append(out, '_', r+('a'-'A'))
			continue
		}
		out = append(out, r)
	}
	return string(out)
}

// aiceClearDefaults are the values that a field takes when a mask names it and the body omits it.
var aiceClearDefaults = map[string]any{
	"description":               "",
	"instructions":              "",
	"violates":                  "",
	"safe":                      "",
	"policyType":                "",
	"shouldIncludeSystemPrompt": false,
}

// aiceFake models the AI custom evaluations and AI applications REST APIs as recorded on a real
// tenant (fidelity/recorded.json):
//   - Create answers {"item": evaluation}. The evaluation always has every config field: strings
//     are "", the examples are [], and the flag is false when the request left them out.
//   - Update is a PATCH of the evaluation. The answer holds the evaluation with applicationIds [],
//     whatever the links are. The update mask is a property of the body (a comma-separated string of
//     proto field names). A mask that names a field makes the evaluation take the body value, or
//     the empty value when the body has none. Without a mask, a field of the body changes the
//     evaluation, except an empty examples list, which keeps the examples. The mask as a query
//     parameter does nothing.
//   - Application links are made with their own calls. Linking twice is 409, and unlinking a link
//     that does not exist is 404.
//   - Names are unique in the company, also on a rename. The empty name is accepted by Update.
//   - The API does not check the policy type and the score of an example.
//   - An id that is not a UUID is 400 on Update (a validation message) and 404 on every other
//     route. There is no Get of one evaluation: the resource reads the list.
//   - The list of applications pages with page_size and a page_offset that counts pages.
type aiceFake struct {
	mu          sync.Mutex
	evaluations []*aiceEvaluation
	apps        []*aiceApp
	requests    []grRequest
	nextEvalID  int
	nextAppID   int
	// The statuses force an answer of the next calls of the route. updateScript answers the calls of
	// Update in order: 0 is a normal answer, and a call after the end of the script is normal.
	createStatus, listStatus, getStatus, appsStatus, deleteStatus, linkStatus, unlinkStatus int
	updateScript                                                                            []int
}

type aiceEvaluation struct {
	id                        string
	name                      string
	description               string
	instructions              string
	violates                  string
	safe                      string
	policyType                string
	shouldIncludeSystemPrompt bool
	examples                  []map[string]any
	appIDs                    []string
}

type aiceApp struct {
	id          string
	application string
	subsystem   string
}

const (
	aiceFirstEvaluationID = 1001
	aiceFirstAppID        = 2001
)

func aiceID(n int) string { return fmt.Sprintf("00000000-0000-4000-8000-%012d", n) }

// aiceFirstEvalID is the id that the fake gives the first evaluation that a scenario makes.
var aiceFirstEvalID = aiceID(aiceFirstEvaluationID)

func newAICEFidelityFake() *aiceFake {
	return &aiceFake{nextEvalID: aiceFirstEvaluationID, nextAppID: aiceFirstAppID}
}

func newAICEFake(t *testing.T) *aiceFake {
	t.Helper()
	f := newAICEFidelityFake()
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

func (h *grHarness) aice() *aiceFake {
	h.t.Helper()
	f, ok := h.api.(*aiceFake)
	if !ok {
		h.t.Fatal("expected *aiceFake")
	}
	return f
}

func (f *aiceFake) takeRequests() []grRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.requests
	f.requests = nil
	return out
}

// seedApp adds an AI application and returns its id.
func (f *aiceFake) seedApp(application, subsystem string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := aiceID(f.nextAppID)
	f.nextAppID++
	f.apps = append(f.apps, &aiceApp{id: id, application: application, subsystem: subsystem})
	return id
}

// forgetApp removes an application, as if it was deleted outside Terraform.
func (f *aiceFake) forgetApp(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, app := range f.apps {
		if app.id == id {
			f.apps = append(f.apps[:i], f.apps[i+1:]...)
			return
		}
	}
}

// seedEvaluation adds an evaluation as if it was created outside Terraform, linked to the
// applications, and returns its id. It has the policy type "quality" and the description "seeded".
func (f *aiceFake) seedEvaluation(name string, appIDs ...string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := aiceID(f.nextEvalID)
	f.nextEvalID++
	f.evaluations = append(f.evaluations, &aiceEvaluation{
		id: id, name: name, description: "seeded", instructions: "Check {prompt}", policyType: "quality",
		examples: []map[string]any{}, appIDs: append([]string{}, appIDs...),
	})
	return id
}

// evaluation returns the evaluation for a test to change outside Terraform.
func (f *aiceFake) evaluation(id string) *aiceEvaluation {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, e := range f.evaluations {
		if e.id == id {
			return e
		}
	}
	panic("aiceFake: no evaluation " + id)
}

// forgetOutside removes an evaluation, as if it was deleted outside Terraform.
func (f *aiceFake) forgetOutside(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, e := range f.evaluations {
		if e.id == id {
			f.evaluations = append(f.evaluations[:i], f.evaluations[i+1:]...)
			return
		}
	}
}

func (f *aiceFake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	raw, _ := io.ReadAll(r.Body)
	path := r.URL.Path
	if r.URL.RawQuery != "" {
		path += "?" + r.URL.RawQuery
	}
	rec := grRequest{Method: r.Method, Path: path}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &rec.Body)
	}
	status, body := f.handle(r.Method, r.URL.Path, r.URL.Query(), rec.Body)
	rec.Status = status
	f.requests = append(f.requests, rec)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func aiceError(status int, message string) (int, any) {
	return status, map[string]any{"code": status, "message": message}
}

func aiceNotFound(what string) (int, any) { return aiceError(http.StatusNotFound, "Not Found: "+what) }

// aiceInvalidID is the answer for an id that is no UUID, where the service has no validation of its
// own: the database refuses the id and the service maps that to 404.
func aiceInvalidID() (int, any) { return aiceNotFound("Resource not found: invalid ID") }

func (f *aiceFake) handle(method, path string, query url.Values, body any) (int, any) {
	i := strings.Index(path, aiceBasePrefix)
	if i < 0 {
		return aiceError(http.StatusNotFound, "Not Found")
	}
	rest := strings.Trim(path[i+len(aiceBasePrefix):], "/")
	if rest == "applications/v3" && method == http.MethodGet {
		return f.listApps(query)
	}
	parts := strings.Split(rest, "/")
	if len(parts) >= 2 && parts[0] == "custom-evaluations" && parts[1] == "v3" {
		if status, answer, ok := f.routeEvaluation(method, parts[2:], body); ok {
			return status, answer
		}
	}
	return aiceError(http.StatusMethodNotAllowed, "Method Not Allowed")
}

// routeEvaluation answers the routes under /custom-evaluations/v3. ok is false for a route that
// the API does not have.
func (f *aiceFake) routeEvaluation(method string, parts []string, body any) (int, any, bool) {
	switch len(parts) {
	case 0:
		return f.routeCollection(method, body)
	case 1:
		return f.routeItem(method, parts[0], body)
	case 3:
		return f.routeLink(method, parts)
	}
	return 0, nil, false
}

func (f *aiceFake) routeCollection(method string, body any) (int, any, bool) {
	switch method {
	case http.MethodGet:
		status, answer := f.list()
		return status, answer, true
	case http.MethodPost:
		status, answer := f.create(asObject(body))
		return status, answer, true
	}
	return 0, nil, false
}

func (f *aiceFake) routeItem(method, id string, body any) (int, any, bool) {
	switch method {
	case http.MethodGet:
		status, answer := f.get(id)
		return status, answer, true
	case http.MethodPatch:
		status, answer := f.update(id, asObject(body))
		return status, answer, true
	case http.MethodDelete:
		status, answer := f.delete(id)
		return status, answer, true
	}
	return 0, nil, false
}

// routeLink answers /{id}/applications/{application_id}.
func (f *aiceFake) routeLink(method string, parts []string) (int, any, bool) {
	if parts[1] != "applications" {
		return 0, nil, false
	}
	switch method {
	case http.MethodPost:
		status, answer := f.link(parts[0], parts[2])
		return status, answer, true
	case http.MethodDelete:
		status, answer := f.unlink(parts[0], parts[2])
		return status, answer, true
	}
	return 0, nil, false
}

func (f *aiceFake) listApps(query url.Values) (int, any) {
	if f.appsStatus != 0 {
		return aiceError(f.appsStatus, http.StatusText(f.appsStatus))
	}
	size, _ := strconv.Atoi(query.Get("page_size"))
	if size <= 0 {
		size = 200
	}
	page, _ := strconv.Atoi(query.Get("page_offset"))
	items := []any{}
	for i := page * size; i < len(f.apps) && i < (page+1)*size; i++ {
		app := f.apps[i]
		items = append(items, map[string]any{
			"id": app.id, "application": app.application, "subsystem": app.subsystem,
			"companyId": aiceFakeCompanyID, "color": "blue", "icon": "bot", "guardrailsIntegrated": false,
			"createdAt": "2026-01-01T00:00:00Z", "updatedAt": "2026-01-01T00:00:00Z",
		})
	}
	return http.StatusOK, map[string]any{"aiApplications": items}
}

func (f *aiceFake) find(id string) *aiceEvaluation {
	for _, e := range f.evaluations {
		if e.id == id {
			return e
		}
	}
	return nil
}

func (f *aiceFake) findByName(name string) *aiceEvaluation {
	for _, e := range f.evaluations {
		if e.name == name {
			return e
		}
	}
	return nil
}

func (f *aiceFake) hasApp(id string) bool {
	for _, app := range f.apps {
		if app.id == id {
			return true
		}
	}
	return false
}

func (e *aiceEvaluation) proto(appIDs []string) map[string]any {
	examples := make([]any, 0, len(e.examples))
	for _, example := range e.examples {
		examples = append(examples, example)
	}
	ids := make([]any, 0, len(appIDs))
	for _, id := range appIDs {
		ids = append(ids, id)
	}
	return map[string]any{
		"id": e.id, "name": e.name, "description": e.description,
		"config": map[string]any{
			"instructions": e.instructions, "violates": e.violates, "safe": e.safe, "examples": examples,
			"shouldIncludeSystemPrompt": e.shouldIncludeSystemPrompt, "policyType": e.policyType,
		},
		"applicationIds": ids,
	}
}

func (f *aiceFake) list() (int, any) {
	if f.listStatus != 0 {
		return aiceError(f.listStatus, http.StatusText(f.listStatus))
	}
	items := []any{}
	for _, e := range f.evaluations {
		items = append(items, e.proto(e.appIDs))
	}
	return http.StatusOK, map[string]any{"items": items}
}

// get answers the read of one evaluation, the call that the generated resource makes. The recording
// has no such call, because the API of the recording does not have it yet: the answer follows the
// contract of the new call, which returns the evaluation itself, with 404 for an id that no
// evaluation has and for an id that is no UUID.
func (f *aiceFake) get(id string) (int, any) {
	if f.getStatus != 0 {
		return aiceError(f.getStatus, http.StatusText(f.getStatus))
	}
	if !aiceUUIDPattern.MatchString(id) {
		return aiceInvalidID()
	}
	e := f.find(id)
	if e == nil {
		return aiceNotFound("Custom evaluation not found")
	}
	return http.StatusOK, e.proto(e.appIDs)
}

func aiceExamples(value any) []map[string]any {
	out := []map[string]any{}
	for _, item := range asSlice(value) {
		example := asObject(item)
		out = append(out, map[string]any{"conversation": example["conversation"], "score": example["score"]})
	}
	return out
}

func (f *aiceFake) create(request map[string]any) (int, any) {
	if f.createStatus != 0 {
		return aiceError(f.createStatus, http.StatusText(f.createStatus))
	}
	name, _ := request["name"].(string)
	if name == "" {
		return aiceError(http.StatusBadRequest, "Bad Request: Name is required")
	}
	if f.findByName(name) != nil {
		return aiceError(http.StatusConflict, fmt.Sprintf("Already Exists: A custom evaluation with name '%s' already exists", name))
	}
	appIDs := []string{}
	for _, item := range asSlice(request["applicationIds"]) {
		id, _ := item.(string)
		if !aiceUUIDPattern.MatchString(id) {
			return aiceInvalidID()
		}
		if !f.hasApp(id) {
			return aiceNotFound("AI application not found")
		}
		appIDs = append(appIDs, id)
	}
	e := &aiceEvaluation{id: aiceID(f.nextEvalID), name: name, appIDs: appIDs, examples: aiceExamples(request["examples"])}
	f.nextEvalID++
	e.description, _ = request["description"].(string)
	e.instructions, _ = request["instructions"].(string)
	e.violates, _ = request["violates"].(string)
	e.safe, _ = request["safe"].(string)
	e.policyType, _ = request["policyType"].(string)
	e.shouldIncludeSystemPrompt, _ = request["shouldIncludeSystemPrompt"].(bool)
	f.evaluations = append(f.evaluations, e)
	return http.StatusOK, map[string]any{"item": e.proto(appIDs)}
}

// aiceZodIssue is the text of a validation error of the service, which lists the issues as JSON.
func aiceZodIssue(issue string) (int, any) {
	return aiceError(http.StatusBadRequest, "Bad Request: [\n"+issue+"\n]")
}

// aiceParseMask reads the update mask of a body. A mask that the service refuses gives the status
// and the answer.
func aiceParseMask(request map[string]any) (mask []string, status int, answer any) {
	text, _ := request["updateMask"].(string)
	if text == "" {
		return nil, 0, nil
	}
	for _, path := range strings.Split(text, ",") {
		if strings.Contains(path, "_") {
			// The gateway refuses a proto name: the mask is written as the JSON body is.
			status, answer = aiceError(http.StatusBadRequest, fmt.Sprintf("Bad Request: proto:\u00a0(line 1:16): google.protobuf.FieldMask.paths contains invalid path: %q", path))
			return nil, status, answer
		}
		proto := aiceSnakeCase(path)
		field, known := aiceMaskPaths[proto]
		if !known {
			status, answer = aiceZodIssue(fmt.Sprintf("  {\n    \"code\": \"custom\",\n    \"message\": \"Unknown field path \\\"%s\\\" in update_mask.\",\n    \"path\": [\n      \"updateMask\"\n    ]\n  }", proto))
			return nil, status, answer
		}
		mask = append(mask, field)
	}
	for _, field := range mask {
		if _, has := request[field]; field == "name" && !has {
			status, answer = aiceZodIssue("  {\n    \"code\": \"custom\",\n    \"path\": [\n      \"updateMask\"\n    ],\n    \"message\": \"Field \\\"name\\\" is required and cannot be cleared via update_mask.\"\n  }")
			return nil, status, answer
		}
	}
	return mask, 0, nil
}

// aiceUpdates are the fields that an Update changes. With a mask, every named field takes the
// value of the body, or the empty value. Without a mask, a field of the body changes the evaluation,
// except an empty examples list.
func aiceUpdates(request map[string]any, mask []string) map[string]any {
	updates := map[string]any{}
	if len(mask) > 0 {
		for _, field := range mask {
			switch value, has := request[field]; {
			case has:
				updates[field] = value
			case field == "examples":
				updates[field] = []any{}
			default:
				updates[field] = aiceClearDefaults[field]
			}
		}
		return updates
	}
	for _, field := range aiceMaskPaths {
		value, has := request[field]
		if has && (field != "examples" || len(asSlice(value)) != 0) {
			updates[field] = value
		}
	}
	return updates
}

func (e *aiceEvaluation) apply(updates map[string]any) {
	for field, value := range updates {
		switch field {
		case "name":
			e.name, _ = value.(string)
		case "description":
			e.description, _ = value.(string)
		case "instructions":
			e.instructions, _ = value.(string)
		case "violates":
			e.violates, _ = value.(string)
		case "safe":
			e.safe, _ = value.(string)
		case "policyType":
			e.policyType, _ = value.(string)
		case "shouldIncludeSystemPrompt":
			e.shouldIncludeSystemPrompt, _ = value.(bool)
		case "examples":
			e.examples = aiceExamples(value)
		}
	}
}

func (f *aiceFake) update(id string, request map[string]any) (int, any) {
	if len(f.updateScript) > 0 {
		status := f.updateScript[0]
		f.updateScript = f.updateScript[1:]
		if status != 0 {
			return aiceError(status, http.StatusText(status))
		}
	}
	if !aiceUUIDPattern.MatchString(id) {
		return aiceZodIssue("  {\n    \"validation\": \"uuid\",\n    \"code\": \"invalid_string\",\n    \"message\": \"Invalid uuid\",\n    \"path\": [\n      \"id\"\n    ]\n  }")
	}
	mask, status, answer := aiceParseMask(request)
	if status != 0 {
		return status, answer
	}
	e := f.find(id)
	if e == nil {
		return aiceNotFound("Custom evaluation not found")
	}
	updates := aiceUpdates(request, mask)
	if name, ok := updates["name"].(string); ok && name != e.name && f.findByName(name) != nil {
		return aiceError(http.StatusConflict, fmt.Sprintf("Already Exists: A custom evaluation with name '%s' already exists", name))
	}
	e.apply(updates)
	return http.StatusOK, map[string]any{"item": e.proto(nil)}
}

func (f *aiceFake) delete(id string) (int, any) {
	if f.deleteStatus != 0 {
		return aiceError(f.deleteStatus, http.StatusText(f.deleteStatus))
	}
	if !aiceUUIDPattern.MatchString(id) {
		return aiceInvalidID()
	}
	for i, e := range f.evaluations {
		if e.id == id {
			f.evaluations = append(f.evaluations[:i], f.evaluations[i+1:]...)
			return http.StatusOK, map[string]any{"id": id}
		}
	}
	return aiceNotFound("Custom evaluation not found")
}

func (f *aiceFake) link(id, appID string) (int, any) {
	if f.linkStatus != 0 {
		return aiceError(f.linkStatus, http.StatusText(f.linkStatus))
	}
	if !aiceUUIDPattern.MatchString(id) || !aiceUUIDPattern.MatchString(appID) {
		return aiceInvalidID()
	}
	e := f.find(id)
	if e == nil {
		return aiceNotFound("Custom evaluation not found")
	}
	if !f.hasApp(appID) {
		return aiceNotFound("AI application not found")
	}
	for _, existing := range e.appIDs {
		if existing == appID {
			return aiceError(http.StatusConflict, "Already Exists: Custom evaluation is already linked to this application")
		}
	}
	e.appIDs = append(e.appIDs, appID)
	return http.StatusOK, map[string]any{"id": id, "applicationId": appID}
}

func (f *aiceFake) unlink(id, appID string) (int, any) {
	if f.unlinkStatus != 0 {
		return aiceError(f.unlinkStatus, http.StatusText(f.unlinkStatus))
	}
	if !aiceUUIDPattern.MatchString(id) || !aiceUUIDPattern.MatchString(appID) {
		return aiceInvalidID()
	}
	e := f.find(id)
	if e == nil {
		return aiceNotFound("Custom evaluation not found")
	}
	for i, existing := range e.appIDs {
		if existing == appID {
			e.appIDs = append(e.appIDs[:i], e.appIDs[i+1:]...)
			return http.StatusOK, map[string]any{"id": id, "applicationId": appID}
		}
	}
	return aiceNotFound("Custom evaluation is not linked to this application")
}

// sortedNames lists the names of the evaluations that the fake holds, for the dry run of the recorder.
func (f *aiceFake) sortedNames() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	names := make([]string, 0, len(f.evaluations))
	for _, e := range f.evaluations {
		names = append(names, e.name)
	}
	sort.Strings(names)
	return names
}
