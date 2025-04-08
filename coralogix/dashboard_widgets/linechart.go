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
	"strings"
	"terraform-provider-coralogix/coralogix/utils"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

func LineChartSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"legend": LegendSchema(),
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
							stringvalidator.OneOf(DashboardValidTooltipTypes...),
						},
						MarkdownDescription: fmt.Sprintf("The tooltip type. Valid values are: %s.", strings.Join(DashboardValidTooltipTypes, ", ")),
					},
				},
				Optional: true,
			},
			"query_definitions": schema.ListNestedAttribute{
				Required: true,
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
										"filters":      LogsFiltersSchema(),
										"aggregations": LogsAggregationsSchema(),
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
										"filters": MetricFiltersSchema(),
										"promql_query_type": schema.StringAttribute{
											Optional: true,
											Computed: true,
											Default:  stringdefault.StaticString(UNSPECIFIED),
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
								"spans": schema.SingleNestedAttribute{
									Attributes: map[string]schema.Attribute{
										"lucene_query": schema.StringAttribute{
											Optional: true,
										},
										"group_by":     SpansFieldsSchema(),
										"aggregations": SpansAggregationsSchema(),
										"filters":      SpansFilterSchema(),
									},
									Optional: true,
									Validators: []validator.Object{
										objectvalidator.ExactlyOneOf(
											path.MatchRelative().AtParent().AtName("metrics"),
											path.MatchRelative().AtParent().AtName("logs"),
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
								},
								"time_frame": TimeFrameSchema(),
							},
							Required: true,
						},
						"series_name_template": schema.StringAttribute{
							Optional: true,
						},
						"series_count_limit": schema.Int64Attribute{
							Optional: true,
						},
						"unit": UnitSchema(),
						"scale_type": schema.StringAttribute{
							Optional: true,
							Computed: true,
							Validators: []validator.String{
								stringvalidator.OneOf(DashboardValidScaleTypes...),
							},
							Default:             stringdefault.StaticString(UNSPECIFIED),
							MarkdownDescription: fmt.Sprintf("The scale type. Valid values are: %s.", strings.Join(DashboardValidScaleTypes, ", ")),
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
								stringvalidator.OneOf(DashboardValidColorSchemes...),
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
								stringvalidator.OneOf(DashboardValidDataModeTypes...),
							},
							Default: stringdefault.StaticString(UNSPECIFIED),
						},
					},
				},
			},
		},
		Validators: []validator.Object{
			SupportedWidgetsValidatorWithout("line_chart"),
			objectvalidator.AlsoRequires(
				path.MatchRelative().AtParent().AtParent().AtName("title"),
			),
		},
	}
}

func LineChartType() types.ObjectType {
	return types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"legend": types.ObjectType{
				AttrTypes: LegendAttr(),
			},
			"tooltip": types.ObjectType{
				AttrTypes: map[string]attr.Type{
					"show_labels": types.BoolType,
					"type":        types.StringType,
				},
			},
			"query_definitions": types.ListType{
				ElemType: types.ObjectType{
					AttrTypes: lineChartQueryDefinitionModelAttr(),
				},
			},
		},
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
								AttrTypes: AggregationModelAttr(),
							},
						},
						"filters": types.ListType{
							ElemType: types.ObjectType{
								AttrTypes: LogsFilterModelAttr(),
							},
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
					},
				},
				"spans": types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"lucene_query": types.StringType,
						"group_by": types.ListType{
							ElemType: types.ObjectType{
								AttrTypes: SpansFieldModelAttr(),
							},
						},
						"aggregations": types.ListType{
							ElemType: types.ObjectType{
								AttrTypes: SpansAggregationModelAttr(),
							},
						},
						"filters": types.ListType{
							ElemType: types.ObjectType{
								AttrTypes: SpansFilterModelAttr(),
							},
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
				"time_frame": types.ObjectType{
					AttrTypes: TimeFrameModelAttr(),
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

func FlattenLineChart(ctx context.Context, lineChart *cxsdk.LineChart) (*WidgetDefinitionModel, diag.Diagnostics) {
	if lineChart == nil {
		return nil, nil
	}

	queryDefinitions, diags := flattenLineChartQueryDefinitions(ctx, lineChart.GetQueryDefinitions())
	if diags.HasError() {
		return nil, diags
	}

	return &WidgetDefinitionModel{
		LineChart: &LineChartModel{
			Legend:           FlattenLegend(lineChart.GetLegend()),
			Tooltip:          flattenTooltip(lineChart.GetTooltip()),
			QueryDefinitions: queryDefinitions,
		},
	}, nil
}

func flattenTooltip(tooltip *cxsdk.LineChartTooltip) *TooltipModel {
	if tooltip == nil {
		return nil
	}
	return &TooltipModel{
		ShowLabels: utils.WrapperspbBoolToTypeBool(tooltip.GetShowLabels()),
		Type:       types.StringValue(DashboardProtoToSchemaTooltipType[tooltip.GetType()]),
	}
}

func flattenLineChartQueryDefinitions(ctx context.Context, definitions []*cxsdk.LineChartQueryDefinition) (types.List, diag.Diagnostics) {
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

	if diagnostics.HasError() {
		return types.ListNull(types.ObjectType{AttrTypes: lineChartQueryDefinitionModelAttr()}), diagnostics
	}
	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: lineChartQueryDefinitionModelAttr()}, definitionsElements)
}

