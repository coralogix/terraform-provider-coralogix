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
	"os"
	"regexp"
	"strings"
	"testing"

	slos "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/slos_service"
	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

var sloV2ResourceName = "coralogix_slo_v2.test"

// sloV2APMServiceEnvVar names an APM Service Catalog service that exists in the
// tenant under test. APM SLOs cannot be created without one - the backend
// rejects unknown service names - and catalog contents differ per tenant, so
// there is no safe default to fall back on.
const sloV2APMServiceEnvVar = "CORALOGIX_SLO_V2_APM_SERVICE"

func testAccSLOV2Client() *slos.SlosServiceAPIService {
	environmentAlias := strings.ToUpper(os.Getenv("CORALOGIX_ENV"))
	grpcURL, sdkEnvironment := terraformEnvironmentAliasToGrpcUrl[environmentAlias], terraformEnvironmentAliasToSdkEnvironment[environmentAlias]
	if domain := os.Getenv("CORALOGIX_DOMAIN"); domain != "" {
		grpcURL, sdkEnvironment = domain, domain
	}

	return clientset.NewClientSet(sdkEnvironment, os.Getenv("CORALOGIX_API_KEY"), grpcURL).SLOs()
}

// testAccSLOV2APMPreCheck fails, rather than skips, an APM acceptance test with
// no service to point at. It belongs in PreCheck: that runs only once the test
// has decided it is an acceptance run, so `make test` still passes without the
// variable.
func testAccSLOV2APMPreCheck(t *testing.T) {
	t.Helper()
	testAccPreCheck(t)
	if os.Getenv(sloV2APMServiceEnvVar) == "" {
		t.Fatalf("%s must be set to the name of an APM Service Catalog service in the test tenant to run the coralogix_slo_v2 APM acceptance tests", sloV2APMServiceEnvVar)
	}
}

func TestAccCoralogixResourceSLOV2RequestBased(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccSLOV2CheckDestroy,
		Steps: []resource.TestStep{
			{
				Config:  testAccCoralogixSLOV2RequestBased(),
				Destroy: false,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(sloV2ResourceName, "name", "coralogix_slo_go_example"),
					resource.TestCheckResourceAttr(sloV2ResourceName, "description", "Example SLO for Coralogix using request-based metrics"),
					resource.TestCheckResourceAttr(sloV2ResourceName, "target_threshold_percentage", "30"),
				),
			},
		},
	})
}

func TestAccCoralogixResourceSLOV2WindowBased(t *testing.T) {

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccSLOV2CheckDestroy,
		Steps: []resource.TestStep{
			{
				Config:  testAccCoralogixSLOV2WindowBased(),
				Destroy: false,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(sloV2ResourceName, "name", "coralogix_window_based_slo"),
					resource.TestCheckResourceAttr(sloV2ResourceName, "description", "Example SLO using window-based metrics"),
					resource.TestCheckResourceAttr(sloV2ResourceName, "target_threshold_percentage", "95"),
				),
			},
		},
	})
}

// TestAccCoralogixResourceSLOV2MissingDataStrategy walks set -> change -> reset
// -> remove. missing_data_strategy is Optional+Computed, so removing it from the
// configuration keeps the last applied value; the reset has to be explicit.
func TestAccCoralogixResourceSLOV2MissingDataStrategy(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccSLOV2CheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixSLOV2WindowSLO(`missing_data_strategy = "good"`, ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(sloV2ResourceName, "sli.window_based_metric_sli.missing_data_strategy", "good"),
					resource.TestCheckResourceAttr(sloV2ResourceName, "product_type", "unspecified"),
				),
			},
			{
				Config: testAccCoralogixSLOV2WindowSLO(`missing_data_strategy = "bad"`, ""),
				Check: resource.TestCheckResourceAttr(
					sloV2ResourceName, "sli.window_based_metric_sli.missing_data_strategy", "bad"),
			},
			{
				Config: testAccCoralogixSLOV2WindowSLO(`missing_data_strategy = "uncounted"`, ""),
				Check: resource.TestCheckResourceAttr(
					sloV2ResourceName, "sli.window_based_metric_sli.missing_data_strategy", "uncounted"),
			},
			{
				// Removing the attribute after the explicit reset must not plan
				// anything: the prior known value is replayed on replace.
				Config:   testAccCoralogixSLOV2WindowSLO("", ""),
				PlanOnly: true,
			},
		},
	})
}

