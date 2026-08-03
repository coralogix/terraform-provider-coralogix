// Copyright 2026 Coralogix Ltd.
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
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

const (
	events2MetricMigrationAcceptanceEnv  = "CORALOGIX_EVENTS2METRIC_MIGRATION_ACC"
	events2MetricMigrationProviderSource = "registry.terraform.io/coralogix/coralogix"
	events2MetricMigrationGRPCVersion    = "= 3.8.0"
)

func TestAccCoralogixResourceEvents2MetricMigration(t *testing.T) {
	requireEvents2MetricMigrationAcceptance(t)

	cases := []struct {
		name          string
		initialConfig func(name, metricField string) string
		updatedConfig func(name, metricField string) string
		extraChecks   func(metricField string) []resource.TestCheckFunc
		updateChecks  func(metricField string) []resource.TestCheckFunc
		skipImport    bool
	}{
		{
			name:          "logs-baseline",
			initialConfig: events2MetricMigrationLogsBaseline,
			updatedConfig: events2MetricMigrationLogsBaselineUpdated,
		},
		{
			name:          "spans-baseline",
			initialConfig: events2MetricMigrationSpansBaseline,
			updatedConfig: events2MetricMigrationSpansBaselineUpdated,
		},
		{
			name:          "no-permutations",
			initialConfig: events2MetricMigrationNoPermutations,
			updatedConfig: events2MetricMigrationNoPermutationsUpdated,
		},
		{
			name:          "empty-aggregations",
			initialConfig: events2MetricMigrationEmptyAggregations,
			updatedConfig: events2MetricMigrationEmptyAggregationsUpdated,
		},
		{
			name:          "data-source-set",
			initialConfig: events2MetricMigrationDataSourceSet,
			updatedConfig: events2MetricMigrationDataSourceRemoved,
			updateChecks: func(string) []resource.TestCheckFunc {
				return []resource.TestCheckFunc{
					resource.TestCheckNoResourceAttr(events2metricResourceName, "data_source"),
				}
			},
		},
		{
			name:          "histogram-buckets",
			initialConfig: events2MetricMigrationHistogramBuckets,
			updatedConfig: events2MetricMigrationHistogramBucketsUpdated,
			extraChecks: func(metricField string) []resource.TestCheckFunc {
				return []resource.TestCheckFunc{
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields."+metricField+".aggregations.histogram.buckets.0", "0.1"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields."+metricField+".aggregations.histogram.buckets.1", "5.5"),
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields."+metricField+".aggregations.histogram.buckets.2", "100"),
				}
			},
		},
		{
			name:          "samples-oneof",
			initialConfig: events2MetricMigrationSamplesMin,
			updatedConfig: events2MetricMigrationSamplesMinUpdated,
		},
		{
			name:          "samples-oneof-max",
			initialConfig: events2MetricMigrationSamplesMax,
			updatedConfig: events2MetricMigrationSamplesMaxUpdated,
		},
		{
			name:          "enable-false",
			initialConfig: events2MetricMigrationEnableFalse,
			updatedConfig: events2MetricMigrationEnableFalseUpdated,
			extraChecks: func(metricField string) []resource.TestCheckFunc {
				return []resource.TestCheckFunc{
					resource.TestCheckResourceAttr(events2metricResourceName, "metric_fields."+metricField+".aggregations.min.enable", "false"),
				}
			},
		},
		{
			name:          "description-set",
			initialConfig: events2MetricMigrationDescriptionSet,
			updatedConfig: events2MetricMigrationDescriptionSetUpdated,
			extraChecks: func(string) []resource.TestCheckFunc {
				return []resource.TestCheckFunc{
					resource.TestCheckResourceAttr(events2metricResourceName, "description", "Created by the gRPC-backed provider"),
				}
			},
		},
		{
			name:          "description-omitted",
			initialConfig: events2MetricMigrationDescriptionOmitted,
			updatedConfig: events2MetricMigrationDescriptionOmittedUpdated,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			name := acctest.RandStringFromCharSet(10, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ012346789_:")
			metricField := "tf_" + acctest.RandStringFromCharSet(12, "abcdefghijklmnopqrstuvwxyz0123456789_")
			initial := tc.initialConfig(name, metricField)
			updated := tc.updatedConfig(name, metricField)
			var extraChecks, updateChecks []resource.TestCheckFunc
			if tc.extraChecks != nil {
				extraChecks = tc.extraChecks(metricField)
			}
			if tc.updateChecks != nil {
				updateChecks = tc.updateChecks(metricField)
			}

			steps := []resource.TestStep{
				{
					Config:            initial,
					ExternalProviders: events2MetricMigrationExternalProvider(events2MetricMigrationGRPCVersion),
					Check:             resource.ComposeAggregateTestCheckFunc(extraChecks...),
				},
				{
					Config:                   initial,
					ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
					ConfigPlanChecks: resource.ConfigPlanChecks{
						PreApply: []plancheck.PlanCheck{
							plancheck.ExpectResourceAction(events2metricResourceName, plancheck.ResourceActionNoop),
						},
						PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
					},
					Check: resource.ComposeAggregateTestCheckFunc(extraChecks...),
				},
				{
					Config:                   updated,
					ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
					ConfigPlanChecks: resource.ConfigPlanChecks{
						PreApply: []plancheck.PlanCheck{
							plancheck.ExpectResourceAction(events2metricResourceName, plancheck.ResourceActionUpdate),
						},
						PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
					},
					Check: resource.ComposeAggregateTestCheckFunc(updateChecks...),
				},
			}
			if !tc.skipImport {
				steps = append(steps, resource.TestStep{
					ResourceName:             events2metricResourceName,
					ImportState:              true,
					ImportStateVerify:        true,
					ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				})
			}

			resource.ParallelTest(t, resource.TestCase{
				PreCheck:     func() { testAccPreCheck(t) },
				CheckDestroy: testAccCheckEvents2MetricDestroy,
				Steps:        steps,
			})
		})
	}
}

