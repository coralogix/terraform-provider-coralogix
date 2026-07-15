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
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"

	dashboardservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_service"
)

type dashboardOneOfCoverageStatus string
type dashboardOneOfSupportDecision string

const (
	dashboardOneOfAcceptanceCovered dashboardOneOfCoverageStatus = "acceptance-covered"
	dashboardOneOfAcceptanceGap     dashboardOneOfCoverageStatus = "acceptance-gap"
	dashboardOneOfAPIOnly           dashboardOneOfCoverageStatus = "api-only"
	dashboardOneOfLegacyMigration   dashboardOneOfCoverageStatus = "legacy-migration"

	dashboardOneOfContentJSONSupported dashboardOneOfSupportDecision = "content-json-create-update-supported"
	dashboardOneOfStructuredRejected   dashboardOneOfSupportDecision = "structured-configuration-rejected"
	dashboardOneOfReadHydratable       dashboardOneOfSupportDecision = "backend-read-hydratable"
	dashboardOneOfReadRejected         dashboardOneOfSupportDecision = "backend-read-rejected"
	dashboardOneOfOutsideCRUD          dashboardOneOfSupportDecision = "outside-provider-crud"
	dashboardOneOfLegacyOnly           dashboardOneOfSupportDecision = "legacy-migration-only"

	dashboardNoProviderPath = "not exposed by the structured coralogix_dashboard schema"

	dashboardContentJSONGeneratedOneOfContractTestName = "TestDashboardContentJSONGeneratedOneOfBranchContract"
	dashboardStructuredRejectionContractTestName       = "TestDashboardStructuredConfigurationRejectsUnsupportedAutoRefreshBranches"
	dashboardOutsideCRUDContractTestName               = "TestDashboardOutsideCRUDOneOfContract"
)

type dashboardOneOfBranchCoverage struct {
	ProviderPath        string
	FixtureOrTest       string
	Status              dashboardOneOfCoverageStatus
	SupportDecision     dashboardOneOfSupportDecision
	ImportHydration     bool
	DataSourceHydration bool
	Explanation         string
}

type dashboardOneOfModelCoverage struct {
	ProtoSource    string
	Reconciliation string
	Branches       map[string]dashboardOneOfBranchCoverage
}

// Coverage policy:
//   - acceptance-covered: a structured provider branch is exercised through apply/read/import.
//   - acceptance-gap: the structured provider supports create and read hydration, but no branch-specific acceptance fixture exists.
//   - api-only: the generated API branch is not safely creatable through the structured provider. Hydration is recorded separately.
//   - legacy-migration: the branch is accepted only for old state/backend normalization and is not valid new configuration.
//
// Enum values are deliberately absent. This manifest contains generated oneOf
// branches only; enum exhaustiveness belongs in unit/table tests unless selecting
// the value changes the generated request shape.
var dashboardOpenAPIOneOfCoverage = dashboardOpenAPIOneOfCoverageManifest()

func covered(path, testName string) dashboardOneOfBranchCoverage {
	return dashboardOneOfBranchCoverage{
		ProviderPath:        path,
		FixtureOrTest:       testName,
		Status:              dashboardOneOfAcceptanceCovered,
		ImportHydration:     true,
		DataSourceHydration: true,
	}
}

func gap(path string) dashboardOneOfBranchCoverage {
	return dashboardOneOfBranchCoverage{
		ProviderPath:        path,
		Status:              dashboardOneOfAcceptanceGap,
		ImportHydration:     true,
		DataSourceHydration: true,
	}
}

func apiOnly(path string, hydration bool, explanation string) dashboardOneOfBranchCoverage {
	return dashboardOneOfBranchCoverage{
		ProviderPath:        path,
		FixtureOrTest:       dashboardContentJSONGeneratedOneOfContractTestName,
		Status:              dashboardOneOfAPIOnly,
		SupportDecision:     dashboardOneOfContentJSONSupported,
		ImportHydration:     hydration,
		DataSourceHydration: hydration,
		Explanation:         explanation,
	}
}

func structuredRejected(path, explanation string) dashboardOneOfBranchCoverage {
	return dashboardOneOfBranchCoverage{
		ProviderPath:    path,
		FixtureOrTest:   dashboardStructuredRejectionContractTestName,
		Status:          dashboardOneOfAPIOnly,
		SupportDecision: dashboardOneOfStructuredRejected,
		Explanation:     explanation,
	}
}

func outsideCRUD(path, explanation string) dashboardOneOfBranchCoverage {
	return dashboardOneOfBranchCoverage{
		ProviderPath:    path,
		FixtureOrTest:   dashboardOutsideCRUDContractTestName,
		Status:          dashboardOneOfAPIOnly,
		SupportDecision: dashboardOneOfOutsideCRUD,
		Explanation:     explanation,
	}
}

func observedAPIOnly(path, testName string, hydration bool, explanation string) dashboardOneOfBranchCoverage {
	coverage := apiOnly(path, hydration, explanation)
	coverage.FixtureOrTest = testName
	return coverage
}

func legacyMigration(path, testName, explanation string) dashboardOneOfBranchCoverage {
	return dashboardOneOfBranchCoverage{
		ProviderPath:        path,
		FixtureOrTest:       testName,
		Status:              dashboardOneOfLegacyMigration,
		SupportDecision:     dashboardOneOfLegacyOnly,
		ImportHydration:     true,
		DataSourceHydration: true,
		Explanation:         explanation,
	}
}

func apiOnlyModel(protoSource, explanation string, branches ...string) dashboardOneOfModelCoverage {
	result := dashboardOneOfModelCoverage{
		ProtoSource: protoSource,
		Branches:    make(map[string]dashboardOneOfBranchCoverage, len(branches)),
	}
	for _, branch := range branches {
		result.Branches[branch] = apiOnly(dashboardNoProviderPath, false, explanation)
	}
	return result
}

func outsideCRUDModel(protoSource, explanation string, branches ...string) dashboardOneOfModelCoverage {
	result := dashboardOneOfModelCoverage{
		ProtoSource: protoSource,
		Branches:    make(map[string]dashboardOneOfBranchCoverage, len(branches)),
	}
	for _, branch := range branches {
		result.Branches[branch] = outsideCRUD(dashboardNoProviderPath, explanation)
	}
	return result
}

func observedAPIOnlyModel(protoSource, explanation, testName string, observed []string, branches ...string) dashboardOneOfModelCoverage {
	result := apiOnlyModel(protoSource, explanation, branches...)
	for _, branch := range observed {
		coverage, ok := result.Branches[branch]
		if !ok {
			panic("observed API-only branch is absent from model: " + branch)
		}
		coverage.FixtureOrTest = testName
		result.Branches[branch] = coverage
	}
	return result
}

