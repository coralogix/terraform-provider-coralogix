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
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	dashboardschema "github.com/coralogix/terraform-provider-coralogix/internal/provider/dashboards/dashboard_schema"
	dashboardwidgets "github.com/coralogix/terraform-provider-coralogix/internal/provider/dashboards/dashboard_widgets"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

var (
	_ resource.ResourceWithConfigure = &DashboardResource{}
	//_ resource.ResourceWithConfigValidators = &DashboardResource{}
	_ resource.ResourceWithImportState  = &DashboardResource{}
	_ resource.ResourceWithUpgradeState = &DashboardResource{}
)

type DashboardResourceModel struct {
	ID          types.String                     `tfsdk:"id"`
	Name        types.String                     `tfsdk:"name"`
	Description types.String                     `tfsdk:"description"`
	Layout      types.Object                     `tfsdk:"layout"`    //DashboardLayoutModel
	Variables   types.List                       `tfsdk:"variables"` //DashboardVariableModel
	Filters     types.List                       `tfsdk:"filters"`   //DashboardFilterModel
	TimeFrame   *dashboardwidgets.TimeFrameModel `tfsdk:"time_frame"`
	Folder      types.Object                     `tfsdk:"folder"`       //DashboardFolderModel
	Annotations types.List                       `tfsdk:"annotations"`  //DashboardAnnotationModel
	AutoRefresh types.Object                     `tfsdk:"auto_refresh"` //DashboardAutoRefreshModel
	ContentJson types.String                     `tfsdk:"content_json"`
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
	StartTimestampField types.Object `tfsdk:"start_time_timestamp_field"` //dashboard_widgets.ObservationFieldModel
	EndTimestampField   types.Object `tfsdk:"end_time_timestamp_field"`   //dashboard_widgets.ObservationFieldModel
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

func NewDashboardResource() resource.Resource {
	return &DashboardResource{}
}

type DashboardResource struct {
	client *cxsdk.DashboardsClient
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
				upgradeDashboardStateV3ToV4(ctx, req, resp, r.client)
			},
		},
		3: {
			PriorSchema: &schemaV3,
			StateUpgrader: func(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
				upgradeDashboardStateV3ToV4(ctx, req, resp, r.client)
			},
		},
	}
}

