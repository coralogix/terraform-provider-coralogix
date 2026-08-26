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
	"testing"

	dashboardservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_service"
	dashboardwidgets "github.com/coralogix/terraform-provider-coralogix/internal/provider/dashboards/dashboard_widgets"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestGaugeDisplayFieldsRoundTrip(t *testing.T) {
	ctx := t.Context()

	model := &dashboardwidgets.GaugeModel{
		Query:      &dashboardwidgets.GaugeQueryModel{Metrics: gaugeMetricsQueryForTest()},
		Thresholds: types.ListNull(types.ObjectType{AttrTypes: gaugeThresholdModelAttr()}),
		ArcDisplay: &dashboardwidgets.ArcDisplayModel{
			ThresholdArc: types.BoolValue(false),
			ValueArc:     types.BoolValue(true),
		},
		DecimalPrecision: types.BoolValue(true),
		CustomUnit:       types.StringValue("errors"),
		Legend:           &dashboardwidgets.LegendModel{IsVisible: types.BoolValue(true), Columns: types.ListNull(types.StringType)},
		LegendBy:         types.StringValue("thresholds"),
		ShowMinMax:       types.BoolValue(true),
	}

	expanded, diags := expandGauge(ctx, model)
	if diags.HasError() {
		t.Fatalf("expandGauge() diagnostics = %v", diags)
	}
	if got := expanded.Gauge.GetLegendBy(); got != dashboardservice.LEGENDBY_LEGEND_BY_THRESHOLDS {
		t.Fatalf("expanded LegendBy = %v, want THRESHOLDS", got)
	}

	flattened, diags := flattenGauge(ctx, expanded.Gauge)
	if diags.HasError() {
		t.Fatalf("flattenGauge() diagnostics = %v", diags)
	}
	if flattened.Gauge.ArcDisplay == nil ||
		!flattened.Gauge.ArcDisplay.ValueArc.Equal(model.ArcDisplay.ValueArc) ||
		!flattened.Gauge.ArcDisplay.ThresholdArc.Equal(model.ArcDisplay.ThresholdArc) {
		t.Fatalf("round-tripped arc_display = %+v, want %+v", flattened.Gauge.ArcDisplay, model.ArcDisplay)
	}
	assertEqualValues(t, map[string][2]attr.Value{
		"decimal_precision": {flattened.Gauge.DecimalPrecision, model.DecimalPrecision},
		"custom_unit":       {flattened.Gauge.CustomUnit, model.CustomUnit},
		"legend_by":         {flattened.Gauge.LegendBy, model.LegendBy},
		"show_min_max":      {flattened.Gauge.ShowMinMax, model.ShowMinMax},
	})
	if flattened.Gauge.Legend == nil || !flattened.Gauge.Legend.IsVisible.ValueBool() {
		t.Fatalf("round-tripped legend = %v, want is_visible true", flattened.Gauge.Legend)
	}
}

func TestGaugeDisplayFieldsStayNullWhenUnset(t *testing.T) {
	ctx := t.Context()

	expanded, diags := expandGauge(ctx, &dashboardwidgets.GaugeModel{
		Query:      &dashboardwidgets.GaugeQueryModel{Metrics: gaugeMetricsQueryForTest()},
		Thresholds: types.ListNull(types.ObjectType{AttrTypes: gaugeThresholdModelAttr()}),
	})
	if diags.HasError() {
		t.Fatalf("expandGauge() diagnostics = %v", diags)
	}
	if expanded.Gauge.ArcDisplay != nil || expanded.Gauge.DecimalPrecision != nil ||
		expanded.Gauge.CustomUnit != nil || expanded.Gauge.Legend != nil || expanded.Gauge.ShowMinMax != nil {
		t.Fatalf("expanded gauge carries unset display fields: %+v", expanded.Gauge)
	}

	flattened, diags := flattenGauge(ctx, &dashboardservice.WidgetsGauge{})
	if diags.HasError() {
		t.Fatalf("flattenGauge() diagnostics = %v", diags)
	}
	if flattened.Gauge.ArcDisplay != nil {
		t.Fatalf("flattened arc_display = %+v, want nil when the API omits the field", flattened.Gauge.ArcDisplay)
	}
	if !flattened.Gauge.CustomUnit.IsNull() || !flattened.Gauge.ShowMinMax.IsNull() {
		t.Fatalf("flattened gauge sets custom_unit or show_min_max, want both null: %+v", flattened.Gauge)
	}
}

