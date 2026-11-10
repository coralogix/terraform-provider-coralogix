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
	"fmt"
	"math"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// Scenarios of the alert types metric_threshold, metric_anomaly, and slo_threshold. The
// notification group, incidents settings, schedule, case settings, access policy, and labels keep
// their defaults here: alert_baseline_common_test.go owns them.
//
// An slo_threshold alert needs an SLO that exists, and a baseline must not create one. The
// slo-threshold scenarios therefore cover the configuration offline (Validate and Plan) and send
// real requests with SLO ids that do not exist. What the API answers is recorded.

const (
	msPromql = `sum(rate(tfbaseline_requests_total[5m]))`
	// The API infers the group-by keys of a metric alert from the by-clause of its query.
	msPromqlPod   = `sum by (pod) (rate(tfbaseline_requests_total[5m]))`
	msPromqlNode  = `max by (node) (tfbaseline_queue_depth)`
	msPromqlPodNs = `sum by (pod, namespace) (rate(tfbaseline_requests_total[5m]))`
)

// msRefresh and msRefreshPlan read a resource that a failed create did not leave behind as no-ops,
// because Read of a null state would panic and never happens in Terraform.
func msRefresh(h *grHarness, name string, s tftypes.Value) tftypes.Value {
	if s.IsNull() {
		return s
	}
	return h.Refresh(name, s)
}

func msRefreshPlan(h *grHarness, name string, s tftypes.Value) tftypes.Value {
	if s.IsNull() {
		return s
	}
	return h.RefreshPlan(name, s)
}

var (
	// msWindows are the values of the enum of_the_last, in schema order.
	msWindows = []string{
		"1_MINUTE", "5_MINUTES", "10_MINUTES", "15_MINUTES", "20_MINUTES", "30_MINUTES",
		"1_HOUR", "2_HOURS", "4_HOURS", "6_HOURS", "12_HOURS", "24_HOURS", "36_HOURS",
	}
	msPriorities = []string{"P1", "P2", "P3", "P4", "P5"}
)

// msAlert is a name and a type definition, plus top-level attributes.
func msAlert(name string, typeDefinition j, top ...j) j {
	out := j{"name": name, "type_definition": typeDefinition}
	for _, t := range top {
		out = with(out, t)
	}
	return out
}

// ---- metric_threshold ----

func mtRule(condition string, threshold, forOverPct float64, window string, override j) j {
	if override == nil {
		override = j{}
	}
	return j{
		"condition": j{"condition_type": condition, "threshold": threshold, "for_over_pct": forOverPct, "of_the_last": window},
		"override":  override,
	}
}

// mtArm is the smallest valid metric_threshold; arm values replace its attributes, and a nil value
// removes one.
func mtArm(arm j) j {
	out := j{
		"metric_filter":  j{"promql": msPromql},
		"missing_values": j{"replace_with_zero": true},
		"rules":          l{mtRule("MORE_THAN", 10, 50, "10_MINUTES", nil)},
	}
	if arm != nil {
		out = with(out, arm)
	}
	return out
}

func mtAlert(name string, arm j, top ...j) j {
	return msAlert(name, j{"metric_threshold": mtArm(arm)}, top...)
}

// mtFullArm sets every optional attribute of the arm.
var mtFullArm = j{
	"metric_filter":           j{"promql": msPromqlPod},
	"custom_evaluation_delay": float64(1000),
	"missing_values":          j{"min_non_null_values_pct": float64(60)},
	"no_data_policy":          j{"state": "KEEP_LAST", "auto_retire_seconds": float64(600)},
	"undetected_values_management": j{
		"trigger_undetected_values": true,
		"auto_retire_timeframe":     "5_MINUTES",
	},
	"rules": l{
		mtRule("LESS_THAN", 5, 100, "15_MINUTES", j{"priority": "P2"}),
		mtRule("LESS_THAN", 1, 10, "25m", j{"priority": "P1"}),
	},
}

func mtFullTop() j {
	return j{"description": "metric threshold baseline", "priority": "P3", "enabled": false, "group_by": l{"pod"}}
}

// ---- metric_anomaly ----

func maRule(condition string, threshold, forOverPct, minNonNullPct float64, window string) j {
	return j{"condition": j{
		"condition_type":          condition,
		"threshold":               threshold,
		"for_over_pct":            forOverPct,
		"min_non_null_values_pct": minNonNullPct,
		"of_the_last":             window,
	}}
}

func maArm(arm j) j {
	out := j{
		"metric_filter": j{"promql": msPromql},
		"rules":         l{maRule("MORE_THAN", 10, 50, 50, "10_MINUTES")},
	}
	if arm != nil {
		out = with(out, arm)
	}
	return out
}

func maAlert(name string, arm j, top ...j) j {
	return msAlert(name, j{"metric_anomaly": maArm(arm)}, top...)
}

// ---- slo_threshold ----

const (
	// sloMissingID is a well-formed SLO id that no SLO has.
	sloMissingID = "00000000-0000-4000-8000-0000000000aa"
	// sloMissingText is no id at all.
	sloMissingText = "tfbaseline-no-such-slo"
)

func sloRule(threshold float64, override j) j {
	if override == nil {
		override = j{}
	}
	return j{"condition": j{"threshold": threshold}, "override": override}
}

func sloErrorBudget(id string, rules ...any) j {
	return j{
		"slo_definition": j{"slo_id": id},
		"error_budget":   j{"rules": l(rules)},
	}
}

func sloBurnRate(id string, kind string, hours float64, rules ...any) j {
	return j{
		"slo_definition": j{"slo_id": id},
		"burn_rate": j{
			kind:    j{"time_duration": j{"duration": hours, "unit": "HOURS"}},
			"rules": l(rules),
		},
	}
}

func sloAlert(name string, arm j, top ...j) j {
	return msAlert(name, j{"slo_threshold": arm}, top...)
}

func init() {
	initMetricSloFake()
	initMetricThresholdScenarios()
	initMetricAnomalyScenarios()
	initSloThresholdScenarios()
}

