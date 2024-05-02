package coralogix

import (
	"fmt"
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
					resource.TestCheckResourceAttr(tcoPoliciesTracesDataSourceName, "policies.0.priority", "low"),
				),
			},
		},
	})
}

func testAccCoralogixResourceTCOPoliciesTraces_read() string {
	return fmt.Sprintf(`data "coralogix_tco_policies_traces" "test" {
		depends_on = [%s]
}
`, tcoPoliciesTracesResourceName)
}
