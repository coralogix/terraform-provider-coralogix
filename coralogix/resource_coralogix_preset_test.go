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
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

const presetResourceName = "coralogix_preset.example"

func TestGenericHttpsPreset(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { TestAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceCoralogixGenericHttpsPreset(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(presetResourceName, "id", "terraform_generic_https_preset_example"),
					resource.TestCheckResourceAttr(presetResourceName, "name", "generic_https example"),
					resource.TestCheckResourceAttr(presetResourceName, "description", "generic_https preset example"),
					resource.TestCheckResourceAttr(presetResourceName, "entity_type", "alerts"),
					resource.TestCheckResourceAttr(presetResourceName, "connector_type", "generic_https"),
					resource.TestCheckResourceAttr(presetResourceName, "parent_id", "preset_system_generic_https_alerts_empty"),
					resource.TestCheckTypeSetElemNestedAttrs(presetResourceName, "config_overrides.*", map[string]string{
						"condition_type.0.match_entity_type_and_sub_type.0.entity_type":     "alerts",
						"condition_type.0.match_entity_type_and_sub_type.0.entity_sub_type": "logsImmediateResolved",
						"message_config.0.fields.0.field_name":                              "headers",
						"message_config.0.fields.0.template":                                "{}",
						"message_config.0.fields.1.field_name":                              "body",
						"message_config.0.fields.1.template":                                "{ \"groupingKey\": \"{{alert.groupingKey}}\", \"status\": \"{{alert.status}}\", \"groups\": \"{{alert.groups}}\" }",
					}),
				),
			},
			{
				ResourceName:      presetResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccResourceCoralogixGenericHttpsPresetUpdate(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(presetResourceName, "id", "terraform_generic_https_preset_example"),
					resource.TestCheckResourceAttr(presetResourceName, "name", "generic_https example updated"),
					resource.TestCheckResourceAttr(presetResourceName, "description", "generic_https preset example"),
					resource.TestCheckResourceAttr(presetResourceName, "entity_type", "alerts"),
					resource.TestCheckResourceAttr(presetResourceName, "connector_type", "generic_https"),
					resource.TestCheckResourceAttr(presetResourceName, "parent_id", "preset_system_generic_https_alerts_empty"),
					resource.TestCheckTypeSetElemNestedAttrs(presetResourceName, "config_overrides.*", map[string]string{
						"condition_type.0.match_entity_type_and_sub_type.0.entity_type":     "alerts",
						"condition_type.0.match_entity_type_and_sub_type.0.entity_sub_type": "logsImmediateResolved",
						"message_config.0.fields.0.field_name":                              "headers",
						"message_config.0.fields.0.template":                                "{}",
						"message_config.0.fields.1.field_name":                              "body",
						"message_config.0.fields.1.template":                                "{ \"groupingKey\": \"{{alert.groupingKey}}\", \"status\": \"{{alert.status}}\", \"groups\": \"{{alert.groups}}\" }",
					}),
				),
			},
		},
	})
}

func TestSlackPreset(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { TestAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceCoralogixSlackPreset(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(presetResourceName, "id", "terraform_slack_preset_example"),
					resource.TestCheckResourceAttr(presetResourceName, "name", "slack example"),
					resource.TestCheckResourceAttr(presetResourceName, "description", "slack preset example"),
					resource.TestCheckResourceAttr(presetResourceName, "entity_type", "alerts"),
					resource.TestCheckResourceAttr(presetResourceName, "connector_type", "slack"),
					resource.TestCheckResourceAttr(presetResourceName, "parent_id", "preset_system_slack_alerts_basic"),
					resource.TestCheckTypeSetElemNestedAttrs(presetResourceName, "config_overrides.*", map[string]string{
						"condition_type.0.match_entity_type_and_sub_type.0.entity_type":     "alerts",
						"condition_type.0.match_entity_type_and_sub_type.0.entity_sub_type": "logsImmediateResolved",
						"message_config.0.fields.0.field_name":                              "title",
						"message_config.0.fields.0.template":                                "{{alert.status}} {{alertDef.priority}} - {{alertDef.name}}",
						"message_config.0.fields.1.field_name":                              "description",
						"message_config.0.fields.1.template":                                "{{alertDef.description}}",
					}),
				),
			},
			{
				ResourceName:      presetResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccResourceCoralogixSlackPresetUpdate(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(presetResourceName, "id", "terraform_slack_preset_example"),
					resource.TestCheckResourceAttr(presetResourceName, "name", "slack example updated"),
					resource.TestCheckResourceAttr(presetResourceName, "description", "slack preset example"),
					resource.TestCheckResourceAttr(presetResourceName, "entity_type", "alerts"),
					resource.TestCheckResourceAttr(presetResourceName, "connector_type", "slack"),
					resource.TestCheckResourceAttr(presetResourceName, "parent_id", "preset_system_slack_alerts_basic"),
					resource.TestCheckTypeSetElemNestedAttrs(presetResourceName, "config_overrides.*", map[string]string{
						"condition_type.0.match_entity_type_and_sub_type.0.entity_type":     "alerts",
						"condition_type.0.match_entity_type_and_sub_type.0.entity_sub_type": "logsImmediateResolved",
						"message_config.0.fields.0.field_name":                              "title",
						"message_config.0.fields.0.template":                                "{{alert.status}} {{alertDef.priority}} - {{alertDef.name}}",
						"message_config.0.fields.1.field_name":                              "description",
						"message_config.0.fields.1.template":                                "{{alertDef.description}}",
					}),
				),
			},
		},
	})
}

