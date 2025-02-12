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
	"log"
	"strings"
	"terraform-provider-coralogix/coralogix/utils"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"

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
	DashboardSchemaToProtoHexagonAggregation = map[string]cxsdk.HexagonMetricAggregation{
		"unspecified": cxsdk.HexagonMetricAggregationUnspecified,
		"last":        cxsdk.HexagonMetricAggregationLast,
		"min":         cxsdk.HexagonMetricAggregationMin,
		"max":         cxsdk.HexagonMetricAggregationMax,
		"avg":         cxsdk.HexagonMetricAggregationAvg,
		"sum":         cxsdk.HexagonMetricAggregationSum,
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
	TimeFrame     *TimeFrameModel    `tfsdk:"time_frame"`
}

type HexagonQueryModel struct {
	Logs      *HexagonQueryLogsModel `tfsdk:"logs"`
	Metrics   *QueryMetricsModel     `tfsdk:"metrics"`
	Spans     *QuerySpansModel       `tfsdk:"spans"`
	DataPrime *DataPrimeModel        `tfsdk:"data_prime"`
}

type HexagonQueryLogsModel struct {
	LuceneQuery types.String          `tfsdk:"lucene_query"`
	GroupBy     types.List            `tfsdk:"group_by"`    //ObservationFieldModel
	Aggregation *LogsAggregationModel `tfsdk:"aggregation"` //AggregationModel
	Filters     types.List            `tfsdk:"filters"`     //FilterModel
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
				Default:  stringdefault.StaticString("unspecified"),
				Validators: []validator.String{
					stringvalidator.OneOf(DashboardValidLegendBys...),
				},
				MarkdownDescription: fmt.Sprintf("The legend by. Valid values are: %s.", strings.Join(DashboardValidLegendBys, ", ")),
			},
			"unit": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString("unspecified"),
				Validators: []validator.String{
					stringvalidator.OneOf(DashboardValidUnits...),
				},
				MarkdownDescription: fmt.Sprintf("The unit. Valid values are: %s.", strings.Join(DashboardValidUnits, ", ")),
			},
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
				Default: stringdefault.StaticString("unspecified"),
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
				Default:             stringdefault.StaticString("unspecified"),
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
								Default:  stringdefault.StaticString(UNSPECIFIED),
							},
							"filters": MetricFiltersSchema(),
							"aggregation": schema.StringAttribute{
								Optional: true,
								Computed: true,
								Default:  stringdefault.StaticString("unspecified"),
								Validators: []validator.String{
									stringvalidator.OneOf(DashboardValidHexagonMetricAggregations...),
								},
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
							"group_by":    SpansFieldsSchema(),
							"aggregation": SpansAggregationSchema(),
							"filters":     SpansFilterSchema(),
						},
						Optional: true,
						Validators: []validator.Object{
							objectvalidator.ExactlyOneOf(
								path.MatchRelative().AtParent().AtName("metrics"),
								path.MatchRelative().AtParent().AtName("logs"),
								path.MatchRelative().AtParent().AtName("data_prime"),
							),
						},
					},
					"data_prime": schema.SingleNestedAttribute{
						Attributes: map[string]schema.Attribute{
							"dataprime_query": schema.StringAttribute{
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
						Validators: []validator.Object{
							objectvalidator.ExactlyOneOf(
								path.MatchRelative().AtParent().AtName("metrics"),
								path.MatchRelative().AtParent().AtName("spans"),
								path.MatchRelative().AtParent().AtName("logs"),
							),
						},
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
					AttrTypes: map[string]attr.Type{
						"from":  types.NumberType,
						"color": types.StringType,
						"label": types.StringType,
					},
				},
			},
			"threshold_type": types.StringType,
			"time_frame": types.ObjectType{
				AttrTypes: TimeFrameModelAttr(),
			},
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
								AttrTypes: AggregationModelAttr(), // something is odd with this. maybe needs its own flattening entirely.
							},
							"group_by": types.ListType{
								ElemType: ObservationFieldsObject(),
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
						},
					},
					"data_prime": types.ObjectType{
						AttrTypes: map[string]attr.Type{
							"dataprime_query": types.StringType,
							"filters": types.ListType{
								ElemType: types.ObjectType{
									AttrTypes: FilterSourceModelAttr(),
								},
							},
						},
					},
				},
			},
		},
	}
}

