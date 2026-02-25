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

# Example: AWS Metrics Collector (v0.1.0)
#
# Note: parameter names are case-sensitive.
resource "coralogix_integration" "aws_metrics_collector_v01" {
  integration_key = "aws-metrics-collector"
  version         = "0.1.0"

  parameters = {
    IntegrationName  = "my-aws-metrics"
    ApplicationName  = "my-app"
    SubsystemName    = "aws-metrics-collector"
    AwsRoleArn       = "arn:aws:iam::123456789012:role/example-role"
    AwsRegion        = "eu-west-1"
    MetricNamespaces = ["AWS/SQS", "AWS/Lambda", "AWS/RDS"]
    EnrichWithTags   = true
    WithAggregations = false
  }
}

# Example: AWS Metrics Collector (v0.9.x)
# Note: parameter names are case-sensitive.
resource "coralogix_integration" "aws_metrics_collector_v09" {
  integration_key = "aws-metrics-collector"
  version         = "0.9.1"

  parameters = {
    IntegrationName       = "my-aws-metrics"
    ApplicationName       = "my-app"
    SubsystemName         = "aws-metrics-collector"
    AwsRoleArn            = "arn:aws:iam::123456789012:role/example-role"
    AwsRegion             = "eu-west-1"
    MetricNamespaces      = ["AWS/SQS", "AWS/Lambda", "AWS/RDS"]
    EnrichWithTags        = true
    WithAggregations      = false
    Statistics            = ["Average", "Sum", "SampleCount", "Minimum", "Maximum"]
    DiscoverNamespaces    = false
    IncludeLinkedAccounts = false
  }
}

# Example: AWS Resource Catalog
resource "coralogix_integration" "aws_resource_catalog" {
  integration_key = "aws-resource-catalog"
  version         = "0.1.0"

  parameters = {
    IntegrationName = "my-aws-resource-catalog"
    AwsRoleArn      = "arn:aws:iam::123456789012:role/example-role"
  }
}
