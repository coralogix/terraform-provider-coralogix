package coralogix

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAccCoralogixDataSourceDashboard_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceDashboard(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("coralogix_dashboard.test", "name", "dont drop me!"),
				),
			},
			{
				Config: testAccCoralogixResourceDashboard() +
					testAccCoralogixDataSourceDashboard_read(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.coralogix_dashboard.test", "name", "dont drop me!"),
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
