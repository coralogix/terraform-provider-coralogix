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
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"testing"

	dashboardservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_service"
)

const dashboardBasePath = "/dashboards"

// dashboardIDLength is the length of every dashboard id. The API checks it on every route (recorded
// on EU2: "dashboard_id is not 21 characters length").
const dashboardIDLength = 21

// dashboardDefaultPolicy is the access policy that the API gives a dashboard created without one.
const dashboardDefaultPolicy = `{"default":{"permissions":{"team-dashboards:Read":"grant","team-dashboards:ReadAccessPolicy":"grant","team-dashboards:Update":"grant","team-dashboards:UpdateAccessPolicy":"grant"}},"rules":[],"version":"2025-01-01"}`

var dashboardPolicyKeys = []string{"team-dashboards:Read", "team-dashboards:ReadAccessPolicy", "team-dashboards:Update", "team-dashboards:UpdateAccessPolicy"}

// normalizeDashboardPolicy fills the permissions that a policy leaves out with "deny", as the API does
// (recorded: a policy that grants only Read reads back with the other three denied). The text is
// compact JSON with sorted keys. The API's own key order varies, and the replay compares JSON.
func normalizeDashboardPolicy(policy string) string {
	if policy == "" {
		return ""
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(policy), &parsed); err != nil {
		return policy
	}
	def, _ := parsed["default"].(map[string]any)
	if def == nil {
		def = map[string]any{}
		parsed["default"] = def
	}
	permissions, _ := def["permissions"].(map[string]any)
	if permissions == nil {
		permissions = map[string]any{}
		def["permissions"] = permissions
	}
	for _, key := range dashboardPolicyKeys {
		if _, ok := permissions[key]; !ok {
			permissions[key] = "deny"
		}
	}
	out, err := json.Marshal(parsed)
	if err != nil {
		panic(err)
	}
	return string(out)
}

func dashboardNotFound(id string) (int, any) {
	return dashboardFailure(http.StatusNotFound, fmt.Sprintf("dashboard with id %s does not exist", id))
}

func dashboardBadID() (int, any) {
	return dashboardFailure(http.StatusBadRequest, "dashboard_id is not 21 characters length")
}

// dashboardFake follows the dashboards service as probed on EU2, not the Go client.
//
//	Create   POST   /dashboards/dashboards/v1        answers {dashboardId} only
//	Get      GET    /dashboards/dashboards/v1/{id}   answers {dashboard, accessPolicy}
//	Replace  PUT    /dashboards/dashboards/v1        answers {}; dashboard.id must match dashboardId
//	Delete   DELETE /dashboards/dashboards/v1/{id}   answers {}
//	Check    POST   /dashboards/check/v1             answers {issues}
//
// Read-back rules from the probe: an omitted list reads back as [], an omitted enum reads back as
// its first value (the *_UNSPECIFIED name), an empty object keeps its empty fields filled, order is
// kept, and no value is changed. Create does not check id formats, duplicate ids, or query syntax.
type dashboardFake struct {
	mu         sync.Mutex
	dashboards map[string]map[string]any
	policies   map[string]string
	seq        int
	requests   []grRequest
	faults     map[string][]dashboardFault
	sticky     map[string]dashboardFault
	replies    map[string][]any
	issues     []any
	transport  error
	// folders answers the folder routes, which the acceptance tests on the fake need.
	folders http.Handler
}

type dashboardFault struct {
	status  int
	message string
}

func newDashboardFake(t *testing.T) *dashboardFake {
	t.Helper()
	f, stop := startDashboardFake()
	t.Cleanup(stop)
	return f
}

// startDashboardFake serves the fake and sends every HTTP call of the process to it, until stop.
func startDashboardFake() (*dashboardFake, func()) {
	f, target, closeServer := serveDashboardFake()
	original := http.DefaultTransport
	http.DefaultTransport = dashboardTransport{fake: f, next: redirectTransport{target: target, next: original}}
	return f, func() {
		http.DefaultTransport = original
		closeServer()
	}
}

