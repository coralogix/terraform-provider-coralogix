terraform {
  required_providers {
    coralogix = {
      version = "~> 1.3"
      source  = "locally/debug/coralogix"
    }
  }
}

provider "coralogix" {
  #api_key = "<add your api key here or add env variable CORALOGIX_API_KEY>"
  #env = "<add the environment you want to work at or add env variable CORALOGIX_ENV>"
}

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

resource "coralogix_enrichment" custom_enrichment2 {
  custom {
    custom_enrichment_id = coralogix_data_set.data_set2.id
    fields {
      name = "coralogix.metadata.IPAddress"
    }
  }
}

resource "coralogix_data_set" data_set {
  name        = "custom enrichment data"
  description = "description"
  uploaded_file {
    path = "./date-to-day-of-the-week.csv"
  }
}

resource "coralogix_data_set" data_set2 {
  name        = "custom enrichment data 2"
  description = "description"
  uploaded_file {
    path = "./date-to-day-of-the-week.csv"
  }
}

data "coralogix_enrichment" "imported_enrichment" {
  id = coralogix_enrichment.geo_ip_enrichment.id
}