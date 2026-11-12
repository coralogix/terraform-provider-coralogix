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
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func init() {
	for name, run := range dashboardMoreScenarios {
		dashboardScenarios[name] = run
	}
	for name, run := range dashboardCheckMoreScenarios {
		dashboardCheckScenarios[name] = run
	}
}

const dashboardPolicyJSON = `{"version":"2025-01-01","default":{"permissions":{"team-dashboards:Read":"grant"}},"rules":[]}`

// dashboardPolicyReordered is dashboardPolicyJSON with other key order and other spacing.
const dashboardPolicyReordered = `{ "rules": [], "default": {"permissions": {"team-dashboards:Read": "grant"}}, "version": "2025-01-01" }`

const dashboardPolicyOther = `{"version":"2025-01-01","default":{"permissions":{"team-dashboards:Read":"deny"}},"rules":[]}`

func dashboardContentJSON(t testing.TB, file string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "dashboards", file))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func contentJSONConfig(content string) j { return j{"content_json": content} }

func mustJSONObject(t testing.TB, text string) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

var dashboardMoreScenarios = map[string]func(*grHarness){
	// --- top-level fields ---
	"folder-id": func(h *grHarness) {
		h.dashboards().addFolder("11111111-1111-4111-8111-111111111111", "first")
		h.dashboards().addFolder("22222222-2222-4222-8222-222222222222", "second")
		cfg := dashboardWith(dashboardConfig(), j{"folder": j{"id": "11111111-1111-4111-8111-111111111111"}})
		s := h.Apply("create in a folder by id", h.null(), cfg)
		s = h.RefreshPlan("refresh and plan", s)
		cfg = dashboardWith(cfg, j{"folder": j{"id": "22222222-2222-4222-8222-222222222222"}})
		s = h.Apply("move to another folder", s, cfg)
		cfg = dashboardWith(cfg, j{"folder": nil})
		s = h.Apply("remove the folder", s, cfg)
		s = h.RefreshPlan("refresh and plan after removing the folder", s)
		h.Destroy("destroy", s)
	},
	"folder-path": func(h *grHarness) {
		h.dashboards().addFolder("11111111-1111-4111-8111-111111111111", "first")
		cfg := dashboardWith(dashboardConfig(), j{"folder": j{"path": "Team/Sub"}})
		s := h.Apply("create in a folder by path", h.null(), cfg)
		s = h.RefreshPlan("refresh and plan", s)
		cfg = dashboardWith(cfg, j{"folder": j{"id": "11111111-1111-4111-8111-111111111111"}})
		s = h.Apply("switch from path to id", s, cfg)
		s = h.RefreshPlan("refresh and plan after the switch", s)
		h.Destroy("destroy", s)
	},
	"time-frame": func(h *grHarness) {
		cfg := dashboardWith(dashboardConfig(), j{"time_frame": j{"relative": j{"duration": "seconds:900"}}})
		s := h.Apply("create with a relative time frame", h.null(), cfg)
		s = h.RefreshPlan("refresh and plan", s)
		cfg = dashboardWith(cfg, j{"time_frame": j{"absolute": j{"start": "2024-01-01T00:00:00Z", "end": "2024-01-02T00:00:00Z"}}})
		s = h.Apply("switch to an absolute time frame", s, cfg)
		s = h.RefreshPlan("refresh and plan after the switch", s)
		cfg = dashboardWith(cfg, j{"time_frame": nil})
		s = h.Apply("remove the time frame", s, cfg)
		s = h.RefreshPlan("refresh and plan after removing it", s)
		h.Destroy("destroy", s)
	},
	"auto-refresh": func(h *grHarness) {
		cfg := dashboardWith(dashboardConfig(), j{"auto_refresh": j{"type": "two_minutes"}})
		s := h.Apply("create with auto refresh", h.null(), cfg)
		s = h.RefreshPlan("refresh and plan", s)
		cfg = dashboardWith(cfg, j{"auto_refresh": j{"type": "five_minutes"}})
		s = h.Apply("change auto refresh", s, cfg)
		cfg = dashboardWith(cfg, j{"auto_refresh": nil})
		s = h.Apply("remove auto refresh", s, cfg)
		s = h.RefreshPlan("refresh and plan after removing it", s)
		h.Destroy("destroy", s)
	},
	"filters": func(h *grHarness) {
		filter := func(value string) j {
			return j{"source": j{"metrics": j{"metric_name": "http_requests_total", "label": "status",
				"operator": j{"type": "equals", "selected_values": []any{value}}}}}
		}
		cfg := dashboardWith(dashboardConfig(), j{"filters": []any{filter("200")}})
		s := h.Apply("create with a filter", h.null(), cfg)
		s = h.RefreshPlan("refresh and plan", s)
		cfg = dashboardWith(cfg, j{"filters": []any{filter("500")}})
		s = h.Apply("change the filter", s, cfg)
		cfg = dashboardWith(cfg, j{"filters": nil})
		s = h.Apply("remove filters", s, cfg)
		s = h.RefreshPlan("refresh and plan after removing them", s)
		h.Destroy("destroy", s)
	},
	"annotations": func(h *grHarness) {
		annotation := func(name string) j {
			return j{"name": name, "source": j{"metrics": j{"promql_query": "vector(1)", "strategy": j{"start_time": j{}}}}}
		}
		cfg := dashboardWith(dashboardConfig(), j{"annotations": []any{annotation("first")}})
		s := h.Apply("create with an annotation", h.null(), cfg)
		s = h.RefreshPlan("refresh and plan", s)
		cfg = dashboardWith(cfg, j{"annotations": []any{annotation("renamed")}})
		s = h.Apply("rename the annotation", s, cfg)
		cfg = dashboardWith(cfg, j{"annotations": nil})
		s = h.Apply("remove annotations", s, cfg)
		s = h.RefreshPlan("refresh and plan after removing them", s)
		h.Destroy("destroy", s)
	},
	"variables-v2": func(h *grHarness) {
		variable := func(value string) j {
			return j{"name": "environment", "display_name": "Environment",
				"source": j{"static": j{"values_order_direction": "none", "all_option": j{"include_all": false},
					"values": []any{j{"value": "production", "is_default": true}, j{"value": "staging"}}}},
				"value": j{"single_string": j{"value": value}}}
		}
		cfg := dashboardWith(dashboardConfig(), j{"variables_v2": []any{variable("production")}})
		s := h.Apply("create with a variable", h.null(), cfg)
		s = h.RefreshPlan("refresh and plan", s)
		cfg = dashboardWith(cfg, j{"variables_v2": []any{variable("staging")}})
		s = h.Apply("change the selected value", s, cfg)
		cfg = dashboardWith(cfg, j{"variables_v2": nil})
		s = h.Apply("remove variables", s, cfg)
		s = h.RefreshPlan("refresh and plan after removing them", s)
		h.Destroy("destroy", s)
	},

	// --- access_policy ---
	"access-policy": func(h *grHarness) {
		cfg := dashboardWith(dashboardConfig(), j{"access_policy": dashboardPolicyJSON})
		s := h.Apply("create with an access policy", h.null(), cfg)
		s = h.RefreshPlan("refresh and plan", s)
		h.Apply("apply the same policy with other key order and spacing", s, dashboardWith(cfg, j{"access_policy": dashboardPolicyReordered}))
		cfg = dashboardWith(cfg, j{"access_policy": dashboardPolicyOther})
		s = h.Apply("change the access policy", s, cfg)
		s = h.RefreshPlan("refresh and plan after the change", s)
		cfg = dashboardWith(cfg, j{"access_policy": nil})
		s = h.Apply("remove the access policy from the config", s, cfg)
		s = h.RefreshPlan("refresh and plan after removing it", s)
		h.Destroy("destroy", s)
	},
	"access-policy-not-set": func(h *grHarness) {
		s := h.Apply("create without an access policy", h.null(), dashboardConfig())
		s = h.RefreshPlan("refresh and plan", s)
		h.Destroy("destroy", s)
	},
	"access-policy-blank": func(h *grHarness) {
		for _, blank := range []string{"", "  "} {
			cfg := dashboardWith(dashboardConfig(), j{"access_policy": blank})
			s := h.Apply("create with a blank access policy "+`"`+blank+`"`, h.null(), cfg)
			s = h.RefreshPlan("refresh and plan", s)
			cfg = dashboardWith(cfg, j{"access_policy": dashboardPolicyJSON})
			s = h.Apply("set a policy on top of the blank one", s, cfg)
			h.Destroy("destroy", s)
		}
	},
	"access-policy-changed-outside": func(h *grHarness) {
		cfg := dashboardWith(dashboardConfig(), j{"access_policy": dashboardPolicyJSON})
		s := h.Apply("create with an access policy", h.null(), cfg)
		h.dashboards().setPolicy(idOf(s), dashboardPolicyOther)
		s = h.RefreshPlan("refresh and plan after the policy changed outside Terraform", s)
		h.Destroy("destroy", s)
	},
	"access-policy-cleared-outside": func(h *grHarness) {
		cfg := dashboardWith(dashboardConfig(), j{"access_policy": dashboardPolicyJSON})
		s := h.Apply("create with an access policy", h.null(), cfg)
		h.dashboards().setPolicy(idOf(s), "")
		s = h.RefreshPlan("refresh and plan after the API returns an empty policy", s)
		h.Destroy("destroy", s)
	},
	"access-policy-import": func(h *grHarness) {
		id := h.dashboards().seed(map[string]any{"name": "seeded", "layout": map[string]any{"sections": []any{}}}, dashboardPolicyReordered)
		s := h.Import("import a dashboard that has a policy", id)
		h.Destroy("destroy", s)
	},
	"access-policy-with-content-json": func(h *grHarness) {
		cfg := dashboardWith(contentJSONConfig(dashboardContentJSON(h.t, "content_json_canonical.json")), j{"access_policy": dashboardPolicyJSON})
		s := h.Apply("create content_json with an access policy", h.null(), cfg)
		s = h.RefreshPlan("refresh and plan", s)
		cfg = dashboardWith(cfg, j{"access_policy": dashboardPolicyOther})
		s = h.Apply("change the policy", s, cfg)
		h.Destroy("destroy", s)
	},
	"access-policy-unknown": func(h *grHarness) {
		cfg := dashboardWith(dashboardConfig(), j{"access_policy": unknownJSON{}})
		h.PlanWithRequests("plan with an unknown access policy", h.null(), cfg)
	},

	// --- content_json ---
	"content-json-canonical":  func(h *grHarness) { contentJSONLifecycle(h, "content_json_canonical.json") },
	"content-json-snake-case": func(h *grHarness) { contentJSONLifecycle(h, "content_json_snake_case.json") },
	"content-json-unknown-fields": func(h *grHarness) {
		contentJSONLifecycle(h, "content_json_unknown_fields.json")
	},
	"content-json-dynamic-queries-table": func(h *grHarness) {
		contentJSONLifecycle(h, "content_json_dynamic_queries_table.json")
	},
	"content-json-dynamic-table-unselected-unions": func(h *grHarness) {
		contentJSONLifecycle(h, "content_json_dynamic_table_unselected_unions.json")
	},
	"content-json-dynamic-unsupported-visualization": func(h *grHarness) {
		contentJSONLifecycle(h, "content_json_dynamic_unsupported_visualization.json")
	},
	"content-json-no-time-frame": func(h *grHarness) {
		// The typed read of this dashboard has no arm of the time frame to show, which the typed schema
		// reports as an error. content_json manages the dashboard, so the read must still work.
		contentJSONLifecycle(h, "content_json_no_time_frame.json")
	},
	"content-json-dynamic-legacy-query": func(h *grHarness) {
		contentJSONLifecycle(h, "content_json_dynamic_legacy_query.json")
	},
	"content-json-invalid": func(h *grHarness) {
		h.Apply("create with text that is not JSON", h.null(), contentJSONConfig("{not json"))
		h.Apply("create with a JSON array", h.null(), contentJSONConfig("[]"))
		h.Apply("create with an empty object", h.null(), contentJSONConfig("{}"))
	},
	"content-json-unknown": func(h *grHarness) {
		h.PlanWithRequests("plan with an unknown content_json", h.null(), j{"content_json": unknownJSON{}})
	},
	"content-json-to-hcl": func(h *grHarness) {
		s := h.Apply("create from content_json", h.null(), contentJSONConfig(dashboardContentJSON(h.t, "content_json_canonical.json")))
		s = h.Apply("switch to typed HCL", s, dashboardConfig())
		s = h.RefreshPlan("refresh and plan", s)
		h.Destroy("destroy", s)
	},
	"hcl-to-content-json": func(h *grHarness) {
		s := h.Apply("create from typed HCL", h.null(), dashboardConfig())
		s = h.Apply("switch to content_json", s, contentJSONConfig(dashboardContentJSON(h.t, "content_json_canonical.json")))
		s = h.RefreshPlan("refresh and plan", s)
		h.Destroy("destroy", s)
	},
	"content-json-import": func(h *grHarness) {
		s := h.Apply("create from content_json", h.null(), contentJSONConfig(dashboardContentJSON(h.t, "content_json_canonical.json")))
		imported := h.Import("import it", idOf(s))
		h.Destroy("destroy", imported)
	},
	"content-json-folder": func(h *grHarness) {
		h.dashboards().addFolder("11111111-1111-4111-8111-111111111111", "first")
		cfg := dashboardWith(contentJSONConfig(dashboardContentJSON(h.t, "content_json_canonical.json")), j{"folder": j{"id": "11111111-1111-4111-8111-111111111111"}})
		s := h.Apply("create content_json next to a folder block", h.null(), cfg)
		s = h.RefreshPlan("refresh and plan", s)
		h.Destroy("destroy", s)
	},
	"content-json-auto-refresh": func(h *grHarness) {
		cfg := dashboardWith(contentJSONConfig(dashboardContentJSON(h.t, "content_json_canonical.json")), j{"auto_refresh": j{"type": "two_minutes"}})
		s := h.Apply("create content_json next to auto_refresh", h.null(), cfg)
		s = h.RefreshPlan("refresh and plan", s)
		h.Destroy("destroy", s)
	},

	// --- deprecated fields ---
	"variables-legacy": func(h *grHarness) {
		variable := func(selected string) j {
			return j{"name": "env", "display_name": "Environment", "definition": j{"multi_select": j{
				"source": j{"constant_list": []any{"production", "staging"}}, "selected_values": []any{selected},
				"values_order_direction": "asc"}}}
		}
		cfg := dashboardWith(dashboardConfig(), j{"variables": []any{variable("production")}})
		s := h.Apply("create with a legacy variable", h.null(), cfg)
		s = h.RefreshPlan("refresh and plan", s)
		imported := h.Import("import it", idOf(s))
		cfg = dashboardWith(cfg, j{"variables": []any{variable("staging")}})
		s = h.Apply("update the legacy variable", s, cfg)
		s = h.RefreshPlan("refresh and plan after the update", s)
		cfg = dashboardWith(cfg, j{"variables": nil})
		s = h.Apply("remove the legacy variables", s, cfg)
		h.Destroy("destroy", s)
		_ = imported
	},
	"variables-legacy-constant-value": func(h *grHarness) {
		cfg := dashboardWith(dashboardConfig(), j{"variables": []any{j{"name": "env", "display_name": "Environment",
			"definition": j{"constant_value": "production"}}}})
		h.Apply("create with the deprecated constant_value", h.null(), cfg)
	},
	"widget-width": func(h *grHarness) {
		widget := func(width float64) j {
			w := markdownWidget("note", "hello")
			w["width"] = width
			return w
		}
		cfg := dashboardWith(dashboardConfig(), j{"layout": dashboardLayout(widget(6))})
		s := h.Apply("create with the deprecated widget width", h.null(), cfg)
		s = h.RefreshPlan("refresh and plan", s)
		cfg = dashboardWith(cfg, j{"layout": dashboardLayout(widget(8))})
		s = h.Apply("change the width", s, cfg)
		s = h.RefreshPlan("refresh and plan after the change", s)
		cfg = dashboardWith(cfg, j{"layout": dashboardLayout(markdownWidget("note", "hello"))})
		s = h.Apply("remove the width from the config", s, cfg)
		s = h.RefreshPlan("refresh and plan after removing it", s)
		h.Destroy("destroy", s)
	},

	// --- the resource is gone ---
	"read-not-found": func(h *grHarness) {
		s := h.Apply("create", h.null(), dashboardConfig())
		h.dashboards().forget(idOf(s))
		s = h.Refresh("refresh a dashboard deleted outside Terraform", s)
		h.Apply("create it again", s, dashboardConfig())
	},
	"update-not-found": func(h *grHarness) {
		cfg := dashboardConfig()
		s := h.Apply("create", h.null(), cfg)
		h.dashboards().forget(idOf(s))
		s = h.Apply("update a dashboard deleted outside Terraform", s, dashboardWith(cfg, j{"name": "changed"}))
		h.Refresh("refresh after the failed update", s)
	},
	"delete-not-found": func(h *grHarness) {
		s := h.Apply("create", h.null(), dashboardConfig())
		h.dashboards().forget(idOf(s))
		h.Destroy("destroy a dashboard that is already gone", s)
	},
	"route-missing": func(h *grHarness) {
		s := h.Apply("create", h.null(), dashboardConfig())
		h.dashboards().failAlways("Get", 404, "no route matched")
		h.Refresh("refresh when the route is missing", s)
		h.dashboards().clearFaults()
		h.Destroy("destroy", s)
	},

	// --- API errors ---
	"create-api-error": func(h *grHarness) {
		h.dashboards().failNext("Create", 500, "dashboards service unavailable")
		s := h.Apply("create API unavailable", h.null(), dashboardConfig())
		h.Apply("retry create", s, dashboardConfig())
	},
	"create-rejected": func(h *grHarness) {
		h.dashboards().failNext("Create", 400, "name is invalid")
		h.Apply("create rejected with a 400", h.null(), dashboardConfig())
	},
	"read-api-error": func(h *grHarness) {
		s := h.Apply("create", h.null(), dashboardConfig())
		h.dashboards().failNext("Get", 500, "dashboards service unavailable")
		s = h.RefreshPlan("refresh and plan after the API failure", s)
		h.Destroy("destroy", s)
	},
	"update-api-error": func(h *grHarness) {
		cfg := dashboardConfig()
		s := h.Apply("create", h.null(), cfg)
		cfg = dashboardWith(cfg, j{"name": "changed"})
		h.dashboards().failNext("Replace", 500, "dashboards service unavailable")
		s = h.Apply("update API unavailable", s, cfg)
		s = h.RefreshPlan("refresh and plan after the failed update", s)
		s = h.Apply("retry update", s, cfg)
		h.Destroy("destroy", s)
	},
	"delete-api-error": func(h *grHarness) {
		s := h.Apply("create", h.null(), dashboardConfig())
		h.dashboards().failNext("Delete", 500, "dashboards service unavailable")
		h.Destroy("delete API unavailable", s)
		s = h.RefreshPlan("refresh and plan after the failed delete", s)
		h.Destroy("retry delete", s)
	},

	// --- transport errors ---
	"transport-create": func(h *grHarness) {
		h.dashboards().failTransport(errors.New("dashboards transport unavailable"))
		s := h.Apply("create transport failure", h.null(), dashboardConfig())
		h.dashboards().failTransport(nil)
		h.Apply("retry create", s, dashboardConfig())
	},
	"transport-read": func(h *grHarness) {
		s := h.Apply("create", h.null(), dashboardConfig())
		h.dashboards().failTransport(errors.New("dashboards transport unavailable"))
		s = h.Refresh("read transport failure", s)
		h.dashboards().failTransport(nil)
		s = h.RefreshPlan("refresh and plan after recovery", s)
		h.Destroy("destroy", s)
	},
	"transport-update": func(h *grHarness) {
		cfg := dashboardConfig()
		s := h.Apply("create", h.null(), cfg)
		cfg = dashboardWith(cfg, j{"name": "changed"})
		h.dashboards().failTransport(errors.New("dashboards transport unavailable"))
		s = h.Apply("update transport failure", s, cfg)
		h.dashboards().failTransport(nil)
		s = h.Apply("retry update", s, cfg)
		h.Destroy("destroy", s)
	},
	"transport-delete": func(h *grHarness) {
		s := h.Apply("create", h.null(), dashboardConfig())
		h.dashboards().failTransport(errors.New("dashboards transport unavailable"))
		h.Destroy("delete transport failure", s)
		h.dashboards().failTransport(nil)
		h.Destroy("retry delete", s)
	},

	// --- replies that are 200 but wrong ---
	"create-missing-id": func(h *grHarness) {
		h.dashboards().replyNext("Create", j{})
		h.Apply("create answers {} without dashboardId", h.null(), dashboardConfig())
	},
	"create-empty-id": func(h *grHarness) {
		h.dashboards().replyNext("Create", j{"dashboardId": ""})
		h.Apply("create answers an empty dashboardId", h.null(), dashboardConfig())
	},
	"read-null-dashboard": func(h *grHarness) {
		s := h.Apply("create", h.null(), dashboardConfig())
		h.dashboards().replyNext("Get", j{"dashboard": nil, "permissions": dashboardPermissions()})
		s = h.Refresh("read answers a null dashboard", s)
		h.Destroy("destroy", s)
	},
	"read-empty-reply": func(h *grHarness) {
		s := h.Apply("create", h.null(), dashboardConfig())
		h.dashboards().replyNext("Get", j{"permissions": dashboardPermissions()})
		s = h.Refresh("read answers a reply without dashboard", s)
		h.Destroy("destroy", s)
	},

	// --- partial failure of Create ---
	"create-get-fails": func(h *grHarness) {
		h.dashboards().failNext("Get", 500, "dashboards service unavailable")
		s := h.Apply("create, then the read fails, so cleanup deletes the dashboard", h.null(), dashboardConfig())
		h.ApplyAfterFailedCreate("apply again", dashboardConfig())
		_ = s
	},
	"create-get-fails-cleanup-fails": func(h *grHarness) {
		h.dashboards().failNext("Get", 500, "dashboards service unavailable")
		h.dashboards().failNext("Delete", 500, "dashboards service unavailable")
		h.Apply("create, then the read and the cleanup fail", h.null(), dashboardConfig())
		h.ApplyAfterFailedCreate("apply again", dashboardConfig())
	},
	"create-flatten-fails": func(h *grHarness) {
		// The read answers a dynamic widget with the deprecated top-level query. The provider cannot
		// flatten it, so Create fails after the dashboard exists, and the cleanup deletes it.
		h.dashboards().replyNext("Get", j{
			"dashboard":   mustJSONObject(h.t, dashboardContentJSON(h.t, "content_json_dynamic_legacy_query.json")),
			"permissions": dashboardPermissions(),
		})
		h.Apply("create, then the answer cannot be flattened", h.null(), dashboardConfig())
	},

	// --- unknown values ---
	"unknown-name": func(h *grHarness) {
		h.PlanWithRequests("create with an unknown name", h.null(), dashboardWith(dashboardConfig(), j{"name": unknownJSON{}}))
	},
	"unknown-widget-title": func(h *grHarness) {
		w := markdownWidget("note", "hello")
		w["title"] = unknownJSON{}
		h.PlanWithRequests("create with an unknown widget title", h.null(), dashboardWith(dashboardConfig(), j{"layout": dashboardLayout(w)}))
	},
	"unknown-widgets": func(h *grHarness) {
		cfg := dashboardWith(dashboardConfig(), j{"layout": j{"sections": []any{j{"rows": []any{j{"height": 10.0, "widgets": unknownJSON{}}}}}}})
		h.PlanWithRequests("create with an unknown widget list", h.null(), cfg)
	},
	"unknown-sections": func(h *grHarness) {
		h.PlanWithRequests("create with an unknown section list", h.null(), dashboardWith(dashboardConfig(), j{"layout": j{"sections": unknownJSON{}}}))
	},
	"unknown-folder-id": func(h *grHarness) {
		h.PlanWithRequests("create with an unknown folder id", h.null(), dashboardWith(dashboardConfig(), j{"folder": j{"id": unknownJSON{}}}))
	},

	// --- config validation ---
	"config-validation": func(h *grHarness) {
		h.Validate("no layout and no content_json", j{"name": "x"})
		h.Validate("layout and content_json together", j{"layout": dashboardLayout(markdownWidget("n", "t")), "content_json": "{}"})
		h.Validate("content_json with name", j{"name": "x", "content_json": "{}"})
		h.Validate("content_json with description", j{"description": "x", "content_json": "{}"})
		h.Validate("content_json with filters", j{"content_json": "{}", "filters": []any{j{"source": j{"metrics": j{"metric_name": "m", "operator": j{"type": "equals"}}}}}})
		h.Validate("folder with id and path", dashboardWith(dashboardConfig(), j{"folder": j{"id": "a", "path": "b"}}))
		h.Validate("folder with neither", dashboardWith(dashboardConfig(), j{"folder": j{}}))
		h.Validate("time_frame with both", dashboardWith(dashboardConfig(), j{"time_frame": j{
			"absolute": j{"start": "a", "end": "b"}, "relative": j{"duration": "seconds:1"}}}))
		h.Validate("time_frame with neither", dashboardWith(dashboardConfig(), j{"time_frame": j{}}))
		h.Validate("auto_refresh with an unknown type", dashboardWith(dashboardConfig(), j{"auto_refresh": j{"type": "hourly"}}))
		h.Validate("empty filters list", dashboardWith(dashboardConfig(), j{"filters": []any{}}))
		h.Validate("row height below 1", dashboardWith(dashboardConfig(), j{"layout": j{"sections": []any{j{"rows": []any{j{"height": 0.0}}}}}}))
		h.Validate("widget with no definition and no reference", dashboardWith(dashboardConfig(), j{"layout": dashboardLayout(j{"title": "x"})}))
		h.Validate("widget with definition and reference", dashboardWith(dashboardConfig(), j{"layout": dashboardLayout(j{
			"title":      "x",
			"definition": j{"markdown": j{"markdown_text": "t"}},
			"reference":  j{"dashboard_id": "d", "widget_id": "w"}})}))
		h.Validate("definition with two widget kinds", dashboardWith(dashboardConfig(), j{"layout": dashboardLayout(j{
			"title":      "x",
			"definition": j{"markdown": j{"markdown_text": "t"}, "dynamic": j{}}})}))
		h.Validate("definition with no widget kind", dashboardWith(dashboardConfig(), j{"layout": dashboardLayout(j{"title": "x", "definition": j{}})}))
		h.Validate("widget reference without widget_id", dashboardWith(dashboardConfig(), j{"layout": dashboardLayout(j{
			"reference": j{"dashboard_id": "d"}})}))
		h.Validate("a valid config", dashboardConfig())
	},
}

