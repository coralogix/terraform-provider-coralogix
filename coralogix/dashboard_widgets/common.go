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
	"fmt"
	"strings"
	"terraform-provider-coralogix/coralogix/utils"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	DashboardSchemaToProtoUnit = map[string]cxsdk.Unit{
		"unspecified":  cxsdk.UnitUnspecified,
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
		"unspecified": cxsdk.LegendPlacementUnspecified,
		"auto":        cxsdk.LegendPlacementAuto,
		"bottom":      cxsdk.LegendPlacementBottom,
		"side":        cxsdk.LegendPlacementSide,
		"hidden":      cxsdk.LegendPlacementHidden,
	}
	DashboardLegendPlacementProtoToSchema = utils.ReverseMap(DashboardLegendPlacementSchemaToProto)
	DashboardValidLegendPlacements        = utils.GetKeys(DashboardLegendPlacementSchemaToProto)

	DashboardRowStyleSchemaToProto = map[string]cxsdk.RowStyle{
		"unspecified": cxsdk.RowStyleUnspecified,
		"one_line":    cxsdk.RowStyleOneLine,
		"two_line":    cxsdk.RowStyleTwoLine,
		"condensed":   cxsdk.RowStyleCondensed,
		"json":        cxsdk.RowStyleJSON,
		"list":        cxsdk.RowStyleList,
	}
	DashboardRowStyleProtoToSchema     = utils.ReverseMap(DashboardRowStyleSchemaToProto)
	DashboardValidRowStyles            = utils.GetKeys(DashboardRowStyleSchemaToProto)
	DashboardLegendColumnSchemaToProto = map[string]cxsdk.DashboardLegendColumn{
		"unspecified": cxsdk.LegendColumnUnspecified,
		"min":         cxsdk.LegendColumnMin,
		"max":         cxsdk.LegendColumnMax,
		"sum":         cxsdk.LegendColumnSum,
		"avg":         cxsdk.LegendColumnAvg,
		"last":        cxsdk.LegendColumnLast,
		"name":        cxsdk.LegendColumnName,
	}
	DashboardLegendColumnProtoToSchema   = utils.ReverseMap(DashboardLegendColumnSchemaToProto)
	DashboardValidLegendColumns          = utils.GetKeys(DashboardLegendColumnSchemaToProto)
	DashboardOrderDirectionSchemaToProto = map[string]cxsdk.OrderDirection{
		"unspecified": cxsdk.OrderDirectionUnspecified,
		"asc":         cxsdk.OrderDirectionAsc,
		"desc":        cxsdk.OrderDirectionDesc,
	}
	DashboardOrderDirectionProtoToSchema = utils.ReverseMap(DashboardOrderDirectionSchemaToProto)
	DashboardValidOrderDirections        = utils.GetKeys(DashboardOrderDirectionSchemaToProto)
	DashboardSchemaToProtoTooltipType    = map[string]cxsdk.LineChartTooltipType{
		"unspecified": cxsdk.LineChartToolTipTypeUnspecified,
		"all":         cxsdk.LineChartToolTipTypeAll,
		"single":      cxsdk.LineChartToolTipTypeSingle,
	}
	DashboardProtoToSchemaTooltipType = utils.ReverseMap(DashboardSchemaToProtoTooltipType)
	DashboardValidTooltipTypes        = utils.GetKeys(DashboardSchemaToProtoTooltipType)
	DashboardSchemaToProtoScaleType   = map[string]cxsdk.ScaleType{
		"unspecified": cxsdk.ScaleTypeUnspecified,
		"linear":      cxsdk.ScaleTypeLinear,
		"logarithmic": cxsdk.ScaleTypeLogarithmic,
	}
	DashboardProtoToSchemaScaleType = utils.ReverseMap(DashboardSchemaToProtoScaleType)
	DashboardValidScaleTypes        = utils.GetKeys(DashboardSchemaToProtoScaleType)

	DashboardSchemaToProtoGaugeUnit = map[string]cxsdk.GaugeUnit{
		"unspecified":  cxsdk.GaugeUnitUnspecified,
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
		"unspecified": cxsdk.PieChartLabelSourceUnspecified,
		"inner":       cxsdk.PieChartLabelSourceInner,
		"stack":       cxsdk.PieChartLabelSourceStack,
	}
	DashboardProtoToSchemaPieChartLabelSource = utils.ReverseMap(DashboardSchemaToProtoPieChartLabelSource)
	DashboardValidPieChartLabelSources        = utils.GetKeys(DashboardSchemaToProtoPieChartLabelSource)
	DashboardSchemaToProtoGaugeAggregation    = map[string]cxsdk.GaugeAggregation{
		"unspecified": cxsdk.GaugeAggregationUnspecified,
		"last":        cxsdk.GaugeAggregationLast,
		"min":         cxsdk.GaugeAggregationMin,
		"max":         cxsdk.GaugeAggregationMax,
		"avg":         cxsdk.GaugeAggregationAvg,
		"sum":         cxsdk.GaugeAggregationSum,
	}
	DashboardProtoToSchemaGaugeAggregation            = utils.ReverseMap(DashboardSchemaToProtoGaugeAggregation)
	DashboardValidGaugeAggregations                   = utils.GetKeys(DashboardSchemaToProtoGaugeAggregation)
	DashboardSchemaToProtoSpansAggregationMetricField = map[string]cxsdk.SpansAggregationMetricAggregationMetricField{
		"unspecified": cxsdk.SpansAggregationMetricAggregationMetricFieldUnspecified,
		"duration":    cxsdk.SpansAggregationMetricAggregationMetricFieldDuration,
	}
	DashboardProtoToSchemaSpansAggregationMetricField           = utils.ReverseMap(DashboardSchemaToProtoSpansAggregationMetricField)
	DashboardValidSpansAggregationMetricFields                  = utils.GetKeys(DashboardSchemaToProtoSpansAggregationMetricField)
	DashboardSchemaToProtoSpansAggregationMetricAggregationType = map[string]cxsdk.SpansAggregationMetricAggregationMetricAggregationType{
		"unspecified":   cxsdk.SpansAggregationMetricAggregationMetricTypeUnspecified,
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
		"unspecified": cxsdk.SpansAggregationDimensionAggregationDimensionFieldUnspecified,
		"trace_id":    cxsdk.SpansAggregationDimensionAggregationDimensionFieldTraceID,
	}
	DashboardSchemaToProtoSpansAggregationDimensionField           = utils.ReverseMap(DashboardProtoToSchemaSpansAggregationDimensionField)
	DashboardValidSpansAggregationDimensionFields                  = utils.GetKeys(DashboardProtoToSchemaSpansAggregationDimensionField)
	DashboardSchemaToProtoSpansAggregationDimensionAggregationType = map[string]cxsdk.SpansAggregationDimensionAggregationType{
		"unspecified":  cxsdk.SpansAggregationDimensionAggregationTypeUnspecified,
		"unique_count": cxsdk.SpansAggregationDimensionAggregationTypeUniqueCount,
		"error_count":  cxsdk.SpansAggregationDimensionAggregationTypeErrorCount,
	}
	DashboardProtoToSchemaSpansAggregationDimensionAggregationType = utils.ReverseMap(DashboardSchemaToProtoSpansAggregationDimensionAggregationType)
	DashboardValidSpansAggregationDimensionAggregationTypes        = utils.GetKeys(DashboardSchemaToProtoSpansAggregationDimensionAggregationType)
	DashboardSchemaToProtoSpanFieldMetadataField                   = map[string]cxsdk.SpanFieldMetadataFieldInner{
		"unspecified":      cxsdk.SpanFieldMetadataFieldUnspecified,
		"application_name": cxsdk.SpanFieldMetadataFieldApplicationName,
		"subsystem_name":   cxsdk.SpanFieldMetadataFieldSubsystemName,
		"service_name":     cxsdk.SpanFieldMetadataFieldServiceName,
		"operation_name":   cxsdk.SpanFieldMetadataFieldOperationName,
	}
	DashboardProtoToSchemaSpanFieldMetadataField = utils.ReverseMap(DashboardSchemaToProtoSpanFieldMetadataField)
	DashboardValidSpanFieldMetadataFields        = utils.GetKeys(DashboardSchemaToProtoSpanFieldMetadataField)
	DashboardSchemaToProtoSortBy                 = map[string]cxsdk.SortByType{
		"unspecified": cxsdk.SortByTypeUnspecified,
		"value":       cxsdk.SortByTypeValue,
		"name":        cxsdk.SortByTypeName,
	}
	DashboardProtoToSchemaSortBy                = utils.ReverseMap(DashboardSchemaToProtoSortBy)
	DashboardValidSortBy                        = utils.GetKeys(DashboardSchemaToProtoSortBy)
	DashboardSchemaToProtoObservationFieldScope = map[string]cxsdk.DatasetScope{
		"unspecified": cxsdk.DatasetScopeUnspecified,
		"user_data":   cxsdk.DatasetScopeUserData,
		"label":       cxsdk.DatasetScopeLabel,
		"metadata":    cxsdk.DatasetScopeMetadata,
	}
	DashboardProtoToSchemaObservationFieldScope = utils.ReverseMap(DashboardSchemaToProtoObservationFieldScope)
	DashboardValidObservationFieldScope         = utils.GetKeys(DashboardSchemaToProtoObservationFieldScope)
	DashboardSchemaToProtoDataModeType          = map[string]cxsdk.DataModeType{
		"unspecified": cxsdk.DataModeTypeHighUnspecified,
		"archive":     cxsdk.DataModeTypeArchive,
	}
	DashboardProtoToSchemaDataModeType     = utils.ReverseMap(DashboardSchemaToProtoDataModeType)
	DashboardValidDataModeTypes            = utils.GetKeys(DashboardSchemaToProtoDataModeType)
	DashboardSchemaToProtoGaugeThresholdBy = map[string]cxsdk.GaugeThresholdBy{
		"unspecified": cxsdk.GaugeThresholdByUnspecified,
		"value":       cxsdk.GaugeThresholdByValue,
		"background":  cxsdk.GaugeThresholdByBackground,
	}
	DashboardProtoToSchemaGaugeThresholdBy = utils.ReverseMap(DashboardSchemaToProtoGaugeThresholdBy)
	DashboardValidGaugeThresholdBy         = utils.GetKeys(DashboardSchemaToProtoGaugeThresholdBy)
	DashboardSchemaToProtoRefreshStrategy  = map[string]cxsdk.MultiSelectRefreshStrategy{
		"unspecified":          cxsdk.MultiSelectRefreshStrategyUnspecified,
		"on_dashboard_load":    cxsdk.MultiSelectRefreshStrategyOnDashboardLoad,
		"on_time_frame_change": cxsdk.MultiSelectRefreshStrategyOnTimeFrameChange,
	}
	DashboardProtoToSchemaRefreshStrategy = utils.ReverseMap(DashboardSchemaToProtoRefreshStrategy)
	DashboardValidRefreshStrategies       = utils.GetKeys(DashboardSchemaToProtoRefreshStrategy)
	DashboardValidLogsAggregationTypes    = []string{"count", "count_distinct", "sum", "avg", "min", "max", "percentile"}
	DashboardValidSpanFieldTypes          = []string{"metadata", "tag", "process_tag"}
	DashboardValidSpanAggregationTypes    = []string{"metric", "dimension"}
	DashboardValidColorSchemes            = []string{"classic", "severity", "cold", "negative", "green", "red", "blue"}
	SectionValidColors                    = []string{"unspecified", "cyan", "green", "blue", "purple", "magenta", "pink", "orange"}

	DashboardThresholdTypeSchemaToProto = map[string]cxsdk.ThresholdType{
		"unspecified": cxsdk.ThresholdTypeUnspecified,
		"absolute":    cxsdk.ThresholdTypeAbsolute,
		"relative":    cxsdk.ThresholdTypeRelative,
	}
	DashboardThresholdTypeProtoToSchema = utils.ReverseMap(DashboardThresholdTypeSchemaToProto)
	DashboardValidThresholdTypes        = utils.GetKeys(DashboardThresholdTypeSchemaToProto)
	DashboardLegendBySchemaToProto      = map[string]cxsdk.LegendBy{
		"unspecified": cxsdk.LegendByUnspecified,
		"thresholds":  cxsdk.LegendByThresholds,
		"groups":      cxsdk.LegendByGroups,
	}
	DashboardLegendByProtoToSchema = utils.ReverseMap(DashboardLegendBySchemaToProto)
	DashboardValidLegendBys        = utils.GetKeys(DashboardLegendBySchemaToProto)
)

type LegendModel struct {
	IsVisible    types.Bool   `tfsdk:"is_visible"`
	Columns      types.List   `tfsdk:"columns"` //types.String (DashboardValidLegendColumns)
	GroupByQuery types.Bool   `tfsdk:"group_by_query"`
	Placement    types.String `tfsdk:"placement"`
}

func legendSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Attributes: map[string]schema.Attribute{
			"is_visible": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
				MarkdownDescription: "Whether to display the legend. True by default.",
			},
			"columns": schema.ListAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Validators: []validator.List{
					listvalidator.ValueStringsAre(stringvalidator.OneOf(DashboardValidLegendColumns...)),
					listvalidator.SizeAtLeast(1),
				},
				MarkdownDescription: fmt.Sprintf("The columns to display in the legend. Valid values are: %s.", strings.Join(DashboardValidLegendColumns, ", ")),
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
					stringvalidator.OneOf(DashboardValidLegendPlacements...),
				},
				MarkdownDescription: fmt.Sprintf("The placement of the legend. Valid values are: %s.", strings.Join(DashboardValidLegendPlacements, ", ")),
			},
		},
		Optional: true,
	}
}
