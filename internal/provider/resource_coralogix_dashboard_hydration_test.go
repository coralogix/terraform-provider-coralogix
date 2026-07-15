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
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	dashboardservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_service"
	"github.com/google/uuid"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

const dashboardOpenAPIBackendHydrationTestName = "TestAccCoralogixResourceDashboardRESTCreatedHydration"

type dashboardOpenAPIHydrationIDs struct {
	BackendGeneratedDashboard string
	ClientSuppliedSection     string
	ClientSuppliedRow         string
	ClientSuppliedWidget      string
}

var dashboardOpenAPIBackendHydrationClaims = []struct {
	model  string
	branch string
}{
	{model: "Dashboard", branch: "absoluteTimeFrame"},
	{model: "Dashboard", branch: "off"},
	{model: "GaugeQuery", branch: "metrics"},
	{model: "TimeFrameSelect", branch: "relativeTimeFrame"},
	{model: "WidgetDefinition", branch: "gauge"},
}

func TestDashboardRESTCreatedHydrationClaimsMatchManifest(t *testing.T) {
	for _, claim := range dashboardOpenAPIBackendHydrationClaims {
		coverage := dashboardOpenAPIOneOfCoverage[claim.model].Branches[claim.branch]
		if coverage.FixtureOrTest != dashboardOpenAPIBackendHydrationTestName {
			t.Errorf("%s.%s fixture = %q, want %q", claim.model, claim.branch, coverage.FixtureOrTest, dashboardOpenAPIBackendHydrationTestName)
		}
		if !coverage.ImportHydration || !coverage.DataSourceHydration {
			t.Errorf("%s.%s hydration = import:%t data-source:%t, want both true", claim.model, claim.branch, coverage.ImportHydration, coverage.DataSourceHydration)
		}
	}
}

func TestDashboardRESTCreatedHydrationRequestCoversBackendReadEdges(t *testing.T) {
	request := dashboardOpenAPIBackendHydrationRequest("fixture")
	dashboard := request.Dashboard

	if dashboard.Off == nil || dashboard.AbsoluteTimeFrame == nil {
		t.Fatal("backend hydration fixture must cover off auto-refresh and an absolute dashboard time frame")
	}
	if dashboard.Variables == nil || dashboard.Filters == nil || dashboard.Annotations == nil || len(dashboard.Variables) != 0 || len(dashboard.Filters) != 0 || len(dashboard.Annotations) != 0 {
		t.Fatal("backend hydration fixture must send empty supported dashboard collections")
	}
	if dashboard.Id != nil {
		t.Fatal("backend hydration fixture must omit the dashboard ID so the backend generates it")
	}
	section := dashboard.Layout.Sections[0]
	row := section.Rows[0]
	widget := row.Widgets[0]
	if section.Id == nil || section.Id.Value == nil || row.Id == nil || row.Id.Value == nil || widget.Id == nil || widget.Id.Value == nil {
		t.Fatal("backend hydration fixture must supply the section, row, and widget IDs required by the backend")
	}
	requestIDs, err := dashboardOpenAPIRequestHydrationIDs(request)
	if err != nil {
		t.Fatalf("capture hydration request IDs: %s", err)
	}
	if requestIDs.BackendGeneratedDashboard != "" || requestIDs.ClientSuppliedSection != *section.Id.Value || requestIDs.ClientSuppliedRow != *row.Id.Value || requestIDs.ClientSuppliedWidget != *widget.Id.Value {
		t.Fatalf("captured hydration request IDs = %#v, want an omitted dashboard ID and exact client-supplied nested IDs", requestIDs)
	}
	gauge := widget.Definition.Gauge
	metrics := gauge.Query.Metrics
	if metrics.TimeFrame == nil || metrics.TimeFrame.RelativeTimeFrame == nil {
		t.Fatal("backend hydration fixture must cover a relative query-level time frame")
	}
	if metrics.Aggregation != nil || metrics.EditorMode != nil || metrics.PromqlQueryType != nil || gauge.DataModeType != nil || gauge.ThresholdBy != nil || gauge.ThresholdType != nil {
		t.Fatal("backend hydration fixture must omit supported optional enums")
	}
	if metrics.Filters == nil || gauge.Thresholds == nil || len(metrics.Filters) != 0 || len(gauge.Thresholds) != 0 {
		t.Fatal("backend hydration fixture must send empty nested collections")
	}

	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("marshal backend hydration REST request: %s", err)
	}
	if count := strings.Count(string(encoded), `"id":`); count != 3 {
		t.Errorf("backend hydration REST request contains %d nested IDs, want exactly 3: %s", count, encoded)
	}
	for _, nestedID := range []string{*section.Id.Value, *row.Id.Value, *widget.Id.Value} {
		if !strings.Contains(string(encoded), nestedID) {
			t.Errorf("backend hydration REST request does not contain client-supplied nested ID %q: %s", nestedID, encoded)
		}
	}
	for _, emptyCollection := range []string{`"variables":[]`, `"annotations":[]`, `"filters":[]`, `"thresholds":[]`} {
		if !strings.Contains(string(encoded), emptyCollection) {
			t.Errorf("backend hydration REST request %s does not contain %s", encoded, emptyCollection)
		}
	}
	for _, omittedEnum := range []string{`"aggregation"`, `"editorMode"`, `"promqlQueryType"`, `"dataModeType"`, `"thresholdBy"`, `"thresholdType"`} {
		if strings.Contains(string(encoded), omittedEnum) {
			t.Errorf("backend hydration REST request unexpectedly contains optional enum %s: %s", omittedEnum, encoded)
		}
	}
}

