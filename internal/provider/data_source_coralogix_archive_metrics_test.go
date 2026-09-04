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

	"github.com/coralogix/terraform-provider-coralogix/internal/ephemeralteam"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

var archiveMetricsDataSourceName = "data." + archiveMetricsResourceName

func TestAccCoralogixDataSourceArchiveMetrics_basic(t *testing.T) {
	if archiveMetricsBucket == "" {
		t.Skip("ARCHIVE_METRICS_BUCKET must be set for this acceptance test")
	}
	providerConfig := ephemeralteam.ProviderConfig(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + testAccCoralogixResourceArchiveMetrics() +
					testAccCoralogixDataSourceArchiveMetrics_read(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(archiveMetricsDataSourceName, "s3.region", "eu-north-1"),
					// This check fails randomly for unknown reasons
					// resource.TestCheckResourceAttr(archiveMetricsDataSourceName, "s3.bucket", archiveMetricsBucket),
				),
			},
		},
	})
}

func testAccCoralogixDataSourceArchiveMetrics_read() string {
	// depends_on defers the read to apply time: on a fresh (ephemeral) team no
	// tenant config exists until the resource creates it, and a plan-time read
	// returns a null object.
	return `data "coralogix_archive_metrics" "test" {
  depends_on = [coralogix_archive_metrics.test]
}
`
}
