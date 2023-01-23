terraform {
  required_providers {
    coralogix = {
      version = "~> 1.3"
      source  = "coralogix/coralogix"
    }
  }
}

provider "coralogix" {
  #api_key = "<add your api key here or add env variable CORALOGIX_API_KEY>"
  #env = "<add the environment you want to work at or add env variable CORALOGIX_ENV>"
}

resource "coralogix_tco_policy" "tco_policy" {
  name     = "Example tco_policy from terraform"
  priority = "medium"
  severities = ["debug", "verbose", "info"]
  application_name {
    starts_with = true
    rule = "prod"
  }
  subsystem_name {
    is = true
    rules = ["mobile", "web"]
  }
}