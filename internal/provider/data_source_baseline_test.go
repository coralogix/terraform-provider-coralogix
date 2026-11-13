// Copyright 2026 Coralogix Ltd.
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
	"testing"
)

// These tests record the data sources of the generated resources through the plugin protocol,
// against the fake backends of the resource baselines. When a data source becomes generated code,
// the golden diff must show only listed fixes or message wording.
//
//	UPDATE_GOLDEN=1 go test ./internal/provider -run TestDataSourceBaseline

// dataSourceBaseline is the data source of one resource: its golden directory, a harness with the
// fake backend, and the scenarios.
type dataSourceBaseline struct {
	dir       string
	harness   func(t *testing.T) *grHarness
	scenarios map[string]func(h *grHarness)
}

var dataSourceBaselines = map[string]dataSourceBaseline{
	"scope":       {scopeGoldenDir, newScopeHarness, scopeDataSourceScenarios},
	"custom_role": {customRoleGoldenDir, newCustomRoleHarness, customRoleDataSourceScenarios},
	"user":        {userGoldenDir, newUserHarness, userDataSourceScenarios},
}

func TestDataSourceBaselineSchema(t *testing.T) {
	for name, baseline := range dataSourceBaselines {
		t.Run(name, func(t *testing.T) {
			h := baseline.harness(t)
			if h.dsSchema == nil {
				t.Fatalf("%s has no data source", h.typeName)
			}
			checkGoldenAt(t, baseline.dir, "datasource_schema.txt", []byte(dataSourceSchemaText(h.dsSchema)))
		})
	}
}

func TestDataSourceBaselineScenarios(t *testing.T) {
	for name, baseline := range dataSourceBaselines {
		for scenario, run := range baseline.scenarios {
			t.Run(name+"/"+scenario, func(t *testing.T) {
				h := baseline.harness(t)
				run(h)
				if name == "user" {
					canonicalizeUserStepValues(h.steps)
				}
				checkGoldenAt(t, baseline.dir, "datasource_"+scenario+".json", marshalDataSourceGolden(t, name, h.steps))
			})
		}
	}
}
