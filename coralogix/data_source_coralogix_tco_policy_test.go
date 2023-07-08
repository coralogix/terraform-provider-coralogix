package coralogix

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

var tcoPolicyDataSourceName = "data." + tcoPolicyResourceName1

func TestAccCoralogixDataSourceTCOPolicy_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceTCOPolicy() +
					testAccCoralogixResourceTCOPolicy_read(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(tcoPolicyDataSourceName, "id"),
				),
			},
		},
	})
}

func testAccCoralogixResourceTCOPolicy_read() string {
	return `data "coralogix_tco_policy" "test_1" {
		id = coralogix_tco_policy.test_1.id
}
`
}
