---
subcategory: "Alerts"
page_title: "Coralogix: coralogix_alert"
---

# coralogix_alert

Use this data source to retrieve information about a Coralogix Alert.

## Example Usage
Once an alert is created with TF you may wish to refer to it.
while the alert will have a UUID it's attributes may be called upon by name.
```hcl
data "coralogix_alert" "my_alert" {
    unique_identifier        = "3dd35de0-0e10-11eb-9d0f-a1073519a608"
}
```

Using this code example will output the alert name:
```hcl
output "name" {
  value       = coralogix_alert.my_alert.name
  description = "Alert name."
}
```

## Argument Reference

* `unique_identifier` - (Required) Alert unique identifier.

## Attribute Reference
The result is an object containing the following attributes.
* `name` - Alert name.
* `description` - Alert description.
* `severity` - Alert severity.
* `enabled` - Alert state.
* `type` - Alert type.
* `filter` - Alert filter.
* `metric` - Alert metric.
* `ratio` - Alert ratio.
* `condition` - Alert condition.
* `content` - List of fields attached to alert notification.
* `schedule` - Configuration of period when alert triggering will be allowed.
* `notifications` - Alert notifications.
* `alert_id` - Alert id.
* `notify_every` - The time an alert is suppressed.