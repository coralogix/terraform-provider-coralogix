terraform {
  required_providers {
    coralogix = {
      version = "1.18.13"
      source  = "coralogix/coralogix"
    }
  }
}

provider "coralogix" {
  #api_key = "<add your api key here or add env variable CORALOGIX_API_KEY>"
  #env = "<add the environment you want to work at or add env variable CORALOGIX_ENV>"
}

resource "coralogix_alert" "logs_immediate" {
  name        = "logs immediate alert"
  description = "Example of logs immediate alert from terraform"
  priority    = "P1"

  incidents_settings = {
    notify_on           = "Triggered and Resolved"
    retriggering_period = {
      minutes = 10
    }
  }
  notification_group = {
    webhooks_settings = [
      {
        recipients = ["example@coralogix.com"]
      },
      {
        integration_id      = coralogix_webhook.slack_webhook.external_id
        retriggering_period = {
          minutes = 15
        }
        notify_on = "Triggered Only"
      }
    ]
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

resource "coralogix_alert" "logs_threshold" {
  name            = "logs-threshold alert example"
  description     = "Example of logs-threshold alert from terraform"
  priority        = "P2"
  type_definition = {
    logs_threshold = {
      rules = [
        {
          condition = {
            threshold   = 2
            time_window = {
              specific_value = "10_MINUTES"
            }
            condition_type = "LESS_THAN"
          }
          override = {
          }
        }
      ]
      logs_filter = {
        simple_filter = {
          lucene_query  = "message:\"error\""
          label_filters = {
            application_name = [
              {
                operation = "NOT"
                value     = "application_name"
              }
            ]
            subsystem_name = [
              {
                operation = "STARTS_WITH"
                value     = "subsystem-name"
              }
            ]
            severities = ["Warning", "Error"]
          }
        }
      }
    }
  }
}

resource "coralogix_alert" "logs_anomaly" {
  name            = "logs-anomaly alert example"
  description     = "Example of logs-anomaly alert from terraform"
  priority        = "P3"
  type_definition = {
    logs_anomaly = {
      simple_filter = {
        label_filters = {
          application_name = [
            {
              value     = "nginx"
              operation = "IS"
            }
          ]
        }
      }
      rules = [
        {
          condition = {
            time_window = {
              specific_value = "10_MINUTES"
            }
            minimum_threshold = 2
          }
        }
      ]
    }
  }
}

resource "coralogix_alert" "logs_ratio_threshold" {
  name        = "logs-ratio-more-than alert example"
  description = "Example of logs-ratio-more-than alert from terraform"
  priority    = "P4"
  group_by    = ["coralogix.metadata.alert_id", "coralogix.metadata.alert_name", "coralogix.metadata.alert_description"]

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

resource "coralogix_alert" "logs_new_value" {
  name        = "logs-new-value alert example"
  description = "Example of logs-new-value alert from terraform"
  priority    = "P5"

  notification_group = {
    webhooks_settings = [
      {
        retriggering_period = {
          minutes = 10
        }
        notify_on  = "Triggered and Resolved"
        recipients = ["example@coralogix.com", "example2@coralogix.com"]
      },
      {
        retriggering_period = {
          minutes = 10
        }
        notify_on      = "Triggered and Resolved"
        integration_id = coralogix_webhook.slack_webhook.external_id
      }
    ]
  }

  type_definition = {
    logs_new_value = {
      notification_payload_filter = [
        "coralogix.metadata.sdkId", "coralogix.metadata.sdkName", "coralogix.metadata.sdkVersion"
      ]
      rules = [
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

resource "coralogix_alert" "logs_unique_count" {
  name        = "logs-unique-count alert example"
  description = "Example of logs-unique-count alert from terraform"
  priority    = "P4"

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

resource "coralogix_alert" "logs_time_relative_threshold" {
  name        = "logs-time-relative-more-than alert example"
  description = "Example of logs-time-relative-more-than alert from terraform"
  priority    = "P1"

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

resource "coralogix_alert" "metric_threshold" {
  name        = "metric-more-than alert example"
  description = "Example of metric-more-than alert from terraform"
  priority    = "P2"
  group_by    = ["status", "method"]

  type_definition = {
    metric_threshold = {
      metric_filter = {
        promql = "sum(rate(http_requests_total{job=\"api-server\"}[5m])) by (status, method)"
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

resource "coralogix_alert" "metric_anomaly" {
  name        = "metric-anomaly alert example"
  description = "Example of metric-anomaly alert from terraform"
  priority    = "P3"

  type_definition = {
    metric_anomaly = {
      metric_filter = {
        promql = "sum(rate(http_requests_total{job=\"apwi-server\"}[5m])) by (status, method)"
      }
      rules = [
        {
          condition = {
            for_over_pct            = 10
            min_non_null_values_pct = 50
            of_the_last             = {
              specific_value = "10_MINUTES"
            }
            threshold      = 2.4
            condition_type = "LESS_THAN"
          }
        }
      ]
    }
  }
}

resource "coralogix_alert" "tracing_threshold" {
  name        = "tracing_more_than alert example"
  description = "Example of tracing_more_than alert from terraform"
  priority    = "P5"

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

resource "coralogix_alert" "tracing_immediate" {
  name        = "tracing_immediate alert example"
  description = "Example of tracing_immediate alert from terraform"
  priority    = "P1"

  type_definition = {
    tracing_immediate = {
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
    }
  }
}

resource "coralogix_alert" "flow" {
    name        = "flow alert example"
    description = "Example of flow alert from terraform"
    priority    = "P2"

    type_definition = {
      flow = {
        stages = [
          {
            flow_stages_groups = [
              {
                alert_defs = [
                  {
                    id = resource.coralogix_alert.logs_immediate.id
                  },
                  {
                    id = resource.coralogix_alert.logs_threshold.id
                  }
                ]
                alerts_op = "AND"
                next_op  = "OR"
              }
            ]
            timeframe_type = "Up To"
            timeframe_ms       = 60000
          },
          {
            flow_stages_groups = [
              {
                alert_defs = [
                  {
                    id = resource.coralogix_alert.logs_anomaly.id
                  },
                  {
                    id = resource.coralogix_alert.logs_ratio_threshold.id
                  }
                ]
                alerts_op = "OR"
                next_op  = "AND"
              }
            ]
            timeframe_type = "Up To"
            timeframe_ms       = 60000
          }
        ]
      }
    }
}

resource "coralogix_webhook" "slack_webhook" {
  name = "slack-webhook"
  slack = {
    notify_on = ["flow_anomalies"]
    url = "https://join.slack.com/example"
  }
}