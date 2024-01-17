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

resource "coralogix_sli" "example" {
  name            = "coralogix_sli_example"
  slo_percentage  = 80
  service_name    = "service_name"
  threshold_value = 3
}

data "coralogix_sli" "data_example" {
  id = coralogix_sli.example.id
  service_name = coralogix_sli.example.service_name
}