// TestAccCoralogixResourceSLOV2MissingDataStrategyOmitted covers the create path
// where the attribute is absent: the backend applies its own default and the
// next plan has to be empty.
func TestAccCoralogixResourceSLOV2MissingDataStrategyOmitted(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccSLOV2CheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixSLOV2WindowSLO("", ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(sloV2ResourceName, "sli.window_based_metric_sli.missing_data_strategy", "uncounted"),
					resource.TestCheckResourceAttr(sloV2ResourceName, "product_type", "unspecified"),
					resource.TestCheckNoResourceAttr(sloV2ResourceName, "ownership_tags"),
					resource.TestCheckNoResourceAttr(sloV2ResourceName, "apm_sli_metadata"),
				),
			},
			{
				Config:   testAccCoralogixSLOV2WindowSLO("", ""),
				PlanOnly: true,
			},
			{
				ResourceName:      sloV2ResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TestAccCoralogixResourceSLOV2OwnershipTags asserts submitted ordering, that a
// label_keys-only dimension keeps static_values null rather than the empty array
// the backend injects, and that removing the block clears the tags.
func TestAccCoralogixResourceSLOV2OwnershipTags(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccSLOV2CheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixSLOV2OwnershipTags(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(sloV2ResourceName, "ownership_tags.environment.static_values.#", "3"),
					resource.TestCheckResourceAttr(sloV2ResourceName, "ownership_tags.environment.static_values.0", "prod"),
					resource.TestCheckResourceAttr(sloV2ResourceName, "ownership_tags.environment.static_values.1", "staging"),
					resource.TestCheckResourceAttr(sloV2ResourceName, "ownership_tags.environment.static_values.2", "dev"),
					resource.TestCheckNoResourceAttr(sloV2ResourceName, "ownership_tags.environment.label_keys"),
					resource.TestCheckResourceAttr(sloV2ResourceName, "ownership_tags.team.label_keys.#", "2"),
					resource.TestCheckResourceAttr(sloV2ResourceName, "ownership_tags.team.label_keys.0", "owning_team"),
					resource.TestCheckNoResourceAttr(sloV2ResourceName, "ownership_tags.team.static_values"),
					resource.TestCheckNoResourceAttr(sloV2ResourceName, "ownership_tags.service"),
				),
			},
			{
				Config:   testAccCoralogixSLOV2OwnershipTags(),
				PlanOnly: true,
			},
			{
				ResourceName:      sloV2ResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				// A replace without the block clears the tags rather than
				// merging them.
				Config: testAccCoralogixSLOV2WindowSLO("", ""),
				Check:  resource.TestCheckNoResourceAttr(sloV2ResourceName, "ownership_tags"),
			},
			{
				Config:   testAccCoralogixSLOV2WindowSLO("", ""),
				PlanOnly: true,
			},
		},
	})
}

