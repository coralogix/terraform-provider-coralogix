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

var enrichmentDataSourceName = "data." + enrichmentResourceName

func TestAccCoralogixDataSourceEnrichment_basic(t *testing.T) {
	fieldName := "coralogix.metadata.sdkId"
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { TestAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckDataSetDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceGeoIpEnrichment(fieldName) +
					testAccCoralogixDataSourceEnrichment_read(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(enrichmentDataSourceName, "id"),
					resource.TestCheckResourceAttr(enrichmentDataSourceName, "geo_ip.0.fields.0.name", fieldName),
				),
			},
		},
	})
}

func testAccCoralogixDataSourceEnrichment_read() string {
	return `data "coralogix_enrichment" "test" {
	id = coralogix_enrichment.test.id
}
`
}
