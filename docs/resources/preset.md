---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "coralogix_preset Resource - terraform-provider-coralogix"
subcategory: ""
description: |-
  Coralogix Preset. NOTE: This resource is in alpha stage.
---

# coralogix_preset (Resource)

Coralogix Preset. **NOTE:** This resource is in alpha stage.

## Example Usage

```terraform
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
            template   = "{{ alert.timestamp }}"
          }
        ]
      }
    }
  ]
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `connector_type` (String) The type of connector for the preset. Valid values are: generic_https, pagerduty, slack, unspecified
- `entity_type` (String) The type of entity for the preset. Valid values are: alerts, unspecified
- `name` (String)
- `parent_id` (String)

### Optional

- `config_overrides` (Attributes List) (see [below for nested schema](#nestedatt--config_overrides))
- `description` (String)
- `id` (String) The ID of the Preset. Can be set to a custom value, or left empty to auto-generate. Requires recreation in case of change.

<a id="nestedatt--config_overrides"></a>
### Nested Schema for `config_overrides`

Required:

- `condition_type` (Attributes) Condition type for the preset. Must be either match_entity_type or match_entity_type_and_sub_type. (see [below for nested schema](#nestedatt--config_overrides--condition_type))
- `message_config` (Attributes) (see [below for nested schema](#nestedatt--config_overrides--message_config))

Optional:

- `payload_type` (String)

<a id="nestedatt--config_overrides--condition_type"></a>
### Nested Schema for `config_overrides.condition_type`

Optional:

- `match_entity_type` (Attributes) (see [below for nested schema](#nestedatt--config_overrides--condition_type--match_entity_type))
- `match_entity_type_and_sub_type` (Attributes) (see [below for nested schema](#nestedatt--config_overrides--condition_type--match_entity_type_and_sub_type))

<a id="nestedatt--config_overrides--condition_type--match_entity_type"></a>
### Nested Schema for `config_overrides.condition_type.match_entity_type`


<a id="nestedatt--config_overrides--condition_type--match_entity_type_and_sub_type"></a>
### Nested Schema for `config_overrides.condition_type.match_entity_type_and_sub_type`

Required:

- `entity_sub_type` (String)



<a id="nestedatt--config_overrides--message_config"></a>
### Nested Schema for `config_overrides.message_config`

Required:

- `fields` (Attributes Set) (see [below for nested schema](#nestedatt--config_overrides--message_config--fields))

<a id="nestedatt--config_overrides--message_config--fields"></a>
### Nested Schema for `config_overrides.message_config.fields`

Required:

- `field_name` (String)
- `template` (String)
