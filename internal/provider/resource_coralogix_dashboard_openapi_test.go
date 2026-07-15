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
	"time"

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

type dashboardOpenAPILifecyclePhase struct {
	Config string
	Check  resource.TestCheckFunc
}

func dashboardOpenAPIStructuredLifecycleSteps(
	create dashboardOpenAPILifecyclePhase,
	updates []dashboardOpenAPILifecyclePhase,
	importStep resource.TestStep,
) []resource.TestStep {
	if create.Config == "" || create.Check == nil {
		panic("structured dashboard lifecycle requires a create config and check")
	}
	if len(updates) == 0 {
		panic("structured dashboard lifecycle requires at least one update")
	}
	if !importStep.ImportState || !importStep.ImportStateVerify || importStep.ImportStateCheck == nil {
		panic("structured dashboard lifecycle requires verified import with structured checks")
	}

	steps := []resource.TestStep{{
		Config: create.Config,
		Check:  create.Check,
		ConfigPlanChecks: resource.ConfigPlanChecks{
			PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
		},
	}}
	previousConfig := create.Config
	for _, update := range updates {
		if update.Config == "" || update.Config == previousConfig || update.Check == nil {
			panic("structured dashboard lifecycle requires each update to change config and retain checks")
		}
		steps = append(steps, resource.TestStep{
			Config: update.Config,
			Check:  update.Check,
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(dashboardResourceName, plancheck.ResourceActionUpdate),
				},
				PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
			},
		})
		previousConfig = update.Config
	}
	return append(steps, importStep)
}

func TestDashboardOpenAPIStructuredLifecycleContract(t *testing.T) {
	check := func(*terraform.State) error { return nil }
	steps := dashboardOpenAPIStructuredLifecycleSteps(
		dashboardOpenAPILifecyclePhase{Config: "create", Check: check},
		[]dashboardOpenAPILifecyclePhase{{Config: "update", Check: check}},
		resource.TestStep{
			ResourceName:      dashboardResourceName,
			ImportState:       true,
			ImportStateVerify: true,
			ImportStateCheck:  func([]*terraform.InstanceState) error { return nil },
		},
	)
	if len(steps) != 3 {
		t.Fatalf("structured lifecycle steps = %d, want create/update/import", len(steps))
	}
	if len(steps[0].ConfigPlanChecks.PostApplyPostRefresh) == 0 {
		t.Fatal("structured lifecycle create omits the post-refresh empty-plan contract")
	}
	if len(steps[1].ConfigPlanChecks.PreApply) == 0 || len(steps[1].ConfigPlanChecks.PostApplyPostRefresh) == 0 {
		t.Fatal("structured lifecycle update omits the update-action or empty-plan contract")
	}
	if !steps[2].ImportState || !steps[2].ImportStateVerify || steps[2].ImportStateCheck == nil {
		t.Fatal("structured lifecycle import omits verification or structured checks")
	}
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
		"dataprime": 7,
	}
	total := 0
	for branch, wantCount := range wantCounts {
		widgets := dashboardOpenAPIStructuredWidgetsForBranch(branch)
		if len(widgets) != wantCount {
			t.Errorf("%s structured query widgets = %d, want %d", branch, len(widgets), wantCount)
		}
		total += len(widgets)
	}
	if total != 28 {
		t.Errorf("HCL-reachable structured widget query branches = %d, want 28", total)
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
	dashboardIdentity := newDashboardOpenAPIIDTracker(dashboardResourceName, fixture)
	nestedIdentity := newDashboardOpenAPINestedIDTracker(fixture)
	checks := func(updated bool, identityCheck resource.TestCheckFunc) resource.TestCheckFunc {
		stateChecks := dashboardOpenAPIStructuredQueryStateChecks(queryBranch, includeMarkdown, updated)
		stateChecks = append(stateChecks, identityCheck, func(state *terraform.State) error {
			dashboard, err := dashboardOpenAPIFetchDashboard(ctx, client, state, dashboardResourceName, fixture)
			if err != nil {
				return err
			}
			if err := nestedIdentity.CaptureOrAssert(dashboard); err != nil {
				return err
			}
			return dashboardOpenAPIAssertStructuredQueryWidgets(dashboard, queryBranch, includeMarkdown, fixture, updated)
		})
		return resource.ComposeAggregateTestCheckFunc(stateChecks...)
	}

	steps := dashboardOpenAPIStructuredLifecycleSteps(
		dashboardOpenAPILifecyclePhase{
			Config: dashboardOpenAPIStructuredDashboardConfig(dashboardName, queryBranch, includeMarkdown),
			Check:  checks(false, dashboardIdentity.Capture()),
		},
		[]dashboardOpenAPILifecyclePhase{{
			Config: dashboardOpenAPIStructuredDashboardUpdateConfig(dashboardName, queryBranch, includeMarkdown),
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
				return dashboardOpenAPIAssertStructuredQueryWidgets(dashboard, queryBranch, includeMarkdown, fixture, true)
			}),
		},
	)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			client = dashboardOpenAPIAcceptanceClient(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDashboardDestroy(t),
		Steps:                    steps,
	})
}

