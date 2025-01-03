# Upgrading API Keys (from < 1.16)

With version 1.16, the API Keys resource has been upgraded to the latest available version of the GRPC API which replaces "roles" with "presets" and "permissions". Read more [https://coralogix.com/docs/api-keys/](). This document is the guide on how you can update your existing Terraform files. 

## Legacy And New API Keys

The change in the underlying permission model created two types of API keys with the old one being described as "legacy keys". Internally all keys have been transitioned to the new model, however there are two key differences:

1. Legacy keys are "Custom" keys that have a custom set of permissions, instead of a preset
2. Legacy key permissions cannot be updated

Therefore legacy keys can remain in use as long as no changes to the associated permissions are required. 

### Mapping Roles to Permissions

Internally, existing roles are mapped to the following set of custom permissions:


| Role                   | Permissions                                                                                                 |
|------------------------|------------------------------------------------------------------------------------------------------------|
| RUM Ingress            | - rum-ingress:SendData                                                                                     |
| Send Data              | - cloud-metadata-ingress:SendData<br>- logs.data-ingress:SendData<br>- metrics.data-ingress:SendData<br>- spans.data-ingress:SendData |
| Coralogix CLI          | - data-usage:Read<br>- org-quota:Manage<br>- org-quota:Read<br>- org-teams:Manage<br>- org-teams:ReadConfig<br>- team-members:Manage<br>- team-members:ReadConfig<br>- team-scim:Manage<br>- team-scim:ReadConfig<br>- team-sso:Manage<br>- team-sso:ReadConfig<br>- team-quota:Manage<br>- team-quota:Read |
| SCIM                   | - team-groups:Manage<br>- team-groups:ReadConfig<br>- team-members:Manage<br>- team-members:ReadConfig<br>- team-roles:ReadConfig |
| Role Management        | - team-roles:Manage<br>- team-roles:ReadConfig                                                             |
| Trigger Webhook        | - contextual-data:SendData                                                                                 |
| Legacy Api Key         | - alerts:ReadConfig<br>- alerts:UpdateConfig<br>- cloud-metadata-enrichment:ReadConfig<br>- cloud-metadata-enrichment:UpdateConfig<br>- data-usage:Read<br>- geo-enrichment:ReadConfig<br>- geo-enrichment:UpdateConfig<br>- grafana:Read<br>- grafana:Update<br>- logs.data-setup#low:ReadConfig<br>- logs.data-setup#low:UpdateConfig<br>- logs.events2metrics:ReadConfig<br>- logs.events2metrics:UpdateConfig<br>- logs.tco:ReadPolicies<br>- logs.tco:UpdatePolicies<br>- metrics.data-analytics#high:Read<br>- metrics.data-analytics#low:Read<br>- metrics.data-setup#high:ReadConfig<br>- metrics.data-setup#high:UpdateConfig<br>- metrics.data-setup#low:ReadConfig<br>- metrics.data-setup#low:UpdateConfig<br>- metrics.recording-rules:ReadConfig<br>- metrics.recording-rules:UpdateConfig<br>- metrics.tco:ReadPolicies<br>- metrics.tco:UpdatePolicies<br>- outbound-webhooks:ReadConfig<br>- outbound-webhooks:UpdateConfig<br>- parsing-rules:ReadConfig<br>- parsing-rules:UpdateConfig<br>- security-enrichment:ReadConfig<br>- security-enrichment:UpdateConfig<br>- serverless:Read<br>- service-catalog:ReadDimensionsConfig<br>- service-catalog:ReadSLIConfig<br>- service-catalog:UpdateDimensionsConfig<br>- service-catalog:UpdateSLIConfig<br>- service-map:Read<br>- source-mapping:UploadMapping<br>- spans.data-api#high:ReadData<br>- spans.data-api#low:ReadData<br>- spans.data-setup#low:ReadConfig<br>- spans.data-setup#low:UpdateConfig<br>- spans.events2metrics:ReadConfig<br>- spans.events2metrics:UpdateConfig<br>- spans.tco:ReadPolicies<br>- spans.tco:UpdatePolicies<br>- team-actions:ReadConfig<br>- team-actions:UpdateConfig<br>- team-api-keys-security-settings:Manage<br>- team-api-keys-security-settings:ReadConfig<br>- team-api-keys:Manage<br>- team-api-keys:ReadConfig<br>- team-custom-enrichment:ReadConfig<br>- team-custom-enrichment:ReadData<br>- team-custom-enrichment:UpdateConfig<br>- team-custom-enrichment:UpdateData<br>- team-dashboards:Read<br>- team-dashboards:Update<br>- user-actions:ReadConfig<br>- user-actions:UpdateConfig<br>- user-dashboards:Read<br>- user-dashboards:Update<br>- version-benchmark-tags:Read<br>- logs.alerts:ReadConfig<br>- logs.alerts:UpdateConfig<br>- spans.alerts:ReadConfig<br>- spans.alerts:UpdateConfig<br>- metrics.alerts:ReadConfig<br>- metrics.alerts:UpdateConfig<br>- livetail:Read<br>- service-catalog:Read<br>- version-benchmark-tags:Update<br>- service-catalog:ReadApdexConfig<br>- service-catalog:UpdateApdexConfig<br>- service-catalog:Update<br>- team-quota:Manage<br>- team-quota:Read |
| Query Data Legacy      | - logs.data-api#high:ReadData<br>- logs.data-api#low:ReadData<br>- metrics.data-api#high:ReadData<br>- metrics.data-api#low:ReadData<br>- opensearch-dashboards:Read<br>- opensearch-dashboards:Update<br>- snowbit.cspm:Read<br>- snowbit.sspm:Read<br>- spans.data-api#high:ReadData<br>- spans.data-api#low:ReadData<br>- livetail:Read |

If an API key had multiple roles, the permissions are merged. 

## Upgrade Procedure

The new provider version upgrades the state automatically, however the `.tf` files need to reflect those updates as well - anything else would be considered a change by terraform. Here is a step by step upgrade guide:

0. Upgrade the provider
1. Run `terraform refresh`
2. Locate the `coralogix_api_key` resources and create a property `permissions` corresponding to the roles in `roles`. Use the table above or examples below.
3. Add a property `presets = []` and remove the `roles` property
4. Run `terraform plan`, there should be no changes to the API keys requested

## Examples 

**1.15.x:**

```hcl
resource "coralogix_api_key" "example" {
    name  = "My SCIM KEY"
    owner = {
        team_id : "5633574"
    }
    active = true
    roles = ["SCIM", "Legacy Api Key", "Role Management", "Send Data"]
}
```

**>1.16:**

```hcl
resource "coralogix_api_key" "example" {
    name        = "My SCIM KEY"
    owner       = {
        team_id : "5633574"
    }
    active      = true
    permissions = [
        "alerts:ReadConfig",
        "alerts:UpdateConfig",
        "cloud-metadata-enrichment:ReadConfig",
        "cloud-metadata-enrichment:UpdateConfig",
        "cloud-metadata-ingress:SendData",
        "data-usage:Read",
        "geo-enrichment:ReadConfig",
        "geo-enrichment:UpdateConfig",
        "grafana:Read",
        "grafana:Update",
        "incidents:Acknowledge",
        "incidents:Assign",
        "incidents:Close",
        "incidents:Read",
        "incidents:Snooze",
        "livetail:Read",
        "logs.alerts:ReadConfig",
        "logs.alerts:UpdateConfig",
        "logs.data-ingress:SendData",
        "logs.data-setup#low:ReadConfig",
        "logs.data-setup#low:UpdateConfig",
        "logs.events2metrics:ReadConfig",
        "logs.events2metrics:UpdateConfig",
        "logs.tco:ReadPolicies",
        "logs.tco:UpdatePolicies",
        "metrics.alerts:ReadConfig",
        "metrics.alerts:UpdateConfig",
        "metrics.data-analytics#high:Read",
        "metrics.data-analytics#low:Read",
        "metrics.data-ingress:SendData",
        "metrics.data-setup#high:ReadConfig",
        "metrics.data-setup#high:UpdateConfig",
        "metrics.data-setup#low:ReadConfig",
        "metrics.data-setup#low:UpdateConfig",
        "metrics.recording-rules:ReadConfig",
        "metrics.recording-rules:UpdateConfig",
        "metrics.tco:ReadPolicies",
        "metrics.tco:UpdatePolicies",
        "outbound-webhooks:ReadConfig",
        "outbound-webhooks:UpdateConfig",
        "parsing-rules:ReadConfig",
        "parsing-rules:UpdateConfig",
        "security-enrichment:ReadConfig",
        "security-enrichment:UpdateConfig",
        "serverless:Read",
        "service-catalog:Read",
        "service-catalog:ReadApdexConfig",
        "service-catalog:ReadDimensionsConfig",
        "service-catalog:ReadSLIConfig",
        "service-catalog:Update",
        "service-catalog:UpdateApdexConfig",
        "service-catalog:UpdateDimensionsConfig",
        "service-catalog:UpdateSLIConfig",
        "service-map:Read",
        "source-mapping:UploadMapping",
        "spans.alerts:ReadConfig",
        "spans.alerts:UpdateConfig",
        "spans.data-api#high:ReadData",
        "spans.data-api#low:ReadData",
        "spans.data-ingress:SendData",
        "spans.data-setup#low:ReadConfig",
        "spans.data-setup#low:UpdateConfig",
        "spans.events2metrics:ReadConfig",
        "spans.events2metrics:UpdateConfig",
        "spans.tco:ReadPolicies",
        "spans.tco:UpdatePolicies",
        "suppression-rules:ReadConfig",
        "suppression-rules:UpdateConfig",
        "team-actions:ReadConfig",
        "team-actions:UpdateConfig",
        "team-api-keys-security-settings:Manage",
        "team-api-keys-security-settings:ReadConfig",
        "team-api-keys:Manage",
        "team-api-keys:ReadConfig",
        "team-custom-enrichment:ReadConfig",
        "team-custom-enrichment:ReadData",
        "team-custom-enrichment:UpdateConfig",
        "team-custom-enrichment:UpdateData",
        "team-dashboards:Read",
        "team-dashboards:Update",
        "team-groups:Manage",
        "team-groups:ReadConfig",
        "team-members:Manage",
        "team-members:ReadConfig",
        "team-quota:Manage",
        "team-quota:Read",
        "team-roles:Manage",
        "team-roles:ReadConfig",
        "user-actions:ReadConfig",
        "user-actions:UpdateConfig",
        "user-dashboards:Read",
        "user-dashboards:Update",
        "version-benchmark-tags:Read",
        "version-benchmark-tags:Update"
    ]
    presets     = []
}
```

---

**1.15.x:**

```hcl
resource "coralogix_api_key" "example" {
    name     = "My RUM KEY"
    owner    = {
        team_id : "5633574"
    }
    active   = true
    roles    = ["RUM Ingress"]
}
```

**>1.16:**

```hcl
resource "coralogix_api_key" "example" {
    name        = "My RUM KEY"
    owner       = {
        team_id : "5633574"
    }
    active      = true
    permissions = [
        "rum-ingress:SendData"
    ]
    presets     = []
}
```
---

**1.15.x:**

```hcl
resource "coralogix_api_key" "example" {
    name     = "My WH KEY"
    owner    = {
        team_id : "5633574"
    }
    active   = true
    roles    = ["Trigger Webhook"]
}
```

**>1.16:**

```hcl
resource "coralogix_api_key" "example" {
    name        = "My WH KEY"
    owner       = {
        team_id : "5633574"
    }
    active      = true
    permissions = [
        "contextual-data:SendData"
    ]
    presets     = []
}
```

--- 

**1.15.x:**

```hcl
resource "coralogix_api_key" "example" {
    name     = "My RBAC KEY"
    owner    = {
        team_id : "5633574"
    }
    active   = true
    roles    = ["SCIM", "Role Management"]
}
```

**>1.16:**

```hcl
resource "coralogix_api_key" "example" {
    name        = "My RBAC KEY"
    owner       = {
        team_id : "5633574"
    }
    active      = true
    permissions = [
        "team-roles:Manage",
        "team-roles:ReadConfig",
        "team-groups:Manage",
        "team-groups:ReadConfig",
        "team-members:Manage",
        "team-members:ReadConfig"
    ]
    presets     = []
}
```

## Limitations

- Encrypted (hashed) keys are unsupported in this provider, therefore all keys are created in plain text

