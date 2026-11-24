terraform {
  required_providers {
    coralogix = {
      source = "coralogix/coralogix"
    }
  }
}

resource "coralogix_product_analytics_settings" "example" {
  enabled = true
}
