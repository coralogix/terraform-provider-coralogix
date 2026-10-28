terraform {
  required_providers {
    coralogix = {
      source = "coralogix/coralogix"
    }
  }
}

data "coralogix_thing" "example" {
  id = "<id>"
}
