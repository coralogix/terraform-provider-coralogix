terraform {
  required_providers {
    coralogix = {
      version = "~> 2.0"
      source  = "coralogix/coralogix"
    }
  }
}

provider "coralogix" {
  #api_key = "<add your api key here or add env variable CORALOGIX_API_KEY>"
  #env = "<add the environment you want to work at or add env variable CORALOGIX_ENV>"
}

resource "coralogix_connector" "example" {
  id          = "custom_id"
  name        = "example-connector"
  description = "example connector"
  type        = "slack"

  connector_config = {
    fields = [
      {
        field_name = "webhook_url"
        template   = "<template>"
      }
    ]
  }

  connector_overrides = [
    {
      entity_type = "alert"
      fields      = [
        {
          field_name = "alert_id"
          value      = "<alert_id>"
        }
      ]
    }
  ]
}