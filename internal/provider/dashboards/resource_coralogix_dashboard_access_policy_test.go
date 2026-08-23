// Copyright 2026 Coralogix Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dashboards

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestFlattenDashboardAccessPolicy(t *testing.T) {
	// The backend's key order for the same policy a jsonencode-based
	// configuration would produce.
	backendText := `{"version":"2025-01-01","default":{"permissions":{"team-dashboards:Read":"grant"}},"rules":[]}`
	canonicalText := `{"default":{"permissions":{"team-dashboards:Read":"grant"}},"rules":[],"version":"2025-01-01"}`
	unparsableText := "not json"
	// jsonencode escapes &, so the stored text must escape it too.
	ampersandText := `{"rules":[{"name":"reads & writes"}]}`
	prettyText := "{\n  \"version\": \"2025-01-01\",\n  \"default\": {\n    \"permissions\": {\n      \"team-dashboards:Read\": \"grant\"\n    }\n  },\n  \"rules\": []\n}"

	tests := []struct {
		name         string
		plan         types.String
		accessPolicy *string
		want         types.String
	}{
		{
			name:         "no policy on the dashboard",
			plan:         types.StringNull(),
			accessPolicy: nil,
			want:         types.StringNull(),
		},
		{
			name:         "an equivalent configured text is preserved as written",
			plan:         types.StringValue(prettyText),
			accessPolicy: &backendText,
			want:         types.StringValue(prettyText),
		},
		{
			name:         "an unknown plan stores the canonical text",
			plan:         types.StringUnknown(),
			accessPolicy: &backendText,
			want:         types.StringValue(canonicalText),
		},
		{
			name:         "import and the data source store the canonical text",
			plan:         types.StringNull(),
			accessPolicy: &backendText,
			want:         types.StringValue(canonicalText),
		},
		{
			name:         "a policy changed outside Terraform stores the canonical text",
			plan:         types.StringValue(`{"version":"2024-01-01","rules":[]}`),
			accessPolicy: &backendText,
			want:         types.StringValue(canonicalText),
		},
		{
			name:         "characters jsonencode escapes are stored escaped",
			plan:         types.StringNull(),
			accessPolicy: &ampersandText,
			want:         types.StringValue(`{"rules":[{"name":"reads \u0026 writes"}]}`),
		},
		{
			name:         "text this provider cannot parse is stored unchanged",
			plan:         types.StringNull(),
			accessPolicy: &unparsableText,
			want:         types.StringValue("not json"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, diags := flattenDashboardAccessPolicy(tt.plan, tt.accessPolicy)
			if diags.HasError() {
				t.Fatalf("diagnostics: %v", diags)
			}
			if !got.Equal(tt.want) {
				t.Fatalf("access_policy = %s, want %s", got, tt.want)
			}
		})
	}
}
