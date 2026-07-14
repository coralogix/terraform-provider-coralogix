// Copyright 2025 Coralogix Ltd.
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
	"context"
	"fmt"
	"math/big"
	"slices"
	"time"

	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	dashboardservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_service"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

var (
	DashboardSchemaToProtoUnit = map[string]dashboardservice.CommonUnit{
		utils.UNSPECIFIED: dashboardservice.COMMONUNIT_UNIT_UNSPECIFIED,
		"microseconds":    dashboardservice.COMMONUNIT_UNIT_MICROSECONDS,
		"milliseconds":    dashboardservice.COMMONUNIT_UNIT_MILLISECONDS,
		"nanoseconds":     dashboardservice.COMMONUNIT_UNIT_NANOSECONDS,
		"seconds":         dashboardservice.COMMONUNIT_UNIT_SECONDS,
		"bytes":           dashboardservice.COMMONUNIT_UNIT_BYTES,
		"kbytes":          dashboardservice.COMMONUNIT_UNIT_KBYTES,
		"mbytes":          dashboardservice.COMMONUNIT_UNIT_MBYTES,
		"gbytes":          dashboardservice.COMMONUNIT_UNIT_GBYTES,
		"bytes_iec":       dashboardservice.COMMONUNIT_UNIT_BYTES_IEC,
		"kibytes":         dashboardservice.COMMONUNIT_UNIT_KIBYTES,
		"mibytes":         dashboardservice.COMMONUNIT_UNIT_MIBYTES,
		"gibytes":         dashboardservice.COMMONUNIT_UNIT_GIBYTES,
		"euro_cents":      dashboardservice.COMMONUNIT_UNIT_EUR_CENTS,
		"euro":            dashboardservice.COMMONUNIT_UNIT_EUR,
		"usd_cents":       dashboardservice.COMMONUNIT_UNIT_USD_CENTS,
		"usd":             dashboardservice.COMMONUNIT_UNIT_USD,
		"custom":          dashboardservice.COMMONUNIT_UNIT_CUSTOM,
		"percent01":       dashboardservice.COMMONUNIT_UNIT_PERCENT_ZERO_ONE,
		"percent100":      dashboardservice.COMMONUNIT_UNIT_PERCENT_ZERO_HUNDRED,
	}
	DashboardProtoToSchemaUnit = utils.ReverseMap(DashboardSchemaToProtoUnit)
	DashboardValidUnits        = utils.GetKeys(DashboardSchemaToProtoUnit)

	DashboardLegendPlacementSchemaToProto = map[string]dashboardservice.LegendPlacement{
		utils.UNSPECIFIED: dashboardservice.LEGENDPLACEMENT_LEGEND_PLACEMENT_UNSPECIFIED,
		"auto":            dashboardservice.LEGENDPLACEMENT_LEGEND_PLACEMENT_AUTO,
		"bottom":          dashboardservice.LEGENDPLACEMENT_LEGEND_PLACEMENT_BOTTOM,
		"side":            dashboardservice.LEGENDPLACEMENT_LEGEND_PLACEMENT_SIDE,
		"hidden":          dashboardservice.LEGENDPLACEMENT_LEGEND_PLACEMENT_HIDDEN,
	}
	DashboardLegendPlacementProtoToSchema = utils.ReverseMap(DashboardLegendPlacementSchemaToProto)
	DashboardValidLegendPlacements        = utils.GetKeys(DashboardLegendPlacementSchemaToProto)

	DashboardRowStyleSchemaToProto = map[string]dashboardservice.RowStyle{
		utils.UNSPECIFIED: dashboardservice.ROWSTYLE_ROW_STYLE_UNSPECIFIED,
		"one_line":        dashboardservice.ROWSTYLE_ROW_STYLE_ONE_LINE,
		"two_line":        dashboardservice.ROWSTYLE_ROW_STYLE_TWO_LINE,
		"condensed":       dashboardservice.ROWSTYLE_ROW_STYLE_CONDENSED,
		"json":            dashboardservice.ROWSTYLE_ROW_STYLE_JSON,
		"list":            dashboardservice.ROWSTYLE_ROW_STYLE_LIST,
	}
	DashboardRowStyleProtoToSchema     = utils.ReverseMap(DashboardRowStyleSchemaToProto)
	DashboardValidRowStyles            = utils.GetKeys(DashboardRowStyleSchemaToProto)
	DashboardLegendColumnSchemaToProto = map[string]dashboardservice.LegendColumn{
		utils.UNSPECIFIED: dashboardservice.LEGENDCOLUMN_LEGEND_COLUMN_UNSPECIFIED,
		"min":             dashboardservice.LEGENDCOLUMN_LEGEND_COLUMN_MIN,
		"max":             dashboardservice.LEGENDCOLUMN_LEGEND_COLUMN_MAX,
		"sum":             dashboardservice.LEGENDCOLUMN_LEGEND_COLUMN_SUM,
		"avg":             dashboardservice.LEGENDCOLUMN_LEGEND_COLUMN_AVG,
		"last":            dashboardservice.LEGENDCOLUMN_LEGEND_COLUMN_LAST,
		"name":            dashboardservice.LEGENDCOLUMN_LEGEND_COLUMN_NAME,
	}
	DashboardLegendColumnProtoToSchema   = utils.ReverseMap(DashboardLegendColumnSchemaToProto)
	DashboardValidLegendColumns          = utils.GetKeys(DashboardLegendColumnSchemaToProto)
	DashboardOrderDirectionSchemaToProto = map[string]dashboardservice.OrderDirection{
		utils.UNSPECIFIED: dashboardservice.ORDERDIRECTION_ORDER_DIRECTION_UNSPECIFIED,
		"asc":             dashboardservice.ORDERDIRECTION_ORDER_DIRECTION_ASC,
		"desc":            dashboardservice.ORDERDIRECTION_ORDER_DIRECTION_DESC,
	}
	DashboardOrderDirectionProtoToSchema = utils.ReverseMap(DashboardOrderDirectionSchemaToProto)
	DashboardValidOrderDirections        = utils.GetKeys(DashboardOrderDirectionSchemaToProto)

	DashboardValidMultiSelectSelectionTypes = []string{
		"multi",
		"single",
	}
	DashboardSchemaToProtoTooltipType = map[string]dashboardservice.TooltipType{
		utils.UNSPECIFIED: dashboardservice.TOOLTIPTYPE_TOOLTIP_TYPE_UNSPECIFIED,
		"all":             dashboardservice.TOOLTIPTYPE_TOOLTIP_TYPE_ALL,
		"single":          dashboardservice.TOOLTIPTYPE_TOOLTIP_TYPE_SINGLE,
	}
	DashboardProtoToSchemaTooltipType = utils.ReverseMap(DashboardSchemaToProtoTooltipType)
	DashboardValidTooltipTypes        = utils.GetKeys(DashboardSchemaToProtoTooltipType)
	DashboardSchemaToProtoScaleType   = map[string]dashboardservice.ScaleType{
		utils.UNSPECIFIED: dashboardservice.SCALETYPE_SCALE_TYPE_UNSPECIFIED,
		"linear":          dashboardservice.SCALETYPE_SCALE_TYPE_LINEAR,
		"logarithmic":     dashboardservice.SCALETYPE_SCALE_TYPE_LOGARITHMIC,
	}
	DashboardProtoToSchemaScaleType = utils.ReverseMap(DashboardSchemaToProtoScaleType)
	DashboardValidScaleTypes        = utils.GetKeys(DashboardSchemaToProtoScaleType)

	DashboardSchemaToProtoGaugeUnit = map[string]dashboardservice.GaugeUnit{
		utils.UNSPECIFIED: dashboardservice.GAUGEUNIT_UNIT_UNSPECIFIED,
		"none":            dashboardservice.GAUGEUNIT_UNIT_NUMBER,
		"percent":         dashboardservice.GAUGEUNIT_UNIT_PERCENT,
		"microseconds":    dashboardservice.GAUGEUNIT_UNIT_MICROSECONDS,
		"milliseconds":    dashboardservice.GAUGEUNIT_UNIT_MILLISECONDS,
		"nanoseconds":     dashboardservice.GAUGEUNIT_UNIT_NANOSECONDS,
		"seconds":         dashboardservice.GAUGEUNIT_UNIT_SECONDS,
		"bytes":           dashboardservice.GAUGEUNIT_UNIT_BYTES,
		"kbytes":          dashboardservice.GAUGEUNIT_UNIT_KBYTES,
		"mbytes":          dashboardservice.GAUGEUNIT_UNIT_MBYTES,
		"gbytes":          dashboardservice.GAUGEUNIT_UNIT_GBYTES,
		"bytes_iec":       dashboardservice.GAUGEUNIT_UNIT_BYTES_IEC,
		"kibytes":         dashboardservice.GAUGEUNIT_UNIT_KIBYTES,
		"mibytes":         dashboardservice.GAUGEUNIT_UNIT_MIBYTES,
		"gibytes":         dashboardservice.GAUGEUNIT_UNIT_GIBYTES,
		"euro_cents":      dashboardservice.GAUGEUNIT_UNIT_EUR_CENTS,
		"euro":            dashboardservice.GAUGEUNIT_UNIT_EUR,
		"usd_cents":       dashboardservice.GAUGEUNIT_UNIT_USD_CENTS,
		"usd":             dashboardservice.GAUGEUNIT_UNIT_USD,
		"custom":          dashboardservice.GAUGEUNIT_UNIT_CUSTOM,
		"percent01":       dashboardservice.GAUGEUNIT_UNIT_PERCENT_ZERO_ONE,
		"percent100":      dashboardservice.GAUGEUNIT_UNIT_PERCENT_ZERO_HUNDRED,
	}
	DashboardProtoToSchemaGaugeUnit           = utils.ReverseMap(DashboardSchemaToProtoGaugeUnit)
	DashboardValidGaugeUnits                  = utils.GetKeys(DashboardSchemaToProtoGaugeUnit)
	DashboardSchemaToProtoPieChartLabelSource = map[string]dashboardservice.WidgetsPieChartLabelSource{
		utils.UNSPECIFIED: dashboardservice.WIDGETSPIECHARTLABELSOURCE_LABEL_SOURCE_UNSPECIFIED,
		"inner":           dashboardservice.WIDGETSPIECHARTLABELSOURCE_LABEL_SOURCE_INNER,
		"stack":           dashboardservice.WIDGETSPIECHARTLABELSOURCE_LABEL_SOURCE_STACK,
	}
	DashboardProtoToSchemaPieChartLabelSource = utils.ReverseMap(DashboardSchemaToProtoPieChartLabelSource)
	DashboardValidPieChartLabelSources        = utils.GetKeys(DashboardSchemaToProtoPieChartLabelSource)
	DashboardSchemaToProtoGaugeAggregation    = map[string]dashboardservice.GaugeAggregation{
		utils.UNSPECIFIED: dashboardservice.GAUGEAGGREGATION_AGGREGATION_UNSPECIFIED,
		"last":            dashboardservice.GAUGEAGGREGATION_AGGREGATION_LAST,
		"min":             dashboardservice.GAUGEAGGREGATION_AGGREGATION_MIN,
		"max":             dashboardservice.GAUGEAGGREGATION_AGGREGATION_MAX,
		"avg":             dashboardservice.GAUGEAGGREGATION_AGGREGATION_AVG,
		"sum":             dashboardservice.GAUGEAGGREGATION_AGGREGATION_SUM,
	}
	DashboardProtoToSchemaGaugeAggregation            = utils.ReverseMap(DashboardSchemaToProtoGaugeAggregation)
	DashboardValidGaugeAggregations                   = utils.GetKeys(DashboardSchemaToProtoGaugeAggregation)
	DashboardSchemaToProtoSpansAggregationMetricField = map[string]dashboardservice.MetricAggregationMetricField{
		utils.UNSPECIFIED: dashboardservice.METRICAGGREGATIONMETRICFIELD_METRIC_FIELD_UNSPECIFIED,
		"duration":        dashboardservice.METRICAGGREGATIONMETRICFIELD_METRIC_FIELD_DURATION,
	}
	DashboardProtoToSchemaSpansAggregationMetricField           = utils.ReverseMap(DashboardSchemaToProtoSpansAggregationMetricField)
	DashboardValidSpansAggregationMetricFields                  = utils.GetKeys(DashboardSchemaToProtoSpansAggregationMetricField)
	DashboardSchemaToProtoSpansAggregationMetricAggregationType = map[string]dashboardservice.MetricAggregationType{
		utils.UNSPECIFIED: dashboardservice.METRICAGGREGATIONTYPE_METRIC_AGGREGATION_TYPE_UNSPECIFIED,
		"min":             dashboardservice.METRICAGGREGATIONTYPE_METRIC_AGGREGATION_TYPE_MIN,
		"max":             dashboardservice.METRICAGGREGATIONTYPE_METRIC_AGGREGATION_TYPE_MAX,
		"avg":             dashboardservice.METRICAGGREGATIONTYPE_METRIC_AGGREGATION_TYPE_AVERAGE,
		"sum":             dashboardservice.METRICAGGREGATIONTYPE_METRIC_AGGREGATION_TYPE_SUM,
		"percentile_99":   dashboardservice.METRICAGGREGATIONTYPE_METRIC_AGGREGATION_TYPE_PERCENTILE_99,
		"percentile_95":   dashboardservice.METRICAGGREGATIONTYPE_METRIC_AGGREGATION_TYPE_PERCENTILE_95,
		"percentile_50":   dashboardservice.METRICAGGREGATIONTYPE_METRIC_AGGREGATION_TYPE_PERCENTILE_50,
	}
	DashboardProtoToSchemaSpansAggregationMetricAggregationType = utils.ReverseMap(DashboardSchemaToProtoSpansAggregationMetricAggregationType)
	DashboardValidSpansAggregationMetricAggregationTypes        = utils.GetKeys(DashboardSchemaToProtoSpansAggregationMetricAggregationType)
	DashboardProtoToSchemaSpansAggregationDimensionField        = map[string]dashboardservice.DimensionField{
		utils.UNSPECIFIED: dashboardservice.DIMENSIONFIELD_DIMENSION_FIELD_UNSPECIFIED,
		"trace_id":        dashboardservice.DIMENSIONFIELD_DIMENSION_FIELD_TRACE_ID,
	}
	DashboardSchemaToProtoSpansAggregationDimensionField           = utils.ReverseMap(DashboardProtoToSchemaSpansAggregationDimensionField)
	DashboardValidSpansAggregationDimensionFields                  = utils.GetKeys(DashboardProtoToSchemaSpansAggregationDimensionField)
	DashboardSchemaToProtoSpansAggregationDimensionAggregationType = map[string]dashboardservice.DimensionAggregationType{
		utils.UNSPECIFIED: dashboardservice.DIMENSIONAGGREGATIONTYPE_DIMENSION_AGGREGATION_TYPE_UNSPECIFIED,
		"unique_count":    dashboardservice.DIMENSIONAGGREGATIONTYPE_DIMENSION_AGGREGATION_TYPE_UNIQUE_COUNT,
		"error_count":     dashboardservice.DIMENSIONAGGREGATIONTYPE_DIMENSION_AGGREGATION_TYPE_ERROR_COUNT,
	}
	DashboardProtoToSchemaSpansAggregationDimensionAggregationType = utils.ReverseMap(DashboardSchemaToProtoSpansAggregationDimensionAggregationType)
	DashboardValidSpansAggregationDimensionAggregationTypes        = utils.GetKeys(DashboardSchemaToProtoSpansAggregationDimensionAggregationType)
	DashboardSchemaToProtoSpanFieldMetadataField                   = map[string]dashboardservice.MetadataField{
		utils.UNSPECIFIED:  dashboardservice.METADATAFIELD_METADATA_FIELD_UNSPECIFIED,
		"application_name": dashboardservice.METADATAFIELD_METADATA_FIELD_APPLICATION_NAME,
		"subsystem_name":   dashboardservice.METADATAFIELD_METADATA_FIELD_SUBSYSTEM_NAME,
		"service_name":     dashboardservice.METADATAFIELD_METADATA_FIELD_SERVICE_NAME,
		"operation_name":   dashboardservice.METADATAFIELD_METADATA_FIELD_OPERATION_NAME,
	}
	DashboardProtoToSchemaSpanFieldMetadataField = utils.ReverseMap(DashboardSchemaToProtoSpanFieldMetadataField)
	DashboardValidSpanFieldMetadataFields        = utils.GetKeys(DashboardSchemaToProtoSpanFieldMetadataField)
	DashboardSchemaToProtoSortBy                 = map[string]dashboardservice.SortByType{
		utils.UNSPECIFIED: dashboardservice.SORTBYTYPE_SORT_BY_TYPE_UNSPECIFIED,
		"value":           dashboardservice.SORTBYTYPE_SORT_BY_TYPE_VALUE,
		"name":            dashboardservice.SORTBYTYPE_SORT_BY_TYPE_NAME,
	}
	DashboardProtoToSchemaSortBy                = utils.ReverseMap(DashboardSchemaToProtoSortBy)
	DashboardValidSortBy                        = utils.GetKeys(DashboardSchemaToProtoSortBy)
	DashboardSchemaToProtoObservationFieldScope = map[string]dashboardservice.DatasetScope{
		utils.UNSPECIFIED: dashboardservice.DATASETSCOPE_DATASET_SCOPE_UNSPECIFIED,
		"user_data":       dashboardservice.DATASETSCOPE_DATASET_SCOPE_USER_DATA,
		"label":           dashboardservice.DATASETSCOPE_DATASET_SCOPE_LABEL,
		"metadata":        dashboardservice.DATASETSCOPE_DATASET_SCOPE_METADATA,
	}
	DashboardProtoToSchemaObservationFieldScope = utils.ReverseMap(DashboardSchemaToProtoObservationFieldScope)
	DashboardValidObservationFieldScope         = utils.GetKeys(DashboardSchemaToProtoObservationFieldScope)
	DashboardSchemaToProtoDataModeType          = map[string]dashboardservice.WidgetsCommonDataModeType{
		utils.UNSPECIFIED: dashboardservice.WIDGETSCOMMONDATAMODETYPE_DATA_MODE_TYPE_HIGH_UNSPECIFIED,
		"archive":         dashboardservice.WIDGETSCOMMONDATAMODETYPE_DATA_MODE_TYPE_ARCHIVE,
	}
	DashboardProtoToSchemaDataModeType     = utils.ReverseMap(DashboardSchemaToProtoDataModeType)
	DashboardValidDataModeTypes            = utils.GetKeys(DashboardSchemaToProtoDataModeType)
	DashboardSchemaToProtoGaugeThresholdBy = map[string]dashboardservice.GaugeThresholdBy{
		utils.UNSPECIFIED: dashboardservice.GAUGETHRESHOLDBY_THRESHOLD_BY_UNSPECIFIED,
		"value":           dashboardservice.GAUGETHRESHOLDBY_THRESHOLD_BY_VALUE,
		"background":      dashboardservice.GAUGETHRESHOLDBY_THRESHOLD_BY_BACKGROUND,
	}
	DashboardProtoToSchemaGaugeThresholdBy = utils.ReverseMap(DashboardSchemaToProtoGaugeThresholdBy)
	DashboardValidGaugeThresholdBy         = utils.GetKeys(DashboardSchemaToProtoGaugeThresholdBy)
	DashboardSchemaToProtoRefreshStrategy  = map[string]dashboardservice.MultiSelectRefreshStrategy{
		utils.UNSPECIFIED:      dashboardservice.MULTISELECTREFRESHSTRATEGY_REFRESH_STRATEGY_UNSPECIFIED,
		"on_dashboard_load":    dashboardservice.MULTISELECTREFRESHSTRATEGY_REFRESH_STRATEGY_ON_DASHBOARD_LOAD,
		"on_time_frame_change": dashboardservice.MULTISELECTREFRESHSTRATEGY_REFRESH_STRATEGY_ON_TIME_FRAME_CHANGE,
	}
	DashboardProtoToSchemaRefreshStrategy = utils.ReverseMap(DashboardSchemaToProtoRefreshStrategy)
	DashboardValidRefreshStrategies       = utils.GetKeys(DashboardSchemaToProtoRefreshStrategy)
	DashboardValidLogsAggregationTypes    = []string{"count", "count_distinct", "sum", "avg", "min", "max", "percentile"}
	DashboardValidSpanFieldTypes          = []string{"metadata", "tag", "process_tag"}
	DashboardValidSpanAggregationTypes    = []string{"metric", "dimension"}
	DashboardValidColorSchemes            = []string{"classic", "severity", "cold", "negative", "green", "red", "blue"}
	SectionValidColors                    = []string{"cyan", "green", "blue", "purple", "magenta", "pink", "orange"}

	DashboardSchemaToProtoThresholdType = map[string]dashboardservice.ThresholdType{
		utils.UNSPECIFIED: dashboardservice.THRESHOLDTYPE_THRESHOLD_TYPE_UNSPECIFIED,
		"absolute":        dashboardservice.THRESHOLDTYPE_THRESHOLD_TYPE_ABSOLUTE,
		"relative":        dashboardservice.THRESHOLDTYPE_THRESHOLD_TYPE_RELATIVE,
	}
	DashboardProtoToSchemaThresholdType = utils.ReverseMap(DashboardSchemaToProtoThresholdType)
	DashboardValidThresholdTypes        = utils.GetKeys(DashboardSchemaToProtoThresholdType)
	DashboardSchemaToProtoLegendBy      = map[string]dashboardservice.LegendBy{
		utils.UNSPECIFIED: dashboardservice.LEGENDBY_LEGEND_BY_UNSPECIFIED,
		"thresholds":      dashboardservice.LEGENDBY_LEGEND_BY_THRESHOLDS,
		"groups":          dashboardservice.LEGENDBY_LEGEND_BY_GROUPS,
	}
	DashboardProtoToSchemaLegendBy = utils.ReverseMap(DashboardSchemaToProtoLegendBy)
	DashboardValidLegendBys        = utils.GetKeys(DashboardSchemaToProtoLegendBy)

	DashboardSchemaToProtoPromQLQueryType = map[string]dashboardservice.PromQLQueryType{
		utils.UNSPECIFIED: dashboardservice.PROMQLQUERYTYPE_PROM_QL_QUERY_TYPE_UNSPECIFIED,
		"range":           dashboardservice.PROMQLQUERYTYPE_PROM_QL_QUERY_TYPE_RANGE,
		"instant":         dashboardservice.PROMQLQUERYTYPE_PROM_QL_QUERY_TYPE_INSTANT,
	}
	DashboardProtoToSchemaPromQLQueryType = utils.ReverseMap(DashboardSchemaToProtoPromQLQueryType)
	DashboardValidPromQLQueryType         = utils.GetKeys(DashboardSchemaToProtoPromQLQueryType)

	SupportedWidgetTypes = []string{
		"data_table",
		"gauge",
		"hexagon",
		"line_chart",
		"pie_chart",
		"bar_chart",
		"horizontal_bar_chart",
		"markdown",
	}
)