func requireEvents2MetricMigrationAcceptance(t *testing.T) {
	t.Helper()
	if os.Getenv(events2MetricMigrationAcceptanceEnv) == "" {
		t.Skipf("set %s=1 to run registry-backed events2metric migration tests", events2MetricMigrationAcceptanceEnv)
	}
	if namespace := os.Getenv(resource.EnvTfAccProviderNamespace); namespace != "coralogix" {
		t.Fatalf("set %s=coralogix to run registry-backed events2metric migration tests", resource.EnvTfAccProviderNamespace)
	}
}

func events2MetricMigrationExternalProvider(version string) map[string]resource.ExternalProvider {
	return map[string]resource.ExternalProvider{
		"coralogix": {
			Source:            events2MetricMigrationProviderSource,
			VersionConstraint: version,
		},
	}
}

func events2MetricMigrationLogsBaseline(name, metricField string) string {
	return fmt.Sprintf(`resource "coralogix_events2metric" "test" {
  name        = %q
  description = "Created by the gRPC-backed provider"
  logs_query = {
    lucene       = "remote_addr_enriched:/.*/"
    applications = ["nginx"]
    severities   = ["Debug"]
  }
  metric_fields = {
    %s = {
      source_field = "duration"
      aggregations = {
        min = { enable = false }
        max = { enable = false }
        avg = { enable = true }
        sum = { enable = true }
        count = { enable = true }
      }
    }
  }
  metric_labels = {
    Status = "status"
    Path   = "http_referer"
  }
  permutations = {
    limit = 1000
  }
}
`, name, metricField)
}

func events2MetricMigrationLogsBaselineUpdated(name, metricField string) string {
	return fmt.Sprintf(`resource "coralogix_events2metric" "test" {
  name        = %q
  description = "Updated by the REST-backed provider"
  logs_query = {
    lucene       = "remote_addr_enriched:/.*/"
    applications = ["nginx"]
    severities   = ["Debug"]
  }
  metric_fields = {
    %s = {
      source_field = "duration"
      aggregations = {
        min = { enable = false }
        max = { enable = false }
        avg = { enable = true }
        sum = { enable = true }
        count = { enable = true }
      }
    }
  }
  metric_labels = {
    Status = "status"
    Path   = "http_referer"
  }
  permutations = {
    limit = 1000
  }
}
`, name, metricField)
}

func events2MetricMigrationSpansBaseline(name, metricField string) string {
	return fmt.Sprintf(`resource "coralogix_events2metric" "test" {
  name        = %q
  description = "Created by the gRPC-backed provider"
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
        min = { enable = false }
        max = { enable = false }
        avg = { enable = true }
        sum = { enable = true }
        count = { enable = true }
      }
    }
  }
  metric_labels = {
    Status = "status"
    Path   = "http_referer"
  }
  permutations = {
    limit = 1000
  }
}
`, name, metricField)
}

