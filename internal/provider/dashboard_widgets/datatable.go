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

	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

func DataTableSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Attributes: map[string]schema.Attribute{
			"query": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"logs": schema.SingleNestedAttribute{
						Attributes: map[string]schema.Attribute{
							"lucene_query": schema.StringAttribute{
								Optional: true,
							},
							"filters": LogsFiltersSchema(),
							"grouping": schema.SingleNestedAttribute{
								Attributes: map[string]schema.Attribute{
									"aggregations": schema.ListNestedAttribute{
										NestedObject: schema.NestedAttributeObject{
											Attributes: map[string]schema.Attribute{
												"id": schema.StringAttribute{
													Computed: true,
													Optional: true,
													PlanModifiers: []planmodifier.String{
														stringplanmodifier.UseStateForUnknown(),
													},
												},
												"name": schema.StringAttribute{
													Optional: true,
												},
												"is_visible": schema.BoolAttribute{
													Optional: true,
													Computed: true,
													Default:  booldefault.StaticBool(true),
												},
												"aggregation": LogsAggregationSchema(),
											},
										},
										Optional: true,
									},
									"group_bys": schema.ListNestedAttribute{
										NestedObject: schema.NestedAttributeObject{
											Attributes: ObservationFieldSchema(),
										},
										Optional: true,
									},
								},
								Optional: true,
							},
							"time_frame": TimeFrameSchema(),
						},
						Optional: true,
						Validators: []validator.Object{
							objectvalidator.ExactlyOneOf(
								path.MatchRelative().AtParent().AtName("spans"),
								path.MatchRelative().AtParent().AtName("metrics"),
								path.MatchRelative().AtParent().AtName("data_prime"),
							),
						},
					},
					"spans": schema.SingleNestedAttribute{
						Attributes: map[string]schema.Attribute{
							"lucene_query": schema.StringAttribute{
								Optional: true,
							},
							"filters": SpansFilterSchema(),
							"grouping": schema.SingleNestedAttribute{
								Attributes: map[string]schema.Attribute{
									"group_by": SpansFieldsSchema(),
									"aggregations": schema.ListNestedAttribute{
										NestedObject: schema.NestedAttributeObject{
											Attributes: map[string]schema.Attribute{
												"id": schema.StringAttribute{
													Computed: true,
													PlanModifiers: []planmodifier.String{
														stringplanmodifier.UseStateForUnknown(),
													},
												},
												"name": schema.StringAttribute{
													Optional: true,
												},
												"is_visible": schema.BoolAttribute{
													Optional: true,
													Computed: true,
													Default:  booldefault.StaticBool(true),
												},
												"aggregation": SpansAggregationSchema(),
											},
										},
										Optional: true,
									},
								},
								Optional: true,
							},
							"time_frame": TimeFrameSchema(),
						},
						Optional: true,
						Validators: []validator.Object{
							objectvalidator.ExactlyOneOf(
								path.MatchRelative().AtParent().AtName("logs"),
								path.MatchRelative().AtParent().AtName("metrics"),
								path.MatchRelative().AtParent().AtName("data_prime"),
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
							"time_frame": TimeFrameSchema(),
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
						Validators: []validator.Object{
							objectvalidator.ExactlyOneOf(
								path.MatchRelative().AtParent().AtName("logs"),
								path.MatchRelative().AtParent().AtName("spans"),
								path.MatchRelative().AtParent().AtName("metrics"),
							),
						},
					},
				},
				Required: true,
			},
			"results_per_page": schema.Int64Attribute{
				Required:            true,
				MarkdownDescription: "The number of results to display per page.",
			},
			"row_style": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.OneOf(DashboardValidRowStyles...),
				},
				MarkdownDescription: fmt.Sprintf("The style of the rows. Can be one of %q.", DashboardValidRowStyles),
			},
			"columns": schema.ListNestedAttribute{
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"field": schema.StringAttribute{
							Required: true,
						},
						"width": schema.Int64Attribute{
							Optional: true,
							Computed: true,
							Default:  int64default.StaticInt64(0),
						},
					},
				},
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
				},
				Optional: true,
			},
			"order_by": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"field": schema.StringAttribute{
						Optional: true,
					},
					"order_direction": schema.StringAttribute{
						Validators: []validator.String{
							stringvalidator.OneOf(DashboardValidOrderDirections...),
						},
						MarkdownDescription: fmt.Sprintf("The order direction. Can be one of %q.", DashboardValidOrderDirections),
						Optional:            true,
						Computed:            true,
						Default:             stringdefault.StaticString(UNSPECIFIED),
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
				Default:             stringdefault.StaticString(UNSPECIFIED),
				MarkdownDescription: fmt.Sprintf("The data mode type. Can be one of %q.", DashboardValidDataModeTypes),
			},
		},
		Validators: []validator.Object{
			SupportedWidgetsValidatorWithout("data_table"),
			objectvalidator.AlsoRequires(
				path.MatchRelative().AtParent().AtParent().AtName("title"),
			),
		},
		Optional: true,
	}
}