func initMetricThresholdScenarios() {
	addAlertScenario("metric-threshold-minimal-lifecycle", func(h *grHarness) {
		cfg := mtAlert("tfbaseline-mt-minimal", nil)
		s := h.Apply("create with only the required attributes", h.null(), cfg)
		s = msRefreshPlan(h, "refresh", s)
		imported := h.Import("import by id", idOf(s))
		h.Plan("plan the same config on the imported state", imported, cfg)
		s = h.Apply("update: rename", s, mtAlert("tfbaseline-mt-minimal-2", nil))
		h.Destroy("destroy", s)
	})

	addAlertScenario("metric-threshold-full-lifecycle", func(h *grHarness) {
		cfg := mtAlert("tfbaseline-mt-full", mtFullArm, mtFullTop())
		s := h.Apply("create with every optional attribute", h.null(), cfg)
		s = msRefreshPlan(h, "refresh", s)
		imported := h.Import("import by id", idOf(s))
		h.Plan("plan the same config on the imported state", imported, cfg)
		s = h.Apply("update: change every attribute", s, mtAlert("tfbaseline-mt-full", j{
			"metric_filter":           j{"promql": msPromqlNode},
			"custom_evaluation_delay": float64(2000),
			"missing_values":          j{"replace_with_zero": true},
			"no_data_policy":          j{"state": "ALERTING", "auto_retire_seconds": float64(1200)},
			"undetected_values_management": j{
				"trigger_undetected_values": false,
				"auto_retire_timeframe":     "1_HOUR",
			},
			"rules": l{
				mtRule("MORE_THAN_OR_EQUALS", 7, 20, "30_MINUTES", j{"priority": "P4"}),
			},
		}, j{"description": "changed", "priority": "P1", "enabled": true, "group_by": l{"node"}}))
		s = msRefreshPlan(h, "refresh after the update", s)
		h.Destroy("destroy", s)
	})

	addAlertScenario("metric-threshold-update-each-field", func(h *grHarness) {
		name := "tfbaseline-mt-fields"
		s := h.Apply("create", h.null(), mtAlert(name, nil))
		s = h.Apply("update: promql", s, mtAlert(name, j{"metric_filter": j{"promql": `sum(rate(tfbaseline_errors_total[1m]))`}}))
		s = h.Apply("update: threshold", s, mtAlert(name, j{"metric_filter": j{"promql": `sum(rate(tfbaseline_errors_total[1m]))`},
			"rules": l{mtRule("MORE_THAN", 99.5, 50, "10_MINUTES", nil)}}))
		s = h.Apply("update: for_over_pct", s, mtAlert(name, j{"metric_filter": j{"promql": `sum(rate(tfbaseline_errors_total[1m]))`},
			"rules": l{mtRule("MORE_THAN", 99.5, 80, "10_MINUTES", nil)}}))
		s = h.Apply("update: of_the_last", s, mtAlert(name, j{"metric_filter": j{"promql": `sum(rate(tfbaseline_errors_total[1m]))`},
			"rules": l{mtRule("MORE_THAN", 99.5, 80, "1_HOUR", nil)}}))
		s = h.Apply("update: condition_type", s, mtAlert(name, j{"metric_filter": j{"promql": `sum(rate(tfbaseline_errors_total[1m]))`},
			"rules": l{mtRule("LESS_THAN", 99.5, 80, "1_HOUR", nil)}}))
		s = h.Apply("update: override.priority", s, mtAlert(name, j{"metric_filter": j{"promql": `sum(rate(tfbaseline_errors_total[1m]))`},
			"rules": l{mtRule("LESS_THAN", 99.5, 80, "1_HOUR", j{"priority": "P2"})}}))
		s = h.Apply("update: missing_values to the other arm", s, mtAlert(name, j{"metric_filter": j{"promql": `sum(rate(tfbaseline_errors_total[1m]))`},
			"missing_values": j{"min_non_null_values_pct": float64(30)},
			"rules":          l{mtRule("LESS_THAN", 99.5, 80, "1_HOUR", j{"priority": "P2"})}}))
		s = h.Apply("update: custom_evaluation_delay", s, mtAlert(name, j{"metric_filter": j{"promql": `sum(rate(tfbaseline_errors_total[1m]))`},
			"custom_evaluation_delay": float64(500),
			"missing_values":          j{"min_non_null_values_pct": float64(30)},
			"rules":                   l{mtRule("LESS_THAN", 99.5, 80, "1_HOUR", j{"priority": "P2"})}}))
		msRefreshPlan(h, "refresh", s)
	})

	addAlertScenario("metric-threshold-optional-removal", func(h *grHarness) {
		name := "tfbaseline-mt-removal"
		full := mtAlert(name, mtFullArm, mtFullTop())
		s := h.Apply("create with every optional attribute", h.null(), full)
		s = h.Apply("update: remove custom_evaluation_delay", s, mtAlert(name, with(mtFullArm, j{"custom_evaluation_delay": nil}), mtFullTop()))
		s = msRefreshPlan(h, "refresh", s)
		s = h.Apply("update: remove no_data_policy", s, mtAlert(name, with(mtFullArm, j{"custom_evaluation_delay": nil, "no_data_policy": nil}), mtFullTop()))
		s = msRefreshPlan(h, "refresh", s)
		s = h.Apply("update: remove undetected_values_management", s, mtAlert(name, with(mtFullArm, j{
			"custom_evaluation_delay": nil, "no_data_policy": nil, "undetected_values_management": nil,
		}), mtFullTop()))
		s = msRefreshPlan(h, "refresh", s)
		s = h.Apply("update: remove group_by", s, mtAlert(name, with(mtFullArm, j{
			"custom_evaluation_delay": nil, "no_data_policy": nil, "undetected_values_management": nil,
		}), j{"description": "metric threshold baseline", "priority": "P3", "enabled": false}))
		s = msRefreshPlan(h, "refresh", s)
		bareArm := with(mtFullArm, j{"custom_evaluation_delay": nil, "no_data_policy": nil, "undetected_values_management": nil})
		s = h.Apply("update: remove priority and enabled", s, mtAlert(name, bareArm, j{"description": "metric threshold baseline"}))
		s = msRefreshPlan(h, "refresh", s)
		// Recorded: a Replace without description keeps the stored one when nothing else changes.
		s = h.Apply("update: remove description", s, mtAlert(name, bareArm))
		msRefreshPlan(h, "refresh", s)
	})

	addAlertScenario("metric-threshold-of-the-last-enum", func(h *grHarness) {
		s := h.null()
		for _, w := range msWindows {
			s = h.Apply("of_the_last "+w, s, mtAlert("tfbaseline-mt-window-enum", j{"rules": l{mtRule("MORE_THAN", 10, 50, w, nil)}}))
		}
		msRefreshPlan(h, "refresh", s)
	})

	addAlertScenario("metric-threshold-of-the-last-duration", func(h *grHarness) {
		// A free duration is sent as the dynamic duration; an enum name is sent as the enum. The
		// API may rewrite a duration that equals an enum value, which would show as drift.
		s := h.null()
		for _, w := range []string{"30s", "1m", "5m", "1h", "90m", "1h20m15s", "15d", "2w", "1y", "500ms", "0"} {
			s = h.Apply("of_the_last "+w, s, mtAlert("tfbaseline-mt-window-free", j{"rules": l{mtRule("MORE_THAN", 10, 50, w, nil)}}))
			s = msRefreshPlan(h, "refresh after "+w, s)
		}
		// The schema accepts the empty string, and the provider sends the 1-minute enum for it.
		s = h.Apply("of_the_last as the empty string", s, mtAlert("tfbaseline-mt-window-free", j{"rules": l{mtRule("MORE_THAN", 10, 50, "", nil)}}))
		s = msRefreshPlan(h, "refresh after the empty string", s)
		s = h.Apply("update: back to an enum value", s, mtAlert("tfbaseline-mt-window-free", j{"rules": l{mtRule("MORE_THAN", 10, 50, "5_MINUTES", nil)}}))
		msRefreshPlan(h, "refresh", s)
	})

	addAlertScenario("metric-threshold-condition-types", func(h *grHarness) {
		s := h.null()
		for _, c := range []string{"MORE_THAN", "MORE_THAN_OR_EQUALS", "LESS_THAN", "LESS_THAN_OR_EQUALS", "EQUALS", "NOT_EQUALS"} {
			s = h.Apply("condition_type "+c, s, mtAlert("tfbaseline-mt-condition", j{"rules": l{mtRule(c, 10, 50, "10_MINUTES", nil)}}))
		}
		msRefreshPlan(h, "refresh", s)
	})

	// A negative for_over_pct or min_non_null_values_pct is not recorded: the API answers with a
	// protobuf parse error that names the byte offset in the request body, which the fake cannot
	// reproduce.
	addAlertScenario("metric-threshold-condition-values", func(h *grHarness) {
		name := "tfbaseline-mt-values"
		rule := func(threshold, forOver float64) j {
			return j{"rules": l{mtRule("MORE_THAN", threshold, forOver, "10_MINUTES", nil)}}
		}
		s := h.Apply("for_over_pct 0 and threshold 0", h.null(), mtAlert(name, rule(0, 0)))
		s = msRefreshPlan(h, "refresh", s)
		s = h.Apply("for_over_pct 100", s, mtAlert(name, rule(0, 100)))
		s = h.Apply("negative threshold", s, mtAlert(name, rule(-3.5, 100)))
		s = h.Apply("fractional threshold", s, mtAlert(name, rule(0.000123, 33)))
		s = h.Apply("large threshold", s, mtAlert(name, rule(1e12, 33)))
		s = msRefreshPlan(h, "refresh", s)
		h.Apply("for_over_pct 101 is refused by the API", s, mtAlert(name, rule(1, 101)))
	})

	addAlertScenario("metric-threshold-missing-values-arms", func(h *grHarness) {
		name := "tfbaseline-mt-missing"
		mv := func(v j) j { return mtAlert(name, j{"missing_values": v}) }
		s := h.Apply("replace_with_zero true", h.null(), mv(j{"replace_with_zero": true}))
		s = msRefreshPlan(h, "refresh", s)
		s = h.Apply("replace_with_zero false", s, mv(j{"replace_with_zero": false}))
		s = msRefreshPlan(h, "refresh", s)
		s = h.Apply("min_non_null_values_pct 0", s, mv(j{"min_non_null_values_pct": float64(0)}))
		s = msRefreshPlan(h, "refresh", s)
		s = h.Apply("min_non_null_values_pct 50", s, mv(j{"min_non_null_values_pct": float64(50)}))
		s = h.Apply("min_non_null_values_pct 100", s, mv(j{"min_non_null_values_pct": float64(100)}))
		s = msRefreshPlan(h, "refresh", s)
		s = h.Apply("switch back to replace_with_zero true", s, mv(j{"replace_with_zero": true}))
		h.Apply("min_non_null_values_pct 101 is refused by the API", s, mv(j{"min_non_null_values_pct": float64(101)}))
	})

	addAlertScenario("metric-threshold-undetected-values", func(h *grHarness) {
		name := "tfbaseline-mt-undetected"
		less := l{mtRule("LESS_THAN", 5, 50, "10_MINUTES", nil)}
		uv := func(v j, rules l) j {
			return mtAlert(name, j{"undetected_values_management": v, "rules": rules})
		}
		s := h.Apply("omitted: the provider plans its defaults", h.null(), mtAlert(name, nil))
		s = msRefreshPlan(h, "refresh", s)
		s = h.Apply("trigger_undetected_values true with a LESS_THAN rule", s, uv(j{"trigger_undetected_values": true, "auto_retire_timeframe": "5_MINUTES"}, less))
		s = msRefreshPlan(h, "refresh", s)
		for _, t := range []string{"NEVER", "5_MINUTES", "10_MINUTES", "1_HOUR", "2_HOURS", "6_HOURS", "12_HOURS", "24_HOURS"} {
			s = h.Apply("auto_retire_timeframe "+t, s, uv(j{"trigger_undetected_values": true, "auto_retire_timeframe": t}, less))
		}
		s = msRefreshPlan(h, "refresh", s)
		s = h.Apply("only trigger_undetected_values: the timeframe takes its default", s, uv(j{"trigger_undetected_values": true}, less))
		s = msRefreshPlan(h, "refresh", s)
		s = h.Apply("only auto_retire_timeframe: the trigger takes its default", s, uv(j{"auto_retire_timeframe": "1_HOUR"}, less))
		s = msRefreshPlan(h, "refresh", s)
		s = h.Apply("empty block", s, uv(j{}, less))
		s = msRefreshPlan(h, "refresh", s)
		s = h.Apply("trigger false with a timeframe", s, uv(j{"trigger_undetected_values": false, "auto_retire_timeframe": "2_HOURS"}, less))
		s = h.Apply("remove the block", s, mtAlert(name, j{"rules": less}))
		s = msRefreshPlan(h, "refresh after removal", s)
		// The attribute says that a trigger needs a LESS_THAN rule. The API decides.
		more := l{mtRule("MORE_THAN", 5, 50, "10_MINUTES", nil)}
		h.Apply("trigger true with only a MORE_THAN rule", s, uv(j{"trigger_undetected_values": true, "auto_retire_timeframe": "5_MINUTES"}, more))
		mixed := l{mtRule("MORE_THAN", 50, 50, "10_MINUTES", nil), mtRule("LESS_THAN", 5, 50, "10_MINUTES", nil)}
		h.Apply("trigger true with a MORE_THAN and a LESS_THAN rule", s, uv(j{"trigger_undetected_values": true}, mixed))
		h.Apply("trigger true with a LESS_THAN_OR_EQUALS rule", s, uv(j{"trigger_undetected_values": true}, l{mtRule("LESS_THAN_OR_EQUALS", 5, 50, "10_MINUTES", nil)}))
	})

	addAlertScenario("metric-threshold-no-data-policy", func(h *grHarness) {
		name := "tfbaseline-mt-nodata"
		nd := func(v j) j { return mtAlert(name, j{"no_data_policy": v}) }
		s := h.Apply("omitted", h.null(), mtAlert(name, nil))
		s = msRefreshPlan(h, "refresh", s)
		for _, st := range []string{"OK", "ALERTING", "KEEP_LAST", "NO_DATA"} {
			s = h.Apply("state "+st+" without auto_retire_seconds", s, nd(j{"state": st}))
			s = msRefreshPlan(h, "refresh", s)
		}
		for _, st := range []string{"OK", "ALERTING", "KEEP_LAST", "NO_DATA"} {
			s = h.Apply("state "+st+" with auto_retire_seconds 600", s, nd(j{"state": st, "auto_retire_seconds": float64(600)}))
		}
		s = msRefreshPlan(h, "refresh", s)
		s = h.Apply("auto_retire_seconds 3600", s, nd(j{"state": "KEEP_LAST", "auto_retire_seconds": float64(3600)}))
		s = h.Apply("remove auto_retire_seconds", s, nd(j{"state": "KEEP_LAST"}))
		s = msRefreshPlan(h, "refresh", s)
		s = h.Apply("only auto_retire_seconds", s, nd(j{"auto_retire_seconds": float64(120)}))
		s = msRefreshPlan(h, "refresh", s)
		s = h.Apply("empty block", s, nd(j{}))
		s = msRefreshPlan(h, "refresh", s)
		s = h.Apply("state UNSPECIFIED (deprecated, accepted with a warning)", s, nd(j{"state": "UNSPECIFIED"}))
		s = msRefreshPlan(h, "refresh", s)
		s = h.Apply("remove the block", s, mtAlert(name, nil))
		s = msRefreshPlan(h, "refresh after removal", s)
		h.Apply("auto_retire_seconds 90 is no multiple of 60", s, nd(j{"state": "KEEP_LAST", "auto_retire_seconds": float64(90)}))
		h.Apply("auto_retire_seconds 0", s, nd(j{"state": "KEEP_LAST", "auto_retire_seconds": float64(0)}))
	})

	addAlertScenario("metric-threshold-no-data-policy-once-set", func(h *grHarness) {
		// The attribute is Optional+Computed with a state-keeping plan modifier: removing the block
		// from the config keeps the value that the state holds.
		name := "tfbaseline-mt-nodata-kept"
		s := h.Apply("create with a policy", h.null(), mtAlert(name, j{"no_data_policy": j{"state": "OK"}}))
		s = h.Apply("remove the policy from the config", s, mtAlert(name, nil))
		s = msRefreshPlan(h, "refresh", s)
		s = h.Apply("change the description to force an update", s, mtAlert(name, nil, j{"description": "forces an update"}))
		msRefreshPlan(h, "refresh", s)
	})

	addAlertScenario("metric-threshold-rules-collection", func(h *grHarness) {
		name := "tfbaseline-mt-rules"
		p := func(pr string) j { return j{"priority": pr} }
		two := l{mtRule("MORE_THAN", 10, 50, "10_MINUTES", p("P3")), mtRule("MORE_THAN", 20, 50, "10_MINUTES", p("P2"))}
		s := h.Apply("one rule", h.null(), mtAlert(name, nil))
		s = h.Apply("update: two rules", s, mtAlert(name, j{"rules": two}))
		s = msRefreshPlan(h, "refresh", s)
		s = h.Apply("update: the same rules in another order", s, mtAlert(name, j{"rules": l{two[1], two[0]}}))
		s = msRefreshPlan(h, "refresh", s)
		five := l{
			mtRule("MORE_THAN", 10, 50, "10_MINUTES", p("P5")),
			mtRule("MORE_THAN", 20, 50, "10_MINUTES", p("P4")),
			mtRule("MORE_THAN", 30, 50, "10_MINUTES", p("P3")),
			mtRule("MORE_THAN", 40, 50, "10_MINUTES", p("P2")),
			mtRule("MORE_THAN", 50, 50, "10_MINUTES", p("P1")),
		}
		s = h.Apply("update: five rules, one per priority", s, mtAlert(name, j{"rules": five}))
		s = msRefreshPlan(h, "refresh", s)
		s = h.Apply("update: back to one rule", s, mtAlert(name, j{"rules": five[:1]}))
		s = msRefreshPlan(h, "refresh", s)
		h.Apply("update: six rules is more than the API accepts", s, mtAlert(name, j{"rules": append(append(l{}, five...), mtRule("MORE_THAN", 60, 50, "10_MINUTES", p("P1")))}))
	})

	addAlertScenario("metric-threshold-rules-mixed-conditions", func(h *grHarness) {
		// The API refuses to mix "more" and "less" conditions in one alert. Every pair of
		// condition types is tried from the same valid state.
		name := "tfbaseline-mt-mixed"
		s := h.Apply("one MORE_THAN rule", h.null(), mtAlert(name, nil))
		types := []string{"MORE_THAN", "MORE_THAN_OR_EQUALS", "LESS_THAN", "LESS_THAN_OR_EQUALS", "EQUALS", "NOT_EQUALS"}
		for i, a := range types {
			for _, b := range types[i+1:] {
				next := h.Apply(a+" and "+b, s, mtAlert(name, j{"rules": l{
					mtRule(a, 1, 50, "10_MINUTES", j{"priority": "P1"}),
					mtRule(b, 2, 50, "10_MINUTES", j{"priority": "P2"}),
				}}))
				if !next.IsNull() {
					s = next
				}
			}
		}
		s = msRefreshPlan(h, "refresh", s)
		s = h.Apply("rules with different windows", s, mtAlert(name, j{"rules": l{
			mtRule("MORE_THAN", 1, 50, "5_MINUTES", j{"priority": "P1"}),
			mtRule("MORE_THAN", 2, 50, "1h", j{"priority": "P2"}),
		}}))
		s = msRefreshPlan(h, "refresh", s)
		h.Apply("two rules with the same priority", s, mtAlert(name, j{"rules": l{
			mtRule("MORE_THAN", 1, 50, "5_MINUTES", j{"priority": "P2"}),
			mtRule("MORE_THAN", 2, 50, "5_MINUTES", j{"priority": "P2"}),
		}}))
		h.Apply("two identical rules", s, mtAlert(name, j{"rules": l{
			mtRule("MORE_THAN", 1, 50, "5_MINUTES", nil),
			mtRule("MORE_THAN", 1, 50, "5_MINUTES", nil),
		}}))
	})
	addAlertScenario("metric-threshold-override-priority", func(h *grHarness) {
		name := "tfbaseline-mt-override"
		rules := func(pr any) j {
			override := j{}
			if pr != nil {
				override["priority"] = pr
			}
			return j{"rules": l{mtRule("MORE_THAN", 10, 50, "10_MINUTES", override)}}
		}
		s := h.Apply("no override priority and no top-level priority: P5", h.null(), mtAlert(name, rules(nil)))
		s = msRefreshPlan(h, "refresh", s)
		s = h.Apply("top-level priority P2: the override follows it", s, mtAlert(name, rules(nil), j{"priority": "P2"}))
		s = msRefreshPlan(h, "refresh", s)
		s = h.Apply("top-level priority P4: the override follows it", s, mtAlert(name, rules(nil), j{"priority": "P4"}))
		s = h.Apply("override P1 beats top-level P4", s, mtAlert(name, rules("P1"), j{"priority": "P4"}))
		s = msRefreshPlan(h, "refresh", s)
		s = h.Apply("remove the top-level priority, the override keeps P1", s, mtAlert(name, rules("P1")))
		s = msRefreshPlan(h, "refresh", s)
		for _, pr := range msPriorities {
			s = h.Apply("override priority "+pr, s, mtAlert(name, rules(pr)))
		}
		s = msRefreshPlan(h, "refresh", s)
		h.Apply("remove the override priority", s, mtAlert(name, rules(nil)))
	})

	addAlertScenario("metric-threshold-group-by", func(h *grHarness) {
		msGroupByScenario(h, func(q string, top ...j) j {
			return mtAlert("tfbaseline-mt-group", j{"metric_filter": j{"promql": q}}, top...)
		})
	})
	addAlertScenario("metric-threshold-promql", func(h *grHarness) {
		name := "tfbaseline-mt-promql"
		pq := func(q string) j { return j{"metric_filter": j{"promql": q}} }
		s := h.Apply("simple selector", h.null(), mtAlert(name, pq(`tfbaseline_up`)))
		s = h.Apply("label matchers with quotes", s, mtAlert(name, pq(`tfbaseline_up{job="api", env=~"prod|stage"}`)))
		s = msRefreshPlan(h, "refresh", s)
		s = h.Apply("multi-line query", s, mtAlert(name, pq("sum by (job) (\n  rate(tfbaseline_requests_total[5m])\n)")))
		s = msRefreshPlan(h, "refresh", s)
		s = h.Apply("trailing space", s, mtAlert(name, pq(`tfbaseline_up `)))
		s = msRefreshPlan(h, "refresh", s)
		h.Apply("query that is not PromQL", s, mtAlert(name, pq(`this is ((not promql`)))
		h.Apply("empty query", s, mtAlert(name, pq(``)))
	})

	addAlertScenario("metric-threshold-custom-evaluation-delay", func(h *grHarness) {
		name := "tfbaseline-mt-delay"
		d := func(v any) j { return j{"custom_evaluation_delay": v} }
		s := h.Apply("omitted", h.null(), mtAlert(name, nil))
		s = msRefreshPlan(h, "refresh", s)
		s = h.Apply("0", s, mtAlert(name, d(float64(0))))
		s = msRefreshPlan(h, "refresh", s)
		s = h.Apply("60000", s, mtAlert(name, d(float64(60000))))
		s = h.Apply("10800000, the largest value", s, mtAlert(name, d(float64(10800000))))
		s = msRefreshPlan(h, "refresh", s)
		s = h.Apply("remove it", s, mtAlert(name, nil))
		s = msRefreshPlan(h, "refresh after removal", s)
		h.Apply("10800001 is refused by the API", s, mtAlert(name, d(float64(10800001))))
	})

	addAlertScenario("metric-threshold-config-validation", func(h *grHarness) {
		name := "tfbaseline-mt-validation"
		bad := func(step string, arm j, top ...j) { h.validateSorted(step, mtAlert(name, arm, top...)) }
		rule := func(c j) j { return j{"rules": l{j{"condition": c, "override": j{}}}} }
		cond := func(extra j) j {
			return with(j{"condition_type": "MORE_THAN", "threshold": float64(1), "for_over_pct": float64(50), "of_the_last": "10_MINUTES"}, extra)
		}
		h.validateSorted("a valid config", mtAlert(name, nil))
		bad("missing_values with both arms", j{"missing_values": j{"replace_with_zero": true, "min_non_null_values_pct": float64(10)}})
		bad("missing_values with neither arm", j{"missing_values": j{}})
		bad("missing_values null", j{"missing_values": nil})
		bad("metric_filter null", j{"metric_filter": nil})
		bad("metric_filter without promql", j{"metric_filter": j{}})
		bad("rules null", j{"rules": nil})
		bad("empty rules", j{"rules": l{}})
		bad("rule without condition", j{"rules": l{j{"override": j{}}}})
		bad("rule without override", j{"rules": l{j{"condition": cond(nil)}}})
		bad("condition_type outside the list", rule(cond(j{"condition_type": "GREATER_THAN"})))
		bad("lowercase condition_type", rule(cond(j{"condition_type": "more_than"})))
		bad("condition_type null", rule(cond(j{"condition_type": nil})))
		bad("of_the_last neither an enum value nor a duration", rule(cond(j{"of_the_last": "5 minutes"})))
		bad("of_the_last with an unknown unit", rule(cond(j{"of_the_last": "5x"})))
		bad("of_the_last with units in the wrong order", rule(cond(j{"of_the_last": "5m1h"})))
		bad("of_the_last lowercase enum value", rule(cond(j{"of_the_last": "5_minutes"})))
		bad("of_the_last an enum value of the logs types", rule(cond(j{"of_the_last": "NOW"})))
		bad("of_the_last empty string", rule(cond(j{"of_the_last": ""})))
		bad("of_the_last null", rule(cond(j{"of_the_last": nil})))
		bad("threshold null", rule(cond(j{"threshold": nil})))
		bad("for_over_pct null", rule(cond(j{"for_over_pct": nil})))
		bad("override priority outside P1..P5", j{"rules": l{mtRule("MORE_THAN", 1, 50, "10_MINUTES", j{"priority": "P6"})}})
		bad("override null", j{"rules": l{j{"condition": cond(nil), "override": nil}}})
		bad("custom_evaluation_delay below 0", j{"custom_evaluation_delay": float64(-1)})
		bad("no_data_policy state outside the list", j{"no_data_policy": j{"state": "BROKEN"}})
		bad("no_data_policy state UNSPECIFIED warns", j{"no_data_policy": j{"state": "UNSPECIFIED"}})
		bad("undetected auto_retire_timeframe outside the list", j{"undetected_values_management": j{"auto_retire_timeframe": "3_HOURS"}})
		bad("group_by is allowed on a metric alert", nil, j{"group_by": l{"pod"}})
		bad("priority warns that it is deprecated", nil, j{"priority": "P2"})
		h.validateSorted("two arms: metric_threshold and metric_anomaly", msAlert(name, j{"metric_threshold": mtArm(nil), "metric_anomaly": maArm(nil)}))
		h.validateSorted("two arms: metric_threshold and slo_threshold", msAlert(name, j{
			"metric_threshold": mtArm(nil), "slo_threshold": sloErrorBudget(sloMissingID, sloRule(1, nil)),
		}))
		h.validateSorted("an empty metric_threshold", msAlert(name, j{"metric_threshold": j{}}))
	})

	addAlertScenario("metric-threshold-api-errors", func(h *grHarness) {
		name := "tfbaseline-mt-errors"
		// A refused create leaves no resource; the next apply creates it.
		h.Apply("create with six rules", h.null(), mtAlert(name, j{"rules": l{
			mtRule("MORE_THAN", 1, 50, "10_MINUTES", nil), mtRule("MORE_THAN", 2, 50, "10_MINUTES", nil),
			mtRule("MORE_THAN", 3, 50, "10_MINUTES", nil), mtRule("MORE_THAN", 4, 50, "10_MINUTES", nil),
			mtRule("MORE_THAN", 5, 50, "10_MINUTES", nil), mtRule("MORE_THAN", 6, 50, "10_MINUTES", nil),
		}}))
		h.Apply("create with an empty promql", h.null(), mtAlert(name, j{"metric_filter": j{"promql": ""}}))
		h.Apply("create with a duration the API does not parse", h.null(), mtAlert(name, j{"rules": l{mtRule("MORE_THAN", 1, 50, "5 minutes", nil)}}))
		s := h.Apply("create a valid alert", h.null(), mtAlert(name, nil))
		s = h.Apply("update: for_over_pct 101", s, mtAlert(name, j{"rules": l{mtRule("MORE_THAN", 1, 101, "10_MINUTES", nil)}}))
		s = msRefreshPlan(h, "refresh after the refused update", s)
		h.Destroy("destroy", s)
	})

	addAlertScenario("metric-threshold-seeded-sparse-import", func(h *grHarness) {
		// An alert made outside Terraform with only the fields the API requires. Terraform imports
		// it, and the provider has to read what the API fills in.
		id := h.alertBackend().seed(map[string]any{
			"name": "tfbaseline-mt-seeded",
			"type": "ALERT_DEF_TYPE_METRIC_THRESHOLD",
			"metricThreshold": map[string]any{
				"metricFilter": map[string]any{"promql": msPromql},
				"rules": []any{map[string]any{
					"condition": map[string]any{
						"threshold":     10,
						"forOverPct":    50,
						"ofTheLast":     map[string]any{"metricTimeWindowSpecificValue": "METRIC_TIME_WINDOW_VALUE_MINUTES_5"},
						"conditionType": "METRIC_THRESHOLD_CONDITION_TYPE_MORE_THAN_OR_UNSPECIFIED",
					},
					"override": map[string]any{"priority": "ALERT_DEF_PRIORITY_P3"},
				}},
				"missingValues": map[string]any{"replaceWithZero": true},
			},
		})
		imported := h.Import("import the alert", id)
		h.Plan("plan a config that matches it", imported, mtAlert("tfbaseline-mt-seeded", j{
			"rules": l{mtRule("MORE_THAN", 10, 50, "5_MINUTES", j{"priority": "P3"})},
		}))
		s := h.Apply("apply the matching config", imported, mtAlert("tfbaseline-mt-seeded", j{
			"rules": l{mtRule("MORE_THAN", 10, 50, "5_MINUTES", j{"priority": "P3"})},
		}))
		h.Destroy("destroy", s)
	})

	addAlertScenario("metric-threshold-seeded-dynamic-duration-import", func(h *grHarness) {
		id := h.alertBackend().seed(map[string]any{
			"name": "tfbaseline-mt-seeded-dyn",
			"type": "ALERT_DEF_TYPE_METRIC_THRESHOLD",
			"metricThreshold": map[string]any{
				"metricFilter": map[string]any{"promql": msPromql},
				"rules": []any{map[string]any{
					"condition": map[string]any{
						"threshold":     10,
						"forOverPct":    50,
						"ofTheLast":     map[string]any{"metricTimeWindowDynamicDuration": "45m"},
						"conditionType": "METRIC_THRESHOLD_CONDITION_TYPE_LESS_THAN",
					},
					"override": map[string]any{"priority": "ALERT_DEF_PRIORITY_P1"},
				}},
				"missingValues": map[string]any{"minNonNullValuesPct": 40},
			},
		})
		imported := h.Import("import the alert", id)
		h.Destroy("destroy", imported)
	})

	addAlertScenario("metric-threshold-deleted-outside-terraform", func(h *grHarness) {
		cfg := mtAlert("tfbaseline-mt-gone", nil)
		s := h.Apply("create", h.null(), cfg)
		h.alertBackend().forget(idOf(s))
		msRefresh(h, "refresh after the alert was deleted outside", s)
		h.Apply("update after the alert was deleted outside", s, mtAlert("tfbaseline-mt-gone", nil, j{"description": "x"}))
	})

	addAlertScenario("metric-type-switch", func(h *grHarness) {
		name := "tfbaseline-metric-switch"
		s := h.Apply("create metric_threshold", h.null(), mtAlert(name, nil))
		s = h.Apply("update: switch to metric_anomaly", s, maAlert(name, nil))
		s = msRefreshPlan(h, "refresh", s)
		s = h.Apply("update: switch back to metric_threshold", s, mtAlert(name, nil))
		s = msRefreshPlan(h, "refresh", s)
		s = h.Apply("update: switch to logs_immediate", s, alertLogsImmediate(name))
		s = msRefreshPlan(h, "refresh", s)
		h.Apply("update: switch to metric_anomaly", s, maAlert(name, nil))
	})
}

