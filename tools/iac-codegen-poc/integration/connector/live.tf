terraform {
  required_providers {
    coralogix = { source = "coralogix/coralogix" }
  }
}

# The generated part (id, name, description, type, config_overrides) and the
# handwritten part (connector_config with a write-only field) in one
# connector. additionalHeaders is not a secret here; it only uses the
# write-only path.
resource "coralogix_connector" "example" {
  id          = "@SUFFIX@"
  name        = "@SUFFIX@"
  description = "@DESCRIPTION@"
  type        = "generic_https"
  connector_config = {
    fields = [
      { field_name = "url", value = "https://example.com/iac-codegen-poc" },
      { field_name = "method", value = "post" },
      { field_name = "additionalBodyFields", value = "{}" },
    ]
    field_values_wo          = { additionalHeaders = "{}" }
    field_values_wo_versions = { additionalHeaders = 1 }
  }
  config_overrides = [
    {
      entity_type = "alerts"
      fields      = [{ field_name = "additionalBodyFields", template = "{}" }]
    },
  ]
}