func FlattenHexagon(ctx context.Context, chart *cxsdk.Hexagon) (*WidgetDefinitionModel, diag.Diagnostics) {
	if chart == nil {
		return nil, nil
	}

	query, timeframe, diags := flattenHexagonQuery(ctx, chart.GetQuery())
	if diags.HasError() {
		return nil, diags
	}

	thresholds, diags := flattenThresholds(ctx, chart.Thresholds)
	if diags.HasError() {
		return nil, diags
	}

	return &WidgetDefinitionModel{
		Hexagon: &HexagonModel{
			Legend:        FlattenLegend(chart.GetLegend()),
			Query:         query,
			Min:           utils.WrapperspbDoubleToNumberType(chart.Min),
			Max:           utils.WrapperspbDoubleToNumberType(chart.Max),
			CustomUnit:    utils.WrapperspbStringToTypeString(chart.CustomUnit),
			Decimal:       utils.WrapperspbInt32ToNumberType(chart.Decimal),
			LegendBy:      basetypes.NewStringValue(DashboardProtoToSchemaLegendBy[chart.LegendBy]),
			Unit:          basetypes.NewStringValue(DashboardProtoToSchemaUnit[chart.Unit]),
			DataModeType:  basetypes.NewStringValue(DashboardProtoToSchemaDataModeType[chart.DataModeType]),
			ThresholdType: basetypes.NewStringValue(DashboardProtoToSchemaThresholdType[chart.ThresholdType]),
			Thresholds:    thresholds,
			TimeFrame:     timeframe,
		},
	}, nil
}

func flattenThresholds(ctx context.Context, set []*cxsdk.Threshold) (types.Set, diag.Diagnostics) {
	if set == nil {
		return types.SetNull(types.ObjectType{AttrTypes: ThresholdAttr()}), nil
	}
	var diagnostics diag.Diagnostics

	thresholds := make([]attr.Value, 0, len(set))
	for _, threshold := range set {
		threshold := HexagonThresholdModel{
			From:  utils.WrapperspbDoubleToNumberType(threshold.From),
			Color: utils.WrapperspbStringToTypeString(threshold.Color),
			Label: utils.WrapperspbStringToTypeString(threshold.Label),
		}
		t, diags := types.ObjectValueFrom(ctx, ThresholdAttr(), threshold)
		if diags.HasError() {
			diagnostics.Append(diags...)
			continue
		}
		thresholds = append(thresholds, t)
	}
	return types.SetValueMust(types.ObjectType{AttrTypes: ThresholdAttr()}, thresholds), nil
}

func flattenHexagonQuery(ctx context.Context, query *cxsdk.HexagonQuery) (*HexagonQueryModel, *TimeFrameModel, diag.Diagnostics) {
	if query == nil {
		return nil, nil, nil
	}

	switch query.GetValue().(type) {
	case *cxsdk.HexagonQueryLogs:
		return flattenHexagonLogsQuery(ctx, query.GetLogs())
	case *cxsdk.HexagonQueryMetrics:
		return flattenHexagonMetricsQuery(ctx, query.GetMetrics())
	case *cxsdk.HexagonQuerySpans:
		return flattenHexagonSpansQuery(ctx, query.GetSpans())
	case *cxsdk.HexagonQueryDataprime:
		return flattenHexagonDataPrimeQuery(ctx, query.GetDataprime())
	default:
		return nil, nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Data Table Query", "unknown data table query type")}
	}
}

func flattenHexagonDataPrimeQuery(ctx context.Context, dataPrime *cxsdk.HexagonDataprimeQuery) (*HexagonQueryModel, *TimeFrameModel, diag.Diagnostics) {
	if dataPrime == nil {
		return nil, nil, nil
	}

	dataPrimeQuery := types.StringNull()
	if dataPrime.GetDataprimeQuery() != nil {
		dataPrimeQuery = types.StringValue(dataPrime.GetDataprimeQuery().GetText())
	}

	filters, diags := FlattenDashboardFiltersSources(ctx, dataPrime.GetFilters())
	if diags.HasError() {
		return nil, nil, diags
	}
	timeframe, diags := FlattenTimeFrameSelect(ctx, dataPrime.TimeFrame)
	if diags.HasError() {
		return nil, nil, diags
	}

	return &HexagonQueryModel{
		DataPrime: &DataPrimeModel{
			Query:   dataPrimeQuery,
			Filters: filters,
		},
	}, timeframe, nil
}

