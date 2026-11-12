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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
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
														stringplanmodifier.UseNonNullStateForUnknown(),
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
							"filters": SpansObservationFiltersSchema(),
							"grouping": schema.SingleNestedAttribute{
								Attributes: map[string]schema.Attribute{
									"group_by": SpansFieldsSchema(),
									"aggregations": schema.ListNestedAttribute{
										NestedObject: schema.NestedAttributeObject{
											Attributes: map[string]schema.Attribute{
												"id": schema.StringAttribute{
													Computed: true,
													PlanModifiers: []planmodifier.String{
														stringplanmodifier.UseNonNullStateForUnknown(),
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
							"editor_mode": MetricsEditorModeSchema(),
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
						// The proto declares the column width as a wrapper value, so
						// an omitted width is absent and not zero. A static default
						// would send a width nobody asked for, so there is none.
						// Computed keeps the width the API returns for a dashboard
						// created elsewhere. The plan modifier copies the prior
						// state, including a null one, so a column the API left
						// without a width does not plan "known after apply" on
						// every run.
						"width": schema.Int64Attribute{
							Optional: true,
							Computed: true,
							PlanModifiers: []planmodifier.Int64{
								int64planmodifier.UseStateForUnknown(),
							},
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
			EditorMode:      FlattenEnum(metrics.GetEditorMode(), DashboardProtoToSchemaMetricsEditorMode),
			TimeFrame:       timeFrame,
		},
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
		EditorMode:      OptionalEnumPointer(dataTableQueryMetric.EditorMode, DashboardSchemaToProtoMetricsEditorMode),
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
