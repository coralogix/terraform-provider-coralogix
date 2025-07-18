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

var sloV2DataSourceName = "data." + sloV2ResourceName

func TestAccCoralogixDataSourceSLOV2_basic(t *testing.T) {
	t.Skip("SLOv2 is deactivated for now")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { TestAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixSLOV2RequestBased() +
					testAccCoralogixResourceSLOV2_read(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(sloV2DataSourceName, "id"),
				),
			},
		},
	})
}

func testAccCoralogixResourceSLOV2_read() string {
	return `data "coralogix_slo_v2" "test" {
		id = coralogix_slo_v2.test.id
}
`
}
