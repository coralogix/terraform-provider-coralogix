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

resource "coralogix_custom_role" "example" {
  name        = "Example custom role"
  description = "This role is created with terraform!"
  parent_role = "Standard User"
  permissions = ["spans.events2metrics:UpdateConfig"]
}

resource "coralogix_user" "example" {
  user_name = "example@coralogix.com"
  name = {
    given_name  = "example"
    family_name = "example"
  }
}

# The data source reads one user by id or by user_name: exactly one of the two is set.
data "coralogix_user" "by_id" {
  id = coralogix_user.example.id
}

data "coralogix_user" "by_user_name" {
  user_name = coralogix_user.example.user_name
}

resource "coralogix_group" "members" {
  display_name = "example-members"
  role         = coralogix_custom_role.example.name
  members      = [data.coralogix_user.by_id.id]
}
