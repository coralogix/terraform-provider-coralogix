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
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

const (
	dashboardOpenAPINestedPresentationTestName = "TestAccCoralogixResourceDashboardOpenAPINestedPresentationBranches"
	dashboardOpenAPILogsAggregationTestName    = "TestAccCoralogixResourceDashboardOpenAPILogsAggregationBranches"
	dashboardOpenAPISpansAndFiltersTestName    = "TestAccCoralogixResourceDashboardOpenAPISpansAndFilterBranches"
	dashboardOpenAPIVariablesTestName          = "TestAccCoralogixResourceDashboardOpenAPIVariableBranches"
	dashboardOpenAPIAnnotationsTestName        = "TestAccCoralogixResourceDashboardOpenAPIAnnotationBranches"
)

func TestDashboardOpenAPINestedAcceptanceConfigsParse(t *testing.T) {
	configs := map[string]string{
		"presentation-relative": dashboardOpenAPIPresentationConfig("dashboard", "folder", "relative", "absolute", "two_minutes", "id"),
		"presentation-absolute": dashboardOpenAPIPresentationConfig("dashboard", "folder", "absolute", "relative", "five_minutes", "path"),
		"logs-aggregations":     dashboardOpenAPILogsAggregationConfig("dashboard"),
		"spans-and-filters":     dashboardOpenAPISpansAndFiltersConfig("dashboard"),
		"variables":             dashboardOpenAPIVariablesConfig("dashboard"),
		"annotations":           dashboardOpenAPIAnnotationsConfig("dashboard"),
		"dataprime-filters":     dashboardOpenAPIStructuredDashboardConfig("dashboard", "dataprime", false),
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
	ctx := context.Background()
	var client *dashboardservice.DashboardServiceAPIService
	fixture := dashboardOpenAPINestedPresentationTestName
	dashboardName := dashboardOpenAPIFixtureName(fixture)
	folderName := dashboardOpenAPIFixtureName(fixture + "-folder")
	identity := newDashboardOpenAPIIDTracker(dashboardResourceName, fixture)
	checkPresentation := func(dashboardTimeFrame, queryTimeFrame, refresh string) resource.TestCheckFunc {
		return func(state *terraform.State) error {
			dashboard, err := dashboardOpenAPIFetchDashboard(ctx, client, state, dashboardResourceName, fixture)
			if err != nil {
				return err
			}
			if err := dashboardOpenAPIAssertPresentation(dashboard, fixture, dashboardTimeFrame, queryTimeFrame, refresh); err != nil {
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

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			client = dashboardOpenAPIAcceptanceClient(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDashboardDestroy,
		Steps: []resource.TestStep{
			{
				Config: dashboardOpenAPIPresentationConfig(dashboardName, folderName, "relative", "absolute", "two_minutes", "id"),
				Check: resource.ComposeAggregateTestCheckFunc(
					identity.Capture(),
					dashboardOpenAPIPresentationStateChecks("relative", "absolute", "two_minutes", "id", folderName),
					checkPresentation("relativeTimeFrame", "absoluteTimeFrame", "twoMinutes"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			{
				Config: dashboardOpenAPIPresentationConfig(dashboardName, folderName, "absolute", "relative", "five_minutes", "path"),
				Check: resource.ComposeAggregateTestCheckFunc(
					identity.AssertUnchanged(),
					dashboardOpenAPIPresentationStateChecks("absolute", "relative", "five_minutes", "path", folderName),
					checkPresentation("absoluteTimeFrame", "relativeTimeFrame", "fiveMinutes"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			{
				Config: dashboardOpenAPIPresentationConfig(dashboardName, folderName, "absolute", "relative", "off", "path"),
				Check: resource.ComposeAggregateTestCheckFunc(
					identity.AssertUnchanged(),
					dashboardOpenAPIPresentationStateChecks("absolute", "relative", "off", "path", folderName),
					checkPresentation("absoluteTimeFrame", "relativeTimeFrame", "off"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			{
				ResourceName:            dashboardResourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"folder"},
			},
		},
	})
}

func dashboardOpenAPIPresentationStateChecks(dashboardTimeFrame, queryTimeFrame, refresh, folderSelector, folderName string) resource.TestCheckFunc {
	checks := []resource.TestCheckFunc{
		resource.TestCheckResourceAttr(dashboardResourceName, "auto_refresh.type", refresh),
		resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.#", "6"),
		resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.line_chart.query_definitions.0.resolution.interval", "seconds:60"),
		resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.1.definition.line_chart.query_definitions.0.resolution.buckets_presented", "20"),
		resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.2.definition.bar_chart.colors_by", "stack"),
		resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.3.definition.bar_chart.colors_by", "group_by"),
		resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.3.definition.bar_chart.xaxis.time.interval", "1m0s"),
		resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.3.definition.bar_chart.xaxis.time.buckets_presented", "30"),
		resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.4.definition.horizontal_bar_chart.colors_by", "aggregation"),
		resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.4.definition.horizontal_bar_chart.y_axis_view_by", "category"),
		resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.5.definition.horizontal_bar_chart.y_axis_view_by", "value"),
	}
	if dashboardTimeFrame == "relative" {
		checks = append(checks, resource.TestCheckResourceAttr(dashboardResourceName, "time_frame.relative.duration", "seconds:900"))
	} else {
		checks = append(checks,
			resource.TestCheckResourceAttr(dashboardResourceName, "time_frame.absolute.start", "2026-01-01T00:00:00Z"),
			resource.TestCheckResourceAttr(dashboardResourceName, "time_frame.absolute.end", "2026-01-01T01:00:00Z"),
		)
	}
	queryPath := "layout.sections.0.rows.0.widgets.0.definition.line_chart.query_definitions.0.query.metrics.time_frame."
	if queryTimeFrame == "relative" {
		checks = append(checks, resource.TestCheckResourceAttr(dashboardResourceName, queryPath+"relative.duration", "seconds:900"))
	} else {
		checks = append(checks,
			resource.TestCheckResourceAttr(dashboardResourceName, queryPath+"absolute.start", "2026-02-01T00:00:00Z"),
			resource.TestCheckResourceAttr(dashboardResourceName, queryPath+"absolute.end", "2026-02-01T00:15:00Z"),
		)
	}
	if folderSelector == "id" {
		checks = append(checks, resource.TestCheckResourceAttrSet(dashboardResourceName, "folder.id"))
	} else {
		checks = append(checks, resource.TestCheckResourceAttr(dashboardResourceName, "folder.path", folderName))
	}
	return resource.ComposeAggregateTestCheckFunc(checks...)
}

func dashboardOpenAPIAssertPresentation(dashboard *dashboardservice.Dashboard, fixture, dashboardTimeFrame, queryTimeFrame, refresh string) error {
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
	if len(widgets) != 6 {
		return fmt.Errorf("dashboard fixture %q: REST widgets = %d, want 6", fixture, len(widgets))
	}

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

	valueBar := widgets[2].GetDefinition().BarChart
	timeBar := widgets[3].GetDefinition().BarChart
	categoryBar := widgets[4].GetDefinition().HorizontalBarChart
	valueHorizontalBar := widgets[5].GetDefinition().HorizontalBarChart
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
          {
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
          },
          {
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
          },
          {
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
          },
        ]
      }]
    }]
  }
}
`, folderName, name, dashboardTF, refresh, folder, queryTF)
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
	checks := []resource.TestCheckFunc{
		resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.line_chart.query_definitions.#", "7"),
		resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.line_chart.query_definitions.1.query.logs.aggregations.0.field", "coralogix.metadata.applicationName"),
		resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.line_chart.query_definitions.6.query.logs.aggregations.0.percent", "95"),
	}
	for index, aggregation := range aggregations {
		checks = append(checks, resource.TestCheckResourceAttr(
			dashboardResourceName,
			fmt.Sprintf("layout.sections.0.rows.0.widgets.0.definition.line_chart.query_definitions.%d.query.logs.aggregations.0.type", index),
			aggregation.typeName,
		))
	}
	dashboardOpenAPIRunNestedScenario(t, dashboardOpenAPILogsAggregationTestName, dashboardOpenAPILogsAggregationConfig, checks, func(dashboard *dashboardservice.Dashboard) error {
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
			if err := dashboardOpenAPIAssertOneOfBranch(&logs.Aggregations[0], "LogsAggregation", aggregation.branch, dashboard.GetId(), dashboardOpenAPILogsAggregationTestName); err != nil {
				return err
			}
		}
		countDistinct := lineChart.QueryDefinitions[1].Query.Logs.Aggregations[0]
		if countDistinct.CountDistinct == nil || countDistinct.CountDistinct.GetField() != "coralogix.metadata.applicationName" {
			return fmt.Errorf("REST countDistinct field did not round-trip")
		}
		percentile := lineChart.QueryDefinitions[6].Query.Logs.Aggregations[0]
		if percentile.Percentile == nil || percentile.Percentile.GetPercent() != 95 {
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
	return dashboardOpenAPIWrapWidgets(name, `{
  title = "all-log-aggregations"
  definition = { line_chart = {
    query_definitions = [
      { query = { logs = { aggregations = [{ type = "count" }] } } },
      { query = { logs = { aggregations = [{ type = "count_distinct", field = "coralogix.metadata.applicationName" }] } } },
      { query = { logs = { aggregations = [{ type = "sum", field = "latency" }] } } },
      { query = { logs = { aggregations = [{ type = "avg", field = "latency" }] } } },
      { query = { logs = { aggregations = [{ type = "min", field = "latency" }] } } },
      { query = { logs = { aggregations = [{ type = "max", field = "latency" }] } } },
      { query = { logs = { aggregations = [{ type = "percentile", field = "latency", percent = 95 }] } } },
    ]
  } }
}`)
}

func TestAccCoralogixResourceDashboardOpenAPISpansAndFilterBranches(t *testing.T) {
	checks := []resource.TestCheckFunc{
		resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.line_chart.query_definitions.0.query.spans.aggregations.#", "2"),
		resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.line_chart.query_definitions.0.query.spans.aggregations.0.type", "metric"),
		resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.line_chart.query_definitions.0.query.spans.aggregations.0.aggregation_type", "avg"),
		resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.line_chart.query_definitions.0.query.spans.aggregations.0.field", "duration"),
		resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.line_chart.query_definitions.0.query.spans.aggregations.1.type", "dimension"),
		resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.line_chart.query_definitions.0.query.spans.aggregations.1.field", "trace_id"),
		resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.line_chart.query_definitions.0.query.spans.group_by.#", "3"),
		resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.line_chart.query_definitions.0.query.spans.group_by.0.type", "metadata"),
		resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.line_chart.query_definitions.0.query.spans.group_by.1.type", "tag"),
		resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.line_chart.query_definitions.0.query.spans.group_by.2.type", "process_tag"),
		resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.line_chart.query_definitions.0.query.spans.group_by.2.value", "service.version"),
		resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.1.definition.data_table.query.logs.filters.#", "2"),
		resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.1.definition.data_table.query.logs.filters.0.field", "coralogix.metadata.applicationName"),
		resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.1.definition.data_table.query.logs.filters.0.operator.type", "equals"),
		resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.1.definition.data_table.query.logs.filters.0.operator.selected_values.#", "0"),
		resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.1.definition.data_table.query.logs.filters.1.observation_field.keypath.0", "applicationName"),
		resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.1.definition.data_table.query.logs.filters.1.operator.type", "not_equals"),
		resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.1.definition.data_table.query.logs.filters.1.operator.selected_values.0", "production"),
	}
	dashboardOpenAPIRunNestedScenario(t, dashboardOpenAPISpansAndFiltersTestName, dashboardOpenAPISpansAndFiltersConfig, checks, func(dashboard *dashboardservice.Dashboard) error {
		widgets, err := dashboardOpenAPIFirstRowWidgets(dashboard)
		if err != nil {
			return err
		}
		spans := widgets[0].GetDefinition().LineChart.QueryDefinitions[0].Query.Spans
		if spans == nil || len(spans.Aggregations) != 2 || len(spans.GroupBy) != 3 {
			return fmt.Errorf("REST spans aggregation/field lists did not round-trip")
		}
		if err := dashboardOpenAPIAssertOneOfBranch(&spans.Aggregations[0], "SpansAggregation", "metricAggregation", dashboard.GetId(), dashboardOpenAPISpansAndFiltersTestName); err != nil {
			return err
		}
		if err := dashboardOpenAPIAssertOneOfBranch(&spans.Aggregations[1], "SpansAggregation", "dimensionAggregation", dashboard.GetId(), dashboardOpenAPISpansAndFiltersTestName); err != nil {
			return err
		}
		for index, branch := range []string{"metadataField", "tagField", "processTagField"} {
			if err := dashboardOpenAPIAssertOneOfBranch(&spans.GroupBy[index], "SpanField", branch, dashboard.GetId(), dashboardOpenAPISpansAndFiltersTestName); err != nil {
				return err
			}
		}
		filters := widgets[1].GetDefinition().DataTable.Query.Logs.Filters
		if len(filters) != 2 || filters[0].Field == nil || filters[0].ObservationField != nil || filters[1].Field != nil || filters[1].ObservationField == nil {
			return fmt.Errorf("REST legacy-field/observation-field targets did not round-trip")
		}
		if err := dashboardOpenAPIAssertOneOfBranch(filters[0].Operator, "FilterOperator", "equals", dashboard.GetId(), dashboardOpenAPISpansAndFiltersTestName); err != nil {
			return err
		}
		if err := dashboardOpenAPIAssertOneOfBranch(filters[0].Operator.Equals.Selection, "EqualsSelection", "all", dashboard.GetId(), dashboardOpenAPISpansAndFiltersTestName); err != nil {
			return err
		}
		return dashboardOpenAPIAssertOneOfBranch(filters[1].Operator, "FilterOperator", "notEquals", dashboard.GetId(), dashboardOpenAPISpansAndFiltersTestName)
	})
}

func dashboardOpenAPISpansAndFiltersConfig(name string) string {
	return dashboardOpenAPIWrapWidgets(name, `{
  title = "span-unions"
  definition = { line_chart = {
    query_definitions = [{ query = { spans = {
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
}`)
}

func TestAccCoralogixResourceDashboardOpenAPIVariableBranches(t *testing.T) {
	checks := []resource.TestCheckFunc{
		resource.TestCheckResourceAttr(dashboardResourceName, "variables.#", "11"),
		resource.TestCheckResourceAttr(dashboardResourceName, "variables.0.definition.multi_select.selected_values.#", "0"),
		resource.TestCheckResourceAttr(dashboardResourceName, "variables.0.definition.multi_select.source.logs_path", "coralogix.metadata.applicationName"),
		resource.TestCheckResourceAttr(dashboardResourceName, "variables.1.definition.multi_select.source.metric_label.metric_name", "http_requests_total"),
		resource.TestCheckResourceAttr(dashboardResourceName, "variables.1.definition.multi_select.source.metric_label.label", "service"),
		resource.TestCheckResourceAttr(dashboardResourceName, "variables.2.definition.multi_select.source.constant_list.0", "http_requests_total"),
		resource.TestCheckResourceAttr(dashboardResourceName, "variables.3.definition.multi_select.source.span_field.type", "process_tag"),
		resource.TestCheckResourceAttr(dashboardResourceName, "variables.4.definition.multi_select.source.query.query.logs.field_name.log_regex", ".*"),
		resource.TestCheckResourceAttr(dashboardResourceName, "variables.5.definition.multi_select.source.query.query.logs.field_value.observation_field.keypath.0", "applicationName"),
		resource.TestCheckResourceAttr(dashboardResourceName, "variables.6.definition.multi_select.source.query.query.metrics.metric_name.metric_regex", "http_.*"),
		resource.TestCheckResourceAttr(dashboardResourceName, "variables.7.definition.multi_select.source.query.query.metrics.label_name.metric_regex", "http_.*"),
		resource.TestCheckResourceAttr(dashboardResourceName, "variables.8.definition.multi_select.source.query.query.metrics.label_value.metric_name.variable_name", "source_metric"),
		resource.TestCheckResourceAttr(dashboardResourceName, "variables.8.definition.multi_select.source.query.query.metrics.label_value.label_filters.0.operator.type", "not_equals"),
		resource.TestCheckResourceAttr(dashboardResourceName, "variables.8.definition.multi_select.source.query.query.metrics.label_value.label_filters.0.operator.selected_values.0.variable_name", "source_metric"),
		resource.TestCheckResourceAttr(dashboardResourceName, "variables.8.definition.multi_select.source.query.query.metrics.label_value.label_filters.1.operator.type", "equals"),
		resource.TestCheckResourceAttr(dashboardResourceName, "variables.9.definition.multi_select.source.query.query.spans.field_name.span_regex", ".*"),
		resource.TestCheckResourceAttr(dashboardResourceName, "variables.10.definition.multi_select.source.query.query.spans.field_value.type", "tag"),
		resource.TestCheckResourceAttr(dashboardResourceName, "variables.10.definition.multi_select.source.query.query.spans.field_value.value", "http.method"),
	}
	dashboardOpenAPIRunNestedScenario(t, dashboardOpenAPIVariablesTestName, dashboardOpenAPIVariablesConfig, checks, func(dashboard *dashboardservice.Dashboard) error {
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
		if variables[0].Definition.MultiSelect.Source.LogsPath == nil || variables[0].Definition.MultiSelect.Source.LogsPath.GetValue() != "coralogix.metadata.applicationName" {
			return fmt.Errorf("REST logsPath source did not round-trip")
		}
		if variables[1].Definition.MultiSelect.Source.MetricLabel == nil || variables[1].Definition.MultiSelect.Source.MetricLabel.GetLabel() != "service" {
			return fmt.Errorf("REST metricLabel source did not round-trip")
		}
		if variables[3].Definition.MultiSelect.Source.SpanField == nil || variables[3].Definition.MultiSelect.Source.SpanField.Value == nil {
			return fmt.Errorf("REST spanField source did not round-trip")
		}
		if err := dashboardOpenAPIAssertOneOfBranch(variables[3].Definition.MultiSelect.Source.SpanField.Value, "SpanField", "processTagField", dashboard.GetId(), dashboardOpenAPIVariablesTestName); err != nil {
			return err
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
		for index, branch := range []string{"metricName", "labelName", "labelValue"} {
			if err := dashboardOpenAPIAssertOneOfBranch(queries[index+2].MetricsQuery.Type, "QueryMetricsQueryType", branch, dashboard.GetId(), dashboardOpenAPIVariablesTestName); err != nil {
				return err
			}
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
		if err := dashboardOpenAPIAssertOneOfBranch(queries[5].SpansQuery.Type, "QuerySpansQueryType", "fieldName", dashboard.GetId(), dashboardOpenAPIVariablesTestName); err != nil {
			return err
		}
		return dashboardOpenAPIAssertOneOfBranch(queries[6].SpansQuery.Type, "QuerySpansQueryType", "fieldValue", dashboard.GetId(), dashboardOpenAPIVariablesTestName)
	})
}

func dashboardOpenAPIVariablesConfig(name string) string {
	return fmt.Sprintf(`
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
}

func TestAccCoralogixResourceDashboardOpenAPIAnnotationBranches(t *testing.T) {
	ctx := context.Background()
	var client *dashboardservice.DashboardServiceAPIService
	fixture := dashboardOpenAPIAnnotationsTestName
	name := dashboardOpenAPIFixtureName(fixture)
	annotationIDs := newDashboardOpenAPIAnnotationIDTracker(fixture)
	checks := []resource.TestCheckFunc{
		resource.TestCheckResourceAttr(dashboardResourceName, "annotations.#", "9"),
		resource.TestCheckResourceAttr(dashboardResourceName, "annotations.0.source.metrics.promql_query", "vector(1)"),
		resource.TestCheckResourceAttr(dashboardResourceName, "annotations.1.source.manual.strategy.instant.value", "42"),
		resource.TestCheckResourceAttr(dashboardResourceName, "annotations.2.source.manual.strategy.range.start_value", "10"),
		resource.TestCheckResourceAttr(dashboardResourceName, "annotations.3.source.logs.lucene_query", "*"),
		resource.TestCheckResourceAttr(dashboardResourceName, "annotations.3.source.logs.strategy.instant.timestamp_field.keypath.0", "timestamp"),
		resource.TestCheckResourceAttr(dashboardResourceName, "annotations.4.source.logs.strategy.range.start_timestamp_field.keypath.0", "start_time"),
		resource.TestCheckResourceAttr(dashboardResourceName, "annotations.5.source.logs.strategy.duration.duration_field.keypath.0", "duration_ms"),
		resource.TestCheckResourceAttr(dashboardResourceName, "annotations.6.source.spans.lucene_query", "*"),
		resource.TestCheckResourceAttr(dashboardResourceName, "annotations.6.source.spans.strategy.instant.timestamp_field.keypath.0", "startTime"),
		resource.TestCheckResourceAttr(dashboardResourceName, "annotations.7.source.spans.strategy.range.end_timestamp_field.keypath.0", "endTime"),
		resource.TestCheckResourceAttr(dashboardResourceName, "annotations.8.source.spans.strategy.duration.duration_field.keypath.0", "durationNano"),
		annotationIDs.CaptureOrAssert(),
	}

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			client = dashboardOpenAPIAcceptanceClient(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDashboardDestroy,
		Steps: []resource.TestStep{
			{
				Config: dashboardOpenAPIAnnotationsConfig(name),
				Check: resource.ComposeAggregateTestCheckFunc(append(checks, func(state *terraform.State) error {
					dashboard, err := dashboardOpenAPIFetchDashboard(ctx, client, state, dashboardResourceName, fixture)
					if err != nil {
						return err
					}
					return dashboardOpenAPIAssertAnnotations(dashboard, fixture)
				})...),
				ConfigPlanChecks: resource.ConfigPlanChecks{PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			{
				ResourceName:      dashboardResourceName,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateCheck:  annotationIDs.AssertImported(),
			},
		},
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

func dashboardOpenAPIAssertAnnotations(dashboard *dashboardservice.Dashboard, fixture string) error {
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
	for index, branch := range []string{"instant", "range"} {
		strategy := annotations[index+1].Source.Manual.Strategy
		if err := dashboardOpenAPIAssertOneOfBranch(strategy, "ManualSourceStrategy", branch, dashboard.GetId(), fixture); err != nil {
			return err
		}
	}
	for index, branch := range []string{"instant", "range", "duration"} {
		logs := annotations[index+3].Source.Logs
		if logs == nil || logs.Strategy == nil {
			return fmt.Errorf("dashboard fixture %q: REST logs annotation %d strategy is nil", fixture, index)
		}
		if logs.LuceneQuery == nil || logs.LuceneQuery.GetValue() != "*" {
			return fmt.Errorf("dashboard fixture %q: REST logs annotation %d lucene query did not round-trip", fixture, index)
		}
		if logs.DataModeType != nil && *logs.DataModeType != dashboardservice.V1COMMONDATAMODETYPE_DATA_MODE_TYPE_HIGH_UNSPECIFIED {
			return fmt.Errorf("dashboard fixture %q: backend normalized unset logs annotation dataModeType to unexpected value %q", fixture, *logs.DataModeType)
		}
		if err := dashboardOpenAPIAssertOneOfBranch(logs.Strategy, "LogsSourceStrategy", branch, dashboard.GetId(), fixture); err != nil {
			return err
		}
	}
	for index, branch := range []string{"instant", "range", "duration"} {
		spans := annotations[index+6].Source.Spans
		if spans == nil || spans.Strategy == nil {
			return fmt.Errorf("dashboard fixture %q: REST spans annotation %d strategy is nil", fixture, index)
		}
		if spans.LuceneQuery == nil || spans.LuceneQuery.GetValue() != "*" {
			return fmt.Errorf("dashboard fixture %q: REST spans annotation %d lucene query did not round-trip", fixture, index)
		}
		if spans.DataModeType != nil && *spans.DataModeType != dashboardservice.V1COMMONDATAMODETYPE_DATA_MODE_TYPE_HIGH_UNSPECIFIED {
			return fmt.Errorf("dashboard fixture %q: backend normalized unset spans annotation dataModeType to unexpected value %q", fixture, *spans.DataModeType)
		}
		if err := dashboardOpenAPIAssertOneOfBranch(spans.Strategy, "SpansSourceStrategy", branch, dashboard.GetId(), fixture); err != nil {
			return err
		}
	}
	return nil
}

func dashboardOpenAPIAnnotationsConfig(name string) string {
	return fmt.Sprintf(`
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
}

func dashboardOpenAPIRunNestedScenario(
	t *testing.T,
	fixture string,
	config func(string) string,
	stateChecks []resource.TestCheckFunc,
	apiCheck func(*dashboardservice.Dashboard) error,
) {
	t.Helper()
	ctx := context.Background()
	var client *dashboardservice.DashboardServiceAPIService
	name := dashboardOpenAPIFixtureName(fixture)
	checks := append([]resource.TestCheckFunc{
		resource.TestCheckResourceAttrSet(dashboardResourceName, "id"),
	}, stateChecks...)
	checks = append(checks, func(state *terraform.State) error {
		dashboard, err := dashboardOpenAPIFetchDashboard(ctx, client, state, dashboardResourceName, fixture)
		if err != nil {
			return err
		}
		return apiCheck(dashboard)
	})

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			client = dashboardOpenAPIAcceptanceClient(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDashboardDestroy,
		Steps: []resource.TestStep{
			{
				Config:           config(name),
				Check:            resource.ComposeAggregateTestCheckFunc(checks...),
				ConfigPlanChecks: resource.ConfigPlanChecks{PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			{ResourceName: dashboardResourceName, ImportState: true, ImportStateVerify: true},
		},
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
