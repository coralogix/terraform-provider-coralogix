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
	"context"
	"encoding/json"
	"maps"
	"os"
	"regexp"
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/internal/provider/dashboards"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils/schemadump"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

const dashboardTypeName = "coralogix_dashboard"
const dashboardGoldenDir = "testdata/dashboard_baseline"

// newDashboardHarness drives the dashboard resource against the fake. The plan-time Check call is
// off, because most scenarios are not about it. The check-* scenarios turn it on.
func newDashboardHarness(t *testing.T) *grHarness {
	t.Setenv("CORALOGIX_DASHBOARD_VALIDATION", "false")
	return newProtocolHarness(t, dashboardTypeName, newDashboardFake(t))
}

// newDashboardCheckHarness has the plan-time Check call on.
func newDashboardCheckHarness(t *testing.T) *grHarness {
	h := newDashboardHarness(t)
	t.Setenv("CORALOGIX_DASHBOARD_VALIDATION", "true")
	return h
}

func (h *grHarness) dashboards() *dashboardFake { return h.api.(*dashboardFake) }

// PlanWithRequests is Plan, and it also records the API calls that the plan makes. The plan-time
// Check call is one.
func (h *grHarness) PlanWithRequests(name string, prior tftypes.Value, cfg map[string]any) {
	h.t.Helper()
	h.Plan(name, prior, cfg)
	h.steps[len(h.steps)-1].Requests = h.api.takeRequests()
}

// UpgradeFrom upgrades a raw state that was written by the given schema version.
func (h *grHarness) UpgradeFrom(name string, version int64, rawJSON string) tftypes.Value {
	h.t.Helper()
	step := grStep{Step: name, Config: json.RawMessage(rawJSON)}
	resp, err := h.server.UpgradeResourceState(h.ctx, &tfprotov6.UpgradeResourceStateRequest{
		TypeName: h.typeName, Version: version, RawState: &tfprotov6.RawState{JSON: []byte(rawJSON)},
	})
	h.must(err)
	step.Requests = h.api.takeRequests()
	step.ApplyDiags = diagStrings(resp.Diagnostics)
	if hasError(resp.Diagnostics) {
		h.record(step)
		return h.null()
	}
	state := h.decode(resp.UpgradedState)
	step.State = toJSON(state)
	h.record(step)
	return state
}

var dashboardUUIDPattern = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)

// marshalDashboardGolden masks the ids that the provider makes at random (section, row, widget,
// filter and action ids). A mask is numbered by the first place it shows in the file, so the golden
// is the same on every run. Dashboard ids come from the fake and are already fixed.
func marshalDashboardGolden(t *testing.T, v any) []byte {
	t.Helper()
	masks := map[string]string{}
	return dashboardUUIDPattern.ReplaceAllFunc(marshalGolden(t, v), func(id []byte) []byte {
		mask, ok := masks[string(id)]
		if !ok {
			mask = "<id-" + itoa(len(masks)+1) + ">"
			masks[string(id)] = mask
		}
		return []byte(mask)
	})
}

func itoa(n int) string {
	digits := []byte{}
	for ; n > 0; n /= 10 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
	}
	return string(digits)
}

func markdownWidget(title, text string) j {
	return j{"title": title, "definition": j{"markdown": j{"markdown_text": text}}}
}

func dashboardLayout(widgets ...any) j {
	return j{"sections": []any{j{"rows": []any{j{"height": 10.0, "widgets": widgets}}}}}
}

func dashboardConfig() j {
	return j{
		"name":   "baseline dashboard",
		"layout": dashboardLayout(markdownWidget("note", "hello")),
	}
}

// dashboardWith returns a copy of the config with the values set. A nil value removes the attribute.
func dashboardWith(cfg j, values j) j {
	out := maps.Clone(cfg)
	for k, v := range values {
		if v == nil {
			delete(out, k)
			continue
		}
		out[k] = v
	}
	return out
}

// Record before switching implementations:
// UPDATE_GOLDEN=1 go test ./internal/provider -run 'TestDashboardBaseline|TestDataSourceBaseline.*/dashboard'
func TestDashboardBaselineSchema(t *testing.T) {
	var resp resource.SchemaResponse
	dashboards.NewDashboardResource().Schema(context.Background(), resource.SchemaRequest{}, &resp)
	checkGoldenAt(t, dashboardGoldenDir, "schema_current.txt", []byte(schemadump.Text(resp.Schema)))
}

func TestDashboardBaselineScenarios(t *testing.T) {
	for name, run := range dashboardScenarios {
		t.Run(name, func(t *testing.T) {
			h := newDashboardHarness(t)
			run(h)
			checkGoldenAt(t, dashboardGoldenDir, "scenario_"+name+".json", marshalDashboardGolden(t, h.steps))
		})
	}
	for name, run := range dashboardCheckScenarios {
		t.Run(name, func(t *testing.T) {
			h := newDashboardCheckHarness(t)
			run(h)
			checkGoldenAt(t, dashboardGoldenDir, "scenario_"+name+".json", marshalDashboardGolden(t, h.steps))
		})
	}
}

func init() {
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		_ = os.MkdirAll(dashboardGoldenDir, 0o755)
	}
}

var dashboardScenarios = map[string]func(*grHarness){
	"minimal": func(h *grHarness) {
		s := h.Apply("create", h.null(), dashboardConfig())
		s = h.RefreshPlan("refresh and plan", s)
		h.Destroy("destroy", s)
	},
	"import": func(h *grHarness) {
		s := h.Apply("create", h.null(), dashboardConfig())
		imported := h.Import("import by id", idOf(s))
		h.Destroy("destroy", imported)
	},
	"import-empty-id": func(h *grHarness) {
		h.Import("import an empty id", "")
	},
	"import-missing": func(h *grHarness) {
		h.Import("import an id that does not exist", "missing-dashboard-1")
	},
	"duplicate-names": func(h *grHarness) {
		first := h.Apply("create first", h.null(), dashboardConfig())
		second := h.Apply("create second with the same name", h.null(), dashboardConfig())
		h.Destroy("destroy second", second)
		h.Destroy("destroy first", first)
	},
	"full-lifecycle": func(h *grHarness) {
		cfg := dashboardConfig()
		s := h.Apply("create", h.null(), cfg)
		cfg = dashboardWith(cfg, j{"name": "renamed", "description": "described"})
		s = h.Apply("update name and description", s, cfg)
		s = h.RefreshPlan("refresh and plan", s)
		cfg = dashboardWith(cfg, j{"description": nil})
		s = h.Apply("remove description", s, cfg)
		s = h.RefreshPlan("refresh and plan after removing description", s)
		h.Destroy("destroy", s)
	},
}

var dashboardCheckScenarios = map[string]func(*grHarness){}
