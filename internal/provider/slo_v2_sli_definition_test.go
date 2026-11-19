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
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
)

const sliProbeBasePath = "/slo/slos/v1"

// sliProbeFake answers like the eu2 API did in a probe on 2026-11-19: a read returns both sliDefinition
// and the legacy requestBasedMetricSli, whichever of them the request sent.
//
// One rule is not from the probe but from the contract: when a request sends sliDefinition, the
// deprecated fields are ignored. The fake follows it, so a request that sends a stale
// sliDefinition next to a changed legacy arm loses the change.
type sliProbeFake struct {
	mu       sync.Mutex
	slos     map[string]map[string]any
	seq      int
	requests []grRequest
}

func newSliProbeFake(t *testing.T) *sliProbeFake {
	t.Helper()
	f := &sliProbeFake{slos: map[string]map[string]any{}}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	target, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	original := http.DefaultTransport
	http.DefaultTransport = redirectTransport{target: target, next: original}
	t.Cleanup(func() { http.DefaultTransport = original })
	return f
}

func (f *sliProbeFake) takeRequests() []grRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.requests
	f.requests = nil
	return out
}

func (f *sliProbeFake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		panic(err)
	}
	var body map[string]any
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &body); err != nil {
			panic(err)
		}
	}
	status, out := f.handle(r, body)
	rec := grRequest{Method: r.Method, Path: r.URL.RequestURI(), Status: status}
	if len(raw) > 0 {
		rec.Body = json.RawMessage(raw)
	}
	f.requests = append(f.requests, rec)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(out)
}

func (f *sliProbeFake) handle(r *http.Request, body map[string]any) (int, any) {
	_, rest, ok := strings.Cut(r.URL.Path, sliProbeBasePath)
	if !ok {
		return 404, map[string]any{"code": 404, "message": "Not Found"}
	}
	id := strings.TrimPrefix(rest, "/")
	switch {
	case r.Method == http.MethodPost && id == "":
		f.seq++
		id = fmt.Sprintf("00000000-0000-4000-8000-%012d", f.seq)
		return 200, map[string]any{"slo": f.store(id, body), "exclusionWindowIds": []any{}}
	case r.Method == http.MethodPut && id == "":
		id, _ = body["id"].(string)
		if _, found := f.slos[id]; !found {
			return 404, map[string]any{"code": 404, "message": "Not Found: SLO not found"}
		}
		return 200, map[string]any{"slo": f.store(id, body), "exclusionWindowIds": []any{}}
	case r.Method == http.MethodGet && id != "":
		slo, found := f.slos[id]
		if !found {
			return 404, map[string]any{"code": 404, "message": "Not Found: SLO not found"}
		}
		return 200, map[string]any{"slo": slo}
	case r.Method == http.MethodDelete && id != "":
		delete(f.slos, id)
		return 200, map[string]any{}
	}
	return 404, map[string]any{"code": 404, "message": "Not Found"}
}

// store keeps the request as the API does: it derives sliDefinition from the legacy arm and the
// legacy arm from sliDefinition, for a request-based custom SLI.
func (f *sliProbeFake) store(id string, body map[string]any) map[string]any {
	slo := maps.Clone(body)
	if definition, ok := slo["sliDefinition"].(map[string]any); ok {
		delete(slo, "requestBasedMetricSli")
		if requestBased, ok := definition["requestBased"].(map[string]any); ok {
			if custom, ok := requestBased["custom"]; ok {
				slo["requestBasedMetricSli"] = custom
			}
		}
	} else if legacy, ok := slo["requestBasedMetricSli"]; ok {
		slo["sliDefinition"] = map[string]any{"requestBased": map[string]any{"custom": legacy}}
	}
	// The contract says that the server stores uncounted when a window-based SLI omits the strategy.
	// That is from the contract, not from a probe.
	if definition, ok := slo["sliDefinition"].(map[string]any); ok {
		if window, ok := definition["windowBased"].(map[string]any); ok {
			if _, set := window["missingDataStrategy"]; !set {
				window["missingDataStrategy"] = "MISSING_DATA_STRATEGY_UNCOUNTED"
			}
		}
	}
	slo["id"] = id
	slo["creator"] = "support@coralogix.com"
	slo["labels"] = map[string]any{}
	slo["grouping"] = map[string]any{"labels": []any{}}
	slo["revision"] = map[string]any{"revision": 1, "updateTime": "2026-11-19T13:10:49.162Z"}
	slo["createTime"] = "2026-11-19T13:10:49.162Z"
	slo["updateTime"] = "2026-11-19T13:10:49.163Z"
	slo["type"], slo["sloType"] = "request", "SLO_TYPE_REQUEST"
	slo["productType"] = "SLO_PRODUCT_TYPE_UNSPECIFIED"
	f.slos[id] = slo
	return slo
}

