package coralogix

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

var (
	archiveLogsResourceName = "coralogix_archive_logs.test"
)

func TestAccCoralogixResourceResourceArchiveLogs(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceArchiveLogs(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(archiveLogsResourceName, "bucket", "coralogix-c4c-eu2-prometheus-data"),
					resource.TestCheckResourceAttr(archiveLogsResourceName, "active", "true"),
				),
			},
			{
				ResourceName:      archiveLogsResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccCoralogixResourceArchiveLogsUpdate(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(archiveLogsResourceName, "bucket", "coralogix-c4c-eu2-prometheus-data"),
					resource.TestCheckResourceAttr(archiveLogsResourceName, "active", "false"),
				),
			},
		},
	})
}

func testAccCoralogixResourceArchiveLogs() string {
	return `resource "coralogix_archive_logs" "test" {
 	bucket = "coralogix-c4c-eu2-prometheus-data"
}
`
}

func testAccCoralogixResourceArchiveLogsUpdate() string {
	return `resource "coralogix_archive_logs" "test" {
  		bucket = coralogix-c4c-eu2-prometheus-data
 		active = false
}
`
}
