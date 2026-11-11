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
	"strings"
	"testing"
)

// The data source reads an alert by its id, with include_access_permissions, and shows the same values
// as the resource.
func TestAlertDataSourceReadsByID(t *testing.T) {
	h := newAlertHarness(t)
	cfg := alertLogsImmediate("tfdatasource", j{"description": "read by id", "schedule": j{"active_on": j{
		"days_of_week": l{"Monday"}, "start_time": "08:00", "end_time": "17:00", "utc_offset": "+0200",
	}}})
	created := h.Apply("create", h.null(), cfg)
	id := alertID(t, created)

	h.ReadDataSource("read by id", j{"id": id})
	step := h.steps[len(h.steps)-1]
	if len(step.ApplyDiags) != 0 {
		t.Fatalf("the read failed: %v", step.ApplyDiags)
	}
	state, ok := step.State.(map[string]any)
	if !ok {
		t.Fatalf("the data source returned no state: %v", step.State)
	}
	if state["id"] != id || state["name"] != "tfdatasource" || state["description"] != "read by id" {
		t.Errorf("state = id %v, name %v, description %v", state["id"], state["name"], state["description"])
	}
	// Both show the schedule.
	if got := toJSON(created).(map[string]any)["schedule"]; got == nil || state["schedule"] == nil {
		t.Errorf("schedule: resource %v, data source %v", got, state["schedule"])
	}
	var queried bool
	for _, request := range step.Requests {
		queried = queried || (request.Method == "GET" && strings.Contains(request.Path, "include_access_permissions=true"))
	}
	if !queried {
		t.Errorf("no Get with include_access_permissions: %v", step.Requests)
	}

	h.ReadDataSource("read an id that does not exist", j{"id": "00000000-0000-4000-8000-0000000000ff"})
	if diags := h.steps[len(h.steps)-1].ApplyDiags; len(diags) == 0 {
		t.Error("a missing alert reported no error")
	}
}
