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
