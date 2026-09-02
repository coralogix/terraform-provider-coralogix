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
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

var groupResourceName = "coralogix_group.test"
var groupUnmanagedMembersResourceName = "coralogix_group.unmanaged_members"
var groupOmittedMembersResourceName = "coralogix_group.omitted_members"

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

func TestAccCoralogixResourceGroupMembersManagedByAttachment(t *testing.T) {
	firstUserName := randUserName()
	secondUserName := randUserName()
	displayName := acctest.RandomWithPrefix("tf-acc-test-group")
	scopeName := acctest.RandomWithPrefix("tf-acc-test-scope")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckGroupDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceGroupUnmanagedMembers(firstUserName, secondUserName, displayName, scopeName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(groupUnmanagedMembersResourceName, "display_name", displayName),
					testAccCheckGroupMemberCount(groupUnmanagedMembersResourceName, 2),
				),
			},
			{
				Config: testAccCoralogixResourceGroupUnmanagedMembers(firstUserName, secondUserName, displayName+"-renamed", scopeName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(groupUnmanagedMembersResourceName, "display_name", displayName+"-renamed"),
					resource.TestCheckResourceAttr(groupUnmanagedMembersResourceName, "members.#", "2"),
					testAccCheckGroupMemberCount(groupUnmanagedMembersResourceName, 2),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

func TestAccCoralogixResourceGroupMembersOmissionAndExplicitClear(t *testing.T) {
	userName := randUserName()
	displayName := acctest.RandomWithPrefix("tf-acc-test-group")
	scopeName := acctest.RandomWithPrefix("tf-acc-test-scope")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckGroupDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceGroupWithMembers(userName, displayName, scopeName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(groupOmittedMembersResourceName, "members.#", "1"),
					resource.TestCheckResourceAttrPair(groupOmittedMembersResourceName, "members.0", "coralogix_user.omitted_members", "id"),
					testAccCheckGroupMemberCount(groupOmittedMembersResourceName, 1),
				),
			},
			{
				// Dropping the argument stops managing membership; it must not remove anyone.
				Config: testAccCoralogixResourceGroupWithoutMembers(userName, displayName+"-renamed", scopeName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(groupOmittedMembersResourceName, "display_name", displayName+"-renamed"),
					resource.TestCheckResourceAttr(groupOmittedMembersResourceName, "members.#", "1"),
					testAccCheckGroupMemberCount(groupOmittedMembersResourceName, 1),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				ResourceName:      groupOmittedMembersResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				// Removing everyone is only possible through an explicit empty list.
				Config: testAccCoralogixResourceGroupWithEmptyMembers(userName, displayName+"-renamed", scopeName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(groupOmittedMembersResourceName, "members.#", "0"),
					testAccCheckGroupMemberCount(groupOmittedMembersResourceName, 0),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

func TestAccCoralogixResourceGroupRoleByName(t *testing.T) {
	for _, role := range []string{"Read Only", "Legacy Read Only"} {
		role := role
		t.Run(role, func(t *testing.T) {
			userName := randUserName()
			displayName := acctest.RandomWithPrefix("tf-acc-test-group")
			updatedName := displayName + "-renamed"
			scopeName := acctest.RandomWithPrefix("tf-acc-test-scope")
			initial := testAccCoralogixResourceGroupWithRole(userName, displayName, scopeName, role)
			updated := testAccCoralogixResourceGroupWithRole(userName, updatedName, scopeName, role)

			resource.Test(t, resource.TestCase{
				PreCheck:                 func() { testAccPreCheck(t) },
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				CheckDestroy:             testAccCheckGroupDestroy,
				Steps: []resource.TestStep{
					{
						Config: initial,
						Check: resource.ComposeAggregateTestCheckFunc(
							resource.TestCheckResourceAttrSet(groupResourceName, "id"),
							resource.TestCheckResourceAttr(groupResourceName, "display_name", displayName),
							resource.TestCheckResourceAttr(groupResourceName, "role", role),
							testAccCheckGroupRoleAssigned(groupResourceName, role),
						),
					},
					{
						Config:   initial,
						PlanOnly: true,
					},
					{
						Config: updated,
						Check: resource.ComposeAggregateTestCheckFunc(
							resource.TestCheckResourceAttr(groupResourceName, "display_name", updatedName),
							resource.TestCheckResourceAttr(groupResourceName, "role", role),
							testAccCheckGroupRoleAssigned(groupResourceName, role),
						),
						ConfigPlanChecks: resource.ConfigPlanChecks{
							PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
						},
					},
				},
			})
		})
	}
}

func testAccCheckGroupMemberCount(resourceAddress string, expected int) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceAddress]
		if !ok {
			return fmt.Errorf("%s not found in state", resourceAddress)
		}

		client, err := testAccTeamGroupsClient()
		if err != nil {
			return err
		}

		userIDs, err := testAccListGroupUserIDs(context.TODO(), client, rs.Primary.ID)
		if err != nil {
			return fmt.Errorf("error getting group %s: %w", rs.Primary.ID, err)
		}
		if len(userIDs) != expected {
			return fmt.Errorf("expected group %s to have %d member(s) in Coralogix, got %d", rs.Primary.ID, expected, len(userIDs))
		}

		return nil
	}
}

func testAccCheckGroupRoleAssigned(resourceAddress, configuredRole string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceAddress]
		if !ok {
			return fmt.Errorf("%s not found in state", resourceAddress)
		}

		client, err := testAccTeamGroupsClient()
		if err != nil {
			return err
		}

		id, err := strconv.ParseInt(rs.Primary.ID, 10, 64)
		if err != nil {
			return fmt.Errorf("parse group id %q: %w", rs.Primary.ID, err)
		}

		resp, httpResp, err := client.GroupsMgmtServiceGetTeamGroup(context.TODO(), id).Execute()
		if err != nil {
			return fmt.Errorf("get group: %s", utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResp, err), "Get", rs.Primary.ID))
		}
		if resp == nil || resp.Group == nil || resp.Group.Role == nil || resp.Group.Role.GetName() == "" {
			return fmt.Errorf("group %s has no assigned role", rs.Primary.ID)
		}

		apiRole := resp.Group.Role.GetName()
		if apiRole != configuredRole {
			return fmt.Errorf("group %s role is %q, configured %q", rs.Primary.ID, apiRole, configuredRole)
		}
		return nil
	}
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

