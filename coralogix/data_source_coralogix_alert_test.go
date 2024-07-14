package coralogix

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

var alertDataSourceName = "data." + alertResourceName

func TestAccCoralogixDataSourceAlert(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckActionDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertLogsImmediate() +
					testAccCoralogixDataSourceAlert_read(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(alertDataSourceName, "name", "logs immediate alert"),
				),
			},
		},
	})
}
func testAccCoralogixDataSourceAlert_read() string {
	return `data "coralogix_alert" "test" {
	id = coralogix_alert.test.id
}
`
}
