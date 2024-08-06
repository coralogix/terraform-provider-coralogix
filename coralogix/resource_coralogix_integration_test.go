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
					resource.TestCheckResourceAttr("coralogix_integration.test", "integration_key", "aws-metrics-collector"),
					resource.TestCheckResourceAttr("coralogix_integration.test", "version", "0.1.0"),
					resource.TestCheckResourceAttr("coralogix_integration.test", "parameters.ApplicationName", "cxsdk"),
					resource.TestCheckResourceAttr("coralogix_integration.test", "parameters.SubsystemName", "aws-metrics-collector"),
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

		resp, err := client.Get(ctx, &integrations.GetIntegrationDetailsRequest{
			Id:                     wrapperspb.String(integrationResourceName),
			IncludeTestingRevision: wrapperspb.Bool(true),
		})
		if err == nil && resp != nil {
			details, _ := integrationDetail(resp, rs.Primary.ID)
			if details != nil {
				return fmt.Errorf("Integration still exists: %v", rs.Primary.ID)
			}
		}
	}
	return nil
}

func testAccCoralogixResourceIntegration() string {
	return fmt.Sprintf(`resource "coralogix_integration" "test" {
		integration_key = "aws-metrics-collector"
		version = "0.1.0"
	    # Note that the attribute casing is important here
		parameters = {
		  ApplicationName = "cxsdk"
		  SubsystemName = "aws-metrics-collector"
		  MetricNamespaces = ["AWS/S3"]
		  AwsRoleArn = "%v"
		  IntegrationName = "sdk-integration-setup"
		  AwsRegion = "eu-north-1"
		  WithAggregations = false
		  EnrichWithTags = true
	  }
	}
	`, testRoleArn)
}
