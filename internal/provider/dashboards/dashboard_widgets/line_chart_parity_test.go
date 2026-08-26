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

package dashboard_widgets

import (
	"math/big"
	"testing"

	dashboardservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_service"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

func TestLineChartDisplayFieldsRoundTrip(t *testing.T) {
	ctx := t.Context()

	model := &LineChartModel{
		QueryDefinitions: types.ListNull(types.ObjectType{AttrTypes: lineChartQueryDefinitionModelAttr()}),
		StackedLine:      types.StringValue("absolute"),
		ConnectNulls:     types.BoolValue(true),
		UseDataTimeRange: types.BoolValue(true),
		XAxisTimeFormat:  types.StringValue("dd_mm_hh_mm"),
	}

	expanded, diags := ExpandLineChart(ctx, model)
	if diags.HasError() {
		t.Fatalf("ExpandLineChart() diagnostics = %v", diags)
	}
	if got := expanded.LineChart.ConnectNulls; got == nil || !*got {
		t.Fatalf("expanded ConnectNulls = %v, want true", got)
	}
	if got := expanded.LineChart.UseDataTimeRange; got == nil || !*got {
		t.Fatalf("expanded UseDataTimeRange = %v, want true", got)
	}
	if got := expanded.LineChart.GetXAxisTimeFormat(); got != dashboardservice.XAXISTIMEFORMAT_X_AXIS_TIME_FORMAT_DD_MM_HH_MM {
		t.Fatalf("expanded XAxisTimeFormat = %v, want DD_MM_HH_MM", got)
	}

	flattened, diags := FlattenLineChart(ctx, expanded.LineChart)
	if diags.HasError() {
		t.Fatalf("FlattenLineChart() diagnostics = %v", diags)
	}
	if !flattened.LineChart.ConnectNulls.Equal(model.ConnectNulls) {
		t.Fatalf("round-tripped connect_nulls = %v, want %v", flattened.LineChart.ConnectNulls, model.ConnectNulls)
	}
	if !flattened.LineChart.UseDataTimeRange.Equal(model.UseDataTimeRange) {
		t.Fatalf("round-tripped use_data_time_range = %v, want %v", flattened.LineChart.UseDataTimeRange, model.UseDataTimeRange)
	}
	if !flattened.LineChart.XAxisTimeFormat.Equal(model.XAxisTimeFormat) {
		t.Fatalf("round-tripped x_axis_time_format = %v, want %v", flattened.LineChart.XAxisTimeFormat, model.XAxisTimeFormat)
	}
}

// An unset connect_nulls or use_data_time_range must reach the API as an absent
// field, so removing it from the configuration produces an empty follow-up plan.
func TestLineChartDisplayFieldsStayNullWhenUnset(t *testing.T) {
	ctx := t.Context()

	expanded, diags := ExpandLineChart(ctx, &LineChartModel{
		QueryDefinitions: types.ListNull(types.ObjectType{AttrTypes: lineChartQueryDefinitionModelAttr()}),
	})
	if diags.HasError() {
		t.Fatalf("ExpandLineChart() diagnostics = %v", diags)
	}
	if expanded.LineChart.ConnectNulls != nil {
		t.Fatalf("expanded ConnectNulls = %t, want nil", *expanded.LineChart.ConnectNulls)
	}
	if expanded.LineChart.UseDataTimeRange != nil {
		t.Fatalf("expanded UseDataTimeRange = %t, want nil", *expanded.LineChart.UseDataTimeRange)
	}

	flattened, diags := FlattenLineChart(ctx, &dashboardservice.LineChart{})
	if diags.HasError() {
		t.Fatalf("FlattenLineChart() diagnostics = %v", diags)
	}
	if !flattened.LineChart.ConnectNulls.IsNull() {
		t.Fatalf("flattened connect_nulls = %v, want null", flattened.LineChart.ConnectNulls)
	}
	if !flattened.LineChart.UseDataTimeRange.IsNull() {
		t.Fatalf("flattened use_data_time_range = %v, want null", flattened.LineChart.UseDataTimeRange)
	}
}

func TestLineChartQueryDefinitionDisplayFieldsRoundTrip(t *testing.T) {
	ctx := t.Context()

	model := &LineChartQueryDefinitionModel{
		ID:               types.StringValue("11111111-1111-1111-1111-111111111111"),
		Query:            lineChartMetricsQueryForTest(),
		Resolution:       types.ObjectNull(lineChartQueryResolutionModelAttr()),
		CustomUnit:       types.StringValue("requests/s"),
		Decimal:          types.NumberValue(big.NewFloat(3)),
		DecimalPrecision: types.BoolValue(true),
		YAxisMax:         Float32Value{Float64Value: basetypes.NewFloat64Value(120)},
		YAxisMin:         Float32Value{Float64Value: basetypes.NewFloat64Value(-5)},
	}

	expanded, diags := expandLineChartQueryDefinition(ctx, model)
	if diags.HasError() {
		t.Fatalf("expandLineChartQueryDefinition() diagnostics = %v", diags)
	}
	if got := expanded.GetCustomUnit(); got != "requests/s" {
		t.Fatalf("expanded CustomUnit = %q, want %q", got, "requests/s")
	}
	if got := expanded.GetDecimal(); got != 3 {
		t.Fatalf("expanded Decimal = %d, want 3", got)
	}
	if got := expanded.GetDecimalPrecision(); !got {
		t.Fatal("expanded DecimalPrecision = false, want true")
	}
	if got := expanded.GetYAxisMax(); got != 120 {
		t.Fatalf("expanded YAxisMax = %v, want 120", got)
	}
	if got := expanded.GetYAxisMin(); got != -5 {
		t.Fatalf("expanded YAxisMin = %v, want -5", got)
	}

	flattened, diags := flattenLineChartQueryDefinition(ctx, expanded)
	if diags.HasError() {
		t.Fatalf("flattenLineChartQueryDefinition() diagnostics = %v", diags)
	}
	for name, pair := range map[string][2]attr.Value{
		"custom_unit":       {flattened.CustomUnit, model.CustomUnit},
		"decimal":           {flattened.Decimal, model.Decimal},
		"decimal_precision": {flattened.DecimalPrecision, model.DecimalPrecision},
		"y_axis_max":        {flattened.YAxisMax, model.YAxisMax},
		"y_axis_min":        {flattened.YAxisMin, model.YAxisMin},
	} {
		if !pair[0].Equal(pair[1]) {
			t.Fatalf("round-tripped %s = %v, want %v", name, pair[0], pair[1])
		}
	}
}

func TestLineChartMetricsQueryEditorFieldsRoundTrip(t *testing.T) {
	ctx := t.Context()

	metrics := &LineChartQueryMetricsModel{
		PromqlQuery:     types.StringValue("http_requests_total"),
		Filters:         types.ListNull(types.ObjectType{AttrTypes: MetricsFilterModelAttr()}),
		EditorMode:      types.StringValue("builder"),
		SeriesLimitType: types.StringValue("by_point_count"),
	}

	expanded, diags := expandLineChartMetricsQuery(ctx, metrics)
	if diags.HasError() {
		t.Fatalf("expandLineChartMetricsQuery() diagnostics = %v", diags)
	}
	if got := expanded.GetEditorMode(); got != dashboardservice.METRICSQUERYEDITORMODE_METRICS_QUERY_EDITOR_MODE_BUILDER {
		t.Fatalf("expanded EditorMode = %v, want BUILDER", got)
	}
	if got := expanded.GetSeriesLimitType(); got != dashboardservice.METRICSSERIESLIMITTYPE_METRICS_SERIES_LIMIT_TYPE_BY_POINT_COUNT {
		t.Fatalf("expanded SeriesLimitType = %v, want BY_POINT_COUNT", got)
	}

	flattened, diags := flattenLineChartQueryMetrics(ctx, expanded)
	if diags.HasError() {
		t.Fatalf("flattenLineChartQueryMetrics() diagnostics = %v", diags)
	}
	if !flattened.Metrics.EditorMode.Equal(metrics.EditorMode) {
		t.Fatalf("round-tripped editor_mode = %v, want %v", flattened.Metrics.EditorMode, metrics.EditorMode)
	}
	if !flattened.Metrics.SeriesLimitType.Equal(metrics.SeriesLimitType) {
		t.Fatalf("round-tripped series_limit_type = %v, want %v", flattened.Metrics.SeriesLimitType, metrics.SeriesLimitType)
	}
}

func TestLineChartLogsQueryGroupBysRoundTrip(t *testing.T) {
	ctx := t.Context()

	logs := &LineChartQueryLogsModel{
		GroupBys: types.ListValueMust(ObservationFieldsObject(), []attr.Value{
			types.ObjectValueMust(ObservationFieldAttr(), map[string]attr.Value{
				"keypath": types.ListValueMust(types.StringType, []attr.Value{types.StringValue("log.level")}),
				"scope":   types.StringValue("user_data"),
			}),
		}),
		Aggregations: types.ListNull(types.ObjectType{AttrTypes: AggregationModelAttr()}),
		Filters:      types.ListNull(types.ObjectType{AttrTypes: LogsFilterModelAttr()}),
	}

	expanded, diags := expandLineChartLogsQuery(ctx, logs)
	if diags.HasError() {
		t.Fatalf("expandLineChartLogsQuery() diagnostics = %v", diags)
	}
	if got := len(expanded.GetGroupBys()); got != 1 {
		t.Fatalf("expanded GroupBys length = %d, want 1", got)
	}
	if got := expanded.GetGroupBys()[0].GetKeypath(); len(got) != 1 || got[0] != "log.level" {
		t.Fatalf("expanded GroupBys keypath = %v, want [log.level]", got)
	}

	flattened, diags := flattenLineChartQueryLogs(ctx, expanded)
	if diags.HasError() {
		t.Fatalf("flattenLineChartQueryLogs() diagnostics = %v", diags)
	}
	if !flattened.Logs.GroupBys.Equal(logs.GroupBys) {
		t.Fatalf("round-tripped group_bys = %v, want %v", flattened.Logs.GroupBys, logs.GroupBys)
	}
}

func TestLineChartSpansQueryGroupBysRoundTrip(t *testing.T) {
	ctx := t.Context()

	spans := &LineChartQuerySpansModel{
		GroupBy: types.ListNull(types.ObjectType{AttrTypes: SpansFieldModelAttr()}),
		GroupBys: types.ListValueMust(types.ObjectType{AttrTypes: SpanObservationFieldAttr()}, []attr.Value{
			types.ObjectValueMust(SpanObservationFieldAttr(), map[string]attr.Value{
				"keypath":       types.ListValueMust(types.StringType, []attr.Value{types.StringValue("service"), types.StringValue("name")}),
				"scope":         types.StringValue("metadata"),
				"relation_type": types.StringValue("unspecified"),
			}),
		}),
		Aggregations: types.ListNull(types.ObjectType{AttrTypes: SpansAggregationModelAttr()}),
		Filters:      types.ListNull(types.ObjectType{AttrTypes: SpansFilterModelAttr()}),
	}

	expanded, diags := expandLineChartSpansQuery(ctx, spans)
	if diags.HasError() {
		t.Fatalf("expandLineChartSpansQuery() diagnostics = %v", diags)
	}
	if got := len(expanded.GetGroupBys()); got != 1 {
		t.Fatalf("expanded GroupBys length = %d, want 1", got)
	}

	flattened, diags := flattenLineChartQuerySpans(ctx, expanded)
	if diags.HasError() {
		t.Fatalf("flattenLineChartQuerySpans() diagnostics = %v", diags)
	}
	if !flattened.Spans.GroupBys.Equal(spans.GroupBys) {
		t.Fatalf("round-tripped group_bys = %v, want %v", flattened.Spans.GroupBys, spans.GroupBys)
	}
}

func lineChartMetricsQueryForTest() *LineChartQueryModel {
	return &LineChartQueryModel{
		Metrics: &LineChartQueryMetricsModel{
			PromqlQuery: types.StringValue("http_requests_total"),
			Filters:     types.ListNull(types.ObjectType{AttrTypes: MetricsFilterModelAttr()}),
		},
	}
}
