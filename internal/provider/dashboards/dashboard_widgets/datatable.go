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

	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	dashboardservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_service"

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
								Validators: []validator.String{
									stringvalidator.OneOf(DashboardValidPromQLQueryType...),
								},
							},
							"time_frame": TimeFrameSchema(),
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
									Validators: []validator.Object{
										ExactlyOneOfChildren("logs", "metrics", "spans"),
									},
								},
								Optional: true,
							},
							"time_frame": TimeFrameSchema(),
						},
						Optional: true,
					},
				},
				Required: true,
				Validators: []validator.Object{
					ExactlyOneOfChildren("logs", "spans", "metrics", "data_prime"),
				},
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
						Default:             stringdefault.StaticString(utils.UNSPECIFIED),
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
				Default:             stringdefault.StaticString(utils.UNSPECIFIED),
				MarkdownDescription: fmt.Sprintf("The data mode type. Can be one of %q.", DashboardValidDataModeTypes),
			},
		},
		Validators: []validator.Object{
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
											AttrTypes: DataTableSpansAggregationModelAttr(),
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

func FlattenDataTable(ctx context.Context, table *dashboardservice.DataTable) (*WidgetDefinitionModel, diag.Diagnostics) {
	if table == nil {
		return nil, nil
	}

	query, diags := flattenDataTableQuery(ctx, table.Query)
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
			ResultsPerPage: int32PointerToInt64Type(table.ResultsPerPage),
			RowStyle:       types.StringValue(DashboardRowStyleProtoToSchema[table.GetRowStyle()]),
			Columns:        columns,
			OrderBy:        flattenOrderBy(table.OrderBy),
			DataModeType:   types.StringValue(DashboardProtoToSchemaDataModeType[table.GetDataModeType()]),
		},
	}, nil
}

func flattenDataTableQuery(ctx context.Context, query *dashboardservice.DataTableQuery) (*DataTableQueryModel, diag.Diagnostics) {
	if query == nil {
		return nil, nil
	}

	switch {
	case query.Logs != nil:
		return flattenDataTableLogsQuery(ctx, query.Logs)
	case query.Metrics != nil:
		return flattenDataTableMetricsQuery(ctx, query.Metrics)
	case query.Spans != nil:
		return flattenDataTableSpansQuery(ctx, query.Spans)
	case query.Dataprime != nil:
		return flattenDataTableDataPrimeQuery(ctx, query.Dataprime)
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Data Table Query", "unknown data table query type")}
	}
}

