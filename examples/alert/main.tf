provider "coralogix" {
    api_key = var.api_key
}

resource "coralogix_alert" "example" {
    name     = var.alert_name
    severity = "info"
    enabled  = var.alert_enabled
    type     = "text"
    filter {
        text         = ""
        applications = []
        subsystems   = []
        severities   = []
    }
    condition {
        condition_type = "more_than"
        threshold      = 100
        timeframe      = "30MIN"
    }
    notifications {
        emails = [
            "user@example.com"
        ]
    }
}