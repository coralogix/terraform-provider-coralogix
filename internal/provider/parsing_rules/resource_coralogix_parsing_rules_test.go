// Copyright 2026 Coralogix Ltd.
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

package parsing_rules

import (
	"testing"

	prgs "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/rule_groups_service"
)

func TestFlattenRuleSubGroupsPreservesID(t *testing.T) {
	subgroupID := "subgroup-id"

	subgroups := flattenRuleSubGroups([]prgs.RuleSubgroup{{Id: &subgroupID}})

	if len(subgroups) != 1 {
		t.Fatalf("expected one subgroup, got %d", len(subgroups))
	}
	if got := subgroups[0].ID.ValueString(); got != subgroupID {
		t.Errorf("subgroup ID = %q, want %q", got, subgroupID)
	}
}