func flattenHexagonLogsQuery(ctx context.Context, logs *cxsdk.HexagonLogsQuery) (*HexagonQueryModel, *TimeFrameModel, diag.Diagnostics) {
	if logs == nil {
		return nil, nil, nil
	}

	filters, diags := FlattenLogsFilters(ctx, logs.GetFilters())
	if diags.HasError() {
		return nil, nil, diags
	}

	grouping, diags := FlattenObservationFields(ctx, logs.GetGroupBy())
	if diags.HasError() {
		return nil, nil, diags
	}
	aggregation, diags := FlattenLogsAggregation(ctx, logs.LogsAggregation)
	if diags.HasError() {
		return nil, nil, diags
	}

	timeframe, diags := FlattenTimeFrameSelect(ctx, logs.TimeFrame)
	if diags.HasError() {
		return nil, nil, diags
	}

	return &HexagonQueryModel{
		Logs: &HexagonQueryLogsModel{
			LuceneQuery: utils.WrapperspbStringToTypeString(logs.GetLuceneQuery().GetValue()),
			Filters:     filters,
			GroupBy:     grouping,
			Aggregation: aggregation,
		},
	}, timeframe, nil
}

func flattenHexagonMetricsQuery(ctx context.Context, metrics *cxsdk.HexagonMetricsQuery) (*HexagonQueryModel, *TimeFrameModel, diag.Diagnostics) {
	if metrics == nil {
		return nil, nil, nil
	}

	filters, diags := FlattenMetricsFilters(ctx, metrics.GetFilters())
	if diags.HasError() {
		return nil, nil, diags
	}

	timeframe, diags := FlattenTimeFrameSelect(ctx, metrics.TimeFrame)
	if diags.HasError() {
		return nil, nil, diags
	}

	return &HexagonQueryModel{
		Metrics: &QueryMetricsModel{
			PromqlQuery:     utils.WrapperspbStringToTypeString(metrics.GetPromqlQuery().GetValue()),
			Filters:         filters,
			PromqlQueryType: types.StringValue(DashboardProtoToSchemaPromQLQueryType[metrics.PromqlQueryType]),
		},
	}, timeframe, nil
}

func flattenHexagonSpansQuery(ctx context.Context, spans *cxsdk.HexagonSpansQuery) (*HexagonQueryModel, *TimeFrameModel, diag.Diagnostics) {
	if spans == nil {
		return nil, nil, nil
	}

	filters, diags := FlattenSpansFilters(ctx, spans.GetFilters())
	if diags.HasError() {
		return nil, nil, diags
	}

	grouping, diags := FlattenSpansFields(ctx, spans.GetGroupBy())
	if diags.HasError() {
		return nil, nil, diags
	}

	timeframe, diags := FlattenTimeFrameSelect(ctx, spans.TimeFrame)
	if diags.HasError() {
		return nil, nil, diags
	}

	return &HexagonQueryModel{
		Spans: &QuerySpansModel{
			LuceneQuery: utils.WrapperspbStringToTypeString(spans.GetLuceneQuery().GetValue()),
			Filters:     filters,
			GroupBy:     grouping,
		},
	}, timeframe, nil
}

