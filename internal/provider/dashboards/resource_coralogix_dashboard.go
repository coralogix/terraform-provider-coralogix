// Copyright 2024 Coralogix Ltd.
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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"strings"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	dashboardschema "github.com/coralogix/terraform-provider-coralogix/internal/provider/dashboards/dashboard_schema"
	dashboardwidgets "github.com/coralogix/terraform-provider-coralogix/internal/provider/dashboards/dashboard_widgets"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	dashboardservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_service"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

var (
	_ resource.ResourceWithConfigure = &DashboardResource{}
	//_ resource.ResourceWithConfigValidators = &DashboardResource{}
	_ resource.ResourceWithImportState  = &DashboardResource{}
	_ resource.ResourceWithUpgradeState = &DashboardResource{}

	dashboardManualAnnotationOrientationToProto = map[string]dashboardservice.AnnotationOrientation{
		"vertical":   dashboardservice.ANNOTATIONORIENTATION_ANNOTATION_ORIENTATION_VERTICAL_UNSPECIFIED,
		"horizontal": dashboardservice.ANNOTATIONORIENTATION_ANNOTATION_ORIENTATION_HORIZONTAL,
	}
	dashboardManualAnnotationOrientationToSchema = utils.ReverseMap(dashboardManualAnnotationOrientationToProto)
)

type DashboardResourceModel struct {
	ID           types.String                     `tfsdk:"id"`
	Name         types.String                     `tfsdk:"name"`
	Description  types.String                     `tfsdk:"description"`
	Layout       types.Object                     `tfsdk:"layout"`    //DashboardLayoutModel
	Variables    types.List                       `tfsdk:"variables"` //DashboardVariableModel
	Filters      types.List                       `tfsdk:"filters"`   //DashboardFilterModel
	TimeFrame    *dashboardwidgets.TimeFrameModel `tfsdk:"time_frame"`
	Folder       types.Object                     `tfsdk:"folder"`       //DashboardFolderModel
	Annotations  types.List                       `tfsdk:"annotations"`  //DashboardAnnotationModel
	AutoRefresh  types.Object                     `tfsdk:"auto_refresh"` //DashboardAutoRefreshModel
	ContentJson  types.String                     `tfsdk:"content_json"`
	AccessPolicy types.String                     `tfsdk:"access_policy"`
}

type DashboardLayoutModel struct {
	Sections types.List `tfsdk:"sections"` //SectionModel
}

type SectionModel struct {
	ID      types.String         `tfsdk:"id"`
	Rows    types.List           `tfsdk:"rows"` //RowModel
	Options *SectionOptionsModel `tfsdk:"options"`
}

type SectionOptionsModel struct {
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Collapsed   types.Bool   `tfsdk:"collapsed"`
	Color       types.String `tfsdk:"color"`
}

type RowModel struct {
	ID      types.String `tfsdk:"id"`
	Height  types.Int64  `tfsdk:"height"`
	Widgets types.List   `tfsdk:"widgets"` //WidgetModel
}

type WidgetModel struct {
	ID          types.String                            `tfsdk:"id"`
	Title       types.String                            `tfsdk:"title"`
	Description types.String                            `tfsdk:"description"`
	Definition  *dashboardwidgets.WidgetDefinitionModel `tfsdk:"definition"`
	Width       types.Int64                             `tfsdk:"width"`
}

type DashboardVariableModel struct {
	Name        types.String                      `tfsdk:"name"`
	Definition  *DashboardVariableDefinitionModel `tfsdk:"definition"`
	DisplayName types.String                      `tfsdk:"display_name"`
}

type MetricMultiSelectSourceModel struct {
	MetricName types.String `tfsdk:"metric_name"`
	Label      types.String `tfsdk:"label"`
}

type DashboardVariableDefinitionModel struct {
	ConstantValue types.String              `tfsdk:"constant_value"`
	MultiSelect   *VariableMultiSelectModel `tfsdk:"multi_select"`
}

type VariableMultiSelectModel struct {
	SelectedValues       types.List                      `tfsdk:"selected_values"` //types.String
	ValuesOrderDirection types.String                    `tfsdk:"values_order_direction"`
	SelectionType        types.String                    `tfsdk:"selection_type"`
	Source               *VariableMultiSelectSourceModel `tfsdk:"source"`
}

type VariableMultiSelectSourceModel struct {
	LogsPath     types.String                      `tfsdk:"logs_path"`
	MetricLabel  *MetricMultiSelectSourceModel     `tfsdk:"metric_label"`
	ConstantList types.List                        `tfsdk:"constant_list"` //types.String
	SpanField    *dashboardwidgets.SpansFieldModel `tfsdk:"span_field"`
	Query        types.Object                      `tfsdk:"query"` //VariableMultiSelectQueryModel
}

type VariableMultiSelectQueryModel struct {
	Query               types.Object `tfsdk:"query"` //MultiSelectQueryModel
	RefreshStrategy     types.String `tfsdk:"refresh_strategy"`
	ValueDisplayOptions types.Object `tfsdk:"value_display_options"` //MultiSelectValueDisplayOptionsModel
}

type MultiSelectQueryModel struct {
	Logs    types.Object `tfsdk:"logs"`    //MultiSelectLogsQueryModel
	Metrics types.Object `tfsdk:"metrics"` //MultiSelectMetricsQueryModel
	Spans   types.Object `tfsdk:"spans"`   //MultiSelectSpansQueryModel
}

type MultiSelectLogsQueryModel struct {
	FieldName  types.Object `tfsdk:"field_name"`  //LogFieldNameModel
	FieldValue types.Object `tfsdk:"field_value"` //FieldValueModel
}

type LogFieldNameModel struct {
	LogRegex types.String `tfsdk:"log_regex"`
}

type SpanFieldNameModel struct {
	SpanRegex types.String `tfsdk:"span_regex"`
}

type FieldValueModel struct {
	ObservationField types.Object `tfsdk:"observation_field"` //dashboard_widgets.ObservationFieldModel
}

type MultiSelectMetricsQueryModel struct {
	MetricName types.Object `tfsdk:"metric_name"` //MetricAndLabelNameModel
	LabelName  types.Object `tfsdk:"label_name"`  //MetricAndLabelNameModel
	LabelValue types.Object `tfsdk:"label_value"` //LabelValueModel
}

type MetricAndLabelNameModel struct {
	MetricRegex types.String `tfsdk:"metric_regex"`
}

type LabelValueModel struct {
	MetricName   types.Object `tfsdk:"metric_name"`   //MetricLabelFilterOperatorSelectedValuesModel
	LabelName    types.Object `tfsdk:"label_name"`    //MetricLabelFilterOperatorSelectedValuesModel
	LabelFilters types.List   `tfsdk:"label_filters"` // MetricLabelFilterModel
}

type MetricLabelFilterModel struct {
	Metric   types.Object `tfsdk:"metric"`   //MetricLabelFilterOperatorSelectedValuesModel
	Label    types.Object `tfsdk:"label"`    //MetricLabelFilterOperatorSelectedValuesModel
	Operator types.Object `tfsdk:"operator"` //MetricLabelFilterOperatorModel
}

type MetricLabelFilterOperatorModel struct {
	Type           types.String `tfsdk:"type"`
	SelectedValues types.List   `tfsdk:"selected_values"` //MetricLabelFilterOperatorSelectedValuesModel
}

type MetricLabelFilterOperatorSelectedValuesModel struct {
	StringValue  types.String `tfsdk:"string_value"`
	VariableName types.String `tfsdk:"variable_name"`
}

type MultiSelectSpansQueryModel struct {
	FieldName  types.Object `tfsdk:"field_name"`  //SpanFieldNameModel
	FieldValue types.Object `tfsdk:"field_value"` //dashboard_widgets.SpansFieldModel
}

type MultiSelectValueDisplayOptionsModel struct {
	ValueRegex types.String `tfsdk:"value_regex"`
	LabelRegex types.String `tfsdk:"label_regex"`
}

type DashboardFilterModel struct {
	Source    *dashboardwidgets.DashboardFilterSourceModel `tfsdk:"source"`
	Enabled   types.Bool                                   `tfsdk:"enabled"`
	Collapsed types.Bool                                   `tfsdk:"collapsed"`
}

type DashboardFolderModel struct {
	ID   types.String `tfsdk:"id"`
	Path types.String `tfsdk:"path"`
}

type DashboardAnnotationModel struct {
	ID      types.String `tfsdk:"id"`
	Name    types.String `tfsdk:"name"`
	Enabled types.Bool   `tfsdk:"enabled"`
	Source  types.Object `tfsdk:"source"` //DashboardAnnotationSourceModel
}

type DashboardAnnotationSourceModel struct {
	Metrics types.Object `tfsdk:"metrics"` //DashboardAnnotationMetricSourceModel
	Spans   types.Object `tfsdk:"spans"`   //DashboardAnnotationSpansOrLogsSourceModel
	Logs    types.Object `tfsdk:"logs"`    //DashboardAnnotationSpansOrLogsSourceModel
	Manual  types.Object `tfsdk:"manual"`  //DashboardAnnotationManualSourceModel
}

type DashboardAnnotationManualSourceModel struct {
	Orientation     types.String `tfsdk:"orientation"`
	MessageTemplate types.String `tfsdk:"message_template"`
	Strategy        types.Object `tfsdk:"strategy"` //DashboardAnnotationManualStrategyModel
}

type DashboardAnnotationManualStrategyModel struct {
	Instant types.Object `tfsdk:"instant"` //DashboardAnnotationManualInstantStrategyModel
	Range   types.Object `tfsdk:"range"`   //DashboardAnnotationManualRangeStrategyModel
}

type DashboardAnnotationManualInstantStrategyModel struct {
	Value      types.Float64 `tfsdk:"value"`
	Unit       types.String  `tfsdk:"unit"`
	CustomUnit types.String  `tfsdk:"custom_unit"`
}

type DashboardAnnotationManualRangeStrategyModel struct {
	StartValue types.Float64 `tfsdk:"start_value"`
	EndValue   types.Float64 `tfsdk:"end_value"`
	Unit       types.String  `tfsdk:"unit"`
	CustomUnit types.String  `tfsdk:"custom_unit"`
}

type DashboardAnnotationMetricSourceModel struct {
	PromqlQuery     types.String `tfsdk:"promql_query"`
	Strategy        types.Object `tfsdk:"strategy"` //DashboardAnnotationMetricStrategyModel
	MessageTemplate types.String `tfsdk:"message_template"`
	Labels          types.List   `tfsdk:"labels"` //types.String
}

type DashboardAnnotationSpansOrLogsSourceModel struct {
	LuceneQuery     types.String `tfsdk:"lucene_query"`
	Strategy        types.Object `tfsdk:"strategy"` //DashboardAnnotationSpanOrLogsStrategyModel
	MessageTemplate types.String `tfsdk:"message_template"`
	LabelFields     types.List   `tfsdk:"label_fields"` //dashboard_widgets.ObservationFieldModel
}

type DashboardAnnotationSpanOrLogsStrategyModel struct {
	Instant  types.Object `tfsdk:"instant"`  //DashboardAnnotationInstantStrategyModel
	Range    types.Object `tfsdk:"range"`    //DashboardAnnotationRangeStrategyModel
	Duration types.Object `tfsdk:"duration"` //DashboardAnnotationDurationStrategyModel
}

type DashboardAnnotationInstantStrategyModel struct {
	TimestampField types.Object `tfsdk:"timestamp_field"` //dashboard_widgets.ObservationFieldModel
}

type DashboardAnnotationRangeStrategyModel struct {
	StartTimestampField types.Object `tfsdk:"start_timestamp_field"` //dashboard_widgets.ObservationFieldModel
	EndTimestampField   types.Object `tfsdk:"end_timestamp_field"`   //dashboard_widgets.ObservationFieldModel
}

type DashboardAnnotationDurationStrategyModel struct {
	StartTimestampField types.Object `tfsdk:"start_timestamp_field"` //dashboard_widgets.ObservationFieldModel
	DurationField       types.Object `tfsdk:"duration_field"`        //dashboard_widgets.ObservationFieldModel
}

type DashboardAnnotationMetricStrategyModel struct {
	StartTime types.Object `tfsdk:"start_time"` //MetricStrategyStartTimeModel
}

type MetricStrategyStartTimeModel struct{}

type DashboardAutoRefreshModel struct {
	Type types.String `tfsdk:"type"`
}

func typeBoolToBoolPointer(value types.Bool) *bool {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	return value.ValueBoolPointer()
}

func typeFloat64ToFloat64Pointer(value types.Float64) *float64 {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	return value.ValueFloat64Pointer()
}

func typeInt64ToInt32Pointer(value types.Int64) *int32 {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	result := int32(value.ValueInt64())
	return &result
}

func typeNumberToInt32Pointer(value types.Number) *int32 {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	result, _ := value.ValueBigFloat().Int64()
	typedResult := int32(result)
	return &typedResult
}

func int32PointerToTypeInt64(value *int32) types.Int64 {
	if value == nil {
		return types.Int64Null()
	}
	return types.Int64Value(int64(*value))
}

func int32PointerToNumberType(value *int32) types.Number {
	if value == nil {
		return types.NumberNull()
	}
	return types.NumberValue(big.NewFloat(float64(*value)))
}

func uuidToTypeString(id *dashboardservice.UUID) types.String {
	if id == nil || id.Value == nil {
		return types.StringNull()
	}
	return types.StringValue(*id.Value)
}

func stringValuesFromList(ctx context.Context, values types.List) ([]string, diag.Diagnostics) {
	var stringValues []types.String
	diags := values.ElementsAs(ctx, &stringValues, true)
	if diags.HasError() {
		return nil, diags
	}

	result := make([]string, 0, len(stringValues))
	for _, value := range stringValues {
		if value.IsNull() || value.IsUnknown() {
			continue
		}
		result = append(result, value.ValueString())
	}
	return result, nil
}

func typeStringValuesToStringSlice(_ context.Context, values []attr.Value) ([]string, diag.Diagnostics) {
	result := make([]string, 0, len(values))
	for _, value := range values {
		stringValue, ok := value.(types.String)
		if !ok {
			return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Invalid string value", fmt.Sprintf("expected types.String, got %T", value))}
		}
		if stringValue.IsNull() || stringValue.IsUnknown() {
			continue
		}
		result = append(result, stringValue.ValueString())
	}
	return result, nil
}

func NewDashboardResource() resource.Resource {
	return &DashboardResource{}
}

type DashboardResource struct {
	openAPIClient *dashboardOpenAPIClient
}

func (r DashboardResource) UpgradeState(_ context.Context) map[int64]resource.StateUpgrader {
	schemaV1 := dashboardschema.V1()
	schemaV2 := dashboardschema.V2()
	schemaV3 := dashboardschema.V3()

	return map[int64]resource.StateUpgrader{
		1: {
			PriorSchema:   &schemaV1,
			StateUpgrader: upgradeDashboardStateV1ToV2,
		},
		2: {
			PriorSchema: &schemaV2,
			StateUpgrader: func(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
				upgradeDashboardStateV3ToV4(ctx, req, resp, r.openAPIClient)
			},
		},
		3: {
			PriorSchema: &schemaV3,
			StateUpgrader: func(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
				upgradeDashboardStateV3ToV4(ctx, req, resp, r.openAPIClient)
			},
		},
	}
}

func upgradeDashboardStateV3ToV4(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse, client *dashboardOpenAPIClient) {
	log.Printf("[INFO] Upgrading state from v%v", req.State.Schema.GetVersion())
	var state DashboardResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	//Get refreshed Dashboard value from Coralogix
	id := state.ID.ValueString()
	log.Printf("[INFO] Reading Dashboard: %s", id)
	getDashboardResp, err := client.Get(ctx, id)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if errors.Is(err, errDashboardOpenAPINotFound) {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("Dashboard %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError(
				"Error reading Dashboard",
				err.Error(),
			)
		}
		return
	}
	log.Printf("[INFO] Received Dashboard: %s", dashboardLogString(getDashboardResp.Dashboard))

	flattenedDashboard, diags := flattenDashboard(ctx, state, getDashboardResp)
	if diags != nil {
		resp.Diagnostics.Append(diags...)
		return
	}
	state = *flattenedDashboard

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func upgradeDashboardStateV2ToV3(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {

	type DataPrimeModelV0 struct {
		Query   types.String `tfsdk:"query"`
		Filters types.List   `tfsdk:"filters"` //DashboardFilterSourceModel
	}

	type QuerySpansModelV0 struct {
		LuceneQuery  types.String `tfsdk:"lucene_query"`
		GroupBy      types.List   `tfsdk:"group_by"`     //SpansFieldModel
		Aggregations types.List   `tfsdk:"aggregations"` //SpansAggregationModel
		Filters      types.List   `tfsdk:"filters"`      //SpansFilterModel
	}

	type HexagonQueryMetricsModelV0 struct {
		PromqlQuery     types.String `tfsdk:"promql_query"`
		Filters         types.List   `tfsdk:"filters"` //MetricsFilterModel
		PromqlQueryType types.String `tfsdk:"promql_query_type"`
		Aggregation     types.String `tfsdk:"aggregation"`
	}

	type HexagonQueryLogsModelV0 struct {
		LuceneQuery types.String                           `tfsdk:"lucene_query"`
		GroupBy     types.List                             `tfsdk:"group_by"` //ObservationFieldModel
		Aggregation *dashboardwidgets.LogsAggregationModel `tfsdk:"aggregation"`
		Filters     types.List                             `tfsdk:"filters"` //LogsFilterModel
	}

	type HexagonQueryModelV0 struct {
		Logs      *HexagonQueryLogsModelV0    `tfsdk:"logs"`
		Metrics   *HexagonQueryMetricsModelV0 `tfsdk:"metrics"`
		Spans     *QuerySpansModelV0          `tfsdk:"spans"`
		DataPrime *DataPrimeModelV0           `tfsdk:"data_prime"`
	}

	type HexagonModelV0 struct {
		CustomUnit    types.String                     `tfsdk:"custom_unit"`
		LegendBy      types.String                     `tfsdk:"legend_by"`
		Decimal       types.Number                     `tfsdk:"decimal"`
		DataModeType  types.String                     `tfsdk:"data_mode_type"`
		Thresholds    types.Set                        `tfsdk:"thresholds"` //HexagonThresholdModel
		ThresholdType types.String                     `tfsdk:"threshold_type"`
		Min           types.Number                     `tfsdk:"min"`
		Max           types.Number                     `tfsdk:"max"`
		Unit          types.String                     `tfsdk:"unit"`
		Legend        *dashboardwidgets.LegendModel    `tfsdk:"legend"`
		Query         *HexagonQueryModelV0             `tfsdk:"query"`
		TimeFrame     *dashboardwidgets.TimeFrameModel `tfsdk:"time_frame"`
	}

	type WidgetDefinitionModelV0 struct {
		LineChart          *dashboardwidgets.LineChartModel          `tfsdk:"line_chart"`
		Hexagon            *HexagonModelV0                           `tfsdk:"hexagon"`
		DataTable          *dashboardwidgets.DataTableModel          `tfsdk:"data_table"`
		Gauge              *dashboardwidgets.GaugeModel              `tfsdk:"gauge"`
		PieChart           *dashboardwidgets.PieChartModel           `tfsdk:"pie_chart"`
		BarChart           *dashboardwidgets.BarChartModel           `tfsdk:"bar_chart"`
		HorizontalBarChart *dashboardwidgets.HorizontalBarChartModel `tfsdk:"horizontal_bar_chart"`
		Markdown           *dashboardwidgets.MarkdownModel           `tfsdk:"markdown"`
	}

	type WidgetModelV0 struct {
		ID          types.String             `tfsdk:"id"`
		Title       types.String             `tfsdk:"title"`
		Description types.String             `tfsdk:"description"`
		Definition  *WidgetDefinitionModelV0 `tfsdk:"definition"`
		Width       types.Int64              `tfsdk:"width"`
	}

	var priorStateData DashboardResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &priorStateData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var layout DashboardLayoutModel // this model did not change
	if !utils.ObjIsNullOrUnknown(priorStateData.Layout) {
		_ = priorStateData.Layout.As(ctx, &layout, basetypes.ObjectAsOptions{})
	}

	if layout.Sections.IsNull() || layout.Sections.IsUnknown() {
		resp.Diagnostics.Append(resp.State.Set(ctx, priorStateData)...)
		return
	}
	var sections []SectionModel
	diags := layout.Sections.ElementsAs(ctx, &sections, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	for _, sec := range sections {
		var rows []RowModel
		diags := sec.Rows.ElementsAs(ctx, &rows, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		for _, row := range rows {
			var widgets []WidgetModelV0
			diags := row.Widgets.ElementsAs(ctx, &widgets, false)
			resp.Diagnostics.Append(diags...)
			if resp.Diagnostics.HasError() {
				return
			}
			newWidgets := make([]attr.Value, 0)

			for _, widget := range widgets {
				newWidget := WidgetModel{
					ID:          widget.ID,
					Title:       widget.Title,
					Description: widget.Description,
					Definition:  nil,
					Width:       widget.Width,
				}
				if widget.Definition != nil {
					var newHex *dashboardwidgets.HexagonModel
					if widget.Definition.Hexagon != nil {
						timeFrame := widget.Definition.Hexagon.TimeFrame

						oldQuery := widget.Definition.Hexagon.Query

						var logs *dashboardwidgets.HexagonQueryLogsModel
						var metrics *dashboardwidgets.HexagonQueryMetricsModel
						var dataprime *dashboardwidgets.DataPrimeModel
						var spans *dashboardwidgets.HexagonQuerySpansModel
						if oldQuery.DataPrime != nil {
							dataprime = &dashboardwidgets.DataPrimeModel{
								TimeFrame: timeFrame,
								Query:     oldQuery.DataPrime.Query,
								Filters:   oldQuery.DataPrime.Filters,
							}
						}
						if oldQuery.Spans != nil {
							var aggregations []dashboardwidgets.SpansAggregationModel
							diags := oldQuery.Spans.Aggregations.ElementsAs(ctx, &aggregations, false)
							resp.Diagnostics.Append(diags...)
							if resp.Diagnostics.HasError() {
								return
							}
							var aggregation *dashboardwidgets.SpansAggregationModel
							if len(aggregations) > 0 {
								aggregation = &aggregations[0]
							}
							spans = &dashboardwidgets.HexagonQuerySpansModel{
								TimeFrame:   timeFrame,
								LuceneQuery: oldQuery.Spans.LuceneQuery,
								Filters:     oldQuery.Spans.Filters,
								Aggregation: aggregation,
								GroupBy:     oldQuery.Spans.GroupBy,
							}
						}
						if oldQuery.Metrics != nil {
							metrics = &dashboardwidgets.HexagonQueryMetricsModel{
								TimeFrame:       timeFrame,
								PromqlQuery:     oldQuery.Metrics.PromqlQuery,
								PromqlQueryType: oldQuery.Metrics.PromqlQueryType,
								Filters:         oldQuery.Metrics.Filters,
								Aggregation:     oldQuery.Metrics.Aggregation,
							}
						}
						if oldQuery.Logs != nil {
							logs = &dashboardwidgets.HexagonQueryLogsModel{
								TimeFrame:   timeFrame,
								LuceneQuery: oldQuery.Logs.LuceneQuery,
								Filters:     oldQuery.Logs.Filters,
								Aggregation: oldQuery.Logs.Aggregation,
								GroupBy:     oldQuery.Logs.GroupBy,
							}
						}

						query := &dashboardwidgets.HexagonQueryModel{
							Logs:      logs,
							Metrics:   metrics,
							DataPrime: dataprime,
							Spans:     spans,
						}
						newHex = &dashboardwidgets.HexagonModel{
							CustomUnit:    widget.Definition.Hexagon.CustomUnit,
							LegendBy:      widget.Definition.Hexagon.LegendBy,
							Decimal:       widget.Definition.Hexagon.Decimal,
							DataModeType:  widget.Definition.Hexagon.DataModeType,
							Thresholds:    widget.Definition.Hexagon.Thresholds,
							ThresholdType: widget.Definition.Hexagon.ThresholdType,
							Min:           widget.Definition.Hexagon.Min,
							Max:           widget.Definition.Hexagon.Max,
							Unit:          widget.Definition.Hexagon.Unit,
							Legend:        widget.Definition.Hexagon.Legend,
							Query:         query,
						}
					}

					newWidget.Definition = &dashboardwidgets.WidgetDefinitionModel{
						LineChart:          widget.Definition.LineChart,
						Hexagon:            newHex,
						DataTable:          widget.Definition.DataTable,
						Gauge:              widget.Definition.Gauge,
						PieChart:           widget.Definition.PieChart,
						BarChart:           widget.Definition.BarChart,
						HorizontalBarChart: widget.Definition.HorizontalBarChart,
						Markdown:           widget.Definition.Markdown,
					}
				}
				widgetElement, diags := types.ObjectValueFrom(ctx, widgetModelAttr(), newWidget)

				if diags.HasError() {
					resp.Diagnostics.Append(diags...)
					continue
				}
				if !utils.ObjIsNullOrUnknown(widgetElement) {
					newWidgets = append(newWidgets, widgetElement)
				}
			}
			row.Widgets = types.ListValueMust(types.ObjectType{AttrTypes: widgetModelAttr()}, newWidgets)
		}
	}
	newLayout, _ := types.ObjectValueFrom(ctx, layoutModelAttr(), layout)

	upgradedStateData := DashboardResourceModel{
		ID:          priorStateData.ID,
		Name:        priorStateData.Name,
		Description: priorStateData.Description,
		Layout:      newLayout,
		Variables:   priorStateData.Variables,
		Filters:     priorStateData.Filters,
		TimeFrame:   priorStateData.TimeFrame,
		Folder:      priorStateData.Folder,
		Annotations: priorStateData.Annotations,
		AutoRefresh: priorStateData.AutoRefresh,
		ContentJson: priorStateData.ContentJson,
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, upgradedStateData)...)
}

func upgradeDashboardStateV1ToV2(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
	type DashboardResourceModelV0 struct {
		ID          types.String `tfsdk:"id"`
		Name        types.String `tfsdk:"name"`
		Description types.String `tfsdk:"description"`
		Layout      types.Object `tfsdk:"layout"`
		Variables   types.List   `tfsdk:"variables"`
		Filters     types.List   `tfsdk:"filters"`
		TimeFrame   types.Object `tfsdk:"time_frame"`
		Folder      types.Object `tfsdk:"folder"`
		Annotations types.List   `tfsdk:"annotations"`
		ContentJson types.String `tfsdk:"content_json"`
	}

	var priorStateData DashboardResourceModelV0
	resp.Diagnostics.Append(req.State.Get(ctx, &priorStateData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	annotations, diags := upgradeDashboardAnnotationsV0(ctx, priorStateData.Annotations)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	var timeframe *dashboardwidgets.TimeFrameModel
	if !utils.ObjIsNullOrUnknown(priorStateData.TimeFrame) {
		_ = priorStateData.TimeFrame.As(ctx, timeframe, basetypes.ObjectAsOptions{})
	} else {
		timeframe = nil
	}

	upgradedStateData := DashboardResourceModel{
		ID:          priorStateData.ID,
		Name:        priorStateData.Name,
		Description: priorStateData.Description,
		Layout:      priorStateData.Layout,
		Variables:   priorStateData.Variables,
		Filters:     priorStateData.Filters,
		TimeFrame:   timeframe,
		Folder:      priorStateData.Folder,
		Annotations: annotations,
		AutoRefresh: types.ObjectNull(dashboardAutoRefreshModelAttr()),
		ContentJson: priorStateData.ContentJson,
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, upgradedStateData)...)
}

func upgradeDashboardAnnotationsV0(ctx context.Context, annotations types.List) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	var priorAnnotationObjects []types.Object
	var upgradedGroups []DashboardAnnotationModel
	annotations.ElementsAs(ctx, &priorAnnotationObjects, true)

	for _, annotationObject := range priorAnnotationObjects {
		var priorAnnotation DashboardAnnotationModel
		if dg := annotationObject.As(ctx, &priorAnnotation, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}

		source, dg := upgradeAnnotationSourceV0(ctx, priorAnnotation.Source)
		if dg.HasError() {
			diags.Append(dg...)
			continue
		}

		upgradedGroup := DashboardAnnotationModel{
			Name:    priorAnnotation.Name,
			Enabled: priorAnnotation.Enabled,
			Source:  source,
			ID:      priorAnnotation.ID,
		}

		upgradedGroups = append(upgradedGroups, upgradedGroup)
	}

	if diags.HasError() {
		return types.ListNull(types.ObjectType{AttrTypes: dashboardsAnnotationsModelAttr()}), diags
	}

	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: dashboardsAnnotationsModelAttr()}, upgradedGroups)
}

func upgradeAnnotationSourceV0(ctx context.Context, source types.Object) (types.Object, diag.Diagnostics) {
	type DashboardAnnotationSourceModelV0 struct {
		Metric types.Object `tfsdk:"metric"` //DashboardAnnotationMetricSourceModel
	}
	var priorSource DashboardAnnotationSourceModelV0
	if dg := source.As(ctx, &priorSource, basetypes.ObjectAsOptions{}); dg.HasError() {
		return types.ObjectNull(annotationSourceModelAttr()), dg
	}

	upgradeSource := DashboardAnnotationSourceModel{
		Metrics: priorSource.Metric,
		Logs:    types.ObjectNull(annotationsLogsAndSpansSourceModelAttr()),
		Spans:   types.ObjectNull(annotationsLogsAndSpansSourceModelAttr()),
		Manual:  types.ObjectNull(annotationsManualSourceModelAttr()),
	}

	return types.ObjectValueFrom(ctx, annotationSourceModelAttr(), upgradeSource)
}

func (r DashboardResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r DashboardResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dashboard"
}

func (r *DashboardResource) Schema(_ context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = dashboardschema.V4()
}

