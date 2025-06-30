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

resource "coralogix_view" "example_view" {
  name        = "Example View"
  time_selection = {
    custom_selection = {
      from_time = "2023-01-01T00:00:00Z"
      to_time   = "2023-01-02T00:00:00Z"
    }
  }
  search_query = {
    query = "error OR warning"

  }
  filters = {
    filters = [
      {
        name = "severity"
        selected_values = {
          "ERROR" = true
          "WARNING" = true
        }
      },
      {
        name = "application"
        selected_values = {
          "my-app" = true
          "another-app" = true
        }
      }
    ]
  }
}