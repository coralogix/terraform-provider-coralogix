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
var userNameToAttach = randUserName()
var membersBeforeRemove int

func TestAccCoralogixResourceGroupAttachment(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { TestAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckGroupDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceGroupAttachment(userNameToAttach),
				Check:  testCheckUserInGroup,
			},
			{
				ResourceName:      groupResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccCoralogixResourceGroupAttachmentDeleted(userNameToAttach),
				Check:  testCheckUserWasRemovedFromGroup,
			},
		},
	})
}

func testCheckUserInGroup(s *terraform.State) error {
	groupsClient := testAccProvider.Meta().(*clientset.ClientSet).Groups()
	ctx := context.TODO()

	var groupId, userId string
	for _, rs := range s.RootModule().Resources {
		if rs.Type == "coralogix_group" {
			if rs.Primary.Attributes["display_name"] == "ReadOnlyUsers" {
				groupId = rs.Primary.ID
			}
		}
		if rs.Type == "coralogix_user" {
			if rs.Primary.Attributes["user_name"] == userNameToAttach {
				userId = rs.Primary.ID
			}
		}

		if groupId != "" && userId != "" {
			break
		}
	}

	if groupId == "" {
		return fmt.Errorf("group not found in state")
	}
	if userId == "" {
		return fmt.Errorf("user not found in state")
	}

	groupResp, err := groupsClient.GetGroup(ctx, groupId)
	if err != nil {
		return fmt.Errorf("error getting group: %w", err)
	}
	if groupResp == nil {
		return fmt.Errorf("group not found")
	}

	memberFound := false
	for _, member := range groupResp.Members {
		if member.Value == userId {
			memberFound = true
			break
		}
	}

	membersBeforeRemove = len(groupResp.Members)

	if !memberFound {
		return fmt.Errorf("user not found in group")
	}

	return nil
}

func testCheckUserWasRemovedFromGroup(s *terraform.State) error {
	groupsClient := testAccProvider.Meta().(*clientset.ClientSet).Groups()
	ctx := context.TODO()

	var groupId, userId string
	for _, rs := range s.RootModule().Resources {
		if rs.Type == "coralogix_group" {
			if rs.Primary.Attributes["display_name"] == "ReadOnlyUsers" {
				groupId = rs.Primary.ID
			}
		}
		if rs.Type == "coralogix_user" {
			if rs.Primary.Attributes["user_name"] == userNameToAttach {
				userId = rs.Primary.ID
			}
		}

		if groupId != "" && userId != "" {
			break
		}
	}

	if groupId == "" {
		return fmt.Errorf("group not found in state")
	}
	if userId == "" {
		return fmt.Errorf("user not found in state")
	}

	groupResp, err := groupsClient.GetGroup(ctx, groupId)
	if err != nil {
		return fmt.Errorf("error getting group: %w", err)
	}
	if groupResp == nil {
		return fmt.Errorf("group not found")
	}

	for _, member := range groupResp.Members {
		if member.Value == userId {
			return fmt.Errorf("user still in group")
		}
	}

	// check if only one member was removed
	if membersBeforeRemove != len(groupResp.Members)+1 {
		return fmt.Errorf("accpected number of members to be %d, but got %d", membersBeforeRemove-1, len(groupResp.Members))
	}

	return nil
}

func testAccCoralogixResourceGroupAttachment(userName string) string {
	return fmt.Sprintf(`
	resource "coralogix_user" "test" {
		user_name = "%s"
	}
	
	data "coralogix_group" "example" {
       display_name = "ReadOnlyUsers"
    }

	resource "coralogix_group_attachment" "test" {
		group_id = data.coralogix_group.test.id
		user_ids = [coralogix_user.test.id]
	}
`, userName)
}

func testAccCoralogixResourceGroupAttachmentDeleted(userName string) string {
	return fmt.Sprintf(`
	resource "coralogix_user" "test" {
		user_name = "%s"
	}
	
	data "coralogix_group" "example" {
       display_name = "ReadOnlyUsers"
    }
`, userName)
}