func TestDashboardRESTCreatedHydrationConfigParses(t *testing.T) {
	configs := map[string]string{
		"structured import": dashboardOpenAPIBackendHydrationResourceConfig("dashboard"),
		"structured read":   dashboardOpenAPIBackendHydrationConfig("dashboard"),
		"dynamic import":    dashboardOpenAPIUnsupportedDynamicImportConfig(),
	}
	for name, config := range configs {
		_, diagnostics := hclsyntax.ParseConfig([]byte(config), name+".tf", hcl.InitialPos)
		if diagnostics.HasErrors() {
			t.Errorf("parse %s config: %s", name, diagnostics.Error())
		}
	}
}

func TestAccCoralogixResourceDashboardRESTCreatedHydration(t *testing.T) {
	const fixture = dashboardOpenAPIBackendHydrationTestName
	dashboardName := dashboardOpenAPIFixtureName(fixture)
	variables := config.Variables{}
	var dashboardID string
	var hydrationIDs dashboardOpenAPIHydrationIDs

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			if dashboardID != "" {
				return
			}
			client := dashboardOpenAPIAcceptanceClient(t)
			dashboardOpenAPIAssertNestedIDsRequired(t, client, fixture)
			request := dashboardOpenAPIBackendHydrationRequest(dashboardName)
			clientSuppliedIDs, err := dashboardOpenAPIRequestHydrationIDs(request)
			if err != nil {
				t.Fatalf("dashboard fixture %q: capture client-supplied nested IDs: %s", fixture, err)
			}
			response, err := dashboardOpenAPICreateDirectFixture(t, client, fixture, request)
			if err != nil {
				t.Fatal(err)
			}
			dashboardID = response.GetDashboardId()
			dashboard, err := dashboardOpenAPIFetchDashboardByID(context.Background(), client, dashboardID, fixture)
			if err != nil {
				t.Fatal(err)
			}
			hydrationIDs, err = dashboardOpenAPIReadHydrationIDs(dashboard, clientSuppliedIDs)
			if err != nil {
				t.Fatalf("dashboard fixture %q: verify hydrated IDs: %s", fixture, err)
			}
			if hydrationIDs.BackendGeneratedDashboard != dashboardID {
				t.Fatalf("dashboard fixture %q: direct read dashboard ID = %q, create response ID = %q", fixture, hydrationIDs.BackendGeneratedDashboard, dashboardID)
			}
			variables["dashboard_id"] = config.StringVariable(dashboardID)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:             dashboardOpenAPIBackendHydrationResourceConfig(dashboardName),
				ResourceName:       dashboardResourceName,
				ImportState:        true,
				ImportStatePersist: true,
				ImportStateIdFunc:  dashboardOpenAPIImportID(&dashboardID, fixture),
				ImportStateCheck:   dashboardOpenAPIBackendHydrationImportCheck(dashboardName, &hydrationIDs),
			},
			{
				Config:          dashboardOpenAPIBackendHydrationConfig(dashboardName),
				ConfigVariables: variables,
				Check: resource.ComposeAggregateTestCheckFunc(
					dashboardOpenAPIBackendHydrationStateCheck(dashboardResourceName, dashboardName, &hydrationIDs),
					dashboardOpenAPIBackendHydrationStateCheck("data.coralogix_dashboard.backend", dashboardName, &hydrationIDs),
					dashboardOpenAPICompareResourceAndDataSourceState,
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(dashboardResourceName, plancheck.ResourceActionNoop),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

func TestAccCoralogixResourceDashboardRESTCreatedUnsupportedDynamicHydration(t *testing.T) {
	const fixture = "TestAccCoralogixResourceDashboardRESTCreatedUnsupportedDynamicHydration"
	dashboardName := dashboardOpenAPIFixtureName(fixture)
	variables := config.Variables{}
	var dashboardID string
	diagnostic := regexp.MustCompile(`(?s)Unsupported Dashboard Widget Definition.*dynamic.*content_json.*import.*data-source`)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			if dashboardID != "" {
				return
			}
			client := dashboardOpenAPIAcceptanceClient(t)
			request := dashboardOpenAPIUnsupportedDynamicRequest(t, dashboardName)
			response, err := dashboardOpenAPICreateDirectFixture(t, client, fixture, request)
			if err != nil {
				t.Fatal(err)
			}
			dashboardID = response.GetDashboardId()
			variables["dashboard_id"] = config.StringVariable(dashboardID)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:             dashboardOpenAPIUnsupportedDynamicImportConfig(),
				ResourceName:       dashboardResourceName,
				ImportState:        true,
				ImportStatePersist: true,
				ImportStateIdFunc:  dashboardOpenAPIImportID(&dashboardID, fixture),
				ExpectError:        diagnostic,
			},
			{
				Config: `
variable "dashboard_id" {
  type = string
}

data "coralogix_dashboard" "backend" {
  id = var.dashboard_id
}
`,
				ConfigVariables: variables,
				ExpectError:     diagnostic,
			},
		},
	})
}

