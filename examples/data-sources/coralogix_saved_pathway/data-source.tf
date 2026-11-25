terraform {
  required_providers {
    coralogix = {
      source = "coralogix/coralogix"
    }
  }
}

data "coralogix_saved_pathway" "example" {
  id = "<id>"
}