func initMetricAnomalyScenarios() {
	addAlertScenario("metric-anomaly-minimal-lifecycle", func(h *grHarness) {
		cfg := maAlert("tfbaseline-ma-minimal", nil)
		s := h.Apply("create with only the required attributes", h.null(), cfg)
		s = msRefreshPlan(h, "refresh", s)
		imported := h.Import("import by id", idOf(s))
		h.Plan("plan the same config on the imported state", imported, cfg)
		s = h.Apply("update: rename", s, maAlert("tfbaseline-ma-minimal-2", nil))
		h.Destroy("destroy", s)
	})

	maFull := j{
		"metric_filter":           j{"promql": msPromqlPod},
		"percentage_of_deviation": float64(25),
		"custom_evaluation_delay": float64(1000),
		"rules":                   l{maRule("LESS_THAN", 20, 80, 40, "1_HOUR")},
	}
	maFullTop := j{"description": "metric anomaly baseline", "priority": "P2", "enabled": false, "group_by": l{"pod"}}

	addAlertScenario("metric-anomaly-full-lifecycle", func(h *grHarness) {
		cfg := maAlert("tfbaseline-ma-full", maFull, maFullTop)
		s := h.Apply("create with every optional attribute", h.null(), cfg)
		s = msRefreshPlan(h, "refresh", s)
		imported := h.Import("import by id", idOf(s))
		h.Plan("plan the same config on the imported state", imported, cfg)
		s = h.Apply("update: change every attribute", s, maAlert("tfbaseline-ma-full", j{
			"metric_filter":           j{"promql": msPromqlNode},
			"percentage_of_deviation": float64(40),
			"custom_evaluation_delay": float64(3000),
			"rules":                   l{maRule("MORE_THAN", 3, 20, 10, "6_HOURS")},
		}, j{"description": "changed", "priority": "P4", "enabled": true, "group_by": l{"node"}}))
		s = msRefreshPlan(h, "refresh after the update", s)
		h.Destroy("destroy", s)
	})

	addAlertScenario("metric-anomaly-update-each-field", func(h *grHarness) {
		name := "tfbaseline-ma-fields"
		q := j{"metric_filter": j{"promql": `sum(rate(tfbaseline_errors_total[1m]))`}}
		arm := func(rule j, extra ...j) j {
			out := with(q, j{"rules": l{rule}})
			for _, e := range extra {
				out = with(out, e)
			}
			return out
		}
		s := h.Apply("create", h.null(), maAlert(name, nil))
		s = h.Apply("update: promql", s, maAlert(name, q))
		s = h.Apply("update: threshold", s, maAlert(name, arm(maRule("MORE_THAN", 99.5, 50, 50, "10_MINUTES"))))
		s = h.Apply("update: for_over_pct", s, maAlert(name, arm(maRule("MORE_THAN", 99.5, 80, 50, "10_MINUTES"))))
		s = h.Apply("update: min_non_null_values_pct", s, maAlert(name, arm(maRule("MORE_THAN", 99.5, 80, 30, "10_MINUTES"))))
		s = h.Apply("update: of_the_last", s, maAlert(name, arm(maRule("MORE_THAN", 99.5, 80, 30, "1_HOUR"))))
		s = h.Apply("update: condition_type", s, maAlert(name, arm(maRule("LESS_THAN", 99.5, 80, 30, "1_HOUR"))))
		s = h.Apply("update: percentage_of_deviation", s, maAlert(name, arm(maRule("LESS_THAN", 99.5, 80, 30, "1_HOUR"), j{"percentage_of_deviation": float64(15)})))
		s = h.Apply("update: custom_evaluation_delay", s, maAlert(name, arm(maRule("LESS_THAN", 99.5, 80, 30, "1_HOUR"), j{"percentage_of_deviation": float64(15), "custom_evaluation_delay": float64(500)})))
		msRefreshPlan(h, "refresh", s)
	})

	addAlertScenario("metric-anomaly-optional-removal", func(h *grHarness) {
		name := "tfbaseline-ma-removal"
		s := h.Apply("create with every optional attribute", h.null(), maAlert(name, maFull, maFullTop))
		s = h.Apply("update: remove percentage_of_deviation", s, maAlert(name, with(maFull, j{"percentage_of_deviation": nil}), maFullTop))
		s = msRefreshPlan(h, "refresh", s)
		s = h.Apply("update: remove custom_evaluation_delay", s, maAlert(name, with(maFull, j{"percentage_of_deviation": nil, "custom_evaluation_delay": nil}), maFullTop))
		s = msRefreshPlan(h, "refresh", s)
		bare := with(maFull, j{"percentage_of_deviation": nil, "custom_evaluation_delay": nil})
		s = h.Apply("update: remove group_by", s, maAlert(name, bare, j{"description": "metric anomaly baseline", "priority": "P2", "enabled": false}))
		s = msRefreshPlan(h, "refresh", s)
		s = h.Apply("update: remove priority and enabled", s, maAlert(name, bare, j{"description": "metric anomaly baseline"}))
		s = msRefreshPlan(h, "refresh", s)
		// Recorded: a Replace without description keeps the stored one when nothing else changes.
		s = h.Apply("update: remove description", s, maAlert(name, bare))
		msRefreshPlan(h, "refresh", s)
	})

	addAlertScenario("metric-anomaly-of-the-last-enum", func(h *grHarness) {
		s := h.null()
		for _, w := range msWindows {
			s = h.Apply("of_the_last "+w, s, maAlert("tfbaseline-ma-window", j{"rules": l{maRule("MORE_THAN", 10, 50, 50, w)}}))
		}
		msRefreshPlan(h, "refresh", s)
	})

	addAlertScenario("metric-anomaly-condition-types", func(h *grHarness) {
		s := h.null()
		for _, c := range []string{"MORE_THAN", "LESS_THAN"} {
			s = h.Apply("condition_type "+c, s, maAlert("tfbaseline-ma-condition", j{"rules": l{maRule(c, 10, 50, 50, "10_MINUTES")}}))
			s = msRefreshPlan(h, "refresh", s)
		}
		h.Apply("MORE_THAN again", s, maAlert("tfbaseline-ma-condition", nil))
	})

	addAlertScenario("metric-anomaly-condition-values", func(h *grHarness) {
		name := "tfbaseline-ma-values"
		rule := func(threshold, forOver, minNonNull float64) j {
			return j{"rules": l{maRule("MORE_THAN", threshold, forOver, minNonNull, "10_MINUTES")}}
		}
		s := h.Apply("every percentage 0 and threshold 0", h.null(), maAlert(name, rule(0, 0, 0)))
		s = msRefreshPlan(h, "refresh", s)
		s = h.Apply("every percentage 100", s, maAlert(name, rule(-2.5, 100, 100)))
		s = msRefreshPlan(h, "refresh", s)
		h.Apply("for_over_pct 101 is refused by the API", s, maAlert(name, rule(1, 101, 50)))
		h.Apply("min_non_null_values_pct 101 is refused by the API", s, maAlert(name, rule(1, 50, 101)))
	})

	addAlertScenario("metric-anomaly-percentage-of-deviation", func(h *grHarness) {
		// The API stores a float32. A value that float32 cannot hold exactly reads back rounded,
		// which shows as inconsistent result or drift.
		name := "tfbaseline-ma-deviation"
		d := func(v float64) j { return j{"percentage_of_deviation": v} }
		s := h.Apply("omitted", h.null(), maAlert(name, nil))
		s = msRefreshPlan(h, "refresh", s)
		s = h.Apply("0", s, maAlert(name, d(0)))
		s = msRefreshPlan(h, "refresh", s)
		s = h.Apply("20", s, maAlert(name, d(20)))
		s = h.Apply("20.5", s, maAlert(name, d(20.5)))
		s = msRefreshPlan(h, "refresh", s)
		s = h.Apply("33.3, which float32 rounds", s, maAlert(name, d(33.3)))
		s = msRefreshPlan(h, "refresh", s)
		s = h.Apply("100", s, maAlert(name, d(100)))
		s = h.Apply("250, above 100", s, maAlert(name, d(250)))
		s = msRefreshPlan(h, "refresh", s)
		s = h.Apply("remove it", s, maAlert(name, nil))
		s = msRefreshPlan(h, "refresh after removal", s)
		h.Apply("-1 is refused by the API", s, maAlert(name, d(-1)))
	})

	addAlertScenario("metric-anomaly-rules-collection", func(h *grHarness) {
		name := "tfbaseline-ma-rules"
		s := h.Apply("one rule", h.null(), maAlert(name, nil))
		// The API accepts exactly one rule.
		h.Apply("two rules", s, maAlert(name, j{"rules": l{
			maRule("MORE_THAN", 10, 50, 50, "10_MINUTES"), maRule("LESS_THAN", 5, 50, 50, "10_MINUTES"),
		}}))
		s = msRefreshPlan(h, "refresh after the refused update", s)
		h.Destroy("destroy", s)
	})

	addAlertScenario("metric-anomaly-group-by", func(h *grHarness) {
		msGroupByScenario(h, func(q string, top ...j) j {
			return maAlert("tfbaseline-ma-group", j{"metric_filter": j{"promql": q}}, top...)
		})
	})
	addAlertScenario("metric-anomaly-override-and-priority", func(h *grHarness) {
		// The anomaly rules have no override: only the top-level priority exists.
		name := "tfbaseline-ma-priority"
		s := h.Apply("no priority", h.null(), maAlert(name, nil))
		s = msRefreshPlan(h, "refresh", s)
		for _, pr := range msPriorities {
			s = h.Apply("priority "+pr, s, maAlert(name, nil, j{"priority": pr}))
		}
		s = msRefreshPlan(h, "refresh", s)
		s = h.Apply("remove the priority", s, maAlert(name, nil))
		msRefreshPlan(h, "refresh after removal", s)
	})

	addAlertScenario("metric-anomaly-custom-evaluation-delay", func(h *grHarness) {
		name := "tfbaseline-ma-delay"
		d := func(v any) j { return j{"custom_evaluation_delay": v} }
		s := h.Apply("omitted", h.null(), maAlert(name, nil))
		s = h.Apply("0", s, maAlert(name, d(float64(0))))
		s = msRefreshPlan(h, "refresh", s)
		s = h.Apply("60000", s, maAlert(name, d(float64(60000))))
		s = h.Apply("remove it", s, maAlert(name, nil))
		s = msRefreshPlan(h, "refresh after removal", s)
		h.Apply("10800001 is refused by the API", s, maAlert(name, d(float64(10800001))))
	})

	addAlertScenario("metric-anomaly-config-validation", func(h *grHarness) {
		name := "tfbaseline-ma-validation"
		bad := func(step string, arm j, top ...j) { h.validateSorted(step, maAlert(name, arm, top...)) }
		rule := func(c j) j { return j{"rules": l{j{"condition": c}}} }
		cond := func(extra j) j {
			return with(j{"condition_type": "MORE_THAN", "threshold": float64(1), "for_over_pct": float64(50), "min_non_null_values_pct": float64(50), "of_the_last": "10_MINUTES"}, extra)
		}
		h.validateSorted("a valid config", maAlert(name, nil))
		bad("metric_filter null", j{"metric_filter": nil})
		bad("metric_filter without promql", j{"metric_filter": j{}})
		bad("rules null", j{"rules": nil})
		bad("empty rules", j{"rules": l{}})
		bad("rule without condition", j{"rules": l{j{}}})
		bad("condition_type MORE_THAN_USUAL is not a schema value", rule(cond(j{"condition_type": "MORE_THAN_USUAL"})))
		bad("condition_type EQUALS is not valid for anomaly alerts", rule(cond(j{"condition_type": "EQUALS"})))
		bad("condition_type null", rule(cond(j{"condition_type": nil})))
		bad("of_the_last as a free duration is not accepted", rule(cond(j{"of_the_last": "30m"})))
		bad("of_the_last empty string", rule(cond(j{"of_the_last": ""})))
		bad("of_the_last null", rule(cond(j{"of_the_last": nil})))
		bad("threshold null", rule(cond(j{"threshold": nil})))
		bad("for_over_pct null", rule(cond(j{"for_over_pct": nil})))
		bad("min_non_null_values_pct null", rule(cond(j{"min_non_null_values_pct": nil})))
		bad("custom_evaluation_delay below 0", j{"custom_evaluation_delay": float64(-1)})
		bad("group_by is allowed on an anomaly alert", nil, j{"group_by": l{"pod"}})
		bad("priority outside P1..P5", nil, j{"priority": "P0"})
		bad("priority P2 on an anomaly alert gives no deprecation warning", nil, j{"priority": "P2"})
		h.validateSorted("an empty metric_anomaly", msAlert(name, j{"metric_anomaly": j{}}))
	})

	addAlertScenario("metric-anomaly-promql", func(h *grHarness) {
		name := "tfbaseline-ma-promql"
		pq := func(q string) j { return j{"metric_filter": j{"promql": q}} }
		s := h.Apply("simple selector", h.null(), maAlert(name, pq(`tfbaseline_up`)))
		s = h.Apply("label matchers with quotes", s, maAlert(name, pq(`tfbaseline_up{job="api", env=~"prod|stage"}`)))
		s = msRefreshPlan(h, "refresh", s)
		s = h.Apply("multi-line query", s, maAlert(name, pq("sum by (job) (\n  rate(tfbaseline_requests_total[5m])\n)")))
		s = msRefreshPlan(h, "refresh", s)
		h.Apply("query that is not PromQL", s, maAlert(name, pq(`this is ((not promql`)))
		h.Apply("empty query", s, maAlert(name, pq(``)))
	})

	addAlertScenario("metric-anomaly-seeded-import", func(h *grHarness) {
		id := h.alertBackend().seed(map[string]any{
			"name": "tfbaseline-ma-seeded",
			"type": "ALERT_DEF_TYPE_METRIC_ANOMALY",
			"metricAnomaly": map[string]any{
				"metricFilter": map[string]any{"promql": msPromql},
				"rules": []any{map[string]any{
					"condition": map[string]any{
						"threshold":           10,
						"forOverPct":          50,
						"minNonNullValuesPct": 50,
						"ofTheLast":           map[string]any{"metricTimeWindowSpecificValue": "METRIC_TIME_WINDOW_VALUE_MINUTES_5"},
						"conditionType":       "METRIC_ANOMALY_CONDITION_TYPE_LESS_THAN_USUAL",
					},
				}},
			},
		})
		imported := h.Import("import the alert", id)
		cfg := maAlert("tfbaseline-ma-seeded", j{"rules": l{maRule("LESS_THAN", 10, 50, 50, "5_MINUTES")}})
		h.Plan("plan a config that matches it", imported, cfg)
		s := h.Apply("apply the matching config", imported, cfg)
		h.Destroy("destroy", s)
	})
}

