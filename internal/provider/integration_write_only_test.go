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
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// randomSecret builds a value at run time, so no secret-like text is in the source.
func randomSecret(t *testing.T) string {
	t.Helper()
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(raw)
}

// requireReplanEmpty fails when the plan after a step is not empty.
func requireReplanEmpty(t *testing.T, step grStep, when string) {
	t.Helper()
	if step.ReplanEmpty == nil || !*step.ReplanEmpty {
		t.Fatalf("plan after %s is not empty: %v", when, step.ReplanDiff)
	}
}

// requireNoSecret fails when the state holds one of the secrets.
func requireNoSecret(t *testing.T, state tftypes.Value, secrets ...string) {
	t.Helper()
	raw, _ := json.Marshal(toJSON(state))
	for _, secret := range secrets {
		if strings.Contains(string(raw), secret) {
			t.Fatalf("a secret is in state: %s", raw)
		}
	}
}

func integrationConfig(parameters, wo, versions j) j {
	out := j{"integration_key": "example-integration", "version": "1.0.0", "parameters": parameters}
	if wo != nil {
		out["parameters_wo"] = wo
		out["parameters_wo_versions"] = versions
	}
	return out
}

// A write-only parameter must reach the API as a string parameter and stay out of state. The
// API holds it as an api key and returns it on a read, so the read must not copy it into
// `parameters`, or the next plan shows a permanent diff.
func TestIntegrationWriteOnlyParameters(t *testing.T) {
	h := newIntegrationHarness(t)
	secret, rotated := randomSecret(t), randomSecret(t)
	plain := j{"IntegrationName": "example", "Region": "eu-west-1"}

	state := h.Apply("create", h.null(), integrationConfig(plain, j{"ApiKey": secret}, j{"ApiKey": float64(1)}))
	step := lastStep(t, h)
	requireReplanEmpty(t, step, "create")
	sent := sentParameters(t, step.Requests)
	if got, _ := sent["ApiKey"]["stringValue"].(string); got != secret {
		t.Fatalf("the API got %v for ApiKey, want the write-only value as a string parameter", sent["ApiKey"])
	}
	if got, _ := sent["Region"]["stringValue"].(string); got != "eu-west-1" {
		t.Fatalf("the API got %v for Region", sent["Region"])
	}
	attrs, _ := toJSON(state).(map[string]any)
	if attrs["parameters_wo"] != nil || attrs["parameters_wo_versions"] == nil {
		t.Fatalf("state: parameters_wo = %v, parameters_wo_versions = %v", attrs["parameters_wo"], attrs["parameters_wo_versions"])
	}
	requireNoSecret(t, state, secret)

	// The read returns ApiKey. The state must not take it.
	state = h.RefreshPlan("refresh", state)
	step = lastStep(t, h)
	requireReplanEmpty(t, step, "refresh")
	if len(step.Drift) != 0 {
		t.Fatalf("refresh drift: %v", step.Drift)
	}

	h.Apply("same", state, integrationConfig(plain, j{"ApiKey": secret}, j{"ApiKey": float64(1)}))
	if !lastStep(t, h).NoChange {
		t.Fatal("an unchanged config plans a change")
	}

	// A new version alone makes an Update, and sends the new value.
	state = h.Apply("rotate", state, integrationConfig(plain, j{"ApiKey": rotated}, j{"ApiKey": float64(2)}))
	step = lastStep(t, h)
	requireReplanEmpty(t, step, "rotation")
	if got, _ := sentParameters(t, step.Requests)["ApiKey"]["stringValue"].(string); got != rotated {
		t.Fatalf("the API got %v after rotation, want the new value", sentParameters(t, step.Requests)["ApiKey"])
	}
	requireNoSecret(t, state, rotated, secret)

	// Moving the key back to `parameters` stops the write-only use.
	state = h.Apply("move back", state, integrationConfig(with(plain, j{"ApiKey": "plain-example"}), nil, nil))
	if sent := sentParameters(t, lastStep(t, h).Requests); sent["ApiKey"]["stringValue"] != "plain-example" {
		t.Fatalf("the API got %v", sent["ApiKey"])
	}
	_ = state
}

// An import has no configuration, so the state takes every parameter of the API, an api key too.
// One apply with parameters_wo then removes it from state.
func TestIntegrationWriteOnlyAfterImport(t *testing.T) {
	h := newIntegrationHarness(t)
	secret := randomSecret(t)
	plain := j{"IntegrationName": "example"}
	created := h.Apply("create", h.null(), integrationConfig(with(plain, j{"ApiKey": secret}), nil, nil))
	lastStep(t, h)

	imported := h.Import("import", toJSON(created).(map[string]any)["id"].(string))
	if raw, _ := json.Marshal(toJSON(imported)); !strings.Contains(string(raw), secret) {
		t.Fatalf("the import does not show the api key: %s", raw)
	}

	state := h.Apply("move to write-only", imported, integrationConfig(plain, j{"ApiKey": secret}, j{"ApiKey": float64(1)}))
	step := lastStep(t, h)
	if step.ReplanEmpty == nil || !*step.ReplanEmpty {
		t.Fatalf("plan after the move is not empty: %v", step.ReplanDiff)
	}
	if raw, _ := json.Marshal(toJSON(state)); strings.Contains(string(raw), secret) {
		t.Fatalf("the secret is still in state: %s", raw)
	}
}

// A config that does not use parameters_wo keeps working.
func TestIntegrationParametersWithoutWriteOnly(t *testing.T) {
	h := newIntegrationHarness(t)
	state := h.Apply("create", h.null(), integrationConfig(j{"IntegrationName": "example"}, nil, nil))
	step := lastStep(t, h)
	if step.ReplanEmpty == nil || !*step.ReplanEmpty {
		t.Fatalf("plan after create is not empty: %v", step.ReplanDiff)
	}
	h.RefreshPlan("refresh", state)
	if step = lastStep(t, h); step.ReplanEmpty == nil || !*step.ReplanEmpty {
		t.Fatalf("plan after refresh is not empty: %v", step.ReplanDiff)
	}
}

// The rules of parameters and parameters_wo, at validation.
func TestIntegrationWriteOnlyValidation(t *testing.T) {
	secret := randomSecret(t)
	for name, cfg := range map[string]j{
		"key in both":            integrationConfig(j{"ApiKey": "x"}, j{"ApiKey": secret}, j{"ApiKey": float64(1)}),
		"value without version":  integrationConfig(j{}, j{"ApiKey": secret}, j{}),
		"version without value":  integrationConfig(j{}, j{}, j{"ApiKey": float64(1)}),
		"version of another key": integrationConfig(j{}, j{"ApiKey": secret}, j{"Other": float64(1)}),
	} {
		h := newIntegrationHarness(t)
		h.Validate(name, cfg)
		if len(h.steps[len(h.steps)-1].ValidateDiags) == 0 {
			t.Errorf("%s: want a validation error", name)
		}
	}
}
