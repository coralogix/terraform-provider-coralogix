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
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"

	"github.com/coralogix/terraform-provider-coralogix/internal/provider/alerts"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils/schemadump"
)

const (
	asTypeName  = "coralogix_alerts_scheduler"
	asGoldenDir = "testdata/alerts_scheduler_baseline"
)

// These tests record the behavior of the registered coralogix_alerts_scheduler. After the
// resource becomes generated code, the golden diff must show only listed fixes or message wording.
//
//	UPDATE_GOLDEN=1 go test ./internal/provider -run TestAlertsSchedulerBaseline

func TestAlertsSchedulerBaselineSchema(t *testing.T) {
	var current resource.SchemaResponse
	alerts.NewAlertsSchedulerResource().Schema(context.Background(), resource.SchemaRequest{}, &current)
	checkGoldenAt(t, asGoldenDir, "schema_current.txt", []byte(schemadump.Text(current.Schema)))
}

func TestAlertsSchedulerBaselineScenarios(t *testing.T) {
	for name, run := range asScenarios {
		t.Run(name, func(t *testing.T) {
			h := newProtocolHarness(t, asTypeName, newASFake(t))
			run(h)
			checkGoldenAt(t, asGoldenDir, "scenario_"+name+".json", marshalGolden(t, h.steps))
		})
	}
}

func (h *grHarness) schedulers() *asFake {
	h.t.Helper()
	f, ok := h.api.(*asFake)
	if !ok {
		h.t.Fatal("expected *asFake")
	}
	return f
}

func asTimeFrame(extra j) j {
	return with(j{
		"start_time": "2030-01-04T00:00:00.000",
		"duration":   j{"for_over": float64(2), "frequency": "hours"},
		"time_zone":  "UTC+2",
	}, extra)
}

var asWeekly = j{
	"operation": "active",
	"recurring": j{"dynamic": j{
		"repeat_every":     float64(2),
		"frequency":        j{"weekly": j{"days": l{"Sunday", "Saturday"}}},
		"time_frame":       asTimeFrame(nil),
		"termination_date": "2031-01-01T00:00:00.000",
	}},
}

var asAlways = j{"operation": "mute", "recurring": j{"always_active": true}}

func asRule(name string, schedule j, extra ...j) j {
	out := j{
		"name":     name,
		"filter":   j{"what_expression": "source logs | filter true"},
		"schedule": schedule,
	}
	for _, e := range extra {
		out = with(out, e)
	}
	return out
}

var asFull = asRule("full", asWeekly, j{
	"description": "full rule",
	"enabled":     false,
	"meta_labels": l{j{"key": "team", "value": "a"}, j{"key": "env"}},
	"filter": j{
		"what_expression": "source logs | filter $d.pod == '1'",
		"meta_labels":     l{j{"key": "severity", "value": "high"}},
	},
})

