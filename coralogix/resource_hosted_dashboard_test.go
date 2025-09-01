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
	"os"
	"path/filepath"
	"testing"
	"time"

	"terraform-provider-coralogix/coralogix/clientset"

	gapi "github.com/grafana/grafana-api-golang-client"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

var hostedDashboardResourceName = "coralogix_hosted_dashboard.test"
var hostedDashboardFolderResourceName = "coralogix_grafana_folder.test_folder"

func TestAccCoralogixResourceHostedGrafanaDashboardCreate(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	parent := filepath.Dir(wd)
	filePath := parent + "/examples/resources/coralogix_hosted_dashboard/grafana_acc_dashboard.json"
	updatedFilePath := parent + "/examples/resources/coralogix_hosted_dashboard/grafana_acc_updated_dashboard.json"

	// Generate unique folder titles for this test run
	uniqueSuffix := fmt.Sprintf("%d", time.Now().UnixMilli())
	folderTitle := fmt.Sprintf("Test Folder %s", uniqueSuffix)
	folderUpdateTitle := fmt.Sprintf("Updated Folder Title %s", uniqueSuffix)

	t.Logf("[INFO] Starting test with folder titles: %s -> %s", folderTitle, folderUpdateTitle)

	expectedInitialConfig := `{"title":"Title test","uid":"UID"}`
	expectedUpdatedTitleConfig := `{"title":"Updated Title","uid":"UID"}`

	var dashboard gapi.Dashboard

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { TestAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccDashboardCheckDestroy,
		Steps: []resource.TestStep{
			{
				// Test resource creation.
				Config: testAccCoralogixResourceGrafanaDashboard(filePath, folderTitle, false, ""),
				Check: resource.ComposeTestCheckFunc(
					testAccDashboardCheckExists(hostedDashboardResourceName, &dashboard),
					resource.TestCheckResourceAttr(
						hostedDashboardResourceName, "grafana.0.config_json", expectedInitialConfig,
					),
					resource.TestCheckResourceAttrSet(
						hostedDashboardResourceName, "grafana.0.folder",
					),
					resource.TestCheckResourceAttrSet(hostedDashboardFolderResourceName, "id"),
					resource.TestCheckResourceAttr(hostedDashboardFolderResourceName, "title", folderTitle),
					func() resource.TestCheckFunc {
						return func(s *terraform.State) error {
							t.Logf("[INFO] Step 1 completed - Dashboard created successfully with folder: %s", folderTitle)
							return nil
						}
					}(),
				),
			},
			{
				Config: testAccCoralogixResourceGrafanaDashboard(updatedFilePath, folderUpdateTitle, false, ""),
				Check: resource.ComposeTestCheckFunc(
					testAccDashboardCheckExists(hostedDashboardResourceName, &dashboard),
					resource.TestCheckResourceAttr(
						hostedDashboardResourceName, "grafana.0.config_json", expectedUpdatedTitleConfig,
					),
					resource.TestCheckResourceAttr(hostedDashboardFolderResourceName, "title", folderUpdateTitle),
					func() resource.TestCheckFunc {
						return func(s *terraform.State) error {
							t.Logf("[INFO] Step 2 completed - Dashboard updated successfully with folder: %s", folderUpdateTitle)
							return nil
						}
					}(),
				),
			},
		},
	})
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
		_, uid := extractDashboardTypeAndUIDFromID(rs.Primary.ID)
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
			_, originalUID := extractDashboardTypeAndUIDFromID(rs.Primary.ID)
			if uid, ok := resp.Model["uid"]; ok && uid.(string) == originalUID {
				return fmt.Errorf("grafana-dashboard still exists: %s", originalUID)
			}
		}
	}

	return nil
}

func testAccCoralogixResourceGrafanaDashboard(filePath, folderTitle string, missingBackend bool, timestamp string) string { // todo - remove missing backend param and timestamp param
	if !missingBackend {
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
	} else {
		return fmt.Sprintf(
			`resource "coralogix_hosted_dashboard" test {
 					grafana{
  						config_json = jsonencode({
							title = "Title test"
							uid = "%s"
						})
  						folder = coralogix_grafana_folder.test_folder.id
  					}
				}
				
				resource "coralogix_grafana_folder" "test_folder" {
  					title = "%s"
				}
`, fmt.Sprintf("test-uid-%s", timestamp), folderTitle)
	}
}