func dashboardOpenAPIBackendHydrationRequest(name string) dashboardservice.CreateDashboardRequestDataStructure {
	start := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	relativeTimeFrame := "900s"
	description := "Created directly through the REST SDK to verify import and data-source hydration."
	height := int32(19)
	minValue := float64(0)
	maxValue := float64(100)
	showInnerArc := false
	showOuterArc := true
	displaySeriesName := true
	unit := dashboardservice.GAUGEUNIT_UNIT_NUMBER
	promql := "vector(1)"
	title := "REST-created gauge"
	sectionID := uuid.NewString()
	rowID := uuid.NewString()
	widgetID := uuid.NewString()

	dashboard := dashboardservice.Dashboard{
		Name:              name,
		Description:       &description,
		AbsoluteTimeFrame: &dashboardservice.TimeFrame{From: &start, To: &end},
		Off:               map[string]interface{}{},
		Variables:         []dashboardservice.Variable{},
		Filters:           []dashboardservice.FiltersFilter{},
		Annotations:       []dashboardservice.Annotation{},
		Layout: dashboardservice.Layout{Sections: []dashboardservice.Section{{
			Id: &dashboardservice.UUID{Value: &sectionID},
			Rows: []dashboardservice.Row{{
				Id:         &dashboardservice.UUID{Value: &rowID},
				Appearance: &dashboardservice.RowAppearance{Height: &height},
				Widgets: []dashboardservice.Widget{{
					Id:    &dashboardservice.UUID{Value: &widgetID},
					Title: &title,
					Definition: &dashboardservice.WidgetDefinition{Gauge: &dashboardservice.WidgetsGauge{
						Query: &dashboardservice.GaugeQuery{Metrics: &dashboardservice.GaugeMetricsQuery{
							PromqlQuery: &dashboardservice.PromQlQuery{Value: &promql},
							Filters:     []dashboardservice.MetricsFilter{},
							TimeFrame:   &dashboardservice.TimeFrameSelect{RelativeTimeFrame: &relativeTimeFrame},
						}},
						Min:               &minValue,
						Max:               &maxValue,
						ShowInnerArc:      &showInnerArc,
						ShowOuterArc:      &showOuterArc,
						DisplaySeriesName: &displaySeriesName,
						Thresholds:        []dashboardservice.GaugeThreshold{},
						Unit:              &unit,
					}},
				}},
			}},
		}}},
	}

	return dashboardservice.CreateDashboardRequestDataStructure{
		Dashboard: dashboard,
		RequestId: dashboardOpenAPIHydrationRequestID(),
	}
}