func dashboardPermissions() j {
	return j{"canRead": true, "canReadAccessPolicy": true, "canUpdate": true, "canUpdateAccessPolicy": true}
}

func contentJSONLifecycle(h *grHarness, file string) {
	cfg := contentJSONConfig(dashboardContentJSON(h.t, file))
	s := h.Apply("create from "+file, h.null(), cfg)
	s = h.RefreshPlan("refresh and plan", s)
	h.Apply("apply the same content again", s, cfg)
	h.Destroy("destroy", s)
}

var dashboardCheckMoreScenarios = map[string]func(*grHarness){
	"check-no-issues": func(h *grHarness) {
		h.PlanWithRequests("plan with no issues", h.null(), dashboardConfig())
		s := h.Apply("create", h.null(), dashboardConfig())
		h.Destroy("destroy", s)
	},
	"check-issues-to-warnings": func(h *grHarness) {
		h.dashboards().checkIssues(
			j{"severity": "SEVERITY_ERROR", "location": "/layout/sections/0/rows/0/widgets/0/title", "message": "synthetic error issue"},
			j{"severity": "SEVERITY_WARNING", "location": "/name", "message": "synthetic warning issue"},
			j{"severity": "SEVERITY_ERROR", "location": "/does/not/exist", "message": "issue at a path that is not in the schema"},
			j{"severity": "SEVERITY_ERROR", "location": "/name", "message": ""},
		)
		cfg := dashboardConfig()
		h.PlanWithRequests("plan with issues", h.null(), cfg)
		s := h.Apply("create with issues", h.null(), cfg)
		h.PlanWithRequests("plan an update with issues", s, dashboardWith(cfg, j{"name": "changed"}))
		h.Destroy("destroy", s)
	},
	"check-api-error": func(h *grHarness) {
		h.dashboards().failNext("Check", 500, "check unavailable")
		h.PlanWithRequests("plan when Check fails", h.null(), dashboardConfig())
		h.dashboards().failTransport(errors.New("dashboards transport unavailable"))
		h.PlanWithRequests("plan when the transport fails", h.null(), dashboardConfig())
		h.dashboards().failTransport(nil)
		h.Apply("create still works", h.null(), dashboardConfig())
	},
	"check-skips-unknown": func(h *grHarness) {
		h.dashboards().checkIssues(j{"severity": "SEVERITY_ERROR", "location": "/name", "message": "issue on a value that is unknown"})
		h.PlanWithRequests("plan with an unknown name", h.null(), dashboardWith(dashboardConfig(), j{"name": unknownJSON{}}))
	},
	"check-no-call-when-unchanged": func(h *grHarness) {
		s := h.Apply("create", h.null(), dashboardConfig())
		h.dashboards().failNext("Check", 500, "must not be called")
		h.PlanWithRequests("plan the same config", s, dashboardConfig())
		h.Destroy("destroy", s)
	},
}

