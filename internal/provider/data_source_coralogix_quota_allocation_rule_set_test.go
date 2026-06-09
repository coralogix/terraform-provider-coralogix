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
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

const quotaAllocationRuleSetDataSourceName = "data.coralogix_quota_allocation_rule_set.test"

func TestAccCoralogixDataSourceQuotaAllocationRuleSet(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceQuotaAllocationRuleSet(60, 40, true) +
					testAccCoralogixDataSourceQuotaAllocationRuleSet(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(quotaAllocationRuleSetDataSourceName, "rules.#", "2"),
					resource.TestCheckTypeSetElemNestedAttrs(quotaAllocationRuleSetDataSourceName, "rules.*", map[string]string{
						"entity_type":  "logs",
						"allocation":   "60",
						"enabled":      "true",
						"can_overflow": "true",
					}),
				),
			},
		},
	})
}

func testAccCoralogixDataSourceQuotaAllocationRuleSet() string {
	return fmt.Sprintf(`
data "coralogix_quota_allocation_rule_set" "test" {
  depends_on = [%s]
}
`, quotaAllocationRuleSetResourceName)
}
