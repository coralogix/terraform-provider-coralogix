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

resource "coralogix_tco_policy_override" "tco_policy" {
  priority         = "medium"
  severity         = "debug"
  application_name = "prod"
  subsystem_name   = "mobile"
}

resource "coralogix_tco_policy_override" "tco_policy_2" {
  priority         = "high"
  severity         = "error"
  application_name = "staging"
  subsystem_name   = "web"
}