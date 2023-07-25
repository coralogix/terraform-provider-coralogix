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
  name       = "Example tco_policy from terraform 1"
  priority   = "Low"
  order      = 1
  source_type = "Logs"
  severities = ["debug", "verbose", "info"]
  applications = {
    names        = ["prod"]
  }
  subsystems = {
    rule_type = "Includes"
    names = ["mobile", "web"]
  }
  archive_retention_id = "e1c980d0-c910-4c54-8326-67f3cf95645a"
}

resource "coralogix_tco_policy" "tco_policy_2" {
  name     = "Example tco_policy from terraform 2"
  priority = "Medium"
  source_type = "Logs"
  order = 2

  severities = ["error", "warning", "critical"]
  applications = {
    names        = ["prod"]
  }
  subsystems = {
    rule_type = "Starts With"
    names = ["mobile", "web"]
  }
}

resource "coralogix_tco_policy" "tco_policy_3" {
  name     = "Example tco_policy from terraform 3"
  priority = "High"
  source_type = "Logs"
  order = 3
  #  currently, for controlling the policies order they have to be created by the order you want them to be.
  #  for this purpose, defining dependency via the 'order' field can control their creation order.

  severities = ["error", "warning", "critical"]
  applications = {
    names        = ["prod"]
  }
  subsystems = {
    rule_type = "Is Not"
    names = ["mobile", "web"]
  }
}

resource "coralogix_tco_policy" "tco_policy_4" {
  name     = "Example tco_policy from terraform 4"
  priority = "High"
  source_type = "Logs"
  order = 4
  #  currently, for controlling the policies order they have to be created by the order you want them to be.
  #  for this purpose, defining dependency via the 'order' field can control their creation order.

  severities = ["error", "warning", "critical"]
  applications = {
    names        = ["prod"]
  }
  subsystems = {
    rule_type = "Includes"
    names = ["mobile", "web"]
  }
}

#resource "coralogix_tco_policy" "tco_policy_option1" {
#  name     = "Example tco_policy from terraform"
#  priority = "high"
#  order = 4
#
#
#  application_name {
#    starts_with = true
#    rule        = "prod"
#  }
#  subsystem_name {
#    is    = true
#    rules = ["mobile", "web"]
#  }
#
#  logs{
#    severities = ["error", "warning", "critical"]
#  }
#  //or
#  traces{
#    actions = {
#      //...
#    }
#  }
#}

#resource "coralogix_tco_policy" "tco_policy_option1" {
#  name     = "Example tco_policy from terraform"
#  priority = "high"
#  order = 4
#
#
#  application_name {
#    starts_with = true
#    rule        = "prod"
#  }
#  subsystem_name {
#    is    = true
#    rules = ["mobile", "web"]
#  }
#
#  source_type = "logs/traces"
#
#  severities = ["error", "warning", "critical"]
#  //or
#  actions = {
#    //...
#  }
#}
#
#resource "coralogix_tco_policy_logs/traces" "tco_policy_option2" {
#  name     = "Example tco_policy from terraform"
#  priority = "high"
#  order = 4
#
#  application_name {
#    starts_with = true
#    rule        = "prod"
#  }
#  subsystem_name {
#    is    = true
#    rules = ["mobile", "web"]
#  }
#
#  severities = ["error", "warning", "critical"]
#}
#
#
#resource "coralogix_tco_policies_logs/traces" "tco_policy_option3" {
#  policies = [
#    {
#      name     = "Example tco_policy from terraform"
#      priority = "high"
#
#      application_name = {
#        starts_with = true
#        rule = "prod"
#      }
#      subsystem_name = {
#        is = true
#        rules = ["mobile", "web"]
#      }
#      severities = ["error", "warning", "critical"]
#    },
#    {
#      name     = "Example tco_policy from terraform"
#      priority = "high"
#      order    = 4
#
#      application_name ={
#        starts_with = true
#        rule = "prod"
#      }
#      subsystem_name = {
#        is = true
#        rules = ["mobile", "web"]
#    }
#      severities = ["error", "warning", "critical"]
#    }
#    ]
#}