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
	"strings"
	"testing"

	dashboardservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_service"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

const (
	dashboardOpenAPINestedPresentationTestName = "TestAccCoralogixResourceDashboardOpenAPINestedPresentationBranches"
	dashboardOpenAPILogsAggregationTestName    = "TestAccCoralogixResourceDashboardOpenAPILogsAggregationBranches"
	dashboardOpenAPISpansAndFiltersTestName    = "TestAccCoralogixResourceDashboardOpenAPISpansAndFilterBranches"
	dashboardOpenAPIVariablesTestName          = "TestAccCoralogixResourceDashboardOpenAPIVariableBranches"
	dashboardOpenAPIAnnotationsTestName        = "TestAccCoralogixResourceDashboardOpenAPIAnnotationBranches"
)

type dashboardOpenAPIPresentationGroup string

const (
	dashboardOpenAPIPresentationAll        dashboardOpenAPIPresentationGroup = ""
	dashboardOpenAPIPresentationResolution dashboardOpenAPIPresentationGroup = "resolution-and-time-frames"
	dashboardOpenAPIPresentationBarCharts  dashboardOpenAPIPresentationGroup = "bar-chart-axes"
	dashboardOpenAPIPresentationHorizontal dashboardOpenAPIPresentationGroup = "horizontal-bar-options"
)

var dashboardOpenAPIPresentationGroups = []dashboardOpenAPIPresentationGroup{
	dashboardOpenAPIPresentationResolution,
	dashboardOpenAPIPresentationBarCharts,
	dashboardOpenAPIPresentationHorizontal,
}

type dashboardOpenAPINestedIDTracker struct {
	fixture string
	ids     []string
}

func newDashboardOpenAPINestedIDTracker(fixture string) *dashboardOpenAPINestedIDTracker {
	return &dashboardOpenAPINestedIDTracker{fixture: fixture}
}

func (tracker *dashboardOpenAPINestedIDTracker) CaptureOrAssert(dashboard *dashboardservice.Dashboard) error {
	var ids []string
	layout := dashboard.GetLayout()
	for sectionIndex, section := range layout.GetSections() {
		if section.Id == nil || section.Id.GetValue() == "" {
			return fmt.Errorf("dashboard fixture %q: REST section %d has no generated ID", tracker.fixture, sectionIndex)
		}
		ids = append(ids, section.Id.GetValue())
		for rowIndex, row := range section.GetRows() {
			if row.Id == nil || row.Id.GetValue() == "" {
				return fmt.Errorf("dashboard fixture %q: REST section %d row %d has no generated ID", tracker.fixture, sectionIndex, rowIndex)
			}
			ids = append(ids, row.Id.GetValue())
			for widgetIndex, widget := range row.GetWidgets() {
				if widget.Id == nil || widget.Id.GetValue() == "" {
					return fmt.Errorf("dashboard fixture %q: REST section %d row %d widget %d has no generated ID", tracker.fixture, sectionIndex, rowIndex, widgetIndex)
				}
				ids = append(ids, widget.Id.GetValue())
			}
		}
	}
	if len(ids) == 0 {
		return fmt.Errorf("dashboard fixture %q: REST layout has no generated nested IDs", tracker.fixture)
	}
	if tracker.ids == nil {
		tracker.ids = ids
		return nil
	}
	if len(ids) != len(tracker.ids) {
		return fmt.Errorf("dashboard fixture %q: REST generated nested IDs = %d, want %d", tracker.fixture, len(ids), len(tracker.ids))
	}
	for index := range ids {
		if ids[index] != tracker.ids[index] {
			return fmt.Errorf("dashboard fixture %q: REST generated nested ID %d changed across in-place update/import: got %q, want %q", tracker.fixture, index, ids[index], tracker.ids[index])
		}
	}
	return nil
}

func TestDashboardOpenAPINestedAcceptanceConfigsParse(t *testing.T) {
	configs := map[string]string{
		"presentation-relative":     dashboardOpenAPIPresentationConfig("dashboard", "folder", "relative", "absolute", "two_minutes", "id"),
		"presentation-absolute":     dashboardOpenAPIPresentationConfig("dashboard", "folder", "absolute", "relative", "five_minutes", "path"),
		"logs-aggregations":         dashboardOpenAPILogsAggregationConfig("dashboard"),
		"logs-aggregations-updated": dashboardOpenAPILogsAggregationUpdateConfig("dashboard"),
		"spans-and-filters":         dashboardOpenAPISpansAndFiltersConfig("dashboard"),
		"spans-and-filters-updated": dashboardOpenAPISpansAndFiltersUpdateConfig("dashboard"),
		"variables":                 dashboardOpenAPIVariablesConfig("dashboard"),
		"variables-updated":         dashboardOpenAPIVariablesUpdateConfig("dashboard"),
		"annotations":               dashboardOpenAPIAnnotationsConfig("dashboard"),
		"annotations-updated":       dashboardOpenAPIAnnotationsUpdateConfig("dashboard"),
		"logs-query-updated":        dashboardOpenAPIStructuredDashboardUpdateConfig("dashboard", "logs", true),
		"metrics-query-updated":     dashboardOpenAPIStructuredDashboardUpdateConfig("dashboard", "metrics", false),
		"spans-query-updated":       dashboardOpenAPIStructuredDashboardUpdateConfig("dashboard", "spans", false),
		"dataprime-filters":         dashboardOpenAPIStructuredDashboardConfig("dashboard", "dataprime", false),
		"dataprime-update":          dashboardOpenAPIStructuredDashboardUpdateConfig("dashboard", "dataprime", false),
		"dynamic-content-json":      dashboardContentJSONDynamicConfig("dashboard.json", "dashboard", ""),
	}
	for _, group := range dashboardOpenAPIPresentationGroups {
		configs["presentation-"+string(group)] = dashboardOpenAPIPresentationConfigForGroup("dashboard", "folder", "relative", "absolute", "two_minutes", "id", group)
	}
	for name, config := range configs {
		t.Run(name, func(t *testing.T) {
			_, diagnostics := hclsyntax.ParseConfig([]byte(config), name+".tf", hcl.InitialPos)
			if diagnostics.HasErrors() {
				t.Fatalf("nested acceptance config is invalid HCL:\n%s", diagnostics.Error())
			}
		})
	}
}

func TestAccCoralogixResourceDashboardOpenAPINestedPresentationBranches(t *testing.T) {
	t.Parallel()

	for _, group := range dashboardOpenAPIPresentationGroups {
		group := group
		t.Run(string(group), func(t *testing.T) {
			dashboardOpenAPIRunPresentationScenario(t, group)
		})
	}
}

func dashboardOpenAPIRunPresentationScenario(t *testing.T, group dashboardOpenAPIPresentationGroup) {
	t.Helper()

	ctx := context.Background()
	var client *dashboardservice.DashboardServiceAPIService
	fixture := dashboardOpenAPINestedPresentationTestName + "-" + string(group)
	dashboardName := dashboardOpenAPIFixtureName(fixture)
	folderName := dashboardOpenAPIFixtureName(fixture + "-folder")
	identity := newDashboardOpenAPIIDTracker(dashboardResourceName, fixture)
	nestedIdentity := newDashboardOpenAPINestedIDTracker(fixture)
	checkPresentation := func(dashboardTimeFrame, queryTimeFrame, refresh string) resource.TestCheckFunc {
		return func(state *terraform.State) error {
			dashboard, err := dashboardOpenAPIFetchDashboard(ctx, client, state, dashboardResourceName, fixture)
			if err != nil {
				return err
			}
			if err := dashboardOpenAPIAssertPresentation(dashboard, fixture, dashboardTimeFrame, queryTimeFrame, refresh, group); err != nil {
				return err
			}
			if err := nestedIdentity.CaptureOrAssert(dashboard); err != nil {
				return err
			}
			folderState, ok := state.RootModule().Resources[folderResourceName]
			if !ok || folderState.Primary == nil || folderState.Primary.ID == "" {
				return fmt.Errorf("dashboard fixture %q: managed folder state is absent", fixture)
			}
			gotFolderID := ""
			if dashboard.FolderId != nil {
				gotFolderID = dashboard.FolderId.GetValue()
			}
			if gotFolderID != folderState.Primary.ID {
				return fmt.Errorf("dashboard fixture %q: REST folder ID = %q, want managed folder %q", fixture, gotFolderID, folderState.Primary.ID)
			}
			return nil
		}
	}
	checkImportedPresentation := func(dashboard *dashboardservice.Dashboard) error {
		if err := nestedIdentity.CaptureOrAssert(dashboard); err != nil {
			return err
		}
		return dashboardOpenAPIAssertPresentation(dashboard, fixture, "absoluteTimeFrame", "relativeTimeFrame", "off", group)
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			client = dashboardOpenAPIAcceptanceClient(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDashboardDestroy(t),
		Steps: dashboardOpenAPIStructuredLifecycleSteps(
			dashboardOpenAPILifecyclePhase{
				Config: dashboardOpenAPIPresentationConfigForGroup(dashboardName, folderName, "relative", "absolute", "two_minutes", "id", group),
				Check: resource.ComposeAggregateTestCheckFunc(
					identity.Capture(),
					dashboardOpenAPIPresentationStateChecks("relative", "absolute", "two_minutes", "id", folderName, group),
					checkPresentation("relativeTimeFrame", "absoluteTimeFrame", "twoMinutes"),
				),
			},
			[]dashboardOpenAPILifecyclePhase{{
				Config: dashboardOpenAPIPresentationConfigForGroup(dashboardName, folderName, "absolute", "relative", "five_minutes", "path", group),
				Check: resource.ComposeAggregateTestCheckFunc(
					identity.AssertUnchanged(),
					dashboardOpenAPIPresentationStateChecks("absolute", "relative", "five_minutes", "path", folderName, group),
					checkPresentation("absoluteTimeFrame", "relativeTimeFrame", "fiveMinutes"),
				),
			},
				{
					Config: dashboardOpenAPIPresentationConfigForGroup(dashboardName, folderName, "absolute", "relative", "off", "path", group),
					Check: resource.ComposeAggregateTestCheckFunc(
						identity.AssertUnchanged(),
						dashboardOpenAPIPresentationStateChecks("absolute", "relative", "off", "path", folderName, group),
						checkPresentation("absoluteTimeFrame", "relativeTimeFrame", "off"),
					),
				}},
			resource.TestStep{
				ResourceName:            dashboardResourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"folder"},
				ImportStateCheck:        dashboardOpenAPIImportDashboardCheck(ctx, &client, fixture, checkImportedPresentation),
			},
		),
	})
}

