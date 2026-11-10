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
	"strings"
)

// The lifecycle of an slo_threshold alert. An SLO alert needs an SLO that exists, so every scenario
// here makes a temporary request-based SLO through alertBackend.seedSLO. The recorder deletes the
// SLOs at the end of its run, after the alerts. The scenarios of alert_baseline_metric_slo_test.go
// cover the configuration and the SLO ids that do not exist.
//
// Recorded facts about the real API:
//   - The SLO service answers Create with {"slo": ..., "exclusionWindowIds": []}, and Delete with
//     {"effectedSloAlertIds": [...]}. Deleting an SLO deletes the alerts that refer to it: they are
//     gone (404) afterwards, and the answer lists their ids.
//   - The API takes group_by from the SLO: groupByKeys always reads back as the by-clause labels of
//     the SLO query, and what the request sends is ignored.

// sloSeedBody is the SLO that the scenarios create: a request-based SLO whose queries group by
// the given labels. An APM SLO is no option, because the test tenant has no service in its catalog.
func sloSeedBody(name string, by ...string) map[string]any {
	suffix := ""
	if len(by) > 0 {
		suffix = " by (" + strings.Join(by, ", ") + ")"
	}
	return map[string]any{
		"name":                      name,
		"description":               "",
		"sloTimeFrame":              "SLO_TIME_FRAME_7_DAYS",
		"targetThresholdPercentage": 99.5,
		"requestBasedMetricSli": map[string]any{
			"goodEvents":  map[string]any{"query": `sum(rate(http_requests_total{status="200"}[1m]))` + suffix},
			"totalEvents": map[string]any{"query": `sum(rate(http_requests_total[1m]))` + suffix},
		},
	}
}

var alertSLOLabelUnsafe = regexp.MustCompile(`[^a-z0-9-]+`)

// alertSLOLabel names the SLOs of a recording run after the scenario, so a leftover is easy to find.
func alertSLOLabel(testName string) string {
	if i := strings.LastIndex(testName, "/"); i >= 0 {
		testName = testName[i+1:]
	}
	label := alertSLOLabelUnsafe.ReplaceAllString(strings.ToLower(testName), "-")
	if len(label) > 60 {
		label = label[:60]
	}
	return label
}

// ---- the fake ----

// sloFake returns the SLO service of the fake: the SLO baseline already checked it against the
// real service. Its ids start high, so that they never equal the ids of the alerts.
func (f *alertFake) sloFake() *sloFake {
	if f.sloStore == nil {
		f.sloStore = &sloFake{slos: map[string]map[string]any{}, seq: 5000}
	}
	return f.sloStore
}

// handleSLO answers a request to the SLO service. rest is the path after the base path.
func (f *alertFake) handleSLO(method, rest string, body any) (int, any) {
	path := sloBasePath
	if rest != "" {
		path += "/" + rest
	}
	status, answer := f.sloFake().handle(method, path, body)
	if method == http.MethodPost && status == http.StatusOK {
		// Recorded: the answer to a Create has the empty description, although the store drops it.
		slo := cloneMap(answer.(map[string]any)["slo"].(map[string]any))
		setDefault(slo, "description", "")
		answer.(map[string]any)["slo"] = slo
	}
	if method != http.MethodDelete || status != http.StatusOK {
		return status, answer
	}
	// Recorded: deleting an SLO deletes its alerts too, and the answer lists their ids.
	gone := []string{}
	for id, stored := range f.alerts {
		props, _ := stored["alertDefProperties"].(map[string]any)
		if sloIDOfAlert(props) == rest {
			gone = append(gone, id)
			delete(f.alerts, id)
		}
	}
	slices.Sort(gone)
	ids := make([]any, len(gone))
	for i, id := range gone {
		ids[i] = id
	}
	return http.StatusOK, map[string]any{"effectedSloAlertIds": ids}
}

func sloIDOfAlert(props map[string]any) string {
	id, _ := msMap(msMap(props, "sloThreshold"), "sloDefinition")["sloId"].(string)
	return id
}

func (f *alertFake) seedSLO(by ...string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	status, body := f.sloFake().write(sloSeedBody("tfbaseline-slo", by...), false)
	if status != http.StatusOK {
		panic(fmt.Sprintf("seed SLO failed: %d %v", status, body))
	}
	return body.(map[string]any)["slo"].(map[string]any)["id"].(string)
}