// dashboardOpenAPIAssertNestedIDsRequired records the live REST contract that
// distinguishes the client-supplied nested UUIDs from the backend-generated
// dashboard ID. Cleanup is registered before every create attempt so an
// unexpected or partial success is still recoverable by its exact unique name.
func dashboardOpenAPIAssertNestedIDsRequired(
	t *testing.T,
	client *dashboardservice.DashboardServiceAPIService,
	fixture string,
) {
	t.Helper()
	tests := []struct {
		name string
		omit func(*dashboardservice.Dashboard)
	}{
		{
			name: "section-id",
			omit: func(dashboard *dashboardservice.Dashboard) {
				dashboard.Layout.Sections[0].Id = nil
			},
		},
		{
			name: "row-id",
			omit: func(dashboard *dashboardservice.Dashboard) {
				dashboard.Layout.Sections[0].Rows[0].Id = nil
			},
		},
		{
			name: "widget-id",
			omit: func(dashboard *dashboardservice.Dashboard) {
				dashboard.Layout.Sections[0].Rows[0].Widgets[0].Id = nil
			},
		},
	}

	for _, test := range tests {
		caseFixture := fixture + "/missing-" + test.name
		request := dashboardOpenAPIBackendHydrationRequest(dashboardOpenAPIFixtureName(caseFixture))
		test.omit(&request.Dashboard)
		dashboardID := ""
		dashboardOpenAPIRegisterDirectCleanupByName(t, client, caseFixture, request.Dashboard.Name, &dashboardID)

		response, httpResponse, err := client.DashboardsServiceCreateDashboard(context.Background()).
			CreateDashboardRequestDataStructure(request).
			Execute()
		if response != nil {
			dashboardID = response.GetDashboardId()
		}
		if err == nil {
			t.Fatalf("dashboard fixture %q: backend accepted a create request without the required %s", caseFixture, test.name)
		}
		if httpResponse == nil || httpResponse.StatusCode != http.StatusBadRequest {
			t.Fatal(dashboardOpenAPISafeRequestError("required nested ID contract", caseFixture, dashboardID, httpResponse, err))
		}
	}
}

func dashboardOpenAPIUnsupportedDynamicRequest(t *testing.T, name string) dashboardservice.CreateDashboardRequestDataStructure {
	t.Helper()
	fixture := dashboardContentJSONFixtureFor(t, "content_json_dynamic_queries_table.json")
	var dashboard dashboardservice.Dashboard
	if err := json.Unmarshal([]byte(fixture.content), &dashboard); err != nil {
		t.Fatalf("unmarshal unsupported dynamic dashboard fixture: %s", err)
	}
	dashboard.Name = name
	dashboard.Id = nil
	for sectionIndex := range dashboard.Layout.Sections {
		section := &dashboard.Layout.Sections[sectionIndex]
		sectionID := uuid.NewString()
		section.Id = &dashboardservice.UUID{Value: &sectionID}
		for rowIndex := range section.Rows {
			row := &section.Rows[rowIndex]
			rowID := uuid.NewString()
			row.Id = &dashboardservice.UUID{Value: &rowID}
			for widgetIndex := range row.Widgets {
				widgetID := uuid.NewString()
				row.Widgets[widgetIndex].Id = &dashboardservice.UUID{Value: &widgetID}
			}
		}
	}

	return dashboardservice.CreateDashboardRequestDataStructure{
		Dashboard: dashboard,
		RequestId: dashboardOpenAPIHydrationRequestID(),
	}
}

func dashboardOpenAPIHydrationRequestID() string {
	return "terraform-provider-coralogix-dashboard-Create-" + uuid.NewString()
}

func dashboardOpenAPIImportID(dashboardID *string, fixture string) resource.ImportStateIdFunc {
	return func(*terraform.State) (string, error) {
		if dashboardID == nil || *dashboardID == "" {
			return "", fmt.Errorf("dashboard fixture %q: direct create did not provide an import ID", fixture)
		}
		return *dashboardID, nil
	}
}

func dashboardOpenAPIBackendHydrationImportCheck(name string, hydrationIDs *dashboardOpenAPIHydrationIDs) resource.ImportStateCheckFunc {
	return func(states []*terraform.InstanceState) error {
		if len(states) != 1 {
			return fmt.Errorf("REST-created dashboard import returned %d states, want 1", len(states))
		}
		return dashboardOpenAPIAssertBackendHydrationAttributes(states[0].Attributes, name, hydrationIDs)
	}
}

