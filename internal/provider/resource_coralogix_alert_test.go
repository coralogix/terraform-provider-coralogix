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

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
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
					resource.TestCheckResourceAttr(alertResourceName, "priority", "P2"),
					resource.TestCheckResourceAttr(alertResourceName, "phantom_mode", "true"),
					resource.TestCheckResourceAttr(alertResourceName, "labels.alert_type", "security"),
					resource.TestCheckResourceAttr(alertResourceName, "labels.security_severity", "high"),
					resource.TestCheckResourceAttr(alertResourceName, "incidents_settings.notify_on", "Triggered and Resolved"),
					resource.TestCheckResourceAttr(alertResourceName, "incidents_settings.retriggering_period.minutes", "10"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.days_of_week.#", "2"),
					resource.TestCheckTypeSetElemAttr(alertResourceName, "schedule.active_on.days_of_week.*", "Wednesday"),
					resource.TestCheckTypeSetElemAttr(alertResourceName, "schedule.active_on.days_of_week.*", "Thursday"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.start_time", "08:30"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.end_time", "20:30"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.utc_offset", "+0300"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_immediate.logs_filter.simple_filter.lucene_query", "message:\"error\""),
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
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "notification_group.webhooks_settings.*",
						map[string]string{
							"retriggering_period.minutes": "1",
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
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.start_time", "09:30"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.end_time", "21:30"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.utc_offset", "+0300"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_immediate.logs_filter.simple_filter.lucene_query", "message:\"error\""),
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
					resource.TestCheckResourceAttr(alertResourceName, "notification_group.webhooks_settings.#", "1"),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "notification_group.webhooks_settings.*",
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
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.start_time", "08:30"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.end_time", "20:30"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_threshold.rules.#", "1"),

					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_threshold.rules.0.condition.threshold", "2"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_threshold.rules.0.condition.time_window", "10_MINUTES"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_threshold.rules.0.condition.condition_type", "MORE_THAN"),

					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_threshold.logs_filter.simple_filter.lucene_query", "message:\"error\""),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.logs_threshold.logs_filter.simple_filter.label_filters.application_name.*",
						map[string]string{
							"operation": "IS",
							"value":     "nginx",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.logs_threshold.logs_filter.simple_filter.label_filters.subsystem_name.*",
						map[string]string{
							"operation": "IS",
							"value":     "subsystem-name",
						},
					),
					resource.TestCheckTypeSetElemAttr(alertResourceName, "type_definition.logs_threshold.logs_filter.simple_filter.label_filters.severities.*", "Warning"),
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
					resource.TestCheckResourceAttr(alertResourceName, "notification_group.webhooks_settings.#", "1"),
					resource.TestCheckResourceAttr(alertResourceName, "incidents_settings.notify_on", "Triggered Only"),
					resource.TestCheckResourceAttr(alertResourceName, "incidents_settings.retriggering_period.minutes", "10"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.days_of_week.#", "2"),
					resource.TestCheckTypeSetElemAttr(alertResourceName, "schedule.active_on.days_of_week.*", "Monday"),
					resource.TestCheckTypeSetElemAttr(alertResourceName, "schedule.active_on.days_of_week.*", "Thursday"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.start_time", "08:30"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.end_time", "20:30"),

					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_threshold.rules.#", "1"),

					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_threshold.rules.0.condition.threshold", "20"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_threshold.rules.0.condition.time_window", "2_HOURS"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_threshold.rules.0.condition.condition_type", "MORE_THAN"),

					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_threshold.logs_filter.simple_filter.lucene_query", "message:\"error\""),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.logs_threshold.logs_filter.simple_filter.label_filters.application_name.*",
						map[string]string{
							"operation": "IS",
							"value":     "nginx",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.logs_threshold.logs_filter.simple_filter.label_filters.application_name.*",
						map[string]string{
							"operation": "INCLUDES",
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
					resource.TestCheckResourceAttr(alertResourceName, "name", "less-than alert example"),
					resource.TestCheckResourceAttr(alertResourceName, "description", "Example of logs-threshold less-than alert example from terraform"),
					resource.TestCheckResourceAttr(alertResourceName, "priority", "P2"),
					resource.TestCheckResourceAttr(alertResourceName, "labels.alert_type", "security"),
					resource.TestCheckResourceAttr(alertResourceName, "labels.security_severity", "high"),
					resource.TestCheckResourceAttr(alertResourceName, "notification_group.webhooks_settings.#", "1"),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "notification_group.webhooks_settings.*",
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
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.start_time", "08:30"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.end_time", "20:30"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_threshold.rules.#", "1"),

					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_threshold.rules.0.condition.threshold", "2"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_threshold.rules.0.condition.time_window", "10_MINUTES"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_threshold.rules.0.condition.condition_type", "LESS_THAN"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_threshold.logs_filter.simple_filter.lucene_query", "message:\"error\""),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.logs_threshold.logs_filter.simple_filter.label_filters.application_name.*",
						map[string]string{
							"operation": "IS",
							"value":     "nginx",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.logs_threshold.logs_filter.simple_filter.label_filters.subsystem_name.*",
						map[string]string{
							"operation": "IS",
							"value":     "subsystem-name",
						},
					),
					resource.TestCheckTypeSetElemAttr(alertResourceName, "type_definition.logs_threshold.logs_filter.simple_filter.label_filters.severities.*", "Warning"),
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
					resource.TestCheckResourceAttr(alertResourceName, "incidents_settings.notify_on", "Triggered Only"),
					resource.TestCheckResourceAttr(alertResourceName, "incidents_settings.retriggering_period.minutes", "10"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.days_of_week.#", "2"),
					resource.TestCheckTypeSetElemAttr(alertResourceName, "schedule.active_on.days_of_week.*", "Monday"),
					resource.TestCheckTypeSetElemAttr(alertResourceName, "schedule.active_on.days_of_week.*", "Thursday"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.start_time", "08:30"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.end_time", "20:30"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_threshold.rules.#", "1"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_threshold.rules.0.condition.threshold", "20"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_threshold.rules.0.condition.time_window", "2_HOURS"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_threshold.rules.0.condition.condition_type", "LESS_THAN"),

					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_threshold.logs_filter.simple_filter.lucene_query", "message:\"error\""),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.logs_threshold.logs_filter.simple_filter.label_filters.application_name.*",
						map[string]string{
							"operation": "IS",
							"value":     "nginx",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.logs_threshold.logs_filter.simple_filter.label_filters.application_name.*",
						map[string]string{
							"operation": "INCLUDES",
							"value":     "application_name",
						},
					),
				),
			},
		},
	},
	)

}

