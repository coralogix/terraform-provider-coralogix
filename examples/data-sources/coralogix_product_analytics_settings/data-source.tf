terraform {
  required_providers {
    coralogix = {
      source = "coralogix/coralogix"
    }
  }
}

data "coralogix_product_analytics_settings" "example" {
}
