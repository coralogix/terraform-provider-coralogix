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
	"context"
	"math/big"
	"testing"

	quotaRules "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/quota_allocation_rule_set_service"
)

func quotaRule(entityType string, allocation float64, allocationType string) quotaRuleConfig {
	return quotaRuleConfig{entityType: entityType, allocation: big.NewFloat(allocation), allocationType: allocationType}
}

func TestValidateQuotaAllocationRulesPercentageAllocationMaximum(t *testing.T) {
	for _, allocationType := range []string{"", "percentage", "unspecified"} {
		if !validateQuotaAllocationRules([]quotaRuleConfig{quotaRule("logs", 101, allocationType)}).HasError() {
			t.Fatalf("expected percentage allocation diagnostic for allocation_type %q", allocationType)
		}
	}
}

func TestValidateQuotaAllocationRulesAllowsLockedUnitsAbovePercentageMaximum(t *testing.T) {
	if diags := validateQuotaAllocationRules([]quotaRuleConfig{quotaRule("logs", 101, "locked_units")}); diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
}

func TestValidateQuotaAllocationRulesSkipsValuesThatAreNotKnown(t *testing.T) {
	diags := validateQuotaAllocationRules([]quotaRuleConfig{{entityType: "logs"}, {allocation: big.NewFloat(50)}})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
}

func TestDropCxManagedQuotaAllocationRules(t *testing.T) {
	managed, notManaged := true, false
	ruleSet := &quotaRules.QuotaAllocationEntityTypeRuleSet{Rules: []quotaRules.QuotaAllocationEntityTypeRule{
		{EntityType: "alerts", CxManaged: &managed},
		{EntityType: "logs"},
		{EntityType: "metrics", CxManaged: &notManaged},
	}}
	if diags := dropCxManagedQuotaAllocationRules(context.Background(), ruleSet); diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if len(ruleSet.Rules) != 2 || ruleSet.Rules[0].EntityType != "logs" || ruleSet.Rules[1].EntityType != "metrics" {
		t.Fatalf("expected the customer rules, got %v", ruleSet.Rules)
	}
}

func TestDropCxManagedQuotaAllocationRulesKeepsEmptyRulesEmpty(t *testing.T) {
	ruleSet := &quotaRules.QuotaAllocationEntityTypeRuleSet{}
	dropCxManagedQuotaAllocationRules(context.Background(), ruleSet)
	if ruleSet.Rules == nil || len(ruleSet.Rules) != 0 {
		t.Fatalf("expected an empty list, got %#v", ruleSet.Rules)
	}
}

func TestValidateQuotaAllocationRulesDefersPercentageMaximumForUnknownAllocationType(t *testing.T) {
	rule := quotaRule("logs", 500, "")
	rule.allocationTypeUnknown = true
	if diags := validateQuotaAllocationRules([]quotaRuleConfig{rule}); diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
}

func TestDropCxManagedQuotaAllocationRulesReadsUnspecifiedAsPercentage(t *testing.T) {
	unspecified := quotaRules.QUOTAALLOCATIONTYPE_QUOTA_ALLOCATION_TYPE_UNSPECIFIED
	locked := quotaRules.QUOTAALLOCATIONTYPE_QUOTA_ALLOCATION_TYPE_LOCKED_UNITS
	ruleSet := &quotaRules.QuotaAllocationEntityTypeRuleSet{Rules: []quotaRules.QuotaAllocationEntityTypeRule{
		{EntityType: "logs"},
		{EntityType: "metrics", AllocationType: &unspecified},
		{EntityType: "spans", AllocationType: &locked},
	}}
	dropCxManagedQuotaAllocationRules(context.Background(), ruleSet)
	got := []quotaRules.QuotaAllocationType{ruleSet.Rules[0].GetAllocationType(), ruleSet.Rules[1].GetAllocationType(), ruleSet.Rules[2].GetAllocationType()}
	want := []quotaRules.QuotaAllocationType{quotaRules.QUOTAALLOCATIONTYPE_QUOTA_ALLOCATION_TYPE_PERCENTAGE, quotaRules.QUOTAALLOCATIONTYPE_QUOTA_ALLOCATION_TYPE_PERCENTAGE, locked}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("rule %d: got %v, want %v", i, got[i], want[i])
		}
	}
}