func serveDashboardFake() (*dashboardFake, *url.URL, func()) {
	f := &dashboardFake{
		dashboards: map[string]map[string]any{},
		policies:   map[string]string{},
		faults:     map[string][]dashboardFault{},
		sticky:     map[string]dashboardFault{},
		replies:    map[string][]any{},
		folders:    &dashboardsFolderFake{folders: map[string]map[string]any{}, seeded: map[string]bool{}},
	}
	srv := httptest.NewServer(f)
	target, err := url.Parse(srv.URL)
	if err != nil {
		panic(err)
	}
	return f, target, srv.Close
}

// routedDashboardFakes lets scenarios run in parallel. The provider sends every call through
// http.DefaultTransport, so one router is installed for the process. It picks the fake by the API key
// of the call, and each scenario has its own key.
type routedDashboardFakes struct {
	original http.RoundTripper
	fakes    sync.Map // API key -> http.RoundTripper
}

func (r *routedDashboardFakes) RoundTrip(req *http.Request) (*http.Response, error) {
	if next, ok := r.fakes.Load(strings.TrimPrefix(req.Header.Get("Authorization"), "Bearer ")); ok {
		return next.(http.RoundTripper).RoundTrip(req)
	}
	return r.original.RoundTrip(req)
}

// installDashboardRouter installs the router until the test and all its subtests end.
func installDashboardRouter(t *testing.T) *routedDashboardFakes {
	t.Helper()
	router := &routedDashboardFakes{original: http.DefaultTransport}
	http.DefaultTransport = router
	// A plan of this schema allocates a lot, and the garbage collector took about 40% of the CPU time.
	previousGC := debug.SetGCPercent(800)
	t.Cleanup(func() {
		debug.SetGCPercent(previousGC)
		http.DefaultTransport = router.original
	})
	return router
}

// newRoutedDashboardFake starts a fake that serves only the calls that carry the key.
func newRoutedDashboardFake(t *testing.T, router *routedDashboardFakes, key string) *dashboardFake {
	t.Helper()
	f, target, closeServer := serveDashboardFake()
	router.fakes.Store(key, dashboardTransport{fake: f, next: redirectTransport{target: target, next: router.original}})
	t.Cleanup(func() {
		router.fakes.Delete(key)
		closeServer()
	})
	return f
}

type dashboardTransport struct {
	fake *dashboardFake
	next http.RoundTripper
}

func (d dashboardTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	d.fake.mu.Lock()
	err := d.fake.transport
	d.fake.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return d.next.RoundTrip(req)
}

func (f *dashboardFake) takeRequests() []grRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.requests
	f.requests = nil
	return out
}

// seed stores a dashboard as if it was made outside Terraform. It returns the id.
func (f *dashboardFake) seed(dashboard map[string]any, accessPolicy string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	body := map[string]any{"dashboard": dashboard}
	if accessPolicy != "" {
		body["accessPolicy"] = accessPolicy
	}
	status, out := f.create(body)
	if status != http.StatusOK {
		panic(fmt.Sprintf("seed failed: %d %v", status, out))
	}
	return out.(map[string]any)["dashboardId"].(string)
}

// setPolicy changes the access policy the API holds, as if it was changed outside Terraform.
func (f *dashboardFake) setPolicy(id, policy string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.policies[id] = policy
}

// forget deletes a dashboard without a request, as if it was deleted outside Terraform.
func (f *dashboardFake) forget(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.dashboards, id)
	delete(f.policies, id)
}

// stored returns a copy of the dashboard as the API holds it.
func (f *dashboardFake) stored(id string) map[string]any {
	f.mu.Lock()
	defer f.mu.Unlock()
	return dashboardRoundTrip(f.dashboards[id]).(map[string]any)
}

func (f *dashboardFake) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.dashboards)
}

// failNext makes the next call of the operation (Create, Get, Replace, Delete, Check) answer with an error.
func (f *dashboardFake) failNext(op string, status int, message string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.faults[op] = append(f.faults[op], dashboardFault{status, message})
}

// failAlways makes every call of the operation answer with an error until clearFaults.
func (f *dashboardFake) failAlways(op string, status int, message string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sticky[op] = dashboardFault{status, message}
}

func (f *dashboardFake) clearFaults() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.faults = map[string][]dashboardFault{}
	f.sticky = map[string]dashboardFault{}
}

// replyNext makes the next call of the operation answer 200 with this body, and change nothing.
func (f *dashboardFake) replyNext(op string, body any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.replies[op] = append(f.replies[op], body)
}

