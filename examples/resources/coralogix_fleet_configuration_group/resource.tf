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

resource "coralogix_fleet_configuration_group" "example" {
  name           = "production-collectors"
  description    = "Collector configuration group for production."
  tags           = ["production"]
  priority_order = 100

  family = {
    active            = true
    collector_version = "0.114.0"
    description       = "Default production family"
    metadata = {
      team = "observability"
    }
    remote_configuration = [
      {
        name              = "default"
        raw_configuration = <<-EOT
          receivers:
            otlp:
              protocols:
                grpc: {}
          processors:
            batch: {}
          exporters:
            nop: {}
          service:
            pipelines:
              traces:
                receivers: [otlp]
                processors: [batch]
                exporters: [nop]
        EOT
        agent_selector = {
          "cx.agent.type" = "agent"
        }
      }
    ]
  }
}