func dashboardOpenAPIOneOfCoverageManifest() map[string]dashboardOneOfModelCoverage {
	const (
		widget        = "layout.sections[*].rows[*].widgets[*].definition"
		variable      = "variables[*].definition.multi_select"
		variableQuery = variable + ".source.query.query"
		filter        = widget + ".*.query.*.filters[*]"
	)

	return map[string]dashboardOneOfModelCoverage{
		"ActionDefinition": apiOnlyModel(
			"common/action.proto#ActionDefinition.type",
			"action definitions are reachable only below WidgetDefinition.dynamic, which the structured provider does not expose or flatten",
			"customAction", "goToDashboardAction",
		),
		"AnnotationSource": {
			ProtoSource: "ast/annotations/annotation.proto#Annotation.Source.value",
			Branches: map[string]dashboardOneOfBranchCoverage{
				"metrics":         covered("annotations[*].source.metrics", dashboardOpenAPIAnnotationsTestName),
				"logs":            covered("annotations[*].source.logs", dashboardOpenAPIAnnotationsTestName),
				"spans":           covered("annotations[*].source.spans", dashboardOpenAPIAnnotationsTestName),
				"manual":          covered("annotations[*].source.manual", "TestAccCoralogixResourceDashboardManualAnnotation"),
				"dataprime":       apiOnly(dashboardNoProviderPath, false, "annotation.proto declares dataprime, but annotationSourceModelAttr and both annotation converters expose only metrics, logs, spans, and manual"),
				"eventRecurrence": apiOnly(dashboardNoProviderPath, false, "annotation.proto declares event_recurrence, but annotationSourceModelAttr and both annotation converters expose only metrics, logs, spans, and manual"),
			},
		},
		"AnnotationWidgetScope": apiOnlyModel(
			"ast/annotations/annotation.proto#Annotation.WidgetScope.value",
			"Annotation.scope is absent from DashboardAnnotationModel and the annotation schema/converters",
			"allWidgets", "specificWidgets",
		),
		"BarChartQuery": {
			ProtoSource: "ast/widgets/bar_chart.proto#BarChart.Query.value",
			Branches: map[string]dashboardOneOfBranchCoverage{
				"logs":      covered(widget+".bar_chart.query.logs", dashboardOpenAPILogsQueryTestName),
				"spans":     covered(widget+".bar_chart.query.spans", dashboardOpenAPISpansQueryTestName),
				"metrics":   covered(widget+".bar_chart.query.metrics", dashboardOpenAPIMetricsQueryTestName),
				"dataprime": covered(widget+".bar_chart.query.data_prime", dashboardOpenAPIDataPrimeQueryTestName),
			},
		},
		"CheckDashboardRequestDataStructure": outsideCRUDModel(
			"services/dashboard_check.proto#CheckDashboardRequest.source",
			"the provider CRUD client does not invoke the dashboard-check endpoint",
			"dashboard", "dashboardId",
		),
		"ColorLabelMapping": apiOnlyModel(
			"ast/widgets/common/color_label_mapping.proto#ColorLabelMapping.mapping_type",
			"color label mappings are reachable only below WidgetDefinition.dynamic",
			"range", "value", "regex",
		),
		"ColorsBy": {
			ProtoSource: "ast/widgets/common/colors_by.proto#ColorsBy.value",
			Branches: map[string]dashboardOneOfBranchCoverage{
				"stack":       covered(widget+".{bar_chart,horizontal_bar_chart}.colors_by=stack", dashboardOpenAPINestedPresentationTestName),
				"groupBy":     covered(widget+".{bar_chart,horizontal_bar_chart}.colors_by=group_by", dashboardOpenAPINestedPresentationTestName),
				"aggregation": covered(widget+".{bar_chart,horizontal_bar_chart}.colors_by=aggregation", dashboardOpenAPINestedPresentationTestName),
				"query":       apiOnly(widget+".{bar_chart,horizontal_bar_chart}.colors_by", false, "ColorsBy.query is declared in colors_by.proto but expandColorsBy and flattenBarChartColorsBy handle only stack, group_by, and aggregation"),
				"category":    apiOnly(widget+".{bar_chart,horizontal_bar_chart}.colors_by", false, "ColorsBy.category is declared in colors_by.proto but expandColorsBy and flattenBarChartColorsBy handle only stack, group_by, and aggregation"),
			},
		},
		"Dashboard": {
			ProtoSource:    "ast/dashboard.proto#Dashboard.auto_refresh + ast/dashboard.proto#Dashboard.time_frame",
			Reconciliation: "the OpenAPI generator places both protobuf oneofs on the single Dashboard REST model",
			Branches: map[string]dashboardOneOfBranchCoverage{
				"off":               covered("auto_refresh.type=off", dashboardOpenAPIBackendHydrationTestName),
				"twoMinutes":        covered("auto_refresh.type=two_minutes", dashboardOpenAPINestedPresentationTestName),
				"fiveMinutes":       covered("auto_refresh.type=five_minutes", dashboardOpenAPINestedPresentationTestName),
				"oneMinute":         structuredRejected("auto_refresh.type=one_minute", "dashboard.proto and the REST model expose one_minute, but the provider validator rejects it before both auto-refresh converters"),
				"fifteenMinutes":    structuredRejected("auto_refresh.type=fifteen_minutes", "dashboard.proto and the REST model expose fifteen_minutes, but the provider validator rejects it before both auto-refresh converters"),
				"absoluteTimeFrame": covered("time_frame.absolute", dashboardOpenAPIBackendHydrationTestName),
				"relativeTimeFrame": covered("time_frame.relative", "TestAccCoralogixResourceDashboard"),
			},
		},
		"DataTableQuery": {
			ProtoSource: "ast/widgets/data_table.proto#DataTable.Query.value",
			Branches: map[string]dashboardOneOfBranchCoverage{
				"logs":      covered(widget+".data_table.query.logs", dashboardOpenAPILogsQueryTestName),
				"spans":     covered(widget+".data_table.query.spans", dashboardOpenAPISpansQueryTestName),
				"metrics":   covered(widget+".data_table.query.metrics", dashboardOpenAPIMetricsQueryTestName),
				"dataprime": covered(widget+".data_table.query.data_prime", dashboardOpenAPIDataPrimeQueryTestName),
			},
		},
		"DataprimeSourceStrategy": apiOnlyModel(
			"ast/annotations/annotation.proto#Annotation.DataprimeSource.Strategy.value",
			"AnnotationSource.dataprime is not exposed by the structured provider",
			"instant", "range", "duration",
		),
		"DisplayNameTemplateVariable": apiOnlyModel(
			"ast/widgets/dynamic.proto#Dynamic.Visualization.PropertyLinks.StatCard.StatVisualElement.DisplayNameTemplateVariable.source",
			"display-name template variables are reachable only below WidgetDefinition.dynamic",
			"observationField", "mappedValues",
		),
		"DynamicQuery": observedAPIOnlyModel(
			"ast/widgets/dynamic.proto#Dynamic.Query.value",
			"WidgetDefinition.dynamic is reachable through content_json but is not exposed or flattened by the structured provider; import and data-source reads have no content_json plan to preserve",
			dashboardContentJSONDynamicQueriesTableTestName,
			[]string{"logs", "metrics", "spans"},
			"logs", "spans", "metrics", "dataprime",
		),
		"EqualsSelection": {
			ProtoSource: "ast/filters/filter.proto#Filter.Equals.Selection.value",
			Branches: map[string]dashboardOneOfBranchCoverage{
				"all":  covered(filter+".operator.selected_values=[]", dashboardOpenAPISpansAndFiltersTestName),
				"list": covered(filter+".operator.selected_values", "TestAccCoralogixResourceDashboard"),
			},
		},
		"EventRecurrenceSourceStrategy": apiOnlyModel(
			"ast/annotations/annotation.proto#Annotation.EventRecurrenceSource.Strategy.value",
			"AnnotationSource.event_recurrence is not exposed by the structured provider",
			"instant", "duration",
		),
		"FilterOperator": {
			ProtoSource: "ast/filters/filter.proto#Filter.Operator.value",
			Branches: map[string]dashboardOneOfBranchCoverage{
				"equals":    covered(filter+".operator.type=equals", "TestAccCoralogixResourceDashboard"),
				"notEquals": covered(filter+".operator.type=not_equals", dashboardOpenAPISpansAndFiltersTestName),
			},
		},
		"FilterPathAndValues": {
			ProtoSource:    "com/coralogixapis/events/v3/events_query_filter.proto#FilterPathAndValues.values",
			Reconciliation: "this guarded REST union is imported through dashboard query models; its protobuf declaration is outside dashboards/v1 and replaces no dashboard-local generated model",
			Branches: map[string]dashboardOneOfBranchCoverage{
				"multipleValues": outsideCRUD(dashboardNoProviderPath, "the imported events-v3 filter structure is not reachable from Dashboard CRUD request or response models"),
				"filters":        outsideCRUD(dashboardNoProviderPath, "the imported events-v3 filter structure is not reachable from Dashboard CRUD request or response models"),
			},
		},
		"FilterSource": {
			ProtoSource: "ast/filters/filter.proto#Filter.Source.value",
			Branches: map[string]dashboardOneOfBranchCoverage{
				"logs":    covered(widget+".*.query.data_prime.filters[*].logs", dashboardOpenAPIDataPrimeQueryTestName),
				"spans":   covered(widget+".*.query.data_prime.filters[*].spans", dashboardOpenAPIDataPrimeQueryTestName),
				"metrics": covered(widget+".*.query.data_prime.filters[*].metrics", dashboardOpenAPIDataPrimeQueryTestName),
			},
		},
		"GaugeQuery": {
			ProtoSource: "ast/widgets/gauge.proto#Gauge.Query.value",
			Branches: map[string]dashboardOneOfBranchCoverage{
				"metrics":   covered(widget+".gauge.query.metrics", dashboardOpenAPIBackendHydrationTestName),
				"logs":      covered(widget+".gauge.query.logs", dashboardOpenAPILogsQueryTestName),
				"spans":     covered(widget+".gauge.query.spans", dashboardOpenAPISpansQueryTestName),
				"dataprime": covered(widget+".gauge.query.data_prime", dashboardOpenAPIDataPrimeQueryTestName),
			},
		},
		"GeomapAggregation": apiOnlyModel(
			"ast/widgets/dynamic.proto#Dynamic.Visualization.PropertyLinks.Geomap.GeomapAggregation.value",
			"geomap aggregation is reachable only below WidgetDefinition.dynamic",
			"count", "sum", "min", "max", "avg",
		),
		"GeomapColor": apiOnlyModel(
			"ast/widgets/dynamic.proto#Dynamic.Visualization.PropertyLinks.Geomap.GeomapColor.value",
			"geomap color is reachable only below WidgetDefinition.dynamic",
			"size", "colorRange",
		),
		"GeomapFieldConfig": apiOnlyModel(
			"ast/widgets/dynamic.proto#Dynamic.Visualization.PropertyLinks.Geomap.GeomapFieldConfig.value",
			"geomap field configuration is reachable only below WidgetDefinition.dynamic",
			"coordinateConfig", "awsRegionConfig",
		),
		"Heatmap": apiOnlyModel(
			"ast/widgets/dynamic.proto#Dynamic.Visualization.PropertyLinks.Heatmap.color_config",
			"heatmap color configuration is reachable only below WidgetDefinition.dynamic",
			"preset", "colorRange",
		),
		"HexagonQuery": {
			ProtoSource: "ast/widgets/hexagon.proto#Hexagon.Query.value",
			Branches: map[string]dashboardOneOfBranchCoverage{
				"metrics":   covered(widget+".hexagon.query.metrics", dashboardOpenAPIMetricsQueryTestName),
				"logs":      covered(widget+".hexagon.query.logs", dashboardOpenAPILogsQueryTestName),
				"spans":     covered(widget+".hexagon.query.spans", dashboardOpenAPISpansQueryTestName),
				"dataprime": covered(widget+".hexagon.query.data_prime", dashboardOpenAPIDataPrimeQueryTestName),
			},
		},
		"HorizontalBarChartQuery": {
			ProtoSource: "ast/widgets/horizontal_bar_chart.proto#HorizontalBarChart.Query.value",
			Branches: map[string]dashboardOneOfBranchCoverage{
				"logs":      covered(widget+".horizontal_bar_chart.query.logs", dashboardOpenAPILogsQueryTestName),
				"spans":     covered(widget+".horizontal_bar_chart.query.spans", dashboardOpenAPISpansQueryTestName),
				"metrics":   covered(widget+".horizontal_bar_chart.query.metrics", dashboardOpenAPIMetricsQueryTestName),
				"dataprime": covered(widget+".horizontal_bar_chart.query.data_prime", dashboardOpenAPIDataPrimeQueryTestName),
			},
		},
		"HorizontalBarChartYAxisViewBy": {
			ProtoSource: "ast/widgets/horizontal_bar_chart.proto#HorizontalBarChart.YAxisViewBy.y_axis_view",
			Branches: map[string]dashboardOneOfBranchCoverage{
				"category": covered(widget+".horizontal_bar_chart.y_axis_view_by=category", dashboardOpenAPINestedPresentationTestName),
				"value":    covered(widget+".horizontal_bar_chart.y_axis_view_by=value", dashboardOpenAPINestedPresentationTestName),
			},
		},
		"IntervalResolution": apiOnlyModel(
			"ast/widgets/common/interval_resolution.proto#IntervalResolution.value",
			"the REST intervalResolution field is distinct from the legacy line-chart resolution block used by the provider and is ignored by both converters",
			"auto", "manual",
		),
		"LineChartQuery": {
			ProtoSource: "ast/widgets/line_chart.proto#LineChart.Query.value",
			Branches: map[string]dashboardOneOfBranchCoverage{
				"logs":      covered(widget+".line_chart.query_definitions[*].query.logs", dashboardOpenAPILogsQueryTestName),
				"metrics":   covered(widget+".line_chart.query_definitions[*].query.metrics", dashboardOpenAPIMetricsQueryTestName),
				"spans":     covered(widget+".line_chart.query_definitions[*].query.spans", dashboardOpenAPISpansQueryTestName),
				"dataprime": covered(widget+".line_chart.query_definitions[*].query.data_prime", dashboardOpenAPIDataPrimeQueryTestName),
			},
		},
		"LogsAggregation": {
			ProtoSource: "common/logs_aggregation.proto#LogsAggregation.value",
			Branches: map[string]dashboardOneOfBranchCoverage{
				"count":         covered(widget+".*.query.logs.aggregation.type=count", "TestAccCoralogixResourceDashboard"),
				"countDistinct": covered(widget+".*.query.logs.aggregation.type=count_distinct", dashboardOpenAPILogsAggregationTestName),
				"sum":           covered(widget+".*.query.logs.aggregation.type=sum", dashboardOpenAPILogsAggregationTestName),
				"average":       covered(widget+".*.query.logs.aggregation.type=avg", dashboardOpenAPILogsAggregationTestName),
				"min":           covered(widget+".*.query.logs.aggregation.type=min", dashboardOpenAPILogsAggregationTestName),
				"max":           covered(widget+".*.query.logs.aggregation.type=max", dashboardOpenAPILogsAggregationTestName),
				"percentile":    covered(widget+".*.query.logs.aggregation.type=percentile", dashboardOpenAPILogsAggregationTestName),
			},
		},
		"LogsSourceStrategy": {
			ProtoSource: "ast/annotations/annotation.proto#Annotation.LogsSource.Strategy.value",
			Branches: map[string]dashboardOneOfBranchCoverage{
				"instant":  covered("annotations[*].source.logs.strategy.instant", dashboardOpenAPIAnnotationsTestName),
				"range":    covered("annotations[*].source.logs.strategy.range", dashboardOpenAPIAnnotationsTestName),
				"duration": covered("annotations[*].source.logs.strategy.duration", dashboardOpenAPIAnnotationsTestName),
			},
		},
		"ManualSourceStrategy": {
			ProtoSource: "ast/annotations/annotation.proto#Annotation.ManualSource.Strategy.value",
			Branches: map[string]dashboardOneOfBranchCoverage{
				"instant": covered("annotations[*].source.manual.strategy.instant", dashboardOpenAPIAnnotationsTestName),
				"range":   covered("annotations[*].source.manual.strategy.range", "TestAccCoralogixResourceDashboardManualAnnotation"),
			},
		},
		"MinMax": apiOnlyModel(
			"ast/widgets/common/min_max.proto#MinMax.value",
			"MinMax is used by the dynamic geomap visualization, which the structured provider does not expose",
			"auto", "custom",
		),
		"MultiSelectQuery": {
			ProtoSource: "ast/variables/variable.proto#MultiSelect.Query.value",
			Branches: map[string]dashboardOneOfBranchCoverage{
				"logsQuery":    covered(variableQuery+".logs", dashboardOpenAPIVariablesTestName),
				"metricsQuery": covered(variableQuery+".metrics", "TestAccCoralogixResourceDashboard"),
				"spansQuery":   covered(variableQuery+".spans", dashboardOpenAPIVariablesTestName),
			},
		},
		"MultiSelectSelection": {
			ProtoSource: "ast/variables/variable.proto#MultiSelect.Selection.value",
			Branches: map[string]dashboardOneOfBranchCoverage{
				"all":  covered(variable+".selected_values=[]", dashboardOpenAPIVariablesTestName),
				"list": covered(variable+".selected_values", "TestAccCoralogixResourceDashboard"),
			},
		},
		"MultiSelectSource": {
			ProtoSource: "ast/variables/variable.proto#MultiSelect.Source.value",
			Branches: map[string]dashboardOneOfBranchCoverage{
				"logsPath":     covered(variable+".source.logs_path", dashboardOpenAPIVariablesTestName),
				"metricLabel":  covered(variable+".source.metric_label", dashboardOpenAPIVariablesTestName),
				"constantList": covered(variable+".source.constant_list", "TestAccCoralogixResourceDashboard"),
				"spanField":    covered(variable+".source.span_field", dashboardOpenAPIVariablesTestName),
				"query":        covered(variable+".source.query", "TestAccCoralogixResourceDashboard"),
			},
		},
		"MultiStringValue": apiOnlyModel(
			"ast/variables_v2/variable_value.proto#VariableValueV2.MultiStringValue.value",
			"the provider exposes legacy variables, not variables_v2 values",
			"all", "list", "selectedAll",
		),
		"PieChartQuery": {
			ProtoSource: "ast/widgets/pie_chart.proto#PieChart.Query.value",
			Branches: map[string]dashboardOneOfBranchCoverage{
				"logs":      covered(widget+".pie_chart.query.logs", dashboardOpenAPILogsQueryTestName),
				"spans":     covered(widget+".pie_chart.query.spans", dashboardOpenAPISpansQueryTestName),
				"metrics":   covered(widget+".pie_chart.query.metrics", dashboardOpenAPIMetricsQueryTestName),
				"dataprime": covered(widget+".pie_chart.query.data_prime", dashboardOpenAPIDataPrimeQueryTestName),
			},
		},
		"PropertyDefinition": apiOnlyModel(
			"ast/widgets/dynamic.proto#Dynamic.Visualization.Table.PropertyDefinition.value",
			"dynamic table property definitions are reachable only below WidgetDefinition.dynamic",
			"thresholds", "alignment", "units", "regexExtract", "link", "valuesAlias", "valuesMapping", "columnDisplayName",
		),
		"QueryLogsQueryType": {
			ProtoSource: "ast/variables/variable.proto#MultiSelect.Query.LogsQuery.Type.value",
			Branches: map[string]dashboardOneOfBranchCoverage{
				"fieldName":  covered(variableQuery+".logs.field_name", dashboardOpenAPIVariablesTestName),
				"fieldValue": covered(variableQuery+".logs.field_value", dashboardOpenAPIVariablesTestName),
			},
		},
		"QueryMetricsQueryOperator": {
			ProtoSource: "ast/variables/variable.proto#MultiSelect.Query.MetricsQuery.Operator.value",
			Branches: map[string]dashboardOneOfBranchCoverage{
				"equals":    covered(variableQuery+".metrics.*.label_filters[*].operator.type=equals", "TestAccCoralogixResourceDashboard"),
				"notEquals": covered(variableQuery+".metrics.*.label_filters[*].operator.type=not_equals", dashboardOpenAPIVariablesTestName),
			},
		},
		"QueryMetricsQueryStringOrVariable": {
			ProtoSource: "ast/variables/variable.proto#MultiSelect.Query.MetricsQuery.StringOrVariable.value",
			Branches: map[string]dashboardOneOfBranchCoverage{
				"stringValue":  covered(variableQuery+".metrics.*.*.string_value", "TestAccCoralogixResourceDashboard"),
				"variableName": covered(variableQuery+".metrics.*.*.variable_name", dashboardOpenAPIVariablesTestName),
			},
		},
		"QueryMetricsQueryType": {
			ProtoSource: "ast/variables/variable.proto#MultiSelect.Query.MetricsQuery.Type.value",
			Branches: map[string]dashboardOneOfBranchCoverage{
				"metricName": covered(variableQuery+".metrics.metric_name", dashboardOpenAPIVariablesTestName),
				"labelName":  covered(variableQuery+".metrics.label_name", dashboardOpenAPIVariablesTestName),
				"labelValue": covered(variableQuery+".metrics.label_value", "TestAccCoralogixResourceDashboard"),
			},
		},
		"QuerySourceLogsQueryType": apiOnlyModel(
			"ast/variables_v2/variable_source.proto#VariableSourceV2.QuerySource.LogsQuery.Type.value",
			"the provider exposes legacy variables, not VariableSourceV2",
			"fieldName", "fieldValue",
		),
		"QuerySourceMetricsQueryOperator": apiOnlyModel(
			"ast/variables_v2/variable_source.proto#VariableSourceV2.QuerySource.MetricsQuery.Operator.value",
			"the provider exposes legacy variables, not VariableSourceV2",
			"equals", "notEquals",
		),
		"QuerySourceMetricsQueryStringOrVariable": apiOnlyModel(
			"ast/variables_v2/variable_source.proto#VariableSourceV2.QuerySource.MetricsQuery.StringOrVariable.value",
			"the provider exposes legacy variables, not VariableSourceV2",
			"stringValue", "variableName",
		),
		"QuerySourceMetricsQueryType": apiOnlyModel(
			"ast/variables_v2/variable_source.proto#VariableSourceV2.QuerySource.MetricsQuery.Type.value",
			"the provider exposes legacy variables, not VariableSourceV2",
			"metricName", "labelName", "labelValue", "promqlQuery",
		),
		"QuerySourceSpansQueryType": apiOnlyModel(
			"ast/variables_v2/variable_source.proto#VariableSourceV2.QuerySource.SpansQuery.Type.value",
			"the provider exposes legacy variables, not VariableSourceV2",
			"fieldName", "fieldValue",
		),
		"QuerySpansQueryType": {
			ProtoSource: "ast/variables/variable.proto#MultiSelect.Query.SpansQuery.Type.value",
			Branches: map[string]dashboardOneOfBranchCoverage{
				"fieldName":  covered(variableQuery+".spans.field_name", dashboardOpenAPIVariablesTestName),
				"fieldValue": covered(variableQuery+".spans.field_value", dashboardOpenAPIVariablesTestName),
			},
		},
		"RuleScope": apiOnlyModel(
			"ast/widgets/dynamic.proto#Dynamic.Visualization.Table.RuleScope.value",
			"dynamic table rule scope is reachable only below WidgetDefinition.dynamic",
			"field", "regex", "fieldType",
		),
		"SectionOptions": {
			ProtoSource: "ast/layout.proto#SectionOptions.value",
			Branches: map[string]dashboardOneOfBranchCoverage{
				"internal": apiOnly(dashboardNoProviderPath, false, "expandSectionOptions always creates Custom and flattenDashboardOptions intentionally ignores Internal"),
				"custom":   covered("layout.sections[*].options", "TestAccCoralogixResourceDashboard"),
			},
		},
		"SortStrategy": apiOnlyModel(
			"ast/widgets/dynamic.proto#Dynamic.Visualization.PropertyLinks.SortOrder.SortStrategy.strategy",
			"dynamic sort strategy is reachable only below WidgetDefinition.dynamic",
			"category", "queryValue",
		),
		"SpanField": {
			ProtoSource: "common/span_field.proto#SpanField.value",
			Branches: map[string]dashboardOneOfBranchCoverage{
				"metadataField":   covered(widget+".*.query.spans.*.field.type=metadata", "TestAccCoralogixResourceDashboardLinechartWidget"),
				"tagField":        covered(widget+".*.query.spans.*.field.type=tag", "TestAccCoralogixResourceDashboardLinechartWidget"),
				"processTagField": covered(widget+".*.query.spans.*.field.type=process_tag", dashboardOpenAPISpansAndFiltersTestName),
			},
		},
		"SpansAggregation": {
			ProtoSource: "common/spans_aggregation.proto#SpansAggregation.aggregation",
			Branches: map[string]dashboardOneOfBranchCoverage{
				"metricAggregation":    covered(widget+".*.query.spans.aggregations[*].type=metric", dashboardOpenAPISpansAndFiltersTestName),
				"dimensionAggregation": covered(widget+".*.query.spans.aggregations[*].type=dimension", "TestAccCoralogixResourceDashboardLinechartWidget"),
			},
		},
		"SpansSourceStrategy": {
			ProtoSource: "ast/annotations/annotation.proto#Annotation.SpansSource.Strategy.value",
			Branches: map[string]dashboardOneOfBranchCoverage{
				"instant":  covered("annotations[*].source.spans.strategy.instant", dashboardOpenAPIAnnotationsTestName),
				"range":    covered("annotations[*].source.spans.strategy.range", dashboardOpenAPIAnnotationsTestName),
				"duration": covered("annotations[*].source.spans.strategy.duration", dashboardOpenAPIAnnotationsTestName),
			},
		},
		"StatVisualElement": apiOnlyModel(
			"ast/widgets/dynamic.proto#Dynamic.Visualization.PropertyLinks.StatCard.StatVisualElement.value_type",
			"stat-card visual elements are reachable only below WidgetDefinition.dynamic",
			"observationField", "mappedValues",
		),
		"TextboxDefaultValue": apiOnlyModel(
			"ast/variables_v2/variable_source.proto#VariableSourceV2.TextboxSource.TextboxDefaultValue.value",
			"the provider exposes legacy variables, not VariableSourceV2 textbox sources",
			"singleString", "singleNumeric", "defaultStringValue", "defaultNumericValue", "defaultLuceneValue", "defaultRegexValue", "defaultIntervalValue",
		),
		"TimeFrameSelect": {
			ProtoSource: "common/time_frame.proto#TimeFrameSelect.value",
			Branches: map[string]dashboardOneOfBranchCoverage{
				"absoluteTimeFrame": covered("time_frame.absolute (also available on query-level time_frame blocks)", dashboardOpenAPINestedPresentationTestName),
				"relativeTimeFrame": covered("time_frame.relative (also available on query-level time_frame blocks)", dashboardOpenAPIBackendHydrationTestName),
			},
		},
		"VariableDefinition": {
			ProtoSource: "ast/variables/variable.proto#Variable.Definition.value",
			Branches: map[string]dashboardOneOfBranchCoverage{
				"constant": legacyMigration("variables[*].definition.constant_value", "TestExpandDashboardVariableDefinition_ConstantValueDeprecated",
					"constant is deprecated in variable.proto; new configuration is rejected and backend/old-state values normalize to a one-item multi_select constant_list"),
				"multiSelect": covered("variables[*].definition.multi_select", "TestAccCoralogixResourceDashboard"),
			},
		},
		"VariableSourceV2": apiOnlyModel(
			"ast/variables_v2/variable_source.proto#VariableSourceV2.value",
			"DashboardResourceModel exposes the legacy variables schema, not variables_v2",
			"static", "query", "textbox",
		),
		"VariableSourceV2QuerySource": apiOnlyModel(
			"ast/variables_v2/variable_source.proto#VariableSourceV2.QuerySource.value",
			"DashboardResourceModel exposes the legacy variables schema, not variables_v2",
			"logsQuery", "metricsQuery", "spansQuery", "dataprimeQuery",
		),
		"VariableValueV2": apiOnlyModel(
			"ast/variables_v2/variable_value.proto#VariableValueV2.value",
			"DashboardResourceModel exposes the legacy variables schema, not variables_v2",
			"multiString", "singleString", "singleNumeric", "regex", "lucene", "interval",
		),
		"Visualization": observedAPIOnlyModel(
			"ast/widgets/dynamic.proto#Dynamic.Visualization.value",
			"all Visualization branches are children of WidgetDefinition.dynamic, which is reachable through content_json but absent from the structured schema and flattener; import and data-source reads cannot reconstruct it",
			dashboardContentJSONDynamicQueriesTableTestName,
			[]string{"table"},
			"table", "timeSeriesLines", "timeSeriesBars", "stat", "gauge", "hexagonBins", "pieChart", "horizontalBars", "verticalBars", "heatmap", "geomap", "timeSeriesLinesMulti", "verticalBarsMulti", "horizontalBarsMulti", "statCard",
		),
		"WidgetDefinition": {
			ProtoSource: "ast/widget.proto#Widget.Definition.value",
			Branches: map[string]dashboardOneOfBranchCoverage{
				"lineChart":          covered(widget+".line_chart", dashboardOpenAPILogsQueryTestName),
				"dataTable":          covered(widget+".data_table", dashboardOpenAPILogsQueryTestName),
				"gauge":              covered(widget+".gauge", dashboardOpenAPIBackendHydrationTestName),
				"pieChart":           covered(widget+".pie_chart", dashboardOpenAPILogsQueryTestName),
				"barChart":           covered(widget+".bar_chart", dashboardOpenAPILogsQueryTestName),
				"horizontalBarChart": covered(widget+".horizontal_bar_chart", dashboardOpenAPILogsQueryTestName),
				"markdown":           covered(widget+".markdown", dashboardOpenAPILogsQueryTestName),
				"hexagon":            covered(widget+".hexagon", dashboardOpenAPILogsQueryTestName),
				"dynamic": observedAPIOnly(dashboardNoProviderPath, dashboardContentJSONDynamicQueriesTableTestName, false,
					"dynamic is supported through content_json, but SupportedWidgetTypes, widgetModelAttr, expandDashboardWidgetDefinition, and flattenDashboardWidgetDefinition omit it; import and data-source reads cannot reconstruct content_json"),
			},
		},
		"XAxis": {
			ProtoSource: "ast/widgets/bar_chart.proto#BarChart.XAxis.type",
			Branches: map[string]dashboardOneOfBranchCoverage{
				"value":       covered(widget+".bar_chart.xaxis.value", dashboardOpenAPINestedPresentationTestName),
				"time":        covered(widget+".bar_chart.xaxis.time", dashboardOpenAPINestedPresentationTestName),
				"timeBuckets": apiOnly(dashboardNoProviderPath, false, "bar_chart.proto retains time_buckets, but the schema model and both XAxis converters support only value and time"),
			},
		},
	}
}

