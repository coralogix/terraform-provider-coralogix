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
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"

	"github.com/coralogix/terraform-provider-coralogix/internal/provider/notifications"
	globalrouterschema "github.com/coralogix/terraform-provider-coralogix/internal/provider/notifications/global_router_schema"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils/schemadump"
)

// These tests record the behavior of coralogix_global_router in golden files.
// They are the baseline for replacing the handwritten resource with generated
// code: the generated resource must produce the same files, except where a
// confirmed bug of the current code is fixed on purpose.
//
// The schema tests compare schema text. The scenario tests drive the provider
// through the plugin protocol against an in-memory backend (see
// global_router_baseline_fake_test.go). The files record the bugs of the
// current code as they are. After the switch, every change in a file must
// map to one listed bug fix. Any other change is a behavior change.
//
//	UPDATE_GOLDEN=1 go test ./internal/provider -run TestGlobalRouterBaseline

type (
	j = map[string]any
	l = []any
)

func TestGlobalRouterBaselineSchema(t *testing.T) {
	var current resource.SchemaResponse
	notifications.NewGlobalRouterResource().Schema(context.Background(), resource.SchemaRequest{}, &current)

	checkGolden(t, "schema_current.txt", []byte(schemadump.Text(current.Schema)))
	// Version 0 stays frozen so that stored state can still be read.
	checkGolden(t, "schema_v0.txt", []byte(schemadump.Text(globalrouterschema.V0())))
}

func TestGlobalRouterBaselineScenarios(t *testing.T) {
	for name, run := range grScenarios {
		t.Run(name, func(t *testing.T) {
			h := newGRHarness(t)
			run(h)
			checkGolden(t, "scenario_"+name+".json", marshalGolden(t, h.steps))
		})
	}
}

// grFullConfig sets every attribute except the deprecated fallback.
var grFullConfig = j{
	"name":           "main",
	"description":    "primary router",
	"disabled":       true,
	"entity_labels":  j{"team": "a"},
	"routing_labels": j{"environment": "prod", "service": "api", "team": "sre"},
	"rules": l{
		j{
			"name": "critical", "condition": "alert.priority == 'P1'", "entity_type": "alerts",
			"custom_details": j{"k": "v"},
			"targets": l{
				j{"connector_id": "c1", "preset_id": "p1", "custom_details": j{"x": "y"}},
				j{"connector_id": "c2"},
			},
		},
		j{
			"name": "cases", "condition": "true", "entity_type": "cases",
			"targets": l{j{"connector_id": "c3"}},
		},
	},
	"fallback_targets": l{
		j{"entity_type": "cases", "target": j{"connector_id": "c4", "preset_id": "p4"}},
	},
}

func with(base j, overrides j) j {
	out := maps.Clone(base)
	maps.Copy(out, overrides)
	return out
}

// router is a config with a name, a description, and one routing label. The real
// API needs at least one label and a unique name.
func router(name string, extra ...j) j {
	out := j{"name": name, "description": "d", "routing_labels": j{"environment": name}}
	for _, e := range extra {
		maps.Copy(out, e)
	}
	return out
}

