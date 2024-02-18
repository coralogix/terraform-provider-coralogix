terraform {
  required_providers {
    coralogix = {
      version = "~> 1.10"
      source = "coralogix/coralogix"
    }
  }
}

provider "coralogix" {
#  org_key = "<ORG_KEY>"
#  domain  = "<DOMAIN>"
}

resource "coralogix_api_key" "example" {
  name  = "My SCIM KEY"
  owner = {
    team_id : "4013254"
  }
  active = true
  hashed = false
  roles = ["SCIM", "Legacy Api Key"]
}

#data "coralogix_api_key" "same_key_by_id" {
#  id = coralogix_api_key.example.id
#}

