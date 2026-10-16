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
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
)

// TestQuotaAllocationRuleSetFakeMatchesRecordedBackend checks quotaFake against
// fidelity/recorded.json. Record that file with RECORD_QUOTA_FIDELITY=1 against a tenant, then
// keep the fake in line with those answers. This test skips when recorded.json does not exist.
//
// The fake starts like the tenant did: when the first recorded answer is a rule set, the fake
// holds the Coralogix-managed rules of that answer.
func TestQuotaAllocationRuleSetFakeMatchesRecordedBackend(t *testing.T) {
	cases, recorded := loadFidelityAt(t, quotaGoldenDir)
	fake := &quotaFake{}
	if first := recorded.Cases[0].Steps[0]; first.Status == http.StatusOK {
		fake.seedManaged(quotaManagedRules(first.Response)...)
	}
	for ci, c := range cases.Cases {
		t.Run(c.Name, func(t *testing.T) {
			rc := recorded.Cases[ci]
			if rc.Name != c.Name || len(rc.Steps) != len(c.Steps) {
				t.Fatalf("recorded.json does not match cases.json for %q: record again", c.Name)
			}
			vars := map[string]string{}
			for si, s := range c.Steps {
				label := fmt.Sprintf("step %d: %s %s", si+1, s.Method, s.Path)
				body := s.Body
				if body != nil {
					body = substJSON(t, body, vars)
				}
				id := quotaQueryID(substString(s.Path, vars))
				fake.mu.Lock()
				status, resp := fake.handle(s.Method, quotaBasePath, id, body)
				fake.mu.Unlock()
				got := quotaFidelityResponse(roundTrip(t, resp), s, status, vars)
				compareFidelityStep(t, label, status, got, rc.Steps[si].Status, rc.Steps[si].Response)
			}
		})
	}
}

// quotaManagedRules returns the rules that Coralogix manages in a recorded rule set.
func quotaManagedRules(response any) []map[string]any {
	body, _ := response.(map[string]any)
	ruleSet, _ := body["ruleSet"].(map[string]any)
	var out []map[string]any
	for _, r := range asList(ruleSet["rules"]) {
		if rule, ok := r.(map[string]any); ok && rule["cxManaged"] == true {
			out = append(out, rule)
		}
	}
	return out
}

func quotaQueryID(path string) string {
	_, query, _ := strings.Cut(path, "?")
	values, _ := url.ParseQuery(query)
	return values.Get("id")
}

// quotaFidelityResponse captures the id of the rule set, and writes a placeholder for it, because
// each tenant has its own.
func quotaFidelityResponse(got any, s fidelityStep, status int, vars map[string]string) any {
	body, ok := got.(map[string]any)
	if !ok {
		return got
	}
	if ruleSet, ok := body["ruleSet"].(map[string]any); ok {
		if id, ok := ruleSet["id"].(string); ok {
			if s.Capture != "" && status == http.StatusOK {
				vars[s.Capture] = id
			}
			ruleSet["id"] = "<id>"
		}
	}
	if _, ok := body["id"].(string); ok {
		body["id"] = "<id>"
	}
	return body
}

// TestRecordQuotaAllocationRuleSetFidelity sends fidelity/cases.json to a real tenant and writes
// fidelity/recorded.json. Set RECORD_QUOTA_FIDELITY=1 with CORALOGIX_API_KEY and CORALOGIX_ENV
// (or CX_REGION). The rule set is one per account, so the test stops when the tenant already
// has customer rules, and clears the customer rules that the cases write when it ends.
func TestRecordQuotaAllocationRuleSetFidelity(t *testing.T) {
	if os.Getenv("RECORD_QUOTA_FIDELITY") != "1" {
		t.Skip("set RECORD_QUOTA_FIDELITY=1 with CORALOGIX_API_KEY and CORALOGIX_ENV to record against a tenant")
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

	rawCases, err := os.ReadFile(filepath.Join(quotaGoldenDir, "fidelity", "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases fidelityCases
	if err := json.Unmarshal(rawCases, &cases); err != nil {
		t.Fatal(err)
	}

	status, current := recordQuotaFidelityStep(t, base, apiKey, fidelityStep{Method: http.MethodGet}, map[string]string{})
	if status != http.StatusOK && status != http.StatusNotFound {
		t.Fatalf("the quota API returned %d on GET. Recording needs a management API key with team-quota-rules:Read and team-quota-rules:Manage.", status)
	}
	if status == http.StatusOK {
		body, _ := current.(map[string]any)
		ruleSet, _ := body["ruleSet"].(map[string]any)
		for _, r := range asList(ruleSet["rules"]) {
			if rule, _ := r.(map[string]any); rule["cxManaged"] != true {
				t.Fatal("the tenant already has customer quota allocation rules, and the recording would replace them. Use a tenant without them.")
			}
		}
	}
	t.Cleanup(func() {
		recordQuotaFidelityStep(t, base, apiKey, fidelityStep{Method: http.MethodDelete}, map[string]string{})
	})

	out := fidelityRecorded{}
	for _, c := range cases.Cases {
		rec := struct {
			Name  string `json:"name"`
			Steps []struct {
				Status   int `json:"status"`
				Response any `json:"response"`
			} `json:"steps"`
		}{Name: c.Name}
		vars := map[string]string{}
		for _, s := range c.Steps {
			status, got := recordQuotaFidelityStep(t, base, apiKey, s, vars)
			if status == http.StatusForbidden || status == http.StatusUnauthorized {
				t.Fatalf("the quota API returned %d on %s %s. Recording needs a management API key with team-quota-rules:Manage.", status, s.Method, s.Path)
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
	if err := os.WriteFile(filepath.Join(quotaGoldenDir, "fidelity", "recorded.json"), append(raw, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func recordQuotaFidelityStep(t *testing.T, base, apiKey string, s fidelityStep, vars map[string]string) (int, any) {
	t.Helper()
	target := strings.TrimRight(base, "/") + quotaBasePath + substString(s.Path, vars)
	var body io.Reader
	if s.Body != nil {
		raw, err := json.Marshal(substJSON(t, s.Body, vars))
		if err != nil {
			t.Fatal(err)
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(s.Method, target, body)
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
	return resp.StatusCode, quotaFidelityResponse(parsed, s, resp.StatusCode, vars)
}
