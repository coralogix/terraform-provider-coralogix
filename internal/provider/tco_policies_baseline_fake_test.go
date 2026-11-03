// Copyright 2024 Coralogix Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an AS IS BASIS,
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

const tcoBasePath = "/dataplans"

// tcoCompanyID is the company id that every policy of the fake has.
const tcoCompanyID = 4711

// tcoTime is the createdAt and updatedAt of every policy. The API writes them in this form.
const tcoTime = "Mon Nov 02 2026 10:00:00 GMT+0000 (Coordinated Universal Time)"

// tcoSources are the source types of the fake: the overwrite path, the Get filter, the rules field,
// and the RPC name in validation messages.
var tcoSources = map[string]struct{ path, filter, rules, rpc string }{
	"logs":  {"/log-policies/v1", "SOURCE_TYPE_LOGS", "logRules", "AtomicOverwriteLogPolicies"},
	"spans": {"/span-policies/v1", "SOURCE_TYPE_SPANS", "spanRules", "AtomicOverwriteSpanPolicies"},
	"rum":   {"/rum-policies/v1", "SOURCE_TYPE_RUM", "rumRules", "AtomicOverwriteRumPolicies"},
}

// tcoPriorityRank orders the priorities from most to least restrictive.
var tcoPriorityRank = map[string]int{"PRIORITY_TYPE_BLOCK": 0, "PRIORITY_TYPE_LOW": 1, "PRIORITY_TYPE_MEDIUM": 2, "PRIORITY_TYPE_HIGH": 3}

// tcoFake is a small in-memory backend of the TCO policies API, as ws-quota-grpc serves it through
// the REST gateway. It follows the answers recorded on EU2 (tco_policies_fidelity_test.go) and the
// backend code:
//   - PUT /dataplans/{log,span,rum}-policies/v1 replaces every policy of the source type with the
//     request list, in order. Each policy gets a new id, order index+1, and enabled = !disabled.
//     The answer is {"createResponses": [{"policy": ...}]}. An empty list deletes every policy.
//   - GET /dataplans/policies/v1?source_type=... returns {"policies": [...]}, [] when there are none.
//     It never answers 404. It adds priorityOverrideStatus to an enabled policy with an override.
//   - A policy without targets reads back with one: logs, or spans for span policies, in dataspace
//     default, with the priority and override of the policy. A span policy always has the spans
//     target. A RUM policy has no targets. A target dataspace reads back as "default".
//   - Rule names are lowercased, except tag values. An absent description reads as "".
//   - Validation runs before anything is stored, and the first error answers: severities or a DPXL
//     expression, never both, for logs and RUM; non-empty and unique targets; priority at one
//     level only, and present; tiers between 0 and 100 and above the base priority; for spans, no
//     rules next to a DPXL expression. Then: duplicate names (409), HIGH for RUM, an unknown
//     archive retention (500), an unknown log dataset (412), and a DPXL expression without a
//     version tag.
//
// The fake does not check limits (policy count, targets, tiers, names in a rule), tier order, the
// target restrictions of each source type, or feature flags.
type tcoFake struct {
	mu       sync.Mutex
	policies map[string][]map[string]any // by source type: logs, spans, rum
	seq      int
	requests []grRequest
	// datasets are the log datasets that exist besides logs.
	datasets map[string]bool
	// archiveRetentions are the archive retention ids that exist.
	archiveRetentions map[string]bool
	// reverseTagRules makes Get return the tag rules of a span policy in reverse order. The API
	// keeps no order there: two recordings of one request answered in both orders.
	reverseTagRules bool
	// getFail and putFail, when set, answer every GET or PUT instead of the stored policies.
	getFail func() (int, any)
	putFail func() (int, any)
}

func newTCOFake(t *testing.T) *tcoFake {
	t.Helper()
	f := &tcoFake{}
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

// seed stores policies of a source type as if another client had sent this overwrite body.
func (f *tcoFake) seed(source string, body map[string]any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	status, resp := f.overwrite(source, roundTripAny(body))
	if status != http.StatusOK {
		panic(fmt.Sprintf("seed %s: %d %v", source, status, resp))
	}
}

// failGets makes every GET answer with the given error.
func (f *tcoFake) failGets(status int, message string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.getFail = func() (int, any) { return tcoFail(status, message) }
}

// failPuts makes every PUT answer with the given error.
func (f *tcoFake) failPuts(status int, message string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.putFail = func() (int, any) { return tcoFail(status, message) }
}

func (f *tcoFake) takeRequests() []grRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.requests
	f.requests = nil
	return out
}

