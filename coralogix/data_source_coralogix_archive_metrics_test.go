package coralogix

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

var archiveMetricsDataSourceName = "data." + archiveMetricsResourceName

func TestAccCoralogixDataSourceArchiveMetrics_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceArchiveMetrics() +
					testAccCoralogixDataSourceArchiveMetrics_read(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(archiveMetricsDataSourceName, "s3.region", "eu-north-1"),
					resource.TestCheckResourceAttr(archiveMetricsDataSourceName, "s3.bucket", "coralogix-c4c-eu2-prometheus-data"),
				),
			},
		},
	})
}

func testAccCoralogixDataSourceArchiveMetrics_read() string {
	return `data "coralogix_archive_metrics" "test" {
}
`
}
