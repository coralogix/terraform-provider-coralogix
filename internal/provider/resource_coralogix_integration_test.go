// Copyright 2024 Coralogix Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package provider

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

var integrationWithoutSensitiveDataName = "aws-metrics-collector"
var integrationWithSensitiveDataName = "gcp-metrics-collector"
var testRoleArn = os.Getenv("AWS_TEST_ROLE")

func TestAccCoralogixResourceIntegrationWithoutSensitiveData(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckIntegrationDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceIntegrationWithoutSensitiveData(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("coralogix_integration.no_sensitive_data_test", "id"),
					resource.TestCheckResourceAttr("coralogix_integration.no_sensitive_data_test", "integration_key", integrationWithoutSensitiveDataName),
					resource.TestCheckResourceAttr("coralogix_integration.no_sensitive_data_test", "version", "0.1.0"),
					resource.TestCheckResourceAttr("coralogix_integration.no_sensitive_data_test", "parameters.ApplicationName", "cxsdk"),
					resource.TestCheckResourceAttr("coralogix_integration.no_sensitive_data_test", "parameters.SubsystemName", integrationWithoutSensitiveDataName),
					resource.TestCheckResourceAttr("coralogix_integration.no_sensitive_data_test", "parameters.AwsRegion", "eu-north-1"),
					resource.TestCheckResourceAttr("coralogix_integration.no_sensitive_data_test", "parameters.WithAggregations", "false"),
					resource.TestCheckResourceAttr("coralogix_integration.no_sensitive_data_test", "parameters.EnrichWithTags", "true"),
					resource.TestCheckResourceAttr("coralogix_integration.no_sensitive_data_test", "parameters.IntegrationName", "sdk-integration-no-sensitive-data-setup"),
					resource.TestCheckResourceAttr("coralogix_integration.no_sensitive_data_test", "parameters.AwsRoleArn", testRoleArn),
				),
			},
		},
	})
}

func TestAccCoralogixResourceIntegrationWithVariablesWithoutSensitiveData(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckIntegrationDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceIntegrationVariablesWithoutSensitiveData(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("coralogix_integration.variable_test", "id"),
					resource.TestCheckResourceAttr("coralogix_integration.variable_test", "integration_key", integrationWithoutSensitiveDataName),
					resource.TestCheckResourceAttr("coralogix_integration.variable_test", "version", "0.1.0"),
					resource.TestCheckResourceAttr("coralogix_integration.variable_test", "parameters.ApplicationName", "cxsdk"),
					resource.TestCheckResourceAttr("coralogix_integration.variable_test", "parameters.SubsystemName", integrationWithoutSensitiveDataName),
					resource.TestCheckResourceAttr("coralogix_integration.variable_test", "parameters.AwsRegion", "eu-north-1"),
					resource.TestCheckResourceAttr("coralogix_integration.variable_test", "parameters.WithAggregations", "false"),
					resource.TestCheckResourceAttr("coralogix_integration.variable_test", "parameters.EnrichWithTags", "true"),
					resource.TestCheckResourceAttr("coralogix_integration.variable_test", "parameters.IntegrationName", "sdk-integration-no-sensitive-data-setup"),
					resource.TestCheckResourceAttr("coralogix_integration.variable_test", "parameters.AwsRoleArn", testRoleArn),
				),
			},
		},
	})
}

func TestAccCoralogixResourceIntegrationWithSensitiveData(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckIntegrationDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceIntegrationWithSensitiveData(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("coralogix_integration.sensitive_data_test", "id"),
					resource.TestCheckResourceAttr("coralogix_integration.sensitive_data_test", "integration_key", integrationWithSensitiveDataName),
					resource.TestCheckResourceAttr("coralogix_integration.sensitive_data_test", "version", "1.0.0"),
					resource.TestCheckResourceAttr("coralogix_integration.sensitive_data_test", "parameters.ApplicationName", "cxsdk"),
					resource.TestCheckResourceAttr("coralogix_integration.sensitive_data_test", "parameters.SubsystemName", integrationWithSensitiveDataName),
					resource.TestCheckResourceAttr("coralogix_integration.sensitive_data_test", "parameters.IntegrationName", "sdk-integration-with-sensitive-data-setup"),
				),
			},
		},
	})
}

