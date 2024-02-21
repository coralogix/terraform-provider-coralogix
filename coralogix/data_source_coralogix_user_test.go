package coralogix

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

var userDataSourceName = "data." + userResourceName

func TestAccCoralogixDataSourceUser_basic(t *testing.T) {
	userName := randUserName()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceUser(userName) +
					testAccCoralogixDataSourceUser_read(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(userDataSourceName, "user_name", userName),
				),
			},
		},
	})
}

func testAccCoralogixDataSourceUser_read() string {
	return `data "coralogix_user" "test" {
	id = coralogix_user.test.id
	}
`
}
