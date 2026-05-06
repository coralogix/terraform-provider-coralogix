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

package aaa

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestCaseInsensitiveStringPlanModifier(t *testing.T) {
	cases := []struct {
		name      string
		state     types.String
		plan      types.String
		wantPlan  types.String
		wantEqual bool // assert resp.PlanValue == state (i.e. modifier suppressed the diff)
	}{
		{
			name:      "case_only_difference_uses_state",
			state:     types.StringValue("Alice.Example@example.com"),
			plan:      types.StringValue("alice.example@example.com"),
			wantEqual: true,
		},
		{
			name:      "exact_match_uses_state",
			state:     types.StringValue("alice.example@example.com"),
			plan:      types.StringValue("alice.example@example.com"),
			wantEqual: true,
		},
		{
			name:      "different_value_passes_through",
			state:     types.StringValue("alice.example@example.com"),
			plan:      types.StringValue("bob.other@example.com"),
			wantPlan:  types.StringValue("bob.other@example.com"),
			wantEqual: false,
		},
		{
			name:      "null_state_passes_through",
			state:     types.StringNull(),
			plan:      types.StringValue("alice.example@example.com"),
			wantPlan:  types.StringValue("alice.example@example.com"),
			wantEqual: false,
		},
		{
			name:      "null_plan_passes_through",
			state:     types.StringValue("alice.example@example.com"),
			plan:      types.StringNull(),
			wantPlan:  types.StringNull(),
			wantEqual: false,
		},
		{
			name:      "unknown_plan_passes_through",
			state:     types.StringValue("alice.example@example.com"),
			plan:      types.StringUnknown(),
			wantPlan:  types.StringUnknown(),
			wantEqual: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := planmodifier.StringRequest{
				StateValue: tc.state,
				PlanValue:  tc.plan,
			}
			resp := &planmodifier.StringResponse{PlanValue: tc.plan}
			caseInsensitiveStringPlanModifier{}.PlanModifyString(context.Background(), req, resp)

			if tc.wantEqual {
				if !resp.PlanValue.Equal(tc.state) {
					t.Fatalf("expected PlanValue to equal state %v, got %v", tc.state, resp.PlanValue)
				}
				return
			}
			if !resp.PlanValue.Equal(tc.wantPlan) {
				t.Fatalf("expected PlanValue %v, got %v", tc.wantPlan, resp.PlanValue)
			}
		})
	}
}
