terraform {
  required_providers {
    coralogix = {
      version = "~> 1.11"
      source  = "coralogix/coralogix"
    }
  }
}

provider "coralogix" {
  #api_key = "<add your api key here or add env variable CORALOGIX_API_KEY>"
  #env = "<add the environment you want to work at or add env variable CORALOGIX_ENV>"
}

resource "coralogix_alert" "standard_alert" {
  name           = "Standard alert example"
  description    = "Example of standard alert from terraform"
  alert_priority = "P3"

  labels = {
    alert_type        = "security"
    security_severity = "high"
  }

  notification_group = {
    notifications   = [
      {
        integration_id = coralogix_webhook.slack_webhook.external_id
      },
      {
        retriggering_period = {
          minutes = 1
        }
        notify_on = "Triggered and Resolved"
        recipients = ["example@coralogix.com"]
      }
    ]
  }

  incidents_settings = {
    notify_on                   = "Triggered and Resolved"
    retriggering_period = {
      minutes = 1
    }
  }

  alert_schedule = {
    days_of_week = ["Wednesday", "Thursday"]
    start_time   = {
      hours   = 8
      minutes = 30
    }
    end_time = {
      hours   = 20
      minutes = 30
    }
  }

  alert_type_definition = {
    logs_immediate = {
      logs_filter = {
        lucene_filter = {
          lucene_query = "message:\"error\""
          label_filters = {
          }
        }
      }
    }
  }
}
#
#data "coralogix_alert" "imported_standard_alert" {
#  id = coralogix_alert.standard_alert.id
#}
#
#resource "coralogix_alert" "ratio_alert" {
#  name        = "Ratio alert example"
#  description = "Example of ratio alert from terraform"
#  severity    = "Critical"
#
#  notifications_group {
#    notification {
#      integration_id              = coralogix_webhook.slack_webhook.external_id
#      retriggering_period_minutes = 1
#      notify_on                   = "Triggered_only"
#    }
#    notification {
#      email_recipients            = ["example@coralogix.com"]
#      retriggering_period_minutes = 1
#      notify_on                   = "Triggered_and_resolved"
#    }
#  }
#
#  scheduling {
#    time_zone = "UTC+2"
#    time_frame {
#      days_enabled = ["Wednesday", "Thursday"]
#      start_time   = "08:30"
#      end_time     = "20:30"
#    }
#  }
#
#  ratio {
#    query_1 {
#
#    }
#    query_2 {
#      applications = ["nginx"] //change here for existing applications from your account
#      subsystems   = ["subsystem-name"] //change here for existing subsystems from your account
#      severities   = ["Warning"]
#    }
#    condition {
#      less_than       = true
#      ratio_threshold = 2
#      time_window     = "10Min"
#      group_by        = ["coralogix.metadata.sdkId"]
#      group_by_q1     = true
#      manage_undetected_values {
#        enable_triggering_on_undetected_values = true
#        auto_retire_ratio                      = "5Min"
#      }
#    }
#  }
#}
#
#resource "coralogix_alert" "new_value_alert" {
#  name        = "New value alert example"
#  description = "Example of new value alert from terraform"
#  severity    = "Info"
#
#  notifications_group {
#    notification {
#      integration_id              = coralogix_webhook.slack_webhook.external_id
#      retriggering_period_minutes = 1
#      notify_on                   = "Triggered_only"
#    }
#    notification {
#      email_recipients            = ["example@coralogix.com"]
#      retriggering_period_minutes = 1
#      notify_on                   = "Triggered_and_resolved"
#    }
#  }
#
#  scheduling {
#    time_zone = "UTC+2"
#    time_frame {
#      days_enabled = ["Wednesday", "Thursday"]
#      start_time   = "08:30"
#      end_time     = "20:30"
#    }
#  }
#
#  new_value {
#    severities = ["Info"]
#    condition {
#      key_to_track = "remote_addr_geoip.country_name"
#      time_window  = "12H"
#    }
#  }
#}
#
#resource "coralogix_alert" "time_relative_alert" {
#  name        = "Time relative alert example"
#  description = "Example of time relative alert from terraform"
#  severity    = "Critical"
#
#  notifications_group {
#    notification {
#      integration_id              = coralogix_webhook.slack_webhook.external_id
#    }
#    notification {
#      email_recipients            = ["example@coralogix.com"]
#    }
#  }
#
#  incident_settings {
#    notify_on = "Triggered_and_resolved"
#    retriggering_period_minutes = 1
#  }
#
#  scheduling {
#    time_zone = "UTC+2"
#    time_frame {
#      days_enabled = ["Wednesday", "Thursday"]
#      start_time   = "08:30"
#      end_time     = "20:30"
#    }
#  }
#
#  time_relative {
#    severities = ["Error"]
#    condition {
#      more_than            = true
#      ratio_threshold      = 2
#      relative_time_window = "Same_hour_last_week"
#    }
#  }
#}
#
#resource "coralogix_alert" "metric_lucene_alert" {
#  name        = "Metric lucene alert example"
#  description = "Example of metric lucene alert from terraform"
#  severity    = "Critical"
#
#  notifications_group {
#    notification {
#      integration_id              = coralogix_webhook.slack_webhook.external_id
#    }
#    notification {
#      email_recipients            = ["example@coralogix.com"]
#    }
#  }
#
#  incident_settings {
#    notify_on = "Triggered_and_resolved"
#    retriggering_period_minutes = 60
#  }
#
#  scheduling {
#    time_zone = "UTC+2"
#    time_frame {
#      days_enabled = ["Wednesday", "Thursday"]
#      start_time   = "08:30"
#      end_time     = "20:30"
#    }
#  }
#
#  metric {
#    lucene {
#      search_query = "name:\"Frontend transactions\""
#      condition {
#        metric_field                 = "subsystem"
#        arithmetic_operator          = "Percentile"
#        arithmetic_operator_modifier = 20
#        less_than                    = true
#        group_by                     = ["coralogix.metadata.sdkId"]
#        threshold                    = 60
#        sample_threshold_percentage  = 50
#        time_window                  = "30Min"
#        manage_undetected_values {
#          enable_triggering_on_undetected_values = false
#        }
#      }
#    }
#  }
#}
#
#resource "coralogix_alert" "metric_promql_alert" {
#  name        = "Metric promql alert example"
#  description = "Example of metric promql alert from terraform"
#  severity    = "Critical"
#
#  notifications_group {
#    notification {
#      notify_on                   = "Triggered_and_resolved"
#      integration_id              = coralogix_webhook.slack_webhook.external_id
#      retriggering_period_minutes = 1
#    }
#    notification {
#      notify_on                   = "Triggered_and_resolved"
#      email_recipients            = ["example@coralogix.com"]
#      retriggering_period_minutes = 24*60
#    }
#  }
#
#  scheduling {
#    time_zone = "UTC-8"
#    time_frame {
#      days_enabled = ["Wednesday", "Thursday"]
#      start_time   = "08:30"
#      end_time     = "20:30"
#    }
#  }
#
#  metric {
#    promql {
#      search_query = "http_requests_total{status!~\"4..\"}"
#      condition {
#        less_than_usual                 = true
#        threshold                       = 3
#        sample_threshold_percentage     = 50
#        time_window                     = "12H"
#        replace_missing_value_with_zero = true
#      }
#    }
#  }
#}
#
#resource "coralogix_alert" "unique_count_alert" {
#  name        = "Unique count alert example"
#  description = "Example of unique count alert from terraform"
#  severity    = "Info"
#
#  notifications_group {
#    group_by_fields = ["coralogix.metadata.sdkId"]
#    notification {
#      integration_id              = coralogix_webhook.slack_webhook.external_id
#      retriggering_period_minutes = 1
#      notify_on                   = "Triggered_and_resolved"
#    }
#    notification {
#      email_recipients            = ["example@coralogix.com"]
#      retriggering_period_minutes = 1
#      notify_on                   = "Triggered_and_resolved"
#    }
#  }
#
#  scheduling {
#    time_zone = "UTC+2"
#    time_frame {
#      days_enabled = ["Wednesday", "Thursday"]
#      start_time   = "08:30"
#      end_time     = "20:30"
#    }
#  }
#
#  unique_count {
#    severities = ["Info"]
#    condition {
#      unique_count_key               = "remote_addr_geoip.country_name"
#      max_unique_values              = 2
#      time_window                    = "10Min"
#      group_by_key                   = "coralogix.metadata.sdkId"
#      max_unique_values_for_group_by = 500
#    }
#  }
#}
#
#resource "coralogix_alert" "tracing_alert" {
#  name        = "Tracing alert example"
#  description = "Example of tracing alert from terraform"
#  severity    = "Info"
#
#  notifications_group {
#    notification {
#      notify_on                   = "Triggered_and_resolved"
#      email_recipients            = ["user@example.com"]
#      retriggering_period_minutes = 1
#    }
#    notification {
#      notify_on                   = "Triggered_and_resolved"
#      integration_id              = coralogix_webhook.slack_webhook.external_id
#      retriggering_period_minutes = 1
#    }
#  }
#
#  scheduling {
#    time_zone = "UTC+2"
#    time_frame {
#      days_enabled = ["Wednesday", "Thursday"]
#      start_time   = "08:30"
#      end_time     = "20:30"
#    }
#  }
#
#  tracing {
#    latency_threshold_milliseconds = 20.5
#    applications                   = [
#      "application_name", "filter:contains:application-name2", "filter:endsWith:application-name3",
#      "filter:startsWith:application-name4"
#    ]
#    subsystems = [
#      "subsystemName", "filter:notEquals:subsystemName2", "filter:contains:subsystemName",
#      "filter:endsWith:subsystemName",
#      "filter:startsWith:subsystemName"
#    ]
#    services = [
#      "serviceName", "filter:contains:serviceName", "filter:endsWith:serviceName", "filter:startsWith:serviceName"
#    ]
#    tag_filter {
#      field  = "status"
#      values = ["filter:contains:400", "500"]
#    }
#    tag_filter {
#      field  = "key"
#      values = ["value"]
#    }
#    condition {
#      more_than   = true
#      time_window = "5Min"
#      threshold   = 2
#    }
#  }
#}
#
#resource "coralogix_alert" "flow_alert" {
#  name        = "Flow alert example"
#  description = "Example of flow alert from terraform"
#  severity    = "Info"
#
#  notifications_group {
#    notification {
#      notify_on                   = "Triggered_and_resolved"
#      email_recipients            = ["user@example.com"]
#      retriggering_period_minutes = 1
#    }
#    notification {
#      notify_on                   = "Triggered_and_resolved"
#      integration_id              = coralogix_webhook.slack_webhook.external_id
#      retriggering_period_minutes = 1
#    }
#  }
#
#  scheduling {
#    time_zone = "UTC+2"
#    time_frame {
#      days_enabled = ["Wednesday", "Thursday"]
#      start_time   = "08:30"
#      end_time     = "20:30"
#    }
#  }
#
#  flow {
#    stage {
#      group {
#        sub_alerts {
#          operator = "OR"
#          flow_alert {
#            user_alert_id = coralogix_alert.standard_alert.id
#          }
#        }
#        next_operator = "OR"
#      }
#      group {
#        sub_alerts {
#          operator = "AND"
#          flow_alert {
#            not           = true
#            user_alert_id = coralogix_alert.unique_count_alert.id
#          }
#        }
#        next_operator = "AND"
#      }
#      time_window {
#        minutes = 20
#      }
#    }
#    stage {
#      group {
#        sub_alerts {
#          operator = "AND"
#          flow_alert {
#            user_alert_id = coralogix_alert.standard_alert.id
#          }
#          flow_alert {
#            not           = true
#            user_alert_id = coralogix_alert.unique_count_alert.id
#          }
#        }
#        next_operator = "OR"
#      }
#    }
#    group_by = ["coralogix.metadata.sdkId"]
#  }
#}
#
resource "coralogix_webhook" "slack_webhook" {
  name  = "slack-webhook"
  slack = {
    notify_on = ["flow_anomalies"]
    url       = "https://join.slack.com/example"
  }
}
