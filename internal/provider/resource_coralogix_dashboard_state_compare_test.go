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

package provider

import (
	"strings"
	"testing"
)

func TestDashboardOpenAPIStateAttributeDifferences(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		resource       map[string]string
		dataSource     map[string]string
		wantDifference string
	}{
		{
			name:       "identical states",
			resource:   map[string]string{"id": "abc", "name": "dash"},
			dataSource: map[string]string{"id": "abc", "name": "dash"},
		},
		{
			name:           "different values",
			resource:       map[string]string{"name": "dash"},
			dataSource:     map[string]string{"name": "other"},
			wantDifference: "name",
		},
		{
			name:           "null on one side, empty on the other",
			resource:       map[string]string{},
			dataSource:     map[string]string{"description": ""},
			wantDifference: "description",
		},
		{
			name:       "access_policy with a different key order",
			resource:   map[string]string{"access_policy": `{"a":1,"b":2}`},
			dataSource: map[string]string{"access_policy": `{"b":2,"a":1}`},
		},
		{
			name:           "access_policy present on one side only",
			resource:       map[string]string{"access_policy": `{"a":1}`},
			dataSource:     map[string]string{},
			wantDifference: "access_policy",
		},
		{
			name:           "access_policy holding a different policy",
			resource:       map[string]string{"access_policy": `{"a":1}`},
			dataSource:     map[string]string{"access_policy": `{"a":2}`},
			wantDifference: "access_policy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			differences := dashboardOpenAPIStateAttributeDifferences(tt.resource, tt.dataSource)
			if tt.wantDifference == "" {
				if len(differences) != 0 {
					t.Fatalf("differences = %v, want none", differences)
				}
				return
			}
			if len(differences) != 1 || !strings.Contains(differences[0], tt.wantDifference) {
				t.Fatalf("differences = %v, want one mentioning %q", differences, tt.wantDifference)
			}
		})
	}
}
