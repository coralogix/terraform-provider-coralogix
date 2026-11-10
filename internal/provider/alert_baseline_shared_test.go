// Copyright 2026 Coralogix Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
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
	"strings"
)

// Scenarios of the shared attributes that the other files leave out: case_settings, access_policy,
// and data_sources. The host type is logs_immediate, as in alert_baseline_common_test.go.
//
// A destination of case_settings needs a Notification Center connector, and the baseline makes no
// connector, so the scenarios record the answer of the API for a connector that does not exist.

const (
	alertPolicyPretty = "{\n  \"version\": \"2025-01-01\",\n  \"default\": {\n    \"permissions\": {\n" +
		"      \"alerts:ReadConfig\": \"grant\",\n      \"alerts:UpdateConfig\": \"grant\",\n" +
		"      \"alerts:ReadAccessPolicy\": \"grant\",\n      \"alerts:UpdateAccessPolicy\": \"grant\"\n    }\n  },\n  \"rules\": []\n}"
	// The same policy as jsonencode writes it: sorted keys, no spaces.
	alertPolicyCanonical = `{"default":{"permissions":{"alerts:ReadAccessPolicy":"grant","alerts:ReadConfig":"grant",` +
		`"alerts:UpdateAccessPolicy":"grant","alerts:UpdateConfig":"grant"}},"rules":[],"version":"2025-01-01"}`
	alertPolicyChanged = `{"default":{"permissions":{"alerts:MetricsUpdateConfig":"grant","alerts:ReadAccessPolicy":"grant",` +
		`"alerts:ReadConfig":"grant","alerts:UpdateAccessPolicy":"grant","alerts:UpdateConfig":"grant"}},"rules":[],"version":"2025-01-01"}`
)

// alertPolicyPermissions are the permissions that the API stores in every access policy of an alert.
// The ones that a request leaves out are stored as "deny" (recorded on EU2).
var alertPolicyPermissions = []string{
	"alerts:LogsReadConfig", "alerts:LogsUpdateConfig", "alerts:MetricsReadConfig", "alerts:MetricsUpdateConfig",
	"alerts:ReadAccessPolicy", "alerts:ReadConfig", "alerts:SloMgmtReadConfig", "alerts:SloMgmtUpdateConfig",
	"alerts:SnowbitCreateSecurityAlert", "alerts:SnowbitRead", "alerts:SpansReadConfig", "alerts:SpansUpdateConfig",
	"alerts:UpdateAccessPolicy", "alerts:UpdateConfig",
}

// alertPolicyEvery is a policy that names every permission, as jsonencode writes it: the four that
// the other policies grant are granted, the rest are denied.
func alertPolicyEvery() string {
	granted := []string{"alerts:ReadAccessPolicy", "alerts:ReadConfig", "alerts:UpdateAccessPolicy", "alerts:UpdateConfig"}
	perms := map[string]any{}
	for _, name := range alertPolicyPermissions {
		perms[name] = "deny"
		for _, g := range granted {
			if g == name {
				perms[name] = "grant"
			}
		}
	}
	raw, _ := json.Marshal(map[string]any{"version": "2025-01-01", "default": map[string]any{"permissions": perms}, "rules": []any{}})
	return string(raw)
}

