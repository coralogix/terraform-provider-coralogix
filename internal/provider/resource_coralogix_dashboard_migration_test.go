// Copyright 2026 Coralogix Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
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
	"strings"
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	dashboardservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_service"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

const (
	dashboardMigrationAcceptanceEnv   = "CORALOGIX_DASHBOARD_MIGRATION_ACC"
	dashboardMigrationProviderSource  = "registry.terraform.io/coralogix/coralogix"
	dashboardMigrationGRPCVersion     = "= 3.6.0"
	dashboardMigrationSchemaV3Version = "= 2.1.2"
)

type dashboardMigrationIdentity struct {
	fixture      string
	client       *dashboardservice.DashboardServiceAPIService
	resourceID   string
	folderID     string
	generatedIDs []string
}

func TestAccCoralogixResourceDashboardMigrationFromV360(t *testing.T) {
	requireDashboardMigrationAcceptance(t)

	for _, group := range dashboardStructuredQueryWidgetGroups {
		group := group
		t.Run(group.name, func(t *testing.T) {
			fixture := "TestAccCoralogixResourceDashboardMigrationFromV360-" + group.name
			dashboardName := dashboardOpenAPIFixtureName(fixture)
			folderName := dashboardOpenAPIFixtureName(fixture + "Folder")
			identity := &dashboardMigrationIdentity{fixture: fixture}
			accessPolicy := testAccCoralogixDashboardAccessPolicyPretty()
			includeMarkdown := group.name == "line-and-table"
			wantWidgets := len(group.widgets)
			if includeMarkdown {
				wantWidgets++
			}
			initialConfig := dashboardMigrationV360Config(dashboardName, folderName, "Created by the gRPC-backed provider", accessPolicy, includeMarkdown, group.widgets)
			updatedConfig := dashboardMigrationV360Config(dashboardName, folderName, "Updated by the REST-backed provider", accessPolicy, includeMarkdown, group.widgets)

			resource.ParallelTest(t, resource.TestCase{
				PreCheck: func() {
					testAccPreCheck(t)
					identity.client = dashboardOpenAPIAcceptanceClient(t)
				},
				CheckDestroy: testAccCheckDashboardDestroy(t),
				Steps: []resource.TestStep{
					{
						Config:            initialConfig,
						ExternalProviders: dashboardMigrationExternalProvider(dashboardMigrationGRPCVersion),
						Check: identity.checkCurrentStateAndBackend(
							wantWidgets,
							"Created by the gRPC-backed provider",
							accessPolicy,
							true,
						),
					},
					{
						Config:                   initialConfig,
						ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
						ConfigPlanChecks: resource.ConfigPlanChecks{
							PreApply: []plancheck.PlanCheck{
								plancheck.ExpectResourceAction(dashboardResourceName, plancheck.ResourceActionNoop),
							},
							PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
						},
						Check: identity.checkCurrentStateAndBackend(
							wantWidgets,
							"Created by the gRPC-backed provider",
							accessPolicy,
							true,
						),
					},
					{
						Config:                   updatedConfig,
						ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
						ConfigPlanChecks: resource.ConfigPlanChecks{
							PreApply: []plancheck.PlanCheck{
								plancheck.ExpectResourceAction(dashboardResourceName, plancheck.ResourceActionUpdate),
							},
							PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
						},
						Check: identity.checkCurrentStateAndBackend(
							wantWidgets,
							"Updated by the REST-backed provider",
							accessPolicy,
							true,
						),
					},
					{
						ResourceName:             dashboardResourceName,
						ImportState:              true,
						ImportStateVerify:        true,
						ImportStateVerifyIgnore:  []string{"access_policy", "folder"},
						ImportStateCheck:         identity.checkImportedState(wantWidgets, accessPolicy, true),
						ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
					},
				},
			})
		})
	}
}

