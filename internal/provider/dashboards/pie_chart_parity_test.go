// Copyright 2026 Coralogix Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dashboards

import (
	"math/big"
	"testing"

	dashboardservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_service"
	dashboardwidgets "github.com/coralogix/terraform-provider-coralogix/internal/provider/dashboards/dashboard_widgets"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestPieChartDisplayFieldsRoundTrip(t *testing.T) {
	ctx := t.Context()

	model := &dashboardwidgets.PieChartModel{
		CustomUnit:       types.StringValue("slices"),
		Decimal:          types.NumberValue(big.NewFloat(1)),
		DecimalPrecision: types.BoolValue(true),
		ShowTotal:        types.BoolValue(true),
		Legend:           &dashboardwidgets.LegendModel{IsVisible: types.BoolValue(false), Columns: types.ListNull(types.StringType)},
	}

	expanded, diags := expandPieChart(ctx, model)
	if diags.HasError() {
		t.Fatalf("expandPieChart() diagnostics = %v", diags)
	}

	flattened, diags := flattenPieChart(ctx, expanded.PieChart)
	if diags.HasError() {
		t.Fatalf("flattenPieChart() diagnostics = %v", diags)
	}
	assertEqualValues(t, map[string][2]attr.Value{
		"custom_unit":       {flattened.PieChart.CustomUnit, model.CustomUnit},
		"decimal":           {flattened.PieChart.Decimal, model.Decimal},
		"decimal_precision": {flattened.PieChart.DecimalPrecision, model.DecimalPrecision},
		"show_total":        {flattened.PieChart.ShowTotal, model.ShowTotal},
	})
	// legend and show_legend are stored independently by the API, verified against
	// a live environment, so the legend block needs no conflict handling.
	if flattened.PieChart.Legend == nil || flattened.PieChart.Legend.IsVisible.ValueBool() {
		t.Fatalf("round-tripped legend = %v, want is_visible false", flattened.PieChart.Legend)
	}
}

func TestPieChartDisplayFieldsStayNullWhenUnset(t *testing.T) {
	ctx := t.Context()

	expanded, diags := expandPieChart(ctx, &dashboardwidgets.PieChartModel{})
	if diags.HasError() {
		t.Fatalf("expandPieChart() diagnostics = %v", diags)
	}
	if expanded.PieChart.CustomUnit != nil || expanded.PieChart.Decimal != nil ||
		expanded.PieChart.DecimalPrecision != nil || expanded.PieChart.ShowTotal != nil ||
		expanded.PieChart.Legend != nil {
		t.Fatalf("expanded pie chart carries unset display fields: %+v", expanded.PieChart)
	}

	flattened, diags := flattenPieChart(ctx, &dashboardservice.WidgetsPieChart{})
	if diags.HasError() {
		t.Fatalf("flattenPieChart() diagnostics = %v", diags)
	}
	for name, value := range map[string]attr.Value{
		"custom_unit":       flattened.PieChart.CustomUnit,
		"decimal":           flattened.PieChart.Decimal,
		"decimal_precision": flattened.PieChart.DecimalPrecision,
		"show_total":        flattened.PieChart.ShowTotal,
	} {
		if !value.IsNull() {
			t.Fatalf("flattened %s = %v, want null when the API omits the field", name, value)
		}
	}
}

func TestPieChartQueryFieldsRoundTrip(t *testing.T) {
	ctx := t.Context()

	metrics := &dashboardwidgets.PieChartQueryMetricsModel{
		PromqlQuery:     types.StringValue("up"),
		Filters:         types.ListNull(types.ObjectType{AttrTypes: dashboardwidgets.MetricsFilterModelAttr()}),
		GroupNames:      types.ListNull(types.StringType),
		Aggregation:     types.StringValue("sum"),
		EditorMode:      types.StringValue("builder"),
		PromqlQueryType: types.StringValue("range"),
	}

	expandedMetrics, diags := expandPieChartMetricsQuery(ctx, metrics)
	if diags.HasError() {
		t.Fatalf("expandPieChartMetricsQuery() diagnostics = %v", diags)
	}
	flattenedMetrics, diags := flattenPieChartQueryMetrics(ctx, expandedMetrics)
	if diags.HasError() {
		t.Fatalf("flattenPieChartQueryMetrics() diagnostics = %v", diags)
	}
	assertEqualValues(t, map[string][2]attr.Value{
		"aggregation":       {flattenedMetrics.Metrics.Aggregation, metrics.Aggregation},
		"editor_mode":       {flattenedMetrics.Metrics.EditorMode, metrics.EditorMode},
		"promql_query_type": {flattenedMetrics.Metrics.PromqlQueryType, metrics.PromqlQueryType},
	})

	spanField := types.ObjectValueMust(dashboardwidgets.SpanObservationFieldAttr(), map[string]attr.Value{
		"keypath":       types.ListValueMust(types.StringType, []attr.Value{types.StringValue("service"), types.StringValue("name")}),
		"scope":         types.StringValue("metadata"),
		"relation_type": types.StringValue("unspecified"),
	})
	spans := &dashboardwidgets.PieChartQuerySpansModel{
		Filters:               types.ListNull(types.ObjectType{AttrTypes: dashboardwidgets.SpansObservationFilterModelAttr()}),
		GroupNames:            types.ListNull(types.ObjectType{AttrTypes: dashboardwidgets.SpansFieldModelAttr()}),
		GroupNamesFields:      types.ListValueMust(types.ObjectType{AttrTypes: dashboardwidgets.SpanObservationFieldAttr()}, []attr.Value{spanField}),
		StackedGroupNameField: spanField,
	}

	expandedSpans, diags := expandPieChartSpansQuery(ctx, spans)
	if diags.HasError() {
		t.Fatalf("expandPieChartSpansQuery() diagnostics = %v", diags)
	}
	flattenedSpans, diags := flattenPieChartQuerySpans(ctx, expandedSpans)
	if diags.HasError() {
		t.Fatalf("flattenPieChartQuerySpans() diagnostics = %v", diags)
	}
	assertEqualValues(t, map[string][2]attr.Value{
		"group_names_fields":       {flattenedSpans.Spans.GroupNamesFields, spans.GroupNamesFields},
		"stacked_group_name_field": {flattenedSpans.Spans.StackedGroupNameField, spans.StackedGroupNameField},
	})
}
