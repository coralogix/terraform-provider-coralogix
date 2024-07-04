package coralogix

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"terraform-provider-coralogix/coralogix/clientset"
	alertsv3 "terraform-provider-coralogix/coralogix/clientset/grpc/alerts/v3"
)

var alertResourceName = "coralogix_alert.test"

func TestAccCoralogixResourceAlert_logs_immediate(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAlertDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertLogsImmediate(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(alertResourceName, "name", "logs immediate alert"),
					resource.TestCheckResourceAttr(alertResourceName, "description", "Example of logs immediate alert from terraform"),
					resource.TestCheckResourceAttr(alertResourceName, "priority", "P1"),
					resource.TestCheckResourceAttr(alertResourceName, "labels.alert_type", "security"),
					resource.TestCheckResourceAttr(alertResourceName, "labels.security_severity", "high"),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "notification_group.simple_target_settings.*",
						map[string]string{
							"recipients.#": "1",
							"recipients.*": "example@coralogix.com",
						}),
					resource.TestCheckResourceAttr(alertResourceName, "incidents_settings.notify_on", "Triggered and Resolved"),
					resource.TestCheckResourceAttr(alertResourceName, "incidents_settings.retriggering_period.minutes", "1"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.days_of_week.#", "2"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.days_of_week.0", "Wednesday"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.days_of_week.1", "Thursday"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.start_time.hours", "8"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.start_time.minutes", "30"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.end_time.hours", "20"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.end_time.minutes", "30"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_immediate.logs_filter.lucene_filter.lucene_query", "message:\"error\""),
				),
			},
			{
				ResourceName: alertResourceName,
				ImportState:  true,
			},
			{
				Config: testAccCoralogixResourceAlertLogsImmediateUpdated(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(alertResourceName, "name", "logs immediate alert updated"),
					resource.TestCheckResourceAttr(alertResourceName, "description", "Example of logs immediate alert from terraform updated"),
					resource.TestCheckResourceAttr(alertResourceName, "priority", "P2"),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "notification_group.advanced_target_settings.*",
						map[string]string{
							"retriggering_period.minutes": "10",
							"notify_on":                   "Triggered Only",
							"recipients.#":                "1",
							"recipients.*":                "example@coralogix.com",
						},
					),
					resource.TestCheckResourceAttr(alertResourceName, "incidents_settings.notify_on", "Triggered and Resolved"),
					resource.TestCheckResourceAttr(alertResourceName, "incidents_settings.retriggering_period.minutes", "10"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.days_of_week.#", "2"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.days_of_week.0", "Wednesday"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.days_of_week.1", "Thursday"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.start_time.hours", "9"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.start_time.minutes", "30"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.end_time.hours", "21"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.end_time.minutes", "30"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_immediate.logs_filter.lucene_filter.lucene_query", "message:\"error\""),
				),
			},
		},
	})
}

