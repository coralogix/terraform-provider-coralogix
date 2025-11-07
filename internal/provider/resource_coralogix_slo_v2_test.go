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
	"context"
	"fmt"
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

var sloV2ResourceName = "coralogix_slo_v2.test"

func TestAccCoralogixResourceSLOV2RequestBased(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccSLOV2CheckDestroy,
		Steps: []resource.TestStep{
			{
				Config:  testAccCoralogixSLOV2RequestBased(),
				Destroy: false,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(sloV2ResourceName, "name", "coralogix_slo_go_example"),
					resource.TestCheckResourceAttr(sloV2ResourceName, "description", "Example SLO for Coralogix using request-based metrics"),
					resource.TestCheckResourceAttr(sloV2ResourceName, "target_threshold_percentage", "30"),
				),
			},
		},
	})
}

func TestAccCoralogixResourceSLOV2WindowBased(t *testing.T) {

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccSLOV2CheckDestroy,
		Steps: []resource.TestStep{
			{
				Config:  testAccCoralogixSLOV2WindowBased(),
				Destroy: false,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(sloV2ResourceName, "name", "coralogix_window_based_slo"),
					resource.TestCheckResourceAttr(sloV2ResourceName, "description", "Example SLO using window-based metrics"),
					resource.TestCheckResourceAttr(sloV2ResourceName, "target_threshold_percentage", "95"),
				),
			},
		},
	})
}

func testAccSLOV2CheckDestroy(s *terraform.State) error {
	client := testAccProvider.Meta().(*clientset.ClientSet).SLOs()
	ctx := context.TODO()
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "coralogix_slo_v2" {
			continue
		}

		_, _, err := client.SlosServiceGetSlo(ctx, rs.Primary.ID).Execute()
		if err == nil {
			return fmt.Errorf("slo still exists: %v, %v", rs.Primary.ID, err)
		}
	}
	return nil
}

func testAccCoralogixSLOV2RequestBased() string {
	return `
resource "coralogix_slo_v2" "test" {
  name                        = "coralogix_slo_go_example"
  description                 = "Example SLO for Coralogix using request-based metrics"
  target_threshold_percentage = 30.0
  labels = {
    label1 = "value1"
  }
  sli = {
    request_based_metric_sli = {
      good_events = {
        query = "avg(rate(cpu_usage_seconds_total[5m])) by (instance)"
      }
      total_events = {
        query = "avg(rate(cpu_usage_seconds_total[5m])) by (instance)"
      }
    }
  }
  window = {
    slo_time_frame = "7_days"
  }
}
`
}

func testAccCoralogixSLOV2WindowBased() string {
	return `
resource "coralogix_slo_v2" "test" {
  name                        = "coralogix_window_based_slo"
  description                 = "Example SLO using window-based metrics"
  target_threshold_percentage = 95.0
  labels = {
    env     = "prod"
    service = "api"
  }
  sli = {
    window_based_metric_sli = {
      query = {
        query = "avg(avg_over_time(request_duration_seconds[1m]))"
      }
      window              = "1_minute"
      comparison_operator = "less_than"
      threshold           = 0.232
    }
  }
  window = {
    slo_time_frame = "28_days"
  }
}
`
}