func (r DashboardResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {

	// Retrieve values from plan
	var plan DashboardResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	dashboard, diags := extractDashboard(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	accessPolicy := dashboardAccessPolicyForRequest(plan.AccessPolicy)
	log.Printf("[INFO] Creating new Dashboard: %s", dashboardLogString(dashboard))
	createResponse, err := r.createDashboard(ctx, dashboard, accessPolicy)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error creating Dashboard",
			err.Error(),
		)
		return
	}

	dashboardID := createResponse.GetDashboardId()
	if dashboardID == "" {
		resp.Diagnostics.AddError(
			"Error creating Dashboard",
			"OpenAPI create response did not include dashboardId",
		)
		return
	}

	getDashboardResp, err := r.openAPIClient.Get(ctx, dashboardID)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error getting Dashboard",
			err.Error(),
		)
		return
	}
	log.Printf("[INFO] Submitted new Dashboard: %s", dashboardLogString(getDashboardResp.Dashboard))

	flattenedDashboard, diags := flattenDashboard(ctx, plan, getDashboardResp)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	log.Printf("[INFO] Flattened Dashboard: %v", flattenedDashboard)
	plan = *flattenedDashboard

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func dashboardAccessPolicyForRequest(accessPolicy types.String) *string {
	if accessPolicy.IsNull() || accessPolicy.IsUnknown() || accessPolicy.ValueString() == "" {
		return nil
	}

	return accessPolicy.ValueStringPointer()
}

func dashboardAccessPolicyForConfiguredRequest(configAccessPolicy, planAccessPolicy types.String) *string {
	if configAccessPolicy.IsNull() {
		return nil
	}

	return dashboardAccessPolicyForRequest(planAccessPolicy)
}

func (r DashboardResource) createDashboard(ctx context.Context, dashboard *dashboardservice.Dashboard, accessPolicy *string) (*dashboardservice.CreateDashboardResponse, error) {
	return r.openAPIClient.Create(ctx, dashboard, accessPolicy)
}

func (r DashboardResource) replaceDashboard(ctx context.Context, dashboard *dashboardservice.Dashboard, accessPolicy *string) error {
	return r.openAPIClient.Replace(ctx, dashboard, accessPolicy)
}

func dashboardLogString(dashboard any) string {
	content, err := json.Marshal(dashboard)
	if err != nil {
		return fmt.Sprintf("%+v", dashboard)
	}
	return string(content)
}

func extractDashboard(ctx context.Context, plan DashboardResourceModel) (*dashboardservice.Dashboard, diag.Diagnostics) {
	if !plan.ContentJson.IsNull() {
		dashboard := new(dashboardservice.Dashboard)
		if err := json.Unmarshal([]byte(plan.ContentJson.ValueString()), dashboard); err != nil {
			return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error unmarshalling dashboard content json", err.Error())}
		}
		if err := restoreOpenAPIProtoFieldNames(dashboard); err != nil {
			return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error normalizing dashboard content json", err.Error())}
		}
		dashboard, diags := expandOpenAPIDashboardFolder(ctx, dashboard, plan.Folder)
		if diags.HasError() {
			return nil, diags
		}
		return dashboard, nil
	}
	layout, diags := expandDashboardLayout(ctx, plan.Layout)
	if diags.HasError() {
		return nil, diags
	}

	variables, diags := expandDashboardVariables(ctx, plan.Variables)
	if diags.HasError() {
		return nil, diags
	}

	filters, diags := expandDashboardFilters(ctx, plan.Filters)
	if diags.HasError() {
		return nil, diags
	}

	annotations, diags := expandDashboardAnnotations(ctx, plan.Annotations)
	if diags.HasError() {
		return nil, diags
	}

	var layoutValue dashboardservice.Layout
	if layout != nil {
		layoutValue = *layout
	}

	dashboard := &dashboardservice.Dashboard{
		Id:          utils.TypeStringToStringPointer(plan.ID),
		Name:        plan.Name.ValueString(),
		Description: utils.TypeStringToStringPointer(plan.Description),
		Layout:      layoutValue,
		Variables:   variables,
		Filters:     filters,
		Annotations: annotations,
	}

	if plan.TimeFrame != nil {
		dashboard, diags = dashboardwidgets.ExpandDashboardTimeFrame(ctx, dashboard, plan.TimeFrame)
		if diags.HasError() {
			return nil, diags
		}
	}

	dashboard, diags = expandDashboardFolder(ctx, dashboard, plan.Folder)
	if diags.HasError() {
		return nil, diags
	}

	dashboard, diags = expandDashboardAutoRefresh(ctx, dashboard, plan.AutoRefresh)
	if diags.HasError() {
		return nil, diags
	}

	return dashboard, nil
}

