resource "coralogix_archive_metrics" example {
  s3 = {
    region = "eu-north-1"
    bucket = "coralogix-c4c-eu2-prometheus-data"
  }
}
