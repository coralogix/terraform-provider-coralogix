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
	"fmt"
	"net/http/httptest"
	"testing"
)

// Replay the anonymized EU2 recording against the fake. The recorder stays outside the repo.
func TestScopeFakeMatchesRecordedBackend(t *testing.T) {
	cases, recorded := loadFidelityAt(t, scopeGoldenDir)
	for ci, c := range cases.Cases {
		t.Run(c.Name, func(t *testing.T) {
			fake := &scopeFake{scopes: map[string]map[string]any{}}
			rc := recorded.Cases[ci]
			if rc.Name != c.Name || len(rc.Steps) != len(c.Steps) {
				t.Fatal("recorded.json does not match cases.json")
			}
			vars := map[string]string{"name": "tf-fidelity-scope"}
			for si, s := range c.Steps {
				var body map[string]any
				if s.Body != nil {
					body = substJSON(t, s.Body, vars).(map[string]any)
				}
				req := httptest.NewRequest(s.Method, scopeBasePath+substString(s.Path, vars), nil)
				status, response := fake.handle(req, body)
				got := roundTrip(t, response)
				if s.Capture != "" && status == 200 {
					vars[s.Capture] = got.(map[string]any)["scope"].(map[string]any)["id"].(string)
				}
				compareFidelityStep(t, fmt.Sprintf("step %d: %s %s", si+1, s.Method, s.Path),
					status, unsubstJSON(t, got, vars), rc.Steps[si].Status, rc.Steps[si].Response)
			}
		})
	}
}
