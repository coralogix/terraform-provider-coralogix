terraform {
  required_providers {
    coralogix = {
      version = "~> 1.8"
      source  = "coralogix/coralogix"
    }
  }
}

provider "coralogix" {
  #api_key = "<add your api key here or add env variable CORALOGIX_API_KEY>"
  #env = "<add the environment you want to work at or add env variable CORALOGIX_ENV>"
}

resource "coralogix_alert" "immediate_alert" {
  name        = "logs immediate alert"
  description = "Example of logs immediate alert from terraform"
  priority    = "P2"
  #  group_by   = ["coralogix.metadata.alert_id", "coralogix.metadata.alert_name"]
  labels      = {
    alert_type        = "security"
    security_severity = "high"
  }

  #  notification_group = {
  #    group_by_keys = ["coralogix.metadata.alert_id", "coralogix.metadata.alert_name"]
  #  }

  incidents_settings = {
    notify_on           = "Triggered and Resolved"
    retriggering_period = {
      minutes = 10
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
        simple_filter = {
          lucene_query = "message:\"error\""
        }
      }
    }
  }
}

ressource "coralogix_alert" "logs_threshold"{

}

resource "coralogix_alert" "ratio_alert" {
  name        = "logs-ratio-more-than alert example"
  description = "Example of logs-ratio-more-than alert from terraform"
  priority    = "P1"
  group_by    = ["coralogix.metadata.alert_id", "coralogix.metadata.alert_name"]

  notification_group = {
    group_by_keys = ["coralogix.metadata.alert_id", "coralogix.metadata.alert_name"]
  }

  type_definition = {
    logs_ratio_threshold = {
      denominator_alias = "denominator"
      denominator       = {
        simple_filter = {
          lucene_query  = "mod_date:[20020101 TO 20030101]"
          label_filters = {
            application_name = [
              {
                operation = "IS"
                value     = "nginx"
              }
            ]
            subsystem_name = [
              {
                operation = "IS"
                value     = "subsystem-name"
              }
            ]
            severities = ["Warning"]
          }
        }
      }
      numerator_alias = "numerator"
      numerator       = {
        simple_filter = {
          lucene_query  = "mod_date:[20030101 TO 20040101]"
          label_filters = {
            application_name = [
              {
                operation = "IS"
                value     = "nginx"
              }
            ]
            subsystem_name = [
              {
                operation = "IS"
                value     = "subsystem-name"
              }
            ]
            severities = ["Error"]
          }
        }
      }
      rules = [
        {
          condition = {
            threshold   = 2
            time_window = {
              specific_value = "10_MINUTES"
            }
            condition_type = "MORE_THAN"
          }
          override = {
            priority = "P2"
          }
        }
      ]
    }
  }
}

resource "coralogix_alert" "new_value_alert" {
  name        = "logs-new-value alert example"
  description = "Example of logs-new-value alert from terraform"
  priority    = "P2"

  notification_group = {
    webhooks_settings = [
      {
        retriggering_period = {
          minutes = 10
        }
        notify_on      = "Triggered and Resolved"
        integration_id = coralogix_webhook.slack_webhook.external_id
      },
      {
        retriggering_period = {
          minutes = 10
        }
        notify_on      = "Triggered and Resolved"
        integration_id = coralogix_webhook.slack_webhook_2.external_id
      }
    ]
  }

  type_definition = {
    logs_new_value = {
      notification_payload_filter = [
        "coralogix.metadata.sdkId", "coralogix.metadata.sdkName", "coralogix.metadata.sdkVersion"
      ]
      rules                       = [
        {
          condition = {
            time_window = {
              specific_value = "12_HOURS"
            }
            keypath_to_track = "remote_addr_geoip.country_name"
          }
        }
      ]
    }
  }
}

resource "coralogix_alert" "time_relative_alert" {
  name        = "logs-time-relative-more-than alert example"
  description = "Example of logs-time-relative-more-than alert from terraform"
  priority    = "P4"

  type_definition = {
    logs_time_relative_threshold = {
      rules = [
        {
          condition = {
            time_window = {
              specific_value = "10_MINUTES"
            }
            compared_to    = "Same Hour Yesterday"
            threshold      = 10
            condition_type = "MORE_THAN"
          }
          override = {
            priority = "P4"
          }
        }
      ]
    }
  }
}

