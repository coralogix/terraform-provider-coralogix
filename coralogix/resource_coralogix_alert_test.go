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
					resource.TestCheckTypeSetElemAttr(alertResourceName, "schedule.active_on.days_of_week.*", "Wednesday"),
					resource.TestCheckTypeSetElemAttr(alertResourceName, "schedule.active_on.days_of_week.*", "Thursday"),
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
					resource.TestCheckTypeSetElemAttr(alertResourceName, "schedule.active_on.days_of_week.*", "Wednesday"),
					resource.TestCheckTypeSetElemAttr(alertResourceName, "schedule.active_on.days_of_week.*", "Thursday"),
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
							"operation": "IS",
							"value":     "nginx",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.logs_more_than.logs_filter.lucene_filter.label_filters.subsystem_name.*",
						map[string]string{
							"operation": "IS",
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
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_more_than.threshold", "20"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_more_than.time_window.specific_value", "2_HOURS"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_more_than.evaluation_window", "Rolling"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_more_than.logs_filter.lucene_filter.lucene_query", "message:\"error\""),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.logs_more_than.logs_filter.lucene_filter.label_filters.application_name.*",
						map[string]string{
							"operation": "IS",
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
					resource.TestCheckResourceAttr(alertResourceName, "notification_group.simple_target_settings.#", "2"),
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
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_less_than.threshold", "2"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_less_than.time_window.specific_value", "10_MINUTES"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_less_than.logs_filter.lucene_filter.lucene_query", "message:\"error\""),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.logs_less_than.logs_filter.lucene_filter.label_filters.application_name.*",
						map[string]string{
							"operation": "IS",
							"value":     "nginx",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.logs_less_than.logs_filter.lucene_filter.label_filters.subsystem_name.*",
						map[string]string{
							"operation": "IS",
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
					resource.TestCheckResourceAttr(alertResourceName, "notification_group.advanced_target_settings.#", "1"),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "notification_group.advanced_target_settings.*",
						map[string]string{
							"integration_id":              "17730",
							"notify_on":                   "Triggered Only",
							"retriggering_period.minutes": "10",
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
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_less_than.logs_filter.lucene_filter.lucene_query", "message:\"error\""),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.logs_less_than.logs_filter.lucene_filter.label_filters.application_name.*",
						map[string]string{
							"operation": "IS",
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

func TestAccCoralogixResourceAlert_logs_more_than_usual(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAlertDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertLogsMoreThanUsual(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(alertResourceName, "name", "logs-more-than-usual alert example"),
					resource.TestCheckResourceAttr(alertResourceName, "description", "Example of logs-more-than-usual alert from terraform"),
					resource.TestCheckResourceAttr(alertResourceName, "priority", "P4"),
					resource.TestCheckResourceAttr(alertResourceName, "labels.alert_type", "security"),
					resource.TestCheckResourceAttr(alertResourceName, "labels.security_severity", "high"),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "notification_group.advanced_target_settings.*",
						map[string]string{
							"retriggering_period.minutes": "1",
							"notify_on":                   "Triggered and Resolved",
							"recipients.#":                "1",
							"recipients.0":                "example@coralogix.com",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "notification_group.advanced_target_settings.*",
						map[string]string{
							"integration_id": "17730",
							"notify_on":      "Triggered and Resolved",
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
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.logs_more_than_usual.logs_filter.lucene_filter.label_filters.application_name.*",
						map[string]string{
							"operation": "IS",
							"value":     "nginx",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.logs_more_than_usual.logs_filter.lucene_filter.label_filters.subsystem_name.*",
						map[string]string{
							"operation": "IS",
							"value":     "subsystem-name",
						},
					),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_more_than_usual.logs_filter.lucene_filter.lucene_query", "message:\"error\""),
					resource.TestCheckTypeSetElemAttr(alertResourceName, "type_definition.logs_more_than_usual.logs_filter.lucene_filter.label_filters.severities.*", "Warning"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_more_than_usual.time_window.specific_value", "10_MINUTES"),
					resource.TestCheckTypeSetElemAttr(alertResourceName, "type_definition.logs_more_than_usual.notification_payload_filter.*", "coralogix.metadata.sdkId"),
					resource.TestCheckTypeSetElemAttr(alertResourceName, "type_definition.logs_more_than_usual.notification_payload_filter.*", "coralogix.metadata.sdkName"),
					resource.TestCheckTypeSetElemAttr(alertResourceName, "type_definition.logs_more_than_usual.notification_payload_filter.*", "coralogix.metadata.sdkVersion"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_more_than_usual.minimum_threshold", "2"),
				),
			},
			{
				ResourceName: alertResourceName,
				ImportState:  true,
			},
			{
				Config: testAccCoralogixResourceAlertLogsMoreThanUsualUpdated(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(alertResourceName, "name", "logs-more-than-usual alert example updated"),
					resource.TestCheckResourceAttr(alertResourceName, "description", "Example of logs-more-than-usual alert from terraform updated"),
					resource.TestCheckResourceAttr(alertResourceName, "priority", "P1"),
					resource.TestCheckResourceAttr(alertResourceName, "labels.#", "0"),
					resource.TestCheckResourceAttr(alertResourceName, "notification_group.advanced_target_settings.#", "1"),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "notification_group.advanced_target_settings.*",
						map[string]string{
							"integration_id":              "17730",
							"notify_on":                   "Triggered and Resolved",
							"retriggering_period.minutes": "10",
						},
					),
					resource.TestCheckResourceAttr(alertResourceName, "incidents_settings.notify_on", "Triggered and Resolved"),
					resource.TestCheckResourceAttr(alertResourceName, "incidents_settings.retriggering_period.minutes", "1"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_more_than_usual.logs_filter.lucene_filter.lucene_query", "message:\"updated_error\""),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.logs_more_than_usual.logs_filter.lucene_filter.label_filters.application_name.*",
						map[string]string{
							"operation": "IS",
							"value":     "nginx",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.logs_more_than_usual.logs_filter.lucene_filter.label_filters.subsystem_name.*",
						map[string]string{
							"operation": "IS",
							"value":     "subsystem-name",
						},
					),
					resource.TestCheckTypeSetElemAttr(alertResourceName, "type_definition.logs_more_than_usual.logs_filter.lucene_filter.label_filters.severities.*", "Warning"),
					resource.TestCheckTypeSetElemAttr(alertResourceName, "type_definition.logs_more_than_usual.logs_filter.lucene_filter.label_filters.severities.*", "Error"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_more_than_usual.time_window.specific_value", "1_HOUR"),
				),
			},
		},
	},
	)
}

func TestAccCoralogixResourceAlert_logs_less_than_usual(t *testing.T) {
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
							"recipients.0": "example2@coralogix.com",
							"recipients.1": "example@coralogix.com",
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
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_less_than.logs_filter.lucene_filter.lucene_query", "message:\"error\""),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.logs_less_than.logs_filter.lucene_filter.label_filters.application_name.*",
						map[string]string{
							"operation": "NOT",
							"value":     "application_name",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.logs_less_than.logs_filter.lucene_filter.label_filters.subsystem_name.*",
						map[string]string{
							"operation": "STARTS_WITH",
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
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_less_than.logs_filter.lucene_filter.lucene_query", "message:\"error\""),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.logs_less_than.logs_filter.lucene_filter.label_filters.application_name.*",
						map[string]string{
							"operation": "IS",
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

func TestAccCoralogixResourceAlert_logs_ratio_more_than(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAlertDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertLogsRatioMoreThan(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(alertResourceName, "name", "logs-ratio-more-than alert example"),
					resource.TestCheckResourceAttr(alertResourceName, "description", "Example of logs-ratio-more-than alert from terraform"),
					resource.TestCheckResourceAttr(alertResourceName, "priority", "P1"),
					resource.TestCheckResourceAttr(alertResourceName, "group_by.#", "2"),
					resource.TestCheckTypeSetElemAttr(alertResourceName, "group_by.*", "coralogix.metadata.alert_id"),
					resource.TestCheckTypeSetElemAttr(alertResourceName, "group_by.*", "coralogix.metadata.alert_name"),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "notification_group.simple_target_settings.*",
						map[string]string{
							"recipients.#": "1",
							"recipients.0": "example@coralogix.com",
						},
					),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_more_than.denominator_alias", "denominator"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_more_than.numerator_alias", "numerator"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_more_than.time_window.specific_value", "10_MINUTES"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_more_than.threshold", "2"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_more_than.denominator_logs_filter.lucene_filter.lucene_query", "mod_date:[20020101 TO 20030101]"),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.logs_ratio_more_than.denominator_logs_filter.lucene_filter.label_filters.application_name.*",
						map[string]string{
							"operation": "IS",
							"value":     "nginx",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.logs_ratio_more_than.denominator_logs_filter.lucene_filter.label_filters.subsystem_name.*",
						map[string]string{
							"operation": "IS",
							"value":     "subsystem-name",
						},
					),
					resource.TestCheckTypeSetElemAttr(alertResourceName, "type_definition.logs_ratio_more_than.denominator_logs_filter.lucene_filter.label_filters.severities.*", "Warning"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_more_than.numerator_logs_filter.lucene_filter.lucene_query", "mod_date:[20030101 TO 20040101]"),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.logs_ratio_more_than.numerator_logs_filter.lucene_filter.label_filters.application_name.*",
						map[string]string{
							"operation": "IS",
							"value":     "nginx",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.logs_ratio_more_than.numerator_logs_filter.lucene_filter.label_filters.subsystem_name.*",
						map[string]string{
							"operation": "IS",
							"value":     "subsystem-name",
						},
					),
					resource.TestCheckTypeSetElemAttr(alertResourceName, "type_definition.logs_ratio_more_than.numerator_logs_filter.lucene_filter.label_filters.severities.*", "Error"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_more_than.group_by_for", "Both"),
				),
			},
			{
				ResourceName: alertResourceName,
				ImportState:  true,
			},
			{
				Config: testAccCoralogixResourceAlertLogsRatioMoreThanUpdated(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(alertResourceName, "name", "logs-ratio-more-than alert example updated"),
					resource.TestCheckResourceAttr(alertResourceName, "description", "Example of logs-ratio-more-than alert from terraform updated"),
					resource.TestCheckResourceAttr(alertResourceName, "priority", "P2"),
					resource.TestCheckResourceAttr(alertResourceName, "group_by.#", "3"),
					resource.TestCheckTypeSetElemAttr(alertResourceName, "group_by.*", "coralogix.metadata.alert_id"),
					resource.TestCheckTypeSetElemAttr(alertResourceName, "group_by.*", "coralogix.metadata.alert_name"),
					resource.TestCheckTypeSetElemAttr(alertResourceName, "group_by.*", "coralogix.metadata.alert_description"),
					resource.TestCheckResourceAttr(alertResourceName, "notification_group.simple_target_settings.#", "2"),
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
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_more_than.denominator_alias", "updated-denominator"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_more_than.numerator_alias", "updated-numerator"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_more_than.time_window.specific_value", "1_HOUR"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_more_than.threshold", "120"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_more_than.group_by_for", "Numerator Only"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_more_than.denominator_logs_filter.lucene_filter.lucene_query", "mod_date:[20030101 TO 20040101]"),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.logs_ratio_more_than.denominator_logs_filter.lucene_filter.label_filters.application_name.*",
						map[string]string{
							"operation": "IS",
							"value":     "nginx",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.logs_ratio_more_than.denominator_logs_filter.lucene_filter.label_filters.subsystem_name.*",
						map[string]string{
							"operation": "IS",
							"value":     "subsystem-name",
						},
					),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_more_than.numerator_logs_filter.lucene_filter.lucene_query", "mod_date:[20040101 TO 20050101]"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_more_than.numerator_logs_filter.lucene_filter.label_filters.application_name.#", "0"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_more_than.numerator_logs_filter.lucene_filter.label_filters.severities.#", "0"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_more_than.numerator_logs_filter.lucene_filter.label_filters.subsystem_name.#", "2"),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.logs_ratio_more_than.numerator_logs_filter.lucene_filter.label_filters.subsystem_name.*",
						map[string]string{
							"operation": "ENDS_WITH",
							"value":     "updated-subsystem-name",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.logs_ratio_more_than.numerator_logs_filter.lucene_filter.label_filters.subsystem_name.*",
						map[string]string{
							"operation": "NOT",
							"value":     "subsystem-name",
						},
					),
				),
			},
		},
	},
	)
}