func TestAccCoralogixResourceAlert_logs_more_than(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAlertDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertLogsMoreThan(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(alertResourceName, "name", "logs-more-than alert example"),
					resource.TestCheckResourceAttr(alertResourceName, "description", "Example of logs-more-than alert example from terraform"),
					resource.TestCheckResourceAttr(alertResourceName, "priority", "P2"),
					resource.TestCheckResourceAttr(alertResourceName, "labels.alert_type", "security"),
					resource.TestCheckResourceAttr(alertResourceName, "labels.security_severity", "high"),
					resource.TestCheckResourceAttr(alertResourceName, "notification_group.0.simple_target_settings.0.integration_id", "17730"),
					resource.TestCheckResourceAttr(alertResourceName, "notification_group.1.simple_target_settings.0.retriggering_period.minutes", "1"),
					resource.TestCheckResourceAttr(alertResourceName, "notification_group.1.simple_target_settings.0.notify_on", "Triggered and Resolved"),
					resource.TestCheckResourceAttr(alertResourceName, "notification_group.1.simple_target_settings.0.recipients.#", "1"),
					resource.TestCheckResourceAttr(alertResourceName, "notification_group.1.simple_target_settings.0.recipients.*", "example@coralogix.com"),
					resource.TestCheckResourceAttr(alertResourceName, "incidents_settings.notify_on", "Triggered and Resolved"),
					resource.TestCheckResourceAttr(alertResourceName, "incidents_settings.retriggering_period.minutes", "1"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.days_of_week.#", "2"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.days_of_week.0", "Wednesday"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.days_of_week.1", "Thursday"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.start_time.hours", "8"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.start_time.minutes", "30"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.end_time.hours", "20"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.end_time.minutes", "30"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_more_than.threshold", "2"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_more_than.time_window.specific_value", "10_MINUTES"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_more_than.evaluation_window", "Dynamic"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_more_than.logs_filter.lucene_filter.lucene_query", "message:\"error\""),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.logs_more_than.logs_filter.lucene_filter.label_filters.application_name.*",
						map[string]string{
							"operation": "OR",
							"value":     "nginx",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.logs_more_than.logs_filter.lucene_filter.label_filters.subsystem_name.*",
						map[string]string{
							"operation": "OR",
							"value":     "subsystem-name",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.logs_more_than.logs_filter.lucene_filter.label_filters.severity.*",
						map[string]string{
							"operation": "OR",
							"value":     "Warning",
						},
					),
				),
			},
			{
				ResourceName: alertResourceName,
				ImportState:  true,
			},
			{
				Config: testAccCoralogixResourceAlertLogsMoreThanUpdated(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(alertResourceName, "name", "logs-more_-than alert example updated"),
					resource.TestCheckResourceAttr(alertResourceName, "description", "Example of standard alert from terraform updated"),
					resource.TestCheckResourceAttr(alertResourceName, "priority", "P3"),
					resource.TestCheckResourceAttr(alertResourceName, "labels.alert_type", "security"),
					resource.TestCheckResourceAttr(alertResourceName, "labels.security_severity", "low"),
					resource.TestCheckResourceAttr(alertResourceName, "notification_group.0.simple_target_settings.0.integration_id", "17730"),
					resource.TestCheckResourceAttr(alertResourceName, "incidents_settings.notify_on", "Triggered Only"),
					resource.TestCheckResourceAttr(alertResourceName, "incidents_settings.retriggering_period.minutes", "10"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.days_of_week.#", "2"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.days_of_week.0", "Monday"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.days_of_week.1", "Thursday"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.start_time.hours", "8"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.start_time.minutes", "30"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.end_time.hours", "20"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.end_time.minutes", "30"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_more_than.threshold", "20"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_more_than.time_window.specific_value", "2_HOURS"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_more_than.evaluation_window", "Rolling"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_more_than.logs_filter.lucene_filter.lucene_query", "message:\"error\""),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_more_than.logs_filter.lucene_filter.label_filters.application_name.#", "2"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_more_than.logs_filter.lucene_filter.label_filters.application_name.0.operation", "OR"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_more_than.logs_filter.lucene_filter.label_filters.application_name.0.value", "nginx"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_more_than.logs_filter.lucene_filter.label_filters.application_name.1.operation", "NOT"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_more_than.logs_filter.lucene_filter.label_filters.application_name.1.value", "application_namee"),
				),
			},
		},
	})
}

