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
	"slices"
	"strings"
	"sync"
	"testing"
)

// grRequest is one request that the provider sent to the fake backend.
type grRequest struct {
	Method string `json:"method"`
	Path   string `json:"path"`
	Body   any    `json:"body,omitempty"`
	Status int    `json:"status"`
}

// grFake is a small in-memory backend for global routers. It is checked
// against recorded answers of the real API (global_router_fidelity_test.go).
// It follows what was probed and recorded on a real tenant:
//   - the server sets createTime, updateTime, and every target id (new on each write)
//   - an omitted scalar or collection is returned as its zero value, but an unset routing label is left out
//   - a rule without entityType becomes ALERTS, and ENTITY_TYPE_UNSPECIFIED is refused
//   - the name of the router and of each rule, the condition, and the connector id are required
//   - a router other than router_default needs at least one routing label
//   - fallback and fallbackTargets exclude each other, and an unknown field is refused
//   - the router name is unique
//   - a client-set router id is kept, and a delete of an unknown id returns 200
//
// The fake does not check limits (rule count, label length), does not check
// that routing labels are unique, and does not look up connectors.
type grFake struct {
	mu       sync.Mutex
	routers  map[string]map[string]any
	seq      int
	clock    int
	requests []grRequest

	// reverseLists makes every response list the targets, fallback, and
	// fallbackTargets in reverse order. The real backend has no ORDER BY there.
	reverseLists bool
	// deleteStatus is the status of a delete. 0 means 200.
	deleteStatus int
}

const grBasePath = "/notifications/notification-center/v1/routers"

// grDefaultRouterID is the id of the default router. It needs no routing labels.
const grDefaultRouterID = "router_default"

func newGRFake(t *testing.T) *grFake {
	t.Helper()
	f := &grFake{routers: map[string]map[string]any{}}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)

	target, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	// The SDK sends requests through http.DefaultTransport. Send all of them to the fake.
	original := http.DefaultTransport
	http.DefaultTransport = redirectTransport{target: target, next: original}
	t.Cleanup(func() { http.DefaultTransport = original })
	return f
}

type redirectTransport struct {
	target *url.URL
	next   http.RoundTripper
}

func (r redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.URL.Scheme = r.target.Scheme
	req.URL.Host = r.target.Host
	req.Host = r.target.Host
	return r.next.RoundTrip(req)
}

// seed stores a router as if it was made outside Terraform. It returns the id.
func (f *grFake) seed(router map[string]any) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	status, body := f.write(map[string]any{"router": router}, false)
	if status != http.StatusOK {
		panic(fmt.Sprintf("seed failed: %d %v", status, body))
	}
	return body.(map[string]any)["router"].(map[string]any)["id"].(string)
}

// forget deletes a router without a request, as if it was deleted outside Terraform.
func (f *grFake) forget(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.routers, id)
}

// takeRequests returns the requests since the last call.
func (f *grFake) takeRequests() []grRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.requests
	f.requests = nil
	return out
}

func (f *grFake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	raw, _ := io.ReadAll(r.Body)
	rec := grRequest{Method: r.Method, Path: r.URL.Path}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &rec.Body)
	}
	status, body := f.handle(r.Method, r.URL.Path, rec.Body)
	// An omitted list or map and an empty one are the same message in proto3, so a recorded request
	// leaves out the empty ones. This keeps a golden file from showing that difference.
	rec.Body = dropEmptyCollections(rec.Body)
	rec.Status = status
	f.requests = append(f.requests, rec)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func (f *grFake) handle(method, path string, body any) (int, any) {
	_, id, ok := strings.Cut(path, grBasePath)
	if !ok {
		return grFail(http.StatusNotFound, "unknown path "+path)
	}
	id = strings.TrimPrefix(id, "/")

	switch {
	case method == http.MethodPost && id == "":
		return f.write(body, false)
	case method == http.MethodPut && id == "":
		return f.write(body, true)
	case method == http.MethodGet && id != "":
		router, found := f.routers[id]
		if !found {
			return grFail(http.StatusNotFound, notFoundReason(id))
		}
		return http.StatusOK, map[string]any{"router": f.present(router)}
	case method == http.MethodDelete && id != "":
		if f.deleteStatus != 0 {
			return grFail(f.deleteStatus, notFoundReason(id))
		}
		delete(f.routers, id)
		return http.StatusOK, map[string]any{}
	}
	return grFail(http.StatusMethodNotAllowed, "unsupported "+method+" "+path)
}

func notFoundReason(id string) string { return "Global router with id '" + id + "' not found" }

// grError is the error body of the real API: the message holds a JSON "reason".
func grError(status int, reason string) map[string]any {
	prefix := map[int]string{http.StatusBadRequest: "Bad Request", http.StatusNotFound: "Not Found"}[status]
	if prefix == "" {
		prefix = http.StatusText(status)
	}
	raw, _ := json.Marshal(map[string]string{"reason": reason})
	return map[string]any{"code": status, "message": prefix + ": " + string(raw)}
}

