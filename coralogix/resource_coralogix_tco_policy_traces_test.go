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

var tcoPolicyTracesResourceName1 = "coralogix_tco_policy_traces.test_1"
var tcoPolicyTracesResourceName2 = "coralogix_tco_policy_traces.test_2"
var tcoPolicyTracesResourceName3 = "coralogix_tco_policy_traces.test_3"

func TestAccCoralogixResourceTCOPolicyTracesCreate(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccTCOPolicyTracesCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config:  testAccCoralogixResourceTCOPolicyTraces(),
				Destroy: false,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(tcoPolicyTracesResourceName1, "name", "Example tco_policy from terraform 1"),
					resource.TestCheckResourceAttr(tcoPolicyTracesResourceName1, "priority", "low"),
					resource.TestCheckResourceAttr(tcoPolicyTracesResourceName1, "order", "1"),
					resource.TestCheckResourceAttr(tcoPolicyTracesResourceName1, "applications.rule_type", "starts with"),
					resource.TestCheckResourceAttr(tcoPolicyTracesResourceName1, "applications.names.0", "prod"),
					resource.TestCheckResourceAttr(tcoPolicyTracesResourceName1, "subsystems.rule_type", "is"),
					resource.TestCheckResourceAttr(tcoPolicyTracesResourceName1, "subsystems.names.#", "2"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyTracesResourceName1, "subsystems.names.*", "mobile"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyTracesResourceName1, "subsystems.names.*", "web"),
					resource.TestCheckResourceAttr(tcoPolicyTracesResourceName1, "actions.rule_type", "is not"),
					resource.TestCheckResourceAttr(tcoPolicyTracesResourceName1, "actions.names.#", "2"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyTracesResourceName1, "actions.names.*", "action-name"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyTracesResourceName1, "actions.names.*", "action-name2"),
					resource.TestCheckResourceAttr(tcoPolicyTracesResourceName1, "services.rule_type", "includes"),
					resource.TestCheckResourceAttr(tcoPolicyTracesResourceName1, "services.names.#", "2"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyTracesResourceName1, "services.names.*", "service-name"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyTracesResourceName1, "services.names.*", "service-name2"),
					//resource.TestCheckResourceAttr(tcoPolicyTracesResourceName1, "services.tags.tags.http.method", "includes"),
					////resource.TestCheckResourceAttr(tcoPolicyTracesResourceName1, "services.tags.tags.http.method.names.#", "2"),
					////resource.TestCheckTypeSetElemAttr(tcoPolicyTracesResourceName1, "services.tags.tags.http.method.names.*", "GET"),
					////resource.TestCheckTypeSetElemAttr(tcoPolicyTracesResourceName1, "services.tags.\"tags.http.method\".names.*", "POST"),
					resource.TestCheckResourceAttr(tcoPolicyTracesResourceName1, "archive_retention_id", "e1c980d0-c910-4c54-8326-67f3cf95645a"),

					resource.TestCheckResourceAttr(tcoPolicyTracesResourceName2, "name", "Example tco_policy from terraform 2"),
					resource.TestCheckResourceAttr(tcoPolicyTracesResourceName2, "priority", "medium"),
					resource.TestCheckResourceAttr(tcoPolicyTracesResourceName2, "order", "2"),
					resource.TestCheckResourceAttr(tcoPolicyTracesResourceName2, "applications.rule_type", "starts with"),
					resource.TestCheckResourceAttr(tcoPolicyTracesResourceName2, "applications.names.0", "staging"),
					resource.TestCheckResourceAttr(tcoPolicyTracesResourceName2, "subsystems.rule_type", "is"),
					resource.TestCheckResourceAttr(tcoPolicyTracesResourceName2, "subsystems.names.#", "2"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyTracesResourceName2, "subsystems.names.*", "mobile"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyTracesResourceName2, "subsystems.names.*", "web"),
					resource.TestCheckResourceAttr(tcoPolicyTracesResourceName2, "actions.rule_type", "is"),
					resource.TestCheckResourceAttr(tcoPolicyTracesResourceName2, "actions.names.#", "2"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyTracesResourceName2, "actions.names.*", "action-name"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyTracesResourceName2, "actions.names.*", "action-name2"),
					resource.TestCheckResourceAttr(tcoPolicyTracesResourceName2, "services.rule_type", "is"),
					resource.TestCheckResourceAttr(tcoPolicyTracesResourceName2, "services.names.#", "2"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyTracesResourceName2, "services.names.*", "service-name"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyTracesResourceName2, "services.names.*", "service-name2"),
					//resource.TestCheckResourceAttr(tcoPolicyTracesResourceName2, "services.tags.\"tags.http.method\".rule_type", "includes"),
					//resource.TestCheckResourceAttr(tcoPolicyTracesResourceName2, "services.tags.\"tags.http.method\".names.#", "2"),
					//resource.TestCheckTypeSetElemAttr(tcoPolicyTracesResourceName2, "services.tags.\"tags.http.method\".names.*", "GET"),
					//resource.TestCheckTypeSetElemAttr(tcoPolicyTracesResourceName2, "services.tags.\"tags.http.method\".names.*", "POST"),

					resource.TestCheckResourceAttr(tcoPolicyTracesResourceName3, "name", "Example tco_policy from terraform 3"),
					resource.TestCheckResourceAttr(tcoPolicyTracesResourceName3, "priority", "high"),
					resource.TestCheckResourceAttr(tcoPolicyTracesResourceName3, "order", "3"),
					resource.TestCheckResourceAttr(tcoPolicyTracesResourceName3, "applications.rule_type", "starts with"),
					resource.TestCheckResourceAttr(tcoPolicyTracesResourceName3, "applications.names.0", "staging"),
					resource.TestCheckResourceAttr(tcoPolicyTracesResourceName3, "subsystems.rule_type", "is"),
					resource.TestCheckResourceAttr(tcoPolicyTracesResourceName3, "subsystems.names.#", "2"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyTracesResourceName3, "subsystems.names.*", "mobile"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyTracesResourceName3, "subsystems.names.*", "web"),
					resource.TestCheckResourceAttr(tcoPolicyTracesResourceName3, "actions.rule_type", "is"),
					resource.TestCheckResourceAttr(tcoPolicyTracesResourceName3, "actions.names.#", "2"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyTracesResourceName3, "actions.names.*", "action-name"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyTracesResourceName3, "actions.names.*", "action-name2"),
					resource.TestCheckResourceAttr(tcoPolicyTracesResourceName3, "services.rule_type", "is"),
					resource.TestCheckResourceAttr(tcoPolicyTracesResourceName3, "services.names.#", "2"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyTracesResourceName3, "services.names.*", "service-name"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyTracesResourceName3, "services.names.*", "service-name2"),
					//resource.TestCheckResourceAttr(tcoPolicyTracesResourceName3, "services.tags.\"tags.http.method\".rule_type", "includes"),
					//resource.TestCheckResourceAttr(tcoPolicyTracesResourceName3, "services.tags.\"tags.http.method\".names.#", "2"),
					//resource.TestCheckTypeSetElemAttr(tcoPolicyTracesResourceName3, "services.tags.\"tags.http.method\".names.*", "GET"),
					//resource.TestCheckTypeSetElemAttr(tcoPolicyTracesResourceName3, "services.tags.\"tags.http.method\".names.*", "POST"),
				),
			},
		},
	})
}

