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
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// The policy that the backend writes for an alert that is created without one: every alert
// permission, with no rules. The text is not in the canonical form of jsonencode, as the real answer
// is not.
const alertDefaultPolicy = `{"default": {"permissions": {"alerts:ReadConfig": "grant", "alerts:UpdateConfig": "grant", "alerts:ReadAccessPolicy": "grant", "alerts:UpdateAccessPolicy": "grant"}}, "rules": []}`

// accessPolicyOf returns the access_policy attribute of a state: nil when it is null.
func accessPolicyOf(t *testing.T, state tftypes.Value) any {
	t.Helper()
	attrs, ok := toJSON(state).(map[string]any)
	if !ok {
		t.Fatalf("state is not an object: %v", toJSON(state))
	}
	return attrs["access_policy"]
}

// sameJSON reports whether two JSON texts hold the same value.
func sameJSON(t *testing.T, got any, want string) bool {
	t.Helper()
	text, ok := got.(string)
	if !ok {
		return false
	}
	var a, b any
	if err := json.Unmarshal([]byte(text), &a); err != nil {
		t.Fatalf("%q is not JSON: %v", text, err)
	}
	if err := json.Unmarshal([]byte(want), &b); err != nil {
		t.Fatal(err)
	}
	return reflect.DeepEqual(a, b)
}

// An alert created without access_policy gets the default policy of the backend. The state after
// Create and Update must hold it, as the state after a Read or an import does, or an import of the
// same alert shows a policy that the state lacks.
func TestAlertWriteReadsTheDefaultPolicyBack(t *testing.T) {
	h := newAlertHarness(t)
	h.api.(*alertFake).defaultPolicy = alertDefaultPolicy

	created := h.Apply("create", h.null(), alertLogsImmediate("tf default policy"))
	step := h.steps[len(h.steps)-1]
	if len(step.ApplyDiags) != 0 || len(step.Inconsistent) != 0 {
		t.Fatalf("apply diagnostics = %v, inconsistent = %v", step.ApplyDiags, step.Inconsistent)
	}
	if got := accessPolicyOf(t, created); !sameJSON(t, got, alertDefaultPolicy) {
		t.Fatalf("access_policy after Create = %v, want the default policy", got)
	}
	// Create writes, then reads the alert with the access permissions.
	var wrote, read bool
	for _, request := range step.Requests {
		wrote = wrote || request.Method == "POST"
		read = read || (wrote && request.Method == "GET" && strings.Contains(request.Path, "include_access_permissions=true"))
	}
	if !wrote || !read {
		t.Fatalf("requests = %v, want a POST and then a GET with include_access_permissions", step.Requests)
	}
	if step.ReplanEmpty == nil || !*step.ReplanEmpty {
		t.Fatalf("plan after Create is not empty: %v", step.ReplanDiff)
	}

	// An import reads the alert, so it must show the same policy as the state after Create.
	imported := h.Import("import", alertID(t, created))
	if got, want := accessPolicyOf(t, imported), accessPolicyOf(t, created); !reflect.DeepEqual(got, want) {
		t.Fatalf("access_policy after import = %v, after Create = %v", got, want)
	}

	// Update does not add a policy, and keeps the one that the alert has.
	updated := h.Apply("rename", created, alertLogsImmediate("tf default policy renamed"))
	step = h.steps[len(h.steps)-1]
	if len(step.ApplyDiags) != 0 || len(step.Inconsistent) != 0 {
		t.Fatalf("update diagnostics = %v, inconsistent = %v", step.ApplyDiags, step.Inconsistent)
	}
	if got := accessPolicyOf(t, updated); !sameJSON(t, got, alertDefaultPolicy) {
		t.Fatalf("access_policy after Update = %v, want the default policy", got)
	}
}

// A team without policy-based access has no policy for the alert: the state stays null after
// Create and after an import, and the extra read changes nothing else.
func TestAlertWriteWithoutAPolicyKeepsItNull(t *testing.T) {
	h := newAlertHarness(t)

	created := h.Apply("create", h.null(), alertLogsImmediate("tf no policy"))
	step := h.steps[len(h.steps)-1]
	if len(step.ApplyDiags) != 0 || len(step.Inconsistent) != 0 {
		t.Fatalf("apply diagnostics = %v, inconsistent = %v", step.ApplyDiags, step.Inconsistent)
	}
	if got := accessPolicyOf(t, created); got != nil {
		t.Fatalf("access_policy after Create = %v, want null", got)
	}
	imported := h.Import("import", alertID(t, created))
	if got := accessPolicyOf(t, imported); got != nil {
		t.Fatalf("access_policy after import = %v, want null", got)
	}
}

// A configured policy stays as written, also when it lists only some permissions: the backend stores
// the others as "deny", and the read after the write must not report that as another policy.
func TestAlertWriteKeepsAConfiguredPolicy(t *testing.T) {
	h := newAlertHarness(t)
	h.api.(*alertFake).defaultPolicy = alertDefaultPolicy
	configured := `{"default":{"permissions":{"alerts:ReadConfig":"grant"}},"rules":[]}`

	created := h.Apply("create", h.null(), alertLogsImmediate("tf configured policy", j{"access_policy": configured}))
	step := h.steps[len(h.steps)-1]
	if len(step.ApplyDiags) != 0 || len(step.Inconsistent) != 0 {
		t.Fatalf("apply diagnostics = %v, inconsistent = %v", step.ApplyDiags, step.Inconsistent)
	}
	if got := accessPolicyOf(t, created); got != configured {
		t.Fatalf("access_policy after Create = %v, want the configured text", got)
	}
	if step.ReplanEmpty == nil || !*step.ReplanEmpty {
		t.Fatalf("plan after Create is not empty: %v", step.ReplanDiff)
	}
	h.RefreshPlan("refresh", created)
	step = h.steps[len(h.steps)-1]
	if len(step.Drift) != 0 || step.ReplanEmpty == nil || !*step.ReplanEmpty {
		t.Fatalf("drift after a refresh = %v, replan diff = %v", step.Drift, step.ReplanDiff)
	}
}

// The write has succeeded when the read after it fails, so Create does not fail: the state holds the
// alert as the write returned it, and the next Read adds the policy.
func TestAlertWriteSurvivesAFailedReadAfterIt(t *testing.T) {
	h := newAlertHarness(t)
	fake := h.api.(*alertFake)
	fake.defaultPolicy = alertDefaultPolicy
	fake.failNextGet = true

	created := h.Apply("create", h.null(), alertLogsImmediate("tf failed read"))
	step := h.steps[len(h.steps)-1]
	if len(step.ApplyDiags) != 0 {
		t.Fatalf("a failed read after the write failed the Create: %v", step.ApplyDiags)
	}
	if created.IsNull() || alertID(t, created) == "" {
		t.Fatalf("Create left no state: %v", toJSON(created))
	}
	if got := accessPolicyOf(t, created); got != nil {
		t.Fatalf("access_policy after a failed read = %v, want null until the next Read", got)
	}
	refreshed := h.Refresh("refresh", created)
	if got := accessPolicyOf(t, refreshed); !sameJSON(t, got, alertDefaultPolicy) {
		t.Fatalf("access_policy after Refresh = %v, want the default policy", got)
	}
}
