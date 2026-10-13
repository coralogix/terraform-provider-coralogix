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
	"strings"
	"testing"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
)

const viewGoldenDir = "testdata/view_baseline"

// TestViewFakeMatchesRecordedBackend checks viewFake against fidelity/recorded.json. Record
// that file with RECORD_VIEW_FIDELITY=1 against a tenant, then keep the fake in line with
// those answers. This test skips when recorded.json does not exist.
func TestViewFakeMatchesRecordedBackend(t *testing.T) {
	cases, recorded := loadFidelityAt(t, viewGoldenDir)
	fake := &viewFake{views: map[string]map[string]any{}}
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
				status, resp := fake.handle(s.Method, viewBasePath+substString(s.Path, vars), body)
				fake.mu.Unlock()
				got := viewFidelityResponse(roundTrip(t, resp), s, status, vars, nil)
				compareFidelityStep(t, label, status, got, rc.Steps[si].Status, rc.Steps[si].Response)
			}
		})
	}
}

// viewFidelityResponse captures the id of a created view and writes "{{id}}" in its place:
// the id is a number that each tenant assigns.
func viewFidelityResponse(got any, s fidelityStep, status int, vars map[string]string, created *[]string) any {
	view, ok := got.(map[string]any)
	if !ok {
		return got
	}
	if id, ok := view["id"].(float64); ok {
		if s.Capture != "" && status == http.StatusOK {
			vars[s.Capture] = fmt.Sprint(int64(id))
			if created != nil {
				*created = append(*created, vars[s.Capture])
			}
		}
		view["id"] = "{{id}}"
	}
	return view
}

// TestRecordViewFidelity sends fidelity/cases.json to a real tenant and writes
// fidelity/recorded.json. Set RECORD_VIEW_FIDELITY=1 with CORALOGIX_API_KEY and
// CORALOGIX_ENV (or CX_REGION). The views that the cases create are deleted afterwards.
func TestRecordViewFidelity(t *testing.T) {
	if os.Getenv("RECORD_VIEW_FIDELITY") != "1" {
		t.Skip("set RECORD_VIEW_FIDELITY=1 with CORALOGIX_API_KEY and CORALOGIX_ENV to record against a tenant")
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

	rawCases, err := os.ReadFile(filepath.Join(viewGoldenDir, "fidelity", "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases fidelityCases
	if err := json.Unmarshal(rawCases, &cases); err != nil {
		t.Fatal(err)
	}

	var created []string
	t.Cleanup(func() { cleanupRecordedViews(base, apiKey, created) })

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
			status, got := recordViewFidelityStep(t, base, apiKey, s, vars, &created)
			if status == http.StatusForbidden || status == http.StatusUnauthorized {
				t.Fatalf("the views API returned %d on %s %s. Recording needs a management API key that can manage saved views.", status, s.Method, s.Path)
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
	if err := os.WriteFile(filepath.Join(viewGoldenDir, "fidelity", "recorded.json"), append(raw, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func recordViewFidelityStep(t *testing.T, base, apiKey string, s fidelityStep, vars map[string]string, created *[]string) (int, any) {
	t.Helper()
	url := strings.TrimRight(base, "/") + viewBasePath + substString(s.Path, vars)
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
	return resp.StatusCode, viewFidelityResponse(parsed, s, resp.StatusCode, vars, created)
}

func cleanupRecordedViews(base, apiKey string, ids []string) {
	for _, id := range ids {
		req, err := http.NewRequest(http.MethodDelete, strings.TrimRight(base, "/")+viewBasePath+"/"+id, nil)
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
