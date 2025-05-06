terraform {
  required_providers {
    coralogix = {
      version = "~> 2.0"
      source  = "coralogix/coralogix"
    }
  }
}

provider "coralogix" {
  #api_key = "<add your api key here or add env variable CORALOGIX_API_KEY>"
  #env = "<add the environment you want to work at or add env variable CORALOGIX_ENV>"
}

resource "coralogix_alert" "test" {
  name        = "logs_immediate alert"
  description = "Example of logs_immediate alert from terraform"
  priority    = "P2"

  labels = {
    alert_type        = "security"
    security_severity = "high"
  }

  incidents_settings = {
    notify_on = "Triggered and Resolved"
    retriggering_period = {
      minutes = 10
    }
  }

  schedule = {
    active_on = {
      days_of_week = ["Wednesday", "Thursday"]
      start_time = "08:30"
      end_time = "20:30"
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

resource "coralogix_connector" "slack_example" {
  id               = "slack_example"
  type             = "slack"
  name             = "slack connector"
  description      = "slack connector example"
  connector_config = {
    fields = [
      {
        field_name = "integrationId"
        value      = "iac-internal"
      },
      {
        field_name = "fallbackChannel"
        value      = "iac-internal"
      },
      {
        field_name = "channel"
        value      = "iac-internal"
      }
    ]
  }
}

resource "coralogix_preset" "slack_example" {
  id               = "slack_example"
  name             = "slack example"
  description      = "slack preset example"
  entity_type      = "alerts"
  connector_type   = "slack"
  parent_id        = "preset_system_slack_alerts_basic"
  config_overrides = [
    {
      condition_type = {
        match_entity_type_and_sub_type = {
          entity_sub_type    = "logsImmediateResolved"
        }
      }
      message_config =    {
        fields = [
          {
            field_name = "title"
            template   = "{{alert.status}} {{alertDef.priority}} - {{alertDef.name}}"
          },
          {
            field_name = "description"
            template   = "{{alertDef.description}}"
          }
        ]
      }
    }
  ]
}

resource "coralogix_alert" "test" {
  name        = "logs_anomaly alert example"
  description = "Example of logs_anomaly alert from terraform"
  priority    = "P4"

  labels = {
    alert_type        = "security"
    security_severity = "high"
  }

  notification_group = {
    webhooks_settings = [{
      retriggering_period = {
        minutes = 1
      }
      notify_on  = "Triggered and Resolved"
      recipients = ["example@coralogix.com"]
    }]
  }

  incidents_settings = {
    notify_on           = "Triggered and Resolved"
    retriggering_period = {
      minutes = 1
    }
  }

  schedule = {
    active_on = {
      days_of_week = ["Wednesday", "Thursday"]
      start_time = "08:30"
      end_time = "20:30"
    }
  }

  type_definition = {
    logs_anomaly = {
      rules = [{
        condition = {
          minimum_threshold   = 2
          time_window = "10_MINUTES"
        }
        override = {
          priority = "P2"
        }
      }]
      logs_filter = {
        simple_filter = {
          lucene_query  = "message:\"error\""
          label_filters = {
            application_name = [{
              operation = "IS"
              value     = "nginx"
            }]
            subsystem_name = [{
              operation = "IS"
              value     = "subsystem-name"
            }]
            severities = ["Warning"]
          }
        }
      }
      notification_payload_filter = [
        "coralogix.metadata.sdkId", "coralogix.metadata.sdkName", "coralogix.metadata.sdkVersion"
      ]
    }
  }
}

resource "coralogix_alert" "test_with_destination" {
  name        = "logs_threshold alert example"
  description = "Example of logs_threshold alert example from terraform"
  priority    = "P2"

  labels = {
    alert_type        = "security"
    security_severity = "high"
  }

  notification_group = {
    webhooks_settings = [{
      recipients = ["example@coralogix.com", "example2@coralogix.com"]
    }]
    destinations = [{
      connector_id = coralogix_connector.slack_example.id
      preset_id    = coralogix_preset.slack_example.id
      notify_on = "Triggered and Resolved"
      triggered_routing_overrides = {
        connector_overrides = [
          {
            field_name = "channel",
            template = "{{alertDef.priority}}"
          }
        ]
        output_schema_id = "slack_raw"
      }
    }]
  }

  incidents_settings = {
    notify_on           = "Triggered and Resolved"
    retriggering_period = {
        minutes = 1
    }
  }

  schedule = {
    active_on = {
        days_of_week = ["Wednesday", "Thursday"]
        start_time = "10:30"
        end_time = "20:30"
    }
  }

  type_definition = {
    logs_threshold = {
      rules = [{
        condition = {
          threshold   = 2
          time_window = "10_MINUTES"
          condition_type   = "LESS_THAN"
        }
        override = {
          priority = "P2"
        }
      }]
      logs_filter       = {
        simple_filter = {
          lucene_query  = "message:\"error\""
          label_filters = {
            application_name = [{
              operation = "NOT"
              value     = "application_name"
            }]
            subsystem_name = [{
              operation = "STARTS_WITH"
              value     = "subsystem-name"
            }]
            severities = ["Warning", "Error"]
          }
        }
      }
    }
  }
}

resource "coralogix_global_router" "example" {
  name        = "global router example"
  description = "global router example"
  entity_type = "alerts"
  rules       = [
    {
      name = "rule-name"
      condition = "alertDef.priority == \"P1\""
      targets = [
        {
          connector_id   = coralogix_connector.slack_example.id
          preset_id      = coralogix_preset.slack_example.id
        }
      ]
    }
  ]
}

resource "coralogix_alert" "test_with_router" {
  depends_on = [coralogix_global_router.example]
  name        = "logs_threshold alert example"
  description = "Example of logs_threshold alert example from terraform"
  priority    = "P2"

  labels = {
    alert_type        = "security"
    security_severity = "high"
  }

  notification_group = {
    webhooks_settings = [{
      recipients = ["example@coralogix.com", "example2@coralogix.com"]
    }]
    router = {}
  }

  incidents_settings = {
    notify_on           = "Triggered and Resolved"
    retriggering_period = {
        minutes = 1
    }
  }

  schedule = {
    active_on = {
        days_of_week = ["Wednesday", "Thursday"]
        start_time = "10:30"
        end_time = "20:30"
    }
  }

  type_definition = {
    logs_threshold = {
      rules = [{
        condition = {
          threshold   = 2
          time_window = "10_MINUTES"
          condition_type   = "LESS_THAN"
        }
        override = {
          priority = "P2"
        }
      }]
      logs_filter       = {
        simple_filter = {
          lucene_query  = "message:\"error\""
          label_filters = {
            application_name = [{
              operation = "NOT"
              value     = "application_name"
            }]
            subsystem_name = [{
              operation = "STARTS_WITH"
              value     = "subsystem-name"
            }]
            severities = ["Warning", "Error"]
          }
        }
      }
    }
  }
}



resource "coralogix_alert" "test" {
  name        = "logs_ratio_threshold alert example"
  description = "Example of logs_ratio_threshold alert from terraform"
  priority    = "P3"

  group_by        = ["coralogix.metadata.alert_id", "coralogix.metadata.alert_name"]
  type_definition = {
    logs_ratio_threshold = {
      numerator_alias   = "numerator"
      denominator_alias = "denominator"
      rules = [{
          condition = {
              threshold         = 2
              time_window       = "10_MINUTES"
              condition_type		 = "LESS_THAN"
          }
          override = {
              priority = "P2"
          }
      }]
      group_by_for = "Denominator Only"
    }
  }
}

resource "coralogix_alert" "test" {
  name        = "logs_new_value alert example"
  description = "Example of logs_new_value alert from terraform"
  priority    = "P2"

  type_definition = {
    logs_new_value = {
      notification_payload_filter = ["coralogix.metadata.sdkId", "coralogix.metadata.sdkName", "coralogix.metadata.sdkVersion"]
      rules = [{
        condition = {
            time_window = "24_HOURS"
            keypath_to_track = "remote_addr_geoip.country_name"
        }
        override = {
            priority = "P2"
        }
      }]
    }
  }
}

resource "coralogix_alert" "test" {
  name        = "logs_unique_count alert example"
  description = "Example of logs_unique_count alert from terraform"
  priority    = "P2"

  group_by        = ["remote_addr_geoip.city_name"]
  type_definition = {
    logs_unique_count = {
        unique_count_keypath = "remote_addr_geoip.country_name"
        max_unique_count_per_group_by_key = 500
          rules = [ {
            condition = {
                max_unique_count     = 2
                time_window          = "5_MINUTES"
            }
        }]
    }
  }
}

resource "coralogix_alert" "test" {
  name        = "logs_time_relative_threshold alert example"
  description = "Example of logs_time_relative_threshold alert from terraform"
  priority    = "P3"

  type_definition = {
    logs_time_relative_threshold = {
        rules = [{
            condition = {
                threshold                   = 50
                compared_to                 = "Same Day Last Week"
                ignore_infinity             = false
                condition_type                   = "LESS_THAN"
            }
            override = {
                priority = "P2"
            }
        }]
        undetected_values_management = {
            trigger_undetected_values = true
            auto_retire_timeframe     = "6_HOURS"
          }
    }
  }
}

resource "coralogix_alert" "test" {
  name        = "metric_anomaly alert example"
  description = "Example of metric_anomaly alert from terraform"
  priority    = "P1"
  type_definition = {
      metric_anomaly = {
          metric_filter = {
              promql = "sum(rate(http_requests_total{job=\"api-server\"}[5m])) by (status)"
          }
          rules = [{
              condition = {
                  threshold = 2
                  for_over_pct = 10
                  of_the_last = "10_MINUTES"
                  condition_type = "LESS_THAN"
                  min_non_null_values_pct = 50
              }
          }]
      }
  }
}

resource "coralogix_alert" "test" {
  name        = "metric_threshold alert example"
  description = "Example of metric_threshold alert from terraform"
  priority    = "P3"

  type_definition = {
    metric_threshold = {
        metric_filter = {
            promql = "sum(rate(http_requests_total{job=\"api-server\"}[5m])) by (status)"
        }
        rules = [{
            condition = {
                threshold    = 2
                for_over_pct = 10
                of_the_last = "10_MINUTES"
                condition_type = "MORE_THAN_OR_EQUALS"
            }
            override = {
                priority = "P2"
            }
        }]
        missing_values = {
            replace_with_zero = true
        }
    }
  }
}

resource "coralogix_alert" "test" {
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
          subsystem_name = [
            {
              values    = ["subsystem-name"]
            }
          ]
          operation_name        = [
            {
              values    = ["operation-name"]
            }
          ]
          span_fields = [
            {
              key         = "status"
              filter_type = {
                values    = ["200"]
              }
            },
            {
              key         = "status"
              filter_type = {
                operation = "STARTS_WITH"
                values    = ["40", "50"]
              }
            },
          ]
        }
      }
    }
  }
}

