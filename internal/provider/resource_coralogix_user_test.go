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
	"slices"
	"strings"
	"testing"

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
					// SCIM created users without a login mode, and create still sends none.
					testAccCheckUserLoginModes(userName),
				),
			},
			{
				ResourceName:      userResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Import by email resolves to the same state as import by id.
			{
				ResourceName:      userResourceName,
				ImportState:       true,
				ImportStateId:     userName,
				ImportStateVerify: true,
			},
		},
	})
}

// TestAccCoralogixResourceUserCreateInactive checks that one create call honours
// `active = false`. The create template carries the status, and the resource makes no
// second call, so a backend that ignores it fails the first apply.
func TestAccCoralogixResourceUserCreateInactive(t *testing.T) {
	userName := randUserName()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckUserDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceUserActive(userName, false),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(userResourceName, "active", "false"),
					testAccCheckUserLoginModes(userName),
				),
			},
			{
				Config: testAccCoralogixResourceUserActive(userName, true),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply:             []plancheck.PlanCheck{plancheck.ExpectResourceAction(userResourceName, plancheck.ResourceActionUpdate)},
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: resource.TestCheckResourceAttr(userResourceName, "active", "true"),
			},
		},
	})
}

// TestAccCoralogixResourceUserKeepsLoginModes checks the echo. PUT replaces the whole
// template, so an update or a destroy that did not send back the login modes the user
// already has would wipe them. Here they are set outside Terraform, the way an SSO
// setup or the UI would set them.
func TestAccCoralogixResourceUserKeepsLoginModes(t *testing.T) {
	userName := randUserName()
	sso := usersservice.ALLOWEDLOGINMODE_ALLOWED_LOGIN_MODE_SSO
	local := usersservice.ALLOWEDLOGINMODE_ALLOWED_LOGIN_MODE_LOCAL

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: resource.ComposeTestCheckFunc(
			testAccCheckUserDestroy,
			testAccCheckUserLoginModes(userName, sso, local),
		),
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceUser(userName),
				Check:  testAccCheckUserLoginModes(userName),
			},
			// The login modes change outside Terraform. They are not in the schema, so
			// the plan stays empty.
			{
				PreConfig: func() {
					testAccPutUserOutOfBand(t, userName, func(template *usersservice.UserTemplate) {
						template.AllowedLoginMode = []usersservice.AllowedLoginMode{sso, local}
					})
				},
				Config:   testAccCoralogixResourceUser(userName),
				PlanOnly: true,
			},
			// A rename and a deactivation each send one PUT, and both have to carry the
			// login modes through. Destroy is checked the same way in CheckDestroy.
			{
				Config: testAccCoralogixResourceUserNamed(userName, "Renamed", "Person"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(userResourceName, "name.given_name", "Renamed"),
					testAccCheckUserLoginModes(userName, sso, local),
				),
			},
			{
				Config: testAccCoralogixResourceUserActive(userName, false),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(userResourceName, "active", "false"),
					testAccCheckUserLoginModes(userName, sso, local),
				),
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
// reports memberships. The Users API returns them as numeric groupIds, which have to
// match the group resource ids the way SCIM groups[].value did.
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
					// The value has to be the group id itself, as SCIM groups[].value was,
					// or every migrated user would show a groups diff.
					resource.TestCheckTypeSetElemAttrPair("data.coralogix_user.by_id", "groups.*", "coralogix_group.test", "id"),
					resource.TestCheckTypeSetElemAttrPair("data.coralogix_user.by_name", "groups.*", "coralogix_group.test", "id"),
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

// testAccFindUser searches for a user by username the way the provider does.
// It builds its own client: testAccProvider is only configured once another test has
// run, and the migration test runs on its own.
func testAccFindUser(ctx context.Context, userName string) (*usersservice.RbacV2User, error) {
	cs, err := testAccNewClientSet()
	if err != nil {
		return nil, err
	}
	searchResp, _, err := cs.Users().UsersMgmtServiceSearchUsers(ctx).
		Username(userName).
		PageSize(100).
		Execute()
	if err != nil {
		return nil, fmt.Errorf("searching for %s: %w", userName, err)
	}
	for i, user := range searchResp.Users {
		if strings.EqualFold(user.GetUsername(), userName) {
			return &searchResp.Users[i], nil
		}
	}
	return nil, nil
}

// testAccPutUserOutOfBand changes a user behind Terraform's back. PUT replaces the
// template, so everything the change does not touch is sent back as read.
func testAccPutUserOutOfBand(t *testing.T, userName string, change func(template *usersservice.UserTemplate)) {
	t.Helper()
	ctx := context.TODO()

	user, err := testAccFindUser(ctx, userName)
	if err != nil {
		t.Fatal(err)
	}
	if user == nil {
		t.Fatalf("user %s not found", userName)
	}

	template := &usersservice.UserTemplate{
		FirstName:        user.FirstName,
		LastName:         user.LastName,
		Status:           user.Status,
		AllowedLoginMode: user.AllowedLoginMode,
		AccessType:       user.AccessType,
	}
	change(template)

	cs, err := testAccNewClientSet()
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := cs.Users().UsersMgmtServiceUpdateUsers(ctx).
		UpdateUserRequest([]usersservice.UpdateUserRequest{{UserId: user.UserId, UserTemplate: template}}).
		Execute(); err != nil {
		t.Fatalf("updating %s: %s", userName, err)
	}
}

func testAccSetUserActiveOutOfBand(t *testing.T, userName string, active bool) {
	t.Helper()
	status := usersservice.USERSTATUS_USER_STATUS_INACTIVE
	if active {
		status = usersservice.USERSTATUS_USER_STATUS_ACTIVE
	}
	testAccPutUserOutOfBand(t, userName, func(template *usersservice.UserTemplate) {
		template.Status = &status
	})
}

// testAccCheckUserLoginModes checks the login modes the backend holds. The resource has
// no attribute for them, so only the API can show them.
func testAccCheckUserLoginModes(userName string, want ...usersservice.AllowedLoginMode) resource.TestCheckFunc {
	return func(*terraform.State) error {
		user, err := testAccFindUser(context.TODO(), userName)
		if err != nil {
			return err
		}
		if user == nil {
			return fmt.Errorf("user %s not found", userName)
		}
		got := slices.Clone(user.AllowedLoginMode)
		slices.Sort(got)
		want = slices.Clone(want)
		slices.Sort(want)
		if !slices.Equal(got, want) {
			return fmt.Errorf("user %s allowedLoginMode = %v, want %v", userName, got, want)
		}
		return nil
	}
}

func testAccCheckUserDestroy(s *terraform.State) error {
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "coralogix_user" {
			continue
		}

		user, err := testAccFindUser(context.TODO(), rs.Primary.Attributes["user_name"])
		if err != nil {
			return err
		}
		if user != nil && user.GetUserId() == rs.Primary.ID && user.GetStatus() == usersservice.USERSTATUS_USER_STATUS_ACTIVE {
			return fmt.Errorf("user still exists and active: %s", rs.Primary.ID)
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
