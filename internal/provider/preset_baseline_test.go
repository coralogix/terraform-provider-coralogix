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
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"

	"github.com/coralogix/terraform-provider-coralogix/internal/provider/notifications"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils/schemadump"
)

const presetGoldenDir = "testdata/preset_baseline"

// These tests record the behavior of the handwritten coralogix_preset. After
// the generator switch, the golden diff must show only message wording.
//
//	UPDATE_GOLDEN=1 go test ./internal/provider -run TestPresetBaseline

func TestPresetBaselineSchema(t *testing.T) {
	var current resource.SchemaResponse
	notifications.NewPresetResource().Schema(context.Background(), resource.SchemaRequest{}, &current)
	checkGoldenAt(t, presetGoldenDir, "schema_current.txt", []byte(schemadump.Text(current.Schema)))
}

func TestPresetBaselineScenarios(t *testing.T) {
	for name, run := range presetScenarios {
		t.Run(name, func(t *testing.T) {
			h := newPresetHarness(t)
			run(h)
			checkGoldenAt(t, presetGoldenDir, "scenario_"+name+".json", marshalGolden(t, h.steps))
		})
	}
}

func presetConfig(name string, extra ...j) j {
	out := j{
		"name":           name,
		"entity_type":    "alerts",
		"connector_type": "generic_https",
		"parent_id":      presetSystemParent,
	}
	for _, e := range extra {
		for k, v := range e {
			out[k] = v
		}
	}
	return out
}

func presetOverride(payload string, fields l) j {
	override := j{
		"condition_type": j{"match_entity_type": j{}},
		"message_config": j{"fields": fields},
	}
	if payload != "" {
		override["payload_type"] = payload
	}
	return override
}

var presetFullConfig = presetConfig("main", j{
	"description":       "primary preset",
	"attachment_config": "ENABLED",
	"config_overrides": l{
		presetOverride("email_default", l{
			j{"field_name": "title", "template": "{{alertDef.name}}"},
			j{"field_name": "body", "template": "{{alert.status}}"},
		}),
		j{
			"condition_type": j{"match_entity_type_and_sub_type": j{"entity_sub_type": "logsImmediateResolved"}},
			"payload_type":   "slack_basic",
			"message_config": j{"fields": l{
				j{"field_name": "description", "template": "{{alertDef.description}}"},
			}},
		},
	},
})

var presetScenarios = map[string]func(h *grHarness){
	"minimal": func(h *grHarness) {
		s := h.Apply("create", h.null(), presetConfig("minimal"))
		s = h.Refresh("refresh", s)
		h.Destroy("destroy", s)
	},

	"full-lifecycle": func(h *grHarness) {
		s := h.Apply("create", h.null(), presetFullConfig)
		s = h.Refresh("refresh", s)
		s = h.Apply("update: change description and name", s, with(presetFullConfig, j{
			"name":        "main-updated",
			"description": "changed",
		}))
		h.Destroy("destroy", s)
	},

	"unset-attachment-and-payload": func(h *grHarness) {
		s := h.Apply("create", h.null(), presetFullConfig)
		unset := presetConfig("main", j{
			"description": "primary preset",
			"config_overrides": l{
				presetOverride("", l{
					j{"field_name": "title", "template": "{{alertDef.name}}"},
					j{"field_name": "body", "template": "{{alert.status}}"},
				}),
				j{
					"condition_type": j{"match_entity_type_and_sub_type": j{"entity_sub_type": "logsImmediateResolved"}},
					"message_config": j{"fields": l{
						j{"field_name": "description", "template": "{{alertDef.description}}"},
					}},
				},
			},
		})
		s = h.Apply("update: omit attachment_config and payload_type", s, unset)
		h.Refresh("refresh", s)
	},

	"client-set-id": func(h *grHarness) {
		s := h.Apply("create with id", h.null(), presetConfig("mine", j{"id": "my-preset"}))
		s = h.Apply("update", s, presetConfig("mine", j{"id": "my-preset", "description": "changed"}))
		h.Import("import", "my-preset")
		h.Destroy("destroy", s)
	},

	"id-requires-replace": func(h *grHarness) {
		s := h.Apply("create", h.null(), presetConfig("r", j{"id": "first"}))
		h.Apply("update: change id", s, presetConfig("r", j{"id": "second"}))
	},

	"config-validation": func(h *grHarness) {
		h.Validate("missing name", j{"entity_type": "alerts", "connector_type": "generic_https", "parent_id": presetSystemParent})
		h.Validate("missing entity_type", j{"name": "n", "connector_type": "generic_https", "parent_id": presetSystemParent})
		h.Validate("connector_type ibm_event_notifications", presetConfig("v", j{"connector_type": "ibm_event_notifications"}))
		h.Validate("connector_type GENERIC_HTTPS in upper case", presetConfig("v", j{"connector_type": "GENERIC_HTTPS"}))
		h.Validate("entity_type olly_scheduled_tasks", presetConfig("v", j{"entity_type": "olly_scheduled_tasks"}))
		h.Validate("attachment_config enabled in lower case", presetConfig("v", j{"attachment_config": "enabled"}))
	},

	"import": func(h *grHarness) {
		id := h.presets().seed(j{
			"name": "seeded", "entityType": "ALERTS", "connectorType": "GENERIC_HTTPS",
			"parentId": presetSystemParent, "description": "made outside Terraform",
			"attachmentConfig": j{"policy": "DISABLED"},
		})
		s := h.Import("import", id)
		h.Apply("plan the matching config", s, presetConfig("seeded", j{
			"description":       "made outside Terraform",
			"attachment_config": "DISABLED",
		}))
	},

	"import-unknown-id": func(h *grHarness) {
		h.Import("import an id that does not exist", "does-not-exist")
	},

	"read-not-found": func(h *grHarness) {
		s := h.Apply("create", h.null(), presetConfig("gone"))
		h.presets().forget(idOf(s))
		h.Refresh("refresh: preset was deleted outside Terraform", s)
	},

	"delete-not-found": func(h *grHarness) {
		s := h.Apply("create", h.null(), presetConfig("gone"))
		h.presets().forget(idOf(s))
		h.presets().deleteStatus = 404
		h.Destroy("destroy: the API answers 404", s)
	},

	"update-not-found": func(h *grHarness) {
		s := h.Apply("create", h.null(), presetConfig("gone"))
		h.presets().forget(idOf(s))
		h.Apply("update: preset was deleted outside Terraform", s, presetConfig("gone", j{"description": "changed"}))
	},
}
