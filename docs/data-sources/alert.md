---
subcategory: "Alerts"
page_title: "Coralogix: coralogix_alert"
---

# coralogix_alert

Use this data source to retrieve information about a Coralogix Alert.

## Example Usage

```hcl
data "coralogix_alert" "alert" {
    alert_id        = "3dd35de0-0e10-11eb-9d0f-a1073519a608"
}
```

## Argument Reference

* `alert_id` - (Required) Alert ID.

## Attribute Reference

* `name` - Alert name.
* `description` - Alert description.
* `severity` - Alert severity.
* `enabled` - Alert state.
* `type` - Alert type.
* `filter` - Alert filter.
* `condition` - Alert condition.
* `notifications` - Alert notifications.