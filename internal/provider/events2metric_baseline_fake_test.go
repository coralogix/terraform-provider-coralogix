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
	"slices"
	"strings"
	"sync"
	"testing"
)

const e2mBasePath = "/events2metrics/events2metrics/v2"

// e2mDefaultPermutationsLimit is the limit the server stores when a request has none.
const e2mDefaultPermutationsLimit = 30000

// e2mLabelLimit is the default number of metric labels an events2metric can have.
const e2mLabelLimit = 6

var e2mNamePattern = regexp.MustCompile(`^[\w/-]+$`)

// e2mDefaultAggregations is the template the server fills every metric field from on Create.
// On Replace the template is the stored aggregations of the field with the same base name.
var e2mDefaultAggregations = []map[string]any{
	{"aggType": "AGG_TYPE_MIN", "targetMetricName": "cx_min", "enabled": true, "none": map[string]any{}},
	{"aggType": "AGG_TYPE_MAX", "targetMetricName": "cx_max", "enabled": true, "none": map[string]any{}},
	{"aggType": "AGG_TYPE_COUNT", "targetMetricName": "cx_count", "enabled": true, "none": map[string]any{}},
	{"aggType": "AGG_TYPE_AVG", "targetMetricName": "cx_avg", "enabled": true, "none": map[string]any{}},
	{"aggType": "AGG_TYPE_SUM", "targetMetricName": "cx_sum", "enabled": true, "none": map[string]any{}},
	{"aggType": "AGG_TYPE_SAMPLES", "targetMetricName": "cx_value", "enabled": true, "samples": map[string]any{"sampleType": "SAMPLE_TYPE_MAX"}},
	{"aggType": "AGG_TYPE_HISTOGRAM", "targetMetricName": "cx_bucket", "enabled": false, "histogram": map[string]any{"buckets": []any{}}},
}

// e2mFake is a small in-memory backend for events2metrics. It follows the
// server code of the events2metrics module (validator, service, and models):
//   - Create takes the definition itself, Replace takes the full events2metric with its id;
//     both answer {"e2m": ...}
//   - every metric field gets all seven aggregations: an omitted one comes from the template
//     (the defaults on Create, the stored field on Replace), an omitted enabled in a sent one
//     is false, and targetMetricName is always the template name (cx_min, ...), never the one sent
//   - metric fields and aggregations come back in no particular order; this fake sorts them by name
//   - permutationsLimit (Create) or permutations (Replace) left out is 30000; a limit below 1
//     with metric labels is refused; hasExceededLimit is false in write answers
//   - type is required and must match the query; an omitted query is the empty query of the type
//   - an omitted lucene is "", alias is dropped, omitted filters are []
//   - Replace keeps the stored description when the request has none; dataSource left out is cleared
//   - names and metric field base names are unique; labels are unique per events2metric, at most six
//   - Get of a missing id is 404; Replace and Delete of a missing id are 400
//
// It is checked against answers recorded from a real tenant (events2metric_fidelity_test.go),
// including the text of every error. It does not check that a source field is numeric, does not
// validate lucene, and does not check the company-wide permutations budget.
type e2mFake struct {
	mu       sync.Mutex
	e2ms     map[string]map[string]any
	seq      int
	clock    int
	requests []grRequest
	// exceeded marks ids whose Get reports hasExceededLimit.
	exceeded map[string]bool
}

func newE2MFake(t *testing.T) *e2mFake {
	t.Helper()
	f := &e2mFake{e2ms: map[string]map[string]any{}, exceeded: map[string]bool{}}
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

// seed stores an events2metric as if it was made outside Terraform. It returns the id.
func (f *e2mFake) seed(def map[string]any) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	status, body := f.write(def, false)
	if status != http.StatusOK {
		panic(fmt.Sprintf("seed failed: %d %v", status, body))
	}
	return body.(map[string]any)["e2m"].(map[string]any)["id"].(string)
}

// forget deletes an events2metric without a request, as if it was deleted outside Terraform.
func (f *e2mFake) forget(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.e2ms, id)
}

func (f *e2mFake) takeRequests() []grRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.requests
	f.requests = nil
	return out
}

