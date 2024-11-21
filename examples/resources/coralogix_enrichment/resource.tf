resource "coralogix_enrichment" geo_ip_enrichment {
  geo_ip {
    fields {
      name = "coralogix.metadata.sdkId"
    }
    fields {
      name = "coralogix.metadata.IPAddress"
    }
  }
}

resource "coralogix_enrichment" suspicious_ip_enrichment {
  suspicious_ip {
    fields {
      name = "coralogix.metadata.sdkId"
    }
  }
}

resource "coralogix_enrichment" custom_enrichment {
  custom {
    custom_enrichment_id = coralogix_data_set.data_set.id
     fields {
       name = "coralogix.metadata.IPAddress"
     }
  }
}

resource "coralogix_data_set" data_set {
  name         = "custom enrichment data"
  description  = "description"
  file_content = file("../data_set/date-to-day-of-the-week.csv")
}