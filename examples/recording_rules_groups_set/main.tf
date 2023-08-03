terraform {
  required_providers {
    coralogix = {
      version = "~> 1.5"
      source  = "coralogix/coralogix"
    }
  }
}

provider "coralogix" {
  #api_key = "<add your api key here or add env variable CORALOGIX_API_KEY>"
  #env = "<add the environment you want to work at or add env variable CORALOGIX_ENV>"
}

resource "coralogix_recording_rules_groups_set" "recording_rules_group" {
  yaml_content = file("./rule-group-set.yaml")
}

resource "coralogix_recording_rules_groups_set" "recording_rules_groups_set_explicit" {
  name = "Name"
  group {
    name     = "Foo"
    interval = 180
    rule {
      record = "ts3db_live_ingester_write_latency:3m"
      expr   = "sum(rate(ts3db_live_ingester_write_latency_seconds_count{CX_LEVEL=\"staging\",pod=~\"ts3db-live-ingester.*\"}[2m])) by (pod)"
    }
    rule {
      record = "job:http_requests_total:sum"
      expr   = "sum(rate(http_requests_total[5m])) by (job)"
    }
  }
  group {
    name     = "Bar"
    interval = 180
    rule {
      record = "ts3db_live_ingester_write_latency:3m"
      expr   = "sum(rate(ts3db_live_ingester_write_latency_seconds_count{CX_LEVEL=\"staging\",pod=~\"ts3db-live-ingester.*\"}[2m])) by (pod)"
    }
    rule {
      record = "job:http_requests_total:sum"
      expr   = "sum(rate(http_requests_total[5m])) by (job)"
    }
  }
}