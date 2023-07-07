package coralogix

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

var hostedDashboardDataSourceName = "data." + hostedDashboardResourceName

func TestAccCoralogixDataSourceGrafanaDashboard_basic(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	parent := filepath.Dir(wd)
	filePath := parent + "/examples/hosted_dashboard/grafana_acc_dashboard.json"

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceGrafanaDashboard(filePath) +
					testAccCoralogixDataSourceGrafanaDashboard_read(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(hostedDashboardDataSourceName, "uid"),
				),
			},
		},
	})
}

func testAccCoralogixDataSourceGrafanaDashboard_read() string {
	return `data "coralogix_hosted_dashboard" "test" {
		uid = coralogix_hosted_dashboard.test.id
}
`
}
