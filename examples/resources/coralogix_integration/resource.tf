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