package coralogix

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"terraform-provider-coralogix-v2/coralogix/clientset"
	logs2metricv2 "terraform-provider-coralogix-v2/coralogix/clientset/grpc/com/coralogix/logs2metrics/v2"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

type logs2MetricTestFields struct {
	name, description string
	limit             int
}

func TestAccCoralogixResourceLogs2Metric(t *testing.T) {
	resourceName := "coralogix_logs2metric.test"
	logs2Metric := getRandomLogs2Metric()
	testAccCoralogixResourceLogs2Metric(logs2Metric)
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckLogs2MetricDestroy,
		Steps: []resource.TestStep{
			{

				Config: testAccCoralogixResourceLogs2Metric(logs2Metric),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "name", logs2Metric.name),
					resource.TestCheckResourceAttr(resourceName, "description", logs2Metric.description),
					resource.TestCheckResourceAttr(resourceName, "query.0.lucene", "remote_addr_enriched:/.*/"),
					resource.TestCheckResourceAttr(resourceName, "query.0.applications.0", "nginx"),
					resource.TestCheckResourceAttr(resourceName, "query.0.severities.0", "Debug"),
					resource.TestCheckTypeSetElemNestedAttrs(resourceName, "metric_fields.*",
						map[string]string{
							"source_field":            "remote_addr_geoip.location_geopoint",
							"target_base_metric_name": "geo_point",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(resourceName, "metric_fields.*",
						map[string]string{
							"source_field":            "method",
							"target_base_metric_name": "method",
						},
					),

					resource.TestCheckTypeSetElemNestedAttrs(resourceName, "metric_labels.*",
						map[string]string{
							"source_field": "status",
							"target_label": "Status",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(resourceName, "metric_labels.*",
						map[string]string{
							"source_field": "http_referer",
							"target_label": "Path",
						},
					),
					resource.TestCheckResourceAttr(resourceName, "permutations.0.limit", strconv.Itoa(logs2Metric.limit)),
					resource.TestCheckResourceAttr(resourceName, "permutations.0.has_exceed_limit", "false"),
				),
			},
			{
				ResourceName:      resourceName,
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
			Id: rs.Primary.ID,
		}

		resp, err := client.GetLogs2Metric(ctx, req)
		if err == nil {
			if resp.GetId() == rs.Primary.ID {
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
		limit:       acctest.RandIntRange(0, 999999),
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
