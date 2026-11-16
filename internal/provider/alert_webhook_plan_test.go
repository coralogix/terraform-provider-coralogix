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

func alertWithWebhooks(webhooks ...j) j {
	items := make(l, len(webhooks))
	for i, w := range webhooks {
		items[i] = w
	}
	return alertLogsImmediate("webhooks", j{"notification_group": j{"webhooks_settings": items}})
}

// The API returns neither notify_on nor retriggering_period for a webhook that sets only its integration,
// so the state holds null for both. The configuration must plan no change.
func TestAlertWebhookWithoutAdvancedNotificationReplansEmpty(t *testing.T) {
	cases := []struct {
		name string
		cfg  j
	}{
		{"integration only", alertWithWebhooks(j{"integration_id": "123"})},
		{"recipients only", alertWithWebhooks(j{"recipients": l{"a@example.com"}})},
		{"two webhooks, one with Advanced Notification", alertWithWebhooks(
			j{"integration_id": "123"},
			j{"integration_id": "456", "notify_on": "Triggered and Resolved", "retriggering_period": j{"minutes": float64(7)}},
		)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newAlertHarness(t)
			state := h.Apply("create", h.null(), tc.cfg)
			if state.IsNull() {
				t.Fatalf("create failed: %v", h.steps[len(h.steps)-1])
			}
			planned, _, diags := h.plan(state, h.config(tc.cfg))
			if hasError(diags) {
				t.Fatalf("plan: %v", diagStrings(diags))
			}
			if !planned.Equal(state) {
				t.Errorf("the configuration plans a change: %v", diffPaths(state, planned))
			}
		})
	}
}

// Adding notify_on and the period to a webhook without them applies and replans empty. Removing them again
// keeps the values: both attributes are Optional+Computed, so a value that the configuration leaves out is
// the one in state. The plan must still be empty then, not a change to (known after apply).
func TestAlertWebhookAdvancedNotificationAddedThenRemovedReplansEmpty(t *testing.T) {
	h := newAlertHarness(t)
	without := alertWithWebhooks(j{"integration_id": "123"})
	with := alertWithWebhooks(j{"integration_id": "123", "notify_on": "Triggered and Resolved", "retriggering_period": j{"minutes": float64(7)}})

	state := h.Apply("create", h.null(), without)
	state = h.Apply("add", state, with)
	if state.IsNull() {
		t.Fatalf("add failed: %v", h.steps[len(h.steps)-1])
	}
	planned, _, diags := h.plan(state, h.config(with))
	if hasError(diags) || !planned.Equal(state) {
		t.Fatalf("the configuration with the values plans a change: %v %v", diagStrings(diags), diffPaths(state, planned))
	}
	planned, _, diags = h.plan(state, h.config(without))
	if hasError(diags) {
		t.Fatalf("plan: %v", diagStrings(diags))
	}
	if !planned.Equal(state) {
		t.Errorf("the configuration without the values plans a change: %v", diffPaths(state, planned))
	}
}
