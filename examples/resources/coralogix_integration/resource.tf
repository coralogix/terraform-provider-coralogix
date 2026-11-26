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

resource "coralogix_integration" "example" {
  integration_key = "aws-metrics-collector"
  version         = "0.1.0"
  # Note that the attribute casing is important here
  parameters = {
    ApplicationName  = "cxsdk"
    SubsystemName    = "aws-metrics-collector"
    MetricNamespaces = ["AWS/S3"]
    AwsRoleArn       = "arn:aws:iam::123456789012:role/example-role"
    IntegrationName  = "sdk-integration-setup"
    AwsRegion        = "eu-north-1"
    WithAggregations = false
    EnrichWithTags   = true
  }
}

resource "coralogix_integration" "aws-resource-catalog" {
  integration_key = "aws-resource-catalog"
  version         = "0.1.0"

  parameters = {
    IntegrationName = "aws-resource-catalog"
    AwsRoleArn      = "arn:aws:iam::123456789012:role/S3Access"
  }
}

# A parameter that carries a secret goes in parameters_wo rather than parameters: Terraform
# sends it to Coralogix and never writes it to state. Use an ephemeral variable for the value.
# Increment the matching version to send a rotated secret. A key is in parameters or in
# parameters_wo, not in both.
#
# variable "integration_secret" {
#   type      = string
#   sensitive = true
#   ephemeral = true
# }
#
# resource "coralogix_integration" "with_secret" {
#   integration_key = "example-integration"
#   version         = "0.1.0"
#   parameters = {
#     IntegrationName = "example"
#   }
#   parameters_wo = {
#     SecretParameter = var.integration_secret
#   }
#   parameters_wo_versions = {
#     SecretParameter = 1
#   }
# }