type dashboardProtoOnlyBranch struct {
	Model       string
	Branch      string
	ProtoSource string
	Explanation string
}

var dashboardProtoOnlyBranches = []dashboardProtoOnlyBranch{
	{
		Model:       "AnnotationEvent",
		Branch:      "instant",
		ProtoSource: "common/annotation_event.proto#AnnotationEvent.value.instant",
		Explanation: "the current OpenAPI document does not reference AnnotationEvent as a guarded generated union",
	},
	{
		Model:       "AnnotationEvent",
		Branch:      "range",
		ProtoSource: "common/annotation_event.proto#AnnotationEvent.value.range",
		Explanation: "the current OpenAPI document does not reference AnnotationEvent as a guarded generated union",
	},
}

// These seven protobuf oneofs are presence wrappers with one live arm, not
// unions. They are intentionally kept out of the 216-branch generated-union
// manifest and should not be confused with enums.
var dashboardSingleArmProtoOneOfs = []string{
	"ast/annotations/annotation.proto#Annotation.EventRecurrenceSource.Recurrence.frequency_type.weekly",
	"ast/annotations/annotation.proto#Annotation.MetricsSource.Strategy.value.start_time_metric",
	"ast/filters/filter.proto#Filter.NotEquals.Selection.value.list",
	"ast/layout.proto#SectionColor.value.predefined",
	"ast/variables/variable.proto#MultiSelect.Query.MetricsQuery.Selection.value.list",
	"ast/variables_v2/variable_source.proto#VariableSourceV2.QuerySource.DataprimeQuery.Type.value.query_text",
	"ast/variables_v2/variable_source.proto#VariableSourceV2.QuerySource.MetricsQuery.Selection.value.list",
}

