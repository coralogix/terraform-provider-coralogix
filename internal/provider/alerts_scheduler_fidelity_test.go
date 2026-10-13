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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
)

// asRuleTokens are the rule ids that the cases capture. The server makes every rule id.
var asRuleTokens = []string{"R1", "R2", "R3", "R4", "R5", "R6", "R7", "R8", "R9"}

// TestAlertsSchedulerFakeMatchesRecordedBackend checks the fake alert scheduler backend against
// recorded.json. Record that file against a tenant, then keep the fake in line with those
// answers. This test skips when recorded.json does not exist.
func TestAlertsSchedulerFakeMatchesRecordedBackend(t *testing.T) {
	cases, recorded := loadFidelityAt(t, asGoldenDir)
	fake := &asFake{rules: map[string]map[string]any{}, alerts: asKnownAlerts}
	for ci, c := range cases.Cases {
		t.Run(c.Name, func(t *testing.T) {
			rc := recorded.Cases[ci]
			if rc.Name != c.Name || len(rc.Steps) != len(c.Steps) {
				t.Fatalf("recorded.json does not match cases.json for %q: record again", c.Name)
			}
			vars := asFidelityVars("fidelity")
			for si, s := range c.Steps {
				label := fmt.Sprintf("step %d: %s %s", si+1, s.Method, s.Path)
				status, got := runASFidelityStep(t, fake, s, vars)
				compareFidelityStep(t, label, status, normalizeASFidelity(got), rc.Steps[si].Status, normalizeASFidelity(rc.Steps[si].Response))
			}
		})
	}
}

func runASFidelityStep(t *testing.T, fake *asFake, s fidelityStep, vars map[string]string) (int, any) {
	t.Helper()
	body := s.Body
	if body != nil {
		body = substJSON(t, body, vars)
	}
	fake.mu.Lock()
	in, _ := body.(map[string]any)
	status, resp := fake.handle(s.Method, asBasePath+substString(s.Path, vars), in)
	fake.mu.Unlock()
	got := roundTrip(t, resp)
	captureASRule(s, status, got, vars)
	return status, unsubstJSON(t, got, vars)
}

// captureASRule stores the rule id of a successful Create under the token that the step names.
func captureASRule(s fidelityStep, status int, got any, vars map[string]string) {
	if s.Capture == "" || status != http.StatusOK {
		return
	}
	m, _ := got.(map[string]any)
	rule, _ := m["alertSchedulerRule"].(map[string]any)
	if id, _ := rule["uniqueIdentifier"].(string); id != "" {
		vars[s.Capture] = id
	}
}

// asFidelityVars returns the tokens of a run: a run name, so that the rule names are unique in a
// tenant, and a rule id that was never created.
func asFidelityVars(name string) map[string]string {
	return map[string]string{"NAME": name, "MISSING": "00000000-0000-4000-8000-999999999999"}
}

// normalizeASFidelity replaces the values that differ in every run: the version id of a rule, the
// ids of meta labels, and the times that the server sets.
func normalizeASFidelity(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, e := range x {
			switch k {
			case "id":
				out[k] = "<id>"
			case "createdAt", "updatedAt":
				out[k] = "<time>"
			default:
				out[k] = normalizeASFidelity(e)
			}
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = normalizeASFidelity(e)
		}
		return out
	}
	return v
}

// TestRecordAlertsSchedulerFidelity sends fidelity/cases.json to a real tenant and writes
// fidelity/recorded.json. Set RECORD_ALERTS_SCHEDULER_FIDELITY=1 with CORALOGIX_API_KEY and
// CORALOGIX_ENV (or CX_REGION). It creates suppression rules named tf-fidelity-<time>-* and
// deletes every rule that it created, also when a step fails. Rule ids are replaced with the
// case tokens before the file is written.
func TestRecordAlertsSchedulerFidelity(t *testing.T) {
	if os.Getenv("RECORD_ALERTS_SCHEDULER_FIDELITY") != "1" {
		t.Skip("set RECORD_ALERTS_SCHEDULER_FIDELITY=1 with CORALOGIX_API_KEY and CORALOGIX_ENV to record against a tenant")
	}
	apiKey := os.Getenv("CORALOGIX_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("CX_API_KEY")
	}
	if apiKey == "" {
		t.Fatal("CORALOGIX_API_KEY is required to record")
	}
	base := os.Getenv("ALERTS_SCHEDULER_FIDELITY_BASE_URL") // a local fake, for a dry run
	if base == "" {
		region := os.Getenv("CORALOGIX_ENV")
		if region == "" {
			region = os.Getenv("CX_REGION")
		}
		if region == "" {
			t.Fatal("CORALOGIX_ENV or CX_REGION is required to record")
		}
		var ok bool
		if base, ok = cxsdkOpenapi.URLFromRegion(strings.ToLower(region)); !ok {
			t.Fatalf("unknown region %q", region)
		}
	}
	recordAlertsSchedulerFidelity(t, base, apiKey, filepath.Join(asGoldenDir, "fidelity", "recorded.json"))
}

