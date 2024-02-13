package coralogix

import (
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

var (
	customRoleResourceName = "coralogix_custom_role.test"
)

func TestCustomRole(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testCustomRoleResource(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(customRoleResourceName, "name", "Test Custom Role"),
					resource.TestCheckResourceAttr(customRoleResourceName, "description", "This role is created with terraform!"),
					resource.TestCheckResourceAttr(customRoleResourceName, "parent_role", "Standard User"),
				),
			},
			{
				ResourceName:      customRoleResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testCustomRoleUpdateResource(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(customRoleResourceName, "name", "Test Custom Role Renamed"),
					resource.TestCheckResourceAttr(customRoleResourceName, "description", "This role is renamed with terraform!"),
					resource.TestCheckResourceAttr(customRoleResourceName, "parent_role", "Standard User"),
				),
			},
		},
	})
}

func testCustomRoleResource() string {
	return strings.Replace(`resource "coralogix_custom_role" "test" {
  name  = "Test Custom Role"
  description = "This role is created with terraform!"
  parent_role = "Standard User"
  permissions = ["spans.events2metrics:UpdateConfig"]
  team_id =  "<TEAM_ID>"
}
`, "<TEAM_ID>", targetTeam, 1)
}

func testCustomRoleUpdateResource() string {
	return strings.Replace(`resource "coralogix_custom_role" "test" {
  name  = "Test Custom Role Renamed"
  description = "This role is renamed with terraform!"
  parent_role = "Standard User"
  permissions = ["spans.events2metrics:UpdateConfig", "spans.events2metrics:ReadConfig"]
  team_id =  "<TEAM_ID>"
}
`, "<TEAM_ID>", targetTeam, 1)
}