func (f *tcoFake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	raw, _ := io.ReadAll(r.Body)
	rec := grRequest{Method: r.Method, Path: r.URL.RequestURI()}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &rec.Body)
	}
	sortTagRules(rec.Body)
	status, body := f.handle(r.Method, rec.Path, rec.Body)
	rec.Status = status
	f.requests = append(f.requests, rec)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func (f *tcoFake) handle(method, rawPath string, body any) (int, any) {
	u, err := url.Parse(rawPath)
	if err != nil {
		return grFail(http.StatusBadRequest, err.Error())
	}
	// The SDK puts the gateway prefix (/mgmt/openapi/...) before the path.
	if i := strings.Index(u.Path, tcoBasePath+"/"); i > 0 {
		u.Path = u.Path[i:]
	}
	if method == http.MethodGet && u.Path == tcoBasePath+"/policies/v1" {
		if f.getFail != nil {
			return f.getFail()
		}
		return http.StatusOK, map[string]any{"policies": f.list(u.Query().Get("source_type"))}
	}
	for source, s := range tcoSources {
		if method == http.MethodPut && u.Path == tcoBasePath+s.path {
			if f.putFail != nil {
				return f.putFail()
			}
			return f.overwrite(source, body)
		}
	}
	return grFail(http.StatusNotFound, "unknown path "+method+" "+rawPath)
}

func (f *tcoFake) list(filter string) []any {
	out := []any{}
	for _, source := range []string{"logs", "spans", "rum"} {
		if filter != "" && filter != tcoSources[source].filter {
			continue
		}
		for _, p := range f.policies[source] {
			p = deepCloneMap(p)
			if p["enabled"] == true {
				addOverrideStatus(p)
				if targets, ok := p["targets"].([]any); ok {
					for _, t := range targets {
						addOverrideStatus(t.(map[string]any))
					}
				}
			}
			if f.reverseTagRules {
				if rules, ok := p["spanRules"].(map[string]any); ok {
					tags, _ := rules["tagRules"].([]any)
					slices.Reverse(tags)
				}
			}
			out = append(out, p)
		}
	}
	return out
}

// addOverrideStatus adds the status of a quota-based override, at no usage: the first tier above 0
// percent sets the current priority, else the base priority.
func addOverrideStatus(p map[string]any) {
	override, _ := p["priorityOverride"].(map[string]any)
	quota, _ := override["quotaBased"].(map[string]any)
	if quota == nil {
		return
	}
	current := p["priority"]
	tiers, _ := quota["usageTiers"].([]any)
	for _, t := range tiers {
		tier := t.(map[string]any)
		if pct, _ := tier["dailyQuotaPercentage"].(float64); pct > 0 {
			current = tier["priority"]
			break
		}
	}
	p["priorityOverrideStatus"] = map[string]any{"quotaBased": map[string]any{
		"currentPriority": current, "currentUsage": 0.0, "dailyLimit": 1000.0,
	}}
}

func (f *tcoFake) overwrite(source string, body any) (int, any) {
	s := tcoSources[source]
	in, _ := body.(map[string]any)
	entries, _ := in["policies"].([]any)

	for i, e := range entries {
		entry, _ := e.(map[string]any)
		policy, _ := entry["policy"].(map[string]any)
		rules, _ := entry[s.rules].(map[string]any)
		if msg := tcoValidate(source, policy, rules); msg != "" {
			return tcoFail(http.StatusBadRequest, "validation failed for method /com.coralogix.quota.v1.PoliciesService/"+s.rpc+": ValidationError: "+msg)
		}
		if msg := tcoValidateCustom(source, policy, rules); msg != "" {
			return tcoFail(http.StatusBadRequest, fmt.Sprintf("validation failed for method /com.coralogix.quota.v1.PoliciesService/%s: ValidationError: \"policies[%d]\" failed custom validation because %s", s.rpc, i, msg))
		}
	}

	if status, resp := f.checkStored(source, entries); status != 0 {
		return status, resp
	}

	stored := make([]map[string]any, 0, len(entries))
	created := make([]any, 0, len(entries))
	for i, e := range entries {
		entry := e.(map[string]any)
		policy, _ := entry["policy"].(map[string]any)
		rules, _ := entry[s.rules].(map[string]any)
		p := f.store(source, i, policy, rules)
		stored = append(stored, p)
		created = append(created, map[string]any{"policy": deepCloneMap(p)})
	}
	if f.policies == nil {
		f.policies = map[string][]map[string]any{}
	}
	f.policies[source] = stored
	return http.StatusOK, map[string]any{"createResponses": created}
}

