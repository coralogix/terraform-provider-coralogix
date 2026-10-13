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
	"reflect"
	"regexp"
	"strings"
	"sync"
	"testing"
)

const sloTypeName = "coralogix_slo_v2"

const sloBasePath = "/slo/slos/v1"

func newSLOHarness(t *testing.T) *grHarness {
	return newProtocolHarness(t, sloTypeName, newSLOFake(t))
}

func (h *grHarness) slos() *sloFake {
	h.t.Helper()
	f, ok := h.api.(*sloFake)
	if !ok {
		h.t.Fatal("expected *sloFake")
	}
	return f
}

// sloFake follows the SLO service as recorded in fidelity/recorded.json, not the Go client.
// Create and replace take the SLO itself as the body. Create returns it as "slo" next to
// exclusionWindowIds, replace next to exclusionWindowIds and effectedSloAlertIds, and delete
// returns effectedSloAlertIds. The server sets id, grouping (the by clause of the query, or
// service_name for an APM SLI), sloType, type, creator, revision (which moves when the SLI
// changes), and the times. labels is {} when the request has none. An omitted
// targetThresholdPercentage or threshold reads as 0, an omitted missingDataStrategy as
// UNCOUNTED, also on a replace, and an omitted productType as UNSPECIFIED. An APM SLI needs
// productType APM, and gets an apmSliMetadata copy. Every APM SLI returns filters and
// groupingKeys, empty when omitted. A written ownership dimension returns staticValues and
// labelKeys as sent, empty when omitted, and an empty resolvedValues; a dimension with no values
// is dropped.
type sloFake struct {
	mu           sync.Mutex
	slos         map[string]map[string]any
	seq          int
	clock        int
	requests     []grRequest
	deleteStatus int
}

func newSLOFake(t *testing.T) *sloFake {
	t.Helper()
	f := &sloFake{slos: map[string]map[string]any{}}
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

func (f *sloFake) seed(slo map[string]any) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	status, body := f.write(slo, false)
	if status != http.StatusOK {
		panic(fmt.Sprintf("seed failed: %d %v", status, body))
	}
	return body.(map[string]any)["slo"].(map[string]any)["id"].(string)
}

// seedRaw stores a record as is, for shapes that the API no longer writes, such as an APM
// filter with only the deprecated singular value.
func (f *sloFake) seedRaw(slo map[string]any) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seq++
	id := fmt.Sprintf("00000000-0000-4000-8000-%012d", f.seq)
	slo["id"] = id
	f.slos[id] = slo
	return id
}

func (f *sloFake) forget(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.slos, id)
}

func (f *sloFake) takeRequests() []grRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.requests
	f.requests = nil
	return out
}

func (f *sloFake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	raw, _ := io.ReadAll(r.Body)
	rec := grRequest{Method: r.Method, Path: r.URL.Path}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &rec.Body)
	}
	status, body := f.handle(r.Method, r.URL.Path, rec.Body)
	rec.Body = dropEmptyCollections(rec.Body)
	rec.Status = status
	f.requests = append(f.requests, rec)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func (f *sloFake) handle(method, path string, body any) (int, any) {
	_, rest, ok := strings.Cut(path, sloBasePath)
	if !ok {
		return sloFail(http.StatusNotFound, "unknown path "+path)
	}
	in, _ := body.(map[string]any)
	switch {
	case method == http.MethodPost && rest == "":
		return f.write(in, false)
	case method == http.MethodPut && rest == "":
		return f.write(in, true)
	case method == http.MethodGet && strings.HasPrefix(rest, "/"):
		id := strings.TrimPrefix(rest, "/")
		stored, found := f.slos[id]
		if !found {
			return sloFail(http.StatusNotFound, "Slo with id "+id+" not found for company 1")
		}
		return http.StatusOK, map[string]any{"slo": stored}
	case method == http.MethodDelete && strings.HasPrefix(rest, "/"):
		id := strings.TrimPrefix(rest, "/")
		if f.deleteStatus != 0 {
			return sloFail(f.deleteStatus, "Slo with id "+id+" not found for company 1")
		}
		if _, found := f.slos[id]; !found {
			return sloFail(http.StatusNotFound, "Slo with id "+id+" not found for company 1")
		}
		delete(f.slos, id)
		return http.StatusOK, map[string]any{"effectedSloAlertIds": []any{}}
	}
	return sloFail(http.StatusMethodNotAllowed, "unsupported "+method+" "+path)
}