#resource "coralogix_alert" "metric_lucene_alert" {
#  name        = "logs-less-than alert example"
#  description = "Example of logs-less-than alert example from terraform"
#  priority    = "P2"
#
#  labels = {
#    alert_type        = "security"
#    security_severity = "high"
#  }
#
#  notification_group = {
#    simple_target_settings = [
#      {
#        recipients = ["example@coralogix.com", "example2@coralogix.com"]
#      },
#    ]
#  }
#
#  incidents_settings = {
#    notify_on = "Triggered and Resolved"
#    retriggering_period = {
#      minutes = 1
#    }
#  }
#
#  schedule = {
#    active_on = {
#      days_of_week = ["Wednesday", "Thursday"]
#      start_time = {
#        hours   = 10
#        minutes = 30
#      }
#      end_time = {
#        hours   = 20
#        minutes = 30
#      }
#    }
#  }
#
#  type_definition = {
#    logs_threshold = {
#      rules = [{
#        threshold   = 2
#        time_window = "10_MINUTES"
#        condition   = "LESS_THAN"
#      }]
#      logs_filter = {
#        simple_filter = {
#          lucene_query = "message:\"error\""
#          label_filters = {
#            application_name = [
#              {
#                operation = "NOT"
#                value     = "application_name"
#              }
#            ]
#            subsystem_name = [
#              {
#                operation = "STARTS_WITH"
#                value     = "subsystem-name"
#              }
#            ]
#            severities = ["Warning", "Error"]
#          }
#        }
#      }
#    }
#  }
#}

resource "coralogix_alert" "metric_promql_alert" {
  name        = "metric-more-than alert example"
  description = "Example of metric-more-than alert from terraform"
  priority    = "P3"

  type_definition = {
    metric_threshold = {
      metric_filter = {
        promql = "sum(rate(http_requests_total{job=\"api-server\"}[5m])) by (status)"
      }
      rules = [
        {
          condition = {
            threshold    = 2
            for_over_pct = 10
            of_the_last  = {
              specific_value = "10_MINUTES"
            }
            missing_values = {
              min_non_null_values_pct = 50
            }
            condition_type = "MORE_THAN"
          }
          override = {
            priority = "P3"
          }
        }
      ]
      missing_values = {
        min_non_null_values_pct = 50
      }
    }
  }
}

resource "coralogix_alert" "unique_count_alert" {
  name        = "logs-unique-count alert example"
  description = "Example of logs-unique-count alert from terraform"
  priority    = "P2"

  group_by        = ["remote_addr_geoip.city_name"]
  type_definition = {
    logs_unique_count = {
      rules = [
        {
          condition = {
            unique_count_keypath = "remote_addr_geoip.country_name"
            max_unique_count     = 2
            time_window          = {
              specific_value = "20_MINUTES"
            }
            max_unique_count_per_group_by_key = 500
          }
        }
      ]
      unique_count_keypath              = "remote_addr_geoip.country_name"
      max_unique_count_per_group_by_key = 500
    }
  }
}

resource "coralogix_alert" "tracing_alert" {
  name        = "tracing_more_than alert example"
  description = "Example of tracing_more_than alert from terraform"
  priority    = "P2"

  type_definition = {
    tracing_threshold = {
      tracing_filter = {
        latency_threshold_ms  = 100
        tracing_label_filters = {
          application_name = [
            {
              operation = "IS"
              values    = ["nginx", "apache"]
            },
            {
              operation = "STARTS_WITH"
              values    = ["application-name:"]
            }
          ]
        }
      }
      rules = [
        {
          condition = {
            span_amount = 5
            time_window = {
              specific_value = "10_MINUTES"
            }
          }
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

resource "coralogix_webhook" "slack_webhook_2" {
  name  = "slack-webhook-2"
  slack = {
    notify_on = ["flow_anomalies"]
    url       = "https://join.slack.com/example"
  }
}