func flattenDataTableDataPrimeQuery(ctx context.Context, dataPrime *dashboardservice.DataTableDataprimeQuery) (*DataTableQueryModel, diag.Diagnostics) {
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

func flattenDataTableLogsQuery(ctx context.Context, logs *dashboardservice.DataTableLogsQuery) (*DataTableQueryModel, diag.Diagnostics) {
	if logs == nil {
		return nil, nil
	}

	filters, diags := FlattenLogsFilters(ctx, logs.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	grouping, diags := flattenDataTableLogsQueryGrouping(ctx, logs.Grouping)
	if diags.HasError() {
		return nil, diags
	}

	timeFrame, diags := FlattenTimeFrameSelect(ctx, logs.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	return &DataTableQueryModel{
		Logs: &DataTableQueryLogsModel{
			LuceneQuery: flattenLuceneQuery(logs.LuceneQuery),
			Filters:     filters,
			Grouping:    grouping,
			TimeFrame:   timeFrame,
		},
	}, nil
}

func flattenDataTableLogsQueryGrouping(ctx context.Context, grouping *dashboardservice.LogsQueryGrouping) (*DataTableLogsQueryGroupingModel, diag.Diagnostics) {
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

func flattenDataTableMetricsQuery(ctx context.Context, metrics *dashboardservice.DataTableMetricsQuery) (*DataTableQueryModel, diag.Diagnostics) {
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
			PromqlQuery:     flattenPromqlQuery(metrics.PromqlQuery),
			Filters:         filters,
			TimeFrame:       timeFrame,
		},
	}, nil
}

func flattenDataTableSpansQuery(ctx context.Context, spans *dashboardservice.DataTableSpansQuery) (*DataTableQueryModel, diag.Diagnostics) {
	if spans == nil {
		return nil, nil
	}

	filters, diags := FlattenSpansFilters(ctx, spans.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	grouping, diags := flattenDataTableSpansQueryGrouping(ctx, spans.Grouping)
	if diags.HasError() {
		return nil, diags
	}

	timeFrame, diags := FlattenTimeFrameSelect(ctx, spans.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	return &DataTableQueryModel{
		Spans: &DataTableQuerySpansModel{
			LuceneQuery: flattenLuceneQuery(spans.LuceneQuery),
			Filters:     filters,
			Grouping:    grouping,
			TimeFrame:   timeFrame,
		},
	}, nil
}

func flattenDataTableSpansQueryGrouping(ctx context.Context, grouping *dashboardservice.SpansQueryGrouping) (*DataTableSpansQueryGroupingModel, diag.Diagnostics) {
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

func flattenDataTableSpansQueryAggregations(ctx context.Context, aggregations []dashboardservice.SpansQueryAggregation) (types.List, diag.Diagnostics) {
	aggregationType := types.ObjectType{AttrTypes: DataTableSpansAggregationModelAttr()}
	if len(aggregations) == 0 {
		return types.ListNull(aggregationType), nil
	}
	var diagnostics diag.Diagnostics
	aggregationElements := make([]attr.Value, 0)
	for _, aggregation := range aggregations {
		flattenedAggregation, dg := flattenDataTableSpansQueryAggregation(&aggregation)
		if dg != nil {
			diagnostics.Append(dg)
			continue
		}
		aggregationElement, diags := types.ObjectValueFrom(ctx, DataTableSpansAggregationModelAttr(), flattenedAggregation)
		if diags.HasError() {
			diagnostics = append(diagnostics, diags...)
			continue
		}
		aggregationElements = append(aggregationElements, aggregationElement)
	}

	if diagnostics.HasError() {
		return types.ListNull(aggregationType), diagnostics
	}

	return types.ListValueFrom(ctx, aggregationType, aggregationElements)
}

func flattenDataTableSpansQueryAggregation(spanAggregation *dashboardservice.SpansQueryAggregation) (*DataTableSpansAggregationModel, diag.Diagnostic) {
	if spanAggregation == nil {
		return nil, nil
	}

	aggregation, dg := FlattenSpansAggregation(spanAggregation.Aggregation)
	if dg != nil {
		return nil, dg
	}

	return &DataTableSpansAggregationModel{
		ID:          utils.StringPointerToTypeString(spanAggregation.Id),
		Name:        utils.StringPointerToTypeString(spanAggregation.Name),
		IsVisible:   types.BoolPointerValue(spanAggregation.IsVisible),
		Aggregation: aggregation,
	}, nil
}

func flattenDataTableColumns(ctx context.Context, columns []dashboardservice.DataTableColumn) (types.List, diag.Diagnostics) {
	if len(columns) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: dataTableColumnModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	columnElements := make([]attr.Value, 0)
	for _, column := range columns {
		flattenedColumn := flattenDataTableColumn(&column)
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

func flattenDataTableColumn(column *dashboardservice.DataTableColumn) *DataTableColumnModel {
	if column == nil {
		return nil
	}
	return &DataTableColumnModel{
		Field: utils.StringPointerToTypeString(column.Field),
		Width: int32PointerToInt64Type(column.Width),
	}
}

func ExpandDataTable(ctx context.Context, table *DataTableModel) (*dashboardservice.WidgetDefinition, diag.Diagnostics) {
	query, diags := expandDataTableQuery(ctx, table.Query)
	if diags.HasError() {
		return nil, diags
	}

	columns, diags := expandDataTableColumns(ctx, table.Columns)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.WidgetDefinition{
		DataTable: &dashboardservice.DataTable{
			Query:          query,
			ResultsPerPage: int64ToInt32Pointer(table.ResultsPerPage),
			RowStyle:       OptionalEnumPointer(table.RowStyle, DashboardRowStyleSchemaToProto),
			Columns:        columns,
			OrderBy:        expandOrderBy(table.OrderBy),
			DataModeType:   OptionalEnumPointer(table.DataModeType, DashboardSchemaToProtoDataModeType),
		},
	}, nil
}

func expandDataTableQuery(ctx context.Context, dataTableQuery *DataTableQueryModel) (*dashboardservice.DataTableQuery, diag.Diagnostics) {
	if dataTableQuery == nil {
		return nil, nil
	}
	switch {
	case dataTableQuery.Metrics != nil:
		metrics, diags := expandDataTableMetricsQuery(ctx, dataTableQuery.Metrics)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboardservice.DataTableQuery{
			Metrics: metrics,
		}, nil
	case dataTableQuery.Logs != nil:
		logs, diags := expandDataTableLogsQuery(ctx, dataTableQuery.Logs)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboardservice.DataTableQuery{
			Logs: logs,
		}, nil
	case dataTableQuery.Spans != nil:
		spans, diags := expandDataTableSpansQuery(ctx, dataTableQuery.Spans)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboardservice.DataTableQuery{
			Spans: spans,
		}, nil
	case dataTableQuery.DataPrime != nil:
		dataPrime, diags := expandDataTableDataPrimeQuery(ctx, dataTableQuery.DataPrime)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboardservice.DataTableQuery{
			Dataprime: dataPrime,
		}, nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand DataTable Query", fmt.Sprintf("unknown data table query type %#v", dataTableQuery))}
	}
}

func expandDataTableDataPrimeQuery(ctx context.Context, dataPrime *DataPrimeModel) (*dashboardservice.DataTableDataprimeQuery, diag.Diagnostics) {
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

	timeFrame, diags := ExpandTimeFrameSelect(ctx, dataPrime.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.DataTableDataprimeQuery{
		DataprimeQuery: dataPrimeQuery,
		Filters:        filters,
		TimeFrame:      timeFrame,
	}, nil
}

func expandDataTableMetricsQuery(ctx context.Context, dataTableQueryMetric *QueryMetricsModel) (*dashboardservice.DataTableMetricsQuery, diag.Diagnostics) {
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

	return &dashboardservice.DataTableMetricsQuery{
		PromqlQuery:     ExpandPromqlQuery(dataTableQueryMetric.PromqlQuery),
		Filters:         filters,
		PromqlQueryType: expandPromqlQueryType(dataTableQueryMetric.PromqlQueryType).Ptr(),
		TimeFrame:       timeFrame,
	}, nil
}

func expandPromqlQueryType(promqlQueryType basetypes.StringValue) dashboardservice.PromQLQueryType {
	ty, found := DashboardSchemaToProtoPromQLQueryType[promqlQueryType.ValueString()]
	if found {
		return ty
	}
	return dashboardservice.PROMQLQUERYTYPE_PROM_QL_QUERY_TYPE_UNSPECIFIED
}

func expandDataTableLogsQuery(ctx context.Context, dataTableQueryLogs *DataTableQueryLogsModel) (*dashboardservice.DataTableLogsQuery, diag.Diagnostics) {
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
	return &dashboardservice.DataTableLogsQuery{
		LuceneQuery: ExpandLuceneQuery(dataTableQueryLogs.LuceneQuery),
		Filters:     filters,
		Grouping:    grouping,
		TimeFrame:   timeframe,
	}, nil
}

func expandDataTableLogsGrouping(ctx context.Context, grouping *DataTableLogsQueryGroupingModel) (*dashboardservice.LogsQueryGrouping, diag.Diagnostics) {
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

	return &dashboardservice.LogsQueryGrouping{
		Aggregations: aggregations,
		GroupBys:     groupBys,
	}, nil

}

func expandDataTableLogsAggregations(ctx context.Context, aggregations types.List) ([]dashboardservice.LogsQueryAggregation, diag.Diagnostics) {
	var aggregationsObjects []types.Object
	var expandedAggregations []dashboardservice.LogsQueryAggregation
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
		expandedAggregations = append(expandedAggregations, *expandedAggregation)
	}

	return expandedAggregations, diags
}

func expandDataTableLogsAggregation(ctx context.Context, aggregation *DataTableLogsAggregationModel) (*dashboardservice.LogsQueryAggregation, diag.Diagnostics) {
	if aggregation == nil {
		return nil, nil
	}

	logsAggregation, diags := ExpandLogsAggregation(ctx, aggregation.Aggregation)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.LogsQueryAggregation{
		Id:          utils.TypeStringToStringPointer(aggregation.ID),
		Name:        utils.TypeStringToStringPointer(aggregation.Name),
		IsVisible:   aggregation.IsVisible.ValueBoolPointer(),
		Aggregation: logsAggregation,
	}, nil
}

func expandDataTableSpansQuery(ctx context.Context, dataTableQuerySpans *DataTableQuerySpansModel) (*dashboardservice.DataTableSpansQuery, diag.Diagnostics) {
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

	return &dashboardservice.DataTableSpansQuery{
		LuceneQuery: ExpandLuceneQuery(dataTableQuerySpans.LuceneQuery),
		Filters:     filters,
		Grouping:    grouping,
		TimeFrame:   timeFrame,
	}, nil
}

func expandDataTableSpansGrouping(ctx context.Context, grouping *DataTableSpansQueryGroupingModel) (*dashboardservice.SpansQueryGrouping, diag.Diagnostics) {
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

	return &dashboardservice.SpansQueryGrouping{
		GroupBy:      groupBy,
		Aggregations: aggregations,
	}, nil
}

func expandDataTableSpansAggregations(ctx context.Context, spansAggregations types.List) ([]dashboardservice.SpansQueryAggregation, diag.Diagnostics) {
	var spansAggregationsObjects []types.Object
	var expandedSpansAggregations []dashboardservice.SpansQueryAggregation
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
		expandedSpansAggregations = append(expandedSpansAggregations, *expandedSpansAggregation)
	}

	return expandedSpansAggregations, diags
}

func expandDataTableSpansAggregation(aggregation *DataTableSpansAggregationModel) (*dashboardservice.SpansQueryAggregation, diag.Diagnostic) {
	if aggregation == nil {
		return nil, nil
	}

	spansAggregation, dg := ExpandSpansAggregation(aggregation.Aggregation)
	if dg != nil {
		return nil, dg
	}

	return &dashboardservice.SpansQueryAggregation{
		Id:          utils.TypeStringToStringPointer(aggregation.ID),
		Name:        utils.TypeStringToStringPointer(aggregation.Name),
		IsVisible:   aggregation.IsVisible.ValueBoolPointer(),
		Aggregation: spansAggregation,
	}, nil
}

func expandDataTableColumns(ctx context.Context, columns types.List) ([]dashboardservice.DataTableColumn, diag.Diagnostics) {
	var columnsObjects []types.Object
	var expandedColumns []dashboardservice.DataTableColumn
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
		expandedColumns = append(expandedColumns, *expandedColumn)
	}

	return expandedColumns, diags
}

func expandDataTableColumn(column DataTableColumnModel) *dashboardservice.DataTableColumn {
	return &dashboardservice.DataTableColumn{
		Field: utils.TypeStringToStringPointer(column.Field),
		Width: int64ToInt32Pointer(column.Width),
	}
}

func expandOrderBy(orderBy *OrderByModel) *dashboardservice.OrderingField {
	if orderBy == nil {
		return nil
	}
	return &dashboardservice.OrderingField{
		Field:          utils.TypeStringToStringPointer(orderBy.Field),
		OrderDirection: OptionalEnumPointer(orderBy.OrderDirection, DashboardOrderDirectionSchemaToProto),
	}
}

func flattenOrderBy(orderBy *dashboardservice.OrderingField) *OrderByModel {
	if orderBy == nil {
		return nil
	}
	return &OrderByModel{
		Field:          utils.StringPointerToTypeString(orderBy.Field),
		OrderDirection: types.StringValue(DashboardOrderDirectionProtoToSchema[orderBy.GetOrderDirection()]),
	}
}

func flattenGroupingAggregations(ctx context.Context, aggregations []dashboardservice.LogsQueryAggregation) (types.List, diag.Diagnostics) {
	if len(aggregations) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: GroupingAggregationModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	aggregationElements := make([]attr.Value, 0)
	for _, aggregation := range aggregations {
		flattenedAggregation, diags := flattenGroupingAggregation(ctx, &aggregation)
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

func flattenGroupingAggregation(ctx context.Context, dataTableAggregation *dashboardservice.LogsQueryAggregation) (*DataTableLogsAggregationModel, diag.Diagnostics) {
	aggregation, diags := FlattenLogsAggregation(ctx, dataTableAggregation.Aggregation)
	if diags.HasError() {
		return nil, diags
	}

	return &DataTableLogsAggregationModel{
		ID:          utils.StringPointerToTypeString(dataTableAggregation.Id),
		Name:        utils.StringPointerToTypeString(dataTableAggregation.Name),
		IsVisible:   types.BoolPointerValue(dataTableAggregation.IsVisible),
		Aggregation: aggregation,
	}, nil
}
