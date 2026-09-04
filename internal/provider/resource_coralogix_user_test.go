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
	"strings"
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"

	usersservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/users_management_service"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

var userResourceName = "coralogix_user.test"

func TestAccCoralogixResourceUser(t *testing.T) {
	userName := randUserName()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckUserDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceUser(userName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(userResourceName, "id"),
					resource.TestCheckResourceAttr(userResourceName, "user_name", userName),
					resource.TestCheckResourceAttr(userResourceName, "name.given_name", "Test"),
					resource.TestCheckResourceAttr(userResourceName, "name.family_name", "User"),
				),
			},
			{
				ResourceName:      userResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TestAccCoralogixResourceUserBackwardsCompatibility pins every part of the state
// contract the SCIM implementation produced: the computed emails and groups sets, the
// stable UUID id, an in-place name update, an active toggle, import by UUID, and a
// second plan with no drift.
func TestAccCoralogixResourceUserBackwardsCompatibility(t *testing.T) {
	userName := randUserName()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckUserDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceUser(userName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(userResourceName, "id"),
					resource.TestCheckResourceAttr(userResourceName, "user_name", userName),
					resource.TestCheckResourceAttr(userResourceName, "active", "true"),
					// SCIM returned exactly one primary work email equal to the username.
					resource.TestCheckResourceAttr(userResourceName, "emails.#", "1"),
					resource.TestCheckTypeSetElemNestedAttrs(userResourceName, "emails.*", map[string]string{
						"primary": "true",
						"type":    "work",
						"value":   userName,
					}),
					// A new user belongs to no group, but the set has to be known and empty
					// rather than null, so other configuration can index into it.
					resource.TestCheckResourceAttr(userResourceName, "groups.#", "0"),
				),
			},
			// A case-only username change must not produce a diff.
			{
				Config:   testAccCoralogixResourceUser(strings.ToUpper(userName)),
				PlanOnly: true,
			},
			// Name updates happen in place; the id must not change.
			{
				Config: testAccCoralogixResourceUserNamed(userName, "Updated", "Person"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply:             []plancheck.PlanCheck{plancheck.ExpectResourceAction(userResourceName, plancheck.ResourceActionUpdate)},
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(userResourceName, "name.given_name", "Updated"),
					resource.TestCheckResourceAttr(userResourceName, "name.family_name", "Person"),
					resource.TestCheckResourceAttr(userResourceName, "user_name", userName),
				),
			},
			// Deactivating and reactivating both work in place.
			{
				Config: testAccCoralogixResourceUserActive(userName, false),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: resource.TestCheckResourceAttr(userResourceName, "active", "false"),
			},
			{
				Config: testAccCoralogixResourceUserActive(userName, true),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: resource.TestCheckResourceAttr(userResourceName, "active", "true"),
			},
			{
				ResourceName:      userResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TestAccCoralogixResourceUserGroupMembership proves the computed groups set still
// reports memberships, which the Users API does not return and the provider has to
// rebuild from the Groups API.
func TestAccCoralogixResourceUserGroupMembership(t *testing.T) {
	userName := randUserName()
	groupName := acctest.RandomWithPrefix("tf-acc-user-group")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckUserDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceUserInGroup(userName, groupName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("coralogix_group.test", "members.#", "1"),
					// The group is created after the user, so the membership only appears on
					// the refresh that follows. The data source reads the user again.
					resource.TestCheckResourceAttr("data.coralogix_user.by_id", "groups.#", "1"),
					resource.TestCheckResourceAttr("data.coralogix_user.by_id", "emails.#", "1"),
					resource.TestCheckResourceAttr("data.coralogix_user.by_name", "user_name", userName),
					resource.TestCheckResourceAttrPair("data.coralogix_user.by_name", "id", userResourceName, "id"),
				),
			},
		},
	})
}

// TestAccCoralogixResourceUserOutOfBandDeactivation covers the drift the migration
// makes possible: destroy is now a status change, so a user deactivated outside
// Terraform looks exactly like a destroyed one. The refresh has to report it and the
// next apply has to bring the user back, rather than treating the resource as gone.
func TestAccCoralogixResourceUserOutOfBandDeactivation(t *testing.T) {
	userName := randUserName()
	config := testAccCoralogixResourceUser(userName)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckUserDestroy,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check:  resource.TestCheckResourceAttr(userResourceName, "active", "true"),
			},
			{
				PreConfig: func() { testAccSetUserActiveOutOfBand(t, userName, false) },
				Config:    config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(userResourceName, plancheck.ResourceActionUpdate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: resource.TestCheckResourceAttr(userResourceName, "active", "true"),
			},
		},
	})
}

