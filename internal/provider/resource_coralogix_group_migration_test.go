// Copyright 2026 Coralogix Ltd.
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
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

const (
	groupMigrationAcceptanceEnv  = "CORALOGIX_GROUP_MIGRATION_ACC"
	groupMigrationProviderSource = "registry.terraform.io/coralogix/coralogix"
	groupMigrationSCIMVersion    = "= 3.13.0"
)

func TestAccCoralogixResourceGroupMigrationFromSCIM(t *testing.T) {
	requireGroupMigrationAcceptance(t)

	userName := randUserName()
	userName2 := randUserName()
	displayName := acctest.RandomWithPrefix("tf-acc-test-group")
	updatedName := displayName + "-openapi"
	scopeName := acctest.RandomWithPrefix("tf-acc-test-scope")
	initial := testAccCoralogixResourceGroupWithRole(userName, displayName, scopeName, "Read Only")
	updated := testAccCoralogixResourceGroupUpdatedMembers(userName, userName2, updatedName, scopeName)

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		CheckDestroy: testAccCheckGroupDestroy,
		Steps: []resource.TestStep{
			{
				Config:            initial,
				ExternalProviders: groupMigrationExternalProvider(groupMigrationSCIMVersion),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(groupResourceName, "display_name", displayName),
					resource.TestCheckResourceAttr(groupResourceName, "role", "Read Only"),
					resource.TestCheckResourceAttr(groupResourceName, "members.#", "1"),
					testAccCheckGroupRoleAssigned(groupResourceName, "Read Only"),
				),
			},
			{
				Config:                   initial,
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(groupResourceName, plancheck.ResourceActionNoop),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(groupResourceName, "role", "Read Only"),
					testAccCheckGroupRoleAssigned(groupResourceName, "Read Only"),
				),
			},
			{
				Config:                   updated,
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(groupResourceName, plancheck.ResourceActionUpdate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(groupResourceName, "display_name", updatedName),
					resource.TestCheckResourceAttr(groupResourceName, "role", "Read Only"),
					resource.TestCheckResourceAttr(groupResourceName, "members.#", "2"),
					testAccCheckGroupRoleAssigned(groupResourceName, "Read Only"),
				),
			},
			{
				ResourceName:             groupResourceName,
				ImportState:              true,
				ImportStateVerify:        true,
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			},
		},
	})
}

func TestAccCoralogixResourceGroupMigrationAttachmentFromSCIM(t *testing.T) {
	requireGroupMigrationAcceptance(t)

	firstUserName := randUserName()
	secondUserName := randUserName()
	displayName := acctest.RandomWithPrefix("tf-acc-test-group")
	updatedName := displayName + "-openapi"
	scopeName := acctest.RandomWithPrefix("tf-acc-test-scope")
	initial := testAccCoralogixResourceGroupUnmanagedMembers(firstUserName, secondUserName, displayName, scopeName)
	updated := testAccCoralogixResourceGroupUnmanagedMembers(firstUserName, secondUserName, updatedName, scopeName)

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		CheckDestroy: testAccCheckGroupDestroy,
		Steps: []resource.TestStep{
			{
				Config:            initial,
				ExternalProviders: groupMigrationExternalProvider(groupMigrationSCIMVersion),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(groupUnmanagedMembersResourceName, "display_name", displayName),
					resource.TestCheckResourceAttr(groupUnmanagedMembersResourceName, "role", "Read Only"),
					testAccCheckGroupMemberCount(groupUnmanagedMembersResourceName, 2),
					testAccCheckGroupRoleAssigned(groupUnmanagedMembersResourceName, "Read Only"),
				),
			},
			{
				Config:                   initial,
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(groupUnmanagedMembersResourceName, plancheck.ResourceActionNoop),
						plancheck.ExpectResourceAction("coralogix_group_attachment.unmanaged_members", plancheck.ResourceActionNoop),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(groupUnmanagedMembersResourceName, "role", "Read Only"),
					testAccCheckGroupMemberCount(groupUnmanagedMembersResourceName, 2),
				),
			},
			{
				Config:                   updated,
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(groupUnmanagedMembersResourceName, plancheck.ResourceActionUpdate),
						plancheck.ExpectResourceAction("coralogix_group_attachment.unmanaged_members", plancheck.ResourceActionNoop),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(groupUnmanagedMembersResourceName, "display_name", updatedName),
					resource.TestCheckResourceAttr(groupUnmanagedMembersResourceName, "role", "Read Only"),
					testAccCheckGroupMemberCount(groupUnmanagedMembersResourceName, 2),
				),
			},
		},
	})
}

func requireGroupMigrationAcceptance(t *testing.T) {
	t.Helper()
	if os.Getenv(groupMigrationAcceptanceEnv) == "" {
		t.Skipf("set %s=1 to run registry-backed group migration tests", groupMigrationAcceptanceEnv)
	}
	if namespace := os.Getenv(resource.EnvTfAccProviderNamespace); namespace != "coralogix" {
		t.Fatalf("set %s=coralogix to run registry-backed group migration tests", resource.EnvTfAccProviderNamespace)
	}
}

func groupMigrationExternalProvider(version string) map[string]resource.ExternalProvider {
	return map[string]resource.ExternalProvider{
		"coralogix": {
			Source:            groupMigrationProviderSource,
			VersionConstraint: version,
		},
	}
}
