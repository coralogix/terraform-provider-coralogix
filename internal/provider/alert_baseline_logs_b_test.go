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
	"math"
	"net/http"
	"strconv"
)

// Baseline scenarios of the logs alert types logs_anomaly, logs_new_value, logs_unique_count, and
// logs_time_relative_threshold. They record the released behavior of the handwritten
// coralogix_alert, including behavior that looks like a bug. The cross-cutting attributes
// (notification_group, incidents_settings, schedule, case_settings, access_policy, labels) use
// their defaults here.

// albAlert is an alert with one type definition arm and the defaults of everything else.
func albAlert(name, arm string, body j, extra ...j) j {
	out := j{"name": name, "type_definition": j{arm: body}}
	for _, e := range extra {
		out = with(out, e)
	}
	return out
}

// albFilterFull is a logs filter with every attribute of the simple filter.
var albFilterFull = j{"simple_filter": j{
	"lucene_query": "message:\"tfbaseline\"",
	"label_filters": j{
		"application_name": l{j{"operation": "IS", "value": "app-a"}, j{"operation": "STARTS_WITH", "value": "app-"}},
		"subsystem_name":   l{j{"operation": "ENDS_WITH", "value": "-sub"}, j{"operation": "INCLUDES", "value": "ub"}},
		"severities":       l{"Warning", "Error"},
	},
}}

var albFilterLucene = j{"simple_filter": j{"lucene_query": "tfbaseline"}}

// albPayload is a notification payload filter.
var albPayload = l{"coralogix.metadata.sdkId", "coralogix.metadata.sdkName"}

// albLogsWindows are the time windows of logs_anomaly.
var albLogsWindows = []string{
	"5_MINUTES", "10_MINUTES", "15_MINUTES", "20_MINUTES", "30_MINUTES",
	"1_HOUR", "2_HOURS", "4_HOURS", "6_HOURS", "12_HOURS", "24_HOURS", "36_HOURS",
}

// --- what the real API does with these types (alert_baseline_fidelity recordings) ---

// albJSONPath writes a path of the validation errors of the API, one element per line. An
// element is a string or an index.
func albJSONPath(path []any) string {
	out := "["
	for i, p := range path {
		if i > 0 {
			out += ","
		}
		if name, ok := p.(string); ok {
			out += "\n      \"" + name + "\""
		} else {
			out += "\n      " + strconv.Itoa(p.(int))
		}
	}
	return out + "\n    ]"
}

// albZod is the text of one validation error of the API, which is the JSON that its validator
// returns, indented by two spaces. limitKey is "minimum" or "maximum".
func albZod(code, limitKey string, limit int, typ string, exact bool, message string, path ...any) string {
	return "[\n  {\n    \"code\": \"" + code + "\",\n    \"" + limitKey + "\": " + strconv.Itoa(limit) + ",\n    \"type\": \"" + typ + "\",\n    \"inclusive\": true,\n    \"exact\": " + strconv.FormatBool(exact) + ",\n    \"message\": \"" + message + "\",\n    \"path\": " + albJSONPath(path) + "\n  }\n]"
}

// albCustom is the text of one custom validation error of the API. The order of the keys is
// the one the recordings show for each rule: "mpf" is message, path, fatal; "mfp" is message,
// fatal, path; "mp" has no fatal.
func albCustom(order, message string, path ...any) string {
	out := "[\n  {\n    \"code\": \"custom\",\n    \"message\": \"" + message + "\""
	switch order {
	case "mpf":
		out += ",\n    \"path\": " + albJSONPath(path) + ",\n    \"fatal\": true"
	case "mfp":
		out += ",\n    \"fatal\": true,\n    \"path\": " + albJSONPath(path)
	default:
		out += ",\n    \"path\": " + albJSONPath(path)
	}
	return out + "\n  }\n]"
}

func albPath(arm string, rest ...any) []any {
	return append([]any{"alertDefProperties", "typeDefinition", arm}, rest...)
}

func albNumber(m map[string]any, key string) (float64, bool) {
	v, ok := m[key].(float64)
	return v, ok
}

// albArmOf returns the type definition arm of the stored properties, or nil.
func albArmOf(props map[string]any, arm string) map[string]any {
	m, _ := props[arm].(map[string]any)
	return m
}

// albFillFilter is what the server does to the logs filter of the four arms: a missing filter
// becomes a simple filter with label filters, and an empty lucene query is dropped.
func albFillFilter(arm map[string]any) {
	filter, _ := arm["logsFilter"].(map[string]any)
	if filter == nil {
		filter = map[string]any{}
		arm["logsFilter"] = filter
	}
	simple, _ := filter["simpleFilter"].(map[string]any)
	if simple == nil {
		simple = map[string]any{}
		filter["simpleFilter"] = simple
	}
	setDefault(simple, "labelFilters", map[string]any{})
	if q, ok := simple["luceneQuery"].(string); ok && q == "" {
		delete(simple, "luceneQuery")
	}
}

// albCheckEvaluationDelay is the limit of custom_evaluation_delay.
func albCheckEvaluationDelay(arm map[string]any, name string) string {
	if v, ok := albNumber(arm, "evaluationDelayMs"); ok && v > 10800000 {
		return albZod("too_big", "maximum", 10800000, "number", false, "Number must be less than or equal to 10800000", albPath(name, "evaluationDelayMs")...)
	}
	return ""
}

// albCheckSingleRule is the limit of the arms that take exactly one rule.
func albCheckSingleRule(arm map[string]any, name string) string {
	if rules, _ := arm["rules"].([]any); len(rules) > 1 {
		return albZod("too_big", "maximum", 1, "array", true, "Array must contain exactly 1 element(s)", albPath(name, "rules")...)
	}
	return ""
}

func init() {
	addAlertFill(albFillLogsB)
	addAlertCheck(albCheckLogsB)
	addAlertStatusCheck(albStatusUniqueCount)
}

// albFillLogsB applies the recorded server-side normalisation of the logs alert arms of this file.
func albFillLogsB(props map[string]any) {
	for _, name := range []string{"logsAnomaly", "logsNewValue", "logsUniqueCount", "logsTimeRelativeThreshold"} {
		if arm := albArmOf(props, name); arm != nil {
			albFillFilter(arm)
		}
	}
	albFillNewValue(props)
	albFillTimeRelative(props)
	albFillUniqueCount(props)
	albFillAnomaly(props)
}

// Recorded: the API derives the group keys of a logs_new_value alert from the keypaths of
// its rule.
func albFillNewValue(props map[string]any) {
	arm := albArmOf(props, "logsNewValue")
	if arm == nil {
		return
	}
	keys := []any{}
	rules, _ := arm["rules"].([]any)
	for _, r := range rules {
		rule, _ := r.(map[string]any)
		cond, _ := rule["condition"].(map[string]any)
		if k, ok := cond["keypathToTrack"].(string); ok && k != "" {
			keys = append(keys, k)
		}
	}
	props["groupByKeys"] = keys
}

// Recorded: the API stores the undetected values management of a time-relative alert only
// when a rule has the condition LESS_THAN.
func albFillTimeRelative(props map[string]any) {
	arm := albArmOf(props, "logsTimeRelativeThreshold")
	if arm == nil {
		return
	}
	less := false
	rules, _ := arm["rules"].([]any)
	for _, r := range rules {
		rule, _ := r.(map[string]any)
		cond, _ := rule["condition"].(map[string]any)
		if cond["conditionType"] == "LOGS_TIME_RELATIVE_CONDITION_TYPE_LESS_THAN" {
			less = true
		}
	}
	if !less {
		delete(arm, "undetectedValuesManagement")
	}
}

// Recorded: max_unique_count is stored as an int32; a larger value reads back as the maximum.
func albFillUniqueCount(props map[string]any) {
	arm := albArmOf(props, "logsUniqueCount")
	if arm == nil {
		return
	}
	rules, _ := arm["rules"].([]any)
	for _, r := range rules {
		rule, _ := r.(map[string]any)
		cond, _ := rule["condition"].(map[string]any)
		if text, ok := cond["maxUniqueCount"].(string); ok {
			if v, err := strconv.ParseFloat(text, 64); err == nil && v > 2147483647 {
				cond["maxUniqueCount"] = "2147483647"
			}
		}
	}
}

func albFillAnomaly(props map[string]any) {
	arm := albArmOf(props, "logsAnomaly")
	if arm == nil {
		return
	}
	setDefault(arm, "anomalyAlertSettings", map[string]any{})
	// Recorded: the API stores minimumThreshold of logs_anomaly as an integer. 7.5 reads back 7.
	rules, _ := arm["rules"].([]any)
	for _, r := range rules {
		rule, _ := r.(map[string]any)
		cond, _ := rule["condition"].(map[string]any)
		if v, ok := cond["minimumThreshold"].(float64); ok {
			cond["minimumThreshold"] = math.Trunc(v)
		}
	}
}

func albCheckLogsB(props map[string]any) string {
	if arm := albArmOf(props, "logsAnomaly"); arm != nil {
		return albCheckAnomaly(arm)
	}
	if arm := albArmOf(props, "logsNewValue"); arm != nil {
		return albCheckSingleRule(arm, "logsNewValue")
	}
	if arm := albArmOf(props, "logsUniqueCount"); arm != nil {
		return albCheckUniqueCount(props, arm)
	}
	if arm := albArmOf(props, "logsTimeRelativeThreshold"); arm != nil {
		return albCheckTimeRelative(arm)
	}
	return ""
}