func initSloThresholdScenarios() {
	addAlertScenario("slo-threshold-config-validation", func(h *grHarness) {
		name := "tfbaseline-slo-validation"
		bad := func(step string, arm j, top ...j) { h.validateSorted(step, sloAlert(name, arm, top...)) }
		r := sloRule(1, nil)
		h.validateSorted("a valid error_budget config", sloAlert(name, sloErrorBudget(sloMissingID, r)))
		h.validateSorted("a valid single burn_rate config", sloAlert(name, sloBurnRate(sloMissingID, "single", 1, r)))
		h.validateSorted("a valid dual burn_rate config", sloAlert(name, sloBurnRate(sloMissingID, "dual", 1, r)))
		bad("neither error_budget nor burn_rate", j{"slo_definition": j{"slo_id": sloMissingID}})
		bad("both error_budget and burn_rate", with(sloBurnRate(sloMissingID, "single", 1, r), j{"error_budget": j{"rules": l{r}}}))
		bad("burn_rate with neither single nor dual", j{"slo_definition": j{"slo_id": sloMissingID}, "burn_rate": j{"rules": l{r}}})
		bad("burn_rate with both single and dual", j{"slo_definition": j{"slo_id": sloMissingID}, "burn_rate": j{
			"rules":  l{r},
			"single": j{"time_duration": j{"duration": float64(1), "unit": "HOURS"}},
			"dual":   j{"time_duration": j{"duration": float64(1), "unit": "HOURS"}},
		}})
		bad("burn_rate with null rules", j{"slo_definition": j{"slo_id": sloMissingID}, "burn_rate": j{
			"single": j{"time_duration": j{"duration": float64(1), "unit": "HOURS"}},
		}})
		bad("error_budget with null rules", j{"slo_definition": j{"slo_id": sloMissingID}, "error_budget": j{}})
		bad("error_budget with an empty rules list", sloErrorBudget(sloMissingID))
		bad("burn_rate with an empty rules list", sloBurnRate(sloMissingID, "single", 1))
		bad("slo_definition null", j{"error_budget": j{"rules": l{r}}})
		bad("slo_definition without slo_id", j{"slo_definition": j{}, "error_budget": j{"rules": l{r}}})
		bad("slo_id null", j{"slo_definition": j{"slo_id": nil}, "error_budget": j{"rules": l{r}}})
		bad("time_duration unit outside the list", j{"slo_definition": j{"slo_id": sloMissingID}, "burn_rate": j{
			"rules": l{r}, "single": j{"time_duration": j{"duration": float64(1), "unit": "DAYS"}},
		}})
		bad("time_duration unit UNSPECIFIED", j{"slo_definition": j{"slo_id": sloMissingID}, "burn_rate": j{
			"rules": l{r}, "single": j{"time_duration": j{"duration": float64(1), "unit": "UNSPECIFIED"}},
		}})
		bad("time_duration without duration", j{"slo_definition": j{"slo_id": sloMissingID}, "burn_rate": j{
			"rules": l{r}, "single": j{"time_duration": j{"unit": "HOURS"}},
		}})
		bad("time_duration without unit", j{"slo_definition": j{"slo_id": sloMissingID}, "burn_rate": j{
			"rules": l{r}, "single": j{"time_duration": j{"duration": float64(1)}},
		}})
		bad("single without time_duration", j{"slo_definition": j{"slo_id": sloMissingID}, "burn_rate": j{
			"rules": l{r}, "single": j{},
		}})
		bad("rule without condition", j{"slo_definition": j{"slo_id": sloMissingID}, "error_budget": j{"rules": l{j{"override": j{}}}}})
		bad("rule without override", j{"slo_definition": j{"slo_id": sloMissingID}, "error_budget": j{"rules": l{j{"condition": j{"threshold": float64(1)}}}}})
		bad("rule without threshold", j{"slo_definition": j{"slo_id": sloMissingID}, "error_budget": j{"rules": l{j{"condition": j{}, "override": j{}}}}})
		bad("override priority outside P1..P5", sloErrorBudget(sloMissingID, sloRule(1, j{"priority": "P6"})))
		bad("group_by is allowed on an slo alert", sloErrorBudget(sloMissingID, r), j{"group_by": l{"service"}})
		bad("priority warns that it is deprecated", sloErrorBudget(sloMissingID, r), j{"priority": "P2"})
		h.validateSorted("two arms: slo_threshold and logs_immediate", msAlert(name, j{
			"slo_threshold": sloErrorBudget(sloMissingID, r), "logs_immediate": j{},
		}))
		h.validateSorted("an empty slo_threshold", msAlert(name, j{"slo_threshold": j{}}))
	})

	addAlertScenario("slo-threshold-plan-offline", func(h *grHarness) {
		// What the provider plans, with no backend: the fallback of the override priority, the
		// unknown group_by, and the computed attributes of the top level.
		name := "tfbaseline-slo-plan"
		r := sloRule(1, nil)
		h.Plan("error_budget", h.null(), sloAlert(name, sloErrorBudget(sloMissingID, r)))
		h.Plan("error_budget with a top-level priority", h.null(), sloAlert(name, sloErrorBudget(sloMissingID, r), j{"priority": "P2"}))
		h.Plan("error_budget with several rules and overrides", h.null(), sloAlert(name, sloErrorBudget(sloMissingID,
			sloRule(1, j{"priority": "P1"}), sloRule(2, j{"priority": "P3"}), sloRule(3, nil))))
		h.Plan("single burn_rate", h.null(), sloAlert(name, sloBurnRate(sloMissingID, "single", 1, r)))
		h.Plan("dual burn_rate", h.null(), sloAlert(name, sloBurnRate(sloMissingID, "dual", 2, r)))
		h.Plan("with group_by", h.null(), sloAlert(name, sloErrorBudget(sloMissingID, r), j{"group_by": l{"service"}}))
		h.Plan("with an empty group_by", h.null(), sloAlert(name, sloErrorBudget(sloMissingID, r), j{"group_by": l{}}))
		h.Plan("with a slo_id from another resource (unknown at plan time)", h.null(), sloAlert(name, j{
			"slo_definition": j{"slo_id": unknownJSON{}},
			"error_budget":   j{"rules": l{r}},
		}))
	})

	// An SLO that does not exist: what the API answers is the fact. If the API accepts the
	// alert anyway, the scenario goes on with refresh, update, import, and destroy.
	addAlertScenario("slo-threshold-create-missing-slo", func(h *grHarness) {
		r := sloRule(1, nil)
		for _, step := range []struct {
			name string
			cfg  j
		}{
			{"error_budget with an id that no SLO has", sloAlert("tfbaseline-slo-missing", sloErrorBudget(sloMissingID, r))},
			{"error_budget with an id that is no id", sloAlert("tfbaseline-slo-missing", sloErrorBudget(sloMissingText, r))},
			{"single burn_rate with an id that no SLO has", sloAlert("tfbaseline-slo-missing", sloBurnRate(sloMissingID, "single", 1, r))},
			{"dual burn_rate with an id that no SLO has", sloAlert("tfbaseline-slo-missing", sloBurnRate(sloMissingID, "dual", 1, r))},
		} {
			s := h.Apply(step.name, h.null(), step.cfg)
			if !s.IsNull() {
				s = msRefreshPlan(h, "refresh", s)
				h.Destroy("destroy", s)
			}
		}
	})

	addAlertScenario("slo-threshold-create-invalid-requests", func(h *grHarness) {
		// Requests the API may judge before it looks up the SLO.
		r := sloRule(1, nil)
		h.Apply("burn_rate duration 0", h.null(), sloAlert("tfbaseline-slo-invalid", sloBurnRate(sloMissingID, "single", 0, r)))
		h.Apply("burn_rate duration 100000", h.null(), sloAlert("tfbaseline-slo-invalid", sloBurnRate(sloMissingID, "single", 100000, r)))
		h.Apply("empty slo_id", h.null(), sloAlert("tfbaseline-slo-invalid", sloErrorBudget("", r)))
		h.Apply("six rules", h.null(), sloAlert("tfbaseline-slo-invalid", sloErrorBudget(sloMissingID,
			r, sloRule(2, nil), sloRule(3, nil), sloRule(4, nil), sloRule(5, nil), sloRule(6, nil))))
		h.Apply("error_budget threshold above 100", h.null(), sloAlert("tfbaseline-slo-invalid", sloErrorBudget(sloMissingID, sloRule(1000, nil))))
		h.Apply("negative error_budget threshold", h.null(), sloAlert("tfbaseline-slo-invalid", sloErrorBudget(sloMissingID, sloRule(-1, nil))))
	})
}

