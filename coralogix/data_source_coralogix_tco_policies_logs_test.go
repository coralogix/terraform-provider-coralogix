package coralogix

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

var tcoPoliciesLogsDataSourceName = "data." + tcoPoliciesResourceName

func TestAccCoralogixDataSourceTCOPoliciesLogs_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceTCOPoliciesLogs() +
					testAccCoralogixResourceTCOLogsPolicies_read(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(tcoPoliciesLogsDataSourceName, "policies.0.priority", "low"),
				),
			},
		},
	})
}

func testAccCoralogixResourceTCOLogsPolicies_read() string {
	return fmt.Sprintf(`data "coralogix_tco_policies_logs" "test" {
		depends_on = [%s]
}
`, tcoPoliciesResourceName)
}
