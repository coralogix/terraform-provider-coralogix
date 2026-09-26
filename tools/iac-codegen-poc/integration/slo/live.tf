terraform {
  required_providers {
    coralogix = { source = "coralogix/coralogix" }
  }
}

# A request-based SLO with ownership tags: the generated types (sli and window
# are wrappers) and the handwritten rules (ownership dimensions) in one SLO.
resource "coralogix_slo_v2" "example" {
  name                        = "@SUFFIX@"
  description                 = "@DESCRIPTION@"
  target_threshold_percentage = 99.5
  labels                      = { owner = "iac-codegen-poc" }
  sli = {
    request_based_metric_sli = {
      # The server accepts only a 1-minute range in SLO queries. The same
      # queries as the provider's acceptance test.
      good_events  = { query = "avg(rate(cpu_usage_seconds_total[1m])) by (instance)" }
      total_events = { query = "avg(rate(cpu_usage_seconds_total[1m])) by (instance)" }
    }
  }
  window = {
    slo_time_frame = "7_days"
  }
  ownership_tags = {
    team = { static_values = ["iac-codegen-poc"] }
  }
}
