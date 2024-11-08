data "coralogix_tco_policies_logs" "data_tco_policies" {
  depends_on = [coralogix_tco_policies_logs.tco_policies]
}
