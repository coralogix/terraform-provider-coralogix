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

import "fmt"

// Scenarios of the attributes that every alert type shares: the top-level attributes, the incident
// settings, the notification group, and the schedule. The host type is logs_immediate, the
// simplest one. Each alert type has its own scenarios in alert_baseline_<group>_test.go.

var alertSchedule = j{"active_on": j{
	"days_of_week": l{"Monday", "Wednesday"},
	"start_time":   "08:00",
	"end_time":     "17:30",
}}

// alertZodError is how the API words a custom validation failure on alertDefProperties.
func alertZodError(message string) string {
	return "[\n  {\n    \"code\": \"custom\",\n    \"message\": \"" + message + "\",\n    \"path\": [\n      \"alertDefProperties\"\n    ]\n  }\n]"
}

func init() {
	// The rules below are recorded answers of the real API.
	addAlertCheck(func(props map[string]any) string {
		group, _ := props["notificationGroup"].(map[string]any)
		if destinations, _ := group["destinations"].([]any); len(destinations) > 0 {
			// No connector exists on the recording tenant under a made-up id.
			first, _ := destinations[0].(map[string]any)
			return "Invalid destinations: {\"reason\":\"Connector not found: " + fmt.Sprint(first["connectorId"]) + "\"}"
		}
		webhooks, _ := group["webhooks"].([]any)
		withNotify, withPeriod := 0, 0
		for _, raw := range webhooks {
			hook, _ := raw.(map[string]any)
			if n, ok := hook["notifyOn"].(string); ok && n != "" {
				withNotify++
			}
			if _, ok := hook["minutes"]; ok {
				withPeriod++
			}
		}
		if len(webhooks) > 0 && (withNotify != withPeriod || (withNotify != 0 && withNotify != len(webhooks))) {
			return alertZodError("RetriggeringPeriod and notifyOn should either be defined for all webhooks or for none of them.")
		}
		if router, ok := group["router"].(map[string]any); ok {
			incidents, _ := props["incidentsSettings"].(map[string]any)
			if rn, ok := router["notifyOn"].(string); ok && rn != "" && incidents != nil && incidents["notifyOn"] != rn {
				return alertZodError("notificationGroup.router.notifyOn must match incidentsSettings.notifyOn (omit it to inherit incidentsSettings.notifyOn)")
			}
		}
		return ""
	})

	addAlertScenario("common-top-level-attributes", func(h *grHarness) {
		full := alertLogsImmediate("tfbaseline-common-top", j{
			"description": "baseline",
			"enabled":     false,
			"priority":    "P2",
			"labels":      j{"team": "a", "env": "test"},
		})
		s := h.Apply("create with every top-level attribute", h.null(), full)
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: change every attribute", s, with(full, j{
			"description": "changed", "enabled": true, "priority": "P4", "labels": j{"team": "b"},
		}))
		s = h.Apply("update: remove description, labels, and priority", s, alertLogsImmediate("tfbaseline-common-top", j{"enabled": true}))
		s = h.RefreshPlan("refresh after removal", s)
		h.Destroy("destroy", s)
	})

	addAlertScenario("common-top-level-omitted", func(h *grHarness) {
		s := h.Apply("create with only name and type_definition", h.null(), alertLogsImmediate("tfbaseline-common-omitted"))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: set description to empty string", s, alertLogsImmediate("tfbaseline-common-omitted", j{"description": ""}))
		h.RefreshPlan("refresh", s)
	})

	addAlertScenario("common-priority-values", func(h *grHarness) {
		var s = h.null()
		for _, p := range []string{"P1", "P2", "P3", "P4", "P5"} {
			s = h.Apply("priority "+p, s, alertLogsImmediate("tfbaseline-common-priority", j{"priority": p}))
		}
		h.RefreshPlan("refresh", s)
	})

	addAlertScenario("common-labels", func(h *grHarness) {
		s := h.Apply("create with labels", h.null(), alertLogsImmediate("tfbaseline-common-labels", j{"labels": j{"a": "1", "b": "2"}}))
		s = h.Apply("update: replace labels", s, alertLogsImmediate("tfbaseline-common-labels", j{"labels": j{"c": "3"}}))
		s = h.Apply("update: remove labels", s, alertLogsImmediate("tfbaseline-common-labels"))
		s = h.RefreshPlan("refresh", s)
		h.Apply("update: add labels again", s, alertLogsImmediate("tfbaseline-common-labels", j{"labels": j{"a": "1"}}))
	})

	addAlertScenario("common-phantom-mode", func(h *grHarness) {
		s := h.Apply("create", h.null(), alertLogsImmediate("tfbaseline-common-phantom", j{"phantom_mode": true}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: phantom_mode false", s, alertLogsImmediate("tfbaseline-common-phantom", j{"phantom_mode": false}))
		s = h.Apply("update: remove phantom_mode", s, alertLogsImmediate("tfbaseline-common-phantom"))
		h.RefreshPlan("refresh", s)
	})

	addAlertScenario("common-incidents-settings", func(h *grHarness) {
		cfg := func(extra j) j { return alertLogsImmediate("tfbaseline-common-incidents", extra) }
		s := h.Apply("create with incidents_settings", h.null(), cfg(j{"incidents_settings": j{
			"notify_on": "Triggered Only", "retriggering_period": j{"minutes": float64(15)},
		}}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: triggered and resolved, 30 minutes", s, cfg(j{"incidents_settings": j{
			"notify_on": "Triggered and Resolved", "retriggering_period": j{"minutes": float64(30)},
		}}))
		s = h.Apply("update: remove incidents_settings", s, cfg(nil))
		s = h.RefreshPlan("refresh after removal", s)
		h.Destroy("destroy", s)
	})

	addAlertScenario("common-schedule", func(h *grHarness) {
		cfg := func(extra j) j { return alertLogsImmediate("tfbaseline-common-schedule", extra) }
		s := h.Apply("create with a schedule", h.null(), cfg(j{"schedule": alertSchedule}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: days, times, and offset", s, cfg(j{"schedule": j{"active_on": j{
			"days_of_week": l{"Sunday", "Saturday"}, "start_time": "22:00", "end_time": "06:00", "utc_offset": "+0200",
		}}}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: remove the schedule", s, cfg(nil))
		s = h.RefreshPlan("refresh after removal", s)
		h.Apply("update: add the schedule again", s, cfg(j{"schedule": alertSchedule}))
	})

	addAlertScenario("common-notification-group-recipients", func(h *grHarness) {
		cfg := func(extra j) j { return alertLogsImmediate("tfbaseline-common-recipients", extra) }
		recipients := func(emails ...any) j {
			return j{"notification_group": j{"webhooks_settings": l{j{"recipients": emails}}}}
		}
		s := h.Apply("create with email recipients", h.null(), cfg(recipients("a@example.com", "b@example.com")))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: one recipient, notify_on, and retriggering_period", s, cfg(j{"notification_group": j{"webhooks_settings": l{j{
			"recipients": l{"a@example.com"}, "notify_on": "Triggered and Resolved", "retriggering_period": j{"minutes": float64(20)},
		}}}}))
		s = h.RefreshPlan("refresh", s)
		// The API refuses notify_on without retriggering_period on a webhook.
		h.Apply("update: notify_on without retriggering_period", s, cfg(j{"notification_group": j{"webhooks_settings": l{j{
			"recipients": l{"a@example.com"}, "notify_on": "Triggered Only",
		}}}}))
		s = h.Apply("update: remove notification_group", s, cfg(nil))
		h.RefreshPlan("refresh after removal", s)
	})

	addAlertScenario("common-notification-group-router", func(h *grHarness) {
		cfg := func(extra j) j { return alertLogsImmediate("tfbaseline-common-router", extra) }
		s := h.Apply("create with the default router", h.null(), cfg(j{"notification_group": j{"router": j{"id": "router_default"}}}))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: router and incidents_settings notify on triggered and resolved", s, cfg(j{
			"incidents_settings": j{"notify_on": "Triggered and Resolved", "retriggering_period": j{"minutes": float64(10)}},
			"notification_group": j{"router": j{"id": "router_default", "notify_on": "Triggered and Resolved"}},
		}))
		// The API refuses a router notify_on that differs from incidents_settings.notify_on.
		s = h.Apply("update: router notify_on differs from incidents_settings", s, cfg(j{
			"incidents_settings": j{"notify_on": "Triggered and Resolved", "retriggering_period": j{"minutes": float64(10)}},
			"notification_group": j{"router": j{"id": "router_default", "notify_on": "Triggered Only"}},
		}))
		s = h.Apply("update: router without an id", s, cfg(j{"notification_group": j{"router": j{}}}))
		s = h.Apply("update: remove notification_group", s, cfg(nil))
		h.RefreshPlan("refresh", s)
	})

	addAlertScenario("common-notification-group-destinations-unknown-connector", func(h *grHarness) {
		// The ids are made up, so the real API refuses them. The answer is part of the baseline.
		h.Apply("create with a destination that does not exist", h.null(), alertLogsImmediate("tfbaseline-common-destination", j{
			"notification_group": j{"destinations": l{j{"connector_id": "does-not-exist", "preset_id": "does-not-exist"}}},
		}))
	})

	addAlertScenario("common-config-validation", func(h *grHarness) {
		bad := func(name string, extra j) {
			h.validateSorted(name, alertLogsImmediate("tfbaseline-common-validation", extra))
		}
		bad("priority outside P1..P5", j{"priority": "P6"})
		bad("lowercase priority", j{"priority": "p1"})
		bad("empty labels map", j{"labels": j{}})
		bad("incidents_settings.notify_on outside the list", j{"incidents_settings": j{"notify_on": "Never", "retriggering_period": j{"minutes": float64(10)}}})
		bad("incidents_settings without retriggering_period", j{"incidents_settings": j{"notify_on": "Triggered Only"}})
		bad("schedule start_time without leading zero", j{"schedule": j{"active_on": j{"days_of_week": l{"Monday"}, "start_time": "8:00", "end_time": "17:00"}}})
		bad("schedule day name outside the list", j{"schedule": j{"active_on": j{"days_of_week": l{"Mon"}, "start_time": "08:00", "end_time": "17:00"}}})
		bad("schedule utc_offset with a colon", j{"schedule": j{"active_on": j{"days_of_week": l{"Monday"}, "start_time": "08:00", "end_time": "17:00", "utc_offset": "+02:00"}}})
		bad("empty destinations list", j{"notification_group": j{"destinations": l{}}})
		bad("empty group_by_keys", j{"notification_group": j{"group_by_keys": l{}}})
		bad("webhook with neither integration_id nor recipients", j{"notification_group": j{"webhooks_settings": l{j{"notify_on": "Triggered Only"}}}})
		bad("webhook with both integration_id and recipients", j{"notification_group": j{"webhooks_settings": l{j{"integration_id": "1", "recipients": l{"a@example.com"}}}}})
		bad("webhook notify_on outside the list", j{"notification_group": j{"webhooks_settings": l{j{"recipients": l{"a@example.com"}, "notify_on": "Always"}}}})
		bad("empty data_sources list", j{"data_sources": l{}})
		h.validateSorted("empty name", alertLogsImmediate("", nil))
		h.validateSorted("no type_definition", j{"name": "tfbaseline-common-validation"})
		h.validateSorted("no name", j{"type_definition": j{"logs_immediate": j{}}})
		h.validateSorted("two type arms", j{"name": "tfbaseline-common-validation", "type_definition": j{
			"logs_immediate": j{}, "tracing_immediate": j{},
		}})
		h.validateSorted("empty type_definition", j{"name": "tfbaseline-common-validation", "type_definition": j{}})
	})

	addAlertScenario("common-import", func(h *grHarness) {
		cfg := alertLogsImmediate("tfbaseline-common-import", j{"description": "imported", "priority": "P3"})
		s := h.Apply("create", h.null(), cfg)
		imported := h.Import("import by id", idOf(s))
		_ = imported
		h.Plan("plan the same config on the imported state", imported, cfg)
		h.Destroy("destroy", s)
		h.Import("import an id that does not exist", "00000000-0000-0000-0000-000000000000")
		h.Import("import something that is no id", "not-an-id")
	})

	addAlertScenario("common-deleted-outside-terraform", func(h *grHarness) {
		cfg := alertLogsImmediate("tfbaseline-common-gone")
		s := h.Apply("create", h.null(), cfg)
		h.alertBackend().forget(idOf(s))
		gone := h.Refresh("refresh after the alert was deleted outside", s)
		h.Apply("update after the alert was deleted outside", s, alertLogsImmediate("tfbaseline-common-gone", j{"description": "x"}))
		h.Destroy("destroy after the alert was deleted outside", s)
		_ = gone
	})

	addAlertScenario("common-type-switch", func(h *grHarness) {
		s := h.Apply("create logs_immediate", h.null(), alertLogsImmediate("tfbaseline-common-switch"))
		s = h.Apply("update: switch to logs_threshold", s, j{
			"name": "tfbaseline-common-switch",
			"type_definition": j{"logs_threshold": j{"rules": l{j{
				"condition": j{"condition_type": "MORE_THAN", "threshold": float64(2), "time_window": "10_MINUTES"},
				"override":  j{"priority": "P2"},
			}}}},
		})
		s = h.RefreshPlan("refresh", s)
		h.Apply("update: switch back to logs_immediate", s, alertLogsImmediate("tfbaseline-common-switch"))
	})
}
