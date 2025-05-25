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
        query = "avg(rate(cpu_usage_seconds_total[5m])) by (instance)"
      }
      total_events = {
        query = "avg(rate(cpu_usage_seconds_total[5m])) by (instance)"
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
