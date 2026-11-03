// Copyright 2024 Coralogix Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/coralogix/terraform-provider-coralogix/internal/provider/parsing_rules"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils/schemadump"
)

const parsingRulesTypeName = "coralogix_parsing_rules"

// These tests record the behavior of the handwritten coralogix_parsing_rules. After the generator
// switch, the golden diff must show only what the generated resource changes on purpose.
//
//	UPDATE_GOLDEN=1 go test ./internal/provider -run TestParsingRulesBaseline

func newParsingRulesHarness(t *testing.T) *grHarness {
	return newProtocolHarness(t, parsingRulesTypeName, newParsingRulesFake(t))
}

func (h *grHarness) ruleGroups() *parsingRulesFake {
	h.t.Helper()
	f, ok := h.api.(*parsingRulesFake)
	if !ok {
		h.t.Fatal("expected *parsingRulesFake")
	}
	return f
}

func TestParsingRulesBaselineSchema(t *testing.T) {
	var current resource.SchemaResponse
	parsing_rules.NewParsingRulesResource().Schema(context.Background(), resource.SchemaRequest{}, &current)
	checkGoldenAt(t, parsingRulesGoldenDir, "schema_current.txt", []byte(schemadump.Text(current.Schema)))
}

func TestParsingRulesBaselineScenarios(t *testing.T) {
	for name, run := range parsingRulesScenarios {
		t.Run(name, func(t *testing.T) {
			h := newParsingRulesHarness(t)
			run(h)
			checkGoldenAt(t, parsingRulesGoldenDir, "scenario_"+name+".json", marshalGolden(t, h.steps))
		})
	}
}

// prRefreshPlan refreshes a state that an apply made. A refused Create leaves no state to refresh.
func prRefreshPlan(h *grHarness, name string, s tftypes.Value) tftypes.Value {
	if s.IsNull() {
		return s
	}
	return h.RefreshPlan(name, s)
}

// prCfg is a rule group with one subgroup that holds the given rules: nil is one extract rule,
// and an empty list is no subgroup (the API refuses that). extra sets or, with nil, removes
// top-level attributes.
func prCfg(rules []any, extra j) j {
	cfg := j{"name": "tf-rule-group"}
	if rules == nil {
		rules = []any{prRule("extract", j{"source_field": "text", "regular_expression": "a"})}
	}
	if len(rules) > 0 {
		cfg["rule_subgroups"] = []any{j{"rules": rules}}
	}
	for k, v := range extra {
		if v == nil {
			delete(cfg, k)
			continue
		}
		cfg[k] = v
	}
	return cfg
}

// prRule is one rule of the given type.
func prRule(kind string, fields j) j {
	r := j{"name": "rule-" + kind}
	for k, v := range fields {
		r[k] = v
	}
	return j{kind: r}
}

// prEveryRule has one rule of each of the nine types.
func prEveryRule() []any {
	return []any{
		prRule("parse", j{"source_field": "text", "destination_field": "text", "regular_expression": "(?P<a>.*)"}),
		prRule("block", j{"source_field": "text", "regular_expression": "error", "keep_blocked_logs": true}),
		prRule("json_extract", j{"json_key": "worker", "destination_field": "Text", "destination_field_text": "text.worker"}),
		prRule("replace", j{"source_field": "text", "destination_field": "text", "regular_expression": ".*{", "replacement_string": "{"}),
		prRule("extract_timestamp", j{"source_field": "text", "field_format_standard": "javaSDF", "time_format": "yyyy-MM-dd"}),
		prRule("remove_fields", j{"excluded_fields": []any{"a", "b"}}),
		prRule("json_stringify", j{"source_field": "text", "destination_field": "text.out", "keep_source_field": true}),
		prRule("extract", j{"source_field": "text", "regular_expression": "(?P<b>.*)"}),
		prRule("parse_json_field", j{"source_field": "text", "destination_field": "text.parsed", "keep_source_field": true, "keep_destination_field": false}),
	}
}

