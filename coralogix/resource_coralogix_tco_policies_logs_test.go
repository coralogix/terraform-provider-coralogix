package coralogix

import (
	"context"
	"fmt"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"terraform-provider-coralogix/coralogix/clientset"
	tcopolicies "terraform-provider-coralogix/coralogix/clientset/grpc/tco-policies"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

var tcoPoliciesResourceName = "coralogix_tco_policies_logs.test"

func TestAccCoralogixResourceTCOPoliciesLogsCreate(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccTCOPoliciesLogsCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config:  testAccCoralogixResourceTCOPolicy(),
				Destroy: false,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(tcoPoliciesResourceName, "policies.0.name", "Example tco_policy from terraform 1"),
					resource.TestCheckResourceAttr(tcoPoliciesResourceName, "policies.0.priority", "low"),
					resource.TestCheckResourceAttr(tcoPoliciesResourceName, "policies.0.order", "1"),
					resource.TestCheckResourceAttr(tcoPoliciesResourceName, "policies.0.severities.#", "3"),
					resource.TestCheckTypeSetElemAttr(tcoPoliciesResourceName, "policies.0.severities.*", "debug"),
					resource.TestCheckTypeSetElemAttr(tcoPoliciesResourceName, "policies.0.severities.*", "verbose"),
					resource.TestCheckTypeSetElemAttr(tcoPoliciesResourceName, "policies.0.severities.*", "info"),
					resource.TestCheckResourceAttr(tcoPoliciesResourceName, "policies.0.applications.rule_type", "starts_with"),
					resource.TestCheckResourceAttr(tcoPoliciesResourceName, "policies.0.applications.names.0", "prod"),
					resource.TestCheckResourceAttr(tcoPoliciesResourceName, "policies.0.subsystems.rule_type", "is"),
					resource.TestCheckResourceAttr(tcoPoliciesResourceName, "policies.0.subsystems.names.#", "2"),
					resource.TestCheckTypeSetElemAttr(tcoPoliciesResourceName, "policies.0.subsystems.names.*", "mobile"),
					resource.TestCheckTypeSetElemAttr(tcoPoliciesResourceName, "policies.0.subsystems.names.*", "web"),
					resource.TestCheckResourceAttr(tcoPoliciesResourceName, "policies.0.archive_retention_id", "e1c980d0-c910-4c54-8326-67f3cf95645a"),

					resource.TestCheckResourceAttr(tcoPoliciesResourceName, "policies.1.name", "Example tco_policy from terraform 2"),
					resource.TestCheckResourceAttr(tcoPoliciesResourceName, "policies.1.priority", "medium"),
					resource.TestCheckResourceAttr(tcoPoliciesResourceName, "policies.1.order", "2"),
					resource.TestCheckResourceAttr(tcoPoliciesResourceName, "policies.1.severities.#", "3"),
					resource.TestCheckTypeSetElemAttr(tcoPoliciesResourceName, "policies.1.severities.*", "error"),
					resource.TestCheckTypeSetElemAttr(tcoPoliciesResourceName, "policies.1.severities.*", "warning"),
					resource.TestCheckTypeSetElemAttr(tcoPoliciesResourceName, "policies.1.severities.*", "critical"),
					resource.TestCheckResourceAttr(tcoPoliciesResourceName, "policies.1.applications.rule_type", "starts_with"),
					resource.TestCheckResourceAttr(tcoPoliciesResourceName, "policies.1.applications.names.0", "prod"),
					resource.TestCheckResourceAttr(tcoPoliciesResourceName, "policies.1.subsystems.rule_type", "is"),
					resource.TestCheckResourceAttr(tcoPoliciesResourceName, "policies.1.subsystems.names.#", "2"),
					resource.TestCheckTypeSetElemAttr(tcoPoliciesResourceName, "policies.1.subsystems.names.*", "mobile"),
					resource.TestCheckTypeSetElemAttr(tcoPoliciesResourceName, "policies.1.subsystems.names.*", "web"),

					resource.TestCheckResourceAttr(tcoPoliciesResourceName, "policies.2.name", "Example tco_policy from terraform 3"),
					resource.TestCheckResourceAttr(tcoPoliciesResourceName, "policies.2.priority", "high"),
					resource.TestCheckResourceAttr(tcoPoliciesResourceName, "policies.2.order", "3"),
					resource.TestCheckResourceAttr(tcoPoliciesResourceName, "policies.2.severities.#", "3"),
					resource.TestCheckTypeSetElemAttr(tcoPoliciesResourceName, "policies.2.severities.*", "debug"),
					resource.TestCheckTypeSetElemAttr(tcoPoliciesResourceName, "policies.2.severities.*", "verbose"),
					resource.TestCheckTypeSetElemAttr(tcoPoliciesResourceName, "policies.2.severities.*", "info"),
					resource.TestCheckResourceAttr(tcoPoliciesResourceName, "policies.2.applications.rule_type", "starts_with"),
					resource.TestCheckResourceAttr(tcoPoliciesResourceName, "policies.2.applications.names.0", "prod"),
					resource.TestCheckResourceAttr(tcoPoliciesResourceName, "policies.2.subsystems.rule_type", "is"),
					resource.TestCheckResourceAttr(tcoPoliciesResourceName, "policies.2.subsystems.names.#", "2"),
					resource.TestCheckTypeSetElemAttr(tcoPoliciesResourceName, "policies.2.subsystems.names.*", "mobile"),
					resource.TestCheckTypeSetElemAttr(tcoPoliciesResourceName, "policies.2.subsystems.names.*", "web"),
				),
			},
		},
	})
}

func testAccTCOPoliciesLogsCheckDestroy(s *terraform.State) error {
	client := testAccProvider.Meta().(*clientset.ClientSet).TCOPolicies()
	ctx := context.TODO()
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "coralogix_tco_policies_logs" {
			continue
		}

		if resp, err := client.GetTCOPolicies(ctx, &tcopolicies.GetCompanyPoliciesRequest{SourceType: &logSource}); err == nil {
			if err == nil {
				if len(resp.GetPolicies()) != 0 {
					return fmt.Errorf("tco-policies still exist: %s", protojson.Format(resp))
				}
			}
		}
	}

	return nil
}

func testAccCoralogixResourceTCOPoliciesLogs() string {
	return `resource "coralogix_tco_policies_logs" test {
					policies = [
					{
 						name       = "Example tco_policy from terraform 1"
  						priority   = "low"
						order      = 1
						severities = ["debug", "verbose", "info"]
 						applications = {
 					  		rule_type = "starts_with"
 					  		names        = ["prod"]
 						}
 						subsystems = {
 					  		rule_type = "is"
 					  		names = ["mobile", "web"]
 						}
 						archive_retention_id = "e1c980d0-c910-4c54-8326-67f3cf95645a"
					},
					{
						name     = "Example tco_policy from terraform 2"
						priority = "medium"
					  	order = coralogix_tco_policy_logs.test_1.order + 1
                      	severities = ["error", "warning", "critical"]
  					 	applications = {
   						 	rule_type = "starts_with"
    						 names        = ["prod"]
					  	}
						subsystems = {
							rule_type = "is"
    						names = ["mobile", "web"]
						}
					},
					{
 						name     = "Example tco_policy from terraform 3"
						order    = coralogix_tco_policy_logs.test_2.order + 1
  						priority = "high"
  						severities = ["debug", "verbose", "info"]
  						applications = {
   						 	rule_type = "starts_with"
							names        = ["prod"]
  						}
  						subsystems = {
    						rule_type = "is"
    						names = ["mobile", "web"]
						}
					}
			]
	}
	`
}
