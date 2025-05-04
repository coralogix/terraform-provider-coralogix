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

resource "coralogix_global_router" "example" {
  id          = "global_router_example"
  name        = "global router example"
  description = "global router example"
  entity_type = "alerts"
  rules       = [
    {
      name = "rule-name"
      condition = "alertDef.priority == \"P1\""
      targets = [
        {
          connector_id   = "<some_connector_id>"
          preset_id      = "<some_preset_id>"
        }
      ]
    }
  ]
}