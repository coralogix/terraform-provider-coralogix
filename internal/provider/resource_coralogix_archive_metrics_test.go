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

var (
	archiveMetricsResourceName = "coralogix_archive_metrics.test"
)

func TestAccCoralogixResourceResourceArchiveMetrics(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceArchiveMetrics(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(archiveMetricsResourceName, "s3.region", "eu-north-1"),
					resource.TestCheckResourceAttr(archiveMetricsResourceName, "s3.bucket", "coralogix-c4c-eu2-prometheus-data"),
				),
			},
			{
				ResourceName:      archiveMetricsResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccCoralogixResourceArchiveMetrics() string {
	return `resource "coralogix_archive_metrics" "test" {
  s3 = {
    region = "eu-north-1"
    bucket = "coralogix-c4c-eu2-prometheus-data"
  }
}
`
}
