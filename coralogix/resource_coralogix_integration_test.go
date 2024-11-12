package coralogix

import (
	"context"
	"fmt"
	"os"
	"testing"

	"terraform-provider-coralogix/coralogix/clientset"
	integrations "terraform-provider-coralogix/coralogix/clientset/grpc/integrations"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

var integrationResourceName = "aws-metrics-collector"
var testRoleArn = os.Getenv("AWS_TEST_ROLE")

func TestAccCoralogixResourceIntegration(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckIntegrationDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceIntegration(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("coralogix_integration.test", "id"),
					resource.TestCheckResourceAttr("coralogix_integration.test", "integration_key", integrationResourceName),
					resource.TestCheckResourceAttr("coralogix_integration.test", "version", "0.1.0"),
					resource.TestCheckResourceAttr("coralogix_integration.test", "parameters.ApplicationName", "cxsdk"),
					resource.TestCheckResourceAttr("coralogix_integration.test", "parameters.SubsystemName", integrationResourceName),
					resource.TestCheckResourceAttr("coralogix_integration.test", "parameters.AwsRegion", "eu-north-1"),
					resource.TestCheckResourceAttr("coralogix_integration.test", "parameters.WithAggregations", "false"),
					resource.TestCheckResourceAttr("coralogix_integration.test", "parameters.EnrichWithTags", "true"),
					resource.TestCheckResourceAttr("coralogix_integration.test", "parameters.IntegrationName", "sdk-integration-setup"),
					resource.TestCheckResourceAttr("coralogix_integration.test", "parameters.AwsRoleArn", testRoleArn),
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

		_, err := client.Get(ctx, &integrations.GetDeployedIntegrationRequest{
			IntegrationId: wrapperspb.String(rs.Primary.ID),
		})
		if err == nil {
			return fmt.Errorf("Integration still exists: %v", rs.Primary.ID)
		}
	}
	return nil
}

func testAccCoralogixResourceIntegration() string {
	return fmt.Sprintf(`resource "coralogix_integration" "test" {
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
		  IntegrationName = "sdk-integration-setup"
		  AwsRegion = "eu-north-1"
		  WithAggregations = false
		  EnrichWithTags = true
	  }
	}
	`, integrationResourceName, integrationResourceName, testRoleArn)
}
