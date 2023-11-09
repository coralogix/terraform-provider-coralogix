package coralogix

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccCoralogixDataSourceWebhook_basic(t *testing.T) {
	w := getRandomWebhook()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceSlackWebhook(w) +
					testAccCoralogixDataSourceWebhook_read(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.coralogix_webhook.test", "name", w.name),
					resource.TestCheckResourceAttr("data.coralogix_webhook.test", "slack.url", w.url),
				),
			},
		},
	})
}

func testAccCoralogixDataSourceWebhook_read() string {
	return `data "coralogix_webhook" "test" {
	id = coralogix_webhook.test.id
}
`
}