//
//func TestAccCoralogixResourceAlert_logs_less_than(t *testing.T) {
//	resource.Test(t, resource.TestCase{
//		PreCheck:          func() { testAccPreCheck(t) },
//		ProviderFactories: testAccProviderFactories,
//		CheckDestroy:      testAccCheckAlertDestroy,
//		Steps: []resource.TestStep{
//			{
//				Config: testAccCoralogixResourceAlertStandard(&alert),
//				Check:  resource.ComposeAggregateTestCheckFunc(checks...),
//			},
//			{
//				ResourceName: alertResourceName,
//				ImportState:  true,
//			},
//			{
//				Config: testAccCoralogixResourceAlertStandard(&updatedAlert),
//				Check:  resource.ComposeAggregateTestCheckFunc(updatedAlertChecks...),
//			},
//		},
//	})
//}
//
//func TestAccCoralogixResourceAlert_logs_more_than_usual(t *testing.T) {
//	resource.Test(t, resource.TestCase{
//		PreCheck:          func() { testAccPreCheck(t) },
//		ProviderFactories: testAccProviderFactories,
//		CheckDestroy:      testAccCheckAlertDestroy,
//		Steps: []resource.TestStep{
//			{
//				Config: testAccCoralogixResourceAlertStandard(&alert),
//				Check:  resource.ComposeAggregateTestCheckFunc(checks...),
//			},
//			{
//				ResourceName: alertResourceName,
//				ImportState:  true,
//			},
//			{
//				Config: testAccCoralogixResourceAlertStandard(&updatedAlert),
//				Check:  resource.ComposeAggregateTestCheckFunc(updatedAlertChecks...),
//			},
//		},
//	})
//}
//
//func TestAccCoralogixResourceAlert_logs_ratio_more_than(t *testing.T) {
//	resource.Test(t, resource.TestCase{
//		PreCheck:          func() { testAccPreCheck(t) },
//		ProviderFactories: testAccProviderFactories,
//		CheckDestroy:      testAccCheckAlertDestroy,
//		Steps: []resource.TestStep{
//			{
//				Config: testAccCoralogixResourceAlertStandard(&alert),
//				Check:  resource.ComposeAggregateTestCheckFunc(checks...),
//			},
//			{
//				ResourceName: alertResourceName,
//				ImportState:  true,
//			},
//			{
//				Config: testAccCoralogixResourceAlertStandard(&updatedAlert),
//				Check:  resource.ComposeAggregateTestCheckFunc(updatedAlertChecks...),
//			},
//		},
//	})
//}
//
//func TestAccCoralogixResourceAlert_logs_ratio_less_than(t *testing.T) {
//	resource.Test(t, resource.TestCase{
//		PreCheck:          func() { testAccPreCheck(t) },
//		ProviderFactories: testAccProviderFactories,
//		CheckDestroy:      testAccCheckAlertDestroy,
//		Steps: []resource.TestStep{
//			{
//				Config: testAccCoralogixResourceAlertStandard(&alert),
//				Check:  resource.ComposeAggregateTestCheckFunc(checks...),
//			},
//			{
//				ResourceName: alertResourceName,
//				ImportState:  true,
//			},
//			{
//				Config: testAccCoralogixResourceAlertStandard(&updatedAlert),
//				Check:  resource.ComposeAggregateTestCheckFunc(updatedAlertChecks...),
//			},
//		},
//	})
//}
//
//func TestAccCoralogixResourceAlert_logs_new_value(t *testing.T) {
//	resource.Test(t, resource.TestCase{
//		PreCheck:          func() { testAccPreCheck(t) },
//		ProviderFactories: testAccProviderFactories,
//		CheckDestroy:      testAccCheckAlertDestroy,
//		Steps: []resource.TestStep{
//			{
//				Config: testAccCoralogixResourceAlertStandard(&alert),
//				Check:  resource.ComposeAggregateTestCheckFunc(checks...),
//			},
//			{
//				ResourceName: alertResourceName,
//				ImportState:  true,
//			},
//			{
//				Config: testAccCoralogixResourceAlertStandard(&updatedAlert),
//				Check:  resource.ComposeAggregateTestCheckFunc(updatedAlertChecks...),
//			},
//		},
//	})
//}
//
//func TestAccCoralogixResourceAlert_logs_unique_count(t *testing.T) {
//	resource.Test(t, resource.TestCase{
//		PreCheck:          func() { testAccPreCheck(t) },
//		ProviderFactories: testAccProviderFactories,
//		CheckDestroy:      testAccCheckAlertDestroy,
//		Steps: []resource.TestStep{
//			{
//				Config: testAccCoralogixResourceAlertStandard(&alert),
//				Check:  resource.ComposeAggregateTestCheckFunc(checks...),
//			},
//			{
//				ResourceName: alertResourceName,
//				ImportState:  true,
//			},
//			{
//				Config: testAccCoralogixResourceAlertStandard(&updatedAlert),
//				Check:  resource.ComposeAggregateTestCheckFunc(updatedAlertChecks...),
//			},
//		},
//	})
//}
//
//func TestAccCoralogixResourceAlert_logs_time_relative_more_than(t *testing.T) {
//	resource.Test(t, resource.TestCase{
//		PreCheck:          func() { testAccPreCheck(t) },
//		ProviderFactories: testAccProviderFactories,
//		CheckDestroy:      testAccCheckAlertDestroy,
//		Steps: []resource.TestStep{
//			{
//				Config: testAccCoralogixResourceAlertStandard(&alert),
//				Check:  resource.ComposeAggregateTestCheckFunc(checks...),
//			},
//			{
//				ResourceName: alertResourceName,
//				ImportState:  true,
//			},
//			{
//				Config: testAccCoralogixResourceAlertStandard(&updatedAlert),
//				Check:  resource.ComposeAggregateTestCheckFunc(updatedAlertChecks...),
//			},
//		},
//	})
//}
//
//func TestAccCoralogixResourceAlert_logs_time_relative_less_than(t *testing.T) {
//	resource.Test(t, resource.TestCase{
//		PreCheck:          func() { testAccPreCheck(t) },
//		ProviderFactories: testAccProviderFactories,
//		CheckDestroy:      testAccCheckAlertDestroy,
//		Steps: []resource.TestStep{
//			{
//				Config: testAccCoralogixResourceAlertStandard(&alert),
//				Check:  resource.ComposeAggregateTestCheckFunc(checks...),
//			},
//			{
//				ResourceName: alertResourceName,
//				ImportState:  true,
//			},
//			{
//				Config: testAccCoralogixResourceAlertStandard(&updatedAlert),
//				Check:  resource.ComposeAggregateTestCheckFunc(updatedAlertChecks...),
//			},
//		},
//	})
//}
//
//func TestAccCoralogixResourceAlert_metric_more_than(t *testing.T) {
//	resource.Test(t, resource.TestCase{
//		PreCheck:          func() { testAccPreCheck(t) },
//		ProviderFactories: testAccProviderFactories,
//		CheckDestroy:      testAccCheckAlertDestroy,
//		Steps: []resource.TestStep{
//			{
//				Config: testAccCoralogixResourceAlertStandard(&alert),
//				Check:  resource.ComposeAggregateTestCheckFunc(checks...),
//			},
//			{
//				ResourceName: alertResourceName,
//				ImportState:  true,
//			},
//			{
//				Config: testAccCoralogixResourceAlertStandard(&updatedAlert),
//				Check:  resource.ComposeAggregateTestCheckFunc(updatedAlertChecks...),
//			},
//		},
//	})
//}
//
//func TestAccCoralogixResourceAlert_metric_less_than(t *testing.T) {
//	resource.Test(t, resource.TestCase{
//		PreCheck:          func() { testAccPreCheck(t) },
//		ProviderFactories: testAccProviderFactories,
//		CheckDestroy:      testAccCheckAlertDestroy,
//		Steps: []resource.TestStep{
//			{
//				Config: testAccCoralogixResourceAlertStandard(&alert),
//				Check:  resource.ComposeAggregateTestCheckFunc(checks...),
//			},
//			{
//				ResourceName: alertResourceName,
//				ImportState:  true,
//			},
//			{
//				Config: testAccCoralogixResourceAlertStandard(&updatedAlert),
//				Check:  resource.ComposeAggregateTestCheckFunc(updatedAlertChecks...),
//			},
//		},
//	})
//}
//
//func TestAccCoralogixResourceAlert_metric_less_than_usual(t *testing.T) {
//	resource.Test(t, resource.TestCase{
//		PreCheck:          func() { testAccPreCheck(t) },
//		ProviderFactories: testAccProviderFactories,
//		CheckDestroy:      testAccCheckAlertDestroy,
//		Steps: []resource.TestStep{
//			{
//				Config: testAccCoralogixResourceAlertStandard(&alert),
//				Check:  resource.ComposeAggregateTestCheckFunc(checks...),
//			},
//			{
//				ResourceName: alertResourceName,
//				ImportState:  true,
//			},
//			{
//				Config: testAccCoralogixResourceAlertStandard(&updatedAlert),
//				Check:  resource.ComposeAggregateTestCheckFunc(updatedAlertChecks...),
//			},
//		},
//	})
//}
//
//func TestAccCoralogixResourceAlert_metric_more_than_usual(t *testing.T) {
//	resource.Test(t, resource.TestCase{
//		PreCheck:          func() { testAccPreCheck(t) },
//		ProviderFactories: testAccProviderFactories,
//		CheckDestroy:      testAccCheckAlertDestroy,
//		Steps: []resource.TestStep{
//			{
//				Config: testAccCoralogixResourceAlertStandard(&alert),
//				Check:  resource.ComposeAggregateTestCheckFunc(checks...),
//			},
//			{
//				ResourceName: alertResourceName,
//				ImportState:  true,
//			},
//			{
//				Config: testAccCoralogixResourceAlertStandard(&updatedAlert),
//				Check:  resource.ComposeAggregateTestCheckFunc(updatedAlertChecks...),
//			},
//		},
//	})
//}
//
//func TestAccCoralogixResourceAlert_metric_more_than_or_equals(t *testing.T) {
//	resource.Test(t, resource.TestCase{
//		PreCheck:          func() { testAccPreCheck(t) },
//		ProviderFactories: testAccProviderFactories,
//		CheckDestroy:      testAccCheckAlertDestroy,
//		Steps: []resource.TestStep{
//			{
//				Config: testAccCoralogixResourceAlertStandard(&alert),
//				Check:  resource.ComposeAggregateTestCheckFunc(checks...),
//			},
//			{
//				ResourceName: alertResourceName,
//				ImportState:  true,
//			},
//			{
//				Config: testAccCoralogixResourceAlertStandard(&updatedAlert),
//				Check:  resource.ComposeAggregateTestCheckFunc(updatedAlertChecks...),
//			},
//		},
//	})
//}
//
//func TestAccCoralogixResourceAlert_metric_less_than_or_equals(t *testing.T) {
//	resource.Test(t, resource.TestCase{
//		PreCheck:          func() { testAccPreCheck(t) },
//		ProviderFactories: testAccProviderFactories,
//		CheckDestroy:      testAccCheckAlertDestroy,
//		Steps: []resource.TestStep{
//			{
//				Config: testAccCoralogixResourceAlertStandard(&alert),
//				Check:  resource.ComposeAggregateTestCheckFunc(checks...),
//			},
//			{
//				ResourceName: alertResourceName,
//				ImportState:  true,
//			},
//			{
//				Config: testAccCoralogixResourceAlertStandard(&updatedAlert),
//				Check:  resource.ComposeAggregateTestCheckFunc(updatedAlertChecks...),
//			},
//		},
//	})
//}
//
//func TestAccCoralogixResourceAlert_tracing_immediate(t *testing.T) {
//	resource.Test(t, resource.TestCase{
//		PreCheck:          func() { testAccPreCheck(t) },
//		ProviderFactories: testAccProviderFactories,
//		CheckDestroy:      testAccCheckAlertDestroy,
//		Steps: []resource.TestStep{
//			{
//				Config: testAccCoralogixResourceAlertStandard(&alert),
//				Check:  resource.ComposeAggregateTestCheckFunc(checks...),
//			},
//			{
//				ResourceName: alertResourceName,
//				ImportState:  true,
//			},
//			{
//				Config: testAccCoralogixResourceAlertStandard(&updatedAlert),
//				Check:  resource.ComposeAggregateTestCheckFunc(updatedAlertChecks...),
//			},
//		},
//	})
//}
//
//func TestAccCoralogixResourceAlert_tracing_more_than(t *testing.T) {
//	resource.Test(t, resource.TestCase{
//		PreCheck:          func() { testAccPreCheck(t) },
//		ProviderFactories: testAccProviderFactories,
//		CheckDestroy:      testAccCheckAlertDestroy,
//		Steps: []resource.TestStep{
//			{
//				Config: testAccCoralogixResourceAlertStandard(&alert),
//				Check:  resource.ComposeAggregateTestCheckFunc(checks...),
//			},
//			{
//				ResourceName: alertResourceName,
//				ImportState:  true,
//			},
//			{
//				Config: testAccCoralogixResourceAlertStandard(&updatedAlert),
//				Check:  resource.ComposeAggregateTestCheckFunc(updatedAlertChecks...),
//			},
//		},
//	})
//}
//
//func TestAccCoralogixResourceAlert_flow(t *testing.T) {
//	resource.Test(t, resource.TestCase{
//		PreCheck:          func() { testAccPreCheck(t) },
//		ProviderFactories: testAccProviderFactories,
//		CheckDestroy:      testAccCheckAlertDestroy,
//		Steps: []resource.TestStep{
//			{
//				Config: testAccCoralogixResourceAlertStandard(&alert),
//				Check:  resource.ComposeAggregateTestCheckFunc(checks...),
//			},
//			{
//				ResourceName: alertResourceName,
//				ImportState:  true,
//			},
//			{
//				Config: testAccCoralogixResourceAlertStandard(&updatedAlert),
//				Check:  resource.ComposeAggregateTestCheckFunc(updatedAlertChecks...),
//			},
//		},
//	})
//}

