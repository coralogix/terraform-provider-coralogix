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

resource "coralogix_dashboards_folder" "example" {
  name     = "example"
}

resource "coralogix_dashboards_folder" "example_2" {
  name     = "example2"
  parent_id = coralogix_dashboards_folder.example.id
}