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

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func scimUser(id, userName string) cxsdk.SCIMUser {
	return cxsdk.SCIMUser{ID: &id, UserName: userName}
}

func TestMatchSCIMUsersByUserName(t *testing.T) {
	cases := []struct {
		name     string
		users    []cxsdk.SCIMUser
		userName string
		wantIDs  []string
	}{
		{
			name:     "exact_match",
			users:    []cxsdk.SCIMUser{scimUser("1", "alice@example.com"), scimUser("2", "bob@example.com")},
			userName: "alice@example.com",
			wantIDs:  []string{"1"},
		},
		{
			name:     "case_insensitive_match",
			users:    []cxsdk.SCIMUser{scimUser("1", "alice@example.com")},
			userName: "Alice@Example.com",
			wantIDs:  []string{"1"},
		},
		{
			name:     "no_match_is_not_the_first_result",
			users:    []cxsdk.SCIMUser{scimUser("1", "alice@example.com"), scimUser("2", "bob@example.com")},
			userName: "carol@example.com",
			wantIDs:  []string{},
		},
		{
			name:     "prefix_is_not_a_match",
			users:    []cxsdk.SCIMUser{scimUser("1", "alice@example.com.br")},
			userName: "alice@example.com",
			wantIDs:  []string{},
		},
		{
			name:     "ambiguous_matches_are_all_returned",
			users:    []cxsdk.SCIMUser{scimUser("1", "alice@example.com"), scimUser("2", "ALICE@example.com")},
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
			got := scimUserIDs(matchSCIMUsersByUserName(tc.users, tc.userName))
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

func TestSCIMUserIDsSkipsMissingIDs(t *testing.T) {
	users := []cxsdk.SCIMUser{{UserName: "alice@example.com"}, scimUser("2", "bob@example.com")}

	got := scimUserIDs(users)
	if len(got) != 1 || got[0] != "2" {
		t.Fatalf("scimUserIDs = %v, want [2]", got)
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
