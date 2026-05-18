terraform {
  required_providers {
    coralogix = {
      version = "~> 3.0"
      source  = "coralogix/coralogix"
    }
  }
}

provider "coralogix" {
  #api_key = "<add your api key here or add env variable CORALOGIX_API_KEY>"
  #env = "<add the environment you want to work at or add env variable CORALOGIX_ENV>"
}

resource "coralogix_data_enrichments" "geo_ip_enrichment" {
  geo_ip = {
    fields = [{
      name                = "coralogix.metadata.sdkId"
      enriched_field_name = "sdkId_enriched"
      selected_columns    = ["city", "country"]
      }, {
      name                = "coralogix.metadata.IPAddress"
      enriched_field_name = "ip_addr_enriched"
    }]
  }
}

resource "coralogix_data_enrichments" "suspicious_ip_enrichment" {
  suspicious_ip = {
    fields = [{
      name                = "coralogix.metadata.sdkId"
      enriched_field_name = "sdkId_enriched"
      selected_columns    = ["classification", "threat_score"]
    }]
  }
}

resource "coralogix_data_enrichments" "aws_enrichment" {
  aws = {
    fields = [{
      name                = "coralogix.metadata.aws_resource_id"
      enriched_field_name = "aws_resource_enriched"
      resource            = "ec2"
      selected_columns    = ["resourceId", "accountId"]
    }]
  }
}

resource "coralogix_data_enrichments" "custom_enrichment" {
  custom = {
    custom_enrichment_data = {
      name        = "my-custom-enrichment"
      description = "description"
      contents    = "local_id,instance_type\nfoo1,t2.micro\nfoo2,t2.micro\nfoo3,t2.micro\nbar1,m3.large\n"
    }
    fields = [{
      name                = "coralogix.metadata.IPAddress"
      enriched_field_name = "ip_addr_custom_enriched"
      selected_columns    = ["instance_type"]
    }]
  }
}

resource "coralogix_data_enrichments" "test" {
  custom = {
    custom_enrichment_data = {
      name        = "custom enrichment data"
      description = "description"
      contents    = file("../coralogix_data_enrichments/date-to-day-of-the-week.csv")
    }
    fields = []
  }
}
