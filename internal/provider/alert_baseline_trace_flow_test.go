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
	"math"
	"net/http"
	"slices"
	"strconv"
	"strings"
)

// Scenarios of the alert types tracing_immediate, tracing_threshold, flow, analytics_immediate,
// and analytics_threshold. The shared attributes (notification group, incidents settings,
// schedule, labels, case settings, access policy) keep their defaults here: they have scenarios
// of their own in alert_baseline_common_test.go.

// tfOperations is every value of the operation of a tracing filter.
var tfOperations = []string{"IS", "IS_NOT", "INCLUDES", "STARTS_WITH", "ENDS_WITH"}

// tfAlert builds the config of an alert of one type, with the defaults for everything else.
func tfAlert(name, arm string, body j, extra ...j) j {
	out := j{"name": name, "type_definition": j{arm: body}}
	for _, e := range extra {
		out = with(out, e)
	}
	return out
}

// tfValues is one element of a set of tracing filters. An empty operation is left out of the
// config, so the schema default applies.
func tfValues(operation string, values ...any) j {
	out := j{"values": l(values)}
	if operation != "" {
		out["operation"] = operation
	}
	return out
}

// tfSpanField is one element of span_fields.
func tfSpanField(key string, filter j) j {
	out := j{"key": key}
	if filter != nil {
		out["filter_type"] = filter
	}
	return out
}

func tfTraceFilter(latency any, labels j) j {
	return j{"latency_threshold_ms": latency, "tracing_label_filters": labels}
}

// tfFullLabels sets every family of the label filters.
var tfFullLabels = j{
	"application_name": l{tfValues("IS", "app-a", "app-b"), tfValues("STARTS_WITH", "app-")},
	"subsystem_name":   l{tfValues("", "sub-a")},
	"service_name":     l{tfValues("INCLUDES", "svc")},
	"operation_name":   l{tfValues("ENDS_WITH", "op-a", "op-b")},
	"span_fields": l{
		tfSpanField("status", tfValues("", "200")),
		tfSpanField("status", tfValues("STARTS_WITH", "40", "50")),
		tfSpanField("http.method", tfValues("IS_NOT", "GET")),
	},
}

// tfChangedLabels changes every family of tfFullLabels.
var tfChangedLabels = j{
	"application_name": l{tfValues("IS_NOT", "app-c")},
	"subsystem_name":   l{tfValues("STARTS_WITH", "sub-b", "sub-c")},
	"service_name":     l{tfValues("IS", "svc-a"), tfValues("ENDS_WITH", "-svc")},
	"operation_name":   l{tfValues("INCLUDES", "op-c")},
	"span_fields": l{
		tfSpanField("status", tfValues("ENDS_WITH", "00")),
		tfSpanField("db.system", tfValues("IS", "postgres")),
	},
}

func tfWindows() []string {
	return []string{"5_MINUTES", "10_MINUTES", "15_MINUTES", "20_MINUTES", "30_MINUTES", "1_HOUR", "2_HOURS", "4_HOURS", "6_HOURS", "12_HOURS", "24_HOURS", "36_HOURS"}
}

func tfTraceRule(window string, spans float64) j {
	return j{"condition": j{"time_window": window, "span_amount": spans}}
}

// tfSeedLogsAlert creates an alert outside Terraform, for a flow to refer to.
func tfSeedLogsAlert(h *grHarness, name string) string {
	return h.alertBackend().seed(map[string]any{
		"name": name,
		"type": "ALERT_DEF_TYPE_LOGS_IMMEDIATE_OR_UNSPECIFIED",
		"logsImmediate": map[string]any{
			"logsFilter": map[string]any{"simpleFilter": map[string]any{"luceneQuery": "tfbaseline"}},
		},
	})
}

