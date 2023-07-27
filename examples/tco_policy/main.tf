terraform {
  required_providers {
    coralogix = {
      version = "~> 1.7"
      source  = "coralogix/coralogix"
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
  order      = 1
  severities = ["debug", "verbose", "info"]
  applications = {
    rule_type = "starts with"
    names        = ["prod"]
  }
  subsystems = {
    rule_type = "is"
    names = ["mobile", "web"]
  }
  archive_retention_id = "e1c980d0-c910-4c54-8326-67f3cf95645a"
}

resource "coralogix_tco_policy" "tco_policy_2" {
  name     = "Example tco_policy from terraform 2"
  priority = "medium"
  order = 2

  severities = ["error", "warning", "critical"]
  applications = {
    rule_type = "starts with"
    names        = ["prod"]
  }
  subsystems = {
    rule_type = "is"
    names = ["mobile", "web"]
  }
}

resource "coralogix_tco_policy" "tco_policy_3" {
  name     = "Example tco_policy from terraform 3"
  priority = "high"
  order = 3

  severities = ["error", "warning", "critical"]
  applications = {
    rule_type = "starts with"
    names        = ["prod"]
  }
  subsystems = {
    rule_type = "is"
    names = ["mobile", "web"]
  }
}

resource "coralogix_tco_policy" "tco_policy_4" {
  name     = "Example tco_policy from terraform 4"
  priority = "high"
  order = 4

  severities = ["error", "warning", "critical"]
  applications = {
    rule_type = "starts with"
    names        = ["prod"]
  }
  subsystems = {
    rule_type = "is"
    names = ["mobile", "web"]
  }
}
