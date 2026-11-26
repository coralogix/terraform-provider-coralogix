---
page_title: "Keeping Secrets Out of Terraform State"
---

# Keeping Secrets Out of Terraform State

Some Coralogix resources need a secret, such as a PagerDuty integration key or a Jira API token.
This guide shows how to pass a secret to Terraform without writing it in the `.tf` file and without storing it in the state.

## The short version

Use a variable that is `sensitive` and `ephemeral`. Pass it to a write-only attribute (a `_wo` attribute).

```hcl
variable "pagerduty_integration_key" {
  type      = string
  sensitive = true # hides the value in plan and apply output
  ephemeral = true # keeps the value out of the state and out of the plan file
}

resource "coralogix_connector" "pagerduty" {
  type = "pagerduty"
  name = "pagerduty connector"
  connector_config = {
    fields = []
    field_values_wo = {
      integrationKey = var.pagerduty_integration_key
    }
    field_values_wo_versions = {
      integrationKey = 1
    }
  }
}
```

Set the value when you run Terraform:

```bash
printf 'PagerDuty integration key: '; read -rs TF_VAR_pagerduty_integration_key; echo
export TF_VAR_pagerduty_integration_key
terraform apply
```

This needs Terraform 1.11 or later. Ephemeral variables need 1.10. Write-only attributes need 1.11.

## What each option protects

| | Hidden in plan and apply output | Stored in state | Stored in saved plan file | Can feed a plain attribute | Can feed a `_wo` attribute |
|---|---|---|---|---|---|
| Literal value in the `.tf` file | no | **yes** | **yes** | yes | yes |
| `sensitive = true` variable | yes | **yes** | **yes** | yes | yes |
| `ephemeral = true` variable | yes, if also `sensitive` | no | no | **no** | yes |

`sensitive = true` hides a value in the output. It does not keep the value out of the state.
Anyone who can read the state can read the value.

## Write-only attributes

A write-only attribute sends the value to Coralogix. Terraform never writes it to the state.

| Resource | Write-only attribute | Version attribute |
|---|---|---|
| `coralogix_connector` | `connector_config.field_values_wo` | `connector_config.field_values_wo_versions` |
| `coralogix_webhook` | `custom.headers_wo` | `custom.headers_wo_versions` |
| `coralogix_webhook` | `pager_duty.service_key_wo` | `pager_duty.service_key_wo_version` |
| `coralogix_webhook` | `jira.api_token_wo` | `jira.api_token_wo_version` |
| `coralogix_webhook` | `url_wo` in `custom`, `slack`, `microsoft_teams`, `opsgenie`, and `demisto` | `url_wo_version` in the same block |
| `coralogix_connector` | `config_overrides_templates_wo` | `config_overrides_templates_wo_versions` |
| `coralogix_integration` | `parameters_wo` | `parameters_wo_versions` |

A plain attribute stores its value in the state. Examples are `connector_config.fields[].value`, `pager_duty.service_key`, `jira.api_token`, `slack.url`, `config_overrides[].fields[].template`, and `parameters` of `coralogix_integration`.
Do not use a plain attribute for a secret.

Most URLs, templates, and parameters are not secret. Keep using the plain attribute for those.
Use the write-only attribute when the value holds a credential. For a webhook URL, set exactly one of `url` and `url_wo`.

An ephemeral variable cannot feed a plain attribute. Terraform stops with an error, so a mistake cannot store the secret by accident:

```text
Error: Invalid use of ephemeral value

Ephemeral values are not valid for "connector_config", because it is not a
write-only attribute and must be persisted to state.
```

## Keys of the map attributes

`field_values_wo`, `headers_wo`, and `parameters_wo` use the name of the field, header, or parameter as the key.
`config_overrides_templates_wo` uses `<entity_type>.<field_name>` as the key. Declare the entity type in `config_overrides`, and leave the secret template out of its `fields`:

```hcl
config_overrides = [
  {
    entity_type = "alerts"
    fields      = []
  }
]
config_overrides_templates_wo = {
  "alerts.additionalHeaders" = var.connector_headers
}
config_overrides_templates_wo_versions = {
  "alerts.additionalHeaders" = 1
}
```

A key is in the plain attribute or in the write-only attribute, not in both.

## Rotate a secret

Terraform keeps no copy of a write-only value. It cannot see that the value changed.
To send a new secret, raise the version number:

```hcl
field_values_wo_versions = {
  integrationKey = 2 # was 1
}
```

## Provide the value again at apply

Terraform does not save an ephemeral value in the plan file. If you run `terraform apply plan.tfplan`, set the variable again.

## Import and data sources

An import and a data source read the secret from the API, because they have no configuration that says the value is write-only.

- `terraform import` writes the secret to the state. The next `terraform apply` removes it. Treat an imported secret as exposed and rotate it.
- `data.coralogix_connector`, `data.coralogix_webhook`, and `data.coralogix_integration` return the secret in their result, and Terraform stores it in the state of the configuration that uses the data source.
