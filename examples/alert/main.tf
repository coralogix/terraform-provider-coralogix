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

resource "coralogix_alert" "logs_immediate_alert" {
  name        = "logs immediate alert"
  description = "Example of logs immediate alert from terraform"
  priority    = "P1"

  labels = {
    alert_type        = "security"
    security_severity = "high"
  }

  notification_group = {
    simple_target_settings = [
      {
        integration_id = coralogix_webhook.slack_webhook.external_id
      },
      {
        recipients = ["example@coralogix.com"]
      }
    ]
  }

  incidents_settings = {
    notify_on           = "Triggered Only"
    retriggering_period = {
      minutes = 1
    }
  }

  schedule = {
    active_on = {
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
  }

  type_definition = {
    logs_immediate = {
      logs_filter = {
        lucene_filter = {
          lucene_query = "message:\"error\""
        }
      }
    }
  }
}

resource "coralogix_alert" "logs_more_than_alert" {
  name        = "logs-more-than alert example"
  description = "Example of logs-more-than alert from terraform"
  priority    = "P2"

  notification_group = {
    advanced_target_settings = [
      {
        retriggering_period = {
          minutes = 5
        }
        integration_id = coralogix_webhook.slack_webhook.external_id
      },
      {
        retriggering_period = {
          minutes = 1
        }
        notify_on  = "Triggered and Resolved"
        recipients = ["example@coralogix.com"]
      }
    ]
  }

  incidents_settings = {
    notify_on           = "Triggered and Resolved"
    retriggering_period = {
      minutes = 15
    }
  }

  type_definition = {
    logs_more_than = {
      threshold   = 2
      time_window = {
        specific_value = "10_MINUTES"
      }
      evaluation_window = "Dynamic"
      logs_filter       = {
        lucene_filter = {
          lucene_query  = "message:\"error\""
          label_filters = {
            application_name = [
              {
                operation = "OR"
                value     = "nginx"
              }
            ]
            subsystem_name = [
              {
                operation = "OR"
                value     = "subsystem-name"
              }
            ]
            severities = ["Warning"]
          }
        }
      }
    }
  }
}

resource "coralogix_alert" "logs_less_than_alert" {
  name        = "logs-less-than alert example"
  description = "Example of logs-less-than alert from terraform"
  priority    = "P3"

  incidents_settings = {
    notify_on           = "Triggered and Resolved"
    retriggering_period = {
      minutes = 15
    }
  }

  type_definition = {
    logs_less_than = {
      notification_payload_filter = [
        "coralogix.metadata.sdkId", "coralogix.metadata.sdkName", "coralogix.metadata.sdkVersion"
      ]
      time_window                 = {
        specific_value = "10_MINUTES"
      }
      threshold                    = 2
      undetected_values_management = {
        trigger_undetected_values = true
        auto_retire_timeframe     = "5_Minutes"
      }
    }
  }
}

resource "coralogix_alert" "logs_more_than_usual_alert" {
  name        = "logs-more-than-usual alert example"
  description = "Example of logs-more-than-usual alert from terraform"
  priority    = "P4"

  schedule = {
    active_on = {
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
  }

  type_definition = {
    logs_more_than_usual = {
      time_window = {
        specific_value = "10_MINUTES"
      }
      minimum_threshold = 2
    }
  }
}

resource "coralogix_alert" "logs_ratio_more_than_alert" {
  name        = "logs-ratio-more-than alert example"
  description = "Example of logs-ratio-more-than alert from terraform"
  priority    = "P1"
  group_by    = ["coralogix.metadata.alert_id", "coralogix.metadata.alert_name"]

  type_definition = {
    logs_ratio_more_than = {
      denominator_alias       = "denominator"
      denominator_logs_filter = {
        lucene_filter = {
          lucene_query  = "mod_date:[20020101 TO 20030101]"
          label_filters = {
            application_name = [
              {
                operation = "OR"
                value     = "nginx"
              }
            ]
            subsystem_name = [
              {
                operation = "OR"
                value     = "subsystem-name"
              }
            ]
            severities = ["Warning"]
          }
        }
      }
      numerator_alias       = "numerator"
      numerator_logs_filter = {
        lucene_filter = {
          lucene_query  = "mod_date:[20030101 TO 20040101]"
          label_filters = {
            application_name = [
              {
                operation = "OR"
                value     = "nginx"
              }
            ]
            subsystem_name = [
              {
                operation = "OR"
                value     = "subsystem-name"
              }
            ]
            severities = ["Error"]
          }
        }
      }
      time_window = {
        specific_value = "10_MINUTES"
      }
      threshold    = 2
      group_by_for = "Denominator Only"
    }
  }
}

resource "coralogix_alert" "logs_ratio_less_than_alert" {
  name        = "logs-ratio-less-than alert example"
  description = "Example of logs-ratio-less-than alert from terraform"
  priority    = "P2"

  group_by        = ["coralogix.metadata.alert_id", "coralogix.metadata.alert_name"]
  type_definition = {
    logs_ratio_less_than = {
      numerator_alias   = "numerator"
      denominator_alias = "denominator"
      threshold         = 2
      time_window       = {
        specific_value = "10_MINUTES"
      }
      group_by_for = "Numerator Only"
    }
  }
}

resource "coralogix_alert" "logs_new_value_alert" {
  name        = "logs-new-value alert example"
  description = "Example of logs-new-value alert from terraform"
  priority    = "P3"

  type_definition = {
    logs_new_value = {
      time_window = {
        specific_value = "24_HOURS"
      }
      keypath_to_track = "remote_addr_geoip.country_name"
    }
  }
}

resource "coralogix_alert" "logs_unique_count_alert" {
  name            = "logs-unique-count alert example"
  description     = "Example of logs-unique-count alert from terraform"
  priority        = "P4"
  group_by        = ["remote_addr_geoip.city_name"]
  type_definition = {
    logs_unique_count = {
      unique_count_keypath = "remote_addr_geoip.country_name"
      max_unique_count     = 2
      time_window          = {
        specific_value = "5_MINUTES"
      }
      max_unique_count_per_group_by_key = 500
    }
  }
}

resource "coralogix_alert" "logs_time_relative_more_than_alert" {
  name        = "logs-time-relative-more-than alert example"
  description = "Example of logs-time-relative-more-than alert from terraform"
  priority    = "P1"

  type_definition = {
    logs_time_relative_more_than = {
      logs_filter       = {
        lucene_filter = {
          lucene_query  = "message:\"error\""
        }
      }
      threshold       = 2
      compared_to     = "Same Hour Yesterday"
      ignore_infinity = true
    }
  }
}

resource "coralogix_alert" "logs_time_relative_less_than_alert" {
  name        = "logs-time-relative-less-than alert example"
  description = "Example of logs-time-relative-less-than alert from terraform"
  priority    = "P2"

  type_definition = {
    logs_time_relative_less_than = {
      threshold                   = 1
      compared_to                 = "Yesterday"
      ignore_infinity             = true
      undetected_values_management = {
        trigger_undetected_values = true
        auto_retire_timeframe     = "5_Minutes"
      }
    }
  }
}

resource "coralogix_alert" "metric_more_than_alert" {
  name        = "metric-more-than alert example"
  description = "Example of metric-more-than alert from terraform"
  priority    = "P3"

  type_definition = {
    metric_more_than = {
      metric_filter = {
        promql = "sum(rate(http_requests_total{job=\"api-server\"}[5m])) by (status)"
      }
      threshold    = 2
      for_over_pct = 10
      of_the_last  = {
        specific_value = "10_MINUTES"
      }
      missing_values = {
        min_non_null_values_pct = 50
      }
    }
  }
}

resource "coralogix_alert" "metric_less_than_alert" {
  name        = "metric-less-than alert example"
  description = "Example of metric-less-than alert from terraform"
  priority    = "P4"

  type_definition = {
    metric_less_than = {
      metric_filter = {
        promql = "sum(rate(http_requests_total{job=\"api-server\"}[5m])) by (status)"
      }
      threshold    = 2
      for_over_pct = 10
      of_the_last  = {
        specific_value = "10_MINUTES"
      }
      missing_values = {
        replace_with_zero = true
      }
      undetected_values_management = {
        trigger_undetected_values = true
        auto_retire_timeframe     = "5_Minutes"
      }
    }
  }
}

resource "coralogix_alert" "metric_less_than_usual_alert" {
  name        = "metric_less_than_usual alert example"
  description = "Example of metric_less_than_usual alert from terraform"
  priority    = "P1"

  type_definition = {
    metric_less_than_usual = {
      metric_filter = {
        promql = "sum(rate(http_requests_total{job=\"api-server\"}[5m])) by (status)"
      }
      threshold    = 2
      for_over_pct = 10
      of_the_last  = {
        specific_value = "10_MINUTES"
      }
      minimum_threshold       = 2
      min_non_null_values_pct = 10
    }
  }
}

resource "coralogix_alert" "metric_more_than_usual_alert" {
  name        = "metric_more_than_usual alert example"
  description = "Example of metric_more_than_usual alert from terraform"
  priority    = "P2"

  type_definition = {
    metric_more_than_usual = {
      metric_filter = {
        promql = "sum(rate(http_requests_total{job=\"api-server\"}[5m])) by (status)"
      }
      threshold    = 2
      for_over_pct = 10
      of_the_last  = {
        specific_value = "10_MINUTES"
      }
      minimum_threshold       = 2
      min_non_null_values_pct = 10
    }
  }
}

resource "coralogix_alert" "metric_less_than_or_equals_alert" {
  name        = "metric_less_than_or_equals alert example"
  description = "Example of metric_less_than_or_equals alert from terraform"
  priority    = "P3"

  type_definition = {
    metric_less_than_or_equals = {
      metric_filter = {
        promql = "sum(rate(http_requests_total{job=\"api-server\"}[5m])) by (status)"
      }
      threshold    = 2
      for_over_pct = 10
      of_the_last  = {
        specific_value = "10_MINUTES"
      }
      undetected_values_management = {
        trigger_undetected_values = true
        auto_retire_timeframe     = "5_Minutes"
      }
    }
  }
}

resource "coralogix_alert" "metric_more_than_or_equals_alert" {
  name        = "metric_more_than_or_equals alert example"
  description = "Example of metric_more_than_or_equals alert from terraform"
  priority    = "P4"

  type_definition = {
    metric_less_than_or_equals = {
      metric_filter = {
        promql = "sum(rate(http_requests_total{job=\"api-server\"}[5m])) by (status)"
      }
      threshold    = 2
      for_over_pct = 10
      of_the_last  = {
        specific_value = "10_MINUTES"
      }
    }
  }
}

resource "coralogix_alert" "tracing_immediate_alert" {
  name        = "tracing_immediate alert example"
  description = "Example of tracing_immediate alert from terraform"
  priority    = "P1"

  notification_group = {
    simple_target_settings = [
      {
        retriggering_period = {
          minutes = 1
        }
        notify_on      = "Triggered and Resolved"
        integration_id = coralogix_webhook.slack_webhook.external_id
      }
    ]
  }

  type_definition = {
    tracing_immediate = {
      tracing_query = {
        latency_threshold_ms  = 100
        tracing_label_filters = {
          application_name = [
            {
              operation = "OR"
              values    = ["nginx", "apache"]
            },
            {
                operation = "STARTS_WITH"
                values    = ["application-name:"]
            }
          ]
          subsystem_name = [
            {
              operation = "OR"
              values    = ["subsystem-name"]
            }
          ]
          severities = ["Warning", "Error"]
        }
      }
    }
  }
}

resource "coralogix_alert" "tracing_more_than_alert" {
  name        = "tracing_more_than alert example"
  description = "Example of tracing_more_than alert from terraform"
  priority    = "P2"

  type_definition = {
    tracing_more_than = {
      tracing_query = {
        latency_threshold_ms  = 100
        tracing_label_filters = {
          severities = ["Warning"]
        }
      }
      span_amount = 2
      time_window = {
        specific_value = "10_MINUTES"
      }
    }
  }
}

resource "coralogix_alert" "flow_alert" {
  name        = "flow alert example"
  description = "Example of flow alert from terraform"
  priority    = "P3"

  notification_group = {
    simple_target_settings = [
      {
        retriggering_period = {
          minutes = 1
        }
        notify_on      = "Triggered and Resolved"
        integration_id = coralogix_webhook.slack_webhook.external_id
      }
    ]
  }

  incidents_settings = {
    notify_on           = "Triggered and Resolved"
    retriggering_period = {
      minutes = 1
    }
  }

  type_definition = {
    flow = {
      stages = [
        {
          flow_stages_groups = [
            {
              alert_defs = [
                {
                  id = coralogix_alert.logs_immediate_alert.id
                },
                {
                  id = coralogix_alert.logs_more_than_alert.id
                },
                {
                  id = coralogix_alert.logs_less_than_alert.id
                },
                {
                  id = coralogix_alert.logs_more_than_usual_alert.id
                }
              ]
              next_op   = "AND"
              alerts_op = "OR"
            }
          ]
          timeframe_ms   = 0
          timeframe_type = "Up To"
        }
      ]
    }
  }
}

resource "coralogix_webhook" "slack_webhook" {
  name  = "slack-webhook"
  slack = {
    notify_on = ["flow_anomalies"]
    url       = "https://join.slack.com/example"
  }
}