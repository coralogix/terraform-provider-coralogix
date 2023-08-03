terraform {
  required_providers {
    coralogix = {
      version = "~> 1.5"
      source  = "coralogix/coralogix"
    }
  }
}

provider "coralogix" {
  #api_key = "<add your api key here or add env variable CORALOGIX_API_KEY>"
  #env = "<add the environment you want to work at or add env variable CORALOGIX_ENV>"
}

resource "coralogix_webhook" "slack_webhook" {
  name = "slack-webhook"
  slack {
    url = "https://join.slack.com/example"
  }
}

data "coralogix_webhook" "imported_coralogix_webhook_example" {
  id = coralogix_webhook.slack_webhook.id
}

resource "coralogix_webhook" "custom_webhook" {
  name = "custom-webhook"
  custom {
    url     = "https://example-url.com/"
    method  = "post"
    headers = jsonencode({ "Content-Type" : "application/json" })
    payload = jsonencode({
      "uuid" : "80019a4b-5ad3-5dd1-4f17-f61a386f0221",
      "alert_id" : "$ALERT_ID",
      "name" : "$ALERT_NAME",
      "description" : "$ALERT_DESCRIPTION",
      "threshold" : "$ALERT_THRESHOLD",
      "timewindow" : "$ALERT_TIMEWINDOW_MINUTES",
      "group_by_labels" : "$ALERT_GROUPBY_LABELS",
      "alert_action" : "$ALERT_ACTION",
      "alert_url" : "$ALERT_URL",
      "log_url" : "$LOG_URL",
      "icon_url" : "$CORALOGIX_ICON_URL",
      "service" : "$SERVICE",
      "duration" : "$DURATION",
      "errors" : "$ERRORS",
      "spans" : "$SPANS",
      "fields" : [
        {
          "key" : "team",
          "value" : "$TEAM_NAME"
        },
        {
          "key" : "application",
          "value" : "$APPLICATION_NAME"
        },
        {
          "key" : "subsystem",
          "value" : "$SUBSYSTEM_NAME"
        },
        {
          "key" : "severity",
          "value" : "$EVENT_SEVERITY"
        },
        {
          "key" : "computer",
          "value" : "$COMPUTER_NAME"
        },
        {
          "key" : "ipAddress",
          "value" : "$IP_ADDRESS"
        },
        {
          "key" : "timestamp",
          "value" : "$EVENT_TIMESTAMP"
        },
        {
          "key" : "hitCount",
          "value" : "$HIT_COUNT"
        },
        {
          "key" : "text",
          "value" : "$LOG_TEXT"
        },
        {
          "key" : "Custom field",
          "value" : "$JSON_KEY"
        },
        {
          "key" : "metricKey",
          "value" : "$METRIC_KEY"
        },
        {
          "key" : "metricOperator",
          "value" : "$METRIC_OPERATOR"
        },
        {
          "key" : "timeframe",
          "value" : "$TIMEFRAME"
        },
        {
          "key" : "timeframePercentageOverThreshold",
          "value" : "$TIMEFRAME_OVER_THRESHOLD"
        },
        {
          "key" : "metricCriteria",
          "value" : "$METRIC_CRITERIA"
        },
        {
          "key" : "ratioQueryOne",
          "value" : "$RATIO_QUERY_ONE"
        },
        {
          "key" : "ratioQueryTwo",
          "value" : "$RATIO_QUERY_TWO"
        },
        {
          "key" : "ratioTimeframe",
          "value" : "$RATIO_TIMEFRAME"
        },
        {
          "key" : "ratioGroupByKeys",
          "value" : "$RATIO_GROUP_BY_KEYS"
        },
        {
          "key" : "ratioGroupByTable",
          "value" : "$RATIO_GROUP_BY_TABLE"
        },
        {
          "key" : "uniqueCountValuesList",
          "value" : "$UNIQUE_COUNT_VALUES_LIST"
        },
        {
          "key" : "newValueTrackedKey",
          "value" : "$NEW_VALUE_TRACKED_KEY"
        },
        {
          "key" : "metaLabels",
          "value" : "$META_LABELS"
        }
      ]
    })
  }
}

resource "coralogix_webhook" "pager_duty_webhook" {
  name = "pagerduty-webhook"
  pager_duty {
    service_key = "service-key"
  }
}

resource "coralogix_webhook" "email_group_webhook" {
  name = "email-group-webhook"
  email_group {
    emails = ["user@example.com"]
  }
}

resource "coralogix_webhook" "microsoft_teams_webhook" {
  name = "microsoft-teams-webhook"
  microsoft_teams {
    url = "https://example-url.com/"
  }
}

resource "coralogix_webhook" "jira_webhook" {
  name = "jira-webhook"
  jira {
    url         = "https://coralogix.atlassian.net/jira/your-work"
    api_token   = "api-token"
    email       = "example@coralogix.com"
    project_key = "project-key"
  }
}

resource "coralogix_webhook" "opsgenie_webhook" {
  name = "opsgenie-webhook"
  opsgenie {
    url = "https://example-url.com/"
  }
}

resource "coralogix_webhook" "demisto_webhook" {
  name = "demisto-webhook"
  demisto {
    payload = jsonencode({
      "privateKey" : "<send-your-logs-privatekey>",
      "applicationName" : "Coralogix Alerts",
      "subsystemName" : "Coralogix Alerts",
      "computerName" : "$COMPUTER_NAME",
      "logEntries" : [
        {
          "severity" : 3,
          "timestamp" : "$EVENT_TIMESTAMP_MS",
          "text" : {
            "integration_text" : "Security Incident",
            "alert_application" : "$APPLICATION_NAME",
            "alert_subsystem" : "$SUBSYSTEM_NAME",
            "alert_severity" : "$EVENT_SEVERITY",
            "alert_id" : "$ALERT_ID",
            "alert_name" : "$ALERT_NAME",
            "alert_url" : "$ALERT_URL",
            "hit_count" : "$HIT_COUNT",
            "alert_type_id" : "53d222e2-e7b2-4fa6-80d4-9935425d47dd"
          }
        }
      ],
      "uuid" : "45c2d83a-1c75-2360-8741-dd75aabc8d57"
    })
  }
}

resource "coralogix_webhook" "sendlog_webhook" {
  name = "sendlog-webhook"
  sendlog {
    payload = jsonencode({
      "privateKey" : "<send-your-logs-privatekey>",
      "applicationName" : "$APPLICATION_NAME",
      "subsystemName" : "$SUBSYSTEM_NAME",
      "computerName" : "$COMPUTER_NAME",
      "logEntries" : [
        {
          "severity" : 3,
          "timestamp" : "$EVENT_TIMESTAMP_MS",
          "text" : {
            "integration_text" : "<Insert your desired integration description>",
            "alert_severity" : "$EVENT_SEVERITY",
            "alert_id" : "$ALERT_ID",
            "alert_name" : "$ALERT_NAME",
            "alert_url" : "$ALERT_URL",
            "hit_count" : "$HIT_COUNT"
          }
        }
      ],
      "uuid" : "<same-uuid>"
    })
  }
}

//example of how to use webhooks that was created via terraform
resource "coralogix_alert" "standard_alert" {
  name        = "Standard alert example"
  description = "Example of standard alert from terraform"
  severity    = "Critical"

  notifications_group {
    notification {
      integration_id              = coralogix_webhook.slack_webhook.id
      retriggering_period_minutes = 60
    }
  }

  standard {
    search_query = "remote_addr_enriched:/.*/"
    condition {
      immediately = true
    }
  }
}