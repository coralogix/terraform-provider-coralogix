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

// TestPresetFakeMatchesRecordedBackend checks the fake preset backend against
// recorded.json. Record that file with RECORD_PRESET_FIDELITY=1 against a
// tenant, then keep the fake in line with those answers. This test skips
// when recorded.json does not exist.
func TestPresetFakeMatchesRecordedBackend(t *testing.T) {
	cases, recorded := loadFidelityAt(t, presetGoldenDir)
	fake := &presetFake{}
	fake.reset()
	for ci, c := range cases.Cases {
		t.Run(c.Name, func(t *testing.T) {
			rc := recorded.Cases[ci]
			if rc.Name != c.Name || len(rc.Steps) != len(c.Steps) {
				t.Fatalf("recorded.json does not match cases.json for %q: record again", c.Name)
			}
			vars := map[string]string{}
			for si, s := range c.Steps {
				label := fmt.Sprintf("step %d: %s %s", si+1, s.Method, s.Path)
				status, got := runPresetFidelityStep(t, fake, s, vars)
				compareFidelityStep(t, label, status, got, rc.Steps[si].Status, rc.Steps[si].Response)
			}
		})
	}
}

func runPresetFidelityStep(t *testing.T, fake *presetFake, s fidelityStep, vars map[string]string) (int, any) {
	t.Helper()
	body := s.Body
	if body != nil {
		body = substJSON(t, body, vars)
	}
	fake.mu.Lock()
	status, resp := fake.handle(s.Method, presetBasePath+substString(s.Path, vars), body)
	fake.mu.Unlock()
	got := roundTrip(t, resp)
	if s.Capture != "" && status == 200 {
		preset, _ := got.(map[string]any)["preset"].(map[string]any)
		vars[s.Capture], _ = preset["id"].(string)
	}
	return status, unsubstJSON(t, got, vars)
}

// TestRecordPresetFidelity sends fidelity/cases.json to a real tenant and
// writes fidelity/recorded.json. Set RECORD_PRESET_FIDELITY=1 with
// CORALOGIX_API_KEY and CORALOGIX_ENV (or CX_REGION). Captured preset ids are
// deleted afterwards. A 403 on a custom create means the key cannot record.
func TestRecordPresetFidelity(t *testing.T) {
	if os.Getenv("RECORD_PRESET_FIDELITY") != "1" {
		t.Skip("set RECORD_PRESET_FIDELITY=1 with CORALOGIX_API_KEY and CORALOGIX_ENV to record against a tenant")
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

	rawCases, err := os.ReadFile(filepath.Join(presetGoldenDir, "fidelity", "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases fidelityCases
	if err := json.Unmarshal(rawCases, &cases); err != nil {
		t.Fatal(err)
	}

	vars := map[string]string{}
	var created []string
	t.Cleanup(func() { cleanupRecordedPresets(base, apiKey, created) })

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
			status, got := recordPresetFidelityStep(t, base, apiKey, s, vars, &created)
			if status == http.StatusForbidden && !presetStepMayForbid(s) {
				t.Fatalf("Notification Center returned 403 Permission Denied. Recording needs a management API key that can create a custom preset.")
			}
			if status >= 200 && status < 300 && presetStepMayForbid(s) {
				t.Fatalf("a system preset step succeeded with %d on %s %s", status, s.Method, s.Path)
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
	if err := os.WriteFile(filepath.Join(presetGoldenDir, "fidelity", "recorded.json"), append(raw, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func recordPresetFidelityStep(t *testing.T, base, apiKey string, s fidelityStep, vars map[string]string, created *[]string) (int, any) {
	t.Helper()
	url := strings.TrimRight(base, "/") + presetBasePath + substString(s.Path, vars)
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
	if s.Capture != "" && resp.StatusCode == http.StatusOK {
		preset, _ := parsed.(map[string]any)["preset"].(map[string]any)
		if id, _ := preset["id"].(string); id != "" {
			vars[s.Capture] = id
			*created = append(*created, id)
		}
	}
	return resp.StatusCode, unsubstJSON(t, parsed, vars)
}

// presetStepMayForbid reports a step whose target is a system preset. Those
// calls are in the cases to record the rejection. A custom create is not.
func presetStepMayForbid(s fidelityStep) bool {
	if s.Method == http.MethodDelete && strings.Contains(s.Path, "/custom/preset_system_") {
		return true
	}
	if s.Method != http.MethodPut {
		return false
	}
	body, _ := s.Body.(map[string]any)
	preset, _ := body["preset"].(map[string]any)
	id, _ := preset["id"].(string)
	return strings.HasPrefix(id, "preset_system_")
}

func cleanupRecordedPresets(base, apiKey string, ids []string) {
	client := http.DefaultClient
	for _, id := range ids {
		if id == "" || strings.HasPrefix(id, "preset_system_") {
			continue
		}
		req, err := http.NewRequest(http.MethodDelete, strings.TrimRight(base, "/")+presetBasePath+"/custom/"+id, nil)
		if err != nil {
			continue
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}
}

func TestRecordPresetFidelityFromFake(t *testing.T) {
	if os.Getenv("RECORD_PRESET_FIDELITY") != "fake" {
		t.Skip("set RECORD_PRESET_FIDELITY=fake to write recorded.json from the fake")
	}
	rawCases, err := os.ReadFile(filepath.Join(presetGoldenDir, "fidelity", "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases fidelityCases
	if err := json.Unmarshal(rawCases, &cases); err != nil {
		t.Fatal(err)
	}
	fake := &presetFake{}
	fake.reset()
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
			status, got := runPresetFidelityStep(t, fake, s, vars)
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
	if err := os.WriteFile(filepath.Join(presetGoldenDir, "fidelity", "recorded.json"), append(raw, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}
