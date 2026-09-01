// Copyright 2024 Coralogix Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package provider

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

type events2MetricTestFields struct {
	name, description, metricField string
	limit                          int
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
				// Named data_source "default/logs" is rejected on accounts that
				// have not provisioned that catalog entry. Omit the field so
				// the backend uses the implicit logs stream.
				Config: testAccCoralogixResourceLogs2Metric(events2Metric, ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(events2metricResourceName, "id"),
					resource.TestCheckResourceAttr(events2metricResourceName, "name", events2Metric.name),
					resource.TestCheckResourceAttr(events2metricResourceName, "description", events2Metric.description),
					resource.TestCheckResourceAttr(events2metricResourceName, "logs_query.lucene", "remote_addr_enriched:/.*/"),
					resource.TestCheckResourceAttr(events2metricResourceName, "logs_query.applications.0", "nginx"),
					resource.TestCheckResourceAttr(events2metricResourceName, "logs_query.severities.0", "Debug"),

					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields."+events2Metric.metricField+".source_field", "duration"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields."+events2Metric.metricField+".aggregations.count.target_metric_name", "cx_count"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields."+events2Metric.metricField+".aggregations.count.enable", "true"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields."+events2Metric.metricField+".aggregations.histogram.target_metric_name", "cx_bucket"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields."+events2Metric.metricField+".aggregations.histogram.enable", "false"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields."+events2Metric.metricField+".aggregations.max.target_metric_name", "cx_max"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields."+events2Metric.metricField+".aggregations.max.enable", "false"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields."+events2Metric.metricField+".aggregations.min.target_metric_name", "cx_min"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields."+events2Metric.metricField+".aggregations.min.enable", "false"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields."+events2Metric.metricField+".aggregations.sum.target_metric_name", "cx_sum"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields."+events2Metric.metricField+".aggregations.sum.enable", "true"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields."+events2Metric.metricField+".aggregations.avg.target_metric_name", "cx_avg"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields."+events2Metric.metricField+".aggregations.avg.enable", "true"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_labels.Status", "status"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_labels.Path", "http_referer"),
					resource.TestCheckResourceAttr(events2metricResourceName, "permutations.limit", strconv.Itoa(events2Metric.limit)),
					resource.TestCheckResourceAttr(events2metricResourceName, "permutations.has_exceed_limit", "false"),
					resource.TestCheckNoResourceAttr(events2metricResourceName, "data_source"),
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

					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields."+events2Metric.metricField+".source_field", "duration"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields."+events2Metric.metricField+".aggregations.count.target_metric_name", "cx_count"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields."+events2Metric.metricField+".aggregations.count.enable", "true"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields."+events2Metric.metricField+".aggregations.histogram.target_metric_name", "cx_bucket"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields."+events2Metric.metricField+".aggregations.histogram.enable", "false"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields."+events2Metric.metricField+".aggregations.max.target_metric_name", "cx_max"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields."+events2Metric.metricField+".aggregations.max.enable", "false"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields."+events2Metric.metricField+".aggregations.min.target_metric_name", "cx_min"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields."+events2Metric.metricField+".aggregations.min.enable", "false"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields."+events2Metric.metricField+".aggregations.sum.target_metric_name", "cx_sum"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields."+events2Metric.metricField+".aggregations.sum.enable", "true"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields."+events2Metric.metricField+".aggregations.avg.target_metric_name", "cx_avg"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields."+events2Metric.metricField+".aggregations.avg.enable", "true"),
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

func TestAccCoralogixResourceEvents2MetricFractionalBuckets(t *testing.T) {
	events2Metric := getRandomEvents2Metric()
	mf := events2Metric.metricField
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckEvents2MetricDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceEvents2MetricHistogram(events2Metric),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields."+mf+".aggregations.histogram.enable", "true"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields."+mf+".aggregations.histogram.buckets.0", "0.1"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields."+mf+".aggregations.histogram.buckets.1", "5.5"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields."+mf+".aggregations.histogram.buckets.2", "100"),
				),
			},
			{
				Config:   testAccCoralogixResourceEvents2MetricHistogram(events2Metric),
				PlanOnly: true,
			},
		},
	})
}