type QueryMetricsModel struct {
	PromqlQuery     types.String    `tfsdk:"promql_query"`
	Filters         types.List      `tfsdk:"filters"` //MetricsFilterModel
	PromqlQueryType types.String    `tfsdk:"promql_query_type"`
	TimeFrame       *TimeFrameModel `tfsdk:"time_frame"`
}

type MetricFilterModel struct {
	Metric   types.String         `tfsdk:"metric"`
	Label    types.String         `tfsdk:"label"`
	Operator *FilterOperatorModel `tfsdk:"operator"`
}

type QuerySpansModel struct {
	LuceneQuery  types.String    `tfsdk:"lucene_query"`
	GroupBy      types.List      `tfsdk:"group_by"`     //SpansFieldModel
	Aggregations types.List      `tfsdk:"aggregations"` //SpansAggregationModel
	Filters      types.List      `tfsdk:"filters"`      //SpansFilterModel
	TimeFrame    *TimeFrameModel `tfsdk:"time_frame"`
}

type SpansFieldModel struct {
	Type  types.String `tfsdk:"type"`
	Value types.String `tfsdk:"value"`
}

type LogsAggregationModel struct {
	Type             types.String  `tfsdk:"type"`
	Field            types.String  `tfsdk:"field"`
	Percent          types.Float64 `tfsdk:"percent"`
	ObservationField types.Object  `tfsdk:"observation_field"`
}