// checkIssues sets the issues that Check reports.
func (f *dashboardFake) checkIssues(issues ...any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.issues = issues
}

func (f *dashboardFake) failTransport(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.transport = err
}

func (f *dashboardFake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if f.folders != nil && strings.Contains(r.URL.Path, dashboardsFolderBasePath) {
		f.folders.ServeHTTP(w, r)
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		panic(err)
	}
	var body map[string]any
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &body); err != nil {
			panic(err)
		}
	}
	// The handler changes the body (a folderPath becomes a folderId), so the request is recorded first.
	var recorded json.RawMessage
	if body != nil {
		sent := maps.Clone(body)
		delete(sent, "requestId")
		clean, err := json.Marshal(sent)
		if err != nil {
			panic(err)
		}
		recorded = clean
	}
	status, out := f.handle(r, body)
	rec := grRequest{Method: r.Method, Path: r.URL.Path, Status: status}
	if recorded != nil {
		rec.Body = recorded
	}
	f.requests = append(f.requests, rec)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(out)
}

func dashboardFailure(status int, reason string) (int, any) {
	prefix := map[int]string{
		http.StatusBadRequest:          "Bad Request",
		http.StatusNotFound:            "Not Found",
		http.StatusInternalServerError: "Internal Server Error",
		http.StatusServiceUnavailable:  "Service Unavailable",
		http.StatusPreconditionFailed:  "Failed Precondition",
	}[status]
	if prefix != "" {
		reason = prefix + ": " + reason
	}
	return status, map[string]any{"code": status, "message": reason}
}

func (f *dashboardFake) operation(method, rest string) string {
	switch {
	case method == http.MethodPost && rest == "/dashboards/v1":
		return "Create"
	case method == http.MethodPut && rest == "/dashboards/v1":
		return "Replace"
	case method == http.MethodGet && rest == "/dashboards/v1/catalog/list":
		return "Catalog"
	case method == http.MethodGet && strings.HasPrefix(rest, "/dashboards/v1/"):
		return "Get"
	case method == http.MethodDelete && strings.HasPrefix(rest, "/dashboards/v1/"):
		return "Delete"
	case method == http.MethodPost && rest == "/check/v1":
		return "Check"
	}
	return ""
}

// catalog lists the dashboards, as the catalog route does.
func (f *dashboardFake) catalog() (int, any) {
	items := []any{}
	ids := make([]string, 0, len(f.dashboards))
	for id := range f.dashboards {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		items = append(items, map[string]any{
			"id": id, "name": f.dashboards[id]["name"],
			"permissions": map[string]any{"canRead": true, "canReadAccessPolicy": true, "canUpdate": true, "canUpdateAccessPolicy": true},
		})
	}
	return http.StatusOK, map[string]any{"items": items}
}

// injected returns the error or the reply that a scenario set for the operation.
func (f *dashboardFake) injected(op string) (int, any, bool) {
	if fault, ok := f.sticky[op]; ok {
		status, out := dashboardFailure(fault.status, fault.message)
		return status, out, true
	}
	if queue := f.faults[op]; len(queue) > 0 {
		f.faults[op] = queue[1:]
		status, out := dashboardFailure(queue[0].status, queue[0].message)
		return status, out, true
	}
	if queue := f.replies[op]; len(queue) > 0 {
		f.replies[op] = queue[1:]
		return http.StatusOK, queue[0], true
	}
	return 0, nil, false
}