func expandDashboardAutoRefresh(ctx context.Context, dashboard *dashboardservice.Dashboard, refresh types.Object) (*dashboardservice.Dashboard, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(refresh) {
		return dashboard, nil
	}
	var refreshObject DashboardAutoRefreshModel
	diags := refresh.As(ctx, &refreshObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	switch refreshObject.Type.ValueString() {
	case "two_minutes":
		dashboard.TwoMinutes = map[string]interface{}{}
		dashboard.FiveMinutes = nil
		dashboard.Off = nil
	case "five_minutes":
		dashboard.FiveMinutes = map[string]interface{}{}
		dashboard.TwoMinutes = nil
		dashboard.Off = nil
	default:
		dashboard.Off = map[string]interface{}{}
		dashboard.TwoMinutes = nil
		dashboard.FiveMinutes = nil
	}

	return dashboard, nil
}

func expandDashboardAnnotations(ctx context.Context, annotations types.List) ([]dashboardservice.Annotation, diag.Diagnostics) {
	var annotationsObjects []types.Object
	var expandedAnnotations []dashboardservice.Annotation
	diags := annotations.ElementsAs(ctx, &annotationsObjects, true)
	if diags.HasError() {
		return nil, diags
	}

	for _, ao := range annotationsObjects {
		var annotation DashboardAnnotationModel
		if dg := ao.As(ctx, &annotation, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expandedAnnotation, expandDiags := expandAnnotation(ctx, annotation)
		if expandDiags.HasError() {
			diags.Append(expandDiags...)
			continue
		}
		expandedAnnotations = append(expandedAnnotations, *expandedAnnotation)
	}

	return expandedAnnotations, diags
}

func expandAnnotation(ctx context.Context, annotation DashboardAnnotationModel) (*dashboardservice.Annotation, diag.Diagnostics) {
	source, diags := expandAnnotationSource(ctx, annotation.Source)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.Annotation{
		Id:      dashboardwidgets.ExpandDashboardIDs(annotation.ID),
		Name:    utils.TypeStringToStringPointer(annotation.Name),
		Enabled: typeBoolToBoolPointer(annotation.Enabled),
		Source:  source,
	}, nil

}

func expandAnnotationSource(ctx context.Context, source types.Object) (*dashboardservice.AnnotationSource, diag.Diagnostics) {
	if source.IsNull() || source.IsUnknown() {
		return nil, nil
	}
	var sourceObject DashboardAnnotationSourceModel
	diags := source.As(ctx, &sourceObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	switch {
	case !(sourceObject.Logs.IsNull() || sourceObject.Logs.IsUnknown()):
		logsSource, diags := expandLogsSource(ctx, sourceObject.Logs)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboardservice.AnnotationSource{Logs: logsSource}, nil
	case !(sourceObject.Metrics.IsNull() || sourceObject.Metrics.IsUnknown()):
		metricSource, diags := expandMetricSource(ctx, sourceObject.Metrics)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboardservice.AnnotationSource{Metrics: metricSource}, nil
	case !(sourceObject.Spans.IsNull() || sourceObject.Spans.IsUnknown()):
		spansSource, diags := expandSpansSource(ctx, sourceObject.Spans)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboardservice.AnnotationSource{Spans: spansSource}, nil
	case !(sourceObject.Manual.IsNull() || sourceObject.Manual.IsUnknown()):
		manualSource, diags := expandManualSource(ctx, sourceObject.Manual)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboardservice.AnnotationSource{Manual: manualSource}, nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand Annotation Source", "Annotation Source must be either Logs, Metric, Spans or Manual")}
	}
}

func expandManualSource(ctx context.Context, manual types.Object) (*dashboardservice.ManualSource, diag.Diagnostics) {
	if manual.IsNull() || manual.IsUnknown() {
		return nil, nil
	}
	var manualObject DashboardAnnotationManualSourceModel
	diags := manual.As(ctx, &manualObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	strategy, diags := expandManualSourceStrategy(ctx, manualObject.Strategy)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.ManualSource{
		Strategy:        strategy,
		MessageTemplate: utils.TypeStringToStringPointer(manualObject.MessageTemplate),
		Orientation:     expandManualAnnotationOrientation(manualObject.Orientation).Ptr(),
	}, nil
}

func expandManualSourceStrategy(ctx context.Context, strategy types.Object) (*dashboardservice.ManualSourceStrategy, diag.Diagnostics) {
	var strategyObject DashboardAnnotationManualStrategyModel
	diags := strategy.As(ctx, &strategyObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	switch {
	case !utils.ObjIsNullOrUnknown(strategyObject.Instant):
		return expandManualSourceInstantStrategy(ctx, strategyObject.Instant)
	case !utils.ObjIsNullOrUnknown(strategyObject.Range):
		return expandManualSourceRangeStrategy(ctx, strategyObject.Range)
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand Manual Source Strategy", "Manual Source Strategy must be either Instant or Range")}
	}
}

func expandManualSourceInstantStrategy(ctx context.Context, instant types.Object) (*dashboardservice.ManualSourceStrategy, diag.Diagnostics) {
	var instantObject DashboardAnnotationManualInstantStrategyModel
	if diags := instant.As(ctx, &instantObject, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	unit := dashboardwidgets.DashboardSchemaToProtoUnit[instantObject.Unit.ValueString()]
	return &dashboardservice.ManualSourceStrategy{
		Instant: &dashboardservice.ManualSourceStrategyInstant{
			Value:      typeFloat64ToFloat64Pointer(instantObject.Value),
			Unit:       unit.Ptr(),
			CustomUnit: utils.TypeStringToStringPointer(instantObject.CustomUnit),
		},
	}, nil
}

func expandManualSourceRangeStrategy(ctx context.Context, object types.Object) (*dashboardservice.ManualSourceStrategy, diag.Diagnostics) {
	var rangeObject DashboardAnnotationManualRangeStrategyModel
	if diags := object.As(ctx, &rangeObject, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	unit := dashboardwidgets.DashboardSchemaToProtoUnit[rangeObject.Unit.ValueString()]
	return &dashboardservice.ManualSourceStrategy{
		Range: &dashboardservice.ManualSourceStrategyRange{
			StartValue: typeFloat64ToFloat64Pointer(rangeObject.StartValue),
			EndValue:   typeFloat64ToFloat64Pointer(rangeObject.EndValue),
			Unit:       unit.Ptr(),
			CustomUnit: utils.TypeStringToStringPointer(rangeObject.CustomUnit),
		},
	}, nil
}

func expandManualAnnotationOrientation(orientation types.String) dashboardservice.AnnotationOrientation {
	if o, ok := dashboardManualAnnotationOrientationToProto[orientation.ValueString()]; ok {
		return o
	}
	return dashboardservice.ANNOTATIONORIENTATION_ANNOTATION_ORIENTATION_VERTICAL_UNSPECIFIED
}

func flattenManualAnnotationOrientation(orientation dashboardservice.AnnotationOrientation) types.String {
	if s, ok := dashboardManualAnnotationOrientationToSchema[orientation]; ok {
		return types.StringValue(s)
	}
	return types.StringValue("vertical")
}

func expandLogsSource(ctx context.Context, logs types.Object) (*dashboardservice.LogsSource, diag.Diagnostics) {
	if logs.IsNull() || logs.IsUnknown() {
		return nil, nil
	}
	var logsObject DashboardAnnotationSpansOrLogsSourceModel
	diags := logs.As(ctx, &logsObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	strategy, diags := expandLogsSourceStrategy(ctx, logsObject.Strategy)
	if diags.HasError() {
		return nil, diags
	}

	labels, diags := dashboardwidgets.ExpandObservationFields(ctx, logsObject.LabelFields)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.LogsSource{
		LuceneQuery:     dashboardwidgets.ExpandLuceneQuery(logsObject.LuceneQuery),
		Strategy:        strategy,
		MessageTemplate: utils.TypeStringToStringPointer(logsObject.MessageTemplate),
		LabelFields:     labels,
	}, nil
}

func expandLogsSourceStrategy(ctx context.Context, strategy types.Object) (*dashboardservice.LogsSourceStrategy, diag.Diagnostics) {
	var strategyObject DashboardAnnotationSpanOrLogsStrategyModel
	diags := strategy.As(ctx, &strategyObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	switch {
	case !utils.ObjIsNullOrUnknown(strategyObject.Instant):
		return expandLogsSourceInstantStrategy(ctx, strategyObject.Instant)
	case !utils.ObjIsNullOrUnknown(strategyObject.Range):
		return expandLogsSourceRangeStrategy(ctx, strategyObject.Range)
	case !utils.ObjIsNullOrUnknown(strategyObject.Duration):
		return expandLogsSourceDurationStrategy(ctx, strategyObject.Duration)
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand Logs Source Strategy", "Logs Source Strategy must be either Instant, Range or Duration")}
	}
}

func expandLogsSourceDurationStrategy(ctx context.Context, duration types.Object) (*dashboardservice.LogsSourceStrategy, diag.Diagnostics) {
	var durationObject DashboardAnnotationDurationStrategyModel
	diags := duration.As(ctx, &durationObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	startTimestampField, diags := dashboardwidgets.ExpandObservationFieldObject(ctx, durationObject.StartTimestampField)
	if diags.HasError() {
		return nil, diags
	}

	durationField, diags := dashboardwidgets.ExpandObservationFieldObject(ctx, durationObject.DurationField)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.LogsSourceStrategy{
		Duration: &dashboardservice.LogsSourceStrategyDuration{
			StartTimestampField: startTimestampField,
			DurationField:       durationField,
		},
	}, nil
}

func expandLogsSourceRangeStrategy(ctx context.Context, object types.Object) (*dashboardservice.LogsSourceStrategy, diag.Diagnostics) {
	var rangeObject DashboardAnnotationRangeStrategyModel
	if diags := object.As(ctx, &rangeObject, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	startTimestampField, diags := dashboardwidgets.ExpandObservationFieldObject(ctx, rangeObject.StartTimestampField)
	if diags.HasError() {
		return nil, diags
	}

	endTimestampField, diags := dashboardwidgets.ExpandObservationFieldObject(ctx, rangeObject.EndTimestampField)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.LogsSourceStrategy{
		Range: &dashboardservice.LogsSourceStrategyRange{
			StartTimestampField: startTimestampField,
			EndTimestampField:   endTimestampField,
		},
	}, nil
}

func expandLogsSourceInstantStrategy(ctx context.Context, instant types.Object) (*dashboardservice.LogsSourceStrategy, diag.Diagnostics) {
	var instantObject DashboardAnnotationInstantStrategyModel
	if diags := instant.As(ctx, &instantObject, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	timestampField, diags := dashboardwidgets.ExpandObservationFieldObject(ctx, instantObject.TimestampField)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.LogsSourceStrategy{
		Instant: &dashboardservice.LogsSourceStrategyInstant{
			TimestampField: timestampField,
		},
	}, nil
}

func expandSpansSourceStrategy(ctx context.Context, strategy types.Object) (*dashboardservice.SpansSourceStrategy, diag.Diagnostics) {
	var strategyObject DashboardAnnotationSpanOrLogsStrategyModel
	diags := strategy.As(ctx, &strategyObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	switch {
	case !(strategyObject.Instant.IsNull() || strategyObject.Instant.IsUnknown()):
		return expandSpansSourceInstantStrategy(ctx, strategyObject.Instant)
	case !(strategyObject.Range.IsNull() || strategyObject.Range.IsUnknown()):
		return expandSpansSourceRangeStrategy(ctx, strategyObject.Range)
	case !(strategyObject.Duration.IsNull() || strategyObject.Duration.IsUnknown()):
		return expandSpansSourceDurationStrategy(ctx, strategyObject.Duration)
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand Spans Source Strategy", "Spans Source Strategy must be either Instant, Range or Duration")}
	}
}

func expandSpansSourceDurationStrategy(ctx context.Context, duration types.Object) (*dashboardservice.SpansSourceStrategy, diag.Diagnostics) {
	var durationObject DashboardAnnotationDurationStrategyModel
	diags := duration.As(ctx, &durationObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	startTimestampField, diags := dashboardwidgets.ExpandObservationFieldObject(ctx, durationObject.StartTimestampField)
	if diags.HasError() {
		return nil, diags
	}

	durationField, diags := dashboardwidgets.ExpandObservationFieldObject(ctx, durationObject.DurationField)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.SpansSourceStrategy{
		Duration: &dashboardservice.SpansSourceStrategyDuration{
			StartTimestampField: startTimestampField,
			DurationField:       durationField,
		},
	}, nil
}

func expandSpansSourceRangeStrategy(ctx context.Context, object types.Object) (*dashboardservice.SpansSourceStrategy, diag.Diagnostics) {
	var rangeObject DashboardAnnotationRangeStrategyModel
	if diags := object.As(ctx, &rangeObject, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	startTimestampField, diags := dashboardwidgets.ExpandObservationFieldObject(ctx, rangeObject.StartTimestampField)
	if diags.HasError() {
		return nil, diags
	}

	endTimestampField, diags := dashboardwidgets.ExpandObservationFieldObject(ctx, rangeObject.EndTimestampField)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.SpansSourceStrategy{
		Range: &dashboardservice.SpansSourceStrategyRange{
			StartTimestampField: startTimestampField,
			EndTimestampField:   endTimestampField,
		},
	}, nil
}

func expandSpansSourceInstantStrategy(ctx context.Context, instant types.Object) (*dashboardservice.SpansSourceStrategy, diag.Diagnostics) {
	var instantObject DashboardAnnotationInstantStrategyModel
	if diags := instant.As(ctx, &instantObject, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	timestampField, diags := dashboardwidgets.ExpandObservationFieldObject(ctx, instantObject.TimestampField)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.SpansSourceStrategy{
		Instant: &dashboardservice.SpansSourceStrategyInstant{
			TimestampField: timestampField,
		},
	}, nil
}

func expandSpansSource(ctx context.Context, spans types.Object) (*dashboardservice.SpansSource, diag.Diagnostics) {
	if spans.IsNull() || spans.IsUnknown() {
		return nil, nil
	}
	var spansObject DashboardAnnotationSpansOrLogsSourceModel
	diags := spans.As(ctx, &spansObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	strategy, diags := expandSpansSourceStrategy(ctx, spansObject.Strategy)
	if diags.HasError() {
		return nil, diags
	}

	labels, diags := dashboardwidgets.ExpandObservationFields(ctx, spansObject.LabelFields)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.SpansSource{
		LuceneQuery:     dashboardwidgets.ExpandLuceneQuery(spansObject.LuceneQuery),
		Strategy:        strategy,
		MessageTemplate: utils.TypeStringToStringPointer(spansObject.MessageTemplate),
		LabelFields:     labels,
	}, nil
}

func expandMetricSource(ctx context.Context, metric types.Object) (*dashboardservice.MetricsSource, diag.Diagnostics) {
	if metric.IsNull() || metric.IsUnknown() {
		return nil, nil
	}
	var metricObject DashboardAnnotationMetricSourceModel
	diags := metric.As(ctx, &metricObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	strategy, diags := expandMetricSourceStrategy(ctx, metricObject.Strategy)
	if diags.HasError() {
		return nil, diags
	}

	labels, diags := stringValuesFromList(ctx, metricObject.Labels)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.MetricsSource{
		PromqlQuery:     dashboardwidgets.ExpandPromqlQuery(metricObject.PromqlQuery),
		Strategy:        strategy,
		MessageTemplate: utils.TypeStringToStringPointer(metricObject.MessageTemplate),
		Labels:          labels,
	}, nil
}

func expandMetricSourceStrategy(ctx context.Context, strategy types.Object) (*dashboardservice.MetricsSourceStrategy, diag.Diagnostics) {
	var strategyObject DashboardAnnotationMetricStrategyModel
	diags := strategy.As(ctx, &strategyObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.MetricsSourceStrategy{StartTimeMetric: map[string]interface{}{}}, nil
}

func expandDashboardLayout(ctx context.Context, layout types.Object) (*dashboardservice.Layout, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(layout) {
		return nil, nil
	}
	var layoutObject DashboardLayoutModel
	if diags := layout.As(ctx, &layoutObject, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}
	sections, diags := expandDashboardSections(ctx, layoutObject.Sections)
	if diags.HasError() {
		return nil, diags
	}
	return &dashboardservice.Layout{
		Sections: sections,
	}, nil
}

func expandDashboardSections(ctx context.Context, sections types.List) ([]dashboardservice.Section, diag.Diagnostics) {
	var sectionsObjects []types.Object
	var expandedSections []dashboardservice.Section
	diags := sections.ElementsAs(ctx, &sectionsObjects, true)
	if diags.HasError() {
		return nil, diags
	}
	for _, so := range sectionsObjects {
		var section SectionModel
		if dg := so.As(ctx, &section, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expandedSection, expandSectionDiags := expandSection(ctx, section)
		if expandSectionDiags.HasError() {
			diags.Append(expandSectionDiags...)
			continue
		}
		expandedSections = append(expandedSections, *expandedSection)
	}

	return expandedSections, diags
}

func expandSection(ctx context.Context, section SectionModel) (*dashboardservice.Section, diag.Diagnostics) {
	id := dashboardwidgets.ExpandDashboardUUID(section.ID)
	rows, diags := expandDashboardRows(ctx, section.Rows)
	if diags.HasError() {
		return nil, diags
	}

	if section.Options != nil {
		options, diags := expandSectionOptions(ctx, *section.Options)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboardservice.Section{
			Id:      id,
			Rows:    rows,
			Options: options,
		}, nil
	} else {
		return &dashboardservice.Section{
			Id:      id,
			Rows:    rows,
			Options: nil,
		}, nil
	}
}

func expandSectionOptions(_ context.Context, option SectionOptionsModel) (*dashboardservice.SectionOptions, diag.Diagnostics) {

	var color *dashboardservice.SectionColor
	if !option.Color.IsNull() {
		predefinedColor := dashboardservice.SectionPredefinedColor(fmt.Sprintf("SECTION_PREDEFINED_COLOR_%s", strings.ToUpper(option.Color.ValueString())))
		if !predefinedColor.IsValid() && option.Color.String() != utils.UNSPECIFIED {
			return nil, diag.Diagnostics{
				diag.NewErrorDiagnostic(
					"Extract Dashboard Section Options Error",
					fmt.Sprintf("Unknown color: %s", option.Color.ValueString()),
				),
			}
		}
		color = &dashboardservice.SectionColor{
			Predefined: predefinedColor.Ptr(),
		}
	}

	return &dashboardservice.SectionOptions{
		Custom: &dashboardservice.CustomSectionOptions{
			Name:        utils.TypeStringToStringPointer(option.Name),
			Description: utils.TypeStringToStringPointer(option.Description),
			Collapsed:   typeBoolToBoolPointer(option.Collapsed),
			Color:       color,
		},
	}, nil
}

func expandDashboardRows(ctx context.Context, rows types.List) ([]dashboardservice.Row, diag.Diagnostics) {
	var rowsObjects []types.Object
	var expandedRows []dashboardservice.Row
	diags := rows.ElementsAs(ctx, &rowsObjects, true)
	if diags.HasError() {
		return nil, diags
	}
	for _, ro := range rowsObjects {
		var row RowModel
		if dg := ro.As(ctx, &row, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expandedRow, expandDiags := expandRow(ctx, row)
		if expandDiags.HasError() {
			diags.Append(expandDiags...)
			continue
		}
		expandedRows = append(expandedRows, *expandedRow)
	}

	return expandedRows, diags
}

func expandRow(ctx context.Context, row RowModel) (*dashboardservice.Row, diag.Diagnostics) {
	id := dashboardwidgets.ExpandDashboardUUID(row.ID)
	appearance := &dashboardservice.RowAppearance{
		Height: typeInt64ToInt32Pointer(row.Height),
	}
	widgets, diags := expandDashboardWidgets(ctx, row.Widgets)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.Row{
		Id:         id,
		Appearance: appearance,
		Widgets:    widgets,
	}, nil
}

func expandDashboardWidgets(ctx context.Context, widgets types.List) ([]dashboardservice.Widget, diag.Diagnostics) {
	var widgetsObjects []types.Object
	var expandedWidgets []dashboardservice.Widget
	diags := widgets.ElementsAs(ctx, &widgetsObjects, true)

	if diags.HasError() {
		return nil, diags
	}

	for _, wo := range widgetsObjects {
		var widget WidgetModel
		if dg := wo.As(ctx, &widget, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expandedWidget, expandDiags := expandWidget(ctx, widget)
		if expandDiags.HasError() {
			diags.Append(expandDiags...)
			continue
		}
		expandedWidgets = append(expandedWidgets, *expandedWidget)
	}

	return expandedWidgets, diags
}

func expandWidget(ctx context.Context, widget WidgetModel) (*dashboardservice.Widget, diag.Diagnostics) {
	id := dashboardwidgets.ExpandDashboardUUID(widget.ID)

	title := utils.TypeStringToStringPointer(widget.Title)
	description := utils.TypeStringToStringPointer(widget.Description)
	appearance := &dashboardservice.WidgetAppearance{
		Width: typeInt64ToInt32Pointer(widget.Width),
	}
	definition, diags := expandWidgetDefinition(ctx, widget.Definition)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.Widget{
		Id:          id,
		Title:       title,
		Description: description,
		Appearance:  appearance,
		Definition:  definition,
	}, nil
}

func expandWidgetDefinition(ctx context.Context, definition *dashboardwidgets.WidgetDefinitionModel) (*dashboardservice.WidgetDefinition, diag.Diagnostics) {
	switch {
	case definition.PieChart != nil:
		return expandPieChart(ctx, definition.PieChart)
	case definition.Gauge != nil:
		return expandGauge(ctx, definition.Gauge)
	case definition.Hexagon != nil:
		return dashboardwidgets.ExpandHexagon(ctx, definition.Hexagon)
	case definition.LineChart != nil:
		return dashboardwidgets.ExpandLineChart(ctx, definition.LineChart)
	case definition.DataTable != nil:
		return dashboardwidgets.ExpandDataTable(ctx, definition.DataTable)
	case definition.BarChart != nil:
		return expandBarChart(ctx, definition.BarChart)
	case definition.HorizontalBarChart != nil:
		return expandHorizontalBarChart(ctx, definition.HorizontalBarChart)
	case definition.Markdown != nil:
		return expandMarkdown(definition.Markdown)
	default:
		return nil, diag.Diagnostics{
			diag.NewErrorDiagnostic(
				"Extract Dashboard Widget Definition Error",
				fmt.Sprintf("Unknown widget definition type: %#v", definition),
			),
		}
	}
}

func expandMarkdown(markdown *dashboardwidgets.MarkdownModel) (*dashboardservice.WidgetDefinition, diag.Diagnostics) {
	return &dashboardservice.WidgetDefinition{
		Markdown: &dashboardservice.Markdown{
			MarkdownText: utils.TypeStringToStringPointer(markdown.MarkdownText),
			TooltipText:  utils.TypeStringToStringPointer(markdown.TooltipText),
		},
	}, nil
}

func expandHorizontalBarChart(ctx context.Context, chart *dashboardwidgets.HorizontalBarChartModel) (*dashboardservice.WidgetDefinition, diag.Diagnostics) {
	var diags diag.Diagnostics

	query, diags := expandHorizontalBarChartQuery(ctx, chart.Query)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.WidgetDefinition{
		HorizontalBarChart: &dashboardservice.HorizontalBarChart{
			Query:             query,
			StackDefinition:   expandHorizontalBarChartStackDefinition(chart.StackDefinition),
			MaxBarsPerChart:   typeInt64ToInt32Pointer(chart.MaxBarsPerChart),
			ScaleType:         dashboardwidgets.OptionalEnumPointer(chart.ScaleType, dashboardwidgets.DashboardSchemaToProtoScaleType),
			GroupNameTemplate: utils.TypeStringToStringPointer(chart.GroupNameTemplate),
			Unit:              dashboardwidgets.OptionalEnumPointer(chart.Unit, dashboardwidgets.DashboardSchemaToProtoUnit),
			ColorsBy:          expandColorsBy(chart.ColorsBy),
			DisplayOnBar:      typeBoolToBoolPointer(chart.DisplayOnBar),
			YAxisViewBy:       expandYAxisViewBy(chart.YAxisViewBy),
			SortBy:            dashboardwidgets.OptionalEnumPointer(chart.SortBy, dashboardwidgets.DashboardSchemaToProtoSortBy),
			ColorScheme:       utils.TypeStringToStringPointer(chart.ColorScheme),
			DataModeType:      dashboardwidgets.OptionalEnumPointer(chart.DataModeType, dashboardwidgets.DashboardSchemaToProtoDataModeType),
		},
	}, nil
}

func expandYAxisViewBy(yAxisViewBy types.String) *dashboardservice.HorizontalBarChartYAxisViewBy {
	switch yAxisViewBy.ValueString() {
	case "category":
		return &dashboardservice.HorizontalBarChartYAxisViewBy{
			Category: map[string]interface{}{},
		}
	case "value":
		return &dashboardservice.HorizontalBarChartYAxisViewBy{
			Value: map[string]interface{}{},
		}
	default:
		return nil
	}
}

func expandPieChart(ctx context.Context, pieChart *dashboardwidgets.PieChartModel) (*dashboardservice.WidgetDefinition, diag.Diagnostics) {
	var diags diag.Diagnostics

	query, diags := expandPieChartQuery(ctx, pieChart.Query)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.WidgetDefinition{
		PieChart: &dashboardservice.WidgetsPieChart{
			Query:              query,
			MaxSlicesPerChart:  typeInt64ToInt32Pointer(pieChart.MaxSlicesPerChart),
			MinSlicePercentage: typeInt64ToInt32Pointer(pieChart.MinSlicePercentage),
			StackDefinition:    expandPieChartStackDefinition(pieChart.StackDefinition),
			LabelDefinition:    expandLabelDefinition(pieChart.LabelDefinition),
			ShowLegend:         typeBoolToBoolPointer(pieChart.ShowLegend),
			GroupNameTemplate:  utils.TypeStringToStringPointer(pieChart.GroupNameTemplate),
			Unit:               dashboardwidgets.OptionalEnumPointer(pieChart.Unit, dashboardwidgets.DashboardSchemaToProtoUnit),
			ColorScheme:        utils.TypeStringToStringPointer(pieChart.ColorScheme),
			DataModeType:       dashboardwidgets.OptionalEnumPointer(pieChart.DataModeType, dashboardwidgets.DashboardSchemaToProtoDataModeType),
		},
	}, nil
}

func expandPieChartStackDefinition(stackDefinition *dashboardwidgets.PieChartStackDefinitionModel) *dashboardservice.PieChartStackDefinition {
	if stackDefinition == nil {
		return nil
	}

	return &dashboardservice.PieChartStackDefinition{
		MaxSlicesPerStack: typeInt64ToInt32Pointer(stackDefinition.MaxSlicesPerStack),
		StackNameTemplate: utils.TypeStringToStringPointer(stackDefinition.StackNameTemplate),
	}
}

func expandBarChartStackDefinition(stackDefinition *dashboardwidgets.BarChartStackDefinitionModel) *dashboardservice.BarChartStackDefinition {
	if stackDefinition == nil {
		return nil
	}

	return &dashboardservice.BarChartStackDefinition{
		MaxSlicesPerBar:   typeInt64ToInt32Pointer(stackDefinition.MaxSlicesPerBar),
		StackNameTemplate: utils.TypeStringToStringPointer(stackDefinition.StackNameTemplate),
	}
}

func expandHorizontalBarChartStackDefinition(stackDefinition *dashboardwidgets.BarChartStackDefinitionModel) *dashboardservice.HorizontalBarChartStackDefinition {
	if stackDefinition == nil {
		return nil
	}

	return &dashboardservice.HorizontalBarChartStackDefinition{
		MaxSlicesPerBar:   typeInt64ToInt32Pointer(stackDefinition.MaxSlicesPerBar),
		StackNameTemplate: utils.TypeStringToStringPointer(stackDefinition.StackNameTemplate),
	}
}

func expandLabelDefinition(labelDefinition *dashboardwidgets.LabelDefinitionModel) *dashboardservice.WidgetsPieChartLabelDefinition {
	if labelDefinition == nil {
		return nil
	}

	return &dashboardservice.WidgetsPieChartLabelDefinition{
		LabelSource:    dashboardwidgets.OptionalEnumPointer(labelDefinition.LabelSource, dashboardwidgets.DashboardSchemaToProtoPieChartLabelSource),
		IsVisible:      typeBoolToBoolPointer(labelDefinition.IsVisible),
		ShowName:       typeBoolToBoolPointer(labelDefinition.ShowName),
		ShowValue:      typeBoolToBoolPointer(labelDefinition.ShowValue),
		ShowPercentage: typeBoolToBoolPointer(labelDefinition.ShowPercentage),
	}
}

func expandGauge(ctx context.Context, gauge *dashboardwidgets.GaugeModel) (*dashboardservice.WidgetDefinition, diag.Diagnostics) {
	var diags diag.Diagnostics

	query, diags := expandGaugeQuery(ctx, gauge.Query)
	if diags.HasError() {
		return nil, diags
	}

	thresholds, diags := expandGaugeThresholds(ctx, gauge.Thresholds)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.WidgetDefinition{
		Gauge: &dashboardservice.WidgetsGauge{
			Query:             query,
			Min:               typeFloat64ToFloat64Pointer(gauge.Min),
			Max:               typeFloat64ToFloat64Pointer(gauge.Max),
			ShowInnerArc:      typeBoolToBoolPointer(gauge.ShowInnerArc),
			ShowOuterArc:      typeBoolToBoolPointer(gauge.ShowOuterArc),
			Unit:              dashboardwidgets.OptionalEnumPointer(gauge.Unit, dashboardwidgets.DashboardSchemaToProtoGaugeUnit),
			Thresholds:        thresholds,
			DataModeType:      dashboardwidgets.OptionalEnumPointer(gauge.DataModeType, dashboardwidgets.DashboardSchemaToProtoDataModeType),
			ThresholdBy:       dashboardwidgets.OptionalEnumPointer(gauge.ThresholdBy, dashboardwidgets.DashboardSchemaToProtoGaugeThresholdBy),
			ThresholdType:     dashboardwidgets.OptionalEnumPointer(gauge.ThresholdType, dashboardwidgets.DashboardSchemaToProtoThresholdType),
			DisplaySeriesName: typeBoolToBoolPointer(gauge.DisplaySeriesName),
			Decimal:           typeNumberToInt32Pointer(gauge.Decimal),
		},
	}, nil
}

func expandGaugeThresholds(ctx context.Context, gaugeThresholds types.List) ([]dashboardservice.GaugeThreshold, diag.Diagnostics) {
	var gaugeThresholdsObjects []types.Object
	var expandedGaugeThresholds []dashboardservice.GaugeThreshold
	diags := gaugeThresholds.ElementsAs(ctx, &gaugeThresholdsObjects, true)
	if diags.HasError() {
		return nil, diags
	}
	for _, gto := range gaugeThresholdsObjects {
		var gaugeThreshold dashboardwidgets.GaugeThresholdModel
		if dg := gto.As(ctx, &gaugeThreshold, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expandedGaugeThreshold := expandGaugeThreshold(&gaugeThreshold)
		if expandedGaugeThreshold != nil {
			expandedGaugeThresholds = append(expandedGaugeThresholds, *expandedGaugeThreshold)
		}
	}

	return expandedGaugeThresholds, diags
}

func expandGaugeThreshold(gaugeThresholds *dashboardwidgets.GaugeThresholdModel) *dashboardservice.GaugeThreshold {
	if gaugeThresholds == nil {
		return nil
	}
	return &dashboardservice.GaugeThreshold{
		From:  typeFloat64ToFloat64Pointer(gaugeThresholds.From),
		Color: utils.TypeStringToStringPointer(gaugeThresholds.Color),
		Label: utils.TypeStringToStringPointer(gaugeThresholds.Label),
	}
}

func expandGaugeQuery(ctx context.Context, gaugeQuery *dashboardwidgets.GaugeQueryModel) (*dashboardservice.GaugeQuery, diag.Diagnostics) {
	switch {
	case gaugeQuery.Metrics != nil:
		metricQuery, diags := expandGaugeQueryMetrics(ctx, gaugeQuery.Metrics)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboardservice.GaugeQuery{
			Metrics: metricQuery,
		}, nil
	case gaugeQuery.Logs != nil:
		logQuery, diags := expandGaugeQueryLogs(ctx, gaugeQuery.Logs)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboardservice.GaugeQuery{
			Logs: logQuery,
		}, nil
	case gaugeQuery.Spans != nil:
		spanQuery, diags := expandGaugeQuerySpans(ctx, gaugeQuery.Spans)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboardservice.GaugeQuery{
			Spans: spanQuery,
		}, nil
	case gaugeQuery.DataPrime != nil:
		dataprimeQuery, diags := expandGaugeQueryDataPrime(ctx, gaugeQuery.DataPrime)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboardservice.GaugeQuery{
			Dataprime: dataprimeQuery,
		}, nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Extract Gauge Query Error", fmt.Sprintf("Unknown gauge query type %#v", gaugeQuery))}
	}
}

func expandGaugeQuerySpans(ctx context.Context, gaugeQuerySpans *dashboardwidgets.GaugeQuerySpansModel) (*dashboardservice.GaugeSpansQuery, diag.Diagnostics) {
	if gaugeQuerySpans == nil {
		return nil, nil
	}
	filters, diags := dashboardwidgets.ExpandSpansFilters(ctx, gaugeQuerySpans.Filters)
	if diags.HasError() {
		return nil, diags
	}

	spansAggregation, dg := dashboardwidgets.ExpandSpansAggregation(gaugeQuerySpans.SpansAggregation)
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	timeFrame, diags := dashboardwidgets.ExpandTimeFrameSelect(ctx, gaugeQuerySpans.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.GaugeSpansQuery{
		LuceneQuery:      dashboardwidgets.ExpandLuceneQuery(gaugeQuerySpans.LuceneQuery),
		SpansAggregation: spansAggregation,
		Filters:          filters,
		TimeFrame:        timeFrame,
	}, nil
}

func expandGaugeQueryDataPrime(ctx context.Context, dataPrime *dashboardwidgets.DataPrimeModel) (*dashboardservice.GaugeDataprimeQuery, diag.Diagnostics) {
	if dataPrime == nil {
		return nil, nil
	}
	filters, diags := dashboardwidgets.ExpandDashboardFiltersSources(ctx, dataPrime.Filters)
	if diags.HasError() {
		return nil, diags
	}
	timeFrame, diags := dashboardwidgets.ExpandTimeFrameSelect(ctx, dataPrime.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}
	dataprimeQuery := &dashboardservice.CommonDataprimeQuery{
		Text: utils.TypeStringToStringPointer(dataPrime.Query),
	}
	return &dashboardservice.GaugeDataprimeQuery{
		DataprimeQuery: dataprimeQuery,
		Filters:        filters,
		TimeFrame:      timeFrame,
	}, nil
}

func expandMultiSelectSourceQuery(ctx context.Context, sourceQuery types.Object) (*dashboardservice.MultiSelectSource, diag.Diagnostics) {
	if sourceQuery.IsNull() || sourceQuery.IsUnknown() {
		return nil, nil
	}

	var queryObject VariableMultiSelectQueryModel
	if diags := sourceQuery.As(ctx, &queryObject, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	query, diags := expandMultiSelectQuery(ctx, queryObject.Query)
	if diags.HasError() {
		return nil, diags
	}

	valueDisplayOptions, diags := expandMultiSelectValueDisplayOptions(ctx, queryObject.ValueDisplayOptions)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.MultiSelectSource{
		Query: &dashboardservice.MultiSelectQuerySource{
			Query:               query,
			RefreshStrategy:     dashboardwidgets.OptionalEnumPointer(queryObject.RefreshStrategy, dashboardwidgets.DashboardSchemaToProtoRefreshStrategy),
			ValueDisplayOptions: valueDisplayOptions,
		},
	}, nil
}

func expandMultiSelectQuery(ctx context.Context, query types.Object) (*dashboardservice.MultiSelectQuery, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(query) {
		return nil, nil
	}

	var queryObject MultiSelectQueryModel
	diags := query.As(ctx, &queryObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	multiSelectQuery := &dashboardservice.MultiSelectQuery{}
	switch {
	case !utils.ObjIsNullOrUnknown(queryObject.Metrics):
		multiSelectQuery.MetricsQuery, diags = expandMultiSelectMetricsQuery(ctx, queryObject.Metrics)
	case !utils.ObjIsNullOrUnknown(queryObject.Logs):
		multiSelectQuery.LogsQuery, diags = expandMultiSelectLogsQuery(ctx, queryObject.Logs)
	case !utils.ObjIsNullOrUnknown(queryObject.Spans):
		multiSelectQuery.SpansQuery, diags = expandMultiSelectSpansQuery(ctx, queryObject.Spans)
	default:
		diags = diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand MultiSelect Query", "MultiSelect Query must be either Metrics, Logs or Spans")}
	}

	if diags.HasError() {
		return nil, diags
	}

	return multiSelectQuery, nil
}

func expandMultiSelectValueDisplayOptions(ctx context.Context, options types.Object) (*dashboardservice.MultiSelectValueDisplayOptions, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(options) {
		return nil, nil
	}

	var optionsObject MultiSelectValueDisplayOptionsModel
	diags := options.As(ctx, &optionsObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.MultiSelectValueDisplayOptions{
		ValueRegex: utils.TypeStringToStringPointer(optionsObject.ValueRegex),
		LabelRegex: utils.TypeStringToStringPointer(optionsObject.LabelRegex),
	}, nil
}

func expandMultiSelectLogsQuery(ctx context.Context, logs types.Object) (*dashboardservice.QueryLogsQuery, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(logs) {
		return nil, nil
	}

	var logsObject MultiSelectLogsQueryModel
	diags := logs.As(ctx, &logsObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	logsQuery := &dashboardservice.QueryLogsQuery{Type: &dashboardservice.QueryLogsQueryType{}}

	switch {
	case !(logsObject.FieldName.IsNull() || logsObject.FieldName.IsUnknown()):
		logsQuery.Type.FieldName, diags = expandMultiSelectLogsQueryTypeFieldName(ctx, logsObject.FieldName)
	case !(logsObject.FieldValue.IsNull() || logsObject.FieldValue.IsUnknown()):
		logsQuery.Type.FieldValue, diags = expandMultiSelectLogsQueryTypFieldValue(ctx, logsObject.FieldValue)
	}

	if diags.HasError() {
		return nil, diags
	}

	return logsQuery, nil
}

func expandMultiSelectLogsQueryTypeFieldName(ctx context.Context, name types.Object) (*dashboardservice.QueryLogsQueryTypeFieldName, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(name) {
		return nil, nil
	}

	var nameObject LogFieldNameModel
	diags := name.As(ctx, &nameObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.QueryLogsQueryTypeFieldName{
		LogRegex: utils.TypeStringToStringPointer(nameObject.LogRegex),
	}, nil
}

func expandMultiSelectLogsQueryTypFieldValue(ctx context.Context, value types.Object) (*dashboardservice.QueryLogsQueryTypeFieldValue, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(value) {
		return nil, nil
	}

	var valueObject FieldValueModel
	diags := value.As(ctx, &valueObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	observationField, diags := dashboardwidgets.ExpandObservationFieldObject(ctx, valueObject.ObservationField)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.QueryLogsQueryTypeFieldValue{
		ObservationField: observationField,
	}, nil
}

func expandMultiSelectMetricsQuery(ctx context.Context, metrics types.Object) (*dashboardservice.QueryMetricsQuery, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(metrics) {
		return nil, nil
	}

	var metricsObject MultiSelectMetricsQueryModel
	diags := metrics.As(ctx, &metricsObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	metricsQuery := &dashboardservice.QueryMetricsQuery{Type: &dashboardservice.QueryMetricsQueryType{}}

	switch {
	case !utils.ObjIsNullOrUnknown(metricsObject.MetricName):
		metricsQuery.Type.MetricName, diags = expandMultiSelectMetricsQueryTypeMetricName(ctx, metricsObject.MetricName)
	case !utils.ObjIsNullOrUnknown(metricsObject.LabelName):
		metricsQuery.Type.LabelName, diags = expandMultiSelectMetricsQueryTypeLabelName(ctx, metricsObject.LabelName)
	case !utils.ObjIsNullOrUnknown(metricsObject.LabelValue):
		metricsQuery.Type.LabelValue, diags = expandMultiSelectMetricsQueryTypeLabelValue(ctx, metricsObject.LabelValue)
	}

	if diags.HasError() {
		return nil, diags
	}

	return metricsQuery, nil
}

func expandMultiSelectMetricsQueryTypeMetricName(ctx context.Context, name types.Object) (*dashboardservice.QueryMetricsQueryTypeMetricName, diag.Diagnostics) {
	if name.IsNull() || name.IsUnknown() {
		return nil, nil
	}

	var nameObject MetricAndLabelNameModel
	diags := name.As(ctx, &nameObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.QueryMetricsQueryTypeMetricName{
		MetricRegex: utils.TypeStringToStringPointer(nameObject.MetricRegex),
	}, nil
}

func expandMultiSelectMetricsQueryTypeLabelName(ctx context.Context, name types.Object) (*dashboardservice.QueryMetricsQueryTypeLabelName, diag.Diagnostics) {
	if name.IsNull() || name.IsUnknown() {
		return nil, nil
	}

	var nameObject MetricAndLabelNameModel
	diags := name.As(ctx, &nameObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.QueryMetricsQueryTypeLabelName{
		MetricRegex: utils.TypeStringToStringPointer(nameObject.MetricRegex),
	}, nil
}

func expandMultiSelectMetricsQueryTypeLabelValue(ctx context.Context, value types.Object) (*dashboardservice.QueryMetricsQueryTypeLabelValue, diag.Diagnostics) {
	if value.IsNull() || value.IsUnknown() {
		return nil, nil
	}

	var valueObject LabelValueModel
	diags := value.As(ctx, &valueObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	metricName, diags := expandStringOrVariable(ctx, valueObject.MetricName)
	if diags.HasError() {
		return nil, diags
	}

	labelName, diags := expandStringOrVariable(ctx, valueObject.LabelName)
	if diags.HasError() {
		return nil, diags
	}

	labelFilters, diags := expandMetricsLabelFilters(ctx, valueObject.LabelFilters)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.QueryMetricsQueryTypeLabelValue{
		MetricName:   metricName,
		LabelName:    labelName,
		LabelFilters: labelFilters,
	}, nil
}

func expandStringOrVariables(ctx context.Context, name types.List) ([]dashboardservice.QueryMetricsQueryStringOrVariable, diag.Diagnostics) {
	var nameObjects []types.Object
	var expandedNames []dashboardservice.QueryMetricsQueryStringOrVariable
	diags := name.ElementsAs(ctx, &nameObjects, true)
	if diags.HasError() {
		return nil, diags
	}

	for _, no := range nameObjects {
		expandedName, expandDiags := expandStringOrVariable(ctx, no)
		if expandDiags.HasError() {
			diags.Append(expandDiags...)
			continue
		}
		expandedNames = append(expandedNames, *expandedName)
	}

	if diags.HasError() {
		return nil, diags
	}

	return expandedNames, nil
}

func expandStringOrVariable(ctx context.Context, name types.Object) (*dashboardservice.QueryMetricsQueryStringOrVariable, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(name) {
		return nil, nil
	}

	var nameObject MetricLabelFilterOperatorSelectedValuesModel
	diags := name.As(ctx, &nameObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	switch {
	case !(nameObject.VariableName.IsNull() || nameObject.VariableName.IsUnknown()):
		return &dashboardservice.QueryMetricsQueryStringOrVariable{
			VariableName: utils.TypeStringToStringPointer(nameObject.VariableName),
		}, nil
	case !(nameObject.StringValue.IsNull() || nameObject.StringValue.IsUnknown()):
		return &dashboardservice.QueryMetricsQueryStringOrVariable{
			StringValue: utils.TypeStringToStringPointer(nameObject.StringValue),
		}, nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand StringOrVariable", "StringOrVariable must be either VariableName or StringValue")}
	}
}

func expandMetricsLabelFilters(ctx context.Context, filters types.List) ([]dashboardservice.QueryMetricsQueryMetricsLabelFilter, diag.Diagnostics) {
	var filtersObjects []types.Object
	var expandedFilters []dashboardservice.QueryMetricsQueryMetricsLabelFilter
	diags := filters.ElementsAs(ctx, &filtersObjects, true)
	if diags.HasError() {
		return nil, diags
	}

	for _, fo := range filtersObjects {
		var filter MetricLabelFilterModel
		if dg := fo.As(ctx, &filter, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expandedFilter, expandDiags := expandMetricLabelFilter(ctx, filter)
		if expandDiags.HasError() {
			diags.Append(expandDiags...)
			continue
		}
		expandedFilters = append(expandedFilters, *expandedFilter)
	}

	if diags.HasError() {
		return nil, diags
	}

	return expandedFilters, nil
}

func expandMetricLabelFilter(ctx context.Context, filter MetricLabelFilterModel) (*dashboardservice.QueryMetricsQueryMetricsLabelFilter, diag.Diagnostics) {
	metric, diags := expandStringOrVariable(ctx, filter.Metric)
	if diags.HasError() {
		return nil, diags
	}

	label, diags := expandStringOrVariable(ctx, filter.Label)
	if diags.HasError() {
		return nil, diags
	}

	operator, diags := expandMetricLabelFilterOperator(ctx, filter.Operator)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.QueryMetricsQueryMetricsLabelFilter{
		Metric:   metric,
		Label:    label,
		Operator: operator,
	}, nil
}

func expandMetricLabelFilterOperator(ctx context.Context, operator types.Object) (*dashboardservice.QueryMetricsQueryOperator, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(operator) {
		return nil, nil
	}

	var operatorObject MetricLabelFilterOperatorModel
	diags := operator.As(ctx, &operatorObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	values, diags := expandStringOrVariables(ctx, operatorObject.SelectedValues)
	if diags.HasError() {
		return nil, diags
	}

	selection := &dashboardservice.QueryMetricsQuerySelection{
		List: &dashboardservice.QueryMetricsQuerySelectionListSelection{
			Values: values,
		},
	}
	switch operatorObject.Type.ValueString() {
	case "equals":
		return &dashboardservice.QueryMetricsQueryOperator{
			Equals: &dashboardservice.QueryMetricsQueryEquals{
				Selection: selection,
			},
		}, nil
	case "not_equals":
		return &dashboardservice.QueryMetricsQueryOperator{
			NotEquals: &dashboardservice.QueryMetricsQueryNotEquals{
				Selection: selection,
			},
		}, nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand MetricLabelFilterOperator", fmt.Sprintf("Unknown operator type %s", operatorObject.Type.ValueString()))}
	}
}

func expandMultiSelectSpansQuery(ctx context.Context, spans types.Object) (*dashboardservice.QuerySpansQuery, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(spans) {
		return nil, nil
	}

	var spansObject MultiSelectSpansQueryModel
	diags := spans.As(ctx, &spansObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	spansQuery := &dashboardservice.QuerySpansQuery{Type: &dashboardservice.QuerySpansQueryType{}}

	switch {
	case !utils.ObjIsNullOrUnknown(spansObject.FieldName):
		spansQuery.Type.FieldName, diags = expandMultiSelectSpansQueryTypeFieldName(ctx, spansObject.FieldName)
	case !utils.ObjIsNullOrUnknown(spansObject.FieldValue):
		spansQuery.Type.FieldValue, diags = expandMultiSelectSpansQueryTypeFieldValue(ctx, spansObject.FieldValue)
	default:
		diags = diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand MultiSelect Spans Query", "MultiSelect Spans Query must be either FieldName or FieldValue")}
	}

	if diags.HasError() {
		return nil, diags
	}

	return spansQuery, nil
}

func expandMultiSelectSpansQueryTypeFieldName(ctx context.Context, name types.Object) (*dashboardservice.QuerySpansQueryTypeFieldName, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(name) {
		return nil, nil
	}

	var nameObject SpanFieldNameModel
	diags := name.As(ctx, &nameObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.QuerySpansQueryTypeFieldName{
		SpanRegex: utils.TypeStringToStringPointer(nameObject.SpanRegex),
	}, nil
}

func expandMultiSelectSpansQueryTypeFieldValue(ctx context.Context, value types.Object) (*dashboardservice.QuerySpansQueryTypeFieldValue, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(value) {
		return nil, nil
	}

	var valueObject dashboardwidgets.SpansFieldModel
	diags := value.As(ctx, &valueObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	spansField, dgs := dashboardwidgets.ExpandSpansField(&valueObject)
	if dgs != nil {
		return nil, diag.Diagnostics{dgs}
	}

	return &dashboardservice.QuerySpansQueryTypeFieldValue{
		Value: spansField,
	}, nil
}

func expandGaugeQueryMetrics(ctx context.Context, gaugeQueryMetrics *dashboardwidgets.GaugeQueryMetricsModel) (*dashboardservice.GaugeMetricsQuery, diag.Diagnostics) {
	filters, diags := dashboardwidgets.ExpandMetricsFilters(ctx, gaugeQueryMetrics.Filters)
	if diags.HasError() {
		return nil, diags
	}

	timeFrame, diags := dashboardwidgets.ExpandTimeFrameSelect(ctx, gaugeQueryMetrics.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.GaugeMetricsQuery{
		PromqlQuery: dashboardwidgets.ExpandPromqlQuery(gaugeQueryMetrics.PromqlQuery),
		Aggregation: dashboardwidgets.OptionalEnumPointer(gaugeQueryMetrics.Aggregation, dashboardwidgets.DashboardSchemaToProtoGaugeAggregation),
		Filters:     filters,
		TimeFrame:   timeFrame,
	}, nil
}

func expandGaugeQueryLogs(ctx context.Context, gaugeQueryLogs *dashboardwidgets.GaugeQueryLogsModel) (*dashboardservice.GaugeLogsQuery, diag.Diagnostics) {
	logsAggregation, diags := dashboardwidgets.ExpandLogsAggregation(ctx, gaugeQueryLogs.LogsAggregation)
	if diags.HasError() {
		return nil, diags
	}

	filters, diags := dashboardwidgets.ExpandLogsFilters(ctx, gaugeQueryLogs.Filters)
	if diags.HasError() {
		return nil, diags
	}

	timeFrame, diags := dashboardwidgets.ExpandTimeFrameSelect(ctx, gaugeQueryLogs.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.GaugeLogsQuery{
		LuceneQuery:     dashboardwidgets.ExpandLuceneQuery(gaugeQueryLogs.LuceneQuery),
		LogsAggregation: logsAggregation,
		Filters:         filters,
		TimeFrame:       timeFrame,
	}, nil
}

func expandBarChart(ctx context.Context, chart *dashboardwidgets.BarChartModel) (*dashboardservice.WidgetDefinition, diag.Diagnostics) {
	var diags diag.Diagnostics

	query, diags := expandBarChartQuery(ctx, chart.Query)
	if diags.HasError() {
		return nil, diags
	}

	xaxis, dg := expandXAis(chart.XAxis)
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	return &dashboardservice.WidgetDefinition{
		BarChart: &dashboardservice.BarChart{
			Query:             query,
			MaxBarsPerChart:   typeInt64ToInt32Pointer(chart.MaxBarsPerChart),
			GroupNameTemplate: utils.TypeStringToStringPointer(chart.GroupNameTemplate),
			StackDefinition:   expandBarChartStackDefinition(chart.StackDefinition),
			ScaleType:         dashboardwidgets.OptionalEnumPointer(chart.ScaleType, dashboardwidgets.DashboardSchemaToProtoScaleType),
			ColorsBy:          expandColorsBy(chart.ColorsBy),
			XAxis:             xaxis,
			Unit:              dashboardwidgets.OptionalEnumPointer(chart.Unit, dashboardwidgets.DashboardSchemaToProtoUnit),
			SortBy:            dashboardwidgets.OptionalEnumPointer(chart.SortBy, dashboardwidgets.DashboardSchemaToProtoSortBy),
			ColorScheme:       utils.TypeStringToStringPointer(chart.ColorScheme),
			DataModeType:      dashboardwidgets.OptionalEnumPointer(chart.DataModeType, dashboardwidgets.DashboardSchemaToProtoDataModeType),
		},
	}, nil
}

func expandColorsBy(colorsBy types.String) *dashboardservice.ColorsBy {
	switch colorsBy.ValueString() {
	case "stack":
		return &dashboardservice.ColorsBy{
			Stack: map[string]interface{}{},
		}
	case "group_by":
		return &dashboardservice.ColorsBy{
			GroupBy: map[string]interface{}{},
		}
	case "aggregation":
		return &dashboardservice.ColorsBy{
			Aggregation: map[string]interface{}{},
		}
	default:
		return nil
	}
}

func expandXAis(xaxis *dashboardwidgets.BarChartXAxisModel) (*dashboardservice.XAxis, diag.Diagnostic) {
	if xaxis == nil {
		return nil, nil
	}

	switch {
	case xaxis.Time != nil:
		interval, diagnostic := dashboardwidgets.GoDurationToOpenAPI(xaxis.Time.Interval, "bar chart x axis")
		if diagnostic != nil {
			return nil, diagnostic
		}
		return &dashboardservice.XAxis{
			Time: &dashboardservice.XAxisByTime{
				Interval:         interval,
				BucketsPresented: typeInt64ToInt32Pointer(xaxis.Time.BucketsPresented),
			},
		}, nil
	case xaxis.Value != nil:
		return &dashboardservice.XAxis{
			Value: map[string]interface{}{},
		}, nil
	default:
		return nil, diag.NewErrorDiagnostic("Error expand bar chart x axis", "unknown x axis type")
	}
}
func expandBarChartQuery(ctx context.Context, query *dashboardwidgets.BarChartQueryModel) (*dashboardservice.BarChartQuery, diag.Diagnostics) {
	if query == nil {
		return nil, nil
	}
	switch {
	case !(query.Logs.IsNull() || query.Logs.IsUnknown()):
		logsQuery, diags := expandBarChartLogsQuery(ctx, query.Logs)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboardservice.BarChartQuery{
			Logs: logsQuery,
		}, nil
	case !(query.Metrics.IsNull() || query.Metrics.IsUnknown()):
		metricsQuery, diags := expandBarChartMetricsQuery(ctx, query.Metrics)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboardservice.BarChartQuery{
			Metrics: metricsQuery,
		}, nil
	case !(query.Spans.IsNull() || query.Spans.IsUnknown()):
		spansQuery, diags := expandBarChartSpansQuery(ctx, query.Spans)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboardservice.BarChartQuery{
			Spans: spansQuery,
		}, nil
	case !(query.DataPrime.IsNull() || query.DataPrime.IsUnknown()):
		dataPrimeQuery, diags := expandBarChartDataPrimeQuery(ctx, query.DataPrime)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboardservice.BarChartQuery{
			Dataprime: dataPrimeQuery,
		}, nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error expand bar chart query", "unknown bar chart query type")}
	}
}

func expandHorizontalBarChartQuery(ctx context.Context, query *dashboardwidgets.HorizontalBarChartQueryModel) (*dashboardservice.HorizontalBarChartQuery, diag.Diagnostics) {
	if query == nil {
		return nil, nil
	}
	switch {
	case !(query.Logs.IsNull() || query.Logs.IsUnknown()):
		logsQuery, diags := expandHorizontalBarChartLogsQuery(ctx, query.Logs)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboardservice.HorizontalBarChartQuery{
			Logs: logsQuery,
		}, nil
	case !(query.Metrics.IsNull() || query.Metrics.IsUnknown()):
		metricsQuery, diags := expandHorizontalBarChartMetricsQuery(ctx, query.Metrics)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboardservice.HorizontalBarChartQuery{
			Metrics: metricsQuery,
		}, nil
	case !(query.Spans.IsNull() || query.Spans.IsUnknown()):
		spansQuery, diags := expandHorizontalBarChartSpansQuery(ctx, query.Spans)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboardservice.HorizontalBarChartQuery{
			Spans: spansQuery,
		}, nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error expand bar chart query", "unknown bar chart query type")}
	}
}

func expandHorizontalBarChartLogsQuery(ctx context.Context, logs types.Object) (*dashboardservice.HorizontalBarChartLogsQuery, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(logs) {
		return nil, nil
	}

	var logsObject dashboardwidgets.BarChartQueryLogsModel
	diags := logs.As(ctx, &logsObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	aggregation, diags := dashboardwidgets.ExpandLogsAggregation(ctx, logsObject.Aggregation)
	if diags.HasError() {
		return nil, diags
	}

	filters, diags := dashboardwidgets.ExpandLogsFilters(ctx, logsObject.Filters)
	if diags.HasError() {
		return nil, diags
	}

	groupNames, diags := typeStringValuesToStringSlice(ctx, logsObject.GroupNames.Elements())
	if diags.HasError() {
		return nil, diags
	}

	timeFrame, diags := dashboardwidgets.ExpandTimeFrameSelect(ctx, logsObject.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.HorizontalBarChartLogsQuery{
		LuceneQuery:      dashboardwidgets.ExpandLuceneQuery(logsObject.LuceneQuery),
		Aggregation:      aggregation,
		Filters:          filters,
		GroupNames:       groupNames,
		StackedGroupName: utils.TypeStringToStringPointer(logsObject.StackedGroupName),
		TimeFrame:        timeFrame,
	}, nil
}

func expandHorizontalBarChartMetricsQuery(ctx context.Context, metrics types.Object) (*dashboardservice.HorizontalBarChartMetricsQuery, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(metrics) {
		return nil, nil
	}

	var metricsObject dashboardwidgets.BarChartQueryMetricsModel
	diags := metrics.As(ctx, &metricsObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	filters, diags := dashboardwidgets.ExpandMetricsFilters(ctx, metricsObject.Filters)
	if diags.HasError() {
		return nil, diags
	}

	groupNames, diags := typeStringValuesToStringSlice(ctx, metricsObject.GroupNames.Elements())
	if diags.HasError() {
		return nil, diags
	}
	timeFrame, diags := dashboardwidgets.ExpandTimeFrameSelect(ctx, metricsObject.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.HorizontalBarChartMetricsQuery{
		PromqlQuery:      dashboardwidgets.ExpandPromqlQuery(metricsObject.PromqlQuery),
		Filters:          filters,
		GroupNames:       groupNames,
		StackedGroupName: utils.TypeStringToStringPointer(metricsObject.StackedGroupName),
		TimeFrame:        timeFrame,
	}, nil
}

func expandHorizontalBarChartSpansQuery(ctx context.Context, spans types.Object) (*dashboardservice.HorizontalBarChartSpansQuery, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(spans) {
		return nil, nil
	}

	var spansObject dashboardwidgets.BarChartQuerySpansModel
	diags := spans.As(ctx, &spansObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	aggregation, dg := dashboardwidgets.ExpandSpansAggregation(spansObject.Aggregation)
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	filters, diags := dashboardwidgets.ExpandSpansFilters(ctx, spansObject.Filters)
	if diags.HasError() {
		return nil, diags
	}

	groupNames, diags := dashboardwidgets.ExpandSpansFields(ctx, spansObject.GroupNames)
	if diags.HasError() {
		return nil, diags
	}

	expandedFilter, dg := dashboardwidgets.ExpandSpansField(spansObject.StackedGroupName)
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	timeFrame, diags := dashboardwidgets.ExpandTimeFrameSelect(ctx, spansObject.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.HorizontalBarChartSpansQuery{
		LuceneQuery:      dashboardwidgets.ExpandLuceneQuery(spansObject.LuceneQuery),
		Aggregation:      aggregation,
		Filters:          filters,
		GroupNames:       groupNames,
		StackedGroupName: expandedFilter,
		TimeFrame:        timeFrame,
	}, nil
}

func expandBarChartLogsQuery(ctx context.Context, barChartQueryLogs types.Object) (*dashboardservice.BarChartLogsQuery, diag.Diagnostics) {
	if barChartQueryLogs.IsNull() || barChartQueryLogs.IsUnknown() {
		return nil, nil
	}

	var barChartQueryLogsObject dashboardwidgets.BarChartQueryLogsModel
	diags := barChartQueryLogs.As(ctx, &barChartQueryLogsObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	aggregation, diags := dashboardwidgets.ExpandLogsAggregation(ctx, barChartQueryLogsObject.Aggregation)
	if diags.HasError() {
		return nil, diags
	}

	filters, diags := dashboardwidgets.ExpandLogsFilters(ctx, barChartQueryLogsObject.Filters)
	if diags.HasError() {
		return nil, diags
	}

	groupNames, diags := typeStringValuesToStringSlice(ctx, barChartQueryLogsObject.GroupNames.Elements())
	if diags.HasError() {
		return nil, diags
	}

	groupNamesFields, diags := dashboardwidgets.ExpandObservationFields(ctx, barChartQueryLogsObject.GroupNamesFields)
	if diags.HasError() {
		return nil, diags
	}

	stackedGroupNameField, diags := dashboardwidgets.ExpandObservationFieldObject(ctx, barChartQueryLogsObject.StackedGroupNameField)
	if diags.HasError() {
		return nil, diags
	}

	timeFrame, diags := dashboardwidgets.ExpandTimeFrameSelect(ctx, barChartQueryLogsObject.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.BarChartLogsQuery{
		LuceneQuery:           dashboardwidgets.ExpandLuceneQuery(barChartQueryLogsObject.LuceneQuery),
		Aggregation:           aggregation,
		Filters:               filters,
		GroupNames:            groupNames,
		StackedGroupName:      utils.TypeStringToStringPointer(barChartQueryLogsObject.StackedGroupName),
		GroupNamesFields:      groupNamesFields,
		StackedGroupNameField: stackedGroupNameField,
		TimeFrame:             timeFrame,
	}, nil
}

func expandBarChartMetricsQuery(ctx context.Context, barChartQueryMetrics types.Object) (*dashboardservice.BarChartMetricsQuery, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(barChartQueryMetrics) {
		return nil, nil
	}

	var barChartQueryMetricsObject dashboardwidgets.BarChartQueryMetricsModel
	diags := barChartQueryMetrics.As(ctx, &barChartQueryMetricsObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	filters, diags := dashboardwidgets.ExpandMetricsFilters(ctx, barChartQueryMetricsObject.Filters)
	if diags.HasError() {
		return nil, diags
	}

	groupNames, diags := typeStringValuesToStringSlice(ctx, barChartQueryMetricsObject.GroupNames.Elements())
	if diags.HasError() {
		return nil, diags
	}

	timeFrame, diags := dashboardwidgets.ExpandTimeFrameSelect(ctx, barChartQueryMetricsObject.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.BarChartMetricsQuery{
		PromqlQuery:      dashboardwidgets.ExpandPromqlQuery(barChartQueryMetricsObject.PromqlQuery),
		Filters:          filters,
		GroupNames:       groupNames,
		StackedGroupName: utils.TypeStringToStringPointer(barChartQueryMetricsObject.StackedGroupName),
		TimeFrame:        timeFrame,
	}, nil
}

func expandBarChartSpansQuery(ctx context.Context, barChartQuerySpans types.Object) (*dashboardservice.BarChartSpansQuery, diag.Diagnostics) {
	if barChartQuerySpans.IsNull() || barChartQuerySpans.IsUnknown() {
		return nil, nil
	}

	var barChartQuerySpansObject dashboardwidgets.BarChartQuerySpansModel
	diags := barChartQuerySpans.As(ctx, &barChartQuerySpansObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	aggregation, dg := dashboardwidgets.ExpandSpansAggregation(barChartQuerySpansObject.Aggregation)
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	filters, diags := dashboardwidgets.ExpandSpansFilters(ctx, barChartQuerySpansObject.Filters)
	if diags.HasError() {
		return nil, diags
	}

	groupNames, diags := dashboardwidgets.ExpandSpansFields(ctx, barChartQuerySpansObject.GroupNames)
	if diags.HasError() {
		return nil, diags
	}

	expandedFilter, dg := dashboardwidgets.ExpandSpansField(barChartQuerySpansObject.StackedGroupName)
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	timeFrame, diags := dashboardwidgets.ExpandTimeFrameSelect(ctx, barChartQuerySpansObject.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.BarChartSpansQuery{
		LuceneQuery:      dashboardwidgets.ExpandLuceneQuery(barChartQuerySpansObject.LuceneQuery),
		Aggregation:      aggregation,
		Filters:          filters,
		GroupNames:       groupNames,
		StackedGroupName: expandedFilter,
		TimeFrame:        timeFrame,
	}, nil
}

func expandBarChartDataPrimeQuery(ctx context.Context, dataPrime types.Object) (*dashboardservice.BarChartDataprimeQuery, diag.Diagnostics) {
	if dataPrime.IsNull() || dataPrime.IsUnknown() {
		return nil, nil
	}

	var dataPrimeObject dashboardwidgets.BarChartQueryDataPrimeModel
	diags := dataPrime.As(ctx, &dataPrimeObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	filters, diags := dashboardwidgets.ExpandDashboardFiltersSources(ctx, dataPrimeObject.Filters)
	if diags.HasError() {
		return nil, diags
	}

	groupNames, diags := typeStringValuesToStringSlice(ctx, dataPrimeObject.GroupNames.Elements())
	if diags.HasError() {
		return nil, diags
	}

	timeFrame, diags := dashboardwidgets.ExpandTimeFrameSelect(ctx, dataPrimeObject.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	dataPrimeQuery := &dashboardservice.CommonDataprimeQuery{
		Text: utils.TypeStringToStringPointer(dataPrimeObject.Query),
	}
	return &dashboardservice.BarChartDataprimeQuery{
		Filters:          filters,
		DataprimeQuery:   dataPrimeQuery,
		GroupNames:       groupNames,
		StackedGroupName: utils.TypeStringToStringPointer(dataPrimeObject.StackedGroupName),
		TimeFrame:        timeFrame,
	}, nil
}

func expandPieChartQuery(ctx context.Context, pieChartQuery *dashboardwidgets.PieChartQueryModel) (*dashboardservice.PieChartQuery, diag.Diagnostics) {
	if pieChartQuery == nil {
		return nil, nil
	}

	switch {
	case pieChartQuery.Logs != nil:
		logs, diags := expandPieChartLogsQuery(ctx, pieChartQuery.Logs)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboardservice.PieChartQuery{
			Logs: logs,
		}, nil
	case pieChartQuery.Metrics != nil:
		metrics, diags := expandPieChartMetricsQuery(ctx, pieChartQuery.Metrics)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboardservice.PieChartQuery{
			Metrics: metrics,
		}, nil
	case pieChartQuery.Spans != nil:
		spans, diags := expandPieChartSpansQuery(ctx, pieChartQuery.Spans)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboardservice.PieChartQuery{
			Spans: spans,
		}, nil
	case pieChartQuery.DataPrime != nil:
		dataPrime, diags := expandPieChartDataPrimeQuery(ctx, pieChartQuery.DataPrime)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboardservice.PieChartQuery{
			Dataprime: dataPrime,
		}, nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand PieChart Query", "Unknown PieChart Query type")}
	}
}

func expandPieChartLogsQuery(ctx context.Context, pieChartQueryLogs *dashboardwidgets.PieChartQueryLogsModel) (*dashboardservice.PieChartLogsQuery, diag.Diagnostics) {
	if pieChartQueryLogs == nil {
		return nil, nil
	}

	aggregation, diags := dashboardwidgets.ExpandLogsAggregation(ctx, pieChartQueryLogs.Aggregation)
	if diags.HasError() {
		return nil, diags
	}

	filters, diags := dashboardwidgets.ExpandLogsFilters(ctx, pieChartQueryLogs.Filters)
	if diags.HasError() {
		return nil, diags
	}

	groupNames, diags := stringValuesFromList(ctx, pieChartQueryLogs.GroupNames)
	if diags.HasError() {
		return nil, diags
	}

	groupNamesFields, diags := dashboardwidgets.ExpandObservationFields(ctx, pieChartQueryLogs.GroupNamesFields)
	if diags.HasError() {
		return nil, diags
	}

	stackedGroupNameField, diags := dashboardwidgets.ExpandObservationFieldObject(ctx, pieChartQueryLogs.StackedGroupNameField)
	if diags.HasError() {
		return nil, diags
	}

	timeFrame, diags := dashboardwidgets.ExpandTimeFrameSelect(ctx, pieChartQueryLogs.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.PieChartLogsQuery{
		LuceneQuery:           dashboardwidgets.ExpandLuceneQuery(pieChartQueryLogs.LuceneQuery),
		Aggregation:           aggregation,
		Filters:               filters,
		GroupNames:            groupNames,
		StackedGroupName:      utils.TypeStringToStringPointer(pieChartQueryLogs.StackedGroupName),
		GroupNamesFields:      groupNamesFields,
		StackedGroupNameField: stackedGroupNameField,
		TimeFrame:             timeFrame,
	}, nil
}

func expandPieChartMetricsQuery(ctx context.Context, pieChartQueryMetrics *dashboardwidgets.PieChartQueryMetricsModel) (*dashboardservice.PieChartMetricsQuery, diag.Diagnostics) {
	if pieChartQueryMetrics == nil {
		return nil, nil
	}

	filters, diags := dashboardwidgets.ExpandMetricsFilters(ctx, pieChartQueryMetrics.Filters)
	if diags.HasError() {
		return nil, diags
	}

	groupNames, diags := stringValuesFromList(ctx, pieChartQueryMetrics.GroupNames)
	if diags.HasError() {
		return nil, diags
	}

	timeFrame, diags := dashboardwidgets.ExpandTimeFrameSelect(ctx, pieChartQueryMetrics.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.PieChartMetricsQuery{
		PromqlQuery:      dashboardwidgets.ExpandPromqlQuery(pieChartQueryMetrics.PromqlQuery),
		GroupNames:       groupNames,
		Filters:          filters,
		StackedGroupName: utils.TypeStringToStringPointer(pieChartQueryMetrics.StackedGroupName),
		TimeFrame:        timeFrame,
	}, nil
}

func expandPieChartSpansQuery(ctx context.Context, pieChartQuerySpans *dashboardwidgets.PieChartQuerySpansModel) (*dashboardservice.PieChartSpansQuery, diag.Diagnostics) {
	if pieChartQuerySpans == nil {
		return nil, nil
	}

	aggregation, dg := dashboardwidgets.ExpandSpansAggregation(pieChartQuerySpans.Aggregation)
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	filters, diags := dashboardwidgets.ExpandSpansFilters(ctx, pieChartQuerySpans.Filters)
	if diags.HasError() {
		return nil, diags
	}

	groupNames, diags := dashboardwidgets.ExpandSpansFields(ctx, pieChartQuerySpans.GroupNames)
	if diags.HasError() {
		return nil, diags
	}

	stackedGroupName, dg := dashboardwidgets.ExpandSpansField(pieChartQuerySpans.StackedGroupName)
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	timeFrame, diags := dashboardwidgets.ExpandTimeFrameSelect(ctx, pieChartQuerySpans.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.PieChartSpansQuery{
		LuceneQuery:      dashboardwidgets.ExpandLuceneQuery(pieChartQuerySpans.LuceneQuery),
		Aggregation:      aggregation,
		Filters:          filters,
		GroupNames:       groupNames,
		StackedGroupName: stackedGroupName,
		TimeFrame:        timeFrame,
	}, nil
}

func expandPieChartDataPrimeQuery(ctx context.Context, dataPrime *dashboardwidgets.PieChartQueryDataPrimeModel) (*dashboardservice.PieChartDataprimeQuery, diag.Diagnostics) {
	if dataPrime == nil {
		return nil, nil
	}

	filters, diags := dashboardwidgets.ExpandDashboardFiltersSources(ctx, dataPrime.Filters)
	if diags.HasError() {
		return nil, diags
	}

	groupNames, diags := stringValuesFromList(ctx, dataPrime.GroupNames)
	if diags.HasError() {
		return nil, diags
	}

	timeFrame, diags := dashboardwidgets.ExpandTimeFrameSelect(ctx, dataPrime.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.PieChartDataprimeQuery{
		DataprimeQuery: &dashboardservice.CommonDataprimeQuery{
			Text: utils.TypeStringToStringPointer(dataPrime.Query),
		},
		Filters:          filters,
		GroupNames:       groupNames,
		StackedGroupName: utils.TypeStringToStringPointer(dataPrime.StackedGroupName),
		TimeFrame:        timeFrame,
	}, nil
}

func expandDashboardVariables(ctx context.Context, variables types.List) ([]dashboardservice.Variable, diag.Diagnostics) {
	var variablesObjects []types.Object
	var expandedVariables []dashboardservice.Variable
	diags := variables.ElementsAs(ctx, &variablesObjects, true)
	if diags.HasError() {
		return nil, diags
	}
	for _, vo := range variablesObjects {
		var variable DashboardVariableModel
		if dg := vo.As(ctx, &variable, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expandedVariable, expandDiags := expandDashboardVariable(ctx, variable)
		if expandDiags.HasError() {
			diags.Append(expandDiags...)
			continue
		}
		expandedVariables = append(expandedVariables, *expandedVariable)
	}

	return expandedVariables, diags
}

func expandDashboardVariable(ctx context.Context, variable DashboardVariableModel) (*dashboardservice.Variable, diag.Diagnostics) {
	definition, diags := expandDashboardVariableDefinition(ctx, variable.Definition)
	if diags.HasError() {
		return nil, diags
	}
	return &dashboardservice.Variable{
		Name:        utils.TypeStringToStringPointer(variable.Name),
		DisplayName: utils.TypeStringToStringPointer(variable.DisplayName),
		Definition:  definition,
	}, nil
}

func expandDashboardVariableDefinition(ctx context.Context, definition *DashboardVariableDefinitionModel) (*dashboardservice.VariableDefinition, diag.Diagnostics) {
	if definition == nil {
		return nil, nil
	}

	switch {
	case definition.MultiSelect != nil:
		return expandMultiSelect(ctx, definition.MultiSelect)
	case !definition.ConstantValue.IsNull():
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic(
			"Deprecated dashboard variable definition: constant_value",
			"`constant_value` is deprecated and is rejected by the Coralogix API. Define the variable as a "+
				"`multi_select` with a `constant_list` source and a single selected value instead:\n\n"+
				"  multi_select = {\n"+
				"    source                 = { constant_list = [\""+definition.ConstantValue.ValueString()+"\"] }\n"+
				"    selected_values        = [\""+definition.ConstantValue.ValueString()+"\"]\n"+
				"    values_order_direction = \"asc\"\n"+
				"  }",
		)}
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand Dashboard Variable", fmt.Sprintf("unknown variable definition type: %T", definition))}
	}
}

// Wire values of variables.MultiSelect_VariableSelectionOptions_SelectionType (proto-stable).
const (
	multiSelectSelectionTypeMulti  = 2
	multiSelectSelectionTypeSingle = 3
)

func expandMultiSelect(ctx context.Context, multiSelect *VariableMultiSelectModel) (*dashboardservice.VariableDefinition, diag.Diagnostics) {
	if multiSelect == nil {
		return nil, nil
	}

	source, diags := expandMultiSelectSource(ctx, multiSelect.Source)
	if diags.HasError() {
		return nil, diags
	}

	selection, diags := expandMultiSelectSelection(ctx, multiSelect.SelectedValues.Elements())
	if diags.HasError() {
		return nil, diags
	}

	ms := &dashboardservice.MultiSelect{
		Source:    source,
		Selection: selection,
	}
	if !multiSelect.ValuesOrderDirection.IsNull() && !multiSelect.ValuesOrderDirection.IsUnknown() {
		orderDirection := dashboardwidgets.DashboardOrderDirectionSchemaToProto[multiSelect.ValuesOrderDirection.ValueString()]
		ms.ValuesOrderDirection = orderDirection.Ptr()
	}

	if !multiSelect.SelectionType.IsNull() && !multiSelect.SelectionType.IsUnknown() {
		ms.SelectionOptions = utils.NewLike(ms.SelectionOptions)
		switch multiSelect.SelectionType.ValueString() {
		case "multi":
			ms.SelectionOptions.SelectionType = dashboardservice.SELECTIONTYPE_SELECTION_TYPE_MULTI.Ptr()
		case "single":
			ms.SelectionOptions.SelectionType = dashboardservice.SELECTIONTYPE_SELECTION_TYPE_SINGLE.Ptr()
		}
	}

	return &dashboardservice.VariableDefinition{
		MultiSelect: ms,
	}, nil
}

func expandMultiSelectSelection(ctx context.Context, selectedValues []attr.Value) (*dashboardservice.MultiSelectSelection, diag.Diagnostics) {
	if len(selectedValues) == 0 {
		return &dashboardservice.MultiSelectSelection{
			All: map[string]interface{}{},
		}, nil
	}

	selections, diags := typeStringValuesToStringSlice(ctx, selectedValues)
	if diags.HasError() {
		return nil, diags
	}
	return &dashboardservice.MultiSelectSelection{
		List: &dashboardservice.MultiSelectSelectionListSelection{
			Values: selections,
		},
	}, nil
}

func expandMultiSelectSource(ctx context.Context, source *VariableMultiSelectSourceModel) (*dashboardservice.MultiSelectSource, diag.Diagnostics) {
	if source == nil {
		return nil, nil
	}

	switch {
	case !source.LogsPath.IsNull():
		return &dashboardservice.MultiSelectSource{
			LogsPath: &dashboardservice.LogsPathSource{
				Value: utils.TypeStringToStringPointer(source.LogsPath),
			},
		}, nil
	case !source.ConstantList.IsNull():
		constantList, diags := stringValuesFromList(ctx, source.ConstantList)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboardservice.MultiSelectSource{
			ConstantList: &dashboardservice.ConstantListSource{
				Values: constantList,
			},
		}, nil
	case source.MetricLabel != nil:
		return &dashboardservice.MultiSelectSource{
			MetricLabel: &dashboardservice.MetricLabelSource{
				MetricName: utils.TypeStringToStringPointer(source.MetricLabel.MetricName),
				Label:      utils.TypeStringToStringPointer(source.MetricLabel.Label),
			},
		}, nil
	case source.SpanField != nil:
		spanField, dg := dashboardwidgets.ExpandSpansField(source.SpanField)
		if dg != nil {
			return nil, diag.Diagnostics{dg}
		}
		return &dashboardservice.MultiSelectSource{
			SpanField: &dashboardservice.SpanFieldSource{
				Value: spanField,
			},
		}, nil
	case !source.Query.IsNull():
		return expandMultiSelectSourceQuery(ctx, source.Query)
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand Multi Select Source", fmt.Sprintf("unknown multi select source type: %T", source))}
	}
}

func expandDashboardFilters(ctx context.Context, filters types.List) ([]dashboardservice.FiltersFilter, diag.Diagnostics) {
	var filtersObjects []types.Object
	var expandedFilters []dashboardservice.FiltersFilter
	diags := filters.ElementsAs(ctx, &filtersObjects, true)
	if diags.HasError() {
		return nil, diags
	}
	for _, fo := range filtersObjects {
		var filter DashboardFilterModel
		if dg := fo.As(ctx, &filter, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expandedFilter, expandDiags := expandDashboardFilter(ctx, &filter)
		if expandDiags.HasError() {
			diags.Append(expandDiags...)
			continue
		}
		expandedFilters = append(expandedFilters, *expandedFilter)
	}

	return expandedFilters, diags
}

func expandDashboardFilter(ctx context.Context, filter *DashboardFilterModel) (*dashboardservice.FiltersFilter, diag.Diagnostics) {
	if filter == nil {
		return nil, nil
	}

	source, diags := dashboardwidgets.ExpandFilterSource(ctx, filter.Source)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.FiltersFilter{
		Source:    source,
		Enabled:   typeBoolToBoolPointer(filter.Enabled),
		Collapsed: typeBoolToBoolPointer(filter.Collapsed),
	}, nil
}

func expandDashboardFolder(ctx context.Context, dashboard *dashboardservice.Dashboard, folder types.Object) (*dashboardservice.Dashboard, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(folder) {
		return dashboard, nil
	}
	var folderModel DashboardFolderModel
	dgs := folder.As(ctx, &folderModel, basetypes.ObjectAsOptions{})
	if dgs.HasError() {
		return nil, dgs
	}

	if !(folderModel.ID.IsNull() || folderModel.ID.IsUnknown()) {
		dashboard.FolderId = dashboardwidgets.ExpandDashboardUUID(folderModel.ID)
	} else if !(folderModel.Path.IsNull() || folderModel.Path.IsUnknown()) {
		segments := strings.Split(folderModel.Path.ValueString(), "/")
		dashboard.FolderPath = &dashboardservice.FolderPath{
			Segments: segments,
		}
	}

	return dashboard, nil
}

func expandOpenAPIDashboardFolder(ctx context.Context, dashboard *dashboardservice.Dashboard, folder types.Object) (*dashboardservice.Dashboard, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(folder) {
		return dashboard, nil
	}
	var folderModel DashboardFolderModel
	dgs := folder.As(ctx, &folderModel, basetypes.ObjectAsOptions{})
	if dgs.HasError() {
		return nil, dgs
	}

	if !(folderModel.ID.IsNull() || folderModel.ID.IsUnknown()) {
		dashboard.FolderId = &dashboardservice.UUID{Value: folderModel.ID.ValueStringPointer()}
	} else if !(folderModel.Path.IsNull() || folderModel.Path.IsUnknown()) {
		dashboard.FolderPath = &dashboardservice.FolderPath{Segments: strings.Split(folderModel.Path.ValueString(), "/")}
	}

	return dashboard, nil
}

func flattenDashboard(ctx context.Context, plan DashboardResourceModel, response *dashboardOpenAPIReadResult) (*DashboardResourceModel, diag.Diagnostics) {
	dashboard := response.Dashboard
	folder, diags := flattenDashboardFolder(ctx, plan.Folder, dashboard)
	if diags.HasError() {
		return nil, diags
	}
	flattenedAccessPolicy, diags := flattenDashboardAccessPolicy(plan.AccessPolicy, response.AccessPolicy)
	if diags.HasError() {
		return nil, diags
	}
	if !(plan.ContentJson.IsNull() || plan.ContentJson.IsUnknown()) {
		openAPIDashboard := response.Dashboard
		if openAPIDashboard == nil {
			return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Dashboard", "OpenAPI response did not include dashboard")}
		}

		folder, diags = flattenOpenAPIDashboardFolder(ctx, plan.Folder, openAPIDashboard)
		if diags.HasError() {
			return nil, diags
		}

		unmarshalledDashboard := new(dashboardservice.Dashboard)
		// Users can set the folder in the dashbaord's json. In that case, the server will return a folder, but we're not supposed to set it in the plan,
		// or terraform will panic.
		err := json.Unmarshal([]byte(plan.ContentJson.ValueString()), unmarshalledDashboard)
		if err != nil {
			return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Unmarshal Dashboard", err.Error())}
		}
		if unmarshalledDashboard.FolderId != nil || unmarshalledDashboard.FolderPath != nil {
			folder = types.ObjectNull(dashboardFolderModelAttr())
		}

		_, err = json.Marshal(openAPIDashboard)
		if err != nil {
			return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Dashboard", err.Error())}
		}

		//if diffType, diffString := jsondiff.Compare([]byte(plan.ContentJson.ValueString()), contentJson, &jsondiff.Options{}); !(diffType == jsondiff.FullMatch || diffType == jsondiff.SupersetMatch) {
		//	return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Dashboard", fmt.Sprintf("ContentJson does not match the dashboard content: %s", diffString))}
		//}

		return &DashboardResourceModel{
			ContentJson:  types.StringValue(plan.ContentJson.ValueString()),
			ID:           types.StringValue(openAPIDashboard.GetId()),
			Name:         types.StringNull(),
			Description:  types.StringNull(),
			Layout:       types.ObjectNull(layoutModelAttr()),
			Variables:    types.ListNull(types.ObjectType{AttrTypes: dashboardsVariablesModelAttr()}),
			Filters:      types.ListNull(types.ObjectType{AttrTypes: dashboardsFiltersModelAttr()}),
			TimeFrame:    nil,
			Folder:       folder,
			Annotations:  types.ListNull(types.ObjectType{AttrTypes: dashboardsAnnotationsModelAttr()}),
			AutoRefresh:  types.ObjectNull(dashboardAutoRefreshModelAttr()),
			AccessPolicy: flattenedAccessPolicy,
		}, nil
	}

	layout, diags := flattenDashboardLayout(ctx, &dashboard.Layout)
	if diags.HasError() {
		log.Printf("[ERROR] ERROR flattenDashboardLayout: %s", diags.Errors())
		return nil, diags
	}

	variables, diags := flattenDashboardVariables(ctx, dashboard.GetVariables())
	if diags.HasError() {
		return nil, diags
	}

	filters, diags := flattenDashboardFilters(ctx, dashboard.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	timeFrame, diags := dashboardwidgets.FlattenDashboardTimeFrame(ctx, dashboard)
	if diags.HasError() {
		return nil, diags
	}

	annotations, diags := flattenDashboardAnnotations(ctx, dashboard.GetAnnotations())
	if diags.HasError() {
		return nil, diags
	}

	autoRefresh, diags := flattenDashboardAutoRefresh(ctx, dashboard)
	if diags.HasError() {
		return nil, diags
	}

	return &DashboardResourceModel{
		ID:           types.StringValue(dashboard.GetId()),
		Name:         types.StringValue(dashboard.GetName()),
		Description:  utils.StringPointerToTypeString(dashboard.Description),
		Layout:       layout,
		Variables:    variables,
		Filters:      filters,
		TimeFrame:    timeFrame,
		Folder:       folder,
		Annotations:  annotations,
		AutoRefresh:  autoRefresh,
		ContentJson:  types.StringNull(),
		AccessPolicy: flattenedAccessPolicy,
	}, nil
}

func flattenDashboardAccessPolicy(planAccessPolicy types.String, accessPolicy *string) (types.String, diag.Diagnostics) {
	if accessPolicy == nil {
		return types.StringNull(), nil
	}
	if !planAccessPolicy.IsNull() && !planAccessPolicy.IsUnknown() && utils.JSONStringsEqual(planAccessPolicy.ValueString(), *accessPolicy) {
		return planAccessPolicy, nil
	}
	return types.StringValue(*accessPolicy), nil
}

func flattenDashboardLayout(ctx context.Context, layout *dashboardservice.Layout) (types.Object, diag.Diagnostics) {
	sections, diags := flattenDashboardSections(ctx, layout.GetSections())
	if diags.HasError() {
		return types.ObjectNull(layoutModelAttr()), diags
	}
	flattenedLayout := &DashboardLayoutModel{
		Sections: sections,
	}
	return types.ObjectValueFrom(ctx, layoutModelAttr(), flattenedLayout)
}

func flattenDashboardSections(ctx context.Context, sections []dashboardservice.Section) (types.List, diag.Diagnostics) {
	if len(sections) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: sectionModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	sectionsElements := make([]attr.Value, 0)
	for i := range sections {
		flattenedSection, diags := flattenDashboardSection(ctx, &sections[i])
		if diags.HasError() {
			diagnostics = append(diagnostics, diags...)
			continue
		}
		sectionsElement, diags := types.ObjectValueFrom(ctx, sectionModelAttr(), flattenedSection)
		if diags.HasError() {
			diagnostics = append(diagnostics, diags...)
			continue
		}
		sectionsElements = append(sectionsElements, sectionsElement)
	}

	return types.ListValueMust(types.ObjectType{AttrTypes: sectionModelAttr()}, sectionsElements), diagnostics
}

func sectionModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"id": types.StringType,
		"rows": types.ListType{
			ElemType: types.ObjectType{
				AttrTypes: rowModelAttr(),
			},
		},
		"options": types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"name":        types.StringType,
				"description": types.StringType,
				"color":       types.StringType,
				"collapsed":   types.BoolType,
			},
		},
	}
}

func rowModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"id":     types.StringType,
		"height": types.Int64Type,
		"widgets": types.ListType{
			ElemType: types.ObjectType{AttrTypes: widgetModelAttr()},
		},
	}
}

func widgetModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"id":          types.StringType,
		"title":       types.StringType,
		"description": types.StringType,
		"definition": types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"hexagon":    dashboardwidgets.HexagonType(),
				"line_chart": dashboardwidgets.LineChartType(),
				"data_table": dashboardwidgets.DataTableType(),
				"gauge": types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"query": types.ObjectType{
							AttrTypes: map[string]attr.Type{
								"logs": types.ObjectType{
									AttrTypes: map[string]attr.Type{
										"lucene_query": types.StringType,
										"logs_aggregation": types.ObjectType{
											AttrTypes: dashboardwidgets.AggregationModelAttr(),
										},
										"filters": types.ListType{
											ElemType: types.ObjectType{
												AttrTypes: dashboardwidgets.LogsFilterModelAttr(),
											},
										},
										"time_frame": types.ObjectType{
											AttrTypes: dashboardwidgets.TimeFrameModelAttr(),
										},
									},
								},
								"metrics": types.ObjectType{
									AttrTypes: map[string]attr.Type{
										"promql_query": types.StringType,
										"aggregation":  types.StringType,
										"filters": types.ListType{
											ElemType: types.ObjectType{
												AttrTypes: dashboardwidgets.MetricsFilterModelAttr(),
											},
										},
										"time_frame": types.ObjectType{
											AttrTypes: dashboardwidgets.TimeFrameModelAttr(),
										},
									},
								},
								"spans": types.ObjectType{
									AttrTypes: map[string]attr.Type{
										"lucene_query": types.StringType,
										"spans_aggregation": types.ObjectType{
											AttrTypes: dashboardwidgets.SpansAggregationModelAttr(),
										},
										"filters": types.ListType{
											ElemType: types.ObjectType{
												AttrTypes: dashboardwidgets.SpansFilterModelAttr(),
											},
										},
										"time_frame": types.ObjectType{
											AttrTypes: dashboardwidgets.TimeFrameModelAttr(),
										},
									},
								},
								"data_prime": types.ObjectType{
									AttrTypes: map[string]attr.Type{
										"query": types.StringType,
										"filters": types.ListType{
											ElemType: types.ObjectType{
												AttrTypes: dashboardwidgets.FilterSourceModelAttr(),
											},
										},
										"time_frame": types.ObjectType{
											AttrTypes: dashboardwidgets.TimeFrameModelAttr(),
										},
									},
								},
							},
						},
						"min":            types.Float64Type,
						"max":            types.Float64Type,
						"show_inner_arc": types.BoolType,
						"show_outer_arc": types.BoolType,
						"unit":           types.StringType,
						"thresholds": types.ListType{
							ElemType: types.ObjectType{
								AttrTypes: gaugeThresholdModelAttr(),
							},
						},
						"data_mode_type":      types.StringType,
						"threshold_by":        types.StringType,
						"threshold_type":      types.StringType,
						"display_series_name": types.BoolType,
						"decimal":             types.NumberType,
					},
				},
				"pie_chart": types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"query": types.ObjectType{
							AttrTypes: map[string]attr.Type{
								"logs": types.ObjectType{
									AttrTypes: map[string]attr.Type{
										"lucene_query": types.StringType,
										"aggregation": types.ObjectType{
											AttrTypes: dashboardwidgets.AggregationModelAttr(),
										},
										"filters": types.ListType{
											ElemType: types.ObjectType{
												AttrTypes: dashboardwidgets.LogsFilterModelAttr(),
											},
										},
										"group_names": types.ListType{
											ElemType: types.StringType,
										},
										"stacked_group_name": types.StringType,
										"group_names_fields": types.ListType{
											ElemType: dashboardwidgets.ObservationFieldsObject(),
										},
										"stacked_group_name_field": dashboardwidgets.ObservationFieldsObject(),
										"time_frame": types.ObjectType{
											AttrTypes: dashboardwidgets.TimeFrameModelAttr(),
										},
									},
								},
								"metrics": types.ObjectType{
									AttrTypes: map[string]attr.Type{
										"promql_query": types.StringType,
										"filters": types.ListType{
											ElemType: types.ObjectType{
												AttrTypes: dashboardwidgets.MetricsFilterModelAttr(),
											},
										},
										"group_names": types.ListType{
											ElemType: types.StringType,
										},
										"stacked_group_name": types.StringType,
										"time_frame": types.ObjectType{
											AttrTypes: dashboardwidgets.TimeFrameModelAttr(),
										},
									},
								},
								"spans": types.ObjectType{
									AttrTypes: map[string]attr.Type{
										"lucene_query": types.StringType,
										"aggregation": types.ObjectType{
											AttrTypes: dashboardwidgets.SpansAggregationModelAttr(),
										},
										"filters": types.ListType{
											ElemType: types.ObjectType{
												AttrTypes: dashboardwidgets.SpansFilterModelAttr(),
											},
										},
										"group_names": types.ListType{
											ElemType: types.ObjectType{
												AttrTypes: dashboardwidgets.SpansFieldModelAttr(),
											},
										},
										"stacked_group_name": types.ObjectType{
											AttrTypes: dashboardwidgets.SpansFieldModelAttr(),
										},
										"time_frame": types.ObjectType{
											AttrTypes: dashboardwidgets.TimeFrameModelAttr(),
										},
									},
								},
								"data_prime": types.ObjectType{
									AttrTypes: map[string]attr.Type{
										"query": types.StringType,
										"filters": types.ListType{
											ElemType: types.ObjectType{
												AttrTypes: dashboardwidgets.FilterSourceModelAttr(),
											},
										},
										"group_names": types.ListType{
											ElemType: types.StringType,
										},
										"stacked_group_name": types.StringType,
										"time_frame": types.ObjectType{
											AttrTypes: dashboardwidgets.TimeFrameModelAttr(),
										},
									},
								},
							},
						},
						"max_slices_per_chart": types.Int64Type,
						"min_slice_percentage": types.Int64Type,
						"stack_definition": types.ObjectType{
							AttrTypes: map[string]attr.Type{
								"max_slices_per_stack": types.Int64Type,
								"stack_name_template":  types.StringType,
							},
						},
						"label_definition": types.ObjectType{
							AttrTypes: map[string]attr.Type{
								"label_source":    types.StringType,
								"is_visible":      types.BoolType,
								"show_name":       types.BoolType,
								"show_value":      types.BoolType,
								"show_percentage": types.BoolType,
							},
						},
						"show_legend":         types.BoolType,
						"group_name_template": types.StringType,
						"unit":                types.StringType,
						"color_scheme":        types.StringType,
						"data_mode_type":      types.StringType,
					},
				},
				"bar_chart": types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"query": types.ObjectType{
							AttrTypes: map[string]attr.Type{
								"logs": types.ObjectType{
									AttrTypes: barChartLogsQueryAttr(),
								},
								"metrics": types.ObjectType{
									AttrTypes: barChartMetricsQueryAttr(),
								},
								"spans": types.ObjectType{
									AttrTypes: barChartSpansQueryAttr(),
								},
								"data_prime": types.ObjectType{
									AttrTypes: barChartDataPrimeQueryAttr(),
								},
							},
						},
						"max_bars_per_chart":  types.Int64Type,
						"group_name_template": types.StringType,
						"stack_definition": types.ObjectType{
							AttrTypes: map[string]attr.Type{
								"stack_name_template": types.StringType,
								"max_slices_per_bar":  types.Int64Type,
							},
						},
						"scale_type": types.StringType,
						"colors_by":  types.StringType,
						"xaxis": types.ObjectType{
							AttrTypes: map[string]attr.Type{
								"time": types.ObjectType{
									AttrTypes: map[string]attr.Type{
										"interval":          types.StringType,
										"buckets_presented": types.Int64Type,
									},
								},
								"value": types.ObjectType{
									AttrTypes: map[string]attr.Type{},
								},
							},
						},
						"unit":           types.StringType,
						"sort_by":        types.StringType,
						"color_scheme":   types.StringType,
						"data_mode_type": types.StringType,
					},
				},
				"horizontal_bar_chart": types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"query": types.ObjectType{
							AttrTypes: map[string]attr.Type{
								"logs": types.ObjectType{
									AttrTypes: map[string]attr.Type{
										"lucene_query": types.StringType,
										"aggregation": types.ObjectType{
											AttrTypes: dashboardwidgets.AggregationModelAttr(),
										},
										"filters": types.ListType{
											ElemType: types.ObjectType{
												AttrTypes: dashboardwidgets.LogsFilterModelAttr(),
											},
										},
										"group_names": types.ListType{
											ElemType: types.StringType,
										},
										"stacked_group_name": types.StringType,
										"group_names_fields": types.ListType{
											ElemType: dashboardwidgets.ObservationFieldsObject(),
										},
										"stacked_group_name_field": dashboardwidgets.ObservationFieldsObject(),
										"time_frame": types.ObjectType{
											AttrTypes: dashboardwidgets.TimeFrameModelAttr(),
										},
									},
								},
								"metrics": types.ObjectType{
									AttrTypes: map[string]attr.Type{
										"promql_query": types.StringType,
										"filters": types.ListType{
											ElemType: types.ObjectType{
												AttrTypes: dashboardwidgets.MetricsFilterModelAttr(),
											},
										},
										"group_names": types.ListType{
											ElemType: types.StringType,
										},
										"stacked_group_name": types.StringType,
										"time_frame": types.ObjectType{
											AttrTypes: dashboardwidgets.TimeFrameModelAttr(),
										},
									},
								},
								"spans": types.ObjectType{
									AttrTypes: map[string]attr.Type{
										"lucene_query": types.StringType,
										"aggregation": types.ObjectType{
											AttrTypes: dashboardwidgets.SpansAggregationModelAttr(),
										},
										"filters": types.ListType{
											ElemType: types.ObjectType{
												AttrTypes: dashboardwidgets.SpansFilterModelAttr(),
											},
										},
										"group_names": types.ListType{
											ElemType: types.ObjectType{
												AttrTypes: dashboardwidgets.SpansFieldModelAttr(),
											},
										},
										"stacked_group_name": types.ObjectType{
											AttrTypes: dashboardwidgets.SpansFieldModelAttr(),
										},
										"time_frame": types.ObjectType{
											AttrTypes: dashboardwidgets.TimeFrameModelAttr(),
										},
									},
								},
								"data_prime": types.ObjectType{
									AttrTypes: barChartDataPrimeQueryAttr(),
								},
							},
						},
						"max_bars_per_chart":  types.Int64Type,
						"group_name_template": types.StringType,
						"stack_definition": types.ObjectType{
							AttrTypes: map[string]attr.Type{
								"stack_name_template": types.StringType,
								"max_slices_per_bar":  types.Int64Type,
							},
						},
						"scale_type":     types.StringType,
						"colors_by":      types.StringType,
						"unit":           types.StringType,
						"sort_by":        types.StringType,
						"color_scheme":   types.StringType,
						"display_on_bar": types.BoolType,
						"y_axis_view_by": types.StringType,
						"data_mode_type": types.StringType,
					},
				},
				"markdown": types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"markdown_text": types.StringType,
						"tooltip_text":  types.StringType,
					},
				},
			},
		},
		"width": types.Int64Type,
	}
}

func barChartLogsQueryAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"lucene_query": types.StringType,
		"aggregation": types.ObjectType{
			AttrTypes: dashboardwidgets.AggregationModelAttr(),
		},
		"filters": types.ListType{
			ElemType: types.ObjectType{
				AttrTypes: dashboardwidgets.LogsFilterModelAttr(),
			},
		},
		"group_names": types.ListType{
			ElemType: types.StringType,
		},
		"stacked_group_name": types.StringType,
		"group_names_fields": types.ListType{
			ElemType: dashboardwidgets.ObservationFieldsObject(),
		},
		"stacked_group_name_field": dashboardwidgets.ObservationFieldsObject(),
		"time_frame": types.ObjectType{
			AttrTypes: dashboardwidgets.TimeFrameModelAttr(),
		},
	}
}

func barChartMetricsQueryAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"promql_query": types.StringType,
		"filters": types.ListType{
			ElemType: types.ObjectType{
				AttrTypes: dashboardwidgets.MetricsFilterModelAttr(),
			},
		},
		"group_names": types.ListType{
			ElemType: types.StringType,
		},
		"stacked_group_name": types.StringType,
		"time_frame": types.ObjectType{
			AttrTypes: dashboardwidgets.TimeFrameModelAttr(),
		},
	}
}

func barChartSpansQueryAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"lucene_query": types.StringType,
		"aggregation": types.ObjectType{
			AttrTypes: dashboardwidgets.SpansAggregationModelAttr(),
		},
		"filters": types.ListType{
			ElemType: types.ObjectType{
				AttrTypes: dashboardwidgets.SpansFilterModelAttr(),
			},
		},
		"group_names": types.ListType{
			ElemType: types.ObjectType{
				AttrTypes: dashboardwidgets.SpansFieldModelAttr(),
			},
		},
		"stacked_group_name": types.ObjectType{
			AttrTypes: dashboardwidgets.SpansFieldModelAttr(),
		},
		"time_frame": types.ObjectType{
			AttrTypes: dashboardwidgets.TimeFrameModelAttr(),
		},
	}
}

func barChartDataPrimeQueryAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"query": types.StringType,
		"filters": types.ListType{
			ElemType: types.ObjectType{
				AttrTypes: dashboardwidgets.FilterSourceModelAttr(),
			},
		},
		"group_names": types.ListType{
			ElemType: types.StringType,
		},
		"stacked_group_name": types.StringType,
		"time_frame": types.ObjectType{
			AttrTypes: dashboardwidgets.TimeFrameModelAttr(),
		},
	}
}

func dashboardsAnnotationsModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"id":      types.StringType,
		"name":    types.StringType,
		"enabled": types.BoolType,
		"source": types.ObjectType{
			AttrTypes: annotationSourceModelAttr(),
		},
	}
}

func annotationSourceModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"metrics": types.ObjectType{
			AttrTypes: annotationsMetricsSourceModelAttr(),
		},
		"logs": types.ObjectType{
			AttrTypes: annotationsLogsAndSpansSourceModelAttr(),
		},
		"spans": types.ObjectType{
			AttrTypes: annotationsLogsAndSpansSourceModelAttr(),
		},
		"manual": types.ObjectType{
			AttrTypes: annotationsManualSourceModelAttr(),
		},
	}
}

func annotationsManualSourceModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"orientation":      types.StringType,
		"message_template": types.StringType,
		"strategy": types.ObjectType{
			AttrTypes: manualStrategyModelAttr(),
		},
	}
}

func manualStrategyModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"instant": types.ObjectType{
			AttrTypes: manualInstantStrategyModelAttr(),
		},
		"range": types.ObjectType{
			AttrTypes: manualRangeStrategyModelAttr(),
		},
	}
}

func manualInstantStrategyModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"value":       types.Float64Type,
		"unit":        types.StringType,
		"custom_unit": types.StringType,
	}
}

func manualRangeStrategyModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"start_value": types.Float64Type,
		"end_value":   types.Float64Type,
		"unit":        types.StringType,
		"custom_unit": types.StringType,
	}
}

func annotationsMetricsSourceModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"promql_query": types.StringType,
		"strategy": types.ObjectType{
			AttrTypes: metricStrategyModelAttr(),
		},
		"message_template": types.StringType,
		"labels": types.ListType{
			ElemType: types.StringType,
		},
	}
}

func annotationsLogsAndSpansSourceModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"lucene_query": types.StringType,
		"strategy": types.ObjectType{
			AttrTypes: logsAndSpansStrategyModelAttr(),
		},
		"message_template": types.StringType,
		"label_fields": types.ListType{
			ElemType: observationFieldModelAttr(),
		},
	}
}

func logsAndSpansStrategyModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"instant": types.ObjectType{
			AttrTypes: instantStrategyModelAttr(),
		},
		"range": types.ObjectType{
			AttrTypes: rangeStrategyModelAttr(),
		},
		"duration": types.ObjectType{
			AttrTypes: durationStrategyModelAttr(),
		},
	}
}

func durationStrategyModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"start_timestamp_field": observationFieldModelAttr(),
		"duration_field":        observationFieldModelAttr(),
	}
}

func rangeStrategyModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"start_timestamp_field": observationFieldModelAttr(),
		"end_timestamp_field":   observationFieldModelAttr(),
	}
}

func instantStrategyModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"timestamp_field": observationFieldModelAttr(),
	}
}

func observationFieldModelAttr() attr.Type {
	return types.ObjectType{
		AttrTypes: dashboardwidgets.ObservationFieldAttr(),
	}
}

func metricStrategyModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"start_time": types.ObjectType{
			AttrTypes: map[string]attr.Type{},
		},
	}
}

func gaugeThresholdModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"from":  types.Float64Type,
		"color": types.StringType,
		"label": types.StringType,
	}
}

func dashboardsVariablesModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"name":         types.StringType,
		"display_name": types.StringType,
		"definition": types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"constant_value": types.StringType,
				"multi_select": types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"selected_values": types.ListType{
							ElemType: types.StringType,
						},
						"values_order_direction": types.StringType,
						"selection_type":         types.StringType,
						"source": types.ObjectType{
							AttrTypes: map[string]attr.Type{
								"logs_path": types.StringType,
								"metric_label": types.ObjectType{
									AttrTypes: map[string]attr.Type{
										"metric_name": types.StringType,
										"label":       types.StringType,
									},
								},
								"constant_list": types.ListType{
									ElemType: types.StringType,
								},
								"span_field": types.ObjectType{
									AttrTypes: dashboardwidgets.SpansFieldModelAttr(),
								},
								"query": types.ObjectType{
									AttrTypes: multiSelectQueryAttr(),
								},
							},
						},
					},
				},
			},
		},
	}
}

func dashboardsFiltersModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"source": types.ObjectType{
			AttrTypes: dashboardwidgets.FilterSourceModelAttr(),
		},
		"enabled":   types.BoolType,
		"collapsed": types.BoolType,
	}
}

func layoutModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"sections": types.ListType{
			ElemType: types.ObjectType{
				AttrTypes: sectionModelAttr(),
			},
		},
	}
}

func dashboardFolderModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"id":   types.StringType,
		"path": types.StringType,
	}
}

func dashboardAutoRefreshModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"type": types.StringType,
	}
}

func flattenDashboardSection(ctx context.Context, section *dashboardservice.Section) (*SectionModel, diag.Diagnostics) {
	if section == nil {
		return nil, nil
	}

	rows, diags := flattenDashboardRows(ctx, section.GetRows())
	if diags.HasError() {
		return nil, diags
	}

	options, diags := flattenDashboardOptions(ctx, section.Options)
	if diags.HasError() {
		return nil, diags
	}

	return &SectionModel{
		ID:      uuidToTypeString(section.Id),
		Rows:    rows,
		Options: options,
	}, nil
}

func flattenDashboardOptions(_ context.Context, opts *dashboardservice.SectionOptions) (*SectionOptionsModel, diag.Diagnostics) {
	if opts == nil || opts.Custom == nil {
		return nil, nil
	}
	custom := opts.Custom
	var description basetypes.StringValue
	if custom.Description != nil {
		description = types.StringValue(*custom.Description)
	} else {
		description = types.StringNull()
	}

	var collapsed basetypes.BoolValue
	if custom.Collapsed != nil {
		collapsed = types.BoolValue(*custom.Collapsed)
	} else {
		collapsed = types.BoolNull()
	}

	var color basetypes.StringValue
	if custom.Color != nil &&
		custom.Color.Predefined != nil &&
		*custom.Color.Predefined != "" &&
		*custom.Color.Predefined != dashboardservice.SECTIONPREDEFINEDCOLOR_SECTION_PREDEFINED_COLOR_UNSPECIFIED {
		colorString := string(*custom.Color.Predefined)
		colors := strings.Split(colorString, "_")
		color = types.StringValue(strings.ToLower(colors[len(colors)-1]))
	} else {
		color = types.StringNull()
	}

	return &SectionOptionsModel{
		Name:        utils.StringPointerToTypeString(custom.Name),
		Description: description,
		Collapsed:   collapsed,
		Color:       color,
	}, nil
}

