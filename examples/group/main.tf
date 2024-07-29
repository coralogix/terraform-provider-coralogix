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

resource "coralogix_custom_role" "example" {
  name  = "Example custom role"
  description = "This role is created with terraform!"
  parent_role = "Standard User"
  permissions = ["spans.events2metrics:UpdateConfig"]
}

resource "coralogix_user" "example" {
  user_name = "example@coralogix.com"
  name      = {
    given_name  = "example"
    family_name = "example"
  }
}

resource "coralogix_scope" "example" {
  display_name       = "ExampleScope"
  default_expression = "<v1> true"
  filters            = [
    {
      entity_type = "logs"
      expression  = "<v1>(subsystemName == 'purchases') || (subsystemName == 'signups')"
    }
  ]
}

resource "coralogix_group" "example" {
  display_name = "example"
  role         = coralogix_custom_role.example.name
  members      = [coralogix_user.example.id]
  scope_id     = data.coralogix_scope.example.id
}