type DataPrimeModel struct {
	Query     types.String    `tfsdk:"query"`
	Filters   types.List      `tfsdk:"filters"` //DashboardFilterSourceModel
	TimeFrame *TimeFrameModel `tfsdk:"time_frame"`
}

type SpansAggregationModel struct {
	Type            types.String `tfsdk:"type"`
	AggregationType types.String `tfsdk:"aggregation_type"`
	Field           types.String `tfsdk:"field"`
}

type WidgetDefinitionModel struct {
	LineChart          *LineChartModel          `tfsdk:"line_chart"`
	Hexagon            *HexagonModel            `tfsdk:"hexagon"`
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
	StackedLine      types.String  `tfsdk:"stacked_line"`
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
	Logs      *LineChartQueryLogsModel  `tfsdk:"logs"`
	Metrics   *QueryMetricsModel        `tfsdk:"metrics"`
	Spans     *LineChartQuerySpansModel `tfsdk:"spans"`
	DataPrime *DataPrimeModel           `tfsdk:"data_prime"`
}

type LineChartQueryLogsModel struct {
	LuceneQuery  types.String    `tfsdk:"lucene_query"`
	GroupBy      types.List      `tfsdk:"group_by"`     //types.String
	Aggregations types.List      `tfsdk:"aggregations"` //AggregationModel
	Filters      types.List      `tfsdk:"filters"`      //FilterModel
	TimeFrame    *TimeFrameModel `tfsdk:"time_frame"`
}

type QueryMetricFilterModel struct {
	Metric   types.String         `tfsdk:"metric"`
	Label    types.String         `tfsdk:"label"`
	Operator *FilterOperatorModel `tfsdk:"operator"`
}

type LineChartQuerySpansModel struct {
	LuceneQuery  types.String    `tfsdk:"lucene_query"`
	GroupBy      types.List      `tfsdk:"group_by"`     //SpansFieldModel
	Aggregations types.List      `tfsdk:"aggregations"` //SpansAggregationModel
	Filters      types.List      `tfsdk:"filters"`      //SpansFilterModel
	TimeFrame    *TimeFrameModel `tfsdk:"time_frame"`
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
	TimeFrame   *TimeFrameModel                  `tfsdk:"time_frame"`
}

type LogsFilterModel struct {
	Field            types.String         `tfsdk:"field"`
	Operator         *FilterOperatorModel `tfsdk:"operator"`
	ObservationField types.Object         `tfsdk:"observation_field"` // ObservationFieldModel
}