// handle answers one request. The caller holds the lock.
func (f *dashboardFake) handle(r *http.Request, body map[string]any) (int, any) {
	_, rest, ok := strings.Cut(r.URL.Path, dashboardBasePath)
	if !ok {
		return dashboardFailure(http.StatusNotFound, "unknown path "+r.URL.Path)
	}
	op := f.operation(r.Method, rest)
	if op == "" {
		return dashboardFailure(http.StatusNotFound, "unknown path "+r.URL.Path)
	}
	if status, out, injected := f.injected(op); injected {
		return status, out
	}
	id := strings.TrimPrefix(strings.TrimPrefix(rest, "/dashboards/v1"), "/")
	switch op {
	case "Catalog":
		return f.catalog()
	case "Create":
		return f.create(body)
	case "Replace":
		return f.replace(body)
	case "Get":
		if len(id) != dashboardIDLength {
			return dashboardBadID()
		}
		dashboard, ok := f.dashboards[id]
		if !ok {
			return dashboardNotFound(id)
		}
		// The pinned SDK fails to decode a Get answer without permissions.
		out := map[string]any{
			"dashboard":   normalizeDashboardReadBack(dashboard),
			"permissions": map[string]any{"canRead": true, "canReadAccessPolicy": true, "canUpdate": true, "canUpdateAccessPolicy": true},
		}
		if policy, ok := f.policies[id]; ok {
			out["accessPolicy"] = policy
		}
		return http.StatusOK, out
	case "Delete":
		if len(id) != dashboardIDLength {
			return dashboardBadID()
		}
		if _, ok := f.dashboards[id]; !ok {
			return dashboardNotFound(id)
		}
		delete(f.dashboards, id)
		delete(f.policies, id)
		return http.StatusOK, map[string]any{}
	default:
		issues := f.issues
		if issues == nil {
			issues = []any{}
		}
		return http.StatusOK, map[string]any{"issues": issues}
	}
}

// resolveFolderPath turns a folderPath into the folderId of the folder at that path, as the API does
// on Create and Replace (the acceptance tests read the id back). A missing folder is created. It
// only runs when the folder fake is attached.
func (f *dashboardFake) resolveFolderPath(dashboard map[string]any) {
	folders, ok := f.folders.(*dashboardsFolderFake)
	path, _ := dashboard["folderPath"].(map[string]any)
	segments, _ := path["segments"].([]any)
	if !ok || len(segments) == 0 {
		return
	}
	folders.mu.Lock()
	defer folders.mu.Unlock()
	parent := ""
	for _, segment := range segments {
		name, _ := segment.(string)
		found := ""
		for id, folder := range folders.folders {
			if folder["name"] == name && folder["parentId"] == nilIfEmpty(parent) {
				found = id
			}
		}
		if found == "" {
			folders.seq++
			// Ids of folders that the API makes look like the recorder's masks.
			found = fmt.Sprintf("00000000-0000-4000-8000-9999999999%02d", folders.seq)
			folder := map[string]any{"id": found, "name": name}
			if parent != "" {
				folder["parentId"] = parent
			}
			folders.folders[found] = folder
		}
		parent = found
	}
	// The API reads back the folder id and not the path (recorded on EU2).
	delete(dashboard, "folderPath")
	dashboard["folderId"] = map[string]any{"value": parent}
}

func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func (f *dashboardFake) create(body map[string]any) (int, any) {
	dashboard, _ := body["dashboard"].(map[string]any)
	if dashboard == nil {
		return dashboardFailure(http.StatusBadRequest, "required field 'dashboard'")
	}
	if status, out := validateDashboardIDs(dashboard); status != http.StatusOK {
		return status, out
	}
	if status, out := validateDashboardArms(dashboard); status != http.StatusOK {
		return status, out
	}
	id, _ := dashboard["id"].(string)
	if id != "" && len(id) != dashboardIDLength {
		return dashboardBadID()
	}
	f.resolveFolderPath(dashboard)
	if status, out := f.validateFolder(dashboard); status != http.StatusOK {
		return status, out
	}
	if id == "" {
		f.seq++
		id = fmt.Sprintf("fakeboard%012d", f.seq)
	}
	stored := dashboardRoundTrip(dashboard).(map[string]any)
	stored["id"] = id
	f.dashboards[id] = stored
	// A dashboard created without a policy gets the default one.
	if policy, sent := body["accessPolicy"].(string); sent {
		f.policies[id] = normalizeDashboardPolicy(policy)
	} else {
		f.policies[id] = dashboardDefaultPolicy
	}
	return http.StatusOK, map[string]any{"dashboardId": id}
}

