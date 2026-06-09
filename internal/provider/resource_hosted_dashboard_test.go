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
	"os"
	"path/filepath"
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/provider/data_exploration"

	gapi "github.com/grafana/grafana-api-golang-client"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

var hostedDashboardResourceName = "coralogix_hosted_dashboard.test"
var hostedDashboardFolderResourceName = "coralogix_grafana_folder.test_folder"

func TestAccCoralogixResourceHostedGrafanaDashboardCreate(t *testing.T) {
	filePath, updatedFilePath, dashboardUID := testAccGrafanaDashboardFiles(t)

	expectedInitialConfig := fmt.Sprintf(`{"title":"Title test","uid":%q}`, dashboardUID)
	expectedUpdatedTitleConfig := fmt.Sprintf(`{"title":"Updated Title","uid":%q}`, dashboardUID)

	expectedFolderTitle := acctest.RandomWithPrefix("tf-acc-test-folder")
	expectedFolderUpdateTitle := expectedFolderTitle + "-updated"

	var dashboard gapi.Dashboard

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccDashboardCheckDestroy,
		Steps: []resource.TestStep{
			{
				// Test resource creation.
				Config: testAccCoralogixResourceGrafanaDashboard(filePath, expectedFolderTitle),
				Check: resource.ComposeTestCheckFunc(
					testAccDashboardCheckExists(hostedDashboardResourceName, &dashboard),
					resource.TestCheckResourceAttr(
						hostedDashboardResourceName, "grafana.0.config_json", expectedInitialConfig,
					),
					resource.TestCheckResourceAttrSet(
						hostedDashboardResourceName, "grafana.0.folder",
					),
					resource.TestCheckResourceAttrSet(hostedDashboardFolderResourceName, "id"),
					resource.TestCheckResourceAttr(hostedDashboardFolderResourceName, "title", expectedFolderTitle),
				),
			},
			{
				PreConfig: func() {
					client := testAccProvider.Meta().(*clientset.ClientSet).Grafana()
					err := client.DeleteGrafanaDashboard(context.TODO(), dashboard.Model["uid"].(string))
					if err != nil {
						panic(err)
					}
				},
				// Test resource creation.
				Config: testAccCoralogixResourceGrafanaDashboard(filePath, expectedFolderTitle),
				Check: resource.ComposeTestCheckFunc(
					testAccDashboardCheckExists(hostedDashboardResourceName, &dashboard),
					resource.TestCheckResourceAttr(
						hostedDashboardResourceName, "grafana.0.config_json", expectedInitialConfig,
					),
					resource.TestCheckResourceAttrSet(
						hostedDashboardResourceName, "grafana.0.folder",
					),
					resource.TestCheckResourceAttrSet(hostedDashboardFolderResourceName, "id"),
					resource.TestCheckResourceAttr(hostedDashboardFolderResourceName, "title", expectedFolderTitle),
				),
			},
			{
				Config: testAccCoralogixResourceGrafanaDashboard(updatedFilePath, expectedFolderUpdateTitle),
				Check: resource.ComposeTestCheckFunc(
					testAccDashboardCheckExists(hostedDashboardResourceName, &dashboard),
					resource.TestCheckResourceAttr(
						hostedDashboardResourceName, "grafana.0.config_json", expectedUpdatedTitleConfig,
					),
					resource.TestCheckResourceAttr(hostedDashboardFolderResourceName, "title", expectedFolderUpdateTitle),
				),
			},
		},
	})
}

func testAccGrafanaDashboardFiles(t *testing.T) (string, string, string) {
	t.Helper()

	dashboardUID := acctest.RandomWithPrefix("tfaccdash")
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "grafana_acc_dashboard.json")
	updatedFilePath := filepath.Join(tempDir, "grafana_acc_updated_dashboard.json")
	if err := os.WriteFile(filePath, []byte(fmt.Sprintf(`{"title":"Title test","uid":%q}`, dashboardUID)), 0600); err != nil {
		t.Fatalf("writing initial dashboard config: %s", err)
	}
	if err := os.WriteFile(updatedFilePath, []byte(fmt.Sprintf(`{"title":"Updated Title","uid":%q}`, dashboardUID)), 0600); err != nil {
		t.Fatalf("writing updated dashboard config: %s", err)
	}

	return filePath, updatedFilePath, dashboardUID
}

func testAccDashboardCheckExists(rn string, dashboard *gapi.Dashboard) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[rn]
		if !ok {
			return fmt.Errorf("resource not found: %s", rn)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("resource id not set")
		}
		client := testAccProvider.Meta().(*clientset.ClientSet).Grafana()
		_, uid := data_exploration.ExtractDashboardTypeAndUIDFromID(rs.Primary.ID)
		gotDashboard, err := client.GetGrafanaDashboard(context.TODO(), uid)
		if err != nil {
			return fmt.Errorf("error getting dashboard: %s", err)
		}
		*dashboard = *gotDashboard
		return nil
	}
}

func testAccDashboardCheckDestroy(s *terraform.State) error {
	client := testAccProvider.Meta().(*clientset.ClientSet).Grafana()
	ctx := context.TODO()
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "coralogix_hosted_dashboard" {
			continue
		}

		resp, err := client.GetGrafanaDashboard(ctx, rs.Primary.ID)
		if err == nil {
			_, originalUID := data_exploration.ExtractDashboardTypeAndUIDFromID(rs.Primary.ID)
			if uid, ok := resp.Model["uid"]; ok && uid.(string) == originalUID {
				return fmt.Errorf("grafana-dashboard still exists: %s", originalUID)
			}
		}
	}

	return nil
}

func testAccCoralogixResourceGrafanaDashboard(filePath, folderTitle string) string {
	return fmt.Sprintf(
		`resource "coralogix_hosted_dashboard" test {
 					grafana{
  						config_json = file("%s")
						folder = coralogix_grafana_folder.test_folder.id
					}
				}
				
				resource "coralogix_grafana_folder" "test_folder" {
  					title = "%s"
				}
`, filePath, folderTitle)
}
