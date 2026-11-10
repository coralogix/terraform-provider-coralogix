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
	"strconv"
	"strings"
)

// Scenarios of logs_immediate, logs_threshold, and logs_ratio_threshold. The cross-cutting parts
// (notification_group, incidents_settings, schedule, case_settings, access_policy, labels) are
// recorded by the common scenarios and stay at their defaults here.

// alertLAWith builds a config with one type definition arm.
func alertLAWith(name, arm string, body j, top ...j) j {
	out := j{"name": name, "type_definition": j{arm: body}}
	for _, e := range top {
		out = with(out, e)
	}
	return out
}

func alertLAImm(name string, body j, top ...j) j {
	return alertLAWith(name, "logs_immediate", body, top...)
}

func alertLAThr(name string, body j, top ...j) j {
	return alertLAWith(name, "logs_threshold", body, top...)
}

func alertLARatio(name string, body j, top ...j) j {
	return alertLAWith(name, "logs_ratio_threshold", body, top...)
}

// alertLAMerge returns base with the keys of every extra on top.
func alertLAMerge(base j, extra ...j) j {
	for _, e := range extra {
		base = with(base, e)
	}
	return base
}

// alertLARule is a rule of a threshold or ratio alert. An empty priority leaves override.priority
// out of the config.
func alertLARule(condition string, threshold float64, window, priority string) j {
	override := j{}
	if priority != "" {
		override["priority"] = priority
	}
	return j{
		"condition": j{"condition_type": condition, "threshold": threshold, "time_window": window},
		"override":  override,
	}
}

func alertLAThrBody(extra ...j) j {
	return alertLAMerge(j{"rules": l{alertLARule("MORE_THAN", 5, "10_MINUTES", "")}}, extra...)
}

func alertLARatioBody(extra ...j) j {
	return alertLAMerge(j{
		"numerator_alias":   "errors",
		"denominator_alias": "all",
		"rules":             l{alertLARule("MORE_THAN", 0.5, "10_MINUTES", "")},
	}, extra...)
}

// alertLAFilter is a logs filter with an optional lucene query and optional label filters.
func alertLAFilter(lucene string, labels j) j {
	simple := j{}
	if lucene != "" {
		simple["lucene_query"] = lucene
	}
	if labels != nil {
		simple["label_filters"] = labels
	}
	return j{"simple_filter": simple}
}

func alertLALabels(app, sub string, appOp, subOp string, severities ...any) j {
	out := j{}
	if app != "" {
		item := j{"value": app}
		if appOp != "" {
			item["operation"] = appOp
		}
		out["application_name"] = l{item}
	}
	if sub != "" {
		item := j{"value": sub}
		if subOp != "" {
			item["operation"] = subOp
		}
		out["subsystem_name"] = l{item}
	}
	if len(severities) > 0 {
		out["severities"] = l(severities)
	}
	return out
}

var (
	alertLAOperations = []string{"IS", "INCLUDES", "STARTS_WITH", "ENDS_WITH"}
	alertLASeverities = []string{"Critical", "Debug", "Error", "Info", "Unspecified", "Warning"}
	alertLAPriorities = []string{"P1", "P2", "P3", "P4", "P5"}
	alertLAConditions = []string{"MORE_THAN", "LESS_THAN", "EQUALS", "NOT_EQUALS"}
	// The time windows of the schema validator, per type.
	alertLAThrWindows = []string{"5_MINUTES", "10_MINUTES", "15_MINUTES", "20_MINUTES", "30_MINUTES", "1_HOUR",
		"2_HOURS", "4_HOURS", "6_HOURS", "12_HOURS", "24_HOURS", "36_HOURS"}
	alertLARatioWindows = []string{"5_MINUTES", "10_MINUTES", "15_MINUTES", "30_MINUTES", "1_HOUR",
		"2_HOURS", "4_HOURS", "6_HOURS", "12_HOURS", "24_HOURS", "36_HOURS"}
	alertLARetireTimeframes = []string{"NEVER", "5_MINUTES", "10_MINUTES", "1_HOUR", "2_HOURS", "6_HOURS", "12_HOURS", "24_HOURS"}
	alertLANoDataStates     = []string{"OK", "ALERTING", "KEEP_LAST", "NO_DATA", "UNSPECIFIED"}
)

func init() {
	addAlertCheck(alertLACheck)
	addAlertFill(alertLAFill)
	alertLAImmediateScenarios()
	alertLAThresholdScenarios()
	alertLARatioScenarios()
}

// --- logs_immediate ---

