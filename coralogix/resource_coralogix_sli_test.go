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
	sli "terraform-provider-coralogix/coralogix/clientset/grpc/sli"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

var sliResourceName = "coralogix_sli.test"

func TestAccCoralogixResourceSLICreate(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccSLICheckDestroy,
		Steps: []resource.TestStep{
			{
				Config:  testAccCoralogixResourceSLI(),
				Destroy: false,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(sliResourceName, "name", "coralogix_sli_example"),
					resource.TestCheckResourceAttr(sliResourceName, "slo_percentage", "80"),
					resource.TestCheckResourceAttr(sliResourceName, "service_name", "service_name"),
					resource.TestCheckResourceAttr(sliResourceName, "threshold_value", "3"),
					resource.TestCheckTypeSetElemNestedAttrs(sliResourceName, "filters.*",
						map[string]string{
							"compare_type":   "is",
							"field":          "tags.http.route",
							"field_values.#": "2",
							"field_values.0": "nidataframe/v1/tables",
							"field_values.1": "nidataframe/v1/tables/{id}/data",
						}),
					resource.TestCheckTypeSetElemNestedAttrs(sliResourceName, "filters.*",
						map[string]string{
							"compare_type":   "is",
							"field":          "tags.http.well_formed_request",
							"field_values.#": "1",
							"field_values.0": "true",
						}),
					resource.TestCheckTypeSetElemNestedAttrs(sliResourceName, "filters.*",
						map[string]string{
							"compare_type":   "is",
							"field":          "tags.http.request.method",
							"field_values.#": "1",
							"field_values.0": "POST",
						}),
				),
			},
		},
	},
	)
}

func testAccSLICheckDestroy(s *terraform.State) error {
	client := testAccProvider.Meta().(*clientset.ClientSet).SLIs()
	ctx := context.TODO()
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "coralogix_sli" {
			continue
		}

		if resp, err := client.GetSLIs(ctx, &sli.GetSlisRequest{ServiceName: wrapperspb.String(rs.Primary.Attributes["service_name"])}); err == nil {
			for _, sli := range resp.GetSlis() {
				if id := sli.SliId.GetValue(); id == rs.Primary.ID {
					return fmt.Errorf("sli still exists: %s", id)
				}
			}
		}
	}

	return nil
}

func testAccCoralogixResourceSLI() string {
	return `resource "coralogix_sli" "test" {
  					name            = "coralogix_sli_example"
					slo_percentage  = 80
  					service_name    = "service_name"
  					threshold_value = 3
					filters = [
					{
    					compare_type = "is"
    					field        = "tags.http.route"
    					field_values = ["nidataframe/v1/tables", "nidataframe/v1/tables/{id}/data"]
    				},
    				{
      					compare_type = "is"
      					field        = "tags.http.well_formed_request"
      					field_values = ["true"]
    				},
    				{
      					compare_type = "is"
      					field        = "tags.http.request.method"
      					field_values = ["POST"]
					},
					]
				}
`
}
