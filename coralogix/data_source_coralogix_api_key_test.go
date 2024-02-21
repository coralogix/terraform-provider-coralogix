package coralogix

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

var teamID = os.Getenv("TEST_TEAM_ID")
var apiKeyDataSourceName = "data." + apiKeyResourceName

func TestAccCoralogixDataSourceApiKey(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testApiKeyResource() +
					testApiKeyResource_read(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(apiKeyDataSourceName, "name", "Test Key 3"),
					resource.TestCheckResourceAttr(apiKeyDataSourceName, "owner.team_id", teamID),
					resource.TestCheckResourceAttr(apiKeyDataSourceName, "active", "true"),
					resource.TestCheckResourceAttr(apiKeyDataSourceName, "hashed", "false"),
				),
			},
		},
	})
}

func testApiKeyResource_read() string {
	return `data "coralogix_api_key" "test" {
		  id = coralogix_api_key.test.id
}
`
}
