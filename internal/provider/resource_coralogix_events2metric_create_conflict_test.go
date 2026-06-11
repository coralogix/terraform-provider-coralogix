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
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccCoralogixResourceEvents2MetricCreateConflict(t *testing.T) {
	suffix := acctest.RandStringFromCharSet(8, "abcdefghijklmnopqrstuvwxyz")
	metricField := "tfacc_conflict_" + suffix
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckEvents2MetricDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccEvents2MetricConflictFirst(suffix, metricField),
				Check:  resource.TestCheckResourceAttrSet("coralogix_events2metric.first", "id"),
			},
			{
				Config:      testAccEvents2MetricConflictBoth(suffix, metricField),
				ExpectError: regexp.MustCompile("Error creating Events2Metric"),
			},
			{
				Config:   testAccEvents2MetricConflictFirst(suffix, metricField),
				PlanOnly: true,
			},
			{
				ResourceName:      "coralogix_events2metric.first",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccEvents2MetricConflictFirst(suffix, metricField string) string {
	return fmt.Sprintf(`resource %q %q {
  name        = "tf_acc_e2m_conflict_first_%s"
  description = "first events2metric owning the generated metric name"
  logs_query = {
    lucene = "remote_addr_enriched:/.*/"
  }

  metric_fields = {
    %s = {
      source_field = "duration"
    }
  }

  permutations = {
    limit = 1000
  }
}
`, "coralogix_events2metric", "first", suffix, metricField)
}

func testAccEvents2MetricConflictBoth(suffix, metricField string) string {
	return testAccEvents2MetricConflictFirst(suffix, metricField) + fmt.Sprintf(`
resource %q %q {
  name        = "tf_acc_e2m_conflict_second_%s"
  description = "second events2metric reusing the same metric name"
  logs_query = {
    lucene = "remote_addr_enriched:/.*/"
  }

  metric_fields = {
    %s = {
      source_field = "duration"
    }
  }

  permutations = {
    limit = 1000
  }

  depends_on = [coralogix_events2metric.first]
}
`, "coralogix_events2metric", "second", suffix, metricField)
}