func (f *dashboardFake) replace(body map[string]any) (int, any) {
	dashboard, _ := body["dashboard"].(map[string]any)
	if dashboard == nil {
		return dashboardFailure(http.StatusBadRequest, "required field 'dashboard'")
	}
	id, _ := dashboard["id"].(string)
	sent, _ := body["dashboardId"].(string)
	switch {
	case sent != "" && len(sent) != dashboardIDLength, id != "" && len(id) != dashboardIDLength:
		return dashboardBadID()
	case id == "":
		return dashboardFailure(http.StatusBadRequest, "dashboard_id not found")
	case sent != "" && sent != id:
		return dashboardFailure(http.StatusBadRequest, "dashboard_id must equal dashboard.id")
	}
	if _, ok := f.dashboards[id]; !ok {
		return dashboardNotFound(id)
	}
	if status, out := validateDashboardIDs(dashboard); status != http.StatusOK {
		return status, out
	}
	if status, out := validateDashboardArms(dashboard); status != http.StatusOK {
		return status, out
	}
	f.resolveFolderPath(dashboard)
	if status, out := f.validateFolder(dashboard); status != http.StatusOK {
		return status, out
	}
	f.dashboards[id] = dashboardRoundTrip(dashboard).(map[string]any)
	// A Replace without accessPolicy keeps the policy that the dashboard has (recorded on EU2). A blank
	// one makes the dashboard read back with a blank policy.
	if policy, sent := body["accessPolicy"].(string); sent {
		f.policies[id] = normalizeDashboardPolicy(policy)
	}
	return http.StatusOK, map[string]any{}
}

// validateFolder rejects a folder id that no folder has (recorded on EU2).
func (f *dashboardFake) validateFolder(dashboard map[string]any) (int, any) {
	folderID, _ := dashboard["folderId"].(map[string]any)
	value, _ := folderID["value"].(string)
	folders, ok := f.folders.(*dashboardsFolderFake)
	if value == "" || !ok {
		return http.StatusOK, nil
	}
	folders.mu.Lock()
	defer folders.mu.Unlock()
	if _, exists := folders.folders[value]; !exists && !folders.seeded[value] {
		return dashboardFailure(http.StatusBadRequest, fmt.Sprintf("dashboard folder with id DashboardFolderId(%s) does not exist", value))
	}
	return http.StatusOK, nil
}

// addFolder makes a folder that a dashboard can name, as if it was made outside the test.
func (f *dashboardFake) addFolder(id, name string) {
	folders, ok := f.folders.(*dashboardsFolderFake)
	if !ok {
		panic("the fake has no folder store")
	}
	folders.mu.Lock()
	defer folders.mu.Unlock()
	folders.folders[id] = map[string]any{"id": id, "name": name}
}

// validateDashboardIDs is the structural rule that Create and Replace share: sections, rows and
// widgets need a non-empty id. The format is not checked.
func validateDashboardIDs(dashboard map[string]any) (int, any) {
	layout, _ := dashboard["layout"].(map[string]any)
	if layout == nil {
		return dashboardFailure(http.StatusBadRequest, "dashboard layout not found")
	}
	sections, _ := layout["sections"].([]any)
	for si, section := range sections {
		s, _ := section.(map[string]any)
		if idValue(s) == "" {
			return dashboardFailure(http.StatusBadRequest, fmt.Sprintf("layout.sections[%d]: section id is required", si))
		}
		if status, out := validateRowIDs(s, si); status != http.StatusOK {
			return status, out
		}
	}
	return http.StatusOK, nil
}

func validateRowIDs(section map[string]any, si int) (int, any) {
	rows, _ := section["rows"].([]any)
	for ri, row := range rows {
		r, _ := row.(map[string]any)
		if idValue(r) == "" {
			return dashboardFailure(http.StatusBadRequest, fmt.Sprintf("layout.sections[%d].rows[%d]: row id is required", si, ri))
		}
		widgets, _ := r["widgets"].([]any)
		for wi, widget := range widgets {
			w, _ := widget.(map[string]any)
			if idValue(w) == "" {
				return dashboardFailure(http.StatusBadRequest, fmt.Sprintf("layout.sections[%d].rows[%d].widgets[%d]: widget id is required", si, ri, wi))
			}
		}
	}
	return http.StatusOK, nil
}

