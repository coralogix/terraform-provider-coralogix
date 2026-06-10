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

package dashboard_schema

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestCanonicalizeDashboardAccessPolicy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		policy string
		want   string
	}{
		{
			name: "pretty object",
			policy: `{
  "version": "2025-01-01",
  "default": {
    "permissions": {
      "team-dashboards:Update": "deny",
      "team-dashboards:Read": "grant"
    }
  },
  "rules": []
}`,
			want: `{"default":{"permissions":{"team-dashboards:Read":"grant","team-dashboards:Update":"deny"}},"rules":[],"version":"2025-01-01"}`,
		},
		{
			name:   "reordered object keys",
			policy: `{"rules":[],"version":"2025-01-01","default":{"permissions":{"team-dashboards:Update":"deny","team-dashboards:Read":"grant"}}}`,
			want:   `{"default":{"permissions":{"team-dashboards:Read":"grant","team-dashboards:Update":"deny"}},"rules":[],"version":"2025-01-01"}`,
		},
		{
			name:   "nested objects",
			policy: `{"z":{"b":2,"a":1},"a":{"d":{"y":true,"x":false},"c":"value"}}`,
			want:   `{"a":{"c":"value","d":{"x":false,"y":true}},"z":{"a":1,"b":2}}`,
		},
		{
			name:   "array order is preserved",
			policy: `{"rules":[{"id":"second"},{"id":"first"}],"version":"2025-01-01"}`,
			want:   `{"rules":[{"id":"second"},{"id":"first"}],"version":"2025-01-01"}`,
		},
		{
			name:   "explicit null and unknown keys",
			policy: `{"version":"2025-01-01","unknown":{"nullable":null,"kept":true},"rules":[]}`,
			want:   `{"rules":[],"unknown":{"kept":true,"nullable":null},"version":"2025-01-01"}`,
		},
		{
			name:   "top level array",
			policy: `[{"b":2,"a":1}]`,
			want:   `[{"a":1,"b":2}]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, diags := CanonicalizeDashboardAccessPolicyJSON(tt.policy)
			if diags.HasError() {
				t.Fatalf("CanonicalizeDashboardAccessPolicyJSON returned diagnostics: %v", diags)
			}
			if got != tt.want {
				t.Fatalf("CanonicalizeDashboardAccessPolicyJSON() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCanonicalizeDashboardAccessPolicyRejectsInvalidJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		policy string
	}{
		{name: "empty", policy: ""},
		{name: "malformed", policy: `{"version":`},
		{name: "trailing value", policy: `{"version":"2025-01-01"} {"rules":[]}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got, diags := CanonicalizeDashboardAccessPolicyJSON(tt.policy); !diags.HasError() {
				t.Fatalf("CanonicalizeDashboardAccessPolicyJSON() = %q with no diagnostics, want error diagnostics", got)
			}
		})
	}
}

func TestDashboardAccessPolicyCanonicalJSONPlanModifier(t *testing.T) {
	t.Parallel()

	req := planmodifier.StringRequest{
		PlanValue: types.StringValue(`{"rules":[],"version":"2025-01-01","default":{"permissions":{"team-dashboards:Update":"deny","team-dashboards:Read":"grant"}}}`),
	}
	resp := &planmodifier.StringResponse{}

	DashboardAccessPolicyCanonicalJSONPlanModifier{}.PlanModifyString(context.Background(), req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("PlanModifyString returned diagnostics: %v", resp.Diagnostics)
	}

	want := `{"default":{"permissions":{"team-dashboards:Read":"grant","team-dashboards:Update":"deny"}},"rules":[],"version":"2025-01-01"}`
	if got := resp.PlanValue.ValueString(); got != want {
		t.Fatalf("PlanModifyString plan value = %q, want %q", got, want)
	}
}
