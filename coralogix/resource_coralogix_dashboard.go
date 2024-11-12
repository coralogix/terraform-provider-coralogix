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

package coralogix

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"terraform-provider-coralogix/coralogix/clientset"
	dashboards "terraform-provider-coralogix/coralogix/clientset/grpc/dashboards"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/float64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/float64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/nsf/jsondiff"
	"golang.org/x/exp/slices"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

var (
	dashboardLegendPlacementSchemaToProto = map[string]dashboards.Legend_LegendPlacement{
		"unspecified": dashboards.Legend_LEGEND_PLACEMENT_UNSPECIFIED,
		"auto":        dashboards.Legend_LEGEND_PLACEMENT_AUTO,
		"bottom":      dashboards.Legend_LEGEND_PLACEMENT_BOTTOM,
		"side":        dashboards.Legend_LEGEND_PLACEMENT_SIDE,
		"hidden":      dashboards.Legend_LEGEND_PLACEMENT_HIDDEN,
	}
	dashboardLegendPlacementProtoToSchema = ReverseMap(dashboardLegendPlacementSchemaToProto)
	dashboardValidLegendPlacements        = GetKeys(dashboardLegendPlacementSchemaToProto)
	dashboardRowStyleSchemaToProto        = map[string]dashboards.RowStyle{
		//"unspecified": dashboards.RowStyle_ROW_STYLE_UNSPECIFIED,
		"one_line":  dashboards.RowStyle_ROW_STYLE_ONE_LINE,
		"two_line":  dashboards.RowStyle_ROW_STYLE_TWO_LINE,
		"condensed": dashboards.RowStyle_ROW_STYLE_CONDENSED,
		"json":      dashboards.RowStyle_ROW_STYLE_JSON,
	}
	dashboardRowStyleProtoToSchema     = ReverseMap(dashboardRowStyleSchemaToProto)
	dashboardValidRowStyles            = GetKeys(dashboardRowStyleSchemaToProto)
	dashboardLegendColumnSchemaToProto = map[string]dashboards.Legend_LegendColumn{
		"unspecified": dashboards.Legend_LEGEND_COLUMN_UNSPECIFIED,
		"min":         dashboards.Legend_LEGEND_COLUMN_MIN,
		"max":         dashboards.Legend_LEGEND_COLUMN_MAX,
		"sum":         dashboards.Legend_LEGEND_COLUMN_SUM,
		"avg":         dashboards.Legend_LEGEND_COLUMN_AVG,
		"last":        dashboards.Legend_LEGEND_COLUMN_LAST,
	}
	dashboardLegendColumnProtoToSchema   = ReverseMap(dashboardLegendColumnSchemaToProto)
	dashboardValidLegendColumns          = GetKeys(dashboardLegendColumnSchemaToProto)
	dashboardOrderDirectionSchemaToProto = map[string]dashboards.OrderDirection{
		//"unspecified": dashboards.OrderDirection_ORDER_DIRECTION_UNSPECIFIED,
		"asc":  dashboards.OrderDirection_ORDER_DIRECTION_ASC,
		"desc": dashboards.OrderDirection_ORDER_DIRECTION_DESC,
	}
	dashboardOrderDirectionProtoToSchema = ReverseMap(dashboardOrderDirectionSchemaToProto)
	dashboardValidOrderDirections        = GetKeys(dashboardOrderDirectionSchemaToProto)
	dashboardSchemaToProtoTooltipType    = map[string]dashboards.LineChart_TooltipType{
		"unspecified": dashboards.LineChart_TOOLTIP_TYPE_UNSPECIFIED,
		"all":         dashboards.LineChart_TOOLTIP_TYPE_ALL,
		"single":      dashboards.LineChart_TOOLTIP_TYPE_SINGLE,
	}
	dashboardProtoToSchemaTooltipType = ReverseMap(dashboardSchemaToProtoTooltipType)
	dashboardValidTooltipTypes        = GetKeys(dashboardSchemaToProtoTooltipType)
	dashboardSchemaToProtoScaleType   = map[string]dashboards.ScaleType{
		"unspecified": dashboards.ScaleType_SCALE_TYPE_UNSPECIFIED,
		"linear":      dashboards.ScaleType_SCALE_TYPE_LINEAR,
		"logarithmic": dashboards.ScaleType_SCALE_TYPE_LOGARITHMIC,
	}
	dashboardProtoToSchemaScaleType = ReverseMap(dashboardSchemaToProtoScaleType)
	dashboardValidScaleTypes        = GetKeys(dashboardSchemaToProtoScaleType)
	dashboardSchemaToProtoUnit      = map[string]dashboards.Unit{
		"unspecified":  dashboards.Unit_UNIT_UNSPECIFIED,
		"microseconds": dashboards.Unit_UNIT_MICROSECONDS,
		"milliseconds": dashboards.Unit_UNIT_MILLISECONDS,
		"seconds":      dashboards.Unit_UNIT_SECONDS,
		"bytes":        dashboards.Unit_UNIT_BYTES,
		"kbytes":       dashboards.Unit_UNIT_KBYTES,
		"mbytes":       dashboards.Unit_UNIT_MBYTES,
		"gbytes":       dashboards.Unit_UNIT_GBYTES,
		"bytes_iec":    dashboards.Unit_UNIT_BYTES_IEC,
		"kibytes":      dashboards.Unit_UNIT_KIBYTES,
		"mibytes":      dashboards.Unit_UNIT_MIBYTES,
		"gibytes":      dashboards.Unit_UNIT_GIBYTES,
		"euro_cents":   dashboards.Unit_UNIT_EUR_CENTS,
		"euro":         dashboards.Unit_UNIT_EUR,
		"usd_cents":    dashboards.Unit_UNIT_USD_CENTS,
		"usd":          dashboards.Unit_UNIT_USD,
	}
	dashboardProtoToSchemaUnit      = ReverseMap(dashboardSchemaToProtoUnit)
	dashboardValidUnits             = GetKeys(dashboardSchemaToProtoUnit)
	dashboardSchemaToProtoGaugeUnit = map[string]dashboards.Gauge_Unit{
		//"unspecified":  dashboards.Gauge_UNIT_UNSPECIFIED,
		"none":         dashboards.Gauge_UNIT_NUMBER,
		"percent":      dashboards.Gauge_UNIT_PERCENT,
		"microseconds": dashboards.Gauge_UNIT_MICROSECONDS,
		"milliseconds": dashboards.Gauge_UNIT_MILLISECONDS,
		"seconds":      dashboards.Gauge_UNIT_SECONDS,
		"bytes":        dashboards.Gauge_UNIT_BYTES,
		"kbytes":       dashboards.Gauge_UNIT_KBYTES,
		"mbytes":       dashboards.Gauge_UNIT_MBYTES,
		"gbytes":       dashboards.Gauge_UNIT_GBYTES,
		"bytes_iec":    dashboards.Gauge_UNIT_BYTES_IEC,
		"kibytes":      dashboards.Gauge_UNIT_KIBYTES,
		"mibytes":      dashboards.Gauge_UNIT_MIBYTES,
		"gibytes":      dashboards.Gauge_UNIT_GIBYTES,
		"euro_cents":   dashboards.Gauge_UNIT_EUR_CENTS,
		"euro":         dashboards.Gauge_UNIT_EUR,
		"usd_cents":    dashboards.Gauge_UNIT_USD_CENTS,
		"usd":          dashboards.Gauge_UNIT_USD,
	}
	dashboardProtoToSchemaGaugeUnit           = ReverseMap(dashboardSchemaToProtoGaugeUnit)
	dashboardValidGaugeUnits                  = GetKeys(dashboardSchemaToProtoGaugeUnit)
	dashboardSchemaToProtoPieChartLabelSource = map[string]dashboards.PieChart_LabelSource{
		"unspecified": dashboards.PieChart_LABEL_SOURCE_UNSPECIFIED,
		"inner":       dashboards.PieChart_LABEL_SOURCE_INNER,
		"stack":       dashboards.PieChart_LABEL_SOURCE_STACK,
	}
	dashboardProtoToSchemaPieChartLabelSource = ReverseMap(dashboardSchemaToProtoPieChartLabelSource)
	dashboardValidPieChartLabelSources        = GetKeys(dashboardSchemaToProtoPieChartLabelSource)
	dashboardSchemaToProtoGaugeAggregation    = map[string]dashboards.Gauge_Aggregation{
		"unspecified": dashboards.Gauge_AGGREGATION_UNSPECIFIED,
		"last":        dashboards.Gauge_AGGREGATION_LAST,
		"min":         dashboards.Gauge_AGGREGATION_MIN,
		"max":         dashboards.Gauge_AGGREGATION_MAX,
		"avg":         dashboards.Gauge_AGGREGATION_AVG,
		"sum":         dashboards.Gauge_AGGREGATION_SUM,
	}
	dashboardProtoToSchemaGaugeAggregation            = ReverseMap(dashboardSchemaToProtoGaugeAggregation)
	dashboardValidGaugeAggregations                   = GetKeys(dashboardSchemaToProtoGaugeAggregation)
	dashboardSchemaToProtoSpansAggregationMetricField = map[string]dashboards.SpansAggregation_MetricAggregation_MetricField{
		"unspecified": dashboards.SpansAggregation_MetricAggregation_METRIC_FIELD_UNSPECIFIED,
		"duration":    dashboards.SpansAggregation_MetricAggregation_METRIC_FIELD_DURATION,
	}
	dashboardProtoToSchemaSpansAggregationMetricField           = ReverseMap(dashboardSchemaToProtoSpansAggregationMetricField)
	dashboardValidSpansAggregationMetricFields                  = GetKeys(dashboardSchemaToProtoSpansAggregationMetricField)
	dashboardSchemaToProtoSpansAggregationMetricAggregationType = map[string]dashboards.SpansAggregation_MetricAggregation_MetricAggregationType{
		"unspecified":   dashboards.SpansAggregation_MetricAggregation_METRIC_AGGREGATION_TYPE_UNSPECIFIED,
		"min":           dashboards.SpansAggregation_MetricAggregation_METRIC_AGGREGATION_TYPE_MIN,
		"max":           dashboards.SpansAggregation_MetricAggregation_METRIC_AGGREGATION_TYPE_MAX,
		"avg":           dashboards.SpansAggregation_MetricAggregation_METRIC_AGGREGATION_TYPE_AVERAGE,
		"sum":           dashboards.SpansAggregation_MetricAggregation_METRIC_AGGREGATION_TYPE_SUM,
		"percentile_99": dashboards.SpansAggregation_MetricAggregation_METRIC_AGGREGATION_TYPE_PERCENTILE_99,
		"percentile_95": dashboards.SpansAggregation_MetricAggregation_METRIC_AGGREGATION_TYPE_PERCENTILE_95,
		"percentile_50": dashboards.SpansAggregation_MetricAggregation_METRIC_AGGREGATION_TYPE_PERCENTILE_50,
	}
	dashboardProtoToSchemaSpansAggregationMetricAggregationType = ReverseMap(dashboardSchemaToProtoSpansAggregationMetricAggregationType)
	dashboardValidSpansAggregationMetricAggregationTypes        = GetKeys(dashboardSchemaToProtoSpansAggregationMetricAggregationType)
	dashboardProtoToSchemaSpansAggregationDimensionField        = map[string]dashboards.SpansAggregation_DimensionAggregation_DimensionField{
		"unspecified": dashboards.SpansAggregation_DimensionAggregation_DIMENSION_FIELD_UNSPECIFIED,
		"trace_id":    dashboards.SpansAggregation_DimensionAggregation_DIMENSION_FIELD_TRACE_ID,
	}
	dashboardSchemaToProtoSpansAggregationDimensionField           = ReverseMap(dashboardProtoToSchemaSpansAggregationDimensionField)
	dashboardValidSpansAggregationDimensionFields                  = GetKeys(dashboardProtoToSchemaSpansAggregationDimensionField)
	dashboardSchemaToProtoSpansAggregationDimensionAggregationType = map[string]dashboards.SpansAggregation_DimensionAggregation_DimensionAggregationType{
		"unspecified":  dashboards.SpansAggregation_DimensionAggregation_DIMENSION_AGGREGATION_TYPE_UNSPECIFIED,
		"unique_count": dashboards.SpansAggregation_DimensionAggregation_DIMENSION_AGGREGATION_TYPE_UNIQUE_COUNT,
		"error_count":  dashboards.SpansAggregation_DimensionAggregation_DIMENSION_AGGREGATION_TYPE_ERROR_COUNT,
	}
	dashboardProtoToSchemaSpansAggregationDimensionAggregationType = ReverseMap(dashboardSchemaToProtoSpansAggregationDimensionAggregationType)
	dashboardValidSpansAggregationDimensionAggregationTypes        = GetKeys(dashboardSchemaToProtoSpansAggregationDimensionAggregationType)
	dashboardSchemaToProtoSpanFieldMetadataField                   = map[string]dashboards.SpanField_MetadataField{
		"unspecified":      dashboards.SpanField_METADATA_FIELD_UNSPECIFIED,
		"application_name": dashboards.SpanField_METADATA_FIELD_APPLICATION_NAME,
		"subsystem_name":   dashboards.SpanField_METADATA_FIELD_SUBSYSTEM_NAME,
		"service_name":     dashboards.SpanField_METADATA_FIELD_SERVICE_NAME,
		"operation_name":   dashboards.SpanField_METADATA_FIELD_OPERATION_NAME,
	}
	dashboardProtoToSchemaSpanFieldMetadataField = ReverseMap(dashboardSchemaToProtoSpanFieldMetadataField)
	dashboardValidSpanFieldMetadataFields        = GetKeys(dashboardSchemaToProtoSpanFieldMetadataField)
	dashboardSchemaToProtoSortBy                 = map[string]dashboards.SortByType{
		"unspecified": dashboards.SortByType_SORT_BY_TYPE_UNSPECIFIED,
		"value":       dashboards.SortByType_SORT_BY_TYPE_VALUE,
		"name":        dashboards.SortByType_SORT_BY_TYPE_NAME,
	}
	dashboardProtoToSchemaSortBy                = ReverseMap(dashboardSchemaToProtoSortBy)
	dashboardValidSortBy                        = GetKeys(dashboardSchemaToProtoSortBy)
	dashboardSchemaToProtoObservationFieldScope = map[string]dashboards.DatasetScope{
		"unspecified": dashboards.DatasetScope_DATASET_SCOPE_UNSPECIFIED,
		"user_data":   dashboards.DatasetScope_DATASET_SCOPE_USER_DATA,
		"label":       dashboards.DatasetScope_DATASET_SCOPE_LABEL,
		"metadata":    dashboards.DatasetScope_DATASET_SCOPE_METADATA,
	}
	dashboardProtoToSchemaObservationFieldScope = ReverseMap(dashboardSchemaToProtoObservationFieldScope)
	dashboardValidObservationFieldScope         = GetKeys(dashboardSchemaToProtoObservationFieldScope)
	dashboardSchemaToProtoDataModeType          = map[string]dashboards.DataModeType{
		"unspecified": dashboards.DataModeType_DATA_MODE_TYPE_HIGH_UNSPECIFIED,
		"archive":     dashboards.DataModeType_DATA_MODE_TYPE_ARCHIVE,
	}
	dashboardProtoToSchemaDataModeType     = ReverseMap(dashboardSchemaToProtoDataModeType)
	dashboardValidDataModeTypes            = GetKeys(dashboardSchemaToProtoDataModeType)
	dashboardSchemaToProtoGaugeThresholdBy = map[string]dashboards.Gauge_ThresholdBy{
		"unspecified": dashboards.Gauge_THRESHOLD_BY_UNSPECIFIED,
		"value":       dashboards.Gauge_THRESHOLD_BY_VALUE,
		"background":  dashboards.Gauge_THRESHOLD_BY_BACKGROUND,
	}
	dashboardProtoToSchemaGaugeThresholdBy = ReverseMap(dashboardSchemaToProtoGaugeThresholdBy)
	dashboardValidGaugeThresholdBy         = GetKeys(dashboardSchemaToProtoGaugeThresholdBy)
	dashboardSchemaToProtoRefreshStrategy  = map[string]dashboards.MultiSelect_RefreshStrategy{
		"unspecified":          dashboards.MultiSelect_REFRESH_STRATEGY_UNSPECIFIED,
		"on_dashboard_load":    dashboards.MultiSelect_REFRESH_STRATEGY_ON_DASHBOARD_LOAD,
		"on_time_frame_change": dashboards.MultiSelect_REFRESH_STRATEGY_ON_TIME_FRAME_CHANGE,
	}
	dashboardProtoToSchemaRefreshStrategy = ReverseMap(dashboardSchemaToProtoRefreshStrategy)
	dashboardValidRefreshStrategies       = GetKeys(dashboardSchemaToProtoRefreshStrategy)
	dashboardValidLogsAggregationTypes    = []string{"count", "count_distinct", "sum", "avg", "min", "max", "percentile"}
	dashboardValidSpanFieldTypes          = []string{"metadata", "tag", "process_tag"}
	dashboardValidSpanAggregationTypes    = []string{"metric", "dimension"}
	dashboardValidColorSchemes            = []string{"classic", "severity", "cold", "negative", "green", "red", "blue"}
	sectionValidColors                    = []string{"unspecified", "cyan", "green", "blue", "purple", "magenta", "pink", "orange"}
	createDashboardURL                    = "com.coralogixapis.dashboards.dashboards.services.DashboardsService/CreateDashboard"
	getDashboardURL                       = "com.coralogixapis.dashboards.dashboards.services.DashboardsService/GetDashboard"
	updateDashboardURL                    = "com.coralogixapis.dashboards.dashboards.services.DashboardsService/ReplaceDashboard"
	deleteDashboardURL                    = "com.coralogixapis.dashboards.dashboards.services.DashboardsService/DeleteDashboard"
)

var (
	_ resource.ResourceWithConfigure = &DashboardResource{}
	//_ resource.ResourceWithConfigValidators = &DashboardResource{}
	_ resource.ResourceWithImportState  = &DashboardResource{}
	_ resource.ResourceWithUpgradeState = &DashboardResource{}
)

type DashboardResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Layout      types.Object `tfsdk:"layout"`       //DashboardLayoutModel
	Variables   types.List   `tfsdk:"variables"`    //DashboardVariableModel
	Filters     types.List   `tfsdk:"filters"`      //DashboardFilterModel
	TimeFrame   types.Object `tfsdk:"time_frame"`   //DashboardTimeFrameModel
	Folder      types.Object `tfsdk:"folder"`       //DashboardFolderModel
	Annotations types.List   `tfsdk:"annotations"`  //DashboardAnnotationModel
	AutoRefresh types.Object `tfsdk:"auto_refresh"` //DashboardAutoRefreshModel
	ContentJson types.String `tfsdk:"content_json"`
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
	ID          types.String           `tfsdk:"id"`
	Title       types.String           `tfsdk:"title"`
	Description types.String           `tfsdk:"description"`
	Definition  *WidgetDefinitionModel `tfsdk:"definition"`
	Width       types.Int64            `tfsdk:"width"`
}

type WidgetDefinitionModel struct {
	LineChart          *LineChartModel          `tfsdk:"line_chart"`
	DataTable          *DataTableModel          `tfsdk:"data_table"`
	Gauge              *GaugeModel              `tfsdk:"gauge"`
	PieChart           *PieChartModel           `tfsdk:"pie_chart"`
	BarChart           *BarChartModel           `tfsdk:"bar_chart"`
	HorizontalBarChart *HorizontalBarChartModel `tfsdk:"horizontal_bar_chart"`
	Markdown           *MarkdownModel           `tfsdk:"markdown"`
}

type LineChartModel struct {
	Legend           *LegendModel  `tfsdk:"legend"`
	Tooltip          *TooltipModel `tfsdk:"tooltip"`
	QueryDefinitions types.List    `tfsdk:"query_definitions"` //LineChartQueryDefinitionModel
}

type LegendModel struct {
	IsVisible    types.Bool   `tfsdk:"is_visible"`
	Columns      types.List   `tfsdk:"columns"` //types.String (dashboardValidLegendColumns)
	GroupByQuery types.Bool   `tfsdk:"group_by_query"`
	Placement    types.String `tfsdk:"placement"`
}

type TooltipModel struct {
	ShowLabels types.Bool   `tfsdk:"show_labels"`
	Type       types.String `tfsdk:"type"`
}

type LineChartQueryDefinitionModel struct {
	ID                 types.String         `tfsdk:"id"`
	Query              *LineChartQueryModel `tfsdk:"query"`
	SeriesNameTemplate types.String         `tfsdk:"series_name_template"`
	SeriesCountLimit   types.Int64          `tfsdk:"series_count_limit"`
	Unit               types.String         `tfsdk:"unit"`
	ScaleType          types.String         `tfsdk:"scale_type"`
	Name               types.String         `tfsdk:"name"`
	IsVisible          types.Bool           `tfsdk:"is_visible"`
	ColorScheme        types.String         `tfsdk:"color_scheme"`
	Resolution         types.Object         `tfsdk:"resolution"` //LineChartResolutionModel
	DataModeType       types.String         `tfsdk:"data_mode_type"`
}

type LineChartResolutionModel struct {
	Interval         types.String `tfsdk:"interval"`
	BucketsPresented types.Int64  `tfsdk:"buckets_presented"`
}

type LineChartQueryModel struct {
	Logs    *LineChartQueryLogsModel    `tfsdk:"logs"`
	Metrics *LineChartQueryMetricsModel `tfsdk:"metrics"`
	Spans   *LineChartQuerySpansModel   `tfsdk:"spans"`
}

type LineChartQueryLogsModel struct {
	LuceneQuery  types.String `tfsdk:"lucene_query"`
	GroupBy      types.List   `tfsdk:"group_by"`     //types.String
	Aggregations types.List   `tfsdk:"aggregations"` //AggregationModel
	Filters      types.List   `tfsdk:"filters"`      //FilterModel
}

type FilterOperatorModel struct {
	Type           types.String `tfsdk:"type"`
	SelectedValues types.List   `tfsdk:"selected_values"` //types.String
}

type LineChartQueryMetricsModel struct {
	PromqlQuery types.String `tfsdk:"promql_query"`
	Filters     types.List   `tfsdk:"filters"` //MetricsFilterModel
}

type QueryMetricFilterModel struct {
	Metric   types.String         `tfsdk:"metric"`
	Label    types.String         `tfsdk:"label"`
	Operator *FilterOperatorModel `tfsdk:"operator"`
}

type LineChartQuerySpansModel struct {
	LuceneQuery  types.String `tfsdk:"lucene_query"`
	GroupBy      types.List   `tfsdk:"group_by"`     //SpansFieldModel
	Aggregations types.List   `tfsdk:"aggregations"` //SpansAggregationModel
	Filters      types.List   `tfsdk:"filters"`      //SpansFilterModel
}

type SpansAggregationModel struct {
	Type            types.String `tfsdk:"type"`
	AggregationType types.String `tfsdk:"aggregation_type"`
	Field           types.String `tfsdk:"field"`
}

type DataTableModel struct {
	Query          *DataTableQueryModel `tfsdk:"query"`
	ResultsPerPage types.Int64          `tfsdk:"results_per_page"`
	RowStyle       types.String         `tfsdk:"row_style"`
	Columns        types.List           `tfsdk:"columns"` //DataTableColumnModel
	OrderBy        *OrderByModel        `tfsdk:"order_by"`
	DataModeType   types.String         `tfsdk:"data_mode_type"`
}

type DataTableQueryLogsModel struct {
	LuceneQuery types.String                     `tfsdk:"lucene_query"`
	Filters     types.List                       `tfsdk:"filters"` //LogsFilterModel
	Grouping    *DataTableLogsQueryGroupingModel `tfsdk:"grouping"`
}

type LogsFilterModel struct {
	Field            types.String         `tfsdk:"field"`
	Operator         *FilterOperatorModel `tfsdk:"operator"`
	ObservationField types.Object         `tfsdk:"observation_field"`
}

type DataTableLogsQueryGroupingModel struct {
	GroupBy      types.List `tfsdk:"group_by"`     //types.String
	Aggregations types.List `tfsdk:"aggregations"` //DataTableLogsAggregationModel
	GroupBys     types.List `tfsdk:"group_bys"`    //types.String
}

type DataTableLogsAggregationModel struct {
	ID          types.String          `tfsdk:"id"`
	Name        types.String          `tfsdk:"name"`
	IsVisible   types.Bool            `tfsdk:"is_visible"`
	Aggregation *LogsAggregationModel `tfsdk:"aggregation"`
}

type LogsAggregationModel struct {
	Type             types.String  `tfsdk:"type"`
	Field            types.String  `tfsdk:"field"`
	Percent          types.Float64 `tfsdk:"percent"`
	ObservationField types.Object  `tfsdk:"observation_field"`
}

type DataTableQueryModel struct {
	Logs      *DataTableQueryLogsModel    `tfsdk:"logs"`
	Metrics   *DataTableQueryMetricsModel `tfsdk:"metrics"`
	Spans     *DataTableQuerySpansModel   `tfsdk:"spans"`
	DataPrime *DataPrimeModel             `tfsdk:"data_prime"`
}

type DataPrimeModel struct {
	Query   types.String `tfsdk:"query"`
	Filters types.List   `tfsdk:"filters"` //DashboardFilterSourceModel
}

type DataTableQueryMetricsModel struct {
	PromqlQuery types.String `tfsdk:"promql_query"`
	Filters     types.List   `tfsdk:"filters"` //MetricsFilterModel
}

type MetricsFilterModel struct {
	Metric   types.String         `tfsdk:"metric"`
	Label    types.String         `tfsdk:"label"`
	Operator *FilterOperatorModel `tfsdk:"operator"`
}

type DataTableColumnModel struct {
	Field types.String `tfsdk:"field"`
	Width types.Int64  `tfsdk:"width"`
}

type OrderByModel struct {
	Field          types.String `tfsdk:"field"`
	OrderDirection types.String `tfsdk:"order_direction"`
}

type DataTableQuerySpansModel struct {
	LuceneQuery types.String                      `tfsdk:"lucene_query"`
	Filters     types.List                        `tfsdk:"filters"` //SpansFilterModel
	Grouping    *DataTableSpansQueryGroupingModel `tfsdk:"grouping"`
}

type SpansFilterModel struct {
	Field    *SpansFieldModel     `tfsdk:"field"`
	Operator *FilterOperatorModel `tfsdk:"operator"`
}

type SpansFieldModel struct {
	Type  types.String `tfsdk:"type"`
	Value types.String `tfsdk:"value"`
}

type DataTableSpansQueryGroupingModel struct {
	GroupBy      types.List `tfsdk:"group_by"`     //SpansFieldModel
	Aggregations types.List `tfsdk:"aggregations"` //DataTableSpansAggregationModel
}

type GaugeModel struct {
	Query        *GaugeQueryModel `tfsdk:"query"`
	Min          types.Float64    `tfsdk:"min"`
	Max          types.Float64    `tfsdk:"max"`
	ShowInnerArc types.Bool       `tfsdk:"show_inner_arc"`
	ShowOuterArc types.Bool       `tfsdk:"show_outer_arc"`
	Unit         types.String     `tfsdk:"unit"`
	Thresholds   types.List       `tfsdk:"thresholds"` //GaugeThresholdModel
	DataModeType types.String     `tfsdk:"data_mode_type"`
	ThresholdBy  types.String     `tfsdk:"threshold_by"`
}

type GaugeQueryModel struct {
	Logs      *GaugeQueryLogsModel    `tfsdk:"logs"`
	Metrics   *GaugeQueryMetricsModel `tfsdk:"metrics"`
	Spans     *GaugeQuerySpansModel   `tfsdk:"spans"`
	DataPrime *DataPrimeModel         `tfsdk:"data_prime"`
}

type GaugeQueryLogsModel struct {
	LuceneQuery     types.String          `tfsdk:"lucene_query"`
	LogsAggregation *LogsAggregationModel `tfsdk:"logs_aggregation"`
	Filters         types.List            `tfsdk:"filters"` //LogsFilterModel
}

type GaugeQueryMetricsModel struct {
	PromqlQuery types.String `tfsdk:"promql_query"`
	Aggregation types.String `tfsdk:"aggregation"`
	Filters     types.List   `tfsdk:"filters"` //MetricsFilterModel
}

type GaugeQuerySpansModel struct {
	LuceneQuery      types.String           `tfsdk:"lucene_query"`
	SpansAggregation *SpansAggregationModel `tfsdk:"spans_aggregation"`
	Filters          types.List             `tfsdk:"filters"` //SpansFilterModel
}

type GaugeThresholdModel struct {
	From  types.Float64 `tfsdk:"from"`
	Color types.String  `tfsdk:"color"`
}

type PieChartModel struct {
	Query              *PieChartQueryModel           `tfsdk:"query"`
	MaxSlicesPerChart  types.Int64                   `tfsdk:"max_slices_per_chart"`
	MinSlicePercentage types.Int64                   `tfsdk:"min_slice_percentage"`
	StackDefinition    *PieChartStackDefinitionModel `tfsdk:"stack_definition"`
	LabelDefinition    *LabelDefinitionModel         `tfsdk:"label_definition"`
	ShowLegend         types.Bool                    `tfsdk:"show_legend"`
	GroupNameTemplate  types.String                  `tfsdk:"group_name_template"`
	Unit               types.String                  `tfsdk:"unit"`
	ColorScheme        types.String                  `tfsdk:"color_scheme"`
	DataModeType       types.String                  `tfsdk:"data_mode_type"`
}

type PieChartStackDefinitionModel struct {
	MaxSlicesPerStack types.Int64  `tfsdk:"max_slices_per_stack"`
	StackNameTemplate types.String `tfsdk:"stack_name_template"`
}

type PieChartQueryModel struct {
	Logs      *PieChartQueryLogsModel      `tfsdk:"logs"`
	Metrics   *PieChartQueryMetricsModel   `tfsdk:"metrics"`
	Spans     *PieChartQuerySpansModel     `tfsdk:"spans"`
	DataPrime *PieChartQueryDataPrimeModel `tfsdk:"data_prime"`
}

type PieChartQueryLogsModel struct {
	LuceneQuery           types.String          `tfsdk:"lucene_query"`
	Aggregation           *LogsAggregationModel `tfsdk:"aggregation"`
	Filters               types.List            `tfsdk:"filters"`     //LogsFilterModel
	GroupNames            types.List            `tfsdk:"group_names"` //types.String
	StackedGroupName      types.String          `tfsdk:"stacked_group_name"`
	GroupNamesFields      types.List            `tfsdk:"group_names_fields"`       //ObservationFieldModel
	StackedGroupNameField types.Object          `tfsdk:"stacked_group_name_field"` //ObservationFieldModel
}

type PieChartQueryMetricsModel struct {
	PromqlQuery      types.String `tfsdk:"promql_query"`
	Filters          types.List   `tfsdk:"filters"`     //MetricsFilterModel
	GroupNames       types.List   `tfsdk:"group_names"` //types.String
	StackedGroupName types.String `tfsdk:"stacked_group_name"`
}

type PieChartQuerySpansModel struct {
	LuceneQuery      types.String           `tfsdk:"lucene_query"`
	Aggregation      *SpansAggregationModel `tfsdk:"aggregation"`
	Filters          types.List             `tfsdk:"filters"`     //SpansFilterModel
	GroupNames       types.List             `tfsdk:"group_names"` //SpansFieldModel
	StackedGroupName *SpansFieldModel       `tfsdk:"stacked_group_name"`
}

type PieChartQueryDataPrimeModel struct {
	Query            types.String `tfsdk:"query"`
	Filters          types.List   `tfsdk:"filters"`     //DashboardFilterSourceModel
	GroupNames       types.List   `tfsdk:"group_names"` //types.String
	StackedGroupName types.String `tfsdk:"stacked_group_name"`
}

type LabelDefinitionModel struct {
	LabelSource    types.String `tfsdk:"label_source"`
	IsVisible      types.Bool   `tfsdk:"is_visible"`
	ShowName       types.Bool   `tfsdk:"show_name"`
	ShowValue      types.Bool   `tfsdk:"show_value"`
	ShowPercentage types.Bool   `tfsdk:"show_percentage"`
}

type BarChartModel struct {
	Query             *BarChartQueryModel           `tfsdk:"query"`
	MaxBarsPerChart   types.Int64                   `tfsdk:"max_bars_per_chart"`
	GroupNameTemplate types.String                  `tfsdk:"group_name_template"`
	StackDefinition   *BarChartStackDefinitionModel `tfsdk:"stack_definition"`
	ScaleType         types.String                  `tfsdk:"scale_type"`
	ColorsBy          types.String                  `tfsdk:"colors_by"`
	XAxis             *BarChartXAxisModel           `tfsdk:"xaxis"`
	Unit              types.String                  `tfsdk:"unit"`
	SortBy            types.String                  `tfsdk:"sort_by"`
	ColorScheme       types.String                  `tfsdk:"color_scheme"`
	DataModeType      types.String                  `tfsdk:"data_mode_type"`
}

type BarChartQueryModel struct {
	Logs      types.Object `tfsdk:"logs"`       //BarChartQueryLogsModel
	Metrics   types.Object `tfsdk:"metrics"`    //BarChartQueryMetricsModel
	Spans     types.Object `tfsdk:"spans"`      //BarChartQuerySpansModel
	DataPrime types.Object `tfsdk:"data_prime"` //BarChartQueryDataPrimeModel
}

type BarChartQueryLogsModel struct {
	LuceneQuery           types.String          `tfsdk:"lucene_query"`
	Aggregation           *LogsAggregationModel `tfsdk:"aggregation"`
	Filters               types.List            `tfsdk:"filters"`     //LogsFilterModel
	GroupNames            types.List            `tfsdk:"group_names"` //types.String
	StackedGroupName      types.String          `tfsdk:"stacked_group_name"`
	GroupNamesFields      types.List            `tfsdk:"group_names_fields"`       //ObservationFieldModel
	StackedGroupNameField types.Object          `tfsdk:"stacked_group_name_field"` //ObservationFieldModel
}

type ObservationFieldModel struct {
	Keypath types.List   `tfsdk:"keypath"` //types.String
	Scope   types.String `tfsdk:"scope"`
}

type BarChartQueryMetricsModel struct {
	PromqlQuery      types.String `tfsdk:"promql_query"`
	Filters          types.List   `tfsdk:"filters"`     //MetricsFilterModel
	GroupNames       types.List   `tfsdk:"group_names"` //types.String
	StackedGroupName types.String `tfsdk:"stacked_group_name"`
}

type BarChartQuerySpansModel struct {
	LuceneQuery      types.String           `tfsdk:"lucene_query"`
	Aggregation      *SpansAggregationModel `tfsdk:"aggregation"`
	Filters          types.List             `tfsdk:"filters"`     //SpansFilterModel
	GroupNames       types.List             `tfsdk:"group_names"` //SpansFieldModel
	StackedGroupName *SpansFieldModel       `tfsdk:"stacked_group_name"`
}

type BarChartQueryDataPrimeModel struct {
	Query            types.String `tfsdk:"query"`
	Filters          types.List   `tfsdk:"filters"`     //DashboardFilterSourceModel
	GroupNames       types.List   `tfsdk:"group_names"` //types.String
	StackedGroupName types.String `tfsdk:"stacked_group_name"`
}

type DataTableSpansAggregationModel struct {
	ID          types.String           `tfsdk:"id"`
	Name        types.String           `tfsdk:"name"`
	IsVisible   types.Bool             `tfsdk:"is_visible"`
	Aggregation *SpansAggregationModel `tfsdk:"aggregation"`
}

type BarChartStackDefinitionModel struct {
	MaxSlicesPerBar   types.Int64  `tfsdk:"max_slices_per_bar"`
	StackNameTemplate types.String `tfsdk:"stack_name_template"`
}

type BarChartXAxisModel struct {
	Time  *BarChartXAxisTimeModel  `tfsdk:"time"`
	Value *BarChartXAxisValueModel `tfsdk:"value"`
}

type BarChartXAxisTimeModel struct {
	Interval         types.String `tfsdk:"interval"`
	BucketsPresented types.Int64  `tfsdk:"buckets_presented"`
}

type BarChartXAxisValueModel struct {
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

type HorizontalBarChartModel struct {
	Query             *HorizontalBarChartQueryModel `tfsdk:"query"`
	MaxBarsPerChart   types.Int64                   `tfsdk:"max_bars_per_chart"`
	GroupNameTemplate types.String                  `tfsdk:"group_name_template"`
	StackDefinition   *BarChartStackDefinitionModel `tfsdk:"stack_definition"`
	ScaleType         types.String                  `tfsdk:"scale_type"`
	ColorsBy          types.String                  `tfsdk:"colors_by"`
	Unit              types.String                  `tfsdk:"unit"`
	DisplayOnBar      types.Bool                    `tfsdk:"display_on_bar"`
	YAxisViewBy       types.String                  `tfsdk:"y_axis_view_by"`
	SortBy            types.String                  `tfsdk:"sort_by"`
	ColorScheme       types.String                  `tfsdk:"color_scheme"`
	DataModeType      types.String                  `tfsdk:"data_mode_type"`
}

type HorizontalBarChartQueryModel struct {
	Logs    types.Object `tfsdk:"logs"`    //BarChartQueryLogsModel
	Metrics types.Object `tfsdk:"metrics"` //BarChartQueryMetricsModel
	Spans   types.Object `tfsdk:"spans"`   //BarChartQuerySpansModel
}

type MarkdownModel struct {
	MarkdownText types.String `tfsdk:"markdown_text"`
	TooltipText  types.String `tfsdk:"tooltip_text"`
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
	LogsPath     types.String                  `tfsdk:"logs_path"`
	MetricLabel  *MetricMultiSelectSourceModel `tfsdk:"metric_label"`
	ConstantList types.List                    `tfsdk:"constant_list"` //types.String
	SpanField    *SpansFieldModel              `tfsdk:"span_field"`
	Query        types.Object                  `tfsdk:"query"` //VariableMultiSelectQueryModel
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
	ObservationField types.Object `tfsdk:"observation_field"` //ObservationFieldModel
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
	FieldValue types.Object `tfsdk:"field_value"` //SpansFieldModel
}

type MultiSelectValueDisplayOptionsModel struct {
	ValueRegex types.String `tfsdk:"value_regex"`
	LabelRegex types.String `tfsdk:"label_regex"`
}

type DashboardFilterModel struct {
	Source    *DashboardFilterSourceModel `tfsdk:"source"`
	Enabled   types.Bool                  `tfsdk:"enabled"`
	Collapsed types.Bool                  `tfsdk:"collapsed"`
}

type DashboardFilterSourceModel struct {
	Logs    *FilterSourceLogsModel    `tfsdk:"logs"`
	Metrics *FilterSourceMetricsModel `tfsdk:"metrics"`
	Spans   *FilterSourceSpansModel   `tfsdk:"spans"`
}

type FilterSourceLogsModel struct {
	Field            types.String         `tfsdk:"field"`
	Operator         *FilterOperatorModel `tfsdk:"operator"`
	ObservationField types.Object         `tfsdk:"observation_field"`
}

type FilterSourceMetricsModel struct {
	MetricName  types.String         `tfsdk:"metric_name"`
	MetricLabel types.String         `tfsdk:"label"`
	Operator    *FilterOperatorModel `tfsdk:"operator"`
}

type FilterSourceSpansModel struct {
	Field    *SpansFieldModel     `tfsdk:"field"`
	Operator *FilterOperatorModel `tfsdk:"operator"`
}

type DashboardTimeFrameModel struct {
	Absolute types.Object `tfsdk:"absolute"` //DashboardTimeFrameAbsoluteModel
	Relative types.Object `tfsdk:"relative"` //DashboardTimeFrameRelativeModel
}

type DashboardTimeFrameAbsoluteModel struct {
	Start types.String `tfsdk:"start"`
	End   types.String `tfsdk:"end"`
}

type DashboardTimeFrameRelativeModel struct {
	Duration types.String `tfsdk:"duration"`
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
	LabelFields     types.List   `tfsdk:"label_fields"` //ObservationFieldModel
}

type DashboardAnnotationSpanOrLogsStrategyModel struct {
	Instant  types.Object `tfsdk:"instant"`  //DashboardAnnotationInstantStrategyModel
	Range    types.Object `tfsdk:"range"`    //DashboardAnnotationRangeStrategyModel
	Duration types.Object `tfsdk:"duration"` //DashboardAnnotationDurationStrategyModel
}

type DashboardAnnotationInstantStrategyModel struct {
	TimestampField types.Object `tfsdk:"timestamp_field"` //ObservationFieldModel
}

type DashboardAnnotationRangeStrategyModel struct {
	StartTimestampField types.Object `tfsdk:"start_time_timestamp_field"` //ObservationFieldModel
	EndTimestampField   types.Object `tfsdk:"end_time_timestamp_field"`   //ObservationFieldModel
}

type DashboardAnnotationDurationStrategyModel struct {
	StartTimestampField types.Object `tfsdk:"start_timestamp_field"` //ObservationFieldModel
	DurationField       types.Object `tfsdk:"duration_field"`        //ObservationFieldModel
}

type DashboardAnnotationMetricStrategyModel struct {
	StartTime types.Object `tfsdk:"start_time"` //MetricStrategyStartTimeModel
}

type MetricStrategyStartTimeModel struct {
}

type DashboardAutoRefreshModel struct {
	Type types.String `tfsdk:"type"`
}

func NewDashboardResource() resource.Resource {
	return &DashboardResource{}
}

type DashboardResource struct {
	client *clientset.DashboardsClient
}

func (r DashboardResource) UpgradeState(_ context.Context) map[int64]resource.StateUpgrader {
	schemaV1 := dashboardV1()
	return map[int64]resource.StateUpgrader{
		1: {
			PriorSchema:   &schemaV1,
			StateUpgrader: upgradeDashboardStateV1ToV2,
		},
	}
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

	upgradedStateData := DashboardResourceModel{
		ID:          priorStateData.ID,
		Name:        priorStateData.Name,
		Description: priorStateData.Description,
		Layout:      priorStateData.Layout,
		Variables:   priorStateData.Variables,
		Filters:     priorStateData.Filters,
		TimeFrame:   priorStateData.TimeFrame,
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

type intervalValidator struct{}

func (i intervalValidator) Description(_ context.Context) string {
	return "A duration string, such as 1s or 1m."
}

func (i intervalValidator) MarkdownDescription(_ context.Context) string {
	return "A duration string, such as 1s or 1m."
}

func (i intervalValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() {
		return
	}
	_, err := time.ParseDuration(req.ConfigValue.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("invalid duration", err.Error())
	}
}

func (r *DashboardResource) Schema(_ context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version:    2,
		Attributes: dashboardSchemaAttributes(),
	}
}

func dashboardSchemaAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed: true,
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
			MarkdownDescription: "Unique identifier for the dashboard.",
		},
		"name": schema.StringAttribute{
			Optional:            true,
			MarkdownDescription: "Display name of the dashboard.",
		},
		"description": schema.StringAttribute{
			Optional:            true,
			MarkdownDescription: "Brief description or summary of the dashboard's purpose or content.",
		},
		"layout": schema.SingleNestedAttribute{
			Optional: true,
			Attributes: map[string]schema.Attribute{
				"sections": schema.ListNestedAttribute{
					NestedObject: schema.NestedAttributeObject{
						Attributes: map[string]schema.Attribute{
							"id": schema.StringAttribute{
								Computed: true,
								PlanModifiers: []planmodifier.String{
									stringplanmodifier.UseStateForUnknown(),
								},
							},
							"rows": schema.ListNestedAttribute{
								NestedObject: schema.NestedAttributeObject{
									Attributes: map[string]schema.Attribute{
										"id": schema.StringAttribute{
											Computed: true,
											PlanModifiers: []planmodifier.String{
												stringplanmodifier.UseStateForUnknown(),
											},
										},
										"height": schema.Int64Attribute{
											Required: true,
											Validators: []validator.Int64{
												int64validator.AtLeast(1),
											},
											MarkdownDescription: "The height of the row.",
										},
										"widgets": schema.ListNestedAttribute{
											Optional: true,
											NestedObject: schema.NestedAttributeObject{
												Attributes: map[string]schema.Attribute{
													"id": schema.StringAttribute{
														Computed: true,
														PlanModifiers: []planmodifier.String{
															stringplanmodifier.UseStateForUnknown(),
														},
													},
													"title": schema.StringAttribute{
														Optional:            true,
														MarkdownDescription: "Widget title. Required for all widgets except markdown.",
													},
													"description": schema.StringAttribute{
														Optional:            true,
														MarkdownDescription: "Widget description.",
													},
													"definition": schema.SingleNestedAttribute{
														Required: true,
														Attributes: map[string]schema.Attribute{
															"line_chart": schema.SingleNestedAttribute{
																Attributes: map[string]schema.Attribute{
																	"legend": schema.SingleNestedAttribute{
																		Attributes: map[string]schema.Attribute{
																			"is_visible": schema.BoolAttribute{
																				Optional:            true,
																				Computed:            true,
																				Default:             booldefault.StaticBool(true),
																				MarkdownDescription: "Whether to display the legend. False by default.",
																			},
																			"columns": schema.ListAttribute{
																				ElementType: types.StringType,
																				Optional:    true,
																				Validators: []validator.List{
																					listvalidator.ValueStringsAre(stringvalidator.OneOf(dashboardValidLegendColumns...)),
																					listvalidator.SizeAtLeast(1),
																				},
																				MarkdownDescription: fmt.Sprintf("The columns to display in the legend. Valid values are: %s.", strings.Join(dashboardValidLegendColumns, ", ")),
																			},
																			"group_by_query": schema.BoolAttribute{
																				Optional: true,
																				Computed: true,
																				Default:  booldefault.StaticBool(false),
																			},
																			"placement": schema.StringAttribute{
																				Optional: true,
																				Computed: true,
																				Validators: []validator.String{
																					stringvalidator.OneOf(dashboardValidLegendPlacements...),
																				},
																				MarkdownDescription: fmt.Sprintf("The placement of the legend. Valid values are: %s.", strings.Join(dashboardValidLegendPlacements, ", ")),
																			},
																		},
																		Optional: true,
																	},
																	"tooltip": schema.SingleNestedAttribute{
																		Attributes: map[string]schema.Attribute{
																			"show_labels": schema.BoolAttribute{
																				Optional: true,
																				Computed: true,
																				Default:  booldefault.StaticBool(false),
																			},
																			"type": schema.StringAttribute{
																				Optional: true,
																				Validators: []validator.String{
																					stringvalidator.OneOf(dashboardValidTooltipTypes...),
																				},
																				MarkdownDescription: fmt.Sprintf("The tooltip type. Valid values are: %s.", strings.Join(dashboardValidTooltipTypes, ", ")),
																			},
																		},
																		Optional: true,
																	},
																	"query_definitions": schema.ListNestedAttribute{
																		NestedObject: schema.NestedAttributeObject{
																			Attributes: map[string]schema.Attribute{
																				"id": schema.StringAttribute{
																					Computed: true, PlanModifiers: []planmodifier.String{
																						stringplanmodifier.UseStateForUnknown(),
																					},
																				},
																				"query": schema.SingleNestedAttribute{
																					Attributes: map[string]schema.Attribute{
																						"logs": schema.SingleNestedAttribute{
																							Attributes: map[string]schema.Attribute{
																								"lucene_query": schema.StringAttribute{
																									Optional: true,
																								},
																								"group_by": schema.ListAttribute{
																									ElementType: types.StringType,
																									Optional:    true,
																								},
																								"filters":      logsFiltersSchema(),
																								"aggregations": logsAggregationsSchema(),
																							},
																							Optional: true,
																							Validators: []validator.Object{
																								objectvalidator.ExactlyOneOf(
																									path.MatchRelative().AtParent().AtName("metrics"),
																									path.MatchRelative().AtParent().AtName("spans"),
																								),
																							},
																						},
																						"metrics": schema.SingleNestedAttribute{
																							Attributes: map[string]schema.Attribute{
																								"promql_query": schema.StringAttribute{
																									Required: true,
																								},
																								"filters": metricFiltersSchema(),
																							},
																							Optional: true,
																							Validators: []validator.Object{
																								objectvalidator.ExactlyOneOf(
																									path.MatchRelative().AtParent().AtName("logs"),
																									path.MatchRelative().AtParent().AtName("spans"),
																								),
																							},
																						},
																						"spans": schema.SingleNestedAttribute{
																							Attributes: map[string]schema.Attribute{
																								"lucene_query": schema.StringAttribute{
																									Optional: true,
																								},
																								"group_by":     spansFieldsSchema(),
																								"aggregations": spansAggregationsSchema(),
																								"filters":      spansFilterSchema(),
																							},
																							Optional: true,
																							Validators: []validator.Object{
																								objectvalidator.ExactlyOneOf(
																									path.MatchRelative().AtParent().AtName("metrics"),
																									path.MatchRelative().AtParent().AtName("logs"),
																								),
																							},
																						},
																					},
																					Required: true,
																				},
																				"series_name_template": schema.StringAttribute{
																					Optional: true,
																				},
																				"series_count_limit": schema.Int64Attribute{
																					Optional: true,
																				},
																				"unit": schema.StringAttribute{
																					Optional: true,
																					Computed: true,
																					Default:  stringdefault.StaticString("unspecified"),
																					Validators: []validator.String{
																						stringvalidator.OneOf(dashboardValidUnits...),
																					},
																					MarkdownDescription: fmt.Sprintf("The unit. Valid values are: %s.", strings.Join(dashboardValidUnits, ", ")),
																				},
																				"scale_type": schema.StringAttribute{
																					Optional: true,
																					Computed: true,
																					Validators: []validator.String{
																						stringvalidator.OneOf(dashboardValidScaleTypes...),
																					},
																					Default:             stringdefault.StaticString("unspecified"),
																					MarkdownDescription: fmt.Sprintf("The scale type. Valid values are: %s.", strings.Join(dashboardValidScaleTypes, ", ")),
																				},
																				"name": schema.StringAttribute{
																					Optional: true,
																				},
																				"is_visible": schema.BoolAttribute{
																					Optional: true,
																					Computed: true,
																					Default:  booldefault.StaticBool(true),
																				},
																				"color_scheme": schema.StringAttribute{
																					Optional: true,
																					Validators: []validator.String{
																						stringvalidator.OneOf(dashboardValidColorSchemes...),
																					},
																				},
																				"resolution": schema.SingleNestedAttribute{
																					Attributes: map[string]schema.Attribute{
																						"interval": schema.StringAttribute{
																							Optional: true,
																							Validators: []validator.String{
																								stringvalidator.ExactlyOneOf(
																									path.MatchRelative().AtParent().AtName("buckets_presented"),
																								),
																							},
																						},
																						"buckets_presented": schema.Int64Attribute{
																							Optional: true,
																							Validators: []validator.Int64{
																								int64validator.ExactlyOneOf(
																									path.MatchRelative().AtParent().AtName("interval"),
																								),
																							},
																						},
																					},
																					Optional: true,
																				},
																				"data_mode_type": schema.StringAttribute{
																					Optional: true,
																					Computed: true,
																					Validators: []validator.String{
																						stringvalidator.OneOf(dashboardValidDataModeTypes...),
																					},
																					Default: stringdefault.StaticString("unspecified"),
																				},
																			},
																		},
																		Required: true,
																		Validators: []validator.List{
																			listvalidator.SizeAtLeast(1),
																		},
																	},
																},
																Validators: []validator.Object{
																	objectvalidator.ExactlyOneOf(
																		path.MatchRelative().AtParent().AtName("data_table"),
																		path.MatchRelative().AtParent().AtName("gauge"),
																		path.MatchRelative().AtParent().AtName("pie_chart"),
																		path.MatchRelative().AtParent().AtName("bar_chart"),
																		path.MatchRelative().AtParent().AtName("horizontal_bar_chart"),
																		path.MatchRelative().AtParent().AtName("markdown"),
																	),
																	objectvalidator.AlsoRequires(
																		path.MatchRelative().AtParent().AtParent().AtName("title"),
																	),
																},
																Optional: true,
															},
															"data_table": schema.SingleNestedAttribute{
																Attributes: map[string]schema.Attribute{
																	"query": schema.SingleNestedAttribute{
																		Attributes: map[string]schema.Attribute{
																			"logs": schema.SingleNestedAttribute{
																				Attributes: map[string]schema.Attribute{
																					"lucene_query": schema.StringAttribute{
																						Optional: true,
																					},
																					"filters": logsFiltersSchema(),
																					"grouping": schema.SingleNestedAttribute{
																						Attributes: map[string]schema.Attribute{
																							"group_by": schema.ListAttribute{
																								ElementType: types.StringType,
																								Optional:    true,
																							},
																							"aggregations": schema.ListNestedAttribute{
																								NestedObject: schema.NestedAttributeObject{
																									Attributes: map[string]schema.Attribute{
																										"id": schema.StringAttribute{
																											Computed: true,
																											Optional: true,
																											PlanModifiers: []planmodifier.String{
																												stringplanmodifier.UseStateForUnknown(),
																											},
																										},
																										"name": schema.StringAttribute{
																											Optional: true,
																										},
																										"is_visible": schema.BoolAttribute{
																											Optional: true,
																											Computed: true,
																											Default:  booldefault.StaticBool(true),
																										},
																										"aggregation": logsAggregationSchema(),
																									},
																								},
																								Optional: true,
																							},
																							"group_bys": schema.ListNestedAttribute{
																								NestedObject: schema.NestedAttributeObject{
																									Attributes: observationFieldSchemaAttributes(),
																								},
																								Optional: true,
																							},
																						},
																						Optional: true,
																					},
																				},
																				Optional: true,
																				Validators: []validator.Object{
																					objectvalidator.ExactlyOneOf(
																						path.MatchRelative().AtParent().AtName("spans"),
																						path.MatchRelative().AtParent().AtName("metrics"),
																						path.MatchRelative().AtParent().AtName("data_prime"),
																					),
																				},
																			},
																			"spans": schema.SingleNestedAttribute{
																				Attributes: map[string]schema.Attribute{
																					"lucene_query": schema.StringAttribute{
																						Optional: true,
																					},
																					"filters": spansFilterSchema(),
																					"grouping": schema.SingleNestedAttribute{
																						Attributes: map[string]schema.Attribute{
																							"group_by": spansFieldsSchema(),
																							"aggregations": schema.ListNestedAttribute{
																								NestedObject: schema.NestedAttributeObject{
																									Attributes: map[string]schema.Attribute{
																										"id": schema.StringAttribute{
																											Computed: true,
																											PlanModifiers: []planmodifier.String{
																												stringplanmodifier.UseStateForUnknown(),
																											},
																										},
																										"name": schema.StringAttribute{
																											Optional: true,
																										},
																										"is_visible": schema.BoolAttribute{
																											Optional: true,
																											Computed: true,
																											Default:  booldefault.StaticBool(true),
																										},
																										"aggregation": spansAggregationSchema(),
																									},
																								},
																								Optional: true,
																							},
																						},
																						Optional: true,
																					},
																				},
																				Optional: true,
																				Validators: []validator.Object{
																					objectvalidator.ExactlyOneOf(
																						path.MatchRelative().AtParent().AtName("logs"),
																						path.MatchRelative().AtParent().AtName("metrics"),
																						path.MatchRelative().AtParent().AtName("data_prime"),
																					),
																				},
																			},
																			"metrics": schema.SingleNestedAttribute{
																				Attributes: map[string]schema.Attribute{
																					"promql_query": schema.StringAttribute{
																						Required: true,
																					},
																					"filters": metricFiltersSchema(),
																				},
																				Optional: true,
																				Validators: []validator.Object{
																					objectvalidator.ExactlyOneOf(
																						path.MatchRelative().AtParent().AtName("logs"),
																						path.MatchRelative().AtParent().AtName("spans"),
																						path.MatchRelative().AtParent().AtName("data_prime"),
																					),
																				},
																			},
																			"data_prime": schema.SingleNestedAttribute{
																				Attributes: map[string]schema.Attribute{
																					"query": schema.StringAttribute{
																						Optional: true,
																					},
																					"filters": schema.ListNestedAttribute{
																						NestedObject: schema.NestedAttributeObject{
																							Attributes: filtersSourceAttribute(),
																						},
																						Optional: true,
																					},
																				},
																				Optional: true,
																				Validators: []validator.Object{
																					objectvalidator.ExactlyOneOf(
																						path.MatchRelative().AtParent().AtName("logs"),
																						path.MatchRelative().AtParent().AtName("spans"),
																						path.MatchRelative().AtParent().AtName("metrics"),
																					),
																				},
																			},
																		},
																		Required: true,
																	},
																	"results_per_page": schema.Int64Attribute{
																		Required:            true,
																		MarkdownDescription: "The number of results to display per page.",
																	},
																	"row_style": schema.StringAttribute{
																		Required: true,
																		Validators: []validator.String{
																			stringvalidator.OneOf(dashboardValidRowStyles...),
																		},
																		MarkdownDescription: fmt.Sprintf("The style of the rows. Can be one of %q.", dashboardValidRowStyles),
																	},
																	"columns": schema.ListNestedAttribute{
																		NestedObject: schema.NestedAttributeObject{
																			Attributes: map[string]schema.Attribute{
																				"field": schema.StringAttribute{
																					Required: true,
																				},
																				"width": schema.Int64Attribute{
																					Optional: true,
																					Computed: true,
																					Default:  int64default.StaticInt64(0),
																				},
																			},
																		},
																		Validators: []validator.List{
																			listvalidator.SizeAtLeast(1),
																		},
																		Optional: true,
																	},
																	"order_by": schema.SingleNestedAttribute{
																		Attributes: map[string]schema.Attribute{
																			"field": schema.StringAttribute{
																				Optional: true,
																			},
																			"order_direction": schema.StringAttribute{
																				Validators: []validator.String{
																					stringvalidator.OneOf(dashboardValidOrderDirections...),
																				},
																				MarkdownDescription: fmt.Sprintf("The order direction. Can be one of %q.", dashboardValidOrderDirections),
																				Optional:            true,
																				Computed:            true,
																				Default:             stringdefault.StaticString("unspecified"),
																			},
																		},
																		Optional: true,
																	},
																	"data_mode_type": schema.StringAttribute{
																		Optional: true,
																		Computed: true,
																		Validators: []validator.String{
																			stringvalidator.OneOf(dashboardValidDataModeTypes...),
																		},
																		Default:             stringdefault.StaticString("unspecified"),
																		MarkdownDescription: fmt.Sprintf("The data mode type. Can be one of %q.", dashboardValidDataModeTypes),
																	},
																},
																Validators: []validator.Object{
																	objectvalidator.ExactlyOneOf(
																		path.MatchRelative().AtParent().AtName("line_chart"),
																		path.MatchRelative().AtParent().AtName("gauge"),
																		path.MatchRelative().AtParent().AtName("pie_chart"),
																		path.MatchRelative().AtParent().AtName("bar_chart"),
																		path.MatchRelative().AtParent().AtName("horizontal_bar_chart"),
																		path.MatchRelative().AtParent().AtName("markdown"),
																	),
																	objectvalidator.AlsoRequires(
																		path.MatchRelative().AtParent().AtParent().AtName("title"),
																	),
																},
																Optional: true,
															},
															"gauge": schema.SingleNestedAttribute{
																Attributes: map[string]schema.Attribute{
																	"query": schema.SingleNestedAttribute{
																		Attributes: map[string]schema.Attribute{
																			"logs": schema.SingleNestedAttribute{
																				Attributes: map[string]schema.Attribute{
																					"lucene_query": schema.StringAttribute{
																						Optional: true,
																					},
																					"filters":          logsFiltersSchema(),
																					"logs_aggregation": logsAggregationSchema(),
																				},
																				Validators: []validator.Object{
																					objectvalidator.ExactlyOneOf(
																						path.MatchRelative().AtParent().AtName("spans"),
																						path.MatchRelative().AtParent().AtName("metrics"),
																						path.MatchRelative().AtParent().AtName("data_prime"),
																					),
																				},
																				Optional: true,
																			},
																			"metrics": schema.SingleNestedAttribute{
																				Attributes: map[string]schema.Attribute{
																					"promql_query": schema.StringAttribute{
																						Required: true,
																					},
																					"aggregation": schema.StringAttribute{
																						Validators: []validator.String{
																							stringvalidator.OneOf(dashboardValidGaugeAggregations...),
																						},
																						MarkdownDescription: fmt.Sprintf("The type of aggregation. Can be one of %q.", dashboardValidGaugeAggregations),
																						Optional:            true,
																						Computed:            true,
																						Default:             stringdefault.StaticString("unspecified"),
																					},
																					"filters": metricFiltersSchema(),
																				},
																				Optional: true,
																				Validators: []validator.Object{
																					objectvalidator.ExactlyOneOf(
																						path.MatchRelative().AtParent().AtName("logs"),
																						path.MatchRelative().AtParent().AtName("spans"),
																						path.MatchRelative().AtParent().AtName("data_prime"),
																					),
																				},
																			},
																			"spans": schema.SingleNestedAttribute{
																				Attributes: map[string]schema.Attribute{
																					"lucene_query": schema.StringAttribute{
																						Optional: true,
																					},
																					"spans_aggregation": spansAggregationSchema(),
																					"filters":           spansFilterSchema(),
																				},
																				Optional: true,
																				Validators: []validator.Object{
																					objectvalidator.ExactlyOneOf(
																						path.MatchRelative().AtParent().AtName("logs"),
																						path.MatchRelative().AtParent().AtName("metrics"),
																						path.MatchRelative().AtParent().AtName("data_prime"),
																					),
																				},
																			},
																			"data_prime": schema.SingleNestedAttribute{
																				Attributes: map[string]schema.Attribute{
																					"query": schema.StringAttribute{
																						Optional: true,
																					},
																					"filters": schema.ListNestedAttribute{
																						NestedObject: schema.NestedAttributeObject{
																							Attributes: filtersSourceAttribute(),
																						},
																						Optional: true,
																					},
																				},
																				Optional: true,
																				Validators: []validator.Object{
																					objectvalidator.ExactlyOneOf(
																						path.MatchRelative().AtParent().AtName("logs"),
																						path.MatchRelative().AtParent().AtName("spans"),
																						path.MatchRelative().AtParent().AtName("metrics"),
																					),
																				},
																			},
																		},
																		Required: true,
																	},
																	"min": schema.Float64Attribute{
																		Optional: true,
																		Computed: true,
																		Default:  float64default.StaticFloat64(0),
																	},
																	"max": schema.Float64Attribute{
																		Optional: true,
																		Computed: true,
																		Default:  float64default.StaticFloat64(100),
																	},
																	"show_inner_arc": schema.BoolAttribute{
																		Optional: true,
																		Computed: true,
																		Default:  booldefault.StaticBool(false),
																	},
																	"show_outer_arc": schema.BoolAttribute{
																		Optional: true,
																		Computed: true,
																		Default:  booldefault.StaticBool(true),
																	},
																	"unit": schema.StringAttribute{
																		Required: true,
																		Validators: []validator.String{
																			stringvalidator.OneOf(dashboardValidGaugeUnits...),
																		},
																		MarkdownDescription: fmt.Sprintf("The unit of the gauge. Can be one of %q.", dashboardValidGaugeUnits),
																	},
																	"thresholds": schema.ListNestedAttribute{
																		NestedObject: schema.NestedAttributeObject{
																			Attributes: map[string]schema.Attribute{
																				"color": schema.StringAttribute{
																					Optional: true,
																				},
																				"from": schema.Float64Attribute{
																					Optional: true,
																				},
																			},
																		},
																		Optional: true,
																	},
																	"data_mode_type": schema.StringAttribute{
																		Optional: true,
																		Computed: true,
																		Default:  stringdefault.StaticString("unspecified"),
																		Validators: []validator.String{
																			stringvalidator.OneOf(dashboardValidDataModeTypes...),
																		},
																		MarkdownDescription: fmt.Sprintf("The data mode type. Can be one of %q.", dashboardValidDataModeTypes),
																	},
																	"threshold_by": schema.StringAttribute{
																		Optional: true,
																		Computed: true,
																		Default:  stringdefault.StaticString("unspecified"),
																		Validators: []validator.String{
																			stringvalidator.OneOf(dashboardValidGaugeThresholdBy...),
																		},
																		MarkdownDescription: fmt.Sprintf("The threshold by. Can be one of %q.", dashboardValidGaugeThresholdBy),
																	},
																},
																Validators: []validator.Object{
																	objectvalidator.ExactlyOneOf(
																		path.MatchRelative().AtParent().AtName("line_chart"),
																		path.MatchRelative().AtParent().AtName("data_table"),
																		path.MatchRelative().AtParent().AtName("pie_chart"),
																		path.MatchRelative().AtParent().AtName("bar_chart"),
																		path.MatchRelative().AtParent().AtName("horizontal_bar_chart"),
																		path.MatchRelative().AtParent().AtName("markdown"),
																	),
																	objectvalidator.AlsoRequires(
																		path.MatchRelative().AtParent().AtParent().AtName("title"),
																	),
																},
																Optional: true,
															},
															"pie_chart": schema.SingleNestedAttribute{
																Attributes: map[string]schema.Attribute{
																	"query": schema.SingleNestedAttribute{
																		Attributes: map[string]schema.Attribute{
																			"logs": schema.SingleNestedAttribute{
																				Attributes: map[string]schema.Attribute{
																					"lucene_query": schema.StringAttribute{
																						Optional: true,
																					},
																					"aggregation": logsAggregationSchema(),
																					"filters":     logsFiltersSchema(),
																					"group_names": schema.ListAttribute{
																						ElementType: types.StringType,
																						Optional:    true,
																						Validators: []validator.List{
																							listvalidator.SizeAtLeast(1),
																						},
																					},
																					"stacked_group_name": schema.StringAttribute{
																						Optional: true,
																					},
																					"group_names_fields": schema.ListNestedAttribute{
																						NestedObject: schema.NestedAttributeObject{
																							Attributes: observationFieldSchemaAttributes(),
																						},
																						Optional: true,
																					},
																					"stacked_group_name_field": schema.SingleNestedAttribute{
																						Attributes: observationFieldSchemaAttributes(),
																						Optional:   true,
																					},
																				},
																				Optional: true,
																				Validators: []validator.Object{
																					objectvalidator.ExactlyOneOf(
																						path.MatchRelative().AtParent().AtName("spans"),
																						path.MatchRelative().AtParent().AtName("metrics"),
																						path.MatchRelative().AtParent().AtName("data_prime"),
																					),
																				},
																			},
																			"spans": schema.SingleNestedAttribute{
																				Attributes: map[string]schema.Attribute{
																					"lucene_query": schema.StringAttribute{
																						Optional: true,
																					},
																					"aggregation":        spansAggregationSchema(),
																					"filters":            spansFilterSchema(),
																					"group_names":        spansFieldsSchema(),
																					"stacked_group_name": spansFieldSchema(),
																				},
																				Optional: true,
																				Validators: []validator.Object{
																					objectvalidator.ExactlyOneOf(
																						path.MatchRelative().AtParent().AtName("logs"),
																						path.MatchRelative().AtParent().AtName("metrics"),
																						path.MatchRelative().AtParent().AtName("data_prime"),
																					),
																				},
																			},
																			"metrics": schema.SingleNestedAttribute{
																				Attributes: map[string]schema.Attribute{
																					"promql_query": schema.StringAttribute{
																						Required: true,
																					},
																					"filters": metricFiltersSchema(),
																					"group_names": schema.ListAttribute{
																						ElementType: types.StringType,
																						Optional:    true,
																					},
																					"stacked_group_name": schema.StringAttribute{
																						Optional: true,
																					},
																				},
																				Optional: true,
																				Validators: []validator.Object{
																					objectvalidator.ExactlyOneOf(
																						path.MatchRelative().AtParent().AtName("logs"),
																						path.MatchRelative().AtParent().AtName("spans"),
																						path.MatchRelative().AtParent().AtName("data_prime"),
																					),
																				},
																			},
																			"data_prime": schema.SingleNestedAttribute{
																				Attributes: map[string]schema.Attribute{
																					"query": schema.StringAttribute{
																						Required: true,
																					},
																					"filters": schema.ListNestedAttribute{
																						NestedObject: schema.NestedAttributeObject{
																							Attributes: filtersSourceAttribute(),
																						},
																						Optional: true,
																					},
																					"group_names": schema.ListAttribute{
																						ElementType: types.StringType,
																						Optional:    true,
																					},
																					"stacked_group_name": schema.StringAttribute{
																						Optional: true,
																					},
																				},
																				Optional: true,
																				Validators: []validator.Object{
																					objectvalidator.ExactlyOneOf(
																						path.MatchRelative().AtParent().AtName("logs"),
																						path.MatchRelative().AtParent().AtName("spans"),
																						path.MatchRelative().AtParent().AtName("metrics"),
																					),
																				},
																			},
																		},
																		Required: true,
																	},
																	"max_slices_per_chart": schema.Int64Attribute{
																		Optional: true,
																	},
																	"min_slice_percentage": schema.Int64Attribute{
																		Optional: true,
																	},
																	"stack_definition": schema.SingleNestedAttribute{
																		Attributes: map[string]schema.Attribute{
																			"max_slices_per_stack": schema.Int64Attribute{
																				Optional: true,
																			},
																			"stack_name_template": schema.StringAttribute{
																				Optional: true,
																			},
																		},
																		Optional: true,
																	},
																	"label_definition": schema.SingleNestedAttribute{
																		Attributes: map[string]schema.Attribute{
																			"label_source": schema.StringAttribute{
																				Optional: true,
																				Computed: true,
																				Default:  stringdefault.StaticString("unspecified"),
																				Validators: []validator.String{
																					stringvalidator.OneOf(dashboardValidPieChartLabelSources...),
																				},
																				MarkdownDescription: fmt.Sprintf("The source of the label. Valid values are: %s", strings.Join(dashboardValidPieChartLabelSources, ", ")),
																			},
																			"is_visible": schema.BoolAttribute{
																				Optional: true,
																				Computed: true,
																				Default:  booldefault.StaticBool(true),
																			},
																			"show_name": schema.BoolAttribute{
																				Optional: true,
																				Computed: true,
																				Default:  booldefault.StaticBool(true),
																			},
																			"show_value": schema.BoolAttribute{
																				Optional: true,
																				Computed: true,
																				Default:  booldefault.StaticBool(true),
																			},
																			"show_percentage": schema.BoolAttribute{
																				Optional: true,
																				Computed: true,
																				Default:  booldefault.StaticBool(true),
																			},
																		},
																		Required: true,
																	},
																	"show_legend": schema.BoolAttribute{
																		Optional: true,
																		Computed: true,
																		Default:  booldefault.StaticBool(true),
																	},
																	"group_name_template": schema.StringAttribute{
																		Optional: true,
																	},
																	"unit": schema.StringAttribute{
																		Optional: true,
																		Computed: true,
																		Default:  stringdefault.StaticString("unspecified"),
																	},
																	"color_scheme": schema.StringAttribute{
																		Optional: true,
																		Validators: []validator.String{
																			stringvalidator.OneOf(dashboardValidColorSchemes...),
																		},
																		Description: fmt.Sprintf("The color scheme. Can be one of %s.", strings.Join(dashboardValidColorSchemes, ", ")),
																	},
																	"data_mode_type": schema.StringAttribute{
																		Optional: true,
																		Computed: true,
																		Default:  stringdefault.StaticString("unspecified"),
																		Validators: []validator.String{
																			stringvalidator.OneOf(dashboardValidDataModeTypes...),
																		},
																	},
																},
																Validators: []validator.Object{
																	objectvalidator.ExactlyOneOf(
																		path.MatchRelative().AtParent().AtName("line_chart"),
																		path.MatchRelative().AtParent().AtName("gauge"),
																		path.MatchRelative().AtParent().AtName("data_table"),
																		path.MatchRelative().AtParent().AtName("bar_chart"),
																		path.MatchRelative().AtParent().AtName("horizontal_bar_chart"),
																		path.MatchRelative().AtParent().AtName("markdown"),
																	),
																},
																Optional: true,
															},
															"bar_chart": schema.SingleNestedAttribute{
																Attributes: map[string]schema.Attribute{
																	"query": schema.SingleNestedAttribute{
																		Attributes: map[string]schema.Attribute{
																			"logs": schema.SingleNestedAttribute{
																				Attributes: map[string]schema.Attribute{
																					"lucene_query": schema.StringAttribute{
																						Optional: true,
																					},
																					"aggregation": logsAggregationSchema(),
																					"filters":     logsFiltersSchema(),
																					"group_names": schema.ListAttribute{
																						ElementType: types.StringType,
																						Optional:    true,
																					},
																					"stacked_group_name": schema.StringAttribute{
																						Optional: true,
																					},
																					"group_names_fields": schema.ListNestedAttribute{
																						NestedObject: schema.NestedAttributeObject{
																							Attributes: observationFieldSchemaAttributes(),
																						},
																						Optional: true,
																					},
																					"stacked_group_name_field": schema.SingleNestedAttribute{
																						Attributes: observationFieldSchemaAttributes(),
																						Optional:   true,
																					},
																				},
																				Optional: true,
																				Validators: []validator.Object{
																					objectvalidator.ExactlyOneOf(
																						path.MatchRelative().AtParent().AtName("metrics"),
																						path.MatchRelative().AtParent().AtName("spans"),
																						path.MatchRelative().AtParent().AtName("data_prime"),
																					),
																				},
																			},
																			"metrics": schema.SingleNestedAttribute{
																				Attributes: map[string]schema.Attribute{
																					"promql_query": schema.StringAttribute{
																						Required: true,
																					},
																					"filters": metricFiltersSchema(),
																					"group_names": schema.ListAttribute{
																						ElementType: types.StringType,
																						Optional:    true,
																					},
																					"stacked_group_name": schema.StringAttribute{
																						Optional: true,
																					},
																				},
																				Optional: true,
																				Validators: []validator.Object{
																					objectvalidator.ExactlyOneOf(
																						path.MatchRelative().AtParent().AtName("logs"),
																						path.MatchRelative().AtParent().AtName("spans"),
																						path.MatchRelative().AtParent().AtName("data_prime"),
																					),
																				},
																			},
																			"spans": schema.SingleNestedAttribute{
																				Attributes: map[string]schema.Attribute{
																					"lucene_query": schema.StringAttribute{
																						Optional: true,
																					},
																					"aggregation":        spansAggregationSchema(),
																					"filters":            spansFilterSchema(),
																					"group_names":        spansFieldsSchema(),
																					"stacked_group_name": spansFieldSchema(),
																				},
																				Optional: true,
																				Validators: []validator.Object{
																					objectvalidator.ExactlyOneOf(
																						path.MatchRelative().AtParent().AtName("logs"),
																						path.MatchRelative().AtParent().AtName("metrics"),
																						path.MatchRelative().AtParent().AtName("data_prime"),
																					),
																				},
																			},
																			"data_prime": schema.SingleNestedAttribute{
																				Attributes: map[string]schema.Attribute{
																					"query": schema.StringAttribute{
																						Required: true,
																					},
																					"filters": schema.ListNestedAttribute{
																						NestedObject: schema.NestedAttributeObject{
																							Attributes: filtersSourceAttribute(),
																						},
																						Optional: true,
																					},
																					"group_names": schema.ListAttribute{
																						ElementType: types.StringType,
																						Optional:    true,
																					},
																					"stacked_group_name": schema.StringAttribute{
																						Optional: true,
																					},
																				},
																				Optional: true,
																				Validators: []validator.Object{
																					objectvalidator.ExactlyOneOf(
																						path.MatchRelative().AtParent().AtName("logs"),
																						path.MatchRelative().AtParent().AtName("metrics"),
																						path.MatchRelative().AtParent().AtName("spans"),
																					),
																				},
																			},
																		},
																		Optional: true,
																	},
																	"max_bars_per_chart": schema.Int64Attribute{
																		Optional: true,
																	},
																	"group_name_template": schema.StringAttribute{
																		Optional: true,
																	},
																	"stack_definition": schema.SingleNestedAttribute{
																		Optional: true,
																		Attributes: map[string]schema.Attribute{
																			"max_slices_per_bar": schema.Int64Attribute{
																				Optional: true,
																			},
																			"stack_name_template": schema.StringAttribute{
																				Optional: true,
																			},
																		},
																	},
																	"scale_type": schema.StringAttribute{
																		Optional: true,
																		Computed: true,
																		Default:  stringdefault.StaticString("unspecified"),
																	},
																	"colors_by": schema.StringAttribute{
																		Optional: true,
																	},
																	"xaxis": schema.SingleNestedAttribute{
																		Optional: true,
																		Attributes: map[string]schema.Attribute{
																			"time": schema.SingleNestedAttribute{
																				Attributes: map[string]schema.Attribute{
																					"interval": schema.StringAttribute{
																						Required: true,
																						Validators: []validator.String{
																							intervalValidator{},
																						},
																						MarkdownDescription: "The time interval to use for the x-axis. Valid values are in duration format, for example `1m0s` or `1h0m0s` (currently leading zeros should be added).",
																					},
																					"buckets_presented": schema.Int64Attribute{
																						Optional: true,
																					},
																				},
																				Optional: true,
																				Validators: []validator.Object{
																					objectvalidator.ExactlyOneOf(
																						path.MatchRelative().AtParent().AtName("value"),
																					),
																				},
																			},
																			"value": schema.SingleNestedAttribute{
																				Attributes: map[string]schema.Attribute{},
																				Optional:   true,
																				Validators: []validator.Object{
																					objectvalidator.ExactlyOneOf(
																						path.MatchRelative().AtParent().AtName("time"),
																					),
																				},
																			},
																		},
																	},
																	"unit": schema.StringAttribute{
																		Optional: true,
																		Computed: true,
																		Default:  stringdefault.StaticString("unspecified"),
																		Validators: []validator.String{
																			stringvalidator.OneOf(dashboardValidUnits...),
																		},
																		MarkdownDescription: fmt.Sprintf("The unit of the chart. Can be one of %s.", strings.Join(dashboardValidUnits, ", ")),
																	},
																	"sort_by": schema.StringAttribute{
																		Optional: true,
																		Computed: true,
																		Default:  stringdefault.StaticString("unspecified"),
																		Validators: []validator.String{
																			stringvalidator.OneOf(dashboardValidSortBy...),
																		},
																		Description: fmt.Sprintf("The field to sort by. Can be one of %s.", strings.Join(dashboardValidSortBy, ", ")),
																	},
																	"color_scheme": schema.StringAttribute{
																		Optional: true,
																		Validators: []validator.String{
																			stringvalidator.OneOf(dashboardValidColorSchemes...),
																		},
																		Description: fmt.Sprintf("The color scheme. Can be one of %s.", strings.Join(dashboardValidColorSchemes, ", ")),
																	},
																	"data_mode_type": schema.StringAttribute{
																		Optional: true,
																		Computed: true,
																		Default:  stringdefault.StaticString("unspecified"),
																		Validators: []validator.String{
																			stringvalidator.OneOf(dashboardValidDataModeTypes...),
																		},
																	},
																},
																Validators: []validator.Object{
																	objectvalidator.ExactlyOneOf(
																		path.MatchRelative().AtParent().AtName("data_table"),
																		path.MatchRelative().AtParent().AtName("gauge"),
																		path.MatchRelative().AtParent().AtName("pie_chart"),
																		path.MatchRelative().AtParent().AtName("line_chart"),
																		path.MatchRelative().AtParent().AtName("horizontal_bar_chart"),
																		path.MatchRelative().AtParent().AtName("markdown"),
																	),
																	objectvalidator.AlsoRequires(
																		path.MatchRelative().AtParent().AtParent().AtName("title"),
																	),
																},
																Optional: true,
															},
															"horizontal_bar_chart": schema.SingleNestedAttribute{
																Attributes: map[string]schema.Attribute{
																	"query": schema.SingleNestedAttribute{
																		Attributes: map[string]schema.Attribute{
																			"logs": schema.SingleNestedAttribute{
																				Attributes: map[string]schema.Attribute{
																					"lucene_query": schema.StringAttribute{
																						Optional: true,
																					},
																					"aggregation": logsAggregationSchema(),
																					"filters":     logsFiltersSchema(),
																					"group_names": schema.ListAttribute{
																						ElementType: types.StringType,
																						Optional:    true,
																						Validators: []validator.List{
																							listvalidator.SizeAtLeast(1),
																						},
																					},
																					"stacked_group_name": schema.StringAttribute{
																						Optional: true,
																					},
																					"group_names_fields": schema.ListNestedAttribute{
																						NestedObject: schema.NestedAttributeObject{
																							Attributes: observationFieldSchemaAttributes(),
																						},
																						Optional: true,
																					},
																					"stacked_group_name_field": schema.SingleNestedAttribute{
																						Attributes: observationFieldSchemaAttributes(),
																						Optional:   true,
																					},
																				},
																				Optional: true,
																			},
																			"metrics": schema.SingleNestedAttribute{
																				Attributes: map[string]schema.Attribute{
																					"promql_query": schema.StringAttribute{
																						Required: true,
																					},
																					"filters": metricFiltersSchema(),
																					"group_names": schema.ListAttribute{
																						ElementType: types.StringType,
																						Optional:    true,
																					},
																					"stacked_group_name": schema.StringAttribute{
																						Optional: true,
																					},
																				},
																				Optional: true,
																			},
																			"spans": schema.SingleNestedAttribute{
																				Attributes: map[string]schema.Attribute{
																					"lucene_query": schema.StringAttribute{
																						Optional: true,
																					},
																					"aggregation":        spansAggregationSchema(),
																					"filters":            spansFilterSchema(),
																					"group_names":        spansFieldsSchema(),
																					"stacked_group_name": spansFieldSchema(),
																				},
																				Optional: true,
																			},
																		},
																		Optional: true,
																	},
																	"max_bars_per_chart": schema.Int64Attribute{
																		Optional: true,
																	},
																	"group_name_template": schema.StringAttribute{
																		Optional: true,
																	},
																	"stack_definition": schema.SingleNestedAttribute{
																		Optional: true,
																		Attributes: map[string]schema.Attribute{
																			"max_slices_per_bar": schema.Int64Attribute{
																				Optional: true,
																			},
																			"stack_name_template": schema.StringAttribute{
																				Optional: true,
																			},
																		},
																	},
																	"scale_type": schema.StringAttribute{
																		Optional: true,
																		Computed: true,
																		Default:  stringdefault.StaticString("unspecified"),
																	},
																	"colors_by": schema.StringAttribute{
																		Optional: true,
																	},
																	"unit": schema.StringAttribute{
																		Optional: true,
																		Computed: true,
																		Default:  stringdefault.StaticString("unspecified"),
																		Validators: []validator.String{
																			stringvalidator.OneOf(dashboardValidUnits...),
																		},
																		MarkdownDescription: fmt.Sprintf("The unit of the chart. Can be one of %s.", strings.Join(dashboardValidUnits, ", ")),
																	},
																	"display_on_bar": schema.BoolAttribute{
																		Optional: true,
																		Computed: true,
																		Default:  booldefault.StaticBool(false),
																	},
																	"y_axis_view_by": schema.StringAttribute{
																		Optional: true,
																		Validators: []validator.String{
																			stringvalidator.OneOf("category", "value"),
																		},
																	},
																	"sort_by": schema.StringAttribute{
																		Optional: true,
																		Computed: true,
																		Default:  stringdefault.StaticString("unspecified"),
																		Validators: []validator.String{
																			stringvalidator.OneOf(dashboardValidSortBy...),
																		},
																	},
																	"color_scheme": schema.StringAttribute{
																		Optional: true,
																		Validators: []validator.String{
																			stringvalidator.OneOf(dashboardValidColorSchemes...),
																		},
																		Description: fmt.Sprintf("The color scheme. Can be one of %s.", strings.Join(dashboardValidColorSchemes, ", ")),
																	},
																	"data_mode_type": schema.StringAttribute{
																		Optional: true,
																		Computed: true,
																		Default:  stringdefault.StaticString("unspecified"),
																		Validators: []validator.String{
																			stringvalidator.OneOf(dashboardValidDataModeTypes...),
																		},
																	},
																},
																Validators: []validator.Object{
																	objectvalidator.ExactlyOneOf(
																		path.MatchRelative().AtParent().AtName("data_table"),
																		path.MatchRelative().AtParent().AtName("gauge"),
																		path.MatchRelative().AtParent().AtName("pie_chart"),
																		path.MatchRelative().AtParent().AtName("line_chart"),
																		path.MatchRelative().AtParent().AtName("bar_chart"),
																		path.MatchRelative().AtParent().AtName("markdown"),
																	),
																	objectvalidator.AlsoRequires(
																		path.MatchRelative().AtParent().AtParent().AtName("title"),
																	),
																},
																Optional: true,
															},
															"markdown": schema.SingleNestedAttribute{
																Attributes: map[string]schema.Attribute{
																	"markdown_text": schema.StringAttribute{
																		Optional: true,
																	},
																	"tooltip_text": schema.StringAttribute{
																		Optional: true,
																	},
																},
																Validators: []validator.Object{
																	objectvalidator.ExactlyOneOf(
																		path.MatchRelative().AtParent().AtName("data_table"),
																		path.MatchRelative().AtParent().AtName("gauge"),
																		path.MatchRelative().AtParent().AtName("pie_chart"),
																		path.MatchRelative().AtParent().AtName("line_chart"),
																		path.MatchRelative().AtParent().AtName("bar_chart"),
																		path.MatchRelative().AtParent().AtName("horizontal_bar_chart"),
																	),
																	objectvalidator.ConflictsWith(
																		path.MatchRelative().AtParent().AtParent().AtName("title"),
																	),
																},
																Optional: true,
															},
														},
														MarkdownDescription: "The widget definition. Can contain one of 'line_chart', 'data_table', 'gauge', 'pie_chart', 'bar_chart', 'horizontal_bar_chart', 'markdown'.",
													},
													"width": schema.Int64Attribute{
														Optional:            true,
														Computed:            true,
														Default:             int64default.StaticInt64(0),
														MarkdownDescription: "The width of the chart.",
													},
												},
											},
											Validators: []validator.List{
												listvalidator.SizeAtLeast(1),
											},
											MarkdownDescription: "The list of widgets to display in the dashboard.",
										},
									},
								},
								Validators: []validator.List{
									listvalidator.SizeAtLeast(1),
								},
								Optional: true,
							},
							"options": schema.SingleNestedAttribute{
								Attributes: map[string]schema.Attribute{
									"name": schema.StringAttribute{
										Required: true,
									},
									"description": schema.StringAttribute{
										Optional: true,
									},
									"color": schema.StringAttribute{
										Optional: true,
										Validators: []validator.String{
											stringvalidator.OneOf(sectionValidColors...),
										},
										MarkdownDescription: fmt.Sprintf("Section color, valid values: %v", sectionValidColors),
									},
									"collapsed": schema.BoolAttribute{
										Optional: true,
									},
								}, Optional: true,
							},
						},
					},
					Optional: true,
				},
			},
			MarkdownDescription: "Layout configuration for the dashboard's visual elements.",
			Validators: []validator.Object{
				objectvalidator.ExactlyOneOf(
					path.MatchRelative().AtParent().AtName("content_json"),
				),
			},
		},
		"variables": schema.ListNestedAttribute{
			Optional: true,
			NestedObject: schema.NestedAttributeObject{
				Attributes: map[string]schema.Attribute{
					"name": schema.StringAttribute{
						Optional: true,
					},
					"definition": schema.SingleNestedAttribute{
						Required: true,
						Attributes: map[string]schema.Attribute{
							"constant_value": schema.StringAttribute{
								Optional: true,
								Validators: []validator.String{
									stringvalidator.ExactlyOneOf(path.MatchRelative().AtParent().AtName("multi_select")),
								},
							},
							"multi_select": schema.SingleNestedAttribute{
								Attributes: map[string]schema.Attribute{
									"selected_values": schema.ListAttribute{
										ElementType: types.StringType,
										Optional:    true,
									},
									"values_order_direction": schema.StringAttribute{
										Required: true,
										Validators: []validator.String{
											stringvalidator.OneOf(dashboardValidOrderDirections...),
										},
										MarkdownDescription: fmt.Sprintf("The order direction of the values. Can be one of `%s`.", strings.Join(dashboardValidOrderDirections, "`, `")),
									},
									"source": schema.SingleNestedAttribute{
										Attributes: map[string]schema.Attribute{
											"logs_path": schema.StringAttribute{
												Optional: true,
												Validators: []validator.String{
													stringvalidator.ExactlyOneOf(
														path.MatchRelative().AtParent().AtName("metric_label"),
														path.MatchRelative().AtParent().AtName("constant_list"),
														path.MatchRelative().AtParent().AtName("span_field"),
														path.MatchRelative().AtParent().AtName("query"),
													),
												},
											},
											"metric_label": schema.SingleNestedAttribute{
												Attributes: map[string]schema.Attribute{
													"metric_name": schema.StringAttribute{
														Required: true,
													},
													"label": schema.StringAttribute{
														Required: true,
													},
												},
												Optional: true,
											},
											"constant_list": schema.ListAttribute{
												ElementType: types.StringType,
												Optional:    true,
											},
											"span_field": schema.SingleNestedAttribute{
												Attributes: spansFieldAttributes(),
												Optional:   true,
												Validators: []validator.Object{
													spansFieldValidator{},
												},
											},
											"query": schema.SingleNestedAttribute{
												Attributes: map[string]schema.Attribute{
													"query": schema.SingleNestedAttribute{
														Attributes: map[string]schema.Attribute{
															"logs": schema.SingleNestedAttribute{
																Attributes: map[string]schema.Attribute{
																	"field_name": schema.SingleNestedAttribute{
																		Optional: true,
																		Attributes: map[string]schema.Attribute{
																			"log_regex": schema.StringAttribute{
																				Required: true,
																			},
																		},
																		Validators: []validator.Object{
																			objectvalidator.ExactlyOneOf(path.MatchRelative().AtParent().AtName("field_value")),
																		},
																	},
																	"field_value": schema.SingleNestedAttribute{
																		Optional: true,
																		Attributes: map[string]schema.Attribute{
																			"observation_field": schema.SingleNestedAttribute{
																				Attributes: observationFieldSchemaAttributes(),
																				Required:   true,
																			},
																		},
																	},
																},
																Optional: true,
																Validators: []validator.Object{
																	objectvalidator.ExactlyOneOf(
																		path.MatchRelative().AtParent().AtName("spans"),
																		path.MatchRelative().AtParent().AtName("metrics"),
																	),
																},
															},
															"metrics": schema.SingleNestedAttribute{
																Attributes: map[string]schema.Attribute{
																	"metric_name": schema.SingleNestedAttribute{
																		Optional: true,
																		Attributes: map[string]schema.Attribute{
																			"metric_regex": schema.StringAttribute{
																				Required: true,
																			},
																		},
																		Validators: []validator.Object{
																			objectvalidator.ExactlyOneOf(
																				path.MatchRelative().AtParent().AtName("label_name"),
																				path.MatchRelative().AtParent().AtName("label_value"),
																			),
																		},
																	},
																	"label_name": schema.SingleNestedAttribute{
																		Optional: true,
																		Attributes: map[string]schema.Attribute{
																			"metric_regex": schema.StringAttribute{
																				Required: true,
																			},
																		},
																	},
																	"label_value": schema.SingleNestedAttribute{
																		Attributes: map[string]schema.Attribute{
																			"metric_name": stringOrVariableSchema(),
																			"label_name":  stringOrVariableSchema(),
																			"label_filters": schema.ListNestedAttribute{
																				Optional: true,
																				NestedObject: schema.NestedAttributeObject{
																					Attributes: map[string]schema.Attribute{
																						"metric": stringOrVariableSchema(),
																						"label":  stringOrVariableSchema(),
																						"operator": schema.SingleNestedAttribute{
																							Optional: true,
																							Attributes: map[string]schema.Attribute{
																								"type": schema.StringAttribute{
																									Required: true,
																									Validators: []validator.String{
																										stringvalidator.OneOf("equals", "not_equals"),
																									},
																								},
																								"selected_values": schema.ListNestedAttribute{
																									Optional: true,
																									NestedObject: schema.NestedAttributeObject{
																										Attributes: stringOrVariableAttr(),
																									},
																								},
																							},
																						},
																					},
																				},
																			},
																		},
																		Optional: true,
																	},
																},
																Optional: true,
															},
															"spans": schema.SingleNestedAttribute{
																Attributes: map[string]schema.Attribute{
																	"field_name": schema.SingleNestedAttribute{
																		Attributes: map[string]schema.Attribute{
																			"span_regex": schema.StringAttribute{
																				Required: true,
																			},
																		},
																		Optional: true,
																		Validators: []validator.Object{
																			objectvalidator.ExactlyOneOf(path.MatchRelative().AtParent().AtName("field_value")),
																		},
																	},
																	"field_value": spansFieldSchema(),
																},
																Optional: true,
															},
														},
														Required: true,
													},
													"refresh_strategy": schema.StringAttribute{
														Optional: true,
														Computed: true,
														Default:  stringdefault.StaticString("unspecified"),
														Validators: []validator.String{
															stringvalidator.OneOf(dashboardValidRefreshStrategies...),
														},
													},
													"value_display_options": schema.SingleNestedAttribute{
														Attributes: map[string]schema.Attribute{
															"value_regex": schema.StringAttribute{
																Optional: true,
															},
															"label_regex": schema.StringAttribute{
																Optional: true,
															},
														},
														Optional: true,
													},
												},
												Optional: true,
											},
										},
										Optional: true,
									},
								},
								Optional: true,
							},
						},
					},
					"display_name": schema.StringAttribute{
						Required: true,
					},
				},
			},
			Validators: []validator.List{
				listvalidator.SizeAtLeast(1),
			},
			MarkdownDescription: "List of variables that can be used within the dashboard for dynamic content.",
		},
		"filters": schema.ListNestedAttribute{
			Optional: true,
			NestedObject: schema.NestedAttributeObject{
				Attributes: map[string]schema.Attribute{
					"source": schema.SingleNestedAttribute{
						Attributes: filtersSourceAttribute(),
						Required:   true,
					},
					"enabled": schema.BoolAttribute{
						Optional: true,
						Computed: true,
						Default:  booldefault.StaticBool(true),
					},
					"collapsed": schema.BoolAttribute{
						Optional: true,
						Computed: true,
						Default:  booldefault.StaticBool(false),
					},
				},
			},
			Validators: []validator.List{
				listvalidator.SizeAtLeast(1),
			},
			MarkdownDescription: "List of filters that can be applied to the dashboard's data.",
		},
		"time_frame": schema.SingleNestedAttribute{
			Optional: true,
			Attributes: map[string]schema.Attribute{
				"absolute": schema.SingleNestedAttribute{
					Attributes: map[string]schema.Attribute{
						"start": schema.StringAttribute{
							Required: true,
						},
						"end": schema.StringAttribute{
							Required: true,
						},
					},
					Optional: true,
					Validators: []validator.Object{
						objectvalidator.ExactlyOneOf(path.MatchRelative().AtParent().AtName("relative")),
					},
					MarkdownDescription: "Absolute time frame specifying a fixed start and end time.",
				},
				"relative": schema.SingleNestedAttribute{
					Attributes: map[string]schema.Attribute{
						"duration": schema.StringAttribute{
							Required: true,
						},
					},
					Optional: true,
					Validators: []validator.Object{
						objectvalidator.ExactlyOneOf(path.MatchRelative().AtParent().AtName("absolute")),
					},
					MarkdownDescription: "Relative time frame specifying a duration from the current time.",
				},
			},
			MarkdownDescription: "Specifies the time frame for the dashboard's data. Can be either absolute or relative.",
		},
		"folder": schema.SingleNestedAttribute{
			Attributes: map[string]schema.Attribute{
				"id": schema.StringAttribute{
					Optional: true,
					Computed: true,
					Validators: []validator.String{
						stringvalidator.ExactlyOneOf(
							path.MatchRelative().AtParent().AtName("path"),
						),
					},
				},
				"path": schema.StringAttribute{
					Optional: true,
					Computed: true,
					Validators: []validator.String{
						stringvalidator.ExactlyOneOf(
							path.MatchRelative().AtParent().AtName("id"),
						),
					},
				},
			},
			Optional: true,
		},
		"annotations": schema.ListNestedAttribute{
			Optional: true,
			NestedObject: schema.NestedAttributeObject{
				Attributes: map[string]schema.Attribute{
					"id": schema.StringAttribute{
						Optional: true,
						Computed: true,
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.UseStateForUnknown(),
						},
					},
					"name": schema.StringAttribute{
						Required: true,
					},
					"enabled": schema.BoolAttribute{
						Optional: true,
						Computed: true,
						Default:  booldefault.StaticBool(true),
					},
					"source": schema.SingleNestedAttribute{
						Attributes: map[string]schema.Attribute{
							"metrics": schema.SingleNestedAttribute{
								Attributes: map[string]schema.Attribute{
									"promql_query": schema.StringAttribute{
										Required: true,
									},
									"strategy": schema.SingleNestedAttribute{
										Attributes: map[string]schema.Attribute{
											"start_time": schema.SingleNestedAttribute{
												Attributes: map[string]schema.Attribute{},
												Required:   true,
											},
										},
										Required: true,
									},
									"message_template": schema.StringAttribute{
										Optional: true,
									},
									"labels": schema.ListAttribute{
										ElementType: types.StringType,
										Optional:    true,
									},
								},
								Optional: true,
								Validators: []validator.Object{
									objectvalidator.ExactlyOneOf(
										path.MatchRelative().AtParent().AtName("logs"),
										path.MatchRelative().AtParent().AtName("spans"),
									),
								},
							},
							"logs": schema.SingleNestedAttribute{
								Attributes: logsAndSpansAttributes(),
								Optional:   true,
								Validators: []validator.Object{
									objectvalidator.ExactlyOneOf(
										path.MatchRelative().AtParent().AtName("metrics"),
										path.MatchRelative().AtParent().AtName("spans"),
									),
								},
							},
							"spans": schema.SingleNestedAttribute{
								Attributes: logsAndSpansAttributes(),
								Optional:   true,
								Validators: []validator.Object{
									objectvalidator.ExactlyOneOf(
										path.MatchRelative().AtParent().AtName("metrics"),
										path.MatchRelative().AtParent().AtName("logs"),
									),
								},
							},
						},
						Required: true,
					},
				},
			},
			Validators: []validator.List{
				listvalidator.SizeAtLeast(1),
			},
		},
		"auto_refresh": schema.SingleNestedAttribute{
			Attributes: map[string]schema.Attribute{
				"type": schema.StringAttribute{
					Optional: true,
					Computed: true,
					Default:  stringdefault.StaticString("off"),
					Validators: []validator.String{
						stringvalidator.OneOf("off", "two_minutes", "five_minutes"),
					},
				},
			},
			Optional: true,
			Computed: true,
		},
		"content_json": schema.StringAttribute{
			Optional: true,
			Validators: []validator.String{
				stringvalidator.ConflictsWith(
					path.MatchRelative().AtParent().AtName("id"),
					path.MatchRelative().AtParent().AtName("name"),
					path.MatchRelative().AtParent().AtName("description"),
					path.MatchRelative().AtParent().AtName("layout"),
					path.MatchRelative().AtParent().AtName("variables"),
					path.MatchRelative().AtParent().AtName("filters"),
					path.MatchRelative().AtParent().AtName("time_frame"),
					path.MatchRelative().AtParent().AtName("folder"),
					path.MatchRelative().AtParent().AtName("annotations"),
				),
				ContentJsonValidator{},
			},
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.RequiresReplaceIf(JSONStringsEqualPlanModifier, "", ""),
			},
			Description: "an option to set the dashboard content from a json file.",
		},
	}
}

func stringOrVariableSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Attributes: stringOrVariableAttr(),
		Optional:   true,
	}
}

func stringOrVariableAttr() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"string_value": schema.StringAttribute{
			Optional: true,
			Validators: []validator.String{
				stringvalidator.ExactlyOneOf(
					path.MatchRelative().AtParent().AtName("variable_name"),
				),
			},
		},
		"variable_name": schema.StringAttribute{
			Optional: true,
		},
	}
}

func logsAndSpansAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"lucene_query": schema.StringAttribute{
			Optional: true,
		},
		"strategy": logsAndSpansStrategy(),
		"message_template": schema.StringAttribute{
			Optional: true,
		},
		"label_fields": schema.ListNestedAttribute{
			NestedObject: schema.NestedAttributeObject{
				Attributes: observationFieldSchemaAttributes(),
			},
			Optional: true,
		},
	}
}

func logsAndSpansStrategy() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Attributes: map[string]schema.Attribute{
			"instant": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"timestamp_field": observationFieldSingleNestedAttribute(),
				},
				Optional: true,
			},
			"range": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"start_timestamp_field": observationFieldSingleNestedAttribute(),
					"end_timestamp_field":   observationFieldSingleNestedAttribute(),
				},
				Optional: true,
			},
			"duration": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"start_timestamp_field": observationFieldSingleNestedAttribute(),
					"duration_field":        observationFieldSingleNestedAttribute(),
				},
				Optional: true,
			},
		},
		Required: true,
	}
}

func relativeTimeFrameAttributes() map[string]attr.Type {
	return map[string]attr.Type{
		"duration": types.StringType,
	}
}

func observationFieldSingleNestedAttribute() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Attributes: observationFieldSchemaAttributes(),
		Required:   true,
	}
}

func observationFieldSchemaAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"keypath": schema.ListAttribute{
			ElementType: types.StringType,
			Required:    true,
		},
		"scope": schema.StringAttribute{
			Required: true,
			Validators: []validator.String{
				stringvalidator.OneOf(dashboardValidObservationFieldScope...),
			},
		},
	}
}

func filtersSourceAttribute() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"logs": schema.SingleNestedAttribute{
			Attributes: map[string]schema.Attribute{
				"field": schema.StringAttribute{
					Required:            true,
					MarkdownDescription: "Field in the logs to apply the filter on.",
				},
				"operator": filterOperatorSchema(),
				"observation_field": schema.SingleNestedAttribute{
					Attributes: observationFieldSchemaAttributes(),
					Optional:   true,
				},
			},
			Optional: true,
			Validators: []validator.Object{
				objectvalidator.ExactlyOneOf(
					path.MatchRelative().AtParent().AtName("metrics"),
					path.MatchRelative().AtParent().AtName("spans"),
				),
			},
		},
		"spans": schema.SingleNestedAttribute{
			Attributes: map[string]schema.Attribute{
				"field": schema.SingleNestedAttribute{
					Attributes: spansFieldAttributes(),
					Required:   true,
					Validators: []validator.Object{
						spansFieldValidator{},
					},
				},
				"operator": filterOperatorSchema(),
			},
			Optional: true,
			Validators: []validator.Object{
				objectvalidator.ExactlyOneOf(
					path.MatchRelative().AtParent().AtName("metrics"),
					path.MatchRelative().AtParent().AtName("logs"),
				),
			},
		},
		"metrics": schema.SingleNestedAttribute{
			Attributes: map[string]schema.Attribute{
				"metric_name": schema.StringAttribute{
					Required: true,
				},
				"label": schema.StringAttribute{
					Required: true,
				},
				"operator": filterOperatorSchema(),
			},
			Validators: []validator.Object{
				objectvalidator.ExactlyOneOf(
					path.MatchRelative().AtParent().AtName("spans"),
					path.MatchRelative().AtParent().AtName("logs"),
				),
			},
			Optional: true,
		},
	}
}

type ContentJsonValidator struct{}

func (c ContentJsonValidator) Description(_ context.Context) string {
	return ""
}

func (c ContentJsonValidator) MarkdownDescription(_ context.Context) string {
	return ""
}

func (c ContentJsonValidator) ValidateString(_ context.Context, request validator.StringRequest, response *validator.StringResponse) {
	if request.ConfigValue.IsNull() {
		return
	}

	err := protojson.Unmarshal([]byte(request.ConfigValue.ValueString()), &dashboards.Dashboard{})
	if err != nil {
		response.Diagnostics.Append(diag.NewErrorDiagnostic("content_json validation failed", fmt.Sprintf("json content is not matching layout schema. got an err while unmarshalling - %s", err)))
	}
}

func JSONStringsEqualPlanModifier(_ context.Context, plan planmodifier.StringRequest, req *stringplanmodifier.RequiresReplaceIfFuncResponse) {
	if diffType, _ := jsondiff.Compare([]byte(plan.PlanValue.ValueString()), []byte(plan.StateValue.ValueString()), &jsondiff.Options{}); !(diffType == jsondiff.FullMatch || diffType == jsondiff.SupersetMatch) {
		req.RequiresReplace = false
	}
	req.RequiresReplace = true
}

func metricFiltersSchema() schema.ListNestedAttribute {
	return schema.ListNestedAttribute{
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"metric": schema.StringAttribute{
					Required:            true,
					MarkdownDescription: "Metric name to apply the filter on.",
				},
				"label": schema.StringAttribute{
					Optional:            true,
					MarkdownDescription: "Label associated with the metric.",
				},
				"operator": filterOperatorSchema(),
			},
		},
		Validators: []validator.List{
			listvalidator.SizeAtLeast(1),
		},
		Optional: true,
	}
}

func filterOperatorSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Attributes: map[string]schema.Attribute{
			"type": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.OneOf("equals", "not_equals"),
				},
				MarkdownDescription: "The type of the operator. Can be one of `equals` or `not_equals`.",
			},
			"selected_values": schema.ListAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				MarkdownDescription: "the values to filter by. When the type is `equals`, this field is optional, the filter will match only the selected values, and all the values if not set. When the type is `not_equals`, this field is required, and the filter will match spans without the selected values.",
			},
		},
		Validators: []validator.Object{
			filterOperatorValidator{},
		},
		Required:            true,
		MarkdownDescription: "Operator to use for filtering.",
	}
}

type filterOperatorValidator struct{}

func (f filterOperatorValidator) Description(_ context.Context) string {
	return ""
}

func (f filterOperatorValidator) MarkdownDescription(_ context.Context) string {
	return ""
}

func (f filterOperatorValidator) ValidateObject(ctx context.Context, req validator.ObjectRequest, resp *validator.ObjectResponse) {
	if req.ConfigValue.IsNull() {
		return
	}

	var filter FilterOperatorModel
	diags := req.ConfigValue.As(ctx, &filter, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	if filter.Type.ValueString() == "not_equals" && filter.SelectedValues.IsNull() {
		resp.Diagnostics.Append(diag.NewErrorDiagnostic("filter operator validation failed", "when type is `not_equals`, `selected_values` must be set"))
	}
}

func logsFiltersSchema() schema.ListNestedAttribute {
	return schema.ListNestedAttribute{
		Optional: true,
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"field": schema.StringAttribute{
					Required: true,
				},
				"operator": filterOperatorSchema(),
				"observation_field": schema.SingleNestedAttribute{
					Attributes: observationFieldSchemaAttributes(),
					Optional:   true,
				},
			},
		},
		Validators: []validator.List{
			listvalidator.SizeAtLeast(1),
		},
	}
}

type logsAggregationValidator struct{}

func (l logsAggregationValidator) Description(ctx context.Context) string {
	return ""
}

func (l logsAggregationValidator) MarkdownDescription(ctx context.Context) string {
	return ""
}

func (l logsAggregationValidator) ValidateObject(ctx context.Context, req validator.ObjectRequest, resp *validator.ObjectResponse) {
	if req.ConfigValue.IsNull() {
		return
	}

	var aggregation LogsAggregationModel
	diags := req.ConfigValue.As(ctx, &aggregation, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	aggregationType := aggregation.Type.ValueString()
	if aggregationType == "count" && !aggregation.Field.IsNull() {
		resp.Diagnostics.Append(diag.NewErrorDiagnostic("logs aggregation validation failed", "when type is `count`, `field` cannot be set"))
	} else if aggregationType != "count" && aggregation.Field.IsNull() {
		resp.Diagnostics.Append(diag.NewErrorDiagnostic("logs aggregation validation failed", fmt.Sprintf("when type is `%s`, `field` must be set", aggregationType)))
	}

	if aggregationType == "percentile" && aggregation.Percent.IsNull() {
		resp.Diagnostics.Append(diag.NewErrorDiagnostic("logs aggregation validation failed", "when type is `percentile`, `percent` must be set"))
	} else if aggregationType != "percentile" && !aggregation.Percent.IsNull() {
		resp.Diagnostics.Append(diag.NewErrorDiagnostic("logs aggregation validation failed", fmt.Sprintf("when type is `%s`, `percent` cannot be set", aggregationType)))
	}
}

func logsAggregationSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Required:   true,
		Attributes: logsAggregationAttributes(),
		Validators: []validator.Object{
			logsAggregationValidator{},
		},
	}
}

func logsAggregationsSchema() schema.ListNestedAttribute {
	return schema.ListNestedAttribute{
		Required: true,
		NestedObject: schema.NestedAttributeObject{
			Attributes: logsAggregationAttributes(),
			Validators: []validator.Object{
				logsAggregationValidator{},
			},
		},
		Validators: []validator.List{
			listvalidator.SizeAtLeast(1),
		},
	}
}

func logsAggregationAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"type": schema.StringAttribute{
			Required: true,
			Validators: []validator.String{
				stringvalidator.OneOf(dashboardValidLogsAggregationTypes...),
			},
			MarkdownDescription: fmt.Sprintf("The type of the aggregation. Can be one of %q", dashboardValidLogsAggregationTypes),
		},
		"field": schema.StringAttribute{
			Optional: true,
		},
		"percent": schema.Float64Attribute{
			Optional: true,
			Validators: []validator.Float64{
				float64validator.Between(0, 100),
			},
			MarkdownDescription: "The percentage of the aggregation to return. required when type is `percentile`.",
		},
		"observation_field": schema.SingleNestedAttribute{
			Attributes: observationFieldSchemaAttributes(),
			Optional:   true,
		},
	}
}

type spansFieldValidator struct{}

func (s spansFieldValidator) Description(ctx context.Context) string {
	return ""
}

func (s spansFieldValidator) MarkdownDescription(ctx context.Context) string {
	return ""
}

func (s spansFieldValidator) ValidateObject(ctx context.Context, request validator.ObjectRequest, response *validator.ObjectResponse) {
	if request.ConfigValue.IsNull() {
		return
	}

	var field SpansFieldModel
	diags := request.ConfigValue.As(ctx, &field, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		response.Diagnostics.Append(diags...)
		return
	}
	if field.Type.ValueString() == "metadata" && !slices.Contains(dashboardValidSpanFieldMetadataFields, field.Value.ValueString()) {
		response.Diagnostics.Append(diag.NewErrorDiagnostic("spans field validation failed", fmt.Sprintf("when type is `metadata`, `value` must be one of %q", dashboardValidSpanFieldMetadataFields)))
	}
}

func spansFieldSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Attributes: spansFieldAttributes(),
		Optional:   true,
		Validators: []validator.Object{
			spansFieldValidator{},
		},
	}
}

func spansFieldsSchema() schema.ListNestedAttribute {
	return schema.ListNestedAttribute{
		NestedObject: schema.NestedAttributeObject{
			Attributes: spansFieldAttributes(),
			Validators: []validator.Object{
				spansFieldValidator{},
			},
		},
		Optional: true,
	}
}

func spansFieldAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"type": schema.StringAttribute{
			Required: true,
			Validators: []validator.String{
				stringvalidator.OneOf(dashboardValidSpanFieldTypes...),
			},
			MarkdownDescription: fmt.Sprintf("The type of the field. Can be one of %q", dashboardValidSpanFieldTypes),
		},
		"value": schema.StringAttribute{
			Required:            true,
			MarkdownDescription: fmt.Sprintf("The value of the field. When the field type is `metadata`, can be one of %q", dashboardValidSpanFieldMetadataFields),
		},
	}
}

type spansAggregationValidator struct{}

func (s spansAggregationValidator) Description(ctx context.Context) string {
	return ""
}

func (s spansAggregationValidator) MarkdownDescription(ctx context.Context) string {
	return ""
}

func (s spansAggregationValidator) ValidateObject(ctx context.Context, request validator.ObjectRequest, response *validator.ObjectResponse) {
	if request.ConfigValue.IsNull() {
		return
	}

	var aggregation SpansAggregationModel
	diags := request.ConfigValue.As(ctx, &aggregation, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		response.Diagnostics.Append(diags...)
		return
	}

	if aggregation.Type.ValueString() == "metrics" && !slices.Contains(dashboardValidSpansAggregationMetricAggregationTypes, aggregation.AggregationType.ValueString()) {
		response.Diagnostics.Append(diag.NewErrorDiagnostic("spans aggregation validation failed", fmt.Sprintf("when type is `metrics`, `aggregation_type` must be one of %q", dashboardValidSpansAggregationMetricAggregationTypes)))
	}
	if aggregation.Type.ValueString() == "dimension" && !slices.Contains(dashboardValidSpansAggregationDimensionAggregationTypes, aggregation.AggregationType.ValueString()) {
		response.Diagnostics.Append(diag.NewErrorDiagnostic("spans aggregation validation failed", fmt.Sprintf("when type is `dimension`, `aggregation_type` must be one of %q", dashboardValidSpansAggregationDimensionAggregationTypes)))
	}
}

func spansAggregationSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Attributes: spansAggregationAttributes(),
		Optional:   true,
		Validators: []validator.Object{
			spansAggregationValidator{},
		},
	}
}

func spansAggregationsSchema() schema.ListNestedAttribute {
	return schema.ListNestedAttribute{
		NestedObject: schema.NestedAttributeObject{
			Attributes: spansAggregationAttributes(),
			Validators: []validator.Object{
				spansAggregationValidator{},
			},
		},
		Optional: true,
	}
}
func spansAggregationAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"type": schema.StringAttribute{
			Required: true,
			Validators: []validator.String{
				stringvalidator.OneOf(dashboardValidSpanAggregationTypes...),
			},
			MarkdownDescription: fmt.Sprintf("Can be one of %q", dashboardValidSpanAggregationTypes),
		},
		"aggregation_type": schema.StringAttribute{
			Required:            true,
			MarkdownDescription: fmt.Sprintf("The type of the aggregation. When the aggregation type is `metrics`, can be one of %q. When the aggregation type is `dimension`, can be one of %q.", dashboardValidSpansAggregationMetricAggregationTypes, dashboardValidSpansAggregationDimensionAggregationTypes),
		},
		"field": schema.StringAttribute{
			Required:            true,
			MarkdownDescription: fmt.Sprintf("The field to aggregate on. When the aggregation type is `metrics`, can be one of %q. When the aggregation type is `dimension`, can be one of %q.", dashboardValidSpansAggregationMetricFields, dashboardValidSpansAggregationDimensionFields),
		},
	}
}

func spansFilterSchema() schema.ListNestedAttribute {
	return schema.ListNestedAttribute{
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"field": schema.SingleNestedAttribute{
					Attributes: spansFieldAttributes(),
					Required:   true,
				},
				"operator": filterOperatorSchema(),
			},
		},
		Optional: true,
	}
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

	createDashboardReq := &dashboards.CreateDashboardRequest{
		Dashboard: dashboard,
	}
	dashboardStr := protojson.Format(createDashboardReq)
	log.Printf("[INFO] Creating new Dashboard: %s", dashboardStr)
	_, err := r.client.CreateDashboard(ctx, createDashboardReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error creating Dashboard",
			formatRpcErrors(err, createDashboardURL, dashboardStr),
		)
		return
	}

	getDashboardReq := &dashboards.GetDashboardRequest{
		DashboardId: createDashboardReq.Dashboard.Id,
	}
	getDashboardResp, err := r.client.GetDashboard(ctx, getDashboardReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		reqStr := protojson.Format(getDashboardReq)
		resp.Diagnostics.AddError(
			"Error getting Dashboard",
			formatRpcErrors(err, getDashboardURL, reqStr),
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
	log.Printf("[INFO] Flattened Dashboard: %s", flattenedDashboard)
	plan = *flattenedDashboard

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func extractDashboard(ctx context.Context, plan DashboardResourceModel) (*dashboards.Dashboard, diag.Diagnostics) {
	if !plan.ContentJson.IsNull() {
		dashboard := new(dashboards.Dashboard)
		if err := protojson.Unmarshal([]byte(plan.ContentJson.ValueString()), dashboard); err != nil {
			return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error unmarshalling dashboard content json", err.Error())}
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

	id := wrapperspb.String(expand21LengthUUID(plan.ID).GetValue())
	dashboard := &dashboards.Dashboard{
		Id:          id,
		Name:        typeStringToWrapperspbString(plan.Name),
		Description: typeStringToWrapperspbString(plan.Description),
		Layout:      layout,
		Variables:   variables,
		Filters:     filters,
		Annotations: annotations,
	}

	dashboard, diags = expandDashboardTimeFrame(ctx, dashboard, plan.TimeFrame)
	if diags.HasError() {
		return nil, diags
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

func expandDashboardAutoRefresh(ctx context.Context, dashboard *dashboards.Dashboard, refresh types.Object) (*dashboards.Dashboard, diag.Diagnostics) {
	if refresh.IsNull() || refresh.IsUnknown() {
		return dashboard, nil
	}
	var refreshObject DashboardAutoRefreshModel
	diags := refresh.As(ctx, &refreshObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	switch refreshObject.Type.ValueString() {
	case "two_minutes":
		dashboard.AutoRefresh = &dashboards.Dashboard_TwoMinutes{
			TwoMinutes: &dashboards.Dashboard_AutoRefreshTwoMinutes{},
		}
	case "five_minutes":
		dashboard.AutoRefresh = &dashboards.Dashboard_FiveMinutes{
			FiveMinutes: &dashboards.Dashboard_AutoRefreshFiveMinutes{},
		}
	default:
		dashboard.AutoRefresh = &dashboards.Dashboard_Off{
			Off: &dashboards.Dashboard_AutoRefreshOff{},
		}
	}

	return dashboard, nil
}

func expandDashboardAnnotations(ctx context.Context, annotations types.List) ([]*dashboards.Annotation, diag.Diagnostics) {
	var annotationsObjects []types.Object
	var expandedAnnotations []*dashboards.Annotation
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

func expandAnnotation(ctx context.Context, annotation DashboardAnnotationModel) (*dashboards.Annotation, diag.Diagnostics) {
	source, diags := expandAnnotationSource(ctx, annotation.Source)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.Annotation{
		Id:      expandDashboardIDs(annotation.ID),
		Name:    typeStringToWrapperspbString(annotation.Name),
		Enabled: typeBoolToWrapperspbBool(annotation.Enabled),
		Source:  source,
	}, nil

}

func expandAnnotationSource(ctx context.Context, source types.Object) (*dashboards.Annotation_Source, diag.Diagnostics) {
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
		return &dashboards.Annotation_Source{Value: logsSource}, nil
	case !(sourceObject.Metrics.IsNull() || sourceObject.Metrics.IsUnknown()):
		metricSource, diags := expandMetricSource(ctx, sourceObject.Metrics)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.Annotation_Source{Value: metricSource}, nil
	case !(sourceObject.Spans.IsNull() || sourceObject.Spans.IsUnknown()):
		spansSource, diags := expandSpansSource(ctx, sourceObject.Spans)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.Annotation_Source{Value: spansSource}, nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand Annotation Source", "Annotation Source must be either Logs or Metric")}
	}
}

func expandLogsSource(ctx context.Context, logs types.Object) (*dashboards.Annotation_Source_Logs, diag.Diagnostics) {
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

	labels, diags := expandObservationFields(ctx, logsObject.LabelFields)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.Annotation_Source_Logs{
		Logs: &dashboards.Annotation_LogsSource{
			LuceneQuery:     expandLuceneQuery(logsObject.LuceneQuery),
			Strategy:        strategy,
			MessageTemplate: typeStringToWrapperspbString(logsObject.MessageTemplate),
			LabelFields:     labels,
		},
	}, nil
}

func expandLogsSourceStrategy(ctx context.Context, strategy types.Object) (*dashboards.Annotation_LogsSource_Strategy, diag.Diagnostics) {
	var strategyObject DashboardAnnotationSpanOrLogsStrategyModel
	diags := strategy.As(ctx, &strategyObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	switch {
	case !(strategyObject.Instant.IsNull() || strategyObject.Instant.IsUnknown()):
		return expandLogsSourceInstantStrategy(ctx, strategyObject.Instant)
	case !(strategyObject.Range.IsNull() || strategyObject.Range.IsUnknown()):
		return expandLogsSourceRangeStrategy(ctx, strategyObject.Range)
	case !(strategyObject.Duration.IsNull() || strategyObject.Duration.IsUnknown()):
		return expandLogsSourceDurationStrategy(ctx, strategyObject.Duration)
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand Logs Source Strategy", "Logs Source Strategy must be either Instant, Range or Duration")}
	}
}

func expandLogsSourceDurationStrategy(ctx context.Context, duration types.Object) (*dashboards.Annotation_LogsSource_Strategy, diag.Diagnostics) {
	var durationObject DashboardAnnotationDurationStrategyModel
	diags := duration.As(ctx, &durationObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	startTimestampField, diags := expandObservationFieldObject(ctx, durationObject.StartTimestampField)
	if diags.HasError() {
		return nil, diags
	}

	durationField, diags := expandObservationFieldObject(ctx, durationObject.DurationField)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.Annotation_LogsSource_Strategy{
		Value: &dashboards.Annotation_LogsSource_Strategy_Duration_{
			Duration: &dashboards.Annotation_LogsSource_Strategy_Duration{
				StartTimestampField: startTimestampField,
				DurationField:       durationField,
			},
		},
	}, nil
}

func expandLogsSourceRangeStrategy(ctx context.Context, object types.Object) (*dashboards.Annotation_LogsSource_Strategy, diag.Diagnostics) {
	var rangeObject DashboardAnnotationRangeStrategyModel
	if diags := object.As(ctx, &rangeObject, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	startTimestampField, diags := expandObservationFieldObject(ctx, rangeObject.StartTimestampField)
	if diags.HasError() {
		return nil, diags
	}

	endTimestampField, diags := expandObservationFieldObject(ctx, rangeObject.EndTimestampField)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.Annotation_LogsSource_Strategy{
		Value: &dashboards.Annotation_LogsSource_Strategy_Range_{
			Range: &dashboards.Annotation_LogsSource_Strategy_Range{
				StartTimestampField: startTimestampField,
				EndTimestampField:   endTimestampField,
			},
		},
	}, nil
}

func expandLogsSourceInstantStrategy(ctx context.Context, instant types.Object) (*dashboards.Annotation_LogsSource_Strategy, diag.Diagnostics) {
	var instantObject DashboardAnnotationInstantStrategyModel
	if diags := instant.As(ctx, &instantObject, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	timestampField, diags := expandObservationFieldObject(ctx, instantObject.TimestampField)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.Annotation_LogsSource_Strategy{
		Value: &dashboards.Annotation_LogsSource_Strategy_Instant_{
			Instant: &dashboards.Annotation_LogsSource_Strategy_Instant{
				TimestampField: timestampField,
			},
		},
	}, nil
}

func expandSpansSourceStrategy(ctx context.Context, strategy types.Object) (*dashboards.Annotation_SpansSource_Strategy, diag.Diagnostics) {
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

func expandSpansSourceDurationStrategy(ctx context.Context, duration types.Object) (*dashboards.Annotation_SpansSource_Strategy, diag.Diagnostics) {
	var durationObject DashboardAnnotationDurationStrategyModel
	diags := duration.As(ctx, &durationObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	startTimestampField, diags := expandObservationFieldObject(ctx, durationObject.StartTimestampField)
	if diags.HasError() {
		return nil, diags
	}

	durationField, diags := expandObservationFieldObject(ctx, durationObject.DurationField)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.Annotation_SpansSource_Strategy{
		Value: &dashboards.Annotation_SpansSource_Strategy_Duration_{
			Duration: &dashboards.Annotation_SpansSource_Strategy_Duration{
				StartTimestampField: startTimestampField,
				DurationField:       durationField,
			},
		},
	}, nil
}

func expandSpansSourceRangeStrategy(ctx context.Context, object types.Object) (*dashboards.Annotation_SpansSource_Strategy, diag.Diagnostics) {
	var rangeObject DashboardAnnotationRangeStrategyModel
	if diags := object.As(ctx, &rangeObject, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	startTimestampField, diags := expandObservationFieldObject(ctx, rangeObject.StartTimestampField)
	if diags.HasError() {
		return nil, diags
	}

	endTimestampField, diags := expandObservationFieldObject(ctx, rangeObject.EndTimestampField)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.Annotation_SpansSource_Strategy{
		Value: &dashboards.Annotation_SpansSource_Strategy_Range_{
			Range: &dashboards.Annotation_SpansSource_Strategy_Range{
				StartTimestampField: startTimestampField,
				EndTimestampField:   endTimestampField,
			},
		},
	}, nil
}

func expandSpansSourceInstantStrategy(ctx context.Context, instant types.Object) (*dashboards.Annotation_SpansSource_Strategy, diag.Diagnostics) {
	var instantObject DashboardAnnotationInstantStrategyModel
	if diags := instant.As(ctx, &instantObject, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	timestampField, diags := expandObservationFieldObject(ctx, instantObject.TimestampField)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.Annotation_SpansSource_Strategy{
		Value: &dashboards.Annotation_SpansSource_Strategy_Instant_{
			Instant: &dashboards.Annotation_SpansSource_Strategy_Instant{
				TimestampField: timestampField,
			},
		},
	}, nil
}

func expandSpansSource(ctx context.Context, spans types.Object) (*dashboards.Annotation_Source_Spans, diag.Diagnostics) {
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

	labels, diags := expandObservationFields(ctx, spansObject.LabelFields)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.Annotation_Source_Spans{
		Spans: &dashboards.Annotation_SpansSource{
			LuceneQuery:     expandLuceneQuery(spansObject.LuceneQuery),
			Strategy:        strategy,
			MessageTemplate: typeStringToWrapperspbString(spansObject.MessageTemplate),
			LabelFields:     labels,
		},
	}, nil
}

func expandMetricSource(ctx context.Context, metric types.Object) (*dashboards.Annotation_Source_Metrics, diag.Diagnostics) {
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

	labels, diags := typeStringSliceToWrappedStringSlice(ctx, metricObject.Labels.Elements())
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.Annotation_Source_Metrics{
		Metrics: &dashboards.Annotation_MetricsSource{
			PromqlQuery:     expandPromqlQuery(metricObject.PromqlQuery),
			Strategy:        strategy,
			MessageTemplate: typeStringToWrapperspbString(metricObject.MessageTemplate),
			Labels:          labels,
		},
	}, nil
}

func expandMetricSourceStrategy(ctx context.Context, strategy types.Object) (*dashboards.Annotation_MetricsSource_Strategy, diag.Diagnostics) {
	var strategyObject DashboardAnnotationMetricStrategyModel
	diags := strategy.As(ctx, &strategyObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.Annotation_MetricsSource_Strategy{
		Value: &dashboards.Annotation_MetricsSource_Strategy_StartTimeMetric{
			StartTimeMetric: &dashboards.Annotation_MetricsSource_StartTimeMetric{},
		},
	}, nil
}

func expandDashboardTimeFrame(ctx context.Context, dashboard *dashboards.Dashboard, timeFrame types.Object) (*dashboards.Dashboard, diag.Diagnostics) {
	if timeFrame.IsNull() || timeFrame.IsUnknown() {
		return dashboard, nil
	}
	var timeFrameObject DashboardTimeFrameModel
	diags := timeFrame.As(ctx, &timeFrameObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}
	switch {
	case !(timeFrameObject.Relative.IsNull() || timeFrameObject.Relative.IsUnknown()):
		dashboard.TimeFrame, diags = expandRelativeDashboardTimeFrame(ctx, timeFrameObject.Relative)
	case !(timeFrameObject.Absolute.IsNull() || timeFrameObject.Absolute.IsUnknown()):
		dashboard.TimeFrame, diags = expandAbsoluteDashboardTimeFrame(ctx, timeFrameObject.Absolute)
	default:
		diags = diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand Time Frame", "Dashboard TimeFrame must be either Relative or Absolute")}
	}
	return dashboard, diags
}

func expandDashboardLayout(ctx context.Context, layout types.Object) (*dashboards.Layout, diag.Diagnostics) {
	if layout.IsNull() || layout.IsUnknown() {
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
	return &dashboards.Layout{
		Sections: sections,
	}, nil
}

func expandDashboardSections(ctx context.Context, sections types.List) ([]*dashboards.Section, diag.Diagnostics) {
	var sectionsObjects []types.Object
	var expandedSections []*dashboards.Section
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

func expandSection(ctx context.Context, section SectionModel) (*dashboards.Section, diag.Diagnostics) {
	id := expandDashboardUUID(section.ID)
	rows, diags := expandDashboardRows(ctx, section.Rows)
	if diags.HasError() {
		return nil, diags
	}

	if section.Options != nil {
		options, diags := expandSectionOptions(ctx, *section.Options)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.Section{
			Id:      id,
			Rows:    rows,
			Options: options,
		}, nil
	} else {
		return &dashboards.Section{
			Id:      id,
			Rows:    rows,
			Options: nil,
		}, nil
	}
}

func expandSectionOptions(_ context.Context, option SectionOptionsModel) (*dashboards.SectionOptions, diag.Diagnostics) {

	var color *dashboards.SectionColor
	if !option.Color.IsNull() {
		mappedColor := dashboards.SectionPredefinedColor_value[fmt.Sprintf("SECTION_PREDEFINED_COLOR_%s", strings.ToUpper(option.Color.ValueString()))]
		// this means the color field somehow wasn't validated
		if mappedColor == 0 && option.Color.String() != "unspecified" {
			return nil, diag.Diagnostics{
				diag.NewErrorDiagnostic(
					"Extract Dashboard Section Options Error",
					fmt.Sprintf("Unknown color: %s", option.Color.ValueString()),
				),
			}
		}
		color = &dashboards.SectionColor{
			Value: &dashboards.SectionColor_Predefined{
				Predefined: dashboards.SectionPredefinedColor(mappedColor),
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

	return &dashboards.SectionOptions{
		Value: &dashboards.SectionOptions_Custom{
			Custom: &dashboards.CustomSectionOptions{
				Name:        wrapperspb.String(option.Name.ValueString()),
				Description: description,
				Collapsed:   collapsed,
				Color:       color,
			},
		},
	}, nil
}

func expandDashboardRows(ctx context.Context, rows types.List) ([]*dashboards.Row, diag.Diagnostics) {
	var rowsObjects []types.Object
	var expandedRows []*dashboards.Row
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

func expandRow(ctx context.Context, row RowModel) (*dashboards.Row, diag.Diagnostics) {
	id := expandDashboardUUID(row.ID)
	appearance := &dashboards.Row_Appearance{
		Height: wrapperspb.Int32(int32(row.Height.ValueInt64())),
	}
	widgets, diags := expandDashboardWidgets(ctx, row.Widgets)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.Row{
		Id:         id,
		Appearance: appearance,
		Widgets:    widgets,
	}, nil
}

func expandDashboardWidgets(ctx context.Context, widgets types.List) ([]*dashboards.Widget, diag.Diagnostics) {
	var widgetsObjects []types.Object
	var expandedWidgets []*dashboards.Widget
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

func expandWidget(ctx context.Context, widget WidgetModel) (*dashboards.Widget, diag.Diagnostics) {
	id := expandDashboardUUID(widget.ID)
	title := typeStringToWrapperspbString(widget.Title)
	description := typeStringToWrapperspbString(widget.Description)
	appearance := &dashboards.Widget_Appearance{
		Width: wrapperspb.Int32(int32(widget.Width.ValueInt64())),
	}
	definition, diags := expandWidgetDefinition(ctx, widget.Definition)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.Widget{
		Id:          id,
		Title:       title,
		Description: description,
		Appearance:  appearance,
		Definition:  definition,
	}, nil
}

func expandWidgetDefinition(ctx context.Context, definition *WidgetDefinitionModel) (*dashboards.Widget_Definition, diag.Diagnostics) {
	switch {
	case definition.PieChart != nil:
		return expandPieChart(ctx, definition.PieChart)
	case definition.Gauge != nil:
		return expandGauge(ctx, definition.Gauge)
	case definition.LineChart != nil:
		return expandLineChart(ctx, definition.LineChart)
	case definition.DataTable != nil:
		return expandDataTable(ctx, definition.DataTable)
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

func expandMarkdown(markdown *MarkdownModel) (*dashboards.Widget_Definition, diag.Diagnostics) {
	return &dashboards.Widget_Definition{
		Value: &dashboards.Widget_Definition_Markdown{
			Markdown: &dashboards.Markdown{
				MarkdownText: typeStringToWrapperspbString(markdown.MarkdownText),
				TooltipText:  typeStringToWrapperspbString(markdown.TooltipText),
			},
		},
	}, nil
}

func expandHorizontalBarChart(ctx context.Context, chart *HorizontalBarChartModel) (*dashboards.Widget_Definition, diag.Diagnostics) {
	var diags diag.Diagnostics

	query, diags := expandHorizontalBarChartQuery(ctx, chart.Query)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.Widget_Definition{
		Value: &dashboards.Widget_Definition_HorizontalBarChart{
			HorizontalBarChart: &dashboards.HorizontalBarChart{
				Query:             query,
				StackDefinition:   expandHorizontalBarChartStackDefinition(chart.StackDefinition),
				MaxBarsPerChart:   typeInt64ToWrappedInt32(chart.MaxBarsPerChart),
				ScaleType:         dashboardSchemaToProtoScaleType[chart.ScaleType.ValueString()],
				GroupNameTemplate: typeStringToWrapperspbString(chart.GroupNameTemplate),
				Unit:              dashboardSchemaToProtoUnit[chart.Unit.ValueString()],
				ColorsBy:          expandColorsBy(chart.ColorsBy),
				DisplayOnBar:      typeBoolToWrapperspbBool(chart.DisplayOnBar),
				YAxisViewBy:       expandYAxisViewBy(chart.YAxisViewBy),
				SortBy:            dashboardSchemaToProtoSortBy[chart.SortBy.ValueString()],
				ColorScheme:       typeStringToWrapperspbString(chart.ColorScheme),
				DataModeType:      dashboardSchemaToProtoDataModeType[chart.DataModeType.ValueString()],
			},
		},
	}, nil
}

func expandYAxisViewBy(yAxisViewBy types.String) *dashboards.HorizontalBarChart_YAxisViewBy {
	switch yAxisViewBy.ValueString() {
	case "category":
		return &dashboards.HorizontalBarChart_YAxisViewBy{
			YAxisView: &dashboards.HorizontalBarChart_YAxisViewBy_Category{},
		}
	case "value":
		return &dashboards.HorizontalBarChart_YAxisViewBy{
			YAxisView: &dashboards.HorizontalBarChart_YAxisViewBy_Value{},
		}
	default:
		return nil
	}
}

func expandPieChart(ctx context.Context, pieChart *PieChartModel) (*dashboards.Widget_Definition, diag.Diagnostics) {
	var diags diag.Diagnostics

	query, diags := expandPieChartQuery(ctx, pieChart.Query)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.Widget_Definition{
		Value: &dashboards.Widget_Definition_PieChart{
			PieChart: &dashboards.PieChart{
				Query:              query,
				MaxSlicesPerChart:  typeInt64ToWrappedInt32(pieChart.MaxSlicesPerChart),
				MinSlicePercentage: typeInt64ToWrappedInt32(pieChart.MinSlicePercentage),
				StackDefinition:    expandPieChartStackDefinition(pieChart.StackDefinition),
				LabelDefinition:    expandLabelDefinition(pieChart.LabelDefinition),
				ShowLegend:         typeBoolToWrapperspbBool(pieChart.ShowLegend),
				GroupNameTemplate:  typeStringToWrapperspbString(pieChart.GroupNameTemplate),
				Unit:               dashboardSchemaToProtoUnit[pieChart.Unit.ValueString()],
				ColorScheme:        typeStringToWrapperspbString(pieChart.ColorScheme),
				DataModeType:       dashboardSchemaToProtoDataModeType[pieChart.DataModeType.ValueString()],
			},
		},
	}, nil
}

func expandPieChartStackDefinition(stackDefinition *PieChartStackDefinitionModel) *dashboards.PieChart_StackDefinition {
	if stackDefinition == nil {
		return nil
	}

	return &dashboards.PieChart_StackDefinition{
		MaxSlicesPerStack: typeInt64ToWrappedInt32(stackDefinition.MaxSlicesPerStack),
		StackNameTemplate: typeStringToWrapperspbString(stackDefinition.StackNameTemplate),
	}
}

func expandBarChartStackDefinition(stackDefinition *BarChartStackDefinitionModel) *dashboards.BarChart_StackDefinition {
	if stackDefinition == nil {
		return nil
	}

	return &dashboards.BarChart_StackDefinition{
		MaxSlicesPerBar:   typeInt64ToWrappedInt32(stackDefinition.MaxSlicesPerBar),
		StackNameTemplate: typeStringToWrapperspbString(stackDefinition.StackNameTemplate),
	}
}

func expandHorizontalBarChartStackDefinition(stackDefinition *BarChartStackDefinitionModel) *dashboards.HorizontalBarChart_StackDefinition {
	if stackDefinition == nil {
		return nil
	}

	return &dashboards.HorizontalBarChart_StackDefinition{
		MaxSlicesPerBar:   typeInt64ToWrappedInt32(stackDefinition.MaxSlicesPerBar),
		StackNameTemplate: typeStringToWrapperspbString(stackDefinition.StackNameTemplate),
	}
}

func expandLabelDefinition(labelDefinition *LabelDefinitionModel) *dashboards.PieChart_LabelDefinition {
	if labelDefinition == nil {
		return nil
	}

	return &dashboards.PieChart_LabelDefinition{
		LabelSource:    dashboardSchemaToProtoPieChartLabelSource[labelDefinition.LabelSource.ValueString()],
		IsVisible:      typeBoolToWrapperspbBool(labelDefinition.IsVisible),
		ShowName:       typeBoolToWrapperspbBool(labelDefinition.ShowName),
		ShowValue:      typeBoolToWrapperspbBool(labelDefinition.ShowValue),
		ShowPercentage: typeBoolToWrapperspbBool(labelDefinition.ShowPercentage),
	}
}

func expandGauge(ctx context.Context, gauge *GaugeModel) (*dashboards.Widget_Definition, diag.Diagnostics) {
	var diags diag.Diagnostics

	query, diags := expandGaugeQuery(ctx, gauge.Query)
	if diags.HasError() {
		return nil, diags
	}

	thresholds, diags := expandGaugeThresholds(ctx, gauge.Thresholds)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.Widget_Definition{
		Value: &dashboards.Widget_Definition_Gauge{
			Gauge: &dashboards.Gauge{
				Query:        query,
				Min:          typeFloat64ToWrapperspbDouble(gauge.Min),
				Max:          typeFloat64ToWrapperspbDouble(gauge.Max),
				ShowInnerArc: typeBoolToWrapperspbBool(gauge.ShowInnerArc),
				ShowOuterArc: typeBoolToWrapperspbBool(gauge.ShowOuterArc),
				Unit:         dashboardSchemaToProtoGaugeUnit[gauge.Unit.ValueString()],
				Thresholds:   thresholds,
				DataModeType: dashboardSchemaToProtoDataModeType[gauge.DataModeType.ValueString()],
				ThresholdBy:  dashboardSchemaToProtoGaugeThresholdBy[gauge.ThresholdBy.ValueString()],
			},
		},
	}, nil
}

func expandGaugeThresholds(ctx context.Context, gaugeThresholds types.List) ([]*dashboards.Gauge_Threshold, diag.Diagnostics) {
	var gaugeThresholdsObjects []types.Object
	var expandedGaugeThresholds []*dashboards.Gauge_Threshold
	diags := gaugeThresholds.ElementsAs(ctx, &gaugeThresholdsObjects, true)
	if diags.HasError() {
		return nil, diags
	}
	for _, gto := range gaugeThresholdsObjects {
		var gaugeThreshold GaugeThresholdModel
		if dg := gto.As(ctx, &gaugeThreshold, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expandedGaugeThreshold := expandGaugeThreshold(&gaugeThreshold)
		expandedGaugeThresholds = append(expandedGaugeThresholds, expandedGaugeThreshold)
	}

	return expandedGaugeThresholds, diags
}

func expandGaugeThreshold(gaugeThresholds *GaugeThresholdModel) *dashboards.Gauge_Threshold {
	if gaugeThresholds == nil {
		return nil
	}
	return &dashboards.Gauge_Threshold{
		From:  typeFloat64ToWrapperspbDouble(gaugeThresholds.From),
		Color: typeStringToWrapperspbString(gaugeThresholds.Color),
	}
}

func expandGaugeQuery(ctx context.Context, gaugeQuery *GaugeQueryModel) (*dashboards.Gauge_Query, diag.Diagnostics) {
	switch {
	case gaugeQuery.Metrics != nil:
		metricQuery, diags := expandGaugeQueryMetrics(ctx, gaugeQuery.Metrics)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.Gauge_Query{
			Value: &dashboards.Gauge_Query_Metrics{
				Metrics: metricQuery,
			},
		}, nil
	case gaugeQuery.Logs != nil:
		logQuery, diags := expandGaugeQueryLogs(ctx, gaugeQuery.Logs)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.Gauge_Query{
			Value: &dashboards.Gauge_Query_Logs{
				Logs: logQuery,
			},
		}, nil
	case gaugeQuery.Spans != nil:
		spanQuery, diags := expandGaugeQuerySpans(ctx, gaugeQuery.Spans)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.Gauge_Query{
			Value: &dashboards.Gauge_Query_Spans{
				Spans: spanQuery,
			},
		}, nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Extract Gauge Query Error", fmt.Sprintf("Unknown gauge query type %#v", gaugeQuery))}
	}
}

func expandGaugeQuerySpans(ctx context.Context, gaugeQuerySpans *GaugeQuerySpansModel) (*dashboards.Gauge_SpansQuery, diag.Diagnostics) {
	if gaugeQuerySpans == nil {
		return nil, nil
	}
	filters, diags := expandSpansFilters(ctx, gaugeQuerySpans.Filters)
	if diags.HasError() {
		return nil, diags
	}

	spansAggregation, dg := expandSpansAggregation(gaugeQuerySpans.SpansAggregation)
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	return &dashboards.Gauge_SpansQuery{
		LuceneQuery:      expandLuceneQuery(gaugeQuerySpans.LuceneQuery),
		SpansAggregation: spansAggregation,
		Filters:          filters,
	}, nil
}

func expandSpansAggregations(ctx context.Context, aggregations types.List) ([]*dashboards.SpansAggregation, diag.Diagnostics) {
	var aggregationsObjects []types.Object
	var expandedAggregations []*dashboards.SpansAggregation
	diags := aggregations.ElementsAs(ctx, &aggregationsObjects, true)
	if diags.HasError() {
		return nil, diags
	}
	for _, ao := range aggregationsObjects {
		var aggregation SpansAggregationModel
		if dg := ao.As(ctx, &aggregation, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expandedAggregation, expandDiag := expandSpansAggregation(&aggregation)
		if expandDiag != nil {
			diags.Append(expandDiag)
			continue
		}
		expandedAggregations = append(expandedAggregations, expandedAggregation)
	}

	return expandedAggregations, diags
}

func expandSpansAggregation(spansAggregation *SpansAggregationModel) (*dashboards.SpansAggregation, diag.Diagnostic) {
	if spansAggregation == nil {
		return nil, nil
	}

	switch spansAggregation.Type.ValueString() {
	case "metric":
		return &dashboards.SpansAggregation{
			Aggregation: &dashboards.SpansAggregation_MetricAggregation_{
				MetricAggregation: &dashboards.SpansAggregation_MetricAggregation{
					MetricField:     dashboardSchemaToProtoSpansAggregationMetricField[spansAggregation.Field.ValueString()],
					AggregationType: dashboardSchemaToProtoSpansAggregationMetricAggregationType[spansAggregation.AggregationType.ValueString()],
				},
			},
		}, nil
	case "dimension":
		return &dashboards.SpansAggregation{
			Aggregation: &dashboards.SpansAggregation_DimensionAggregation_{
				DimensionAggregation: &dashboards.SpansAggregation_DimensionAggregation{
					DimensionField:  dashboardProtoToSchemaSpansAggregationDimensionField[spansAggregation.Field.ValueString()],
					AggregationType: dashboardSchemaToProtoSpansAggregationDimensionAggregationType[spansAggregation.AggregationType.ValueString()],
				},
			},
		}, nil
	default:
		return nil, diag.NewErrorDiagnostic("Extract Spans Aggregation Error", fmt.Sprintf("Unknown spans aggregation type %#v", spansAggregation))
	}
}

func expandSpansFilters(ctx context.Context, spansFilters types.List) ([]*dashboards.Filter_SpansFilter, diag.Diagnostics) {
	var spansFiltersObjects []types.Object
	var expandedSpansFilters []*dashboards.Filter_SpansFilter
	diags := spansFilters.ElementsAs(ctx, &spansFiltersObjects, true)
	if diags.HasError() {
		return nil, diags
	}
	for _, sfo := range spansFiltersObjects {
		var spansFilter SpansFilterModel
		if dg := sfo.As(ctx, &spansFilter, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expandedSpansFilter, expandDiags := expandSpansFilter(ctx, spansFilter)
		if expandDiags.HasError() {
			diags.Append(expandDiags...)
			continue
		}
		expandedSpansFilters = append(expandedSpansFilters, expandedSpansFilter)
	}

	return expandedSpansFilters, diags
}

func expandSpansFilter(ctx context.Context, spansFilter SpansFilterModel) (*dashboards.Filter_SpansFilter, diag.Diagnostics) {
	operator, diags := expandFilterOperator(ctx, spansFilter.Operator)
	if diags.HasError() {
		return nil, diags
	}

	field, dg := expandSpansField(spansFilter.Field)
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	return &dashboards.Filter_SpansFilter{
		Field:    field,
		Operator: operator,
	}, nil
}

func expandSpansField(spansFilterField *SpansFieldModel) (*dashboards.SpanField, diag.Diagnostic) {
	if spansFilterField == nil {
		return nil, nil
	}

	switch spansFilterField.Type.ValueString() {
	case "metadata":
		return &dashboards.SpanField{
			Value: &dashboards.SpanField_MetadataField_{
				MetadataField: dashboardSchemaToProtoSpanFieldMetadataField[spansFilterField.Value.ValueString()],
			},
		}, nil
	case "tag":
		return &dashboards.SpanField{
			Value: &dashboards.SpanField_TagField{
				TagField: typeStringToWrapperspbString(spansFilterField.Value),
			},
		}, nil
	case "process_tag":
		return &dashboards.SpanField{
			Value: &dashboards.SpanField_ProcessTagField{
				ProcessTagField: typeStringToWrapperspbString(spansFilterField.Value),
			},
		}, nil
	default:
		return nil, diag.NewErrorDiagnostic("Extract Spans Filter Field Error", fmt.Sprintf("Unknown spans filter field type %s", spansFilterField.Type.ValueString()))
	}
}

func expandMultiSelectSourceQuery(ctx context.Context, sourceQuery types.Object) (*dashboards.MultiSelect_Source, diag.Diagnostics) {
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

	return &dashboards.MultiSelect_Source{
		Value: &dashboards.MultiSelect_Source_Query{
			Query: &dashboards.MultiSelect_QuerySource{
				Query:               query,
				RefreshStrategy:     dashboardSchemaToProtoRefreshStrategy[queryObject.RefreshStrategy.ValueString()],
				ValueDisplayOptions: valueDisplayOptions,
			},
		},
	}, nil
}

func expandMultiSelectQuery(ctx context.Context, query types.Object) (*dashboards.MultiSelect_Query, diag.Diagnostics) {
	if query.IsNull() || query.IsUnknown() {
		return nil, nil
	}

	var queryObject MultiSelectQueryModel
	diags := query.As(ctx, &queryObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	multiSelectQuery := &dashboards.MultiSelect_Query{}
	switch {
	case !(queryObject.Metrics.IsNull() || queryObject.Metrics.IsUnknown()):
		multiSelectQuery.Value, diags = expandMultiSelectMetricsQuery(ctx, queryObject.Metrics)
	case !(queryObject.Logs.IsNull() || queryObject.Logs.IsUnknown()):
		multiSelectQuery.Value, diags = expandMultiSelectLogsQuery(ctx, queryObject.Logs)
	case !(queryObject.Spans.IsNull() || queryObject.Spans.IsUnknown()):
		multiSelectQuery.Value, diags = expandMultiSelectSpansQuery(ctx, queryObject.Spans)
	default:
		diags = diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand MultiSelect Query", "MultiSelect Query must be either Metrics, Logs or Spans")}
	}

	if diags.HasError() {
		return nil, diags
	}

	return multiSelectQuery, nil
}

func expandMultiSelectValueDisplayOptions(ctx context.Context, options types.Object) (*dashboards.MultiSelect_ValueDisplayOptions, diag.Diagnostics) {
	if options.IsNull() || options.IsUnknown() {
		return nil, nil
	}

	var optionsObject MultiSelectValueDisplayOptionsModel
	diags := options.As(ctx, &optionsObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.MultiSelect_ValueDisplayOptions{
		ValueRegex: typeStringToWrapperspbString(optionsObject.ValueRegex),
		LabelRegex: typeStringToWrapperspbString(optionsObject.LabelRegex),
	}, nil
}

func expandMultiSelectLogsQuery(ctx context.Context, logs types.Object) (*dashboards.MultiSelect_Query_LogsQuery_, diag.Diagnostics) {
	if logs.IsNull() || logs.IsUnknown() {
		return nil, nil
	}

	var logsObject MultiSelectLogsQueryModel
	diags := logs.As(ctx, &logsObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	logsQuery := &dashboards.MultiSelect_Query_LogsQuery_{
		LogsQuery: &dashboards.MultiSelect_Query_LogsQuery{
			Type: &dashboards.MultiSelect_Query_LogsQuery_Type{},
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

func expandMultiSelectLogsQueryTypeFieldName(ctx context.Context, name types.Object) (*dashboards.MultiSelect_Query_LogsQuery_Type_FieldName_, diag.Diagnostics) {
	if name.IsNull() || name.IsUnknown() {
		return nil, nil
	}

	var nameObject LogFieldNameModel
	diags := name.As(ctx, &nameObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.MultiSelect_Query_LogsQuery_Type_FieldName_{
		FieldName: &dashboards.MultiSelect_Query_LogsQuery_Type_FieldName{
			LogRegex: typeStringToWrapperspbString(nameObject.LogRegex),
		},
	}, nil
}

func expandMultiSelectLogsQueryTypFieldValue(ctx context.Context, value types.Object) (*dashboards.MultiSelect_Query_LogsQuery_Type_FieldValue_, diag.Diagnostics) {
	if value.IsNull() || value.IsUnknown() {
		return nil, nil
	}

	var valueObject FieldValueModel
	diags := value.As(ctx, &valueObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	observationField, diags := expandObservationFieldObject(ctx, valueObject.ObservationField)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.MultiSelect_Query_LogsQuery_Type_FieldValue_{
		FieldValue: &dashboards.MultiSelect_Query_LogsQuery_Type_FieldValue{
			ObservationField: observationField,
		},
	}, nil
}

func expandMultiSelectMetricsQuery(ctx context.Context, metrics types.Object) (*dashboards.MultiSelect_Query_MetricsQuery_, diag.Diagnostics) {
	if metrics.IsNull() || metrics.IsUnknown() {
		return nil, nil
	}

	var metricsObject MultiSelectMetricsQueryModel
	diags := metrics.As(ctx, &metricsObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	metricsQuery := &dashboards.MultiSelect_Query_MetricsQuery_{
		MetricsQuery: &dashboards.MultiSelect_Query_MetricsQuery{
			Type: &dashboards.MultiSelect_Query_MetricsQuery_Type{},
		},
	}

	switch {
	case !(metricsObject.MetricName.IsNull() || metricsObject.MetricName.IsUnknown()):
		metricsQuery.MetricsQuery.Type.Value, diags = expandMultiSelectMetricsQueryTypeMetricName(ctx, metricsObject.MetricName)
	case !(metricsObject.LabelName.IsNull() || metricsObject.LabelName.IsUnknown()):
		metricsQuery.MetricsQuery.Type.Value, diags = expandMultiSelectMetricsQueryTypeLabelName(ctx, metricsObject.LabelName)
	case !(metricsObject.LabelValue.IsNull() || metricsObject.LabelValue.IsUnknown()):
		metricsQuery.MetricsQuery.Type.Value, diags = expandMultiSelectMetricsQueryTypeLabelValue(ctx, metricsObject.LabelValue)
	}

	if diags.HasError() {
		return nil, diags
	}

	return metricsQuery, nil
}

func expandMultiSelectMetricsQueryTypeMetricName(ctx context.Context, name types.Object) (*dashboards.MultiSelect_Query_MetricsQuery_Type_MetricName_, diag.Diagnostics) {
	if name.IsNull() || name.IsUnknown() {
		return nil, nil
	}

	var nameObject MetricAndLabelNameModel
	diags := name.As(ctx, &nameObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.MultiSelect_Query_MetricsQuery_Type_MetricName_{
		MetricName: &dashboards.MultiSelect_Query_MetricsQuery_Type_MetricName{
			MetricRegex: typeStringToWrapperspbString(nameObject.MetricRegex),
		},
	}, nil
}

func expandMultiSelectMetricsQueryTypeLabelName(ctx context.Context, name types.Object) (*dashboards.MultiSelect_Query_MetricsQuery_Type_LabelName_, diag.Diagnostics) {
	if name.IsNull() || name.IsUnknown() {
		return nil, nil
	}

	var nameObject MetricAndLabelNameModel
	diags := name.As(ctx, &nameObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.MultiSelect_Query_MetricsQuery_Type_LabelName_{
		LabelName: &dashboards.MultiSelect_Query_MetricsQuery_Type_LabelName{
			MetricRegex: typeStringToWrapperspbString(nameObject.MetricRegex),
		},
	}, nil
}

func expandMultiSelectMetricsQueryTypeLabelValue(ctx context.Context, value types.Object) (*dashboards.MultiSelect_Query_MetricsQuery_Type_LabelValue_, diag.Diagnostics) {
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

	return &dashboards.MultiSelect_Query_MetricsQuery_Type_LabelValue_{
		LabelValue: &dashboards.MultiSelect_Query_MetricsQuery_Type_LabelValue{
			MetricName:   metricName,
			LabelName:    labelName,
			LabelFilters: labelFilters,
		},
	}, nil
}

func expandStringOrVariables(ctx context.Context, name types.List) ([]*dashboards.MultiSelect_Query_MetricsQuery_StringOrVariable, diag.Diagnostics) {
	var nameObjects []types.Object
	var expandedNames []*dashboards.MultiSelect_Query_MetricsQuery_StringOrVariable
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

func expandStringOrVariable(ctx context.Context, name types.Object) (*dashboards.MultiSelect_Query_MetricsQuery_StringOrVariable, diag.Diagnostics) {
	if name.IsNull() || name.IsUnknown() {
		return nil, nil
	}

	var nameObject MetricLabelFilterOperatorSelectedValuesModel
	diags := name.As(ctx, &nameObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	switch {
	case !(nameObject.VariableName.IsNull() || nameObject.VariableName.IsUnknown()):
		return &dashboards.MultiSelect_Query_MetricsQuery_StringOrVariable{
			Value: &dashboards.MultiSelect_Query_MetricsQuery_StringOrVariable_VariableName{
				VariableName: typeStringToWrapperspbString(nameObject.VariableName),
			},
		}, nil
	case !(nameObject.StringValue.IsNull() || nameObject.StringValue.IsUnknown()):
		return &dashboards.MultiSelect_Query_MetricsQuery_StringOrVariable{
			Value: &dashboards.MultiSelect_Query_MetricsQuery_StringOrVariable_StringValue{
				StringValue: typeStringToWrapperspbString(nameObject.StringValue),
			},
		}, nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand StringOrVariable", "StringOrVariable must be either VariableName or StringValue")}
	}
}

func expandMetricsLabelFilters(ctx context.Context, filters types.List) ([]*dashboards.MultiSelect_Query_MetricsQuery_MetricsLabelFilter, diag.Diagnostics) {
	var filtersObjects []types.Object
	var expandedFilters []*dashboards.MultiSelect_Query_MetricsQuery_MetricsLabelFilter
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

func expandMetricLabelFilter(ctx context.Context, filter MetricLabelFilterModel) (*dashboards.MultiSelect_Query_MetricsQuery_MetricsLabelFilter, diag.Diagnostics) {
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

	return &dashboards.MultiSelect_Query_MetricsQuery_MetricsLabelFilter{
		Metric:   metric,
		Label:    label,
		Operator: operator,
	}, nil
}

func expandMetricLabelFilterOperator(ctx context.Context, operator types.Object) (*dashboards.MultiSelect_Query_MetricsQuery_Operator, diag.Diagnostics) {
	if operator.IsNull() || operator.IsUnknown() {
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

	selection := &dashboards.MultiSelect_Query_MetricsQuery_Selection{
		Value: &dashboards.MultiSelect_Query_MetricsQuery_Selection_List{
			List: &dashboards.MultiSelect_Query_MetricsQuery_Selection_ListSelection{
				Values: values,
			},
		},
	}
	switch operatorObject.Type.ValueString() {
	case "equals":
		return &dashboards.MultiSelect_Query_MetricsQuery_Operator{
			Value: &dashboards.MultiSelect_Query_MetricsQuery_Operator_Equals{
				Equals: &dashboards.MultiSelect_Query_MetricsQuery_Equals{
					Selection: selection,
				},
			},
		}, nil
	case "not_equals":
		return &dashboards.MultiSelect_Query_MetricsQuery_Operator{
			Value: &dashboards.MultiSelect_Query_MetricsQuery_Operator_NotEquals{
				NotEquals: &dashboards.MultiSelect_Query_MetricsQuery_NotEquals{
					Selection: selection,
				},
			},
		}, nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand MetricLabelFilterOperator", fmt.Sprintf("Unknown operator type %s", operatorObject.Type.ValueString()))}
	}
}

func expandMultiSelectSpansQuery(ctx context.Context, spans types.Object) (*dashboards.MultiSelect_Query_SpansQuery_, diag.Diagnostics) {
	if spans.IsNull() || spans.IsUnknown() {
		return nil, nil
	}

	var spansObject MultiSelectSpansQueryModel
	diags := spans.As(ctx, &spansObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	spansQuery := &dashboards.MultiSelect_Query_SpansQuery_{
		SpansQuery: &dashboards.MultiSelect_Query_SpansQuery{
			Type: &dashboards.MultiSelect_Query_SpansQuery_Type{},
		},
	}

	switch {
	case !(spansObject.FieldName.IsNull() || spansObject.FieldName.IsUnknown()):
		spansQuery.SpansQuery.Type.Value, diags = expandMultiSelectSpansQueryTypeFieldName(ctx, spansObject.FieldName)
	case !(spansObject.FieldValue.IsNull() || spansObject.FieldValue.IsUnknown()):
		spansQuery.SpansQuery.Type.Value, diags = expandMultiSelectSpansQueryTypeFieldValue(ctx, spansObject.FieldValue)
	default:
		diags = diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand MultiSelect Spans Query", "MultiSelect Spans Query must be either FieldName or FieldValue")}
	}

	if diags.HasError() {
		return nil, diags
	}

	return spansQuery, nil
}

func expandMultiSelectSpansQueryTypeFieldName(ctx context.Context, name types.Object) (*dashboards.MultiSelect_Query_SpansQuery_Type_FieldName_, diag.Diagnostics) {
	if name.IsNull() || name.IsUnknown() {
		return nil, nil
	}

	var nameObject SpanFieldNameModel
	diags := name.As(ctx, &nameObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.MultiSelect_Query_SpansQuery_Type_FieldName_{
		FieldName: &dashboards.MultiSelect_Query_SpansQuery_Type_FieldName{
			SpanRegex: typeStringToWrapperspbString(nameObject.SpanRegex),
		},
	}, nil
}

func expandMultiSelectSpansQueryTypeFieldValue(ctx context.Context, value types.Object) (*dashboards.MultiSelect_Query_SpansQuery_Type_FieldValue_, diag.Diagnostics) {
	if value.IsNull() || value.IsUnknown() {
		return nil, nil
	}

	var valueObject SpansFieldModel
	diags := value.As(ctx, &valueObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	spansField, dgs := expandSpansField(&valueObject)
	if dgs != nil {
		return nil, diag.Diagnostics{dgs}
	}

	return &dashboards.MultiSelect_Query_SpansQuery_Type_FieldValue_{
		FieldValue: &dashboards.MultiSelect_Query_SpansQuery_Type_FieldValue{
			Value: spansField,
		},
	}, nil
}

func expandGaugeQueryMetrics(ctx context.Context, gaugeQueryMetrics *GaugeQueryMetricsModel) (*dashboards.Gauge_MetricsQuery, diag.Diagnostics) {
	filters, diags := expandMetricsFilters(ctx, gaugeQueryMetrics.Filters)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.Gauge_MetricsQuery{
		PromqlQuery: expandPromqlQuery(gaugeQueryMetrics.PromqlQuery),
		Aggregation: dashboardSchemaToProtoGaugeAggregation[gaugeQueryMetrics.Aggregation.ValueString()],
		Filters:     filters,
	}, nil
}

func expandMetricsFilters(ctx context.Context, metricFilters types.List) ([]*dashboards.Filter_MetricsFilter, diag.Diagnostics) {
	var metricFiltersObjects []types.Object
	var expandedMetricFilters []*dashboards.Filter_MetricsFilter
	diags := metricFilters.ElementsAs(ctx, &metricFiltersObjects, true)
	if diags.HasError() {
		return nil, diags
	}
	for _, mfo := range metricFiltersObjects {
		var metricsFilter MetricsFilterModel
		if dg := mfo.As(ctx, &metricsFilter, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expandedMetricFilter, expandDiags := expandMetricFilter(ctx, metricsFilter)
		if expandDiags.HasError() {
			diags.Append(expandDiags...)
			continue
		}
		expandedMetricFilters = append(expandedMetricFilters, expandedMetricFilter)
	}

	return expandedMetricFilters, diags
}

func expandMetricFilter(ctx context.Context, metricFilter MetricsFilterModel) (*dashboards.Filter_MetricsFilter, diag.Diagnostics) {
	operator, diags := expandFilterOperator(ctx, metricFilter.Operator)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.Filter_MetricsFilter{
		Metric:   typeStringToWrapperspbString(metricFilter.Metric),
		Label:    typeStringToWrapperspbString(metricFilter.Label),
		Operator: operator,
	}, nil
}

func expandFilterOperator(ctx context.Context, operator *FilterOperatorModel) (*dashboards.Filter_Operator, diag.Diagnostics) {
	if operator == nil {
		return nil, nil
	}

	selectedValues, diags := typeStringSliceToWrappedStringSlice(ctx, operator.SelectedValues.Elements())
	if diags.HasError() {
		return nil, diags
	}

	switch operator.Type.ValueString() {
	case "equals":
		filterOperator := &dashboards.Filter_Operator{
			Value: &dashboards.Filter_Operator_Equals{
				Equals: &dashboards.Filter_Equals{
					Selection: &dashboards.Filter_Equals_Selection{},
				},
			},
		}
		if len(selectedValues) != 0 {
			filterOperator.GetEquals().Selection.Value = &dashboards.Filter_Equals_Selection_List{
				List: &dashboards.Filter_Equals_Selection_ListSelection{
					Values: selectedValues,
				},
			}
		} else {
			filterOperator.GetEquals().Selection.Value = &dashboards.Filter_Equals_Selection_All{
				All: &dashboards.Filter_Equals_Selection_AllSelection{},
			}
		}
		return filterOperator, nil
	case "not_equals":
		return &dashboards.Filter_Operator{
			Value: &dashboards.Filter_Operator_NotEquals{
				NotEquals: &dashboards.Filter_NotEquals{
					Selection: &dashboards.Filter_NotEquals_Selection{
						Value: &dashboards.Filter_NotEquals_Selection_List{
							List: &dashboards.Filter_NotEquals_Selection_ListSelection{
								Values: selectedValues,
							},
						},
					},
				},
			},
		}, nil
	default:
		diags.Append(diag.NewErrorDiagnostic(
			"Error expand filter operator",
			fmt.Sprintf("unknown filter operator type %s", operator.Type.ValueString())))
		return nil, diags
	}
}

func expandPromqlQuery(promqlQuery types.String) *dashboards.PromQlQuery {
	if promqlQuery.IsNull() || promqlQuery.IsUnknown() {
		return nil
	}

	return &dashboards.PromQlQuery{
		Value: wrapperspb.String(promqlQuery.ValueString()),
	}
}

func expandGaugeQueryLogs(ctx context.Context, gaugeQueryLogs *GaugeQueryLogsModel) (*dashboards.Gauge_LogsQuery, diag.Diagnostics) {
	logsAggregation, diags := expandLogsAggregation(ctx, gaugeQueryLogs.LogsAggregation)
	if diags.HasError() {
		return nil, diags
	}

	filters, diags := expandLogsFilters(ctx, gaugeQueryLogs.Filters)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.Gauge_LogsQuery{
		LuceneQuery:     expandLuceneQuery(gaugeQueryLogs.LuceneQuery),
		LogsAggregation: logsAggregation,
		Filters:         filters,
	}, nil
}

func expandLuceneQuery(luceneQuery types.String) *dashboards.LuceneQuery {
	if luceneQuery.IsNull() || luceneQuery.IsUnknown() {
		return nil
	}
	return &dashboards.LuceneQuery{
		Value: wrapperspb.String(luceneQuery.ValueString()),
	}
}

func expandLogsAggregations(ctx context.Context, logsAggregations types.List) ([]*dashboards.LogsAggregation, diag.Diagnostics) {
	var logsAggregationsObjects []types.Object
	var expandedLogsAggregations []*dashboards.LogsAggregation
	diags := logsAggregations.ElementsAs(ctx, &logsAggregationsObjects, true)
	if diags.HasError() {
		return nil, diags
	}
	for _, qdo := range logsAggregationsObjects {
		var aggregation LogsAggregationModel
		if dg := qdo.As(ctx, &aggregation, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expandedLogsAggregation, expandDiags := expandLogsAggregation(ctx, &aggregation)
		if expandDiags.HasError() {
			diags.Append(expandDiags...)
			continue
		}
		expandedLogsAggregations = append(expandedLogsAggregations, expandedLogsAggregation)
	}

	return expandedLogsAggregations, diags
}

func expandLogsAggregation(ctx context.Context, logsAggregation *LogsAggregationModel) (*dashboards.LogsAggregation, diag.Diagnostics) {
	if logsAggregation == nil {
		return nil, nil
	}
	switch logsAggregation.Type.ValueString() {
	case "count":
		return &dashboards.LogsAggregation{
			Value: &dashboards.LogsAggregation_Count_{
				Count: &dashboards.LogsAggregation_Count{},
			},
		}, nil
	case "count_distinct":
		observationField, diags := expandObservationFieldObject(ctx, logsAggregation.ObservationField)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.LogsAggregation{
			Value: &dashboards.LogsAggregation_CountDistinct_{
				CountDistinct: &dashboards.LogsAggregation_CountDistinct{
					Field:            typeStringToWrapperspbString(logsAggregation.Field),
					ObservationField: observationField,
				},
			},
		}, nil
	case "sum":
		observationField, diags := expandObservationFieldObject(ctx, logsAggregation.ObservationField)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.LogsAggregation{
			Value: &dashboards.LogsAggregation_Sum_{
				Sum: &dashboards.LogsAggregation_Sum{
					Field:            typeStringToWrapperspbString(logsAggregation.Field),
					ObservationField: observationField,
				},
			},
		}, nil
	case "avg":
		observationField, diags := expandObservationFieldObject(ctx, logsAggregation.ObservationField)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.LogsAggregation{
			Value: &dashboards.LogsAggregation_Average_{
				Average: &dashboards.LogsAggregation_Average{
					Field:            typeStringToWrapperspbString(logsAggregation.Field),
					ObservationField: observationField,
				},
			},
		}, nil
	case "min":
		observationField, diags := expandObservationFieldObject(ctx, logsAggregation.ObservationField)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.LogsAggregation{
			Value: &dashboards.LogsAggregation_Min_{
				Min: &dashboards.LogsAggregation_Min{
					Field:            typeStringToWrapperspbString(logsAggregation.Field),
					ObservationField: observationField,
				},
			},
		}, nil
	case "max":
		observationField, diags := expandObservationFieldObject(ctx, logsAggregation.ObservationField)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.LogsAggregation{
			Value: &dashboards.LogsAggregation_Max_{
				Max: &dashboards.LogsAggregation_Max{
					Field:            typeStringToWrapperspbString(logsAggregation.Field),
					ObservationField: observationField,
				},
			},
		}, nil
	case "percentile":
		observationField, diags := expandObservationFieldObject(ctx, logsAggregation.ObservationField)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.LogsAggregation{
			Value: &dashboards.LogsAggregation_Percentile_{
				Percentile: &dashboards.LogsAggregation_Percentile{
					Field:            typeStringToWrapperspbString(logsAggregation.Field),
					Percent:          typeFloat64ToWrapperspbDouble(logsAggregation.Percent),
					ObservationField: observationField,
				},
			},
		}, nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error expand logs aggregation", fmt.Sprintf("unknown logs aggregation type %s", logsAggregation.Type.ValueString()))}
	}
}

func expandLogsFilters(ctx context.Context, logsFilters types.List) ([]*dashboards.Filter_LogsFilter, diag.Diagnostics) {
	var filtersObjects []types.Object
	var expandedFilters []*dashboards.Filter_LogsFilter
	diags := logsFilters.ElementsAs(ctx, &filtersObjects, true)
	if diags.HasError() {
		return nil, diags
	}

	for _, fo := range filtersObjects {
		var filter LogsFilterModel
		if dg := fo.As(ctx, &filter, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expandedFilter, expandDiags := expandLogsFilter(ctx, filter)
		if expandDiags.HasError() {
			diags.Append(expandDiags...)
			continue
		}
		expandedFilters = append(expandedFilters, expandedFilter)
	}

	return expandedFilters, diags
}

func expandLogsFilter(ctx context.Context, logsFilter LogsFilterModel) (*dashboards.Filter_LogsFilter, diag.Diagnostics) {
	operator, diags := expandFilterOperator(ctx, logsFilter.Operator)
	if diags.HasError() {
		return nil, diags
	}

	observationField, diags := expandObservationFieldObject(ctx, logsFilter.ObservationField)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.Filter_LogsFilter{
		Field:            typeStringToWrapperspbString(logsFilter.Field),
		Operator:         operator,
		ObservationField: observationField,
	}, nil
}

func expandBarChart(ctx context.Context, chart *BarChartModel) (*dashboards.Widget_Definition, diag.Diagnostics) {
	var diags diag.Diagnostics

	query, diags := expandBarChartQuery(ctx, chart.Query)
	if diags.HasError() {
		return nil, diags
	}

	xaxis, dg := expandXAis(chart.XAxis)
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	return &dashboards.Widget_Definition{
		Value: &dashboards.Widget_Definition_BarChart{
			BarChart: &dashboards.BarChart{
				Query:             query,
				MaxBarsPerChart:   typeInt64ToWrappedInt32(chart.MaxBarsPerChart),
				GroupNameTemplate: typeStringToWrapperspbString(chart.GroupNameTemplate),
				StackDefinition:   expandBarChartStackDefinition(chart.StackDefinition),
				ScaleType:         dashboardSchemaToProtoScaleType[chart.ScaleType.ValueString()],
				ColorsBy:          expandColorsBy(chart.ColorsBy),
				XAxis:             xaxis,
				Unit:              dashboardSchemaToProtoUnit[chart.Unit.ValueString()],
				SortBy:            dashboardSchemaToProtoSortBy[chart.SortBy.ValueString()],
				ColorScheme:       typeStringToWrapperspbString(chart.ColorScheme),
				DataModeType:      dashboardSchemaToProtoDataModeType[chart.DataModeType.ValueString()],
			},
		},
	}, nil
}

func expandColorsBy(colorsBy types.String) *dashboards.ColorsBy {
	switch colorsBy.ValueString() {
	case "stack":
		return &dashboards.ColorsBy{
			Value: &dashboards.ColorsBy_Stack{
				Stack: &dashboards.ColorsBy_ColorsByStack{},
			},
		}
	case "group_by":
		return &dashboards.ColorsBy{
			Value: &dashboards.ColorsBy_GroupBy{
				GroupBy: &dashboards.ColorsBy_ColorsByGroupBy{},
			},
		}
	case "aggregation":
		return &dashboards.ColorsBy{
			Value: &dashboards.ColorsBy_Aggregation{
				Aggregation: &dashboards.ColorsBy_ColorsByAggregation{},
			},
		}
	default:
		return nil
	}
}

func expandXAis(xaxis *BarChartXAxisModel) (*dashboards.BarChart_XAxis, diag.Diagnostic) {
	if xaxis == nil {
		return nil, nil
	}

	switch {
	case xaxis.Time != nil:
		duration, err := time.ParseDuration(xaxis.Time.Interval.ValueString())
		if err != nil {
			return nil, diag.NewErrorDiagnostic("Error expand bar chart x axis", err.Error())
		}
		return &dashboards.BarChart_XAxis{
			Type: &dashboards.BarChart_XAxis_Time{
				Time: &dashboards.BarChart_XAxis_XAxisByTime{
					Interval:         durationpb.New(duration),
					BucketsPresented: typeInt64ToWrappedInt32(xaxis.Time.BucketsPresented),
				},
			},
		}, nil
	case xaxis.Value != nil:
		return &dashboards.BarChart_XAxis{
			Type: &dashboards.BarChart_XAxis_Value{
				Value: &dashboards.BarChart_XAxis_XAxisByValue{},
			},
		}, nil
	default:
		return nil, diag.NewErrorDiagnostic("Error expand bar chart x axis", "unknown x axis type")
	}
}
func expandBarChartQuery(ctx context.Context, query *BarChartQueryModel) (*dashboards.BarChart_Query, diag.Diagnostics) {
	if query == nil {
		return nil, nil
	}
	switch {
	case !(query.Logs.IsNull() || query.Logs.IsUnknown()):
		logsQuery, diags := expandBarChartLogsQuery(ctx, query.Logs)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.BarChart_Query{
			Value: &dashboards.BarChart_Query_Logs{
				Logs: logsQuery,
			},
		}, nil
	case !(query.Metrics.IsNull() || query.Metrics.IsUnknown()):
		metricsQuery, diags := expandBarChartMetricsQuery(ctx, query.Metrics)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.BarChart_Query{
			Value: &dashboards.BarChart_Query_Metrics{
				Metrics: metricsQuery,
			},
		}, nil
	case !(query.Spans.IsNull() || query.Spans.IsUnknown()):
		spansQuery, diags := expandBarChartSpansQuery(ctx, query.Spans)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.BarChart_Query{
			Value: &dashboards.BarChart_Query_Spans{
				Spans: spansQuery,
			},
		}, nil
	case !(query.DataPrime.IsNull() || query.DataPrime.IsUnknown()):
		dataPrimeQuery, diags := expandBarChartDataPrimeQuery(ctx, query.DataPrime)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.BarChart_Query{
			Value: &dashboards.BarChart_Query_Dataprime{
				Dataprime: dataPrimeQuery,
			},
		}, nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error expand bar chart query", "unknown bar chart query type")}
	}
}

func expandHorizontalBarChartQuery(ctx context.Context, query *HorizontalBarChartQueryModel) (*dashboards.HorizontalBarChart_Query, diag.Diagnostics) {
	if query == nil {
		return nil, nil
	}
	switch {
	case !(query.Logs.IsNull() || query.Logs.IsUnknown()):
		logsQuery, diags := expandHorizontalBarChartLogsQuery(ctx, query.Logs)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.HorizontalBarChart_Query{
			Value: &dashboards.HorizontalBarChart_Query_Logs{
				Logs: logsQuery,
			},
		}, nil
	case !(query.Metrics.IsNull() || query.Metrics.IsUnknown()):
		metricsQuery, diags := expandHorizontalBarChartMetricsQuery(ctx, query.Metrics)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.HorizontalBarChart_Query{
			Value: &dashboards.HorizontalBarChart_Query_Metrics{
				Metrics: metricsQuery,
			},
		}, nil
	case !(query.Spans.IsNull() || query.Spans.IsUnknown()):
		spansQuery, diags := expandHorizontalBarChartSpansQuery(ctx, query.Spans)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.HorizontalBarChart_Query{
			Value: &dashboards.HorizontalBarChart_Query_Spans{
				Spans: spansQuery,
			},
		}, nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error expand bar chart query", "unknown bar chart query type")}
	}
}

func expandHorizontalBarChartLogsQuery(ctx context.Context, logs types.Object) (*dashboards.HorizontalBarChart_LogsQuery, diag.Diagnostics) {
	if logs.IsNull() || logs.IsUnknown() {
		return nil, nil
	}

	var logsObject BarChartQueryLogsModel
	diags := logs.As(ctx, &logsObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	aggregation, diags := expandLogsAggregation(ctx, logsObject.Aggregation)
	if diags.HasError() {
		return nil, diags
	}

	filters, diags := expandLogsFilters(ctx, logsObject.Filters)
	if diags.HasError() {
		return nil, diags
	}

	groupNames, diags := typeStringSliceToWrappedStringSlice(ctx, logsObject.GroupNames.Elements())
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.HorizontalBarChart_LogsQuery{
		LuceneQuery:      expandLuceneQuery(logsObject.LuceneQuery),
		Aggregation:      aggregation,
		Filters:          filters,
		GroupNames:       groupNames,
		StackedGroupName: typeStringToWrapperspbString(logsObject.StackedGroupName),
	}, nil
}

func expandHorizontalBarChartMetricsQuery(ctx context.Context, metrics types.Object) (*dashboards.HorizontalBarChart_MetricsQuery, diag.Diagnostics) {
	if metrics.IsNull() || metrics.IsUnknown() {
		return nil, nil
	}

	var metricsObject BarChartQueryMetricsModel
	diags := metrics.As(ctx, &metricsObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	filters, diags := expandMetricsFilters(ctx, metricsObject.Filters)
	if diags.HasError() {
		return nil, diags
	}

	groupNames, diags := typeStringSliceToWrappedStringSlice(ctx, metricsObject.GroupNames.Elements())
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.HorizontalBarChart_MetricsQuery{
		PromqlQuery:      expandPromqlQuery(metricsObject.PromqlQuery),
		Filters:          filters,
		GroupNames:       groupNames,
		StackedGroupName: typeStringToWrapperspbString(metricsObject.StackedGroupName),
	}, nil
}

func expandHorizontalBarChartSpansQuery(ctx context.Context, spans types.Object) (*dashboards.HorizontalBarChart_SpansQuery, diag.Diagnostics) {
	if spans.IsNull() || spans.IsUnknown() {
		return nil, nil
	}

	var spansObject BarChartQuerySpansModel
	diags := spans.As(ctx, &spansObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	aggregation, dg := expandSpansAggregation(spansObject.Aggregation)
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	filters, diags := expandSpansFilters(ctx, spansObject.Filters)
	if diags.HasError() {
		return nil, diags
	}

	groupNames, diags := expandSpansFields(ctx, spansObject.GroupNames)
	if diags.HasError() {
		return nil, diags
	}

	expandedFilter, dg := expandSpansField(spansObject.StackedGroupName)
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	return &dashboards.HorizontalBarChart_SpansQuery{
		LuceneQuery:      expandLuceneQuery(spansObject.LuceneQuery),
		Aggregation:      aggregation,
		Filters:          filters,
		GroupNames:       groupNames,
		StackedGroupName: expandedFilter,
	}, nil
}

func expandBarChartLogsQuery(ctx context.Context, barChartQueryLogs types.Object) (*dashboards.BarChart_LogsQuery, diag.Diagnostics) {
	if barChartQueryLogs.IsNull() || barChartQueryLogs.IsUnknown() {
		return nil, nil
	}

	var barChartQueryLogsObject BarChartQueryLogsModel
	diags := barChartQueryLogs.As(ctx, &barChartQueryLogsObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	aggregation, diags := expandLogsAggregation(ctx, barChartQueryLogsObject.Aggregation)
	if diags.HasError() {
		return nil, diags
	}

	filters, diags := expandLogsFilters(ctx, barChartQueryLogsObject.Filters)
	if diags.HasError() {
		return nil, diags
	}

	groupNames, diags := typeStringSliceToWrappedStringSlice(ctx, barChartQueryLogsObject.GroupNames.Elements())
	if diags.HasError() {
		return nil, diags
	}

	groupNamesFields, diags := expandObservationFields(ctx, barChartQueryLogsObject.GroupNamesFields)
	if diags.HasError() {
		return nil, diags
	}

	stackedGroupNameField, diags := expandObservationFieldObject(ctx, barChartQueryLogsObject.StackedGroupNameField)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.BarChart_LogsQuery{
		LuceneQuery:           expandLuceneQuery(barChartQueryLogsObject.LuceneQuery),
		Aggregation:           aggregation,
		Filters:               filters,
		GroupNames:            groupNames,
		StackedGroupName:      typeStringToWrapperspbString(barChartQueryLogsObject.StackedGroupName),
		GroupNamesFields:      groupNamesFields,
		StackedGroupNameField: stackedGroupNameField,
	}, nil
}

func expandObservationFields(ctx context.Context, namesFields types.List) ([]*dashboards.ObservationField, diag.Diagnostics) {
	var namesFieldsObjects []types.Object
	var expandedNamesFields []*dashboards.ObservationField
	diags := namesFields.ElementsAs(ctx, &namesFieldsObjects, true)
	if diags.HasError() {
		return nil, diags
	}
	for _, nfo := range namesFieldsObjects {
		var namesField ObservationFieldModel
		if dg := nfo.As(ctx, &namesField, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expandedNamesField, expandDiags := expandObservationField(ctx, namesField)
		if expandDiags != nil {
			diags.Append(expandDiags...)
			continue
		}
		expandedNamesFields = append(expandedNamesFields, expandedNamesField)
	}

	return expandedNamesFields, diags
}

func expandObservationFieldObject(ctx context.Context, field types.Object) (*dashboards.ObservationField, diag.Diagnostics) {
	if field.IsNull() || field.IsUnknown() {
		return nil, nil
	}

	var observationField ObservationFieldModel
	if dg := field.As(ctx, &observationField, basetypes.ObjectAsOptions{}); dg.HasError() {
		return nil, dg
	}

	return expandObservationField(ctx, observationField)
}

func expandObservationField(ctx context.Context, observationField ObservationFieldModel) (*dashboards.ObservationField, diag.Diagnostics) {
	keypath, dg := typeStringSliceToWrappedStringSlice(ctx, observationField.Keypath.Elements())
	if dg.HasError() {
		return nil, dg
	}

	scope := dashboardSchemaToProtoObservationFieldScope[observationField.Scope.ValueString()]

	return &dashboards.ObservationField{
		Keypath: keypath,
		Scope:   scope,
	}, nil
}

func expandBarChartMetricsQuery(ctx context.Context, barChartQueryMetrics types.Object) (*dashboards.BarChart_MetricsQuery, diag.Diagnostics) {
	if barChartQueryMetrics.IsNull() || barChartQueryMetrics.IsUnknown() {
		return nil, nil
	}

	var barChartQueryMetricsObject BarChartQueryMetricsModel
	diags := barChartQueryMetrics.As(ctx, &barChartQueryMetricsObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	filters, diags := expandMetricsFilters(ctx, barChartQueryMetricsObject.Filters)
	if diags.HasError() {
		return nil, diags
	}

	groupNames, diags := typeStringSliceToWrappedStringSlice(ctx, barChartQueryMetricsObject.GroupNames.Elements())
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.BarChart_MetricsQuery{
		PromqlQuery:      expandPromqlQuery(barChartQueryMetricsObject.PromqlQuery),
		Filters:          filters,
		GroupNames:       groupNames,
		StackedGroupName: typeStringToWrapperspbString(barChartQueryMetricsObject.StackedGroupName),
	}, nil
}

func expandBarChartSpansQuery(ctx context.Context, barChartQuerySpans types.Object) (*dashboards.BarChart_SpansQuery, diag.Diagnostics) {
	if barChartQuerySpans.IsNull() || barChartQuerySpans.IsUnknown() {
		return nil, nil
	}

	var barChartQuerySpansObject BarChartQuerySpansModel
	diags := barChartQuerySpans.As(ctx, &barChartQuerySpansObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	aggregation, dg := expandSpansAggregation(barChartQuerySpansObject.Aggregation)
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	filters, diags := expandSpansFilters(ctx, barChartQuerySpansObject.Filters)
	if diags.HasError() {
		return nil, diags
	}

	groupNames, diags := expandSpansFields(ctx, barChartQuerySpansObject.GroupNames)
	if diags.HasError() {
		return nil, diags
	}

	expandedFilter, dg := expandSpansField(barChartQuerySpansObject.StackedGroupName)
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	return &dashboards.BarChart_SpansQuery{
		LuceneQuery:      expandLuceneQuery(barChartQuerySpansObject.LuceneQuery),
		Aggregation:      aggregation,
		Filters:          filters,
		GroupNames:       groupNames,
		StackedGroupName: expandedFilter,
	}, nil
}

func expandSpansFields(ctx context.Context, spanFields types.List) ([]*dashboards.SpanField, diag.Diagnostics) {
	var spanFieldsObjects []types.Object
	var expandedSpanFields []*dashboards.SpanField
	diags := spanFields.ElementsAs(ctx, &spanFieldsObjects, true)
	if diags.HasError() {
		return nil, diags
	}
	for _, sfo := range spanFieldsObjects {
		var spansField SpansFieldModel
		if dg := sfo.As(ctx, &spansField, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expandedSpanField, expandDiag := expandSpansField(&spansField)
		if expandDiag != nil {
			diags.Append(expandDiag)
			continue
		}
		expandedSpanFields = append(expandedSpanFields, expandedSpanField)
	}

	return expandedSpanFields, diags
}

func expandBarChartDataPrimeQuery(ctx context.Context, dataPrime types.Object) (*dashboards.BarChart_DataprimeQuery, diag.Diagnostics) {
	if dataPrime.IsNull() || dataPrime.IsUnknown() {
		return nil, nil
	}

	var dataPrimeObject BarChartQueryDataPrimeModel
	diags := dataPrime.As(ctx, &dataPrimeObject, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	filters, diags := expandDashboardFiltersSources(ctx, dataPrimeObject.Filters)
	if diags.HasError() {
		return nil, diags
	}

	groupNames, diags := typeStringSliceToWrappedStringSlice(ctx, dataPrimeObject.GroupNames.Elements())
	if diags.HasError() {
		return nil, diags
	}

	dataPrimeQuery := &dashboards.DataprimeQuery{
		Text: dataPrimeObject.Query.ValueString(),
	}
	return &dashboards.BarChart_DataprimeQuery{
		Filters:          filters,
		DataprimeQuery:   dataPrimeQuery,
		GroupNames:       groupNames,
		StackedGroupName: typeStringToWrapperspbString(dataPrimeObject.StackedGroupName),
	}, nil
}

func expandDataTable(ctx context.Context, table *DataTableModel) (*dashboards.Widget_Definition, diag.Diagnostics) {
	query, diags := expandDataTableQuery(ctx, table.Query)
	if diags.HasError() {
		return nil, diags
	}

	columns, diags := expandDataTableColumns(ctx, table.Columns)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.Widget_Definition{
		Value: &dashboards.Widget_Definition_DataTable{
			DataTable: &dashboards.DataTable{
				Query:          query,
				ResultsPerPage: typeInt64ToWrappedInt32(table.ResultsPerPage),
				RowStyle:       dashboardRowStyleSchemaToProto[table.RowStyle.ValueString()],
				Columns:        columns,
				OrderBy:        expandOrderBy(table.OrderBy),
				DataModeType:   dashboardSchemaToProtoDataModeType[table.DataModeType.ValueString()],
			},
		},
	}, nil
}

func expandDataTableQuery(ctx context.Context, dataTableQuery *DataTableQueryModel) (*dashboards.DataTable_Query, diag.Diagnostics) {
	if dataTableQuery == nil {
		return nil, nil
	}
	switch {
	case dataTableQuery.Metrics != nil:
		metrics, diags := expandDataTableMetricsQuery(ctx, dataTableQuery.Metrics)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.DataTable_Query{
			Value: metrics,
		}, nil
	case dataTableQuery.Logs != nil:
		logs, diags := expandDataTableLogsQuery(ctx, dataTableQuery.Logs)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.DataTable_Query{
			Value: logs,
		}, nil
	case dataTableQuery.Spans != nil:
		spans, diags := expandDataTableSpansQuery(ctx, dataTableQuery.Spans)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.DataTable_Query{
			Value: spans,
		}, nil
	case dataTableQuery.DataPrime != nil:
		dataPrime, diags := expandDataTableDataPrimeQuery(ctx, dataTableQuery.DataPrime)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.DataTable_Query{
			Value: dataPrime,
		}, nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand DataTable Query", fmt.Sprintf("unknown data table query type %#v", dataTableQuery))}
	}
}

func expandDataTableDataPrimeQuery(ctx context.Context, dataPrime *DataPrimeModel) (*dashboards.DataTable_Query_Dataprime, diag.Diagnostics) {
	if dataPrime == nil {
		return nil, nil
	}

	filters, diags := expandDashboardFiltersSources(ctx, dataPrime.Filters)
	if diags.HasError() {
		return nil, diags
	}

	var dataPrimeQuery *dashboards.DataprimeQuery
	if !dataPrime.Query.IsNull() {
		dataPrimeQuery = &dashboards.DataprimeQuery{
			Text: dataPrime.Query.ValueString(),
		}
	}

	return &dashboards.DataTable_Query_Dataprime{
		Dataprime: &dashboards.DataTable_DataprimeQuery{
			DataprimeQuery: dataPrimeQuery,
			Filters:        filters,
		},
	}, nil
}

func expandDashboardFiltersSources(ctx context.Context, filters types.List) ([]*dashboards.Filter_Source, diag.Diagnostics) {
	var filtersObjects []types.Object
	var expandedFiltersSources []*dashboards.Filter_Source
	diags := filters.ElementsAs(ctx, &filtersObjects, true)
	if diags.HasError() {
		return nil, diags
	}
	for _, fo := range filtersObjects {
		var filterSource DashboardFilterSourceModel
		if dg := fo.As(ctx, &filterSource, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expandedFilter, expandDiags := expandFilterSource(ctx, &filterSource)
		if expandDiags.HasError() {
			diags.Append(expandDiags...)
			continue
		}
		expandedFiltersSources = append(expandedFiltersSources, expandedFilter)
	}

	return expandedFiltersSources, diags
}

func expandDataTableMetricsQuery(ctx context.Context, dataTableQueryMetric *DataTableQueryMetricsModel) (*dashboards.DataTable_Query_Metrics, diag.Diagnostics) {
	if dataTableQueryMetric == nil {
		return nil, nil
	}

	filters, diags := expandMetricsFilters(ctx, dataTableQueryMetric.Filters)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.DataTable_Query_Metrics{
		Metrics: &dashboards.DataTable_MetricsQuery{
			PromqlQuery: expandPromqlQuery(dataTableQueryMetric.PromqlQuery),
			Filters:     filters,
		},
	}, nil
}

func expandDataTableLogsQuery(ctx context.Context, dataTableQueryLogs *DataTableQueryLogsModel) (*dashboards.DataTable_Query_Logs, diag.Diagnostics) {
	if dataTableQueryLogs == nil {
		return nil, nil
	}

	filters, diags := expandLogsFilters(ctx, dataTableQueryLogs.Filters)
	if diags.HasError() {
		return nil, diags
	}

	grouping, diags := expandDataTableLogsGrouping(ctx, dataTableQueryLogs.Grouping)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.DataTable_Query_Logs{
		Logs: &dashboards.DataTable_LogsQuery{
			LuceneQuery: expandLuceneQuery(dataTableQueryLogs.LuceneQuery),
			Filters:     filters,
			Grouping:    grouping,
		},
	}, nil
}

func expandDataTableLogsGrouping(ctx context.Context, grouping *DataTableLogsQueryGroupingModel) (*dashboards.DataTable_LogsQuery_Grouping, diag.Diagnostics) {
	if grouping == nil {
		return nil, nil
	}

	groupBy, diags := typeStringSliceToWrappedStringSlice(ctx, grouping.GroupBy.Elements())
	if diags.HasError() {
		return nil, diags
	}

	aggregations, diags := expandDataTableLogsAggregations(ctx, grouping.Aggregations)
	if diags.HasError() {
		return nil, diags
	}

	groupBys, diags := expandObservationFields(ctx, grouping.GroupBys)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.DataTable_LogsQuery_Grouping{
		GroupBy:      groupBy,
		Aggregations: aggregations,
		GroupBys:     groupBys,
	}, nil

}

func expandDataTableLogsAggregations(ctx context.Context, aggregations types.List) ([]*dashboards.DataTable_LogsQuery_Aggregation, diag.Diagnostics) {
	var aggregationsObjects []types.Object
	var expandedAggregations []*dashboards.DataTable_LogsQuery_Aggregation
	diags := aggregations.ElementsAs(ctx, &aggregationsObjects, true)
	if diags.HasError() {
		return nil, diags
	}
	for _, ao := range aggregationsObjects {
		var aggregation DataTableLogsAggregationModel
		if dg := ao.As(ctx, &aggregation, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expandedAggregation, expandDiags := expandDataTableLogsAggregation(ctx, &aggregation)
		if expandDiags.HasError() {
			diags.Append(expandDiags...)
			continue
		}
		expandedAggregations = append(expandedAggregations, expandedAggregation)
	}

	return expandedAggregations, diags
}

func expandDataTableLogsAggregation(ctx context.Context, aggregation *DataTableLogsAggregationModel) (*dashboards.DataTable_LogsQuery_Aggregation, diag.Diagnostics) {
	if aggregation == nil {
		return nil, nil
	}

	logsAggregation, diags := expandLogsAggregation(ctx, aggregation.Aggregation)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.DataTable_LogsQuery_Aggregation{
		Id:          typeStringToWrapperspbString(aggregation.ID),
		Name:        typeStringToWrapperspbString(aggregation.Name),
		IsVisible:   typeBoolToWrapperspbBool(aggregation.IsVisible),
		Aggregation: logsAggregation,
	}, nil
}

func expandDataTableSpansQuery(ctx context.Context, dataTableQuerySpans *DataTableQuerySpansModel) (*dashboards.DataTable_Query_Spans, diag.Diagnostics) {
	if dataTableQuerySpans == nil {
		return nil, nil
	}

	filters, diags := expandSpansFilters(ctx, dataTableQuerySpans.Filters)
	if diags.HasError() {
		return nil, diags
	}

	grouping, diags := expandDataTableSpansGrouping(ctx, dataTableQuerySpans.Grouping)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.DataTable_Query_Spans{
		Spans: &dashboards.DataTable_SpansQuery{
			LuceneQuery: expandLuceneQuery(dataTableQuerySpans.LuceneQuery),
			Filters:     filters,
			Grouping:    grouping,
		},
	}, nil
}

func expandDataTableSpansGrouping(ctx context.Context, grouping *DataTableSpansQueryGroupingModel) (*dashboards.DataTable_SpansQuery_Grouping, diag.Diagnostics) {
	if grouping == nil {
		return nil, nil
	}

	groupBy, diags := expandSpansFields(ctx, grouping.GroupBy)
	if diags.HasError() {
		return nil, diags
	}

	aggregations, diags := expandDataTableSpansAggregations(ctx, grouping.Aggregations)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.DataTable_SpansQuery_Grouping{
		GroupBy:      groupBy,
		Aggregations: aggregations,
	}, nil
}

func expandDataTableSpansAggregations(ctx context.Context, spansAggregations types.List) ([]*dashboards.DataTable_SpansQuery_Aggregation, diag.Diagnostics) {
	var spansAggregationsObjects []types.Object
	var expandedSpansAggregations []*dashboards.DataTable_SpansQuery_Aggregation
	diags := spansAggregations.ElementsAs(ctx, &spansAggregationsObjects, true)
	if diags.HasError() {
		return nil, diags
	}
	for _, sfo := range spansAggregationsObjects {
		var aggregation DataTableSpansAggregationModel
		if dg := sfo.As(ctx, &aggregation, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expandedSpansAggregation, expandDiag := expandDataTableSpansAggregation(&aggregation)
		if expandDiag != nil {
			diags.Append(expandDiag)
			continue
		}
		expandedSpansAggregations = append(expandedSpansAggregations, expandedSpansAggregation)
	}

	return expandedSpansAggregations, diags
}

func expandDataTableSpansAggregation(aggregation *DataTableSpansAggregationModel) (*dashboards.DataTable_SpansQuery_Aggregation, diag.Diagnostic) {
	if aggregation == nil {
		return nil, nil
	}

	spansAggregation, dg := expandSpansAggregation(aggregation.Aggregation)
	if dg != nil {
		return nil, dg
	}

	return &dashboards.DataTable_SpansQuery_Aggregation{
		Id:          typeStringToWrapperspbString(aggregation.ID),
		Name:        typeStringToWrapperspbString(aggregation.Name),
		IsVisible:   typeBoolToWrapperspbBool(aggregation.IsVisible),
		Aggregation: spansAggregation,
	}, nil
}

func expandDataTableColumns(ctx context.Context, columns types.List) ([]*dashboards.DataTable_Column, diag.Diagnostics) {
	var columnsObjects []types.Object
	var expandedColumns []*dashboards.DataTable_Column
	diags := columns.ElementsAs(ctx, &columnsObjects, true)
	if diags.HasError() {
		return nil, diags
	}
	for _, co := range columnsObjects {
		var column DataTableColumnModel
		if dg := co.As(ctx, &column, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expandedColumn := expandDataTableColumn(column)
		expandedColumns = append(expandedColumns, expandedColumn)
	}

	return expandedColumns, diags
}

func expandDataTableColumn(column DataTableColumnModel) *dashboards.DataTable_Column {
	return &dashboards.DataTable_Column{
		Field: typeStringToWrapperspbString(column.Field),
		Width: typeInt64ToWrappedInt32(column.Width),
	}
}

func expandOrderBy(orderBy *OrderByModel) *dashboards.OrderingField {
	if orderBy == nil {
		return nil
	}
	return &dashboards.OrderingField{
		Field:          typeStringToWrapperspbString(orderBy.Field),
		OrderDirection: dashboardOrderDirectionSchemaToProto[orderBy.OrderDirection.ValueString()],
	}
}
func expandLineChart(ctx context.Context, lineChart *LineChartModel) (*dashboards.Widget_Definition, diag.Diagnostics) {
	if lineChart == nil {
		return nil, nil
	}

	legend, diags := expandLineChartLegend(ctx, lineChart.Legend)
	if diags.HasError() {
		return nil, diags
	}

	queryDefinitions, diags := expandLineChartQueryDefinitions(ctx, lineChart.QueryDefinitions)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.Widget_Definition{
		Value: &dashboards.Widget_Definition_LineChart{
			LineChart: &dashboards.LineChart{
				Legend:           legend,
				Tooltip:          expandLineChartTooltip(lineChart.Tooltip),
				QueryDefinitions: queryDefinitions,
			},
		},
	}, nil
}

func expandLineChartLegend(ctx context.Context, legend *LegendModel) (*dashboards.Legend, diag.Diagnostics) {
	if legend == nil {
		return nil, nil
	}

	columns, diags := expandLineChartLegendColumns(ctx, legend.Columns.Elements())
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.Legend{
		IsVisible:    typeBoolToWrapperspbBool(legend.IsVisible),
		Columns:      columns,
		GroupByQuery: typeBoolToWrapperspbBool(legend.GroupByQuery),
		Placement:    dashboardLegendPlacementSchemaToProto[legend.Placement.ValueString()],
	}, nil
}

func expandLineChartLegendColumns(ctx context.Context, columns []attr.Value) ([]dashboards.Legend_LegendColumn, diag.Diagnostics) {
	var diags diag.Diagnostics
	expandedColumns := make([]dashboards.Legend_LegendColumn, 0, len(columns))
	for _, s := range columns {
		v, err := s.ToTerraformValue(ctx)
		if err != nil {
			diags.AddError("Extract LineChart Legend Columns Error", err.Error())
			continue
		}
		var column string
		if err = v.As(&column); err != nil {
			diags.AddError("Extract LineChart Legend Columns Error", err.Error())
			continue
		}

		expandedColumn := dashboardLegendColumnSchemaToProto[column]
		expandedColumns = append(expandedColumns, expandedColumn)
	}

	return expandedColumns, diags
}

func expandLineChartTooltip(tooltip *TooltipModel) *dashboards.LineChart_Tooltip {
	if tooltip == nil {
		return nil
	}

	return &dashboards.LineChart_Tooltip{
		ShowLabels: typeBoolToWrapperspbBool(tooltip.ShowLabels),
		Type:       dashboardSchemaToProtoTooltipType[tooltip.Type.ValueString()],
	}
}

func expandLineChartQueryDefinitions(ctx context.Context, queryDefinitions types.List) ([]*dashboards.LineChart_QueryDefinition, diag.Diagnostics) {
	var queryDefinitionsObjects []types.Object
	var expandedQueryDefinitions []*dashboards.LineChart_QueryDefinition
	diags := queryDefinitions.ElementsAs(ctx, &queryDefinitionsObjects, true)
	if diags.HasError() {
		return nil, diags
	}
	for _, qdo := range queryDefinitionsObjects {
		var queryDefinition LineChartQueryDefinitionModel
		if dg := qdo.As(ctx, &queryDefinition, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expandedQueryDefinition, expandDiag := expandLineChartQueryDefinition(ctx, &queryDefinition)
		if expandDiag != nil {
			diags.Append(expandDiag...)
			continue
		}
		expandedQueryDefinitions = append(expandedQueryDefinitions, expandedQueryDefinition)
	}

	return expandedQueryDefinitions, diags
}

func expandLineChartQueryDefinition(ctx context.Context, queryDefinition *LineChartQueryDefinitionModel) (*dashboards.LineChart_QueryDefinition, diag.Diagnostics) {
	if queryDefinition == nil {
		return nil, nil
	}
	query, diags := expandLineChartQuery(ctx, queryDefinition.Query)
	if diags.HasError() {
		return nil, diags
	}

	resolution, diags := expandResolution(ctx, queryDefinition.Resolution)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.LineChart_QueryDefinition{
		Id:                 expandDashboardIDs(queryDefinition.ID),
		Query:              query,
		SeriesNameTemplate: typeStringToWrapperspbString(queryDefinition.SeriesNameTemplate),
		SeriesCountLimit:   typeInt64ToWrappedInt64(queryDefinition.SeriesCountLimit),
		Unit:               dashboardSchemaToProtoUnit[queryDefinition.Unit.ValueString()],
		ScaleType:          dashboardSchemaToProtoScaleType[queryDefinition.ScaleType.ValueString()],
		Name:               typeStringToWrapperspbString(queryDefinition.Name),
		IsVisible:          typeBoolToWrapperspbBool(queryDefinition.IsVisible),
		ColorScheme:        typeStringToWrapperspbString(queryDefinition.ColorScheme),
		Resolution:         resolution,
		DataModeType:       dashboardSchemaToProtoDataModeType[queryDefinition.DataModeType.ValueString()],
	}, nil
}

func expandResolution(ctx context.Context, resolution types.Object) (*dashboards.LineChart_Resolution, diag.Diagnostics) {
	if resolution.IsNull() || resolution.IsUnknown() {
		return nil, nil
	}

	var resolutionModel LineChartResolutionModel
	if diags := resolution.As(ctx, &resolutionModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	if !(resolutionModel.Interval.IsNull() || resolutionModel.Interval.IsUnknown()) {
		interval, dg := parseDuration(resolutionModel.Interval.ValueString(), "resolution.interval")
		if dg != nil {
			return nil, diag.Diagnostics{dg}
		}

		return &dashboards.LineChart_Resolution{
			Interval: durationpb.New(*interval),
		}, nil
	}

	return &dashboards.LineChart_Resolution{
		BucketsPresented: typeInt64ToWrappedInt32(resolutionModel.BucketsPresented),
	}, nil
}

func expandLineChartQuery(ctx context.Context, query *LineChartQueryModel) (*dashboards.LineChart_Query, diag.Diagnostics) {
	if query == nil {
		return nil, nil
	}

	switch {
	case query.Logs != nil:
		logs, diags := expandLineChartLogsQuery(ctx, query.Logs)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.LineChart_Query{
			Value: logs,
		}, nil
	case query.Metrics != nil:
		metrics, diags := expandLineChartMetricsQuery(ctx, query.Metrics)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.LineChart_Query{
			Value: metrics,
		}, nil
	case query.Spans != nil:
		spans, diags := expandLineChartSpansQuery(ctx, query.Spans)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.LineChart_Query{
			Value: spans,
		}, nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand LineChart Query", "Unknown LineChart Query type")}
	}
}

func expandLineChartLogsQuery(ctx context.Context, logs *LineChartQueryLogsModel) (*dashboards.LineChart_Query_Logs, diag.Diagnostics) {
	if logs == nil {
		return nil, nil
	}

	groupBy, diags := typeStringSliceToWrappedStringSlice(ctx, logs.GroupBy.Elements())
	if diags.HasError() {
		return nil, diags
	}

	aggregations, diags := expandLogsAggregations(ctx, logs.Aggregations)
	if diags.HasError() {
		return nil, diags
	}

	filters, diags := expandLogsFilters(ctx, logs.Filters)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.LineChart_Query_Logs{
		Logs: &dashboards.LineChart_LogsQuery{
			LuceneQuery:  expandLuceneQuery(logs.LuceneQuery),
			GroupBy:      groupBy,
			Aggregations: aggregations,
			Filters:      filters,
		},
	}, nil
}

func expandLineChartMetricsQuery(ctx context.Context, metrics *LineChartQueryMetricsModel) (*dashboards.LineChart_Query_Metrics, diag.Diagnostics) {
	if metrics == nil {
		return nil, nil
	}

	filters, diags := expandMetricsFilters(ctx, metrics.Filters)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.LineChart_Query_Metrics{
		Metrics: &dashboards.LineChart_MetricsQuery{
			PromqlQuery: expandPromqlQuery(metrics.PromqlQuery),
			Filters:     filters,
		},
	}, nil
}

func expandLineChartSpansQuery(ctx context.Context, spans *LineChartQuerySpansModel) (*dashboards.LineChart_Query_Spans, diag.Diagnostics) {
	if spans == nil {
		return nil, nil
	}

	groupBy, diags := expandSpansFields(ctx, spans.GroupBy)
	if diags.HasError() {
		return nil, diags
	}

	aggregations, diags := expandSpansAggregations(ctx, spans.Aggregations)
	if diags.HasError() {
		return nil, diags
	}

	filters, diags := expandSpansFilters(ctx, spans.Filters)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.LineChart_Query_Spans{
		Spans: &dashboards.LineChart_SpansQuery{
			LuceneQuery:  expandLuceneQuery(spans.LuceneQuery),
			GroupBy:      groupBy,
			Aggregations: aggregations,
			Filters:      filters,
		},
	}, nil
}

func expandPieChartQuery(ctx context.Context, pieChartQuery *PieChartQueryModel) (*dashboards.PieChart_Query, diag.Diagnostics) {
	if pieChartQuery == nil {
		return nil, nil
	}

	switch {
	case pieChartQuery.Logs != nil:
		logs, diags := expandPieChartLogsQuery(ctx, pieChartQuery.Logs)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.PieChart_Query{
			Value: logs,
		}, nil
	case pieChartQuery.Metrics != nil:
		metrics, diags := expandPieChartMetricsQuery(ctx, pieChartQuery.Metrics)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.PieChart_Query{
			Value: metrics,
		}, nil
	case pieChartQuery.Spans != nil:
		spans, diags := expandPieChartSpansQuery(ctx, pieChartQuery.Spans)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.PieChart_Query{
			Value: spans,
		}, nil
	case pieChartQuery.DataPrime != nil:
		dataPrime, diags := expandPieChartDataPrimeQuery(ctx, pieChartQuery.DataPrime)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.PieChart_Query{
			Value: dataPrime,
		}, nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand PieChart Query", "Unknown PieChart Query type")}
	}
}

func expandPieChartLogsQuery(ctx context.Context, pieChartQueryLogs *PieChartQueryLogsModel) (*dashboards.PieChart_Query_Logs, diag.Diagnostics) {
	if pieChartQueryLogs == nil {
		return nil, nil
	}

	aggregation, diags := expandLogsAggregation(ctx, pieChartQueryLogs.Aggregation)
	if diags.HasError() {
		return nil, diags
	}

	filters, diags := expandLogsFilters(ctx, pieChartQueryLogs.Filters)
	if diags.HasError() {
		return nil, diags
	}

	groupNames, diags := typeStringSliceToWrappedStringSlice(ctx, pieChartQueryLogs.GroupNames.Elements())
	if diags.HasError() {
		return nil, diags
	}

	groupNamesFields, diags := expandObservationFields(ctx, pieChartQueryLogs.GroupNamesFields)
	if diags.HasError() {
		return nil, diags
	}

	stackedGroupNameField, diags := expandObservationFieldObject(ctx, pieChartQueryLogs.StackedGroupNameField)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.PieChart_Query_Logs{
		Logs: &dashboards.PieChart_LogsQuery{
			LuceneQuery:           expandLuceneQuery(pieChartQueryLogs.LuceneQuery),
			Aggregation:           aggregation,
			Filters:               filters,
			GroupNames:            groupNames,
			StackedGroupName:      typeStringToWrapperspbString(pieChartQueryLogs.StackedGroupName),
			GroupNamesFields:      groupNamesFields,
			StackedGroupNameField: stackedGroupNameField,
		},
	}, nil
}

func expandPieChartMetricsQuery(ctx context.Context, pieChartQueryMetrics *PieChartQueryMetricsModel) (*dashboards.PieChart_Query_Metrics, diag.Diagnostics) {
	if pieChartQueryMetrics == nil {
		return nil, nil
	}

	filters, diags := expandMetricsFilters(ctx, pieChartQueryMetrics.Filters)
	if diags.HasError() {
		return nil, diags
	}

	groupNames, diags := typeStringSliceToWrappedStringSlice(ctx, pieChartQueryMetrics.GroupNames.Elements())
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.PieChart_Query_Metrics{
		Metrics: &dashboards.PieChart_MetricsQuery{
			PromqlQuery:      expandPromqlQuery(pieChartQueryMetrics.PromqlQuery),
			GroupNames:       groupNames,
			Filters:          filters,
			StackedGroupName: typeStringToWrapperspbString(pieChartQueryMetrics.StackedGroupName),
		},
	}, nil
}

func expandPieChartSpansQuery(ctx context.Context, pieChartQuerySpans *PieChartQuerySpansModel) (*dashboards.PieChart_Query_Spans, diag.Diagnostics) {
	if pieChartQuerySpans == nil {
		return nil, nil
	}

	aggregation, dg := expandSpansAggregation(pieChartQuerySpans.Aggregation)
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	filters, diags := expandSpansFilters(ctx, pieChartQuerySpans.Filters)
	if diags.HasError() {
		return nil, diags
	}

	groupNames, diags := expandSpansFields(ctx, pieChartQuerySpans.GroupNames)
	if diags.HasError() {
		return nil, diags
	}

	stackedGroupName, dg := expandSpansField(pieChartQuerySpans.StackedGroupName)
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	return &dashboards.PieChart_Query_Spans{
		Spans: &dashboards.PieChart_SpansQuery{
			LuceneQuery:      expandLuceneQuery(pieChartQuerySpans.LuceneQuery),
			Aggregation:      aggregation,
			Filters:          filters,
			GroupNames:       groupNames,
			StackedGroupName: stackedGroupName,
		},
	}, nil
}

func expandPieChartDataPrimeQuery(ctx context.Context, dataPrime *PieChartQueryDataPrimeModel) (*dashboards.PieChart_Query_Dataprime, diag.Diagnostics) {
	if dataPrime == nil {
		return nil, nil
	}

	filters, diags := expandDashboardFiltersSources(ctx, dataPrime.Filters)
	if diags.HasError() {
		return nil, diags
	}

	groupNames, diags := typeStringSliceToWrappedStringSlice(ctx, dataPrime.GroupNames.Elements())
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.PieChart_Query_Dataprime{
		Dataprime: &dashboards.PieChart_DataprimeQuery{
			DataprimeQuery: &dashboards.DataprimeQuery{
				Text: dataPrime.Query.ValueString(),
			},
			Filters:          filters,
			GroupNames:       groupNames,
			StackedGroupName: typeStringToWrapperspbString(dataPrime.StackedGroupName),
		},
	}, nil
}

func expandDashboardVariables(ctx context.Context, variables types.List) ([]*dashboards.Variable, diag.Diagnostics) {
	var variablesObjects []types.Object
	var expandedVariables []*dashboards.Variable
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

func expandDashboardVariable(ctx context.Context, variable DashboardVariableModel) (*dashboards.Variable, diag.Diagnostics) {
	definition, diags := expandDashboardVariableDefinition(ctx, variable.Definition)
	if diags.HasError() {
		return nil, diags
	}
	return &dashboards.Variable{
		Name:        typeStringToWrapperspbString(variable.Name),
		DisplayName: typeStringToWrapperspbString(variable.DisplayName),
		Definition:  definition,
	}, nil
}

func expandDashboardVariableDefinition(ctx context.Context, definition *DashboardVariableDefinitionModel) (*dashboards.Variable_Definition, diag.Diagnostics) {
	if definition == nil {
		return nil, nil
	}

	switch {
	case definition.MultiSelect != nil:
		return expandMultiSelect(ctx, definition.MultiSelect)
	case !definition.ConstantValue.IsNull():
		return &dashboards.Variable_Definition{
			Value: &dashboards.Variable_Definition_Constant{
				Constant: &dashboards.Constant{
					Value: typeStringToWrapperspbString(definition.ConstantValue),
				},
			},
		}, nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand Dashboard Variable", fmt.Sprintf("unknown variable definition type: %T", definition))}
	}
}

func expandMultiSelect(ctx context.Context, multiSelect *VariableMultiSelectModel) (*dashboards.Variable_Definition, diag.Diagnostics) {
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

	return &dashboards.Variable_Definition{
		Value: &dashboards.Variable_Definition_MultiSelect{
			MultiSelect: &dashboards.MultiSelect{
				Source:               source,
				Selection:            selection,
				ValuesOrderDirection: dashboardOrderDirectionSchemaToProto[multiSelect.ValuesOrderDirection.ValueString()],
			},
		},
	}, nil
}

func expandMultiSelectSelection(ctx context.Context, selectedValues []attr.Value) (*dashboards.MultiSelect_Selection, diag.Diagnostics) {
	if len(selectedValues) == 0 {
		return &dashboards.MultiSelect_Selection{
			Value: &dashboards.MultiSelect_Selection_All{
				All: &dashboards.MultiSelect_Selection_AllSelection{},
			},
		}, nil
	}

	selections, diags := typeStringSliceToWrappedStringSlice(ctx, selectedValues)
	if diags.HasError() {
		return nil, diags
	}
	return &dashboards.MultiSelect_Selection{
		Value: &dashboards.MultiSelect_Selection_List{
			List: &dashboards.MultiSelect_Selection_ListSelection{
				Values: selections,
			},
		},
	}, nil
}

func expandMultiSelectSource(ctx context.Context, source *VariableMultiSelectSourceModel) (*dashboards.MultiSelect_Source, diag.Diagnostics) {
	if source == nil {
		return nil, nil
	}

	switch {
	case !source.LogsPath.IsNull():
		return &dashboards.MultiSelect_Source{
			Value: &dashboards.MultiSelect_Source_LogsPath{
				LogsPath: &dashboards.MultiSelect_LogsPathSource{
					Value: typeStringToWrapperspbString(source.LogsPath),
				},
			},
		}, nil
	case !source.ConstantList.IsNull():
		constantList, diags := typeStringSliceToWrappedStringSlice(ctx, source.ConstantList.Elements())
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.MultiSelect_Source{
			Value: &dashboards.MultiSelect_Source_ConstantList{
				ConstantList: &dashboards.MultiSelect_ConstantListSource{
					Values: constantList,
				},
			},
		}, nil
	case source.MetricLabel != nil:
		return &dashboards.MultiSelect_Source{
			Value: &dashboards.MultiSelect_Source_MetricLabel{
				MetricLabel: &dashboards.MultiSelect_MetricLabelSource{
					MetricName: typeStringToWrapperspbString(source.MetricLabel.MetricName),
					Label:      typeStringToWrapperspbString(source.MetricLabel.Label),
				},
			},
		}, nil
	case source.SpanField != nil:
		spanField, dg := expandSpansField(source.SpanField)
		if dg != nil {
			return nil, diag.Diagnostics{dg}
		}
		return &dashboards.MultiSelect_Source{
			Value: &dashboards.MultiSelect_Source_SpanField{
				SpanField: &dashboards.MultiSelect_SpanFieldSource{
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

func expandDashboardFilters(ctx context.Context, filters types.List) ([]*dashboards.Filter, diag.Diagnostics) {
	var filtersObjects []types.Object
	var expandedFilters []*dashboards.Filter
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

func expandDashboardFilter(ctx context.Context, filter *DashboardFilterModel) (*dashboards.Filter, diag.Diagnostics) {
	if filter == nil {
		return nil, nil
	}

	source, diags := expandFilterSource(ctx, filter.Source)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.Filter{
		Source:    source,
		Enabled:   typeBoolToWrapperspbBool(filter.Enabled),
		Collapsed: typeBoolToWrapperspbBool(filter.Collapsed),
	}, nil
}

func expandFilterSource(ctx context.Context, source *DashboardFilterSourceModel) (*dashboards.Filter_Source, diag.Diagnostics) {
	if source == nil {
		return nil, nil
	}

	switch {
	case source.Logs != nil:
		return expandFilterSourceLogs(ctx, source.Logs)
	case source.Metrics != nil:
		return expandFilterSourceMetrics(ctx, source.Metrics)
	case source.Spans != nil:
		return expandFilterSourceSpans(ctx, source.Spans)
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand Filter Source", fmt.Sprintf("Unknown filter source type: %#v", source))}
	}
}

func expandFilterSourceLogs(ctx context.Context, logs *FilterSourceLogsModel) (*dashboards.Filter_Source, diag.Diagnostics) {
	if logs == nil {
		return nil, nil
	}

	operator, diags := expandFilterOperator(ctx, logs.Operator)
	if diags.HasError() {
		return nil, diags
	}

	observationField, diags := expandObservationFieldObject(ctx, logs.ObservationField)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.Filter_Source{
		Value: &dashboards.Filter_Source_Logs{
			Logs: &dashboards.Filter_LogsFilter{
				Field:            typeStringToWrapperspbString(logs.Field),
				Operator:         operator,
				ObservationField: observationField,
			},
		},
	}, nil
}

func expandFilterSourceMetrics(ctx context.Context, metrics *FilterSourceMetricsModel) (*dashboards.Filter_Source, diag.Diagnostics) {
	if metrics == nil {
		return nil, nil
	}

	operator, diags := expandFilterOperator(ctx, metrics.Operator)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.Filter_Source{
		Value: &dashboards.Filter_Source_Metrics{
			Metrics: &dashboards.Filter_MetricsFilter{
				Metric:   typeStringToWrapperspbString(metrics.MetricName),
				Label:    typeStringToWrapperspbString(metrics.MetricLabel),
				Operator: operator,
			},
		},
	}, nil
}

func expandFilterSourceSpans(ctx context.Context, spans *FilterSourceSpansModel) (*dashboards.Filter_Source, diag.Diagnostics) {
	if spans == nil {
		return nil, nil
	}

	field, dg := expandSpansField(spans.Field)
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	operator, diags := expandFilterOperator(ctx, spans.Operator)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.Filter_Source{
		Value: &dashboards.Filter_Source_Spans{
			Spans: &dashboards.Filter_SpansFilter{
				Field:    field,
				Operator: operator,
			},
		},
	}, nil
}

func expandDashboardFolder(ctx context.Context, dashboard *dashboards.Dashboard, folder types.Object) (*dashboards.Dashboard, diag.Diagnostics) {
	if folder.IsNull() || folder.IsUnknown() {
		return dashboard, nil
	}
	var folderModel DashboardFolderModel
	dgs := folder.As(ctx, &folderModel, basetypes.ObjectAsOptions{})
	if dgs.HasError() {
		return nil, dgs
	}

	if !(folderModel.Path.IsNull() || folderModel.Path.IsUnknown()) {
		segments := strings.Split(folderModel.Path.ValueString(), "/")
		dashboard.Folder = &dashboards.Dashboard_FolderPath{
			FolderPath: &dashboards.FolderPath{
				Segments: segments,
			},
		}
	} else if !(folderModel.ID.IsNull() || folderModel.ID.IsUnknown()) {
		dashboard.Folder = &dashboards.Dashboard_FolderId{
			FolderId: expandDashboardUUID(folderModel.ID),
		}
	}

	return dashboard, nil
}

func expandAbsoluteDashboardTimeFrame(ctx context.Context, timeFrame types.Object) (*dashboards.Dashboard_AbsoluteTimeFrame, diag.Diagnostics) {
	timeFrameModel := &DashboardTimeFrameAbsoluteModel{}
	dgs := timeFrame.As(ctx, timeFrameModel, basetypes.ObjectAsOptions{})
	if dgs.HasError() {
		return nil, dgs
	}
	fromTime, err := time.Parse(time.RFC3339, timeFrameModel.Start.ValueString())
	if err != nil {
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand Absolute Dashboard Time Frame", fmt.Sprintf("Error parsing from time: %s", err.Error()))}
	}
	toTime, err := time.Parse(time.RFC3339, timeFrameModel.End.ValueString())
	if err != nil {
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand Absolute Dashboard Time Frame", fmt.Sprintf("Error parsing from time: %s", err.Error()))}
	}

	from := timestamppb.New(fromTime)
	to := timestamppb.New(toTime)

	return &dashboards.Dashboard_AbsoluteTimeFrame{
		AbsoluteTimeFrame: &dashboards.TimeFrame{
			From: from,
			To:   to,
		},
	}, nil
}

func parseDuration(ti, fieldsName string) (*time.Duration, diag.Diagnostic) {
	// This for some reason has format seconds:900
	durStr := strings.Split(ti, ":")
	var duration time.Duration
	if len(durStr) != 2 {
		return nil, diag.NewErrorDiagnostic(fmt.Sprintf("Error Expand %s", fieldsName), fmt.Sprintf("error parsing duration: %s", durStr))
	}
	unit := durStr[0]
	no, err := strconv.Atoi(durStr[1])
	if err != nil {
		return nil, diag.NewErrorDiagnostic(fmt.Sprintf("Error Expand %s", fieldsName), fmt.Sprintf("error parsing duration numbers: %s", durStr))
	}
	switch unit {
	case "seconds":
		duration = time.Second * time.Duration(no)
	case "minutes":
		duration = time.Minute * time.Duration(no)
	default:
		return nil, diag.NewErrorDiagnostic(fmt.Sprintf("Error Expand %s", fieldsName), fmt.Sprintf("error parsing duration unit: %s", unit))
	}
	return &duration, nil
}

func expandRelativeDashboardTimeFrame(ctx context.Context, timeFrame types.Object) (*dashboards.Dashboard_RelativeTimeFrame, diag.Diagnostics) {
	timeFrameModel := &DashboardTimeFrameRelativeModel{}
	dgs := timeFrame.As(ctx, timeFrameModel, basetypes.ObjectAsOptions{})
	if dgs.HasError() {
		return nil, dgs
	}
	duration, dg := parseDuration(timeFrameModel.Duration.ValueString(), "Relative Dashboard Time Frame")
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}
	return &dashboards.Dashboard_RelativeTimeFrame{
		RelativeTimeFrame: durationpb.New(*duration),
	}, nil
}

func expand21LengthUUID(id types.String) *dashboards.UUID {
	if id.IsNull() || id.IsUnknown() {
		return &dashboards.UUID{Value: RandStringBytes(21)}
	}
	return &dashboards.UUID{Value: id.ValueString()}
}

func expandDashboardUUID(id types.String) *dashboards.UUID {
	if id.IsNull() || id.IsUnknown() {
		return &dashboards.UUID{Value: uuid.NewString()}
	}
	return &dashboards.UUID{Value: id.ValueString()}
}

func expandDashboardIDs(id types.String) *wrapperspb.StringValue {
	if id.IsNull() || id.IsUnknown() {
		return &wrapperspb.StringValue{Value: uuid.NewString()}
	}
	return &wrapperspb.StringValue{Value: id.ValueString()}
}

func flattenDashboard(ctx context.Context, plan DashboardResourceModel, dashboard *dashboards.Dashboard) (*DashboardResourceModel, diag.Diagnostics) {
	if !(plan.ContentJson.IsNull() || plan.ContentJson.IsUnknown()) {
		_, err := protojson.Marshal(dashboard)
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
			TimeFrame:   types.ObjectNull(dashboardTimeFrameModelAttr()),
			Folder:      types.ObjectNull(dashboardFolderModelAttr()),
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

	timeFrame, diags := flattenDashboardTimeFrame(ctx, dashboard)
	if diags.HasError() {
		return nil, diags
	}

	folder, diags := flattenDashboardFolder(ctx, plan.Folder, dashboard)
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
		Name:        wrapperspbStringToTypeString(dashboard.GetName()),
		Description: wrapperspbStringToTypeString(dashboard.GetDescription()),
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

func flattenDashboardLayout(ctx context.Context, layout *dashboards.Layout) (types.Object, diag.Diagnostics) {
	sections, diags := flattenDashboardSections(ctx, layout.GetSections())
	if diags.HasError() {
		return types.ObjectNull(layoutModelAttr()), diags
	}
	flattenedLayout := &DashboardLayoutModel{
		Sections: sections,
	}
	return types.ObjectValueFrom(ctx, layoutModelAttr(), flattenedLayout)
}

func flattenDashboardSections(ctx context.Context, sections []*dashboards.Section) (types.List, diag.Diagnostics) {
	if len(sections) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: sectionModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	sectionsElements := make([]attr.Value, 0, len(sections))
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
				"line_chart": types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"legend": types.ObjectType{
							AttrTypes: map[string]attr.Type{
								"is_visible": types.BoolType,
								"columns": types.ListType{
									ElemType: types.StringType,
								},
								"group_by_query": types.BoolType,
								"placement":      types.StringType,
							},
						},
						"tooltip": types.ObjectType{
							AttrTypes: map[string]attr.Type{
								"show_labels": types.BoolType,
								"type":        types.StringType,
							},
						},
						"query_definitions": types.ListType{
							ElemType: types.ObjectType{
								AttrTypes: map[string]attr.Type{
									"id": types.StringType,
									"query": types.ObjectType{
										AttrTypes: map[string]attr.Type{
											"logs": types.ObjectType{
												AttrTypes: map[string]attr.Type{
													"lucene_query": types.StringType,
													"group_by": types.ListType{
														ElemType: types.StringType,
													},
													"aggregations": types.ListType{
														ElemType: types.ObjectType{
															AttrTypes: aggregationModelAttr(),
														},
													},
													"filters": types.ListType{
														ElemType: types.ObjectType{
															AttrTypes: logsFilterModelAttr(),
														},
													},
												},
											},
											"metrics": types.ObjectType{
												AttrTypes: map[string]attr.Type{
													"promql_query": types.StringType,
													"filters": types.ListType{
														ElemType: types.ObjectType{
															AttrTypes: metricsFilterModelAttr(),
														},
													},
												},
											},
											"spans": types.ObjectType{
												AttrTypes: map[string]attr.Type{
													"lucene_query": types.StringType,
													"group_by": types.ListType{
														ElemType: types.ObjectType{
															AttrTypes: spansFieldModelAttr(),
														},
													},
													"aggregations": types.ListType{
														ElemType: types.ObjectType{
															AttrTypes: spansAggregationModelAttr(),
														},
													},
													"filters": types.ListType{
														ElemType: types.ObjectType{
															AttrTypes: spansFilterModelAttr(),
														},
													},
												},
											},
										},
									},
									"series_name_template": types.StringType,
									"series_count_limit":   types.Int64Type,
									"unit":                 types.StringType,
									"scale_type":           types.StringType,
									"name":                 types.StringType,
									"is_visible":           types.BoolType,
									"color_scheme":         types.StringType,
									"resolution": types.ObjectType{
										AttrTypes: map[string]attr.Type{
											"interval":          types.StringType,
											"buckets_presented": types.Int64Type,
										},
									},
									"data_mode_type": types.StringType,
								},
							},
						},
					},
				},
				"data_table": types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"query": types.ObjectType{
							AttrTypes: map[string]attr.Type{
								"logs": types.ObjectType{
									AttrTypes: map[string]attr.Type{
										"lucene_query": types.StringType,
										"filters": types.ListType{
											ElemType: types.ObjectType{
												AttrTypes: logsFilterModelAttr(),
											},
										},
										"grouping": types.ObjectType{
											AttrTypes: map[string]attr.Type{
												"group_by": types.ListType{
													ElemType: types.StringType,
												},
												"aggregations": types.ListType{
													ElemType: types.ObjectType{
														AttrTypes: map[string]attr.Type{
															"id":         types.StringType,
															"name":       types.StringType,
															"is_visible": types.BoolType,
															"aggregation": types.ObjectType{
																AttrTypes: aggregationModelAttr(),
															},
														},
													},
												},
												"group_bys": types.ListType{
													ElemType: observationFieldsObject(),
												},
											},
										},
									},
								},
								"spans": types.ObjectType{
									AttrTypes: map[string]attr.Type{
										"lucene_query": types.StringType,
										"filters": types.ListType{
											ElemType: types.ObjectType{
												AttrTypes: spansFilterModelAttr(),
											},
										},
										"grouping": types.ObjectType{
											AttrTypes: map[string]attr.Type{
												"group_by": types.ListType{
													ElemType: types.ObjectType{
														AttrTypes: spansFieldModelAttr(),
													},
												},
												"aggregations": types.ListType{
													ElemType: types.ObjectType{
														AttrTypes: map[string]attr.Type{
															"id":         types.StringType,
															"name":       types.StringType,
															"is_visible": types.BoolType,
															"aggregation": types.ObjectType{
																AttrTypes: spansAggregationModelAttr(),
															},
														},
													},
												},
											},
										},
									},
								},
								"metrics": types.ObjectType{
									AttrTypes: map[string]attr.Type{
										"promql_query": types.StringType,
										"filters": types.ListType{
											ElemType: types.ObjectType{
												AttrTypes: metricsFilterModelAttr(),
											},
										},
									},
								},
								"data_prime": types.ObjectType{
									AttrTypes: map[string]attr.Type{
										"query": types.StringType,
										"filters": types.ListType{
											ElemType: types.ObjectType{
												AttrTypes: filterSourceModelAttr(),
											},
										},
									},
								},
							},
						},
						"results_per_page": types.Int64Type,
						"row_style":        types.StringType,
						"columns": types.ListType{
							ElemType: types.ObjectType{
								AttrTypes: dataTableColumnModelAttr(),
							},
						},
						"order_by": types.ObjectType{
							AttrTypes: map[string]attr.Type{
								"field":           types.StringType,
								"order_direction": types.StringType,
							},
						},
						"data_mode_type": types.StringType,
					},
				},
				"gauge": types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"query": types.ObjectType{
							AttrTypes: map[string]attr.Type{
								"logs": types.ObjectType{
									AttrTypes: map[string]attr.Type{
										"lucene_query": types.StringType,
										"logs_aggregation": types.ObjectType{
											AttrTypes: aggregationModelAttr(),
										},
										"filters": types.ListType{
											ElemType: types.ObjectType{
												AttrTypes: logsFilterModelAttr(),
											},
										},
									},
								},
								"metrics": types.ObjectType{
									AttrTypes: map[string]attr.Type{
										"promql_query": types.StringType,
										"aggregation":  types.StringType,
										"filters": types.ListType{
											ElemType: types.ObjectType{
												AttrTypes: metricsFilterModelAttr(),
											},
										},
									},
								},
								"spans": types.ObjectType{
									AttrTypes: map[string]attr.Type{
										"lucene_query": types.StringType,
										"spans_aggregation": types.ObjectType{
											AttrTypes: spansAggregationModelAttr(),
										},
										"filters": types.ListType{
											ElemType: types.ObjectType{
												AttrTypes: spansFilterModelAttr(),
											},
										},
									},
								},
								"data_prime": types.ObjectType{
									AttrTypes: map[string]attr.Type{
										"query": types.StringType,
										"filters": types.ListType{
											ElemType: types.ObjectType{
												AttrTypes: filterSourceModelAttr(),
											},
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
						"data_mode_type": types.StringType,
						"threshold_by":   types.StringType,
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
											AttrTypes: aggregationModelAttr(),
										},
										"filters": types.ListType{
											ElemType: types.ObjectType{
												AttrTypes: logsFilterModelAttr(),
											},
										},
										"group_names": types.ListType{
											ElemType: types.StringType,
										},
										"stacked_group_name": types.StringType,
										"group_names_fields": types.ListType{
											ElemType: observationFieldsObject(),
										},
										"stacked_group_name_field": observationFieldsObject(),
									},
								},
								"metrics": types.ObjectType{
									AttrTypes: map[string]attr.Type{
										"promql_query": types.StringType,
										"filters": types.ListType{
											ElemType: types.ObjectType{
												AttrTypes: metricsFilterModelAttr(),
											},
										},
										"group_names": types.ListType{
											ElemType: types.StringType,
										},
										"stacked_group_name": types.StringType,
									},
								},
								"spans": types.ObjectType{
									AttrTypes: map[string]attr.Type{
										"lucene_query": types.StringType,
										"aggregation": types.ObjectType{
											AttrTypes: spansAggregationModelAttr(),
										},
										"filters": types.ListType{
											ElemType: types.ObjectType{
												AttrTypes: spansFilterModelAttr(),
											},
										},
										"group_names": types.ListType{
											ElemType: types.ObjectType{
												AttrTypes: spansFieldModelAttr(),
											},
										},
										"stacked_group_name": types.ObjectType{
											AttrTypes: spansFieldModelAttr(),
										},
									},
								},
								"data_prime": types.ObjectType{
									AttrTypes: map[string]attr.Type{
										"query": types.StringType,
										"filters": types.ListType{
											ElemType: types.ObjectType{
												AttrTypes: filterSourceModelAttr(),
											},
										},
										"group_names": types.ListType{
											ElemType: types.StringType,
										},
										"stacked_group_name": types.StringType,
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
											AttrTypes: aggregationModelAttr(),
										},
										"filters": types.ListType{
											ElemType: types.ObjectType{
												AttrTypes: logsFilterModelAttr(),
											},
										},
										"group_names": types.ListType{
											ElemType: types.StringType,
										},
										"stacked_group_name": types.StringType,
										"group_names_fields": types.ListType{
											ElemType: observationFieldsObject(),
										},
										"stacked_group_name_field": observationFieldsObject(),
									},
								},
								"metrics": types.ObjectType{
									AttrTypes: map[string]attr.Type{
										"promql_query": types.StringType,
										"filters": types.ListType{
											ElemType: types.ObjectType{
												AttrTypes: metricsFilterModelAttr(),
											},
										},
										"group_names": types.ListType{
											ElemType: types.StringType,
										},
										"stacked_group_name": types.StringType,
									},
								},
								"spans": types.ObjectType{
									AttrTypes: map[string]attr.Type{
										"lucene_query": types.StringType,
										"aggregation": types.ObjectType{
											AttrTypes: spansAggregationModelAttr(),
										},
										"filters": types.ListType{
											ElemType: types.ObjectType{
												AttrTypes: spansFilterModelAttr(),
											},
										},
										"group_names": types.ListType{
											ElemType: types.ObjectType{
												AttrTypes: spansFieldModelAttr(),
											},
										},
										"stacked_group_name": types.ObjectType{
											AttrTypes: spansFieldModelAttr(),
										},
									},
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
			AttrTypes: aggregationModelAttr(),
		},
		"filters": types.ListType{
			ElemType: types.ObjectType{
				AttrTypes: logsFilterModelAttr(),
			},
		},
		"group_names": types.ListType{
			ElemType: types.StringType,
		},
		"stacked_group_name": types.StringType,
		"group_names_fields": types.ListType{
			ElemType: observationFieldsObject(),
		},
		"stacked_group_name_field": observationFieldsObject(),
	}
}

func barChartMetricsQueryAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"promql_query": types.StringType,
		"filters": types.ListType{
			ElemType: types.ObjectType{
				AttrTypes: metricsFilterModelAttr(),
			},
		},
		"group_names": types.ListType{
			ElemType: types.StringType,
		},
		"stacked_group_name": types.StringType,
	}
}

func barChartSpansQueryAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"lucene_query": types.StringType,
		"aggregation": types.ObjectType{
			AttrTypes: spansAggregationModelAttr(),
		},
		"filters": types.ListType{
			ElemType: types.ObjectType{
				AttrTypes: spansFilterModelAttr(),
			},
		},
		"group_names": types.ListType{
			ElemType: types.ObjectType{
				AttrTypes: spansFieldModelAttr(),
			},
		},
		"stacked_group_name": types.ObjectType{
			AttrTypes: spansFieldModelAttr(),
		},
	}
}

func barChartDataPrimeQueryAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"query": types.StringType,
		"filters": types.ListType{
			ElemType: types.ObjectType{
				AttrTypes: filterSourceModelAttr(),
			},
		},
		"group_names": types.ListType{
			ElemType: types.StringType,
		},
		"stacked_group_name": types.StringType,
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
		AttrTypes: observationFieldAttributes(),
	}
}

func metricStrategyModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"start_time": types.ObjectType{
			AttrTypes: map[string]attr.Type{},
		},
	}
}

func observationFieldsObject() types.ObjectType {
	return types.ObjectType{
		AttrTypes: observationFieldAttributes(),
	}
}

func observationFieldAttributes() map[string]attr.Type {
	return map[string]attr.Type{
		"keypath": types.ListType{
			ElemType: types.StringType,
		},
		"scope": types.StringType,
	}
}

func gaugeThresholdModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"from":  types.Float64Type,
		"color": types.StringType,
	}
}

func lineChartQueryDefinitionModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"id": types.StringType,
		"query": types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"logs": types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"lucene_query": types.StringType,
						"group_by": types.ListType{
							ElemType: types.StringType,
						},
						"aggregations": types.ListType{
							ElemType: types.ObjectType{
								AttrTypes: aggregationModelAttr(),
							},
						},
						"filters": types.ListType{
							ElemType: types.ObjectType{
								AttrTypes: logsFilterModelAttr(),
							},
						},
					},
				},
				"metrics": types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"promql_query": types.StringType,
						"filters": types.ListType{
							ElemType: types.ObjectType{
								AttrTypes: metricsFilterModelAttr(),
							},
						},
					},
				},
				"spans": types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"lucene_query": types.StringType,
						"group_by": types.ListType{
							ElemType: types.ObjectType{
								AttrTypes: spansFieldModelAttr(),
							},
						},
						"aggregations": types.ListType{
							ElemType: types.ObjectType{
								AttrTypes: map[string]attr.Type{
									"type":             types.StringType,
									"aggregation_type": types.StringType,
									"field":            types.StringType,
								},
							},
						},
						"filters": types.ListType{
							ElemType: types.ObjectType{
								AttrTypes: spansFilterModelAttr(),
							},
						},
					},
				},
			},
		},
		"series_name_template": types.StringType,
		"series_count_limit":   types.Int64Type,
		"unit":                 types.StringType,
		"scale_type":           types.StringType,
		"name":                 types.StringType,
		"is_visible":           types.BoolType,
		"color_scheme":         types.StringType,
		"resolution": types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"interval":          types.StringType,
				"buckets_presented": types.Int64Type,
			},
		},
		"data_mode_type": types.StringType,
	}
}

func aggregationModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"type":              types.StringType,
		"field":             types.StringType,
		"percent":           types.Float64Type,
		"observation_field": observationFieldsObject(),
	}
}

func logsFilterModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"field": types.StringType,
		"operator": types.ObjectType{
			AttrTypes: filterOperatorModelAttr(),
		},
		"observation_field": types.ObjectType{
			AttrTypes: observationFieldAttributes(),
		},
	}
}

func filterOperatorModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"type": types.StringType,
		"selected_values": types.ListType{
			ElemType: types.StringType,
		},
	}
}

func metricsFilterModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"metric": types.StringType,
		"label":  types.StringType,
		"operator": types.ObjectType{
			AttrTypes: filterOperatorModelAttr(),
		},
	}
}

func spansFieldModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"type":  types.StringType,
		"value": types.StringType,
	}
}

func groupingAggregationModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"id":         types.StringType,
		"name":       types.StringType,
		"is_visible": types.BoolType,
		"aggregation": types.ObjectType{
			AttrTypes: aggregationModelAttr(),
		},
	}
}

func dataTableColumnModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"field": types.StringType,
		"width": types.Int64Type,
	}
}

func spansAggregationModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"type":             types.StringType,
		"aggregation_type": types.StringType,
		"field":            types.StringType,
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
									AttrTypes: spansFieldModelAttr(),
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
			AttrTypes: filterSourceModelAttr(),
		},
		"enabled":   types.BoolType,
		"collapsed": types.BoolType,
	}
}

func filterSourceModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"logs":    types.ObjectType{AttrTypes: logsFilterModelAttr()},
		"metrics": types.ObjectType{AttrTypes: filterSourceMetricsModelAttr()},
		"spans":   types.ObjectType{AttrTypes: filterSourceSpansModelAttr()},
	}
}

func filterSourceSpansModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"field": types.ObjectType{
			AttrTypes: spansFieldModelAttr(),
		},
		"operator": types.ObjectType{
			AttrTypes: filterOperatorModelAttr(),
		},
	}
}

func filterSourceMetricsModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"metric_name": types.StringType,
		"label":       types.StringType,
		"operator": types.ObjectType{
			AttrTypes: filterOperatorModelAttr(),
		},
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

func flattenDashboardSection(ctx context.Context, section *dashboards.Section) (*SectionModel, diag.Diagnostics) {
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

func flattenDashboardOptions(_ context.Context, opts *dashboards.SectionOptions) (*SectionOptionsModel, diag.Diagnostics) {
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

func flattenDashboardRows(ctx context.Context, rows []*dashboards.Row) (types.List, diag.Diagnostics) {
	if len(rows) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: rowModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	rowsElements := make([]attr.Value, 0, len(rows))
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

func flattenDashboardRow(ctx context.Context, row *dashboards.Row) (*RowModel, diag.Diagnostics) {
	if row == nil {
		return nil, nil
	}

	widgets, diags := flattenDashboardWidgets(ctx, row.GetWidgets())
	if diags.HasError() {
		return nil, diags
	}
	return &RowModel{
		ID:      types.StringValue(row.GetId().GetValue()),
		Height:  wrapperspbInt32ToTypeInt64(row.GetAppearance().GetHeight()),
		Widgets: widgets,
	}, nil
}

func flattenDashboardWidgets(ctx context.Context, widgets []*dashboards.Widget) (types.List, diag.Diagnostics) {
	if len(widgets) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: widgetModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	widgetsElements := make([]attr.Value, 0, len(widgets))
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

func flattenDashboardWidget(ctx context.Context, widget *dashboards.Widget) (*WidgetModel, diag.Diagnostics) {
	if widget == nil {
		return nil, nil
	}

	definition, diags := flattenDashboardWidgetDefinition(ctx, widget.GetDefinition())
	if diags.HasError() {
		return nil, diags
	}

	return &WidgetModel{
		ID:          types.StringValue(widget.GetId().GetValue()),
		Title:       wrapperspbStringToTypeString(widget.GetTitle()),
		Description: wrapperspbStringToTypeString(widget.GetDescription()),
		Width:       wrapperspbInt32ToTypeInt64(widget.GetAppearance().GetWidth()),
		Definition:  definition,
	}, nil
}

func flattenDashboardWidgetDefinition(ctx context.Context, definition *dashboards.Widget_Definition) (*WidgetDefinitionModel, diag.Diagnostics) {
	if definition == nil {
		return nil, nil
	}

	switch definition.GetValue().(type) {
	case *dashboards.Widget_Definition_LineChart:
		return flattenLineChart(ctx, definition.GetLineChart())
	case *dashboards.Widget_Definition_DataTable:
		return flattenDataTable(ctx, definition.GetDataTable())
	case *dashboards.Widget_Definition_Gauge:
		return flattenGauge(ctx, definition.GetGauge())
	case *dashboards.Widget_Definition_PieChart:
		return flattenPieChart(ctx, definition.GetPieChart())
	case *dashboards.Widget_Definition_BarChart:
		return flattenBarChart(ctx, definition.GetBarChart())
	case *dashboards.Widget_Definition_HorizontalBarChart:
		return flattenHorizontalBarChart(ctx, definition.GetHorizontalBarChart())
	case *dashboards.Widget_Definition_Markdown:
		return flattenMarkdown(definition.GetMarkdown()), nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Widget Definition", "unknown widget definition type")}
	}
}

func flattenMarkdown(markdown *dashboards.Markdown) *WidgetDefinitionModel {
	return &WidgetDefinitionModel{
		Markdown: &MarkdownModel{
			MarkdownText: wrapperspbStringToTypeString(markdown.GetMarkdownText()),
			TooltipText:  wrapperspbStringToTypeString(markdown.GetTooltipText()),
		},
	}
}

func flattenHorizontalBarChart(ctx context.Context, chart *dashboards.HorizontalBarChart) (*WidgetDefinitionModel, diag.Diagnostics) {
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

	return &WidgetDefinitionModel{
		HorizontalBarChart: &HorizontalBarChartModel{
			Query:             query,
			MaxBarsPerChart:   wrapperspbInt32ToTypeInt64(chart.GetMaxBarsPerChart()),
			GroupNameTemplate: wrapperspbStringToTypeString(chart.GetGroupNameTemplate()),
			StackDefinition:   flattenHorizontalBarChartStackDefinition(chart.GetStackDefinition()),
			ScaleType:         types.StringValue(dashboardProtoToSchemaScaleType[chart.GetScaleType()]),
			ColorsBy:          colorsBy,
			Unit:              types.StringValue(dashboardProtoToSchemaUnit[chart.GetUnit()]),
			DisplayOnBar:      wrapperspbBoolToTypeBool(chart.GetDisplayOnBar()),
			YAxisViewBy:       flattenYAxisViewBy(chart.GetYAxisViewBy()),
			SortBy:            types.StringValue(dashboardProtoToSchemaSortBy[chart.GetSortBy()]),
			ColorScheme:       wrapperspbStringToTypeString(chart.GetColorScheme()),
			DataModeType:      types.StringValue(dashboardProtoToSchemaDataModeType[chart.GetDataModeType()]),
		},
	}, nil
}

func flattenYAxisViewBy(yAxisViewBy *dashboards.HorizontalBarChart_YAxisViewBy) types.String {
	switch yAxisViewBy.GetYAxisView().(type) {
	case *dashboards.HorizontalBarChart_YAxisViewBy_Category:
		return types.StringValue("category")
	case *dashboards.HorizontalBarChart_YAxisViewBy_Value:
		return types.StringValue("value")
	default:
		return types.StringNull()
	}
}

func flattenHorizontalBarChartQueryDefinitions(ctx context.Context, query *dashboards.HorizontalBarChart_Query) (*HorizontalBarChartQueryModel, diag.Diagnostics) {
	if query == nil {
		return nil, nil
	}

	switch query.GetValue().(type) {
	case *dashboards.HorizontalBarChart_Query_Logs:
		return flattenHorizontalBarChartQueryLogs(ctx, query.GetLogs())
	case *dashboards.HorizontalBarChart_Query_Metrics:
		return flattenHorizontalBarChartQueryMetrics(ctx, query.GetMetrics())
	case *dashboards.HorizontalBarChart_Query_Spans:
		return flattenHorizontalBarChartQuerySpans(ctx, query.GetSpans())
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Horizontal Bar Chart Query", "unknown horizontal bar chart query type")}
	}
}

func flattenHorizontalBarChartQueryLogs(ctx context.Context, logs *dashboards.HorizontalBarChart_LogsQuery) (*HorizontalBarChartQueryModel, diag.Diagnostics) {
	if logs == nil {
		return nil, nil
	}

	aggregation, diags := flattenLogsAggregation(ctx, logs.GetAggregation())
	if diags.HasError() {
		return nil, diags
	}

	filters, diags := flattenLogsFilters(ctx, logs.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	groupNamesFields, diags := flattenObservationFields(ctx, logs.GetGroupNamesFields())
	if diags.HasError() {
		return nil, diags
	}

	stackedGroupNameField, diags := flattenObservationField(ctx, logs.GetStackedGroupNameField())
	if diags.HasError() {
		return nil, diags
	}

	logsModel := &BarChartQueryLogsModel{
		LuceneQuery:           wrapperspbStringToTypeString(logs.GetLuceneQuery().GetValue()),
		Aggregation:           aggregation,
		Filters:               filters,
		GroupNames:            wrappedStringSliceToTypeStringList(logs.GetGroupNames()),
		StackedGroupName:      wrapperspbStringToTypeString(logs.GetStackedGroupName()),
		GroupNamesFields:      groupNamesFields,
		StackedGroupNameField: stackedGroupNameField,
	}

	logsObject, diags := types.ObjectValueFrom(ctx, barChartLogsQueryAttr(), logsModel)
	if diags.HasError() {
		return nil, diags
	}

	return &HorizontalBarChartQueryModel{
		Logs:    logsObject,
		Metrics: types.ObjectNull(barChartMetricsQueryAttr()),
		Spans:   types.ObjectNull(barChartSpansQueryAttr()),
	}, nil
}

func flattenHorizontalBarChartQueryMetrics(ctx context.Context, metrics *dashboards.HorizontalBarChart_MetricsQuery) (*HorizontalBarChartQueryModel, diag.Diagnostics) {
	if metrics == nil {
		return nil, nil
	}

	filters, diags := flattenMetricsFilters(ctx, metrics.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	flattenedMetrics := &BarChartQueryMetricsModel{
		PromqlQuery:      wrapperspbStringToTypeString(metrics.GetPromqlQuery().GetValue()),
		Filters:          filters,
		GroupNames:       wrappedStringSliceToTypeStringList(metrics.GetGroupNames()),
		StackedGroupName: wrapperspbStringToTypeString(metrics.GetStackedGroupName()),
	}

	metricsObject, diags := types.ObjectValueFrom(ctx, barChartMetricsQueryAttr(), flattenedMetrics)
	if diags.HasError() {
		return nil, diags
	}

	return &HorizontalBarChartQueryModel{
		Metrics: metricsObject,
		Logs:    types.ObjectNull(barChartLogsQueryAttr()),
		Spans:   types.ObjectNull(barChartSpansQueryAttr()),
	}, nil
}

func flattenHorizontalBarChartQuerySpans(ctx context.Context, spans *dashboards.HorizontalBarChart_SpansQuery) (*HorizontalBarChartQueryModel, diag.Diagnostics) {
	if spans == nil {
		return nil, nil
	}

	aggregation, dg := flattenSpansAggregation(spans.GetAggregation())
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	filters, diags := flattenSpansFilters(ctx, spans.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	groupNames, diags := flattenSpansFields(ctx, spans.GetGroupNames())
	if diags.HasError() {
		return nil, diags
	}

	stackedGroupName, dg := flattenSpansField(spans.GetStackedGroupName())
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	flattenedSpans := &BarChartQuerySpansModel{
		LuceneQuery:      wrapperspbStringToTypeString(spans.GetLuceneQuery().GetValue()),
		Aggregation:      aggregation,
		Filters:          filters,
		GroupNames:       groupNames,
		StackedGroupName: stackedGroupName,
	}

	spansObject, diags := types.ObjectValueFrom(ctx, barChartSpansQueryAttr(), flattenedSpans)
	if diags.HasError() {
		return nil, diags
	}

	return &HorizontalBarChartQueryModel{
		Spans:   spansObject,
		Logs:    types.ObjectNull(barChartLogsQueryAttr()),
		Metrics: types.ObjectNull(barChartMetricsQueryAttr()),
	}, nil
}

func flattenLineChart(ctx context.Context, lineChart *dashboards.LineChart) (*WidgetDefinitionModel, diag.Diagnostics) {
	if lineChart == nil {
		return nil, nil
	}

	queryDefinitions, diags := flattenLineChartQueryDefinitions(ctx, lineChart.GetQueryDefinitions())
	if diags.HasError() {
		return nil, diags
	}

	return &WidgetDefinitionModel{
		LineChart: &LineChartModel{
			Legend:           flattenLegend(lineChart.GetLegend()),
			Tooltip:          flattenTooltip(lineChart.GetTooltip()),
			QueryDefinitions: queryDefinitions,
		},
	}, nil
}

func flattenLegend(legend *dashboards.Legend) *LegendModel {
	if legend == nil {
		return nil
	}

	return &LegendModel{
		IsVisible:    wrapperspbBoolToTypeBool(legend.GetIsVisible()),
		GroupByQuery: wrapperspbBoolToTypeBool(legend.GetGroupByQuery()),
		Columns:      flattenLegendColumns(legend.GetColumns()),
		Placement:    types.StringValue(dashboardLegendPlacementProtoToSchema[legend.GetPlacement()]),
	}
}

func flattenLegendColumns(columns []dashboards.Legend_LegendColumn) types.List {
	if len(columns) == 0 {
		return types.ListNull(types.StringType)
	}

	columnsElements := make([]attr.Value, 0, len(columns))
	for _, column := range columns {
		flattenedColumn := dashboardLegendColumnProtoToSchema[column]
		columnElement := types.StringValue(flattenedColumn)
		columnsElements = append(columnsElements, columnElement)
	}

	return types.ListValueMust(types.StringType, columnsElements)
}

func flattenTooltip(tooltip *dashboards.LineChart_Tooltip) *TooltipModel {
	if tooltip == nil {
		return nil
	}
	return &TooltipModel{
		ShowLabels: wrapperspbBoolToTypeBool(tooltip.GetShowLabels()),
		Type:       types.StringValue(dashboardProtoToSchemaTooltipType[tooltip.GetType()]),
	}
}

func flattenLineChartQueryDefinitions(ctx context.Context, definitions []*dashboards.LineChart_QueryDefinition) (types.List, diag.Diagnostics) {
	if len(definitions) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: lineChartQueryDefinitionModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	definitionsElements := make([]attr.Value, 0, len(definitions))
	for _, definition := range definitions {
		flattenedDefinition, diags := flattenLineChartQueryDefinition(ctx, definition)
		if diags.HasError() {
			diagnostics = append(diagnostics, diags...)
			continue
		}
		definitionElement, diags := types.ObjectValueFrom(ctx, lineChartQueryDefinitionModelAttr(), flattenedDefinition)
		if diags.HasError() {
			diagnostics = append(diagnostics, diags...)
			continue
		}
		definitionsElements = append(definitionsElements, definitionElement)
	}

	return types.ListValueMust(types.ObjectType{AttrTypes: lineChartQueryDefinitionModelAttr()}, definitionsElements), diagnostics
}

func flattenLineChartQueryDefinition(ctx context.Context, definition *dashboards.LineChart_QueryDefinition) (*LineChartQueryDefinitionModel, diag.Diagnostics) {
	if definition == nil {
		return nil, nil
	}

	query, diags := flattenLineChartQuery(ctx, definition.GetQuery())
	if diags.HasError() {
		return nil, diags
	}

	resolution, diags := flattenLineChartQueryResolution(ctx, definition.GetResolution())
	if diags.HasError() {
		return nil, diags
	}

	return &LineChartQueryDefinitionModel{
		ID:                 wrapperspbStringToTypeString(definition.GetId()),
		Query:              query,
		SeriesNameTemplate: wrapperspbStringToTypeString(definition.GetSeriesNameTemplate()),
		SeriesCountLimit:   wrapperspbInt64ToTypeInt64(definition.GetSeriesCountLimit()),
		Unit:               types.StringValue(dashboardProtoToSchemaUnit[definition.GetUnit()]),
		ScaleType:          types.StringValue(dashboardProtoToSchemaScaleType[definition.GetScaleType()]),
		Name:               wrapperspbStringToTypeString(definition.GetName()),
		IsVisible:          wrapperspbBoolToTypeBool(definition.GetIsVisible()),
		ColorScheme:        wrapperspbStringToTypeString(definition.GetColorScheme()),
		Resolution:         resolution,
		DataModeType:       types.StringValue(dashboardProtoToSchemaDataModeType[definition.GetDataModeType()]),
	}, nil
}

func flattenLineChartQueryResolution(ctx context.Context, resolution *dashboards.LineChart_Resolution) (types.Object, diag.Diagnostics) {
	if resolution == nil {
		return types.ObjectNull(lineChartQueryResolutionModelAttr()), nil
	}

	interval := types.StringNull()
	if resolution.GetInterval() != nil {
		interval = types.StringValue(resolution.GetInterval().String())
	}
	bucketsPresented := wrapperspbInt32ToTypeInt64(resolution.GetBucketsPresented())

	resolutionModel := LineChartResolutionModel{
		Interval:         interval,
		BucketsPresented: bucketsPresented,
	}
	return types.ObjectValueFrom(ctx, lineChartQueryResolutionModelAttr(), &resolutionModel)
}

func lineChartQueryResolutionModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"interval":          types.StringType,
		"buckets_presented": types.Int64Type,
	}
}

func flattenLineChartQuery(ctx context.Context, query *dashboards.LineChart_Query) (*LineChartQueryModel, diag.Diagnostics) {
	if query == nil {
		return nil, nil
	}

	switch query.GetValue().(type) {
	case *dashboards.LineChart_Query_Logs:
		return flattenLineChartQueryLogs(ctx, query.GetLogs())
	case *dashboards.LineChart_Query_Metrics:
		return flattenLineChartQueryMetrics(ctx, query.GetMetrics())
	case *dashboards.LineChart_Query_Spans:
		return flattenLineChartQuerySpans(ctx, query.GetSpans())
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Line Chart Query", "unknown line chart query type")}
	}
}

func flattenLineChartQueryLogs(ctx context.Context, logs *dashboards.LineChart_LogsQuery) (*LineChartQueryModel, diag.Diagnostics) {
	if logs == nil {
		return nil, nil
	}

	aggregations, diags := flattenAggregations(ctx, logs.GetAggregations())
	if diags.HasError() {
		return nil, diags
	}

	filters, diags := flattenLogsFilters(ctx, logs.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	return &LineChartQueryModel{
		Logs: &LineChartQueryLogsModel{
			LuceneQuery:  wrapperspbStringToTypeString(logs.GetLuceneQuery().GetValue()),
			GroupBy:      wrappedStringSliceToTypeStringList(logs.GetGroupBy()),
			Aggregations: aggregations,
			Filters:      filters,
		},
	}, nil
}

func flattenAggregations(ctx context.Context, aggregations []*dashboards.LogsAggregation) (types.List, diag.Diagnostics) {
	if len(aggregations) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: aggregationModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	aggregationsElements := make([]attr.Value, 0, len(aggregations))
	for _, aggregation := range aggregations {
		flattenedAggregation, diags := flattenLogsAggregation(ctx, aggregation)
		if diags.HasError() {
			diagnostics.Append(diags...)
			continue
		}
		aggregationElement, diags := types.ObjectValueFrom(ctx, aggregationModelAttr(), flattenedAggregation)
		if diags.HasError() {
			diagnostics.Append(diags...)
			continue
		}
		aggregationsElements = append(aggregationsElements, aggregationElement)
	}

	return types.ListValueMust(types.ObjectType{AttrTypes: aggregationModelAttr()}, aggregationsElements), diagnostics
}

func flattenLogsAggregation(ctx context.Context, aggregation *dashboards.LogsAggregation) (*LogsAggregationModel, diag.Diagnostics) {
	if aggregation == nil {
		return nil, nil
	}

	switch aggregationValue := aggregation.GetValue().(type) {
	case *dashboards.LogsAggregation_Count_:
		return &LogsAggregationModel{
			Type:             types.StringValue("count"),
			ObservationField: types.ObjectNull(observationFieldAttributes()),
		}, nil
	case *dashboards.LogsAggregation_CountDistinct_:
		observationField, diags := flattenObservationField(ctx, aggregationValue.CountDistinct.GetObservationField())
		if diags.HasError() {
			return nil, diags
		}
		return &LogsAggregationModel{
			Type:             types.StringValue("count_distinct"),
			Field:            wrapperspbStringToTypeString(aggregationValue.CountDistinct.GetField()),
			ObservationField: observationField,
		}, nil
	case *dashboards.LogsAggregation_Sum_:
		observationField, diags := flattenObservationField(ctx, aggregationValue.Sum.GetObservationField())
		if diags.HasError() {
			return nil, diags
		}
		return &LogsAggregationModel{
			Type:             types.StringValue("sum"),
			Field:            wrapperspbStringToTypeString(aggregationValue.Sum.GetField()),
			ObservationField: observationField,
		}, nil
	case *dashboards.LogsAggregation_Average_:
		observationField, diags := flattenObservationField(ctx, aggregationValue.Average.GetObservationField())
		if diags.HasError() {
			return nil, diags
		}
		return &LogsAggregationModel{
			Type:             types.StringValue("avg"),
			Field:            wrapperspbStringToTypeString(aggregationValue.Average.GetField()),
			ObservationField: observationField,
		}, nil
	case *dashboards.LogsAggregation_Min_:
		observationField, diags := flattenObservationField(ctx, aggregationValue.Min.GetObservationField())
		if diags.HasError() {
			return nil, diags
		}
		return &LogsAggregationModel{
			Type:             types.StringValue("min"),
			Field:            wrapperspbStringToTypeString(aggregationValue.Min.GetField()),
			ObservationField: observationField,
		}, nil
	case *dashboards.LogsAggregation_Max_:
		observationField, diags := flattenObservationField(ctx, aggregationValue.Max.GetObservationField())
		if diags.HasError() {
			return nil, diags
		}
		return &LogsAggregationModel{
			Type:             types.StringValue("max"),
			Field:            wrapperspbStringToTypeString(aggregationValue.Max.GetField()),
			ObservationField: observationField,
		}, nil
	case *dashboards.LogsAggregation_Percentile_:
		observationField, diags := flattenObservationField(ctx, aggregationValue.Percentile.GetObservationField())
		if diags.HasError() {
			return nil, diags
		}
		return &LogsAggregationModel{
			Type:             types.StringValue("percentile"),
			Field:            wrapperspbStringToTypeString(aggregationValue.Percentile.GetField()),
			Percent:          wrapperspbDoubleToTypeFloat64(aggregationValue.Percentile.GetPercent()),
			ObservationField: observationField,
		}, nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Logs Aggregation", "unknown logs aggregation type")}
	}
}

func flattenLogsFilters(ctx context.Context, filters []*dashboards.Filter_LogsFilter) (types.List, diag.Diagnostics) {
	if len(filters) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: logsFilterModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	filtersElements := make([]attr.Value, 0, len(filters))
	for _, filter := range filters {
		flattenedFilter, diags := flattenLogsFilter(ctx, filter)
		if diags.HasError() {
			diagnostics.Append(diags...)
			continue
		}
		filterElement, diags := types.ObjectValueFrom(ctx, logsFilterModelAttr(), flattenedFilter)
		if diags.HasError() {
			diagnostics.Append(diags...)
			continue
		}
		filtersElements = append(filtersElements, filterElement)
	}

	return types.ListValueMust(types.ObjectType{AttrTypes: logsFilterModelAttr()}, filtersElements), diagnostics
}

func flattenLogsFilter(ctx context.Context, filter *dashboards.Filter_LogsFilter) (*LogsFilterModel, diag.Diagnostics) {
	if filter == nil {
		return nil, nil
	}

	operator, dg := flattenFilterOperator(filter.GetOperator())
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	observationField, diags := flattenObservationField(ctx, filter.GetObservationField())
	if diags.HasError() {
		return nil, diags
	}

	return &LogsFilterModel{
		Field:            wrapperspbStringToTypeString(filter.GetField()),
		Operator:         operator,
		ObservationField: observationField,
	}, nil
}

func flattenFilterOperator(operator *dashboards.Filter_Operator) (*FilterOperatorModel, diag.Diagnostic) {
	switch operator.GetValue().(type) {
	case *dashboards.Filter_Operator_Equals:
		switch operator.GetEquals().GetSelection().GetValue().(type) {
		case *dashboards.Filter_Equals_Selection_All:
			return &FilterOperatorModel{
				Type:           types.StringValue("equals"),
				SelectedValues: types.ListNull(types.StringType),
			}, nil
		case *dashboards.Filter_Equals_Selection_List:
			return &FilterOperatorModel{
				Type:           types.StringValue("equals"),
				SelectedValues: wrappedStringSliceToTypeStringList(operator.GetEquals().GetSelection().GetList().GetValues()),
			}, nil
		default:
			return nil, diag.NewErrorDiagnostic("Error Flatten Logs Filter Operator Equals", "unknown logs filter operator equals selection type")
		}
	case *dashboards.Filter_Operator_NotEquals:
		switch operator.GetNotEquals().GetSelection().GetValue().(type) {
		case *dashboards.Filter_NotEquals_Selection_List:
			return &FilterOperatorModel{
				Type:           types.StringValue("not_equals"),
				SelectedValues: wrappedStringSliceToTypeStringList(operator.GetNotEquals().GetSelection().GetList().GetValues()),
			}, nil
		default:
			return nil, diag.NewErrorDiagnostic("Error Flatten Logs Filter Operator NotEquals", "unknown logs filter operator not_equals selection type")
		}
	default:
		return nil, diag.NewErrorDiagnostic("Error Flatten Logs Filter Operator", "unknown logs filter operator type")
	}
}

func flattenLineChartQueryMetrics(ctx context.Context, metrics *dashboards.LineChart_MetricsQuery) (*LineChartQueryModel, diag.Diagnostics) {
	if metrics == nil {
		return nil, nil
	}

	filters, diags := flattenMetricsFilters(ctx, metrics.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	return &LineChartQueryModel{
		Metrics: &LineChartQueryMetricsModel{
			PromqlQuery: wrapperspbStringToTypeString(metrics.GetPromqlQuery().GetValue()),
			Filters:     filters,
		},
	}, nil
}

func flattenMetricsFilters(ctx context.Context, filters []*dashboards.Filter_MetricsFilter) (types.List, diag.Diagnostics) {
	if len(filters) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: metricsFilterModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	filtersElements := make([]attr.Value, 0, len(filters))
	for _, filter := range filters {
		flattenedFilter, dg := flattenMetricsFilter(filter)
		if dg != nil {
			diagnostics.Append(dg)
			continue
		}
		filterElement, diags := types.ObjectValueFrom(ctx, metricsFilterModelAttr(), flattenedFilter)
		if diags.HasError() {
			diagnostics = append(diagnostics, diags...)
			continue
		}
		filtersElements = append(filtersElements, filterElement)
	}

	return types.ListValueMust(types.ObjectType{AttrTypes: metricsFilterModelAttr()}, filtersElements), diagnostics
}

func flattenMetricsFilter(filter *dashboards.Filter_MetricsFilter) (*MetricsFilterModel, diag.Diagnostic) {
	if filter == nil {
		return nil, nil
	}

	operator, dg := flattenFilterOperator(filter.GetOperator())
	if dg != nil {
		return nil, dg
	}

	return &MetricsFilterModel{
		Metric:   wrapperspbStringToTypeString(filter.GetMetric()),
		Label:    wrapperspbStringToTypeString(filter.GetLabel()),
		Operator: operator,
	}, nil
}

func flattenLineChartQuerySpans(ctx context.Context, spans *dashboards.LineChart_SpansQuery) (*LineChartQueryModel, diag.Diagnostics) {
	if spans == nil {
		return nil, nil
	}

	groupBy, diags := flattenSpansFields(ctx, spans.GetGroupBy())
	if diags.HasError() {
		return nil, diags
	}

	filters, diags := flattenSpansFilters(ctx, spans.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	return &LineChartQueryModel{
		Spans: &LineChartQuerySpansModel{
			LuceneQuery: wrapperspbStringToTypeString(spans.GetLuceneQuery().GetValue()),
			GroupBy:     groupBy,
			Filters:     filters,
		},
	}, nil
}

func flattenSpansFilters(ctx context.Context, filters []*dashboards.Filter_SpansFilter) (types.List, diag.Diagnostics) {
	if len(filters) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: spansFilterModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	filtersElements := make([]attr.Value, 0, len(filters))
	for _, filter := range filters {
		flattenedFilter, dg := flattenSpansFilter(filter)
		if dg != nil {
			diagnostics.Append(dg)
			continue
		}
		filterElement, diags := types.ObjectValueFrom(ctx, spansFilterModelAttr(), flattenedFilter)
		if diags.HasError() {
			diagnostics = append(diagnostics, diags...)
			continue
		}
		filtersElements = append(filtersElements, filterElement)
	}

	return types.ListValueMust(types.ObjectType{AttrTypes: spansFilterModelAttr()}, filtersElements), diagnostics

}

func spansFilterModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"field": types.ObjectType{
			AttrTypes: spansFieldModelAttr(),
		},
		"operator": types.ObjectType{
			AttrTypes: filterOperatorModelAttr(),
		},
	}
}

func flattenSpansFilter(filter *dashboards.Filter_SpansFilter) (*SpansFilterModel, diag.Diagnostic) {
	if filter == nil {
		return nil, nil
	}

	operator, dg := flattenFilterOperator(filter.GetOperator())
	if dg != nil {
		return nil, dg
	}

	field, dg := flattenSpansField(filter.GetField())
	if dg != nil {
		return nil, dg
	}

	return &SpansFilterModel{
		Field:    field,
		Operator: operator,
	}, nil
}

func flattenSpansFields(ctx context.Context, spanFields []*dashboards.SpanField) (types.List, diag.Diagnostics) {
	if len(spanFields) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: spansFieldModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	spanFieldElements := make([]attr.Value, 0, len(spanFields))
	for _, field := range spanFields {
		flattenedField, dg := flattenSpansField(field)
		if dg != nil {
			diagnostics.Append(dg)
			continue
		}
		fieldElement, diags := types.ObjectValueFrom(ctx, spansFieldModelAttr(), flattenedField)
		if diags.HasError() {
			diagnostics = append(diagnostics, diags...)
			continue
		}
		spanFieldElements = append(spanFieldElements, fieldElement)
	}

	return types.ListValueMust(types.ObjectType{AttrTypes: spansFieldModelAttr()}, spanFieldElements), diagnostics
}

func flattenSpansField(field *dashboards.SpanField) (*SpansFieldModel, diag.Diagnostic) {
	if field == nil {
		return nil, nil
	}

	switch field.GetValue().(type) {
	case *dashboards.SpanField_MetadataField_:
		return &SpansFieldModel{
			Type:  types.StringValue("metadata"),
			Value: types.StringValue(dashboardProtoToSchemaSpanFieldMetadataField[field.GetMetadataField()]),
		}, nil
	case *dashboards.SpanField_TagField:
		return &SpansFieldModel{
			Type:  types.StringValue("tag"),
			Value: wrapperspbStringToTypeString(field.GetTagField()),
		}, nil
	case *dashboards.SpanField_ProcessTagField:
		return &SpansFieldModel{
			Type:  types.StringValue("process_tag"),
			Value: wrapperspbStringToTypeString(field.GetProcessTagField()),
		}, nil

	default:
		return nil, diag.NewErrorDiagnostic("Error Flatten Spans Field", "unknown spans field type")
	}
}

func flattenDataTable(ctx context.Context, table *dashboards.DataTable) (*WidgetDefinitionModel, diag.Diagnostics) {
	if table == nil {
		return nil, nil
	}

	query, diags := flattenDataTableQuery(ctx, table.GetQuery())
	if diags.HasError() {
		return nil, diags
	}

	columns, diags := flattenDataTableColumns(ctx, table.GetColumns())
	if diags.HasError() {
		return nil, diags
	}

	return &WidgetDefinitionModel{
		DataTable: &DataTableModel{
			Query:          query,
			ResultsPerPage: wrapperspbInt32ToTypeInt64(table.GetResultsPerPage()),
			RowStyle:       types.StringValue(dashboardRowStyleProtoToSchema[table.GetRowStyle()]),
			Columns:        columns,
			OrderBy:        flattenOrderBy(table.GetOrderBy()),
			DataModeType:   types.StringValue(dashboardProtoToSchemaDataModeType[table.GetDataModeType()]),
		},
	}, nil
}

func flattenDataTableQuery(ctx context.Context, query *dashboards.DataTable_Query) (*DataTableQueryModel, diag.Diagnostics) {
	if query == nil {
		return nil, nil
	}

	switch query.GetValue().(type) {
	case *dashboards.DataTable_Query_Logs:
		return flattenDataTableLogsQuery(ctx, query.GetLogs())
	case *dashboards.DataTable_Query_Metrics:
		return flattenDataTableMetricsQuery(ctx, query.GetMetrics())
	case *dashboards.DataTable_Query_Spans:
		return flattenDataTableSpansQuery(ctx, query.GetSpans())
	case *dashboards.DataTable_Query_Dataprime:
		return flattenDataTableDataPrimeQuery(ctx, query.GetDataprime())
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Data Table Query", "unknown data table query type")}
	}
}

func flattenDataTableDataPrimeQuery(ctx context.Context, dataPrime *dashboards.DataTable_DataprimeQuery) (*DataTableQueryModel, diag.Diagnostics) {
	if dataPrime == nil {
		return nil, nil
	}

	dataPrimeQuery := types.StringNull()
	if dataPrime.GetDataprimeQuery() != nil {
		dataPrimeQuery = types.StringValue(dataPrime.GetDataprimeQuery().GetText())
	}

	filters, diags := flattenDashboardFiltersSources(ctx, dataPrime.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	return &DataTableQueryModel{
		DataPrime: &DataPrimeModel{
			Query:   dataPrimeQuery,
			Filters: filters,
		},
	}, nil
}

func flattenDataTableLogsQuery(ctx context.Context, logs *dashboards.DataTable_LogsQuery) (*DataTableQueryModel, diag.Diagnostics) {
	if logs == nil {
		return nil, nil
	}

	filters, diags := flattenLogsFilters(ctx, logs.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	grouping, diags := flattenDataTableLogsQueryGrouping(ctx, logs.GetGrouping())
	if diags.HasError() {
		return nil, diags
	}

	return &DataTableQueryModel{
		Logs: &DataTableQueryLogsModel{
			LuceneQuery: wrapperspbStringToTypeString(logs.GetLuceneQuery().GetValue()),
			Filters:     filters,
			Grouping:    grouping,
		},
	}, nil
}

func flattenDataTableLogsQueryGrouping(ctx context.Context, grouping *dashboards.DataTable_LogsQuery_Grouping) (*DataTableLogsQueryGroupingModel, diag.Diagnostics) {
	if grouping == nil {
		return nil, nil
	}

	aggregations, diags := flattenGroupingAggregations(ctx, grouping.GetAggregations())
	if diags.HasError() {
		return nil, diags
	}

	groupBys, diags := flattenObservationFields(ctx, grouping.GetGroupBys())
	if diags.HasError() {
		return nil, diags
	}

	return &DataTableLogsQueryGroupingModel{
		Aggregations: aggregations,
		GroupBy:      wrappedStringSliceToTypeStringList(grouping.GetGroupBy()),
		GroupBys:     groupBys,
	}, nil
}

func flattenGroupingAggregations(ctx context.Context, aggregations []*dashboards.DataTable_LogsQuery_Aggregation) (types.List, diag.Diagnostics) {
	if len(aggregations) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: groupingAggregationModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	aggregationElements := make([]attr.Value, 0, len(aggregations))
	for _, aggregation := range aggregations {
		flattenedAggregation, diags := flattenGroupingAggregation(ctx, aggregation)
		if diags.HasError() {
			diagnostics.Append(diags...)
			continue
		}
		aggregationElement, diags := types.ObjectValueFrom(ctx, groupingAggregationModelAttr(), flattenedAggregation)
		if diags.HasError() {
			diagnostics.Append(diags...)
			continue
		}
		aggregationElements = append(aggregationElements, aggregationElement)
	}

	return types.ListValueMust(types.ObjectType{AttrTypes: groupingAggregationModelAttr()}, aggregationElements), diagnostics
}

func flattenGroupingAggregation(ctx context.Context, dataTableAggregation *dashboards.DataTable_LogsQuery_Aggregation) (*DataTableLogsAggregationModel, diag.Diagnostics) {
	aggregation, diags := flattenLogsAggregation(ctx, dataTableAggregation.GetAggregation())
	if diags.HasError() {
		return nil, diags
	}

	return &DataTableLogsAggregationModel{
		ID:          wrapperspbStringToTypeString(dataTableAggregation.GetId()),
		Name:        wrapperspbStringToTypeString(dataTableAggregation.GetName()),
		IsVisible:   wrapperspbBoolToTypeBool(dataTableAggregation.GetIsVisible()),
		Aggregation: aggregation,
	}, nil
}

func flattenDataTableMetricsQuery(ctx context.Context, metrics *dashboards.DataTable_MetricsQuery) (*DataTableQueryModel, diag.Diagnostics) {
	if metrics == nil {
		return nil, nil
	}

	filters, diags := flattenMetricsFilters(ctx, metrics.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	return &DataTableQueryModel{
		Metrics: &DataTableQueryMetricsModel{
			PromqlQuery: wrapperspbStringToTypeString(metrics.GetPromqlQuery().GetValue()),
			Filters:     filters,
		},
	}, nil
}

func flattenDataTableSpansQuery(ctx context.Context, spans *dashboards.DataTable_SpansQuery) (*DataTableQueryModel, diag.Diagnostics) {
	if spans == nil {
		return nil, nil
	}

	filters, diags := flattenSpansFilters(ctx, spans.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	grouping, diags := flattenDataTableSpansQueryGrouping(ctx, spans.GetGrouping())
	if diags.HasError() {
		return nil, diags
	}

	return &DataTableQueryModel{
		Spans: &DataTableQuerySpansModel{
			LuceneQuery: wrapperspbStringToTypeString(spans.GetLuceneQuery().GetValue()),
			Filters:     filters,
			Grouping:    grouping,
		},
	}, nil
}

func flattenDataTableSpansQueryGrouping(ctx context.Context, grouping *dashboards.DataTable_SpansQuery_Grouping) (*DataTableSpansQueryGroupingModel, diag.Diagnostics) {
	if grouping == nil {
		return nil, nil
	}

	aggregations, diags := flattenDataTableSpansQueryAggregations(ctx, grouping.GetAggregations())
	if diags.HasError() {
		return nil, diags
	}

	groupBy, diags := flattenSpansFields(ctx, grouping.GetGroupBy())
	if diags.HasError() {
		return nil, diags
	}
	return &DataTableSpansQueryGroupingModel{
		Aggregations: aggregations,
		GroupBy:      groupBy,
	}, nil
}

func flattenDataTableSpansQueryAggregations(ctx context.Context, aggregations []*dashboards.DataTable_SpansQuery_Aggregation) (types.List, diag.Diagnostics) {
	if len(aggregations) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: spansAggregationModelAttr()}), nil
	}
	var diagnostics diag.Diagnostics
	aggregationElements := make([]attr.Value, 0, len(aggregations))
	for _, aggregation := range aggregations {
		flattenedAggregation, dg := flattenDataTableSpansQueryAggregation(aggregation)
		if dg != nil {
			diagnostics.Append(dg)
			continue
		}
		aggregationElement, diags := types.ObjectValueFrom(ctx, spansAggregationModelAttr(), flattenedAggregation)
		if diags.HasError() {
			diagnostics = append(diagnostics, diags...)
			continue
		}
		aggregationElements = append(aggregationElements, aggregationElement)
	}
	return types.ListValueMust(types.ObjectType{AttrTypes: spansAggregationModelAttr()}, aggregationElements), diagnostics
}

func flattenDataTableSpansQueryAggregation(spanAggregation *dashboards.DataTable_SpansQuery_Aggregation) (*DataTableSpansAggregationModel, diag.Diagnostic) {
	if spanAggregation == nil {
		return nil, nil
	}

	aggregation, dg := flattenSpansAggregation(spanAggregation.GetAggregation())
	if dg != nil {
		return nil, dg
	}

	return &DataTableSpansAggregationModel{
		ID:          wrapperspbStringToTypeString(spanAggregation.GetId()),
		Name:        wrapperspbStringToTypeString(spanAggregation.GetName()),
		IsVisible:   wrapperspbBoolToTypeBool(spanAggregation.GetIsVisible()),
		Aggregation: aggregation,
	}, nil
}

func flattenSpansAggregation(aggregation *dashboards.SpansAggregation) (*SpansAggregationModel, diag.Diagnostic) {
	if aggregation == nil || aggregation.GetAggregation() == nil {
		return nil, nil
	}
	switch aggregation := aggregation.GetAggregation().(type) {
	case *dashboards.SpansAggregation_MetricAggregation_:
		return &SpansAggregationModel{
			Type:            types.StringValue("metric"),
			AggregationType: types.StringValue(dashboardProtoToSchemaSpansAggregationMetricAggregationType[aggregation.MetricAggregation.GetAggregationType()]),
			Field:           types.StringValue(dashboardProtoToSchemaSpansAggregationMetricField[aggregation.MetricAggregation.GetMetricField()]),
		}, nil
	case *dashboards.SpansAggregation_DimensionAggregation_:
		return &SpansAggregationModel{
			Type:            types.StringValue("dimension"),
			AggregationType: types.StringValue(dashboardProtoToSchemaSpansAggregationDimensionAggregationType[aggregation.DimensionAggregation.GetAggregationType()]),
			Field:           types.StringValue(dashboardSchemaToProtoSpansAggregationDimensionField[aggregation.DimensionAggregation.GetDimensionField()]),
		}, nil
	default:
		return nil, diag.NewErrorDiagnostic("Error Flatten Span Aggregation", fmt.Sprintf("unknown aggregation type %T", aggregation))
	}
}

func flattenDataTableColumns(ctx context.Context, columns []*dashboards.DataTable_Column) (types.List, diag.Diagnostics) {
	if len(columns) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: dataTableColumnModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	columnElements := make([]attr.Value, 0, len(columns))
	for _, column := range columns {
		flattenedColumn := flattenDataTableColumn(column)
		columnElement, diags := types.ObjectValueFrom(ctx, dataTableColumnModelAttr(), flattenedColumn)
		if diags.HasError() {
			diagnostics = append(diagnostics, diags...)
			continue
		}
		columnElements = append(columnElements, columnElement)
	}

	return types.ListValueMust(types.ObjectType{AttrTypes: dataTableColumnModelAttr()}, columnElements), diagnostics
}

func flattenDataTableColumn(column *dashboards.DataTable_Column) *DataTableColumnModel {
	if column == nil {
		return nil
	}
	return &DataTableColumnModel{
		Field: wrapperspbStringToTypeString(column.GetField()),
		Width: wrapperspbInt32ToTypeInt64(column.GetWidth()),
	}
}

func flattenOrderBy(orderBy *dashboards.OrderingField) *OrderByModel {
	if orderBy == nil {
		return nil
	}
	return &OrderByModel{
		Field:          wrapperspbStringToTypeString(orderBy.GetField()),
		OrderDirection: types.StringValue(dashboardOrderDirectionProtoToSchema[orderBy.GetOrderDirection()]),
	}
}

func flattenGauge(ctx context.Context, gauge *dashboards.Gauge) (*WidgetDefinitionModel, diag.Diagnostics) {
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

	return &WidgetDefinitionModel{
		Gauge: &GaugeModel{
			Query:        query,
			Min:          wrapperspbDoubleToTypeFloat64(gauge.GetMin()),
			Max:          wrapperspbDoubleToTypeFloat64(gauge.GetMax()),
			ShowInnerArc: wrapperspbBoolToTypeBool(gauge.GetShowInnerArc()),
			ShowOuterArc: wrapperspbBoolToTypeBool(gauge.GetShowOuterArc()),
			Unit:         types.StringValue(dashboardProtoToSchemaGaugeUnit[gauge.GetUnit()]),
			Thresholds:   thresholds,
			DataModeType: types.StringValue(dashboardProtoToSchemaDataModeType[gauge.GetDataModeType()]),
			ThresholdBy:  types.StringValue(dashboardProtoToSchemaGaugeThresholdBy[gauge.GetThresholdBy()]),
		},
	}, nil
}

func flattenGaugeThresholds(ctx context.Context, thresholds []*dashboards.Gauge_Threshold) (types.List, diag.Diagnostics) {
	if len(thresholds) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: gaugeThresholdModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	thresholdElements := make([]attr.Value, 0, len(thresholds))
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

func flattenGaugeThreshold(threshold *dashboards.Gauge_Threshold) *GaugeThresholdModel {
	if threshold == nil {
		return nil
	}
	return &GaugeThresholdModel{
		From:  wrapperspbDoubleToTypeFloat64(threshold.GetFrom()),
		Color: wrapperspbStringToTypeString(threshold.GetColor()),
	}
}

func flattenGaugeQueries(ctx context.Context, query *dashboards.Gauge_Query) (*GaugeQueryModel, diag.Diagnostics) {
	switch query.GetValue().(type) {
	case *dashboards.Gauge_Query_Metrics:
		return flattenGaugeQueryMetrics(ctx, query.GetMetrics())
	case *dashboards.Gauge_Query_Logs:
		return flattenGaugeQueryLogs(ctx, query.GetLogs())
	case *dashboards.Gauge_Query_Spans:
		return flattenGaugeQuerySpans(ctx, query.GetSpans())
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Gauge Query", fmt.Sprintf("unknown query type %T", query))}
	}
}

func flattenGaugeQueryMetrics(ctx context.Context, metrics *dashboards.Gauge_MetricsQuery) (*GaugeQueryModel, diag.Diagnostics) {
	if metrics == nil {
		return nil, nil
	}

	filters, diags := flattenMetricsFilters(ctx, metrics.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	return &GaugeQueryModel{
		Metrics: &GaugeQueryMetricsModel{
			PromqlQuery: wrapperspbStringToTypeString(metrics.GetPromqlQuery().GetValue()),
			Aggregation: types.StringValue(dashboardProtoToSchemaGaugeAggregation[metrics.GetAggregation()]),
			Filters:     filters,
		},
	}, nil
}

func flattenGaugeQueryLogs(ctx context.Context, logs *dashboards.Gauge_LogsQuery) (*GaugeQueryModel, diag.Diagnostics) {
	if logs == nil {
		return nil, nil
	}

	filters, diags := flattenLogsFilters(ctx, logs.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	logsAggregation, diags := flattenLogsAggregation(ctx, logs.GetLogsAggregation())
	if diags.HasError() {
		return nil, diags
	}

	return &GaugeQueryModel{
		Logs: &GaugeQueryLogsModel{
			LuceneQuery:     wrapperspbStringToTypeString(logs.GetLuceneQuery().GetValue()),
			LogsAggregation: logsAggregation,
			Filters:         filters,
		},
	}, nil
}

func flattenGaugeQuerySpans(ctx context.Context, spans *dashboards.Gauge_SpansQuery) (*GaugeQueryModel, diag.Diagnostics) {
	if spans == nil {
		return nil, nil
	}

	filters, diags := flattenSpansFilters(ctx, spans.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	spansAggregation, dg := flattenSpansAggregation(spans.GetSpansAggregation())
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	return &GaugeQueryModel{
		Spans: &GaugeQuerySpansModel{
			LuceneQuery:      wrapperspbStringToTypeString(spans.GetLuceneQuery().GetValue()),
			Filters:          filters,
			SpansAggregation: spansAggregation,
		},
	}, nil
}

func flattenPieChart(ctx context.Context, pieChart *dashboards.PieChart) (*WidgetDefinitionModel, diag.Diagnostics) {
	if pieChart == nil {
		return nil, nil
	}

	query, diags := flattenPieChartQueries(ctx, pieChart.GetQuery())
	if diags != nil {
		return nil, diags
	}

	return &WidgetDefinitionModel{
		PieChart: &PieChartModel{
			Query:              query,
			MaxSlicesPerChart:  wrapperspbInt32ToTypeInt64(pieChart.GetMaxSlicesPerChart()),
			MinSlicePercentage: wrapperspbInt32ToTypeInt64(pieChart.GetMinSlicePercentage()),
			StackDefinition:    flattenPieChartStackDefinition(pieChart.GetStackDefinition()),
			LabelDefinition:    flattenPieChartLabelDefinition(pieChart.GetLabelDefinition()),
			ShowLegend:         wrapperspbBoolToTypeBool(pieChart.GetShowLegend()),
			GroupNameTemplate:  wrapperspbStringToTypeString(pieChart.GetGroupNameTemplate()),
			Unit:               types.StringValue(dashboardProtoToSchemaUnit[pieChart.GetUnit()]),
			ColorScheme:        wrapperspbStringToTypeString(pieChart.GetColorScheme()),
			DataModeType:       types.StringValue(dashboardProtoToSchemaDataModeType[pieChart.GetDataModeType()]),
		},
	}, nil
}

func flattenPieChartQueries(ctx context.Context, query *dashboards.PieChart_Query) (*PieChartQueryModel, diag.Diagnostics) {
	if query == nil {
		return nil, nil
	}

	switch query.GetValue().(type) {
	case *dashboards.PieChart_Query_Metrics:
		return flattenPieChartQueryMetrics(ctx, query.GetMetrics())
	case *dashboards.PieChart_Query_Logs:
		return flattenPieChartQueryLogs(ctx, query.GetLogs())
	case *dashboards.PieChart_Query_Spans:
		return flattenPieChartQuerySpans(ctx, query.GetSpans())
	case *dashboards.PieChart_Query_Dataprime:
		return flattenPieChartDataPrimeQuery(ctx, query.GetDataprime())
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Pie Chart Query", fmt.Sprintf("unknown query type %T", query))}
	}
}

func flattenPieChartStackDefinition(stackDefinition *dashboards.PieChart_StackDefinition) *PieChartStackDefinitionModel {
	if stackDefinition == nil {
		return nil
	}

	return &PieChartStackDefinitionModel{
		MaxSlicesPerStack: wrapperspbInt32ToTypeInt64(stackDefinition.GetMaxSlicesPerStack()),
		StackNameTemplate: wrapperspbStringToTypeString(stackDefinition.GetStackNameTemplate()),
	}
}

func flattenPieChartLabelDefinition(labelDefinition *dashboards.PieChart_LabelDefinition) *LabelDefinitionModel {
	if labelDefinition == nil {
		return nil
	}
	return &LabelDefinitionModel{
		LabelSource:    types.StringValue(dashboardProtoToSchemaPieChartLabelSource[labelDefinition.GetLabelSource()]),
		IsVisible:      wrapperspbBoolToTypeBool(labelDefinition.GetIsVisible()),
		ShowName:       wrapperspbBoolToTypeBool(labelDefinition.GetShowName()),
		ShowValue:      wrapperspbBoolToTypeBool(labelDefinition.GetShowValue()),
		ShowPercentage: wrapperspbBoolToTypeBool(labelDefinition.GetShowPercentage()),
	}
}

func flattenPieChartQueryMetrics(ctx context.Context, metrics *dashboards.PieChart_MetricsQuery) (*PieChartQueryModel, diag.Diagnostics) {
	if metrics == nil {
		return nil, nil
	}

	filters, diags := flattenMetricsFilters(ctx, metrics.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	return &PieChartQueryModel{
		Metrics: &PieChartQueryMetricsModel{
			PromqlQuery:      wrapperspbStringToTypeString(metrics.GetPromqlQuery().GetValue()),
			Filters:          filters,
			GroupNames:       wrappedStringSliceToTypeStringList(metrics.GetGroupNames()),
			StackedGroupName: wrapperspbStringToTypeString(metrics.GetStackedGroupName()),
		},
	}, nil
}

func flattenPieChartQueryLogs(ctx context.Context, logs *dashboards.PieChart_LogsQuery) (*PieChartQueryModel, diag.Diagnostics) {
	if logs == nil {
		return nil, nil
	}

	filters, diags := flattenLogsFilters(ctx, logs.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	aggregation, diags := flattenLogsAggregation(ctx, logs.GetAggregation())
	if diags.HasError() {
		return nil, diags
	}

	groupNamesFields, diags := flattenObservationFields(ctx, logs.GetGroupNamesFields())
	if diags.HasError() {
		return nil, diags
	}

	stackedGroupNameField, diags := flattenObservationField(ctx, logs.GetStackedGroupNameField())
	if diags.HasError() {
		return nil, diags
	}

	return &PieChartQueryModel{
		Logs: &PieChartQueryLogsModel{
			LuceneQuery:           wrapperspbStringToTypeString(logs.GetLuceneQuery().GetValue()),
			Aggregation:           aggregation,
			Filters:               filters,
			GroupNames:            wrappedStringSliceToTypeStringList(logs.GetGroupNames()),
			StackedGroupName:      wrapperspbStringToTypeString(logs.GetStackedGroupName()),
			GroupNamesFields:      groupNamesFields,
			StackedGroupNameField: stackedGroupNameField,
		},
	}, nil
}

func flattenPieChartQuerySpans(ctx context.Context, spans *dashboards.PieChart_SpansQuery) (*PieChartQueryModel, diag.Diagnostics) {
	if spans == nil {
		return nil, nil
	}

	filters, diags := flattenSpansFilters(ctx, spans.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	aggregation, dg := flattenSpansAggregation(spans.GetAggregation())
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	stackedGroupName, dg := flattenSpansField(spans.GetStackedGroupName())
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	groupNames, diags := flattenSpansFields(ctx, spans.GetGroupNames())
	if diags.HasError() {
		return nil, diags
	}

	return &PieChartQueryModel{
		Spans: &PieChartQuerySpansModel{
			LuceneQuery:      wrapperspbStringToTypeString(spans.GetLuceneQuery().GetValue()),
			Filters:          filters,
			Aggregation:      aggregation,
			GroupNames:       groupNames,
			StackedGroupName: stackedGroupName,
		},
	}, nil
}

func flattenPieChartDataPrimeQuery(ctx context.Context, dataPrime *dashboards.PieChart_DataprimeQuery) (*PieChartQueryModel, diag.Diagnostics) {
	if dataPrime == nil {
		return nil, nil
	}

	filters, diags := flattenDashboardFiltersSources(ctx, dataPrime.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	return &PieChartQueryModel{
		DataPrime: &PieChartQueryDataPrimeModel{
			Query:            types.StringValue(dataPrime.GetDataprimeQuery().GetText()),
			Filters:          filters,
			GroupNames:       wrappedStringSliceToTypeStringList(dataPrime.GetGroupNames()),
			StackedGroupName: wrapperspbStringToTypeString(dataPrime.GetStackedGroupName()),
		},
	}, nil
}

func flattenBarChart(ctx context.Context, barChart *dashboards.BarChart) (*WidgetDefinitionModel, diag.Diagnostics) {
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

	return &WidgetDefinitionModel{
		BarChart: &BarChartModel{
			Query:             query,
			MaxBarsPerChart:   wrapperspbInt32ToTypeInt64(barChart.GetMaxBarsPerChart()),
			GroupNameTemplate: wrapperspbStringToTypeString(barChart.GetGroupNameTemplate()),
			StackDefinition:   flattenBarChartStackDefinition(barChart.GetStackDefinition()),
			ScaleType:         types.StringValue(dashboardProtoToSchemaScaleType[barChart.GetScaleType()]),
			ColorsBy:          colorsBy,
			XAxis:             xAxis,
			Unit:              types.StringValue(dashboardProtoToSchemaUnit[barChart.GetUnit()]),
			SortBy:            types.StringValue(dashboardProtoToSchemaSortBy[barChart.GetSortBy()]),
			ColorScheme:       wrapperspbStringToTypeString(barChart.GetColorScheme()),
			DataModeType:      types.StringValue(dashboardProtoToSchemaDataModeType[barChart.GetDataModeType()]),
		},
	}, nil
}

func flattenBarChartXAxis(axis *dashboards.BarChart_XAxis) (*BarChartXAxisModel, diag.Diagnostic) {
	if axis == nil {
		return nil, nil
	}

	switch axis.GetType().(type) {
	case *dashboards.BarChart_XAxis_Time:
		return &BarChartXAxisModel{
			Time: &BarChartXAxisTimeModel{
				Interval:         types.StringValue(axis.GetTime().GetInterval().AsDuration().String()),
				BucketsPresented: wrapperspbInt32ToTypeInt64(axis.GetTime().GetBucketsPresented()),
			},
		}, nil
	case *dashboards.BarChart_XAxis_Value:
		return &BarChartXAxisModel{
			Value: &BarChartXAxisValueModel{},
		}, nil
	default:
		return nil, diag.NewErrorDiagnostic("Error Flatten BarChart XAxis", fmt.Sprintf("unknown bar chart x axis type: %T", axis.GetType()))
	}

}

func flattenBarChartQuery(ctx context.Context, query *dashboards.BarChart_Query) (*BarChartQueryModel, diag.Diagnostics) {
	if query == nil {
		return nil, nil
	}

	switch queryType := query.GetValue().(type) {
	case *dashboards.BarChart_Query_Logs:
		return flattenBarChartQueryLogs(ctx, queryType.Logs)
	case *dashboards.BarChart_Query_Spans:
		return flattenBarChartQuerySpans(ctx, queryType.Spans)
	case *dashboards.BarChart_Query_Metrics:
		return flattenBarChartQueryMetrics(ctx, queryType.Metrics)
	case *dashboards.BarChart_Query_Dataprime:
		return flattenBarChartQueryDataPrime(ctx, queryType.Dataprime)
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten BarChart Query", fmt.Sprintf("unknown bar chart query type: %T", query.GetValue()))}
	}
}

func flattenBarChartQueryLogs(ctx context.Context, logs *dashboards.BarChart_LogsQuery) (*BarChartQueryModel, diag.Diagnostics) {
	if logs == nil {
		return nil, nil
	}

	filters, diags := flattenLogsFilters(ctx, logs.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	aggregation, diags := flattenLogsAggregation(ctx, logs.GetAggregation())
	if diags.HasError() {
		return nil, diags
	}

	groupNamesFields, diags := flattenObservationFields(ctx, logs.GetGroupNamesFields())
	if diags.HasError() {
		return nil, diags
	}

	stackedGroupNameField, diags := flattenObservationField(ctx, logs.GetStackedGroupNameField())
	if diags.HasError() {
		return nil, diags
	}

	flattenedLogs := &BarChartQueryLogsModel{
		LuceneQuery:           wrapperspbStringToTypeString(logs.GetLuceneQuery().GetValue()),
		Filters:               filters,
		Aggregation:           aggregation,
		GroupNames:            wrappedStringSliceToTypeStringList(logs.GetGroupNames()),
		StackedGroupName:      wrapperspbStringToTypeString(logs.GetStackedGroupName()),
		GroupNamesFields:      groupNamesFields,
		StackedGroupNameField: stackedGroupNameField,
	}

	logsObject, diags := types.ObjectValueFrom(ctx, barChartLogsQueryAttr(), flattenedLogs)
	if diags.HasError() {
		return nil, diags
	}
	return &BarChartQueryModel{
		Logs:      logsObject,
		Metrics:   types.ObjectNull(barChartMetricsQueryAttr()),
		Spans:     types.ObjectNull(barChartSpansQueryAttr()),
		DataPrime: types.ObjectNull(barChartDataPrimeQueryAttr()),
	}, nil
}

func flattenObservationFields(ctx context.Context, namesFields []*dashboards.ObservationField) (types.List, diag.Diagnostics) {
	if len(namesFields) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: observationFieldAttributes()}), nil
	}

	var diagnostics diag.Diagnostics
	fieldElements := make([]attr.Value, 0, len(namesFields))
	for _, field := range namesFields {
		flattenedField, diags := flattenObservationField(ctx, field)
		if diags != nil {
			diagnostics.Append(diags...)
			continue
		}
		fieldElement, diags := types.ObjectValueFrom(ctx, observationFieldAttributes(), flattenedField)
		if diags.HasError() {
			diagnostics.Append(diags...)
			continue
		}
		fieldElements = append(fieldElements, fieldElement)
	}

	if diagnostics.HasError() {
		return types.ListNull(types.ObjectType{AttrTypes: observationFieldAttributes()}), diagnostics
	}

	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: observationFieldAttributes()}, fieldElements)
}

func flattenObservationField(ctx context.Context, field *dashboards.ObservationField) (types.Object, diag.Diagnostics) {
	if field == nil {
		return types.ObjectNull(observationFieldAttributes()), nil
	}

	return types.ObjectValueFrom(ctx, observationFieldAttributes(), flattenLogsFieldModel(field))
}

func flattenLogsFieldModel(field *dashboards.ObservationField) *ObservationFieldModel {
	return &ObservationFieldModel{
		Keypath: wrappedStringSliceToTypeStringList(field.GetKeypath()),
		Scope:   types.StringValue(dashboardProtoToSchemaObservationFieldScope[field.GetScope()]),
	}
}

func flattenBarChartQuerySpans(ctx context.Context, spans *dashboards.BarChart_SpansQuery) (*BarChartQueryModel, diag.Diagnostics) {
	if spans == nil {
		return nil, nil
	}

	filters, diags := flattenSpansFilters(ctx, spans.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	aggregation, dg := flattenSpansAggregation(spans.GetAggregation())
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	groupNames, diags := flattenSpansFields(ctx, spans.GetGroupNames())
	if diags.HasError() {
		return nil, diags
	}

	stackedGroupName, dg := flattenSpansField(spans.GetStackedGroupName())
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	flattenedSpans := &BarChartQuerySpansModel{
		LuceneQuery:      wrapperspbStringToTypeString(spans.GetLuceneQuery().GetValue()),
		Aggregation:      aggregation,
		Filters:          filters,
		GroupNames:       groupNames,
		StackedGroupName: stackedGroupName,
	}
	spansObject, diags := types.ObjectValueFrom(ctx, barChartSpansQueryAttr(), flattenedSpans)
	if diags.HasError() {
		return nil, diags
	}

	return &BarChartQueryModel{
		Spans:     spansObject,
		Metrics:   types.ObjectNull(barChartMetricsQueryAttr()),
		Logs:      types.ObjectNull(barChartLogsQueryAttr()),
		DataPrime: types.ObjectNull(barChartDataPrimeQueryAttr()),
	}, nil
}

func flattenBarChartQueryMetrics(ctx context.Context, metrics *dashboards.BarChart_MetricsQuery) (*BarChartQueryModel, diag.Diagnostics) {
	if metrics == nil {
		return nil, nil
	}

	filters, diags := flattenMetricsFilters(ctx, metrics.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	flattenedMetric := &BarChartQueryMetricsModel{
		PromqlQuery:      wrapperspbStringToTypeString(metrics.GetPromqlQuery().GetValue()),
		Filters:          filters,
		GroupNames:       wrappedStringSliceToTypeStringList(metrics.GetGroupNames()),
		StackedGroupName: wrapperspbStringToTypeString(metrics.GetStackedGroupName()),
	}

	metricObject, diags := types.ObjectValueFrom(ctx, barChartMetricsQueryAttr(), flattenedMetric)
	if diags.HasError() {
		return nil, diags
	}
	return &BarChartQueryModel{
		Logs:      types.ObjectNull(barChartLogsQueryAttr()),
		Spans:     types.ObjectNull(barChartSpansQueryAttr()),
		DataPrime: types.ObjectNull(barChartDataPrimeQueryAttr()),
		Metrics:   metricObject,
	}, nil
}

func flattenBarChartQueryDataPrime(ctx context.Context, dataPrime *dashboards.BarChart_DataprimeQuery) (*BarChartQueryModel, diag.Diagnostics) {
	if dataPrime == nil {
		return nil, nil
	}

	filters, diags := flattenDashboardFiltersSources(ctx, dataPrime.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	flattenedDataPrime := &BarChartQueryDataPrimeModel{
		Query:            types.StringValue(dataPrime.GetDataprimeQuery().GetText()),
		Filters:          filters,
		GroupNames:       wrappedStringSliceToTypeStringList(dataPrime.GetGroupNames()),
		StackedGroupName: wrapperspbStringToTypeString(dataPrime.GetStackedGroupName()),
	}

	dataPrimeObject, diags := types.ObjectValueFrom(ctx, barChartDataPrimeQueryAttr(), flattenedDataPrime)
	if diags.HasError() {
		return nil, diags
	}
	return &BarChartQueryModel{
		Logs:      types.ObjectNull(barChartLogsQueryAttr()),
		Spans:     types.ObjectNull(barChartSpansQueryAttr()),
		Metrics:   types.ObjectNull(barChartMetricsQueryAttr()),
		DataPrime: dataPrimeObject,
	}, nil
}

func flattenBarChartStackDefinition(stackDefinition *dashboards.BarChart_StackDefinition) *BarChartStackDefinitionModel {
	if stackDefinition == nil {
		return nil
	}

	return &BarChartStackDefinitionModel{
		MaxSlicesPerBar:   wrapperspbInt32ToTypeInt64(stackDefinition.GetMaxSlicesPerBar()),
		StackNameTemplate: wrapperspbStringToTypeString(stackDefinition.GetStackNameTemplate()),
	}
}

func flattenHorizontalBarChartStackDefinition(stackDefinition *dashboards.HorizontalBarChart_StackDefinition) *BarChartStackDefinitionModel {
	if stackDefinition == nil {
		return nil
	}

	return &BarChartStackDefinitionModel{
		MaxSlicesPerBar:   wrapperspbInt32ToTypeInt64(stackDefinition.GetMaxSlicesPerBar()),
		StackNameTemplate: wrapperspbStringToTypeString(stackDefinition.GetStackNameTemplate()),
	}
}

func flattenBarChartColorsBy(colorsBy *dashboards.ColorsBy) (types.String, diag.Diagnostic) {
	if colorsBy == nil {
		return types.StringNull(), nil
	}
	switch colorsBy.GetValue().(type) {
	case *dashboards.ColorsBy_GroupBy:
		return types.StringValue("group_by"), nil
	case *dashboards.ColorsBy_Stack:
		return types.StringValue("stack"), nil
	case *dashboards.ColorsBy_Aggregation:
		return types.StringValue("aggregation"), nil
	default:
		return types.StringNull(), diag.NewErrorDiagnostic("", fmt.Sprintf("unknown colors by type %T", colorsBy))
	}
}

func flattenDashboardVariables(ctx context.Context, variables []*dashboards.Variable) (types.List, diag.Diagnostics) {
	if len(variables) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: dashboardsVariablesModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	variablesElements := make([]attr.Value, 0, len(variables))
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

func flattenDashboardVariable(ctx context.Context, variable *dashboards.Variable) (*DashboardVariableModel, diag.Diagnostics) {
	if variable == nil {
		return nil, nil
	}

	definition, diags := flattenDashboardVariableDefinition(ctx, variable.GetDefinition())
	if diags.HasError() {
		return nil, diags
	}

	return &DashboardVariableModel{
		Name:        wrapperspbStringToTypeString(variable.GetName()),
		DisplayName: wrapperspbStringToTypeString(variable.GetDisplayName()),
		Definition:  definition,
	}, nil
}

func flattenDashboardVariableDefinition(ctx context.Context, variableDefinition *dashboards.Variable_Definition) (*DashboardVariableDefinitionModel, diag.Diagnostics) {
	if variableDefinition == nil {
		return nil, nil
	}

	switch variableDefinition.GetValue().(type) {
	case *dashboards.Variable_Definition_Constant:
		return &DashboardVariableDefinitionModel{
			ConstantValue: wrapperspbStringToTypeString(variableDefinition.GetConstant().GetValue()),
		}, nil
	case *dashboards.Variable_Definition_MultiSelect:
		return flattenDashboardVariableDefinitionMultiSelect(ctx, variableDefinition.GetMultiSelect())
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Dashboard Variable Definition", fmt.Sprintf("unknown variable definition type %T", variableDefinition))}
	}
}

func flattenDashboardVariableDefinitionMultiSelect(ctx context.Context, multiSelect *dashboards.MultiSelect) (*DashboardVariableDefinitionModel, diag.Diagnostics) {
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
			ValuesOrderDirection: types.StringValue(dashboardOrderDirectionProtoToSchema[multiSelect.GetValuesOrderDirection()]),
			Source:               source,
		},
	}, nil
}

func flattenDashboardVariableSource(ctx context.Context, source *dashboards.MultiSelect_Source) (*VariableMultiSelectSourceModel, diag.Diagnostics) {
	if source == nil {
		return nil, nil
	}

	result := &VariableMultiSelectSourceModel{
		LogsPath:     types.StringNull(),
		ConstantList: types.ListNull(types.StringType),
		Query:        types.ObjectNull(multiSelectQueryAttr()),
	}

	switch source.GetValue().(type) {
	case *dashboards.MultiSelect_Source_LogsPath:
		result.LogsPath = wrapperspbStringToTypeString(source.GetLogsPath().GetValue())
	case *dashboards.MultiSelect_Source_MetricLabel:
		result.MetricLabel = &MetricMultiSelectSourceModel{
			MetricName: wrapperspbStringToTypeString(source.GetMetricLabel().GetMetricName()),
			Label:      wrapperspbStringToTypeString(source.GetMetricLabel().GetLabel()),
		}
	case *dashboards.MultiSelect_Source_ConstantList:
		result.ConstantList = wrappedStringSliceToTypeStringList(source.GetConstantList().GetValues())
	case *dashboards.MultiSelect_Source_SpanField:
		spansField, dg := flattenSpansField(source.GetSpanField().GetValue())
		if dg != nil {
			return nil, diag.Diagnostics{dg}
		}
		result.SpanField = spansField
	case *dashboards.MultiSelect_Source_Query:
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

func flattenDashboardVariableDefinitionMultiSelectQuery(ctx context.Context, querySource *dashboards.MultiSelect_QuerySource) (types.Object, diag.Diagnostics) {
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
		RefreshStrategy:     types.StringValue(dashboardProtoToSchemaRefreshStrategy[querySource.GetRefreshStrategy()]),
		ValueDisplayOptions: valueDisplayOptions,
	})
}

func flattenDashboardVariableDefinitionMultiSelectQueryModel(ctx context.Context, query *dashboards.MultiSelect_Query) (types.Object, diag.Diagnostics) {
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
	case *dashboards.MultiSelect_Query_LogsQuery_:
		multiSelectQueryModel.Logs, diags = flattenDashboardVariableDefinitionMultiSelectQueryLogsModel(ctx, queryType.LogsQuery)
	case *dashboards.MultiSelect_Query_MetricsQuery_:
		multiSelectQueryModel.Metrics, diags = flattenDashboardVariableDefinitionMultiSelectQueryMetricsModel(ctx, queryType.MetricsQuery)
	case *dashboards.MultiSelect_Query_SpansQuery_:
		multiSelectQueryModel.Spans, diags = flattenDashboardVariableDefinitionMultiSelectQuerySpansModel(ctx, queryType.SpansQuery)
	}

	if diags.HasError() {
		return types.ObjectNull(multiSelectQueryModelAttr()), diags
	}

	return types.ObjectValueFrom(ctx, multiSelectQueryModelAttr(), multiSelectQueryModel)
}

func flattenDashboardVariableDefinitionMultiSelectQueryLogsModel(ctx context.Context, query *dashboards.MultiSelect_Query_LogsQuery) (types.Object, diag.Diagnostics) {
	if query == nil {
		return types.ObjectNull(multiSelectQueryLogsQueryModelAttr()), nil
	}

	logsQuery := &MultiSelectLogsQueryModel{
		FieldName:  types.ObjectNull(multiSelectQueryLogsQueryFieldNameModelAttr()),
		FieldValue: types.ObjectNull(multiSelectQueryLogsQueryFieldValueModelAttr()),
	}

	var diags diag.Diagnostics
	switch queryType := query.GetType().GetValue().(type) {
	case *dashboards.MultiSelect_Query_LogsQuery_Type_FieldName_:
		logsQuery.FieldName, diags = flattenDashboardVariableDefinitionMultiSelectQueryLogsFieldNameModel(ctx, queryType.FieldName)
	case *dashboards.MultiSelect_Query_LogsQuery_Type_FieldValue_:
		logsQuery.FieldValue, diags = flattenDashboardVariableDefinitionMultiSelectQueryLogsFieldValueModel(ctx, queryType.FieldValue)
	}

	if diags.HasError() {
		return types.ObjectNull(multiSelectQueryLogsQueryModelAttr()), diags
	}

	return types.ObjectValueFrom(ctx, multiSelectQueryLogsQueryModelAttr(), logsQuery)
}

func flattenDashboardVariableDefinitionMultiSelectQueryLogsFieldNameModel(ctx context.Context, name *dashboards.MultiSelect_Query_LogsQuery_Type_FieldName) (types.Object, diag.Diagnostics) {
	if name == nil {
		return types.ObjectNull(multiSelectQueryLogsQueryFieldNameModelAttr()), nil
	}

	return types.ObjectValueFrom(ctx, multiSelectQueryLogsQueryFieldNameModelAttr(), &LogFieldNameModel{
		LogRegex: wrapperspbStringToTypeString(name.GetLogRegex()),
	})
}

func flattenDashboardVariableDefinitionMultiSelectQueryLogsFieldValueModel(ctx context.Context, value *dashboards.MultiSelect_Query_LogsQuery_Type_FieldValue) (types.Object, diag.Diagnostics) {
	if value == nil {
		return types.ObjectNull(multiSelectQueryLogsQueryFieldValueModelAttr()), nil
	}

	observationField, diags := flattenObservationField(ctx, value.GetObservationField())
	if diags.HasError() {
		return types.ObjectNull(multiSelectQueryLogsQueryFieldValueModelAttr()), diags
	}

	return types.ObjectValueFrom(ctx, multiSelectQueryLogsQueryFieldValueModelAttr(), &FieldValueModel{
		ObservationField: observationField,
	})
}

func flattenDashboardVariableDefinitionMultiSelectQueryMetricsModel(ctx context.Context, query *dashboards.MultiSelect_Query_MetricsQuery) (types.Object, diag.Diagnostics) {
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
	case *dashboards.MultiSelect_Query_MetricsQuery_Type_MetricName_:
		metricQuery.MetricName, diags = flattenDashboardVariableDefinitionMultiSelectQueryMetricsMetricNameModel(ctx, queryType.MetricName)
	case *dashboards.MultiSelect_Query_MetricsQuery_Type_LabelName_:
		metricQuery.LabelName, diags = flattenDashboardVariableDefinitionMultiSelectQueryMetricsLabelNameModel(ctx, queryType.LabelName)
	case *dashboards.MultiSelect_Query_MetricsQuery_Type_LabelValue_:
		metricQuery.LabelValue, diags = flattenDashboardVariableDefinitionMultiSelectQueryMetricsLabelValueModel(ctx, queryType.LabelValue)
	}

	if diags.HasError() {
		return types.ObjectNull(multiSelectQueryMetricsQueryModelAttr()), diags
	}

	return types.ObjectValueFrom(ctx, multiSelectQueryMetricsQueryModelAttr(), metricQuery)
}

func flattenDashboardVariableDefinitionMultiSelectQueryMetricsMetricNameModel(ctx context.Context, name *dashboards.MultiSelect_Query_MetricsQuery_Type_MetricName) (types.Object, diag.Diagnostics) {
	if name == nil {
		return types.ObjectNull(multiSelectQueryMetricsNameAttr()), nil
	}

	return types.ObjectValueFrom(ctx, multiSelectQueryMetricsNameAttr(), &MetricAndLabelNameModel{
		MetricRegex: wrapperspbStringToTypeString(name.GetMetricRegex()),
	})
}

func flattenDashboardVariableDefinitionMultiSelectQueryMetricsLabelNameModel(ctx context.Context, name *dashboards.MultiSelect_Query_MetricsQuery_Type_LabelName) (types.Object, diag.Diagnostics) {
	if name == nil {
		return types.ObjectNull(multiSelectQueryMetricsNameAttr()), nil
	}

	return types.ObjectValueFrom(ctx, multiSelectQueryMetricsNameAttr(), &MetricAndLabelNameModel{
		MetricRegex: wrapperspbStringToTypeString(name.GetMetricRegex()),
	})
}

func flattenDashboardVariableDefinitionMultiSelectQueryMetricsLabelValueModel(ctx context.Context, value *dashboards.MultiSelect_Query_MetricsQuery_Type_LabelValue) (types.Object, diag.Diagnostics) {
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

func flattenMultiSelectQueryMetricsQueryMetricsLabelFilters(ctx context.Context, filters []*dashboards.MultiSelect_Query_MetricsQuery_MetricsLabelFilter) (types.List, diag.Diagnostics) {
	var diagnostics diag.Diagnostics
	flattenedFilters := make([]attr.Value, 0, len(filters))
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

func flattenMultiSelectQueryMetricsQueryMetricsLabelFilter(ctx context.Context, filter *dashboards.MultiSelect_Query_MetricsQuery_MetricsLabelFilter) (*MetricLabelFilterModel, diag.Diagnostics) {
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

func flattenMultiSelectQueryMetricsQueryMetricsLabelFilterOperator(ctx context.Context, operator *dashboards.MultiSelect_Query_MetricsQuery_Operator) (types.Object, diag.Diagnostics) {
	if operator == nil {
		return types.ObjectNull(multiSelectQueryMetricsQueryMetricsLabelFilterOperatorAttr()), nil
	}

	var diags diag.Diagnostics
	metricLabelFilterOperatorModel := &MetricLabelFilterOperatorModel{}
	switch operatorType := operator.GetValue().(type) {
	case *dashboards.MultiSelect_Query_MetricsQuery_Operator_Equals:
		metricLabelFilterOperatorModel.Type = types.StringValue("equals")
		metricLabelFilterOperatorModel.SelectedValues, diags = flattenMultiSelectQueryMetricsQueryOperatorSelectedValues(ctx, operatorType.Equals.GetSelection().GetList().GetValues())
	case *dashboards.MultiSelect_Query_MetricsQuery_Operator_NotEquals:
		metricLabelFilterOperatorModel.Type = types.StringValue("not_equals")
		metricLabelFilterOperatorModel.SelectedValues, diags = flattenMultiSelectQueryMetricsQueryOperatorSelectedValues(ctx, operatorType.NotEquals.GetSelection().GetList().GetValues())
	}

	if diags.HasError() {
		return types.ObjectNull(multiSelectQueryMetricsQueryMetricsLabelFilterOperatorAttr()), diags
	}
	return types.ObjectValueFrom(ctx, multiSelectQueryMetricsQueryMetricsLabelFilterOperatorAttr(), metricLabelFilterOperatorModel)
}

func flattenMultiSelectQueryMetricsQueryOperatorSelectedValues(ctx context.Context, values []*dashboards.MultiSelect_Query_MetricsQuery_StringOrVariable) (types.List, diag.Diagnostics) {
	var diagnostics diag.Diagnostics
	flattenedValues := make([]types.Object, 0, len(values))
	for _, value := range values {
		flattenedValue, diags := flattenMultiSelectQueryMetricsQueryStringOrVariable(ctx, value)
		if diags.HasError() {
			diagnostics.Append(diags...)
			continue
		}
		valuesElement, diags := types.ObjectValueFrom(ctx, multiSelectQueryMetricsQueryMetricsLabelFilterOperatorAttr(), flattenedValue)
		if diags.HasError() {
			diagnostics.Append(diags...)
			continue
		}
		flattenedValues = append(flattenedValues, valuesElement)
	}

	if diagnostics.HasError() {
		return types.ListNull(types.ObjectType{AttrTypes: multiSelectQueryMetricsQueryMetricsLabelFilterOperatorAttr()}), diagnostics
	}

	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: multiSelectQueryMetricsQueryMetricsLabelFilterOperatorAttr()}, flattenedValues)
}

func flattenMultiSelectQueryMetricsQueryStringOrVariable(ctx context.Context, stringOrVariable *dashboards.MultiSelect_Query_MetricsQuery_StringOrVariable) (types.Object, diag.Diagnostics) {
	if stringOrVariable == nil {
		return types.ObjectNull(multiSelectQueryStringOrValueAttr()), nil
	}

	metricLabelFilterOperatorSelectedValuesModel := &MetricLabelFilterOperatorSelectedValuesModel{
		StringValue:  types.StringNull(),
		VariableName: types.StringNull(),
	}

	switch stringOrVariableType := stringOrVariable.GetValue().(type) {
	case *dashboards.MultiSelect_Query_MetricsQuery_StringOrVariable_StringValue:
		metricLabelFilterOperatorSelectedValuesModel.StringValue = wrapperspbStringToTypeString(stringOrVariableType.StringValue)
	case *dashboards.MultiSelect_Query_MetricsQuery_StringOrVariable_VariableName:
		metricLabelFilterOperatorSelectedValuesModel.VariableName = wrapperspbStringToTypeString(stringOrVariableType.VariableName)
	}

	return types.ObjectValueFrom(ctx, multiSelectQueryStringOrValueAttr(), metricLabelFilterOperatorSelectedValuesModel)
}

func flattenDashboardVariableDefinitionMultiSelectQuerySpansModel(ctx context.Context, query *dashboards.MultiSelect_Query_SpansQuery) (types.Object, diag.Diagnostics) {
	if query == nil {
		return types.ObjectNull(multiSelectQuerySpansQueryModelAttr()), nil
	}

	var diags diag.Diagnostics
	multiSelectSpansQueryModel := &MultiSelectSpansQueryModel{
		FieldName:  types.ObjectNull(spansQueryFieldNameAttr()),
		FieldValue: types.ObjectNull(spansFieldModelAttr()),
	}
	switch queryType := query.GetType().GetValue().(type) {
	case *dashboards.MultiSelect_Query_SpansQuery_Type_FieldName_:
		multiSelectSpansQueryModel.FieldName, diags = flattenMultiSelectQuerySpansFieldName(ctx, queryType.FieldName)
	case *dashboards.MultiSelect_Query_SpansQuery_Type_FieldValue_:
		multiSelectSpansQueryModel.FieldValue, diags = flattenMultiSelectQuerySpansFieldValue(ctx, queryType.FieldValue)
	default:
		return types.ObjectNull(multiSelectQuerySpansQueryModelAttr()), diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Dashboard Variable Definition Multi Select Query Spans Model", fmt.Sprintf("unknown variable definition multi select query spans type %T", queryType))}
	}

	if diags.HasError() {
		return types.ObjectNull(multiSelectQuerySpansQueryModelAttr()), diags
	}

	return types.ObjectValueFrom(ctx, multiSelectQuerySpansQueryModelAttr(), multiSelectSpansQueryModel)
}

func flattenMultiSelectQuerySpansFieldName(ctx context.Context, name *dashboards.MultiSelect_Query_SpansQuery_Type_FieldName) (types.Object, diag.Diagnostics) {
	if name == nil {
		return types.ObjectNull(multiSelectQuerySpansQueryModelAttr()), nil
	}

	return types.ObjectValueFrom(ctx, multiSelectQuerySpansQueryModelAttr(), &SpanFieldNameModel{
		SpanRegex: wrapperspbStringToTypeString(name.GetSpanRegex()),
	})
}

func flattenMultiSelectQuerySpansFieldValue(ctx context.Context, value *dashboards.MultiSelect_Query_SpansQuery_Type_FieldValue) (types.Object, diag.Diagnostics) {
	if value == nil || value.GetValue() == nil {
		return types.ObjectNull(spansFieldModelAttr()), nil
	}

	spanField, dg := flattenSpansField(value.GetValue())
	if dg != nil {
		return types.ObjectNull(spansFieldModelAttr()), diag.Diagnostics{dg}
	}

	return types.ObjectValueFrom(ctx, spansFieldModelAttr(), spanField)
}

func flattenDashboardVariableDefinitionMultiSelectValueDisplayOptions(ctx context.Context, options *dashboards.MultiSelect_ValueDisplayOptions) (types.Object, diag.Diagnostics) {
	if options == nil {
		return types.ObjectNull(multiSelectValueDisplayOptionsModelAttr()), nil
	}

	return types.ObjectValueFrom(ctx, multiSelectValueDisplayOptionsModelAttr(), &MultiSelectValueDisplayOptionsModel{
		ValueRegex: wrapperspbStringToTypeString(options.GetValueRegex()),
		LabelRegex: wrapperspbStringToTypeString(options.GetLabelRegex()),
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
		"observation_field": types.ObjectType{AttrTypes: observationFieldAttributes()},
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
		"field_value": types.ObjectType{AttrTypes: spansFieldModelAttr()},
	}
}

func spansQueryFieldNameAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"span_regex": types.StringType,
	}
}

func flattenDashboardVariableSelectedValues(selection *dashboards.MultiSelect_Selection) (types.List, diag.Diagnostics) {
	switch selection.GetValue().(type) {
	case *dashboards.MultiSelect_Selection_List:
		return wrappedStringSliceToTypeStringList(selection.GetList().GetValues()), nil
	case *dashboards.MultiSelect_Selection_All:
		return types.ListNull(types.StringType), nil
	default:
		return types.ListNull(types.StringType), diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Dashboard Variable Definition Multi Select Selection", fmt.Sprintf("unknown variable definition multi select selection type %T", selection))}
	}
}

func flattenDashboardFilters(ctx context.Context, filters []*dashboards.Filter) (types.List, diag.Diagnostics) {
	if len(filters) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: dashboardsFiltersModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	filtersElements := make([]attr.Value, 0, len(filters))
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

func flattenDashboardFilter(ctx context.Context, filter *dashboards.Filter) (*DashboardFilterModel, diag.Diagnostics) {
	if filter == nil {
		return nil, nil
	}

	source, diags := flattenDashboardFilterSource(ctx, filter.GetSource())
	if diags != nil {
		return nil, diags
	}

	return &DashboardFilterModel{
		Source:    source,
		Enabled:   wrapperspbBoolToTypeBool(filter.GetEnabled()),
		Collapsed: wrapperspbBoolToTypeBool(filter.GetCollapsed()),
	}, nil
}

func flattenDashboardFiltersSources(ctx context.Context, sources []*dashboards.Filter_Source) (types.List, diag.Diagnostics) {
	if len(sources) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: filterSourceModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	filtersElements := make([]attr.Value, 0, len(sources))
	for _, source := range sources {
		flattenedFilter, diags := flattenDashboardFilterSource(ctx, source)
		if diags.HasError() {
			diagnostics.Append(diags...)
			continue
		}
		filterElement, diags := types.ObjectValueFrom(ctx, filterSourceModelAttr(), flattenedFilter)
		if diags.HasError() {
			diagnostics.Append(diags...)
			continue
		}
		filtersElements = append(filtersElements, filterElement)
	}

	return types.ListValueMust(types.ObjectType{AttrTypes: filterSourceModelAttr()}, filtersElements), diagnostics
}

func flattenDashboardFilterSource(ctx context.Context, source *dashboards.Filter_Source) (*DashboardFilterSourceModel, diag.Diagnostics) {
	if source == nil {
		return nil, nil
	}

	switch source.GetValue().(type) {
	case *dashboards.Filter_Source_Logs:
		logs, diags := flattenDashboardFilterSourceLogs(ctx, source.GetLogs())
		if diags.HasError() {
			return nil, diags
		}
		return &DashboardFilterSourceModel{Logs: logs}, nil
	case *dashboards.Filter_Source_Spans:
		spans, dg := flattenDashboardFilterSourceSpans(source.GetSpans())
		if dg != nil {
			return nil, diag.Diagnostics{dg}
		}
		return &DashboardFilterSourceModel{Spans: spans}, nil
	case *dashboards.Filter_Source_Metrics:
		metrics, dg := flattenDashboardFilterSourceMetrics(source.GetMetrics())
		if dg != nil {
			return nil, diag.Diagnostics{dg}
		}
		return &DashboardFilterSourceModel{Metrics: metrics}, nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Dashboard Filter Source", fmt.Sprintf("unknown filter source type %T", source))}
	}
}

func flattenDashboardFilterSourceLogs(ctx context.Context, logs *dashboards.Filter_LogsFilter) (*FilterSourceLogsModel, diag.Diagnostics) {
	if logs == nil {
		return nil, nil
	}

	operator, dg := flattenFilterOperator(logs.GetOperator())
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	observationField, diags := flattenObservationField(ctx, logs.GetObservationField())
	if diags.HasError() {
		return nil, diags
	}

	return &FilterSourceLogsModel{
		Field:            wrapperspbStringToTypeString(logs.GetField()),
		Operator:         operator,
		ObservationField: observationField,
	}, nil
}

func flattenDashboardFilterSourceSpans(spans *dashboards.Filter_SpansFilter) (*FilterSourceSpansModel, diag.Diagnostic) {
	if spans == nil {
		return nil, nil
	}

	field, dg := flattenSpansField(spans.GetField())
	if dg != nil {
		return nil, dg
	}

	operator, dg := flattenFilterOperator(spans.GetOperator())
	if dg != nil {
		return nil, dg
	}

	return &FilterSourceSpansModel{
		Field:    field,
		Operator: operator,
	}, nil
}

func flattenDashboardFilterSourceMetrics(metrics *dashboards.Filter_MetricsFilter) (*FilterSourceMetricsModel, diag.Diagnostic) {
	if metrics == nil {
		return nil, nil
	}

	operator, dg := flattenFilterOperator(metrics.GetOperator())
	if dg != nil {
		return nil, dg
	}

	return &FilterSourceMetricsModel{
		MetricName:  wrapperspbStringToTypeString(metrics.GetMetric()),
		MetricLabel: wrapperspbStringToTypeString(metrics.GetLabel()),
		Operator:    operator,
	}, nil
}

func flattenDashboardTimeFrame(ctx context.Context, d *dashboards.Dashboard) (types.Object, diag.Diagnostics) {
	if d.GetTimeFrame() == nil {
		return types.ObjectNull(dashboardTimeFrameModelAttr()), nil
	}
	switch timeFrameType := d.GetTimeFrame().(type) {
	case *dashboards.Dashboard_AbsoluteTimeFrame:
		return flattenAbsoluteDashboardTimeFrame(ctx, timeFrameType.AbsoluteTimeFrame)
	case *dashboards.Dashboard_RelativeTimeFrame:
		return flattenRelativeDashboardTimeFrame(ctx, timeFrameType.RelativeTimeFrame)
	default:
		return types.ObjectNull(dashboardFolderModelAttr()), diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Dashboard Time Frame", fmt.Sprintf("unknown time frame type %T", timeFrameType))}
	}
}

func flattenAbsoluteDashboardTimeFrame(ctx context.Context, timeFrame *dashboards.TimeFrame) (types.Object, diag.Diagnostics) {
	absoluteTimeFrame := &DashboardTimeFrameAbsoluteModel{
		Start: types.StringValue(timeFrame.GetFrom().String()),
		End:   types.StringValue(timeFrame.GetTo().String()),
	}
	timeFrameObject, dgs := types.ObjectValueFrom(ctx, absoluteTimeFrameAttributes(), absoluteTimeFrame)
	if dgs.HasError() {
		return types.ObjectNull(dashboardTimeFrameModelAttr()), dgs
	}
	flattenedTimeFrame := &DashboardTimeFrameModel{
		Absolute: timeFrameObject,
		Relative: types.ObjectNull(relativeTimeFrameAttributes()),
	}
	return types.ObjectValueFrom(ctx, dashboardTimeFrameModelAttr(), flattenedTimeFrame)
}

func flattenDashboardFolder(ctx context.Context, planedDashboard types.Object, dashboard *dashboards.Dashboard) (types.Object, diag.Diagnostics) {
	if dashboard.GetFolder() == nil {
		return types.ObjectNull(dashboardFolderModelAttr()), nil
	}
	switch folderType := dashboard.GetFolder().(type) {
	case *dashboards.Dashboard_FolderId:
		path := types.StringNull()
		if !(planedDashboard.IsNull() || planedDashboard.IsUnknown()) {
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
			ID:   types.StringValue(folderType.FolderId.GetValue()),
			Path: path,
		}
		return types.ObjectValueFrom(ctx, dashboardFolderModelAttr(), folderObject)
	case *dashboards.Dashboard_FolderPath:
		folderObject := &DashboardFolderModel{
			ID:   types.StringNull(),
			Path: types.StringValue(strings.Join(folderType.FolderPath.GetSegments(), "/")),
		}
		return types.ObjectValueFrom(ctx, dashboardFolderModelAttr(), folderObject)
	default:
		return types.ObjectNull(dashboardFolderModelAttr()), diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Dashboard Folder", fmt.Sprintf("unknown folder type %T", dashboard.GetFolder()))}
	}
}

func flattenDashboardAnnotations(ctx context.Context, annotations []*dashboards.Annotation) (types.List, diag.Diagnostics) {
	if len(annotations) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: dashboardsAnnotationsModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	annotationsElements := make([]attr.Value, 0, len(annotations))
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

func flattenDashboardAnnotation(ctx context.Context, annotation *dashboards.Annotation) (*DashboardAnnotationModel, diag.Diagnostics) {
	if annotation == nil {
		return nil, nil
	}

	source, diags := flattenDashboardAnnotationSource(ctx, annotation.GetSource())
	if diags.HasError() {
		return nil, diags
	}

	return &DashboardAnnotationModel{
		ID:      wrapperspbStringToTypeString(annotation.GetId()),
		Name:    wrapperspbStringToTypeString(annotation.GetName()),
		Enabled: wrapperspbBoolToTypeBool(annotation.GetEnabled()),
		Source:  source,
	}, nil
}

func flattenDashboardAnnotationSource(ctx context.Context, source *dashboards.Annotation_Source) (types.Object, diag.Diagnostics) {
	if source == nil {
		return types.ObjectNull(dashboardsAnnotationsModelAttr()), nil
	}

	var sourceObject DashboardAnnotationSourceModel
	var diags diag.Diagnostics
	switch source.Value.(type) {
	case *dashboards.Annotation_Source_Metrics:
		sourceObject.Metrics, diags = flattenDashboardAnnotationMetricSourceModel(ctx, source.GetMetrics())
		sourceObject.Logs = types.ObjectNull(annotationsLogsAndSpansSourceModelAttr())
		sourceObject.Spans = types.ObjectNull(annotationsLogsAndSpansSourceModelAttr())
	case *dashboards.Annotation_Source_Logs:
		sourceObject.Logs, diags = flattenDashboardAnnotationLogsSourceModel(ctx, source.GetLogs())
		sourceObject.Metrics = types.ObjectNull(annotationsMetricsSourceModelAttr())
		sourceObject.Spans = types.ObjectNull(annotationsLogsAndSpansSourceModelAttr())
	case *dashboards.Annotation_Source_Spans:
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

func flattenDashboardAnnotationSpansSourceModel(ctx context.Context, spans *dashboards.Annotation_SpansSource) (types.Object, diag.Diagnostics) {
	if spans == nil {
		return types.ObjectNull(annotationsLogsAndSpansSourceModelAttr()), nil
	}

	strategy, diags := flattenAnnotationSpansStrategy(ctx, spans.GetStrategy())
	if diags.HasError() {
		return types.ObjectNull(annotationsLogsAndSpansSourceModelAttr()), diags
	}

	labelFields, diags := flattenObservationFields(ctx, spans.GetLabelFields())
	if diags.HasError() {
		return types.ObjectNull(annotationsLogsAndSpansSourceModelAttr()), diags
	}

	spansObject := &DashboardAnnotationSpansOrLogsSourceModel{
		LuceneQuery:     wrapperspbStringToTypeString(spans.GetLuceneQuery().GetValue()),
		Strategy:        strategy,
		MessageTemplate: wrapperspbStringToTypeString(spans.GetMessageTemplate()),
		LabelFields:     labelFields,
	}

	return types.ObjectValueFrom(ctx, annotationsLogsAndSpansSourceModelAttr(), spansObject)
}

func flattenAnnotationSpansStrategy(ctx context.Context, strategy *dashboards.Annotation_SpansSource_Strategy) (types.Object, diag.Diagnostics) {
	if strategy == nil {
		return types.ObjectNull(logsAndSpansStrategyModelAttr()), nil
	}

	var strategyModel DashboardAnnotationSpanOrLogsStrategyModel
	var diags diag.Diagnostics
	switch strategy.Value.(type) {
	case *dashboards.Annotation_SpansSource_Strategy_Instant_:
		strategyModel.Instant, diags = flattenSpansStrategyInstant(ctx, strategy.GetInstant())
		strategyModel.Range = types.ObjectNull(rangeStrategyModelAttr())
		strategyModel.Duration = types.ObjectNull(durationStrategyModelAttr())
	case *dashboards.Annotation_SpansSource_Strategy_Range_:
		strategyModel.Range, diags = flattenSpansStrategyRange(ctx, strategy.GetRange())
		strategyModel.Instant = types.ObjectNull(instantStrategyModelAttr())
		strategyModel.Duration = types.ObjectNull(durationStrategyModelAttr())
	case *dashboards.Annotation_SpansSource_Strategy_Duration_:
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

func flattenSpansStrategyDuration(ctx context.Context, duration *dashboards.Annotation_SpansSource_Strategy_Duration) (types.Object, diag.Diagnostics) {
	if duration == nil {
		return types.ObjectNull(durationStrategyModelAttr()), nil
	}

	startTimestampField, diags := flattenObservationField(ctx, duration.GetStartTimestampField())
	if diags.HasError() {
		return types.ObjectNull(durationStrategyModelAttr()), diags
	}

	endTimestampField, diags := flattenObservationField(ctx, duration.GetDurationField())
	if diags.HasError() {
		return types.ObjectNull(durationStrategyModelAttr()), diags
	}

	durationStrategy := &DashboardAnnotationDurationStrategyModel{
		StartTimestampField: startTimestampField,
		DurationField:       endTimestampField,
	}

	return types.ObjectValueFrom(ctx, durationStrategyModelAttr(), durationStrategy)
}

func flattenSpansStrategyRange(ctx context.Context, getRange *dashboards.Annotation_SpansSource_Strategy_Range) (types.Object, diag.Diagnostics) {
	if getRange == nil {
		return types.ObjectNull(rangeStrategyModelAttr()), nil
	}

	startTimestampField, diags := flattenObservationField(ctx, getRange.GetStartTimestampField())
	if diags.HasError() {
		return types.ObjectNull(rangeStrategyModelAttr()), diags
	}

	endTimestampField, diags := flattenObservationField(ctx, getRange.GetEndTimestampField())
	if diags.HasError() {
		return types.ObjectNull(rangeStrategyModelAttr()), diags
	}

	rangeStrategy := &DashboardAnnotationRangeStrategyModel{
		StartTimestampField: startTimestampField,
		EndTimestampField:   endTimestampField,
	}

	return types.ObjectValueFrom(ctx, rangeStrategyModelAttr(), rangeStrategy)
}

func flattenSpansStrategyInstant(ctx context.Context, instant *dashboards.Annotation_SpansSource_Strategy_Instant) (types.Object, diag.Diagnostics) {
	if instant == nil {
		return types.ObjectNull(instantStrategyModelAttr()), nil
	}

	timestampField, diags := flattenObservationField(ctx, instant.GetTimestampField())
	if diags.HasError() {
		return types.ObjectNull(instantStrategyModelAttr()), diags
	}

	instantStrategy := &DashboardAnnotationInstantStrategyModel{
		TimestampField: timestampField,
	}

	return types.ObjectValueFrom(ctx, instantStrategyModelAttr(), instantStrategy)
}

func flattenLogsStrategyDuration(ctx context.Context, duration *dashboards.Annotation_LogsSource_Strategy_Duration) (types.Object, diag.Diagnostics) {
	if duration == nil {
		return types.ObjectNull(durationStrategyModelAttr()), nil
	}

	startTimestampField, diags := flattenObservationField(ctx, duration.GetStartTimestampField())
	if diags.HasError() {
		return types.ObjectNull(durationStrategyModelAttr()), diags
	}

	endTimestampField, diags := flattenObservationField(ctx, duration.GetDurationField())
	if diags.HasError() {
		return types.ObjectNull(durationStrategyModelAttr()), diags
	}

	durationStrategy := &DashboardAnnotationDurationStrategyModel{
		StartTimestampField: startTimestampField,
		DurationField:       endTimestampField,
	}

	return types.ObjectValueFrom(ctx, durationStrategyModelAttr(), durationStrategy)
}

func flattenLogsStrategyRange(ctx context.Context, getRange *dashboards.Annotation_LogsSource_Strategy_Range) (types.Object, diag.Diagnostics) {
	if getRange == nil {
		return types.ObjectNull(rangeStrategyModelAttr()), nil
	}

	startTimestampField, diags := flattenObservationField(ctx, getRange.GetStartTimestampField())
	if diags.HasError() {
		return types.ObjectNull(rangeStrategyModelAttr()), diags
	}

	endTimestampField, diags := flattenObservationField(ctx, getRange.GetEndTimestampField())
	if diags.HasError() {
		return types.ObjectNull(rangeStrategyModelAttr()), diags
	}

	rangeStrategy := &DashboardAnnotationRangeStrategyModel{
		StartTimestampField: startTimestampField,
		EndTimestampField:   endTimestampField,
	}

	return types.ObjectValueFrom(ctx, rangeStrategyModelAttr(), rangeStrategy)
}

func flattenLogsStrategyInstant(ctx context.Context, instant *dashboards.Annotation_LogsSource_Strategy_Instant) (types.Object, diag.Diagnostics) {
	if instant == nil {
		return types.ObjectNull(instantStrategyModelAttr()), nil
	}

	timestampField, diags := flattenObservationField(ctx, instant.GetTimestampField())
	if diags.HasError() {
		return types.ObjectNull(instantStrategyModelAttr()), diags
	}

	instantStrategy := &DashboardAnnotationInstantStrategyModel{
		TimestampField: timestampField,
	}

	return types.ObjectValueFrom(ctx, instantStrategyModelAttr(), instantStrategy)
}

func flattenDashboardAnnotationLogsSourceModel(ctx context.Context, logs *dashboards.Annotation_LogsSource) (types.Object, diag.Diagnostics) {
	if logs == nil {
		return types.ObjectNull(annotationsLogsAndSpansSourceModelAttr()), nil
	}

	strategy, diags := flattenAnnotationLogsStrategy(ctx, logs.GetStrategy())
	if diags.HasError() {
		return types.ObjectNull(annotationsLogsAndSpansSourceModelAttr()), diags
	}

	labelFields, diags := flattenObservationFields(ctx, logs.GetLabelFields())
	if diags.HasError() {
		return types.ObjectNull(annotationsLogsAndSpansSourceModelAttr()), diags
	}

	logsObject := &DashboardAnnotationSpansOrLogsSourceModel{
		LuceneQuery:     wrapperspbStringToTypeString(logs.GetLuceneQuery().GetValue()),
		Strategy:        strategy,
		MessageTemplate: wrapperspbStringToTypeString(logs.GetMessageTemplate()),
		LabelFields:     labelFields,
	}

	return types.ObjectValueFrom(ctx, annotationsLogsAndSpansSourceModelAttr(), logsObject)
}

func flattenAnnotationLogsStrategy(ctx context.Context, strategy *dashboards.Annotation_LogsSource_Strategy) (types.Object, diag.Diagnostics) {
	if strategy == nil {
		return types.ObjectNull(logsAndSpansStrategyModelAttr()), nil
	}

	var strategyModel DashboardAnnotationSpanOrLogsStrategyModel
	var diags diag.Diagnostics
	switch strategy.Value.(type) {
	case *dashboards.Annotation_LogsSource_Strategy_Instant_:
		strategyModel.Instant, diags = flattenLogsStrategyInstant(ctx, strategy.GetInstant())
		strategyModel.Range = types.ObjectNull(rangeStrategyModelAttr())
		strategyModel.Duration = types.ObjectNull(durationStrategyModelAttr())
	case *dashboards.Annotation_LogsSource_Strategy_Range_:
		strategyModel.Range, diags = flattenLogsStrategyRange(ctx, strategy.GetRange())
		strategyModel.Instant = types.ObjectNull(instantStrategyModelAttr())
		strategyModel.Duration = types.ObjectNull(durationStrategyModelAttr())
	case *dashboards.Annotation_LogsSource_Strategy_Duration_:
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

func flattenDashboardAnnotationMetricSourceModel(ctx context.Context, metricSource *dashboards.Annotation_MetricsSource) (types.Object, diag.Diagnostics) {
	if metricSource == nil {
		return types.ObjectNull(annotationsMetricsSourceModelAttr()), nil
	}

	strategy, diags := flattenDashboardAnnotationStrategy(ctx, metricSource.GetStrategy())
	if diags.HasError() {
		return types.ObjectNull(annotationsMetricsSourceModelAttr()), diags
	}

	metricSourceObject := &DashboardAnnotationMetricSourceModel{
		PromqlQuery:     wrapperspbStringToTypeString(metricSource.GetPromqlQuery().GetValue()),
		Strategy:        strategy,
		MessageTemplate: wrapperspbStringToTypeString(metricSource.GetMessageTemplate()),
		Labels:          wrappedStringSliceToTypeStringList(metricSource.GetLabels()),
	}

	return types.ObjectValueFrom(ctx, annotationsMetricsSourceModelAttr(), metricSourceObject)
}

func flattenDashboardAnnotationStrategy(ctx context.Context, strategy *dashboards.Annotation_MetricsSource_Strategy) (types.Object, diag.Diagnostics) {
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

func dashboardTimeFrameModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"absolute": types.ObjectType{AttrTypes: absoluteTimeFrameAttributes()},
		"relative": types.ObjectType{AttrTypes: relativeTimeFrameAttributes()},
	}
}

func absoluteTimeFrameAttributes() map[string]attr.Type {
	return map[string]attr.Type{
		"start": types.StringType,
		"end":   types.StringType,
	}
}

func flattenRelativeDashboardTimeFrame(ctx context.Context, timeFrame *durationpb.Duration) (types.Object, diag.Diagnostics) {
	relativeTimeFrame := &DashboardTimeFrameRelativeModel{
		Duration: flattenDuration(timeFrame),
	}
	timeFrameObject, dgs := types.ObjectValueFrom(ctx, relativeTimeFrameAttributes(), relativeTimeFrame)
	if dgs.HasError() {
		return types.ObjectNull(dashboardTimeFrameModelAttr()), dgs
	}
	flattenedTimeFrame := &DashboardTimeFrameModel{
		Relative: timeFrameObject,
		Absolute: types.ObjectNull(absoluteTimeFrameAttributes()),
	}
	return types.ObjectValueFrom(ctx, dashboardTimeFrameModelAttr(), flattenedTimeFrame)
}

func flattenDuration(timeFrame *durationpb.Duration) basetypes.StringValue {
	if timeFrame == nil {
		return types.StringNull()
	}
	if timeFrame.Seconds == 0 && timeFrame.Nanos == 0 {
		return types.StringValue("seconds:0")
	}
	return types.StringValue(timeFrame.String())
}

func flattenDashboardAutoRefresh(ctx context.Context, dashboard *dashboards.Dashboard) (types.Object, diag.Diagnostics) {
	autoRefresh := dashboard.GetAutoRefresh()
	if autoRefresh == nil {
		return types.ObjectNull(dashboardAutoRefreshModelAttr()), nil
	}

	var refreshType DashboardAutoRefreshModel
	switch autoRefresh.(type) {
	case *dashboards.Dashboard_Off:
		refreshType.Type = types.StringValue("off")
	case *dashboards.Dashboard_FiveMinutes:
		refreshType.Type = types.StringValue("five_minutes")
	case *dashboards.Dashboard_TwoMinutes:
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
	getDashboardReq := &dashboards.GetDashboardRequest{DashboardId: wrapperspb.String(id)}
	getDashboardResp, err := r.client.GetDashboard(ctx, getDashboardReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if status.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("Dashboard %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError(
				"Error reading Dashboard",
				formatRpcErrors(err, getDashboardURL, protojson.Format(getDashboardReq)),
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

	updateReq := &dashboards.ReplaceDashboardRequest{Dashboard: dashboard}
	reqStr := protojson.Format(updateReq)
	log.Printf("[INFO] Updating Dashboard: %s", reqStr)
	_, err := r.client.UpdateDashboard(ctx, updateReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error updating Dashboard",
			formatRpcErrors(err, updateDashboardURL, reqStr),
		)
		return
	}

	getDashboardReq := &dashboards.GetDashboardRequest{
		DashboardId: dashboard.GetId(),
	}
	getDashboardResp, err := r.client.GetDashboard(ctx, getDashboardReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error getting Dashboard",
			formatRpcErrors(err, getDashboardURL, protojson.Format(getDashboardReq)),
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

	// Set state to fully populated data
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
	deleteReq := &dashboards.DeleteDashboardRequest{DashboardId: wrapperspb.String(id)}
	if _, err := r.client.DeleteDashboard(ctx, deleteReq); err != nil {
		resp.Diagnostics.AddError(
			fmt.Sprintf("Error Deleting Dashboard %s", id),
			formatRpcErrors(err, deleteDashboardURL, protojson.Format(deleteReq)),
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

func dashboardV1() schema.Schema {
	attributes := dashboardSchemaAttributes()
	delete(attributes, "auto_refresh")
	attributes["annotations"] = schema.ListNestedAttribute{
		Optional: true,
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"id": schema.StringAttribute{
					Optional: true,
					Computed: true,
					PlanModifiers: []planmodifier.String{
						stringplanmodifier.UseStateForUnknown(),
					},
				},
				"name": schema.StringAttribute{
					Required: true,
				},
				"enabled": schema.BoolAttribute{
					Optional: true,
					Computed: true,
					Default:  booldefault.StaticBool(true),
				},
				"source": schema.SingleNestedAttribute{
					Attributes: map[string]schema.Attribute{
						"metric": schema.SingleNestedAttribute{
							Attributes: map[string]schema.Attribute{
								"promql_query": schema.StringAttribute{
									Required: true,
								},
								"strategy": schema.SingleNestedAttribute{
									Attributes: map[string]schema.Attribute{
										"start_time": schema.SingleNestedAttribute{
											Attributes: map[string]schema.Attribute{},
											Required:   true,
										},
									},
									Required: true,
								},
								"message_template": schema.StringAttribute{
									Optional: true,
								},
								"labels": schema.ListAttribute{
									ElementType: types.StringType,
									Optional:    true,
								},
							},
							Optional: true,
							Validators: []validator.Object{
								objectvalidator.ExactlyOneOf(
									path.MatchRelative().AtParent().AtName("logs"),
									path.MatchRelative().AtParent().AtName("spans"),
								),
							},
						},
					},
					Required: true,
				},
			},
		},
	}
	return schema.Schema{
		Version:    1,
		Attributes: attributes,
	}
}
