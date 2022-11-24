terraform {
  required_providers {
    coralogix = {
      version = "1.3.0"
      source  = "coralogix/coralogix"
    }
  }
}

provider "coralogix" {
  #api_key = "<add your api key here or add env variable CORALOGIX_API_KEY>"
  #env = "<add the environment you want to work at or add env variable CORALOGIX_ENV>"
}

resource "coralogix_logs2metric" "logs2metric" {
  name        = "name"
  description = "logs2metric from coralogix terraform provide"
  query {
    lucene       = "remote_addr_enriched:/.*/"
    applications = ["nginx"]
    severities   = ["Debug"]
  }

  metric_fields {
    target_base_metric_name = "method"
    source_field            = "method"
  }
  metric_fields {
    target_base_metric_name = "geo_point"
    source_field            = "remote_addr_geoip.location_geopoint"
  }

  metric_labels {
    target_label = "Status"
    source_field = "status"
  }
  metric_labels {
    target_label = "Path"
    source_field = "http_referer"
  }

  permutations {
    limit = 20000
  }
}

data "coralogix_logs2metric" "imported_logs2metric" {
  id = coralogix_logs2metric.logs2metric.id
}