func dashboardOpenAPIPresentationStateChecks(dashboardTimeFrame, queryTimeFrame, refresh, folderSelector, folderName string, group dashboardOpenAPIPresentationGroup) resource.TestCheckFunc {
	checks := []resource.TestCheckFunc{
		resource.TestCheckResourceAttr(dashboardResourceName, "auto_refresh.type", refresh),
		resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.#", "2"),
	}
	switch group {
	case dashboardOpenAPIPresentationResolution:
		checks = append(checks,
			resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.line_chart.query_definitions.0.resolution.interval", "seconds:60"),
			resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.1.definition.line_chart.query_definitions.0.resolution.buckets_presented", "20"),
		)
	case dashboardOpenAPIPresentationBarCharts:
		checks = append(checks,
			resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.bar_chart.colors_by", "stack"),
			resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.1.definition.bar_chart.colors_by", "group_by"),
			resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.1.definition.bar_chart.xaxis.time.interval", "1m0s"),
			resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.1.definition.bar_chart.xaxis.time.buckets_presented", "30"),
		)
	case dashboardOpenAPIPresentationHorizontal:
		checks = append(checks,
			resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.horizontal_bar_chart.colors_by", "aggregation"),
			resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.horizontal_bar_chart.y_axis_view_by", "category"),
			resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.1.definition.horizontal_bar_chart.y_axis_view_by", "value"),
		)
	default:
		panic(fmt.Sprintf("unsupported presentation group %q", group))
	}
	if dashboardTimeFrame == "relative" {
		checks = append(checks, resource.TestCheckResourceAttr(dashboardResourceName, "time_frame.relative.duration", "seconds:900"))
	} else {
		checks = append(checks,
			resource.TestCheckResourceAttr(dashboardResourceName, "time_frame.absolute.start", "2026-01-01T00:00:00Z"),
			resource.TestCheckResourceAttr(dashboardResourceName, "time_frame.absolute.end", "2026-01-01T01:00:00Z"),
		)
	}
	if group == dashboardOpenAPIPresentationResolution {
		queryPath := "layout.sections.0.rows.0.widgets.0.definition.line_chart.query_definitions.0.query.metrics.time_frame."
		if queryTimeFrame == "relative" {
			checks = append(checks, resource.TestCheckResourceAttr(dashboardResourceName, queryPath+"relative.duration", "seconds:900"))
		} else {
			checks = append(checks,
				resource.TestCheckResourceAttr(dashboardResourceName, queryPath+"absolute.start", "2026-02-01T00:00:00Z"),
				resource.TestCheckResourceAttr(dashboardResourceName, queryPath+"absolute.end", "2026-02-01T00:15:00Z"),
			)
		}
	}
	if folderSelector == "id" {
		checks = append(checks, resource.TestCheckResourceAttrSet(dashboardResourceName, "folder.id"))
	} else {
		checks = append(checks, resource.TestCheckResourceAttr(dashboardResourceName, "folder.path", folderName))
	}
	return resource.ComposeAggregateTestCheckFunc(checks...)
}

func dashboardOpenAPIAssertPresentation(dashboard *dashboardservice.Dashboard, fixture, dashboardTimeFrame, queryTimeFrame, refresh string, group dashboardOpenAPIPresentationGroup) error {
	if dashboard == nil {
		return fmt.Errorf("dashboard fixture %q: fetched dashboard is nil", fixture)
	}
	if err := dashboardOpenAPIAssertOneOfBranch(dashboard, "Dashboard", dashboardTimeFrame, dashboard.GetId(), fixture); err != nil {
		return err
	}
	if err := dashboardOpenAPIAssertOneOfBranch(dashboard, "Dashboard", refresh, dashboard.GetId(), fixture); err != nil {
		return err
	}
	if dashboardTimeFrame == "relativeTimeFrame" && dashboard.GetRelativeTimeFrame() != "900s" {
		return fmt.Errorf("dashboard fixture %q: REST relativeTimeFrame = %q, want protobuf JSON duration 900s", fixture, dashboard.GetRelativeTimeFrame())
	}
	widgets, err := dashboardOpenAPIFirstRowWidgets(dashboard)
	if err != nil {
		return fmt.Errorf("dashboard fixture %q: %w", fixture, err)
	}
	if len(widgets) != 2 {
		return fmt.Errorf("dashboard fixture %q: REST widgets = %d, want 2", fixture, len(widgets))
	}

	switch group {
	case dashboardOpenAPIPresentationResolution:
		return dashboardOpenAPIAssertPresentationResolution(dashboard, widgets, fixture, queryTimeFrame)
	case dashboardOpenAPIPresentationBarCharts:
		return dashboardOpenAPIAssertPresentationBarCharts(dashboard, widgets, fixture)
	case dashboardOpenAPIPresentationHorizontal:
		return dashboardOpenAPIAssertPresentationHorizontalBars(dashboard, widgets, fixture)
	default:
		return fmt.Errorf("dashboard fixture %q: unsupported presentation group %q", fixture, group)
	}
}

func dashboardOpenAPIAssertPresentationResolution(dashboard *dashboardservice.Dashboard, widgets []dashboardservice.Widget, fixture, queryTimeFrame string) error {
	firstLine := widgets[0].GetDefinition().LineChart
	secondLine := widgets[1].GetDefinition().LineChart
	if firstLine == nil || len(firstLine.QueryDefinitions) != 1 || firstLine.QueryDefinitions[0].Resolution == nil {
		return fmt.Errorf("dashboard fixture %q: manual line resolution is absent", fixture)
	}
	if got := firstLine.QueryDefinitions[0].Resolution.GetInterval(); got != "60s" {
		return fmt.Errorf("dashboard fixture %q: line resolution interval = %q, want 60s", fixture, got)
	}
	if secondLine == nil || len(secondLine.QueryDefinitions) != 1 || secondLine.QueryDefinitions[0].Resolution == nil || secondLine.QueryDefinitions[0].Resolution.GetBucketsPresented() != 20 {
		return fmt.Errorf("dashboard fixture %q: line bucket resolution did not round-trip", fixture)
	}
	query := firstLine.QueryDefinitions[0].Query.Metrics
	if query == nil || query.TimeFrame == nil {
		return fmt.Errorf("dashboard fixture %q: query-level time frame is absent", fixture)
	}
	if err := dashboardOpenAPIAssertOneOfBranch(query.TimeFrame, "TimeFrameSelect", queryTimeFrame, dashboard.GetId(), fixture); err != nil {
		return err
	}
	if queryTimeFrame == "relativeTimeFrame" && query.TimeFrame.GetRelativeTimeFrame() != "900s" {
		return fmt.Errorf("dashboard fixture %q: REST query relativeTimeFrame = %q, want protobuf JSON duration 900s", fixture, query.TimeFrame.GetRelativeTimeFrame())
	}
	return nil
}

func dashboardOpenAPIAssertPresentationBarCharts(dashboard *dashboardservice.Dashboard, widgets []dashboardservice.Widget, fixture string) error {
	valueBar := widgets[0].GetDefinition().BarChart
	timeBar := widgets[1].GetDefinition().BarChart
	if valueBar == nil || valueBar.XAxis == nil || valueBar.ColorsBy == nil {
		return fmt.Errorf("dashboard fixture %q: value bar chart adapters are absent", fixture)
	}
	if err := dashboardOpenAPIAssertOneOfBranch(valueBar.XAxis, "XAxis", "value", dashboard.GetId(), fixture); err != nil {
		return err
	}
	if err := dashboardOpenAPIAssertOneOfBranch(valueBar.ColorsBy, "ColorsBy", "stack", dashboard.GetId(), fixture); err != nil {
		return err
	}
	if timeBar == nil || timeBar.XAxis == nil || timeBar.XAxis.Time == nil || timeBar.ColorsBy == nil {
		return fmt.Errorf("dashboard fixture %q: time bar chart adapters are absent", fixture)
	}
	if err := dashboardOpenAPIAssertOneOfBranch(timeBar.XAxis, "XAxis", "time", dashboard.GetId(), fixture); err != nil {
		return err
	}
	if timeBar.XAxis.Time.GetInterval() != "60s" || timeBar.XAxis.Time.GetBucketsPresented() != 30 {
		return fmt.Errorf("dashboard fixture %q: REST bar x-axis = %q/%d, want 60s/30", fixture, timeBar.XAxis.Time.GetInterval(), timeBar.XAxis.Time.GetBucketsPresented())
	}
	if err := dashboardOpenAPIAssertOneOfBranch(timeBar.ColorsBy, "ColorsBy", "groupBy", dashboard.GetId(), fixture); err != nil {
		return err
	}
	return nil
}

func dashboardOpenAPIAssertPresentationHorizontalBars(dashboard *dashboardservice.Dashboard, widgets []dashboardservice.Widget, fixture string) error {
	categoryBar := widgets[0].GetDefinition().HorizontalBarChart
	valueHorizontalBar := widgets[1].GetDefinition().HorizontalBarChart
	if categoryBar == nil || categoryBar.ColorsBy == nil || categoryBar.YAxisViewBy == nil {
		return fmt.Errorf("dashboard fixture %q: category horizontal bar adapters are absent", fixture)
	}
	if err := dashboardOpenAPIAssertOneOfBranch(categoryBar.ColorsBy, "ColorsBy", "aggregation", dashboard.GetId(), fixture); err != nil {
		return err
	}
	if err := dashboardOpenAPIAssertOneOfBranch(categoryBar.YAxisViewBy, "HorizontalBarChartYAxisViewBy", "category", dashboard.GetId(), fixture); err != nil {
		return err
	}
	if valueHorizontalBar == nil || valueHorizontalBar.YAxisViewBy == nil {
		return fmt.Errorf("dashboard fixture %q: value horizontal bar adapter is absent", fixture)
	}
	return dashboardOpenAPIAssertOneOfBranch(valueHorizontalBar.YAxisViewBy, "HorizontalBarChartYAxisViewBy", "value", dashboard.GetId(), fixture)
}

func dashboardOpenAPIPresentationConfig(name, folderName, dashboardTimeFrame, queryTimeFrame, refresh, folderSelector string) string {
	return dashboardOpenAPIPresentationConfigForGroup(name, folderName, dashboardTimeFrame, queryTimeFrame, refresh, folderSelector, dashboardOpenAPIPresentationAll)
}

