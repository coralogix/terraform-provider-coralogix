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

resource "coralogix_archive_metrics" "example" {
  s3 = {
    region = "eu-north-1"
    bucket = "<your-bucket-name>"
  }
}

# The archive has exactly one destination: `s3`, `ibm` or `gcs`. To archive into Google Cloud
# Storage, replace the `s3` block above with:
#
# resource "coralogix_archive_metrics" "example" {
#   gcs = {
#     bucket = "<your-bucket-name>"
#   }
# }
