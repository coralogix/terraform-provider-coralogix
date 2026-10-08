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
	"slices"
	"testing"
)

// tgt builds a target as the API stores it. An empty preset leaves presetId out.
func tgt(connector, preset, detail string) any {
	t := j{"connectorId": connector}
	if preset != "" {
		t["presetId"] = preset
	}
	if detail != "" {
		t["customDetails"] = j{"k": detail}
	}
	return t
}

// cfgTgt builds the same target as a Terraform config.
func cfgTgt(connector, preset, detail string) any {
	t := j{"connector_id": connector}
	if preset != "" {
		t["preset_id"] = preset
	}
	if detail != "" {
		t["custom_details"] = j{"k": detail}
	}
	return t
}

// ruleTargetKeys lists the targets of rule i in the state as connector/preset{k=detail}.
func ruleTargetKeys(t *testing.T, state any, rule int) []string {
	t.Helper()
	rules := state.(map[string]any)["rules"].([]any)
	var keys []string
	for _, tg := range rules[rule].(map[string]any)["targets"].([]any) {
		m := tg.(map[string]any)
		key := m["connector_id"].(string) + "/"
		if p, ok := m["preset_id"].(string); ok {
			key += p
		}
		if d, ok := m["custom_details"].(map[string]any); ok {
			if v, ok := d["k"]; ok {
				key += "{k=" + v.(string) + "}"
			}
		}
		keys = append(keys, key)
	}
	return keys
}

func keyList(keys ...string) []string { return keys }

// The API does not keep the order of the targets of a rule. These cases say when the resource
// returns the order of the plan, and when it returns the order of the API.
func TestGlobalRouterTargetOrder(t *testing.T) {
	tests := []struct {
		name   string
		prior  []any // targets in the config
		api    []any // targets that the API returns after the change outside Terraform
		expect []string
	}{
		{
			name:   "api rotated the targets",
			prior:  []any{cfgTgt("http", "http", ""), cfgTgt("slack", "slack", ""), cfgTgt("pd", "pd", "")},
			api:    []any{tgt("pd", "pd", ""), tgt("http", "http", ""), tgt("slack", "slack", "")},
			expect: keyList("http/http", "slack/slack", "pd/pd"),
		},
		{
			name:   "target added outside terraform keeps the api order",
			prior:  []any{cfgTgt("http", "http", ""), cfgTgt("pd", "pd", "")},
			api:    []any{tgt("new", "", ""), tgt("pd", "pd", ""), tgt("http", "http", "")},
			expect: keyList("new/", "pd/pd", "http/http"),
		},
		{
			name:   "target removed outside terraform keeps the api order",
			prior:  []any{cfgTgt("http", "http", ""), cfgTgt("pd", "pd", "")},
			api:    []any{tgt("pd", "pd", "")},
			expect: keyList("pd/pd"),
		},
		{
			name:   "same connector with different presets match on preset",
			prior:  []any{cfgTgt("http", "a", ""), cfgTgt("http", "b", ""), cfgTgt("http", "", "")},
			api:    []any{tgt("http", "b", ""), tgt("http", "", ""), tgt("http", "a", "")},
			expect: keyList("http/a", "http/b", "http/"),
		},
		{
			name:   "duplicate targets are each matched once",
			prior:  []any{cfgTgt("pd", "", ""), cfgTgt("pd", "", ""), cfgTgt("http", "", "")},
			api:    []any{tgt("pd", "", ""), tgt("http", "", ""), tgt("pd", "", "")},
			expect: keyList("pd/", "pd/", "http/"),
		},
		{
			// Probe on eu2: the API accepts this pair and can return it in any order.
			name:   "same connector and preset with different custom details",
			prior:  []any{cfgTgt("http", "", "1"), cfgTgt("http", "", "2")},
			api:    []any{tgt("http", "", "2"), tgt("http", "", "1")},
			expect: keyList("http/{k=1}", "http/{k=2}"),
		},
		{
			name:   "custom details changed outside terraform keeps the api order",
			prior:  []any{cfgTgt("http", "", "old"), cfgTgt("pd", "", "")},
			api:    []any{tgt("pd", "", ""), tgt("http", "", "new")},
			expect: keyList("pd/", "http/{k=new}"),
		},
		{
			// Probe on eu2: 1 of 10 creates with mixed connector types came back rotated.
			name:   "api rotated mixed connector types",
			prior:  []any{cfgTgt("http-a", "", ""), cfgTgt("pd", "", ""), cfgTgt("http-b", "", "")},
			api:    []any{tgt("http-b", "", ""), tgt("http-a", "", ""), tgt("pd", "", "")},
			expect: keyList("http-a/", "pd/", "http-b/"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newGRHarness(t)
			cfg := router("order", j{"rules": l{j{"name": "r", "condition": "true", "entity_type": "alerts", "targets": tt.prior}}})
			state := h.Apply("create", h.null(), cfg)
			h.routers().mutate(idOf(state), func(f *grFake, stored map[string]any) { f.setRuleTargets(stored, 0, tt.api) })

			got := ruleTargetKeys(t, toJSON(h.Refresh("refresh", state)), 0)
			if !slices.Equal(got, tt.expect) {
				t.Fatalf("got %v, want %v", got, tt.expect)
			}
		})
	}
}

// Rules are paired by index. A rule that has no prior rule keeps the API order.
func TestGlobalRouterTargetOrderPairsRulesByIndex(t *testing.T) {
	h := newGRHarness(t)
	cfg := router("pairs", j{"rules": l{j{"name": "r0", "condition": "true", "entity_type": "alerts", "targets": l{cfgTgt("a", "", ""), cfgTgt("b", "", "")}}}})
	state := h.Apply("create", h.null(), cfg)
	h.routers().mutate(idOf(state), func(f *grFake, stored map[string]any) {
		stored["rules"] = []any{
			map[string]any{"name": "r0", "condition": "true", "entityType": "ALERTS", "customDetails": map[string]any{}, "targets": []any{}},
			map[string]any{"name": "r1", "condition": "true", "entityType": "ALERTS", "customDetails": map[string]any{}, "targets": []any{}},
		}
		f.setRuleTargets(stored, 0, []any{tgt("b", "", ""), tgt("a", "", "")})
		f.setRuleTargets(stored, 1, []any{tgt("d", "", ""), tgt("c", "", "")})
	})

	refreshed := toJSON(h.Refresh("refresh", state))
	if got := ruleTargetKeys(t, refreshed, 0); !slices.Equal(got, []string{"a/", "b/"}) {
		t.Fatalf("rule 0: got %v, want [a/ b/]", got)
	}
	if got := ruleTargetKeys(t, refreshed, 1); !slices.Equal(got, []string{"d/", "c/"}) {
		t.Fatalf("rule 1: got %v, want [d/ c/]", got)
	}
}

// Import starts with only an id, so Read gets null rules from the state. The API order stays.
func TestGlobalRouterTargetOrderAfterImport(t *testing.T) {
	h := newGRHarness(t)
	id := h.routers().seed(j{
		"name": "imported", "routingLabels": j{"environment": "imported"},
		"rules": l{j{"name": "r", "condition": "true", "entityType": "ALERTS", "targets": l{tgt("pd", "", ""), tgt("http", "", "")}}},
	})
	state := toJSON(h.Import("import", id))
	if got := ruleTargetKeys(t, state, 0); !slices.Equal(got, []string{"pd/", "http/"}) {
		t.Fatalf("got %v, want [pd/ http/]", got)
	}
}