func TestPagerdutyPreset(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { TestAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceCoralogixPagerdutyPreset(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(presetResourceName, "id", "terraform_pagerduty_preset_example"),
					resource.TestCheckResourceAttr(presetResourceName, "name", "pagerduty example"),
					resource.TestCheckResourceAttr(presetResourceName, "description", "pagerduty preset example"),
					resource.TestCheckResourceAttr(presetResourceName, "entity_type", "alerts"),
					resource.TestCheckResourceAttr(presetResourceName, "connector_type", "pagerduty"),
					resource.TestCheckResourceAttr(presetResourceName, "parent_id", "preset_system_pagerduty_alerts_basic"),
					resource.TestCheckTypeSetElemNestedAttrs(presetResourceName, "config_overrides.*", map[string]string{
						"condition_type.0.match_entity_type.0.entity_type": "alerts",
						"message_config.0.fields.0.field_name":             "summary",
						"message_config.0.fields.0.template":               "{{ alertDef.description }}",
						"message_config.0.fields.1.field_name":             "severity",
						"message_config.0.fields.1.template": `
						{% if alert.highestPriority | default(value = alertDef.priority) == 'P1' %}
						critical
						{% elif alert.highestPriority | default(value = alertDef.priority) == 'P2' %}
						error
						{% elif alert.highestPriority | default(value = alertDef.priority) == 'P3' %}
						warning
						{% elif alert.highestPriority | default(value = alertDef.priority) == 'P4' or alert.highestPriority | default(value = alertDef.priority)  == 'P5' %}
						info
						{% else %}
						info
						{% endif %}
						`,
						"message_config.0.fields.2.field_name": "timestamp",
						"message_config.0.fields.2.template":   "{{ alertDef.timestamp }}",
					}),
				),
			},
			{
				ResourceName:      presetResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccResourceCoralogixPagerdutyPresetUpdate(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(presetResourceName, "id", "terraform_pagerduty_preset_example"),
					resource.TestCheckResourceAttr(presetResourceName, "name", "pagerduty example updated"),
					resource.TestCheckResourceAttr(presetResourceName, "description", "pagerduty preset example"),
					resource.TestCheckResourceAttr(presetResourceName, "entity_type", "alerts"),
					resource.TestCheckResourceAttr(presetResourceName, "connector_type", "pagerduty"),
					resource.TestCheckResourceAttr(presetResourceName, "parent_id", "preset_system_pagerduty_alerts_basic"),
					resource.TestCheckTypeSetElemNestedAttrs(presetResourceName, "config_overrides.*", map[string]string{
						"condition_type.0.match_entity_type.0.entity_type": "alerts",
						"message_config.0.fields.0.field_name":             "summary",
						"message_config.0.fields.0.template":               "{{ alertDef.description }}",
						"message_config.0.fields.1.field_name":             "severity",
						"message_config.0.fields.1.template": `
						{% if alert.highestPriority | default(value = alertDef.priority) == 'P1' %}
						critical
						{% elif alert.highestPriority | default(value = alertDef.priority) == 'P2' %}
						error
						{% elif alert.highestPriority | default(value = alertDef.priority) == 'P3' %}
						warning
						{% elif alert.highestPriority | default(value = alertDef.priority) == 'P4' or alert.highestPriority | default(value = alertDef.priority)  == 'P5' %}
						info
						{% else %}
						info
						{% endif %}
						`,
						"message_config.0.fields.2.field_name": "timestamp",
						"message_config.0.fields.2.template":   "{{ alertDef.timestamp }}",
					}),
				),
			},
		},
	})
}

func testAccResourceCoralogixGenericHttpsPreset() string {
	return `resource "coralogix_preset" "example" {
      id               = "terraform_generic_https_preset_example"
      name             = "generic_https example"
      description      = "generic_https preset example"
      entity_type      = "alerts"
      connector_type   = "generic_https"
      parent_id        = "preset_system_generic_https_alerts_empty"
      config_overrides = [
        {
          condition_type = {
            match_entity_type_and_sub_type = {
              entity_type = "alerts"
              entity_sub_type    = "logsImmediateResolved"
            }
          }
          message_config = {
            fields = [
              {
                field_name = "headers"
                template   = "{}"
              },
              {
                field_name = "body"
                template   = "{ \"groupingKey\": \"{{alert.groupingKey}}\", \"status\": \"{{alert.status}}\", \"groups\": \"{{alert.groups}}\" }"
              }
            ]
          }
        }
      ]
 	}
  `
}