func flattenDashboardRows(ctx context.Context, rows []dashboardservice.Row) (types.List, diag.Diagnostics) {
	if len(rows) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: rowModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	rowsElements := make([]attr.Value, 0)
	for i := range rows {
		flattenedRow, diags := flattenDashboardRow(ctx, &rows[i])
		if diags.HasError() {
			diagnostics = append(diagnostics, diags...)
			continue
		}
		rowElement, diags := types.ObjectValueFrom(ctx, rowModelAttr(), flattenedRow)
		if diags.HasError() {
			diagnostics = append(diagnostics, diags...)
			continue
		}
		rowsElements = append(rowsElements, rowElement)
	}

	return types.ListValueMust(types.ObjectType{AttrTypes: rowModelAttr()}, rowsElements), diagnostics
}

func flattenDashboardRow(ctx context.Context, row *dashboardservice.Row) (*RowModel, diag.Diagnostics) {
	if row == nil {
		return nil, nil
	}

	widgets, diags := flattenDashboardWidgets(ctx, row.GetWidgets())
	if diags.HasError() {
		return nil, diags
	}
	return &RowModel{
		ID:      uuidToTypeString(row.Id),
		Height:  int32PointerToTypeInt64(row.GetAppearance().Height),
		Widgets: widgets,
	}, nil
}

func flattenDashboardWidgets(ctx context.Context, widgets []dashboardservice.Widget) (types.List, diag.Diagnostics) {
	if len(widgets) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: widgetModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	widgetsElements := make([]attr.Value, 0)
	for i := range widgets {
		flattenedWidget, diags := flattenDashboardWidget(ctx, &widgets[i])
		if diags.HasError() {
			diagnostics.Append(diags...)
			continue
		}
		widgetElement, diags := types.ObjectValueFrom(ctx, widgetModelAttr(), flattenedWidget)
		if diags.HasError() {
			diagnostics.Append(diags...)
			continue
		}
		widgetsElements = append(widgetsElements, widgetElement)
	}

	return types.ListValueMust(types.ObjectType{AttrTypes: widgetModelAttr()}, widgetsElements), diagnostics
}

func flattenDashboardWidget(ctx context.Context, widget *dashboardservice.Widget) (*WidgetModel, diag.Diagnostics) {
	if widget == nil {
		return nil, nil
	}

	definition, diags := flattenDashboardWidgetDefinition(ctx, widget.Definition)
	if diags.HasError() {
		return nil, diags
	}

	return &WidgetModel{
		ID:          uuidToTypeString(widget.Id),
		Title:       utils.StringPointerToTypeString(widget.Title),
		Description: utils.StringPointerToTypeString(widget.Description),
		Width:       int32PointerToTypeInt64(widget.GetAppearance().Width),
		Definition:  definition,
	}, nil
}

func flattenDashboardWidgetDefinition(ctx context.Context, definition *dashboardservice.WidgetDefinition) (*dashboardwidgets.WidgetDefinitionModel, diag.Diagnostics) {
	if definition == nil {
		return nil, nil
	}

	switch {
	case definition.LineChart != nil:
		return dashboardwidgets.FlattenLineChart(ctx, definition.LineChart)
	case definition.Hexagon != nil:
		return dashboardwidgets.FlattenHexagon(ctx, definition.Hexagon)
	case definition.DataTable != nil:
		return dashboardwidgets.FlattenDataTable(ctx, definition.DataTable)
	case definition.Gauge != nil:
		return flattenGauge(ctx, definition.Gauge)
	case definition.PieChart != nil:
		return flattenPieChart(ctx, definition.PieChart)
	case definition.BarChart != nil:
		return flattenBarChart(ctx, definition.BarChart)
	case definition.HorizontalBarChart != nil:
		return flattenHorizontalBarChart(ctx, definition.HorizontalBarChart)
	case definition.Markdown != nil:
		return flattenMarkdown(definition.Markdown), nil
	case definition.Dynamic != nil:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic(
			"Unsupported Dashboard Widget Definition",
			"The backend returned a dynamic widget. Dynamic widgets are supported only when configuring content_json; import and data-source reads cannot reconstruct content_json as structured Terraform state.",
		)}
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Widget Definition", "unknown widget definition type")}
	}
}

func flattenMarkdown(markdown *dashboardservice.Markdown) *dashboardwidgets.WidgetDefinitionModel {
	return &dashboardwidgets.WidgetDefinitionModel{
		Markdown: &dashboardwidgets.MarkdownModel{
			MarkdownText: utils.StringPointerToTypeString(markdown.MarkdownText),
			TooltipText:  utils.StringPointerToTypeString(markdown.TooltipText),
		},
	}
}

func flattenHorizontalBarChart(ctx context.Context, chart *dashboardservice.HorizontalBarChart) (*dashboardwidgets.WidgetDefinitionModel, diag.Diagnostics) {
	if chart == nil {
		return nil, nil
	}

	query, diags := flattenHorizontalBarChartQueryDefinitions(ctx, chart.Query)
	if diags.HasError() {
		return nil, diags
	}

	colorsBy, dg := flattenBarChartColorsBy(chart.ColorsBy)
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	return &dashboardwidgets.WidgetDefinitionModel{
		HorizontalBarChart: &dashboardwidgets.HorizontalBarChartModel{
			Query:             query,
			MaxBarsPerChart:   int32PointerToTypeInt64(chart.MaxBarsPerChart),
			GroupNameTemplate: utils.StringPointerToTypeString(chart.GroupNameTemplate),
			StackDefinition:   flattenHorizontalBarChartStackDefinition(chart.StackDefinition),
			ScaleType:         types.StringValue(dashboardwidgets.DashboardProtoToSchemaScaleType[chart.GetScaleType()]),
			ColorsBy:          colorsBy,
			Unit:              types.StringValue(dashboardwidgets.DashboardProtoToSchemaUnit[chart.GetUnit()]),
			DisplayOnBar:      types.BoolPointerValue(chart.DisplayOnBar),
			YAxisViewBy:       flattenYAxisViewBy(chart.YAxisViewBy),
			SortBy:            types.StringValue(dashboardwidgets.DashboardProtoToSchemaSortBy[chart.GetSortBy()]),
			ColorScheme:       utils.StringPointerToTypeString(chart.ColorScheme),
			DataModeType:      types.StringValue(dashboardwidgets.DashboardProtoToSchemaDataModeType[chart.GetDataModeType()]),
		},
	}, nil
}

func flattenYAxisViewBy(yAxisViewBy *dashboardservice.HorizontalBarChartYAxisViewBy) types.String {
	switch {
	case yAxisViewBy == nil:
		return types.StringNull()
	case yAxisViewBy.Category != nil:
		return types.StringValue("category")
	case yAxisViewBy.Value != nil:
		return types.StringValue("value")
	default:
		return types.StringNull()
	}
}

