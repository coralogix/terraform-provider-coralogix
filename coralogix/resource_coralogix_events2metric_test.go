package coralogix

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"terraform-provider-coralogix/coralogix/clientset"
	e2m "terraform-provider-coralogix/coralogix/clientset/grpc/events2metrics/v2"

	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

type events2MetricTestFields struct {
	name, description string
	limit             int
}

var events2metricResourceName = "coralogix_events2metric.test"

func TestAccCoralogixResourceLogs2Metric(t *testing.T) {
	events2Metric := getRandomEvents2Metric()
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckEvents2MetricDestroy,
		Steps: []resource.TestStep{
			{

				Config: testAccCoralogixResourceLogs2Metric(events2Metric),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(events2metricResourceName, "id"),
					resource.TestCheckResourceAttr(events2metricResourceName, "name", events2Metric.name),
					resource.TestCheckResourceAttr(events2metricResourceName, "description", events2Metric.description),
					resource.TestCheckResourceAttr(events2metricResourceName, "logs_query.0.lucene", "remote_addr_enriched:/.*/"),
					resource.TestCheckResourceAttr(events2metricResourceName, "logs_query.0.applications.0", "nginx"),
					resource.TestCheckResourceAttr(events2metricResourceName, "logs_query.0.severities.0", "Debug"),
					resource.TestCheckTypeSetElemNestedAttrs(events2metricResourceName, "metric_fields.*",
						map[string]string{
							"source_field":            "remote_addr_geoip.location_geopoint",
							"target_base_metric_name": "geo_point",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(events2metricResourceName, "metric_fields.*",
						map[string]string{
							"source_field":            "method",
							"target_base_metric_name": "method",
						},
					),

					resource.TestCheckTypeSetElemNestedAttrs(events2metricResourceName, "metric_labels.*",
						map[string]string{
							"source_field": "status",
							"target_label": "Status",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(events2metricResourceName, "metric_labels.*",
						map[string]string{
							"source_field": "http_referer",
							"target_label": "Path",
						},
					),
					resource.TestCheckResourceAttr(events2metricResourceName, "permutations.0.limit", strconv.Itoa(events2Metric.limit)),
					resource.TestCheckResourceAttr(events2metricResourceName, "permutations.0.has_exceed_limit", "false"),
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
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckEvents2MetricDestroy,
		Steps: []resource.TestStep{
			{

				Config: testAccCoralogixResourceSpans2Metric(events2Metric),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(events2metricResourceName, "id"),
					resource.TestCheckResourceAttr(events2metricResourceName, "name", events2Metric.name),
					resource.TestCheckResourceAttr(events2metricResourceName, "description", events2Metric.description),
					resource.TestCheckResourceAttr(events2metricResourceName, "spans_query.0.lucene", "remote_addr_enriched:/.*/"),
					resource.TestCheckResourceAttr(events2metricResourceName, "spans_query.0.applications.0", "nginx"),
					resource.TestCheckResourceAttr(events2metricResourceName, "spans_query.0.actions.0", "action-name"),
					resource.TestCheckResourceAttr(events2metricResourceName, "spans_query.0.services.0", "service-name"),
					resource.TestCheckTypeSetElemNestedAttrs(events2metricResourceName, "metric_fields.*",
						map[string]string{
							"source_field":            "remote_addr_geoip.location_geopoint",
							"target_base_metric_name": "geo_point",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(events2metricResourceName, "metric_fields.*",
						map[string]string{
							"source_field":            "method",
							"target_base_metric_name": "method",
						},
					),

					resource.TestCheckTypeSetElemNestedAttrs(events2metricResourceName, "metric_labels.*",
						map[string]string{
							"source_field": "status",
							"target_label": "Status",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(events2metricResourceName, "metric_labels.*",
						map[string]string{
							"source_field": "http_referer",
							"target_label": "Path",
						},
					),
					resource.TestCheckResourceAttr(events2metricResourceName, "permutations.0.limit", strconv.Itoa(events2Metric.limit)),
					resource.TestCheckResourceAttr(events2metricResourceName, "permutations.0.has_exceed_limit", "false"),
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
  logs_query {
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

func testAccCoralogixResourceSpans2Metric(l *events2MetricTestFields) string {
	return fmt.Sprintf(`resource "coralogix_events2metric" "test" {
  name        = "%s"
  description = "%s"
  spans_query {
    lucene       = "remote_addr_enriched:/.*/"
    applications = ["nginx"]
    actions = ["action-name"]
	services = ["service-name"]
  }

  metric_fields {
    target_base_metric_name = "method"
    source_field            = "method"
  }
  metric_fields {
    target_base_metric_name = "geo_point"
    source_field            = "remote_addr_geoip.location_geopoint"
	aggregations {
     min{
        enable = false
      }      
	 max{
        enable = false
      }
      avg{
        enable = false
      }
      histogram{
		buckets = [1.3, 2, 2.7]
      }
  	}
  }
  metric_fields {
    target_base_metric_name = "method"
    source_field            = "method"
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