func dashboardOpenAPIPresentationConfigForGroup(name, folderName, dashboardTimeFrame, queryTimeFrame, refresh, folderSelector string, group dashboardOpenAPIPresentationGroup) string {
	dashboardTF := `relative = { duration = "seconds:900" }`
	if dashboardTimeFrame == "absolute" {
		dashboardTF = `absolute = { start = "2026-01-01T00:00:00Z", end = "2026-01-01T01:00:00Z" }`
	}
	queryTF := `relative = { duration = "seconds:900" }`
	if queryTimeFrame == "absolute" {
		queryTF = `absolute = { start = "2026-02-01T00:00:00Z", end = "2026-02-01T00:15:00Z" }`
	}
	folder := `id = coralogix_dashboards_folder.test_folder.id`
	if folderSelector == "path" {
		folder = `path = coralogix_dashboards_folder.test_folder.name`
	}
	widgets := dashboardOpenAPIPresentationWidgets(group, queryTF)

	return fmt.Sprintf(`
resource "coralogix_dashboards_folder" "test_folder" {
  name = %q
}

resource "coralogix_dashboard" "test" {
  name         = %q
  description  = "Nested structured presentation branches"
  time_frame   = { %s }
  auto_refresh = { type = %q }
  folder       = { %s }
  layout = {
    sections = [{
      rows = [{
        height = 24
        widgets = [
%s
        ]
      }]
    }]
  }
}
`, folderName, name, dashboardTF, refresh, folder, widgets)
}

func dashboardOpenAPIPresentationWidgets(group dashboardOpenAPIPresentationGroup, queryTimeFrame string) string {
	resolution := fmt.Sprintf(`          {
            title = "line-manual-resolution"
            definition = { line_chart = {
              query_definitions = [{
                query = { metrics = { promql_query = "vector(1)", time_frame = { %s } } }
                resolution = { interval = "seconds:60" }
              }]
            } }
          },
          {
            title = "line-bucket-resolution"
            definition = { line_chart = {
              query_definitions = [{
                query      = { metrics = { promql_query = "vector(2)" } }
                resolution = { buckets_presented = 20 }
              }]
            } }
          },`, queryTimeFrame)
	barCharts := `          {
            title = "bar-value-stack"
            definition = { bar_chart = {
              query     = { logs = { aggregation = { type = "count" } } }
              colors_by = "stack"
              xaxis     = { value = {} }
            } }
          },
          {
            title = "bar-time-group-by"
            definition = { bar_chart = {
              query     = { logs = { aggregation = { type = "count" } } }
              colors_by = "group_by"
              xaxis     = { time = { interval = "1m0s", buckets_presented = 30 } }
            } }
          },`
	horizontalBars := `          {
            title = "horizontal-category-aggregation"
            definition = { horizontal_bar_chart = {
              query          = { logs = { aggregation = { type = "count" } } }
              colors_by      = "aggregation"
              y_axis_view_by = "category"
            } }
          },
          {
            title = "horizontal-value"
            definition = { horizontal_bar_chart = {
              query          = { logs = { aggregation = { type = "count" } } }
              y_axis_view_by = "value"
            } }
          },`

	switch group {
	case dashboardOpenAPIPresentationAll:
		return strings.Join([]string{resolution, barCharts, horizontalBars}, "\n")
	case dashboardOpenAPIPresentationResolution:
		return resolution
	case dashboardOpenAPIPresentationBarCharts:
		return barCharts
	case dashboardOpenAPIPresentationHorizontal:
		return horizontalBars
	default:
		panic(fmt.Sprintf("unsupported presentation group %q", group))
	}
}

func TestAccCoralogixResourceDashboardOpenAPILogsAggregationBranches(t *testing.T) {
	aggregations := []struct {
		typeName string
		branch   string
	}{
		{typeName: "count", branch: "count"},
		{typeName: "count_distinct", branch: "countDistinct"},
		{typeName: "sum", branch: "sum"},
		{typeName: "avg", branch: "average"},
		{typeName: "min", branch: "min"},
		{typeName: "max", branch: "max"},
		{typeName: "percentile", branch: "percentile"},
	}
	stateChecks := func(updated bool) []resource.TestCheckFunc {
		checks := []resource.TestCheckFunc{
			resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.line_chart.query_definitions.#", "7"),
			resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.line_chart.query_definitions.1.query.logs.aggregations.0.field", dashboardOpenAPILogsAggregationField(updated)),
			resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.line_chart.query_definitions.6.query.logs.aggregations.0.percent", fmt.Sprintf("%g", dashboardOpenAPILogsAggregationPercent(updated))),
		}
		for index, aggregation := range aggregations {
			checks = append(checks,
				resource.TestCheckResourceAttr(dashboardResourceName, fmt.Sprintf("layout.sections.0.rows.0.widgets.0.definition.line_chart.query_definitions.%d.query.logs.lucene_query", index), dashboardOpenAPILogsAggregationLuceneQuery(updated)),
				resource.TestCheckResourceAttr(dashboardResourceName, fmt.Sprintf("layout.sections.0.rows.0.widgets.0.definition.line_chart.query_definitions.%d.query.logs.aggregations.0.type", index), aggregation.typeName),
			)
		}
		return checks
	}
	dashboardOpenAPIRunNestedScenario(t, dashboardOpenAPILogsAggregationTestName, dashboardOpenAPILogsAggregationConfig, dashboardOpenAPILogsAggregationUpdateConfig, stateChecks, func(dashboard *dashboardservice.Dashboard, updated bool) error {
		widgets, err := dashboardOpenAPIFirstRowWidgets(dashboard)
		if err != nil {
			return err
		}
		lineChart := widgets[0].GetDefinition().LineChart
		if lineChart == nil || len(lineChart.QueryDefinitions) != len(aggregations) {
			return fmt.Errorf("REST logs query definitions = %d, want %d", dashboardOpenAPILineChartQueryDefinitionCount(lineChart), len(aggregations))
		}
		for index, aggregation := range aggregations {
			query := lineChart.QueryDefinitions[index].Query
			logs := query.Logs
			if logs == nil || len(logs.Aggregations) != 1 {
				return fmt.Errorf("REST logs query definition %d aggregations = %d, want 1", index, dashboardOpenAPILogsAggregationCount(logs))
			}
			luceneQuery := logs.GetLuceneQuery()
			if luceneQuery.GetValue() != dashboardOpenAPILogsAggregationLuceneQuery(updated) {
				return fmt.Errorf("REST logs query definition %d Lucene query did not round-trip", index)
			}
			if err := dashboardOpenAPIAssertOneOfBranch(&logs.Aggregations[0], "LogsAggregation", aggregation.branch, dashboard.GetId(), dashboardOpenAPILogsAggregationTestName); err != nil {
				return err
			}
		}
		countDistinct := lineChart.QueryDefinitions[1].Query.Logs.Aggregations[0]
		if countDistinct.CountDistinct == nil || countDistinct.CountDistinct.GetField() != dashboardOpenAPILogsAggregationField(updated) {
			return fmt.Errorf("REST countDistinct field did not round-trip")
		}
		percentile := lineChart.QueryDefinitions[6].Query.Logs.Aggregations[0]
		if percentile.Percentile == nil || percentile.Percentile.GetPercent() != dashboardOpenAPILogsAggregationPercent(updated) {
			return fmt.Errorf("REST percentile percent did not round-trip")
		}
		return nil
	})
}

func dashboardOpenAPILineChartQueryDefinitionCount(lineChart *dashboardservice.LineChart) int {
	if lineChart == nil {
		return 0
	}
	return len(lineChart.QueryDefinitions)
}

func dashboardOpenAPILogsAggregationCount(logs *dashboardservice.LineChartLogsQuery) int {
	if logs == nil {
		return 0
	}
	return len(logs.Aggregations)
}

func dashboardOpenAPILogsAggregationConfig(name string) string {
	return dashboardOpenAPILogsAggregationConfigVariant(name, false)
}

func dashboardOpenAPILogsAggregationUpdateConfig(name string) string {
	return dashboardOpenAPILogsAggregationConfigVariant(name, true)
}

func dashboardOpenAPILogsAggregationConfigVariant(name string, updated bool) string {
	return dashboardOpenAPIWrapWidgets(name, fmt.Sprintf(`{
  title = "all-log-aggregations"
  definition = { line_chart = {
    query_definitions = [
      { query = { logs = { lucene_query = %[1]q, aggregations = [{ type = "count" }] } } },
      { query = { logs = { lucene_query = %[1]q, aggregations = [{ type = "count_distinct", field = %[2]q }] } } },
      { query = { logs = { lucene_query = %[1]q, aggregations = [{ type = "sum", field = "latency" }] } } },
      { query = { logs = { lucene_query = %[1]q, aggregations = [{ type = "avg", field = "latency" }] } } },
      { query = { logs = { lucene_query = %[1]q, aggregations = [{ type = "min", field = "latency" }] } } },
      { query = { logs = { lucene_query = %[1]q, aggregations = [{ type = "max", field = "latency" }] } } },
      { query = { logs = { lucene_query = %[1]q, aggregations = [{ type = "percentile", field = "latency", percent = %[3]g }] } } },
    ]
  } }
}`, dashboardOpenAPILogsAggregationLuceneQuery(updated), dashboardOpenAPILogsAggregationField(updated), dashboardOpenAPILogsAggregationPercent(updated)))
}

func dashboardOpenAPILogsAggregationLuceneQuery(updated bool) string {
	if updated {
		return "coralogix.metadata.severity:INFO"
	}
	return "*"
}

func dashboardOpenAPILogsAggregationField(updated bool) string {
	if updated {
		return "coralogix.metadata.subsystemName"
	}
	return "coralogix.metadata.applicationName"
}

func dashboardOpenAPILogsAggregationPercent(updated bool) float64 {
	if updated {
		return 90
	}
	return 95
}

