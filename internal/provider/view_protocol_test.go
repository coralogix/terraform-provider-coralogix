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
	"fmt"
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/coralogix/coralogix-management-sdk/go/openapi/gen/views_service"
	"github.com/coralogix/terraform-provider-coralogix/internal/provider/generated/view"
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

// The follow-up Update after Create is a write like the others, so it runs the BeforeWrite
// overlay: once for the Create body, and once for the Update body.
func TestViewFollowUpUpdateRunsBeforeWrite(t *testing.T) {
	h := newViewHarness(t)
	cfg := h.config(viewConfig(j{"is_compact_mode": true}))
	planned, _, diags := h.plan(h.null(), cfg)
	if hasError(diags) {
		t.Fatal(diagStrings(diags))
	}
	var writes []string
	r := view.NewResourceWithClient(views_service.NewAPIClient(views_service.NewConfiguration()).ViewsServiceAPI, view.Hooks{
		BeforeWrite: func(_ context.Context, _ tfsdk.Config, body any) diag.Diagnostics {
			writes = append(writes, fmt.Sprintf("%T", body))
			return nil
		},
	})
	var schemaResp frameworkresource.SchemaResponse
	r.Schema(h.ctx, frameworkresource.SchemaRequest{}, &schemaResp)
	resp := &frameworkresource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: h.null()}}
	r.Create(h.ctx, frameworkresource.CreateRequest{
		Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: cfg},
		Plan:   tfsdk.Plan{Schema: schemaResp.Schema, Raw: planned},
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if got := methods(h.views().takeRequests()); len(got) != 2 || got[1] != http.MethodPut {
		t.Fatalf("requests = %v, want POST then PUT", got)
	}
	if len(writes) != 2 {
		t.Fatalf("BeforeWrite ran for %v, want the Create and the follow-up Update bodies", writes)
	}
}

// coralogix_view has no released version, so a request value that is still unknown stops
// Create before the API call instead of being left out. That holds for a configured nested
// value with a server default too: the request drops only an unknown that omission caused.
func TestViewCreateRejectsAnUnknownRequestValue(t *testing.T) {
	for name, unknownPath := range map[string]*tftypes.AttributePath{
		"folder_id":                tftypes.NewAttributePath().WithAttributeName("folder_id"),
		"search_query.syntax_type": tftypes.NewAttributePath().WithAttributeName("search_query").WithAttributeName("syntax_type"),
	} {
		t.Run(name, func(t *testing.T) {
			h := newViewHarness(t)
			known := h.config(viewConfig(j{"folder_id": "folder", "search_query": j{"query": "error", "syntax_type": "SYNTAX_TYPE_DATAPRIME"}}))
			unknown, err := tftypes.Transform(known, func(p *tftypes.AttributePath, v tftypes.Value) (tftypes.Value, error) {
				if p.Equal(unknownPath) {
					return tftypes.NewValue(v.Type(), tftypes.UnknownValue), nil
				}
				return v, nil
			})
			if err != nil {
				t.Fatal(err)
			}
			r := view.NewResourceWithClient(views_service.NewAPIClient(views_service.NewConfiguration()).ViewsServiceAPI, view.Hooks{})
			var schemaResp frameworkresource.SchemaResponse
			r.Schema(h.ctx, frameworkresource.SchemaRequest{}, &schemaResp)
			planned, _, diags := h.plan(h.null(), unknown)
			if hasError(diags) {
				t.Fatal(diagStrings(diags))
			}
			resp := &frameworkresource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: h.null()}}
			r.Create(h.ctx, frameworkresource.CreateRequest{
				Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: unknown},
				Plan:   tfsdk.Plan{Schema: schemaResp.Schema, Raw: planned},
			}, resp)
			if !resp.Diagnostics.HasError() {
				t.Fatalf("Create with an unknown %s succeeded, want an error", name)
			}
			if got := h.views().takeRequests(); len(got) != 0 {
				t.Fatalf("requests = %v, want none", methods(got))
			}
		})
	}
}

// A read can leave out a value that equals its declared default. A configured default then
// keeps its value: the apply matches the plan, and the next plan is empty.
func TestViewConfiguredDefaultSurvivesAnOmittedRead(t *testing.T) {
	h := newViewHarness(t)
	h.views().omitDefaults = true
	state := h.Apply("create", h.null(), viewConfig(j{"view_type": "VIEW_TYPE_LOGS", "is_compact_mode": false, "search_query": j{"query": "error", "syntax_type": "SYNTAX_TYPE_LUCENE"}}))
	step := lastStep(t, h)
	if got := attrOf(t, step.State, "view_type"); got != "VIEW_TYPE_LOGS" {
		t.Fatalf("view_type = %v, want VIEW_TYPE_LOGS", got)
	}
	if got := attrOf(t, step.State, "search_query", "syntax_type"); got != "SYNTAX_TYPE_LUCENE" {
		t.Fatalf("search_query.syntax_type = %v, want SYNTAX_TYPE_LUCENE", got)
	}
	h.Refresh("refresh", state)
	h.Apply("plan", state, viewConfig(j{"view_type": "VIEW_TYPE_LOGS", "is_compact_mode": false, "search_query": j{"query": "error", "syntax_type": "SYNTAX_TYPE_LUCENE"}}))
	if step := h.steps[len(h.steps)-1]; !step.NoChange {
		t.Fatal("the plan after an omitted read changes the view")
	}
}

// The views service stores another seconds than a non-preset value, and drops a false selected
// value, so the read would disagree with the plan. Validation rejects both before any request.
func TestViewRejectsValuesThatTheServiceChanges(t *testing.T) {
	h := newViewHarness(t)
	for name, test := range map[string]struct {
		config  j
		wantErr bool
	}{
		"preset seconds":       {viewConfig(), false},
		"non-preset seconds":   {viewConfig(j{"time_selection": j{"quick_selection": j{"seconds": 3601.0}}}), true},
		"true selected value":  {viewConfig(j{"filters": j{"filters": []any{j{"name": "severity", "selected_values": j{"ERROR": true}}}}}), false},
		"false selected value": {viewConfig(j{"filters": j{"filters": []any{j{"name": "severity", "selected_values": j{"ERROR": true, "INFO": false}}}}}), true},
	} {
		if got := hasError(h.validate(h.config(test.config))); got != test.wantErr {
			t.Errorf("%s: validation error %t, want %t", name, got, test.wantErr)
		}
	}
	if got := h.views().takeRequests(); len(got) != 0 {
		t.Fatalf("requests = %v, want none", methods(got))
	}
}
