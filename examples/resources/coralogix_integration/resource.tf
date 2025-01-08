terraform {
  required_providers {
    coralogix = {
      version = "~> 2.0"
      source  = "coralogix/coralogix"
    }
  }
}

provider "coralogix" {
  #api_key = "<add your api key here or add env variable CORALOGIX_API_KEY>"
  #env = "<add the environment you want to work at or add env variable CORALOGIX_ENV>"
}

resource "coralogix_integration" "example" {
  integration_key = "aws-metrics-collector"
  version = "0.1.0"
  # Note that the attribute casing is important here
  parameters = {
    ApplicationName = "cxsdk"
    SubsystemName = "aws-metrics-collector"
    MetricNamespaces = ["AWS/S3"]
    AwsRoleArn = "arn:aws:iam::123456789012:role/example-role"
    IntegrationName = "sdk-integration-setup"
    AwsRegion = "eu-north-1"
    WithAggregations = false
    EnrichWithTags = true
  }
}