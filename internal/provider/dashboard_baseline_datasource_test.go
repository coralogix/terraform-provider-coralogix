// Copyright 2024 Coralogix Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package provider

import (
	"errors"
	"os"
	"slices"
	"testing"
)

// The data source looks a dashboard up by id and flattens it with an empty prior model, so a widget
// can read differently there than in the resource. Each walk base is read by id once. The generated
// configs set every id, so the goldens do not hold random ids.
func init() {
	for _, b := range walkBases() {
		dashboardDataSourceScenarios["read-"+b.name] = func(h *grHarness) {
			if os.Getenv("DASHBOARD_WALK") != "all" && !slices.Contains(walkSmokeBases, b.name) {
				h.t.Skip("set DASHBOARD_WALK=all to read every walk base")
			}
			cfg, _ := b.config(0, nil)
			s := h.Apply("create the dashboard that the data source reads", h.null(), cfg)
			if s.IsNull() {
				return
			}
			h.steps = nil
			h.ReadDataSource("read by id", byID(idOf(s)))
		}
	}
}

var dashboardDataSourceScenarios = map[string]func(*grHarness){
	"no-id":    func(h *grHarness) { h.ReadDataSource("read without id", j{}) },
	"empty-id": func(h *grHarness) { h.ReadDataSource("read an empty id", byID("")) },
	"not-found": func(h *grHarness) {
		h.ReadDataSource("read an id that does not exist", byID("missing-dashboard-001"))
	},
	"api-error": func(h *grHarness) {
		id := h.dashboards().seed(map[string]any{"name": "seeded", "layout": map[string]any{"sections": []any{}}}, "")
		h.dashboards().failNext("Get", 500, "dashboards service unavailable")
		h.ReadDataSource("data source API unavailable", byID(id))
	},
	"transport-data-source": func(h *grHarness) {
		id := h.dashboards().seed(map[string]any{"name": "seeded", "layout": map[string]any{"sections": []any{}}}, "")
		h.dashboards().failTransport(errors.New("dashboards transport unavailable"))
		h.ReadDataSource("data source transport failure", byID(id))
		h.dashboards().failTransport(nil)
		h.ReadDataSource("retry the data source read", byID(id))
	},
	"read-minimal": func(h *grHarness) {
		s := h.Apply("create", h.null(), dashboardConfig())
		h.ReadDataSource("read by id", byID(idOf(s)))
	},
	"read-access-policy": func(h *grHarness) {
		id := h.dashboards().seed(map[string]any{"name": "seeded", "layout": map[string]any{"sections": []any{}}}, dashboardPolicyReordered)
		h.ReadDataSource("read a dashboard that has a policy", byID(id))
	},
	"read-content-json": func(h *grHarness) {
		s := h.Apply("create from content_json", h.null(), contentJSONConfig(dashboardContentJSON(h.t, "content_json_canonical.json")))
		h.ReadDataSource("read it by id", byID(idOf(s)))
	},
	"read-content-json-legacy-query": func(h *grHarness) {
		id := h.dashboards().seed(mustJSONObject(h.t, dashboardContentJSON(h.t, "content_json_dynamic_legacy_query.json")), "")
		h.ReadDataSource("read a dynamic widget with the deprecated top-level query", byID(id))
	},
	"read-deprecated-variables": func(h *grHarness) {
		cfg := dashboardWith(dashboardConfig(), j{"variables": []any{j{"name": "env", "display_name": "Environment",
			"definition": j{"multi_select": j{"source": j{"constant_list": []any{"production", "staging"}},
				"selected_values": []any{"production"}, "values_order_direction": "asc"}}}}})
		s := h.Apply("create with a legacy variable", h.null(), cfg)
		h.ReadDataSource("read it by id", byID(idOf(s)))
	},
	"read-null-dashboard": func(h *grHarness) {
		id := h.dashboards().seed(map[string]any{"name": "seeded", "layout": map[string]any{"sections": []any{}}}, "")
		h.dashboards().replyNext("Get", j{"dashboard": nil, "permissions": dashboardPermissions()})
		h.ReadDataSource("read answers a null dashboard", byID(id))
	},
}

// marshalDataSourceGolden masks the random ids of the dashboard goldens. The other data sources
// have none.
func marshalDataSourceGolden(t *testing.T, name string, steps any) []byte {
	if name == "dashboard" {
		return marshalDashboardGolden(t, steps)
	}
	return marshalGolden(t, steps)
}