// checkStored is the validation that the API runs after the request validation, against what the
// company has. It returns status 0 when the request passes.
func (f *tcoFake) checkStored(source string, entries []any) (int, any) {
	names := map[string]bool{}
	for _, e := range entries {
		policy, _ := e.(map[string]any)["policy"].(map[string]any)
		name, _ := policy["name"].(string)
		if names[name] {
			return tcoFail(http.StatusConflict, "Duplicate names found")
		}
		names[name] = true
	}
	for _, e := range entries {
		entry := e.(map[string]any)
		policy, _ := entry["policy"].(map[string]any)
		rules, _ := entry[tcoSources[source].rules].(map[string]any)
		if source == "rum" && policy["priority"] == "PRIORITY_TYPE_HIGH" {
			return tcoFail(http.StatusBadRequest, "RUM policies do not support HIGH priority; use MEDIUM or lower.")
		}
		if retention, ok := policy["archiveRetention"].(map[string]any); ok {
			if id, _ := retention["id"].(string); !f.archiveRetentions[id] {
				return tcoFail(http.StatusInternalServerError, "Unexpected server error occurred")
			}
		}
		if source == "logs" {
			for _, t := range tcoList(policy["targets"]) {
				if dataset, _ := t["dataset"].(string); dataset != "logs" && !f.datasets[dataset] {
					return tcoFail(http.StatusPreconditionFailed, fmt.Sprintf("Dataset '%s' not found", dataset))
				}
			}
		}
		if dpxl, _ := rules["dpxlExpression"].(string); dpxl != "" && !strings.HasPrefix(dpxl, "<v1>") {
			// The compiler reports the expression without its last character.
			n := len(dpxl) - 1
			return tcoFail(http.StatusBadRequest, fmt.Sprintf("DPXL compile failed for policy %s, id:%s, error: DPXL compilation errors:\n - expected version tag. e.g. \"<v1> foo == 'bar'\" at [0:0-0:%d]: %s",
				policy["name"], f.nextID(), n, dpxl[:n]))
		}
	}
	return 0, nil
}

// tcoValidate is the validation of the request shape.
func tcoValidate(source string, policy, rules map[string]any) string {
	if source != "spans" {
		dpxl, _ := rules["dpxlExpression"].(string)
		severities, _ := rules["severities"].([]any)
		switch {
		case dpxl != "" && len(severities) > 0:
			return "Cannot provide both 'dpxlExpression' and 'severities' in LogRules"
		case dpxl == "" && len(severities) == 0:
			return "must contain at least 1 item when not using dpxlExpression"
		}
	}
	seen := map[string]bool{}
	for _, t := range tcoList(policy["targets"]) {
		dataset, _ := t["dataset"].(string)
		if dataset == "" {
			return "Dataset name must not be empty"
		}
		key := strings.ToLower(tcoDataspace(t) + "/" + dataset)
		if seen[key] {
			return "Targets must be unique by (dataspace, dataset)"
		}
		seen[key] = true
	}
	return ""
}

// tcoValidateCustom is the validation that the API reports per policy.
func tcoValidateCustom(source string, policy, rules map[string]any) string {
	policyLevel := tcoHasPriority(policy) || policy["priorityOverride"] != nil
	targets := tcoList(policy["targets"])
	targetLevel, everyTarget := false, len(targets) > 0
	for _, t := range targets {
		if tcoHasPriority(t) || t["priorityOverride"] != nil {
			targetLevel = true
		}
		if !tcoHasPriority(t) {
			everyTarget = false
		}
	}
	switch {
	case policyLevel && targetLevel:
		return "FAILED_PRECONDITION: priority/priorityOverride must be set either at the policy level or per-target, not both."
	case !policyLevel && !everyTarget:
		return "FAILED_PRECONDITION: A priority is required: set the policy priority (or a quota-based priority override), or provide targets that each specify a priority."
	}
	for _, holder := range append([]map[string]any{policy}, targets...) {
		if msg := tcoValidateTiers(holder); msg != "" {
			return msg
		}
	}
	if source == "spans" {
		return tcoValidateSpanDPXL(policy, rules)
	}
	return ""
}

