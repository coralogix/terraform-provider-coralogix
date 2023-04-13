terraform {
  required_providers {
    coralogix = {
      version = "~> 1.5"
      source  = "coralogix/coralogix"
    }
  }
}

provider "coralogix" {
  #api_key = "<add your api key here or add env variable CORALOGIX_API_KEY>"
  #env = "<add the environment you want to work at or add env variable CORALOGIX_ENV>"
}

resource "coralogix_data_set" data_set {
  name         = "custom enrichment data"
  description  = "description"
  file_content = file("./date-to-day-of-the-week.csv")
}

resource "coralogix_data_set" data_set2 {
  name        = "custom enrichment data 2"
  description = "description"
  uploaded_file {
    path = "./date-to-day-of-the-week.csv"
  }
}