// msGroupByScenario drives group_by of a metric alert. The API derives the group-by keys from the
// by-clause of the query: it accepts an empty list (and infers the keys) or the same keys, and it
// refuses anything else.
func msGroupByScenario(h *grHarness, build func(promql string, top ...j) j) {
	by := func(keys ...any) j { return j{"group_by": l(keys)} }
	s := h.Apply("ungrouped query, group_by omitted", h.null(), build(msPromql))
	s = msRefreshPlan(h, "refresh", s)
	s = h.Apply("grouped query, group_by omitted: the plan makes it unknown and the API infers it", s, build(msPromqlPod))
	s = msRefreshPlan(h, "refresh", s)
	s = h.Apply("grouped query, group_by with the inferred key", s, build(msPromqlPod, by("pod")))
	s = msRefreshPlan(h, "refresh", s)
	s = h.Apply("remove group_by: the state keeps the keys", s, build(msPromqlPod))
	s = msRefreshPlan(h, "refresh after removal", s)
	s = h.Apply("two keys, in the order of the query", s, build(msPromqlPodNs, by("pod", "namespace")))
	s = msRefreshPlan(h, "refresh", s)
	s = h.Apply("two keys, in another order", s, build(msPromqlPodNs, by("namespace", "pod")))
	s = msRefreshPlan(h, "refresh", s)
	s = h.Apply("another query with group_by omitted: unknown, inferred again", s, build(msPromqlNode))
	s = msRefreshPlan(h, "refresh", s)
	s = h.Apply("explicit empty group_by on a grouped query", s, build(msPromqlNode, by()))
	s = msRefreshPlan(h, "refresh", s)
	s = h.Apply("ungrouped query with an explicit empty group_by", s, build(msPromql, by()))
	s = msRefreshPlan(h, "refresh", s)
	h.Apply("group_by with a key that the query does not group by", s, build(msPromqlPod, by("namespace")))
	h.Apply("group_by with a key on an ungrouped query", s, build(msPromql, by("pod")))
	h.Apply("group_by with only some of the keys", s, build(msPromqlPodNs, by("pod")))
	h.Apply("group_by with an extra key", s, build(msPromqlPod, by("pod", "namespace")))
	h.Apply("group_by with a repeated key", s, build(msPromqlPod, by("pod", "pod")))
	h.Destroy("destroy", s)
}