func grFail(status int, reason string) (int, any) { return status, grError(status, reason) }

// grProtoFail is the answer to a request that the JSON decoder refuses. The
// real message has a no-break space after "proto:".
func grProtoFail(detail string) (int, any) {
	return http.StatusBadRequest, map[string]any{
		"code": http.StatusBadRequest, "message": "Bad Request: proto: (line 1:12): " + detail,
	}
}

// uuidLike makes ids that look like the ids of the real server and are the same in every run.
func (f *grFake) uuidLike() string {
	f.seq++
	return fmt.Sprintf("00000000-0000-4000-8000-%012d", f.seq)
}

func (f *grFake) write(body any, replace bool) (int, any) {
	in, _ := body.(map[string]any)
	router, _ := in["router"].(map[string]any)
	if router == nil {
		return grFail(http.StatusBadRequest, "router is required")
	}
	if status, resp := f.check(router); status != 0 {
		return status, resp
	}

	id, _ := router["id"].(string)
	existing, found := f.routers[id]
	// The real API checks the name before the id.
	if !replace && f.nameTaken(router["name"], "") {
		return grFail(http.StatusBadRequest, fmt.Sprintf("Global router with name '%s' already exists", router["name"]))
	}
	switch {
	case replace && !found:
		if id == "" {
			id = f.uuidLike() // the real server looks up a new id, and does not find it
		}
		return grFail(http.StatusNotFound, notFoundReason(id))
	case !replace && found:
		return grFail(http.StatusBadRequest, "Global router with id '"+id+"' already exists")
	case !replace && id == "":
		id = f.uuidLike()
	}
	if replace && f.nameTaken(router["name"], id) {
		return grFail(http.StatusBadRequest, fmt.Sprintf("Global router with name '%s' already exists", router["name"]))
	}

	f.clock++
	stored := f.normalize(router)
	stored["id"] = id
	stored["updateTime"] = fmt.Sprintf("2026-01-01T00:00:%02d.000Z", f.clock)
	if replace {
		stored["createTime"] = existing["createTime"]
	} else {
		stored["createTime"] = stored["updateTime"]
	}
	f.routers[id] = stored
	return http.StatusOK, map[string]any{"router": f.present(stored)}
}

func (f *grFake) nameTaken(name any, ownID string) bool {
	for id, r := range f.routers {
		if id != ownID && r["name"] == name {
			return true
		}
	}
	return false
}

var (
	grRouterFields = []string{
		"id", "name", "description", "disabled", "entityLabels", "routingLabels", "rules",
		"fallback", "fallbackTargets", "entityType", "createTime", "updateTime",
	}
	// grSelectableEntityTypes are the values that a rule or a fallback target can have.
	grSelectableEntityTypes = []string{"ALERTS", "CASES", "TEST_NOTIFICATIONS", "OLLY_SCHEDULED_TASKS"}
)

// check returns a non-zero status and an error body for the first violation.
func (f *grFake) check(router map[string]any) (int, any) {
	for key := range router {
		if !slices.Contains(grRouterFields, key) {
			return grProtoFail(fmt.Sprintf("unknown field %q", key))
		}
	}
	if s, _ := router["name"].(string); s == "" {
		return grFail(http.StatusBadRequest, "GlobalRouter.name must not be empty")
	}
	if id, _ := router["id"].(string); id != grDefaultRouterID && !hasRoutingLabels(router["routingLabels"]) {
		return grFail(http.StatusBadRequest, "Routing labels cannot be empty.")
	}
	if status, resp := checkRules(router["rules"]); status != 0 {
		return status, resp
	}
	if status, resp := checkFallbacks(router); status != 0 {
		return status, resp
	}
	return 0, nil
}

func checkRules(rules any) (int, any) {
	for _, r := range asList(rules) {
		rule, _ := r.(map[string]any)
		if s, _ := rule["name"].(string); s == "" {
			return grFail(http.StatusBadRequest, "Every rule must have a name")
		}
		if s, _ := rule["condition"].(string); s == "" {
			return grFail(http.StatusBadRequest, "RoutingRule.condition must not be empty")
		}
		if et, has := rule["entityType"].(string); has {
			if status, resp := checkEntityType(et); status != 0 {
				return status, resp
			}
		}
		if status, resp := checkTargets(rule["targets"]); status != 0 {
			return status, resp
		}
	}
	return 0, nil
}

func checkFallbacks(router map[string]any) (int, any) {
	if status, resp := checkTargets(router["fallback"]); status != 0 {
		return status, resp
	}
	for _, ft := range asList(router["fallbackTargets"]) {
		m, _ := ft.(map[string]any)
		et, _ := m["entityType"].(string)
		if status, resp := checkEntityType(et); status != 0 {
			return status, resp
		}
		target, ok := m["target"].(map[string]any)
		if !ok {
			return grFail(http.StatusBadRequest, "fallback_target.target must be defined")
		}
		if status, resp := checkTargets([]any{target}); status != 0 {
			return status, resp
		}
	}
	if len(asList(router["fallback"])) > 0 && len(asList(router["fallbackTargets"])) > 0 {
		return grFail(http.StatusBadRequest, "Only one type of fallback can be provided.")
	}
	return 0, nil
}

