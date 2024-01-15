terraform {
  required_providers {
    coralogix = {
      version = "~> 1.10"
      source  = "coralogix/coralogix"
    }
  }
}

provider "coralogix" {
  #api_key = "<add your api key here or add env variable CORALOGIX_API_KEY>"
  #env = "<add the environment you want to work at or add env variable CORALOGIX_ENV>"
}

resource "coralogix_team" "example" {
  name                    = "example"
  team_admins_emails      = ["example@coralogix.com"]
  retention               = 1
}

resource "coralogix_team" "example-2" {
  name                    = "example-2"
  team_admins_emails      = ["example@coralogix.com"]
  retention               = 1
}

resource "coralogix_moving_quota" "example" {
  source_team_id            = coralogix_team.example.id
  destination_team_id       = coralogix_team.example-2.id
  desired_source_team_quota = 85
}