// check-disabled needs the plan-time Check call off, with issues ready.
func init() {
	dashboardScenarios["check-disabled"] = func(h *grHarness) {
		h.dashboards().checkIssues(j{"severity": "SEVERITY_ERROR", "location": "/name", "message": "must not show"})
		h.PlanWithRequests("plan with validation disabled", h.null(), dashboardConfig())
		h.Apply("create", h.null(), dashboardConfig())
	}
}

// Old states are written by older provider versions. The upgrader reads the dashboard from the API,
// so the old state only has to name an id that exists. The old schemas are not recorded here.
func init() {
	for _, version := range []int64{1, 2, 3} {
		dashboardScenarios["upgrade-v"+itoa(int(version))] = func(h *grHarness) {
			cfg := dashboardConfig()
			s := h.Apply("create the dashboard that the old state names", h.null(), cfg)
			id := idOf(s)
			upgraded := h.UpgradeFrom("upgrade a state written by schema version "+itoa(int(version)), version,
				`{"id":"`+id+`","name":"old name","description":"old description"}`)
			upgraded = h.RefreshPlan("refresh and plan the upgraded state", upgraded)
			h.Destroy("destroy", upgraded)
		}
		dashboardScenarios["upgrade-v"+itoa(int(version))+"-missing"] = func(h *grHarness) {
			h.UpgradeFrom("upgrade a state whose dashboard does not exist", version,
				`{"id":"missing-dashboard-001","name":"old name"}`)
		}
	}
}
