package provider

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	teamGroups "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/team_groups_management_service"
	terraform2 "github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

var userNameToAttach = randUserName()
var membersBeforeRemove int

func TestAccCoralogixResourceGroupAttachment(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceGroupAttachment(userNameToAttach),
				Check:  testCheckUserInGroup,
			},
			{
				Config: testAccCoralogixResourceGroupAttachmentDeleted(userNameToAttach),
				Check:  testCheckUserWasRemovedFromGroup,
			},
		},
	})
}

func testCheckUserInGroup(s *terraform.State) error {
	client, err := testAccTeamGroupsClient()
	if err != nil {
		return err
	}
	ctx := context.TODO()

	groupId, userId, err := groupAndUserIDsFromState(s)
	if err != nil {
		return err
	}

	userIDs, err := testAccListGroupUserIDs(ctx, client, groupId)
	if err != nil {
		return err
	}

	memberFound := false
	for _, id := range userIDs {
		if id == userId {
			memberFound = true
			break
		}
	}

	membersBeforeRemove = len(userIDs)

	if !memberFound {
		return fmt.Errorf("user not found in group")
	}

	return nil
}

func testCheckUserWasRemovedFromGroup(s *terraform.State) error {
	client, err := testAccTeamGroupsClient()
	if err != nil {
		return err
	}
	ctx := context.TODO()

	groupId, userId, err := groupAndUserIDsFromState(s)
	if err != nil {
		return err
	}

	userIDs, err := testAccListGroupUserIDs(ctx, client, groupId)
	if err != nil {
		return err
	}

	for _, id := range userIDs {
		if id == userId {
			return fmt.Errorf("user still in group")
		}
	}

	if membersBeforeRemove != len(userIDs)+1 {
		return fmt.Errorf("accpected number of members to be %d, but got %d", membersBeforeRemove-1, len(userIDs))
	}

	return nil
}

func testAccTeamGroupsClient() (*teamGroups.TeamGroupsManagementServiceAPIService, error) {
	rc := terraform2.ResourceConfig{}
	_ = testAccProvider.Configure(context.Background(), &rc)
	meta := testAccProvider.Meta()
	if meta == nil {
		return nil, fmt.Errorf("provider meta is nil")
	}
	return meta.(*clientset.ClientSet).TeamGroups(), nil
}

func groupAndUserIDsFromState(s *terraform.State) (string, string, error) {
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
		return "", "", fmt.Errorf("group not found in state")
	}
	if userId == "" {
		return "", "", fmt.Errorf("user not found in state")
	}
	return groupId, userId, nil
}

func testAccListGroupUserIDs(ctx context.Context, client *teamGroups.TeamGroupsManagementServiceAPIService, groupID string) ([]string, error) {
	id, err := strconv.ParseInt(groupID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid group ID %q: %w", groupID, err)
	}
	resp, httpResp, err := client.GroupsMgmtServiceGetGroupUsers(ctx, id).Execute()
	if err != nil {
		return nil, fmt.Errorf("error getting group users: %s", utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResp, err), "Read", nil))
	}
	if resp == nil {
		return nil, fmt.Errorf("group users not found")
	}
	var userIDs []string
	for _, user := range resp.Users {
		if user.UserId != nil {
			userIDs = append(userIDs, *user.UserId)
		}
	}
	return userIDs, nil
}

func testAccCoralogixResourceGroupAttachment(userName string) string {
	return fmt.Sprintf(`
	resource "coralogix_user" "test" {
		user_name = "%s"
	}
	
	data "coralogix_group" "test" {
       display_name = "ReadOnlyUsers"
    }

	resource "coralogix_group_attachment" "test" {
		group_id = data.coralogix_group.test.id
		user_ids = [coralogix_user.test.id]
		depends_on = [coralogix_user.test]
	}
`, userName)
}

func testAccCoralogixResourceGroupAttachmentDeleted(userName string) string {
	return fmt.Sprintf(`
	resource "coralogix_user" "test" {
		user_name = "%s"
	}
	
	data "coralogix_group" "test" {
       display_name = "ReadOnlyUsers"
    }
`, userName)
}
