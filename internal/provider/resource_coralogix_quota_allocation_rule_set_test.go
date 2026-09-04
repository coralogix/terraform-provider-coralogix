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

	"github.com/coralogix/terraform-provider-coralogix/internal/ephemeralteam"

	quotaRules "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/quota_allocation_rule_set_service"
	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

const quotaAllocationRuleSetResourceName = "coralogix_quota_allocation_rule_set.test"

func TestAccCoralogixResourceQuotaAllocationRuleSet(t *testing.T) {
	providerConfig := ephemeralteam.ProviderConfig(t)
	checkDestroy := testAccQuotaAllocationRuleSetCheckDestroy
	if providerConfig != "" {
		checkDestroy = nil
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             checkDestroy,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + testAccCoralogixResourceQuotaAllocationRuleSet(60, 40, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(quotaAllocationRuleSetResourceName, "rules.#", "2"),
					resource.TestCheckTypeSetElemNestedAttrs(quotaAllocationRuleSetResourceName, "rules.*", map[string]string{
						"entity_type":     "logs",
						"allocation":      "60",
						"allocation_type": "percentage",
						"enabled":         "true",
						"can_overflow":    "true",
					}),
					resource.TestCheckTypeSetElemNestedAttrs(quotaAllocationRuleSetResourceName, "rules.*", map[string]string{
						"entity_type":     "metrics",
						"allocation":      "40",
						"allocation_type": "percentage",
						"enabled":         "true",
						"can_overflow":    "false",
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
				Config: providerConfig + testAccCoralogixResourceQuotaAllocationRuleSet(55, 45, false),
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
				Config:   providerConfig + testAccCoralogixResourceQuotaAllocationRuleSet(55, 45, false),
				PlanOnly: true,
			},
			{
				Config: providerConfig + testAccCoralogixResourceQuotaAllocationRuleSetMixed(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(quotaAllocationRuleSetResourceName, "rules.#", "4"),
					resource.TestCheckTypeSetElemNestedAttrs(quotaAllocationRuleSetResourceName, "rules.*", map[string]string{
						"entity_type":     "logs",
						"allocation":      "50",
						"allocation_type": "percentage",
						"enabled":         "true",
						"can_overflow":    "true",
					}),
					resource.TestCheckTypeSetElemNestedAttrs(quotaAllocationRuleSetResourceName, "rules.*", map[string]string{
						"entity_type":     "spans",
						"allocation":      "0.001",
						"allocation_type": "locked_units",
						"enabled":         "true",
						"can_overflow":    "false",
					}),
					resource.TestCheckTypeSetElemNestedAttrs(quotaAllocationRuleSetResourceName, "rules.*", map[string]string{
						"entity_type":     "browserLogs",
						"allocation":      "10",
						"allocation_type": "percentage",
						"enabled":         "true",
						"can_overflow":    "false",
					}),
					resource.TestCheckTypeSetElemNestedAttrs(quotaAllocationRuleSetResourceName, "rules.*", map[string]string{
						"entity_type":     "browserLogs/v2",
						"allocation":      "10",
						"allocation_type": "percentage",
						"enabled":         "true",
						"can_overflow":    "false",
					}),
				),
			},
			{
				Config:   providerConfig + testAccCoralogixResourceQuotaAllocationRuleSetMixed(),
				PlanOnly: true,
			},
			{
				Config: providerConfig + testAccCoralogixResourceQuotaAllocationRuleSetEmpty(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(quotaAllocationRuleSetResourceName, "rules.#", "0"),
				),
			},
			{
				Config:   providerConfig + testAccCoralogixResourceQuotaAllocationRuleSetEmpty(),
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

func testAccCoralogixResourceQuotaAllocationRuleSetMixed() string {
	return `
resource "coralogix_quota_allocation_rule_set" "test" {
  rules = [
    {
      entity_type     = "logs"
      allocation      = 50
      allocation_type = "percentage"
      enabled         = true
      can_overflow    = true
    },
    {
      entity_type     = "spans"
      # Fractional units so the fixture fits an ephemeral team's minimal
      # 0.01-unit daily quota; locked units must not exceed the team quota.
      allocation      = 0.001
      allocation_type = "locked_units"
      enabled         = true
      can_overflow    = false
    },
    {
      entity_type     = "browserLogs"
      allocation      = 10
      allocation_type = "percentage"
      enabled         = true
      can_overflow    = false
    },
    {
      entity_type     = "browserLogs/v2"
      allocation      = 10
      allocation_type = "percentage"
      enabled         = true
      can_overflow    = false
    }
  ]
}
`
}

func testAccCoralogixResourceQuotaAllocationRuleSetEmpty() string {
	return `
resource "coralogix_quota_allocation_rule_set" "test" {
  rules = []
}
`
}
