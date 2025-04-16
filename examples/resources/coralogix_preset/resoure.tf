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

resource "coralogix_preset" "example" {
    name        = "example"
    description = "example description"
    tags        = ["tag1", "tag2"]
    type        = "LOGS"

    # Optional
    # preset_id = "<add the preset id you want to work at or add env variable CORALOGIX_PRESET_ID>"
}