func DataTableType() types.ObjectType {
	return types.ObjectType{
		AttrTypes: map[string]attr.Type{
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
							"grouping": types.ObjectType{
								AttrTypes: map[string]attr.Type{
									"aggregations": types.ListType{
										ElemType: types.ObjectType{
											AttrTypes: map[string]attr.Type{
												"id":         types.StringType,
												"name":       types.StringType,
												"is_visible": types.BoolType,
												"aggregation": types.ObjectType{
													AttrTypes: AggregationModelAttr(),
												},
											},
										},
									},
									"group_bys": types.ListType{
										ElemType: ObservationFieldsObject(),
									},
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
							"filters": types.ListType{
								ElemType: types.ObjectType{
									AttrTypes: SpansFilterModelAttr(),
								},
							},
							"grouping": types.ObjectType{
								AttrTypes: map[string]attr.Type{
									"group_by": types.ListType{
										ElemType: types.ObjectType{
											AttrTypes: SpansFieldModelAttr(),
										},
									},
									"aggregations": types.ListType{
										ElemType: types.ObjectType{
											AttrTypes: map[string]attr.Type{
												"id":         types.StringType,
												"name":       types.StringType,
												"is_visible": types.BoolType,
												"aggregation": types.ObjectType{
													AttrTypes: SpansAggregationModelAttr(),
												},
											},
										},
									},
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
			"results_per_page": types.Int64Type,
			"row_style":        types.StringType,
			"columns": types.ListType{
				ElemType: types.ObjectType{
					AttrTypes: dataTableColumnModelAttr(),
				},
			},
			"order_by": types.ObjectType{
				AttrTypes: map[string]attr.Type{
					"field":           types.StringType,
					"order_direction": types.StringType,
				},
			},
			"data_mode_type": types.StringType,
		},
	}
}

func dataTableColumnModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"field": types.StringType,
		"width": types.Int64Type,
	}
}

func FlattenDataTable(ctx context.Context, table *cxsdk.DashboardDataTable) (*WidgetDefinitionModel, diag.Diagnostics) {
	if table == nil {
		return nil, nil
	}

	query, diags := flattenDataTableQuery(ctx, table.GetQuery())
	if diags.HasError() {
		return nil, diags
	}

	columns, diags := flattenDataTableColumns(ctx, table.GetColumns())
	if diags.HasError() {
		return nil, diags
	}

	return &WidgetDefinitionModel{
		DataTable: &DataTableModel{
			Query:          query,
			ResultsPerPage: utils.WrapperspbInt32ToTypeInt64(table.GetResultsPerPage()),
			RowStyle:       types.StringValue(DashboardRowStyleProtoToSchema[table.GetRowStyle()]),
			Columns:        columns,
			OrderBy:        flattenOrderBy(table.GetOrderBy()),
			DataModeType:   types.StringValue(DashboardProtoToSchemaDataModeType[table.GetDataModeType()]),
		},
	}, nil
}

func flattenDataTableQuery(ctx context.Context, query *cxsdk.DashboardDataTableQuery) (*DataTableQueryModel, diag.Diagnostics) {
	if query == nil {
		return nil, nil
	}

	switch query.GetValue().(type) {
	case *cxsdk.DashboardDataTableQueryLogs:
		return flattenDataTableLogsQuery(ctx, query.GetLogs())
	case *cxsdk.DashboardDataTableQueryMetrics:
		return flattenDataTableMetricsQuery(ctx, query.GetMetrics())
	case *cxsdk.DashboardDataTableQuerySpans:
		return flattenDataTableSpansQuery(ctx, query.GetSpans())
	case *cxsdk.DashboardDataTableQueryDataprime:
		return flattenDataTableDataPrimeQuery(ctx, query.GetDataprime())
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Data Table Query", "unknown data table query type")}
	}
}

func flattenDataTableDataPrimeQuery(ctx context.Context, dataPrime *cxsdk.DashboardDataTableDataprimeQuery) (*DataTableQueryModel, diag.Diagnostics) {
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

	timeFrame, diags := FlattenTimeFrameSelect(ctx, dataPrime.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	return &DataTableQueryModel{
		DataPrime: &DataPrimeModel{
			Query:     dataPrimeQuery,
			Filters:   filters,
			TimeFrame: timeFrame,
		},
	}, nil
}

func flattenDataTableLogsQuery(ctx context.Context, logs *cxsdk.DashboardDataTableLogsQuery) (*DataTableQueryModel, diag.Diagnostics) {
	if logs == nil {
		return nil, nil
	}

	filters, diags := FlattenLogsFilters(ctx, logs.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	grouping, diags := flattenDataTableLogsQueryGrouping(ctx, logs.GetGrouping())
	if diags.HasError() {
		return nil, diags
	}

	timeFrame, diags := FlattenTimeFrameSelect(ctx, logs.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	return &DataTableQueryModel{
		Logs: &DataTableQueryLogsModel{
			LuceneQuery: utils.WrapperspbStringToTypeString(logs.GetLuceneQuery().GetValue()),
			Filters:     filters,
			Grouping:    grouping,
			TimeFrame:   timeFrame,
		},
	}, nil
}

func flattenDataTableLogsQueryGrouping(ctx context.Context, grouping *cxsdk.DashboardDataTableLogsQueryGrouping) (*DataTableLogsQueryGroupingModel, diag.Diagnostics) {
	if grouping == nil {
		return nil, nil
	}

	aggregations, diags := flattenGroupingAggregations(ctx, grouping.GetAggregations())
	if diags.HasError() {
		return nil, diags
	}

	groupBys, diags := FlattenObservationFields(ctx, grouping.GetGroupBys())
	if diags.HasError() {
		return nil, diags
	}

	return &DataTableLogsQueryGroupingModel{
		Aggregations: aggregations,
		GroupBys:     groupBys,
	}, nil
}

func flattenDataTableMetricsQuery(ctx context.Context, metrics *cxsdk.DashboardDataTableMetricsQuery) (*DataTableQueryModel, diag.Diagnostics) {
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

	return &DataTableQueryModel{
		Metrics: &QueryMetricsModel{
			PromqlQueryType: types.StringValue(DashboardProtoToSchemaPromQLQueryType[metrics.GetPromqlQueryType()]),
			PromqlQuery:     utils.WrapperspbStringToTypeString(metrics.GetPromqlQuery().GetValue()),
			Filters:         filters,
			TimeFrame:       timeFrame,
		},
	}, nil
}

func flattenDataTableSpansQuery(ctx context.Context, spans *cxsdk.DashboardDataTableSpansQuery) (*DataTableQueryModel, diag.Diagnostics) {
	if spans == nil {
		return nil, nil
	}

	filters, diags := FlattenSpansFilters(ctx, spans.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	grouping, diags := flattenDataTableSpansQueryGrouping(ctx, spans.GetGrouping())
	if diags.HasError() {
		return nil, diags
	}

	timeFrame, diags := FlattenTimeFrameSelect(ctx, spans.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	return &DataTableQueryModel{
		Spans: &DataTableQuerySpansModel{
			LuceneQuery: utils.WrapperspbStringToTypeString(spans.GetLuceneQuery().GetValue()),
			Filters:     filters,
			Grouping:    grouping,
			TimeFrame:   timeFrame,
		},
	}, nil
}

func flattenDataTableSpansQueryGrouping(ctx context.Context, grouping *cxsdk.DashboardDataTableSpansQueryGrouping) (*DataTableSpansQueryGroupingModel, diag.Diagnostics) {
	if grouping == nil {
		return nil, nil
	}

	aggregations, diags := flattenDataTableSpansQueryAggregations(ctx, grouping.GetAggregations())
	if diags.HasError() {
		return nil, diags
	}

	groupBy, diags := FlattenSpansFields(ctx, grouping.GetGroupBy())
	if diags.HasError() {
		return nil, diags
	}
	return &DataTableSpansQueryGroupingModel{
		Aggregations: aggregations,
		GroupBy:      groupBy,
	}, nil
}

func flattenDataTableSpansQueryAggregations(ctx context.Context, aggregations []*cxsdk.DashboardDataTableSpansQueryAggregation) (types.List, diag.Diagnostics) {
	if len(aggregations) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: SpansAggregationModelAttr()}), nil
	}
	var diagnostics diag.Diagnostics
	aggregationElements := make([]attr.Value, 0)
	for _, aggregation := range aggregations {
		flattenedAggregation, dg := flattenDataTableSpansQueryAggregation(aggregation)
		if dg != nil {
			diagnostics.Append(dg)
			continue
		}
		aggregationElement, diags := types.ObjectValueFrom(ctx, SpansAggregationModelAttr(), flattenedAggregation)
		if diags.HasError() {
			diagnostics = append(diagnostics, diags...)
			continue
		}
		aggregationElements = append(aggregationElements, aggregationElement)
	}

	if diagnostics.HasError() {
		return types.ListNull(types.ObjectType{AttrTypes: SpansAggregationModelAttr()}), diagnostics
	}

	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: SpansAggregationModelAttr()}, aggregationElements)
}

func flattenDataTableSpansQueryAggregation(spanAggregation *cxsdk.DashboardDataTableSpansQueryAggregation) (*DataTableSpansAggregationModel, diag.Diagnostic) {
	if spanAggregation == nil {
		return nil, nil
	}

	aggregation, dg := FlattenSpansAggregation(spanAggregation.GetAggregation())
	if dg != nil {
		return nil, dg
	}

	return &DataTableSpansAggregationModel{
		ID:          utils.WrapperspbStringToTypeString(spanAggregation.GetId()),
		Name:        utils.WrapperspbStringToTypeString(spanAggregation.GetName()),
		IsVisible:   utils.WrapperspbBoolToTypeBool(spanAggregation.GetIsVisible()),
		Aggregation: aggregation,
	}, nil
}

func flattenDataTableColumns(ctx context.Context, columns []*cxsdk.DashboardDataTableColumn) (types.List, diag.Diagnostics) {
	if len(columns) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: dataTableColumnModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	columnElements := make([]attr.Value, 0)
	for _, column := range columns {
		flattenedColumn := flattenDataTableColumn(column)
		columnElement, diags := types.ObjectValueFrom(ctx, dataTableColumnModelAttr(), flattenedColumn)
		if diags.HasError() {
			diagnostics = append(diagnostics, diags...)
			continue
		}
		columnElements = append(columnElements, columnElement)
	}

	if diagnostics.HasError() {
		return types.ListNull(types.ObjectType{AttrTypes: dataTableColumnModelAttr()}), diagnostics
	}

	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: dataTableColumnModelAttr()}, columnElements)
}

