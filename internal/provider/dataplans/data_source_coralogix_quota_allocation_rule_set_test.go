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
)

func TestFlattenQuotaAllocationRuleSetDataSourceIncludesCxManaged(t *testing.T) {
	cxManaged := true
	state, diags := flattenQuotaAllocationRuleSetDataSource(&quotaRules.QuotaAllocationEntityTypeRuleSet{
		Rules: []quotaRules.QuotaAllocationEntityTypeRule{
			{
				EntityType:  "metrics",
				Allocation:  10,
				CxManaged:   &cxManaged,
				Enabled:     true,
				CanOverflow: true,
			},
		},
	})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if len(state.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(state.Rules))
	}
	if !state.Rules[0].CxManaged.ValueBool() {
		t.Fatal("expected data source cx_managed to be true")
	}
}

func TestFlattenQuotaAllocationRuleSetDataSourceDefaultsOmittedCxManagedToFalse(t *testing.T) {
	state, diags := flattenQuotaAllocationRuleSetDataSource(&quotaRules.QuotaAllocationEntityTypeRuleSet{
		Rules: []quotaRules.QuotaAllocationEntityTypeRule{
			{
				EntityType:  "metrics",
				Allocation:  10,
				Enabled:     true,
				CanOverflow: true,
			},
		},
	})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if len(state.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(state.Rules))
	}
	if state.Rules[0].CxManaged.IsNull() {
		t.Fatal("expected data source cx_managed to default to false, got null")
	}
	if state.Rules[0].CxManaged.ValueBool() {
		t.Fatal("expected data source cx_managed to be false")
	}
}

func TestFlattenQuotaAllocationRuleSetDataSourceRoundsFloat32Allocation(t *testing.T) {
	state, diags := flattenQuotaAllocationRuleSetDataSource(&quotaRules.QuotaAllocationEntityTypeRuleSet{
		Rules: []quotaRules.QuotaAllocationEntityTypeRule{
			{
				EntityType:  "logs",
				Allocation:  33.33,
				Enabled:     true,
				CanOverflow: true,
			},
		},
	})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if got := state.Rules[0].Allocation.ValueFloat64(); got != 33.33 {
		t.Fatalf("expected allocation 33.33, got %.17f", got)
	}
}
