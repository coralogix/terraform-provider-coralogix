// Copyright 2026 Coralogix Ltd.
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
	"testing"
)

func connectorTemplateConfig(overrides []any, wo, versions j) j {
	out := j{"name": "tf template connector", "type": "generic_https", "config_overrides": overrides}
	if wo != nil {
		out["config_overrides_templates_wo"] = wo
		out["config_overrides_templates_wo_versions"] = versions
	}
	return out
}

func alertsOverride(fields ...any) j {
	if fields == nil {
		fields = []any{}
	}
	return j{"entity_type": "alerts", "fields": fields}
}

// A write-only template must reach the API and stay out of state. The API returns every template
// on a read, so the read must not copy it into config_overrides, or the next plan shows a diff.
func TestConnectorWriteOnlyTemplates(t *testing.T) {
	h := newConnectorHarness(t)
	secret, rotated := randomSecret(t), randomSecret(t)
	plainField := j{"field_name": "title", "template": "plain title"}
	overrides := []any{alertsOverride(plainField)}

	state := h.Apply("create", h.null(), connectorTemplateConfig(overrides, j{"alerts.body": secret}, j{"alerts.body": float64(1)}))
	step := lastStep(t, h)
	requireReplanEmpty(t, step, "create")
	sent := sentOverrideTemplates(t, step.Requests)
	if sent["ALERTS.body"] != secret || sent["ALERTS.title"] != "plain title" {
		t.Fatalf("the API got %v, want the write-only template and the plain one", sent)
	}
	attrs, _ := toJSON(state).(map[string]any)
	if attrs["config_overrides_templates_wo"] != nil || attrs["config_overrides_templates_wo_versions"] == nil {
		t.Fatalf("state: wo = %v, versions = %v", attrs["config_overrides_templates_wo"], attrs["config_overrides_templates_wo_versions"])
	}
	requireNoSecret(t, state, secret)

	// The read returns the template. The state must not take it.
	state = h.RefreshPlan("refresh", state)
	step = lastStep(t, h)
	requireReplanEmpty(t, step, "refresh")
	if len(step.Drift) != 0 {
		t.Fatalf("refresh drift: %v", step.Drift)
	}

	h.Apply("same", state, connectorTemplateConfig(overrides, j{"alerts.body": secret}, j{"alerts.body": float64(1)}))
	if !lastStep(t, h).NoChange {
		t.Fatal("an unchanged config plans a change")
	}

	// A new version alone makes an Update, and sends the new value.
	state = h.Apply("rotate", state, connectorTemplateConfig(overrides, j{"alerts.body": rotated}, j{"alerts.body": float64(2)}))
	step = lastStep(t, h)
	requireReplanEmpty(t, step, "rotation")
	if got := sentOverrideTemplates(t, step.Requests)["ALERTS.body"]; got != rotated {
		t.Fatalf("the API got %q after rotation, want the new value", got)
	}
	requireNoSecret(t, state, rotated, secret)
}

// Overrides without parameters_wo keep working: a plain template stays in state.
func TestConnectorPlainTemplatesStillWork(t *testing.T) {
	h := newConnectorHarness(t)
	overrides := []any{alertsOverride(j{"field_name": "title", "template": "plain title"})}
	state := h.Apply("create", h.null(), connectorTemplateConfig(overrides, nil, nil))
	if step := lastStep(t, h); step.ReplanEmpty == nil || !*step.ReplanEmpty {
		t.Fatalf("plan after create is not empty: %v", step.ReplanDiff)
	}
	h.RefreshPlan("refresh", state)
	if step := lastStep(t, h); step.ReplanEmpty == nil || !*step.ReplanEmpty {
		t.Fatalf("plan after refresh is not empty: %v", step.ReplanDiff)
	}
}

