// Copyright 2024 Coralogix Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccCoralogixDataSourceWebhook_basic(t *testing.T) {
	w := &customWebhookTestFields{
		webhookTestFields: *getRandomWebhook(),
		method:            "post",
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceCustomWebhook(w) +
					testAccCoralogixDataSourceWebhook_readByID(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.coralogix_webhook.test_by_id", "name", w.name),
					resource.TestCheckResourceAttr("data.coralogix_webhook.test_by_id", "custom.url", w.url),
				),
			},
			{
				Config: testAccCoralogixResourceCustomWebhook(w) +
					testAccCoralogixDataSourceWebhook_readByName(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.coralogix_webhook.test_by_name", "name", w.name),
					resource.TestCheckResourceAttr("data.coralogix_webhook.test_by_name", "custom.url", w.url),
				),
			},
		},
	})
}

func testAccCoralogixDataSourceWebhook_readByID() string {
	return `data "coralogix_webhook" "test_by_id" {
	id = coralogix_webhook.test.id
}
`
}

func testAccCoralogixDataSourceWebhook_readByName() string {
	return `data "coralogix_webhook" "test_by_name" {
	name = coralogix_webhook.test.name
}
`
}