func TestDashboardOpenAPIOneOfCoverageManifest(t *testing.T) {
	generated := generatedDashboardOneOfGuards(t)
	tests := dashboardTestFunctions(t)

	if got := len(generated); got != 63 {
		t.Fatalf("generated dashboard union-bearing models = %d, want 63", got)
	}

	generatedBranches := 0
	for _, branches := range generated {
		generatedBranches += len(branches)
	}
	if generatedBranches != 216 {
		t.Fatalf("generated dashboard union branches = %d, want 216", generatedBranches)
	}

	for model, generatedModelBranches := range generated {
		modelCoverage, ok := dashboardOpenAPIOneOfCoverage[model]
		if !ok {
			t.Errorf("generated model %s is unclassified", model)
			continue
		}
		if modelCoverage.ProtoSource == "" {
			t.Errorf("model %s has no protobuf source", model)
		}

		for branch := range generatedModelBranches {
			coverage, ok := modelCoverage.Branches[branch]
			if !ok {
				t.Errorf("generated branch %s.%s is unclassified", model, branch)
				continue
			}
			validateDashboardOneOfCoverage(t, tests, model, branch, coverage)
		}

		for branch := range modelCoverage.Branches {
			if _, ok := generatedModelBranches[branch]; !ok {
				t.Errorf("manifest branch %s.%s does not exist in the pinned SDK", model, branch)
			}
		}
	}

	for model := range dashboardOpenAPIOneOfCoverage {
		if _, ok := generated[model]; !ok {
			t.Errorf("manifest model %s does not exist in the pinned SDK guards", model)
		}
	}

	manifestBranches := 0
	for _, model := range dashboardOpenAPIOneOfCoverage {
		manifestBranches += len(model.Branches)
	}
	if manifestBranches != 216 {
		t.Errorf("manifest branches = %d, want 216", manifestBranches)
	}

	assertDashboardAPIOnlyBranch(t, "WidgetDefinition", "dynamic", false)
	assertDashboardAPIOnlyBranch(t, "AnnotationSource", "dataprime", false)
	assertDashboardAPIOnlyBranch(t, "AnnotationSource", "eventRecurrence", false)
	assertDashboardAPIOnlyBranch(t, "Dashboard", "oneMinute", false)
	assertDashboardAPIOnlyBranch(t, "Dashboard", "fifteenMinutes", false)
}

