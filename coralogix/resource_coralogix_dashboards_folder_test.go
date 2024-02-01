package coralogix

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"terraform-provider-coralogix/coralogix/clientset"
	dashboard "terraform-provider-coralogix/coralogix/clientset/grpc/dashboards"
)

var dashboardsFolderResourceName = "coralogix_dashboards_folder.test"

func TestAccCoralogixResourceDashboardsFolder(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
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
		}`
}

func testAccCheckDashboardsFolderDestroy(s *terraform.State) error {
	client := testAccProvider.Meta().(*clientset.ClientSet).DashboardsFolders()

	ctx := context.TODO()

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "coralogix_dashboards_folder" {
			continue
		}
		resp, err := client.GetDashboardsFolders(ctx, &dashboard.ListDashboardFoldersRequest{})
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
