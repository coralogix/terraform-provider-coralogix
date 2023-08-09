package coralogix

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/golang/protobuf/jsonpb"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"terraform-provider-coralogix/coralogix/clientset"
	dashboards "terraform-provider-coralogix/coralogix/clientset/grpc/coralogix-dashboards/v1"
)

var (
	dashboardRowStyleSchemaToProto = map[string]dashboards.RowStyle{
		"unspecified": dashboards.RowStyle_ROW_STYLE_UNSPECIFIED,
		"one_line":    dashboards.RowStyle_ROW_STYLE_ONE_LINE,
		"two_line":    dashboards.RowStyle_ROW_STYLE_TWO_LINE,
		"condensed":   dashboards.RowStyle_ROW_STYLE_CONDENSED,
		"json":        dashboards.RowStyle_ROW_STYLE_JSON,
	}
	dashboardRowStyleProtoToSchema     = ReverseMap(dashboardRowStyleSchemaToProto)
	dashboardValidRowStyles            = GetKeys(dashboardRowStyleSchemaToProto)
	dashboardLegendColumnSchemaToProto = map[string]dashboards.Legend_LegendColumn{
		"unspecified": dashboards.Legend_LEGEND_COLUMN_UNSPECIFIED,
		"min":         dashboards.Legend_LEGEND_COLUMN_MIN,
		"max":         dashboards.Legend_LEGEND_COLUMN_MAX,
		"sum":         dashboards.Legend_LEGEND_COLUMN_SUM,
		"avg":         dashboards.Legend_LEGEND_COLUMN_AVG,
		"last":        dashboards.Legend_LEGEND_COLUMN_LAST,
	}
	dashboardLegendColumnProtoToSchema   = ReverseMap(dashboardLegendColumnSchemaToProto)
	dashboardValidLegendColumns          = GetKeys(dashboardLegendColumnSchemaToProto)
	dashboardOrderDirectionSchemaToProto = map[string]dashboards.OrderDirection{
		"unspecified": dashboards.OrderDirection_ORDER_DIRECTION_UNSPECIFIED,
		"asc":         dashboards.OrderDirection_ORDER_DIRECTION_ASC,
		"desc":        dashboards.OrderDirection_ORDER_DIRECTION_DESC,
	}
	dashboardOrderDirectionProtoToSchema = ReverseMap(dashboardOrderDirectionSchemaToProto)
	dashboardValidOrderDirection         = GetKeys(dashboardOrderDirectionSchemaToProto)
	dashboardAggregationSchemaToProto    = map[string]dashboards.Gauge_Aggregation{
		"unspecified": dashboards.Gauge_AGGREGATION_UNSPECIFIED,
		"last":        dashboards.Gauge_AGGREGATION_LAST,
		"min":         dashboards.Gauge_AGGREGATION_MIN,
		"max":         dashboards.Gauge_AGGREGATION_MAX,
		"avg":         dashboards.Gauge_AGGREGATION_AVG,
		"sum":         dashboards.Gauge_AGGREGATION_SUM,
	}
	dashboardAggregationProtoToSchema        = ReverseMap(dashboardAggregationSchemaToProto)
	dashboardValidAggregation                = GetKeys(dashboardAggregationSchemaToProto)
	dashboardSchemaGaugeUnitToProtoGaugeUnit = map[string]string{
		"Unspecified": "Gauge_UNIT_UNSPECIFIED",
		"Number":      "Gauge_UNIT_NUMBER",
		"Percent":     "Gauge_UNIT_PERCENT",
	}
	dashboardProtoGaugeUnitToSchemaGaugeUnit = reverseMapStrings(dashboardSchemaGaugeUnitToProtoGaugeUnit)
	dashboardValidGaugeUnit                  = getKeysStrings(dashboardSchemaGaugeUnitToProtoGaugeUnit)
	dashboardSchemaToProtoTooltipType        = map[string]dashboards.LineChart_TooltipType{
		"unspecified": dashboards.LineChart_TOOLTIP_TYPE_UNSPECIFIED,
		"all":         dashboards.LineChart_TOOLTIP_TYPE_ALL,
		"single":      dashboards.LineChart_TOOLTIP_TYPE_SINGLE,
	}
	dashboardProtoToSchemaTooltipType = ReverseMap(dashboardSchemaToProtoTooltipType)
	dashboardValidTooltipType         = GetKeys(dashboardSchemaToProtoTooltipType)
	dashboardSchemaToProtoScaleType   = map[string]dashboards.ScaleType{
		"unspecified": dashboards.ScaleType_SCALE_TYPE_UNSPECIFIED,
		"linear":      dashboards.ScaleType_SCALE_TYPE_LINEAR,
		"logarithmic": dashboards.ScaleType_SCALE_TYPE_LOGARITHMIC,
	}
	dashboardProtoToSchemaScaleType = ReverseMap(dashboardSchemaToProtoScaleType)
	dashboardValidScaleType         = GetKeys(dashboardSchemaToProtoScaleType)
	dashboardSchemaToProtoUnit      = map[string]dashboards.Unit{
		"UNSPECIFIED":  dashboards.Unit_UNIT_UNSPECIFIED,
		"MICROSECONDS": dashboards.Unit_UNIT_MICROSECONDS,
		"MILLISECONDS": dashboards.Unit_UNIT_MILLISECONDS,
		"SECONDS":      dashboards.Unit_UNIT_SECONDS,
		"BYTES":        dashboards.Unit_UNIT_BYTES,
		"KBYTES":       dashboards.Unit_UNIT_KBYTES,
		"MBYTES":       dashboards.Unit_UNIT_MBYTES,
		"GBYTES":       dashboards.Unit_UNIT_GBYTES,
		"BYTES_IEC":    dashboards.Unit_UNIT_BYTES_IEC,
		"KIBYTES":      dashboards.Unit_UNIT_KIBYTES,
		"MIBYTES":      dashboards.Unit_UNIT_MIBYTES,
		"GIBYTES":      dashboards.Unit_UNIT_GIBYTES,
	}
	dashboardProtoToSchemaUnit                = ReverseMap(dashboardSchemaToProtoUnit)
	dashboardValidUnit                        = GetKeys(dashboardSchemaToProtoUnit)
	dashboardSchemaToProtoPieChartLabelSource = map[string]dashboards.PieChart_LabelSource{
		"unspecified": dashboards.PieChart_LABEL_SOURCE_UNSPECIFIED,
		"inner":       dashboards.PieChart_LABEL_SOURCE_INNER,
		"stack":       dashboards.PieChart_LABEL_SOURCE_STACK,
	}
	dashboardProtoToSchemaPieChartLabelSource = ReverseMap(dashboardSchemaToProtoPieChartLabelSource)
	dashboardValidPieChartLabelSource         = GetKeys(dashboardSchemaToProtoPieChartLabelSource)
	dashboardSchemaToProtoGaugeAggregation    = map[string]dashboards.Gauge_Aggregation{
		"unspecified": dashboards.Gauge_AGGREGATION_UNSPECIFIED,
		"last":        dashboards.Gauge_AGGREGATION_LAST,
		"min":         dashboards.Gauge_AGGREGATION_MIN,
		"max":         dashboards.Gauge_AGGREGATION_MAX,
		"avg":         dashboards.Gauge_AGGREGATION_AVG,
		"sum":         dashboards.Gauge_AGGREGATION_SUM,
	}
	dashboardProtoToSchemaGaugeAggregation            = ReverseMap(dashboardSchemaToProtoGaugeAggregation)
	dashboardValidGaugeAggregation                    = GetKeys(dashboardSchemaToProtoGaugeAggregation)
	dashboardSchemaToProtoSpansAggregationMetricField = map[string]dashboards.SpansAggregation_MetricAggregation_MetricField{
		"unspecified": dashboards.SpansAggregation_MetricAggregation_METRIC_FIELD_UNSPECIFIED,
		"duration":    dashboards.SpansAggregation_MetricAggregation_METRIC_FIELD_DURATION,
	}
	dashboardProtoToSchemaSpansAggregationMetricField           = ReverseMap(dashboardSchemaToProtoSpansAggregationMetricField)
	dashboardValidSpansAggregationMetricField                   = GetKeys(dashboardSchemaToProtoSpansAggregationMetricField)
	dashboardSchemaToProtoSpansAggregationMetricAggregationType = map[string]dashboards.SpansAggregation_MetricAggregation_MetricAggregationType{
		"unspecified":   dashboards.SpansAggregation_MetricAggregation_METRIC_AGGREGATION_TYPE_UNSPECIFIED,
		"min":           dashboards.SpansAggregation_MetricAggregation_METRIC_AGGREGATION_TYPE_MIN,
		"max":           dashboards.SpansAggregation_MetricAggregation_METRIC_AGGREGATION_TYPE_MAX,
		"avg":           dashboards.SpansAggregation_MetricAggregation_METRIC_AGGREGATION_TYPE_AVERAGE,
		"sum":           dashboards.SpansAggregation_MetricAggregation_METRIC_AGGREGATION_TYPE_SUM,
		"percentile_99": dashboards.SpansAggregation_MetricAggregation_METRIC_AGGREGATION_TYPE_PERCENTILE_99,
		"percentile_95": dashboards.SpansAggregation_MetricAggregation_METRIC_AGGREGATION_TYPE_PERCENTILE_95,
		"percentile_50": dashboards.SpansAggregation_MetricAggregation_METRIC_AGGREGATION_TYPE_PERCENTILE_50,
	}
	dashboardProtoToSchemaSpansAggregationDimensionField = map[string]dashboards.SpansAggregation_DimensionAggregation_DimensionField{
		"unspecified": dashboards.SpansAggregation_DimensionAggregation_DIMENSION_FIELD_UNSPECIFIED,
		"trace_id":    dashboards.SpansAggregation_DimensionAggregation_DIMENSION_FIELD_TRACE_ID,
	}
	dashboardSchemaToProtoSpansAggregationDimensionField           = ReverseMap(dashboardProtoToSchemaSpansAggregationDimensionField)
	dashboardValidSpansAggregationDimensionFields                  = GetKeys(dashboardProtoToSchemaSpansAggregationDimensionField)
	dashboardSchemaToProtoSpansAggregationDimensionAggregationType = map[string]dashboards.SpansAggregation_DimensionAggregation_DimensionAggregationType{
		"unspecified":  dashboards.SpansAggregation_DimensionAggregation_DIMENSION_AGGREGATION_TYPE_UNSPECIFIED,
		"unique_count": dashboards.SpansAggregation_DimensionAggregation_DIMENSION_AGGREGATION_TYPE_UNIQUE_COUNT,
		"error_count":  dashboards.SpansAggregation_DimensionAggregation_DIMENSION_AGGREGATION_TYPE_ERROR_COUNT,
	}
	dashboardProtoToSchemaSpansAggregationDimensionAggregationType = ReverseMap(dashboardSchemaToProtoSpansAggregationDimensionAggregationType)
	dashboardValidSpansAggregationDimensionAggregationTypes        = GetKeys(dashboardSchemaToProtoSpansAggregationDimensionAggregationType)
	dashboardSchemaToProtoSpanFieldMetadataField                   = map[string]dashboards.SpanField_MetadataField{
		"unspecified":      dashboards.SpanField_METADATA_FIELD_UNSPECIFIED,
		"application_name": dashboards.SpanField_METADATA_FIELD_APPLICATION_NAME,
		"subsystem_name":   dashboards.SpanField_METADATA_FIELD_SUBSYSTEM_NAME,
		"service_name":     dashboards.SpanField_METADATA_FIELD_SERVICE_NAME,
		"operation_name":   dashboards.SpanField_METADATA_FIELD_OPERATION_NAME,
	}
	dashboardProtoToSchemaLegendColumn = map[string]dashboards.Legend_LegendColumn{
		"unspecified": dashboards.Legend_LEGEND_COLUMN_UNSPECIFIED,
		"min":         dashboards.Legend_LEGEND_COLUMN_MIN,
		"max":         dashboards.Legend_LEGEND_COLUMN_MAX,
		"sum":         dashboards.Legend_LEGEND_COLUMN_SUM,
		"avg":         dashboards.Legend_LEGEND_COLUMN_AVG,
		"last":        dashboards.Legend_LEGEND_COLUMN_LAST,
		"name":        dashboards.Legend_LEGEND_COLUMN_NAME,
	}
	dashboardSchemaToProtoLegendColumn = ReverseMap(dashboardProtoToSchemaLegendColumn)
	dashboardValidLegendColumn         = GetKeys(dashboardSchemaToProtoLegendColumn)
)

var (
	_ resource.ResourceWithConfigure        = &DashboardResource{}
	_ resource.ResourceWithConfigValidators = &DashboardResource{}
	_ resource.ResourceWithImportState      = &DashboardResource{}
)

func NewDashboardResource() resource.Resource {
	return &DashboardResource{}
}

type DashboardResource struct {
	client *clientset.DashboardsClient
}

func (r DashboardResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r DashboardResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	//TODO implement me
	panic("implement me")
}

func (r DashboardResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dashboard"
}

