terraform {
  required_providers {
    coralogix = {
      version = "~> 3.0"
      source  = "coralogix/coralogix"
    }
  }
}

provider "coralogix" {
  #api_key = "<add your api key here or add env variable CORALOGIX_API_KEY>"
  #env = "<add the environment you want to work at or add env variable CORALOGIX_ENV>"
}

# A secret comes in through a variable that is both sensitive and ephemeral.
# sensitive hides the value in the plan and apply output.
# ephemeral keeps it out of the state and out of the saved plan file.
# Terraform lets an ephemeral value feed a write-only attribute only, such as field_values_wo.
# Set the value with the environment variable TF_VAR_<name>.
variable "pagerduty_integration_key" {
  type      = string
  sensitive = true
  ephemeral = true
}

variable "service_now_password" {
  type      = string
  sensitive = true
  ephemeral = true
}

resource "coralogix_connector" "generic_https_example" {
  id          = "generic_https_example" //This field is optional, if not provided a random id will be generated
  type        = "generic_https"
  name        = "generic-https connector"
  description = "generic-https connector example"
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
        field_name = "additionalBodyFields"
        value = jsonencode(
          {
            alias = "{{alert.groupingKey}}"
        })
      }
    ]
    # additionalHeaders carries the API key, so it is set here rather than in
    # fields: Terraform sends these values to Coralogix and never writes them
    # to state. Increment the matching version to send a rotated secret.
    field_values_wo = {
      additionalHeaders = jsonencode(
        {
          "Authorization" : "GenieKey <key>",
          "Content-Type" : "application/json"
      })
    }
    field_values_wo_versions = {
      additionalHeaders = 1
    }
  }
  config_overrides = [
    {
      entity_type = "alerts"
      fields = [
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
  type        = "slack"
  name        = "slack connector"
  description = "slack connector example"
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
      fields = [
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
  type        = "pagerduty"
  name        = "pagerduty connector"
  description = "pagerduty connector example"
  connector_config = {
    fields = []
    # The integration key is a secret, so it is set here rather than in fields.
    # Increment the matching version to send a rotated key.
    field_values_wo = {
      integrationKey = var.pagerduty_integration_key
    }
    field_values_wo_versions = {
      integrationKey = 1
    }
  }
  config_overrides = [
    {
      entity_type = "alerts"
      fields = [
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

resource "coralogix_connector" "email_example" {
  type        = "email"
  name        = "email connector"
  description = "email connector example"
  connector_config = {
    fields = [
      {
        field_name = "emailAddresses"
        value      = "[\"email1@example.com\",\"email2@example.com\"]"
      }
    ]
  }
}

resource "coralogix_connector" "pagerduty_incidents_example" {
  type        = "pagerduty_incidents"
  name        = "pagerduty incidents connector"
  description = "pagerduty incidents connector example"
  connector_config = {
    fields = [
      {
        field_name = "integrationId"
        value      = "integrationId-example"
      },
      {
        field_name = "service"
        value      = "PXXXXXX"
      }
    ]
  }
}

resource "coralogix_connector" "service_now_example" {
  type        = "service_now"
  name        = "service now connector"
  description = "service now connector example"
  connector_config = {
    fields = [
      {
        field_name = "instanceUrl"
        value      = "https://your-instance.service-now.com"
      },
      {
        field_name = "username"
        value      = "username-example"
      }
    ]
    field_values_wo = {
      password = var.service_now_password
    }
    field_values_wo_versions = {
      password = 1
    }
  }
}

resource "coralogix_connector" "microsoft_teams_example" {
  type        = "microsoft_teams"
  name        = "microsoft teams connector"
  description = "microsoft teams connector example"
  connector_config = {
    fields = [
      {
        field_name = "integrationId"
        value      = "integrationId-example"
      },
      {
        field_name = "teamId"
        value      = "teamId-example"
      },
      {
        field_name = "channelId"
        value      = "19:channel-id@thread.tacv2"
      }
    ]
  }
}

resource "coralogix_connector" "eventbridge_example" {
  type        = "eventbridge"
  name        = "eventbridge connector"
  description = "eventbridge connector example"
  connector_config = {
    fields = [
      {
        field_name = "integrationId"
        value      = "integrationId-example"
      },
      {
        field_name = "additionalDetail"
        value      = "{\"pipeline_id\":\"p123\"}"
      }
    ]
  }
}

resource "coralogix_connector" "incident_io_example" {
  type        = "incident_io" # Preview connector type.
  name        = "incident.io connector"
  description = "incident.io connector example"
  connector_config = {
    fields = [
      {
        field_name = "apiKey"
        value      = "apiKey-example"
      },
      {
        field_name = "alertEventsUrl"
        value      = "https://api.incident.io/v2/alert_events/http/source-id"
      },
      {
        field_name = "alertSourceToken"
        value      = "alertSourceToken-example"
      }
    ]
  }
}