// sloFail is an error answer of the SLO service: the status text, a colon, and the message.
func sloFail(status int, message string) (int, any) {
	return status, map[string]any{"code": status, "message": http.StatusText(status) + ": " + message}
}

// sloValidation is the answer to a request that the validation layer refuses.
func sloValidation(replace bool, reason string) (int, any) {
	method := "CreateSlo"
	if replace {
		method = "ReplaceSlo"
	}
	return sloFail(http.StatusBadRequest, "validation failed for method /com.coralogixapis.slo.v1.SlosService/"+method+": ValidationError: "+reason)
}

var sloArms = []string{"apmSli", "requestBasedMetricSli", "windowBasedMetricSli"}

func (f *sloFake) write(in map[string]any, replace bool) (int, any) {
	if in == nil {
		return sloFail(http.StatusBadRequest, "slo is required")
	}
	if status, resp := f.check(in, replace); status != 0 {
		return status, resp
	}
	stored := map[string]any{
		"name":                      in["name"],
		"targetThresholdPercentage": 0,
		"sloTimeFrame":              in["sloTimeFrame"],
		"productType":               "SLO_PRODUCT_TYPE_UNSPECIFIED",
		"labels":                    map[string]any{},
	}
	if v, ok := in["targetThresholdPercentage"]; ok {
		stored["targetThresholdPercentage"] = v
	}
	if desc, _ := in["description"].(string); desc != "" {
		stored["description"] = desc
	}
	if labels, _ := in["labels"].(map[string]any); len(labels) > 0 {
		stored["labels"] = labels
	}
	if pt, _ := in["productType"].(string); pt != "" {
		stored["productType"] = pt
	}
	if tags := sloOwnershipTags(in["ownershipTags"]); tags != nil {
		stored["ownershipTags"] = tags
	}
	arm := ""
	switch {
	case in["requestBasedMetricSli"] != nil:
		arm = "requestBasedMetricSli"
		sli := in[arm].(map[string]any)
		stored[arm] = sli
		stored["grouping"] = map[string]any{"labels": sloGroupBy(sli["totalEvents"])}
	case in["windowBasedMetricSli"] != nil:
		arm = "windowBasedMetricSli"
		sli := cloneMap(in[arm].(map[string]any))
		if sli["missingDataStrategy"] == nil {
			sli["missingDataStrategy"] = "MISSING_DATA_STRATEGY_UNCOUNTED"
		}
		if sli["threshold"] == nil {
			sli["threshold"] = 0
		}
		stored[arm] = sli
		stored["grouping"] = map[string]any{"labels": sloGroupBy(sli["query"])}
	default:
		arm = "apmSli"
		sli := sloAPMDefaults(in[arm].(map[string]any))
		stored[arm] = sli
		stored["apmSliMetadata"] = cloneMap(sli)
		stored["grouping"] = map[string]any{"labels": []any{"service_name"}}
	}
	stored["sloType"], stored["type"] = sloTypeOf(in), "request"
	if stored["sloType"] == "SLO_TYPE_WINDOW" {
		stored["type"] = "window"
	}

	f.clock++
	now := fmt.Sprintf("2026-01-01T00:00:%02dZ", f.clock)
	id, _ := in["id"].(string)
	var prevDesc any
	if replace {
		prev := f.slos[id]
		prevDesc = prev["description"]
		stored["createTime"], stored["creator"] = prev["createTime"], prev["creator"]
		prevRevision, _ := prev["revision"].(map[string]any)
		revision, _ := prevRevision["revision"].(int)
		if !reflect.DeepEqual(prev[arm], stored[arm]) {
			revision++
		}
		stored["revision"] = map[string]any{"revision": revision, "updateTime": now}
	} else {
		f.seq++
		id = fmt.Sprintf("00000000-0000-4000-8000-%012d", f.seq)
		stored["createTime"], stored["creator"] = now, "terraform@example.com"
		stored["revision"] = map[string]any{"revision": 1, "updateTime": now}
	}
	stored["id"] = id
	stored["updateTime"] = now
	f.slos[id] = stored
	if _, sent := in["description"]; replace && !sent {
		// The store skips a column that the request leaves out, so a replace without a
		// description keeps the stored one. The response still shows the request.
		if prevDesc != nil {
			kept := cloneMap(stored)
			kept["description"] = prevDesc
			f.slos[id] = kept
		}
	}
	if replace {
		return http.StatusOK, map[string]any{"slo": stored, "effectedSloAlertIds": []any{}, "exclusionWindowIds": []any{}}
	}
	return http.StatusOK, map[string]any{"slo": stored, "exclusionWindowIds": []any{}}
}