func (f *alertFake) forgetSLO(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if status, body := f.handleSLO(http.MethodDelete, id, nil); status != http.StatusOK {
		panic(fmt.Sprintf("forget SLO failed: %d %v", status, body))
	}
}

// ---- the real tenant ----

func (l *alertLive) seedSLO(by ...string) string {
	name := fmt.Sprintf("tfbaseline-slo-%s-%d", l.label, len(l.slos)+1)
	status, answer := alertDirectVia(l, http.MethodPost, sloBasePath, sloSeedBody(name, by...))
	id, _ := asObject(asObject(answer)["slo"])["id"].(string)
	if status != http.StatusOK || id == "" {
		panic(fmt.Sprintf("seed SLO failed: %d %v", status, answer))
	}
	return id
}

func (l *alertLive) forgetSLO(id string) {
	if status, answer := alertDirectVia(l, http.MethodDelete, sloBasePath+"/"+id, nil); status != http.StatusOK {
		panic(fmt.Sprintf("forget SLO failed: %d %v", status, answer))
	}
}

// ---- the rules of the fake ----

func init() {
	addAlertStoreCheck(msStoreSloLookup)
	addAlertStoreFill(msFillSloGroupBy)
	addAlertFill(msFillSloDurationUnit)
}

// msFillSloDurationUnit is recorded: the API stores the unit UNSPECIFIED of a burn rate as HOURS.
func msFillSloDurationUnit(props map[string]any) {
	burn := msMap(msMap(props, "sloThreshold"), "burnRate")
	for _, variant := range []string{"single", "dual"} {
		duration := msMap(msMap(burn, variant), "timeDuration")
		if duration["unit"] == "DURATION_UNIT_UNSPECIFIED" {
			duration["unit"] = "DURATION_UNIT_HOURS"
		}
	}
}

// msStoreSloLookup is the lookup of the SLO: an alert that refers to an SLO of the fake is
// accepted, and any other id gets the answers that msStatusSloLookup records.
func msStoreSloLookup(f *alertFake, _ string, props map[string]any) (int, any) {
	slo, found := f.sloFake().slos[sloIDOfAlert(props)]
	if !found {
		return msStatusSloLookup(props)
	}
	if msCheckSloThreshold(props) == "" && msSloBurnRateOutOfRange(msMap(props, "sloThreshold"), slo) {
		return http.StatusBadRequest, map[string]any{"code": 400, "message": "Bad Request: Failed getting SLO metric alert"}
	}
	return 0, nil
}

// msSloBurnRateOutOfRange is recorded with an SLO that exists: a burn rate threshold of 0 is
// refused, and so is one above 1 / (1 - target), which is 200 for an SLO with a target of 99.5.
func msSloBurnRateOutOfRange(arm, slo map[string]any) bool {
	rules := msList(msMap(arm, "burnRate"), "rules")
	target, _ := msNum(slo, "targetThresholdPercentage")
	limit := math.Round(1/(1-target/100)*1e6) / 1e6
	for _, raw := range rules {
		rule, _ := raw.(map[string]any)
		if v, ok := msNum(msMap(rule, "condition"), "threshold"); ok && (v <= 0 || v > limit) {
			return true
		}
	}
	return false
}

// msFillSloGroupBy is recorded: the API ignores group_by and stores the by-clause labels of the
// SLO query, an empty list for an SLO without a by clause.
func msFillSloGroupBy(f *alertFake, props map[string]any) {
	if msMap(props, "sloThreshold") == nil {
		return
	}
	slo, found := f.sloFake().slos[sloIDOfAlert(props)]
	if !found {
		return
	}
	labels := msList(msMap(slo, "grouping"), "labels")
	props["groupByKeys"] = append([]any{}, labels...)
}

// ---- the scenarios ----

