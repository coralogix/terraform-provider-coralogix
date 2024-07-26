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

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

var alertDataSourceName = "data." + alertResourceName

func TestAccCoralogixDataSourceAlert_basic(t *testing.T) {
	alert := standardAlertTestParams{
		alertCommonTestParams: *getRandomAlert(),
		groupBy:               []string{"EventType"},
		occurrencesThreshold:  acctest.RandIntRange(1, 1000),
		timeWindow:            selectRandomlyFromSlice(alertValidTimeFrames),
		deadmanRatio:          selectRandomlyFromSlice(alertValidDeadmanRatioValues),
	}

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertStandard(&alert) +
					testAccCoralogixDataSourceAlert_read(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(alertDataSourceName, "name", alert.name),
				),
			},
		},
	})
}

func testAccCoralogixDataSourceAlert_read() string {
	return `data "coralogix_alert" "test" {
	id = coralogix_alert.test.id
}
`
}
