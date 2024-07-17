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

resource "coralogix_scope" "example" {
  display_name       = "ExampleScope"
  team_id            = "4013254"
  default_expression = "subsystemName == 'newsletter'"
  filters            = [
    {
      entity_type = "logs"
      expression  = "(subsystemName == 'purchases') || (subsystemName == 'signups')"
    }
  ]
}

data "coralogix_scope" "data_example" {
  id = coralogix_scope.example.id
}