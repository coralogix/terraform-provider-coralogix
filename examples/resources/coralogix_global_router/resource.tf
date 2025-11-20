terraform {
  required_providers {
    coralogix = {
      version = "~> 3.0"
      source  = "coralogix/coralogix"
    }
  }
}


resource "coralogix_global_router" "example" {
  # id          = "router_default" # specify your own or leave blank for custom routers. router_default refers to the "global" router
  name        = "global router example"
  description = "global router example"

  matching_routing_labels = {
    "routing.environment" = "production"
  }

  rules = [{
    entity_type = "alerts"
    name        = "rule-name"
    condition   = "alertDef.priority == \"P1\""
    targets = [{
      connector_id = "<some_connector_id>"
      preset_id    = "<some_preset_id>"
    }]
    }
  ]
}