type DataTableLogsQueryGroupingModel struct {
	Aggregations types.List `tfsdk:"aggregations"` //DataTableLogsAggregationModel
	GroupBys     types.List `tfsdk:"group_bys"`    //types.String
}

type DataTableLogsAggregationModel struct {
	ID          types.String          `tfsdk:"id"`
	Name        types.String          `tfsdk:"name"`
	IsVisible   types.Bool            `tfsdk:"is_visible"`
	Aggregation *LogsAggregationModel `tfsdk:"aggregation"`
}

type DataTableQueryModel struct {
	Logs      *DataTableQueryLogsModel  `tfsdk:"logs"`
	Metrics   *QueryMetricsModel        `tfsdk:"metrics"`
	Spans     *DataTableQuerySpansModel `tfsdk:"spans"`
	DataPrime *DataPrimeModel           `tfsdk:"data_prime"`
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
	TimeFrame   *TimeFrameModel                   `tfsdk:"time_frame"`
}

type SpansFilterModel struct {
	Field    *SpansFieldModel     `tfsdk:"field"`
	Operator *FilterOperatorModel `tfsdk:"operator"`
}

type DataTableSpansQueryGroupingModel struct {
	GroupBy      types.List `tfsdk:"group_by"`     //SpansFieldModel
	Aggregations types.List `tfsdk:"aggregations"` //DataTableSpansAggregationModel
}

type GaugeModel struct {
	Query             *GaugeQueryModel `tfsdk:"query"`
	Min               types.Float64    `tfsdk:"min"`
	Max               types.Float64    `tfsdk:"max"`
	ShowInnerArc      types.Bool       `tfsdk:"show_inner_arc"`
	ShowOuterArc      types.Bool       `tfsdk:"show_outer_arc"`
	Unit              types.String     `tfsdk:"unit"`
	Thresholds        types.List       `tfsdk:"thresholds"` //GaugeThresholdModel
	DataModeType      types.String     `tfsdk:"data_mode_type"`
	ThresholdBy       types.String     `tfsdk:"threshold_by"`
	ThresholdType     types.String     `tfsdk:"threshold_type"`
	DisplaySeriesName types.Bool       `tfsdk:"display_series_name"`
	Decimal           types.Number     `tfsdk:"decimal"`
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
	TimeFrame       *TimeFrameModel       `tfsdk:"time_frame"`
}

type GaugeQueryMetricsModel struct {
	PromqlQuery types.String    `tfsdk:"promql_query"`
	Aggregation types.String    `tfsdk:"aggregation"`
	Filters     types.List      `tfsdk:"filters"` //MetricsFilterModel
	TimeFrame   *TimeFrameModel `tfsdk:"time_frame"`
}

type GaugeQuerySpansModel struct {
	LuceneQuery      types.String           `tfsdk:"lucene_query"`
	SpansAggregation *SpansAggregationModel `tfsdk:"spans_aggregation"`
	Filters          types.List             `tfsdk:"filters"` //SpansFilterModel
	TimeFrame        *TimeFrameModel        `tfsdk:"time_frame"`
}

type GaugeThresholdModel struct {
	From  types.Float64 `tfsdk:"from"`
	Color types.String  `tfsdk:"color"`
	Label types.String  `tfsdk:"label"`
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
	TimeFrame             *TimeFrameModel       `tfsdk:"time_frame"`
}

type PieChartQueryMetricsModel struct {
	PromqlQuery      types.String    `tfsdk:"promql_query"`
	Filters          types.List      `tfsdk:"filters"`     //MetricsFilterModel
	GroupNames       types.List      `tfsdk:"group_names"` //types.String
	StackedGroupName types.String    `tfsdk:"stacked_group_name"`
	TimeFrame        *TimeFrameModel `tfsdk:"time_frame"`
}

type PieChartQuerySpansModel struct {
	LuceneQuery      types.String           `tfsdk:"lucene_query"`
	Aggregation      *SpansAggregationModel `tfsdk:"aggregation"`
	Filters          types.List             `tfsdk:"filters"`     //SpansFilterModel
	GroupNames       types.List             `tfsdk:"group_names"` //SpansFieldModel
	StackedGroupName *SpansFieldModel       `tfsdk:"stacked_group_name"`
	TimeFrame        *TimeFrameModel        `tfsdk:"time_frame"`
}

type PieChartQueryDataPrimeModel struct {
	Query            types.String    `tfsdk:"query"`
	Filters          types.List      `tfsdk:"filters"`     //DashboardFilterSourceModel
	GroupNames       types.List      `tfsdk:"group_names"` //types.String
	StackedGroupName types.String    `tfsdk:"stacked_group_name"`
	TimeFrame        *TimeFrameModel `tfsdk:"time_frame"`
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
	TimeFrame             *TimeFrameModel       `tfsdk:"time_frame"`
}

type ObservationFieldModel struct {
	Keypath types.List   `tfsdk:"keypath"` //types.String
	Scope   types.String `tfsdk:"scope"`
}

type BarChartQueryMetricsModel struct {
	PromqlQuery      types.String    `tfsdk:"promql_query"`
	Filters          types.List      `tfsdk:"filters"`     //MetricsFilterModel
	GroupNames       types.List      `tfsdk:"group_names"` //types.String
	StackedGroupName types.String    `tfsdk:"stacked_group_name"`
	TimeFrame        *TimeFrameModel `tfsdk:"time_frame"`
}

type BarChartQuerySpansModel struct {
	LuceneQuery      types.String           `tfsdk:"lucene_query"`
	Aggregation      *SpansAggregationModel `tfsdk:"aggregation"`
	Filters          types.List             `tfsdk:"filters"`     //SpansFilterModel
	GroupNames       types.List             `tfsdk:"group_names"` //SpansFieldModel
	StackedGroupName *SpansFieldModel       `tfsdk:"stacked_group_name"`
	TimeFrame        *TimeFrameModel        `tfsdk:"time_frame"`
}

type BarChartQueryDataPrimeModel struct {
	Query            types.String    `tfsdk:"query"`
	Filters          types.List      `tfsdk:"filters"`     //DashboardFilterSourceModel
	GroupNames       types.List      `tfsdk:"group_names"` //types.String
	StackedGroupName types.String    `tfsdk:"stacked_group_name"`
	TimeFrame        *TimeFrameModel `tfsdk:"time_frame"`
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
	Logs      types.Object `tfsdk:"logs"`       //BarChartQueryLogsModel
	Metrics   types.Object `tfsdk:"metrics"`    //BarChartQueryMetricsModel
	Spans     types.Object `tfsdk:"spans"`      //BarChartQuerySpansModel
	DataPrime types.Object `tfsdk:"data_prime"` //BarChartQueryDataPrimeModel
}

