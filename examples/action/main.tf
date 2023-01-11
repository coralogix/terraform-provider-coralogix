terraform {
  required_providers {
    coralogix = {
      version = "~> 1.3"
      source  = "locally/debug/coralogix"
    }
  }
}

provider "coralogix" {
  #api_key = "<add your api key here or add env variable CORALOGIX_API_KEY>"
  #env = "<add the environment you want to work at or add env variable CORALOGIX_ENV>"
}

resource "coralogix_action" "action" {
  is_private = true
  source_type = "Log"
  name = "aaa"
  url = "https://ng-api-http.eu2.coralogix.com/opendashboards"
}