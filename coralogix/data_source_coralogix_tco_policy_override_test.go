package coralogix

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

var tcoPolicyOverrideDataSourceName = "data." + tcoPolicyOverrideResourceName

func TestAccCoralogixDataSourceTCOPolicyOverride_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceTCOPolicyOverride() +
					testAccCoralogixResourceTCOPolicyOverride_read(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(tcoPolicyOverrideDataSourceName, "id"),
				),
			},
		},
	})
}

func testAccCoralogixResourceTCOPolicyOverride_read() string {
	return `data "coralogix_tco_policy_override" "test" {
		id = coralogix_tco_policy_override.test.id
}
`
}
