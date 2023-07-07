package coralogix

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"terraform-provider-coralogix/coralogix/clientset"
	e2m "terraform-provider-coralogix/coralogix/clientset/grpc/events2metrics/v2"

	"google.golang.org/protobuf/types/known/wrapperspb"
)

type events2MetricTestFields struct {
	name, description string
	limit             int
}

var events2metricResourceName = "coralogix_events2metric.test"

func TestAccCoralogixResourceLogs2Metric(t *testing.T) {
	events2Metric := getRandomEvents2Metric()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckEvents2MetricDestroy,
		Steps: []resource.TestStep{
			{

				Config: testAccCoralogixResourceLogs2Metric(events2Metric),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(events2metricResourceName, "id"),
					resource.TestCheckResourceAttr(events2metricResourceName, "name", events2Metric.name),
					resource.TestCheckResourceAttr(events2metricResourceName, "description", events2Metric.description),
					resource.TestCheckResourceAttr(events2metricResourceName, "logs_query.lucene", "remote_addr_enriched:/.*/"),
					resource.TestCheckResourceAttr(events2metricResourceName, "logs_query.applications.0", "nginx"),
					resource.TestCheckResourceAttr(events2metricResourceName, "logs_query.severities.0", "Debug"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields.geo_point.source_field", "remote_addr_geoip.location_geopoint"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields.geo_point.aggregations.avg.target_metric_name", "cx_avg"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields.geo_point.aggregations.avg.enable", "true"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields.geo_point.aggregations.count.target_metric_name", "cx_count"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields.geo_point.aggregations.count.enable", "true"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields.geo_point.aggregations.histogram.target_metric_name", "cx_bucket"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields.geo_point.aggregations.histogram.enable", "true"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields.geo_point.aggregations.max.target_metric_name", "cx_max"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields.geo_point.aggregations.max.enable", "true"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields.geo_point.aggregations.min.target_metric_name", "cx_min"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields.geo_point.aggregations.min.enable", "true"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields.geo_point.aggregations.sum.target_metric_name", "cx_sum"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields.geo_point.aggregations.sum.enable", "true"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields.method.source_field", "method"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields.method.aggregations.count.target_metric_name", "cx_count"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields.method.aggregations.count.enable", "true"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields.method.aggregations.histogram.target_metric_name", "cx_bucket"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields.method.aggregations.histogram.enable", "true"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields.method.aggregations.max.target_metric_name", "cx_max"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields.method.aggregations.max.enable", "true"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields.method.aggregations.min.target_metric_name", "cx_min"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields.method.aggregations.min.enable", "true"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields.method.aggregations.sum.target_metric_name", "cx_sum"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields.method.aggregations.sum.enable", "true"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields.method.aggregations.avg.target_metric_name", "cx_avg"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields.method.aggregations.avg.enable", "true"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_labels.Status", "status"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_labels.Path", "http_referer"),
					resource.TestCheckResourceAttr(events2metricResourceName, "permutations.limit", strconv.Itoa(events2Metric.limit)),
					resource.TestCheckResourceAttr(events2metricResourceName, "permutations.has_exceed_limit", "false"),
				),
			},
			{
				ResourceName:      events2metricResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccCoralogixResourceSpans2Metric(t *testing.T) {
	events2Metric := getRandomEvents2Metric()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckEvents2MetricDestroy,
		Steps: []resource.TestStep{
			{

				Config: testAccCoralogixResourceSpans2Metric(events2Metric),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(events2metricResourceName, "id"),
					resource.TestCheckResourceAttr(events2metricResourceName, "name", events2Metric.name),
					resource.TestCheckResourceAttr(events2metricResourceName, "description", events2Metric.description),
					resource.TestCheckResourceAttr(events2metricResourceName, "spans_query.lucene", "remote_addr_enriched:/.*/"),
					resource.TestCheckResourceAttr(events2metricResourceName, "spans_query.applications.0", "nginx"),
					resource.TestCheckResourceAttr(events2metricResourceName, "spans_query.actions.0", "action-name"),
					resource.TestCheckResourceAttr(events2metricResourceName, "spans_query.services.0", "service-name"),
					resource.TestCheckTypeSetElemNestedAttrs(events2metricResourceName, "metric_fields",
						map[string]string{
							"geo_point": "remote_addr_geoip.location_geopoint",
							"method":    "method",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(events2metricResourceName, "metric_labels",
						map[string]string{
							"Status": "status",
							"Path":   "http_referer",
						},
					),
					resource.TestCheckResourceAttr(events2metricResourceName, "permutations.limit", strconv.Itoa(events2Metric.limit)),
					resource.TestCheckResourceAttr(events2metricResourceName, "permutations.has_exceed_limit", "false"),
				),
			},
			{
				ResourceName:      events2metricResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccCheckEvents2MetricDestroy(s *terraform.State) error {
	client := testAccProvider.Meta().(*clientset.ClientSet).Events2Metrics()

	ctx := context.TODO()

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "coralogix_events2metric" {
			continue
		}

		req := &e2m.GetE2MRequest{
			Id: wrapperspb.String(rs.Primary.ID),
		}

		resp, err := client.GetEvents2Metric(ctx, req)
		if err == nil {
			if resp.GetE2M().GetId().GetValue() == rs.Primary.ID {
				return fmt.Errorf("events2metric still exists: %s", rs.Primary.ID)
			}
		}
	}

	return nil
}

func getRandomEvents2Metric() *events2MetricTestFields {
	return &events2MetricTestFields{
		name:        acctest.RandStringFromCharSet(10, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ012346789_:"),
		description: acctest.RandomWithPrefix("tf-acc-test"),
		limit:       acctest.RandIntRange(0, 500000),
	}
}

func testAccCoralogixResourceLogs2Metric(l *events2MetricTestFields) string {
	return fmt.Sprintf(`resource "coralogix_events2metric" "test" {
  name        = "%s"
  description = "%s"
  logs_query = {
    lucene       = "remote_addr_enriched:/.*/"
    applications = ["nginx"]
    severities   = ["Debug"]
  }

  metric_fields = {
    method = {
      source_field = "method"
    },
    geo_point = {
      source_field = "remote_addr_geoip.location_geopoint"
      aggregations = {
        max = {
          enable = false
        }
        min = {
          enable = false
        }
        avg = {
          enable = true
        }
      }
    }
  }

  metric_labels = {
    Status = "status"
    Path   = "http_referer"
  }

  permutations = {
    limit = %d
  }
}
`,
		l.name, l.description, l.limit)
}

func testAccCoralogixResourceSpans2Metric(l *events2MetricTestFields) string {
	return fmt.Sprintf(`resource "coralogix_events2metric" "test" {
  name        = "%s"
  description = "%s"
  spans_query = {
    lucene       = "remote_addr_enriched:/.*/"
    applications = ["nginx"]
    actions = ["action-name"]
	services = ["service-name"]
  }

  metric_fields = {
    method = {
      source_field = "method"
    },
    geo_point = {
      source_field = "remote_addr_geoip.location_geopoint"
      aggregations = {
        max = {
          enable = false
        }
        min = {
          enable = false
        }
        avg = {
          enable = true
        }
      }
    }
  }

  metric_labels = {
    Status = "status"
    Path   = "http_referer"
  }

  permutations = {
    limit = %d
  }
}
`,
		l.name, l.description, l.limit)
}
