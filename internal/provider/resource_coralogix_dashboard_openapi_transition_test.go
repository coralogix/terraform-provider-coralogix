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
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

const dashboardOpenAPITransitionTestName = "TestAccCoralogixResourceDashboardOpenAPIOneOfTransitions"

type dashboardOpenAPITransition struct {
	definitionBranch   string
	queryBranch        string
	timeFrame          string
	annotationSource   string
	annotationStrategy string
	variableSource     string
	selectionType      string
	selectionBranch    string
	filterBranch       string
}

var dashboardOpenAPITransitions = []dashboardOpenAPITransition{
	{
		definitionBranch: "lineChart",
		queryBranch:      "metrics",
		timeFrame:        "relative",
		annotationSource: "metrics",
		variableSource:   "logsPath",
		selectionType:    "single",
		selectionBranch:  "list",
		filterBranch:     "metrics",
	},
	{
		definitionBranch:   "pieChart",
		queryBranch:        "logs",
		timeFrame:          "absolute",
		annotationSource:   "logs",
		annotationStrategy: "range",
		variableSource:     "metricLabel",
		selectionType:      "multi",
		selectionBranch:    "list",
		filterBranch:       "spans",
	},
	{
		definitionBranch:   "markdown",
		queryBranch:        "spans",
		annotationSource:   "spans",
		annotationStrategy: "duration",
		variableSource:     "constantList",
		selectionBranch:    "all",
	},
	{
		definitionBranch:   "markdown",
		queryBranch:        "dataprime",
		annotationSource:   "manual",
		annotationStrategy: "range",
		variableSource:     "query",
		selectionBranch:    "all",
	},
}

func TestDashboardOpenAPITransitionConfigsParse(t *testing.T) {
	for index, transition := range dashboardOpenAPITransitions {
		config := dashboardOpenAPITransitionConfig("dashboard", transition)
		_, diagnostics := hclsyntax.ParseConfig([]byte(config), fmt.Sprintf("transition-%d.tf", index), hcl.InitialPos)
		if diagnostics.HasErrors() {
			t.Errorf("transition %d config is invalid HCL:\n%s", index, diagnostics.Error())
		}
	}
}

