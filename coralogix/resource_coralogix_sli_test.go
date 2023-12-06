package coralogix

import (
	"context"
	"fmt"
	"testing"

	"terraform-provider-coralogix/coralogix/clientset"
	sli "terraform-provider-coralogix/coralogix/clientset/grpc/sli"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

var sliResourceName = "coralogix_sli.test"

func TestAccCoralogixResourceSLIyCreate(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccSLICheckDestroy,
		Steps: []resource.TestStep{
			{
				Config:  testAccCoralogixResourceSLI(),
				Destroy: false,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(sliResourceName, "name", "coralogix_sli_example"),
					resource.TestCheckResourceAttr(sliResourceName, "slo_percentage", "80"),
					resource.TestCheckResourceAttr(sliResourceName, "service_name", "service_name"),
					resource.TestCheckResourceAttr(sliResourceName, "threshold_value", "3"),
				),
			},
		},
	},
	)
}

func testAccSLICheckDestroy(s *terraform.State) error {
	client := testAccProvider.Meta().(*clientset.ClientSet).SLIs()
	ctx := context.TODO()
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "coralogix_sli" {
			continue
		}

		if resp, err := client.GetSLIs(ctx, &sli.GetSlisRequest{ServiceName: wrapperspb.String(rs.Primary.Attributes["service_name"])}); err == nil {
			for _, sli := range resp.GetSlis() {
				if id := sli.SliId.GetValue(); id == rs.Primary.ID {
					return fmt.Errorf("sli still exists: %s", id)
				}
			}
		}
	}

	return nil
}

func testAccCoralogixResourceSLI() string {
	return `resource "coralogix_sli" "test" {
  					name            = "coralogix_sli_example"
					slo_percentage  = 80
  					service_name    = "service_name"
  					threshold_value = 3
				}
	`
}
