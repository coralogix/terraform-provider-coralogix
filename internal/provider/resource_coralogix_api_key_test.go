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
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
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
				ResourceName:      apiKeyResourceName,
				ImportState:       true,
				ImportStateVerify: true,
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
	t.Skip("Different Permissions Required")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testApiKeyResourceWithAccessPolicy(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(apiKeyResourceName, "name", "Test Key 3"),
					resource.TestCheckResourceAttr(apiKeyResourceName, "owner.team_id", teamID),
					resource.TestCheckResourceAttr(apiKeyResourceName, "active", "true"),
					resource.TestCheckResourceAttr(apiKeyResourceName, "access_policy", "{ \"version\": \"2025-01-01\", \"default\": { \"permissions\": { \"data-ingest-api-keys:ReadAccessPolicy\": \"grant\", \"data-ingest-api-keys:Manage\": \"deny\", \"data-ingest-api-keys:UpdateAccessPolicy\": \"deny\", \"data-ingest-api-keys:ReadConfig\": \"grant\" } }, \"rules\": [] }"),
					resource.TestCheckResourceAttr(apiKeyResourceName, "permissions.#", "0"),
					resource.TestCheckTypeSetElemAttr(apiKeyResourceName, "presets.*", "Alerts"),
					resource.TestCheckTypeSetElemAttr(apiKeyResourceName, "presets.*", "APM"),
				),
			},
			{
				ResourceName:      apiKeyResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: updateApiKeyResource(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(apiKeyResourceName, "name", "Test Key 5"),
					resource.TestCheckResourceAttr(apiKeyResourceName, "owner.team_id", teamID),
					resource.TestCheckResourceAttr(apiKeyResourceName, "active", "false"),
					resource.TestCheckResourceAttr(apiKeyResourceName, "access_policy", ""),
					resource.TestCheckResourceAttr(apiKeyResourceName, "permissions.#", "0"),
					resource.TestCheckTypeSetElemAttr(apiKeyResourceName, "presets.*", "Alerts"),
					resource.TestCheckTypeSetElemAttr(apiKeyResourceName, "presets.*", "APM")),
			},
		},
	})
}

func testApiKeyResourceWithAccessPolicy() string {
	return strings.Replace(`resource "coralogix_api_key" "test" {
  name  = "Test Key 3"
  owner = {
    team_id : "<TEAM_ID>"
  }
  active = true
  permissions = []
  presets = ["Alerts", "APM"]
  access_policy = "{ \"version\": \"v2025-01-01\", \"rules\": [], \"default\": { \"permissions\": { \"team-custom-api-keys:ReadConfig\": \"grant\", \"team-custom-api-keys:Manage\": \"grant\", \"team-custom-api-keys:ReadAccessPolicy\": \"grant\", \"team-custom-api-keys:UpdateAccessPolicy\": \"grant\" }, \"additionalInfo\": null } }"
}
`, "<TEAM_ID>", teamID, 1)
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