func TestDashboardProtoAndRESTOneOfReconciliation(t *testing.T) {
	// Source inventory: 71 protobuf oneofs = 64 multi-branch unions and seven
	// single-arm presence wrappers. The 64 unions have 216 branches.
	const (
		protoOneOfs            = 71
		protoMultiBranchOneOfs = 64
		protoMultiBranches     = 216
	)
	if protoOneOfs-protoMultiBranchOneOfs != len(dashboardSingleArmProtoOneOfs) {
		t.Fatalf("single-arm protobuf reconciliation is incomplete")
	}
	if protoMultiBranches != 216 {
		t.Fatalf("protobuf multi-branch count changed")
	}
	if len(dashboardProtoOnlyBranches) != 2 {
		t.Fatalf("proto-only branch reconciliation = %d, want 2", len(dashboardProtoOnlyBranches))
	}

	filterPath := dashboardOpenAPIOneOfCoverage["FilterPathAndValues"]
	if !strings.Contains(filterPath.ProtoSource, "events/v3/events_query_filter.proto") {
		t.Fatalf("FilterPathAndValues must identify its imported, REST-only-for-dashboards protobuf source")
	}
	if filterPath.Reconciliation == "" {
		t.Fatalf("FilterPathAndValues discrepancy has no explanation")
	}

	dashboard := dashboardOpenAPIOneOfCoverage["Dashboard"]
	if !strings.Contains(dashboard.ProtoSource, "auto_refresh") || !strings.Contains(dashboard.ProtoSource, "time_frame") {
		t.Fatalf("Dashboard must identify both protobuf oneofs merged into its generated REST model")
	}
	if dashboard.Reconciliation == "" {
		t.Fatalf("Dashboard merge discrepancy has no explanation")
	}

	// The two imported FilterPathAndValues branches replace the two proto-only
	// AnnotationEvent branches in the guarded REST inventory. Merging the two
	// Dashboard oneofs reduces 64 source unions to 63 generated models without
	// changing the reconciled 216 branch count.
	if got := len(dashboardOpenAPIOneOfCoverage); got != 63 {
		t.Fatalf("reconciled generated models = %d, want 63", got)
	}
}

