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

resource "coralogix_tco_policies_logs" "tco_policies" {
  policies = [
    # Standard policy without targets.
    {
      name       = "Example tco_policy from terraform 1"
      priority   = "low"
      severities = ["debug", "verbose", "info"]
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
      name       = "Example tco_policy from terraform 2"
      priority   = "medium"
      severities = ["error", "warning", "critical"]
      applications = {
        rule_type = "starts_with"
        names     = ["prod"]
      }
      subsystems = {
        rule_type = "is"
        names     = ["mobile", "web"]
      }
    },
    {
      name       = "Example tco_policy from terraform 3"
      priority   = "high"
      severities = ["error", "warning", "critical"]
      applications = {
        rule_type = "starts_with"
        names     = ["prod"]
      }
      subsystems = {
        rule_type = "is"
        names     = ["mobile", "web"]
      }
    },
    {
      name       = "Example tco_policy from terraform 4"
      priority   = "high"
      severities = ["error", "warning", "critical"]
      applications = {
        rule_type = "starts_with"
        names     = ["prod"]
      }
      subsystems = {
        rule_type = "is"
        names     = ["mobile", "web"]
      }
    },
    # DPXL-expression-based matcher. Mutually exclusive with `severities` — set
    # exactly one. The expression must include a version prefix, e.g. `<v1>`.
    {
      name            = "Example tco_policy with DPXL expression"
      description     = "Match logs via DataPrime expression instead of severities"
      priority        = "high"
      dpxl_expression = "<v1> $d.severity == 'INFO'"
    },
    # Quota-based priority override: dynamically reassign the policy's priority
    # based on daily quota consumption tiers.
    {
      name        = "Example tco_policy with quota-based override"
      description = "Drop priority as daily quota is consumed"
      # priority is the "Route the remaining quota to" fallback, applied once all
      # usage_tiers are exhausted. It must be more restrictive than the last tier
      # (most to least restrictive: block, low, medium, high); the last tier here
      # is "low", so the fallback is "block".
      priority   = "block"
      severities = ["info", "warning"]
      quota_based_priority_override = {
        usage_tiers = [
          { daily_quota_percentage = 50, priority = "medium" },
          { daily_quota_percentage = 80, priority = "low" },
        ]
      }
    },
    # No targets — standard priority-based routing. Behind the scenes the backend routes the
    # policy to a single default `logs` dataset using the policy-level priority; that implicit
    # target is not reflected in Terraform state, so `targets` stays absent here.
    {
      name       = "Example tco_policy without targets"
      priority   = "medium"
      severities = ["info", "warning"]
    },
    # Targets: route matched logs to specific named datasets. Every target carries its own
    # priority, and the policy-level priority is omitted.
    {
      name       = "Example tco_policy with targets"
      severities = ["info", "warning"]
      targets = [
        {
          dataset   = "dataset-a"
          dataspace = "default"
          priority  = "medium"
        },
        {
          dataset  = "dataset-b"
          priority = "low"
        },
      ]
    },
  ]
}
