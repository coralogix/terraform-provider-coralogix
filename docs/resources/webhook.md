---
subcategory: "Webhooks"
page_title: "Coralogix: coralogix_webhook"
---

# coralogix_webhook

Provides the Coralogix Webhook resource. This allows webhooks to be created, updated, and deleted.

## Example Usage

```hcl
resource "coralogix_webhook" "slack-webhook" {
  alias = "tf-slack-webhook"
  url = "https://test.coralogix.com"
  type = "slack"
}

resource "coralogix_webhook" "microsoft-webhook" {
  alias = "tf-microsoft-webhook"
  url = "https://test.coralogix.com"
  type = "microsoft_teams"
}

resource "coralogix_webhook" "opsgenie-webhook" {
  alias = "tf-opsgenie-webhook"
  url = "https://test.coralogix.com"
  type = "opsgenie"
}

resource "coralogix_webhook" "pager-webhook" {
  alias = "tf-pager-webhook"
  url = "https://test.coralogix.com"
  type = "pager_duty"
  pager_duty = "example-service-key"
}

resource "coralogix_webhook" "jira-webhook" {
  alias = "tf-jira-webhook"
  url = "https://test.coralogix.com"
  type = "jira"
  jira {
    api_token = "example-api-token"
    email = "user@example.com"
    project_key = "exampleKey"
  }
}

resource "coralogix_webhook" "email-webhook" {
  alias = "tf-email-webhook"
  url = "https://test.coralogix.com"
  type = "email_group"
  email_group = ["user@example.com", "user2@example.com"]
}

resource "coralogix_webhook" "webrequest-webhook" {
  alias = "tf-webrequest-webhook"
  url = "https://test.coralogix.com"
  type = "webhook"
  web_request {
    uuid = "f2f29f49-6256-4ee7-b0f8-dc48d1eccf03"
    method = "get"

  }
}

resource "coralogix_webhook" "demisto-webhook" {
  alias = "tf-demisto-webhook"
  url = "https://api.coralogix.com/api/v1/logs"
  type = "demisto"
  web_request {
    uuid = "cc3a9052-59b1-92e0-0a9c-108a1c0a9e08"
    method = "post"
    headers = jsonencode({"Content-Type":"application/json"}) 
    payload = jsonencode({
      "privateKey": "example-coralogix-private-key",
      "applicationName": "Coralogix Alerts",
      "subsystemName": "Coralogix Alerts",
      "computerName": "$COMPUTER_NAME",
      "logEntries": [
        {
          "severity": 3,
          "timestamp": "$EVENT_TIMESTAMP_MS",
          "text": {
            "integration_text": "Security Incident",
            "alert_application": "$APPLICATION_NAME",
            "alert_subsystem": "$SUBSYSTEM_NAME",
            "alert_severity": "$EVENT_SEVERITY",
            "alert_id": "$ALERT_ID",
            "alert_name": "$ALERT_NAME",
            "alert_url": "$ALERT_URL",
            "hit_count": "$HIT_COUNT",
            "alert_type_id": "53d222e2-e7b2-4fa6-80d4-9935425d47dd"
          }
        }
      ],
      "uuid": "cc3a9052-59b1-92e0-0a9c-108a1c0a9e08"
    })
  }
}

resource "coralogix_webhook" "sendlog-webhook" {
  alias = "tf-sendlog-webhook"
  url = "https://api.coralogix.com/api/v1/logs"
  type = "sendlog"
  web_request {
    uuid = "74dcd6ee-1e94-eaef-fe06-cf6635dfee20"
    method = "post"
    headers = jsonencode({"Content-Type":"application/json"}) 
    payload = jsonencode({
      "privateKey": "example-coralogix-private-key",
      "applicationName": "$APPLICATION_NAME",
      "subsystemName": "$SUBSYSTEM_NAME",
      "computerName": "$COMPUTER_NAME",
      "logEntries": [
        {
          "severity": 3,
          "timestamp": "$EVENT_TIMESTAMP_MS",
          "text": {
            "integration_text": "Insert your desired integration description",
            "alert_severity": "$EVENT_SEVERITY",
            "alert_id": "$ALERT_ID",
            "alert_name": "$ALERT_NAME",
            "alert_url": "$ALERT_URL",
            "hit_count": "$HIT_COUNT"
          }
        }
      ],
      "uuid": "74dcd6ee-1e94-eaef-fe06-cf6635dfee20"
    })
  }
}
```

## Argument Reference

* `alias` - (Required) Webhook friendly name.
* `url` - (Required) Webhook destination, a full vaild URL.
* `type` - (Required) Webhook type, one of the following: `slack`, `microsoft_teams`, `opsgenie`, `pager_duty`, `jira`, `email_group`, `webhook`, `demisto`, `sendlog`.
* `pager_duty` - (Optional) Pager duty service key, required on `pager_duty` webhook type
* `email_group` - (Optional) An array of emails to send to, required on `email_group` webhook type.
* `jira` - (Optional) A `jira` block as documented below, required on `jira` webhook type.
* `web_request` - (Optional) A `web_request` block as documented below, required on `webhook`, `sendlog`, `demisto` webhooks type.

---

Each `jira` block should contains the following:

* `api_token` - (Required) The jira api token.
* `email` - (Required) The jira email.
* `project_key` - (Required) The jira project key.

Each `web_request` block should contains the following:

* `uuid` - (Required) A unique uuid of the web request.
* `method` - (Required) The method of the webhook, can be `get`, `post` or `put`.
* `headers` - (Optional) The headers to be used in the web_request. must be in escaped json format.
* `payload` - (Optional) The payload to be used in the web_request. must be in escaped json format.

** To escape a json in terraform use jsonencode().

## Import

Webhooks can be imported using their id.

First create a new webhook block:

```hcl
resource "coralogix_webhook" "my_webhook" {
}
```
And then import it:

```
$ terraform import coralogix_webhook.my_webhook ID
```

After that go to your .tfstate file and implement the data for your webhook inside the resource block.

The id can be retrieved from the API with a GET request,
for more information regarding the API - https://coralogix.com/docs/webhooks-api/