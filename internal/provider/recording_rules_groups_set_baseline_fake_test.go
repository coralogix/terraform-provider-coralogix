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
	"bytes"
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

const (
	rrBasePath  = "/recording-rules/recording-rules/v1"
	rrGoldenDir = "testdata/recording_rules_groups_set_baseline"

	rrNotFound   = "Not Found: No rule group set with the given id"
	rrNoGroups   = "No rule groups in the set"
	rrBadPromQL  = `SingleExpr: unexpected token ; want ""(", "{", "-", "+""`
	rrMaxDelayMs = 1800000
)

var rrUnsigned = regexp.MustCompile(`^[0-9]+$`)

// rrFake follows metrics-rule-manager as recorded in fidelity/recorded.json, not the Go client:
//   - Create answers {id}, Update and Delete answer {}. A missing or malformed id is 404 on Get,
//     Update, and Delete.
//   - An omitted set name is the id. A name is 1 to 255 characters and unique among the sets of
//     the company; a group name is unique only within its set.
//   - An omitted interval is stored as 60 and an omitted limit as "0", also on an Update, which
//     replaces the groups. An omitted name on an Update keeps the stored name.
//   - Labels read back as {} when omitted. An omitted evaluationDelayMs stays omitted, and 0 is
//     stored as 0.
//   - Get returns the groups sorted by name, and the rules in the order of the request. An Update
//     keeps the id of a group with the same name and adds one to its version.
//   - A limit that is not an unsigned number and a negative delay fail to decode (proto). The
//     other checks answer a JSON document of the failed fields.
//
// The fake checks PromQL only for an empty expression and unbalanced parentheses.
type rrFake struct {
	mu       sync.Mutex
	sets     map[string]map[string]any
	seq      int
	groupSeq int
	requests []grRequest
	// fail answers every request of a method with this status and message, until heal.
	fail map[string]rrFailure
}

type rrFailure struct {
	status  int
	message string
}

func newRRFakeState() *rrFake { return &rrFake{sets: map[string]map[string]any{}} }

func newRRFake(t *testing.T) *rrFake {
	t.Helper()
	f := newRRFakeState()
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

// seed stores a set as if it was made outside Terraform. It returns the id.
func (f *rrFake) seed(set map[string]any) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	status, body := f.create(roundTripAny(set))
	if status != http.StatusOK {
		panic(fmt.Sprintf("seed failed: %d %v", status, body))
	}
	return rrIDOf(body)
}

// forget deletes a set without a request, as if it was deleted outside Terraform.
func (f *rrFake) forget(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.sets, id)
}

// failMethod answers every later request of the method with the status and the message.
func (f *rrFake) failMethod(method string, status int, message string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail == nil {
		f.fail = map[string]rrFailure{}
	}
	f.fail[method] = rrFailure{status: status, message: message}
}

func (f *rrFake) heal() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fail = nil
}

func (f *rrFake) takeRequests() []grRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.requests
	f.requests = nil
	return out
}

func (f *rrFake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(r.Body)
	rec := grRequest{Method: r.Method, Path: r.URL.Path}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &rec.Body)
	}
	f.mu.Lock()
	status, body := f.handle(r.Method, r.URL.Path, rec.Body)
	rec.Status = status
	f.requests = append(f.requests, rec)
	f.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func rrError(status int, message string) (int, any) {
	return status, map[string]any{"code": status, "message": message}
}

func (f *rrFake) handle(method, path string, body any) (int, any) {
	_, rest, ok := strings.Cut(path, rrBasePath)
	if !ok {
		return rrError(http.StatusNotFound, "Not Found")
	}
	if failure, ok := f.fail[method]; ok {
		return rrError(failure.status, failure.message)
	}
	id := strings.TrimPrefix(rest, "/")
	in, _ := body.(map[string]any)
	switch {
	case method == http.MethodPost && rest == "":
		return f.create(in)
	case method == http.MethodGet && rest == "":
		ids := make([]string, 0, len(f.sets))
		for k := range f.sets {
			ids = append(ids, k)
		}
		sort.Strings(ids)
		list := []any{}
		for _, k := range ids {
			list = append(list, f.sets[k])
		}
		return http.StatusOK, map[string]any{"sets": list}
	case method == http.MethodGet && id != "":
		s, ok := f.sets[id]
		if !ok {
			return rrError(http.StatusNotFound, rrNotFound)
		}
		return http.StatusOK, s
	case method == http.MethodPut && id != "":
		return f.update(id, in)
	case method == http.MethodDelete && id != "":
		if _, ok := f.sets[id]; !ok {
			return rrError(http.StatusNotFound, rrNotFound)
		}
		delete(f.sets, id)
		return http.StatusOK, map[string]any{}
	}
	return rrError(http.StatusNotFound, "Not Found")
}

