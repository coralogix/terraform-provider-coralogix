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
	"sort"
	"strings"
	"sync"
	"testing"
)

const (
	parsingRulesBasePath  = "/parsing-rules/rule-groups/v1"
	parsingRulesGoldenDir = "testdata/parsing_rules_baseline"
)

// parsingRulesFake follows the rule groups API (openapi-facade → rules-grpc-api → rules-api) as
// the services' code reads, and as fidelity/recorded.json records:
//   - The gateway requires the group name, a name, sourceField, order, and parameters on each
//     rule, an order on each subgroup, and the fields of each parameters arm. An empty name, a
//     group without subgroups, a subgroup without rules, and a removeFields rule without fields
//     are refused by rules-api. Both answer 400.
//   - Omitted values are stored as defaults, on Create and on a PUT alike: description "",
//     creator "", enabled true, hidden false, order 1. An explicit group order of 0 appends the
//     group last. Other groups keep their order; duplicates are accepted.
//   - A PUT replaces the whole group: every subgroup and rule gets a new id.
//   - Get returns every scalar, and the subgroups and rules sorted by their order.
//   - teamId and jsonParseParameters.escapedValue are accepted and ignored; escapedValue is not
//     returned.
//   - A missing id is 404 on Get, PUT, and Delete. Delete closes the gap in the group orders.
//
// The fake does not check regular expressions (the API does not) or the order of validations.
type parsingRulesFake struct {
	mu       sync.Mutex
	groups   map[string]map[string]any
	seq      int
	fail     int
	requests []grRequest
}

func newParsingRulesFakeState() *parsingRulesFake {
	return &parsingRulesFake{groups: map[string]map[string]any{}}
}

func newParsingRulesFake(t *testing.T) *parsingRulesFake {
	t.Helper()
	f := newParsingRulesFakeState()
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

// seed stores a rule group as if it was made outside Terraform. It returns the id.
func (f *parsingRulesFake) seed(group map[string]any) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	status, body := f.write("", roundTripAny(group))
	if status != http.StatusOK {
		panic(fmt.Sprintf("seed failed: %d %v", status, body))
	}
	return parsingRulesIDOf(body)
}

// edit changes a stored rule group without a request, as if it was changed outside Terraform.
func (f *parsingRulesFake) edit(id string, change func(map[string]any)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	change(f.groups[id])
}

// forget deletes a rule group without a request, as if it was deleted outside Terraform.
func (f *parsingRulesFake) forget(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.groups, id)
}

// failNext makes the next request answer status.
func (f *parsingRulesFake) failNext(status int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fail = status
}

func (f *parsingRulesFake) takeRequests() []grRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.requests
	f.requests = nil
	return out
}

func (f *parsingRulesFake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(r.Body)
	rec := grRequest{Method: r.Method, Path: r.URL.Path}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &rec.Body)
	}
	f.mu.Lock()
	var status int
	var body any
	if f.fail != 0 {
		status, body = parsingRulesFail(f.fail, "injected failure")
		f.fail = 0
	} else {
		status, body = f.handle(r.Method, r.URL.Path, rec.Body)
	}
	// The request keeps its empty lists, so a golden shows whether a config sends [] or omits the
	// field. For ruleSubgroups, the API answers both the same way (fidelity case "refused requests").
	rec.Status = status
	f.requests = append(f.requests, rec)
	f.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func parsingRulesFail(status int, reason string) (int, any) {
	prefix := map[int]string{
		http.StatusBadRequest:          "Bad Request",
		http.StatusNotFound:            "Not Found",
		http.StatusInternalServerError: "Internal",
	}[status]
	return status, map[string]any{"code": status, "message": prefix + ": " + reason}
}

func parsingRulesMissing(id string) (int, any) {
	return parsingRulesFail(http.StatusNotFound, "RuleGroup not found: "+id)
}

