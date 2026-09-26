terraform {
  required_providers {
    coralogix = { source = "coralogix/coralogix" }
  }
}

resource "coralogix_connector" "http" {
  id          = "@SUFFIX@"
  name        = "@SUFFIX@"
  type        = "generic_https"
  description = "IaC codegen POC live check"
  connector_config = {
    fields = [
      { field_name = "url", value = "https://example.com/iac-codegen-poc" },
      { field_name = "method", value = "post" },
    ]
  }
}

resource "coralogix_global_router" "example" {
  name        = "@SUFFIX@"
  description = "@DESCRIPTION@"
  routing_labels = {
    environment = "@SUFFIX@"
  }
  entity_labels = { owner = "iac-codegen-poc" }
  rules = [
    {
      entity_type = "alerts"
      name        = "p1"
      condition   = "alertDef.priority == \"P1\""
      targets     = [{ connector_id = coralogix_connector.http.id }]
    },
  ]
  fallback_targets = [
    {
      entity_type = "alerts"
      target      = { connector_id = coralogix_connector.http.id }
    },
  ]
}
