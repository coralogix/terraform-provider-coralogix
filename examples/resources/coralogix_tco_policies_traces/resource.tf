resource "coralogix_tco_policies_traces" "tco_policies" {
  policies = [
    {
      name         = "Example tco_policy from terraform 1"
      priority     = "low"
      applications = {
        rule_type = "starts_with"
        names     = ["prod"]
      }
      subsystems = {
        rule_type = "is"
        names     = ["mobile", "web"]
      }
      actions = {
        rule_type = "is_not"
        names     = ["action-name", "action-name2"]
      }
      services = {
        rule_type = "is"
        names     = ["service-name", "service-name2"]
      }
      tags = {
        "tags.http.method" = {
          rule_type = "includes"
          names     = ["GET"]
        }
      }
      archive_retention_id = "e1c980d0-c910-4c54-8326-67f3cf95645a"
    },
    {
      name         = "Example tco_policy from terraform 2"
      priority     = "medium"
      applications = {
        rule_type = "starts_with"
        names     = ["staging"]
      }
      subsystems = {
        rule_type = "is_not"
        names     = ["mobile", "web"]
      }
      actions = {
        names = ["action-name", "action-name2"]
      }
      services = {
        names = ["service-name", "service-name2"]
      }
      tags = {
        "tags.http.method" = {
          rule_type = "is_not"
          names     = ["GET", "POST"]
        }
      }
    },
    {
      name         = "Example tco_policy from terraform 3"
      priority     = "medium"
      applications = {
        rule_type = "starts_with"
        names     = ["staging"]
      }
      subsystems = {
        rule_type = "is_not"
        names     = ["mobile", "web"]
      }
      actions = {
        names = ["action-name", "action-name2"]
      }
      services = {
        names = ["service-name", "service-name2"]
      }
      tags = {
        "tags.http.method" = {
          rule_type = "is_not"
          names     = ["GET", "POST"]
        }
      }
    }
    ]
}
