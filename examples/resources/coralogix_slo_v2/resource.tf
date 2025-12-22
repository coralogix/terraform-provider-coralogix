terraform {
  required_providers {
    coralogix = {
      version = "~> 3.0"
      source  = "coralogix/coralogix"
    }
  }
}

provider "coralogix" {
  #api_key = "<add your api key here or add env variable CORALOGIX_API_KEY>"
  #env = "<add the environment you want to work at or add env variable CORALOGIX_ENV>"
}

resource "coralogix_slo_v2" "example_request_based_slo" {
  name                        = "coralogix_slo_go_example"
  description                 = "Example SLO for Coralogix using request-based metrics"
  target_threshold_percentage = 30.5
  labels = {
    label1 = "value1"
  }
  sli = {
    request_based_metric_sli = {
      good_events = {
        query = "avg(rate(cpu_usage_seconds_total[1m])) by (instance)"
      }
      total_events = {
        query = "avg(rate(cpu_usage_seconds_total[1m])) by (instance)"
      }
    }
  }

  window = {
    slo_time_frame = "7_days"
  }
}

resource "coralogix_slo_v2" "example_window_based_slo" {
  name                        = "coralogix_window_based_slo"
  description                 = "Example SLO using window-based metrics"
  target_threshold_percentage = 95
  labels = {
    env     = "prod"
    service = "api"
  }
  sli = {
    window_based_metric_sli = {
      query = {
        query = "avg(avg_over_time(request_duration_seconds[1m]))"
      }
      window              = "1_minute"
      comparison_operator = "less_than"
      threshold           = 0.232
    }
  }
  window = {
    slo_time_frame = "28_days"
  }
}

resource "coralogix_alert" "slo_alert_burn_rate" {
  name        = "SLO burn rate alert"
  description = "Alert based on SLO burn rate threshold"
  phantom_mode = false
  labels = {
    alert_type        = "security"
    security_severity = "high"
  }
  notification_group = {
    webhooks_settings = [{
      retriggering_period = {
        minutes = 5
      }
      notify_on  = "Triggered and Resolved"
      recipients = ["example@coralogix.com"]
    }]
  }
  schedule = {
    active_on = {
      days_of_week = ["Wednesday", "Thursday"]
      start_time   = "08:30"
      end_time     = "20:30"
    }
  }
  type_definition = {
    slo_threshold = {
      slo_definition = {
        slo_id = coralogix_slo_v2.example_request_based_slo.id
      }
      burn_rate = {
        rules = [
          {
            condition = {
              threshold = 1.0
            }
            override = {
              priority = "P1"
            }
          },
          {
            condition = {
              threshold = 1.3
            }
            override = {
              priority = "P2"
            }
          }
        ]
        single = {
          time_duration = {
            duration = 1
            unit     = "HOURS"
          }
        }
      }
    }
  }
}

resource "coralogix_alert" "slo_alert_error_budget" {
  name         = "SLO error budget alert"
  description  = "Alert based on SLO error budget threshold"
  phantom_mode = false
  labels = {
    alert_type        = "performance"
    security_severity = "medium"
  }
  notification_group = {
    webhooks_settings = [{
      retriggering_period = {
        minutes = 10
      }
      notify_on  = "Triggered and Resolved"
      recipients = ["example@coralogix.com"]
    }]
  }
  schedule = {
    active_on = {
      days_of_week = ["Monday", "Friday"]
      start_time   = "09:00"
      end_time     = "18:00"
    }
  }
  type_definition = {
    slo_threshold = {
      slo_definition = {
        slo_id = coralogix_slo_v2.example_window_based_slo.id
      }
      error_budget = {
        rules = [{
          condition = {
            threshold = 0.8
          }
          override = {
            priority = "P2"
          }
        }]
      }
    }
  }
}