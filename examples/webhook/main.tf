terraform {
  required_providers {
    coralogix = {
      version = "~> 1.3"
      source  = "locally/debug/coralogix"
    }
  }
}

provider "coralogix" {
  #api_key = "<add your api key here or add env variable CORALOGIX_API_KEY>"
  #env = "<add the environment you want to work at or add env variable CORALOGIX_ENV>"
}

resource "coralogix_webhook" "slack_webhook" {
  slack {
    name = "slack-webhook"
    url  = "https://join.slack.com/example"
  }
}

data "coralogix_webhook" "imported_coralogix_webhook_example"{
  id = coralogix_webhook.slack_webhook.id
}

resource "coralogix_webhook" "webhook_webhook" {
  webhook {
    name    = "webhook-webhook"
    url     = "https://api.eu2.coralogix.com/api/v1/external/integrations/"
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
  pager_duty {
    name        = "pagerduty-webhook"
    service_key = "service-key"
  }
}

resource "coralogix_webhook" "email_group_webhook" {
  email_group {
    name   = "email-group-webhook"
    emails = ["or.novog@gmail.com", "or.novogroder@coralgix.com"]
  }
}

resource "coralogix_webhook" "microsoft_teams_webhook" {
  microsoft_teams {
    name = "microsoft-teams-webhook"
    url  = "https://api.eu2.coralogix.com/api/v1/external/integrations/"
  }
}

resource "coralogix_webhook" "jira_webhook" {
  jira {
    name        = "jira-webhook"
    url         = "https://coralogix.atlassian.net/jira/your-work"
    api_token   = "api-token"
    email       = "example@coralogix.com"
    project_key = "project-key"
  }
}

resource "coralogix_webhook" "opsgenie_webhook" {
  opsgenie {
    name = "opsgenie-webhook"
    url  = "https://api.eu2.coralogix.com/api/v1/external/integrations/"
  }
}

resource "coralogix_webhook" "demisto_webhook" {
  demisto {
    name    = "demisto-webhook"
    payload = jsonencode({
      "privateKey" : "d3bd1e44-5603-09db-52b6-cce4c850dc9e",
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
  sendlog {
    name    = "sendlog-webhook"
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