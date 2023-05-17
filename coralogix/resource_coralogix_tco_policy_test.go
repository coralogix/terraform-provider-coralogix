package coralogix

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"terraform-provider-coralogix/coralogix/clientset"
)

var tcoPolicyResourceName = "coralogix_tco_policy.test"
var tcoPolicyResourceName2 = "coralogix_tco_policy.test_2"

func TestAccCoralogixResourceTCOPolicyCreate(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccTCOPolicyCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceTCOPolicy(),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(tcoPolicyResourceName, "name", "Example tco_policy from terraform"),
					resource.TestCheckResourceAttr(tcoPolicyResourceName, "priority", "medium"),
					resource.TestCheckResourceAttr(tcoPolicyResourceName, "order", "1"),
					resource.TestCheckResourceAttr(tcoPolicyResourceName, "severities.#", "3"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyResourceName, "severities.*", "debug"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyResourceName, "severities.*", "verbose"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyResourceName, "severities.*", "info"),
					resource.TestCheckResourceAttr(tcoPolicyResourceName, "application_name.0.starts_with", "true"),
					resource.TestCheckResourceAttr(tcoPolicyResourceName, "application_name.0.rule", "prod"),
					resource.TestCheckResourceAttr(tcoPolicyResourceName, "subsystem_name.0.is", "true"),
					resource.TestCheckResourceAttr(tcoPolicyResourceName, "subsystem_name.0.rules.#", "2"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyResourceName, "subsystem_name.0.rules.*", "mobile"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyResourceName, "subsystem_name.0.rules.*", "web"),

					resource.TestCheckResourceAttr(tcoPolicyResourceName2, "name", "Example tco_policy from terraform 2"),
					resource.TestCheckResourceAttr(tcoPolicyResourceName2, "priority", "medium"),
					resource.TestCheckResourceAttr(tcoPolicyResourceName2, "order", "2"),
					resource.TestCheckResourceAttr(tcoPolicyResourceName2, "severities.#", "3"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyResourceName2, "severities.*", "debug"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyResourceName2, "severities.*", "verbose"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyResourceName2, "severities.*", "info"),
					resource.TestCheckResourceAttr(tcoPolicyResourceName2, "application_name.0.starts_with", "true"),
					resource.TestCheckResourceAttr(tcoPolicyResourceName2, "application_name.0.rule", "prod"),
					resource.TestCheckResourceAttr(tcoPolicyResourceName2, "subsystem_name.0.is", "true"),
					resource.TestCheckResourceAttr(tcoPolicyResourceName2, "subsystem_name.0.rules.#", "2"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyResourceName2, "subsystem_name.0.rules.*", "mobile"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyResourceName2, "subsystem_name.0.rules.*", "web"),
				),
			},
			{
				ResourceName:      tcoPolicyResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccCoralogixUpdatedResourceTCOPolicy(),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(tcoPolicyResourceName, "name", "Example updated tco_policy from terraform"),
					resource.TestCheckResourceAttr(tcoPolicyResourceName, "priority", "low"),
					resource.TestCheckResourceAttr(tcoPolicyResourceName2, "order", "2"),
					resource.TestCheckResourceAttr(tcoPolicyResourceName, "severities.#", "3"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyResourceName, "severities.*", "warning"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyResourceName, "severities.*", "error"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyResourceName, "severities.*", "critical"),
					resource.TestCheckResourceAttr(tcoPolicyResourceName, "application_name.0.includes", "true"),
					resource.TestCheckResourceAttr(tcoPolicyResourceName, "application_name.0.rule", "dev"),
					resource.TestCheckResourceAttr(tcoPolicyResourceName, "subsystem_name.0.is_not", "true"),
					resource.TestCheckResourceAttr(tcoPolicyResourceName, "subsystem_name.0.rules.#", "2"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyResourceName, "subsystem_name.0.rules.*", "mobile"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyResourceName, "subsystem_name.0.rules.*", "web"),

					resource.TestCheckResourceAttr(tcoPolicyResourceName2, "name", "Example tco_policy from terraform 2"),
					resource.TestCheckResourceAttr(tcoPolicyResourceName2, "priority", "medium"),
					resource.TestCheckResourceAttr(tcoPolicyResourceName2, "order", "1"),
					resource.TestCheckResourceAttr(tcoPolicyResourceName2, "severities.#", "3"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyResourceName2, "severities.*", "debug"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyResourceName2, "severities.*", "verbose"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyResourceName2, "severities.*", "info"),
					resource.TestCheckResourceAttr(tcoPolicyResourceName2, "application_name.0.starts_with", "true"),
					resource.TestCheckResourceAttr(tcoPolicyResourceName2, "application_name.0.rule", "prod"),
					resource.TestCheckResourceAttr(tcoPolicyResourceName2, "subsystem_name.0.is", "true"),
					resource.TestCheckResourceAttr(tcoPolicyResourceName2, "subsystem_name.0.rules.#", "2"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyResourceName2, "subsystem_name.0.rules.*", "mobile"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyResourceName2, "subsystem_name.0.rules.*", "web"),
				),
			},
		},
	})
}

func testAccTCOPolicyCheckDestroy(s *terraform.State) error {
	client := testAccProvider.Meta().(*clientset.ClientSet).TCOPolicies()
	ctx := context.TODO()
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "coralogix_tco_policy" {
			continue
		}

		if resp, err := client.GetTCOPolicy(ctx, rs.Primary.ID); err == nil {
			var m map[string]interface{}
			if err = json.Unmarshal([]byte(resp), &m); err == nil {
				if id, ok := m["id"]; ok && id.(string) == rs.Primary.ID {
					return fmt.Errorf("tco-policy still exists: %s", id)
				}
			}
		}
	}

	return nil
}

func testAccCoralogixResourceTCOPolicy() string {
	return fmt.Sprintf(
		`resource "coralogix_tco_policy" test_1 {
 					name     = "Example tco_policy from terraform"
                    order    = 1
  					priority = "medium"
  					severities = ["debug", "verbose", "info"]
  					application_name {
    					starts_with = true
    					rule = "prod"
					}
  					subsystem_name {
    					is = true
    					rules = ["mobile", "web"]
  					}
				}

				resource "coralogix_tco_policy" test_2 {
 					name     = "Example tco_policy from terraform 2"
                    order    = coralogix_tco_policy.test_1.order + 1
  					priority = "medium"
  					severities = ["debug", "verbose", "info"]
  					application_name {
    					starts_with = true
    					rule = "prod"
					}
  					subsystem_name {
    					is = true
    					rules = ["mobile", "web"]
  					}
				}
	`)
}

func testAccCoralogixUpdatedResourceTCOPolicy() string {
	return fmt.Sprintf(
		`resource "coralogix_tco_policy" test {
 					name     = "Example updated tco_policy from terraform"
                    order    = 2
  					priority = "low"
  					severities = ["warning", "error", "critical"]
  					application_name {
    					includes = true
    					rule = "dev"
					}
  					subsystem_name {
    					is_not = true
    					rules = ["mobile", "web"]
  					}
				}

				resource "coralogix_tco_policy" test_2 {
 					name     = "Example tco_policy from terraform 2"
                    order    = 1
  					priority = "medium"
  					severities = ["debug", "verbose", "info"]
  					application_name {
    					starts_with = true
    					rule = "prod"
					}
  					subsystem_name {
    					is = true
    					rules = ["mobile", "web"]
  					}
				}
	`)
}
