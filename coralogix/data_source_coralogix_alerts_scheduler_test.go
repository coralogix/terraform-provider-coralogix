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

package coralogix

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

var alertsSchedulerDataSourceName = "data." + alertsSchedulerResourceName

func TestAccCoralogixDataSourceAlertsScheduler(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAlertsSchedulerDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertsScheduler() +
					testAccCoralogixAlertsScheduler_read(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(alertsSchedulerDataSourceName, "name", "example"),
				),
			},
		},
	})
}

func testAccCoralogixAlertsScheduler_read() string {
	return `data "coralogix_alerts_scheduler" "test" {
             id = coralogix_alerts_scheduler.test.id
			}
`
}
