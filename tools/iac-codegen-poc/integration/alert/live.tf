# Live check of the coralogix_alert switch (integration/live.sh alert). It
# creates these alerts with the current provider; the switched one must plan
# no changes, import coralogix_alert.example the same, and update them. The
# alerts watch applications that do not exist, so they never fire. live.sh
# replaces @SUFFIX@ and @DESCRIPTION@.
terraform {
  required_providers {
    coralogix = {
      source = "coralogix/coralogix"
    }
  }
}

provider "coralogix" {}

# The imported alert: the common properties, a schedule in another time zone,
# a webhook, and a logs threshold with every read rule of the switch.
resource "coralogix_alert" "example" {
  name        = "iac-poc logs threshold @SUFFIX@"
  description = "@DESCRIPTION@"
  priority    = "P3"
  labels      = { owner = "iac-poc" }
  group_by    = ["coralogix.metadata.applicationName"]

  incidents_settings = {
    notify_on           = "Triggered and Resolved"
    retriggering_period = { minutes = 10 }
  }

  notification_group = {
    webhooks_settings = [{
      notify_on           = "Triggered Only"
      recipients          = ["example@coralogix.com"]
      retriggering_period = { minutes = 30 }
    }]
  }

  schedule = {
    active_on = {
      days_of_week = ["Monday", "Friday"]
      start_time   = "23:30"
      end_time     = "06:15"
      utc_offset   = "+0300"
    }
  }

  type_definition = {
    logs_threshold = {
      logs_filter = {
        simple_filter = {
          lucene_query = "message:\"iac-poc\""
          label_filters = {
            application_name = [{ operation = "IS", value = "iac-poc-@SUFFIX@" }]
            severities       = ["Error", "Critical"]
          }
        }
      }
      notification_payload_filter = ["coralogix.metadata.sdkId"]
      custom_evaluation_delay     = 60000
      rules = [{
        condition = {
          threshold      = 10
          time_window    = "10_MINUTES"
          condition_type = "LESS_THAN" # undetected values need a less-than rule
        }
        override = { priority = "P2" }
      }]
      undetected_values_management = {
        trigger_undetected_values = true
        auto_retire_timeframe     = "6_HOURS"
      }
      no_data_policy = {
        state               = "KEEP_LAST"
        auto_retire_seconds = 600
      }
    }
  }
}

# of_the_last as a duration (custom), missing values. The API accepts one
# condition type per metric threshold, so the named window is a second alert.
resource "coralogix_alert" "metric" {
  name        = "iac-poc metric threshold @SUFFIX@"
  description = "@DESCRIPTION@"
  type_definition = {
    metric_threshold = {
      metric_filter = { promql = "sum(rate(iac_poc_requests_total{app=\"@SUFFIX@\"}[5m]))" }
      rules = [{
        condition = {
          threshold      = 2
          for_over_pct   = 10
          of_the_last    = "1h15m"
          condition_type = "MORE_THAN_OR_EQUALS"
        }
        override = { priority = "P1" }
      }]
      missing_values = { replace_with_zero = true }
    }
  }
}

# of_the_last as a window name (custom).
resource "coralogix_alert" "metric_named" {
  name        = "iac-poc metric threshold named @SUFFIX@"
  description = "@DESCRIPTION@"
  type_definition = {
    metric_threshold = {
      metric_filter = { promql = "sum(rate(iac_poc_requests_total{app=\"@SUFFIX@\"}[5m]))" }
      rules = [{
        condition = {
          threshold      = 5
          for_over_pct   = 50
          of_the_last    = "10_MINUTES"
          condition_type = "MORE_THAN"
        }
        override = { priority = "P4" }
      }]
      missing_values = { min_non_null_values_pct = 50 }
    }
  }
}

# percentage_of_deviation (inline, wide) and the anomaly of_the_last.
resource "coralogix_alert" "anomaly" {
  name        = "iac-poc metric anomaly @SUFFIX@"
  description = "@DESCRIPTION@"
  type_definition = {
    metric_anomaly = {
      percentage_of_deviation = 20
      metric_filter           = { promql = "sum(rate(iac_poc_requests_total{app=\"@SUFFIX@\"}[5m]))" }
      rules = [{
        condition = {
          threshold               = 2
          for_over_pct            = 10
          of_the_last             = "10_MINUTES"
          condition_type          = "LESS_THAN"
          min_non_null_values_pct = 50
        }
      }]
    }
  }
}

# latency_threshold_ms (custom) and the tracing filters (unwrap, sets).
resource "coralogix_alert" "tracing" {
  name        = "iac-poc tracing threshold @SUFFIX@"
  description = "@DESCRIPTION@"
  type_definition = {
    tracing_threshold = {
      tracing_filter = {
        latency_threshold_ms = 1500
        tracing_label_filters = {
          application_name = [{ operation = "IS", values = ["iac-poc-@SUFFIX@"] }]
          span_fields = [{
            key         = "status"
            filter_type = { operation = "STARTS_WITH", values = ["50"] }
          }]
        }
      }
      rules = [{
        condition = {
          time_window = "10_MINUTES"
          span_amount = 5
        }
      }]
    }
  }
}

# group_by_for, data sources, and two logs filters.
resource "coralogix_alert" "ratio" {
  name         = "iac-poc logs ratio @SUFFIX@"
  description  = "@DESCRIPTION@"
  group_by     = ["coralogix.metadata.applicationName"]
  data_sources = [{ data_space = "default", data_set = "logs" }]
  type_definition = {
    logs_ratio_threshold = {
      numerator_alias   = "errors"
      denominator_alias = "all"
      numerator = {
        simple_filter = { lucene_query = "level:error AND app:iac-poc-@SUFFIX@" }
      }
      denominator = {
        simple_filter = { lucene_query = "app:iac-poc-@SUFFIX@" }
      }
      group_by_for = "Denominator Only"
      rules = [{
        condition = {
          threshold      = 2
          time_window    = "10_MINUTES"
          condition_type = "LESS_THAN"
        }
        override = { priority = "P2" }
      }]
    }
  }
}

# A logs immediate alert for the flow.
resource "coralogix_alert" "immediate" {
  name        = "iac-poc logs immediate @SUFFIX@"
  description = "@DESCRIPTION@"
  type_definition = {
    logs_immediate = {
      logs_filter = {
        simple_filter = { lucene_query = "app:iac-poc-@SUFFIX@" }
      }
    }
  }
}

# The flow (unwrap of the stage groups, int64 timeframe_ms).
resource "coralogix_alert" "flow" {
  name        = "iac-poc flow @SUFFIX@"
  description = "@DESCRIPTION@"
  type_definition = {
    flow = {
      stages = [{
        flow_stages_groups = [{
          alert_defs = [{ id = coralogix_alert.immediate.id }]
          next_op    = "AND"
          alerts_op  = "OR"
        }]
        timeframe_ms   = 60000
        timeframe_type = "Up To"
      }]
    }
  }
}
