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
	"terraform-provider-coralogix/coralogix/clientset"
	"testing"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccCoralogixResourceScope(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { TestAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckScopeDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceScope(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("coralogix_scope.test", "id"),
					resource.TestCheckResourceAttr("coralogix_scope.test", "display_name", "ExampleScope"),
					resource.TestCheckResourceAttr("coralogix_scope.test", "default_expression", "<v1>true"),
					resource.TestCheckResourceAttr("coralogix_scope.test", "filters.0.entity_type", "logs"),
					resource.TestCheckResourceAttr("coralogix_scope.test", "filters.0.expression", "<v1>(subsystemName == 'purchases') || (subsystemName == 'signups')"),
				),
			},
			{
				ResourceName:      "coralogix_scope.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccCoralogixResourceUpdatedScope(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("coralogix_scope.test_upgraded", "id"),
					resource.TestCheckResourceAttr("coralogix_scope.test_upgraded", "display_name", "NewExampleScope"),
					resource.TestCheckResourceAttr("coralogix_scope.test_upgraded", "default_expression", "<v1>true"),
					resource.TestCheckResourceAttr("coralogix_scope.test_upgraded", "filters.0.entity_type", "logs"),
					resource.TestCheckResourceAttr("coralogix_scope.test_upgraded", "filters.0.expression", "<v1>(subsystemName == 'purchases') || (subsystemName == 'signups')"),
				),
			},
		},
	})
}

func testAccCheckScopeDestroy(s *terraform.State) error {
	client := testAccProvider.Meta().(*clientset.ClientSet).Scopes()
	ctx := context.TODO()

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "coralogix_scope" {
			continue
		}

		resp, err := client.Get(ctx, &cxsdk.GetTeamScopesByIDsRequest{
			Ids: []string{rs.Primary.ID},
		})
		if err == nil && resp != nil && resp.Scopes != nil && len(resp.Scopes) > 0 {
			return fmt.Errorf("Scope still exists: %v", rs.Primary.ID)
		}
	}
	return nil
}

func testAccCoralogixResourceScope() string {
	return `resource "coralogix_scope" "test" {
		display_name       = "ExampleScope"
		default_expression = "<v1>true"
		filters            = [
		  {
			entity_type = "logs"
			expression  = "<v1>(subsystemName == 'purchases') || (subsystemName == 'signups')"
		  }
		]
	}
	`
}

func testAccCoralogixResourceUpdatedScope() string {
	return `resource "coralogix_scope" "test_upgraded" {  
		display_name       = "NewExampleScope"
		default_expression = "<v1>true"
		filters            = [
		{
			entity_type = "logs"
			expression  = "<v1>(subsystemName == 'purchases') || (subsystemName == 'signups')"
		}
		]
	}
	`
}
