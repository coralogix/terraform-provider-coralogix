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
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newBareDashboardFake is the fake without a server. The replay calls handle directly.
func newBareDashboardFake() *dashboardFake {
	return &dashboardFake{
		dashboards: map[string]map[string]any{},
		policies:   map[string]string{},
		faults:     map[string][]dashboardFault{},
		sticky:     map[string]dashboardFault{},
		replies:    map[string][]any{},
		folders:    &dashboardsFolderFake{folders: map[string]map[string]any{}, seeded: map[string]bool{}},
	}
}

// Replay the anonymized EU2 recording against the fake. The recorder stays outside the repo:
// scratch/dashboard-probe/record_fidelity.py. A step that differs names a place where the fake is not the
// API, and the baseline goldens are then not a record of the API.
func TestDashboardFakeMatchesRecordedBackend(t *testing.T) {
	cases, recorded := loadFidelityAt(t, dashboardGoldenDir)
	for ci, c := range cases.Cases {
		t.Run(c.Name, func(t *testing.T) {
			fake := newBareDashboardFake()
			rc := recorded.Cases[ci]
			if rc.Name != c.Name || len(rc.Steps) != len(c.Steps) {
				t.Fatal("recorded.json does not match cases.json")
			}
			vars := map[string]string{"name": "tf-fidelity-dashboard-test", "rand": "abcde12345"}
			for si, s := range c.Steps {
				label := fmt.Sprintf("step %d: %s %s", si+1, s.Method, s.Path)
				if rc.Steps[si].Status == 0 {
					// The recorder skips a step that needs the id of a create that failed.
					continue
				}
				var body map[string]any
				if s.Body != nil {
					body = substJSON(t, s.Body, vars).(map[string]any)
				}
				if s.Path == "/check/v1" {
					fake.issues = recordedIssues(rc.Steps[si].Response)
				}
				req := httptest.NewRequest(s.Method, dashboardBasePath+substString(s.Path, vars), nil)
				status, response := fake.handle(req, body)
				got := roundTrip(t, response)
				if s.Capture != "" && status == 200 {
					vars[s.Capture] = got.(map[string]any)["dashboardId"].(string)
				}
				want := recordedIDsAsVars(rc.Steps[si].Response)
				got = recordedIDsAsVars(unsubstJSON(t, got, vars))
				if parserMessage(want) && status == rc.Steps[si].Status {
					// The API's parser message names a protobuf type and a position. The fake does
					// not model them, so only the status counts.
					continue
				}
				compareFidelityStep(t, label, status, policyAsJSON(got), rc.Steps[si].Status, policyAsJSON(want))
			}
		})
	}
}

// recordedIssues returns the issues of a recorded Check answer. The fake cannot find issues on its own,
// so the replay hands it the recorded ones and only compares the shape of the answer.
func recordedIssues(response any) []any {
	m, _ := response.(map[string]any)
	issues, _ := m["issues"].([]any)
	return issues
}

// The cases carry synthetic values only: fixed placeholder ids, and the run name as a placeholder.
func TestDashboardFidelityCasesAreSynthetic(t *testing.T) {
	cases := loadDashboardFidelityCases(t)
	for _, c := range cases.Cases {
		for _, s := range c.Steps {
			raw := fmt.Sprint(s.Body)
			if dashboardUUIDPattern.MatchString(raw) {
				for _, id := range dashboardUUIDPattern.FindAllString(raw, -1) {
					if len(id) != 36 || id[:23] != "00000000-0000-4000-8000" {
						t.Errorf("%s: %q is not a synthetic id", c.Name, id)
					}
				}
			}
		}
	}
}

func loadDashboardFidelityCases(t *testing.T) fidelityCases {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dashboardGoldenDir, "fidelity", "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases fidelityCases
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	return cases
}

// parserMessage is an error of the API's protobuf parser, such as "oneof ... is already set".
func parserMessage(response any) bool {
	return strings.Contains(errorMessage(response), "Bad Request: proto:")
}

// policyAsJSON turns the accessPolicy text of an answer into JSON, so two policies compare by value.
// The API writes the keys in a different order on every read.
func policyAsJSON(response any) any {
	m, ok := response.(map[string]any)
	if !ok {
		return response
	}
	text, _ := m["accessPolicy"].(string)
	if text == "" {
		return response
	}
	var parsed any
	if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		return response
	}
	out := map[string]any{}
	for k, v := range m {
		out[k] = v
	}
	out["accessPolicy"] = parsed
	return out
}

// recordedIDsAsVars undoes an ordering quirk of the first recorder: it replaced the random part of the
// sent id before the id itself, so the sent id stayed as text with a {{rand}} placeholder in it.
func recordedIDsAsVars(response any) any {
	raw, err := json.Marshal(response)
	if err != nil {
		panic(err)
	}
	var out any
	if err := json.Unmarshal([]byte(strings.ReplaceAll(string(raw), "fidelity{{rand}}abc", "{{id}}")), &out); err != nil {
		panic(err)
	}
	return out
}
