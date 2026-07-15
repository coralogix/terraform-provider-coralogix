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
	"strings"

	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	dashboardservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_service"

	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

var (
	DashboardSchemaToProtoHexagonAggregation = map[string]dashboardservice.CommonAggregation{
		utils.UNSPECIFIED: dashboardservice.COMMONAGGREGATION_AGGREGATION_UNSPECIFIED,
		"last":            dashboardservice.COMMONAGGREGATION_AGGREGATION_LAST,
		"min":             dashboardservice.COMMONAGGREGATION_AGGREGATION_MIN,
		"max":             dashboardservice.COMMONAGGREGATION_AGGREGATION_MAX,
		"avg":             dashboardservice.COMMONAGGREGATION_AGGREGATION_AVG,
		"sum":             dashboardservice.COMMONAGGREGATION_AGGREGATION_SUM,
	}

	DashboardProtoToSchemaHexagonMetricAggregation = utils.ReverseMap(DashboardSchemaToProtoHexagonAggregation)
	DashboardValidHexagonMetricAggregations        = utils.GetKeys(DashboardSchemaToProtoHexagonAggregation)
)

type HexagonModel struct {
	CustomUnit    types.String       `tfsdk:"custom_unit"`
	LegendBy      types.String       `tfsdk:"legend_by"`
	Decimal       types.Number       `tfsdk:"decimal"`
	DataModeType  types.String       `tfsdk:"data_mode_type"`
	Thresholds    types.Set          `tfsdk:"thresholds"` //HexagonThresholdModel
	ThresholdType types.String       `tfsdk:"threshold_type"`
	Min           types.Number       `tfsdk:"min"`
	Max           types.Number       `tfsdk:"max"`
	Unit          types.String       `tfsdk:"unit"`
	Legend        *LegendModel       `tfsdk:"legend"`
	Query         *HexagonQueryModel `tfsdk:"query"`
}

type HexagonQueryModel struct {
	Logs      *HexagonQueryLogsModel    `tfsdk:"logs"`
	Metrics   *HexagonQueryMetricsModel `tfsdk:"metrics"`
	Spans     *HexagonQuerySpansModel   `tfsdk:"spans"`
	DataPrime *DataPrimeModel           `tfsdk:"data_prime"`
}

type HexagonQuerySpansModel struct {
	LuceneQuery types.String           `tfsdk:"lucene_query"`
	GroupBy     types.List             `tfsdk:"group_by"` //SpansFieldModel
	Aggregation *SpansAggregationModel `tfsdk:"aggregation"`
	Filters     types.List             `tfsdk:"filters"` //SpansFilterModel
	TimeFrame   *TimeFrameModel        `tfsdk:"time_frame"`
}

type HexagonQueryMetricsModel struct {
	PromqlQuery     types.String    `tfsdk:"promql_query"`
	Filters         types.List      `tfsdk:"filters"` //MetricsFilterModel
	PromqlQueryType types.String    `tfsdk:"promql_query_type"`
	Aggregation     types.String    `tfsdk:"aggregation"`
	TimeFrame       *TimeFrameModel `tfsdk:"time_frame"`
}

type HexagonQueryLogsModel struct {
	LuceneQuery types.String          `tfsdk:"lucene_query"`
	GroupBy     types.List            `tfsdk:"group_by"` //ObservationFieldModel
	Aggregation *LogsAggregationModel `tfsdk:"aggregation"`
	Filters     types.List            `tfsdk:"filters"` //LogsFilterModel
	TimeFrame   *TimeFrameModel       `tfsdk:"time_frame"`
}

type HexagonThresholdModel struct {
	From  types.Number `tfsdk:"from"`
	Color types.String `tfsdk:"color"`
	Label types.String `tfsdk:"label"`
}

func HexagonSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"min": schema.NumberAttribute{
				Optional: true,
			},
			"max": schema.NumberAttribute{
				Optional: true,
			},
			"decimal": schema.NumberAttribute{
				Optional: true,
			},
			"legend": LegendSchema(),
			"legend_by": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(utils.UNSPECIFIED),
				Validators: []validator.String{
					stringvalidator.OneOf(DashboardValidLegendBys...),
				},
				MarkdownDescription: fmt.Sprintf("The legend by. Valid values are: %s.", strings.Join(DashboardValidLegendBys, ", ")),
			},
			"unit": UnitSchema(),
			"custom_unit": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "A custom unit",
			},
			"data_mode_type": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Validators: []validator.String{
					stringvalidator.OneOf(DashboardValidDataModeTypes...),
				},
				Default: stringdefault.StaticString(utils.UNSPECIFIED),
			},
			"thresholds": schema.SetNestedAttribute{
				Optional: true,
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"from": schema.NumberAttribute{
							Required: true,
						},
						"color": schema.StringAttribute{
							Optional: true,
						},
						"label": schema.StringAttribute{
							Optional: true,
						},
					},
				},
			},
			"threshold_type": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Validators: []validator.String{
					stringvalidator.OneOf(DashboardValidThresholdTypes...),
				},
				Default:             stringdefault.StaticString(utils.UNSPECIFIED),
				MarkdownDescription: fmt.Sprintf("The threshold type. Valid values are: %s.", strings.Join(DashboardValidThresholdTypes, ", ")),
			},
			"query": schema.SingleNestedAttribute{
				Required: true,
				Attributes: map[string]schema.Attribute{
					"logs": schema.SingleNestedAttribute{
						Attributes: map[string]schema.Attribute{
							"lucene_query": schema.StringAttribute{
								Optional: true,
							},
							"group_by": schema.ListNestedAttribute{
								NestedObject: schema.NestedAttributeObject{
									Attributes: ObservationFieldSchema(),
								},
								Optional: true,
							},
							"filters":     LogsFiltersSchema(),
							"aggregation": LogsAggregationSchema(),
							"time_frame":  TimeFrameSchema(),
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
							"promql_query_type": schema.StringAttribute{
								Optional: true,
								Computed: true,
								Default:  stringdefault.StaticString(utils.UNSPECIFIED),
								Validators: []validator.String{
									stringvalidator.OneOf(DashboardValidPromQLQueryType...),
								},
							},
							"filters": MetricFiltersSchema(),
							"aggregation": schema.StringAttribute{
								Optional: true,
								Computed: true,
								Default:  stringdefault.StaticString(utils.UNSPECIFIED),
								Validators: []validator.String{
									stringvalidator.OneOf(DashboardValidHexagonMetricAggregations...),
								},
							},
							"time_frame": TimeFrameSchema(),
						},
						Optional: true,
					},
					"spans": schema.SingleNestedAttribute{
						Attributes: map[string]schema.Attribute{
							"lucene_query": schema.StringAttribute{
								Optional: true,
							},
							"group_by":    SpansFieldsSchema(),
							"aggregation": SpansAggregationSchema(),
							"filters":     SpansFilterSchema(),
							"time_frame":  TimeFrameSchema(),
						},
						Optional: true,
					},
					"data_prime": schema.SingleNestedAttribute{
						Attributes: map[string]schema.Attribute{
							"query": schema.StringAttribute{
								Optional: true,
							},
							"filters": schema.ListNestedAttribute{
								NestedObject: schema.NestedAttributeObject{
									Attributes: FiltersSourceSchema(),
								},
								Optional: true,
							},
							"time_frame": TimeFrameSchema(),
						},
						Optional: true,
					},
				},
			},
		},
		Validators: []validator.Object{
			SupportedWidgetsValidatorWithout("hexagon"),
			objectvalidator.AlsoRequires(
				path.MatchRelative().AtParent().AtParent().AtName("title"),
			),
		},
	}
}
func HexagonSchemaV0() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"min": schema.NumberAttribute{
				Optional: true,
			},
			"max": schema.NumberAttribute{
				Optional: true,
			},
			"decimal": schema.NumberAttribute{
				Optional: true,
			},
			"legend": LegendSchema(),
			"legend_by": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(utils.UNSPECIFIED),
				Validators: []validator.String{
					stringvalidator.OneOf(DashboardValidLegendBys...),
				},
				MarkdownDescription: fmt.Sprintf("The legend by. Valid values are: %s.", strings.Join(DashboardValidLegendBys, ", ")),
			},
			"unit": UnitSchema(),
			"custom_unit": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "A custom unit",
			},
			"data_mode_type": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Validators: []validator.String{
					stringvalidator.OneOf(DashboardValidDataModeTypes...),
				},
				Default: stringdefault.StaticString(utils.UNSPECIFIED),
			},
			"thresholds": schema.SetNestedAttribute{
				Optional: true,
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"from": schema.NumberAttribute{
							Required: true,
						},
						"color": schema.StringAttribute{
							Optional: true,
						},
						"label": schema.StringAttribute{
							Optional: true,
						},
					},
				},
			},
			"threshold_type": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Validators: []validator.String{
					stringvalidator.OneOf(DashboardValidThresholdTypes...),
				},
				Default:             stringdefault.StaticString(utils.UNSPECIFIED),
				MarkdownDescription: fmt.Sprintf("The threshold type. Valid values are: %s.", strings.Join(DashboardValidThresholdTypes, ", ")),
			},
			"time_frame": TimeFrameSchema(),

			"query": schema.SingleNestedAttribute{
				Required: true,
				Attributes: map[string]schema.Attribute{
					"logs": schema.SingleNestedAttribute{
						Attributes: map[string]schema.Attribute{
							"lucene_query": schema.StringAttribute{
								Optional: true,
							},
							"group_by": schema.ListNestedAttribute{
								NestedObject: schema.NestedAttributeObject{
									Attributes: ObservationFieldSchema(),
								},
								Optional: true,
							},
							"filters":     LogsFiltersSchema(),
							"aggregation": LogsAggregationSchema(),
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
							"promql_query_type": schema.StringAttribute{
								Optional: true,
								Computed: true,
								Default:  stringdefault.StaticString(utils.UNSPECIFIED),
							},
							"filters": MetricFiltersSchema(),
							"aggregation": schema.StringAttribute{
								Optional: true,
								Computed: true,
								Default:  stringdefault.StaticString(utils.UNSPECIFIED),
								Validators: []validator.String{
									stringvalidator.OneOf(DashboardValidHexagonMetricAggregations...),
								},
							},
						},
						Optional: true,
					},
					"spans": schema.SingleNestedAttribute{
						Attributes: map[string]schema.Attribute{
							"lucene_query": schema.StringAttribute{
								Optional: true,
							},
							"group_by":    SpansFieldsSchema(),
							"aggregation": SpansAggregationSchema(),
							"filters":     SpansFilterSchema(),
							"time_frame":  TimeFrameSchema(),
						},
						Optional: true,
					},
					"data_prime": schema.SingleNestedAttribute{
						Attributes: map[string]schema.Attribute{
							"query": schema.StringAttribute{
								Optional: true,
							},
							"filters": schema.ListNestedAttribute{
								NestedObject: schema.NestedAttributeObject{
									Attributes: FiltersSourceSchema(),
								},
								Optional: true,
							},
						},
						Optional: true,
					},
				},
			},
		},
		Validators: []validator.Object{
			SupportedWidgetsValidatorWithout("hexagon"),
			objectvalidator.AlsoRequires(
				path.MatchRelative().AtParent().AtParent().AtName("title"),
			),
		},
	}
}

