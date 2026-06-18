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
	"net/http"
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

const quotaRuleResourceName = "coralogix_quota_rule.test"

func TestAccCoralogixResourceQuotaRule(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-quota-rule")
	updatedName := acctest.RandomWithPrefix("tf-acc-quota-rule-updated")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccQuotaRuleCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceQuotaRuleLogTarget(name, "managed by terraform", true, "medium"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(quotaRuleResourceName, "id"),
					resource.TestCheckResourceAttr(quotaRuleResourceName, "name", name),
					resource.TestCheckResourceAttr(quotaRuleResourceName, "description", "managed by terraform"),
					resource.TestCheckResourceAttr(quotaRuleResourceName, "enabled", "true"),
					resource.TestCheckResourceAttr(quotaRuleResourceName, "log_rules.dpxl_expression", "<v1> $d.severity == 'INFO'"),
					resource.TestCheckResourceAttr(quotaRuleResourceName, "targets.#", "1"),
					resource.TestCheckResourceAttr(quotaRuleResourceName, "targets.0.dataset", "logs"),
					resource.TestCheckResourceAttr(quotaRuleResourceName, "targets.0.priority", "medium"),
				),
			},
			{
				ResourceName:      quotaRuleResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccCoralogixResourceQuotaRuleLogTarget(updatedName, "managed by terraform updated", false, "low"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(quotaRuleResourceName, "id"),
					resource.TestCheckResourceAttr(quotaRuleResourceName, "name", updatedName),
					resource.TestCheckResourceAttr(quotaRuleResourceName, "description", "managed by terraform updated"),
					resource.TestCheckResourceAttr(quotaRuleResourceName, "enabled", "false"),
					resource.TestCheckResourceAttr(quotaRuleResourceName, "log_rules.dpxl_expression", "<v1> $d.severity == 'INFO'"),
					resource.TestCheckResourceAttr(quotaRuleResourceName, "targets.#", "1"),
					resource.TestCheckResourceAttr(quotaRuleResourceName, "targets.0.dataset", "logs"),
					resource.TestCheckResourceAttr(quotaRuleResourceName, "targets.0.dataspace", "default"),
					resource.TestCheckResourceAttr(quotaRuleResourceName, "targets.0.priority", "low"),
				),
			},
			{
				Config:   testAccCoralogixResourceQuotaRuleLogTarget(updatedName, "managed by terraform updated", false, "low"),
				PlanOnly: true,
			},
		},
	})
}

func testAccQuotaRuleCheckDestroy(s *terraform.State) error {
	meta := testAccProvider.Meta()
	if meta == nil {
		return nil
	}
	client := meta.(*clientset.ClientSet).TCOPolicies()
	ctx := context.TODO()

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "coralogix_quota_rule" {
			continue
		}

		result, httpResponse, err := client.PoliciesServiceGetPolicy(ctx, rs.Primary.ID).Execute()
		if err != nil {
			if httpResponse != nil && httpResponse.StatusCode == http.StatusNotFound {
				continue
			}
			return err
		}
		if result != nil && result.Policy != nil && result.Policy.GetActualInstance() != nil {
			return fmt.Errorf("quota rule still exists: %s", rs.Primary.ID)
		}
	}

	return nil
}

func testAccCoralogixResourceQuotaRuleLog(name, description string, enabled bool, priority string) string {
	return fmt.Sprintf(`
resource "coralogix_quota_rule" "test" {
  name        = %[1]q
  description = %[2]q
  enabled     = %[3]t
  priority    = %[4]q

  application_rule = {
    rule_type = "is"
    names     = ["tf-acc"]
  }

  log_rules = {
    severities = ["info"]
  }
}
`, name, description, enabled, priority)
}

func testAccCoralogixResourceQuotaRuleLogTarget(name, description string, enabled bool, targetPriority string) string {
	return fmt.Sprintf(`
resource "coralogix_quota_rule" "test" {
  name        = %[1]q
  description = %[2]q
  enabled     = %[3]t

  log_rules = {
    dpxl_expression = "<v1> $d.severity == 'INFO'"
  }

  targets = [
    {
      dataset   = "logs"
      dataspace = "default"
      priority  = %[4]q
    }
  ]
}
`, name, description, enabled, targetPriority)
}
