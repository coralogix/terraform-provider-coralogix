package coralogix

import (
	"context"
	"fmt"
	"os"
	"testing"

	"terraform-provider-coralogix/coralogix/clientset"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

var userResourceName = "coralogix_user.test"
var teamID = os.Getenv("TEST_TEAM_ID")

func TestAccCoralogixResourceUser(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckUserDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceUser(teamID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(userResourceName, "id"),
					resource.TestCheckResourceAttr(userResourceName, "user_name", "test@coralogix.com"),
					resource.TestCheckResourceAttr(userResourceName, "name.given_name", "Test"),
					resource.TestCheckResourceAttr(userResourceName, "name.family_name", "User"),
				),
			},
			{
				ResourceName:      userResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccCheckUserDestroy(s *terraform.State) error {
	client := testAccProvider.Meta().(*clientset.ClientSet).Users()

	ctx := context.TODO()

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "coralogix_user" {
			continue
		}

		resp, err := client.GetUser(ctx, teamID, rs.Primary.ID)
		if err == nil {
			if *resp.ID == rs.Primary.ID {
				return fmt.Errorf("user still exists: %s", rs.Primary.ID)
			}
		}
	}

	return nil
}

func testAccCoralogixResourceUser(teamID string) string {
	return fmt.Sprintf(`
	resource "coralogix_user" "test" {
	  team_id = "%s"
	  user_name = "test@coralogix.com"
	  name = {
		given_name = "Test"
		family_name = "User"
      }
	}
`, teamID)
}
