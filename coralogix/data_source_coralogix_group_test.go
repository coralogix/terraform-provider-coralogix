package coralogix

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

var groupDataSourceName = "data." + groupResourceName

func TestAccCoralogixDataSourceGroup_basic(t *testing.T) {
	userName := randUserName()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceGroup(userName) +
					testAccCoralogixDataSourceGroup_read(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(groupDataSourceName, "display_name", "example"),
				),
			},
		},
	})
}

func testAccCoralogixDataSourceGroup_read() string {
	return fmt.Sprintf(`data "coralogix_group" "test" {
	id = coralogix_group.test.id
    team_id = "%s"
}
`, teamID)
}