func HexagonType() types.ObjectType {
	return types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"min":     types.NumberType,
			"max":     types.NumberType,
			"decimal": types.NumberType,
			"legend": types.ObjectType{
				AttrTypes: LegendAttr(),
			},
			"legend_by":      types.StringType,
			"unit":           types.StringType,
			"custom_unit":    types.StringType,
			"data_mode_type": types.StringType,
			"thresholds": types.SetType{
				ElemType: types.ObjectType{
					AttrTypes: ThresholdAttr(),
				},
			},
			"threshold_type": types.StringType,
			"query": types.ObjectType{
				AttrTypes: map[string]attr.Type{
					"logs": types.ObjectType{
						AttrTypes: map[string]attr.Type{
							"lucene_query": types.StringType,
							"filters": types.ListType{
								ElemType: types.ObjectType{
									AttrTypes: LogsFilterModelAttr(),
								},
							},
							"aggregation": types.ObjectType{
								AttrTypes: AggregationModelAttr(),
							},
							"group_by": types.ListType{
								ElemType: ObservationFieldsObject(),
							},
							"time_frame": types.ObjectType{
								AttrTypes: TimeFrameModelAttr(),
							},
						},
					},
					"metrics": types.ObjectType{
						AttrTypes: map[string]attr.Type{
							"promql_query":      types.StringType,
							"promql_query_type": types.StringType,
							"filters": types.ListType{
								ElemType: types.ObjectType{
									AttrTypes: MetricsFilterModelAttr(),
								},
							},
							"aggregation": types.StringType,
							"time_frame": types.ObjectType{
								AttrTypes: TimeFrameModelAttr(),
							},
						},
					},
					"spans": types.ObjectType{
						AttrTypes: map[string]attr.Type{
							"lucene_query": types.StringType,
							"filters": types.ListType{
								ElemType: types.ObjectType{
									AttrTypes: SpansFilterModelAttr(),
								},
							},
							"group_by": types.ListType{
								ElemType: types.ObjectType{
									AttrTypes: SpansFieldModelAttr(),
								},
							},
							"aggregation": types.ObjectType{
								AttrTypes: SpansAggregationModelAttr(),
							},
							"time_frame": types.ObjectType{
								AttrTypes: TimeFrameModelAttr(),
							},
						},
					},
					"data_prime": types.ObjectType{
						AttrTypes: map[string]attr.Type{
							"query": types.StringType,
							"filters": types.ListType{
								ElemType: types.ObjectType{
									AttrTypes: FilterSourceModelAttr(),
								},
							},
							"time_frame": types.ObjectType{
								AttrTypes: TimeFrameModelAttr(),
							},
						},
					},
				},
			},
		},
	}
}

