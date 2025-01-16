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
	"strings"
	"terraform-provider-coralogix/coralogix/utils"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	"github.com/hashicorp/terraform-plugin-framework-validators/float64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
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

type QueryLogsModel struct {
	LuceneQuery  types.String `tfsdk:"lucene_query"`
	GroupBy      types.List   `tfsdk:"group_by"`     //types.String
	Aggregations types.List   `tfsdk:"aggregations"` //AggregationModel
	Filters      types.List   `tfsdk:"filters"`      //FilterModel
}

type QueryMetricsModel struct {
	PromqlQuery types.String `tfsdk:"promql_query"`
	Filters     types.List   `tfsdk:"filters"` //MetricsFilterModel
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

func LegendSchema() schema.SingleNestedAttribute {
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

func LogsFiltersSchema() schema.ListNestedAttribute {
	return schema.ListNestedAttribute{
		Optional: true,
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"field": schema.StringAttribute{
					Required: true,
				},
				"operator": FilterOperatorSchema(),
				"observation_field": schema.SingleNestedAttribute{
					Attributes: ObservationFieldSchemaAttributes(),
					Optional:   true,
				},
			},
		},
		Validators: []validator.List{
			listvalidator.SizeAtLeast(1),
		},
	}
}

func UnitSchema() schema.StringAttribute {
	return schema.StringAttribute{
		Optional: true,
		Computed: true,
		Default:  stringdefault.StaticString("unspecified"),
		Validators: []validator.String{
			stringvalidator.OneOf(DashboardValidUnits...),
		},
		MarkdownDescription: fmt.Sprintf("The unit. Valid values are: %s.", strings.Join(DashboardValidUnits, ", ")),
	}
}

func FiltersSourceAttribute() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"logs": schema.SingleNestedAttribute{
			Attributes: map[string]schema.Attribute{
				"field": schema.StringAttribute{
					Required:            true,
					MarkdownDescription: "Field in the logs to apply the filter on.",
				},
				"operator": FilterOperatorSchema(),
				"observation_field": schema.SingleNestedAttribute{
					Attributes: ObservationFieldSchemaAttributes(),
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
					Attributes: SpansFieldAttributes(),
					Required:   true,
					Validators: []validator.Object{
						spansFieldValidator{},
					},
				},
				"operator": FilterOperatorSchema(),
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
				"operator": FilterOperatorSchema(),
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

func FilterOperatorSchema() schema.SingleNestedAttribute {
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

func ObservationFieldSchemaAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"keypath": schema.ListAttribute{
			ElementType: types.StringType,
			Required:    true,
		},
		"scope": schema.StringAttribute{
			Required: true,
			Validators: []validator.String{
				stringvalidator.OneOf(DashboardValidObservationFieldScope...),
			},
		},
	}
}

func SpansFilterSchema() schema.ListNestedAttribute {
	return schema.ListNestedAttribute{
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"field": schema.SingleNestedAttribute{
					Attributes: SpansFieldAttributes(),
					Required:   true,
				},
				"operator": FilterOperatorSchema(),
			},
		},
		Optional: true,
	}
}

func SpansFieldSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Attributes: SpansFieldAttributes(),
		Optional:   true,
		Validators: []validator.Object{
			spansFieldValidator{},
		},
	}
}

func SpansFieldsSchema() schema.ListNestedAttribute {
	return schema.ListNestedAttribute{
		NestedObject: schema.NestedAttributeObject{
			Attributes: SpansFieldAttributes(),
			Validators: []validator.Object{
				spansFieldValidator{},
			},
		},
		Optional: true,
	}
}

func SpansFieldAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"type": schema.StringAttribute{
			Required: true,
			Validators: []validator.String{
				stringvalidator.OneOf(DashboardValidSpanFieldTypes...),
			},
			MarkdownDescription: fmt.Sprintf("The type of the field. Can be one of %q", DashboardValidSpanFieldTypes),
		},
		"value": schema.StringAttribute{
			Required:            true,
			MarkdownDescription: fmt.Sprintf("The value of the field. When the field type is `metadata`, can be one of %q", DashboardValidSpanFieldMetadataFields),
		},
	}
}

func SpansAggregationsSchema() schema.ListNestedAttribute {
	return schema.ListNestedAttribute{
		NestedObject: schema.NestedAttributeObject{
			Attributes: SpansAggregationAttributes(),
			Validators: []validator.Object{
				spansAggregationValidator{},
			},
		},
		Optional: true,
	}
}

func SpansAggregationSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Attributes: SpansAggregationAttributes(),
		Optional:   true,
		Validators: []validator.Object{
			spansAggregationValidator{},
		},
	}
}
func SpansAggregationAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"type": schema.StringAttribute{
			Required: true,
			Validators: []validator.String{
				stringvalidator.OneOf(DashboardValidSpanAggregationTypes...),
			},
			MarkdownDescription: fmt.Sprintf("Can be one of %q", DashboardValidSpanAggregationTypes),
		},
		"aggregation_type": schema.StringAttribute{
			Required:            true,
			MarkdownDescription: fmt.Sprintf("The type of the aggregation. When the aggregation type is `metrics`, can be one of %q. When the aggregation type is `dimension`, can be one of %q.", DashboardValidSpansAggregationMetricAggregationTypes, DashboardValidSpansAggregationDimensionAggregationTypes),
		},
		"field": schema.StringAttribute{
			Required:            true,
			MarkdownDescription: fmt.Sprintf("The field to aggregate on. When the aggregation type is `metrics`, can be one of %q. When the aggregation type is `dimension`, can be one of %q.", DashboardValidSpansAggregationMetricFields, DashboardValidSpansAggregationDimensionFields),
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

func LogsAggregationSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Required:   true,
		Attributes: LogsAggregationAttributes(),
		Validators: []validator.Object{
			logsAggregationValidator{},
		},
	}
}

func LogsAggregationsSchema() schema.ListNestedAttribute {
	return schema.ListNestedAttribute{
		Required: true,
		NestedObject: schema.NestedAttributeObject{
			Attributes: LogsAggregationAttributes(),
			Validators: []validator.Object{
				logsAggregationValidator{},
			},
		},
		Validators: []validator.List{
			listvalidator.SizeAtLeast(1),
		},
	}
}

func LogsAggregationAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"type": schema.StringAttribute{
			Required: true,
			Validators: []validator.String{
				stringvalidator.OneOf(DashboardValidLogsAggregationTypes...),
			},
			MarkdownDescription: fmt.Sprintf("The type of the aggregation. Can be one of %q", DashboardValidLogsAggregationTypes),
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
			Attributes: ObservationFieldSchemaAttributes(),
			Optional:   true,
		},
	}
}

func MetricFiltersSchema() schema.ListNestedAttribute {
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
				"operator": FilterOperatorSchema(),
			},
		},
		Validators: []validator.List{
			listvalidator.SizeAtLeast(1),
		},
		Optional: true,
	}
}

func TimeFrameSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
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
	}
}
