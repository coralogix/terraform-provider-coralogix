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
	"strconv"
	"strings"

	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	dashboardservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_service"

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

var (
	lineChartStackedLineProtoToSchemaMap = map[dashboardservice.LineChartStackedLine]string{
		dashboardservice.LINECHARTSTACKEDLINE_STACKED_LINE_UNSPECIFIED: utils.UNSPECIFIED,
		dashboardservice.LINECHARTSTACKEDLINE_STACKED_LINE_ABSOLUTE:    "absolute",
		dashboardservice.LINECHARTSTACKEDLINE_STACKED_LINE_RELATIVE:    "relative",
	}
	lineChartStackedLineSchemaToProtoMap      = utils.ReverseMap(lineChartStackedLineProtoToSchemaMap)
	DashboardValidLineChartStackedLineOptions = utils.GetKeys(lineChartStackedLineSchemaToProtoMap)
)

func LineChartSchema() schema.Attribute {
	return lineChartSchema(true)
}

func LineChartSchemaWithoutWidgetValidation() schema.Attribute {
	return lineChartSchema(false)
}

func lineChartSchema(includeWidgetValidation bool) schema.Attribute {
	validators := []validator.Object{
		objectvalidator.AlsoRequires(
			path.MatchRelative().AtParent().AtParent().AtName("title"),
		),
	}
	if includeWidgetValidation {
		validators = append([]validator.Object{
			SupportedWidgetsValidatorWithout("line_chart"),
		}, validators...)
	}

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
			"stacked_line": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Validators: []validator.String{
					stringvalidator.OneOf(DashboardValidLineChartStackedLineOptions...),
				},
				Default:             stringdefault.StaticString(utils.UNSPECIFIED),
				MarkdownDescription: fmt.Sprintf("Option to show lines as stacked. Possible values: %v", strings.Join(DashboardValidLineChartStackedLineOptions, ", ")),
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
										"time_frame":   TimeFrameSchema(),
									},
									Optional: true,
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
											Default:  stringdefault.StaticString(utils.UNSPECIFIED),
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
										"group_by":     SpansFieldsSchema(),
										"aggregations": SpansAggregationsSchema(),
										"filters":      SpansFilterSchema(),
										"time_frame":   TimeFrameSchema(),
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
							Validators: []validator.Object{
								AtMostOneOfAttributes("logs", "metrics", "spans", "data_prime"),
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
							Default:             stringdefault.StaticString(utils.UNSPECIFIED),
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
								},
								"buckets_presented": schema.Int64Attribute{
									Optional: true,
								},
							},
							Validators: []validator.Object{
								AtMostOneOfAttributes("interval", "buckets_presented"),
							},
							Optional: true,
						},
						"data_mode_type": schema.StringAttribute{
							Optional: true,
							Computed: true,
							Validators: []validator.String{
								stringvalidator.OneOf(DashboardValidDataModeTypes...),
							},
							Default: stringdefault.StaticString(utils.UNSPECIFIED),
						},
					},
				},
			},
		},
		Validators: validators,
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
			"stacked_line": types.StringType,
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
						"time_frame": types.ObjectType{
							AttrTypes: TimeFrameModelAttr(),
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

func FlattenLineChart(ctx context.Context, lineChart *dashboardservice.LineChart) (*WidgetDefinitionModel, diag.Diagnostics) {
	if lineChart == nil {
		return nil, nil
	}

	queryDefinitions, diags := flattenLineChartQueryDefinitions(ctx, lineChart.GetQueryDefinitions())
	if diags.HasError() {
		return nil, diags
	}

	return &WidgetDefinitionModel{
		LineChart: &LineChartModel{
			Legend:           FlattenLegend(lineChart.Legend),
			Tooltip:          flattenTooltip(lineChart.Tooltip),
			QueryDefinitions: queryDefinitions,
			StackedLine:      types.StringValue(lineChartStackedLineProtoToSchemaMap[lineChart.GetStackedLine()]),
		},
	}, nil
}

func flattenTooltip(tooltip *dashboardservice.Tooltip) *TooltipModel {
	if tooltip == nil {
		return nil
	}
	return &TooltipModel{
		ShowLabels: types.BoolPointerValue(tooltip.ShowLabels),
		Type:       types.StringValue(DashboardProtoToSchemaTooltipType[tooltip.GetType()]),
	}
}

func flattenLineChartQueryDefinitions(ctx context.Context, definitions []dashboardservice.LineChartQueryDefinition) (types.List, diag.Diagnostics) {
	if len(definitions) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: lineChartQueryDefinitionModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	definitionsElements := make([]attr.Value, 0, len(definitions))
	for _, definition := range definitions {
		flattenedDefinition, diags := flattenLineChartQueryDefinition(ctx, &definition)
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

func flattenLineChartQueryDefinition(ctx context.Context, definition *dashboardservice.LineChartQueryDefinition) (*LineChartQueryDefinitionModel, diag.Diagnostics) {
	if definition == nil {
		return nil, nil
	}

	query, diags := flattenLineChartQuery(ctx, &definition.Query)
	if diags.HasError() {
		return nil, diags
	}

	resolution, diags := flattenLineChartQueryResolution(ctx, definition.Resolution)
	if diags.HasError() {
		return nil, diags
	}

	return &LineChartQueryDefinitionModel{
		ID:                 types.StringValue(definition.GetId()),
		Query:              query,
		SeriesNameTemplate: utils.StringPointerToTypeString(definition.SeriesNameTemplate),
		SeriesCountLimit:   stringPointerToInt64(definition.SeriesCountLimit),
		Unit:               types.StringValue(DashboardProtoToSchemaUnit[definition.GetUnit()]),
		ScaleType:          types.StringValue(DashboardProtoToSchemaScaleType[definition.GetScaleType()]),
		Name:               utils.StringPointerToTypeString(definition.Name),
		IsVisible:          types.BoolPointerValue(definition.IsVisible),
		ColorScheme:        utils.StringPointerToTypeString(definition.ColorScheme),
		Resolution:         resolution,
		DataModeType:       types.StringValue(DashboardProtoToSchemaDataModeType[definition.GetDataModeType()]),
	}, nil
}

func flattenLineChartQueryResolution(ctx context.Context, resolution *dashboardservice.LineChartResolution) (types.Object, diag.Diagnostics) {
	if resolution == nil {
		return types.ObjectNull(lineChartQueryResolutionModelAttr()), nil
	}

	interval := types.StringNull()
	if resolution.Interval != nil {
		interval = flattenDuration(resolution.Interval)
	}
	bucketsPresented := int32PointerToInt64Type(resolution.BucketsPresented)

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

func flattenLineChartQuery(ctx context.Context, query *dashboardservice.LineChartQuery) (*LineChartQueryModel, diag.Diagnostics) {
	if query == nil {
		return nil, nil
	}

	switch {
	case query.Logs != nil:
		return flattenLineChartQueryLogs(ctx, query.Logs)
	case query.Metrics != nil:
		return flattenLineChartQueryMetrics(ctx, query.Metrics)
	case query.Spans != nil:
		return flattenLineChartQuerySpans(ctx, query.Spans)
	case query.Dataprime != nil:
		return flattenLineChartDataPrimeQuery(ctx, query.Dataprime)
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Line Chart Query", "unknown line chart query type")}
	}
}

func flattenLineChartDataPrimeQuery(ctx context.Context, dataPrime *dashboardservice.LineChartDataprimeQuery) (*LineChartQueryModel, diag.Diagnostics) {
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

	return &LineChartQueryModel{
		DataPrime: &DataPrimeModel{
			Query:     dataPrimeQuery,
			Filters:   filters,
			TimeFrame: timeframe,
		},
	}, nil
}

func flattenLineChartQueryLogs(ctx context.Context, logs *dashboardservice.LineChartLogsQuery) (*LineChartQueryModel, diag.Diagnostics) {
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

	timeFrame, diags := FlattenTimeFrameSelect(ctx, logs.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	return &LineChartQueryModel{
		Logs: &LineChartQueryLogsModel{
			LuceneQuery:  flattenLuceneQuery(logs.LuceneQuery),
			GroupBy:      utils.StringSliceToTypeStringList(logs.GetGroupBy()),
			Aggregations: aggregations,
			Filters:      filters,
			TimeFrame:    timeFrame,
		},
	}, nil
}

func flattenAggregations(ctx context.Context, aggregations []dashboardservice.LogsAggregation) (types.List, diag.Diagnostics) {
	if len(aggregations) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: AggregationModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	aggregationsElements := make([]attr.Value, 0, len(aggregations))
	for _, aggregation := range aggregations {
		flattenedAggregation, diags := FlattenLogsAggregation(ctx, &aggregation)
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

func flattenLineChartQueryMetrics(ctx context.Context, metrics *dashboardservice.LineChartMetricsQuery) (*LineChartQueryModel, diag.Diagnostics) {
	if metrics == nil {
		return nil, nil
	}

	filters, diags := FlattenMetricsFilters(ctx, metrics.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	timeFrame, diags := FlattenTimeFrameSelect(ctx, metrics.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	return &LineChartQueryModel{
		Metrics: &QueryMetricsModel{
			PromqlQuery:     flattenPromqlQuery(metrics.PromqlQuery),
			Filters:         filters,
			PromqlQueryType: types.StringValue(utils.UNSPECIFIED),
			TimeFrame:       timeFrame,
		},
	}, nil
}

func flattenLineChartQuerySpans(ctx context.Context, spans *dashboardservice.LineChartSpansQuery) (*LineChartQueryModel, diag.Diagnostics) {
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

	timeFrame, diags := FlattenTimeFrameSelect(ctx, spans.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	return &LineChartQueryModel{
		Spans: &LineChartQuerySpansModel{
			LuceneQuery:  flattenLuceneQuery(spans.LuceneQuery),
			GroupBy:      groupBy,
			Filters:      filters,
			Aggregations: aggregations,
			TimeFrame:    timeFrame,
		},
	}, nil
}

func flattenLineChartSpansAggregation(ctx context.Context, aggregations []dashboardservice.SpansAggregation) (types.List, diag.Diagnostics) {
	if aggregations == nil {
		return types.ListNull(types.ObjectType{AttrTypes: SpansAggregationModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	columnElements := make([]attr.Value, 0, len(aggregations))
	for _, column := range aggregations {
		flattenedColumn, _ := FlattenSpansAggregation(&column)
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

func ExpandLineChart(ctx context.Context, lineChart *LineChartModel) (*dashboardservice.WidgetDefinition, diag.Diagnostics) {
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

	var stackedLine dashboardservice.LineChartStackedLine
	if !(lineChart.StackedLine.IsNull() || lineChart.StackedLine.IsUnknown()) {
		stackedLine = lineChartStackedLineSchemaToProtoMap[lineChart.StackedLine.ValueString()]
	} else {
		stackedLine = dashboardservice.LINECHARTSTACKEDLINE_STACKED_LINE_UNSPECIFIED
	}

	return &dashboardservice.WidgetDefinition{
		LineChart: &dashboardservice.LineChart{
			Legend:           legend,
			Tooltip:          expandLineChartTooltip(lineChart.Tooltip),
			QueryDefinitions: queryDefinitions,
			StackedLine:      stackedLine.Ptr(),
		},
	}, nil
}

func expandLineChartTooltip(tooltip *TooltipModel) *dashboardservice.Tooltip {
	if tooltip == nil {
		return nil
	}

	return &dashboardservice.Tooltip{
		ShowLabels: tooltip.ShowLabels.ValueBoolPointer(),
		Type:       OptionalEnumPointer(tooltip.Type, DashboardSchemaToProtoTooltipType),
	}
}

func expandLineChartQueryDefinitions(ctx context.Context, queryDefinitions types.List) ([]dashboardservice.LineChartQueryDefinition, diag.Diagnostics) {
	var queryDefinitionsObjects []types.Object
	var expandedQueryDefinitions []dashboardservice.LineChartQueryDefinition
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
		expandedQueryDefinitions = append(expandedQueryDefinitions, *expandedQueryDefinition)
	}

	return expandedQueryDefinitions, diags
}

func expandLineChartQueryDefinition(ctx context.Context, queryDefinition *LineChartQueryDefinitionModel) (*dashboardservice.LineChartQueryDefinition, diag.Diagnostics) {
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

	return &dashboardservice.LineChartQueryDefinition{
		Id:                 *ExpandDashboardIDs(queryDefinition.ID),
		Query:              query,
		SeriesNameTemplate: utils.TypeStringToStringPointer(queryDefinition.SeriesNameTemplate),
		SeriesCountLimit:   int64ToStringPointer(queryDefinition.SeriesCountLimit),
		Unit:               OptionalEnumPointer(queryDefinition.Unit, DashboardSchemaToProtoUnit),
		ScaleType:          OptionalEnumPointer(queryDefinition.ScaleType, DashboardSchemaToProtoScaleType),
		Name:               utils.TypeStringToStringPointer(queryDefinition.Name),
		IsVisible:          queryDefinition.IsVisible.ValueBoolPointer(),
		ColorScheme:        utils.TypeStringToStringPointer(queryDefinition.ColorScheme),
		Resolution:         resolution,
		DataModeType:       OptionalEnumPointer(queryDefinition.DataModeType, DashboardSchemaToProtoDataModeType),
	}, nil
}

func expandLineChartQuery(ctx context.Context, query *LineChartQueryModel) (dashboardservice.LineChartQuery, diag.Diagnostics) {
	if query == nil {
		return dashboardservice.LineChartQuery{}, nil
	}

	switch {
	case query.Logs != nil:
		logs, diags := expandLineChartLogsQuery(ctx, query.Logs)
		if diags.HasError() {
			return dashboardservice.LineChartQuery{}, diags
		}
		return dashboardservice.LineChartQuery{
			Logs: logs,
		}, nil
	case query.Metrics != nil:
		metrics, diags := expandLineChartMetricsQuery(ctx, query.Metrics)
		if diags.HasError() {
			return dashboardservice.LineChartQuery{}, diags
		}
		return dashboardservice.LineChartQuery{
			Metrics: metrics,
		}, nil
	case query.Spans != nil:
		spans, diags := expandLineChartSpansQuery(ctx, query.Spans)
		if diags.HasError() {
			return dashboardservice.LineChartQuery{}, diags
		}
		return dashboardservice.LineChartQuery{
			Spans: spans,
		}, nil
	case query.DataPrime != nil:
		dataPrime, diags := expandLineChartDataPrimeQuery(ctx, query.DataPrime)
		if diags.HasError() {
			return dashboardservice.LineChartQuery{}, diags
		}
		return dashboardservice.LineChartQuery{
			Dataprime: dataPrime,
		}, nil
	default:
		return dashboardservice.LineChartQuery{}, diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand LineChart Query", "Unknown LineChart Query type")}
	}
}

func expandLineChartDataPrimeQuery(ctx context.Context, dataPrime *DataPrimeModel) (*dashboardservice.LineChartDataprimeQuery, diag.Diagnostics) {
	if dataPrime == nil {
		return nil, nil
	}

	filters, diags := ExpandDashboardFiltersSources(ctx, dataPrime.Filters)
	if diags.HasError() {
		return nil, diags
	}

	timeFrame, diags := ExpandTimeFrameSelect(ctx, dataPrime.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	dataPrimeQuery := &dashboardservice.CommonDataprimeQuery{
		Text: dataPrime.Query.ValueStringPointer(),
	}
	return &dashboardservice.LineChartDataprimeQuery{
		Filters:        filters,
		DataprimeQuery: dataPrimeQuery,
		TimeFrame:      timeFrame,
	}, nil
}

func expandLineChartLogsQuery(ctx context.Context, logs *LineChartQueryLogsModel) (*dashboardservice.LineChartLogsQuery, diag.Diagnostics) {
	if logs == nil {
		return nil, nil
	}

	groupBy, diags := typeStringListToStringSlice(ctx, logs.GroupBy)
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

	return &dashboardservice.LineChartLogsQuery{
		LuceneQuery:  ExpandLuceneQuery(logs.LuceneQuery),
		GroupBy:      groupBy,
		Aggregations: aggregations,
		Filters:      filters,
		TimeFrame:    timeFrame,
	}, nil
}

func expandLineChartMetricsQuery(ctx context.Context, metrics *QueryMetricsModel) (*dashboardservice.LineChartMetricsQuery, diag.Diagnostics) {
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

	return &dashboardservice.LineChartMetricsQuery{
		PromqlQuery: ExpandPromqlQuery(metrics.PromqlQuery),
		Filters:     filters,
		TimeFrame:   timeFrame,
	}, nil
}

func expandLineChartSpansQuery(ctx context.Context, spans *LineChartQuerySpansModel) (*dashboardservice.LineChartSpansQuery, diag.Diagnostics) {
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

	return &dashboardservice.LineChartSpansQuery{
		LuceneQuery:  ExpandLuceneQuery(spans.LuceneQuery),
		GroupBy:      groupBy,
		Aggregations: aggregations,
		Filters:      filters,
		TimeFrame:    timeFrame,
	}, nil
}

func int64ToStringPointer(value types.Int64) *string {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	converted := strconv.FormatInt(value.ValueInt64(), 10)
	return &converted
}

func stringPointerToInt64(value *string) types.Int64 {
	if value == nil {
		return types.Int64Null()
	}
	converted, err := strconv.ParseInt(*value, 10, 64)
	if err != nil {
		return types.Int64Null()
	}
	return types.Int64Value(converted)
}
