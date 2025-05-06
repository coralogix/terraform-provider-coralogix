resource "coralogix_global_router" "example" {
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