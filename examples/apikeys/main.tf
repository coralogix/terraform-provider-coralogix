terraform {
  required_providers {
    coralogix = {
      version = "~> 1.10"
      source = "coralogix/coralogix"
    }
  }
}

provider "coralogix" {
#  api_key = "<API_KEY>"
#  domain  = "<DOMAIN>"
}

resource "coralogix_api_key" "example" {
  name  = "My APM KEY"
  owner = {
    team_id : "4013254"
  }
  active = true
  presets = ["APM"]
  permissions = ["livetail:Read"]
}

data "coralogix_api_key" "same_key_by_id" {
  id = coralogix_api_key.example.id
}