func (f *rrFake) create(in map[string]any) (int, any) {
	if in == nil {
		in = map[string]any{}
	}
	if status, body, ok := f.check(in, ""); !ok {
		return status, body
	}
	f.seq++
	id := fmt.Sprintf("01J%023d", f.seq)
	name := id
	if n, ok := in["name"].(string); ok {
		name = n
	}
	f.sets[id] = map[string]any{"id": id, "name": name, "groups": f.storeGroups(in["groups"], nil)}
	return http.StatusOK, map[string]any{"id": id}
}

func (f *rrFake) update(id string, in map[string]any) (int, any) {
	stored, ok := f.sets[id]
	if !ok {
		return rrError(http.StatusNotFound, rrNotFound)
	}
	if in == nil {
		in = map[string]any{}
	}
	if status, body, ok := f.check(in, id); !ok {
		return status, body
	}
	if n, ok := in["name"].(string); ok {
		stored["name"] = n
	}
	prior := map[string]map[string]any{}
	for i, g := range stored["groups"].([]any) {
		group := g.(map[string]any)
		prior[group["name"].(string)] = map[string]any{"id": group["id"], "version": group["version"], "index": i}
	}
	stored["groups"] = f.storeGroups(in["groups"], prior)
	return http.StatusOK, map[string]any{}
}

// storeGroups returns the groups as Get returns them. A group with the name of a prior group
// keeps its id, and its version goes up by one.
func (f *rrFake) storeGroups(raw any, prior map[string]map[string]any) []any {
	groups, _ := raw.([]any)
	out := make([]any, 0, len(groups))
	for _, g := range groups {
		in, _ := g.(map[string]any)
		name, _ := in["name"].(string)
		group := map[string]any{"name": name, "interval": 60, "limit": "0", "version": 0}
		if p, ok := prior[name]; ok {
			group["id"] = p["id"]
			group["version"] = p["version"].(int) + 1
		} else {
			f.groupSeq++
			group["id"] = fmt.Sprintf("01G%023d", f.groupSeq)
		}
		if v, ok := in["interval"].(float64); ok {
			group["interval"] = int(v)
		}
		if v, ok := in["limit"].(string); ok {
			n, _ := strconv.ParseUint(v, 10, 64)
			group["limit"] = strconv.FormatUint(n, 10)
		}
		rules := []any{}
		inRules, _ := in["rules"].([]any)
		for _, r := range inRules {
			rule, _ := r.(map[string]any)
			labels, _ := rule["labels"].(map[string]any)
			if labels == nil {
				labels = map[string]any{}
			}
			stored := map[string]any{"record": rule["record"], "expr": rule["expr"], "labels": labels}
			if v, ok := rule["evaluationDelayMs"].(float64); ok {
				stored["evaluationDelayMs"] = int64(v)
			}
			rules = append(rules, stored)
		}
		group["rules"] = rules
		out = append(out, group)
	}
	// The service reads the groups by creation time, in seconds, so the groups of one request tie
	// and come back in the order of the name index. A kept group keeps its place before them.
	rank := map[string]int{}
	for name, p := range prior {
		rank[name] = p["index"].(int)
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i].(map[string]any)["name"].(string), out[j].(map[string]any)["name"].(string)
		ra, okA := rank[a]
		rb, okB := rank[b]
		switch {
		case okA && okB:
			return ra < rb
		case okA != okB:
			return okA
		}
		return a < b
	})
	return out
}

// check runs the checks in the order of the service: decoding, the field checks, the groups, and
// the unique set name. except is the id of the set that an Update changes.
func (f *rrFake) check(in map[string]any, except string) (int, any, bool) {
	groups, _ := in["groups"].([]any)
	for _, g := range groups {
		group, _ := g.(map[string]any)
		if v, ok := group["limit"]; ok {
			if s, isString := v.(string); !isString || !rrUnsigned.MatchString(s) {
				status, body := rrError(http.StatusBadRequest, fmt.Sprintf("Bad Request: proto: (line 1:1): invalid value for uint64 field limit: %s", compact(v)))
				return status, body, false
			}
		}
		rules, _ := group["rules"].([]any)
		for _, r := range rules {
			rule, _ := r.(map[string]any)
			if v, ok := rule["evaluationDelayMs"].(float64); ok && v < 0 {
				status, body := rrError(http.StatusBadRequest, fmt.Sprintf("Bad Request: proto: (line 1:1): invalid value for uint32 field evaluationDelayMs: %v", v))
				return status, body, false
			}
		}
	}

	fields := map[string]any{}
	if name, ok := in["name"].(string); ok && (name == "" || len([]rune(name)) > 255) {
		fields["name"] = []any{rrLength(name, 255)}
	}
	groupFields := map[string]any{}
	for i, g := range groups {
		group, _ := g.(map[string]any)
		if e := rrGroupErrors(group); len(e) > 0 {
			groupFields[strconv.Itoa(i)] = e
		}
	}
	if len(groupFields) > 0 {
		fields["groups"] = groupFields
	} else if dup := rrDuplicateGroupName(groups); dup != nil {
		fields["groups"] = []any{dup}
	}
	if len(fields) > 0 {
		status, body := rrError(http.StatusBadRequest, "Bad Request: "+rrIndented(fields))
		return status, body, false
	}

	if len(groups) == 0 {
		status, body := rrError(http.StatusBadRequest, "Bad Request: "+rrNoGroups)
		return status, body, false
	}
	if name, ok := in["name"].(string); ok {
		for id, s := range f.sets {
			if id != except && s["name"] == name {
				status, body := rrError(http.StatusBadRequest, "Bad Request: duplicate_rule_group_set_name")
				return status, body, false
			}
		}
	}
	return 0, nil, true
}

