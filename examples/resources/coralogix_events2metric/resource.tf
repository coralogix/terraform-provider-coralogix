resource "coralogix_events2metric" "logs2metric" {
  name        = "logs2metricExample"
  description = "logs2metric from coralogix terraform provider"
  logs_query  = {
    lucene       = "remote_addr_enriched:/.*/"
    applications = ["filter:startsWith:nginx"] //change here for existing applications from your account
    severities   = ["Debug"]
  }

  metric_fields = {
    method = {
      source_field = "method"
    },
    geo_point = {
      source_field = "remote_addr_geoip.location_geopoint"
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
  }

  metric_labels = {
    Status = "status"
    Path   = "http_referer"
  }

  permutations = {
    limit = 20000
  }
}

resource "coralogix_events2metric" "spans2metric" {
  name        = "spans2metricExample"
  description = "spans2metric from coralogix terraform provider"

  spans_query = {
    lucene       = "remote_addr_enriched:/.*/"
    applications = ["filter:startsWith:nginx"] //change here for existing applications from your account
    actions      = ["action-name"]
    services     = ["service-name"]
  }

  metric_fields = {
    method = {
      source_field = "method"
    },
    geo_point = {
      source_field = "remote_addr_geoip.location_geopoint"
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
  }

  metric_labels = {
    Status = "status"
    Path   = "http_referer"
  }

  permutations = {
    limit = 20000
  }
}