func flattenLineChartQueryDefinition(ctx context.Context, definition *cxsdk.LineChartQueryDefinition) (*LineChartQueryDefinitionModel, diag.Diagnostics) {
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
		ID:                 utils.WrapperspbStringToTypeString(definition.GetId()),
		Query:              query,
		SeriesNameTemplate: utils.WrapperspbStringToTypeString(definition.GetSeriesNameTemplate()),
		SeriesCountLimit:   utils.WrapperspbInt64ToTypeInt64(definition.GetSeriesCountLimit()),
		Unit:               types.StringValue(DashboardProtoToSchemaUnit[definition.GetUnit()]),
		ScaleType:          types.StringValue(DashboardProtoToSchemaScaleType[definition.GetScaleType()]),
		Name:               utils.WrapperspbStringToTypeString(definition.GetName()),
		IsVisible:          utils.WrapperspbBoolToTypeBool(definition.GetIsVisible()),
		ColorScheme:        utils.WrapperspbStringToTypeString(definition.GetColorScheme()),
		Resolution:         resolution,
		DataModeType:       types.StringValue(DashboardProtoToSchemaDataModeType[definition.GetDataModeType()]),
	}, nil
}

func flattenLineChartQueryResolution(ctx context.Context, resolution *cxsdk.LineChartResolution) (types.Object, diag.Diagnostics) {
	if resolution == nil {
		return types.ObjectNull(lineChartQueryResolutionModelAttr()), nil
	}

	interval := types.StringNull()
	if resolution.GetInterval() != nil {
		interval = types.StringValue(resolution.GetInterval().String())
	}
	bucketsPresented := utils.WrapperspbInt32ToTypeInt64(resolution.GetBucketsPresented())

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

func flattenLineChartQuery(ctx context.Context, query *cxsdk.LineChartQuery) (*LineChartQueryModel, diag.Diagnostics) {
	if query == nil {
		return nil, nil
	}

	switch query.GetValue().(type) {
	case *cxsdk.LineChartQueryLogs:
		return flattenLineChartQueryLogs(ctx, query.GetLogs())
	case *cxsdk.LineChartQueryMetrics:
		return flattenLineChartQueryMetrics(ctx, query.GetMetrics())
	case *cxsdk.LineChartQuerySpans:
		return flattenLineChartQuerySpans(ctx, query.GetSpans())
	case *cxsdk.LineChartQueryDataprime:
		return flattenLineChartDataPrimeQuery(ctx, query.GetDataprime())
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Line Chart Query", "unknown line chart query type")}
	}
}

func flattenLineChartDataPrimeQuery(ctx context.Context, dataPrime *cxsdk.LineChartDataprimeQuery) (*LineChartQueryModel, diag.Diagnostics) {
	if dataPrime == nil {
		return nil, nil
	}

	dataPrimeQuery := types.StringNull()
	if dataPrime.GetDataprimeQuery() != nil {
		dataPrimeQuery = types.StringValue(dataPrime.GetDataprimeQuery().GetText())
	}

	filters, diags := FlattenDashboardFiltersSources(ctx, dataPrime.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	timeframe, diags := FlattenTimeFrameSelect(ctx, dataPrime.GetTimeFrame())
	if diags.HasError() {
		return nil, diags
	}

	return &LineChartQueryModel{
		DataPrime: &DataPrimeModel{
			Query:     dataPrimeQuery,
			Filters:   filters,
			TimeFrame: timeframe,
		},
	}, nil
}

func flattenLineChartQueryLogs(ctx context.Context, logs *cxsdk.LineChartLogsQuery) (*LineChartQueryModel, diag.Diagnostics) {
	if logs == nil {
		return nil, nil
	}

	aggregations, diags := flattenAggregations(ctx, logs.GetAggregations())
	if diags.HasError() {
		return nil, diags
	}

	filters, diags := FlattenLogsFilters(ctx, logs.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	timeFrame, diags := FlattenTimeFrameSelect(ctx, logs.GetTimeFrame())
	if diags.HasError() {
		return nil, diags
	}

	return &LineChartQueryModel{
		Logs: &LineChartQueryLogsModel{
			LuceneQuery:  utils.WrapperspbStringToTypeString(logs.GetLuceneQuery().GetValue()),
			GroupBy:      utils.WrappedStringSliceToTypeStringList(logs.GetGroupBy()),
			Aggregations: aggregations,
			Filters:      filters,
			TimeFrame:    timeFrame,
		},
	}, nil
}

func flattenAggregations(ctx context.Context, aggregations []*cxsdk.LogsAggregation) (types.List, diag.Diagnostics) {
	if len(aggregations) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: AggregationModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	aggregationsElements := make([]attr.Value, 0, len(aggregations))
	for _, aggregation := range aggregations {
		flattenedAggregation, diags := FlattenLogsAggregation(ctx, aggregation)
		if diags.HasError() {
			diagnostics.Append(diags...)
			continue
		}
		aggregationElement, diags := types.ObjectValueFrom(ctx, AggregationModelAttr(), flattenedAggregation)
		if diags.HasError() {
			diagnostics.Append(diags...)
			continue
		}
		aggregationsElements = append(aggregationsElements, aggregationElement)
	}
	if diagnostics.HasError() {
		return types.ListNull(types.ObjectType{AttrTypes: lineChartQueryDefinitionModelAttr()}), diagnostics
	}
	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: AggregationModelAttr()}, aggregationsElements)
}

func flattenLineChartQueryMetrics(ctx context.Context, metrics *cxsdk.LineChartMetricsQuery) (*LineChartQueryModel, diag.Diagnostics) {
	if metrics == nil {
		return nil, nil
	}

	filters, diags := FlattenMetricsFilters(ctx, metrics.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	timeFrame, diags := FlattenTimeFrameSelect(ctx, metrics.GetTimeFrame())
	if diags.HasError() {
		return nil, diags
	}

	return &LineChartQueryModel{
		Metrics: &QueryMetricsModel{
			PromqlQuery:     utils.WrapperspbStringToTypeString(metrics.GetPromqlQuery().GetValue()),
			Filters:         filters,
			PromqlQueryType: types.StringValue(UNSPECIFIED),
			TimeFrame:       timeFrame,
		},
	}, nil
}

func flattenLineChartQuerySpans(ctx context.Context, spans *cxsdk.LineChartSpansQuery) (*LineChartQueryModel, diag.Diagnostics) {
	if spans == nil {
		return nil, nil
	}

	groupBy, diags := FlattenSpansFields(ctx, spans.GetGroupBy())
	if diags.HasError() {
		return nil, diags
	}

	filters, diags := FlattenSpansFilters(ctx, spans.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	aggregations, diags := flattenLineChartSpansAggregation(ctx, spans.GetAggregations())
	if diags.HasError() {
		return nil, diags
	}

	timeFrame, diags := FlattenTimeFrameSelect(ctx, spans.GetTimeFrame())
	if diags.HasError() {
		return nil, diags
	}

	return &LineChartQueryModel{
		Spans: &LineChartQuerySpansModel{
			LuceneQuery:  utils.WrapperspbStringToTypeString(spans.GetLuceneQuery().GetValue()),
			GroupBy:      groupBy,
			Filters:      filters,
			Aggregations: aggregations,
			TimeFrame:    timeFrame,
		},
	}, nil
}

func flattenLineChartSpansAggregation(ctx context.Context, aggregations []*cxsdk.SpansAggregation) (types.List, diag.Diagnostics) {
	if aggregations == nil {
		return types.ListNull(types.ObjectType{AttrTypes: SpansAggregationModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	columnElements := make([]attr.Value, 0, len(aggregations))
	for _, column := range aggregations {
		flattenedColumn, _ := FlattenSpansAggregation(column)
		columnElement, diags := types.ObjectValueFrom(ctx, SpansAggregationModelAttr(), flattenedColumn)
		if diags.HasError() {
			diagnostics = append(diagnostics, diags...)
			continue
		}
		columnElements = append(columnElements, columnElement)
	}

	if diagnostics.HasError() {
		return types.ListNull(types.ObjectType{AttrTypes: SpansAggregationModelAttr()}), diagnostics
	}

	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: SpansAggregationModelAttr()}, columnElements)
}

func ExpandLineChart(ctx context.Context, lineChart *LineChartModel) (*cxsdk.WidgetDefinition, diag.Diagnostics) {
	if lineChart == nil {
		return nil, nil
	}

	legend, diags := ExpandLegend(ctx, lineChart.Legend)
	if diags.HasError() {
		return nil, diags
	}

	queryDefinitions, diags := expandLineChartQueryDefinitions(ctx, lineChart.QueryDefinitions)
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.WidgetDefinition{
		Value: &cxsdk.WidgetDefinitionLineChart{
			LineChart: &cxsdk.LineChart{
				Legend:           legend,
				Tooltip:          expandLineChartTooltip(lineChart.Tooltip),
				QueryDefinitions: queryDefinitions,
				// TODO: Stacked Line
			},
		},
	}, nil
}

func expandLineChartTooltip(tooltip *TooltipModel) *cxsdk.LineChartTooltip {
	if tooltip == nil {
		return nil
	}

	return &cxsdk.LineChartTooltip{
		ShowLabels: utils.TypeBoolToWrapperspbBool(tooltip.ShowLabels),
		Type:       DashboardSchemaToProtoTooltipType[tooltip.Type.ValueString()],
	}
}

func expandLineChartQueryDefinitions(ctx context.Context, queryDefinitions types.List) ([]*cxsdk.LineChartQueryDefinition, diag.Diagnostics) {
	var queryDefinitionsObjects []types.Object
	var expandedQueryDefinitions []*cxsdk.LineChartQueryDefinition
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

func expandLineChartQueryDefinition(ctx context.Context, queryDefinition *LineChartQueryDefinitionModel) (*cxsdk.LineChartQueryDefinition, diag.Diagnostics) {
	if queryDefinition == nil {
		return nil, nil
	}
	query, diags := expandLineChartQuery(ctx, queryDefinition.Query)
	if diags.HasError() {
		return nil, diags
	}

	resolution, diags := ExpandResolution(ctx, queryDefinition.Resolution)
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.LineChartQueryDefinition{
		Id:                 ExpandDashboardIDs(queryDefinition.ID),
		Query:              query,
		SeriesNameTemplate: utils.TypeStringToWrapperspbString(queryDefinition.SeriesNameTemplate),
		SeriesCountLimit:   utils.TypeInt64ToWrappedInt64(queryDefinition.SeriesCountLimit),
		Unit:               DashboardSchemaToProtoUnit[queryDefinition.Unit.ValueString()],
		ScaleType:          DashboardSchemaToProtoScaleType[queryDefinition.ScaleType.ValueString()],
		Name:               utils.TypeStringToWrapperspbString(queryDefinition.Name),
		IsVisible:          utils.TypeBoolToWrapperspbBool(queryDefinition.IsVisible),
		ColorScheme:        utils.TypeStringToWrapperspbString(queryDefinition.ColorScheme),
		Resolution:         resolution,
		DataModeType:       DashboardSchemaToProtoDataModeType[queryDefinition.DataModeType.ValueString()],
	}, nil
}

func expandLineChartQuery(ctx context.Context, query *LineChartQueryModel) (*cxsdk.LineChartQuery, diag.Diagnostics) {
	if query == nil {
		return nil, nil
	}

	switch {
	case query.Logs != nil:
		logs, diags := expandLineChartLogsQuery(ctx, query.Logs)
		if diags.HasError() {
			return nil, diags
		}
		return &cxsdk.LineChartQuery{
			Value: logs,
		}, nil
	case query.Metrics != nil:
		metrics, diags := expandLineChartMetricsQuery(ctx, query.Metrics)
		if diags.HasError() {
			return nil, diags
		}
		return &cxsdk.LineChartQuery{
			Value: metrics,
		}, nil
	case query.Spans != nil:
		spans, diags := expandLineChartSpansQuery(ctx, query.Spans)
		if diags.HasError() {
			return nil, diags
		}
		return &cxsdk.LineChartQuery{
			Value: spans,
		}, nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand LineChart Query", "Unknown LineChart Query type")}
	}
}

func expandLineChartLogsQuery(ctx context.Context, logs *LineChartQueryLogsModel) (*cxsdk.LineChartQueryLogs, diag.Diagnostics) {
	if logs == nil {
		return nil, nil
	}

	groupBy, diags := utils.TypeStringSliceToWrappedStringSlice(ctx, logs.GroupBy.Elements())
	if diags.HasError() {
		return nil, diags
	}

	aggregations, diags := ExpandLogsAggregations(ctx, logs.Aggregations)
	if diags.HasError() {
		return nil, diags
	}

	filters, diags := ExpandLogsFilters(ctx, logs.Filters)
	if diags.HasError() {
		return nil, diags
	}

	timeFrame, diags := ExpandTimeFrameSelect(ctx, logs.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.LineChartQueryLogs{
		Logs: &cxsdk.LineChartLogsQuery{
			LuceneQuery:  ExpandLuceneQuery(logs.LuceneQuery),
			GroupBy:      groupBy,
			Aggregations: aggregations,
			Filters:      filters,
			TimeFrame:    timeFrame,
		},
	}, nil
}

func expandLineChartMetricsQuery(ctx context.Context, metrics *QueryMetricsModel) (*cxsdk.LineChartQueryMetrics, diag.Diagnostics) {
	if metrics == nil {
		return nil, nil
	}

	filters, diags := ExpandMetricsFilters(ctx, metrics.Filters)
	if diags.HasError() {
		return nil, diags
	}

	timeFrame, diags := ExpandTimeFrameSelect(ctx, metrics.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.LineChartQueryMetrics{
		Metrics: &cxsdk.LineChartMetricsQuery{
			PromqlQuery: ExpandPromqlQuery(metrics.PromqlQuery),
			Filters:     filters,
			TimeFrame:   timeFrame,
		},
	}, nil
}

func expandLineChartSpansQuery(ctx context.Context, spans *LineChartQuerySpansModel) (*cxsdk.LineChartQuerySpans, diag.Diagnostics) {
	if spans == nil {
		return nil, nil
	}

	groupBy, diags := ExpandSpansFields(ctx, spans.GroupBy)
	if diags.HasError() {
		return nil, diags
	}

	aggregations, diags := ExpandSpansAggregations(ctx, spans.Aggregations)
	if diags.HasError() {
		return nil, diags
	}

	filters, diags := ExpandSpansFilters(ctx, spans.Filters)
	if diags.HasError() {
		return nil, diags
	}

	timeFrame, diags := ExpandTimeFrameSelect(ctx, spans.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.LineChartQuerySpans{
		Spans: &cxsdk.LineChartSpansQuery{
			LuceneQuery:  ExpandLuceneQuery(spans.LuceneQuery),
			GroupBy:      groupBy,
			Aggregations: aggregations,
			Filters:      filters,
			TimeFrame:    timeFrame,
		},
	}, nil
}