func testAccCheckAlertDestroy(s *terraform.State) error {
	client := testAccProvider.Meta().(*clientset.ClientSet).Alerts()

	ctx := context.TODO()

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "coralogix_alert" {
			continue
		}

		req := &alertsv3.GetAlertDefRequest{
			Id: wrapperspb.String(rs.Primary.ID),
		}

		resp, err := client.GetAlert(ctx, req)
		if err == nil {
			if resp.GetAlertDef().Id.Value == rs.Primary.ID {
				return fmt.Errorf("alert still exists: %s", rs.Primary.ID)
			}
		}
	}

	return nil
}

func testAccCoralogixResourceAlertLogsImmediate() string {
	return `resource "coralogix_alert" "test" {
  name        = "logs immediate alert"
  description = "Example of logs immediate alert from terraform"
  priority    = "P1"

  labels = {
    alert_type        = "security"
    security_severity = "high"
  }

  notification_group = {
    simple_target_settings = [
      {
        recipients = ["example@coralogix.com"]
      }
    ]
  }

  incidents_settings = {
    notify_on           = "Triggered and Resolved"
    retriggering_period = {
      minutes = 1
    }
  }

  schedule = {
    active_on = {
      days_of_week = ["Wednesday", "Thursday"]
      start_time   = {
        hours   = 8
        minutes = 30
      }
      end_time = {
        hours   = 20
        minutes = 30
      }
    }
  }

  type_definition = {
    logs_immediate = {
      logs_filter = {
        lucene_filter = {
          lucene_query  = "message:\"error\""
          label_filters = {
          }
        }
      }
    }
  }
}
`
}