// sloTypeOf is the SLO type that the server derives from the SLI. A window-based metric SLI
// reads SLO_TYPE_REQUEST; an APM latency SLI reads SLO_TYPE_WINDOW.
func sloTypeOf(in map[string]any) string {
	if apm, ok := in["apmSli"].(map[string]any); ok && apm["latencyConfig"] != nil {
		return "SLO_TYPE_WINDOW"
	}
	return "SLO_TYPE_REQUEST"
}

func (f *sloFake) check(in map[string]any, replace bool) (int, any) {
	var set []string
	for _, arm := range sloArms {
		if in[arm] != nil {
			set = append(set, arm)
		}
	}
	if len(set) > 1 {
		// The JSON decoder stops at the second arm of the oneOf.
		return sloFail(http.StatusBadRequest, "proto: (line 1:1): error parsing \""+set[1]+"\", oneof com.coralogixapis.slo.v1.Slo.sli is already set")
	}
	if in["grouping"] != nil {
		return sloValidation(replace, `"slo.grouping" is not allowed`)
	}
	if name, _ := in["name"].(string); name == "" {
		return sloValidation(replace, `"slo.name" is not allowed to be empty`)
	}
	if in["sloTimeFrame"] == nil {
		return sloValidation(replace, `"slo.window" is required`)
	}
	if got, _ := in["sloType"].(string); got != "" && got != sloTypeOf(in) {
		return sloValidation(replace, "sloType must match the SLI type. Expected "+sloTypeOf(in)+", got "+got)
	}
	if in["apmSliMetadata"] != nil {
		return sloValidation(replace, `"slo.apmSliMetadata" is not allowed`)
	}
	if len(set) == 0 {
		return sloValidation(replace, `"slo.sli" is required`)
	}
	if status, resp := sloCheckQueries(in); status != 0 {
		return status, resp
	}
	if window, ok := in["windowBasedMetricSli"].(map[string]any); ok {
		if op, _ := window["comparisonOperator"].(string); op == "" || op == "COMPARISON_OPERATOR_UNSPECIFIED" {
			return sloFail(http.StatusBadRequest, "Invalid comparison operator: 0")
		}
		if w, _ := window["window"].(string); w == "" || w == "WINDOW_SLO_WINDOW_UNSPECIFIED" {
			return sloFail(http.StatusInternalServerError, "failed to create SLO: Not implemented yet: WindowSloWindow.WINDOW_SLO_WINDOW_UNSPECIFIED case")
		}
	}
	if apm, ok := in["apmSli"].(map[string]any); ok {
		if status, resp := sloCheckAPM(in, apm); status != 0 {
			return status, resp
		}
	}
	if replace {
		id, _ := in["id"].(string)
		if _, found := f.slos[id]; !found {
			return sloFail(http.StatusNotFound, "Cannot replace Slo with id "+id+" because it does not exist")
		}
	}
	return 0, nil
}

