// Copyright 2024 Coralogix Ltd.
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

package coralogix

import (
	"context"
	"fmt"
	"testing"

	"terraform-provider-coralogix/coralogix/clientset"
	tcopolicies "terraform-provider-coralogix/coralogix/clientset/grpc/tco-policies"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"google.golang.org/protobuf/types/known/wrapperspb"
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
					resource.TestCheckResourceAttr(tcoPolicyTracesResourceName1, "applications.rule_type", "starts_with"),
					resource.TestCheckResourceAttr(tcoPolicyTracesResourceName1, "applications.names.0", "prod"),
					resource.TestCheckResourceAttr(tcoPolicyTracesResourceName1, "subsystems.rule_type", "is"),
					resource.TestCheckResourceAttr(tcoPolicyTracesResourceName1, "subsystems.names.#", "2"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyTracesResourceName1, "subsystems.names.*", "mobile"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyTracesResourceName1, "subsystems.names.*", "web"),
					resource.TestCheckResourceAttr(tcoPolicyTracesResourceName1, "actions.rule_type", "is_not"),
					resource.TestCheckResourceAttr(tcoPolicyTracesResourceName1, "actions.names.#", "2"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyTracesResourceName1, "actions.names.*", "action-name"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyTracesResourceName1, "actions.names.*", "action-name2"),
					resource.TestCheckResourceAttr(tcoPolicyTracesResourceName1, "services.rule_type", "is"),
					resource.TestCheckResourceAttr(tcoPolicyTracesResourceName1, "services.names.#", "2"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyTracesResourceName1, "services.names.*", "service-name"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyTracesResourceName1, "services.names.*", "service-name2"),
					resource.TestCheckResourceAttr(tcoPolicyTracesResourceName1, "tags.tags.http.method.rule_type", "includes"),
					resource.TestCheckResourceAttr(tcoPolicyTracesResourceName1, "tags.tags.http.method.names.#", "1"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyTracesResourceName1, "tags.tags.http.method.names.*", "GET"),
					resource.TestCheckResourceAttr(tcoPolicyTracesResourceName1, "archive_retention_id", "e1c980d0-c910-4c54-8326-67f3cf95645a"),

					resource.TestCheckResourceAttr(tcoPolicyTracesResourceName2, "name", "Example tco_policy from terraform 2"),
					resource.TestCheckResourceAttr(tcoPolicyTracesResourceName2, "priority", "medium"),
					resource.TestCheckResourceAttr(tcoPolicyTracesResourceName2, "order", "2"),
					resource.TestCheckResourceAttr(tcoPolicyTracesResourceName2, "applications.rule_type", "starts_with"),
					resource.TestCheckResourceAttr(tcoPolicyTracesResourceName2, "applications.names.0", "staging"),
					resource.TestCheckResourceAttr(tcoPolicyTracesResourceName2, "subsystems.rule_type", "is_not"),
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
					resource.TestCheckResourceAttr(tcoPolicyTracesResourceName2, "tags.tags.http.method.rule_type", "includes"),
					resource.TestCheckResourceAttr(tcoPolicyTracesResourceName2, "tags.tags.http.method.names.#", "2"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyTracesResourceName2, "tags.tags.http.method.names.*", "GET"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyTracesResourceName2, "tags.tags.http.method.names.*", "POST"),

					resource.TestCheckResourceAttr(tcoPolicyTracesResourceName3, "name", "Example tco_policy from terraform 3"),
					resource.TestCheckResourceAttr(tcoPolicyTracesResourceName3, "priority", "high"),
					resource.TestCheckResourceAttr(tcoPolicyTracesResourceName3, "order", "3"),
					resource.TestCheckResourceAttr(tcoPolicyTracesResourceName3, "applications.rule_type", "starts_with"),
					resource.TestCheckResourceAttr(tcoPolicyTracesResourceName3, "applications.names.0", "prod"),
					resource.TestCheckResourceAttr(tcoPolicyTracesResourceName3, "subsystems.rule_type", "is_not"),
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
					resource.TestCheckResourceAttr(tcoPolicyTracesResourceName3, "tags.tags.http.method.rule_type", "includes"),
					resource.TestCheckResourceAttr(tcoPolicyTracesResourceName3, "tags.tags.http.method.names.#", "2"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyTracesResourceName3, "tags.tags.http.method.names.*", "GET"),
					resource.TestCheckTypeSetElemAttr(tcoPolicyTracesResourceName3, "tags.tags.http.method.names.*", "POST"),
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
	return `resource "coralogix_tco_policy_traces" "test_1" {
				  name       = "Example tco_policy from terraform 1"
				  priority   = "low"
				  order      = 1
				  applications = {
				    rule_type = "starts_with"
				    names        = ["prod"]
				  }
				  subsystems = {
				    names = ["mobile", "web"]
				  }
				  actions = {
				    rule_type = "is_not"
				    names = ["action-name", "action-name2"]
				  }
				  services = {
				      rule_type = "is"
				      names = ["service-name", "service-name2"]
				  }
				  tags = {
					"tags.http.method" = {
				    	rule_type = "includes"
				        names = ["GET"]
				    }
				  }
				  archive_retention_id = "e1c980d0-c910-4c54-8326-67f3cf95645a"
				}
				
				resource "coralogix_tco_policy_traces" "test_2" {
				  name       = "Example tco_policy from terraform 2"
				  priority   = "medium"
				  order      = coralogix_tco_policy_traces.test_1.order + 1
				  applications = {
				    rule_type = "starts_with"
				    names        = ["staging"]
				  }
				  subsystems = {
				    rule_type = "is_not"
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
				  priority   = "high"
				  order      = coralogix_tco_policy_traces.test_2.order + 1
				  applications = {
				    rule_type = "starts_with"
				    names        = ["prod"]
				  }
				  subsystems = {
				    rule_type = "is_not"
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
	`
}
