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
	"testing"

	dashboardservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_service"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

const (
	dashboardOpenAPILogsQueryTestName      = "TestAccCoralogixResourceDashboardOpenAPIWidgetLogsQueries"
	dashboardOpenAPIMetricsQueryTestName   = "TestAccCoralogixResourceDashboardOpenAPIWidgetMetricsQueries"
	dashboardOpenAPISpansQueryTestName     = "TestAccCoralogixResourceDashboardOpenAPIWidgetSpansQueries"
	dashboardOpenAPIDataPrimeQueryTestName = "TestAccCoralogixResourceDashboardOpenAPIWidgetDataPrimeQueries"
)

type dashboardStructuredWidgetSpec struct {
	name             string
	definitionBranch string
	queryModel       string
}

var dashboardStructuredQueryWidgets = []dashboardStructuredWidgetSpec{
	{name: "line_chart", definitionBranch: "lineChart", queryModel: "LineChartQuery"},
	{name: "data_table", definitionBranch: "dataTable", queryModel: "DataTableQuery"},
	{name: "gauge", definitionBranch: "gauge", queryModel: "GaugeQuery"},
	{name: "pie_chart", definitionBranch: "pieChart", queryModel: "PieChartQuery"},
	{name: "bar_chart", definitionBranch: "barChart", queryModel: "BarChartQuery"},
	{name: "horizontal_bar_chart", definitionBranch: "horizontalBarChart", queryModel: "HorizontalBarChartQuery"},
	{name: "hexagon", definitionBranch: "hexagon", queryModel: "HexagonQuery"},
}

func TestDashboardOpenAPIStructuredWidgetQueryMatrix(t *testing.T) {
	wantCounts := map[string]int{
		"logs":      7,
		"metrics":   7,
		"spans":     7,
		"dataprime": 6,
	}
	total := 0
	for branch, wantCount := range wantCounts {
		widgets := dashboardOpenAPIStructuredWidgetsForBranch(branch)
		if len(widgets) != wantCount {
			t.Errorf("%s structured query widgets = %d, want %d", branch, len(widgets), wantCount)
		}
		for _, widget := range widgets {
			if branch == "dataprime" && widget.name == "horizontal_bar_chart" {
				t.Error("horizontal_bar_chart.dataprime must remain outside the positive structured query matrix")
			}
		}
		total += len(widgets)
	}
	if total != 27 {
		t.Errorf("HCL-reachable structured widget query branches = %d, want 27", total)
	}
	if got := len(dashboardOpenAPIStructuredWidgetsForBranch("logs")) + 1; got != 8 {
		t.Errorf("structured WidgetDefinition branches including markdown = %d, want 8", got)
	}
}

func TestAccCoralogixResourceDashboardOpenAPIWidgetLogsQueries(t *testing.T) {
	dashboardOpenAPIRunStructuredQueryScenario(t, "logs", true, dashboardOpenAPILogsQueryTestName)
}

func TestAccCoralogixResourceDashboardOpenAPIWidgetMetricsQueries(t *testing.T) {
	dashboardOpenAPIRunStructuredQueryScenario(t, "metrics", false, dashboardOpenAPIMetricsQueryTestName)
}

func TestAccCoralogixResourceDashboardOpenAPIWidgetSpansQueries(t *testing.T) {
	dashboardOpenAPIRunStructuredQueryScenario(t, "spans", false, dashboardOpenAPISpansQueryTestName)
}

func TestAccCoralogixResourceDashboardOpenAPIWidgetDataPrimeQueries(t *testing.T) {
	dashboardOpenAPIRunStructuredQueryScenario(t, "dataprime", false, dashboardOpenAPIDataPrimeQueryTestName)
}

