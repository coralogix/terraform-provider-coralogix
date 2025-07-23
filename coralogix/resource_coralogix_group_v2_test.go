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
	"strconv"
	"testing"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	"terraform-provider-coralogix/coralogix/clientset"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

var groupV2ResourceName = "coralogix_group_v2.test"

func TestAccCoralogixResourceGroupV2(t *testing.T) {
	userName := randUserName()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { TestAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckGroupV2Destroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceGroupV2(userName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(groupV2ResourceName, "id"),
					resource.TestCheckResourceAttr(groupV2ResourceName, "name", "example"),
					resource.TestCheckResourceAttr(groupV2ResourceName, "roles.#", "1"),
					resource.TestCheckResourceAttr(groupV2ResourceName, "roles.0.id", "1"),
					resource.TestCheckResourceAttr(groupV2ResourceName, "scope.%", "2"),
					resource.TestCheckResourceAttr(groupV2ResourceName, "scope.filters.subsystems.%", "2"),
				),
			},
			{
				ResourceName:      groupV2ResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccCoralogixResourceGroupV2(userName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(groupV2ResourceName, "id"),
					resource.TestCheckResourceAttr(groupV2ResourceName, "name", "example"),
					resource.TestCheckResourceAttr(groupV2ResourceName, "roles.#", "1"),
					resource.TestCheckResourceAttr(groupV2ResourceName, "roles.0.id", "1"),
					resource.TestCheckResourceAttr(groupV2ResourceName, "scope.%", "2"),
					resource.TestCheckResourceAttr(groupV2ResourceName, "scope.filters.%", "2"),
					resource.TestCheckResourceAttr(groupV2ResourceName, "users.#", "1"),
					resource.TestCheckResourceAttr(groupV2ResourceName, "users.0.user_name", userName),
				),
			},
		},
	})
}

func testAccCheckGroupV2Destroy(s *terraform.State) error {
	client := testAccProvider.Meta().(*clientset.ClientSet).GroupGrpc()

	ctx := context.TODO()

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "coralogix_group_v2" {
			continue
		}

		id, err := strconv.Atoi(rs.Primary.ID)
		if err != nil {
			return fmt.Errorf("invalid group ID: %s", rs.Primary.ID)
		}

		resp, err := client.Get(ctx, &cxsdk.GetTeamGroupRequest{GroupId: &cxsdk.TeamGroupID{Id: uint32(id)}})
		if err == nil {
			if resp.Group != nil && resp.Group.GroupId != nil {
				return fmt.Errorf("group still exists: %s", rs.Primary.ID)
			}
		}
	}

	return nil
}

func testAccCoralogixResourceGroupV2(userName string) string {
	return fmt.Sprintf(`
	resource "coralogix_scope" "test" {
		display_name       = "ExampleScope"
		default_expression = "<v1>true"
		filters            = [
		{
			entity_type = "logs"
			expression  = "<v1>(subsystemName == 'purchases') || (subsystemName == 'signups')"
		}
		]
	}

	resource "coralogix_user" "test" {
		user_name = "%s"
	}
	
	resource "coralogix_group_v2" "test" {
		name = "example"
		roles       = [
			{
      			id = "1"
    		},
		]
		scope = {
    		filters = {
      			subsystem = [
				{
          			filter_type = "exact"
          			term        = "purchases"
        		},
	  			{
          			filter_type = "exact"
          			term        = "signups"
				}
				]
			}
		}
	}

	resource "coralogix_group_attachment" "example" {
  		group_id = coralogix_group_v2.test.id
  		user_ids = [coralogix_user.test.id]
	}
`, userName)
}