// tcoValidateSpanDPXL refuses a span policy with a DPXL expression and any rule.
func tcoValidateSpanDPXL(policy, rules map[string]any) string {
	dpxl, _ := rules["dpxlExpression"].(string)
	tags, _ := rules["tagRules"].([]any)
	if dpxl != "" && (policy["applicationRule"] != nil || policy["subsystemRule"] != nil || rules["serviceRule"] != nil || rules["actionRule"] != nil || len(tags) > 0) {
		return "INVALID_ARGUMENT: Cannot have both rules (applicationRule, subsystemRule, serviceRule, actionRule, tagRules) and dpxlExpression"
	}
	return ""
}

func tcoValidateTiers(holder map[string]any) string {
	override, _ := holder["priorityOverride"].(map[string]any)
	quota, _ := override["quotaBased"].(map[string]any)
	base, _ := holder["priority"].(string)
	for _, t := range tcoList(quota["usageTiers"]) {
		if pct, _ := t["dailyQuotaPercentage"].(float64); pct < 0 || pct > 100 {
			return "FAILED_PRECONDITION: each usageTier.dailyQuotaPercentage must be between 0 and 100"
		}
		if p, _ := t["priority"].(string); base != "" && tcoPriorityRank[p] <= tcoPriorityRank[base] {
			return "FAILED_PRECONDITION: usageTiers priority must be higher than the base priority (a tier at or below the base has no effect)"
		}
	}
	return ""
}

// store builds the policy that the API stores and returns for one request entry.
func (f *tcoFake) store(source string, index int, policy, rules map[string]any) map[string]any {
	p := map[string]any{
		"id": f.nextID(), "companyId": tcoCompanyID, "name": policy["name"], "description": "",
		"enabled": policy["disabled"] != true, "order": index + 1, "deleted": false,
		"createdAt": tcoTime, "updatedAt": tcoTime,
	}
	if d, ok := policy["description"].(string); ok {
		p["description"] = d
	}
	priority, _ := policy["priority"].(string)
	if priority == "" {
		priority = "PRIORITY_TYPE_UNSPECIFIED"
	}
	if priority == "PRIORITY_TYPE_UNSPECIFIED" && policy["priorityOverride"] != nil {
		// An override without a priority falls back to BLOCK.
		priority = "PRIORITY_TYPE_BLOCK"
	}
	p["priority"] = priority
	for _, k := range []string{"applicationRule", "subsystemRule"} {
		if r, ok := policy[k].(map[string]any); ok {
			p[k] = tcoLowerRule(r)
		}
	}
	for _, k := range []string{"archiveRetention", "priorityOverride"} {
		if v, ok := policy[k]; ok && v != nil {
			p[k] = deepCloneAny(v)
		}
	}

	p[tcoSources[source].rules] = tcoStoreRules(source, rules)
	p["targets"] = tcoStoreTargets(source, policy, p)
	return p
}

// tcoStoreRules builds the rules of a stored policy.
func tcoStoreRules(source string, rules map[string]any) map[string]any {
	if source != "spans" {
		out := map[string]any{"severities": []any{}}
		if sev, ok := rules["severities"].([]any); ok {
			out["severities"] = deepCloneAny(sev)
		}
		if dpxl, ok := rules["dpxlExpression"].(string); ok {
			out["dpxlExpression"] = dpxl
		}
		return out
	}
	out := map[string]any{"tagRules": []any{}}
	if tags, ok := rules["tagRules"].([]any); ok {
		out["tagRules"] = deepCloneAny(tags)
	}
	for _, k := range []string{"serviceRule", "actionRule"} {
		if r, ok := rules[k].(map[string]any); ok {
			out[k] = tcoLowerRule(r)
		}
	}
	if dpxl, ok := rules["dpxlExpression"].(string); ok {
		out["dpxlExpression"] = dpxl
	}
	return out
}

