package coralogix

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

var logs2metricDataSourceName = "data." + logs2metricResourceName

func TestAccCoralogixDataSourceLogs2Metric_basic(t *testing.T) {
	logsToMetric := getRandomLogs2Metric()
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceLogs2Metric(logsToMetric) +
					testAccCoralogixDataSourceLogs2Metric_read(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(logs2metricDataSourceName, "name", logsToMetric.name),
				),
			},
		},
	})
}

func testAccCoralogixDataSourceLogs2Metric_read() string {
	return `data "coralogix_logs2metric" "test" {
	id = coralogix_logs2metric.test.id
}
`
}
