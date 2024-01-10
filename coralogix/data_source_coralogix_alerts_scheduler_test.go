package coralogix

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

var alertsSchedulerDataSourceName = "data." + alertsSchedulerResourceName

func TestAccCoralogixDataSourceAlertsScheduler(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAlertsSchedulerDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertsScheduler() +
					testAccCoralogixAlertsScheduler_read(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(alertsSchedulerDataSourceName, "name", "example"),
				),
			},
		},
	})
}

func testAccCoralogixAlertsScheduler_read() string {
	return `data "coralogix_alerts_scheduler" "test" {
             id = coralogix_alerts_scheduler.test.id
			}
`
}
