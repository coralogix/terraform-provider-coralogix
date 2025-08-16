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

resource "coralogix_views_folder" "example_view_folder" {
  name        = "Example View Folder"
}

resource "coralogix_view" "example_view" {
  name        = "Example View"
  time_selection = {
    quick_selection = {
      seconds = 3600
    }
  }
  folder_id = coralogix_views_folder.example_view_folder.id
}