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
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/google/uuid"

	alertscheduler "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/alert_scheduler_rule_service"
	terraform2 "github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

var (
	alertsSchedulerResourceName         = "coralogix_alerts_scheduler.test"
	importedAlertsSchedulerResourceName = "coralogix_alerts_scheduler.imported"
	alertsSchedulerTargetAlertName      = "coralogix_alert.scheduler_target"
)

func TestAccCoralogixResourceResourceAlertsScheduler(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
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

func TestAccCoralogixResourceResourceAlertsSchedulerAllAlerts(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		CheckDestroy:             testAccCheckAlertsSchedulerDestroy,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertsSchedulerAllAlerts(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(alertsSchedulerResourceName, "name", "example"),
					resource.TestCheckResourceAttr(alertsSchedulerResourceName, "filter.what_expression", "source logs | filter true"),
					resource.TestCheckNoResourceAttr(alertsSchedulerResourceName, "filter.alerts_unique_ids"),
					resource.TestCheckNoResourceAttr(alertsSchedulerResourceName, "filter.meta_labels"),
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

func TestAccCoralogixResourceAlertsSchedulerImportAllAlertsNoDrift(t *testing.T) {
	name := fmt.Sprintf("alerts-scheduler-all-alerts-%s", uuid.NewString())
	config := testAccCoralogixResourceAlertsSchedulerImportedAllAlerts(name)
	var schedulerID string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		CheckDestroy:             testAccCheckAlertsSchedulerDestroy,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				PreConfig: func() {
					schedulerID = testAccCreateAlertsScheduler(t, testAccBackendAlertsSchedulerAllAlerts(name))
				},
				Config:             config,
				ResourceName:       importedAlertsSchedulerResourceName,
				ImportState:        true,
				ImportStatePersist: true,
				ImportStateIdFunc: func(*terraform.State) (string, error) {
					if schedulerID == "" {
						return "", fmt.Errorf("missing imported alert scheduler ID")
					}
					return schedulerID, nil
				},
				ImportStateCheck: testAccCheckImportedAlertsScheduler(map[string]string{
					"name":                             name,
					"filter.what_expression":           "source logs | filter true",
					"schedule.operation":               "mute",
					"schedule.recurring.always_active": "true",
				}, []string{
					"filter.alerts_unique_ids.#",
					"filter.meta_labels.#",
				}),
			},
			{
				Config:   config,
				PlanOnly: true,
			},
		},
	})
}

func TestAccCoralogixResourceAlertsSchedulerImportDirectAlertIDNoDrift(t *testing.T) {
	alertName := fmt.Sprintf("alerts-scheduler-target-%s", uuid.NewString())
	schedulerName := fmt.Sprintf("alerts-scheduler-direct-id-%s", uuid.NewString())
	config := testAccCoralogixResourceAlertsSchedulerImportedDirectAlertID(alertName, schedulerName)
	var alertID string
	var schedulerID string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		CheckDestroy:             testAccCheckAlertAndAlertsSchedulerDestroy,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixAlertsSchedulerTargetAlert(alertName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(alertsSchedulerTargetAlertName, "name", alertName),
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources[alertsSchedulerTargetAlertName]
						if !ok {
							return fmt.Errorf("missing target alert resource")
						}
						alertID = rs.Primary.ID
						return nil
					},
				),
			},
			{
				PreConfig: func() {
					if alertID == "" {
						t.Fatal("missing target alert ID")
					}
					schedulerID = testAccCreateAlertsScheduler(t, testAccBackendAlertsSchedulerDirectAlertID(schedulerName, alertID))
				},
				Config:             config,
				ResourceName:       importedAlertsSchedulerResourceName,
				ImportState:        true,
				ImportStatePersist: true,
				ImportStateIdFunc: func(*terraform.State) (string, error) {
					if schedulerID == "" {
						return "", fmt.Errorf("missing imported alert scheduler ID")
					}
					return schedulerID, nil
				},
				ImportStateCheck: testAccCheckImportedAlertsScheduler(map[string]string{
					"name":                             schedulerName,
					"filter.what_expression":           "source logs | filter true",
					"filter.alerts_unique_ids.#":       "1",
					"schedule.operation":               "mute",
					"schedule.recurring.always_active": "true",
				}, []string{
					"filter.meta_labels.#",
				}),
			},
			{
				Config:   config,
				PlanOnly: true,
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

		resp, _, err := client.
			AlertSchedulerRuleServiceGetAlertSchedulerRule(ctx, rs.Primary.ID).
			Execute()
		if err == nil && resp != nil {
			if resp.AlertSchedulerRule.Id != nil && *resp.AlertSchedulerRule.Id == rs.Primary.ID {
				return fmt.Errorf("alerts-scheduler still exists: %s", rs.Primary.ID)
			}
		}
	}

	return nil
}

