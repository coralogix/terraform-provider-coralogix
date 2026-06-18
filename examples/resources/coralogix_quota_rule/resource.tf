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

resource "coralogix_quota_rule" "logs_info" {
  name        = "Terraform example log quota rule"
  description = "Route info logs while quota policy is managed as code"
  enabled     = true
  priority    = "medium"

  application_rule = {
    rule_type = "starts_with"
    names     = ["prod"]
  }

  subsystem_rule = {
    rule_type = "is"
    names     = ["api"]
  }

  log_rules = {
    severities = ["info"]
  }
}

resource "coralogix_quota_rule" "dataset_target" {
  name    = "Terraform example target quota rule"
  enabled = true

  log_rules = {
    dpxl_expression = "<v1> $d.severity == 'INFO'"
  }

  targets = [
    {
      dataspace = "default"
      dataset   = "logs"
      priority  = "medium"
      quota_based_priority_override = {
        usage_tiers = [
          {
            daily_quota_percentage = 50
            priority               = "medium"
          },
          {
            daily_quota_percentage = 80
            priority               = "low"
          },
        ]
      }
    }
  ]
}

resource "coralogix_quota_rule" "span_rule" {
  name     = "Terraform example span quota rule"
  enabled  = true
  priority = "low"

  span_rules = {
    service_rule = {
      rule_type = "is"
      names     = ["checkout"]
    }

    action_rule = {
      rule_type = "includes"
      names     = ["POST"]
    }

    tag_rules = {
      "tags.http.status_code" = {
        rule_type = "starts_with"
        names     = ["5"]
      }
    }
  }
}