// tcoStoreTargets builds the targets of a stored policy p.
func tcoStoreTargets(source string, policy, p map[string]any) []any {
	targets := []any{}
	switch source {
	case "logs":
		for _, t := range tcoList(policy["targets"]) {
			out := map[string]any{"dataset": t["dataset"], "dataspace": tcoDataspace(t)}
			if tcoHasPriority(t) {
				out["priority"] = t["priority"]
			}
			if t["priorityOverride"] != nil {
				out["priorityOverride"] = deepCloneAny(t["priorityOverride"])
			}
			targets = append(targets, out)
		}
		if len(targets) == 0 {
			targets = append(targets, tcoDefaultTarget("logs", p))
		}
	case "spans":
		targets = append(targets, tcoDefaultTarget("spans", p))
	}
	return targets
}

// sortTagRules sorts the tag rules of every span policy of an overwrite request by tag name. The
// released coralogix_tco_policies_traces builds them from a map, so it sends them in a new order on
// every run; sorted, the goldens stay stable. The API does not keep the order of the tag rules
// either (see sortResponseTagRules).
func sortTagRules(body any) {
	in, _ := body.(map[string]any)
	for _, e := range tcoList(in["policies"]) {
		rules, _ := e["spanRules"].(map[string]any)
		tags, _ := rules["tagRules"].([]any)
		slices.SortStableFunc(tags, func(a, b any) int {
			an, _ := a.(map[string]any)["tagName"].(string)
			bn, _ := b.(map[string]any)["tagName"].(string)
			return strings.Compare(an, bn)
		})
	}
}

// sortResponseTagRules sorts the tag rules of each policy of a Get or overwrite response by tag
// name, in place. The real API returned the same tag rules in different orders on two recordings, so
// the fidelity test compares them as a set.
func sortResponseTagRules(body any) {
	in, _ := body.(map[string]any)
	policies := tcoList(in["policies"])
	for _, c := range tcoList(in["createResponses"]) {
		if p, ok := c["policy"].(map[string]any); ok {
			policies = append(policies, p)
		}
	}
	for _, p := range policies {
		rules, _ := p["spanRules"].(map[string]any)
		tags, _ := rules["tagRules"].([]any)
		slices.SortStableFunc(tags, func(a, b any) int {
			an, _ := a.(map[string]any)["tagName"].(string)
			bn, _ := b.(map[string]any)["tagName"].(string)
			return strings.Compare(an, bn)
		})
	}
}

func tcoDefaultTarget(dataset string, p map[string]any) map[string]any {
	out := map[string]any{"dataset": dataset, "dataspace": "default", "priority": p["priority"]}
	if p["priorityOverride"] != nil {
		out["priorityOverride"] = deepCloneAny(p["priorityOverride"])
	}
	return out
}

func tcoLowerRule(r map[string]any) map[string]any {
	out := deepCloneMap(r)
	if name, ok := out["name"].(string); ok {
		out["name"] = strings.ToLower(name)
	}
	return out
}

func tcoHasPriority(m map[string]any) bool {
	p, _ := m["priority"].(string)
	return p != "" && p != "PRIORITY_TYPE_UNSPECIFIED"
}

func tcoDataspace(t map[string]any) string {
	if d, _ := t["dataspace"].(string); d != "" {
		return strings.ToLower(d)
	}
	return "default"
}

func tcoList(v any) []map[string]any {
	items, _ := v.([]any)
	out := make([]map[string]any, 0, len(items))
	for _, i := range items {
		if m, ok := i.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func (f *tcoFake) nextID() string {
	f.seq++
	return fmt.Sprintf("00000000-0000-4000-8000-%012d", f.seq)
}

// tcoFail is an error answer of the policies API through the REST gateway: the HTTP status text and
// the message.
func tcoFail(status int, message string) (int, any) {
	prefix := map[int]string{
		http.StatusBadRequest:          "Bad Request",
		http.StatusConflict:            "Already Exists",
		http.StatusPreconditionFailed:  "Failed Precondition",
		http.StatusInternalServerError: "Internal Server Error",
		http.StatusNotFound:            "Not Found",
	}[status]
	return status, map[string]any{"code": status, "message": prefix + ": " + message}
}

func deepCloneAny(v any) any {
	raw, _ := json.Marshal(v)
	var out any
	_ = json.Unmarshal(raw, &out)
	return out
}

func deepCloneMap(m map[string]any) map[string]any {
	out, _ := deepCloneAny(m).(map[string]any)
	return out
}

func roundTripAny(v any) any { return deepCloneAny(v) }