func dashboardOpenAPIStructuredQueryStateChecks(queryBranch string, includeMarkdown, updated bool) []resource.TestCheckFunc {
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
				dashboardOpenAPIQueryStateValue(widget.name, queryBranch, updated),
			),
		)
		if widget.name == "hexagon" {
			checks = append(checks,
				resource.TestCheckResourceAttr(dashboardResourceName, basePath+".definition.hexagon.min", "0"),
				resource.TestCheckResourceAttr(dashboardResourceName, basePath+".definition.hexagon.max", "100"),
			)
		}
		if queryBranch == "dataprime" && index < 3 {
			filterBranch := []string{"logs", "spans", "metrics"}[index]
			checks = append(checks,
				resource.TestCheckResourceAttr(
					dashboardResourceName,
					basePath+dashboardOpenAPIDataPrimeFiltersStatePath(widget.name)+".#",
					"1",
				),
				resource.TestCheckResourceAttr(
					dashboardResourceName,
					basePath+dashboardOpenAPIDataPrimeFilterStatePath(widget.name, filterBranch),
					dashboardOpenAPIDataPrimeFilterStateValue(filterBranch, updated),
				),
			)
		}
		if queryBranch == "dataprime" && widget.name == "horizontal_bar_chart" {
			queryPath := basePath + ".definition.horizontal_bar_chart.query.data_prime"
			checks = append(checks,
				resource.TestCheckResourceAttr(dashboardResourceName, queryPath+".filters.#", "0"),
				resource.TestCheckResourceAttr(dashboardResourceName, queryPath+".group_names.#", "1"),
				resource.TestCheckResourceAttr(dashboardResourceName, queryPath+".group_names.0", dashboardOpenAPIDataPrimeGroupName(updated)),
				resource.TestCheckResourceAttr(dashboardResourceName, queryPath+".stacked_group_name", dashboardOpenAPIDataPrimeStackedGroupName(updated)),
			)
			if updated {
				checks = append(checks,
					resource.TestCheckResourceAttr(dashboardResourceName, queryPath+".time_frame.absolute.start", "2026-02-01T00:00:00Z"),
					resource.TestCheckResourceAttr(dashboardResourceName, queryPath+".time_frame.absolute.end", "2026-02-01T00:15:00Z"),
				)
			} else {
				checks = append(checks, resource.TestCheckResourceAttr(dashboardResourceName, queryPath+".time_frame.relative.duration", "seconds:900"))
			}
		}
	}

	if includeMarkdown {
		markdownPath := fmt.Sprintf("layout.sections.0.rows.0.widgets.%d.definition.markdown.markdown_text", len(widgets))
		checks = append(checks, resource.TestCheckResourceAttr(dashboardResourceName, markdownPath, dashboardOpenAPIMarkdownText(updated)))
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
		return queryPath + "logs.lucene_query"
	case "metrics":
		return queryPath + "metrics.promql_query"
	case "spans":
		return queryPath + "spans.lucene_query"
	case "dataprime":
		return queryPath + "data_prime.query"
	default:
		panic(fmt.Sprintf("unsupported structured dashboard query branch %q", queryBranch))
	}
}