func testAccCoralogixResourceAlertLogsImmediateUpdated() string {
	return `resource "coralogix_alert" "test" {
  name        = "logs immediate alert updated"
  description = "Example of logs immediate alert from terraform updated"
  priority    = "P2"

  notification_group = {
    advanced_target_settings = [
      {
        retriggering_period = {
          minutes = 10
        }
        notify_on  = "Triggered Only"
        recipients = ["example@coralogix.com"]
      }
    ]
  }
	
  incidents_settings = {
	notify_on           = "Triggered and Resolved"
	retriggering_period = {
		minutes = 10
	}
  }

  schedule = {
    active_on = {
      days_of_week = ["Wednesday", "Thursday"]
      start_time   = {
        hours   = 9
        minutes = 30
      }
      end_time = {
        hours   = 21
        minutes = 30
      }
    }
  }

  type_definition = {
    logs_immediate = {
      logs_filter = {
        lucene_filter = {
          lucene_query  = "message:\"error\""
          label_filters = {
          }
        }
      }
    }
  }
}
`
}

func testAccCoralogixResourceAlertLogsMoreThan() string {
	return `resource "coralogix_alert" "test" {
  name        = "logs-more-than alert example"
  description = "Example of logs-more-than alert example from terraform"
  priority    = "P2"

  labels = {
    alert_type        = "security"
    security_severity = "high"
  }

  notification_group = {
    simple_target_settings = [
      {
        integration_id = "17730"
      },
      {
        retriggering_period = {
          minutes = 1
        }
        notify_on  = "Triggered and Resolved"
        recipients = ["example@coralogix.com"]
      }
    ]
  }

  incidents_settings = {
    notify_on           = "Triggered and Resolved"
    retriggering_period = {
      minutes = 1
    }
  }

  schedule = {
    active_on = {
      days_of_week = ["Wednesday", "Thursday"]
      start_time   = {
        hours   = 8
        minutes = 30
      }
      end_time = {
        hours   = 20
        minutes = 30
      }
    }
  }

  type_definition = {
    logs_more_than = {
      threshold   = 2
      time_window = {
        specific_value = "10_MINUTES"
      }
      evaluation_window = "Dynamic"
      logs_filter       = {
        lucene_filter = {
          lucene_query  = "message:\"error\""
          label_filters = {
            application_name = [
              {
                operation = "OR"
                value     = "nginx"
              }
            ]
            subsystem_name = [
              {
                operation = "OR"
                value     = "subsystem-name"
              }
            ]
            severity = ["Warning"]
          }
        }
      }
    }
  }
}
`
}

