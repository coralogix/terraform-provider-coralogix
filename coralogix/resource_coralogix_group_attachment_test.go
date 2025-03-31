package coralogix

import (
	"context"
	"fmt"
	"testing"

	"terraform-provider-coralogix/coralogix/clientset"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

var groupAttachmentResourceName = "coralogix_group_attachment.test"

func TestAccCoralogixResourceGroupAttachment(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { TestAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckGroupDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceGroupAttachment(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(groupAttachmentResourceName, "id"),
					resource.TestCheckResourceAttr(groupResourceName, "display_name", "example"),
					resource.TestCheckResourceAttr(groupResourceName, "role", "Read Only"),
					resource.TestCheckResourceAttr(groupResourceName, "members.#", "1"),
					resource.TestCheckResourceAttrPair(groupResourceName, "members.0", "coralogix_user.test", "id"),
					resource.TestCheckResourceAttrPair(groupResourceName, "scope_id", "coralogix_scope.test", "id"),
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

func testAccCheckGroupAttachmentDestroy(s *terraform.State) error {
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

func testAccCoralogixResourceGroupAttachment() string {
	return fmt.Sprintf(`
	resource "coralogix_user" "test" {
		user_name = "user_name"
	}
	
	data "coralogix_group" "example" {
       display_name = "ReadOnlyUsers"
    }

	resource "coralogix_group_attachment" "test" {
		group_id = data.coralogix_group.test.id
		user_ids = [coralogix_user.test.id]
	}
`)
}
