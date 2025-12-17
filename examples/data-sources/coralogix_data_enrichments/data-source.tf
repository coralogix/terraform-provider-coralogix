data "coralogix_data_enrichments" "imported_enrichment" {
  id = "geo_ip,sus_ip"
}

data "coralogix_data_enrichments" "imported_enrichment" {
  id = "12345" // a custom enrichments id
}