// validateDashboardArms rejects a body with two arms of a oneof, as the API's protobuf parser does
// (recorded on EU2: error parsing "lineChart", oneof ... is already set). The groups come from the SDK
// source. The message names the arm and the type, and leaves out the position.
func validateDashboardArms(dashboard map[string]any) (int, any) {
	dashboardEnumZeroValues()
	if arm, owner := oneOfConflict(dashboard, reflect.TypeOf(dashboardservice.Dashboard{})); arm != "" {
		return dashboardFailure(http.StatusBadRequest, fmt.Sprintf("proto: error parsing %q, oneof %s is already set", arm, owner))
	}
	return http.StatusOK, nil
}

// oneOfConflict returns the second arm that a body sets in a oneof group, and the type that owns the group.
func oneOfConflict(value map[string]any, typ reflect.Type) (string, string) {
	for _, group := range oneOfGroups[typ.Name()] {
		seen := 0
		for _, arm := range group {
			if _, set := value[arm]; set {
				seen++
				if seen == 2 {
					return arm, typ.Name()
				}
			}
		}
	}
	for i := 0; i < typ.NumField(); i++ {
		name, _, _ := strings.Cut(typ.Field(i).Tag.Get("json"), ",")
		if name == "" || name == "-" {
			continue
		}
		if arm, owner := oneOfConflictIn(value[name], derefType(typ.Field(i).Type)); arm != "" {
			return arm, owner
		}
	}
	return "", ""
}

func oneOfConflictIn(child any, ft reflect.Type) (string, string) {
	switch {
	case ft.Kind() == reflect.Struct:
		if m, ok := child.(map[string]any); ok {
			return oneOfConflict(m, ft)
		}
	case ft.Kind() == reflect.Slice && derefType(ft.Elem()).Kind() == reflect.Struct:
		items, _ := child.([]any)
		for _, item := range items {
			if m, ok := item.(map[string]any); ok {
				if arm, owner := oneOfConflict(m, derefType(ft.Elem())); arm != "" {
					return arm, owner
				}
			}
		}
	}
	return "", ""
}

func idValue(m map[string]any) string {
	id, _ := m["id"].(map[string]any)
	value, _ := id["value"].(string)
	return value
}

func dashboardRoundTrip(v any) any {
	raw, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		panic(err)
	}
	return out
}

// normalizeDashboardReadBack fills what the API fills: omitted lists as [] and omitted enums as
// their first value. It walks the SDK type, so the fake needs no list of fields.
func normalizeDashboardReadBack(stored map[string]any) map[string]any {
	out := dashboardRoundTrip(stored).(map[string]any)
	fillReadBack(out, reflect.TypeOf(dashboardservice.Dashboard{}))
	fillWidgetHighlighted(out)
	return out
}

// fillWidgetHighlighted reads an unset widget highlighted back as false. The probe did not cover it.
// The evidence is the acceptance test TestAccCoralogixResourceDashboardAccessPolicy, which passes on
// the real API only if a widget created without highlighted reads back with a value. The EU2
// recording (fidelity case widget-highlighted) confirms or corrects it.
func fillWidgetHighlighted(dashboard map[string]any) {
	layout, _ := dashboard["layout"].(map[string]any)
	sections, _ := layout["sections"].([]any)
	for _, section := range sections {
		rows, _ := section.(map[string]any)["rows"].([]any)
		for _, row := range rows {
			widgets, _ := row.(map[string]any)["widgets"].([]any)
			for _, widget := range widgets {
				w, _ := widget.(map[string]any)
				if _, ok := w["highlighted"]; !ok {
					w["highlighted"] = false
				}
			}
		}
	}
}

func fillReadBack(value map[string]any, typ reflect.Type) {
	wasEmpty := len(value) == 0
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		if name == "" || name == "-" {
			continue
		}
		ft := derefType(field.Type)
		// An object sent as {} reads back with some strings filled. Each one is a recorded fact.
		if _, present := value[name]; wasEmpty && !present && filledEmptyStrings[typ.Name()+"."+name] {
			value[name] = ""
		}
		fillReadBackField(value, name, ft, typ.Name())
	}
}

func derefType(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t
}