func TestDashboardDynamicContentJSONImportAndDataSourceWaiver(t *testing.T) {
	models := map[string][]string{
		"WidgetDefinition": {"dynamic"},
		"DynamicQuery":     {"logs", "metrics", "spans", "dataprime"},
		"Visualization":    {"table", "timeSeriesLines", "timeSeriesBars", "stat", "gauge", "hexagonBins", "pieChart", "horizontalBars", "verticalBars", "heatmap", "geomap", "timeSeriesLinesMulti", "verticalBarsMulti", "horizontalBarsMulti", "statCard"},
	}
	observed := map[string]struct{}{
		"WidgetDefinition.dynamic": {},
		"DynamicQuery.logs":        {},
		"DynamicQuery.metrics":     {},
		"DynamicQuery.spans":       {},
		"Visualization.table":      {},
	}
	for model, branches := range models {
		for _, branch := range branches {
			coverage := dashboardOpenAPIOneOfCoverage[model].Branches[branch]
			if coverage.Status != dashboardOneOfAPIOnly {
				t.Errorf("%s.%s status = %q, want api-only", model, branch, coverage.Status)
			}
			_, isObserved := observed[model+"."+branch]
			if isObserved && coverage.FixtureOrTest != dashboardContentJSONDynamicQueriesTableTestName {
				t.Errorf("%s.%s fixture = %q, want %q", model, branch, coverage.FixtureOrTest, dashboardContentJSONDynamicQueriesTableTestName)
			}
			if !isObserved && coverage.FixtureOrTest != dashboardContentJSONGeneratedOneOfContractTestName {
				t.Errorf("%s.%s fixture = %q, want shared generated-model contract %q", model, branch, coverage.FixtureOrTest, dashboardContentJSONGeneratedOneOfContractTestName)
			}
			if coverage.SupportDecision != dashboardOneOfContentJSONSupported {
				t.Errorf("%s.%s support decision = %q, want %q", model, branch, coverage.SupportDecision, dashboardOneOfContentJSONSupported)
			}
			if coverage.ImportHydration || coverage.DataSourceHydration {
				t.Errorf("%s.%s must not claim import/data-source hydration for content_json-only coverage", model, branch)
			}
			if !strings.Contains(coverage.Explanation, "import") || !strings.Contains(coverage.Explanation, "data-source") {
				t.Errorf("%s.%s does not document the content_json hydration waiver", model, branch)
			}
		}
	}
}

func TestDashboardAPIOnlyDecisionReport(t *testing.T) {
	report := dashboardAPIOnlyDecisionReport()
	if report == "" {
		t.Fatal("API-only decision report is empty")
	}
	lines := strings.Split(report, "\n")
	if !sort.StringsAreSorted(lines) {
		t.Fatal("API-only decision report is not deterministic")
	}

	apiOnlyBranches := 0
	for _, model := range dashboardOpenAPIOneOfCoverage {
		for _, branch := range model.Branches {
			if branch.Status == dashboardOneOfAPIOnly {
				apiOnlyBranches++
			}
		}
	}
	if len(lines) != apiOnlyBranches {
		t.Fatalf("API-only report lines = %d, want %d", len(lines), apiOnlyBranches)
	}
	t.Logf("API-only dashboard oneOf decisions:\n%s", report)
}

