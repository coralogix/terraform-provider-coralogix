package coralogix

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

var sliDataSourceName = "data." + sliResourceName

func TestAccCoralogixDataSourceSLI_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceSLI() +
					testAccCoralogixResourceSLI_read(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(sliDataSourceName, "id"),
				),
			},
		},
	})
}

func testAccCoralogixResourceSLI_read() string {
	return `data "coralogix_sli" "test" {
		id = coralogix_sli.test.id
		service_name = coralogix_sli.test.service_name
}
`
}