func testAccCheckIntegrationDestroy(s *terraform.State) error {
	client := testAccProvider.Meta().(*clientset.ClientSet).Integrations()
	ctx := context.TODO()

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "coralogix_integration" {
			continue
		}

		_, _, err := client.IntegrationServiceGetDeployedIntegration(ctx, rs.Primary.ID).Execute()
		if err == nil {
			return fmt.Errorf("Integration still exists: %v, %v", rs.Primary.ID, err)
		}
	}
	return nil
}

func testAccCoralogixResourceIntegrationWithoutSensitiveData() string {
	return fmt.Sprintf(`resource "coralogix_integration" "no_sensitive_data_test" {
integration_key = "%v"
version = "0.1.0"
# Note that the attribute casing is important here
parameters = {
	ApplicationName = "cxsdk"
	SubsystemName = "%v"
	MetricNamespaces = [
		"AWS/S3",
		"AWS/ECR",
		"AWS/EFS",
		"AWS/RDS",
		"AWS/ApplicationELB",
		"AWS/Lambda",
		"AWS/Backup",
		"AWS/EBS",
		"AWS/SNS",
		"AWS/EC2"
	]
	AwsRoleArn = "%v"
	IntegrationName = "sdk-integration-no-sensitive-data-setup"
	AwsRegion = "eu-north-1"
	WithAggregations = false
	EnrichWithTags = true
}
}
	`, integrationWithoutSensitiveDataName, integrationWithoutSensitiveDataName, testRoleArn)
}

func testAccCoralogixResourceIntegrationWithSensitiveData() string {
	return fmt.Sprintf("%40s", `resource "coralogix_integration" "sensitive_data_test" {
		integration_key = "gcp-metrics-collector"
		version         = "1.0.0"
		# Note that the attribute casing is important here
		parameters = {
			ApplicationName = "cxsdk"
			SubsystemName   = "gcp-metrics-collector"
			IntegrationName   = "sdk-integration-with-sensitive-data-setup"
			MetricPrefixes    = ["appengine.googleapis.com","cloudfunctions.googleapis.com","cloudkms.googleapis.com","cloudsql.googleapis.com","compute.googleapis.com","container.googleapis.com","datastream.googleapis.com","firestore.googleapis.com","loadbalancing.googleapis.com","network.googleapis.com","run.googleapis.com","storage.googleapis.com"]
			ServiceAccountKey = "{\"type\": \"service_account\",\"project_id\": \"redacted\",\"private_key_id\": \"redacted\",\"private_key\": \"-----BEGIN PRIVATE KEY-----\\redacted\",\"client_email\": \"redacted@redacted.iam.gserviceaccount.com\",\"client_id\": \"redacted\",\"auth_uri\": \"https://accounts.google.com/o/oauth2/auth\",\"token_uri\": \"https://oauth2.googleapis.com/token\",\"auth_provider_x509_cert_url\": \"https://www.googleapis.com/oauth2/v1/certs\",\"client_x509_cert_url\": \"https://www.googleapis.com/robot/v1/metadata/x509/redacted%40assen-project.iam.gserviceaccount.com\",\"universe_domain\": \"googleapis.com\"}"
		}
	}`)
}

func testAccCoralogixResourceIntegrationVariablesWithoutSensitiveData() string {
	return fmt.Sprintf(`resource "coralogix_integration" "variable_test" {
integration_key = "%v"
version = "0.1.0"
# Note that the attribute casing is important here
parameters = {
	ApplicationName = "cxsdk"
	SubsystemName = "%v"
	MetricNamespaces = var.metrics_to_collect
	AwsRoleArn = "%v"
	IntegrationName = "sdk-integration-no-sensitive-data-setup"
	AwsRegion = "eu-north-1"
	WithAggregations = false
	EnrichWithTags = true
}
}


variable "metrics_to_collect" {
	description = "metric namespaces to collect"
	type = list(string)
	default = [
	  "AWS/RDS",
	  "AWS/SQS",
	  "AWS/S3",
	  "AWS/AmazonMQ",
	  "AWS/Lambda",
	  "AWS/Transfer"
	]
  }
	`, integrationWithoutSensitiveDataName, integrationWithoutSensitiveDataName, testRoleArn)
}
