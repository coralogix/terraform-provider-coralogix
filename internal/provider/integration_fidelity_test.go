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
	"sort"
	"strings"
	"testing"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
)

// TestIntegrationFakeMatchesRecordedBackend checks integrationFake against fidelity/recorded.json.
// Record that file with RECORD_INTEGRATION_FIDELITY=1 against a tenant, then keep the fake in line
// with those answers. This test skips when recorded.json does not exist.
func TestIntegrationFakeMatchesRecordedBackend(t *testing.T) {
	cases, recorded := loadFidelityAt(t, integrationGoldenDir)
	fake := newIntegrationFakeState()
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
				fake.mu.Lock()
				status, resp := fake.handle(s.Method, integrationBasePath+substString(s.Path, vars), body)
				fake.mu.Unlock()
				got := roundTrip(t, resp)
				if s.Capture != "" && status == http.StatusOK {
					vars[s.Capture] = integrationResponseID(got)
				}
				compareFidelityStep(t, label, status, integrationNormalize(unsubstJSON(t, got, vars)), rc.Steps[si].Status, integrationNormalize(rc.Steps[si].Response))
			}
		})
	}
}

// integrationNormalize sorts the parameters of a read by key, because the real API lists them in
// no fixed order, and drops the status of the integration, which the provider never reads.
func integrationNormalize(v any) any {
	body, ok := v.(map[string]any)
	if !ok {
		return v
	}
	integration, ok := body["integration"].(map[string]any)
	if !ok {
		return v
	}
	delete(integration, "integrationStatus")
	params := asList(integration["parameters"])
	sort.Slice(params, func(i, j int) bool {
		return params[i].(map[string]any)["key"].(string) < params[j].(map[string]any)["key"].(string)
	})
	integration["parameters"] = params
	return v
}

// integrationResponseID returns the id of a save answer or of a deployed integration.
func integrationResponseID(got any) string {
	body, _ := got.(map[string]any)
	if id, ok := body["integrationId"].(string); ok {
		return id
	}
	integration, _ := body["integration"].(map[string]any)
	id, _ := integration["id"].(string)
	return id
}

// TestRecordIntegrationFidelity sends fidelity/cases.json to a real tenant and writes
// fidelity/recorded.json. Set RECORD_INTEGRATION_FIDELITY=1 with CORALOGIX_API_KEY and
// CORALOGIX_ENV (or CX_REGION). The cases create integrations with the names tf-baseline-*, and
// the test deletes every one that a case leaves behind. Recording needs a management API key with
// integrations:ReadConfig and integrations:Deploy. The ids that the server makes are replaced by
// {{id}}, so the recording holds no tenant data.
func TestRecordIntegrationFidelity(t *testing.T) {
	if os.Getenv("RECORD_INTEGRATION_FIDELITY") != "1" {
		t.Skip("set RECORD_INTEGRATION_FIDELITY=1 with CORALOGIX_API_KEY and CORALOGIX_ENV to record against a tenant")
	}
	apiKey := os.Getenv("CORALOGIX_API_KEY")
	if apiKey == "" {
		t.Fatal("CORALOGIX_API_KEY is required to record")
	}
	region := os.Getenv("CORALOGIX_ENV")
	if region == "" {
		region = os.Getenv("CX_REGION")
	}
	base, ok := cxsdkOpenapi.URLFromRegion(strings.ToLower(region))
	if !ok {
		t.Fatalf("unknown region %q", region)
	}

	rawCases, err := os.ReadFile(filepath.Join(integrationGoldenDir, "fidelity", "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases fidelityCases
	if err := json.Unmarshal(rawCases, &cases); err != nil {
		t.Fatal(err)
	}

	var created []string
	t.Cleanup(func() {
		for _, id := range created {
			// A case may have deleted the integration already, which answers 404. Any other status
			// leaves a test integration in the tenant.
			status, _ := recordIntegrationStep(t, base, apiKey, fidelityStep{Method: http.MethodDelete, Path: "/integrations/v1/instances/" + id}, nil)
			if status != http.StatusOK && status != http.StatusNotFound {
				t.Errorf("cleanup: deleting the integration %s answered %d, so it may still exist in the tenant", id, status)
			}
		}
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
			status, got := recordIntegrationStep(t, base, apiKey, s, vars)
			if status == http.StatusForbidden || status == http.StatusUnauthorized {
				t.Fatalf("the integrations API returned %d on %s %s", status, s.Method, s.Path)
			}
			if s.Capture != "" && status == http.StatusOK {
				id := vars[s.Capture]
				created = append(created, id)
			}
			rec.Steps = append(rec.Steps, struct {
				Status   int `json:"status"`
				Response any `json:"response"`
			}{Status: status, Response: unsubstJSON(t, got, vars)})
		}
		out.Cases = append(out.Cases, rec)
	}
	raw, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(integrationGoldenDir, "fidelity", "recorded.json"), append(raw, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func recordIntegrationStep(t *testing.T, base, apiKey string, s fidelityStep, vars map[string]string) (int, any) {
	t.Helper()
	if vars == nil {
		vars = map[string]string{}
	}
	target := strings.TrimRight(base, "/") + integrationBasePath + substString(s.Path, vars)
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
	if s.Capture != "" && resp.StatusCode == http.StatusOK {
		vars[s.Capture] = integrationResponseID(parsed)
	}
	return resp.StatusCode, parsed
}
