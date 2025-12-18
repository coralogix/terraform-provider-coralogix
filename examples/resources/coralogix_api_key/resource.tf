terraform {
  required_providers {
    coralogix = {
      version = "~> 3.0"
      source  = "coralogix/coralogix"
    }
  }
}

provider "coralogix" {
  #api_key = "<add your api key here or add env variable CORALOGIX_API_KEY>"
  #env = "<add the environment you want to work at or add env variable CORALOGIX_ENV>"
}

resource "coralogix_api_key" "example" {
  name  = "My APM KEY"
  owner = {
    team_id : "4013254"
  }
  active = true
  presets = ["APM"]
  permissions = ["livetail:Read"]
  access_policy = "{ \"version\": \"2025-01-01\", \"default\": { \"permissions\": { \"data-ingest-api-keys:ReadAccessPolicy\": \"grant\", \"data-ingest-api-keys:Manage\": \"deny\", \"data-ingest-api-keys:UpdateAccessPolicy\": \"deny\", \"data-ingest-api-keys:ReadConfig\": \"grant\" } }, \"rules\": [] }"
}
