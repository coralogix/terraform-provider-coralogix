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

package dataplans

import (
	"testing"

	quotaRules "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/quota_allocation_rule_set_service"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestExpandQuotaAllocationRuleSet(t *testing.T) {
	plan := QuotaAllocationRuleSetModel{
		ID: types.StringValue("rule-set-id"),
		Rules: []QuotaAllocationRuleModel{
			{
				EntityType:  types.StringValue("metrics"),
				Allocation:  types.Float64Value(25),
				Enabled:     types.BoolValue(true),
				CanOverflow: types.BoolValue(false),
			},
			{
				EntityType:  types.StringValue("logs"),
				Allocation:  types.Float64Value(75),
				Enabled:     types.BoolValue(false),
				CanOverflow: types.BoolValue(true),
			},
		},
	}

	ruleSet, diags := expandQuotaAllocationRuleSet(plan)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if ruleSet.GetId() != "rule-set-id" {
		t.Fatalf("expected id to round-trip, got %q", ruleSet.GetId())
	}
	if len(ruleSet.Rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(ruleSet.Rules))
	}
	if ruleSet.Rules[0].GetEntityType() != "logs" {
		t.Fatalf("expected rules to be sorted by entity type, got %q first", ruleSet.Rules[0].GetEntityType())
	}
	if ruleSet.Rules[0].GetAllocation() != 75 {
		t.Fatalf("expected logs allocation 75, got %v", ruleSet.Rules[0].GetAllocation())
	}
	if !ruleSet.Rules[0].GetCanOverflow() {
		t.Fatal("expected logs can_overflow to be true")
	}
}

func TestExpandQuotaAllocationRuleSetSyntheticImportID(t *testing.T) {
	plan := QuotaAllocationRuleSetModel{
		ID: types.StringValue(quotaAllocationRuleSetImportID),
		Rules: []QuotaAllocationRuleModel{
			{
				EntityType:  types.StringValue("logs"),
				Allocation:  types.Float64Value(100),
				Enabled:     types.BoolValue(true),
				CanOverflow: types.BoolValue(false),
			},
		},
	}

	ruleSet, diags := expandQuotaAllocationRuleSet(plan)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if ruleSet.HasId() {
		t.Fatalf("synthetic import id should not be sent to the API, got %q", ruleSet.GetId())
	}
}

func TestValidateQuotaAllocationRulesRejectsDuplicateEntityType(t *testing.T) {
	diags := validateQuotaAllocationRules([]QuotaAllocationRuleModel{
		{
			EntityType: types.StringValue("logs"),
		},
		{
			EntityType: types.StringValue("logs"),
		},
	})

	if !diags.HasError() {
		t.Fatal("expected duplicate entity type diagnostic")
	}
}

func TestFlattenQuotaAllocationRuleSet(t *testing.T) {
	id := "rule-set-id"
	state, diags := flattenQuotaAllocationRuleSet(&quotaRules.QuotaAllocationEntityTypeRuleSet{
		Id: &id,
		Rules: []quotaRules.QuotaAllocationEntityTypeRule{
			{
				EntityType:  "metrics",
				Allocation:  10,
				Enabled:     true,
				CanOverflow: true,
			},
			{
				EntityType:  "logs",
				Allocation:  90,
				Enabled:     false,
				CanOverflow: false,
			},
		},
	})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if state.ID.ValueString() != "rule-set-id" {
		t.Fatalf("expected id to round-trip, got %q", state.ID.ValueString())
	}
	if len(state.Rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(state.Rules))
	}
	if state.Rules[0].EntityType.ValueString() != "logs" {
		t.Fatalf("expected rules to be sorted by entity type, got %q first", state.Rules[0].EntityType.ValueString())
	}
	if state.Rules[0].Allocation.ValueFloat64() != 90 {
		t.Fatalf("expected logs allocation 90, got %v", state.Rules[0].Allocation.ValueFloat64())
	}
}

func TestFlattenQuotaAllocationRuleSetUsesSyntheticID(t *testing.T) {
	state, diags := flattenQuotaAllocationRuleSet(&quotaRules.QuotaAllocationEntityTypeRuleSet{
		Rules: []quotaRules.QuotaAllocationEntityTypeRule{},
	})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if state.ID.ValueString() != quotaAllocationRuleSetImportID {
		t.Fatalf("expected synthetic id, got %q", state.ID.ValueString())
	}
}
