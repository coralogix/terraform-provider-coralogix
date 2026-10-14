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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
)

// e2mFidelityIDs are the tokens of the events2metric ids that the server makes.
var e2mFidelityIDs = []string{"E1", "E2", "E3", "E4", "E5", "E6", "E7", "E8", "E9"}

// TestEvents2MetricFakeMatchesRecordedBackend checks the fake events2metrics
// backend against recorded.json. Record that file against a tenant, then keep
// the fake in line with those answers. This test skips when recorded.json does
// not exist.
func TestEvents2MetricFakeMatchesRecordedBackend(t *testing.T) {
	cases, recorded := loadFidelityAt(t, e2mGoldenDir)
	fake := &e2mFake{e2ms: map[string]map[string]any{}, exceeded: map[string]bool{}}
	for ci, c := range cases.Cases {
		t.Run(c.Name, func(t *testing.T) {
			rc := recorded.Cases[ci]
			if rc.Name != c.Name || len(rc.Steps) != len(c.Steps) {
				t.Fatalf("recorded.json does not match cases.json for %q: record again", c.Name)
			}
			vars := e2mFidelityNames("fid")
			for si, s := range c.Steps {
				label := fmt.Sprintf("step %d: %s %s", si+1, s.Method, s.Path)
				status, got := runE2MFidelityStep(t, fake, s, vars)
				compareFidelityStep(t, label, status, sortE2MFidelity(got), rc.Steps[si].Status, sortE2MFidelity(rc.Steps[si].Response))
			}
		})
	}
}

// e2mFidelityNames gives the name and base-name tokens their values. The API keeps both
// unique per company, so a recording uses a prefix that no other run uses.
func e2mFidelityNames(prefix string) map[string]string {
	vars := map[string]string{"MISSING": "00000000-0000-4000-8000-999999999999"}
	for i := 1; i <= 9; i++ {
		vars[fmt.Sprintf("N%d", i)] = fmt.Sprintf("%s_name_%d", prefix, i)
		vars[fmt.Sprintf("B%d", i)] = fmt.Sprintf("%s_metric_%d", prefix, i)
	}
	return vars
}

func runE2MFidelityStep(t *testing.T, fake *e2mFake, s fidelityStep, vars map[string]string) (int, any) {
	t.Helper()
	body := s.Body
	if body != nil {
		body = substJSON(t, body, vars)
	}
	fake.mu.Lock()
	status, resp := fake.handle(s.Method, e2mBasePath+substString(s.Path, vars), body)
	fake.mu.Unlock()
	got := roundTrip(t, resp)
	captureE2MID(s, status, got, vars)
	return status, unsubstJSON(t, got, vars)
}

func captureE2MID(s fidelityStep, status int, got any, vars map[string]string) {
	if s.Capture == "" || status != http.StatusOK {
		return
	}
	e2m, _ := got.(map[string]any)["e2m"].(map[string]any)
	if id, _ := e2m["id"].(string); id != "" {
		vars[s.Capture] = id
	}
}

// sortE2MFidelity sorts the metric fields by base name and the aggregations by type. The API
// returns both in no particular order, so the order is not part of the comparison.
func sortE2MFidelity(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, e := range x {
			out[k] = sortE2MFidelity(e)
		}
		if fields, ok := out["metricFields"].([]any); ok {
			slices.SortFunc(fields, byKey("targetBaseMetricName"))
		}
		if aggs, ok := out["aggregations"].([]any); ok {
			slices.SortFunc(aggs, byKey("aggType"))
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = sortE2MFidelity(e)
		}
		return out
	}
	return v
}

// TestRecordEvents2MetricFidelity sends fidelity/cases.json to a real tenant and
// writes fidelity/recorded.json. Set RECORD_EVENTS2METRIC_FIDELITY=1 with
// CORALOGIX_API_KEY and CORALOGIX_ENV (or CX_REGION). Ids, names, and metric
// names are replaced with the case tokens before the file is written.
func TestRecordEvents2MetricFidelity(t *testing.T) {
	if os.Getenv("RECORD_EVENTS2METRIC_FIDELITY") != "1" {
		t.Skip("set RECORD_EVENTS2METRIC_FIDELITY=1 with CORALOGIX_API_KEY and CORALOGIX_ENV to record against a tenant")
	}
	apiKey := os.Getenv("CORALOGIX_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("CX_API_KEY")
	}
	if apiKey == "" {
		t.Fatal("CORALOGIX_API_KEY is required to record")
	}
	region := os.Getenv("CORALOGIX_ENV")
	if region == "" {
		region = os.Getenv("CX_REGION")
	}
	if region == "" {
		t.Fatal("CORALOGIX_ENV or CX_REGION is required to record")
	}
	base, ok := cxsdkOpenapi.URLFromRegion(strings.ToLower(region))
	if !ok {
		t.Fatalf("unknown region %q", region)
	}

	rawCases, err := os.ReadFile(filepath.Join(e2mGoldenDir, "fidelity", "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases fidelityCases
	if err := json.Unmarshal(rawCases, &cases); err != nil {
		t.Fatal(err)
	}

	vars := e2mFidelityNames("tf_fid_" + strings.ReplaceAll(newUUID()[:8], "-", ""))
	t.Cleanup(func() { cleanupRecordedE2Ms(base, apiKey, vars) })

	out := fidelityRecorded{}
	for _, c := range cases.Cases {
		rec := struct {
			Name  string `json:"name"`
			Steps []struct {
				Status   int `json:"status"`
				Response any `json:"response"`
			} `json:"steps"`
		}{Name: c.Name}
		for _, s := range c.Steps {
			status, got := recordE2MFidelityStep(t, base, apiKey, s, vars)
			if status == http.StatusForbidden || status == http.StatusUnauthorized {
				t.Fatalf("events2metrics returned %d. Recording needs a management API key with events2metrics permissions.", status)
			}
			rec.Steps = append(rec.Steps, struct {
				Status   int `json:"status"`
				Response any `json:"response"`
			}{Status: status, Response: got})
		}
		out.Cases = append(out.Cases, rec)
	}
	raw, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(e2mGoldenDir, "fidelity", "recorded.json"), append(raw, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func recordE2MFidelityStep(t *testing.T, base, apiKey string, s fidelityStep, vars map[string]string) (int, any) {
	t.Helper()
	url := strings.TrimRight(base, "/") + e2mBasePath + substString(s.Path, vars)
	var body io.Reader
	if s.Body != nil {
		raw, err := json.Marshal(substJSON(t, s.Body, vars))
		if err != nil {
			t.Fatal(err)
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(s.Method, url, body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	var parsed any
	if len(bytes.TrimSpace(raw)) > 0 {
		if err := json.Unmarshal(raw, &parsed); err != nil {
			parsed = map[string]any{"raw": string(raw)}
		}
	} else {
		parsed = map[string]any{}
	}
	captureE2MID(s, resp.StatusCode, parsed, vars)
	return resp.StatusCode, unsubstJSON(t, parsed, vars)
}

// cleanupRecordedE2Ms deletes every events2metric that the recording made, also when a case
// stopped before its own Delete.
func cleanupRecordedE2Ms(base, apiKey string, vars map[string]string) {
	for _, name := range e2mFidelityIDs {
		id := vars[name]
		if id == "" {
			continue
		}
		req, err := http.NewRequest(http.MethodDelete, strings.TrimRight(base, "/")+e2mBasePath+"/"+id, nil)
		if err != nil {
			continue
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			continue
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}
}
