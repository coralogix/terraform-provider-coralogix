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

package dashboardwidgets

import (
	"context"
	"fmt"
	"slices"
	"terraform-provider-coralogix/coralogix/utils"
	"time"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Protobuf default value
const UNSPECIFIED = "unspecified"

var (
	DashboardSchemaToProtoUnit = map[string]cxsdk.Unit{
		UNSPECIFIED:    cxsdk.UnitUnspecified,
		"microseconds": cxsdk.UnitMicroseconds,
		"milliseconds": cxsdk.UnitMilliseconds,
		"nanoseconds":  cxsdk.UnitNanoseconds,
		"seconds":      cxsdk.UnitSeconds,
		"bytes":        cxsdk.UnitBytes,
		"kbytes":       cxsdk.UnitKbytes,
		"mbytes":       cxsdk.UnitMbytes,
		"gbytes":       cxsdk.UnitGbytes,
		"bytes_iec":    cxsdk.UnitBytesIec,
		"kibytes":      cxsdk.UnitKibytes,
		"mibytes":      cxsdk.UnitMibytes,
		"gibytes":      cxsdk.UnitGibytes,
		"euro_cents":   cxsdk.UnitEurCents,
		"euro":         cxsdk.UnitEur,
		"usd_cents":    cxsdk.UnitUsdCents,
		"usd":          cxsdk.UnitUsd,
		"custom":       cxsdk.UnitCustom,
		"percent01":    cxsdk.UnitPercent01,
		"percent100":   cxsdk.UnitPercent100,
	}
	DashboardProtoToSchemaUnit = utils.ReverseMap(DashboardSchemaToProtoUnit)
	DashboardValidUnits        = utils.GetKeys(DashboardSchemaToProtoUnit)

	DashboardLegendPlacementSchemaToProto = map[string]cxsdk.LegendPlacement{
		UNSPECIFIED: cxsdk.LegendPlacementUnspecified,
		"auto":      cxsdk.LegendPlacementAuto,
		"bottom":    cxsdk.LegendPlacementBottom,
		"side":      cxsdk.LegendPlacementSide,
		"hidden":    cxsdk.LegendPlacementHidden,
	}
	DashboardLegendPlacementProtoToSchema = utils.ReverseMap(DashboardLegendPlacementSchemaToProto)
	DashboardValidLegendPlacements        = utils.GetKeys(DashboardLegendPlacementSchemaToProto)

	DashboardRowStyleSchemaToProto = map[string]cxsdk.RowStyle{
		UNSPECIFIED: cxsdk.RowStyleUnspecified,
		"one_line":  cxsdk.RowStyleOneLine,
		"two_line":  cxsdk.RowStyleTwoLine,
		"condensed": cxsdk.RowStyleCondensed,
		"json":      cxsdk.RowStyleJSON,
		"list":      cxsdk.RowStyleList,
	}
	DashboardRowStyleProtoToSchema     = utils.ReverseMap(DashboardRowStyleSchemaToProto)
	DashboardValidRowStyles            = utils.GetKeys(DashboardRowStyleSchemaToProto)
	DashboardLegendColumnSchemaToProto = map[string]cxsdk.DashboardLegendColumn{
		UNSPECIFIED: cxsdk.LegendColumnUnspecified,
		"min":       cxsdk.LegendColumnMin,
		"max":       cxsdk.LegendColumnMax,
		"sum":       cxsdk.LegendColumnSum,
		"avg":       cxsdk.LegendColumnAvg,
		"last":      cxsdk.LegendColumnLast,
		"name":      cxsdk.LegendColumnName,
	}
	DashboardLegendColumnProtoToSchema   = utils.ReverseMap(DashboardLegendColumnSchemaToProto)
	DashboardValidLegendColumns          = utils.GetKeys(DashboardLegendColumnSchemaToProto)
	DashboardOrderDirectionSchemaToProto = map[string]cxsdk.OrderDirection{
		UNSPECIFIED: cxsdk.OrderDirectionUnspecified,
		"asc":       cxsdk.OrderDirectionAsc,
		"desc":      cxsdk.OrderDirectionDesc,
	}
	DashboardOrderDirectionProtoToSchema = utils.ReverseMap(DashboardOrderDirectionSchemaToProto)
	DashboardValidOrderDirections        = utils.GetKeys(DashboardOrderDirectionSchemaToProto)
	DashboardSchemaToProtoTooltipType    = map[string]cxsdk.LineChartTooltipType{
		UNSPECIFIED: cxsdk.LineChartToolTipTypeUnspecified,
		"all":       cxsdk.LineChartToolTipTypeAll,
		"single":    cxsdk.LineChartToolTipTypeSingle,
	}
	DashboardProtoToSchemaTooltipType = utils.ReverseMap(DashboardSchemaToProtoTooltipType)
	DashboardValidTooltipTypes        = utils.GetKeys(DashboardSchemaToProtoTooltipType)
	DashboardSchemaToProtoScaleType   = map[string]cxsdk.ScaleType{
		UNSPECIFIED:   cxsdk.ScaleTypeUnspecified,
		"linear":      cxsdk.ScaleTypeLinear,
		"logarithmic": cxsdk.ScaleTypeLogarithmic,
	}
	DashboardProtoToSchemaScaleType = utils.ReverseMap(DashboardSchemaToProtoScaleType)
	DashboardValidScaleTypes        = utils.GetKeys(DashboardSchemaToProtoScaleType)

	DashboardSchemaToProtoGaugeUnit = map[string]cxsdk.GaugeUnit{
		UNSPECIFIED:    cxsdk.GaugeUnitUnspecified,
		"none":         cxsdk.GaugeUnitMicroseconds,
		"percent":      cxsdk.GaugeUnitMilliseconds,
		"microseconds": cxsdk.GaugeUnitNanoseconds,
		"milliseconds": cxsdk.GaugeUnitNumber,
		"nanoseconds":  cxsdk.GaugeUnitPercent,
		"seconds":      cxsdk.GaugeUnitSeconds,
		"bytes":        cxsdk.GaugeUnitBytes,
		"kbytes":       cxsdk.GaugeUnitKbytes,
		"mbytes":       cxsdk.GaugeUnitMbytes,
		"gbytes":       cxsdk.GaugeUnitGbytes,
		"bytes_iec":    cxsdk.GaugeUnitBytesIec,
		"kibytes":      cxsdk.GaugeUnitKibytes,
		"mibytes":      cxsdk.GaugeUnitMibytes,
		"gibytes":      cxsdk.GaugeUnitGibytes,
		"euro_cents":   cxsdk.GaugeUnitEurCents,
		"euro":         cxsdk.GaugeUnitEur,
		"usd_cents":    cxsdk.GaugeUnitUsdCents,
		"usd":          cxsdk.GaugeUnitUsd,
		"custom":       cxsdk.GaugeUnitCustom,
		"percent01":    cxsdk.GaugeUnitPercent01,
		"percent100":   cxsdk.GaugeUnitPercent100,
	}
	DashboardProtoToSchemaGaugeUnit           = utils.ReverseMap(DashboardSchemaToProtoGaugeUnit)
	DashboardValidGaugeUnits                  = utils.GetKeys(DashboardSchemaToProtoGaugeUnit)
	DashboardSchemaToProtoPieChartLabelSource = map[string]cxsdk.PieChartLabelSource{
		UNSPECIFIED: cxsdk.PieChartLabelSourceUnspecified,
		"inner":     cxsdk.PieChartLabelSourceInner,
		"stack":     cxsdk.PieChartLabelSourceStack,
	}
	DashboardProtoToSchemaPieChartLabelSource = utils.ReverseMap(DashboardSchemaToProtoPieChartLabelSource)
	DashboardValidPieChartLabelSources        = utils.GetKeys(DashboardSchemaToProtoPieChartLabelSource)
	DashboardSchemaToProtoGaugeAggregation    = map[string]cxsdk.GaugeAggregation{
		UNSPECIFIED: cxsdk.GaugeAggregationUnspecified,
		"last":      cxsdk.GaugeAggregationLast,
		"min":       cxsdk.GaugeAggregationMin,
		"max":       cxsdk.GaugeAggregationMax,
		"avg":       cxsdk.GaugeAggregationAvg,
		"sum":       cxsdk.GaugeAggregationSum,
	}
	DashboardProtoToSchemaGaugeAggregation            = utils.ReverseMap(DashboardSchemaToProtoGaugeAggregation)
	DashboardValidGaugeAggregations                   = utils.GetKeys(DashboardSchemaToProtoGaugeAggregation)
	DashboardSchemaToProtoSpansAggregationMetricField = map[string]cxsdk.SpansAggregationMetricAggregationMetricField{
		UNSPECIFIED: cxsdk.SpansAggregationMetricAggregationMetricFieldUnspecified,
		"duration":  cxsdk.SpansAggregationMetricAggregationMetricFieldDuration,
	}
	DashboardProtoToSchemaSpansAggregationMetricField           = utils.ReverseMap(DashboardSchemaToProtoSpansAggregationMetricField)
	DashboardValidSpansAggregationMetricFields                  = utils.GetKeys(DashboardSchemaToProtoSpansAggregationMetricField)
	DashboardSchemaToProtoSpansAggregationMetricAggregationType = map[string]cxsdk.SpansAggregationMetricAggregationMetricAggregationType{
		UNSPECIFIED:     cxsdk.SpansAggregationMetricAggregationMetricTypeUnspecified,
		"min":           cxsdk.SpansAggregationMetricAggregationMetricTypeMin,
		"max":           cxsdk.SpansAggregationMetricAggregationMetricTypeMax,
		"avg":           cxsdk.SpansAggregationMetricAggregationMetricTypeAverage,
		"sum":           cxsdk.SpansAggregationMetricAggregationMetricTypeSum,
		"percentile_99": cxsdk.SpansAggregationMetricAggregationMetricTypePercentile99,
		"percentile_95": cxsdk.SpansAggregationMetricAggregationMetricTypePercentile95,
		"percentile_50": cxsdk.SpansAggregationMetricAggregationMetricTypePercentile50,
	}
	DashboardProtoToSchemaSpansAggregationMetricAggregationType = utils.ReverseMap(DashboardSchemaToProtoSpansAggregationMetricAggregationType)
	DashboardValidSpansAggregationMetricAggregationTypes        = utils.GetKeys(DashboardSchemaToProtoSpansAggregationMetricAggregationType)
	DashboardProtoToSchemaSpansAggregationDimensionField        = map[string]cxsdk.SpansAggregationDimensionAggregationDimensionField{
		UNSPECIFIED: cxsdk.SpansAggregationDimensionAggregationDimensionFieldUnspecified,
		"trace_id":  cxsdk.SpansAggregationDimensionAggregationDimensionFieldTraceID,
	}
	DashboardSchemaToProtoSpansAggregationDimensionField           = utils.ReverseMap(DashboardProtoToSchemaSpansAggregationDimensionField)
	DashboardValidSpansAggregationDimensionFields                  = utils.GetKeys(DashboardProtoToSchemaSpansAggregationDimensionField)
	DashboardSchemaToProtoSpansAggregationDimensionAggregationType = map[string]cxsdk.SpansAggregationDimensionAggregationType{
		UNSPECIFIED:    cxsdk.SpansAggregationDimensionAggregationTypeUnspecified,
		"unique_count": cxsdk.SpansAggregationDimensionAggregationTypeUniqueCount,
		"error_count":  cxsdk.SpansAggregationDimensionAggregationTypeErrorCount,
	}
	DashboardProtoToSchemaSpansAggregationDimensionAggregationType = utils.ReverseMap(DashboardSchemaToProtoSpansAggregationDimensionAggregationType)
	DashboardValidSpansAggregationDimensionAggregationTypes        = utils.GetKeys(DashboardSchemaToProtoSpansAggregationDimensionAggregationType)
	DashboardSchemaToProtoSpanFieldMetadataField                   = map[string]cxsdk.SpanFieldMetadataFieldInner{
		UNSPECIFIED:        cxsdk.SpanFieldMetadataFieldUnspecified,
		"application_name": cxsdk.SpanFieldMetadataFieldApplicationName,
		"subsystem_name":   cxsdk.SpanFieldMetadataFieldSubsystemName,
		"service_name":     cxsdk.SpanFieldMetadataFieldServiceName,
		"operation_name":   cxsdk.SpanFieldMetadataFieldOperationName,
	}
	DashboardProtoToSchemaSpanFieldMetadataField = utils.ReverseMap(DashboardSchemaToProtoSpanFieldMetadataField)
	DashboardValidSpanFieldMetadataFields        = utils.GetKeys(DashboardSchemaToProtoSpanFieldMetadataField)
	DashboardSchemaToProtoSortBy                 = map[string]cxsdk.SortByType{
		UNSPECIFIED: cxsdk.SortByTypeUnspecified,
		"value":     cxsdk.SortByTypeValue,
		"name":      cxsdk.SortByTypeName,
	}
	DashboardProtoToSchemaSortBy                = utils.ReverseMap(DashboardSchemaToProtoSortBy)
	DashboardValidSortBy                        = utils.GetKeys(DashboardSchemaToProtoSortBy)
	DashboardSchemaToProtoObservationFieldScope = map[string]cxsdk.DatasetScope{
		UNSPECIFIED: cxsdk.DatasetScopeUnspecified,
		"user_data": cxsdk.DatasetScopeUserData,
		"label":     cxsdk.DatasetScopeLabel,
		"metadata":  cxsdk.DatasetScopeMetadata,
	}
	DashboardProtoToSchemaObservationFieldScope = utils.ReverseMap(DashboardSchemaToProtoObservationFieldScope)
	DashboardValidObservationFieldScope         = utils.GetKeys(DashboardSchemaToProtoObservationFieldScope)
	DashboardSchemaToProtoDataModeType          = map[string]cxsdk.DataModeType{
		UNSPECIFIED: cxsdk.DataModeTypeHighUnspecified,
		"archive":   cxsdk.DataModeTypeArchive,
	}
	DashboardProtoToSchemaDataModeType     = utils.ReverseMap(DashboardSchemaToProtoDataModeType)
	DashboardValidDataModeTypes            = utils.GetKeys(DashboardSchemaToProtoDataModeType)
	DashboardSchemaToProtoGaugeThresholdBy = map[string]cxsdk.GaugeThresholdBy{
		UNSPECIFIED:  cxsdk.GaugeThresholdByUnspecified,
		"value":      cxsdk.GaugeThresholdByValue,
		"background": cxsdk.GaugeThresholdByBackground,
	}
	DashboardProtoToSchemaGaugeThresholdBy = utils.ReverseMap(DashboardSchemaToProtoGaugeThresholdBy)
	DashboardValidGaugeThresholdBy         = utils.GetKeys(DashboardSchemaToProtoGaugeThresholdBy)
	DashboardSchemaToProtoRefreshStrategy  = map[string]cxsdk.MultiSelectRefreshStrategy{
		UNSPECIFIED:            cxsdk.MultiSelectRefreshStrategyUnspecified,
		"on_dashboard_load":    cxsdk.MultiSelectRefreshStrategyOnDashboardLoad,
		"on_time_frame_change": cxsdk.MultiSelectRefreshStrategyOnTimeFrameChange,
	}
	DashboardProtoToSchemaRefreshStrategy = utils.ReverseMap(DashboardSchemaToProtoRefreshStrategy)
	DashboardValidRefreshStrategies       = utils.GetKeys(DashboardSchemaToProtoRefreshStrategy)
	DashboardValidLogsAggregationTypes    = []string{"count", "count_distinct", "sum", "avg", "min", "max", "percentile"}
	DashboardValidSpanFieldTypes          = []string{"metadata", "tag", "process_tag"}
	DashboardValidSpanAggregationTypes    = []string{"metric", "dimension"}
	DashboardValidColorSchemes            = []string{"classic", "severity", "cold", "negative", "green", "red", "blue"}
	SectionValidColors                    = []string{UNSPECIFIED, "cyan", "green", "blue", "purple", "magenta", "pink", "orange"}

	DashboardSchemaToProtoThresholdType = map[string]cxsdk.ThresholdType{
		UNSPECIFIED: cxsdk.ThresholdTypeUnspecified,
		"absolute":  cxsdk.ThresholdTypeAbsolute,
		"relative":  cxsdk.ThresholdTypeRelative,
	}
	DashboardProtoToSchemaThresholdType = utils.ReverseMap(DashboardSchemaToProtoThresholdType)
	DashboardValidThresholdTypes        = utils.GetKeys(DashboardSchemaToProtoThresholdType)
	DashboardSchemaToProtoLegendBy      = map[string]cxsdk.LegendBy{
		UNSPECIFIED:  cxsdk.LegendByUnspecified,
		"thresholds": cxsdk.LegendByThresholds,
		"groups":     cxsdk.LegendByGroups,
	}
	DashboardProtoToSchemaLegendBy = utils.ReverseMap(DashboardSchemaToProtoLegendBy)
	DashboardValidLegendBys        = utils.GetKeys(DashboardSchemaToProtoLegendBy)

	DashboardSchemaToProtoPromQLQueryType = map[string]cxsdk.PromQLQueryType{
		UNSPECIFIED: cxsdk.PromQLQueryTypeUnspecified,
		"range":     cxsdk.PromQLQueryTypeRange,
		"instant":   cxsdk.PromQLQueryTypeInstant,
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
	PromqlQuery     types.String `tfsdk:"promql_query"`
	Filters         types.List   `tfsdk:"filters"` //MetricsFilterModel
	PromqlQueryType types.String `tfsdk:"promql_query_type"`
}

type MetricFilterModel struct {
	Metric   types.String         `tfsdk:"metric"`
	Label    types.String         `tfsdk:"label"`
	Operator *FilterOperatorModel `tfsdk:"operator"`
}

type QuerySpansModel struct {
	LuceneQuery  types.String `tfsdk:"lucene_query"`
	GroupBy      types.List   `tfsdk:"group_by"`     //SpansFieldModel
	Aggregations types.List   `tfsdk:"aggregations"` //SpansAggregationModel
	Filters      types.List   `tfsdk:"filters"`      //SpansFilterModel
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
	Query   types.String `tfsdk:"query"`
	Filters types.List   `tfsdk:"filters"` //DashboardFilterSourceModel
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
	Logs    *LineChartQueryLogsModel  `tfsdk:"logs"`
	Metrics *QueryMetricsModel        `tfsdk:"metrics"`
	Spans   *LineChartQuerySpansModel `tfsdk:"spans"`
}

type LineChartQueryLogsModel struct {
	LuceneQuery  types.String `tfsdk:"lucene_query"`
	GroupBy      types.List   `tfsdk:"group_by"`     //types.String
	Aggregations types.List   `tfsdk:"aggregations"` //AggregationModel
	Filters      types.List   `tfsdk:"filters"`      //FilterModel
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
	Timeframe   *TimeFrameModel                  `tfsdk:"time_frame"`
}

type LogsFilterModel struct {
	Field            types.String         `tfsdk:"field"`
	Operator         *FilterOperatorModel `tfsdk:"operator"`
	ObservationField types.Object         `tfsdk:"observation_field"` // ObservationFieldModel
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
	Absolute types.Object `tfsdk:"absolute"` //TimeFrameAbsoluteModel
	Relative types.Object `tfsdk:"relative"` //TimeFrameRelativeModel
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

func FlattenLegend(legend *cxsdk.DashboardLegend) *LegendModel {
	if legend == nil {
		return nil
	}

	return &LegendModel{
		IsVisible:    utils.WrapperspbBoolToTypeBool(legend.GetIsVisible()),
		GroupByQuery: utils.WrapperspbBoolToTypeBool(legend.GetGroupByQuery()),
		Columns:      flattenLegendColumns(legend.GetColumns()),
		Placement:    types.StringValue(DashboardLegendPlacementProtoToSchema[legend.GetPlacement()]),
	}
}

func flattenLegendColumns(columns []cxsdk.DashboardLegendColumn) types.List {
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

func ExpandLegend(ctx context.Context, legend *LegendModel) (*cxsdk.DashboardLegend, diag.Diagnostics) {
	if legend == nil {
		return nil, nil
	}

	columns := make([]cxsdk.DashboardLegendColumn, 0, len(legend.Columns.Elements()))
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

	return &cxsdk.DashboardLegend{
		IsVisible:    utils.TypeBoolToWrapperspbBool(legend.IsVisible),
		Columns:      columns,
		GroupByQuery: utils.TypeBoolToWrapperspbBool(legend.GroupByQuery),
		Placement:    DashboardLegendPlacementSchemaToProto[legend.Placement.ValueString()],
	}, nil
}

func FlattenSpansFields(ctx context.Context, spanFields []*cxsdk.SpanField) (types.List, diag.Diagnostics) {
	if len(spanFields) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: SpansFieldModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	spanFieldElements := make([]attr.Value, 0, len(spanFields))
	for _, field := range spanFields {
		flattenedField, dg := FlattenSpansField(field)
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

	return types.ListValueMust(types.ObjectType{AttrTypes: SpansFieldModelAttr()}, spanFieldElements), diagnostics
}

func FlattenSpansField(field *cxsdk.SpanField) (*SpansFieldModel, diag.Diagnostic) {
	if field == nil {
		return nil, nil
	}

	switch field.GetValue().(type) {
	case *cxsdk.SpanFieldMetadataField:
		return &SpansFieldModel{
			Type:  types.StringValue("metadata"),
			Value: types.StringValue(DashboardProtoToSchemaSpanFieldMetadataField[field.GetMetadataField()]),
		}, nil
	case *cxsdk.SpanFieldTagField:
		return &SpansFieldModel{
			Type:  types.StringValue("tag"),
			Value: utils.WrapperspbStringToTypeString(field.GetTagField()),
		}, nil
	case *cxsdk.SpanFieldProcessTagField:
		return &SpansFieldModel{
			Type:  types.StringValue("process_tag"),
			Value: utils.WrapperspbStringToTypeString(field.GetProcessTagField()),
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

func FlattenDashboardFiltersSources(ctx context.Context, sources []*cxsdk.DashboardFilterSource) (types.List, diag.Diagnostics) {
	if len(sources) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: FilterSourceModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	filtersElements := make([]attr.Value, 0, len(sources))
	for _, source := range sources {
		flattenedFilter, diags := FlattenDashboardFilterSource(ctx, source)
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

func FlattenDashboardFilterSource(ctx context.Context, source *cxsdk.DashboardFilterSource) (*DashboardFilterSourceModel, diag.Diagnostics) {
	if source == nil {
		return nil, nil
	}

	switch source.GetValue().(type) {
	case *cxsdk.DashboardFilterSourceLogs:
		logs, diags := FlattenDashboardFilterSourceLogs(ctx, source.GetLogs())
		if diags.HasError() {
			return nil, diags
		}
		return &DashboardFilterSourceModel{Logs: logs}, nil
	case *cxsdk.DashboardFilterSourceSpans:
		spans, dg := FlattenDashboardFilterSourceSpans(source.GetSpans())
		if dg != nil {
			return nil, diag.Diagnostics{dg}
		}
		return &DashboardFilterSourceModel{Spans: spans}, nil
	case *cxsdk.DashboardFilterSourceMetrics:
		metrics, dg := FlattenDashboardFilterSourceMetrics(source.GetMetrics())
		if dg != nil {
			return nil, diag.Diagnostics{dg}
		}
		return &DashboardFilterSourceModel{Metrics: metrics}, nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Dashboard Filter Source", fmt.Sprintf("unknown filter source type %T", source))}
	}
}

func FlattenDashboardFilterSourceLogs(ctx context.Context, logs *cxsdk.DashboardFilterLogsFilter) (*FilterSourceLogsModel, diag.Diagnostics) {
	if logs == nil {
		return nil, nil
	}

	operator, dg := FlattenFilterOperator(logs.GetOperator())
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	observationField, diags := FlattenObservationField(ctx, logs.GetObservationField())
	if diags.HasError() {
		return nil, diags
	}

	return &FilterSourceLogsModel{
		Field:            utils.WrapperspbStringToTypeString(logs.GetField()),
		Operator:         operator,
		ObservationField: observationField,
	}, nil
}

func FlattenDashboardFilterSourceSpans(spans *cxsdk.DashboardFilterSpansFilter) (*FilterSourceSpansModel, diag.Diagnostic) {
	if spans == nil {
		return nil, nil
	}

	field, dg := FlattenSpansField(spans.GetField())
	if dg != nil {
		return nil, dg
	}

	operator, dg := FlattenFilterOperator(spans.GetOperator())
	if dg != nil {
		return nil, dg
	}

	return &FilterSourceSpansModel{
		Field:    field,
		Operator: operator,
	}, nil
}

func FlattenDashboardFilterSourceMetrics(metrics *cxsdk.DashboardFilterMetricsFilter) (*FilterSourceMetricsModel, diag.Diagnostic) {
	if metrics == nil {
		return nil, nil
	}

	operator, dg := FlattenFilterOperator(metrics.GetOperator())
	if dg != nil {
		return nil, dg
	}

	return &FilterSourceMetricsModel{
		MetricName:  utils.WrapperspbStringToTypeString(metrics.GetMetric()),
		MetricLabel: utils.WrapperspbStringToTypeString(metrics.GetLabel()),
		Operator:    operator,
	}, nil
}

func FlattenDashboardTimeFrame(ctx context.Context, d *cxsdk.Dashboard) (*TimeFrameModel, diag.Diagnostics) {
	if d.GetTimeFrame() == nil {
		return nil, nil
	}
	switch timeFrameType := d.GetTimeFrame().(type) {
	case *cxsdk.DashboardAbsoluteTimeFrame:
		return flattenAbsoluteTimeFrame(ctx, timeFrameType.AbsoluteTimeFrame)
	case *cxsdk.DashboardRelativeTimeFrame:
		return flattenRelativeTimeFrame(ctx, timeFrameType.RelativeTimeFrame)
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Dashboard Time Frame", fmt.Sprintf("unknown time frame type %T", timeFrameType))}
	}
}

func FlattenTimeFrameSelect(ctx context.Context, d *cxsdk.TimeframeSelect) (*TimeFrameModel, diag.Diagnostics) {
	if d.GetValue() == nil {
		return nil, nil
	}
	switch timeFrameType := d.GetValue().(type) {
	case *cxsdk.TimeframeSelectAbsolute:
		return flattenAbsoluteTimeFrame(ctx, timeFrameType.AbsoluteTimeFrame)
	case *cxsdk.TimeframeSelectRelative:
		return flattenRelativeTimeFrame(ctx, timeFrameType.RelativeTimeFrame)
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Dashboard Time Frame", fmt.Sprintf("unknown time frame type %T", timeFrameType))}
	}
}

func FlattenObservationField(ctx context.Context, field *cxsdk.ObservationField) (types.Object, diag.Diagnostics) {
	if field == nil {
		return types.ObjectNull(ObservationFieldAttr()), nil
	}

	return types.ObjectValueFrom(ctx, ObservationFieldAttr(), FlattenLogsFieldModel(field))
}

func FlattenLogsFieldModel(field *cxsdk.ObservationField) *ObservationFieldModel {
	return &ObservationFieldModel{
		Keypath: utils.WrappedStringSliceToTypeStringList(field.GetKeypath()),
		Scope:   types.StringValue(DashboardProtoToSchemaObservationFieldScope[field.GetScope()]),
	}
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

func flattenAbsoluteTimeFrame(ctx context.Context, timeFrame *cxsdk.DashboardTimeFrame) (*TimeFrameModel, diag.Diagnostics) {
	absoluteTimeFrame := &TimeFrameAbsoluteModel{
		Start: types.StringValue(timeFrame.GetFrom().String()),
		End:   types.StringValue(timeFrame.GetTo().String()),
	}

	timeFrameObject, dgs := types.ObjectValueFrom(ctx, AbsoluteTimeFrameAttr(), absoluteTimeFrame)
	if dgs.HasError() {
		return nil, dgs
	}
	flattenedTimeFrame := &TimeFrameModel{
		Absolute: timeFrameObject,
		Relative: types.ObjectNull(RelativeTimeFrameAttr()),
	}
	return flattenedTimeFrame, nil
}

func flattenRelativeTimeFrame(ctx context.Context, timeFrame *durationpb.Duration) (*TimeFrameModel, diag.Diagnostics) {
	relativeTimeFrame := &TimeFrameRelativeModel{
		Duration: flattenDuration(timeFrame),
	}
	timeFrameObject, dgs := types.ObjectValueFrom(ctx, RelativeTimeFrameAttr(), relativeTimeFrame)
	if dgs.HasError() {
		return nil, dgs
	}
	flattenedTimeFrame := &TimeFrameModel{
		Relative: timeFrameObject,
		Absolute: types.ObjectNull(AbsoluteTimeFrameAttr()),
	}
	return flattenedTimeFrame, nil
}

func FlattenSpansFilters(ctx context.Context, filters []*cxsdk.DashboardFilterSpansFilter) (types.List, diag.Diagnostics) {
	if len(filters) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: SpansFilterModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	filtersElements := make([]attr.Value, 0, len(filters))
	for _, filter := range filters {
		flattenedFilter, dg := FlattenSpansFilter(filter)
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

func FlattenSpansFilter(filter *cxsdk.DashboardFilterSpansFilter) (*SpansFilterModel, diag.Diagnostic) {
	if filter == nil {
		return nil, nil
	}

	operator, dg := FlattenFilterOperator(filter.GetOperator())
	if dg != nil {
		return nil, dg
	}

	field, dg := FlattenSpansField(filter.GetField())
	if dg != nil {
		return nil, dg
	}

	return &SpansFilterModel{
		Field:    field,
		Operator: operator,
	}, nil
}

func FlattenFilterOperator(operator *cxsdk.DashboardFilterOperator) (*FilterOperatorModel, diag.Diagnostic) {
	switch operator.GetValue().(type) {
	case *cxsdk.DashboardFilterOperatorEquals:
		switch operator.GetEquals().GetSelection().GetValue().(type) {
		case *cxsdk.DashboardFilterEqualsSelectionAll:
			return &FilterOperatorModel{
				Type:           types.StringValue("equals"),
				SelectedValues: types.ListNull(types.StringType),
			}, nil
		case *cxsdk.DashboardFilterEqualsSelectionList:
			return &FilterOperatorModel{
				Type:           types.StringValue("equals"),
				SelectedValues: utils.WrappedStringSliceToTypeStringList(operator.GetEquals().GetSelection().GetList().GetValues()),
			}, nil
		default:
			return nil, diag.NewErrorDiagnostic("Error Flatten Logs Filter Operator Equals", "unknown logs filter operator equals selection type")
		}
	case *cxsdk.DashboardFilterOperatorNotEquals:
		switch operator.GetNotEquals().GetSelection().GetValue().(type) {
		case *cxsdk.DashboardFilterNotEqualsSelectionList:
			return &FilterOperatorModel{
				Type:           types.StringValue("not_equals"),
				SelectedValues: utils.WrappedStringSliceToTypeStringList(operator.GetNotEquals().GetSelection().GetList().GetValues()),
			}, nil
		default:
			return nil, diag.NewErrorDiagnostic("Error Flatten Logs Filter Operator NotEquals", "unknown logs filter operator not_equals selection type")
		}
	default:
		return nil, diag.NewErrorDiagnostic("Error Flatten Logs Filter Operator", "unknown logs filter operator type")
	}
}

func FlattenMetricsFilters(ctx context.Context, filters []*cxsdk.DashboardFilterMetricsFilter) (types.List, diag.Diagnostics) {
	if len(filters) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: MetricsFilterModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	filtersElements := make([]attr.Value, 0, len(filters))
	for _, filter := range filters {
		flattenedFilter, dg := FlattenMetricsFilter(filter)
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

func FlattenMetricsFilter(filter *cxsdk.DashboardFilterMetricsFilter) (*MetricsFilterModel, diag.Diagnostic) {
	if filter == nil {
		return nil, nil
	}

	operator, dg := FlattenFilterOperator(filter.GetOperator())
	if dg != nil {
		return nil, dg
	}

	return &MetricsFilterModel{
		Metric:   utils.WrapperspbStringToTypeString(filter.GetMetric()),
		Label:    utils.WrapperspbStringToTypeString(filter.GetLabel()),
		Operator: operator,
	}, nil
}

func FlattenLogsFilters(ctx context.Context, filters []*cxsdk.DashboardFilterLogsFilter) (types.List, diag.Diagnostics) {
	if len(filters) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: LogsFilterModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	filtersElements := make([]attr.Value, 0, len(filters))
	for _, filter := range filters {
		flattenedFilter, diags := flattenLogsFilter(ctx, filter)
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

func flattenLogsFilter(ctx context.Context, filter *cxsdk.DashboardFilterLogsFilter) (*LogsFilterModel, diag.Diagnostics) {
	if filter == nil {
		return nil, nil
	}

	operator, dg := FlattenFilterOperator(filter.GetOperator())
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	observationField, diags := FlattenObservationField(ctx, filter.GetObservationField())
	if diags.HasError() {
		return nil, diags
	}

	return &LogsFilterModel{
		Field:            utils.WrapperspbStringToTypeString(filter.GetField()),
		Operator:         operator,
		ObservationField: observationField,
	}, nil
}

func FlattenObservationFields(ctx context.Context, namesFields []*cxsdk.ObservationField) (types.List, diag.Diagnostics) {
	if len(namesFields) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: ObservationFieldAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	fieldElements := make([]attr.Value, 0, len(namesFields))
	for _, field := range namesFields {
		flattenedField, diags := FlattenObservationField(ctx, field)
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

func FlattenLogsAggregation(ctx context.Context, aggregation *cxsdk.LogsAggregation) (*LogsAggregationModel, diag.Diagnostics) {
	if aggregation == nil {
		return nil, nil
	}

	switch aggregationValue := aggregation.GetValue().(type) {
	case *cxsdk.LogsAggregationCount:
		return &LogsAggregationModel{
			Type:             types.StringValue("count"),
			ObservationField: types.ObjectNull(ObservationFieldAttr()),
		}, nil
	case *cxsdk.LogsAggregationCountDistinct:
		observationField, diags := FlattenObservationField(ctx, aggregationValue.CountDistinct.GetObservationField())
		if diags.HasError() {
			return nil, diags
		}
		return &LogsAggregationModel{
			Type:             types.StringValue("count_distinct"),
			Field:            utils.WrapperspbStringToTypeString(aggregationValue.CountDistinct.GetField()),
			ObservationField: observationField,
		}, nil
	case *cxsdk.LogsAggregationSum:
		observationField, diags := FlattenObservationField(ctx, aggregationValue.Sum.GetObservationField())
		if diags.HasError() {
			return nil, diags
		}
		return &LogsAggregationModel{
			Type:             types.StringValue("sum"),
			Field:            utils.WrapperspbStringToTypeString(aggregationValue.Sum.GetField()),
			ObservationField: observationField,
		}, nil
	case *cxsdk.LogsAggregationAverage:
		observationField, diags := FlattenObservationField(ctx, aggregationValue.Average.GetObservationField())
		if diags.HasError() {
			return nil, diags
		}
		return &LogsAggregationModel{
			Type:             types.StringValue("avg"),
			Field:            utils.WrapperspbStringToTypeString(aggregationValue.Average.GetField()),
			ObservationField: observationField,
		}, nil
	case *cxsdk.LogsAggregationMin:
		observationField, diags := FlattenObservationField(ctx, aggregationValue.Min.GetObservationField())
		if diags.HasError() {
			return nil, diags
		}
		return &LogsAggregationModel{
			Type:             types.StringValue("min"),
			Field:            utils.WrapperspbStringToTypeString(aggregationValue.Min.GetField()),
			ObservationField: observationField,
		}, nil
	case *cxsdk.LogsAggregationMax:
		observationField, diags := FlattenObservationField(ctx, aggregationValue.Max.GetObservationField())
		if diags.HasError() {
			return nil, diags
		}
		return &LogsAggregationModel{
			Type:             types.StringValue("max"),
			Field:            utils.WrapperspbStringToTypeString(aggregationValue.Max.GetField()),
			ObservationField: observationField,
		}, nil
	case *cxsdk.LogsAggregationPercentile:
		observationField, diags := FlattenObservationField(ctx, aggregationValue.Percentile.GetObservationField())
		if diags.HasError() {
			return nil, diags
		}
		return &LogsAggregationModel{
			Type:             types.StringValue("percentile"),
			Field:            utils.WrapperspbStringToTypeString(aggregationValue.Percentile.GetField()),
			Percent:          utils.WrapperspbDoubleToTypeFloat64(aggregationValue.Percentile.GetPercent()),
			ObservationField: observationField,
		}, nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Logs Aggregation", "unknown logs aggregation type")}
	}
}

func ExpandObservationFields(ctx context.Context, namesFields types.List) ([]*cxsdk.ObservationField, diag.Diagnostics) {
	var namesFieldsObjects []types.Object
	var expandedNamesFields []*cxsdk.ObservationField
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

func ExpandObservationFieldObject(ctx context.Context, field types.Object) (*cxsdk.ObservationField, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(field) {
		return nil, nil
	}

	var observationField ObservationFieldModel
	if dg := field.As(ctx, &observationField, basetypes.ObjectAsOptions{}); dg.HasError() {
		return nil, dg
	}

	return expandObservationField(ctx, observationField)
}

func expandObservationField(ctx context.Context, observationField ObservationFieldModel) (*cxsdk.ObservationField, diag.Diagnostics) {
	keypath, dg := utils.TypeStringSliceToWrappedStringSlice(ctx, observationField.Keypath.Elements())
	if dg.HasError() {
		return nil, dg
	}

	scope := DashboardSchemaToProtoObservationFieldScope[observationField.Scope.ValueString()]

	return &cxsdk.ObservationField{
		Keypath: keypath,
		Scope:   scope,
	}, nil
}

func ExpandSpansField(spansFilterField *SpansFieldModel) (*cxsdk.SpanField, diag.Diagnostic) {
	if spansFilterField == nil {
		return nil, nil
	}

	switch spansFilterField.Type.ValueString() {
	case "metadata":
		return &cxsdk.SpanField{
			Value: &cxsdk.SpanFieldMetadataField{
				MetadataField: DashboardSchemaToProtoSpanFieldMetadataField[spansFilterField.Value.ValueString()],
			},
		}, nil
	case "tag":
		return &cxsdk.SpanField{
			Value: &cxsdk.SpanFieldTagField{
				TagField: utils.TypeStringToWrapperspbString(spansFilterField.Value),
			},
		}, nil
	case "process_tag":
		return &cxsdk.SpanField{
			Value: &cxsdk.SpanFieldProcessTagField{
				ProcessTagField: utils.TypeStringToWrapperspbString(spansFilterField.Value),
			},
		}, nil
	default:
		return nil, diag.NewErrorDiagnostic("Extract Spans Filter Field Error", fmt.Sprintf("Unknown spans filter field type %s", spansFilterField.Type.ValueString()))
	}
}

func ExpandSpansFields(ctx context.Context, spanFields types.List) ([]*cxsdk.SpanField, diag.Diagnostics) {
	var spanFieldsObjects []types.Object
	var expandedSpanFields []*cxsdk.SpanField
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
		expandedSpanFields = append(expandedSpanFields, expandedSpanField)
	}

	return expandedSpanFields, diags
}

func ExpandLogsAggregations(ctx context.Context, logsAggregations types.List) ([]*cxsdk.LogsAggregation, diag.Diagnostics) {
	var logsAggregationsObjects []types.Object
	var expandedLogsAggregations []*cxsdk.LogsAggregation
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
		expandedLogsAggregations = append(expandedLogsAggregations, expandedLogsAggregation)
	}

	return expandedLogsAggregations, diags
}

func ExpandLogsAggregation(ctx context.Context, logsAggregation *LogsAggregationModel) (*cxsdk.LogsAggregation, diag.Diagnostics) {
	if logsAggregation == nil {
		return nil, nil
	}
	switch logsAggregation.Type.ValueString() {
	case "count":
		return &cxsdk.LogsAggregation{
			Value: &cxsdk.LogsAggregationCount{
				Count: &cxsdk.LogsAggregationCountInner{},
			},
		}, nil
	case "count_distinct":
		observationField, diags := ExpandObservationFieldObject(ctx, logsAggregation.ObservationField)
		if diags.HasError() {
			return nil, diags
		}
		return &cxsdk.LogsAggregation{
			Value: &cxsdk.LogsAggregationCountDistinct{
				CountDistinct: &cxsdk.LogsAggregationCountDistinctInner{
					Field:            utils.TypeStringToWrapperspbString(logsAggregation.Field),
					ObservationField: observationField,
				},
			},
		}, nil
	case "sum":
		observationField, diags := ExpandObservationFieldObject(ctx, logsAggregation.ObservationField)
		if diags.HasError() {
			return nil, diags
		}
		return &cxsdk.LogsAggregation{
			Value: &cxsdk.LogsAggregationSum{
				Sum: &cxsdk.LogsAggregationSumInner{
					Field:            utils.TypeStringToWrapperspbString(logsAggregation.Field),
					ObservationField: observationField,
				},
			},
		}, nil
	case "avg":
		observationField, diags := ExpandObservationFieldObject(ctx, logsAggregation.ObservationField)
		if diags.HasError() {
			return nil, diags
		}
		return &cxsdk.LogsAggregation{
			Value: &cxsdk.LogsAggregationAverage{
				Average: &cxsdk.LogsAggregationAverageInner{
					Field:            utils.TypeStringToWrapperspbString(logsAggregation.Field),
					ObservationField: observationField,
				},
			},
		}, nil
	case "min":
		observationField, diags := ExpandObservationFieldObject(ctx, logsAggregation.ObservationField)
		if diags.HasError() {
			return nil, diags
		}
		return &cxsdk.LogsAggregation{
			Value: &cxsdk.LogsAggregationMin{
				Min: &cxsdk.LogsAggregationMinInner{
					Field:            utils.TypeStringToWrapperspbString(logsAggregation.Field),
					ObservationField: observationField,
				},
			},
		}, nil
	case "max":
		observationField, diags := ExpandObservationFieldObject(ctx, logsAggregation.ObservationField)
		if diags.HasError() {
			return nil, diags
		}
		return &cxsdk.LogsAggregation{
			Value: &cxsdk.LogsAggregationMax{
				Max: &cxsdk.LogsAggregationMaxInner{
					Field:            utils.TypeStringToWrapperspbString(logsAggregation.Field),
					ObservationField: observationField,
				},
			},
		}, nil
	case "percentile":
		observationField, diags := ExpandObservationFieldObject(ctx, logsAggregation.ObservationField)
		if diags.HasError() {
			return nil, diags
		}
		return &cxsdk.LogsAggregation{
			Value: &cxsdk.LogsAggregationPercentile{
				Percentile: &cxsdk.LogsAggregationPercentileInner{
					Field:            utils.TypeStringToWrapperspbString(logsAggregation.Field),
					Percent:          utils.TypeFloat64ToWrapperspbDouble(logsAggregation.Percent),
					ObservationField: observationField,
				},
			},
		}, nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error expand logs aggregation", fmt.Sprintf("unknown logs aggregation type %s", logsAggregation.Type.ValueString()))}
	}
}

func ExpandTimeFrameSelect(ctx context.Context, timeFrame *TimeFrameModel) (*cxsdk.TimeframeSelect, diag.Diagnostics) {
	if timeFrame == nil {
		return nil, nil
	}

	tf := cxsdk.TimeframeSelect{}

	switch {
	case !utils.ObjIsNullOrUnknown(timeFrame.Relative):
		val, diags := expandRelativeTimeFrame(ctx, timeFrame.Relative)
		if diags.HasError() {
			return nil, diags
		}
		tf.Value = &cxsdk.TimeframeSelectRelative{
			RelativeTimeFrame: val,
		}
	case !utils.ObjIsNullOrUnknown(timeFrame.Absolute):
		from, to, diags := expandAbsoluteTimeFrame(ctx, timeFrame.Absolute)
		if diags.HasError() {
			return nil, diags
		}
		tf.Value = &cxsdk.TimeframeSelectAbsolute{
			AbsoluteTimeFrame: &cxsdk.DashboardTimeFrame{
				From: from,
				To:   to,
			},
		}
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand Time Frame", "Dashboard TimeFrame must be either Relative or Absolute")}
	}
	return &tf, nil
}

func ExpandDashboardTimeFrame(ctx context.Context, dashboard *cxsdk.Dashboard, timeFrame *TimeFrameModel) (*cxsdk.Dashboard, diag.Diagnostics) {
	var diags diag.Diagnostics
	switch {
	case !utils.ObjIsNullOrUnknown(timeFrame.Relative):
		relative, diags := expandRelativeTimeFrame(ctx, timeFrame.Relative)
		if diags.HasError() {
			return nil, diags
		}
		dashboard.TimeFrame = &cxsdk.DashboardRelativeTimeFrame{
			RelativeTimeFrame: relative,
		}
	case !utils.ObjIsNullOrUnknown(timeFrame.Absolute):
		from, to, diags := expandAbsoluteTimeFrame(ctx, timeFrame.Absolute)
		if diags.HasError() {
			return nil, diags
		}
		dashboard.TimeFrame = &cxsdk.DashboardAbsoluteTimeFrame{
			AbsoluteTimeFrame: &cxsdk.DashboardTimeFrame{
				From: from,
				To:   to,
			},
		}
	default:
		diags = diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand Time Frame", "Dashboard TimeFrame must be either Relative or Absolute")}
	}
	return dashboard, diags
}

func expandRelativeTimeFrame(ctx context.Context, timeFrame types.Object) (*durationpb.Duration, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(timeFrame) {
		return nil, nil
	}
	timeFrameModel := &TimeFrameRelativeModel{}
	dgs := timeFrame.As(ctx, timeFrameModel, basetypes.ObjectAsOptions{})
	if dgs.HasError() {
		return nil, dgs
	}
	duration, dg := utils.ParseDuration(timeFrameModel.Duration.ValueString(), "Relative Dashboard Time Frame")
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}
	return durationpb.New(*duration), nil
}

func expandAbsoluteTimeFrame(ctx context.Context, timeFrame types.Object) (*timestamppb.Timestamp, *timestamppb.Timestamp, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(timeFrame) {
		return nil, nil, nil
	}

	timeFrameModel := &TimeFrameAbsoluteModel{}
	dgs := timeFrame.As(ctx, timeFrameModel, basetypes.ObjectAsOptions{})
	if dgs.HasError() {
		return nil, nil, dgs
	}
	fromTime, err := time.Parse(time.RFC3339, timeFrameModel.Start.ValueString())
	if err != nil {
		return nil, nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand Absolute Dashboard Time Frame", fmt.Sprintf("Error parsing from time: %s", err.Error()))}
	}
	from := timestamppb.New(fromTime)

	toTime, err := time.Parse(time.RFC3339, timeFrameModel.End.ValueString())
	if err != nil {
		return from, nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand Absolute Dashboard Time Frame", fmt.Sprintf("Error parsing from time: %s", err.Error()))}
	}
	to := timestamppb.New(toTime)

	return from, to, nil
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
