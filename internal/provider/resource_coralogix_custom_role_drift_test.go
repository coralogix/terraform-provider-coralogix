// Copyright 2026 Coralogix Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
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
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	roless "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/role_management_service"
)

const (
	customRoleDriftDescription     = "Created by the permissions drift acceptance test"
	customRoleDriftPermission      = "alerts:MetricsUpdateConfig"
	customRoleDriftExtraPermission = "alerts:UpdateConfig"
)

// TestAccCoralogixResourceCustomRolePermissionsDrift covers the permission set
// crossing the resource boundary, at the level Terraform runs it rather than
// through flattenCustomRole directly.
//
// Step 2 grants an extra permission through the API, behind Terraform's back.
// Refresh must accept that and record it, so the plan reports drift. An earlier
// version of the resource failed the refresh outright, because it required the
// API's set to match the set already in state.
//
// Step 3 reapplies the original configuration and asserts an empty plan
// afterwards, which is what proves the resource converges instead of settling
// into a perpetual diff.
//
// What this cannot cover: Create and Update deliberately keep the planned set,
// since Terraform rejects a post-apply value that differs from the plan. That
// only misbehaves if the backend stores something other than what the write
// sent, which no test can force through the real API.
func TestAccCoralogixResourceCustomRolePermissionsDrift(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-custom-role-drift")
	var roleID string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// 1. Apply the configured permission set and record the role id for
			//    the out-of-band call in step 2.
			{
				Config: testAccCustomRoleDriftConfig(name, customRoleDriftPermission),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(customRoleResourceName, "permissions.#", "1"),
					resource.TestCheckTypeSetElemAttr(customRoleResourceName, "permissions.*", customRoleDriftPermission),
					testAccCaptureCustomRoleID(&roleID),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			// 2. Grant an extra permission through the API, then refresh only.
			//    State must take the API's set, leaving a non-empty plan.
			{
				PreConfig: func() {
					testAccSetCustomRolePermissions(t, &roleID, name, []string{
						customRoleDriftPermission,
						customRoleDriftExtraPermission,
					})
				},
				RefreshState: true,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(customRoleResourceName, "permissions.#", "2"),
					resource.TestCheckTypeSetElemAttr(customRoleResourceName, "permissions.*", customRoleDriftPermission),
					resource.TestCheckTypeSetElemAttr(customRoleResourceName, "permissions.*", customRoleDriftExtraPermission),
				),
				ExpectNonEmptyPlan: true,
			},
			// 3. The unchanged configuration removes the extra permission again,
			//    and the follow-up plan is empty.
			{
				Config: testAccCustomRoleDriftConfig(name, customRoleDriftPermission),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(customRoleResourceName, "permissions.#", "1"),
					resource.TestCheckTypeSetElemAttr(customRoleResourceName, "permissions.*", customRoleDriftPermission),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

// testAccCaptureCustomRoleID records the role id from state so a later step can
// address the role through the API.
func testAccCaptureCustomRoleID(target *string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		resourceState, ok := state.RootModule().Resources[customRoleResourceName]
		if !ok || resourceState.Primary == nil {
			return fmt.Errorf("resource %s not found in state", customRoleResourceName)
		}

		id := resourceState.Primary.Attributes["id"]
		if id == "" {
			id = resourceState.Primary.ID
		}
		if id == "" {
			return fmt.Errorf("resource %s has no id in state", customRoleResourceName)
		}
		*target = id

		return nil
	}
}

// testAccSetCustomRolePermissions replaces the role's permissions through the
// API, simulating an edit made outside Terraform. Name and description are sent
// unchanged so the request is complete and only permissions drift.
func testAccSetCustomRolePermissions(t *testing.T, roleID *string, name string, permissions []string) {
	t.Helper()

	if *roleID == "" {
		t.Fatal("custom role id was not captured from state")
	}
	id, err := strconv.ParseInt(*roleID, 10, 64)
	if err != nil {
		t.Fatalf("parse custom role id %q: %s", *roleID, err)
	}

	clients, err := testAccNewClientSet()
	if err != nil {
		t.Fatalf("build acceptance client: %s", err)
	}

	description := customRoleDriftDescription
	if _, _, err = clients.CustomRoles().
		RoleManagementServiceUpdateRole(context.Background(), id).
		RoleManagementServiceUpdateRoleRequest(roless.RoleManagementServiceUpdateRoleRequest{
			NewName:        &name,
			NewDescription: &description,
			NewPermissions: &roless.V2Permissions{Permissions: permissions},
		}).
		Execute(); err != nil {
		t.Fatalf("set permissions %v on custom role %d out of band: %s", permissions, id, err)
	}
}

func testAccCustomRoleDriftConfig(name string, permissions ...string) string {
	quoted := make([]string, 0, len(permissions))
	for _, permission := range permissions {
		quoted = append(quoted, strconv.Quote(permission))
	}

	return fmt.Sprintf(`resource "coralogix_custom_role" "test" {
  name        = %q
  description = %q
  parent_role = "Read-Only User"
  permissions = [%s]
}
`, name, customRoleDriftDescription, strings.Join(quoted, ", "))
}
