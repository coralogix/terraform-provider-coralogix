terraform {
  required_providers {
    coralogix = {
      version = "~> 1.5"
      source  = "locally/debug/coralogix"
    }
  }
}

provider "coralogix" {
  #api_key = "<add your api key here or add env variable CORALOGIX_API_KEY>"
  #env = "<add the environment you want to work at or add env variable CORALOGIX_ENV>"
}

resource "coralogix_tco_policy" "tco_policy_1" {
  name       = "Example tco_policy from terraform 1"
  priority   = "low"
  order      = 2
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
  priority = "medium"

  order = 3

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

resource "coralogix_tco_policy" "tco_policy_3" {
  name     = "Example tco_policy from terraform 3"
  priority = "high"

  order = 1
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

resource "coralogix_tco_policy" "tco_policy_4" {
  name     = "Example tco_policy from terraform 4"
  priority = "high"

  order = 4
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