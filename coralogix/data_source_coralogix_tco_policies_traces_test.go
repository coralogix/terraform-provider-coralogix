package coralogix

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

var tcoPoliciesTracesDataSourceName = "data." + tcoPoliciesTracesResourceName

func TestAccCoralogixDataSourceTCOPoliciesTraces_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceTCOPoliciesTraces() +
					testAccCoralogixResourceTCOPoliciesTraces_read(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(tcoPoliciesTracesDataSourceName, "policies.#", "3"),
				),
			},
		},
	})
}

func testAccCoralogixResourceTCOPoliciesTraces_read() string {
	return `data "coralogix_tco_policies_traces" "test" {
}
`
}
