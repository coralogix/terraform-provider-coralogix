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

//Examples of connectors
resource "coralogix_connector" "generic_https_example" {
  id               = "generic_https_example" //This field is optional, if not provided a random id will be generated
  type             = "generic_https"
  name             = "generic-https connector"
  description      = "generic-https connector example"
  connector_config = {
    fields = [
      {
        field_name = "url"
        value      = "https://api.opsgenie.com/v2/alerts"
      },
      {
        field_name = "method"
        value      = "POST"
      },
      {
        field_name = "additionalHeaders"
        value      = jsonencode(
          {
            "Authorization" : "GenieKey <key>",
            "Content-Type" : "application/json"
          })
      },
      {
        field_name = "additionalBodyFields"
        value      = jsonencode(
          {
            alias = "{{alert.groupingKey}}"
          })
      }
    ]
  }
  config_overrides = [
    {
      entity_type = "alerts"
      fields      = [
        {
          field_name = "url"
          template   = <<EOF
            {% if alert.status == 'Triggered' %}
            https://api.opsgenie.com/v2/alerts
            {% else %}
            https://api.opsgenie.com/v2/alerts/{{alert.groupingKey}}/close?identifierType=alias
            {% endif %}
EOF
        },
        {
          field_name = "additionalHeaders"
          template   = <<EOF
                {
                 "Authorization": "GenieKey some-key",
                 "Content-Type": "application/json"
              }
EOF
        },
        {
          field_name : "additionalBodyFields"
          template : <<EOF
          {
            "alias": "{{alert.groupingKey}}"
          }
EOF
        }
      ]
    }
  ]
}

resource "coralogix_connector" "slack_example" {
  type             = "slack"
  name             = "slack connector"
  description      = "slack connector example"
  connector_config = {
    fields = [
      {
        field_name = "integrationId"
        value      = "iac-internal"
      },
      {
        field_name = "fallbackChannel"
        value      = "iac-internal"
      },
      {
        field_name = "channel"
        value      = "iac-internal"
      }
    ]
  }
  config_overrides = [
    {
      entity_type = "alerts"
      fields      = [
        {
          field_name = "channel"
          template   = <<EOF
            {% if alert.groups[0].keyValues[alertDef.groupByKeys[1]]|lower == "sample" %}
            sample-channel
            {% elif alert.groups[0].keyValues[alertDef.groupByKeys[1]]|lower == "another" %}
            another-channel
            {% else %}
            generic-channel
            {% endif %}
EOF
        }
      ]
    }
  ]
}

resource "coralogix_connector" "pagerduty_example" {
  type             = "pagerduty"
  name             = "pagerduty connector"
  description      = "pagerduty connector example"
  connector_config = {
    fields = [
      {
        field_name = "integrationKey"
        value      = "integrationKey-example"
      }
    ]
  }
  config_overrides = [
    {
      entity_type = "alerts"
      fields      = [
        {
          field_name = "integrationKey"
          template   = <<EOF
            {% if alert.groups[0].keyValues[alertDef.groupByKeys[1]]|lower == "sample" %}
            sample-integration-key
            {% elif alert.groups[0].keyValues[alertDef.groupByKeys[1]]|lower == "another" %}
            another-integrations-key
            {% else %}
            generic-integration-key
            {% endif %}
EOF
        }
      ]
    }
  ]
}


//Examples of presets
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
          entity_type     = "alerts"
          entity_sub_type = "logsImmediateResolved"
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
          entity_type     = "alerts"
          entity_sub_type = "logsImmediateResolved"
        }
      }
      message_config = {
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

//Examples of global router
resource "coralogix_global_router" "example" {
  name        = "global router example"
  description = "global router example"
  entity_type = "alerts"
  rules       = [
    {
      name      = "rule-name"
      condition = "alertDef.priority == \"P1\""
      targets   = [
        {
          connector_id = coralogix_connector.generic_https_example.id
          preset_id    = coralogix_preset.generic_https_example.id
        },
        {
          connector_id = coralogix_connector.slack_example.id
          preset_id    = coralogix_preset.slack_example.id
        },
        {
          connector_id = coralogix_connector.pagerduty_example.id
          preset_id    = coralogix_preset.pagerduty_example.id
        }
      ]
    }
  ]
}