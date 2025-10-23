// Copyright 2025 Coralogix Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	https://www.apache.org/licenses/LICENSE-2.0
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

	"github.com/coralogix/terraform-provider-coralogix/coralogix/clientset"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestAccCoralogixResourceViewsFolder(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { TestAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckViewsFolderDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceViewsFolder(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("coralogix_views_folder.test", "id"),
					resource.TestCheckResourceAttr("coralogix_views_folder.test", "name", "Example Views Folder"),
				),
			},
			{
				ResourceName:      "coralogix_views_folder.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccCoralogixResourceUpdatedViewsFolder(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("coralogix_views_folder.test", "id"),
					resource.TestCheckResourceAttr("coralogix_views_folder.test", "name", "Example Views Folder Updated"),
				),
			},
		},
	})
}

func testAccCheckViewsFolderDestroy(s *terraform.State) error {
	client := testAccProvider.Meta().(*clientset.ClientSet).ViewsFolders()
	ctx := context.TODO()

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "coralogix_views_folder" {
			continue
		}

		if rs.Primary.ID == "" {
			return nil
		}

		resp, err := client.Get(ctx, &cxsdk.GetViewFolderRequest{Id: wrapperspb.String(rs.Primary.ID)})
		if err == nil && resp != nil && resp.Folder != nil {
			return fmt.Errorf("views-folder still exists: %v", rs.Primary.ID)
		}
	}
	return nil
}

func testAccCoralogixResourceViewsFolder() string {
	return `resource "coralogix_views_folder" "test" {
  name        = "Example Views Folder"
}
	`
}

func testAccCoralogixResourceUpdatedViewsFolder() string {
	return `resource "coralogix_views_folder" "test" { 
		name        = "Example Views Folder Updated"
}
	`
}