func flattenDataTableColumn(column *cxsdk.DashboardDataTableColumn) *DataTableColumnModel {
	if column == nil {
		return nil
	}
	return &DataTableColumnModel{
		Field: utils.WrapperspbStringToTypeString(column.GetField()),
		Width: utils.WrapperspbInt32ToTypeInt64(column.GetWidth()),
	}
}

func ExpandDataTable(ctx context.Context, table *DataTableModel) (*cxsdk.WidgetDefinition, diag.Diagnostics) {
	query, diags := expandDataTableQuery(ctx, table.Query)
	if diags.HasError() {
		return nil, diags
	}

	columns, diags := expandDataTableColumns(ctx, table.Columns)
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.WidgetDefinition{
		Value: &cxsdk.WidgetDefinitionDataTable{
			DataTable: &cxsdk.DashboardDataTable{
				Query:          query,
				ResultsPerPage: utils.TypeInt64ToWrappedInt32(table.ResultsPerPage),
				RowStyle:       DashboardRowStyleSchemaToProto[table.RowStyle.ValueString()],
				Columns:        columns,
				OrderBy:        expandOrderBy(table.OrderBy),
				DataModeType:   DashboardSchemaToProtoDataModeType[table.DataModeType.ValueString()],
			},
		},
	}, nil
}

