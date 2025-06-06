---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "coralogix_tco_policies_logs Resource - terraform-provider-coralogix"
subcategory: ""
description: |-
  Coralogix TCO-Policies-List. For more information - https://coralogix.com/docs/tco-optimizer-api.
---

# coralogix_tco_policies_logs (Resource)

Coralogix TCO-Policies-List. For more information - https://coralogix.com/docs/tco-optimizer-api.

## Example Usage

```terraform
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

resource "coralogix_tco_policies_logs" "tco_policies" {
  policies = [
    {
      name       = "Example tco_policy from terraform 1"
      priority   = "low"
      severities = ["debug", "verbose", "info"]
      applications = {
        rule_type = "starts_with"
        names        = ["prod"]
      }
      subsystems = {
        rule_type = "is"
        names = ["mobile", "web"]
      }
      archive_retention_id = "e1c980d0-c910-4c54-8326-67f3cf95645a"
    },
    {
      name     = "Example tco_policy from terraform 2"
      priority = "medium"
      severities = ["error", "warning", "critical"]
      applications = {
        rule_type = "starts_with"
        names        = ["prod"]
      }
      subsystems = {
        rule_type = "is"
        names = ["mobile", "web"]
      }
    },
    {
      name     = "Example tco_policy from terraform 3"
      priority = "high"

      severities = ["error", "warning", "critical"]
      applications = {
        rule_type = "starts_with"
        names        = ["prod"]
      }
      subsystems = {
        rule_type = "is"
        names = ["mobile", "web"]
      }
    },
    {
      name     = "Example tco_policy from terraform 4"
      priority = "high"

      severities = ["error", "warning", "critical"]
      applications = {
        rule_type = "starts_with"
        names        = ["prod"]
      }
      subsystems = {
        rule_type = "is"
        names = ["mobile", "web"]
      }
    }
  ]
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `policies` (Attributes List) (see [below for nested schema](#nestedatt--policies))

### Read-Only

- `id` (String) This field can be ignored

<a id="nestedatt--policies"></a>
### Nested Schema for `policies`

Required:

- `name` (String) tco-policy name.
- `priority` (String) The policy priority. Can be one of ["block" "high" "low" "medium"].

Optional:

- `applications` (Attributes) The applications to apply the policy on. Applies the policy on all the applications by default. (see [below for nested schema](#nestedatt--policies--applications))
- `archive_retention_id` (String) Allowing logs with a specific retention to be tagged.
- `description` (String) The policy description
- `enabled` (Boolean) Determines weather the policy will be enabled. True by default.
- `severities` (Set of String) The severities to apply the policy on. Valid severities are ["critical" "debug" "error" "info" "verbose" "warning"].
- `subsystems` (Attributes) The subsystems to apply the policy on. Applies the policy on all the subsystems by default. (see [below for nested schema](#nestedatt--policies--subsystems))

Read-Only:

- `id` (String) tco-policy ID.
- `order` (Number) The policy's order between the other policies.

<a id="nestedatt--policies--applications"></a>
### Nested Schema for `policies.applications`

Required:

- `names` (Set of String)

Optional:

- `rule_type` (String) The rule type. Can be one of ["includes" "is" "is_not" "starts_with" "unspecified"].


<a id="nestedatt--policies--subsystems"></a>
### Nested Schema for `policies.subsystems`

Required:

- `names` (Set of String)

Optional:

- `rule_type` (String)
