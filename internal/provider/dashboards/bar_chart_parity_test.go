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
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

func TestBarChartDisplayFieldsRoundTrip(t *testing.T) {
	ctx := t.Context()

	model := &dashboardwidgets.BarChartModel{
		XAxis:            &dashboardwidgets.BarChartXAxisModel{Value: &dashboardwidgets.BarChartXAxisValueModel{}},
		BarValueDisplay:  types.StringValue("inside"),
		CustomUnit:       types.StringValue("requests/s"),
		Decimal:          types.NumberValue(big.NewFloat(2)),
		DecimalPrecision: types.BoolValue(true),
		Legend:           &dashboardwidgets.LegendModel{IsVisible: types.BoolValue(true), Columns: types.ListNull(types.StringType)},
		XAxisTimeFormat:  types.StringValue("hh_mm"),
		YAxisMax:         dashboardwidgets.Float32Value{Float64Value: basetypes.NewFloat64Value(90)},
		YAxisMin:         dashboardwidgets.Float32Value{Float64Value: basetypes.NewFloat64Value(-1)},
	}

	expanded, diags := expandBarChart(ctx, model)
	if diags.HasError() {
		t.Fatalf("expandBarChart() diagnostics = %v", diags)
	}
	if got := expanded.BarChart.GetBarValueDisplay(); got != dashboardservice.WIDGETSBARVALUEDISPLAY_BAR_VALUE_DISPLAY_INSIDE {
		t.Fatalf("expanded BarValueDisplay = %v, want INSIDE", got)
	}
	if got := expanded.BarChart.GetXAxisTimeFormat(); got != dashboardservice.XAXISTIMEFORMAT_X_AXIS_TIME_FORMAT_HH_MM {
		t.Fatalf("expanded XAxisTimeFormat = %v, want HH_MM", got)
	}
	if expanded.BarChart.Legend == nil {
		t.Fatal("expanded Legend = nil, want the legend to reach the API")
	}

	flattened, diags := flattenBarChart(ctx, expanded.BarChart)
	if diags.HasError() {
		t.Fatalf("flattenBarChart() diagnostics = %v", diags)
	}
	assertEqualValues(t, map[string][2]attr.Value{
		"bar_value_display":  {flattened.BarChart.BarValueDisplay, model.BarValueDisplay},
		"custom_unit":        {flattened.BarChart.CustomUnit, model.CustomUnit},
		"decimal":            {flattened.BarChart.Decimal, model.Decimal},
		"decimal_precision":  {flattened.BarChart.DecimalPrecision, model.DecimalPrecision},
		"x_axis_time_format": {flattened.BarChart.XAxisTimeFormat, model.XAxisTimeFormat},
		"y_axis_max":         {flattened.BarChart.YAxisMax, model.YAxisMax},
		"y_axis_min":         {flattened.BarChart.YAxisMin, model.YAxisMin},
	})
	if flattened.BarChart.Legend == nil || !flattened.BarChart.Legend.IsVisible.ValueBool() {
		t.Fatalf("round-tripped legend = %v, want is_visible true", flattened.BarChart.Legend)
	}
}

func TestHorizontalBarChartDisplayFieldsRoundTrip(t *testing.T) {
	ctx := t.Context()

	model := &dashboardwidgets.HorizontalBarChartModel{
		CustomUnit:       types.StringValue("errors"),
		Decimal:          types.NumberValue(big.NewFloat(4)),
		DecimalPrecision: types.BoolValue(false),
		Legend:           &dashboardwidgets.LegendModel{IsVisible: types.BoolValue(false), Columns: types.ListNull(types.StringType)},
		YAxisMax:         dashboardwidgets.Float32Value{Float64Value: basetypes.NewFloat64Value(50)},
		YAxisMin:         dashboardwidgets.Float32Value{Float64Value: basetypes.NewFloat64Value(0)},
	}

	expanded, diags := expandHorizontalBarChart(ctx, model)
	if diags.HasError() {
		t.Fatalf("expandHorizontalBarChart() diagnostics = %v", diags)
	}

	flattened, diags := flattenHorizontalBarChart(ctx, expanded.HorizontalBarChart)
	if diags.HasError() {
		t.Fatalf("flattenHorizontalBarChart() diagnostics = %v", diags)
	}
	assertEqualValues(t, map[string][2]attr.Value{
		"custom_unit":       {flattened.HorizontalBarChart.CustomUnit, model.CustomUnit},
		"decimal":           {flattened.HorizontalBarChart.Decimal, model.Decimal},
		"decimal_precision": {flattened.HorizontalBarChart.DecimalPrecision, model.DecimalPrecision},
		"y_axis_max":        {flattened.HorizontalBarChart.YAxisMax, model.YAxisMax},
		"y_axis_min":        {flattened.HorizontalBarChart.YAxisMin, model.YAxisMin},
	})
	if flattened.HorizontalBarChart.Legend == nil || flattened.HorizontalBarChart.Legend.IsVisible.ValueBool() {
		t.Fatalf("round-tripped legend = %v, want is_visible false", flattened.HorizontalBarChart.Legend)
	}
}