func TestAccCoralogixResourceDashboardMigrationFromSchemaV3(t *testing.T) {
	requireDashboardMigrationAcceptance(t)

	const fixture = "TestAccCoralogixResourceDashboardMigrationFromSchemaV3"
	dashboardName := dashboardOpenAPIFixtureName(fixture)
	identity := &dashboardMigrationIdentity{fixture: fixture}
	initialConfig := dashboardMigrationSchemaV3Config(dashboardName, "Created with dashboard schema v3")
	updatedConfig := dashboardMigrationSchemaV3Config(dashboardName, "Updated after REST-backed state upgrade")

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			identity.client = dashboardOpenAPIAcceptanceClient(t)
		},
		CheckDestroy: testAccCheckDashboardDestroy(t),
		Steps: []resource.TestStep{
			{
				Config:            initialConfig,
				ExternalProviders: dashboardMigrationExternalProvider(dashboardMigrationSchemaV3Version),
				Check: identity.checkCurrentStateAndBackend(
					1,
					"Created with dashboard schema v3",
					"",
					false,
				),
			},
			{
				Config:                   initialConfig,
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(dashboardResourceName, plancheck.ResourceActionNoop),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: identity.checkCurrentStateAndBackend(
					1,
					"Created with dashboard schema v3",
					"",
					false,
				),
			},
			{
				Config:                   updatedConfig,
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(dashboardResourceName, plancheck.ResourceActionUpdate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: identity.checkCurrentStateAndBackend(
					1,
					"Updated after REST-backed state upgrade",
					"",
					false,
				),
			},
			{
				ResourceName:             dashboardResourceName,
				ImportState:              true,
				ImportStateVerify:        true,
				ImportStateCheck:         identity.checkImportedState(1, "", false),
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			},
		},
	})
}

func requireDashboardMigrationAcceptance(t *testing.T) {
	t.Helper()
	if os.Getenv(dashboardMigrationAcceptanceEnv) == "" {
		t.Skipf("set %s=1 to run registry-backed dashboard migration tests", dashboardMigrationAcceptanceEnv)
	}
	// Keep the in-process provider address identical to the registry provider
	// address recorded in state by the first test step. This must be set before
	// go test starts because migration subtests run in parallel.
	if namespace := os.Getenv(resource.EnvTfAccProviderNamespace); namespace != "coralogix" {
		t.Fatalf("set %s=coralogix to run registry-backed dashboard migration tests", resource.EnvTfAccProviderNamespace)
	}
}

func dashboardMigrationExternalProvider(version string) map[string]resource.ExternalProvider {
	return map[string]resource.ExternalProvider{
		"coralogix": {
			Source:            dashboardMigrationProviderSource,
			VersionConstraint: version,
		},
	}
}

func dashboardMigrationV360Config(name, folderName, description, accessPolicy string, includeMarkdown bool, widgets []dashboardStructuredWidgetSpec) string {
	dashboard := strings.TrimSuffix(dashboardOpenAPIStructuredDashboardConfigForWidgets(name, "logs", includeMarkdown, false, widgets), "}\n")
	dashboard = strings.Replace(
		dashboard,
		`  description = "Exercises every structured dashboard widget query carrier."`,
		fmt.Sprintf("  description = %q", description),
		1,
	)
	return fmt.Sprintf(`
resource "coralogix_dashboards_folder" "test_folder" {
  name = %q
}

%s
  auto_refresh = {
    type = "two_minutes"
  }
  folder = {
    id = coralogix_dashboards_folder.test_folder.id
  }
  access_policy = <<EOT
%s
EOT
}
`, folderName, dashboard, accessPolicy)
}

func dashboardMigrationSchemaV3Config(name, description string) string {
	return fmt.Sprintf(`
resource "coralogix_dashboard" "test" {
  name        = %q
  description = %q
  time_frame = {
    relative = {
      duration = "seconds:900"
    }
  }
  layout = {
    sections = [{
      rows = [{
        height = 19
        widgets = [{
          title = "count"
          definition = {
            line_chart = {
              query_definitions = [{
                query = {
                  logs = {
                    aggregations = [{ type = "count" }]
                  }
                }
              }]
            }
          }
        }]
      }]
    }]
  }
}
`, name, description)
}