func albCheckAnomaly(arm map[string]any) string {
	if msg := albCheckEvaluationDelay(arm, "logsAnomaly"); msg != "" {
		return msg
	}
	if settings, _ := arm["anomalyAlertSettings"].(map[string]any); settings != nil {
		if v, ok := albNumber(settings, "percentageOfDeviation"); ok && v < 0 {
			return albZod("too_small", "minimum", 0, "number", false, "Number must be greater than or equal to 0", albPath("logsAnomaly", "anomalyAlertSettings", "percentageOfDeviation")...)
		}
	}
	return albCheckSingleRule(arm, "logsAnomaly")
}

// Recorded: a value above Number.MAX_SAFE_INTEGER is a server error, not a validation error.
func albStatusUniqueCount(props map[string]any) (int, any) {
	arm := albArmOf(props, "logsUniqueCount")
	if arm == nil {
		return 0, nil
	}
	if text, ok := arm["maxUniqueCountPerGroupByKey"].(string); ok {
		if v, err := strconv.ParseFloat(text, 64); err == nil && v > 9007199254740991 {
			return http.StatusInternalServerError, map[string]any{"code": 500, "message": "Internal Server Error: Error deserializing request: Value is larger than Number.MAX_SAFE_INTEGER"}
		}
	}
	return 0, nil
}

func albCheckUniqueCount(props, arm map[string]any) string {
	if keypath, ok := arm["uniqueCountKeypath"].(string); ok && keypath == "" {
		return albZod("too_small", "minimum", 1, "string", false, "String must contain at least 1 character(s)", albPath("logsUniqueCount", "uniqueCountKeypath")...)
	}
	if msg := albCheckSingleRule(arm, "logsUniqueCount"); msg != "" {
		return msg
	}
	rules, _ := arm["rules"].([]any)
	for i, r := range rules {
		rule, _ := r.(map[string]any)
		cond, _ := rule["condition"].(map[string]any)
		if text, ok := cond["maxUniqueCount"].(string); ok {
			if v, err := strconv.ParseFloat(text, 64); err == nil && v < 0 {
				return albZod("too_small", "minimum", 0, "number", false, "Number must be greater than or equal to 0", albPath("logsUniqueCount", "rules", i, "condition", "maxUniqueCount")...)
			}
		}
	}
	groupBy, _ := props["groupByKeys"].([]any)
	perKey, hasPerKey := arm["maxUniqueCountPerGroupByKey"].(string)
	switch {
	case len(groupBy) > 0 && !hasPerKey:
		return albCustom("mpf", "Logs Unique Count alerts with group-by keys must also have maxUniqueCountPerGroupByKey", albPath("logsUniqueCount", "maxUniqueCountPerGroupByKey")...)
	case len(groupBy) == 0 && hasPerKey:
		return albCustom("mpf", "Logs Unique Count alerts cannot set maxUniqueCountPerGroupByKey without specifying group-by keys.", albPath("logsUniqueCount", "maxUniqueCountPerGroupByKey")...)
	}
	if v, err := strconv.ParseFloat(perKey, 64); hasPerKey && err == nil {
		if v > 1000 {
			return albZod("too_big", "maximum", 1000, "number", false, "Number must be less than or equal to 1000", albPath("logsUniqueCount", "maxUniqueCountPerGroupByKey")...)
		}
		if v < 1 {
			return albZod("too_small", "minimum", 1, "number", false, "Number must be greater than or equal to 1", albPath("logsUniqueCount", "maxUniqueCountPerGroupByKey")...)
		}
	}
	return ""
}

func albCheckTimeRelative(arm map[string]any) string {
	if msg := albCheckEvaluationDelay(arm, "logsTimeRelativeThreshold"); msg != "" {
		return msg
	}
	rules, _ := arm["rules"].([]any)
	less, more := false, false
	priorities := map[string]bool{}
	duplicate := false
	for i, r := range rules {
		rule, _ := r.(map[string]any)
		cond, _ := rule["condition"].(map[string]any)
		if v, ok := albNumber(cond, "threshold"); ok && v < 0 {
			return albZod("too_small", "minimum", 0, "number", false, "Number must be greater than or equal to 0", albPath("logsTimeRelativeThreshold", "rules", i, "condition", "threshold")...)
		}
		if t, _ := cond["conditionType"].(string); t == "LOGS_TIME_RELATIVE_CONDITION_TYPE_LESS_THAN" {
			less = true
		} else {
			more = true
		}
		override, _ := rule["override"].(map[string]any)
		priority, _ := override["priority"].(string)
		if priorities[priority] {
			duplicate = true
		}
		priorities[priority] = true
	}
	if less && more {
		return albCustom("mfp", "Cannot mix thresholds less/more than", albPath("logsTimeRelativeThreshold", "rules")...)
	}
	if duplicate {
		return albCustom("mfp", "Different condition rules cannot have the same priority", albPath("logsTimeRelativeThreshold", "rules")...)
	}
	if undetected, _ := arm["undetectedValuesManagement"].(map[string]any); undetected != nil && !less {
		trigger, _ := undetected["triggerUndetectedValues"].(bool)
		if retire, _ := undetected["autoRetireTimeframe"].(string); trigger || (retire != "" && retire != "AUTO_RETIRE_TIMEFRAME_NEVER_OR_UNSPECIFIED") {
			return albCustom("mp", "Cannot set undetected values management for alert that doesnt have at least one rule with less than as a condition", albPath("logsTimeRelativeThreshold")...)
		}
	}
	return ""
}

// --- logs_anomaly ---

func albAnomaly(extra ...j) j {
	out := j{"rules": l{j{"condition": j{"minimum_threshold": float64(5), "time_window": "10_MINUTES"}}}}
	for _, e := range extra {
		out = with(out, e)
	}
	return out
}

func albAnomalyRule(threshold float64, window string) l {
	return l{j{"condition": j{"minimum_threshold": threshold, "time_window": window}}}
}