func fillReadBackField(value map[string]any, name string, ft reflect.Type, owner string) {
	current, present := value[name]
	absent := !present || current == nil
	switch {
	case ft.Kind() == reflect.Slice && ft.Elem().Kind() != reflect.Uint8:
		if absent {
			value[name] = []any{}
			return
		}
		fillReadBackItems(current, derefType(ft.Elem()))
	case ft.Kind() == reflect.String && ft.Name() != "string":
		// An enum that is an arm of a oneof is not filled: a second arm would make the body invalid.
		// The API also leaves some enums unfilled. Each one is a recorded fact.
		if zero, ok := dashboardEnumZeroValues()[ft.Name()]; ok && absent && !oneOfMembers[owner][name] && !unfilledEnums[owner+"."+name] {
			value[name] = zero
		}
	case ft.Kind() == reflect.Struct && !absent:
		if m, ok := current.(map[string]any); ok {
			fillReadBack(m, ft)
		}
	}
}

func fillReadBackItems(current any, elem reflect.Type) {
	if elem.Kind() != reflect.Struct {
		return
	}
	items, _ := current.([]any)
	for _, item := range items {
		if m, ok := item.(map[string]any); ok {
			fillReadBack(m, elem)
		}
	}
}

// filledEmptyStrings are the strings that the API fills when their object is sent as {}. Recorded on
// EU2: dataprimeQuery {} reads back with text "". The API leaves other strings out, for example the
// minimumInterval of an empty time bucket "auto" (an acceptance test shows it).
var filledEmptyStrings = map[string]bool{
	"CommonDataprimeQuery.text": true,
}

// unfilledEnums are the enums that the API leaves out when a body omits them. Recorded on EU2.
var unfilledEnums = map[string]bool{
	"LineChart.xAxisTimeFormat": true,
}

var (
	enumZeroOnce   sync.Once
	enumZeroValues map[string]string
	// oneOfMembers lists, for each SDK type, the JSON names that belong to a oneof group.
	oneOfMembers map[string]map[string]bool
	// oneOfGroups lists the groups of each SDK type, each group as the JSON names of its arms.
	oneOfGroups map[string][][]string
)

var (
	typeNamePattern    = regexp.MustCompile(`(?m)^type (\w+) struct`)
	oneOfBlockPattern  = regexp.MustCompile(`(?s)OneOfGroup\d+Matches := 0(.*?)if \w+OneOfGroup\d+Matches > 1`)
	oneOfMemberPattern = regexp.MustCompile(`toSerialize\["(\w+)"\]`)
)

var enumValuesPattern = regexp.MustCompile(`var Allowed(\w+)EnumValues = \[\]\w+\{\s*"([^"]+)"`)

// dashboardEnumZeroValues maps each SDK enum type to its first allowed value, which is the value
// the API reads back for an omitted enum. It reads the pinned SDK source.
func dashboardEnumZeroValues() map[string]string {
	enumZeroOnce.Do(func() {
		enumZeroValues = map[string]string{}
		oneOfMembers = map[string]map[string]bool{}
		oneOfGroups = map[string][][]string{}
		out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", "github.com/coralogix/coralogix-management-sdk").Output()
		if err != nil {
			panic(fmt.Sprintf("find the SDK: %v", err))
		}
		files, err := filepath.Glob(filepath.Join(strings.TrimSpace(string(out)), "go/openapi/gen/dashboard_service/model_*.go"))
		if err != nil || len(files) == 0 {
			panic("no SDK dashboard models found")
		}
		for _, file := range files {
			raw, err := os.ReadFile(file)
			if err != nil {
				panic(err)
			}
			for _, m := range enumValuesPattern.FindAllStringSubmatch(string(raw), -1) {
				enumZeroValues[m[1]] = m[2]
			}
			collectOneOfMembers(string(raw))
		}
	})
	return enumZeroValues
}

// collectOneOfMembers reads the oneof groups of one SDK model file.
func collectOneOfMembers(source string) {
	typeName := typeNamePattern.FindStringSubmatch(source)
	if typeName == nil {
		return
	}
	for _, block := range oneOfBlockPattern.FindAllStringSubmatch(source, -1) {
		var group []string
		for _, member := range oneOfMemberPattern.FindAllStringSubmatch(block[1], -1) {
			if oneOfMembers[typeName[1]] == nil {
				oneOfMembers[typeName[1]] = map[string]bool{}
			}
			oneOfMembers[typeName[1]][member[1]] = true
			group = append(group, member[1])
		}
		oneOfGroups[typeName[1]] = append(oneOfGroups[typeName[1]], group)
	}
}
