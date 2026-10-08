// Copyright 2024 Coralogix Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an AS IS BASIS,
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
	"github.com/coralogix/terraform-provider-coralogix/internal/utils/schemadump"
)

const cxGoldenDir = "testdata/connector_baseline"

// These tests record the behavior of the released handwritten coralogix_connector.
// Record the goldens on master first. The generated-resource switch then shows
// the real protocol diff in these files.
//
//	UPDATE_GOLDEN=1 go test ./internal/provider -run TestConnectorBaseline

func TestConnectorBaselineSchema(t *testing.T) {
	var current resource.SchemaResponse
	notifications.NewConnectorResource().Schema(context.Background(), resource.SchemaRequest{}, &current)
	checkGoldenAt(t, cxGoldenDir, "schema_current.txt", []byte(schemadump.Text(current.Schema)))
}

func TestConnectorBaselineScenarios(t *testing.T) {
	for name, run := range cxScenarios {
		t.Run(name, func(t *testing.T) {
			h := newCXHarness(t)
			run(h)
			checkGoldenAt(t, cxGoldenDir, "scenario_"+name+".json", marshalGolden(t, h.steps))
		})
	}
}

func httpsFields(url string) j {
	return j{"fields": l{
		j{"field_name": "method", "value": "post"},
		j{"field_name": "url", "value": url},
	}}
}

func httpsConnector(name string, extra ...j) j {
	out := j{
		"name":             name,
		"type":             "generic_https",
		"connector_config": httpsFields("https://example.com/hook"),
	}
	for _, e := range extra {
		maps.Copy(out, e)
	}
	return out
}

var cxFullConfig = httpsConnector("main", j{
	"description": "primary connector",
	"config_overrides": l{
		j{
			"entity_type": "alerts",
			"fields":      l{j{"field_name": "url", "template": "https://example.com/alerts"}},
		},
	},
})

var cxScenarios = map[string]func(h *grHarness){
	"minimal": func(h *grHarness) {
		cfg := httpsConnector("minimal")
		s := h.Apply("create", h.null(), cfg)
		s = h.Refresh("refresh", s)
		h.Destroy("destroy", s)
	},

	"full-lifecycle": func(h *grHarness) {
		s := h.Apply("create", h.null(), cxFullConfig)
		s = h.Refresh("refresh", s)
		s = h.Apply("update: change description and name", s, with(cxFullConfig, j{
			"name":        "main-updated",
			"description": "changed",
		}))
		h.Destroy("destroy", s)
	},

	"unset-optional-attributes": func(h *grHarness) {
		s := h.Apply("create", h.null(), cxFullConfig)
		h.Apply("update: only name, type, and connector_config are left", s, httpsConnector("main"))
		h.Refresh("refresh", s)
	},

	"client-set-id": func(h *grHarness) {
		s := h.Apply("create with id", h.null(), httpsConnector("mine", j{"id": "my-connector"}))
		s = h.Apply("update", s, httpsConnector("mine", j{"id": "my-connector", "description": "changed"}))
		h.Import("import", "my-connector")
		h.Destroy("destroy", s)
	},

	"id-requires-replace": func(h *grHarness) {
		s := h.Apply("create", h.null(), httpsConnector("r", j{"id": "first"}))
		h.Apply("update: change id", s, httpsConnector("r", j{"id": "second"}))
	},

	"config-validation": func(h *grHarness) {
		h.Validate("missing name", j{"type": "generic_https"})
		h.Validate("missing type", j{"name": "n"})
		h.Validate("type ibm_event_notifications", httpsConnector("v", j{"type": "ibm_event_notifications"}))
		h.Validate("type GENERIC_HTTPS in upper case", httpsConnector("v", j{"type": "GENERIC_HTTPS"}))
		h.Validate("entity_type olly_scheduled_tasks", httpsConnector("v", j{"config_overrides": l{
			j{"entity_type": "olly_scheduled_tasks", "fields": l{j{"field_name": "url", "template": "https://example.com"}}},
		}}))
	},

	"import": func(h *grHarness) {
		id := h.connectors().seed(j{
			"name": "seeded", "type": "GENERIC_HTTPS", "description": "made outside Terraform",
			"connectorConfig": j{"fields": l{
				j{"fieldName": "method", "value": "post"},
				j{"fieldName": "url", "value": "https://example.com/hook"},
			}},
		})
		s := h.Import("import", id)
		h.Apply("plan the matching config", s, httpsConnector("seeded", j{"description": "made outside Terraform"}))
	},

	"import-unknown-id": func(h *grHarness) {
		h.Import("import an id that does not exist", "does-not-exist")
	},

	"read-not-found": func(h *grHarness) {
		s := h.Apply("create", h.null(), httpsConnector("gone"))
		h.connectors().forget(idOf(s))
		h.Refresh("refresh: connector was deleted outside Terraform", s)
	},

	"delete-not-found": func(h *grHarness) {
		s := h.Apply("create", h.null(), httpsConnector("gone"))
		h.connectors().forget(idOf(s))
		h.connectors().deleteStatus = 404
		h.Destroy("destroy: the API answers 404", s)
	},

	"update-not-found": func(h *grHarness) {
		s := h.Apply("create", h.null(), httpsConnector("gone"))
		h.connectors().forget(idOf(s))
		h.Apply("update: connector was deleted outside Terraform", s, httpsConnector("gone", j{"description": "changed"}))
	},

	"write-only-secret": func(h *grHarness) {
		cfg := httpsConnector("wo", j{
			"connector_config": j{
				"fields":                   l{j{"field_name": "url", "value": "https://example.com/hook"}},
				"field_values_wo":          j{"token": "secret-value"},
				"field_values_wo_versions": j{"token": float64(1)},
			},
		})
		s := h.Apply("create with a write-only field", h.null(), cfg)
		s = h.Refresh("refresh", s)
		h.Destroy("destroy", s)
	},
}