func init() {
	addAlertScenario("logs-anomaly-lifecycle", func(h *grHarness) {
		name := "tfbaseline-anomaly-lifecycle"
		s := h.Apply("create with only the required attributes", h.null(), albAlert(name, "logs_anomaly", albAnomaly()))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: change threshold and time window", s, albAlert(name, "logs_anomaly", albAnomaly(j{"rules": albAnomalyRule(7, "1_HOUR")})))
		s = h.RefreshPlan("refresh after update", s)
		imported := h.Import("import by id", idOf(s))
		_ = imported
		h.Destroy("destroy", s)
	})

	addAlertScenario("logs-anomaly-full", func(h *grHarness) {
		name := "tfbaseline-anomaly-full"
		full := albAnomaly(j{
			"logs_filter":                 albFilterFull,
			"notification_payload_filter": albPayload,
			"percentage_of_deviation":     float64(15.5),
			"custom_evaluation_delay":     float64(100),
		})
		s := h.Apply("create with every attribute", h.null(), albAlert(name, "logs_anomaly", full, j{"group_by": l{"coralogix.metadata.applicationName"}}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: change every attribute", s, albAlert(name, "logs_anomaly", albAnomaly(j{
			"logs_filter": j{"simple_filter": j{
				"lucene_query": "message:\"changed\"",
				"label_filters": j{
					"application_name": l{j{"operation": "INCLUDES", "value": "app-b"}},
					"subsystem_name":   l{j{"operation": "IS", "value": "sub-b"}},
					"severities":       l{"Critical"},
				},
			}},
			"notification_payload_filter": l{"coralogix.metadata.sdkVersion"},
			"percentage_of_deviation":     float64(25.25),
			"custom_evaluation_delay":     float64(250),
			"rules":                       albAnomalyRule(20, "2_HOURS"),
		}), j{"group_by": l{"coralogix.metadata.subsystemName", "coralogix.metadata.applicationName"}}))
		s = h.RefreshPlan("refresh after update", s)
		h.Import("import by id", idOf(s))
		h.Destroy("destroy", s)
	})

	addAlertScenario("logs-anomaly-unset-optionals", func(h *grHarness) {
		name := "tfbaseline-anomaly-unset"
		full := albAnomaly(j{
			"logs_filter":                 albFilterFull,
			"notification_payload_filter": albPayload,
			"percentage_of_deviation":     float64(15.5),
			"custom_evaluation_delay":     float64(100),
		})
		s := h.Apply("create with every attribute", h.null(), albAlert(name, "logs_anomaly", full, j{"group_by": l{"coralogix.metadata.applicationName"}}))
		s = h.Apply("update: remove percentage_of_deviation", s, albAlert(name, "logs_anomaly", with(full, j{"percentage_of_deviation": nil}), j{"group_by": l{"coralogix.metadata.applicationName"}}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: remove custom_evaluation_delay", s, albAlert(name, "logs_anomaly", with(full, j{"percentage_of_deviation": nil, "custom_evaluation_delay": nil}), j{"group_by": l{"coralogix.metadata.applicationName"}}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: remove notification_payload_filter", s, albAlert(name, "logs_anomaly", albAnomaly(j{"logs_filter": albFilterFull}), j{"group_by": l{"coralogix.metadata.applicationName"}}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: remove logs_filter and group_by", s, albAlert(name, "logs_anomaly", albAnomaly()))
		s = h.RefreshPlan("refresh", s)
		h.Destroy("destroy", s)
	})

	addAlertScenario("logs-anomaly-time-windows", func(h *grHarness) {
		name := "tfbaseline-anomaly-windows"
		s := h.null()
		for _, w := range albLogsWindows {
			s = h.Apply("time window "+w, s, albAlert(name, "logs_anomaly", albAnomaly(j{"rules": albAnomalyRule(5, w)})))
		}
		h.RefreshPlan("refresh", s)
	})

	addAlertScenario("logs-anomaly-percentage-of-deviation", func(h *grHarness) {
		// The API stores a float32. The values show the round trip of 0.1, which float32 cannot hold.
		name := "tfbaseline-anomaly-percentage"
		s := h.null()
		for _, v := range []float64{0, 0.1, 33.3, 100, 250.5} {
			s = h.Apply("percentage_of_deviation", s, albAlert(name, "logs_anomaly", albAnomaly(j{"percentage_of_deviation": v})))
		}
		h.RefreshPlan("refresh", s)
	})

	addAlertScenario("logs-anomaly-threshold-values", func(h *grHarness) {
		name := "tfbaseline-anomaly-threshold"
		s := h.null()
		for _, v := range []float64{0, 0.5, 1, 1000000} {
			s = h.Apply("minimum_threshold", s, albAlert(name, "logs_anomaly", albAnomaly(j{"rules": albAnomalyRule(v, "10_MINUTES")})))
		}
		h.RefreshPlan("refresh", s)
	})

	addAlertScenario("logs-anomaly-filter-collections", func(h *grHarness) {
		name := "tfbaseline-anomaly-filters"
		withFilter := func(extra j) j {
			return albAlert(name, "logs_anomaly", albAnomaly(j{"logs_filter": j{"simple_filter": with(j{"lucene_query": "tfbaseline"}, extra)}}))
		}
		s := h.Apply("create: no label_filters", h.null(), withFilter(nil))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: empty label_filters", s, withFilter(j{"label_filters": j{}}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: empty severities, application_name, and subsystem_name", s, withFilter(j{"label_filters": j{
			"severities": l{}, "application_name": l{}, "subsystem_name": l{},
		}}))
		s = h.RefreshPlan("refresh", s)
		for _, sev := range []string{"Debug", "Info", "Warning", "Error", "Critical", "Unspecified"} {
			s = h.Apply("severity "+sev, s, withFilter(j{"label_filters": j{"severities": l{sev}}}))
		}
		s = h.RefreshPlan("refresh", s)
		for _, op := range []string{"IS", "INCLUDES", "STARTS_WITH", "ENDS_WITH"} {
			s = h.Apply("operation "+op, s, withFilter(j{"label_filters": j{
				"application_name": l{j{"operation": op, "value": "a"}},
				"subsystem_name":   l{j{"operation": op, "value": "s"}},
			}}))
		}
		s = h.Apply("update: operation omitted", s, withFilter(j{"label_filters": j{
			"application_name": l{j{"value": "a"}}, "subsystem_name": l{j{"value": "s"}},
		}}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: clear the label filters again", s, withFilter(nil))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: logs_filter without lucene_query", s, albAlert(name, "logs_anomaly", albAnomaly(j{"logs_filter": j{"simple_filter": j{
			"label_filters": j{"severities": l{"Error"}},
		}}})))
		s = h.RefreshPlan("refresh", s)
		h.Apply("update: logs_filter with an empty simple_filter", s, albAlert(name, "logs_anomaly", albAnomaly(j{"logs_filter": j{"simple_filter": j{}}})))
	})

	addAlertScenario("logs-anomaly-payload-filter", func(h *grHarness) {
		name := "tfbaseline-anomaly-payload"
		s := h.Apply("create: omitted", h.null(), albAlert(name, "logs_anomaly", albAnomaly()))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: empty list", s, albAlert(name, "logs_anomaly", albAnomaly(j{"notification_payload_filter": l{}})))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: two values", s, albAlert(name, "logs_anomaly", albAnomaly(j{"notification_payload_filter": albPayload})))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: other value", s, albAlert(name, "logs_anomaly", albAnomaly(j{"notification_payload_filter": l{"a.b"}})))
		s = h.Apply("update: cleared", s, albAlert(name, "logs_anomaly", albAnomaly()))
		h.RefreshPlan("refresh", s)
	})

	addAlertScenario("logs-anomaly-custom-evaluation-delay", func(h *grHarness) {
		name := "tfbaseline-anomaly-delay"
		s := h.Apply("create: omitted", h.null(), albAlert(name, "logs_anomaly", albAnomaly()))
		s = h.Apply("update: zero", s, albAlert(name, "logs_anomaly", albAnomaly(j{"custom_evaluation_delay": float64(0)})))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: 60000", s, albAlert(name, "logs_anomaly", albAnomaly(j{"custom_evaluation_delay": float64(60000)})))
		s = h.Apply("update: the maximum of the API", s, albAlert(name, "logs_anomaly", albAnomaly(j{"custom_evaluation_delay": float64(10800000)})))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: removed", s, albAlert(name, "logs_anomaly", albAnomaly()))
		h.RefreshPlan("refresh", s)
	})

	addAlertScenario("logs-anomaly-validation", func(h *grHarness) {
		name := "tfbaseline-anomaly-validation"
		h.Validate("a valid config", albAlert(name, "logs_anomaly", albAnomaly()))
		h.Validate("rules missing", albAlert(name, "logs_anomaly", j{}))
		h.Validate("rules null", albAlert(name, "logs_anomaly", j{"rules": nil}))
		h.Validate("rules empty", albAlert(name, "logs_anomaly", j{"rules": l{}}))
		h.Validate("rule without condition", albAlert(name, "logs_anomaly", j{"rules": l{j{}}}))
		h.Validate("condition without minimum_threshold", albAlert(name, "logs_anomaly", j{"rules": l{j{"condition": j{"time_window": "10_MINUTES"}}}}))
		h.Validate("condition without time_window", albAlert(name, "logs_anomaly", j{"rules": l{j{"condition": j{"minimum_threshold": float64(1)}}}}))
		h.Validate("null minimum_threshold", albAlert(name, "logs_anomaly", j{"rules": l{j{"condition": j{"minimum_threshold": nil, "time_window": "10_MINUTES"}}}}))
		h.Validate("null time_window", albAlert(name, "logs_anomaly", j{"rules": l{j{"condition": j{"minimum_threshold": float64(1), "time_window": nil}}}}))
		h.Validate("time_window of another type", albAlert(name, "logs_anomaly", albAnomaly(j{"rules": albAnomalyRule(1, "1_MINUTE")})))
		h.Validate("time_window in lowercase", albAlert(name, "logs_anomaly", albAnomaly(j{"rules": albAnomalyRule(1, "10_minutes")})))
		h.Validate("condition_type is computed only", albAlert(name, "logs_anomaly", j{"rules": l{j{"condition": j{
			"minimum_threshold": float64(1), "time_window": "10_MINUTES", "condition_type": "MORE_THAN_USUAL",
		}}}}))
		h.Validate("negative custom_evaluation_delay", albAlert(name, "logs_anomaly", albAnomaly(j{"custom_evaluation_delay": float64(-1)})))
		h.Validate("fractional custom_evaluation_delay", albAlert(name, "logs_anomaly", albAnomaly(j{"custom_evaluation_delay": float64(1.5)})))
		h.Validate("unknown severity", albAlert(name, "logs_anomaly", albAnomaly(j{"logs_filter": j{"simple_filter": j{"label_filters": j{"severities": l{"Fatal"}}}}})))
		h.Validate("null severity", albAlert(name, "logs_anomaly", albAnomaly(j{"logs_filter": j{"simple_filter": j{"label_filters": j{"severities": l{nil}}}}})))
		h.Validate("unknown label operation", albAlert(name, "logs_anomaly", albAnomaly(j{"logs_filter": j{"simple_filter": j{"label_filters": j{
			"application_name": l{j{"operation": "EQUALS", "value": "a"}},
		}}}})))
		h.Validate("label filter without value", albAlert(name, "logs_anomaly", albAnomaly(j{"logs_filter": j{"simple_filter": j{"label_filters": j{
			"subsystem_name": l{j{"operation": "IS"}},
		}}}})))
		h.Validate("negative percentage_of_deviation is not validated", albAlert(name, "logs_anomaly", albAnomaly(j{"percentage_of_deviation": float64(-1)})))
		h.Validate("two type definitions", with(albAlert(name, "logs_anomaly", albAnomaly()), j{"type_definition": j{
			"logs_anomaly":   albAnomaly(),
			"logs_immediate": j{"logs_filter": albFilterLucene},
		}}))
		h.Validate("no type definition arm", j{"name": name, "type_definition": j{}})
		h.Validate("group_by with a null member", albAlert(name, "logs_anomaly", albAnomaly(), j{"group_by": l{nil}}))
	})

	addAlertScenario("logs-anomaly-api-errors", func(h *grHarness) {
		name := "tfbaseline-anomaly-errors"
		h.Apply("create: custom_evaluation_delay above the API maximum", h.null(), albAlert(name, "logs_anomaly", albAnomaly(j{"custom_evaluation_delay": float64(10800001)})))
		h.Apply("create: negative percentage_of_deviation", h.null(), albAlert(name, "logs_anomaly", albAnomaly(j{"percentage_of_deviation": float64(-5)})))
		h.Apply("create: two rules", h.null(), albAlert(name, "logs_anomaly", albAnomaly(j{"rules": l{
			j{"condition": j{"minimum_threshold": float64(1), "time_window": "10_MINUTES"}},
			j{"condition": j{"minimum_threshold": float64(2), "time_window": "1_HOUR"}},
		}})))
		s := h.Apply("create: a valid alert", h.null(), albAlert(name, "logs_anomaly", albAnomaly()))
		s = h.Apply("update: custom_evaluation_delay above the API maximum", s, albAlert(name, "logs_anomaly", albAnomaly(j{"custom_evaluation_delay": float64(10800001)})))
		s = h.RefreshPlan("refresh after the failed update", s)
		h.Destroy("destroy", s)
	})

	addAlertScenario("logs-anomaly-group-by", func(h *grHarness) {
		name := "tfbaseline-anomaly-group-by"
		s := h.Apply("create without group_by", h.null(), albAlert(name, "logs_anomaly", albAnomaly()))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: one key", s, albAlert(name, "logs_anomaly", albAnomaly(), j{"group_by": l{"coralogix.metadata.applicationName"}}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: two keys, other order", s, albAlert(name, "logs_anomaly", albAnomaly(), j{"group_by": l{"b.key", "a.key"}}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: empty list", s, albAlert(name, "logs_anomaly", albAnomaly(), j{"group_by": l{}}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: removed", s, albAlert(name, "logs_anomaly", albAnomaly()))
		h.RefreshPlan("refresh", s)
	})
}

// --- logs_new_value ---

var albNewValueWindows = []string{"12_HOURS", "24_HOURS", "48_HOURS", "72_HOURS", "1_WEEK", "1_MONTH", "2_MONTHS", "3_MONTHS"}

func albNewValueRule(keypath, window string) l {
	return l{j{"condition": j{"keypath_to_track": keypath, "time_window": window}}}
}

func albNewValue(extra ...j) j {
	out := j{"rules": albNewValueRule("remote_addr_geoip.country_name", "24_HOURS")}
	for _, e := range extra {
		out = with(out, e)
	}
	return out
}

func init() {
	addAlertScenario("logs-new-value-lifecycle", func(h *grHarness) {
		name := "tfbaseline-new-value-lifecycle"
		s := h.Apply("create with only the required attributes", h.null(), albAlert(name, "logs_new_value", albNewValue()))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: change keypath and time window", s, albAlert(name, "logs_new_value", albNewValue(j{"rules": albNewValueRule("remote_addr_geoip.city_name", "1_WEEK")})))
		s = h.RefreshPlan("refresh after update", s)
		h.Import("import by id", idOf(s))
		h.Destroy("destroy", s)
	})

	addAlertScenario("logs-new-value-full", func(h *grHarness) {
		name := "tfbaseline-new-value-full"
		full := albNewValue(j{"logs_filter": albFilterFull, "notification_payload_filter": albPayload})
		s := h.Apply("create with every attribute", h.null(), albAlert(name, "logs_new_value", full))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: change every attribute", s, albAlert(name, "logs_new_value", albNewValue(j{
			"logs_filter": j{"simple_filter": j{
				"lucene_query": "message:\"changed\"",
				"label_filters": j{
					"application_name": l{j{"operation": "INCLUDES", "value": "app-b"}},
					"subsystem_name":   l{j{"operation": "IS", "value": "sub-b"}},
					"severities":       l{"Critical"},
				},
			}},
			"notification_payload_filter": l{"coralogix.metadata.sdkVersion"},
			"rules":                       albNewValueRule("other.keypath", "2_MONTHS"),
		})))
		s = h.RefreshPlan("refresh after update", s)
		h.Import("import by id", idOf(s))
		h.Destroy("destroy", s)
	})

	addAlertScenario("logs-new-value-unset-optionals", func(h *grHarness) {
		name := "tfbaseline-new-value-unset"
		full := albNewValue(j{"logs_filter": albFilterFull, "notification_payload_filter": albPayload})
		s := h.Apply("create with every attribute", h.null(), albAlert(name, "logs_new_value", full))
		s = h.Apply("update: remove notification_payload_filter", s, albAlert(name, "logs_new_value", albNewValue(j{"logs_filter": albFilterFull})))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: remove logs_filter", s, albAlert(name, "logs_new_value", albNewValue()))
		s = h.RefreshPlan("refresh", s)
		h.Destroy("destroy", s)
	})

	addAlertScenario("logs-new-value-time-windows", func(h *grHarness) {
		name := "tfbaseline-new-value-windows"
		s := h.null()
		for _, w := range albNewValueWindows {
			s = h.Apply("time window "+w, s, albAlert(name, "logs_new_value", albNewValue(j{"rules": albNewValueRule("a.b", w)})))
		}
		h.RefreshPlan("refresh", s)
	})

	addAlertScenario("logs-new-value-keypath", func(h *grHarness) {
		name := "tfbaseline-new-value-keypath"
		s := h.null()
		for _, k := range []string{"a", "a.b.c", "with space", "$d.field"} {
			s = h.Apply("keypath "+k, s, albAlert(name, "logs_new_value", albNewValue(j{"rules": albNewValueRule(k, "12_HOURS")})))
		}
		s = h.RefreshPlan("refresh", s)
		h.Apply("update: empty keypath", s, albAlert(name, "logs_new_value", albNewValue(j{"rules": albNewValueRule("", "12_HOURS")})))
	})

	// group_by of a logs_new_value alert cannot be configured, but the state has it: the API
	// answers the keypaths of the rules as the group keys, or an empty list.
	addAlertScenario("logs-new-value-group-by", func(h *grHarness) {
		name := "tfbaseline-new-value-group-by"
		s := h.Apply("create", h.null(), albAlert(name, "logs_new_value", albNewValue()))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: other keypath, so group_by is planned unknown", s, albAlert(name, "logs_new_value", albNewValue(j{"rules": albNewValueRule("other.key", "24_HOURS")})))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: only the time window changes, so group_by keeps the state", s, albAlert(name, "logs_new_value", albNewValue(j{"rules": albNewValueRule("other.key", "48_HOURS")})))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: description only", s, albAlert(name, "logs_new_value", albNewValue(j{"rules": albNewValueRule("other.key", "48_HOURS")}), j{"description": "d"}))
		h.RefreshPlan("refresh", s)
		h.Import("import by id", idOf(s))
	})

	addAlertScenario("logs-new-value-filter-collections", func(h *grHarness) {
		name := "tfbaseline-new-value-filters"
		withFilter := func(extra j) j {
			return albAlert(name, "logs_new_value", albNewValue(j{"logs_filter": j{"simple_filter": with(j{"lucene_query": "tfbaseline"}, extra)}}))
		}
		s := h.Apply("create: no label_filters", h.null(), withFilter(nil))
		s = h.Apply("update: empty severities, application_name, and subsystem_name", s, withFilter(j{"label_filters": j{
			"severities": l{}, "application_name": l{}, "subsystem_name": l{},
		}}))
		s = h.RefreshPlan("refresh", s)
		for _, sev := range []string{"Debug", "Info", "Warning", "Error", "Critical", "Unspecified"} {
			s = h.Apply("severity "+sev, s, withFilter(j{"label_filters": j{"severities": l{sev}}}))
		}
		for _, op := range []string{"IS", "INCLUDES", "STARTS_WITH", "ENDS_WITH"} {
			s = h.Apply("operation "+op, s, withFilter(j{"label_filters": j{
				"application_name": l{j{"operation": op, "value": "a"}},
				"subsystem_name":   l{j{"operation": op, "value": "s"}},
			}}))
		}
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: clear the label filters", s, withFilter(nil))
		h.RefreshPlan("refresh", s)
	})

	addAlertScenario("logs-new-value-payload-filter", func(h *grHarness) {
		name := "tfbaseline-new-value-payload"
		s := h.Apply("create: omitted", h.null(), albAlert(name, "logs_new_value", albNewValue()))
		s = h.Apply("update: empty list", s, albAlert(name, "logs_new_value", albNewValue(j{"notification_payload_filter": l{}})))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: two values", s, albAlert(name, "logs_new_value", albNewValue(j{"notification_payload_filter": albPayload})))
		s = h.Apply("update: cleared", s, albAlert(name, "logs_new_value", albNewValue()))
		h.RefreshPlan("refresh", s)
	})

	addAlertScenario("logs-new-value-validation", func(h *grHarness) {
		name := "tfbaseline-new-value-validation"
		h.Validate("a valid config", albAlert(name, "logs_new_value", albNewValue()))
		h.Validate("rules missing", albAlert(name, "logs_new_value", j{}))
		h.Validate("rules empty", albAlert(name, "logs_new_value", j{"rules": l{}}))
		h.Validate("rule without condition", albAlert(name, "logs_new_value", j{"rules": l{j{}}}))
		h.Validate("condition without keypath_to_track", albAlert(name, "logs_new_value", j{"rules": l{j{"condition": j{"time_window": "24_HOURS"}}}}))
		h.Validate("condition without time_window", albAlert(name, "logs_new_value", j{"rules": l{j{"condition": j{"keypath_to_track": "a"}}}}))
		h.Validate("null keypath_to_track", albAlert(name, "logs_new_value", j{"rules": l{j{"condition": j{"keypath_to_track": nil, "time_window": "24_HOURS"}}}}))
		h.Validate("null time_window", albAlert(name, "logs_new_value", j{"rules": l{j{"condition": j{"keypath_to_track": "a", "time_window": nil}}}}))
		h.Validate("time_window of another type", albAlert(name, "logs_new_value", albNewValue(j{"rules": albNewValueRule("a", "1_HOUR")})))
		h.Validate("time_window 6_HOURS", albAlert(name, "logs_new_value", albNewValue(j{"rules": albNewValueRule("a", "6_HOURS")})))
		h.Validate("group_by is not allowed", albAlert(name, "logs_new_value", albNewValue(), j{"group_by": l{"a"}}))
		h.Validate("empty group_by is not allowed either", albAlert(name, "logs_new_value", albNewValue(), j{"group_by": l{}}))
		h.Validate("group_by with a null member", albAlert(name, "logs_new_value", albNewValue(), j{"group_by": l{nil}}))
		h.Validate("unknown severity", albAlert(name, "logs_new_value", albNewValue(j{"logs_filter": j{"simple_filter": j{"label_filters": j{"severities": l{"Fatal"}}}}})))
		h.Validate("unknown label operation", albAlert(name, "logs_new_value", albNewValue(j{"logs_filter": j{"simple_filter": j{"label_filters": j{
			"subsystem_name": l{j{"operation": "EQUALS", "value": "a"}},
		}}}})))
		h.Validate("two type definitions", j{"name": name, "type_definition": j{
			"logs_new_value": albNewValue(),
			"logs_anomaly":   albAnomaly(),
		}})
	})

	addAlertScenario("logs-new-value-api-errors", func(h *grHarness) {
		name := "tfbaseline-new-value-errors"
		h.Apply("create: two rules", h.null(), albAlert(name, "logs_new_value", albNewValue(j{"rules": l{
			j{"condition": j{"keypath_to_track": "a", "time_window": "24_HOURS"}},
			j{"condition": j{"keypath_to_track": "b", "time_window": "12_HOURS"}},
		}})))
		s := h.Apply("create: a valid alert", h.null(), albAlert(name, "logs_new_value", albNewValue()))
		s = h.Apply("update: two rules", s, albAlert(name, "logs_new_value", albNewValue(j{"rules": l{
			j{"condition": j{"keypath_to_track": "a", "time_window": "24_HOURS"}},
			j{"condition": j{"keypath_to_track": "b", "time_window": "12_HOURS"}},
		}})))
		s = h.RefreshPlan("refresh after the failed update", s)
		h.Destroy("destroy", s)
	})
}

