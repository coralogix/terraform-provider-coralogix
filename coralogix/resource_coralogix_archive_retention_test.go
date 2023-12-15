package coralogix

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

var (
	archiveRetentionsResourceName = "coralogix_archive_retentions.test"
)

func TestAccCoralogixResourceResourceArchiveRetentions(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceArchiveRetentions(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(archiveRetentionsResourceName, "retentions.0.name", "Default"),
					resource.TestCheckResourceAttr(archiveRetentionsResourceName, "retentions.1.name", "name_2"),
					resource.TestCheckResourceAttr(archiveRetentionsResourceName, "retentions.2.name", "name_3"),
					resource.TestCheckResourceAttr(archiveRetentionsResourceName, "retentions.3.name", "name_4"),
				),
			},
			{
				ResourceName:      archiveRetentionsResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccCoralogixResourceArchiveRetentionsUpdate(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(archiveRetentionsResourceName, "retentions.0.name", "Default"),
					resource.TestCheckResourceAttr(archiveRetentionsResourceName, "retentions.1.name", "new_name_2"),
					resource.TestCheckResourceAttr(archiveRetentionsResourceName, "retentions.2.name", "new_name_3"),
					resource.TestCheckResourceAttr(archiveRetentionsResourceName, "retentions.3.name", "new_name_4"),
				),
			},
		},
	})
}

func testAccCoralogixResourceArchiveRetentions() string {
	return `resource "coralogix_archive_retentions" "test" {
	retentions = [
		{
		},
		{
			name = "name_2"
		},
		{
			name = "name_3"
		},
		{
			name = "name_4"
		},
	]
}
`
}

func testAccCoralogixResourceArchiveRetentionsUpdate() string {
	return `resource "coralogix_archive_retentions" "test" {
	retentions = [
		{
		},
		{
			name = "new_name_2"
		},
		{
			name = "new_name_3"
		},
		{
			name = "new_name_4"
		},
	]
}
`
}