func hasRoutingLabels(v any) bool {
	labels, _ := v.(map[string]any)
	for _, l := range labels {
		if s, _ := l.(string); s != "" {
			return true
		}
	}
	return false
}

// checkEntityType refuses a missing value, ENTITY_TYPE_UNSPECIFIED, and a name that is not an enum value.
func checkEntityType(et string) (int, any) {
	switch {
	case et == "" || et == "ENTITY_TYPE_UNSPECIFIED":
		return grFail(http.StatusBadRequest, "Valid entity_type must be provided")
	case !slices.Contains(grSelectableEntityTypes, et):
		return grProtoFail(fmt.Sprintf("invalid value for enum field entityType: %q", et))
	}
	return 0, nil
}

func checkTargets(v any) (int, any) {
	for _, tg := range asList(v) {
		m, _ := tg.(map[string]any)
		if s, _ := m["connectorId"].(string); s == "" {
			return grFail(http.StatusBadRequest, "'' is not a valid identifier")
		}
	}
	return 0, nil
}

// normalize returns the stored form: zero values filled in, ids generated.
func (f *grFake) normalize(in map[string]any) map[string]any {
	out := map[string]any{
		"name":            in["name"],
		"description":     orDefault(in["description"], ""),
		"disabled":        orDefault(in["disabled"], false),
		"entityLabels":    orDefault(in["entityLabels"], map[string]any{}),
		"rules":           []any{},
		"fallback":        f.targets(in["fallback"]),
		"fallbackTargets": []any{},
	}
	if labels, ok := in["routingLabels"].(map[string]any); ok {
		set := map[string]any{}
		for _, k := range []string{"environment", "service", "team"} {
			if s, _ := labels[k].(string); s != "" {
				set[k] = s
			}
		}
		out["routingLabels"] = set
	}
	rules := []any{}
	for _, r := range asList(in["rules"]) {
		rule, _ := r.(map[string]any)
		rules = append(rules, map[string]any{
			"name":          rule["name"],
			"condition":     rule["condition"],
			"entityType":    orDefault(rule["entityType"], "ALERTS"),
			"customDetails": orDefault(rule["customDetails"], map[string]any{}),
			"targets":       f.targets(rule["targets"]),
		})
	}
	out["rules"] = rules
	fallbackTargets := []any{}
	for _, ft := range asList(in["fallbackTargets"]) {
		m, _ := ft.(map[string]any)
		fallbackTargets = append(fallbackTargets, map[string]any{
			"entityType": m["entityType"],
			"target":     f.target(m["target"]),
		})
	}
	out["fallbackTargets"] = fallbackTargets
	return out
}

func (f *grFake) targets(v any) []any {
	out := []any{}
	for _, tg := range asList(v) {
		out = append(out, f.target(tg))
	}
	return out
}

func (f *grFake) target(v any) map[string]any {
	in, _ := v.(map[string]any)
	out := map[string]any{
		"id":            f.uuidLike(),
		"connectorId":   in["connectorId"],
		"customDetails": orDefault(in["customDetails"], map[string]any{}),
	}
	if p, ok := in["presetId"]; ok {
		out["presetId"] = p
	}
	return out
}

// present returns the router as a response, with list order changed when reverseLists is set.
func (f *grFake) present(stored map[string]any) map[string]any {
	raw, _ := json.Marshal(stored)
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	if !f.reverseLists {
		return out
	}
	for _, r := range asList(out["rules"]) {
		rule, _ := r.(map[string]any)
		slices.Reverse(rule["targets"].([]any))
	}
	slices.Reverse(out["fallback"].([]any))
	slices.Reverse(out["fallbackTargets"].([]any))
	return out
}

func orDefault(v, def any) any {
	if v == nil {
		return def
	}
	return v
}

func asList(v any) []any {
	l, _ := v.([]any)
	return l
}

// dropEmptyCollections removes the keys with an empty list or an empty map.
func dropEmptyCollections(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, e := range x {
			e = dropEmptyCollections(e)
			switch c := e.(type) {
			case []any:
				if len(c) == 0 {
					continue
				}
			case map[string]any:
				if len(c) == 0 {
					continue
				}
			}
			out[k] = e
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = dropEmptyCollections(e)
		}
		return out
	}
	return v
}

// mutate changes a stored router without a request, as if it was changed outside Terraform.
func (f *grFake) mutate(id string, change func(f *grFake, stored map[string]any)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	change(f, f.routers[id])
}

// setRuleTargets replaces the targets of rule i, as the API stores them. Each target is
// {"connectorId", "presetId"?, "customDetails"?}; the fake sets the target ids.
func (f *grFake) setRuleTargets(stored map[string]any, rule int, targets []any) {
	r := stored["rules"].([]any)[rule].(map[string]any)
	r["targets"] = f.targets(targets)
}
