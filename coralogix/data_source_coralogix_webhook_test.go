package coralogix

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAccCoralogixDataSourceWebhook_basic(t *testing.T) {
	w := getRandomWebhook()
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceSlackWebhook(w) +
					testAccCoralogixDataSourceWebhook_read(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.coralogix_webhook.test", "name", w.name),
					resource.TestCheckResourceAttr("data.coralogix_webhook.test", "slack.0.url", w.url),
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
