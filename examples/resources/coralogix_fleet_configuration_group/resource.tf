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
# The required metadata keys and the available observability features depend on chart_name and chart_version.
resource "coralogix_fleet_configuration_group" "kubernetes" {
  name = "kubernetes-collectors"

  family = {
    preset = {
      chart_name    = "otel_integration"
      chart_version = "0.0.353"
      metadata = {
        ClusterName         = "production"
        KubernetesRunningOn = "openshift"
      }
      observability_features = jsonencode({
        apm = {
          enabled      = true
          ebpf         = false
          profiling    = { enabled = false }
          sampling     = {}
          span_metrics = { enabled = true, histogram_buckets = [], transform_statements = [] }
        }
        coralogix_operator = false
        fleet_management   = { enabled = false, remote_config = false }
        kubernetes_events  = true
        logs               = { enabled = false }
        metrics = {
          cluster          = false
          collector        = false
          host             = { enabled = false }
          kubelet          = false
          kubernetes_extra = { enabled = false, scrape_all = false }
          statsd           = false
          target_allocator = false
        }
        reduce_resources_attributes = true
        resource_catalog            = false
      })
    }
  }
}
