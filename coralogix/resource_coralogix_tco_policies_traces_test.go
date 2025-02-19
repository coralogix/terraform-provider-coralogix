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

	"google.golang.org/protobuf/encoding/protojson"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

var tcoPoliciesTracesResourceName = "coralogix_tco_policies_traces.test"

func TestAccCoralogixResourceTCOPoliciesTracesCreate(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { TestAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccTCOPoliciesTracesCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config:  testAccCoralogixResourceTCOPoliciesTraces(),
				Destroy: false,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(tcoPoliciesTracesResourceName, "policies.0.name", "Example tco_policy from terraform 1"),
					resource.TestCheckResourceAttr(tcoPoliciesTracesResourceName, "policies.0.priority", "low"),
					resource.TestCheckResourceAttr(tcoPoliciesTracesResourceName, "policies.0.order", "1"),
					resource.TestCheckResourceAttr(tcoPoliciesTracesResourceName, "policies.0.applications.rule_type", "starts_with"),
					resource.TestCheckResourceAttr(tcoPoliciesTracesResourceName, "policies.0.applications.names.0", "prod"),
					resource.TestCheckResourceAttr(tcoPoliciesTracesResourceName, "policies.0.subsystems.rule_type", "is"),
					resource.TestCheckResourceAttr(tcoPoliciesTracesResourceName, "policies.0.subsystems.names.#", "2"),
					resource.TestCheckTypeSetElemAttr(tcoPoliciesTracesResourceName, "policies.0.subsystems.names.*", "mobile"),
					resource.TestCheckTypeSetElemAttr(tcoPoliciesTracesResourceName, "policies.0.subsystems.names.*", "web"),
					resource.TestCheckResourceAttr(tcoPoliciesTracesResourceName, "policies.0.actions.rule_type", "is_not"),
					resource.TestCheckResourceAttr(tcoPoliciesTracesResourceName, "policies.0.actions.names.#", "2"),
					resource.TestCheckTypeSetElemAttr(tcoPoliciesTracesResourceName, "policies.0.actions.names.*", "action-name"),
					resource.TestCheckTypeSetElemAttr(tcoPoliciesTracesResourceName, "policies.0.actions.names.*", "action-name2"),
					resource.TestCheckResourceAttr(tcoPoliciesTracesResourceName, "policies.0.services.rule_type", "is"),
					resource.TestCheckResourceAttr(tcoPoliciesTracesResourceName, "policies.0.services.names.#", "2"),
					resource.TestCheckTypeSetElemAttr(tcoPoliciesTracesResourceName, "policies.0.services.names.*", "service-name"),
					resource.TestCheckTypeSetElemAttr(tcoPoliciesTracesResourceName, "policies.0.services.names.*", "service-name2"),
					resource.TestCheckResourceAttr(tcoPoliciesTracesResourceName, "policies.0.tags.tags.http.method.rule_type", "includes"),
					resource.TestCheckResourceAttr(tcoPoliciesTracesResourceName, "policies.0.tags.tags.http.method.names.#", "1"),
					resource.TestCheckTypeSetElemAttr(tcoPoliciesTracesResourceName, "policies.0.tags.tags.http.method.names.*", "GET"),
					resource.TestCheckResourceAttr(tcoPoliciesTracesResourceName, "policies.0.archive_retention_id", "e1c980d0-c910-4c54-8326-67f3cf95645a"),

					resource.TestCheckResourceAttr(tcoPoliciesTracesResourceName, "policies.1.name", "Example tco_policy from terraform 2"),
					resource.TestCheckResourceAttr(tcoPoliciesTracesResourceName, "policies.1.priority", "medium"),
					resource.TestCheckResourceAttr(tcoPoliciesTracesResourceName, "policies.1.order", "2"),
					resource.TestCheckResourceAttr(tcoPoliciesTracesResourceName, "policies.1.applications.rule_type", "starts_with"),
					resource.TestCheckResourceAttr(tcoPoliciesTracesResourceName, "policies.1.applications.names.0", "staging"),
					resource.TestCheckResourceAttr(tcoPoliciesTracesResourceName, "policies.1.subsystems.rule_type", "is_not"),
					resource.TestCheckResourceAttr(tcoPoliciesTracesResourceName, "policies.1.subsystems.names.#", "2"),
					resource.TestCheckTypeSetElemAttr(tcoPoliciesTracesResourceName, "policies.1.subsystems.names.*", "mobile"),
					resource.TestCheckTypeSetElemAttr(tcoPoliciesTracesResourceName, "policies.1.subsystems.names.*", "web"),
					resource.TestCheckResourceAttr(tcoPoliciesTracesResourceName, "policies.1.actions.rule_type", "is"),
					resource.TestCheckResourceAttr(tcoPoliciesTracesResourceName, "policies.1.actions.names.#", "2"),
					resource.TestCheckTypeSetElemAttr(tcoPoliciesTracesResourceName, "policies.1.actions.names.*", "action-name"),
					resource.TestCheckTypeSetElemAttr(tcoPoliciesTracesResourceName, "policies.1.actions.names.*", "action-name2"),
					resource.TestCheckResourceAttr(tcoPoliciesTracesResourceName, "policies.1.services.rule_type", "is"),
					resource.TestCheckResourceAttr(tcoPoliciesTracesResourceName, "policies.1.services.names.#", "2"),
					resource.TestCheckTypeSetElemAttr(tcoPoliciesTracesResourceName, "policies.1.services.names.*", "service-name"),
					resource.TestCheckTypeSetElemAttr(tcoPoliciesTracesResourceName, "policies.1.services.names.*", "service-name2"),
					resource.TestCheckResourceAttr(tcoPoliciesTracesResourceName, "policies.1.tags.tags.http.method.rule_type", "includes"),
					resource.TestCheckResourceAttr(tcoPoliciesTracesResourceName, "policies.1.tags.tags.http.method.names.#", "2"),
					resource.TestCheckTypeSetElemAttr(tcoPoliciesTracesResourceName, "policies.1.tags.tags.http.method.names.*", "GET"),
					resource.TestCheckTypeSetElemAttr(tcoPoliciesTracesResourceName, "policies.1.tags.tags.http.method.names.*", "POST"),

					resource.TestCheckResourceAttr(tcoPoliciesTracesResourceName, "policies.2.name", "Example tco_policy from terraform 3"),
					resource.TestCheckResourceAttr(tcoPoliciesTracesResourceName, "policies.2.priority", "high"),
					resource.TestCheckResourceAttr(tcoPoliciesTracesResourceName, "policies.2.order", "3"),
					resource.TestCheckResourceAttr(tcoPoliciesTracesResourceName, "policies.2.applications.rule_type", "starts_with"),
					resource.TestCheckResourceAttr(tcoPoliciesTracesResourceName, "policies.2.applications.names.0", "prod"),
					resource.TestCheckResourceAttr(tcoPoliciesTracesResourceName, "policies.2.subsystems.rule_type", "is_not"),
					resource.TestCheckResourceAttr(tcoPoliciesTracesResourceName, "policies.2.subsystems.names.#", "2"),
					resource.TestCheckTypeSetElemAttr(tcoPoliciesTracesResourceName, "policies.2.subsystems.names.*", "mobile"),
					resource.TestCheckTypeSetElemAttr(tcoPoliciesTracesResourceName, "policies.2.subsystems.names.*", "web"),
					resource.TestCheckResourceAttr(tcoPoliciesTracesResourceName, "policies.2.actions.rule_type", "is"),
					resource.TestCheckResourceAttr(tcoPoliciesTracesResourceName, "policies.2.actions.names.#", "2"),
					resource.TestCheckTypeSetElemAttr(tcoPoliciesTracesResourceName, "policies.2.actions.names.*", "action-name"),
					resource.TestCheckTypeSetElemAttr(tcoPoliciesTracesResourceName, "policies.2.actions.names.*", "action-name2"),
					resource.TestCheckResourceAttr(tcoPoliciesTracesResourceName, "policies.2.services.rule_type", "is"),
					resource.TestCheckResourceAttr(tcoPoliciesTracesResourceName, "policies.2.services.names.#", "2"),
					resource.TestCheckTypeSetElemAttr(tcoPoliciesTracesResourceName, "policies.2.services.names.*", "service-name"),
					resource.TestCheckTypeSetElemAttr(tcoPoliciesTracesResourceName, "policies.2.services.names.*", "service-name2"),
					resource.TestCheckResourceAttr(tcoPoliciesTracesResourceName, "policies.2.tags.tags.http.method.rule_type", "includes"),
					resource.TestCheckResourceAttr(tcoPoliciesTracesResourceName, "policies.2.tags.tags.http.method.names.#", "2"),
					resource.TestCheckTypeSetElemAttr(tcoPoliciesTracesResourceName, "policies.2.tags.tags.http.method.names.*", "GET"),
					resource.TestCheckTypeSetElemAttr(tcoPoliciesTracesResourceName, "policies.2.tags.tags.http.method.names.*", "POST"),
				),
			},
		},
	})
}

func testAccTCOPoliciesTracesCheckDestroy(s *terraform.State) error {
	client := testAccProvider.Meta().(*clientset.ClientSet).TCOPolicies()
	ctx := context.TODO()
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "coralogix_tco_policies_traces" {
			continue
		}

		if resp, err := client.List(ctx, &cxsdk.GetCompanyPoliciesRequest{SourceType: &tracesSource}); err == nil {
			if err == nil && len(resp.Policies) > 0 {
				return fmt.Errorf("tco-policies still exists: %s", protojson.Format(resp))
			}
		}
	}

	return nil
}

func testAccCoralogixResourceTCOPoliciesTraces() string {
	return `resource "coralogix_tco_policies_traces" "test"{
				policies = [
				{
				  name       = "Example tco_policy from terraform 1"
				  priority   = "low"
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
				},
				{
				  name       = "Example tco_policy from terraform 2"
				  priority   = "medium"
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
				},
				{
				  name       = "Example tco_policy from terraform 3"
				  priority   = "high"
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
				]
			}
	`
}
