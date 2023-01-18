package coralogix

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"terraform-provider-coralogix/coralogix/clientset"
	logs2metricv2 "terraform-provider-coralogix/coralogix/clientset/grpc/logs2metrics/v2"

	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

type logs2MetricTestFields struct {
	name, description string
	limit             int
}

var logs2metricResourceName = "coralogix_logs2metric.test"

func TestAccCoralogixResourceLogs2Metric(t *testing.T) {
	logs2Metric := getRandomLogs2Metric()
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckLogs2MetricDestroy,
		Steps: []resource.TestStep{
			{

				Config: testAccCoralogixResourceLogs2Metric(logs2Metric),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(logs2metricResourceName, "id"),
					resource.TestCheckResourceAttr(logs2metricResourceName, "name", logs2Metric.name),
					resource.TestCheckResourceAttr(logs2metricResourceName, "description", logs2Metric.description),
					resource.TestCheckResourceAttr(logs2metricResourceName, "query.0.lucene", "remote_addr_enriched:/.*/"),
					resource.TestCheckResourceAttr(logs2metricResourceName, "query.0.applications.0", "nginx"),
					resource.TestCheckResourceAttr(logs2metricResourceName, "query.0.severities.0", "Debug"),
					resource.TestCheckTypeSetElemNestedAttrs(logs2metricResourceName, "metric_fields.*",
						map[string]string{
							"source_field":            "remote_addr_geoip.location_geopoint",
							"target_base_metric_name": "geo_point",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(logs2metricResourceName, "metric_fields.*",
						map[string]string{
							"source_field":            "method",
							"target_base_metric_name": "method",
						},
					),

					resource.TestCheckTypeSetElemNestedAttrs(logs2metricResourceName, "metric_labels.*",
						map[string]string{
							"source_field": "status",
							"target_label": "Status",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(logs2metricResourceName, "metric_labels.*",
						map[string]string{
							"source_field": "http_referer",
							"target_label": "Path",
						},
					),
					resource.TestCheckResourceAttr(logs2metricResourceName, "permutations.0.limit", strconv.Itoa(logs2Metric.limit)),
					resource.TestCheckResourceAttr(logs2metricResourceName, "permutations.0.has_exceed_limit", "false"),
				),
			},
			{
				ResourceName:      logs2metricResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccCheckLogs2MetricDestroy(s *terraform.State) error {
	client := testAccProvider.Meta().(*clientset.ClientSet).Logs2Metrics()

	ctx := context.TODO()

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "coralogix_logs2metric" {
			continue
		}

		req := &logs2metricv2.GetL2MRequest{
			Id: wrapperspb.String(rs.Primary.ID),
		}

		resp, err := client.GetLogs2Metric(ctx, req)
		if err == nil {
			if resp.GetId().GetValue() == rs.Primary.ID {
				return fmt.Errorf("logs2metric still exists: %s", rs.Primary.ID)
			}
		}
	}

	return nil
}

func getRandomLogs2Metric() *logs2MetricTestFields {
	return &logs2MetricTestFields{
		name:        acctest.RandStringFromCharSet(10, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ012346789_:"),
		description: acctest.RandomWithPrefix("tf-acc-test"),
		limit:       acctest.RandIntRange(0, 500000),
	}

}

func testAccCoralogixResourceLogs2Metric(l *logs2MetricTestFields) string {
	return fmt.Sprintf(`resource "coralogix_logs2metric" "test" {
  name        = "%s"
  description = "%s"
  query {
    lucene       = "remote_addr_enriched:/.*/"
    applications = ["nginx"]
    severities   = ["Debug"]
  }

  metric_fields {
    target_base_metric_name = "method"
    source_field            = "method"
  }
  metric_fields {
    target_base_metric_name = "geo_point"
    source_field            = "remote_addr_geoip.location_geopoint"
  }

  metric_labels {
    target_label = "Status"
    source_field = "status"
  }
  metric_labels {
    target_label = "Path"
    source_field = "http_referer"
  }

  permutations {
    limit = %d
  }
}
`,
		l.name, l.description, l.limit)
}