func TestAccCoralogixResourceDashboardOpenAPISpansAndFilterBranches(t *testing.T) {
	stateChecks := func(updated bool) []resource.TestCheckFunc {
		return []resource.TestCheckFunc{
			resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.line_chart.query_definitions.0.query.spans.aggregations.#", "2"),
			resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.line_chart.query_definitions.0.query.spans.lucene_query", dashboardOpenAPIUpdatedValue(updated, "*", "serviceName:api")),
			resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.line_chart.query_definitions.0.query.spans.aggregations.0.type", "metric"),
			resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.line_chart.query_definitions.0.query.spans.aggregations.0.aggregation_type", dashboardOpenAPIUpdatedValue(updated, "avg", "max")),
			resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.line_chart.query_definitions.0.query.spans.aggregations.0.field", "duration"),
			resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.line_chart.query_definitions.0.query.spans.aggregations.1.type", "dimension"),
			resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.line_chart.query_definitions.0.query.spans.aggregations.1.field", "trace_id"),
			resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.line_chart.query_definitions.0.query.spans.group_by.#", "3"),
			resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.line_chart.query_definitions.0.query.spans.group_by.0.type", "metadata"),
			resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.line_chart.query_definitions.0.query.spans.group_by.1.type", "tag"),
			resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.line_chart.query_definitions.0.query.spans.group_by.2.type", "process_tag"),
			resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.line_chart.query_definitions.0.query.spans.group_by.0.value", dashboardOpenAPIUpdatedValue(updated, "service_name", "subsystem_name")),
			resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.line_chart.query_definitions.0.query.spans.group_by.1.value", dashboardOpenAPIUpdatedValue(updated, "http.method", "http.route")),
			resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.line_chart.query_definitions.0.query.spans.group_by.2.value", dashboardOpenAPIUpdatedValue(updated, "service.version", "deployment.environment")),
			resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.1.definition.data_table.query.logs.filters.#", "2"),
			resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.1.definition.data_table.query.logs.filters.0.field", dashboardOpenAPIUpdatedValue(updated, "coralogix.metadata.applicationName", "coralogix.metadata.subsystemName")),
			resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.1.definition.data_table.query.logs.filters.0.operator.type", "equals"),
			resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.1.definition.data_table.query.logs.filters.0.operator.selected_values.#", "0"),
			resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.1.definition.data_table.query.logs.filters.1.observation_field.keypath.0", dashboardOpenAPIUpdatedValue(updated, "applicationName", "subsystemName")),
			resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.1.definition.data_table.query.logs.filters.1.operator.type", "not_equals"),
			resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.1.definition.data_table.query.logs.filters.1.operator.selected_values.0", dashboardOpenAPIUpdatedValue(updated, "production", "staging")),
		}
	}
	dashboardOpenAPIRunNestedScenario(t, dashboardOpenAPISpansAndFiltersTestName, dashboardOpenAPISpansAndFiltersConfig, dashboardOpenAPISpansAndFiltersUpdateConfig, stateChecks, func(dashboard *dashboardservice.Dashboard, updated bool) error {
		widgets, err := dashboardOpenAPIFirstRowWidgets(dashboard)
		if err != nil {
			return err
		}
		spans := widgets[0].GetDefinition().LineChart.QueryDefinitions[0].Query.Spans
		if spans == nil || len(spans.Aggregations) != 2 || len(spans.GroupBy) != 3 {
			return fmt.Errorf("REST spans aggregation/field lists did not round-trip")
		}
		spansLuceneQuery := spans.GetLuceneQuery()
		if spansLuceneQuery.GetValue() != dashboardOpenAPIUpdatedValue(updated, "*", "serviceName:api") {
			return fmt.Errorf("REST spans Lucene query did not round-trip after updated=%t", updated)
		}
		if err := dashboardOpenAPIAssertOneOfBranch(&spans.Aggregations[0], "SpansAggregation", "metricAggregation", dashboard.GetId(), dashboardOpenAPISpansAndFiltersTestName); err != nil {
			return err
		}
		if err := dashboardOpenAPIAssertOneOfBranch(&spans.Aggregations[1], "SpansAggregation", "dimensionAggregation", dashboard.GetId(), dashboardOpenAPISpansAndFiltersTestName); err != nil {
			return err
		}
		wantMetricAggregation := dashboardservice.METRICAGGREGATIONTYPE_METRIC_AGGREGATION_TYPE_AVERAGE
		if updated {
			wantMetricAggregation = dashboardservice.METRICAGGREGATIONTYPE_METRIC_AGGREGATION_TYPE_MAX
		}
		if spans.Aggregations[0].MetricAggregation == nil || spans.Aggregations[0].MetricAggregation.GetAggregationType() != wantMetricAggregation {
			return fmt.Errorf("REST metric aggregation value did not round-trip after updated=%t", updated)
		}
		for index, branch := range []string{"metadataField", "tagField", "processTagField"} {
			if err := dashboardOpenAPIAssertOneOfBranch(&spans.GroupBy[index], "SpanField", branch, dashboard.GetId(), dashboardOpenAPISpansAndFiltersTestName); err != nil {
				return err
			}
		}
		if spans.GroupBy[1].GetTagField() != dashboardOpenAPIUpdatedValue(updated, "http.method", "http.route") ||
			spans.GroupBy[2].GetProcessTagField() != dashboardOpenAPIUpdatedValue(updated, "service.version", "deployment.environment") {
			return fmt.Errorf("REST span field values did not round-trip after updated=%t", updated)
		}
		filters := widgets[1].GetDefinition().DataTable.Query.Logs.Filters
		if len(filters) != 2 || filters[0].Field == nil || filters[0].ObservationField != nil || filters[1].Field != nil || filters[1].ObservationField == nil {
			return fmt.Errorf("REST legacy-field/observation-field targets did not round-trip")
		}
		observationField := filters[1].GetObservationField()
		if filters[0].GetField() != dashboardOpenAPIUpdatedValue(updated, "coralogix.metadata.applicationName", "coralogix.metadata.subsystemName") ||
			len(observationField.GetKeypath()) != 1 || observationField.GetKeypath()[0] != dashboardOpenAPIUpdatedValue(updated, "applicationName", "subsystemName") {
			return fmt.Errorf("REST filter target values did not round-trip after updated=%t", updated)
		}
		if err := dashboardOpenAPIAssertOneOfBranch(filters[0].Operator, "FilterOperator", "equals", dashboard.GetId(), dashboardOpenAPISpansAndFiltersTestName); err != nil {
			return err
		}
		if err := dashboardOpenAPIAssertOneOfBranch(filters[0].Operator.Equals.Selection, "EqualsSelection", "all", dashboard.GetId(), dashboardOpenAPISpansAndFiltersTestName); err != nil {
			return err
		}
		if err := dashboardOpenAPIAssertOneOfBranch(filters[1].Operator, "FilterOperator", "notEquals", dashboard.GetId(), dashboardOpenAPISpansAndFiltersTestName); err != nil {
			return err
		}
		if filters[1].Operator.NotEquals == nil || filters[1].Operator.NotEquals.Selection == nil || filters[1].Operator.NotEquals.Selection.List == nil {
			return fmt.Errorf("REST not-equals filter list selection is absent")
		}
		selectedValues := filters[1].Operator.NotEquals.Selection.List.GetValues()
		if len(selectedValues) != 1 || selectedValues[0] != dashboardOpenAPIUpdatedValue(updated, "production", "staging") {
			return fmt.Errorf("REST not-equals filter selected values did not round-trip")
		}
		return nil
	})
}

func dashboardOpenAPISpansAndFiltersConfig(name string) string {
	return dashboardOpenAPISpansAndFiltersConfigVariant(name, false)
}

func dashboardOpenAPISpansAndFiltersUpdateConfig(name string) string {
	return dashboardOpenAPISpansAndFiltersConfigVariant(name, true)
}

func dashboardOpenAPISpansAndFiltersConfigVariant(name string, updated bool) string {
	widgets := `{
  title = "span-unions"
  definition = { line_chart = {
    query_definitions = [{ query = { spans = {
      lucene_query = "*"
      aggregations = [
        { type = "metric", aggregation_type = "avg", field = "duration" },
        { type = "dimension", aggregation_type = "unique_count", field = "trace_id" },
      ]
      group_by = [
        { type = "metadata", value = "service_name" },
        { type = "tag", value = "http.method" },
        { type = "process_tag", value = "service.version" },
      ]
    } } }]
  } }
},
{
  title = "filter-targets-and-operators"
  definition = { data_table = {
    results_per_page = 10
    row_style        = "one_line"
    columns          = [{ field = "coralogix.timestamp" }]
    query = { logs = { filters = [
      {
        field    = "coralogix.metadata.applicationName"
        operator = { type = "equals", selected_values = [] }
      },
      {
        observation_field = { keypath = ["applicationName"], scope = "metadata" }
        operator           = { type = "not_equals", selected_values = ["production"] }
      },
    ] } }
  } }
}`
	if updated {
		widgets = strings.NewReplacer(
			`lucene_query = "*"`, `lucene_query = "serviceName:api"`,
			`aggregation_type = "avg", field = "duration"`, `aggregation_type = "max", field = "duration"`,
			`type = "metadata", value = "service_name"`, `type = "metadata", value = "subsystem_name"`,
			`type = "tag", value = "http.method"`, `type = "tag", value = "http.route"`,
			`type = "process_tag", value = "service.version"`, `type = "process_tag", value = "deployment.environment"`,
			`field    = "coralogix.metadata.applicationName"`, `field    = "coralogix.metadata.subsystemName"`,
			`keypath = ["applicationName"]`, `keypath = ["subsystemName"]`,
			`selected_values = ["production"]`, `selected_values = ["staging"]`,
		).Replace(widgets)
	}
	return dashboardOpenAPIWrapWidgets(name, widgets)
}

func dashboardOpenAPIUpdatedValue(updated bool, initial, changed string) string {
	if updated {
		return changed
	}
	return initial
}

