package coralogix

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

var teamDataSourceName = "data." + teamResourceName

func TestAccCoralogixDataSourceTeam_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceTeam() +
					testAccCoralogixDataSourceTeam_read(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(teamDataSourceName, "name", "example"),
				),
			},
		},
	})
}

func testAccCoralogixDataSourceTeam_read() string {
	return `data "coralogix_team" "test" {
		id = coralogix_team.test.id
	}
`
}
