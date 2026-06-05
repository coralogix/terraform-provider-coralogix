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

package provider

import (
	"context"
	"fmt"
	"testing"

	quotaRules "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/quota_allocation_rule_set_service"
	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

const quotaAllocationRuleSetResourceName = "coralogix_quota_allocation_rule_set.test"

func TestAccCoralogixResourceQuotaAllocationRuleSet(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccQuotaAllocationRuleSetCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceQuotaAllocationRuleSet(60, 40, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(quotaAllocationRuleSetResourceName, "rules.#", "2"),
					resource.TestCheckTypeSetElemNestedAttrs(quotaAllocationRuleSetResourceName, "rules.*", map[string]string{
						"entity_type":  "logs",
						"allocation":   "60",
						"enabled":      "true",
						"can_overflow": "true",
					}),
					resource.TestCheckTypeSetElemNestedAttrs(quotaAllocationRuleSetResourceName, "rules.*", map[string]string{
						"entity_type":  "metrics",
						"allocation":   "40",
						"enabled":      "true",
						"can_overflow": "false",
					}),
				),
			},
			{
				ResourceName:      quotaAllocationRuleSetResourceName,
				ImportState:       true,
				ImportStateId:     "quota-allocation-rule-set",
				ImportStateVerify: true,
			},
			{
				Config: testAccCoralogixResourceQuotaAllocationRuleSet(55, 45, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckTypeSetElemNestedAttrs(quotaAllocationRuleSetResourceName, "rules.*", map[string]string{
						"entity_type":  "logs",
						"allocation":   "55",
						"enabled":      "false",
						"can_overflow": "true",
					}),
					resource.TestCheckTypeSetElemNestedAttrs(quotaAllocationRuleSetResourceName, "rules.*", map[string]string{
						"entity_type":  "metrics",
						"allocation":   "45",
						"enabled":      "true",
						"can_overflow": "false",
					}),
				),
			},
			{
				Config:   testAccCoralogixResourceQuotaAllocationRuleSet(55, 45, false),
				PlanOnly: true,
			},
		},
	})
}

func testAccQuotaAllocationRuleSetCheckDestroy(s *terraform.State) error {
	meta := testAccProvider.Meta()
	if meta == nil {
		return nil
	}
	client := meta.(*clientset.ClientSet).QuotaAllocationRules()
	ctx := context.TODO()
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "coralogix_quota_allocation_rule_set" {
			continue
		}

		result, _, err := client.QuotaAllocationRuleSetServiceGetQuotaAllocationRuleSet(ctx).Execute()
		if err == nil && quotaAllocationRuleSetHasUserManagedRules(result) {
			return fmt.Errorf("quota allocation rule set still exists: %s", utils.FormatJSON(result))
		}
	}

	return nil
}

func quotaAllocationRuleSetHasUserManagedRules(result *quotaRules.GetQuotaAllocationRuleSetResponse) bool {
	if result == nil || result.RuleSet == nil {
		return false
	}
	for _, rule := range result.RuleSet.GetRules() {
		if !rule.GetCxManaged() {
			return true
		}
	}
	return false
}

func testAccCoralogixResourceQuotaAllocationRuleSet(logsAllocation, metricsAllocation int, logsEnabled bool) string {
	return fmt.Sprintf(`
resource "coralogix_quota_allocation_rule_set" "test" {
  rules = [
    {
      entity_type  = "logs"
      allocation   = %d
      allocation_type = "percentage"
      enabled      = %t
      can_overflow = true
    },
    {
      entity_type  = "metrics"
      allocation   = %d
      allocation_type = "percentage"
      enabled      = true
      can_overflow = false
    }
  ]
}
`, logsAllocation, logsEnabled, metricsAllocation)
}
