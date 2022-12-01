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
    field_name = "coralogix.metadata.sdkId"
  }
}

resource "coralogix_enrichment" suspicious_ip_enrichment {
  suspicious_ip {
    field_name = "coralogix.metadata.sdkId"
  }
}

resource "coralogix_enrichment" aws_enrichment {
  aws {
    field_name    = "coralogix.metadata.sdkId"
    resource_type = "cluster"
  }
}

resource "coralogix_enrichment_data" enrichment_data {
  name         = "custom enrichment data"
  description  = "description.ssss"
  file_content = file("./date-to-day-of-the-week.csv")
}

resource "coralogix_enrichment_data" enrichment_data2 {
  name        = "custom enrichment data 2"
  description = "description"
  uploaded_file {
    path = "./date-to-day-of-the-week.csv"
  }
}

resource "coralogix_enrichment" custom_enrichment {
  custom {
    custom_enrichment_id = coralogix_enrichment_data.enrichment_data.id
    field_name           = "field name"
  }
}

resource "coralogix_enrichment" custom_enrichment2 {
  custom {
    custom_enrichment_id = coralogix_enrichment_data.enrichment_data2.id
    field_name           = "field name"
  }
}
