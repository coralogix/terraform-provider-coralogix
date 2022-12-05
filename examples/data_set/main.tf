terraform {
  required_providers {
    coralogix = {
      version = "~> 1.3"
      source  = "coralogix/coralogix"
    }
  }
}

provider "coralogix" {
  #api_key = "<add your api key here or add env variable CORALOGIX_API_KEY>"
  #env = "<add the environment you want to work at or add env variable CORALOGIX_ENV>"
}

resource "coralogix_data_set" data_set {
  name        = "custom enrichment data"
  description = "description"
  file_content = "Date,day of week\n7/30/21,Friday\n7/31/21,Saturday\n8/1/21,Sunday\n8/2/21,Monday\n8/4/21,Wednesday\n8/5/21,Thursday\n8/6/21,Friday\n"
}

resource "coralogix_data_set" data_set2 {
  name        = "custom enrichment data 2"
  description = "description"
  uploaded_file {
    path = "./date-to-day-of-the-week.csv"
  }
}