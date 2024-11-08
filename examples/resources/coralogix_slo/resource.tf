resource "coralogix_slo" "example" {
  name            = "coralogix_slo_example"
  service_name    = "service_name"
  description     = "description"
  target_percentage = 30
  type            = "error"
  period          = "7_days"
}

resource "coralogix_slo" "example_2" {
  name            = "coralogix_slo_example"
  service_name    = "service_name"
  description     = "description"
  target_percentage = 30
  type            = "latency"
  threshold_microseconds = 1000000
  threshold_symbol_type = "greater"
  period          = "7_days"
  filters = [
    {
      field = "severity"
      compare_type = "is"
      field_values = ["error", "warning"]
    },
  ]
}