func recordAlertsSchedulerFidelity(t *testing.T, base, apiKey, outPath string) {
	t.Helper()
	rawCases, err := os.ReadFile(filepath.Join(asGoldenDir, "fidelity", "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases fidelityCases
	if err := json.Unmarshal(rawCases, &cases); err != nil {
		t.Fatal(err)
	}
	vars := asFidelityVars(fmt.Sprintf("tf-fidelity-%d", time.Now().Unix()))
	run := vars["NAME"]
	t.Cleanup(func() { cleanupRecordedASRules(base, apiKey, vars) })

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
			status, got := recordASFidelityStep(t, base, apiKey, s, vars)
			if status == http.StatusUnauthorized || status == http.StatusForbidden {
				t.Fatalf("the API answered %d. Recording needs a management API key with alert suppression rule permissions.", status)
			}
			// The run name makes the rule names unique in the tenant. The recording keeps the token.
			got = unsubstJSON(t, got, map[string]string{"NAME": run})
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
	if err := os.WriteFile(outPath, append(raw, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func recordASFidelityStep(t *testing.T, base, apiKey string, s fidelityStep, vars map[string]string) (int, any) {
	t.Helper()
	url := strings.TrimRight(base, "/") + asBasePath + substString(s.Path, vars)
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
	captureASRule(s, resp.StatusCode, parsed, vars)
	return resp.StatusCode, unsubstJSON(t, parsed, recordedRuleTokens(vars))
}

// recordedRuleTokens returns the captured rule ids, so that a recording names them by token. The
// run name is replaced separately, because the rule names contain it.
func recordedRuleTokens(vars map[string]string) map[string]string {
	out := map[string]string{}
	for _, name := range append(asRuleTokens, "MISSING") {
		if v := vars[name]; v != "" {
			out[name] = v
		}
	}
	return out
}

func cleanupRecordedASRules(base, apiKey string, vars map[string]string) {
	for _, name := range asRuleTokens {
		id := vars[name]
		if id == "" {
			continue
		}
		req, err := http.NewRequest(http.MethodDelete, strings.TrimRight(base, "/")+asBasePath+"/"+id, nil)
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

// TestAlertsSchedulerRecorderDryRun records the cases against the fake through HTTP, as against a
// tenant, and checks the recording against the fake. It keeps the recorder, the cases, and the
// comparison working without credentials.
func TestAlertsSchedulerRecorderDryRun(t *testing.T) {
	srv := httptest.NewServer(&asFake{rules: map[string]map[string]any{}, alerts: asKnownAlerts})
	t.Cleanup(srv.Close)
	out := filepath.Join(t.TempDir(), "recorded.json")
	recordAlertsSchedulerFidelity(t, srv.URL, "dry-run-key", out)

	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var recorded fidelityRecorded
	if err := json.Unmarshal(raw, &recorded); err != nil {
		t.Fatal(err)
	}
	rawCases, err := os.ReadFile(filepath.Join(asGoldenDir, "fidelity", "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases fidelityCases
	if err := json.Unmarshal(rawCases, &cases); err != nil {
		t.Fatal(err)
	}
	fake := &asFake{rules: map[string]map[string]any{}, alerts: asKnownAlerts}
	for ci, c := range cases.Cases {
		vars := asFidelityVars("fidelity")
		for si, s := range c.Steps {
			status, got := runASFidelityStep(t, fake, s, vars)
			want := recorded.Cases[ci].Steps[si]
			compareFidelityStep(t, fmt.Sprintf("%s step %d", c.Name, si+1), status, normalizeASFidelity(got), want.Status, normalizeASFidelity(want.Response))
		}
	}
}