// prEveryRuleChanged is prEveryRule with every field changed, and every optional field set to a
// value that is not its default.
func prEveryRuleChanged() []any {
	set := j{"description": "changed", "active": false}
	return []any{
		prRule("parse", prWith(set, j{"name": "parse-2", "source_field": "text.a", "destination_field": "text.b", "regular_expression": "(?P<c>.*)"})),
		prRule("block", prWith(set, j{"source_field": "text.a", "regular_expression": "warn", "keep_blocked_logs": false, "block_all_matching_blocks": false})),
		prRule("json_extract", prWith(set, j{"json_key": "thread", "destination_field": "ThreadID"})),
		prRule("replace", prWith(set, j{"source_field": "text.a", "destination_field": "text.b", "regular_expression": "x", "replacement_string": "y"})),
		prRule("extract_timestamp", prWith(set, j{"source_field": "text.a", "field_format_standard": "golang", "time_format": "2006"})),
		prRule("remove_fields", prWith(set, j{"excluded_fields": []any{"c"}})),
		prRule("json_stringify", prWith(set, j{"source_field": "text.a", "destination_field": "text.out2", "keep_source_field": false})),
		prRule("extract", prWith(set, j{"source_field": "text.a", "regular_expression": "(?P<d>.*)"})),
		prRule("parse_json_field", prWith(set, j{"source_field": "text.a", "destination_field": "text.parsed2", "keep_source_field": false, "keep_destination_field": true})),
	}
}

// prEveryRuleRequired is one rule of each type with only the required fields, and the
// replacement_string that the API requires. The keep flags default to deleting the source, which
// the API allows only for a text.<field>.
func prEveryRuleRequired() []any {
	return []any{
		prRule("parse", j{"source_field": "text", "destination_field": "text", "regular_expression": "(?P<a>.*)"}),
		prRule("block", j{"source_field": "text", "regular_expression": "error", "keep_blocked_logs": true}),
		prRule("json_extract", j{"json_key": "worker", "destination_field": "Category"}),
		prRule("replace", j{"source_field": "text", "destination_field": "text", "regular_expression": ".*{", "replacement_string": "{"}),
		prRule("extract_timestamp", j{"source_field": "text", "field_format_standard": "javaSDF", "time_format": "yyyy-MM-dd"}),
		prRule("remove_fields", j{"excluded_fields": []any{"a"}}),
		prRule("json_stringify", j{"source_field": "text.src", "destination_field": "text.out"}),
		prRule("extract", j{"source_field": "text", "regular_expression": "(?P<b>.*)"}),
		prRule("parse_json_field", j{"source_field": "text.src", "destination_field": "text.parsed"}),
	}
}

// prWith returns the fields of both maps.
func prWith(a, b j) j {
	out := j{}
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}

