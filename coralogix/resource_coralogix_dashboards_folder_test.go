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

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

var dashboardsFolderResourceName = "coralogix_dashboards_folder.test"

func TestAccCoralogixResourceDashboardsFolder(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { TestAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDashboardsFolderDestroy,
		Steps: []resource.TestStep{
			{

				Config: testAccCoralogixResourceDashboardsFolder(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(dashboardsFolderResourceName, "id"),
					resource.TestCheckResourceAttr(dashboardsFolderResourceName, "name", "test"),
				),
			},
			{
				ResourceName:      dashboardsFolderResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
func testAccCoralogixResourceDashboardsFolder() string {
	return `resource "coralogix_dashboards_folder" "test" {
			name = "test"
		}
`
}

func testAccCheckDashboardsFolderDestroy(s *terraform.State) error {
	client := testAccProvider.Meta().(*clientset.ClientSet).DashboardsFolders()

	ctx := context.TODO()

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "coralogix_dashboards_folder" {
			continue
		}
		resp, err := client.List(ctx)
		if err == nil {
			for _, folder := range resp.GetFolder() {
				if folder.GetId().GetValue() == rs.Primary.ID {
					return fmt.Errorf("dashboard folder still exists: %s", rs.Primary.ID)
				}
			}
		}
	}

	return nil
}
