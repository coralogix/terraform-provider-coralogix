terraform {
  required_providers {
    coralogix = {
      version = "~> 3.0"
      source  = "coralogix/coralogix"
    }
  }
}

resource "coralogix_view" "example" {
  name = "Logs view"
  time_selection = {
    quick_selection = {
      seconds = 3600
    }
  }
}
