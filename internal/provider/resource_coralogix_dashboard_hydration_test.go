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
	"encoding/json"
	"fmt"
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
		t.Fatal("backend hydration fixture must follow the provider create path and supply nested UUIDs")
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

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			if dashboardID != "" {
				return
			}
			client := dashboardOpenAPIAcceptanceClient(t)
			response, err := dashboardOpenAPICreateDirectFixture(t, client, fixture, dashboardOpenAPIBackendHydrationRequest(dashboardName))
			if err != nil {
				t.Fatal(err)
			}
			dashboardID = response.GetDashboardId()
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
				ImportStateCheck:   dashboardOpenAPIBackendHydrationImportCheck(dashboardName),
			},
			{
				Config:          dashboardOpenAPIBackendHydrationConfig(dashboardName),
				ConfigVariables: variables,
				Check: resource.ComposeAggregateTestCheckFunc(
					dashboardOpenAPIBackendHydrationStateCheck(dashboardResourceName, dashboardName),
					dashboardOpenAPIBackendHydrationStateCheck("data.coralogix_dashboard.backend", dashboardName),
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

func dashboardOpenAPIBackendHydrationImportCheck(name string) resource.ImportStateCheckFunc {
	return func(states []*terraform.InstanceState) error {
		if len(states) != 1 {
			return fmt.Errorf("REST-created dashboard import returned %d states, want 1", len(states))
		}
		return dashboardOpenAPIAssertBackendHydrationAttributes(states[0].Attributes, name)
	}
}

func dashboardOpenAPIBackendHydrationStateCheck(resourceName, name string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		resourceState, ok := state.RootModule().Resources[resourceName]
		if !ok || resourceState.Primary == nil {
			return fmt.Errorf("dashboard state %q is absent", resourceName)
		}
		return dashboardOpenAPIAssertBackendHydrationAttributes(resourceState.Primary.Attributes, name)
	}
}

func dashboardOpenAPIAssertBackendHydrationAttributes(attributes map[string]string, name string) error {
	want := map[string]string{
		"name":                      name,
		"description":               "Created directly through the REST SDK to verify import and data-source hydration.",
		"auto_refresh.type":         "off",
		"time_frame.absolute.start": "2026-03-01T00:00:00Z",
		"time_frame.absolute.end":   "2026-03-01T01:00:00Z",
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
	for _, path := range []string{
		"layout.sections.0.id",
		"layout.sections.0.rows.0.id",
		"layout.sections.0.rows.0.widgets.0.id",
	} {
		if attributes[path] == "" {
			return fmt.Errorf("dashboard state %s is empty; expected a hydrated nested ID", path)
		}
	}
	if attributes["id"] == "" {
		return fmt.Errorf("dashboard state id is empty; expected a backend-generated dashboard ID")
	}
	return nil
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
