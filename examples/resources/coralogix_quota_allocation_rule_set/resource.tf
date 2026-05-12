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

resource "coralogix_quota_allocation_rule_set" "example" {
  rules = [
    {
      entity_type  = "logs"
      allocation   = 60
      enabled      = true
      can_overflow = true
    },
    {
      entity_type  = "metrics"
      allocation   = 40
      enabled      = true
      can_overflow = false
    }
  ]
}
