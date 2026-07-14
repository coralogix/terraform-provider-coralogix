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
	"time"

	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

var (
	ProtoDashboardSchemaToProtoUnit = map[string]cxsdk.Unit{
		utils.UNSPECIFIED: cxsdk.UnitUnspecified,
		"microseconds":    cxsdk.UnitMicroseconds,
		"milliseconds":    cxsdk.UnitMilliseconds,
		"nanoseconds":     cxsdk.UnitNanoseconds,
		"seconds":         cxsdk.UnitSeconds,
		"bytes":           cxsdk.UnitBytes,
		"kbytes":          cxsdk.UnitKbytes,
		"mbytes":          cxsdk.UnitMbytes,
		"gbytes":          cxsdk.UnitGbytes,
		"bytes_iec":       cxsdk.UnitBytesIec,
		"kibytes":         cxsdk.UnitKibytes,
		"mibytes":         cxsdk.UnitMibytes,
		"gibytes":         cxsdk.UnitGibytes,
		"euro_cents":      cxsdk.UnitEurCents,
		"euro":            cxsdk.UnitEur,
		"usd_cents":       cxsdk.UnitUsdCents,
		"usd":             cxsdk.UnitUsd,
		"custom":          cxsdk.UnitCustom,
		"percent01":       cxsdk.UnitPercent01,
		"percent100":      cxsdk.UnitPercent100,
	}
	ProtoDashboardProtoToSchemaUnit = utils.ReverseMap(ProtoDashboardSchemaToProtoUnit)
	ProtoDashboardValidUnits        = utils.GetKeys(ProtoDashboardSchemaToProtoUnit)

	ProtoDashboardLegendPlacementSchemaToProto = map[string]cxsdk.LegendPlacement{
		utils.UNSPECIFIED: cxsdk.LegendPlacementUnspecified,
		"auto":            cxsdk.LegendPlacementAuto,
		"bottom":          cxsdk.LegendPlacementBottom,
		"side":            cxsdk.LegendPlacementSide,
		"hidden":          cxsdk.LegendPlacementHidden,
	}
	ProtoDashboardLegendPlacementProtoToSchema = utils.ReverseMap(ProtoDashboardLegendPlacementSchemaToProto)
	ProtoDashboardValidLegendPlacements        = utils.GetKeys(ProtoDashboardLegendPlacementSchemaToProto)

	ProtoDashboardRowStyleSchemaToProto = map[string]cxsdk.RowStyle{
		utils.UNSPECIFIED: cxsdk.RowStyleUnspecified,
		"one_line":        cxsdk.RowStyleOneLine,
		"two_line":        cxsdk.RowStyleTwoLine,
		"condensed":       cxsdk.RowStyleCondensed,
		"json":            cxsdk.RowStyleJSON,
		"list":            cxsdk.RowStyleList,
	}
	ProtoDashboardRowStyleProtoToSchema     = utils.ReverseMap(ProtoDashboardRowStyleSchemaToProto)
	ProtoDashboardValidRowStyles            = utils.GetKeys(ProtoDashboardRowStyleSchemaToProto)
	ProtoDashboardLegendColumnSchemaToProto = map[string]cxsdk.DashboardLegendColumn{
		utils.UNSPECIFIED: cxsdk.LegendColumnUnspecified,
		"min":             cxsdk.LegendColumnMin,
		"max":             cxsdk.LegendColumnMax,
		"sum":             cxsdk.LegendColumnSum,
		"avg":             cxsdk.LegendColumnAvg,
		"last":            cxsdk.LegendColumnLast,
		"name":            cxsdk.LegendColumnName,
	}
	ProtoDashboardLegendColumnProtoToSchema   = utils.ReverseMap(ProtoDashboardLegendColumnSchemaToProto)
	ProtoDashboardValidLegendColumns          = utils.GetKeys(ProtoDashboardLegendColumnSchemaToProto)
	ProtoDashboardOrderDirectionSchemaToProto = map[string]cxsdk.OrderDirection{
		utils.UNSPECIFIED: cxsdk.OrderDirectionUnspecified,
		"asc":             cxsdk.OrderDirectionAsc,
		"desc":            cxsdk.OrderDirectionDesc,
	}
	ProtoDashboardOrderDirectionProtoToSchema = utils.ReverseMap(ProtoDashboardOrderDirectionSchemaToProto)
	ProtoDashboardValidOrderDirections        = utils.GetKeys(ProtoDashboardOrderDirectionSchemaToProto)

	ProtoDashboardValidMultiSelectSelectionTypes = []string{
		"multi",
		"single",
	}
	ProtoDashboardSchemaToProtoTooltipType = map[string]cxsdk.LineChartTooltipType{
		utils.UNSPECIFIED: cxsdk.LineChartToolTipTypeUnspecified,
		"all":             cxsdk.LineChartToolTipTypeAll,
		"single":          cxsdk.LineChartToolTipTypeSingle,
	}
	ProtoDashboardProtoToSchemaTooltipType = utils.ReverseMap(ProtoDashboardSchemaToProtoTooltipType)
	ProtoDashboardValidTooltipTypes        = utils.GetKeys(ProtoDashboardSchemaToProtoTooltipType)
	ProtoDashboardSchemaToProtoScaleType   = map[string]cxsdk.ScaleType{
		utils.UNSPECIFIED: cxsdk.ScaleTypeUnspecified,
		"linear":          cxsdk.ScaleTypeLinear,
		"logarithmic":     cxsdk.ScaleTypeLogarithmic,
	}
	ProtoDashboardProtoToSchemaScaleType = utils.ReverseMap(ProtoDashboardSchemaToProtoScaleType)
	ProtoDashboardValidScaleTypes        = utils.GetKeys(ProtoDashboardSchemaToProtoScaleType)

	ProtoDashboardSchemaToProtoGaugeUnit = map[string]cxsdk.GaugeUnit{
		utils.UNSPECIFIED: cxsdk.GaugeUnitUnspecified,
		"none":            cxsdk.GaugeUnitNumber,
		"percent":         cxsdk.GaugeUnitPercent,
		"microseconds":    cxsdk.GaugeUnitMicroseconds,
		"milliseconds":    cxsdk.GaugeUnitMilliseconds,
		"nanoseconds":     cxsdk.GaugeUnitNanoseconds,
		"seconds":         cxsdk.GaugeUnitSeconds,
		"bytes":           cxsdk.GaugeUnitBytes,
		"kbytes":          cxsdk.GaugeUnitKbytes,
		"mbytes":          cxsdk.GaugeUnitMbytes,
		"gbytes":          cxsdk.GaugeUnitGbytes,
		"bytes_iec":       cxsdk.GaugeUnitBytesIec,
		"kibytes":         cxsdk.GaugeUnitKibytes,
		"mibytes":         cxsdk.GaugeUnitMibytes,
		"gibytes":         cxsdk.GaugeUnitGibytes,
		"euro_cents":      cxsdk.GaugeUnitEurCents,
		"euro":            cxsdk.GaugeUnitEur,
		"usd_cents":       cxsdk.GaugeUnitUsdCents,
		"usd":             cxsdk.GaugeUnitUsd,
		"custom":          cxsdk.GaugeUnitCustom,
		"percent01":       cxsdk.GaugeUnitPercent01,
		"percent100":      cxsdk.GaugeUnitPercent100,
	}
	ProtoDashboardProtoToSchemaGaugeUnit           = utils.ReverseMap(ProtoDashboardSchemaToProtoGaugeUnit)
	ProtoDashboardValidGaugeUnits                  = utils.GetKeys(ProtoDashboardSchemaToProtoGaugeUnit)
	ProtoDashboardSchemaToProtoPieChartLabelSource = map[string]cxsdk.PieChartLabelSource{
		utils.UNSPECIFIED: cxsdk.PieChartLabelSourceUnspecified,
		"inner":           cxsdk.PieChartLabelSourceInner,
		"stack":           cxsdk.PieChartLabelSourceStack,
	}
	ProtoDashboardProtoToSchemaPieChartLabelSource = utils.ReverseMap(ProtoDashboardSchemaToProtoPieChartLabelSource)
	ProtoDashboardValidPieChartLabelSources        = utils.GetKeys(ProtoDashboardSchemaToProtoPieChartLabelSource)
	ProtoDashboardSchemaToProtoGaugeAggregation    = map[string]cxsdk.GaugeAggregation{
		utils.UNSPECIFIED: cxsdk.GaugeAggregationUnspecified,
		"last":            cxsdk.GaugeAggregationLast,
		"min":             cxsdk.GaugeAggregationMin,
		"max":             cxsdk.GaugeAggregationMax,
		"avg":             cxsdk.GaugeAggregationAvg,
		"sum":             cxsdk.GaugeAggregationSum,
	}
	ProtoDashboardProtoToSchemaGaugeAggregation            = utils.ReverseMap(ProtoDashboardSchemaToProtoGaugeAggregation)
	ProtoDashboardValidGaugeAggregations                   = utils.GetKeys(ProtoDashboardSchemaToProtoGaugeAggregation)
	ProtoDashboardSchemaToProtoSpansAggregationMetricField = map[string]cxsdk.SpansAggregationMetricAggregationMetricField{
		utils.UNSPECIFIED: cxsdk.SpansAggregationMetricAggregationMetricFieldUnspecified,
		"duration":        cxsdk.SpansAggregationMetricAggregationMetricFieldDuration,
	}
	ProtoDashboardProtoToSchemaSpansAggregationMetricField           = utils.ReverseMap(ProtoDashboardSchemaToProtoSpansAggregationMetricField)
	ProtoDashboardValidSpansAggregationMetricFields                  = utils.GetKeys(ProtoDashboardSchemaToProtoSpansAggregationMetricField)
	ProtoDashboardSchemaToProtoSpansAggregationMetricAggregationType = map[string]cxsdk.SpansAggregationMetricAggregationMetricAggregationType{
		utils.UNSPECIFIED: cxsdk.SpansAggregationMetricAggregationMetricTypeUnspecified,
		"min":             cxsdk.SpansAggregationMetricAggregationMetricTypeMin,
		"max":             cxsdk.SpansAggregationMetricAggregationMetricTypeMax,
		"avg":             cxsdk.SpansAggregationMetricAggregationMetricTypeAverage,
		"sum":             cxsdk.SpansAggregationMetricAggregationMetricTypeSum,
		"percentile_99":   cxsdk.SpansAggregationMetricAggregationMetricTypePercentile99,
		"percentile_95":   cxsdk.SpansAggregationMetricAggregationMetricTypePercentile95,
		"percentile_50":   cxsdk.SpansAggregationMetricAggregationMetricTypePercentile50,
	}
	ProtoDashboardProtoToSchemaSpansAggregationMetricAggregationType = utils.ReverseMap(ProtoDashboardSchemaToProtoSpansAggregationMetricAggregationType)
	ProtoDashboardValidSpansAggregationMetricAggregationTypes        = utils.GetKeys(ProtoDashboardSchemaToProtoSpansAggregationMetricAggregationType)
	ProtoDashboardProtoToSchemaSpansAggregationDimensionField        = map[string]cxsdk.SpansAggregationDimensionAggregationDimensionField{
		utils.UNSPECIFIED: cxsdk.SpansAggregationDimensionAggregationDimensionFieldUnspecified,
		"trace_id":        cxsdk.SpansAggregationDimensionAggregationDimensionFieldTraceID,
	}
	ProtoDashboardSchemaToProtoSpansAggregationDimensionField           = utils.ReverseMap(ProtoDashboardProtoToSchemaSpansAggregationDimensionField)
	ProtoDashboardValidSpansAggregationDimensionFields                  = utils.GetKeys(ProtoDashboardProtoToSchemaSpansAggregationDimensionField)
	ProtoDashboardSchemaToProtoSpansAggregationDimensionAggregationType = map[string]cxsdk.SpansAggregationDimensionAggregationType{
		utils.UNSPECIFIED: cxsdk.SpansAggregationDimensionAggregationTypeUnspecified,
		"unique_count":    cxsdk.SpansAggregationDimensionAggregationTypeUniqueCount,
		"error_count":     cxsdk.SpansAggregationDimensionAggregationTypeErrorCount,
	}
	ProtoDashboardProtoToSchemaSpansAggregationDimensionAggregationType = utils.ReverseMap(ProtoDashboardSchemaToProtoSpansAggregationDimensionAggregationType)
	ProtoDashboardValidSpansAggregationDimensionAggregationTypes        = utils.GetKeys(ProtoDashboardSchemaToProtoSpansAggregationDimensionAggregationType)
	ProtoDashboardSchemaToProtoSpanFieldMetadataField                   = map[string]cxsdk.SpanFieldMetadataFieldInner{
		utils.UNSPECIFIED:  cxsdk.SpanFieldMetadataFieldUnspecified,
		"application_name": cxsdk.SpanFieldMetadataFieldApplicationName,
		"subsystem_name":   cxsdk.SpanFieldMetadataFieldSubsystemName,
		"service_name":     cxsdk.SpanFieldMetadataFieldServiceName,
		"operation_name":   cxsdk.SpanFieldMetadataFieldOperationName,
	}
	ProtoDashboardProtoToSchemaSpanFieldMetadataField = utils.ReverseMap(ProtoDashboardSchemaToProtoSpanFieldMetadataField)
	ProtoDashboardValidSpanFieldMetadataFields        = utils.GetKeys(ProtoDashboardSchemaToProtoSpanFieldMetadataField)
	ProtoDashboardSchemaToProtoSortBy                 = map[string]cxsdk.SortByType{
		utils.UNSPECIFIED: cxsdk.SortByTypeUnspecified,
		"value":           cxsdk.SortByTypeValue,
		"name":            cxsdk.SortByTypeName,
	}
	ProtoDashboardProtoToSchemaSortBy                = utils.ReverseMap(ProtoDashboardSchemaToProtoSortBy)
	ProtoDashboardValidSortBy                        = utils.GetKeys(ProtoDashboardSchemaToProtoSortBy)
	ProtoDashboardSchemaToProtoObservationFieldScope = map[string]cxsdk.DatasetScope{
		utils.UNSPECIFIED: cxsdk.DatasetScopeUnspecified,
		"user_data":       cxsdk.DatasetScopeUserData,
		"label":           cxsdk.DatasetScopeLabel,
		"metadata":        cxsdk.DatasetScopeMetadata,
	}
	ProtoDashboardProtoToSchemaObservationFieldScope = utils.ReverseMap(ProtoDashboardSchemaToProtoObservationFieldScope)
	ProtoDashboardValidObservationFieldScope         = utils.GetKeys(ProtoDashboardSchemaToProtoObservationFieldScope)
	ProtoDashboardSchemaToProtoDataModeType          = map[string]cxsdk.DataModeType{
		utils.UNSPECIFIED: cxsdk.DataModeTypeHighUnspecified,
		"archive":         cxsdk.DataModeTypeArchive,
	}
	ProtoDashboardProtoToSchemaDataModeType     = utils.ReverseMap(ProtoDashboardSchemaToProtoDataModeType)
	ProtoDashboardValidDataModeTypes            = utils.GetKeys(ProtoDashboardSchemaToProtoDataModeType)
	ProtoDashboardSchemaToProtoGaugeThresholdBy = map[string]cxsdk.GaugeThresholdBy{
		utils.UNSPECIFIED: cxsdk.GaugeThresholdByUnspecified,
		"value":           cxsdk.GaugeThresholdByValue,
		"background":      cxsdk.GaugeThresholdByBackground,
	}
	ProtoDashboardProtoToSchemaGaugeThresholdBy = utils.ReverseMap(ProtoDashboardSchemaToProtoGaugeThresholdBy)
	ProtoDashboardValidGaugeThresholdBy         = utils.GetKeys(ProtoDashboardSchemaToProtoGaugeThresholdBy)
	ProtoDashboardSchemaToProtoRefreshStrategy  = map[string]cxsdk.MultiSelectRefreshStrategy{
		utils.UNSPECIFIED:      cxsdk.MultiSelectRefreshStrategyUnspecified,
		"on_dashboard_load":    cxsdk.MultiSelectRefreshStrategyOnDashboardLoad,
		"on_time_frame_change": cxsdk.MultiSelectRefreshStrategyOnTimeFrameChange,
	}
	ProtoDashboardProtoToSchemaRefreshStrategy = utils.ReverseMap(ProtoDashboardSchemaToProtoRefreshStrategy)
	ProtoDashboardValidRefreshStrategies       = utils.GetKeys(ProtoDashboardSchemaToProtoRefreshStrategy)
	ProtoDashboardValidLogsAggregationTypes    = []string{"count", "count_distinct", "sum", "avg", "min", "max", "percentile"}
	ProtoDashboardValidSpanFieldTypes          = []string{"metadata", "tag", "process_tag"}
	ProtoDashboardValidSpanAggregationTypes    = []string{"metric", "dimension"}
	ProtoDashboardValidColorSchemes            = []string{"classic", "severity", "cold", "negative", "green", "red", "blue"}
	ProtoSectionValidColors                    = []string{"cyan", "green", "blue", "purple", "magenta", "pink", "orange"}

	ProtoDashboardSchemaToProtoThresholdType = map[string]cxsdk.ThresholdType{
		utils.UNSPECIFIED: cxsdk.ThresholdTypeUnspecified,
		"absolute":        cxsdk.ThresholdTypeAbsolute,
		"relative":        cxsdk.ThresholdTypeRelative,
	}
	ProtoDashboardProtoToSchemaThresholdType = utils.ReverseMap(ProtoDashboardSchemaToProtoThresholdType)
	ProtoDashboardValidThresholdTypes        = utils.GetKeys(ProtoDashboardSchemaToProtoThresholdType)
	ProtoDashboardSchemaToProtoLegendBy      = map[string]cxsdk.LegendBy{
		utils.UNSPECIFIED: cxsdk.LegendByUnspecified,
		"thresholds":      cxsdk.LegendByThresholds,
		"groups":          cxsdk.LegendByGroups,
	}
	ProtoDashboardProtoToSchemaLegendBy = utils.ReverseMap(ProtoDashboardSchemaToProtoLegendBy)
	ProtoDashboardValidLegendBys        = utils.GetKeys(ProtoDashboardSchemaToProtoLegendBy)

	ProtoDashboardSchemaToProtoPromQLQueryType = map[string]cxsdk.PromQLQueryType{
		utils.UNSPECIFIED: cxsdk.PromQLQueryTypeUnspecified,
		"range":           cxsdk.PromQLQueryTypeRange,
		"instant":         cxsdk.PromQLQueryTypeInstant,
	}
	ProtoDashboardProtoToSchemaPromQLQueryType = utils.ReverseMap(ProtoDashboardSchemaToProtoPromQLQueryType)
	ProtoDashboardValidPromQLQueryType         = utils.GetKeys(ProtoDashboardSchemaToProtoPromQLQueryType)

	ProtoSupportedWidgetTypes = []string{
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

func ProtoFlattenLegend(legend *cxsdk.DashboardLegend) *LegendModel {
	if legend == nil {
		return nil
	}

	return &LegendModel{
		IsVisible:    utils.WrapperspbBoolToTypeBool(legend.GetIsVisible()),
		GroupByQuery: utils.WrapperspbBoolToTypeBool(legend.GetGroupByQuery()),
		Columns:      protoFlattenLegendColumns(legend.GetColumns()),
		Placement:    types.StringValue(ProtoDashboardLegendPlacementProtoToSchema[legend.GetPlacement()]),
	}
}

func protoFlattenLegendColumns(columns []cxsdk.DashboardLegendColumn) types.List {
	if len(columns) == 0 {
		return types.ListNull(types.StringType)
	}

	columnsElements := make([]attr.Value, 0, len(columns))
	for _, column := range columns {
		flattenedColumn := ProtoDashboardLegendColumnProtoToSchema[column]
		columnElement := types.StringValue(flattenedColumn)
		columnsElements = append(columnsElements, columnElement)
	}

	return types.ListValueMust(types.StringType, columnsElements)
}

func ProtoExpandLegend(ctx context.Context, legend *LegendModel) (*cxsdk.DashboardLegend, diag.Diagnostics) {
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
		columns = append(columns, ProtoDashboardLegendColumnSchemaToProto[col.ValueString()])
	}
	if diagnostics.HasError() {
		return nil, diagnostics
	}

	return &cxsdk.DashboardLegend{
		IsVisible:    utils.TypeBoolToWrapperspbBool(legend.IsVisible),
		Columns:      columns,
		GroupByQuery: utils.TypeBoolToWrapperspbBool(legend.GroupByQuery),
		Placement:    ProtoDashboardLegendPlacementSchemaToProto[legend.Placement.ValueString()],
	}, nil
}

func ProtoFlattenSpansFields(ctx context.Context, spanFields []*cxsdk.SpanField) (types.List, diag.Diagnostics) {
	if len(spanFields) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: SpansFieldModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	spanFieldElements := make([]attr.Value, 0, len(spanFields))
	for _, field := range spanFields {
		flattenedField, dg := ProtoFlattenSpansField(field)
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

func ProtoFlattenSpansField(field *cxsdk.SpanField) (*SpansFieldModel, diag.Diagnostic) {
	if field == nil {
		return nil, nil
	}

	switch field.GetValue().(type) {
	case *cxsdk.SpanFieldMetadataField:
		return &SpansFieldModel{
			Type:  types.StringValue("metadata"),
			Value: types.StringValue(ProtoDashboardProtoToSchemaSpanFieldMetadataField[field.GetMetadataField()]),
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

func ProtoObservationFieldsObject() types.ObjectType {
	return types.ObjectType{
		AttrTypes: ObservationFieldAttr(),
	}
}

func ProtoFlattenDashboardFiltersSources(ctx context.Context, sources []*cxsdk.DashboardFilterSource) (types.List, diag.Diagnostics) {
	if len(sources) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: FilterSourceModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	filtersElements := make([]attr.Value, 0, len(sources))
	for _, source := range sources {
		flattenedFilter, diags := ProtoFlattenDashboardFilterSource(ctx, source)
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

func ProtoFlattenDashboardFilterSource(ctx context.Context, source *cxsdk.DashboardFilterSource) (*DashboardFilterSourceModel, diag.Diagnostics) {
	if source == nil {
		return nil, nil
	}

	switch source.GetValue().(type) {
	case *cxsdk.DashboardFilterSourceLogs:
		logs, diags := ProtoFlattenDashboardFilterSourceLogs(ctx, source.GetLogs())
		if diags.HasError() {
			return nil, diags
		}
		return &DashboardFilterSourceModel{Logs: logs}, nil
	case *cxsdk.DashboardFilterSourceSpans:
		spans, dg := ProtoFlattenDashboardFilterSourceSpans(source.GetSpans())
		if dg != nil {
			return nil, diag.Diagnostics{dg}
		}
		return &DashboardFilterSourceModel{Spans: spans}, nil
	case *cxsdk.DashboardFilterSourceMetrics:
		metrics, dg := ProtoFlattenDashboardFilterSourceMetrics(source.GetMetrics())
		if dg != nil {
			return nil, diag.Diagnostics{dg}
		}
		return &DashboardFilterSourceModel{Metrics: metrics}, nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Dashboard Filter Source", fmt.Sprintf("unknown filter source type %T", source))}
	}
}

func ProtoFlattenDashboardFilterSourceLogs(ctx context.Context, logs *cxsdk.DashboardFilterLogsFilter) (*FilterSourceLogsModel, diag.Diagnostics) {
	if logs == nil {
		return nil, nil
	}

	operator, dg := ProtoFlattenFilterOperator(logs.GetOperator())
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	observationField, diags := ProtoFlattenObservationField(ctx, logs.GetObservationField())
	if diags.HasError() {
		return nil, diags
	}

	return &FilterSourceLogsModel{
		Field:            utils.WrapperspbStringToTypeString(logs.GetField()),
		Operator:         operator,
		ObservationField: observationField,
	}, nil
}

func ProtoFlattenDashboardFilterSourceSpans(spans *cxsdk.DashboardFilterSpansFilter) (*FilterSourceSpansModel, diag.Diagnostic) {
	if spans == nil {
		return nil, nil
	}

	field, dg := ProtoFlattenSpansField(spans.GetField())
	if dg != nil {
		return nil, dg
	}

	operator, dg := ProtoFlattenFilterOperator(spans.GetOperator())
	if dg != nil {
		return nil, dg
	}

	return &FilterSourceSpansModel{
		Field:    field,
		Operator: operator,
	}, nil
}

func ProtoFlattenDashboardFilterSourceMetrics(metrics *cxsdk.DashboardFilterMetricsFilter) (*FilterSourceMetricsModel, diag.Diagnostic) {
	if metrics == nil {
		return nil, nil
	}

	operator, dg := ProtoFlattenFilterOperator(metrics.GetOperator())
	if dg != nil {
		return nil, dg
	}

	return &FilterSourceMetricsModel{
		MetricName:  utils.WrapperspbStringToTypeString(metrics.GetMetric()),
		MetricLabel: utils.WrapperspbStringToTypeString(metrics.GetLabel()),
		Operator:    operator,
	}, nil
}

func ProtoFlattenDashboardTimeFrame(ctx context.Context, d *cxsdk.Dashboard) (*TimeFrameModel, diag.Diagnostics) {
	if d.GetTimeFrame() == nil {
		return nil, nil
	}
	switch timeFrameType := d.GetTimeFrame().(type) {
	case *cxsdk.DashboardAbsoluteTimeFrame:
		return protoFlattenAbsoluteTimeFrame(ctx, timeFrameType.AbsoluteTimeFrame)
	case *cxsdk.DashboardRelativeTimeFrame:
		return protoFlattenRelativeTimeFrame(ctx, timeFrameType.RelativeTimeFrame)
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Dashboard Time Frame", fmt.Sprintf("unknown time frame type %T", timeFrameType))}
	}
}

func ProtoFlattenTimeFrameSelect(ctx context.Context, d *cxsdk.TimeframeSelect) (*TimeFrameModel, diag.Diagnostics) {
	if d.GetValue() == nil {
		return nil, nil
	}
	switch timeFrameType := d.GetValue().(type) {
	case *cxsdk.TimeframeSelectAbsolute:
		return protoFlattenAbsoluteTimeFrame(ctx, timeFrameType.AbsoluteTimeFrame)
	case *cxsdk.TimeframeSelectRelative:
		return protoFlattenRelativeTimeFrame(ctx, timeFrameType.RelativeTimeFrame)
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Dashboard Time Frame", fmt.Sprintf("unknown time frame type %T", timeFrameType))}
	}
}

func ProtoFlattenObservationField(ctx context.Context, field *cxsdk.ObservationField) (types.Object, diag.Diagnostics) {
	if field == nil {
		return types.ObjectNull(ObservationFieldAttr()), nil
	}

	return types.ObjectValueFrom(ctx, ObservationFieldAttr(), ProtoFlattenLogsFieldModel(field))
}

func ProtoFlattenLogsFieldModel(field *cxsdk.ObservationField) *ObservationFieldModel {
	return &ObservationFieldModel{
		Keypath: utils.WrappedStringSliceToTypeStringList(field.GetKeypath()),
		Scope:   types.StringValue(ProtoDashboardProtoToSchemaObservationFieldScope[field.GetScope()]),
	}
}

func protoFlattenDuration(timeFrame *durationpb.Duration) basetypes.StringValue {
	if timeFrame == nil {
		return types.StringNull()
	}
	if timeFrame.Seconds == 0 && timeFrame.Nanos == 0 {
		return types.StringValue("seconds:0")
	}
	return types.StringValue(timeFrame.String())
}

func protoFlattenAbsoluteTimeFrame(ctx context.Context, timeFrame *cxsdk.DashboardTimeFrame) (*TimeFrameModel, diag.Diagnostics) {
	absoluteTimeFrame := &TimeFrameAbsoluteModel{
		Start: types.StringValue(timeFrame.GetFrom().String()),
		End:   types.StringValue(timeFrame.GetTo().String()),
	}

	flattenedTimeFrame := &TimeFrameModel{
		Relative: nil,
		Absolute: absoluteTimeFrame,
	}
	return flattenedTimeFrame, nil
}

func protoFlattenRelativeTimeFrame(ctx context.Context, timeFrame *durationpb.Duration) (*TimeFrameModel, diag.Diagnostics) {
	relativeTimeFrame := &TimeFrameRelativeModel{
		Duration: protoFlattenDuration(timeFrame),
	}

	flattenedTimeFrame := &TimeFrameModel{
		Relative: relativeTimeFrame,
		Absolute: nil,
	}
	return flattenedTimeFrame, nil
}

func ProtoFlattenSpansFilters(ctx context.Context, filters []*cxsdk.DashboardFilterSpansFilter) (types.List, diag.Diagnostics) {
	if len(filters) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: SpansFilterModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	filtersElements := make([]attr.Value, 0, len(filters))
	for _, filter := range filters {
		flattenedFilter, dg := ProtoFlattenSpansFilter(filter)
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

func ProtoFlattenSpansFilter(filter *cxsdk.DashboardFilterSpansFilter) (*SpansFilterModel, diag.Diagnostic) {
	if filter == nil {
		return nil, nil
	}

	operator, dg := ProtoFlattenFilterOperator(filter.GetOperator())
	if dg != nil {
		return nil, dg
	}

	field, dg := ProtoFlattenSpansField(filter.GetField())
	if dg != nil {
		return nil, dg
	}

	return &SpansFilterModel{
		Field:    field,
		Operator: operator,
	}, nil
}

func ProtoFlattenFilterOperator(operator *cxsdk.DashboardFilterOperator) (*FilterOperatorModel, diag.Diagnostic) {
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

func ProtoFlattenMetricsFilters(ctx context.Context, filters []*cxsdk.DashboardFilterMetricsFilter) (types.List, diag.Diagnostics) {
	if len(filters) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: MetricsFilterModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	filtersElements := make([]attr.Value, 0, len(filters))
	for _, filter := range filters {
		flattenedFilter, dg := ProtoFlattenMetricsFilter(filter)
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

func ProtoFlattenMetricsFilter(filter *cxsdk.DashboardFilterMetricsFilter) (*MetricsFilterModel, diag.Diagnostic) {
	if filter == nil {
		return nil, nil
	}

	operator, dg := ProtoFlattenFilterOperator(filter.GetOperator())
	if dg != nil {
		return nil, dg
	}

	return &MetricsFilterModel{
		Metric:   utils.WrapperspbStringToTypeString(filter.GetMetric()),
		Label:    utils.WrapperspbStringToTypeString(filter.GetLabel()),
		Operator: operator,
	}, nil
}

func ProtoFlattenLogsFilters(ctx context.Context, filters []*cxsdk.DashboardFilterLogsFilter) (types.List, diag.Diagnostics) {
	if len(filters) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: LogsFilterModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	filtersElements := make([]attr.Value, 0, len(filters))
	for _, filter := range filters {
		flattenedFilter, diags := protoFlattenLogsFilter(ctx, filter)
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

func protoFlattenLogsFilter(ctx context.Context, filter *cxsdk.DashboardFilterLogsFilter) (*LogsFilterModel, diag.Diagnostics) {
	if filter == nil {
		return nil, nil
	}

	operator, dg := ProtoFlattenFilterOperator(filter.GetOperator())
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	observationField, diags := ProtoFlattenObservationField(ctx, filter.GetObservationField())
	if diags.HasError() {
		return nil, diags
	}

	return &LogsFilterModel{
		Field:            utils.WrapperspbStringToTypeString(filter.GetField()),
		Operator:         operator,
		ObservationField: observationField,
	}, nil
}

func ProtoFlattenObservationFields(ctx context.Context, namesFields []*cxsdk.ObservationField) (types.List, diag.Diagnostics) {
	if len(namesFields) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: ObservationFieldAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	fieldElements := make([]attr.Value, 0, len(namesFields))
	for _, field := range namesFields {
		flattenedField, diags := ProtoFlattenObservationField(ctx, field)
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

func ProtoFlattenLogsAggregation(ctx context.Context, aggregation *cxsdk.LogsAggregation) (*LogsAggregationModel, diag.Diagnostics) {
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
		observationField, diags := ProtoFlattenObservationField(ctx, aggregationValue.CountDistinct.GetObservationField())
		if diags.HasError() {
			return nil, diags
		}
		return &LogsAggregationModel{
			Type:             types.StringValue("count_distinct"),
			Field:            utils.WrapperspbStringToTypeString(aggregationValue.CountDistinct.GetField()),
			ObservationField: observationField,
		}, nil
	case *cxsdk.LogsAggregationSum:
		observationField, diags := ProtoFlattenObservationField(ctx, aggregationValue.Sum.GetObservationField())
		if diags.HasError() {
			return nil, diags
		}
		return &LogsAggregationModel{
			Type:             types.StringValue("sum"),
			Field:            utils.WrapperspbStringToTypeString(aggregationValue.Sum.GetField()),
			ObservationField: observationField,
		}, nil
	case *cxsdk.LogsAggregationAverage:
		observationField, diags := ProtoFlattenObservationField(ctx, aggregationValue.Average.GetObservationField())
		if diags.HasError() {
			return nil, diags
		}
		return &LogsAggregationModel{
			Type:             types.StringValue("avg"),
			Field:            utils.WrapperspbStringToTypeString(aggregationValue.Average.GetField()),
			ObservationField: observationField,
		}, nil
	case *cxsdk.LogsAggregationMin:
		observationField, diags := ProtoFlattenObservationField(ctx, aggregationValue.Min.GetObservationField())
		if diags.HasError() {
			return nil, diags
		}
		return &LogsAggregationModel{
			Type:             types.StringValue("min"),
			Field:            utils.WrapperspbStringToTypeString(aggregationValue.Min.GetField()),
			ObservationField: observationField,
		}, nil
	case *cxsdk.LogsAggregationMax:
		observationField, diags := ProtoFlattenObservationField(ctx, aggregationValue.Max.GetObservationField())
		if diags.HasError() {
			return nil, diags
		}
		return &LogsAggregationModel{
			Type:             types.StringValue("max"),
			Field:            utils.WrapperspbStringToTypeString(aggregationValue.Max.GetField()),
			ObservationField: observationField,
		}, nil
	case *cxsdk.LogsAggregationPercentile:
		observationField, diags := ProtoFlattenObservationField(ctx, aggregationValue.Percentile.GetObservationField())
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

func ProtoExpandObservationFields(ctx context.Context, namesFields types.List) ([]*cxsdk.ObservationField, diag.Diagnostics) {
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
		expandedNamesField, expandDiags := protoExpandObservationField(ctx, namesField)
		if expandDiags != nil {
			diags.Append(expandDiags...)
			continue
		}
		expandedNamesFields = append(expandedNamesFields, expandedNamesField)
	}

	return expandedNamesFields, diags
}

func ProtoExpandObservationFieldObject(ctx context.Context, field types.Object) (*cxsdk.ObservationField, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(field) {
		return nil, nil
	}

	var observationField ObservationFieldModel
	if dg := field.As(ctx, &observationField, basetypes.ObjectAsOptions{}); dg.HasError() {
		return nil, dg
	}

	return protoExpandObservationField(ctx, observationField)
}

func protoExpandObservationField(ctx context.Context, observationField ObservationFieldModel) (*cxsdk.ObservationField, diag.Diagnostics) {
	keypath, dg := utils.TypeStringSliceToWrappedStringSlice(ctx, observationField.Keypath.Elements())
	if dg.HasError() {
		return nil, dg
	}

	scope := ProtoDashboardSchemaToProtoObservationFieldScope[observationField.Scope.ValueString()]

	return &cxsdk.ObservationField{
		Keypath: keypath,
		Scope:   scope,
	}, nil
}

func ProtoExpandSpansField(spansFilterField *SpansFieldModel) (*cxsdk.SpanField, diag.Diagnostic) {
	if spansFilterField == nil {
		return nil, nil
	}

	switch spansFilterField.Type.ValueString() {
	case "metadata":
		return &cxsdk.SpanField{
			Value: &cxsdk.SpanFieldMetadataField{
				MetadataField: ProtoDashboardSchemaToProtoSpanFieldMetadataField[spansFilterField.Value.ValueString()],
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

func ProtoExpandSpansFields(ctx context.Context, spanFields types.List) ([]*cxsdk.SpanField, diag.Diagnostics) {
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
		expandedSpanField, expandDiag := ProtoExpandSpansField(&spansField)
		if expandDiag != nil {
			diags.Append(expandDiag)
			continue
		}
		expandedSpanFields = append(expandedSpanFields, expandedSpanField)
	}

	return expandedSpanFields, diags
}

func ProtoExpandLogsAggregations(ctx context.Context, logsAggregations types.List) ([]*cxsdk.LogsAggregation, diag.Diagnostics) {
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
		expandedLogsAggregation, expandDiags := ProtoExpandLogsAggregation(ctx, &aggregation)
		if expandDiags.HasError() {
			diags.Append(expandDiags...)
			continue
		}
		expandedLogsAggregations = append(expandedLogsAggregations, expandedLogsAggregation)
	}

	return expandedLogsAggregations, diags
}

func ProtoExpandLogsAggregation(ctx context.Context, logsAggregation *LogsAggregationModel) (*cxsdk.LogsAggregation, diag.Diagnostics) {
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
		observationField, diags := ProtoExpandObservationFieldObject(ctx, logsAggregation.ObservationField)
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
		observationField, diags := ProtoExpandObservationFieldObject(ctx, logsAggregation.ObservationField)
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
		observationField, diags := ProtoExpandObservationFieldObject(ctx, logsAggregation.ObservationField)
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
		observationField, diags := ProtoExpandObservationFieldObject(ctx, logsAggregation.ObservationField)
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
		observationField, diags := ProtoExpandObservationFieldObject(ctx, logsAggregation.ObservationField)
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
		observationField, diags := ProtoExpandObservationFieldObject(ctx, logsAggregation.ObservationField)
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

func ProtoExpandTimeFrameSelect(ctx context.Context, timeFrame *TimeFrameModel) (*cxsdk.TimeframeSelect, diag.Diagnostics) {
	if timeFrame == nil {
		return nil, nil
	}

	tf := cxsdk.TimeframeSelect{}

	switch {
	case timeFrame.Relative != nil:
		val, diags := protoExpandRelativeTimeFrame(ctx, timeFrame.Relative)
		if diags.HasError() {
			return nil, diags
		}
		tf.Value = &cxsdk.TimeframeSelectRelative{
			RelativeTimeFrame: val,
		}
	case timeFrame.Absolute != nil:
		from, to, diags := protoExpandAbsoluteTimeFrame(ctx, timeFrame.Absolute)
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

func ProtoExpandDashboardTimeFrame(ctx context.Context, dashboard *cxsdk.Dashboard, timeFrame *TimeFrameModel) (*cxsdk.Dashboard, diag.Diagnostics) {
	if timeFrame == nil {
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("No time frame received", "time frame was nil")}
	}

	var diags diag.Diagnostics
	switch {
	case timeFrame.Relative != nil:
		relative, diags := protoExpandRelativeTimeFrame(ctx, timeFrame.Relative)
		if diags.HasError() {
			return nil, diags
		}
		dashboard.TimeFrame = &cxsdk.DashboardRelativeTimeFrame{
			RelativeTimeFrame: relative,
		}
	case timeFrame.Absolute != nil:
		from, to, diags := protoExpandAbsoluteTimeFrame(ctx, timeFrame.Absolute)
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

func protoExpandRelativeTimeFrame(ctx context.Context, timeFrame *TimeFrameRelativeModel) (*durationpb.Duration, diag.Diagnostics) {
	duration, dg := utils.ParseDuration(timeFrame.Duration.ValueString(), "Relative Dashboard Time Frame")
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}
	return durationpb.New(*duration), nil
}

func protoExpandAbsoluteTimeFrame(ctx context.Context, timeFrame *TimeFrameAbsoluteModel) (*timestamppb.Timestamp, *timestamppb.Timestamp, diag.Diagnostics) {
	fromTime, err := time.Parse(time.RFC3339, timeFrame.Start.ValueString())
	if err != nil {
		return nil, nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand Absolute Dashboard Time Frame", fmt.Sprintf("Error parsing from time: %s", err.Error()))}
	}
	from := timestamppb.New(fromTime)

	toTime, err := time.Parse(time.RFC3339, timeFrame.End.ValueString())
	if err != nil {
		return from, nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand Absolute Dashboard Time Frame", fmt.Sprintf("Error parsing from time: %s", err.Error()))}
	}
	to := timestamppb.New(toTime)

	return from, to, nil
}

func ProtoSupportedWidgetsValidatorWithout(current string) validator.Object {
	matchers := make([]path.Expression, len(ProtoSupportedWidgetTypes)-1)
	for _, name := range ProtoSupportedWidgetTypes {
		if name != current {
			matchers = append(matchers, path.MatchRelative().AtParent().AtName(name))
		}
	}
	return objectvalidator.ExactlyOneOf(matchers...)
}

func ProtoFlattenSpansAggregation(aggregation *cxsdk.SpansAggregation) (*SpansAggregationModel, diag.Diagnostic) {
	if aggregation == nil || aggregation.GetAggregation() == nil {
		return nil, nil
	}
	switch aggregation := aggregation.GetAggregation().(type) {
	case *cxsdk.SpansAggregationMetricAggregation:
		return &SpansAggregationModel{
			Type:            types.StringValue("metric"),
			AggregationType: types.StringValue(ProtoDashboardProtoToSchemaSpansAggregationMetricAggregationType[aggregation.MetricAggregation.GetAggregationType()]),
			Field:           types.StringValue(ProtoDashboardProtoToSchemaSpansAggregationMetricField[aggregation.MetricAggregation.GetMetricField()]),
		}, nil
	case *cxsdk.SpansAggregationDimensionAggregation:
		return &SpansAggregationModel{
			Type:            types.StringValue("dimension"),
			AggregationType: types.StringValue(ProtoDashboardProtoToSchemaSpansAggregationDimensionAggregationType[aggregation.DimensionAggregation.GetAggregationType()]),
			Field:           types.StringValue(ProtoDashboardSchemaToProtoSpansAggregationDimensionField[aggregation.DimensionAggregation.GetDimensionField()]),
		}, nil
	default:
		return nil, diag.NewErrorDiagnostic("Error Flatten Span Aggregation", fmt.Sprintf("unknown aggregation type %T", aggregation))
	}
}

func ProtoExpandResolution(ctx context.Context, resolution types.Object) (*cxsdk.LineChartResolution, diag.Diagnostics) {
	if resolution.IsNull() || resolution.IsUnknown() {
		return nil, nil
	}

	var resolutionModel LineChartResolutionModel
	if diags := resolution.As(ctx, &resolutionModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	if !(resolutionModel.Interval.IsNull() || resolutionModel.Interval.IsUnknown()) {
		interval, dg := utils.ParseDuration(resolutionModel.Interval.ValueString(), "resolution.interval")
		if dg != nil {
			return nil, diag.Diagnostics{dg}
		}

		return &cxsdk.LineChartResolution{
			Interval: durationpb.New(*interval),
		}, nil
	}

	return &cxsdk.LineChartResolution{
		BucketsPresented: utils.TypeInt64ToWrappedInt32(resolutionModel.BucketsPresented),
	}, nil
}

func ProtoExpandDashboardUUID(id types.String) *cxsdk.UUID {
	if id.IsNull() || id.IsUnknown() {
		return &cxsdk.UUID{Value: uuid.NewString()}
	}
	return &cxsdk.UUID{Value: id.ValueString()}
}

func ProtoExpandDashboardIDs(id types.String) *wrapperspb.StringValue {
	if id.IsNull() || id.IsUnknown() {
		return &wrapperspb.StringValue{Value: uuid.NewString()}
	}
	return &wrapperspb.StringValue{Value: id.ValueString()}
}

func ProtoExpandDashboardFiltersSources(ctx context.Context, filters types.List) ([]*cxsdk.DashboardFilterSource, diag.Diagnostics) {
	var filtersObjects []types.Object
	var expandedFiltersSources []*cxsdk.DashboardFilterSource
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
		expandedFilter, expandDiags := ProtoExpandFilterSource(ctx, &filterSource)
		if expandDiags.HasError() {
			diags.Append(expandDiags...)
			continue
		}
		expandedFiltersSources = append(expandedFiltersSources, expandedFilter)
	}

	return expandedFiltersSources, diags
}

func ProtoExpandMetricsFilters(ctx context.Context, metricFilters types.List) ([]*cxsdk.DashboardMetricsFilter, diag.Diagnostics) {
	var metricFiltersObjects []types.Object
	var expandedMetricFilters []*cxsdk.DashboardMetricsFilter
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
		expandedMetricFilter, expandDiags := protoExpandMetricFilter(ctx, metricsFilter)
		if expandDiags.HasError() {
			diags.Append(expandDiags...)
			continue
		}
		expandedMetricFilters = append(expandedMetricFilters, expandedMetricFilter)
	}

	return expandedMetricFilters, diags
}

func protoExpandMetricFilter(ctx context.Context, metricFilter MetricsFilterModel) (*cxsdk.DashboardMetricsFilter, diag.Diagnostics) {
	operator, diags := protoExpandFilterOperator(ctx, metricFilter.Operator)
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.DashboardMetricsFilter{
		Metric:   utils.TypeStringToWrapperspbString(metricFilter.Metric),
		Label:    utils.TypeStringToWrapperspbString(metricFilter.Label),
		Operator: operator,
	}, nil
}

func ProtoExpandLogsFilters(ctx context.Context, logsFilters types.List) ([]*cxsdk.DashboardFilterLogsFilter, diag.Diagnostics) {
	var filtersObjects []types.Object
	var expandedFilters []*cxsdk.DashboardFilterLogsFilter
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
		expandedFilter, expandDiags := protoExpandLogsFilter(ctx, filter)
		if expandDiags.HasError() {
			diags.Append(expandDiags...)
			continue
		}
		expandedFilters = append(expandedFilters, expandedFilter)
	}

	return expandedFilters, diags
}

func protoExpandLogsFilter(ctx context.Context, logsFilter LogsFilterModel) (*cxsdk.DashboardFilterLogsFilter, diag.Diagnostics) {
	operator, diags := protoExpandFilterOperator(ctx, logsFilter.Operator)
	if diags.HasError() {
		return nil, diags
	}

	observationField, diags := ProtoExpandObservationFieldObject(ctx, logsFilter.ObservationField)
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.DashboardFilterLogsFilter{
		Field:            utils.TypeStringToWrapperspbString(logsFilter.Field),
		Operator:         operator,
		ObservationField: observationField,
	}, nil
}

func ProtoExpandLuceneQuery(luceneQuery types.String) *cxsdk.DashboardLuceneQuery {
	if luceneQuery.IsNull() || luceneQuery.IsUnknown() {
		return nil
	}
	return &cxsdk.DashboardLuceneQuery{
		Value: wrapperspb.String(luceneQuery.ValueString()),
	}
}

func ProtoExpandFilterSource(ctx context.Context, source *DashboardFilterSourceModel) (*cxsdk.DashboardFilterSource, diag.Diagnostics) {
	if source == nil {
		return nil, nil
	}

	switch {
	case source.Logs != nil:
		return protoExpandFilterSourceLogs(ctx, source.Logs)
	case source.Metrics != nil:
		return protoExpandFilterSourceMetrics(ctx, source.Metrics)
	case source.Spans != nil:
		return protoExpandFilterSourceSpans(ctx, source.Spans)
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand Filter Source", fmt.Sprintf("Unknown filter source type: %#v", source))}
	}
}
func protoExpandFilterSourceLogs(ctx context.Context, logs *FilterSourceLogsModel) (*cxsdk.DashboardFilterSource, diag.Diagnostics) {
	if logs == nil {
		return nil, nil
	}

	operator, diags := protoExpandFilterOperator(ctx, logs.Operator)
	if diags.HasError() {
		return nil, diags
	}

	observationField, diags := ProtoExpandObservationFieldObject(ctx, logs.ObservationField)
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.DashboardFilterSource{
		Value: &cxsdk.DashboardFilterSourceLogs{
			Logs: &cxsdk.DashboardFilterLogsFilter{
				Field:            utils.TypeStringToWrapperspbString(logs.Field),
				Operator:         operator,
				ObservationField: observationField,
			},
		},
	}, nil
}

func protoExpandFilterSourceMetrics(ctx context.Context, metrics *FilterSourceMetricsModel) (*cxsdk.DashboardFilterSource, diag.Diagnostics) {
	if metrics == nil {
		return nil, nil
	}

	operator, diags := protoExpandFilterOperator(ctx, metrics.Operator)
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.DashboardFilterSource{
		Value: &cxsdk.DashboardFilterSourceMetrics{
			Metrics: &cxsdk.DashboardFilterMetricsFilter{
				Metric:   utils.TypeStringToWrapperspbString(metrics.MetricName),
				Label:    utils.TypeStringToWrapperspbString(metrics.MetricLabel),
				Operator: operator,
			},
		},
	}, nil
}

func protoExpandFilterSourceSpans(ctx context.Context, spans *FilterSourceSpansModel) (*cxsdk.DashboardFilterSource, diag.Diagnostics) {
	if spans == nil {
		return nil, nil
	}

	field, dg := ProtoExpandSpansField(spans.Field)
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	operator, diags := protoExpandFilterOperator(ctx, spans.Operator)
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.DashboardFilterSource{
		Value: &cxsdk.DashboardFilterSourceSpans{
			Spans: &cxsdk.DashboardFilterSpansFilter{
				Field:    field,
				Operator: operator,
			},
		},
	}, nil
}

func protoExpandFilterOperator(ctx context.Context, operator *FilterOperatorModel) (*cxsdk.DashboardFilterOperator, diag.Diagnostics) {
	if operator == nil {
		return nil, nil
	}

	selectedValues, diags := utils.TypeStringSliceToWrappedStringSlice(ctx, operator.SelectedValues.Elements())
	if diags.HasError() {
		return nil, diags
	}

	switch operator.Type.ValueString() {
	case "equals":
		filterOperator := &cxsdk.DashboardFilterOperator{
			Value: &cxsdk.DashboardFilterOperatorEquals{
				Equals: &cxsdk.DashboardFilterEquals{
					Selection: &cxsdk.DashboardFilterEqualsSelection{},
				},
			},
		}
		if len(selectedValues) != 0 {
			filterOperator.GetEquals().Selection.Value = &cxsdk.DashboardFilterEqualsSelectionList{
				List: &cxsdk.DashboardFilterEqualsSelectionListSelection{
					Values: selectedValues,
				},
			}
		} else {
			filterOperator.GetEquals().Selection.Value = &cxsdk.DashboardFilterEqualsSelectionAll{
				All: &cxsdk.DashboardFilterEqualsSelectionAllSelection{},
			}
		}
		return filterOperator, nil
	case "not_equals":
		return &cxsdk.DashboardFilterOperator{
			Value: &cxsdk.DashboardFilterOperatorNotEquals{
				NotEquals: &cxsdk.DashboardFilterNotEquals{
					Selection: &cxsdk.DashboardFilterNotEqualsSelection{
						Value: &cxsdk.DashboardFilterNotEqualsSelectionList{
							List: &cxsdk.DashboardFilterNotEqualsSelectionListSelection{
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

func ProtoExpandPromqlQuery(promqlQuery types.String) *cxsdk.DashboardPromQLQuery {
	if promqlQuery.IsNull() || promqlQuery.IsUnknown() {
		return nil
	}

	return &cxsdk.DashboardPromQLQuery{
		Value: wrapperspb.String(promqlQuery.ValueString()),
	}
}

func ProtoExpandSpansAggregations(ctx context.Context, aggregations types.List) ([]*cxsdk.SpansAggregation, diag.Diagnostics) {
	var aggregationsObjects []types.Object
	var expandedAggregations []*cxsdk.SpansAggregation
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
		expandedAggregation, expandDiag := ProtoExpandSpansAggregation(&aggregation)
		if expandDiag != nil {
			diags.Append(expandDiag)
			continue
		}
		expandedAggregations = append(expandedAggregations, expandedAggregation)
	}

	return expandedAggregations, diags
}

func ProtoExpandSpansAggregation(spansAggregation *SpansAggregationModel) (*cxsdk.SpansAggregation, diag.Diagnostic) {
	if spansAggregation == nil {
		return nil, nil
	}

	switch spansAggregation.Type.ValueString() {
	case "metric":
		return &cxsdk.SpansAggregation{
			Aggregation: &cxsdk.SpansAggregationMetricAggregation{
				MetricAggregation: &cxsdk.SpansAggregationMetricAggregationInner{
					MetricField:     ProtoDashboardSchemaToProtoSpansAggregationMetricField[spansAggregation.Field.ValueString()],
					AggregationType: ProtoDashboardSchemaToProtoSpansAggregationMetricAggregationType[spansAggregation.AggregationType.ValueString()],
				},
			},
		}, nil
	case "dimension":
		return &cxsdk.SpansAggregation{
			Aggregation: &cxsdk.SpansAggregationDimensionAggregation{
				DimensionAggregation: &cxsdk.SpansAggregationDimensionAggregationInner{
					DimensionField:  ProtoDashboardProtoToSchemaSpansAggregationDimensionField[spansAggregation.Field.ValueString()],
					AggregationType: ProtoDashboardSchemaToProtoSpansAggregationDimensionAggregationType[spansAggregation.AggregationType.ValueString()],
				},
			},
		}, nil
	default:
		return nil, diag.NewErrorDiagnostic("Extract Spans Aggregation Error", fmt.Sprintf("Unknown spans aggregation type %#v", spansAggregation))
	}
}

func ProtoExpandSpansFilters(ctx context.Context, spansFilters types.List) ([]*cxsdk.DashboardFilterSpansFilter, diag.Diagnostics) {
	var spansFiltersObjects []types.Object
	var expandedSpansFilters []*cxsdk.DashboardFilterSpansFilter
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
		expandedSpansFilter, expandDiags := protoExpandSpansFilter(ctx, spansFilter)
		if expandDiags.HasError() {
			diags.Append(expandDiags...)
			continue
		}
		expandedSpansFilters = append(expandedSpansFilters, expandedSpansFilter)
	}

	return expandedSpansFilters, diags
}

func protoExpandSpansFilter(ctx context.Context, spansFilter SpansFilterModel) (*cxsdk.DashboardFilterSpansFilter, diag.Diagnostics) {
	operator, diags := protoExpandFilterOperator(ctx, spansFilter.Operator)
	if diags.HasError() {
		return nil, diags
	}

	field, dg := ProtoExpandSpansField(spansFilter.Field)
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	return &cxsdk.DashboardFilterSpansFilter{
		Field:    field,
		Operator: operator,
	}, nil
}
