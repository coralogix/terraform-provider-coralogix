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
	"fmt"
	"strings"
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

var (
	apiKeyResourceName = "coralogix_api_key.test"
)

func TestApiKeyResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testApiKeyResource(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(apiKeyResourceName, "name", "Test Key 3"),
					resource.TestCheckResourceAttr(apiKeyResourceName, "owner.team_id", teamID),
					resource.TestCheckResourceAttr(apiKeyResourceName, "active", "true"),
					resource.TestCheckResourceAttr(apiKeyResourceName, "permissions.#", "0"),
					resource.TestCheckTypeSetElemAttr(apiKeyResourceName, "presets.*", "Alerts"),
					resource.TestCheckTypeSetElemAttr(apiKeyResourceName, "presets.*", "APM"),
				),
			},
			{
				ResourceName:            apiKeyResourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"value"},
			},
			{
				Config: updateApiKeyResource(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(apiKeyResourceName, "name", "Test Key 5"),
					resource.TestCheckResourceAttr(apiKeyResourceName, "owner.team_id", teamID),
					resource.TestCheckResourceAttr(apiKeyResourceName, "active", "false"),
					resource.TestCheckResourceAttr(apiKeyResourceName, "permissions.#", "0"),
					resource.TestCheckTypeSetElemAttr(apiKeyResourceName, "presets.*", "Alerts"),
					resource.TestCheckTypeSetElemAttr(apiKeyResourceName, "presets.*", "APM")),
			},
		},
	})
}

func TestApiKeyResourceWithAccessPolicy(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-api-key")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testApiKeyResourceWithAccessPolicy(name, testAccApiKeyAccessPolicy()),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(apiKeyResourceName, "name", name),
					resource.TestCheckResourceAttr(apiKeyResourceName, "owner.team_id", teamID),
					testAccCheckApiKeyAccessPolicy(apiKeyResourceName, testAccApiKeyAccessPolicy()),
				),
			},
			{
				ResourceName:            apiKeyResourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"value", "access_policy"},
				ImportStateCheck: func(states []*terraform.InstanceState) error {
					if len(states) == 0 {
						return fmt.Errorf("no imported state")
					}
					got := states[0].Attributes["access_policy"]
					if !utils.JSONStringsEqual(got, testAccApiKeyAccessPolicy()) {
						return fmt.Errorf("imported access_policy = %q, want JSON equivalent to %q", got, testAccApiKeyAccessPolicy())
					}
					return nil
				},
			},
			{
				Config: testApiKeyResourceWithAccessPolicy(name, testAccApiKeyAccessPolicyUpdated()),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(apiKeyResourceName, "name", name),
					testAccCheckApiKeyAccessPolicy(apiKeyResourceName, testAccApiKeyAccessPolicyUpdated()),
				),
			},
			{
				Config: testApiKeyResourceWithAccessPolicy(name, ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(apiKeyResourceName, "name", name),
					resource.TestCheckResourceAttr(apiKeyResourceName, "access_policy", ""),
				),
			},
		},
	})
}

func testAccCheckApiKeyAccessPolicy(resourceName, expected string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		resourceState, ok := state.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource %s not found", resourceName)
		}
		got := resourceState.Primary.Attributes["access_policy"]
		if !utils.JSONStringsEqual(got, expected) {
			return fmt.Errorf("%s access_policy = %q, want JSON equivalent to %q", resourceName, got, expected)
		}
		return nil
	}
}

func testAccApiKeyAccessPolicy() string {
	return `{ "version": "2025-01-01", "default": { "permissions": { "team-custom-api-keys:ReadConfig": "grant", "team-custom-api-keys:Manage": "grant", "team-custom-api-keys:ReadAccessPolicy": "grant", "team-custom-api-keys:UpdateAccessPolicy": "grant" } }, "rules": [] }`
}

func testAccApiKeyAccessPolicyUpdated() string {
	return `{ "version": "2025-01-01", "default": { "permissions": { "team-custom-api-keys:ReadConfig": "grant", "team-custom-api-keys:Manage": "deny", "team-custom-api-keys:ReadAccessPolicy": "grant", "team-custom-api-keys:UpdateAccessPolicy": "grant" } }, "rules": [] }`
}

func testApiKeyResourceWithAccessPolicy(name, accessPolicy string) string {
	return strings.Replace(fmt.Sprintf(`resource "coralogix_api_key" "test" {
  name  = %q
  owner = {
    team_id : "<TEAM_ID>"
  }
  active = true
  permissions = []
  presets = ["Alerts", "APM"]
  access_policy = %q
}
`, name, accessPolicy), "<TEAM_ID>", teamID, 1)
}

func testApiKeyResource() string {
	return strings.Replace(`resource "coralogix_api_key" "test" {
  name  = "Test Key 3"
  owner = {
    team_id : "<TEAM_ID>"
  }
  active = true
  permissions = []
  presets = ["Alerts", "APM"]
}
`, "<TEAM_ID>", teamID, 1)
}

func updateApiKeyResource() string {
	return strings.Replace(`resource "coralogix_api_key" "test" {
  name  = "Test Key 5"
  owner = {
    team_id : "<TEAM_ID>"
  }
  active = false
  permissions = []
  presets = ["Alerts", "APM"]
}
`, "<TEAM_ID>", teamID, 1)
}