func testAccResourceCoralogixGenericHttpsPresetUpdate() string {
	return `resource "coralogix_preset" "example" {
      id               = "terraform_generic_https_preset_example"
      name             = "generic_https example updated"
      description      = "generic_https preset example"
      entity_type      = "alerts"
      connector_type   = "generic_https"
      parent_id        = "preset_system_generic_https_alerts_empty"
      config_overrides = [
        {
          condition_type = {
            match_entity_type_and_sub_type = {
              entity_type = "alerts"
              entity_sub_type    = "logsImmediateResolved"
            }
          }
          message_config = {
            fields = [
              {
                field_name = "headers"
                template   = "{}"
              },
              {
                field_name = "body"
                template   = "{ \"groupingKey\": \"{{alert.groupingKey}}\", \"status\": \"{{alert.status}}\", \"groups\": \"{{alert.groups}}\" }"
              }
            ]
          }
        }
      ]
    }
  `
}

func testAccResourceCoralogixSlackPreset() string {
	return `resource "coralogix_preset" "example" {
      id               = "terraform_slack_preset_example"
      name             = "slack example"
      description      = "slack preset example"
      entity_type      = "alerts"
      connector_type   = "slack"
      parent_id        = "preset_system_slack_alerts_basic"
      config_overrides = [
        {
          condition_type = {
            match_entity_type_and_sub_type = {
              entity_type = "alerts"
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
  `
}

func testAccResourceCoralogixSlackPresetUpdate() string {
	return `resource "coralogix_preset" "example" {
      id               = "terraform_slack_preset_example"
      name             = "slack example updated"
      description      = "slack preset example"
      entity_type      = "alerts"
      connector_type   = "slack"
      parent_id        = "preset_system_slack_alerts_basic"
      config_overrides = [
        {
          condition_type = {
            match_entity_type_and_sub_type = {
              entity_type = "alerts"
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
  `
}

func testAccResourceCoralogixPagerdutyPreset() string {
	return `resource "coralogix_preset" "example" {
      id               = "terraform_pagerduty_preset_example"
      name             = "pagerduty example"
      description      = "pagerduty preset example"
      entity_type      = "alerts"
      connector_type   = "pagerduty"
      parent_id        = "preset_system_pagerduty_alerts_basic"
      config_overrides = [
        {
          condition_type = {
            match_entity_type = {
              entity_type = "alerts"
            }
          }
          message_config = {
            fields = [
              {
                field_name = "summary"
                template   = "{{ alertDef.description }}"
              },
              {
                field_name = "severity"
                template   = <<EOF
                {% if alert.highestPriority | default(value = alertDef.priority) == 'P1' %}
                critical
                {% elif alert.highestPriority | default(value = alertDef.priority) == 'P2' %}
                error
                {% elif alert.highestPriority | default(value = alertDef.priority) == 'P3' %}
                warning
                {% elif alert.highestPriority | default(value = alertDef.priority) == 'P4' or alert.highestPriority | default(value = alertDef.priority)  == 'P5' %}
                info
                {% else %}
                info
                {% endif %}
                EOF
              },
              {
                field_name = "timestamp"
                template   = "{{ alertDef.timestamp }}"
              }
            ]
          }
        }
      ]
    }
  `
}

func testAccResourceCoralogixPagerdutyPresetUpdate() string {
	return `resource "coralogix_preset" "example" {
      id               = "terraform_pagerduty_preset_example"
      name             = "pagerduty example updated"
      description      = "pagerduty preset example"
      entity_type      = "alerts"
      connector_type   = "pagerduty"
      parent_id        = "preset_system_pagerduty_alerts_basic"
      config_overrides = [
        {
          condition_type = {
            match_entity_type = {
              entity_type = "alerts"
            }
          }
          message_config = {
            fields = [
              {
                field_name = "summary"
                template   = "{{ alertDef.description }}"
              },
              {
                field_name = "severity"
                template   = <<EOF
                {% if alert.highestPriority | default(value = alertDef.priority) == 'P1' %}
                critical
                {% elif alert.highestPriority | default(value = alertDef.priority) == 'P2' %}
                error
                {% elif alert.highestPriority | default(value = alertDef.priority) == 'P3' %}
                warning
                {% elif alert.highestPriority | default(value = alertDef.priority) == 'P4' or alert.highestPriority | default(value = alertDef.priority)  == 'P5' %}
                info
                {% else %}
                info
                {% endif %}
                EOF
              },
              {
                field_name = "timestamp"
                template   = "{{ alertDef.timestamp }}"
              }
            ]
          }
        }
      ]
    }
  `
}