func (r DashboardResource) Schema(_ context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version: 1,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"name": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Dashboard name.",
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Dashboard description.",
			},
			"layout": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"sections": schema.ListNestedAttribute{
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"id": schema.StringAttribute{
									Computed: true,
								},
								"rows": schema.ListNestedAttribute{
									NestedObject: schema.NestedAttributeObject{
										Attributes: map[string]schema.Attribute{
											"id": schema.StringAttribute{
												Computed: true,
											},
											"height": schema.Int64Attribute{},
											"widgets": schema.ListNestedAttribute{
												NestedObject: schema.NestedAttributeObject{
													Attributes: map[string]schema.Attribute{
														"id": schema.StringAttribute{
															Computed: true,
														},
														"title": schema.StringAttribute{
															Optional: true,
														},
														"description": schema.StringAttribute{
															Optional: true,
														},
														"definition": schema.SingleNestedAttribute{
															Optional: true,
															Attributes: map[string]schema.Attribute{
																"line_chart": schema.SingleNestedAttribute{
																	Attributes: map[string]schema.Attribute{
																		"legend": schema.SingleNestedAttribute{
																			Attributes: map[string]schema.Attribute{
																				"is_visible": schema.BoolAttribute{
																					Optional: true,
																				},
																				"columns": schema.ListAttribute{
																					ElementType: types.StringType,
																				},
																				"group_by_query": schema.BoolAttribute{},
																			},
																		},
																		"tooltip": schema.SingleNestedAttribute{
																			Attributes: map[string]schema.Attribute{
																				"show_labels": schema.BoolAttribute{},
																				"type":        schema.StringAttribute{},
																			},
																		},
																		"query_definitions": schema.ListNestedAttribute{
																			NestedObject: schema.NestedAttributeObject{
																				Attributes: map[string]schema.Attribute{
																					"id": schema.StringAttribute{
																						Computed: true,
																					},
																					"query":                schema.StringAttribute{},
																					"series_name_template": schema.StringAttribute{},
																					"series_count_limit":   schema.Int64Attribute{},
																					"unit":                 schema.StringAttribute{},
																					"scale_type":           schema.StringAttribute{},
																					"name":                 schema.StringAttribute{},
																					"is_visible":           schema.BoolAttribute{},
																				},
																			},
																		},
																	},
																},
																"data_table": schema.SingleNestedAttribute{
																	Attributes: map[string]schema.Attribute{
																		"query": schema.SingleNestedAttribute{
																			Attributes: map[string]schema.Attribute{
																				"logs": schema.SingleNestedAttribute{
																					Attributes: map[string]schema.Attribute{
																						"lucene_query": schema.StringAttribute{},
																						"filters": schema.ListNestedAttribute{
																							NestedObject: schema.NestedAttributeObject{
																								Attributes: map[string]schema.Attribute{
																									"field":    schema.StringAttribute{},
																									"operator": schema.SingleNestedAttribute{},
																								},
																							},
																						},
																						"grouping": schema.SingleNestedAttribute{
																							Attributes: map[string]schema.Attribute{
																								"group_by": schema.StringAttribute{},
																								"aggregations": schema.ListNestedAttribute{
																									NestedObject: schema.NestedAttributeObject{
																										Attributes: map[string]schema.Attribute{
																											"id":         schema.StringAttribute{},
																											"name":       schema.StringAttribute{},
																											"is_visible": schema.BoolAttribute{},
																											"aggregation": schema.SingleNestedAttribute{
																												Attributes: map[string]schema.Attribute{
																													"type": schema.StringAttribute{
																														Required: true,
																														Validators: []validator.String{
																															stringvalidator.OneOf("count", "count_distinct", "sum", "average", "min", "max"),
																														},
																													},
																													"field": schema.StringAttribute{
																														Optional: true,
																													},
																												},
																											},
																										},
																									},
																								},
																							},
																						},
																					},
																				},
																				"spans":   schema.SingleNestedAttribute{},
																				"metrics": schema.SingleNestedAttribute{},
																			},
																		},
																		"results_per_page": schema.Int64Attribute{},
																		"row_style":        schema.StringAttribute{},
																		"columns": schema.ListNestedAttribute{
																			NestedObject: schema.NestedAttributeObject{
																				Attributes: map[string]schema.Attribute{
																					"field": schema.StringAttribute{},
																					"width": schema.Int64Attribute{},
																				},
																			},
																		},
																		"order_by": schema.SingleNestedAttribute{
																			Attributes: map[string]schema.Attribute{
																				"field": schema.StringAttribute{},
																				"order_direction": schema.StringAttribute{
																					Validators: []validator.String{
																						stringvalidator.OneOf("asc", "desc"),
																					},
																				},
																			},
																		},
																	},
																},
																"gauge": schema.SingleNestedAttribute{
																	Attributes: map[string]schema.Attribute{
																		"query": schema.SingleNestedAttribute{
																			Attributes: map[string]schema.Attribute{
																				"metrics": schema.SingleNestedAttribute{
																					Attributes: map[string]schema.Attribute{
																						"promql_query": schema.StringAttribute{},
																						"aggregation": schema.StringAttribute{
																							Validators: []validator.String{
																								stringvalidator.OneOf("sum", "avg", "min", "max", "last"),
																							},
																						},
																						"filters": schema.ListNestedAttribute{
																							NestedObject: schema.NestedAttributeObject{
																								Attributes: map[string]schema.Attribute{
																									"metric": schema.StringAttribute{},
																									"label":  schema.StringAttribute{},
																									"operator": schema.SingleNestedAttribute{
																										Attributes: map[string]schema.Attribute{
																											"type": schema.StringAttribute{
																												Required: true,
																												Validators: []validator.String{
																													stringvalidator.OneOf("equals", "not_equals"),
																												},
																											},
																											"values": schema.ListAttribute{
																												ElementType: types.StringType,
																											},
																										},
																									},
																								},
																							},
																						},
																						"logs": schema.SingleNestedAttribute{
																							Attributes: map[string]schema.Attribute{
																								"lucene_query": schema.StringAttribute{},
																								"logs_aggregation": schema.SingleNestedAttribute{
																									Attributes: map[string]schema.Attribute{
																										"type": schema.StringAttribute{
																											Required: true,
																											Validators: []validator.String{
																												stringvalidator.OneOf("count", "count_distinct", "sum", "average", "min", "max"),
																											},
																										},
																										"field": schema.StringAttribute{
																											Optional: true,
																										},
																									},
																								},
																								"aggregation": schema.StringAttribute{
																									Validators: []validator.String{
																										stringvalidator.OneOf("sum", "avg", "min", "max", "last"),
																									},
																								},
																								"filters": schema.ListNestedAttribute{
																									NestedObject: schema.NestedAttributeObject{
																										Attributes: map[string]schema.Attribute{
																											"field": schema.SingleNestedAttribute{
																												Attributes: map[string]schema.Attribute{
																													"type": schema.StringAttribute{
																														Required: true,
																														Validators: []validator.String{
																															stringvalidator.OneOf("metadata", "tag", "process_tag"),
																														},
																													},
																													"field": schema.StringAttribute{
																														Required: true,
																													},
																												},
																											},
																											"operator": schema.SingleNestedAttribute{
																												Attributes: map[string]schema.Attribute{
																													"type": schema.StringAttribute{
																														Required: true,
																														Validators: []validator.String{
																															stringvalidator.OneOf("equals", "not_equals"),
																														},
																													},
																													"values": schema.ListAttribute{
																														ElementType: types.StringType,
																													},
																												},
																											},
																										},
																									},
																								},
																							},
																						},
																					},
																				},
																				"spans": schema.SingleNestedAttribute{},
																			},
																		},
																	},
																},
																"pie_chart": schema.SingleNestedAttribute{
																	Attributes: map[string]schema.Attribute{
																		"query": schema.SingleNestedAttribute{
																			Attributes: map[string]schema.Attribute{
																				"logs": schema.SingleNestedAttribute{
																					Attributes: map[string]schema.Attribute{
																						"lucene_query": schema.StringAttribute{},
																						"aggregation": schema.SingleNestedAttribute{
																							Attributes: map[string]schema.Attribute{
																								"type": schema.StringAttribute{
																									Required: true,
																									Validators: []validator.String{
																										stringvalidator.OneOf("count", "count_distinct", "sum", "average", "min", "max"),
																									},
																								},
																								"field": schema.StringAttribute{
																									Optional: true,
																								},
																							},
																						},
																						"filters": schema.ListNestedAttribute{
																							NestedObject: schema.NestedAttributeObject{
																								Attributes: map[string]schema.Attribute{
																									"field":    schema.StringAttribute{},
																									"operator": schema.SingleNestedAttribute{},
																								},
																							},
																						},
																						"group_names": schema.ListAttribute{
																							ElementType: types.StringType,
																						},
																						"stacked_group_name": schema.StringAttribute{},
																					},
																				},
																				"spans": schema.SingleNestedAttribute{
																					Attributes: map[string]schema.Attribute{
																						"lucene_query": schema.StringAttribute{},
																						"aggregation": schema.SingleNestedAttribute{
																							Attributes: map[string]schema.Attribute{
																								"type": schema.StringAttribute{
																									Required: true,
																									Validators: []validator.String{
																										stringvalidator.OneOf("metric", "dimension"),
																									},
																								},
																								"metric_field":     schema.StringAttribute{},
																								"aggregation_type": schema.StringAttribute{},
																							},
																						},
																						"filters": schema.ListNestedAttribute{
																							NestedObject: schema.NestedAttributeObject{
																								Attributes: map[string]schema.Attribute{
																									"field": schema.SingleNestedAttribute{
																										Attributes: map[string]schema.Attribute{
																											"type": schema.StringAttribute{
																												Required: true,
																												Validators: []validator.String{
																													stringvalidator.OneOf("metadata", "tag", "process_tag"),
																												},
																											},
																											"field": schema.StringAttribute{
																												Required: true,
																											},
																										},
																									},
																									"operator": schema.SingleNestedAttribute{
																										Attributes: map[string]schema.Attribute{
																											"type": schema.StringAttribute{
																												Required: true,
																												Validators: []validator.String{
																													stringvalidator.OneOf("equals", "not_equals"),
																												},
																											},
																											"values": schema.ListAttribute{
																												ElementType: types.StringType,
																											},
																										},
																									},
																								},
																							},
																						},
																						"group_names": schema.ListNestedAttribute{
																							NestedObject: schema.NestedAttributeObject{
																								Attributes: map[string]schema.Attribute{
																									"type": schema.StringAttribute{
																										Required: true,
																										Validators: []validator.String{
																											stringvalidator.OneOf("metadata", "tag", "process_tag"),
																										},
																									},
																									"field": schema.StringAttribute{
																										Required: true,
																									},
																								},
																							},
																						},
																						"stacked_group_name": schema.SingleNestedAttribute{
																							Attributes: map[string]schema.Attribute{
																								"type": schema.StringAttribute{
																									Required: true,
																									Validators: []validator.String{
																										stringvalidator.OneOf("metadata", "tag", "process_tag"),
																									},
																								},
																								"field": schema.StringAttribute{
																									Required: true,
																								},
																							},
																						},
																					},
																				},
																				"metrics": schema.SingleNestedAttribute{
																					Attributes: map[string]schema.Attribute{
																						"promql_query": schema.StringAttribute{},
																						"filters": schema.ListNestedAttribute{
																							NestedObject: schema.NestedAttributeObject{
																								Attributes: map[string]schema.Attribute{
																									"metric": schema.StringAttribute{},
																									"label":  schema.StringAttribute{},
																									"operator": schema.SingleNestedAttribute{
																										Attributes: map[string]schema.Attribute{
																											"type": schema.StringAttribute{
																												Required: true,
																												Validators: []validator.String{
																													stringvalidator.OneOf("equals", "not_equals"),
																												},
																											},
																											"values": schema.ListAttribute{
																												ElementType: types.StringType,
																											},
																										},
																									},
																								},
																							},
																						},
																						"group_names": schema.ListNestedAttribute{
																							NestedObject: schema.NestedAttributeObject{
																								Attributes: map[string]schema.Attribute{
																									"type": schema.StringAttribute{
																										Required: true,
																										Validators: []validator.String{
																											stringvalidator.OneOf("metadata", "tag", "process_tag"),
																										},
																									},
																									"field": schema.StringAttribute{
																										Required: true,
																									},
																								},
																							},
																						},
																						"stacked_group_name": schema.SingleNestedAttribute{
																							Attributes: map[string]schema.Attribute{
																								"type": schema.StringAttribute{
																									Required: true,
																									Validators: []validator.String{
																										stringvalidator.OneOf("metadata", "tag", "process_tag"),
																									},
																								},
																								"field": schema.StringAttribute{
																									Required: true,
																								},
																							},
																						},
																					},
																				},
																			},
																		},
																		"max_slices_per_chart": schema.Int64Attribute{},
																		"min_slices_per_chart": schema.Int64Attribute{},
																		"stack_definition": schema.SingleNestedAttribute{
																			Attributes: map[string]schema.Attribute{
																				"max_slices_per_stack": schema.Int64Attribute{},
																				"stack_name_template":  schema.StringAttribute{},
																			},
																		},
																		"label_definition": schema.SingleNestedAttribute{
																			Attributes: map[string]schema.Attribute{
																				"label_source":    schema.StringAttribute{},
																				"is_visible":      schema.BoolAttribute{},
																				"show_name":       schema.BoolAttribute{},
																				"show_value":      schema.BoolAttribute{},
																				"show_percentage": schema.BoolAttribute{},
																			},
																		},
																		"show_legend":         schema.BoolAttribute{},
																		"group_name_template": schema.StringAttribute{},
																		"unit":                schema.StringAttribute{},
																	},
																},
																"bar_chart": schema.SingleNestedAttribute{
																	Attributes: map[string]schema.Attribute{
																		"query": schema.SingleNestedAttribute{
																			Attributes: map[string]schema.Attribute{
																				"logs": schema.SingleNestedAttribute{
																					Attributes: map[string]schema.Attribute{
																						"lucene_query": schema.StringAttribute{},
																						"aggregation": schema.SingleNestedAttribute{
																							Attributes: map[string]schema.Attribute{
																								"type": schema.StringAttribute{
																									Required: true,
																									Validators: []validator.String{
																										stringvalidator.OneOf("count", "count_distinct", "sum", "average", "min", "max"),
																									},
																								},
																								"field": schema.StringAttribute{
																									Optional: true,
																								},
																							},
																						},
																						"filters": schema.ListNestedAttribute{
																							NestedObject: schema.NestedAttributeObject{
																								Attributes: map[string]schema.Attribute{
																									"field": schema.StringAttribute{
																										Required: true,
																									},
																									"operator": schema.SingleNestedAttribute{
																										Attributes: map[string]schema.Attribute{
																											"type": schema.StringAttribute{},
																										},
																									},
																								},
																							},
																						},
																					},
																				},
																			},
																		},
																		"max_slices_per_chart": schema.Int64Attribute{},
																		"group_name_template":  schema.StringAttribute{},
																		"stack_definition": schema.SingleNestedAttribute{
																			Attributes: map[string]schema.Attribute{
																				"max_slices_per_stack": schema.Int64Attribute{},
																				"stack_name_template":  schema.StringAttribute{},
																			},
																		},
																		"scale_type": schema.StringAttribute{},
																		"colors_by":  schema.StringAttribute{},
																		"x_axis": schema.SingleNestedAttribute{
																			Attributes: map[string]schema.Attribute{
																				"type": schema.StringAttribute{
																					Required: true,
																					Validators: []validator.String{
																						stringvalidator.OneOf("value", "time"),
																					},
																				},
																				"interval":          schema.StringAttribute{},
																				"buckets_presented": schema.Int64Attribute{},
																			},
																		},
																		"unit": schema.StringAttribute{},
																	},
																},
															},
														},
														"width": schema.Int64Attribute{},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			"variables": schema.ListNestedAttribute{
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{},
						"definition": schema.SingleNestedAttribute{
							Attributes: map[string]schema.Attribute{
								"constant": schema.SingleNestedAttribute{
									Attributes: map[string]schema.Attribute{
										"value": schema.StringAttribute{},
									},
								},
								"multi_select": schema.SingleNestedAttribute{
									Attributes: map[string]schema.Attribute{
										"selected": schema.ListAttribute{
											ElementType: types.StringType,
										},
										"source": schema.SingleNestedAttribute{
											Attributes: map[string]schema.Attribute{
												"logs_path": schema.StringAttribute{},
												"metric_label": schema.SingleNestedAttribute{
													Attributes: map[string]schema.Attribute{
														"metric_name": schema.StringAttribute{},
														"label":       schema.StringAttribute{},
													},
												},
												"constant_list": schema.ListAttribute{
													ElementType: types.StringType,
												},
												"span_field": schema.SingleNestedAttribute{
													Attributes: map[string]schema.Attribute{
														"type":  schema.StringAttribute{},
														"field": schema.StringAttribute{},
													},
												},
											},
										},
									},
								},
							},
						},
						"display_name": schema.StringAttribute{},
					},
				},
			},
			"filters": schema.ListNestedAttribute{
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"source": schema.SingleNestedAttribute{
							Attributes: map[string]schema.Attribute{
								"logs": schema.SingleNestedAttribute{
									Attributes: map[string]schema.Attribute{
										"field": schema.StringAttribute{},
										"operator": schema.SingleNestedAttribute{
											Attributes: map[string]schema.Attribute{
												"type": schema.StringAttribute{
													Required: true,
													Validators: []validator.String{
														stringvalidator.OneOf("equals", "not_equals"),
													},
												},
												"values": schema.ListAttribute{
													Optional:    true,
													ElementType: types.StringType,
												},
											},
										},
									},
								},
								"spans": schema.SingleNestedAttribute{
									Attributes: map[string]schema.Attribute{
										"field_type": schema.StringAttribute{
											Required: true,
											Validators: []validator.String{
												stringvalidator.OneOf("metadata", "tag", "process_tag"),
											},
										},
										"field_value": schema.StringAttribute{},
										"operator": schema.SingleNestedAttribute{
											Attributes: map[string]schema.Attribute{
												"type": schema.StringAttribute{
													Required: true,
													Validators: []validator.String{
														stringvalidator.OneOf("equals", "not_equals"),
													},
												},
												"values": schema.ListAttribute{
													Optional: true,
												},
											},
										},
									},
								},
								"metrics": schema.SingleNestedAttribute{
									Attributes: map[string]schema.Attribute{
										"metric": schema.StringAttribute{},
										"label":  schema.StringAttribute{},
										"operator": schema.SingleNestedAttribute{
											Attributes: map[string]schema.Attribute{
												"type": schema.StringAttribute{
													Required: true,
													Validators: []validator.String{
														stringvalidator.OneOf("equals", "not_equals"),
													},
												},
												"values": schema.ListAttribute{
													Optional: true,
												},
											},
										},
									},
								},
							},
						},
						"enabled":   schema.BoolAttribute{},
						"collapsed": schema.BoolAttribute{},
					},
				},
			},
			"time_frame": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"type":     schema.StringAttribute{},
					"start":    schema.StringAttribute{},
					"end":      schema.StringAttribute{},
					"duration": schema.StringAttribute{},
				},
			},
		},
	}
}

