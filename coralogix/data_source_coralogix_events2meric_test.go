package coralogix

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

var events2metricDataSourceName = "data." + events2metricResourceName

func TestAccCoralogixDataSourceEvents2Metric_basic(t *testing.T) {
	logsToMetric := getRandomEvents2Metric()
	resource.Test(t, resource.TestCase{
		PreCheck: func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			// newProvider is an example function that returns a provider.Provider
			"coralogix": providerserver.NewProtocol6WithError(NewCoralogixProvider()),
		}, Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceLogs2Metric(logsToMetric) +
					testAccCoralogixDataSourceEvents2Metric_read(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(events2metricDataSourceName, "name", logsToMetric.name),
				),
			},
		},
	})
}

func testAccCoralogixDataSourceEvents2Metric_read() string {
	return `data "coralogix_events2metric" "test" {
	id = coralogix_events2metric.test.id
}
`
}
