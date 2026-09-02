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

resource "coralogix_quota_allocation_rule_set" "example" {
  rules = [
    {
      entity_type     = "logs"
      allocation      = 50
      allocation_type = "percentage"
      enabled         = true
      can_overflow    = true
    },
    {
      entity_type     = "spans"
      allocation      = 10
      allocation_type = "locked_units"
      enabled         = true
      can_overflow    = false
    },
    {
      entity_type     = "browserLogs"
      allocation      = 10
      allocation_type = "percentage"
      enabled         = true
      can_overflow    = false
    },
    {
      entity_type     = "browserLogs/v2"
      allocation      = 10
      allocation_type = "percentage"
      enabled         = true
      can_overflow    = false
    },
  ]
}