func rrGroupErrors(group map[string]any) map[string]any {
	out := map[string]any{}
	name, _ := group["name"].(string)
	if name == "" || len([]rune(name)) > 255 {
		out["name"] = []any{rrLength(name, 255)}
	}
	if v, ok := group["interval"].(float64); ok && v < 60 {
		out["interval"] = []any{map[string]any{"code": "range", "message": nil, "params": map[string]any{"min": 60, "value": v}}}
	}
	ruleFields := map[string]any{}
	rules, _ := group["rules"].([]any)
	for i, r := range rules {
		rule, _ := r.(map[string]any)
		e := map[string]any{}
		record, _ := rule["record"].(string)
		if record == "" {
			e["record"] = []any{
				map[string]any{"code": "length", "message": nil, "params": map[string]any{"min": 1, "value": ""}},
				rrInvalidPromQL(""),
			}
		}
		expr, _ := rule["expr"].(string)
		if expr == "" || strings.Count(expr, "(") != strings.Count(expr, ")") {
			e["expr"] = []any{rrInvalidPromQL(expr)}
		}
		if v, ok := rule["evaluationDelayMs"].(float64); ok && v > rrMaxDelayMs {
			e["evaluation_delay_ms"] = []any{map[string]any{
				"code":    "evaluation_delay_out_of_range",
				"message": "evaluation_delay_ms must be between 0 and 1800000 (30 minutes)",
				"params":  map[string]any{"value": v},
			}}
		}
		if len(e) > 0 {
			ruleFields[strconv.Itoa(i)] = e
		}
	}
	if len(ruleFields) > 0 {
		out["rules"] = ruleFields
	}
	return out
}

func rrLength(value string, max int) map[string]any {
	return map[string]any{"code": "length", "message": nil, "params": map[string]any{"min": 1, "max": max, "value": value}}
}

func rrInvalidPromQL(value string) map[string]any {
	return map[string]any{"code": "invalid_promql", "message": rrBadPromQL, "params": map[string]any{"value": value}}
}

// rrDuplicateGroupName returns the error of the first name that two groups share. The service
// shows every group of the request, with its own field names.
func rrDuplicateGroupName(groups []any) map[string]any {
	seen := map[string]bool{}
	dup := false
	for _, g := range groups {
		name, _ := g.(map[string]any)["name"].(string)
		dup = dup || seen[name]
		seen[name] = true
	}
	if !dup {
		return nil
	}
	value := []any{}
	for _, g := range groups {
		group, _ := g.(map[string]any)
		rules := []any{}
		inRules, _ := group["rules"].([]any)
		for _, r := range inRules {
			rule, _ := r.(map[string]any)
			labels, _ := rule["labels"].(map[string]any)
			if labels == nil {
				labels = map[string]any{}
			}
			rules = append(rules, map[string]any{
				"evaluation_delay_ms": rule["evaluationDelayMs"], "expr": rule["expr"], "labels": labels, "record": rule["record"],
			})
		}
		var limit any
		if s, ok := group["limit"].(string); ok {
			limit, _ = strconv.ParseUint(s, 10, 64)
		}
		value = append(value, map[string]any{
			"id": nil, "interval": group["interval"], "limit": limit, "name": group["name"], "rules": rules, "version": nil,
		})
	}
	return map[string]any{"code": "duplicate_rule_group_name", "message": nil, "params": map[string]any{"value": value}}
}

// rrIndented writes a document as the service does, two spaces per level, with sorted keys.
func rrIndented(v any) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
	return strings.TrimSuffix(buf.String(), "\n")
}

// rrSortedMessage writes the JSON document of a validation message again with sorted keys.
func rrSortedMessage(msg string) string {
	prefix, doc, ok := strings.Cut(msg, ": {")
	if !ok {
		return msg
	}
	var v any
	if err := json.Unmarshal([]byte("{"+doc), &v); err != nil {
		return msg
	}
	return prefix + ": " + rrIndented(v)
}

func rrIDOf(resp any) string {
	m, _ := resp.(map[string]any)
	id, _ := m["id"].(string)
	return id
}

func roundTripAny(v map[string]any) map[string]any {
	raw, _ := json.Marshal(v)
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	return out
}
