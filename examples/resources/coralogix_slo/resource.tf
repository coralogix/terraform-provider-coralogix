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

resource "coralogix_slo" "example" {
  name        = "coralogix_slo_example"
  description = "description"
  labels = {
    "key1" = "value1"
    "key2" = "value2"
  }
  target_threshold_percentage = 30
  sli = [
    {
      good_events = {
        query = "query"
      }
      total_events = {
        query = "query"
      }
      group_by_labels = ["label1", "label2"]
    }
  ]

  window = {
    slo_time_frame = "7_days"
  }
}