func upgradeDashboardStateV3ToV4(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse, client *cxsdk.DashboardsClient) {
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
	getDashboardReq := &cxsdk.GetDashboardRequest{DashboardId: wrapperspb.String(id)}
	getDashboardResp, err := client.Get(ctx, getDashboardReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if cxsdk.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("Dashboard %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError(
				"Error reading Dashboard",
				utils.FormatRpcErrors(err, cxsdk.GetDashboardRPC, protojson.Format(getDashboardReq)),
			)
		}
		return
	}
	log.Printf("[INFO] Received Dashboard: %s", protojson.Format(getDashboardResp))

	flattenedDashboard, diags := flattenDashboard(ctx, state, getDashboardResp.GetDashboard())
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
						var spans *dashboardwidgets.QuerySpansModel
						if oldQuery.DataPrime != nil {
							dataprime = &dashboardwidgets.DataPrimeModel{
								TimeFrame: timeFrame,
								Query:     oldQuery.DataPrime.Query,
								Filters:   oldQuery.DataPrime.Filters,
							}
						}
						if oldQuery.Spans != nil {
							spans = &dashboardwidgets.QuerySpansModel{
								TimeFrame:    timeFrame,
								LuceneQuery:  oldQuery.Spans.LuceneQuery,
								Filters:      oldQuery.Spans.Filters,
								Aggregations: oldQuery.Spans.Aggregations,
								GroupBy:      oldQuery.Spans.GroupBy,
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

	createDashboardReq := &cxsdk.CreateDashboardRequest{
		Dashboard: dashboard,
	}
	dashboardStr := protojson.Format(createDashboardReq)
	log.Printf("[INFO] Creating new Dashboard: %s", dashboardStr)
	createResponse, err := r.client.Create(ctx, createDashboardReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error creating Dashboard",
			utils.FormatRpcErrors(err, cxsdk.CreateDashboardRPC, dashboardStr),
		)
		return
	}

	getDashboardReq := &cxsdk.GetDashboardRequest{
		DashboardId: createResponse.DashboardId,
	}
	getDashboardResp, err := r.client.Get(ctx, getDashboardReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		reqStr := protojson.Format(getDashboardReq)
		resp.Diagnostics.AddError(
			"Error getting Dashboard",
			utils.FormatRpcErrors(err, cxsdk.GetDashboardRPC, reqStr),
		)
		return
	}
	createDashboardRespStr := protojson.Format(getDashboardResp.GetDashboard())
	log.Printf("[INFO] Submitted new Dashboard: %s", createDashboardRespStr)

	flattenedDashboard, diags := flattenDashboard(ctx, plan, getDashboardResp.GetDashboard())
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	log.Printf("[INFO] Flattened Dashboard: %v", flattenedDashboard)
	plan = *flattenedDashboard

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func extractDashboard(ctx context.Context, plan DashboardResourceModel) (*cxsdk.Dashboard, diag.Diagnostics) {
	if !plan.ContentJson.IsNull() {
		dashboard := new(cxsdk.Dashboard)
		if err := dashboardschema.JSONUnmarshal.Unmarshal([]byte(plan.ContentJson.ValueString()), dashboard); err != nil {
			return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error unmarshalling dashboard content json", err.Error())}
		}

		dashboard, diags := expandDashboardFolder(ctx, dashboard, plan.Folder)
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

	var id *wrapperspb.StringValue // the service auto-generates IDs if they are null
	if !(plan.ID.IsNull() || plan.ID.IsUnknown()) {
		id = wrapperspb.String(plan.ID.ValueString())
	}

	dashboard := &cxsdk.Dashboard{
		Id:          id,
		Name:        utils.TypeStringToWrapperspbString(plan.Name),
		Description: utils.TypeStringToWrapperspbString(plan.Description),
		Layout:      layout,
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

func expandDashboardAutoRefresh(ctx context.Context, dashboard *cxsdk.Dashboard, refresh types.Object) (*cxsdk.Dashboard, diag.Diagnostics) {
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
		dashboard.AutoRefresh = &cxsdk.DashboardTwoMinutes{
			TwoMinutes: &cxsdk.DashboardAutoRefreshTwoMinutes{},
		}
	case "five_minutes":
		dashboard.AutoRefresh = &cxsdk.DashboardFiveMinutes{
			FiveMinutes: &cxsdk.DashboardAutoRefreshFiveMinutes{},
		}
	default:
		dashboard.AutoRefresh = &cxsdk.DashboardOff{
			Off: &cxsdk.DashboardAutoRefreshOff{},
		}
	}

	return dashboard, nil
}

func expandDashboardAnnotations(ctx context.Context, annotations types.List) ([]*cxsdk.Annotation, diag.Diagnostics) {
	var annotationsObjects []types.Object
	var expandedAnnotations []*cxsdk.Annotation
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
		expandedAnnotations = append(expandedAnnotations, expandedAnnotation)
	}

	return expandedAnnotations, diags
}

func expandAnnotation(ctx context.Context, annotation DashboardAnnotationModel) (*cxsdk.Annotation, diag.Diagnostics) {
	source, diags := expandAnnotationSource(ctx, annotation.Source)
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.Annotation{
		Id:      dashboardwidgets.ExpandDashboardIDs(annotation.ID),
		Name:    utils.TypeStringToWrapperspbString(annotation.Name),
		Enabled: utils.TypeBoolToWrapperspbBool(annotation.Enabled),
		Source:  source,
	}, nil

}

func expandAnnotationSource(ctx context.Context, source types.Object) (*cxsdk.AnnotationSource, diag.Diagnostics) {
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
		return &cxsdk.AnnotationSource{Value: logsSource}, nil
	case !(sourceObject.Metrics.IsNull() || sourceObject.Metrics.IsUnknown()):
		metricSource, diags := expandMetricSource(ctx, sourceObject.Metrics)
		if diags.HasError() {
			return nil, diags
		}
		return &cxsdk.AnnotationSource{Value: metricSource}, nil
	case !(sourceObject.Spans.IsNull() || sourceObject.Spans.IsUnknown()):
		spansSource, diags := expandSpansSource(ctx, sourceObject.Spans)
		if diags.HasError() {
			return nil, diags
		}
		return &cxsdk.AnnotationSource{Value: spansSource}, nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand Annotation Source", "Annotation Source must be either Logs or Metric")}
	}
}

func expandLogsSource(ctx context.Context, logs types.Object) (*cxsdk.AnnotationSourceLogs, diag.Diagnostics) {
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

	return &cxsdk.AnnotationSourceLogs{
		Logs: &cxsdk.AnnotationLogsSource{
			LuceneQuery:     dashboardwidgets.ExpandLuceneQuery(logsObject.LuceneQuery),
			Strategy:        strategy,
			MessageTemplate: utils.TypeStringToWrapperspbString(logsObject.MessageTemplate),
			LabelFields:     labels,
		},
	}, nil
}

func expandLogsSourceStrategy(ctx context.Context, strategy types.Object) (*cxsdk.AnnotationLogsSourceStrategy, diag.Diagnostics) {
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

func expandLogsSourceDurationStrategy(ctx context.Context, duration types.Object) (*cxsdk.AnnotationLogsSourceStrategy, diag.Diagnostics) {
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

	return &cxsdk.AnnotationLogsSourceStrategy{
		Value: &cxsdk.AnnotationLogsSourceStrategyDuration{
			Duration: &cxsdk.AnnotationLogsSourceStrategyDurationInner{
				StartTimestampField: startTimestampField,
				DurationField:       durationField,
			},
		},
	}, nil
}

func expandLogsSourceRangeStrategy(ctx context.Context, object types.Object) (*cxsdk.AnnotationLogsSourceStrategy, diag.Diagnostics) {
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

	return &cxsdk.AnnotationLogsSourceStrategy{
		Value: &cxsdk.AnnotationLogsSourceStrategyRange{
			Range: &cxsdk.AnnotationLogsSourceStrategyRangeInner{
				StartTimestampField: startTimestampField,
				EndTimestampField:   endTimestampField,
			},
		},
	}, nil
}

func expandLogsSourceInstantStrategy(ctx context.Context, instant types.Object) (*cxsdk.AnnotationLogsSourceStrategy, diag.Diagnostics) {
	var instantObject DashboardAnnotationInstantStrategyModel
	if diags := instant.As(ctx, &instantObject, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	timestampField, diags := dashboardwidgets.ExpandObservationFieldObject(ctx, instantObject.TimestampField)
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.AnnotationLogsSourceStrategy{
		Value: &cxsdk.AnnotationLogsSourceStrategyInstant{
			Instant: &cxsdk.AnnotationLogsSourceStrategyInstantInner{
				TimestampField: timestampField,
			},
		},
	}, nil
}

func expandSpansSourceStrategy(ctx context.Context, strategy types.Object) (*cxsdk.AnnotationSpansSourceStrategy, diag.Diagnostics) {
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

func expandSpansSourceDurationStrategy(ctx context.Context, duration types.Object) (*cxsdk.AnnotationSpansSourceStrategy, diag.Diagnostics) {
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

	return &cxsdk.AnnotationSpansSourceStrategy{
		Value: &cxsdk.AnnotationSpansSourceStrategyDuration{
			Duration: &cxsdk.AnnotationSpansSourceStrategyDurationInner{
				StartTimestampField: startTimestampField,
				DurationField:       durationField,
			},
		},
	}, nil
}

func expandSpansSourceRangeStrategy(ctx context.Context, object types.Object) (*cxsdk.AnnotationSpansSourceStrategy, diag.Diagnostics) {
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

	return &cxsdk.AnnotationSpansSourceStrategy{
		Value: &cxsdk.AnnotationSpansSourceStrategyRange{
			Range: &cxsdk.AnnotationSpansSourceStrategyRangeInner{
				StartTimestampField: startTimestampField,
				EndTimestampField:   endTimestampField,
			},
		},
	}, nil
}

func expandSpansSourceInstantStrategy(ctx context.Context, instant types.Object) (*cxsdk.AnnotationSpansSourceStrategy, diag.Diagnostics) {
	var instantObject DashboardAnnotationInstantStrategyModel
	if diags := instant.As(ctx, &instantObject, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	timestampField, diags := dashboardwidgets.ExpandObservationFieldObject(ctx, instantObject.TimestampField)
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.AnnotationSpansSourceStrategy{
		Value: &cxsdk.AnnotationSpansSourceStrategyInstant{
			Instant: &cxsdk.AnnotationSpansSourceStrategyInstantInner{
				TimestampField: timestampField,
			},
		},
	}, nil
}

func expandSpansSource(ctx context.Context, spans types.Object) (*cxsdk.AnnotationSourceSpans, diag.Diagnostics) {
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

	return &cxsdk.AnnotationSourceSpans{
		Spans: &cxsdk.AnnotationSpansSource{
			LuceneQuery:     dashboardwidgets.ExpandLuceneQuery(spansObject.LuceneQuery),
			Strategy:        strategy,
			MessageTemplate: utils.TypeStringToWrapperspbString(spansObject.MessageTemplate),
			LabelFields:     labels,
		},
	}, nil
}

func expandMetricSource(ctx context.Context, metric types.Object) (*cxsdk.AnnotationSourceMetrics, diag.Diagnostics) {
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

	labels, diags := utils.TypeStringSliceToWrappedStringSlice(ctx, metricObject.Labels.Elements())
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.AnnotationSourceMetrics{
		Metrics: &cxsdk.AnnotationMetricsSource{
			PromqlQuery:     dashboardwidgets.ExpandPromqlQuery(metricObject.PromqlQuery),
			Strategy:        strategy,
			MessageTemplate: utils.TypeStringToWrapperspbString(metricObject.MessageTemplate),
			Labels:          labels,
		},
	}, nil
}

func expandMetricSourceStrategy(ctx context.Context, strategy types.Object) (*cxsdk.AnnotationMetricsSourceStrategy, diag.Diagnostics) {
	var strategyObject DashboardAnnotationMetricStrategyModel
	diags := strategy.As(ctx, &strategyObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.AnnotationMetricsSourceStrategy{
		Value: &cxsdk.AnnotationMetricsSourceStrategyStartTimeMetric{
			StartTimeMetric: &cxsdk.AnnotationMetricsSourceStartTimeMetric{},
		},
	}, nil
}

func expandDashboardLayout(ctx context.Context, layout types.Object) (*cxsdk.DashboardLayout, diag.Diagnostics) {
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
	return &cxsdk.DashboardLayout{
		Sections: sections,
	}, nil
}

func expandDashboardSections(ctx context.Context, sections types.List) ([]*cxsdk.DashboardSection, diag.Diagnostics) {
	var sectionsObjects []types.Object
	var expandedSections []*cxsdk.DashboardSection
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
		expandedSections = append(expandedSections, expandedSection)
	}

	return expandedSections, diags
}

func expandSection(ctx context.Context, section SectionModel) (*cxsdk.DashboardSection, diag.Diagnostics) {
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
		return &cxsdk.DashboardSection{
			Id:      id,
			Rows:    rows,
			Options: options,
		}, nil
	} else {
		return &cxsdk.DashboardSection{
			Id:      id,
			Rows:    rows,
			Options: nil,
		}, nil
	}
}

func expandSectionOptions(_ context.Context, option SectionOptionsModel) (*cxsdk.DashboardSectionOptions, diag.Diagnostics) {

	var color *cxsdk.DashboardSectionColor
	if !option.Color.IsNull() {
		mappedColor := cxsdk.DashboardSectionPredefinedColorValueLookup[fmt.Sprintf("SECTION_PREDEFINED_COLOR_%s", strings.ToUpper(option.Color.ValueString()))]
		// this means the color field somehow wasn't validated
		if mappedColor == 0 && option.Color.String() != utils.UNSPECIFIED {
			return nil, diag.Diagnostics{
				diag.NewErrorDiagnostic(
					"Extract Dashboard Section Options Error",
					fmt.Sprintf("Unknown color: %s", option.Color.ValueString()),
				),
			}
		}
		color = &cxsdk.DashboardSectionColor{
			Value: &cxsdk.DashboardSectionColorPredefined{
				Predefined: cxsdk.DashboardSectionColorPredefinedColor(mappedColor),
			},
		}
	}

	var description *wrapperspb.StringValue
	if !option.Description.IsNull() {
		description = wrapperspb.String(option.Description.ValueString())
	}

	var collapsed *wrapperspb.BoolValue
	if !option.Collapsed.IsNull() {
		collapsed = wrapperspb.Bool(option.Collapsed.ValueBool())
	}

	return &cxsdk.DashboardSectionOptions{
		Value: &cxsdk.DashboardSectionOptionsCustom{
			Custom: &cxsdk.CustomSectionOptions{
				Name:        wrapperspb.String(option.Name.ValueString()),
				Description: description,
				Collapsed:   collapsed,
				Color:       color,
			},
		},
	}, nil
}

func expandDashboardRows(ctx context.Context, rows types.List) ([]*cxsdk.DashboardRow, diag.Diagnostics) {
	var rowsObjects []types.Object
	var expandedRows []*cxsdk.DashboardRow
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
		expandedRows = append(expandedRows, expandedRow)
	}

	return expandedRows, diags
}

func expandRow(ctx context.Context, row RowModel) (*cxsdk.DashboardRow, diag.Diagnostics) {
	id := dashboardwidgets.ExpandDashboardUUID(row.ID)
	appearance := &cxsdk.DashboardRowAppearance{
		Height: wrapperspb.Int32(int32(row.Height.ValueInt64())),
	}
	widgets, diags := expandDashboardWidgets(ctx, row.Widgets)
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.DashboardRow{
		Id:         id,
		Appearance: appearance,
		Widgets:    widgets,
	}, nil
}

func expandDashboardWidgets(ctx context.Context, widgets types.List) ([]*cxsdk.DashboardWidget, diag.Diagnostics) {
	var widgetsObjects []types.Object
	var expandedWidgets []*cxsdk.DashboardWidget
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
		expandedWidgets = append(expandedWidgets, expandedWidget)
	}

	return expandedWidgets, diags
}

func expandWidget(ctx context.Context, widget WidgetModel) (*cxsdk.DashboardWidget, diag.Diagnostics) {
	id := dashboardwidgets.ExpandDashboardUUID(widget.ID)

	title := utils.TypeStringToWrapperspbString(widget.Title)
	description := utils.TypeStringToWrapperspbString(widget.Description)
	appearance := &cxsdk.DashboardWidgetAppearance{
		Width: wrapperspb.Int32(int32(widget.Width.ValueInt64())),
	}
	definition, diags := expandWidgetDefinition(ctx, widget.Definition)
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.DashboardWidget{
		Id:          id,
		Title:       title,
		Description: description,
		Appearance:  appearance,
		Definition:  definition,
	}, nil
}

func expandWidgetDefinition(ctx context.Context, definition *dashboardwidgets.WidgetDefinitionModel) (*cxsdk.WidgetDefinition, diag.Diagnostics) {
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

func expandMarkdown(markdown *dashboardwidgets.MarkdownModel) (*cxsdk.WidgetDefinition, diag.Diagnostics) {
	return &cxsdk.WidgetDefinition{
		Value: &cxsdk.WidgetDefinitionMarkdown{
			Markdown: &cxsdk.Markdown{
				MarkdownText: utils.TypeStringToWrapperspbString(markdown.MarkdownText),
				TooltipText:  utils.TypeStringToWrapperspbString(markdown.TooltipText),
			},
		},
	}, nil
}

func expandHorizontalBarChart(ctx context.Context, chart *dashboardwidgets.HorizontalBarChartModel) (*cxsdk.WidgetDefinition, diag.Diagnostics) {
	var diags diag.Diagnostics

	query, diags := expandHorizontalBarChartQuery(ctx, chart.Query)
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.WidgetDefinition{
		Value: &cxsdk.WidgetDefinitionHorizontalBarChart{
			HorizontalBarChart: &cxsdk.HorizontalBarChart{
				Query:             query,
				StackDefinition:   expandHorizontalBarChartStackDefinition(chart.StackDefinition),
				MaxBarsPerChart:   utils.TypeInt64ToWrappedInt32(chart.MaxBarsPerChart),
				ScaleType:         dashboardwidgets.DashboardSchemaToProtoScaleType[chart.ScaleType.ValueString()],
				GroupNameTemplate: utils.TypeStringToWrapperspbString(chart.GroupNameTemplate),
				Unit:              dashboardwidgets.DashboardSchemaToProtoUnit[chart.Unit.ValueString()],
				ColorsBy:          expandColorsBy(chart.ColorsBy),
				DisplayOnBar:      utils.TypeBoolToWrapperspbBool(chart.DisplayOnBar),
				YAxisViewBy:       expandYAxisViewBy(chart.YAxisViewBy),
				SortBy:            dashboardwidgets.DashboardSchemaToProtoSortBy[chart.SortBy.ValueString()],
				ColorScheme:       utils.TypeStringToWrapperspbString(chart.ColorScheme),
				DataModeType:      dashboardwidgets.DashboardSchemaToProtoDataModeType[chart.DataModeType.ValueString()],
			},
		},
	}, nil
}

func expandYAxisViewBy(yAxisViewBy types.String) *cxsdk.HorizontalBarChartYAxisViewBy {
	switch yAxisViewBy.ValueString() {
	case "category":
		return &cxsdk.HorizontalBarChartYAxisViewBy{
			YAxisView: &cxsdk.HorizontalBarChartYAxisViewByCategory{},
		}
	case "value":
		return &cxsdk.HorizontalBarChartYAxisViewBy{
			YAxisView: &cxsdk.HorizontalBarChartYAxisViewByValue{},
		}
	default:
		return nil
	}
}

func expandPieChart(ctx context.Context, pieChart *dashboardwidgets.PieChartModel) (*cxsdk.WidgetDefinition, diag.Diagnostics) {
	var diags diag.Diagnostics

	query, diags := expandPieChartQuery(ctx, pieChart.Query)
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.WidgetDefinition{
		Value: &cxsdk.WidgetDefinitionPieChart{
			PieChart: &cxsdk.PieChart{
				Query:              query,
				MaxSlicesPerChart:  utils.TypeInt64ToWrappedInt32(pieChart.MaxSlicesPerChart),
				MinSlicePercentage: utils.TypeInt64ToWrappedInt32(pieChart.MinSlicePercentage),
				StackDefinition:    expandPieChartStackDefinition(pieChart.StackDefinition),
				LabelDefinition:    expandLabelDefinition(pieChart.LabelDefinition),
				ShowLegend:         utils.TypeBoolToWrapperspbBool(pieChart.ShowLegend),
				GroupNameTemplate:  utils.TypeStringToWrapperspbString(pieChart.GroupNameTemplate),
				Unit:               dashboardwidgets.DashboardSchemaToProtoUnit[pieChart.Unit.ValueString()],
				ColorScheme:        utils.TypeStringToWrapperspbString(pieChart.ColorScheme),
				DataModeType:       dashboardwidgets.DashboardSchemaToProtoDataModeType[pieChart.DataModeType.ValueString()],
			},
		},
	}, nil
}

func expandPieChartStackDefinition(stackDefinition *dashboardwidgets.PieChartStackDefinitionModel) *cxsdk.PieChartStackDefinition {
	if stackDefinition == nil {
		return nil
	}

	return &cxsdk.PieChartStackDefinition{
		MaxSlicesPerStack: utils.TypeInt64ToWrappedInt32(stackDefinition.MaxSlicesPerStack),
		StackNameTemplate: utils.TypeStringToWrapperspbString(stackDefinition.StackNameTemplate),
	}
}

func expandBarChartStackDefinition(stackDefinition *dashboardwidgets.BarChartStackDefinitionModel) *cxsdk.BarChartStackDefinition {
	if stackDefinition == nil {
		return nil
	}

	return &cxsdk.BarChartStackDefinition{
		MaxSlicesPerBar:   utils.TypeInt64ToWrappedInt32(stackDefinition.MaxSlicesPerBar),
		StackNameTemplate: utils.TypeStringToWrapperspbString(stackDefinition.StackNameTemplate),
	}
}

func expandHorizontalBarChartStackDefinition(stackDefinition *dashboardwidgets.BarChartStackDefinitionModel) *cxsdk.HorizontalBarChartStackDefinition {
	if stackDefinition == nil {
		return nil
	}

	return &cxsdk.HorizontalBarChartStackDefinition{
		MaxSlicesPerBar:   utils.TypeInt64ToWrappedInt32(stackDefinition.MaxSlicesPerBar),
		StackNameTemplate: utils.TypeStringToWrapperspbString(stackDefinition.StackNameTemplate),
	}
}

func expandLabelDefinition(labelDefinition *dashboardwidgets.LabelDefinitionModel) *cxsdk.PieChartLabelDefinition {
	if labelDefinition == nil {
		return nil
	}

	return &cxsdk.PieChartLabelDefinition{
		LabelSource:    dashboardwidgets.DashboardSchemaToProtoPieChartLabelSource[labelDefinition.LabelSource.ValueString()],
		IsVisible:      utils.TypeBoolToWrapperspbBool(labelDefinition.IsVisible),
		ShowName:       utils.TypeBoolToWrapperspbBool(labelDefinition.ShowName),
		ShowValue:      utils.TypeBoolToWrapperspbBool(labelDefinition.ShowValue),
		ShowPercentage: utils.TypeBoolToWrapperspbBool(labelDefinition.ShowPercentage),
	}
}

func expandGauge(ctx context.Context, gauge *dashboardwidgets.GaugeModel) (*cxsdk.WidgetDefinition, diag.Diagnostics) {
	var diags diag.Diagnostics

	query, diags := expandGaugeQuery(ctx, gauge.Query)
	if diags.HasError() {
		return nil, diags
	}

	thresholds, diags := expandGaugeThresholds(ctx, gauge.Thresholds)
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.WidgetDefinition{
		Value: &cxsdk.WidgetDefinitionGauge{
			Gauge: &cxsdk.Gauge{
				Query:             query,
				Min:               utils.TypeFloat64ToWrapperspbDouble(gauge.Min),
				Max:               utils.TypeFloat64ToWrapperspbDouble(gauge.Max),
				ShowInnerArc:      utils.TypeBoolToWrapperspbBool(gauge.ShowInnerArc),
				ShowOuterArc:      utils.TypeBoolToWrapperspbBool(gauge.ShowOuterArc),
				Unit:              dashboardwidgets.DashboardSchemaToProtoGaugeUnit[gauge.Unit.ValueString()],
				Thresholds:        thresholds,
				DataModeType:      dashboardwidgets.DashboardSchemaToProtoDataModeType[gauge.DataModeType.ValueString()],
				ThresholdBy:       dashboardwidgets.DashboardSchemaToProtoGaugeThresholdBy[gauge.ThresholdBy.ValueString()],
				DisplaySeriesName: utils.TypeBoolToWrapperspbBool(gauge.DisplaySeriesName),
				Decimal:           utils.NumberTypeToWrapperspbInt32(gauge.Decimal),
			},
		},
	}, nil
}

func expandGaugeThresholds(ctx context.Context, gaugeThresholds types.List) ([]*cxsdk.GaugeThreshold, diag.Diagnostics) {
	var gaugeThresholdsObjects []types.Object
	var expandedGaugeThresholds []*cxsdk.GaugeThreshold
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
		expandedGaugeThresholds = append(expandedGaugeThresholds, expandedGaugeThreshold)
	}

	return expandedGaugeThresholds, diags
}

func expandGaugeThreshold(gaugeThresholds *dashboardwidgets.GaugeThresholdModel) *cxsdk.GaugeThreshold {
	if gaugeThresholds == nil {
		return nil
	}
	return &cxsdk.GaugeThreshold{
		From:  utils.TypeFloat64ToWrapperspbDouble(gaugeThresholds.From),
		Color: utils.TypeStringToWrapperspbString(gaugeThresholds.Color),
		Label: utils.TypeStringToWrapperspbString(gaugeThresholds.Label),
	}
}

func expandGaugeQuery(ctx context.Context, gaugeQuery *dashboardwidgets.GaugeQueryModel) (*cxsdk.GaugeQuery, diag.Diagnostics) {
	switch {
	case gaugeQuery.Metrics != nil:
		metricQuery, diags := expandGaugeQueryMetrics(ctx, gaugeQuery.Metrics)
		if diags.HasError() {
			return nil, diags
		}
		return &cxsdk.GaugeQuery{
			Value: &cxsdk.GaugeQueryMetrics{
				Metrics: metricQuery,
			},
		}, nil
	case gaugeQuery.Logs != nil:
		logQuery, diags := expandGaugeQueryLogs(ctx, gaugeQuery.Logs)
		if diags.HasError() {
			return nil, diags
		}
		return &cxsdk.GaugeQuery{
			Value: &cxsdk.GaugeQueryLogs{
				Logs: logQuery,
			},
		}, nil
	case gaugeQuery.Spans != nil:
		spanQuery, diags := expandGaugeQuerySpans(ctx, gaugeQuery.Spans)
		if diags.HasError() {
			return nil, diags
		}
		return &cxsdk.GaugeQuery{
			Value: &cxsdk.GaugeQuerySpans{
				Spans: spanQuery,
			},
		}, nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Extract Gauge Query Error", fmt.Sprintf("Unknown gauge query type %#v", gaugeQuery))}
	}
}

func expandGaugeQuerySpans(ctx context.Context, gaugeQuerySpans *dashboardwidgets.GaugeQuerySpansModel) (*cxsdk.GaugeSpansQuery, diag.Diagnostics) {
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

	return &cxsdk.GaugeSpansQuery{
		LuceneQuery:      dashboardwidgets.ExpandLuceneQuery(gaugeQuerySpans.LuceneQuery),
		SpansAggregation: spansAggregation,
		Filters:          filters,
		TimeFrame:        timeFrame,
	}, nil
}

func expandMultiSelectSourceQuery(ctx context.Context, sourceQuery types.Object) (*cxsdk.MultiSelectSource, diag.Diagnostics) {
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

	return &cxsdk.MultiSelectSource{
		Value: &cxsdk.MultiSelectSourceQuery{
			Query: &cxsdk.MultiSelectQuerySource{
				Query:               query,
				RefreshStrategy:     dashboardwidgets.DashboardSchemaToProtoRefreshStrategy[queryObject.RefreshStrategy.ValueString()],
				ValueDisplayOptions: valueDisplayOptions,
			},
		},
	}, nil
}

func expandMultiSelectQuery(ctx context.Context, query types.Object) (*cxsdk.MultiSelectQuery, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(query) {
		return nil, nil
	}

	var queryObject MultiSelectQueryModel
	diags := query.As(ctx, &queryObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	multiSelectQuery := &cxsdk.MultiSelectQuery{}
	switch {
	case !utils.ObjIsNullOrUnknown(queryObject.Metrics):
		multiSelectQuery.Value, diags = expandMultiSelectMetricsQuery(ctx, queryObject.Metrics)
	case !utils.ObjIsNullOrUnknown(queryObject.Logs):
		multiSelectQuery.Value, diags = expandMultiSelectLogsQuery(ctx, queryObject.Logs)
	case !utils.ObjIsNullOrUnknown(queryObject.Spans):
		multiSelectQuery.Value, diags = expandMultiSelectSpansQuery(ctx, queryObject.Spans)
	default:
		diags = diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand MultiSelect Query", "MultiSelect Query must be either Metrics, Logs or Spans")}
	}

	if diags.HasError() {
		return nil, diags
	}

	return multiSelectQuery, nil
}

func expandMultiSelectValueDisplayOptions(ctx context.Context, options types.Object) (*cxsdk.MultiSelectValueDisplayOptions, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(options) {
		return nil, nil
	}

	var optionsObject MultiSelectValueDisplayOptionsModel
	diags := options.As(ctx, &optionsObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.MultiSelectValueDisplayOptions{
		ValueRegex: utils.TypeStringToWrapperspbString(optionsObject.ValueRegex),
		LabelRegex: utils.TypeStringToWrapperspbString(optionsObject.LabelRegex),
	}, nil
}

func expandMultiSelectLogsQuery(ctx context.Context, logs types.Object) (*cxsdk.MultiSelectQueryLogsQuery, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(logs) {
		return nil, nil
	}

	var logsObject MultiSelectLogsQueryModel
	diags := logs.As(ctx, &logsObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	logsQuery := &cxsdk.MultiSelectQueryLogsQuery{
		LogsQuery: &cxsdk.MultiSelectQueryLogsQueryInner{
			Type: &cxsdk.MultiSelectQueryLogsQueryType{},
		},
	}

	switch {
	case !(logsObject.FieldName.IsNull() || logsObject.FieldName.IsUnknown()):
		logsQuery.LogsQuery.Type.Value, diags = expandMultiSelectLogsQueryTypeFieldName(ctx, logsObject.FieldName)
	case !(logsObject.FieldValue.IsNull() || logsObject.FieldValue.IsUnknown()):
		logsQuery.LogsQuery.Type.Value, diags = expandMultiSelectLogsQueryTypFieldValue(ctx, logsObject.FieldValue)
	}

	if diags.HasError() {
		return nil, diags
	}

	return logsQuery, nil
}

func expandMultiSelectLogsQueryTypeFieldName(ctx context.Context, name types.Object) (*cxsdk.MultiSelectQueryLogsQueryTypeFieldName, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(name) {
		return nil, nil
	}

	var nameObject LogFieldNameModel
	diags := name.As(ctx, &nameObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.MultiSelectQueryLogsQueryTypeFieldName{
		FieldName: &cxsdk.MultiSelectQueryLogsQueryTypeFieldNameInner{
			LogRegex: utils.TypeStringToWrapperspbString(nameObject.LogRegex),
		},
	}, nil
}

func expandMultiSelectLogsQueryTypFieldValue(ctx context.Context, value types.Object) (*cxsdk.MultiSelectQueryLogsQueryTypeFieldValue, diag.Diagnostics) {
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

	return &cxsdk.MultiSelectQueryLogsQueryTypeFieldValue{
		FieldValue: &cxsdk.MultiSelectQueryLogsQueryTypeFieldValueInner{
			ObservationField: observationField,
		},
	}, nil
}

func expandMultiSelectMetricsQuery(ctx context.Context, metrics types.Object) (*cxsdk.MultiSelectQueryMetricsQuery, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(metrics) {
		return nil, nil
	}

	var metricsObject MultiSelectMetricsQueryModel
	diags := metrics.As(ctx, &metricsObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	metricsQuery := &cxsdk.MultiSelectQueryMetricsQuery{
		MetricsQuery: &cxsdk.MultiSelectQueryMetricsQueryInner{
			Type: &cxsdk.MultiSelectQueryMetricsQueryType{},
		},
	}

	switch {
	case !utils.ObjIsNullOrUnknown(metricsObject.MetricName):
		metricsQuery.MetricsQuery.Type.Value, diags = expandMultiSelectMetricsQueryTypeMetricName(ctx, metricsObject.MetricName)
	case !utils.ObjIsNullOrUnknown(metricsObject.LabelName):
		metricsQuery.MetricsQuery.Type.Value, diags = expandMultiSelectMetricsQueryTypeLabelName(ctx, metricsObject.LabelName)
	case !utils.ObjIsNullOrUnknown(metricsObject.LabelValue):
		metricsQuery.MetricsQuery.Type.Value, diags = expandMultiSelectMetricsQueryTypeLabelValue(ctx, metricsObject.LabelValue)
	}

	if diags.HasError() {
		return nil, diags
	}

	return metricsQuery, nil
}

func expandMultiSelectMetricsQueryTypeMetricName(ctx context.Context, name types.Object) (*cxsdk.MultiSelectQueryMetricsQueryTypeMetricName, diag.Diagnostics) {
	if name.IsNull() || name.IsUnknown() {
		return nil, nil
	}

	var nameObject MetricAndLabelNameModel
	diags := name.As(ctx, &nameObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.MultiSelectQueryMetricsQueryTypeMetricName{
		MetricName: &cxsdk.MultiSelectQueryMetricsQueryTypeMetricNameInner{
			MetricRegex: utils.TypeStringToWrapperspbString(nameObject.MetricRegex),
		},
	}, nil
}

func expandMultiSelectMetricsQueryTypeLabelName(ctx context.Context, name types.Object) (*cxsdk.MultiSelectQueryMetricsQueryTypeLabelName, diag.Diagnostics) {
	if name.IsNull() || name.IsUnknown() {
		return nil, nil
	}

	var nameObject MetricAndLabelNameModel
	diags := name.As(ctx, &nameObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.MultiSelectQueryMetricsQueryTypeLabelName{
		LabelName: &cxsdk.MultiSelectQueryMetricsQueryTypeLabelNameInner{
			MetricRegex: utils.TypeStringToWrapperspbString(nameObject.MetricRegex),
		},
	}, nil
}

func expandMultiSelectMetricsQueryTypeLabelValue(ctx context.Context, value types.Object) (*cxsdk.MultiSelectQueryMetricsQueryTypeLabelValue, diag.Diagnostics) {
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

	return &cxsdk.MultiSelectQueryMetricsQueryTypeLabelValue{
		LabelValue: &cxsdk.MultiSelectQueryMetricsQueryTypeLabelValueInner{
			MetricName:   metricName,
			LabelName:    labelName,
			LabelFilters: labelFilters,
		},
	}, nil
}

func expandStringOrVariables(ctx context.Context, name types.List) ([]*cxsdk.MultiSelectQueryMetricsQueryStringOrVariable, diag.Diagnostics) {
	var nameObjects []types.Object
	var expandedNames []*cxsdk.MultiSelectQueryMetricsQueryStringOrVariable
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
		expandedNames = append(expandedNames, expandedName)
	}

	if diags.HasError() {
		return nil, diags
	}

	return expandedNames, nil
}

func expandStringOrVariable(ctx context.Context, name types.Object) (*cxsdk.MultiSelectQueryMetricsQueryStringOrVariable, diag.Diagnostics) {
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
		return &cxsdk.MultiSelectQueryMetricsQueryStringOrVariable{
			Value: &cxsdk.MultiSelectQueryMetricsQueryStringOrVariableVariable{
				VariableName: utils.TypeStringToWrapperspbString(nameObject.VariableName),
			},
		}, nil
	case !(nameObject.StringValue.IsNull() || nameObject.StringValue.IsUnknown()):
		return &cxsdk.MultiSelectQueryMetricsQueryStringOrVariable{
			Value: &cxsdk.MultiSelectQueryMetricsQueryStringOrVariableString{
				StringValue: utils.TypeStringToWrapperspbString(nameObject.StringValue),
			},
		}, nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand StringOrVariable", "StringOrVariable must be either VariableName or StringValue")}
	}
}

func expandMetricsLabelFilters(ctx context.Context, filters types.List) ([]*cxsdk.MultiSelectQueryMetricsQueryMetricsLabelFilter, diag.Diagnostics) {
	var filtersObjects []types.Object
	var expandedFilters []*cxsdk.MultiSelectQueryMetricsQueryMetricsLabelFilter
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
		expandedFilters = append(expandedFilters, expandedFilter)
	}

	if diags.HasError() {
		return nil, diags
	}

	return expandedFilters, nil
}

func expandMetricLabelFilter(ctx context.Context, filter MetricLabelFilterModel) (*cxsdk.MultiSelectQueryMetricsQueryMetricsLabelFilter, diag.Diagnostics) {
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

	return &cxsdk.MultiSelectQueryMetricsQueryMetricsLabelFilter{
		Metric:   metric,
		Label:    label,
		Operator: operator,
	}, nil
}

func expandMetricLabelFilterOperator(ctx context.Context, operator types.Object) (*cxsdk.MultiSelectQueryMetricsQueryOperator, diag.Diagnostics) {
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

	selection := &cxsdk.MultiSelectQueryMetricsQuerySelection{
		Value: &cxsdk.MultiSelectQueryMetricsQuerySelectionList{
			List: &cxsdk.MultiSelectQueryMetricsQuerySelectionListSelection{
				Values: values,
			},
		},
	}
	switch operatorObject.Type.ValueString() {
	case "equals":
		return &cxsdk.MultiSelectQueryMetricsQueryOperator{
			Value: &cxsdk.MultiSelectQueryMetricsQueryOperatorEquals{
				Equals: &cxsdk.MultiSelectQueryMetricsQueryEquals{
					Selection: selection,
				},
			},
		}, nil
	case "not_equals":
		return &cxsdk.MultiSelectQueryMetricsQueryOperator{
			Value: &cxsdk.MultiSelectQueryMetricsQueryOperatorNotEquals{
				NotEquals: &cxsdk.MultiSelectQueryMetricsQueryNotEquals{
					Selection: selection,
				},
			},
		}, nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand MetricLabelFilterOperator", fmt.Sprintf("Unknown operator type %s", operatorObject.Type.ValueString()))}
	}
}

func expandMultiSelectSpansQuery(ctx context.Context, spans types.Object) (*cxsdk.MultiSelectQuerySpansQuery, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(spans) {
		return nil, nil
	}

	var spansObject MultiSelectSpansQueryModel
	diags := spans.As(ctx, &spansObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	spansQuery := &cxsdk.MultiSelectQuerySpansQuery{
		SpansQuery: &cxsdk.MultiSelectQuerySpansQueryInner{
			Type: &cxsdk.MultiSelectQuerySpansQueryType{},
		},
	}

	switch {
	case !utils.ObjIsNullOrUnknown(spansObject.FieldName):
		spansQuery.SpansQuery.Type.Value, diags = expandMultiSelectSpansQueryTypeFieldName(ctx, spansObject.FieldName)
	case !utils.ObjIsNullOrUnknown(spansObject.FieldValue):
		spansQuery.SpansQuery.Type.Value, diags = expandMultiSelectSpansQueryTypeFieldValue(ctx, spansObject.FieldValue)
	default:
		diags = diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand MultiSelect Spans Query", "MultiSelect Spans Query must be either FieldName or FieldValue")}
	}

	if diags.HasError() {
		return nil, diags
	}

	return spansQuery, nil
}

func expandMultiSelectSpansQueryTypeFieldName(ctx context.Context, name types.Object) (*cxsdk.MultiSelectQuerySpansQueryTypeFieldName, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(name) {
		return nil, nil
	}

	var nameObject SpanFieldNameModel
	diags := name.As(ctx, &nameObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.MultiSelectQuerySpansQueryTypeFieldName{
		FieldName: &cxsdk.MultiSelectQuerySpansQueryTypeFieldNameInner{
			SpanRegex: utils.TypeStringToWrapperspbString(nameObject.SpanRegex),
		},
	}, nil
}

func expandMultiSelectSpansQueryTypeFieldValue(ctx context.Context, value types.Object) (*cxsdk.MultiSelectQuerySpansQueryTypeFieldValue, diag.Diagnostics) {
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

	return &cxsdk.MultiSelectQuerySpansQueryTypeFieldValue{
		FieldValue: &cxsdk.MultiSelectQuerySpansQueryTypeFieldValueInner{
			Value: spansField,
		},
	}, nil
}

func expandGaugeQueryMetrics(ctx context.Context, gaugeQueryMetrics *dashboardwidgets.GaugeQueryMetricsModel) (*cxsdk.GaugeMetricsQuery, diag.Diagnostics) {
	filters, diags := dashboardwidgets.ExpandMetricsFilters(ctx, gaugeQueryMetrics.Filters)
	if diags.HasError() {
		return nil, diags
	}

	timeFrame, diags := dashboardwidgets.ExpandTimeFrameSelect(ctx, gaugeQueryMetrics.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.GaugeMetricsQuery{
		PromqlQuery: dashboardwidgets.ExpandPromqlQuery(gaugeQueryMetrics.PromqlQuery),
		Aggregation: dashboardwidgets.DashboardSchemaToProtoGaugeAggregation[gaugeQueryMetrics.Aggregation.ValueString()],
		Filters:     filters,
		TimeFrame:   timeFrame,
	}, nil
}

func expandGaugeQueryLogs(ctx context.Context, gaugeQueryLogs *dashboardwidgets.GaugeQueryLogsModel) (*cxsdk.GaugeLogsQuery, diag.Diagnostics) {
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

	return &cxsdk.GaugeLogsQuery{
		LuceneQuery:     dashboardwidgets.ExpandLuceneQuery(gaugeQueryLogs.LuceneQuery),
		LogsAggregation: logsAggregation,
		Filters:         filters,
		TimeFrame:       timeFrame,
	}, nil
}

func expandBarChart(ctx context.Context, chart *dashboardwidgets.BarChartModel) (*cxsdk.WidgetDefinition, diag.Diagnostics) {
	var diags diag.Diagnostics

	query, diags := expandBarChartQuery(ctx, chart.Query)
	if diags.HasError() {
		return nil, diags
	}

	xaxis, dg := expandXAis(chart.XAxis)
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	return &cxsdk.WidgetDefinition{
		Value: &cxsdk.WidgetDefinitionBarChart{
			BarChart: &cxsdk.BarChart{
				Query:             query,
				MaxBarsPerChart:   utils.TypeInt64ToWrappedInt32(chart.MaxBarsPerChart),
				GroupNameTemplate: utils.TypeStringToWrapperspbString(chart.GroupNameTemplate),
				StackDefinition:   expandBarChartStackDefinition(chart.StackDefinition),
				ScaleType:         dashboardwidgets.DashboardSchemaToProtoScaleType[chart.ScaleType.ValueString()],
				ColorsBy:          expandColorsBy(chart.ColorsBy),
				XAxis:             xaxis,
				Unit:              dashboardwidgets.DashboardSchemaToProtoUnit[chart.Unit.ValueString()],
				SortBy:            dashboardwidgets.DashboardSchemaToProtoSortBy[chart.SortBy.ValueString()],
				ColorScheme:       utils.TypeStringToWrapperspbString(chart.ColorScheme),
				DataModeType:      dashboardwidgets.DashboardSchemaToProtoDataModeType[chart.DataModeType.ValueString()],
			},
		},
	}, nil
}

func expandColorsBy(colorsBy types.String) *cxsdk.DashboardsColorsBy {
	switch colorsBy.ValueString() {
	case "stack":
		return &cxsdk.DashboardsColorsBy{
			Value: &cxsdk.DashboardsColorsByStack{
				Stack: &cxsdk.DashboardsColorsByStackInner{},
			},
		}
	case "group_by":
		return &cxsdk.DashboardsColorsBy{
			Value: &cxsdk.DashboardsColorsByGroupBy{
				GroupBy: &cxsdk.DashboardsColorsByGroupByInner{},
			},
		}
	case "aggregation":
		return &cxsdk.DashboardsColorsBy{
			Value: &cxsdk.DashboardsColorsByAggregation{
				Aggregation: &cxsdk.DashboardsColorsByAggregationInner{},
			},
		}
	default:
		return nil
	}
}

func expandXAis(xaxis *dashboardwidgets.BarChartXAxisModel) (*cxsdk.BarChartXAxis, diag.Diagnostic) {
	if xaxis == nil {
		return nil, nil
	}

	switch {
	case xaxis.Time != nil:
		duration, err := time.ParseDuration(xaxis.Time.Interval.ValueString())
		if err != nil {
			return nil, diag.NewErrorDiagnostic("Error expand bar chart x axis", err.Error())
		}
		return &cxsdk.BarChartXAxis{
			Type: &cxsdk.BarChartXAxisTime{
				Time: &cxsdk.BarChartXAxisByTime{
					Interval:         durationpb.New(duration),
					BucketsPresented: utils.TypeInt64ToWrappedInt32(xaxis.Time.BucketsPresented),
				},
			},
		}, nil
	case xaxis.Value != nil:
		return &cxsdk.BarChartXAxis{
			Type: &cxsdk.BarChartXAxisValue{
				Value: &cxsdk.BarChartXAxisByValue{},
			},
		}, nil
	default:
		return nil, diag.NewErrorDiagnostic("Error expand bar chart x axis", "unknown x axis type")
	}
}
func expandBarChartQuery(ctx context.Context, query *dashboardwidgets.BarChartQueryModel) (*cxsdk.BarChartQuery, diag.Diagnostics) {
	if query == nil {
		return nil, nil
	}
	switch {
	case !(query.Logs.IsNull() || query.Logs.IsUnknown()):
		logsQuery, diags := expandBarChartLogsQuery(ctx, query.Logs)
		if diags.HasError() {
			return nil, diags
		}
		return &cxsdk.BarChartQuery{
			Value: &cxsdk.BarChartQueryLogs{
				Logs: logsQuery,
			},
		}, nil
	case !(query.Metrics.IsNull() || query.Metrics.IsUnknown()):
		metricsQuery, diags := expandBarChartMetricsQuery(ctx, query.Metrics)
		if diags.HasError() {
			return nil, diags
		}
		return &cxsdk.BarChartQuery{
			Value: &cxsdk.BarChartQueryMetrics{
				Metrics: metricsQuery,
			},
		}, nil
	case !(query.Spans.IsNull() || query.Spans.IsUnknown()):
		spansQuery, diags := expandBarChartSpansQuery(ctx, query.Spans)
		if diags.HasError() {
			return nil, diags
		}
		return &cxsdk.BarChartQuery{
			Value: &cxsdk.BarChartQuerySpans{
				Spans: spansQuery,
			},
		}, nil
	case !(query.DataPrime.IsNull() || query.DataPrime.IsUnknown()):
		dataPrimeQuery, diags := expandBarChartDataPrimeQuery(ctx, query.DataPrime)
		if diags.HasError() {
			return nil, diags
		}
		return &cxsdk.BarChartQuery{
			Value: &cxsdk.BarChartQueryDataprime{
				Dataprime: dataPrimeQuery,
			},
		}, nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error expand bar chart query", "unknown bar chart query type")}
	}
}

func expandHorizontalBarChartQuery(ctx context.Context, query *dashboardwidgets.HorizontalBarChartQueryModel) (*cxsdk.HorizontalBarChartQuery, diag.Diagnostics) {
	if query == nil {
		return nil, nil
	}
	switch {
	case !(query.Logs.IsNull() || query.Logs.IsUnknown()):
		logsQuery, diags := expandHorizontalBarChartLogsQuery(ctx, query.Logs)
		if diags.HasError() {
			return nil, diags
		}
		return &cxsdk.HorizontalBarChartQuery{
			Value: &cxsdk.HorizontalBarChartQueryLogs{
				Logs: logsQuery,
			},
		}, nil
	case !(query.Metrics.IsNull() || query.Metrics.IsUnknown()):
		metricsQuery, diags := expandHorizontalBarChartMetricsQuery(ctx, query.Metrics)
		if diags.HasError() {
			return nil, diags
		}
		return &cxsdk.HorizontalBarChartQuery{
			Value: &cxsdk.HorizontalBarChartQueryMetrics{
				Metrics: metricsQuery,
			},
		}, nil
	case !(query.Spans.IsNull() || query.Spans.IsUnknown()):
		spansQuery, diags := expandHorizontalBarChartSpansQuery(ctx, query.Spans)
		if diags.HasError() {
			return nil, diags
		}
		return &cxsdk.HorizontalBarChartQuery{
			Value: &cxsdk.HorizontalBarChartQuerySpans{
				Spans: spansQuery,
			},
		}, nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error expand bar chart query", "unknown bar chart query type")}
	}
}

func expandHorizontalBarChartLogsQuery(ctx context.Context, logs types.Object) (*cxsdk.HorizontalBarChartLogsQuery, diag.Diagnostics) {
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

	groupNames, diags := utils.TypeStringSliceToWrappedStringSlice(ctx, logsObject.GroupNames.Elements())
	if diags.HasError() {
		return nil, diags
	}

	timeFrame, diags := dashboardwidgets.ExpandTimeFrameSelect(ctx, logsObject.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.HorizontalBarChartLogsQuery{
		LuceneQuery:      dashboardwidgets.ExpandLuceneQuery(logsObject.LuceneQuery),
		Aggregation:      aggregation,
		Filters:          filters,
		GroupNames:       groupNames,
		StackedGroupName: utils.TypeStringToWrapperspbString(logsObject.StackedGroupName),
		TimeFrame:        timeFrame,
	}, nil
}

func expandHorizontalBarChartMetricsQuery(ctx context.Context, metrics types.Object) (*cxsdk.HorizontalBarChartMetricsQuery, diag.Diagnostics) {
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

	groupNames, diags := utils.TypeStringSliceToWrappedStringSlice(ctx, metricsObject.GroupNames.Elements())
	if diags.HasError() {
		return nil, diags
	}
	timeFrame, diags := dashboardwidgets.ExpandTimeFrameSelect(ctx, metricsObject.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.HorizontalBarChartMetricsQuery{
		PromqlQuery:      dashboardwidgets.ExpandPromqlQuery(metricsObject.PromqlQuery),
		Filters:          filters,
		GroupNames:       groupNames,
		StackedGroupName: utils.TypeStringToWrapperspbString(metricsObject.StackedGroupName),
		TimeFrame:        timeFrame,
	}, nil
}

func expandHorizontalBarChartSpansQuery(ctx context.Context, spans types.Object) (*cxsdk.HorizontalBarChartSpansQuery, diag.Diagnostics) {
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

	return &cxsdk.HorizontalBarChartSpansQuery{
		LuceneQuery:      dashboardwidgets.ExpandLuceneQuery(spansObject.LuceneQuery),
		Aggregation:      aggregation,
		Filters:          filters,
		GroupNames:       groupNames,
		StackedGroupName: expandedFilter,
		TimeFrame:        timeFrame,
	}, nil
}

func expandBarChartLogsQuery(ctx context.Context, barChartQueryLogs types.Object) (*cxsdk.BarChartLogsQuery, diag.Diagnostics) {
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

	groupNames, diags := utils.TypeStringSliceToWrappedStringSlice(ctx, barChartQueryLogsObject.GroupNames.Elements())
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

	return &cxsdk.BarChartLogsQuery{
		LuceneQuery:           dashboardwidgets.ExpandLuceneQuery(barChartQueryLogsObject.LuceneQuery),
		Aggregation:           aggregation,
		Filters:               filters,
		GroupNames:            groupNames,
		StackedGroupName:      utils.TypeStringToWrapperspbString(barChartQueryLogsObject.StackedGroupName),
		GroupNamesFields:      groupNamesFields,
		StackedGroupNameField: stackedGroupNameField,
		TimeFrame:             timeFrame,
	}, nil
}

func expandBarChartMetricsQuery(ctx context.Context, barChartQueryMetrics types.Object) (*cxsdk.BarChartMetricsQuery, diag.Diagnostics) {
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

	groupNames, diags := utils.TypeStringSliceToWrappedStringSlice(ctx, barChartQueryMetricsObject.GroupNames.Elements())
	if diags.HasError() {
		return nil, diags
	}

	timeFrame, diags := dashboardwidgets.ExpandTimeFrameSelect(ctx, barChartQueryMetricsObject.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.BarChartMetricsQuery{
		PromqlQuery:      dashboardwidgets.ExpandPromqlQuery(barChartQueryMetricsObject.PromqlQuery),
		Filters:          filters,
		GroupNames:       groupNames,
		StackedGroupName: utils.TypeStringToWrapperspbString(barChartQueryMetricsObject.StackedGroupName),
		TimeFrame:        timeFrame,
	}, nil
}

func expandBarChartSpansQuery(ctx context.Context, barChartQuerySpans types.Object) (*cxsdk.BarChartSpansQuery, diag.Diagnostics) {
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

	return &cxsdk.BarChartSpansQuery{
		LuceneQuery:      dashboardwidgets.ExpandLuceneQuery(barChartQuerySpansObject.LuceneQuery),
		Aggregation:      aggregation,
		Filters:          filters,
		GroupNames:       groupNames,
		StackedGroupName: expandedFilter,
		TimeFrame:        timeFrame,
	}, nil
}

func expandBarChartDataPrimeQuery(ctx context.Context, dataPrime types.Object) (*cxsdk.BarChartDataprimeQuery, diag.Diagnostics) {
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

	groupNames, diags := utils.TypeStringSliceToWrappedStringSlice(ctx, dataPrimeObject.GroupNames.Elements())
	if diags.HasError() {
		return nil, diags
	}

	timeFrame, diags := dashboardwidgets.ExpandTimeFrameSelect(ctx, dataPrimeObject.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	dataPrimeQuery := &cxsdk.DashboardDataprimeQuery{
		Text: dataPrimeObject.Query.ValueString(),
	}
	return &cxsdk.BarChartDataprimeQuery{
		Filters:          filters,
		DataprimeQuery:   dataPrimeQuery,
		GroupNames:       groupNames,
		StackedGroupName: utils.TypeStringToWrapperspbString(dataPrimeObject.StackedGroupName),
		TimeFrame:        timeFrame,
	}, nil
}

func expandPieChartQuery(ctx context.Context, pieChartQuery *dashboardwidgets.PieChartQueryModel) (*cxsdk.PieChartQuery, diag.Diagnostics) {
	if pieChartQuery == nil {
		return nil, nil
	}

	switch {
	case pieChartQuery.Logs != nil:
		logs, diags := expandPieChartLogsQuery(ctx, pieChartQuery.Logs)
		if diags.HasError() {
			return nil, diags
		}
		return &cxsdk.PieChartQuery{
			Value: logs,
		}, nil
	case pieChartQuery.Metrics != nil:
		metrics, diags := expandPieChartMetricsQuery(ctx, pieChartQuery.Metrics)
		if diags.HasError() {
			return nil, diags
		}
		return &cxsdk.PieChartQuery{
			Value: metrics,
		}, nil
	case pieChartQuery.Spans != nil:
		spans, diags := expandPieChartSpansQuery(ctx, pieChartQuery.Spans)
		if diags.HasError() {
			return nil, diags
		}
		return &cxsdk.PieChartQuery{
			Value: spans,
		}, nil
	case pieChartQuery.DataPrime != nil:
		dataPrime, diags := expandPieChartDataPrimeQuery(ctx, pieChartQuery.DataPrime)
		if diags.HasError() {
			return nil, diags
		}
		return &cxsdk.PieChartQuery{
			Value: dataPrime,
		}, nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand PieChart Query", "Unknown PieChart Query type")}
	}
}

func expandPieChartLogsQuery(ctx context.Context, pieChartQueryLogs *dashboardwidgets.PieChartQueryLogsModel) (*cxsdk.PieChartQueryLogs, diag.Diagnostics) {
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

	groupNames, diags := utils.TypeStringSliceToWrappedStringSlice(ctx, pieChartQueryLogs.GroupNames.Elements())
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

	return &cxsdk.PieChartQueryLogs{
		Logs: &cxsdk.PieChartLogsQuery{
			LuceneQuery:           dashboardwidgets.ExpandLuceneQuery(pieChartQueryLogs.LuceneQuery),
			Aggregation:           aggregation,
			Filters:               filters,
			GroupNames:            groupNames,
			StackedGroupName:      utils.TypeStringToWrapperspbString(pieChartQueryLogs.StackedGroupName),
			GroupNamesFields:      groupNamesFields,
			StackedGroupNameField: stackedGroupNameField,
			TimeFrame:             timeFrame,
		},
	}, nil
}

func expandPieChartMetricsQuery(ctx context.Context, pieChartQueryMetrics *dashboardwidgets.PieChartQueryMetricsModel) (*cxsdk.PieChartQueryMetrics, diag.Diagnostics) {
	if pieChartQueryMetrics == nil {
		return nil, nil
	}

	filters, diags := dashboardwidgets.ExpandMetricsFilters(ctx, pieChartQueryMetrics.Filters)
	if diags.HasError() {
		return nil, diags
	}

	groupNames, diags := utils.TypeStringSliceToWrappedStringSlice(ctx, pieChartQueryMetrics.GroupNames.Elements())
	if diags.HasError() {
		return nil, diags
	}

	timeFrame, diags := dashboardwidgets.ExpandTimeFrameSelect(ctx, pieChartQueryMetrics.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.PieChartQueryMetrics{
		Metrics: &cxsdk.PieChartMetricsQuery{
			PromqlQuery:      dashboardwidgets.ExpandPromqlQuery(pieChartQueryMetrics.PromqlQuery),
			GroupNames:       groupNames,
			Filters:          filters,
			StackedGroupName: utils.TypeStringToWrapperspbString(pieChartQueryMetrics.StackedGroupName),
			TimeFrame:        timeFrame,
		},
	}, nil
}

func expandPieChartSpansQuery(ctx context.Context, pieChartQuerySpans *dashboardwidgets.PieChartQuerySpansModel) (*cxsdk.PieChartQuerySpans, diag.Diagnostics) {
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

	return &cxsdk.PieChartQuerySpans{
		Spans: &cxsdk.PieChartSpansQuery{
			LuceneQuery:      dashboardwidgets.ExpandLuceneQuery(pieChartQuerySpans.LuceneQuery),
			Aggregation:      aggregation,
			Filters:          filters,
			GroupNames:       groupNames,
			StackedGroupName: stackedGroupName,
			TimeFrame:        timeFrame,
		},
	}, nil
}

func expandPieChartDataPrimeQuery(ctx context.Context, dataPrime *dashboardwidgets.PieChartQueryDataPrimeModel) (*cxsdk.PieChartQueryDataprime, diag.Diagnostics) {
	if dataPrime == nil {
		return nil, nil
	}

	filters, diags := dashboardwidgets.ExpandDashboardFiltersSources(ctx, dataPrime.Filters)
	if diags.HasError() {
		return nil, diags
	}

	groupNames, diags := utils.TypeStringSliceToWrappedStringSlice(ctx, dataPrime.GroupNames.Elements())
	if diags.HasError() {
		return nil, diags
	}

	timeFrame, diags := dashboardwidgets.ExpandTimeFrameSelect(ctx, dataPrime.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.PieChartQueryDataprime{
		Dataprime: &cxsdk.PieChartDataprimeQuery{
			DataprimeQuery: &cxsdk.DashboardDataprimeQuery{
				Text: dataPrime.Query.ValueString(),
			},
			Filters:          filters,
			GroupNames:       groupNames,
			StackedGroupName: utils.TypeStringToWrapperspbString(dataPrime.StackedGroupName),
			TimeFrame:        timeFrame,
		},
	}, nil
}

func expandDashboardVariables(ctx context.Context, variables types.List) ([]*cxsdk.DashboardVariable, diag.Diagnostics) {
	var variablesObjects []types.Object
	var expandedVariables []*cxsdk.DashboardVariable
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
		expandedVariables = append(expandedVariables, expandedVariable)
	}

	return expandedVariables, diags
}

func expandDashboardVariable(ctx context.Context, variable DashboardVariableModel) (*cxsdk.DashboardVariable, diag.Diagnostics) {
	definition, diags := expandDashboardVariableDefinition(ctx, variable.Definition)
	if diags.HasError() {
		return nil, diags
	}
	return &cxsdk.DashboardVariable{
		Name:        utils.TypeStringToWrapperspbString(variable.Name),
		DisplayName: utils.TypeStringToWrapperspbString(variable.DisplayName),
		Definition:  definition,
	}, nil
}

func expandDashboardVariableDefinition(ctx context.Context, definition *DashboardVariableDefinitionModel) (*cxsdk.DashboardVariableDefinition, diag.Diagnostics) {
	if definition == nil {
		return nil, nil
	}

	switch {
	case definition.MultiSelect != nil:
		return expandMultiSelect(ctx, definition.MultiSelect)
	case !definition.ConstantValue.IsNull():
		return &cxsdk.DashboardVariableDefinition{
			Value: &cxsdk.DashboardVariableDefinitionConstant{
				Constant: &cxsdk.DashboardConstant{
					Value: utils.TypeStringToWrapperspbString(definition.ConstantValue),
				},
			},
		}, nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand Dashboard Variable", fmt.Sprintf("unknown variable definition type: %T", definition))}
	}
}

func expandMultiSelect(ctx context.Context, multiSelect *VariableMultiSelectModel) (*cxsdk.DashboardVariableDefinition, diag.Diagnostics) {
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

	return &cxsdk.DashboardVariableDefinition{
		Value: &cxsdk.DashboardVariableDefinitionMultiSelect{
			MultiSelect: &cxsdk.DashboardMultiSelect{
				Source:               source,
				Selection:            selection,
				ValuesOrderDirection: dashboardwidgets.DashboardOrderDirectionSchemaToProto[multiSelect.ValuesOrderDirection.ValueString()],
			},
		},
	}, nil
}

func expandMultiSelectSelection(ctx context.Context, selectedValues []attr.Value) (*cxsdk.DashboardMultiSelectSelection, diag.Diagnostics) {
	if len(selectedValues) == 0 {
		return &cxsdk.DashboardMultiSelectSelection{
			Value: &cxsdk.DashboardMultiSelectSelectionAll{
				All: &cxsdk.DashboardMultiSelectAllSelection{},
			},
		}, nil
	}

	selections, diags := utils.TypeStringSliceToWrappedStringSlice(ctx, selectedValues)
	if diags.HasError() {
		return nil, diags
	}
	return &cxsdk.DashboardMultiSelectSelection{
		Value: &cxsdk.DashboardMultiSelectSelectionList{
			List: &cxsdk.DashboardMultiSelectListSelection{
				Values: selections,
			},
		},
	}, nil
}

func expandMultiSelectSource(ctx context.Context, source *VariableMultiSelectSourceModel) (*cxsdk.DashboardMultiSelectSource, diag.Diagnostics) {
	if source == nil {
		return nil, nil
	}

	switch {
	case !source.LogsPath.IsNull():
		return &cxsdk.MultiSelectSource{
			Value: &cxsdk.MultiSelectSourceLogsPath{
				LogsPath: &cxsdk.MultiSelectLogsPathSource{
					Value: utils.TypeStringToWrapperspbString(source.LogsPath),
				},
			},
		}, nil
	case !source.ConstantList.IsNull():
		constantList, diags := utils.TypeStringSliceToWrappedStringSlice(ctx, source.ConstantList.Elements())
		if diags.HasError() {
			return nil, diags
		}
		return &cxsdk.MultiSelectSource{
			Value: &cxsdk.MultiSelectSourceConstantList{
				ConstantList: &cxsdk.MultiSelectConstantListSource{
					Values: constantList,
				},
			},
		}, nil
	case source.MetricLabel != nil:
		return &cxsdk.MultiSelectSource{
			Value: &cxsdk.MultiSelectSourceMetricLabel{
				MetricLabel: &cxsdk.MultiSelectMetricLabelSource{
					MetricName: utils.TypeStringToWrapperspbString(source.MetricLabel.MetricName),
					Label:      utils.TypeStringToWrapperspbString(source.MetricLabel.Label),
				},
			},
		}, nil
	case source.SpanField != nil:
		spanField, dg := dashboardwidgets.ExpandSpansField(source.SpanField)
		if dg != nil {
			return nil, diag.Diagnostics{dg}
		}
		return &cxsdk.MultiSelectSource{
			Value: &cxsdk.MultiSelectSourceSpanField{
				SpanField: &cxsdk.MultiSelectSpanFieldSource{
					Value: spanField,
				},
			},
		}, nil
	case !source.Query.IsNull():
		return expandMultiSelectSourceQuery(ctx, source.Query)
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand Multi Select Source", fmt.Sprintf("unknown multi select source type: %T", source))}
	}
}

func expandDashboardFilters(ctx context.Context, filters types.List) ([]*cxsdk.DashboardFilter, diag.Diagnostics) {
	var filtersObjects []types.Object
	var expandedFilters []*cxsdk.DashboardFilter
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
		expandedFilters = append(expandedFilters, expandedFilter)
	}

	return expandedFilters, diags
}

func expandDashboardFilter(ctx context.Context, filter *DashboardFilterModel) (*cxsdk.DashboardFilter, diag.Diagnostics) {
	if filter == nil {
		return nil, nil
	}

	source, diags := dashboardwidgets.ExpandFilterSource(ctx, filter.Source)
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.DashboardFilter{
		Source:    source,
		Enabled:   utils.TypeBoolToWrapperspbBool(filter.Enabled),
		Collapsed: utils.TypeBoolToWrapperspbBool(filter.Collapsed),
	}, nil
}

func expandDashboardFolder(ctx context.Context, dashboard *cxsdk.Dashboard, folder types.Object) (*cxsdk.Dashboard, diag.Diagnostics) {
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
		dashboard.FolderPath = &cxsdk.FolderPath{
			Segments: segments,
		}
	}

	return dashboard, nil
}

func flattenDashboard(ctx context.Context, plan DashboardResourceModel, dashboard *cxsdk.Dashboard) (*DashboardResourceModel, diag.Diagnostics) {
	folder, diags := flattenDashboardFolder(ctx, plan.Folder, dashboard)
	if diags.HasError() {
		return nil, diags
	}
	if !(plan.ContentJson.IsNull() || plan.ContentJson.IsUnknown()) {

		var unmarshalledDashboard = new(cxsdk.Dashboard)
		// Users can set the folder in the dashbaord's json. In that case, the server will return a folder, but we're not supposed to set it in the plan,
		// or terraform will panic.
		err := dashboardschema.JSONUnmarshal.Unmarshal([]byte(plan.ContentJson.ValueString()), unmarshalledDashboard)
		if err != nil {
			return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Unmarshal Dashboard", err.Error())}
		}
		if unmarshalledDashboard.FolderId != nil || unmarshalledDashboard.FolderPath != nil {
			folder = types.ObjectNull(dashboardFolderModelAttr())
		}

		_, err = protojson.Marshal(dashboard)
		if err != nil {
			return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Dashboard", err.Error())}
		}

		//if diffType, diffString := jsondiff.Compare([]byte(plan.ContentJson.ValueString()), contentJson, &jsondiff.Options{}); !(diffType == jsondiff.FullMatch || diffType == jsondiff.SupersetMatch) {
		//	return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Dashboard", fmt.Sprintf("ContentJson does not match the dashboard content: %s", diffString))}
		//}

		return &DashboardResourceModel{
			ContentJson: types.StringValue(plan.ContentJson.ValueString()),
			ID:          types.StringValue(dashboard.GetId().GetValue()),
			Name:        types.StringNull(),
			Description: types.StringNull(),
			Layout:      types.ObjectNull(layoutModelAttr()),
			Variables:   types.ListNull(types.ObjectType{AttrTypes: dashboardsVariablesModelAttr()}),
			Filters:     types.ListNull(types.ObjectType{AttrTypes: dashboardsFiltersModelAttr()}),
			TimeFrame:   nil,
			Folder:      folder,
			Annotations: types.ListNull(types.ObjectType{AttrTypes: dashboardsAnnotationsModelAttr()}),
			AutoRefresh: types.ObjectNull(dashboardAutoRefreshModelAttr()),
		}, nil
	}

	layout, diags := flattenDashboardLayout(ctx, dashboard.GetLayout())
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
		ID:          types.StringValue(dashboard.GetId().GetValue()),
		Name:        utils.WrapperspbStringToTypeString(dashboard.GetName()),
		Description: utils.WrapperspbStringToTypeString(dashboard.GetDescription()),
		Layout:      layout,
		Variables:   variables,
		Filters:     filters,
		TimeFrame:   timeFrame,
		Folder:      folder,
		Annotations: annotations,
		AutoRefresh: autoRefresh,
		ContentJson: types.StringNull(),
	}, nil
}

func flattenDashboardLayout(ctx context.Context, layout *cxsdk.DashboardLayout) (types.Object, diag.Diagnostics) {
	sections, diags := flattenDashboardSections(ctx, layout.GetSections())
	if diags.HasError() {
		return types.ObjectNull(layoutModelAttr()), diags
	}
	flattenedLayout := &DashboardLayoutModel{
		Sections: sections,
	}
	return types.ObjectValueFrom(ctx, layoutModelAttr(), flattenedLayout)
}

func flattenDashboardSections(ctx context.Context, sections []*cxsdk.DashboardSection) (types.List, diag.Diagnostics) {
	if len(sections) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: sectionModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	sectionsElements := make([]attr.Value, 0)
	for _, section := range sections {
		flattenedSection, diags := flattenDashboardSection(ctx, section)
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

func flattenDashboardSection(ctx context.Context, section *cxsdk.DashboardSection) (*SectionModel, diag.Diagnostics) {
	if section == nil {
		return nil, nil
	}

	rows, diags := flattenDashboardRows(ctx, section.GetRows())
	if diags.HasError() {
		return nil, diags
	}

	options, diags := flattenDashboardOptions(ctx, section.GetOptions())
	if diags.HasError() {
		return nil, diags
	}

	return &SectionModel{
		ID:      types.StringValue(section.GetId().GetValue()),
		Rows:    rows,
		Options: options,
	}, nil
}

func flattenDashboardOptions(_ context.Context, opts *cxsdk.DashboardSectionOptions) (*SectionOptionsModel, diag.Diagnostics) {
	if opts == nil || opts.GetCustom() == nil {
		return nil, nil
	}
	var description basetypes.StringValue
	if opts.GetCustom().Description != nil {
		description = types.StringValue(opts.GetCustom().Description.GetValue())
	} else {
		description = types.StringNull()
	}

	var collapsed basetypes.BoolValue
	if opts.GetCustom().Description != nil {
		collapsed = types.BoolValue(opts.GetCustom().Collapsed.GetValue())
	} else {
		collapsed = types.BoolNull()
	}

	var color basetypes.StringValue
	if opts.GetCustom().Color != nil {
		colorString := opts.GetCustom().Color.GetPredefined().String()
		colors := strings.Split(colorString, "_")
		color = types.StringValue(strings.ToLower(colors[len(colors)-1]))
	} else {
		color = types.StringNull()
	}

	return &SectionOptionsModel{
		Name:        types.StringValue(opts.GetCustom().Name.GetValue()),
		Description: description,
		Collapsed:   collapsed,
		Color:       color,
	}, nil
}

func flattenDashboardRows(ctx context.Context, rows []*cxsdk.DashboardRow) (types.List, diag.Diagnostics) {
	if len(rows) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: rowModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	rowsElements := make([]attr.Value, 0)
	for _, row := range rows {
		flattenedRow, diags := flattenDashboardRow(ctx, row)
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

func flattenDashboardRow(ctx context.Context, row *cxsdk.DashboardRow) (*RowModel, diag.Diagnostics) {
	if row == nil {
		return nil, nil
	}

	widgets, diags := flattenDashboardWidgets(ctx, row.GetWidgets())
	if diags.HasError() {
		return nil, diags
	}
	return &RowModel{
		ID:      types.StringValue(row.GetId().GetValue()),
		Height:  utils.WrapperspbInt32ToTypeInt64(row.GetAppearance().GetHeight()),
		Widgets: widgets,
	}, nil
}

func flattenDashboardWidgets(ctx context.Context, widgets []*cxsdk.DashboardWidget) (types.List, diag.Diagnostics) {
	if len(widgets) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: widgetModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	widgetsElements := make([]attr.Value, 0)
	for _, widget := range widgets {
		flattenedWidget, diags := flattenDashboardWidget(ctx, widget)
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

func flattenDashboardWidget(ctx context.Context, widget *cxsdk.DashboardWidget) (*WidgetModel, diag.Diagnostics) {
	if widget == nil {
		return nil, nil
	}

	definition, diags := flattenDashboardWidgetDefinition(ctx, widget.GetDefinition())
	if diags.HasError() {
		return nil, diags
	}

	return &WidgetModel{
		ID:          types.StringValue(widget.GetId().GetValue()),
		Title:       utils.WrapperspbStringToTypeString(widget.GetTitle()),
		Description: utils.WrapperspbStringToTypeString(widget.GetDescription()),
		Width:       utils.WrapperspbInt32ToTypeInt64(widget.GetAppearance().GetWidth()),
		Definition:  definition,
	}, nil
}

func flattenDashboardWidgetDefinition(ctx context.Context, definition *cxsdk.WidgetDefinition) (*dashboardwidgets.WidgetDefinitionModel, diag.Diagnostics) {
	if definition == nil {
		return nil, nil
	}

	switch definition.GetValue().(type) {
	case *cxsdk.WidgetDefinitionLineChart:
		return dashboardwidgets.FlattenLineChart(ctx, definition.GetLineChart())
	case *cxsdk.WidgetDefinitionHexagon:
		return dashboardwidgets.FlattenHexagon(ctx, definition.GetHexagon())
	case *cxsdk.WidgetDefinitionDataTable:
		return dashboardwidgets.FlattenDataTable(ctx, definition.GetDataTable())
	case *cxsdk.WidgetDefinitionGauge:
		return flattenGauge(ctx, definition.GetGauge())
	case *cxsdk.WidgetDefinitionPieChart:
		return flattenPieChart(ctx, definition.GetPieChart())
	case *cxsdk.WidgetDefinitionBarChart:
		return flattenBarChart(ctx, definition.GetBarChart())
	case *cxsdk.WidgetDefinitionHorizontalBarChart:
		return flattenHorizontalBarChart(ctx, definition.GetHorizontalBarChart())
	case *cxsdk.WidgetDefinitionMarkdown:
		return flattenMarkdown(definition.GetMarkdown()), nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Widget Definition", "unknown widget definition type")}
	}
}

func flattenMarkdown(markdown *cxsdk.Markdown) *dashboardwidgets.WidgetDefinitionModel {
	return &dashboardwidgets.WidgetDefinitionModel{
		Markdown: &dashboardwidgets.MarkdownModel{
			MarkdownText: utils.WrapperspbStringToTypeString(markdown.GetMarkdownText()),
			TooltipText:  utils.WrapperspbStringToTypeString(markdown.GetTooltipText()),
		},
	}
}

func flattenHorizontalBarChart(ctx context.Context, chart *cxsdk.HorizontalBarChart) (*dashboardwidgets.WidgetDefinitionModel, diag.Diagnostics) {
	if chart == nil {
		return nil, nil
	}

	query, diags := flattenHorizontalBarChartQueryDefinitions(ctx, chart.GetQuery())
	if diags.HasError() {
		return nil, diags
	}

	colorsBy, dg := flattenBarChartColorsBy(chart.GetColorsBy())
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	return &dashboardwidgets.WidgetDefinitionModel{
		HorizontalBarChart: &dashboardwidgets.HorizontalBarChartModel{
			Query:             query,
			MaxBarsPerChart:   utils.WrapperspbInt32ToTypeInt64(chart.GetMaxBarsPerChart()),
			GroupNameTemplate: utils.WrapperspbStringToTypeString(chart.GetGroupNameTemplate()),
			StackDefinition:   flattenHorizontalBarChartStackDefinition(chart.GetStackDefinition()),
			ScaleType:         types.StringValue(dashboardwidgets.DashboardProtoToSchemaScaleType[chart.GetScaleType()]),
			ColorsBy:          colorsBy,
			Unit:              types.StringValue(dashboardwidgets.DashboardProtoToSchemaUnit[chart.GetUnit()]),
			DisplayOnBar:      utils.WrapperspbBoolToTypeBool(chart.GetDisplayOnBar()),
			YAxisViewBy:       flattenYAxisViewBy(chart.GetYAxisViewBy()),
			SortBy:            types.StringValue(dashboardwidgets.DashboardProtoToSchemaSortBy[chart.GetSortBy()]),
			ColorScheme:       utils.WrapperspbStringToTypeString(chart.GetColorScheme()),
			DataModeType:      types.StringValue(dashboardwidgets.DashboardProtoToSchemaDataModeType[chart.GetDataModeType()]),
		},
	}, nil
}

func flattenYAxisViewBy(yAxisViewBy *cxsdk.HorizontalBarChartYAxisViewBy) types.String {
	switch yAxisViewBy.GetYAxisView().(type) {
	case *cxsdk.HorizontalBarChartYAxisViewByCategory:
		return types.StringValue("category")
	case *cxsdk.HorizontalBarChartYAxisViewByValue:
		return types.StringValue("value")
	default:
		return types.StringNull()
	}
}

func flattenHorizontalBarChartQueryDefinitions(ctx context.Context, query *cxsdk.HorizontalBarChartQuery) (*dashboardwidgets.HorizontalBarChartQueryModel, diag.Diagnostics) {
	if query == nil {
		return nil, nil
	}

	switch query.GetValue().(type) {
	case *cxsdk.HorizontalBarChartQueryLogs:
		return flattenHorizontalBarChartQueryLogs(ctx, query.GetLogs())
	case *cxsdk.HorizontalBarChartQueryMetrics:
		return flattenHorizontalBarChartQueryMetrics(ctx, query.GetMetrics())
	case *cxsdk.HorizontalBarChartQuerySpans:
		return flattenHorizontalBarChartQuerySpans(ctx, query.GetSpans())
	case *cxsdk.HorizontalBarChartQueryDataprime:
		return flattenHorizontalBarChartQueryDataPrime(ctx, query.GetDataprime())
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Horizontal Bar Chart Query", "unknown horizontal bar chart query type")}
	}
}

func flattenHorizontalBarChartQueryDataPrime(ctx context.Context, dataPrime *cxsdk.HorizontalBarChartDataprimeQuery) (*dashboardwidgets.HorizontalBarChartQueryModel, diag.Diagnostics) {
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
		Query:            types.StringValue(dataPrime.GetDataprimeQuery().GetText()),
		Filters:          filters,
		GroupNames:       utils.WrappedStringSliceToTypeStringList(dataPrime.GetGroupNames()),
		StackedGroupName: utils.WrapperspbStringToTypeString(dataPrime.GetStackedGroupName()),
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

func flattenHorizontalBarChartQueryLogs(ctx context.Context, logs *cxsdk.HorizontalBarChartLogsQuery) (*dashboardwidgets.HorizontalBarChartQueryModel, diag.Diagnostics) {
	if logs == nil {
		return nil, nil
	}

	aggregation, diags := dashboardwidgets.FlattenLogsAggregation(ctx, logs.GetAggregation())
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

	stackedGroupNameField, diags := dashboardwidgets.FlattenObservationField(ctx, logs.GetStackedGroupNameField())
	if diags.HasError() {
		return nil, diags
	}

	timeFrame, diags := dashboardwidgets.FlattenTimeFrameSelect(ctx, logs.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	logsModel := &dashboardwidgets.BarChartQueryLogsModel{
		LuceneQuery:           utils.WrapperspbStringToTypeString(logs.GetLuceneQuery().GetValue()),
		Aggregation:           aggregation,
		Filters:               filters,
		GroupNames:            utils.WrappedStringSliceToTypeStringList(logs.GetGroupNames()),
		StackedGroupName:      utils.WrapperspbStringToTypeString(logs.GetStackedGroupName()),
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

func flattenHorizontalBarChartQueryMetrics(ctx context.Context, metrics *cxsdk.HorizontalBarChartMetricsQuery) (*dashboardwidgets.HorizontalBarChartQueryModel, diag.Diagnostics) {
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
		PromqlQuery:      utils.WrapperspbStringToTypeString(metrics.GetPromqlQuery().GetValue()),
		Filters:          filters,
		GroupNames:       utils.WrappedStringSliceToTypeStringList(metrics.GetGroupNames()),
		StackedGroupName: utils.WrapperspbStringToTypeString(metrics.GetStackedGroupName()),
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

func flattenHorizontalBarChartQuerySpans(ctx context.Context, spans *cxsdk.HorizontalBarChartSpansQuery) (*dashboardwidgets.HorizontalBarChartQueryModel, diag.Diagnostics) {
	if spans == nil {
		return nil, nil
	}

	aggregation, dg := dashboardwidgets.FlattenSpansAggregation(spans.GetAggregation())
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

	stackedGroupName, dg := dashboardwidgets.FlattenSpansField(spans.GetStackedGroupName())
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	timeFrame, diags := dashboardwidgets.FlattenTimeFrameSelect(ctx, spans.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	flattenedSpans := &dashboardwidgets.BarChartQuerySpansModel{
		LuceneQuery:      utils.WrapperspbStringToTypeString(spans.GetLuceneQuery().GetValue()),
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

func flattenGauge(ctx context.Context, gauge *cxsdk.Gauge) (*dashboardwidgets.WidgetDefinitionModel, diag.Diagnostics) {
	if gauge == nil {
		return nil, nil
	}

	query, diags := flattenGaugeQueries(ctx, gauge.GetQuery())
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
			Min:               utils.WrapperspbDoubleToTypeFloat64(gauge.GetMin()),
			Max:               utils.WrapperspbDoubleToTypeFloat64(gauge.GetMax()),
			ShowInnerArc:      utils.WrapperspbBoolToTypeBool(gauge.GetShowInnerArc()),
			ShowOuterArc:      utils.WrapperspbBoolToTypeBool(gauge.GetShowOuterArc()),
			Unit:              types.StringValue(dashboardwidgets.DashboardProtoToSchemaGaugeUnit[gauge.GetUnit()]),
			Thresholds:        thresholds,
			DataModeType:      types.StringValue(dashboardwidgets.DashboardProtoToSchemaDataModeType[gauge.GetDataModeType()]),
			ThresholdBy:       types.StringValue(dashboardwidgets.DashboardProtoToSchemaGaugeThresholdBy[gauge.GetThresholdBy()]),
			DisplaySeriesName: utils.WrapperspbBoolToTypeBool(gauge.GetDisplaySeriesName()),
			Decimal:           utils.WrappedInt32TotypeNumber(gauge.GetDecimal()),
		},
	}, nil
}

func flattenGaugeThresholds(ctx context.Context, thresholds []*cxsdk.GaugeThreshold) (types.List, diag.Diagnostics) {
	if len(thresholds) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: gaugeThresholdModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	thresholdElements := make([]attr.Value, 0)
	for _, threshold := range thresholds {
		flattenedThreshold := flattenGaugeThreshold(threshold)
		thresholdElement, diags := types.ObjectValueFrom(ctx, gaugeThresholdModelAttr(), flattenedThreshold)
		if diags.HasError() {
			diagnostics = append(diagnostics, diags...)
			continue
		}
		thresholdElements = append(thresholdElements, thresholdElement)
	}

	return types.ListValueMust(types.ObjectType{AttrTypes: gaugeThresholdModelAttr()}, thresholdElements), diagnostics
}

func flattenGaugeThreshold(threshold *cxsdk.GaugeThreshold) *dashboardwidgets.GaugeThresholdModel {
	if threshold == nil {
		return nil
	}
	return &dashboardwidgets.GaugeThresholdModel{
		From:  utils.WrapperspbDoubleToTypeFloat64(threshold.GetFrom()),
		Color: utils.WrapperspbStringToTypeString(threshold.GetColor()),
		Label: utils.WrapperspbStringToTypeString(threshold.GetLabel()),
	}
}

func flattenGaugeQueries(ctx context.Context, query *cxsdk.GaugeQuery) (*dashboardwidgets.GaugeQueryModel, diag.Diagnostics) {
	switch query.GetValue().(type) {
	case *cxsdk.GaugeQueryMetrics:
		return flattenGaugeQueryMetrics(ctx, query.GetMetrics())
	case *cxsdk.GaugeQueryLogs:
		return flattenGaugeQueryLogs(ctx, query.GetLogs())
	case *cxsdk.GaugeQuerySpans:
		return flattenGaugeQuerySpans(ctx, query.GetSpans())
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Gauge Query", fmt.Sprintf("unknown query type %T", query))}
	}
}

func flattenGaugeQueryMetrics(ctx context.Context, metrics *cxsdk.GaugeMetricsQuery) (*dashboardwidgets.GaugeQueryModel, diag.Diagnostics) {
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
			PromqlQuery: utils.WrapperspbStringToTypeString(metrics.GetPromqlQuery().GetValue()),
			Aggregation: types.StringValue(dashboardwidgets.DashboardProtoToSchemaGaugeAggregation[metrics.GetAggregation()]),
			Filters:     filters,
			TimeFrame:   timeFrame,
		},
	}, nil
}

func flattenGaugeQueryLogs(ctx context.Context, logs *cxsdk.GaugeLogsQuery) (*dashboardwidgets.GaugeQueryModel, diag.Diagnostics) {
	if logs == nil {
		return nil, nil
	}

	filters, diags := dashboardwidgets.FlattenLogsFilters(ctx, logs.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	logsAggregation, diags := dashboardwidgets.FlattenLogsAggregation(ctx, logs.GetLogsAggregation())
	if diags.HasError() {
		return nil, diags
	}
	timeFrame, diags := dashboardwidgets.FlattenTimeFrameSelect(ctx, logs.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardwidgets.GaugeQueryModel{
		Logs: &dashboardwidgets.GaugeQueryLogsModel{
			LuceneQuery:     utils.WrapperspbStringToTypeString(logs.GetLuceneQuery().GetValue()),
			LogsAggregation: logsAggregation,
			Filters:         filters,
			TimeFrame:       timeFrame,
		},
	}, nil
}

func flattenGaugeQuerySpans(ctx context.Context, spans *cxsdk.GaugeSpansQuery) (*dashboardwidgets.GaugeQueryModel, diag.Diagnostics) {
	if spans == nil {
		return nil, nil
	}

	filters, diags := dashboardwidgets.FlattenSpansFilters(ctx, spans.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	spansAggregation, dg := dashboardwidgets.FlattenSpansAggregation(spans.GetSpansAggregation())
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}
	timeFrame, diags := dashboardwidgets.FlattenTimeFrameSelect(ctx, spans.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardwidgets.GaugeQueryModel{
		Spans: &dashboardwidgets.GaugeQuerySpansModel{
			LuceneQuery:      utils.WrapperspbStringToTypeString(spans.GetLuceneQuery().GetValue()),
			Filters:          filters,
			SpansAggregation: spansAggregation,
			TimeFrame:        timeFrame,
		},
	}, nil
}

func flattenPieChart(ctx context.Context, pieChart *cxsdk.PieChart) (*dashboardwidgets.WidgetDefinitionModel, diag.Diagnostics) {
	if pieChart == nil {
		return nil, nil
	}

	query, diags := flattenPieChartQueries(ctx, pieChart.GetQuery())
	if diags != nil {
		return nil, diags
	}

	return &dashboardwidgets.WidgetDefinitionModel{
		PieChart: &dashboardwidgets.PieChartModel{
			Query:              query,
			MaxSlicesPerChart:  utils.WrapperspbInt32ToTypeInt64(pieChart.GetMaxSlicesPerChart()),
			MinSlicePercentage: utils.WrapperspbInt32ToTypeInt64(pieChart.GetMinSlicePercentage()),
			StackDefinition:    flattenPieChartStackDefinition(pieChart.GetStackDefinition()),
			LabelDefinition:    flattenPieChartLabelDefinition(pieChart.GetLabelDefinition()),
			ShowLegend:         utils.WrapperspbBoolToTypeBool(pieChart.GetShowLegend()),
			GroupNameTemplate:  utils.WrapperspbStringToTypeString(pieChart.GetGroupNameTemplate()),
			Unit:               types.StringValue(dashboardwidgets.DashboardProtoToSchemaUnit[pieChart.GetUnit()]),
			ColorScheme:        utils.WrapperspbStringToTypeString(pieChart.GetColorScheme()),
			DataModeType:       types.StringValue(dashboardwidgets.DashboardProtoToSchemaDataModeType[pieChart.GetDataModeType()]),
		},
	}, nil
}

func flattenPieChartQueries(ctx context.Context, query *cxsdk.PieChartQuery) (*dashboardwidgets.PieChartQueryModel, diag.Diagnostics) {
	if query == nil {
		return nil, nil
	}

	switch query.GetValue().(type) {
	case *cxsdk.PieChartQueryMetrics:
		return flattenPieChartQueryMetrics(ctx, query.GetMetrics())
	case *cxsdk.PieChartQueryLogs:
		return flattenPieChartQueryLogs(ctx, query.GetLogs())
	case *cxsdk.PieChartQuerySpans:
		return flattenPieChartQuerySpans(ctx, query.GetSpans())
	case *cxsdk.PieChartQueryDataprime:
		return flattenPieChartDataPrimeQuery(ctx, query.GetDataprime())
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Pie Chart Query", fmt.Sprintf("unknown query type %T", query))}
	}
}

func flattenPieChartStackDefinition(stackDefinition *cxsdk.PieChartStackDefinition) *dashboardwidgets.PieChartStackDefinitionModel {
	if stackDefinition == nil {
		return nil
	}

	return &dashboardwidgets.PieChartStackDefinitionModel{
		MaxSlicesPerStack: utils.WrapperspbInt32ToTypeInt64(stackDefinition.GetMaxSlicesPerStack()),
		StackNameTemplate: utils.WrapperspbStringToTypeString(stackDefinition.GetStackNameTemplate()),
	}
}

func flattenPieChartLabelDefinition(labelDefinition *cxsdk.PieChartLabelDefinition) *dashboardwidgets.LabelDefinitionModel {
	if labelDefinition == nil {
		return nil
	}
	return &dashboardwidgets.LabelDefinitionModel{
		LabelSource:    types.StringValue(dashboardwidgets.DashboardProtoToSchemaPieChartLabelSource[labelDefinition.GetLabelSource()]),
		IsVisible:      utils.WrapperspbBoolToTypeBool(labelDefinition.GetIsVisible()),
		ShowName:       utils.WrapperspbBoolToTypeBool(labelDefinition.GetShowName()),
		ShowValue:      utils.WrapperspbBoolToTypeBool(labelDefinition.GetShowValue()),
		ShowPercentage: utils.WrapperspbBoolToTypeBool(labelDefinition.GetShowPercentage()),
	}
}

func flattenPieChartQueryMetrics(ctx context.Context, metrics *cxsdk.PieChartMetricsQuery) (*dashboardwidgets.PieChartQueryModel, diag.Diagnostics) {
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
			PromqlQuery:      utils.WrapperspbStringToTypeString(metrics.GetPromqlQuery().GetValue()),
			Filters:          filters,
			GroupNames:       utils.WrappedStringSliceToTypeStringList(metrics.GetGroupNames()),
			StackedGroupName: utils.WrapperspbStringToTypeString(metrics.GetStackedGroupName()),
			TimeFrame:        timeFrame,
		},
	}, nil
}

func flattenPieChartQueryLogs(ctx context.Context, logs *cxsdk.PieChartLogsQuery) (*dashboardwidgets.PieChartQueryModel, diag.Diagnostics) {
	if logs == nil {
		return nil, nil
	}

	filters, diags := dashboardwidgets.FlattenLogsFilters(ctx, logs.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	aggregation, diags := dashboardwidgets.FlattenLogsAggregation(ctx, logs.GetAggregation())
	if diags.HasError() {
		return nil, diags
	}

	groupNamesFields, diags := dashboardwidgets.FlattenObservationFields(ctx, logs.GetGroupNamesFields())
	if diags.HasError() {
		return nil, diags
	}

	stackedGroupNameField, diags := dashboardwidgets.FlattenObservationField(ctx, logs.GetStackedGroupNameField())
	if diags.HasError() {
		return nil, diags
	}

	timeFrame, diags := dashboardwidgets.FlattenTimeFrameSelect(ctx, logs.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardwidgets.PieChartQueryModel{
		Logs: &dashboardwidgets.PieChartQueryLogsModel{
			LuceneQuery:           utils.WrapperspbStringToTypeString(logs.GetLuceneQuery().GetValue()),
			Aggregation:           aggregation,
			Filters:               filters,
			GroupNames:            utils.WrappedStringSliceToTypeStringList(logs.GetGroupNames()),
			StackedGroupName:      utils.WrapperspbStringToTypeString(logs.GetStackedGroupName()),
			GroupNamesFields:      groupNamesFields,
			StackedGroupNameField: stackedGroupNameField,
			TimeFrame:             timeFrame,
		},
	}, nil
}

func flattenPieChartQuerySpans(ctx context.Context, spans *cxsdk.PieChartSpansQuery) (*dashboardwidgets.PieChartQueryModel, diag.Diagnostics) {
	if spans == nil {
		return nil, nil
	}

	filters, diags := dashboardwidgets.FlattenSpansFilters(ctx, spans.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	aggregation, dg := dashboardwidgets.FlattenSpansAggregation(spans.GetAggregation())
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	stackedGroupName, dg := dashboardwidgets.FlattenSpansField(spans.GetStackedGroupName())
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
			LuceneQuery:      utils.WrapperspbStringToTypeString(spans.GetLuceneQuery().GetValue()),
			Filters:          filters,
			Aggregation:      aggregation,
			GroupNames:       groupNames,
			StackedGroupName: stackedGroupName,
			TimeFrame:        timeFrame,
		},
	}, nil
}

func flattenPieChartDataPrimeQuery(ctx context.Context, dataPrime *cxsdk.PieChartDataprimeQuery) (*dashboardwidgets.PieChartQueryModel, diag.Diagnostics) {
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
			Query:            types.StringValue(dataPrime.GetDataprimeQuery().GetText()),
			Filters:          filters,
			GroupNames:       utils.WrappedStringSliceToTypeStringList(dataPrime.GetGroupNames()),
			StackedGroupName: utils.WrapperspbStringToTypeString(dataPrime.GetStackedGroupName()),
			TimeFrame:        timeFrame,
		},
	}, nil
}

func flattenBarChart(ctx context.Context, barChart *cxsdk.BarChart) (*dashboardwidgets.WidgetDefinitionModel, diag.Diagnostics) {
	if barChart == nil {
		return nil, nil
	}

	query, diags := flattenBarChartQuery(ctx, barChart.GetQuery())
	if diags != nil {
		return nil, diags
	}

	colorsBy, dg := flattenBarChartColorsBy(barChart.GetColorsBy())
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	xAxis, dg := flattenBarChartXAxis(barChart.GetXAxis())
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	return &dashboardwidgets.WidgetDefinitionModel{
		BarChart: &dashboardwidgets.BarChartModel{
			Query:             query,
			MaxBarsPerChart:   utils.WrapperspbInt32ToTypeInt64(barChart.GetMaxBarsPerChart()),
			GroupNameTemplate: utils.WrapperspbStringToTypeString(barChart.GetGroupNameTemplate()),
			StackDefinition:   flattenBarChartStackDefinition(barChart.GetStackDefinition()),
			ScaleType:         types.StringValue(dashboardwidgets.DashboardProtoToSchemaScaleType[barChart.GetScaleType()]),
			ColorsBy:          colorsBy,
			XAxis:             xAxis,
			Unit:              types.StringValue(dashboardwidgets.DashboardProtoToSchemaUnit[barChart.GetUnit()]),
			SortBy:            types.StringValue(dashboardwidgets.DashboardProtoToSchemaSortBy[barChart.GetSortBy()]),
			ColorScheme:       utils.WrapperspbStringToTypeString(barChart.GetColorScheme()),
			DataModeType:      types.StringValue(dashboardwidgets.DashboardProtoToSchemaDataModeType[barChart.GetDataModeType()]),
		},
	}, nil
}

func flattenBarChartXAxis(axis *cxsdk.BarChartXAxis) (*dashboardwidgets.BarChartXAxisModel, diag.Diagnostic) {
	if axis == nil {
		return nil, nil
	}

	switch axis.GetType().(type) {
	case *cxsdk.BarChartXAxisTime:
		return &dashboardwidgets.BarChartXAxisModel{
			Time: &dashboardwidgets.BarChartXAxisTimeModel{
				Interval:         types.StringValue(axis.GetTime().GetInterval().AsDuration().String()),
				BucketsPresented: utils.WrapperspbInt32ToTypeInt64(axis.GetTime().GetBucketsPresented()),
			},
		}, nil
	case *cxsdk.BarChartXAxisValue:
		return &dashboardwidgets.BarChartXAxisModel{
			Value: &dashboardwidgets.BarChartXAxisValueModel{},
		}, nil
	default:
		return nil, diag.NewErrorDiagnostic("Error Flatten BarChart XAxis", fmt.Sprintf("unknown bar chart x axis type: %T", axis.GetType()))
	}

}

func flattenBarChartQuery(ctx context.Context, query *cxsdk.BarChartQuery) (*dashboardwidgets.BarChartQueryModel, diag.Diagnostics) {
	if query == nil {
		return nil, nil
	}

	switch queryType := query.GetValue().(type) {
	case *cxsdk.BarChartQueryLogs:
		return flattenBarChartQueryLogs(ctx, queryType.Logs)
	case *cxsdk.BarChartQuerySpans:
		return flattenBarChartQuerySpans(ctx, queryType.Spans)
	case *cxsdk.BarChartQueryMetrics:
		return flattenBarChartQueryMetrics(ctx, queryType.Metrics)
	case *cxsdk.BarChartQueryDataprime:
		return flattenBarChartQueryDataPrime(ctx, queryType.Dataprime)
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten BarChart Query", fmt.Sprintf("unknown bar chart query type: %T", query.GetValue()))}
	}
}

func flattenBarChartQueryLogs(ctx context.Context, logs *cxsdk.BarChartLogsQuery) (*dashboardwidgets.BarChartQueryModel, diag.Diagnostics) {
	if logs == nil {
		return nil, nil
	}

	filters, diags := dashboardwidgets.FlattenLogsFilters(ctx, logs.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	aggregation, diags := dashboardwidgets.FlattenLogsAggregation(ctx, logs.GetAggregation())
	if diags.HasError() {
		return nil, diags
	}

	groupNamesFields, diags := dashboardwidgets.FlattenObservationFields(ctx, logs.GetGroupNamesFields())
	if diags.HasError() {
		return nil, diags
	}

	stackedGroupNameField, diags := dashboardwidgets.FlattenObservationField(ctx, logs.GetStackedGroupNameField())
	if diags.HasError() {
		return nil, diags
	}

	timeFrame, diags := dashboardwidgets.FlattenTimeFrameSelect(ctx, logs.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	flattenedLogs := &dashboardwidgets.BarChartQueryLogsModel{
		LuceneQuery:           utils.WrapperspbStringToTypeString(logs.GetLuceneQuery().GetValue()),
		Filters:               filters,
		Aggregation:           aggregation,
		GroupNames:            utils.WrappedStringSliceToTypeStringList(logs.GetGroupNames()),
		StackedGroupName:      utils.WrapperspbStringToTypeString(logs.GetStackedGroupName()),
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

func flattenBarChartQuerySpans(ctx context.Context, spans *cxsdk.BarChartSpansQuery) (*dashboardwidgets.BarChartQueryModel, diag.Diagnostics) {
	if spans == nil {
		return nil, nil
	}

	filters, diags := dashboardwidgets.FlattenSpansFilters(ctx, spans.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	aggregation, dg := dashboardwidgets.FlattenSpansAggregation(spans.GetAggregation())
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	groupNames, diags := dashboardwidgets.FlattenSpansFields(ctx, spans.GetGroupNames())
	if diags.HasError() {
		return nil, diags
	}

	stackedGroupName, dg := dashboardwidgets.FlattenSpansField(spans.GetStackedGroupName())
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	timeFrame, diags := dashboardwidgets.FlattenTimeFrameSelect(ctx, spans.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	flattenedSpans := &dashboardwidgets.BarChartQuerySpansModel{
		LuceneQuery:      utils.WrapperspbStringToTypeString(spans.GetLuceneQuery().GetValue()),
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

func flattenBarChartQueryMetrics(ctx context.Context, metrics *cxsdk.BarChartMetricsQuery) (*dashboardwidgets.BarChartQueryModel, diag.Diagnostics) {
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
		PromqlQuery:      utils.WrapperspbStringToTypeString(metrics.GetPromqlQuery().GetValue()),
		Filters:          filters,
		GroupNames:       utils.WrappedStringSliceToTypeStringList(metrics.GetGroupNames()),
		StackedGroupName: utils.WrapperspbStringToTypeString(metrics.GetStackedGroupName()),
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

func flattenBarChartQueryDataPrime(ctx context.Context, dataPrime *cxsdk.BarChartDataprimeQuery) (*dashboardwidgets.BarChartQueryModel, diag.Diagnostics) {
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
		Query:            types.StringValue(dataPrime.GetDataprimeQuery().GetText()),
		Filters:          filters,
		GroupNames:       utils.WrappedStringSliceToTypeStringList(dataPrime.GetGroupNames()),
		StackedGroupName: utils.WrapperspbStringToTypeString(dataPrime.GetStackedGroupName()),
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

func flattenBarChartStackDefinition(stackDefinition *cxsdk.BarChartStackDefinition) *dashboardwidgets.BarChartStackDefinitionModel {
	if stackDefinition == nil {
		return nil
	}

	return &dashboardwidgets.BarChartStackDefinitionModel{
		MaxSlicesPerBar:   utils.WrapperspbInt32ToTypeInt64(stackDefinition.GetMaxSlicesPerBar()),
		StackNameTemplate: utils.WrapperspbStringToTypeString(stackDefinition.GetStackNameTemplate()),
	}
}

func flattenHorizontalBarChartStackDefinition(stackDefinition *cxsdk.HorizontalBarChartStackDefinition) *dashboardwidgets.BarChartStackDefinitionModel {
	if stackDefinition == nil {
		return nil
	}

	return &dashboardwidgets.BarChartStackDefinitionModel{
		MaxSlicesPerBar:   utils.WrapperspbInt32ToTypeInt64(stackDefinition.GetMaxSlicesPerBar()),
		StackNameTemplate: utils.WrapperspbStringToTypeString(stackDefinition.GetStackNameTemplate()),
	}
}

func flattenBarChartColorsBy(colorsBy *cxsdk.DashboardsColorsBy) (types.String, diag.Diagnostic) {
	if colorsBy == nil {
		return types.StringNull(), nil
	}
	switch colorsBy.GetValue().(type) {
	case *cxsdk.DashboardsColorsByGroupBy:
		return types.StringValue("group_by"), nil
	case *cxsdk.DashboardsColorsByStack:
		return types.StringValue("stack"), nil
	case *cxsdk.DashboardsColorsByAggregation:
		return types.StringValue("aggregation"), nil
	default:
		return types.StringNull(), diag.NewErrorDiagnostic("", fmt.Sprintf("unknown colors by type %T", colorsBy))
	}
}

func flattenDashboardVariables(ctx context.Context, variables []*cxsdk.DashboardVariable) (types.List, diag.Diagnostics) {
	if len(variables) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: dashboardsVariablesModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	variablesElements := make([]attr.Value, 0)
	for _, variable := range variables {
		flattenedVariable, diags := flattenDashboardVariable(ctx, variable)
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

func flattenDashboardVariable(ctx context.Context, variable *cxsdk.DashboardVariable) (*DashboardVariableModel, diag.Diagnostics) {
	if variable == nil {
		return nil, nil
	}

	definition, diags := flattenDashboardVariableDefinition(ctx, variable.GetDefinition())
	if diags.HasError() {
		return nil, diags
	}

	return &DashboardVariableModel{
		Name:        utils.WrapperspbStringToTypeString(variable.GetName()),
		DisplayName: utils.WrapperspbStringToTypeString(variable.GetDisplayName()),
		Definition:  definition,
	}, nil
}

func flattenDashboardVariableDefinition(ctx context.Context, variableDefinition *cxsdk.DashboardVariableDefinition) (*DashboardVariableDefinitionModel, diag.Diagnostics) {
	if variableDefinition == nil {
		return nil, nil
	}

	switch variableDefinition.GetValue().(type) {
	case *cxsdk.DashboardVariableDefinitionConstant:
		return &DashboardVariableDefinitionModel{
			ConstantValue: utils.WrapperspbStringToTypeString(variableDefinition.GetConstant().GetValue()),
		}, nil
	case *cxsdk.DashboardVariableDefinitionMultiSelect:
		return flattenDashboardVariableDefinitionMultiSelect(ctx, variableDefinition.GetMultiSelect())
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Dashboard Variable Definition", fmt.Sprintf("unknown variable definition type %T", variableDefinition))}
	}
}

func flattenDashboardVariableDefinitionMultiSelect(ctx context.Context, multiSelect *cxsdk.DashboardMultiSelect) (*DashboardVariableDefinitionModel, diag.Diagnostics) {
	if multiSelect == nil {
		return nil, nil
	}

	source, diags := flattenDashboardVariableSource(ctx, multiSelect.GetSource())
	if diags.HasError() {
		return nil, diags
	}

	selectedValues, diags := flattenDashboardVariableSelectedValues(multiSelect.GetSelection())
	if diags.HasError() {
		return nil, diags
	}

	return &DashboardVariableDefinitionModel{
		ConstantValue: types.StringNull(),
		MultiSelect: &VariableMultiSelectModel{
			SelectedValues:       selectedValues,
			ValuesOrderDirection: types.StringValue(dashboardwidgets.DashboardOrderDirectionProtoToSchema[multiSelect.GetValuesOrderDirection()]),
			Source:               source,
		},
	}, nil
}

func flattenDashboardVariableSource(ctx context.Context, source *cxsdk.MultiSelectSource) (*VariableMultiSelectSourceModel, diag.Diagnostics) {
	if source == nil {
		return nil, nil
	}

	result := &VariableMultiSelectSourceModel{
		LogsPath:     types.StringNull(),
		ConstantList: types.ListNull(types.StringType),
		Query:        types.ObjectNull(multiSelectQueryAttr()),
	}

	switch source.GetValue().(type) {
	case *cxsdk.MultiSelectSourceLogsPath:
		result.LogsPath = utils.WrapperspbStringToTypeString(source.GetLogsPath().GetValue())
	case *cxsdk.MultiSelectSourceMetricLabel:
		result.MetricLabel = &MetricMultiSelectSourceModel{
			MetricName: utils.WrapperspbStringToTypeString(source.GetMetricLabel().GetMetricName()),
			Label:      utils.WrapperspbStringToTypeString(source.GetMetricLabel().GetLabel()),
		}
	case *cxsdk.MultiSelectSourceConstantList:
		result.ConstantList = utils.WrappedStringSliceToTypeStringList(source.GetConstantList().GetValues())
	case *cxsdk.MultiSelectSourceSpanField:
		spansField, dg := dashboardwidgets.FlattenSpansField(source.GetSpanField().GetValue())
		if dg != nil {
			return nil, diag.Diagnostics{dg}
		}
		result.SpanField = spansField
	case *cxsdk.MultiSelectSourceQuery:
		query, diags := flattenDashboardVariableDefinitionMultiSelectQuery(ctx, source.GetQuery())
		if diags != nil {
			return nil, diags
		}
		result.Query = query
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Dashboard Variable Definition Multi Select Source", fmt.Sprintf("unknown variable definition multi select source type %T", source))}
	}

	return result, nil
}

func flattenDashboardVariableDefinitionMultiSelectQuery(ctx context.Context, querySource *cxsdk.MultiSelectQuerySource) (types.Object, diag.Diagnostics) {
	if querySource == nil {
		return types.ObjectNull(multiSelectQueryAttr()), nil
	}

	query, diags := flattenDashboardVariableDefinitionMultiSelectQueryModel(ctx, querySource.GetQuery())
	if diags.HasError() {
		return types.ObjectNull(multiSelectQueryAttr()), diags
	}

	valueDisplayOptions, diags := flattenDashboardVariableDefinitionMultiSelectValueDisplayOptions(ctx, querySource.GetValueDisplayOptions())
	if diags.HasError() {
		return types.ObjectNull(multiSelectQueryAttr()), diags
	}

	return types.ObjectValueFrom(ctx, multiSelectQueryAttr(), &VariableMultiSelectQueryModel{
		Query:               query,
		RefreshStrategy:     types.StringValue(dashboardwidgets.DashboardProtoToSchemaRefreshStrategy[querySource.GetRefreshStrategy()]),
		ValueDisplayOptions: valueDisplayOptions,
	})
}

func flattenDashboardVariableDefinitionMultiSelectQueryModel(ctx context.Context, query *cxsdk.MultiSelectQuery) (types.Object, diag.Diagnostics) {
	if query == nil {
		return types.ObjectNull(multiSelectQueryModelAttr()), nil
	}

	multiSelectQueryModel := &MultiSelectQueryModel{
		Logs:    types.ObjectNull(multiSelectQueryLogsQueryModelAttr()),
		Metrics: types.ObjectNull(multiSelectQueryMetricsQueryModelAttr()),
		Spans:   types.ObjectNull(multiSelectQuerySpansQueryModelAttr()),
	}
	var diags diag.Diagnostics
	switch queryType := query.GetValue().(type) {
	case *cxsdk.MultiSelectQueryLogsQuery:
		multiSelectQueryModel.Logs, diags = flattenDashboardVariableDefinitionMultiSelectQueryLogsModel(ctx, queryType.LogsQuery)
	case *cxsdk.MultiSelectQueryMetricsQuery:
		multiSelectQueryModel.Metrics, diags = flattenDashboardVariableDefinitionMultiSelectQueryMetricsModel(ctx, queryType.MetricsQuery)
	case *cxsdk.MultiSelectQuerySpansQuery:
		multiSelectQueryModel.Spans, diags = flattenDashboardVariableDefinitionMultiSelectQuerySpansModel(ctx, queryType.SpansQuery)
	}

	if diags.HasError() {
		return types.ObjectNull(multiSelectQueryModelAttr()), diags
	}

	return types.ObjectValueFrom(ctx, multiSelectQueryModelAttr(), multiSelectQueryModel)
}

func flattenDashboardVariableDefinitionMultiSelectQueryLogsModel(ctx context.Context, query *cxsdk.MultiSelectQueryLogsQueryInner) (types.Object, diag.Diagnostics) {
	if query == nil {
		return types.ObjectNull(multiSelectQueryLogsQueryModelAttr()), nil
	}

	logsQuery := &MultiSelectLogsQueryModel{
		FieldName:  types.ObjectNull(multiSelectQueryLogsQueryFieldNameModelAttr()),
		FieldValue: types.ObjectNull(multiSelectQueryLogsQueryFieldValueModelAttr()),
	}

	var diags diag.Diagnostics
	switch queryType := query.GetType().GetValue().(type) {
	case *cxsdk.MultiSelectQueryLogsQueryTypeFieldName:
		logsQuery.FieldName, diags = flattenDashboardVariableDefinitionMultiSelectQueryLogsFieldNameModel(ctx, queryType.FieldName)
	case *cxsdk.MultiSelectQueryLogsQueryTypeFieldValue:
		logsQuery.FieldValue, diags = flattenDashboardVariableDefinitionMultiSelectQueryLogsFieldValueModel(ctx, queryType.FieldValue)
	}

	if diags.HasError() {
		return types.ObjectNull(multiSelectQueryLogsQueryModelAttr()), diags
	}

	return types.ObjectValueFrom(ctx, multiSelectQueryLogsQueryModelAttr(), logsQuery)
}

func flattenDashboardVariableDefinitionMultiSelectQueryLogsFieldNameModel(ctx context.Context, name *cxsdk.MultiSelectQueryLogsQueryTypeFieldNameInner) (types.Object, diag.Diagnostics) {
	if name == nil {
		return types.ObjectNull(multiSelectQueryLogsQueryFieldNameModelAttr()), nil
	}

	return types.ObjectValueFrom(ctx, multiSelectQueryLogsQueryFieldNameModelAttr(), &LogFieldNameModel{
		LogRegex: utils.WrapperspbStringToTypeString(name.GetLogRegex()),
	})
}

func flattenDashboardVariableDefinitionMultiSelectQueryLogsFieldValueModel(ctx context.Context, value *cxsdk.MultiSelectQueryLogsQueryTypeFieldValueInner) (types.Object, diag.Diagnostics) {
	if value == nil {
		return types.ObjectNull(multiSelectQueryLogsQueryFieldValueModelAttr()), nil
	}

	observationField, diags := dashboardwidgets.FlattenObservationField(ctx, value.GetObservationField())
	if diags.HasError() {
		return types.ObjectNull(multiSelectQueryLogsQueryFieldValueModelAttr()), diags
	}

	return types.ObjectValueFrom(ctx, multiSelectQueryLogsQueryFieldValueModelAttr(), &FieldValueModel{
		ObservationField: observationField,
	})
}

func flattenDashboardVariableDefinitionMultiSelectQueryMetricsModel(ctx context.Context, query *cxsdk.MultiSelectQueryMetricsQueryInner) (types.Object, diag.Diagnostics) {
	if query == nil {
		return types.ObjectNull(multiSelectQueryMetricsQueryModelAttr()), nil
	}

	var diags diag.Diagnostics
	metricQuery := &MultiSelectMetricsQueryModel{
		MetricName: types.ObjectNull(multiSelectQueryMetricsNameAttr()),
		LabelName:  types.ObjectNull(multiSelectQueryMetricsNameAttr()),
		LabelValue: types.ObjectNull(multiSelectQueryLabelValueModelAttr()),
	}

	switch queryType := query.GetType().GetValue().(type) {
	case *cxsdk.MultiSelectQueryMetricsQueryTypeMetricName:
		metricQuery.MetricName, diags = flattenDashboardVariableDefinitionMultiSelectQueryMetricsMetricNameModel(ctx, queryType.MetricName)
	case *cxsdk.MultiSelectQueryMetricsQueryTypeLabelName:
		metricQuery.LabelName, diags = flattenDashboardVariableDefinitionMultiSelectQueryMetricsLabelNameModel(ctx, queryType.LabelName)
	case *cxsdk.MultiSelectQueryMetricsQueryTypeLabelValue:
		metricQuery.LabelValue, diags = flattenDashboardVariableDefinitionMultiSelectQueryMetricsLabelValueModel(ctx, queryType.LabelValue)
	}

	if diags.HasError() {
		return types.ObjectNull(multiSelectQueryMetricsQueryModelAttr()), diags
	}

	return types.ObjectValueFrom(ctx, multiSelectQueryMetricsQueryModelAttr(), metricQuery)
}

func flattenDashboardVariableDefinitionMultiSelectQueryMetricsMetricNameModel(ctx context.Context, name *cxsdk.MultiSelectQueryMetricsQueryTypeMetricNameInner) (types.Object, diag.Diagnostics) {
	if name == nil {
		return types.ObjectNull(multiSelectQueryMetricsNameAttr()), nil
	}

	return types.ObjectValueFrom(ctx, multiSelectQueryMetricsNameAttr(), &MetricAndLabelNameModel{
		MetricRegex: utils.WrapperspbStringToTypeString(name.GetMetricRegex()),
	})
}

func flattenDashboardVariableDefinitionMultiSelectQueryMetricsLabelNameModel(ctx context.Context, name *cxsdk.MultiSelectQueryMetricsQueryTypeLabelNameInner) (types.Object, diag.Diagnostics) {
	if name == nil {
		return types.ObjectNull(multiSelectQueryMetricsNameAttr()), nil
	}

	return types.ObjectValueFrom(ctx, multiSelectQueryMetricsNameAttr(), &MetricAndLabelNameModel{
		MetricRegex: utils.WrapperspbStringToTypeString(name.GetMetricRegex()),
	})
}

func flattenDashboardVariableDefinitionMultiSelectQueryMetricsLabelValueModel(ctx context.Context, value *cxsdk.MultiSelectQueryMetricsQueryTypeLabelValueInner) (types.Object, diag.Diagnostics) {
	if value == nil {
		return types.ObjectNull(multiSelectQueryLabelValueModelAttr()), nil
	}

	metricName, diags := flattenMultiSelectQueryMetricsQueryStringOrVariable(ctx, value.GetMetricName())
	if diags.HasError() {
		return types.ObjectNull(multiSelectQueryLabelValueModelAttr()), diags
	}

	labelName, diags := flattenMultiSelectQueryMetricsQueryStringOrVariable(ctx, value.GetLabelName())
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

func flattenMultiSelectQueryMetricsQueryMetricsLabelFilters(ctx context.Context, filters []*cxsdk.MultiSelectQueryMetricsQueryMetricsLabelFilter) (types.List, diag.Diagnostics) {
	var diagnostics diag.Diagnostics
	flattenedFilters := make([]attr.Value, 0)
	for _, filter := range filters {
		flattenedFilter, diags := flattenMultiSelectQueryMetricsQueryMetricsLabelFilter(ctx, filter)
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

func flattenMultiSelectQueryMetricsQueryMetricsLabelFilter(ctx context.Context, filter *cxsdk.MultiSelectQueryMetricsQueryMetricsLabelFilter) (*MetricLabelFilterModel, diag.Diagnostics) {
	if filter == nil {
		return nil, nil
	}

	metric, diags := flattenMultiSelectQueryMetricsQueryStringOrVariable(ctx, filter.GetMetric())
	if diags.HasError() {
		return nil, diags
	}

	label, diags := flattenMultiSelectQueryMetricsQueryStringOrVariable(ctx, filter.GetLabel())
	if diags.HasError() {
		return nil, diags
	}

	operator, diags := flattenMultiSelectQueryMetricsQueryMetricsLabelFilterOperator(ctx, filter.GetOperator())
	if diags.HasError() {
		return nil, diags
	}

	return &MetricLabelFilterModel{
		Metric:   metric,
		Label:    label,
		Operator: operator,
	}, nil
}

func flattenMultiSelectQueryMetricsQueryMetricsLabelFilterOperator(ctx context.Context, operator *cxsdk.MultiSelectQueryMetricsQueryOperator) (types.Object, diag.Diagnostics) {
	if operator == nil {
		return types.ObjectNull(multiSelectQueryMetricsQueryMetricsLabelFilterOperatorAttr()), nil
	}

	var diags diag.Diagnostics
	metricLabelFilterOperatorModel := &MetricLabelFilterOperatorModel{}
	switch operatorType := operator.GetValue().(type) {
	case *cxsdk.MultiSelectQueryMetricsQueryOperatorEquals:
		metricLabelFilterOperatorModel.Type = types.StringValue("equals")
		metricLabelFilterOperatorModel.SelectedValues, diags = flattenMultiSelectQueryMetricsQueryOperatorSelectedValues(ctx, operatorType.Equals.GetSelection().GetList().GetValues())
	case *cxsdk.MultiSelectQueryMetricsQueryOperatorNotEquals:
		metricLabelFilterOperatorModel.Type = types.StringValue("not_equals")
		metricLabelFilterOperatorModel.SelectedValues, diags = flattenMultiSelectQueryMetricsQueryOperatorSelectedValues(ctx, operatorType.NotEquals.GetSelection().GetList().GetValues())
	}

	if diags.HasError() {
		return types.ObjectNull(multiSelectQueryMetricsQueryMetricsLabelFilterOperatorAttr()), diags
	}
	return types.ObjectValueFrom(ctx, multiSelectQueryMetricsQueryMetricsLabelFilterOperatorAttr(), metricLabelFilterOperatorModel)
}

func flattenMultiSelectQueryMetricsQueryOperatorSelectedValues(ctx context.Context, values []*cxsdk.MultiSelectQueryMetricsQueryStringOrVariable) (types.List, diag.Diagnostics) {
	var diagnostics diag.Diagnostics
	flattenedValues := make([]types.Object, 0)
	for _, value := range values {
		flattenedValue, diags := flattenMultiSelectQueryMetricsQueryStringOrVariable(ctx, value)
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

func flattenMultiSelectQueryMetricsQueryStringOrVariable(ctx context.Context, stringOrVariable *cxsdk.MultiSelectQueryMetricsQueryStringOrVariable) (types.Object, diag.Diagnostics) {
	if stringOrVariable == nil {
		return types.ObjectNull(multiSelectQueryStringOrValueAttr()), nil
	}

	metricLabelFilterOperatorSelectedValuesModel := &MetricLabelFilterOperatorSelectedValuesModel{
		StringValue:  types.StringNull(),
		VariableName: types.StringNull(),
	}

	switch stringOrVariableType := stringOrVariable.GetValue().(type) {
	case *cxsdk.MultiSelectQueryMetricsQueryStringOrVariableString:
		metricLabelFilterOperatorSelectedValuesModel.StringValue = utils.WrapperspbStringToTypeString(stringOrVariableType.StringValue)
	case *cxsdk.MultiSelectQueryMetricsQueryStringOrVariableVariable:
		metricLabelFilterOperatorSelectedValuesModel.VariableName = utils.WrapperspbStringToTypeString(stringOrVariableType.VariableName)
	}

	return types.ObjectValueFrom(ctx, multiSelectQueryStringOrValueAttr(), metricLabelFilterOperatorSelectedValuesModel)
}

func flattenDashboardVariableDefinitionMultiSelectQuerySpansModel(ctx context.Context, query *cxsdk.MultiSelectQuerySpansQueryInner) (types.Object, diag.Diagnostics) {
	if query == nil {
		return types.ObjectNull(multiSelectQuerySpansQueryModelAttr()), nil
	}

	var diags diag.Diagnostics
	multiSelectSpansQueryModel := &MultiSelectSpansQueryModel{
		FieldName:  types.ObjectNull(spansQueryFieldNameAttr()),
		FieldValue: types.ObjectNull(dashboardwidgets.SpansFieldModelAttr()),
	}
	switch queryType := query.GetType().GetValue().(type) {
	case *cxsdk.MultiSelectQuerySpansQueryTypeFieldName:
		multiSelectSpansQueryModel.FieldName, diags = flattenMultiSelectQuerySpansFieldName(ctx, queryType.FieldName)
	case *cxsdk.MultiSelectQuerySpansQueryTypeFieldValue:
		multiSelectSpansQueryModel.FieldValue, diags = flattenMultiSelectQuerySpansFieldValue(ctx, queryType.FieldValue)
	default:
		return types.ObjectNull(multiSelectQuerySpansQueryModelAttr()), diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Dashboard Variable Definition Multi Select Query Spans Model", fmt.Sprintf("unknown variable definition multi select query spans type %T", queryType))}
	}

	if diags.HasError() {
		return types.ObjectNull(multiSelectQuerySpansQueryModelAttr()), diags
	}

	return types.ObjectValueFrom(ctx, multiSelectQuerySpansQueryModelAttr(), multiSelectSpansQueryModel)
}

func flattenMultiSelectQuerySpansFieldName(ctx context.Context, name *cxsdk.MultiSelectQuerySpansQueryTypeFieldNameInner) (types.Object, diag.Diagnostics) {
	if name == nil {
		return types.ObjectNull(multiSelectQuerySpansQueryModelAttr()), nil
	}

	return types.ObjectValueFrom(ctx, multiSelectQuerySpansQueryModelAttr(), &SpanFieldNameModel{
		SpanRegex: utils.WrapperspbStringToTypeString(name.GetSpanRegex()),
	})
}

func flattenMultiSelectQuerySpansFieldValue(ctx context.Context, value *cxsdk.MultiSelectQuerySpansQueryTypeFieldValueInner) (types.Object, diag.Diagnostics) {
	if value == nil || value.GetValue() == nil {
		return types.ObjectNull(dashboardwidgets.SpansFieldModelAttr()), nil
	}

	spanField, dg := dashboardwidgets.FlattenSpansField(value.GetValue())
	if dg != nil {
		return types.ObjectNull(dashboardwidgets.SpansFieldModelAttr()), diag.Diagnostics{dg}
	}

	return types.ObjectValueFrom(ctx, dashboardwidgets.SpansFieldModelAttr(), spanField)
}

func flattenDashboardVariableDefinitionMultiSelectValueDisplayOptions(ctx context.Context, options *cxsdk.MultiSelectValueDisplayOptions) (types.Object, diag.Diagnostics) {
	if options == nil {
		return types.ObjectNull(multiSelectValueDisplayOptionsModelAttr()), nil
	}

	return types.ObjectValueFrom(ctx, multiSelectValueDisplayOptionsModelAttr(), &MultiSelectValueDisplayOptionsModel{
		ValueRegex: utils.WrapperspbStringToTypeString(options.GetValueRegex()),
		LabelRegex: utils.WrapperspbStringToTypeString(options.GetLabelRegex()),
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

func flattenDashboardVariableSelectedValues(selection *cxsdk.DashboardMultiSelectSelection) (types.List, diag.Diagnostics) {
	switch selection.GetValue().(type) {
	case *cxsdk.DashboardMultiSelectSelectionList:
		return utils.WrappedStringSliceToTypeStringList(selection.GetList().GetValues()), nil
	case *cxsdk.DashboardMultiSelectSelectionAll:
		return types.ListNull(types.StringType), nil
	default:
		return types.ListNull(types.StringType), diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Dashboard Variable Definition Multi Select Selection", fmt.Sprintf("unknown variable definition multi select selection type %T", selection))}
	}
}

func flattenDashboardFilters(ctx context.Context, filters []*cxsdk.DashboardFilter) (types.List, diag.Diagnostics) {
	if len(filters) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: dashboardsFiltersModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	filtersElements := make([]attr.Value, 0)
	for _, filter := range filters {
		flattenedFilter, dgs := flattenDashboardFilter(ctx, filter)
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

func flattenDashboardFilter(ctx context.Context, filter *cxsdk.DashboardFilter) (*DashboardFilterModel, diag.Diagnostics) {
	if filter == nil {
		return nil, nil
	}

	source, diags := dashboardwidgets.FlattenDashboardFilterSource(ctx, filter.GetSource())
	if diags != nil {
		return nil, diags
	}

	return &DashboardFilterModel{
		Source:    source,
		Enabled:   utils.WrapperspbBoolToTypeBool(filter.GetEnabled()),
		Collapsed: utils.WrapperspbBoolToTypeBool(filter.GetCollapsed()),
	}, nil
}

func flattenDashboardFolder(ctx context.Context, planedDashboard types.Object, dashboard *cxsdk.Dashboard) (types.Object, diag.Diagnostics) {
	if dashboard.FolderId != nil {
		path := types.StringNull()
		if !utils.ObjIsNullOrUnknown(planedDashboard) {
			var folderModel DashboardFolderModel
			dgs := planedDashboard.As(context.Background(), &folderModel, basetypes.ObjectAsOptions{})
			if dgs.HasError() {
				return types.ObjectNull(dashboardFolderModelAttr()), dgs
			}
			if !(folderModel.Path.IsUnknown() || folderModel.Path.IsNull()) {
				path = folderModel.Path
			}
		}

		folderObject := &DashboardFolderModel{
			ID:   types.StringValue(dashboard.FolderId.GetValue()),
			Path: path,
		}
		return types.ObjectValueFrom(ctx, dashboardFolderModelAttr(), folderObject)
	} else if dashboard.FolderPath != nil {
		folderObject := &DashboardFolderModel{
			ID:   types.StringNull(),
			Path: types.StringValue(strings.Join(dashboard.FolderPath.GetSegments(), "/")),
		}
		return types.ObjectValueFrom(ctx, dashboardFolderModelAttr(), folderObject)
	} else {
		return types.ObjectNull(dashboardFolderModelAttr()), nil
	}
}

func flattenDashboardAnnotations(ctx context.Context, annotations []*cxsdk.Annotation) (types.List, diag.Diagnostics) {
	if len(annotations) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: dashboardsAnnotationsModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	annotationsElements := make([]attr.Value, 0)
	for _, annotation := range annotations {
		flattenedAnnotation, diags := flattenDashboardAnnotation(ctx, annotation)
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

func flattenDashboardAnnotation(ctx context.Context, annotation *cxsdk.Annotation) (*DashboardAnnotationModel, diag.Diagnostics) {
	if annotation == nil {
		return nil, nil
	}

	source, diags := flattenDashboardAnnotationSource(ctx, annotation.GetSource())
	if diags.HasError() {
		return nil, diags
	}

	return &DashboardAnnotationModel{
		ID:      utils.WrapperspbStringToTypeString(annotation.GetId()),
		Name:    utils.WrapperspbStringToTypeString(annotation.GetName()),
		Enabled: utils.WrapperspbBoolToTypeBool(annotation.GetEnabled()),
		Source:  source,
	}, nil
}

func flattenDashboardAnnotationSource(ctx context.Context, source *cxsdk.AnnotationSource) (types.Object, diag.Diagnostics) {
	if source == nil {
		return types.ObjectNull(dashboardsAnnotationsModelAttr()), nil
	}

	var sourceObject DashboardAnnotationSourceModel
	var diags diag.Diagnostics
	switch source.Value.(type) {
	case *cxsdk.AnnotationSourceMetrics:
		sourceObject.Metrics, diags = flattenDashboardAnnotationMetricSourceModel(ctx, source.GetMetrics())
		sourceObject.Logs = types.ObjectNull(annotationsLogsAndSpansSourceModelAttr())
		sourceObject.Spans = types.ObjectNull(annotationsLogsAndSpansSourceModelAttr())
	case *cxsdk.AnnotationSourceLogs:
		sourceObject.Logs, diags = flattenDashboardAnnotationLogsSourceModel(ctx, source.GetLogs())
		sourceObject.Metrics = types.ObjectNull(annotationsMetricsSourceModelAttr())
		sourceObject.Spans = types.ObjectNull(annotationsLogsAndSpansSourceModelAttr())
	case *cxsdk.AnnotationSourceSpans:
		sourceObject.Spans, diags = flattenDashboardAnnotationSpansSourceModel(ctx, source.GetSpans())
		sourceObject.Metrics = types.ObjectNull(annotationsMetricsSourceModelAttr())
		sourceObject.Logs = types.ObjectNull(annotationsLogsAndSpansSourceModelAttr())
	default:
		diags = diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Dashboard Annotation Source", fmt.Sprintf("unknown annotation source type %T", source.Value))}
	}

	if diags.HasError() {
		return types.ObjectNull(annotationSourceModelAttr()), diags
	}

	return types.ObjectValueFrom(ctx, annotationSourceModelAttr(), sourceObject)
}

func flattenDashboardAnnotationSpansSourceModel(ctx context.Context, spans *cxsdk.AnnotationSpansSource) (types.Object, diag.Diagnostics) {
	if spans == nil {
		return types.ObjectNull(annotationsLogsAndSpansSourceModelAttr()), nil
	}

	strategy, diags := flattenAnnotationSpansStrategy(ctx, spans.GetStrategy())
	if diags.HasError() {
		return types.ObjectNull(annotationsLogsAndSpansSourceModelAttr()), diags
	}

	labelFields, diags := dashboardwidgets.FlattenObservationFields(ctx, spans.GetLabelFields())
	if diags.HasError() {
		return types.ObjectNull(annotationsLogsAndSpansSourceModelAttr()), diags
	}

	spansObject := &DashboardAnnotationSpansOrLogsSourceModel{
		LuceneQuery:     utils.WrapperspbStringToTypeString(spans.GetLuceneQuery().GetValue()),
		Strategy:        strategy,
		MessageTemplate: utils.WrapperspbStringToTypeString(spans.GetMessageTemplate()),
		LabelFields:     labelFields,
	}

	return types.ObjectValueFrom(ctx, annotationsLogsAndSpansSourceModelAttr(), spansObject)
}

func flattenAnnotationSpansStrategy(ctx context.Context, strategy *cxsdk.AnnotationSpansSourceStrategy) (types.Object, diag.Diagnostics) {
	if strategy == nil {
		return types.ObjectNull(logsAndSpansStrategyModelAttr()), nil
	}

	var strategyModel DashboardAnnotationSpanOrLogsStrategyModel
	var diags diag.Diagnostics
	switch strategy.Value.(type) {
	case *cxsdk.AnnotationSpansSourceStrategyInstant:
		strategyModel.Instant, diags = flattenSpansStrategyInstant(ctx, strategy.GetInstant())
		strategyModel.Range = types.ObjectNull(rangeStrategyModelAttr())
		strategyModel.Duration = types.ObjectNull(durationStrategyModelAttr())
	case *cxsdk.AnnotationSpansSourceStrategyRange:
		strategyModel.Range, diags = flattenSpansStrategyRange(ctx, strategy.GetRange())
		strategyModel.Instant = types.ObjectNull(instantStrategyModelAttr())
		strategyModel.Duration = types.ObjectNull(durationStrategyModelAttr())
	case *cxsdk.AnnotationSpansSourceStrategyDuration:
		strategyModel.Duration, diags = flattenSpansStrategyDuration(ctx, strategy.GetDuration())
		strategyModel.Instant = types.ObjectNull(instantStrategyModelAttr())
		strategyModel.Range = types.ObjectNull(rangeStrategyModelAttr())
	default:
		diags = diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Annotation Spans Strategy", fmt.Sprintf("unknown annotation spans strategy type %T", strategy.Value))}
	}

	if diags.HasError() {
		return types.ObjectNull(logsAndSpansStrategyModelAttr()), diags
	}

	return types.ObjectValueFrom(ctx, logsAndSpansStrategyModelAttr(), strategyModel)
}

func flattenSpansStrategyDuration(ctx context.Context, duration *cxsdk.AnnotationSpansSourceStrategyDurationInner) (types.Object, diag.Diagnostics) {
	if duration == nil {
		return types.ObjectNull(durationStrategyModelAttr()), nil
	}

	startTimestampField, diags := dashboardwidgets.FlattenObservationField(ctx, duration.GetStartTimestampField())
	if diags.HasError() {
		return types.ObjectNull(durationStrategyModelAttr()), diags
	}

	endTimestampField, diags := dashboardwidgets.FlattenObservationField(ctx, duration.GetDurationField())
	if diags.HasError() {
		return types.ObjectNull(durationStrategyModelAttr()), diags
	}

	durationStrategy := &DashboardAnnotationDurationStrategyModel{
		StartTimestampField: startTimestampField,
		DurationField:       endTimestampField,
	}

	return types.ObjectValueFrom(ctx, durationStrategyModelAttr(), durationStrategy)
}

func flattenSpansStrategyRange(ctx context.Context, getRange *cxsdk.AnnotationSpansSourceStrategyRangeInner) (types.Object, diag.Diagnostics) {
	if getRange == nil {
		return types.ObjectNull(rangeStrategyModelAttr()), nil
	}

	startTimestampField, diags := dashboardwidgets.FlattenObservationField(ctx, getRange.GetStartTimestampField())
	if diags.HasError() {
		return types.ObjectNull(rangeStrategyModelAttr()), diags
	}

	endTimestampField, diags := dashboardwidgets.FlattenObservationField(ctx, getRange.GetEndTimestampField())
	if diags.HasError() {
		return types.ObjectNull(rangeStrategyModelAttr()), diags
	}

	rangeStrategy := &DashboardAnnotationRangeStrategyModel{
		StartTimestampField: startTimestampField,
		EndTimestampField:   endTimestampField,
	}

	return types.ObjectValueFrom(ctx, rangeStrategyModelAttr(), rangeStrategy)
}

func flattenSpansStrategyInstant(ctx context.Context, instant *cxsdk.AnnotationSpansSourceStrategyInstantInner) (types.Object, diag.Diagnostics) {
	if instant == nil {
		return types.ObjectNull(instantStrategyModelAttr()), nil
	}

	timestampField, diags := dashboardwidgets.FlattenObservationField(ctx, instant.GetTimestampField())
	if diags.HasError() {
		return types.ObjectNull(instantStrategyModelAttr()), diags
	}

	instantStrategy := &DashboardAnnotationInstantStrategyModel{
		TimestampField: timestampField,
	}

	return types.ObjectValueFrom(ctx, instantStrategyModelAttr(), instantStrategy)
}

func flattenLogsStrategyDuration(ctx context.Context, duration *cxsdk.AnnotationLogsSourceStrategyDurationInner) (types.Object, diag.Diagnostics) {
	if duration == nil {
		return types.ObjectNull(durationStrategyModelAttr()), nil
	}

	startTimestampField, diags := dashboardwidgets.FlattenObservationField(ctx, duration.GetStartTimestampField())
	if diags.HasError() {
		return types.ObjectNull(durationStrategyModelAttr()), diags
	}

	endTimestampField, diags := dashboardwidgets.FlattenObservationField(ctx, duration.GetDurationField())
	if diags.HasError() {
		return types.ObjectNull(durationStrategyModelAttr()), diags
	}

	durationStrategy := &DashboardAnnotationDurationStrategyModel{
		StartTimestampField: startTimestampField,
		DurationField:       endTimestampField,
	}

	return types.ObjectValueFrom(ctx, durationStrategyModelAttr(), durationStrategy)
}

func flattenLogsStrategyRange(ctx context.Context, getRange *cxsdk.AnnotationLogsSourceStrategyRangeInner) (types.Object, diag.Diagnostics) {
	if getRange == nil {
		return types.ObjectNull(rangeStrategyModelAttr()), nil
	}

	startTimestampField, diags := dashboardwidgets.FlattenObservationField(ctx, getRange.GetStartTimestampField())
	if diags.HasError() {
		return types.ObjectNull(rangeStrategyModelAttr()), diags
	}

	endTimestampField, diags := dashboardwidgets.FlattenObservationField(ctx, getRange.GetEndTimestampField())
	if diags.HasError() {
		return types.ObjectNull(rangeStrategyModelAttr()), diags
	}

	rangeStrategy := &DashboardAnnotationRangeStrategyModel{
		StartTimestampField: startTimestampField,
		EndTimestampField:   endTimestampField,
	}

	return types.ObjectValueFrom(ctx, rangeStrategyModelAttr(), rangeStrategy)
}

func flattenLogsStrategyInstant(ctx context.Context, instant *cxsdk.AnnotationLogsSourceStrategyInstantInner) (types.Object, diag.Diagnostics) {
	if instant == nil {
		return types.ObjectNull(instantStrategyModelAttr()), nil
	}

	timestampField, diags := dashboardwidgets.FlattenObservationField(ctx, instant.GetTimestampField())
	if diags.HasError() {
		return types.ObjectNull(instantStrategyModelAttr()), diags
	}

	instantStrategy := &DashboardAnnotationInstantStrategyModel{
		TimestampField: timestampField,
	}

	return types.ObjectValueFrom(ctx, instantStrategyModelAttr(), instantStrategy)
}

func flattenDashboardAnnotationLogsSourceModel(ctx context.Context, logs *cxsdk.AnnotationLogsSource) (types.Object, diag.Diagnostics) {
	if logs == nil {
		return types.ObjectNull(annotationsLogsAndSpansSourceModelAttr()), nil
	}

	strategy, diags := flattenAnnotationLogsStrategy(ctx, logs.GetStrategy())
	if diags.HasError() {
		return types.ObjectNull(annotationsLogsAndSpansSourceModelAttr()), diags
	}

	labelFields, diags := dashboardwidgets.FlattenObservationFields(ctx, logs.GetLabelFields())
	if diags.HasError() {
		return types.ObjectNull(annotationsLogsAndSpansSourceModelAttr()), diags
	}

	logsObject := &DashboardAnnotationSpansOrLogsSourceModel{
		LuceneQuery:     utils.WrapperspbStringToTypeString(logs.GetLuceneQuery().GetValue()),
		Strategy:        strategy,
		MessageTemplate: utils.WrapperspbStringToTypeString(logs.GetMessageTemplate()),
		LabelFields:     labelFields,
	}

	return types.ObjectValueFrom(ctx, annotationsLogsAndSpansSourceModelAttr(), logsObject)
}

func flattenAnnotationLogsStrategy(ctx context.Context, strategy *cxsdk.AnnotationLogsSourceStrategy) (types.Object, diag.Diagnostics) {
	if strategy == nil {
		return types.ObjectNull(logsAndSpansStrategyModelAttr()), nil
	}

	var strategyModel DashboardAnnotationSpanOrLogsStrategyModel
	var diags diag.Diagnostics
	switch strategy.Value.(type) {
	case *cxsdk.AnnotationLogsSourceStrategyInstant:
		strategyModel.Instant, diags = flattenLogsStrategyInstant(ctx, strategy.GetInstant())
		strategyModel.Range = types.ObjectNull(rangeStrategyModelAttr())
		strategyModel.Duration = types.ObjectNull(durationStrategyModelAttr())
	case *cxsdk.AnnotationLogsSourceStrategyRange:
		strategyModel.Range, diags = flattenLogsStrategyRange(ctx, strategy.GetRange())
		strategyModel.Instant = types.ObjectNull(instantStrategyModelAttr())
		strategyModel.Duration = types.ObjectNull(durationStrategyModelAttr())
	case *cxsdk.AnnotationLogsSourceStrategyDuration:
		strategyModel.Duration, diags = flattenLogsStrategyDuration(ctx, strategy.GetDuration())
		strategyModel.Instant = types.ObjectNull(instantStrategyModelAttr())
		strategyModel.Range = types.ObjectNull(rangeStrategyModelAttr())
	default:
		diags = diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Annotation Logs Strategy", fmt.Sprintf("unknown annotation logs strategy type %T", strategy.Value))}
	}

	if diags.HasError() {
		return types.ObjectNull(logsAndSpansStrategyModelAttr()), diags
	}

	return types.ObjectValueFrom(ctx, logsAndSpansStrategyModelAttr(), strategyModel)
}

func flattenDashboardAnnotationMetricSourceModel(ctx context.Context, metricSource *cxsdk.AnnotationMetricsSource) (types.Object, diag.Diagnostics) {
	if metricSource == nil {
		return types.ObjectNull(annotationsMetricsSourceModelAttr()), nil
	}

	strategy, diags := flattenDashboardAnnotationStrategy(ctx, metricSource.GetStrategy())
	if diags.HasError() {
		return types.ObjectNull(annotationsMetricsSourceModelAttr()), diags
	}

	metricSourceObject := &DashboardAnnotationMetricSourceModel{
		PromqlQuery:     utils.WrapperspbStringToTypeString(metricSource.GetPromqlQuery().GetValue()),
		Strategy:        strategy,
		MessageTemplate: utils.WrapperspbStringToTypeString(metricSource.GetMessageTemplate()),
		Labels:          utils.WrappedStringSliceToTypeStringList(metricSource.GetLabels()),
	}

	return types.ObjectValueFrom(ctx, annotationsMetricsSourceModelAttr(), metricSourceObject)
}

func flattenDashboardAnnotationStrategy(ctx context.Context, strategy *cxsdk.AnnotationMetricsSourceStrategy) (types.Object, diag.Diagnostics) {
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

func flattenDashboardAutoRefresh(ctx context.Context, dashboard *cxsdk.Dashboard) (types.Object, diag.Diagnostics) {
	autoRefresh := dashboard.GetAutoRefresh()
	if autoRefresh == nil {
		return types.ObjectNull(dashboardAutoRefreshModelAttr()), nil
	}

	var refreshType DashboardAutoRefreshModel
	switch autoRefresh.(type) {
	case *cxsdk.DashboardOff:
		refreshType.Type = types.StringValue("off")
	case *cxsdk.DashboardFiveMinutes:
		refreshType.Type = types.StringValue("five_minutes")
	case *cxsdk.DashboardTwoMinutes:
		refreshType.Type = types.StringValue("two_minutes")
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
	getDashboardReq := &cxsdk.GetDashboardRequest{DashboardId: wrapperspb.String(id)}
	getDashboardResp, err := r.client.Get(ctx, getDashboardReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if cxsdk.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("Dashboard %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError(
				"Error reading Dashboard",
				utils.FormatRpcErrors(err, cxsdk.GetDashboardRPC, protojson.Format(getDashboardReq)),
			)
		}
		return
	}
	log.Printf("[INFO] Received Dashboard: %s", protojson.Format(getDashboardResp))

	flattenedDashboard, diags := flattenDashboard(ctx, state, getDashboardResp.GetDashboard())
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

	dashboard, diags := extractDashboard(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	updateReq := &cxsdk.ReplaceDashboardRequest{Dashboard: dashboard}
	reqStr := protojson.Format(updateReq)
	log.Printf("[INFO] Updating Dashboard: %s", reqStr)
	_, err := r.client.Replace(ctx, updateReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error updating Dashboard",
			utils.FormatRpcErrors(err, cxsdk.ReplaceDashboardRPC, reqStr),
		)
		return
	}

	getDashboardReq := &cxsdk.GetDashboardRequest{
		DashboardId: dashboard.GetId(),
	}
	getDashboardResp, err := r.client.Get(ctx, getDashboardReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error getting Dashboard",
			utils.FormatRpcErrors(err, cxsdk.GetDashboardRPC, protojson.Format(getDashboardReq)),
		)
		return
	}

	updateDashboardRespStr := protojson.Format(getDashboardResp.GetDashboard())
	log.Printf("[INFO] Submitted updated Dashboard: %s", updateDashboardRespStr)

	flattenedDashboard, diags := flattenDashboard(ctx, plan, getDashboardResp.GetDashboard())
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
	deleteReq := &cxsdk.DeleteDashboardRequest{DashboardId: wrapperspb.String(id)}
	if _, err := r.client.Delete(ctx, deleteReq); err != nil {
		resp.Diagnostics.AddError(
			fmt.Sprintf("Error Deleting Dashboard %s", id),
			utils.FormatRpcErrors(err, cxsdk.DeleteDashboardRPC, protojson.Format(deleteReq)),
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

	r.client = clientSet.Dashboards()
}
