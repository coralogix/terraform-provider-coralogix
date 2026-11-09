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
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"

	dashboardservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_service"
)

const dashboardBasePath = "/dashboards"

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
	original := http.DefaultTransport
	http.DefaultTransport = dashboardTransport{fake: f, next: redirectTransport{target: target, next: original}}
	return f, func() {
		http.DefaultTransport = original
		srv.Close()
	}
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
	status, out := f.handle(r, body)
	rec := grRequest{Method: r.Method, Path: r.URL.Path, Status: status}
	if body != nil {
		delete(body, "requestId")
		clean, err := json.Marshal(body)
		if err != nil {
			panic(err)
		}
		rec.Body = json.RawMessage(clean)
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
	case "Create":
		return f.create(body)
	case "Replace":
		return f.replace(body)
	case "Get":
		dashboard, ok := f.dashboards[id]
		if !ok {
			return dashboardFailure(http.StatusNotFound, fmt.Sprintf("DashboardId(%s)", id))
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
		if _, ok := f.dashboards[id]; !ok {
			return dashboardFailure(http.StatusNotFound, fmt.Sprintf("DashboardId(%s)", id))
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
			found = fmt.Sprintf("00000000-0000-4000-8000-%012d", folders.seq)
			folder := map[string]any{"id": found, "name": name}
			if parent != "" {
				folder["parentId"] = parent
			}
			folders.folders[found] = folder
		}
		parent = found
	}
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
		return dashboardFailure(http.StatusBadRequest, "dashboard is required")
	}
	if status, out := validateDashboardIDs(dashboard); status != http.StatusOK {
		return status, out
	}
	if status, out := validateDashboardArms(dashboard); status != http.StatusOK {
		return status, out
	}
	f.resolveFolderPath(dashboard)
	id, _ := dashboard["id"].(string)
	if id == "" {
		f.seq++
		id = fmt.Sprintf("fakeboard%012d", f.seq)
	}
	stored := dashboardRoundTrip(dashboard).(map[string]any)
	stored["id"] = id
	f.dashboards[id] = stored
	if policy, _ := body["accessPolicy"].(string); policy != "" {
		f.policies[id] = policy
	}
	return http.StatusOK, map[string]any{"dashboardId": id}
}

func (f *dashboardFake) replace(body map[string]any) (int, any) {
	dashboard, _ := body["dashboard"].(map[string]any)
	if dashboard == nil {
		return dashboardFailure(http.StatusBadRequest, "dashboard is required")
	}
	id, _ := dashboard["id"].(string)
	if sent, ok := body["dashboardId"].(string); ok && sent != id {
		return dashboardFailure(http.StatusBadRequest, "dashboard.id must be equal to dashboardId")
	}
	if _, ok := f.dashboards[id]; !ok {
		return dashboardFailure(http.StatusNotFound, fmt.Sprintf("DashboardId(%s)", id))
	}
	if status, out := validateDashboardIDs(dashboard); status != http.StatusOK {
		return status, out
	}
	if status, out := validateDashboardArms(dashboard); status != http.StatusOK {
		return status, out
	}
	f.resolveFolderPath(dashboard)
	f.dashboards[id] = dashboardRoundTrip(dashboard).(map[string]any)
	// A Replace without accessPolicy keeps the policy that the dashboard has. The acceptance test
	// TestAccCoralogixResourceDashboardAccessPolicy relies on it: it removes the policy from the
	// config and expects the policy to stay.
	if policy, _ := body["accessPolicy"].(string); policy != "" {
		f.policies[id] = policy
	}
	return http.StatusOK, map[string]any{}
}

// validateDashboardIDs is the structural rule that Create and Replace share: sections, rows and
// widgets need a non-empty id. The format is not checked.
func validateDashboardIDs(dashboard map[string]any) (int, any) {
	layout, _ := dashboard["layout"].(map[string]any)
	sections, _ := layout["sections"].([]any)
	for _, section := range sections {
		s, _ := section.(map[string]any)
		if idValue(s) == "" {
			return dashboardFailure(http.StatusBadRequest, "section id is required")
		}
		rows, _ := s["rows"].([]any)
		for _, row := range rows {
			r, _ := row.(map[string]any)
			if idValue(r) == "" {
				return dashboardFailure(http.StatusBadRequest, "row id is required")
			}
			widgets, _ := r["widgets"].([]any)
			for _, widget := range widgets {
				w, _ := widget.(map[string]any)
				if idValue(w) == "" {
					return dashboardFailure(http.StatusBadRequest, "widget id is required")
				}
			}
		}
	}
	return http.StatusOK, nil
}

var oneOfViolation = regexp.MustCompile(`(at most|exactly) one of \[[^\]]*\] may be set|exactly one of`)

// validateDashboardArms rejects a body with two arms of a oneof, as the API's JSON parser does
// (probe: both arms at once were rejected in every message). The check is the SDK's own decoder,
// and only its oneof errors count, because the SDK also enforces fields that the API does not.
func validateDashboardArms(dashboard map[string]any) (int, any) {
	raw, err := json.Marshal(dashboard)
	if err != nil {
		panic(err)
	}
	var decoded dashboardservice.Dashboard
	if err := json.Unmarshal(raw, &decoded); err != nil && oneOfViolation.MatchString(err.Error()) {
		return dashboardFailure(http.StatusBadRequest, err.Error())
	}
	return http.StatusOK, nil
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
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		if name == "" || name == "-" {
			continue
		}
		fillReadBackField(value, name, derefType(field.Type), typ.Name())
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
		if zero, ok := dashboardEnumZeroValues()[ft.Name()]; ok && absent && !oneOfMembers[owner][name] {
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

var (
	enumZeroOnce   sync.Once
	enumZeroValues map[string]string
	// oneOfMembers lists, for each SDK type, the JSON names that belong to a oneof group.
	oneOfMembers map[string]map[string]bool
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
		for _, member := range oneOfMemberPattern.FindAllStringSubmatch(block[1], -1) {
			if oneOfMembers[typeName[1]] == nil {
				oneOfMembers[typeName[1]] = map[string]bool{}
			}
			oneOfMembers[typeName[1]][member[1]] = true
		}
	}
}