func TestAccCoralogixResourceEvents2MetricSamples(t *testing.T) {
	events2Metric := getRandomEvents2Metric()
	mf := events2Metric.metricField
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckEvents2MetricDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceEvents2MetricSamples(events2Metric, "Min"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields."+mf+".aggregations.samples.enable", "true"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields."+mf+".aggregations.samples.type", "Min"),
				),
			},
			{
				Config:   testAccCoralogixResourceEvents2MetricSamples(events2Metric, "Min"),
				PlanOnly: true,
			},
		},
	})
}

// Full aggregations oneOf: every aggregation branch on one metric field
// (simple/none + histogram + samples), matching the API contract shape.
func TestAccCoralogixResourceEvents2MetricFullAggregations(t *testing.T) {
	events2Metric := getRandomEvents2Metric()
	mf := events2Metric.metricField
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckEvents2MetricDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceEvents2MetricFullAggregations(events2Metric),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields."+mf+".source_field", "duration"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields."+mf+".aggregations.avg.enable", "true"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields."+mf+".aggregations.avg.target_metric_name", "cx_avg"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields."+mf+".aggregations.histogram.enable", "false"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields."+mf+".aggregations.histogram.target_metric_name", "cx_bucket"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields."+mf+".aggregations.count.enable", "true"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields."+mf+".aggregations.count.target_metric_name", "cx_count"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields."+mf+".aggregations.max.enable", "true"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields."+mf+".aggregations.max.target_metric_name", "cx_max"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields."+mf+".aggregations.min.enable", "true"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields."+mf+".aggregations.min.target_metric_name", "cx_min"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields."+mf+".aggregations.sum.enable", "true"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields."+mf+".aggregations.sum.target_metric_name", "cx_sum"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields."+mf+".aggregations.samples.enable", "true"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields."+mf+".aggregations.samples.type", "Max"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields."+mf+".aggregations.samples.target_metric_name", "cx_value"),
				),
			},
			{
				Config:   testAccCoralogixResourceEvents2MetricFullAggregations(events2Metric),
				PlanOnly: true,
			},
		},
	})
}

func TestAccCoralogixResourceEvents2MetricEnableFalse(t *testing.T) {
	events2Metric := getRandomEvents2Metric()
	mf := events2Metric.metricField
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckEvents2MetricDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceEvents2MetricEnableFalse(events2Metric),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields."+mf+".aggregations.min.enable", "false"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields."+mf+".aggregations.max.enable", "false"),
				),
			},
			{
				Config:   testAccCoralogixResourceEvents2MetricEnableFalse(events2Metric),
				PlanOnly: true,
			},
		},
	})
}

func TestAccCoralogixResourceEvents2MetricEmptyAggregations(t *testing.T) {
	events2Metric := getRandomEvents2Metric()
	mf := events2Metric.metricField
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckEvents2MetricDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceEvents2MetricEmptyAggregations(events2Metric),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields."+mf+".source_field", "method"),
				),
			},
			{
				Config:   testAccCoralogixResourceEvents2MetricEmptyAggregations(events2Metric),
				PlanOnly: true,
			},
		},
	})
}

func testAccCheckEvents2MetricDestroy(s *terraform.State) error {
	clients, err := testAccNewClientSet()
	if err != nil {
		return err
	}
	client := clients.Events2Metrics()

	ctx := context.TODO()

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "coralogix_events2metric" {
			continue
		}

		resp, httpResponse, err := client.Events2MetricServiceGetE2M(ctx, rs.Primary.ID).Execute()
		if err == nil {
			if resp != nil && resp.E2m.GetId() == rs.Primary.ID {
				return fmt.Errorf("events2metric still exists: %s", rs.Primary.ID)
			}
			continue
		}
		if httpResponse == nil || httpResponse.StatusCode != http.StatusNotFound {
			return fmt.Errorf("error checking events2metric destroy for %s: %w", rs.Primary.ID, err)
		}
	}

	return nil
}