func expandDataTableQuery(ctx context.Context, dataTableQuery *DataTableQueryModel) (*cxsdk.DashboardDataTableQuery, diag.Diagnostics) {
	if dataTableQuery == nil {
		return nil, nil
	}
	switch {
	case dataTableQuery.Metrics != nil:
		metrics, diags := expandDataTableMetricsQuery(ctx, dataTableQuery.Metrics)
		if diags.HasError() {
			return nil, diags
		}
		return &cxsdk.DashboardDataTableQuery{
			Value: metrics,
		}, nil
	case dataTableQuery.Logs != nil:
		logs, diags := expandDataTableLogsQuery(ctx, dataTableQuery.Logs)
		if diags.HasError() {
			return nil, diags
		}
		return &cxsdk.DashboardDataTableQuery{
			Value: logs,
		}, nil
	case dataTableQuery.Spans != nil:
		spans, diags := expandDataTableSpansQuery(ctx, dataTableQuery.Spans)
		if diags.HasError() {
			return nil, diags
		}
		return &cxsdk.DashboardDataTableQuery{
			Value: spans,
		}, nil
	case dataTableQuery.DataPrime != nil:
		dataPrime, diags := expandDataTableDataPrimeQuery(ctx, dataTableQuery.DataPrime)
		if diags.HasError() {
			return nil, diags
		}
		return &cxsdk.DashboardDataTableQuery{
			Value: dataPrime,
		}, nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand DataTable Query", fmt.Sprintf("unknown data table query type %#v", dataTableQuery))}
	}
}

