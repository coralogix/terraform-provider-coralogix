terraform {
  required_providers {
    coralogix = {
      version = "~> 1.5"
      source  = "coralogix/coralogix"
    }
  }
}

provider "coralogix" {
  #api_key = "<add your api key here or add env variable CORALOGIX_API_KEY>"
  #env = "<add the environment you want to work at or add env variable CORALOGIX_ENV>"
}

resource "coralogix_tco_policy" "tco_policy_1" {
  name       = "Example tco_policy from terraform"
  priority   = "medium"
  order      = 1
  severities = ["debug", "verbose", "info"]
  application_name {
    starts_with = true
    rule        = "prod"
  }
  subsystem_name {
    is    = true
    rules = ["mobile", "web"]
  }
}

resource "coralogix_tco_policy" "tco_policy_2" {
  name     = "Example tco_policy from terraform 2"
  priority = "high"

  order    = coralogix_tco_policy.tco_policy_1.order + 1
#  currently, for controlling the policies order they have to be created by the order you want them to be.
#  for this purpose, defining dependency via the 'order' field can control their creation order.

  severities = ["error", "warning", "critical"]
  application_name {
    starts_with = true
    rule        = "prod"
  }
  subsystem_name {
    is    = true
    rules = ["mobile", "web"]
  }
}