func sliProbeLegacyConfig(query string) map[string]any {
	metric := map[string]any{"query": query}
	return map[string]any{
		"name":                        "sli-definition-probe",
		"target_threshold_percentage": 99.0,
		"window":                      map[string]any{"slo_time_frame": "7_days"},
		"sli": map[string]any{
			"request_based_metric_sli": map[string]any{"good_events": metric, "total_events": metric},
		},
	}
}

func sliProbeDefinitionConfig(query string) map[string]any {
	metric := map[string]any{"query": query}
	return map[string]any{
		"name":                        "sli-definition-probe",
		"target_threshold_percentage": 99.0,
		"window":                      map[string]any{"slo_time_frame": "7_days"},
		"sli_definition": map[string]any{
			"request_based": map[string]any{"custom": map[string]any{"good_events": metric, "total_events": metric}},
		},
	}
}

// requestBodies returns the JSON bodies of the writes among the requests.
func sliProbeWriteBodies(t *testing.T, requests []grRequest) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, request := range requests {
		if request.Method != http.MethodPost && request.Method != http.MethodPut {
			continue
		}
		raw, ok := request.Body.(json.RawMessage)
		if !ok {
			t.Fatalf("%s %s has no body", request.Method, request.Path)
		}
		var body map[string]any
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Fatal(err)
		}
		out = append(out, body)
	}
	return out
}

func requireCleanStep(t *testing.T, step grStep) {
	t.Helper()
	// The legacy arms are deprecated, so a validation warning is expected. An error is not.
	for _, diags := range [][]string{step.ValidateDiags, step.PlanDiags, step.ApplyDiags} {
		for _, diag := range diags {
			if !strings.HasPrefix(diag, "warning") {
				t.Fatalf("%s: diagnostics: validate %v, plan %v, apply %v", step.Step, step.ValidateDiags, step.PlanDiags, step.ApplyDiags)
			}
		}
	}
	if len(step.Inconsistent) != 0 {
		t.Fatalf("%s: Terraform core would fail the apply with inconsistent result at %v", step.Step, step.Inconsistent)
	}
	if step.ReplanEmpty == nil || !*step.ReplanEmpty {
		t.Fatalf("%s: the plan after the apply is not empty: %v", step.Step, step.ReplanDiff)
	}
}

// hasErrorDiag reports whether a diagnostic is an error. The deprecated legacy arms add a warning.
func hasErrorDiag(diags []string) bool {
	for _, diag := range diags {
		if strings.HasPrefix(diag, "error") {
			return true
		}
	}
	return false
}

func stateAttribute(state any, name string) any {
	object, _ := state.(map[string]any)
	return object[name]
}

// A read returns sliDefinition for an SLO that was written with the legacy arm. The apply must not
// fail with an inconsistent result, and a change of the legacy arm must reach the API: the request
// must not carry the stale sliDefinition, which the API prefers over the legacy arms.
func TestSloV2LegacyArmUserGetsSliDefinitionBackAndCanEdit(t *testing.T) {
	fake := newSliProbeFake(t)
	h := newProtocolHarness(t, "coralogix_slo_v2", fake)

	state := h.Apply("create with the legacy arm", h.null(), sliProbeLegacyConfig("sum(rate(a[1m]))"))
	requireCleanStep(t, h.steps[len(h.steps)-1])
	if stateAttribute(h.steps[len(h.steps)-1].State, "sli_definition") == nil {
		t.Fatal("state after create has no sli_definition, but the API returns it")
	}
	for _, body := range sliProbeWriteBodies(t, h.steps[len(h.steps)-1].Requests) {
		if _, sent := body["sliDefinition"]; sent {
			t.Fatalf("create sent sliDefinition for a legacy config: %v", body)
		}
	}

	h.Apply("edit the legacy query", state, sliProbeLegacyConfig("sum(rate(b[1m]))"))
	step := h.steps[len(h.steps)-1]
	requireCleanStep(t, step)
	writes := sliProbeWriteBodies(t, step.Requests)
	if len(writes) != 1 {
		t.Fatalf("want one write, got %d", len(writes))
	}
	if _, sent := writes[0]["sliDefinition"]; sent {
		t.Fatalf("update sent the stale sliDefinition, which the API prefers over the legacy arm: %v", writes[0])
	}
	for _, slo := range fake.slos {
		legacy := slo["requestBasedMetricSli"].(map[string]any)["goodEvents"].(map[string]any)["query"]
		if legacy != "sum(rate(b[1m]))" {
			t.Fatalf("the API still holds query %v after the edit", legacy)
		}
	}
}

