---
subcategory: "Webhooks"
page_title: "Coralogix: coralogix_webhook"
---

# coralogix_webhook

Use this data source to retrieve information about a Coralogix webhook.

## Example Usage
Once a webhook is created with TF and you wish to refer to it.
while the webhook needs its id to use it, all it's attributes may be called upon by name.
```hcl
data "coralogix_webhook" "my_webhook" {
    id        = 3495
}
```

Using this code example will output the webhook alias:
```hcl
output "name" {
  value       = coralogix_webhook.my_webhook.alias
  description = "Webhook friendly name."
}
```

## Argument Reference

* `id` - (Required) Webhook id.

## Attribute Reference
The result is an object containing the following attributes.
* `alias` - Webhook friendly name.
* `type` - Webhook type.
* `url` - Webhook url.
* `updated_at` - Webhook last updated time in ISO 8601 format.
* `created_at` - Webhook creation time in ISO 8601 format.
* `company_id` - Webhook company id.
* `pager_duty` - Webhook pager_duty service key, only on 'pager_duty' type.
* `web_request` - Webhook web_request block, only on ['webhook','sendlog','demisto'] type.
* `jira` - Webhook jira block, only on 'jira' type.
* `email_group` - Webhook email_group array, only on 'email_group' type.