var parsingRulesScenarios = map[string]func(h *grHarness){
	"minimal": func(h *grHarness) {
		h.Apply("create with a name only: the API needs a rule", h.null(), prCfg([]any{}, nil))
		s := h.Apply("create with one rule", h.null(), prCfg(nil, nil))
		s = prRefreshPlan(h, "refresh", s)
		h.Destroy("destroy", s)
	},

	"full-lifecycle": func(h *grHarness) {
		full := prCfg(prEveryRule(), j{
			"description":  "first",
			"active":       true,
			"hidden":       false,
			"creator":      "terraform",
			"order":        1.0,
			"applications": []any{"app-a", "app-b"},
			"subsystems":   []any{"sub-a"},
			"severities":   []any{"error", "critical"},
		})
		s := h.Apply("create with every field and every rule type", h.null(), full)
		s = prRefreshPlan(h, "refresh", s)
		s = h.Apply("update: change every field", s, prCfg([]any{
			prRule("parse", j{"name": "renamed", "description": "parse", "active": false, "source_field": "text.a", "destination_field": "text.b", "regular_expression": "(?P<c>.*)"}),
		}, j{
			"name":         "tf-rule-group-2",
			"description":  "second",
			"active":       false,
			"hidden":       true,
			"creator":      "someone",
			"order":        2.0,
			"applications": []any{"app-c"},
			"subsystems":   []any{"sub-b", "sub-c"},
			"severities":   []any{"debug"},
		}))
		s = h.Apply("update: remove every optional field", s, prCfg(nil, j{"name": "tf-rule-group-2"}))
		s = prRefreshPlan(h, "refresh", s)
		s = h.Apply("update: no subgroup", s, prCfg([]any{}, j{"name": "tf-rule-group-2"}))
		s = prRefreshPlan(h, "refresh", s)
		s = h.Apply("update: set them again", s, full)
		s = prRefreshPlan(h, "refresh", s)
		h.Destroy("destroy", s)
	},

	"rule-update": func(h *grHarness) {
		// Every rule type through an update: change every field, then remove every optional one.
		s := h.Apply("create with every rule type", h.null(), prCfg(prEveryRule(), nil))
		s = h.Apply("update: change every field of every rule", s, prCfg(prEveryRuleChanged(), nil))
		s = prRefreshPlan(h, "refresh", s)
		s = h.Apply("update: remove every optional field of every rule", s, prCfg(prEveryRuleRequired(), nil))
		s = prRefreshPlan(h, "refresh", s)
		// replacement_string is optional, but the API requires replaceNewVal.
		h.Apply("update: remove replacement_string", s, prCfg([]any{
			prRule("replace", j{"source_field": "text", "destination_field": "text", "regular_expression": ".*{"}),
		}, nil))
	},

	"empty-replacement": func(h *grHarness) {
		// replacement_string = "" removes the match. The API takes an empty replaceNewVal.
		empty := prCfg([]any{prRule("replace", j{"source_field": "text", "destination_field": "text", "regular_expression": "a", "replacement_string": ""})}, nil)
		s := h.Apply("create with replacement_string = \"\"", h.null(), empty)
		s = prRefreshPlan(h, "refresh", s)
		s = h.Apply("update: replacement_string = \"b\"", s, prCfg([]any{prRule("replace", j{"source_field": "text", "destination_field": "text", "regular_expression": "a", "replacement_string": "b"})}, nil))
		s = h.Apply("update: replacement_string = \"\" again", s, empty)
		prRefreshPlan(h, "refresh", s)
	},

	"unknown-config": func(h *grHarness) {
		// Values that come from another resource are unknown during the plan. The validators wait
		// for them, and the plan keeps them unknown.
		h.Plan("plan an unknown rule", h.null(), prCfg([]any{unknownJSON{}}, nil))
		h.Plan("plan an unknown rule arm", h.null(), prCfg([]any{j{"extract": unknownJSON{}}}, nil))
		h.Plan("plan an unknown rule arm next to a known one", h.null(), prCfg([]any{j{
			"extract": unknownJSON{},
			"parse":   j{"name": "b", "source_field": "text", "destination_field": "text", "regular_expression": "b"},
		}}, nil))
		h.Plan("plan unknown values in a rule", h.null(), prCfg([]any{
			prRule("extract", j{"source_field": unknownJSON{}, "regular_expression": unknownJSON{}, "active": unknownJSON{}}),
			prRule("json_extract", j{"json_key": "k", "destination_field": unknownJSON{}}),
			prRule("extract_timestamp", j{"source_field": "text", "field_format_standard": unknownJSON{}, "time_format": "%Y"}),
			prRule("remove_fields", j{"excluded_fields": unknownJSON{}}),
		}, nil))
		h.Plan("plan unknown subgroups", h.null(), prCfg(nil, j{"rule_subgroups": unknownJSON{}}))
		h.Plan("plan unknown group values", h.null(), prCfg(nil, j{
			"name": unknownJSON{}, "order": unknownJSON{}, "active": unknownJSON{},
			"applications": unknownJSON{}, "severities": unknownJSON{}, "subsystems": []any{unknownJSON{}},
		}))
		s := h.Apply("create", h.null(), prCfg(nil, nil))
		h.Plan("plan an update with an unknown rule arm", s, prCfg([]any{j{"extract": unknownJSON{}}}, nil))
	},

	"write-api-error": func(h *grHarness) {
		h.ruleGroups().failNext(500)
		h.Apply("create: the API answers 500", h.null(), prCfg(nil, nil))
		s := h.Apply("create", h.null(), prCfg(nil, nil))
		h.ruleGroups().failNext(500)
		h.Apply("update: the API answers 500", s, prCfg(nil, j{"name": "renamed"}))
		h.ruleGroups().failNext(500)
		h.Destroy("destroy: the API answers 500", s)
	},

	"severities": func(h *grHarness) {
		s := h.Apply("create with every severity", h.null(), prCfg(nil, j{
			"severities": []any{"debug", "verbose", "info", "warning", "error", "critical"},
		}))
		s = prRefreshPlan(h, "refresh", s)
		h.Apply("update: no severity", s, prCfg(nil, nil))
	},

	"empty-matchers": func(h *grHarness) {
		cfg := prCfg(nil, j{"applications": []any{}, "subsystems": []any{}, "severities": []any{}})
		s := h.Apply("create with empty applications, subsystems, and severities", h.null(), cfg)
		s = prRefreshPlan(h, "refresh", s)
		h.Apply("update: remove them", s, prCfg(nil, nil))
	},

	"block": func(h *grHarness) {
		// keep_blocked_logs is optional, but the API requires keepBlockedLogs.
		h.Apply("create a block rule without keep_blocked_logs", h.null(), prCfg([]any{
			prRule("block", j{"source_field": "text", "regular_expression": "error"}),
		}, nil))
		s := h.Apply("create a block rule with defaults", h.null(), prCfg([]any{
			prRule("block", j{"source_field": "text", "regular_expression": "error", "keep_blocked_logs": false}),
		}, nil))
		s = prRefreshPlan(h, "refresh", s)
		s = h.Apply("update: allow the matching logs", s, prCfg([]any{
			prRule("block", j{"source_field": "text", "regular_expression": "error", "block_all_matching_blocks": false, "keep_blocked_logs": true}),
		}, nil))
		s = prRefreshPlan(h, "refresh", s)
		s = h.Apply("update: keep_blocked_logs = false", s, prCfg([]any{
			prRule("block", j{"source_field": "text", "regular_expression": "error", "block_all_matching_blocks": false, "keep_blocked_logs": false}),
		}, nil))
		s = h.Apply("update: remove keep_blocked_logs and block_all_matching_blocks", s, prCfg([]any{
			prRule("block", j{"source_field": "text", "regular_expression": "error"}),
		}, nil))
		prRefreshPlan(h, "refresh", s)
	},

	"json-extract-casing": func(h *grHarness) {
		// The attribute is case-insensitive. The state keeps the configured spelling.
		for _, field := range []string{"Text", "TEXT", "threadid", "threadID", "Category", "class", "Method", "severity"} {
			s := h.Apply("create with destination_field "+field, h.null(), prCfg([]any{
				prRule("json_extract", j{"json_key": "k", "destination_field": field, "destination_field_text": "text.k"}),
			}, nil))
			prRefreshPlan(h, "refresh", s)
		}
		s := h.Apply("create with destination_field text", h.null(), prCfg([]any{
			prRule("json_extract", j{"json_key": "k", "destination_field": "text", "destination_field_text": "text.k"}),
		}, nil))
		h.Apply("update: only the case changes", s, prCfg([]any{
			prRule("json_extract", j{"json_key": "k", "destination_field": "Text", "destination_field_text": "text.k"}),
		}, nil))
	},

	"json-extract-casing-import": func(h *grHarness) {
		id := h.ruleGroups().seed(prAPIGroup("seeded", prAPIRule("json_extract", "", j{
			"jsonExtractParameters": j{"destinationFieldType": "DESTINATION_FIELD_THREADID", "rule": "k"},
		})))
		s := h.Import("import a json_extract rule", id)
		h.Apply("plan the config in another case", s, prCfg([]any{
			prRule("json_extract", j{"name": "rule-json_extract", "json_key": "k", "destination_field": "ThreadID"}),
		}, j{"name": "seeded"}))
	},

	"extract-timestamp-standards": func(h *grHarness) {
		var rules []any
		for _, std := range []string{"strftime", "javaSDF", "golang", "secondTS", "milliTS", "microTS", "nanoTS"} {
			rules = append(rules, prRule("extract_timestamp", j{"name": std, "source_field": "text", "field_format_standard": std, "time_format": "%Y"}))
		}
		s := h.Apply("create with every format standard", h.null(), prCfg(rules, nil))
		prRefreshPlan(h, "refresh", s)
	},

	"keep-flags": func(h *grHarness) {
		for _, keep := range []bool{true, false} {
			s := h.Apply("create json_stringify and parse_json_field with every flag "+map[bool]string{true: "true", false: "false"}[keep], h.null(), prCfg([]any{
				prRule("json_stringify", j{"source_field": "text.src", "destination_field": "text.out", "keep_source_field": keep}),
				prRule("parse_json_field", j{"source_field": "text.src", "destination_field": "text.in", "keep_source_field": keep, "keep_destination_field": keep}),
			}, nil))
			prRefreshPlan(h, "refresh", s)
		}
		// keep_source_field = false deletes the source, which the API allows only for a text.<field>.
		s := h.Apply("create json_stringify and parse_json_field from a text field with the defaults", h.null(), prCfg([]any{
			prRule("json_stringify", j{"source_field": "text.src", "destination_field": "text.out"}),
			prRule("parse_json_field", j{"source_field": "text.src", "destination_field": "text.in"}),
		}, nil))
		prRefreshPlan(h, "refresh", s)
		h.Apply("create json_stringify with keep_source_field = false from text", h.null(), prCfg([]any{
			prRule("json_stringify", j{"source_field": "text", "destination_field": "text.out", "keep_source_field": false}),
		}, nil))
		h.Apply("create json_stringify and parse_json_field from text with the defaults", h.null(), prCfg([]any{
			prRule("json_stringify", j{"source_field": "text", "destination_field": "text.out"}),
			prRule("parse_json_field", j{"source_field": "text", "destination_field": "text.in"}),
		}, nil))

	},

	"rule-defaults": func(h *grHarness) {
		// A rule without description and active: the defaults are "" and true.
		s := h.Apply("create rules without description and active", h.null(), prCfg([]any{
			prRule("extract", j{"source_field": "text", "regular_expression": "a"}),
			prRule("remove_fields", j{"excluded_fields": []any{"x"}}),
		}, nil))
		s = prRefreshPlan(h, "refresh", s)
		s = h.Apply("update: description and active set", s, prCfg([]any{
			prRule("extract", j{"source_field": "text", "regular_expression": "a", "description": "d", "active": false}),
			prRule("remove_fields", j{"excluded_fields": []any{"x"}, "description": "", "active": true}),
		}, nil))
		prRefreshPlan(h, "refresh", s)
	},

	"subgroups": func(h *grHarness) {
		one := j{"rules": []any{prRule("extract", j{"source_field": "text", "regular_expression": "a"})}}
		two := j{"rules": []any{
			prRule("parse", j{"source_field": "text", "destination_field": "text", "regular_expression": "b"}),
			prRule("extract", j{"name": "second", "source_field": "text", "regular_expression": "c"}),
		}, "active": false}
		s := h.Apply("create two subgroups", h.null(), j{"name": "tf-rule-group", "rule_subgroups": []any{one, two}})
		s = prRefreshPlan(h, "refresh", s)
		s = h.Apply("update: swap the subgroups", s, j{"name": "tf-rule-group", "rule_subgroups": []any{two, one}})
		s = h.Apply("update: swap the rules of a subgroup", s, j{"name": "tf-rule-group", "rule_subgroups": []any{j{"rules": []any{
			prRule("extract", j{"name": "second", "source_field": "text", "regular_expression": "c"}),
			prRule("parse", j{"source_field": "text", "destination_field": "text", "regular_expression": "b"}),
		}}, one}})
		s = h.Apply("update: remove a subgroup", s, j{"name": "tf-rule-group", "rule_subgroups": []any{one}})
		h.Apply("update: no subgroup", s, prCfg([]any{}, nil))
	},

	"empty-subgroups": func(h *grHarness) {
		h.Apply("create with rule_subgroups = []: the API needs a rule", h.null(), prCfg([]any{}, j{"rule_subgroups": []any{}}))
	},

	"order": func(h *grHarness) {
		first := h.Apply("create a rule group without order", h.null(), prCfg(nil, j{"name": "first"}))
		second := h.Apply("create another rule group without order", h.null(), prCfg(nil, j{"name": "second"}))
		h.Apply("create a rule group at order 1", h.null(), prCfg(nil, j{"name": "third", "order": 1.0}))
		h.Refresh("refresh the first: its order did not move", first)
		h.Apply("update the second to order 1", second, prCfg(nil, j{"name": "second", "order": 1.0}))
	},

	"order-on-update": func(h *grHarness) {
		// Without order in the config, an update plans it unknown. The released resource sends 0,
		// and the API moves the group after every other group.
		first := h.Apply("create a rule group without order", h.null(), prCfg(nil, j{"name": "first"}))
		h.Apply("create another rule group without order", h.null(), prCfg(nil, j{"name": "second"}))
		h.Apply("update: change only the description of the first", first, prCfg(nil, j{"name": "first", "description": "changed"}))
	},

	"config-validation": func(h *grHarness) {
		h.Validate("empty config", j{})
		h.Validate("unknown severity", prCfg(nil, j{"severities": []any{"fatal"}}))
		h.Validate("rule with no type", prCfg([]any{j{}}, nil))
		h.Validate("rule with two types", prCfg([]any{j{
			"extract": j{"name": "a", "source_field": "text", "regular_expression": "a"},
			"parse":   j{"name": "b", "source_field": "text", "destination_field": "text", "regular_expression": "b"},
		}}, nil))
		h.Validate("subgroup without rules", prCfg(nil, j{"rule_subgroups": []any{j{}}}))
		h.Validate("subgroup with empty rules", prCfg(nil, j{"rule_subgroups": []any{j{"rules": []any{}}}}))
		h.Apply("create json_extract to Text without destination_field_text", h.null(), prCfg([]any{
			prRule("json_extract", j{"json_key": "k", "destination_field": "Text"}),
		}, nil))
		h.Validate("unknown destination_field", prCfg([]any{prRule("json_extract", j{"json_key": "k", "destination_field": "message"})}, nil))
		h.Validate("unknown field_format_standard", prCfg([]any{prRule("extract_timestamp", j{"source_field": "text", "field_format_standard": "JavaSDF", "time_format": "x"})}, nil))
		h.Validate("empty excluded_fields", prCfg([]any{prRule("remove_fields", j{"excluded_fields": []any{}})}, nil))
		h.Validate("rule without name", prCfg([]any{j{"extract": j{"source_field": "text", "regular_expression": "a"}}}, nil))
	},

	"create-rejected": func(h *grHarness) {
		h.Apply("create with an invalid regular expression", h.null(), prCfg([]any{
			prRule("extract", j{"source_field": "text", "regular_expression": "(?P<a"}),
		}, nil))
		h.Apply("create with an empty name", h.null(), prCfg(nil, j{"name": ""}))
	},

	"import": func(h *grHarness) {
		id := h.ruleGroups().seed(prAPIGroup("seeded",
			prAPIRule("block", "text", j{"allowParameters": j{"rule": "x", "keepBlockedLogs": true}}),
			prAPIRule("json_parse", "text", j{"jsonParseParameters": j{"destinationField": "text.p", "deleteSource": false, "overrideDest": false, "escapedValue": true}}),
		))
		s := h.Import("import", id)
		h.Apply("plan the matching config", s, prCfg([]any{
			prRule("block", j{"name": "rule-block", "source_field": "text", "regular_expression": "x", "keep_blocked_logs": true, "block_all_matching_blocks": false}),
			prRule("parse_json_field", j{"name": "rule-json_parse", "source_field": "text", "destination_field": "text.p", "keep_source_field": true, "keep_destination_field": true}),
		}, j{"name": "seeded"}))
	},

	"import-minimal": func(h *grHarness) {
		id := h.ruleGroups().seed(prAPIGroup("seeded", prAPIRule("extract", "text", j{"extractParameters": j{"rule": "a"}})))
		s := h.Import("import", id)
		h.Apply("plan the matching config", s, prCfg(nil, j{"name": "seeded"}))
	},

	"import-every-rule": func(h *grHarness) {
		// Every rule type and every group field, with values that are not the defaults.
		id := h.ruleGroups().seed(prAPIGroupEvery())
		s := h.Import("import", id)
		h.Apply("plan the matching config", s, prCfg([]any{
			prRule("parse", j{"description": "d", "active": false, "source_field": "text.a", "destination_field": "text.b", "regular_expression": "(?P<a>.*)"}),
			prRule("block", j{"source_field": "text", "regular_expression": "error", "keep_blocked_logs": true}),
			prRule("json_extract", j{"json_key": "k", "destination_field": "severity"}),
			prRule("replace", j{"source_field": "text", "destination_field": "text.r", "regular_expression": "a", "replacement_string": "b"}),
			prRule("extract_timestamp", j{"source_field": "text", "field_format_standard": "golang", "time_format": "2006"}),
			prRule("remove_fields", j{"excluded_fields": []any{"a", "b"}}),
			prRule("json_stringify", j{"source_field": "text.src", "destination_field": "text.out", "keep_source_field": false}),
			prRule("extract", j{"source_field": "text", "regular_expression": "(?P<b>.*)"}),
			prRule("parse_json_field", j{"source_field": "text.src", "destination_field": "text.p", "keep_source_field": false, "keep_destination_field": false}),
		}, j{
			"name": "seeded", "description": "made outside Terraform", "active": false, "hidden": true, "creator": "someone", "order": 1.0,
			"applications": []any{"app-a"}, "subsystems": []any{"sub-a"}, "severities": []any{"error"},
		}))
	},

	"import-unknown-id": func(h *grHarness) {
		h.Import("import an id that does not exist", "00000000-0000-4000-8000-00000000dead")
	},

	"drift-outside": func(h *grHarness) {
		cfg := prCfg([]any{prRule("extract", j{"source_field": "text", "regular_expression": "a"})}, j{"description": "mine"})
		s := h.Apply("create", h.null(), cfg)
		h.ruleGroups().edit(idOf(s), func(g map[string]any) {
			g["description"] = "changed outside"
			g["enabled"] = false
		})
		s = prRefreshPlan(h, "refresh: changed outside Terraform", s)
		h.Apply("apply the config again", s, cfg)
	},

	"read-not-found": func(h *grHarness) {
		s := h.Apply("create", h.null(), prCfg(nil, nil))
		h.ruleGroups().forget(idOf(s))
		h.Refresh("refresh: rule group was deleted outside Terraform", s)
	},

	"read-api-error": func(h *grHarness) {
		s := h.Apply("create", h.null(), prCfg(nil, nil))
		h.ruleGroups().failNext(500)
		h.Refresh("refresh: the API answers 500", s)
	},

	"delete-not-found": func(h *grHarness) {
		s := h.Apply("create", h.null(), prCfg(nil, nil))
		h.ruleGroups().forget(idOf(s))
		h.Destroy("destroy: the rule group was deleted outside Terraform", s)
	},

	"update-not-found": func(h *grHarness) {
		s := h.Apply("create", h.null(), prCfg(nil, nil))
		h.ruleGroups().forget(idOf(s))
		h.Apply("update: rule group was deleted outside Terraform", s, prCfg(nil, j{"name": "renamed"}))
	},
}

