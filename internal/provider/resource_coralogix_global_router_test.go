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

const globalRouterResourceName = "coralogix_global_router.example"

func TestAccCoralogixResourceGlobalRouter(t *testing.T) {
	name := uuid.NewString()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceCoralogixGlobalRouter(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(globalRouterResourceName, "name", "global router example"),
					resource.TestCheckResourceAttr(globalRouterResourceName, "description", "global router example"),
					resource.TestCheckTypeSetElemNestedAttrs(globalRouterResourceName, "rules.*", map[string]string{
						"name":      "rule-name",
						"condition": "alertDef.priority == \"P1\"",
					}),
					resource.TestCheckTypeSetElemNestedAttrs(globalRouterResourceName, "rules.*.targets.*", map[string]string{
						"connector_id": name,
						"preset_id":    name,
					}),
					resource.TestCheckTypeSetElemNestedAttrs(globalRouterResourceName, "rules.*.targets.*", map[string]string{
						"connector_id": name,
						"preset_id":    name,
					}),
					resource.TestCheckTypeSetElemNestedAttrs(globalRouterResourceName, "rules.*.targets.*", map[string]string{
						"connector_id": name,
						"preset_id":    name,
					}),
				),
			},
			{
				ResourceName:      globalRouterResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccResourceCoralogixGlobalRouterUpdate(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(globalRouterResourceName, "name", "global router example updated"),
					resource.TestCheckResourceAttr(globalRouterResourceName, "description", "global router example"),
					resource.TestCheckResourceAttr(globalRouterResourceName, "entity_type", "alerts"),
					resource.TestCheckTypeSetElemNestedAttrs(globalRouterResourceName, "rules.*", map[string]string{
						"name":      "rule-name",
						"condition": "alertDef.priority == \"P1\"",
					}),
					resource.TestCheckTypeSetElemNestedAttrs(globalRouterResourceName, "rules.*.targets.*", map[string]string{
						"connector_id": name,
						"preset_id":    name,
					}),
					resource.TestCheckTypeSetElemNestedAttrs(globalRouterResourceName, "rules.*.targets.*", map[string]string{
						"connector_id": name,
						"preset_id":    name,
					}),
					resource.TestCheckTypeSetElemNestedAttrs(globalRouterResourceName, "rules.*.targets.*", map[string]string{
						"connector_id": name,
						"preset_id":    name,
					}),
				),
			},
		},
	})
}

func testAccResourceCoralogixGlobalRouter(name string) string {
	return fmt.Sprintf(`
    resource "coralogix_connector" "generic_https_example" {
      id               = "http-%[1]v"
      name             = "http-%[1]v"
      type             = "generic_https"
      description      = "generic-https connector example"
      connector_config = {
        fields = [
          {
            field_name = "url"
            value      = "https://httpbin.org/post"
          },
          {
            field_name = "method"
            value      = "post"
          }
        ]
      }
    }
    
    resource "coralogix_connector" "slack_example" {
      id               = "slack-%[1]v"
      name             = "slack-%[1]v"
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
    
    resource "coralogix_connector" "pagerduty_example" {
      id               = "pd-%[1]v"
      name             = "pd-%[1]v"
      type             = "pagerduty"
      description      = "pagerduty connector example"
      connector_config = {
        fields = [
          {
            field_name = "integrationKey"
            value      = "integrationKey-eample"
          }
        ]
      }
    }
    
    
    resource "coralogix_preset" "generic_https_example" {
      id               = "http-%[1]v"
      name             = "http-%[1]v"
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
    
    resource "coralogix_preset" "slack_example" {
      id               = "slack-%[1]v"
      name             = "slack-%[1]v"
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
    
    resource "coralogix_preset" "pagerduty_example" {
      id               = "pd-%[1]v"
      name             = "pd-%[1]v"
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
                template   = "{{ alert.timestamp }}"
              }
            ]
          }
        }
      ]
    }

    resource "coralogix_global_router" "example" {
      name        = "global router example"
      description = "global router example"
      entity_type = "unspecified"
      rules       = [
        {
          name = "rule-name"
          condition = "alertDef.priority == \"P1\""
          targets = [
            {
              connector_id   = coralogix_connector.generic_https_example.id
              preset_id      = coralogix_preset.generic_https_example.id
            },
            {
              connector_id   = coralogix_connector.slack_example.id
              preset_id      = coralogix_preset.slack_example.id
            },
            {
              connector_id   = coralogix_connector.pagerduty_example.id
              preset_id      = coralogix_preset.pagerduty_example.id
            }
          ]
        }
      ]
    }
  `, name)
}

func testAccResourceCoralogixGlobalRouterUpdate(name string) string {
	return fmt.Sprintf(`
    resource "coralogix_connector" "generic_https_example" {
      id               = "http-%[1]v"
      name             = "http-%[1]v"
      type             = "generic_https"
      description      = "generic-https connector example"
      connector_config = {
        fields = [
          {
            field_name = "url"
            value      = "https://httpbin.org/post"
          },
          {
            field_name = "method"
            value      = "post"
          }
        ]
      }
    }
    
    resource "coralogix_connector" "slack_example" {
      id               = "slack-%[1]v"
      name             = "slack-%[1]v"
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
    
    resource "coralogix_connector" "pagerduty_example" {
      id               = "pd-%[1]v"
      name             = "pd-%[1]v"
      type             = "pagerduty"
      description      = "pagerduty connector example"
      connector_config = {
        fields = [
          {
            field_name = "integrationKey"
            value      = "integrationKey-eample"
          }
        ]
      }
    }
    
    
    resource "coralogix_preset" "generic_https_example" {
      id               = "http-%[1]v"
      name             = "http-%[1]v"
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
    
    resource "coralogix_preset" "slack_example" {
      id               = "slack-%[1]v"
      name             = "slack-%[1]v"
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
    
    resource "coralogix_preset" "pagerduty_example" {
      id               = "pd-%[1]v"
      name             = "pd-%[1]v"
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
                template   = "{{ alert.timestamp }}"
              }
            ]
          }
        }
      ]
    }

    resource "coralogix_global_router" "example" {
      name        = "global router example updated"
      description = "global router example"
      entity_type = "alerts"
      rules       = [
        {
          name = "rule-name"
          condition = "alertDef.priority == \"P1\""
          targets = [
            {
              connector_id   = coralogix_connector.generic_https_example.id
              preset_id      = coralogix_preset.generic_https_example.id
            },
            {
              connector_id   = coralogix_connector.slack_example.id
              preset_id      = coralogix_preset.slack_example.id
            },
            {
              connector_id   = coralogix_connector.pagerduty_example.id
              preset_id      = coralogix_preset.pagerduty_example.id
            }
          ]
        }
      ]
    }
  `, name)
}
