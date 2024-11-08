data "coralogix_tco_policies_traces" "tco_policies_data" {
  depends_on = [coralogix_tco_policies_traces.tco_policies]
}