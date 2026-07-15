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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	dashboardservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_service"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

const (
	dashboardContentJSONCompatibilityTestName       = "TestAccCoralogixResourceDashboardContentJSONProtobufSpellings"
	dashboardContentJSONDynamicQueriesTableTestName = "TestAccCoralogixResourceDashboardContentJSONDynamicQueriesTable"
)

var dashboardContentJSONUnknownFields = []string{
	"unknownRoot",
	"unknownLayout",
	"unknownSection",
	"unknownRow",
	"unknownWidget",
	"unknownDefinition",
	"unknownDataTable",
	"unknownQuery",
	"unknownMetrics",
	"unknownPromqlQuery",
}

var dashboardContentJSONUUIDPattern = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)

type dashboardContentJSONFixture struct {
	path    string
	content string
}

func TestAccCoralogixResourceDashboardContentJSONProtobufSpellings(t *testing.T) {
	ctx := context.Background()
	var client *dashboardservice.DashboardServiceAPIService
	fixture := dashboardContentJSONCompatibilityTestName
	dashboardName := dashboardOpenAPIFixtureName(fixture)
	identity := newDashboardOpenAPIIDTracker(dashboardResourceName, fixture)
	snakeCase := dashboardContentJSONNamedFixtureFor(t, "content_json_snake_case.json", dashboardName)
	canonical := dashboardContentJSONNamedFixtureFor(t, "content_json_canonical.json", dashboardName)
	unknownFields := dashboardContentJSONNamedFixtureFor(t, "content_json_unknown_fields.json", dashboardName)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			client = dashboardOpenAPIAcceptanceClient(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDashboardDestroy(t),
		Steps: []resource.TestStep{
			{
				Config: dashboardContentJSONConfig(snakeCase.path),
				Check: resource.ComposeAggregateTestCheckFunc(
					identity.Capture(),
					resource.TestCheckResourceAttr(dashboardResourceName, "content_json", snakeCase.content),
					dashboardContentJSONCheckDashboard(ctx, &client, fixture, func(dashboard *dashboardservice.Dashboard) error {
						return dashboardOpenAPIAssertContentJSONTransport(dashboard, fixture)
					}),
				),
				ConfigPlanChecks: dashboardContentJSONPlanChecks(false),
			},
			{
				Config: dashboardContentJSONConfig(canonical.path),
				Check: resource.ComposeAggregateTestCheckFunc(
					dashboardContentJSONAssertReplaced(identity),
					resource.TestCheckResourceAttr(dashboardResourceName, "content_json", canonical.content),
					dashboardContentJSONCheckDashboard(ctx, &client, fixture, func(dashboard *dashboardservice.Dashboard) error {
						return dashboardOpenAPIAssertContentJSONTransport(dashboard, fixture)
					}),
				),
				ConfigPlanChecks: dashboardContentJSONPlanChecks(true),
			},
			{
				Config: dashboardContentJSONConfig(unknownFields.path),
				Check: resource.ComposeAggregateTestCheckFunc(
					dashboardContentJSONAssertReplaced(identity),
					resource.TestCheckResourceAttr(dashboardResourceName, "content_json", unknownFields.content),
					dashboardContentJSONCheckDashboard(ctx, &client, fixture, func(dashboard *dashboardservice.Dashboard) error {
						if err := dashboardOpenAPIAssertContentJSONTransport(dashboard, fixture); err != nil {
							return err
						}
						return dashboardOpenAPIAssertUnknownContentJSONFieldsDiscarded(dashboard, fixture)
					}),
				),
				ConfigPlanChecks: dashboardContentJSONPlanChecks(true),
			},
		},
	})
}

func TestAccCoralogixResourceDashboardContentJSONFolderOverride(t *testing.T) {
	ctx := context.Background()
	var client *dashboardservice.DashboardServiceAPIService
	fixture := "TestAccCoralogixResourceDashboardContentJSONFolderOverride"
	dashboardName := dashboardOpenAPIFixtureName(fixture)
	folderName := dashboardOpenAPIFixtureName(fixture + "-folder")
	canonical := dashboardContentJSONNamedFixtureFor(t, "content_json_canonical.json", dashboardName)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			client = dashboardOpenAPIAcceptanceClient(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDashboardDestroy(t),
		Steps: []resource.TestStep{
			{
				Config: dashboardContentJSONFolderOverrideConfig(canonical.path, folderName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dashboardResourceName, "content_json", canonical.content),
					resource.TestCheckResourceAttrSet(dashboardResourceName, "folder.id"),
					dashboardContentJSONCheckDashboard(ctx, &client, fixture, func(dashboard *dashboardservice.Dashboard) error {
						return dashboardOpenAPIAssertContentJSONTransport(dashboard, fixture)
					}),
					dashboardOpenAPICheckContentJSONFolderOverride(ctx, &client, fixture),
				),
				ConfigPlanChecks: dashboardContentJSONPlanChecks(false),
			},
		},
	})
}

