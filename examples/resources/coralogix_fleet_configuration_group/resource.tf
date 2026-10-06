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

# Raw family: you supply the OpenTelemetry Collector YAML for each remote configuration.
resource "coralogix_fleet_configuration_group" "example" {
  name           = "production-collectors"
  description    = "Collector configuration group for production."
  tags           = ["production"]
  priority_order = 100

  family = {
    active      = true
    description = "Default production family"
    raw = {
      collector_version = "0.114.0"
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
}

# Preset family: Coralogix renders the remote configurations from a configuration template.
resource "coralogix_fleet_configuration_group" "kubernetes" {
  name = "kubernetes-collectors"

  family = {
    preset = {
      chart_name    = "otel_integration"
      chart_version = "0.0.200"
      metadata = {
        ClusterName         = "production"
        KubernetesRunningOn = "eks"
      }
      observability_features = jsonencode({
        logs    = { enabled = true }
        metrics = { enabled = true }
      })
    }
  }
}
