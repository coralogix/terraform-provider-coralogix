package coralogix

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
					resource.TestCheckResourceAttr(apiKeyResourceName, "hashed", "false"),
					resource.TestCheckResourceAttr(apiKeyResourceName, "permissions.#", "0"),
					resource.TestCheckResourceAttr(apiKeyResourceName, "presets.0", "Alerts"),
					resource.TestCheckResourceAttr(apiKeyResourceName, "presets.1", "APM"),
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
					resource.TestCheckResourceAttr(apiKeyResourceName, "hashed", "false"),
					resource.TestCheckResourceAttr(apiKeyResourceName, "permissions.#", "0"),
					resource.TestCheckResourceAttr(apiKeyResourceName, "presets.0", "Alerts"),
					resource.TestCheckResourceAttr(apiKeyResourceName, "presets.1", "APM"),
				),
			},
		},
	})
}

func testApiKeyResource() string {
	return strings.Replace(`resource "coralogix_api_key" "test" {
  name  = "Test Key 3"
  owner = {
    team_id : "<TEAM_ID>"
  }
  active = true
  hashed = false
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
  hashed = false
  permissions = []
  presets = ["Alerts", "APM"]
}
`, "<TEAM_ID>", teamID, 1)
}