func testAccCoralogixResourceAlertLogsMoreThanUpdated() string {
	return `resource "coralogix_alert" "test" {
  name        = "logs-more_-than alert example updated"
  description = "Example of standard alert from terraform updated"
  priority    = "P3"

  labels = {
    alert_type        = "security"
    security_severity = "low"
  }

  notification_group = {
    simple_target_settings = [
      {
        integration_id = "17730"
      }
    ]
  }

  incidents_settings = {
    notify_on           = "Triggered Only"
    retriggering_period = {
      minutes = 10
    }
  }

  schedule = {
    active_on = {
      days_of_week = ["Monday", "Thursday"]
      start_time   = {
        hours   = 8
        minutes = 30
      }
      end_time = {
        hours   = 20
        minutes = 30
      }
    }
  }

  type_definition = {
    logs_more_than = {
      threshold   = 20
      time_window = {
        specific_value = "2_HOURS"
      }
      evaluation_window = "Rolling"
      logs_filter       = {
        lucene_filter = {
          lucene_query  = "message:\"error\""
          label_filters = {
            application_name = [
              {
                operation = "OR"
                value     = "nginx"
              },
		      { 
                operation = "NOT"
                value     = "application_namee"
              }
            ]
          }
        }
      }
    }
  }
}
`
}

