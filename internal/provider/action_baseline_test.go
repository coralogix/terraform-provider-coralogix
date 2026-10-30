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
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"

	"github.com/coralogix/terraform-provider-coralogix/internal/provider/actions"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils/schemadump"
)

const actionTypeName = "coralogix_action"

// These tests record the behavior of the handwritten coralogix_action. After the generator
// switch, the golden diff must show only what the generated resource changes on purpose.
//
//	UPDATE_GOLDEN=1 go test ./internal/provider -run TestActionBaseline

func newActionHarness(t *testing.T) *grHarness {
	return newProtocolHarness(t, actionTypeName, newActionFake(t))
}

func (h *grHarness) actionAPI() *actionFake {
	h.t.Helper()
	f, ok := h.api.(*actionFake)
	if !ok {
		h.t.Fatal("expected *actionFake")
	}
	return f
}

// actionDataSourceScenarios are the scenarios of the data source. It finds an action by id only. A
// reading by id of an action that has no optional values shows how the data source reads an API
// [] and "".
var actionDataSourceScenarios = func() map[string]func(h *grHarness) {
	out := idLookupScenarios(func(h *grHarness) string {
		return h.actionAPI().seed(map[string]any{
			"name": "seeded", "url": "https://example.com/hook", "sourceType": "SOURCE_TYPE_LOG",
			"isPrivate": false, "description": "made outside Terraform",
			"dpxlFilter":       "<v1> $d.severity == 'ERROR'",
			"applicationNames": []any{"app"}, "subsystemNames": []any{"sub"},
			"urlFields": []any{map[string]any{"name": "field", "required": true}},
		})
	})
	out["read-minimal"] = func(h *grHarness) {
		id := h.actionAPI().seed(map[string]any{
			"name": "bare", "url": "https://example.com/hook", "sourceType": "SOURCE_TYPE_LOG", "isPrivate": false,
		})
		h.ReadDataSource("read an action with no optional values", byID(id))
	}
	out["read-empty-strings"] = func(h *grHarness) {
		id := h.actionAPI().seed(map[string]any{
			"name": "empty", "url": "https://example.com/hook", "sourceType": "SOURCE_TYPE_LOG", "isPrivate": false,
			"description": "", "dpxlFilter": "",
		})
		h.ReadDataSource("read an action whose description and filter are empty strings", byID(id))
	}
	return out
}()

func TestActionBaselineSchema(t *testing.T) {
	var current resource.SchemaResponse
	actions.NewActionResource().Schema(context.Background(), resource.SchemaRequest{}, &current)
	checkGoldenAt(t, actionGoldenDir, "schema_current.txt", []byte(schemadump.Text(current.Schema)))
}

func TestActionBaselineScenarios(t *testing.T) {
	for name, run := range actionScenarios {
		t.Run(name, func(t *testing.T) {
			h := newActionHarness(t)
			run(h)
			checkGoldenAt(t, actionGoldenDir, "scenario_"+name+".json", marshalGolden(t, h.steps))
		})
	}
}

// actionCfg is a config that the API accepts with an API key: a private action needs a user.
func actionCfg(extra j) j {
	cfg := j{"name": "tf-action", "url": "https://example.com/{{p}}", "source_type": "Log", "is_private": false}
	for k, v := range extra {
		if v == nil {
			delete(cfg, k)
			continue
		}
		cfg[k] = v
	}
	return cfg
}