// ---- the validation of the API, as recorded ----

// msZodKV is one key of a validation error in the order that the API writes it.
type msZodKV struct {
	key   string
	value any
}

// msZod is the text of one validation error of the API: a JSON array with one object, indented by
// two spaces. The value of "path" is a []any of names and indexes.
func msZod(kvs ...msZodKV) string {
	var lines []string
	for _, kv := range kvs {
		var text string
		switch v := kv.value.(type) {
		case string:
			text = strconv.Quote(v)
		case bool:
			text = strconv.FormatBool(v)
		case float64:
			text = strconv.FormatFloat(v, 'f', -1, 64)
		case []any:
			var items []string
			for _, item := range v {
				if name, ok := item.(string); ok {
					items = append(items, "      "+strconv.Quote(name))
				} else {
					items = append(items, fmt.Sprintf("      %v", item))
				}
			}
			text = "[\n" + strings.Join(items, ",\n") + "\n    ]"
		}
		lines = append(lines, "    "+strconv.Quote(kv.key)+": "+text)
	}
	return "[\n  {\n" + strings.Join(lines, ",\n") + "\n  }\n]"
}

func msPath(arm string, rest ...any) []any {
	return append([]any{"alertDefProperties", "typeDefinition", arm}, rest...)
}

func msTooBig(limit float64, path []any) string {
	return msZod(msZodKV{"code", "too_big"}, msZodKV{"maximum", limit}, msZodKV{"type", "number"}, msZodKV{"inclusive", true},
		msZodKV{"exact", false}, msZodKV{"message", "Number must be less than or equal to " + strconv.FormatFloat(limit, 'f', -1, 64)}, msZodKV{"path", path})
}

