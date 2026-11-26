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
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// webhookSecretURL builds a URL with a random path at run time, so no secret-like text is in the source.
func webhookSecretURL(t *testing.T) string {
	t.Helper()
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		t.Fatal(err)
	}
	return "https://example.com/hook/" + hex.EncodeToString(raw)
}

// webhookArms are the arms that take url_wo. Each entry holds the arm, the Terraform name of its
// type in the API body, and the attributes that the arm needs besides the URL.
var webhookArms = []struct {
	arm    string
	apiArm string
	extra  j
	// edit changes an attribute of the arm that is not a URL. Opsgenie has none.
	edit j
}{
	{arm: "custom", apiArm: "genericWebhook", extra: j{"method": "post"}, edit: j{"method": "put"}},
	{arm: "slack", apiArm: "slack", extra: j{}, edit: j{"notify_on": []any{"data_usage"}}},
	{arm: "opsgenie", apiArm: "opsgenie", extra: j{}, edit: j{}},
	{arm: "demisto", apiArm: "demisto", extra: j{}, edit: j{"payload": "{}"}},
}

func webhookConfig(name, arm string, body j) j {
	return j{"name": name, arm: body}
}

// requestURL returns the url that the last write request of a step sent.
func requestURL(t *testing.T, step grStep) string {
	t.Helper()
	for i := len(step.Requests) - 1; i >= 0; i-- {
		r := step.Requests[i]
		if r.Method != "POST" && r.Method != "PUT" {
			continue
		}
		body, _ := r.Body.(map[string]any)
		data, _ := body["data"].(map[string]any)
		got, _ := data["url"].(string)
		return got
	}
	t.Fatalf("no write request in %v", step.Requests)
	return ""
}

func armState(t *testing.T, state tftypes.Value, arm string) map[string]any {
	t.Helper()
	attrs, _ := toJSON(state).(map[string]any)
	got, _ := attrs[arm].(map[string]any)
	if got == nil {
		t.Fatalf("state has no %s: %v", arm, attrs)
	}
	return got
}

// A write-only URL must reach the API and stay out of state. A read returns the URL, so the read
// must not copy it back, or the next plan shows a permanent diff.
func TestWebhookWriteOnlyURL(t *testing.T) {
	for _, tc := range webhookArms {
		t.Run(tc.arm, func(t *testing.T) {
			h := newWebhookHarness(t)
			secret, rotated := webhookSecretURL(t), webhookSecretURL(t)
			body := func(url string, version float64) j {
				return webhookConfig("tf wo url", tc.arm, with(tc.extra, j{"url_wo": url, "url_wo_version": version}))
			}

			state := h.Apply("create", h.null(), body(secret, 1))
			step := lastStep(t, h)
			requireReplanEmpty(t, step, "create")
			requireSentURL(t, step, secret)
			requireWriteOnlyArm(t, state, tc.arm)
			requireNoSecret(t, state, secret)

			// A refresh reads the URL from the API. The state must not take it.
			state = h.RefreshPlan("refresh", state)
			step = lastStep(t, h)
			requireReplanEmpty(t, step, "refresh")
			if len(step.Drift) != 0 {
				t.Fatalf("refresh drift: %v", step.Drift)
			}

			// The same version sends nothing, because Terraform sees no change.
			h.Apply("same", state, body(secret, 1))
			if !lastStep(t, h).NoChange {
				t.Fatalf("an unchanged config plans a change")
			}

			// A new version sends the new value.
			state = h.Apply("rotate", state, body(rotated, 2))
			step = lastStep(t, h)
			requireReplanEmpty(t, step, "rotation")
			requireSentURL(t, step, rotated)
			requireNoSecret(t, state, rotated, secret)
		})
	}
}

func requireSentURL(t *testing.T, step grStep, want string) {
	t.Helper()
	if got := requestURL(t, step); got != want {
		t.Fatalf("the API got url %q, want %q", got, want)
	}
}

