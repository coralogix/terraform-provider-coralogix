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

import "fmt"

// The field matrix: every attribute is set, left out, emptied, changed, removed, and changed outside
// Terraform. Each group is one golden file, so a difference points at one field.

// base is a router with a name and one label, and nothing else.
func base(name string, extra ...j) j {
	out := j{"name": name, "routing_labels": j{"environment": name}}
	for _, e := range extra {
		for k, v := range e {
			out[k] = v
		}
	}
	return out
}

func rule(name string, extra ...j) j {
	out := j{"name": name, "condition": "true", "entity_type": "alerts", "targets": l{j{"connector_id": "c1"}}}
	for _, e := range extra {
		for k, v := range e {
			out[k] = v
		}
	}
	return out
}

func init() {
	for name, run := range fieldScenarios {
		grScenarios["fields-"+name] = run
	}
}

var fieldScenarios = map[string]func(h *grHarness){
	"name": func(h *grHarness) {
		s := h.Apply("create", h.null(), base("n1"))
		s = h.Apply("rename", s, base("n1", j{"name": "n2"}))
		h.Refresh("refresh", s)
	},

	"description": func(h *grHarness) {
		s := h.Apply("create with text", h.null(), base("d1", j{"description": "text"}))
		s = h.Apply("change", s, base("d1", j{"description": "other"}))
		s = h.Apply("set to empty string", s, base("d1", j{"description": ""}))
		s = h.Apply("set again", s, base("d1", j{"description": "again"}))
		s = h.Apply("remove", s, base("d1"))
		h.Refresh("refresh", s)
		h.Apply("create with empty string", h.null(), base("d2", j{"description": ""}))
		h.Apply("create without it", h.null(), base("d3"))
	},

	"disabled": func(h *grHarness) {
		s := h.Apply("create without it", h.null(), base("b1"))
		s = h.Apply("true", s, base("b1", j{"disabled": true}))
		s = h.Apply("false", s, base("b1", j{"disabled": false}))
		s = h.Apply("true again", s, base("b1", j{"disabled": true}))
		s = h.Apply("remove", s, base("b1"))
		h.Refresh("refresh", s)
	},

	"entity-labels": func(h *grHarness) {
		s := h.Apply("create without it", h.null(), base("e1"))
		s = h.Apply("set", s, base("e1", j{"entity_labels": j{"a": "b", "c": "d"}}))
		s = h.Apply("change", s, base("e1", j{"entity_labels": j{"a": "x"}}))
		s = h.Apply("empty map", s, base("e1", j{"entity_labels": j{}}))
		s = h.Apply("set again", s, base("e1", j{"entity_labels": j{"k": "v"}}))
		s = h.Apply("remove", s, base("e1"))
		h.Refresh("refresh", s)
		h.Apply("create with empty map", h.null(), base("e2", j{"entity_labels": j{}}))
	},

	"routing-labels": func(h *grHarness) {
		s := h.Apply("environment only", h.null(), j{"name": "l1", "routing_labels": j{"environment": "l1"}})
		s = h.Apply("add service", s, j{"name": "l1", "routing_labels": j{"environment": "l1", "service": "svc"}})
		s = h.Apply("all three", s, j{"name": "l1", "routing_labels": j{"environment": "l1", "service": "svc", "team": "tm"}})
		s = h.Apply("remove service", s, j{"name": "l1", "routing_labels": j{"environment": "l1", "team": "tm"}})
		s = h.Apply("change environment", s, j{"name": "l1", "routing_labels": j{"environment": "l1b", "team": "tm"}})
		h.Refresh("refresh", s)
		h.Apply("service only", h.null(), j{"name": "l2", "routing_labels": j{"service": "svc2"}})
		h.Apply("team only", h.null(), j{"name": "l3", "routing_labels": j{"team": "tm3"}})
		h.Apply("empty block", h.null(), j{"name": "l4", "routing_labels": j{}})
		h.Apply("no block", h.null(), j{"name": "l5"})
		s = h.Apply("create then remove block", h.null(), j{"name": "l6", "routing_labels": j{"environment": "l6"}})
		h.Apply("remove block", s, j{"name": "l6"})
	},

	"rule-entity-type": func(h *grHarness) {
		for _, et := range []string{"alerts", "cases", "test_notifications"} {
			s := h.Apply("create "+et, h.null(), base("r-"+et, j{"rules": l{rule("r", j{"entity_type": et})}}))
			h.Refresh("refresh "+et, s)
		}
		s := h.Apply("create alerts", h.null(), base("r-chg", j{"rules": l{rule("r")}}))
		h.Apply("change to cases", s, base("r-chg", j{"rules": l{rule("r", j{"entity_type": "cases"})}}))
		h.Apply("unspecified", h.null(), base("r-un", j{"rules": l{rule("r", j{"entity_type": "unspecified"})}}))
		omitted := rule("r")
		delete(omitted, "entity_type")
		h.Apply("omitted", h.null(), base("r-om", j{"rules": l{omitted}}))
	},

	"rule-attributes": func(h *grHarness) {
		s := h.Apply("create", h.null(), base("ra", j{"rules": l{rule("one")}}))
		s = h.Apply("change condition", s, base("ra", j{"rules": l{rule("one", j{"condition": "alert.priority == 'P1'"})}}))
		s = h.Apply("change name", s, base("ra", j{"rules": l{rule("two", j{"condition": "alert.priority == 'P1'"})}}))
		s = h.Apply("custom_details set", s, base("ra", j{"rules": l{rule("two", j{"custom_details": j{"a": "b"}})}}))
		s = h.Apply("custom_details change", s, base("ra", j{"rules": l{rule("two", j{"custom_details": j{"a": "c", "d": "e"}})}}))
		s = h.Apply("custom_details empty", s, base("ra", j{"rules": l{rule("two", j{"custom_details": j{}})}}))
		s = h.Apply("custom_details set again", s, base("ra", j{"rules": l{rule("two", j{"custom_details": j{"k": "v"}})}}))
		s = h.Apply("custom_details remove", s, base("ra", j{"rules": l{rule("two")}}))
		h.Refresh("refresh", s)
		h.Apply("rule without targets", h.null(), base("rb", j{"rules": l{j{"name": "r", "condition": "true", "entity_type": "alerts"}}}))
		h.Apply("rule with empty targets", h.null(), base("rc", j{"rules": l{rule("r", j{"targets": l{}})}}))
	},

	"rule-targets": func(h *grHarness) {
		s := h.Apply("create", h.null(), base("t1", j{"rules": l{rule("r")}}))
		s = h.Apply("change connector", s, base("t1", j{"rules": l{rule("r", j{"targets": l{j{"connector_id": "c2"}}})}}))
		s = h.Apply("preset set", s, base("t1", j{"rules": l{rule("r", j{"targets": l{j{"connector_id": "c2", "preset_id": "p1"}}})}}))
		s = h.Apply("preset change", s, base("t1", j{"rules": l{rule("r", j{"targets": l{j{"connector_id": "c2", "preset_id": "p2"}}})}}))
		s = h.Apply("preset remove", s, base("t1", j{"rules": l{rule("r", j{"targets": l{j{"connector_id": "c2"}}})}}))
		s = h.Apply("custom_details set", s, base("t1", j{"rules": l{rule("r", j{"targets": l{j{"connector_id": "c2", "custom_details": j{"a": "b"}}}})}}))
		s = h.Apply("custom_details change", s, base("t1", j{"rules": l{rule("r", j{"targets": l{j{"connector_id": "c2", "custom_details": j{"a": "c"}}}})}}))
		s = h.Apply("custom_details empty", s, base("t1", j{"rules": l{rule("r", j{"targets": l{j{"connector_id": "c2", "custom_details": j{}}}})}}))
		s = h.Apply("custom_details remove", s, base("t1", j{"rules": l{rule("r", j{"targets": l{j{"connector_id": "c2"}}})}}))
		s = h.Apply("three targets", s, base("t1", j{"rules": l{rule("r", j{"targets": l{j{"connector_id": "c1"}, j{"connector_id": "c2"}, j{"connector_id": "c3"}}})}}))
		s = h.Apply("reorder in config", s, base("t1", j{"rules": l{rule("r", j{"targets": l{j{"connector_id": "c3"}, j{"connector_id": "c1"}, j{"connector_id": "c2"}}})}}))
		s = h.Apply("remove one", s, base("t1", j{"rules": l{rule("r", j{"targets": l{j{"connector_id": "c3"}, j{"connector_id": "c2"}}})}}))
		h.Refresh("refresh", s)
	},

	"rules-list": func(h *grHarness) {
		s := h.Apply("two rules", h.null(), base("rl", j{"rules": l{rule("a"), rule("b", j{"entity_type": "cases"})}}))
		s = h.Apply("reorder rules", s, base("rl", j{"rules": l{rule("b", j{"entity_type": "cases"}), rule("a")}}))
		s = h.Apply("add third", s, base("rl", j{"rules": l{rule("b", j{"entity_type": "cases"}), rule("a"), rule("c")}}))
		s = h.Apply("remove first", s, base("rl", j{"rules": l{rule("a"), rule("c")}}))
		s = h.Apply("empty list", s, base("rl", j{"rules": l{}}))
		s = h.Apply("one rule again", s, base("rl", j{"rules": l{rule("z")}}))
		h.Refresh("refresh", s)
	},

	"fallback": func(h *grHarness) {
		s := h.Apply("create", h.null(), base("f1", j{"fallback": l{j{"connector_id": "c1"}}}))
		s = h.Apply("preset set", s, base("f1", j{"fallback": l{j{"connector_id": "c1", "preset_id": "p"}}}))
		s = h.Apply("custom_details set", s, base("f1", j{"fallback": l{j{"connector_id": "c1", "custom_details": j{"a": "b"}}}}))
		s = h.Apply("two targets", s, base("f1", j{"fallback": l{j{"connector_id": "c1"}, j{"connector_id": "c2"}}}))
		s = h.Apply("reorder in config", s, base("f1", j{"fallback": l{j{"connector_id": "c2"}, j{"connector_id": "c1"}}}))
		s = h.Apply("change connector", s, base("f1", j{"fallback": l{j{"connector_id": "c3"}, j{"connector_id": "c1"}}}))
		s = h.Apply("remove", s, base("f1"))
		h.Refresh("refresh", s)
	},

	"fallback-targets": func(h *grHarness) {
		ft := func(et, connector string, extra ...j) j {
			t := j{"connector_id": connector}
			for _, e := range extra {
				for k, v := range e {
					t[k] = v
				}
			}
			return j{"entity_type": et, "target": t}
		}
		s := h.Apply("create", h.null(), base("g1", j{"fallback_targets": l{ft("alerts", "c1")}}))
		for _, et := range []string{"cases", "test_notifications", "unspecified"} {
			s = h.Apply("entity_type "+et, s, base("g1", j{"fallback_targets": l{ft(et, "c1")}}))
		}
		s = h.Apply("preset set", s, base("g1", j{"fallback_targets": l{ft("alerts", "c1", j{"preset_id": "p"})}}))
		s = h.Apply("custom_details set", s, base("g1", j{"fallback_targets": l{ft("alerts", "c1", j{"custom_details": j{"a": "b"}})}}))
		s = h.Apply("two entries", s, base("g1", j{"fallback_targets": l{ft("alerts", "c1"), ft("cases", "c2")}}))
		s = h.Apply("reorder in config", s, base("g1", j{"fallback_targets": l{ft("cases", "c2"), ft("alerts", "c1")}}))
		s = h.Apply("remove", s, base("g1"))
		h.Refresh("refresh", s)
	},

	// Each case changes one field outside Terraform, then refreshes.
	"drift": func(h *grHarness) {
		full := base("dr", j{
			"description": "d", "disabled": true, "entity_labels": j{"a": "b"},
			"rules":            l{rule("r", j{"custom_details": j{"k": "v"}, "targets": l{j{"connector_id": "c1", "preset_id": "p", "custom_details": j{"x": "y"}}}})},
			"fallback_targets": l{j{"entity_type": "cases", "target": j{"connector_id": "c9"}}},
		})
		cases := []struct {
			name   string
			change func(f *grFake, stored map[string]any)
		}{
			{"description", func(f *grFake, s map[string]any) { s["description"] = "changed" }},
			{"disabled", func(f *grFake, s map[string]any) { s["disabled"] = false }},
			{"entity_labels", func(f *grFake, s map[string]any) { s["entityLabels"] = map[string]any{"z": "1"} }},
			{"routing label", func(f *grFake, s map[string]any) { s["routingLabels"] = map[string]any{"environment": "other"} }},
			{"rule name", func(f *grFake, s map[string]any) { s["rules"].([]any)[0].(map[string]any)["name"] = "renamed" }},
			{"rule condition", func(f *grFake, s map[string]any) { s["rules"].([]any)[0].(map[string]any)["condition"] = "false" }},
			{"rule entity type", func(f *grFake, s map[string]any) { s["rules"].([]any)[0].(map[string]any)["entityType"] = "CASES" }},
			{"rule custom_details", func(f *grFake, s map[string]any) {
				s["rules"].([]any)[0].(map[string]any)["customDetails"] = map[string]any{"n": "m"}
			}},
			{"rule removed", func(f *grFake, s map[string]any) { s["rules"] = []any{} }},
			{"target preset", func(f *grFake, s map[string]any) { f.setRuleTargets(s, 0, []any{tgt("c1", "p-new", "y")}) }},
			{"target custom_details", func(f *grFake, s map[string]any) { f.setRuleTargets(s, 0, []any{tgt("c1", "p", "")}) }},
			{"fallback added", func(f *grFake, s map[string]any) {
				s["fallbackTargets"] = []any{}
				s["fallback"] = f.targets([]any{tgt("c5", "", "")})
			}},
			{"fallback_targets entity type", func(f *grFake, s map[string]any) {
				s["fallbackTargets"].([]any)[0].(map[string]any)["entityType"] = "ALERTS"
			}},
			{"fallback_targets removed", func(f *grFake, s map[string]any) { s["fallbackTargets"] = []any{} }},
		}
		for i, c := range cases {
			cfg := with(full, j{"name": fmt.Sprintf("dr%d", i), "routing_labels": j{"environment": fmt.Sprintf("dr%d", i)}})
			s := h.Apply("create for "+c.name, h.null(), cfg)
			h.routers().mutate(idOf(s), c.change)
			next := h.Refresh("refresh after "+c.name+" changed", s)
			if !next.IsNull() {
				h.Apply("plan the same config after "+c.name, next, cfg)
			}
		}
	},
}
