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

	users "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/users_management_service"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func searchedUser(id, userName string) users.RbacV2User {
	return users.RbacV2User{UserId: &id, Username: &userName}
}

// The Users API username filter can match partially, so the provider compares the
// username itself. A prefix is not a match and letter case is ignored.
func TestMatchUsersByUsername(t *testing.T) {
	cases := []struct {
		name     string
		users    []users.RbacV2User
		userName string
		wantIDs  []string
	}{
		{
			name:     "exact_match",
			users:    []users.RbacV2User{searchedUser("1", "alice@example.com"), searchedUser("2", "bob@example.com")},
			userName: "alice@example.com",
			wantIDs:  []string{"1"},
		},
		{
			name:     "case_insensitive_match",
			users:    []users.RbacV2User{searchedUser("1", "alice@example.com")},
			userName: "Alice@Example.com",
			wantIDs:  []string{"1"},
		},
		{
			name:     "no_match_is_not_the_first_result",
			users:    []users.RbacV2User{searchedUser("1", "alice@example.com"), searchedUser("2", "bob@example.com")},
			userName: "carol@example.com",
			wantIDs:  []string{},
		},
		{
			name:     "prefix_is_not_a_match",
			users:    []users.RbacV2User{searchedUser("1", "alice@example.com.br")},
			userName: "alice@example.com",
			wantIDs:  []string{},
		},
		{
			name:     "ambiguous_matches_are_all_returned",
			users:    []users.RbacV2User{searchedUser("1", "alice@example.com"), searchedUser("2", "ALICE@example.com")},
			userName: "alice@example.com",
			wantIDs:  []string{"1", "2"},
		},
		{
			name:     "empty_list",
			users:    nil,
			userName: "alice@example.com",
			wantIDs:  []string{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := userIDs(matchUsersByUsername(tc.users, tc.userName))
			if len(got) != len(tc.wantIDs) {
				t.Fatalf("matched ids = %v, want %v", got, tc.wantIDs)
			}
			for i, id := range tc.wantIDs {
				if got[i] != id {
					t.Fatalf("matched ids = %v, want %v", got, tc.wantIDs)
				}
			}
		})
	}
}

func TestUserIDsSkipsMissingIDs(t *testing.T) {
	userName := "alice@example.com"
	candidates := []users.RbacV2User{{Username: &userName}, searchedUser("2", "bob@example.com")}

	got := userIDs(candidates)
	if len(got) != 1 || got[0] != "2" {
		t.Fatalf("userIDs = %v, want [2]", got)
	}
}

func TestIsKnownString(t *testing.T) {
	cases := []struct {
		name  string
		value types.String
		want  bool
	}{
		{name: "known", value: types.StringValue("alice@example.com"), want: true},
		{name: "empty_is_known", value: types.StringValue(""), want: true},
		{name: "null", value: types.StringNull(), want: false},
		{name: "unknown", value: types.StringUnknown(), want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isKnownString(tc.value); got != tc.want {
				t.Fatalf("isKnownString(%v) = %t, want %t", tc.value, got, tc.want)
			}
		})
	}
}
