terraform {
  required_providers {
    coralogix = {
      version = "~> 3.0"
      source  = "coralogix/coralogix"
    }
  }
}

provider "coralogix" {
  #api_key = "<add your api key here or add env variable CORALOGIX_API_KEY>"
  #env = "<add the environment you want to work at or add env variable CORALOGIX_ENV>"
}

# RUM TCO policies behave like coralogix_tco_policies_logs but have no dataset
# routing (no `targets`). Priority may be set directly, or driven by a
# quota_based_priority_override; if both are omitted the API rejects the policy.
resource "coralogix_tco_policies_rum" "tco_policies" {
  policies = [
    {
      name       = "Example rum tco_policy 1"
      priority   = "low"
      severities = ["error", "warning", "info"]
      applications = {
        rule_type = "starts_with"
        names     = ["prod"]
      }
      subsystems = {
        rule_type = "is"
        names     = ["mobile", "web"]
      }
      archive_retention_id = "e1c980d0-c910-4c54-8326-67f3cf95645a"
    },
    {
      name       = "Example rum tco_policy 2"
      priority   = "medium"
      severities = ["error", "critical"]
      subsystems = {
        rule_type = "is"
        names     = ["checkout"]
      }
    },
    # DPXL-expression-based matcher. Mutually exclusive with `severities` — set
    # exactly one. The expression must include a version prefix (e.g. `<v1>`) and
    # reference the canonical `$d.*` schema (not `$d.cx_rum.*`).
    {
      name            = "Example rum tco_policy with DPXL expression"
      description     = "Match RUM events via DataPrime expression instead of severities"
      priority        = "high"
      dpxl_expression = "<v1> $d.severity == 'Error'"
    },
    # Quota-based priority override: dynamically reassign the policy's priority
    # based on daily quota consumption tiers. `priority` here is the fallback
    # applied once all tiers are exhausted; if omitted the backend defaults it to
    # `block`.
    {
      name        = "Example rum tco_policy with quota-based override"
      description = "Drop priority as daily quota is consumed"
      priority    = "block"
      severities  = ["info", "warning"]
      quota_based_priority_override = {
        usage_tiers = [
          { daily_quota_percentage = 50, priority = "medium" },
          { daily_quota_percentage = 80, priority = "low" },
        ]
      }
    },
  ]
}