type MarkdownModel struct {
	MarkdownText types.String `tfsdk:"markdown_text"`
	TooltipText  types.String `tfsdk:"tooltip_text"`
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

type TimeFrameAbsoluteModel struct {
	Start types.String `tfsdk:"start"`
	End   types.String `tfsdk:"end"`
}

type TimeFrameRelativeModel struct {
	Duration types.String `tfsdk:"duration"`
}

type TimeFrameModel struct {
	Absolute *TimeFrameAbsoluteModel `tfsdk:"absolute"` //TimeFrameAbsoluteModel
	Relative *TimeFrameRelativeModel `tfsdk:"relative"` //TimeFrameRelativeModel
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
	if field.Type.ValueString() == "metadata" && !slices.Contains(DashboardValidSpanFieldMetadataFields, field.Value.ValueString()) {
		response.Diagnostics.Append(diag.NewErrorDiagnostic("spans field validation failed", fmt.Sprintf("when type is `metadata`, `value` must be one of %q", DashboardValidSpanFieldMetadataFields)))
	}
}

type FilterOperatorModel struct {
	Type           types.String `tfsdk:"type"`
	SelectedValues types.List   `tfsdk:"selected_values"` //types.String
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

type LegendModel struct {
	IsVisible    types.Bool   `tfsdk:"is_visible"`
	Columns      types.List   `tfsdk:"columns"` //types.String (DashboardValidLegendColumns)
	GroupByQuery types.Bool   `tfsdk:"group_by_query"`
	Placement    types.String `tfsdk:"placement"`
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

	if aggregation.Type.ValueString() == "metrics" && !slices.Contains(DashboardValidSpansAggregationMetricAggregationTypes, aggregation.AggregationType.ValueString()) {
		response.Diagnostics.Append(diag.NewErrorDiagnostic("spans aggregation validation failed", fmt.Sprintf("when type is `metrics`, `aggregation_type` must be one of %q", DashboardValidSpansAggregationMetricAggregationTypes)))
	}
	if aggregation.Type.ValueString() == "dimension" && !slices.Contains(DashboardValidSpansAggregationDimensionAggregationTypes, aggregation.AggregationType.ValueString()) {
		response.Diagnostics.Append(diag.NewErrorDiagnostic("spans aggregation validation failed", fmt.Sprintf("when type is `dimension`, `aggregation_type` must be one of %q", DashboardValidSpansAggregationDimensionAggregationTypes)))
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

	if aggregation.Type.IsNull() || aggregation.Type.IsUnknown() {
		return
	}

	aggregationType := aggregation.Type.ValueString()
	fieldKnownSet := !aggregation.Field.IsNull() && !aggregation.Field.IsUnknown()
	fieldKnownUnset := aggregation.Field.IsNull()
	obsKnownSet := !aggregation.ObservationField.IsNull() && !aggregation.ObservationField.IsUnknown()
	obsKnownUnset := aggregation.ObservationField.IsNull()

	if aggregationType == "count" {
		if fieldKnownSet || obsKnownSet {
			resp.Diagnostics.Append(diag.NewErrorDiagnostic("logs aggregation validation failed", "when type is `count`, neither `field` nor `observation_field` can be set"))
		}
	} else {
		if fieldKnownUnset && obsKnownUnset {
			resp.Diagnostics.Append(diag.NewErrorDiagnostic("logs aggregation validation failed", fmt.Sprintf("when type is `%s`, either `field` or `observation_field` must be set", aggregationType)))
		} else if fieldKnownSet && obsKnownSet {
			resp.Diagnostics.Append(diag.NewErrorDiagnostic("logs aggregation validation failed", fmt.Sprintf("when type is `%s`, `field` and `observation_field` are mutually exclusive — set exactly one", aggregationType)))
		}
	}

	percentKnownSet := !aggregation.Percent.IsNull() && !aggregation.Percent.IsUnknown()
	percentKnownUnset := aggregation.Percent.IsNull()

	if aggregationType == "percentile" && percentKnownUnset {
		resp.Diagnostics.Append(diag.NewErrorDiagnostic("logs aggregation validation failed", "when type is `percentile`, `percent` must be set"))
	} else if aggregationType != "percentile" && percentKnownSet {
		resp.Diagnostics.Append(diag.NewErrorDiagnostic("logs aggregation validation failed", fmt.Sprintf("when type is `%s`, `percent` cannot be set", aggregationType)))
	}
}

func FlattenLegend(legend *dashboardservice.Legend) *LegendModel {
	if legend == nil {
		return nil
	}

	return &LegendModel{
		IsVisible:    types.BoolPointerValue(legend.IsVisible),
		GroupByQuery: types.BoolPointerValue(legend.GroupByQuery),
		Columns:      flattenLegendColumns(legend.GetColumns()),
		Placement:    types.StringValue(DashboardLegendPlacementProtoToSchema[legend.GetPlacement()]),
	}
}

func flattenLegendColumns(columns []dashboardservice.LegendColumn) types.List {
	if len(columns) == 0 {
		return types.ListNull(types.StringType)
	}

	columnsElements := make([]attr.Value, 0, len(columns))
	for _, column := range columns {
		flattenedColumn := DashboardLegendColumnProtoToSchema[column]
		columnElement := types.StringValue(flattenedColumn)
		columnsElements = append(columnsElements, columnElement)
	}

	return types.ListValueMust(types.StringType, columnsElements)
}

func ExpandLegend(ctx context.Context, legend *LegendModel) (*dashboardservice.Legend, diag.Diagnostics) {
	if legend == nil {
		return nil, nil
	}

	columns := make([]dashboardservice.LegendColumn, 0, len(legend.Columns.Elements()))
	var columnsParsed []types.String
	if diags := legend.Columns.ElementsAs(ctx, &columnsParsed, true); diags.HasError() {
		return nil, diags
	}
	var diagnostics diag.Diagnostics
	for _, col := range columnsParsed {
		columns = append(columns, DashboardLegendColumnSchemaToProto[col.ValueString()])
	}
	if diagnostics.HasError() {
		return nil, diagnostics
	}

	return &dashboardservice.Legend{
		IsVisible:    legend.IsVisible.ValueBoolPointer(),
		Columns:      columns,
		GroupByQuery: legend.GroupByQuery.ValueBoolPointer(),
		Placement:    DashboardLegendPlacementSchemaToProto[legend.Placement.ValueString()].Ptr(),
	}, nil
}

func FlattenSpansFields(ctx context.Context, spanFields []dashboardservice.SpanField) (types.List, diag.Diagnostics) {
	if len(spanFields) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: SpansFieldModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	spanFieldElements := make([]attr.Value, 0, len(spanFields))
	for _, field := range spanFields {
		flattenedField, dg := FlattenSpansField(&field)
		if dg != nil {
			diagnostics.Append(dg)
			continue
		}
		fieldElement, diags := types.ObjectValueFrom(ctx, SpansFieldModelAttr(), flattenedField)
		if diags.HasError() {
			diagnostics = append(diagnostics, diags...)
			continue
		}
		spanFieldElements = append(spanFieldElements, fieldElement)
	}

	if diagnostics.HasError() {
		return types.ListNull(types.ObjectType{AttrTypes: SpansFieldModelAttr()}), diagnostics
	}

	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: SpansFieldModelAttr()}, spanFieldElements)
}

func FlattenSpansField(field *dashboardservice.SpanField) (*SpansFieldModel, diag.Diagnostic) {
	if field == nil {
		return nil, nil
	}

	switch {
	case field.MetadataField != nil:
		return &SpansFieldModel{
			Type:  types.StringValue("metadata"),
			Value: types.StringValue(DashboardProtoToSchemaSpanFieldMetadataField[field.GetMetadataField()]),
		}, nil
	case field.TagField != nil:
		return &SpansFieldModel{
			Type:  types.StringValue("tag"),
			Value: types.StringPointerValue(field.TagField),
		}, nil
	case field.ProcessTagField != nil:
		return &SpansFieldModel{
			Type:  types.StringValue("process_tag"),
			Value: types.StringPointerValue(field.ProcessTagField),
		}, nil

	default:
		return nil, diag.NewErrorDiagnostic("Error Flatten Spans Field", "unknown spans field type")
	}
}

func ObservationFieldsObject() types.ObjectType {
	return types.ObjectType{
		AttrTypes: ObservationFieldAttr(),
	}
}

func typeStringListToStringSlice(ctx context.Context, list types.List) ([]string, diag.Diagnostics) {
	if list.IsNull() || list.IsUnknown() {
		return nil, nil
	}
	var values []types.String
	diags := list.ElementsAs(ctx, &values, true)
	if diags.HasError() {
		return nil, diags
	}
	return utils.TypeStringSliceToStringSlice(values), nil
}

func int64ToInt32Pointer(value types.Int64) *int32 {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	converted := int32(value.ValueInt64())
	return &converted
}

func int32PointerToInt64Type(value *int32) types.Int64 {
	if value == nil {
		return types.Int64Null()
	}
	return types.Int64Value(int64(*value))
}

func numberTypeToFloat64Pointer(value types.Number) *float64 {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	converted, _ := value.ValueBigFloat().Float64()
	return &converted
}

func float64PointerToNumberType(value *float64) types.Number {
	if value == nil {
		return types.NumberNull()
	}
	return types.NumberValue(big.NewFloat(*value))
}

func numberTypeToInt32Pointer(value types.Number) *int32 {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	converted, _ := value.ValueBigFloat().Int64()
	result := int32(converted)
	return &result
}

func int32PointerToNumberType(value *int32) types.Number {
	if value == nil {
		return types.NumberNull()
	}
	return types.NumberValue(big.NewFloat(float64(*value)))
}

func FlattenDashboardFiltersSources(ctx context.Context, sources []dashboardservice.FilterSource) (types.List, diag.Diagnostics) {
	if len(sources) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: FilterSourceModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	filtersElements := make([]attr.Value, 0, len(sources))
	for i := range sources {
		flattenedFilter, diags := FlattenDashboardFilterSource(ctx, &sources[i])
		if diags.HasError() {
			diagnostics.Append(diags...)
			continue
		}
		filterElement, diags := types.ObjectValueFrom(ctx, FilterSourceModelAttr(), flattenedFilter)
		if diags.HasError() {
			diagnostics.Append(diags...)
			continue
		}
		filtersElements = append(filtersElements, filterElement)
	}

	return types.ListValueMust(types.ObjectType{AttrTypes: FilterSourceModelAttr()}, filtersElements), diagnostics
}

func FlattenDashboardFilterSource(ctx context.Context, source *dashboardservice.FilterSource) (*DashboardFilterSourceModel, diag.Diagnostics) {
	if source == nil {
		return nil, nil
	}

	switch {
	case source.Logs != nil:
		logs, diags := FlattenDashboardFilterSourceLogs(ctx, source.Logs)
		if diags.HasError() {
			return nil, diags
		}
		return &DashboardFilterSourceModel{Logs: logs}, nil
	case source.Spans != nil:
		spans, dg := FlattenDashboardFilterSourceSpans(source.Spans)
		if dg != nil {
			return nil, diag.Diagnostics{dg}
		}
		return &DashboardFilterSourceModel{Spans: spans}, nil
	case source.Metrics != nil:
		metrics, dg := FlattenDashboardFilterSourceMetrics(source.Metrics)
		if dg != nil {
			return nil, diag.Diagnostics{dg}
		}
		return &DashboardFilterSourceModel{Metrics: metrics}, nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Dashboard Filter Source", fmt.Sprintf("unknown filter source type %T", source))}
	}
}

func FlattenDashboardFilterSourceLogs(ctx context.Context, logs *dashboardservice.FilterLogsFilter) (*FilterSourceLogsModel, diag.Diagnostics) {
	if logs == nil {
		return nil, nil
	}

	operator, dg := FlattenFilterOperator(logs.Operator)
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	observationField, diags := FlattenObservationField(ctx, logs.ObservationField)
	if diags.HasError() {
		return nil, diags
	}

	return &FilterSourceLogsModel{
		Field:            utils.StringPointerToTypeString(logs.Field),
		Operator:         operator,
		ObservationField: observationField,
	}, nil
}

func FlattenDashboardFilterSourceSpans(spans *dashboardservice.SpansFilter) (*FilterSourceSpansModel, diag.Diagnostic) {
	if spans == nil {
		return nil, nil
	}

	field, dg := FlattenSpansField(spans.Field)
	if dg != nil {
		return nil, dg
	}

	operator, dg := FlattenFilterOperator(spans.Operator)
	if dg != nil {
		return nil, dg
	}

	return &FilterSourceSpansModel{
		Field:    field,
		Operator: operator,
	}, nil
}

func FlattenDashboardFilterSourceMetrics(metrics *dashboardservice.MetricsFilter) (*FilterSourceMetricsModel, diag.Diagnostic) {
	if metrics == nil {
		return nil, nil
	}

	operator, dg := FlattenFilterOperator(metrics.Operator)
	if dg != nil {
		return nil, dg
	}

	return &FilterSourceMetricsModel{
		MetricName:  utils.StringPointerToTypeString(metrics.Metric),
		MetricLabel: utils.StringPointerToTypeString(metrics.Label),
		Operator:    operator,
	}, nil
}

func FlattenDashboardTimeFrame(ctx context.Context, d *dashboardservice.Dashboard) (*TimeFrameModel, diag.Diagnostics) {
	switch {
	case d == nil:
		return nil, nil
	case d.AbsoluteTimeFrame != nil:
		return flattenAbsoluteTimeFrame(ctx, d.AbsoluteTimeFrame)
	case d.RelativeTimeFrame != nil:
		return flattenRelativeTimeFrame(ctx, d.RelativeTimeFrame)
	default:
		return nil, nil
	}
}

func FlattenTimeFrameSelect(ctx context.Context, d *dashboardservice.TimeFrameSelect) (*TimeFrameModel, diag.Diagnostics) {
	if d == nil {
		return nil, nil
	}
	switch {
	case d.AbsoluteTimeFrame != nil:
		return flattenAbsoluteTimeFrame(ctx, d.AbsoluteTimeFrame)
	case d.RelativeTimeFrame != nil:
		return flattenRelativeTimeFrame(ctx, d.RelativeTimeFrame)
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Dashboard Time Frame", fmt.Sprintf("unknown time frame type %T", d))}
	}
}

func FlattenObservationField(ctx context.Context, field *dashboardservice.ObservationField) (types.Object, diag.Diagnostics) {
	if field == nil {
		return types.ObjectNull(ObservationFieldAttr()), nil
	}

	return types.ObjectValueFrom(ctx, ObservationFieldAttr(), FlattenLogsFieldModel(field))
}

func FlattenLogsFieldModel(field *dashboardservice.ObservationField) *ObservationFieldModel {
	return &ObservationFieldModel{
		Keypath: utils.StringSliceToTypeStringList(field.GetKeypath()),
		Scope:   types.StringValue(DashboardProtoToSchemaObservationFieldScope[field.GetScope()]),
	}
}

func flattenDuration(timeFrame *string) basetypes.StringValue {
	if timeFrame == nil {
		return types.StringNull()
	}
	return types.StringValue(*timeFrame)
}

func flattenAbsoluteTimeFrame(ctx context.Context, timeFrame *dashboardservice.TimeFrame) (*TimeFrameModel, diag.Diagnostics) {
	absoluteTimeFrame := &TimeFrameAbsoluteModel{
		Start: types.StringValue(timeFrame.GetFrom().Format(time.RFC3339)),
		End:   types.StringValue(timeFrame.GetTo().Format(time.RFC3339)),
	}

	flattenedTimeFrame := &TimeFrameModel{
		Relative: nil,
		Absolute: absoluteTimeFrame,
	}
	return flattenedTimeFrame, nil
}

func flattenRelativeTimeFrame(ctx context.Context, timeFrame *string) (*TimeFrameModel, diag.Diagnostics) {
	relativeTimeFrame := &TimeFrameRelativeModel{
		Duration: flattenDuration(timeFrame),
	}

	flattenedTimeFrame := &TimeFrameModel{
		Relative: relativeTimeFrame,
		Absolute: nil,
	}
	return flattenedTimeFrame, nil
}

func FlattenSpansFilters(ctx context.Context, filters []dashboardservice.SpansFilter) (types.List, diag.Diagnostics) {
	if len(filters) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: SpansFilterModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	filtersElements := make([]attr.Value, 0, len(filters))
	for i := range filters {
		flattenedFilter, dg := FlattenSpansFilter(&filters[i])
		if dg != nil {
			diagnostics.Append(dg)
			continue
		}
		filterElement, diags := types.ObjectValueFrom(ctx, SpansFilterModelAttr(), flattenedFilter)
		if diags.HasError() {
			diagnostics = append(diagnostics, diags...)
			continue
		}
		filtersElements = append(filtersElements, filterElement)
	}

	return types.ListValueMust(types.ObjectType{AttrTypes: SpansFilterModelAttr()}, filtersElements), diagnostics
}

func FlattenSpansFilter(filter *dashboardservice.SpansFilter) (*SpansFilterModel, diag.Diagnostic) {
	if filter == nil {
		return nil, nil
	}

	operator, dg := FlattenFilterOperator(filter.Operator)
	if dg != nil {
		return nil, dg
	}

	field, dg := FlattenSpansField(filter.Field)
	if dg != nil {
		return nil, dg
	}

	return &SpansFilterModel{
		Field:    field,
		Operator: operator,
	}, nil
}

func FlattenFilterOperator(operator *dashboardservice.FilterOperator) (*FilterOperatorModel, diag.Diagnostic) {
	if operator == nil {
		return nil, nil
	}

	switch {
	case operator.Equals != nil:
		switch {
		case operator.Equals.Selection != nil && operator.Equals.Selection.All != nil:
			return &FilterOperatorModel{
				Type:           types.StringValue("equals"),
				SelectedValues: types.ListNull(types.StringType),
			}, nil
		case operator.Equals.Selection != nil && operator.Equals.Selection.List != nil:
			return &FilterOperatorModel{
				Type:           types.StringValue("equals"),
				SelectedValues: utils.StringSliceToTypeStringList(operator.Equals.Selection.List.GetValues()),
			}, nil
		default:
			return nil, diag.NewErrorDiagnostic("Error Flatten Logs Filter Operator Equals", "unknown logs filter operator equals selection type")
		}
	case operator.NotEquals != nil:
		switch {
		case operator.NotEquals.Selection != nil && operator.NotEquals.Selection.List != nil:
			return &FilterOperatorModel{
				Type:           types.StringValue("not_equals"),
				SelectedValues: utils.StringSliceToTypeStringList(operator.NotEquals.Selection.List.GetValues()),
			}, nil
		default:
			return nil, diag.NewErrorDiagnostic("Error Flatten Logs Filter Operator NotEquals", "unknown logs filter operator not_equals selection type")
		}
	default:
		return nil, diag.NewErrorDiagnostic("Error Flatten Logs Filter Operator", "unknown logs filter operator type")
	}
}

func FlattenMetricsFilters(ctx context.Context, filters []dashboardservice.MetricsFilter) (types.List, diag.Diagnostics) {
	if len(filters) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: MetricsFilterModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	filtersElements := make([]attr.Value, 0, len(filters))
	for i := range filters {
		flattenedFilter, dg := FlattenMetricsFilter(&filters[i])
		if dg != nil {
			diagnostics.Append(dg)
			continue
		}
		filterElement, diags := types.ObjectValueFrom(ctx, MetricsFilterModelAttr(), flattenedFilter)
		if diags.HasError() {
			diagnostics = append(diagnostics, diags...)
			continue
		}
		filtersElements = append(filtersElements, filterElement)
	}

	return types.ListValueMust(types.ObjectType{AttrTypes: MetricsFilterModelAttr()}, filtersElements), diagnostics
}

func FlattenMetricsFilter(filter *dashboardservice.MetricsFilter) (*MetricsFilterModel, diag.Diagnostic) {
	if filter == nil {
		return nil, nil
	}

	operator, dg := FlattenFilterOperator(filter.Operator)
	if dg != nil {
		return nil, dg
	}

	return &MetricsFilterModel{
		Metric:   utils.StringPointerToTypeString(filter.Metric),
		Label:    utils.StringPointerToTypeString(filter.Label),
		Operator: operator,
	}, nil
}

func FlattenLogsFilters(ctx context.Context, filters []dashboardservice.FilterLogsFilter) (types.List, diag.Diagnostics) {
	if len(filters) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: LogsFilterModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	filtersElements := make([]attr.Value, 0, len(filters))
	for i := range filters {
		flattenedFilter, diags := flattenLogsFilter(ctx, &filters[i])
		if diags.HasError() {
			diagnostics.Append(diags...)
			continue
		}
		filterElement, diags := types.ObjectValueFrom(ctx, LogsFilterModelAttr(), flattenedFilter)
		if diags.HasError() {
			diagnostics.Append(diags...)
			continue
		}
		filtersElements = append(filtersElements, filterElement)
	}

	return types.ListValueMust(types.ObjectType{AttrTypes: LogsFilterModelAttr()}, filtersElements), diagnostics
}

func flattenLogsFilter(ctx context.Context, filter *dashboardservice.FilterLogsFilter) (*LogsFilterModel, diag.Diagnostics) {
	if filter == nil {
		return nil, nil
	}

	operator, dg := FlattenFilterOperator(filter.Operator)
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	observationField, diags := FlattenObservationField(ctx, filter.ObservationField)
	if diags.HasError() {
		return nil, diags
	}

	return &LogsFilterModel{
		Field:            utils.StringPointerToTypeString(filter.Field),
		Operator:         operator,
		ObservationField: observationField,
	}, nil
}

func FlattenObservationFields(ctx context.Context, namesFields []dashboardservice.ObservationField) (types.List, diag.Diagnostics) {
	if len(namesFields) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: ObservationFieldAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	fieldElements := make([]attr.Value, 0, len(namesFields))
	for i := range namesFields {
		flattenedField, diags := FlattenObservationField(ctx, &namesFields[i])
		if diags != nil {
			diagnostics.Append(diags...)
			continue
		}
		fieldElement, diags := types.ObjectValueFrom(ctx, ObservationFieldAttr(), flattenedField)
		if diags.HasError() {
			diagnostics.Append(diags...)
			continue
		}
		fieldElements = append(fieldElements, fieldElement)
	}

	if diagnostics.HasError() {
		return types.ListNull(types.ObjectType{AttrTypes: ObservationFieldAttr()}), diagnostics
	}

	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: ObservationFieldAttr()}, fieldElements)
}

