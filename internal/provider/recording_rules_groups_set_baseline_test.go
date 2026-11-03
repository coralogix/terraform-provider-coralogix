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
	"net/http"
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/internal/utils/schemadump"
)

const rrTypeName = "coralogix_recording_rules_groups_set"

// These tests record the behavior of the handwritten coralogix_recording_rules_groups_set. After
// the generator switch, the golden diff must show only what the generated resource changes on
// purpose.
//
//	UPDATE_GOLDEN=1 go test ./internal/provider -run TestRecordingRulesBaseline

func newRRHarness(t *testing.T) *grHarness {
	return newProtocolHarness(t, rrTypeName, newRRFake(t))
}

func (h *grHarness) recordingRules() *rrFake {
	h.t.Helper()
	f, ok := h.api.(*rrFake)
	if !ok {
		h.t.Fatal("expected *rrFake")
	}
	return f
}

func TestRecordingRulesBaselineSchema(t *testing.T) {
	checkGoldenAt(t, rrGoldenDir, "schema_current.txt", []byte(schemadump.Text(registeredResourceSchema(t, rrTypeName))))
}

func TestRecordingRulesBaselineScenarios(t *testing.T) {
	for name, run := range rrScenarios {
		t.Run(name, func(t *testing.T) {
			h := newRRHarness(t)
			run(h)
			checkGoldenAt(t, rrGoldenDir, "scenario_"+name+".json", marshalGolden(t, h.steps))
		})
	}
}

func rrRule(record string, extra j) j {
	r := j{"record": record, "expr": "vector(1)"}
	for k, v := range extra {
		r[k] = v
	}
	return r
}

func rrGroup(name string, extra j, rules ...any) j {
	g := j{"name": name, "rules": rules}
	if rules == nil {
		g["rules"] = []any{}
	}
	for k, v := range extra {
		g[k] = v
	}
	return g
}

func rrCfg(name string, groups ...any) j {
	cfg := j{"groups": groups}
	if name != "" {
		cfg["name"] = name
	}
	return cfg
}

const rrYAML = `name: from-yaml
groups:
  - name: yaml-a
    interval: 120
    limit: 7
    rules:
      - record: tf:yaml_a
        expr: vector(1)
        labels:
          team: a
      - record: tf:yaml_b
        expr: vector(2)
        evaluationdelayms: 30000
  - name: yaml-b
    rules:
      - record: tf:yaml_c
        expr: vector(3)
`

const rrYAMLChanged = `name: from-yaml
groups:
  - name: yaml-a
    interval: 180
    rules:
      - record: tf:yaml_a
        expr: vector(1)
`

// rrStateV0 is a state that schema version 0 wrote: `group` and `rule` instead of `groups` and
// `rules`, and a rule with only record, expr, and labels.
const rrStateV0 = `{
  "id": "01J00000000000000000000001",
  "yaml_content": null,
  "name": "set-v0",
  "group": [{
    "name": "g1",
    "interval": 180,
    "limit": 100,
    "rule": [{"record": "tf:v0", "expr": "vector(1)", "labels": {"team": "backend"}}]
  }]
}`

