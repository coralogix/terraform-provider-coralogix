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
	"fmt"
	"testing"

	teamGroups "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/team_groups_management_service"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestFlattenTeamGroup(t *testing.T) {
	t.Parallel()

	groupID := int64(42)
	name := "example"
	roleName := "Read Only"
	scopeID := "scope-1"
	group := &teamGroups.TeamGroup{
		GroupId: &groupID,
		Name:    &name,
		Role:    &teamGroups.Role{Name: &roleName},
		Scope:   &teamGroups.V2Scope{ScopeId: &scopeID},
	}

	model, diags := flattenTeamGroup(group, []string{"user-a", "user-b"})
	if diags.HasError() {
		t.Fatalf("flattenTeamGroup diagnostics: %v", diags)
	}
	if model.ID.ValueString() != "42" {
		t.Errorf("ID = %q", model.ID.ValueString())
	}
	if model.DisplayName.ValueString() != name {
		t.Errorf("DisplayName = %q", model.DisplayName.ValueString())
	}
	if model.Role.ValueString() != roleName {
		t.Errorf("Role = %q", model.Role.ValueString())
	}
	if model.ScopeID.ValueString() != scopeID {
		t.Errorf("ScopeID = %q", model.ScopeID.ValueString())
	}
	if model.Members.IsNull() {
		t.Fatal("Members is null")
	}
}

func TestFlattenTeamGroupEmptyMembersAndScope(t *testing.T) {
	t.Parallel()

	groupID := int64(7)
	name := "empty"
	group := &teamGroups.TeamGroup{
		GroupId: &groupID,
		Name:    &name,
	}

	model, diags := flattenTeamGroup(group, nil)
	if diags.HasError() {
		t.Fatalf("flattenTeamGroup diagnostics: %v", diags)
	}
	if !model.Members.IsNull() {
		t.Errorf("Members = %#v, want null", model.Members)
	}
	if !model.ScopeID.IsNull() {
		t.Errorf("ScopeID = %q, want null", model.ScopeID.ValueString())
	}
	if model.Role.ValueString() != "" {
		t.Errorf("Role = %q, want empty", model.Role.ValueString())
	}
}

func TestMembershipAddRemove(t *testing.T) {
	t.Parallel()

	add, remove := userIDDiff([]string{"a", "b"}, []string{"b", "c"})
	if !sameStrings(add, []string{"a"}) {
		t.Errorf("add = %v", add)
	}
	if !sameStrings(remove, []string{"c"}) {
		t.Errorf("remove = %v", remove)
	}
}

func TestDesiredAttachmentGroupUserIDs(t *testing.T) {
	t.Parallel()

	got := desiredAttachmentGroupUserIDs([]string{"alice", "bob"}, []string{"bob"}, []string{"carol"})
	if !sameStrings(got, []string{"alice", "carol"}) {
		t.Errorf("replace attachment user: got %v", got)
	}

	got = desiredAttachmentGroupUserIDs([]string{"alice", "bob"}, []string{"bob"}, []string{"bob", "carol"})
	if !sameStrings(got, []string{"alice", "bob", "carol"}) {
		t.Errorf("add attachment user: got %v", got)
	}

	got = desiredAttachmentGroupUserIDs([]string{"alice", "bob"}, []string{"bob"}, []string{})
	if !sameStrings(got, []string{"alice"}) {
		t.Errorf("clear attachment users: got %v", got)
	}

	got = desiredAttachmentGroupUserIDs(nil, nil, []string{"carol"})
	if !sameStrings(got, []string{"carol"}) {
		t.Errorf("empty group: got %v", got)
	}
}

func TestIdsToAddAndIntersect(t *testing.T) {
	t.Parallel()

	add := userIDsToAdd([]string{"a", "b"}, []string{"b"})
	if !sameStrings(add, []string{"a"}) {
		t.Errorf("userIDsToAdd = %v", add)
	}
	kept := userIDsToRemove([]string{"a", "c"}, []string{"a", "b"})
	if !sameStrings(kept, []string{"a"}) {
		t.Errorf("userIDsToRemove = %v", kept)
	}
}

func TestRoleNameForStatePrefersConfiguredAlias(t *testing.T) {
	t.Parallel()

	if got := roleNameForState("Legacy Read Only", "Read Only"); got != "Read Only" {
		t.Errorf("preferred alias: got %q", got)
	}
	if got := roleNameForState("Legacy Read Only", ""); got != "Read Only" {
		t.Errorf("import reverse alias: got %q", got)
	}
	if got := roleNameForState("Read-Only User", ""); got != "Read-Only User" {
		t.Errorf("unrelated role: got %q", got)
	}
}

func TestScopeUpdateFromPlan(t *testing.T) {
	t.Parallel()

	if got := scopeUpdateFromPlan(&GroupResourceModel{ScopeID: types.StringUnknown()}); got != nil {
		t.Errorf("unknown: got %#v, want nil", got)
	}
	if got := scopeUpdateFromPlan(&GroupResourceModel{ScopeID: types.StringNull()}); got != nil {
		t.Errorf("null: got %#v, want nil", got)
	}
	if got := scopeUpdateFromPlan(&GroupResourceModel{ScopeID: types.StringValue("")}); got != nil {
		t.Errorf("empty: got %#v, want nil", got)
	}
	got := scopeUpdateFromPlan(&GroupResourceModel{ScopeID: types.StringValue("scope-1")})
	if got == nil || got.Action == nil || got.Action.SetScopeId == nil || got.Action.SetScopeId.GetValue() != "scope-1" {
		t.Errorf("set: got %#v", got)
	}
}

func TestIsGroupNotFoundErr(t *testing.T) {
	t.Parallel()

	if !isGroupNotFoundErr(&groupNotFoundError{id: 7}) {
		t.Fatal("typed error")
	}
	if !isGroupNotFoundErr(fmt.Errorf("wrap: %w", &groupNotFoundError{id: 7})) {
		t.Fatal("wrapped typed error")
	}
	if isGroupNotFoundErr(fmt.Errorf("group 7 not found")) {
		t.Fatal("plain string must not match")
	}
}

func TestFlattenMemberIDsNullWhenEmpty(t *testing.T) {
	t.Parallel()

	got, diags := flattenMemberIDs(nil)
	if diags.HasError() {
		t.Fatalf("diagnostics: %v", diags)
	}
	if !got.Equal(types.SetNull(types.StringType)) {
		t.Errorf("got %#v, want null set", got)
	}
}

func sameStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	counts := map[string]int{}
	for _, v := range want {
		counts[v]++
	}
	for _, v := range got {
		counts[v]--
		if counts[v] < 0 {
			return false
		}
	}
	return true
}