// requireWriteOnlyArm fails unless the arm keeps url and url_wo null and holds a version.
func requireWriteOnlyArm(t *testing.T, state tftypes.Value, name string) {
	t.Helper()
	arm := armState(t, state, name)
	if arm["url"] != nil || arm["url_wo"] != nil || arm["url_wo_version"] == nil {
		t.Fatalf("state of %s: url = %v, url_wo = %v, url_wo_version = %v; want url and url_wo null and a version", name, arm["url"], arm["url_wo"], arm["url_wo_version"])
	}
}

// A config that sets the plain url keeps working, and its url stays in state.
func TestWebhookPlainURLStillWorks(t *testing.T) {
	for _, tc := range webhookArms {
		t.Run(tc.arm, func(t *testing.T) {
			h := newWebhookHarness(t)
			plain := "https://example.com/plain"
			state := h.Apply("create", h.null(), webhookConfig("tf plain url", tc.arm, with(tc.extra, j{"url": plain})))
			step := lastStep(t, h)
			if len(step.ApplyDiags) != 0 || len(step.Inconsistent) != 0 || len(step.ValidateDiags) != 0 {
				t.Fatalf("create: validate %v, apply %v, inconsistent %v", step.ValidateDiags, step.ApplyDiags, step.Inconsistent)
			}
			if got := requestURL(t, step); got != plain {
				t.Fatalf("the API got url %q, want %q", got, plain)
			}
			if step.ReplanEmpty == nil || !*step.ReplanEmpty {
				t.Fatalf("plan after create is not empty: %v", step.ReplanDiff)
			}
			if got := armState(t, state, tc.arm)["url"]; got != plain {
				t.Fatalf("state url = %v, want %q", got, plain)
			}
			h.RefreshPlan("refresh", state)
			if step = lastStep(t, h); step.ReplanEmpty == nil || !*step.ReplanEmpty {
				t.Fatalf("plan after refresh is not empty: %v", step.ReplanDiff)
			}
		})
	}
}

// The rules of url and url_wo, at validation.
func TestWebhookURLValidation(t *testing.T) {
	secret := webhookSecretURL(t)
	plain := "https://example.com/plain"
	for _, tc := range webhookArms {
		both := with(tc.extra, j{"url": plain, "url_wo": secret, "url_wo_version": float64(1)})
		noVersion := with(tc.extra, j{"url_wo": secret})
		versionOnly := with(tc.extra, j{"url_wo_version": float64(1)})
		h := newWebhookHarness(t)
		for name, body := range map[string]j{"both url and url_wo": both, "url_wo without version": noVersion, "version without url_wo": versionOnly} {
			h.Validate(tc.arm+": "+name, webhookConfig("tf invalid", tc.arm, body))
			if len(h.steps[len(h.steps)-1].ValidateDiags) == 0 {
				t.Errorf("%s with %s: want a validation error", tc.arm, name)
			}
		}
	}

	// A URL is required in the arms that required it before: exactly one of url and url_wo.
	for _, arm := range []string{"opsgenie", "demisto"} {
		h := newWebhookHarness(t)
		h.Validate(arm+": no url", webhookConfig("tf invalid", arm, j{}))
		if len(h.steps[len(h.steps)-1].ValidateDiags) == 0 {
			t.Errorf("%s with no url and no url_wo: want a validation error", arm)
		}
	}
}

// An edit of another attribute sends the whole webhook again. The write-only URL must be in that
// request, or the API would lose it, and the state must still not hold it.
func TestWebhookEditKeepsWriteOnlyURL(t *testing.T) {
	for _, tc := range webhookArms {
		t.Run(tc.arm, func(t *testing.T) {
			h := newWebhookHarness(t)
			secret := webhookSecretURL(t)
			config := func(name string, edit j) j {
				body := with(with(tc.extra, edit), j{"url_wo": secret, "url_wo_version": float64(1)})
				return webhookConfig(name, tc.arm, body)
			}
			state := h.Apply("create", h.null(), config("tf edit", nil))
			lastStep(t, h)

			edits := map[string]j{"name": nil}
			if len(tc.edit) > 0 {
				edits["arm attribute"] = tc.edit
			}
			for what, edit := range edits {
				name := "tf edit"
				if what == "name" {
					name = "tf edit renamed"
				}
				state = h.Apply("edit "+what, state, config(name, edit))
				step := lastStep(t, h)
				if step.NoChange {
					t.Fatalf("editing %s planned no change", what)
				}
				requireSentURL(t, step, secret)
				requireReplanEmpty(t, step, "editing "+what)
				requireNoSecret(t, state, secret)
			}
		})
	}
}