func TestDashboardOutsideCRUDOneOfContract(t *testing.T) {
	outOfScopeModels := []reflect.Type{
		reflect.TypeOf(dashboardservice.CheckDashboardRequestDataStructure{}),
		reflect.TypeOf(dashboardservice.FilterPathAndValues{}),
	}
	crudModels := []reflect.Type{
		reflect.TypeOf(dashboardservice.Dashboard{}),
		reflect.TypeOf(dashboardservice.CreateDashboardRequestDataStructure{}),
		reflect.TypeOf(dashboardservice.ReplaceDashboardRequestDataStructure{}),
	}
	for _, outOfScope := range outOfScopeModels {
		for _, crud := range crudModels {
			if dashboardGeneratedTypeReachable(crud, outOfScope) {
				t.Fatalf("%s unexpectedly reaches out-of-scope union %s", crud.Name(), outOfScope.Name())
			}
		}
	}

	expected := map[string][]string{
		"CheckDashboardRequestDataStructure": {"dashboard", "dashboardId"},
		"FilterPathAndValues":                {"multipleValues", "filters"},
	}
	for model, branches := range expected {
		coverage := dashboardOpenAPIOneOfCoverage[model]
		for _, branch := range branches {
			decision := coverage.Branches[branch]
			if decision.SupportDecision != dashboardOneOfOutsideCRUD || decision.FixtureOrTest != dashboardOutsideCRUDContractTestName {
				t.Errorf("%s.%s decision = %q/%q, want outside-CRUD contract", model, branch, decision.SupportDecision, decision.FixtureOrTest)
			}
		}
	}
}

func dashboardAPIOnlyDecisionReport() string {
	lines := make([]string, 0)
	for modelName, model := range dashboardOpenAPIOneOfCoverage {
		for branchName, branch := range model.Branches {
			if branch.Status != dashboardOneOfAPIOnly {
				continue
			}
			lines = append(lines, fmt.Sprintf(
				"%s.%s\tdecision=%s\tprovider_path=%s\timport_hydration=%t\tdata_source_hydration=%t\ttest=%s\texplanation=%s",
				modelName,
				branchName,
				branch.SupportDecision,
				branch.ProviderPath,
				branch.ImportHydration,
				branch.DataSourceHydration,
				branch.FixtureOrTest,
				branch.Explanation,
			))
		}
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

func dashboardGeneratedTypeReachable(root, target reflect.Type) bool {
	packagePath := reflect.TypeOf(dashboardservice.Dashboard{}).PkgPath()
	queue := []reflect.Type{root}
	visited := make(map[reflect.Type]struct{})
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for current.Kind() == reflect.Pointer || current.Kind() == reflect.Slice || current.Kind() == reflect.Array {
			current = current.Elem()
		}
		if current == target {
			return true
		}
		if current.Kind() != reflect.Struct || current.PkgPath() != packagePath {
			continue
		}
		if _, seen := visited[current]; seen {
			continue
		}
		visited[current] = struct{}{}
		for fieldIndex := 0; fieldIndex < current.NumField(); fieldIndex++ {
			queue = append(queue, current.Field(fieldIndex).Type)
		}
	}
	return false
}

func dashboardGeneratedModelType(name string) (reflect.Type, bool) {
	packagePath := reflect.TypeOf(dashboardservice.Dashboard{}).PkgPath()
	queue := []reflect.Type{reflect.TypeOf(dashboardservice.Dashboard{})}
	visited := make(map[reflect.Type]struct{})
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for current.Kind() == reflect.Pointer || current.Kind() == reflect.Slice || current.Kind() == reflect.Array {
			current = current.Elem()
		}
		if current.Kind() != reflect.Struct || current.PkgPath() != packagePath {
			continue
		}
		if current.Name() == name {
			return current, true
		}
		if _, seen := visited[current]; seen {
			continue
		}
		visited[current] = struct{}{}
		for fieldIndex := 0; fieldIndex < current.NumField(); fieldIndex++ {
			queue = append(queue, current.Field(fieldIndex).Type)
		}
	}
	return nil, false
}

func TestDashboardProtoOneOfInventoryAgainstCheckout(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate repository root")
	}
	repositoryRoot := filepath.Dir(filepath.Dir(filepath.Dir(file)))
	protoRoot := filepath.Join(filepath.Dir(repositoryRoot), "cx-management-apis", "proto")
	dashboardRoot := filepath.Join(protoRoot, "com", "coralogixapis", "dashboards", "v1")
	if _, err := os.Stat(dashboardRoot); os.IsNotExist(err) {
		t.Skip("sibling cx-management-apis checkout is unavailable; pinned SDK guard coverage is still enforced")
	} else if err != nil {
		t.Fatalf("stat dashboard protobuf source: %s", err)
	}

	protoInventory := parseDashboardProtoOneOfs(t, dashboardRoot, dashboardRoot)
	if got := len(protoInventory); got != 71 {
		t.Fatalf("dashboard protobuf oneofs = %d, want 71", got)
	}
	multiBranchOneOfs := 0
	multiBranches := 0
	for _, branches := range protoInventory {
		if len(branches) > 1 {
			multiBranchOneOfs++
			multiBranches += len(branches)
		}
	}
	if multiBranchOneOfs != 64 || multiBranches != 216 {
		t.Fatalf("dashboard protobuf multi-branch inventory = %d oneofs/%d branches, want 64/216", multiBranchOneOfs, multiBranches)
	}

	for _, source := range dashboardSingleArmProtoOneOfs {
		assertDashboardProtoBranch(t, protoInventory, source)
	}
	for _, branch := range dashboardProtoOnlyBranches {
		assertDashboardProtoBranch(t, protoInventory, branch.ProtoSource)
	}

	externalFile := filepath.Join(protoRoot, "com", "coralogixapis", "events", "v3", "events_query_filter.proto")
	parseDashboardProtoFile(t, protoRoot, externalFile, protoInventory)
	for model, coverage := range dashboardOpenAPIOneOfCoverage {
		var protoBranches []string
		for _, source := range strings.Split(coverage.ProtoSource, " + ") {
			branches, ok := protoInventory[source]
			if !ok {
				t.Errorf("%s references nonexistent protobuf oneof %s", model, source)
				continue
			}
			for _, branch := range branches {
				protoBranches = append(protoBranches, snakeToLowerCamel(branch))
			}
		}
		sort.Strings(protoBranches)
		manifestBranches := make([]string, 0, len(coverage.Branches))
		for branch := range coverage.Branches {
			manifestBranches = append(manifestBranches, branch)
		}
		sort.Strings(manifestBranches)
		if !reflect.DeepEqual(protoBranches, manifestBranches) {
			t.Errorf("%s protobuf branches = %v, manifest branches = %v", model, protoBranches, manifestBranches)
		}
	}
}

func validateDashboardOneOfCoverage(t *testing.T, tests map[string]struct{}, model, branch string, coverage dashboardOneOfBranchCoverage) {
	t.Helper()
	if coverage.ProviderPath == "" {
		t.Errorf("%s.%s has no provider entry path classification", model, branch)
	}

	switch coverage.Status {
	case dashboardOneOfAcceptanceCovered:
		if coverage.FixtureOrTest == "" {
			t.Errorf("%s.%s is covered without a fixture/test", model, branch)
		}
		if !coverage.ImportHydration || !coverage.DataSourceHydration {
			t.Errorf("%s.%s is acceptance-covered without both hydration paths", model, branch)
		}
	case dashboardOneOfAcceptanceGap:
		if coverage.FixtureOrTest != "" {
			t.Errorf("%s.%s is an acceptance gap but references %s", model, branch, coverage.FixtureOrTest)
		}
		if !coverage.ImportHydration || !coverage.DataSourceHydration {
			t.Errorf("%s.%s is a structured branch without both hydration paths", model, branch)
		}
	case dashboardOneOfAPIOnly:
		if coverage.Explanation == "" || coverage.FixtureOrTest == "" || coverage.SupportDecision == "" {
			t.Errorf("%s.%s API-only decision is missing category, explanation, or executable evidence", model, branch)
		}
		switch coverage.SupportDecision {
		case dashboardOneOfContentJSONSupported:
			modelType, ok := dashboardGeneratedModelType(model)
			if !ok || !dashboardGeneratedTypeReachable(reflect.TypeOf(dashboardservice.Dashboard{}), modelType) {
				t.Errorf("%s.%s claims content_json support but %s is not reachable from Dashboard", model, branch, model)
			}
			if coverage.FixtureOrTest != dashboardContentJSONGeneratedOneOfContractTestName && coverage.FixtureOrTest != dashboardContentJSONDynamicQueriesTableTestName {
				t.Errorf("%s.%s content_json decision references unrelated evidence %s", model, branch, coverage.FixtureOrTest)
			}
		case dashboardOneOfStructuredRejected:
			if coverage.FixtureOrTest != dashboardStructuredRejectionContractTestName {
				t.Errorf("%s.%s structured rejection does not reference %s", model, branch, dashboardStructuredRejectionContractTestName)
			}
		case dashboardOneOfReadHydratable:
			if !coverage.ImportHydration && !coverage.DataSourceHydration {
				t.Errorf("%s.%s claims read hydration without an import or data-source path", model, branch)
			}
		case dashboardOneOfReadRejected:
			if coverage.ImportHydration || coverage.DataSourceHydration {
				t.Errorf("%s.%s claims deterministic read rejection and hydration", model, branch)
			}
		case dashboardOneOfOutsideCRUD:
			if coverage.FixtureOrTest != dashboardOutsideCRUDContractTestName || coverage.ImportHydration || coverage.DataSourceHydration {
				t.Errorf("%s.%s outside-CRUD decision has inconsistent evidence or hydration", model, branch)
			}
		default:
			t.Errorf("%s.%s has unknown API-only support decision %q", model, branch, coverage.SupportDecision)
		}
	case dashboardOneOfLegacyMigration:
		if coverage.Explanation == "" || coverage.FixtureOrTest == "" || coverage.SupportDecision != dashboardOneOfLegacyOnly {
			t.Errorf("%s.%s legacy migration classification is incomplete", model, branch)
		}
	default:
		t.Errorf("%s.%s has unknown status %q", model, branch, coverage.Status)
	}

	if coverage.FixtureOrTest != "" {
		if _, ok := tests[coverage.FixtureOrTest]; !ok {
			t.Errorf("%s.%s references nonexistent test %s", model, branch, coverage.FixtureOrTest)
		}
	}
}