func FlattenLogsAggregation(ctx context.Context, aggregation *dashboardservice.LogsAggregation) (*LogsAggregationModel, diag.Diagnostics) {
	if aggregation == nil {
		return nil, nil
	}

	switch {
	case aggregation.Count != nil:
		return &LogsAggregationModel{
			Type:             types.StringValue("count"),
			ObservationField: types.ObjectNull(ObservationFieldAttr()),
		}, nil
	case aggregation.CountDistinct != nil:
		observationField, diags := FlattenObservationField(ctx, aggregation.CountDistinct.ObservationField)
		if diags.HasError() {
			return nil, diags
		}
		return &LogsAggregationModel{
			Type:             types.StringValue("count_distinct"),
			Field:            utils.StringPointerToTypeString(aggregation.CountDistinct.Field),
			ObservationField: observationField,
		}, nil
	case aggregation.Sum != nil:
		observationField, diags := FlattenObservationField(ctx, aggregation.Sum.ObservationField)
		if diags.HasError() {
			return nil, diags
		}
		return &LogsAggregationModel{
			Type:             types.StringValue("sum"),
			Field:            utils.StringPointerToTypeString(aggregation.Sum.Field),
			ObservationField: observationField,
		}, nil
	case aggregation.Average != nil:
		observationField, diags := FlattenObservationField(ctx, aggregation.Average.ObservationField)
		if diags.HasError() {
			return nil, diags
		}
		return &LogsAggregationModel{
			Type:             types.StringValue("avg"),
			Field:            utils.StringPointerToTypeString(aggregation.Average.Field),
			ObservationField: observationField,
		}, nil
	case aggregation.Min != nil:
		observationField, diags := FlattenObservationField(ctx, aggregation.Min.ObservationField)
		if diags.HasError() {
			return nil, diags
		}
		return &LogsAggregationModel{
			Type:             types.StringValue("min"),
			Field:            utils.StringPointerToTypeString(aggregation.Min.Field),
			ObservationField: observationField,
		}, nil
	case aggregation.Max != nil:
		observationField, diags := FlattenObservationField(ctx, aggregation.Max.ObservationField)
		if diags.HasError() {
			return nil, diags
		}
		return &LogsAggregationModel{
			Type:             types.StringValue("max"),
			Field:            utils.StringPointerToTypeString(aggregation.Max.Field),
			ObservationField: observationField,
		}, nil
	case aggregation.Percentile != nil:
		observationField, diags := FlattenObservationField(ctx, aggregation.Percentile.ObservationField)
		if diags.HasError() {
			return nil, diags
		}
		return &LogsAggregationModel{
			Type:             types.StringValue("percentile"),
			Field:            utils.StringPointerToTypeString(aggregation.Percentile.Field),
			Percent:          types.Float64PointerValue(aggregation.Percentile.Percent),
			ObservationField: observationField,
		}, nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Logs Aggregation", "unknown logs aggregation type")}
	}
}

