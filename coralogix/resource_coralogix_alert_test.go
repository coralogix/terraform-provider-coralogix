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
							"recipients.0": "example@coralogix.com",
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
							"recipients.0":                "example@coralogix.com",
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
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "notification_group.simple_target_settings.*",
						map[string]string{
							"integration_id": "17730",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "notification_group.simple_target_settings.*",
						map[string]string{
							"recipients.#": "1",
							"recipients.0": "example@coralogix.com",
						},
					),
					resource.TestCheckResourceAttr(alertResourceName, "incidents_settings.notify_on", "Triggered and Resolved"),
					resource.TestCheckResourceAttr(alertResourceName, "incidents_settings.retriggering_period.minutes", "1"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.days_of_week.#", "2"),
					resource.TestCheckTypeSetElemAttr(alertResourceName, "schedule.active_on.days_of_week.*", "Wednesday"),
					resource.TestCheckTypeSetElemAttr(alertResourceName, "schedule.active_on.days_of_week.*", "Thursday"),
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
					resource.TestCheckTypeSetElemAttr(alertResourceName, "type_definition.logs_more_than.logs_filter.lucene_filter.label_filters.severities.*", "Warning"),
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
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.logs_more_than.logs_filter.lucene_filter.label_filters.application_name.*",
						map[string]string{
							"operation": "OR",
							"value":     "nginx",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.logs_more_than.logs_filter.lucene_filter.label_filters.application_name.*",
						map[string]string{
							"operation": "NOT",
							"value":     "application_name",
						},
					),
				),
			},
		},
	})
}

func TestAccCoralogixResourceAlert_logs_less_than(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAlertDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertLogsLessThan(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(alertResourceName, "name", "logs-less-than alert example"),
					resource.TestCheckResourceAttr(alertResourceName, "description", "Example of logs-less-than alert example from terraform"),
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
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_less_than.threshold", "2"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_less_than.time_window.specific_value", "10_MINUTES"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_less_than.evaluation_window", "Dynamic"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_less_than.logs_filter.lucene_filter.lucene_query", "message:\"error\""),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.logs_less_than.logs_filter.lucene_filter.label_filters.application_name.*",
						map[string]string{
							"operation": "OR",
							"value":     "nginx",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.logs_less_than.logs_filter.lucene_filter.label_filters.subsystem_name.*",
						map[string]string{
							"operation": "OR",
							"value":     "subsystem-name",
						},
					),
					resource.TestCheckTypeSetElemAttr(alertResourceName, "type_definition.logs_less_than.logs_filter.lucene_filter.label_filters.severities.*", "Warning"),
				),
			},
			{
				ResourceName: alertResourceName,
				ImportState:  true,
			},
			{
				Config: testAccCoralogixResourceAlertLogsLessThanUpdated(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(alertResourceName, "name", "logs-less-than alert example updated"),
					resource.TestCheckResourceAttr(alertResourceName, "description", "Example of logs-less-than alert example from terraform updated"),
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
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_less_than.threshold", "20"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_less_than.time_window.specific_value", "2_HOURS"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_less_than.evaluation_window", "Rolling"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_less_than.logs_filter.lucene_filter.lucene_query", "message:\"error\""),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_less_than.logs_filter.lucene_filter.label_filters.application_name.#", "2"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_less_than.logs_filter.lucene_filter.label_filters.application_name.0.operation", "OR"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_less_than.logs_filter.lucene_filter.label_filters.application_name.0.value", "nginx"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_less_than.logs_filter.lucene_filter.label_filters.application_name.1.operation", "NOT"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_less_than.logs_filter.lucene_filter.label_filters.application_name.1.value", "application_name"),
				),
			},
		},
	},
	)
}