func TestAccCoralogixResourceDashboardContentJSONDynamicQueriesTable(t *testing.T) {
	ctx := context.Background()
	var client *dashboardservice.DashboardServiceAPIService
	fixture := dashboardContentJSONDynamicQueriesTableTestName
	identity := newDashboardOpenAPIIDTracker(dashboardResourceName, fixture)
	dashboardName := dashboardOpenAPIFixtureName(fixture)
	dynamic := dashboardContentJSONUniqueUUIDsFixtureFor(t, "content_json_dynamic_queries_table.json")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			client = dashboardOpenAPIAcceptanceClient(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDashboardDestroy(t),
		Steps: []resource.TestStep{
			{
				Config: dashboardContentJSONDynamicConfig(dynamic.path, dashboardName, ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					identity.Capture(),
					resource.TestCheckResourceAttrSet(dashboardResourceName, "content_json"),
					dashboardContentJSONCheckDashboard(ctx, &client, fixture, func(dashboard *dashboardservice.Dashboard) error {
						return dashboardOpenAPIAssertDynamicQueriesTable(dashboard, fixture)
					}),
				),
				ConfigPlanChecks: dashboardContentJSONPlanChecks(false),
			},
			{
				Config: dashboardContentJSONDynamicConfig(dynamic.path, dashboardName, testAccCoralogixDashboardAccessPolicyPretty()),
				Check: resource.ComposeAggregateTestCheckFunc(
					identity.AssertUnchanged(),
					resource.TestCheckResourceAttrSet(dashboardResourceName, "access_policy"),
					dashboardContentJSONCheckDashboard(ctx, &client, fixture, func(dashboard *dashboardservice.Dashboard) error {
						return dashboardOpenAPIAssertDynamicQueriesTable(dashboard, fixture)
					}),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply:             []plancheck.PlanCheck{plancheck.ExpectResourceAction(dashboardResourceName, plancheck.ResourceActionUpdate)},
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

func dashboardContentJSONFixtureFor(t *testing.T, name string) dashboardContentJSONFixture {
	t.Helper()

	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("get dashboard test working directory: %s", err)
	}
	fixturePath := filepath.Join(workingDirectory, "testdata", "dashboards", name)
	content, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read dashboard content_json fixture %q: %s", name, err)
	}

	return dashboardContentJSONFixture{path: fixturePath, content: string(content)}
}

func dashboardContentJSONNamedFixtureFor(t *testing.T, name, dashboardName string) dashboardContentJSONFixture {
	t.Helper()
	fixture := dashboardContentJSONFixtureFor(t, name)
	var metadata struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal([]byte(fixture.content), &metadata); err != nil {
		t.Fatalf("decode dashboard content_json fixture %q: %s", name, err)
	}
	if metadata.Name == "" {
		t.Fatalf("dashboard content_json fixture %q has no name", name)
	}

	oldName := fmt.Sprintf("\"name\": %q", metadata.Name)
	newName := fmt.Sprintf("\"name\": %q", dashboardName)
	content := strings.Replace(fixture.content, oldName, newName, 1)
	if content == fixture.content {
		t.Fatalf("replace dashboard name in content_json fixture %q", name)
	}
	fixturePath := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(fixturePath, []byte(content), 0o600); err != nil {
		t.Fatalf("write dashboard content_json fixture %q: %s", name, err)
	}

	return dashboardContentJSONFixture{path: fixturePath, content: content}
}

func dashboardContentJSONUniqueUUIDsFixtureFor(t *testing.T, name string) dashboardContentJSONFixture {
	t.Helper()
	fixture := dashboardContentJSONFixtureFor(t, name)
	content := dashboardContentJSONUUIDPattern.ReplaceAllStringFunc(fixture.content, func(string) string {
		return uuid.NewString()
	})
	if content == fixture.content {
		t.Fatalf("dashboard content_json fixture %q has no UUIDs to randomize", name)
	}
	fixturePath := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(fixturePath, []byte(content), 0o600); err != nil {
		t.Fatalf("write dashboard content_json fixture %q with unique UUIDs: %s", name, err)
	}

	return dashboardContentJSONFixture{path: fixturePath, content: content}
}

func dashboardContentJSONConfig(fixturePath string) string {
	return fmt.Sprintf(`
resource "coralogix_dashboard" "test" {
  content_json = file(%q)
}
`, fixturePath)
}

func dashboardContentJSONFolderOverrideConfig(fixturePath, folderName string) string {
	return fmt.Sprintf(`
resource "coralogix_dashboards_folder" "test_folder" {
  name = %q
}

resource "coralogix_dashboard" "test" {
  content_json = file(%q)
  folder = {
    id = coralogix_dashboards_folder.test_folder.id
  }
}
`, folderName, fixturePath)
}

func dashboardContentJSONDynamicConfig(fixturePath, dashboardName, accessPolicy string) string {
	accessPolicyBlock := ""
	if accessPolicy != "" {
		accessPolicyBlock = fmt.Sprintf("  access_policy = <<EOT\n%s\nEOT\n", accessPolicy)
	}
	return fmt.Sprintf(`
resource "coralogix_dashboard" "test" {
  content_json = jsonencode(merge(jsondecode(file(%q)), { name = %q }))
%s}
`, fixturePath, dashboardName, accessPolicyBlock)
}

func dashboardContentJSONPlanChecks(expectReplacement bool) resource.ConfigPlanChecks {
	checks := resource.ConfigPlanChecks{
		PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
	}
	if expectReplacement {
		checks.PreApply = []plancheck.PlanCheck{
			plancheck.ExpectResourceAction(dashboardResourceName, plancheck.ResourceActionReplace),
		}
	}

	return checks
}

func dashboardContentJSONAssertReplaced(identity *dashboardOpenAPIIDTracker) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		id, err := dashboardOpenAPIResourceID(state, identity.resourceName, identity.fixture)
		if err != nil {
			return err
		}
		if identity.id == "" {
			return fmt.Errorf("dashboard fixture %q (dashboard %q): resource ID was not captured before replacement", identity.fixture, id)
		}
		if id == identity.id {
			return fmt.Errorf("dashboard fixture %q: resource ID did not change across required replacement: got %q", identity.fixture, id)
		}
		identity.id = id

		return nil
	}
}

func dashboardContentJSONCheckDashboard(
	ctx context.Context,
	client **dashboardservice.DashboardServiceAPIService,
	fixture string,
	check func(*dashboardservice.Dashboard) error,
) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		if client == nil {
			return fmt.Errorf("dashboard fixture %q: OpenAPI client reference is nil", fixture)
		}
		dashboard, err := dashboardOpenAPIFetchDashboard(ctx, *client, state, dashboardResourceName, fixture)
		if err != nil {
			return err
		}
		if check == nil {
			return fmt.Errorf("dashboard fixture %q (dashboard %q): check callback is nil", fixture, dashboard.GetId())
		}

		return check(dashboard)
	}
}