func init() {
	addAlertScenario("slo-threshold-error-budget-lifecycle", sloErrorBudgetLifecycle)
	addAlertScenario("slo-threshold-error-budget-full", sloErrorBudgetFull)
	addAlertScenario("slo-threshold-burn-rate-single-lifecycle", func(h *grHarness) { sloBurnRateLifecycle(h, "single") })
	addAlertScenario("slo-threshold-burn-rate-dual-lifecycle", func(h *grHarness) { sloBurnRateLifecycle(h, "dual") })
	addAlertScenario("slo-threshold-update-each-field", sloUpdateEachField)
	addAlertScenario("slo-threshold-optional-removal", sloOptionalRemoval)
	addAlertScenario("slo-threshold-override-priority-values", sloOverridePriorityValues)
	addAlertScenario("slo-threshold-burn-rate-duration-values", sloBurnRateDurationValues)
	addAlertScenario("slo-threshold-rules-collection", sloRulesCollection)
	addAlertScenario("slo-threshold-threshold-values", sloThresholdValues)
	addAlertScenario("slo-threshold-group-by", sloGroupByScenario)
	addAlertScenario("slo-threshold-switch-arms", sloSwitchArms)
	addAlertScenario("slo-threshold-switch-type", sloSwitchType)
	addAlertScenario("slo-threshold-change-slo", sloChangeSLO)
	addAlertScenario("slo-threshold-slo-deleted-outside-terraform", sloDeletedOutside)
	addAlertScenario("slo-threshold-alert-deleted-outside-terraform", sloAlertDeletedOutside)
	addAlertScenario("slo-threshold-import-seeded", sloImportSeeded)
	addAlertScenario("slo-threshold-update-invalid-requests", sloUpdateInvalid)
}

// sloFullTop is every optional top-level attribute of an alert.
func sloFullTop() j {
	return j{"description": "slo baseline", "priority": "P3", "enabled": false, "group_by": l{"service"}}
}

func sloErrorBudgetLifecycle(h *grHarness) {
	id := h.alertBackend().seedSLO("service")
	cfg := sloAlert("tfbaseline-slo-eb", sloErrorBudget(id, sloRule(10, nil)))
	s := h.Apply("create with only the required attributes", h.null(), cfg)
	s = msRefreshPlan(h, "refresh", s)
	imported := h.Import("import by id", idOf(s))
	h.Plan("plan the same config on the imported state", imported, cfg)
	s = h.Apply("update: rename", s, sloAlert("tfbaseline-slo-eb-2", sloErrorBudget(id, sloRule(10, nil))))
	s = h.Apply("update: threshold", s, sloAlert("tfbaseline-slo-eb-2", sloErrorBudget(id, sloRule(20.5, nil))))
	s = msRefreshPlan(h, "refresh after the update", s)
	h.Destroy("destroy", s)
}

func sloErrorBudgetFull(h *grHarness) {
	id := h.alertBackend().seedSLO("service")
	rules := func(base float64) []any {
		return []any{
			sloRule(base, j{"priority": "P1"}), sloRule(base+10, j{"priority": "P2"}), sloRule(base+20, j{"priority": "P3"}),
			sloRule(base+30, j{"priority": "P4"}), sloRule(base+40, j{"priority": "P5"}),
		}
	}
	cfg := sloAlert("tfbaseline-slo-eb-full", sloErrorBudget(id, rules(5)...), sloFullTop())
	s := h.Apply("create with every optional attribute and five rules", h.null(), cfg)
	s = msRefreshPlan(h, "refresh", s)
	imported := h.Import("import by id", idOf(s))
	h.Plan("plan the same config on the imported state", imported, cfg)
	s = h.Apply("update: change every attribute", s, sloAlert("tfbaseline-slo-eb-full", sloErrorBudget(id, rules(1)...),
		j{"description": "changed", "priority": "P1", "enabled": true, "group_by": l{"service"}}))
	s = msRefreshPlan(h, "refresh after the update", s)
	h.Destroy("destroy", s)
}

func sloBurnRateLifecycle(h *grHarness, kind string) {
	id := h.alertBackend().seedSLO("service")
	name := "tfbaseline-slo-br-" + kind
	cfg := sloAlert(name, sloBurnRate(id, kind, 2, sloRule(14, nil)))
	s := h.Apply("create with only the required attributes", h.null(), cfg)
	s = msRefreshPlan(h, "refresh", s)
	imported := h.Import("import by id", idOf(s))
	h.Plan("plan the same config on the imported state", imported, cfg)
	s = h.Apply("update: duration", s, sloAlert(name, sloBurnRate(id, kind, 6, sloRule(14, nil))))
	s = h.Apply("update: threshold", s, sloAlert(name, sloBurnRate(id, kind, 6, sloRule(3.5, nil))))
	s = h.Apply("update: every optional attribute", s, sloAlert(name, sloBurnRate(id, kind, 6, sloRule(3.5, j{"priority": "P2"})), sloFullTop()))
	s = msRefreshPlan(h, "refresh", s)
	imported = h.Import("import by id", idOf(s))
	h.Plan("plan the full config on the imported state", imported, sloAlert(name, sloBurnRate(id, kind, 6, sloRule(3.5, j{"priority": "P2"})), sloFullTop()))
	h.Destroy("destroy", s)
}

