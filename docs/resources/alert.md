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
        threshold        = 10
        timeframe        = "1H"
        unique_count_key = "concurrent_connections"
    }
    notifications {
        emails = [
            "user@example.com"
        ]
    }
}

# Create "Time Relative Alert" Alert
resource "coralogix_alert" "time_relative_alert" {
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
        condition_type      = "more_than"
        threshold           = 10
        timeframe           = "HOUR"
        relative_timeframe  = "DAY"
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
        unique_count_key = "ip_address"
    }
    notifications {
        emails = [
            "user@example.com"
        ]
    }
}

# Create "Ratio Alert" Alert
resource "coralogix_alert" "ratio_alert" {
    name     = "Ratio Alert"
    severity = "critical"
    enabled  = true
    type     = "ratio"
    filter {
        text         = ""
        applications = ["app1", "app2"]
        subsystems   = []
        severities   = []
        alias        = "query 1"
    }
    ratio {
        text         = ""
        applications = ["app1", "app2"]
        subsystems   = []
        severities   = []
        alias        = "query 2"
        group_by     = []
    }
    condition {
        condition_type = "more_than"
        threshold      = 10
        timeframe      = "30MIN"
        group_by_array       = []
    }
    notifications {
        emails = [
            "user@example.com"
        ]
    }
}

# Create "Metric Alert" Alert
resource "coralogix_alert" "metric_alert" {
    name     = "Metric alert"
    severity = "info"
    enabled  = true
    type     = "metric"
    filter {
        text         = ""
        applications = []
        subsystems   = []
        severities   = []
    }
    metric {
        field                       = "cpuUsagePercent"
        source                      = "prometheus"
        sample_threshold_percentage = 30
        arithmetic_operator         = 2
        non_null_percentage         = 0
    }
    condition {
        condition_type = "more_than"
        threshold      = 80
        timeframe      = "10MIN"
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
* `type` - (Required) Alert type, one of the following: `text`, `unique_count`, `relative_time`, `metric`, `ratio`. For new_value alerts the value should be `text`.
* `severity` - (Required) Alert severity, one of the following: `info`, `warning`, `critical`.
* `enabled` - (Required) Alert state.
* `filter` - (Required) A `filter` block as documented below.
* `description` - (Optional) Alert description.
* `metric` - (Optional) A `metric` block as documented below.
* `ratio` - (Optional) A `ratio` block as documented below.
* `condition` - (Required) A `condition` block as documented below, only optional when using alert of type `text` (standard alert).
* `schedule` - (Optional) A `schedule` block as documented below.
* `content` - (Optional) An array that contains log fields to be included with the alert notification.
* `notifications` - (Optional) A `notifications` block as documented below.
* `notify_every` - (Optional) the time an alert is supressed after it was triggered in seconds, default to `60`. when using condition.condition_type 'less_than' , the value has to be more than the timeframe picked.

---

Each `filter` block should contains the following:

* `text` - (Optional) String query to be alerted on. empty string is for all records without filtering
* `applications` - (Optional) List of application names to be alerted on, an empty list is all application names.
* `subsystems` - (Optional) List of subsystem names to be alerted on, empty list is all subsystem names.
* `severities` - (Optional) List of log severities to be alerted on, one of the following: `debug`, `verbose`, `info`, `warning`, `error`, `critical`, an empty list is all severities.
* `alias` - (Optional) An alias for the query, required only for alerts of type `ratio`.

Each `metric` block should contains the following:

* `field` - (Optional) The name of the metric field to alert on.
* `source` - (Optional) The source of the metric. Either `logs2metrics` or `prometheus`.
* `arithmetic_operator` - (Optional) The arithmetic operator to use on the alert, Integer: `0` - avg, `1` - min, `2` - max, `3` - sum, `4` - count, `5` - percentile (for percentile you need to supply the requested percentile in arithmetic_operator_modifier).
* `arithmetic_operator_modifier` - (Optional) For percentile(5) arithmetic_operator you need to supply the value in this property, `0 < value < 100`.
* `sample_threshold_percentage` - (Required) The metric value must cross the threshold within this percentage of the timeframe (sum and count arithmetic operators do not use this parameter since they aggregate over the entire requested timeframe), `increments of 10`, `0 <= value <= 90`.
* `non_null_percentage` - (Required) The minimum percentage of the timeframe that should have values for this alert to trigger, `increments of 10`, `0 <= value <= 100`.
* `swap_null_values` - (Optional) If set to `true`, missing data will be considered as 0, otherwise, it will not be considered at all.
* `promql_text` - (Optional) use PromQL instead of Lucene in the query. when used the fields [metric.field, metric.source, metric.arithmetic_operator, metric.arithmetic_operator_modifier, filter.text, condition.group_by, condition.group_by_array] must not be set.

** when defining a metric alert, [filter.applications, filter.subsystems, filter.severities] should not be defined

Each `ratio` block should contains the following:

* `text` - (Required) String query 2. empty string is for all records without filtering
* `applications` - (Required) List of application names for query 2, an empty list is all application names.
* `subsystems` - (Required) List of subsystem names for query 2, empty list is all subsystem names.
* `severities` - (Required) List of log severities for query 2, one of the following: `debug`, `verbose`, `info`, `warning`, `error`, `critical`, an empty list is all severities.
* `alias` - (Required) An alias for query 2.
* `group_by` - (Required) A list of fields to group by.

Each `condition` block should contains the following:

* `condition_type` - (Required) Alert condition type, one of the following: [`less_than`, `more_than`, `more_than_usual`, `new_value`] For 'unique_count' alerts, the value should be [`more_than`]
For 'metric', 'ratio', 'relative_time' alerts, the value can be one of [`less_than`, `more_than`].
* `threshold` - (Required) Number of log occurrences that is needed to trigger the alert.
* `timeframe` - (Required) The bounded time frame for the threshold to be occurred within, to trigger the alert one of the following: [`5MIN`, `10MIN`, `20MIN`, `30MIN`, `1H`, `2H`, `3H`, `4H`, `6H`, `12H`, `24H`], for 'new value' alerts [`12H`, `24H`, `48H`, `72H`, `1W`, `1M`, `2M`, `3M`], for 'time relative' alerts [`HOUR`, `DAY`].
* `relative_timeframe` - (Optional) required only for `time relative` alerts one of the following: [`HOUR`, `DAY`, `WEEK`, `MONTH`].
* `unique_count_key` - (Optional) required only for `unique_count` alerts, the key to track.
* `group_by` - (Optional) DEPRECATED please use group_by_array. The field to group by on.
* `group_by_array` - (Optional) An array of fields to group by on, on `new_value` alerts is required with only 1 element and it is the key to track.

Each `schedule` block should contains the following:

* `days` - (Required) Days when alert triggering is allowed, one of the following: `Mo`, `Tu`, `We`, `Th`, `Fr`, `Sa`, `Su`.
* `start` - (Required) Time from which alert triggering is allowed, for example `00:00:00`.
* `end` - (Required) Time till which alert triggering is allowed, for example `23:59:59`.

Each `notifications` block should contains the following:

* `emails` - (Optional) List of email address to notify.
* `integrations` - (Optional) List of integration channels to notify.

## Import

Alerts can be imported using their unique identifier.

First create a new alert block:

```hcl
resource "coralogix_alert" "my_alert" {
}
```
And then import it:

```
$ terraform import coralogix_alert.my_alert UUID
```

After that go to your .tfstate file and implement the data for your alert inside the resource block.

The unique identifer can be retrieved from the API with a GET request,
for more information regarding the API - https://coralogix.com/docs/alerts-api/