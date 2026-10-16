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

package alerts

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	alertscheduler "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/alert_scheduler_rule_service"
	"github.com/coralogix/terraform-provider-coralogix/internal/provider/generated/alertsscheduler"

	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// Update reads the rule from the replace response, while the released resource read it again.
// In the recording of the real API, every replace response and the read after it give one state.
func TestUpdateResponseGivesTheStateOfTheNextRead(t *testing.T) {
	dir := filepath.Join("..", "testdata", "alerts_scheduler_baseline", "fidelity")
	var cases struct {
		Cases []struct {
			Name  string `json:"name"`
			Steps []struct {
				Method string `json:"method"`
			} `json:"steps"`
		} `json:"cases"`
	}
	var recorded struct {
		Cases []struct {
			Steps []struct {
				Status   int             `json:"status"`
				Response json.RawMessage `json:"response"`
			} `json:"steps"`
		} `json:"cases"`
	}
	for name, target := range map[string]any{"cases.json": &cases, "recorded.json": &recorded} {
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, target); err != nil {
			t.Fatal(err)
		}
	}
	pairs := 0
	for ci, c := range cases.Cases {
		for si := 0; si+1 < len(c.Steps); si++ {
			put, get := recorded.Cases[ci].Steps[si], recorded.Cases[ci].Steps[si+1]
			if c.Steps[si].Method != "PUT" || c.Steps[si+1].Method != "GET" || put.Status != 200 || get.Status != 200 {
				continue
			}
			pairs++
			if a, b := ruleState(t, put.Response), ruleState(t, get.Response); !a.Equal(b) {
				t.Errorf("%s step %d: the replace response gives another state than the next read:\n%s\n%s", c.Name, si+1, a, b)
			}
		}
	}
	if pairs < 6 {
		t.Fatalf("the recording has %d replace and read pairs, want at least 6", pairs)
	}
}

// ruleState flattens the rule of a recorded response into Terraform state, as Update and Read do.
func ruleState(t *testing.T, response json.RawMessage) tftypes.Value {
	t.Helper()
	var body struct {
		AlertSchedulerRule alertscheduler.AlertSchedulerRule `json:"alertSchedulerRule"`
	}
	if err := json.Unmarshal(response, &body); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if diags := normalizeStartTimes(ctx, &body.AlertSchedulerRule); diags.HasError() {
		t.Fatal(diags)
	}
	m, diags := alertsscheduler.Flatten(ctx, &body.AlertSchedulerRule)
	if diags.HasError() {
		t.Fatal(diags)
	}
	state := tfsdk.State{Schema: alertsscheduler.Schema()}
	if diags := state.Set(ctx, m); diags.HasError() {
		t.Fatal(diags)
	}
	return state.Raw
}
