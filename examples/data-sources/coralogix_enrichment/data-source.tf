data "coralogix_enrichment" "imported_enrichment" {
  id = coralogix_enrichment.geo_ip_enrichment.id
}