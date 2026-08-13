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

package provider

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"

	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

var groupResourceName = "coralogix_group.test"

func TestAccCoralogixResourceGroup(t *testing.T) {
	userName := randUserName()
	userName2 := randUserName()
	displayName := acctest.RandomWithPrefix("tf-acc-test-group")
	scopeName := acctest.RandomWithPrefix("tf-acc-test-scope")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckGroupDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceGroup(userName, displayName, scopeName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(groupResourceName, "id"),
					resource.TestCheckResourceAttr(groupResourceName, "display_name", displayName),
					resource.TestCheckResourceAttr(groupResourceName, "role", "Read Only"),
					resource.TestCheckResourceAttr(groupResourceName, "members.#", "1"),
					resource.TestCheckResourceAttrPair(groupResourceName, "members.0", "coralogix_user.test", "id"),
					resource.TestCheckResourceAttrPair(groupResourceName, "scope_id", "coralogix_scope.test", "id"),
				),
			},
			{
				Config:   testAccCoralogixResourceGroup(userName, displayName, scopeName),
				PlanOnly: true,
			},
			{
				ResourceName:      groupResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccCoralogixResourceGroupUpdatedMembers(userName, userName2, displayName, scopeName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(groupResourceName, "members.#", "2"),
					resource.TestCheckResourceAttrPair(groupResourceName, "scope_id", "coralogix_scope.test", "id"),
				),
			},
			{
				Config: testAccCoralogixResourceGroupNoScope(userName, userName2, displayName, scopeName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(groupResourceName, "display_name", displayName),
					resource.TestCheckResourceAttr(groupResourceName, "members.#", "2"),
					resource.TestCheckResourceAttrPair(groupResourceName, "scope_id", "coralogix_scope.test", "id"),
				),
			},
			{
				Config:   testAccCoralogixResourceGroupNoScope(userName, userName2, displayName, scopeName),
				PlanOnly: true,
			},
		},
	})
}

func testAccCheckGroupDestroy(s *terraform.State) error {
	clients, err := testAccNewClientSet()
	if err != nil {
		return err
	}
	client := clients.TeamGroups()
	ctx := context.TODO()

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "coralogix_group" {
			continue
		}

		id, err := strconv.ParseInt(rs.Primary.ID, 10, 64)
		if err != nil {
			return fmt.Errorf("parse group id %q: %w", rs.Primary.ID, err)
		}

		resp, httpResp, err := client.GroupsMgmtServiceGetTeamGroup(ctx, id).Execute()
		if err != nil {
			apiErr := cxsdkOpenapi.NewAPIError(httpResp, err)
			if cxsdkOpenapi.IsNotFound(apiErr) {
				continue
			}
			return fmt.Errorf("get group: %s", utils.FormatOpenAPIErrors(apiErr, "Get", rs.Primary.ID))
		}
		if resp != nil && resp.Group != nil {
			return fmt.Errorf("group still exists: %s", rs.Primary.ID)
		}
	}

	return nil
}

func testAccCoralogixResourceGroup(userName, displayName, scopeName string) string {
	return fmt.Sprintf(`
	resource "coralogix_scope" "test" {
		display_name       = "%s"
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
	
	resource "coralogix_group" "test" {
		display_name = "%s"
		role         = "Read Only"
		members      = [coralogix_user.test.id]
		scope_id     = coralogix_scope.test.id
	}
`, scopeName, userName, displayName)
}

func testAccCoralogixResourceGroupUpdatedMembers(userName, userName2, displayName, scopeName string) string {
	return fmt.Sprintf(`
	resource "coralogix_scope" "test" {
		display_name       = "%s"
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

	resource "coralogix_user" "test2" {
		user_name = "%s"
	}
	
	resource "coralogix_group" "test" {
		display_name = "%s"
		role         = "Read Only"
		members      = [coralogix_user.test.id, coralogix_user.test2.id]
		scope_id     = coralogix_scope.test.id
	}
`, scopeName, userName, userName2, displayName)
}

func testAccCoralogixResourceGroupNoScope(userName, userName2, displayName, scopeName string) string {
	return fmt.Sprintf(`
	resource "coralogix_scope" "test" {
		display_name       = "%s"
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

	resource "coralogix_user" "test2" {
		user_name = "%s"
	}
	
	resource "coralogix_group" "test" {
		display_name = "%s"
		role         = "Read Only"
		members      = [coralogix_user.test.id, coralogix_user.test2.id]
	}
`, scopeName, userName, userName2, displayName)
}