// parsingRulesInvalid is a refusal of the gateway, which checks the request before rules-api.
func parsingRulesInvalid(errors ...string) (int, any) {
	return parsingRulesFail(http.StatusBadRequest, "Request errors:\n* "+strings.Join(errors, "\n* "))
}

// parsingRulesRefused is a refusal of rules-api: the go-playground validator error of one field,
// with its tag, path, kind, type, value, and parameter.
func parsingRulesRefused(verb, tag, field, kind, typ, value, param string) (int, any) {
	return parsingRulesFail(http.StatusBadRequest, fmt.Sprintf("statusCode: 400, response: Failed to %s rule group err: [{%s ParsingThemes%s %s %s %s %s}]", verb, tag, field, kind, typ, value, param))
}

func (f *parsingRulesFake) handle(method, path string, body any) (int, any) {
	_, rest, ok := strings.Cut(path, parsingRulesBasePath)
	if !ok {
		return parsingRulesFail(http.StatusNotFound, "unknown path "+path)
	}
	id := strings.TrimPrefix(rest, "/")
	in, _ := body.(map[string]any)
	switch {
	case method == http.MethodPost && rest == "":
		return f.write("", in)
	case method == http.MethodGet && rest == "":
		list := []any{}
		for _, id := range f.sortedIDs() {
			list = append(list, f.groups[id])
		}
		return http.StatusOK, map[string]any{"ruleGroups": list}
	case method == http.MethodGet && id != "":
		g, ok := f.groups[id]
		if !ok {
			return parsingRulesMissing(id)
		}
		return http.StatusOK, map[string]any{"ruleGroup": g}
	case method == http.MethodPut && id != "":
		if _, ok := f.groups[id]; !ok {
			return parsingRulesMissing(id)
		}
		return f.write(id, in)
	case method == http.MethodDelete && id != "":
		g, ok := f.groups[id]
		if !ok {
			return parsingRulesMissing(id)
		}
		removed := toInt(g["order"])
		delete(f.groups, id)
		for _, other := range f.groups {
			if o := toInt(other["order"]); o > removed {
				other["order"] = o - 1
			}
		}
		return http.StatusOK, map[string]any{}
	}
	return parsingRulesFail(http.StatusNotFound, "unknown route "+method+" "+path)
}