var actionScenarios = map[string]func(h *grHarness){
	"minimal": func(h *grHarness) {
		s := h.Apply("create", h.null(), actionCfg(nil))
		s = h.Refresh("refresh", s)
		h.Destroy("destroy", s)
	},

	"full-lifecycle": func(h *grHarness) {
		full := actionCfg(j{
			"description":  "first",
			"dpxl_filter":  "<v1> $d.severity == 'ERROR'",
			"url_fields":   []any{j{"name": "p", "required": true}, j{"name": "q", "required": false}},
			"applications": []any{"app-a", "app-b"},
			"subsystems":   []any{"sub-a"},
		})
		s := h.Apply("create with every field", h.null(), full)
		s = h.Refresh("refresh", s)
		s = h.Apply("update: change every field", s, actionCfg(j{
			"name":         "tf-action-2",
			"url":          "https://example.org/",
			"source_type":  "DataMap",
			"description":  "second",
			"dpxl_filter":  "<v1> $d.severity == 'INFO'",
			"url_fields":   []any{j{"name": "p", "required": false}},
			"applications": []any{"app-a"},
			"subsystems":   []any{"sub-b"},
		}))
		s = h.Apply("update: remove every optional field", s, actionCfg(j{"name": "tf-action-2", "source_type": "DataMap"}))
		s = h.Refresh("refresh", s)
		s = h.Apply("update: set them again", s, full)
		h.Refresh("refresh", s)
		h.Destroy("destroy", s)
	},

	"is-private-default": func(h *grHarness) {
		// The released default is true, and an API key cannot own a private action.
		h.Apply("create without is_private", h.null(), actionCfg(j{"is_private": nil}))
		h.Apply("create with is_private = true", h.null(), actionCfg(j{"is_private": true}))
	},

	"is-private-change": func(h *grHarness) {
		s := h.Apply("create", h.null(), actionCfg(nil))
		h.Apply("update: is_private = true", s, actionCfg(j{"is_private": true}))
	},

	"is-hidden": func(h *grHarness) {
		// The API treats isHidden as a per-user preference and keeps its value on a replace.
		s := h.Apply("create", h.null(), actionCfg(nil))
		s = h.Apply("update: is_hidden = true", s, actionCfg(j{"is_hidden": true}))
		h.Refresh("refresh", s)
		h.Apply("create with is_hidden = true", h.null(), actionCfg(j{"name": "tf-hidden", "is_hidden": true}))
	},

	"config-validation": func(h *grHarness) {
		h.Validate("empty config", j{})
		h.Validate("empty name", actionCfg(j{"name": ""}))
		h.Validate("empty url", actionCfg(j{"url": ""}))
		h.Validate("url without a scheme", actionCfg(j{"url": "example.com"}))
		h.Validate("url with another scheme", actionCfg(j{"url": "ftp://example.com"}))
		h.Validate("source_type not in the list", actionCfg(j{"source_type": "Metric"}))
		h.Validate("dpxl_filter without the version prefix", actionCfg(j{"dpxl_filter": "$d.severity == 'ERROR'"}))
		h.Validate("empty dpxl_filter", actionCfg(j{"dpxl_filter": ""}))
		h.Validate("empty url_fields name", actionCfg(j{"url_fields": []any{j{"name": "", "required": true}}}))
		h.Validate("url_fields without required", actionCfg(j{"url_fields": []any{j{"name": "p"}}}))
		h.Validate("empty applications", actionCfg(j{"applications": []any{}}))
		h.Validate("empty subsystems", actionCfg(j{"subsystems": []any{}}))
		h.Validate("name longer than 50 characters", actionCfg(j{"name": strings.Repeat("n", 51)}))
	},

	"name-too-long": func(h *grHarness) {
		h.Apply("create with a name of 50 characters", h.null(), actionCfg(j{"name": strings.Repeat("n", 50)}))
		h.Apply("create with a name of 51 characters", h.null(), actionCfg(j{"name": strings.Repeat("n", 51)}))
	},

	"empty-optional-values": func(h *grHarness) {
		cfg := actionCfg(j{"description": "", "dpxl_filter": "", "url_fields": []any{}})
		s := h.Apply("create with empty description, dpxl_filter, and url_fields", h.null(), cfg)
		s = h.Refresh("refresh", s)
		s = h.Apply("update: remove them", s, actionCfg(nil))
		s = h.Apply("update: set them empty again", s, cfg)
		h.Refresh("refresh", s)
	},

	"dpxl-filter-canonical": func(h *grHarness) {
		// Applying the config of a filter with the prefix is stable, and the API adds the prefix to a
		// filter that has none (a config with none is refused at plan time).
		id := h.actionAPI().seed(j{"name": "seeded-filter", "url": "https://example.com/", "sourceType": "SOURCE_TYPE_LOG", "isPrivate": false, "dpxlFilter": "$d.severity == 'ERROR'"})
		s := h.Import("import an action whose filter has no prefix", id)
		h.Apply("plan the filter with the prefix", s, actionCfg(j{"name": "seeded-filter", "url": "https://example.com/", "dpxl_filter": "<v1> $d.severity == 'ERROR'"}))
	},

	"duplicate-name": func(h *grHarness) {
		h.Apply("create the first action", h.null(), actionCfg(j{"name": "first"}))
		s := h.Apply("create the second action", h.null(), actionCfg(j{"name": "second"}))
		h.Apply("create another action named first", h.null(), actionCfg(j{"name": "first"}))
		h.Apply("update: rename to the name of the first", s, actionCfg(j{"name": "first"}))
	},

	"import": func(h *grHarness) {
		id := h.actionAPI().seed(j{
			"name": "seeded", "url": "https://example.com/", "sourceType": "SOURCE_TYPE_DATA_MAP", "isPrivate": false,
			"description": "seeded", "urlFields": []any{j{"name": "p", "required": true}},
			"applicationNames": []any{"app-a"}, "subsystemNames": []any{"sub-a"},
		})
		s := h.Import("import", id)
		h.Apply("plan the matching config", s, actionCfg(j{
			"name": "seeded", "url": "https://example.com/", "source_type": "DataMap", "description": "seeded",
			"url_fields": []any{j{"name": "p", "required": true}}, "applications": []any{"app-a"}, "subsystems": []any{"sub-a"},
		}))
	},

	"import-minimal": func(h *grHarness) {
		id := h.actionAPI().seed(j{"name": "seeded", "url": "https://example.com/", "sourceType": "SOURCE_TYPE_LOG", "isPrivate": false})
		s := h.Import("import", id)
		h.Apply("plan the matching config", s, actionCfg(j{"name": "seeded", "url": "https://example.com/"}))
	},

	"import-unknown-id": func(h *grHarness) {
		h.Import("import an id that does not exist", "00000000-0000-4000-8000-00000000dead")
	},

	"read-malformed-id": func(h *grHarness) {
		h.Import("import an id that is not a UUID", "not-a-uuid")
	},

	"read-not-found": func(h *grHarness) {
		s := h.Apply("create", h.null(), actionCfg(nil))
		h.actionAPI().forget(idOf(s))
		h.Refresh("refresh: action was deleted outside Terraform", s)
	},

	"delete-not-found": func(h *grHarness) {
		s := h.Apply("create", h.null(), actionCfg(nil))
		h.actionAPI().forget(idOf(s))
		h.Destroy("destroy: the API answers 404 for an action that does not exist", s)
	},

	"update-not-found": func(h *grHarness) {
		s := h.Apply("create", h.null(), actionCfg(nil))
		h.actionAPI().forget(idOf(s))
		h.Apply("update: action was deleted outside Terraform", s, actionCfg(j{"name": "tf-action-renamed"}))
	},
}