// parsingRulesDataSourceScenarios are the scenarios of the data source. It finds a rule group by
// id only.
var parsingRulesDataSourceScenarios = func() map[string]func(h *grHarness) {
	out := idLookupScenarios(func(h *grHarness) string {
		return h.ruleGroups().seed(prAPIGroupEvery())
	})
	out["read-api-error"] = func(h *grHarness) {
		id := h.ruleGroups().seed(prAPIGroup("seeded", prAPIRule("extract", "text", j{"extractParameters": j{"rule": "a"}})))
		h.ruleGroups().failNext(500)
		h.ReadDataSource("read when the API answers 500", byID(id))
	}
	return out
}()

// prAPIGroup is a rule group as the API takes it, with one subgroup that holds the rules.
func prAPIGroup(name string, rules ...any) j {
	g := j{"name": name, "enabled": true}
	for i, r := range rules {
		r.(j)["order"] = i + 1
	}
	if len(rules) > 0 {
		g["ruleSubgroups"] = []any{j{"enabled": true, "order": 1.0, "rules": rules}}
	}
	return g
}

// prAPIRule is a rule as the API takes it. kind names the rule.
func prAPIRule(kind, sourceField string, parameters j) j {
	r := j{"name": "rule-" + kind, "description": "", "enabled": true, "parameters": parameters}
	if sourceField != "" {
		r["sourceField"] = sourceField
	} else {
		r["sourceField"] = "text"
	}
	return r
}