func TestAccCoralogixResourceDashboardOpenAPIOneOfTransitions(t *testing.T) {
	ctx := context.Background()
	var client *dashboardservice.DashboardServiceAPIService
	fixture := dashboardOpenAPITransitionTestName
	name := dashboardOpenAPIFixtureName(fixture)
	dashboardIdentity := newDashboardOpenAPIIDTracker(dashboardResourceName, fixture)
	nestedIdentity := newDashboardOpenAPINestedIDTracker(fixture)
	checkTransition := func(transition dashboardOpenAPITransition, identityCheck resource.TestCheckFunc) resource.TestCheckFunc {
		return resource.ComposeAggregateTestCheckFunc(
			identityCheck,
			dashboardOpenAPITransitionStateCheck(transition),
			func(state *terraform.State) error {
				dashboard, err := dashboardOpenAPIFetchDashboard(ctx, client, state, dashboardResourceName, fixture)
				if err != nil {
					return err
				}
				if err := nestedIdentity.CaptureOrAssert(dashboard); err != nil {
					return err
				}
				return dashboardOpenAPIAssertTransition(dashboard, transition, fixture)
			},
		)
	}
	updates := make([]dashboardOpenAPILifecyclePhase, 0, len(dashboardOpenAPITransitions)-1)
	for _, transition := range dashboardOpenAPITransitions[1:] {
		updates = append(updates, dashboardOpenAPILifecyclePhase{
			Config: dashboardOpenAPITransitionConfig(name, transition),
			Check:  checkTransition(transition, dashboardIdentity.AssertUnchanged()),
		})
	}
	lastTransition := dashboardOpenAPITransitions[len(dashboardOpenAPITransitions)-1]
	steps := dashboardOpenAPIStructuredLifecycleSteps(
		dashboardOpenAPILifecyclePhase{
			Config: dashboardOpenAPITransitionConfig(name, dashboardOpenAPITransitions[0]),
			Check:  checkTransition(dashboardOpenAPITransitions[0], dashboardIdentity.Capture()),
		},
		updates,
		resource.TestStep{
			ResourceName:      dashboardResourceName,
			ImportState:       true,
			ImportStateVerify: true,
			ImportStateCheck: dashboardOpenAPIImportDashboardCheck(ctx, &client, fixture, func(dashboard *dashboardservice.Dashboard) error {
				if err := nestedIdentity.CaptureOrAssert(dashboard); err != nil {
					return err
				}
				return dashboardOpenAPIAssertTransition(dashboard, lastTransition, fixture)
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

func dashboardOpenAPITransitionStateCheck(transition dashboardOpenAPITransition) resource.TestCheckFunc {
	checks := []resource.TestCheckFunc{
		resource.TestCheckResourceAttrSet(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.id"),
		resource.TestCheckResourceAttrSet(dashboardResourceName, "layout.sections.0.rows.0.widgets.1.id"),
		resource.TestCheckResourceAttr(dashboardResourceName, "variables.0.definition.multi_select.values_order_direction", "asc"),
		resource.TestCheckResourceAttr(dashboardResourceName, "annotations.#", "1"),
		resource.TestCheckNoResourceAttr(dashboardResourceName, "auto_refresh"),
	}

	switch transition.definitionBranch {
	case "lineChart":
		checks = append(checks, resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.line_chart.query_definitions.0.query.metrics.promql_query", "vector(10)"))
	case "pieChart":
		checks = append(checks, resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.pie_chart.query.metrics.promql_query", "vector(20)"))
	case "markdown":
		checks = append(checks, resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.markdown.markdown_text", "## Transition complete"))
	}

	queryPath := "layout.sections.0.rows.0.widgets.1" + dashboardOpenAPIQueryStatePath("gauge", transition.queryBranch)
	checks = append(checks, resource.TestCheckResourceAttr(dashboardResourceName, queryPath, dashboardOpenAPIQueryStateValue("gauge", transition.queryBranch, false)))

	switch transition.timeFrame {
	case "relative":
		checks = append(checks, resource.TestCheckResourceAttr(dashboardResourceName, "time_frame.relative.duration", "seconds:900"))
	case "absolute":
		checks = append(checks, resource.TestCheckResourceAttr(dashboardResourceName, "time_frame.absolute.start", "2026-03-01T00:00:00Z"))
	default:
		checks = append(checks, resource.TestCheckNoResourceAttr(dashboardResourceName, "time_frame"))
	}

	if transition.selectionType == "" {
		checks = append(checks, resource.TestCheckNoResourceAttr(dashboardResourceName, "variables.0.definition.multi_select.selection_type"))
	} else {
		checks = append(checks, resource.TestCheckResourceAttr(dashboardResourceName, "variables.0.definition.multi_select.selection_type", transition.selectionType))
	}
	if transition.selectionBranch == "all" {
		checks = append(checks, resource.TestCheckResourceAttr(dashboardResourceName, "variables.0.definition.multi_select.selected_values.#", "0"))
	}
	if transition.filterBranch == "" {
		checks = append(checks, resource.TestCheckNoResourceAttr(dashboardResourceName, "filters"))
	} else {
		checks = append(checks, resource.TestCheckResourceAttr(dashboardResourceName, "filters.#", "1"))
	}

	return resource.ComposeAggregateTestCheckFunc(checks...)
}

func dashboardOpenAPIAssertTransition(dashboard *dashboardservice.Dashboard, transition dashboardOpenAPITransition, fixture string) error {
	if dashboard == nil {
		return fmt.Errorf("dashboard fixture %q: fetched dashboard is nil", fixture)
	}
	widgets, err := dashboardOpenAPIFirstRowWidgets(dashboard)
	if err != nil {
		return fmt.Errorf("dashboard fixture %q: %w", fixture, err)
	}
	if len(widgets) != 2 {
		return fmt.Errorf("dashboard fixture %q: REST widgets = %d, want 2", fixture, len(widgets))
	}

	definition := widgets[0].GetDefinition()
	if err := dashboardOpenAPIAssertOneOfBranch(&definition, "WidgetDefinition", transition.definitionBranch, dashboard.GetId(), fixture); err != nil {
		return err
	}
	if err := dashboardOpenAPIAssertTransitionDefinition(&definition, transition.definitionBranch); err != nil {
		return fmt.Errorf("dashboard fixture %q: %w", fixture, err)
	}

	gaugeDefinition := widgets[1].GetDefinition()
	if err := dashboardOpenAPIAssertOneOfBranch(&gaugeDefinition, "WidgetDefinition", "gauge", dashboard.GetId(), fixture); err != nil {
		return err
	}
	query, err := dashboardOpenAPIStructuredQueryCarrier(&gaugeDefinition, "gauge")
	if err != nil {
		return fmt.Errorf("dashboard fixture %q: %w", fixture, err)
	}
	if err := dashboardOpenAPIAssertOneOfBranch(query, "GaugeQuery", transition.queryBranch, dashboard.GetId(), fixture); err != nil {
		return err
	}
	gaugeQuery, ok := query.(*dashboardservice.GaugeQuery)
	if !ok {
		return fmt.Errorf("dashboard fixture %q: gauge query carrier has type %T", fixture, query)
	}
	if err := dashboardOpenAPIAssertTransitionGaugeQuery(gaugeQuery, transition.queryBranch, dashboard.GetId(), fixture); err != nil {
		return err
	}

	if err := dashboardOpenAPIAssertTransitionTimeFrame(dashboard, transition.timeFrame, fixture); err != nil {
		return err
	}
	if err := dashboardOpenAPIAssertTransitionAnnotation(dashboard, transition, fixture); err != nil {
		return err
	}
	if err := dashboardOpenAPIAssertTransitionVariable(dashboard, transition, fixture); err != nil {
		return err
	}
	return dashboardOpenAPIAssertTransitionFilters(dashboard, transition.filterBranch, fixture)
}

func dashboardOpenAPIAssertTransitionGaugeQuery(query *dashboardservice.GaugeQuery, branch, dashboardID, fixture string) error {
	switch branch {
	case "metrics":
		if query.Metrics == nil || query.Metrics.PromqlQuery.GetValue() != "vector(1)" {
			return fmt.Errorf("dashboard fixture %q: REST gauge metrics typed field did not round-trip", fixture)
		}
	case "logs":
		if query.Logs == nil || query.Logs.LogsAggregation == nil {
			return fmt.Errorf("dashboard fixture %q: REST gauge logs typed field did not round-trip", fixture)
		}
		return dashboardOpenAPIAssertOneOfBranch(query.Logs.LogsAggregation, "LogsAggregation", "count", dashboardID, fixture)
	case "spans":
		if query.Spans == nil || query.Spans.SpansAggregation == nil {
			return fmt.Errorf("dashboard fixture %q: REST gauge spans typed field did not round-trip", fixture)
		}
		return dashboardOpenAPIAssertOneOfBranch(query.Spans.SpansAggregation, "SpansAggregation", "dimensionAggregation", dashboardID, fixture)
	case "dataprime":
		if query.Dataprime == nil || query.Dataprime.DataprimeQuery == nil || query.Dataprime.DataprimeQuery.GetText() != dashboardOpenAPIDataPrimeQuery(false) {
			return fmt.Errorf("dashboard fixture %q: REST gauge Dataprime typed field did not round-trip", fixture)
		}
		filter, err := dashboardOpenAPIDataPrimeFilter(query)
		if err != nil {
			return fmt.Errorf("dashboard fixture %q: %w", fixture, err)
		}
		return dashboardOpenAPIAssertOneOfBranch(filter, "FilterSource", "metrics", dashboardID, fixture)
	default:
		return fmt.Errorf("dashboard fixture %q: unsupported gauge query branch %q", fixture, branch)
	}
	return nil
}

func dashboardOpenAPIAssertTransitionDefinition(definition *dashboardservice.WidgetDefinition, branch string) error {
	switch branch {
	case "lineChart":
		if definition.LineChart == nil || len(definition.LineChart.QueryDefinitions) != 1 || definition.LineChart.QueryDefinitions[0].Query.Metrics == nil || definition.LineChart.QueryDefinitions[0].Query.Metrics.PromqlQuery.GetValue() != "vector(10)" {
			return fmt.Errorf("REST lineChart typed field did not round-trip")
		}
	case "pieChart":
		if definition.PieChart == nil || definition.PieChart.Query == nil || definition.PieChart.Query.Metrics == nil || definition.PieChart.Query.Metrics.PromqlQuery.GetValue() != "vector(20)" {
			return fmt.Errorf("REST pieChart typed field did not round-trip")
		}
	case "markdown":
		if definition.Markdown == nil || definition.Markdown.GetMarkdownText() != "## Transition complete" {
			return fmt.Errorf("REST markdown typed field did not round-trip")
		}
	default:
		return fmt.Errorf("unsupported transition definition branch %q", branch)
	}
	return nil
}

func dashboardOpenAPIAssertTransitionTimeFrame(dashboard *dashboardservice.Dashboard, branch, fixture string) error {
	switch branch {
	case "relative":
		if err := dashboardOpenAPIAssertOneOfBranch(dashboard, "Dashboard", "relativeTimeFrame", dashboard.GetId(), fixture); err != nil {
			return err
		}
		if dashboard.GetRelativeTimeFrame() != "900s" {
			return fmt.Errorf("dashboard fixture %q: REST relativeTimeFrame = %q, want 900s", fixture, dashboard.GetRelativeTimeFrame())
		}
	case "absolute":
		if err := dashboardOpenAPIAssertOneOfBranch(dashboard, "Dashboard", "absoluteTimeFrame", dashboard.GetId(), fixture); err != nil {
			return err
		}
		if dashboard.AbsoluteTimeFrame == nil || dashboard.AbsoluteTimeFrame.GetFrom().Format("2006-01-02T15:04:05Z07:00") != "2026-03-01T00:00:00Z" {
			return fmt.Errorf("dashboard fixture %q: REST absoluteTimeFrame did not round-trip", fixture)
		}
	case "":
		if dashboard.RelativeTimeFrame != nil || dashboard.AbsoluteTimeFrame != nil ||
			dashboard.Off != nil || dashboard.OneMinute != nil || dashboard.TwoMinutes != nil ||
			dashboard.FiveMinutes != nil || dashboard.FifteenMinutes != nil {
			return fmt.Errorf(
				"dashboard fixture %q: optional dashboard oneOf remains populated: relative=%v absolute=%v off=%v oneMinute=%v twoMinutes=%v fiveMinutes=%v fifteenMinutes=%v",
				fixture,
				dashboard.RelativeTimeFrame != nil,
				dashboard.AbsoluteTimeFrame != nil,
				dashboard.Off != nil,
				dashboard.OneMinute != nil,
				dashboard.TwoMinutes != nil,
				dashboard.FiveMinutes != nil,
				dashboard.FifteenMinutes != nil,
			)
		}
	default:
		return fmt.Errorf("dashboard fixture %q: unsupported time frame transition %q", fixture, branch)
	}
	return nil
}

func dashboardOpenAPIAssertTransitionAnnotation(dashboard *dashboardservice.Dashboard, transition dashboardOpenAPITransition, fixture string) error {
	annotations := dashboard.GetAnnotations()
	if len(annotations) != 1 || annotations[0].Source == nil {
		return fmt.Errorf("dashboard fixture %q: REST annotations = %d or source is nil, want one typed annotation", fixture, len(annotations))
	}
	source := annotations[0].Source
	if err := dashboardOpenAPIAssertOneOfBranch(source, "AnnotationSource", transition.annotationSource, dashboard.GetId(), fixture); err != nil {
		return err
	}
	switch transition.annotationSource {
	case "metrics":
		if source.Metrics == nil || source.Metrics.Strategy == nil || source.Metrics.Strategy.StartTimeMetric == nil || source.Metrics.PromqlQuery.GetValue() != "vector(1)" {
			return fmt.Errorf("dashboard fixture %q: REST metrics annotation typed field did not round-trip", fixture)
		}
	case "logs":
		if source.Logs == nil || source.Logs.Strategy == nil {
			return fmt.Errorf("dashboard fixture %q: REST logs annotation typed field is nil", fixture)
		}
		return dashboardOpenAPIAssertOneOfBranch(source.Logs.Strategy, "LogsSourceStrategy", transition.annotationStrategy, dashboard.GetId(), fixture)
	case "spans":
		if source.Spans == nil || source.Spans.Strategy == nil {
			return fmt.Errorf("dashboard fixture %q: REST spans annotation typed field is nil", fixture)
		}
		return dashboardOpenAPIAssertOneOfBranch(source.Spans.Strategy, "SpansSourceStrategy", transition.annotationStrategy, dashboard.GetId(), fixture)
	case "manual":
		if source.Manual == nil || source.Manual.Strategy == nil {
			return fmt.Errorf("dashboard fixture %q: REST manual annotation typed field is nil", fixture)
		}
		return dashboardOpenAPIAssertOneOfBranch(source.Manual.Strategy, "ManualSourceStrategy", transition.annotationStrategy, dashboard.GetId(), fixture)
	default:
		return fmt.Errorf("dashboard fixture %q: unsupported annotation source %q", fixture, transition.annotationSource)
	}
	return nil
}

func dashboardOpenAPIAssertTransitionVariable(dashboard *dashboardservice.Dashboard, transition dashboardOpenAPITransition, fixture string) error {
	variables := dashboard.GetVariables()
	if len(variables) != 1 || variables[0].Definition == nil || variables[0].Definition.MultiSelect == nil {
		return fmt.Errorf("dashboard fixture %q: REST variables = %d or multiSelect is nil, want one multiSelect variable", fixture, len(variables))
	}
	multiSelect := variables[0].Definition.MultiSelect
	if err := dashboardOpenAPIAssertOneOfBranch(variables[0].Definition, "VariableDefinition", "multiSelect", dashboard.GetId(), fixture); err != nil {
		return err
	}
	if err := dashboardOpenAPIAssertOneOfBranch(multiSelect.Source, "MultiSelectSource", transition.variableSource, dashboard.GetId(), fixture); err != nil {
		return err
	}
	if err := dashboardOpenAPIAssertOneOfBranch(multiSelect.Selection, "MultiSelectSelection", transition.selectionBranch, dashboard.GetId(), fixture); err != nil {
		return err
	}

	if transition.selectionType == "" {
		if multiSelect.SelectionOptions != nil && multiSelect.SelectionOptions.SelectionType != nil {
			return fmt.Errorf("dashboard fixture %q: removed REST variable selectionType remains %q", fixture, *multiSelect.SelectionOptions.SelectionType)
		}
	} else {
		want := dashboardservice.SELECTIONTYPE_SELECTION_TYPE_MULTI
		if transition.selectionType == "single" {
			want = dashboardservice.SELECTIONTYPE_SELECTION_TYPE_SINGLE
		}
		if multiSelect.SelectionOptions == nil || multiSelect.SelectionOptions.SelectionType == nil || *multiSelect.SelectionOptions.SelectionType != want {
			return fmt.Errorf("dashboard fixture %q: REST variable selectionType did not round-trip as %q", fixture, transition.selectionType)
		}
	}

	if transition.variableSource == "query" {
		if multiSelect.Source.Query == nil || multiSelect.Source.Query.Query == nil {
			return fmt.Errorf("dashboard fixture %q: REST variable query source is nil", fixture)
		}
		if err := dashboardOpenAPIAssertOneOfBranch(multiSelect.Source.Query.Query, "MultiSelectQuery", "spansQuery", dashboard.GetId(), fixture); err != nil {
			return err
		}
		return dashboardOpenAPIAssertOneOfBranch(multiSelect.Source.Query.Query.SpansQuery.Type, "QuerySpansQueryType", "fieldName", dashboard.GetId(), fixture)
	}
	return nil
}

func dashboardOpenAPIAssertTransitionFilters(dashboard *dashboardservice.Dashboard, branch, fixture string) error {
	if branch == "" {
		if len(dashboard.Filters) != 0 {
			return fmt.Errorf("dashboard fixture %q: removed REST filters retain %d entries", fixture, len(dashboard.Filters))
		}
		return nil
	}
	if len(dashboard.Filters) != 1 || dashboard.Filters[0].Source == nil {
		return fmt.Errorf("dashboard fixture %q: REST filters = %d or source is nil, want one typed filter", fixture, len(dashboard.Filters))
	}
	return dashboardOpenAPIAssertOneOfBranch(dashboard.Filters[0].Source, "FilterSource", branch, dashboard.GetId(), fixture)
}

func dashboardOpenAPITransitionConfig(name string, transition dashboardOpenAPITransition) string {
	return fmt.Sprintf(`
resource "coralogix_dashboard" "test" {
  name        = %q
  description = "REST oneOf transition coverage"
%s
  layout = {
    sections = [{
      rows = [{
        height = 12
        widgets = [
%s,
          {
            title = "query-transition"
            definition = { gauge = {
              query = {
%s
              }
              unit = "none"
            } }
          },
        ]
      }]
    }]
  }
  variables = [%s]
  annotations = [%s]
%s
}
`, name, dashboardOpenAPITransitionTimeFrameConfig(transition.timeFrame), dashboardOpenAPITransitionDefinitionConfig(transition.definitionBranch), dashboardOpenAPIStructuredQueryConfig("gauge", transition.queryBranch), dashboardOpenAPITransitionVariableConfig(transition), dashboardOpenAPITransitionAnnotationConfig(transition), dashboardOpenAPITransitionFilterConfig(transition.filterBranch))
}

func dashboardOpenAPITransitionTimeFrameConfig(branch string) string {
	switch branch {
	case "relative":
		return `  time_frame = { relative = { duration = "seconds:900" } }`
	case "absolute":
		return `  time_frame = { absolute = { start = "2026-03-01T00:00:00Z", end = "2026-03-01T01:00:00Z" } }`
	default:
		return ""
	}
}

func dashboardOpenAPITransitionDefinitionConfig(branch string) string {
	switch branch {
	case "lineChart":
		return `          {
            title = "definition-transition"
            definition = { line_chart = {
              query_definitions = [{ query = { metrics = { promql_query = "vector(10)" } } }]
            } }
          }`
	case "pieChart":
		return `          {
            title = "definition-transition"
            definition = { pie_chart = {
              query            = { metrics = { promql_query = "vector(20)", group_names = ["job"] } }
              label_definition = {}
            } }
          }`
	case "markdown":
		return `          {
            definition = { markdown = { markdown_text = "## Transition complete" } }
          }`
	default:
		panic(fmt.Sprintf("unsupported transition definition branch %q", branch))
	}
}

func dashboardOpenAPITransitionVariableConfig(transition dashboardOpenAPITransition) string {
	selected := ""
	if transition.selectionBranch == "list" {
		selected = `selected_values = ["api"]`
	}
	selectionType := ""
	if transition.selectionType != "" {
		selectionType = fmt.Sprintf("selection_type = %q", transition.selectionType)
	}

	var source string
	switch transition.variableSource {
	case "logsPath":
		source = `logs_path = "coralogix.metadata.applicationName"`
	case "metricLabel":
		source = `metric_label = { metric_name = "http_requests_total", label = "service" }`
	case "constantList":
		source = `constant_list = ["api", "worker"]`
	case "query":
		source = `query = { query = { spans = { field_name = { span_regex = ".*" } } } }`
	default:
		panic(fmt.Sprintf("unsupported transition variable source %q", transition.variableSource))
	}

	return fmt.Sprintf(`{
    name = "transition-variable", display_name = "Transition variable"
    definition = { multi_select = {
      values_order_direction = "asc"
      %s
      %s
      source = { %s }
    } }
  }`, selected, selectionType, source)
}

func dashboardOpenAPITransitionAnnotationConfig(transition dashboardOpenAPITransition) string {
	var source string
	switch transition.annotationSource {
	case "metrics":
		source = `metrics = {
        promql_query = "vector(1)"
        strategy     = { start_time = {} }
        labels       = ["service"]
      }`
	case "logs":
		source = `logs = {
        lucene_query = "*"
        strategy = { range = {
          start_timestamp_field = { keypath = ["start_time"], scope = "user_data" }
          end_timestamp_field   = { keypath = ["end_time"], scope = "user_data" }
        } }
      }`
	case "spans":
		source = `spans = {
        lucene_query = "*"
        strategy = { duration = {
          start_timestamp_field = { keypath = ["startTime"], scope = "metadata" }
          duration_field        = { keypath = ["durationNano"], scope = "metadata" }
        } }
      }`
	case "manual":
		source = `manual = {
        strategy = { range = { start_value = 10, end_value = 20, unit = "unspecified" } }
      }`
	default:
		panic(fmt.Sprintf("unsupported transition annotation source %q", transition.annotationSource))
	}
	return fmt.Sprintf(`{
    name   = "transition-annotation"
    source = { %s }
  }`, source)
}

func dashboardOpenAPITransitionFilterConfig(branch string) string {
	switch branch {
	case "metrics":
		return `  filters = [{ source = { metrics = {
    metric_name = "http_requests_total"
    label       = "service"
    operator    = { type = "equals", selected_values = ["api"] }
  } } }]`
	case "spans":
		return `  filters = [{ source = { spans = {
    field    = { type = "metadata", value = "service_name" }
    operator = { type = "equals", selected_values = ["api"] }
  } } }]`
	default:
		return ""
	}
}
