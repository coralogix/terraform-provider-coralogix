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

	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
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
	Logs      *QueryLogsModel    `tfsdk:"logs"`
	Metrics   *QueryMetricsModel `tfsdk:"metrics"`
	Spans     *QuerySpansModel   `tfsdk:"spans"`
	DataPrime *DataPrimeModel    `tfsdk:"dataprime"`
}

type HexagonThresholdModel struct {
	From  types.Number `tfsdk:"from"`
	Color types.String `tfsdk:"color"`
	Label types.String `tfsdk:"label"`
}

func HexagonSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Required: true,
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
			"data_mode_type": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Validators: []validator.String{
					stringvalidator.OneOf(DashboardValidDataModeTypes...),
				},
				Default: stringdefault.StaticString("unspecified"),
			},
			"thresholds": schema.SetNestedAttribute{
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
								path.MatchRelative().AtParent().AtName("dataprime"),
							),
						},
					},
					"metrics": schema.SingleNestedAttribute{
						Attributes: map[string]schema.Attribute{
							"promql_query": schema.StringAttribute{
								Required: true,
							},
							"filters": MetricFiltersSchema(),
						},
						Optional: true,
						Validators: []validator.Object{
							objectvalidator.ExactlyOneOf(
								path.MatchRelative().AtParent().AtName("logs"),
								path.MatchRelative().AtParent().AtName("spans"),
								path.MatchRelative().AtParent().AtName("dataprime"),
							),
						},
					},
					"spans": schema.SingleNestedAttribute{
						Attributes: map[string]schema.Attribute{
							"lucene_query": schema.StringAttribute{
								Optional: true,
							},
							"group_by":     SpansFieldsSchema(),
							"aggregations": SpansAggregationSchema(),
							"filters":      SpansFilterSchema(),
						},
						Optional: true,
						Validators: []validator.Object{
							objectvalidator.ExactlyOneOf(
								path.MatchRelative().AtParent().AtName("metrics"),
								path.MatchRelative().AtParent().AtName("logs"),
								path.MatchRelative().AtParent().AtName("dataprime"),
							),
						},
					},
					"dataprime": schema.SingleNestedAttribute{
						Attributes: map[string]schema.Attribute{
							"dataprime_query": schema.StringAttribute{
								Optional: true,
							},
							"timeframe": TimeFrameSchema(),
							"filters": schema.ListNestedAttribute{
								NestedObject: schema.NestedAttributeObject{
									Attributes: FiltersSourceAttribute(),
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
			objectvalidator.ExactlyOneOf(
				path.MatchRelative().AtParent().AtName("data_table"),
				path.MatchRelative().AtParent().AtName("gauge"),
				path.MatchRelative().AtParent().AtName("line_chart"),
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
	}
}

func FlattenHexagon(ctx context.Context, chart *cxsdk.Hexagon) (*WidgetDefinitionModel, diag.Diagnostics) {
	if chart == nil {
		return nil, nil
	}

	query, diags := flattenHexagonQuery(ctx, chart.GetQuery())
	if diags.HasError() {
		return nil, diags
	}

	return &WidgetDefinitionModel{
		Hexagon: &HexagonModel{
			Legend: FlattenLegend(chart.GetLegend()),
			Query:  query,
		},
	}, nil
}

func flattenHexagonQuery(ctx context.Context, query *cxsdk.HexagonQuery) (*HexagonQueryModel, diag.Diagnostics) {
	if query == nil {
		return nil, nil
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
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Data Table Query", "unknown data table query type")}
	}
}

func flattenHexagonDataPrimeQuery(ctx context.Context, dataPrime *cxsdk.HexagonDataprimeQuery) (*HexagonQueryModel, diag.Diagnostics) {
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

	return &HexagonQueryModel{
		DataPrime: &DataPrimeModel{
			Query:   dataPrimeQuery,
			Filters: filters,
		},
	}, nil
}

func flattenHexagonLogsQuery(ctx context.Context, logs *cxsdk.HexagonLogsQuery) (*HexagonQueryModel, diag.Diagnostics) {
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

	return &HexagonQueryModel{
		Logs: &QueryLogsModel{
			LuceneQuery: utils.WrapperspbStringToTypeString(logs.GetLuceneQuery().GetValue()),
			Filters:     filters,
			GroupBy:     grouping,
		},
	}, nil
}

func flattenGroupingAggregations(ctx context.Context, aggregations []*cxsdk.DashboardDataTableLogsQueryAggregation) (types.List, diag.Diagnostics) {
	if len(aggregations) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: GroupingAggregationModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	aggregationElements := make([]attr.Value, 0, len(aggregations))
	for _, aggregation := range aggregations {
		flattenedAggregation, diags := flattenGroupingAggregation(ctx, aggregation)
		if diags.HasError() {
			diagnostics.Append(diags...)
			continue
		}
		aggregationElement, diags := types.ObjectValueFrom(ctx, GroupingAggregationModelAttr(), flattenedAggregation)
		if diags.HasError() {
			diagnostics.Append(diags...)
			continue
		}
		aggregationElements = append(aggregationElements, aggregationElement)
	}

	return types.ListValueMust(types.ObjectType{AttrTypes: GroupingAggregationModelAttr()}, aggregationElements), diagnostics
}

func flattenGroupingAggregation(ctx context.Context, dataTableAggregation *cxsdk.DashboardDataTableLogsQueryAggregation) (*DataTableLogsAggregationModel, diag.Diagnostics) {
	aggregation, diags := FlattenLogsAggregation(ctx, dataTableAggregation.GetAggregation())
	if diags.HasError() {
		return nil, diags
	}

	return &DataTableLogsAggregationModel{
		ID:          utils.WrapperspbStringToTypeString(dataTableAggregation.GetId()),
		Name:        utils.WrapperspbStringToTypeString(dataTableAggregation.GetName()),
		IsVisible:   utils.WrapperspbBoolToTypeBool(dataTableAggregation.GetIsVisible()),
		Aggregation: aggregation,
	}, nil
}

func flattenHexagonMetricsQuery(ctx context.Context, metrics *cxsdk.HexagonMetricsQuery) (*HexagonQueryModel, diag.Diagnostics) {
	if metrics == nil {
		return nil, nil
	}

	filters, diags := FlattenMetricsFilters(ctx, metrics.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	return &HexagonQueryModel{
		Metrics: &QueryMetricsModel{
			PromqlQuery: utils.WrapperspbStringToTypeString(metrics.GetPromqlQuery().GetValue()),
			Filters:     filters,
		},
	}, nil
}

func flattenHexagonSpansQuery(ctx context.Context, spans *cxsdk.HexagonSpansQuery) (*HexagonQueryModel, diag.Diagnostics) {
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

	return &HexagonQueryModel{
		Spans: &QuerySpansModel{
			LuceneQuery: utils.WrapperspbStringToTypeString(spans.GetLuceneQuery().GetValue()),
			Filters:     filters,
			GroupBy:     grouping,
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