var rrScenarios = map[string]func(h *grHarness){
	"minimal": func(h *grHarness) {
		s := h.Apply("create", h.null(), rrCfg("tf-set", rrGroup("g1", nil, rrRule("tf:r1", nil))))
		s = h.RefreshPlan("refresh", s)
		h.Destroy("destroy", s)
	},

	// The resource sends the unknown name as "", which the API refuses.
	"without-name": func(h *grHarness) {
		h.Apply("create without a name", h.null(), rrCfg("", rrGroup("g1", nil, rrRule("tf:r1", nil))))
	},

	"full-lifecycle": func(h *grHarness) {
		full := rrCfg("tf-set",
			rrGroup("g-z", j{"interval": 120.0, "limit": 5.0},
				rrRule("tf:b", j{"labels": j{"a": "1", "empty": ""}, "evaluation_delay_ms": 0.0}),
				rrRule("tf:a", j{"evaluation_delay_ms": 60000.0})),
			rrGroup("g-a", j{"interval": 60.0, "limit": 0.0}, rrRule("tf:r1", j{"labels": j{}})))
		s := h.Apply("create", h.null(), full)
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: rename the set, change a rule, add a group, drop a group", s, rrCfg("tf-set-2",
			rrGroup("g-z", j{"interval": 180.0, "limit": 5.0},
				rrRule("tf:b", j{"labels": j{"a": "2"}, "evaluation_delay_ms": 1800000.0}),
				rrRule("tf:a", nil)),
			rrGroup("g-m", j{"interval": 300.0, "limit": 10.0})))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: reorder the rules", s, rrCfg("tf-set-2",
			rrGroup("g-z", j{"interval": 180.0, "limit": 5.0},
				rrRule("tf:a", nil),
				rrRule("tf:b", j{"labels": j{"a": "2"}, "evaluation_delay_ms": 1800000.0})),
			rrGroup("g-m", j{"interval": 300.0, "limit": 10.0})))
		h.Destroy("destroy", s)
	},

	// interval, limit, and labels keep the state value when removed, and evaluation_delay_ms is
	// cleared.
	"unset-optional": func(h *grHarness) {
		s := h.Apply("create", h.null(), rrCfg("tf-set",
			rrGroup("g1", j{"interval": 120.0, "limit": 5.0}, rrRule("tf:r1", j{"labels": j{"a": "1"}, "evaluation_delay_ms": 1000.0}))))
		s = h.Apply("update: remove interval, limit, labels, and evaluation_delay_ms", s, rrCfg("tf-set",
			rrGroup("g1", nil, rrRule("tf:r1", nil))))
		h.RefreshPlan("refresh", s)
	},

	"add-optional": func(h *grHarness) {
		s := h.Apply("create", h.null(), rrCfg("tf-set", rrGroup("g1", nil, rrRule("tf:r1", nil))))
		s = h.Apply("update: add interval, limit, labels, and evaluation_delay_ms", s, rrCfg("tf-set",
			rrGroup("g1", j{"interval": 120.0, "limit": 5.0}, rrRule("tf:r1", j{"labels": j{"a": "1"}, "evaluation_delay_ms": 1000.0}))))
		h.RefreshPlan("refresh", s)
	},

	"remove-name": func(h *grHarness) {
		s := h.Apply("create", h.null(), rrCfg("tf-set", rrGroup("g1", nil, rrRule("tf:r1", nil))))
		s = h.Apply("update: remove the name", s, rrCfg("", rrGroup("g1", nil, rrRule("tf:r1", nil))))
		h.RefreshPlan("refresh", s)
	},

	"zero-values": func(h *grHarness) {
		s := h.Apply("create: limit and evaluation_delay_ms are 0, labels are empty", h.null(), rrCfg("tf-set",
			rrGroup("g1", j{"interval": 60.0, "limit": 0.0}, rrRule("tf:r1", j{"labels": j{}, "evaluation_delay_ms": 0.0}))))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: evaluation_delay_ms from 0 to null", s, rrCfg("tf-set",
			rrGroup("g1", j{"interval": 60.0, "limit": 0.0}, rrRule("tf:r1", j{"labels": j{}}))))
		h.RefreshPlan("refresh", s)
	},

	"empty-rules": func(h *grHarness) {
		s := h.Apply("create a group without rules", h.null(), rrCfg("tf-set", rrGroup("g1", nil)))
		h.RefreshPlan("refresh", s)
	},

	"change-record-and-expr": func(h *grHarness) {
		s := h.Apply("create", h.null(), rrCfg("tf-set", rrGroup("g1", nil, rrRule("tf:r1", nil))))
		s = h.Apply("update: change the record and the expression", s, rrCfg("tf-set",
			rrGroup("g1", nil, rrRule("tf:r1_changed", j{"expr": "vector(2)"}))))
		h.RefreshPlan("refresh", s)
	},

	// The API accepts a group without rules on Update, and refuses a set without groups. The
	// handwritten resource reads the empty rules back as null, an inconsistent result.
	"clear-collections": func(h *grHarness) {
		s := h.Apply("create", h.null(), rrCfg("tf-set",
			rrGroup("g1", nil, rrRule("tf:r1", nil)), rrGroup("g2", nil, rrRule("tf:r2", nil))))
		s = h.Apply("update: empty the rules of a group", s, rrCfg("tf-set",
			rrGroup("g1", nil, rrRule("tf:r1", nil)), rrGroup("g2", nil)))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: empty groups", s, j{"name": "tf-set", "groups": []any{}})
		h.RefreshPlan("refresh", s)
	},

	"yaml-content": func(h *grHarness) {
		s := h.Apply("create", h.null(), j{"yaml_content": rrYAML})
		s = h.RefreshPlan("refresh", s)
		h.Apply("update: another document replaces the resource", s, j{"yaml_content": rrYAMLChanged})
		h.Destroy("destroy", s)
	},

	"yaml-content-name": func(h *grHarness) {
		s := h.Apply("create: the name attribute wins over the document", h.null(), j{"yaml_content": rrYAML, "name": "tf-name"})
		s = h.RefreshPlan("refresh", s)
		h.Apply("update: rename", s, j{"yaml_content": rrYAML, "name": "tf-name-2"})
	},

	// Neither the document nor the name attribute names the set, so the API names it after its id.
	"yaml-content-without-name": func(h *grHarness) {
		s := h.Apply("create", h.null(), j{"yaml_content": "groups:\n  - name: yaml-a\n    rules:\n      - record: tf:yaml_a\n        expr: vector(1)\n"})
		s = h.RefreshPlan("refresh", s)
		h.Destroy("destroy", s)
	},

	"yaml-content-to-groups": func(h *grHarness) {
		s := h.Apply("create", h.null(), j{"yaml_content": rrYAMLChanged})
		h.Apply("update: groups instead of yaml_content", s, rrCfg("from-yaml", rrGroup("yaml-a", j{"interval": 180.0}, rrRule("tf:yaml_a", nil))))
	},

	"groups-to-yaml-content": func(h *grHarness) {
		s := h.Apply("create", h.null(), rrCfg("from-yaml", rrGroup("yaml-a", j{"interval": 180.0}, rrRule("tf:yaml_a", nil))))
		h.Apply("update: yaml_content instead of groups", s, j{"yaml_content": rrYAMLChanged})
	},

	"config-validation": func(h *grHarness) {
		h.Validate("no groups and no yaml_content", j{"name": "tf-set"})
		h.Validate("groups and yaml_content", j{"yaml_content": rrYAML, "groups": []any{rrGroup("g1", nil, rrRule("tf:r1", nil))}})
		h.Validate("interval below 60", rrCfg("tf-set", rrGroup("g1", j{"interval": 30.0}, rrRule("tf:r1", nil))))
		h.Validate("evaluation_delay_ms above 30 minutes", rrCfg("tf-set", rrGroup("g1", nil, rrRule("tf:r1", j{"evaluation_delay_ms": 1800001.0}))))
		h.Validate("negative evaluation_delay_ms", rrCfg("tf-set", rrGroup("g1", nil, rrRule("tf:r1", j{"evaluation_delay_ms": -1.0}))))
		h.Validate("empty group name", rrCfg("tf-set", rrGroup("", nil, rrRule("tf:r1", nil))))
		h.Validate("negative limit", rrCfg("tf-set", rrGroup("g1", j{"limit": -1.0}, rrRule("tf:r1", nil))))
		h.Validate("empty groups", j{"name": "tf-set", "groups": []any{}})
		h.Validate("yaml_content that is not YAML", j{"yaml_content": "groups: [\n"})
		h.Validate("yaml_content with an empty group name", j{"yaml_content": "groups:\n  - rules: []\n"})
		h.Validate("yaml_content with an unknown key", j{"yaml_content": "groups:\n  - name: g\n    evaluation_delay_ms: 5\n"})
	},

	"create-rejected": func(h *grHarness) {
		h.Apply("create: the expression is not PromQL", h.null(), rrCfg("tf-set", rrGroup("g1", nil, rrRule("tf:r1", j{"expr": "sum("}))))
		h.Apply("create: two groups have one name", h.null(), rrCfg("tf-set",
			rrGroup("g1", j{"interval": 60.0}, rrRule("tf:r1", nil)), rrGroup("g1", j{"interval": 120.0}, rrRule("tf:r1", nil))))
		h.Apply("create: empty groups", h.null(), j{"name": "tf-set", "groups": []any{}})
	},

	"duplicate-name": func(h *grHarness) {
		h.recordingRules().seed(map[string]any{"name": "tf-set", "groups": []any{map[string]any{"name": "g", "rules": []any{}}}})
		h.Apply("create: another set has the name", h.null(), rrCfg("tf-set", rrGroup("g1", nil, rrRule("tf:r1", nil))))
	},

	"create-read-fails": func(h *grHarness) {
		h.recordingRules().failMethod(http.MethodGet, http.StatusInternalServerError, "Internal: rule store unavailable")
		h.Apply("create: the read after create fails", h.null(), rrCfg("tf-set", rrGroup("g1", nil, rrRule("tf:r1", nil))))
	},

	"import": func(h *grHarness) {
		id := h.recordingRules().seed(map[string]any{"name": "seeded", "groups": []any{
			map[string]any{"name": "g-b", "interval": 120, "limit": "5", "rules": []any{
				map[string]any{"record": "tf:b", "expr": "vector(1)", "labels": map[string]any{"a": "1"}, "evaluationDelayMs": 0},
				map[string]any{"record": "tf:a", "expr": "vector(2)"},
			}},
			map[string]any{"name": "g-a", "rules": []any{map[string]any{"record": "tf:c", "expr": "vector(3)"}}},
		}})
		s := h.Import("import", id)
		h.Apply("plan the matching config", s, rrCfg("seeded",
			rrGroup("g-b", j{"interval": 120.0, "limit": 5.0},
				rrRule("tf:b", j{"labels": j{"a": "1"}, "evaluation_delay_ms": 0.0}),
				rrRule("tf:a", j{"expr": "vector(2)"})),
			rrGroup("g-a", nil, rrRule("tf:c", j{"expr": "vector(3)"}))))
	},

	"import-minimal-config": func(h *grHarness) {
		id := h.recordingRules().seed(map[string]any{"groups": []any{map[string]any{"name": "g1", "rules": []any{map[string]any{"record": "tf:r1", "expr": "vector(1)"}}}}})
		s := h.Import("import a set without a name", id)
		h.Apply("plan a config without name, interval, limit, or labels", s, rrCfg("", rrGroup("g1", nil, rrRule("tf:r1", nil))))
	},

	"import-yaml-content": func(h *grHarness) {
		id := h.recordingRules().seed(map[string]any{"name": "from-yaml", "groups": []any{map[string]any{"name": "yaml-a", "interval": 180, "rules": []any{map[string]any{"record": "tf:yaml_a", "expr": "vector(1)"}}}}})
		s := h.Import("import", id)
		h.Apply("plan a yaml_content config", s, j{"yaml_content": rrYAMLChanged})
	},

	"import-unknown-id": func(h *grHarness) {
		h.Import("import an id that does not exist", "01J00000000000000000000099")
	},

	"read-not-found": func(h *grHarness) {
		s := h.Apply("create", h.null(), rrCfg("tf-set", rrGroup("g1", nil, rrRule("tf:r1", nil))))
		h.recordingRules().forget(idOf(s))
		h.RefreshPlan("refresh: the set was deleted outside Terraform", s)
	},

	"read-api-error": func(h *grHarness) {
		s := h.Apply("create", h.null(), rrCfg("tf-set", rrGroup("g1", nil, rrRule("tf:r1", nil))))
		h.recordingRules().failMethod(http.MethodGet, http.StatusInternalServerError, "Internal: rule store unavailable")
		h.RefreshPlan("refresh: the API fails", s)
	},

	"route-missing": func(h *grHarness) {
		s := h.Apply("create", h.null(), rrCfg("tf-set", rrGroup("g1", nil, rrRule("tf:r1", nil))))
		h.recordingRules().failMethod(http.MethodGet, http.StatusNotFound, "Not Found")
		h.RefreshPlan("refresh: the gateway has no such route", s)
	},

	"drift": func(h *grHarness) {
		s := h.Apply("create", h.null(), rrCfg("tf-set", rrGroup("g1", nil, rrRule("tf:r1", nil))))
		f := h.recordingRules()
		f.mu.Lock()
		f.update(idOf(s), map[string]any{"name": "renamed", "groups": []any{
			map[string]any{"name": "g1", "interval": 300.0, "limit": "3", "rules": []any{map[string]any{"record": "tf:r1", "expr": "vector(1)", "labels": map[string]any{"x": "y"}}}},
			map[string]any{"name": "g2", "rules": []any{}},
		}})
		f.mu.Unlock()
		s = h.RefreshPlan("refresh: the set was changed outside Terraform", s)
		h.Apply("apply the config again", s, rrCfg("tf-set", rrGroup("g1", nil, rrRule("tf:r1", nil))))
	},

	"update-rejected": func(h *grHarness) {
		s := h.Apply("create", h.null(), rrCfg("tf-set", rrGroup("g1", nil, rrRule("tf:r1", nil))))
		s = h.Apply("update: the expression is not PromQL", s, rrCfg("tf-set", rrGroup("g1", nil, rrRule("tf:r1", j{"expr": "sum("}))))
		h.RefreshPlan("refresh", s)
	},

	"update-not-found": func(h *grHarness) {
		s := h.Apply("create", h.null(), rrCfg("tf-set", rrGroup("g1", nil, rrRule("tf:r1", nil))))
		h.recordingRules().forget(idOf(s))
		h.Apply("update: the set was deleted outside Terraform", s, rrCfg("tf-set-2", rrGroup("g1", nil, rrRule("tf:r1", nil))))
	},

	"update-read-fails": func(h *grHarness) {
		s := h.Apply("create", h.null(), rrCfg("tf-set", rrGroup("g1", nil, rrRule("tf:r1", nil))))
		h.recordingRules().failMethod(http.MethodGet, http.StatusInternalServerError, "Internal: rule store unavailable")
		s = h.Apply("update: the read after the update fails", s, rrCfg("tf-set-2", rrGroup("g1", nil, rrRule("tf:r1", nil))))
		h.recordingRules().heal()
		h.RefreshPlan("refresh after the API recovers", s)
	},

	"delete-not-found": func(h *grHarness) {
		s := h.Apply("create", h.null(), rrCfg("tf-set", rrGroup("g1", nil, rrRule("tf:r1", nil))))
		h.recordingRules().forget(idOf(s))
		h.Destroy("destroy: the set was deleted outside Terraform", s)
	},

	"delete-api-error": func(h *grHarness) {
		s := h.Apply("create", h.null(), rrCfg("tf-set", rrGroup("g1", nil, rrRule("tf:r1", nil))))
		h.recordingRules().failMethod(http.MethodDelete, http.StatusInternalServerError, "Internal: rule store unavailable")
		h.Destroy("destroy: the API fails", s)
	},

	"upgrade-v0": func(h *grHarness) {
		h.recordingRules().seed(map[string]any{"name": "set-v0", "groups": []any{map[string]any{"name": "g1", "interval": 180, "limit": "100", "rules": []any{
			map[string]any{"record": "tf:v0", "expr": "vector(1)", "labels": map[string]any{"team": "backend"}},
		}}}})
		s := h.UpgradeV0("upgrade a version 0 state", rrStateV0)
		s = h.Refresh("refresh", s)
		h.Apply("plan the matching config", s, rrCfg("set-v0", rrGroup("g1", j{"interval": 180.0, "limit": 100.0}, rrRule("tf:v0", j{"labels": j{"team": "backend"}}))))
	},
}

// rrDataSourceScenarios read a set by its id, the only lookup of the data source.
var rrDataSourceScenarios = func() map[string]func(h *grHarness) {
	out := idLookupScenarios(func(h *grHarness) string {
		return h.recordingRules().seed(map[string]any{"name": "seeded", "groups": []any{
			map[string]any{"name": "g1", "interval": 120, "limit": "5", "rules": []any{
				map[string]any{"record": "tf:r1", "expr": "vector(1)", "labels": map[string]any{"a": "1"}, "evaluationDelayMs": 0},
			}},
		}})
	})
	out["read-minimal"] = func(h *grHarness) {
		id := h.recordingRules().seed(map[string]any{"groups": []any{map[string]any{"name": "g1", "rules": []any{}}}})
		h.ReadDataSource("read a set without a name, interval, limit, or rules", byID(id))
	}
	return out
}()