func dashboardOpenAPIAssertContentJSONTransport(dashboard *dashboardservice.Dashboard, fixture string) error {
	if dashboard == nil {
		return fmt.Errorf("dashboard fixture %q: REST read returned no dashboard", fixture)
	}
	if dashboard.RelativeTimeFrame == nil || *dashboard.RelativeTimeFrame != "900s" {
		return fmt.Errorf("dashboard fixture %q (dashboard %q): relativeTimeFrame = %v, want protobuf JSON duration 900s", fixture, dashboard.GetId(), dashboard.RelativeTimeFrame)
	}
	if len(dashboard.Layout.Sections) != 1 || len(dashboard.Layout.Sections[0].Rows) != 1 || len(dashboard.Layout.Sections[0].Rows[0].Widgets) != 1 {
		return fmt.Errorf("dashboard fixture %q (dashboard %q): REST layout does not contain exactly one section, row, and widget", fixture, dashboard.GetId())
	}

	widget := dashboard.Layout.Sections[0].Rows[0].Widgets[0]
	if widget.LayoutColumns == nil || *widget.LayoutColumns != 12 {
		return fmt.Errorf("dashboard fixture %q (dashboard %q): layoutColumns = %v, want 12", fixture, dashboard.GetId(), widget.LayoutColumns)
	}
	if widget.Definition == nil {
		return fmt.Errorf("dashboard fixture %q (dashboard %q): widget definition is nil", fixture, dashboard.GetId())
	}
	if err := dashboardOpenAPIAssertOneOfBranch(widget.Definition, "WidgetDefinition", "dataTable", dashboard.GetId(), fixture); err != nil {
		return err
	}

	dataTable := widget.Definition.DataTable
	if dataTable.ResultsPerPage == nil || *dataTable.ResultsPerPage != 10 {
		return fmt.Errorf("dashboard fixture %q (dashboard %q): dataTable.resultsPerPage = %v, want 10", fixture, dashboard.GetId(), dataTable.ResultsPerPage)
	}
	if dataTable.RowStyle == nil || *dataTable.RowStyle != dashboardservice.ROWSTYLE_ROW_STYLE_ONE_LINE {
		return fmt.Errorf("dashboard fixture %q (dashboard %q): dataTable.rowStyle = %v, want ROW_STYLE_ONE_LINE", fixture, dashboard.GetId(), dataTable.RowStyle)
	}
	if dataTable.Query == nil {
		return fmt.Errorf("dashboard fixture %q (dashboard %q): dataTable.query is nil", fixture, dashboard.GetId())
	}
	if err := dashboardOpenAPIAssertOneOfBranch(dataTable.Query, "DataTableQuery", "metrics", dashboard.GetId(), fixture); err != nil {
		return err
	}
	if dataTable.Query.Metrics.PromqlQuery == nil || dataTable.Query.Metrics.PromqlQuery.GetValue() != "vector(1)" {
		return fmt.Errorf("dashboard fixture %q (dashboard %q): typed dataTable metrics promqlQuery was not hydrated", fixture, dashboard.GetId())
	}

	return nil
}

