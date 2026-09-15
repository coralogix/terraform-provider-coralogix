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
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

var sloV2DataSourceName = "data." + sloV2ResourceName

func TestAccCoralogixDataSourceSLOV2_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
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

// The data source inherits the resource schema by delegation and reuses
// flattenSLOV2, so the APM branch is only exercised here.
func TestAccCoralogixDataSourceSLOV2_apm(t *testing.T) {
	service := os.Getenv(sloV2APMServiceEnvVar)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccSLOV2APMPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixSLOV2APMSLI(service, `error_config = {}`, "") +
					testAccCoralogixResourceSLOV2_read(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(sloV2DataSourceName, "id"),
					resource.TestCheckResourceAttr(sloV2DataSourceName, "product_type", "apm"),
					resource.TestCheckResourceAttr(sloV2DataSourceName, "sli.apm_sli.services.0", service),
					resource.TestCheckResourceAttr(sloV2DataSourceName, "apm_sli_metadata.services.0", service),
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
