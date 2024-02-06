terraform {
  required_providers {
    coralogix = {
      version = "~> 1.10"
      source = "coralogix/coralogix"
    }
  }
}

provider "coralogix" {
  #api_key = "<add your api key here or add env variable CORALOGIX_API_KEY>"
  #env = "<add the environment you want to work at or add env variable CORALOGIX_ENV>"
}

resource "coralogix_api_key" "example" {
  name  = "example key 3"
  owner = {
    team_id : 123
  }
  active = true
  hashed = true
  roles = ["SCIM"]
}


