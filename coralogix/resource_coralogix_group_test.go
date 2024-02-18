package coralogix

import (
	"context"
	"fmt"
	"testing"

	"terraform-provider-coralogix/coralogix/clientset"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

var groupResourceName = "coralogix_group.test"

func TestAccCoralogixResourceGroup(t *testing.T) {
	userName := randUserName()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckGroupDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceGroup(userName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(groupResourceName, "id"),
					resource.TestCheckResourceAttr(groupResourceName, "display_name", "example"),
					resource.TestCheckResourceAttr(groupResourceName, "role", "Read Only"),
					resource.TestCheckResourceAttr(groupResourceName, "members.#", "1"),
					resource.TestCheckResourceAttrPair(groupResourceName, "members.0", "coralogix_user.test", "id"),
				),
			},
			{
				ResourceName:      groupResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccCheckGroupDestroy(s *terraform.State) error {
	client := testAccProvider.Meta().(*clientset.ClientSet).Groups()

	ctx := context.TODO()

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "coralogix_group" {
			continue
		}

		resp, err := client.GetGroup(ctx, rs.Primary.ID)
		if err == nil {
			if resp.ID == rs.Primary.ID {
				return fmt.Errorf("group still exists: %s", rs.Primary.ID)
			}
		}
	}

	return nil
}

func testAccCoralogixResourceGroup(userName string) string {
	return fmt.Sprintf(`
	resource "coralogix_user" "test" {
	  user_name = "%s"
	}

	resource "coralogix_group" "test" {
	  display_name = "example"
      role         = "Read Only"
      members      = [coralogix_user.test.id]
	}
`, userName)
}