func events2MetricMigrationSpansBaselineUpdated(name, metricField string) string {
	return fmt.Sprintf(`resource "coralogix_events2metric" "test" {
  name        = %q
  description = "Updated by the REST-backed provider"
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
        min = { enable = false }
        max = { enable = false }
        avg = { enable = true }
        sum = { enable = true }
        count = { enable = true }
      }
    }
  }
  metric_labels = {
    Status = "status"
    Path   = "http_referer"
  }
  permutations = {
    limit = 1000
  }
}
`, name, metricField)
}

// Registry 3.8.0 cannot create when permutations is omitted (*PermutationsModel
// cannot hold unknown). Use limit=0 as the pre-fix "absent" equivalent.
func events2MetricMigrationNoPermutations(name, metricField string) string {
	return fmt.Sprintf(`resource "coralogix_events2metric" "test" {
  name        = %q
  description = "Created by the gRPC-backed provider"
  logs_query = {
    lucene       = "remote_addr_enriched:/.*/"
    applications = ["nginx"]
    severities   = ["Debug"]
  }
  metric_fields = {
    %s = {
      source_field = "duration"
      aggregations = {
        avg = { enable = true }
      }
    }
  }
  permutations = {
    limit = 0
  }
}
`, name, metricField)
}

func events2MetricMigrationNoPermutationsUpdated(name, metricField string) string {
	return fmt.Sprintf(`resource "coralogix_events2metric" "test" {
  name        = %q
  description = "Updated by the REST-backed provider"
  logs_query = {
    lucene       = "remote_addr_enriched:/.*/"
    applications = ["nginx"]
    severities   = ["Debug"]
  }
  metric_fields = {
    %s = {
      source_field = "duration"
      aggregations = {
        avg = { enable = true }
      }
    }
  }
  permutations = {
    limit = 0
  }
}
`, name, metricField)
}

func events2MetricMigrationEmptyAggregations(name, metricField string) string {
	return fmt.Sprintf(`resource "coralogix_events2metric" "test" {
  name        = %q
  description = "Created by the gRPC-backed provider"
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
  permutations = {
    limit = 0
  }
}
`, name, metricField)
}

func events2MetricMigrationEmptyAggregationsUpdated(name, metricField string) string {
	return fmt.Sprintf(`resource "coralogix_events2metric" "test" {
  name        = %q
  description = "Updated by the REST-backed provider"
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
  permutations = {
    limit = 0
  }
}
`, name, metricField)
}

func events2MetricMigrationDataSourceSet(name, metricField string) string {
	return fmt.Sprintf(`resource "coralogix_events2metric" "test" {
  name        = %q
  description = "Created by the gRPC-backed provider"
  data_source = "default/logs"
  logs_query = {
    lucene       = "remote_addr_enriched:/.*/"
    applications = ["nginx"]
    severities   = ["Debug"]
  }
  metric_fields = {
    %s = {
      source_field = "duration"
      aggregations = {
        avg = { enable = true }
      }
    }
  }
  permutations = {
    limit = 0
  }
}
`, name, metricField)
}

func events2MetricMigrationDataSourceRemoved(name, metricField string) string {
	return fmt.Sprintf(`resource "coralogix_events2metric" "test" {
  name        = %q
  description = "Updated by the REST-backed provider"
  logs_query = {
    lucene       = "remote_addr_enriched:/.*/"
    applications = ["nginx"]
    severities   = ["Debug"]
  }
  metric_fields = {
    %s = {
      source_field = "duration"
      aggregations = {
        avg = { enable = true }
      }
    }
  }
  permutations = {
    limit = 0
  }
}
`, name, metricField)
}

func events2MetricMigrationHistogramBuckets(name, metricField string) string {
	return fmt.Sprintf(`resource "coralogix_events2metric" "test" {
  name        = %q
  description = "Created by the gRPC-backed provider"
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
  permutations = {
    limit = 0
  }
}
`, name, metricField)
}

func events2MetricMigrationHistogramBucketsUpdated(name, metricField string) string {
	return fmt.Sprintf(`resource "coralogix_events2metric" "test" {
  name        = %q
  description = "Updated by the REST-backed provider"
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
  permutations = {
    limit = 0
  }
}
`, name, metricField)
}

func events2MetricMigrationSamplesMin(name, metricField string) string {
	return fmt.Sprintf(`resource "coralogix_events2metric" "test" {
  name        = %q
  description = "Created by the gRPC-backed provider"
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
          type   = "Min"
        }
      }
    }
  }
  permutations = {
    limit = 0
  }
}
`, name, metricField)
}