func dashboardOpenAPIQueryStateValue(widget, queryBranch string, updated bool) string {
	switch queryBranch {
	case "logs":
		return dashboardOpenAPILuceneQuery(updated)
	case "metrics":
		return dashboardOpenAPIPromQLQuery(updated)
	case "spans":
		return dashboardOpenAPISpansLuceneQuery(updated)
	case "dataprime":
		if widget == "horizontal_bar_chart" {
			return dashboardOpenAPIHorizontalBarDataPrimeQuery(updated)
		}
		return dashboardOpenAPIDataPrimeQuery(updated)
	default:
		panic(fmt.Sprintf("unsupported structured dashboard query branch %q", queryBranch))
	}
}

func dashboardOpenAPIAssertStructuredQueryWidgets(dashboard *dashboardservice.Dashboard, queryBranch string, includeMarkdown bool, fixture string, updated bool) error {
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
		if err := dashboardOpenAPIAssertStructuredTypedQueryValue(queryCarrier, queryBranch, updated); err != nil {
			return fmt.Errorf("dashboard fixture %q (dashboard %q): widget %q: %w", fixture, dashboardID, spec.name, err)
		}
		if queryBranch == "dataprime" && spec.name == "horizontal_bar_chart" {
			query, ok := queryCarrier.(*dashboardservice.HorizontalBarChartQuery)
			if !ok || query.Dataprime == nil {
				return fmt.Errorf("dashboard fixture %q (dashboard %q): horizontal-bar Dataprime query has type %T", fixture, dashboardID, queryCarrier)
			}
			if err := dashboardOpenAPIAssertHorizontalBarDataPrime(query.Dataprime, updated); err != nil {
				return fmt.Errorf("dashboard fixture %q (dashboard %q): %w", fixture, dashboardID, err)
			}
		}
		if queryBranch == "dataprime" && index < 3 {
			filterBranch := []string{"logs", "spans", "metrics"}[index]
			filter, err := dashboardOpenAPIDataPrimeFilter(queryCarrier)
			if err != nil {
				return fmt.Errorf("dashboard fixture %q (dashboard %q): widget %q: %w", fixture, dashboardID, spec.name, err)
			}
			if err := dashboardOpenAPIAssertOneOfBranch(filter, "FilterSource", filterBranch, dashboardID, fixture); err != nil {
				return err
			}
			if err := dashboardOpenAPIAssertDataPrimeFilterValue(filter, filterBranch, updated); err != nil {
				return fmt.Errorf("dashboard fixture %q (dashboard %q): widget %q: %w", fixture, dashboardID, spec.name, err)
			}
		}
	}

	if includeMarkdown {
		definition := widgets[len(widgetSpecs)].GetDefinition()
		if err := dashboardOpenAPIAssertOneOfBranch(&definition, "WidgetDefinition", "markdown", dashboardID, fixture); err != nil {
			return err
		}
		if definition.Markdown == nil || definition.Markdown.GetMarkdownText() != dashboardOpenAPIMarkdownText(updated) {
			return fmt.Errorf("dashboard fixture %q (dashboard %q): markdown typed field did not round-trip", fixture, dashboardID)
		}
	}

	return nil
}

