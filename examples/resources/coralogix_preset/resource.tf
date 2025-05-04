terraform {
  required_providers {
    coralogix = {
      version = "~> 2.0"
      source  = "coralogix/coralogix"
    }
  }
}

provider "coralogix" {
  #api_key = "<add your api key here or add env variable CORALOGIX_API_KEY>"
  #env = "<add the environment you want to work at or add env variable CORALOGIX_ENV>"
}

resource "coralogix_preset" "generic_https_example" {
  id               = "generic_https_example"
  name             = "generic_https example"
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
  id               = "slack_example"
  name             = "slack example"
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
  id               = "pagerduty_example"
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