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
  name = "overlay-example-collectors"

  family = {
    active = true
    remote_configuration = [
      {
        name              = "otel-agent"
        raw_configuration = <<-EOT
          receivers:
            otlp:
              protocols:
                grpc: {}
          exporters:
            debug: {}
          service:
            pipelines:
              logs:
                receivers: [otlp]
                exporters: [debug]
        EOT
        agent_selector = {
          "cx.agent.type" = "agent"
        }
      }
    ]
  }
}

resource "coralogix_fleet_configuration_overlay" "debug_verbosity" {
  name           = "debug-exporter-verbosity"
  description    = "Raise debug exporter verbosity on the agent configuration."
  tags           = ["debugging"]
  priority_order = 10

  raw_overlay_configuration = <<-EOT
    exporters:
      debug:
        verbosity: detailed
  EOT

  # Remote configuration IDs, not configuration group IDs.
  targets = [
    coralogix_fleet_configuration_group.example.family.remote_configuration[0].id,
  ]

  # Set to true to apply the overlay to its targets.
  active = false
}