func FlattenHexagon(ctx context.Context, hexagon *dashboardservice.Hexagon) (*WidgetDefinitionModel, diag.Diagnostics) {
	if hexagon == nil {
		return nil, nil
	}

	query, diags := flattenHexagonQuery(ctx, hexagon.Query)
	if diags.HasError() {
		return nil, diags
	}

	thresholds, diags := flattenThresholds(ctx, hexagon.GetThresholds())
	if diags.HasError() {
		return nil, diags
	}

	return &WidgetDefinitionModel{
		Hexagon: &HexagonModel{
			Legend:        FlattenLegend(hexagon.Legend),
			Query:         query,
			Min:           float64PointerToNumberType(hexagon.Min),
			Max:           float64PointerToNumberType(hexagon.Max),
			CustomUnit:    utils.StringPointerToTypeString(hexagon.CustomUnit),
			Decimal:       int32PointerToNumberType(hexagon.Decimal),
			LegendBy:      basetypes.NewStringValue(DashboardProtoToSchemaLegendBy[hexagon.GetLegendBy()]),
			Unit:          basetypes.NewStringValue(DashboardProtoToSchemaUnit[hexagon.GetUnit()]),
			DataModeType:  basetypes.NewStringValue(DashboardProtoToSchemaDataModeType[hexagon.GetDataModeType()]),
			ThresholdType: basetypes.NewStringValue(DashboardProtoToSchemaThresholdType[hexagon.GetThresholdType()]),
			Thresholds:    thresholds,
		},
	}, nil
}

func flattenThresholds(ctx context.Context, set []dashboardservice.CommonThreshold) (types.Set, diag.Diagnostics) {
	if set == nil {
		return types.SetNull(types.ObjectType{AttrTypes: ThresholdAttr()}), nil
	}
	var diagnostics diag.Diagnostics

	thresholds := make([]attr.Value, 0, len(set))
	for _, threshold := range set {
		threshold := HexagonThresholdModel{
			From:  float64PointerToNumberType(threshold.From),
			Color: utils.StringPointerToTypeString(threshold.Color),
			Label: utils.StringPointerToTypeString(threshold.Label),
		}
		t, diags := types.ObjectValueFrom(ctx, ThresholdAttr(), threshold)
		if diags.HasError() {
			diagnostics.Append(diags...)
			continue
		}
		thresholds = append(thresholds, t)
	}

	if diagnostics.HasError() {
		return types.SetNull(types.ObjectType{
			AttrTypes: ThresholdAttr(),
		}), diagnostics
	}

	return types.SetValueFrom(ctx, types.ObjectType{AttrTypes: ThresholdAttr()}, thresholds)
}