func (r DashboardResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	jsm := &jsonpb.Marshaler{}
	var plan DashboardResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	dashboard, diags := extractDashboard(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	dashboardStr, _ := jsm.MarshalToString(dashboard)
	log.Printf("[INFO] Creating new Dashboard: %#v", dashboardStr)
	createDashboardReq := &dashboards.CreateDashboardRequest{
		Dashboard: dashboard,
	}
	createDashboardResp, err := r.client.CreateDashboard(ctx, createDashboardReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		resp.Diagnostics.AddError(
			"Error creating Dashboard",
			"Could not create Dashboard, unexpected error: "+err.Error(),
		)
		return
	}
	createDashboardRespStr, _ := jsm.MarshalToString(createDashboardResp)
	log.Printf("[INFO] Submitted new Dashboard: %#v", createDashboardRespStr)

	getDashboardReq := &dashboards.GetDashboardRequest{
		DashboardId: dashboard.GetId(),
	}
	getDashboardResp, err := r.client.GetDashboard(ctx, getDashboardReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		resp.Diagnostics.AddError(
			"Error getting Dashboard",
			"Could not create Dashboard, unexpected error: "+err.Error(),
		)
		return
	}

	plan = flattenDashboard(ctx, getDashboardResp.GetDashboard())

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func extractDashboard(ctx context.Context, plan DashboardResourceModel) (*dashboards.Dashboard, diag.Diagnostics) {
	layout, diags := expandDashboardLayout(ctx, plan.Layout)
	if diags.HasError() {
		return nil, diags
	}

	variables, diags := expandDashboardVariables(ctx, plan.Variables.Elements())
	if diags.HasError() {
		return nil, diags
	}

	filters, diags := expandDashboardFilters(ctx, plan.Filters.Elements())
	if diags.HasError() {
		return nil, diags
	}

	dashboard := &dashboards.Dashboard{
		Name:        typeStringToWrapperspbString(plan.Name),
		Description: typeStringToWrapperspbString(plan.Description),
		Layout:      layout,
		Variables:   variables,
		Filters:     filters,
	}

	dashboard, dg := expandDashboardTimeFrame(dashboard, plan.TimeFrame)
	if diags.HasError() {
		return nil, diag.Diagnostics{dg}
	}

	return dashboard, nil
}

func expandDashboardTimeFrame(dashboard *dashboards.Dashboard, timeFrame *DashboardTimeFrameModel) (*dashboards.Dashboard, diag.Diagnostic) {
	if timeFrame == nil {
		return dashboard, nil
	}
	var dg diag.Diagnostic
	switch {
	case timeFrame.Relative != nil:
		dashboard.TimeFrame, dg = expandRelativeDashboardTimeFrame(timeFrame.Relative)
	case timeFrame.Absolute != nil:
		dashboard.TimeFrame, dg = expandAbsoluteeDashboardTimeFrame(timeFrame.Absolute)
	default:
		dg = diag.NewErrorDiagnostic("Error Expand Time Frame", "Dashboard TimeFrame must be either Relative or Absolutee")
	}
	return dashboard, dg
}

func expandDashboardLayout(ctx context.Context, layout *DashboardLayoutModel) (*dashboards.Layout, diag.Diagnostics) {
	sections, diags := expandDashboardSections(ctx, layout.Sections.Elements())
	if diags.HasError() {
		return nil, diags
	}
	return &dashboards.Layout{
		Sections: sections,
	}, nil
}

func expandDashboardSections(ctx context.Context, sections []attr.Value) ([]*dashboards.Section, diag.Diagnostics) {
	var diags diag.Diagnostics
	expandedSections := make([]*dashboards.Section, len(sections))
	for _, s := range sections {
		v, err := s.ToTerraformValue(ctx)
		if err != nil {
			diags.AddError("Extract Dashboard Section Error", err.Error())
			continue
		}
		var section SectionModel
		if err = v.As(&section); err != nil {
			diags.AddError("Extract Dashboard Section Error", err.Error())
			continue
		}

		expandedSection, expandSectionDiags := expandSection(ctx, section)
		if expandSectionDiags.HasError() {
			diags.Append(expandSectionDiags...)
			continue
		}
		expandedSections = append(expandedSections, expandedSection)
	}

	return expandedSections, diags
}

func expandSection(ctx context.Context, section SectionModel) (*dashboards.Section, diag.Diagnostics) {
	id := expandDashboardUUID(section.ID)
	rows, diags := expandDashboardRows(ctx, section.Rows.Elements())
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.Section{
		Id:   id,
		Rows: rows,
	}, nil
}

func expandDashboardRows(ctx context.Context, elements []attr.Value) ([]*dashboards.Row, diag.Diagnostics) {
	var diags diag.Diagnostics
	expandedRows := make([]*dashboards.Row, len(elements))
	for _, e := range elements {
		v, err := e.ToTerraformValue(ctx)
		if err != nil {
			diags.AddError("Extract Dashboard Row Error", err.Error())
			continue
		}
		var row RowModel
		if err = v.As(&row); err != nil {
			diags.AddError("Extract Dashboard Row Error", err.Error())
			continue
		}

		expandedRow, expandRowDiags := expandRow(ctx, row)
		if expandRowDiags.HasError() {
			diags.Append(expandRowDiags...)
			continue
		}

		expandedRows = append(expandedRows, expandedRow)
	}

	return expandedRows, diags
}

func expandRow(ctx context.Context, row RowModel) (*dashboards.Row, diag.Diagnostics) {
	id := expandDashboardUUID(row.ID)
	appearance := &dashboards.Row_Appearance{
		Height: wrapperspb.Int32(int32(row.Height.ValueInt64())),
	}
	widgets, diags := expandDashboardWidgets(ctx, row.Widgets.Elements())
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.Row{
		Id:         id,
		Appearance: appearance,
		Widgets:    widgets,
	}, nil
}

func expandDashboardWidgets(ctx context.Context, widgets []attr.Value) ([]*dashboards.Widget, diag.Diagnostics) {
	var diags diag.Diagnostics

	expandedWidgets := make([]*dashboards.Widget, len(widgets))
	for _, w := range widgets {
		v, err := w.ToTerraformValue(ctx)
		if err != nil {
			diags.AddError("Extract Dashboard Widget Error", err.Error())
			continue
		}
		var widget WidgetModel
		if err = v.As(&widget); err != nil {
			diags.AddError("Extract Dashboard Widget Error", err.Error())
			continue
		}

		expandedWidget, expandWidgetDiags := expandWidget(ctx, widget)
		if expandWidgetDiags.HasError() {
			diags.Append(expandWidgetDiags...)
			continue
		}

		expandedWidgets = append(expandedWidgets, expandedWidget)
	}

	return expandedWidgets, diags
}

func expandWidget(ctx context.Context, widget WidgetModel) (*dashboards.Widget, diag.Diagnostics) {
	id := expandDashboardUUID(widget.ID)
	title := typeStringToWrapperspbString(widget.Title)
	description := typeStringToWrapperspbString(widget.Description)
	appearance := &dashboards.Widget_Appearance{
		Width: wrapperspb.Int32(int32(widget.Width.ValueInt64())),
	}
	definition, diags := expandWidgetDefinition(ctx, widget.Definition)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.Widget{
		Id:          id,
		Title:       title,
		Description: description,
		Appearance:  appearance,
		Definition:  definition,
	}, nil
}

func expandWidgetDefinition(ctx context.Context, definition *WidgetDefinitionModel) (*dashboards.Widget_Definition, diag.Diagnostics) {
	switch {
	case definition.PieChart != nil:
		pieChart, diags := expandPieChart(ctx, definition.PieChart)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.Widget_Definition{
			Value: pieChart,
		}, nil
	case definition.Gauge != nil:
		gauge, diags := expandGauge(ctx, definition.Gauge)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.Widget_Definition{
			Value: gauge,
		}, nil
	case definition.LineChart != nil:
		lineChart, diags := expandLineChart(ctx, definition.LineChart)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.Widget_Definition{
			Value: lineChart,
		}, nil
	case definition.DataTable != nil:
		dataTable, diags := expandDataTable(ctx, definition.DataTable)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.Widget_Definition{
			Value: dataTable,
		}, nil
	case definition.BarChart != nil:
		barChart, diags := expandBarChart(ctx, definition.BarChart)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.Widget_Definition{
			Value: barChart,
		}, nil
	default:
		return nil, diag.Diagnostics{
			diag.NewErrorDiagnostic(
				"Extract Dashboard Widget Definition Error",
				fmt.Sprintf("Unknown widget definition type: %#v", definition),
			),
		}
	}
}

func expandPieChart(ctx context.Context, pieChart *PieChartModel) (*dashboards.Widget_Definition_PieChart, diag.Diagnostics) {
	var diags diag.Diagnostics

	query, diags := expandDashboardQuery(ctx, pieChart.Query)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.Widget_Definition_PieChart{
		PieChart: &dashboards.PieChart{
			Query:              query,
			MaxSlicesPerChart:  typeInt64ToWrappedInt32(pieChart.MaxSlicesPerChart),
			MinSlicePercentage: typeInt64ToWrappedInt32(pieChart.MinSlicePercentage),
			StackDefinition:    expandPieChartStackDefinition(pieChart.StackDefinition),
			LabelDefinition:    expandLabelDefinition(pieChart.LabelDefinition),
		},
	}, nil
}

func expandPieChartStackDefinition(stackDefinition *PieChartStackDefinitionModel) *dashboards.PieChart_StackDefinition {
	if stackDefinition == nil {
		return nil
	}

	return &dashboards.PieChart_StackDefinition{
		MaxSlicesPerStack: typeInt64ToWrappedInt32(stackDefinition.MaxSlicesPerStack),
		StackNameTemplate: typeStringToWrapperspbString(stackDefinition.StackNameTemplate),
	}
}

func expandBarChartStackDefinition(stackDefinition *BarChartStackDefinitionModel) *dashboards.BarChart_StackDefinition {
	if stackDefinition == nil {
		return nil
	}

	return &dashboards.BarChart_StackDefinition{
		MaxSlicesPerBar:   typeInt64ToWrappedInt32(stackDefinition.MaxSlicesPerBar),
		StackNameTemplate: typeStringToWrapperspbString(stackDefinition.StackNameTemplate),
	}
}

func expandLabelDefinition(labelDefinition *LabelDefinitionModel) *dashboards.PieChart_LabelDefinition {
	if labelDefinition == nil {
		return nil
	}

	return &dashboards.PieChart_LabelDefinition{
		LabelSource:    dashboardSchemaToProtoPieChartLabelSource[labelDefinition.LabelSource.ValueString()],
		IsVisible:      typeBoolToWrapperspbBool(labelDefinition.IsVisible),
		ShowName:       typeBoolToWrapperspbBool(labelDefinition.ShowName),
		ShowValue:      typeBoolToWrapperspbBool(labelDefinition.ShowValue),
		ShowPercentage: typeBoolToWrapperspbBool(labelDefinition.ShowPercentage),
	}
}

func expandGauge(ctx context.Context, gauge *GaugeModel) (*dashboards.Widget_Definition_Gauge, diag.Diagnostics) {
	var diags diag.Diagnostics

	query, diags := expandGaugeQuery(ctx, gauge.Query)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.Widget_Definition_Gauge{
		Gauge: &dashboards.Gauge{
			Query: query,
		},
	}, nil

}

func expandGaugeQuery(ctx context.Context, gaugeQuery *GaugeQueryModel) (*dashboards.Gauge_Query, diag.Diagnostics) {
	switch {
	case gaugeQuery.Metrics != nil:
		metricQuery, diags := expandGaugeQueryMetrics(ctx, gaugeQuery.Metrics)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.Gauge_Query{
			Value: &dashboards.Gauge_Query_Metrics{
				Metrics: metricQuery,
			},
		}, nil
	case gaugeQuery.Logs != nil:
		logQuery, diags := expandGaugeQueryLogs(ctx, gaugeQuery.Logs)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.Gauge_Query{
			Value: &dashboards.Gauge_Query_Logs{
				Logs: logQuery,
			},
		}, nil
	case gaugeQuery.Spans != nil:
		spanQuery, diags := expandGaugeQuerySpans(ctx, gaugeQuery.Spans)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.Gauge_Query{
			Value: &dashboards.Gauge_Query_Spans{
				Spans: spanQuery,
			},
		}, nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Extract Gauge Query Error", fmt.Sprintf("Unknown gauge query type %#v", gaugeQuery))}
	}
}

func expandGaugeQuerySpans(ctx context.Context, gaugeQuerySpans *GaugeQuerySpansModel) (*dashboards.Gauge_SpansQuery, diag.Diagnostics) {
	if gaugeQuerySpans == nil {
		return nil, nil
	}
	filters, diags := expandSpansFilters(ctx, gaugeQuerySpans.Filters.Elements())
	if diags.HasError() {
		return nil, diags
	}

	spansAggregation, dg := expandSpansAggregation(gaugeQuerySpans.SpansAggregation)
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	return &dashboards.Gauge_SpansQuery{
		LuceneQuery:      expandLuceneQuery(gaugeQuerySpans.LuceneQuery),
		SpansAggregation: spansAggregation,
		Filters:          filters,
		Aggregation:      dashboardSchemaToProtoGaugeAggregation[gaugeQuerySpans.Aggregation.ValueString()],
	}, nil
}

func expandSpansAggregations(ctx context.Context, spansAggregations []attr.Value) ([]*dashboards.SpansAggregation, diag.Diagnostics) {
	var diags diag.Diagnostics
	expandedSpansAggregations := make([]*dashboards.SpansAggregation, 0, len(spansAggregations))
	for _, sa := range spansAggregations {
		v, err := sa.ToTerraformValue(ctx)
		if err != nil {
			diags.AddError("Extract Spans Aggregations Error", err.Error())
			continue
		}
		var spansAggregation SpansAggregationModel
		if err = v.As(&spansAggregation); err != nil {
			diags.AddError("Extract Dashboard Spans Aggregations Error", err.Error())
			continue
		}
		expandedSpansAggregation, expandDiag := expandSpansAggregation(&spansAggregation)
		if expandDiag != nil {
			diags.Append(expandDiag)
			continue
		}
		expandedSpansAggregations = append(expandedSpansAggregations, expandedSpansAggregation)
	}
	return expandedSpansAggregations, diags
}

func expandSpansAggregation(spansAggregation *SpansAggregationModel) (*dashboards.SpansAggregation, diag.Diagnostic) {
	if spansAggregation == nil {
		return nil, nil
	}

	switch spansAggregation.Type.ValueString() {
	case "metric":
		return &dashboards.SpansAggregation{
			Aggregation: &dashboards.SpansAggregation_MetricAggregation_{
				MetricAggregation: &dashboards.SpansAggregation_MetricAggregation{
					MetricField:     dashboardSchemaToProtoSpansAggregationMetricField[spansAggregation.Field.ValueString()],
					AggregationType: dashboardSchemaToProtoSpansAggregationMetricAggregationType[spansAggregation.AggregationType.ValueString()],
				},
			},
		}, nil
	case "dimension":
		return &dashboards.SpansAggregation{
			Aggregation: &dashboards.SpansAggregation_DimensionAggregation_{
				DimensionAggregation: &dashboards.SpansAggregation_DimensionAggregation{
					DimensionField:  dashboardProtoToSchemaSpansAggregationDimensionField[spansAggregation.Field.ValueString()],
					AggregationType: dashboardSchemaToProtoSpansAggregationDimensionAggregationType[spansAggregation.AggregationType.ValueString()],
				},
			},
		}, nil
	default:
		return nil, diag.NewErrorDiagnostic("Extract Spans Aggregation Error", fmt.Sprintf("Unknown spans aggregation type %#v", spansAggregation))
	}
}

func expandSpansFilters(ctx context.Context, spansFilters []attr.Value) ([]*dashboards.Filter_SpansFilter, diag.Diagnostics) {
	var diags diag.Diagnostics
	expandedFilters := make([]*dashboards.Filter_SpansFilter, len(spansFilters))
	for _, w := range spansFilters {
		v, err := w.ToTerraformValue(ctx)
		if err != nil {
			diags.AddError("Extract Dashboard Spans Filter Error", err.Error())
			continue
		}
		var filter SpansFilterModel
		if err = v.As(&filter); err != nil {
			diags.AddError("Extract Dashboard Spans Filter Error", err.Error())
			continue
		}
		expandedFilter, expandFilterDiags := expandSpansFilter(ctx, filter)
		if expandFilterDiags.HasError() {
			diags.Append(expandFilterDiags...)
			continue
		}
		expandedFilters = append(expandedFilters, expandedFilter)
	}

	return expandedFilters, diags
}

func expandSpansFilter(ctx context.Context, spansFilter SpansFilterModel) (*dashboards.Filter_SpansFilter, diag.Diagnostics) {
	operator, diags := expandFilterOperator(ctx, spansFilter.Operator)
	if diags.HasError() {
		return nil, diags
	}

	field, dg := expandSpansField(spansFilter.Field)
	if dg != nil {
		diags.Append(dg)
		return nil, diags
	}

	return &dashboards.Filter_SpansFilter{
		Field:    field,
		Operator: operator,
	}, nil
}

func expandSpansField(spansFilterField *SpansFieldModel) (*dashboards.SpanField, diag.Diagnostic) {
	if spansFilterField == nil {
		return nil, nil
	}

	switch spansFilterField.Type.ValueString() {
	case "metadata":
		return &dashboards.SpanField{
			Value: &dashboards.SpanField_MetadataField_{
				MetadataField: dashboardSchemaToProtoSpanFieldMetadataField[spansFilterField.Value.ValueString()],
			},
		}, nil
	case "tag":
		return &dashboards.SpanField{
			Value: &dashboards.SpanField_TagField{
				TagField: typeStringToWrapperspbString(spansFilterField.Value),
			},
		}, nil
	case "process_tag":
		return &dashboards.SpanField{
			Value: &dashboards.SpanField_ProcessTagField{
				ProcessTagField: typeStringToWrapperspbString(spansFilterField.Value),
			},
		}, nil
	default:
		return nil, diag.NewErrorDiagnostic("Extract Spans Filter Field Error", fmt.Sprintf("Unknown spans filter field type %s", spansFilterField.Type.ValueString()))
	}
}

func expandGaugeQueryMetrics(ctx context.Context, gaugeQueryMetrics *GaugeQueryMetricsModel) (*dashboards.Gauge_MetricsQuery, diag.Diagnostics) {
	filters, diags := expandMetricsFilters(ctx, gaugeQueryMetrics.Filters.Elements())
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.Gauge_MetricsQuery{
		PromqlQuery: expandPromqlQuery(gaugeQueryMetrics.PromqlQuery),
		Aggregation: dashboardSchemaToProtoGaugeAggregation[gaugeQueryMetrics.Aggregation.ValueString()],
		Filters:     filters,
	}, nil
}

func expandMetricsFilters(ctx context.Context, metricFilters []attr.Value) ([]*dashboards.Filter_MetricsFilter, diag.Diagnostics) {
	var diags diag.Diagnostics
	expandedFilters := make([]*dashboards.Filter_MetricsFilter, len(metricFilters))
	for _, w := range metricFilters {
		v, err := w.ToTerraformValue(ctx)
		if err != nil {
			diags.AddError("Extract Dashboard Metric Filter Error", err.Error())
			continue
		}
		var filter MetricsFilterModel
		if err = v.As(&filter); err != nil {
			diags.AddError("Extract Dashboard Metric Filter Error", err.Error())
			continue
		}

		expandedFilter, expandFilterDiags := expandMetricFilter(ctx, filter)
		if expandFilterDiags.HasError() {
			diags.Append(expandFilterDiags...)
			continue
		}

		expandedFilters = append(expandedFilters, expandedFilter)
	}

	return expandedFilters, diags
}

func expandMetricFilter(ctx context.Context, metricFilter MetricsFilterModel) (*dashboards.Filter_MetricsFilter, diag.Diagnostics) {
	operator, diags := expandFilterOperator(ctx, metricFilter.Operator)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.Filter_MetricsFilter{
		Metric:   typeStringToWrapperspbString(metricFilter.Metric),
		Label:    typeStringToWrapperspbString(metricFilter.Label),
		Operator: operator,
	}, nil
}

func expandFilterOperator(ctx context.Context, operator *FilterOperatorModel) (*dashboards.Filter_Operator, diag.Diagnostics) {
	if operator == nil {
		return nil, nil
	}

	selectedValues, diags := typeStringSliceToWrappedStringSlice(ctx, operator.SelectedValues.Elements())
	if diags.HasError() {
		return nil, diags
	}

	switch operator.Type.ValueString() {
	case "equals":
		filterOperator := &dashboards.Filter_Operator{
			Value: &dashboards.Filter_Operator_Equals{
				Equals: &dashboards.Filter_Equals{
					Selection: &dashboards.Filter_Equals_Selection{},
				},
			},
		}
		if len(selectedValues) != 0 {
			filterOperator.GetEquals().Selection.Value = &dashboards.Filter_Equals_Selection_List{
				List: &dashboards.Filter_Equals_Selection_ListSelection{
					Values: selectedValues,
				},
			}
		} else {
			filterOperator.GetEquals().Selection.Value = &dashboards.Filter_Equals_Selection_All{
				All: &dashboards.Filter_Equals_Selection_AllSelection{},
			}
		}
		return filterOperator, nil
	case "not_equals":
		return &dashboards.Filter_Operator{
			Value: &dashboards.Filter_Operator_NotEquals{
				NotEquals: &dashboards.Filter_NotEquals{
					Selection: &dashboards.Filter_NotEquals_Selection{
						Value: &dashboards.Filter_NotEquals_Selection_List{
							List: &dashboards.Filter_NotEquals_Selection_ListSelection{
								Values: selectedValues,
							},
						},
					},
				},
			},
		}, nil
	default:
		diags.Append(diag.NewErrorDiagnostic(
			"Error expand filter operator",
			fmt.Sprintf("unknown filter operator type %s", operator.Type.ValueString())))
		return nil, diags
	}
}

func expandPromqlQuery(promqlQuery types.String) *dashboards.PromQlQuery {
	if promqlQuery.IsNull() || promqlQuery.IsUnknown() {
		return nil
	}

	return &dashboards.PromQlQuery{
		Value: wrapperspb.String(promqlQuery.ValueString()),
	}
}

func expandGaugeQueryLogs(ctx context.Context, gaugeQueryLogs *GaugeQueryLogsModel) (*dashboards.Gauge_LogsQuery, diag.Diagnostics) {
	logsAggregation, dg := expandLogsAggregation(gaugeQueryLogs.LogsAggregation)
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	filters, diags := expandLogsFilters(ctx, gaugeQueryLogs.Filters.Elements())
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.Gauge_LogsQuery{
		LuceneQuery:     expandLuceneQuery(gaugeQueryLogs.LuceneQuery),
		LogsAggregation: logsAggregation,
		Filters:         filters,
		Aggregation:     dashboardSchemaToProtoGaugeAggregation[gaugeQueryLogs.Aggregation.ValueString()],
	}, nil
}

func expandLuceneQuery(luceneQuery types.String) *dashboards.LuceneQuery {
	if luceneQuery.IsNull() || luceneQuery.IsUnknown() {
		return nil
	}
	return &dashboards.LuceneQuery{
		Value: wrapperspb.String(luceneQuery.ValueString()),
	}
}

func expandLogsAggregations(ctx context.Context, logsAggregations []attr.Value) ([]*dashboards.LogsAggregation, diag.Diagnostics) {
	var diags diag.Diagnostics
	expandedLogsAggregations := make([]*dashboards.LogsAggregation, len(logsAggregations))
	for _, w := range logsAggregations {
		v, err := w.ToTerraformValue(ctx)
		if err != nil {
			diags.AddError("Extract Dashboard Logs Aggregation Error", err.Error())
			continue
		}
		var aggregation AggregationModel
		if err = v.As(&aggregation); err != nil {
			diags.AddError("Extract Dashboard Logs Aggregation Error", err.Error())
			continue
		}

		expandedLogsAggregation, expandDiags := expandLogsAggregation(&aggregation)
		if expandDiags != nil {
			diags.Append(expandDiags)
			continue
		}

		expandedLogsAggregations = append(expandedLogsAggregations, expandedLogsAggregation)
	}

	return expandedLogsAggregations, diags
}

func expandLogsAggregation(logsAggregation *AggregationModel) (*dashboards.LogsAggregation, diag.Diagnostic) {
	if logsAggregation == nil {
		return nil, nil
	}
	switch logsAggregation.Type.ValueString() {
	case "count":
		return &dashboards.LogsAggregation{
			Value: &dashboards.LogsAggregation_Count_{
				Count: &dashboards.LogsAggregation_Count{},
			},
		}, nil
	case "count_distinct":
		return &dashboards.LogsAggregation{
			Value: &dashboards.LogsAggregation_CountDistinct_{
				CountDistinct: &dashboards.LogsAggregation_CountDistinct{
					Field: typeStringToWrapperspbString(logsAggregation.Field),
				},
			},
		}, nil
	case "sum":
		return &dashboards.LogsAggregation{
			Value: &dashboards.LogsAggregation_Sum_{
				Sum: &dashboards.LogsAggregation_Sum{
					Field: typeStringToWrapperspbString(logsAggregation.Field),
				},
			},
		}, nil
	case "avg":
		return &dashboards.LogsAggregation{
			Value: &dashboards.LogsAggregation_Average_{
				Average: &dashboards.LogsAggregation_Average{
					Field: typeStringToWrapperspbString(logsAggregation.Field),
				},
			},
		}, nil
	case "min":
		return &dashboards.LogsAggregation{
			Value: &dashboards.LogsAggregation_Min_{
				Min: &dashboards.LogsAggregation_Min{
					Field: typeStringToWrapperspbString(logsAggregation.Field),
				},
			},
		}, nil
	case "max":
		return &dashboards.LogsAggregation{
			Value: &dashboards.LogsAggregation_Max_{
				Max: &dashboards.LogsAggregation_Max{
					Field: typeStringToWrapperspbString(logsAggregation.Field),
				},
			},
		}, nil
	default:
		return nil, diag.NewErrorDiagnostic("Error expand logs aggregation", fmt.Sprintf("unknown logs aggregation type %s", logsAggregation.Type.ValueString()))
	}
}

func expandLogsFilters(ctx context.Context, logsFilters []attr.Value) ([]*dashboards.Filter_LogsFilter, diag.Diagnostics) {
	var diags diag.Diagnostics
	expandedFilters := make([]*dashboards.Filter_LogsFilter, len(logsFilters))
	for _, w := range logsFilters {
		v, err := w.ToTerraformValue(ctx)
		if err != nil {
			diags.AddError("Extract Dashboard Logs Filter Error", err.Error())
			continue
		}
		var filter LogsFilterModel
		if err = v.As(&filter); err != nil {
			diags.AddError("Extract Dashboard Logs Filter Error", err.Error())
			continue
		}

		expandedFilter, expandFilterDiags := expandLogsFilter(ctx, filter)
		if expandFilterDiags.HasError() {
			diags.Append(expandFilterDiags...)
			continue
		}

		expandedFilters = append(expandedFilters, expandedFilter)
	}

	return expandedFilters, diags
}

func expandLogsFilter(ctx context.Context, logsFilter LogsFilterModel) (*dashboards.Filter_LogsFilter, diag.Diagnostics) {
	operator, diags := expandFilterOperator(ctx, logsFilter.Operator)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.Filter_LogsFilter{
		Field:    typeStringToWrapperspbString(logsFilter.Field),
		Operator: operator,
	}, nil
}

func expandBarChart(ctx context.Context, chart *BarChartModel) (*dashboards.Widget_Definition_BarChart, diag.Diagnostics) {
	var diags diag.Diagnostics

	query, diags := expandBarChartQuery(ctx, chart.Query)
	if diags.HasError() {
		return nil, diags
	}

	xaxis, dg := expandXAis(chart.XAxis)
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	return &dashboards.Widget_Definition_BarChart{
		BarChart: &dashboards.BarChart{
			Query:             query,
			MaxBarsPerChart:   typeInt64ToWrappedInt32(chart.MaxBarsPerChart),
			GroupNameTemplate: typeStringToWrapperspbString(chart.GroupNameTemplate),
			StackDefinition:   expandBarChartStackDefinition(chart.StackDefinition),
			ScaleType:         dashboardSchemaToProtoScaleType[chart.ScaleType.ValueString()],
			ColorsBy:          expandColorsBy(chart.ColorsBy),
			XAxis:             xaxis,
			Unit:              dashboardSchemaToProtoUnit[chart.Unit.ValueString()],
		},
	}, nil
}

func expandColorsBy(colorsBy types.String) *dashboards.BarChart_ColorsBy {
	switch colorsBy.ValueString() {
	case "stack":
		return &dashboards.BarChart_ColorsBy{
			Value: &dashboards.BarChart_ColorsBy_Stack{
				Stack: &dashboards.BarChart_ColorsBy_ColorsByStack{},
			},
		}
	case "group_by":
		return &dashboards.BarChart_ColorsBy{
			Value: &dashboards.BarChart_ColorsBy_GroupBy{
				GroupBy: &dashboards.BarChart_ColorsBy_ColorsByGroupBy{},
			},
		}
	default:
		return nil
	}
}

func expandXAis(xaxis *BarChartXAxisModel) (*dashboards.BarChart_XAxis, diag.Diagnostic) {
	if xaxis == nil {
		return nil, nil
	}

	switch xaxis.Type.ValueString() {
	case "time":
		duration, err := time.ParseDuration(xaxis.Interval.ValueString())
		if err != nil {
			return nil, diag.NewErrorDiagnostic("Error expand bar chart x axis", err.Error())
		}
		return &dashboards.BarChart_XAxis{
			Type: &dashboards.BarChart_XAxis_Time{
				Time: &dashboards.BarChart_XAxis_XAxisByTime{
					Interval:         durationpb.New(duration),
					BucketsPresented: typeInt64ToWrappedInt32(xaxis.BucketsPresented),
				},
			},
		}, nil
	case "value":
		return &dashboards.BarChart_XAxis{
			Type: &dashboards.BarChart_XAxis_Value{
				Value: &dashboards.BarChart_XAxis_XAxisByValue{},
			},
		}, nil
	default:
		return nil, diag.NewErrorDiagnostic("Error expand bar chart x axis", fmt.Sprintf("unknown bar chart x axis type %s", xaxis.Type.ValueString()))
	}
}
func expandBarChartQuery(ctx context.Context, query *BarChartQueryModel) (*dashboards.BarChart_Query, diag.Diagnostics) {
	if query == nil {
		return nil, nil
	}
	switch {
	case query.Logs != nil:
		logsQuery, diags := expandBarChartLogsQuery(ctx, query.Logs)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.BarChart_Query{
			Value: &dashboards.BarChart_Query_Logs{
				Logs: logsQuery,
			},
		}, nil
	case query.Metrics != nil:
		metricsQuery, diags := expandBarChartMetricsQuery(ctx, query.Metrics)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.BarChart_Query{
			Value: &dashboards.BarChart_Query_Metrics{
				Metrics: metricsQuery,
			},
		}, nil
	case query.Spans != nil:
		spansQuery, diags := expandBarChartSpansQuery(ctx, query.Spans)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.BarChart_Query{
			Value: &dashboards.BarChart_Query_Spans{
				Spans: spansQuery,
			},
		}, nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error expand bar chart query", "unknown bar chart query type")}
	}
}

func expandBarChartLogsQuery(ctx context.Context, barChartQueryLogs *BarChartQueryLogsModel) (*dashboards.BarChart_LogsQuery, diag.Diagnostics) {
	if barChartQueryLogs == nil {
		return nil, nil
	}

	aggregation, dg := expandLogsAggregation(barChartQueryLogs.Aggregation)
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	filters, diags := expandLogsFilters(ctx, barChartQueryLogs.Filters.Elements())
	if diags.HasError() {
		return nil, diags
	}

	groupNames, diags := typeStringSliceToWrappedStringSlice(ctx, barChartQueryLogs.GroupNames.Elements())
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.BarChart_LogsQuery{
		LuceneQuery:      expandLuceneQuery(barChartQueryLogs.LuceneQuery),
		Aggregation:      aggregation,
		Filters:          filters,
		GroupNames:       groupNames,
		StackedGroupName: typeStringToWrapperspbString(barChartQueryLogs.StackedGroupName),
	}, nil
}

func expandBarChartMetricsQuery(ctx context.Context, barChartQueryMetrics *BarChartQueryMetricsModel) (*dashboards.BarChart_MetricsQuery, diag.Diagnostics) {
	if barChartQueryMetrics == nil {
		return nil, nil
	}

	filters, diags := expandMetricsFilters(ctx, barChartQueryMetrics.Filters.Elements())
	if diags.HasError() {
		return nil, diags
	}

	groupNames, diags := typeStringSliceToWrappedStringSlice(ctx, barChartQueryMetrics.GroupNames.Elements())
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.BarChart_MetricsQuery{
		PromqlQuery:      expandPromqlQuery(barChartQueryMetrics.PromqlQuery),
		Filters:          filters,
		GroupNames:       groupNames,
		StackedGroupName: typeStringToWrapperspbString(barChartQueryMetrics.StackedGroupName),
	}, nil
}

func expandBarChartSpansQuery(ctx context.Context, barChartQuerySpans *BarChartQuerySpansModel) (*dashboards.BarChart_SpansQuery, diag.Diagnostics) {
	if barChartQuerySpans == nil {
		return nil, nil
	}

	aggregation, dg := expandSpansAggregation(barChartQuerySpans.Aggregation)
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	filters, diags := expandSpansFilters(ctx, barChartQuerySpans.Filters.Elements())
	if diags.HasError() {
		return nil, diags
	}

	groupNames, diags := expandSpansFields(ctx, barChartQuerySpans.GroupNames.Elements())
	if diags.HasError() {
		return nil, diags
	}

	expandedFilter, dg := expandSpansField(barChartQuerySpans.StackedGroupName)
	if dg != nil {
		diags.Append(dg)
		return nil, diags
	}

	return &dashboards.BarChart_SpansQuery{
		LuceneQuery:      expandLuceneQuery(barChartQuerySpans.LuceneQuery),
		Aggregation:      aggregation,
		Filters:          filters,
		GroupNames:       groupNames,
		StackedGroupName: expandedFilter,
	}, nil
}

func expandSpansFields(ctx context.Context, spanFields []attr.Value) ([]*dashboards.SpanField, diag.Diagnostics) {
	var diags diag.Diagnostics
	expandedSpanFields := make([]*dashboards.SpanField, len(spanFields))
	for _, w := range spanFields {
		v, err := w.ToTerraformValue(ctx)
		if err != nil {
			diags.AddError("Extract Dashboard Spans Field Error", err.Error())
			continue
		}
		var field SpansFieldModel
		if err = v.As(&field); err != nil {
			diags.AddError("Extract Dashboard Spans Field Error", err.Error())
			continue
		}

		expandedFilter, expandFilterDiag := expandSpansField(&field)
		if expandFilterDiag != nil {
			diags.Append(expandFilterDiag)
			continue
		}

		expandedSpanFields = append(expandedSpanFields, expandedFilter)
	}

	return expandedSpanFields, diags
}

func expandDataTable(ctx context.Context, table *DataTableModel) (*dashboards.Widget_Definition_DataTable, diag.Diagnostics) {
	query, diags := expandDataTableQuery(ctx, table.Query)
	if diags.HasError() {
		return nil, diags
	}

	columns, diags := expandDataTableColumns(ctx, table.Columns.Elements())
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.Widget_Definition_DataTable{
		DataTable: &dashboards.DataTable{
			Query:          query,
			ResultsPerPage: typeInt64ToWrappedInt32(table.ResultsPerPage),
			RowStyle:       dashboardRowStyleSchemaToProto[table.RowStyle.ValueString()],
			Columns:        columns,
			OrderBy:        expandOrderBy(table.OrderBy),
		},
	}, nil
}

func expandDataTableQuery(ctx context.Context, dataTableQuery *DataTableQueryModel) (*dashboards.DataTable_Query, diag.Diagnostics) {
	if dataTableQuery == nil {
		return nil, nil
	}
	switch {
	case dataTableQuery.Metrics != nil:
		metrics, diags := expandDataTableMetricsQuery(ctx, dataTableQuery.Metrics)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.DataTable_Query{
			Value: metrics,
		}, nil
	case dataTableQuery.Logs != nil:
		logs, diags := expandDataTableLogsQuery(ctx, dataTableQuery.Logs)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.DataTable_Query{
			Value: logs,
		}, nil
	case dataTableQuery.Spans != nil:
		spans, diags := expandDataTableSpansQuery(ctx, dataTableQuery.Spans)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.DataTable_Query{
			Value: spans,
		}, nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand DataTable Query", "unknown data table query type")}
	}
}

func expandDataTableMetricsQuery(ctx context.Context, dataTableQueryMetric *DataTableQueryMetricsModel) (*dashboards.DataTable_Query_Metrics, diag.Diagnostics) {
	if dataTableQueryMetric == nil {
		return nil, nil
	}

	filters, diags := expandMetricsFilters(ctx, dataTableQueryMetric.Filters.Elements())
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.DataTable_Query_Metrics{
		Metrics: &dashboards.DataTable_MetricsQuery{
			PromqlQuery: expandPromqlQuery(dataTableQueryMetric.PromqlQuery),
			Filters:     filters,
		},
	}, nil
}

func expandDataTableLogsQuery(ctx context.Context, dataTableQueryLogs *DataTableQueryLogsModel) (*dashboards.DataTable_Query_Logs, diag.Diagnostics) {
	if dataTableQueryLogs == nil {
		return nil, nil
	}

	filters, diags := expandLogsFilters(ctx, dataTableQueryLogs.Filters.Elements())
	if diags.HasError() {
		return nil, diags
	}

	grouping, diags := expandDataTableLogsGrouping(ctx, dataTableQueryLogs.Grouping)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.DataTable_Query_Logs{
		Logs: &dashboards.DataTable_LogsQuery{
			LuceneQuery: expandLuceneQuery(dataTableQueryLogs.LuceneQuery),
			Filters:     filters,
			Grouping:    grouping,
		},
	}, nil
}

func expandDataTableLogsGrouping(ctx context.Context, grouping *DataTableLogsQueryGroupingModel) (*dashboards.DataTable_LogsQuery_Grouping, diag.Diagnostics) {
	groupBy, diags := typeStringSliceToWrappedStringSlice(ctx, grouping.GroupBy.Elements())
	if diags.HasError() {
		return nil, diags
	}

	aggregations, diags := expandDataTableLogsAggregations(ctx, grouping.Aggregations.Elements())
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.DataTable_LogsQuery_Grouping{
		GroupBy:      groupBy,
		Aggregations: aggregations,
	}, nil

}

func expandDataTableLogsAggregations(ctx context.Context, aggregations []attr.Value) ([]*dashboards.DataTable_LogsQuery_Aggregation, diag.Diagnostics) {
	var diags diag.Diagnostics
	expandedAggregations := make([]*dashboards.DataTable_LogsQuery_Aggregation, len(aggregations))
	for _, s := range aggregations {
		v, err := s.ToTerraformValue(ctx)
		if err != nil {
			diags.AddError("Extract DataTable Logs Aggregations Error", err.Error())
			continue
		}
		var aggregation DataTableAggregationModel
		if err = v.As(&aggregation); err != nil {
			diags.AddError("Extract DataTable Logs Aggregations Error", err.Error())
			continue
		}

		expandedSection, expandSectionDiag := expandDataTableAggregation(&aggregation)
		if expandSectionDiag != nil {
			diags.Append(expandSectionDiag)
			continue
		}
		expandedAggregations = append(expandedAggregations, expandedSection)
	}

	return expandedAggregations, diags
}

func expandDataTableAggregation(aggregation *DataTableAggregationModel) (*dashboards.DataTable_LogsQuery_Aggregation, diag.Diagnostic) {
	if aggregation == nil {
		return nil, nil
	}

	logsAggregation, dg := expandLogsAggregation(aggregation.Aggregation)
	if dg != nil {
		return nil, dg
	}

	return &dashboards.DataTable_LogsQuery_Aggregation{
		Id:          typeStringToWrapperspbString(aggregation.ID),
		Name:        typeStringToWrapperspbString(aggregation.Name),
		IsVisible:   typeBoolToWrapperspbBool(aggregation.IsVisible),
		Aggregation: logsAggregation,
	}, nil
}

func expandDataTableSpansQuery(ctx context.Context, dataTableQuerySpans *DataTableQuerySpansModel) (*dashboards.DataTable_Query_Spans, diag.Diagnostics) {
	if dataTableQuerySpans == nil {
		return nil, nil
	}

	filters, diags := expandSpansFilters(ctx, dataTableQuerySpans.Filters.Elements())
	if diags.HasError() {
		return nil, diags
	}

	grouping, diags := expandDataTableSpansGrouping(ctx, dataTableQuerySpans.Grouping)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.DataTable_Query_Spans{
		Spans: &dashboards.DataTable_SpansQuery{
			LuceneQuery: expandLuceneQuery(dataTableQuerySpans.LuceneQuery),
			Filters:     filters,
			Grouping:    grouping,
		},
	}, nil
}

func expandDataTableSpansGrouping(ctx context.Context, grouping *DataTableSpansQueryGroupingModel) (*dashboards.DataTable_SpansQuery_Grouping, diag.Diagnostics) {
	if grouping == nil {
		return nil, nil
	}

	groupBy, diags := expandSpansFields(ctx, grouping.GroupBy.Elements())
	if diags.HasError() {
		return nil, diags
	}

	aggregations, diags := expandDataTableSpansAggregations(ctx, grouping.Aggregations.Elements())
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.DataTable_SpansQuery_Grouping{
		GroupBy:      groupBy,
		Aggregations: aggregations,
	}, nil
}

func expandDataTableSpansAggregations(ctx context.Context, aggregations []attr.Value) ([]*dashboards.DataTable_SpansQuery_Aggregation, diag.Diagnostics) {
	var diags diag.Diagnostics
	expandedAggregations := make([]*dashboards.DataTable_SpansQuery_Aggregation, len(aggregations))
	for _, s := range aggregations {
		v, err := s.ToTerraformValue(ctx)
		if err != nil {
			diags.AddError("Extract DataTable Spans Aggregations Error", err.Error())
			continue
		}
		var aggregation DataTableSpansAggregationModel
		if err = v.As(&aggregation); err != nil {
			diags.AddError("Extract DataTable Spans Aggregations Error", err.Error())
			continue
		}

		expandedSection, expandSectionDiag := expandDataTableSpansAggregation(&aggregation)
		if expandSectionDiag != nil {
			diags.Append(expandSectionDiag)
			continue
		}
		expandedAggregations = append(expandedAggregations, expandedSection)
	}

	return expandedAggregations, diags
}

func expandDataTableSpansAggregation(aggregation *DataTableSpansAggregationModel) (*dashboards.DataTable_SpansQuery_Aggregation, diag.Diagnostic) {
	if aggregation == nil {
		return nil, nil
	}

	spansAggregation, dg := expandSpansAggregation(aggregation.Aggregation)
	if dg != nil {
		return nil, dg
	}

	return &dashboards.DataTable_SpansQuery_Aggregation{
		Id:          typeStringToWrapperspbString(aggregation.ID),
		Name:        typeStringToWrapperspbString(aggregation.Name),
		IsVisible:   typeBoolToWrapperspbBool(aggregation.IsVisible),
		Aggregation: spansAggregation,
	}, nil
}

func expandDataTableColumns(ctx context.Context, columns []attr.Value) ([]*dashboards.DataTable_Column, diag.Diagnostics) {
	var diags diag.Diagnostics
	expandedColumns := make([]*dashboards.DataTable_Column, len(columns))
	for _, s := range columns {
		v, err := s.ToTerraformValue(ctx)
		if err != nil {
			diags.AddError("Extract DataTable DataTable Columns Error", err.Error())
			continue
		}
		var column DataTableColumnModel
		if err = v.As(&column); err != nil {
			diags.AddError("Extract DataTable DataTable Columns Error", err.Error())
			continue
		}

		expandedColumn := expandDataTableColumn(&column)
		expandedColumns = append(expandedColumns, expandedColumn)
	}

	return expandedColumns, diags
}

func expandDataTableColumn(column *DataTableColumnModel) *dashboards.DataTable_Column {
	if column == nil {
		return nil
	}
	return &dashboards.DataTable_Column{
		Field: typeStringToWrapperspbString(column.Field),
		Width: typeInt64ToWrappedInt32(column.Width),
	}
}

func expandOrderBy(orderBy *OrderByModel) *dashboards.OrderingField {
	if orderBy == nil {
		return nil
	}
	return &dashboards.OrderingField{
		Field:          typeStringToWrapperspbString(orderBy.Field),
		OrderDirection: dashboardOrderDirectionSchemaToProto[orderBy.OrderDirection.ValueString()],
	}
}
func expandLineChart(ctx context.Context, lineChart *LineChartModel) (*dashboards.Widget_Definition_LineChart, diag.Diagnostics) {
	if lineChart == nil {
		return nil, nil
	}

	legend, diags := expandLineChartLegend(ctx, lineChart.Legend)
	if diags.HasError() {
		return nil, diags
	}

	queryDefinitions, diags := expandLineChartQueryDefinitions(ctx, lineChart.QueryDefinitions.Elements())
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.Widget_Definition_LineChart{
		LineChart: &dashboards.LineChart{
			Legend:           legend,
			Tooltip:          expandLineChartTooltip(lineChart.Tooltip),
			QueryDefinitions: queryDefinitions,
		},
	}, nil
}

func expandLineChartLegend(ctx context.Context, legend *LegendModel) (*dashboards.Legend, diag.Diagnostics) {
	if legend == nil {
		return nil, nil
	}

	columns, diags := expandLineChartLegendColumns(ctx, legend.Columns.Elements())
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.Legend{
		IsVisible:    typeBoolToWrapperspbBool(legend.IsVisible),
		Columns:      columns,
		GroupByQuery: typeBoolToWrapperspbBool(legend.GroupByQuery),
	}, nil
}

func expandLineChartLegendColumns(ctx context.Context, columns []attr.Value) ([]dashboards.Legend_LegendColumn, diag.Diagnostics) {
	var diags diag.Diagnostics
	expandedColumns := make([]dashboards.Legend_LegendColumn, len(columns))
	for _, s := range columns {
		v, err := s.ToTerraformValue(ctx)
		if err != nil {
			diags.AddError("Extract LineChart Legend Columns Error", err.Error())
			continue
		}
		var column string
		if err = v.As(&column); err != nil {
			diags.AddError("Extract LineChart Legend Columns Error", err.Error())
			continue
		}

		expandedColumn := dashboardProtoToSchemaLegendColumn[column]
		expandedColumns = append(expandedColumns, expandedColumn)
	}

	return expandedColumns, diags
}

func expandLineChartTooltip(tooltip *LineChartTooltipModel) *dashboards.LineChart_Tooltip {
	if tooltip == nil {
		return nil
	}

	return &dashboards.LineChart_Tooltip{
		ShowLabels: typeBoolToWrapperspbBool(tooltip.ShowLabels),
		Type:       dashboardSchemaToProtoTooltipType[tooltip.Type.ValueString()],
	}
}

func expandLineChartQueryDefinitions(ctx context.Context, queryDefinitions []attr.Value) ([]*dashboards.LineChart_QueryDefinition, diag.Diagnostics) {
	var diags diag.Diagnostics
	expandedQueryDefinitions := make([]*dashboards.LineChart_QueryDefinition, len(queryDefinitions))
	for _, s := range queryDefinitions {
		v, err := s.ToTerraformValue(ctx)
		if err != nil {
			diags.AddError("Extract LineChart DataTable Query Definitions Error", err.Error())
			continue
		}
		var queryDefinition LineChartQueryDefinitionModel
		if err = v.As(&queryDefinition); err != nil {
			diags.AddError("Extract LineChart DataTable Query Definitions Error", err.Error())
			continue
		}

		expandedQueryDefinition, expandedDiags := expandLineChartQueryDefinition(ctx, &queryDefinition)
		if expandedDiags.HasError() {
			diags.Append(expandedDiags...)
		}
		expandedQueryDefinitions = append(expandedQueryDefinitions, expandedQueryDefinition)
	}

	return expandedQueryDefinitions, diags
}

func expandLineChartQueryDefinition(ctx context.Context, queryDefinition *LineChartQueryDefinitionModel) (*dashboards.LineChart_QueryDefinition, diag.Diagnostics) {
	if queryDefinition == nil {
		return nil, nil
	}
	query, diags := expandLineChartQuery(ctx, queryDefinition.Query)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.LineChart_QueryDefinition{
		Id:                 typeStringToWrapperspbString(queryDefinition.ID),
		Query:              query,
		SeriesNameTemplate: typeStringToWrapperspbString(queryDefinition.SeriesNameTemplate),
		SeriesCountLimit:   typeInt64ToWrappedInt64(queryDefinition.SeriesCountLimit),
		Unit:               dashboardSchemaToProtoUnit[queryDefinition.Unit.ValueString()],
		ScaleType:          dashboardSchemaToProtoScaleType[queryDefinition.ScaleType.ValueString()],
		Name:               typeStringToWrapperspbString(queryDefinition.Name),
		IsVisible:          typeBoolToWrapperspbBool(queryDefinition.IsVisible),
	}, nil
}

func expandLineChartQuery(ctx context.Context, query *LineChartQueryModel) (*dashboards.LineChart_Query, diag.Diagnostics) {
	if query == nil {
		return nil, nil
	}

	switch {
	case query.Logs != nil:
		logs, diags := expandLineChartLogsQuery(ctx, query.Logs)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.LineChart_Query{
			Value: logs,
		}, nil
	case query.Metrics != nil:
		metrics, diags := expandLineChartMetricsQuery(ctx, query.Metrics)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.LineChart_Query{
			Value: metrics,
		}, nil
	case query.Spans != nil:
		spans, diags := expandLineChartSpansQuery(ctx, query.Spans)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.LineChart_Query{
			Value: spans,
		}, nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand LineChart Query", "Unknown LineChart Query type")}
	}
}

func expandLineChartLogsQuery(ctx context.Context, logs *LineChartQueryLogsModel) (*dashboards.LineChart_Query_Logs, diag.Diagnostics) {
	if logs == nil {
		return nil, nil
	}

	groupBy, diags := typeStringSliceToWrappedStringSlice(ctx, logs.GroupBy.Elements())
	if diags.HasError() {
		return nil, diags
	}

	aggregations, diags := expandLogsAggregations(ctx, logs.Aggregations.Elements())
	if diags.HasError() {
		return nil, diags
	}

	filters, diags := expandLogsFilters(ctx, logs.Filters.Elements())
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.LineChart_Query_Logs{
		Logs: &dashboards.LineChart_LogsQuery{
			LuceneQuery:  expandLuceneQuery(logs.LuceneQuery),
			GroupBy:      groupBy,
			Aggregations: aggregations,
			Filters:      filters,
		},
	}, nil
}

func expandLineChartMetricsQuery(ctx context.Context, metrics *LineChartQueryMetricsModel) (*dashboards.LineChart_Query_Metrics, diag.Diagnostics) {
	if metrics == nil {
		return nil, nil
	}

	filters, diags := expandMetricsFilters(ctx, metrics.Filters.Elements())
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.LineChart_Query_Metrics{
		Metrics: &dashboards.LineChart_MetricsQuery{
			PromqlQuery: expandPromqlQuery(metrics.PromqlQuery),
			Filters:     filters,
		},
	}, nil
}

func expandLineChartSpansQuery(ctx context.Context, spans *LineChartQuerySpansModel) (*dashboards.LineChart_Query_Spans, diag.Diagnostics) {
	if spans == nil {
		return nil, nil
	}

	groupBy, diags := expandSpansFields(ctx, spans.GroupBy.Elements())

	aggregations, diags := expandSpansAggregations(ctx, spans.Aggregations.Elements())
	if diags.HasError() {
		return nil, diags
	}

	filters, diags := expandSpansFilters(ctx, spans.Filters.Elements())
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.LineChart_Query_Spans{
		Spans: &dashboards.LineChart_SpansQuery{
			LuceneQuery:  expandLuceneQuery(spans.LuceneQuery),
			GroupBy:      groupBy,
			Aggregations: aggregations,
			Filters:      filters,
		},
	}, nil
}

func expandDashboardQuery(ctx context.Context, pieChartQuery *PieChartQueryModel) (*dashboards.PieChart_Query, diag.Diagnostics) {
	if pieChartQuery == nil {
		return nil, nil
	}

	switch {
	case pieChartQuery.Logs != nil:
		logs, diags := expandPieChartLogsQuery(ctx, pieChartQuery.Logs)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.PieChart_Query{
			Value: logs,
		}, nil
	case pieChartQuery.Metrics != nil:
		metrics, diags := expandPieChartMetricsQuery(ctx, pieChartQuery.Metrics)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.PieChart_Query{
			Value: metrics,
		}, nil
	case pieChartQuery.Spans != nil:
		spans, diags := expandPieChartSpansQuery(ctx, pieChartQuery.Spans)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.PieChart_Query{
			Value: spans,
		}, nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand PieChart Query", "Unknown PieChart Query type")}
	}
}

func expandPieChartLogsQuery(ctx context.Context, pieChartQueryLogs *PieChartQueryLogsModel) (*dashboards.PieChart_Query_Logs, diag.Diagnostics) {
	if pieChartQueryLogs == nil {
		return nil, nil
	}

	aggregation, dg := expandLogsAggregation(pieChartQueryLogs.Aggregation)
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	filters, diags := expandLogsFilters(ctx, pieChartQueryLogs.Filters.Elements())
	if diags.HasError() {
		return nil, diags
	}

	groupNames, diags := typeStringSliceToWrappedStringSlice(ctx, pieChartQueryLogs.GroupNames.Elements())
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.PieChart_Query_Logs{
		Logs: &dashboards.PieChart_LogsQuery{
			LuceneQuery:      expandLuceneQuery(pieChartQueryLogs.LuceneQuery),
			Aggregation:      aggregation,
			Filters:          filters,
			GroupNames:       groupNames,
			StackedGroupName: typeStringToWrapperspbString(pieChartQueryLogs.StackedGroupName),
		},
	}, nil
}

func expandPieChartMetricsQuery(ctx context.Context, pieChartQueryMetrics *PieChartQueryMetricsModel) (*dashboards.PieChart_Query_Metrics, diag.Diagnostics) {
	if pieChartQueryMetrics == nil {
		return nil, nil
	}

	filters, diags := expandMetricsFilters(ctx, pieChartQueryMetrics.Filters.Elements())
	if diags.HasError() {
		return nil, diags
	}

	groupNames, diags := typeStringSliceToWrappedStringSlice(ctx, pieChartQueryMetrics.GroupNames.Elements())
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.PieChart_Query_Metrics{
		Metrics: &dashboards.PieChart_MetricsQuery{
			PromqlQuery:      expandPromqlQuery(pieChartQueryMetrics.PromqlQuery),
			GroupNames:       groupNames,
			Filters:          filters,
			StackedGroupName: typeStringToWrapperspbString(pieChartQueryMetrics.StackedGroupName),
		},
	}, nil
}

func expandPieChartSpansQuery(ctx context.Context, pieChartQuerySpans *PieChartQuerySpansModel) (*dashboards.PieChart_Query_Spans, diag.Diagnostics) {
	if pieChartQuerySpans == nil {
		return nil, nil
	}

	aggregation, dg := expandSpansAggregation(pieChartQuerySpans.Aggregation)
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	filters, diags := expandSpansFilters(ctx, pieChartQuerySpans.Filters.Elements())
	if diags.HasError() {
		return nil, diags
	}

	groupNames, diags := expandSpansFields(ctx, pieChartQuerySpans.GroupNames.Elements())
	if diags.HasError() {
		return nil, diags
	}

	stackedGroupName, dg := expandSpansField(pieChartQuerySpans.StackedGroupName)
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	return &dashboards.PieChart_Query_Spans{
		Spans: &dashboards.PieChart_SpansQuery{
			LuceneQuery:      expandLuceneQuery(pieChartQuerySpans.LuceneQuery),
			Aggregation:      aggregation,
			Filters:          filters,
			GroupNames:       groupNames,
			StackedGroupName: stackedGroupName,
		},
	}, nil
}

func expandDashboardVariables(ctx context.Context, variables []attr.Value) ([]*dashboards.Variable, diag.Diagnostics) {
	var diags diag.Diagnostics
	expandedVariables := make([]*dashboards.Variable, len(variables))
	for _, e := range variables {
		v, err := e.ToTerraformValue(ctx)
		if err != nil {
			diags.AddError("Extract Dashboard Variable Error", err.Error())
			continue
		}
		var variable DashboardVariableModel
		if err = v.As(&variable); err != nil {
			diags.AddError("Extract Dashboard Variable Error", err.Error())
			continue
		}

		expandedVariable, expandDiags := expandedDashboardVariable(ctx, variable)
		if expandDiags.HasError() {
			diags.Append(expandDiags...)
			continue
		}

		expandedVariables = append(expandedVariables, expandedVariable)
	}

	return expandedVariables, diags
}

func expandedDashboardVariable(ctx context.Context, variable DashboardVariableModel) (*dashboards.Variable, diag.Diagnostics) {
	definition, diags := expandDashboardVariableDefinition(ctx, variable.Definition)
	if diags.HasError() {
		return nil, diags
	}
	return &dashboards.Variable{
		Name:        typeStringToWrapperspbString(variable.Name),
		DisplayName: typeStringToWrapperspbString(variable.DisplayName),
		Definition:  definition,
	}, nil
}

func expandDashboardVariableDefinition(ctx context.Context, definition *DashboardVariableDefinitionModel) (*dashboards.Variable_Definition, diag.Diagnostics) {
	if definition == nil {
		return nil, nil
	}

	switch {
	case definition.MultiSelect != nil:
		return expandMultiSelect(ctx, definition.MultiSelect)
	case !definition.ConstantValue.IsNull():
		return &dashboards.Variable_Definition{
			Value: &dashboards.Variable_Definition_Constant{
				Constant: &dashboards.Constant{
					Value: typeStringToWrapperspbString(definition.ConstantValue),
				},
			},
		}, nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand Dashboard Variable", fmt.Sprintf("unknown variable definition type: %T", definition))}
	}
}

func expandMultiSelect(ctx context.Context, multiSelect *VariableMultiSelectModel) (*dashboards.Variable_Definition, diag.Diagnostics) {
	if multiSelect == nil {
		return nil, nil
	}

	source, diags := expandMultiSelectSource(ctx, multiSelect.Source)
	if diags.HasError() {
		return nil, diags
	}

	selection, diags := expandMultiSelectSelection(ctx, multiSelect.SelectedValues.Elements())
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.Variable_Definition{
		Value: &dashboards.Variable_Definition_MultiSelect{
			MultiSelect: &dashboards.MultiSelect{
				Source:               source,
				Selection:            selection,
				ValuesOrderDirection: dashboardOrderDirectionSchemaToProto[multiSelect.ValuesOrderDirection.ValueString()],
			},
		},
	}, nil
}

func expandMultiSelectSelection(ctx context.Context, selectedValues []attr.Value) (*dashboards.MultiSelect_Selection, diag.Diagnostics) {
	if len(selectedValues) == 0 {
		return &dashboards.MultiSelect_Selection{
			Value: &dashboards.MultiSelect_Selection_All{
				All: &dashboards.MultiSelect_Selection_AllSelection{},
			},
		}, nil
	}

	selections, diags := typeStringSliceToWrappedStringSlice(ctx, selectedValues)
	if diags.HasError() {
		return nil, diags
	}
	return &dashboards.MultiSelect_Selection{
		Value: &dashboards.MultiSelect_Selection_List{
			List: &dashboards.MultiSelect_Selection_ListSelection{
				Values: selections,
			},
		},
	}, nil
}

func expandMultiSelectSource(ctx context.Context, source *VariableMultiSelectSourceModel) (*dashboards.MultiSelect_Source, diag.Diagnostics) {
	if source == nil {
		return nil, nil
	}

	switch {
	case source.LogPath.IsNull():
		return &dashboards.MultiSelect_Source{
			Value: &dashboards.MultiSelect_Source_LogsPath{
				LogsPath: &dashboards.MultiSelect_LogsPathSource{
					Value: typeStringToWrapperspbString(source.LogPath),
				},
			},
		}, nil
	case len(source.ConstantList.Elements()) > 0:
		constantList, diags := typeStringSliceToWrappedStringSlice(ctx, source.ConstantList.Elements())
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.MultiSelect_Source{
			Value: &dashboards.MultiSelect_Source_ConstantList{
				ConstantList: &dashboards.MultiSelect_ConstantListSource{
					Values: constantList,
				},
			},
		}, nil
	case source.Metric != nil:
		return &dashboards.MultiSelect_Source{
			Value: &dashboards.MultiSelect_Source_MetricLabel{
				MetricLabel: &dashboards.MultiSelect_MetricLabelSource{
					MetricName: typeStringToWrapperspbString(source.Metric.Name),
					Label:      typeStringToWrapperspbString(source.Metric.Label),
				},
			},
		}, nil
	case source.SpanField != nil:
		spanField, dg := expandSpansField(source.SpanField)
		if dg != nil {
			return nil, diag.Diagnostics{dg}
		}
		return &dashboards.MultiSelect_Source{
			Value: &dashboards.MultiSelect_Source_SpanField{
				SpanField: &dashboards.MultiSelect_SpanFieldSource{
					Value: spanField,
				},
			},
		}, nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand Multi Select Source", fmt.Sprintf("unknown multi select source type: %T", source))}
	}
}

func expandDashboardFilters(ctx context.Context, filters []attr.Value) ([]*dashboards.Filter, diag.Diagnostics) {
	var diags diag.Diagnostics
	expandedFilters := make([]*dashboards.Filter, len(filters))
	for _, s := range filters {
		v, err := s.ToTerraformValue(ctx)
		if err != nil {
			diags.AddError("Extract Dashboard Filters Error", err.Error())
			continue
		}
		var filter DashboardFilterModel
		if err = v.As(&filter); err != nil {
			diags.AddError("Extract Dashboard Filters Error", err.Error())
			continue
		}

		expandedFilter, expandDiags := expandDashboardFilter(ctx, &filter)
		if expandDiags.HasError() {
			diags.Append(expandDiags...)
			continue
		}
		expandedFilters = append(expandedFilters, expandedFilter)
	}

	return expandedFilters, diags
}

func expandDashboardFilter(ctx context.Context, filter *DashboardFilterModel) (*dashboards.Filter, diag.Diagnostics) {
	if filter == nil {
		return nil, nil
	}

	source, diags := expandFilterSource(ctx, filter)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.Filter{
		Source:    source,
		Enabled:   typeBoolToWrapperspbBool(filter.Enabled),
		Collapsed: typeBoolToWrapperspbBool(filter.Collapsed),
	}, nil
}

func expandFilterSource(ctx context.Context, filter *DashboardFilterModel) (*dashboards.Filter_Source, diag.Diagnostics) {
	switch {
	case filter.Logs != nil:
		return expandFilterSourceLogs(ctx, filter.Logs)
	case filter.Metrics != nil:
		return expandFilterSourceMetrics(ctx, filter.Metrics)
	case filter.Spans != nil:
		return expandFilterSourceSpans(ctx, filter.Spans)
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand Filter Source", fmt.Sprintf("Unknown filter source type: %#v", filter))}
	}
}

func expandFilterSourceLogs(ctx context.Context, logs *FilterSourceLogsModel) (*dashboards.Filter_Source, diag.Diagnostics) {
	if logs == nil {
		return nil, nil
	}

	operator, diags := expandFilterOperator(ctx, logs.Operator)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.Filter_Source{
		Value: &dashboards.Filter_Source_Logs{
			Logs: &dashboards.Filter_LogsFilter{
				Field:    typeStringToWrapperspbString(logs.Field),
				Operator: operator,
			},
		},
	}, nil
}

func expandFilterSourceMetrics(ctx context.Context, metrics *FilterSourceMetricsModel) (*dashboards.Filter_Source, diag.Diagnostics) {
	if metrics == nil {
		return nil, nil
	}

	operator, diags := expandFilterOperator(ctx, metrics.Operator)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.Filter_Source{
		Value: &dashboards.Filter_Source_Metrics{
			Metrics: &dashboards.Filter_MetricsFilter{
				Metric:   typeStringToWrapperspbString(metrics.MetricName),
				Label:    typeStringToWrapperspbString(metrics.MetricLabel),
				Operator: operator,
			},
		},
	}, nil
}

func expandFilterSourceSpans(ctx context.Context, spans *FilterSourceSpansModel) (*dashboards.Filter_Source, diag.Diagnostics) {
	if spans == nil {
		return nil, nil
	}

	field, dg := expandSpansField(spans.Field)
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	operator, diags := expandFilterOperator(ctx, spans.Operator)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.Filter_Source{
		Value: &dashboards.Filter_Source_Spans{
			Spans: &dashboards.Filter_SpansFilter{
				Field:    field,
				Operator: operator,
			},
		},
	}, nil
}

func expandAbsoluteeDashboardTimeFrame(timeFrame *DashboardTimeFrameAbsoluteModel) (*dashboards.Dashboard_AbsoluteTimeFrame, diag.Diagnostic) {
	if timeFrame == nil {
		return nil, nil
	}

	fromTime, err := time.Parse(time.RFC3339, timeFrame.From.ValueString())
	if err != nil {
		return nil, diag.NewErrorDiagnostic("Error Expand Absolutee Dashboard Time Frame", fmt.Sprintf("Error parsing from time: %s", err.Error()))
	}
	toTime, err := time.Parse(time.RFC3339, timeFrame.To.ValueString())
	if err != nil {
		return nil, diag.NewErrorDiagnostic("Error Expand Absolutee Dashboard Time Frame", fmt.Sprintf("Error parsing from time: %s", err.Error()))
	}

	from := timestamppb.New(fromTime)
	to := timestamppb.New(toTime)

	return &dashboards.Dashboard_AbsoluteTimeFrame{
		AbsoluteTimeFrame: &dashboards.TimeFrame{
			From: from,
			To:   to,
		},
	}, nil
}

func expandRelativeDashboardTimeFrame(timeFrame *DashboardTimeFrameRelativeModel) (*dashboards.Dashboard_RelativeTimeFrame, diag.Diagnostic) {
	if timeFrame == nil {
		return nil, nil
	}
	duration, err := time.ParseDuration(timeFrame.Duration.ValueString())
	if err != nil {
		return nil, diag.NewErrorDiagnostic("Error Expand Relative Dashboard Time Frame", fmt.Sprintf("Error parsing duration: %s", err.Error()))
	}
	return &dashboards.Dashboard_RelativeTimeFrame{
		RelativeTimeFrame: durationpb.New(duration),
	}, nil
}

func expandDashboardUUID(id types.String) *dashboards.UUID {
	if id.IsNull() || id.IsUnknown() {
		return &dashboards.UUID{Value: RandStringBytes(21)}
	}
	return &dashboards.UUID{Value: id.ValueString()}
}

func flattenDashboard(ctx context.Context, dashboard *dashboards.Dashboard) DashboardResourceModel {
	return DashboardResourceModel{
		ID:          types.StringValue(dashboard.GetId().GetValue()),
		Name:        types.StringValue(dashboard.GetName().GetValue()),
		Description: types.StringValue(dashboard.GetDescription().GetValue()),
		Layout:      flattenDashboardLayout(ctx, dashboard.GetLayout()),
		Variables:   flattenDashboardVariables(ctx, dashboard.GetVariables()),
		Filters:     flattenDashboardFilters(ctx, dashboard.GetFilters()),
		TimeFrame:   flattenDashboardTimeFrame(ctx, dashboard),
	}
}

func flattenDashboardLayout(ctx context.Context, layout *dashboards.Layout) *DashboardLayoutModel {
	return &DashboardLayoutModel{
		Sections: flattenDashboardSections(ctx, layout.GetSections()),
	}
}

func flattenDashboardSections(ctx context.Context, sections []*dashboards.Section) (types.List, diag.Diagnostics) {
	if len(sections) == 0 {
		return types.ListNull(types.ObjectType{}), nil
	}
	elements := make([]attr.Value, 0, len(sections))
	for _, v := range sections {
		elements = append(elements, types.ObjectValueMust())
	}
	return types.SetValueMust(types.StringType, elements)

	sectionList := make([]SectionModel, 0, len(sections))
	for _, section := range sections {
		flattenedSection := flattenDashboardSection(ctx, section)
		sectionList = append(sectionList, flattenedSection)
	}

	return types.ListValueFrom(ctx, types.ObjectType{}, sectionList)
}

func flattenDashboardSection(ctx context.Context, section *dashboards.Section) SectionModel {

}

func flattenDashboardVariables(ctx context.Context, variables []*dashboards.Variable) types.List {

}

func flattenDashboardFilters(ctx context.Context, filters []*dashboards.Filter) types.List {

}

func flattenDashboardTimeFrame(ctx context.Context, d *dashboards.Dashboard) *DashboardTimeFrameModel {
	switch d.GetTimeFrame().(type) {
	case *dashboards.Dashboard_AbsoluteeTimeFrame:
		return flattenAbsoluteDashboardTimeFrame(ctx, d.GetAbsoluteeTimeFrame())
	case *dashboards.Dashboard_RelativeTimeFrame:
		return flattenRelativeDashboardTimeFrame(ctx, d.GetRelativeTimeFrame())
	default:
		return nil
	}
}

func flattenAbsoluteDashboardTimeFrame(ctx context.Context, timeFrame *dashboards.TimeFrame) *DashboardTimeFrameModel {
	return &DashboardTimeFrameModel{
		Absolute: &DashboardTimeFrameAbsoluteModel{
			From: types.TimestampValue(timeFrame.GetFrom()),
			To:   types.TimestampValue(timeFrame.GetTo()),
		},
	}
}

func flattenRelativeDashboardTimeFrame(ctx context.Context, timeFrame *durationpb.Duration) *DashboardTimeFrameModel {
	return &DashboardTimeFrameModel{
		Relative: &DashboardTimeFrameRelativeModel{
			Duration: types.StringValue(timeFrame.String()),
		},
	}
}

func (r DashboardResource) Read(ctx context.Context, request resource.ReadRequest, response *resource.ReadResponse) {
	//TODO implement me
	panic("implement me")
}

func (r DashboardResource) Update(ctx context.Context, request resource.UpdateRequest, response *resource.UpdateResponse) {
	//TODO implement me
	panic("implement me")
}

func (r DashboardResource) Delete(ctx context.Context, request resource.DeleteRequest, response *resource.DeleteResponse) {
	//TODO implement me
	panic("implement me")
}

func (r DashboardResource) Configure(ctx context.Context, request resource.ConfigureRequest, response *resource.ConfigureResponse) {
	//TODO implement me
	panic("implement me")
}

type DashboardResourceModel struct {
	ID          types.String             `tfsdk:"id"`
	Name        types.String             `tfsdk:"name"`
	Description types.String             `tfsdk:"description"`
	Layout      *DashboardLayoutModel    `tfsdk:"layout"`
	Variables   types.List               `tfsdk:"variables"`
	Filters     types.List               `tfsdk:"filters"`
	TimeFrame   *DashboardTimeFrameModel `tfsdk:"time_frame"`
	ContentJson types.String             `tfsdk:"content_json"`
}

type DashboardLayoutModel struct {
	Sections types.List `tfsdk:"sections"`
}

type SectionModel struct {
	ID   types.String `tfsdk:"id"`
	Rows types.List   `tfsdk:"rows"`
}

type RowModel struct {
	ID      types.String `tfsdk:"id"`
	Height  types.Int64  `tfsdk:"height"`
	Widgets types.List   `tfsdk:"widget"`
}

type WidgetModel struct {
	ID          types.String           `tfsdk:"id"`
	Title       types.String           `tfsdk:"title"`
	Description types.String           `tfsdk:"description"`
	Definition  *WidgetDefinitionModel `tfsdk:"definition"`
	Width       types.Int64            `tfsdk:"width"`
}

type WidgetDefinitionModel struct {
	LineChart *LineChartModel `tfsdk:"line_chart"`
	DataTable *DataTableModel `tfsdk:"data_table"`
	Gauge     *GaugeModel     `tfsdk:"gauge"`
	PieChart  *PieChartModel  `tfsdk:"pie_chart"`
	BarChart  *BarChartModel  `tfsdk:"bar_chart"`
}

type LineChartModel struct {
	Legend           *LegendModel           `tfsdk:"legend"`
	Tooltip          *LineChartTooltipModel `tfsdk:"tooltip"`
	QueryDefinitions types.List             `tfsdk:"query_definitions"`
}

type LegendModel struct {
	IsVisible    types.Bool `tfsdk:"is_visible"`
	Columns      types.List `tfsdk:"columns"`
	GroupByQuery types.Bool `tfsdk:"group_by_query"`
}

type LineChartTooltipModel struct {
	ShowLabels types.Bool   `tfsdk:"show_labels"`
	Type       types.String `tfsdk:"type"`
}

type LineChartQueryDefinitionModel struct {
	ID                 types.String         `tfsdk:"id"`
	Query              *LineChartQueryModel `tfsdk:"query"`
	SeriesNameTemplate types.String         `tfsdk:"series_name_template"`
	SeriesCountLimit   types.Int64          `tfsdk:"series_count_limit"`
	Unit               types.String         `tfsdk:"unit"`
	ScaleType          types.String         `tfsdk:"scale_type"`
	Name               types.String         `tfsdk:"name"`
	IsVisible          types.Bool           `tfsdk:"is_visible"`
}

type LineChartQueryModel struct {
	Logs    *LineChartQueryLogsModel    `tfsdk:"logs"`
	Metrics *LineChartQueryMetricsModel `tfsdk:"metrics"`
	Spans   *LineChartQuerySpansModel   `tfsdk:"spans"`
}

type LineChartQueryLogsModel struct {
	LuceneQuery  types.String `tfsdk:"lucene_query"`
	GroupBy      types.List   `tfsdk:"group_by"`
	Aggregations types.List   `tfsdk:"aggregations"`
	Filters      types.List   `tfsdk:"filters"`
}

type QueryLogsAggregationModel struct {
	Type  types.String `tfsdk:"type"`
	Field types.String `tfsdk:"field"`
}

type FilterModel struct {
	Field    types.String         `tfsdk:"field"`
	Operator *FilterOperatorModel `tfsdk:"operator"`
}

type FilterOperatorModel struct {
	Type           types.String `tfsdk:"type"`
	SelectedValues types.List   `tfsdk:"selected_values"`
}

type LineChartQueryMetricsModel struct {
	PromqlQuery types.String `tfsdk:"promql_query"`
	Filters     types.List   `tfsdk:"filters"`
}

type QueryMetricFilterModel struct {
	Metric   types.String         `tfsdk:"metric"`
	Label    types.String         `tfsdk:"label"`
	Operator *FilterOperatorModel `tfsdk:"operator"`
}

type LineChartQuerySpansModel struct {
	LuceneQuery  types.String `tfsdk:"lucene_query"`
	GroupBy      types.List   `tfsdk:"group_by"`
	Aggregations types.List   `tfsdk:"aggregations"`
	Filters      types.List   `tfsdk:"filters"`
}

type SpansAggregationModel struct {
	Type            types.String `tfsdk:"type"`
	AggregationType types.String `tfsdk:"aggregation_type"`
	Field           types.String `tfsdk:"field"`
}

type DataTableModel struct {
	Query          *DataTableQueryModel `tfsdk:"query"`
	ResultsPerPage types.Int64          `tfsdk:"results_per_page"`
	RowStyle       types.String         `tfsdk:"row_style"`
	Columns        types.List           `tfsdk:"columns"`
	OrderBy        *OrderByModel        `tfsdk:"order_by"`
}

type DataTableQueryLogsModel struct {
	LuceneQuery types.String                     `tfsdk:"lucene_query"`
	Filters     types.List                       `tfsdk:"filters"`
	Grouping    *DataTableLogsQueryGroupingModel `tfsdk:"grouping"`
}

type LogsFilterModel struct {
	Field    types.String         `tfsdk:"field"`
	Operator *FilterOperatorModel `tfsdk:"operator"`
}

type DataTableLogsQueryGroupingModel struct {
	GroupBy      types.List `tfsdk:"group_by"`
	Aggregations types.List `tfsdk:"aggregations"`
}

type DataTableAggregationModel struct {
	ID          types.String      `tfsdk:"id"`
	Name        types.String      `tfsdk:"name"`
	IsVisible   types.Bool        `tfsdk:"is_visible"`
	Aggregation *AggregationModel `tfsdk:"aggregation"`
}

type AggregationModel struct {
	Type  types.String `tfsdk:"type"`
	Field types.String `tfsdk:"field"`
}

type DataTableQueryModel struct {
	Logs    *DataTableQueryLogsModel    `tfsdk:"logs"`
	Metrics *DataTableQueryMetricsModel `tfsdk:"metrics"`
	Spans   *DataTableQuerySpansModel   `tfsdk:"spans"`
}

type DataTableQueryMetricsModel struct {
	PromqlQuery types.String `tfsdk:"promql_query"`
	Filters     types.List   `tfsdk:"filters"`
}

type MetricsFilterModel struct {
	Metric   types.String         `tfsdk:"metric"`
	Label    types.String         `tfsdk:"label"`
	Operator *FilterOperatorModel `tfsdk:"operator"`
}

type DataTableColumnModel struct {
	Field types.String `tfsdk:"field"`
	Width types.Int64  `tfsdk:"width"`
}

type OrderByModel struct {
	Field          types.String `tfsdk:"field"`
	OrderDirection types.String `tfsdk:"order_direction"`
}

type DataTableQuerySpansModel struct {
	LuceneQuery types.String                      `tfsdk:"lucene_query"`
	Filters     types.List                        `tfsdk:"filters"`
	Grouping    *DataTableSpansQueryGroupingModel `tfsdk:"grouping"`
}

type SpansFilterModel struct {
	Field    *SpansFieldModel     `tfsdk:"field"`
	Operator *FilterOperatorModel `tfsdk:"operator"`
}

type SpansFieldModel struct {
	Type  types.String `tfsdk:"type"`
	Value types.String `tfsdk:"value"`
}

type DataTableSpansQueryGroupingModel struct {
	GroupBy      types.List `tfsdk:"group_by"`
	Aggregations types.List `tfsdk:"aggregations"`
}

type GaugeModel struct {
	Query        *GaugeQueryModel `tfsdk:"query"`
	Min          types.Float64    `tfsdk:"min"`
	Max          types.Float64    `tfsdk:"max"`
	ShowInnerArc types.Bool       `tfsdk:"show_inner_arc"`
	ShowOuterArc types.Bool       `tfsdk:"show_outer_arc"`
	Unit         types.String     `tfsdk:"unit"`
	Thresholds   types.List       `tfsdk:"thresholds"`
}

type GaugeQueryModel struct {
	Logs    *GaugeQueryLogsModel    `tfsdk:"logs"`
	Metrics *GaugeQueryMetricsModel `tfsdk:"metrics"`
	Spans   *GaugeQuerySpansModel   `tfsdk:"spans"`
}

type GaugeQueryLogsModel struct {
	LuceneQuery     types.String      `tfsdk:"lucene_query"`
	LogsAggregation *AggregationModel `tfsdk:"logs_aggregation"`
	Aggregation     types.String      `tfsdk:"aggregation"`
	Filters         types.List        `tfsdk:"filters"`
}

type GaugeQueryMetricsModel struct {
	PromqlQuery types.String `tfsdk:"promql_query"`
	Aggregation types.String `tfsdk:"aggregation"`
	Filters     types.List   `tfsdk:"filters"`
}

type GaugeQuerySpansModel struct {
	LuceneQuery      types.String           `tfsdk:"lucene_query"`
	SpansAggregation *SpansAggregationModel `tfsdk:"spans_aggregation"`
	Aggregation      types.String           `tfsdk:"aggregation"`
	Filters          types.List             `tfsdk:"filters"`
}

type GaugeThreshold struct {
	From  types.Float64 `tfsdk:"from"`
	Color types.String  `tfsdk:"color"`
}

type PieChartModel struct {
	Query              *PieChartQueryModel           `tfsdk:"query"`
	MaxSlicesPerChart  types.Int64                   `tfsdk:"max_slices_per_chart"`
	MinSlicePercentage types.Int64                   `tfsdk:"min_slice_percentage"`
	StackDefinition    *PieChartStackDefinitionModel `tfsdk:"stack_definition"`
	LabelDefinition    *LabelDefinitionModel         `tfsdk:"label_definition"`
	ShowLegend         types.Bool                    `tfsdk:"show_legend"`
	GroupNameTemplate  types.String                  `tfsdk:"group_name_template"`
	Unit               types.String                  `tfsdk:"unit"`
}

type PieChartStackDefinitionModel struct {
	MaxSlicesPerStack types.Int64  `tfsdk:"max_slices_per_stack"`
	StackNameTemplate types.String `tfsdk:"stack_name_template"`
}

type PieChartQueryModel struct {
	Logs    *PieChartQueryLogsModel    `tfsdk:"logs"`
	Metrics *PieChartQueryMetricsModel `tfsdk:"metrics"`
	Spans   *PieChartQuerySpansModel   `tfsdk:"spans"`
}

type PieChartQueryLogsModel struct {
	LuceneQuery      types.String      `tfsdk:"lucene_query"`
	Aggregation      *AggregationModel `tfsdk:"aggregation"`
	Filters          types.List        `tfsdk:"filters"`
	GroupNames       types.List        `tfsdk:"group_names"`
	StackedGroupName types.String      `tfsdk:"stacked_group_name"`
}

type PieChartQueryMetricsModel struct {
	PromqlQuery      types.String `tfsdk:"promql_query"`
	Filters          types.List   `tfsdk:"filters"`
	GroupNames       types.List   `tfsdk:"group_names"`
	StackedGroupName types.String `tfsdk:"stacked_group_name"`
}

type PieChartQuerySpansModel struct {
	LuceneQuery      types.String           `tfsdk:"lucene_query"`
	Aggregation      *SpansAggregationModel `tfsdk:"aggregation"`
	Filters          types.List             `tfsdk:"filters"`
	GroupNames       types.List             `tfsdk:"group_names"`
	StackedGroupName *SpansFieldModel       `tfsdk:"stacked_group_name"`
}

type LabelDefinitionModel struct {
	LabelSource    types.String `tfsdk:"label_source"`
	IsVisible      types.Bool   `tfsdk:"is_visible"`
	ShowName       types.Bool   `tfsdk:"show_name"`
	ShowValue      types.Bool   `tfsdk:"show_value"`
	ShowPercentage types.Bool   `tfsdk:"show_percentage"`
}

type BarChartModel struct {
	Query             *BarChartQueryModel           `tfsdk:"query"`
	MaxBarsPerChart   types.Int64                   `tfsdk:"max_bars_per_chart"`
	GroupNameTemplate types.String                  `tfsdk:"group_name_template"`
	StackDefinition   *BarChartStackDefinitionModel `tfsdk:"stack_definition"`
	ScaleType         types.String                  `tfsdk:"scale_type"`
	ColorsBy          types.String                  `tfsdk:"colors_by"`
	XAxis             *BarChartXAxisModel           `tfsdk:"xaxis"`
	Unit              types.String                  `tfsdk:"unit"`
}

type BarChartQueryModel struct {
	Logs    *BarChartQueryLogsModel    `tfsdk:"logs"`
	Metrics *BarChartQueryMetricsModel `tfsdk:"metrics"`
	Spans   *BarChartQuerySpansModel   `tfsdk:"spans"`
}

type BarChartQueryLogsModel struct {
	LuceneQuery      types.String      `tfsdk:"lucene_query"`
	Aggregation      *AggregationModel `tfsdk:"logs_aggregation"`
	Filters          types.List        `tfsdk:"filters"`
	GroupNames       types.List        `tfsdk:"group_names"`
	StackedGroupName types.String      `tfsdk:"stacked_group_name"`
}

type BarChartQueryMetricsModel struct {
	PromqlQuery      types.String `tfsdk:"promql_query"`
	Filters          types.List   `tfsdk:"filters"`
	GroupNames       types.List   `tfsdk:"group_names"`
	StackedGroupName types.String `tfsdk:"stacked_group_name"`
}

type BarChartQuerySpansModel struct {
	LuceneQuery      types.String           `tfsdk:"lucene_query"`
	Aggregation      *SpansAggregationModel `tfsdk:"aggregation"`
	Filters          types.List             `tfsdk:"filters"`
	GroupNames       types.List             `tfsdk:"group_names"`
	StackedGroupName *SpansFieldModel       `tfsdk:"stacked_group_name"`
}

type DataTableSpansAggregationModel struct {
	ID          types.String           `json:"id"`
	Name        types.String           `json:"name"`
	IsVisible   types.Bool             `json:"is_visible"`
	Aggregation *SpansAggregationModel `json:"aggregation"`
}

type BarChartStackDefinitionModel struct {
	MaxSlicesPerBar   types.Int64  `tfsdk:"max_slices_per_bar"`
	StackNameTemplate types.String `tfsdk:"stack_name_template"`
}

type BarChartXAxisModel struct {
	Type             types.String `tfsdk:"type"`
	Interval         types.String `tfsdk:"interval"`
	BucketsPresented types.Int64  `tfsdk:"buckets_presented"`
}

type DashboardVariableModel struct {
	Name        types.String                      `tfsdk:"name"`
	Definition  *DashboardVariableDefinitionModel `tfsdk:"definition"`
	DisplayName types.String                      `tfsdk:"display_name"`
}

type MetricMultiSelectSourceModel struct {
	Name  types.String `tfsdk:"name"`
	Label types.String `tfsdk:"label"`
}

type DashboardVariableDefinitionModel struct {
	ConstantValue types.String              `tfsdk:"constant_value"`
	MultiSelect   *VariableMultiSelectModel `tfsdk:"multi_select"`
}

type VariableMultiSelectModel struct {
	SelectedValues       types.List                      `tfsdk:"selected_values"`
	ValuesOrderDirection types.String                    `tfsdk:"values_order_direction"`
	Source               *VariableMultiSelectSourceModel `tfsdk:"source"`
}

type VariableMultiSelectSourceModel struct {
	LogPath      types.String                  `tfsdk:"log_path"`
	Metric       *MetricMultiSelectSourceModel `tfsdk:"metric"`
	ConstantList types.List                    `tfsdk:"constant_list"`
	SpanField    *SpansFieldModel              `tfsdk:"span_field"`
}

type DashboardFilterModel struct {
	Logs      *FilterSourceLogsModel    `tfsdk:"logs"`
	Metrics   *FilterSourceMetricsModel `tfsdk:"metrics"`
	Spans     *FilterSourceSpansModel   `tfsdk:"spans"`
	Enabled   types.Bool                `tfsdk:"enabled"`
	Collapsed types.Bool                `tfsdk:"collapsed"`
}

type FilterSourceLogsModel struct {
	Field    types.String         `tfsdk:"field"`
	Operator *FilterOperatorModel `tfsdk:"operator"`
}

type FilterSourceMetricsModel struct {
	MetricName  types.String         `tfsdk:"name"`
	MetricLabel types.String         `tfsdk:"label"`
	Operator    *FilterOperatorModel `tfsdk:"operator"`
}

type FilterSourceSpansModel struct {
	Field    *SpansFieldModel     `tfsdk:"field"`
	Operator *FilterOperatorModel `tfsdk:"operator"`
}

type DashboardTimeFrameModel struct {
	Absolute *DashboardTimeFrameAbsoluteModel `tfsdk:"absolute"`
	Relative *DashboardTimeFrameRelativeModel `tfsdk:"relative"`
}

type DashboardTimeFrameAbsoluteModel struct {
	From types.String `tfsdk:"from"`
	To   types.String `tfsdk:"to"`
}

type DashboardTimeFrameRelativeModel struct {
	Duration types.String `tfsdk:"duration"`
}