func TestGaugeQueryGroupingFieldsRoundTrip(t *testing.T) {
	ctx := t.Context()

	observationField := types.ObjectValueMust(dashboardwidgets.ObservationFieldAttr(), map[string]attr.Value{
		"keypath": types.ListValueMust(types.StringType, []attr.Value{types.StringValue("log.level")}),
		"scope":   types.StringValue("user_data"),
	})
	logs := &dashboardwidgets.GaugeQueryLogsModel{
		Filters: types.ListNull(types.ObjectType{AttrTypes: dashboardwidgets.LogsFilterModelAttr()}),
		GroupBy: types.ListValueMust(dashboardwidgets.ObservationFieldsObject(), []attr.Value{observationField}),
	}

	expandedLogs, diags := expandGaugeQueryLogs(ctx, logs)
	if diags.HasError() {
		t.Fatalf("expandGaugeQueryLogs() diagnostics = %v", diags)
	}
	if got := len(expandedLogs.GetGroupBy()); got != 1 {
		t.Fatalf("expanded logs GroupBy length = %d, want 1", got)
	}
	flattenedLogs, diags := flattenGaugeQueryLogs(ctx, expandedLogs)
	if diags.HasError() {
		t.Fatalf("flattenGaugeQueryLogs() diagnostics = %v", diags)
	}
	if !flattenedLogs.Logs.GroupBy.Equal(logs.GroupBy) {
		t.Fatalf("round-tripped logs group_by = %v, want %v", flattenedLogs.Logs.GroupBy, logs.GroupBy)
	}

	spanField := types.ObjectValueMust(dashboardwidgets.SpanObservationFieldAttr(), map[string]attr.Value{
		"keypath":       types.ListValueMust(types.StringType, []attr.Value{types.StringValue("service"), types.StringValue("name")}),
		"scope":         types.StringValue("metadata"),
		"relation_type": types.StringValue("unspecified"),
	})
	spans := &dashboardwidgets.GaugeQuerySpansModel{
		Filters:  types.ListNull(types.ObjectType{AttrTypes: dashboardwidgets.SpansFilterModelAttr()}),
		GroupBy:  types.ListNull(types.ObjectType{AttrTypes: dashboardwidgets.SpansFieldModelAttr()}),
		GroupBys: types.ListValueMust(types.ObjectType{AttrTypes: dashboardwidgets.SpanObservationFieldAttr()}, []attr.Value{spanField}),
	}

	expandedSpans, diags := expandGaugeQuerySpans(ctx, spans)
	if diags.HasError() {
		t.Fatalf("expandGaugeQuerySpans() diagnostics = %v", diags)
	}
	if got := len(expandedSpans.GetGroupBys()); got != 1 {
		t.Fatalf("expanded spans GroupBys length = %d, want 1", got)
	}
	flattenedSpans, diags := flattenGaugeQuerySpans(ctx, expandedSpans)
	if diags.HasError() {
		t.Fatalf("flattenGaugeQuerySpans() diagnostics = %v", diags)
	}
	if !flattenedSpans.Spans.GroupBys.Equal(spans.GroupBys) {
		t.Fatalf("round-tripped spans group_bys = %v, want %v", flattenedSpans.Spans.GroupBys, spans.GroupBys)
	}
}

func TestGaugeMetricsQueryEditorFieldsRoundTrip(t *testing.T) {
	ctx := t.Context()

	metrics := gaugeMetricsQueryForTest()
	metrics.EditorMode = types.StringValue("builder")
	metrics.PromqlQueryType = types.StringValue("instant")

	expanded, diags := expandGaugeQueryMetrics(ctx, metrics)
	if diags.HasError() {
		t.Fatalf("expandGaugeQueryMetrics() diagnostics = %v", diags)
	}
	flattened, diags := flattenGaugeQueryMetrics(ctx, expanded)
	if diags.HasError() {
		t.Fatalf("flattenGaugeQueryMetrics() diagnostics = %v", diags)
	}
	assertEqualValues(t, map[string][2]attr.Value{
		"editor_mode":       {flattened.Metrics.EditorMode, metrics.EditorMode},
		"promql_query_type": {flattened.Metrics.PromqlQueryType, metrics.PromqlQueryType},
	})
}

func gaugeMetricsQueryForTest() *dashboardwidgets.GaugeQueryMetricsModel {
	return &dashboardwidgets.GaugeQueryMetricsModel{
		PromqlQuery: types.StringValue("vector(1)"),
		Filters:     types.ListNull(types.ObjectType{AttrTypes: dashboardwidgets.MetricsFilterModelAttr()}),
	}
}
