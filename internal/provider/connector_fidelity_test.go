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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
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

func TestRecordConnectorFidelityFromFake(t *testing.T) {
	if os.Getenv("RECORD_CONNECTOR_FIDELITY") != "1" {
		t.Skip("set RECORD_CONNECTOR_FIDELITY=1 to write recorded.json from the fake")
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
