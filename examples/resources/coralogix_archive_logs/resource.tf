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

resource "coralogix_archive_logs" "example" {
  bucket = "<your-bucket-name>"
}
# In a GCP environment (US3) the archive is a Google Cloud Storage bucket. Give its name as
# `bucket` and its location as `region`:
#
# resource "coralogix_archive_logs" "example" {
#   bucket = "<your-gcs-bucket-name>"
#   region = "us-central1"
# }