// sloCheckQueries refuses a metric SLI without a query.
func sloCheckQueries(in map[string]any) (int, any) {
	missing := func(metric any) bool {
		m, _ := metric.(map[string]any)
		q, _ := m["query"].(string)
		return q == ""
	}
	if sli, ok := in["requestBasedMetricSli"].(map[string]any); ok {
		if missing(sli["goodEvents"]) {
			return sloFail(http.StatusBadRequest, "Good events query: Query is missing")
		}
		if missing(sli["totalEvents"]) {
			return sloFail(http.StatusBadRequest, "Total events query: Query is missing")
		}
	}
	if sli, ok := in["windowBasedMetricSli"].(map[string]any); ok && missing(sli["query"]) {
		return sloFail(http.StatusBadRequest, "Window query: Query is missing")
	}
	return 0, nil
}

// sloCheckAPM refuses an APM SLI that the released resource documents as invalid. The messages
// are not recorded: the recording tenant has no APM service.
func sloCheckAPM(in, apm map[string]any) (int, any) {
	if in["productType"] != "SLO_PRODUCT_TYPE_APM" {
		return sloFail(http.StatusBadRequest, "an APM SLI needs productType SLO_PRODUCT_TYPE_APM")
	}
	if services, _ := apm["services"].([]any); len(services) != 1 {
		return sloFail(http.StatusBadRequest, "an APM SLI needs exactly one service")
	}
	if (apm["errorConfig"] == nil) == (apm["latencyConfig"] == nil) {
		return sloFail(http.StatusBadRequest, "an APM SLI needs errorConfig or latencyConfig")
	}
	if latency, ok := apm["latencyConfig"].(map[string]any); ok {
		if latency["timeWindow"] == nil {
			return sloFail(http.StatusBadRequest, "latencyConfig.timeWindow is required")
		}
		if (latency["quantile"] == nil) == (latency["average"] == nil) {
			return sloFail(http.StatusBadRequest, "latencyConfig needs quantile or average")
		}
	}
	return 0, nil
}

// sloAPMDefaults stores the defaults of an APM SLI and the empty arrays that the API returns.
func sloAPMDefaults(in map[string]any) map[string]any {
	sli := cloneMap(in)
	if sli["filters"] == nil {
		sli["filters"] = []any{}
	}
	if sli["groupingKeys"] == nil {
		sli["groupingKeys"] = []any{}
	}
	if latency, ok := sli["latencyConfig"].(map[string]any); ok {
		latency = cloneMap(latency)
		if latency["threshold"] == nil {
			latency["threshold"] = 0
		}
		if quantile, ok := latency["quantile"].(map[string]any); ok {
			quantile = cloneMap(quantile)
			if quantile["percentile"] == nil {
				quantile["percentile"] = 0
			}
			latency["quantile"] = quantile
		}
		sli["latencyConfig"] = latency
	}
	return sli
}

// sloOwnershipTags drops a dimension with no values, and fills the arrays of the others. The
// static values stay as sent. resolvedValues is empty right after a write: the server resolves it
// later.
func sloOwnershipTags(in any) map[string]any {
	tags, _ := in.(map[string]any)
	out := map[string]any{}
	for _, dim := range []string{"service", "environment", "team"} {
		tag, _ := tags[dim].(map[string]any)
		static, _ := tag["staticValues"].([]any)
		keys, _ := tag["labelKeys"].([]any)
		if len(static) == 0 && len(keys) == 0 {
			continue
		}
		out[dim] = map[string]any{
			"staticValues":   append([]any{}, static...),
			"labelKeys":      append([]any{}, keys...),
			"resolvedValues": []any{},
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

var sloByClause = regexp.MustCompile(`by\s*\(([^)]*)\)`)

// sloGroupBy returns the labels of the by (...) clause of a metric query.
func sloGroupBy(metric any) []any {
	m, _ := metric.(map[string]any)
	query, _ := m["query"].(string)
	match := sloByClause.FindStringSubmatch(query)
	out := []any{}
	if match == nil {
		return out
	}
	for _, label := range strings.Split(match[1], ",") {
		if label = strings.TrimSpace(label); label != "" {
			out = append(out, label)
		}
	}
	return out
}

func cloneMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
