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

resource "coralogix_user" "example" {
  user_name = "example1000@coralogix.com"
  team_id   = "team_id"
  name      = {
    given_name  = "example"
    family_name = "example"
  }
}

resource "coralogix_group" "example" {
  team_id      = "team_id"
  display_name = "example"
  role         = "Read Only"
  members      = [coralogix_user.example.id]
}

