data "coralogix_webhook" "imported_webhook_by_id" {
  id = coralogix_webhook.slack_webhook.id
}

data "coralogix_webhook" "imported_webhook_by_name" {
  name = coralogix_webhook.slack_webhook.name
}
