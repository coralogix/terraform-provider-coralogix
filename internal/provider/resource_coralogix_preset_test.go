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
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

const genericHttpsPresetResourceName = "coralogix_preset.generic_https_example"
const slackPresetResourceName = "coralogix_preset.slack_example"
const pagerdutyPresetResourceName = "coralogix_preset.pagerduty_example"
const emailPresetResourceName = "coralogix_preset.email_example"

func TestAccCoralogixResourceGenericHttpsPreset(t *testing.T) {
	name := uuid.NewString()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceCoralogixGenericHttpsPreset(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(genericHttpsPresetResourceName, "id", name),
					resource.TestCheckResourceAttr(genericHttpsPresetResourceName, "name", name),
					resource.TestCheckResourceAttr(genericHttpsPresetResourceName, "description", "generic_https preset example"),
					resource.TestCheckResourceAttr(genericHttpsPresetResourceName, "entity_type", "alerts"),
					resource.TestCheckResourceAttr(genericHttpsPresetResourceName, "connector_type", "generic_https"),
					resource.TestCheckResourceAttr(genericHttpsPresetResourceName, "parent_id", "preset_system_generic_https_alerts_empty"),
					resource.TestCheckResourceAttr(genericHttpsPresetResourceName, "config_overrides.#", "1"),
					resource.TestCheckTypeSetElemNestedAttrs(genericHttpsPresetResourceName, "config_overrides.*", map[string]string{
						"condition_type.match_entity_type_and_sub_type.entity_sub_type": "logsImmediateResolved",
						"message_config.fields.#": "2",
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
				Config: testAccResourceCoralogixGenericHttpsPresetUpdate(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(genericHttpsPresetResourceName, "id", name),
					resource.TestCheckResourceAttr(genericHttpsPresetResourceName, "name", "generic_https example updated"),
					resource.TestCheckResourceAttr(genericHttpsPresetResourceName, "description", "generic_https preset example"),
					resource.TestCheckResourceAttr(genericHttpsPresetResourceName, "entity_type", "alerts"),
					resource.TestCheckResourceAttr(genericHttpsPresetResourceName, "connector_type", "generic_https"),
					resource.TestCheckResourceAttr(genericHttpsPresetResourceName, "parent_id", "preset_system_generic_https_alerts_empty"),
					resource.TestCheckResourceAttr(genericHttpsPresetResourceName, "config_overrides.#", "1"),
					resource.TestCheckTypeSetElemNestedAttrs(genericHttpsPresetResourceName, "config_overrides.*", map[string]string{
						"condition_type.match_entity_type_and_sub_type.entity_sub_type": "logsImmediateResolved",
						"message_config.fields.#": "2",
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
	name := uuid.NewString()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceCoralogixSlackPreset(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(slackPresetResourceName, "id", name),
					resource.TestCheckResourceAttr(slackPresetResourceName, "name", name),
					resource.TestCheckResourceAttr(slackPresetResourceName, "description", "slack preset example"),
					resource.TestCheckResourceAttr(slackPresetResourceName, "entity_type", "alerts"),
					resource.TestCheckResourceAttr(slackPresetResourceName, "connector_type", "slack"),
					resource.TestCheckResourceAttr(slackPresetResourceName, "parent_id", "preset_system_slack_alerts_basic"),
					resource.TestCheckResourceAttr(slackPresetResourceName, "config_overrides.#", "1"),
					resource.TestCheckTypeSetElemNestedAttrs(slackPresetResourceName, "config_overrides.*", map[string]string{
						"condition_type.match_entity_type_and_sub_type.entity_sub_type": "logsImmediateResolved",
						"message_config.fields.#": "2",
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
				Config: testAccResourceCoralogixSlackPresetUpdate(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(slackPresetResourceName, "id", name),
					resource.TestCheckResourceAttr(slackPresetResourceName, "name", name),
					resource.TestCheckResourceAttr(slackPresetResourceName, "description", "slack preset example updated"),
					resource.TestCheckResourceAttr(slackPresetResourceName, "entity_type", "alerts"),
					resource.TestCheckResourceAttr(slackPresetResourceName, "connector_type", "slack"),
					resource.TestCheckResourceAttr(slackPresetResourceName, "parent_id", "preset_system_slack_alerts_basic"),
					resource.TestCheckResourceAttr(slackPresetResourceName, "config_overrides.#", "1"),
					resource.TestCheckTypeSetElemNestedAttrs(slackPresetResourceName, "config_overrides.*", map[string]string{
						"condition_type.match_entity_type_and_sub_type.entity_sub_type": "logsImmediateResolved",
						"message_config.fields.#": "2",
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
	name := uuid.NewString()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceCoralogixPagerdutyPreset(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(pagerdutyPresetResourceName, "id", name),
					resource.TestCheckResourceAttr(pagerdutyPresetResourceName, "name", name),
					resource.TestCheckResourceAttr(pagerdutyPresetResourceName, "description", "pagerduty preset example"),
					resource.TestCheckResourceAttr(pagerdutyPresetResourceName, "entity_type", "alerts"),
					resource.TestCheckResourceAttr(pagerdutyPresetResourceName, "connector_type", "pagerduty"),
					resource.TestCheckResourceAttr(pagerdutyPresetResourceName, "parent_id", "preset_system_pagerduty_alerts_basic"),
					resource.TestCheckResourceAttr(pagerdutyPresetResourceName, "config_overrides.#", "1"),
					resource.TestCheckTypeSetElemNestedAttrs("coralogix_preset.pagerduty_example", "config_overrides.*", map[string]string{
						"condition_type.match_entity_type.%": "0",
						"message_config.fields.#":            "3",
					}),
					resource.TestCheckTypeSetElemNestedAttrs(pagerdutyPresetResourceName, "config_overrides.*.message_config.fields.*", map[string]string{
						"field_name": "summary",
						"template":   "{{ alertDef.description }}",
					}),
					resource.TestCheckTypeSetElemNestedAttrs("coralogix_preset.pagerduty_example", "config_overrides.*.message_config.fields.*", map[string]string{
						"field_name": "severity",
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
				Config: testAccResourceCoralogixPagerdutyPresetUpdate(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(pagerdutyPresetResourceName, "id", name),
					resource.TestCheckResourceAttr(pagerdutyPresetResourceName, "name", "pagerduty example updated"),
					resource.TestCheckResourceAttr(pagerdutyPresetResourceName, "description", "pagerduty preset example"),
					resource.TestCheckResourceAttr(pagerdutyPresetResourceName, "entity_type", "alerts"),
					resource.TestCheckResourceAttr(pagerdutyPresetResourceName, "connector_type", "pagerduty"),
					resource.TestCheckResourceAttr(pagerdutyPresetResourceName, "parent_id", "preset_system_pagerduty_alerts_basic"),
					resource.TestCheckResourceAttr(pagerdutyPresetResourceName, "config_overrides.#", "1"),
					resource.TestCheckTypeSetElemNestedAttrs("coralogix_preset.pagerduty_example", "config_overrides.*", map[string]string{
						"condition_type.match_entity_type.%": "0",
						"message_config.fields.#":            "3",
					}),
					resource.TestCheckTypeSetElemNestedAttrs(pagerdutyPresetResourceName, "config_overrides.*.message_config.fields.*", map[string]string{
						"field_name": "summary",
						"template":   "{{ alertDef.description }}",
					}),
					resource.TestCheckTypeSetElemNestedAttrs(pagerdutyPresetResourceName, "config_overrides.*.message_config.fields.*", map[string]string{
						"field_name": "severity",
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

func TestAccCoralogixResourceEmailPreset(t *testing.T) {
	name := uuid.NewString()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceCoralogixEmailPreset(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(emailPresetResourceName, "id", name),
					resource.TestCheckResourceAttr(emailPresetResourceName, "name", name),
					resource.TestCheckResourceAttr(emailPresetResourceName, "description", "email preset example"),
					resource.TestCheckResourceAttr(emailPresetResourceName, "entity_type", "alerts"),
					resource.TestCheckResourceAttr(emailPresetResourceName, "connector_type", "email"),
					resource.TestCheckResourceAttr(emailPresetResourceName, "parent_id", "preset_system_email_alerts"),
					resource.TestCheckResourceAttr(emailPresetResourceName, "config_overrides.#", "1"),
					resource.TestCheckTypeSetElemNestedAttrs(emailPresetResourceName, "config_overrides.*", map[string]string{
						"payload_type": "email_default",
					}),
					resource.TestCheckTypeSetElemNestedAttrs(emailPresetResourceName, "config_overrides.*.message_config.fields.*", map[string]string{
						"field_name": "customSubject",
						"template":   "{{ alertDef.name }}",
					}),
					resource.TestCheckTypeSetElemNestedAttrs(emailPresetResourceName, "config_overrides.*.message_config.fields.*", map[string]string{
						"field_name": "customContent",
						"template":   "<div>content-example</div>",
					}),
				),
			},
			{
				ResourceName:      emailPresetResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccResourceCoralogixEmailPresetUpdate(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(emailPresetResourceName, "id", name),
					resource.TestCheckResourceAttr(emailPresetResourceName, "name", "email example updated"),
					resource.TestCheckResourceAttr(emailPresetResourceName, "description", "email preset example"),
					resource.TestCheckResourceAttr(emailPresetResourceName, "entity_type", "alerts"),
					resource.TestCheckResourceAttr(emailPresetResourceName, "connector_type", "email"),
					resource.TestCheckResourceAttr(emailPresetResourceName, "parent_id", "preset_system_email_alerts"),
					resource.TestCheckResourceAttr(emailPresetResourceName, "config_overrides.#", "1"),
					resource.TestCheckTypeSetElemNestedAttrs(emailPresetResourceName, "config_overrides.*.message_config.fields.*", map[string]string{
						"field_name": "customSubject",
						"template":   "{{ alertDef.name }} - updated",
					}),
				),
			},
		},
	})
}

func testAccResourceCoralogixGenericHttpsPreset(name string) string {
	return fmt.Sprintf(`
	resource "coralogix_preset" "generic_https_example" {
      id               = "%[1]v"
      name             = "%[1]v"
      description      = "generic_https preset example"
      entity_type      = "alerts"
      connector_type   = "generic_https"
      parent_id        = "preset_system_generic_https_alerts_empty"
      config_overrides = [
        {
          condition_type = {
            match_entity_type_and_sub_type = {
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
  `, name)
}

func testAccResourceCoralogixGenericHttpsPresetUpdate(name string) string {
	return fmt.Sprintf(`resource "coralogix_preset" "generic_https_example" {
      id               = "%[1]v"
      name             = "generic_https example updated"
      description      = "generic_https preset example"
      entity_type      = "alerts"
      connector_type   = "generic_https"
      parent_id        = "preset_system_generic_https_alerts_empty"
      config_overrides = [
        {
          condition_type = {
            match_entity_type_and_sub_type = {
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
  `, name)
}

func testAccResourceCoralogixSlackPreset(name string) string {
	return fmt.Sprintf(`resource "coralogix_preset" "slack_example" {
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
  `, name)
}

func testAccResourceCoralogixSlackPresetUpdate(name string) string {
	return fmt.Sprintf(`resource "coralogix_preset" "slack_example" {
      id               = "%[1]v"
      name             = "%[1]v"
      description      = "slack preset example updated"
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
  `, name)
}

func testAccResourceCoralogixPagerdutyPreset(name string) string {
	return fmt.Sprintf(`resource "coralogix_preset" "pagerduty_example" {
      id               = "%[1]v"
      name             = "%[1]v"
      description      = "pagerduty preset example"
      entity_type      = "alerts"
      connector_type   = "pagerduty"
      parent_id        = "preset_system_pagerduty_alerts_basic"
      config_overrides = [
        {
          condition_type = {
            match_entity_type = {
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
                {%% if alert.highestPriority | default(value = alertDef.priority) == 'P1' %%}
                critical
                {%% elif alert.highestPriority | default(value = alertDef.priority) == 'P2' %%}
                error
                {%% elif alert.highestPriority | default(value = alertDef.priority) == 'P3' %%}
                warning
                {%% elif alert.highestPriority | default(value = alertDef.priority) == 'P4' or alert.highestPriority | default(value = alertDef.priority)  == 'P5' %%}
                info
                {%% else %%}
                info
                {%% endif %%}
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
  `, name)
}

func testAccResourceCoralogixPagerdutyPresetUpdate(name string) string {
	return fmt.Sprintf(`resource "coralogix_preset" "pagerduty_example" {
      id               = "%[1]v"
      name             = "pagerduty example updated"
      description      = "pagerduty preset example"
      entity_type      = "alerts"
      connector_type   = "pagerduty"
      parent_id        = "preset_system_pagerduty_alerts_basic"
      config_overrides = [
        {
          condition_type = {
            match_entity_type = {
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
                {%% if alert.highestPriority | default(value = alertDef.priority) == 'P1' %%}
                critical
                {%% elif alert.highestPriority | default(value = alertDef.priority) == 'P2' %%}
                error
                {%% elif alert.highestPriority | default(value = alertDef.priority) == 'P3' %%}
                warning
                {%% elif alert.highestPriority | default(value = alertDef.priority) == 'P4' or alert.highestPriority | default(value = alertDef.priority)  == 'P5' %%}
                info
                {%% else %%}
                info
                {%% endif %%}
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
  `, name)
}

func testAccResourceCoralogixEmailPreset(name string) string {
	return fmt.Sprintf(`resource "coralogix_preset" "email_example" {
      id             = "%[1]v"
      name           = "%[1]v"
      description    = "email preset example"
      entity_type    = "alerts"
      connector_type = "email"
      parent_id      = "preset_system_email_alerts"
      config_overrides = [
        {
          payload_type = "email_default"
          condition_type = {
            match_entity_type = {}
          }
          message_config = {
            fields = [
              {
                field_name = "customSubject"
                template   = "{{ alertDef.name }}"
              },
              {
                field_name = "customContent"
                template   = "<div>content-example</div>"
              }
            ]
          }
        }
      ]
    }
  `, name)
}

func testAccResourceCoralogixEmailPresetUpdate(name string) string {
	return fmt.Sprintf(`resource "coralogix_preset" "email_example" {
      id             = "%[1]v"
      name           = "email example updated"
      description    = "email preset example"
      entity_type    = "alerts"
      connector_type = "email"
      parent_id      = "preset_system_email_alerts"
      config_overrides = [
        {
          payload_type = "email_default"
          condition_type = {
            match_entity_type = {}
          }
          message_config = {
            fields = [
              {
                field_name = "customSubject"
                template   = "{{ alertDef.name }} - updated"
              },
              {
                field_name = "customContent"
                template   = "<div>content-example</div>"
              }
            ]
          }
        }
      ]
    }
  `, name)
}