// --- logs_unique_count ---

var albUniqueWindows = []string{
	"1_MINUTE", "5_MINUTES", "10_MINUTES", "15_MINUTES", "20_MINUTES", "30_MINUTES",
	"1_HOUR", "2_HOURS", "4_HOURS", "6_HOURS", "12_HOURS", "24_HOURS", "36_HOURS",
}

func albUniqueRule(max float64, window string) l {
	return l{j{"condition": j{"max_unique_count": max, "time_window": window}}}
}

func albUnique(extra ...j) j {
	out := j{"unique_count_keypath": "remote_addr_geoip.country_name", "rules": albUniqueRule(2, "5_MINUTES")}
	for _, e := range extra {
		out = with(out, e)
	}
	return out
}

var albGroupBy = j{"group_by": l{"remote_addr_geoip.city_name"}}

func init() {
	addAlertScenario("logs-unique-count-lifecycle", func(h *grHarness) {
		name := "tfbaseline-unique-lifecycle"
		s := h.Apply("create with only the required attributes", h.null(), albAlert(name, "logs_unique_count", albUnique()))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: change keypath, max_unique_count, and time window", s, albAlert(name, "logs_unique_count", albUnique(j{
			"unique_count_keypath": "remote_addr_geoip.city_name", "rules": albUniqueRule(5, "20_MINUTES"),
		})))
		s = h.RefreshPlan("refresh after update", s)
		h.Import("import by id", idOf(s))
		h.Destroy("destroy", s)
	})

	addAlertScenario("logs-unique-count-full", func(h *grHarness) {
		name := "tfbaseline-unique-full"
		full := albUnique(j{
			"logs_filter": albFilterFull, "notification_payload_filter": albPayload,
			"max_unique_count_per_group_by_key": float64(500),
		})
		s := h.Apply("create with every attribute", h.null(), albAlert(name, "logs_unique_count", full, albGroupBy))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: change every attribute", s, albAlert(name, "logs_unique_count", albUnique(j{
			"logs_filter": j{"simple_filter": j{
				"lucene_query": "message:\"changed\"",
				"label_filters": j{
					"application_name": l{j{"operation": "INCLUDES", "value": "app-b"}},
					"subsystem_name":   l{j{"operation": "IS", "value": "sub-b"}},
					"severities":       l{"Critical"},
				},
			}},
			"notification_payload_filter":       l{"coralogix.metadata.sdkVersion"},
			"max_unique_count_per_group_by_key": float64(900),
			"unique_count_keypath":              "other.keypath",
			"rules":                             albUniqueRule(9, "2_HOURS"),
		}), j{"group_by": l{"a.key", "b.key"}}))
		s = h.RefreshPlan("refresh after update", s)
		h.Import("import by id", idOf(s))
		h.Destroy("destroy", s)
	})

	addAlertScenario("logs-unique-count-unset-optionals", func(h *grHarness) {
		name := "tfbaseline-unique-unset"
		full := albUnique(j{
			"logs_filter": albFilterFull, "notification_payload_filter": albPayload,
			"max_unique_count_per_group_by_key": float64(500),
		})
		s := h.Apply("create with every attribute", h.null(), albAlert(name, "logs_unique_count", full, albGroupBy))
		s = h.Apply("update: remove max_unique_count_per_group_by_key but keep group_by (the API rejects it)", s, albAlert(name, "logs_unique_count", with(full, j{"max_unique_count_per_group_by_key": nil}), albGroupBy))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: remove notification_payload_filter", s, albAlert(name, "logs_unique_count", with(full, j{"notification_payload_filter": nil}), albGroupBy))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: remove max_unique_count_per_group_by_key and group_by", s, albAlert(name, "logs_unique_count", albUnique(j{"logs_filter": albFilterFull})))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: remove logs_filter", s, albAlert(name, "logs_unique_count", albUnique()))
		s = h.RefreshPlan("refresh", s)
		h.Destroy("destroy", s)
	})

	addAlertScenario("logs-unique-count-time-windows", func(h *grHarness) {
		name := "tfbaseline-unique-windows"
		s := h.null()
		for _, w := range albUniqueWindows {
			s = h.Apply("time window "+w, s, albAlert(name, "logs_unique_count", albUnique(j{"rules": albUniqueRule(2, w)})))
		}
		h.RefreshPlan("refresh", s)
	})

	addAlertScenario("logs-unique-count-max-unique-count", func(h *grHarness) {
		name := "tfbaseline-unique-max"
		s := h.null()
		for _, v := range []float64{0, 1, 100, 2147483648} {
			s = h.Apply("max_unique_count", s, albAlert(name, "logs_unique_count", albUnique(j{"rules": albUniqueRule(v, "5_MINUTES")})))
		}
		s = h.RefreshPlan("refresh", s)
		h.Apply("update: negative max_unique_count", s, albAlert(name, "logs_unique_count", albUnique(j{"rules": albUniqueRule(-1, "5_MINUTES")})))
	})

	// max_unique_count_per_group_by_key is an Int64 in the schema and a string in the API.
	addAlertScenario("logs-unique-count-per-group-by-key", func(h *grHarness) {
		name := "tfbaseline-unique-per-key"
		s := h.Apply("create: omitted, no group_by", h.null(), albAlert(name, "logs_unique_count", albUnique()))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: set without group_by", s, albAlert(name, "logs_unique_count", albUnique(j{"max_unique_count_per_group_by_key": float64(10)})))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: removed again", s, albAlert(name, "logs_unique_count", albUnique()))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: group_by without max_unique_count_per_group_by_key (the API rejects it)", s, albAlert(name, "logs_unique_count", albUnique(), albGroupBy))
		for _, v := range []float64{1, 500, 2147483648, 9007199254740992} {
			s = h.Apply("max_unique_count_per_group_by_key", s, albAlert(name, "logs_unique_count", albUnique(j{"max_unique_count_per_group_by_key": v}), albGroupBy))
		}
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: zero", s, albAlert(name, "logs_unique_count", albUnique(j{"max_unique_count_per_group_by_key": float64(0)}), albGroupBy))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: negative", s, albAlert(name, "logs_unique_count", albUnique(j{"max_unique_count_per_group_by_key": float64(-1)}), albGroupBy))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: removed while group_by stays (the API rejects it)", s, albAlert(name, "logs_unique_count", albUnique(), albGroupBy))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: removed with group_by", s, albAlert(name, "logs_unique_count", albUnique()))
		h.RefreshPlan("refresh after removal", s)
	})

	addAlertScenario("logs-unique-count-keypath", func(h *grHarness) {
		name := "tfbaseline-unique-keypath"
		s := h.null()
		for _, k := range []string{"a", "a.b.c", "$d.field"} {
			s = h.Apply("unique_count_keypath "+k, s, albAlert(name, "logs_unique_count", albUnique(j{"unique_count_keypath": k})))
		}
		s = h.RefreshPlan("refresh", s)
		h.Apply("update: empty unique_count_keypath", s, albAlert(name, "logs_unique_count", albUnique(j{"unique_count_keypath": ""})))
	})

	addAlertScenario("logs-unique-count-group-by", func(h *grHarness) {
		name := "tfbaseline-unique-group-by"
		perKey := j{"max_unique_count_per_group_by_key": float64(100)}
		s := h.Apply("create without group_by", h.null(), albAlert(name, "logs_unique_count", albUnique()))
		s = h.Apply("update: one key", s, albAlert(name, "logs_unique_count", albUnique(perKey), albGroupBy))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: two keys, other order", s, albAlert(name, "logs_unique_count", albUnique(perKey), j{"group_by": l{"b.key", "a.key"}}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: empty list while max_unique_count_per_group_by_key stays (the API rejects it)", s, albAlert(name, "logs_unique_count", albUnique(perKey), j{"group_by": l{}}))
		s = h.Apply("update: empty list and no max_unique_count_per_group_by_key", s, albAlert(name, "logs_unique_count", albUnique(), j{"group_by": l{}}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: one key again", s, albAlert(name, "logs_unique_count", albUnique(perKey), albGroupBy))
		s = h.Apply("update: group_by removed while max_unique_count_per_group_by_key stays (the API rejects it)", s, albAlert(name, "logs_unique_count", albUnique(perKey)))
		s = h.Apply("update: both removed", s, albAlert(name, "logs_unique_count", albUnique()))
		h.RefreshPlan("refresh", s)
	})

	addAlertScenario("logs-unique-count-filter-collections", func(h *grHarness) {
		name := "tfbaseline-unique-filters"
		withFilter := func(extra j) j {
			return albAlert(name, "logs_unique_count", albUnique(j{"logs_filter": j{"simple_filter": with(j{"lucene_query": "tfbaseline"}, extra)}}))
		}
		s := h.Apply("create: no label_filters", h.null(), withFilter(nil))
		s = h.Apply("update: empty severities, application_name, and subsystem_name", s, withFilter(j{"label_filters": j{
			"severities": l{}, "application_name": l{}, "subsystem_name": l{},
		}}))
		s = h.RefreshPlan("refresh", s)
		for _, sev := range []string{"Debug", "Info", "Warning", "Error", "Critical", "Unspecified"} {
			s = h.Apply("severity "+sev, s, withFilter(j{"label_filters": j{"severities": l{sev}}}))
		}
		for _, op := range []string{"IS", "INCLUDES", "STARTS_WITH", "ENDS_WITH"} {
			s = h.Apply("operation "+op, s, withFilter(j{"label_filters": j{
				"application_name": l{j{"operation": op, "value": "a"}},
				"subsystem_name":   l{j{"operation": op, "value": "s"}},
			}}))
		}
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: clear the label filters", s, withFilter(nil))
		h.RefreshPlan("refresh", s)
	})

	addAlertScenario("logs-unique-count-payload-filter", func(h *grHarness) {
		name := "tfbaseline-unique-payload"
		s := h.Apply("create: omitted", h.null(), albAlert(name, "logs_unique_count", albUnique()))
		s = h.Apply("update: empty list", s, albAlert(name, "logs_unique_count", albUnique(j{"notification_payload_filter": l{}})))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: two values", s, albAlert(name, "logs_unique_count", albUnique(j{"notification_payload_filter": albPayload})))
		s = h.Apply("update: cleared", s, albAlert(name, "logs_unique_count", albUnique()))
		h.RefreshPlan("refresh", s)
	})

	addAlertScenario("logs-unique-count-validation", func(h *grHarness) {
		name := "tfbaseline-unique-validation"
		h.Validate("a valid config", albAlert(name, "logs_unique_count", albUnique()))
		h.Validate("unique_count_keypath missing", albAlert(name, "logs_unique_count", j{"rules": albUniqueRule(1, "5_MINUTES")}))
		h.Validate("null unique_count_keypath", albAlert(name, "logs_unique_count", albUnique(j{"unique_count_keypath": nil})))
		h.Validate("rules missing", albAlert(name, "logs_unique_count", j{"unique_count_keypath": "a"}))
		h.Validate("rules empty", albAlert(name, "logs_unique_count", albUnique(j{"rules": l{}})))
		h.Validate("rule without condition", albAlert(name, "logs_unique_count", albUnique(j{"rules": l{j{}}})))
		h.Validate("condition without max_unique_count", albAlert(name, "logs_unique_count", albUnique(j{"rules": l{j{"condition": j{"time_window": "5_MINUTES"}}}})))
		h.Validate("condition without time_window", albAlert(name, "logs_unique_count", albUnique(j{"rules": l{j{"condition": j{"max_unique_count": float64(1)}}}})))
		h.Validate("null max_unique_count", albAlert(name, "logs_unique_count", albUnique(j{"rules": l{j{"condition": j{"max_unique_count": nil, "time_window": "5_MINUTES"}}}})))
		h.Validate("null time_window", albAlert(name, "logs_unique_count", albUnique(j{"rules": l{j{"condition": j{"max_unique_count": float64(1), "time_window": nil}}}})))
		h.Validate("fractional max_unique_count", albAlert(name, "logs_unique_count", albUnique(j{"rules": albUniqueRule(1.5, "5_MINUTES")})))
		h.Validate("time_window of another type", albAlert(name, "logs_unique_count", albUnique(j{"rules": albUniqueRule(1, "3_HOURS")})))
		h.Validate("time_window 48_HOURS", albAlert(name, "logs_unique_count", albUnique(j{"rules": albUniqueRule(1, "48_HOURS")})))
		h.Validate("fractional max_unique_count_per_group_by_key", albAlert(name, "logs_unique_count", albUnique(j{"max_unique_count_per_group_by_key": float64(1.5)})))
		h.Validate("null max_unique_count_per_group_by_key", albAlert(name, "logs_unique_count", albUnique(j{"max_unique_count_per_group_by_key": nil})))
		h.Validate("unknown severity", albAlert(name, "logs_unique_count", albUnique(j{"logs_filter": j{"simple_filter": j{"label_filters": j{"severities": l{"Fatal"}}}}})))
		h.Validate("unknown label operation", albAlert(name, "logs_unique_count", albUnique(j{"logs_filter": j{"simple_filter": j{"label_filters": j{
			"application_name": l{j{"operation": "EQUALS", "value": "a"}},
		}}}})))
		h.Validate("group_by with a null member", albAlert(name, "logs_unique_count", albUnique(), j{"group_by": l{nil}}))
		h.Validate("two type definitions", j{"name": name, "type_definition": j{
			"logs_unique_count": albUnique(),
			"logs_new_value":    albNewValue(),
		}})
	})

	addAlertScenario("logs-unique-count-api-errors", func(h *grHarness) {
		name := "tfbaseline-unique-errors"
		h.Apply("create: empty unique_count_keypath", h.null(), albAlert(name, "logs_unique_count", albUnique(j{"unique_count_keypath": ""})))
		h.Apply("create: group_by without max_unique_count_per_group_by_key", h.null(), albAlert(name, "logs_unique_count", albUnique(), albGroupBy))
		h.Apply("create: two rules", h.null(), albAlert(name, "logs_unique_count", albUnique(j{"rules": l{
			j{"condition": j{"max_unique_count": float64(1), "time_window": "5_MINUTES"}},
			j{"condition": j{"max_unique_count": float64(2), "time_window": "1_HOUR"}},
		}})))
		s := h.Apply("create: a valid alert", h.null(), albAlert(name, "logs_unique_count", albUnique()))
		s = h.Apply("update: empty unique_count_keypath", s, albAlert(name, "logs_unique_count", albUnique(j{"unique_count_keypath": ""})))
		s = h.RefreshPlan("refresh after the failed update", s)
		h.Destroy("destroy", s)
	})
}

