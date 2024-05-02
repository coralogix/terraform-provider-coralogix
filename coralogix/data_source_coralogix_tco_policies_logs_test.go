package coralogix

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

var tcoPolicyDataSourceName = "data." + tcoPoliciesResourceName

func TestAccCoralogixDataSourceTCOPolicy_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceTCOPolicy() +
					testAccCoralogixResourceTCOPolicy_read(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(tcoPolicyDataSourceName, "policies.#", "3"),
				),
			},
		},
	})
}

func testAccCoralogixResourceTCOPolicy_read() string {
	return `data "coralogix_tco_policies_logs" "test" {
}
`
}