func ExpandObservationFields(ctx context.Context, namesFields types.List) ([]dashboardservice.ObservationField, diag.Diagnostics) {
	var namesFieldsObjects []types.Object
	var expandedNamesFields []dashboardservice.ObservationField
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
		expandedNamesFields = append(expandedNamesFields, *expandedNamesField)
	}

	return expandedNamesFields, diags
}

func ExpandObservationFieldObject(ctx context.Context, field types.Object) (*dashboardservice.ObservationField, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(field) {
		return nil, nil
	}

	var observationField ObservationFieldModel
	if dg := field.As(ctx, &observationField, basetypes.ObjectAsOptions{}); dg.HasError() {
		return nil, dg
	}

	return expandObservationField(ctx, observationField)
}

func expandObservationField(ctx context.Context, observationField ObservationFieldModel) (*dashboardservice.ObservationField, diag.Diagnostics) {
	keypath, dg := typeStringListToStringSlice(ctx, observationField.Keypath)
	if dg.HasError() {
		return nil, dg
	}

	scope := DashboardSchemaToProtoObservationFieldScope[observationField.Scope.ValueString()]

	return &dashboardservice.ObservationField{
		Keypath: keypath,
		Scope:   scope.Ptr(),
	}, nil
}

func ExpandSpansField(spansFilterField *SpansFieldModel) (*dashboardservice.SpanField, diag.Diagnostic) {
	if spansFilterField == nil {
		return nil, nil
	}

	switch spansFilterField.Type.ValueString() {
	case "metadata":
		return &dashboardservice.SpanField{
			MetadataField: DashboardSchemaToProtoSpanFieldMetadataField[spansFilterField.Value.ValueString()].Ptr(),
		}, nil
	case "tag":
		return &dashboardservice.SpanField{
			TagField: utils.TypeStringToStringPointer(spansFilterField.Value),
		}, nil
	case "process_tag":
		return &dashboardservice.SpanField{
			ProcessTagField: utils.TypeStringToStringPointer(spansFilterField.Value),
		}, nil
	default:
		return nil, diag.NewErrorDiagnostic("Extract Spans Filter Field Error", fmt.Sprintf("Unknown spans filter field type %s", spansFilterField.Type.ValueString()))
	}
}

func ExpandSpansFields(ctx context.Context, spanFields types.List) ([]dashboardservice.SpanField, diag.Diagnostics) {
	var spanFieldsObjects []types.Object
	var expandedSpanFields []dashboardservice.SpanField
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
		expandedSpanField, expandDiag := ExpandSpansField(&spansField)
		if expandDiag != nil {
			diags.Append(expandDiag)
			continue
		}
		expandedSpanFields = append(expandedSpanFields, *expandedSpanField)
	}

	return expandedSpanFields, diags
}

func ExpandLogsAggregations(ctx context.Context, logsAggregations types.List) ([]dashboardservice.LogsAggregation, diag.Diagnostics) {
	var logsAggregationsObjects []types.Object
	var expandedLogsAggregations []dashboardservice.LogsAggregation
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
		expandedLogsAggregation, expandDiags := ExpandLogsAggregation(ctx, &aggregation)
		if expandDiags.HasError() {
			diags.Append(expandDiags...)
			continue
		}
		expandedLogsAggregations = append(expandedLogsAggregations, *expandedLogsAggregation)
	}

	return expandedLogsAggregations, diags
}