// --- logs_time_relative_threshold ---

var albComparedTo = []string{
	"Previous Hour", "Same Hour Yesterday", "Same Hour Last Week", "Yesterday", "Same Day Last Week", "Same Day Last Month",
}

var albRetire = []string{"NEVER", "5_MINUTES", "10_MINUTES", "1_HOUR", "2_HOURS", "6_HOURS", "12_HOURS", "24_HOURS"}

// albRelRule is one rule. A nil priority leaves override.priority out of the config.
func albRelRule(threshold float64, comparedTo, conditionType string, priority any) j {
	override := j{}
	if priority != nil {
		override["priority"] = priority
	}
	return j{
		"condition": j{"threshold": threshold, "compared_to": comparedTo, "condition_type": conditionType},
		"override":  override,
	}
}

func albRelative(extra ...j) j {
	out := j{"rules": l{albRelRule(10, "Same Hour Yesterday", "MORE_THAN", "P2")}}
	for _, e := range extra {
		out = with(out, e)
	}
	return out
}

func init() {
	addAlertScenario("logs-time-relative-lifecycle", func(h *grHarness) {
		name := "tfbaseline-relative-lifecycle"
		s := h.Apply("create with only the required attributes", h.null(), albAlert(name, "logs_time_relative_threshold", albRelative()))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: change every rule attribute", s, albAlert(name, "logs_time_relative_threshold", albRelative(j{"rules": l{
			albRelRule(50, "Same Day Last Week", "LESS_THAN", "P4"),
		}})))
		s = h.RefreshPlan("refresh after update", s)
		h.Import("import by id", idOf(s))
		h.Destroy("destroy", s)
	})

	addAlertScenario("logs-time-relative-full", func(h *grHarness) {
		name := "tfbaseline-relative-full"
		full := albRelative(j{
			"logs_filter":                  albFilterFull,
			"notification_payload_filter":  albPayload,
			"ignore_infinity":              true,
			"custom_evaluation_delay":      float64(100),
			"undetected_values_management": j{"trigger_undetected_values": true, "auto_retire_timeframe": "6_HOURS"},
			"rules":                        l{albRelRule(10, "Same Hour Yesterday", "LESS_THAN", "P2"), albRelRule(20, "Yesterday", "LESS_THAN", "P1")},
		})
		s := h.Apply("create with every attribute", h.null(), albAlert(name, "logs_time_relative_threshold", full, j{"group_by": l{"coralogix.metadata.applicationName"}}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: change every attribute", s, albAlert(name, "logs_time_relative_threshold", albRelative(j{
			"logs_filter": j{"simple_filter": j{
				"lucene_query": "message:\"changed\"",
				"label_filters": j{
					"application_name": l{j{"operation": "INCLUDES", "value": "app-b"}},
					"subsystem_name":   l{j{"operation": "IS", "value": "sub-b"}},
					"severities":       l{"Critical"},
				},
			}},
			"notification_payload_filter":  l{"coralogix.metadata.sdkVersion"},
			"ignore_infinity":              false,
			"custom_evaluation_delay":      float64(250),
			"undetected_values_management": j{"trigger_undetected_values": false, "auto_retire_timeframe": "1_HOUR"},
			"rules":                        l{albRelRule(5, "Previous Hour", "LESS_THAN", "P3")},
		}), j{"group_by": l{"coralogix.metadata.subsystemName"}}))
		s = h.RefreshPlan("refresh after update", s)
		h.Import("import by id", idOf(s))
		h.Destroy("destroy", s)
	})

	addAlertScenario("logs-time-relative-unset-optionals", func(h *grHarness) {
		name := "tfbaseline-relative-unset"
		full := albRelative(j{
			"logs_filter":                  albFilterFull,
			"notification_payload_filter":  albPayload,
			"ignore_infinity":              true,
			"custom_evaluation_delay":      float64(100),
			"undetected_values_management": j{"trigger_undetected_values": true, "auto_retire_timeframe": "6_HOURS"},
			"rules":                        l{albRelRule(10, "Same Hour Yesterday", "LESS_THAN", "P2")},
		})
		grp := j{"group_by": l{"coralogix.metadata.applicationName"}}
		s := h.Apply("create with every attribute", h.null(), albAlert(name, "logs_time_relative_threshold", full, grp))
		s = h.Apply("update: remove custom_evaluation_delay", s, albAlert(name, "logs_time_relative_threshold", with(full, j{"custom_evaluation_delay": nil}), grp))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: remove ignore_infinity", s, albAlert(name, "logs_time_relative_threshold", with(full, j{"custom_evaluation_delay": nil, "ignore_infinity": nil}), grp))
		s = h.RefreshPlan("refresh", s)
		noUndetected := with(full, j{"custom_evaluation_delay": nil, "ignore_infinity": nil, "undetected_values_management": nil})
		s = h.Apply("update: remove undetected_values_management", s, albAlert(name, "logs_time_relative_threshold", noUndetected, grp))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: remove notification_payload_filter", s, albAlert(name, "logs_time_relative_threshold", with(noUndetected, j{"notification_payload_filter": nil}), grp))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: remove logs_filter and group_by", s, albAlert(name, "logs_time_relative_threshold", albRelative(j{"rules": full["rules"]})))
		s = h.RefreshPlan("refresh", s)
		h.Destroy("destroy", s)
	})

	addAlertScenario("logs-time-relative-compared-to", func(h *grHarness) {
		name := "tfbaseline-relative-compared-to"
		s := h.null()
		for _, c := range albComparedTo {
			s = h.Apply("compared_to "+c, s, albAlert(name, "logs_time_relative_threshold", albRelative(j{"rules": l{albRelRule(10, c, "MORE_THAN", "P2")}})))
		}
		h.RefreshPlan("refresh", s)
	})

	addAlertScenario("logs-time-relative-condition-type", func(h *grHarness) {
		name := "tfbaseline-relative-condition-type"
		s := h.null()
		for _, c := range []string{"MORE_THAN", "LESS_THAN"} {
			s = h.Apply("condition_type "+c, s, albAlert(name, "logs_time_relative_threshold", albRelative(j{"rules": l{albRelRule(10, "Yesterday", c, "P2")}})))
		}
		h.RefreshPlan("refresh", s)
	})

	addAlertScenario("logs-time-relative-threshold-values", func(h *grHarness) {
		name := "tfbaseline-relative-threshold"
		s := h.null()
		for _, v := range []float64{0, 0.5, 100, 1000000} {
			s = h.Apply("threshold", s, albAlert(name, "logs_time_relative_threshold", albRelative(j{"rules": l{albRelRule(v, "Yesterday", "MORE_THAN", "P2")}})))
		}
		s = h.RefreshPlan("refresh", s)
		h.Apply("update: negative threshold", s, albAlert(name, "logs_time_relative_threshold", albRelative(j{"rules": l{albRelRule(-1, "Yesterday", "MORE_THAN", "P2")}})))
	})

	addAlertScenario("logs-time-relative-override-priority", func(h *grHarness) {
		name := "tfbaseline-relative-priority"
		s := h.null()
		for _, p := range []string{"P1", "P2", "P3", "P4", "P5"} {
			s = h.Apply("override.priority "+p, s, albAlert(name, "logs_time_relative_threshold", albRelative(j{"rules": l{albRelRule(10, "Yesterday", "MORE_THAN", p)}})))
		}
		h.RefreshPlan("refresh", s)
	})

	// An omitted override.priority falls back to the top-level priority, or P5.
	addAlertScenario("logs-time-relative-priority-fallback", func(h *grHarness) {
		name := "tfbaseline-relative-fallback"
		omitted := albRelative(j{"rules": l{albRelRule(10, "Yesterday", "MORE_THAN", nil)}})
		s := h.Apply("create: no override.priority and no top-level priority", h.null(), albAlert(name, "logs_time_relative_threshold", omitted))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: top-level priority P2", s, albAlert(name, "logs_time_relative_threshold", omitted, j{"priority": "P2"}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: top-level priority P4", s, albAlert(name, "logs_time_relative_threshold", omitted, j{"priority": "P4"}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: override.priority P1 beside the top-level priority", s, albAlert(name, "logs_time_relative_threshold", albRelative(j{"rules": l{albRelRule(10, "Yesterday", "MORE_THAN", "P1")}}), j{"priority": "P4"}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: override.priority removed again, top-level priority P4", s, albAlert(name, "logs_time_relative_threshold", omitted, j{"priority": "P4"}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: both removed", s, albAlert(name, "logs_time_relative_threshold", omitted))
		h.RefreshPlan("refresh", s)
	})

	addAlertScenario("logs-time-relative-rules", func(h *grHarness) {
		name := "tfbaseline-relative-rules"
		rules := func(n int) l {
			all := l{
				albRelRule(1, "Previous Hour", "MORE_THAN", "P1"),
				albRelRule(2, "Same Hour Yesterday", "MORE_THAN", "P2"),
				albRelRule(3, "Same Hour Last Week", "MORE_THAN", "P3"),
				albRelRule(4, "Yesterday", "MORE_THAN", "P4"),
				albRelRule(5, "Same Day Last Week", "MORE_THAN", "P5"),
				albRelRule(6, "Same Day Last Month", "MORE_THAN", "P1"),
			}
			return all[:n]
		}
		s := h.Apply("create: one rule", h.null(), albAlert(name, "logs_time_relative_threshold", albRelative(j{"rules": rules(1)})))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: three rules", s, albAlert(name, "logs_time_relative_threshold", albRelative(j{"rules": rules(3)})))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: five rules", s, albAlert(name, "logs_time_relative_threshold", albRelative(j{"rules": rules(5)})))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: back to two rules", s, albAlert(name, "logs_time_relative_threshold", albRelative(j{"rules": rules(2)})))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: six rules", s, albAlert(name, "logs_time_relative_threshold", albRelative(j{"rules": rules(6)})))
		h.RefreshPlan("refresh after the failed update", s)
	})

	addAlertScenario("logs-time-relative-ignore-infinity", func(h *grHarness) {
		name := "tfbaseline-relative-infinity"
		s := h.Apply("create: omitted", h.null(), albAlert(name, "logs_time_relative_threshold", albRelative()))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: true", s, albAlert(name, "logs_time_relative_threshold", albRelative(j{"ignore_infinity": true})))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: false", s, albAlert(name, "logs_time_relative_threshold", albRelative(j{"ignore_infinity": false})))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: true again", s, albAlert(name, "logs_time_relative_threshold", albRelative(j{"ignore_infinity": true})))
		s = h.Apply("update: removed", s, albAlert(name, "logs_time_relative_threshold", albRelative()))
		h.RefreshPlan("refresh", s)
	})

	addAlertScenario("logs-time-relative-undetected-values", func(h *grHarness) {
		name := "tfbaseline-relative-undetected"
		less := l{albRelRule(10, "Yesterday", "LESS_THAN", "P2")}
		more := l{albRelRule(10, "Yesterday", "MORE_THAN", "P2")}
		s := h.Apply("create: omitted", h.null(), albAlert(name, "logs_time_relative_threshold", albRelative(j{"rules": less})))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: trigger on a LESS_THAN rule", s, albAlert(name, "logs_time_relative_threshold", albRelative(j{"rules": less, "undetected_values_management": j{"trigger_undetected_values": true}})))
		s = h.RefreshPlan("refresh", s)
		for _, v := range albRetire {
			s = h.Apply("auto_retire_timeframe "+v, s, albAlert(name, "logs_time_relative_threshold", albRelative(j{"rules": less, "undetected_values_management": j{"trigger_undetected_values": true, "auto_retire_timeframe": v}})))
		}
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: only auto_retire_timeframe", s, albAlert(name, "logs_time_relative_threshold", albRelative(j{"rules": less, "undetected_values_management": j{"auto_retire_timeframe": "2_HOURS"}})))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: empty undetected_values_management", s, albAlert(name, "logs_time_relative_threshold", albRelative(j{"rules": less, "undetected_values_management": j{}})))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: trigger false", s, albAlert(name, "logs_time_relative_threshold", albRelative(j{"rules": less, "undetected_values_management": j{"trigger_undetected_values": false}})))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: trigger on, then the block is removed", s, albAlert(name, "logs_time_relative_threshold", albRelative(j{"rules": less, "undetected_values_management": j{"trigger_undetected_values": true, "auto_retire_timeframe": "1_HOUR"}})))
		s = h.Apply("update: block removed", s, albAlert(name, "logs_time_relative_threshold", albRelative(j{"rules": less})))
		s = h.RefreshPlan("refresh after removal", s)
		s = h.Apply("update: auto_retire_timeframe with only MORE_THAN rules", s, albAlert(name, "logs_time_relative_threshold", albRelative(j{"rules": more, "undetected_values_management": j{"auto_retire_timeframe": "2_HOURS"}})))
		s = h.RefreshPlan("refresh", s)
		h.Apply("update: trigger on with only MORE_THAN rules", s, albAlert(name, "logs_time_relative_threshold", albRelative(j{"rules": more, "undetected_values_management": j{"trigger_undetected_values": true}})))
	})

	addAlertScenario("logs-time-relative-custom-evaluation-delay", func(h *grHarness) {
		name := "tfbaseline-relative-delay"
		s := h.Apply("create: omitted", h.null(), albAlert(name, "logs_time_relative_threshold", albRelative()))
		s = h.Apply("update: zero", s, albAlert(name, "logs_time_relative_threshold", albRelative(j{"custom_evaluation_delay": float64(0)})))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: 60000", s, albAlert(name, "logs_time_relative_threshold", albRelative(j{"custom_evaluation_delay": float64(60000)})))
		s = h.Apply("update: the maximum of the API", s, albAlert(name, "logs_time_relative_threshold", albRelative(j{"custom_evaluation_delay": float64(10800000)})))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: removed", s, albAlert(name, "logs_time_relative_threshold", albRelative()))
		h.RefreshPlan("refresh", s)
	})

	addAlertScenario("logs-time-relative-filter-collections", func(h *grHarness) {
		name := "tfbaseline-relative-filters"
		withFilter := func(extra j) j {
			return albAlert(name, "logs_time_relative_threshold", albRelative(j{"logs_filter": j{"simple_filter": with(j{"lucene_query": "tfbaseline"}, extra)}}))
		}
		s := h.Apply("create: no label_filters", h.null(), withFilter(nil))
		s = h.Apply("update: empty severities, application_name, and subsystem_name", s, withFilter(j{"label_filters": j{
			"severities": l{}, "application_name": l{}, "subsystem_name": l{},
		}}))
		s = h.RefreshPlan("refresh", s)
		for _, sev := range []string{"Debug", "Info", "Warning", "Error", "Critical", "Unspecified"} {
			s = h.Apply("severity "+sev, s, withFilter(j{"label_filters": j{"severities": l{sev}}}))
		}
		for _, op := range []string{"IS", "INCLUDES", "STARTS_WITH", "ENDS_WITH"} {
			s = h.Apply("operation "+op, s, withFilter(j{"label_filters": j{
				"application_name": l{j{"operation": op, "value": "a"}},
				"subsystem_name":   l{j{"operation": op, "value": "s"}},
			}}))
		}
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: clear the label filters", s, withFilter(nil))
		h.RefreshPlan("refresh", s)
	})

	addAlertScenario("logs-time-relative-payload-filter", func(h *grHarness) {
		name := "tfbaseline-relative-payload"
		s := h.Apply("create: omitted", h.null(), albAlert(name, "logs_time_relative_threshold", albRelative()))
		s = h.Apply("update: empty list", s, albAlert(name, "logs_time_relative_threshold", albRelative(j{"notification_payload_filter": l{}})))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: two values", s, albAlert(name, "logs_time_relative_threshold", albRelative(j{"notification_payload_filter": albPayload})))
		s = h.Apply("update: cleared", s, albAlert(name, "logs_time_relative_threshold", albRelative()))
		h.RefreshPlan("refresh", s)
	})

	addAlertScenario("logs-time-relative-group-by", func(h *grHarness) {
		name := "tfbaseline-relative-group-by"
		s := h.Apply("create without group_by", h.null(), albAlert(name, "logs_time_relative_threshold", albRelative()))
		s = h.Apply("update: one key", s, albAlert(name, "logs_time_relative_threshold", albRelative(), albGroupBy))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: two keys, other order", s, albAlert(name, "logs_time_relative_threshold", albRelative(), j{"group_by": l{"b.key", "a.key"}}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: empty list", s, albAlert(name, "logs_time_relative_threshold", albRelative(), j{"group_by": l{}}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: removed", s, albAlert(name, "logs_time_relative_threshold", albRelative()))
		h.RefreshPlan("refresh", s)
	})

	addAlertScenario("logs-time-relative-validation", func(h *grHarness) {
		name := "tfbaseline-relative-validation"
		rule := func(condition, override j) l { return l{j{"condition": condition, "override": override}} }
		okCond := j{"threshold": float64(1), "compared_to": "Yesterday", "condition_type": "MORE_THAN"}
		h.Validate("a valid config", albAlert(name, "logs_time_relative_threshold", albRelative()))
		h.Validate("rules missing", albAlert(name, "logs_time_relative_threshold", j{}))
		h.Validate("rules empty", albAlert(name, "logs_time_relative_threshold", j{"rules": l{}}))
		h.Validate("rule without condition", albAlert(name, "logs_time_relative_threshold", j{"rules": l{j{"override": j{}}}}))
		h.Validate("rule without override", albAlert(name, "logs_time_relative_threshold", j{"rules": l{j{"condition": okCond}}}))
		h.Validate("null override", albAlert(name, "logs_time_relative_threshold", j{"rules": l{j{"condition": okCond, "override": nil}}}))
		h.Validate("condition without threshold", albAlert(name, "logs_time_relative_threshold", j{"rules": rule(j{"compared_to": "Yesterday", "condition_type": "MORE_THAN"}, j{})}))
		h.Validate("condition without compared_to", albAlert(name, "logs_time_relative_threshold", j{"rules": rule(j{"threshold": float64(1), "condition_type": "MORE_THAN"}, j{})}))
		h.Validate("condition without condition_type", albAlert(name, "logs_time_relative_threshold", j{"rules": rule(j{"threshold": float64(1), "compared_to": "Yesterday"}, j{})}))
		h.Validate("null threshold", albAlert(name, "logs_time_relative_threshold", j{"rules": rule(with(okCond, j{"threshold": nil}), j{})}))
		h.Validate("null compared_to", albAlert(name, "logs_time_relative_threshold", j{"rules": rule(with(okCond, j{"compared_to": nil}), j{})}))
		h.Validate("null condition_type", albAlert(name, "logs_time_relative_threshold", j{"rules": rule(with(okCond, j{"condition_type": nil}), j{})}))
		h.Validate("unknown compared_to", albAlert(name, "logs_time_relative_threshold", j{"rules": rule(with(okCond, j{"compared_to": "Last Year"}), j{})}))
		h.Validate("compared_to in enum spelling", albAlert(name, "logs_time_relative_threshold", j{"rules": rule(with(okCond, j{"compared_to": "SAME_HOUR_YESTERDAY"}), j{})}))
		h.Validate("unknown condition_type", albAlert(name, "logs_time_relative_threshold", j{"rules": rule(with(okCond, j{"condition_type": "EQUALS"}), j{})}))
		h.Validate("unknown override.priority", albAlert(name, "logs_time_relative_threshold", j{"rules": rule(okCond, j{"priority": "P6"})}))
		h.Validate("lowercase override.priority", albAlert(name, "logs_time_relative_threshold", j{"rules": rule(okCond, j{"priority": "p1"})}))
		h.Validate("unknown auto_retire_timeframe", albAlert(name, "logs_time_relative_threshold", albRelative(j{"undetected_values_management": j{"auto_retire_timeframe": "3_HOURS"}})))
		h.Validate("null members of undetected_values_management", albAlert(name, "logs_time_relative_threshold", albRelative(j{"undetected_values_management": j{"auto_retire_timeframe": nil, "trigger_undetected_values": nil}})))
		h.Validate("negative custom_evaluation_delay", albAlert(name, "logs_time_relative_threshold", albRelative(j{"custom_evaluation_delay": float64(-1)})))
		h.Validate("top-level priority beside override", albAlert(name, "logs_time_relative_threshold", albRelative(), j{"priority": "P2"}))
		h.Validate("unknown severity", albAlert(name, "logs_time_relative_threshold", albRelative(j{"logs_filter": j{"simple_filter": j{"label_filters": j{"severities": l{"Fatal"}}}}})))
		h.Validate("unknown label operation", albAlert(name, "logs_time_relative_threshold", albRelative(j{"logs_filter": j{"simple_filter": j{"label_filters": j{
			"application_name": l{j{"operation": "EQUALS", "value": "a"}},
		}}}})))
		h.Validate("group_by with a null member", albAlert(name, "logs_time_relative_threshold", albRelative(), j{"group_by": l{nil}}))
		h.Validate("two type definitions", j{"name": name, "type_definition": j{
			"logs_time_relative_threshold": albRelative(),
			"logs_anomaly":                 albAnomaly(),
		}})
	})

	addAlertScenario("logs-time-relative-api-errors", func(h *grHarness) {
		name := "tfbaseline-relative-errors"
		h.Apply("create: custom_evaluation_delay above the API maximum", h.null(), albAlert(name, "logs_time_relative_threshold", albRelative(j{"custom_evaluation_delay": float64(10800001)})))
		h.Apply("create: negative threshold", h.null(), albAlert(name, "logs_time_relative_threshold", albRelative(j{"rules": l{albRelRule(-1, "Yesterday", "MORE_THAN", "P2")}})))
		samePriority := albRelative(j{"rules": l{albRelRule(1, "Yesterday", "MORE_THAN", "P1"), albRelRule(2, "Previous Hour", "MORE_THAN", "P1")}})
		h.Apply("create: two rules with the same priority", h.null(), albAlert(name, "logs_time_relative_threshold", samePriority))
		mixed := albRelative(j{"rules": l{albRelRule(1, "Yesterday", "MORE_THAN", "P1"), albRelRule(2, "Previous Hour", "LESS_THAN", "P2")}})
		h.Apply("create: rules that mix MORE_THAN and LESS_THAN", h.null(), albAlert(name, "logs_time_relative_threshold", mixed))
		s := h.Apply("create: a valid alert", h.null(), albAlert(name, "logs_time_relative_threshold", albRelative()))
		s = h.Apply("update: rules that mix MORE_THAN and LESS_THAN", s, albAlert(name, "logs_time_relative_threshold", mixed))
		s = h.Apply("update: custom_evaluation_delay above the API maximum", s, albAlert(name, "logs_time_relative_threshold", albRelative(j{"custom_evaluation_delay": float64(10800001)})))
		s = h.RefreshPlan("refresh after the failed update", s)
		h.Destroy("destroy", s)
	})
}
