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

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// alertStateV1 and alertStateV2 are states that the released provider wrote with schema versions 1 and 2.
// The two differ in the name of the output schema attribute of a routing override. The upgrade reads the
// alert from the API, so every attribute comes back in the shape of the current schema.
func alertOldState(version int64, id string) string {
	overrideKey := map[int64]string{1: "output_schema_id", 2: "payload_type"}[version]
	return fmt.Sprintf(`{
  "id": %q,
  "name": "tfupgrade",
  "enabled": true,
  "priority": "P3",
  "type_definition": {"logs_immediate": {"logs_filter": {"simple_filter": {"lucene_query": "tfbaseline"}}}},
  "schedule": {"active_on": {"days_of_week": ["Monday", "Wednesday"], "start_time": "08:00", "end_time": "17:30", "utc_offset": "+0200"}},
  "notification_group": {"destinations": [{"connector_id": "c", "preset_id": "p", "notify_on": "Triggered Only",
    "triggered_routing_overrides": {"%s": "old"}, "resolved_routing_overrides": {"%s": "old"}}]}
}`, id, overrideKey, overrideKey)
}

func alertID(t *testing.T, state tftypes.Value) string {
	t.Helper()
	var attrs map[string]tftypes.Value
	if err := state.As(&attrs); err != nil {
		t.Fatal(err)
	}
	var id string
	if err := attrs["id"].As(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

// TestAlertStateUpgrade upgrades states of every prior schema version through the protocol. The upgrade
// reads the alert, so the upgraded state equals what a read of the current state gives, and the
// configuration plans without a change on it.
func TestAlertStateUpgrade(t *testing.T) {
	for _, version := range []int64{1, 2} {
		t.Run(fmt.Sprintf("version %d", version), func(t *testing.T) {
			h := newAlertHarness(t)
			cfg := alertLogsImmediate("tfupgrade", j{"priority": "P3", "schedule": j{"active_on": j{
				"days_of_week": l{"Monday", "Wednesday"}, "start_time": "08:00", "end_time": "17:30", "utc_offset": "+0200",
			}}})
			created := h.Apply("create", h.null(), cfg)
			id := alertID(t, created)
			refreshed := h.Refresh("refresh", created)

			upgraded := h.UpgradeFrom(version, "upgrade", alertOldState(version, id))
			if upgraded.IsNull() {
				t.Fatalf("the upgrade of version %d failed: %v", version, h.steps[len(h.steps)-1].ApplyDiags)
			}
			// The destinations of the old state do not exist in the alert, so the API value wins.
			if !upgraded.Equal(refreshed) {
				t.Errorf("the upgraded state differs from a read of the alert:\n%v", diffPaths(refreshed, upgraded))
			}
			replanned, _, diags := h.plan(upgraded, h.config(cfg))
			if hasError(diags) {
				t.Fatalf("plan: %v", diagStrings(diags))
			}
			if !replanned.Equal(upgraded) {
				t.Errorf("the configuration plans a change after the upgrade: %v", diffPaths(upgraded, replanned))
			}
		})
	}

	t.Run("an alert that does not exist", func(t *testing.T) {
		h := newAlertHarness(t)
		h.UpgradeFrom(2, "upgrade", alertOldState(2, "00000000-0000-4000-8000-0000000000ff"))
		diags := h.steps[len(h.steps)-1].ApplyDiags
		if len(diags) == 0 {
			t.Fatal("the upgrade of a missing alert reported no error")
		}
		t.Logf("%v", diags)
	})
}