func ExpandHexagon(ctx context.Context, hexagon *HexagonModel) (*cxsdk.WidgetDefinition, diag.Diagnostics) {
	log.Printf("[INFO] Expanding Hexagon")

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

	timeframe, diags := ExpandTimeFrameSelect(ctx, hexagon.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}
	query, diags := expandHexagonQuery(ctx, hexagon.Query, timeframe)
	if diags.HasError() {
		return nil, diags
	}
	return &cxsdk.WidgetDefinition{
		Value: &cxsdk.WidgetDefinitionHexagon{
			Hexagon: &cxsdk.Hexagon{
				Min:           utils.NumberTypeToWrapperspbDouble(hexagon.Min),
				Max:           utils.NumberTypeToWrapperspbDouble(hexagon.Max),
				CustomUnit:    utils.TypeStringToWrapperspbString(hexagon.CustomUnit),
				Decimal:       utils.NumberTypeToWrapperspbInt32(hexagon.Decimal),
				LegendBy:      DashboardSchemaToProtoLegendBy[hexagon.LegendBy.ValueString()],
				ThresholdType: DashboardSchemaToProtoThresholdType[hexagon.ThresholdType.ValueString()],
				Unit:          DashboardSchemaToProtoUnit[hexagon.Unit.ValueString()],
				DataModeType:  DashboardSchemaToProtoDataModeType[hexagon.DataModeType.ValueString()],
				Thresholds:    thresholds,
				Legend:        legend,
				Query:         query,
			}}}, nil
}

func expandThresholds(ctx context.Context, set types.Set) ([]*cxsdk.Threshold, diag.Diagnostics) {
	thresholds := make([]*cxsdk.Threshold, 0)
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

		thresholds = append(thresholds, &cxsdk.Threshold{
			From:  utils.NumberTypeToWrapperspbDouble(threshold.From),
			Color: utils.TypeStringToWrapperspbString(threshold.Color),
			Label: utils.TypeStringToWrapperspbString(threshold.Label),
		})
	}
	if diagnostics.HasError() {
		return nil, diagnostics
	}
	return thresholds, nil
}

func expandHexagonQuery(ctx context.Context, dataTableQuery *HexagonQueryModel, timeframe *cxsdk.TimeframeSelect) (*cxsdk.HexagonQuery, diag.Diagnostics) {
	if dataTableQuery == nil {
		return nil, nil
	}
	switch {
	case dataTableQuery.Metrics != nil:
		metrics, diags := expandHexagonMetricsQuery(ctx, dataTableQuery.Metrics, timeframe)
		if diags.HasError() {
			return nil, diags
		}
		return &cxsdk.HexagonQuery{
			Value: metrics,
		}, nil
	case dataTableQuery.Logs != nil:
		logs, diags := expandHexagonLogsQuery(ctx, dataTableQuery.Logs, timeframe)
		if diags.HasError() {
			return nil, diags
		}
		return &cxsdk.HexagonQuery{
			Value: logs,
		}, nil
	case dataTableQuery.Spans != nil:
		spans, diags := expandHexagonSpansQuery(ctx, dataTableQuery.Spans, timeframe)
		if diags.HasError() {
			return nil, diags
		}
		return &cxsdk.HexagonQuery{
			Value: spans,
		}, nil
	case dataTableQuery.DataPrime != nil:
		dataPrime, diags := expandDataTableDataPrimeQuery(ctx, dataTableQuery.DataPrime, timeframe)
		if diags.HasError() {
			return nil, diags
		}
		return &cxsdk.HexagonQuery{
			Value: dataPrime,
		}, nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand DataTable Query", fmt.Sprintf("unknown data table query type %#v", dataTableQuery))}
	}
}

func expandDataTableDataPrimeQuery(ctx context.Context, dataPrime *DataPrimeModel, timeframe *cxsdk.TimeframeSelect) (*cxsdk.HexagonQueryDataprime, diag.Diagnostics) {
	if dataPrime == nil {
		return nil, nil
	}

	filters, diags := expandDashboardFiltersSources(ctx, dataPrime.Filters)
	if diags.HasError() {
		return nil, diags
	}

	var dataPrimeQuery *cxsdk.DashboardDataprimeQuery
	if !dataPrime.Query.IsNull() {
		dataPrimeQuery = &cxsdk.DashboardDataprimeQuery{
			Text: dataPrime.Query.ValueString(),
		}
	}

	return &cxsdk.HexagonQueryDataprime{
		Dataprime: &cxsdk.HexagonDataprimeQuery{
			DataprimeQuery: dataPrimeQuery,
			Filters:        filters,
			TimeFrame:      timeframe,
		},
	}, nil
}

func expandDashboardFiltersSources(ctx context.Context, filters types.List) ([]*cxsdk.DashboardFilterSource, diag.Diagnostics) {
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
		expandedFilter, expandDiags := ExpandFilterSource(ctx, &filterSource)
		if expandDiags.HasError() {
			diags.Append(expandDiags...)
			continue
		}
		expandedFiltersSources = append(expandedFiltersSources, expandedFilter)
	}

	return expandedFiltersSources, diags
}