func sloUpdateEachField(h *grHarness) {
	id := h.alertBackend().seedSLO("service")
	other := h.alertBackend().seedSLO("service")
	name := "tfbaseline-slo-fields"
	eb := func(sloID string, rule j, top ...j) j { return sloAlert(name, sloErrorBudget(sloID, rule), top...) }
	s := h.Apply("create", h.null(), eb(id, sloRule(10, nil)))
	s = h.Apply("update: threshold", s, eb(id, sloRule(25, nil)))
	s = h.Apply("update: override.priority", s, eb(id, sloRule(25, j{"priority": "P2"})))
	s = h.Apply("update: description", s, eb(id, sloRule(25, j{"priority": "P2"}), j{"description": "d1"}))
	s = h.Apply("update: top-level priority", s, eb(id, sloRule(25, j{"priority": "P2"}), j{"description": "d1", "priority": "P4"}))
	s = h.Apply("update: enabled false", s, eb(id, sloRule(25, j{"priority": "P2"}), j{"description": "d1", "priority": "P4", "enabled": false}))
	s = h.Apply("update: group_by", s, eb(id, sloRule(25, j{"priority": "P2"}), j{"description": "d1", "priority": "P4", "enabled": false, "group_by": l{"service"}}))
	s = msRefreshPlan(h, "refresh", s)
	s = h.Apply("update: slo_id to another SLO", s, eb(other, sloRule(25, j{"priority": "P2"}), j{"description": "d1", "priority": "P4", "enabled": false, "group_by": l{"service"}}))
	s = msRefreshPlan(h, "refresh after the SLO changed", s)
	h.Destroy("destroy", s)
}

func sloOptionalRemoval(h *grHarness) {
	id := h.alertBackend().seedSLO("service")
	name := "tfbaseline-slo-removal"
	full := sloAlert(name, sloErrorBudget(id, sloRule(10, j{"priority": "P2"})), sloFullTop())
	s := h.Apply("create with every optional attribute", h.null(), full)
	s = h.Apply("update: remove group_by", s, sloAlert(name, sloErrorBudget(id, sloRule(10, j{"priority": "P2"})), j{"description": "slo baseline", "priority": "P3", "enabled": false}))
	s = msRefreshPlan(h, "refresh", s)
	s = h.Apply("update: remove the override priority", s, sloAlert(name, sloErrorBudget(id, sloRule(10, nil)), j{"description": "slo baseline", "priority": "P3", "enabled": false}))
	s = msRefreshPlan(h, "refresh", s)
	s = h.Apply("update: remove priority and enabled", s, sloAlert(name, sloErrorBudget(id, sloRule(10, nil)), j{"description": "slo baseline"}))
	s = msRefreshPlan(h, "refresh", s)
	// Recorded in the other scenarios: a Replace without description keeps the stored one.
	s = h.Apply("update: remove description", s, sloAlert(name, sloErrorBudget(id, sloRule(10, nil))))
	s = msRefreshPlan(h, "refresh", s)
	h.Destroy("destroy", s)
}

func sloOverridePriorityValues(h *grHarness) {
	id := h.alertBackend().seedSLO("service")
	s := h.null()
	for _, p := range []string{"P1", "P2", "P3", "P4", "P5"} {
		s = h.Apply("override.priority "+p, s, sloAlert("tfbaseline-slo-override", sloErrorBudget(id, sloRule(10, j{"priority": p}))))
	}
	s = msRefreshPlan(h, "refresh", s)
	s = h.Apply("override.priority omitted: it falls back to the top-level priority", s,
		sloAlert("tfbaseline-slo-override", sloErrorBudget(id, sloRule(10, nil)), j{"priority": "P1"}))
	s = msRefreshPlan(h, "refresh", s)
	for _, p := range []string{"P1", "P2", "P3", "P4", "P5"} {
		s = h.Apply("top-level priority "+p, s, sloAlert("tfbaseline-slo-override", sloErrorBudget(id, sloRule(10, nil)), j{"priority": p}))
	}
	msRefreshPlan(h, "refresh", s)
}

