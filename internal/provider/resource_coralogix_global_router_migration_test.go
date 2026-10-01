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
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

const (
	globalRouterMigrationAcceptanceEnv  = "CORALOGIX_GLOBAL_ROUTER_MIGRATION_ACC"
	globalRouterMigrationProviderSource = "registry.terraform.io/coralogix/coralogix"
	// The release before the generated resource. The router in it is handwritten.
	globalRouterMigrationReleasedVersion = "= 3.19.0"
)

// TestAccCoralogixResourceGlobalRouterMigration creates a router with the released provider, then
// plans the same config with this build. A user who upgrades must see no change, and a later
// update must work. Every config sets `description` and the entity types, because the released
// provider cannot create a router without them.
func TestAccCoralogixResourceGlobalRouterMigration(t *testing.T) {
	requireGlobalRouterMigrationAcceptance(t)

	cases := []struct {
		name    string
		initial string // attributes of the router, in HCL
		updated string
	}{
		{
			name: "all-attributes",
			initial: `
      description   = "migration"
      disabled      = true
      entity_labels = { team = "a" }
      routing_labels = { environment = "%[1]v", service = "api" }
      rules = [
        {
          name           = "first"
          condition      = "alertDef.priority == \"P1\""
          entity_type    = "alerts"
          custom_details = { k = "v" }
          targets = [
            { connector_id = coralogix_connector.generic_https_example.id, custom_details = { x = "y" } },
          ]
        },
        {
          name        = "second"
          condition   = "true"
          entity_type = "cases"
          targets     = [{ connector_id = coralogix_connector.generic_https_example.id }]
        },
      ]
      fallback_targets = [
        { entity_type = "alerts", target = { connector_id = coralogix_connector.generic_https_example.id } },
      ]`,
			updated: `
      description   = "migration updated"
      disabled      = false
      entity_labels = { team = "b" }
      routing_labels = { environment = "%[1]v", service = "api" }
      rules = [
        {
          name        = "first"
          condition   = "alertDef.priority == \"P2\""
          entity_type = "alerts"
          targets     = [{ connector_id = coralogix_connector.generic_https_example.id }]
        },
      ]
      fallback_targets = [
        { entity_type = "alerts", target = { connector_id = coralogix_connector.generic_https_example.id } },
      ]`,
		},
		{
			name: "deprecated-fallback",
			initial: `
      description    = "migration"
      routing_labels = { environment = "%[1]v" }
      rules = [
        {
          name        = "first"
          condition   = "true"
          entity_type = "alerts"
          targets     = [{ connector_id = coralogix_connector.generic_https_example.id }]
        },
      ]
      fallback = [{ connector_id = coralogix_connector.generic_https_example.id }]`,
			updated: `
      description    = "migration updated"
      routing_labels = { environment = "%[1]v" }
      rules = [
        {
          name        = "first"
          condition   = "true"
          entity_type = "alerts"
          targets     = [{ connector_id = coralogix_connector.generic_https_example.id }]
        },
      ]
      fallback = [{ connector_id = coralogix_connector.generic_https_example.id }]`,
		},
		{
			name: "minimal",
			initial: `
      description    = "migration"
      routing_labels = { environment = "%[1]v" }`,
			updated: `
      description    = "migration updated"
      routing_labels = { environment = "%[1]v" }
      entity_labels  = { a = "b" }`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			name := uuid.NewString()
			initial := globalRouterMigrationConfig(name, tc.initial)
			updated := globalRouterMigrationConfig(name, tc.updated)

			resource.ParallelTest(t, resource.TestCase{
				PreCheck:     func() { testAccPreCheck(t) },
				CheckDestroy: testAccCheckGlobalRouterDestroy,
				Steps: []resource.TestStep{
					{
						Config: initial,
						ExternalProviders: map[string]resource.ExternalProvider{
							"coralogix": {Source: globalRouterMigrationProviderSource, VersionConstraint: globalRouterMigrationReleasedVersion},
						},
						Check: resource.TestCheckResourceAttr(globalRouterResourceName, "name", name),
					},
					{
						// This build reads the state of the released provider and plans no change.
						Config:                   initial,
						ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
						ConfigPlanChecks: resource.ConfigPlanChecks{
							PreApply:             []plancheck.PlanCheck{plancheck.ExpectResourceAction(globalRouterResourceName, plancheck.ResourceActionNoop)},
							PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
						},
						Check: resource.ComposeAggregateTestCheckFunc(
							resource.TestCheckResourceAttr(globalRouterResourceName, "name", name),
							resource.TestCheckResourceAttrSet(globalRouterResourceName, "create_time"),
						),
					},
					{
						Config:                   updated,
						ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
						ConfigPlanChecks: resource.ConfigPlanChecks{
							PreApply:             []plancheck.PlanCheck{plancheck.ExpectResourceAction(globalRouterResourceName, plancheck.ResourceActionUpdate)},
							PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
						},
					},
					{
						ResourceName:             globalRouterResourceName,
						ImportState:              true,
						ImportStateVerify:        true,
						ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
					},
				},
			})
		})
	}
}

func requireGlobalRouterMigrationAcceptance(t *testing.T) {
	t.Helper()
	if os.Getenv(globalRouterMigrationAcceptanceEnv) == "" {
		t.Skipf("set %s=1 to run registry-backed global router migration tests", globalRouterMigrationAcceptanceEnv)
	}
	if namespace := os.Getenv(resource.EnvTfAccProviderNamespace); namespace != "coralogix" {
		t.Fatalf("set %s=coralogix to run registry-backed global router migration tests", resource.EnvTfAccProviderNamespace)
	}
}

// globalRouterMigrationConfig is a generic HTTPS connector and a router. attributes is the HCL of the
// router, and %[1]v in it is the name.
func globalRouterMigrationConfig(name, attributes string) string {
	return fmt.Sprintf(`
    resource "coralogix_connector" "generic_https_example" {
      id               = "http-%[1]v"
      name             = "http-%[1]v"
      type             = "generic_https"
      description      = "generic-https connector example"
      connector_config = {
        fields = [
          {
            field_name = "url"
            value      = "https://api.staging.coralogix.net/mgmt/testing/tools/httpbin/post"
          },
          {
            field_name = "method"
            value      = "post"
          }
        ]
      }
    }

    resource "coralogix_global_router" "example" {
      name = "%[1]v"
%[2]s
    }
  `, name, fmt.Sprintf(attributes, name))
}

func testAccCheckGlobalRouterDestroy(s *terraform.State) error {
	clients, err := testAccNewClientSet()
	if err != nil {
		return err
	}
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "coralogix_global_router" {
			continue
		}
		_, httpResp, err := clients.GlobalRouters().GlobalRoutersServiceGetGlobalRouter(context.TODO(), rs.Primary.ID).Execute()
		if err == nil {
			return fmt.Errorf("global router %s still exists", rs.Primary.ID)
		}
		if httpResp == nil || httpResp.StatusCode != http.StatusNotFound {
			return fmt.Errorf("read global router %s: %w", rs.Primary.ID, err)
		}
	}
	return nil
}