// prAPIGroupEvery is a rule group as the API takes it, with every group field and one rule of each
// type, all with values that are not the defaults.
func prAPIGroupEvery() j {
	parse := prAPIRule("parse", "text.a", j{"parseParameters": j{"destinationField": "text.b", "rule": "(?P<a>.*)"}})
	parse["description"], parse["enabled"] = "d", false
	g := prAPIGroup("seeded",
		parse,
		prAPIRule("block", "text", j{"blockParameters": j{"keepBlockedLogs": true, "rule": "error"}}),
		prAPIRule("json_extract", "text", j{"jsonExtractParameters": j{"destinationFieldType": "DESTINATION_FIELD_SEVERITY", "rule": "k"}}),
		prAPIRule("replace", "text", j{"replaceParameters": j{"destinationField": "text.r", "replaceNewVal": "b", "rule": "a"}}),
		prAPIRule("extract_timestamp", "text", j{"extractTimestampParameters": j{"format": "2006", "standard": "FORMAT_STANDARD_GOLANG"}}),
		prAPIRule("remove_fields", "text", j{"removeFieldsParameters": j{"fields": []any{"a", "b"}}}),
		prAPIRule("json_stringify", "text.src", j{"jsonStringifyParameters": j{"deleteSource": true, "destinationField": "text.out"}}),
		prAPIRule("extract", "text", j{"extractParameters": j{"rule": "(?P<b>.*)"}}),
		prAPIRule("parse_json_field", "text.src", j{"jsonParseParameters": j{"deleteSource": true, "destinationField": "text.p", "overrideDest": true}}),
	)
	g["description"], g["enabled"], g["hidden"], g["creator"] = "made outside Terraform", false, true, "someone"
	g["ruleMatchers"] = []any{
		j{"applicationName": j{"value": "app-a"}},
		j{"subsystemName": j{"value": "sub-a"}},
		j{"severity": j{"value": "VALUE_ERROR"}},
	}
	return g
}
