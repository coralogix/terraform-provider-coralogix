package coralogix

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	gapi "github.com/grafana/grafana-api-golang-client"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"terraform-provider-coralogix/coralogix/clientset"
)

func TestAccCoralogixResourceHostedGrafanaDashboardCreate(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	parent := filepath.Dir(wd)
	filePath := parent + "/examples/hosted_dashboard/grafana_acc_dashboard.json"
	updatedFilePath := parent + "/examples/hosted_dashboard/grafana_acc_updated_dashboard.json"

	expectedInitialConfig := `{"title":"Title","uid":"UID"}`
	expectedUpdatedTitleConfig := `{"title":"Updated Title","uid":"UpdatedUID"}`

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
					testAccDashboardCheckExists("coralogix_hosted_dashboard.test", &dashboard),
					resource.TestCheckResourceAttr("coralogix_hosted_dashboard.test", "uid", "UID"),
					resource.TestCheckResourceAttr(
						"coralogix_hosted_dashboard.test", "grafana.0.config_json", expectedInitialConfig,
					),
				),
			},
			{
				Config: testAccCoralogixResourceGrafanaDashboard(updatedFilePath),
				Check: resource.ComposeTestCheckFunc(
					testAccDashboardCheckExists("coralogix_hosted_dashboard.test", &dashboard),
					resource.TestCheckResourceAttr("coralogix_hosted_dashboard.test", "uid", "UpdatedUID"),
					resource.TestCheckResourceAttr(
						"coralogix_hosted_dashboard.test", "grafana.0.config_json", expectedUpdatedTitleConfig,
					),
				),
			},
			{
				// Importing matches the state of the previous step.
				ResourceName:            "coralogix_hosted_dashboard.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"message"},
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
		client := testAccProvider.Meta().(*clientset.ClientSet).GrafanaDashboards()
		gotDashboard, err := client.GetGrafanaDashboard(context.TODO(), rs.Primary.ID)
		if err != nil {
			return fmt.Errorf("error getting dashboard: %s", err)
		}
		*dashboard = *gotDashboard
		return nil
	}
}

func testAccDashboardCheckDestroy(s *terraform.State) error {
	client := testAccProvider.Meta().(*clientset.ClientSet).GrafanaDashboards()
	ctx := context.TODO()
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "coralogix_hosted_dashboard" {
			continue
		}

		resp, err := client.GetGrafanaDashboard(ctx, rs.Primary.ID)
		if err == nil {
			if uid, ok := resp.Model["uid"]; ok && uid.(string) == rs.Primary.ID {
				return fmt.Errorf("grafana-dashboard still exists: %s", rs.Primary.ID)
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