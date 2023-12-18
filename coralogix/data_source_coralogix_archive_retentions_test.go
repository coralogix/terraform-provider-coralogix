package coralogix

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

var archiveRetentionsDataSourceName = "data." + archiveRetentionsResourceName

func TestAccCoralogixDataSourceArchiveRetentions_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceArchiveRetentions() +
					testAccCoralogixDataSourceArchiveRetentions_read(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(archiveRetentionsDataSourceName, "retentions.#", "4"),
				),
			},
		},
	})
}

func testAccCoralogixDataSourceArchiveRetentions_read() string {
	return `data "coralogix_archive_retentions" "test" {
}
`
}
