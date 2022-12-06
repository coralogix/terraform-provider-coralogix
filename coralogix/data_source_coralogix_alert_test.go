package coralogix

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAccCoralogixDataSourceAlert_basic(t *testing.T) {
	alert := standardAlertTestParams{
		alertCommonTestParams: *getRandomAlert(),
		groupBy:               []string{"EventType"},
		occurrencesThreshold:  acctest.RandIntRange(1, 1000),
		timeWindow:            selectRandomlyFromSlice(alertValidTimeFrames),
		deadmanRatio:          selectRandomlyFromSlice(alertValidUndetectedValuesAutoRetireRatios),
	}

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertStandard(&alert),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("coralogix_alert.test", "name", alert.name),
				),
			},
			{
				Config: testAccCoralogixResourceAlertStandard(&alert) +
					testAccCoralogixDataSourceAlert_read(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.coralogix_alert.test", "name", alert.name),
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