func testAccCoralogixResourceGroupWithRole(userName, displayName, scopeName, role string) string {
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
		role         = %q
		members      = [coralogix_user.test.id]
		scope_id     = coralogix_scope.test.id
	}
`, scopeName, userName, displayName, role)
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

func testAccCoralogixResourceGroupUnmanagedMembers(firstUserName, secondUserName, displayName, scopeName string) string {
	return fmt.Sprintf(`
	resource "coralogix_scope" "unmanaged_members" {
		display_name       = "%s"
		default_expression = "<v1>true"
		filters            = [
		{
			entity_type = "logs"
			expression  = "<v1>(subsystemName == 'purchases') || (subsystemName == 'signups')"
		}
		]
	}

	resource "coralogix_user" "unmanaged_members_first" {
		user_name = "%s"
	}

	resource "coralogix_user" "unmanaged_members_second" {
		user_name = "%s"
	}

	resource "coralogix_group" "unmanaged_members" {
		display_name = "%s"
		role         = "Read Only"
		scope_id     = coralogix_scope.unmanaged_members.id
	}

	resource "coralogix_group_attachment" "unmanaged_members" {
		group_id = coralogix_group.unmanaged_members.id
		user_ids = [
			coralogix_user.unmanaged_members_first.id,
			coralogix_user.unmanaged_members_second.id,
		]
	}
`, scopeName, firstUserName, secondUserName, displayName)
}

func testAccCoralogixResourceGroupWithMembers(userName, displayName, scopeName string) string {
	return fmt.Sprintf(`
	resource "coralogix_scope" "omitted_members" {
		display_name       = "%s"
		default_expression = "<v1>true"
		filters            = [
		{
			entity_type = "logs"
			expression  = "<v1>(subsystemName == 'purchases') || (subsystemName == 'signups')"
		}
		]
	}

	resource "coralogix_user" "omitted_members" {
		user_name = "%s"
	}

	resource "coralogix_group" "omitted_members" {
		display_name = "%s"
		role         = "Read Only"
		members      = [coralogix_user.omitted_members.id]
		scope_id     = coralogix_scope.omitted_members.id
	}
`, scopeName, userName, displayName)
}

func testAccCoralogixResourceGroupWithoutMembers(userName, displayName, scopeName string) string {
	return fmt.Sprintf(`
	resource "coralogix_scope" "omitted_members" {
		display_name       = "%s"
		default_expression = "<v1>true"
		filters            = [
		{
			entity_type = "logs"
			expression  = "<v1>(subsystemName == 'purchases') || (subsystemName == 'signups')"
		}
		]
	}

	resource "coralogix_user" "omitted_members" {
		user_name = "%s"
	}

	resource "coralogix_group" "omitted_members" {
		display_name = "%s"
		role         = "Read Only"
		scope_id     = coralogix_scope.omitted_members.id
	}
`, scopeName, userName, displayName)
}

func testAccCoralogixResourceGroupWithEmptyMembers(userName, displayName, scopeName string) string {
	return fmt.Sprintf(`
	resource "coralogix_scope" "omitted_members" {
		display_name       = "%s"
		default_expression = "<v1>true"
		filters            = [
		{
			entity_type = "logs"
			expression  = "<v1>(subsystemName == 'purchases') || (subsystemName == 'signups')"
		}
		]
	}

	resource "coralogix_user" "omitted_members" {
		user_name = "%s"
	}

	resource "coralogix_group" "omitted_members" {
		display_name = "%s"
		role         = "Read Only"
		members      = []
		scope_id     = coralogix_scope.omitted_members.id
	}
`, scopeName, userName, displayName)
}
