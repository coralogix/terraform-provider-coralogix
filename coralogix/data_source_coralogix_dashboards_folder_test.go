package coralogix

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

var dashboardsFolderDataSourceName = "data." + dashboardsFolderResourceName

func TestAccCoralogixDataSourceDashboardsFolder_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceDashboardsFolder() +
					testAccCoralogixDataSourceDashboardsFolder_read(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dashboardsFolderDataSourceName, "name", "test"),
				),
			},
		},
	})
}

func testAccCoralogixDataSourceDashboardsFolder_read() string {
	return `data "coralogix_dashboards_folder" "test" {
		id = coralogix_dashboard.test.id
	}
`
}
