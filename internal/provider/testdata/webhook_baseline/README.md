# Webhook protocol baseline

These files record the handwritten `coralogix_webhook` resource on `master`.
The tests use the Terraform provider protocol and a local fake HTTP service.
They make no live API calls.

- `schema_current.txt` records every resource attribute, default, validator, and plan modifier.
- `scenario_*.json` records create, refresh, import, update, and delete for all ten writable webhook types.
- `field_*.json`, `optional_*.json`, and `write_only_*.json` record field changes, omission, and secret rotation.
- `edge_*.json` records validation, legacy Teams import, duplicate items, missing resources, and API errors.

The edge files also record that legacy Teams Create fails before an API call,
legacy Teams Update sends an ID without data, and an omitted generic method
sends an empty enum value and gets a 400 response.
They record a service-returned SendLog value without a usable address.
The write-only files record rotation and a switch to the ordinary field.
Their test values are synthetic and masked in the JSON files.
For fields whose omission rejects Create, the optional files also record
removal from an existing webhook on Update.
The enum cases cover every allowed custom method, Slack digest, and Slack
attachment type, including `empty`.

The fake models the webhook read and write shapes. It also models duplicate
email addresses and Slack items.
It does not implement every backend validation rule. A golden records what the
handwritten provider does with these replies. It is not a live API recording.

Run `go test ./internal/provider -run TestWebhookBaseline` to compare the files.
Set `UPDATE_GOLDEN=1` to record them after an intentional behavior change.