func expandDataTableDataPrimeQuery(ctx context.Context, dataPrime *DataPrimeModel) (*cxsdk.DashboardDataTableQueryDataprime, diag.Diagnostics) {
	if dataPrime == nil {
		return nil, nil
	}

	filters, diags := ExpandDashboardFiltersSources(ctx, dataPrime.Filters)
	if diags.HasError() {
		return nil, diags
	}

	var dataPrimeQuery *cxsdk.DashboardDataprimeQuery
	if !dataPrime.Query.IsNull() {
		dataPrimeQuery = &cxsdk.DashboardDataprimeQuery{
			Text: dataPrime.Query.ValueString(),
		}
	}

	timeFrame, diags := ExpandTimeFrameSelect(ctx, dataPrime.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.DashboardDataTableQueryDataprime{
		Dataprime: &cxsdk.DashboardDataTableDataprimeQuery{
			DataprimeQuery: dataPrimeQuery,
			Filters:        filters,
			TimeFrame:      timeFrame,
		},
	}, nil
}

func expandDataTableMetricsQuery(ctx context.Context, dataTableQueryMetric *QueryMetricsModel) (*cxsdk.DashboardDataTableQueryMetrics, diag.Diagnostics) {
	if dataTableQueryMetric == nil {
		return nil, nil
	}

	filters, diags := ExpandMetricsFilters(ctx, dataTableQueryMetric.Filters)
	if diags.HasError() {
		return nil, diags
	}

	timeFrame, diags := ExpandTimeFrameSelect(ctx, dataTableQueryMetric.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.DashboardDataTableQueryMetrics{
		Metrics: &cxsdk.DashboardDataTableMetricsQuery{
			PromqlQuery:     ExpandPromqlQuery(dataTableQueryMetric.PromqlQuery),
			Filters:         filters,
			PromqlQueryType: expandPromqlQueryType(dataTableQueryMetric.PromqlQueryType),
			TimeFrame:       timeFrame,
		},
	}, nil
}

func expandPromqlQueryType(promqlQueryType basetypes.StringValue) cxsdk.PromQLQueryType {
	if promqlQueryType.ValueString() == "PROM_QL_QUERY_TYPE_INSTANT" {
		return cxsdk.PromQLQueryTypeInstant
	} else if promqlQueryType.ValueString() == "PROM_QL_QUERY_TYPE_RANGE" {
		return cxsdk.PromQLQueryTypeRange
	}
	return cxsdk.PromQLQueryTypeUnspecified
}

func expandDataTableLogsQuery(ctx context.Context, dataTableQueryLogs *DataTableQueryLogsModel) (*cxsdk.DashboardDataTableQueryLogs, diag.Diagnostics) {
	if dataTableQueryLogs == nil {
		return nil, nil
	}

	filters, diags := ExpandLogsFilters(ctx, dataTableQueryLogs.Filters)
	if diags.HasError() {
		return nil, diags
	}

	grouping, diags := expandDataTableLogsGrouping(ctx, dataTableQueryLogs.Grouping)
	if diags.HasError() {
		return nil, diags
	}

	timeframe, diags := ExpandTimeFrameSelect(ctx, dataTableQueryLogs.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}
	return &cxsdk.DashboardDataTableQueryLogs{
		Logs: &cxsdk.DashboardDataTableLogsQuery{
			LuceneQuery: ExpandLuceneQuery(dataTableQueryLogs.LuceneQuery),
			Filters:     filters,
			Grouping:    grouping,
			TimeFrame:   timeframe,
		},
	}, nil
}

func expandDataTableLogsGrouping(ctx context.Context, grouping *DataTableLogsQueryGroupingModel) (*cxsdk.DashboardDataTableLogsQueryGrouping, diag.Diagnostics) {
	if grouping == nil {
		return nil, nil
	}

	aggregations, diags := expandDataTableLogsAggregations(ctx, grouping.Aggregations)
	if diags.HasError() {
		return nil, diags
	}

	groupBys, diags := ExpandObservationFields(ctx, grouping.GroupBys)
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.DashboardDataTableLogsQueryGrouping{
		Aggregations: aggregations,
		GroupBys:     groupBys,
	}, nil

}

func expandDataTableLogsAggregations(ctx context.Context, aggregations types.List) ([]*cxsdk.DashboardDataTableLogsQueryAggregation, diag.Diagnostics) {
	var aggregationsObjects []types.Object
	var expandedAggregations []*cxsdk.DashboardDataTableLogsQueryAggregation
	diags := aggregations.ElementsAs(ctx, &aggregationsObjects, true)
	if diags.HasError() {
		return nil, diags
	}
	for _, ao := range aggregationsObjects {
		var aggregation DataTableLogsAggregationModel
		if dg := ao.As(ctx, &aggregation, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expandedAggregation, expandDiags := expandDataTableLogsAggregation(ctx, &aggregation)
		if expandDiags.HasError() {
			diags.Append(expandDiags...)
			continue
		}
		expandedAggregations = append(expandedAggregations, expandedAggregation)
	}

	return expandedAggregations, diags
}

func expandDataTableLogsAggregation(ctx context.Context, aggregation *DataTableLogsAggregationModel) (*cxsdk.DashboardDataTableLogsQueryAggregation, diag.Diagnostics) {
	if aggregation == nil {
		return nil, nil
	}

	logsAggregation, diags := ExpandLogsAggregation(ctx, aggregation.Aggregation)
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.DashboardDataTableLogsQueryAggregation{
		Id:          utils.TypeStringToWrapperspbString(aggregation.ID),
		Name:        utils.TypeStringToWrapperspbString(aggregation.Name),
		IsVisible:   utils.TypeBoolToWrapperspbBool(aggregation.IsVisible),
		Aggregation: logsAggregation,
	}, nil
}

func expandDataTableSpansQuery(ctx context.Context, dataTableQuerySpans *DataTableQuerySpansModel) (*cxsdk.DashboardDataTableQuerySpans, diag.Diagnostics) {
	if dataTableQuerySpans == nil {
		return nil, nil
	}

	filters, diags := ExpandSpansFilters(ctx, dataTableQuerySpans.Filters)
	if diags.HasError() {
		return nil, diags
	}

	grouping, diags := expandDataTableSpansGrouping(ctx, dataTableQuerySpans.Grouping)
	if diags.HasError() {
		return nil, diags
	}

	timeFrame, diags := ExpandTimeFrameSelect(ctx, dataTableQuerySpans.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.DashboardDataTableQuerySpans{
		Spans: &cxsdk.DashboardDataTableSpansQuery{
			LuceneQuery: ExpandLuceneQuery(dataTableQuerySpans.LuceneQuery),
			Filters:     filters,
			Grouping:    grouping,
			TimeFrame:   timeFrame,
		},
	}, nil
}

func expandDataTableSpansGrouping(ctx context.Context, grouping *DataTableSpansQueryGroupingModel) (*cxsdk.DashboardDataTableSpansQueryGrouping, diag.Diagnostics) {
	if grouping == nil {
		return nil, nil
	}

	groupBy, diags := ExpandSpansFields(ctx, grouping.GroupBy)
	if diags.HasError() {
		return nil, diags
	}

	aggregations, diags := expandDataTableSpansAggregations(ctx, grouping.Aggregations)
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.DashboardDataTableSpansQueryGrouping{
		GroupBy:      groupBy,
		Aggregations: aggregations,
	}, nil
}

func expandDataTableSpansAggregations(ctx context.Context, spansAggregations types.List) ([]*cxsdk.DashboardDataTableSpansQueryAggregation, diag.Diagnostics) {
	var spansAggregationsObjects []types.Object
	var expandedSpansAggregations []*cxsdk.DashboardDataTableSpansQueryAggregation
	diags := spansAggregations.ElementsAs(ctx, &spansAggregationsObjects, true)
	if diags.HasError() {
		return nil, diags
	}
	for _, sfo := range spansAggregationsObjects {
		var aggregation DataTableSpansAggregationModel
		if dg := sfo.As(ctx, &aggregation, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expandedSpansAggregation, expandDiag := expandDataTableSpansAggregation(&aggregation)
		if expandDiag != nil {
			diags.Append(expandDiag)
			continue
		}
		expandedSpansAggregations = append(expandedSpansAggregations, expandedSpansAggregation)
	}

	return expandedSpansAggregations, diags
}

func expandDataTableSpansAggregation(aggregation *DataTableSpansAggregationModel) (*cxsdk.DashboardDataTableSpansQueryAggregation, diag.Diagnostic) {
	if aggregation == nil {
		return nil, nil
	}

	spansAggregation, dg := ExpandSpansAggregation(aggregation.Aggregation)
	if dg != nil {
		return nil, dg
	}

	return &cxsdk.DashboardDataTableSpansQueryAggregation{
		Id:          utils.TypeStringToWrapperspbString(aggregation.ID),
		Name:        utils.TypeStringToWrapperspbString(aggregation.Name),
		IsVisible:   utils.TypeBoolToWrapperspbBool(aggregation.IsVisible),
		Aggregation: spansAggregation,
	}, nil
}

func expandDataTableColumns(ctx context.Context, columns types.List) ([]*cxsdk.DashboardDataTableColumn, diag.Diagnostics) {
	var columnsObjects []types.Object
	var expandedColumns []*cxsdk.DashboardDataTableColumn
	diags := columns.ElementsAs(ctx, &columnsObjects, true)
	if diags.HasError() {
		return nil, diags
	}
	for _, co := range columnsObjects {
		var column DataTableColumnModel
		if dg := co.As(ctx, &column, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expandedColumn := expandDataTableColumn(column)
		expandedColumns = append(expandedColumns, expandedColumn)
	}

	return expandedColumns, diags
}

func expandDataTableColumn(column DataTableColumnModel) *cxsdk.DashboardDataTableColumn {
	return &cxsdk.DashboardDataTableColumn{
		Field: utils.TypeStringToWrapperspbString(column.Field),
		Width: utils.TypeInt64ToWrappedInt32(column.Width),
	}
}

func expandOrderBy(orderBy *OrderByModel) *cxsdk.DashboardOrderingField {
	if orderBy == nil {
		return nil
	}
	return &cxsdk.DashboardOrderingField{
		Field:          utils.TypeStringToWrapperspbString(orderBy.Field),
		OrderDirection: DashboardOrderDirectionSchemaToProto[orderBy.OrderDirection.ValueString()],
	}
}

func flattenOrderBy(orderBy *cxsdk.DashboardOrderingField) *OrderByModel {
	if orderBy == nil {
		return nil
	}
	return &OrderByModel{
		Field:          utils.WrapperspbStringToTypeString(orderBy.GetField()),
		OrderDirection: types.StringValue(DashboardOrderDirectionProtoToSchema[orderBy.GetOrderDirection()]),
	}
}

func flattenGroupingAggregations(ctx context.Context, aggregations []*cxsdk.DashboardDataTableLogsQueryAggregation) (types.List, diag.Diagnostics) {
	if len(aggregations) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: GroupingAggregationModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	aggregationElements := make([]attr.Value, 0)
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
