---
subcategory: "Alerts"
page_title: "Coralogix: coralogix_alert"
---

# coralogix_alert

Use this data source to retrieve information about a Coralogix Alert.

## Example Usage
Once an alert is created with TF you mau wish to refer to it.
while the alert will have a UUID it's attributes may be caleld upon by name.
```hcl
data "coralogix_alert" "my_alert" {
    alert_id        = "3dd35de0-0e10-11eb-9d0f-a1073519a608"
}
```

Using a code exmple like this will output the id of suce an alert:
```hcl
output "name" {
  value       = coralogix_alert.my_alert.alert_id
  description = "Alert name."
}
```

## Argument Reference

* `alert_id` - (Required) Alert ID.

## Attribute Reference
The result is an object containing the following attributes.
* `name` - Alert name.
* `description` - Alert description.
* `severity` - Alert severity.
* `enabled` - Alert state.
* `type` - Alert type.
* `filter` - Alert filter.
* `condition` - Alert condition.
* `content` - List of fields attached to alert notification.
* `schedule` - Configuration of period when alert triggering will be allowed.
* `notifications` - Alert notifications.