resource "coralogix_alert" "test" {
  name        = "tracing_threshold alert example"
  description = "Example of tracing_threshold alert from terraform"
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
        rules = [{
            condition = {
                time_window = "10_MINUTES"
                span_amount = 5
            }
        }]
    }
  }
}



resource "coralogix_alert" "test_1"{
    name        = "logs immediate alert 1"
    priority    = "P1"
    type_definition = {
        logs_immediate = {
        }
    }
}

resource "coralogix_alert" "test_2"{
    name        = "logs immediate alert 2"
    priority    = "P2"
    type_definition = {
        logs_immediate = {
        }
    }
}

resource "coralogix_alert" "test_3"{
    name        = "logs immediate alert 3"
    priority    = "P3"
    type_definition = {
        logs_immediate = {
        }
    }
}

resource "coralogix_alert" "test_4"{
    name        = "logs immediate alert 4"
    priority    = "P4"
    type_definition = {
        logs_immediate = {
        }
    }
}

resource "coralogix_alert" "test" {
    name        = "flow alert example"
    description = "Example of flow alert from terraform"
    priority    = "P3"
    type_definition = {
        flow = {
            enforce_suppression = false
            stages = [{
                flow_stages_groups = [{
                    alert_defs = [
                        {
                            id = coralogix_alert.test_1.id
                        },
                        {
                            id = coralogix_alert.test_2.id
                        },
                    ]
                    next_op   = "AND"
                    alerts_op = "OR"
                },
                {
                    alert_defs = [
                        {
                            id = coralogix_alert.test_3.id
                        },
                        {
                            id = coralogix_alert.test_4.id
                        },
                    ]
                    next_op   = "OR"
                    alerts_op = "AND"
                },]
                timeframe_ms   = 10
                timeframe_type = "Up To"
            }]
        }
    }
}