func expandHexagonMetricsQuery(ctx context.Context, dataTableQueryMetric *QueryMetricsModel, timeframe *cxsdk.TimeframeSelect) (*cxsdk.HexagonQueryMetrics, diag.Diagnostics) {
	if dataTableQueryMetric == nil {
		return nil, nil
	}

	filters, diags := ExpandMetricsFilters(ctx, dataTableQueryMetric.Filters)
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.HexagonQueryMetrics{
		Metrics: &cxsdk.HexagonMetricsQuery{
			PromqlQuery:     ExpandPromqlQuery(dataTableQueryMetric.PromqlQuery),
			Filters:         filters,
			TimeFrame:       timeframe,
			PromqlQueryType: DashboardSchemaToProtoPromQLQueryType[dataTableQueryMetric.PromqlQueryType.ValueString()],
		},
	}, nil
}

func expandHexagonLogsQuery(ctx context.Context, queryLogs *HexagonQueryLogsModel, timeframe *cxsdk.TimeframeSelect) (*cxsdk.HexagonQueryLogs, diag.Diagnostics) {
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

	return &cxsdk.HexagonQueryLogs{
		Logs: &cxsdk.HexagonLogsQuery{
			LuceneQuery:     ExpandLuceneQuery(queryLogs.LuceneQuery),
			Filters:         filters,
			LogsAggregation: aggregation,
			GroupBy:         groupBys,
			TimeFrame:       timeframe,
		},
	}, nil
}

func expandHexagonSpansQuery(ctx context.Context, hexagonQuerySpans *QuerySpansModel, timeframe *cxsdk.TimeframeSelect) (*cxsdk.HexagonQuerySpans, diag.Diagnostics) {
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

	return &cxsdk.HexagonQuerySpans{
		Spans: &cxsdk.HexagonSpansQuery{
			LuceneQuery: ExpandLuceneQuery(hexagonQuerySpans.LuceneQuery),
			Filters:     filters,
			GroupBy:     grouping,
			TimeFrame:   timeframe,
		},
	}, nil
}

// func flattenHexagonSpansQueryAggregations(ctx context.Context, aggregations []*cxsdk.SpansAggregation) (types.List, diag.Diagnostics) {
// 	if len(aggregations) == 0 {
// 		return types.ListNull(types.ObjectType{AttrTypes: SpansAggregationModelAttr()}), nil
// 	}
// 	var diagnostics diag.Diagnostics
// 	aggregationElements := make([]attr.Value, 0, len(aggregations))
// 	for _, aggregation := range aggregations {
// 		flattenedAggregation, dg := flattenHexagonSpansQueryAggregation(aggregation)
// 		if dg != nil {
// 			diagnostics.Append(dg)
// 			continue
// 		}
// 		aggregationElement, diags := types.ObjectValueFrom(ctx, SpansAggregationModelAttr(), flattenedAggregation)
// 		if diags.HasError() {
// 			diagnostics = append(diagnostics, diags...)
// 			continue
// 		}
// 		aggregationElements = append(aggregationElements, aggregationElement)
// 	}
// 	return types.ListValueMust(types.ObjectType{AttrTypes: SpansAggregationModelAttr()}, aggregationElements), diagnostics
// }

// func flattenHexagonSpansQueryAggregation(spanAggregation *cxsdk.SpansAggregation) (*DataTableSpansAggregationModel, diag.Diagnostic) {
// 	if spanAggregation == nil {
// 		return nil, nil
// 	}

// 	aggregation, dg := flattenSpansAggregation(spanAggregation.GetAggregation())
// 	if dg != nil {
// 		return nil, dg
// 	}

// 	return &DataTableSpansAggregationModel{
// 		ID:          utils.WrapperspbStringToTypeString(spanAggregation.GetId()),
// 		Name:        utils.WrapperspbStringToTypeString(spanAggregation.GetName()),
// 		IsVisible:   utils.WrapperspbBoolToTypeBool(spanAggregation.GetIsVisible()),
// 		Aggregation: aggregation,
// 	}, nil
// }
