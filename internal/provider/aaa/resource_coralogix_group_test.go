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
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
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
