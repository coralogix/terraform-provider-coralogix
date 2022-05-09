---
subcategory: "Alerts"
page_title: "Coralogix: coralogix_alert"
---

# coralogix_alert

Provides the Coralogix Alert resource. This allows Alert to be created, updated, and deleted.

The specification of Coralogix API requires recreation of this resource everytime when you'll modify it.

## Example Usage

```hcl
# Create "Standard Alert" Alert
resource "coralogix_alert" "standard_alert" {
    name     = "Standard Alert"
    severity = "critical"
    enabled  = true
    type     = "text"
    filter {
        text         = ".*ERROR.*"
        applications = []
        subsystems   = []
        severities   = []
    }
    condition {
        condition_type = "more_than"
        threshold      = 10
        timeframe      = "30MIN"
    }
    notifications {
        emails = [
            "user@example.com"
        ]
    }
}

# Create "Unique Count Alert" Alert
resource "coralogix_alert" "unique_count_alert" {
    name     = "Unique Count Alert"
    severity = "info"
    enabled  = true
    type     = "unique_count"
    filter {
        text         = ".*INFO.*"
        applications = []
        subsystems   = []
        severities   = []
    }
    condition {
        condition_type = "more_than"
        threshold      = 10
        timeframe      = "1H"
        group_by       = "severity"
    }
    notifications {
        emails = [
            "user@example.com"
        ]
    }
}

# Create "Time Relative Alert" Alert
resource "coralogix_alert" "unique_count_alert" {
    name     = "Time Relative Alert"
    severity = "info"
    enabled  = true
    type     = "relative_time"
    filter {
        text         = ""
        applications = []
        subsystems   = []
        severities   = []
    }
    condition {
        condition_type = "more_than"
        threshold      = 10
        timeframe      = "1H"
    }
    notifications {
        emails = [
            "user@example.com"
        ]
    }
}


# Create "New Value Alert" Alert
resource "coralogix_alert" "new_value_alert" {
    name     = "New Value Alert"
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
        condition_type = "new_value"
        timeframe      = "12H"
        group_by       = "my_field"
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
* `type` - (Required) Alert type, one of the following: `text`, `ratio`, `unique_count`, `relative_time`, `metric`. For `new_value` alerts the value should be `text`.
* `severity` - (Required) Alert severity, one of the following: `info`, `warning`, `critical`.
* `enabled` - (Required) Alert state.
* `filter` - (Required) A `filter` block as documented below.
* `description` - (Optional) Alert description.
* `metric` - (Optional) A `metric` block as documented below.
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

Each `metric` block should contains the following:

* `field` - (Required) The name of the metric field to alert on.
* `source` - (Required) The source of the metric. Either `logs2metrics` or `Prometheus`.
* `arithmetic_operator` - (Required) `0` - avg, `1` - min, `2` - max, `3` - sum, `4` - count, `5` - percentile (for percentile you need to supply the requested percentile in arithmetic_operator_modifier).
* `arithmetic_operator_modifier` - (Optional) For `percentile(5)` `arithmetic_operator` you need to supply the value in this property.
* `sample_threshold_percentage` - (Required) The metric value must cross the threshold within this percentage of the timeframe (sum and count arithmetic operators do not use this parameter since they aggregate over the entire requested timeframe), `increments of 10`, `0 <= value <= 90`.
* `non_null_percentage` - (Required) The minimum percentage of the timeframe that should have values for this alert to trigger, `increments of 10`, `0 <= value <= 100`.
* `swap_null_values` - (Optional) If set to `true`, missing data will be considered as 0, otherwise, it will not be considered at all.

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