func (identity *dashboardMigrationIdentity) checkCurrentStateAndBackend(
	wantWidgets int,
	wantDescription string,
	wantAccessPolicy string,
	wantFolder bool,
) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		resourceState, response, err := dashboardMigrationRead(identity.client, state, identity.fixture)
		if err != nil {
			return err
		}
		dashboard := response.Dashboard
		if dashboard.GetDescription() != wantDescription {
			return fmt.Errorf("dashboard fixture %q: backend description = %q, want %q", identity.fixture, dashboard.GetDescription(), wantDescription)
		}
		if err := identity.checkResourceAndBackendIDs(resourceState, dashboard, wantWidgets); err != nil {
			return err
		}
		if err := identity.checkFolder(state, resourceState, dashboard, wantFolder); err != nil {
			return err
		}
		if err := dashboardMigrationCheckAccessPolicy(resourceState.Primary.Attributes["access_policy"], response.GetAccessPolicy(), wantAccessPolicy, identity.fixture); err != nil {
			return err
		}
		return nil
	}
}

func (identity *dashboardMigrationIdentity) checkImportedState(
	wantWidgets int,
	wantAccessPolicy string,
	wantFolder bool,
) resource.ImportStateCheckFunc {
	return func(states []*terraform.InstanceState) error {
		if identity.client == nil {
			return fmt.Errorf("dashboard fixture %q: acceptance REST client is not initialized", identity.fixture)
		}
		for _, imported := range states {
			if imported.ID != identity.resourceID {
				continue
			}
			return identity.checkImportedResource(imported, wantWidgets, wantAccessPolicy, wantFolder)
		}
		return fmt.Errorf("dashboard fixture %q: imported resource ID %q was not found in %d state entries", identity.fixture, identity.resourceID, len(states))
	}
}

func (identity *dashboardMigrationIdentity) checkImportedResource(
	imported *terraform.InstanceState,
	wantWidgets int,
	wantAccessPolicy string,
	wantFolder bool,
) error {
	if err := identity.checkFlatGeneratedIDs(imported.Attributes, wantWidgets); err != nil {
		return err
	}
	if wantFolder && imported.Attributes["folder.id"] != identity.folderID {
		return fmt.Errorf("dashboard fixture %q: imported folder.id = %q, want %q", identity.fixture, imported.Attributes["folder.id"], identity.folderID)
	}
	if err := dashboardMigrationCheckAccessPolicy(imported.Attributes["access_policy"], imported.Attributes["access_policy"], wantAccessPolicy, identity.fixture); err != nil {
		return err
	}

	response, httpResponse, err := identity.client.DashboardsServiceGetDashboard(context.Background(), imported.ID).Execute()
	if err != nil {
		return dashboardOpenAPISafeRequestError("migration import read", identity.fixture, imported.ID, httpResponse, err)
	}
	if response == nil || response.Dashboard == nil {
		return fmt.Errorf("dashboard fixture %q: import read returned no dashboard", identity.fixture)
	}
	if wantFolder && (response.Dashboard.FolderId == nil || response.Dashboard.FolderId.GetValue() != identity.folderID) {
		return fmt.Errorf("dashboard fixture %q: backend folder association changed during import", identity.fixture)
	}
	if err := dashboardMigrationCheckAccessPolicy(imported.Attributes["access_policy"], response.GetAccessPolicy(), wantAccessPolicy, identity.fixture); err != nil {
		return err
	}
	return identity.checkBackendGeneratedIDs(response.Dashboard, wantWidgets)
}