func init() {
	// --- tracing_immediate ---

	addAlertScenario("tracing-immediate-lifecycle", func(h *grHarness) {
		cfg := func(latency float64, extra ...j) j {
			return tfAlert("tfbaseline-tracing-immediate", "tracing_immediate", j{
				"tracing_filter": tfTraceFilter(latency, j{}),
			}, extra...)
		}
		s := h.Apply("create with an empty tracing_label_filters", h.null(), cfg(100))
		s = h.RefreshPlan("refresh", s)
		imported := h.Import("import by id", idOf(s))
		h.Plan("plan the same config on the imported state", imported, cfg(100))
		s = h.Apply("update: latency_threshold_ms", s, cfg(250))
		s = h.RefreshPlan("refresh after update", s)
		s = h.Apply("update: rename", s, with(cfg(250), j{"name": "tfbaseline-tracing-immediate-2"}))
		h.Destroy("destroy", s)
	})

	addAlertScenario("tracing-immediate-full", func(h *grHarness) {
		cfg := func(labels j, extra ...j) j {
			return tfAlert("tfbaseline-tracing-immediate-full", "tracing_immediate", j{
				"tracing_filter":              tfTraceFilter(float64(100), labels),
				"notification_payload_filter": l{"obj.field", "obj.other"},
			}, append([]j{{"description": "full", "priority": "P2", "enabled": false}}, extra...)...)
		}
		s := h.Apply("create with every optional attribute", h.null(), cfg(tfFullLabels))
		s = h.RefreshPlan("refresh", s)
		imported := h.Import("import by id", idOf(s))
		h.Plan("plan the same config on the imported state", imported, cfg(tfFullLabels))
		s = h.Apply("update: change every value", s, tfAlert("tfbaseline-tracing-immediate-full", "tracing_immediate", j{
			"tracing_filter":              tfTraceFilter(float64(250), tfChangedLabels),
			"notification_payload_filter": l{"obj.changed"},
		}, j{"description": "changed", "priority": "P4", "enabled": true}))
		s = h.RefreshPlan("refresh after the change", s)
		s = h.Apply("update: remove every optional attribute", s, tfAlert("tfbaseline-tracing-immediate-full", "tracing_immediate", j{
			"tracing_filter": tfTraceFilter(float64(250), j{}),
		}))
		s = h.RefreshPlan("refresh after the removal", s)
		s = h.Apply("update: set them again", s, cfg(tfFullLabels))
		h.Destroy("destroy", s)
	})

	// Removing the description while the rest of the type body stays the same, and while it changes.
	addAlertScenario("tracing-immediate-description-removal", func(h *grHarness) {
		cfg := func(latency float64, extra ...j) j {
			return tfAlert("tfbaseline-tracing-immediate-desc", "tracing_immediate", j{"tracing_filter": tfTraceFilter(latency, j{})}, extra...)
		}
		s := h.Apply("create with a description", h.null(), cfg(100, j{"description": "first"}))
		s = h.Apply("update: remove the description and nothing else", s, cfg(100))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: set the description", s, cfg(100, j{"description": "second"}))
		s = h.Apply("update: remove the description and change the latency", s, cfg(200))
		h.RefreshPlan("refresh", s)
	})

	// Each family of the label filters alone, so a removal of one family shows on its own.
	addAlertScenario("tracing-immediate-families", func(h *grHarness) {
		cfg := func(labels j) j {
			return tfAlert("tfbaseline-tracing-immediate-families", "tracing_immediate", j{"tracing_filter": tfTraceFilter(float64(100), labels)})
		}
		s := h.Apply("create with application_name only", h.null(), cfg(j{"application_name": l{tfValues("", "a")}}))
		s = h.Apply("update: subsystem_name only", s, cfg(j{"subsystem_name": l{tfValues("", "b")}}))
		s = h.Apply("update: service_name only", s, cfg(j{"service_name": l{tfValues("", "c")}}))
		s = h.Apply("update: operation_name only", s, cfg(j{"operation_name": l{tfValues("", "d")}}))
		s = h.Apply("update: span_fields only", s, cfg(j{"span_fields": l{tfSpanField("k", tfValues("", "v"))}}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: no family", s, cfg(j{}))
		h.RefreshPlan("refresh after the removal", s)
	})

	addAlertScenario("tracing-immediate-operations", func(h *grHarness) {
		s := h.null()
		for _, op := range tfOperations {
			s = h.Apply("operation "+op+" in every family", s, tfAlert("tfbaseline-tracing-immediate-ops", "tracing_immediate", j{
				"tracing_filter": tfTraceFilter(float64(100), j{
					"application_name": l{tfValues(op, "a")},
					"subsystem_name":   l{tfValues(op, "b")},
					"service_name":     l{tfValues(op, "c")},
					"operation_name":   l{tfValues(op, "d")},
					"span_fields":      l{tfSpanField("k", tfValues(op, "e"))},
				}),
			}))
		}
		s = h.RefreshPlan("refresh", s)
		h.Apply("update: omit every operation, so the default applies", s, tfAlert("tfbaseline-tracing-immediate-ops", "tracing_immediate", j{
			"tracing_filter": tfTraceFilter(float64(100), j{
				"application_name": l{tfValues("", "a")},
				"subsystem_name":   l{tfValues("", "b")},
				"service_name":     l{tfValues("", "c")},
				"operation_name":   l{tfValues("", "d")},
				"span_fields":      l{tfSpanField("k", tfValues("", "e"))},
			}),
		}))
	})

	addAlertScenario("tracing-immediate-collections", func(h *grHarness) {
		cfg := func(labels j, extra ...j) j {
			return tfAlert("tfbaseline-tracing-immediate-coll", "tracing_immediate", j{"tracing_filter": tfTraceFilter(float64(100), labels)}, extra...)
		}
		withPayload := func(labels j, payload any) j {
			return tfAlert("tfbaseline-tracing-immediate-coll", "tracing_immediate", j{
				"tracing_filter": tfTraceFilter(float64(100), labels), "notification_payload_filter": payload,
			})
		}
		s := h.Apply("create: every collection omitted", h.null(), cfg(j{}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: every collection explicitly empty", s, withPayload(j{
			"application_name": l{}, "subsystem_name": l{}, "service_name": l{}, "operation_name": l{}, "span_fields": l{},
		}, l{}))
		s = h.RefreshPlan("refresh after the empty collections", s)
		s = h.Apply("update: every collection with elements", s, withPayload(tfFullLabels, l{"a", "b"}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: every collection empty again", s, withPayload(j{
			"application_name": l{}, "subsystem_name": l{}, "service_name": l{}, "operation_name": l{}, "span_fields": l{},
		}, l{}))
		s = h.Apply("update: every collection omitted", s, cfg(j{}))
		s = h.RefreshPlan("refresh after the omission", s)
		s = h.Apply("update: a value set with several values", s, cfg(j{"application_name": l{tfValues("", "z", "y", "x", "w")}}))
		s = h.Apply("update: a value set with an empty string", s, cfg(j{"application_name": l{tfValues("", "")}}))
		s = h.Apply("update: a family with two elements of the same operation", s, cfg(j{"application_name": l{tfValues("IS", "a"), tfValues("IS", "b")}}))
		h.RefreshPlan("refresh", s)
	})

	// The API stores the latency as a string of digits. The provider sends big.Float.String(),
	// which writes a long number with an exponent, and reads the answer with ParseInt.
	addAlertScenario("tracing-immediate-latency", func(h *grHarness) {
		cfg := func(latency any) j {
			return tfAlert("tfbaseline-tracing-immediate-latency", "tracing_immediate", j{"tracing_filter": tfTraceFilter(latency, j{})})
		}
		s := h.null()
		for _, v := range []float64{0, 1, 1000, 9999999999, 10000000000, 12345678901} {
			s = h.Apply("latency "+floatText(v), s, cfg(v))
			s = h.RefreshPlan("refresh "+floatText(v), s)
		}
		h.Apply("update: a fraction", s, cfg(1.5))
		h.Apply("update: a negative value", s, cfg(float64(-1)))
		h.Apply("update: 2^53", s, cfg(float64(9007199254740992)))
		h.Apply("update: 2^63, above the largest int64", s, cfg(float64(9223372036854775808)))
		h.Apply("update: 2^64, above the largest uint64", s, cfg(float64(18446744073709551616)))
		h.Apply("create: 2^63 on a new alert", h.null(), cfg(float64(9223372036854775808)))
	})

	addAlertScenario("tracing-immediate-span-field-without-filter-type", func(h *grHarness) {
		// filter_type is optional in the schema, but the provider reads it as an object.
		cfg := tfAlert("tfbaseline-tracing-immediate-span", "tracing_immediate", j{
			"tracing_filter": tfTraceFilter(float64(100), j{"span_fields": l{tfSpanField("status", nil)}}),
		})
		h.Apply("create a span field with a key only", h.null(), cfg)
	})

	addAlertScenario("tracing-immediate-empty-values", func(h *grHarness) {
		// The API wants at least one value in a filter.
		cfg := func(labels j) j {
			return tfAlert("tfbaseline-tracing-immediate-empty-values", "tracing_immediate", j{"tracing_filter": tfTraceFilter(float64(100), labels)})
		}
		h.Apply("create with an empty values set", h.null(), cfg(j{"application_name": l{tfValues("IS")}}))
		h.Apply("create with an empty values set in a span field", h.null(), cfg(j{"span_fields": l{tfSpanField("k", tfValues("IS"))}}))
		h.Apply("create with an empty key in a span field", h.null(), cfg(j{"span_fields": l{tfSpanField("", tfValues("IS", "v"))}}))
	})

	addAlertScenario("tracing-immediate-config-validation", func(h *grHarness) {
		cfg := func(td j, extra ...j) j {
			out := j{"name": "tfbaseline-tracing-immediate-validation", "type_definition": j{"tracing_immediate": td}}
			for _, e := range extra {
				out = with(out, e)
			}
			return out
		}
		okFilter := tfTraceFilter(float64(100), j{})
		h.validateSorted("no tracing_filter", cfg(j{}))
		h.validateSorted("tracing_filter without latency_threshold_ms", cfg(j{"tracing_filter": j{"tracing_label_filters": j{}}}))
		h.validateSorted("tracing_filter without tracing_label_filters", cfg(j{"tracing_filter": j{"latency_threshold_ms": float64(100)}}))
		h.validateSorted("an empty tracing_filter", cfg(j{"tracing_filter": j{}}))
		h.validateSorted("operation outside the list", cfg(j{"tracing_filter": tfTraceFilter(float64(100), j{"application_name": l{tfValues("EQUALS", "a")}})}))
		h.validateSorted("lowercase operation", cfg(j{"tracing_filter": tfTraceFilter(float64(100), j{"subsystem_name": l{tfValues("is", "a")}})}))
		h.validateSorted("operation outside the list in a span field", cfg(j{"tracing_filter": tfTraceFilter(float64(100), j{"span_fields": l{tfSpanField("k", tfValues("NOT", "a"))}})}))
		h.validateSorted("operation outside the list in service_name", cfg(j{"tracing_filter": tfTraceFilter(float64(100), j{"service_name": l{tfValues("LIKE", "a")}})}))
		h.validateSorted("operation outside the list in operation_name", cfg(j{"tracing_filter": tfTraceFilter(float64(100), j{"operation_name": l{tfValues("LIKE", "a")}})}))
		h.validateSorted("an element without values", cfg(j{"tracing_filter": tfTraceFilter(float64(100), j{"application_name": l{j{"operation": "IS"}}})}))
		h.validateSorted("a span field without key", cfg(j{"tracing_filter": tfTraceFilter(float64(100), j{"span_fields": l{j{"filter_type": tfValues("", "a")}}})}))
		h.validateSorted("a span field filter_type without values", cfg(j{"tracing_filter": tfTraceFilter(float64(100), j{"span_fields": l{j{"key": "k", "filter_type": j{"operation": "IS"}}}})}))
		h.validateSorted("group_by is not allowed", cfg(j{"tracing_filter": okFilter}, j{"group_by": l{"coralogix.metadata.applicationName"}}))
		h.validateSorted("an empty group_by is not allowed either", cfg(j{"tracing_filter": okFilter}, j{"group_by": l{}}))
		h.validateSorted("a null group_by is fine", cfg(j{"tracing_filter": okFilter}, j{"group_by": nil}))
		h.validateSorted("two arms: tracing_immediate and tracing_threshold", j{"name": "tfbaseline-tracing-immediate-validation", "type_definition": j{
			"tracing_immediate": j{"tracing_filter": okFilter},
			"tracing_threshold": j{"tracing_filter": okFilter, "rules": l{tfTraceRule("5_MINUTES", 1)}},
		}})
		h.validateSorted("two arms: tracing_immediate and flow", j{"name": "tfbaseline-tracing-immediate-validation", "type_definition": j{
			"tracing_immediate": j{"tracing_filter": okFilter},
			"flow":              j{"stages": l{}},
		}})
	})

	// --- tracing_threshold ---

	addAlertScenario("tracing-threshold-lifecycle", func(h *grHarness) {
		cfg := func(window string, spans float64, extra ...j) j {
			return tfAlert("tfbaseline-tracing-threshold", "tracing_threshold", j{
				"tracing_filter": tfTraceFilter(float64(100), j{}),
				"rules":          l{tfTraceRule(window, spans)},
			}, extra...)
		}
		s := h.Apply("create", h.null(), cfg("10_MINUTES", 5))
		s = h.RefreshPlan("refresh", s)
		imported := h.Import("import by id", idOf(s))
		h.Plan("plan the same config on the imported state", imported, cfg("10_MINUTES", 5))
		s = h.Apply("update: time_window", s, cfg("1_HOUR", 5))
		s = h.Apply("update: span_amount", s, cfg("1_HOUR", 7))
		s = h.RefreshPlan("refresh after update", s)
		s = h.Apply("update: rename", s, cfg("1_HOUR", 7, j{"name": "tfbaseline-tracing-threshold-2"}))
		h.Destroy("destroy", s)
	})

	addAlertScenario("tracing-threshold-full", func(h *grHarness) {
		cfg := func(latency float64, labels j, window string, spans float64, payload any, extra j) j {
			td := j{
				"tracing_filter": tfTraceFilter(latency, labels),
				"rules":          l{tfTraceRule(window, spans)},
			}
			if payload != nil {
				td["notification_payload_filter"] = payload
			}
			return tfAlert("tfbaseline-tracing-threshold-full", "tracing_threshold", td, extra)
		}
		s := h.Apply("create with every optional attribute", h.null(), cfg(100, tfFullLabels, "15_MINUTES", 3, l{"obj.field"},
			j{"description": "full", "priority": "P2", "enabled": false, "group_by": l{"coralogix.metadata.applicationName"}}))
		s = h.RefreshPlan("refresh", s)
		imported := h.Import("import by id", idOf(s))
		h.Plan("plan the same config on the imported state", imported, cfg(100, tfFullLabels, "15_MINUTES", 3, l{"obj.field"},
			j{"description": "full", "priority": "P2", "enabled": false, "group_by": l{"coralogix.metadata.applicationName"}}))
		s = h.Apply("update: change every value", s, cfg(250, tfChangedLabels, "2_HOURS", 9, l{"obj.changed", "obj.other"},
			j{"description": "changed", "priority": "P4", "enabled": true, "group_by": l{"coralogix.metadata.subsystemName", "coralogix.metadata.applicationName"}}))
		s = h.RefreshPlan("refresh after the change", s)
		s = h.Apply("update: remove every optional attribute", s, cfg(250, j{}, "2_HOURS", 9, nil, nil))
		s = h.RefreshPlan("refresh after the removal", s)
		s = h.Apply("update: set them again", s, cfg(100, tfFullLabels, "15_MINUTES", 3, l{"obj.field"},
			j{"description": "full", "priority": "P2", "enabled": false, "group_by": l{"coralogix.metadata.applicationName"}}))
		h.Destroy("destroy", s)
	})

	addAlertScenario("tracing-threshold-time-windows", func(h *grHarness) {
		s := h.null()
		for _, w := range tfWindows() {
			s = h.Apply("time_window "+w, s, tfAlert("tfbaseline-tracing-threshold-windows", "tracing_threshold", j{
				"tracing_filter": tfTraceFilter(float64(100), j{}),
				"rules":          l{tfTraceRule(w, 1)},
			}))
		}
		h.RefreshPlan("refresh", s)
	})

	addAlertScenario("tracing-threshold-operations", func(h *grHarness) {
		s := h.null()
		for _, op := range tfOperations {
			s = h.Apply("operation "+op+" in every family", s, tfAlert("tfbaseline-tracing-threshold-ops", "tracing_threshold", j{
				"tracing_filter": tfTraceFilter(float64(100), j{
					"application_name": l{tfValues(op, "a")},
					"subsystem_name":   l{tfValues(op, "b")},
					"service_name":     l{tfValues(op, "c")},
					"operation_name":   l{tfValues(op, "d")},
					"span_fields":      l{tfSpanField("k", tfValues(op, "e"))},
				}),
				"rules": l{tfTraceRule("5_MINUTES", 1)},
			}))
		}
		h.RefreshPlan("refresh", s)
	})

	addAlertScenario("tracing-threshold-collections", func(h *grHarness) {
		cfg := func(labels j, payload any) j {
			td := j{"tracing_filter": tfTraceFilter(float64(100), labels), "rules": l{tfTraceRule("5_MINUTES", 1)}}
			if payload != nil {
				td["notification_payload_filter"] = payload
			}
			return tfAlert("tfbaseline-tracing-threshold-coll", "tracing_threshold", td)
		}
		empty := j{"application_name": l{}, "subsystem_name": l{}, "service_name": l{}, "operation_name": l{}, "span_fields": l{}}
		s := h.Apply("create: every collection omitted", h.null(), cfg(j{}, nil))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: every collection explicitly empty", s, cfg(empty, l{}))
		s = h.RefreshPlan("refresh after the empty collections", s)
		s = h.Apply("update: every collection with elements", s, cfg(tfFullLabels, l{"a", "b"}))
		s = h.Apply("update: every collection omitted", s, cfg(j{}, nil))
		h.RefreshPlan("refresh after the omission", s)
	})

	addAlertScenario("tracing-threshold-latency", func(h *grHarness) {
		cfg := func(latency any) j {
			return tfAlert("tfbaseline-tracing-threshold-latency", "tracing_threshold", j{
				"tracing_filter": tfTraceFilter(latency, j{}), "rules": l{tfTraceRule("5_MINUTES", 1)},
			})
		}
		s := h.null()
		for _, v := range []float64{0, 1000, 12345678901} {
			s = h.Apply("latency "+floatText(v), s, cfg(v))
			s = h.RefreshPlan("refresh "+floatText(v), s)
		}
		h.Apply("update: 2^63, above the largest int64", s, cfg(float64(9223372036854775808)))
		h.Apply("update: a fraction", s, cfg(1.5))
	})

	addAlertScenario("tracing-threshold-rules", func(h *grHarness) {
		cfg := func(rules l, extra ...j) j {
			return tfAlert("tfbaseline-tracing-threshold-rules", "tracing_threshold", j{
				"tracing_filter": tfTraceFilter(float64(100), j{}), "rules": rules,
			}, extra...)
		}
		s := h.Apply("create with span_amount 0", h.null(), cfg(l{tfTraceRule("5_MINUTES", 0)}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: a fractional span_amount", s, cfg(l{tfTraceRule("5_MINUTES", 2.5)}))
		s = h.Apply("update: span_amount 2.7", s, cfg(l{tfTraceRule("5_MINUTES", 2.7)}))
		s = h.Apply("update: span_amount 0.5", s, cfg(l{tfTraceRule("5_MINUTES", 0.5)}))
		s = h.Apply("update: span_amount 3.5", s, cfg(l{tfTraceRule("5_MINUTES", 3.5)}))
		s = h.Apply("update: a large span_amount", s, cfg(l{tfTraceRule("5_MINUTES", 1000000)}))
		s = h.RefreshPlan("refresh", s)
		h.Apply("update: a negative span_amount", s, cfg(l{tfTraceRule("5_MINUTES", -1)}))
		h.Apply("update: condition_type set to its only value", s, cfg(l{j{"condition": j{
			"time_window": "5_MINUTES", "span_amount": float64(1), "condition_type": "MORE_THAN",
		}}}))
		h.Apply("update: two rules (the API accepts one)", s, cfg(l{tfTraceRule("5_MINUTES", 1), tfTraceRule("1_HOUR", 2)}))
		h.Apply("update: two rules with the same time_window", s, cfg(l{tfTraceRule("5_MINUTES", 1), tfTraceRule("5_MINUTES", 2)}))
	})

	addAlertScenario("tracing-threshold-group-by", func(h *grHarness) {
		cfg := func(extra ...j) j {
			return tfAlert("tfbaseline-tracing-threshold-group-by", "tracing_threshold", j{
				"tracing_filter": tfTraceFilter(float64(100), j{}), "rules": l{tfTraceRule("5_MINUTES", 1)},
			}, extra...)
		}
		s := h.Apply("create without group_by", h.null(), cfg())
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: group_by with one key", s, cfg(j{"group_by": l{"coralogix.metadata.applicationName"}}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: group_by with two keys", s, cfg(j{"group_by": l{"coralogix.metadata.applicationName", "coralogix.metadata.subsystemName"}}))
		s = h.Apply("update: remove group_by", s, cfg())
		s = h.RefreshPlan("refresh after the removal", s)
		s = h.Apply("update: group_by = []", s, cfg(j{"group_by": l{}}))
		s = h.RefreshPlan("refresh after the empty list", s)
		s = h.Apply("update: group_by with one key again", s, cfg(j{"group_by": l{"coralogix.metadata.applicationName"}}))
		h.Apply("update: group_by = []", s, cfg(j{"group_by": l{}}))
	})

	addAlertScenario("tracing-threshold-config-validation", func(h *grHarness) {
		okFilter := tfTraceFilter(float64(100), j{})
		cfg := func(td j) j {
			return j{"name": "tfbaseline-tracing-threshold-validation", "type_definition": j{"tracing_threshold": td}}
		}
		rule := func(c j) l { return l{j{"condition": c}} }
		h.validateSorted("no tracing_filter and no rules", cfg(j{}))
		h.validateSorted("no rules", cfg(j{"tracing_filter": okFilter}))
		h.validateSorted("no tracing_filter", cfg(j{"rules": rule(j{"time_window": "5_MINUTES", "span_amount": float64(1)})}))
		h.validateSorted("empty rules", cfg(j{"tracing_filter": okFilter, "rules": l{}}))
		h.validateSorted("a rule without condition", cfg(j{"tracing_filter": okFilter, "rules": l{j{}}}))
		h.validateSorted("a condition without time_window", cfg(j{"tracing_filter": okFilter, "rules": rule(j{"span_amount": float64(1)})}))
		h.validateSorted("a condition without span_amount", cfg(j{"tracing_filter": okFilter, "rules": rule(j{"time_window": "5_MINUTES"})}))
		h.validateSorted("time_window outside the list", cfg(j{"tracing_filter": okFilter, "rules": rule(j{"time_window": "7_MINUTES", "span_amount": float64(1)})}))
		h.validateSorted("time_window of another alert type", cfg(j{"tracing_filter": okFilter, "rules": rule(j{"time_window": "1_WEEK", "span_amount": float64(1)})}))
		h.validateSorted("lowercase time_window", cfg(j{"tracing_filter": okFilter, "rules": rule(j{"time_window": "1_hour", "span_amount": float64(1)})}))
		h.validateSorted("condition_type is computed and cannot be set to another value", cfg(j{"tracing_filter": okFilter, "rules": rule(j{
			"time_window": "5_MINUTES", "span_amount": float64(1), "condition_type": "LESS_THAN",
		})}))
		h.validateSorted("operation outside the list", cfg(j{
			"tracing_filter": tfTraceFilter(float64(100), j{"application_name": l{tfValues("EQUALS", "a")}}),
			"rules":          rule(j{"time_window": "5_MINUTES", "span_amount": float64(1)}),
		}))
		h.validateSorted("a null group_by is fine", with(cfg(j{"tracing_filter": okFilter, "rules": rule(j{"time_window": "5_MINUTES", "span_amount": float64(1)})}), j{"group_by": nil}))
	})
}

// floatText names a number in a step. It writes whole numbers without an exponent.
func floatText(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// atfZod is the text of one validation error of the API: the JSON of its validator, indented by
// two spaces. A path element is a string or an index.
func atfZod(code, limitKey string, limit float64, typ string, exact bool, message string, path ...any) string {
	var sb strings.Builder
	sb.WriteString("[\n  {\n    \"code\": \"" + code + "\",\n    \"" + limitKey + "\": " + strconv.FormatFloat(limit, 'f', -1, 64))
	sb.WriteString(",\n    \"type\": \"" + typ + "\",\n    \"inclusive\": true,\n    \"exact\": " + strconv.FormatBool(exact))
	sb.WriteString(",\n    \"message\": \"" + message + "\",\n    \"path\": [")
	for i, p := range path {
		if i > 0 {
			sb.WriteString(",")
		}
		if name, ok := p.(string); ok {
			sb.WriteString("\n      \"" + name + "\"")
		} else {
			sb.WriteString(fmt.Sprintf("\n      %v", p))
		}
	}
	sb.WriteString("\n    ]\n  }\n]")
	return sb.String()
}

func atfPath(arm string, rest ...any) []any {
	return append([]any{"alertDefProperties", "typeDefinition", arm}, rest...)
}

func atfObj(m map[string]any, key string) map[string]any {
	out, _ := m[key].(map[string]any)
	return out
}

// atfTracingFilter returns the simple filter of a tracing arm, or nil.
func atfTracingFilter(props map[string]any, arm string) map[string]any {
	return atfObj(atfObj(atfObj(props, arm), "tracingFilter"), "simpleFilter")
}

func init() {
	addAlertStatusCheck(atfStatusLatency)
	addAlertFill(atfFillLatency)
	addAlertFill(atfFillSpanAmount)
	addAlertCheck(atfCheckTracingThreshold)
}

// Recorded: the latency is a uint64 in a string. The gateway refuses a fraction and a sign
// with the position of the value in the body, and the service refuses a value that no double
// holds exactly. It accepts an exponent ("1e+10") and stores the integer.
func atfStatusLatency(props map[string]any) (int, any) {
	for _, arm := range []string{"tracingImmediate", "tracingThreshold"} {
		filter := atfTracingFilter(props, arm)
		text, ok := filter["latencyThresholdMs"].(string)
		if !ok {
			continue
		}
		value, err := strconv.ParseFloat(text, 64)
		if err != nil || value != math.Trunc(value) || value < 0 {
			body, _ := json.Marshal(map[string]any{"alertDefProperties": props})
			at := strings.Index(string(body), `"latencyThresholdMs":"`+text+`"`) + len(`"latencyThresholdMs":"`)
			return http.StatusBadRequest, map[string]any{"code": 400, "message": fmt.Sprintf("Bad Request: proto:\u00a0(line 1:%d): invalid value for uint64 field value: %q", at, text)}
		}
		if value > 9007199254740991 {
			return http.StatusInternalServerError, map[string]any{"code": 500, "message": "Internal Server Error: Error deserializing request: Value is larger than Number.MAX_SAFE_INTEGER"}
		}
	}
	return 0, nil
}

func atfFillLatency(props map[string]any) {
	for _, arm := range []string{"tracingImmediate", "tracingThreshold"} {
		filter := atfTracingFilter(props, arm)
		if text, ok := filter["latencyThresholdMs"].(string); ok {
			if value, err := strconv.ParseFloat(text, 64); err == nil {
				filter["latencyThresholdMs"] = strconv.FormatFloat(value, 'f', -1, 64)
			}
		}
	}
}

// Recorded: the span amount is an integer on the server, and a fraction is cut off (2.7 reads 2).
func atfFillSpanAmount(props map[string]any) {
	rules, _ := atfObj(props, "tracingThreshold")["rules"].([]any)
	for _, raw := range rules {
		rule, _ := raw.(map[string]any)
		cond := atfObj(rule, "condition")
		if spans, ok := cond["spanAmount"].(float64); ok {
			cond["spanAmount"] = math.Trunc(spans)
		}
	}
}

func atfCheckTracingThreshold(props map[string]any) string {
	rules, _ := atfObj(props, "tracingThreshold")["rules"].([]any)
	if _, present := atfObj(props, "tracingThreshold")["rules"]; present && len(rules) > 1 {
		return atfZod("too_big", "maximum", 1, "array", true, "Array must contain exactly 1 element(s)", atfPath("tracingThreshold", "rules")...)
	}
	for i, raw := range rules {
		rule, _ := raw.(map[string]any)
		if spans, ok := atfObj(rule, "condition")["spanAmount"].(float64); ok && spans < 0 {
			return atfZod("too_small", "minimum", 0, "number", false, "Number must be greater than or equal to 0", atfPath("tracingThreshold", "rules", i, "condition", "spanAmount")...)
		}
	}
	return ""
}

// --- flow ---

func tfFlowDef(id any, not ...bool) j {
	out := j{"id": id}
	if len(not) > 0 {
		out["not"] = not[0]
	}
	return out
}

func tfFlowGroup(alertsOp, nextOp string, defs ...j) j {
	items := make(l, len(defs))
	for i, d := range defs {
		items[i] = d
	}
	return j{"alert_defs": items, "alerts_op": alertsOp, "next_op": nextOp}
}

// tfFlowStage is one stage. A nil timeframe_ms is left out of the config.
func tfFlowStage(timeframeType string, ms any, groups ...j) j {
	items := make(l, len(groups))
	for i, g := range groups {
		items[i] = g
	}
	out := j{"timeframe_type": timeframeType, "flow_stages_groups": items}
	if ms != nil {
		out["timeframe_ms"] = ms
	}
	return out
}

func tfFlow(stages ...j) j {
	items := make(l, len(stages))
	for i, s := range stages {
		items[i] = s
	}
	return j{"stages": items}
}

// tfSeedThresholdAlert creates a logs_threshold alert with group_by keys outside Terraform.
func tfSeedThresholdAlert(h *grHarness, name string, keys ...any) string {
	return h.alertBackend().seed(map[string]any{
		"name":        name,
		"type":        "ALERT_DEF_TYPE_LOGS_THRESHOLD",
		"groupByKeys": keys,
		"logsThreshold": map[string]any{
			"logsFilter": map[string]any{"simpleFilter": map[string]any{"luceneQuery": "tfbaseline"}},
			"rules": []any{map[string]any{
				"condition": map[string]any{
					"threshold":     float64(1),
					"conditionType": "LOGS_THRESHOLD_CONDITION_TYPE_MORE_THAN_OR_UNSPECIFIED",
					"timeWindow":    map[string]any{"logsTimeWindowSpecificValue": "LOGS_TIME_WINDOW_VALUE_MINUTES_10"},
				},
				"override": map[string]any{"priority": "ALERT_DEF_PRIORITY_P3"},
			}},
		},
	})
}

func init() {
	addAlertScenario("flow-lifecycle", func(h *grHarness) {
		a, b := tfSeedLogsAlert(h, "tfbaseline-flow-lifecycle-a"), tfSeedLogsAlert(h, "tfbaseline-flow-lifecycle-b")
		cfg := func(body j, extra ...j) j { return tfAlert("tfbaseline-flow-lifecycle", "flow", body, extra...) }
		one := tfFlow(tfFlowStage("Up To", float64(10), tfFlowGroup("OR", "AND", tfFlowDef(a))))
		s := h.Apply("create with one stage, one group, one alert", h.null(), cfg(one))
		s = h.RefreshPlan("refresh", s)
		imported := h.Import("import by id", idOf(s))
		h.Plan("plan the same config on the imported state", imported, cfg(one))
		s = h.Apply("update: a second alert in the group", s, cfg(tfFlow(tfFlowStage("Up To", float64(10), tfFlowGroup("OR", "AND", tfFlowDef(a), tfFlowDef(b))))))
		s = h.Apply("update: a second group", s, cfg(tfFlow(tfFlowStage("Up To", float64(10),
			tfFlowGroup("OR", "AND", tfFlowDef(a), tfFlowDef(b)), tfFlowGroup("AND", "OR", tfFlowDef(b))))))
		s = h.Apply("update: a second stage", s, cfg(tfFlow(
			tfFlowStage("Up To", float64(10), tfFlowGroup("OR", "AND", tfFlowDef(a), tfFlowDef(b)), tfFlowGroup("AND", "OR", tfFlowDef(b))),
			tfFlowStage("Up To", float64(20), tfFlowGroup("AND", "OR", tfFlowDef(a))))))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: back to one stage", s, cfg(one))
		s = h.Apply("update: rename", s, cfg(one, j{"name": "tfbaseline-flow-lifecycle-2"}))
		h.Destroy("destroy", s)
	})

	addAlertScenario("flow-full", func(h *grHarness) {
		a, b, c := tfSeedLogsAlert(h, "tfbaseline-flow-full-a"), tfSeedLogsAlert(h, "tfbaseline-flow-full-b"), tfSeedLogsAlert(h, "tfbaseline-flow-full-c")
		full := func(extra j) j {
			return tfAlert("tfbaseline-flow-full", "flow", j{
				"enforce_suppression": true,
				"stages": l{
					tfFlowStage("Up To", float64(60000), tfFlowGroup("OR", "AND", tfFlowDef(a, true), tfFlowDef(b, false)), tfFlowGroup("AND", "OR", tfFlowDef(c, true))),
					tfFlowStage("Up To", float64(120000), tfFlowGroup("AND", "AND", tfFlowDef(a))),
				},
			}, j{"description": "full", "priority": "P2", "enabled": false}, extra)
		}
		s := h.Apply("create with every optional attribute", h.null(), full(nil))
		s = h.RefreshPlan("refresh", s)
		imported := h.Import("import by id", idOf(s))
		h.Plan("plan the same config on the imported state", imported, full(nil))
		s = h.Apply("update: change every value", s, tfAlert("tfbaseline-flow-full", "flow", j{
			"enforce_suppression": false,
			"stages": l{
				tfFlowStage("Up To", float64(30000), tfFlowGroup("AND", "OR", tfFlowDef(c, false), tfFlowDef(b, true)), tfFlowGroup("OR", "OR", tfFlowDef(a))),
				tfFlowStage("Up To", float64(90000), tfFlowGroup("OR", "AND", tfFlowDef(b, true))),
			},
		}, j{"description": "changed", "priority": "P4", "enabled": true}))
		s = h.RefreshPlan("refresh after the change", s)
		s = h.Apply("update: remove every optional attribute", s, tfAlert("tfbaseline-flow-full", "flow", j{
			"stages": l{tfFlowStage("Up To", nil, tfFlowGroup("OR", "AND", tfFlowDef(a)))},
		}))
		s = h.RefreshPlan("refresh after the removal", s)
		s = h.Apply("update: set them again", s, full(nil))
		h.Destroy("destroy", s)
	})

	addAlertScenario("flow-enums", func(h *grHarness) {
		a := tfSeedLogsAlert(h, "tfbaseline-flow-enums-a")
		cfg := func(stage j) j { return tfAlert("tfbaseline-flow-enums", "flow", tfFlow(stage)) }
		s := h.null()
		for _, alertsOp := range []string{"AND", "OR"} {
			for _, nextOp := range []string{"AND", "OR"} {
				s = h.Apply("alerts_op "+alertsOp+", next_op "+nextOp, s, cfg(tfFlowStage("Up To", float64(10), tfFlowGroup(alertsOp, nextOp, tfFlowDef(a)))))
			}
		}
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("timeframe_type Unspecified, with timeframe_ms", s, cfg(tfFlowStage("Unspecified", float64(10), tfFlowGroup("OR", "OR", tfFlowDef(a)))))
		s = h.Apply("timeframe_type Unspecified, without timeframe_ms", s, cfg(tfFlowStage("Unspecified", nil, tfFlowGroup("OR", "OR", tfFlowDef(a)))))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("timeframe_type Up To, without timeframe_ms", s, cfg(tfFlowStage("Up To", nil, tfFlowGroup("OR", "OR", tfFlowDef(a)))))
		h.RefreshPlan("refresh", s)
	})

	addAlertScenario("flow-timeframe", func(h *grHarness) {
		a := tfSeedLogsAlert(h, "tfbaseline-flow-timeframe-a")
		cfg := func(ms any) j {
			return tfAlert("tfbaseline-flow-timeframe", "flow", tfFlow(tfFlowStage("Up To", ms, tfFlowGroup("OR", "AND", tfFlowDef(a)))))
		}
		s := h.null()
		for _, v := range []float64{0, 1, 3600000, 9007199254740992} {
			s = h.Apply("timeframe_ms "+floatText(v), s, cfg(v))
			s = h.RefreshPlan("refresh "+floatText(v), s)
		}
		h.Apply("update: a second stage without timeframe_ms", s, tfAlert("tfbaseline-flow-timeframe", "flow", tfFlow(
			tfFlowStage("Up To", float64(10), tfFlowGroup("OR", "AND", tfFlowDef(a))),
			tfFlowStage("Up To", nil, tfFlowGroup("OR", "AND", tfFlowDef(a))))))
		h.Apply("update: a second stage with timeframe_ms 0", s, tfAlert("tfbaseline-flow-timeframe", "flow", tfFlow(
			tfFlowStage("Up To", float64(10), tfFlowGroup("OR", "AND", tfFlowDef(a))),
			tfFlowStage("Up To", float64(0), tfFlowGroup("OR", "AND", tfFlowDef(a))))))
		h.Apply("update: a first stage with timeframe_ms 0 and a second stage with 5", s, tfAlert("tfbaseline-flow-timeframe", "flow", tfFlow(
			tfFlowStage("Up To", float64(0), tfFlowGroup("OR", "AND", tfFlowDef(a))),
			tfFlowStage("Up To", float64(5), tfFlowGroup("OR", "AND", tfFlowDef(a))))))
		h.Apply("update: a negative timeframe_ms", s, cfg(float64(-1)))
		h.Apply("update: a fractional timeframe_ms", s, cfg(1.5))
		h.Apply("update: 2^63, above the largest int64", s, cfg(float64(9223372036854775808)))
		h.Apply("update: omit timeframe_ms", s, cfg(nil))
	})

	addAlertScenario("flow-collections", func(h *grHarness) {
		a, b := tfSeedLogsAlert(h, "tfbaseline-flow-coll-a"), tfSeedLogsAlert(h, "tfbaseline-flow-coll-b")
		cfg := func(stages ...j) j { return tfAlert("tfbaseline-flow-coll", "flow", tfFlow(stages...)) }
		ok := tfFlowStage("Up To", float64(10), tfFlowGroup("OR", "AND", tfFlowDef(a)))
		s := h.Apply("create with no stages", h.null(), cfg())
		s = h.Apply("create with a stage that has no groups", s, cfg(tfFlowStage("Up To", float64(10))))
		s = h.Apply("create with a group that has no alert_defs", s, cfg(tfFlowStage("Up To", float64(10), tfFlowGroup("OR", "AND"))))
		s = h.Apply("create with one group", s, cfg(ok))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: the same alert twice, with and without not", s, cfg(tfFlowStage("Up To", float64(10), tfFlowGroup("OR", "AND", tfFlowDef(a, false), tfFlowDef(a, true)))))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: the same alert twice with the same flags", s, cfg(tfFlowStage("Up To", float64(10), tfFlowGroup("OR", "AND", tfFlowDef(a), tfFlowDef(a, false)))))
		s = h.Apply("update: the same alert in two groups", s, cfg(tfFlowStage("Up To", float64(10), tfFlowGroup("OR", "AND", tfFlowDef(a)), tfFlowGroup("OR", "AND", tfFlowDef(a)))))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: groups with the same content in two stages", s, cfg(ok, ok))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: two stages in the other order", s, cfg(
			tfFlowStage("Up To", float64(20), tfFlowGroup("AND", "OR", tfFlowDef(b))),
			tfFlowStage("Up To", float64(10), tfFlowGroup("OR", "AND", tfFlowDef(a))),
		))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: stages = []", s, tfAlert("tfbaseline-flow-coll", "flow", j{"stages": l{}}))
		h.RefreshPlan("refresh", s)
	})

	addAlertScenario("flow-references", func(h *grHarness) {
		a := tfSeedLogsAlert(h, "tfbaseline-flow-refs-a")
		gone := tfSeedLogsAlert(h, "tfbaseline-flow-refs-gone")
		h.alertBackend().forget(gone)
		cfg := func(id any) j {
			return tfAlert("tfbaseline-flow-refs", "flow", tfFlow(tfFlowStage("Up To", float64(10), tfFlowGroup("OR", "AND", tfFlowDef(id)))))
		}
		s := h.Apply("create with an alert that was deleted", h.null(), cfg(gone))
		s = h.Apply("create with an id that is no uuid", s, cfg("not-a-uuid"))
		s = h.Apply("create with an empty id", s, cfg(""))
		s = h.Apply("create with a uuid that no alert has", s, cfg("00000000-0000-4000-8000-0000000000ff"))
		s = h.Apply("create with a real alert", s, cfg(a))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: an alert that was deleted", s, cfg(gone))
		s = h.Apply("update: an id that is no uuid", s, cfg("not-a-uuid"))
		h.Plan("plan an alert id that is not known yet", s, cfg(unknownJSON{}))
		h.Destroy("destroy", s)
	})

	// The real API refuses to delete an alert that a flow refers to, so a referenced alert cannot
	// be deleted outside Terraform. The flow itself can.
	addAlertScenario("flow-deleted-outside-terraform", func(h *grHarness) {
		a := tfSeedLogsAlert(h, "tfbaseline-flow-outside-a")
		cfg := tfAlert("tfbaseline-flow-outside", "flow", tfFlow(tfFlowStage("Up To", float64(10), tfFlowGroup("OR", "AND", tfFlowDef(a)))))
		s := h.Apply("create", h.null(), cfg)
		h.alertBackend().forget(idOf(s))
		h.Refresh("refresh after the flow was deleted outside", s)
		h.Apply("update after the flow was deleted outside", s, with(cfg, j{"name": "tfbaseline-flow-outside-2"}))
		h.Destroy("destroy after the flow was deleted outside", s)
	})

	addAlertScenario("flow-group-by", func(h *grHarness) {
		// The API wants the group_by of a flow inside what its alerts have in common.
		a := tfSeedThresholdAlert(h, "tfbaseline-flow-group-by-a", "coralogix.metadata.applicationName", "coralogix.metadata.subsystemName")
		b := tfSeedThresholdAlert(h, "tfbaseline-flow-group-by-b", "coralogix.metadata.applicationName")
		plain := tfSeedLogsAlert(h, "tfbaseline-flow-group-by-plain")
		cfg := func(alert string, extra ...j) j {
			return tfAlert("tfbaseline-flow-group-by", "flow", tfFlow(tfFlowStage("Up To", float64(10), tfFlowGroup("OR", "AND", tfFlowDef(alert)))), extra...)
		}
		s := h.Apply("create without group_by", h.null(), cfg(a))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: group_by with one key of the alert", s, cfg(a, j{"group_by": l{"coralogix.metadata.applicationName"}}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: group_by with two keys of the alert", s, cfg(a, j{"group_by": l{"coralogix.metadata.applicationName", "coralogix.metadata.subsystemName"}}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: remove group_by", s, cfg(a))
		s = h.RefreshPlan("refresh after the removal", s)
		s = h.Apply("update: group_by = []", s, cfg(a, j{"group_by": l{}}))
		s = h.RefreshPlan("refresh after the empty list", s)
		s = h.Apply("update: group_by with a key that the alert does not have", s, cfg(a, j{"group_by": l{"coralogix.metadata.severity"}}))
		s = h.Apply("update: group_by with a key of the other alert only", s, cfg(b, j{"group_by": l{"coralogix.metadata.subsystemName"}}))
		s = h.Apply("update: an alert without group_by, and group_by on the flow", s, cfg(plain, j{"group_by": l{"coralogix.metadata.applicationName"}}))
		s = h.Apply("update: an alert without group_by, and no group_by on the flow", s, cfg(plain))
		h.RefreshPlan("refresh", s)
	})

	addAlertScenario("flow-enforce-suppression", func(h *grHarness) {
		a := tfSeedLogsAlert(h, "tfbaseline-flow-suppression-a")
		cfg := func(body j) j { return tfAlert("tfbaseline-flow-suppression", "flow", body) }
		stages := l{tfFlowStage("Up To", float64(10), tfFlowGroup("OR", "AND", tfFlowDef(a)))}
		s := h.Apply("create without enforce_suppression", h.null(), cfg(j{"stages": stages}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: true", s, cfg(j{"stages": stages, "enforce_suppression": true}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: remove it", s, cfg(j{"stages": stages}))
		s = h.RefreshPlan("refresh after the removal", s)
		s = h.Apply("update: false", s, cfg(j{"stages": stages, "enforce_suppression": false}))
		h.RefreshPlan("refresh", s)
	})

	addAlertScenario("flow-config-validation", func(h *grHarness) {
		stage := tfFlowStage("Up To", float64(10), tfFlowGroup("OR", "AND", tfFlowDef("11111111-1111-4111-8111-111111111111")))
		cfg := func(body j, extra ...j) j { return tfAlert("tfbaseline-flow-validation", "flow", body, extra...) }
		group := func(g j) j { return cfg(tfFlow(tfFlowStage("Up To", float64(10), g))) }
		h.validateSorted("no stages", cfg(j{}))
		h.validateSorted("a stage without timeframe_type", cfg(j{"stages": l{j{"flow_stages_groups": l{}}}}))
		h.validateSorted("a stage without flow_stages_groups", cfg(j{"stages": l{j{"timeframe_type": "Up To"}}}))
		h.validateSorted("timeframe_type outside the list", cfg(j{"stages": l{tfFlowStage("Within", float64(10))}}))
		h.validateSorted("lowercase timeframe_type", cfg(j{"stages": l{tfFlowStage("up to", float64(10))}}))
		h.validateSorted("the enum name as timeframe_type", cfg(j{"stages": l{tfFlowStage("TIMEFRAME_TYPE_UP_TO", float64(10))}}))
		h.validateSorted("a group without alert_defs", group(j{"alerts_op": "OR", "next_op": "AND"}))
		h.validateSorted("a group without alerts_op", group(j{"alert_defs": l{tfFlowDef("a")}, "next_op": "AND"}))
		h.validateSorted("a group without next_op", group(j{"alert_defs": l{tfFlowDef("a")}, "alerts_op": "OR"}))
		h.validateSorted("alerts_op outside the list", group(tfFlowGroup("XOR", "AND", tfFlowDef("a"))))
		h.validateSorted("next_op outside the list", group(tfFlowGroup("OR", "XOR", tfFlowDef("a"))))
		h.validateSorted("lowercase alerts_op", group(tfFlowGroup("or", "AND", tfFlowDef("a"))))
		h.validateSorted("lowercase next_op", group(tfFlowGroup("OR", "and", tfFlowDef("a"))))
		h.validateSorted("an alert_def without id", group(j{"alert_defs": l{j{"not": true}}, "alerts_op": "OR", "next_op": "AND"}))
		h.validateSorted("a null id", group(j{"alert_defs": l{j{"id": nil}}, "alerts_op": "OR", "next_op": "AND"}))
		h.validateSorted("a stage with a null timeframe_ms is fine", cfg(j{"stages": l{j{"timeframe_type": "Up To", "timeframe_ms": nil, "flow_stages_groups": l{}}}}))
		h.validateSorted("group_by is allowed", cfg(j{"stages": l{stage}}, j{"group_by": l{"coralogix.metadata.applicationName"}}))
		h.validateSorted("a null group_by is fine", cfg(j{"stages": l{stage}}, j{"group_by": nil}))
		h.validateSorted("two arms: flow and logs_immediate", j{"name": "tfbaseline-flow-validation", "type_definition": j{
			"flow": j{"stages": l{stage}}, "logs_immediate": j{},
		}})
	})
}

func init() {
	// Recorded: an alert that is made without phantomMode (a seeded one) reads phantomMode false,
	// and a flow reads enforceStrictSequencing false, which the provider has no attribute for.
	addAlertFill(func(props map[string]any) {
		setDefault(props, "phantomMode", false)
		if flow := atfObj(props, "flow"); flow != nil {
			setDefault(flow, "enforceStrictSequencing", false)
		}
	})
}

// atfCustom is the text of one custom validation error of the API, with a path.
func atfCustom(message string, path ...any) string {
	var sb strings.Builder
	sb.WriteString("[\n  {\n    \"code\": \"custom\",\n    \"message\": \"" + message + "\",\n    \"path\": [")
	for i, p := range path {
		if i > 0 {
			sb.WriteString(",")
		}
		if name, ok := p.(string); ok {
			sb.WriteString("\n      \"" + name + "\"")
		} else {
			sb.WriteString(fmt.Sprintf("\n      %v", p))
		}
	}
	sb.WriteString("\n    ]\n  }\n]")
	return sb.String()
}

func init() {
	addAlertCheck(atfCheckFlowStages)
	addAlertStatusCheck(atfStatusFlowTimeframe)
	addAlertStoreCheck(atfStoreFlowGroupBy)
}

// Recorded for flow: the type of a stage timeframe must not be UNSPECIFIED, a timeframe is
// not negative, and every stage after the first needs a timeframe above 0. A timeframe above
// MAX_SAFE_INTEGER fails as a 500, like the tracing latency.
func atfCheckFlowStages(props map[string]any) string {
	stages, _ := atfObj(props, "flow")["stages"].([]any)
	for i, raw := range stages {
		stage, _ := raw.(map[string]any)
		if stage["timeframeType"] == "TIMEFRAME_TYPE_UNSPECIFIED" {
			return atfCustom("Invalid timeframe type", atfPath("flow", "stages", i, "timeframeType")...)
		}
	}
	for i, raw := range stages {
		stage, _ := raw.(map[string]any)
		ms, _ := strconv.ParseFloat(fmt.Sprint(stage["timeframeMs"]), 64)
		if ms < 0 || (i > 0 && ms == 0) {
			return atfCustom("Timeframe ms must exist and be greater than 0 for all stages except the first one", atfPath("flow", "stages")...)
		}
	}
	return ""
}

func atfStatusFlowTimeframe(props map[string]any) (int, any) {
	stages, _ := atfObj(props, "flow")["stages"].([]any)
	for _, raw := range stages {
		stage, _ := raw.(map[string]any)
		if ms, err := strconv.ParseFloat(fmt.Sprint(stage["timeframeMs"]), 64); err == nil && ms > 9007199254740991 {
			return http.StatusInternalServerError, map[string]any{"code": 500, "message": "Internal Server Error: Error deserializing request: Value is larger than Number.MAX_SAFE_INTEGER"}
		}
	}
	return 0, nil
}

// Recorded: a flow may only group by keys that every alert it refers to has in common. The
// alerts are looked up in the fake. An alert that does not exist counts for nothing.
func atfStoreFlowGroupBy(f *alertFake, _ string, props map[string]any) (int, any) {
	flow := atfObj(props, "flow")
	keys, _ := props["groupByKeys"].([]any)
	if flow == nil || len(keys) == 0 {
		return 0, nil
	}
	common, found := atfCommonGroupByKeys(f, flow)
	if !found {
		return 0, nil
	}
	for _, k := range keys {
		if !slices.Contains(common, fmt.Sprint(k)) {
			wanted := make([]string, len(keys))
			for i, w := range keys {
				wanted[i] = fmt.Sprint(w)
			}
			return http.StatusBadRequest, map[string]any{"code": 400, "message": fmt.Sprintf(
				"Bad Request: Invalid alerts in flow alert type: \"Invalid group by keys. internal alerts intersection - [%s] flow alert group by keys [%s]\"",
				strings.Join(common, ", "), strings.Join(wanted, ", "))}
		}
	}
	return 0, nil
}

// atfCommonGroupByKeys intersects the group-by keys of the stored alerts that a flow refers to.
// The second result is false when no referred alert exists.
func atfCommonGroupByKeys(f *alertFake, flow map[string]any) ([]string, bool) {
	var common []string
	first := true
	stages, _ := flow["stages"].([]any)
	for _, rawStage := range stages {
		stage, _ := rawStage.(map[string]any)
		groups, _ := atfObj(stage, "flowStagesGroups")["groups"].([]any)
		for _, rawGroup := range groups {
			group, _ := rawGroup.(map[string]any)
			defs, _ := group["alertDefs"].([]any)
			for _, rawDef := range defs {
				def, _ := rawDef.(map[string]any)
				id, _ := def["id"].(string)
				stored, found := f.alerts[id]
				if !found {
					continue
				}
				have := map[string]bool{}
				var ordered []string
				storedKeys, _ := atfObj(stored, "alertDefProperties")["groupByKeys"].([]any)
				for _, k := range storedKeys {
					have[fmt.Sprint(k)] = true
					ordered = append(ordered, fmt.Sprint(k))
				}
				if first {
					common, first = ordered, false
					continue
				}
				var kept []string
				for _, k := range common {
					if have[k] {
						kept = append(kept, k)
					}
				}
				common = kept
			}
		}
	}
	return common, !first
}

// --- analytics_immediate and analytics_threshold (preview types) ---

func tfAnalyticsQuery(q string) j { return j{"query": q} }

// tfGroupedQuery returns the columns app, sub, and c.
const tfGroupedQuery = "source logs | groupby $l.applicationName as app, $l.subsystemName as sub aggregate count() as c"

func tfImmediateBody(extra ...j) j {
	out := j{"dataprime_query": tfAnalyticsQuery("source logs | count"), "timeframe_minutes": float64(10)}
	for _, e := range extra {
		out = with(out, e)
	}
	return out
}

func tfThresholdRule(threshold float64, priority string) j {
	out := j{"condition": j{"threshold": threshold}, "override": j{}}
	if priority != "" {
		out["override"] = j{"priority": priority}
	}
	return out
}

func tfThresholdBody(extra ...j) j {
	out := j{
		"dataprime_query":   tfAnalyticsQuery("source logs | count as c"),
		"rules":             l{tfThresholdRule(5, "P3")},
		"operator":          "MORE_THAN",
		"target_column":     "c",
		"timeframe_minutes": float64(10),
	}
	for _, e := range extra {
		out = with(out, e)
	}
	return out
}

func init() {
	addAlertScenario("analytics-immediate-lifecycle", func(h *grHarness) {
		cfg := func(body j, extra ...j) j {
			return tfAlert("tfbaseline-analytics-immediate", "analytics_immediate", body, extra...)
		}
		s := h.Apply("create with the required attributes", h.null(), cfg(tfImmediateBody()))
		s = h.RefreshPlan("refresh", s)
		imported := h.Import("import by id", idOf(s))
		h.Plan("plan the same config on the imported state", imported, cfg(tfImmediateBody()))
		s = h.Apply("update: query and timeframe_minutes", s, cfg(tfImmediateBody(j{"dataprime_query": tfAnalyticsQuery("source logs | filter $m.severity == 'Error' | count"), "timeframe_minutes": float64(30)})))
		s = h.RefreshPlan("refresh after update", s)
		s = h.Apply("update: rename", s, cfg(tfImmediateBody(), j{"name": "tfbaseline-analytics-immediate-2"}))
		h.Destroy("destroy", s)
	})

	addAlertScenario("analytics-threshold-lifecycle", func(h *grHarness) {
		cfg := func(body j, extra ...j) j {
			return tfAlert("tfbaseline-analytics-threshold", "analytics_threshold", body, extra...)
		}
		s := h.Apply("create with the required attributes", h.null(), cfg(tfThresholdBody()))
		s = h.RefreshPlan("refresh", s)
		imported := h.Import("import by id", idOf(s))
		h.Plan("plan the same config on the imported state", imported, cfg(tfThresholdBody()))
		s = h.Apply("update: query, target_column, and timeframe_minutes", s, cfg(tfThresholdBody(j{
			"dataprime_query": tfAnalyticsQuery("source logs | count as errors"), "target_column": "errors", "timeframe_minutes": float64(30),
		})))
		s = h.RefreshPlan("refresh after update", s)
		h.Destroy("destroy", s)
	})
}

func init() {
	for _, kind := range []string{"immediate", "threshold"} {
		kind := kind
		arm := "analytics_" + kind
		name := "tfbaseline-analytics-" + kind
		// body builds the type body of the kind with the shared attributes.
		body := func(extra ...j) j {
			if kind == "immediate" {
				return tfImmediateBody(extra...)
			}
			return tfThresholdBody(extra...)
		}
		cfg := func(b j, extra ...j) j { return tfAlert(name, arm, b, extra...) }

		addAlertScenario("analytics-"+kind+"-full", func(h *grHarness) {
			full := body(j{
				"dataprime_query":          tfAnalyticsQuery(tfGroupedQuery),
				"use_rows_as_permutations": true,
				"custom_evaluation_delay":  float64(120000),
				"timeframe_minutes":        float64(45),
				"no_data_policy":           j{"state": "ALERTING", "auto_retire_seconds": float64(3600)},
			})
			top := j{"description": "full", "priority": "P2", "enabled": false, "group_by": l{"app"}}
			s := h.Apply("create with every optional attribute", h.null(), cfg(full, top))
			s = h.RefreshPlan("refresh", s)
			imported := h.Import("import by id", idOf(s))
			h.Plan("plan the same config on the imported state", imported, cfg(full, top))
			s = h.Apply("update: change every value", s, cfg(body(j{
				"dataprime_query":          tfAnalyticsQuery("source logs | filter $m.severity == 'Warning' | groupby $l.applicationName as app, $l.subsystemName as sub aggregate count() as c"),
				"use_rows_as_permutations": false,
				"custom_evaluation_delay":  float64(60000),
				"timeframe_minutes":        float64(15),
				"no_data_policy":           j{"state": "OK", "auto_retire_seconds": float64(7200)},
			}), j{"description": "changed", "priority": "P4", "enabled": true, "group_by": l{"sub", "app"}}))
			s = h.RefreshPlan("refresh after the change", s)
			s = h.Apply("update: remove every optional attribute", s, cfg(body()))
			s = h.RefreshPlan("refresh after the removal", s)
			s = h.Apply("update: set them again", s, cfg(full, top))
			h.Destroy("destroy", s)
		})

		addAlertScenario("analytics-"+kind+"-no-data-policy", func(h *grHarness) {
			s := h.Apply("create without no_data_policy", h.null(), cfg(body()))
			s = h.RefreshPlan("refresh", s)
			for _, state := range []string{"ALERTING", "KEEP_LAST", "NO_DATA", "OK", "UNSPECIFIED"} {
				s = h.Apply("state "+state+" with auto_retire_seconds", s, cfg(body(j{"no_data_policy": j{"state": state, "auto_retire_seconds": float64(3600)}})))
				s = h.RefreshPlan("refresh "+state, s)
			}
			s = h.Apply("update: state only", s, cfg(body(j{"no_data_policy": j{"state": "KEEP_LAST"}})))
			s = h.RefreshPlan("refresh", s)
			s = h.Apply("update: auto_retire_seconds only", s, cfg(body(j{"no_data_policy": j{"auto_retire_seconds": float64(1800)}})))
			s = h.RefreshPlan("refresh", s)
			s = h.Apply("update: remove no_data_policy", s, cfg(body()))
			s = h.RefreshPlan("refresh after the removal", s)
			s = h.Apply("update: auto_retire_seconds that is no multiple of 60", s, cfg(body(j{"no_data_policy": j{"state": "OK", "auto_retire_seconds": float64(61)}})))
			s = h.Apply("update: auto_retire_seconds 60", s, cfg(body(j{"no_data_policy": j{"state": "OK", "auto_retire_seconds": float64(60)}})))
			s = h.Apply("update: auto_retire_seconds 0", s, cfg(body(j{"no_data_policy": j{"state": "OK", "auto_retire_seconds": float64(0)}})))
			s = h.Apply("update: auto_retire_seconds below 60", s, cfg(body(j{"no_data_policy": j{"state": "OK", "auto_retire_seconds": float64(30)}})))
			s = h.Apply("update: a negative auto_retire_seconds", s, cfg(body(j{"no_data_policy": j{"state": "OK", "auto_retire_seconds": float64(-60)}})))
			s = h.Apply("update: auto_retire_seconds only, 120", s, cfg(body(j{"no_data_policy": j{"auto_retire_seconds": float64(120)}})))
			s = h.Apply("update: remove no_data_policy again", s, cfg(body()))
			h.RefreshPlan("refresh", s)
		})

		addAlertScenario("analytics-"+kind+"-optional-scalars", func(h *grHarness) {
			s := h.Apply("create with none", h.null(), cfg(body()))
			s = h.RefreshPlan("refresh", s)
			s = h.Apply("use_rows_as_permutations true", s, cfg(body(j{"use_rows_as_permutations": true})))
			s = h.RefreshPlan("refresh", s)
			s = h.Apply("use_rows_as_permutations false", s, cfg(body(j{"use_rows_as_permutations": false})))
			s = h.RefreshPlan("refresh", s)
			s = h.Apply("use_rows_as_permutations removed", s, cfg(body()))
			s = h.RefreshPlan("refresh after the removal", s)
			s = h.Apply("custom_evaluation_delay 0", s, cfg(body(j{"custom_evaluation_delay": float64(0)})))
			s = h.RefreshPlan("refresh", s)
			s = h.Apply("custom_evaluation_delay 1", s, cfg(body(j{"custom_evaluation_delay": float64(1)})))
			s = h.Apply("custom_evaluation_delay 10800000", s, cfg(body(j{"custom_evaluation_delay": float64(10800000)})))
			s = h.RefreshPlan("refresh", s)
			s = h.Apply("custom_evaluation_delay above the largest value", s, cfg(body(j{"custom_evaluation_delay": float64(10800001)})))
			s = h.Apply("custom_evaluation_delay removed", s, cfg(body()))
			s = h.RefreshPlan("refresh after the removal", s)
			s = h.Apply("timeframe_minutes 1", s, cfg(body(j{"timeframe_minutes": float64(1)})))
			s = h.Apply("timeframe_minutes 1440", s, cfg(body(j{"timeframe_minutes": float64(1440)})))
			s = h.Apply("timeframe_minutes 100000", s, cfg(body(j{"timeframe_minutes": float64(100000)})))
			s = h.RefreshPlan("refresh", s)
			s = h.Apply("timeframe_minutes 2147483647", s, cfg(body(j{"timeframe_minutes": float64(2147483647)})))
			// The server keeps an evaluation period of 10 minutes that the provider cannot set.
			h.Apply("update: timeframe_minutes 9, below that period", s, cfg(body(j{"timeframe_minutes": float64(9)})))
			h.Apply("create with timeframe_minutes 9 on a new alert", h.null(), cfg(body(j{"timeframe_minutes": float64(9)})))
			h.Apply("create with timeframe_minutes 1 on a new alert", h.null(), cfg(body(j{"timeframe_minutes": float64(1)})))
		})

		addAlertScenario("analytics-"+kind+"-group-by", func(h *grHarness) {
			s := h.Apply("create without group_by", h.null(), cfg(body()))
			s = h.RefreshPlan("refresh", s)
			s = h.Apply("update: group_by with one key", s, cfg(body(), j{"group_by": l{"coralogix.metadata.applicationName"}}))
			s = h.RefreshPlan("refresh", s)
			s = h.Apply("update: group_by with two keys", s, cfg(body(), j{"group_by": l{"coralogix.metadata.applicationName", "coralogix.metadata.subsystemName"}}))
			s = h.Apply("update: remove group_by", s, cfg(body()))
			s = h.RefreshPlan("refresh after the removal", s)
			s = h.Apply("update: group_by = []", s, cfg(body(), j{"group_by": l{}}))
			s = h.RefreshPlan("refresh after the empty list", s)
			s = h.Apply("update: group_by with one key again", s, cfg(body(), j{"group_by": l{"coralogix.metadata.applicationName"}}))
			s = h.Apply("update: group_by = []", s, cfg(body(), j{"group_by": l{}}))
			// With use_rows_as_permutations the keys must be columns of the query result.
			grouped := j{"dataprime_query": tfAnalyticsQuery(tfGroupedQuery), "use_rows_as_permutations": true}
			s = h.Apply("update: use_rows_as_permutations with a key that the query returns", s, cfg(body(grouped), j{"group_by": l{"app"}}))
			s = h.RefreshPlan("refresh", s)
			s = h.Apply("update: use_rows_as_permutations with two keys that the query returns", s, cfg(body(grouped), j{"group_by": l{"app", "sub"}}))
			s = h.Apply("update: use_rows_as_permutations with a key that the query does not return", s, cfg(body(grouped), j{"group_by": l{"app", "zzz"}}))
			s = h.Apply("update: use_rows_as_permutations with two keys that the query does not return", s, cfg(body(grouped), j{"group_by": l{"zzz", "yyy"}}))
			s = h.Apply("update: use_rows_as_permutations false with a key that the query does not return", s, cfg(body(j{"dataprime_query": tfAnalyticsQuery(tfGroupedQuery), "use_rows_as_permutations": false}), j{"group_by": l{"zzz"}}))
			s = h.Apply("update: use_rows_as_permutations with no group_by", s, cfg(body(grouped)))
			h.RefreshPlan("refresh", s)
		})

		addAlertScenario("analytics-"+kind+"-query", func(h *grHarness) {
			s := h.Apply("create with a query", h.null(), cfg(body()))
			s = h.Apply("update: a query with a newline and quotes", s, cfg(body(j{"dataprime_query": tfAnalyticsQuery("source logs\n| filter $d.msg == \"a 'b'\"\n| count as c")})))
			s = h.RefreshPlan("refresh", s)
			s = h.Apply("update: a query with a trailing space", s, cfg(body(j{"dataprime_query": tfAnalyticsQuery("source logs | count as c ")})))
			s = h.RefreshPlan("refresh", s)
			s = h.Apply("update: a query that is no dataprime", s, cfg(body(j{"dataprime_query": tfAnalyticsQuery("this is not a query")})))
			s = h.RefreshPlan("refresh", s)
			s = h.Apply("update: a query of one character", s, cfg(body(j{"dataprime_query": tfAnalyticsQuery("x")})))
			h.RefreshPlan("refresh", s)
		})
	}

	addAlertScenario("analytics-threshold-operators", func(h *grHarness) {
		s := h.null()
		for _, op := range []string{"MORE_THAN", "LESS_THAN", "MORE_THAN_OR_EQUALS", "LESS_THAN_OR_EQUALS", "EQUALS", "NOT_EQUALS"} {
			s = h.Apply("operator "+op, s, tfAlert("tfbaseline-analytics-threshold-ops", "analytics_threshold", tfThresholdBody(j{"operator": op})))
			s = h.RefreshPlan("refresh "+op, s)
		}
		h.Apply("update: target_column of 255 characters", s, tfAlert("tfbaseline-analytics-threshold-ops", "analytics_threshold", tfThresholdBody(j{"target_column": strings255()})))
		h.Apply("update: target_column of one character", s, tfAlert("tfbaseline-analytics-threshold-ops", "analytics_threshold", tfThresholdBody(j{"target_column": "c"})))
		h.Apply("update: a target_column that the query has not", s, tfAlert("tfbaseline-analytics-threshold-ops", "analytics_threshold", tfThresholdBody(j{"target_column": "missing"})))
		h.Apply("update: an empty target_column", s, tfAlert("tfbaseline-analytics-threshold-ops", "analytics_threshold", tfThresholdBody(j{"target_column": ""})))
	})

	addAlertScenario("analytics-threshold-rules", func(h *grHarness) {
		cfg := func(rules l, extra ...j) j {
			return tfAlert("tfbaseline-analytics-threshold-rules", "analytics_threshold", tfThresholdBody(j{"rules": rules}), extra...)
		}
		s := h.Apply("create with one rule", h.null(), cfg(l{tfThresholdRule(1, "P1")}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: three ordered rules", s, cfg(l{tfThresholdRule(10, "P4"), tfThresholdRule(20, "P3"), tfThresholdRule(30, "P2")}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: the rules in the other order", s, cfg(l{tfThresholdRule(30, "P2"), tfThresholdRule(20, "P3"), tfThresholdRule(10, "P4")}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: five rules", s, cfg(l{tfThresholdRule(1, "P1"), tfThresholdRule(2, "P2"), tfThresholdRule(3, "P3"), tfThresholdRule(4, "P4"), tfThresholdRule(5, "P5")}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: six rules", s, cfg(l{tfThresholdRule(1, "P1"), tfThresholdRule(2, "P2"), tfThresholdRule(3, "P3"), tfThresholdRule(4, "P4"), tfThresholdRule(5, "P5"), tfThresholdRule(6, "P1")}))
		s = h.Apply("update: the same threshold twice", s, cfg(l{tfThresholdRule(7, "P1"), tfThresholdRule(7, "P2")}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: a zero, a fraction, and a negative threshold", s, cfg(l{tfThresholdRule(0, "P1"), tfThresholdRule(0.5, "P2"), tfThresholdRule(-3, "P3")}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: a very large threshold", s, cfg(l{tfThresholdRule(1e15, "P1")}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: rules = []", s, cfg(l{}))
		s = h.Apply("update: rules without an override priority", s, cfg(l{tfThresholdRule(1, "")}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: rules without an override priority, and a top-level priority", s, cfg(l{tfThresholdRule(1, "")}, j{"priority": "P2"}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: the top-level priority changes", s, cfg(l{tfThresholdRule(1, "")}, j{"priority": "P4"}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: the top-level priority is removed", s, cfg(l{tfThresholdRule(1, "")}))
		h.RefreshPlan("refresh after the removal", s)
	})

	addAlertScenario("analytics-threshold-priorities", func(h *grHarness) {
		s := h.null()
		for _, p := range []string{"P1", "P2", "P3", "P4", "P5"} {
			s = h.Apply("override priority "+p, s, tfAlert("tfbaseline-analytics-threshold-prio", "analytics_threshold", tfThresholdBody(j{"rules": l{tfThresholdRule(1, p)}})))
		}
		h.RefreshPlan("refresh", s)
	})

	addAlertScenario("analytics-immediate-priority", func(h *grHarness) {
		s := h.null()
		for _, p := range []string{"P1", "P2", "P3", "P4", "P5"} {
			s = h.Apply("priority "+p, s, tfAlert("tfbaseline-analytics-immediate-prio", "analytics_immediate", tfImmediateBody(), j{"priority": p}))
		}
		h.RefreshPlan("refresh", s)
	})

	addAlertScenario("analytics-immediate-config-validation", func(h *grHarness) {
		cfg := func(body j, extra ...j) j {
			return tfAlert("tfbaseline-analytics-validation", "analytics_immediate", body, extra...)
		}
		h.validateSorted("no body", cfg(j{}))
		h.validateSorted("no dataprime_query", cfg(j{"timeframe_minutes": float64(10)}))
		h.validateSorted("no timeframe_minutes", cfg(j{"dataprime_query": tfAnalyticsQuery("source logs")}))
		h.validateSorted("a dataprime_query without query", cfg(j{"dataprime_query": j{}, "timeframe_minutes": float64(10)}))
		h.validateSorted("an empty query", cfg(tfImmediateBody(j{"dataprime_query": tfAnalyticsQuery("")})))
		h.validateSorted("a query of 65535 characters is fine", cfg(tfImmediateBody(j{"dataprime_query": tfAnalyticsQuery(strings.Repeat("a", 65535))})))
		h.validateSorted("a query of 65536 characters", cfg(tfImmediateBody(j{"dataprime_query": tfAnalyticsQuery(strings.Repeat("a", 65536))})))
		h.validateSorted("timeframe_minutes 0", cfg(tfImmediateBody(j{"timeframe_minutes": float64(0)})))
		h.validateSorted("timeframe_minutes negative", cfg(tfImmediateBody(j{"timeframe_minutes": float64(-1)})))
		h.validateSorted("custom_evaluation_delay negative", cfg(tfImmediateBody(j{"custom_evaluation_delay": float64(-1)})))
		h.validateSorted("custom_evaluation_delay 0 is fine", cfg(tfImmediateBody(j{"custom_evaluation_delay": float64(0)})))
		h.validateSorted("an empty no_data_policy", cfg(tfImmediateBody(j{"no_data_policy": j{}})))
		h.validateSorted("a no_data_policy with null members", cfg(tfImmediateBody(j{"no_data_policy": j{"state": nil, "auto_retire_seconds": nil}})))
		h.validateSorted("no_data_policy state outside the list", cfg(tfImmediateBody(j{"no_data_policy": j{"state": "BROKEN"}})))
		h.validateSorted("no_data_policy state lowercase", cfg(tfImmediateBody(j{"no_data_policy": j{"state": "ok"}})))
		h.validateSorted("no_data_policy state UNSPECIFIED warns", cfg(tfImmediateBody(j{"no_data_policy": j{"state": "UNSPECIFIED"}})))
		h.validateSorted("a no_data_policy with an unknown state is fine", cfg(tfImmediateBody(j{"no_data_policy": j{"state": unknownJSON{}}})))
		h.validateSorted("group_by is allowed", cfg(tfImmediateBody(), j{"group_by": l{"coralogix.metadata.applicationName"}}))
		h.validateSorted("a null group_by is fine", cfg(tfImmediateBody(), j{"group_by": nil}))
		h.validateSorted("two arms: analytics_immediate and analytics_threshold", j{"name": "tfbaseline-analytics-validation", "type_definition": j{
			"analytics_immediate": tfImmediateBody(), "analytics_threshold": tfThresholdBody(),
		}})
		h.validateSorted("two arms: analytics_immediate and logs_immediate", j{"name": "tfbaseline-analytics-validation", "type_definition": j{
			"analytics_immediate": tfImmediateBody(), "logs_immediate": j{},
		}})
	})

	addAlertScenario("analytics-threshold-config-validation", func(h *grHarness) {
		cfg := func(body j, extra ...j) j {
			return tfAlert("tfbaseline-analytics-validation", "analytics_threshold", body, extra...)
		}
		without := func(key string) j {
			out := tfThresholdBody()
			delete(out, key)
			return out
		}
		h.validateSorted("no body", cfg(j{}))
		for _, key := range []string{"dataprime_query", "rules", "operator", "target_column", "timeframe_minutes"} {
			h.validateSorted("no "+key, cfg(without(key)))
		}
		h.validateSorted("an empty query", cfg(tfThresholdBody(j{"dataprime_query": tfAnalyticsQuery("")})))
		h.validateSorted("operator outside the list", cfg(tfThresholdBody(j{"operator": "GREATER_THAN"})))
		h.validateSorted("lowercase operator", cfg(tfThresholdBody(j{"operator": "more_than"})))
		h.validateSorted("the enum name as operator", cfg(tfThresholdBody(j{"operator": "ANALYTICS_THRESHOLD_OPERATOR_LESS_THAN"})))
		h.validateSorted("empty rules", cfg(tfThresholdBody(j{"rules": l{}})))
		h.validateSorted("six rules", cfg(tfThresholdBody(j{"rules": l{
			tfThresholdRule(1, "P1"), tfThresholdRule(2, "P1"), tfThresholdRule(3, "P1"), tfThresholdRule(4, "P1"), tfThresholdRule(5, "P1"), tfThresholdRule(6, "P1"),
		}})))
		h.validateSorted("five rules are fine", cfg(tfThresholdBody(j{"rules": l{
			tfThresholdRule(1, "P1"), tfThresholdRule(2, "P1"), tfThresholdRule(3, "P1"), tfThresholdRule(4, "P1"), tfThresholdRule(5, "P1"),
		}})))
		h.validateSorted("a rule without condition", cfg(tfThresholdBody(j{"rules": l{j{"override": j{"priority": "P1"}}}})))
		h.validateSorted("a rule without override", cfg(tfThresholdBody(j{"rules": l{j{"condition": j{"threshold": float64(1)}}}})))
		h.validateSorted("a condition without threshold", cfg(tfThresholdBody(j{"rules": l{j{"condition": j{}, "override": j{}}}})))
		h.validateSorted("override priority outside the list", cfg(tfThresholdBody(j{"rules": l{tfThresholdRule(1, "P6")}})))
		h.validateSorted("lowercase override priority", cfg(tfThresholdBody(j{"rules": l{tfThresholdRule(1, "p1")}})))
		h.validateSorted("target_column of 256 characters", cfg(tfThresholdBody(j{"target_column": strings255() + "x"})))
		h.validateSorted("target_column of 255 characters is fine", cfg(tfThresholdBody(j{"target_column": strings255()})))
		h.validateSorted("an empty target_column is fine for the schema", cfg(tfThresholdBody(j{"target_column": ""})))
		h.validateSorted("timeframe_minutes 0", cfg(tfThresholdBody(j{"timeframe_minutes": float64(0)})))
		h.validateSorted("custom_evaluation_delay negative", cfg(tfThresholdBody(j{"custom_evaluation_delay": float64(-1)})))
		h.validateSorted("an empty no_data_policy", cfg(tfThresholdBody(j{"no_data_policy": j{}})))
		h.validateSorted("no_data_policy state outside the list", cfg(tfThresholdBody(j{"no_data_policy": j{"state": "BROKEN"}})))
		h.validateSorted("no_data_policy state UNSPECIFIED warns", cfg(tfThresholdBody(j{"no_data_policy": j{"state": "UNSPECIFIED"}})))
		h.validateSorted("group_by is allowed", cfg(tfThresholdBody(), j{"group_by": l{"coralogix.metadata.applicationName"}}))
		h.validateSorted("priority gets a deprecation warning next to rules", cfg(tfThresholdBody(), j{"priority": "P2"}))
	})
}

func strings255() string { return strings.Repeat("c", 255) }

// atfQueryColumns reads the columns that a DataPrime query returns, for the queries of these
// scenarios: count, and groupby ... aggregate. It returns nil for a query it does not know.
func atfQueryColumns(query string) []string {
	var cols []string
	alias := func(item, fallback string) string {
		item = strings.TrimSpace(item)
		if _, after, ok := strings.Cut(item, " as "); ok {
			return strings.TrimSpace(after)
		}
		return fallback
	}
	for _, segment := range strings.Split(query, "|") {
		segment = strings.TrimSpace(segment)
		switch {
		case strings.HasPrefix(segment, "count"):
			cols = []string{alias(segment, "_count")}
		case strings.HasPrefix(segment, "groupby"):
			groups, aggregates, _ := strings.Cut(strings.TrimPrefix(segment, "groupby"), " aggregate ")
			cols = nil
			for _, g := range strings.Split(groups, ",") {
				name := strings.TrimSpace(g)
				cols = append(cols, alias(name, name[strings.LastIndex(name, ".")+1:]))
			}
			for _, a := range strings.Split(aggregates, ",") {
				name := strings.TrimSpace(a)
				cols = append(cols, alias(name, "_"+strings.Split(name, "(")[0]))
			}
		}
	}
	return cols
}

func init() {
	// Recorded: with use_rows_as_permutations, every group_by key must be a column of the query.
	// The message lists the keys that are missing, and the columns that the query returns.
	addAlertCheck(func(props map[string]any) string {
		keys, _ := props["groupByKeys"].([]any)
		for _, arm := range []string{"analyticsImmediate", "analyticsThreshold"} {
			body := atfObj(props, arm)
			if body == nil || body["useRowsAsPermutations"] != true || len(keys) == 0 {
				continue
			}
			columns := atfQueryColumns(fmt.Sprint(atfObj(body, "dataprimeQuery")["query"]))
			if columns == nil {
				continue
			}
			if target, ok := body["targetColumn"].(string); ok {
				// The column that the threshold reads is not a column to group by.
				columns = slices.DeleteFunc(slices.Clone(columns), func(c string) bool { return c == target })
			}
			var missing []string
			for _, k := range keys {
				if !slices.Contains(columns, fmt.Sprint(k)) {
					missing = append(missing, fmt.Sprint(k))
				}
			}
			if len(missing) > 0 {
				return fmt.Sprintf("groupByKeys [%s] are not returned by the query. Available: [%s].", strings.Join(missing, ", "), strings.Join(columns, ", "))
			}
		}
		return ""
	})
}

// The API keeps the evaluation period of an analytics alert only when the request sets it: a
// Create or Replace that omits it stores no period, so a read returns none. The period the alert
// runs with is the smaller of 10 and its timeframe, and it is not part of the readable resource.
// The fake therefore stores what a request sends and adds nothing.

// atfIssue is the text of one validation error of the API. fields holds the keys and the JSON
// values of the issue before its path, in order.
func atfIssue(fields [][2]string, path ...any) string {
	var sb strings.Builder
	sb.WriteString("[\n  {")
	for i, f := range fields {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString("\n    \"" + f[0] + "\": " + f[1])
	}
	sb.WriteString(",\n    \"path\": [")
	for i, p := range path {
		if i > 0 {
			sb.WriteString(",")
		}
		if name, ok := p.(string); ok {
			sb.WriteString("\n      \"" + name + "\"")
		} else {
			sb.WriteString(fmt.Sprintf("\n      %v", p))
		}
	}
	sb.WriteString("\n    ]\n  }\n]")
	return sb.String()
}

func init() {
	// Recorded for both analytics types: the limits of the numbers, and the check of the query.
	addAlertCheck(func(props map[string]any) string {
		for _, arm := range []string{"analyticsImmediate", "analyticsThreshold"} {
			body := atfObj(props, arm)
			if body == nil {
				continue
			}
			if delay, ok := body["evaluationDelayMs"].(float64); ok && delay > 10800000 {
				return atfIssue([][2]string{{"code", `"too_big"`}, {"maximum", "10800000"}, {"type", `"number"`}, {"inclusive", "true"}, {"exact", "false"}, {"message", `"Number must be less than or equal to 10800000"`}},
					atfPath(arm, "evaluationDelayMs")...)
			}
			if policy := atfObj(body, "noDataPolicy"); policy != nil {
				if retire, ok := policy["autoRetireSeconds"].(float64); ok {
					if retire <= 0 {
						return atfIssue([][2]string{{"code", `"too_small"`}, {"minimum", "0"}, {"type", `"number"`}, {"inclusive", "false"}, {"exact", "false"}, {"message", `"Number must be greater than 0"`}},
							atfPath(arm, "noDataPolicy", "autoRetireSeconds")...)
					}
					if math.Mod(retire, 60) != 0 {
						return atfIssue([][2]string{{"code", `"not_multiple_of"`}, {"multipleOf", "60"}, {"message", `"Number must be a multiple of 60"`}},
							atfPath(arm, "noDataPolicy", "autoRetireSeconds")...)
					}
				}
			}
			// The query is compiled when the rows are permutations. "count as" is no DataPrime.
			if body["useRowsAsPermutations"] == true {
				query := fmt.Sprint(atfObj(body, "dataprimeQuery")["query"])
				if at := strings.Index(query, "count as "); at >= 0 {
					return fmt.Sprintf("The query is not valid DataPrime: 0:%d expected keyword 'into'", at+len("count "))
				}
			}
		}
		return ""
	})
}
