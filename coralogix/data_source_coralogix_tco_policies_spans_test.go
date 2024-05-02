package coralogix

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

var tcoPoliciesTracesDataSourceName = "data." + tcoPoliciesTracesResourceName

func TestAccCoralogixDataSourceTCOPolicyTraces_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceTCOPoliciesTraces() +
					testAccCoralogixResourceTCOPolicyTraces_read(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(tcoPoliciesTracesDataSourceName, "policies.#", "3"),
				),
			},
		},
	})
}

func testAccCoralogixResourceTCOPolicyTraces_read() string {
	return `data "coralogix_tco_policy_traces" "test_1" {
		id = coralogix_tco_policy_traces.test_1.id
}
`
}
