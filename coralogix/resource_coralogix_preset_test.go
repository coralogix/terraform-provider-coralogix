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

const genericHttpsPresetResourceName = "coralogix_preset.generic_https_example"
const slackPresetResourceName = "coralogix_preset.slack_example"
const pagerdutyPresetResourceName = "coralogix_preset.pagerduty_example"

func TestAccCoralogixResourceGenericHttpsPreset(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { TestAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceCoralogixGenericHttpsPreset(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(genericHttpsPresetResourceName, "id", "terraform_generic_https_preset_example"),
					resource.TestCheckResourceAttr(genericHttpsPresetResourceName, "name", "generic_https example"),
					resource.TestCheckResourceAttr(genericHttpsPresetResourceName, "description", "generic_https preset example"),
					resource.TestCheckResourceAttr(genericHttpsPresetResourceName, "entity_type", "alerts"),
					resource.TestCheckResourceAttr(genericHttpsPresetResourceName, "connector_type", "generic_https"),
					resource.TestCheckResourceAttr(genericHttpsPresetResourceName, "parent_id", "preset_system_generic_https_alerts_empty"),
					resource.TestCheckResourceAttr(genericHttpsPresetResourceName, "config_overrides.#", "1"),
					resource.TestCheckTypeSetElemNestedAttrs(genericHttpsPresetResourceName, "config_overrides.*", map[string]string{
						"condition_type.match_entity_type_and_sub_type.entity_type":     "alerts",
						"condition_type.match_entity_type_and_sub_type.entity_sub_type": "logsImmediateResolved",
						"message_config.fields.#":                                       "2",
					}),
					resource.TestCheckTypeSetElemNestedAttrs(genericHttpsPresetResourceName, "config_overrides.*.message_config.fields.*", map[string]string{
						"field_name": "headers",
						"template":   "{}",
					}),
					resource.TestCheckTypeSetElemNestedAttrs(genericHttpsPresetResourceName, "config_overrides.*.message_config.fields.*", map[string]string{
						"field_name": "body",
						"template":   `{ "groupingKey": "{{alert.groupingKey}}", "status": "{{alert.status}}", "groups": "{{alert.groups}}" }`,
					}),
				),
			},
			{
				ResourceName:      genericHttpsPresetResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccResourceCoralogixGenericHttpsPresetUpdate(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(genericHttpsPresetResourceName, "id", "terraform_generic_https_preset_example"),
					resource.TestCheckResourceAttr(genericHttpsPresetResourceName, "name", "generic_https example updated"),
					resource.TestCheckResourceAttr(genericHttpsPresetResourceName, "description", "generic_https preset example"),
					resource.TestCheckResourceAttr(genericHttpsPresetResourceName, "entity_type", "alerts"),
					resource.TestCheckResourceAttr(genericHttpsPresetResourceName, "connector_type", "generic_https"),
					resource.TestCheckResourceAttr(genericHttpsPresetResourceName, "parent_id", "preset_system_generic_https_alerts_empty"),
					resource.TestCheckResourceAttr(genericHttpsPresetResourceName, "config_overrides.#", "1"),
					resource.TestCheckTypeSetElemNestedAttrs(genericHttpsPresetResourceName, "config_overrides.*", map[string]string{
						"condition_type.match_entity_type_and_sub_type.entity_type":     "alerts",
						"condition_type.match_entity_type_and_sub_type.entity_sub_type": "logsImmediateResolved",
						"message_config.fields.#":                                       "2",
					}),
					resource.TestCheckTypeSetElemNestedAttrs(genericHttpsPresetResourceName, "config_overrides.*.message_config.fields.*", map[string]string{
						"field_name": "headers",
						"template":   "{}",
					}),
					resource.TestCheckTypeSetElemNestedAttrs(genericHttpsPresetResourceName, "config_overrides.*.message_config.fields.*", map[string]string{
						"field_name": "body",
						"template":   `{ "groupingKey": "{{alert.groupingKey}}", "status": "{{alert.status}}", "groups": "{{alert.groups}}" }`,
					}),
				),
			},
		},
	},
	)
}

