package coralogix

import (
	"fmt"
	"testing"

	"terraform-provider-coralogix/coralogix/clientset"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

var teamResourceName = "coralogix_scope.test"

func TestAccCoralogixResourceScope(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckUserDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceScope(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(userResourceName, "id"),
					resource.TestCheckResourceAttr(userResourceName, "name", "example"),
					resource.TestCheckResourceAttr(userResourceName, "retention", "1"),
					resource.TestCheckResourceAttr(userResourceName, "daily_quota", "0.025"),
				),
			},
			{
				ResourceName:      userResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccCoralogixResourceUpdatedScope(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(userResourceName, "id"),
					resource.TestCheckResourceAttr(userResourceName, "name", "updated_example"),
					resource.TestCheckTypeSetElemAttr(userResourceName, "team_admins_emails.*", "example@coralogix.com"),
					resource.TestCheckResourceAttr(userResourceName, "retention", "1"),
					resource.TestCheckResourceAttr(userResourceName, "daily_quota", "0.1"),
				),
			},
		},
	})
}

func testAccCheckScopeDestroy(s *terraform.State) error {
	client := testAccProvider.Meta().(*clientset.ClientSet).Scopes()

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "coralogix_scope" {
			continue
		}

		resp, err := client.Get(ctx, rs.Primary.ID)
		if err == nil && resp != nil {
			return fmt.Errorf("Scopes still exists and active: %s", rs.Primary.ID)
		}
	}

	return nil
}

func testAccCoralogixResourceScope() string {
	return `resource "coralogix_scope" { "example" {
 		name                    = "example"
 		retention               = 1
 		daily_quota             = 0.025
	}
	`
}

func testAccCoralogixResourceUpdatedScope() string {
	return `resource "coralogix_scope" { "example" {
 		name                    = "updated_example
 		team_admins_emails      = ["example@coralogix.com"]
 		retention               = 1
 		daily_quota             = 0.1
	}
	`
}