func sloBurnRateDurationValues(h *grHarness) {
	id := h.alertBackend().seedSLO("service")
	name := "tfbaseline-slo-duration"
	for _, kind := range []string{"single", "dual"} {
		s := h.null()
		for _, hours := range []float64{1, 2, 24, 48} {
			s = h.Apply(fmt.Sprintf("%s: %v hours", kind, hours), s, sloAlert(name, sloBurnRate(id, kind, hours, sloRule(2, nil))))
		}
		s = msRefreshPlan(h, "refresh", s)
		unspecified := j{"slo_definition": j{"slo_id": id}, "burn_rate": j{
			kind:    j{"time_duration": j{"duration": float64(3), "unit": "UNSPECIFIED"}},
			"rules": l{sloRule(2, nil)},
		}}
		s = h.Apply(kind+": unit UNSPECIFIED", s, sloAlert(name, unspecified))
		s = msRefreshPlan(h, "refresh", s)
		for _, hours := range []float64{0, 49} {
			h.Apply(fmt.Sprintf("%s: %v hours is refused", kind, hours), s, sloAlert(name, sloBurnRate(id, kind, hours, sloRule(2, nil))))
		}
		h.Destroy("destroy "+kind, s)
	}
}

func sloRulesCollection(h *grHarness) {
	id := h.alertBackend().seedSLO("service")
	name := "tfbaseline-slo-rules"
	r := func(threshold float64, p string) j { return sloRule(threshold, j{"priority": p}) }
	eb := func(rules ...any) j { return sloAlert(name, sloErrorBudget(id, rules...)) }
	br := func(rules ...any) j { return sloAlert(name, sloBurnRate(id, "single", 1, rules...)) }
	s := h.Apply("one rule", h.null(), eb(r(10, "P1")))
	s = h.Apply("two rules", s, eb(r(10, "P1"), r(20, "P2")))
	s = h.Apply("five rules", s, eb(r(10, "P1"), r(20, "P2"), r(30, "P3"), r(40, "P4"), r(50, "P5")))
	s = msRefreshPlan(h, "refresh", s)
	s = h.Apply("the same rules in another order", s, eb(r(50, "P5"), r(40, "P4"), r(30, "P3"), r(20, "P2"), r(10, "P1")))
	s = msRefreshPlan(h, "refresh after the reorder", s)
	s = h.Apply("shrink to two rules", s, eb(r(50, "P5"), r(10, "P1")))
	s = msRefreshPlan(h, "refresh", s)
	h.Apply("six rules are refused", s, eb(r(1, "P1"), r(2, "P2"), r(3, "P3"), r(4, "P4"), r(5, "P5"), r(6, "P1")))
	h.Apply("two rules with the same priority are refused", s, eb(r(10, "P1"), r(20, "P1")))
	h.Apply("two rules without a priority: both fall back to the same priority", s, eb(sloRule(10, nil), sloRule(20, nil)))
	s = h.Apply("burn_rate: five rules", s, br(r(1, "P1"), r(2, "P2"), r(3, "P3"), r(4, "P4"), r(5, "P5")))
	s = msRefreshPlan(h, "refresh", s)
	s = h.Apply("burn_rate: shrink to one rule", s, br(r(1, "P1")))
	s = msRefreshPlan(h, "refresh", s)
	h.Apply("burn_rate: six rules are refused", s, br(r(1, "P1"), r(2, "P2"), r(3, "P3"), r(4, "P4"), r(5, "P5"), r(6, "P1")))
	h.Destroy("destroy", s)
}

func sloThresholdValues(h *grHarness) {
	id := h.alertBackend().seedSLO("service")
	name := "tfbaseline-slo-values"
	for _, tc := range []struct {
		arm    string
		values []float64
	}{
		{"error_budget", []float64{0, 100, 101, 0.0001, 99.9999, 33.333}},
		{"burn_rate", []float64{0, 1, 14.4, 200, 200.5, 1000, 1e9, 0.5}},
	} {
		s := h.null()
		for _, v := range tc.values {
			arm := sloErrorBudget(id, sloRule(v, nil))
			if tc.arm == "burn_rate" {
				arm = sloBurnRate(id, "single", 1, sloRule(v, nil))
			}
			s = h.Apply(fmt.Sprintf("%s threshold %v", tc.arm, v), s, sloAlert(name, arm))
		}
		s = msRefreshPlan(h, "refresh", s)
		h.Destroy("destroy "+tc.arm, s)
	}
	// Recorded: the API accepts 101 and refuses 101.5, and the message says "between 0 and 100". A
	// burn rate threshold of 0 or above 1 / (1 - target) (200 for this SLO) is refused.
	s := h.Apply("error_budget threshold 101.5 is refused", h.null(), sloAlert(name, sloErrorBudget(id, sloRule(101.5, nil))))
	s = h.Apply("negative error_budget threshold is refused", s, sloAlert(name, sloErrorBudget(id, sloRule(-1, nil))))
	h.Apply("negative burn_rate threshold is refused", s, sloAlert(name, sloBurnRate(id, "single", 1, sloRule(-1, nil))))
}