func TestAccCoralogixResourceSlackPreset(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { TestAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceCoralogixSlackPreset(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(slackPresetResourceName, "id", "terraform_slack_preset_example"),
					resource.TestCheckResourceAttr(slackPresetResourceName, "name", "slack example"),
					resource.TestCheckResourceAttr(slackPresetResourceName, "description", "slack preset example"),
					resource.TestCheckResourceAttr(slackPresetResourceName, "entity_type", "alerts"),
					resource.TestCheckResourceAttr(slackPresetResourceName, "connector_type", "slack"),
					resource.TestCheckResourceAttr(slackPresetResourceName, "parent_id", "preset_system_slack_alerts_basic"),
					resource.TestCheckResourceAttr(slackPresetResourceName, "config_overrides.#", "1"),
					resource.TestCheckTypeSetElemNestedAttrs(slackPresetResourceName, "config_overrides.*", map[string]string{
						"condition_type.match_entity_type_and_sub_type.entity_type":     "alerts",
						"condition_type.match_entity_type_and_sub_type.entity_sub_type": "logsImmediateResolved",
						"message_config.fields.#":                                       "2",
					}),
					resource.TestCheckTypeSetElemNestedAttrs(slackPresetResourceName, "config_overrides.*.message_config.fields.*", map[string]string{
						"field_name": "title",
						"template":   "{{alert.status}} {{alertDef.priority}} - {{alertDef.name}}",
					}),
					resource.TestCheckTypeSetElemNestedAttrs(slackPresetResourceName, "config_overrides.*.message_config.fields.*", map[string]string{
						"field_name": "description",
						"template":   "{{alertDef.description}}",
					}),
				),
			},
			{
				ResourceName:      slackPresetResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccResourceCoralogixSlackPresetUpdate(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(slackPresetResourceName, "id", "terraform_slack_preset_example"),
					resource.TestCheckResourceAttr(slackPresetResourceName, "name", "slack example updated"),
					resource.TestCheckResourceAttr(slackPresetResourceName, "description", "slack preset example"),
					resource.TestCheckResourceAttr(slackPresetResourceName, "entity_type", "alerts"),
					resource.TestCheckResourceAttr(slackPresetResourceName, "connector_type", "slack"),
					resource.TestCheckResourceAttr(slackPresetResourceName, "parent_id", "preset_system_slack_alerts_basic"),
					resource.TestCheckResourceAttr(slackPresetResourceName, "config_overrides.#", "1"),
					resource.TestCheckTypeSetElemNestedAttrs(slackPresetResourceName, "config_overrides.*", map[string]string{
						"condition_type.match_entity_type_and_sub_type.entity_type":     "alerts",
						"condition_type.match_entity_type_and_sub_type.entity_sub_type": "logsImmediateResolved",
						"message_config.fields.#":                                       "2",
					}),
					resource.TestCheckTypeSetElemNestedAttrs(slackPresetResourceName, "config_overrides.*.message_config.fields.*", map[string]string{
						"field_name": "title",
						"template":   "{{alert.status}} {{alertDef.priority}} - {{alertDef.name}}",
					}),
					resource.TestCheckTypeSetElemNestedAttrs(slackPresetResourceName, "config_overrides.*.message_config.fields.*", map[string]string{
						"field_name": "description",
						"template":   "{{alertDef.description}}",
					}),
				),
			},
		},
	})
}

