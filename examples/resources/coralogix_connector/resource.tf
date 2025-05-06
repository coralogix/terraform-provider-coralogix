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