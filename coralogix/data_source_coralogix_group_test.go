package coralogix

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

var groupDataSourceName = "data." + groupResourceName

func TestAccCoralogixDataSourceGroup_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceGroup() +
					testAccCoralogixDataSourceGroup_read(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(groupDataSourceName, "display_name", "example"),
				),
			},
		},
	})
}

func testAccCoralogixDataSourceGroup_read() string {
	return `data "coralogix_events2metric" "test" {
	id = coralogix_group.test.id
}
`
}