func TestAccCoralogixResourceAlert_logs_more_than_usual_alert(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAlertDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertLogsMoreThanUsual(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(alertResourceName, "name", "logs-more-than alert example"),
					resource.TestCheckResourceAttr(alertResourceName, "description", "Example of logs-more-than alert example from terraform"),
					resource.TestCheckResourceAttr(alertResourceName, "priority", "P2"),
					resource.TestCheckResourceAttr(alertResourceName, "labels.alert_type", "security"),
					resource.TestCheckResourceAttr(alertResourceName, "labels.security_severity", "high"),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "notification_group.simple_target_settings.*",
						map[string]string{
							"integration_id": "17730",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "notification_group.simple_target_settings.*",
						map[string]string{
							"recipients.#": "1",
							"recipients.0": "example@coralogix.com",
						},
					),
					resource.TestCheckResourceAttr(alertResourceName, "incidents_settings.notify_on", "Triggered and Resolved"),
					resource.TestCheckResourceAttr(alertResourceName, "incidents_settings.retriggering_period.minutes", "1"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.days_of_week.#", "2"),
					resource.TestCheckTypeSetElemAttr(alertResourceName, "schedule.active_on.days_of_week.*", "Wednesday"),
					resource.TestCheckTypeSetElemAttr(alertResourceName, "schedule.active_on.days_of_week.*", "Thursday"),
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
					resource.TestCheckTypeSetElemAttr(alertResourceName, "type_definition.logs_more_than.logs_filter.lucene_filter.label_filters.severities.*", "Warning"),
				),
			},
			{
				ResourceName: alertResourceName,
				ImportState:  true,
			},
			{
				Config: testAccCoralogixResourceAlertLogsMoreThanUsualUpdated(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(alertResourceName, "name", "logs-more-than alert example updated"),
					resource.TestCheckResourceAttr(alertResourceName, "description", "Example of standard alert from terraform updated"),
					resource.TestCheckResourceAttr(alertResourceName, "priority", "P1"),
					resource.TestCheckResourceAttr(alertResourceName, "labels.#", "0"),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "notification_group.simple_target_settings.*",
						map[string]string{
							"integration_id": "17730",
						},
					),
					resource.TestCheckResourceAttr(alertResourceName, "incidents_settings.notify_on", "Triggered Only"),
					resource.TestCheckResourceAttr(alertResourceName, "incidents_settings.retriggering_period.minutes", "10"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.days_of_week.#", "0"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_more_than.threshold", "20"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_more_than.time_window.specific_value", "2_HOURS"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_more_than.evaluation_window", "Rolling"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_more_than.logs_filter.lucene_filter.lucene_query", "message:\"updated_error\""),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.logs_more_than.logs_filter.lucene_filter.label_filters.application_name.*",
						map[string]string{
							"operation": "OR",
							"value":     "nginx",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.logs_more_than.logs_filter.lucene_filter.label_filters.application_name.*",
						map[string]string{
							"operation": "NOT",
							"value":     "application_name",
						},
					),
				),
			},
		},
	},
	)
}

func TestAccCoralogixResourceAlert_logs_less_than_usual_alert(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAlertDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertLogsLessThanUsual(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(alertResourceName, "name", "logs-less-than alert example"),
					resource.TestCheckResourceAttr(alertResourceName, "description", "Example of logs-less-than alert example from terraform"),
					resource.TestCheckResourceAttr(alertResourceName, "priority", "P2"),
					resource.TestCheckResourceAttr(alertResourceName, "labels.alert_type", "security"),
					resource.TestCheckResourceAttr(alertResourceName, "labels.security_severity", "high"),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "notification_group.simple_target_settings.*",
						map[string]string{
							"recipients.#": "2",
							"recipients.0": "example@coralogix.com",
							"recipients.1": "example2@coralogix.com",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "notification_group.simple_target_settings.*",
						map[string]string{
							"integration_id": "17730",
						},
					),
					resource.TestCheckResourceAttr(alertResourceName, "incidents_settings.notify_on", "Triggered and Resolved"),
					resource.TestCheckResourceAttr(alertResourceName, "incidents_settings.retriggering_period.minutes", "1"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.days_of_week.#", "2"),
					resource.TestCheckTypeSetElemAttr(alertResourceName, "schedule.active_on.days_of_week.*", "Wednesday"),
					resource.TestCheckTypeSetElemAttr(alertResourceName, "schedule.active_on.days_of_week.*", "Thursday"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.start_time.hours", "10"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.start_time.minutes", "30"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.end_time.hours", "20"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.end_time.minutes", "30"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_less_than.threshold", "2"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_less_than.time_window.specific_value", "10_MINUTES"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_less_than.evaluation_window", "Dynamic"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_less_than.logs_filter.lucene_filter.lucene_query", "message:\"error\""),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.logs_less_than.logs_filter.lucene_filter.label_filters.application_name.*",
						map[string]string{
							"operation": "NOT",
							"value":     "nginx",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.logs_less_than.logs_filter.lucene_filter.label_filters.subsystem_name.*",
						map[string]string{
							"operation": "AND",
							"value":     "subsystem-name",
						},
					),
					resource.TestCheckTypeSetElemAttr(alertResourceName, "type_definition.logs_less_than.logs_filter.lucene_filter.label_filters.severities.*", "Warning"),
					resource.TestCheckTypeSetElemAttr(alertResourceName, "type_definition.logs_less_than.logs_filter.lucene_filter.label_filters.severities.*", "Error"),
				),
			},
			{
				ResourceName: alertResourceName,
				ImportState:  true,
			},
			{
				Config: testAccCoralogixResourceAlertLogsLessThanUsualUpdated(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(alertResourceName, "name", "logs-less-than alert example updated"),
					resource.TestCheckResourceAttr(alertResourceName, "description", "Example of logs-less-than alert example from terraform updated"),
					resource.TestCheckResourceAttr(alertResourceName, "priority", "P3"),
					resource.TestCheckResourceAttr(alertResourceName, "notification_group.simple_target_settings.#", "1"),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "notification_group.simple_target_settings.*",
						map[string]string{
							"integration_id": "17730",
						},
					),
					resource.TestCheckResourceAttr(alertResourceName, "incidents_settings.notify_on", "Triggered Only"),
					resource.TestCheckResourceAttr(alertResourceName, "incidents_settings.retriggering_period.minutes", "10"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.days_of_week.#", "2"),
					resource.TestCheckTypeSetElemAttr(alertResourceName, "schedule.active_on.days_of_week.*", "Monday"),
					resource.TestCheckTypeSetElemAttr(alertResourceName, "schedule.active_on.days_of_week.*", "Thursday"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.start_time.hours", "8"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.start_time.minutes", "30"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.end_time.hours", "20"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.end_time.minutes", "30"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_less_than.threshold", "20"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_less_than.time_window.specific_value", "2_HOURS"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_less_than.evaluation_window", "Rolling"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_less_than.logs_filter.lucene_filter.lucene_query", "message:\"error\""),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.logs_less_than.logs_filter.lucene_filter.label_filters.application_name.*",
						map[string]string{
							"operation": "OR",
							"value":     "nginx",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.logs_less_than.logs_filter.lucene_filter.label_filters.application_name.*",
						map[string]string{
							"operation": "NOT",
							"value":     "application_name",
						},
					),
				),
			},
		},
	},
	)
}