func testAccCheckAlertAndAlertsSchedulerDestroy(s *terraform.State) error {
	if err := testAccCheckAlertsSchedulerDestroy(s); err != nil {
		return err
	}
	return testAccCheckAlertDestroyState(s)
}

func testAccCreateAlertsScheduler(t *testing.T, scheduler alertscheduler.AlertSchedulerRule) string {
	t.Helper()

	client := testAccAlertsSchedulerClient(t)
	createRequest := alertscheduler.CreateAlertSchedulerRuleRequestDataStructure{
		AlertSchedulerRule: scheduler,
	}
	createResp, _, err := client.
		AlertSchedulerRuleServiceCreateAlertSchedulerRule(context.Background()).
		CreateAlertSchedulerRuleRequestDataStructure(createRequest).
		Execute()
	if err != nil {
		t.Fatalf("creating out-of-band alert scheduler: %s", err)
	}

	schedulerID := createResp.AlertSchedulerRule.GetUniqueIdentifier()
	t.Cleanup(func() {
		_, _, _ = client.AlertSchedulerRuleServiceDeleteAlertSchedulerRule(context.Background(), schedulerID).Execute()
	})
	return schedulerID
}

func testAccAlertsSchedulerClient(t *testing.T) *alertscheduler.AlertSchedulerRuleServiceAPIService {
	t.Helper()

	testAccProvider = OldProvider()
	rc := terraform2.ResourceConfig{}
	if diags := testAccProvider.Configure(context.Background(), &rc); diags.HasError() {
		t.Fatalf("configuring provider for alert scheduler client: %v", diags)
	}
	return testAccProvider.Meta().(*clientset.ClientSet).AlertSchedulers()
}

func testAccBackendAlertsSchedulerAllAlerts(name string) alertscheduler.AlertSchedulerRule {
	return testAccBackendAlertsScheduler(name, nil)
}

func testAccBackendAlertsSchedulerDirectAlertID(name string, alertID string) alertscheduler.AlertSchedulerRule {
	return testAccBackendAlertsScheduler(name, []string{alertID})
}

func testAccBackendAlertsScheduler(name string, alertIDs []string) alertscheduler.AlertSchedulerRule {
	operation := alertscheduler.SCHEDULEOPERATION_SCHEDULE_OPERATION_MUTE
	return alertscheduler.AlertSchedulerRule{
		Name:        alertscheduler.PtrString(name),
		Description: alertscheduler.PtrString("Imported parity scheduler"),
		Enabled:     alertscheduler.PtrBool(true),
		Filter: &alertscheduler.AlertSchedulerRuleProtobufV1Filter{
			WhatExpression: alertscheduler.PtrString("source logs | filter true"),
			AlertUniqueIds: &alertscheduler.AlertUniqueIds{Value: alertIDs},
		},
		Schedule: &alertscheduler.Schedule{
			ScheduleOperation: &operation,
			Recurring: &alertscheduler.Recurring{
				AlwaysActive: map[string]interface{}{},
			},
		},
	}
}