func TestAccCoralogixResourceAlert_logs_ratio_less_than(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAlertDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertLogsRatioLessThan(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(alertResourceName, "name", "logs-ratio-less-than alert example"),
					resource.TestCheckResourceAttr(alertResourceName, "description", "Example of logs-ratio-less-than alert from terraform"),
					resource.TestCheckResourceAttr(alertResourceName, "priority", "P3"),
					resource.TestCheckResourceAttr(alertResourceName, "group_by.#", "2"),
					resource.TestCheckTypeSetElemAttr(alertResourceName, "group_by.*", "coralogix.metadata.alert_id"),
					resource.TestCheckTypeSetElemAttr(alertResourceName, "group_by.*", "coralogix.metadata.alert_name"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_less_than.denominator_alias", "denominator"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_less_than.numerator_alias", "numerator"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_less_than.time_window.specific_value", "10_MINUTES"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_less_than.threshold", "2"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_less_than.group_by_for", "Denominator Only"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_less_than.undetected_values_management.trigger_undetected_values", "false"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_less_than.undetected_values_management.auto_retire_timeframe", "Never"),
				),
			},
			{
				ResourceName: alertResourceName,
				ImportState:  true,
			},
			{
				Config: testAccCoralogixResourceAlertLogsRatioLessThanUpdated(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(alertResourceName, "name", "logs-ratio-less-than alert example updated"),
					resource.TestCheckResourceAttr(alertResourceName, "description", "Example of logs-ratio-less-than alert from terraform updated"),
					resource.TestCheckResourceAttr(alertResourceName, "priority", "P2"),
					resource.TestCheckResourceAttr(alertResourceName, "group_by.#", "0"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_less_than.denominator_alias", "updated-denominator"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_less_than.numerator_alias", "updated-numerator"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_less_than.time_window.specific_value", "2_HOURS"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_less_than.threshold", "20"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_less_than.undetected_values_management.trigger_undetected_values", "true"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_less_than.undetected_values_management.auto_retire_timeframe", "6_Hours"),
				),
			},
		},
	})
}

func TestAccCoralogixResourceAlert_logs_new_value(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAlertDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertLogsNewValue(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(alertResourceName, "name", "logs-new-value alert example"),
					resource.TestCheckResourceAttr(alertResourceName, "description", "Example of logs-new-value alert from terraform"),
					resource.TestCheckResourceAttr(alertResourceName, "priority", "P2"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_new_value.notification_payload_filter.#", "3"),
					resource.TestCheckTypeSetElemAttr(alertResourceName, "type_definition.logs_new_value.notification_payload_filter.*", "coralogix.metadata.sdkId"),
					resource.TestCheckTypeSetElemAttr(alertResourceName, "type_definition.logs_new_value.notification_payload_filter.*", "coralogix.metadata.sdkName"),
					resource.TestCheckTypeSetElemAttr(alertResourceName, "type_definition.logs_new_value.notification_payload_filter.*", "coralogix.metadata.sdkVersion"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_new_value.time_window.specific_value", "24_HOURS"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_new_value.keypath_to_track", "remote_addr_geoip.country_name"),
				),
			},
			{
				ResourceName: alertResourceName,
				ImportState:  true,
			},
			{
				Config: testAccCoralogixResourceAlertLogsNewValueUpdated(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(alertResourceName, "name", "logs-new-value alert example updated"),
					resource.TestCheckResourceAttr(alertResourceName, "description", "Example of logs-new-value alert from terraform updated"),
					resource.TestCheckResourceAttr(alertResourceName, "priority", "P3"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_new_value.notification_payload_filter.#", "0"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_new_value.time_window.specific_value", "12_HOURS"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_new_value.keypath_to_track", "remote_addr_geoip.city_name"),
				),
			},
		},
	},
	)
}

