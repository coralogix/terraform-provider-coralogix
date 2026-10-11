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

# A family defined by its own collector configurations.
resource "coralogix_configuration_group" "raw" {
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
      remote_configurations = [
        {
          name              = "otel-agent"
          raw_configuration = file("./otel-agent.yaml")
          agent_selector = {
            # service.version must match collector_version when it is set.
            attributes = { "cx.agent.type" = "agent", "service.version" = "0.114.0" }
          }
        },
        {
          name              = "otel-cluster-collector"
          raw_configuration = file("./otel-cluster-collector.yaml")
          agent_selector = {
            attributes = { "cx.agent.type" = "cluster-collector", "service.version" = "0.114.0" }
          }
        },
      ]
    }
  }
}

# A family generated from a configuration template.
resource "coralogix_configuration_group" "preset" {
  name = "kubernetes-collectors"

  family = {
    active = true
    preset = {
      chart_name    = "chart_name_otel_integration"
      chart_version = "0.0.353"
      metadata = {
        ClusterName         = "production"
        KubernetesRunningOn = "eks"
      }
      observability_features = jsonencode({
        kubernetes_events = true
        logs              = { enabled = true }
      })
    }
  }
}