// Moving the URL to url_wo, and back, leaves an empty plan each time, and the secret is not in state.
func TestWebhookMoveURLBetweenPlainAndWriteOnly(t *testing.T) {
	for _, tc := range webhookArms {
		t.Run(tc.arm, func(t *testing.T) {
			h := newWebhookHarness(t)
			secret := webhookSecretURL(t)
			plain := "https://example.com/plain"
			withPlain := webhookConfig("tf move", tc.arm, with(tc.extra, j{"url": plain}))
			withWriteOnly := webhookConfig("tf move", tc.arm, with(tc.extra, j{"url_wo": secret, "url_wo_version": float64(1)}))

			state := h.Apply("plain", h.null(), withPlain)
			requireReplanEmpty(t, lastStep(t, h), "create")

			state = h.Apply("to write-only", state, withWriteOnly)
			step := lastStep(t, h)
			requireReplanEmpty(t, step, "the move to url_wo")
			requireSentURL(t, step, secret)
			requireWriteOnlyArm(t, state, tc.arm)
			requireNoSecret(t, state, secret)
			state = h.RefreshPlan("refresh", state)
			requireReplanEmpty(t, lastStep(t, h), "refresh")

			state = h.Apply("back to plain", state, withPlain)
			step = lastStep(t, h)
			requireReplanEmpty(t, step, "the move back to url")
			requireSentURL(t, step, plain)
			arm := armState(t, state, tc.arm)
			if arm["url"] != plain || arm["url_wo_version"] != nil {
				t.Fatalf("state of %s: url = %v, url_wo_version = %v; want the plain url and no version", tc.arm, arm["url"], arm["url_wo_version"])
			}
			requireNoSecret(t, state, secret)
		})
	}
}

// The Microsoft Teams arm is deprecated. The released resource, and this one, refuse to create it:
// webhookBeforeWrite returns "Unknown webhook type" for a Create with the arm. Only a webhook that
// exists already, made by the UI or an older provider, takes an Update. url_wo works there, and the
// read does not copy the URL into state.
func TestWebhookMicrosoftTeamsWriteOnlyURL(t *testing.T) {
	h := newWebhookHarness(t)
	secret := webhookSecretURL(t)
	config := j{"name": "legacy teams", "microsoft_teams": j{"url_wo": secret, "url_wo_version": float64(1)}}

	h.Apply("create", h.null(), config)
	if diags := h.steps[len(h.steps)-1].ApplyDiags; len(diags) != 1 || !strings.Contains(diags[0], "Unknown webhook type") {
		t.Fatalf("create diagnostics = %v, want the legacy Unknown webhook type error", diags)
	}

	h.api.(*webhookFake).seedLegacyTeams("legacy-1", "legacy teams", "https://example.com/old")
	state := h.Import("import", "legacy-1")
	state = h.Apply("move to write-only", state, config)
	// The deprecation warning of the arm is a diagnostic, so lastStep would stop on it.
	step := h.steps[len(h.steps)-1]
	if len(step.ApplyDiags) != 0 || len(step.PlanDiags) != 0 || len(step.Inconsistent) != 0 {
		t.Fatalf("update: plan %v, apply %v, inconsistent %v", step.PlanDiags, step.ApplyDiags, step.Inconsistent)
	}
	requireSentURL(t, step, secret)
	requireReplanEmpty(t, step, "the move to url_wo")
	requireWriteOnlyArm(t, state, "microsoft_teams")
	requireNoSecret(t, state, secret)
}