func dashboardOpenAPIRunStructuredQueryScenario(t *testing.T, queryBranch string, includeMarkdown bool, fixture string) {
	t.Helper()

	ctx := context.Background()
	var client *dashboardservice.DashboardServiceAPIService
	dashboardName := dashboardOpenAPIFixtureName(fixture)
	stateChecks := dashboardOpenAPIStructuredQueryStateChecks(queryBranch, includeMarkdown)
	stateChecks = append(stateChecks, func(state *terraform.State) error {
		dashboard, err := dashboardOpenAPIFetchDashboard(ctx, client, state, dashboardResourceName, fixture)
		if err != nil {
			return err
		}
		return dashboardOpenAPIAssertStructuredQueryWidgets(dashboard, queryBranch, includeMarkdown, fixture)
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
				Config: dashboardOpenAPIStructuredDashboardConfig(dashboardName, queryBranch, includeMarkdown),
				Check:  resource.ComposeAggregateTestCheckFunc(stateChecks...),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				ResourceName:      dashboardResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func dashboardOpenAPIStructuredQueryStateChecks(queryBranch string, includeMarkdown bool) []resource.TestCheckFunc {
	widgets := dashboardOpenAPIStructuredWidgetsForBranch(queryBranch)
	checks := []resource.TestCheckFunc{
		resource.TestCheckResourceAttrSet(dashboardResourceName, "id"),
		resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.#", fmt.Sprintf("%d", len(widgets)+boolToInt(includeMarkdown))),
	}

	for index, widget := range widgets {
		basePath := fmt.Sprintf("layout.sections.0.rows.0.widgets.%d", index)
		checks = append(checks,
			resource.TestCheckResourceAttr(dashboardResourceName, basePath+".title", queryBranch+"-"+widget.name),
			resource.TestCheckResourceAttr(
				dashboardResourceName,
				basePath+dashboardOpenAPIQueryStatePath(widget.name, queryBranch),
				dashboardOpenAPIQueryStateValue(widget.name, queryBranch),
			),
		)
		if widget.name == "hexagon" {
			checks = append(checks,
				resource.TestCheckResourceAttr(dashboardResourceName, basePath+".definition.hexagon.min", "0"),
				resource.TestCheckResourceAttr(dashboardResourceName, basePath+".definition.hexagon.max", "100"),
			)
		}
	}

	if includeMarkdown {
		markdownPath := fmt.Sprintf("layout.sections.0.rows.0.widgets.%d.definition.markdown.markdown_text", len(widgets))
		checks = append(checks, resource.TestCheckResourceAttr(dashboardResourceName, markdownPath, "## Structured dashboard coverage"))
	}

	return checks
}

func dashboardOpenAPIQueryStatePath(widget, queryBranch string) string {
	queryPath := ".definition." + widget + ".query."
	if widget == "line_chart" {
		queryPath = ".definition.line_chart.query_definitions.0.query."
	}

	switch queryBranch {
	case "logs":
		switch widget {
		case "line_chart":
			return queryPath + "logs.aggregations.0.type"
		case "data_table":
			return queryPath + "logs.lucene_query"
		case "gauge":
			return queryPath + "logs.logs_aggregation.type"
		default:
			return queryPath + "logs.aggregation.type"
		}
	case "metrics":
		return queryPath + "metrics.promql_query"
	case "spans":
		switch widget {
		case "line_chart":
			return queryPath + "spans.aggregations.0.aggregation_type"
		case "data_table":
			return queryPath + "spans.grouping.aggregations.0.aggregation.aggregation_type"
		case "gauge":
			return queryPath + "spans.spans_aggregation.aggregation_type"
		default:
			return queryPath + "spans.aggregation.aggregation_type"
		}
	case "dataprime":
		return queryPath + "data_prime.query"
	default:
		panic(fmt.Sprintf("unsupported structured dashboard query branch %q", queryBranch))
	}
}

func dashboardOpenAPIQueryStateValue(widget, queryBranch string) string {
	switch queryBranch {
	case "logs":
		if widget == "data_table" {
			return "coralogix.metadata.severity:ERROR"
		}
		return "count"
	case "metrics":
		return "vector(1)"
	case "spans":
		return "unique_count"
	case "dataprime":
		return dashboardOpenAPIDataPrimeQuery()
	default:
		panic(fmt.Sprintf("unsupported structured dashboard query branch %q", queryBranch))
	}
}

func dashboardOpenAPIAssertStructuredQueryWidgets(dashboard *dashboardservice.Dashboard, queryBranch string, includeMarkdown bool, fixture string) error {
	if dashboard == nil {
		return fmt.Errorf("dashboard fixture %q: fetched dashboard is nil", fixture)
	}
	dashboardID := dashboard.GetId()
	layout := dashboard.GetLayout()
	sections := layout.GetSections()
	if len(sections) != 1 || len(sections[0].GetRows()) != 1 {
		return fmt.Errorf("dashboard fixture %q (dashboard %q): layout has %d sections and %d rows in the first section, want 1 and 1", fixture, dashboardID, len(sections), dashboardOpenAPIFirstSectionRowCount(sections))
	}
	widgets := sections[0].GetRows()[0].GetWidgets()
	widgetSpecs := dashboardOpenAPIStructuredWidgetsForBranch(queryBranch)
	wantWidgetCount := len(widgetSpecs) + boolToInt(includeMarkdown)
	if len(widgets) != wantWidgetCount {
		return fmt.Errorf("dashboard fixture %q (dashboard %q): fetched %d widgets, want %d", fixture, dashboardID, len(widgets), wantWidgetCount)
	}

	for index, spec := range widgetSpecs {
		widget := &widgets[index]
		wantTitle := queryBranch + "-" + spec.name
		if widget.GetTitle() != wantTitle {
			return fmt.Errorf("dashboard fixture %q (dashboard %q): widget %d title = %q, want %q", fixture, dashboardID, index, widget.GetTitle(), wantTitle)
		}
		definition := widget.GetDefinition()
		if err := dashboardOpenAPIAssertOneOfBranch(&definition, "WidgetDefinition", spec.definitionBranch, dashboardID, fixture); err != nil {
			return err
		}
		queryCarrier, err := dashboardOpenAPIStructuredQueryCarrier(&definition, spec.name)
		if err != nil {
			return fmt.Errorf("dashboard fixture %q (dashboard %q): widget %q: %w", fixture, dashboardID, spec.name, err)
		}
		if err := dashboardOpenAPIAssertOneOfBranch(queryCarrier, spec.queryModel, queryBranch, dashboardID, fixture); err != nil {
			return err
		}
	}

	if includeMarkdown {
		definition := widgets[len(widgetSpecs)].GetDefinition()
		if err := dashboardOpenAPIAssertOneOfBranch(&definition, "WidgetDefinition", "markdown", dashboardID, fixture); err != nil {
			return err
		}
		if definition.Markdown == nil || definition.Markdown.GetMarkdownText() != "## Structured dashboard coverage" {
			return fmt.Errorf("dashboard fixture %q (dashboard %q): markdown typed field did not round-trip", fixture, dashboardID)
		}
	}

	return nil
}

func dashboardOpenAPIStructuredQueryCarrier(definition *dashboardservice.WidgetDefinition, widget string) (any, error) {
	switch widget {
	case "line_chart":
		if definition.LineChart == nil || len(definition.LineChart.QueryDefinitions) != 1 {
			return nil, fmt.Errorf("lineChart typed field has %d query definitions, want 1", dashboardOpenAPILineChartQueryCount(definition.LineChart))
		}
		return &definition.LineChart.QueryDefinitions[0].Query, nil
	case "data_table":
		if definition.DataTable == nil || definition.DataTable.Query == nil {
			return nil, fmt.Errorf("dataTable.query typed field is nil")
		}
		return definition.DataTable.Query, nil
	case "gauge":
		if definition.Gauge == nil || definition.Gauge.Query == nil {
			return nil, fmt.Errorf("gauge.query typed field is nil")
		}
		return definition.Gauge.Query, nil
	case "pie_chart":
		if definition.PieChart == nil || definition.PieChart.Query == nil {
			return nil, fmt.Errorf("pieChart.query typed field is nil")
		}
		return definition.PieChart.Query, nil
	case "bar_chart":
		if definition.BarChart == nil || definition.BarChart.Query == nil {
			return nil, fmt.Errorf("barChart.query typed field is nil")
		}
		return definition.BarChart.Query, nil
	case "horizontal_bar_chart":
		if definition.HorizontalBarChart == nil || definition.HorizontalBarChart.Query == nil {
			return nil, fmt.Errorf("horizontalBarChart.query typed field is nil")
		}
		return definition.HorizontalBarChart.Query, nil
	case "hexagon":
		if definition.Hexagon == nil || definition.Hexagon.Query == nil {
			return nil, fmt.Errorf("hexagon.query typed field is nil")
		}
		return definition.Hexagon.Query, nil
	default:
		return nil, fmt.Errorf("unsupported structured widget %q", widget)
	}
}

func dashboardOpenAPIStructuredDashboardConfig(name, queryBranch string, includeMarkdown bool) string {
	widgets := ""
	widgetSpecs := dashboardOpenAPIStructuredWidgetsForBranch(queryBranch)
	for index, widget := range widgetSpecs {
		if index > 0 {
			widgets += ",\n"
		}
		widgets += dashboardOpenAPIStructuredWidgetConfig(widget.name, queryBranch)
	}
	if includeMarkdown {
		widgets += `,
        {
          definition = {
            markdown = {
              markdown_text = "## Structured dashboard coverage"
            }
          }
        }`
	}

	return fmt.Sprintf(`
resource "coralogix_dashboard" "test" {
  name        = %q
  description = "Exercises every structured dashboard widget query carrier."
  time_frame = {
    relative = {
      duration = "seconds:900"
    }
  }
  layout = {
    sections = [{
      rows = [{
        height = 19
        widgets = [
%s
        ]
      }]
    }]
  }
}
`, name, widgets)
}

func dashboardOpenAPIStructuredWidgetConfig(widget, queryBranch string) string {
	query := dashboardOpenAPIStructuredQueryConfig(widget, queryBranch)
	widgetBody := fmt.Sprintf("query = {\n%s\n            }", query)

	switch widget {
	case "line_chart":
		widgetBody = fmt.Sprintf("query_definitions = [{\n              query = {\n%s\n              }\n            }]", query)
	case "data_table":
		widgetBody = fmt.Sprintf(`query = {
%s
            }
            results_per_page = 10
            row_style        = "one_line"
            columns = [{
              field = "coralogix.timestamp"
            }]`, query)
	case "gauge":
		widgetBody = fmt.Sprintf(`query = {
%s
            }
            unit = "none"`, query)
	case "pie_chart":
		widgetBody = fmt.Sprintf(`query = {
%s
            }
            label_definition = {}`, query)
	case "hexagon":
		widgetBody = fmt.Sprintf(`query = {
%s
            }
            min = 0
            max = 100`, query)
	}

	return fmt.Sprintf(`        {
          title = %q
          definition = {
            %s = {
              %s
            }
          }
        }`, queryBranch+"-"+widget, widget, widgetBody)
}

func dashboardOpenAPIStructuredQueryConfig(widget, queryBranch string) string {
	const indent = "                "
	switch queryBranch {
	case "logs":
		switch widget {
		case "line_chart":
			return indent + `logs = {
                  lucene_query = "coralogix.metadata.severity:ERROR"
                  aggregations = [{ type = "count" }]
                }`
		case "data_table":
			return indent + `logs = {
                  lucene_query = "coralogix.metadata.severity:ERROR"
                }`
		case "gauge":
			return indent + `logs = {
                  lucene_query = "coralogix.metadata.severity:ERROR"
                  logs_aggregation = { type = "count" }
                }`
		default:
			return indent + `logs = {
                  lucene_query = "coralogix.metadata.severity:ERROR"
                  aggregation = { type = "count" }
                }`
		}
	case "metrics":
		if widget == "pie_chart" {
			return indent + `metrics = {
                  promql_query = "vector(1)"
                  group_names  = ["job"]
                }`
		}
		return indent + `metrics = {
                  promql_query = "vector(1)"
                }`
	case "spans":
		aggregation := `{
                    type             = "dimension"
                    aggregation_type = "unique_count"
                    field            = "trace_id"
                  }`
		switch widget {
		case "line_chart":
			return fmt.Sprintf("%sspans = {\n                  aggregations = [%s]\n                }", indent, aggregation)
		case "data_table":
			return fmt.Sprintf("%sspans = {\n                  grouping = {\n                    aggregations = [{ aggregation = %s }]\n                  }\n                }", indent, aggregation)
		case "gauge":
			return fmt.Sprintf("%sspans = {\n                  spans_aggregation = %s\n                }", indent, aggregation)
		default:
			return fmt.Sprintf("%sspans = {\n                  aggregation = %s\n                }", indent, aggregation)
		}
	case "dataprime":
		groupNames := ""
		if widget == "pie_chart" {
			groupNames = "\n                  group_names = [\"c\"]"
		}
		return fmt.Sprintf("%sdata_prime = {\n                  query = %q%s\n                }", indent, dashboardOpenAPIDataPrimeQuery(), groupNames)
	default:
		panic(fmt.Sprintf("unsupported structured dashboard query branch %q", queryBranch))
	}
}

func dashboardOpenAPIDataPrimeQuery() string {
	return "source logs\n| filter 1 == 1\n| aggregate count() as c\n| choose c"
}

func dashboardOpenAPIStructuredWidgetsForBranch(queryBranch string) []dashboardStructuredWidgetSpec {
	if queryBranch != "dataprime" {
		return dashboardStructuredQueryWidgets
	}

	widgets := make([]dashboardStructuredWidgetSpec, 0, len(dashboardStructuredQueryWidgets)-1)
	for _, widget := range dashboardStructuredQueryWidgets {
		if widget.name != "horizontal_bar_chart" {
			widgets = append(widgets, widget)
		}
	}
	return widgets
}

func dashboardOpenAPIFirstSectionRowCount(sections []dashboardservice.Section) int {
	if len(sections) == 0 {
		return 0
	}
	return len(sections[0].GetRows())
}

func dashboardOpenAPILineChartQueryCount(lineChart *dashboardservice.LineChart) int {
	if lineChart == nil {
		return 0
	}
	return len(lineChart.QueryDefinitions)
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