func TestAccCoralogixResourceDashboardOpenAPIVariableBranches(t *testing.T) {
	stateChecks := func(updated bool) []resource.TestCheckFunc {
		return []resource.TestCheckFunc{
			resource.TestCheckResourceAttr(dashboardResourceName, "variables.#", "11"),
			resource.TestCheckResourceAttr(dashboardResourceName, "variables.0.definition.multi_select.selected_values.#", "0"),
			resource.TestCheckResourceAttr(dashboardResourceName, "variables.0.definition.multi_select.source.logs_path", dashboardOpenAPIUpdatedValue(updated, "coralogix.metadata.applicationName", "coralogix.metadata.subsystemName")),
			resource.TestCheckResourceAttr(dashboardResourceName, "variables.1.definition.multi_select.source.metric_label.metric_name", dashboardOpenAPIUpdatedValue(updated, "http_requests_total", "http_server_requests_total")),
			resource.TestCheckResourceAttr(dashboardResourceName, "variables.1.definition.multi_select.source.metric_label.label", dashboardOpenAPIUpdatedValue(updated, "service", "job")),
			resource.TestCheckResourceAttr(dashboardResourceName, "variables.2.definition.multi_select.source.constant_list.0", dashboardOpenAPIUpdatedValue(updated, "http_requests_total", "http_server_requests_total")),
			resource.TestCheckResourceAttr(dashboardResourceName, "variables.3.definition.multi_select.source.span_field.type", "process_tag"),
			resource.TestCheckResourceAttr(dashboardResourceName, "variables.3.definition.multi_select.source.span_field.value", dashboardOpenAPIUpdatedValue(updated, "service.version", "deployment.environment")),
			resource.TestCheckResourceAttr(dashboardResourceName, "variables.4.definition.multi_select.source.query.query.logs.field_name.log_regex", dashboardOpenAPIUpdatedValue(updated, ".*", ".+")),
			resource.TestCheckResourceAttr(dashboardResourceName, "variables.5.definition.multi_select.source.query.query.logs.field_value.observation_field.keypath.0", dashboardOpenAPIUpdatedValue(updated, "applicationName", "subsystemName")),
			resource.TestCheckResourceAttr(dashboardResourceName, "variables.6.definition.multi_select.source.query.query.metrics.metric_name.metric_regex", dashboardOpenAPIUpdatedValue(updated, "http_.*", "api_.*")),
			resource.TestCheckResourceAttr(dashboardResourceName, "variables.7.definition.multi_select.source.query.query.metrics.label_name.metric_regex", dashboardOpenAPIUpdatedValue(updated, "http_.*", "api_.*")),
			resource.TestCheckResourceAttr(dashboardResourceName, "variables.8.definition.multi_select.source.query.query.metrics.label_value.metric_name.variable_name", "source_metric"),
			resource.TestCheckResourceAttr(dashboardResourceName, "variables.8.definition.multi_select.source.query.query.metrics.label_value.label_name.string_value", dashboardOpenAPIUpdatedValue(updated, "service", "job")),
			resource.TestCheckResourceAttr(dashboardResourceName, "variables.8.definition.multi_select.source.query.query.metrics.label_value.label_filters.0.metric.string_value", dashboardOpenAPIUpdatedValue(updated, "http_requests_total", "http_server_requests_total")),
			resource.TestCheckResourceAttr(dashboardResourceName, "variables.8.definition.multi_select.source.query.query.metrics.label_value.label_filters.0.label.string_value", dashboardOpenAPIUpdatedValue(updated, "region", "zone")),
			resource.TestCheckResourceAttr(dashboardResourceName, "variables.8.definition.multi_select.source.query.query.metrics.label_value.label_filters.0.operator.type", "not_equals"),
			resource.TestCheckResourceAttr(dashboardResourceName, "variables.8.definition.multi_select.source.query.query.metrics.label_value.label_filters.0.operator.selected_values.0.variable_name", "source_metric"),
			resource.TestCheckResourceAttr(dashboardResourceName, "variables.8.definition.multi_select.source.query.query.metrics.label_value.label_filters.1.operator.type", "equals"),
			resource.TestCheckResourceAttr(dashboardResourceName, "variables.8.definition.multi_select.source.query.query.metrics.label_value.label_filters.1.label.string_value", dashboardOpenAPIUpdatedValue(updated, "environment", "deployment.environment")),
			resource.TestCheckResourceAttr(dashboardResourceName, "variables.8.definition.multi_select.source.query.query.metrics.label_value.label_filters.1.operator.selected_values.0.string_value", dashboardOpenAPIUpdatedValue(updated, "production", "staging")),
			resource.TestCheckResourceAttr(dashboardResourceName, "variables.9.definition.multi_select.source.query.query.spans.field_name.span_regex", dashboardOpenAPIUpdatedValue(updated, ".*", ".+")),
			resource.TestCheckResourceAttr(dashboardResourceName, "variables.10.definition.multi_select.source.query.query.spans.field_value.type", "tag"),
			resource.TestCheckResourceAttr(dashboardResourceName, "variables.10.definition.multi_select.source.query.query.spans.field_value.value", dashboardOpenAPIUpdatedValue(updated, "http.method", "http.route")),
		}
	}
	dashboardOpenAPIRunNestedScenario(t, dashboardOpenAPIVariablesTestName, dashboardOpenAPIVariablesConfig, dashboardOpenAPIVariablesUpdateConfig, stateChecks, func(dashboard *dashboardservice.Dashboard, updated bool) error {
		variables := dashboard.GetVariables()
		if len(variables) != 11 {
			return fmt.Errorf("REST variables = %d, want 11", len(variables))
		}
		for index := range variables {
			if variables[index].Definition == nil {
				return fmt.Errorf("REST variable %d definition is nil", index)
			}
			if err := dashboardOpenAPIAssertOneOfBranch(variables[index].Definition, "VariableDefinition", "multiSelect", dashboard.GetId(), dashboardOpenAPIVariablesTestName); err != nil {
				return err
			}
		}
		if err := dashboardOpenAPIAssertOneOfBranch(variables[0].Definition.MultiSelect.Selection, "MultiSelectSelection", "all", dashboard.GetId(), dashboardOpenAPIVariablesTestName); err != nil {
			return err
		}
		if err := dashboardOpenAPIAssertOneOfBranch(variables[1].Definition.MultiSelect.Selection, "MultiSelectSelection", "list", dashboard.GetId(), dashboardOpenAPIVariablesTestName); err != nil {
			return err
		}
		for index, branch := range []string{"logsPath", "metricLabel", "constantList", "spanField"} {
			source := variables[index].Definition.MultiSelect.Source
			if err := dashboardOpenAPIAssertOneOfBranch(source, "MultiSelectSource", branch, dashboard.GetId(), dashboardOpenAPIVariablesTestName); err != nil {
				return err
			}
		}
		for index := 4; index < len(variables); index++ {
			source := variables[index].Definition.MultiSelect.Source
			if err := dashboardOpenAPIAssertOneOfBranch(source, "MultiSelectSource", "query", dashboard.GetId(), dashboardOpenAPIVariablesTestName); err != nil {
				return err
			}
		}
		if variables[0].Definition.MultiSelect.Source.LogsPath == nil || variables[0].Definition.MultiSelect.Source.LogsPath.GetValue() != dashboardOpenAPIUpdatedValue(updated, "coralogix.metadata.applicationName", "coralogix.metadata.subsystemName") {
			return fmt.Errorf("REST logsPath source did not round-trip")
		}
		if variables[1].Definition.MultiSelect.Source.MetricLabel == nil || variables[1].Definition.MultiSelect.Source.MetricLabel.GetLabel() != dashboardOpenAPIUpdatedValue(updated, "service", "job") ||
			variables[1].Definition.MultiSelect.Source.MetricLabel.GetMetricName() != dashboardOpenAPIUpdatedValue(updated, "http_requests_total", "http_server_requests_total") {
			return fmt.Errorf("REST metricLabel source did not round-trip")
		}
		if variables[3].Definition.MultiSelect.Source.SpanField == nil || variables[3].Definition.MultiSelect.Source.SpanField.Value == nil {
			return fmt.Errorf("REST spanField source did not round-trip")
		}
		constantValues := variables[2].Definition.MultiSelect.Source.ConstantList.GetValues()
		if len(constantValues) != 1 || constantValues[0] != dashboardOpenAPIUpdatedValue(updated, "http_requests_total", "http_server_requests_total") {
			return fmt.Errorf("REST constantList source did not round-trip")
		}
		if err := dashboardOpenAPIAssertOneOfBranch(variables[3].Definition.MultiSelect.Source.SpanField.Value, "SpanField", "processTagField", dashboard.GetId(), dashboardOpenAPIVariablesTestName); err != nil {
			return err
		}
		if variables[3].Definition.MultiSelect.Source.SpanField.Value.GetProcessTagField() != dashboardOpenAPIUpdatedValue(updated, "service.version", "deployment.environment") {
			return fmt.Errorf("REST spanField source value did not round-trip")
		}

		queries := make([]*dashboardservice.MultiSelectQuery, 0, 7)
		for index := 4; index < len(variables); index++ {
			querySource := variables[index].Definition.MultiSelect.Source.Query
			if querySource == nil || querySource.Query == nil {
				return fmt.Errorf("REST variable %d query source is nil", index)
			}
			queries = append(queries, querySource.Query)
		}
		for index, branch := range []string{"logsQuery", "logsQuery", "metricsQuery", "metricsQuery", "metricsQuery", "spansQuery", "spansQuery"} {
			if err := dashboardOpenAPIAssertOneOfBranch(queries[index], "MultiSelectQuery", branch, dashboard.GetId(), dashboardOpenAPIVariablesTestName); err != nil {
				return err
			}
		}
		if err := dashboardOpenAPIAssertOneOfBranch(queries[0].LogsQuery.Type, "QueryLogsQueryType", "fieldName", dashboard.GetId(), dashboardOpenAPIVariablesTestName); err != nil {
			return err
		}
		if err := dashboardOpenAPIAssertOneOfBranch(queries[1].LogsQuery.Type, "QueryLogsQueryType", "fieldValue", dashboard.GetId(), dashboardOpenAPIVariablesTestName); err != nil {
			return err
		}
		logsFieldValue := queries[1].LogsQuery.Type.FieldValue.GetObservationField()
		if queries[0].LogsQuery.Type.FieldName.GetLogRegex() != dashboardOpenAPIUpdatedValue(updated, ".*", ".+") ||
			len(logsFieldValue.GetKeypath()) != 1 || logsFieldValue.GetKeypath()[0] != dashboardOpenAPIUpdatedValue(updated, "applicationName", "subsystemName") {
			return fmt.Errorf("REST variable logs query values did not round-trip")
		}
		for index, branch := range []string{"metricName", "labelName", "labelValue"} {
			if err := dashboardOpenAPIAssertOneOfBranch(queries[index+2].MetricsQuery.Type, "QueryMetricsQueryType", branch, dashboard.GetId(), dashboardOpenAPIVariablesTestName); err != nil {
				return err
			}
		}
		if queries[2].MetricsQuery.Type.MetricName.GetMetricRegex() != dashboardOpenAPIUpdatedValue(updated, "http_.*", "api_.*") ||
			queries[3].MetricsQuery.Type.LabelName.GetMetricRegex() != dashboardOpenAPIUpdatedValue(updated, "http_.*", "api_.*") {
			return fmt.Errorf("REST variable metrics regex values did not round-trip")
		}
		labelValue := queries[4].MetricsQuery.Type.LabelValue
		if labelValue == nil || labelValue.MetricName == nil || len(labelValue.LabelFilters) != 2 || labelValue.LabelFilters[0].Operator == nil || labelValue.LabelFilters[1].Operator == nil {
			return fmt.Errorf("REST metrics label-value sub-branches did not round-trip")
		}
		if err := dashboardOpenAPIAssertOneOfBranch(labelValue.MetricName, "QueryMetricsQueryStringOrVariable", "variableName", dashboard.GetId(), dashboardOpenAPIVariablesTestName); err != nil {
			return err
		}
		if err := dashboardOpenAPIAssertOneOfBranch(labelValue.LabelName, "QueryMetricsQueryStringOrVariable", "stringValue", dashboard.GetId(), dashboardOpenAPIVariablesTestName); err != nil {
			return err
		}
		if err := dashboardOpenAPIAssertOneOfBranch(labelValue.LabelFilters[0].Operator, "QueryMetricsQueryOperator", "notEquals", dashboard.GetId(), dashboardOpenAPIVariablesTestName); err != nil {
			return err
		}
		if err := dashboardOpenAPIAssertOneOfBranch(labelValue.LabelFilters[1].Operator, "QueryMetricsQueryOperator", "equals", dashboard.GetId(), dashboardOpenAPIVariablesTestName); err != nil {
			return err
		}
		firstMetric := labelValue.LabelFilters[0].GetMetric()
		firstLabel := labelValue.LabelFilters[0].GetLabel()
		secondLabel := labelValue.LabelFilters[1].GetLabel()
		if labelValue.LabelName.GetStringValue() != dashboardOpenAPIUpdatedValue(updated, "service", "job") ||
			firstMetric.GetStringValue() != dashboardOpenAPIUpdatedValue(updated, "http_requests_total", "http_server_requests_total") ||
			firstLabel.GetStringValue() != dashboardOpenAPIUpdatedValue(updated, "region", "zone") ||
			secondLabel.GetStringValue() != dashboardOpenAPIUpdatedValue(updated, "environment", "deployment.environment") {
			return fmt.Errorf("REST variable metrics label-value targets did not round-trip")
		}
		firstOperator := labelValue.LabelFilters[0].Operator.NotEquals
		secondOperator := labelValue.LabelFilters[1].Operator.Equals
		if firstOperator == nil || firstOperator.Selection == nil || firstOperator.Selection.List == nil ||
			secondOperator == nil || secondOperator.Selection == nil || secondOperator.Selection.List == nil {
			return fmt.Errorf("REST variable metrics label-value selections are absent")
		}
		firstSelection := firstOperator.Selection.List.GetValues()
		secondSelection := secondOperator.Selection.List.GetValues()
		if len(firstSelection) != 1 || firstSelection[0].GetVariableName() != "source_metric" || len(secondSelection) != 1 ||
			secondSelection[0].GetStringValue() != dashboardOpenAPIUpdatedValue(updated, "production", "staging") {
			return fmt.Errorf("REST variable metrics label-value selections did not round-trip")
		}
		if err := dashboardOpenAPIAssertOneOfBranch(queries[5].SpansQuery.Type, "QuerySpansQueryType", "fieldName", dashboard.GetId(), dashboardOpenAPIVariablesTestName); err != nil {
			return err
		}
		if err := dashboardOpenAPIAssertOneOfBranch(queries[6].SpansQuery.Type, "QuerySpansQueryType", "fieldValue", dashboard.GetId(), dashboardOpenAPIVariablesTestName); err != nil {
			return err
		}
		spansFieldValue := queries[6].SpansQuery.Type.FieldValue.GetValue()
		if queries[5].SpansQuery.Type.FieldName.GetSpanRegex() != dashboardOpenAPIUpdatedValue(updated, ".*", ".+") ||
			spansFieldValue.GetTagField() != dashboardOpenAPIUpdatedValue(updated, "http.method", "http.route") {
			return fmt.Errorf("REST variable spans query values did not round-trip")
		}
		return nil
	})
}