func (f *parsingRulesFake) sortedIDs() []string {
	ids := make([]string, 0, len(f.groups))
	for id := range f.groups {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// parsingRulesArmChecks are the fields that the gateway requires in each parameters arm, with
// the name of the arm in its messages.
var parsingRulesArmChecks = map[string]struct {
	name   string
	fields []string
}{
	"extractParameters":          {"ExtractParameters", []string{"rule"}},
	"jsonExtractParameters":      {"JsonExtractParameters", []string{"rule"}},
	"replaceParameters":          {"ReplaceParameters", []string{"destinationField", "replaceNewVal", "rule"}},
	"parseParameters":            {"ParseParameters", []string{"destinationField", "rule"}},
	"allowParameters":            {"AllowParameters", []string{"keepBlockedLogs", "rule"}},
	"blockParameters":            {"BlockParameters", []string{"keepBlockedLogs", "rule"}},
	"extractTimestampParameters": {"ExtractTimestampParameters", []string{"format"}},
	"removeFieldsParameters":     {"RemoveFieldsParameters", nil},
	"jsonStringifyParameters":    {"JsonStringifyParameters", []string{"deleteSource", "destinationField"}},
	"jsonParseParameters":        {"JsonParseParameters", []string{"deleteSource", "destinationField", "overrideDest"}},
}

// write stores a group. id is "" for a Create.
func (f *parsingRulesFake) write(id string, in map[string]any) (int, any) {
	if in == nil {
		in = map[string]any{}
	}
	if errs := parsingRulesGatewayErrors(in); len(errs) > 0 {
		return parsingRulesInvalid(errs...)
	}
	verb := "create"
	if id != "" {
		verb = "update"
	}
	if name, _ := in["name"].(string); name == "" {
		return parsingRulesRefused(verb, "required", ".Name", "string", "string", "", "")
	}
	subgroups := asList(in["ruleSubgroups"])
	if len(subgroups) == 0 {
		return parsingRulesRefused(verb, "min", ".RulesGroups", "slice", "[]*shared.RulesGroup", "[]", "1")
	}
	var outSubgroups []any
	for gi, raw := range subgroups {
		sg, _ := raw.(map[string]any)
		rules := asList(sg["rules"])
		if len(rules) == 0 {
			return parsingRulesRefused(verb, "min", fmt.Sprintf(".RulesGroups[%d].Rules", gi), "slice", "[]*shared.Rule", "[]", "1")
		}
		var outRules []any
		for ri, raw := range rules {
			r, _ := raw.(map[string]any)
			stored, status, body := f.rule(verb, fmt.Sprintf(".RulesGroups[%d].Rules[%d]", gi, ri), r)
			if status != 0 {
				return status, body
			}
			outRules = append(outRules, stored)
		}
		sortByOrder(outRules)
		outSubgroups = append(outSubgroups, map[string]any{
			"id": f.nextID(), "enabled": boolOr(sg["enabled"], true), "order": toInt(sg["order"]), "rules": outRules,
		})
	}
	sortByOrder(outSubgroups)

	order := 1
	if v, ok := in["order"]; ok {
		order = toInt(v)
	}
	if order == 0 {
		order = 1
		for gid, g := range f.groups {
			if gid != id && toInt(g["order"]) >= order {
				order = toInt(g["order"]) + 1
			}
		}
	}
	if id == "" {
		id = f.nextID()
	}
	g := map[string]any{
		"id":            id,
		"name":          in["name"],
		"description":   stringOr(in["description"], ""),
		"creator":       stringOr(in["creator"], ""),
		"enabled":       boolOr(in["enabled"], true),
		"hidden":        boolOr(in["hidden"], false),
		"order":         order,
		"ruleMatchers":  listOrEmpty(in["ruleMatchers"]),
		"ruleSubgroups": outSubgroups,
	}
	f.groups[id] = g
	return http.StatusOK, map[string]any{"ruleGroup": g}
}

// parsingRulesGatewayErrors are the presence checks of the gateway.
func parsingRulesGatewayErrors(in map[string]any) []string {
	var errs []string
	if _, ok := in["name"]; !ok {
		errs = append(errs, "Missing CreateRuleGroup.name")
	}
	for _, raw := range asList(in["ruleSubgroups"]) {
		sg, _ := raw.(map[string]any)
		if _, ok := sg["order"]; !ok {
			errs = append(errs, "Missing CreateRuleSubgroup.order")
		}
		for _, raw := range asList(sg["rules"]) {
			r, _ := raw.(map[string]any)
			for _, field := range []string{"name", "sourceField", "order"} {
				if _, ok := r[field]; !ok {
					errs = append(errs, "Missing CreateRule."+field)
				}
			}
			params, ok := r["parameters"].(map[string]any)
			switch {
			case !ok:
				errs = append(errs, "Missing CreateRule.RuleParameters")
				continue
			case len(params) == 0:
				errs = append(errs, "Empty RuleParameters")
				continue
			}
			for arm, value := range params {
				fields, _ := value.(map[string]any)
				check := parsingRulesArmChecks[arm]
				for _, field := range check.fields {
					if _, ok := fields[field]; !ok {
						errs = append(errs, "Missing "+check.name+"."+field)
					}
				}
				if arm == "jsonExtractParameters" && fields["destinationFieldType"] == "DESTINATION_FIELD_TEXT" {
					text, _ := fields["destinationFieldText"].(string)
					if text != "text" && !strings.HasPrefix(text, "text.") {
						errs = append(errs, "JsonExtractParameters.DestinationFieldText should be 'text' or start with 'text.'")
					}
				}
			}
		}
	}
	return errs
}

// rule returns the stored rule, or the status and body of the refusal of rules-api. at is the
// path of the rule in its messages.
func (f *parsingRulesFake) rule(verb, at string, r map[string]any) (map[string]any, int, any) {
	if name, _ := r["name"].(string); name == "" {
		s, b := parsingRulesRefused(verb, "required", at+".Name", "string", "string", "", "")
		return nil, s, b
	}
	source, _ := r["sourceField"].(string)
	params := map[string]any{}
	for arm, raw := range r["parameters"].(map[string]any) {
		fields, _ := raw.(map[string]any)
		stored := map[string]any{}
		for k, v := range fields {
			stored[k] = v
		}
		deleteSource, _ := fields["deleteSource"].(bool)
		switch arm {
		case "removeFieldsParameters":
			if len(asList(fields["fields"])) == 0 {
				s, b := parsingRulesRefused(verb, "required", at+".Rule", "string", "string", "", "")
				return nil, s, b
			}
		case "jsonExtractParameters":
			if _, ok := stored["destinationFieldType"]; !ok {
				stored["destinationFieldType"] = "DESTINATION_FIELD_CATEGORY_OR_UNSPECIFIED"
			}
		case "jsonParseParameters":
			delete(stored, "escapedValue")
		case "extractTimestampParameters":
			if _, ok := stored["standard"]; !ok {
				stored["standard"] = "FORMAT_STANDARD_STRFTIME_OR_UNSPECIFIED"
			}
		case "replaceParameters", "parseParameters":
			if d, _ := stored["destinationField"].(string); d == "" {
				stored["destinationField"] = source
			}
		}
		if !parsingRulesSourceField(source) || deleteSource && !strings.HasPrefix(source, "text.") {
			s, b := parsingRulesRefused(verb, "source-field-validation", at+".SourceField", "string", "string", source, "")
			return nil, s, b
		}
		params[arm] = stored
	}
	if toInt(r["order"]) == 0 {
		s, b := parsingRulesRefused(verb, "required", at+".Order", "int", "int", "0", "")
		return nil, s, b
	}
	return map[string]any{
		"id":          f.nextID(),
		"name":        r["name"],
		"description": stringOr(r["description"], ""),
		"enabled":     boolOr(r["enabled"], true),
		"order":       toInt(r["order"]),
		"sourceField": source,
		"parameters":  params,
	}, 0, nil
}

// parsingRulesSourceField is the check of rules-api: text, a text subfield, or a metadata field.
func parsingRulesSourceField(s string) bool {
	switch s {
	case "text", "applicationName", "subsystemName", "computerName", "category", "className", "methodName", "threadId", "severity":
		return true
	}
	return strings.HasPrefix(s, "text.")
}

func (f *parsingRulesFake) nextID() string {
	f.seq++
	return fmt.Sprintf("00000000-0000-4000-9000-%012d", f.seq)
}

func sortByOrder(items []any) {
	sort.SliceStable(items, func(a, b int) bool {
		return toInt(items[a].(map[string]any)["order"]) < toInt(items[b].(map[string]any)["order"])
	})
}

func toInt(v any) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case int:
		return x
	case int64:
		return int(x)
	case string:
		var n int
		_, _ = fmt.Sscan(x, &n)
		return n
	}
	return 0
}

func boolOr(v any, def bool) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return def
}

func stringOr(v any, def string) string {
	if s, ok := v.(string); ok {
		return s
	}
	return def
}

// roundTripAny returns v as it reads after JSON encoding: numbers are float64.
func roundTripAny(v map[string]any) map[string]any {
	raw, _ := json.Marshal(v)
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	return out
}

// parsingRulesIDOf returns the id of the rule group in a response, or "".
func parsingRulesIDOf(resp any) string {
	m, _ := resp.(map[string]any)
	g, _ := m["ruleGroup"].(map[string]any)
	id, _ := g["id"].(string)
	return id
}