// TestAccCoralogixResourceSLOV2Validation covers the plan-time rejections that
// keep a configuration from round-tripping ambiguously. None of these steps
// reaches the API.
func TestAccCoralogixResourceSLOV2Validation(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccSLOV2CheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixSLOV2WithOwnershipTags(`
    environment = {
      static_values = ["prod"]
      label_keys    = ["env"]
    }`),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`Invalid Attribute Combination`),
			},
			{
				Config: testAccCoralogixSLOV2WithOwnershipTags(`
    environment = {
      static_values = []
    }`),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`(?s)static_values.*at least\s+1 element`),
			},
			{
				Config:      testAccCoralogixSLOV2WithOwnershipTags(""),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`Invalid Attribute Combination`),
			},
			{
				Config:      testAccCoralogixSLOV2APMSLI("svc-does-not-matter", `error_config = {}`, `filters = []`),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`(?s)filters.*at least\s+1 element`),
			},
			{
				Config:      testAccCoralogixSLOV2APMSLI("svc-does-not-matter", `error_config = {}`, `grouping_keys = []`),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`(?s)grouping_keys.*at least\s+1 element`),
			},
			{
				Config: strings.Replace(
					testAccCoralogixSLOV2APMSLI("svc-does-not-matter", `error_config = {}`, ""),
					`services = ["svc-does-not-matter"]`,
					`services = ["service-a", "service-b"]`,
					1,
				),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`(?s)services.*at least\s+1 elements.*at\s+most\s+1 elements`),
			},
			{
				Config: testAccCoralogixSLOV2APMSLI("svc-does-not-matter", `
      error_config   = {}
      latency_config = {
        time_window = "1_minute"
      }`, ""),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`Invalid Attribute Combination`),
			},
			{
				// An apm_sli carrying neither branch is rejected by the API with
				// "Unsupported APM SLI type: undefined".
				Config:      testAccCoralogixSLOV2APMSLI("svc-does-not-matter", "", ""),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`Invalid Attribute Combination`),
			},
			{
				// A latency_config carrying neither quantile nor average is
				// rejected by the API with "Latency query type must be specified".
				Config: testAccCoralogixSLOV2APMSLI("svc-does-not-matter", `
      latency_config = {
        time_window = "1_minute"
        threshold   = 500
      }`, ""),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`Invalid Attribute Combination`),
			},
			{
				// Both set never reaches the API: the SDK's MarshalJSON refuses
				// with "at most one of [quantile, average] may be set".
				Config: testAccCoralogixSLOV2APMSLI("svc-does-not-matter", `
      latency_config = {
        time_window = "1_minute"
        quantile    = { percentile = 0.95 }
        average     = {}
      }`, ""),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`Invalid Attribute Combination`),
			},
			{
				// The backend has no implementation for the unspecified window and
				// answers an HTTP 500, so it must not be offered as a value.
				Config: testAccCoralogixSLOV2APMSLI("svc-does-not-matter", `
      latency_config = {
        time_window = "unspecified"
        quantile    = { percentile = 0.95 }
      }`, ""),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`(?s)time_window value must be one of.*got: "unspecified"`),
			},
			{
				Config:      testAccCoralogixSLOV2WindowBasedWithWindow("unspecified"),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`(?s)window value must be one of.*got: "unspecified"`),
			},
		},
	})
}

// testAccCoralogixSLOV2WindowBasedWithWindow varies the window of a window-based
// SLI, which testAccCoralogixSLOV2WindowSLO hard-codes.
func testAccCoralogixSLOV2WindowBasedWithWindow(window string) string {
	return fmt.Sprintf(`
resource "coralogix_slo_v2" "test" {
  name                        = "coralogix_slo_v2_acc_window"
  description                 = "Window based SLO used by the coralogix_slo_v2 acceptance tests"
  target_threshold_percentage = 95.0
  sli = {
    window_based_metric_sli = {
      query = {
        query = "avg(avg_over_time(request_duration_seconds[1m]))"
      }
      window              = %q
      comparison_operator = "less_than"
      threshold           = 0.25
    }
  }
  window = {
    slo_time_frame = "7_days"
  }
}
`, window)
}