var grScenarios = map[string]func(h *grHarness){
	// A router with only the required attributes. The API returns description "".
	"minimal": func(h *grHarness) {
		cfg := j{"name": "minimal", "routing_labels": j{"environment": "minimal"}}
		s := h.Apply("create", h.null(), cfg)
		s = h.Refresh("refresh", s)
		h.Destroy("destroy", s)
	},

	"full-lifecycle": func(h *grHarness) {
		s := h.Apply("create", h.null(), grFullConfig)
		s = h.Refresh("refresh", s)
		s = h.Apply("update: drop the second rule, change description", s, with(grFullConfig, j{
			"description": "changed",
			"rules":       grFullConfig["rules"].(l)[:1],
		}))
		h.Destroy("destroy", s)
	},

	// Removing optional attributes from the config.
	"unset-optional-attributes": func(h *grHarness) {
		s := h.Apply("create", h.null(), grFullConfig)
		s = h.Apply("update: only name and routing_labels are left", s, j{
			"name": "main", "routing_labels": grFullConfig["routing_labels"],
		})
		h.Refresh("refresh", s)
	},

	// Removing one Optional+Computed attribute while nothing else changes.
	"unset-one-computed-attribute": func(h *grHarness) {
		s := h.Apply("create", h.null(), grFullConfig)
		noRules := maps.Clone(grFullConfig)
		delete(noRules, "rules")
		h.Apply("update: rules removed from the config", s, noRules)
		noLabels := maps.Clone(grFullConfig)
		delete(noLabels, "entity_labels")
		h.Apply("update: entity_labels removed from the config", s, noLabels)
		h.Apply("update: rules set to an empty list", s, with(grFullConfig, j{"rules": l{}}))
	},

	"deprecated-fallback": func(h *grHarness) {
		cfg := router("fb", j{"fallback": l{j{"connector_id": "c1"}, j{"connector_id": "c2", "preset_id": "p2"}}})
		s := h.Apply("create", h.null(), cfg)
		s = h.Apply("update: remove fallback", s, router("fb"))
		h.Refresh("refresh", s)
	},

	// The API refuses ENTITY_TYPE_UNSPECIFIED, and the schema default sends it.
	"rule-entity-type": func(h *grHarness) {
		rule := j{"name": "r", "condition": "true", "targets": l{j{"connector_id": "c1"}}}
		h.Apply("create: rule without entity_type", h.null(), router("et1", j{"rules": l{rule}}))
		h.Apply("create: entity_type unspecified", h.null(), router("et2", j{"rules": l{with(rule, j{"entity_type": "unspecified"})}}))
		h.Apply("create: entity_type test_notifications", h.null(), router("et3", j{"rules": l{with(rule, j{"entity_type": "test_notifications"})}}))
	},

	"explicit-empty-collections": func(h *grHarness) {
		h.Apply("create: empty fallback_targets", h.null(), router("e1", j{"fallback_targets": l{}}))
		h.Apply("create: empty fallback", h.null(), router("e2", j{"fallback": l{}}))
		h.Apply("create: empty rules, entity_labels, custom_details", h.null(), router("e3", j{
			"rules": l{}, "entity_labels": j{},
		}))
		h.Apply("create: empty targets and custom_details in a rule", h.null(), router("e4", j{
			"rules": l{j{"name": "r", "condition": "true", "entity_type": "alerts", "targets": l{}, "custom_details": j{}}},
		}))
	},

	"config-validation": func(h *grHarness) {
		rule := j{"name": "r", "condition": "true", "entity_type": "alerts"}
		h.Validate("missing name", j{})
		h.Validate("entity_type olly_scheduled_tasks", router("v", j{"rules": l{with(rule, j{"entity_type": "olly_scheduled_tasks"})}}))
		h.Validate("entity_type ALERTS in upper case", router("v", j{"rules": l{with(rule, j{"entity_type": "ALERTS"})}}))
		h.Validate("rule without condition", router("v", j{"rules": l{j{"name": "r"}}}))
		h.Validate("rule without name", router("v", j{"rules": l{j{"condition": "true"}}}))
		h.Validate("target without connector_id", router("v", j{"rules": l{with(rule, j{"targets": l{j{"preset_id": "p"}}})}}))
		h.Validate("fallback_targets without target", router("v", j{"fallback_targets": l{j{"entity_type": "alerts"}}}))
		h.Validate("fallback_targets entity_type olly_scheduled_tasks", router("v", j{"fallback_targets": l{
			j{"entity_type": "olly_scheduled_tasks", "target": j{"connector_id": "c"}},
		}}))
	},

	// The API returns rule targets in a different order than the plan.
	"order-rule-targets": func(h *grHarness) {
		cfg := router("o", j{"rules": l{j{"name": "r", "condition": "true", "entity_type": "alerts", "targets": l{
			j{"connector_id": "c1"}, j{"connector_id": "c2"}, j{"connector_id": "c3"},
		}}}})
		h.api.reverseLists = true
		s := h.Apply("create: response has reversed targets", h.null(), cfg)
		s = h.Refresh("refresh: response has reversed targets", s)
		h.Apply("update: same config", s, with(cfg, j{"description": "x"}))
	},

	// The same for the lists that are not protected today.
	"order-fallback-lists": func(h *grHarness) {
		fallback := j{"fallback": l{j{"connector_id": "c1"}, j{"connector_id": "c2"}}}
		h.api.reverseLists = true
		h.Apply("create fallback: response has reversed order", h.null(), router("o1", fallback))
		h.Apply("create fallback_targets: response has reversed order", h.null(), router("o2", j{"fallback_targets": l{
			j{"entity_type": "alerts", "target": j{"connector_id": "c1"}},
			j{"entity_type": "cases", "target": j{"connector_id": "c2"}},
		}}))
		h.api.reverseLists = false
		s := h.Apply("create in order", h.null(), router("o3", fallback))
		h.api.reverseLists = true
		h.Refresh("refresh: response has reversed order", s)
	},

	// The user sets the id.
	"client-set-id": func(h *grHarness) {
		s := h.Apply("create with id", h.null(), router("mine", j{"id": "my-router"}))
		s = h.Apply("update", s, router("mine", j{"id": "my-router", "description": "changed"}))
		h.Import("import", "my-router")
		h.Destroy("destroy", s)
	},

	"import": func(h *grHarness) {
		id := h.api.seed(j{
			"name": "seeded", "description": "made outside Terraform",
			"routingLabels": j{"environment": "prod"},
			"rules": l{j{"name": "r", "condition": "true", "entityType": "CASES", "targets": l{
				j{"connectorId": "c1"}, j{"connectorId": "c2", "presetId": "p"},
			}}},
			"fallbackTargets": l{j{"entityType": "ALERTS", "target": j{"connectorId": "c9"}}},
		})
		s := h.Import("import", id)
		h.Apply("plan the matching config", s, j{
			"name": "seeded", "description": "made outside Terraform",
			"routing_labels": j{"environment": "prod"},
			"rules": l{j{"name": "r", "condition": "true", "entity_type": "cases", "targets": l{
				j{"connector_id": "c1"}, j{"connector_id": "c2", "preset_id": "p"},
			}}},
			"fallback_targets": l{j{"entity_type": "alerts", "target": j{"connector_id": "c9"}}},
		})
	},

	"import-unknown-id": func(h *grHarness) {
		h.Import("import an id that does not exist", "does-not-exist")
	},

	"read-not-found": func(h *grHarness) {
		s := h.Apply("create", h.null(), router("gone"))
		h.api.forget(idOf(s))
		h.Refresh("refresh: router was deleted outside Terraform", s)
	},

	"delete-not-found": func(h *grHarness) {
		s := h.Apply("create", h.null(), router("gone"))
		h.api.forget(idOf(s))
		h.api.deleteStatus = 404
		h.Destroy("destroy: the API answers 404", s)
	},

	"update-not-found": func(h *grHarness) {
		s := h.Apply("create", h.null(), router("gone"))
		h.api.forget(idOf(s))
		h.Apply("update: router was deleted outside Terraform", s, router("gone", j{"description": "changed"}))
	},

	// State written by schema version 0 is upgraded by reading the router.
	"upgrade-from-v0": func(h *grHarness) {
		id := h.api.seed(j{
			"name": "old", "description": "written by v0", "routingLabels": j{"environment": "old"},
			"rules": l{j{"name": "r", "condition": "true", "entityType": "ALERTS", "targets": l{j{"connectorId": "c1"}}}},
		})
		h.UpgradeV0("upgrade", `{"id":"`+id+`","name":"old"}`)
		h.UpgradeV0("upgrade an id that does not exist", `{"id":"does-not-exist","name":"old"}`)
	},
}