// A user of sli_definition gets the legacy arm back from a read.
func TestSloV2SliDefinitionUserGetsLegacyArmBack(t *testing.T) {
	fake := newSliProbeFake(t)
	h := newProtocolHarness(t, "coralogix_slo_v2", fake)

	state := h.Apply("create with sli_definition", h.null(), sliProbeDefinitionConfig("sum(rate(a[1m]))"))
	step := h.steps[len(h.steps)-1]
	requireCleanStep(t, step)
	if sli, _ := stateAttribute(step.State, "sli").(map[string]any); sli == nil || sli["request_based_metric_sli"] == nil {
		t.Fatalf("state has no sli.request_based_metric_sli, but the API returns it: %v", stateAttribute(step.State, "sli"))
	}

	h.Apply("edit the sli_definition query", state, sliProbeDefinitionConfig("sum(rate(b[1m]))"))
	step = h.steps[len(h.steps)-1]
	requireCleanStep(t, step)
	for _, slo := range fake.slos {
		query := slo["sliDefinition"].(map[string]any)["requestBased"].(map[string]any)["custom"].(map[string]any)["goodEvents"].(map[string]any)["query"]
		if query != "sum(rate(b[1m]))" {
			t.Fatalf("the API still holds query %v after the edit", query)
		}
	}
}

// Exactly one of sli and sli_definition is required.
func TestSloV2NeedsOneOfSliAndSliDefinition(t *testing.T) {
	h := newProtocolHarness(t, "coralogix_slo_v2", newSliProbeFake(t))

	both := sliProbeLegacyConfig("sum(rate(a[1m]))")
	both["sli_definition"] = sliProbeDefinitionConfig("sum(rate(a[1m]))")["sli_definition"]
	h.Validate("both", both)
	if !hasErrorDiag(h.steps[len(h.steps)-1].ValidateDiags) {
		t.Error("sli and sli_definition together must be rejected")
	}

	neither := sliProbeLegacyConfig("sum(rate(a[1m]))")
	delete(neither, "sli")
	h.Validate("neither", neither)
	if !hasErrorDiag(h.steps[len(h.steps)-1].ValidateDiags) {
		t.Error("a config with neither sli nor sli_definition must be rejected")
	}
}

// A window-based sli_definition that leaves out missing_data_strategy gets the server default back.
// The state holds the Terraform spelling of the enum, so the default that the plan keeps for an
// omitted value must be in that spelling too, or every plan proposes an update.
func TestSloV2WindowBasedServerDefaultPlansClean(t *testing.T) {
	fake := newSliProbeFake(t)
	h := newProtocolHarness(t, "coralogix_slo_v2", fake)

	config := map[string]any{
		"name":                        "sli-definition-window",
		"target_threshold_percentage": 99.0,
		"window":                      map[string]any{"slo_time_frame": "7_days"},
		"sli_definition": map[string]any{
			"window_based": map[string]any{
				"window": "1_minute",
				"custom": map[string]any{
					"comparison_operator": "greater_than",
					"query":               map[string]any{"query": "sum(rate(a[1m]))"},
					"threshold":           1.0,
				},
			},
		},
	}
	h.Apply("create a window-based SLI without missing_data_strategy", h.null(), config)
	step := h.steps[len(h.steps)-1]
	requireCleanStep(t, step)
	window, _ := stateAttribute(step.State, "sli_definition").(map[string]any)
	inner, _ := window["window_based"].(map[string]any)
	if got := inner["missing_data_strategy"]; got != "uncounted" {
		t.Fatalf("state holds missing_data_strategy %v, want the Terraform spelling uncounted", got)
	}
}
