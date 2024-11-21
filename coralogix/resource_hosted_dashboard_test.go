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

	"terraform-provider-coralogix/coralogix/clientset"

	gapi "github.com/grafana/grafana-api-golang-client"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

var hostedDashboardResourceName = "coralogix_hosted_dashboard.test"

func TestAccCoralogixResourceHostedGrafanaDashboardCreate(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	parent := filepath.Dir(wd)
	filePath := parent + "/examples/resources/coralogix_hosted_dashboard/grafana_acc_dashboard.json"
	updatedFilePath := parent + "/examples/resources/coralogix_hosted_dashboard/grafana_acc_updated_dashboard.json"

	expectedInitialConfig := `{"title":"Title test","uid":"UID"}`
	expectedUpdatedTitleConfig := `{"title":"Updated Title","uid":"UID"}`

	var dashboard gapi.Dashboard

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccDashboardCheckDestroy,
		Steps: []resource.TestStep{
			{
				// Test resource creation.
				Config: testAccCoralogixResourceGrafanaDashboard(filePath),
				Check: resource.ComposeTestCheckFunc(
					testAccDashboardCheckExists(hostedDashboardResourceName, &dashboard),
					resource.TestCheckResourceAttr(
						hostedDashboardResourceName, "grafana.0.config_json", expectedInitialConfig,
					),
				),
			},
			{
				Config: testAccCoralogixResourceGrafanaDashboard(updatedFilePath),
				Check: resource.ComposeTestCheckFunc(
					testAccDashboardCheckExists(hostedDashboardResourceName, &dashboard),
					resource.TestCheckResourceAttr(
						hostedDashboardResourceName, "grafana.0.config_json", expectedUpdatedTitleConfig,
					),
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

func testAccCoralogixResourceGrafanaDashboard(filePath string) string {
	return fmt.Sprintf(
		`resource "coralogix_hosted_dashboard" test {
 					grafana{
  						config_json = file("%s")
					}
				}
`, filePath)
}