func dashboardMigrationRead(
	client *dashboardservice.DashboardServiceAPIService,
	state *terraform.State,
	fixture string,
) (*terraform.ResourceState, *dashboardservice.GetDashboardResponse, error) {
	if client == nil {
		return nil, nil, fmt.Errorf("dashboard fixture %q: acceptance REST client is not initialized", fixture)
	}
	if state == nil || state.RootModule() == nil {
		return nil, nil, fmt.Errorf("dashboard fixture %q: Terraform state is nil", fixture)
	}
	resourceState, ok := state.RootModule().Resources[dashboardResourceName]
	if !ok || resourceState.Primary == nil || resourceState.Primary.ID == "" {
		return nil, nil, fmt.Errorf("dashboard fixture %q: dashboard state is absent", fixture)
	}
	response, httpResponse, err := client.DashboardsServiceGetDashboard(context.Background(), resourceState.Primary.ID).Execute()
	if err != nil {
		return nil, nil, dashboardOpenAPISafeRequestError("migration read", fixture, resourceState.Primary.ID, httpResponse, err)
	}
	if response == nil || response.Dashboard == nil {
		return nil, nil, fmt.Errorf("dashboard fixture %q: migration read returned no dashboard", fixture)
	}
	return resourceState, response, nil
}

func TestDashboardMigrationReadRejectsUninitializedClient(t *testing.T) {
	_, _, err := dashboardMigrationRead(nil, nil, "uninitialized-client")
	if err == nil || !strings.Contains(err.Error(), "REST client is not initialized") {
		t.Fatalf("dashboardMigrationRead() error = %v, want uninitialized REST client error", err)
	}
}

func (identity *dashboardMigrationIdentity) checkResourceAndBackendIDs(
	resourceState *terraform.ResourceState,
	dashboard *dashboardservice.Dashboard,
	wantWidgets int,
) error {
	if resourceState.Primary.Attributes["id"] != resourceState.Primary.ID {
		return fmt.Errorf("dashboard fixture %q: state id attribute = %q, resource ID = %q", identity.fixture, resourceState.Primary.Attributes["id"], resourceState.Primary.ID)
	}
	if dashboard.GetId() != resourceState.Primary.ID {
		return fmt.Errorf("dashboard fixture %q: backend dashboard ID = %q, resource ID = %q", identity.fixture, dashboard.GetId(), resourceState.Primary.ID)
	}
	if identity.resourceID == "" {
		identity.resourceID = resourceState.Primary.ID
	} else if resourceState.Primary.ID != identity.resourceID {
		return fmt.Errorf("dashboard fixture %q: resource ID changed from %q to %q", identity.fixture, identity.resourceID, resourceState.Primary.ID)
	}
	if err := identity.checkBackendGeneratedIDs(dashboard, wantWidgets); err != nil {
		return err
	}
	return identity.checkFlatGeneratedIDs(resourceState.Primary.Attributes, wantWidgets)
}

func (identity *dashboardMigrationIdentity) checkBackendGeneratedIDs(dashboard *dashboardservice.Dashboard, wantWidgets int) error {
	generatedIDs, err := dashboardMigrationGeneratedIDs(dashboard, wantWidgets)
	if err != nil {
		return fmt.Errorf("dashboard fixture %q: %w", identity.fixture, err)
	}
	if identity.generatedIDs == nil {
		identity.generatedIDs = generatedIDs
		return nil
	}
	for i, got := range generatedIDs {
		if got != identity.generatedIDs[i] {
			return fmt.Errorf("dashboard fixture %q: generated nested ID %d changed from %q to %q", identity.fixture, i, identity.generatedIDs[i], got)
		}
	}
	return nil
}

