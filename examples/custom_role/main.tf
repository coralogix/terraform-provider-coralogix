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

resource "coralogix_custom_role" "example" {
  name  = "My Tf Role 7"
  description = "This role is created with terraform!"
  parent_role = "Standard User"
  permissions = ["data-map:ReadMaps"]
  team_id = 563577
}


