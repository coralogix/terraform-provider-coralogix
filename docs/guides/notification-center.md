# Managing Coralogix Notification Center with the Terraform Provider

## Overview
This guide walks you through configuring Coralogix Notification Center resources using the Terraform Provider. 
It introduces the core components of the Notification Center and demonstrates how to integrate them into your 
Terraform configurations to control how alerts are formatted, routed, and delivered to external systems.

For more information on the Notification Center, refer to the [official documentation](https://coralogix.com/docs/user-guides/notification-center/introduction/).

## Notification Center Components
Coralogix Notification Center allows you to define and manage how alerts are dispatched to external systems such as 
Slack, PagerDuty, and generic webhooks. 
It offers flexible routing, templating, and integration capabilities through the following resource types:

### Connector
Defines the external destination for notifications. 
Connectors determine where alerts are sent—such as a Slack channel, PagerDuty service, or webhook endpoint.
Current supported connector types include:
- Slack
- PagerDuty
- Generic Webhook

### Preset
Defines the structure and content of the notification message. Coralogix provides system Presets for common use cases,
and allows configuring custom Presets, which inherit from system Presets but can be customized further. 
Just like connectors, presets are tailored for specific platforms like Slack, PagerDuty, and webhooks.

### Global Router
Determines how alerts are routed to specific connectors and presets. 
A Global Router evaluates routing rules based on alert conditions and matches them to appropriate notification targets.
The default Global Router is called `router_default`, if you name your resource like this, and `terraform apply`, you can simply overtake it instead of having to `terraform import`.

### Alerts
There are two ways to configure notification behavior in an alert:
1. **Using Global Routers**: Alerts are routed through a centralized Global Router, 
which applies logic to determine the appropriate connector and preset.

## Example: Slack Notification Center Configuration
The following sections demonstrate how to configure a Slack-based notification workflow using all Notification Center components.

### Connector Configuration
The following resource defines a Slack connector with fallback logic and a dynamic override based on the alert's `channel` label:
```hcl
resource "coralogix_connector" "slack_example" {
  name        = "slack connector"
  description = "slack connector example"
  type        = "slack" # The type of connector, in this case, Slack.
  connector_config = {
    fields = [
      {
        field_name = "integrationId"
        value      = "slack-integration-id" # A provided integration ID for the Slack connector.
      },
      {
        field_name = "channel"
        value      = "channel-example" # The primary Slack channel where notifications will be sent to.
      },
      {
        field_name = "fallbackChannel"
        value      = "fallback-channel-example" # The fallback channel to use if the primary channel is not available.
      }
    ]
  }

  config_overrides = [ # Optional overrides for the connector configuration, based on entity type.
    {
      entity_type = "alerts" # The entity type to apply the override for. Allows using alerts schema in the override.
      fields = [
        {
          field_name = "channel" # Override the channel field for alerts.
          template   = "{{alertDef.entityLabels.channel}}" # Use a template to dynamically set the channel based on alert labels.
        }
      ]
    }
  ]
}
```
Running `terraform apply` with the above configuration will create a Connector in Coralogix, as shown in the screenshot below:

<img width="1711" height="880" alt="Screenshot of a connector on the Coralogix web UI" src="https://github.com/user-attachments/assets/c8120831-bbd3-49cc-87fa-5ba6677e73f6" />

---

### Preset Configuration
The following resource defines a Slack preset for formatting messages, with overrides based on the alert subtype:
```hcl
resource "coralogix_preset" "slack_example" {
  name        = "slack example"
  description = "slack preset example"
  connector_type = "slack" # The type of connector this preset is designed for, in this case, Slack.
  entity_type = "alerts" # The entity type for which this preset is applicable.
  parent_id = "preset_system_slack_alerts_basic" # The ID of the parent preset to inherit default configurations.
  config_overrides = [
    # Optional overrides for the preset configuration, based on entity sub type.
    {
      condition_type = {
        match_entity_type_and_sub_type = {
          entity_sub_type = "logsImmediateResolved" # This override applies to alerts of type logsImmediateResolved.
        }
      }
      message_config = {
        fields = [ # Fields to override in the Slack message.
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
```
Running `terraform apply` with the above configuration will create a Preset in Coralogix, as shown in the screenshot below:

<img width="1720" height="845" alt="Screenshot of a preset on the Coralogix web UI" src="https://github.com/user-attachments/assets/6aa676e3-4bee-4f8f-86da-b53dc3253791" />

---

### Global Router Configuration
The following resource defines a Global Router that dispatches alerts based on priority:
```hcl
resource "coralogix_global_router" "router_example" {
  name        = "global router"
  description = "global router example"
  entity_type = "alerts" # The entity type for which this router is applicable.
  rules = [
    {
      name      = "P1-alerts"
      condition = "alertDef.priority == \"P1\"" # Condition to match P1 alerts.
      targets = [
        {
          connector_id = coralogix_connector.slack_example.id # Slack connector to use for P1 alerts.
          preset_id    = coralogix_preset.slack_example.id # Slack Preset to apply for P1 alerts.
        }
      ]
    },
    {
      name      = "P2-alerts"
      condition = "alertDef.priority == \"P2\"" # Condition to match P2 alerts.
      targets = [
        {
          connector_id = coralogix_connector.slack_example.id # Slack connector to use for P2 alerts.
          preset_id    = "preset_system_slack_alerts_basic" # Slack Preset to apply for P1 alerts.
        }
      ]
    }
  ]
}
```
Running `terraform apply` with the above configuration will create a Global Router in Coralogix, as shown in the screenshot below:

<img width="1715" height="851" alt="Screenshot of a router on the Coralogix web UI" src="https://github.com/user-attachments/assets/248da4ed-fbdc-43d1-b15a-1290e9827a10" />

---


### Alert using Global Router Configuration
The following resource defines an Alert that uses the Global Router for routing notifications:
```hcl
resource "coralogix_alert" "example_with_router" {
  depends_on = [coralogix_global_router.router_example]
  name        = "metric_threshold alert"
  description = "metric_threshold alert example with routing"
  notification_group = {
    router = {} # Enabling routing to the Global Router.
  }
  type_definition = {
    metric_threshold = {
      metric_filter = {
        promql = "sum(rate(http_requests_total{job=\"api-server\"}[5m])) by (status)"
      }
      rules = [{
        condition = {
          threshold    = 2
          for_over_pct = 10
          of_the_last = "1h15m"
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
```
Running `terraform apply` with the above configuration will create an Alert in Coralogix, as shown in the screenshot below:

<img width="1709" height="884" alt="Screenshot of an alert on the Coralogix web UI" src="https://github.com/user-attachments/assets/5f2b2759-4a31-4393-89b7-b16b89d06684" />

---

### Alert using Router Configuration
The following resource defines an Alert that directly references a connector and preset for notifications:
```hcl
resource "coralogix_alert" "example_with_router" {
  name        = "metric_threshold alert"
  description = "metric_threshold alert example with router"
  notification_group = {
    router = {
      notify_on = "Triggered and Resolved" # Specifies when to notify (on alert trigger and resolution).
    }
  }
  type_definition = {
    metric_threshold = {
      metric_filter = {
        promql = "sum(rate(http_requests_total{job=\"api-server\"}[5m])) by (status)"
      }
      rules = [{
        condition = {
          threshold    = 2
          for_over_pct = 10
          of_the_last = "1h15m"
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
```
Running `terraform apply` with the above configuration will create an Alert in Coralogix, as shown in the screenshot below:

<img width="1706" height="880" alt="Screenshot of an alert on the Coralogix web UI" src="https://github.com/user-attachments/assets/4ae5619b-d118-4566-a3f2-a12d3eeed6f2" />

---

## Conclusion
This guide provided an overview of how to configure Coralogix Notification Center resources using Terraform.
You learned how to create connectors, presets, and global routers, and how to use them in alerts configurations.