func dashboardOpenAPIAssertDynamicQueriesTable(dashboard *dashboardservice.Dashboard, fixture string) error {
	if dashboard == nil {
		return fmt.Errorf("dashboard fixture %q: REST read returned no dashboard", fixture)
	}
	if len(dashboard.Layout.Sections) != 1 || len(dashboard.Layout.Sections[0].Rows) != 3 {
		return fmt.Errorf("dashboard fixture %q (dashboard %q): REST layout does not contain exactly one section with three rows", fixture, dashboard.GetId())
	}

	for rowIndex, queryBranch := range []string{"logs", "metrics", "spans"} {
		row := dashboard.Layout.Sections[0].Rows[rowIndex]
		if len(row.Widgets) != 1 || row.Widgets[0].Definition == nil {
			return fmt.Errorf("dashboard fixture %q (dashboard %q): row %d does not contain exactly one typed widget", fixture, dashboard.GetId(), rowIndex)
		}
		definition := row.Widgets[0].Definition
		if err := dashboardOpenAPIAssertOneOfBranch(definition, "WidgetDefinition", "dynamic", dashboard.GetId(), fixture); err != nil {
			return err
		}
		dynamic := definition.Dynamic
		if dynamic == nil || len(dynamic.QueryDefinitions) != 1 {
			return fmt.Errorf("dashboard fixture %q (dashboard %q): row %d dynamic queryDefinitions count = %d, want 1", fixture, dashboard.GetId(), rowIndex, len(dynamic.GetQueryDefinitions()))
		}
		query := &dynamic.QueryDefinitions[0].Query
		if err := dashboardOpenAPIAssertOneOfBranch(query, "DynamicQuery", queryBranch, dashboard.GetId(), fixture); err != nil {
			return err
		}
		if dynamic.Visualization == nil {
			return fmt.Errorf("dashboard fixture %q (dashboard %q): row %d dynamic visualization is nil", fixture, dashboard.GetId(), rowIndex)
		}
		if err := dashboardOpenAPIAssertOneOfBranch(dynamic.Visualization, "Visualization", "table", dashboard.GetId(), fixture); err != nil {
			return err
		}
	}

	return nil
}

func dashboardOpenAPIAssertUnknownContentJSONFieldsDiscarded(dashboard *dashboardservice.Dashboard, fixture string) error {
	encoded, err := json.Marshal(dashboard)
	if err != nil {
		return fmt.Errorf("dashboard fixture %q (dashboard %q): marshal REST dashboard: %w", fixture, dashboard.GetId(), err)
	}
	for _, field := range dashboardContentJSONUnknownFields {
		if strings.Contains(string(encoded), fmt.Sprintf("%q", field)) {
			return fmt.Errorf("dashboard fixture %q (dashboard %q): unknown property %q survived in REST AdditionalProperties", fixture, dashboard.GetId(), field)
		}
	}

	return nil
}

func dashboardOpenAPICheckContentJSONFolderOverride(
	ctx context.Context,
	client **dashboardservice.DashboardServiceAPIService,
	fixture string,
) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		if client == nil {
			return fmt.Errorf("dashboard fixture %q: OpenAPI client reference is nil", fixture)
		}
		dashboard, err := dashboardOpenAPIFetchDashboard(ctx, *client, state, dashboardResourceName, fixture)
		if err != nil {
			return err
		}
		folderState, ok := state.RootModule().Resources[folderResourceName]
		if !ok || folderState.Primary == nil || folderState.Primary.ID == "" {
			return fmt.Errorf("dashboard fixture %q: managed folder state is absent", fixture)
		}
		if dashboard.FolderId == nil || dashboard.FolderId.GetValue() != folderState.Primary.ID {
			return fmt.Errorf("dashboard fixture %q (dashboard %q): REST folder ID = %v, want %q", fixture, dashboard.GetId(), dashboard.FolderId, folderState.Primary.ID)
		}

		return nil
	}
}
