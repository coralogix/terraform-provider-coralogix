data "coralogix_data_enrichments" "imported_standard_enrichments" {
  id = "geo_ip,suspicious_ip"
}

data "coralogix_data_enrichments" "imported_custom_enrichment" {
  id = "12345" // a custom enrichment ID
}