func dashboardOpenAPIVariablesConfig(name string) string {
	return dashboardOpenAPIVariablesConfigVariant(name, false)
}

func dashboardOpenAPIVariablesUpdateConfig(name string) string {
	return dashboardOpenAPIVariablesConfigVariant(name, true)
}

func dashboardOpenAPIVariablesConfigVariant(name string, updated bool) string {
	config := fmt.Sprintf(`
resource "coralogix_dashboard" "test" {
  name        = %q
  description = "Nested structured variable branches"
  time_frame  = { relative = { duration = "seconds:900" } }
  layout = {
    sections = [{ rows = [{ height = 10, widgets = [{
      title      = "placeholder"
      definition = { line_chart = { query_definitions = [{ query = { metrics = { promql_query = "vector(1)" } } }] } }
    }] }] }]
  }
  variables = [
    {
      name = "logs_path", display_name = "Logs path"
      definition = { multi_select = {
        selected_values = [], values_order_direction = "asc"
        source = { logs_path = "coralogix.metadata.applicationName" }
      } }
    },
    {
      name = "metric_label", display_name = "Metric label"
      definition = { multi_select = {
        selected_values = ["api"], values_order_direction = "asc"
        source = { metric_label = { metric_name = "http_requests_total", label = "service" } }
      } }
    },
    {
      name = "source_metric", display_name = "Source metric"
      definition = { multi_select = {
        selected_values = ["http_requests_total"], values_order_direction = "asc"
        source = { constant_list = ["http_requests_total"] }
      } }
    },
    {
      name = "span_source", display_name = "Span source"
      definition = { multi_select = {
        selected_values = ["v1"], values_order_direction = "asc"
        source = { span_field = { type = "process_tag", value = "service.version" } }
      } }
    },
    {
      name = "logs_field_name", display_name = "Logs field name"
      definition = { multi_select = {
        values_order_direction = "asc"
        source = { query = { query = { logs = { field_name = { log_regex = ".*" } } } } }
      } }
    },
    {
      name = "logs_field_value", display_name = "Logs field value"
      definition = { multi_select = {
        values_order_direction = "asc"
        source = { query = { query = { logs = { field_value = {
          observation_field = { keypath = ["applicationName"], scope = "metadata" }
        } } } } }
      } }
    },
    {
      name = "metric_name", display_name = "Metric name"
      definition = { multi_select = {
        values_order_direction = "asc"
        source = { query = { query = { metrics = { metric_name = { metric_regex = "http_.*" } } } } }
      } }
    },
    {
      name = "label_name", display_name = "Label name"
      definition = { multi_select = {
        values_order_direction = "asc"
        source = { query = { query = { metrics = { label_name = { metric_regex = "http_.*" } } } } }
      } }
    },
    {
      name = "label_value", display_name = "Label value"
      definition = { multi_select = {
        values_order_direction = "asc"
        source = { query = { query = { metrics = { label_value = {
          metric_name = { variable_name = "source_metric" }
          label_name  = { string_value = "service" }
          label_filters = [{
            metric = { string_value = "http_requests_total" }
            label  = { string_value = "region" }
            operator = {
              type            = "not_equals"
              selected_values = [{ variable_name = "source_metric" }]
            }
          }, {
            metric = { string_value = "http_requests_total" }
            label  = { string_value = "environment" }
            operator = {
              type            = "equals"
              selected_values = [{ string_value = "production" }]
            }
          }]
        } } } } }
      } }
    },
    {
      name = "span_field_name", display_name = "Span field name"
      definition = { multi_select = {
        values_order_direction = "asc"
        source = { query = { query = { spans = { field_name = { span_regex = ".*" } } } } }
      } }
    },
    {
      name = "span_field_value", display_name = "Span field value"
      definition = { multi_select = {
        values_order_direction = "asc"
        source = { query = { query = { spans = { field_value = { type = "tag", value = "http.method" } } } } }
      } }
    },
  ]
}
`, name)
	if updated {
		config = strings.NewReplacer(
			"coralogix.metadata.applicationName", "coralogix.metadata.subsystemName",
			"http_requests_total", "http_server_requests_total",
			`label = "service"`, `label = "job"`,
			`value = "service.version"`, `value = "deployment.environment"`,
			`log_regex = ".*"`, `log_regex = ".+"`,
			`keypath = ["applicationName"]`, `keypath = ["subsystemName"]`,
			`metric_regex = "http_.*"`, `metric_regex = "api_.*"`,
			`span_regex = ".*"`, `span_regex = ".+"`,
			`value = "http.method"`, `value = "http.route"`,
			`string_value = "service"`, `string_value = "job"`,
			`string_value = "region"`, `string_value = "zone"`,
			`string_value = "environment"`, `string_value = "deployment.environment"`,
			`string_value = "production"`, `string_value = "staging"`,
		).Replace(config)
	}
	return config
}

func TestAccCoralogixResourceDashboardOpenAPIAnnotationBranches(t *testing.T) {
	ctx := context.Background()
	var client *dashboardservice.DashboardServiceAPIService
	fixture := dashboardOpenAPIAnnotationsTestName
	name := dashboardOpenAPIFixtureName(fixture)
	dashboardIdentity := newDashboardOpenAPIIDTracker(dashboardResourceName, fixture)
	nestedIdentity := newDashboardOpenAPINestedIDTracker(fixture)
	annotationIDs := newDashboardOpenAPIAnnotationIDTracker(fixture)
	checks := func(updated bool, identityCheck resource.TestCheckFunc) resource.TestCheckFunc {
		checks := []resource.TestCheckFunc{
			resource.TestCheckResourceAttr(dashboardResourceName, "annotations.#", "9"),
			resource.TestCheckResourceAttr(dashboardResourceName, "annotations.0.source.metrics.promql_query", dashboardOpenAPIUpdatedValue(updated, "vector(1)", "vector(2)")),
			resource.TestCheckResourceAttr(dashboardResourceName, "annotations.1.source.manual.strategy.instant.value", dashboardOpenAPIUpdatedValue(updated, "42", "84")),
			resource.TestCheckResourceAttr(dashboardResourceName, "annotations.2.source.manual.strategy.range.start_value", dashboardOpenAPIUpdatedValue(updated, "10", "20")),
			resource.TestCheckResourceAttr(dashboardResourceName, "annotations.3.source.logs.lucene_query", dashboardOpenAPIUpdatedValue(updated, "*", "coralogix.metadata.severity:INFO")),
			resource.TestCheckResourceAttr(dashboardResourceName, "annotations.3.source.logs.strategy.instant.timestamp_field.keypath.0", dashboardOpenAPIUpdatedValue(updated, "timestamp", "event_timestamp")),
			resource.TestCheckResourceAttr(dashboardResourceName, "annotations.4.source.logs.strategy.range.start_timestamp_field.keypath.0", dashboardOpenAPIUpdatedValue(updated, "start_time", "range_start")),
			resource.TestCheckResourceAttr(dashboardResourceName, "annotations.5.source.logs.strategy.duration.duration_field.keypath.0", dashboardOpenAPIUpdatedValue(updated, "duration_ms", "elapsed_ms")),
			resource.TestCheckResourceAttr(dashboardResourceName, "annotations.6.source.spans.lucene_query", dashboardOpenAPIUpdatedValue(updated, "*", "serviceName:api")),
			resource.TestCheckResourceAttr(dashboardResourceName, "annotations.6.source.spans.strategy.instant.timestamp_field.keypath.0", dashboardOpenAPIUpdatedValue(updated, "startTime", "startTimeUnixNano")),
			resource.TestCheckResourceAttr(dashboardResourceName, "annotations.7.source.spans.strategy.range.end_timestamp_field.keypath.0", dashboardOpenAPIUpdatedValue(updated, "endTime", "endTimeUnixNano")),
			resource.TestCheckResourceAttr(dashboardResourceName, "annotations.8.source.spans.strategy.duration.duration_field.keypath.0", dashboardOpenAPIUpdatedValue(updated, "durationNano", "durationMillis")),
			identityCheck,
			annotationIDs.CaptureOrAssert(),
		}
		checks = append(checks, func(state *terraform.State) error {
			dashboard, err := dashboardOpenAPIFetchDashboard(ctx, client, state, dashboardResourceName, fixture)
			if err != nil {
				return err
			}
			if err := nestedIdentity.CaptureOrAssert(dashboard); err != nil {
				return err
			}
			return dashboardOpenAPIAssertAnnotations(dashboard, fixture, updated)
		})
		return resource.ComposeAggregateTestCheckFunc(checks...)
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			client = dashboardOpenAPIAcceptanceClient(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDashboardDestroy(t),
		Steps: dashboardOpenAPIStructuredLifecycleSteps(
			dashboardOpenAPILifecyclePhase{
				Config: dashboardOpenAPIAnnotationsConfig(name),
				Check:  checks(false, dashboardIdentity.Capture()),
			},
			[]dashboardOpenAPILifecyclePhase{{
				Config: dashboardOpenAPIAnnotationsUpdateConfig(name),
				Check:  checks(true, dashboardIdentity.AssertUnchanged()),
			}},
			resource.TestStep{
				ResourceName:      dashboardResourceName,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateCheck: dashboardOpenAPIComposeImportStateChecks(
					annotationIDs.AssertImported(),
					dashboardOpenAPIImportDashboardCheck(ctx, &client, fixture, func(dashboard *dashboardservice.Dashboard) error {
						if err := nestedIdentity.CaptureOrAssert(dashboard); err != nil {
							return err
						}
						return dashboardOpenAPIAssertAnnotations(dashboard, fixture, true)
					}),
				),
			},
		),
	})
}