func flattenHexagonQuery(ctx context.Context, query *dashboardservice.HexagonQuery) (*HexagonQueryModel, diag.Diagnostics) {
	if query == nil {
		return nil, nil
	}

	switch {
	case query.Logs != nil:
		return flattenHexagonLogsQuery(ctx, query.Logs)
	case query.Metrics != nil:
		return flattenHexagonMetricsQuery(ctx, query.Metrics)
	case query.Spans != nil:
		return flattenHexagonSpansQuery(ctx, query.Spans)
	case query.Dataprime != nil:
		return flattenHexagonDataPrimeQuery(ctx, query.Dataprime)
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Hexagon Query", "unknown Hexagon query type")}
	}
}

func flattenHexagonDataPrimeQuery(ctx context.Context, dataPrime *dashboardservice.HexagonDataprimeQuery) (*HexagonQueryModel, diag.Diagnostics) {
	if dataPrime == nil {
		return nil, nil
	}

	dataPrimeQuery := types.StringNull()
	if dataPrime.DataprimeQuery != nil && dataPrime.DataprimeQuery.Text != nil {
		dataPrimeQuery = types.StringPointerValue(dataPrime.DataprimeQuery.Text)
	}

	filters, diags := FlattenDashboardFiltersSources(ctx, dataPrime.GetFilters())
	if diags.HasError() {
		return nil, diags
	}
	timeframe, diags := FlattenTimeFrameSelect(ctx, dataPrime.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	return &HexagonQueryModel{
		DataPrime: &DataPrimeModel{
			Query:     dataPrimeQuery,
			Filters:   filters,
			TimeFrame: timeframe,
		},
	}, nil
}

func flattenHexagonLogsQuery(ctx context.Context, logs *dashboardservice.HexagonLogsQuery) (*HexagonQueryModel, diag.Diagnostics) {
	if logs == nil {
		return nil, nil
	}

	filters, diags := FlattenLogsFilters(ctx, logs.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	grouping, diags := FlattenObservationFields(ctx, logs.GetGroupBy())
	if diags.HasError() {
		return nil, diags
	}
	aggregation, diags := FlattenLogsAggregation(ctx, logs.LogsAggregation)
	if diags.HasError() {
		return nil, diags
	}

	timeframe, diags := FlattenTimeFrameSelect(ctx, logs.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	return &HexagonQueryModel{
		Logs: &HexagonQueryLogsModel{
			LuceneQuery: flattenLuceneQuery(logs.LuceneQuery),
			Filters:     filters,
			GroupBy:     grouping,
			Aggregation: aggregation,
			TimeFrame:   timeframe,
		},
	}, nil
}

func flattenHexagonMetricsQuery(ctx context.Context, metrics *dashboardservice.HexagonMetricsQuery) (*HexagonQueryModel, diag.Diagnostics) {
	if metrics == nil {
		return nil, nil
	}

	filters, diags := FlattenMetricsFilters(ctx, metrics.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	timeframe, diags := FlattenTimeFrameSelect(ctx, metrics.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	return &HexagonQueryModel{
		Metrics: &HexagonQueryMetricsModel{
			PromqlQuery:     flattenPromqlQuery(metrics.PromqlQuery),
			Filters:         filters,
			PromqlQueryType: types.StringValue(DashboardProtoToSchemaPromQLQueryType[metrics.GetPromqlQueryType()]),
			Aggregation:     types.StringValue(DashboardProtoToSchemaHexagonMetricAggregation[metrics.GetAggregation()]),
			TimeFrame:       timeframe,
		},
	}, nil
}

func flattenHexagonSpansQuery(ctx context.Context, spans *dashboardservice.HexagonSpansQuery) (*HexagonQueryModel, diag.Diagnostics) {
	if spans == nil {
		return nil, nil
	}

	filters, diags := FlattenSpansFilters(ctx, spans.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	grouping, diags := FlattenSpansFields(ctx, spans.GetGroupBy())
	if diags.HasError() {
		return nil, diags
	}

	timeframe, diags := FlattenTimeFrameSelect(ctx, spans.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	aggregation, dg := FlattenSpansAggregation(spans.SpansAggregation)
	if dg != nil {
		diags.Append(dg)
		return nil, diags
	}

	return &HexagonQueryModel{
		Spans: &HexagonQuerySpansModel{
			LuceneQuery: flattenLuceneQuery(spans.LuceneQuery),
			Filters:     filters,
			GroupBy:     grouping,
			Aggregation: aggregation,
			TimeFrame:   timeframe,
		},
	}, nil
}

func ExpandHexagon(ctx context.Context, hexagon *HexagonModel) (*dashboardservice.WidgetDefinition, diag.Diagnostics) {
	if hexagon == nil {
		return nil, nil
	}

	thresholds, diags := expandThresholds(ctx, hexagon.Thresholds)
	if diags.HasError() {
		return nil, diags
	}

	legend, diags := ExpandLegend(ctx, hexagon.Legend)
	if diags.HasError() {
		return nil, diags
	}

	query, diags := expandHexagonQuery(ctx, hexagon.Query)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.WidgetDefinition{
		Hexagon: &dashboardservice.Hexagon{
			Min:           numberTypeToFloat64Pointer(hexagon.Min),
			Max:           numberTypeToFloat64Pointer(hexagon.Max),
			CustomUnit:    utils.TypeStringToStringPointer(hexagon.CustomUnit),
			Decimal:       numberTypeToInt32Pointer(hexagon.Decimal),
			LegendBy:      OptionalEnumPointer(hexagon.LegendBy, DashboardSchemaToProtoLegendBy),
			ThresholdType: OptionalEnumPointer(hexagon.ThresholdType, DashboardSchemaToProtoThresholdType),
			Unit:          OptionalEnumPointer(hexagon.Unit, DashboardSchemaToProtoUnit),
			DataModeType:  OptionalEnumPointer(hexagon.DataModeType, DashboardSchemaToProtoDataModeType),
			Thresholds:    thresholds,
			Legend:        legend,
			Query:         query,
		},
	}, nil
}

func expandThresholds(ctx context.Context, set types.Set) ([]dashboardservice.CommonThreshold, diag.Diagnostics) {
	thresholds := make([]dashboardservice.CommonThreshold, 0)
	if set.IsNull() || set.IsUnknown() {
		return thresholds, nil
	}

	var thresholdElementObjs []types.Object
	diags := set.ElementsAs(ctx, &thresholdElementObjs, true)
	if diags.HasError() {
		return nil, diags
	}

	var diagnostics diag.Diagnostics
	for _, obj := range thresholdElementObjs {
		var threshold HexagonThresholdModel
		if dg := obj.As(ctx, &threshold, basetypes.ObjectAsOptions{}); dg.HasError() {
			diagnostics.Append(dg...)
			continue
		}

		thresholds = append(thresholds, dashboardservice.CommonThreshold{
			From:  numberTypeToFloat64Pointer(threshold.From),
			Color: utils.TypeStringToStringPointer(threshold.Color),
			Label: utils.TypeStringToStringPointer(threshold.Label),
		})
	}

	if diagnostics.HasError() {
		return nil, diagnostics
	}

	return thresholds, nil
}

func expandHexagonQuery(ctx context.Context, hexagonQuery *HexagonQueryModel) (*dashboardservice.HexagonQuery, diag.Diagnostics) {
	if hexagonQuery == nil {
		return nil, nil
	}

	switch {
	case hexagonQuery.Metrics != nil:
		metrics, diags := expandHexagonMetricsQuery(ctx, hexagonQuery.Metrics)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboardservice.HexagonQuery{
			Metrics: metrics,
		}, nil
	case hexagonQuery.Logs != nil:
		logs, diags := expandHexagonLogsQuery(ctx, hexagonQuery.Logs)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboardservice.HexagonQuery{
			Logs: logs,
		}, nil
	case hexagonQuery.Spans != nil:
		spans, diags := expandHexagonSpansQuery(ctx, hexagonQuery.Spans)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboardservice.HexagonQuery{
			Spans: spans,
		}, nil
	case hexagonQuery.DataPrime != nil:
		dataPrime, diags := expandHexagonDataPrimeQuery(ctx, hexagonQuery.DataPrime)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboardservice.HexagonQuery{
			Dataprime: dataPrime,
		}, nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand Hexagon Query", fmt.Sprintf("unknown data hexagon type %#v", hexagonQuery))}
	}
}

func expandHexagonDataPrimeQuery(ctx context.Context, dataPrime *DataPrimeModel) (*dashboardservice.HexagonDataprimeQuery, diag.Diagnostics) {
	if dataPrime == nil {
		return nil, nil
	}

	filters, diags := ExpandDashboardFiltersSources(ctx, dataPrime.Filters)
	if diags.HasError() {
		return nil, diags
	}

	var dataPrimeQuery *dashboardservice.CommonDataprimeQuery
	if !dataPrime.Query.IsNull() {
		dataPrimeQuery = &dashboardservice.CommonDataprimeQuery{
			Text: dataPrime.Query.ValueStringPointer(),
		}
	}

	timeframe, diags := ExpandTimeFrameSelect(ctx, dataPrime.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.HexagonDataprimeQuery{
		DataprimeQuery: dataPrimeQuery,
		Filters:        filters,
		TimeFrame:      timeframe,
	}, nil
}

func expandHexagonMetricsQuery(ctx context.Context, queryMetrics *HexagonQueryMetricsModel) (*dashboardservice.HexagonMetricsQuery, diag.Diagnostics) {
	if queryMetrics == nil {
		return nil, nil
	}

	filters, diags := ExpandMetricsFilters(ctx, queryMetrics.Filters)
	if diags.HasError() {
		return nil, diags
	}

	timeframe, diags := ExpandTimeFrameSelect(ctx, queryMetrics.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.HexagonMetricsQuery{
		PromqlQuery:     ExpandPromqlQuery(queryMetrics.PromqlQuery),
		Filters:         filters,
		TimeFrame:       timeframe,
		PromqlQueryType: OptionalEnumPointer(queryMetrics.PromqlQueryType, DashboardSchemaToProtoPromQLQueryType),
		Aggregation:     OptionalEnumPointer(queryMetrics.Aggregation, DashboardSchemaToProtoHexagonAggregation),
	}, nil
}

func expandHexagonLogsQuery(ctx context.Context, queryLogs *HexagonQueryLogsModel) (*dashboardservice.HexagonLogsQuery, diag.Diagnostics) {
	if queryLogs == nil {
		return nil, nil
	}

	filters, diags := ExpandLogsFilters(ctx, queryLogs.Filters)
	if diags.HasError() {
		return nil, diags
	}

	aggregation, diags := ExpandLogsAggregation(ctx, queryLogs.Aggregation)
	if diags.HasError() {
		return nil, diags
	}

	groupBys, diags := ExpandObservationFields(ctx, queryLogs.GroupBy)
	if diags.HasError() {
		return nil, diags
	}

	timeframe, diags := ExpandTimeFrameSelect(ctx, queryLogs.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.HexagonLogsQuery{
		LuceneQuery:     ExpandLuceneQuery(queryLogs.LuceneQuery),
		Filters:         filters,
		LogsAggregation: aggregation,
		GroupBy:         groupBys,
		TimeFrame:       timeframe,
	}, nil
}

func expandHexagonSpansQuery(ctx context.Context, hexagonQuerySpans *HexagonQuerySpansModel) (*dashboardservice.HexagonSpansQuery, diag.Diagnostics) {
	if hexagonQuerySpans == nil {
		return nil, nil
	}

	filters, diags := ExpandSpansFilters(ctx, hexagonQuerySpans.Filters)
	if diags.HasError() {
		return nil, diags
	}

	grouping, diags := ExpandSpansFields(ctx, hexagonQuerySpans.GroupBy)
	if diags.HasError() {
		return nil, diags
	}

	timeframe, diags := ExpandTimeFrameSelect(ctx, hexagonQuerySpans.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	aggregation, dg := ExpandSpansAggregation(hexagonQuerySpans.Aggregation)
	if dg != nil {
		diags.Append(dg)
		return nil, diags
	}

	return &dashboardservice.HexagonSpansQuery{
		LuceneQuery:      ExpandLuceneQuery(hexagonQuerySpans.LuceneQuery),
		Filters:          filters,
		GroupBy:          grouping,
		SpansAggregation: aggregation,
		TimeFrame:        timeframe,
	}, nil
}

func flattenLuceneQuery(query *dashboardservice.LuceneQuery) types.String {
	if query == nil {
		return types.StringNull()
	}
	return utils.StringPointerToTypeString(query.Value)
}

func flattenPromqlQuery(query *dashboardservice.PromQlQuery) types.String {
	if query == nil {
		return types.StringNull()
	}
	return utils.StringPointerToTypeString(query.Value)
}