//func testAccCoralogixResourceAlertRatio(a *ratioAlertTestParams) string {
//	return fmt.Sprintf(`resource "coralogix_alert" "test" {
//  name               = "%s"
//  description        = "%s"
//  severity           = "%s"
//
//  notifications_group {
//  	notification {
//			integration_id       = "%s"
//   }
//	notification {
//		email_recipients             = %s
//	}
//  }
//
//	incident_settings {
//		notify_on = "%s"
//		retriggering_period_minutes = %d
//	}
//
//  scheduling {
//    time_zone =  "%s"
//
//	time_frame {
//    	days_enabled = %s
//    	start_time = "%s"
//    	end_time = "%s"
//  	}
//  }
//
//  ratio {
//    query_1 {
//		severities   = %s
//		search_query = "%s"
//    }
//    query_2 {
//      severities   = %s
//      search_query = "%s"
//    }
//    condition {
//      more_than     = true
//      ratio_threshold = %f
//      time_window   = "%s"
//      group_by      = %s
//      group_by_q1   = true
//	  ignore_infinity = %t
//    }
//  }
//}`,
//		a.name, a.description, a.severity, a.webhookID, sliceToString(a.emailRecipients), a.notifyOn, a.notifyEveryMin, a.timeZone,
//		sliceToString(a.daysOfWeek), a.activityStarts, a.activityEnds,
//		sliceToString(a.severities), a.searchQuery, sliceToString(a.q2Severities), a.q2SearchQuery,
//		a.ratio, a.timeWindow, sliceToString(a.groupBy), a.ignoreInfinity)
//}
//
//func testAccCoralogixResourceAlertNewValue(a *newValueAlertTestParams) string {
//	return fmt.Sprintf(`resource "coralogix_alert" "test" {
//  name               = "%s"
//  description        = "%s"
//  severity           = "%s"
//
//  notifications_group {
//		notification {
//        	integration_id       = "%s"
//		}
//		notification{
//     		email_recipients             = %s
//     	}
//	}
//
//	  incident_settings {
//			notify_on = "%s"
//			retriggering_period_minutes = %d
//		}
//
//  scheduling {
//    time_zone =  "%s"
//
//	time_frame {
//    	days_enabled = %s
//    	start_time = "%s"
//    	end_time = "%s"
//  	}
//  }
//
//  new_value {
//    severities = %s
//	search_query = "%s"
//    condition {
//      key_to_track = "%s"
//      time_window  = "%s"
//    }
//  }
//}`,
//		a.name, a.description, a.severity, a.webhookID, sliceToString(a.emailRecipients), a.notifyOn, a.notifyEveryMin, a.timeZone,
//		sliceToString(a.daysOfWeek), a.activityStarts, a.activityEnds,
//		sliceToString(a.severities), a.searchQuery, a.keyToTrack, a.timeWindow)
//}
//
//func testAccCoralogixResourceAlertUniqueCount(a *uniqueCountAlertTestParams) string {
//	return fmt.Sprintf(`resource "coralogix_alert" "test" {
//  name               = "%s"
//  description        = "%s"
//  severity           = "%s"
//
//  notifications_group {
//  		group_by_fields = %s
//		notification {
//        	integration_id       = "%s"
//		}
//		notification{
//     		email_recipients             = %s
//     	}
//	}
//
//	incident_settings {
//    	notify_on = "%s"
//    	retriggering_period_minutes = %d
//  	}
//
//  scheduling {
//    time_zone =  "%s"
//	time_frame {
//    	days_enabled = %s
//    	start_time = "%s"
//    	end_time = "%s"
//  	}
//  }
//
//  unique_count {
//    severities = %s
//    search_query = "%s"
//    condition {
//      unique_count_key  = "%s"
//      max_unique_values = %d
//      time_window       = "%s"
//      group_by_key                   = "%s"
//      max_unique_values_for_group_by = %d
//    }
//  }
//}`,
//		a.name, a.description, a.severity, sliceToString([]string{a.groupByKey}), a.webhookID, sliceToString(a.emailRecipients), a.notifyOn, a.notifyEveryMin, a.timeZone,
//		sliceToString(a.daysOfWeek), a.activityStarts, a.activityEnds, sliceToString(a.severities),
//		a.searchQuery, a.uniqueCountKey, a.maxUniqueValues, a.timeWindow, a.groupByKey, a.maxUniqueValuesForGroupBy)
//}
//
//func testAccCoralogixResourceAlertTimeRelative(a *timeRelativeAlertTestParams) string {
//	return fmt.Sprintf(`resource "coralogix_alert" "test" {
//  name               = "%s"
//  description        = "%s"
//  severity           = "%s"
//
//  notifications_group {
//		notification {
//        	integration_id       = "%s"
//		}
//		notification{
//     		email_recipients             = %s
//     	}
//	}
//
//  incident_settings {
//    	notify_on = "%s"
//    	retriggering_period_minutes = %d
// }
//
//  scheduling {
//    time_zone =  "%s"
//
//	time_frame {
//    	days_enabled = %s
//    	start_time = "%s"
//    	end_time = "%s"
//  	}
//  }
//
//  time_relative {
//    severities = %s
//    search_query = "%s"
//    condition {
//      more_than            = true
//      group_by             = %s
//      ratio_threshold      = %d
//      relative_time_window = "%s"
//      ignore_infinity = %t
//    }
//  }
//}`,
//		a.name, a.description, a.severity, a.webhookID, sliceToString(a.emailRecipients), a.notifyOn, a.notifyEveryMin, a.timeZone,
//		sliceToString(a.daysOfWeek), a.activityStarts, a.activityEnds,
//		sliceToString(a.severities), a.searchQuery, sliceToString(a.groupBy), a.ratioThreshold, a.relativeTimeWindow, a.ignoreInfinity)
//}
//
//func testAccCoralogixResourceAlertMetricLucene(a *metricLuceneAlertTestParams) string {
//	return fmt.Sprintf(`resource "coralogix_alert" "test" {
//  name               = "%s"
//  description        = "%s"
//  severity           = "%s"
//
//  notifications_group {
//		notification {
//        	integration_id       = "%s"
//		}
//		notification{
//     		email_recipients             = %s
//     	}
//	}
//
//	incident_settings {
//    	notify_on = "%s"
//    	retriggering_period_minutes = %d
// 	}
//
//  scheduling {
//    time_zone =  "%s"
//
//	time_frame {
//    	days_enabled = %s
//    	start_time = "%s"
//    	end_time = "%s"
//  	}
//  }
//
//  metric {
//    lucene {
//      search_query = "%s"
//      condition {
//        metric_field                 = "%s"
//        arithmetic_operator          = "%s"
//        less_than                    = true
//        threshold                    = %d
//        arithmetic_operator_modifier = %d
//        sample_threshold_percentage  = %d
//        time_window                  = "%s"
//		group_by = %s
//		manage_undetected_values{
//			enable_triggering_on_undetected_values = false
//		}
//      }
//    }
//  }
//}`,
//		a.name, a.description, a.severity, a.webhookID, sliceToString(a.emailRecipients), a.notifyOn, a.notifyEveryMin, a.timeZone,
//		sliceToString(a.daysOfWeek), a.activityStarts, a.activityEnds, a.searchQuery, a.metricField, a.arithmeticOperator,
//		a.threshold, a.arithmeticOperatorModifier, a.sampleThresholdPercentage, a.timeWindow, sliceToString(a.groupBy))
//}
//
//func testAccCoralogixResourceAlertMetricPromql(a *metricPromqlAlertTestParams) string {
//	return fmt.Sprintf(`resource "coralogix_alert" "test" {
//  name               = "%s"
//  description        = "%s"
//  severity           = "%s"
//
//  notifications_group {
//		notification {
//        	integration_id       = "%s"
//		}
//		notification{
//     		email_recipients             = %s
//     	}
//	}
//
//  incident_settings {
//	notify_on = "%s"
//	retriggering_period_minutes = %d
//  }
//
//  scheduling {
//    time_zone =  "%s"
//	time_frame {
//    	days_enabled = %s
//    	start_time = "%s"
//    	end_time = "%s"
//  	}
//  }
//
//  metric {
//    promql {
//      search_query = "http_requests_total{status!~\"4..\"}"
//      condition {
//        %s                    	     = true
//        threshold                    = %d
//        sample_threshold_percentage  = %d
//        time_window                  = "%s"
//        min_non_null_values_percentage = %d
//      }
//    }
//  }
//}`,
//		a.name, a.description, a.severity, a.webhookID, sliceToString(a.emailRecipients), a.notifyOn, a.notifyEveryMin, a.timeZone,
//		sliceToString(a.daysOfWeek), a.activityStarts, a.activityEnds, a.condition, a.threshold, a.sampleThresholdPercentage,
//		a.timeWindow, a.nonNullPercentage)
//}
//
//func testAccCoralogixResourceAlertTracing(a *tracingAlertTestParams) string {
//	return fmt.Sprintf(`resource "coralogix_alert" "test" {
//  name               = "%s"
//  description        = "%s"
//  severity           = "%s"
//
//	notifications_group {
//		notification {
//        	integration_id       = "%s"
//		}
//		notification{
//     		email_recipients             = %s
//     	}
//	}
//
// incident_settings {
// 	notify_on = "%s"
//    retriggering_period_minutes = %d
// }
//
//  scheduling {
//    time_zone =  "%s"
//	time_frame {
//    	days_enabled = %s
//    	start_time = "%s"
//    	end_time = "%s"
//  	}
//  }
//
//  tracing {
//    latency_threshold_milliseconds = %f
//    applications = ["nginx"]
//    subsystems = ["subsystem-name"]
//	tag_filter {
//      field = "Status"
//      values = ["filter:contains:400", "500"]
//    }
//
//    condition {
//      more_than             = true
//      time_window           = "%s"
//      threshold = %d
//    }
//  }
//}`,
//		a.name, a.description, a.severity, a.webhookID, sliceToString(a.emailRecipients), a.notifyOn, a.notifyEveryMin, a.timeZone,
//		sliceToString(a.daysOfWeek), a.activityStarts, a.activityEnds,
//		a.conditionLatencyMs, a.timeWindow, a.occurrencesThreshold)
//}
//
//func testAccCoralogixResourceAlertFLow(a *flowAlertTestParams) string {
//	return fmt.Sprintf(`resource "coralogix_alert" "standard_alert" {
//	name               = "standard"
//	severity           = "Info"
//
//	notifications_group {
//    	notification {
//      		email_recipients            = ["example@coralogix.com"]
//			retriggering_period_minutes = 1
//     		notify_on                   = "Triggered_only"
//    	}
//  	}
//
//	standard {
//		condition {
//      		more_than         = true
//      		threshold         = 5
//      		time_window       = "30Min"
//      		group_by          = ["coralogix.metadata.sdkId"]
//    	}
//	}
//}
//
//	resource "coralogix_alert" "test" {
//  		name               = "%s"
//  		description        = "%s"
//	  	severity           = "%s"
//
//	  notifications_group {
//		notification {
//        	integration_id       = "%s"
//		}
//		notification{
//     		email_recipients             = %s
//     	}
//	}
//
//	incident_settings {
//			notify_on = "%s"
//			retriggering_period_minutes = %d
//    }
//
//  	scheduling {
//    	time_zone =  "%s"
//		time_frame {
//    		days_enabled = %s
//    		start_time = "%s"
//			end_time = "%s"
//  		}
//	}
//
//  	flow {
//    	stage {
//      		group {
//        		sub_alerts {
//          			operator = "OR"
//          			flow_alert{
//            			user_alert_id = coralogix_alert.standard_alert.id
//          			}
//        		}
//        next_operator = "OR"
//      }
//      group {
//        sub_alerts {
//          operator = "AND"
//          flow_alert{
//            not = true
//            user_alert_id = coralogix_alert.standard_alert.id
//          }
//        }
//        next_operator = "AND"
//      }
//      time_window {
//        minutes = 20
//      }
//    }
//    stage {
//      group {
//        sub_alerts {
//          operator = "AND"
//          flow_alert {
//            user_alert_id = coralogix_alert.standard_alert.id
//          }
//          flow_alert {
//            not = true
//            user_alert_id = coralogix_alert.standard_alert.id
//          }
//        }
//        next_operator = "OR"
//      }
//    }
//    group_by          = ["coralogix.metadata.sdkId"]
//  }
//}`,
//		a.name, a.description, a.severity, a.webhookID, sliceToString(a.emailRecipients), a.notifyOn, a.notifyEveryMin, a.timeZone,
//		sliceToString(a.daysOfWeek), a.activityStarts, a.activityEnds)
//}