func TestAccCoralogixResourceAlert_logs_unique_count(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAlertDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertLogsUniqueCount(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(alertResourceName, "name", "logs-unique-count alert example"),
					resource.TestCheckResourceAttr(alertResourceName, "description", "Example of logs-unique-count alert from terraform"),
					resource.TestCheckResourceAttr(alertResourceName, "priority", "P2"),
					resource.TestCheckResourceAttr(alertResourceName, "group_by.#", "1"),
					resource.TestCheckTypeSetElemAttr(alertResourceName, "group_by.*", "remote_addr_geoip.city_name"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_unique_count.unique_count_keypath", "remote_addr_geoip.country_name"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_unique_count.max_unique_count", "2"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_unique_count.time_window.specific_value", "5_MINUTES"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_unique_count.max_unique_count_per_group_by_key", "500"),
				),
			},
			{
				ResourceName: alertResourceName,
				ImportState:  true,
			},
			{
				Config: testAccCoralogixResourceAlertLogsUniqueCountUpdated(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(alertResourceName, "name", "logs-unique-count alert example updated"),
					resource.TestCheckResourceAttr(alertResourceName, "description", "Example of logs-unique-count alert from terraform updated"),
					resource.TestCheckResourceAttr(alertResourceName, "priority", "P2"),
					resource.TestCheckResourceAttr(alertResourceName, "group_by.#", "0"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_unique_count.unique_count_keypath", "remote_addr_geoip.city_name"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_unique_count.max_unique_count", "5"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_unique_count.time_window.specific_value", "20_MINUTES"),
				),
			},
		},
	},
	)
}

func TestAccCoralogixResourceAlert_logs_time_relative_more_than(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAlertDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertLogsTimeRelativeMoreThan(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(alertResourceName, "name", "logs-time-relative-more-than alert example"),
					resource.TestCheckResourceAttr(alertResourceName, "description", "Example of logs-time-relative-more-than alert from terraform"),
					resource.TestCheckResourceAttr(alertResourceName, "priority", "P4"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_time_relative_more_than.threshold", "10"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_time_relative_more_than.compared_to", "Same Hour Yesterday"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_time_relative_more_than.ignore_infinity", "true"),
				),
			},
			{
				ResourceName: alertResourceName,
				ImportState:  true,
			},
			{
				Config: testAccCoralogixResourceAlertLogsTimeRelativeMoreThanUpdated(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(alertResourceName, "name", "logs-time-relative-more-than alert example updated"),
					resource.TestCheckResourceAttr(alertResourceName, "description", "Example of logs-time-relative-more-than alert from terraform updated"),
					resource.TestCheckResourceAttr(alertResourceName, "priority", "P3"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_time_relative_more_than.threshold", "50"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_time_relative_more_than.compared_to", "Same Day Last Week"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_time_relative_more_than.ignore_infinity", "false"),
				),
			},
		},
	},
	)
}

func TestAccCoralogixResourceAlert_logs_time_relative_less_than(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAlertDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertLogsTimeRelativeLessThan(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(alertResourceName, "name", "logs-time-relative-more-than alert example"),
					resource.TestCheckResourceAttr(alertResourceName, "description", "Example of logs-time-relative-more-than alert from terraform"),
					resource.TestCheckResourceAttr(alertResourceName, "priority", "P4"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_time_relative_less_than.threshold", "10"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_time_relative_less_than.compared_to", "Same Hour Yesterday"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_time_relative_less_than.ignore_infinity", "true"),
				),
			},
			{
				ResourceName: alertResourceName,
				ImportState:  true,
			},
			{
				Config: testAccCoralogixResourceAlertLogsTimeRelativeLessThanUpdated(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(alertResourceName, "name", "logs-time-relative-more-than alert example updated"),
					resource.TestCheckResourceAttr(alertResourceName, "description", "Example of logs-time-relative-more-than alert from terraform updated"),
					resource.TestCheckResourceAttr(alertResourceName, "priority", "P3"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_time_relative_less_than.threshold", "50"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_time_relative_less_than.compared_to", "Same Day Last Week"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_time_relative_less_than.ignore_infinity", "false"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_time_relative_less_than.undetected_values_management.trigger_undetected_values", "true"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_time_relative_less_than.undetected_values_management.auto_retire_timeframe", "6_Hours"),
				),
			},
		},
	},
	)
}

