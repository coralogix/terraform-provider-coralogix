// Copyright 2025 Coralogix Ltd.
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
	"strconv"
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/coralogix/clientset"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestAccCoralogixResourceView(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { TestAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckViewDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceView(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("coralogix_view.test", "id"),
					resource.TestCheckResourceAttr("coralogix_view.test", "name", "Example View"),
					resource.TestCheckResourceAttr("coralogix_view.test", "time_selection.custom_selection.from_time", "2023-01-01T00:00:00Z"),
					resource.TestCheckResourceAttr("coralogix_view.test", "time_selection.custom_selection.to_time", "2023-01-02T00:00:00Z"),
					resource.TestCheckResourceAttr("coralogix_view.test", "search_query.query", "error OR warning"),
				),
			},
			{
				ResourceName:      "coralogix_view.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccCoralogixResourceUpdatedView(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("coralogix_view.test", "id"),
					resource.TestCheckResourceAttr("coralogix_view.test", "name", "Example View Updated"),
					resource.TestCheckResourceAttr("coralogix_view.test", "time_selection.quick_selection.seconds", "86400"), // 24 hours in seconds
					resource.TestCheckResourceAttr("coralogix_view.test", "search_query.query", "error OR warning"),
				),
			},
		},
	})
}

func testAccCheckViewDestroy(s *terraform.State) error {
	client := testAccProvider.Meta().(*clientset.ClientSet).Views()
	ctx := context.TODO()

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "coralogix_view" {
			continue
		}

		if rs.Primary.ID == "" {
			return nil
		}

		intID, err := strconv.Atoi(rs.Primary.ID)
		if err != nil {
			return fmt.Errorf("invalid ID format: %s", rs.Primary.ID)
		}

		resp, err := client.Get(ctx, &cxsdk.GetViewRequest{
			Id: wrapperspb.Int32(int32(intID)),
		})
		if err == nil && resp != nil && resp.View != nil {
			return fmt.Errorf("view still exists: %v", rs.Primary.ID)
		}
	}
	return nil
}

func testAccCoralogixResourceView() string {
	return `resource "coralogix_view" "test" {
  name        = "Example View"
  time_selection = {
    custom_selection = {
      from_time = "2023-01-01T00:00:00Z"
      to_time   = "2023-01-02T00:00:00Z"
    }
  }
  search_query = {
    query = "error OR warning"
  }
}
	`
}

func testAccCoralogixResourceUpdatedView() string {
	return `resource "coralogix_view" "test" { 
		name        = "Example View Updated"
  time_selection = {
    quick_selection = {
    	seconds = 86400 # 24 hours in seconds
    }
  }
  search_query = {
    query = "error OR warning"
  }
}
	`
}