func flattenHorizontalBarChartQueryDefinitions(ctx context.Context, query *dashboardservice.HorizontalBarChartQuery) (*dashboardwidgets.HorizontalBarChartQueryModel, diag.Diagnostics) {
	if query == nil {
		return nil, nil
	}

	switch {
	case query.Logs != nil:
		return flattenHorizontalBarChartQueryLogs(ctx, query.Logs)
	case query.Metrics != nil:
		return flattenHorizontalBarChartQueryMetrics(ctx, query.Metrics)
	case query.Spans != nil:
		return flattenHorizontalBarChartQuerySpans(ctx, query.Spans)
	case query.Dataprime != nil:
		return flattenHorizontalBarChartQueryDataPrime(ctx, query.Dataprime)
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Horizontal Bar Chart Query", "unknown horizontal bar chart query type")}
	}
}

func flattenHorizontalBarChartQueryDataPrime(ctx context.Context, dataPrime *dashboardservice.HorizontalBarChartDataprimeQuery) (*dashboardwidgets.HorizontalBarChartQueryModel, diag.Diagnostics) {
	if dataPrime == nil {
		return nil, nil
	}

	filters, diags := dashboardwidgets.FlattenDashboardFiltersSources(ctx, dataPrime.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	timeFrame, diags := dashboardwidgets.FlattenTimeFrameSelect(ctx, dataPrime.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}
	query := &dashboardwidgets.BarChartQueryDataPrimeModel{
		Query:            utils.StringPointerToTypeString(dataPrime.GetDataprimeQuery().Text),
		Filters:          filters,
		GroupNames:       utils.StringSliceToTypeStringList(dataPrime.GetGroupNames()),
		StackedGroupName: utils.StringPointerToTypeString(dataPrime.StackedGroupName),
		TimeFrame:        timeFrame,
	}

	queryObj, diags := types.ObjectValueFrom(ctx, barChartDataPrimeQueryAttr(), query)
	if diags.HasError() {
		return nil, diags
	}
	return &dashboardwidgets.HorizontalBarChartQueryModel{
		Logs:      types.ObjectNull(barChartLogsQueryAttr()),
		Spans:     types.ObjectNull(barChartSpansQueryAttr()),
		Metrics:   types.ObjectNull(barChartMetricsQueryAttr()),
		DataPrime: queryObj,
	}, nil
}

func flattenHorizontalBarChartQueryLogs(ctx context.Context, logs *dashboardservice.HorizontalBarChartLogsQuery) (*dashboardwidgets.HorizontalBarChartQueryModel, diag.Diagnostics) {
	if logs == nil {
		return nil, nil
	}

	aggregation, diags := dashboardwidgets.FlattenLogsAggregation(ctx, logs.Aggregation)
	if diags.HasError() {
		return nil, diags
	}

	filters, diags := dashboardwidgets.FlattenLogsFilters(ctx, logs.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	groupNamesFields, diags := dashboardwidgets.FlattenObservationFields(ctx, logs.GetGroupNamesFields())
	if diags.HasError() {
		return nil, diags
	}

	stackedGroupNameField, diags := dashboardwidgets.FlattenObservationField(ctx, logs.StackedGroupNameField)
	if diags.HasError() {
		return nil, diags
	}

	timeFrame, diags := dashboardwidgets.FlattenTimeFrameSelect(ctx, logs.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	logsModel := &dashboardwidgets.BarChartQueryLogsModel{
		LuceneQuery:           utils.StringPointerToTypeString(logs.GetLuceneQuery().Value),
		Aggregation:           aggregation,
		Filters:               filters,
		GroupNames:            utils.StringSliceToTypeStringList(logs.GetGroupNames()),
		StackedGroupName:      utils.StringPointerToTypeString(logs.StackedGroupName),
		GroupNamesFields:      groupNamesFields,
		StackedGroupNameField: stackedGroupNameField,
		TimeFrame:             timeFrame,
	}

	logsObject, diags := types.ObjectValueFrom(ctx, barChartLogsQueryAttr(), logsModel)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardwidgets.HorizontalBarChartQueryModel{
		Logs:      logsObject,
		Metrics:   types.ObjectNull(barChartMetricsQueryAttr()),
		Spans:     types.ObjectNull(barChartSpansQueryAttr()),
		DataPrime: types.ObjectNull(barChartDataPrimeQueryAttr()),
	}, nil
}

func flattenHorizontalBarChartQueryMetrics(ctx context.Context, metrics *dashboardservice.HorizontalBarChartMetricsQuery) (*dashboardwidgets.HorizontalBarChartQueryModel, diag.Diagnostics) {
	if metrics == nil {
		return nil, nil
	}

	filters, diags := dashboardwidgets.FlattenMetricsFilters(ctx, metrics.GetFilters())
	if diags.HasError() {
		return nil, diags
	}
	timeFrame, diags := dashboardwidgets.FlattenTimeFrameSelect(ctx, metrics.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	flattenedMetrics := &dashboardwidgets.BarChartQueryMetricsModel{
		PromqlQuery:      utils.StringPointerToTypeString(metrics.GetPromqlQuery().Value),
		Filters:          filters,
		GroupNames:       utils.StringSliceToTypeStringList(metrics.GetGroupNames()),
		StackedGroupName: utils.StringPointerToTypeString(metrics.StackedGroupName),
		TimeFrame:        timeFrame,
	}

	metricsObject, diags := types.ObjectValueFrom(ctx, barChartMetricsQueryAttr(), flattenedMetrics)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardwidgets.HorizontalBarChartQueryModel{
		Metrics:   metricsObject,
		Logs:      types.ObjectNull(barChartLogsQueryAttr()),
		Spans:     types.ObjectNull(barChartSpansQueryAttr()),
		DataPrime: types.ObjectNull(barChartDataPrimeQueryAttr()),
	}, nil
}

func flattenHorizontalBarChartQuerySpans(ctx context.Context, spans *dashboardservice.HorizontalBarChartSpansQuery) (*dashboardwidgets.HorizontalBarChartQueryModel, diag.Diagnostics) {
	if spans == nil {
		return nil, nil
	}

	aggregation, dg := dashboardwidgets.FlattenSpansAggregation(spans.Aggregation)
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	filters, diags := dashboardwidgets.FlattenSpansFilters(ctx, spans.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	groupNames, diags := dashboardwidgets.FlattenSpansFields(ctx, spans.GetGroupNames())
	if diags.HasError() {
		return nil, diags
	}

	stackedGroupName, dg := dashboardwidgets.FlattenSpansField(spans.StackedGroupName)
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	timeFrame, diags := dashboardwidgets.FlattenTimeFrameSelect(ctx, spans.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	flattenedSpans := &dashboardwidgets.BarChartQuerySpansModel{
		LuceneQuery:      utils.StringPointerToTypeString(spans.GetLuceneQuery().Value),
		Aggregation:      aggregation,
		Filters:          filters,
		GroupNames:       groupNames,
		StackedGroupName: stackedGroupName,
		TimeFrame:        timeFrame,
	}

	spansObject, diags := types.ObjectValueFrom(ctx, barChartSpansQueryAttr(), flattenedSpans)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardwidgets.HorizontalBarChartQueryModel{
		Spans:     spansObject,
		Logs:      types.ObjectNull(barChartLogsQueryAttr()),
		Metrics:   types.ObjectNull(barChartMetricsQueryAttr()),
		DataPrime: types.ObjectNull(barChartDataPrimeQueryAttr()),
	}, nil
}

func flattenGauge(ctx context.Context, gauge *dashboardservice.WidgetsGauge) (*dashboardwidgets.WidgetDefinitionModel, diag.Diagnostics) {
	if gauge == nil {
		return nil, nil
	}

	query, diags := flattenGaugeQueries(ctx, gauge.Query)
	if diags != nil {
		return nil, diags
	}

	thresholds, diags := flattenGaugeThresholds(ctx, gauge.GetThresholds())
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardwidgets.WidgetDefinitionModel{
		Gauge: &dashboardwidgets.GaugeModel{
			Query:             query,
			Min:               types.Float64PointerValue(gauge.Min),
			Max:               types.Float64PointerValue(gauge.Max),
			ShowInnerArc:      types.BoolPointerValue(gauge.ShowInnerArc),
			ShowOuterArc:      types.BoolPointerValue(gauge.ShowOuterArc),
			Unit:              types.StringValue(dashboardwidgets.DashboardProtoToSchemaGaugeUnit[gauge.GetUnit()]),
			Thresholds:        thresholds,
			DataModeType:      types.StringValue(dashboardwidgets.DashboardProtoToSchemaDataModeType[gauge.GetDataModeType()]),
			ThresholdBy:       types.StringValue(dashboardwidgets.DashboardProtoToSchemaGaugeThresholdBy[gauge.GetThresholdBy()]),
			ThresholdType:     types.StringValue(dashboardwidgets.DashboardProtoToSchemaThresholdType[gauge.GetThresholdType()]),
			DisplaySeriesName: types.BoolPointerValue(gauge.DisplaySeriesName),
			Decimal:           int32PointerToNumberType(gauge.Decimal),
		},
	}, nil
}

func flattenGaugeThresholds(ctx context.Context, thresholds []dashboardservice.GaugeThreshold) (types.List, diag.Diagnostics) {
	if len(thresholds) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: gaugeThresholdModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	thresholdElements := make([]attr.Value, 0)
	for i := range thresholds {
		flattenedThreshold := flattenGaugeThreshold(&thresholds[i])
		thresholdElement, diags := types.ObjectValueFrom(ctx, gaugeThresholdModelAttr(), flattenedThreshold)
		if diags.HasError() {
			diagnostics = append(diagnostics, diags...)
			continue
		}
		thresholdElements = append(thresholdElements, thresholdElement)
	}

	return types.ListValueMust(types.ObjectType{AttrTypes: gaugeThresholdModelAttr()}, thresholdElements), diagnostics
}

func flattenGaugeThreshold(threshold *dashboardservice.GaugeThreshold) *dashboardwidgets.GaugeThresholdModel {
	if threshold == nil {
		return nil
	}
	return &dashboardwidgets.GaugeThresholdModel{
		From:  types.Float64PointerValue(threshold.From),
		Color: utils.StringPointerToTypeString(threshold.Color),
		Label: utils.StringPointerToTypeString(threshold.Label),
	}
}

func flattenGaugeQueries(ctx context.Context, query *dashboardservice.GaugeQuery) (*dashboardwidgets.GaugeQueryModel, diag.Diagnostics) {
	switch {
	case query == nil:
		return nil, nil
	case query.Metrics != nil:
		return flattenGaugeQueryMetrics(ctx, query.Metrics)
	case query.Logs != nil:
		return flattenGaugeQueryLogs(ctx, query.Logs)
	case query.Spans != nil:
		return flattenGaugeQuerySpans(ctx, query.Spans)
	case query.Dataprime != nil:
		return flattenGaugeQueryDataPrime(ctx, query.Dataprime)
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Gauge Query", fmt.Sprintf("unknown query type %T", query))}
	}
}

func flattenGaugeQueryMetrics(ctx context.Context, metrics *dashboardservice.GaugeMetricsQuery) (*dashboardwidgets.GaugeQueryModel, diag.Diagnostics) {
	if metrics == nil {
		return nil, nil
	}

	filters, diags := dashboardwidgets.FlattenMetricsFilters(ctx, metrics.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	timeFrame, diags := dashboardwidgets.FlattenTimeFrameSelect(ctx, metrics.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardwidgets.GaugeQueryModel{
		Metrics: &dashboardwidgets.GaugeQueryMetricsModel{
			PromqlQuery: utils.StringPointerToTypeString(metrics.GetPromqlQuery().Value),
			Aggregation: types.StringValue(dashboardwidgets.DashboardProtoToSchemaGaugeAggregation[metrics.GetAggregation()]),
			Filters:     filters,
			TimeFrame:   timeFrame,
		},
	}, nil
}

func flattenGaugeQueryLogs(ctx context.Context, logs *dashboardservice.GaugeLogsQuery) (*dashboardwidgets.GaugeQueryModel, diag.Diagnostics) {
	if logs == nil {
		return nil, nil
	}

	filters, diags := dashboardwidgets.FlattenLogsFilters(ctx, logs.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	logsAggregation, diags := dashboardwidgets.FlattenLogsAggregation(ctx, logs.LogsAggregation)
	if diags.HasError() {
		return nil, diags
	}
	timeFrame, diags := dashboardwidgets.FlattenTimeFrameSelect(ctx, logs.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardwidgets.GaugeQueryModel{
		Logs: &dashboardwidgets.GaugeQueryLogsModel{
			LuceneQuery:     utils.StringPointerToTypeString(logs.GetLuceneQuery().Value),
			LogsAggregation: logsAggregation,
			Filters:         filters,
			TimeFrame:       timeFrame,
		},
	}, nil
}

func flattenGaugeQuerySpans(ctx context.Context, spans *dashboardservice.GaugeSpansQuery) (*dashboardwidgets.GaugeQueryModel, diag.Diagnostics) {
	if spans == nil {
		return nil, nil
	}

	filters, diags := dashboardwidgets.FlattenSpansFilters(ctx, spans.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	spansAggregation, dg := dashboardwidgets.FlattenSpansAggregation(spans.SpansAggregation)
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}
	timeFrame, diags := dashboardwidgets.FlattenTimeFrameSelect(ctx, spans.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardwidgets.GaugeQueryModel{
		Spans: &dashboardwidgets.GaugeQuerySpansModel{
			LuceneQuery:      utils.StringPointerToTypeString(spans.GetLuceneQuery().Value),
			Filters:          filters,
			SpansAggregation: spansAggregation,
			TimeFrame:        timeFrame,
		},
	}, nil
}

func flattenGaugeQueryDataPrime(ctx context.Context, dataPrime *dashboardservice.GaugeDataprimeQuery) (*dashboardwidgets.GaugeQueryModel, diag.Diagnostics) {
	if dataPrime == nil {
		return nil, nil
	}
	queryStr := types.StringNull()
	if dataPrime.DataprimeQuery != nil {
		queryStr = utils.StringPointerToTypeString(dataPrime.DataprimeQuery.Text)
	}
	filters, diags := dashboardwidgets.FlattenDashboardFiltersSources(ctx, dataPrime.GetFilters())
	if diags.HasError() {
		return nil, diags
	}
	timeFrame, diags := dashboardwidgets.FlattenTimeFrameSelect(ctx, dataPrime.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}
	return &dashboardwidgets.GaugeQueryModel{
		DataPrime: &dashboardwidgets.DataPrimeModel{
			Query:     queryStr,
			Filters:   filters,
			TimeFrame: timeFrame,
		},
	}, nil
}

func flattenPieChart(ctx context.Context, pieChart *dashboardservice.WidgetsPieChart) (*dashboardwidgets.WidgetDefinitionModel, diag.Diagnostics) {
	if pieChart == nil {
		return nil, nil
	}

	query, diags := flattenPieChartQueries(ctx, pieChart.Query)
	if diags != nil {
		return nil, diags
	}

	return &dashboardwidgets.WidgetDefinitionModel{
		PieChart: &dashboardwidgets.PieChartModel{
			Query:              query,
			MaxSlicesPerChart:  int32PointerToTypeInt64(pieChart.MaxSlicesPerChart),
			MinSlicePercentage: int32PointerToTypeInt64(pieChart.MinSlicePercentage),
			StackDefinition:    flattenPieChartStackDefinition(pieChart.StackDefinition),
			LabelDefinition:    flattenPieChartLabelDefinition(pieChart.LabelDefinition),
			ShowLegend:         types.BoolPointerValue(pieChart.ShowLegend),
			GroupNameTemplate:  utils.StringPointerToTypeString(pieChart.GroupNameTemplate),
			Unit:               types.StringValue(dashboardwidgets.DashboardProtoToSchemaUnit[pieChart.GetUnit()]),
			ColorScheme:        utils.StringPointerToTypeString(pieChart.ColorScheme),
			DataModeType:       types.StringValue(dashboardwidgets.DashboardProtoToSchemaDataModeType[pieChart.GetDataModeType()]),
		},
	}, nil
}

func flattenPieChartQueries(ctx context.Context, query *dashboardservice.PieChartQuery) (*dashboardwidgets.PieChartQueryModel, diag.Diagnostics) {
	if query == nil {
		return nil, nil
	}

	switch {
	case query.Metrics != nil:
		return flattenPieChartQueryMetrics(ctx, query.Metrics)
	case query.Logs != nil:
		return flattenPieChartQueryLogs(ctx, query.Logs)
	case query.Spans != nil:
		return flattenPieChartQuerySpans(ctx, query.Spans)
	case query.Dataprime != nil:
		return flattenPieChartDataPrimeQuery(ctx, query.Dataprime)
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Pie Chart Query", fmt.Sprintf("unknown query type %T", query))}
	}
}

func flattenPieChartStackDefinition(stackDefinition *dashboardservice.PieChartStackDefinition) *dashboardwidgets.PieChartStackDefinitionModel {
	if stackDefinition == nil {
		return nil
	}

	return &dashboardwidgets.PieChartStackDefinitionModel{
		MaxSlicesPerStack: int32PointerToTypeInt64(stackDefinition.MaxSlicesPerStack),
		StackNameTemplate: utils.StringPointerToTypeString(stackDefinition.StackNameTemplate),
	}
}

func flattenPieChartLabelDefinition(labelDefinition *dashboardservice.WidgetsPieChartLabelDefinition) *dashboardwidgets.LabelDefinitionModel {
	if labelDefinition == nil {
		return nil
	}
	return &dashboardwidgets.LabelDefinitionModel{
		LabelSource:    types.StringValue(dashboardwidgets.DashboardProtoToSchemaPieChartLabelSource[labelDefinition.GetLabelSource()]),
		IsVisible:      types.BoolPointerValue(labelDefinition.IsVisible),
		ShowName:       types.BoolPointerValue(labelDefinition.ShowName),
		ShowValue:      types.BoolPointerValue(labelDefinition.ShowValue),
		ShowPercentage: types.BoolPointerValue(labelDefinition.ShowPercentage),
	}
}

func flattenPieChartQueryMetrics(ctx context.Context, metrics *dashboardservice.PieChartMetricsQuery) (*dashboardwidgets.PieChartQueryModel, diag.Diagnostics) {
	if metrics == nil {
		return nil, nil
	}

	filters, diags := dashboardwidgets.FlattenMetricsFilters(ctx, metrics.GetFilters())
	if diags.HasError() {
		return nil, diags
	}
	timeFrame, diags := dashboardwidgets.FlattenTimeFrameSelect(ctx, metrics.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardwidgets.PieChartQueryModel{
		Metrics: &dashboardwidgets.PieChartQueryMetricsModel{
			PromqlQuery:      utils.StringPointerToTypeString(metrics.GetPromqlQuery().Value),
			Filters:          filters,
			GroupNames:       utils.StringSliceToTypeStringList(metrics.GetGroupNames()),
			StackedGroupName: utils.StringPointerToTypeString(metrics.StackedGroupName),
			TimeFrame:        timeFrame,
		},
	}, nil
}

func flattenPieChartQueryLogs(ctx context.Context, logs *dashboardservice.PieChartLogsQuery) (*dashboardwidgets.PieChartQueryModel, diag.Diagnostics) {
	if logs == nil {
		return nil, nil
	}

	filters, diags := dashboardwidgets.FlattenLogsFilters(ctx, logs.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	aggregation, diags := dashboardwidgets.FlattenLogsAggregation(ctx, logs.Aggregation)
	if diags.HasError() {
		return nil, diags
	}

	groupNamesFields, diags := dashboardwidgets.FlattenObservationFields(ctx, logs.GetGroupNamesFields())
	if diags.HasError() {
		return nil, diags
	}

	stackedGroupNameField, diags := dashboardwidgets.FlattenObservationField(ctx, logs.StackedGroupNameField)
	if diags.HasError() {
		return nil, diags
	}

	timeFrame, diags := dashboardwidgets.FlattenTimeFrameSelect(ctx, logs.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardwidgets.PieChartQueryModel{
		Logs: &dashboardwidgets.PieChartQueryLogsModel{
			LuceneQuery:           utils.StringPointerToTypeString(logs.GetLuceneQuery().Value),
			Aggregation:           aggregation,
			Filters:               filters,
			GroupNames:            utils.StringSliceToTypeStringList(logs.GetGroupNames()),
			StackedGroupName:      utils.StringPointerToTypeString(logs.StackedGroupName),
			GroupNamesFields:      groupNamesFields,
			StackedGroupNameField: stackedGroupNameField,
			TimeFrame:             timeFrame,
		},
	}, nil
}

func flattenPieChartQuerySpans(ctx context.Context, spans *dashboardservice.PieChartSpansQuery) (*dashboardwidgets.PieChartQueryModel, diag.Diagnostics) {
	if spans == nil {
		return nil, nil
	}

	filters, diags := dashboardwidgets.FlattenSpansFilters(ctx, spans.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	aggregation, dg := dashboardwidgets.FlattenSpansAggregation(spans.Aggregation)
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	stackedGroupName, dg := dashboardwidgets.FlattenSpansField(spans.StackedGroupName)
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	groupNames, diags := dashboardwidgets.FlattenSpansFields(ctx, spans.GetGroupNames())
	if diags.HasError() {
		return nil, diags
	}

	timeFrame, diags := dashboardwidgets.FlattenTimeFrameSelect(ctx, spans.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardwidgets.PieChartQueryModel{
		Spans: &dashboardwidgets.PieChartQuerySpansModel{
			LuceneQuery:      utils.StringPointerToTypeString(spans.GetLuceneQuery().Value),
			Filters:          filters,
			Aggregation:      aggregation,
			GroupNames:       groupNames,
			StackedGroupName: stackedGroupName,
			TimeFrame:        timeFrame,
		},
	}, nil
}

func flattenPieChartDataPrimeQuery(ctx context.Context, dataPrime *dashboardservice.PieChartDataprimeQuery) (*dashboardwidgets.PieChartQueryModel, diag.Diagnostics) {
	if dataPrime == nil {
		return nil, nil
	}

	filters, diags := dashboardwidgets.FlattenDashboardFiltersSources(ctx, dataPrime.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	timeFrame, diags := dashboardwidgets.FlattenTimeFrameSelect(ctx, dataPrime.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardwidgets.PieChartQueryModel{
		DataPrime: &dashboardwidgets.PieChartQueryDataPrimeModel{
			Query:            utils.StringPointerToTypeString(dataPrime.GetDataprimeQuery().Text),
			Filters:          filters,
			GroupNames:       utils.StringSliceToTypeStringList(dataPrime.GetGroupNames()),
			StackedGroupName: utils.StringPointerToTypeString(dataPrime.StackedGroupName),
			TimeFrame:        timeFrame,
		},
	}, nil
}

func flattenBarChart(ctx context.Context, barChart *dashboardservice.BarChart) (*dashboardwidgets.WidgetDefinitionModel, diag.Diagnostics) {
	if barChart == nil {
		return nil, nil
	}

	query, diags := flattenBarChartQuery(ctx, barChart.Query)
	if diags != nil {
		return nil, diags
	}

	colorsBy, dg := flattenBarChartColorsBy(barChart.ColorsBy)
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	xAxis, dg := flattenBarChartXAxis(barChart.XAxis)
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	return &dashboardwidgets.WidgetDefinitionModel{
		BarChart: &dashboardwidgets.BarChartModel{
			Query:             query,
			MaxBarsPerChart:   int32PointerToTypeInt64(barChart.MaxBarsPerChart),
			GroupNameTemplate: utils.StringPointerToTypeString(barChart.GroupNameTemplate),
			StackDefinition:   flattenBarChartStackDefinition(barChart.StackDefinition),
			ScaleType:         types.StringValue(dashboardwidgets.DashboardProtoToSchemaScaleType[barChart.GetScaleType()]),
			ColorsBy:          colorsBy,
			XAxis:             xAxis,
			Unit:              types.StringValue(dashboardwidgets.DashboardProtoToSchemaUnit[barChart.GetUnit()]),
			SortBy:            types.StringValue(dashboardwidgets.DashboardProtoToSchemaSortBy[barChart.GetSortBy()]),
			ColorScheme:       utils.StringPointerToTypeString(barChart.ColorScheme),
			DataModeType:      types.StringValue(dashboardwidgets.DashboardProtoToSchemaDataModeType[barChart.GetDataModeType()]),
		},
	}, nil
}

func flattenBarChartXAxis(axis *dashboardservice.XAxis) (*dashboardwidgets.BarChartXAxisModel, diag.Diagnostic) {
	if axis == nil {
		return nil, nil
	}

	switch {
	case axis.Time != nil:
		return &dashboardwidgets.BarChartXAxisModel{
			Time: &dashboardwidgets.BarChartXAxisTimeModel{
				Interval:         dashboardwidgets.OpenAPIDurationToGo(axis.Time.Interval),
				BucketsPresented: int32PointerToTypeInt64(axis.Time.BucketsPresented),
			},
		}, nil
	case axis.Value != nil:
		return &dashboardwidgets.BarChartXAxisModel{
			Value: &dashboardwidgets.BarChartXAxisValueModel{},
		}, nil
	default:
		return nil, diag.NewErrorDiagnostic("Error Flatten BarChart XAxis", fmt.Sprintf("unknown bar chart x axis type: %T", axis))
	}

}

func flattenBarChartQuery(ctx context.Context, query *dashboardservice.BarChartQuery) (*dashboardwidgets.BarChartQueryModel, diag.Diagnostics) {
	if query == nil {
		return nil, nil
	}

	switch {
	case query.Logs != nil:
		return flattenBarChartQueryLogs(ctx, query.Logs)
	case query.Spans != nil:
		return flattenBarChartQuerySpans(ctx, query.Spans)
	case query.Metrics != nil:
		return flattenBarChartQueryMetrics(ctx, query.Metrics)
	case query.Dataprime != nil:
		return flattenBarChartQueryDataPrime(ctx, query.Dataprime)
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten BarChart Query", fmt.Sprintf("unknown bar chart query type: %T", query))}
	}
}

func flattenBarChartQueryLogs(ctx context.Context, logs *dashboardservice.BarChartLogsQuery) (*dashboardwidgets.BarChartQueryModel, diag.Diagnostics) {
	if logs == nil {
		return nil, nil
	}

	filters, diags := dashboardwidgets.FlattenLogsFilters(ctx, logs.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	aggregation, diags := dashboardwidgets.FlattenLogsAggregation(ctx, logs.Aggregation)
	if diags.HasError() {
		return nil, diags
	}

	groupNamesFields, diags := dashboardwidgets.FlattenObservationFields(ctx, logs.GetGroupNamesFields())
	if diags.HasError() {
		return nil, diags
	}

	stackedGroupNameField, diags := dashboardwidgets.FlattenObservationField(ctx, logs.StackedGroupNameField)
	if diags.HasError() {
		return nil, diags
	}

	timeFrame, diags := dashboardwidgets.FlattenTimeFrameSelect(ctx, logs.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	flattenedLogs := &dashboardwidgets.BarChartQueryLogsModel{
		LuceneQuery:           utils.StringPointerToTypeString(logs.GetLuceneQuery().Value),
		Filters:               filters,
		Aggregation:           aggregation,
		GroupNames:            utils.StringSliceToTypeStringList(logs.GetGroupNames()),
		StackedGroupName:      utils.StringPointerToTypeString(logs.StackedGroupName),
		GroupNamesFields:      groupNamesFields,
		StackedGroupNameField: stackedGroupNameField,
		TimeFrame:             timeFrame,
	}

	logsObject, diags := types.ObjectValueFrom(ctx, barChartLogsQueryAttr(), flattenedLogs)
	if diags.HasError() {
		return nil, diags
	}
	return &dashboardwidgets.BarChartQueryModel{
		Logs:      logsObject,
		Metrics:   types.ObjectNull(barChartMetricsQueryAttr()),
		Spans:     types.ObjectNull(barChartSpansQueryAttr()),
		DataPrime: types.ObjectNull(barChartDataPrimeQueryAttr()),
	}, nil
}

func flattenBarChartQuerySpans(ctx context.Context, spans *dashboardservice.BarChartSpansQuery) (*dashboardwidgets.BarChartQueryModel, diag.Diagnostics) {
	if spans == nil {
		return nil, nil
	}

	filters, diags := dashboardwidgets.FlattenSpansFilters(ctx, spans.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	aggregation, dg := dashboardwidgets.FlattenSpansAggregation(spans.Aggregation)
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	groupNames, diags := dashboardwidgets.FlattenSpansFields(ctx, spans.GetGroupNames())
	if diags.HasError() {
		return nil, diags
	}

	stackedGroupName, dg := dashboardwidgets.FlattenSpansField(spans.StackedGroupName)
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	timeFrame, diags := dashboardwidgets.FlattenTimeFrameSelect(ctx, spans.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	flattenedSpans := &dashboardwidgets.BarChartQuerySpansModel{
		LuceneQuery:      utils.StringPointerToTypeString(spans.GetLuceneQuery().Value),
		Aggregation:      aggregation,
		Filters:          filters,
		GroupNames:       groupNames,
		StackedGroupName: stackedGroupName,
		TimeFrame:        timeFrame,
	}
	spansObject, diags := types.ObjectValueFrom(ctx, barChartSpansQueryAttr(), flattenedSpans)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardwidgets.BarChartQueryModel{
		Spans:     spansObject,
		Metrics:   types.ObjectNull(barChartMetricsQueryAttr()),
		Logs:      types.ObjectNull(barChartLogsQueryAttr()),
		DataPrime: types.ObjectNull(barChartDataPrimeQueryAttr()),
	}, nil
}

func flattenBarChartQueryMetrics(ctx context.Context, metrics *dashboardservice.BarChartMetricsQuery) (*dashboardwidgets.BarChartQueryModel, diag.Diagnostics) {
	if metrics == nil {
		return nil, nil
	}

	filters, diags := dashboardwidgets.FlattenMetricsFilters(ctx, metrics.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	timeFrame, diags := dashboardwidgets.FlattenTimeFrameSelect(ctx, metrics.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	flattenedMetric := &dashboardwidgets.BarChartQueryMetricsModel{
		PromqlQuery:      utils.StringPointerToTypeString(metrics.GetPromqlQuery().Value),
		Filters:          filters,
		GroupNames:       utils.StringSliceToTypeStringList(metrics.GetGroupNames()),
		StackedGroupName: utils.StringPointerToTypeString(metrics.StackedGroupName),
		TimeFrame:        timeFrame,
	}

	metricObject, diags := types.ObjectValueFrom(ctx, barChartMetricsQueryAttr(), flattenedMetric)
	if diags.HasError() {
		return nil, diags
	}
	return &dashboardwidgets.BarChartQueryModel{
		Logs:      types.ObjectNull(barChartLogsQueryAttr()),
		Spans:     types.ObjectNull(barChartSpansQueryAttr()),
		DataPrime: types.ObjectNull(barChartDataPrimeQueryAttr()),
		Metrics:   metricObject,
	}, nil
}

func flattenBarChartQueryDataPrime(ctx context.Context, dataPrime *dashboardservice.BarChartDataprimeQuery) (*dashboardwidgets.BarChartQueryModel, diag.Diagnostics) {
	if dataPrime == nil {
		return nil, nil
	}

	filters, diags := dashboardwidgets.FlattenDashboardFiltersSources(ctx, dataPrime.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	timeFrame, diags := dashboardwidgets.FlattenTimeFrameSelect(ctx, dataPrime.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	flattenedDataPrime := &dashboardwidgets.BarChartQueryDataPrimeModel{
		Query:            utils.StringPointerToTypeString(dataPrime.GetDataprimeQuery().Text),
		Filters:          filters,
		GroupNames:       utils.StringSliceToTypeStringList(dataPrime.GetGroupNames()),
		StackedGroupName: utils.StringPointerToTypeString(dataPrime.StackedGroupName),
		TimeFrame:        timeFrame,
	}

	dataPrimeObject, diags := types.ObjectValueFrom(ctx, barChartDataPrimeQueryAttr(), flattenedDataPrime)
	if diags.HasError() {
		return nil, diags
	}
	return &dashboardwidgets.BarChartQueryModel{
		Logs:      types.ObjectNull(barChartLogsQueryAttr()),
		Spans:     types.ObjectNull(barChartSpansQueryAttr()),
		Metrics:   types.ObjectNull(barChartMetricsQueryAttr()),
		DataPrime: dataPrimeObject,
	}, nil
}

func flattenBarChartStackDefinition(stackDefinition *dashboardservice.BarChartStackDefinition) *dashboardwidgets.BarChartStackDefinitionModel {
	if stackDefinition == nil {
		return nil
	}

	return &dashboardwidgets.BarChartStackDefinitionModel{
		MaxSlicesPerBar:   int32PointerToTypeInt64(stackDefinition.MaxSlicesPerBar),
		StackNameTemplate: utils.StringPointerToTypeString(stackDefinition.StackNameTemplate),
	}
}

func flattenHorizontalBarChartStackDefinition(stackDefinition *dashboardservice.HorizontalBarChartStackDefinition) *dashboardwidgets.BarChartStackDefinitionModel {
	if stackDefinition == nil {
		return nil
	}

	return &dashboardwidgets.BarChartStackDefinitionModel{
		MaxSlicesPerBar:   int32PointerToTypeInt64(stackDefinition.MaxSlicesPerBar),
		StackNameTemplate: utils.StringPointerToTypeString(stackDefinition.StackNameTemplate),
	}
}

func flattenBarChartColorsBy(colorsBy *dashboardservice.ColorsBy) (types.String, diag.Diagnostic) {
	if colorsBy == nil {
		return types.StringNull(), nil
	}
	switch {
	case colorsBy.GroupBy != nil:
		return types.StringValue("group_by"), nil
	case colorsBy.Stack != nil:
		return types.StringValue("stack"), nil
	case colorsBy.Aggregation != nil:
		return types.StringValue("aggregation"), nil
	default:
		return types.StringNull(), diag.NewErrorDiagnostic("", fmt.Sprintf("unknown colors by type %T", colorsBy))
	}
}

func flattenDashboardVariables(ctx context.Context, variables []dashboardservice.Variable) (types.List, diag.Diagnostics) {
	if len(variables) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: dashboardsVariablesModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	variablesElements := make([]attr.Value, 0)
	for i := range variables {
		flattenedVariable, diags := flattenDashboardVariable(ctx, &variables[i])
		if diags.HasError() {
			diagnostics = append(diagnostics, diags...)
			continue
		}

		variablesElement, diags := types.ObjectValueFrom(ctx, dashboardsVariablesModelAttr(), flattenedVariable)
		if diags.HasError() {
			diagnostics = append(diagnostics, diags...)
			continue
		}
		variablesElements = append(variablesElements, variablesElement)
	}

	return types.ListValueMust(types.ObjectType{AttrTypes: dashboardsVariablesModelAttr()}, variablesElements), diagnostics
}

func flattenDashboardVariable(ctx context.Context, variable *dashboardservice.Variable) (*DashboardVariableModel, diag.Diagnostics) {
	if variable == nil {
		return nil, nil
	}

	definition, diags := flattenDashboardVariableDefinition(ctx, variable.Definition)
	if diags.HasError() {
		return nil, diags
	}

	return &DashboardVariableModel{
		Name:        utils.StringPointerToTypeString(variable.Name),
		DisplayName: utils.StringPointerToTypeString(variable.DisplayName),
		Definition:  definition,
	}, nil
}

func flattenDashboardVariableDefinition(ctx context.Context, variableDefinition *dashboardservice.VariableDefinition) (*DashboardVariableDefinitionModel, diag.Diagnostics) {
	if variableDefinition == nil {
		return nil, nil
	}

	switch {
	case variableDefinition.Constant != nil:
		value := variableDefinition.Constant.Value
		values := []string{}
		if value != nil {
			values = append(values, *value)
		}
		return flattenDashboardVariableDefinitionMultiSelect(ctx, &dashboardservice.MultiSelect{
			Source: &dashboardservice.MultiSelectSource{
				ConstantList: &dashboardservice.ConstantListSource{
					Values: values,
				},
			},
			Selection: &dashboardservice.MultiSelectSelection{
				List: &dashboardservice.MultiSelectSelectionListSelection{
					Values: values,
				},
			},
		})
	case variableDefinition.MultiSelect != nil:
		return flattenDashboardVariableDefinitionMultiSelect(ctx, variableDefinition.MultiSelect)
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Dashboard Variable Definition", fmt.Sprintf("unknown variable definition type %T", variableDefinition))}
	}
}

func flattenDashboardVariableDefinitionMultiSelect(ctx context.Context, multiSelect *dashboardservice.MultiSelect) (*DashboardVariableDefinitionModel, diag.Diagnostics) {
	if multiSelect == nil {
		return nil, nil
	}

	source, diags := flattenDashboardVariableSource(ctx, multiSelect.Source)
	if diags.HasError() {
		return nil, diags
	}

	selectedValues, diags := flattenDashboardVariableSelectedValues(multiSelect.Selection)
	if diags.HasError() {
		return nil, diags
	}

	selectionType := types.StringNull()
	if multiSelect.SelectionOptions != nil && multiSelect.SelectionOptions.SelectionType != nil {
		switch *multiSelect.SelectionOptions.SelectionType {
		case dashboardservice.SELECTIONTYPE_SELECTION_TYPE_MULTI:
			selectionType = types.StringValue("multi")
		case dashboardservice.SELECTIONTYPE_SELECTION_TYPE_SINGLE:
			selectionType = types.StringValue("single")
		}
	}

	return &DashboardVariableDefinitionModel{
		ConstantValue: types.StringNull(),
		MultiSelect: &VariableMultiSelectModel{
			SelectedValues:       selectedValues,
			ValuesOrderDirection: types.StringValue(dashboardwidgets.DashboardOrderDirectionProtoToSchema[multiSelect.GetValuesOrderDirection()]),
			SelectionType:        selectionType,
			Source:               source,
		},
	}, nil
}

func flattenDashboardVariableSource(ctx context.Context, source *dashboardservice.MultiSelectSource) (*VariableMultiSelectSourceModel, diag.Diagnostics) {
	if source == nil {
		return nil, nil
	}

	result := &VariableMultiSelectSourceModel{
		LogsPath:     types.StringNull(),
		ConstantList: types.ListNull(types.StringType),
		Query:        types.ObjectNull(multiSelectQueryAttr()),
	}

	switch {
	case source.LogsPath != nil:
		result.LogsPath = utils.StringPointerToTypeString(source.LogsPath.Value)
	case source.MetricLabel != nil:
		result.MetricLabel = &MetricMultiSelectSourceModel{
			MetricName: utils.StringPointerToTypeString(source.MetricLabel.MetricName),
			Label:      utils.StringPointerToTypeString(source.MetricLabel.Label),
		}
	case source.ConstantList != nil:
		result.ConstantList = utils.StringSliceToTypeStringList(source.ConstantList.Values)
	case source.SpanField != nil:
		spansField, dg := dashboardwidgets.FlattenSpansField(source.SpanField.Value)
		if dg != nil {
			return nil, diag.Diagnostics{dg}
		}
		result.SpanField = spansField
	case source.Query != nil:
		query, diags := flattenDashboardVariableDefinitionMultiSelectQuery(ctx, source.Query)
		if diags != nil {
			return nil, diags
		}
		result.Query = query
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Dashboard Variable Definition Multi Select Source", fmt.Sprintf("unknown variable definition multi select source type %T", source))}
	}

	return result, nil
}

func flattenDashboardVariableDefinitionMultiSelectQuery(ctx context.Context, querySource *dashboardservice.MultiSelectQuerySource) (types.Object, diag.Diagnostics) {
	if querySource == nil {
		return types.ObjectNull(multiSelectQueryAttr()), nil
	}

	query, diags := flattenDashboardVariableDefinitionMultiSelectQueryModel(ctx, querySource.Query)
	if diags.HasError() {
		return types.ObjectNull(multiSelectQueryAttr()), diags
	}

	valueDisplayOptions, diags := flattenDashboardVariableDefinitionMultiSelectValueDisplayOptions(ctx, querySource.ValueDisplayOptions)
	if diags.HasError() {
		return types.ObjectNull(multiSelectQueryAttr()), diags
	}

	return types.ObjectValueFrom(ctx, multiSelectQueryAttr(), &VariableMultiSelectQueryModel{
		Query:               query,
		RefreshStrategy:     types.StringValue(dashboardwidgets.DashboardProtoToSchemaRefreshStrategy[querySource.GetRefreshStrategy()]),
		ValueDisplayOptions: valueDisplayOptions,
	})
}

func flattenDashboardVariableDefinitionMultiSelectQueryModel(ctx context.Context, query *dashboardservice.MultiSelectQuery) (types.Object, diag.Diagnostics) {
	if query == nil {
		return types.ObjectNull(multiSelectQueryModelAttr()), nil
	}

	multiSelectQueryModel := &MultiSelectQueryModel{
		Logs:    types.ObjectNull(multiSelectQueryLogsQueryModelAttr()),
		Metrics: types.ObjectNull(multiSelectQueryMetricsQueryModelAttr()),
		Spans:   types.ObjectNull(multiSelectQuerySpansQueryModelAttr()),
	}
	var diags diag.Diagnostics
	switch {
	case query.LogsQuery != nil:
		multiSelectQueryModel.Logs, diags = flattenDashboardVariableDefinitionMultiSelectQueryLogsModel(ctx, query.LogsQuery)
	case query.MetricsQuery != nil:
		multiSelectQueryModel.Metrics, diags = flattenDashboardVariableDefinitionMultiSelectQueryMetricsModel(ctx, query.MetricsQuery)
	case query.SpansQuery != nil:
		multiSelectQueryModel.Spans, diags = flattenDashboardVariableDefinitionMultiSelectQuerySpansModel(ctx, query.SpansQuery)
	}

	if diags.HasError() {
		return types.ObjectNull(multiSelectQueryModelAttr()), diags
	}

	return types.ObjectValueFrom(ctx, multiSelectQueryModelAttr(), multiSelectQueryModel)
}

func flattenDashboardVariableDefinitionMultiSelectQueryLogsModel(ctx context.Context, query *dashboardservice.QueryLogsQuery) (types.Object, diag.Diagnostics) {
	if query == nil {
		return types.ObjectNull(multiSelectQueryLogsQueryModelAttr()), nil
	}

	logsQuery := &MultiSelectLogsQueryModel{
		FieldName:  types.ObjectNull(multiSelectQueryLogsQueryFieldNameModelAttr()),
		FieldValue: types.ObjectNull(multiSelectQueryLogsQueryFieldValueModelAttr()),
	}

	var diags diag.Diagnostics
	queryType := query.GetType()
	switch {
	case queryType.FieldName != nil:
		logsQuery.FieldName, diags = flattenDashboardVariableDefinitionMultiSelectQueryLogsFieldNameModel(ctx, queryType.FieldName)
	case queryType.FieldValue != nil:
		logsQuery.FieldValue, diags = flattenDashboardVariableDefinitionMultiSelectQueryLogsFieldValueModel(ctx, queryType.FieldValue)
	}

	if diags.HasError() {
		return types.ObjectNull(multiSelectQueryLogsQueryModelAttr()), diags
	}

	return types.ObjectValueFrom(ctx, multiSelectQueryLogsQueryModelAttr(), logsQuery)
}

func flattenDashboardVariableDefinitionMultiSelectQueryLogsFieldNameModel(ctx context.Context, name *dashboardservice.QueryLogsQueryTypeFieldName) (types.Object, diag.Diagnostics) {
	if name == nil {
		return types.ObjectNull(multiSelectQueryLogsQueryFieldNameModelAttr()), nil
	}

	return types.ObjectValueFrom(ctx, multiSelectQueryLogsQueryFieldNameModelAttr(), &LogFieldNameModel{
		LogRegex: utils.StringPointerToTypeString(name.LogRegex),
	})
}

func flattenDashboardVariableDefinitionMultiSelectQueryLogsFieldValueModel(ctx context.Context, value *dashboardservice.QueryLogsQueryTypeFieldValue) (types.Object, diag.Diagnostics) {
	if value == nil {
		return types.ObjectNull(multiSelectQueryLogsQueryFieldValueModelAttr()), nil
	}

	observationField, diags := dashboardwidgets.FlattenObservationField(ctx, value.ObservationField)
	if diags.HasError() {
		return types.ObjectNull(multiSelectQueryLogsQueryFieldValueModelAttr()), diags
	}

	return types.ObjectValueFrom(ctx, multiSelectQueryLogsQueryFieldValueModelAttr(), &FieldValueModel{
		ObservationField: observationField,
	})
}

func flattenDashboardVariableDefinitionMultiSelectQueryMetricsModel(ctx context.Context, query *dashboardservice.QueryMetricsQuery) (types.Object, diag.Diagnostics) {
	if query == nil {
		return types.ObjectNull(multiSelectQueryMetricsQueryModelAttr()), nil
	}

	var diags diag.Diagnostics
	metricQuery := &MultiSelectMetricsQueryModel{
		MetricName: types.ObjectNull(multiSelectQueryMetricsNameAttr()),
		LabelName:  types.ObjectNull(multiSelectQueryMetricsNameAttr()),
		LabelValue: types.ObjectNull(multiSelectQueryLabelValueModelAttr()),
	}

	queryType := query.GetType()
	switch {
	case queryType.MetricName != nil:
		metricQuery.MetricName, diags = flattenDashboardVariableDefinitionMultiSelectQueryMetricsMetricNameModel(ctx, queryType.MetricName)
	case queryType.LabelName != nil:
		metricQuery.LabelName, diags = flattenDashboardVariableDefinitionMultiSelectQueryMetricsLabelNameModel(ctx, queryType.LabelName)
	case queryType.LabelValue != nil:
		metricQuery.LabelValue, diags = flattenDashboardVariableDefinitionMultiSelectQueryMetricsLabelValueModel(ctx, queryType.LabelValue)
	}

	if diags.HasError() {
		return types.ObjectNull(multiSelectQueryMetricsQueryModelAttr()), diags
	}

	return types.ObjectValueFrom(ctx, multiSelectQueryMetricsQueryModelAttr(), metricQuery)
}

func flattenDashboardVariableDefinitionMultiSelectQueryMetricsMetricNameModel(ctx context.Context, name *dashboardservice.QueryMetricsQueryTypeMetricName) (types.Object, diag.Diagnostics) {
	if name == nil {
		return types.ObjectNull(multiSelectQueryMetricsNameAttr()), nil
	}

	return types.ObjectValueFrom(ctx, multiSelectQueryMetricsNameAttr(), &MetricAndLabelNameModel{
		MetricRegex: utils.StringPointerToTypeString(name.MetricRegex),
	})
}

func flattenDashboardVariableDefinitionMultiSelectQueryMetricsLabelNameModel(ctx context.Context, name *dashboardservice.QueryMetricsQueryTypeLabelName) (types.Object, diag.Diagnostics) {
	if name == nil {
		return types.ObjectNull(multiSelectQueryMetricsNameAttr()), nil
	}

	return types.ObjectValueFrom(ctx, multiSelectQueryMetricsNameAttr(), &MetricAndLabelNameModel{
		MetricRegex: utils.StringPointerToTypeString(name.MetricRegex),
	})
}

func flattenDashboardVariableDefinitionMultiSelectQueryMetricsLabelValueModel(ctx context.Context, value *dashboardservice.QueryMetricsQueryTypeLabelValue) (types.Object, diag.Diagnostics) {
	if value == nil {
		return types.ObjectNull(multiSelectQueryLabelValueModelAttr()), nil
	}

	metricName, diags := flattenMultiSelectQueryMetricsQueryStringOrVariable(ctx, value.MetricName)
	if diags.HasError() {
		return types.ObjectNull(multiSelectQueryLabelValueModelAttr()), diags
	}

	labelName, diags := flattenMultiSelectQueryMetricsQueryStringOrVariable(ctx, value.LabelName)
	if diags.HasError() {
		return types.ObjectNull(multiSelectQueryLabelValueModelAttr()), diags
	}

	labelFilters, diags := flattenMultiSelectQueryMetricsQueryMetricsLabelFilters(ctx, value.GetLabelFilters())
	if diags.HasError() {
		return types.ObjectNull(multiSelectQueryLabelValueModelAttr()), diags
	}

	return types.ObjectValueFrom(ctx, multiSelectQueryLabelValueModelAttr(), &LabelValueModel{
		MetricName:   metricName,
		LabelName:    labelName,
		LabelFilters: labelFilters,
	})
}

func flattenMultiSelectQueryMetricsQueryMetricsLabelFilters(ctx context.Context, filters []dashboardservice.QueryMetricsQueryMetricsLabelFilter) (types.List, diag.Diagnostics) {
	if len(filters) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: multiSelectQueryLabelFilterAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	flattenedFilters := make([]attr.Value, 0)
	for _, filter := range filters {
		flattenedFilter, diags := flattenMultiSelectQueryMetricsQueryMetricsLabelFilter(ctx, &filter)
		if diags.HasError() {
			diagnostics.Append(diags...)
			continue
		}
		filtersElement, diags := types.ObjectValueFrom(ctx, multiSelectQueryLabelFilterAttr(), flattenedFilter)
		if diags.HasError() {
			diagnostics.Append(diags...)
			continue
		}
		flattenedFilters = append(flattenedFilters, filtersElement)
	}

	if diagnostics.HasError() {
		return types.ListNull(types.ObjectType{AttrTypes: multiSelectQueryLabelFilterAttr()}), diagnostics
	}

	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: multiSelectQueryLabelFilterAttr()}, flattenedFilters)
}

func flattenMultiSelectQueryMetricsQueryMetricsLabelFilter(ctx context.Context, filter *dashboardservice.QueryMetricsQueryMetricsLabelFilter) (*MetricLabelFilterModel, diag.Diagnostics) {
	if filter == nil {
		return nil, nil
	}

	metric, diags := flattenMultiSelectQueryMetricsQueryStringOrVariable(ctx, filter.Metric)
	if diags.HasError() {
		return nil, diags
	}

	label, diags := flattenMultiSelectQueryMetricsQueryStringOrVariable(ctx, filter.Label)
	if diags.HasError() {
		return nil, diags
	}

	operator, diags := flattenMultiSelectQueryMetricsQueryMetricsLabelFilterOperator(ctx, filter.Operator)
	if diags.HasError() {
		return nil, diags
	}

	return &MetricLabelFilterModel{
		Metric:   metric,
		Label:    label,
		Operator: operator,
	}, nil
}

func flattenMultiSelectQueryMetricsQueryMetricsLabelFilterOperator(ctx context.Context, operator *dashboardservice.QueryMetricsQueryOperator) (types.Object, diag.Diagnostics) {
	if operator == nil {
		return types.ObjectNull(multiSelectQueryMetricsQueryMetricsLabelFilterOperatorAttr()), nil
	}

	var diags diag.Diagnostics
	metricLabelFilterOperatorModel := &MetricLabelFilterOperatorModel{}
	switch {
	case operator.Equals != nil:
		metricLabelFilterOperatorModel.Type = types.StringValue("equals")
		var values []dashboardservice.QueryMetricsQueryStringOrVariable
		if operator.Equals.Selection != nil && operator.Equals.Selection.List != nil {
			values = operator.Equals.Selection.List.Values
		}
		metricLabelFilterOperatorModel.SelectedValues, diags = flattenMultiSelectQueryMetricsQueryOperatorSelectedValues(ctx, values)
	case operator.NotEquals != nil:
		metricLabelFilterOperatorModel.Type = types.StringValue("not_equals")
		var values []dashboardservice.QueryMetricsQueryStringOrVariable
		if operator.NotEquals.Selection != nil && operator.NotEquals.Selection.List != nil {
			values = operator.NotEquals.Selection.List.Values
		}
		metricLabelFilterOperatorModel.SelectedValues, diags = flattenMultiSelectQueryMetricsQueryOperatorSelectedValues(ctx, values)
	}

	if diags.HasError() {
		return types.ObjectNull(multiSelectQueryMetricsQueryMetricsLabelFilterOperatorAttr()), diags
	}
	return types.ObjectValueFrom(ctx, multiSelectQueryMetricsQueryMetricsLabelFilterOperatorAttr(), metricLabelFilterOperatorModel)
}

func flattenMultiSelectQueryMetricsQueryOperatorSelectedValues(ctx context.Context, values []dashboardservice.QueryMetricsQueryStringOrVariable) (types.List, diag.Diagnostics) {
	if len(values) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: multiSelectQueryStringOrValueAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	flattenedValues := make([]types.Object, 0)
	for _, value := range values {
		flattenedValue, diags := flattenMultiSelectQueryMetricsQueryStringOrVariable(ctx, &value)
		if diags.HasError() {
			diagnostics.Append(diags...)
			continue
		}
		valuesElement, diags := types.ObjectValueFrom(ctx, multiSelectQueryStringOrValueAttr(), flattenedValue)
		if diags.HasError() {
			diagnostics.Append(diags...)
			continue
		}
		flattenedValues = append(flattenedValues, valuesElement)
	}

	if diagnostics.HasError() {
		return types.ListNull(types.ObjectType{AttrTypes: multiSelectQueryStringOrValueAttr()}), diagnostics
	}

	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: multiSelectQueryStringOrValueAttr()}, flattenedValues)
}

func flattenMultiSelectQueryMetricsQueryStringOrVariable(ctx context.Context, stringOrVariable *dashboardservice.QueryMetricsQueryStringOrVariable) (types.Object, diag.Diagnostics) {
	if stringOrVariable == nil {
		return types.ObjectNull(multiSelectQueryStringOrValueAttr()), nil
	}

	metricLabelFilterOperatorSelectedValuesModel := &MetricLabelFilterOperatorSelectedValuesModel{
		StringValue:  types.StringNull(),
		VariableName: types.StringNull(),
	}

	switch {
	case stringOrVariable.StringValue != nil:
		metricLabelFilterOperatorSelectedValuesModel.StringValue = utils.StringPointerToTypeString(stringOrVariable.StringValue)
	case stringOrVariable.VariableName != nil:
		metricLabelFilterOperatorSelectedValuesModel.VariableName = utils.StringPointerToTypeString(stringOrVariable.VariableName)
	}

	return types.ObjectValueFrom(ctx, multiSelectQueryStringOrValueAttr(), metricLabelFilterOperatorSelectedValuesModel)
}

func flattenDashboardVariableDefinitionMultiSelectQuerySpansModel(ctx context.Context, query *dashboardservice.QuerySpansQuery) (types.Object, diag.Diagnostics) {
	if query == nil {
		return types.ObjectNull(multiSelectQuerySpansQueryModelAttr()), nil
	}

	var diags diag.Diagnostics
	multiSelectSpansQueryModel := &MultiSelectSpansQueryModel{
		FieldName:  types.ObjectNull(spansQueryFieldNameAttr()),
		FieldValue: types.ObjectNull(dashboardwidgets.SpansFieldModelAttr()),
	}
	queryType := query.GetType()
	switch {
	case queryType.FieldName != nil:
		multiSelectSpansQueryModel.FieldName, diags = flattenMultiSelectQuerySpansFieldName(ctx, queryType.FieldName)
	case queryType.FieldValue != nil:
		multiSelectSpansQueryModel.FieldValue, diags = flattenMultiSelectQuerySpansFieldValue(ctx, queryType.FieldValue)
	default:
		return types.ObjectNull(multiSelectQuerySpansQueryModelAttr()), diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Dashboard Variable Definition Multi Select Query Spans Model", fmt.Sprintf("unknown variable definition multi select query spans type %T", queryType))}
	}

	if diags.HasError() {
		return types.ObjectNull(multiSelectQuerySpansQueryModelAttr()), diags
	}

	return types.ObjectValueFrom(ctx, multiSelectQuerySpansQueryModelAttr(), multiSelectSpansQueryModel)
}

func flattenMultiSelectQuerySpansFieldName(ctx context.Context, name *dashboardservice.QuerySpansQueryTypeFieldName) (types.Object, diag.Diagnostics) {
	if name == nil {
		return types.ObjectNull(spansQueryFieldNameAttr()), nil
	}

	return types.ObjectValueFrom(ctx, spansQueryFieldNameAttr(), &SpanFieldNameModel{
		SpanRegex: utils.StringPointerToTypeString(name.SpanRegex),
	})
}

func flattenMultiSelectQuerySpansFieldValue(ctx context.Context, value *dashboardservice.QuerySpansQueryTypeFieldValue) (types.Object, diag.Diagnostics) {
	if value == nil || value.Value == nil {
		return types.ObjectNull(dashboardwidgets.SpansFieldModelAttr()), nil
	}

	spanField, dg := dashboardwidgets.FlattenSpansField(value.Value)
	if dg != nil {
		return types.ObjectNull(dashboardwidgets.SpansFieldModelAttr()), diag.Diagnostics{dg}
	}

	return types.ObjectValueFrom(ctx, dashboardwidgets.SpansFieldModelAttr(), spanField)
}

func flattenDashboardVariableDefinitionMultiSelectValueDisplayOptions(ctx context.Context, options *dashboardservice.MultiSelectValueDisplayOptions) (types.Object, diag.Diagnostics) {
	if options == nil {
		return types.ObjectNull(multiSelectValueDisplayOptionsModelAttr()), nil
	}

	return types.ObjectValueFrom(ctx, multiSelectValueDisplayOptionsModelAttr(), &MultiSelectValueDisplayOptionsModel{
		ValueRegex: utils.StringPointerToTypeString(options.ValueRegex),
		LabelRegex: utils.StringPointerToTypeString(options.LabelRegex),
	})
}

func multiSelectQueryAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"query": types.ObjectType{
			AttrTypes: multiSelectQueryOptionsAttr(),
		},
		"refresh_strategy": types.StringType,
		"value_display_options": types.ObjectType{
			AttrTypes: multiSelectValueDisplayOptionsModelAttr(),
		},
	}
}

func multiSelectQueryOptionsAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"logs":    types.ObjectType{AttrTypes: multiSelectQueryLogsQueryModelAttr()},
		"metrics": types.ObjectType{AttrTypes: multiSelectQueryMetricsQueryModelAttr()},
		"spans":   types.ObjectType{AttrTypes: multiSelectQuerySpansQueryModelAttr()},
	}
}

func multiSelectQueryLogsQueryModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"field_name":  types.ObjectType{AttrTypes: multiSelectQueryLogsQueryFieldNameModelAttr()},
		"field_value": types.ObjectType{AttrTypes: multiSelectQueryLogsQueryFieldValueModelAttr()},
	}
}

func multiSelectQueryMetricsQueryModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"metric_name": types.ObjectType{AttrTypes: multiSelectQueryMetricsNameAttr()},
		"label_name":  types.ObjectType{AttrTypes: multiSelectQueryMetricsNameAttr()},
		"label_value": types.ObjectType{AttrTypes: multiSelectQueryLabelValueModelAttr()},
	}
}

func multiSelectQueryMetricsNameAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"metric_regex": types.StringType,
	}
}

func multiSelectQueryLabelValueModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"metric_name": types.ObjectType{AttrTypes: multiSelectQueryStringOrValueAttr()},
		"label_name":  types.ObjectType{AttrTypes: multiSelectQueryStringOrValueAttr()},
		"label_filters": types.ListType{
			ElemType: types.ObjectType{
				AttrTypes: multiSelectQueryLabelFilterAttr(),
			},
		},
	}
}

func multiSelectQueryLabelFilterAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"metric":   types.ObjectType{AttrTypes: multiSelectQueryStringOrValueAttr()},
		"label":    types.ObjectType{AttrTypes: multiSelectQueryStringOrValueAttr()},
		"operator": types.ObjectType{AttrTypes: multiSelectQueryMetricsQueryMetricsLabelFilterOperatorAttr()},
	}
}

func multiSelectQueryModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"logs":    types.ObjectType{AttrTypes: multiSelectQueryLogsQueryModelAttr()},
		"metrics": types.ObjectType{AttrTypes: multiSelectQueryMetricsQueryModelAttr()},
		"spans":   types.ObjectType{AttrTypes: multiSelectQuerySpansQueryModelAttr()},
	}
}

func multiSelectQueryLogsQueryFieldNameModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"log_regex": types.StringType,
	}
}

func multiSelectQueryLogsQueryFieldValueModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"observation_field": types.ObjectType{AttrTypes: dashboardwidgets.ObservationFieldAttr()},
	}
}

func multiSelectQueryMetricsQueryMetricsLabelFilterOperatorAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"type": types.StringType,
		"selected_values": types.ListType{ElemType: types.ObjectType{
			AttrTypes: multiSelectQueryStringOrValueAttr(),
		}},
	}
}

func multiSelectQueryStringOrValueAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"string_value":  types.StringType,
		"variable_name": types.StringType,
	}
}

func multiSelectValueDisplayOptionsModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"value_regex": types.StringType,
		"label_regex": types.StringType,
	}
}

func multiSelectQuerySpansQueryModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"field_name":  types.ObjectType{AttrTypes: spansQueryFieldNameAttr()},
		"field_value": types.ObjectType{AttrTypes: dashboardwidgets.SpansFieldModelAttr()},
	}
}

func spansQueryFieldNameAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"span_regex": types.StringType,
	}
}

func flattenDashboardVariableSelectedValues(selection *dashboardservice.MultiSelectSelection) (types.List, diag.Diagnostics) {
	switch {
	case selection == nil:
		return types.ListNull(types.StringType), nil
	case selection.List != nil:
		return utils.StringSliceToTypeStringList(selection.List.Values), nil
	case selection.All != nil:
		return types.ListValueMust(types.StringType, []attr.Value{}), nil
	default:
		return types.ListNull(types.StringType), diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Dashboard Variable Definition Multi Select Selection", fmt.Sprintf("unknown variable definition multi select selection type %T", selection))}
	}
}

func flattenDashboardFilters(ctx context.Context, filters []dashboardservice.FiltersFilter) (types.List, diag.Diagnostics) {
	if len(filters) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: dashboardsFiltersModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	filtersElements := make([]attr.Value, 0)
	for i := range filters {
		flattenedFilter, dgs := flattenDashboardFilter(ctx, &filters[i])
		if dgs.HasError() {
			diagnostics.Append(dgs...)
			continue
		}
		filterElement, diags := types.ObjectValueFrom(ctx, dashboardsFiltersModelAttr(), flattenedFilter)
		if diags.HasError() {
			diagnostics.Append(diags...)
			continue
		}
		filtersElements = append(filtersElements, filterElement)
	}

	return types.ListValueMust(types.ObjectType{AttrTypes: dashboardsFiltersModelAttr()}, filtersElements), diagnostics
}