func TestAccCoralogixResourceHostedGrafanaDashboard_MissingInBackend(t *testing.T) {
	uniqueSuffix := fmt.Sprintf("%d", time.Now().UnixMilli())
	expectedFolderTitle := fmt.Sprintf("Test Folder %s", uniqueSuffix)
	expectedUID := fmt.Sprintf("test-uid-%s", uniqueSuffix)
	expectedInitialConfig := fmt.Sprintf(`{"title":"Title test","uid":"%s"}`, expectedUID)

	var dashboard gapi.Dashboard

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { TestAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      nil, // Disable destroy check since we manually deleted resources
		Steps: []resource.TestStep{
			{
				// Step 1: create the dashboard normally
				Config: testAccCoralogixResourceGrafanaDashboard("", expectedFolderTitle, true, uniqueSuffix),
				Check: resource.ComposeTestCheckFunc(
					testAccDashboardCheckExists(hostedDashboardResourceName, &dashboard),
					resource.TestCheckResourceAttr(
						hostedDashboardResourceName, "grafana.0.config_json", expectedInitialConfig,
					),
					resource.TestCheckResourceAttrSet(
						hostedDashboardResourceName, "grafana.0.folder",
					),
					func() resource.TestCheckFunc {
						return func(s *terraform.State) error {
							t.Log("[INFO] Step 1 completed - Dashboard created successfully")
							return nil
						}
					}(),
				),
			},
			{
				// Step 2: simulate drift by deleting the dashboard from backend
				PreConfig: func() {
					// Verify dashboard was created successfully in Step 1
					if dashboard.Model == nil || dashboard.Model["uid"] == nil {

						t.Fatalf("dashboard UID not available from Step 1")
					}

					// Get the dashboard UID
					dashboardUID := dashboard.Model["uid"].(string)
					t.Logf("[INFO] Using dashboard UID: %s", dashboardUID)

					// Delete the dashboard from Grafana API to simulate drift
					client := testAccProvider.Meta().(*clientset.ClientSet).Grafana()
					t.Logf("[INFO] Attempting to delete dashboard with UID: %s", dashboardUID)
					if err := client.DeleteGrafanaDashboard(context.TODO(), dashboardUID); err != nil {
						t.Fatalf("failed to delete dashboard from backend: %s", err)
					}
					t.Logf("[INFO] Successfully deleted dashboard: %s", dashboardUID)

					// Verify the dashboard is actually deleted
					t.Logf("[INFO] Verifying dashboard deletion by attempting to read it")
					if _, err := client.GetGrafanaDashboard(context.TODO(), dashboardUID); err != nil {
						t.Logf("[INFO] Dashboard read failed as expected: %s", err.Error())
					} else {
						t.Fatalf("Dashboard still exists after deletion")
					}

					t.Log("[INFO] Step 2 completed - Dashboard deleted from backend to simulate drift")
				},
				Config:             testAccCoralogixResourceGrafanaDashboard("", expectedFolderTitle, true, uniqueSuffix),
				PlanOnly:           true, // Only run plan, don't apply
				ExpectNonEmptyPlan: true, // we expect Terraform to detect drift and plan recreation
			},
			{
				// Step 3: verify that Terraform can successfully recreate the resources
				Config: testAccCoralogixResourceGrafanaDashboard("", expectedFolderTitle, true, uniqueSuffix),
				Check: resource.ComposeTestCheckFunc(
					testAccDashboardCheckExists(hostedDashboardResourceName, &dashboard),
					resource.TestCheckResourceAttr(
						hostedDashboardResourceName, "grafana.0.config_json", expectedInitialConfig,
					),
					resource.TestCheckResourceAttrSet(
						hostedDashboardResourceName, "grafana.0.folder",
					),
					func() resource.TestCheckFunc {
						return func(s *terraform.State) error {
							t.Log("[INFO] Step 3 completed - Resources successfully recreated by Terraform")
							return nil
						}
					}(),
				),
			},
		},
	})
}