type dashboardOpenAPIAnnotationIDTracker struct {
	fixture string
	ids     map[string]string
}

func newDashboardOpenAPIAnnotationIDTracker(fixture string) *dashboardOpenAPIAnnotationIDTracker {
	return &dashboardOpenAPIAnnotationIDTracker{fixture: fixture}
}

func (tracker *dashboardOpenAPIAnnotationIDTracker) CaptureOrAssert() resource.TestCheckFunc {
	return func(state *terraform.State) error {
		resourceState, ok := state.RootModule().Resources[dashboardResourceName]
		if !ok || resourceState.Primary == nil {
			return fmt.Errorf("dashboard fixture %q: dashboard state is absent", tracker.fixture)
		}
		ids, err := dashboardOpenAPIAnnotationIDs(resourceState.Primary.Attributes)
		if err != nil {
			return fmt.Errorf("dashboard fixture %q: %w", tracker.fixture, err)
		}
		if tracker.ids == nil {
			tracker.ids = ids
			return nil
		}
		return dashboardOpenAPICompareAnnotationIDs(tracker.ids, ids)
	}
}

func (tracker *dashboardOpenAPIAnnotationIDTracker) AssertImported() resource.ImportStateCheckFunc {
	return func(states []*terraform.InstanceState) error {
		for _, state := range states {
			if _, ok := state.Attributes["annotations.#"]; !ok {
				continue
			}
			ids, err := dashboardOpenAPIAnnotationIDs(state.Attributes)
			if err != nil {
				return fmt.Errorf("dashboard fixture %q import: %w", tracker.fixture, err)
			}
			return dashboardOpenAPICompareAnnotationIDs(tracker.ids, ids)
		}
		return fmt.Errorf("dashboard fixture %q import: annotations state is absent", tracker.fixture)
	}
}

func dashboardOpenAPIAnnotationIDs(attributes map[string]string) (map[string]string, error) {
	count := 0
	if _, err := fmt.Sscanf(attributes["annotations.#"], "%d", &count); err != nil {
		return nil, fmt.Errorf("parse annotations count %q: %w", attributes["annotations.#"], err)
	}
	ids := make(map[string]string, count)
	for index := 0; index < count; index++ {
		name := attributes[fmt.Sprintf("annotations.%d.name", index)]
		id := attributes[fmt.Sprintf("annotations.%d.id", index)]
		if name == "" || id == "" {
			return nil, fmt.Errorf("annotation %d has name %q and ID %q", index, name, id)
		}
		if _, exists := ids[name]; exists {
			return nil, fmt.Errorf("annotation name %q is duplicated", name)
		}
		ids[name] = id
	}
	return ids, nil
}

func dashboardOpenAPICompareAnnotationIDs(want, got map[string]string) error {
	var mismatches []string
	if len(got) != len(want) {
		mismatches = append(mismatches, fmt.Sprintf("annotation IDs = %d, want %d", len(got), len(want)))
	}
	for name, wantID := range want {
		if gotID := got[name]; gotID != wantID {
			mismatches = append(mismatches, fmt.Sprintf("annotation %q ID = %q, want %q", name, gotID, wantID))
		}
	}
	return dashboardOpenAPIJoinErrors(mismatches)
}

func dashboardOpenAPIAssertAnnotations(dashboard *dashboardservice.Dashboard, fixture string, updated bool) error {
	annotations := dashboard.GetAnnotations()
	if len(annotations) != 9 {
		return fmt.Errorf("dashboard fixture %q: REST annotations = %d, want 9", fixture, len(annotations))
	}
	sourceBranches := []string{"metrics", "manual", "manual", "logs", "logs", "logs", "spans", "spans", "spans"}
	for index, branch := range sourceBranches {
		if annotations[index].GetId() == "" {
			return fmt.Errorf("dashboard fixture %q: REST annotation %d has no generated ID", fixture, index)
		}
		if err := dashboardOpenAPIAssertOneOfBranch(annotations[index].Source, "AnnotationSource", branch, dashboard.GetId(), fixture); err != nil {
			return err
		}
	}
	if annotations[0].Source.Metrics == nil || annotations[0].Source.Metrics.Strategy == nil || annotations[0].Source.Metrics.Strategy.StartTimeMetric == nil {
		return fmt.Errorf("dashboard fixture %q: REST metrics annotation strategy did not round-trip", fixture)
	}
	if annotations[0].Source.Metrics.PromqlQuery == nil || annotations[0].Source.Metrics.PromqlQuery.GetValue() != dashboardOpenAPIUpdatedValue(updated, "vector(1)", "vector(2)") ||
		len(annotations[0].Source.Metrics.GetLabels()) != 1 || annotations[0].Source.Metrics.GetLabels()[0] != dashboardOpenAPIUpdatedValue(updated, "service", "job") {
		return fmt.Errorf("dashboard fixture %q: REST metrics annotation values did not round-trip", fixture)
	}
	for index, branch := range []string{"instant", "range"} {
		strategy := annotations[index+1].Source.Manual.Strategy
		if err := dashboardOpenAPIAssertOneOfBranch(strategy, "ManualSourceStrategy", branch, dashboard.GetId(), fixture); err != nil {
			return err
		}
	}
	manualInstant := annotations[1].Source.Manual.Strategy.Instant
	manualRange := annotations[2].Source.Manual.Strategy.Range
	wantInstant := 42.0
	wantRangeStart, wantRangeEnd := 10.0, 20.0
	if updated {
		wantInstant = 84
		wantRangeStart, wantRangeEnd = 20, 40
	}
	if manualInstant == nil || manualInstant.GetValue() != wantInstant || manualRange == nil ||
		manualRange.GetStartValue() != wantRangeStart || manualRange.GetEndValue() != wantRangeEnd {
		return fmt.Errorf("dashboard fixture %q: REST manual annotation values did not round-trip", fixture)
	}
	wantLogsLucene := dashboardOpenAPIUpdatedValue(updated, "*", "coralogix.metadata.severity:INFO")
	for index, branch := range []string{"instant", "range", "duration"} {
		logs := annotations[index+3].Source.Logs
		if logs == nil || logs.Strategy == nil {
			return fmt.Errorf("dashboard fixture %q: REST logs annotation %d strategy is nil", fixture, index)
		}
		if logs.LuceneQuery == nil || logs.LuceneQuery.GetValue() != wantLogsLucene {
			return fmt.Errorf("dashboard fixture %q: REST logs annotation %d lucene query did not round-trip", fixture, index)
		}
		if logs.DataModeType != nil && *logs.DataModeType != dashboardservice.V1COMMONDATAMODETYPE_DATA_MODE_TYPE_HIGH_UNSPECIFIED {
			return fmt.Errorf("dashboard fixture %q: backend normalized unset logs annotation dataModeType to unexpected value %q", fixture, *logs.DataModeType)
		}
		if err := dashboardOpenAPIAssertOneOfBranch(logs.Strategy, "LogsSourceStrategy", branch, dashboard.GetId(), fixture); err != nil {
			return err
		}
		if err := dashboardOpenAPIAssertLogsAnnotationStrategyValues(logs.Strategy, index, updated); err != nil {
			return fmt.Errorf("dashboard fixture %q: %w", fixture, err)
		}
	}
	for index, branch := range []string{"instant", "range", "duration"} {
		spans := annotations[index+6].Source.Spans
		wantSpansLucene := dashboardOpenAPIUpdatedValue(updated, "*", "serviceName:api")
		if spans == nil || spans.Strategy == nil {
			return fmt.Errorf("dashboard fixture %q: REST spans annotation %d strategy is nil", fixture, index)
		}
		if spans.LuceneQuery == nil || spans.LuceneQuery.GetValue() != wantSpansLucene {
			return fmt.Errorf("dashboard fixture %q: REST spans annotation %d lucene query did not round-trip", fixture, index)
		}
		if spans.DataModeType != nil && *spans.DataModeType != dashboardservice.V1COMMONDATAMODETYPE_DATA_MODE_TYPE_HIGH_UNSPECIFIED {
			return fmt.Errorf("dashboard fixture %q: backend normalized unset spans annotation dataModeType to unexpected value %q", fixture, *spans.DataModeType)
		}
		if err := dashboardOpenAPIAssertOneOfBranch(spans.Strategy, "SpansSourceStrategy", branch, dashboard.GetId(), fixture); err != nil {
			return err
		}
		if err := dashboardOpenAPIAssertSpansAnnotationStrategyValues(spans.Strategy, index, updated); err != nil {
			return fmt.Errorf("dashboard fixture %q: %w", fixture, err)
		}
	}
	return nil
}