// TestAccCoralogixResourceSLOV2APMError creates an APM error SLO. The second
// plan is what catches the empty filters, empty groupingKeys and derived
// grouping labels the backend injects leaking into state as drift.
func TestAccCoralogixResourceSLOV2APMError(t *testing.T) {
	service := os.Getenv(sloV2APMServiceEnvVar)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccSLOV2APMPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccSLOV2CheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixSLOV2APMSLI(service, `error_config = {}`, ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(sloV2ResourceName, "product_type", "apm"),
					resource.TestCheckResourceAttr(sloV2ResourceName, "sli.apm_sli.services.0", service),
					resource.TestCheckNoResourceAttr(sloV2ResourceName, "sli.apm_sli.filters"),
					resource.TestCheckNoResourceAttr(sloV2ResourceName, "sli.apm_sli.grouping_keys"),
					resource.TestCheckResourceAttr(sloV2ResourceName, "grouping.labels.0", "service_name"),
					// apm_sli_metadata mirrors the configured APM SLI.
					resource.TestCheckResourceAttr(sloV2ResourceName, "apm_sli_metadata.services.0", service),
					resource.TestCheckResourceAttrSet(sloV2ResourceName, "apm_sli_metadata.error_config.%"),
				),
			},
			{
				Config:   testAccCoralogixSLOV2APMSLI(service, `error_config = {}`, ""),
				PlanOnly: true,
			},
			{
				ResourceName:      sloV2ResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				// Switching back to a metric SLI needs the product type reset
				// explicitly, because the attribute is computed.
				Config: testAccCoralogixSLOV2WindowSLO("", `product_type = "unspecified"`),
				Check:  resource.TestCheckResourceAttr(sloV2ResourceName, "product_type", "unspecified"),
			},
			{
				Config:   testAccCoralogixSLOV2WindowSLO("", `product_type = "unspecified"`),
				PlanOnly: true,
			},
		},
	})
}