func dashboardOpenAPIBackendHydrationStateCheck(resourceName, name string, hydrationIDs *dashboardOpenAPIHydrationIDs) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		resourceState, ok := state.RootModule().Resources[resourceName]
		if !ok || resourceState.Primary == nil {
			return fmt.Errorf("dashboard state %q is absent", resourceName)
		}
		return dashboardOpenAPIAssertBackendHydrationAttributes(resourceState.Primary.Attributes, name, hydrationIDs)
	}
}

func dashboardOpenAPIAssertBackendHydrationAttributes(attributes map[string]string, name string, hydrationIDs *dashboardOpenAPIHydrationIDs) error {
	if hydrationIDs == nil || hydrationIDs.BackendGeneratedDashboard == "" || hydrationIDs.ClientSuppliedSection == "" || hydrationIDs.ClientSuppliedRow == "" || hydrationIDs.ClientSuppliedWidget == "" {
		return fmt.Errorf("backend-generated dashboard ID and client-supplied nested IDs were not captured from the direct REST request/read pair")
	}
	want := map[string]string{
		"id":                                    hydrationIDs.BackendGeneratedDashboard,
		"name":                                  name,
		"description":                           "Created directly through the REST SDK to verify import and data-source hydration.",
		"auto_refresh.type":                     "off",
		"time_frame.absolute.start":             "2026-03-01T00:00:00Z",
		"time_frame.absolute.end":               "2026-03-01T01:00:00Z",
		"layout.sections.0.id":                  hydrationIDs.ClientSuppliedSection,
		"layout.sections.0.rows.0.id":           hydrationIDs.ClientSuppliedRow,
		"layout.sections.0.rows.0.widgets.0.id": hydrationIDs.ClientSuppliedWidget,
		"layout.sections.0.rows.0.widgets.0.title":                                                       "REST-created gauge",
		"layout.sections.0.rows.0.widgets.0.definition.gauge.query.metrics.promql_query":                 "vector(1)",
		"layout.sections.0.rows.0.widgets.0.definition.gauge.query.metrics.aggregation":                  "unspecified",
		"layout.sections.0.rows.0.widgets.0.definition.gauge.query.metrics.time_frame.relative.duration": "seconds:900",
		"layout.sections.0.rows.0.widgets.0.definition.gauge.data_mode_type":                             "unspecified",
		"layout.sections.0.rows.0.widgets.0.definition.gauge.threshold_by":                               "unspecified",
		"layout.sections.0.rows.0.widgets.0.definition.gauge.threshold_type":                             "unspecified",
	}
	for path, expected := range want {
		if actual := attributes[path]; actual != expected {
			return fmt.Errorf("dashboard state %s = %q, want %q", path, actual, expected)
		}
	}
	for _, path := range []string{
		"variables.#",
		"filters.#",
		"annotations.#",
		"layout.sections.0.rows.0.widgets.0.definition.gauge.query.metrics.filters.#",
		"layout.sections.0.rows.0.widgets.0.definition.gauge.thresholds.#",
	} {
		if count := attributes[path]; count != "" && count != "0" {
			return fmt.Errorf("dashboard state %s = %q, want an empty or null collection", path, count)
		}
	}
	return nil
}

func dashboardOpenAPIRequestHydrationIDs(request dashboardservice.CreateDashboardRequestDataStructure) (dashboardOpenAPIHydrationIDs, error) {
	dashboard := request.Dashboard
	if dashboard.Id != nil {
		return dashboardOpenAPIHydrationIDs{}, fmt.Errorf("direct REST request supplied dashboard ID %q; it must be omitted for backend generation", dashboard.GetId())
	}
	return dashboardOpenAPIReadHydrationIDs(&dashboard, dashboardOpenAPIHydrationIDs{})
}