func TestAccCoralogixResourceAlert_logs_less_than_with_routing(t *testing.T) {
	name := uuid.NewString()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAlertDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertLogsLessThanWithRouter(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(alertResourceName, "name", "less-than alert example"),
					resource.TestCheckResourceAttr(alertResourceName, "description", "Example of logs-threshold less-than alert example from terraform"),
					resource.TestCheckResourceAttr(alertResourceName, "priority", "P2"),
					resource.TestCheckResourceAttr(alertResourceName, "labels.alert_type", "security"),
					resource.TestCheckResourceAttr(alertResourceName, "labels.security_severity", "high"),
					resource.TestCheckResourceAttr(alertResourceName, "notification_group.webhooks_settings.#", "1"),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "notification_group.webhooks_settings.*",
						map[string]string{
							"recipients.#": "1",
							"recipients.0": "example@coralogix.com",
						},
					),
					resource.TestCheckResourceAttr(alertResourceName, "notification_group.router.notify_on", "Triggered Only"),
					resource.TestCheckResourceAttr(alertResourceName, "incidents_settings.notify_on", "Triggered and Resolved"),
					resource.TestCheckResourceAttr(alertResourceName, "incidents_settings.retriggering_period.minutes", "1"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.days_of_week.#", "2"),
					resource.TestCheckTypeSetElemAttr(alertResourceName, "schedule.active_on.days_of_week.*", "Wednesday"),
					resource.TestCheckTypeSetElemAttr(alertResourceName, "schedule.active_on.days_of_week.*", "Thursday"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.start_time", "08:30"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.end_time", "20:30"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_threshold.rules.#", "1"),

					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_threshold.rules.0.condition.threshold", "2"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_threshold.rules.0.condition.time_window", "10_MINUTES"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_threshold.rules.0.condition.condition_type", "LESS_THAN"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_threshold.logs_filter.simple_filter.lucene_query", "message:\"error\""),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.logs_threshold.logs_filter.simple_filter.label_filters.application_name.*",
						map[string]string{
							"operation": "IS",
							"value":     "nginx",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.logs_threshold.logs_filter.simple_filter.label_filters.subsystem_name.*",
						map[string]string{
							"operation": "IS",
							"value":     "subsystem-name",
						},
					),
					resource.TestCheckTypeSetElemAttr(alertResourceName, "type_definition.logs_threshold.logs_filter.simple_filter.label_filters.severities.*", "Warning"),
				),
			},
			{
				ResourceName: alertResourceName,
				ImportState:  true,
			},
			{
				Config: testAccCoralogixResourceAlertLogsLessThanWithRoutingUpdated(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(alertResourceName, "name", "logs-less-than alert example updated"),
					resource.TestCheckResourceAttr(alertResourceName, "description", "Example of logs-less-than alert example from terraform updated"),
					resource.TestCheckResourceAttr(alertResourceName, "priority", "P3"),
					resource.TestCheckResourceAttr(alertResourceName, "labels.alert_type", "security"),
					resource.TestCheckResourceAttr(alertResourceName, "labels.security_severity", "low"),
					resource.TestCheckResourceAttr(alertResourceName, "incidents_settings.notify_on", "Triggered Only"),
					resource.TestCheckResourceAttr(alertResourceName, "incidents_settings.retriggering_period.minutes", "10"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.days_of_week.#", "2"),
					resource.TestCheckTypeSetElemAttr(alertResourceName, "schedule.active_on.days_of_week.*", "Monday"),
					resource.TestCheckTypeSetElemAttr(alertResourceName, "schedule.active_on.days_of_week.*", "Thursday"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.start_time", "08:30"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.end_time", "20:30"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_threshold.rules.#", "1"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_threshold.rules.0.condition.threshold", "20"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_threshold.rules.0.condition.time_window", "2_HOURS"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_threshold.rules.0.condition.condition_type", "LESS_THAN"),
					resource.TestCheckResourceAttr(alertResourceName, "notification_group.router.notify_on", "Triggered and Resolved"),

					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_threshold.logs_filter.simple_filter.lucene_query", "message:\"error\""),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.logs_threshold.logs_filter.simple_filter.label_filters.application_name.*",
						map[string]string{
							"operation": "IS",
							"value":     "nginx",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.logs_threshold.logs_filter.simple_filter.label_filters.application_name.*",
						map[string]string{
							"operation": "INCLUDES",
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
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "notification_group.webhooks_settings.*",
						map[string]string{
							"retriggering_period.minutes": "1",
							"notify_on":                   "Triggered and Resolved",
							"recipients.#":                "1",
							"recipients.0":                "example@coralogix.com",
						},
					),
					resource.TestCheckResourceAttr(alertResourceName, "incidents_settings.notify_on", "Triggered and Resolved"),
					resource.TestCheckResourceAttr(alertResourceName, "incidents_settings.retriggering_period.minutes", "1"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.days_of_week.#", "2"),
					resource.TestCheckTypeSetElemAttr(alertResourceName, "schedule.active_on.days_of_week.*", "Wednesday"),
					resource.TestCheckTypeSetElemAttr(alertResourceName, "schedule.active_on.days_of_week.*", "Thursday"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.start_time", "08:30"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.end_time", "20:30"),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.logs_anomaly.logs_filter.simple_filter.label_filters.application_name.*",
						map[string]string{
							"operation": "IS",
							"value":     "nginx",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.logs_anomaly.logs_filter.simple_filter.label_filters.subsystem_name.*",
						map[string]string{
							"operation": "IS",
							"value":     "subsystem-name",
						},
					),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_anomaly.logs_filter.simple_filter.lucene_query", "message:\"error\""),
					resource.TestCheckTypeSetElemAttr(alertResourceName, "type_definition.logs_anomaly.logs_filter.simple_filter.label_filters.severities.*", "Warning"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_anomaly.rules.#", "1"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_anomaly.rules.0.condition.minimum_threshold", "2"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_anomaly.rules.0.condition.time_window", "10_MINUTES"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_anomaly.percentage_of_deviation", "15.5"),

					resource.TestCheckTypeSetElemAttr(alertResourceName, "type_definition.logs_anomaly.notification_payload_filter.*", "coralogix.metadata.sdkId"),
					resource.TestCheckTypeSetElemAttr(alertResourceName, "type_definition.logs_anomaly.notification_payload_filter.*", "coralogix.metadata.sdkName"),
					resource.TestCheckTypeSetElemAttr(alertResourceName, "type_definition.logs_anomaly.notification_payload_filter.*", "coralogix.metadata.sdkVersion"),
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
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_anomaly.custom_evaluation_delay", "100"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_anomaly.percentage_of_deviation", "25.5"),
					resource.TestCheckResourceAttr(alertResourceName, "priority", "P1"),
					resource.TestCheckResourceAttr(alertResourceName, "labels.#", "0"),
					resource.TestCheckResourceAttr(alertResourceName, "notification_group.webhooks_settings.#", "1"),
					resource.TestCheckResourceAttr(alertResourceName, "incidents_settings.notify_on", "Triggered and Resolved"),
					resource.TestCheckResourceAttr(alertResourceName, "incidents_settings.retriggering_period.minutes", "1"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_anomaly.logs_filter.simple_filter.lucene_query", "message:\"updated_error\""),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.logs_anomaly.logs_filter.simple_filter.label_filters.application_name.*",
						map[string]string{
							"operation": "IS",
							"value":     "nginx",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.logs_anomaly.logs_filter.simple_filter.label_filters.subsystem_name.*",
						map[string]string{
							"operation": "IS",
							"value":     "subsystem-name",
						},
					),
					resource.TestCheckTypeSetElemAttr(alertResourceName, "type_definition.logs_anomaly.logs_filter.simple_filter.label_filters.severities.*", "Warning"),
					resource.TestCheckTypeSetElemAttr(alertResourceName, "type_definition.logs_anomaly.logs_filter.simple_filter.label_filters.severities.*", "Error"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_anomaly.rules.#", "1"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_anomaly.rules.0.condition.minimum_threshold", "20"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_anomaly.rules.0.condition.time_window", "1_HOUR"),
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
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "notification_group.webhooks_settings.*",
						map[string]string{
							"recipients.#": "2",
							"recipients.0": "example2@coralogix.com",
							"recipients.1": "example@coralogix.com",
						},
					),
					resource.TestCheckResourceAttr(alertResourceName, "incidents_settings.notify_on", "Triggered and Resolved"),
					resource.TestCheckResourceAttr(alertResourceName, "incidents_settings.retriggering_period.minutes", "1"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.days_of_week.#", "2"),
					resource.TestCheckTypeSetElemAttr(alertResourceName, "schedule.active_on.days_of_week.*", "Wednesday"),
					resource.TestCheckTypeSetElemAttr(alertResourceName, "schedule.active_on.days_of_week.*", "Thursday"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.start_time", "10:30"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.end_time", "20:30"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_threshold.rules.#", "1"),

					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_threshold.rules.0.condition.threshold", "2"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_threshold.rules.0.condition.time_window", "10_MINUTES"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_threshold.rules.0.condition.condition_type", "LESS_THAN"),

					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_threshold.logs_filter.simple_filter.lucene_query", "message:\"error\""),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.logs_threshold.logs_filter.simple_filter.label_filters.application_name.*",
						map[string]string{
							"operation": "INCLUDES",
							"value":     "application_name",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.logs_threshold.logs_filter.simple_filter.label_filters.subsystem_name.*",
						map[string]string{
							"operation": "STARTS_WITH",
							"value":     "subsystem-name",
						},
					),
					resource.TestCheckTypeSetElemAttr(alertResourceName, "type_definition.logs_threshold.logs_filter.simple_filter.label_filters.severities.*", "Warning"),
					resource.TestCheckTypeSetElemAttr(alertResourceName, "type_definition.logs_threshold.logs_filter.simple_filter.label_filters.severities.*", "Error"),
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
					resource.TestCheckResourceAttr(alertResourceName, "notification_group.webhooks_settings.#", "1"),
					resource.TestCheckResourceAttr(alertResourceName, "incidents_settings.notify_on", "Triggered Only"),
					resource.TestCheckResourceAttr(alertResourceName, "incidents_settings.retriggering_period.minutes", "10"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.days_of_week.#", "2"),
					resource.TestCheckTypeSetElemAttr(alertResourceName, "schedule.active_on.days_of_week.*", "Monday"),
					resource.TestCheckTypeSetElemAttr(alertResourceName, "schedule.active_on.days_of_week.*", "Thursday"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.start_time", "08:30"),
					resource.TestCheckResourceAttr(alertResourceName, "schedule.active_on.end_time", "20:30"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_threshold.rules.#", "1"),

					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_threshold.rules.0.condition.threshold", "20"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_threshold.rules.0.condition.time_window", "2_HOURS"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_threshold.rules.0.condition.condition_type", "LESS_THAN"),

					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_threshold.logs_filter.simple_filter.lucene_query", "message:\"error\""),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.logs_threshold.logs_filter.simple_filter.label_filters.application_name.*",
						map[string]string{
							"operation": "IS",
							"value":     "nginx",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.logs_threshold.logs_filter.simple_filter.label_filters.application_name.*",
						map[string]string{
							"operation": "INCLUDES",
							"value":     "application_name",
						},
					),
				),
			},
		},
	},
	)
}

func TestAccCoralogixResourceAlert_logs_ratio_threshold(t *testing.T) {
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
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "notification_group.webhooks_settings.*",
						map[string]string{
							"recipients.#": "1",
							"recipients.0": "example@coralogix.com",
						},
					),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_threshold.denominator_alias", "denominator"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_threshold.numerator_alias", "numerator"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_threshold.rules.#", "1"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_threshold.rules.0.condition.threshold", "2"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_threshold.rules.0.condition.time_window", "10_MINUTES"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_threshold.rules.0.condition.condition_type", "MORE_THAN"),

					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_threshold.denominator.simple_filter.lucene_query", "mod_date:[20020101 TO 20030101]"),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.logs_ratio_threshold.denominator.simple_filter.label_filters.application_name.*",
						map[string]string{
							"operation": "IS",
							"value":     "nginx",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.logs_ratio_threshold.denominator.simple_filter.label_filters.subsystem_name.*",
						map[string]string{
							"operation": "IS",
							"value":     "subsystem-name",
						},
					),
					resource.TestCheckTypeSetElemAttr(alertResourceName, "type_definition.logs_ratio_threshold.denominator.simple_filter.label_filters.severities.*", "Warning"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_threshold.numerator.simple_filter.lucene_query", "mod_date:[20030101 TO 20040101]"),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.logs_ratio_threshold.numerator.simple_filter.label_filters.application_name.*",
						map[string]string{
							"operation": "IS",
							"value":     "nginx",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.logs_ratio_threshold.numerator.simple_filter.label_filters.subsystem_name.*",
						map[string]string{
							"operation": "IS",
							"value":     "subsystem-name",
						},
					),
					resource.TestCheckTypeSetElemAttr(alertResourceName, "type_definition.logs_ratio_threshold.numerator.simple_filter.label_filters.severities.*", "Error"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_threshold.group_by_for", "Both"),
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
					resource.TestCheckResourceAttr(alertResourceName, "notification_group.webhooks_settings.#", "1"),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "notification_group.webhooks_settings.*",
						map[string]string{
							"recipients.#": "1",
							"recipients.0": "example@coralogix.com",
						},
					),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_threshold.denominator_alias", "updated-denominator"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_threshold.numerator_alias", "updated-numerator"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_threshold.rules.#", "1"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_threshold.rules.0.condition.time_window", "1_HOUR"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_threshold.rules.0.condition.threshold", "120"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_threshold.rules.0.condition.condition_type", "MORE_THAN"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_threshold.group_by_for", "Numerator Only"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_threshold.denominator.simple_filter.lucene_query", "mod_date:[20030101 TO 20040101]"),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.logs_ratio_threshold.denominator.simple_filter.label_filters.application_name.*",
						map[string]string{
							"operation": "IS",
							"value":     "nginx",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.logs_ratio_threshold.denominator.simple_filter.label_filters.subsystem_name.*",
						map[string]string{
							"operation": "IS",
							"value":     "subsystem-name",
						},
					),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_threshold.numerator.simple_filter.lucene_query", "mod_date:[20040101 TO 20050101]"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_threshold.numerator.simple_filter.label_filters.application_name.#", "0"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_threshold.numerator.simple_filter.label_filters.severities.#", "0"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_threshold.numerator.simple_filter.label_filters.subsystem_name.#", "2"),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.logs_ratio_threshold.numerator.simple_filter.label_filters.subsystem_name.*",
						map[string]string{
							"operation": "ENDS_WITH",
							"value":     "updated-subsystem-name",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.logs_ratio_threshold.numerator.simple_filter.label_filters.subsystem_name.*",
						map[string]string{
							"operation": "INCLUDES",
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
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_threshold.denominator_alias", "denominator"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_threshold.numerator_alias", "numerator"),

					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_threshold.rules.#", "1"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_threshold.rules.0.condition.time_window", "10_MINUTES"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_threshold.rules.0.condition.threshold", "2"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_threshold.rules.0.condition.condition_type", "LESS_THAN"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_threshold.group_by_for", "Denominator Only"),
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
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_threshold.denominator_alias", "updated-denominator"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_threshold.numerator_alias", "updated-numerator"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_threshold.rules.#", "1"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_threshold.rules.0.condition.time_window", "2_HOURS"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_threshold.rules.0.condition.threshold", "20"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_ratio_threshold.rules.0.condition.condition_type", "LESS_THAN"),
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
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_new_value.rules.#", "1"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_new_value.rules.0.condition.time_window", "24_HOURS"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_new_value.rules.0.condition.keypath_to_track", "remote_addr_geoip.country_name"),
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
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_new_value.rules.#", "1"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_new_value.rules.0.condition.time_window", "12_HOURS"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_new_value.rules.0.condition.keypath_to_track", "remote_addr_geoip.city_name"),
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
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_unique_count.rules.#", "1"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_unique_count.rules.0.condition.time_window", "5_MINUTES"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_unique_count.rules.0.condition.max_unique_count", "2"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_unique_count.unique_count_keypath", "remote_addr_geoip.country_name"),
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
					resource.TestCheckResourceAttr(alertResourceName, "group_by.#", "1"),

					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_unique_count.rules.#", "1"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_unique_count.rules.0.condition.time_window", "20_MINUTES"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_unique_count.rules.0.condition.max_unique_count", "5"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_unique_count.max_unique_count_per_group_by_key", "500"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_unique_count.unique_count_keypath", "remote_addr_geoip.city_name"),
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
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_time_relative_threshold.rules.#", "1"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_time_relative_threshold.rules.0.condition.threshold", "10"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_time_relative_threshold.rules.0.condition.compared_to", "Same Hour Yesterday"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_time_relative_threshold.rules.0.condition.condition_type", "MORE_THAN"),
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
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_time_relative_threshold.rules.#", "1"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_time_relative_threshold.rules.0.condition.threshold", "50"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_time_relative_threshold.rules.0.condition.compared_to", "Same Day Last Week"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_time_relative_threshold.rules.0.condition.condition_type", "MORE_THAN"),
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
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_time_relative_threshold.custom_evaluation_delay", "100"),

					resource.TestCheckResourceAttr(alertResourceName, "priority", "P4"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_time_relative_threshold.rules.#", "1"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_time_relative_threshold.rules.0.condition.threshold", "10"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_time_relative_threshold.rules.0.condition.compared_to", "Same Hour Yesterday"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_time_relative_threshold.rules.0.condition.condition_type", "LESS_THAN"),
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
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_time_relative_threshold.undetected_values_management.trigger_undetected_values", "true"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_time_relative_threshold.undetected_values_management.auto_retire_timeframe", "6_HOURS"),

					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_time_relative_threshold.rules.#", "1"),

					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_time_relative_threshold.rules.0.condition.threshold", "50"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_time_relative_threshold.rules.0.condition.compared_to", "Same Day Last Week"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.logs_time_relative_threshold.rules.0.condition.condition_type", "LESS_THAN"),
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
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_threshold.metric_filter.promql", "sum(rate(http_requests_total{job=\"api-server\"}[5m])) by (status)"),

					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_threshold.rules.#", "1"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_threshold.custom_evaluation_delay", "100"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_threshold.missing_values.min_non_null_values_pct", "50"),

					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_threshold.rules.0.condition.threshold", "2"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_threshold.rules.0.condition.of_the_last", "10_MINUTES"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_threshold.rules.0.condition.for_over_pct", "10"),
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
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_threshold.metric_filter.promql", "sum(rate(http_requests_total{job=\"api-server\"}[5m])) by (status)"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_threshold.rules.#", "1"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_threshold.missing_values.replace_with_zero", "true"),

					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_threshold.rules.0.condition.threshold", "10"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_threshold.rules.0.condition.of_the_last", "1h15m"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_threshold.rules.0.condition.for_over_pct", "15"),
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
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_threshold.metric_filter.promql", "sum(rate(http_requests_total{job=\"api-server\"}[5m])) by (status)"),

					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_threshold.rules.#", "1"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_threshold.missing_values.replace_with_zero", "true"),

					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_threshold.rules.0.condition.threshold", "2"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_threshold.rules.0.condition.of_the_last", "10_MINUTES"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_threshold.rules.0.condition.for_over_pct", "10"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_threshold.undetected_values_management.trigger_undetected_values", "true"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_threshold.undetected_values_management.auto_retire_timeframe", "5_MINUTES"),
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
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_threshold.metric_filter.promql", "sum(rate(http_requests_total{job=\"api-server\"}[5m])) by (status)"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_threshold.undetected_values_management.trigger_undetected_values", "true"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_threshold.undetected_values_management.auto_retire_timeframe", "5_MINUTES"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_threshold.missing_values.min_non_null_values_pct", "50"),

					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_threshold.rules.#", "1"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_threshold.rules.0.condition.threshold", "5"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_threshold.rules.0.condition.of_the_last", "10m"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_threshold.rules.0.condition.for_over_pct", "15"),
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
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_anomaly.metric_filter.promql", "sum(rate(http_requests_total{job=\"api-server\"}[5m])) by (status)"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_anomaly.percentage_of_deviation", "20"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_anomaly.rules.#", "1"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_anomaly.rules.0.condition.threshold", "20"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_anomaly.rules.0.condition.of_the_last", "12_HOURS"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_anomaly.rules.0.condition.for_over_pct", "10"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_anomaly.rules.0.condition.min_non_null_values_pct", "50"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_anomaly.rules.0.condition.condition_type", "LESS_THAN"),
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
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_anomaly.metric_filter.promql", "sum(rate(http_requests_total{job=\"api-server\"}[5m])) by (status)"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_anomaly.percentage_of_deviation", "30"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_anomaly.rules.#", "1"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_anomaly.rules.0.condition.threshold", "2"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_anomaly.rules.0.condition.of_the_last", "10_MINUTES"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_anomaly.rules.0.condition.for_over_pct", "10"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_anomaly.rules.0.condition.min_non_null_values_pct", "50"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_anomaly.rules.0.condition.condition_type", "LESS_THAN"),
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
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_threshold.metric_filter.promql", "sum(rate(http_requests_total{job=\"api-server\"}[5m])) by (status)"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_threshold.rules.#", "1"),

					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_threshold.rules.0.condition.threshold", "2"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_threshold.rules.0.condition.of_the_last", "10_MINUTES"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_threshold.rules.0.condition.for_over_pct", "10"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_threshold.rules.0.condition.condition_type", "LESS_THAN_OR_EQUALS"),

					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_threshold.missing_values.replace_with_zero", "true"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_threshold.undetected_values_management.trigger_undetected_values", "true"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_threshold.undetected_values_management.auto_retire_timeframe", "5_MINUTES"),
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
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_threshold.metric_filter.promql", "sum(rate(http_requests_total{job=\"api-server\"}[5m])) by (status)"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_threshold.rules.#", "1"),

					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_threshold.rules.0.condition.threshold", "5"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_threshold.rules.0.condition.of_the_last", "10_MINUTES"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_threshold.rules.0.condition.for_over_pct", "15"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_threshold.rules.0.condition.condition_type", "LESS_THAN_OR_EQUALS"),

					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_threshold.missing_values.min_non_null_values_pct", "50"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_threshold.undetected_values_management.trigger_undetected_values", "true"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_threshold.undetected_values_management.auto_retire_timeframe", "5_MINUTES"),
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
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_threshold.metric_filter.promql", "sum(rate(http_requests_total{job=\"api-server\"}[5m])) by (status)"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_threshold.rules.#", "1"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_threshold.missing_values.replace_with_zero", "true"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_threshold.rules.0.override.priority", "P2"),

					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_threshold.rules.0.condition.threshold", "2"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_threshold.rules.0.condition.of_the_last", "10_MINUTES"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_threshold.rules.0.condition.for_over_pct", "10"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_threshold.rules.0.condition.condition_type", "MORE_THAN_OR_EQUALS"),
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
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_threshold.metric_filter.promql", "sum(rate(http_requests_total{job=\"api-server\"}[5m])) by (status)"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_threshold.rules.#", "1"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_threshold.missing_values.replace_with_zero", "true"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_threshold.rules.0.override.priority", "P2"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_threshold.rules.0.condition.threshold", "10"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_threshold.rules.0.condition.of_the_last", "1_HOUR"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_threshold.rules.0.condition.for_over_pct", "15"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.metric_threshold.rules.0.condition.condition_type", "MORE_THAN_OR_EQUALS"),
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
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.tracing_immediate.tracing_filter.latency_threshold_ms", "100"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.tracing_immediate.tracing_filter.tracing_label_filters.application_name.#", "2"),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.tracing_immediate.tracing_filter.tracing_label_filters.application_name.*",
						map[string]string{
							"operation": "IS",
							"values.#":  "2",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.tracing_immediate.tracing_filter.tracing_label_filters.application_name.*",
						map[string]string{
							"operation": "STARTS_WITH",
							"values.#":  "1",
						},
					),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.tracing_immediate.tracing_filter.tracing_label_filters.subsystem_name.#", "1"),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.tracing_immediate.tracing_filter.tracing_label_filters.subsystem_name.*",
						map[string]string{
							"operation": "IS",
							"values.#":  "1",
						},
					),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.tracing_immediate.tracing_filter.tracing_label_filters.operation_name.#", "1"),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.tracing_immediate.tracing_filter.tracing_label_filters.operation_name.*",
						map[string]string{
							"operation": "IS",
							"values.#":  "1",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.tracing_immediate.tracing_filter.tracing_label_filters.span_fields.*",
						map[string]string{
							"key":                   "status",
							"filter_type.operation": "IS",
							"filter_type.values.#":  "1",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.tracing_immediate.tracing_filter.tracing_label_filters.span_fields.*",
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
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.tracing_immediate.tracing_filter.latency_threshold_ms", "200"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.tracing_immediate.tracing_filter.tracing_label_filters.application_name.#", "2"),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.tracing_immediate.tracing_filter.tracing_label_filters.application_name.*",
						map[string]string{
							"operation": "IS",
							"values.#":  "2",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.tracing_immediate.tracing_filter.tracing_label_filters.application_name.*",
						map[string]string{
							"operation": "STARTS_WITH",
							"values.#":  "1",
						},
					),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.tracing_immediate.tracing_filter.tracing_label_filters.subsystem_name.#", "1"),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.tracing_immediate.tracing_filter.tracing_label_filters.subsystem_name.*",
						map[string]string{
							"operation": "IS",
							"values.#":  "1",
						},
					),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.tracing_immediate.tracing_filter.tracing_label_filters.operation_name.#", "1"),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.tracing_immediate.tracing_filter.tracing_label_filters.operation_name.*",
						map[string]string{
							"operation": "IS",
							"values.#":  "1",
						},
					),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.tracing_immediate.tracing_filter.tracing_label_filters.span_fields.#", "3"),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.tracing_immediate.tracing_filter.tracing_label_filters.span_fields.*",
						map[string]string{
							"key":                   "status",
							"filter_type.operation": "STARTS_WITH",
							"filter_type.values.#":  "2",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.tracing_immediate.tracing_filter.tracing_label_filters.span_fields.*",
						map[string]string{
							"key":                   "status",
							"filter_type.operation": "ENDS_WITH",
							"filter_type.values.#":  "2",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.tracing_immediate.tracing_filter.tracing_label_filters.span_fields.*",
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
					resource.TestCheckResourceAttr(alertResourceName, "name", "tracing_more_than alert example"),
					resource.TestCheckResourceAttr(alertResourceName, "description", "Example of tracing_more_than alert from terraform"),
					resource.TestCheckResourceAttr(alertResourceName, "priority", "P2"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.tracing_threshold.tracing_filter.latency_threshold_ms", "100"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.tracing_threshold.tracing_filter.tracing_label_filters.application_name.#", "2"),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.tracing_threshold.tracing_filter.tracing_label_filters.application_name.*",
						map[string]string{
							"operation": "IS",
							"values.#":  "2",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.tracing_threshold.tracing_filter.tracing_label_filters.application_name.*",
						map[string]string{
							"operation": "STARTS_WITH",
							"values.#":  "1",
						},
					),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.tracing_threshold.rules.#", "1"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.tracing_threshold.rules.0.condition.time_window", "10_MINUTES"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.tracing_threshold.rules.0.condition.span_amount", "5"),
				),
			},
			{
				ResourceName: alertResourceName,
				ImportState:  true,
			},
			{
				Config: testAccCoralogixResourceAlertTracingMoreThanUpdated(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(alertResourceName, "name", "tracing_more_than alert example updated"),
					resource.TestCheckResourceAttr(alertResourceName, "description", "Example of tracing_more_than alert from terraform updated"),
					resource.TestCheckResourceAttr(alertResourceName, "priority", "P3"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.tracing_threshold.tracing_filter.latency_threshold_ms", "200"),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.tracing_threshold.tracing_filter.tracing_label_filters.application_name.*",
						map[string]string{
							"operation": "IS",
							"values.#":  "2",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "type_definition.tracing_threshold.tracing_filter.tracing_label_filters.application_name.*",
						map[string]string{
							"operation": "STARTS_WITH",
							"values.#":  "1",
						},
					),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.tracing_threshold.rules.#", "1"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.tracing_threshold.rules.0.condition.time_window", "1_HOUR"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.tracing_threshold.rules.0.condition.span_amount", "5"),
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
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.flow.stages.#", "1"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.flow.stages.0.flow_stages_groups.#", "2"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.flow.stages.0.flow_stages_groups.0.alerts_op", "OR"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.flow.stages.0.flow_stages_groups.0.next_op", "AND"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.flow.stages.0.flow_stages_groups.0.alert_defs.#", "2"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.flow.stages.0.timeframe_ms", "10"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.flow.stages.0.timeframe_type", "Up To"),
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
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.flow.stages.0.flow_stages_groups.#", "2"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.flow.stages.0.flow_stages_groups.0.alerts_op", "AND"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.flow.stages.0.flow_stages_groups.0.next_op", "OR"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.flow.stages.0.flow_stages_groups.0.alert_defs.#", "2"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.flow.stages.0.timeframe_ms", "10"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.flow.stages.0.timeframe_type", "Up To"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.flow.stages.1.flow_stages_groups.#", "1"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.flow.stages.1.flow_stages_groups.0.alerts_op", "AND"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.flow.stages.1.flow_stages_groups.0.next_op", "OR"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.flow.stages.1.flow_stages_groups.0.alert_defs.#", "1"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.flow.stages.1.timeframe_ms", "20"),
					resource.TestCheckResourceAttr(alertResourceName, "type_definition.flow.stages.1.timeframe_type", "Up To"),
				),
			},
		},
	})
}

func TestAccCoralogixResourceAlert_sloBurnRate(t *testing.T) {
	t.Skip("Skipping SLO v2 for now")
	sloName := "coralogix_slo_go_example"
	alertName := "SLO burn rate alert"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAlertDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertSloBurnRate(sloName, alertName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("coralogix_alert.slo_alert_burn_rate", "name", alertName),
					resource.TestCheckResourceAttr("coralogix_alert.slo_alert_burn_rate", "description", "Alert based on SLO burn rate threshold"),
					resource.TestCheckResourceAttr("coralogix_alert.slo_alert_burn_rate", "priority", "P1"),
					resource.TestCheckResourceAttr("coralogix_alert.slo_alert_burn_rate", "labels.alert_type", "security"),
					resource.TestCheckResourceAttr("coralogix_alert.slo_alert_burn_rate", "labels.security_severity", "high"),
					resource.TestCheckResourceAttr("coralogix_alert.slo_alert_burn_rate", "notification_group.webhooks_settings.#", "1"),
					resource.TestCheckTypeSetElemAttr("coralogix_alert.slo_alert_burn_rate", "notification_group.webhooks_settings.*.recipients.*", "example@coralogix.com"),
					resource.TestCheckResourceAttr("coralogix_alert.slo_alert_burn_rate", "schedule.active_on.days_of_week.#", "2"),
					resource.TestCheckResourceAttr("coralogix_alert.slo_alert_burn_rate", "schedule.active_on.start_time", "08:30"),
					resource.TestCheckResourceAttr("coralogix_alert.slo_alert_burn_rate", "schedule.active_on.end_time", "20:30"),
					resource.TestCheckResourceAttr("coralogix_alert.slo_alert_burn_rate", "type_definition.slo_threshold.burn_rate.rules.#", "2"),
					resource.TestCheckResourceAttr("coralogix_alert.slo_alert_burn_rate", "type_definition.slo_threshold.burn_rate.rules.0.condition.threshold", "1"),
					resource.TestCheckResourceAttr("coralogix_alert.slo_alert_burn_rate", "type_definition.slo_threshold.burn_rate.rules.1.condition.threshold", "1.3"),
				),
			},
			{
				ResourceName: "coralogix_alert.slo_alert_burn_rate",
				ImportState:  true,
			},
		},
	})
}

func testAccCheckAlertDestroy(s *terraform.State) error {
	meta := testAccProvider.Meta()
	if meta == nil {
		return nil
	}
	client := meta.(*clientset.ClientSet).Alerts()

	ctx := context.TODO()

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "coralogix_alert" {
			continue
		}

		resp, _, err := client.AlertDefsServiceGetAlertDef(ctx, rs.Primary.ID).Execute()
		if err == nil {
			if *resp.AlertDef.Id == rs.Primary.ID {
				return fmt.Errorf("alert still exists: %s", rs.Primary.ID)
			}
		}
	}

	return nil
}

func testAccCoralogixResourceAlertLogsImmediateUpdated() string {
	return `resource "coralogix_alert" "test" {
  name        = "logs immediate alert updated"
  description = "Example of logs immediate alert from terraform updated"
  priority    = "P2"

  labels = {
    alert_type        = "security"
    security_severity = "high"
  }

  notification_group = {
    webhooks_settings = [
      {
        retriggering_period = {
          minutes = 1
        }
        notify_on  = "Triggered Only"
        recipients = ["example@coralogix.com"]
      }
    ]
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
      start_time = "09:30"
      end_time   = "21:30"
      utc_offset = "+0300"
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
`
}

func testAccCoralogixResourceAlertLogsImmediate() string {
	return `resource "coralogix_alert" "test" {
  name        = "logs immediate alert"
  description = "Example of logs immediate alert from terraform"
  priority    = "P2"
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
      start_time = "08:30"
      end_time = "20:30"
      utc_offset = "+0300"
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
    webhooks_settings = [
      {
        notify_on = "Triggered Only"
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
      start_time = "08:30"
      end_time = "20:30"
    }
  }

  type_definition = {
    logs_threshold = {
        rules = [{
            condition = {
                threshold   = 2
                time_window = "10_MINUTES"
                condition_type = "MORE_THAN"
            }
            override = {
                priority = "P2"
            }
        }]
      logs_filter       = {
        simple_filter = {
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
    webhooks_settings = [
      {
        notify_on = "Triggered Only"
        recipients = ["example@coralogix.com"]
      }
    ]
  }

  incidents_settings = {
    notify_on = "Triggered Only"
    retriggering_period = {
      minutes = 10
    }
  }

  schedule = {
    active_on = {
      days_of_week = ["Monday", "Thursday"]
      start_time = "08:30"
      end_time = "20:30"
    }
  }

  type_definition = {
    logs_threshold = {
      rules = [
        {
          condition = {
            threshold   = 20
              time_window = "2_HOURS"
              condition_type = "MORE_THAN"
            }
          override = {
              priority = "P2"
            }
        }
      ]

      logs_filter = {
        simple_filter = {
          lucene_query = "message:\"error\""
          label_filters = {
            application_name = [
              {
                operation = "IS"
                value     = "nginx"
              },
              {
                operation = "INCLUDES"
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
  name        = "less-than alert example"
  description = "Example of logs-threshold less-than alert example from terraform"
  priority    = "P2"

  labels = {
    alert_type        = "security"
    security_severity = "high"
  }

  notification_group = {
    webhooks_settings = [
      {
        notify_on = "Triggered Only"
        recipients = ["example@coralogix.com"]
      }
    ]
  }

  incidents_settings = {
    notify_on = "Triggered and Resolved"
    retriggering_period = {
      minutes = 1
    }
  }

  schedule = {
    active_on = {
      days_of_week = ["Wednesday", "Thursday"]
      start_time = "08:30"
      end_time = "20:30"
    }
  }

  type_definition = {
    logs_threshold = {
      logs_filter = {
        simple_filter = {
          lucene_query = "message:\"error\""
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
      rules = [
        {
         condition = {
              threshold   = 2
              time_window = "10_MINUTES"
              condition_type   = "LESS_THAN"
            }
          override = {
            priority = "P2"
          }
        }
      ]
    }
  }
}
`
}

func testAccCoralogixResourceAlertLogsLessThanWithRoutingUpdated(name string) string {
	return fmt.Sprintf(`
  resource "coralogix_connector" "slack_example" {
    id               = "%[1]v"
    name             = "%[1]v"
    type             = "slack"
    description      = "slack connector example"
    connector_config = {
      fields = [
        {
          field_name = "integrationId"
          value      = "luigis-testing-grounds"
        },
        {
          field_name = "fallbackChannel"
          value      = "luigis-testing-grounds"
        },
        {
          field_name = "channel"
          value      = "luigis-testing-grounds"
        }
      ]
    }
  }
  
  resource "coralogix_preset" "slack_example" {
    id               = "%[1]v"
    name             = "%[1]v"
    description      = "slack preset example"
    entity_type      = "alerts"
    connector_type   = "slack"
    parent_id        = "preset_system_slack_alerts_basic"
    config_overrides = [
      {
        condition_type = {
          match_entity_type_and_sub_type = {
            entity_sub_type    = "logsImmediateResolved"
          }
        }
        message_config =    {
          fields = [
            {
              field_name = "title"
              template   = "{{alert.status}} {{alertDef.priority}} - {{alertDef.name}}"
            },
            {
              field_name = "description"
              template   = "{{alertDef.description}}"
            }
          ]
        }
      }
    ]
  }

  resource "coralogix_global_router" "example" {
    name        = "%[1]v"
    description = "global router example"
    routing_labels = {
      environment = "%[1]v"
    }
    rules       = [
      {
        entity_type = "alerts"
        name = "rule-name"
        condition = "alertDef.priority == \"P1\""
        targets = [
          {
            connector_id   = coralogix_connector.slack_example.id
            preset_id      = coralogix_preset.slack_example.id
          }
        ]
      }
    ]
  }

  resource "coralogix_alert" "test" {
  depends_on = [coralogix_global_router.example]
  name        = "logs-less-than alert example updated"
  description = "Example of logs-less-than alert example from terraform updated"
  priority    = "P3"

  labels = {
    "alert_type"        = "security"
    "security_severity" = "low"
    "environment" = "production"
  }

  notification_group = {
    router = {
      notify_on = "Triggered and Resolved"
    }
  }

  incidents_settings = {
    notify_on = "Triggered Only"
    retriggering_period = {
      minutes = 10
    }
  }

  schedule = {
    active_on = {
      days_of_week = ["Monday", "Thursday"]
      start_time = "08:30"
      end_time = "20:30"
    }
  }

  type_definition = {
    logs_threshold = {
      rules = [
        {
        condition = {
            threshold   = 20
            time_window = "2_HOURS"
            condition_type   = "LESS_THAN"
            }
        override = {
            priority = "P2"
        }
        }
      ]
      logs_filter = {
        simple_filter = {
          lucene_query = "message:\"error\""
          label_filters = {
            application_name = [
              {
                operation = "IS"
                value     = "nginx"
              },
              {
                operation = "INCLUDES"
                value     = "application_name"
              }
            ]
          }
        }
      }
    }
  }
}
`, name)
}

func testAccCoralogixResourceAlertLogsLessThanWithRouter(name string) string {
	return fmt.Sprintf(`
  resource "coralogix_connector" "slack_example" {
    id               = "%[1]v"
    name             = "%[1]v"
    type             = "slack"
    description      = "slack connector example"
    connector_config = {
      fields = [
        {
          field_name = "integrationId"
          value      = "luigis-testing-grounds"
        },
        {
          field_name = "fallbackChannel"
          value      = "luigis-testing-grounds"
        },
        {
          field_name = "channel"
          value      = "luigis-testing-grounds"
        }
      ]
    }
  }
  
  resource "coralogix_preset" "slack_example" {
    id               = "%[1]v"
    name             = "%[1]v"
    description      = "slack preset example"
    entity_type      = "alerts"
    connector_type   = "slack"
    parent_id        = "preset_system_slack_alerts_basic"
    config_overrides = [
      {
        condition_type = {
          match_entity_type_and_sub_type = {
            entity_sub_type    = "logsImmediateResolved"
          }
        }
        message_config =    {
          fields = [
            {
              field_name = "title"
              template   = "{{alert.status}} {{alertDef.priority}} - {{alertDef.name}}"
            },
            {
              field_name = "description"
              template   = "{{alertDef.description}}"
            }
          ]
        }
      }
    ]
  }

  resource "coralogix_global_router" "example" {
    id          = "%[1]v"
    name        = "%[1]v"
    description = "global router example"
    routing_labels = {
      environment = "%[1]v"
    }

    rules       = [{
        entity_type = "alerts"
        name = "rule-name"
        condition = "alertDef.priority == \"P1\""
        targets = [{
            connector_id   = coralogix_connector.slack_example.id
            preset_id      = coralogix_preset.slack_example.id
        }]
    }]
  }

  resource "coralogix_alert" "test" {
  depends_on = [coralogix_global_router.example]
  name        = "less-than alert example"
  description = "Example of logs-threshold less-than alert example from terraform"
  priority    = "P2"

  labels = {
    alert_type        = "security"
    security_severity = "high"
  }

  notification_group = {
    webhooks_settings = [
      {
        notify_on = "Triggered Only"
        recipients = ["example@coralogix.com"]
      }
    ]
    router = {}
  }

  incidents_settings = {
    notify_on = "Triggered and Resolved"
    retriggering_period = {
      minutes = 1
    }
  }

  schedule = {
    active_on = {
      days_of_week = ["Wednesday", "Thursday"]
      start_time = "08:30"
      end_time = "20:30"
    }
  }

  type_definition = {
    logs_threshold = {
      logs_filter = {
        simple_filter = {
          lucene_query = "message:\"error\""
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
      rules = [
        {
         condition = {
              threshold   = 2
              time_window = "10_MINUTES"
              condition_type   = "LESS_THAN"
            }
          override = {
            priority = "P2"
          }
        }
      ]
    }
  }
}`, name)
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

  incidents_settings = {
    notify_on = "Triggered Only"
    retriggering_period = {
      minutes = 10
    }
  }

  schedule = {
    active_on = {
      days_of_week = ["Monday", "Thursday"]
      start_time = "08:30"
      end_time = "20:30"
    }
  }

  type_definition = {
    logs_threshold = {
      rules = [
        {
        condition = {
            threshold   = 20
            time_window = "2_HOURS"
            condition_type   = "LESS_THAN"
            }
        override = {
            priority = "P2"
        }
        }
      ]
      logs_filter = {
        simple_filter = {
          lucene_query = "message:\"error\""
          label_filters = {
            application_name = [
              {
                operation = "IS"
                value     = "nginx"
              },
              {
                operation = "INCLUDES"
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
    webhooks_settings = [
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
      start_time = "08:30"
      end_time = "20:30"
    }
  }

  type_definition = {
    logs_anomaly = {
        percentage_of_deviation = 15.5
        rules = [{
            condition = {
            minimum_threshold   = 2
            time_window = "10_MINUTES"
            }
            override = {
                priority = "P2"
            }
        }]
      logs_filter = {
        simple_filter = {
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
    webhooks_settings = [
    {
        retriggering_period = {
          minutes = 1
        }
        notify_on  = "Triggered and Resolved"
        recipients = ["example@coralogix.com"]
      }
    ]
  }

  type_definition = {
    logs_anomaly = {
      custom_evaluation_delay = 100
      percentage_of_deviation = 25.5
      logs_filter = {
        simple_filter = {
          lucene_query  = "message:\"updated_error\""
          label_filters = {
            application_name = [{
                operation = "IS"
                value     = "nginx"
            }]
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
      rules = [{
        condition = {
            time_window = "1_HOUR"
            minimum_threshold = 20
        }
        override = {
            priority = "P2"
        }
      }]
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
        webhooks_settings = [
        {
            notify_on = "Triggered Only"
            recipients = ["example@coralogix.com", "example2@coralogix.com"]
        },
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
            start_time = "10:30"
            end_time = "20:30"
        }
      }

      type_definition = {
        logs_threshold = {
            rules = [{
                condition = {
                threshold   = 2
                time_window = "10_MINUTES"
                condition_type   = "LESS_THAN"
                }
                override = {
                    priority = "P2"
                }
            }]
            logs_filter       = {
                simple_filter = {
                    lucene_query  = "message:\"error\""
                    label_filters = {
                        application_name = [
                            {
                                operation = "INCLUDES"
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
        webhooks_settings = [
            { notify_on = "Triggered Only", recipients = ["example@coralogix.com"] }
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
            start_time = "08:30"
            end_time = "20:30"
        }
      }

      type_definition = {
        logs_threshold = {
            rules = [{
                condition = {
                threshold   = 20
                time_window = "2_HOURS"
                condition_type   = "LESS_THAN"
                }
                override = {
                    priority = "P2"
                }
            }]
            logs_filter       = {
                simple_filter = {
                    lucene_query  = "message:\"error\""
                    label_filters = {
                        application_name = [
                            {
                                operation = "IS"
                                value     = "nginx"
                            },
                            {
                                operation = "INCLUDES"
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
    webhooks_settings = [
      {
        notify_on = "Triggered Only"
        recipients = ["example@coralogix.com"]
      }
    ]
  }

  type_definition = {
    logs_ratio_threshold = {
      denominator_alias = "denominator"
      denominator = {
        simple_filter = {
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
      numerator = {
            simple_filter = {
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
      rules = [{
            condition = {
                threshold         = 2
                time_window = "10_MINUTES"
                condition_type		 = "MORE_THAN"
            }
            override = { }
        }]
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
    webhooks_settings = [
      {
        notify_on = "Triggered Only"
        recipients = ["example@coralogix.com"]
      }
    ]
  }

  type_definition = {
    logs_ratio_threshold = {
      denominator_alias       = "updated-denominator"
      denominator = {
        simple_filter = {
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
      numerator = {
        simple_filter = {
          lucene_query  = "mod_date:[20040101 TO 20050101]"
          label_filters = {
            subsystem_name = [
              {
                operation = "ENDS_WITH"
                value     = "updated-subsystem-name"
              },
              {
                operation = "INCLUDES"
                value     = "subsystem-name"
              }
            ]
          }
        }
      }
      rules = [ {
        condition = {
            time_window = "1_HOUR"
            threshold = 120
            condition_type = "MORE_THAN"
        }
        override = {}
      }]
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
        logs_ratio_threshold = {
              numerator_alias   = "numerator"
              denominator_alias = "denominator"
            rules = [{
                condition = {
                    threshold         = 2
                    time_window       = "10_MINUTES"
                    condition_type		 = "LESS_THAN"
                }
                override = {
                    priority = "P2"
                }
            }]
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
        logs_ratio_threshold = {
              numerator_alias   = "updated-numerator"
              denominator_alias = "updated-denominator"
            rules = [{
                condition = {
                    threshold         = 20
                    time_window       = "2_HOURS"
                    condition_type		 = "LESS_THAN"
                }
                override = {
                    priority = "P2"
                  }
            }]
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
      rules = [{
        condition = {
            time_window = "24_HOURS"
            keypath_to_track = "remote_addr_geoip.country_name"
        }
        override = {
            priority = "P2"
        }
      }]
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
        rules = [{
            condition = {
                time_window  = "12_HOURS"
                keypath_to_track = "remote_addr_geoip.city_name"
            }
            override = {
                priority = "P2"
            }
        }]
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
        max_unique_count_per_group_by_key = 500
          rules = [ {
            condition = {
                max_unique_count     = 2
                time_window          = "5_MINUTES"
            }
        }]
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
  group_by        = ["remote_addr_geoip.city_name"]

  type_definition = {
    logs_unique_count = {
        unique_count_keypath = "remote_addr_geoip.city_name"
        max_unique_count_per_group_by_key = 500
        rules = [{
            condition ={
                max_unique_count     = 5
                time_window          = "20_MINUTES"
            }
        }]
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
    logs_time_relative_threshold = {
    rules = [ {
        condition = {
            threshold        = 10
            compared_to      = "Same Hour Yesterday"
            condition_type 	 = "MORE_THAN"
        }
        override = {
            priority = "P2"
        }
    }]
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
    logs_time_relative_threshold = {
    rules = [{	
        condition = {
            threshold   = 50
            compared_to = "Same Day Last Week"
            condition_type   = "MORE_THAN"
        }
        override = {
            priority = "P2"
        }
    }]
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
  logs_time_relative_threshold = {
    custom_evaluation_delay = 100
    rules = [{
        condition = {
            threshold        = 10
            compared_to      = "Same Hour Yesterday"
            condition_type   = "LESS_THAN"
        }
        override = {
            priority = "P2"
        }
    }]
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
    logs_time_relative_threshold = {
        rules = [{
            condition = {
                threshold                   = 50
                compared_to                 = "Same Day Last Week"
                ignore_infinity             = false
                condition_type                   = "LESS_THAN"
            }
            override = {
                priority = "P2"
            }
        }]
        undetected_values_management = {
            trigger_undetected_values = true
            auto_retire_timeframe     = "6_HOURS"
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
    metric_threshold = {
        custom_evaluation_delay = 100
        metric_filter = {
            promql = "sum(rate(http_requests_total{job=\"api-server\"}[5m])) by (status)"
        }
        missing_values = {
            min_non_null_values_pct = 50
        }
        rules = [{
            override = {
                priority = "P2"
            }
            condition =	{
                threshold    = 2
                for_over_pct = 10
                of_the_last  = "10_MINUTES"
                condition_type = "MORE_THAN"
            }
        }]
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
    metric_threshold = {
        metric_filter = {
            promql = "sum(rate(http_requests_total{job=\"api-server\"}[5m])) by (status)"
        }
        missing_values = {
            replace_with_zero = true
        }
        rules = [{
            override = {
                priority = "P2"
            }
            condition = {
                threshold    = 10
                for_over_pct = 15
                of_the_last  = "1h15m"
                condition_type = "MORE_THAN"
            }
        }]
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
    metric_threshold = {
        metric_filter = {
            promql = "sum(rate(http_requests_total{job=\"api-server\"}[5m])) by (status)"
        }
        missing_values = {
            replace_with_zero = true
        }
        rules = [{
            override = {
                priority = "P2"
            }
            condition = {
                threshold    = 2
                for_over_pct = 10
                of_the_last  = "10_MINUTES"
                condition_type = "LESS_THAN"
            }
        }]
        undetected_values_management = {
            trigger_undetected_values = true
            auto_retire_timeframe     = "5_MINUTES"
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
    metric_threshold = {
        metric_filter = {
            promql = "sum(rate(http_requests_total{job=\"api-server\"}[5m])) by (status)"
        }
        missing_values = {
            min_non_null_values_pct = 50
        }
        rules = [{
            override = {
                priority = "P2"
            }
            condition = {
                threshold    = 5
                for_over_pct = 15
                of_the_last  = "10m"
                condition_type = "LESS_THAN"
            }
        }]
      undetected_values_management = {
        trigger_undetected_values = true
        auto_retire_timeframe     = "5_MINUTES"
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
    metric_anomaly = { 
        percentage_of_deviation = 20
        metric_filter = { 
            promql = "sum(rate(http_requests_total{job=\"api-server\"}[5m])) by (status)" 
        } 
        rules = [{ 
            condition = { 
                threshold    = 20 
                for_over_pct = 10 
                of_the_last = "12_HOURS" 
                condition_type = "LESS_THAN" 
                min_non_null_values_pct = 50 
            } 
        }] 
    } 
}
}`
}

func testAccCoralogixResourceAlertMetricsLessThanUsualUpdated() string {
	return `resource "coralogix_alert" "test" { 
name        = "metric-less-than-usual alert example updated" 
description = "Example of metric-less-than-usual alert from terraform updated" 
priority    = "P1" 
type_definition = { 
    metric_anomaly = { 
        percentage_of_deviation = 30
        metric_filter = { 
            promql = "sum(rate(http_requests_total{job=\"api-server\"}[5m])) by (status)" 
        } 
        rules = [{ 
            condition = { 
                threshold = 2 
                for_over_pct = 10 
                of_the_last = "10_MINUTES" 
                condition_type = "LESS_THAN" 
                min_non_null_values_pct = 50 
            } 
        }] 
    } 
}
}`
}

func testAccCoralogixResourceAlertMetricLessThanOrEquals() string {
	return `resource "coralogix_alert" "test" {
name        = "metric-less-than-or-equals alert example"
description = "Example of metric-less-than-or-equals alert from terraform"
priority    = "P1"

type_definition = {
    metric_threshold = {
        metric_filter = {
            promql = "sum(rate(http_requests_total{job=\"api-server\"}[5m])) by (status)"
        }
        missing_values = {
            replace_with_zero = true
        }
        rules = [{
            condition = {
                threshold    = 2
                for_over_pct = 10
                of_the_last = "10_MINUTES"
                condition_type = "LESS_THAN_OR_EQUALS"
            }
            override = {
                priority = "P2"
            }
        }]
        undetected_values_management = {
            trigger_undetected_values = true
            auto_retire_timeframe     = "5_MINUTES"
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
    metric_threshold = {
        metric_filter = {
            promql = "sum(rate(http_requests_total{job=\"api-server\"}[5m])) by (status)"
        }  
        missing_values = {
            min_non_null_values_pct = 50
        }
        rules = [{
            condition = {
                threshold    = 5
                for_over_pct = 15
                of_the_last = "10_MINUTES"
                condition_type = "LESS_THAN_OR_EQUALS"
            }
            override = {
                priority = "P2"
            }
        }]
        undetected_values_management = {
            trigger_undetected_values = true
            auto_retire_timeframe     = "5_MINUTES"
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
    metric_threshold = {
        metric_filter = {
            promql = "sum(rate(http_requests_total{job=\"api-server\"}[5m])) by (status)"
        }
        rules = [{
            condition = {
                threshold    = 2
                for_over_pct = 10
                of_the_last = "10_MINUTES"
                condition_type = "MORE_THAN_OR_EQUALS"
            }
            override = {
                priority = "P2"
            }
        }]
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
    metric_threshold = {
        metric_filter = {
            promql = "sum(rate(http_requests_total{job=\"api-server\"}[5m])) by (status)"
        }
        rules = [{
            condition = {
                threshold    = 10
                for_over_pct = 15
                of_the_last = "1_HOUR"
                condition_type = "MORE_THAN_OR_EQUALS"
            }
            override = {
                priority = "P2"
            }
        }]
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
      tracing_filter = {
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
      tracing_filter = {
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
    tracing_threshold = {
        tracing_filter = {
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
        rules = [{
            condition = {
                time_window = "10_MINUTES"
                span_amount = 5
            }
        }]
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
    tracing_threshold = {
      tracing_filter = {
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
        rules = [{
            condition = {
                span_amount = 5
                time_window = "1_HOUR"
            }
        }]
    }
  }
}
`
}

func testAccCoralogixResourceAlertFlow() string {
	return `
resource "coralogix_alert" "test_1"{
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

resource "coralogix_alert" "test_4"{
    name        = "logs immediate alert 4"
    priority    = "P4"
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
            enforce_suppression = false
            stages = [{
                flow_stages_groups = [{
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
                            id = coralogix_alert.test_4.id
                        },
                    ]
                    next_op   = "OR"
                    alerts_op = "AND"
                },]
                timeframe_ms   = 10
                timeframe_type = "Up To"
            }]
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

resource "coralogix_alert" "test_4"{
    name        = "logs immediate alert 4"
    priority    = "P4"
    type_definition = {
        logs_immediate = {
        }
    }
}

resource "coralogix_alert" "test_5"{
    name        = "logs immediate alert 5"
    priority    = "P5"
    type_definition = {
        logs_immediate = {
        }
    }
}

resource "coralogix_alert" "test" {
    name        = "flow alert example updated"
    description = "Example of flow alert from terraform updated"
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
                        id = coralogix_alert.test_4.id
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
                        id = coralogix_alert.test_5.id
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

func testAccCoralogixResourceAlertSloBurnRate(sloName, alertName string) string {
	return fmt.Sprintf(`
resource "coralogix_slo_v2" "example" {
  name        = "%[1]s"
  description = "My SLO for CPU usage"
  target_threshold_percentage = 30

  sli = {
    request_based_metric_sli = {
      good_events = {
        query = "avg(rate(cpu_usage_seconds_total[5m])) by (instance)"
      }
      total_events = {
        query = "avg(rate(cpu_usage_seconds_total[5m])) by (instance)"
      }
    }
  }

  window = {
    slo_time_frame = "7_days"
  }

  labels = {
    label1 = "value1"
  }
}

resource "coralogix_alert" "slo_alert_burn_rate" {
  name         = "%[2]s"
  description  = "Alert based on SLO burn rate threshold"
  priority     = "P1"
  phantom_mode = false

  labels = {
    alert_type        = "security"
    security_severity = "high"
  }

  notification_group = {
    webhooks_settings = [{
      retriggering_period = {
        minutes = 5
      }
      notify_on  = "Triggered and Resolved"
      recipients = ["example@coralogix.com"]
    }]
  }

  schedule = {
    active_on = {
      days_of_week = ["Wednesday", "Thursday"]
      start_time   = "08:30"
      end_time     = "20:30"
    }
  }

  type_definition = {
    slo_threshold = {
      slo_definition = {
        slo_id = coralogix_slo_v2.example.id
      }
      burn_rate = {
        rules = [
          {
            condition = {
              threshold = 1.0
            }
            override = {
              priority = "P1"
            }
          },
          {
            condition = {
              threshold = 1.3
            }
            override = {
              priority = "P2"
            }
          }
        ]
        single = {
          time_duration = {
            duration = 1
            unit     = "HOURS"
          }
        }
      }
    }
  }
}
`, sloName, alertName)
}
