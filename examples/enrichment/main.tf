terraform {
  required_providers {
    coralogix = {
      version = "~> 1.3"
      source  = "coralogix/coralogix"
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

resource "coralogix_data_set" data_set {
  name        = "custom enrichment data"
  description = "description"
  file_content = "Date,day of week\n7/30/21,Friday\n7/31/21,Saturday\n8/1/21,Sunday\n8/2/21,Monday\n8/4/21,Wednesday\n8/5/21,Thursday\n8/6/21,Friday\n"
}

data "coralogix_enrichment" "imported_enrichment" {
  id = coralogix_enrichment.geo_ip_enrichment.id
}