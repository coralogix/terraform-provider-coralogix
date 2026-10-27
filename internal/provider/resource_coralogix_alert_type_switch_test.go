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
	"testing"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

const (
	alertUpgradeSource  = "registry.terraform.io/coralogix/coralogix"
	alertUpgradeVersion = "= 3.21.0"
)

// TestAccCoralogixResourceAlert_type_switch_to_threshold_omitting_optional_blocks changes an
// existing alert in place from logs_ratio_threshold to a threshold type, leaving out the
// optional blocks the API fills in on its own. Those blocks are null in the prior state
// because the previous type_definition had no such block, and the read flattens the API's
// answer to a non-null object, so a plan that carries the prior null over fails the apply
// with "Provider produced inconsistent result after apply". Creating either alert from
// scratch always worked.
func TestAccCoralogixResourceAlert_type_switch_to_threshold_omitting_optional_blocks(t *testing.T) {
	cases := []struct {
		name    string
		stepTwo func(string) string
		unset   []string
	}{
		{
			name:    "metric_threshold",
			stepTwo: testAccCoralogixResourceAlertMetricThresholdMinimal,
			unset: []string{
				"type_definition.metric_threshold.no_data_policy.state",
				"type_definition.metric_threshold.no_data_policy.auto_retire_seconds",
			},
		},
		{
			name:    "logs_threshold",
			stepTwo: testAccCoralogixResourceAlertLogsThresholdMinimal,
			unset: []string{
				"type_definition.logs_threshold.no_data_policy.state",
				"type_definition.logs_threshold.no_data_policy.auto_retire_seconds",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			name := uuid.NewString()
			checks := make([]resource.TestCheckFunc, 0, len(tc.unset))
			for _, attribute := range tc.unset {
				checks = append(checks, resource.TestCheckNoResourceAttr(alertResourceName, attribute))
			}
			resource.ParallelTest(t, resource.TestCase{
				PreCheck:                 func() { testAccPreCheck(t) },
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				CheckDestroy:             testAccCheckAlertDestroy(t),
				Steps: []resource.TestStep{
					{
						Config: testAccCoralogixResourceAlertLogsRatioThresholdMinimal(name),
						ConfigPlanChecks: resource.ConfigPlanChecks{
							PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
						},
					},
					{
						Config: tc.stepTwo(name),
						Check:  resource.ComposeAggregateTestCheckFunc(checks...),
						ConfigPlanChecks: resource.ConfigPlanChecks{
							PreApply:             []plancheck.PlanCheck{plancheck.ExpectResourceAction(alertResourceName, plancheck.ResourceActionUpdate)},
							PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
						},
					},
					{
						ResourceName:      alertResourceName,
						ImportState:       true,
						ImportStateVerify: true,
					},
				},
			})
		})
	}
}

// TestAccCoralogixResourceAlert_upgrade_keeps_empty_plan creates an alert with the released
// provider and plans the same configuration with this build. Alerts created before the plan
// modifier change hold the objects the read wrote in state, and the plan has to keep
// returning them, so the upgrade plans no change.
func TestAccCoralogixResourceAlert_upgrade_keeps_empty_plan(t *testing.T) {
	name := uuid.NewString()
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		CheckDestroy: testAccCheckAlertDestroy(t),
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertMetricThresholdMinimal(name),
				ExternalProviders: map[string]resource.ExternalProvider{
					"coralogix": {Source: alertUpgradeSource, VersionConstraint: alertUpgradeVersion},
				},
			},
			{
				Config:                   testAccCoralogixResourceAlertMetricThresholdMinimal(name),
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr(alertResourceName, "type_definition.metric_threshold.no_data_policy.state"),
					resource.TestCheckNoResourceAttr(alertResourceName, "type_definition.metric_threshold.no_data_policy.auto_retire_seconds"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply:             []plancheck.PlanCheck{plancheck.ExpectResourceAction(alertResourceName, plancheck.ResourceActionNoop)},
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

func testAccCoralogixResourceAlertLogsRatioThresholdMinimal(name string) string {
	return fmt.Sprintf(`resource "coralogix_alert" "test" {
  name     = "%s"
  priority = "P2"
  enabled  = true

  type_definition = {
    logs_ratio_threshold = {
      numerator = {
        simple_filter = {
          lucene_query = "message:\"request failed\""
        }
      }
      numerator_alias = "failed"
      denominator = {
        simple_filter = {
          lucene_query = "message:\"request handled\""
        }
      }
      denominator_alias = "handled"
      rules = [{
        condition = {
          condition_type = "MORE_THAN"
          threshold      = 0.1
          time_window    = "1_HOUR"
        }
        override = {
          priority = "P2"
        }
      }]
    }
  }
}
`, name)
}

func testAccCoralogixResourceAlertMetricThresholdMinimal(name string) string {
	return fmt.Sprintf(`resource "coralogix_alert" "test" {
  name     = "%s"
  priority = "P2"
  enabled  = true

  type_definition = {
    metric_threshold = {
      metric_filter = {
        promql = "sum(rate(requests_failed_total[1h])) / sum(rate(requests_total[1h]))"
      }
      missing_values = {
        replace_with_zero = true
      }
      rules = [{
        condition = {
          condition_type = "MORE_THAN"
          threshold      = 0.1
          of_the_last    = "10_MINUTES"
          for_over_pct   = 0
        }
        override = {
          priority = "P2"
        }
      }]
    }
  }
}
`, name)
}

func testAccCoralogixResourceAlertLogsThresholdMinimal(name string) string {
	return fmt.Sprintf(`resource "coralogix_alert" "test" {
  name     = "%s"
  priority = "P2"
  enabled  = true

  type_definition = {
    logs_threshold = {
      rules = [{
        condition = {
          condition_type = "MORE_THAN"
          threshold      = 2
          time_window    = "10_MINUTES"
        }
        override = {
          priority = "P2"
        }
      }]
    }
  }
}
`, name)
}