func TestAccCoralogixResourceAlert_metric_more_than(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAlertDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertMetricMoreThan(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(alertResourceName, "name", "metric-more-than alert example"),
					resource.TestCheckResourceAttr(alertResourceName, "description", "Example of metric-more-than alert from terraform"),
					resource.TestCheckResourceAttr(alertResourceName, "priority", "P3"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_more_than.metric_filter.promql", "sum(rate(http_requests_total{job=\"api-server\"}[5m])) by (status)"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_more_than.threshold", "2"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_more_than.for_over_pct", "10"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_more_than.of_the_last.specific_value", "10_MINUTES"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_more_than.missing_values.min_non_null_values_pct", "50"),
				),
			},
			{
				ResourceName: alertResourceName,
				ImportState:  true,
			},
			{
				Config: testAccCoralogixResourceAlertMetricMoreThanUpdated(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(alertResourceName, "name", "metric-more-than alert example updated"),
					resource.TestCheckResourceAttr(alertResourceName, "description", "Example of metric-more-than alert from terraform updated"),
					resource.TestCheckResourceAttr(alertResourceName, "priority", "P4"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_more_than.metric_filter.promql", "sum(rate(http_requests_total{job=\"api-server\"}[5m])) by (status)"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_more_than.threshold", "10"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_more_than.for_over_pct", "15"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_more_than.of_the_last.specific_value", "1_HOUR"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_more_than.missing_values.replace_with_zero", "true"),
				),
			},
		},
	},
	)
}

func TestAccCoralogixResourceAlert_metric_less_than(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAlertDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertMetricLessThan(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(alertResourceName, "name", "metric-less-than alert example"),
					resource.TestCheckResourceAttr(alertResourceName, "description", "Example of metric-less-than alert from terraform"),
					resource.TestCheckResourceAttr(alertResourceName, "priority", "P4"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_less_than.metric_filter.promql", "sum(rate(http_requests_total{job=\"api-server\"}[5m])) by (status)"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_less_than.threshold", "2"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_less_than.for_over_pct", "10"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_less_than.of_the_last.specific_value", "10_MINUTES"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_less_than.missing_values.replace_with_zero", "true"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_less_than.undetected_values_management.trigger_undetected_values", "true"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_less_than.undetected_values_management.auto_retire_timeframe", "5_Minutes"),
				),
			},
			{
				ResourceName: alertResourceName,
				ImportState:  true,
			},
			{
				Config: testAccCoralogixResourceAlertMetricLessThanUpdated(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(alertResourceName, "name", "metric-less-than alert example updated"),
					resource.TestCheckResourceAttr(alertResourceName, "description", "Example of metric-less-than alert from terraform updated"),
					resource.TestCheckResourceAttr(alertResourceName, "priority", "P3"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_less_than.metric_filter.promql", "sum(rate(http_requests_total{job=\"api-server\"}[5m])) by (status)"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_less_than.threshold", "5"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_less_than.for_over_pct", "15"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_less_than.of_the_last.specific_value", "10_MINUTES"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_less_than.missing_values.min_non_null_values_pct", "50"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_less_than.undetected_values_management.trigger_undetected_values", "true"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_less_than.undetected_values_management.auto_retire_timeframe", "5_Minutes"),
				),
			},
		},
	},
	)
}

func TestAccCoralogixResourceAlert_metric_less_than_usual(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAlertDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertMetricsLessThanUsual(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(alertResourceName, "name", "metric-less-than-usual alert example"),
					resource.TestCheckResourceAttr(alertResourceName, "description", "Example of metric-less-than-usual alert from terraform"),
					resource.TestCheckResourceAttr(alertResourceName, "priority", "P1"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_less_than_usual.metric_filter.promql", "sum(rate(http_requests_total{job=\"api-server\"}[5m])) by (status)"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_less_than_usual.threshold", "20"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_less_than_usual.for_over_pct", "10"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_less_than_usual.of_the_last.specific_value", "12_HOURS"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_less_than_usual.min_non_null_values_pct", "15"),
				),
			},
			{
				ResourceName: alertResourceName,
				ImportState:  true,
			},
			{
				Config: testAccCoralogixResourceAlertMetricsLessThanUsualUpdated(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(alertResourceName, "name", "metric-less-than-usual alert example updated"),
					resource.TestCheckResourceAttr(alertResourceName, "description", "Example of metric-less-than-usual alert from terraform updated"),
					resource.TestCheckResourceAttr(alertResourceName, "priority", "P1"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_less_than_usual.metric_filter.promql", "sum(rate(http_requests_total{job=\"api-server\"}[5m])) by (status)"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_less_than_usual.threshold", "2"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_less_than_usual.for_over_pct", "15"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_less_than_usual.of_the_last.specific_value", "10_MINUTES"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_less_than_usual.min_non_null_values_pct", "10"),
				),
			},
		},
	},
	)
}

func TestAccCoralogixResourceAlert_metric_more_than_usual(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAlertDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertMetricsMoreThanUsual(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(alertResourceName, "name", "metric_more_than_usual alert example"),
					resource.TestCheckResourceAttr(alertResourceName, "description", "Example of metric_more_than_usual alert from terraform"),
					resource.TestCheckResourceAttr(alertResourceName, "priority", "P2"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_more_than_usual.metric_filter.promql", "sum(rate(http_requests_total{job=\"api-server\"}[5m])) by (status)"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_more_than_usual.threshold", "2"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_more_than_usual.for_over_pct", "10"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_more_than_usual.of_the_last.specific_value", "10_MINUTES"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_more_than_usual.min_non_null_values_pct", "10"),
				),
			},
			{
				ResourceName: alertResourceName,
				ImportState:  true,
			},
			{
				Config: testAccCoralogixResourceAlertMetricsMoreThanUsualUpdated(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(alertResourceName, "name", "metric_more_than_usual alert example updated"),
					resource.TestCheckResourceAttr(alertResourceName, "description", "Example of metric_more_than_usual alert from terraform updated"),
					resource.TestCheckResourceAttr(alertResourceName, "priority", "P3"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_more_than_usual.metric_filter.promql", "sum(rate(http_requests_total{job=\"api-server\"}[5m])) by (status)"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_more_than_usual.threshold", "20"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_more_than_usual.for_over_pct", "10"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_more_than_usual.of_the_last.specific_value", "10_MINUTES"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_more_than_usual.min_non_null_values_pct", "10"),
				),
			},
		},
	},
	)
}

func TestAccCoralogixResourceAlert_metric_less_than_or_equals(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAlertDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertMetricLessThanOrEquals(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(alertResourceName, "name", "metric-less-than-or-equals alert example"),
					resource.TestCheckResourceAttr(alertResourceName, "description", "Example of metric-less-than-or-equals alert from terraform"),
					resource.TestCheckResourceAttr(alertResourceName, "priority", "P1"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_less_than_or_equals.metric_filter.promql", "sum(rate(http_requests_total{job=\"api-server\"}[5m])) by (status)"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_less_than_or_equals.threshold", "2"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_less_than_or_equals.for_over_pct", "10"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_less_than_or_equals.of_the_last.specific_value", "10_MINUTES"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_less_than_or_equals.missing_values.replace_with_zero", "true"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_less_than_or_equals.undetected_values_management.trigger_undetected_values", "true"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_less_than_or_equals.undetected_values_management.auto_retire_timeframe", "5_Minutes"),
				),
			},
			{
				ResourceName: alertResourceName,
				ImportState:  true,
			},
			{
				Config: testAccCoralogixResourceAlertMetricLessThanOrEqualsUpdated(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(alertResourceName, "name", "metric-less-than-or-equals alert example updated"),
					resource.TestCheckResourceAttr(alertResourceName, "description", "Example of metric-less-than-or-equals alert from terraform updated"),
					resource.TestCheckResourceAttr(alertResourceName, "priority", "P2"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_less_than_or_equals.metric_filter.promql", "sum(rate(http_requests_total{job=\"api-server\"}[5m])) by (status)"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_less_than_or_equals.threshold", "5"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_less_than_or_equals.for_over_pct", "15"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_less_than_or_equals.of_the_last.specific_value", "10_MINUTES"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_less_than_or_equals.missing_values.min_non_null_values_pct", "50"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_less_than_or_equals.undetected_values_management.trigger_undetected_values", "true"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_less_than_or_equals.undetected_values_management.auto_retire_timeframe", "5_Minutes"),
				),
			},
		},
	},
	)
}

func TestAccCoralogixResourceAlert_metric_more_than_or_equals(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAlertDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertMetricMoreThanOrEquals(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(alertResourceName, "name", "metric-more-than-or-equals alert example"),
					resource.TestCheckResourceAttr(alertResourceName, "description", "Example of metric-more-than-or-equals alert from terraform"),
					resource.TestCheckResourceAttr(alertResourceName, "priority", "P3"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_more_than_or_equals.metric_filter.promql", "sum(rate(http_requests_total{job=\"api-server\"}[5m])) by (status)"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_more_than_or_equals.threshold", "2"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_more_than_or_equals.for_over_pct", "10"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_more_than_or_equals.of_the_last.specific_value", "10_MINUTES"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_more_than_or_equals.missing_values.replace_with_zero", "true"),
				),
			},
			{
				ResourceName: alertResourceName,
				ImportState:  true,
			},
			{
				Config: testAccCoralogixResourceAlertMetricMoreThanOrEqualsUpdated(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(alertResourceName, "name", "metric-more-than-or-equals alert example updated"),
					resource.TestCheckResourceAttr(alertResourceName, "description", "Example of metric-more-than-or-equals alert from terraform updated"),
					resource.TestCheckResourceAttr(alertResourceName, "priority", "P4"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_more_than_or_equals.metric_filter.promql", "sum(rate(http_requests_total{job=\"api-server\"}[5m])) by (status)"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_more_than_or_equals.threshold", "10"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_more_than_or_equals.for_over_pct", "15"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_more_than_or_equals.of_the_last.specific_value", "1_HOUR"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_more_than_or_equals.missing_values.replace_with_zero", "true"),
				),
			},
		},
	},
	)
}

func TestAccCoralogixResourceAlert_tracing_immediate(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAlertDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertTracingImmediate(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(alertResourceName, "name", "tracing_immediate alert example"),
					resource.TestCheckResourceAttr(alertResourceName, "description", "Example of tracing_immediate alert from terraform"),
					resource.TestCheckResourceAttr(alertResourceName, "priority", "P1"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.tracing_immediate.tracing_query.latency_threshold_ms", "100"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.tracing_immediate.tracing_query.tracing_label_filters.application_name.#", "2"),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.tracing_immediate.tracing_query.tracing_label_filters.application_name.*",
						map[string]string{
							"operation": "IS",
							"values.#":  "2",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.tracing_immediate.tracing_query.tracing_label_filters.application_name.*",
						map[string]string{
							"operation": "STARTS_WITH",
							"values.#":  "1",
						},
					),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.tracing_immediate.tracing_query.tracing_label_filters.subsystem_name.#", "1"),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.tracing_immediate.tracing_query.tracing_label_filters.subsystem_name.*",
						map[string]string{
							"operation": "IS",
							"values.#":  "1",
						},
					),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.tracing_immediate.tracing_query.tracing_label_filters.operation_name.#", "1"),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.tracing_immediate.tracing_query.tracing_label_filters.operation_name.*",
						map[string]string{
							"operation": "IS",
							"values.#":  "1",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.tracing_immediate.tracing_query.tracing_label_filters.span_fields.*",
						map[string]string{
							"key":                   "status",
							"filter_type.operation": "IS",
							"filter_type.values.#":  "1",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.tracing_immediate.tracing_query.tracing_label_filters.span_fields.*",
						map[string]string{
							"key":                   "status",
							"filter_type.operation": "STARTS_WITH",
							"filter_type.values.#":  "2",
						},
					),
				),
			},
			{
				ResourceName: alertResourceName,
				ImportState:  true,
			},
			{
				Config: testAccCoralogixResourceAlertTracingImmediateUpdated(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(alertResourceName, "name", "tracing_immediate alert example updated"),
					resource.TestCheckResourceAttr(alertResourceName, "description", "Example of tracing_immediate alert from terraform updated"),
					resource.TestCheckResourceAttr(alertResourceName, "priority", "P2"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.tracing_immediate.tracing_query.latency_threshold_ms", "200"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.tracing_immediate.tracing_query.tracing_label_filters.application_name.#", "2"),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.tracing_immediate.tracing_query.tracing_label_filters.application_name.*",
						map[string]string{
							"operation": "IS",
							"values.#":  "2",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.tracing_immediate.tracing_query.tracing_label_filters.application_name.*",
						map[string]string{
							"operation": "STARTS_WITH",
							"values.#":  "1",
						},
					),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.tracing_immediate.tracing_query.tracing_label_filters.subsystem_name.#", "1"),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.tracing_immediate.tracing_query.tracing_label_filters.subsystem_name.*",
						map[string]string{
							"operation": "IS",
							"values.#":  "1",
						},
					),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.tracing_immediate.tracing_query.tracing_label_filters.operation_name.#", "1"),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.tracing_immediate.tracing_query.tracing_label_filters.operation_name.*",
						map[string]string{
							"operation": "IS",
							"values.#":  "1",
						},
					),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.tracing_immediate.tracing_query.tracing_label_filters.span_fields.#", "3"),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.tracing_immediate.tracing_query.tracing_label_filters.span_fields.*",
						map[string]string{
							"key":                   "status",
							"filter_type.operation": "STARTS_WITH",
							"filter_type.values.#":  "2",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.tracing_immediate.tracing_query.tracing_label_filters.span_fields.*",
						map[string]string{
							"key":                   "status",
							"filter_type.operation": "ENDS_WITH",
							"filter_type.values.#":  "2",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.tracing_immediate.tracing_query.tracing_label_filters.span_fields.*",
						map[string]string{
							"key":                   "status",
							"filter_type.operation": "IS",
							"filter_type.values.#":  "1",
						},
					),
				),
			},
		},
	})
}

func TestAccCoralogixResourceAlert_tracing_more_than(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAlertDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertTracingMoreThan(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(alertResourceName, "name", "tracing-more-than alert example"),
					resource.TestCheckResourceAttr(alertResourceName, "description", "Example of tracing_more_than alert from terraform"),
					resource.TestCheckResourceAttr(alertResourceName, "priority", "P2"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.tracing_more_than.tracing_query.latency_threshold_ms", "100"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.tracing_more_than.tracing_query.tracing_label_filters.application_name.#", "2"),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.tracing_more_than.tracing_query.tracing_label_filters.application_name.*",
						map[string]string{
							"operation": "IS",
							"values.#":  "2",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.tracing_more_than.tracing_query.tracing_label_filters.application_name.*",
						map[string]string{
							"operation": "STARTS_WITH",
							"values.#":  "1",
						},
					),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.tracing_more_than.span_amount", "5"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.tracing_more_than.time_window.specific_value", "10_MINUTES"),
				),
			},
			{
				ResourceName: alertResourceName,
				ImportState:  true,
			},
			{
				Config: testAccCoralogixResourceAlertTracingMoreThanUpdated(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(alertResourceName, "name", "tracing-more-than alert example updated"),
					resource.TestCheckResourceAttr(alertResourceName, "description", "Example of tracing-more-than alert from terraform updated"),
					resource.TestCheckResourceAttr(alertResourceName, "priority", "P3"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.tracing_more_than.tracing_query.latency_threshold_ms", "200"),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.tracing_more_than.tracing_query.tracing_label_filters.application_name.*",
						map[string]string{
							"operation": "IS",
							"values.#":  "2",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.tracing_more_than.tracing_label_filters.application_name.*",
						map[string]string{
							"operation": "STARTS_WITH",
							"values.#":  "1",
						},
					),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.tracing_more_than.span_amount", "5"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.tracing_more_than.time_window.specific_value", "1_HOUR"),
				),
			},
		},
	})
}

func TestAccCoralogixResourceAlert_flow(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAlertDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertFlow(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(alertResourceName, "name", "flow alert example"),
					resource.TestCheckResourceAttr(alertResourceName, "description", "Example of flow alert from terraform"),
					resource.TestCheckResourceAttr(alertResourceName, "priority", "P3"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.flow.stages.#", "2"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.flow.stages.0.flow_stages_groups.#", "2"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.flow.stages.0.flow_stages_groups.0.alerts_op", "OR"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.flow.stages.0.flow_stages_groups.0.next_op", "AND"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.flow.stages.0.flow_stages_groups.0.alert_defs.#", "2"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.flow.stages.0.timeframe_ms", "10"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.flow.stages.0.timeframe_type", "Up To"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.flow.stages.1.flow_stages_groups.#", "1"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.flow.stages.1.flow_stages_groups.0.alerts_op", "AND"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.flow.stages.1.flow_stages_groups.0.next_op", "OR"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.flow.stages.1.flow_stages_groups.0.alert_defs.#", "2"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.flow.stages.1.timeframe_ms", "10"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.flow.stages.1.timeframe_type", "Up To"),
				),
			},
			{
				ResourceName: alertResourceName,
				ImportState:  true,
			},
			{
				Config: testAccCoralogixResourceAlertFlowUpdated(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(alertResourceName, "name", "flow alert example updated"),
					resource.TestCheckResourceAttr(alertResourceName, "description", "Example of flow alert from terraform updated"),
					resource.TestCheckResourceAttr(alertResourceName, "priority", "P3"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.flow.stages.#", "2"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.flow.stages.0.flow_stages_groups.#", "1"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.flow.stages.0.flow_stages_groups.0.alerts_op", "AND"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.flow.stages.0.flow_stages_groups.0.next_op", "OR"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.flow.stages.0.alert_defs.#", "2"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.flow.stages.0.timeframe_ms", "10"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.flow.stages.0.timeframe_type", "Up To"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.flow.stages.1.flow_stages_groups.#", "1"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.flow.stages.1.flow_stages_groups.0.alerts_op", "OR"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.flow.stages.1.flow_stages_groups.0.next_op", "AND"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.flow.stages.1.alert_defs.#", "2"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.flow.stages.1.timeframe_ms", "10"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.flow.stages.1.timeframe_type", "Up To"),
				),
			},
		},
	})
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
                operation = "IS"
                value     = "nginx"
              }
            ]
            subsystem_name = [
              {
                operation = "IS"
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
                operation = "IS"
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
		  logs_filter       = {
			lucene_filter = {
			  lucene_query  = "message:\"error\""
			  label_filters = {
				application_name = [
				  {
					operation = "IS"
					value     = "nginx"
				  }
				]
				subsystem_name = [
				  {
					operation = "IS"
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
	  logs_filter       = {
		lucene_filter = {
		  lucene_query  = "message:\"error\""
		  label_filters = {
			application_name = [
			  {
				operation = "IS"
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
        integration_id = "17730"
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
                operation = "IS"
                value     = "nginx"
              }
            ]
            subsystem_name = [
              {
                operation = "IS"
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
        notify_on      = "Triggered and Resolved"
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
                operation = "IS"
                value     = "nginx"
              }
            ]
            subsystem_name = [
              {
                operation = "IS"
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
								operation = "STARTS_WITH"
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
			logs_filter       = {
				lucene_filter = {
					lucene_query  = "message:\"error\""
					label_filters = {
						application_name = [
							{
								operation = "IS"
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

func testAccCoralogixResourceAlertLogsRatioMoreThan() string {
	return `resource "coralogix_alert" "test" {
  name        = "logs-ratio-more-than alert example"
  description = "Example of logs-ratio-more-than alert from terraform"
  priority    = "P1"
  group_by        = ["coralogix.metadata.alert_id", "coralogix.metadata.alert_name"]

  notification_group = {
    simple_target_settings = [
      {
        recipients = ["example@coralogix.com"]
      }
    ]
  }

  type_definition = {
    logs_ratio_more_than = {
      denominator_alias = "denominator"
      denominator_logs_filter = {
        lucene_filter = {
          lucene_query  = "mod_date:[20020101 TO 20030101]"
          label_filters = {
            application_name = [
              {
                operation = "IS"
                value     = "nginx"
              }
            ]
            subsystem_name = [
              {
                operation = "IS"
                value     = "subsystem-name"
              }
            ]
            severities = ["Warning"]
          }
        }
      }
      numerator_alias   = "numerator"
      numerator_logs_filter = {
            lucene_filter = {
            lucene_query  = "mod_date:[20030101 TO 20040101]"
            label_filters = {
                application_name = [
                {
                    operation = "IS"
                    value     = "nginx"
                }
                ]
                subsystem_name = [
                {
                    operation = "IS"
                    value     = "subsystem-name"
                }
                ]
                severities = ["Error"]
            }
            }
        }
      time_window = {
        specific_value = "10_MINUTES"
      }
      threshold         = 2
    }
  }
}
`
}

func testAccCoralogixResourceAlertLogsRatioMoreThanUpdated() string {
	return `resource "coralogix_alert" "test" {
  name        = "logs-ratio-more-than alert example updated"
  description = "Example of logs-ratio-more-than alert from terraform updated"
  priority    = "P2"
  group_by    = ["coralogix.metadata.alert_id", "coralogix.metadata.alert_name", "coralogix.metadata.alert_description"]

  notification_group = {
    simple_target_settings = [
      {
        recipients = ["example@coralogix.com"]
      },
      {
            integration_id = "17730"
      }
    ]
  }

  type_definition = {
    logs_ratio_more_than = {
      denominator_alias       = "updated-denominator"
      denominator_logs_filter = {
        lucene_filter = {
          lucene_query  = "mod_date:[20030101 TO 20040101]"
          label_filters = {
            application_name = [
              {
                operation = "IS"
                value     = "nginx"
              }
            ]
            subsystem_name = [
              {
                operation = "IS"
                value     = "subsystem-name"
              }
            ]
            severities = ["Warning"]
          }
        }
      }
      numerator_alias       = "updated-numerator"
      numerator_logs_filter = {
        lucene_filter = {
          lucene_query  = "mod_date:[20040101 TO 20050101]"
          label_filters = {
            subsystem_name = [
              {
                operation = "ENDS_WITH"
                value     = "updated-subsystem-name"
              },
              {
                operation = "NOT"
                value     = "subsystem-name"
              }
            ]
          }
        }
      }
      time_window = {
        specific_value = "1_HOUR"
      }
      threshold = 120
      group_by_for = "Numerator Only"
    }
  }
}
`
}

func testAccCoralogixResourceAlertLogsRatioLessThan() string {
	return `resource "coralogix_alert" "test" {
	name        = "logs-ratio-less-than alert example"
  	description = "Example of logs-ratio-less-than alert from terraform"
  	priority    = "P3"

  	group_by        = ["coralogix.metadata.alert_id", "coralogix.metadata.alert_name"]
  	type_definition = {
    	logs_ratio_less_than = {
      		numerator_alias   = "numerator"
      		denominator_alias = "denominator"
      		threshold         = 2
      		time_window       = {
        		specific_value = "10_MINUTES"
      		}
      		group_by_for = "Denominator Only"
    	}
  	}
}
`
}

func testAccCoralogixResourceAlertLogsRatioLessThanUpdated() string {
	return `resource "coralogix_alert" "test" {
	name        = "logs-ratio-less-than alert example updated"
  	description = "Example of logs-ratio-less-than alert from terraform updated"
  	priority    = "P2"

  	type_definition = {
		logs_ratio_less_than = {
	  		numerator_alias   = "updated-numerator"
	  		denominator_alias = "updated-denominator"
	  		threshold         = 20
	  		time_window       = {
				specific_value = "2_HOURS"
	  		}
			undetected_values_management  = {
				trigger_undetected_values = true
				auto_retire_timeframe = "6_Hours"
			}
		}
  	}
}
`
}

func testAccCoralogixResourceAlertLogsNewValue() string {
	return `resource "coralogix_alert" "test" {
  name        = "logs-new-value alert example"
  description = "Example of logs-new-value alert from terraform"
  priority    = "P2"

  type_definition = {
    logs_new_value = {
      notification_payload_filter = ["coralogix.metadata.sdkId", "coralogix.metadata.sdkName", "coralogix.metadata.sdkVersion"]
      time_window = {
        specific_value = "24_HOURS"
      }
      keypath_to_track = "remote_addr_geoip.country_name"
    }
  }
}
`
}

func testAccCoralogixResourceAlertLogsNewValueUpdated() string {
	return `resource "coralogix_alert" "test" {
  name        = "logs-new-value alert example updated"
  description = "Example of logs-new-value alert from terraform updated"
  priority    = "P3"

  type_definition = {
	logs_new_value = {
	  time_window = {
		specific_value = "12_HOURS"
	  }
	  keypath_to_track = "remote_addr_geoip.city_name"
	}
  }
}
`
}

func testAccCoralogixResourceAlertLogsUniqueCount() string {
	return `resource "coralogix_alert" "test" {
  name        = "logs-unique-count alert example"
  description = "Example of logs-unique-count alert from terraform"
  priority    = "P2"

  group_by        = ["remote_addr_geoip.city_name"]
  type_definition = {
    logs_unique_count = {
      unique_count_keypath = "remote_addr_geoip.country_name"
      max_unique_count     = 2
      time_window          = {
        specific_value = "5_MINUTES"
      }
      max_unique_count_per_group_by_key = 500
    }
  }
}
`
}

func testAccCoralogixResourceAlertLogsUniqueCountUpdated() string {
	return `resource "coralogix_alert" "test" {
  name        = "logs-unique-count alert example updated"
  description = "Example of logs-unique-count alert from terraform updated"
  priority    = "P2"

  type_definition = {
    logs_unique_count = {
      unique_count_keypath = "remote_addr_geoip.city_name"
      max_unique_count     = 5
      time_window          = {
        specific_value = "20_MINUTES"
      }
    }
  }
}
`
}

func testAccCoralogixResourceAlertLogsTimeRelativeMoreThan() string {
	return `resource "coralogix_alert" "test" {
  name        = "logs-time-relative-more-than alert example"
  description = "Example of logs-time-relative-more-than alert from terraform"
  priority    = "P4"

  type_definition = {
    logs_time_relative_more_than = {
      threshold                   = 10
      compared_to                 = "Same Hour Yesterday"
	  ignore_infinity             = true
    }
  }
}
`
}

func testAccCoralogixResourceAlertLogsTimeRelativeMoreThanUpdated() string {
	return `resource "coralogix_alert" "test" {
  name        = "logs-time-relative-more-than alert example updated"
  description = "Example of logs-time-relative-more-than alert from terraform updated"
  priority    = "P3"

  type_definition = {
	logs_time_relative_more_than = {
	  threshold                   = 50
	  compared_to                 = "Same Day Last Week"
	}
  }
}
`
}

func testAccCoralogixResourceAlertLogsTimeRelativeLessThan() string {
	return `resource "coralogix_alert" "test" {
  name        = "logs-time-relative-more-than alert example"
  description = "Example of logs-time-relative-more-than alert from terraform"
  priority    = "P4"

  type_definition = {
	logs_time_relative_less_than = {
	  threshold                   = 10
	  compared_to                 = "Same Hour Yesterday"
	  ignore_infinity             = true
	}
  }
}
`
}

func testAccCoralogixResourceAlertLogsTimeRelativeLessThanUpdated() string {
	return `resource "coralogix_alert" "test" {
  name        = "logs-time-relative-more-than alert example updated"
  description = "Example of logs-time-relative-more-than alert from terraform updated"
  priority    = "P3"

  type_definition = {
	logs_time_relative_less_than = {
	  threshold                   = 50
	  compared_to                 = "Same Day Last Week"
	  ignore_infinity             = false
      undetected_values_management = {
        trigger_undetected_values = true
        auto_retire_timeframe     = "6_Hours"
	  }
	}
  }
}
`
}

func testAccCoralogixResourceAlertMetricMoreThan() string {
	return `resource "coralogix_alert" "test" {
  name        = "metric-more-than alert example"
  description = "Example of metric-more-than alert from terraform"
  priority    = "P3"

  type_definition = {
    metric_more_than = {
      metric_filter = {
        promql = "sum(rate(http_requests_total{job=\"api-server\"}[5m])) by (status)"
      }
      threshold    = 2
      for_over_pct = 10
      of_the_last  = {
        specific_value = "10_MINUTES"
      }
      missing_values = {
        min_non_null_values_pct = 50
      }
    }
  }
}
`
}

func testAccCoralogixResourceAlertMetricMoreThanUpdated() string {
	return `resource "coralogix_alert" "test" {
  name        = "metric-more-than alert example updated"
  description = "Example of metric-more-than alert from terraform updated"
  priority    = "P4"

  type_definition = {
    metric_more_than = {
      metric_filter = {
        promql = "sum(rate(http_requests_total{job=\"api-server\"}[5m])) by (status)"
      }
      threshold    = 10
      for_over_pct = 15
      of_the_last  = {
        specific_value = "1_HOUR"
      }
      missing_values = {
        replace_with_zero = true
      }
    }
  }
}
`
}

func testAccCoralogixResourceAlertMetricLessThan() string {
	return `resource "coralogix_alert" "test" {
  name        = "metric-less-than alert example"
  description = "Example of metric-less-than alert from terraform"
  priority    = "P4"

  type_definition = {
    metric_less_than = {
      metric_filter = {
        promql = "sum(rate(http_requests_total{job=\"api-server\"}[5m])) by (status)"
      }
      threshold    = 2
      for_over_pct = 10
      of_the_last  = {
        specific_value = "10_MINUTES"
      }
      missing_values = {
        replace_with_zero = true
      }
      undetected_values_management = {
        trigger_undetected_values = true
        auto_retire_timeframe     = "5_Minutes"
      }
    }
  }
}
`
}

func testAccCoralogixResourceAlertMetricLessThanUpdated() string {
	return `resource "coralogix_alert" "test" {
  name        = "metric-less-than alert example updated"
  description = "Example of metric-less-than alert from terraform updated"
  priority    = "P3"

  type_definition = {
    metric_less_than = {
      metric_filter = {
        promql = "sum(rate(http_requests_total{job=\"api-server\"}[5m])) by (status)"
      }
      threshold    = 5
      for_over_pct = 15
      of_the_last  = {
        specific_value = "10_MINUTES"
      }
      missing_values = {
        min_non_null_values_pct = 50
      }
      undetected_values_management = {
        trigger_undetected_values = true
        auto_retire_timeframe     = "5_Minutes"
      }
    }
  }
}
`
}

func testAccCoralogixResourceAlertMetricsLessThanUsual() string {
	return `resource "coralogix_alert" "test" {
  name        = "metric-less-than-usual alert example"
  description = "Example of metric-less-than-usual alert from terraform"
  priority    = "P1"

  type_definition = {
    metric_less_than_usual = {
      metric_filter = {
        promql = "sum(rate(http_requests_total{job=\"api-server\"}[5m])) by (status)"
      }
      threshold    = 2
      for_over_pct = 10
      of_the_last  = {
        specific_value = "12_HOURS"
      }
      threshold       = 20
      min_non_null_values_pct = 15
    }
  }
}
`
}

func testAccCoralogixResourceAlertMetricsLessThanUsualUpdated() string {
	return `resource "coralogix_alert" "test" {
  name        = "metric-less-than-usual alert example updated"
  description = "Example of metric-less-than-usual alert from terraform updated"
  priority    = "P1"

  type_definition = {
	metric_less_than_usual = {
	  metric_filter = {
		promql = "sum(rate(http_requests_total{job=\"api-server\"}[5m])) by (status)"
	  }
	  for_over_pct = 15
	  of_the_last  = {
		specific_value = "10_MINUTES"
	  }
	  threshold       = 2
	  min_non_null_values_pct = 10
	}
  }
}
`
}

func testAccCoralogixResourceAlertMetricsMoreThanUsual() string {
	return `resource "coralogix_alert" "test" {
  name        = "metric_more_than_usual alert example"
  description = "Example of metric_more_than_usual alert from terraform"
  priority    = "P2"

  type_definition = {
    metric_more_than_usual = {
      metric_filter = {
        promql = "sum(rate(http_requests_total{job=\"api-server\"}[5m])) by (status)"
      }
      threshold    = 2
      for_over_pct = 10
      of_the_last  = {
        specific_value = "10_MINUTES"
      }
      min_non_null_values_pct = 10
    }
  }
}
`
}

func testAccCoralogixResourceAlertMetricsMoreThanUsualUpdated() string {
	return `resource "coralogix_alert" "test" {
  name        = "metric_more_than_usual alert example updated"
  description = "Example of metric_more_than_usual alert from terraform updated"
  priority    = "P3"

  type_definition = {
	metric_more_than_usual = {
	  metric_filter = {
		promql = "sum(rate(http_requests_total{job=\"api-server\"}[5m])) by (status)"
	  }
	  threshold    = 20
	  for_over_pct = 10
	  of_the_last  = {
		specific_value = "10_MINUTES"
	  }
	  min_non_null_values_pct = 10
	}
  }
}
`
}

func testAccCoralogixResourceAlertMetricLessThanOrEquals() string {
	return `resource "coralogix_alert" "test" {
  name        = "metric-less-than-or-equals alert example"
  description = "Example of metric-less-than-or-equals alert from terraform"
  priority    = "P1"

  type_definition = {
	metric_less_than_or_equals = {
	  metric_filter = {
		promql = "sum(rate(http_requests_total{job=\"api-server\"}[5m])) by (status)"
	  }
	  threshold    = 2
	  for_over_pct = 10
	  of_the_last  = {
		specific_value = "10_MINUTES"
	  }
	  missing_values = {
		replace_with_zero = true
	  }
	  undetected_values_management = {
		trigger_undetected_values = true
		auto_retire_timeframe     = "5_Minutes"
	  }
	}
  }
}
`
}

func testAccCoralogixResourceAlertMetricLessThanOrEqualsUpdated() string {
	return `resource "coralogix_alert" "test" {
  name        = "metric-less-than-or-equals alert example updated"
  description = "Example of metric-less-than-or-equals alert from terraform updated"
  priority    = "P2"

  type_definition = {
	metric_less_than_or_equals = {
	  metric_filter = {
		promql = "sum(rate(http_requests_total{job=\"api-server\"}[5m])) by (status)"
	  }
	  threshold    = 5
	  for_over_pct = 15
	  of_the_last  = {
		specific_value = "10_MINUTES"
	  }
	  missing_values = {
		min_non_null_values_pct = 50
	  }
	  undetected_values_management = {
		trigger_undetected_values = true
		auto_retire_timeframe     = "5_Minutes"
	  }
	}
  }
}
`
}

func testAccCoralogixResourceAlertMetricMoreThanOrEquals() string {
	return `resource "coralogix_alert" "test" {
  name        = "metric-more-than-or-equals alert example"
  description = "Example of metric-more-than-or-equals alert from terraform"
  priority    = "P3"

  type_definition = {
	metric_more_than_or_equals = {
	  metric_filter = {
		promql = "sum(rate(http_requests_total{job=\"api-server\"}[5m])) by (status)"
	  }
	  threshold    = 2
	  for_over_pct = 10
	  of_the_last  = {
		specific_value = "10_MINUTES"
	  }
	  missing_values = {
		replace_with_zero = true
	  }
	}
  }
}
`
}

func testAccCoralogixResourceAlertMetricMoreThanOrEqualsUpdated() string {
	return `resource "coralogix_alert" "test" {
  name        = "metric-more-than-or-equals alert example updated"
  description = "Example of metric-more-than-or-equals alert from terraform updated"
  priority    = "P4"

  type_definition = {
	metric_more_than_or_equals = {
	  metric_filter = {
		promql = "sum(rate(http_requests_total{job=\"api-server\"}[5m])) by (status)"
	  }
	  threshold    = 10
	  for_over_pct = 15
	  of_the_last  = {
		specific_value = "1_HOUR"
	  }
	  missing_values = {
		replace_with_zero = true
	  }
	}
  }
}
`
}

func testAccCoralogixResourceAlertTracingImmediate() string {
	return `resource "coralogix_alert" "test" {
  name        = "tracing_immediate alert example"
  description = "Example of tracing_immediate alert from terraform"
  priority    = "P1"

  type_definition = {
    tracing_immediate = {
      tracing_query = {
        latency_threshold_ms  = 100
        tracing_label_filters = {
          application_name = [
            {
              operation = "IS"
              values    = ["nginx", "apache"]
            },
            {
                operation = "STARTS_WITH"
                values    = ["application-name:"]
            }
          ]
          subsystem_name = [
            {
              values    = ["subsystem-name"]
            }
          ]
          operation_name        = [
            {
              values    = ["operation-name"]
            }
          ]
          span_fields = [
            {
              key         = "status"
              filter_type = {
                values    = ["200"]
              }
            },
            {
              key         = "status"
              filter_type = {
                operation = "STARTS_WITH"
                values    = ["40", "50"]
              }
            },
          ]
        }
      }
    }
  }
}
`
}

func testAccCoralogixResourceAlertTracingImmediateUpdated() string {
	return `resource "coralogix_alert" "test" {
  name        = "tracing_immediate alert example updated"
  description = "Example of tracing_immediate alert from terraform updated"
  priority    = "P2"

  type_definition = {
	tracing_immediate = {
	  tracing_query = {
		latency_threshold_ms  = 200
		tracing_label_filters = {
		  application_name = [
			{
			  operation = "IS"
			  values    = ["nginx", "apache"]
			},
			{
				operation = "STARTS_WITH"
				values    = ["application-name:"]
			}
		  ]
		  subsystem_name = [
			{
			  operation = "IS"
			  values    = ["subsystem-name"]
			}
		  ]
		  operation_name        = [
            {
              operation = "IS"
              values    = ["operation-name"]
            }
          ]
		  span_fields = [
			{
			  key         = "status"
			  filter_type = {
				values    = ["200"]
			  }
			},
			{
			  key         = "status"
			  filter_type = {
				operation = "STARTS_WITH"
				values    = ["40", "50"]
			  }
			},
            {
              key         = "status"
              filter_type = {
                operation = "ENDS_WITH"
                values    = ["500", "404"]
              }
            },
		  ]
		}
	  }
	}
  }
}
`
}

func testAccCoralogixResourceAlertTracingMoreThan() string {
	return `resource "coralogix_alert" "test" {
  name        = "tracing_more_than alert example"
  description = "Example of tracing_more_than alert from terraform"
  priority    = "P2"

  type_definition = {
    tracing_more_than = {
      tracing_query = {
        latency_threshold_ms  = 100
        tracing_label_filters = {
          application_name = [
            {
              operation = "IS"
              values    = ["nginx", "apache"]
            },
            {
              operation = "STARTS_WITH"
              values    = ["application-name:"]
            }
          ]
        }
      }
      span_amount = 5
      time_window = {
        specific_value = "10_MINUTES"
      }
    }
  }
}
`
}

func testAccCoralogixResourceAlertTracingMoreThanUpdated() string {
	return `resource "coralogix_alert" "test" {
  name        = "tracing_more_than alert example updated"
  description = "Example of tracing_more_than alert from terraform updated"
  priority    = "P3"

  type_definition = {
	tracing_more_than = {
	  tracing_query = {
		latency_threshold_ms  = 200
		tracing_label_filters = {
		  application_name = [
			{
			  values    = ["nginx", "apache"]
			},
			{
				operation = "STARTS_WITH"	
				values    = ["application-name:"]
			}
		  ]
		}
	  }
	  span_amount = 5
	  time_window = {
		specific_value = "1_HOUR"
	  }
	}
  }
}
`
}

func testAccCoralogixResourceAlertFlow() string {
	return `resource "coralogix_alert" "test_1"{
  name        = "logs immediate alert 1"
  priority    = "P1"
  type_definition = {
    logs_immediate = {
    }
  }
}

resource "coralogix_alert" "test_2"{
  name        = "logs immediate alert 2"
  priority    = "P2"
  type_definition = {
    logs_immediate = {
    }
  }
}

resource "coralogix_alert" "test_3"{
  name        = "logs immediate alert 3"
  priority    = "P3"
  type_definition = {
    logs_immediate = {
    }
  }
}

resource "coralogix_alert" "test" {
  name        = "flow alert example"
  description = "Example of flow alert from terraform"
  priority    = "P3"
  type_definition = {
    flow = {
      stages = [
        {
          flow_stages_groups = [
            {
              alert_defs = [
                {
                  id = coralogix_alert.test_1.id
                },
                {
                  id = coralogix_alert.test_2.id
                },
              ]
              next_op   = "AND"
              alerts_op = "OR"
            },
            {
              alert_defs = [
                {
                  id = coralogix_alert.test_3.id
                },
                {
                  id = coralogix_alert.test_2.id
                },
              ]
              next_op   = "OR"
              alerts_op = "AND"
            },
          ]
          timeframe_ms   = 10
          timeframe_type = "Up To"
        }
      ]
    }
  }
}
`
}

func testAccCoralogixResourceAlertFlowUpdated() string {
	return `resource "coralogix_alert" "test_1"{
  name        = "logs immediate alert 1"
  priority    = "P1"
  type_definition = {
    logs_immediate = {
    }
  }
}

resource "coralogix_alert" "test_2"{
  name        = "logs immediate alert 2"
  priority    = "P2"
  type_definition = {
    logs_immediate = {
    }
  }
}

resource "coralogix_alert" "test_3"{
  name        = "logs immediate alert 3"
  priority    = "P3"
  type_definition = {
    logs_immediate = {
    }
  }
}

resource "coralogix_alert" "test" {
  name        = "flow alert example"
  description = "Example of flow alert from terraform"
  priority    = "P3"
  type_definition = {
    flow = {
      stages = [
        {
          flow_stages_groups = [
            {
              alert_defs = [
                {
                  id = coralogix_alert.test_2.id
                },
                {
                  id = coralogix_alert.test_1.id
                },
              ]
              next_op   = "OR"
              alerts_op = "AND"
            },
            {
              alert_defs = [
                {
                  id = coralogix_alert.test_2.id
                },
                {
                  id = coralogix_alert.test_3.id
                },
              ]
              next_op   = "AND"
              alerts_op = "OR"
            },
          ]
          timeframe_ms   = 10
          timeframe_type = "Up To"
        },
		{
          flow_stages_groups = [
            {
              alert_defs = [
                {
                  id = coralogix_alert.test_2.id
                },
              ]
              next_op   = "OR"
              alerts_op = "AND"
            },
          ]
          timeframe_ms   = 20
          timeframe_type = "Up To"
        }
      ]
    }
  }
}
`
}
