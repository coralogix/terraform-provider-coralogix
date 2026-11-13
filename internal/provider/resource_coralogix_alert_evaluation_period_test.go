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

// TestAccCoralogixResourceAlert_analytics_evaluation_period covers evaluation_period_minutes of the
// analytics alert types: it stays null while the configuration omits it, takes a set value, goes
// back to null when the line is removed (the API resets an omitted period to its default), and
// accepts a timeframe below the default after the removal.
func TestAccCoralogixResourceAlert_analytics_evaluation_period(t *testing.T) {
	for _, arm := range []string{"analytics_immediate", "analytics_threshold"} {
		t.Run(arm, func(t *testing.T) {
			name := uuid.NewString()
			attribute := fmt.Sprintf("type_definition.%s.evaluation_period_minutes", arm)
			emptyPlan := resource.ConfigPlanChecks{PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}
			resource.ParallelTest(t, resource.TestCase{
				PreCheck:                 func() { testAccPreCheck(t) },
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				CheckDestroy:             testAccCheckAlertDestroy(t),
				Steps: []resource.TestStep{
					{
						Config:           testAccAnalyticsEvaluationPeriodConfig(name, arm, 20, nil),
						Check:            resource.TestCheckNoResourceAttr(alertResourceName, attribute),
						ConfigPlanChecks: emptyPlan,
					},
					{
						Config:           testAccAnalyticsEvaluationPeriodConfig(name, arm, 20, ptrInt(5)),
						Check:            resource.TestCheckResourceAttr(alertResourceName, attribute, "5"),
						ConfigPlanChecks: emptyPlan,
					},
					{
						Config:           testAccAnalyticsEvaluationPeriodConfig(name, arm, 1440, ptrInt(1440)),
						Check:            resource.TestCheckResourceAttr(alertResourceName, attribute, "1440"),
						ConfigPlanChecks: emptyPlan,
					},
					{
						// Removing the line resets the period, so a timeframe below the default is accepted.
						Config: testAccAnalyticsEvaluationPeriodConfig(name, arm, 3, nil),
						Check:  resource.TestCheckNoResourceAttr(alertResourceName, attribute),
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

func ptrInt(n int) *int { return &n }

func testAccAnalyticsEvaluationPeriodConfig(name, arm string, timeframe int, period *int) string {
	evaluation := ""
	if period != nil {
		evaluation = fmt.Sprintf("\n      evaluation_period_minutes = %d", *period)
	}
	body := fmt.Sprintf(`
      dataprime_query = {
        query = "source logs | filter $m.severity == ERROR"
      }
      timeframe_minutes = %d%s
`, timeframe, evaluation)
	if arm == "analytics_threshold" {
		body = fmt.Sprintf(`
      dataprime_query = {
        query = "source logs | create n from $d.x:number | filter n > 0"
      }
      target_column     = "n"
      operator          = "MORE_THAN"
      timeframe_minutes = %d%s
      rules = [{
        condition = {
          threshold = 5
        }
        override = {
          priority = "P3"
        }
      }]
`, timeframe, evaluation)
	}
	return fmt.Sprintf(`resource "coralogix_alert" "test" {
  name         = "tf-acc-%s-%s"
  priority     = "P3"
  phantom_mode = true

  type_definition = {
    %s = {%s    }
  }
}
`, arm, name, arm, body)
}