func msTooSmall(limit float64, path []any) string {
	return msZod(msZodKV{"code", "too_small"}, msZodKV{"minimum", limit}, msZodKV{"type", "number"}, msZodKV{"inclusive", true},
		msZodKV{"exact", false}, msZodKV{"message", "Number must be greater than or equal to " + strconv.FormatFloat(limit, 'f', -1, 64)}, msZodKV{"path", path})
}

func msCustom(message string, fatal bool, path []any) string {
	if fatal {
		return msZod(msZodKV{"code", "custom"}, msZodKV{"message", message}, msZodKV{"fatal", true}, msZodKV{"path", path})
	}
	return msZod(msZodKV{"code", "custom"}, msZodKV{"message", message}, msZodKV{"path", path})
}

func msMap(m map[string]any, key string) map[string]any {
	out, _ := m[key].(map[string]any)
	return out
}

func msList(m map[string]any, key string) []any {
	out, _ := m[key].([]any)
	return out
}

func msNum(m map[string]any, key string) (float64, bool) {
	v, ok := m[key].(float64)
	return v, ok
}

// msMinutes is the length of an enum value or a free duration in minutes, or -1 when it has none.
var (
	msEnumMinutes = map[string]float64{
		"METRIC_TIME_WINDOW_VALUE_MINUTES_1_OR_UNSPECIFIED": 1, "METRIC_TIME_WINDOW_VALUE_MINUTES_5": 5,
		"METRIC_TIME_WINDOW_VALUE_MINUTES_10": 10, "METRIC_TIME_WINDOW_VALUE_MINUTES_15": 15,
		"METRIC_TIME_WINDOW_VALUE_MINUTES_20": 20, "METRIC_TIME_WINDOW_VALUE_MINUTES_30": 30,
		"METRIC_TIME_WINDOW_VALUE_HOUR_1": 60, "METRIC_TIME_WINDOW_VALUE_HOURS_2": 120,
		"METRIC_TIME_WINDOW_VALUE_HOURS_4": 240, "METRIC_TIME_WINDOW_VALUE_HOURS_6": 360,
		"METRIC_TIME_WINDOW_VALUE_HOURS_12": 720, "METRIC_TIME_WINDOW_VALUE_HOURS_24": 1440,
		"METRIC_TIME_WINDOW_VALUE_HOURS_36": 2160,
	}
	msDurationPattern = regexp.MustCompile(`^(?:(\d+)y)?(?:(\d+)w)?(?:(\d+)d)?(?:(\d+)h)?(?:(\d+)m)?(?:(\d+)s)?(?:(\d+)ms)?$`)
	msSelector        = regexp.MustCompile(`^\s*[A-Za-z_:][A-Za-z0-9_:]*\{([^}]*)\}\s*$`)
	msByClause        = regexp.MustCompile(`\bby\s*\(([^)]*)\)`)
)

// msDurationMinutes parses a free duration. Recorded: "500ms" is accepted although it is not a
// whole minute, so a "ms" count is read as minutes here; the real parser is not known.
func msDurationMinutes(text string) float64 {
	m := msDurationPattern.FindStringSubmatch(text)
	if m == nil {
		return -1
	}
	n := func(i int) float64 { v, _ := strconv.ParseFloat(m[i], 64); return v }
	return n(1)*525600 + n(2)*10080 + n(3)*1440 + n(4)*60 + n(5) + n(6)/60 + n(7)
}

func msWindowMinutes(window map[string]any) float64 {
	if v, ok := window["metricTimeWindowDynamicDuration"].(string); ok {
		return msDurationMinutes(v)
	}
	if v, ok := window["metricTimeWindowSpecificValue"].(string); ok {
		return msEnumMinutes[v]
	}
	return 1
}

// msWindowGroup is the time group of a window: [1, 30], (30, 720), [720, 2160] minutes.
func msWindowGroup(minutes float64) int {
	switch {
	case minutes <= 30:
		return 0
	case minutes < 720:
		return 1
	}
	return 2
}

// msPromqlKeys are the group-by keys that the API infers from the by-clause of a query, sorted.
func msPromqlKeys(promql string) []string {
	keys := []string{}
	if m := msByClause.FindStringSubmatch(promql); m != nil {
		for _, key := range strings.Split(m[1], ",") {
			if key = strings.TrimSpace(key); key != "" {
				keys = append(keys, key)
			}
		}
	}
	if len(keys) == 0 {
		// Recorded: a bare selector with matchers groups by the labels of its matchers. This is the
		// only other shape that the recordings show.
		if m := msSelector.FindStringSubmatch(promql); m != nil {
			for _, matcher := range strings.Split(m[1], ",") {
				if name, _, found := strings.Cut(matcher, "="); found {
					keys = append(keys, strings.TrimSpace(name))
				}
			}
		}
	}
	slices.Sort(keys)
	return keys
}

func msStrings(list []any) []string {
	out := []string{}
	for _, item := range list {
		if text, ok := item.(string); ok {
			out = append(out, text)
		}
	}
	return out
}

// msCheckGroupBy is the check of a metric alert: the group-by keys are empty (and are inferred)
// or the keys of the query.
func msCheckGroupBy(props map[string]any, arm, kind string) string {
	given := msStrings(msList(props, "groupByKeys"))
	if len(given) == 0 {
		return ""
	}
	promql, _ := msMap(msMap(props, arm), "metricFilter")["promql"].(string)
	inferred := msPromqlKeys(promql)
	if slices.Equal(given, inferred) {
		return ""
	}
	// Recorded: the keys are compared as a set, so a repeated key and another order are accepted.
	sorted := slices.Compact(slices.Sorted(slices.Values(given)))
	if slices.Equal(sorted, inferred) {
		return ""
	}
	keys := "["
	for i, key := range inferred {
		if i > 0 {
			keys += ","
		}
		keys += strconv.Quote(key)
	}
	keys += "]"
	return msCustom("groupByKeys do not match the promql keys("+keys+"): Metric "+kind+" alerts are inferring the group by keys from the promql query. "+
		"You can either provide an empty value and it will be inferred, otherwise it will have to match the promql query's inferred keys", false, []any{"alertDefProperties", "groupByKeys"})
}

func msCheckMetricThreshold(props map[string]any) string {
	arm := msMap(props, "metricThreshold")
	if arm == nil {
		return ""
	}
	if msg := msMTLimits(arm); msg != "" {
		return msg
	}
	if msg := msMTNoDataPolicy(arm); msg != "" {
		return msg
	}
	rules := msList(arm, "rules")
	if msg := msMTWindows(rules); msg != "" {
		return msg
	}
	if msg := msMTRuleSet(rules); msg != "" {
		return msg
	}
	if msg := msMTUndetected(arm, rules); msg != "" {
		return msg
	}
	return msCheckGroupBy(props, "metricThreshold", "threshold")
}

// msMTLimits validates the query and the numeric limits of a metric threshold arm.
func msMTLimits(arm map[string]any) string {
	if promql, _ := msMap(arm, "metricFilter")["promql"].(string); promql == "" {
		return msZod(msZodKV{"code", "too_small"}, msZodKV{"minimum", float64(1)}, msZodKV{"type", "string"}, msZodKV{"inclusive", true}, msZodKV{"exact", false},
			msZodKV{"message", "String must contain at least 1 character(s)"}, msZodKV{"path", msPath("metricThreshold", "metricFilter", "type", "promql")})
	}
	for i, raw := range msList(arm, "rules") {
		rule, _ := raw.(map[string]any)
		cond := msMap(rule, "condition")
		if v, ok := msNum(cond, "forOverPct"); ok && v > 100 {
			return msTooBig(100, msPath("metricThreshold", "rules", i, "condition", "forOverPct"))
		}
	}
	if v, ok := msNum(arm, "evaluationDelayMs"); ok && v > 10800000 {
		return msTooBig(10800000, msPath("metricThreshold", "evaluationDelayMs"))
	}
	if v, ok := msNum(msMap(arm, "missingValues"), "minNonNullValuesPct"); ok && v > 100 {
		return msTooBig(100, msPath("metricThreshold", "missingValues", "missingValues", "minNonNullValuesPct"))
	}
	return ""
}

func msMTNoDataPolicy(arm map[string]any) string {
	policy := msMap(arm, "noDataPolicy")
	if policy == nil {
		return ""
	}
	if v, ok := msNum(policy, "autoRetireSeconds"); ok {
		switch {
		case v <= 0:
			return msZod(msZodKV{"code", "too_small"}, msZodKV{"minimum", float64(0)}, msZodKV{"type", "number"}, msZodKV{"inclusive", false}, msZodKV{"exact", false},
				msZodKV{"message", "Number must be greater than 0"}, msZodKV{"path", msPath("metricThreshold", "noDataPolicy", "autoRetireSeconds")})
		case math.Mod(v, 60) != 0:
			return msZod(msZodKV{"code", "not_multiple_of"}, msZodKV{"multipleOf", float64(60)}, msZodKV{"message", "Number must be a multiple of 60"},
				msZodKV{"path", msPath("metricThreshold", "noDataPolicy", "autoRetireSeconds")})
		}
	}
	return ""
}