func alertLAImmediateScenarios() {
	const base = "tfbaseline-logs-immediate-"

	addAlertScenario("logs-immediate-full-lifecycle", func(h *grHarness) {
		name := base + "full"
		full := alertLAImm(name, j{
			"logs_filter": alertLAFilter("message:\"error\"", j{
				"application_name": l{j{"value": "app-a", "operation": "IS"}, j{"value": "app-b", "operation": "STARTS_WITH"}},
				"subsystem_name":   l{j{"value": "sub-a", "operation": "ENDS_WITH"}},
				"severities":       l{"Error", "Critical"},
			}),
			"notification_payload_filter": l{"coralogix.metadata.applicationName", "coralogix.metadata.subsystemName"},
		})
		s := h.Apply("create with every optional attribute", h.null(), full)
		s = h.RefreshPlan("refresh", s)
		h.Import("import", idOf(s))
		s = h.Apply("update: change every value", s, alertLAImm(name, j{
			"logs_filter": alertLAFilter("message:\"warn\"", j{
				"application_name": l{j{"value": "app-c", "operation": "INCLUDES"}},
				"subsystem_name":   l{j{"value": "sub-b", "operation": "IS"}, j{"value": "sub-c", "operation": "INCLUDES"}},
				"severities":       l{"Warning"},
			}),
			"notification_payload_filter": l{"coralogix.metadata.severity"},
		}))
		s = h.RefreshPlan("refresh after update", s)
		h.Destroy("destroy", s)
	})

	addAlertScenario("logs-immediate-minimal-import", func(h *grHarness) {
		// logs_filter is omitted: the provider builds an empty filter and the API fills the rest.
		name := base + "minimal"
		s := h.Apply("create with an empty type definition", h.null(), alertLAImm(name, j{}))
		s = h.RefreshPlan("refresh", s)
		imported := h.Import("import", idOf(s))
		h.Apply("plan after import", imported, alertLAImm(name, j{}))
		h.Destroy("destroy", s)
	})

	addAlertScenario("logs-immediate-label-operations", func(h *grHarness) {
		name := base + "operations"
		s := h.null()
		for i, op := range alertLAOperations {
			sub := alertLAOperations[(i+1)%len(alertLAOperations)]
			s = h.Apply(fmt.Sprintf("application_name %s, subsystem_name %s", op, sub), s, alertLAImm(name, j{
				"logs_filter": alertLAFilter("q", alertLALabels("app", "sub", op, sub)),
			}))
		}
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("operation omitted defaults to IS", s, alertLAImm(name, j{
			"logs_filter": alertLAFilter("q", alertLALabels("app", "sub", "", "")),
		}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("several items with one value and different operations", s, alertLAImm(name, j{
			"logs_filter": alertLAFilter("q", j{
				"application_name": l{j{"value": "app", "operation": "IS"}, j{"value": "app", "operation": "INCLUDES"}},
				"subsystem_name":   l{j{"value": "sub"}, j{"value": "other"}},
			}),
		}))
		h.Destroy("destroy", s)
	})

	addAlertScenario("logs-immediate-severities", func(h *grHarness) {
		name := base + "severities"
		s := h.null()
		for _, sev := range alertLASeverities {
			s = h.Apply("severity "+sev, s, alertLAImm(name, j{
				"logs_filter": alertLAFilter("q", alertLALabels("", "", "", "", sev)),
			}))
			s = h.RefreshPlan("refresh "+sev, s)
		}
		s = h.Apply("all severities", s, alertLAImm(name, j{
			"logs_filter": alertLAFilter("q", j{"severities": l{"Critical", "Debug", "Error", "Info", "Unspecified", "Warning"}}),
		}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("severities cleared", s, alertLAImm(name, j{"logs_filter": alertLAFilter("q", j{})}))
		h.RefreshPlan("refresh", s)
	})

	addAlertScenario("logs-immediate-filter-shapes", func(h *grHarness) {
		name := base + "shapes"
		s := h.Apply("create without logs_filter", h.null(), alertLAImm(name, j{}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("logs_filter with an empty simple_filter", s, alertLAImm(name, j{"logs_filter": j{"simple_filter": j{}}}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("logs_filter without simple_filter", s, alertLAImm(name, j{"logs_filter": j{}}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("lucene_query only", s, alertLAImm(name, j{"logs_filter": alertLAFilter("only lucene", nil)}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("empty lucene_query and a label filter", s, alertLAImm(name, j{
			"logs_filter": j{"simple_filter": j{"lucene_query": "", "label_filters": alertLALabels("app", "", "", "")}},
		}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("label_filters only", s, alertLAImm(name, j{
			"logs_filter": j{"simple_filter": j{"label_filters": alertLALabels("", "sub", "", "")}},
		}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("empty label_filters", s, alertLAImm(name, j{
			"logs_filter": j{"simple_filter": j{"lucene_query": "q", "label_filters": j{}}},
		}))
		h.RefreshPlan("refresh", s)
	})

	addAlertScenario("logs-immediate-unset-optionals", func(h *grHarness) {
		name := base + "unset"
		labels := j{
			"application_name": l{j{"value": "app", "operation": "INCLUDES"}},
			"subsystem_name":   l{j{"value": "sub", "operation": "STARTS_WITH"}},
			"severities":       l{"Error"},
		}
		s := h.Apply("create with every optional", h.null(), alertLAImm(name, j{
			"logs_filter":                 alertLAFilter("message:\"x\"", labels),
			"notification_payload_filter": l{"coralogix.metadata.applicationName"},
		}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("remove notification_payload_filter", s, alertLAImm(name, j{"logs_filter": alertLAFilter("message:\"x\"", labels)}))
		s = h.RefreshPlan("refresh", s)
		labels = j{"application_name": labels["application_name"], "subsystem_name": labels["subsystem_name"]}
		s = h.Apply("remove severities", s, alertLAImm(name, j{"logs_filter": alertLAFilter("message:\"x\"", labels)}))
		s = h.RefreshPlan("refresh", s)
		labels = j{"application_name": labels["application_name"]}
		s = h.Apply("remove subsystem_name", s, alertLAImm(name, j{"logs_filter": alertLAFilter("message:\"x\"", labels)}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("remove application_name", s, alertLAImm(name, j{"logs_filter": alertLAFilter("message:\"x\"", j{})}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("remove lucene_query", s, alertLAImm(name, j{"logs_filter": j{"simple_filter": j{"label_filters": j{}}}}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("remove logs_filter", s, alertLAImm(name, j{}))
		h.RefreshPlan("refresh", s)
	})

	addAlertScenario("logs-immediate-collections", func(h *grHarness) {
		// Released behavior, recorded: an explicit empty set (application_name, subsystem_name,
		// severities) is accepted, the API returns it as [] and the provider reads it back as null, so
		// the apply reports an inconsistent result and the next plan is not empty.
		name := base + "collections"
		empty := alertLAImm(name, j{
			"logs_filter": j{"simple_filter": j{"lucene_query": "q", "label_filters": j{
				"application_name": l{}, "subsystem_name": l{}, "severities": l{},
			}}},
			"notification_payload_filter": l{},
		})
		// Omitted on create, then explicit empty, then values, then cleared by omission.
		s := h.Apply("create with every collection omitted", h.null(), alertLAImm(name, j{"logs_filter": alertLAFilter("q", j{})}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: explicit empty collections", s, empty)
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: non-empty collections", s, alertLAImm(name, j{
			"logs_filter":                 alertLAFilter("q", alertLALabels("app", "sub", "IS", "IS", "Info")),
			"notification_payload_filter": l{"coralogix.metadata.applicationName"},
		}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: cleared by omission", s, alertLAImm(name, j{"logs_filter": alertLAFilter("q", j{})}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: non-empty again", s, alertLAImm(name, j{
			"logs_filter":                 alertLAFilter("q", alertLALabels("app", "sub", "IS", "IS", "Info")),
			"notification_payload_filter": l{"coralogix.metadata.applicationName"},
		}))
		s = h.Apply("update: cleared by explicit empty", s, empty)
		s = h.RefreshPlan("refresh", s)
		h.Destroy("destroy", s)
	})

	addAlertScenario("logs-immediate-empty-collections-on-create", func(h *grHarness) {
		// Released behavior, recorded: see logs-immediate-collections (inconsistent result, replan not empty).
		name := base + "empty-create"
		s := h.Apply("create with explicit empty collections", h.null(), alertLAImm(name, j{
			"logs_filter": j{"simple_filter": j{"lucene_query": "q", "label_filters": j{
				"application_name": l{}, "subsystem_name": l{}, "severities": l{},
			}}},
			"notification_payload_filter": l{},
		}))
		s = h.RefreshPlan("refresh", s)
		h.Destroy("destroy", s)
	})

	addAlertScenario("logs-immediate-payload-filter", func(h *grHarness) {
		name := base + "payload"
		s := h.Apply("create with one key", h.null(), alertLAImm(name, j{"notification_payload_filter": l{"coralogix.metadata.applicationName"}}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: two keys", s, alertLAImm(name, j{"notification_payload_filter": l{"coralogix.metadata.applicationName", "coralogix.metadata.severity"}}))
		s = h.Apply("update: replace a key", s, alertLAImm(name, j{"notification_payload_filter": l{"coralogix.metadata.subsystemName", "coralogix.metadata.severity"}}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: remove", s, alertLAImm(name, j{}))
		h.RefreshPlan("refresh", s)
	})

	addAlertScenario("logs-immediate-api-errors", func(h *grHarness) {
		name := base + "errors"
		// The API validates the lucene query.
		h.Apply("create with an invalid lucene query", h.null(), alertLAImm(name, j{"logs_filter": alertLAFilter("((", nil)}))
		s := h.Apply("create", h.null(), alertLAImm(name, j{"logs_filter": alertLAFilter("ok", nil)}))
		s = h.Apply("update with an invalid lucene query", s, alertLAImm(name, j{"logs_filter": alertLAFilter("((", nil)}))
		s = h.RefreshPlan("refresh after the failed update", s)
		h.Destroy("destroy", s)
	})

	addAlertScenario("logs-immediate-config-validation", func(h *grHarness) {
		ok := j{"logs_filter": alertLAFilter("q", nil)}
		h.Validate("valid", alertLAImm("v", ok))
		h.Validate("no name", j{"type_definition": j{"logs_immediate": ok}})
		h.Validate("empty name", alertLAImm("", ok))
		h.Validate("no type_definition", j{"name": "v"})
		h.Validate("empty type_definition", j{"name": "v", "type_definition": j{}})
		h.Validate("two type definition arms", j{"name": "v", "type_definition": j{
			"logs_immediate": ok,
			"logs_threshold": alertLAThrBody(),
		}})
		h.Validate("operation IS_NOT", alertLAImm("v", j{"logs_filter": alertLAFilter("q", alertLALabels("a", "", "IS_NOT", ""))}))
		h.Validate("operation in lower case", alertLAImm("v", j{"logs_filter": alertLAFilter("q", alertLALabels("", "s", "", "is"))}))
		h.Validate("operation empty", alertLAImm("v", j{"logs_filter": alertLAFilter("q", alertLALabels("a", "", "", ""))}))
		h.Validate("operation null with a value", alertLAImm("v", j{"logs_filter": alertLAFilter("q", j{"application_name": l{j{"operation": "IS"}}})}))
		h.Validate("value null in subsystem_name", alertLAImm("v", j{"logs_filter": alertLAFilter("q", j{"subsystem_name": l{j{"operation": "IS"}}})}))
		h.Validate("severity Fatal", alertLAImm("v", j{"logs_filter": alertLAFilter("q", j{"severities": l{"Fatal"}})}))
		h.Validate("severity in upper case", alertLAImm("v", j{"logs_filter": alertLAFilter("q", j{"severities": l{"CRITICAL"}})}))
		h.Validate("severity null member", alertLAImm("v", j{"logs_filter": alertLAFilter("q", j{"severities": l{nil}})}))
		h.Validate("payload filter null member", alertLAImm("v", j{"notification_payload_filter": l{nil}}))
		h.Validate("group_by is not allowed", alertLAImm("v", ok, j{"group_by": l{"coralogix.metadata.applicationName"}}))
		h.Validate("empty group_by is not allowed either", alertLAImm("v", ok, j{"group_by": l{}}))
		h.Validate("unknown group_by", alertLAImm("v", ok, j{"group_by": unknownJSON{}}))
		h.Validate("unknown lucene_query", alertLAImm("v", j{"logs_filter": j{"simple_filter": j{"lucene_query": unknownJSON{}}}}))
		h.Validate("priority P6", alertLAImm("v", ok, j{"priority": "P6"}))
		h.Validate("priority in lower case", alertLAImm("v", ok, j{"priority": "p1"}))
		h.Validate("priority is not deprecated for logs_immediate", alertLAImm("v", ok, j{"priority": "P1"}))
	})
}

// --- logs_threshold ---

func alertLAThresholdScenarios() {
	const base = "tfbaseline-logs-threshold-"

	addAlertScenario("logs-threshold-full-lifecycle", func(h *grHarness) {
		name := base + "full"
		full := alertLAThr(name, alertLAThrBody(j{
			"logs_filter": alertLAFilter("message:\"error\"", alertLALabels("app-a", "sub-a", "INCLUDES", "ENDS_WITH", "Error", "Critical")),
			"rules": l{
				alertLARule("LESS_THAN", 3, "30_MINUTES", "P1"),
			},
			"notification_payload_filter":  l{"coralogix.metadata.applicationName"},
			"undetected_values_management": j{"trigger_undetected_values": true, "auto_retire_timeframe": "1_HOUR"},
			"no_data_policy":               j{"state": "ALERTING", "auto_retire_seconds": float64(300)},
		}), j{"group_by": l{"coralogix.metadata.applicationName"}, "priority": "P3"})
		s := h.Apply("create with every optional attribute", h.null(), full)
		s = h.RefreshPlan("refresh", s)
		h.Import("import", idOf(s))
		s = h.Apply("update: change every value", s, alertLAThr(name, alertLAThrBody(j{
			"logs_filter": alertLAFilter("message:\"warn\"", alertLALabels("app-b", "sub-b", "STARTS_WITH", "IS", "Warning")),
			"rules": l{
				alertLARule("LESS_THAN", 7, "1_HOUR", "P2"),
			},
			"notification_payload_filter":  l{"coralogix.metadata.subsystemName"},
			"undetected_values_management": j{"trigger_undetected_values": true, "auto_retire_timeframe": "6_HOURS"},
			"no_data_policy":               j{"state": "KEEP_LAST", "auto_retire_seconds": float64(600)},
		}), j{"group_by": l{"coralogix.metadata.subsystemName"}, "priority": "P2"}))
		s = h.RefreshPlan("refresh after update", s)
		h.Destroy("destroy", s)
	})

	addAlertScenario("logs-threshold-minimal-import", func(h *grHarness) {
		name := base + "minimal"
		cfg := alertLAThr(name, alertLAThrBody())
		s := h.Apply("create with only the required attributes", h.null(), cfg)
		s = h.RefreshPlan("refresh", s)
		imported := h.Import("import", idOf(s))
		h.Apply("plan after import", imported, cfg)
		s = h.Apply("update: change the threshold", s, alertLAThr(name, j{"rules": l{alertLARule("MORE_THAN", 9, "10_MINUTES", "")}}))
		h.Destroy("destroy", s)
	})

	addAlertScenario("logs-threshold-condition-types", func(h *grHarness) {
		name := base + "conditions"
		s := h.null()
		for _, c := range alertLAConditions {
			s = h.Apply("condition_type "+c, s, alertLAThr(name, j{"rules": l{alertLARule(c, 5, "10_MINUTES", "")}}))
			s = h.RefreshPlan("refresh "+c, s)
		}
		h.Destroy("destroy", s)
	})

	addAlertScenario("logs-threshold-time-windows", func(h *grHarness) {
		name := base + "windows"
		s := h.null()
		for _, w := range alertLAThrWindows {
			s = h.Apply("time_window "+w, s, alertLAThr(name, j{"rules": l{alertLARule("MORE_THAN", 5, w, "")}}))
			s = h.RefreshPlan("refresh "+w, s)
		}
		h.Destroy("destroy", s)
	})

	addAlertScenario("logs-threshold-threshold-values", func(h *grHarness) {
		name := base + "values"
		s := h.null()
		for _, v := range []float64{0, 1, 2.5, 100000, -1} {
			s = h.Apply(fmt.Sprintf("threshold %v", v), s, alertLAThr(name, j{"rules": l{alertLARule("MORE_THAN", v, "10_MINUTES", "")}}))
			s = h.RefreshPlan("refresh", s)
		}
		h.Destroy("destroy", s)
	})

	addAlertScenario("logs-threshold-override-priority", func(h *grHarness) {
		name := base + "override"
		s := h.null()
		for _, p := range alertLAPriorities {
			s = h.Apply("override.priority "+p, s, alertLAThr(name, j{"rules": l{alertLARule("MORE_THAN", 5, "10_MINUTES", p)}}))
			s = h.RefreshPlan("refresh "+p, s)
		}
		s = h.Apply("override.priority removed, no top-level priority: falls back to P5", s,
			alertLAThr(name, j{"rules": l{alertLARule("MORE_THAN", 5, "10_MINUTES", "")}}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("override.priority set, then priority P2 on top", s,
			alertLAThr(name, j{"rules": l{alertLARule("MORE_THAN", 5, "10_MINUTES", "P4")}}, j{"priority": "P2"}))
		s = h.Apply("override.priority removed, top-level priority P2: falls back to P2", s,
			alertLAThr(name, j{"rules": l{alertLARule("MORE_THAN", 5, "10_MINUTES", "")}}, j{"priority": "P2"}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("top-level priority changed to P1: the override follows", s,
			alertLAThr(name, j{"rules": l{alertLARule("MORE_THAN", 5, "10_MINUTES", "")}}, j{"priority": "P1"}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("top-level priority removed", s,
			alertLAThr(name, j{"rules": l{alertLARule("MORE_THAN", 5, "10_MINUTES", "")}}))
		h.RefreshPlan("refresh", s)
	})

	addAlertScenario("logs-threshold-deprecated-priority", func(h *grHarness) {
		name := base + "priority"
		cfg := func(p string) j {
			top := j{}
			if p != "" {
				top["priority"] = p
			}
			return alertLAThr(name, alertLAThrBody(), top)
		}
		h.Validate("priority on a threshold alert warns", cfg("P3"))
		s := h.null()
		for _, p := range alertLAPriorities {
			s = h.Apply("priority "+p, s, cfg(p))
			s = h.RefreshPlan("refresh "+p, s)
		}
		s = h.Apply("priority removed on update", s, cfg(""))
		s = h.RefreshPlan("refresh", s)
		h.Destroy("destroy", s)
	})

	addAlertScenario("logs-threshold-undetected-values", func(h *grHarness) {
		name := base + "undetected"
		less := l{alertLARule("LESS_THAN", 1, "10_MINUTES", "")}
		more := l{alertLARule("MORE_THAN", 1, "10_MINUTES", "")}
		s := h.Apply("create with undetected_values_management omitted", h.null(), alertLAThr(name, j{"rules": more}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: trigger on a LESS_THAN rule", s, alertLAThr(name, j{"rules": less, "undetected_values_management": j{"trigger_undetected_values": true, "auto_retire_timeframe": "NEVER"}}))
		s = h.RefreshPlan("refresh", s)
		for _, tf := range alertLARetireTimeframes {
			s = h.Apply("auto_retire_timeframe "+tf, s, alertLAThr(name, j{"rules": less, "undetected_values_management": j{"trigger_undetected_values": true, "auto_retire_timeframe": tf}}))
			s = h.RefreshPlan("refresh "+tf, s)
		}
		s = h.Apply("trigger_undetected_values false with a timeframe", s, alertLAThr(name, j{"rules": less, "undetected_values_management": j{"trigger_undetected_values": false, "auto_retire_timeframe": "1_HOUR"}}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("only trigger_undetected_values", s, alertLAThr(name, j{"rules": less, "undetected_values_management": j{"trigger_undetected_values": true}}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("only auto_retire_timeframe", s, alertLAThr(name, j{"rules": less, "undetected_values_management": j{"auto_retire_timeframe": "2_HOURS"}}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("empty undetected_values_management", s, alertLAThr(name, j{"rules": less, "undetected_values_management": j{}}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("set again", s, alertLAThr(name, j{"rules": less, "undetected_values_management": j{"trigger_undetected_values": true, "auto_retire_timeframe": "1_HOUR"}}))
		s = h.Apply("removed on update", s, alertLAThr(name, j{"rules": less}))
		s = h.RefreshPlan("refresh", s)
		h.Destroy("destroy", s)
	})

	addAlertScenario("logs-threshold-undetected-values-errors", func(h *grHarness) {
		name := base + "undetected-err"
		more := l{alertLARule("MORE_THAN", 1, "10_MINUTES", "")}
		h.Apply("create: trigger on a MORE_THAN rule", h.null(), alertLAThr(name, j{"rules": more, "undetected_values_management": j{"trigger_undetected_values": true, "auto_retire_timeframe": "1_HOUR"}}))
		s := h.Apply("create", h.null(), alertLAThr(name, j{"rules": more}))
		s = h.Apply("update: trigger on a MORE_THAN rule", s, alertLAThr(name, j{"rules": more, "undetected_values_management": j{"trigger_undetected_values": true, "auto_retire_timeframe": "1_HOUR"}}))
		s = h.RefreshPlan("refresh after the failed update", s)
		h.Destroy("destroy", s)
	})

	addAlertScenario("logs-threshold-no-data-policy", func(h *grHarness) {
		// Released behavior, recorded: removing no_data_policy does not clear it (the state is kept, no
		// request is sent). A no_data_policy with only auto_retire_seconds, or an empty one, makes the API
		// answer state UNSPECIFIED, which the next plan shows as a diff against the null config.
		name := base + "nodata"
		rules := l{alertLARule("MORE_THAN", 1, "10_MINUTES", "")}
		s := h.Apply("create with no_data_policy omitted", h.null(), alertLAThr(name, j{"rules": rules}))
		s = h.RefreshPlan("refresh", s)
		for _, st := range alertLANoDataStates {
			s = h.Apply("state "+st, s, alertLAThr(name, j{"rules": rules, "no_data_policy": j{"state": st}}))
			s = h.RefreshPlan("refresh "+st, s)
		}
		s = h.Apply("state with auto_retire_seconds", s, alertLAThr(name, j{"rules": rules, "no_data_policy": j{"state": "ALERTING", "auto_retire_seconds": float64(600)}}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("change auto_retire_seconds", s, alertLAThr(name, j{"rules": rules, "no_data_policy": j{"state": "ALERTING", "auto_retire_seconds": float64(1200)}}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("state OK with auto_retire_seconds", s, alertLAThr(name, j{"rules": rules, "no_data_policy": j{"state": "OK", "auto_retire_seconds": float64(600)}}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("only auto_retire_seconds", s, alertLAThr(name, j{"rules": rules, "no_data_policy": j{"auto_retire_seconds": float64(600)}}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("remove auto_retire_seconds, keep state", s, alertLAThr(name, j{"rules": rules, "no_data_policy": j{"state": "NO_DATA"}}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("empty no_data_policy", s, alertLAThr(name, j{"rules": rules, "no_data_policy": j{}}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("set again", s, alertLAThr(name, j{"rules": rules, "no_data_policy": j{"state": "ALERTING", "auto_retire_seconds": float64(300)}}))
		s = h.Apply("removed on update", s, alertLAThr(name, j{"rules": rules}))
		s = h.RefreshPlan("refresh", s)
		h.Destroy("destroy", s)
	})

	addAlertScenario("logs-threshold-no-data-policy-errors", func(h *grHarness) {
		name := base + "nodata-err"
		rules := l{alertLARule("MORE_THAN", 1, "10_MINUTES", "")}
		h.Apply("create: auto_retire_seconds is no multiple of 60", h.null(), alertLAThr(name, j{"rules": rules, "no_data_policy": j{"state": "ALERTING", "auto_retire_seconds": float64(90)}}))
		s := h.Apply("create", h.null(), alertLAThr(name, j{"rules": rules}))
		s = h.Apply("update: auto_retire_seconds below 60", s, alertLAThr(name, j{"rules": rules, "no_data_policy": j{"state": "ALERTING", "auto_retire_seconds": float64(30)}}))
		s = h.RefreshPlan("refresh after the failed update", s)
		h.Destroy("destroy", s)
	})

	addAlertScenario("logs-threshold-evaluation-delay", func(h *grHarness) {
		name := base + "delay"
		rules := l{alertLARule("MORE_THAN", 1, "10_MINUTES", "")}
		s := h.Apply("create with custom_evaluation_delay omitted", h.null(), alertLAThr(name, j{"rules": rules}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: 0", s, alertLAThr(name, j{"rules": rules, "custom_evaluation_delay": float64(0)}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: 60000", s, alertLAThr(name, j{"rules": rules, "custom_evaluation_delay": float64(60000)}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: 120000", s, alertLAThr(name, j{"rules": rules, "custom_evaluation_delay": float64(120000)}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: removed", s, alertLAThr(name, j{"rules": rules}))
		s = h.RefreshPlan("refresh", s)
		h.Destroy("destroy", s)
	})

	addAlertScenario("logs-threshold-group-by", func(h *grHarness) {
		// Released behavior, recorded: an explicit empty group_by is sent as [] and read back as null, so the
		// apply reports an inconsistent result and the next plan is not empty.
		name := base + "groupby"
		body := alertLAThrBody()
		s := h.Apply("create with group_by omitted", h.null(), alertLAThr(name, body))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: one key", s, alertLAThr(name, body, j{"group_by": l{"coralogix.metadata.applicationName"}}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: two keys", s, alertLAThr(name, body, j{"group_by": l{"coralogix.metadata.applicationName", "coralogix.metadata.subsystemName"}}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: reorder the keys", s, alertLAThr(name, body, j{"group_by": l{"coralogix.metadata.subsystemName", "coralogix.metadata.applicationName"}}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: removed by omission", s, alertLAThr(name, body))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: explicit empty list", s, alertLAThr(name, body, j{"group_by": l{}}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: one key again", s, alertLAThr(name, body, j{"group_by": l{"coralogix.metadata.applicationName"}}))
		s = h.Apply("update: removed by explicit empty list", s, alertLAThr(name, body, j{"group_by": l{}}))
		s = h.RefreshPlan("refresh", s)
		h.Destroy("destroy", s)
	})

	addAlertScenario("logs-threshold-group-by-on-create", func(h *grHarness) {
		// Released behavior, recorded: see logs-threshold-group-by.
		s := h.Apply("create with an explicit empty group_by", h.null(), alertLAThr(base+"groupby-empty", alertLAThrBody(), j{"group_by": l{}}))
		s = h.RefreshPlan("refresh", s)
		h.Destroy("destroy", s)
	})

	addAlertScenario("logs-threshold-rules", func(h *grHarness) {
		name := base + "rules"
		r1 := alertLARule("MORE_THAN", 10, "10_MINUTES", "P3")
		r2 := alertLARule("MORE_THAN", 100, "10_MINUTES", "P1")
		r3 := alertLARule("MORE_THAN", 50, "10_MINUTES", "P2")
		s := h.Apply("create with one rule", h.null(), alertLAThr(name, j{"rules": l{r1}}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: two rules", s, alertLAThr(name, j{"rules": l{r1, r2}}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: three rules", s, alertLAThr(name, j{"rules": l{r1, r2, r3}}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: two rules, the middle one removed", s, alertLAThr(name, j{"rules": l{r1, r2}}))
		s = h.Apply("update: change a threshold in one rule", s, alertLAThr(name, j{"rules": l{r1, alertLARule("MORE_THAN", 200, "10_MINUTES", "P1")}}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: back to one rule", s, alertLAThr(name, j{"rules": l{r1}}))
		s = h.RefreshPlan("refresh", s)
		h.Destroy("destroy", s)
	})

	addAlertScenario("logs-threshold-rules-mixed-conditions", func(h *grHarness) {
		name := base + "rules-mixed"
		// The API refuses a mix of MORE_THAN and LESS_THAN rules.
		h.Apply("create: MORE_THAN and LESS_THAN rules", h.null(), alertLAThr(name, j{"rules": l{
			alertLARule("MORE_THAN", 100, "10_MINUTES", "P1"),
			alertLARule("LESS_THAN", 1, "10_MINUTES", "P2"),
		}}))
		s := h.Apply("create: two MORE_THAN rules", h.null(), alertLAThr(name, j{"rules": l{
			alertLARule("MORE_THAN", 100, "10_MINUTES", "P1"),
			alertLARule("MORE_THAN", 10, "10_MINUTES", "P2"),
		}}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: MORE_THAN and LESS_THAN rules", s, alertLAThr(name, j{"rules": l{
			alertLARule("MORE_THAN", 100, "10_MINUTES", "P1"),
			alertLARule("LESS_THAN", 1, "10_MINUTES", "P2"),
		}}))
		s = h.RefreshPlan("refresh after the failed update", s)
		// The API groups the time windows: [5, 30] minutes, [1, 12] hours, [12, 36] hours.
		s = h.Apply("update: two rules in different time window groups", s, alertLAThr(name, j{"rules": l{
			alertLARule("MORE_THAN", 100, "10_MINUTES", "P1"),
			alertLARule("MORE_THAN", 10, "1_HOUR", "P2"),
		}}))
		s = h.Apply("update: two rules in the same time window group", s, alertLAThr(name, j{"rules": l{
			alertLARule("MORE_THAN", 100, "10_MINUTES", "P1"),
			alertLARule("MORE_THAN", 10, "30_MINUTES", "P2"),
		}}))
		s = h.RefreshPlan("refresh", s)
		// Rules with different conditions must have different priorities.
		s = h.Apply("update: EQUALS and NOT_EQUALS rules with the same priority", s, alertLAThr(name, j{"rules": l{
			alertLARule("EQUALS", 1, "10_MINUTES", ""),
			alertLARule("NOT_EQUALS", 2, "10_MINUTES", ""),
		}}))
		s = h.Apply("update: EQUALS and NOT_EQUALS rules with different priorities", s, alertLAThr(name, j{"rules": l{
			alertLARule("EQUALS", 1, "10_MINUTES", "P1"),
			alertLARule("NOT_EQUALS", 2, "10_MINUTES", "P2"),
		}}))
		s = h.RefreshPlan("refresh", s)
		h.Destroy("destroy", s)
	})

	addAlertScenario("logs-threshold-filter", func(h *grHarness) {
		name := base + "filter"
		body := func(filter j) j { return alertLAThrBody(j{"logs_filter": filter}) }
		s := h.Apply("create without logs_filter", h.null(), alertLAThr(name, alertLAThrBody()))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: lucene_query and every label filter", s, alertLAThr(name, body(alertLAFilter("message:\"a\"", alertLALabels("app", "sub", "ENDS_WITH", "INCLUDES", "Debug", "Info")))))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: change them", s, alertLAThr(name, body(alertLAFilter("message:\"b\"", alertLALabels("app2", "sub2", "STARTS_WITH", "IS", "Critical")))))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: remove the label filters", s, alertLAThr(name, body(alertLAFilter("message:\"b\"", j{}))))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: remove lucene_query", s, alertLAThr(name, body(j{"simple_filter": j{"label_filters": alertLALabels("app", "", "", "")}})))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: remove logs_filter", s, alertLAThr(name, alertLAThrBody()))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: logs_filter with an empty simple_filter", s, alertLAThr(name, body(j{"simple_filter": j{}})))
		h.RefreshPlan("refresh", s)
	})

	addAlertScenario("logs-threshold-payload-filter", func(h *grHarness) {
		// Released behavior, recorded: an explicit empty notification_payload_filter reads back as null
		// (inconsistent result, replan not empty).
		name := base + "payload"
		s := h.Apply("create with a key", h.null(), alertLAThr(name, alertLAThrBody(j{"notification_payload_filter": l{"coralogix.metadata.applicationName"}})))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: other keys", s, alertLAThr(name, alertLAThrBody(j{"notification_payload_filter": l{"coralogix.metadata.severity", "coralogix.metadata.subsystemName"}})))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: explicit empty", s, alertLAThr(name, alertLAThrBody(j{"notification_payload_filter": l{}})))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: a key again", s, alertLAThr(name, alertLAThrBody(j{"notification_payload_filter": l{"coralogix.metadata.severity"}})))
		s = h.Apply("update: removed by omission", s, alertLAThr(name, alertLAThrBody()))
		h.RefreshPlan("refresh", s)
	})

	addAlertScenario("logs-threshold-config-validation", func(h *grHarness) {
		ok := alertLAThrBody()
		oneRule := func(rule j) j { return alertLAThr("v", j{"rules": l{rule}}) }
		h.Validate("valid", alertLAThr("v", ok))
		h.Validate("rules omitted", alertLAThr("v", j{}))
		h.Validate("rules empty", alertLAThr("v", j{"rules": l{}}))
		h.Validate("rule without condition", oneRule(j{"override": j{}}))
		h.Validate("rule without override", oneRule(j{"condition": j{"condition_type": "MORE_THAN", "threshold": float64(1), "time_window": "10_MINUTES"}}))
		h.Validate("condition without condition_type", oneRule(j{"condition": j{"threshold": float64(1), "time_window": "10_MINUTES"}, "override": j{}}))
		h.Validate("condition without threshold", oneRule(j{"condition": j{"condition_type": "MORE_THAN", "time_window": "10_MINUTES"}, "override": j{}}))
		h.Validate("condition without time_window", oneRule(j{"condition": j{"condition_type": "MORE_THAN", "threshold": float64(1)}, "override": j{}}))
		h.Validate("condition_type MORE", oneRule(alertLARule("MORE", 1, "10_MINUTES", "")))
		h.Validate("condition_type in lower case", oneRule(alertLARule("more_than", 1, "10_MINUTES", "")))
		h.Validate("condition_type empty", oneRule(alertLARule("", 1, "10_MINUTES", "")))
		h.Validate("time_window 7_MINUTES", oneRule(alertLARule("MORE_THAN", 1, "7_MINUTES", "")))
		h.Validate("time_window in lower case", oneRule(alertLARule("MORE_THAN", 1, "1_hour", "")))
		h.Validate("time_window 48_HOURS", oneRule(alertLARule("MORE_THAN", 1, "48_HOURS", "")))
		h.Validate("override.priority P6", oneRule(alertLARule("MORE_THAN", 1, "10_MINUTES", "P6")))
		h.Validate("override.priority in lower case", oneRule(alertLARule("MORE_THAN", 1, "10_MINUTES", "p1")))
		h.Validate("unknown threshold", oneRule(j{"condition": j{"condition_type": "MORE_THAN", "threshold": unknownJSON{}, "time_window": "10_MINUTES"}, "override": j{}}))
		h.Validate("auto_retire_timeframe 20_MINUTES", alertLAThr("v", alertLAMerge(ok, j{"undetected_values_management": j{"auto_retire_timeframe": "20_MINUTES"}})))
		h.Validate("auto_retire_timeframe in lower case", alertLAThr("v", alertLAMerge(ok, j{"undetected_values_management": j{"auto_retire_timeframe": "never"}})))
		h.Validate("no_data_policy state in lower case", alertLAThr("v", alertLAMerge(ok, j{"no_data_policy": j{"state": "ok"}})))
		h.Validate("no_data_policy state unknown value", alertLAThr("v", alertLAMerge(ok, j{"no_data_policy": j{"state": "SILENT"}})))
		h.Validate("no_data_policy state UNSPECIFIED warns", alertLAThr("v", alertLAMerge(ok, j{"no_data_policy": j{"state": "UNSPECIFIED"}})))
		h.Validate("no_data_policy state empty", alertLAThr("v", alertLAMerge(ok, j{"no_data_policy": j{"state": ""}})))
		h.Validate("custom_evaluation_delay negative", alertLAThr("v", alertLAMerge(ok, j{"custom_evaluation_delay": float64(-1)})))
		h.Validate("custom_evaluation_delay fraction", alertLAThr("v", alertLAMerge(ok, j{"custom_evaluation_delay": 0.5})))
		h.Validate("group_by is allowed", alertLAThr("v", ok, j{"group_by": l{"coralogix.metadata.applicationName"}}))
		h.Validate("priority warns", alertLAThr("v", ok, j{"priority": "P2"}))
		h.Validate("priority P6", alertLAThr("v", ok, j{"priority": "P6"}))
		h.Validate("filter operation IS_NOT", alertLAThr("v", alertLAMerge(ok, j{"logs_filter": alertLAFilter("q", alertLALabels("a", "", "IS_NOT", ""))})))
		h.Validate("filter severity Fatal", alertLAThr("v", alertLAMerge(ok, j{"logs_filter": alertLAFilter("q", j{"severities": l{"Fatal"}})})))
	})
}

// --- logs_ratio_threshold ---

func alertLARatioScenarios() {
	const base = "tfbaseline-logs-ratio-"

	addAlertScenario("logs-ratio-full-lifecycle", func(h *grHarness) {
		name := base + "full"
		full := alertLARatio(name, alertLARatioBody(j{
			"numerator":   alertLAFilter("level:error", alertLALabels("app-a", "sub-a", "INCLUDES", "ENDS_WITH", "Error")),
			"denominator": alertLAFilter("level:*", alertLALabels("app-a", "sub-b", "STARTS_WITH", "IS", "Info", "Warning")),
			"rules": l{
				alertLARule("LESS_THAN", 0.25, "30_MINUTES", "P1"),
			},
			"notification_payload_filter":  l{"coralogix.metadata.applicationName"},
			"group_by_for":                 "Numerator Only",
			"ignore_infinity":              true,
			"undetected_values_management": j{"trigger_undetected_values": true, "auto_retire_timeframe": "1_HOUR"},
		}), j{"group_by": l{"coralogix.metadata.applicationName"}, "priority": "P3"})
		s := h.Apply("create with every optional attribute", h.null(), full)
		s = h.RefreshPlan("refresh", s)
		h.Import("import", idOf(s))
		s = h.Apply("update: change every value", s, alertLARatio(name, alertLARatioBody(j{
			"numerator_alias":   "failures",
			"denominator_alias": "requests",
			"numerator":         alertLAFilter("level:fatal", alertLALabels("app-b", "sub-c", "STARTS_WITH", "INCLUDES", "Critical")),
			"denominator":       alertLAFilter("level:warn", alertLALabels("app-c", "sub-d", "ENDS_WITH", "IS", "Debug")),
			"rules": l{
				alertLARule("LESS_THAN", 0.75, "1_HOUR", "P2"),
			},
			"notification_payload_filter":  l{"coralogix.metadata.subsystemName"},
			"group_by_for":                 "Denominator Only",
			"ignore_infinity":              false,
			"undetected_values_management": j{"trigger_undetected_values": true, "auto_retire_timeframe": "6_HOURS"},
		}), j{"group_by": l{"coralogix.metadata.subsystemName"}, "priority": "P2"}))
		s = h.RefreshPlan("refresh after update", s)
		h.Destroy("destroy", s)
	})

	addAlertScenario("logs-ratio-minimal-import", func(h *grHarness) {
		name := base + "minimal"
		cfg := alertLARatio(name, alertLARatioBody())
		s := h.Apply("create with only the required attributes (numerator and denominator omitted)", h.null(), cfg)
		s = h.RefreshPlan("refresh", s)
		imported := h.Import("import", idOf(s))
		h.Apply("plan after import", imported, cfg)
		s = h.Apply("update: change the threshold", s, alertLARatio(name, alertLARatioBody(j{"rules": l{alertLARule("MORE_THAN", 0.9, "10_MINUTES", "")}})))
		h.Destroy("destroy", s)
	})

	addAlertScenario("logs-ratio-condition-types", func(h *grHarness) {
		name := base + "conditions"
		s := h.null()
		for _, c := range []string{"MORE_THAN", "LESS_THAN"} {
			s = h.Apply("condition_type "+c, s, alertLARatio(name, alertLARatioBody(j{"rules": l{alertLARule(c, 0.5, "10_MINUTES", "")}})))
			s = h.RefreshPlan("refresh "+c, s)
		}
		h.Destroy("destroy", s)
	})

	addAlertScenario("logs-ratio-time-windows", func(h *grHarness) {
		name := base + "windows"
		s := h.null()
		for _, w := range alertLARatioWindows {
			s = h.Apply("time_window "+w, s, alertLARatio(name, alertLARatioBody(j{"rules": l{alertLARule("MORE_THAN", 0.5, w, "")}})))
			s = h.RefreshPlan("refresh "+w, s)
		}
		h.Destroy("destroy", s)
	})

	addAlertScenario("logs-ratio-aliases", func(h *grHarness) {
		name := base + "aliases"
		s := h.Apply("create with aliases", h.null(), alertLARatio(name, alertLARatioBody()))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: change both aliases", s, alertLARatio(name, alertLARatioBody(j{"numerator_alias": "bad", "denominator_alias": "total"})))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: the default alias names of the API", s, alertLARatio(name, alertLARatioBody(j{"numerator_alias": "Query 1", "denominator_alias": "Query 2"})))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: equal aliases", s, alertLARatio(name, alertLARatioBody(j{"numerator_alias": "same", "denominator_alias": "same"})))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: empty aliases", s, alertLARatio(name, alertLARatioBody(j{"numerator_alias": "", "denominator_alias": ""})))
		s = h.RefreshPlan("refresh", s)
		h.Destroy("destroy", s)
	})

	addAlertScenario("logs-ratio-numerator-denominator", func(h *grHarness) {
		name := base + "filters"
		body := func(num, den j) j {
			extra := j{}
			if num != nil {
				extra["numerator"] = num
			}
			if den != nil {
				extra["denominator"] = den
			}
			return alertLARatioBody(extra)
		}
		s := h.Apply("create with both omitted", h.null(), alertLARatio(name, body(nil, nil)))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: numerator only", s, alertLARatio(name, body(alertLAFilter("level:error", nil), nil)))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: denominator only", s, alertLARatio(name, body(nil, alertLAFilter("level:*", nil))))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: both with every label filter", s, alertLARatio(name, body(
			alertLAFilter("a", alertLALabels("app", "sub", "IS", "INCLUDES", "Error")),
			alertLAFilter("b", alertLALabels("app", "sub", "STARTS_WITH", "ENDS_WITH", "Info")))))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: change them", s, alertLARatio(name, body(
			alertLAFilter("c", alertLALabels("app2", "sub2", "ENDS_WITH", "IS", "Critical")),
			alertLAFilter("d", alertLALabels("app3", "sub3", "INCLUDES", "STARTS_WITH", "Warning")))))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: remove the label filters", s, alertLARatio(name, body(alertLAFilter("c", j{}), alertLAFilter("d", j{}))))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: remove lucene_query", s, alertLARatio(name, body(
			j{"simple_filter": j{"label_filters": alertLALabels("app", "", "", "")}},
			j{"simple_filter": j{"label_filters": alertLALabels("", "sub", "", "")}})))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: remove both", s, alertLARatio(name, body(nil, nil)))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: empty filter objects", s, alertLARatio(name, body(j{}, j{"simple_filter": j{}})))
		h.RefreshPlan("refresh", s)
	})

	addAlertScenario("logs-ratio-severities", func(h *grHarness) {
		name := base + "severities"
		s := h.null()
		for _, sev := range alertLASeverities {
			s = h.Apply("numerator severity "+sev, s, alertLARatio(name, alertLARatioBody(j{
				"numerator":   alertLAFilter("q", alertLALabels("", "", "", "", sev)),
				"denominator": alertLAFilter("q", alertLALabels("", "", "", "", sev)),
			})))
			s = h.RefreshPlan("refresh "+sev, s)
		}
		h.Destroy("destroy", s)
	})

	addAlertScenario("logs-ratio-label-operations", func(h *grHarness) {
		name := base + "operations"
		s := h.null()
		for i, op := range alertLAOperations {
			sub := alertLAOperations[(i+1)%len(alertLAOperations)]
			s = h.Apply(fmt.Sprintf("numerator %s/%s, denominator %s/%s", op, sub, sub, op), s, alertLARatio(name, alertLARatioBody(j{
				"numerator":   alertLAFilter("q", alertLALabels("app", "sub", op, sub)),
				"denominator": alertLAFilter("q", alertLALabels("app", "sub", sub, op)),
			})))
			s = h.RefreshPlan("refresh", s)
		}
		s = h.Apply("operation omitted defaults to IS", s, alertLARatio(name, alertLARatioBody(j{
			"numerator":   alertLAFilter("q", alertLALabels("app", "sub", "", "")),
			"denominator": alertLAFilter("q", alertLALabels("app", "sub", "", "")),
		})))
		s = h.RefreshPlan("refresh", s)
		h.Destroy("destroy", s)
	})

	addAlertScenario("logs-ratio-group-by-for", func(h *grHarness) {
		name := base + "groupbyfor"
		by := j{"group_by": l{"coralogix.metadata.applicationName"}}
		s := h.Apply("create with group_by and group_by_for omitted", h.null(), alertLARatio(name, alertLARatioBody(), by))
		s = h.RefreshPlan("refresh", s)
		for _, v := range []string{"Both", "Numerator Only", "Denominator Only"} {
			s = h.Apply("group_by_for "+v, s, alertLARatio(name, alertLARatioBody(j{"group_by_for": v}), by))
			s = h.RefreshPlan("refresh "+v, s)
		}
		s = h.Apply("group_by_for removed", s, alertLARatio(name, alertLARatioBody(), by))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("group_by removed", s, alertLARatio(name, alertLARatioBody()))
		s = h.RefreshPlan("refresh", s)
		h.Destroy("destroy", s)
	})

	addAlertScenario("logs-ratio-ignore-infinity", func(h *grHarness) {
		name := base + "infinity"
		s := h.Apply("create with ignore_infinity omitted", h.null(), alertLARatio(name, alertLARatioBody()))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: true", s, alertLARatio(name, alertLARatioBody(j{"ignore_infinity": true})))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: false", s, alertLARatio(name, alertLARatioBody(j{"ignore_infinity": false})))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: true again", s, alertLARatio(name, alertLARatioBody(j{"ignore_infinity": true})))
		s = h.Apply("update: removed", s, alertLARatio(name, alertLARatioBody()))
		s = h.RefreshPlan("refresh", s)
		h.Destroy("destroy", s)
	})

	addAlertScenario("logs-ratio-undetected-values", func(h *grHarness) {
		name := base + "undetected"
		less := l{alertLARule("LESS_THAN", 0.1, "10_MINUTES", "")}
		more := l{alertLARule("MORE_THAN", 0.1, "10_MINUTES", "")}
		s := h.Apply("create with undetected_values_management omitted", h.null(), alertLARatio(name, alertLARatioBody(j{"rules": more})))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: trigger on a LESS_THAN rule", s, alertLARatio(name, alertLARatioBody(j{"rules": less, "undetected_values_management": j{"trigger_undetected_values": true, "auto_retire_timeframe": "NEVER"}})))
		s = h.RefreshPlan("refresh", s)
		for _, tf := range alertLARetireTimeframes {
			s = h.Apply("auto_retire_timeframe "+tf, s, alertLARatio(name, alertLARatioBody(j{"rules": less, "undetected_values_management": j{"trigger_undetected_values": true, "auto_retire_timeframe": tf}})))
			s = h.RefreshPlan("refresh "+tf, s)
		}
		s = h.Apply("only trigger_undetected_values", s, alertLARatio(name, alertLARatioBody(j{"rules": less, "undetected_values_management": j{"trigger_undetected_values": true}})))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("only auto_retire_timeframe", s, alertLARatio(name, alertLARatioBody(j{"rules": less, "undetected_values_management": j{"auto_retire_timeframe": "2_HOURS"}})))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("empty undetected_values_management", s, alertLARatio(name, alertLARatioBody(j{"rules": less, "undetected_values_management": j{}})))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("set again", s, alertLARatio(name, alertLARatioBody(j{"rules": less, "undetected_values_management": j{"trigger_undetected_values": true, "auto_retire_timeframe": "1_HOUR"}})))
		s = h.Apply("removed on update", s, alertLARatio(name, alertLARatioBody(j{"rules": less})))
		s = h.RefreshPlan("refresh", s)
		h.Destroy("destroy", s)
	})

	addAlertScenario("logs-ratio-undetected-values-errors", func(h *grHarness) {
		name := base + "undetected-err"
		more := j{"rules": l{alertLARule("MORE_THAN", 0.1, "10_MINUTES", "")}}
		bad := alertLAMerge(more, j{"undetected_values_management": j{"trigger_undetected_values": true, "auto_retire_timeframe": "1_HOUR"}})
		h.Apply("create: trigger on a MORE_THAN rule", h.null(), alertLARatio(name, alertLARatioBody(bad)))
		s := h.Apply("create", h.null(), alertLARatio(name, alertLARatioBody(more)))
		s = h.Apply("update: trigger on a MORE_THAN rule", s, alertLARatio(name, alertLARatioBody(bad)))
		s = h.RefreshPlan("refresh after the failed update", s)
		h.Destroy("destroy", s)
	})

	addAlertScenario("logs-ratio-override-priority", func(h *grHarness) {
		name := base + "override"
		s := h.null()
		for _, p := range alertLAPriorities {
			s = h.Apply("override.priority "+p, s, alertLARatio(name, alertLARatioBody(j{"rules": l{alertLARule("MORE_THAN", 0.5, "10_MINUTES", p)}})))
			s = h.RefreshPlan("refresh "+p, s)
		}
		rule := l{alertLARule("MORE_THAN", 0.5, "10_MINUTES", "")}
		s = h.Apply("override.priority removed, no top-level priority: falls back to P5", s, alertLARatio(name, alertLARatioBody(j{"rules": rule})))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("override.priority removed, top-level priority P2: falls back to P2", s, alertLARatio(name, alertLARatioBody(j{"rules": rule}), j{"priority": "P2"}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("top-level priority changed to P1: the override follows", s, alertLARatio(name, alertLARatioBody(j{"rules": rule}), j{"priority": "P1"}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("top-level priority removed", s, alertLARatio(name, alertLARatioBody(j{"rules": rule})))
		h.RefreshPlan("refresh", s)
	})

	addAlertScenario("logs-ratio-deprecated-priority", func(h *grHarness) {
		name := base + "priority"
		cfg := func(p string) j {
			top := j{}
			if p != "" {
				top["priority"] = p
			}
			return alertLARatio(name, alertLARatioBody(), top)
		}
		h.Validate("priority on a ratio alert warns", cfg("P3"))
		s := h.null()
		for _, p := range alertLAPriorities {
			s = h.Apply("priority "+p, s, cfg(p))
			s = h.RefreshPlan("refresh "+p, s)
		}
		s = h.Apply("priority removed on update", s, cfg(""))
		s = h.RefreshPlan("refresh", s)
		h.Destroy("destroy", s)
	})

	addAlertScenario("logs-ratio-evaluation-delay", func(h *grHarness) {
		name := base + "delay"
		s := h.Apply("create with custom_evaluation_delay omitted", h.null(), alertLARatio(name, alertLARatioBody()))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: 0", s, alertLARatio(name, alertLARatioBody(j{"custom_evaluation_delay": float64(0)})))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: 60000", s, alertLARatio(name, alertLARatioBody(j{"custom_evaluation_delay": float64(60000)})))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: removed", s, alertLARatio(name, alertLARatioBody()))
		s = h.RefreshPlan("refresh", s)
		h.Destroy("destroy", s)
	})

	addAlertScenario("logs-ratio-group-by", func(h *grHarness) {
		// Released behavior, recorded: an explicit empty group_by reads back as null (inconsistent result).
		name := base + "groupby"
		body := alertLARatioBody()
		s := h.Apply("create with group_by omitted", h.null(), alertLARatio(name, body))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: one key", s, alertLARatio(name, body, j{"group_by": l{"coralogix.metadata.applicationName"}}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: two keys", s, alertLARatio(name, body, j{"group_by": l{"coralogix.metadata.applicationName", "coralogix.metadata.subsystemName"}}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: removed by omission", s, alertLARatio(name, body))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: explicit empty list", s, alertLARatio(name, body, j{"group_by": l{}}))
		s = h.RefreshPlan("refresh", s)
		h.Destroy("destroy", s)
	})

	addAlertScenario("logs-ratio-rules", func(h *grHarness) {
		name := base + "rules"
		r1 := alertLARule("MORE_THAN", 0.1, "10_MINUTES", "P3")
		r2 := alertLARule("MORE_THAN", 0.5, "10_MINUTES", "P1")
		r3 := alertLARule("MORE_THAN", 0.3, "10_MINUTES", "P2")
		s := h.Apply("create with one rule", h.null(), alertLARatio(name, alertLARatioBody(j{"rules": l{r1}})))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: two rules", s, alertLARatio(name, alertLARatioBody(j{"rules": l{r1, r2}})))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: three rules", s, alertLARatio(name, alertLARatioBody(j{"rules": l{r1, r2, r3}})))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: two rules, the middle one removed", s, alertLARatio(name, alertLARatioBody(j{"rules": l{r1, r2}})))
		s = h.Apply("update: change a threshold in one rule", s, alertLARatio(name, alertLARatioBody(j{"rules": l{r1, alertLARule("MORE_THAN", 0.9, "10_MINUTES", "P1")}})))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: MORE_THAN and LESS_THAN rules", s, alertLARatio(name, alertLARatioBody(j{"rules": l{r1, alertLARule("LESS_THAN", 0.05, "10_MINUTES", "P2")}})))
		s = h.RefreshPlan("refresh", s)
		h.Destroy("destroy", s)
	})

	addAlertScenario("logs-ratio-payload-filter", func(h *grHarness) {
		// Released behavior, recorded: an explicit empty notification_payload_filter reads back as null
		// (inconsistent result, replan not empty).
		name := base + "payload"
		s := h.Apply("create with a key", h.null(), alertLARatio(name, alertLARatioBody(j{"notification_payload_filter": l{"coralogix.metadata.applicationName"}})))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: other keys", s, alertLARatio(name, alertLARatioBody(j{"notification_payload_filter": l{"coralogix.metadata.severity", "coralogix.metadata.subsystemName"}})))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: explicit empty", s, alertLARatio(name, alertLARatioBody(j{"notification_payload_filter": l{}})))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: removed by omission", s, alertLARatio(name, alertLARatioBody()))
		h.RefreshPlan("refresh", s)
	})

	addAlertScenario("logs-ratio-config-validation", func(h *grHarness) {
		ok := alertLARatioBody()
		oneRule := func(rule j) j { return alertLARatio("v", alertLARatioBody(j{"rules": l{rule}})) }
		by := j{"group_by": l{"coralogix.metadata.applicationName"}}
		h.Validate("valid", alertLARatio("v", ok))
		h.Validate("numerator_alias omitted", alertLARatio("v", j{"denominator_alias": "all", "rules": ok["rules"]}))
		h.Validate("denominator_alias omitted", alertLARatio("v", j{"numerator_alias": "errors", "rules": ok["rules"]}))
		h.Validate("rules omitted", alertLARatio("v", j{"numerator_alias": "errors", "denominator_alias": "all"}))
		h.Validate("rules empty", alertLARatio("v", alertLARatioBody(j{"rules": l{}})))
		h.Validate("condition_type EQUALS", oneRule(alertLARule("EQUALS", 1, "10_MINUTES", "")))
		h.Validate("condition_type NOT_EQUALS", oneRule(alertLARule("NOT_EQUALS", 1, "10_MINUTES", "")))
		h.Validate("condition_type in lower case", oneRule(alertLARule("more_than", 1, "10_MINUTES", "")))
		h.Validate("time_window 20_MINUTES is not allowed for a ratio alert", oneRule(alertLARule("MORE_THAN", 1, "20_MINUTES", "")))
		h.Validate("time_window 7_MINUTES", oneRule(alertLARule("MORE_THAN", 1, "7_MINUTES", "")))
		h.Validate("override.priority P6", oneRule(alertLARule("MORE_THAN", 1, "10_MINUTES", "P6")))
		h.Validate("rule without override", oneRule(j{"condition": j{"condition_type": "MORE_THAN", "threshold": float64(1), "time_window": "10_MINUTES"}}))
		h.Validate("rule without condition", oneRule(j{"override": j{}}))
		h.Validate("group_by_for without group_by", alertLARatio("v", alertLARatioBody(j{"group_by_for": "Both"})))
		h.Validate("group_by_for with an empty group_by", alertLARatio("v", alertLARatioBody(j{"group_by_for": "Both"}), j{"group_by": l{}}))
		h.Validate("group_by_for with group_by", alertLARatio("v", alertLARatioBody(j{"group_by_for": "Both"}), by))
		h.Validate("group_by without group_by_for", alertLARatio("v", ok, by))
		h.Validate("group_by_for with an unknown group_by", alertLARatio("v", alertLARatioBody(j{"group_by_for": "Both"}), j{"group_by": unknownJSON{}}))
		h.Validate("unknown group_by_for without group_by", alertLARatio("v", alertLARatioBody(j{"group_by_for": unknownJSON{}})))
		h.Validate("group_by_for Numerator", alertLARatio("v", alertLARatioBody(j{"group_by_for": "Numerator"}), by))
		h.Validate("group_by_for in lower case", alertLARatio("v", alertLARatioBody(j{"group_by_for": "both"}), by))
		h.Validate("group_by_for empty", alertLARatio("v", alertLARatioBody(j{"group_by_for": ""}), by))
		h.Validate("auto_retire_timeframe 20_MINUTES", alertLARatio("v", alertLARatioBody(j{"undetected_values_management": j{"auto_retire_timeframe": "20_MINUTES"}})))
		h.Validate("custom_evaluation_delay negative", alertLARatio("v", alertLARatioBody(j{"custom_evaluation_delay": float64(-1)})))
		h.Validate("priority warns", alertLARatio("v", ok, j{"priority": "P2"}))
		h.Validate("numerator operation IS_NOT", alertLARatio("v", alertLARatioBody(j{"numerator": alertLAFilter("q", alertLALabels("a", "", "IS_NOT", ""))})))
		h.Validate("denominator severity Fatal", alertLARatio("v", alertLARatioBody(j{"denominator": alertLAFilter("q", j{"severities": l{"Fatal"}})})))
		h.Validate("numerator value null", alertLARatio("v", alertLARatioBody(j{"numerator": alertLAFilter("q", j{"application_name": l{j{"operation": "IS"}}})})))
	})
}

// --- what the real API answers, as recorded ---

// alertLAIssue renders one validation issue as the API writes it: the issues of a typed request
// come as an indented JSON array. The fields come in the order of the recording, then the path.
func alertLAIssue(path []any, fields ...any) string {
	var b strings.Builder
	b.WriteString("[\n  {\n")
	for i := 0; i < len(fields); i += 2 {
		value, _ := json.Marshal(fields[i+1])
		fmt.Fprintf(&b, "    %q: %s,\n", fields[i], value)
	}
	b.WriteString("    \"path\": [\n")
	for i, p := range path {
		value, _ := json.Marshal(p)
		fmt.Fprintf(&b, "      %s", value)
		if i < len(path)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString("    ]\n  }\n]")
	return b.String()
}

var alertLAThresholdArms = []string{"logsThreshold", "logsRatioThreshold"}

func alertLAObj(m map[string]any, key string) map[string]any {
	out, _ := m[key].(map[string]any)
	return out
}

// alertLAMinutes reads the length of a time window from the name of its enum value.
func alertLAMinutes(name string) int {
	_, rest, ok := strings.Cut(name, "VALUE_")
	if !ok {
		return 0
	}
	switch {
	case strings.HasPrefix(rest, "MINUTES_"):
		n, _ := strconv.Atoi(strings.TrimPrefix(strings.TrimSuffix(rest, "_OR_UNSPECIFIED"), "MINUTES_"))
		return n
	case strings.HasPrefix(rest, "HOUR_"):
		n, _ := strconv.Atoi(strings.TrimPrefix(rest, "HOUR_"))
		return n * 60
	case strings.HasPrefix(rest, "HOURS_"):
		n, _ := strconv.Atoi(strings.TrimPrefix(rest, "HOURS_"))
		return n * 60
	}
	return 0
}

// alertLAWindowGroup is the time group of the API: [1 minute - 30 minutes], (30 minutes - 12
// hours), [12 hours - 36 hours].
func alertLAWindowGroup(minutes int) int {
	switch {
	case minutes <= 30:
		return 0
	case minutes < 12*60:
		return 1
	}
	return 2
}

// alertLACheck is the validation of the threshold and ratio alerts that the recordings show.
func alertLACheck(props map[string]any) string {
	if filter := alertLAObj(alertLAObj(props, "logsImmediate"), "logsFilter"); filter != nil {
		if msg := alertLALucene(alertLAObj(filter, "simpleFilter")); msg != "" {
			return msg
		}
	}
	for _, arm := range alertLAThresholdArms {
		body := alertLAObj(props, arm)
		if body == nil {
			continue
		}
		if msg := alertLACheckArm(body, []any{"alertDefProperties", "typeDefinition", arm}); msg != "" {
			return msg
		}
	}
	return ""
}

// alertLARuleSet is what the rules of one threshold arm add up to: the condition types, the
// time-window groups and the priorities per condition type.
type alertLARuleSet struct {
	types      map[string]string
	groups     map[int]bool
	priorities map[string][]string
}

// alertLACheckArm validates one threshold or ratio arm.
func alertLACheckArm(body map[string]any, path []any) string {
	for _, side := range []string{"logsFilter", "numerator", "denominator"} {
		if msg := alertLALucene(alertLAObj(alertLAObj(body, side), "simpleFilter")); msg != "" {
			return msg
		}
	}
	set, msg := alertLAScanRules(body, path)
	if msg != "" {
		return msg
	}
	rulesPath := append(append([]any{}, path...), "rules")
	seen := map[string]string{}
	for kind, list := range set.priorities {
		for _, p := range list {
			if other, dup := seen[p]; dup && other != kind {
				return alertLAIssue(rulesPath, "code", "custom", "message", "Different condition rules cannot have the same priority", "fatal", true)
			}
			seen[p] = kind
		}
	}
	if len(set.types) > 1 {
		return alertLAIssue(rulesPath, "code", "custom", "message", "Cannot mix thresholds less/more than", "fatal", true)
	}
	if len(set.groups) > 1 {
		return alertLAIssue(rulesPath, "code", "custom", "message",
			"Time window is not of the same time group interval ([1 minute - 30 minutes], (30 minutes - 12 hours), [12 hours - 36 hours])", "fatal", true)
	}
	return alertLACheckArmSettings(body, path, set)
}

// alertLAScanRules reads the rules of an arm and returns the first per-rule validation failure.
func alertLAScanRules(body map[string]any, path []any) (alertLARuleSet, string) {
	set := alertLARuleSet{types: map[string]string{}, groups: map[int]bool{}, priorities: map[string][]string{}}
	rules, _ := body["rules"].([]any)
	for i, raw := range rules {
		rule, _ := raw.(map[string]any)
		condition := alertLAObj(rule, "condition")
		if threshold, ok := condition["threshold"].(float64); ok && threshold < 0 {
			return set, alertLAIssue(append(append([]any{}, path...), "rules", i, "condition", "threshold"),
				"code", "too_small", "minimum", 0, "type", "number", "inclusive", true, "exact", false,
				"message", "Number must be greater than or equal to 0")
		}
		kind, _ := condition["conditionType"].(string)
		key := kind
		if at := strings.LastIndex(kind, "TYPE_"); at >= 0 {
			key = kind[at+5:]
		}
		set.types[key] = kind
		priority, _ := alertLAObj(rule, "override")["priority"].(string)
		set.priorities[kind] = append(set.priorities[kind], priority)
		window := alertLAObj(condition, "timeWindow")
		for _, v := range window {
			name, _ := v.(string)
			set.groups[alertLAWindowGroup(alertLAMinutes(name))] = true
		}
	}
	return set, ""
}

// alertLACheckArmSettings validates the undetected values management and the no-data policy.
func alertLACheckArmSettings(body map[string]any, path []any, set alertLARuleSet) string {
	if undetected := alertLAObj(body, "undetectedValuesManagement"); undetected != nil && undetected["triggerUndetectedValues"] == true {
		if _, less := set.types["LESS_THAN"]; !less {
			return alertLAIssue(path, "code", "custom", "message",
				"Cannot set undetected values management for alert that doesnt have at least one rule with less than as a condition")
		}
	}
	if policy := alertLAObj(body, "noDataPolicy"); policy != nil {
		if seconds, ok := policy["autoRetireSeconds"].(float64); ok && int(seconds)%60 != 0 {
			return alertLAIssue(append(append([]any{}, path...), "noDataPolicy", "autoRetireSeconds"),
				"code", "not_multiple_of", "multipleOf", 60, "message", "Number must be a multiple of 60")
		}
	}
	return ""
}

// alertLALucene returns the answer to a lucene query that does not parse. The recording has an
// unbalanced parenthesis only.
func alertLALucene(simple map[string]any) string {
	query, _ := simple["luceneQuery"].(string)
	if strings.Count(query, "(") == strings.Count(query, ")") {
		return ""
	}
	return fmt.Sprintf("Lucene query validation failed at line 1, column 1, token (: Syntax Error, cannot parse %s:  ", query)
}

// alertLAFill adds what the server writes although the request has none: the logs filter with an
// empty simple filter, and the label filters of every simple filter.
func alertLAFill(props map[string]any) {
	for _, arm := range []string{"logsImmediate", "logsThreshold"} {
		if body := alertLAObj(props, arm); body != nil {
			alertLAFillFilter(body, "logsFilter")
		}
	}
	if body := alertLAObj(props, "logsRatioThreshold"); body != nil {
		alertLAFillFilter(body, "numerator")
		alertLAFillFilter(body, "denominator")
	}
	// Recorded: an alert without a LESS_THAN rule reads back without undetectedValuesManagement,
	// whatever the request holds.
	for _, arm := range alertLAThresholdArms {
		body := alertLAObj(props, arm)
		if body == nil {
			continue
		}
		less := false
		rules, _ := body["rules"].([]any)
		for _, raw := range rules {
			rule, _ := raw.(map[string]any)
			kind, _ := alertLAObj(rule, "condition")["conditionType"].(string)
			less = less || strings.HasSuffix(kind, "_LESS_THAN")
		}
		if !less {
			delete(body, "undetectedValuesManagement")
		}
	}
}

func alertLAFillFilter(body map[string]any, key string) {
	filter := alertLAObj(body, key)
	if filter == nil {
		filter = map[string]any{}
		body[key] = filter
	}
	simple := alertLAObj(filter, "simpleFilter")
	if simple == nil {
		simple = map[string]any{}
		filter["simpleFilter"] = simple
	}
	setDefault(simple, "labelFilters", map[string]any{})
	// Recorded: an empty lucene query is dropped.
	if query, ok := simple["luceneQuery"].(string); ok && query == "" {
		delete(simple, "luceneQuery")
	}
}
