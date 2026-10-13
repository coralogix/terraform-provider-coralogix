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
)

// These tests drive the generated coralogix_view through the plugin protocol against
// viewFake. Each apply must succeed, match its plan, and plan no change again.

func viewConfig(extra ...j) j {
	out := j{
		"name":           "Logs view",
		"time_selection": j{"quick_selection": j{"seconds": 3600.0}},
	}
	for _, e := range extra {
		for k, v := range e {
			out[k] = v
		}
	}
	return out
}

// lastStep returns the last recorded step and fails the test when its apply did not
// match the plan or the next plan is not empty.
func lastStep(t *testing.T, h *grHarness) grStep {
	t.Helper()
	step := h.steps[len(h.steps)-1]
	if len(step.ApplyDiags) != 0 || len(step.PlanDiags) != 0 || len(step.ValidateDiags) != 0 {
		t.Fatalf("%s: diagnostics validate=%v plan=%v apply=%v", step.Step, step.ValidateDiags, step.PlanDiags, step.ApplyDiags)
	}
	if len(step.Inconsistent) != 0 {
		t.Fatalf("%s: inconsistent result at %v", step.Step, step.Inconsistent)
	}
	if step.ReplanEmpty != nil && !*step.ReplanEmpty {
		t.Fatalf("%s: the next plan changes %v", step.Step, step.ReplanDiff)
	}
	return step
}

func methods(requests []grRequest) []string {
	out := make([]string, 0, len(requests))
	for _, r := range requests {
		out = append(out, r.Method)
	}
	return out
}

func attrOf(t *testing.T, state any, path ...string) any {
	t.Helper()
	v := state
	for _, p := range path {
		m, ok := v.(map[string]any)
		if !ok {
			t.Fatalf("%v: %v is not an object", path, v)
		}
		v = m[p]
	}
	return v
}

// Create cannot send is_compact_mode, so a configured true follows Create with a replace.
func TestViewCreateSetsCompactModeWithAReplace(t *testing.T) {
	h := newViewHarness(t)
	h.Apply("create", h.null(), viewConfig(j{"is_compact_mode": true}))
	step := lastStep(t, h)
	if got := methods(step.Requests); len(got) != 2 || got[0] != http.MethodPost || got[1] != http.MethodPut {
		t.Fatalf("requests = %v, want POST then PUT", got)
	}
	if body, _ := step.Requests[1].Body.(map[string]any); body["isCompactMode"] != true {
		t.Fatalf("replace body = %v, want isCompactMode true", step.Requests[1].Body)
	}
	if got := attrOf(t, step.State, "is_compact_mode"); got != true {
		t.Fatalf("is_compact_mode = %v, want true", got)
	}
}

// An omitted is_compact_mode, view_type, and syntax_type read back as the server defaults with
// no extra request.
func TestViewCreateReadsTheServerDefaults(t *testing.T) {
	h := newViewHarness(t)
	h.Apply("create", h.null(), viewConfig(j{"search_query": j{"query": "error"}}))
	step := lastStep(t, h)
	if got := methods(step.Requests); len(got) != 1 || got[0] != http.MethodPost {
		t.Fatalf("requests = %v, want one POST", got)
	}
	if body, _ := step.Requests[0].Body.(map[string]any); body["viewType"] != nil {
		t.Fatalf("create body = %v, want no viewType", body)
	}
	for path, want := range map[string]any{"is_compact_mode": false, "view_type": "VIEW_TYPE_LOGS"} {
		if got := attrOf(t, step.State, path); got != want {
			t.Fatalf("%s = %v, want %v", path, got, want)
		}
	}
	if got := attrOf(t, step.State, "search_query", "syntax_type"); got != "SYNTAX_TYPE_LUCENE" {
		t.Fatalf("search_query.syntax_type = %v, want SYNTAX_TYPE_LUCENE", got)
	}
}

// Removing a configured syntax_type or is_compact_mode plans it unknown, and the replace that
// omits it stores the server default.
func TestViewRemovingAValueRestoresTheServerDefault(t *testing.T) {
	h := newViewHarness(t)
	state := h.Apply("create", h.null(), viewConfig(j{
		"is_compact_mode": true,
		"search_query":    j{"query": "error", "syntax_type": "SYNTAX_TYPE_DATAPRIME"},
	}))
	lastStep(t, h)
	h.Apply("remove", state, viewConfig(j{"search_query": j{"query": "error"}}))
	step := lastStep(t, h)
	if got := methods(step.Requests); len(got) != 1 || got[0] != http.MethodPut {
		t.Fatalf("requests = %v, want one PUT", got)
	}
	if got := attrOf(t, step.State, "search_query", "syntax_type"); got != "SYNTAX_TYPE_LUCENE" {
		t.Fatalf("search_query.syntax_type = %v, want SYNTAX_TYPE_LUCENE", got)
	}
	if got := attrOf(t, step.State, "is_compact_mode"); got != false {
		t.Fatalf("is_compact_mode = %v, want false", got)
	}
}

// A view saved on another Explore tab reads back without viewType. Importing it gives a null
// view_type, and a configuration without view_type plans no change.
func TestViewImportOfAnotherTabPlansNoChange(t *testing.T) {
	h := newViewHarness(t)
	id := h.views().seedLegacy("Legacy view")
	state := h.Import("import", id)
	if got := attrOf(t, toJSON(state), "view_type"); got != nil {
		t.Fatalf("view_type = %v, want null", got)
	}
	h.Apply("plan", state, viewConfig(j{"name": "Legacy view"}))
	if step := h.steps[len(h.steps)-1]; !step.NoChange {
		t.Fatalf("plan changes the imported view: %v", toJSON(state))
	}
}

// A time selection needs quick_selection or custom_selection: the service rejects neither.
func TestViewRequiresATimeSelectionArm(t *testing.T) {
	h := newViewHarness(t)
	if diags := h.validate(h.config(viewConfig(j{"time_selection": j{}}))); !hasError(diags) {
		t.Fatal("time_selection = {} passed validation, want an error")
	}
}

// When Create stores another is_compact_mode than the server default and the configuration
// omits it, the follow-up replace omits it too, so the service stores the default.
func TestViewCreateConvergesAnOmittedUpdateOnlyValue(t *testing.T) {
	h := newViewHarness(t)
	h.views().createCompact = true
	h.Apply("create", h.null(), viewConfig())
	step := lastStep(t, h)
	if got := methods(step.Requests); len(got) != 2 || got[1] != http.MethodPut {
		t.Fatalf("requests = %v, want POST then PUT", got)
	}
	if body, _ := step.Requests[1].Body.(map[string]any); body["isCompactMode"] != nil {
		t.Fatalf("replace body = %v, want no isCompactMode", body)
	}
	if got := attrOf(t, step.State, "is_compact_mode"); got != false {
		t.Fatalf("is_compact_mode = %v, want false", got)
	}
}
