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

// An empty member list from the API has to come back as whichever emptiness the caller asked for.
// Terraform requires the value returned from an apply to equal the value that was planned, and an
// empty set is not a null set, so getting this wrong makes one of the two configurations impossible
// to apply. Both directions are covered because fixing either one in isolation breaks the other.
func TestFlattenSCIMGroupMembersEmptyList(t *testing.T) {
	emptySet, diags := types.SetValue(types.StringType, []attr.Value{})
	if diags.HasError() {
		t.Fatalf("building an empty set: %v", diags)
	}
	populatedSet, diags := types.SetValue(types.StringType, []attr.Value{types.StringValue("user-1")})
	if diags.HasError() {
		t.Fatalf("building a populated set: %v", diags)
	}

	for _, tc := range []struct {
		name       string
		configured types.Set
		wantNull   bool
	}{
		{
			// `members = []`: the configuration says "no members", so state must say the same.
			name:       "explicit empty set stays empty",
			configured: emptySet,
			wantNull:   false,
		},
		{
			// The attribute is absent, which is a different value from an empty set.
			name:       "absent attribute stays null",
			configured: types.SetNull(types.StringType),
			wantNull:   true,
		},
		{
			// Unknown at plan time (a member id computed elsewhere) resolves like an absent value:
			// there is nothing to echo back yet.
			name:       "unknown attribute becomes null",
			configured: types.SetUnknown(types.StringType),
			wantNull:   true,
		},
		{
			// The group was emptied outside Terraform. Refresh must report the emptiness rather than
			// keep the stale member, so the next plan can propose the change.
			name:       "members removed elsewhere are reported as empty",
			configured: populatedSet,
			wantNull:   false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, diags := flattenSCIMGroupMembers(nil, tc.configured)
			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}
			if got.IsNull() != tc.wantNull {
				t.Fatalf("IsNull() = %t, want %t (value: %s)", got.IsNull(), tc.wantNull, got)
			}
			if !tc.wantNull && len(got.Elements()) != 0 {
				t.Fatalf("expected an empty set, got %s", got)
			}
		})
	}
}

// A non-empty list is unambiguous and must be returned as-is whatever the caller had.
func TestFlattenSCIMGroupMembersPopulatedList(t *testing.T) {
	members := []clientset.SCIMGroupMember{{Value: "user-1"}, {Value: "user-2"}}

	for name, configured := range map[string]types.Set{
		"from null":  types.SetNull(types.StringType),
		"from empty": types.SetValueMust(types.StringType, []attr.Value{}),
	} {
		t.Run(name, func(t *testing.T) {
			got, diags := flattenSCIMGroupMembers(members, configured)
			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}
			if got.IsNull() {
				t.Fatal("a group with members must not flatten to null")
			}
			if len(got.Elements()) != 2 {
				t.Fatalf("expected 2 members, got %s", got)
			}
		})
	}
}