func testAccCheckImportedAlertsScheduler(expected map[string]string, absent []string) resource.ImportStateCheckFunc {
	return func(states []*terraform.InstanceState) error {
		var schedulerState *terraform.InstanceState
		for _, state := range states {
			if _, ok := state.Attributes["filter.what_expression"]; ok {
				schedulerState = state
				break
			}
		}
		if schedulerState == nil {
			return fmt.Errorf("expected imported alert scheduler state, got %d state entries", len(states))
		}
		attrs := schedulerState.Attributes
		for key, want := range expected {
			if got := attrs[key]; got != want {
				return fmt.Errorf("imported %s = %q, want %q", key, got, want)
			}
		}
		for _, key := range absent {
			if got, ok := attrs[key]; ok && got != "" && got != "0" {
				return fmt.Errorf("imported %s = %q, want absent or empty", key, got)
			}
		}
		return nil
	}
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

func testAccCoralogixResourceAlertsSchedulerAllAlerts() string {
	return `resource "coralogix_alerts_scheduler" "test" {
  name        = "example"
  description = "example"
  filter      = {
    what_expression = "source logs | filter true"
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

func testAccCoralogixResourceAlertsSchedulerImportedAllAlerts(name string) string {
	return fmt.Sprintf(`resource "coralogix_alerts_scheduler" "imported" {
  name        = %[1]q
  description = "Imported parity scheduler"
  enabled     = true
  filter = {
    what_expression = "source logs | filter true"
  }
  schedule = {
    operation = "mute"
    recurring = {
      always_active = true
    }
  }
}
`, name)
}

func testAccCoralogixResourceAlertsSchedulerImportedDirectAlertID(alertName string, schedulerName string) string {
	return testAccCoralogixAlertsSchedulerTargetAlert(alertName) + fmt.Sprintf(`
resource "coralogix_alerts_scheduler" "imported" {
  name        = %[1]q
  description = "Imported parity scheduler"
  enabled     = true
  filter = {
    what_expression   = "source logs | filter true"
    alerts_unique_ids = [coralogix_alert.scheduler_target.id]
  }
  schedule = {
    operation = "mute"
    recurring = {
      always_active = true
    }
  }
}
`, schedulerName)
}

func testAccCoralogixAlertsSchedulerTargetAlert(name string) string {
	return fmt.Sprintf(`resource "coralogix_alert" "scheduler_target" {
  name         = %[1]q
  description  = "Alert scheduler parity target"
  priority     = "P2"
  phantom_mode = true

  labels = {
    alert_type        = "security"
    security_severity = "high"
  }

  incidents_settings = {
    notify_on = "Triggered and Resolved"
    retriggering_period = {
      minutes = 10
    }
  }

  schedule = {
    active_on = {
      days_of_week = ["Wednesday", "Thursday"]
      start_time   = "08:30"
      end_time     = "20:30"
      utc_offset   = "+0300"
    }
  }

  type_definition = {
    logs_immediate = {
      logs_filter = {
        simple_filter = {
          lucene_query = "message:\"error\""
        }
      }
    }
  }
}
`, name)
}

func TestAccCoralogixResourceResourceAlertsSchedulerAlwaysActive(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		CheckDestroy:             testAccCheckAlertsSchedulerDestroy,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertsSchedulerAlwaysActive(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(alertsSchedulerResourceName, "name", "permanent-suppression"),
					resource.TestCheckResourceAttr(alertsSchedulerResourceName, "filter.what_expression", "source logs | filter true"),
					resource.TestCheckResourceAttr(alertsSchedulerResourceName, "schedule.operation", "mute"),
					resource.TestCheckResourceAttr(alertsSchedulerResourceName, "schedule.recurring.always_active", "true"),
					resource.TestCheckNoResourceAttr(alertsSchedulerResourceName, "schedule.recurring.dynamic"),
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

func testAccCoralogixResourceAlertsSchedulerAlwaysActive() string {
	return `resource "coralogix_alerts_scheduler" "test" {
  name        = "permanent-suppression"
  description = "Permanent suppression rule - always active"
  filter = {
    what_expression = "source logs | filter true"
  }
  schedule = {
    operation = "mute"
    recurring = {
      always_active = true
    }
  }
}
`
}
