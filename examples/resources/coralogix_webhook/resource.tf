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

resource "coralogix_webhook" "slack_webhook" {
  name  = "slack-webhook"
  slack = {
    notify_on    = ["flow_anomalies"]
    url          = "https://join.slack.com/example"
    attachments  = [{
      type  = "metric_snapshot"
      active = true
    }]
  }
}

resource "coralogix_webhook" "custom_webhook" {
  name   = "custom-webhook"
  custom = {
    method  = "post"
    headers = { "Content-Type" : "application/json" }
    url     = "https://example-url.com/"
  }
}

resource "coralogix_webhook" "pager_duty_webhook" {
  name       = "pagerduty-webhook"
  pager_duty = {
    service_key = "service-key"
  }
}

resource "coralogix_webhook" "email_group_webhook" {
  name        = "email-group-webhook"
  email_group = {
    emails = ["user@example.com"]
  }
}

resource "coralogix_webhook" "microsoft_teams_webhook" {
  name            = "microsoft-teams-webhook"
  microsoft_teams_workflow = {
    url = "https://example-url.com/"
  }
}

resource "coralogix_webhook" "jira_webhook" {
  name = "jira-webhook"
  jira = {
    api_token   = "api-token"
    email       = "example@coralogix.com"
    project_key = "project-key"
    url         = "https://coralogix.atlassian.net/jira/your-work"
  }
}

resource "coralogix_webhook" "opsgenie_webhook" {
  name     = "opsgenie-webhook"
  opsgenie = {
    url = "https://example-url.com/"
  }
}

resource "coralogix_webhook" "demisto_webhook" {
  name    = "demisto-webhook"
  demisto = {
    url = "https://example-url.com/"
  }
}

resource "coralogix_webhook" "sendlog_webhook" {
  name    = "sendlog-webhook"
  sendlog = {
    url = "https://example-url.com/"
  }
}

resource "coralogix_webhook" "event_bridge_webhook" {
  name         = "event_bridge_webhook"
  event_bridge = {
    event_bus_arn = "arn:aws:events:us-east-1:123456789012:event-bus/default"
    detail        = "example_detail"
    detail_type   = "example_detail_type"
    source        = "example_source"
    role_name     = "example_role_name"
  }
}

//example of how to use webhooks that was created via terraform
resource "coralogix_alert" "alert_with_webhook" {
  name        = "alert example with webhook"
  description = "Example of logs_immediate alert from terraform"
  priority    = "P2"

  notification_group {
    webhooks_settings {
      integration_id              = coralogix_webhook.slack_webhook.external_id
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