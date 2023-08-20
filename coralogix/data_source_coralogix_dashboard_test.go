package coralogix

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

var dashboardDataSourceName = "data." + dashboardResourceName

func TestAccCoralogixDataSourceDashboard_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceDashboard() +
					testAccCoralogixDataSourceDashboard_read(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dashboardDataSourceName, "name", "dont drop me!"),
				),
			},
		},
	})
}

func testAccCoralogixDataSourceDashboard_read() string {
	return `data "coralogix_dashboard" "test" {
	id = coralogix_dashboard.test.id
}
`
}
