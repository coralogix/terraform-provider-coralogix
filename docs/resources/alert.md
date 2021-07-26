---
subcategory: "Alerts"
page_title: "Coralogix: coralogix_alert"
---

# coralogix_alert

Provides the Coralogix Alert resource. This allows Alert to be created, updated, and deleted.

## Example Usage

```hcl
# Create "My Alert" Alert
resource "coralogix_alert" "example" {
    name     = "My Alert"
    severity = "info"
    enabled  = true
    type     = "text"
    filter {
        text         = ""
        applications = []
        subsystems   = []
        severities   = []
    }
    condition {
        type      = "more_than"
        threshold = 100
        timeframe = "30MIN"
    }
    notifications {
        emails = [
            "user@example.com"
        ]
    }
}
```

## Argument Reference

* `name` - (Required) Alert name.
* `type` - (Required) Alert type, one of the following: `text`, `ratio`.
* `severity` - (Required) Alert severity, one of the following: `info`, `warning`, `critical`.
* `enabled` - (Required) Alert state.
* `filter` - (Required) A `filter` block as documented below.
* `description` - (Optional) Alert description.
* `condition` - (Optional) A `condition` block as documented below.
* `schedule` - (Optional) A `schedule` block as documented below.
* `content` - (Optional) An array that contains log fields to be included with the alert notification.
* `notifications` - (Optional) A `notifications` block as documented below.

---

Each `filter` block should contains the following:

* `text` - (Optional) String query to be alerted on.
* `applications` - (Optional) List of application names to be alerted on.
* `subsystems` - (Optional) List of subsystem names to be alerted on.
* `severities` - (Optional) List of log severities to be alerted on, one of the following: `debug`, `verbose`, `info`, `warning`, `error`, `critical`.

Each `condition` block should contains the following:

* `condition_type` - (Required) Alert condition type, one of the following: `less_than`, `more_than`, `more_than_usual`, `new_value`.
* `threshold` - (Required) Number of log occurrences that is needed to trigger the alert.
* `timeframe` - (Required) The bounded time frame for the threshold to be occurred within, to trigger the alert.
* `group_by` - (Optional) The field to **group by** on.

Each `schedule` block should contains the following:

* `days` - (Required) Days when alert triggering is allowed, one of the following: `Mo`, `Tu`, `We`, `Th`, `Fr`, `Sa`, `Su`.
* `start` - (Required) Time from which alert triggering is allowed, for example `00:00:00`.
* `end` - (Required) Time till which alert triggering is allowed, for example `23:59:59`.

Each `notifications` block should contains the following:

* `emails` - (Optional) List of email address to notify.
* `integrations` - (Optional) List of integration channels to notify.

## Import

Alerts can be imported using their ID.

```
$ terraform import coralogix_alert.alert <alert_id>
```