// TestAccCoralogixResourceUserDestroyWhenAlreadyInactive checks that destroy is
// idempotent. Destroy deactivates the user, so destroying one that somebody already
// deactivated has nothing left to do and must still succeed. The test leaves the user
// inactive without applying, and the automatic destroy at the end is the assertion.
func TestAccCoralogixResourceUserDestroyWhenAlreadyInactive(t *testing.T) {
	userName := randUserName()
	config := testAccCoralogixResourceUser(userName)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckUserDestroy,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check:  resource.TestCheckResourceAttr(userResourceName, "active", "true"),
			},
			{
				PreConfig:          func() { testAccSetUserActiveOutOfBand(t, userName, false) },
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// testAccSetUserActiveOutOfBand changes a user's status behind Terraform's back.
func testAccSetUserActiveOutOfBand(t *testing.T, userName string, active bool) {
	t.Helper()

	cs := testAccProvider.Meta().(*clientset.ClientSet)
	ctx := context.TODO()

	teamID, err := cs.TeamID(ctx)
	if err != nil {
		t.Fatalf("resolving team id: %s", err)
	}

	searchResp, _, err := cs.Users().UsersMgmtServiceSearchUsers(ctx, teamID).
		Username(userName).
		PageSize(100).
		Execute()
	if err != nil {
		t.Fatalf("searching for %s: %s", userName, err)
	}

	var accountID int64
	for _, user := range searchResp.Users {
		if strings.EqualFold(user.GetUsername(), userName) {
			accountID = user.GetUserAccountId()
			break
		}
	}
	if accountID == 0 {
		t.Fatalf("user %s not found, or returned without a userAccountId", userName)
	}

	status := usersservice.USERSTATUS_USER_STATUS_INACTIVE
	if active {
		status = usersservice.USERSTATUS_USER_STATUS_ACTIVE
	}
	if _, _, err := cs.Users().UsersMgmtServiceUpdateUsersStatuses(ctx, teamID).
		UpdateUserStatusRequest(usersservice.UpdateUserStatusRequest{
			Status:         &status,
			UserAccountIds: []int64{accountID},
		}).Execute(); err != nil {
		t.Fatalf("setting %s status to %s: %s", userName, status, err)
	}
}

func testAccCheckUserDestroy(s *terraform.State) error {
	cs := testAccProvider.Meta().(*clientset.ClientSet)
	ctx := context.TODO()

	teamID, err := cs.TeamID(ctx)
	if err != nil {
		return err
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "coralogix_user" {
			continue
		}

		searchResp, _, err := cs.Users().UsersMgmtServiceSearchUsers(ctx, teamID).PageSize(100).Execute()
		if err != nil {
			return err
		}
		for _, user := range searchResp.Users {
			if user.GetUserId() != rs.Primary.ID {
				continue
			}
			if user.GetStatus() == usersservice.USERSTATUS_USER_STATUS_ACTIVE {
				return fmt.Errorf("user still exists and active: %s", rs.Primary.ID)
			}
		}
	}

	return nil
}

func randUserName() string {
	return fmt.Sprintf("%s@coralogix.com", acctest.RandomWithPrefix("tf-acc-user"))
}

func testAccCoralogixResourceUser(userName string) string {
	return testAccCoralogixResourceUserNamed(userName, "Test", "User")
}

func testAccCoralogixResourceUserNamed(userName, givenName, familyName string) string {
	return fmt.Sprintf(`
	resource "coralogix_user" "test" {
	  user_name = "%s"
	  name = {
		given_name = "%s"
		family_name = "%s"
      }
	}
`, userName, givenName, familyName)
}

func testAccCoralogixResourceUserActive(userName string, active bool) string {
	return fmt.Sprintf(`
	resource "coralogix_user" "test" {
	  user_name = "%s"
	  name = {
		given_name = "Test"
		family_name = "User"
      }
	  active = %t
	}
`, userName, active)
}

func testAccCoralogixResourceUserInGroup(userName, groupName string) string {
	return fmt.Sprintf(`
	resource "coralogix_user" "test" {
	  user_name = "%s"
	  name = {
		given_name = "Test"
		family_name = "User"
      }
	}

	resource "coralogix_group" "test" {
	  display_name = "%s"
	  role         = "Read Only"
	  members      = [coralogix_user.test.id]
	}

	data "coralogix_user" "by_id" {
	  id         = coralogix_user.test.id
	  depends_on = [coralogix_group.test]
	}

	data "coralogix_user" "by_name" {
	  user_name  = coralogix_user.test.user_name
	  depends_on = [coralogix_group.test]
	}
`, userName, groupName)
}