func dashboardOpenAPIAssertLogsAnnotationStrategyValues(strategy *dashboardservice.LogsSourceStrategy, index int, updated bool) error {
	want := [][]string{
		{dashboardOpenAPIUpdatedValue(updated, "timestamp", "event_timestamp")},
		{dashboardOpenAPIUpdatedValue(updated, "start_time", "range_start"), dashboardOpenAPIUpdatedValue(updated, "end_time", "range_end")},
		{dashboardOpenAPIUpdatedValue(updated, "start_time", "range_start"), dashboardOpenAPIUpdatedValue(updated, "duration_ms", "elapsed_ms")},
	}[index]
	var got [][]string
	switch index {
	case 0:
		got = [][]string{dashboardOpenAPIObservationFieldKeypath(strategy.Instant.GetTimestampField())}
	case 1:
		got = [][]string{dashboardOpenAPIObservationFieldKeypath(strategy.Range.GetStartTimestampField()), dashboardOpenAPIObservationFieldKeypath(strategy.Range.GetEndTimestampField())}
	case 2:
		got = [][]string{dashboardOpenAPIObservationFieldKeypath(strategy.Duration.GetStartTimestampField()), dashboardOpenAPIObservationFieldKeypath(strategy.Duration.GetDurationField())}
	}
	return dashboardOpenAPIAssertAnnotationKeypaths("logs", got, want)
}

func dashboardOpenAPIAssertSpansAnnotationStrategyValues(strategy *dashboardservice.SpansSourceStrategy, index int, updated bool) error {
	want := [][]string{
		{dashboardOpenAPIUpdatedValue(updated, "startTime", "startTimeUnixNano")},
		{dashboardOpenAPIUpdatedValue(updated, "startTime", "startTimeUnixNano"), dashboardOpenAPIUpdatedValue(updated, "endTime", "endTimeUnixNano")},
		{dashboardOpenAPIUpdatedValue(updated, "startTime", "startTimeUnixNano"), dashboardOpenAPIUpdatedValue(updated, "durationNano", "durationMillis")},
	}[index]
	var got [][]string
	switch index {
	case 0:
		got = [][]string{dashboardOpenAPIObservationFieldKeypath(strategy.Instant.GetTimestampField())}
	case 1:
		got = [][]string{dashboardOpenAPIObservationFieldKeypath(strategy.Range.GetStartTimestampField()), dashboardOpenAPIObservationFieldKeypath(strategy.Range.GetEndTimestampField())}
	case 2:
		got = [][]string{dashboardOpenAPIObservationFieldKeypath(strategy.Duration.GetStartTimestampField()), dashboardOpenAPIObservationFieldKeypath(strategy.Duration.GetDurationField())}
	}
	return dashboardOpenAPIAssertAnnotationKeypaths("spans", got, want)
}

func dashboardOpenAPIObservationFieldKeypath(field dashboardservice.ObservationField) []string {
	return field.GetKeypath()
}

func dashboardOpenAPIAssertAnnotationKeypaths(source string, got [][]string, want []string) error {
	if len(got) != len(want) {
		return fmt.Errorf("REST %s annotation keypaths = %v, want %v", source, got, want)
	}
	for index := range want {
		if len(got[index]) != 1 || got[index][0] != want[index] {
			return fmt.Errorf("REST %s annotation keypaths = %v, want %v", source, got, want)
		}
	}
	return nil
}

func dashboardOpenAPIAnnotationsConfig(name string) string {
	return dashboardOpenAPIAnnotationsConfigVariant(name, false)
}

func dashboardOpenAPIAnnotationsUpdateConfig(name string) string {
	return dashboardOpenAPIAnnotationsConfigVariant(name, true)
}

func dashboardOpenAPIAnnotationsConfigVariant(name string, updated bool) string {
	config := fmt.Sprintf(`
resource "coralogix_dashboard" "test" {
  name        = %q
  description = "Nested structured annotation branches"
  time_frame  = { relative = { duration = "seconds:900" } }
  layout = {
    sections = [{ rows = [{ height = 10, widgets = [{
      title      = "placeholder"
      definition = { line_chart = { query_definitions = [{ query = { metrics = { promql_query = "vector(1)" } } }] } }
    }] }] }]
  }
  annotations = [
    {
      name = "metrics-start", source = { metrics = {
        promql_query = "vector(1)", strategy = { start_time = {} }
        labels = ["service"]
      } }
    },
    {
      name = "manual-instant", source = { manual = {
        strategy = { instant = { value = 42, unit = "unspecified" } }
      } }
    },
    {
      name = "manual-range", source = { manual = {
        strategy = { range = { start_value = 10, end_value = 20, unit = "unspecified" } }
      } }
    },
    {
      name = "logs-instant", source = { logs = {
        lucene_query = "*"
        strategy = { instant = { timestamp_field = { keypath = ["timestamp"], scope = "metadata" } } }
      } }
    },
    {
      name = "logs-range", source = { logs = {
        lucene_query = "*"
        strategy = { range = {
          start_timestamp_field = { keypath = ["start_time"], scope = "user_data" }
          end_timestamp_field   = { keypath = ["end_time"], scope = "user_data" }
        } }
      } }
    },
    {
      name = "logs-duration", source = { logs = {
        lucene_query = "*"
        strategy = { duration = {
          start_timestamp_field = { keypath = ["start_time"], scope = "user_data" }
          duration_field        = { keypath = ["duration_ms"], scope = "user_data" }
        } }
      } }
    },
    {
      name = "spans-instant", source = { spans = {
        lucene_query = "*"
        strategy = { instant = { timestamp_field = { keypath = ["startTime"], scope = "metadata" } } }
      } }
    },
    {
      name = "spans-range", source = { spans = {
        lucene_query = "*"
        strategy = { range = {
          start_timestamp_field = { keypath = ["startTime"], scope = "metadata" }
          end_timestamp_field   = { keypath = ["endTime"], scope = "metadata" }
        } }
      } }
    },
    {
      name = "spans-duration", source = { spans = {
        lucene_query = "*"
        strategy = { duration = {
          start_timestamp_field = { keypath = ["startTime"], scope = "metadata" }
          duration_field        = { keypath = ["durationNano"], scope = "metadata" }
        } }
      } }
    },
  ]
}
`, name)
	if updated {
		config = strings.NewReplacer(
			`promql_query = "vector(1)"`, `promql_query = "vector(2)"`,
			`labels = ["service"]`, `labels = ["job"]`,
			`value = 42`, `value = 84`,
			`start_value = 10, end_value = 20`, `start_value = 20, end_value = 40`,
			"source = { logs = {\n        lucene_query = \"*\"", "source = { logs = {\n        lucene_query = \"coralogix.metadata.severity:INFO\"",
			"source = { spans = {\n        lucene_query = \"*\"", "source = { spans = {\n        lucene_query = \"serviceName:api\"",
			`["timestamp"]`, `["event_timestamp"]`,
			`["start_time"]`, `["range_start"]`,
			`["end_time"]`, `["range_end"]`,
			`["duration_ms"]`, `["elapsed_ms"]`,
			`["startTime"]`, `["startTimeUnixNano"]`,
			`["endTime"]`, `["endTimeUnixNano"]`,
			`["durationNano"]`, `["durationMillis"]`,
		).Replace(config)
	}
	return config
}

func dashboardOpenAPIRunNestedScenario(
	t *testing.T,
	fixture string,
	createConfig func(string) string,
	updateConfig func(string) string,
	stateChecks func(bool) []resource.TestCheckFunc,
	apiCheck func(*dashboardservice.Dashboard, bool) error,
) {
	t.Helper()
	ctx := context.Background()
	var client *dashboardservice.DashboardServiceAPIService
	name := dashboardOpenAPIFixtureName(fixture)
	dashboardIdentity := newDashboardOpenAPIIDTracker(dashboardResourceName, fixture)
	nestedIdentity := newDashboardOpenAPINestedIDTracker(fixture)
	checks := func(updated bool, identityCheck resource.TestCheckFunc) resource.TestCheckFunc {
		checks := append([]resource.TestCheckFunc{
			resource.TestCheckResourceAttrSet(dashboardResourceName, "id"),
			identityCheck,
		}, stateChecks(updated)...)
		checks = append(checks, func(state *terraform.State) error {
			dashboard, err := dashboardOpenAPIFetchDashboard(ctx, client, state, dashboardResourceName, fixture)
			if err != nil {
				return err
			}
			if err := nestedIdentity.CaptureOrAssert(dashboard); err != nil {
				return err
			}
			return apiCheck(dashboard, updated)
		})
		return resource.ComposeAggregateTestCheckFunc(checks...)
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			client = dashboardOpenAPIAcceptanceClient(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDashboardDestroy(t),
		Steps: dashboardOpenAPIStructuredLifecycleSteps(
			dashboardOpenAPILifecyclePhase{
				Config: createConfig(name),
				Check:  checks(false, dashboardIdentity.Capture()),
			},
			[]dashboardOpenAPILifecyclePhase{{
				Config: updateConfig(name),
				Check:  checks(true, dashboardIdentity.AssertUnchanged()),
			}},
			resource.TestStep{
				ResourceName:      dashboardResourceName,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateCheck: dashboardOpenAPIImportDashboardCheck(ctx, &client, fixture, func(dashboard *dashboardservice.Dashboard) error {
					if err := nestedIdentity.CaptureOrAssert(dashboard); err != nil {
						return err
					}
					return apiCheck(dashboard, true)
				}),
			},
		),
	})
}

func dashboardOpenAPIWrapWidgets(name, widgets string) string {
	return fmt.Sprintf(`
resource "coralogix_dashboard" "test" {
  name        = %q
  description = "Nested structured oneOf coverage"
  time_frame  = { relative = { duration = "seconds:900" } }
  layout = {
    sections = [{ rows = [{ height = 24, widgets = [%s] }] }]
  }
}
`, name, widgets)
}

func dashboardOpenAPIFirstRowWidgets(dashboard *dashboardservice.Dashboard) ([]dashboardservice.Widget, error) {
	if dashboard == nil {
		return nil, fmt.Errorf("REST dashboard is nil")
	}
	layout := dashboard.GetLayout()
	sections := layout.GetSections()
	if len(sections) != 1 || len(sections[0].GetRows()) != 1 {
		return nil, fmt.Errorf("REST layout has %d sections and %d first-section rows, want 1/1", len(sections), dashboardOpenAPIFirstSectionRowCount(sections))
	}
	return sections[0].GetRows()[0].GetWidgets(), nil
}

func dashboardOpenAPIJoinErrors(errors []string) error {
	if len(errors) == 0 {
		return nil
	}
	return fmt.Errorf("%s", strings.Join(errors, "; "))
}
