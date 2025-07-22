terraform {
  required_providers {
    coralogix = {
      version = "~> 2.0"
      source  = "coralogix/coralogix"
    }
  }
}

provider "coralogix" {
  #api_key = "<add your api key here or add env variable CORALOGIX_API_KEY>"
  #env = "<add the environment you want to work at or add env variable CORALOGIX_ENV>"
}

resource "coralogix_group_v2" "example" {
  name        = "example-group"
  description = "This is an example group created using Terraform"
  external_id = "example-group-id"
  roles       = [
    {
      id = "1"
    },
    {
      id = "2"
    }
  ]
  scope = {
    filters = {
      subsystems = [
        {
          filter_type = "exact"
          term        = "purchases"
        },
        {
          filter_type = "exact"
          term        = "signups"
        }
      ]
    }
  }
}

resource "coralogix_user" "example" {
  user_name = "example@coralogix.com"
  name      = {
    given_name  = "private-name"
    family_name = "last-name"
  }
}


resource "coralogix_scope" "test" {
  display_name       = "ExampleScope"
  default_expression = "<v1>true"
  filters            = [
    {
      entity_type = "logs"
      expression  = "<v1>(subsystemName == 'purchases') || (subsystemName == 'signups')"
    }
  ]
}

resource "coralogix_group_attachment" "example" {
  group_id = coralogix_group_v2.example.id
  user_ids = [coralogix_user.example.id]
}