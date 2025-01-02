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
	"context"
	"fmt"
	"testing"

	"terraform-provider-coralogix/coralogix/clientset"
	slos "terraform-provider-coralogix/coralogix/clientset/grpc/slo"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

var sloResourceName = "coralogix_slo.test"

func TestAccCoralogixResourceSLOCreate(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccSLOCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config:  testAccCoralogixResourceSLO(),
				Destroy: false,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(sloResourceName, "name", "coralogix_slo_example"),
					resource.TestCheckResourceAttr(sloResourceName, "service_name", "service_name"),
					resource.TestCheckResourceAttr(sloResourceName, "description", "description"),
					resource.TestCheckResourceAttr(sloResourceName, "target_percentage", "30"),
					resource.TestCheckResourceAttr(sloResourceName, "type", "latency"),
					resource.TestCheckResourceAttr(sloResourceName, "threshold_microseconds", "1000000"),
					resource.TestCheckResourceAttr(sloResourceName, "threshold_symbol_type", "greater"),
					resource.TestCheckResourceAttr(sloResourceName, "period", "7_days"),
					resource.TestCheckResourceAttr(sloResourceName, "filters.0.field", "severity"),
					resource.TestCheckResourceAttr(sloResourceName, "filters.0.compare_type", "is"),
					resource.TestCheckResourceAttr(sloResourceName, "filters.0.field_values.0", "error"),
					resource.TestCheckResourceAttr(sloResourceName, "filters.0.field_values.1", "warning"),
				),
			},
		},
	},
	)
}

func testAccSLOCheckDestroy(s *terraform.State) error {
	client := testAccProvider.Meta().(*clientset.ClientSet).SLOs()
	ctx := context.TODO()
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "coralogix_slo" {
			continue
		}

		if resp, err := client.GetSLO(ctx, &slos.GetServiceSloRequest{Id: wrapperspb.String(rs.Primary.ID)}); err == nil {
			if resp.GetSlo().GetId().GetValue() == rs.Primary.ID {
				return fmt.Errorf("slo still exists: %s", rs.Primary.ID)
			}
		}
	}

	return nil
}

func testAccCoralogixResourceSLO() string {
	return `variable "test" {
  				type = number
  				default = 1000000
			}

			resource "coralogix_slo" "test" {
  				name            = "coralogix_slo_example"
  				service_name    = "service_name"
  				description     = "description"
  				target_percentage = 30
  				type            = "latency"
  				threshold_microseconds = var.test
  				threshold_symbol_type = "greater"
  				period          = "7_days"
  				filters = [
    				{
      					field = "severity"
      					compare_type = "is"
      					field_values = ["error", "warning"]
    				},
  				]
	}
	`
}