// sloGroupByScenario drives group_by. Recorded: the API stores the by-clause labels of the SLO
// whatever the request sends, so a group_by that differs from them gives an inconsistent result.
func sloGroupByScenario(h *grHarness) {
	grouped := h.alertBackend().seedSLO("service")
	multi := h.alertBackend().seedSLO("service", "env")
	plain := h.alertBackend().seedSLO()
	build := func(id string, top ...j) j {
		return sloAlert("tfbaseline-slo-group", sloErrorBudget(id, sloRule(10, nil)), top...)
	}
	by := func(keys ...any) j { return j{"group_by": l(keys)} }

	s := h.Apply("grouped SLO, group_by omitted: the plan makes it unknown and the API fills it", h.null(), build(grouped))
	s = msRefreshPlan(h, "refresh", s)
	s = h.Apply("grouped SLO, group_by with the label of the SLO", s, build(grouped, by("service")))
	s = msRefreshPlan(h, "refresh", s)
	s = h.Apply("remove group_by: the state keeps the labels", s, build(grouped))
	s = msRefreshPlan(h, "refresh after the removal", s)
	s = h.Apply("explicit empty group_by on a grouped SLO", s, build(grouped, by()))
	s = msRefreshPlan(h, "refresh", s)
	s = h.Apply("group_by with another key than the SLO has", s, build(grouped, by("pod")))
	s = msRefreshPlan(h, "refresh", s)
	s = h.Apply("group_by with an extra key", s, build(grouped, by("service", "pod")))
	s = msRefreshPlan(h, "refresh", s)
	s = h.Apply("an SLO with two labels, group_by omitted", s, build(multi))
	s = msRefreshPlan(h, "refresh", s)
	s = h.Apply("an SLO with two labels, group_by in another order", s, build(multi, by("env", "service")))
	s = msRefreshPlan(h, "refresh", s)
	s = h.Apply("an SLO with two labels, group_by with one of them", s, build(multi, by("env")))
	s = msRefreshPlan(h, "refresh", s)
	s = h.Apply("an SLO without labels, group_by omitted", s, build(plain))
	s = msRefreshPlan(h, "refresh", s)
	s = h.Apply("an SLO without labels, group_by with a key", s, build(plain, by("service")))
	s = msRefreshPlan(h, "refresh", s)
	s = h.Apply("an SLO without labels, an explicit empty group_by", s, build(plain, by()))
	s = msRefreshPlan(h, "refresh", s)
	h.Destroy("destroy", s)
}

func sloSwitchArms(h *grHarness) {
	id := h.alertBackend().seedSLO("service")
	name := "tfbaseline-slo-switch"
	r := sloRule(5, j{"priority": "P2"})
	s := h.Apply("create error_budget", h.null(), sloAlert(name, sloErrorBudget(id, r)))
	s = h.Apply("update: switch to a single burn_rate", s, sloAlert(name, sloBurnRate(id, "single", 2, r)))
	s = msRefreshPlan(h, "refresh", s)
	s = h.Apply("update: switch to a dual burn_rate", s, sloAlert(name, sloBurnRate(id, "dual", 4, r)))
	s = msRefreshPlan(h, "refresh", s)
	s = h.Apply("update: switch back to a single burn_rate", s, sloAlert(name, sloBurnRate(id, "single", 4, r)))
	s = msRefreshPlan(h, "refresh", s)
	s = h.Apply("update: switch back to error_budget", s, sloAlert(name, sloErrorBudget(id, r)))
	s = msRefreshPlan(h, "refresh", s)
	imported := h.Import("import by id", idOf(s))
	h.Plan("plan the config on the imported state", imported, sloAlert(name, sloErrorBudget(id, r)))
	h.Destroy("destroy", s)
}