// msMTWindows validates the dynamic duration of the time window of every rule.
func msMTWindows(rules []any) string {
	for i, raw := range rules {
		rule, _ := raw.(map[string]any)
		window := msMap(msMap(rule, "condition"), "ofTheLast")
		if text, ok := window["metricTimeWindowDynamicDuration"].(string); ok {
			if minutes := msDurationMinutes(text); minutes < 1 || minutes > 2160 {
				return msCustom("Duration must be within the valid range: 1 - 2160 minutes.", true,
					msPath("metricThreshold", "rules", i, "condition", "ofTheLast", "type", "metricTimeWindowDynamicDuration"))
			}
		}
	}
	return ""
}

// msMTRuleSet validates what the rules have in common: priorities, condition types and the time
// group of the windows.
func msMTRuleSet(rules []any) string {
	seen := map[string]bool{}
	conditionTypes := map[string]bool{}
	for _, raw := range rules {
		rule, _ := raw.(map[string]any)
		priority, _ := msMap(rule, "override")["priority"].(string)
		if seen[priority] {
			return msCustom("Different condition rules cannot have the same priority", true, msPath("metricThreshold", "rules"))
		}
		seen[priority] = true
		kind, _ := msMap(rule, "condition")["conditionType"].(string)
		conditionTypes[kind] = true
	}
	if len(conditionTypes) > 1 {
		return msCustom("Cannot mix thresholds less/more than condition types", true, msPath("metricThreshold", "rules"))
	}
	groups := map[int]bool{}
	for _, raw := range rules {
		rule, _ := raw.(map[string]any)
		groups[msWindowGroup(msWindowMinutes(msMap(msMap(rule, "condition"), "ofTheLast")))] = true
	}
	if len(groups) > 1 {
		return msCustom("Time window is not of the same time group interval ([1 minute - 30 minutes], (30 minutes - 12 hours), [12 hours - 36 hours])", true, msPath("metricThreshold", "rules"))
	}
	return ""
}

func msMTUndetected(arm map[string]any, rules []any) string {
	undetected := msMap(arm, "undetectedValuesManagement")
	if undetected == nil {
		return ""
	}
	trigger, _ := undetected["triggerUndetectedValues"].(bool)
	timeframe, _ := undetected["autoRetireTimeframe"].(string)
	// Recorded: a timeframe other than NEVER needs a "less" rule as well, with no trigger.
	if trigger || (timeframe != "" && timeframe != "AUTO_RETIRE_TIMEFRAME_NEVER_OR_UNSPECIFIED") {
		less := false
		for _, raw := range rules {
			rule, _ := raw.(map[string]any)
			kind, _ := msMap(rule, "condition")["conditionType"].(string)
			less = less || strings.Contains(kind, "LESS_THAN")
		}
		if !less {
			return msCustom("Cannot set undetected values management for alert that doesnt have at least one rule with less than / less than or equals as a condition", false, msPath("metricThreshold"))
		}
	}
	return ""
}

func msCheckMetricAnomaly(props map[string]any) string {
	arm := msMap(props, "metricAnomaly")
	if arm == nil {
		return ""
	}
	if promql, _ := msMap(arm, "metricFilter")["promql"].(string); promql == "" {
		return msZod(msZodKV{"code", "too_small"}, msZodKV{"minimum", float64(1)}, msZodKV{"type", "string"}, msZodKV{"inclusive", true}, msZodKV{"exact", false},
			msZodKV{"message", "String must contain at least 1 character(s)"}, msZodKV{"path", msPath("metricAnomaly", "metricFilter", "type", "promql")})
	}
	rules := msList(arm, "rules")
	if len(rules) != 1 {
		return msZod(msZodKV{"code", "too_big"}, msZodKV{"maximum", float64(1)}, msZodKV{"type", "array"}, msZodKV{"inclusive", true}, msZodKV{"exact", true},
			msZodKV{"message", "Array must contain exactly 1 element(s)"}, msZodKV{"path", msPath("metricAnomaly", "rules")})
	}
	rule, _ := rules[0].(map[string]any)
	condition := msMap(rule, "condition")
	for _, key := range []string{"forOverPct", "minNonNullValuesPct"} {
		if v, ok := msNum(condition, key); ok && v > 100 {
			return msTooBig(100, msPath("metricAnomaly", "rules", 0, "condition", key))
		}
	}
	if v, ok := msNum(arm, "evaluationDelayMs"); ok && v > 10800000 {
		return msTooBig(10800000, msPath("metricAnomaly", "evaluationDelayMs"))
	}
	if v, ok := msNum(msMap(arm, "anomalyAlertSettings"), "percentageOfDeviation"); ok && v < 0 {
		return msTooSmall(0, msPath("metricAnomaly", "anomalyAlertSettings", "percentageOfDeviation"))
	}
	return msCheckGroupBy(props, "metricAnomaly", "anomaly")
}

func msCheckSloThreshold(props map[string]any) string {
	arm := msMap(props, "sloThreshold")
	if arm == nil {
		return ""
	}
	if id, _ := msMap(arm, "sloDefinition")["sloId"].(string); id == "" {
		return msZod(msZodKV{"code", "too_small"}, msZodKV{"minimum", float64(1)}, msZodKV{"type", "string"}, msZodKV{"inclusive", true}, msZodKV{"exact", false},
			msZodKV{"message", "sloId cannot be empty"}, msZodKV{"path", msPath("sloThreshold", "sloDefinition", "sloId")})
	}
	kind, rules := "errorBudget", msList(msMap(arm, "errorBudget"), "rules")
	if burn := msMap(arm, "burnRate"); burn != nil {
		kind, rules = "burnRate", msList(burn, "rules")
		if msg := msSloBurnDurations(burn); msg != "" {
			return msg
		}
	}
	return msSloRules(kind, rules)
}

// msSloBurnDurations validates the time duration of the single and the dual variant of a burn rate.
func msSloBurnDurations(burn map[string]any) string {
	for _, variant := range []string{"single", "dual"} {
		duration := msMap(msMap(burn, variant), "timeDuration")
		if text, ok := duration["duration"].(string); ok {
			v, _ := strconv.ParseFloat(text, 64)
			path := msPath("sloThreshold", "threshold", "burnRate", "type", variant, "timeDuration", "duration")
			if v < 1 {
				return msTooSmall(1, path)
			}
			if v > 48 {
				return msTooBig(48, path)
			}
		}
	}
	return ""
}

// msSloRules validates the rules of an SLO threshold arm of the given kind.
func msSloRules(kind string, rules []any) string {
	for i, raw := range rules {
		rule, _ := raw.(map[string]any)
		if v, ok := msNum(msMap(rule, "condition"), "threshold"); ok && v < 0 {
			return msTooSmall(0, msPath("sloThreshold", "threshold", kind, "rules", i, "condition", "threshold"))
		}
	}
	seen := map[string]bool{}
	for _, raw := range rules {
		rule, _ := raw.(map[string]any)
		priority, _ := msMap(rule, "override")["priority"].(string)
		if seen[priority] {
			return msCustom("Different condition rules cannot have the same priority", true, msPath("sloThreshold", "threshold", kind, "rules"))
		}
		seen[priority] = true
	}
	if kind == "errorBudget" {
		for _, raw := range rules {
			rule, _ := raw.(map[string]any)
			// Probed with an SLO that exists: 101 is accepted and 101.5 is refused, although the message
			// says 100.
			if v, ok := msNum(msMap(rule, "condition"), "threshold"); ok && v > 101 {
				return msCustom("Error budget threshold must be between 0 and 100", true, msPath("sloThreshold", "threshold", "errorBudget", "rules"))
			}
		}
	}
	return ""
}

// A recorded id is replaced by <id:N>, which the replay of a recording sends.
var msUUID = regexp.MustCompile(`^(<id:[0-9]+>|[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12})$`)

func initMetricSloFake() {
	addAlertCheck(msCheckMetricThreshold)
	addAlertCheck(msCheckMetricAnomaly)
	addAlertCheck(msCheckSloThreshold)
	addAlertFill(msFillUndetected)
	addAlertFill(msFillAnomalySettings)
	addAlertFill(msFillGroupBy)
}

// Recorded: no SLO exists in the test tenant for the ids of the scenarios. The API judges the
// request first, and looks the SLO up afterwards. An id that is no UUID is a server error.
func msStatusSloLookup(props map[string]any) (int, any) {
	arm := msMap(props, "sloThreshold")
	if arm == nil || msCheckSloThreshold(props) != "" {
		return 0, nil
	}
	id, _ := msMap(arm, "sloDefinition")["sloId"].(string)
	if msUUID.MatchString(id) {
		return http.StatusNotFound, map[string]any{"code": 404, "message": "Not Found: Failed Getting SLO definition"}
	}
	return http.StatusInternalServerError, map[string]any{"code": 500, "message": "Internal Server Error: Failed Getting SLO definition"}
}

// Recorded: the API keeps undetected values management that equals its zero value (no trigger,
// NEVER) when a rule has a "less" condition, and stores it even if the request has none. For
// other rules it reads back as none.
func msFillUndetected(props map[string]any) {
	arm := msMap(props, "metricThreshold")
	if arm == nil {
		return
	}
	less := false
	for _, raw := range msList(arm, "rules") {
		rule, _ := raw.(map[string]any)
		kind, _ := msMap(rule, "condition")["conditionType"].(string)
		less = less || strings.Contains(kind, "LESS_THAN")
	}
	undetected := msMap(arm, "undetectedValuesManagement")
	if less {
		if undetected == nil {
			undetected = map[string]any{}
			arm["undetectedValuesManagement"] = undetected
		}
		setDefault(undetected, "autoRetireTimeframe", "AUTO_RETIRE_TIMEFRAME_NEVER_OR_UNSPECIFIED")
		setDefault(undetected, "triggerUndetectedValues", false)
		return
	}
	if undetected == nil {
		return
	}
	trigger, _ := undetected["triggerUndetectedValues"].(bool)
	timeframe, _ := undetected["autoRetireTimeframe"].(string)
	if !trigger && (timeframe == "" || timeframe == "AUTO_RETIRE_TIMEFRAME_NEVER_OR_UNSPECIFIED") {
		delete(arm, "undetectedValuesManagement")
	}
}

// Recorded: the API stores an empty anomaly settings message when the request has none.
func msFillAnomalySettings(props map[string]any) {
	if arm := msMap(props, "metricAnomaly"); arm != nil {
		setDefault(arm, "anomalyAlertSettings", map[string]any{})
	}
}

// Recorded: the API infers the group-by keys of a metric alert from the query when the request
// has none.
func msFillGroupBy(props map[string]any) {
	for _, arm := range []string{"metricThreshold", "metricAnomaly"} {
		if msMap(props, arm) == nil || len(msStrings(msList(props, "groupByKeys"))) > 0 {
			continue
		}
		promql, _ := msMap(msMap(props, arm), "metricFilter")["promql"].(string)
		keys := []any{}
		for _, key := range msPromqlKeys(promql) {
			keys = append(keys, key)
		}
		props["groupByKeys"] = keys
	}
}
