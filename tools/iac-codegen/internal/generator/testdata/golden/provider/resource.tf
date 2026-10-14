terraform {
  required_providers {
    coralogix = {
      source = "coralogix/coralogix"
    }
  }
}

resource "coralogix_thing" "example" {
  name        = "example-name"
  description = "example-description"
  enabled     = true
  kind        = "THING_KIND_STANDARD"
  config = {
    http = {
      endpoint = "example-endpoint"
    }
  }
  spec = {
    mode   = "example-mode"
    region = "example-region"
    items = [
      {
        key        = "example-key"
        name       = "example-name"
        start_time = "2030-01-01T00:00:00Z"
      },
    ]
    targets = [
      {
        name = "example-name"
      },
    ]
  }
  destinations = ["https://a"]
  tags         = ["example-tags"]
  details = [
    {
      name = "example-name"
    },
  ]
  labels      = { key = "example-labels" }
  expire_time = "2030-01-01T00:00:00Z"
  windows     = ["2030-01-01T00:00:00Z"]
}
