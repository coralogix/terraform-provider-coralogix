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
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestUpgradeGroupStateV0ToV1(t *testing.T) {
	ctx := context.Background()

	upgrader, ok := (&GroupResource{}).UpgradeState(ctx)[0]
	if !ok {
		t.Fatal("no state upgrader registered for schema version 0")
	}
	if upgrader.PriorSchema == nil {
		t.Fatal("version 0 state upgrader has no PriorSchema")
	}
	priorSchema := *upgrader.PriorSchema

	var schemaResp resource.SchemaResponse
	(&GroupResource{}).Schema(ctx, resource.SchemaRequest{}, &schemaResp)
	currentSchema := schemaResp.Schema

	if currentSchema.Version != 1 {
		t.Fatalf("current schema version = %d, want 1", currentSchema.Version)
	}

	for name, members := range map[string][]string{
		"members adopted by the version 0 read are dropped": {"user-1", "user-2"},
		"an already-empty members value stays null":         nil,
	} {
		t.Run(name, func(t *testing.T) {
			raw := groupV0RawState(ctx, t, priorSchema, members)

			priorState := tfsdk.State{Raw: raw, Schema: priorSchema}
			resp := resource.UpgradeStateResponse{State: tfsdk.State{Schema: currentSchema}}
			upgrader.StateUpgrader(ctx, resource.UpgradeStateRequest{State: &priorState}, &resp)
			if resp.Diagnostics.HasError() {
				t.Fatalf("state upgrade diagnostics = %v", resp.Diagnostics)
			}

			var upgradedMembers types.Set
			if diags := resp.State.GetAttribute(ctx, path.Root("members"), &upgradedMembers); diags.HasError() {
				t.Fatalf("read members: %v", diags)
			}
			if !upgradedMembers.IsNull() {
				t.Errorf("members = %s, want null", upgradedMembers)
			}

			var displayName types.String
			if diags := resp.State.GetAttribute(ctx, path.Root("display_name"), &displayName); diags.HasError() {
				t.Fatalf("read display_name: %v", diags)
			}
			if displayName.ValueString() != "group-under-upgrade" {
				t.Errorf("display_name = %q, want %q", displayName.ValueString(), "group-under-upgrade")
			}
		})
	}
}

func groupV0RawState(ctx context.Context, t *testing.T, priorSchema schema.Schema, members []string) tftypes.Value {
	t.Helper()

	terraformType := priorSchema.Type().TerraformType(ctx)
	objectType, ok := terraformType.(tftypes.Object)
	if !ok {
		t.Fatalf("group v0 schema Terraform type is %T, want tftypes.Object", terraformType)
	}

	membersValue := tftypes.NewValue(objectType.AttributeTypes["members"], nil)
	if members != nil {
		elements := make([]tftypes.Value, 0, len(members))
		for _, member := range members {
			elements = append(elements, tftypes.NewValue(tftypes.String, member))
		}
		membersValue = tftypes.NewValue(objectType.AttributeTypes["members"], elements)
	}

	return tftypes.NewValue(objectType, map[string]tftypes.Value{
		"id":           tftypes.NewValue(tftypes.String, "4242"),
		"display_name": tftypes.NewValue(tftypes.String, "group-under-upgrade"),
		"members":      membersValue,
		"role":         tftypes.NewValue(tftypes.String, "Read Only"),
		"scope_id":     tftypes.NewValue(tftypes.String, nil),
	})
}

func TestMembersMatchingPrior(t *testing.T) {
	memberSet := types.SetValueMust(types.StringType, []attr.Value{types.StringValue("user-1")})
	emptySet := types.SetValueMust(types.StringType, []attr.Value{})

	for name, tc := range map[string]struct {
		prior     types.Set
		flattened types.Set
		expected  types.Set
	}{
		"null plan keeps state null even when the backend reports members": {
			prior:     types.SetNull(types.StringType),
			flattened: memberSet,
			expected:  types.SetNull(types.StringType),
		},
		"null plan stays null when the backend reports no members": {
			prior:     types.SetNull(types.StringType),
			flattened: types.SetNull(types.StringType),
			expected:  types.SetNull(types.StringType),
		},
		"empty plan round-trips as an empty set": {
			prior:     emptySet,
			flattened: types.SetNull(types.StringType),
			expected:  emptySet,
		},
		"populated plan keeps the backend members": {
			prior:     memberSet,
			flattened: memberSet,
			expected:  memberSet,
		},
	} {
		t.Run(name, func(t *testing.T) {
			if got := membersMatchingPrior(tc.prior, tc.flattened); !got.Equal(tc.expected) {
				t.Errorf("membersMatchingPrior(%s, %s) = %s, want %s", tc.prior, tc.flattened, got, tc.expected)
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
}