// Removing an optional display field from the configuration must clear it, so an
// unset value has to reach the API as an absent field.
func TestBarChartDisplayFieldsStayNullWhenUnset(t *testing.T) {
	ctx := t.Context()

	expanded, diags := expandBarChart(ctx, &dashboardwidgets.BarChartModel{
		XAxis: &dashboardwidgets.BarChartXAxisModel{Value: &dashboardwidgets.BarChartXAxisValueModel{}},
	})
	if diags.HasError() {
		t.Fatalf("expandBarChart() diagnostics = %v", diags)
	}
	if expanded.BarChart.CustomUnit != nil {
		t.Fatalf("expanded CustomUnit = %q, want nil", *expanded.BarChart.CustomUnit)
	}
	if expanded.BarChart.Decimal != nil {
		t.Fatalf("expanded Decimal = %d, want nil", *expanded.BarChart.Decimal)
	}
	if expanded.BarChart.DecimalPrecision != nil {
		t.Fatalf("expanded DecimalPrecision = %t, want nil", *expanded.BarChart.DecimalPrecision)
	}
	if expanded.BarChart.YAxisMax != nil || expanded.BarChart.YAxisMin != nil {
		t.Fatal("expanded y-axis bounds are set, want nil")
	}
	if expanded.BarChart.Legend != nil {
		t.Fatal("expanded Legend is set, want nil")
	}

	flattened, diags := flattenBarChart(ctx, &dashboardservice.BarChart{})
	if diags.HasError() {
		t.Fatalf("flattenBarChart() diagnostics = %v", diags)
	}
	for name, value := range map[string]attr.Value{
		"custom_unit":       flattened.BarChart.CustomUnit,
		"decimal":           flattened.BarChart.Decimal,
		"decimal_precision": flattened.BarChart.DecimalPrecision,
		"y_axis_max":        flattened.BarChart.YAxisMax,
		"y_axis_min":        flattened.BarChart.YAxisMin,
	} {
		if !value.IsNull() {
			t.Fatalf("flattened %s = %v, want null when the API omits the field", name, value)
		}
	}
}