func (f *e2mFake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	raw, _ := io.ReadAll(r.Body)
	rec := grRequest{Method: r.Method, Path: r.URL.Path}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &rec.Body)
	}
	status, body := f.handle(r.Method, r.URL.Path, rec.Body)
	// Empty lists and objects stay in the recorded request: sending {} or [] and leaving the key
	// out are different requests, and the switch to generated code must show such a change.
	rec.Body = e2mSortRequest(rec.Body)
	rec.Status = status
	f.requests = append(f.requests, rec)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func (f *e2mFake) handle(method, path string, body any) (int, any) {
	_, id, ok := strings.Cut(path, e2mBasePath)
	if !ok {
		return grFail(http.StatusNotFound, "unknown path "+path)
	}
	id = strings.TrimPrefix(id, "/")

	switch {
	case method == http.MethodPost && id == "":
		in, _ := body.(map[string]any)
		return f.write(in, false)
	case method == http.MethodPut && id == "":
		in, _ := body.(map[string]any)
		return f.write(in, true)
	case method == http.MethodGet && id != "":
		stored, found := f.e2ms[id]
		if !found {
			return e2mNotFound(id)
		}
		return http.StatusOK, map[string]any{"e2m": f.present(stored, f.exceeded[id])}
	case method == http.MethodDelete && id != "":
		if _, found := f.e2ms[id]; !found {
			return e2mBadRequest(e2mMissingID)
		}
		delete(f.e2ms, id)
		return http.StatusOK, map[string]any{"id": id}
	}
	return grFail(http.StatusMethodNotAllowed, "unsupported "+method+" "+path)
}

func e2mFail(status int, message string) (int, any) {
	return status, map[string]any{"code": status, "message": message}
}

// e2mNotFound is the answer of the real API to a Get of a missing id.
func e2mNotFound(id string) (int, any) {
	raw, _ := json.Marshal(map[string]any{"message": "NOT_FOUND: Events2Metrics with id " + id + " not found", "status": 5})
	return e2mFail(http.StatusNotFound, "Not Found: "+string(raw))
}

// e2mBadRequest is the answer of the real API to a request that its validation refuses. detail
// is one zod issue, as JSON text, because the real API keeps the key order of each issue kind.
func e2mBadRequest(detail string) (int, any) {
	return e2mFail(http.StatusBadRequest, `Bad Request: {"message":"Bad Request","status":400,"details":[`+detail+`]}`)
}

// e2mCustomIssue is a zod issue that the service validator adds.
func e2mCustomIssue(message string, path ...string) string {
	m, _ := json.Marshal(message)
	p, _ := json.Marshal(path)
	return `{"code":"custom","message":` + string(m) + `,"path":` + string(p) + `}`
}

const e2mMissingID = `{"code":"custom","message":"L2M does not exist","path":["id"]}`

func (f *e2mFake) uuidLike() string {
	f.seq++
	return fmt.Sprintf("00000000-0000-4000-8000-%012d", f.seq)
}

func (f *e2mFake) write(in map[string]any, replace bool) (int, any) {
	if in == nil {
		return e2mFail(http.StatusBadRequest, "body is required")
	}
	var prior map[string]any
	id := f.uuidLike()
	if replace {
		id, _ = in["id"].(string)
		var found bool
		if prior, found = f.e2ms[id]; !found {
			return e2mBadRequest(e2mMissingID)
		}
	}
	if status, resp := f.check(in, id); status != 0 {
		return status, resp
	}
	stored, status, resp := f.normalize(in, prior, replace)
	if status != 0 {
		return status, resp
	}
	stored["id"] = id
	f.clock++
	stored["updateTime"] = fmt.Sprintf("2026-01-01T00:00:%02dZ", f.clock)
	if prior != nil {
		stored["createTime"] = prior["createTime"]
	} else {
		stored["createTime"] = stored["updateTime"]
	}
	f.e2ms[id] = stored
	return http.StatusOK, map[string]any{"e2m": f.present(stored, false)}
}

func (f *e2mFake) check(in map[string]any, ownID string) (int, any) {
	name, ok := in["name"].(string)
	if !ok {
		return e2mBadRequest(`{"code":"invalid_type","expected":"string","received":"undefined","path":["name"],"message":"Required"}`)
	}
	// The zod model checks the type and its query first, then the service checks the name.
	typ, _ := in["type"].(string)
	switch typ {
	case "E2M_TYPE_LOGS2METRICS":
		if in["spansQuery"] != nil {
			return e2mQueryMismatch("spansQuery", "logsQuery")
		}
	case "E2M_TYPE_SPANS2METRICS":
		if in["logsQuery"] != nil {
			return e2mQueryMismatch("logsQuery", "spansQuery")
		}
	default:
		return e2mBadRequest(`{"code":"invalid_union_discriminator","options":[1,2],"path":["type"],"message":"Invalid discriminator value. Expected 1 | 2"}`)
	}
	for id, other := range f.e2ms {
		if id != ownID && other["name"] == name {
			m, _ := json.Marshal(fmt.Sprintf("Name (%s) is not unique", name))
			return e2mBadRequest(`{"code":"custom","path":["e2m","name"],"message":` + string(m) + `}`)
		}
	}
	if status, resp := f.checkFields(asList(in["metricFields"]), ownID); status != 0 {
		return status, resp
	}
	return checkLabels(asList(in["metricLabels"]))
}

