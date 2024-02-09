terraform {
  required_providers {
    coralogix = {
      version = "~> 1.10"
      source = "coralogix/coralogix"
    }
  }
}

provider "coralogix" {
  org_key = "<ORG_KEY>"
  domain  = "<DOMAIN>"
}

resource "coralogix_api_key" "example" {
  name  = "My SCIM KEY"
  owner = {
    team_id : "<TEAM_ID>"
  }   
  active = false
  hashed = false
  roles = ["SCIM"]
}

data "coralogix_api_key" "same_key_by_id" {
  id = coralogix_api_key.example.id
}