func TestAccCoralogixResourcePagerdutyPreset(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { TestAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceCoralogixPagerdutyPreset(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(pagerdutyPresetResourceName, "id", "terraform_pagerduty_preset_example"),
					resource.TestCheckResourceAttr(pagerdutyPresetResourceName, "name", "pagerduty example"),
					resource.TestCheckResourceAttr(pagerdutyPresetResourceName, "description", "pagerduty preset example"),
					resource.TestCheckResourceAttr(pagerdutyPresetResourceName, "entity_type", "alerts"),
					resource.TestCheckResourceAttr(pagerdutyPresetResourceName, "connector_type", "pagerduty"),
					resource.TestCheckResourceAttr(pagerdutyPresetResourceName, "parent_id", "preset_system_pagerduty_alerts_basic"),
					resource.TestCheckResourceAttr(pagerdutyPresetResourceName, "config_overrides.#", "1"),
					resource.TestCheckTypeSetElemNestedAttrs(pagerdutyPresetResourceName, "config_overrides.*", map[string]string{
						"condition_type.match_entity_type.entity_type": "alerts",
						"message_config.fields.#":                      "3",
					}),
					resource.TestCheckTypeSetElemNestedAttrs(pagerdutyPresetResourceName, "config_overrides.*.message_config.fields.*", map[string]string{
						"field_name": "summary",
						"template":   "{{ alertDef.description }}",
					}),
					resource.TestCheckTypeSetElemNestedAttrs(pagerdutyPresetResourceName, "config_overrides.*.message_config.fields.*", map[string]string{
						"field_name": "severity",
						"template":   `{{% if alert.highestPriority | default(value = alertDef.priority) == 'P1' %}}critical{{% elif alert.highestPriority | default(value = alertDef.priority) == 'P2' %}}error{{% elif alert.highestPriority | default(value = alertDef.priority) == 'P3' %}}warning{{% elif alert.highestPriority | default(value = alertDef.priority) == 'P4' or alert.highestPriority | default(value = alertDef.priority) == 'P5' %}}info{{% else %}}info{{% endif %}}`,
					}),
					resource.TestCheckTypeSetElemNestedAttrs(pagerdutyPresetResourceName, "config_overrides.*.message_config.fields.*", map[string]string{
						"field_name": "timestamp",
						"template":   "{{ alertDef.timestamp }}",
					}),
				),
			},
			{
				ResourceName:      pagerdutyPresetResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccResourceCoralogixPagerdutyPresetUpdate(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(pagerdutyPresetResourceName, "id", "terraform_pagerduty_preset_example"),
					resource.TestCheckResourceAttr(pagerdutyPresetResourceName, "name", "pagerduty example updated"),
					resource.TestCheckResourceAttr(pagerdutyPresetResourceName, "description", "pagerduty preset example"),
					resource.TestCheckResourceAttr(pagerdutyPresetResourceName, "entity_type", "alerts"),
					resource.TestCheckResourceAttr(pagerdutyPresetResourceName, "connector_type", "pagerduty"),
					resource.TestCheckResourceAttr(pagerdutyPresetResourceName, "parent_id", "preset_system_pagerduty_alerts_basic"),
					resource.TestCheckResourceAttr(pagerdutyPresetResourceName, "config_overrides.#", "1"),
					resource.TestCheckTypeSetElemNestedAttrs(pagerdutyPresetResourceName, "config_overrides.*", map[string]string{
						"condition_type.match_entity_type.entity_type": "alerts",
						"message_config.fields.#":                      "3",
					}),
					resource.TestCheckTypeSetElemNestedAttrs(pagerdutyPresetResourceName, "config_overrides.*.message_config.fields.*", map[string]string{
						"field_name": "summary",
						"template":   "{{ alertDef.description }}",
					}),
					resource.TestCheckTypeSetElemNestedAttrs(pagerdutyPresetResourceName, "config_overrides.*.message_config.fields.*", map[string]string{
						"field_name": "severity",
						"template":   `{{% if alert.highestPriority | default(value = alertDef.priority) == 'P1' %}}critical{{% elif alert.highestPriority | default(value = alertDef.priority) == 'P2' %}}error{{% elif alert.highestPriority | default(value = alertDef.priority) == 'P3' %}}warning{{% elif alert.highestPriority | default(value = alertDef.priority) == 'P4' or alert.highestPriority | default(value = alertDef.priority) == 'P5' %}}info{{% else %}}info{{% endif %}}`,
					}),
					resource.TestCheckTypeSetElemNestedAttrs(pagerdutyPresetResourceName, "config_overrides.*.message_config.fields.*", map[string]string{
						"field_name": "timestamp",
						"template":   "{{ alertDef.timestamp }}",
					}),
				),
			},
		},
	})
}

func testAccResourceCoralogixGenericHttpsPreset() string {
	return `
	resource "coralogix_preset" "generic_https_example" {
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
	return `resource "coralogix_preset" "generic_https_example" {
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
	return `resource "coralogix_preset" "slack_example" {
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
	return `resource "coralogix_preset" "slack_example" {
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
	return `resource "coralogix_preset" "pagerduty_example" {
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
	return `resource "coralogix_preset" "pagerduty_example" {
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
