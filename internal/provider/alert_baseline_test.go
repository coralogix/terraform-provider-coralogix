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
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/internal/utils/schemadump"
)

const (
	alertTypeName  = "coralogix_alert"
	alertGoldenDir = "testdata/alert_baseline"
)

// These tests record the behavior of the registered coralogix_alert. After the resource becomes
// generated code, the golden diff must show only listed fixes or message wording.
//
//	UPDATE_GOLDEN=1 go test ./internal/provider -run TestAlertBaseline

func TestAlertBaselineSchema(t *testing.T) {
	checkGoldenAt(t, alertGoldenDir, "schema_current.txt", []byte(schemadump.Text(registeredResourceSchema(t, alertTypeName))))
}

func newAlertHarness(t *testing.T) *grHarness {
	return newProtocolHarness(t, alertTypeName, newAlertFake(t))
}

func TestAlertBaselineScenarios(t *testing.T) {
	for name, run := range alertScenarios {
		t.Run(name, func(t *testing.T) {
			h := newAlertHarness(t)
			run(h)
			checkGoldenAt(t, alertGoldenDir, "scenario_"+name+".json", marshalAlertGolden(t, h.steps))
		})
	}
}

// alertLogsImmediate is the smallest alert: a name and one type.
func alertLogsImmediate(name string, extra ...j) j {
	out := j{
		"name": name,
		"type_definition": j{"logs_immediate": j{
			"logs_filter": j{"simple_filter": j{"lucene_query": "tfbaseline"}},
		}},
	}
	for _, e := range extra {
		out = with(out, e)
	}
	return out
}

// alertScenarios holds every scenario, by name. Each file of scenarios adds its own from init, so
// groups of scenarios never edit the same lines. A scenario runs against the fake for the goldens,
// and against a real tenant for the recording (alert_fidelity_test.go), so it may use only the
// harness and the alertBackend helpers, never the fake directly.
var alertScenarios = map[string]func(h *grHarness){}

func addAlertScenario(name string, run func(h *grHarness)) {
	if _, dup := alertScenarios[name]; dup {
		panic("duplicate alert scenario " + name)
	}
	alertScenarios[name] = run
}

func init() {
	addAlertScenario("logs-immediate-lifecycle", func(h *grHarness) {
		s := h.Apply("create", h.null(), alertLogsImmediate("tfbaseline-logs-immediate"))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: rename", s, alertLogsImmediate("tfbaseline-logs-immediate-2"))
		h.Destroy("destroy", s)
	})
}

// alertBackend is what a scenario may do to the backend outside Terraform. The fake and the live
// recorder both implement it.
type alertBackend interface {
	// seed creates an alert as if it was made outside Terraform and returns its id.
	seed(props map[string]any) string
	// forget deletes an alert without Terraform.
	forget(id string)
	// seedSLO creates a temporary request-based SLO that is grouped by the given labels (none: no
	// grouping) and returns its id. The recorder deletes it at the end of the run.
	seedSLO(by ...string) string
	// forgetSLO deletes an SLO without Terraform. The API deletes the alerts that refer to it too.
	forgetSLO(id string)
}

func (h *grHarness) alertBackend() alertBackend {
	h.t.Helper()
	b, ok := h.api.(alertBackend)
	if !ok {
		h.t.Fatal("the backend is no alertBackend")
	}
	return b
}
