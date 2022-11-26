terraform {
  required_providers {
    coralogix = {
      version = "~> 1.3"
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
  alert_severity = "Critical"

  meta_labels {
    key   = "alert_type"
    value = "security"
  }
  meta_labels {
    key   = "security_severity"
    value = "high"
  }

  notification {
    recipients {
      emails      = ["or.novogroder@coralogix.com"]
      webhook_ids = ["WebhookAlerts"]
    }
    notify_every_sec = 60
  }

  scheduling {
    days_enabled = ["Wednesday", "Thursday"]
    start_time   = "7:30"
    end_time     = "22:30"
  }

  scheduling {
    days_enabled = ["Sunday", "Monday"]
    start_time   = "7:30"
    end_time     = "22:30"
  }

  standard {
    applications = ["nginx"]
    subsystems   = ["training"]
    severities   = ["Warning", "Info"]
    search_query = "remote_addr_enriched:/.*/"
    condition {
      immediately = true
    }
  }
}

data "coralogix_alert" "imported_standard_alert" {
  id = coralogix_alert.standard_alert.id
}

resource "coralogix_alert" "ratio_alert" {
  name           = "Ratio alert example"
  description    = "Example of ratio alert from terraform"
  alert_severity = "Critical"

  notification {
    on_trigger_and_resolved = true
    recipients {
      emails      = ["or.novogroder@coralogix.com"]
      webhook_ids = ["WebhookAlerts"]
    }
    notify_every_sec = 60
    notify_only_on_triggered_group_by_values = true
  }

  scheduling {
    days_enabled = ["Sunday", "Monday"]
    start_time   = "7:30"
    end_time     = "22:30"
  }

  ratio {
    query_1 {

    }
    query_2 {
      applications = ["nginx"]
      subsystems   = ["training"]
      severities   = ["Warning"]
    }
    condition {
      more_than     = true
      queries_ratio = 2
      time_window   = "10Min"
      group_by      = ["coralogix.metadata.sdkId"]
      group_by_q1   = true
    }
  }
}

resource "coralogix_alert" "new_value_alert" {
  name           = "New value alert example"
  description    = "Example of new value alert from terraform"
  alert_severity = "Info"
  notification {
    recipients {
      emails      = ["or.novogroder@coralogix.com"]
      webhook_ids = ["WebhookAlerts"]
    }
    notify_every_sec = 60
  }

  scheduling {
    days_enabled = ["Sunday", "Monday"]
    start_time   = "7:30"
    end_time     = "22:30"
  }


  new_value {
    severities = ["Info"]
    condition {
      key_to_track = "remote_addr_geoip.country_name"
      time_window  = "12H"
    }
  }
}

resource "coralogix_alert" "time_relative_alert" {
  name           = "Time relative alert example"
  description    = "Example of time relative alert from terraform"
  alert_severity = "Critical"
  notification {
    recipients {
      emails      = ["or.novogroder@coralogix.com"]
      webhook_ids = ["WebhookAlerts"]
    }
    notify_every_sec = 60
  }

  scheduling {
    days_enabled = ["Sunday", "Monday"]
    start_time   = "7:30"
    end_time     = "22:30"
  }


  time_relative {
    severities = ["Error"]
    condition {
      more_than            = true
      ratio_threshold      = 2
      relative_time_window = "Same_hour_last_week"
    }
  }
}

resource "coralogix_alert" "metric_lucene_alert" {
  name           = "Metric lucene alert example"
  description    = "Example of metric lucene alert from terraform"
  alert_severity = "Critical"

  notification {
    on_trigger_and_resolved = true
    recipients {
      emails      = ["or.novogroder@coralogix.com"]
      webhook_ids = ["WebhookAlerts"]
    }
    notify_every_sec = 60
  }

  scheduling {
    days_enabled = ["Sunday", "Monday"]
    start_time   = "7:30"
    end_time     = "22:30"
  }

  metric {
    lucene {
      search_query = "name:\"Frontend transactions\""
      condition {
        metric_field                 = "subsystem"
        arithmetic_operator          = "Avg"
        more_than                    = true
        threshold                    = 60
        arithmetic_operator_modifier = 2
        sample_threshold_percentage  = 50
        time_window                  = "30Min"
      }
    }
  }
}

resource "coralogix_alert" "metric_promql_alert" {
  name           = "Metric promql alert example"
  description    = "Example of metric promql alert from terraform"
  alert_severity = "Critical"

  notification {
    on_trigger_and_resolved = true
    recipients {
      emails      = ["or.novogroder@coralogix.com"]
      webhook_ids = ["WebhookAlerts"]
    }
    notify_every_sec = 60
  }

  scheduling {
    days_enabled = ["Sunday", "Monday"]
    start_time   = "7:30"
    end_time     = "22:30"
  }

  metric {
    promql {
      search_query = "status.numeric:[500 TO *] AND env:production"
      condition {
        more_than                      = true
        threshold                      = 3
        sample_threshold_percentage    = 50
        time_window                    = "12H"
        min_non_null_values_percentage = 55
      }
    }
  }
}

resource "coralogix_alert" "unique_count_alert" {
  name           = "Unique count alert example"
  description    = "Example of unique count alert from terraform"
  alert_severity = "Info"

  notification {
    recipients {
      emails      = ["or.novogroder@coralogix.com"]
      webhook_ids = ["WebhookAlerts"]
    }
    notify_every_sec = 60
  }

  scheduling {
    days_enabled = ["Sunday", "Monday"]
    start_time   = "7:30"
    end_time     = "22:30"
  }

  unique_count {
    severities = ["Info"]
    condition {
      unique_count_key               = "remote_addr_geoip.country_name"
      max_unique_values              = 2
      time_window                    = "10Min"
      group_by_key                   = "EventType"
      max_unique_values_for_group_by = 500
    }
  }
}

resource "coralogix_alert" "tracing_alert" {
  name           = "Tracing alert example"
  description    = "Example of tracing alert from terraform"
  alert_severity = "Info"

  notification {
    //on_trigger_and_resolved = true
    recipients {
      emails      = ["or.novogroder@coralogix.com"]
      webhook_ids = ["WebhookAlerts"]
    }
    notify_every_sec = 60
  }

  scheduling {
    days_enabled = ["Sunday", "Monday"]
    start_time   = "7:30"
    end_time     = "22:30"
  }


  tracing {
    severities           = ["Info"]
    latency_threshold_ms = 20.5
    field_filters {
      field = "Application"
      filters {
        values   = ["nginx"]
        operator = "Equals"
      }
    }
    condition {
      more_than             = true
      time_window           = "5Min"
      occurrences_threshold = 2
    }
  }
}

resource "coralogix_alert" "flow_alert" {
  name           = "Flow alert example"
  description    = "Example of flow alert from terraform"
  alert_severity = "Info"

  notification {
    recipients {
      emails      = ["or.novogroder@coralogix.com"]
      webhook_ids = ["WebhookAlerts"]
    }
    notify_every_sec = 60
  }

  scheduling {
    days_enabled = ["Sunday", "Monday"]
    start_time   = "7:30"
    end_time     = "22:30"
  }

  flow {
    stages {
      groups {
        sub_alerts {
          user_alert_id = "00bf3eb5-5681-4167-9611-ab0d6b902d84" //coralogix_alert.standard_alert.id
        }
        operator = "OR"
      }
    }
    stages {
      groups {
        sub_alerts {
          user_alert_id = "d47a5aef-3fa3-4cdd-87df-9e0367372647" //coralogix_alert.unique_count_alert.id
        }
        sub_alerts {
          user_alert_id = "7a65d9fd-c52a-4eae-953e-6ac24558aa20" //coralogix_alert.new_value_alert.id
        }
        operator = "OR"
      }
      time_window {
        minutes = 20
      }
    }
  }
}