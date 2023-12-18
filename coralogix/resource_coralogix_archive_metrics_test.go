package coralogix

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

var (
	archiveMetricsResourceName = "coralogix_archive_metrics.test"
)

func TestAccCoralogixResourceResourceArchiveMetrics(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceArchiveMetrics(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(archiveMetricsResourceName, "s3.region", "eu-north-1"),
					resource.TestCheckResourceAttr(archiveMetricsResourceName, "s3.bucket", "coralogix-c4c-eu2-prometheus-data"),
				),
			},
			{
				ResourceName:      archiveMetricsResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccCoralogixResourceArchiveMetrics() string {
	return `resource "coralogix_archive_metrics" "test" {
  s3 = {
    region = "eu-north-1"
    bucket = "coralogix-c4c-eu2-prometheus-data"
  }
}
`
}
