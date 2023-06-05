terraform {
  required_providers {
    coralogix = {
#      version = "~> 1.5"
       source = "coralogix.com/coralogix/coralogix"
    }
  }
}

provider "coralogix" {
  #api_key = "<add your api key here or add env variable CORALOGIX_API_KEY>"
  #env = "aaa"
}

resource "coralogix_events2metric" "logs2metric" {
  name        = "logs2metricExample"
  description = "logs2metric from coralogix terraform provider"
  logs_query = {
    lucene       = "remote_addr_enriched:/.*/"
    applications = ["filter:startsWith:nginx"] //change here for existing applications from your account
    severities   = ["Debug"]
  }

  metric_fields = [
    {
      target_base_metric_name = "method"
      source_field            = "method" //change here for existing source field from your account
    },
    {
      target_base_metric_name = "geo_point"
      source_field            = "remote_addr_geoip.location_geopoint"
      aggregations = {
        max = {
          enable = false
        }
        min = {
          enable = false
        }
        avg = {
          enable = true
        }
      }
    }
  ]

  //change here for existing source field from your account

  metric_labels = [
    {
      target_label = "Status"
      source_field = "status" //change here for existing source field from your account
    },
    {
      target_label = "Path"
      source_field = "http_referer" //change here for existing source field from your account
    },
  ]

  permutations = {
    limit = 20000
  }
}


#resource "coralogix_events2metric" "spans2metric" {
#  name        = "spans2metricExample"
#  description = "spans2metric from coralogix terraform provider"
#
#  spans_query {
#    lucene       = "remote_addr_enriched:/.*/"
#    applications = ["filter:startsWith:nginx"] //change here for existing applications from your account
#    actions      = ["action-name"]
#    services     = ["service-name"]
#  }
#
#  metric_fields {
#    target_base_metric_name = "method"
#    source_field            = "method" //change here for existing source field from your account
#  }
#  metric_fields {
#    target_base_metric_name = "geo_point"
#    source_field            = "remote_addr_geoip.location_geopoint"
#    //change here for existing source field from your account
#  }
#
#  metric_labels {
#    target_label = "Status"
#    source_field = "status" //change here for existing source field from your account
#  }
#  metric_labels {
#    target_label = "Path"
#    source_field = "http_referer" //change here for existing source field from your account
#  }
#
#  permutations {
#    limit = 20000
#  }
#}
#
#data "coralogix_events2metric" "imported_logs2metric" {
#  id = coralogix_events2metric.logs2metric.id
#}