func ExpandLogsAggregation(ctx context.Context, logsAggregation *LogsAggregationModel) (*dashboardservice.LogsAggregation, diag.Diagnostics) {
	if logsAggregation == nil {
		return nil, nil
	}
	switch logsAggregation.Type.ValueString() {
	case "count":
		return &dashboardservice.LogsAggregation{
			Count: map[string]interface{}{},
		}, nil
	case "count_distinct":
		observationField, diags := ExpandObservationFieldObject(ctx, logsAggregation.ObservationField)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboardservice.LogsAggregation{
			CountDistinct: &dashboardservice.CountDistinct{
				Field:            utils.TypeStringToStringPointer(logsAggregation.Field),
				ObservationField: observationField,
			},
		}, nil
	case "sum":
		observationField, diags := ExpandObservationFieldObject(ctx, logsAggregation.ObservationField)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboardservice.LogsAggregation{
			Sum: &dashboardservice.Sum{
				Field:            utils.TypeStringToStringPointer(logsAggregation.Field),
				ObservationField: observationField,
			},
		}, nil
	case "avg":
		observationField, diags := ExpandObservationFieldObject(ctx, logsAggregation.ObservationField)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboardservice.LogsAggregation{
			Average: &dashboardservice.Average{
				Field:            utils.TypeStringToStringPointer(logsAggregation.Field),
				ObservationField: observationField,
			},
		}, nil
	case "min":
		observationField, diags := ExpandObservationFieldObject(ctx, logsAggregation.ObservationField)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboardservice.LogsAggregation{
			Min: &dashboardservice.Min{
				Field:            utils.TypeStringToStringPointer(logsAggregation.Field),
				ObservationField: observationField,
			},
		}, nil
	case "max":
		observationField, diags := ExpandObservationFieldObject(ctx, logsAggregation.ObservationField)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboardservice.LogsAggregation{
			Max: &dashboardservice.Max{
				Field:            utils.TypeStringToStringPointer(logsAggregation.Field),
				ObservationField: observationField,
			},
		}, nil
	case "percentile":
		observationField, diags := ExpandObservationFieldObject(ctx, logsAggregation.ObservationField)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboardservice.LogsAggregation{
			Percentile: &dashboardservice.Percentile{
				Field:            utils.TypeStringToStringPointer(logsAggregation.Field),
				Percent:          logsAggregation.Percent.ValueFloat64Pointer(),
				ObservationField: observationField,
			},
		}, nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error expand logs aggregation", fmt.Sprintf("unknown logs aggregation type %s", logsAggregation.Type.ValueString()))}
	}
}

func ExpandTimeFrameSelect(ctx context.Context, timeFrame *TimeFrameModel) (*dashboardservice.TimeFrameSelect, diag.Diagnostics) {
	if timeFrame == nil {
		return nil, nil
	}

	tf := dashboardservice.TimeFrameSelect{}

	switch {
	case timeFrame.Relative != nil:
		val, diags := expandRelativeTimeFrame(ctx, timeFrame.Relative)
		if diags.HasError() {
			return nil, diags
		}
		tf.RelativeTimeFrame = val
	case timeFrame.Absolute != nil:
		absoluteTimeFrame, diags := expandAbsoluteTimeFrame(ctx, timeFrame.Absolute)
		if diags.HasError() {
			return nil, diags
		}
		tf.AbsoluteTimeFrame = absoluteTimeFrame
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand Time Frame", "Dashboard TimeFrame must be either Relative or Absolute")}
	}
	return &tf, nil
}

func ExpandDashboardTimeFrame(ctx context.Context, dashboard *dashboardservice.Dashboard, timeFrame *TimeFrameModel) (*dashboardservice.Dashboard, diag.Diagnostics) {
	if timeFrame == nil {
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("No time frame received", "time frame was nil")}
	}

	var diags diag.Diagnostics
	switch {
	case timeFrame.Relative != nil:
		relative, diags := expandRelativeTimeFrame(ctx, timeFrame.Relative)
		if diags.HasError() {
			return nil, diags
		}
		dashboard.RelativeTimeFrame = relative
	case timeFrame.Absolute != nil:
		absoluteTimeFrame, diags := expandAbsoluteTimeFrame(ctx, timeFrame.Absolute)
		if diags.HasError() {
			return nil, diags
		}
		dashboard.AbsoluteTimeFrame = absoluteTimeFrame
	default:
		diags = diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand Time Frame", "Dashboard TimeFrame must be either Relative or Absolute")}
	}
	return dashboard, diags
}

func expandRelativeTimeFrame(ctx context.Context, timeFrame *TimeFrameRelativeModel) (*string, diag.Diagnostics) {
	if _, dg := utils.ParseDuration(timeFrame.Duration.ValueString(), "Relative Dashboard Time Frame"); dg != nil {
		return nil, diag.Diagnostics{dg}
	}
	return timeFrame.Duration.ValueStringPointer(), nil
}

func expandAbsoluteTimeFrame(ctx context.Context, timeFrame *TimeFrameAbsoluteModel) (*dashboardservice.TimeFrame, diag.Diagnostics) {
	fromTime, err := time.Parse(time.RFC3339, timeFrame.Start.ValueString())
	if err != nil {
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand Absolute Dashboard Time Frame", fmt.Sprintf("Error parsing from time: %s", err.Error()))}
	}

	toTime, err := time.Parse(time.RFC3339, timeFrame.End.ValueString())
	if err != nil {
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand Absolute Dashboard Time Frame", fmt.Sprintf("Error parsing from time: %s", err.Error()))}
	}

	return &dashboardservice.TimeFrame{
		From: &fromTime,
		To:   &toTime,
	}, nil
}

func SupportedWidgetsValidatorWithout(current string) validator.Object {
	matchers := make([]path.Expression, len(SupportedWidgetTypes)-1)
	for _, name := range SupportedWidgetTypes {
		if name != current {
			matchers = append(matchers, path.MatchRelative().AtParent().AtName(name))
		}
	}
	return objectvalidator.ExactlyOneOf(matchers...)
}

func FlattenSpansAggregation(aggregation *dashboardservice.SpansAggregation) (*SpansAggregationModel, diag.Diagnostic) {
	if aggregation == nil {
		return nil, nil
	}
	switch {
	case aggregation.MetricAggregation != nil:
		return &SpansAggregationModel{
			Type:            types.StringValue("metric"),
			AggregationType: types.StringValue(DashboardProtoToSchemaSpansAggregationMetricAggregationType[aggregation.MetricAggregation.GetAggregationType()]),
			Field:           types.StringValue(DashboardProtoToSchemaSpansAggregationMetricField[aggregation.MetricAggregation.GetMetricField()]),
		}, nil
	case aggregation.DimensionAggregation != nil:
		return &SpansAggregationModel{
			Type:            types.StringValue("dimension"),
			AggregationType: types.StringValue(DashboardProtoToSchemaSpansAggregationDimensionAggregationType[aggregation.DimensionAggregation.GetAggregationType()]),
			Field:           types.StringValue(DashboardSchemaToProtoSpansAggregationDimensionField[aggregation.DimensionAggregation.GetDimensionField()]),
		}, nil
	default:
		return nil, diag.NewErrorDiagnostic("Error Flatten Span Aggregation", fmt.Sprintf("unknown aggregation type %T", aggregation))
	}
}

func ExpandResolution(ctx context.Context, resolution types.Object) (*dashboardservice.LineChartResolution, diag.Diagnostics) {
	if resolution.IsNull() || resolution.IsUnknown() {
		return nil, nil
	}

	var resolutionModel LineChartResolutionModel
	if diags := resolution.As(ctx, &resolutionModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	if !(resolutionModel.Interval.IsNull() || resolutionModel.Interval.IsUnknown()) {
		if _, dg := utils.ParseDuration(resolutionModel.Interval.ValueString(), "resolution.interval"); dg != nil {
			return nil, diag.Diagnostics{dg}
		}

		return &dashboardservice.LineChartResolution{
			Interval: resolutionModel.Interval.ValueStringPointer(),
		}, nil
	}

	return &dashboardservice.LineChartResolution{
		BucketsPresented: int64ToInt32Pointer(resolutionModel.BucketsPresented),
	}, nil
}

func ExpandDashboardUUID(id types.String) *dashboardservice.UUID {
	if id.IsNull() || id.IsUnknown() {
		value := uuid.NewString()
		return &dashboardservice.UUID{Value: &value}
	}
	return &dashboardservice.UUID{Value: id.ValueStringPointer()}
}

func ExpandDashboardIDs(id types.String) *string {
	if id.IsNull() || id.IsUnknown() {
		value := uuid.NewString()
		return &value
	}
	return id.ValueStringPointer()
}

func ExpandDashboardFiltersSources(ctx context.Context, filters types.List) ([]dashboardservice.FilterSource, diag.Diagnostics) {
	var filtersObjects []types.Object
	var expandedFiltersSources []dashboardservice.FilterSource
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
		expandedFilter, expandDiags := ExpandFilterSource(ctx, &filterSource)
		if expandDiags.HasError() {
			diags.Append(expandDiags...)
			continue
		}
		expandedFiltersSources = append(expandedFiltersSources, *expandedFilter)
	}

	return expandedFiltersSources, diags
}
