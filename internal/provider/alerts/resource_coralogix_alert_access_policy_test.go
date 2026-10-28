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
	"encoding/json"
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
		readable     bool
		want         types.String
	}{
		{
			name:         "no policy on the alert, read authorized: state clears",
			current:      types.StringNull(),
			accessPolicy: nil,
			readable:     true,
			want:         types.StringNull(),
		},
		{
			name:         "an equivalent configured text is preserved as written",
			current:      types.StringValue(prettyText),
			accessPolicy: &backendText,
			readable:     true,
			want:         types.StringValue(prettyText),
		},
		{
			name:         "an unknown current stores the canonical text",
			current:      types.StringUnknown(),
			accessPolicy: &backendText,
			readable:     true,
			want:         types.StringValue(canonicalText),
		},
		{
			name:         "import and the data source store the canonical text",
			current:      types.StringNull(),
			accessPolicy: &backendText,
			readable:     true,
			want:         types.StringValue(canonicalText),
		},
		{
			name:         "a policy changed outside Terraform stores the canonical text",
			current:      types.StringValue(`{"version":"2024-01-01","rules":[]}`),
			accessPolicy: &backendText,
			readable:     true,
			want:         types.StringValue(canonicalText),
		},
		{
			name:         "characters jsonencode escapes are stored escaped",
			current:      types.StringNull(),
			accessPolicy: &ampersandText,
			readable:     true,
			want:         types.StringValue(`{"rules":[{"name":"reads \u0026 writes"}]}`),
		},
		{
			name:         "text this provider cannot parse is stored unchanged",
			current:      types.StringNull(),
			accessPolicy: &unparsableText,
			readable:     true,
			want:         types.StringValue("not json"),
		},
		{
			// The key lacks alerts:ReadAccessPolicy (or a create/replace
			// response echoed no access). Policy presence is unknown: clearing
			// the state would drop a policy the apply just wrote, so the prior
			// value is kept. Absent means unknown, not deleted.
			name:         "an unreadable absent policy preserves the prior value",
			current:      types.StringValue(canonicalText),
			accessPolicy: nil,
			readable:     false,
			want:         types.StringValue(canonicalText),
		},
		{
			name:         "an unreadable absent policy with no prior value stays null",
			current:      types.StringNull(),
			accessPolicy: nil,
			readable:     false,
			want:         types.StringNull(),
		},
		{
			// Authoritative "no policy" from an authorized read: the remote
			// deletion is real, so drift shows and the next apply re-applies.
			name:         "an authorized absent policy clears the prior value",
			current:      types.StringValue(canonicalText),
			accessPolicy: nil,
			readable:     true,
			want:         types.StringNull(),
		},
		{
			// State after a "" delete: the "" sentinel remains the record of
			// the successful removal and an authorized no-policy read keeps it.
			name:         "an authorized absent policy keeps the empty sentinel",
			current:      types.StringValue(""),
			accessPolicy: nil,
			readable:     true,
			want:         types.StringNull(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, diags := flattenAlertAccessPolicy(tt.current, tt.accessPolicy, tt.readable)
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
			// UseStateForUnknown carries the prior policy into the plan when
			// the configuration line is removed; the action must still be
			// omitted (removal from config relinquishes management, Option A).
			name:   "removed from configuration with the prior value carried into the plan is omitted",
			config: types.StringNull(),
			plan:   types.StringValue(value),
			check: func(t *testing.T, got *alerts.AccessPolicyAction) {
				t.Helper()
				if got != nil {
					t.Fatalf("action = %v, want nil", got)
				}
			},
		},
		{
			// Same, after a "" delete: state "" carried into the plan.
			name:   "removed from configuration with the empty sentinel carried into the plan is omitted",
			config: types.StringNull(),
			plan:   types.StringValue(empty),
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
				// Wire-format regression: a nil map would serialize as an
				// empty oneof object and be dropped by the backend (the
				// operator hit exactly this trap). Assert the actual JSON.
				raw, err := json.Marshal(&alerts.ReplaceAlertDefinitionRequest{AccessPolicy: got})
				if err != nil {
					t.Fatalf("marshal replace request: %v", err)
				}
				var wire map[string]interface{}
				if err := json.Unmarshal(raw, &wire); err != nil {
					t.Fatalf("unmarshal replace request: %v", err)
				}
				access, ok := wire["accessPolicy"].(map[string]interface{})
				if !ok {
					t.Fatalf("wire accessPolicy = %v, want a delete object", wire["accessPolicy"])
				}
				if _, hasDelete := access["delete"]; !hasDelete {
					t.Fatalf("wire accessPolicy = %v, want {\"delete\":{}}", access)
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

	if got, readable := alertAccessFromGetResponse(nil); got != nil || readable {
		t.Fatalf("nil response = (%v, %v), want (nil, false)", got, readable)
	}
	if got, readable := alertAccessFromGetResponse(withoutAccess); got != nil || readable {
		t.Fatalf("response without access = (%v, %v), want (nil, false): presence unknown", got, readable)
	}
	if got, readable := alertAccessFromGetResponse(withoutPolicy); got != nil || !readable {
		t.Fatalf("access without policy = (%v, %v), want (nil, true): authoritative absence", got, readable)
	}
	if got, readable := alertAccessFromGetResponse(withPolicy); got == nil || *got != policy || !readable {
		t.Fatalf("access with policy = (%v, %v), want (%q, true)", got, readable, policy)
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