// TestAccCoralogixResourceSLOV2APMLatency covers the latency branch: the float
// trim of 500.0 to 500, a filter carrying only `values`, and the replay of
// threshold and percentile once they have been set. Both are Optional+Computed,
// so removing them keeps the last applied value; the create-time default lives
// in TestAccCoralogixResourceSLOV2APMLatencyOmitted.
func TestAccCoralogixResourceSLOV2APMLatency(t *testing.T) {
	service := os.Getenv(sloV2APMServiceEnvVar)

	withQuantile := testAccCoralogixSLOV2APMSLI(service, `
      latency_config = {
        time_window = "5_minutes"
        threshold   = 500.0
        quantile = {
          percentile = 0.95
        }
      }`, `
    filters = [{
      key    = "http.status_code"
      values = ["500", "503"]
    }]`)

	// quantile is present but empty: exactly one query type is required, while
	// percentile stays omitted so the replay is what is tested.
	thresholdsRemoved := testAccCoralogixSLOV2APMSLI(service, `
      latency_config = {
        time_window = "5_minutes"
        quantile    = {}
      }`, "")

	thresholdsReset := testAccCoralogixSLOV2APMSLI(service, `
      latency_config = {
        time_window = "5_minutes"
        threshold   = 0
        quantile = {
          percentile = 0
        }
      }`, "")

	withAverage := testAccCoralogixSLOV2APMSLI(service, `
      latency_config = {
        time_window = "5_minutes"
        threshold   = 250
        average     = {}
      }`, "")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccSLOV2APMPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccSLOV2CheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: withQuantile,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(sloV2ResourceName, "sli.apm_sli.latency_config.threshold", "500"),
					resource.TestCheckResourceAttr(sloV2ResourceName, "sli.apm_sli.latency_config.quantile.percentile", "0.95"),
					resource.TestCheckResourceAttr(sloV2ResourceName, "sli.apm_sli.filters.#", "1"),
					resource.TestCheckResourceAttr(sloV2ResourceName, "sli.apm_sli.filters.0.values.#", "2"),
					resource.TestCheckResourceAttr(sloV2ResourceName, "sli.apm_sli.filters.0.values.0", "500"),
				),
			},
			{
				Config:   withQuantile,
				PlanOnly: true,
			},
			{
				// threshold and percentile are Optional+Computed, so removing
				// them replays the last applied value rather than clearing it.
				// filters is plain Optional and does clear.
				Config: thresholdsRemoved,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(sloV2ResourceName, "sli.apm_sli.latency_config.threshold", "500"),
					resource.TestCheckResourceAttr(sloV2ResourceName, "sli.apm_sli.latency_config.quantile.percentile", "0.95"),
					resource.TestCheckNoResourceAttr(sloV2ResourceName, "sli.apm_sli.filters"),
				),
			},
			{
				Config:   thresholdsRemoved,
				PlanOnly: true,
			},
			{
				// Clearing them takes an explicit zero, as the attribute
				// descriptions state.
				Config: thresholdsReset,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(sloV2ResourceName, "sli.apm_sli.latency_config.threshold", "0"),
					resource.TestCheckResourceAttr(sloV2ResourceName, "sli.apm_sli.latency_config.quantile.percentile", "0"),
				),
			},
			{
				Config:   thresholdsReset,
				PlanOnly: true,
			},
			{
				// The other latency query type. Switching to it must drop the
				// quantile rather than send both, which the SDK refuses to encode.
				Config: withAverage,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(sloV2ResourceName, "sli.apm_sli.latency_config.threshold", "250"),
					resource.TestCheckResourceAttrSet(sloV2ResourceName, "sli.apm_sli.latency_config.average.%"),
					resource.TestCheckNoResourceAttr(sloV2ResourceName, "sli.apm_sli.latency_config.quantile"),
				),
			},
			{
				Config:   withAverage,
				PlanOnly: true,
			},
			{
				// Imported last, on the configuration whose floats are exactly
				// representable: a value such as 0.95 is not, so state written
				// from a plan and state rebuilt from a read render it
				// differently and ImportStateVerify would compare unequal.
				ResourceName:      sloV2ResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TestAccCoralogixResourceSLOV2APMLatencyOmitted covers the create path where
// threshold and percentile are absent from the start: the backend stores 0 for
// both and the next plan has to be empty. Once either has been set, removing it
// replays the prior value instead - TestAccCoralogixResourceSLOV2APMLatency
// covers that.
func TestAccCoralogixResourceSLOV2APMLatencyOmitted(t *testing.T) {
	service := os.Getenv(sloV2APMServiceEnvVar)
	config := testAccCoralogixSLOV2APMSLI(service, `
      latency_config = {
        time_window = "5_minutes"
        quantile    = {}
      }`, "")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccSLOV2APMPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccSLOV2CheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(sloV2ResourceName, "sli.apm_sli.latency_config.threshold", "0"),
					resource.TestCheckResourceAttr(sloV2ResourceName, "sli.apm_sli.latency_config.quantile.percentile", "0"),
					resource.TestCheckNoResourceAttr(sloV2ResourceName, "sli.apm_sli.latency_config.average"),
				),
			},
			{
				Config:   config,
				PlanOnly: true,
			},
			{
				ResourceName:      sloV2ResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TestAccCoralogixResourceSLOV2APMOwnershipService covers the one ownership
// dimension the backend validates against the APM Service Catalog.
func TestAccCoralogixResourceSLOV2APMOwnershipService(t *testing.T) {
	service := os.Getenv(sloV2APMServiceEnvVar)
	config := testAccCoralogixSLOV2WithOwnershipTags(fmt.Sprintf(`
    service = {
      static_values = [%q]
    }`, service))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccSLOV2APMPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccSLOV2CheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(sloV2ResourceName, "ownership_tags.service.static_values.0", service),
					resource.TestCheckNoResourceAttr(sloV2ResourceName, "ownership_tags.service.label_keys"),
				),
			},
			{
				Config:   config,
				PlanOnly: true,
			},
		},
	})
}

