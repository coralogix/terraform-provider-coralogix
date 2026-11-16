// Copyright 2026 Coralogix Ltd.
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

import "testing"

// alertLogsRatio is a logs_ratio_threshold alert. group_by_for and group_by are added by the test.
func alertLogsRatio(name string, extra ...j) j {
	filter := j{"simple_filter": j{"lucene_query": "tfbaseline"}}
	out := j{
		"name":     name,
		"priority": "P3",
		"type_definition": j{"logs_ratio_threshold": j{
			"numerator":         filter,
			"numerator_alias":   "numerator",
			"denominator":       filter,
			"denominator_alias": "denominator",
			"rules": l{j{
				"condition": j{"threshold": float64(2), "time_window": "10_MINUTES", "condition_type": "MORE_THAN"},
				"override":  j{"priority": "P3"},
			}},
		}},
	}
	for _, e := range extra {
		out = with(out, e)
	}
	return out
}

func withGroupByFor(cfg j, value string) j {
	out := cloneJSON(cfg).(map[string]any)
	out["type_definition"].(map[string]any)["logs_ratio_threshold"].(map[string]any)["group_by_for"] = value
	return out
}

// "Both" is what the API stores for an alert without group-by keys, and the provider reads it back for
// each of them, so the configuration can say it without group_by. The other two values need a group_by.
func TestAlertLogsRatioGroupByForNeedsGroupByOnlyForTheSideValues(t *testing.T) {
	cases := []struct {
		name      string
		cfg       j
		wantError bool
	}{
		{"Both without group_by", withGroupByFor(alertLogsRatio("ratio"), "Both"), false},
		{"Both with group_by", withGroupByFor(alertLogsRatio("ratio", j{"group_by": l{"coralogix.metadata.alert_id"}}), "Both"), false},
		{"Numerator Only with group_by", withGroupByFor(alertLogsRatio("ratio", j{"group_by": l{"coralogix.metadata.alert_id"}}), "Numerator Only"), false},
		{"Numerator Only without group_by", withGroupByFor(alertLogsRatio("ratio"), "Numerator Only"), true},
		{"Denominator Only with an empty group_by", withGroupByFor(alertLogsRatio("ratio", j{"group_by": l{}}), "Denominator Only"), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newAlertHarness(t)
			diags := h.validate(h.config(tc.cfg))
			if hasError(diags) != tc.wantError {
				t.Fatalf("diagnostics = %v, want an error: %v", diagStrings(diags), tc.wantError)
			}
		})
	}
}

// An alert with group_by_for = "Both" and no group_by creates, reads back, and plans no change.
func TestAlertLogsRatioBothWithoutGroupByReplansEmpty(t *testing.T) {
	h := newAlertHarness(t)
	cfg := withGroupByFor(alertLogsRatio("ratio"), "Both")
	state := h.Apply("create", h.null(), cfg)
	if state.IsNull() {
		t.Fatalf("create failed: %v", h.steps[len(h.steps)-1])
	}
	planned, _, diags := h.plan(state, h.config(cfg))
	if hasError(diags) {
		t.Fatalf("plan: %v", diagStrings(diags))
	}
	if !planned.Equal(state) {
		t.Errorf("the configuration plans a change: %v", diffPaths(state, planned))
	}
}