func dashboardOpenAPIAssertStructuredTypedQueryValue(queryCarrier any, queryBranch string, updated bool) error {
	typedQuery := dashboardOpenAPIStructuredTypedQuery(queryCarrier, queryBranch)
	switch queryBranch {
	case "logs":
		query, ok := typedQuery.(interface {
			GetLuceneQuery() dashboardservice.LuceneQuery
		})
		if !ok {
			return fmt.Errorf("REST logs typed Lucene query is absent")
		}
		luceneQuery := query.GetLuceneQuery()
		if luceneQuery.GetValue() != dashboardOpenAPILuceneQuery(updated) {
			return fmt.Errorf("REST logs Lucene query did not round-trip after updated=%t", updated)
		}
	case "spans":
		query, ok := typedQuery.(interface {
			GetLuceneQuery() dashboardservice.LuceneQuery
		})
		if !ok {
			return fmt.Errorf("REST spans typed Lucene query is absent")
		}
		luceneQuery := query.GetLuceneQuery()
		if luceneQuery.GetValue() != dashboardOpenAPISpansLuceneQuery(updated) {
			return fmt.Errorf("REST spans Lucene query did not round-trip after updated=%t", updated)
		}
	case "metrics":
		query, ok := typedQuery.(interface {
			GetPromqlQuery() dashboardservice.PromQlQuery
		})
		if !ok {
			return fmt.Errorf("REST metrics typed PromQL query is absent")
		}
		promQLQuery := query.GetPromqlQuery()
		if promQLQuery.GetValue() != dashboardOpenAPIPromQLQuery(updated) {
			return fmt.Errorf("REST metrics PromQL query did not round-trip after updated=%t", updated)
		}
	case "dataprime":
		query, ok := typedQuery.(interface {
			GetDataprimeQuery() dashboardservice.CommonDataprimeQuery
		})
		want := dashboardOpenAPIDataPrimeQuery(updated)
		if _, horizontal := typedQuery.(*dashboardservice.HorizontalBarChartDataprimeQuery); horizontal {
			want = dashboardOpenAPIHorizontalBarDataPrimeQuery(updated)
		}
		if !ok {
			return fmt.Errorf("REST typed Dataprime query is absent")
		}
		dataPrimeQuery := query.GetDataprimeQuery()
		if dataPrimeQuery.GetText() != want {
			return fmt.Errorf("REST Dataprime query did not round-trip after updated=%t", updated)
		}
	default:
		return fmt.Errorf("unsupported structured query branch %q", queryBranch)
	}
	return nil
}

func dashboardOpenAPIStructuredTypedQuery(queryCarrier any, queryBranch string) any {
	selectQuery := func(logs, metrics, spans, dataprime any) any {
		switch queryBranch {
		case "logs":
			return logs
		case "metrics":
			return metrics
		case "spans":
			return spans
		case "dataprime":
			return dataprime
		default:
			return nil
		}
	}
	switch query := queryCarrier.(type) {
	case *dashboardservice.LineChartQuery:
		return selectQuery(query.Logs, query.Metrics, query.Spans, query.Dataprime)
	case *dashboardservice.DataTableQuery:
		return selectQuery(query.Logs, query.Metrics, query.Spans, query.Dataprime)
	case *dashboardservice.GaugeQuery:
		return selectQuery(query.Logs, query.Metrics, query.Spans, query.Dataprime)
	case *dashboardservice.PieChartQuery:
		return selectQuery(query.Logs, query.Metrics, query.Spans, query.Dataprime)
	case *dashboardservice.BarChartQuery:
		return selectQuery(query.Logs, query.Metrics, query.Spans, query.Dataprime)
	case *dashboardservice.HorizontalBarChartQuery:
		return selectQuery(query.Logs, query.Metrics, query.Spans, query.Dataprime)
	case *dashboardservice.HexagonQuery:
		return selectQuery(query.Logs, query.Metrics, query.Spans, query.Dataprime)
	default:
		return nil
	}
}