func testAccSLOV2CheckDestroy(s *terraform.State) error {
	// coralogix_slo_v2 lives on the plugin-framework provider, so the SDKv2
	// testAccProvider is never configured and its Meta() is nil; build a client
	// from the same environment the acceptance run already requires.
	client := testAccSLOV2Client()
	ctx := context.TODO()
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "coralogix_slo_v2" {
			continue
		}

		_, _, err := client.SlosServiceGetSlo(ctx, rs.Primary.ID).Execute()
		if err == nil {
			return fmt.Errorf("slo still exists: %v, %v", rs.Primary.ID, err)
		}
	}
	return nil
}

func testAccCoralogixSLOV2RequestBased() string {
	return `
resource "coralogix_slo_v2" "test" {
  name                        = "coralogix_slo_go_example"
  description                 = "Example SLO for Coralogix using request-based metrics"
  target_threshold_percentage = 30.0
  labels = {
    label1 = "value1"
  }
  sli = {
    request_based_metric_sli = {
      good_events = {
        query = "avg(rate(cpu_usage_seconds_total[1m])) by (instance)"
      }
      total_events = {
        query = "avg(rate(cpu_usage_seconds_total[1m])) by (instance)"
      }
    }
  }
  window = {
    slo_time_frame = "7_days"
  }
}
`
}

// testAccCoralogixSLOV2WindowSLO renders a window-based SLO, splicing extra
// attributes into window_based_metric_sli and into the resource body.
func testAccCoralogixSLOV2WindowSLO(sliAttributes, sloAttributes string) string {
	return fmt.Sprintf(`
resource "coralogix_slo_v2" "test" {
  name                        = "coralogix_slo_v2_acc_window"
  description                 = "Window based SLO used by the coralogix_slo_v2 acceptance tests"
  target_threshold_percentage = 95.0
  sli = {
    window_based_metric_sli = {
      query = {
        query = "avg(avg_over_time(request_duration_seconds[1m]))"
      }
      window              = "1_minute"
      comparison_operator = "less_than"
      threshold           = 0.25
      %s
    }
  }
  window = {
    slo_time_frame = "7_days"
  }
  %s
}
`, sliAttributes, sloAttributes)
}

func testAccCoralogixSLOV2WithOwnershipTags(dimensions string) string {
	return testAccCoralogixSLOV2WindowSLO("", fmt.Sprintf(`ownership_tags = {%s
  }`, dimensions))
}

func testAccCoralogixSLOV2OwnershipTags() string {
	return testAccCoralogixSLOV2WithOwnershipTags(`
    environment = {
      static_values = ["prod", "staging", "dev"]
    }
    team = {
      label_keys = ["owning_team", "squad"]
    }`)
}

// testAccCoralogixSLOV2APMSLI renders an APM SLO. sliConfig carries the
// error_config/latency_config branch and apmAttributes the remaining apm_sli
// attributes, so one fixture covers every APM case.
func testAccCoralogixSLOV2APMSLI(service, sliConfig, apmAttributes string) string {
	return fmt.Sprintf(`
resource "coralogix_slo_v2" "test" {
  name                        = "coralogix_slo_v2_acc_apm"
  description                 = "APM SLO used by the coralogix_slo_v2 acceptance tests"
  target_threshold_percentage = 99.0
  product_type                = "apm"
  sli = {
    apm_sli = {
      services = [%q]
      %s
      %s
    }
  }
  window = {
    slo_time_frame = "7_days"
  }
}
`, service, sliConfig, apmAttributes)
}

func testAccCoralogixSLOV2WindowBased() string {
	return `
resource "coralogix_slo_v2" "test" {
  name                        = "coralogix_window_based_slo"
  description                 = "Example SLO using window-based metrics"
  target_threshold_percentage = 95.0
  labels = {
    env     = "prod"
    service = "api"
  }
  sli = {
    window_based_metric_sli = {
      query = {
        query = "avg(avg_over_time(request_duration_seconds[1m]))"
      }
      window              = "1_minute"
      comparison_operator = "less_than"
      threshold           = 0.232
    }
  }
  window = {
    slo_time_frame = "28_days"
  }
}
`
}