func sloSwitchType(h *grHarness) {
	id := h.alertBackend().seedSLO("service")
	name := "tfbaseline-slo-type-switch"
	s := h.Apply("create logs_immediate", h.null(), alertLogsImmediate(name))
	s = h.Apply("update: switch to slo_threshold", s, sloAlert(name, sloErrorBudget(id, sloRule(10, nil))))
	s = msRefreshPlan(h, "refresh", s)
	s = h.Apply("update: switch back to logs_immediate", s, alertLogsImmediate(name))
	s = msRefreshPlan(h, "refresh", s)
	s = h.Apply("update: switch to a burn_rate slo_threshold", s, sloAlert(name, sloBurnRate(id, "dual", 1, sloRule(10, nil))))
	s = msRefreshPlan(h, "refresh", s)
	s = h.Apply("update: switch to metric_threshold", s, mtAlert(name, nil))
	s = msRefreshPlan(h, "refresh", s)
	h.Destroy("destroy", s)
}

func sloChangeSLO(h *grHarness) {
	grouped := h.alertBackend().seedSLO("service")
	plain := h.alertBackend().seedSLO()
	name := "tfbaseline-slo-change"
	cfg := func(id string) j { return sloAlert(name, sloErrorBudget(id, sloRule(10, nil))) }
	s := h.Apply("create on a grouped SLO", h.null(), cfg(grouped))
	s = h.Apply("update: another SLO without labels (group_by was filled by the API)", s, cfg(plain))
	s = msRefreshPlan(h, "refresh", s)
	s = h.Apply("update: back to the grouped SLO", s, cfg(grouped))
	s = msRefreshPlan(h, "refresh", s)
	// Recorded: the PUT answers 404 for an SLO that does not exist, and the provider reads that as
	// "the alert is gone": a warning and Missing Resource State After Update, not the SLO error.
	// The alert still exists, so each failed update below starts from the same state.
	h.Apply("update: an SLO id that no SLO has", s, cfg(sloMissingID))
	h.Apply("update: an id that is no id", s, cfg(sloMissingText))
	h.Destroy("destroy", s)
}

// sloDeletedOutside deletes the SLO, and the API deletes the alert with it.
func sloDeletedOutside(h *grHarness) {
	id := h.alertBackend().seedSLO("service")
	cfg := sloAlert("tfbaseline-slo-gone", sloErrorBudget(id, sloRule(10, nil)))
	s := h.Apply("create", h.null(), cfg)
	h.alertBackend().forgetSLO(id)
	h.Plan("plan the same config, no refresh", s, cfg)
	gone := msRefresh(h, "refresh after the SLO was deleted outside: the alert is gone too", s)
	h.Plan("plan the config on the state that the refresh left", gone, cfg)
	h.Apply("update after the SLO was deleted outside", s, sloAlert("tfbaseline-slo-gone", sloErrorBudget(id, sloRule(10, nil)), j{"description": "x"}))
	h.Destroy("destroy after the SLO was deleted outside", s)
	h.Apply("create again: the SLO is still gone", h.null(), cfg)
	fresh := h.alertBackend().seedSLO("service")
	s = h.Apply("create on a new SLO", h.null(), sloAlert("tfbaseline-slo-gone", sloErrorBudget(fresh, sloRule(10, nil))))
	h.Destroy("destroy", s)
}

// sloAlertDeletedOutside deletes only the alert, and the SLO stays.
func sloAlertDeletedOutside(h *grHarness) {
	id := h.alertBackend().seedSLO("service")
	cfg := sloAlert("tfbaseline-slo-alert-gone", sloErrorBudget(id, sloRule(10, nil)))
	s := h.Apply("create", h.null(), cfg)
	h.alertBackend().forget(idOf(s))
	msRefresh(h, "refresh after the alert was deleted outside", s)
	h.Apply("update after the alert was deleted outside", s, sloAlert("tfbaseline-slo-alert-gone", sloErrorBudget(id, sloRule(10, nil)), j{"description": "x"}))
	h.Destroy("destroy after the alert was deleted outside", s)
	s = h.Apply("create again on the same SLO", h.null(), cfg)
	h.Destroy("destroy", s)
}

