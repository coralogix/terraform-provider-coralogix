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

resource "coralogix_action" action {
  is_private  = true
  source_type = "Log"
  name        = "google search action"
  url         = "https://www.google.com/search?q={{$p.selected_value}}"
}

data "coralogix_action" imported_action {
  id = coralogix_action.action.id
}