func testAccTCOPolicyTracesCheckDestroy(s *terraform.State) error {
	client := testAccProvider.Meta().(*clientset.ClientSet).TCOPolicies()
	ctx := context.TODO()
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "coralogix_tco_policy_traces" {
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

func testAccCoralogixResourceTCOPolicyTraces() string {
	return fmt.Sprintf(
		`resource "coralogix_tco_policy_traces" "test_1" {
				  name       = "Example tco_policy from terraform 1"
				  priority   = "low"
				  order      = 1
				  applications = {
				    rule_type = "starts with"
				    names        = ["prod"]
				  }
				  subsystems = {
				    names = ["mobile", "web"]
				  }
				  actions = {
				    rule_type = "is not"
				    names = ["action-name", "action-name2"]
				  }
				  services = {
				      rule_type = "includes"
				      names = ["service-name", "service-name2"]
				  }
				  tags = {
					"tags.http.method" = {
				    	rule_type = "includes"
				        names = ["GET", "POST"]
				    }
				  }
				  archive_retention_id = "e1c980d0-c910-4c54-8326-67f3cf95645a"
				}
				
				resource "coralogix_tco_policy_traces" "test_2" {
				  name       = "Example tco_policy from terraform 2"
				  priority   = "medium"
				  order      = coralogix_tco_policy_traces.test_1.order + 1
				  applications = {
				    rule_type = "starts with"
				    names        = ["staging"]
				  }
				  subsystems = {
				    rule_type = "is not"
				    names = ["mobile", "web"]
				  }
				  actions = {
				        names = ["action-name", "action-name2"]
				  }
				  services = {
				      names = ["service-name", "service-name2"]
				  }
				  tags = {
					"tags.http.method" = {
				    	rule_type = "includes"
				        names = ["GET", "POST"]
				    }
				  }
				}
				
				resource "coralogix_tco_policy_traces" "test_3" {
				  name       = "Example tco_policy from terraform 3"
				  priority   = "medium"
				  order      = coralogix_tco_policy_traces.test_2.order + 1
				  applications = {
				    rule_type = "starts with"
				    names        = ["staging"]
				  }
				  subsystems = {
				    rule_type = "is not"
				    names = ["mobile", "web"]
				  }
				  actions = {
				        names = ["action-name", "action-name2"]
				  }
				  services = {
				      names = ["service-name", "service-name2"]
				  }
				   tags = {
					"tags.http.method" = {
				    	rule_type = "includes"
				        names = ["GET", "POST"]
				    }
				   }
				}
	`)
}
