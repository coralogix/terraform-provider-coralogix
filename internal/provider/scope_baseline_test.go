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
	"maps"
	"os"
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/internal/provider/aaa"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils/schemadump"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

const scopeTypeName = "coralogix_scope"
const scopeGoldenDir = "testdata/scope_baseline"

func newScopeHarness(t *testing.T) *grHarness {
	return newProtocolHarness(t, scopeTypeName, newScopeFake(t))
}

func (h *grHarness) scopes() *scopeFake { return h.api.(*scopeFake) }

func scopeConfig() j {
	return j{"display_name": "example", "default_expression": "<v1>true", "filters": []any{j{"entity_type": "logs", "expression": "<v1>true"}}}
}

// Record before switching implementations:
// UPDATE_GOLDEN=1 go test ./internal/provider -run 'TestScopeBaseline|TestDataSourceBaseline.*/scope'
func TestScopeBaselineSchema(t *testing.T) {
	var resp resource.SchemaResponse
	aaa.NewScopeResource().Schema(context.Background(), resource.SchemaRequest{}, &resp)
	checkGoldenAt(t, scopeGoldenDir, "schema_current.txt", []byte(schemadump.Text(resp.Schema)))
}

func TestScopeBaselineScenarios(t *testing.T) {
	for name, run := range scopeScenarios {
		t.Run(name, func(t *testing.T) {
			h := newScopeHarness(t)
			run(h)
			checkGoldenAt(t, scopeGoldenDir, "scenario_"+name+".json", marshalGolden(t, h.steps))
		})
	}
}

func TestScopeBaselinePartialSuccess(t *testing.T) {
	for _, tc := range []struct {
		name     string
		response map[string]any
	}{
		{"null-scope", j{"scope": nil}},
		{"missing-id", j{"scope": j{}}},
		{"empty-id", j{"scope": j{"id": ""}}},
	} {
		t.Run("create-"+tc.name, func(t *testing.T) {
			h := newScopeHarness(t)
			h.scopes().replyToWrite(tc.response)
			h.Apply("create returns "+tc.name, h.null(), scopeConfig())
			checkGoldenAt(t, scopeGoldenDir, "scenario_create-"+tc.name+".json", marshalGolden(t, h.steps))
		})
		t.Run("update-"+tc.name, func(t *testing.T) {
			h := newScopeHarness(t)
			cfg := scopeConfig()
			s := h.Apply("create", h.null(), cfg)
			h.scopes().replyToWrite(tc.response)
			cfg = maps.Clone(cfg)
			cfg["display_name"] = "changed"
			s = h.Apply("update returns "+tc.name, s, cfg)
			h.scopes().replyToWrite(nil)
			s = h.RefreshPlan("refresh and plan after partial success", s)
			h.Destroy("destroy", s)
			checkGoldenAt(t, scopeGoldenDir, "scenario_update-"+tc.name+".json", marshalGolden(t, h.steps))
		})
	}
}

