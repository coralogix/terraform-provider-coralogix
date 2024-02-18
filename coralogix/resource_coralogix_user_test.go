package coralogix

import (
	"context"
	"fmt"
	"testing"

	"terraform-provider-coralogix/coralogix/clientset"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

var userResourceName = "coralogix_user.test"

func TestAccCoralogixResourceUser(t *testing.T) {
	userName := randUserName()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckUserDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceUser(userName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(userResourceName, "id"),
					resource.TestCheckResourceAttr(userResourceName, "user_name", userName),
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

		resp, err := client.GetUser(ctx, rs.Primary.ID)
		if err == nil && resp != nil {
			if *resp.ID == rs.Primary.ID && resp.Active {
				return fmt.Errorf("user still exists and active: %s", rs.Primary.ID)
			}
		}
	}

	return nil
}

func randUserName() string {
	return "test@coralogix.com"
}

func testAccCoralogixResourceUser(userName string) string {
	return fmt.Sprintf(`
	resource "coralogix_user" "test" {
	  user_name = "%s"
	  name = {
		given_name = "Test"
		family_name = "User"
      }
	}
`, userName)
}