// alertPolicyChange reads the access policy of a Create (a string) or of a Replace (an action).
// It returns nil when the request leaves the policy as it is, a pointer to "" for a delete, and a
// pointer to the text that the API stores for an upsert. msg is the message of a 400.
func alertPolicyChange(raw any) (change *string, msg string) {
	var text string
	switch v := raw.(type) {
	case nil:
		return nil, ""
	case string:
		text = v
	case map[string]any:
		if _, del := v["delete"]; del {
			empty := ""
			return &empty, ""
		}
		upsert, ok := v["upsert"].(string)
		if !ok {
			return nil, ""
		}
		text = upsert
	default:
		return nil, ""
	}
	var policy map[string]any
	if err := json.Unmarshal([]byte(text), &policy); err != nil {
		// Recorded for the text "not json": the API words the parse error of its own JSON reader.
		return nil, "Access policy rejected: Invalid argument: Provided access_policy_raw_json is not a valid JSON: expected ident at line 1 column 2"
	}
	def, _ := policy["default"].(map[string]any)
	if def == nil {
		def = map[string]any{}
		policy["default"] = def
	}
	perms, _ := def["permissions"].(map[string]any)
	if perms == nil {
		perms = map[string]any{}
		def["permissions"] = perms
	}
	for _, name := range alertPolicyPermissions {
		if _, set := perms[name]; !set {
			perms[name] = "deny"
		}
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(policy)
	stored := strings.TrimSuffix(buf.String(), "\n")
	return &stored, ""
}

func init() {
	// The rules below are recorded answers of the real API.
	addAlertCheck(func(props map[string]any) string {
		settings, _ := props["caseSettings"].(map[string]any)
		if queries, _ := settings["enrichmentQueries"].([]any); len(queries) > 1 {
			return "[\n  {\n    \"code\": \"too_big\",\n    \"maximum\": 1,\n    \"type\": \"array\",\n    \"inclusive\": true,\n" +
				"    \"exact\": false,\n    \"message\": \"Array must contain at most 1 element(s)\",\n    \"path\": [\n" +
				"      \"alertDefProperties\",\n      \"caseSettings\",\n      \"enrichmentQueries\"\n    ]\n  }\n]"
		}
		if destinations, _ := settings["destinations"].([]any); len(destinations) > 0 {
			first, _ := destinations[0].(map[string]any)
			return "Invalid destinations: {\"reason\":\"Connector not found: " + fmt.Sprint(first["connectorId"]) + "\"}"
		}
		return ""
	})
	addAlertCheck(func(props map[string]any) string {
		// The recording tenant has one data set, default/logs.
		sources, _ := props["dataSources"].([]any)
		for _, raw := range sources {
			source, _ := raw.(map[string]any)
			if source["dataSpace"] != "default" || source["dataSet"] != "logs" {
				return "Failed creating or updating alert, could not find specified datasets in the current datasests list"
			}
		}
		return ""
	})

	addAlertScenario("common-case-settings", func(h *grHarness) {
		cfg := func(extra j) j { return alertLogsImmediate("tfbaseline-common-case", extra) }
		s := h.Apply("create with case_settings", h.null(), cfg(j{"case_settings": j{
			"auto_resolve_mode":  "disabled",
			"enrichment_queries": l{j{"query": "source logs | limit 1"}},
		}}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: enabled and another query", s, cfg(j{"case_settings": j{
			"auto_resolve_mode":  "enabled",
			"enrichment_queries": l{j{"query": "source logs | limit 2", "type": "dataprime"}},
		}}))
		s = h.Apply("update: an empty case_settings", s, cfg(j{"case_settings": j{}}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: remove case_settings", s, cfg(nil))
		s = h.RefreshPlan("refresh after removal", s)
		s = h.Apply("update: add case_settings with only a mode", s, cfg(j{"case_settings": j{"auto_resolve_mode": "disabled"}}))
		imported := h.Import("import by id", idOf(s))
		h.Plan("plan the same config on the imported state", imported, cfg(j{"case_settings": j{"auto_resolve_mode": "disabled"}}))
		s = h.Apply("update: two enrichment queries", s, cfg(j{"case_settings": j{"enrichment_queries": l{
			j{"query": "source logs | limit 1"}, j{"query": "source logs | limit 2"},
		}}}))
		s = h.Apply("update: a destination with a connector that does not exist", s, cfg(j{"case_settings": j{"destinations": l{
			j{"connector_id": "does-not-exist", "condition": "true"},
		}}}))
		h.Destroy("destroy", s)
	})

	addAlertScenario("common-case-settings-validation", func(h *grHarness) {
		bad := func(name string, caseSettings j) {
			h.validateSorted(name, alertLogsImmediate("tfbaseline-common-case-validation", j{"case_settings": caseSettings}))
		}
		bad("auto_resolve_mode outside the list", j{"auto_resolve_mode": "auto"})
		bad("enrichment query type outside the list", j{"enrichment_queries": l{j{"query": "q", "type": "sql"}}})
		bad("enrichment query without a query", j{"enrichment_queries": l{j{"type": "dataprime"}}})
		bad("destination without a condition", j{"destinations": l{j{"connector_id": "c"}}})
		bad("destination without a connector_id", j{"destinations": l{j{"condition": "true"}}})
	})

	addAlertScenario("common-access-policy", func(h *grHarness) {
		cfg := func(extra j) j { return alertLogsImmediate("tfbaseline-common-policy", extra) }
		s := h.Apply("create without access_policy", h.null(), cfg(nil))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: set access_policy", s, cfg(j{"access_policy": alertPolicyPretty}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: the same policy as jsonencode writes it", s, cfg(j{"access_policy": alertPolicyCanonical}))
		s = h.Apply("update: change the policy", s, cfg(j{"access_policy": alertPolicyChanged}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: remove access_policy", s, cfg(nil))
		s = h.RefreshPlan("refresh after removal", s)
		s = h.Apply("update: set access_policy again", s, cfg(j{"access_policy": alertPolicyCanonical}))
		s = h.Apply("update: access_policy is an empty string", s, cfg(j{"access_policy": ""}))
		s = h.RefreshPlan("refresh after the delete", s)
		imported := h.Import("import by id", idOf(s))
		h.Plan("plan the same config on the imported state", imported, cfg(j{"access_policy": ""}))
		h.Destroy("destroy", s)
	})

	addAlertScenario("common-access-policy-create", func(h *grHarness) {
		cfg := func(extra j) j { return alertLogsImmediate("tfbaseline-common-policy-create", extra) }
		s := h.Apply("create with access_policy", h.null(), cfg(j{"access_policy": alertPolicyCanonical}))
		s = h.RefreshPlan("refresh", s)
		h.Destroy("destroy", s)
		// A policy that names every permission is what the API stores, so a refresh changes nothing.
		s = h.Apply("create with a policy that names every permission", h.null(), cfg(j{"access_policy": alertPolicyEvery()}))
		s = h.RefreshPlan("refresh", s)
		h.Destroy("destroy", s)
		s = h.Apply("create with an empty access_policy", h.null(), cfg(j{"access_policy": ""}))
		s = h.RefreshPlan("refresh", s)
		h.Destroy("destroy", s)
		h.Apply("create with an access_policy that is no JSON", h.null(), cfg(j{"access_policy": "not json"}))
	})

	addAlertScenario("common-data-sources", func(h *grHarness) {
		cfg := func(extra j) j { return alertLogsImmediate("tfbaseline-common-data-sources", extra) }
		logs := j{"data_sources": l{j{"data_space": "default", "data_set": "logs"}}}
		s := h.Apply("create with data_sources", h.null(), cfg(logs))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: remove data_sources", s, cfg(nil))
		s = h.RefreshPlan("refresh after removal", s)
		s = h.Apply("update: add data_sources again", s, cfg(logs))
		imported := h.Import("import by id", idOf(s))
		h.Plan("plan the same config on the imported state", imported, cfg(logs))
		s = h.Apply("update: a data set that does not exist", s, cfg(j{"data_sources": l{j{"data_space": "default", "data_set": "does-not-exist"}}}))
		s = h.Apply("update: a data space that does not exist", s, cfg(j{"data_sources": l{j{"data_space": "does-not-exist", "data_set": "logs"}}}))
		h.Destroy("destroy", s)
	})

	addAlertScenario("common-data-sources-validation", func(h *grHarness) {
		bad := func(name string, dataSources l) {
			h.validateSorted(name, alertLogsImmediate("tfbaseline-common-data-sources-validation", j{"data_sources": dataSources}))
		}
		bad("data_sources without data_set", l{j{"data_space": "default"}})
		bad("data_sources without data_space", l{j{"data_set": "logs"}})
		bad("empty data_space", l{j{"data_space": "", "data_set": "logs"}})
		bad("empty data_set", l{j{"data_space": "default", "data_set": ""}})
	})
}
