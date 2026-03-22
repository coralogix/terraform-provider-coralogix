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

# Example 1: Suppress ALL alert activity for specific alerts (no group-by filtering)
# Use "source logs | filter true" to suppress all triggered values
resource "coralogix_alerts_scheduler" "suppress_all" {
  name        = "Maintenance Window - Suppress All"
  description = "Suppress all alert activity during maintenance window"
  filter = {
    what_expression   = "source logs | filter true"
    alerts_unique_ids = ["ed6f3713-d827-49a2-9bb6-a8dba8b8c580"]
  }
  schedule = {
    operation = "mute"
    one_time = {
      time_frame = {
        start_time = "2025-01-04T00:00:00.000"
        end_time   = "2025-01-04T06:00:00.000"
        time_zone  = "UTC+2"
      }
    }
  }
}

# Example 2: Suppress only specific group-by values
# The what_expression filters which triggered values to suppress
# Note: "source logs" syntax works for ALL alert types (logs, metrics, tracing)
resource "coralogix_alerts_scheduler" "suppress_specific_values" {
  name        = "Suppress Test Environment Alerts"
  description = "Suppress alerts only when environment=test"
  filter = {
    what_expression   = "source logs | filter $d.environment == 'test'"
    alerts_unique_ids = ["ed6f3713-d827-49a2-9bb6-a8dba8b8c580"]
  }
  schedule = {
    operation = "mute"
    one_time = {
      time_frame = {
        start_time = "2025-01-04T00:00:00.000"
        end_time   = "2025-01-05T00:00:00.000"
        time_zone  = "UTC+2"
      }
    }
  }
}

# Example 3: Suppress with multiple conditions
# Works with any alert group-by keys including metric labels
resource "coralogix_alerts_scheduler" "suppress_multiple_conditions" {
  name        = "Suppress Staging Cluster Alerts"
  description = "Suppress alerts for staging cluster in us-east region"
  filter = {
    what_expression   = "source logs | filter $d.cluster == 'staging' && $d.region == 'us-east-1'"
    alerts_unique_ids = ["ed6f3713-d827-49a2-9bb6-a8dba8b8c580"]
  }
  schedule = {
    operation = "mute"
    one_time = {
      time_frame = {
        start_time = "2025-01-04T00:00:00.000"
        end_time   = "2025-01-05T00:00:00.000"
        time_zone  = "UTC+2"
      }
    }
  }
}

# Example 4: Recurring suppression with meta_labels selector
resource "coralogix_alerts_scheduler" "recurring_suppression" {
  name        = "Weekly Maintenance Window"
  description = "Suppress alerts every Sunday for maintenance"
  filter = {
    what_expression = "source logs | filter true"
    meta_labels = [
      {
        key   = "team"
        value = "platform"
      }
    ]
  }
  schedule = {
    operation = "mute"
    recurring = {
      dynamic = {
        repeat_every = 1
        frequency = {
          weekly = {
            days = ["Sunday"]
          }
        }
        time_frame = {
          start_time = "2025-01-05T02:00:00.000"
          duration = {
            for_over  = 4
            frequency = "hours"
          }
          time_zone = "UTC+0"
        }
        termination_date = "2026-01-01T00:00:00.000"
      }
    }
  }
}