// The rules of the write-only template maps, at validation.
func TestConnectorWriteOnlyTemplateValidation(t *testing.T) {
	secret := randomSecret(t)
	one := j{"alerts.body": float64(1)}
	declared := []any{alertsOverride()}
	for name, cfg := range map[string]j{
		"key is not entity.field":     connectorTemplateConfig(declared, j{"body": secret}, j{"body": float64(1)}),
		"unknown entity type":         connectorTemplateConfig(declared, j{"unknown.body": secret}, j{"unknown.body": float64(1)}),
		"no version":                  connectorTemplateConfig(declared, j{"alerts.body": secret}, j{}),
		"version without a value":     connectorTemplateConfig(declared, j{}, one),
		"entity type is not declared": connectorTemplateConfig([]any{j{"entity_type": "cases", "fields": []any{}}}, j{"alerts.body": secret}, one),
		"no overrides at all":         connectorTemplateConfig(nil, j{"alerts.body": secret}, one),
		"template also set in fields": connectorTemplateConfig([]any{alertsOverride(j{"field_name": "body", "template": "x"})}, j{"alerts.body": secret}, one),
	} {
		h := newConnectorHarness(t)
		h.Validate(name, cfg)
		if len(h.steps[len(h.steps)-1].ValidateDiags) == 0 {
			t.Errorf("%s: want a validation error", name)
		}
	}
}

// An override with only `fields = []` and a write-only template works: the plain list stays empty
// in state, and the template is in the request only.
func TestConnectorWriteOnlyTemplateWithEmptyFields(t *testing.T) {
	h := newConnectorHarness(t)
	secret := randomSecret(t)
	config := connectorTemplateConfig([]any{alertsOverride()}, j{"alerts.body": secret}, j{"alerts.body": float64(1)})

	state := h.Apply("create", h.null(), config)
	step := lastStep(t, h)
	requireReplanEmpty(t, step, "create")
	if got := sentOverrideTemplates(t, step.Requests)["ALERTS.body"]; got != secret {
		t.Fatalf("the API got %q, want the write-only template", got)
	}
	requireNoSecret(t, state, secret)
	h.RefreshPlan("refresh", state)
	requireReplanEmpty(t, lastStep(t, h), "refresh")
}

// A field name may hold dots. The key is split at the first dot only.
func TestConnectorWriteOnlyTemplateWithDottedFieldName(t *testing.T) {
	h := newConnectorHarness(t)
	secret := randomSecret(t)
	config := connectorTemplateConfig([]any{alertsOverride()}, j{"alerts.a.b": secret}, j{"alerts.a.b": float64(1)})

	state := h.Apply("create", h.null(), config)
	step := lastStep(t, h)
	requireReplanEmpty(t, step, "create")
	if got := sentOverrideTemplates(t, step.Requests)["ALERTS.a.b"]; got != secret {
		t.Fatalf("the API got %v, want the template under the field a.b", sentOverrideTemplates(t, step.Requests))
	}
	requireNoSecret(t, state, secret)
	state = h.RefreshPlan("refresh", state)
	requireReplanEmpty(t, lastStep(t, h), "refresh")
	requireNoSecret(t, state, secret)
}

// An edit of another attribute sends the whole connector again. The write-only template must be in
// that request, or the API would lose it, and the state must still not hold it.
func TestConnectorEditKeepsWriteOnlyTemplate(t *testing.T) {
	h := newConnectorHarness(t)
	secret := randomSecret(t)
	wo, versions := j{"alerts.body": secret}, j{"alerts.body": float64(1)}
	overrides := []any{alertsOverride(j{"field_name": "title", "template": "plain title"})}

	state := h.Apply("create", h.null(), connectorTemplateConfig(overrides, wo, versions))
	lastStep(t, h)

	renamed := connectorTemplateConfig(overrides, wo, versions)
	renamed["name"] = "tf template connector renamed"
	state = h.Apply("edit", state, renamed)
	step := lastStep(t, h)
	if step.NoChange {
		t.Fatal("editing the name planned no change")
	}
	if got := sentOverrideTemplates(t, step.Requests)["ALERTS.body"]; got != secret {
		t.Fatalf("the API got %q after an edit, want the write-only template", got)
	}
	requireReplanEmpty(t, step, "the edit")
	requireNoSecret(t, state, secret)
}