func e2mQueryMismatch(received, expected string) (int, any) {
	return e2mBadRequest(`{"received":"` + received + `","code":"invalid_literal","expected":"` + expected +
		`","path":["query","$case"],"message":"Invalid literal value, expected \"` + expected + `\""}`)
}

func (f *e2mFake) checkFields(fields []any, ownID string) (int, any) {
	seen := map[string]bool{}
	for _, raw := range fields {
		field, _ := raw.(map[string]any)
		base, _ := field["targetBaseMetricName"].(string)
		if !e2mNamePattern.MatchString(base) {
			return e2mBadRequest(e2mCustomIssue("Invalid", "metricFields", "targetBaseMetricName"))
		}
		if seen[base] || f.baseNameTaken(base, ownID) {
			return e2mBadRequest(e2mCustomIssue("Metric fields are already in use: "+base, "e2m", "metricFields"))
		}
		seen[base] = true
		for _, rawAgg := range asList(field["aggregations"]) {
			agg, _ := rawAgg.(map[string]any)
			if status, resp := checkAggregation(agg); status != 0 {
				return status, resp
			}
		}
	}
	return 0, nil
}

func (f *e2mFake) baseNameTaken(base, ownID string) bool {
	for id, other := range f.e2ms {
		if id == ownID {
			continue
		}
		for _, raw := range asList(other["metricFields"]) {
			if raw.(map[string]any)["targetBaseMetricName"] == base {
				return true
			}
		}
	}
	return false
}

func checkAggregation(agg map[string]any) (int, any) {
	if _, ok := agg["targetMetricName"].(string); !ok {
		return e2mBadRequest(`{"code":"invalid_type","expected":"string","received":"undefined","path":["targetMetricName"],"message":"Required"}`)
	}
	if samples, ok := agg["samples"].(map[string]any); ok {
		if st, _ := samples["sampleType"].(string); st != "SAMPLE_TYPE_MIN" && st != "SAMPLE_TYPE_MAX" {
			return e2mFail(http.StatusInternalServerError, "Unexpected server error occurred")
		}
	}
	if histogram, ok := agg["histogram"].(map[string]any); ok && agg["enabled"] != false && len(asList(histogram["buckets"])) == 0 {
		return e2mBadRequest(e2mCustomIssue("Histogram aggregation must have buckets", "aggregations", "histogram"))
	}
	return 0, nil
}

func checkLabels(labels []any) (int, any) {
	if len(labels) > e2mLabelLimit {
		return e2mBadRequest(e2mCustomIssue(fmt.Sprintf("Labels limit is %d", e2mLabelLimit), "e2m", "metricLabels"))
	}
	seen := map[string]bool{}
	for _, raw := range labels {
		label, _ := raw.(map[string]any)
		target, _ := label["targetLabel"].(string)
		if !e2mNamePattern.MatchString(target) {
			return e2mBadRequest(e2mCustomIssue("Invalid", "metricLabels", "targetLabel"))
		}
		if _, ok := label["sourceField"].(string); !ok {
			return e2mBadRequest(`{"code":"invalid_type","expected":"string","received":"undefined","path":["sourceField"],"message":"Required"}`)
		}
		if seen[target] {
			return e2mBadRequest(e2mCustomIssue("Metric labels are not unique: "+target, "e2m", "metricLabels"))
		}
		seen[target] = true
	}
	return 0, nil
}

