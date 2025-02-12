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

package coralogix

import (
	"context"
	"fmt"
	"testing"

	"terraform-provider-coralogix/coralogix/clientset"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	terraform2 "github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

var (
	alertsSchedulerResourceName = "coralogix_alerts_scheduler.test"
)

func TestAccCoralogixResourceResourceAlertsScheduler(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { TestAccPreCheck(t) },
		CheckDestroy:             testAccCheckAlertsSchedulerDestroy,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertsScheduler(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(alertsSchedulerResourceName, "name", "example"),
					resource.TestCheckResourceAttr(alertsSchedulerResourceName, "filter.what_expression", "source logs | filter $d.cpodId:string == '122'"),
					resource.TestCheckTypeSetElemNestedAttrs(alertsSchedulerResourceName, "filter.meta_labels.*", map[string]string{
						"key":   "key",
						"value": "value",
					}),
					resource.TestCheckResourceAttr(alertsSchedulerResourceName, "schedule.operation", "active"),
					resource.TestCheckResourceAttr(alertsSchedulerResourceName, "schedule.recurring.dynamic.repeat_every", "2"),
					resource.TestCheckTypeSetElemAttr(alertsSchedulerResourceName, "schedule.recurring.dynamic.frequency.weekly.days.*", "Sunday"),
					resource.TestCheckResourceAttr(alertsSchedulerResourceName, "schedule.recurring.dynamic.time_frame.start_time", "2021-01-04T00:00:00.000"),
					resource.TestCheckResourceAttr(alertsSchedulerResourceName, "schedule.recurring.dynamic.time_frame.duration.for_over", "2"),
					resource.TestCheckResourceAttr(alertsSchedulerResourceName, "schedule.recurring.dynamic.time_frame.duration.frequency", "hours"),
					resource.TestCheckResourceAttr(alertsSchedulerResourceName, "schedule.recurring.dynamic.time_frame.time_zone", "UTC+2"),
					resource.TestCheckResourceAttr(alertsSchedulerResourceName, "schedule.recurring.dynamic.termination_date", "2025-01-01T00:00:00.000"),
				),
			},
			{
				ResourceName:      alertsSchedulerResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccCheckAlertsSchedulerDestroy(s *terraform.State) error {
	testAccProvider = OldProvider()
	rc := terraform2.ResourceConfig{}
	testAccProvider.Configure(context.Background(), &rc)
	client := testAccProvider.Meta().(*clientset.ClientSet).AlertSchedulers()
	ctx := context.TODO()

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "coralogix_alerts_scheduler" {
			continue
		}

		req := &cxsdk.GetAlertSchedulerRuleRequest{
			AlertSchedulerRuleId: rs.Primary.ID,
		}

		resp, err := client.Get(ctx, req)
		if err == nil {
			if resp.GetAlertSchedulerRule().GetId() == rs.Primary.ID {
				return fmt.Errorf("alerts-scheduler still exists: %s", rs.Primary.ID)
			}
		}
	}

	return nil
}

func testAccCoralogixResourceAlertsScheduler() string {
	return `resource "coralogix_alerts_scheduler" "test" {
  name        = "example"
  description = "example"
  filter      = {
    what_expression = "source logs | filter $d.cpodId:string == '122'"
    meta_labels     = [
      {
        key   = "key"
        value = "value"
      }
    ]
  }
  schedule = {
    operation = "active"
    recurring = {
      dynamic = {
        repeat_every = 2
        frequency = {
          weekly = {
            days = ["Sunday"]
          }
        }
        time_frame = {
          start_time = "2021-01-04T00:00:00.000"
          duration = {
            for_over = 2
            frequency = "hours"
          }
          time_zone = "UTC+2"
        }
        termination_date = "2025-01-01T00:00:00.000"
      }
    }
  }
}
`
}