func testAccCoralogixResourceAlertLogsLessThanUsual() string {
	return `resource "coralogix_alert" "test" {
	  name        = "logs-less-than alert example"
	  description = "Example of logs-less-than alert example from terraform"
	  priority    = "P2"
      
	  labels = {
		alert_type        = "security"
		security_severity = "high"
		}

	  notification_group = {
		simple_target_settings = [
		{
			recipients = ["example@coralogix.com", "example2@coralogix.com"]
		},
		{
			integration_id = "17730"
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
				hours   = 10
				minutes = 30
			}
			end_time = {	
				hours   = 20
				minutes = 30
			}
		}
	  }

	  type_definition = {
		logs_less_than = {
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
								operation = "NOT"
								value     = "application_name"
							}	
						]	
						subsystem_name = [			
							{
								operation = "AND"
								value     = "subsystem-name"		
							}
						]
						severities = ["Warning", "Error"]
					}
				}
			}
		}
	  }
	}
	`
}

func testAccCoralogixResourceAlertLogsLessThanUsualUpdated() string {
	return `resource "coralogix_alert" "test" {
	  name        = "logs-less-than alert example updated"
	  description = "Example of logs-less-than alert example from terraform updated"
	  priority    = "P3"

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
		logs_less_than = {
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
								value     = "application_name"
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
            severities = ["Warning"]
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
                value     = "application_name"
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

func testAccCoralogixResourceAlertLogsLessThan() string {
	return `resource "coralogix_alert" "test" {
  name        = "logs-less-than alert example"
  description = "Example of logs-less-than alert example from terraform"
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
		logs_less_than = {
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
				severities= ["Warning"]
			  }
			}
		  }
		}
	  }
	}
`
}

func testAccCoralogixResourceAlertLogsLessThanUpdated() string {
	return `resource "coralogix_alert" "test" {
  name        = "logs-less-than alert example updated"
  description = "Example of logs-less-than alert example from terraform updated"
  priority    = "P3"

  labels = {
	alert_type        = "security"
	security_severity = "low"
  }

  notification_group = {
	advanced_target_settings = [
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
	logs_less_than = {
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
				value     = "application_name"
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

func testAccCoralogixResourceAlertLogsMoreThanUsual() string {
	return `resource "coralogix_alert" "test" {
  name        = "logs-more-than-usual alert example"
  description = "Example of logs-more-than-usual alert from terraform"
  priority    = "P4"

  labels = {
    alert_type        = "security"
    security_severity = "high"
  }

  notification_group = {
    advanced_target_settings = [
      {
        integration_id = coralogix_webhook.slack_webhook.external_id
        notify_on      = "Triggered and Resolved"
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
    logs_more_than_usual = {
      logs_filter = {
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
            severities = ["Warning"]
          }
        }
      }
      notification_payload_filter = [
        "coralogix.metadata.sdkId", "coralogix.metadata.sdkName", "coralogix.metadata.sdkVersion"
      ]
      time_window = {
        specific_value = "10_MINUTES"
      }
      minimum_threshold = 2
    }
  }
}
`
}

func testAccCoralogixResourceAlertLogsMoreThanUsualUpdated() string {
	return `resource "coralogix_alert" "test" {
  name        = "logs-more-than-usual alert example updated"
  description = "Example of logs-more-than-usual alert from terraform updated"
  priority    = "P1"

  notification_group = {
    advanced_target_settings = [
      {
        integration_id = "17730"
        notify_on      = "Triggered Only"
      }
    ]
  }

  type_definition = {
    logs_more_than_usual = {
      logs_filter = {
        lucene_filter = {
          lucene_query  = "message:\"updated_error\""
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
            severities = ["Warning", "Error"]
          }
        }
      }
      time_window = {
        specific_value = "1_HOUR"
      }
      minimum_threshold = 20
    }
  }
}
`
}
