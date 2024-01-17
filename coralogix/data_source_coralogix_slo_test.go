package coralogix

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

var sloDataSourceName = "data." + sloResourceName

func TestAccCoralogixDataSourceSLO_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceSLO() +
					testAccCoralogixResourceSLO_read(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(sloDataSourceName, "id"),
				),
			},
		},
	})
}

func testAccCoralogixResourceSLO_read() string {
	return `data "coralogix_slo" "test" {
		id = coralogix_slo.test.id
}
`
}