var asScenarios = map[string]func(h *grHarness){
	"minimal": func(h *grHarness) {
		s := h.Apply("create", h.null(), asRule("minimal", asAlways))
		s = h.Refresh("refresh", s)
		h.Destroy("destroy", s)
	},

	"full-lifecycle": func(h *grHarness) {
		s := h.Apply("create", h.null(), asFull)
		s = h.Refresh("refresh", s)
		s = h.Apply("update: change name, description, enabled, and labels", s, with(asFull, j{
			"name":        "full-updated",
			"description": "changed",
			"enabled":     true,
			"meta_labels": l{j{"key": "team", "value": "b"}},
		}))
		h.Destroy("destroy", s)
	},

	"unset-optional-attributes": func(h *grHarness) {
		s := h.Apply("create", h.null(), asFull)
		s = h.Apply("update: only name, filter, and schedule are left", s, asRule("full", asWeekly))
		h.Refresh("refresh", s)
	},

	"filter-alert-ids": func(h *grHarness) {
		cfg := asRule("ids", asAlways, j{"filter": j{
			"what_expression":   "source logs | filter true",
			"alerts_unique_ids": l{"alert-1", "alert-2"},
		}})
		s := h.Apply("create", h.null(), cfg)
		s = h.Refresh("refresh", s)
		s = h.Apply("update: all alerts (no selector)", s, asRule("ids", asAlways))
		h.Apply("update: meta label selector", s, asRule("ids", asAlways, j{"filter": j{
			"what_expression": "source logs | filter true",
			"meta_labels":     l{j{"key": "team", "value": "a"}},
		}}))
	},

	"filter-empty-alert-ids": func(h *grHarness) {
		h.Apply("create with alerts_unique_ids = []", h.null(), asRule("empty-ids", asAlways, j{"filter": j{
			"what_expression":   "source logs | filter true",
			"alerts_unique_ids": l{},
		}}))
	},

	"schedule-one-time": func(h *grHarness) {
		cfg := asRule("once", j{"operation": "mute", "one_time": j{"time_frame": j{
			"start_time": "2030-01-04T00:00:00.000",
			"end_time":   "2030-01-05T00:00:00.000",
			"time_zone":  "UTC",
		}}})
		s := h.Apply("create", h.null(), cfg)
		s = h.Refresh("refresh", s)
		h.Apply("update: duration instead of end_time", s, asRule("once", j{"operation": "mute", "one_time": j{
			"time_frame": asTimeFrame(j{"time_zone": "UTC"}),
		}}))
	},

	"schedule-start-time-formats": func(h *grHarness) {
		s := h.Apply("create with a Z suffix", h.null(), asRule("z", j{"operation": "mute", "one_time": j{
			"time_frame": asTimeFrame(j{"start_time": "2030-01-04T00:00:00Z", "time_zone": "UTC+0"}),
		}}))
		h.Refresh("refresh", s)
	},

	"schedule-recurring-frequencies": func(h *grHarness) {
		dyn := func(freq j) j {
			return j{"operation": "active", "recurring": j{"dynamic": j{
				"repeat_every": float64(1), "frequency": freq, "time_frame": asTimeFrame(nil),
			}}}
		}
		s := h.Apply("create daily", h.null(), asRule("freq", dyn(j{"daily": j{}})))
		s = h.Refresh("refresh", s)
		s = h.Apply("update: monthly", s, asRule("freq", dyn(j{"monthly": j{"days": l{float64(1), float64(15)}}})))
		s = h.Apply("update: weekly", s, asRule("freq", dyn(j{"weekly": j{"days": l{"Monday", "Friday"}}})))
		h.Apply("update: always active", s, asRule("freq", asAlways))
	},

	"config-validation": func(h *grHarness) {
		h.Validate("missing filter", j{"name": "v", "schedule": asAlways})
		h.Validate("missing schedule", j{"name": "v", "filter": j{"what_expression": "x"}})
		h.Validate("both selectors", asRule("v", asAlways, j{"filter": j{
			"what_expression": "x", "alerts_unique_ids": l{"a"}, "meta_labels": l{j{"key": "k"}},
		}}))
		h.Validate("both one_time and recurring", asRule("v", j{
			"operation": "mute", "recurring": j{"always_active": true},
			"one_time": j{"time_frame": asTimeFrame(nil)},
		}))
		h.Validate("neither one_time nor recurring", asRule("v", j{"operation": "mute"}))
		h.Validate("always_active and dynamic", asRule("v", j{"operation": "mute", "recurring": j{
			"always_active": true,
			"dynamic":       j{"repeat_every": float64(1), "frequency": j{"daily": j{}}, "time_frame": asTimeFrame(nil)},
		}}))
		h.Validate("operation in upper case", asRule("v", j{"operation": "MUTE", "recurring": j{"always_active": true}}))
		h.Validate("operation unspecified", asRule("v", j{"operation": "unspecified", "recurring": j{"always_active": true}}))
		h.Validate("unknown time zone", asRule("v", j{"operation": "mute", "one_time": j{"time_frame": asTimeFrame(j{"time_zone": "UTC+15"})}}))
		h.Validate("end_time and duration", asRule("v", j{"operation": "mute", "one_time": j{"time_frame": asTimeFrame(j{"end_time": "2030-01-05T00:00:00.000"})}}))
		h.Validate("weekday in lower case", asRule("v", j{"operation": "mute", "recurring": j{"dynamic": j{
			"repeat_every": float64(1), "frequency": j{"weekly": j{"days": l{"sunday"}}}, "time_frame": asTimeFrame(nil),
		}}}))
		h.Validate("duration frequency in singular", asRule("v", j{"operation": "mute", "one_time": j{
			"time_frame": asTimeFrame(j{"duration": j{"for_over": float64(1), "frequency": "hour"}}),
		}}))
	},

	"import": func(h *grHarness) {
		id := h.schedulers().seed(map[string]any{
			"name": "seeded", "description": "made outside Terraform", "enabled": true,
			"filter": map[string]any{"whatExpression": "source logs | filter true", "alertUniqueIds": map[string]any{"value": []any{"alert-1"}}},
			"schedule": map[string]any{
				"scheduleOperation": "SCHEDULE_OPERATION_MUTE",
				"recurring":         map[string]any{"alwaysActive": map[string]any{}},
			},
		})
		s := h.Import("import", id)
		h.Apply("plan the matching config", s, asRule("seeded", asAlways, j{
			"description": "made outside Terraform",
			"filter":      j{"what_expression": "source logs | filter true", "alerts_unique_ids": l{"alert-1"}},
		}))
	},

	"import-unknown-id": func(h *grHarness) {
		h.Import("import an id that does not exist", "does-not-exist")
	},

	"read-not-found": func(h *grHarness) {
		s := h.Apply("create", h.null(), asRule("gone", asAlways))
		h.schedulers().forget(idOf(s))
		h.Refresh("refresh: deleted outside Terraform", s)
	},

	"update-not-found": func(h *grHarness) {
		s := h.Apply("create", h.null(), asRule("gone", asAlways))
		h.schedulers().forget(idOf(s))
		h.Apply("update: deleted outside Terraform", s, asRule("gone", asAlways, j{"description": "changed"}))
	},

	"delete-not-found": func(h *grHarness) {
		s := h.Apply("create", h.null(), asRule("gone", asAlways))
		h.schedulers().forget(idOf(s))
		h.Destroy("destroy: deleted outside Terraform", s)
	},
}