func dashboardOpenAPIReadHydrationIDs(dashboard *dashboardservice.Dashboard, expected dashboardOpenAPIHydrationIDs) (dashboardOpenAPIHydrationIDs, error) {
	if dashboard == nil {
		return dashboardOpenAPIHydrationIDs{}, fmt.Errorf("REST dashboard is absent")
	}
	if len(dashboard.Layout.Sections) != 1 {
		return dashboardOpenAPIHydrationIDs{}, fmt.Errorf("REST dashboard contains %d sections, want 1", len(dashboard.Layout.Sections))
	}
	section := dashboard.Layout.Sections[0]
	if len(section.Rows) != 1 {
		return dashboardOpenAPIHydrationIDs{}, fmt.Errorf("REST dashboard contains %d rows, want 1", len(section.Rows))
	}
	row := section.Rows[0]
	if len(row.Widgets) != 1 {
		return dashboardOpenAPIHydrationIDs{}, fmt.Errorf("REST dashboard contains %d widgets, want 1", len(row.Widgets))
	}

	ids := dashboardOpenAPIHydrationIDs{
		BackendGeneratedDashboard: dashboard.GetId(),
		ClientSuppliedSection:     dashboardOpenAPIUUIDValue(section.Id),
		ClientSuppliedRow:         dashboardOpenAPIUUIDValue(row.Id),
		ClientSuppliedWidget:      dashboardOpenAPIUUIDValue(row.Widgets[0].Id),
	}
	for kind, id := range map[string]string{
		"section": ids.ClientSuppliedSection,
		"row":     ids.ClientSuppliedRow,
		"widget":  ids.ClientSuppliedWidget,
	} {
		if id == "" {
			return dashboardOpenAPIHydrationIDs{}, fmt.Errorf("REST dashboard contains no %s ID", kind)
		}
	}
	for kind, id := range map[string]string{
		"section": ids.ClientSuppliedSection,
		"row":     ids.ClientSuppliedRow,
		"widget":  ids.ClientSuppliedWidget,
	} {
		if _, err := uuid.Parse(id); err != nil {
			return dashboardOpenAPIHydrationIDs{}, fmt.Errorf("REST dashboard contains invalid %s UUID %q: %w", kind, id, err)
		}
	}
	if expected.ClientSuppliedSection != "" {
		for kind, values := range map[string][2]string{
			"section": {ids.ClientSuppliedSection, expected.ClientSuppliedSection},
			"row":     {ids.ClientSuppliedRow, expected.ClientSuppliedRow},
			"widget":  {ids.ClientSuppliedWidget, expected.ClientSuppliedWidget},
		} {
			if values[0] != values[1] {
				return dashboardOpenAPIHydrationIDs{}, fmt.Errorf("direct REST read %s ID = %q, client supplied %q", kind, values[0], values[1])
			}
		}
	}

	return ids, nil
}

func dashboardOpenAPIUUIDValue(id *dashboardservice.UUID) string {
	if id == nil {
		return ""
	}
	return id.GetValue()
}

func dashboardOpenAPICompareResourceAndDataSourceState(state *terraform.State) error {
	resourceState, resourceOK := state.RootModule().Resources[dashboardResourceName]
	dataSourceState, dataSourceOK := state.RootModule().Resources["data.coralogix_dashboard.backend"]
	if !resourceOK || resourceState.Primary == nil || !dataSourceOK || dataSourceState.Primary == nil {
		return fmt.Errorf("resource/data-source dashboard states are not both present")
	}
	if !reflect.DeepEqual(resourceState.Primary.Attributes, dataSourceState.Primary.Attributes) {
		return fmt.Errorf("imported resource and data-source dashboard states differ")
	}
	return nil
}

func dashboardOpenAPIBackendHydrationConfig(name string) string {
	return dashboardOpenAPIBackendHydrationResourceConfig(name) + `
variable "dashboard_id" {
  type = string
}

data "coralogix_dashboard" "backend" {
  id = var.dashboard_id
}
`
}

func dashboardOpenAPIBackendHydrationResourceConfig(name string) string {
	return fmt.Sprintf(`
resource "coralogix_dashboard" "test" {
  name        = %q
  description = "Created directly through the REST SDK to verify import and data-source hydration."

  time_frame = {
    absolute = {
      start = "2026-03-01T00:00:00Z"
      end   = "2026-03-01T01:00:00Z"
    }
  }

  layout = {
    sections = [{
      rows = [{
        height = 19
        widgets = [{
          title = "REST-created gauge"
          definition = {
            gauge = {
              query = {
                metrics = {
                  promql_query = "vector(1)"
                  time_frame = {
                    relative = {
                      duration = "seconds:900"
                    }
                  }
                }
              }
              unit = "none"
            }
          }
        }]
      }]
    }]
  }
}
`, name)
}

func dashboardOpenAPIUnsupportedDynamicImportConfig() string {
	return `
resource "coralogix_dashboard" "test" {
  content_json = jsonencode({
    name = "Import configuration placeholder"
    layout = {
      sections = []
    }
  })
}
`
}
