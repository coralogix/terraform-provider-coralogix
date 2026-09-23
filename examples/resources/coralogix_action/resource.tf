terraform {
  required_providers {
    coralogix = {
      version = "~> 3.0"
      source  = "coralogix/coralogix"
    }
  }
}

provider "coralogix" {
  #api_key = "<add your api key here or add env variable CORALOGIX_API_KEY>"
  #env = "<add the environment you want to work at or add env variable CORALOGIX_ENV>"
}

resource "coralogix_action" "action" {
  is_private  = false
  source_type = "Log"
  name        = "google search action"
  url         = "https://www.google.com/search?q={{$p.selected_value}}"
  description = "Search the selected value on Google."
  # DPXL expressions must include the `<v1> ` version prefix.
  dpxl_filter = "<v1> $d.severity == 'ERROR'"
  url_fields = [
    { name = "selected_value", required = true },
    { name = "region", required = false },
  ]
}