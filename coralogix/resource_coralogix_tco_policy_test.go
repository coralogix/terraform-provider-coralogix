package coralogix

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"terraform-provider-coralogix/coralogix/clientset"
	tcopolicies "terraform-provider-coralogix/coralogix/clientset/grpc/tco-policies"
)

var tcoPolicyResourceName1 = "coralogix_tco_policy.test_1"
var tcoPolicyResourceName2 = "coralogix_tco_policy.test_2"
var tcoPolicyResourceName3 = "coralogix_tco_policy.test_3"

func TestAccCoralogixResourceTCOPolicyCreate(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccTCOPolicyCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config:  testAccCoralogixResourceTCOPolicy(),
				Destroy: false,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(tcoPolicyResourceName1, "name", "Example tco_policy from terraform 1"),
					resource.TestCheckResourceAttr(tcoPolicyResourceName1, "priority", "medium"),
					resource.TestCheckResourceAttr(tcoPolicyResourceName1, "order", "1"),
					resource.TestCheckResourceAttr(tcoPolicyResourceName1, "severities.#", "3"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyResourceName1, "severities.*", "debug"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyResourceName1, "severities.*", "verbose"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyResourceName1, "severities.*", "info"),
					resource.TestCheckResourceAttr(tcoPolicyResourceName1, "applications.rule_type", "starts with"),
					resource.TestCheckResourceAttr(tcoPolicyResourceName1, "applications.names.0", "prod"),
					resource.TestCheckResourceAttr(tcoPolicyResourceName1, "subsystems.rule_type", "is"),
					resource.TestCheckResourceAttr(tcoPolicyResourceName1, "subsystems.names.#", "2"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyResourceName1, "subsystems.names.#.*", "mobile"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyResourceName1, "subsystems.names.#.*", "web"),
					//resource.TestCheckResourceAttr(tcoPolicyResourceName1, "archive_retention_id", "e1c980d0-c910-4c54-8326-67f3cf95645a"),

					resource.TestCheckResourceAttr(tcoPolicyResourceName2, "name", "Example tco_policy from terraform 2"),
					resource.TestCheckResourceAttr(tcoPolicyResourceName2, "priority", "medium"),
					resource.TestCheckResourceAttr(tcoPolicyResourceName2, "order", "2"),
					resource.TestCheckResourceAttr(tcoPolicyResourceName2, "severities.#", "3"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyResourceName2, "severities.*", "error"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyResourceName2, "severities.*", "warning"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyResourceName2, "severities.*", "critical"),
					resource.TestCheckResourceAttr(tcoPolicyResourceName2, "applications.rule_type", "starts with"),
					resource.TestCheckResourceAttr(tcoPolicyResourceName2, "applications.names.0", "prod"),
					resource.TestCheckResourceAttr(tcoPolicyResourceName2, "subsystems.rule_type", "is"),
					resource.TestCheckResourceAttr(tcoPolicyResourceName2, "subsystems.names.#", "2"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyResourceName2, "subsystems.names.#.*", "mobile"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyResourceName2, "subsystems.names.#.*", "web"),

					resource.TestCheckResourceAttr(tcoPolicyResourceName3, "name", "Example tco_policy from terraform 3"),
					resource.TestCheckResourceAttr(tcoPolicyResourceName3, "priority", "medium"),
					resource.TestCheckResourceAttr(tcoPolicyResourceName3, "order", "3"),
					resource.TestCheckResourceAttr(tcoPolicyResourceName3, "severities.#", "3"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyResourceName3, "severities.*", "debug"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyResourceName3, "severities.*", "verbose"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyResourceName3, "severities.*", "info"),
					resource.TestCheckResourceAttr(tcoPolicyResourceName3, "applications.rule_type", "starts with"),
					resource.TestCheckResourceAttr(tcoPolicyResourceName3, "applications.names.0", "prod"),
					resource.TestCheckResourceAttr(tcoPolicyResourceName3, "subsystems.rule_type", "is"),
					resource.TestCheckResourceAttr(tcoPolicyResourceName3, "subsystems.names.#", "2"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyResourceName3, "subsystems.names.#.*", "mobile"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyResourceName3, "subsystems.names.#.*", "web"),
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

		if resp, err := client.GetTCOPolicy(ctx, &tcopolicies.GetPolicyRequest{Id: wrapperspb.String(rs.Primary.ID)}); err == nil {
			id := resp.GetPolicy().GetId().GetValue()
			if err == nil {
				if id == rs.Primary.ID {
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
 					name       = "Example tco_policy from terraform 1"
  					priority   = "low"
					order      = 1
					severities = ["debug", "verbose", "info"]
 					applications = {
 					  rule_type = "starts with"
 					  names        = ["prod"]
 					}
 					subsystems = {
 					  rule_type = "is"
 					  names = ["mobile", "web"]
 					}
 					//archive_retention_id = "e1c980d0-c910-4c54-8326-67f3cf95645a"
				}

				resource "coralogix_tco_policy" test_2 {
 					  name     = "Example tco_policy from terraform 2"
                      priority = "medium"
					  order = coralogix_tco_policy.test_1.order + 1
  					 severities = ["error", "warning", "critical"]
  					 applications = {
   						 rule_type = "starts with"
    					 names        = ["prod"]
  					}
  					subsystems = {
    					rule_type = "is"
    					names = ["mobile", "web"]
					}
				}

				resource "coralogix_tco_policy" test_3 {
 					name     = "Example tco_policy from terraform 3"
                    order    = coralogix_tco_policy.test_2.order + 1
  					priority = "medium"
  					severities = ["debug", "verbose", "info"]
  					applications = {
   						 rule_type = "starts with"
    					 names        = ["prod"]
  					}
  					subsystems = {
    					rule_type = "is"
    					names = ["mobile", "web"]
					}
				}
	`)
}
