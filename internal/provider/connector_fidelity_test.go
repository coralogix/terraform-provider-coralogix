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
	"crypto/rand"
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

// TestConnectorFakeMatchesRecordedBackend checks the fake connector backend
// against recorded.json. Record that file against a tenant, then keep the fake
// in line with those answers. This test skips when recorded.json does not exist.
func TestConnectorFakeMatchesRecordedBackend(t *testing.T) {
	cases, recorded := loadFidelityAt(t, cxGoldenDir)
	fake := &cxFake{connectors: map[string]map[string]any{}}
	for ci, c := range cases.Cases {
		t.Run(c.Name, func(t *testing.T) {
			rc := recorded.Cases[ci]
			if rc.Name != c.Name || len(rc.Steps) != len(c.Steps) {
				t.Fatalf("recorded.json does not match cases.json for %q: record again", c.Name)
			}
			vars := map[string]string{}
			for si, s := range c.Steps {
				label := fmt.Sprintf("step %d: %s %s", si+1, s.Method, s.Path)
				status, got := runConnectorFidelityStep(t, fake, s, vars)
				compareFidelityStep(t, label, status, got, rc.Steps[si].Status, rc.Steps[si].Response)
			}
		})
	}
}

func runConnectorFidelityStep(t *testing.T, fake *cxFake, s fidelityStep, vars map[string]string) (int, any) {
	t.Helper()
	body := s.Body
	if body != nil {
		body = substJSON(t, body, vars)
	}
	fake.mu.Lock()
	status, resp := fake.handle(s.Method, cxBasePath+substString(s.Path, vars), body)
	fake.mu.Unlock()
	got := roundTrip(t, resp)
	if s.Capture != "" && status == 200 {
		connector, _ := got.(map[string]any)["connector"].(map[string]any)
		vars[s.Capture], _ = connector["id"].(string)
	}
	return status, unsubstJSON(t, got, vars)
}

// TestRecordConnectorFidelity sends fidelity/cases.json to a real tenant and
// writes fidelity/recorded.json. Set RECORD_CONNECTOR_FIDELITY=1 with
// CORALOGIX_API_KEY and CORALOGIX_ENV (or CX_REGION). Connector ids are replaced
// with the case tokens before the file is written.
func TestRecordConnectorFidelity(t *testing.T) {
	if os.Getenv("RECORD_CONNECTOR_FIDELITY") != "1" {
		t.Skip("set RECORD_CONNECTOR_FIDELITY=1 with CORALOGIX_API_KEY and CORALOGIX_ENV to record against a tenant")
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

	rawCases, err := os.ReadFile(filepath.Join(cxGoldenDir, "fidelity", "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases fidelityCases
	if err := json.Unmarshal(rawCases, &cases); err != nil {
		t.Fatal(err)
	}

	vars := newConnectorFidelityVars()
	t.Cleanup(func() { cleanupRecordedConnectors(base, apiKey, vars) })

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
			status, got := recordConnectorFidelityStep(t, base, apiKey, s, vars)
			if status == http.StatusForbidden {
				t.Fatalf("Notification Center returned 403 Permission Denied. Recording needs a management API key with connector permissions.")
			}
			rec.Steps = append(rec.Steps, struct {
				Status   int `json:"status"`
				Response any `json:"response"`
			}{Status: status, Response: sanitizeRecordedConnector(got)})
		}
		out.Cases = append(out.Cases, rec)
	}
	raw, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cxGoldenDir, "fidelity", "recorded.json"), append(raw, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func newConnectorFidelityVars() map[string]string {
	vars := map[string]string{
		"MISSING": "00000000-0000-4000-8000-999999999999",
	}
	for _, name := range []string{"C1", "C2", "C3", "C4", "C5", "C6"} {
		vars[name] = newUUID()
	}
	return vars
}

func recordConnectorFidelityStep(t *testing.T, base, apiKey string, s fidelityStep, vars map[string]string) (int, any) {
	t.Helper()
	url := strings.TrimRight(base, "/") + cxBasePath + substString(s.Path, vars)
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
		connector, _ := parsed.(map[string]any)["connector"].(map[string]any)
		if id, _ := connector["id"].(string); id != "" {
			vars[s.Capture] = id
		}
	}
	return resp.StatusCode, unsubstJSON(t, parsed, vars)
}

func cleanupRecordedConnectors(base, apiKey string, vars map[string]string) {
	client := http.DefaultClient
	for _, name := range []string{"C1", "C2", "C3", "C4", "C5", "C6", "GEN"} {
		id := vars[name]
		if id == "" {
			continue
		}
		req, err := http.NewRequest(http.MethodDelete, strings.TrimRight(base, "/")+cxBasePath+"/"+id, nil)
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

func sanitizeRecordedConnector(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, e := range x {
			if k == "teamId" {
				out[k] = float64(1)
				continue
			}
			out[k] = sanitizeRecordedConnector(e)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = sanitizeRecordedConnector(e)
		}
		return out
	}
	return v
}

func TestRecordConnectorFidelityFromFake(t *testing.T) {
	if os.Getenv("RECORD_CONNECTOR_FIDELITY") != "fake" {
		t.Skip("set RECORD_CONNECTOR_FIDELITY=fake to write recorded.json from the fake")
	}
	rawCases, err := os.ReadFile(filepath.Join(cxGoldenDir, "fidelity", "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases fidelityCases
	if err := json.Unmarshal(rawCases, &cases); err != nil {
		t.Fatal(err)
	}
	fake := &cxFake{connectors: map[string]map[string]any{}}
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
			status, got := runConnectorFidelityStep(t, fake, s, vars)
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
	if err := os.WriteFile(filepath.Join(cxGoldenDir, "fidelity", "recorded.json"), append(raw, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func newUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
