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
	"fmt"
	"os"
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/internal/ephemeralteam"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

var (
	archiveMetricsResourceName = "coralogix_archive_metrics.test"
	archiveMetricsBucket       = os.Getenv("ARCHIVE_METRICS_BUCKET")
)

func TestAccCoralogixResourceResourceArchiveMetrics(t *testing.T) {
	if archiveMetricsBucket == "" {
		t.Skip("ARCHIVE_METRICS_BUCKET must be set for this acceptance test")
	}
	// Archive-metrics settings are a team-wide singleton; run inside an
	// ephemeral team when the org key is available. The shared-team cleanup
	// pre-check and destroy check use the environment API key, so they only
	// apply on the shared-team path.
	providerConfig := ephemeralteam.ProviderConfig(t)
	usingEphemeralTeam := providerConfig != ""
	checkDestroy := testAccCheckArchiveMetricsDestroy
	if usingEphemeralTeam {
		checkDestroy = nil
	}
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			if usingEphemeralTeam {
				testAccPreCheck(t)
			} else {
				testAccArchivePreCheck(t)
			}
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             checkDestroy,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + testAccCoralogixResourceArchiveMetrics(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(archiveMetricsResourceName, "s3.region", "eu-north-1"),
					resource.TestCheckResourceAttr(archiveMetricsResourceName, "s3.bucket", archiveMetricsBucket),
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
	return fmt.Sprintf(`resource "coralogix_archive_metrics" "test" {
  s3 = {
    region = "eu-north-1"
    bucket = %q
  }
}
`, archiveMetricsBucket)
}
