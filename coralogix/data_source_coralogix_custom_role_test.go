package coralogix

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

var customRoleSourceName = "data." + customRoleResourceName

func TestAccCoralogixDataSourceCustomRole(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testCustomRoleResource() +
					testCustomRoleResource_read(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(customRoleSourceName, "name", "Test Custom Role"),
					resource.TestCheckResourceAttr(customRoleSourceName, "description", "This role is created with terraform!"),
					resource.TestCheckResourceAttr(customRoleSourceName, "parent_role", "Standard User"),
				),
			},
		},
	})
}

func testCustomRoleResource_read() string {
	return `data "coralogix_custom_role" "test" {
		  id = coralogix_custom_role.test.id
}
`
}