func TestScopeBaselineReadResponses(t *testing.T) {
	for _, tc := range []struct {
		name     string
		response map[string]any
	}{
		{"empty-scopes", j{"scopes": []any{}}},
		{"null-scopes", j{"scopes": nil}},
		{"missing-id", j{"scopes": []any{j{}}}},
		{"empty-id", j{"scopes": []any{j{"id": ""}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newScopeHarness(t)
			s := h.Apply("create", h.null(), scopeConfig())
			h.scopes().replyToRead(tc.response)
			h.Refresh("resource read returns "+tc.name, s)
			h.ReadDataSource("data source read returns "+tc.name, byID(idOf(s)))
			checkGoldenAt(t, scopeGoldenDir, "scenario_read-"+tc.name+".json", marshalGolden(t, h.steps))
		})
	}
}

var scopeScenarios = map[string]func(*grHarness){
	"clear-filters": func(h *grHarness) {
		cfg := scopeConfig()
		s := h.Apply("create nonempty filters", h.null(), cfg)
		cfg = maps.Clone(cfg)
		cfg["filters"] = []any{}
		s = h.Apply("clear filters", s, cfg)
		s = h.RefreshPlan("refresh and plan cleared filters", s)
		h.ReadDataSource("read cleared filters", byID(idOf(s)))
		h.Destroy("destroy", s)
	},
	"unspecified-entity-type": func(h *grHarness) {
		cfg := scopeConfig()
		cfg["filters"] = []any{j{"entity_type": "unspecified", "expression": "<v1>true"}}
		h.Apply("apply allowed unspecified entity type", h.null(), cfg)
	},
	"unversioned-filter-expression": func(h *grHarness) {
		cfg := scopeConfig()
		cfg["filters"] = []any{j{"entity_type": "logs", "expression": "true"}}
		h.Apply("apply filter expression without version", h.null(), cfg)
	},
	"empty-filter-expression": func(h *grHarness) {
		cfg := scopeConfig()
		cfg["filters"] = []any{j{"entity_type": "logs", "expression": ""}}
		h.Apply("apply empty filter expression", h.null(), cfg)
	},
	"loose-default-expression-validator": func(h *grHarness) {
		cfg := scopeConfig()
		cfg["default_expression"] = "<v1>true-whatever"
		h.Apply("apply expression accepted by the released validator", h.null(), cfg)
	},
	"create-api-error": func(h *grHarness) {
		h.scopes().fail(500, "Internal Server Error: scope service unavailable")
		h.Apply("create API unavailable", h.null(), scopeConfig())
	},
	"read-api-error": func(h *grHarness) {
		s := h.Apply("create", h.null(), scopeConfig())
		h.scopes().fail(500, "Internal Server Error: scope service unavailable")
		s = h.RefreshPlan("refresh and plan after API failure", s)
		h.scopes().fail(0, "")
		h.Destroy("destroy", s)
	},
	"delete-api-error": func(h *grHarness) {
		s := h.Apply("create", h.null(), scopeConfig())
		h.scopes().fail(500, "Internal Server Error: scope service unavailable")
		h.Destroy("delete API unavailable", s)
		h.scopes().fail(0, "")
		s = h.RefreshPlan("refresh and plan after failed delete", s)
		h.Destroy("retry delete", s)
	},
	"duplicate-display-names": func(h *grHarness) {
		first := h.Apply("create first scope", h.null(), scopeConfig())
		second := h.Apply("create scope with the same display name", h.null(), scopeConfig())
		h.Destroy("destroy second scope", second)
		h.Destroy("destroy first scope", first)
	},
	"empty-filters": func(h *grHarness) {
		cfg := scopeConfig()
		cfg["filters"] = []any{}
		s := h.Apply("create empty filters", h.null(), cfg)
		s = h.RefreshPlan("refresh and plan empty filters", s)
		s = h.Import("import empty filters", idOf(s))
		s = h.Apply("plan imported empty filters", s, cfg)
		h.ReadDataSource("read empty filters", byID(idOf(s)))
		h.Destroy("destroy", s)
	},
	"import-empty-description": func(h *grHarness) {
		fake := h.scopes()
		id := fake.seed()
		fake.mu.Lock()
		fake.scopes[id]["description"] = ""
		fake.mu.Unlock()
		s := h.Import("import explicit empty description", id)
		cfg := scopeConfig()
		cfg["display_name"], cfg["description"] = "seeded", ""
		h.Apply("plan matching empty description", s, cfg)
		h.ReadDataSource("read explicit empty description", byID(id))
	},
	"import-empty-id": func(h *grHarness) { h.Import("import empty id", "") },
	"empty-description": func(h *grHarness) {
		cfg := scopeConfig()
		cfg["description"] = ""
		s := h.Apply("create with empty description", h.null(), cfg)
		s = h.RefreshPlan("refresh and plan empty description", s)
		h.Destroy("destroy", s)
	},
	"delete-route-missing": func(h *grHarness) {
		s := h.Apply("create", h.null(), scopeConfig())
		h.scopes().fail(404, "Not Found: Not Found")
		h.Destroy("delete route unavailable", s)
	},
	"upgrade-version-zero": func(h *grHarness) {
		h.scopes().seed()
		raw, err := os.ReadFile(scopeGoldenDir + "/state_v0.json")
		if err != nil {
			h.t.Fatal(err)
		}
		s := h.UpgradeV0("load handwritten version-zero state", string(raw))
		cfg := scopeConfig()
		cfg["display_name"] = "seeded"
		s = h.Apply("plan matching config after upgrade", s, cfg)
		cfg = maps.Clone(cfg)
		cfg["description"] = "after upgrade"
		s = h.Apply("update imported state", s, cfg)
		s = h.RefreshPlan("refresh and plan upgraded state", s)
		h.Destroy("destroy", s)
	},
	"full-lifecycle": func(h *grHarness) {
		cfg := scopeConfig()
		cfg["description"] = "original"
		s := h.Apply("create", h.null(), cfg)
		s = h.RefreshPlan("refresh and plan", s)
		cfg = maps.Clone(cfg)
		cfg["display_name"], cfg["description"], cfg["default_expression"] = "renamed", "changed", "<v1>false"
		cfg["filters"] = []any{j{"entity_type": "spans", "expression": "<v1>false"}, j{"entity_type": "logs", "expression": "<v1>true"}}
		s = h.Apply("change all writable fields", s, cfg)
		s = h.RefreshPlan("refresh and plan updated config", s)
		cfg = maps.Clone(cfg)
		delete(cfg, "description")
		s = h.Apply("remove description", s, cfg)
		s = h.RefreshPlan("refresh and plan without description", s)
		h.Destroy("destroy", s)
	},
	"minimal": func(h *grHarness) {
		s := h.Apply("create without description", h.null(), scopeConfig())
		s = h.RefreshPlan("refresh and plan", s)
		h.Destroy("destroy", s)
	},
	"import": func(h *grHarness) {
		s := h.Import("import", h.scopes().seed())
		cfg := scopeConfig()
		cfg["display_name"] = "seeded"
		h.Apply("plan matching config", s, cfg)
	},
	"read-not-found": func(h *grHarness) {
		s := h.Apply("create", h.null(), scopeConfig())
		h.scopes().forget(idOf(s))
		h.Refresh("read deleted scope", s)
	},
	"update-not-found": func(h *grHarness) {
		s := h.Apply("create", h.null(), scopeConfig())
		h.scopes().forget(idOf(s))
		cfg := scopeConfig()
		cfg["display_name"] = "changed"
		h.Apply("update deleted scope", s, cfg)
	},
	"delete-not-found": func(h *grHarness) {
		s := h.Apply("create", h.null(), scopeConfig())
		h.scopes().forget(idOf(s))
		h.Destroy("delete missing scope", s)
	},
	"route-missing": func(h *grHarness) {
		s := h.Apply("create", h.null(), scopeConfig())
		h.scopes().fail(404, "Not Found: Not Found")
		h.Refresh("read route unavailable", s)
	},
	"config-validation": func(h *grHarness) {
		h.Validate("missing required fields", j{})
		cfg := scopeConfig()
		cfg["default_expression"] = "true"
		h.Validate("expression without version", cfg)
		cfg = scopeConfig()
		cfg["filters"] = []any{j{"entity_type": "metrics", "expression": "<v1>true"}}
		h.Validate("unsupported entity type", cfg)
	},
}

var scopeDataSourceScenarios = map[string]func(*grHarness){
	"empty-id":   func(h *grHarness) { h.ReadDataSource("read empty id", byID("")) },
	"read-by-id": func(h *grHarness) { h.ReadDataSource("read", byID(h.scopes().seed())) },
	"not-found":  func(h *grHarness) { h.ReadDataSource("read missing scope", byID(missingID)) },
	"no-id":      func(h *grHarness) { h.ReadDataSource("read without id", j{}) },
}
