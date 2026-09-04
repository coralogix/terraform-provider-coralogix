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

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

const (
	userMigrationAcceptanceEnv  = "CORALOGIX_USER_MIGRATION_ACC"
	userMigrationProviderSource = "registry.terraform.io/coralogix/coralogix"
	userMigrationSCIMVersion    = "= 3.14.0"
)

// TestAccCoralogixResourceUserMigrationFromSCIM creates the user with the last released
// SCIM-backed provider, then hands the same state to this build. The migration is only
// backwards compatible if the first plan under the new provider is a no-op and every
// later step stays clean.
func TestAccCoralogixResourceUserMigrationFromSCIM(t *testing.T) {
	requireUserMigrationAcceptance(t)

	userName := randUserName()
	initial := testAccCoralogixResourceUser(userName)
	renamed := testAccCoralogixResourceUserNamed(userName, "Migrated", "Person")

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		CheckDestroy: testAccCheckUserDestroy,
		Steps: []resource.TestStep{
			// State written by the SCIM implementation.
			{
				Config:            initial,
				ExternalProviders: userMigrationExternalProvider(userMigrationSCIMVersion),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(userResourceName, "id"),
					resource.TestCheckResourceAttr(userResourceName, "user_name", userName),
					resource.TestCheckResourceAttr(userResourceName, "name.given_name", "Test"),
					resource.TestCheckResourceAttr(userResourceName, "emails.#", "1"),
				),
			},
			// The same configuration under this build must plan as a no-op. Any change to
			// the id, the derived emails or the reconstructed groups shows up here.
			{
				Config:                   initial,
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(userResourceName, plancheck.ResourceActionNoop),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(userResourceName, "user_name", userName),
					resource.TestCheckResourceAttr(userResourceName, "name.given_name", "Test"),
					resource.TestCheckResourceAttr(userResourceName, "name.family_name", "User"),
					resource.TestCheckResourceAttr(userResourceName, "active", "true"),
					resource.TestCheckResourceAttr(userResourceName, "emails.#", "1"),
					resource.TestCheckTypeSetElemNestedAttrs(userResourceName, "emails.*", map[string]string{
						"primary": "true",
						"type":    "work",
						"value":   userName,
					}),
				),
			},
			// A name update on SCIM-created state must stay an in-place update.
			{
				Config:                   renamed,
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(userResourceName, plancheck.ResourceActionUpdate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(userResourceName, "name.given_name", "Migrated"),
					resource.TestCheckResourceAttr(userResourceName, "name.family_name", "Person"),
				),
			},
			// Import by the SCIM UUID must still work.
			{
				ResourceName:             userResourceName,
				ImportState:              true,
				ImportStateVerify:        true,
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			},
		},
	})
}

func requireUserMigrationAcceptance(t *testing.T) {
	t.Helper()
	if os.Getenv(userMigrationAcceptanceEnv) == "" {
		t.Skipf("set %s=1 to run registry-backed user migration tests", userMigrationAcceptanceEnv)
	}
	if namespace := os.Getenv(resource.EnvTfAccProviderNamespace); namespace != "coralogix" {
		t.Fatalf("set %s=coralogix to run registry-backed user migration tests", resource.EnvTfAccProviderNamespace)
	}
}

func userMigrationExternalProvider(version string) map[string]resource.ExternalProvider {
	return map[string]resource.ExternalProvider{
		"coralogix": {
			Source:            userMigrationProviderSource,
			VersionConstraint: version,
		},
	}
}