// sloImportSeeded imports alerts that were made outside Terraform, as a user of the API makes them.
func sloImportSeeded(h *grHarness) {
	sloID := h.alertBackend().seedSLO("service")
	rule := func(threshold float64, priority string) map[string]any {
		return map[string]any{
			"condition": map[string]any{"threshold": threshold},
			"override":  map[string]any{"priority": priority},
		}
	}
	seed := func(name string, arm map[string]any) string {
		arm["sloDefinition"] = map[string]any{"sloId": sloID}
		return h.alertBackend().seed(map[string]any{
			"name": name, "enabled": true, "priority": "ALERT_DEF_PRIORITY_P3",
			"type": "ALERT_DEF_TYPE_SLO_THRESHOLD", "sloThreshold": arm,
		})
	}
	budgetID := seed("tfbaseline-slo-seeded-eb", map[string]any{"errorBudget": map[string]any{"rules": []any{rule(10, "ALERT_DEF_PRIORITY_P2")}}})
	imported := h.Import("import an error_budget alert", budgetID)
	cfg := sloAlert("tfbaseline-slo-seeded-eb", sloErrorBudget(sloID, sloRule(10, j{"priority": "P2"})), j{"priority": "P3"})
	h.Plan("plan a config that matches it", imported, cfg)
	s := h.Apply("apply the matching config", imported, cfg)
	h.Destroy("destroy", s)

	burnID := seed("tfbaseline-slo-seeded-br", map[string]any{"burnRate": map[string]any{
		"dual":  map[string]any{"timeDuration": map[string]any{"duration": "3", "unit": "DURATION_UNIT_HOURS"}},
		"rules": []any{rule(2, "ALERT_DEF_PRIORITY_P1"), rule(4, "ALERT_DEF_PRIORITY_P2")},
	}})
	imported = h.Import("import a dual burn_rate alert", burnID)
	cfg = sloAlert("tfbaseline-slo-seeded-br", sloBurnRate(sloID, "dual", 3, sloRule(2, j{"priority": "P1"}), sloRule(4, j{"priority": "P2"})), j{"priority": "P3"})
	h.Plan("plan a config that matches it", imported, cfg)
	s = h.Apply("apply the matching config", imported, cfg)
	h.Destroy("destroy", s)

	singleID := seed("tfbaseline-slo-seeded-single", map[string]any{"burnRate": map[string]any{
		"single": map[string]any{"timeDuration": map[string]any{"duration": "5", "unit": "DURATION_UNIT_HOURS"}},
		"rules":  []any{rule(7, "ALERT_DEF_PRIORITY_P4")},
	}})
	imported = h.Import("import a single burn_rate alert", singleID)
	h.Plan("plan the config of the error_budget alert on it", imported, cfg)
	h.Destroy("destroy", imported)
}

// sloUpdateInvalid sends requests that the API refuses on an alert that exists, with an SLO that
// exists too, so the API cannot refuse them for the SLO.
func sloUpdateInvalid(h *grHarness) {
	id := h.alertBackend().seedSLO("service")
	name := "tfbaseline-slo-invalid-update"
	s := h.Apply("create", h.null(), sloAlert(name, sloErrorBudget(id, sloRule(10, nil))))
	s = h.Apply("update: threshold 101 is accepted", s, sloAlert(name, sloErrorBudget(id, sloRule(101, nil))))
	s = h.Apply("update: threshold 150 is refused", s, sloAlert(name, sloErrorBudget(id, sloRule(150, nil))))
	s = msRefreshPlan(h, "refresh after the failed update", s)
	s = h.Apply("update: negative threshold", s, sloAlert(name, sloErrorBudget(id, sloRule(-1, nil))))
	s = h.Apply("update: same priority on two rules", s, sloAlert(name, sloErrorBudget(id, sloRule(1, j{"priority": "P1"}), sloRule(2, j{"priority": "P1"}))))
	s = h.Apply("update: burn_rate with a duration of 0", s, sloAlert(name, sloBurnRate(id, "single", 0, sloRule(1, nil))))
	s = h.Apply("update: burn_rate with a duration of 100000", s, sloAlert(name, sloBurnRate(id, "dual", 100000, sloRule(1, nil))))
	s = h.Apply("update: empty slo_id", s, sloAlert(name, sloErrorBudget("", sloRule(1, nil))))
	s = msRefreshPlan(h, "refresh", s)
	h.Destroy("destroy", s)
}
