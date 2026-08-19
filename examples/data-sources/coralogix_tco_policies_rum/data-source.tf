data "coralogix_tco_policies_rum" "data_tco_policies" {
  depends_on = [coralogix_tco_policies_rum.tco_policies]
}