func getRandomEvents2Metric() *events2MetricTestFields {
	// Metric field keys are unique account-wide; randomize to avoid collisions
	// with leftovers and parallel tests.
	return &events2MetricTestFields{
		name:        acctest.RandStringFromCharSet(10, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ012346789_:"),
		description: acctest.RandomWithPrefix("tf-acc-test"),
		metricField: "tf_" + acctest.RandStringFromCharSet(12, "abcdefghijklmnopqrstuvwxyz0123456789_"),
		limit:       acctest.RandIntRange(0, 500000),
	}
}

func testAccCoralogixResourceLogs2Metric(l *events2MetricTestFields, dataSource string) string {
	dataSourceAttr := ""
	if dataSource != "" {
		dataSourceAttr = fmt.Sprintf("data_source = %q\n", dataSource)
	}
	return fmt.Sprintf(`resource "coralogix_events2metric" "test" {
name        = "%s"
description = "%s"
%slogs_query = {
    lucene       = "remote_addr_enriched:/.*/"
    applications = ["nginx"]
    severities   = ["Debug"]
}

metric_fields = {
    %s = {
        source_field = "duration"
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
		l.name, l.description, dataSourceAttr, l.metricField, l.limit)
}

func testAccCoralogixResourceSpans2Metric(l *events2MetricTestFields) string {
	return fmt.Sprintf(`resource "coralogix_events2metric" "test" {
name        = "%s"
description = "%s"
spans_query = {
    lucene       = "remote_addr_enriched:/.*/"
    applications = ["nginx"]
    actions      = ["action-name"]
    services     = ["service-name"]
}

metric_fields = {
    %s = {
        source_field = "duration"
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
		l.name, l.description, l.metricField, l.limit)
}

func testAccCoralogixResourceEvents2MetricHistogram(l *events2MetricTestFields) string {
	return fmt.Sprintf(`resource "coralogix_events2metric" "test" {
  name        = %q
  description = %q
  logs_query = {
    lucene       = "remote_addr_enriched:/.*/"
    applications = ["nginx"]
    severities   = ["Debug"]
  }
  metric_fields = {
    %s = {
      source_field = "duration"
      aggregations = {
        histogram = {
          enable  = true
          buckets = [0.1, 5.5, 100]
        }
      }
    }
  }
}
`, l.name, l.description, l.metricField)
}

func testAccCoralogixResourceEvents2MetricSamples(l *events2MetricTestFields, sampleType string) string {
	return fmt.Sprintf(`resource "coralogix_events2metric" "test" {
  name        = %q
  description = %q
  logs_query = {
    lucene       = "remote_addr_enriched:/.*/"
    applications = ["nginx"]
    severities   = ["Debug"]
  }
  metric_fields = {
    %s = {
      source_field = "duration"
      aggregations = {
        samples = {
          enable = true
          type   = %q
        }
      }
    }
  }
}
`, l.name, l.description, l.metricField, sampleType)
}

func testAccCoralogixResourceEvents2MetricFullAggregations(l *events2MetricTestFields) string {
	return fmt.Sprintf(`resource "coralogix_events2metric" "test" {
  name        = %q
  description = %q
  logs_query = {
    lucene       = "remote_addr_enriched:/.*/"
    applications = ["nginx"]
    severities   = ["Debug"]
  }
  metric_fields = {
    %s = {
      source_field = "duration"
      aggregations = {
        avg = {
          enable = true
        }
        histogram = {
          enable  = false
          buckets = []
        }
        count = {
          enable = true
        }
        max = {
          enable = true
        }
        min = {
          enable = true
        }
        sum = {
          enable = true
        }
        samples = {
          enable = true
          type   = "Max"
        }
      }
    }
  }
}
`, l.name, l.description, l.metricField)
}

func testAccCoralogixResourceEvents2MetricEnableFalse(l *events2MetricTestFields) string {
	return fmt.Sprintf(`resource "coralogix_events2metric" "test" {
  name        = %q
  description = %q
  logs_query = {
    lucene       = "remote_addr_enriched:/.*/"
    applications = ["nginx"]
    severities   = ["Debug"]
  }
  metric_fields = {
    %s = {
      source_field = "duration"
      aggregations = {
        min = {
          enable = false
        }
        max = {
          enable = false
        }
      }
    }
  }
}
`, l.name, l.description, l.metricField)
}

func testAccCoralogixResourceEvents2MetricEmptyAggregations(l *events2MetricTestFields) string {
	return fmt.Sprintf(`resource "coralogix_events2metric" "test" {
  name        = %q
  description = %q
  logs_query = {
    lucene       = "remote_addr_enriched:/.*/"
    applications = ["nginx"]
    severities   = ["Debug"]
  }
  metric_fields = {
    %s = {
      source_field = "method"
    }
  }
}
`, l.name, l.description, l.metricField)
}
