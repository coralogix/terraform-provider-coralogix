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

package alerts

import (
	"testing"

	alerts "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/alert_definitions_service"
	alertschema "github.com/coralogix/terraform-provider-coralogix/internal/provider/alerts/alert_schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestFlattenAlertAccessPolicy(t *testing.T) {
	// The backend's key order for the same policy a jsonencode-based
	// configuration would produce.
	backendText := `{"version":"2025-01-01","default":{"permissions":{"alerts:Read":"grant"}},"rules":[]}`
	canonicalText := `{"default":{"permissions":{"alerts:Read":"grant"}},"rules":[],"version":"2025-01-01"}`
	unparsableText := "not json"
	// jsonencode escapes &, so the stored text must escape it too.
	ampersandText := `{"rules":[{"name":"reads & writes"}]}`
	prettyText := "{\n  \"version\": \"2025-01-01\",\n  \"default\": {\n    \"permissions\": {\n      \"alerts:Read\": \"grant\"\n    }\n  },\n  \"rules\": []\n}"

	tests := []struct {
		name         string
		current      types.String
		accessPolicy *string
		want         types.String
	}{
		{
			name:         "no policy on the alert",
			current:      types.StringNull(),
			accessPolicy: nil,
			want:         types.StringNull(),
		},
		{
			name:         "an equivalent configured text is preserved as written",
			current:      types.StringValue(prettyText),
			accessPolicy: &backendText,
			want:         types.StringValue(prettyText),
		},
		{
			name:         "an unknown current stores the canonical text",
			current:      types.StringUnknown(),
			accessPolicy: &backendText,
			want:         types.StringValue(canonicalText),
		},
		{
			name:         "import and the data source store the canonical text",
			current:      types.StringNull(),
			accessPolicy: &backendText,
			want:         types.StringValue(canonicalText),
		},
		{
			name:         "a policy changed outside Terraform stores the canonical text",
			current:      types.StringValue(`{"version":"2024-01-01","rules":[]}`),
			accessPolicy: &backendText,
			want:         types.StringValue(canonicalText),
		},
		{
			name:         "characters jsonencode escapes are stored escaped",
			current:      types.StringNull(),
			accessPolicy: &ampersandText,
			want:         types.StringValue(`{"rules":[{"name":"reads \u0026 writes"}]}`),
		},
		{
			name:         "text this provider cannot parse is stored unchanged",
			current:      types.StringNull(),
			accessPolicy: &unparsableText,
			want:         types.StringValue("not json"),
		},
		{
			// The read sends no include_access_permissions flag, so an
			// absent policy also means the caller may not read policies.
			// Clearing the state would drop a policy the apply just wrote,
			// so the prior value is kept: absent is unknown, not deleted.
			name:         "an absent policy preserves the prior value",
			current:      types.StringValue(canonicalText),
			accessPolicy: nil,
			want:         types.StringValue(canonicalText),
		},
		{
			name:         "an absent policy with no prior value stays null",
			current:      types.StringNull(),
			accessPolicy: nil,
			want:         types.StringNull(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, diags := flattenAlertAccessPolicy(tt.current, tt.accessPolicy)
			if diags.HasError() {
				t.Fatalf("diagnostics: %v", diags)
			}
			if !got.Equal(tt.want) {
				t.Fatalf("access_policy = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestAlertAccessPolicyForCreate(t *testing.T) {
	value := "policy"
	tests := []struct {
		name string
		plan types.String
		want *string
	}{
		{name: "null is omitted", plan: types.StringNull(), want: nil},
		{name: "unknown is omitted", plan: types.StringUnknown(), want: nil},
		{name: "empty is omitted", plan: types.StringValue(""), want: nil},
		{name: "value is sent", plan: types.StringValue(value), want: &value},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := alertAccessPolicyForCreate(tt.plan)
			if tt.want == nil {
				if got != nil {
					t.Fatalf("access policy = %q, want nil", *got)
				}
				return
			}
			if got == nil || *got != *tt.want {
				t.Fatalf("access policy = %v, want %q", got, *tt.want)
			}
		})
	}
}

func TestAlertAccessPolicyActionForUpdate(t *testing.T) {
	value := "policy"
	empty := ""
	tests := []struct {
		name   string
		config types.String
		plan   types.String
		check  func(t *testing.T, got *alerts.AccessPolicyAction)
	}{
		{
			name:   "removed from configuration is omitted",
			config: types.StringNull(),
			plan:   types.StringNull(),
			check: func(t *testing.T, got *alerts.AccessPolicyAction) {
				t.Helper()
				if got != nil {
					t.Fatalf("action = %v, want nil", got)
				}
			},
		},
		{
			name:   "unknown config is omitted",
			config: types.StringUnknown(),
			plan:   types.StringUnknown(),
			check: func(t *testing.T, got *alerts.AccessPolicyAction) {
				t.Helper()
				if got != nil {
					t.Fatalf("action = %v, want nil", got)
				}
			},
		},
		{
			name:   "unknown plan is omitted",
			config: types.StringValue(value),
			plan:   types.StringUnknown(),
			check: func(t *testing.T, got *alerts.AccessPolicyAction) {
				t.Helper()
				if got != nil {
					t.Fatalf("action = %v, want nil", got)
				}
			},
		},
		{
			name:   "empty string deletes the remote policy",
			config: types.StringValue(empty),
			plan:   types.StringValue(empty),
			check: func(t *testing.T, got *alerts.AccessPolicyAction) {
				t.Helper()
				if got == nil {
					t.Fatal("action is nil, want delete")
				}
				if got.Upsert != nil {
					t.Fatalf("upsert = %q, want nil", *got.Upsert)
				}
				if got.Delete == nil {
					t.Fatal("delete is nil, want non-nil marker")
				}
			},
		},
		{
			name:   "value upserts the policy",
			config: types.StringValue(value),
			plan:   types.StringValue(value),
			check: func(t *testing.T, got *alerts.AccessPolicyAction) {
				t.Helper()
				if got == nil {
					t.Fatal("action is nil, want upsert")
				}
				if got.Delete != nil {
					t.Fatalf("delete = %v, want nil", got.Delete)
				}
				if got.Upsert == nil || *got.Upsert != value {
					t.Fatalf("upsert = %v, want %q", got.Upsert, value)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.check(t, alertAccessPolicyActionForUpdate(tt.config, tt.plan))
		})
	}
}

func TestAlertAccessFromGetResponse(t *testing.T) {
	policy := "policy"
	withPolicy := &alerts.GetAlertDefResponse{
		Access: &alerts.AlertDefAccess{AccessPolicy: &policy},
	}
	withoutAccess := &alerts.GetAlertDefResponse{}
	withoutPolicy := &alerts.GetAlertDefResponse{Access: &alerts.AlertDefAccess{}}

	if got := alertAccessFromGetResponse(nil); got != nil {
		t.Fatalf("nil response = %q, want nil", *got)
	}
	if got := alertAccessFromGetResponse(withoutAccess); got != nil {
		t.Fatalf("response without access = %q, want nil", *got)
	}
	if got := alertAccessFromGetResponse(withoutPolicy); got != nil {
		t.Fatalf("access without policy = %q, want nil", *got)
	}
	if got := alertAccessFromGetResponse(withPolicy); got == nil || *got != policy {
		t.Fatalf("access with policy = %v, want %q", got, policy)
	}
}

func TestAlertAccessPolicySchema(t *testing.T) {
	attr, ok := alertschema.V3().Attributes["access_policy"]
	if !ok {
		t.Fatal("V3 schema has no access_policy attribute")
	}
	strAttr, ok := attr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("access_policy is %T, want schema.StringAttribute", attr)
	}
	if !strAttr.Optional {
		t.Error("access_policy is not optional")
	}
	if !strAttr.Computed {
		t.Error("access_policy is not computed")
	}
	if strAttr.MarkdownDescription == "" {
		t.Error("access_policy has no description documenting the set/delete/unmanaged semantics")
	}
}
