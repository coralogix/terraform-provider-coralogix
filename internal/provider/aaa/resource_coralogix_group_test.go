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

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestMembersForState(t *testing.T) {
	memberSet := types.SetValueMust(types.StringType, []attr.Value{types.StringValue("user-1")})
	emptySet := types.SetValueMust(types.StringType, []attr.Value{})

	for name, tc := range map[string]struct {
		planned   types.Set
		flattened types.Set
		expected  types.Set
	}{
		"an unknown plan takes the group's actual membership": {
			planned:   types.SetUnknown(types.StringType),
			flattened: memberSet,
			expected:  memberSet,
		},
		"an unknown plan accepts an empty membership": {
			planned:   types.SetUnknown(types.StringType),
			flattened: emptySet,
			expected:  emptySet,
		},
		"a known empty plan is kept so clearing round-trips": {
			planned:   emptySet,
			flattened: emptySet,
			expected:  emptySet,
		},
		"a known plan wins over a backend value read mid-apply": {
			planned:   memberSet,
			flattened: emptySet,
			expected:  memberSet,
		},
	} {
		t.Run(name, func(t *testing.T) {
			if got := membersForState(tc.planned, tc.flattened); !got.Equal(tc.expected) {
				t.Errorf("membersForState(%s, %s) = %s, want %s", tc.planned, tc.flattened, got, tc.expected)
			}
		})
	}
}

// The members plan modifier must retain prior state only when the argument is omitted.
// A configured value that Terraform has still to resolve has to keep its diff, or the
// member list is never applied.
func TestGroupMembersPlanModifierRetainsStateOnlyWhenOmitted(t *testing.T) {
	ctx := context.Background()

	var schemaResp resource.SchemaResponse
	(&GroupResource{}).Schema(ctx, resource.SchemaRequest{}, &schemaResp)
	groupSchema := schemaResp.Schema

	membersAttr, ok := groupSchema.Attributes["members"].(schema.SetAttribute)
	if !ok {
		t.Fatalf("members attribute is %T, want schema.SetAttribute", groupSchema.Attributes["members"])
	}
	if len(membersAttr.PlanModifiers) != 1 {
		t.Fatalf("members has %d plan modifiers, want 1", len(membersAttr.PlanModifiers))
	}
	modifier := membersAttr.PlanModifiers[0]

	stateMembers := types.SetValueMust(types.StringType, []attr.Value{types.StringValue("existing-user")})
	unresolvedElement := types.SetValueMust(types.StringType, []attr.Value{types.StringUnknown()})

	for name, tc := range map[string]struct {
		config   types.Set
		plan     types.Set
		expected types.Set
	}{
		"an omitted argument retains the stored members": {
			config:   types.SetNull(types.StringType),
			plan:     types.SetUnknown(types.StringType),
			expected: stateMembers,
		},
		"a wholly unresolved configured value keeps its diff": {
			config:   types.SetUnknown(types.StringType),
			plan:     types.SetUnknown(types.StringType),
			expected: types.SetUnknown(types.StringType),
		},
		"a configured value holding an unresolved id keeps its diff": {
			config:   unresolvedElement,
			plan:     unresolvedElement,
			expected: unresolvedElement,
		},
	} {
		t.Run(name, func(t *testing.T) {
			resp := planmodifier.SetResponse{PlanValue: tc.plan}
			modifier.PlanModifySet(ctx, planmodifier.SetRequest{
				State:       groupStateForPlanModifier(ctx, t, groupSchema),
				StateValue:  stateMembers,
				ConfigValue: tc.config,
				PlanValue:   tc.plan,
			}, &resp)

			if !resp.PlanValue.Equal(tc.expected) {
				t.Errorf("planned members = %s, want %s", resp.PlanValue, tc.expected)
			}
		})
	}
}

func groupStateForPlanModifier(ctx context.Context, t *testing.T, groupSchema schema.Schema) tfsdk.State {
	t.Helper()

	objectType, ok := groupSchema.Type().TerraformType(ctx).(tftypes.Object)
	if !ok {
		t.Fatalf("group schema Terraform type is %T, want tftypes.Object", groupSchema.Type().TerraformType(ctx))
	}

	return tfsdk.State{
		Schema: groupSchema,
		Raw: tftypes.NewValue(objectType, map[string]tftypes.Value{
			"id":           tftypes.NewValue(tftypes.String, "4242"),
			"display_name": tftypes.NewValue(tftypes.String, "existing-group"),
			"members": tftypes.NewValue(objectType.AttributeTypes["members"], []tftypes.Value{
				tftypes.NewValue(tftypes.String, "existing-user"),
			}),
			"role":     tftypes.NewValue(tftypes.String, "Read Only"),
			"scope_id": tftypes.NewValue(tftypes.String, nil),
		}),
	}
}

func TestMembersUnmanaged(t *testing.T) {
	for name, tc := range map[string]struct {
		configured types.Set
		expected   bool
	}{
		"an omitted argument leaves membership unmanaged": {
			configured: types.SetNull(types.StringType),
			expected:   true,
		},
		"an unresolved value was still configured": {
			configured: types.SetUnknown(types.StringType),
			expected:   false,
		},
		"an explicit empty list manages membership as empty": {
			configured: types.SetValueMust(types.StringType, []attr.Value{}),
			expected:   false,
		},
		"an explicit list manages membership": {
			configured: types.SetValueMust(types.StringType, []attr.Value{types.StringValue("user-1")}),
			expected:   false,
		},
	} {
		t.Run(name, func(t *testing.T) {
			if got := membersUnmanaged(tc.configured); got != tc.expected {
				t.Errorf("membersUnmanaged(%s) = %t, want %t", tc.configured, got, tc.expected)
			}
		})
	}
}

func TestFlattenSCIMGroupMembers(t *testing.T) {
	members, diags := flattenSCIMGroupMembers([]clientset.SCIMGroupMember{{Value: "user-1"}})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	expected := types.SetValueMust(types.StringType, []attr.Value{types.StringValue("user-1")})
	if !members.Equal(expected) {
		t.Errorf("flattenSCIMGroupMembers = %s, want %s", members, expected)
	}

	// A computed member list is always known, so no members is an empty set rather than null.
	empty, diags := flattenSCIMGroupMembers(nil)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if empty.IsNull() {
		t.Error("flattenSCIMGroupMembers(nil) = null, want an empty set")
	}
	if len(empty.Elements()) != 0 {
		t.Errorf("flattenSCIMGroupMembers(nil) = %s, want an empty set", empty)
	}
}
