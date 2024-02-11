terraform {
  required_providers {
    coralogix = {
      version = "~> 1.8"
      source  = "coralogix/coralogix"
    }
  }
}

provider "coralogix" {
  #api_key = "<add your api key here or add env variable CORALOGIX_API_KEY>"
  #env = "<add the environment you want to work at or add env variable CORALOGIX_ENV>"
}

resource "coralogix_user" "test" {
  team_id   = "team-id"
  user_name = "test3@coralogix.com"
  active    = true
}

resource "coralogix_group" "test" {
  team_id      = "team-id"
  display_name = "example"
  role         = "Read Only"
  members      = [coralogix_user.test.id]
}