func TestBarChartMetricsQueryFieldsRoundTrip(t *testing.T) {
	ctx := t.Context()

	metrics := &dashboardwidgets.BarChartQueryMetricsModel{
		PromqlQuery:      types.StringValue("up"),
		Filters:          types.ListNull(types.ObjectType{AttrTypes: dashboardwidgets.MetricsFilterModelAttr()}),
		GroupNames:       types.ListNull(types.StringType),
		Aggregation:      types.StringValue("avg"),
		EditorMode:       types.StringValue("text"),
		PromqlQueryType:  types.StringValue("instant"),
		StackedGroupName: types.StringNull(),
	}
	metricsObject, diags := types.ObjectValueFrom(ctx, barChartMetricsQueryAttr(), metrics)
	if diags.HasError() {
		t.Fatalf("building the metrics object failed: %v", diags)
	}

	expanded, diags := expandBarChartMetricsQuery(ctx, metricsObject)
	if diags.HasError() {
		t.Fatalf("expandBarChartMetricsQuery() diagnostics = %v", diags)
	}
	if got := expanded.GetAggregation(); got != dashboardservice.COMMONAGGREGATION_AGGREGATION_AVG {
		t.Fatalf("expanded Aggregation = %v, want AVG", got)
	}
	if got := expanded.GetEditorMode(); got != dashboardservice.METRICSQUERYEDITORMODE_METRICS_QUERY_EDITOR_MODE_TEXT {
		t.Fatalf("expanded EditorMode = %v, want TEXT", got)
	}
	if got := expanded.GetPromqlQueryType(); got != dashboardservice.PROMQLQUERYTYPE_PROM_QL_QUERY_TYPE_INSTANT {
		t.Fatalf("expanded PromqlQueryType = %v, want INSTANT", got)
	}

	flattened, diags := flattenBarChartQueryMetrics(ctx, expanded)
	if diags.HasError() {
		t.Fatalf("flattenBarChartQueryMetrics() diagnostics = %v", diags)
	}
	var roundTripped dashboardwidgets.BarChartQueryMetricsModel
	if diags := flattened.Metrics.As(ctx, &roundTripped, basetypes.ObjectAsOptions{}); diags.HasError() {
		t.Fatalf("reading the flattened metrics object failed: %v", diags)
	}
	assertEqualValues(t, map[string][2]attr.Value{
		"aggregation":       {roundTripped.Aggregation, metrics.Aggregation},
		"editor_mode":       {roundTripped.EditorMode, metrics.EditorMode},
		"promql_query_type": {roundTripped.PromqlQueryType, metrics.PromqlQueryType},
	})
}

func TestBarChartSpansQueryObservationFieldsRoundTrip(t *testing.T) {
	ctx := t.Context()

	spanField := types.ObjectValueMust(dashboardwidgets.SpanObservationFieldAttr(), map[string]attr.Value{
		"keypath":       types.ListValueMust(types.StringType, []attr.Value{types.StringValue("service"), types.StringValue("name")}),
		"scope":         types.StringValue("metadata"),
		"relation_type": types.StringValue("unspecified"),
	})
	spans := &dashboardwidgets.BarChartQuerySpansModel{
		LuceneQuery:           types.StringNull(),
		Filters:               types.ListNull(types.ObjectType{AttrTypes: dashboardwidgets.SpansFilterModelAttr()}),
		GroupNames:            types.ListNull(types.ObjectType{AttrTypes: dashboardwidgets.SpansFieldModelAttr()}),
		GroupNamesFields:      types.ListValueMust(types.ObjectType{AttrTypes: dashboardwidgets.SpanObservationFieldAttr()}, []attr.Value{spanField}),
		StackedGroupNameField: spanField,
	}
	spansObject, diags := types.ObjectValueFrom(ctx, barChartSpansQueryAttr(), spans)
	if diags.HasError() {
		t.Fatalf("building the spans object failed: %v", diags)
	}

	expanded, diags := expandBarChartSpansQuery(ctx, spansObject)
	if diags.HasError() {
		t.Fatalf("expandBarChartSpansQuery() diagnostics = %v", diags)
	}
	if got := len(expanded.GetGroupNamesFields()); got != 1 {
		t.Fatalf("expanded GroupNamesFields length = %d, want 1", got)
	}
	if expanded.StackedGroupNameField == nil {
		t.Fatal("expanded StackedGroupNameField = nil, want the field to reach the API")
	}

	flattened, diags := flattenBarChartQuerySpans(ctx, expanded)
	if diags.HasError() {
		t.Fatalf("flattenBarChartQuerySpans() diagnostics = %v", diags)
	}
	var roundTripped dashboardwidgets.BarChartQuerySpansModel
	if diags := flattened.Spans.As(ctx, &roundTripped, basetypes.ObjectAsOptions{}); diags.HasError() {
		t.Fatalf("reading the flattened spans object failed: %v", diags)
	}
	assertEqualValues(t, map[string][2]attr.Value{
		"group_names_fields":       {roundTripped.GroupNamesFields, spans.GroupNamesFields},
		"stacked_group_name_field": {roundTripped.StackedGroupNameField, spans.StackedGroupNameField},
	})
}

func assertEqualValues(t *testing.T, pairs map[string][2]attr.Value) {
	t.Helper()
	for name, pair := range pairs {
		if !pair[0].Equal(pair[1]) {
			t.Fatalf("round-tripped %s = %v, want %v", name, pair[0], pair[1])
		}
	}
}