func dashboardOpenAPIAssertHorizontalBarDataPrime(query *dashboardservice.HorizontalBarChartDataprimeQuery, updated bool) error {
	dataPrimeQuery, ok := query.GetDataprimeQueryOk()
	if !ok || dataPrimeQuery.GetText() != dashboardOpenAPIHorizontalBarDataPrimeQuery(updated) {
		return fmt.Errorf("horizontal-bar Dataprime query text did not round-trip")
	}
	if len(query.GetFilters()) != 0 {
		return fmt.Errorf("horizontal-bar Dataprime filters = %d, want omitted/empty", len(query.GetFilters()))
	}
	groupNames := query.GetGroupNames()
	if len(groupNames) != 1 || groupNames[0] != dashboardOpenAPIDataPrimeGroupName(updated) {
		return fmt.Errorf("horizontal-bar Dataprime group names = %v, want [%s]", groupNames, dashboardOpenAPIDataPrimeGroupName(updated))
	}
	if query.GetStackedGroupName() != dashboardOpenAPIDataPrimeStackedGroupName(updated) {
		return fmt.Errorf("horizontal-bar Dataprime stacked group name = %q, want %s", query.GetStackedGroupName(), dashboardOpenAPIDataPrimeStackedGroupName(updated))
	}
	if query.TimeFrame == nil {
		return fmt.Errorf("horizontal-bar Dataprime time frame is nil")
	}
	if !updated {
		if query.TimeFrame.GetRelativeTimeFrame() != "900s" || query.TimeFrame.AbsoluteTimeFrame != nil {
			return fmt.Errorf("horizontal-bar Dataprime initial time frame = %#v, want relative 900s", query.TimeFrame)
		}
		return nil
	}

	wantStart := time.Date(2026, time.February, 1, 0, 0, 0, 0, time.UTC)
	wantEnd := time.Date(2026, time.February, 1, 0, 15, 0, 0, time.UTC)
	if query.TimeFrame.RelativeTimeFrame != nil || query.TimeFrame.AbsoluteTimeFrame == nil ||
		query.TimeFrame.AbsoluteTimeFrame.GetFrom() != wantStart || query.TimeFrame.AbsoluteTimeFrame.GetTo() != wantEnd {
		return fmt.Errorf("horizontal-bar Dataprime updated time frame = %#v, want absolute %s to %s", query.TimeFrame, wantStart, wantEnd)
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
	return dashboardOpenAPIStructuredDashboardConfigVariant(name, queryBranch, includeMarkdown, false)
}

func dashboardOpenAPIStructuredDashboardUpdateConfig(name, queryBranch string, includeMarkdown bool) string {
	return dashboardOpenAPIStructuredDashboardConfigVariant(name, queryBranch, includeMarkdown, true)
}

func dashboardOpenAPIStructuredDashboardConfigVariant(name, queryBranch string, includeMarkdown, updated bool) string {
	widgets := ""
	widgetSpecs := dashboardOpenAPIStructuredWidgetsForBranch(queryBranch)
	for index, widget := range widgetSpecs {
		if index > 0 {
			widgets += ",\n"
		}
		widgets += dashboardOpenAPIStructuredWidgetConfig(widget.name, queryBranch, updated)
	}
	if includeMarkdown {
		widgets += fmt.Sprintf(`,
        {
          definition = {
            markdown = {
              markdown_text = %q
            }
          }
        }`, dashboardOpenAPIMarkdownText(updated))
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

func dashboardOpenAPIStructuredWidgetConfig(widget, queryBranch string, updated bool) string {
	query := dashboardOpenAPIStructuredQueryConfigVariant(widget, queryBranch, updated)
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
	return dashboardOpenAPIStructuredQueryConfigVariant(widget, queryBranch, false)
}

func dashboardOpenAPIStructuredQueryConfigVariant(widget, queryBranch string, updated bool) string {
	const indent = "                "
	switch queryBranch {
	case "logs":
		luceneQuery := dashboardOpenAPILuceneQuery(updated)
		aggregation := `{ type = "count" }`
		switch widget {
		case "line_chart":
			return fmt.Sprintf(`%slogs = {
                  lucene_query = %q
                  aggregations = [%s]
                }`, indent, luceneQuery, aggregation)
		case "data_table":
			return fmt.Sprintf(`%slogs = {
                  lucene_query = %q
                }`, indent, luceneQuery)
		case "gauge":
			return fmt.Sprintf(`%slogs = {
                  lucene_query = %q
                  logs_aggregation = %s
                }`, indent, luceneQuery, aggregation)
		default:
			return fmt.Sprintf(`%slogs = {
                  lucene_query = %q
                  aggregation = %s
                }`, indent, luceneQuery, aggregation)
		}
	case "metrics":
		if widget == "pie_chart" {
			return fmt.Sprintf(`%smetrics = {
                  promql_query = %q
                  group_names  = [%q]
                }`, indent, dashboardOpenAPIPromQLQuery(updated), dashboardOpenAPIMetricsGroupName(updated))
		}
		return fmt.Sprintf(`%smetrics = {
                  promql_query = %q
                }`, indent, dashboardOpenAPIPromQLQuery(updated))
	case "spans":
		aggregation := `{
                    type             = "dimension"
					aggregation_type = "unique_count"
					field            = "trace_id"
				  }`
		luceneQuery := dashboardOpenAPISpansLuceneQuery(updated)
		switch widget {
		case "line_chart":
			return fmt.Sprintf("%sspans = {\n                  lucene_query = %q\n                  aggregations = [%s]\n                }", indent, luceneQuery, aggregation)
		case "data_table":
			return fmt.Sprintf("%sspans = {\n                  lucene_query = %q\n                  grouping = {\n                    aggregations = [{ aggregation = %s }]\n                  }\n                }", indent, luceneQuery, aggregation)
		case "gauge":
			return fmt.Sprintf("%sspans = {\n                  lucene_query = %q\n                  spans_aggregation = %s\n                }", indent, luceneQuery, aggregation)
		default:
			return fmt.Sprintf("%sspans = {\n                  lucene_query = %q\n                  aggregation = %s\n                }", indent, luceneQuery, aggregation)
		}
	case "dataprime":
		filter := dashboardOpenAPIDataPrimeFilterConfig(widget, updated)
		groupNames := ""
		if widget == "pie_chart" {
			groupNames = fmt.Sprintf("\n                  group_names = [%q]", dashboardOpenAPIDataPrimePieGroupName(updated))
		}
		if widget == "horizontal_bar_chart" {
			timeFrame := `relative = { duration = "seconds:900" }`
			if updated {
				timeFrame = `absolute = {
                      start = "2026-02-01T00:00:00Z"
                      end   = "2026-02-01T00:15:00Z"
                    }`
			}
			return fmt.Sprintf(`%sdata_prime = {
                  query              = %q
				  group_names        = [%q]
				  stacked_group_name = %q
                  time_frame = {
                    %s
                  }
				}`, indent, dashboardOpenAPIHorizontalBarDataPrimeQuery(updated), dashboardOpenAPIDataPrimeGroupName(updated), dashboardOpenAPIDataPrimeStackedGroupName(updated), timeFrame)
		}
		return fmt.Sprintf("%sdata_prime = {\n                  query = %q%s%s\n                }", indent, dashboardOpenAPIDataPrimeQuery(updated), groupNames, filter)
	default:
		panic(fmt.Sprintf("unsupported structured dashboard query branch %q", queryBranch))
	}
}

func dashboardOpenAPIDataPrimeFilterConfig(widget string, updated bool) string {
	switch widget {
	case "line_chart":
		return fmt.Sprintf(`
                  filters = [{
                    logs = {
					  field    = %q
					  operator = { type = "equals", selected_values = [%q] }
                    }
				  }]`, dashboardOpenAPIDataPrimeFilterStateValue("logs", updated), dashboardOpenAPIDataPrimeFilterSelection(updated))
	case "data_table":
		return fmt.Sprintf(`
                  filters = [{
                    spans = {
					  field    = { type = "metadata", value = %q }
					  operator = { type = "equals", selected_values = [%q] }
                    }
				  }]`, dashboardOpenAPIDataPrimeSpansFilterField(updated), dashboardOpenAPIDataPrimeFilterSelection(updated))
	case "gauge":
		return fmt.Sprintf(`
                  filters = [{
                    metrics = {
					  metric_name = %q
					  label       = %q
					  operator    = { type = "equals", selected_values = [%q] }
                    }
				  }]`, dashboardOpenAPIDataPrimeFilterStateValue("metrics", updated), dashboardOpenAPIDataPrimeMetricsFilterLabel(updated), dashboardOpenAPIDataPrimeFilterSelection(updated))
	default:
		return ""
	}
}

func dashboardOpenAPIDataPrimeFilterStatePath(widget, branch string) string {
	queryPath := dashboardOpenAPIDataPrimeFiltersStatePath(widget) + ".0."
	switch branch {
	case "logs":
		return queryPath + "logs.field"
	case "spans":
		return queryPath + "spans.field.type"
	case "metrics":
		return queryPath + "metrics.metric_name"
	default:
		panic(fmt.Sprintf("unsupported Dataprime filter branch %q", branch))
	}
}

func dashboardOpenAPIDataPrimeFiltersStatePath(widget string) string {
	if widget == "line_chart" {
		return ".definition.line_chart.query_definitions.0.query.data_prime.filters"
	}
	return ".definition." + widget + ".query.data_prime.filters"
}

func dashboardOpenAPIDataPrimeFilterStateValue(branch string, updated bool) string {
	switch branch {
	case "logs":
		if updated {
			return "coralogix.metadata.subsystemName"
		}
		return "coralogix.metadata.applicationName"
	case "spans":
		return "metadata"
	case "metrics":
		if updated {
			return "http_server_requests_total"
		}
		return "http_requests_total"
	default:
		panic(fmt.Sprintf("unsupported Dataprime filter branch %q", branch))
	}
}

func dashboardOpenAPIDataPrimeFilter(queryCarrier any) (*dashboardservice.FilterSource, error) {
	var filters []dashboardservice.FilterSource
	switch query := queryCarrier.(type) {
	case *dashboardservice.LineChartQuery:
		if query.Dataprime != nil {
			filters = query.Dataprime.Filters
		}
	case *dashboardservice.DataTableQuery:
		if query.Dataprime != nil {
			filters = query.Dataprime.Filters
		}
	case *dashboardservice.GaugeQuery:
		if query.Dataprime != nil {
			filters = query.Dataprime.Filters
		}
	default:
		return nil, fmt.Errorf("unsupported Dataprime query carrier %T", queryCarrier)
	}
	if len(filters) != 1 {
		return nil, fmt.Errorf("REST Dataprime filters = %d, want 1", len(filters))
	}
	return &filters[0], nil
}

func dashboardOpenAPIAssertDataPrimeFilterValue(filter *dashboardservice.FilterSource, branch string, updated bool) error {
	switch branch {
	case "logs":
		if filter.Logs == nil || filter.Logs.GetField() != dashboardOpenAPIDataPrimeFilterStateValue(branch, updated) {
			return fmt.Errorf("REST logs filter field did not round-trip")
		}
	case "spans":
		wantField := dashboardservice.METADATAFIELD_METADATA_FIELD_SERVICE_NAME
		if updated {
			wantField = dashboardservice.METADATAFIELD_METADATA_FIELD_SUBSYSTEM_NAME
		}
		if filter.Spans == nil || filter.Spans.Field == nil || filter.Spans.Field.MetadataField == nil || *filter.Spans.Field.MetadataField != wantField {
			return fmt.Errorf("REST spans filter field did not round-trip")
		}
	case "metrics":
		if filter.Metrics == nil || filter.Metrics.GetMetric() != dashboardOpenAPIDataPrimeFilterStateValue(branch, updated) ||
			filter.Metrics.GetLabel() != dashboardOpenAPIDataPrimeMetricsFilterLabel(updated) {
			return fmt.Errorf("REST metrics filter target did not round-trip")
		}
	default:
		return fmt.Errorf("unsupported Dataprime filter branch %q", branch)
	}
	operator := dashboardOpenAPIDataPrimeFilterOperator(filter)
	if operator == nil || operator.Equals == nil || operator.Equals.Selection == nil || operator.Equals.Selection.List == nil {
		return fmt.Errorf("REST %s filter list selection is absent", branch)
	}
	selectedValues := operator.Equals.Selection.List.GetValues()
	if len(selectedValues) != 1 || selectedValues[0] != dashboardOpenAPIDataPrimeFilterSelection(updated) {
		return fmt.Errorf("REST %s filter selection did not round-trip", branch)
	}
	return nil
}

func dashboardOpenAPIDataPrimeFilterOperator(filter *dashboardservice.FilterSource) *dashboardservice.FilterOperator {
	switch {
	case filter == nil:
		return nil
	case filter.Logs != nil:
		return filter.Logs.Operator
	case filter.Spans != nil:
		return filter.Spans.Operator
	case filter.Metrics != nil:
		return filter.Metrics.Operator
	default:
		return nil
	}
}

func dashboardOpenAPIDataPrimeQuery(updated bool) string {
	if updated {
		return "source logs\n| filter $m.severity == 'WARNING'\n| aggregate count() as updated_count\n| choose updated_count"
	}
	return "source logs\n| filter 1 == 1\n| aggregate count() as c\n| choose c"
}

func dashboardOpenAPIHorizontalBarDataPrimeQuery(updated bool) string {
	if updated {
		return "source logs\n| filter $m.severity == 'WARNING'\n| groupby $l.subsystemname as subsystem, $m.severity as priority aggregate count() as c"
	}
	return "source logs\n| filter 1 == 1\n| groupby $l.applicationname as application, $m.severity as severity aggregate count() as c"
}

func dashboardOpenAPILuceneQuery(updated bool) string {
	if updated {
		return "coralogix.metadata.severity:WARNING"
	}
	return "coralogix.metadata.severity:ERROR"
}

func dashboardOpenAPISpansLuceneQuery(updated bool) string {
	if updated {
		return "serviceName:api"
	}
	return "*"
}

func dashboardOpenAPIPromQLQuery(updated bool) string {
	if updated {
		return "vector(2)"
	}
	return "vector(1)"
}

func dashboardOpenAPIMarkdownText(updated bool) string {
	if updated {
		return "## Updated structured dashboard coverage"
	}
	return "## Structured dashboard coverage"
}

func dashboardOpenAPIMetricsGroupName(updated bool) string {
	if updated {
		return "service"
	}
	return "job"
}

func dashboardOpenAPIDataPrimePieGroupName(updated bool) string {
	if updated {
		return "updated_count"
	}
	return "c"
}

func dashboardOpenAPIDataPrimeGroupName(updated bool) string {
	if updated {
		return "subsystem"
	}
	return "application"
}

func dashboardOpenAPIDataPrimeStackedGroupName(updated bool) string {
	if updated {
		return "priority"
	}
	return "severity"
}

func dashboardOpenAPIDataPrimeSpansFilterField(updated bool) string {
	if updated {
		return "subsystem_name"
	}
	return "service_name"
}

func dashboardOpenAPIDataPrimeMetricsFilterLabel(updated bool) string {
	if updated {
		return "job"
	}
	return "service"
}

func dashboardOpenAPIDataPrimeFilterSelection(updated bool) string {
	if updated {
		return "worker"
	}
	return "api"
}

func dashboardOpenAPIStructuredWidgetsForBranch(queryBranch string) []dashboardStructuredWidgetSpec {
	switch queryBranch {
	case "logs", "metrics", "spans", "dataprime":
		return dashboardStructuredQueryWidgets
	default:
		panic(fmt.Sprintf("unsupported structured dashboard query branch %q", queryBranch))
	}
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