func assertDashboardAPIOnlyBranch(t *testing.T, model, branch string, hydration bool) {
	t.Helper()
	coverage, ok := dashboardOpenAPIOneOfCoverage[model].Branches[branch]
	if !ok {
		t.Fatalf("required API-only branch %s.%s is absent", model, branch)
	}
	if coverage.Status != dashboardOneOfAPIOnly {
		t.Fatalf("%s.%s status = %q, want %q", model, branch, coverage.Status, dashboardOneOfAPIOnly)
	}
	if coverage.ImportHydration != hydration || coverage.DataSourceHydration != hydration {
		t.Fatalf("%s.%s hydration = import:%t data-source:%t, want %t", model, branch, coverage.ImportHydration, coverage.DataSourceHydration, hydration)
	}
}

func generatedDashboardOneOfGuards(t *testing.T) map[string]map[string]struct{} {
	t.Helper()
	pc := reflect.ValueOf(dashboardservice.Dashboard.ToMap).Pointer()
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		t.Fatal("cannot locate pinned dashboard SDK source")
	}
	file, _ := fn.FileLine(pc)
	dir := filepath.Dir(file)

	typePattern := regexp.MustCompile(`(?m)^type ([A-Za-z0-9_]+) struct \{`)
	guardPattern := regexp.MustCompile(`oneOf field ([A-Za-z0-9_]+) must be set through the typed field`)
	result := make(map[string]map[string]struct{})

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read pinned dashboard SDK model directory: %s", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "model_") || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		content, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatalf("read %s: %s", entry.Name(), err)
		}
		guards := guardPattern.FindAllSubmatch(content, -1)
		if len(guards) == 0 {
			continue
		}
		modelMatch := typePattern.FindSubmatch(content)
		if len(modelMatch) != 2 {
			t.Fatalf("find model type in %s", entry.Name())
		}
		model := string(modelMatch[1])
		branches := make(map[string]struct{}, len(guards))
		for _, guard := range guards {
			branches[string(guard[1])] = struct{}{}
		}
		result[model] = branches
	}
	return result
}

func dashboardTestFunctions(t *testing.T) map[string]struct{} {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate provider test directory")
	}
	root := filepath.Dir(file)
	result := make(map[string]struct{})
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_test.go") {
			return nil
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		for _, declaration := range parsed.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if ok && strings.HasPrefix(function.Name.Name, "Test") {
				result[function.Name.Name] = struct{}{}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("discover dashboard tests: %s", err)
	}
	return result
}

type dashboardProtoScope struct {
	name  string
	depth int
}

type dashboardProtoOneOf struct {
	key      string
	depth    int
	branches []string
}

func parseDashboardProtoOneOfs(t *testing.T, relativeRoot, walkRoot string) map[string][]string {
	t.Helper()
	result := make(map[string][]string)
	err := filepath.WalkDir(walkRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".proto") {
			return nil
		}
		parseDashboardProtoFile(t, relativeRoot, path, result)
		return nil
	})
	if err != nil {
		t.Fatalf("inventory dashboard protobuf oneofs: %s", err)
	}
	return result
}

func parseDashboardProtoFile(t *testing.T, relativeRoot, path string, result map[string][]string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read protobuf source %s: %s", path, err)
	}
	relativePath, err := filepath.Rel(relativeRoot, path)
	if err != nil {
		t.Fatalf("make protobuf path relative: %s", err)
	}
	relativePath = filepath.ToSlash(relativePath)

	blockComments := regexp.MustCompile(`(?s)/\*.*?\*/`)
	quotedStrings := regexp.MustCompile(`"(?:\\.|[^"\\])*"`)
	messagePattern := regexp.MustCompile(`\bmessage\s+([A-Za-z0-9_]+)\s*\{`)
	oneOfPattern := regexp.MustCompile(`\boneof\s+([A-Za-z0-9_]+)\s*\{`)
	fieldPattern := regexp.MustCompile(`^\s*(?:repeated\s+)?[.A-Za-z_][A-Za-z0-9_.<>]*\s+([A-Za-z0-9_]+)\s*=\s*[0-9]+`)

	content = blockComments.ReplaceAll(content, nil)
	depth := 0
	var messages []dashboardProtoScope
	var active *dashboardProtoOneOf
	for _, originalLine := range strings.Split(string(content), "\n") {
		line := strings.SplitN(originalLine, "//", 2)[0]
		structuralLine := quotedStrings.ReplaceAllString(line, `""`)

		if match := messagePattern.FindStringSubmatch(structuralLine); len(match) == 2 {
			messages = append(messages, dashboardProtoScope{name: match[1], depth: depth + 1})
		}
		if match := oneOfPattern.FindStringSubmatch(structuralLine); len(match) == 2 {
			owner := make([]string, 0, len(messages))
			for _, message := range messages {
				owner = append(owner, message.name)
			}
			active = &dashboardProtoOneOf{
				key:   relativePath + "#" + strings.Join(owner, ".") + "." + match[1],
				depth: depth + 1,
			}
		}
		if active != nil && depth == active.depth {
			if match := fieldPattern.FindStringSubmatch(structuralLine); len(match) == 2 {
				active.branches = append(active.branches, match[1])
			}
		}

		depth += strings.Count(structuralLine, "{") - strings.Count(structuralLine, "}")
		if active != nil && depth < active.depth {
			result[active.key] = active.branches
			active = nil
		}
		for len(messages) > 0 && depth < messages[len(messages)-1].depth {
			messages = messages[:len(messages)-1]
		}
	}
}

func assertDashboardProtoBranch(t *testing.T, inventory map[string][]string, source string) {
	t.Helper()
	separator := strings.LastIndex(source, ".")
	if separator == -1 {
		t.Fatalf("invalid protobuf branch source %q", source)
	}
	oneOfSource, branch := source[:separator], source[separator+1:]
	branches, ok := inventory[oneOfSource]
	if !ok {
		t.Fatalf("protobuf oneof %s is absent", oneOfSource)
	}
	for _, candidate := range branches {
		if candidate == branch {
			return
		}
	}
	t.Fatalf("protobuf branch %s is absent from %s", branch, oneOfSource)
}

func snakeToLowerCamel(value string) string {
	parts := strings.Split(value, "_")
	for i := 1; i < len(parts); i++ {
		if parts[i] != "" {
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	return strings.Join(parts, "")
}