func (f *e2mFake) normalize(in, prior map[string]any, replace bool) (map[string]any, int, any) {
	out := map[string]any{
		"name":         in["name"],
		"type":         in["type"],
		"metricLabels": orDefault(in["metricLabels"], []any{}),
		"isInternal":   false,
	}
	switch {
	case in["description"] != nil:
		out["description"] = in["description"]
	case prior != nil && prior["description"] != nil:
		out["description"] = prior["description"]
	}
	if in["dataSource"] != nil {
		out["dataSource"] = in["dataSource"]
	}
	if replace && in["isInternal"] == true {
		out["isInternal"] = true
	}

	limit := float64(e2mDefaultPermutationsLimit)
	if replace {
		if perms, ok := in["permutations"].(map[string]any); ok {
			limit, _ = perms["limit"].(float64)
		}
	} else if v, ok := in["permutationsLimit"].(float64); ok {
		limit = v
	}
	if limit < 1 && len(asList(out["metricLabels"])) > 0 {
		status, body := e2mBadRequest(e2mCustomIssue(`Permutation amount needs to be bigger than 1 when "metricLabels" are set`, "e2m", "permutations", "limit"))
		return nil, status, body
	}
	out["permutations"] = map[string]any{"limit": limit}

	if out["type"] == "E2M_TYPE_LOGS2METRICS" {
		q, _ := in["logsQuery"].(map[string]any)
		out["logsQuery"] = map[string]any{
			"lucene":                 orDefault(q["lucene"], ""),
			"applicationnameFilters": orDefault(q["applicationnameFilters"], []any{}),
			"subsystemnameFilters":   orDefault(q["subsystemnameFilters"], []any{}),
			"severityFilters":        orDefault(q["severityFilters"], []any{}),
		}
	} else {
		q, _ := in["spansQuery"].(map[string]any)
		out["spansQuery"] = map[string]any{
			"lucene":                 orDefault(q["lucene"], ""),
			"applicationnameFilters": orDefault(q["applicationnameFilters"], []any{}),
			"subsystemnameFilters":   orDefault(q["subsystemnameFilters"], []any{}),
			"actionFilters":          orDefault(q["actionFilters"], []any{}),
			"serviceFilters":         orDefault(q["serviceFilters"], []any{}),
		}
	}

	var fields []any
	for _, raw := range asList(in["metricFields"]) {
		field := raw.(map[string]any)
		template := e2mDefaultAggregations
		if prior != nil {
			template = storedAggregations(prior, field["targetBaseMetricName"])
		}
		fields = append(fields, map[string]any{
			"targetBaseMetricName": field["targetBaseMetricName"],
			"sourceField":          field["sourceField"],
			"aggregations":         backfill(asList(field["aggregations"]), template),
		})
	}
	if fields == nil {
		fields = []any{}
	}
	out["metricFields"] = fields
	return out, 0, nil
}

func storedAggregations(prior map[string]any, base any) []map[string]any {
	for _, raw := range asList(prior["metricFields"]) {
		field := raw.(map[string]any)
		if field["targetBaseMetricName"] == base {
			var out []map[string]any
			for _, agg := range asList(field["aggregations"]) {
				out = append(out, agg.(map[string]any))
			}
			return out
		}
	}
	return e2mDefaultAggregations
}

// backfill gives a field every aggregation of the template, in template order.
func backfill(sent []any, template []map[string]any) []any {
	byType := map[any]map[string]any{}
	for _, raw := range sent {
		agg := raw.(map[string]any)
		byType[agg["aggType"]] = agg
	}
	out := make([]any, 0, len(template))
	for _, t := range template {
		agg := map[string]any{"aggType": t["aggType"], "targetMetricName": t["targetMetricName"], "enabled": t["enabled"]}
		meta := pickMeta(t)
		if in, ok := byType[t["aggType"]]; ok {
			agg["enabled"] = orDefault(in["enabled"], false)
			if m := pickMeta(in); m != nil {
				meta = m
			}
		}
		for k, v := range meta {
			agg[k] = v
		}
		out = append(out, agg)
	}
	return out
}

func pickMeta(agg map[string]any) map[string]any {
	for _, k := range []string{"samples", "histogram"} {
		if v, ok := agg[k]; ok {
			return map[string]any{k: v}
		}
	}
	return nil
}

// present returns the answer for a stored events2metric: fields and aggregations sorted by name.
func (f *e2mFake) present(stored map[string]any, exceeded bool) map[string]any {
	out := mapsClone(stored)
	out["permutations"] = map[string]any{
		"limit":            stored["permutations"].(map[string]any)["limit"],
		"hasExceededLimit": exceeded,
	}
	fields := slices.Clone(asList(stored["metricFields"]))
	slices.SortFunc(fields, byKey("targetBaseMetricName"))
	for i, raw := range fields {
		field := mapsClone(raw.(map[string]any))
		aggs := slices.Clone(asList(field["aggregations"]))
		slices.SortFunc(aggs, byKey("targetMetricName"))
		for j, agg := range aggs {
			a := mapsClone(agg.(map[string]any))
			if pickMeta(a) == nil {
				a["none"] = map[string]any{}
			}
			aggs[j] = a
		}
		field["aggregations"] = aggs
		fields[i] = field
	}
	out["metricFields"] = fields
	return out
}

func byKey(key string) func(a, b any) int {
	return func(a, b any) int {
		return strings.Compare(fmt.Sprint(a.(map[string]any)[key]), fmt.Sprint(b.(map[string]any)[key]))
	}
}

// e2mSortRequest sorts the metric fields and labels of a recorded request. The provider sends
// them from Terraform maps, so their order changes from run to run and means nothing.
func e2mSortRequest(v any) any {
	body, ok := v.(map[string]any)
	if !ok {
		return v
	}
	if fields, ok := body["metricFields"].([]any); ok {
		slices.SortFunc(fields, byKey("targetBaseMetricName"))
	}
	if labels, ok := body["metricLabels"].([]any); ok {
		slices.SortFunc(labels, byKey("targetLabel"))
	}
	return body
}
