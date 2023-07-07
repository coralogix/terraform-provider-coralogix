package coralogix

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

var actionDataSourceName = "data." + actionResourceName

func TestAccCoralogixDataSourceAction(t *testing.T) {
	action := actionTestParams{
		name:         acctest.RandomWithPrefix("tf-acc-test"),
		url:          "https://www.google.com/",
		sourceType:   selectRandomlyFromSlice(actionValidSourceTypes),
		applications: []string{acctest.RandomWithPrefix("tf-acc-test")},
		subsystems:   []string{acctest.RandomWithPrefix("tf-acc-test")},
		isPrivate:    true,
		isHidden:     false,
	}

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckActionDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAction(action) +
					testAccCoralogixAction_read(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(actionDataSourceName, "name", action.name),
				),
			},
		},
	})
}

func testAccCoralogixAction_read() string {
	return `data "coralogix_action" "test" {
             id = coralogix_action.test.id
			}
`
}
