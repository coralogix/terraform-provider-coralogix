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
        name              = "otel-agent"
        raw_configuration = file("./otel-agent.yaml")
        agent_selector = {
          "cx.agent.type" = "agent"
        }
      },
      {
        name              = "otel-cluster-collector"
        raw_configuration = file("./otel-cluster-collector.yaml")
        agent_selector = {
          "cx.agent.type" = "cluster-collector"
        }
      }
    ]
  }
}