func flattenDashboardFilter(ctx context.Context, filter *dashboardservice.FiltersFilter) (*DashboardFilterModel, diag.Diagnostics) {
	if filter == nil {
		return nil, nil
	}

	source, diags := dashboardwidgets.FlattenDashboardFilterSource(ctx, filter.Source)
	if diags != nil {
		return nil, diags
	}

	return &DashboardFilterModel{
		Source:    source,
		Enabled:   types.BoolPointerValue(filter.Enabled),
		Collapsed: types.BoolPointerValue(filter.Collapsed),
	}, nil
}

func flattenDashboardFolder(ctx context.Context, planedDashboard types.Object, dashboard *dashboardservice.Dashboard) (types.Object, diag.Diagnostics) {
	planPath := types.StringNull()
	planID := types.StringNull()
	if !utils.ObjIsNullOrUnknown(planedDashboard) {
		var folderModel DashboardFolderModel
		dgs := planedDashboard.As(ctx, &folderModel, basetypes.ObjectAsOptions{})
		if dgs.HasError() {
			return types.ObjectNull(dashboardFolderModelAttr()), dgs
		}
		if !(folderModel.Path.IsUnknown() || folderModel.Path.IsNull()) {
			planPath = folderModel.Path
		}
		if !(folderModel.ID.IsUnknown() || folderModel.ID.IsNull()) {
			planID = folderModel.ID
		}
	}

	if dashboard.FolderId != nil {
		if !planPath.IsNull() {
			return types.ObjectValueFrom(ctx, dashboardFolderModelAttr(), &DashboardFolderModel{
				ID:   types.StringNull(),
				Path: planPath,
			})
		}
		return types.ObjectValueFrom(ctx, dashboardFolderModelAttr(), &DashboardFolderModel{
			ID:   types.StringValue(dashboard.FolderId.GetValue()),
			Path: types.StringNull(),
		})
	} else if dashboard.FolderPath != nil {
		if !planID.IsNull() {
			return types.ObjectValueFrom(ctx, dashboardFolderModelAttr(), &DashboardFolderModel{
				ID:   planID,
				Path: types.StringNull(),
			})
		}
		return types.ObjectValueFrom(ctx, dashboardFolderModelAttr(), &DashboardFolderModel{
			ID:   types.StringNull(),
			Path: types.StringValue(strings.Join(dashboard.FolderPath.GetSegments(), "/")),
		})
	}
	return types.ObjectNull(dashboardFolderModelAttr()), nil
}

func flattenOpenAPIDashboardFolder(ctx context.Context, planedDashboard types.Object, dashboard *dashboardservice.Dashboard) (types.Object, diag.Diagnostics) {
	planPath := types.StringNull()
	planID := types.StringNull()
	if !utils.ObjIsNullOrUnknown(planedDashboard) {
		var folderModel DashboardFolderModel
		dgs := planedDashboard.As(ctx, &folderModel, basetypes.ObjectAsOptions{})
		if dgs.HasError() {
			return types.ObjectNull(dashboardFolderModelAttr()), dgs
		}
		if !(folderModel.Path.IsUnknown() || folderModel.Path.IsNull()) {
			planPath = folderModel.Path
		}
		if !(folderModel.ID.IsUnknown() || folderModel.ID.IsNull()) {
			planID = folderModel.ID
		}
	}

	if dashboard.FolderId != nil {
		if !planPath.IsNull() {
			return types.ObjectValueFrom(ctx, dashboardFolderModelAttr(), &DashboardFolderModel{
				ID:   types.StringNull(),
				Path: planPath,
			})
		}
		return types.ObjectValueFrom(ctx, dashboardFolderModelAttr(), &DashboardFolderModel{
			ID:   types.StringValue(dashboard.FolderId.GetValue()),
			Path: types.StringNull(),
		})
	} else if dashboard.FolderPath != nil {
		if !planID.IsNull() {
			return types.ObjectValueFrom(ctx, dashboardFolderModelAttr(), &DashboardFolderModel{
				ID:   planID,
				Path: types.StringNull(),
			})
		}
		return types.ObjectValueFrom(ctx, dashboardFolderModelAttr(), &DashboardFolderModel{
			ID:   types.StringNull(),
			Path: types.StringValue(strings.Join(dashboard.FolderPath.GetSegments(), "/")),
		})
	}
	return types.ObjectNull(dashboardFolderModelAttr()), nil
}

func flattenDashboardAnnotations(ctx context.Context, annotations []dashboardservice.Annotation) (types.List, diag.Diagnostics) {
	if len(annotations) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: dashboardsAnnotationsModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	annotationsElements := make([]attr.Value, 0)
	for _, annotation := range annotations {
		flattenedAnnotation, diags := flattenDashboardAnnotation(ctx, &annotation)
		if diags.HasError() {
			diagnostics.Append(diags...)
			continue
		}
		annotationElement, diags := types.ObjectValueFrom(ctx, dashboardsAnnotationsModelAttr(), flattenedAnnotation)
		if diags.HasError() {
			diagnostics.Append(diags...)
			continue
		}
		annotationsElements = append(annotationsElements, annotationElement)
	}

	return types.ListValueMust(types.ObjectType{AttrTypes: dashboardsAnnotationsModelAttr()}, annotationsElements), diagnostics
}

func flattenDashboardAnnotation(ctx context.Context, annotation *dashboardservice.Annotation) (*DashboardAnnotationModel, diag.Diagnostics) {
	if annotation == nil {
		return nil, nil
	}

	source, diags := flattenDashboardAnnotationSource(ctx, annotation.Source)
	if diags.HasError() {
		return nil, diags
	}

	return &DashboardAnnotationModel{
		ID:      utils.StringPointerToTypeString(annotation.Id),
		Name:    utils.StringPointerToTypeString(annotation.Name),
		Enabled: types.BoolPointerValue(annotation.Enabled),
		Source:  source,
	}, nil
}

func flattenDashboardAnnotationSource(ctx context.Context, source *dashboardservice.AnnotationSource) (types.Object, diag.Diagnostics) {
	if source == nil {
		return types.ObjectNull(dashboardsAnnotationsModelAttr()), nil
	}

	var sourceObject DashboardAnnotationSourceModel
	var diags diag.Diagnostics
	switch {
	case source.Metrics != nil:
		sourceObject.Metrics, diags = flattenDashboardAnnotationMetricSourceModel(ctx, source.Metrics)
		sourceObject.Logs = types.ObjectNull(annotationsLogsAndSpansSourceModelAttr())
		sourceObject.Spans = types.ObjectNull(annotationsLogsAndSpansSourceModelAttr())
		sourceObject.Manual = types.ObjectNull(annotationsManualSourceModelAttr())
	case source.Logs != nil:
		sourceObject.Logs, diags = flattenDashboardAnnotationLogsSourceModel(ctx, source.Logs)
		sourceObject.Metrics = types.ObjectNull(annotationsMetricsSourceModelAttr())
		sourceObject.Spans = types.ObjectNull(annotationsLogsAndSpansSourceModelAttr())
		sourceObject.Manual = types.ObjectNull(annotationsManualSourceModelAttr())
	case source.Spans != nil:
		sourceObject.Spans, diags = flattenDashboardAnnotationSpansSourceModel(ctx, source.Spans)
		sourceObject.Metrics = types.ObjectNull(annotationsMetricsSourceModelAttr())
		sourceObject.Logs = types.ObjectNull(annotationsLogsAndSpansSourceModelAttr())
		sourceObject.Manual = types.ObjectNull(annotationsManualSourceModelAttr())
	case source.Manual != nil:
		sourceObject.Manual, diags = flattenDashboardAnnotationManualSourceModel(ctx, source.Manual)
		sourceObject.Metrics = types.ObjectNull(annotationsMetricsSourceModelAttr())
		sourceObject.Logs = types.ObjectNull(annotationsLogsAndSpansSourceModelAttr())
		sourceObject.Spans = types.ObjectNull(annotationsLogsAndSpansSourceModelAttr())
	default:
		diags = diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Dashboard Annotation Source", fmt.Sprintf("unknown annotation source type %T", source))}
	}

	if diags.HasError() {
		return types.ObjectNull(annotationSourceModelAttr()), diags
	}

	return types.ObjectValueFrom(ctx, annotationSourceModelAttr(), sourceObject)
}

func flattenDashboardAnnotationManualSourceModel(ctx context.Context, manual *dashboardservice.ManualSource) (types.Object, diag.Diagnostics) {
	if manual == nil {
		return types.ObjectNull(annotationsManualSourceModelAttr()), nil
	}

	strategy, diags := flattenAnnotationManualStrategy(ctx, manual.Strategy)
	if diags.HasError() {
		return types.ObjectNull(annotationsManualSourceModelAttr()), diags
	}

	manualObject := &DashboardAnnotationManualSourceModel{
		Orientation:     flattenManualAnnotationOrientation(manual.GetOrientation()),
		MessageTemplate: utils.StringPointerToTypeString(manual.MessageTemplate),
		Strategy:        strategy,
	}

	return types.ObjectValueFrom(ctx, annotationsManualSourceModelAttr(), manualObject)
}

func flattenAnnotationManualStrategy(ctx context.Context, strategy *dashboardservice.ManualSourceStrategy) (types.Object, diag.Diagnostics) {
	if strategy == nil {
		return types.ObjectNull(manualStrategyModelAttr()), nil
	}

	var strategyModel DashboardAnnotationManualStrategyModel
	var diags diag.Diagnostics
	switch {
	case strategy.Instant != nil:
		strategyModel.Instant, diags = flattenManualStrategyInstant(ctx, strategy.Instant)
		strategyModel.Range = types.ObjectNull(manualRangeStrategyModelAttr())
	case strategy.Range != nil:
		strategyModel.Range, diags = flattenManualStrategyRange(ctx, strategy.Range)
		strategyModel.Instant = types.ObjectNull(manualInstantStrategyModelAttr())
	default:
		diags = diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Annotation Manual Strategy", fmt.Sprintf("unknown annotation manual strategy type %T", strategy))}
	}

	if diags.HasError() {
		return types.ObjectNull(manualStrategyModelAttr()), diags
	}

	return types.ObjectValueFrom(ctx, manualStrategyModelAttr(), strategyModel)
}

func flattenManualStrategyInstant(ctx context.Context, instant *dashboardservice.ManualSourceStrategyInstant) (types.Object, diag.Diagnostics) {
	if instant == nil {
		return types.ObjectNull(manualInstantStrategyModelAttr()), nil
	}

	instantStrategy := &DashboardAnnotationManualInstantStrategyModel{
		Value:      types.Float64PointerValue(instant.Value),
		Unit:       types.StringValue(dashboardwidgets.DashboardProtoToSchemaUnit[instant.GetUnit()]),
		CustomUnit: utils.StringPointerToTypeString(instant.CustomUnit),
	}

	return types.ObjectValueFrom(ctx, manualInstantStrategyModelAttr(), instantStrategy)
}

func flattenManualStrategyRange(ctx context.Context, getRange *dashboardservice.ManualSourceStrategyRange) (types.Object, diag.Diagnostics) {
	if getRange == nil {
		return types.ObjectNull(manualRangeStrategyModelAttr()), nil
	}

	rangeStrategy := &DashboardAnnotationManualRangeStrategyModel{
		StartValue: types.Float64PointerValue(getRange.StartValue),
		EndValue:   types.Float64PointerValue(getRange.EndValue),
		Unit:       types.StringValue(dashboardwidgets.DashboardProtoToSchemaUnit[getRange.GetUnit()]),
		CustomUnit: utils.StringPointerToTypeString(getRange.CustomUnit),
	}

	return types.ObjectValueFrom(ctx, manualRangeStrategyModelAttr(), rangeStrategy)
}

func flattenDashboardAnnotationSpansSourceModel(ctx context.Context, spans *dashboardservice.SpansSource) (types.Object, diag.Diagnostics) {
	if spans == nil {
		return types.ObjectNull(annotationsLogsAndSpansSourceModelAttr()), nil
	}

	strategy, diags := flattenAnnotationSpansStrategy(ctx, spans.Strategy)
	if diags.HasError() {
		return types.ObjectNull(annotationsLogsAndSpansSourceModelAttr()), diags
	}

	labelFields, diags := dashboardwidgets.FlattenObservationFields(ctx, spans.GetLabelFields())
	if diags.HasError() {
		return types.ObjectNull(annotationsLogsAndSpansSourceModelAttr()), diags
	}

	spansObject := &DashboardAnnotationSpansOrLogsSourceModel{
		LuceneQuery:     utils.StringPointerToTypeString(spans.GetLuceneQuery().Value),
		Strategy:        strategy,
		MessageTemplate: utils.StringPointerToTypeString(spans.MessageTemplate),
		LabelFields:     labelFields,
	}

	return types.ObjectValueFrom(ctx, annotationsLogsAndSpansSourceModelAttr(), spansObject)
}

func flattenAnnotationSpansStrategy(ctx context.Context, strategy *dashboardservice.SpansSourceStrategy) (types.Object, diag.Diagnostics) {
	if strategy == nil {
		return types.ObjectNull(logsAndSpansStrategyModelAttr()), nil
	}

	var strategyModel DashboardAnnotationSpanOrLogsStrategyModel
	var diags diag.Diagnostics
	switch {
	case strategy.Instant != nil:
		strategyModel.Instant, diags = flattenSpansStrategyInstant(ctx, strategy.Instant)
		strategyModel.Range = types.ObjectNull(rangeStrategyModelAttr())
		strategyModel.Duration = types.ObjectNull(durationStrategyModelAttr())
	case strategy.Range != nil:
		strategyModel.Range, diags = flattenSpansStrategyRange(ctx, strategy.Range)
		strategyModel.Instant = types.ObjectNull(instantStrategyModelAttr())
		strategyModel.Duration = types.ObjectNull(durationStrategyModelAttr())
	case strategy.Duration != nil:
		strategyModel.Duration, diags = flattenSpansStrategyDuration(ctx, strategy.Duration)
		strategyModel.Instant = types.ObjectNull(instantStrategyModelAttr())
		strategyModel.Range = types.ObjectNull(rangeStrategyModelAttr())
	default:
		diags = diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Annotation Spans Strategy", fmt.Sprintf("unknown annotation spans strategy type %T", strategy))}
	}

	if diags.HasError() {
		return types.ObjectNull(logsAndSpansStrategyModelAttr()), diags
	}

	return types.ObjectValueFrom(ctx, logsAndSpansStrategyModelAttr(), strategyModel)
}

func flattenSpansStrategyDuration(ctx context.Context, duration *dashboardservice.SpansSourceStrategyDuration) (types.Object, diag.Diagnostics) {
	if duration == nil {
		return types.ObjectNull(durationStrategyModelAttr()), nil
	}

	startTimestampField, diags := dashboardwidgets.FlattenObservationField(ctx, duration.StartTimestampField)
	if diags.HasError() {
		return types.ObjectNull(durationStrategyModelAttr()), diags
	}

	endTimestampField, diags := dashboardwidgets.FlattenObservationField(ctx, duration.DurationField)
	if diags.HasError() {
		return types.ObjectNull(durationStrategyModelAttr()), diags
	}

	durationStrategy := &DashboardAnnotationDurationStrategyModel{
		StartTimestampField: startTimestampField,
		DurationField:       endTimestampField,
	}

	return types.ObjectValueFrom(ctx, durationStrategyModelAttr(), durationStrategy)
}

func flattenSpansStrategyRange(ctx context.Context, getRange *dashboardservice.SpansSourceStrategyRange) (types.Object, diag.Diagnostics) {
	if getRange == nil {
		return types.ObjectNull(rangeStrategyModelAttr()), nil
	}

	startTimestampField, diags := dashboardwidgets.FlattenObservationField(ctx, getRange.StartTimestampField)
	if diags.HasError() {
		return types.ObjectNull(rangeStrategyModelAttr()), diags
	}

	endTimestampField, diags := dashboardwidgets.FlattenObservationField(ctx, getRange.EndTimestampField)
	if diags.HasError() {
		return types.ObjectNull(rangeStrategyModelAttr()), diags
	}

	rangeStrategy := &DashboardAnnotationRangeStrategyModel{
		StartTimestampField: startTimestampField,
		EndTimestampField:   endTimestampField,
	}

	return types.ObjectValueFrom(ctx, rangeStrategyModelAttr(), rangeStrategy)
}

func flattenSpansStrategyInstant(ctx context.Context, instant *dashboardservice.SpansSourceStrategyInstant) (types.Object, diag.Diagnostics) {
	if instant == nil {
		return types.ObjectNull(instantStrategyModelAttr()), nil
	}

	timestampField, diags := dashboardwidgets.FlattenObservationField(ctx, instant.TimestampField)
	if diags.HasError() {
		return types.ObjectNull(instantStrategyModelAttr()), diags
	}

	instantStrategy := &DashboardAnnotationInstantStrategyModel{
		TimestampField: timestampField,
	}

	return types.ObjectValueFrom(ctx, instantStrategyModelAttr(), instantStrategy)
}

func flattenLogsStrategyDuration(ctx context.Context, duration *dashboardservice.LogsSourceStrategyDuration) (types.Object, diag.Diagnostics) {
	if duration == nil {
		return types.ObjectNull(durationStrategyModelAttr()), nil
	}

	startTimestampField, diags := dashboardwidgets.FlattenObservationField(ctx, duration.StartTimestampField)
	if diags.HasError() {
		return types.ObjectNull(durationStrategyModelAttr()), diags
	}

	endTimestampField, diags := dashboardwidgets.FlattenObservationField(ctx, duration.DurationField)
	if diags.HasError() {
		return types.ObjectNull(durationStrategyModelAttr()), diags
	}

	durationStrategy := &DashboardAnnotationDurationStrategyModel{
		StartTimestampField: startTimestampField,
		DurationField:       endTimestampField,
	}

	return types.ObjectValueFrom(ctx, durationStrategyModelAttr(), durationStrategy)
}

func flattenLogsStrategyRange(ctx context.Context, getRange *dashboardservice.LogsSourceStrategyRange) (types.Object, diag.Diagnostics) {
	if getRange == nil {
		return types.ObjectNull(rangeStrategyModelAttr()), nil
	}

	startTimestampField, diags := dashboardwidgets.FlattenObservationField(ctx, getRange.StartTimestampField)
	if diags.HasError() {
		return types.ObjectNull(rangeStrategyModelAttr()), diags
	}

	endTimestampField, diags := dashboardwidgets.FlattenObservationField(ctx, getRange.EndTimestampField)
	if diags.HasError() {
		return types.ObjectNull(rangeStrategyModelAttr()), diags
	}

	rangeStrategy := &DashboardAnnotationRangeStrategyModel{
		StartTimestampField: startTimestampField,
		EndTimestampField:   endTimestampField,
	}

	return types.ObjectValueFrom(ctx, rangeStrategyModelAttr(), rangeStrategy)
}

func flattenLogsStrategyInstant(ctx context.Context, instant *dashboardservice.LogsSourceStrategyInstant) (types.Object, diag.Diagnostics) {
	if instant == nil {
		return types.ObjectNull(instantStrategyModelAttr()), nil
	}

	timestampField, diags := dashboardwidgets.FlattenObservationField(ctx, instant.TimestampField)
	if diags.HasError() {
		return types.ObjectNull(instantStrategyModelAttr()), diags
	}

	instantStrategy := &DashboardAnnotationInstantStrategyModel{
		TimestampField: timestampField,
	}

	return types.ObjectValueFrom(ctx, instantStrategyModelAttr(), instantStrategy)
}

func flattenDashboardAnnotationLogsSourceModel(ctx context.Context, logs *dashboardservice.LogsSource) (types.Object, diag.Diagnostics) {
	if logs == nil {
		return types.ObjectNull(annotationsLogsAndSpansSourceModelAttr()), nil
	}

	strategy, diags := flattenAnnotationLogsStrategy(ctx, logs.Strategy)
	if diags.HasError() {
		return types.ObjectNull(annotationsLogsAndSpansSourceModelAttr()), diags
	}

	labelFields, diags := dashboardwidgets.FlattenObservationFields(ctx, logs.GetLabelFields())
	if diags.HasError() {
		return types.ObjectNull(annotationsLogsAndSpansSourceModelAttr()), diags
	}

	logsObject := &DashboardAnnotationSpansOrLogsSourceModel{
		LuceneQuery:     utils.StringPointerToTypeString(logs.GetLuceneQuery().Value),
		Strategy:        strategy,
		MessageTemplate: utils.StringPointerToTypeString(logs.MessageTemplate),
		LabelFields:     labelFields,
	}

	return types.ObjectValueFrom(ctx, annotationsLogsAndSpansSourceModelAttr(), logsObject)
}

func flattenAnnotationLogsStrategy(ctx context.Context, strategy *dashboardservice.LogsSourceStrategy) (types.Object, diag.Diagnostics) {
	if strategy == nil {
		return types.ObjectNull(logsAndSpansStrategyModelAttr()), nil
	}

	var strategyModel DashboardAnnotationSpanOrLogsStrategyModel
	var diags diag.Diagnostics
	switch {
	case strategy.Instant != nil:
		strategyModel.Instant, diags = flattenLogsStrategyInstant(ctx, strategy.Instant)
		strategyModel.Range = types.ObjectNull(rangeStrategyModelAttr())
		strategyModel.Duration = types.ObjectNull(durationStrategyModelAttr())
	case strategy.Range != nil:
		strategyModel.Range, diags = flattenLogsStrategyRange(ctx, strategy.Range)
		strategyModel.Instant = types.ObjectNull(instantStrategyModelAttr())
		strategyModel.Duration = types.ObjectNull(durationStrategyModelAttr())
	case strategy.Duration != nil:
		strategyModel.Duration, diags = flattenLogsStrategyDuration(ctx, strategy.Duration)
		strategyModel.Instant = types.ObjectNull(instantStrategyModelAttr())
		strategyModel.Range = types.ObjectNull(rangeStrategyModelAttr())
	default:
		diags = diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Annotation Logs Strategy", fmt.Sprintf("unknown annotation logs strategy type %T", strategy))}
	}

	if diags.HasError() {
		return types.ObjectNull(logsAndSpansStrategyModelAttr()), diags
	}

	return types.ObjectValueFrom(ctx, logsAndSpansStrategyModelAttr(), strategyModel)
}

func flattenDashboardAnnotationMetricSourceModel(ctx context.Context, metricSource *dashboardservice.MetricsSource) (types.Object, diag.Diagnostics) {
	if metricSource == nil {
		return types.ObjectNull(annotationsMetricsSourceModelAttr()), nil
	}

	strategy, diags := flattenDashboardAnnotationStrategy(ctx, metricSource.Strategy)
	if diags.HasError() {
		return types.ObjectNull(annotationsMetricsSourceModelAttr()), diags
	}

	metricSourceObject := &DashboardAnnotationMetricSourceModel{
		PromqlQuery:     utils.StringPointerToTypeString(metricSource.GetPromqlQuery().Value),
		Strategy:        strategy,
		MessageTemplate: utils.StringPointerToTypeString(metricSource.MessageTemplate),
		Labels:          utils.StringSliceToTypeStringList(metricSource.GetLabels()),
	}

	return types.ObjectValueFrom(ctx, annotationsMetricsSourceModelAttr(), metricSourceObject)
}

func flattenDashboardAnnotationStrategy(ctx context.Context, strategy *dashboardservice.MetricsSourceStrategy) (types.Object, diag.Diagnostics) {
	if strategy == nil {
		return types.ObjectNull(metricStrategyModelAttr()), nil
	}
	startTime, diags := types.ObjectValueFrom(ctx, map[string]attr.Type{}, &MetricStrategyStartTimeModel{})
	if diags.HasError() {
		return types.ObjectNull(metricStrategyModelAttr()), diags
	}
	strategyObject := &DashboardAnnotationMetricStrategyModel{
		StartTime: startTime,
	}

	return types.ObjectValueFrom(ctx, metricStrategyModelAttr(), strategyObject)
}

func flattenDashboardAutoRefresh(ctx context.Context, dashboard *dashboardservice.Dashboard) (types.Object, diag.Diagnostics) {
	if dashboard == nil {
		return types.ObjectNull(dashboardAutoRefreshModelAttr()), nil
	}

	var refreshType DashboardAutoRefreshModel
	switch {
	case dashboard.Off != nil:
		refreshType.Type = types.StringValue("off")
	case dashboard.FiveMinutes != nil:
		refreshType.Type = types.StringValue("five_minutes")
	case dashboard.TwoMinutes != nil:
		refreshType.Type = types.StringValue("two_minutes")
	default:
		return types.ObjectNull(dashboardAutoRefreshModelAttr()), nil
	}
	return types.ObjectValueFrom(ctx, dashboardAutoRefreshModelAttr(), &refreshType)
}

func (r *DashboardResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state DashboardResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	//Get refreshed Dashboard value from Coralogix
	id := state.ID.ValueString()
	log.Printf("[INFO] Reading Dashboard: %s", id)
	getDashboardResp, err := r.openAPIClient.Get(ctx, id)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if errors.Is(err, errDashboardOpenAPINotFound) {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("Dashboard %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError(
				"Error reading Dashboard",
				err.Error(),
			)
		}
		return
	}
	log.Printf("[INFO] Received Dashboard: %s", dashboardLogString(getDashboardResp.Dashboard))

	flattenedDashboard, diags := flattenDashboard(ctx, state, getDashboardResp)
	if diags != nil {
		resp.Diagnostics.Append(diags...)
		return
	}
	state = *flattenedDashboard

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *DashboardResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var plan DashboardResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	var configAccessPolicy types.String
	diags = req.Config.GetAttribute(ctx, path.Root("access_policy"), &configAccessPolicy)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	dashboard, diags := extractDashboard(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	if plan.ID.IsNull() || plan.ID.IsUnknown() || plan.ID.ValueString() == "" {
		resp.Diagnostics.AddError("Error updating Dashboard", "Dashboard ID is unavailable in the Terraform plan")
		return
	}
	dashboard.SetId(plan.ID.ValueString())

	accessPolicy := dashboardAccessPolicyForConfiguredRequest(configAccessPolicy, plan.AccessPolicy)
	log.Printf("[INFO] Updating Dashboard: %s", dashboardLogString(dashboard))
	err := r.replaceDashboard(ctx, dashboard, accessPolicy)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error updating Dashboard",
			err.Error(),
		)
		return
	}

	getDashboardResp, err := r.openAPIClient.Get(ctx, plan.ID.ValueString())
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error getting Dashboard",
			err.Error(),
		)
		return
	}
	log.Printf("[INFO] Submitted updated Dashboard: %s", dashboardLogString(getDashboardResp.Dashboard))

	flattenedDashboard, diags := flattenDashboard(ctx, plan, getDashboardResp)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	plan = *flattenedDashboard

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func (r *DashboardResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state DashboardResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	log.Printf("[INFO] Deleting Dashboard %s", id)
	if err := r.openAPIClient.Delete(ctx, id); err != nil {
		resp.Diagnostics.AddError(
			fmt.Sprintf("Error Deleting Dashboard %s", id),
			err.Error(),
		)
		return
	}
	log.Printf("[INFO] Dashboard %s deleted", id)
}

func (r *DashboardResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	clientSet, ok := req.ProviderData.(*clientset.ClientSet)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *clientset.ClientSet, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.openAPIClient = newDashboardOpenAPIClient(clientSet.DashboardsOpenAPI())
}
