# __generated__ by Terraform
# Please review these resources and move them into your main configuration files.

# __generated__ by Terraform
resource "coralogix_alert" "standard_alert" {
  description = "Example of standard alert from terraform"
  enabled     = true
  meta_labels = {
    alert_type        = "security"
    security_severity = "high"
  }
  name            = "Standard alert example"
  payload_filters = []
  severity        = "Critical"
  incident_settings {
    notify_on                   = "Triggered_and_resolved"
    retriggering_period_minutes = 60
  }
  notifications_group {
    group_by_fields = ["coralogix.metadata.sdkId", "EventType"]
    notification {
      email_recipients            = ["example@coralogix.com"]
      integration_id              = null
      notify_on                   = null
      retriggering_period_minutes = 0
    }
    notification {
      email_recipients            = []
      integration_id              = "12416"
      notify_on                   = null
      retriggering_period_minutes = 0
    }
  }
  notifications_group {
    group_by_fields = []
    notification {
      email_recipients            = ["example@coralogix.com"]
      integration_id              = null
      notify_on                   = null
      retriggering_period_minutes = 0
    }
  }
  standard {
    applications = ["filter:contains:nginx"]
    categories   = []
    classes      = []
    computers    = []
    ip_addresses = []
    methods      = []
    search_query = "remote_addr_enriched:/.*/"
    severities   = ["Info", "Warning"]
    subsystems   = ["filter:startsWith:subsystem-name"]
    condition {
      evaluation_window = "Dynamic"
      group_by          = ["coralogix.metadata.sdkId", "EventType"]
      group_by_key      = null
      immediately       = false
      less_than         = false
      more_than         = true
      more_than_usual   = false
      threshold         = 5
      time_window       = "30Min"
    }
  }
}
