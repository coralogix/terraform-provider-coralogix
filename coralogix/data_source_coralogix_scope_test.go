package coralogix

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccCoralogixDataSourceScopes_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceSLI() +
					testAccCoralogixResourceScopes_read(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(sliDataSourceName, "id"),
				),
			},
		},
	})
}

func testAccCoralogixResourceScopes_read() string {
	return `data "coralogix_scope" "test" {
		id = coralogix_scope.test.id
}
`
}