func (identity *dashboardMigrationIdentity) checkFlatGeneratedIDs(attributes map[string]string, wantWidgets int) error {
	paths := []string{"layout.sections.0.id", "layout.sections.0.rows.0.id"}
	for i := 0; i < wantWidgets; i++ {
		paths = append(paths, fmt.Sprintf("layout.sections.0.rows.0.widgets.%d.id", i))
	}
	if len(identity.generatedIDs) != len(paths) {
		return fmt.Errorf("dashboard fixture %q: captured %d generated IDs, want %d", identity.fixture, len(identity.generatedIDs), len(paths))
	}
	for i, path := range paths {
		if attributes[path] != identity.generatedIDs[i] {
			return fmt.Errorf("dashboard fixture %q: state %s = %q, backend ID = %q", identity.fixture, path, attributes[path], identity.generatedIDs[i])
		}
	}
	return nil
}

func (identity *dashboardMigrationIdentity) checkFolder(
	state *terraform.State,
	resourceState *terraform.ResourceState,
	dashboard *dashboardservice.Dashboard,
	wantFolder bool,
) error {
	if !wantFolder {
		return nil
	}
	folderState, ok := state.RootModule().Resources[folderResourceName]
	if !ok || folderState.Primary == nil || folderState.Primary.ID == "" {
		return fmt.Errorf("dashboard fixture %q: managed folder state is absent", identity.fixture)
	}
	if identity.folderID == "" {
		identity.folderID = folderState.Primary.ID
	} else if folderState.Primary.ID != identity.folderID {
		return fmt.Errorf("dashboard fixture %q: folder ID changed from %q to %q", identity.fixture, identity.folderID, folderState.Primary.ID)
	}
	if resourceState.Primary.Attributes["folder.id"] != identity.folderID {
		return fmt.Errorf("dashboard fixture %q: state folder.id = %q, want %q", identity.fixture, resourceState.Primary.Attributes["folder.id"], identity.folderID)
	}
	if dashboard.FolderId == nil || dashboard.FolderId.GetValue() != identity.folderID {
		backendFolderID := ""
		if dashboard.FolderId != nil {
			backendFolderID = dashboard.FolderId.GetValue()
		}
		return fmt.Errorf("dashboard fixture %q: backend folder ID = %q, want %q", identity.fixture, backendFolderID, identity.folderID)
	}
	return nil
}

func dashboardMigrationGeneratedIDs(dashboard *dashboardservice.Dashboard, wantWidgets int) ([]string, error) {
	layout := dashboard.GetLayout()
	sections := layout.GetSections()
	if len(sections) != 1 {
		return nil, fmt.Errorf("backend sections = %d, want 1", len(sections))
	}
	rows := sections[0].GetRows()
	if len(rows) != 1 {
		return nil, fmt.Errorf("backend rows = %d, want 1", len(rows))
	}
	widgets := rows[0].GetWidgets()
	if len(widgets) != wantWidgets {
		return nil, fmt.Errorf("backend widgets = %d, want %d", len(widgets), wantWidgets)
	}
	if sections[0].Id == nil || rows[0].Id == nil {
		return nil, fmt.Errorf("backend section or row generated ID is absent")
	}
	ids := []string{sections[0].Id.GetValue(), rows[0].Id.GetValue()}
	for i := range widgets {
		if widgets[i].Id == nil {
			return nil, fmt.Errorf("backend widget %d generated ID is absent", i)
		}
		ids = append(ids, widgets[i].Id.GetValue())
	}
	for i, id := range ids {
		if id == "" {
			return nil, fmt.Errorf("backend generated nested ID %d is empty", i)
		}
	}
	return ids, nil
}

func dashboardMigrationCheckAccessPolicy(statePolicy, backendPolicy, wantPolicy, fixture string) error {
	if wantPolicy == "" {
		return nil
	}
	if !utils.JSONStringsEqual(statePolicy, wantPolicy) {
		return fmt.Errorf("dashboard fixture %q: state access policy is not JSON-equivalent to the configured policy", fixture)
	}
	if !utils.JSONStringsEqual(backendPolicy, wantPolicy) {
		return fmt.Errorf("dashboard fixture %q: backend access policy is not JSON-equivalent to the configured policy", fixture)
	}
	return nil
}