func events2MetricMigrationSamplesMinUpdated(name, metricField string) string {
	return fmt.Sprintf(`resource "coralogix_events2metric" "test" {
  name        = %q
  description = "Updated by the REST-backed provider"
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
          type   = "Min"
        }
      }
    }
  }
  permutations = {
    limit = 0
  }
}
`, name, metricField)
}

func events2MetricMigrationSamplesMax(name, metricField string) string {
	return fmt.Sprintf(`resource "coralogix_events2metric" "test" {
  name        = %q
  description = "Created by the gRPC-backed provider"
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
          type   = "Max"
        }
      }
    }
  }
  permutations = {
    limit = 0
  }
}
`, name, metricField)
}

func events2MetricMigrationSamplesMaxUpdated(name, metricField string) string {
	return fmt.Sprintf(`resource "coralogix_events2metric" "test" {
  name        = %q
  description = "Updated by the REST-backed provider"
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
          type   = "Max"
        }
      }
    }
  }
  permutations = {
    limit = 0
  }
}
`, name, metricField)
}

func events2MetricMigrationEnableFalse(name, metricField string) string {
	return fmt.Sprintf(`resource "coralogix_events2metric" "test" {
  name        = %q
  description = "Created by the gRPC-backed provider"
  logs_query = {
    lucene       = "remote_addr_enriched:/.*/"
    applications = ["nginx"]
    severities   = ["Debug"]
  }
  metric_fields = {
    %s = {
      source_field = "duration"
      aggregations = {
        min = { enable = false }
      }
    }
  }
  permutations = {
    limit = 0
  }
}
`, name, metricField)
}

func events2MetricMigrationEnableFalseUpdated(name, metricField string) string {
	return fmt.Sprintf(`resource "coralogix_events2metric" "test" {
  name        = %q
  description = "Updated by the REST-backed provider"
  logs_query = {
    lucene       = "remote_addr_enriched:/.*/"
    applications = ["nginx"]
    severities   = ["Debug"]
  }
  metric_fields = {
    %s = {
      source_field = "duration"
      aggregations = {
        min = { enable = false }
      }
    }
  }
  permutations = {
    limit = 0
  }
}
`, name, metricField)
}

func events2MetricMigrationDescriptionSet(name, metricField string) string {
	return fmt.Sprintf(`resource "coralogix_events2metric" "test" {
  name        = %q
  description = "Created by the gRPC-backed provider"
  logs_query = {
    lucene       = "remote_addr_enriched:/.*/"
    applications = ["nginx"]
    severities   = ["Debug"]
  }
  metric_fields = {
    %s = {
      source_field = "duration"
      aggregations = {
        avg = { enable = true }
      }
    }
  }
  permutations = {
    limit = 0
  }
}
`, name, metricField)
}

func events2MetricMigrationDescriptionSetUpdated(name, metricField string) string {
	return fmt.Sprintf(`resource "coralogix_events2metric" "test" {
  name        = %q
  description = "Updated by the REST-backed provider"
  logs_query = {
    lucene       = "remote_addr_enriched:/.*/"
    applications = ["nginx"]
    severities   = ["Debug"]
  }
  metric_fields = {
    %s = {
      source_field = "duration"
      aggregations = {
        avg = { enable = true }
      }
    }
  }
  permutations = {
    limit = 0
  }
}
`, name, metricField)
}

func events2MetricMigrationDescriptionOmitted(name, metricField string) string {
	return fmt.Sprintf(`resource "coralogix_events2metric" "test" {
  name = %q
  logs_query = {
    lucene       = "remote_addr_enriched:/.*/"
    applications = ["nginx"]
    severities   = ["Debug"]
  }
  metric_fields = {
    %s = {
      source_field = "duration"
      aggregations = {
        avg = { enable = true }
      }
    }
  }
  permutations = {
    limit = 0
  }
}
`, name, metricField)
}

func events2MetricMigrationDescriptionOmittedUpdated(name, metricField string) string {
	return fmt.Sprintf(`resource "coralogix_events2metric" "test" {
  name        = %q
  description = "Updated by the REST-backed provider"
  logs_query = {
    lucene       = "remote_addr_enriched:/.*/"
    applications = ["nginx"]
    severities   = ["Debug"]
  }
  metric_fields = {
    %s = {
      source_field = "duration"
      aggregations = {
        avg = { enable = true }
      }
    }
  }
  permutations = {
    limit = 0
  }
}
`, name, metricField)
}
