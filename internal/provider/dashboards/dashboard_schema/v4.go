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

package dashboard_schema

import (
	"fmt"
	"strings"

	dashboardwidgets "github.com/coralogix/terraform-provider-coralogix/internal/provider/dashboards/dashboard_widgets"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/float64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func V4() schema.Schema {
	attributes := dashboardSchemaAttributesV4()

	return schema.Schema{
		Version:             4,
		Attributes:          attributes,
		MarkdownDescription: "Coralogix Custom Dashboard. For more info please review - https://coralogix.com/docs/user-guides/custom-dashboards/introduction/.",
	}
}

func dashboardSchemaAttributesV4() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed: true,
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
			MarkdownDescription: "Unique identifier for the dashboard.",
		},
		"name": schema.StringAttribute{
			Optional:            true,
			MarkdownDescription: "Display name of the dashboard.",
		},
		"description": schema.StringAttribute{
			Optional:            true,
			MarkdownDescription: "Brief description or summary of the dashboard's purpose or content.",
		},
		"layout": schema.SingleNestedAttribute{
			Optional: true,
			Attributes: map[string]schema.Attribute{
				"sections": schema.ListNestedAttribute{
					NestedObject: schema.NestedAttributeObject{
						Attributes: map[string]schema.Attribute{
							"id": schema.StringAttribute{
								Optional: true,
								Computed: true,
								PlanModifiers: []planmodifier.String{
									stringplanmodifier.UseNonNullStateForUnknown(),
								},
							},
							"rows": schema.ListNestedAttribute{
								NestedObject: schema.NestedAttributeObject{
									Attributes: map[string]schema.Attribute{
										"id": schema.StringAttribute{
											Optional: true,
											Computed: true,
											PlanModifiers: []planmodifier.String{
												stringplanmodifier.UseNonNullStateForUnknown(),
											},
										},
										"height": schema.Int64Attribute{
											Required: true,
											Validators: []validator.Int64{
												int64validator.AtLeast(1),
											},
											MarkdownDescription: "The height of the row.",
										},
										"widgets": schema.ListNestedAttribute{
											Optional: true,
											NestedObject: schema.NestedAttributeObject{
												Attributes: map[string]schema.Attribute{
													"id": schema.StringAttribute{
														Optional: true,
														Computed: true,
														PlanModifiers: []planmodifier.String{
															stringplanmodifier.UseNonNullStateForUnknown(),
														},
													},
													"title": schema.StringAttribute{
														Optional:            true,
														MarkdownDescription: "Widget title. Required for all inline widgets except markdown, where it is optional.",
													},
													"description": schema.StringAttribute{
														Optional:            true,
														MarkdownDescription: "Widget description.",
													},
													"highlighted": schema.BoolAttribute{
														// The API returns a value for every widget, so the
														// attribute has to be computed: plain optional would
														// fail the apply with "was null, but now false". A
														// computed attribute that is null in configuration
														// plans as "known after apply" on every run, so the
														// prior value is kept. Write false to stop
														// highlighting a widget; deleting the line leaves the
														// value as it was.
														Optional: true,
														Computed: true,
														PlanModifiers: []planmodifier.Bool{
															boolplanmodifier.UseNonNullStateForUnknown(),
														},
														MarkdownDescription: "Marks the widget as highlighted for every user of the dashboard. Set `false` to stop highlighting it: the API returns a value for every widget, so deleting the line keeps the last value. The API rejects it on a widget that only holds a `reference`.",
													},
													"definition": schema.SingleNestedAttribute{
														Optional: true,
														Attributes: map[string]schema.Attribute{
															"line_chart": dashboardwidgets.LineChartSchema(),
															"hexagon":    dashboardwidgets.HexagonSchemaV4(),
															"data_table": dashboardwidgets.DataTableSchema(),
															"dynamic":    dashboardwidgets.DynamicSchema(),
															"gauge": schema.SingleNestedAttribute{
																Attributes: map[string]schema.Attribute{
																	"query": schema.SingleNestedAttribute{
																		Attributes: map[string]schema.Attribute{
																			"logs": schema.SingleNestedAttribute{
																				Attributes: map[string]schema.Attribute{
																					"lucene_query": schema.StringAttribute{
																						Optional: true,
																					},
																					"filters":          dashboardwidgets.LogsFiltersSchema(),
																					"logs_aggregation": dashboardwidgets.LogsAggregationSchema(),
																					"time_frame":       dashboardwidgets.TimeFrameSchema(),
																					"group_by":         dashboardwidgets.ObservationFieldsSchema(),
																				},
																				Optional: true,
																			},
																			"metrics": schema.SingleNestedAttribute{
																				Attributes: map[string]schema.Attribute{
																					"promql_query": schema.StringAttribute{
																						Required: true,
																					},
																					"aggregation": schema.StringAttribute{
																						Validators: []validator.String{
																							stringvalidator.OneOf(dashboardwidgets.DashboardValidGaugeAggregations...),
																						},
																						MarkdownDescription: fmt.Sprintf("The type of aggregation. Can be one of %q.", dashboardwidgets.DashboardValidGaugeAggregations),
																						Optional:            true,
																						Computed:            true,
																						Default:             stringdefault.StaticString(utils.UNSPECIFIED),
																					},
																					"filters":           dashboardwidgets.MetricFiltersSchema(),
																					"time_frame":        dashboardwidgets.TimeFrameSchema(),
																					"editor_mode":       dashboardwidgets.MetricsEditorModeSchema(),
																					"promql_query_type": dashboardwidgets.PromQLQueryTypeSchema(),
																				},
																				Optional: true,
																			},
																			"spans": schema.SingleNestedAttribute{
																				Attributes: map[string]schema.Attribute{
																					"lucene_query": schema.StringAttribute{
																						Optional: true,
																					},
																					"spans_aggregation": dashboardwidgets.SpansAggregationSchema(),
																					"filters":           dashboardwidgets.SpansObservationFiltersSchema(),
																					"time_frame":        dashboardwidgets.TimeFrameSchema(),
																					"group_by":          dashboardwidgets.NonEmptySpansFieldsSchema(),
																					"group_bys":         dashboardwidgets.SpanObservationFieldsSchema(),
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
																							Attributes: dashboardwidgets.FiltersSourceSchema(),
																							Validators: []validator.Object{
																								dashboardwidgets.ExactlyOneOfChildren("logs", "metrics", "spans"),
																							},
																						},
																						Optional: true,
																					},
																					"time_frame": dashboardwidgets.TimeFrameSchema(),
																				},
																				Optional: true,
																			},
																		},
																		Required: true,
																		Validators: []validator.Object{
																			dashboardwidgets.ExactlyOneOfChildren("logs", "metrics", "spans", "data_prime"),
																		},
																	},
																	// The proto declares min and max as wrapper values, so
																	// an omitted one is absent and not zero. A static default
																	// would send a bound the dashboard never had, so there is
																	// none. Computed keeps the bound the API returns for a
																	// dashboard created elsewhere. The plan modifier copies
																	// the prior state, and it has to be UseStateForUnknown
																	// and not the non-null variant: the API returns no bound
																	// at all for a gauge built in the Coralogix UI, so the
																	// prior state is null and the non-null variant would plan
																	// "known after apply" on every run.
																	"min": schema.Float64Attribute{
																		Optional: true,
																		Computed: true,
																		PlanModifiers: []planmodifier.Float64{
																			float64planmodifier.UseStateForUnknown(),
																		},
																	},
																	"max": schema.Float64Attribute{
																		Optional: true,
																		Computed: true,
																		PlanModifiers: []planmodifier.Float64{
																			float64planmodifier.UseStateForUnknown(),
																		},
																	},
																	"show_inner_arc": schema.BoolAttribute{
																		Optional: true,
																		Computed: true,
																		Default:  booldefault.StaticBool(false),
																	},
																	"show_outer_arc": schema.BoolAttribute{
																		Optional: true,
																		Computed: true,
																		Default:  booldefault.StaticBool(true),
																	},
																	"unit": schema.StringAttribute{
																		Required: true,
																		Validators: []validator.String{
																			stringvalidator.OneOf(dashboardwidgets.DashboardValidGaugeUnits...),
																		},
																		MarkdownDescription: fmt.Sprintf("The unit of the gauge. Can be one of %q.", dashboardwidgets.DashboardValidGaugeUnits),
																	},
																	"thresholds": schema.ListNestedAttribute{
																		NestedObject: schema.NestedAttributeObject{
																			Attributes: map[string]schema.Attribute{
																				"color": schema.StringAttribute{
																					Optional: true,
																				},
																				"from": schema.Float64Attribute{
																					Optional: true,
																				},
																				"label": schema.StringAttribute{
																					Optional: true,
																				},
																			},
																		},
																		Optional: true,
																	},
																	"data_mode_type": schema.StringAttribute{
																		Optional: true,
																		Computed: true,
																		Default:  stringdefault.StaticString(utils.UNSPECIFIED),
																		Validators: []validator.String{
																			stringvalidator.OneOf(dashboardwidgets.DashboardValidDataModeTypes...),
																		},
																		MarkdownDescription: fmt.Sprintf("The data mode type. Can be one of %q.", dashboardwidgets.DashboardValidDataModeTypes),
																	},
																	"threshold_by": schema.StringAttribute{
																		Optional: true,
																		Computed: true,
																		Default:  stringdefault.StaticString(utils.UNSPECIFIED),
																		Validators: []validator.String{
																			stringvalidator.OneOf(dashboardwidgets.DashboardValidGaugeThresholdBy...),
																		},
																		MarkdownDescription: fmt.Sprintf("The threshold by. Can be one of %q.", dashboardwidgets.DashboardValidGaugeThresholdBy),
																	},
																	"threshold_type": schema.StringAttribute{
																		Optional: true,
																		Computed: true,
																		Default:  stringdefault.StaticString(utils.UNSPECIFIED),
																		Validators: []validator.String{
																			stringvalidator.OneOf(dashboardwidgets.DashboardValidThresholdTypes...),
																		},
																		MarkdownDescription: fmt.Sprintf("The threshold type. Can be one of %q.", dashboardwidgets.DashboardValidThresholdTypes),
																	},
																	"display_series_name": schema.BoolAttribute{
																		Optional: true,
																		Computed: true,
																		Default:  booldefault.StaticBool(true),
																	},
																	"decimal": dashboardwidgets.DecimalSchema(),
																	"arc_display": schema.SingleNestedAttribute{
																		Optional: true,
																		Attributes: map[string]schema.Attribute{
																			"threshold_arc": schema.BoolAttribute{Optional: true},
																			"value_arc":     schema.BoolAttribute{Optional: true},
																		},
																	},
																	"decimal_precision": dashboardwidgets.DecimalPrecisionSchema(),
																	"custom_unit":       dashboardwidgets.CustomUnitSchema(),
																	"legend":            dashboardwidgets.LegendSchema(),
																	"legend_by":         dashboardwidgets.LegendBySchema(),
																	"show_min_max": schema.BoolAttribute{
																		Optional:            true,
																		MarkdownDescription: "Whether to display the min and max range values on the gauge.",
																	},
																},
																Validators: []validator.Object{
																	objectvalidator.AlsoRequires(
																		path.MatchRelative().AtParent().AtParent().AtName("title"),
																	),
																},
																Optional: true,
															},
															"pie_chart": schema.SingleNestedAttribute{
																Attributes: map[string]schema.Attribute{
																	"query": schema.SingleNestedAttribute{
																		Attributes: map[string]schema.Attribute{
																			"logs": schema.SingleNestedAttribute{
																				Attributes: map[string]schema.Attribute{
																					"lucene_query": schema.StringAttribute{
																						Optional: true,
																					},
																					"aggregation": dashboardwidgets.LogsAggregationSchema(),
																					"filters":     dashboardwidgets.LogsFiltersSchema(),
																					"group_names": schema.ListAttribute{
																						ElementType: types.StringType,
																						Optional:    true,
																						Validators: []validator.List{
																							listvalidator.SizeAtLeast(1),
																						},
																					},
																					"stacked_group_name": schema.StringAttribute{
																						Optional: true,
																					},
																					"group_names_fields": schema.ListNestedAttribute{
																						NestedObject: schema.NestedAttributeObject{
																							Attributes: dashboardwidgets.ObservationFieldSchema(),
																						},
																						Optional: true,
																					},
																					"stacked_group_name_field": schema.SingleNestedAttribute{
																						Attributes: dashboardwidgets.ObservationFieldSchema(),
																						Optional:   true,
																					},
																					"time_frame": dashboardwidgets.TimeFrameSchema(),
																				},
																				Optional: true,
																			},
																			"spans": schema.SingleNestedAttribute{
																				Attributes: map[string]schema.Attribute{
																					"lucene_query": schema.StringAttribute{
																						Optional: true,
																					},
																					"aggregation":              dashboardwidgets.SpansAggregationSchema(),
																					"filters":                  dashboardwidgets.SpansObservationFiltersSchema(),
																					"group_names":              dashboardwidgets.SpansFieldsSchema(),
																					"stacked_group_name":       dashboardwidgets.SpansFieldSchema(),
																					"time_frame":               dashboardwidgets.TimeFrameSchema(),
																					"group_names_fields":       dashboardwidgets.SpanObservationFieldsSchema(),
																					"stacked_group_name_field": dashboardwidgets.SpanObservationFieldSchema(),
																				},
																				Optional: true,
																			},
																			"metrics": schema.SingleNestedAttribute{
																				Attributes: map[string]schema.Attribute{
																					"promql_query": schema.StringAttribute{
																						Required: true,
																					},
																					"filters": dashboardwidgets.MetricFiltersSchema(),
																					"group_names": schema.ListAttribute{
																						ElementType: types.StringType,
																						Optional:    true,
																					},
																					"stacked_group_name": schema.StringAttribute{
																						Optional: true,
																					},
																					"time_frame":        dashboardwidgets.TimeFrameSchema(),
																					"aggregation":       dashboardwidgets.CommonAggregationSchema(),
																					"editor_mode":       dashboardwidgets.MetricsEditorModeSchema(),
																					"promql_query_type": dashboardwidgets.PromQLQueryTypeSchema(),
																				},
																				Optional: true,
																			},
																			"data_prime": schema.SingleNestedAttribute{
																				Attributes: map[string]schema.Attribute{
																					"query": schema.StringAttribute{
																						Required: true,
																					},
																					"filters": schema.ListNestedAttribute{
																						NestedObject: schema.NestedAttributeObject{
																							Attributes: dashboardwidgets.FiltersSourceSchema(),
																							Validators: []validator.Object{
																								dashboardwidgets.ExactlyOneOfChildren("logs", "metrics", "spans"),
																							},
																						},
																						Optional: true,
																					},
																					"group_names": schema.ListAttribute{
																						ElementType: types.StringType,
																						Optional:    true,
																					},
																					"stacked_group_name": schema.StringAttribute{
																						Optional: true,
																					},
																					"time_frame": dashboardwidgets.TimeFrameSchema(),
																				},
																				Optional: true,
																			},
																		},
																		Required: true,
																		Validators: []validator.Object{
																			dashboardwidgets.ExactlyOneOfChildren("logs", "metrics", "spans", "data_prime"),
																		},
																	},
																	"max_slices_per_chart": schema.Int64Attribute{
																		Optional: true,
																	},
																	"min_slice_percentage": schema.Int64Attribute{
																		Optional: true,
																	},
																	"stack_definition": schema.SingleNestedAttribute{
																		Attributes: map[string]schema.Attribute{
																			"max_slices_per_stack": schema.Int64Attribute{
																				Optional: true,
																			},
																			"stack_name_template": schema.StringAttribute{
																				Optional: true,
																			},
																		},
																		Optional: true,
																	},
																	"label_definition": schema.SingleNestedAttribute{
																		Attributes: map[string]schema.Attribute{
																			"label_source": schema.StringAttribute{
																				Optional: true,
																				Computed: true,
																				Default:  stringdefault.StaticString(utils.UNSPECIFIED),
																				Validators: []validator.String{
																					stringvalidator.OneOf(dashboardwidgets.DashboardValidPieChartLabelSources...),
																				},
																				MarkdownDescription: fmt.Sprintf("The source of the label. Valid values are: %s", strings.Join(dashboardwidgets.DashboardValidPieChartLabelSources, ", ")),
																			},
																			"is_visible": schema.BoolAttribute{
																				Optional: true,
																				Computed: true,
																				Default:  booldefault.StaticBool(true),
																			},
																			"show_name": schema.BoolAttribute{
																				Optional: true,
																				Computed: true,
																				Default:  booldefault.StaticBool(true),
																			},
																			"show_value": schema.BoolAttribute{
																				Optional: true,
																				Computed: true,
																				Default:  booldefault.StaticBool(true),
																			},
																			"show_percentage": schema.BoolAttribute{
																				Optional: true,
																				Computed: true,
																				Default:  booldefault.StaticBool(true),
																			},
																		},
																		Required: true,
																	},
																	"show_legend": schema.BoolAttribute{
																		Optional: true,
																		Computed: true,
																		Default:  booldefault.StaticBool(true),
																	},
																	"group_name_template": schema.StringAttribute{
																		Optional: true,
																	},
																	"unit": schema.StringAttribute{
																		Optional: true,
																		Computed: true,
																		Default:  stringdefault.StaticString(utils.UNSPECIFIED),
																	},
																	"color_scheme": schema.StringAttribute{
																		Optional: true,
																		Validators: []validator.String{
																			stringvalidator.OneOf(dashboardwidgets.DashboardValidColorSchemes...),
																		},
																		Description: fmt.Sprintf("The color scheme. Can be one of %s.", strings.Join(dashboardwidgets.DashboardValidColorSchemes, ", ")),
																	},
																	"hash_colors":       dashboardwidgets.HashColorsSchema(),
																	"custom_unit":       dashboardwidgets.CustomUnitSchema(),
																	"decimal":           dashboardwidgets.DecimalSchema(),
																	"decimal_precision": dashboardwidgets.DecimalPrecisionSchema(),
																	"legend":            dashboardwidgets.LegendSchema(),
																	"show_total": schema.BoolAttribute{
																		Optional:            true,
																		MarkdownDescription: "When true, the total of all slices is shown as the chart title.",
																	},
																	"data_mode_type": schema.StringAttribute{
																		Optional: true,
																		Computed: true,
																		Default:  stringdefault.StaticString(utils.UNSPECIFIED),
																		Validators: []validator.String{
																			stringvalidator.OneOf(dashboardwidgets.DashboardValidDataModeTypes...),
																		},
																	},
																},
																Optional: true,
															},
															"bar_chart": schema.SingleNestedAttribute{
																Attributes: map[string]schema.Attribute{
																	"query": schema.SingleNestedAttribute{
																		Attributes: map[string]schema.Attribute{
																			"logs": schema.SingleNestedAttribute{
																				Attributes: map[string]schema.Attribute{
																					"lucene_query": schema.StringAttribute{
																						Optional: true,
																					},
																					"aggregation": dashboardwidgets.LogsAggregationSchema(),
																					"filters":     dashboardwidgets.LogsFiltersSchema(),
																					"group_names": schema.ListAttribute{
																						ElementType: types.StringType,
																						Optional:    true,
																					},
																					"stacked_group_name": schema.StringAttribute{
																						Optional: true,
																					},
																					"group_names_fields": schema.ListNestedAttribute{
																						NestedObject: schema.NestedAttributeObject{
																							Attributes: dashboardwidgets.ObservationFieldSchema(),
																						},
																						Optional: true,
																					},
																					"stacked_group_name_field": schema.SingleNestedAttribute{
																						Attributes: dashboardwidgets.ObservationFieldSchema(),
																						Optional:   true,
																					},
																					"time_frame": dashboardwidgets.TimeFrameSchema(),
																				},
																				Optional: true,
																			},
																			"metrics": schema.SingleNestedAttribute{
																				Attributes: map[string]schema.Attribute{
																					"promql_query": schema.StringAttribute{
																						Required: true,
																					},
																					"filters": dashboardwidgets.MetricFiltersSchema(),
																					"group_names": schema.ListAttribute{
																						ElementType: types.StringType,
																						Optional:    true,
																					},
																					"stacked_group_name": schema.StringAttribute{
																						Optional: true,
																					},
																					"time_frame":        dashboardwidgets.TimeFrameSchema(),
																					"aggregation":       dashboardwidgets.CommonAggregationSchema(),
																					"editor_mode":       dashboardwidgets.MetricsEditorModeSchema(),
																					"promql_query_type": dashboardwidgets.PromQLQueryTypeSchema(),
																				},
																				Optional: true,
																			},
																			"spans": schema.SingleNestedAttribute{
																				Attributes: map[string]schema.Attribute{
																					"lucene_query": schema.StringAttribute{
																						Optional: true,
																					},
																					"aggregation":              dashboardwidgets.SpansAggregationSchema(),
																					"filters":                  dashboardwidgets.SpansObservationFiltersSchema(),
																					"group_names":              dashboardwidgets.SpansFieldsSchema(),
																					"stacked_group_name":       dashboardwidgets.SpansFieldSchema(),
																					"time_frame":               dashboardwidgets.TimeFrameSchema(),
																					"group_names_fields":       dashboardwidgets.SpanObservationFieldsSchema(),
																					"stacked_group_name_field": dashboardwidgets.SpanObservationFieldSchema(),
																				},
																				Optional: true,
																			},
																			"data_prime": schema.SingleNestedAttribute{
																				Attributes: map[string]schema.Attribute{
																					"query": schema.StringAttribute{
																						Required: true,
																					},
																					"filters": schema.ListNestedAttribute{
																						NestedObject: schema.NestedAttributeObject{
																							Attributes: dashboardwidgets.FiltersSourceSchema(),
																							Validators: []validator.Object{
																								dashboardwidgets.ExactlyOneOfChildren("logs", "metrics", "spans"),
																							},
																						},
																						Optional: true,
																					},
																					"group_names": schema.ListAttribute{
																						ElementType: types.StringType,
																						Optional:    true,
																					},
																					"stacked_group_name": schema.StringAttribute{
																						Optional: true,
																					},
																					"time_frame": dashboardwidgets.TimeFrameSchema(),
																				},
																				Optional: true,
																			},
																		},
																		Optional: true,
																		Validators: []validator.Object{
																			dashboardwidgets.ExactlyOneOfChildren("logs", "metrics", "spans", "data_prime"),
																		},
																	},
																	"max_bars_per_chart": schema.Int64Attribute{
																		Optional: true,
																	},
																	"group_name_template": schema.StringAttribute{
																		Optional: true,
																	},
																	"stack_definition": schema.SingleNestedAttribute{
																		Optional: true,
																		Attributes: map[string]schema.Attribute{
																			"max_slices_per_bar": schema.Int64Attribute{
																				Optional: true,
																			},
																			"stack_name_template": schema.StringAttribute{
																				Optional: true,
																			},
																		},
																	},
																	"scale_type": schema.StringAttribute{
																		Optional: true,
																		Computed: true,
																		Default:  stringdefault.StaticString(utils.UNSPECIFIED),
																	},
																	"colors_by": schema.StringAttribute{
																		Optional: true,
																		Validators: []validator.String{
																			stringvalidator.OneOf(dashboardwidgets.DashboardValidColorsBy...),
																		},
																		MarkdownDescription: fmt.Sprintf("Which dimension the bar colors follow. Can be one of %s.", strings.Join(dashboardwidgets.DashboardValidColorsBy, ", ")),
																	},
																	"xaxis": schema.SingleNestedAttribute{
																		Optional: true,
																		Attributes: map[string]schema.Attribute{
																			"time": schema.SingleNestedAttribute{
																				Attributes: map[string]schema.Attribute{
																					"interval": schema.StringAttribute{
																						Required: true,
																						Validators: []validator.String{
																							intervalValidator{},
																						},
																						MarkdownDescription: "The time interval to use for the x-axis. Valid values are in duration format, for example `1m0s` or `1h0m0s` (currently leading zeros should be added).",
																					},
																					"buckets_presented": schema.Int64Attribute{
																						Optional: true,
																					},
																				},
																				Optional:            true,
																				DeprecationMessage:  "The Coralogix UI writes `time_buckets` and rewrites this block to it when the dashboard is saved, so a configuration using `time` can stop matching the dashboard it manages. Use `time_buckets` instead.",
																				MarkdownDescription: "Deprecated: use `time_buckets` instead. The Coralogix UI no longer writes this block and rewrites it to `time_buckets` when the dashboard is saved. Retained at full fidelity for dashboards that still set it.",
																			},
																			"value": schema.SingleNestedAttribute{
																				Attributes: map[string]schema.Attribute{},
																				Optional:   true,
																			},
																			"time_buckets": dashboardwidgets.IntervalResolutionSchema(),
																		},
																		Validators: []validator.Object{
																			dashboardwidgets.ExactlyOneOfChildren("time", "value", "time_buckets"),
																		},
																	},
																	"unit": schema.StringAttribute{
																		Optional: true,
																		Computed: true,
																		Default:  stringdefault.StaticString(utils.UNSPECIFIED),
																		Validators: []validator.String{
																			stringvalidator.OneOf(dashboardwidgets.DashboardValidUnits...),
																		},
																		MarkdownDescription: fmt.Sprintf("The unit of the chart. Can be one of %s.", strings.Join(dashboardwidgets.DashboardValidUnits, ", ")),
																	},
																	"sort_by": schema.StringAttribute{
																		Optional: true,
																		Computed: true,
																		Default:  stringdefault.StaticString(utils.UNSPECIFIED),
																		Validators: []validator.String{
																			stringvalidator.OneOf(dashboardwidgets.DashboardValidSortBy...),
																		},
																		Description: fmt.Sprintf("The field to sort by. Can be one of %s.", strings.Join(dashboardwidgets.DashboardValidSortBy, ", ")),
																	},
																	"color_scheme": schema.StringAttribute{
																		Optional: true,
																		Validators: []validator.String{
																			stringvalidator.OneOf(dashboardwidgets.DashboardValidColorSchemes...),
																		},
																		Description: fmt.Sprintf("The color scheme. Can be one of %s.", strings.Join(dashboardwidgets.DashboardValidColorSchemes, ", ")),
																	},
																	"hash_colors":        dashboardwidgets.HashColorsSchema(),
																	"bar_value_display":  dashboardwidgets.BarValueDisplaySchema(),
																	"custom_unit":        dashboardwidgets.CustomUnitSchema(),
																	"decimal":            dashboardwidgets.DecimalSchema(),
																	"decimal_precision":  dashboardwidgets.DecimalPrecisionSchema(),
																	"legend":             dashboardwidgets.LegendSchema(),
																	"x_axis_time_format": dashboardwidgets.XAxisTimeFormatSchema(),
																	"y_axis_max":         dashboardwidgets.YAxisMaxSchema(),
																	"y_axis_min":         dashboardwidgets.YAxisMinSchema(),
																	"data_mode_type": schema.StringAttribute{
																		Optional: true,
																		Computed: true,
																		Default:  stringdefault.StaticString(utils.UNSPECIFIED),
																		Validators: []validator.String{
																			stringvalidator.OneOf(dashboardwidgets.DashboardValidDataModeTypes...),
																		},
																	},
																},
																Validators: []validator.Object{
																	objectvalidator.AlsoRequires(
																		path.MatchRelative().AtParent().AtParent().AtName("title"),
																	),
																},
																Optional: true,
															},
															"horizontal_bar_chart": schema.SingleNestedAttribute{
																Attributes: map[string]schema.Attribute{
																	"query": schema.SingleNestedAttribute{
																		Attributes: map[string]schema.Attribute{
																			"logs": schema.SingleNestedAttribute{
																				Attributes: map[string]schema.Attribute{
																					"lucene_query": schema.StringAttribute{
																						Optional: true,
																					},
																					"aggregation": dashboardwidgets.LogsAggregationSchema(),
																					"filters":     dashboardwidgets.LogsFiltersSchema(),
																					"group_names": schema.ListAttribute{
																						ElementType: types.StringType,
																						Optional:    true,
																						Validators: []validator.List{
																							listvalidator.SizeAtLeast(1),
																						},
																					},
																					"stacked_group_name": schema.StringAttribute{
																						Optional: true,
																					},
																					"group_names_fields": schema.ListNestedAttribute{
																						NestedObject: schema.NestedAttributeObject{
																							Attributes: dashboardwidgets.ObservationFieldSchema(),
																						},
																						Optional: true,
																					},
																					"stacked_group_name_field": schema.SingleNestedAttribute{
																						Attributes: dashboardwidgets.ObservationFieldSchema(),
																						Optional:   true,
																					},
																					"time_frame": dashboardwidgets.TimeFrameSchema(),
																				},
																				Optional: true,
																			},
																			"metrics": schema.SingleNestedAttribute{
																				Attributes: map[string]schema.Attribute{
																					"promql_query": schema.StringAttribute{
																						Required: true,
																					},
																					"filters": dashboardwidgets.MetricFiltersSchema(),
																					"group_names": schema.ListAttribute{
																						ElementType: types.StringType,
																						Optional:    true,
																					},
																					"stacked_group_name": schema.StringAttribute{
																						Optional: true,
																					},
																					"time_frame":        dashboardwidgets.TimeFrameSchema(),
																					"aggregation":       dashboardwidgets.CommonAggregationSchema(),
																					"editor_mode":       dashboardwidgets.MetricsEditorModeSchema(),
																					"promql_query_type": dashboardwidgets.PromQLQueryTypeSchema(),
																				},
																				Optional: true,
																			},
																			"spans": schema.SingleNestedAttribute{
																				Attributes: map[string]schema.Attribute{
																					"lucene_query": schema.StringAttribute{
																						Optional: true,
																					},
																					"aggregation":              dashboardwidgets.SpansAggregationSchema(),
																					"filters":                  dashboardwidgets.SpansObservationFiltersSchema(),
																					"group_names":              dashboardwidgets.SpansFieldsSchema(),
																					"stacked_group_name":       dashboardwidgets.SpansFieldSchema(),
																					"time_frame":               dashboardwidgets.TimeFrameSchema(),
																					"group_names_fields":       dashboardwidgets.SpanObservationFieldsSchema(),
																					"stacked_group_name_field": dashboardwidgets.SpanObservationFieldSchema(),
																				},
																				Optional: true,
																			},
																			"data_prime": schema.SingleNestedAttribute{
																				Attributes: map[string]schema.Attribute{
																					"query": schema.StringAttribute{
																						Required: true,
																					},
																					"filters": schema.ListNestedAttribute{
																						NestedObject: schema.NestedAttributeObject{
																							Attributes: dashboardwidgets.FiltersSourceSchema(),
																							Validators: []validator.Object{
																								dashboardwidgets.ExactlyOneOfChildren("logs", "metrics", "spans"),
																							},
																						},
																						Optional: true,
																					},
																					"group_names": schema.ListAttribute{
																						ElementType: types.StringType,
																						Optional:    true,
																					},
																					"stacked_group_name": schema.StringAttribute{
																						Optional: true,
																					},
																					"time_frame": dashboardwidgets.TimeFrameSchema(),
																				},
																				Optional: true,
																			},
																		},
																		Optional: true,
																		Validators: []validator.Object{
																			dashboardwidgets.ExactlyOneOfChildren("logs", "metrics", "spans", "data_prime"),
																		},
																	},
																	"max_bars_per_chart": schema.Int64Attribute{
																		Optional: true,
																	},
																	"group_name_template": schema.StringAttribute{
																		Optional: true,
																	},
																	"stack_definition": schema.SingleNestedAttribute{
																		Optional: true,
																		Attributes: map[string]schema.Attribute{
																			"max_slices_per_bar": schema.Int64Attribute{
																				Optional: true,
																			},
																			"stack_name_template": schema.StringAttribute{
																				Optional: true,
																			},
																		},
																	},
																	"scale_type": schema.StringAttribute{
																		Optional: true,
																		Computed: true,
																		Default:  stringdefault.StaticString(utils.UNSPECIFIED),
																	},
																	"colors_by": schema.StringAttribute{
																		Optional: true,
																		Validators: []validator.String{
																			stringvalidator.OneOf(dashboardwidgets.DashboardValidColorsBy...),
																		},
																		MarkdownDescription: fmt.Sprintf("Which dimension the bar colors follow. Can be one of %s.", strings.Join(dashboardwidgets.DashboardValidColorsBy, ", ")),
																	},
																	"unit": schema.StringAttribute{
																		Optional: true,
																		Computed: true,
																		Default:  stringdefault.StaticString(utils.UNSPECIFIED),
																		Validators: []validator.String{
																			stringvalidator.OneOf(dashboardwidgets.DashboardValidUnits...),
																		},
																		MarkdownDescription: fmt.Sprintf("The unit of the chart. Can be one of %s.", strings.Join(dashboardwidgets.DashboardValidUnits, ", ")),
																	},
																	"display_on_bar": schema.BoolAttribute{
																		Optional: true,
																		Computed: true,
																		Default:  booldefault.StaticBool(false),
																	},
																	"y_axis_view_by": schema.StringAttribute{
																		Optional: true,
																		Validators: []validator.String{
																			stringvalidator.OneOf("category", "value"),
																		},
																	},
																	"sort_by": schema.StringAttribute{
																		Optional: true,
																		Computed: true,
																		Default:  stringdefault.StaticString(utils.UNSPECIFIED),
																		Validators: []validator.String{
																			stringvalidator.OneOf(dashboardwidgets.DashboardValidSortBy...),
																		},
																	},
																	"color_scheme": schema.StringAttribute{
																		Optional: true,
																		Validators: []validator.String{
																			stringvalidator.OneOf(dashboardwidgets.DashboardValidColorSchemes...),
																		},
																		Description: fmt.Sprintf("The color scheme. Can be one of %s.", strings.Join(dashboardwidgets.DashboardValidColorSchemes, ", ")),
																	},
																	"hash_colors":       dashboardwidgets.HashColorsSchema(),
																	"custom_unit":       dashboardwidgets.CustomUnitSchema(),
																	"decimal":           dashboardwidgets.DecimalSchema(),
																	"decimal_precision": dashboardwidgets.DecimalPrecisionSchema(),
																	"legend":            dashboardwidgets.LegendSchema(),
																	"y_axis_max":        dashboardwidgets.YAxisMaxSchema(),
																	"y_axis_min":        dashboardwidgets.YAxisMinSchema(),
																	"data_mode_type": schema.StringAttribute{
																		Optional: true,
																		Computed: true,
																		Default:  stringdefault.StaticString(utils.UNSPECIFIED),
																		Validators: []validator.String{
																			stringvalidator.OneOf(dashboardwidgets.DashboardValidDataModeTypes...),
																		},
																	},
																},
																Validators: []validator.Object{
																	objectvalidator.AlsoRequires(
																		path.MatchRelative().AtParent().AtParent().AtName("title"),
																	),
																},
																Optional: true,
															},
															"markdown": schema.SingleNestedAttribute{
																Attributes: map[string]schema.Attribute{
																	"markdown_text": schema.StringAttribute{
																		Optional: true,
																	},
																	"tooltip_text": schema.StringAttribute{
																		Optional: true,
																	},
																},
																Optional: true,
															},
														},
														MarkdownDescription: fmt.Sprintf("Inline widget definition. Can contain one of %v. Exactly one of `definition` or `reference` must be set.", dashboardwidgets.SupportedWidgetTypes),
														Validators: []validator.Object{
															dashboardwidgets.SupportedWidgetsExactlyOneOfChildren(),
														},
													},
													"reference": schema.SingleNestedAttribute{
														Optional: true,
														Attributes: map[string]schema.Attribute{
															"dashboard_id": schema.StringAttribute{
																Required:            true,
																MarkdownDescription: "ID of the dashboard that owns the source widget.",
															},
															"widget_id": schema.StringAttribute{
																Required:            true,
																MarkdownDescription: "ID of the source widget within the referenced dashboard.",
															},
														},
														MarkdownDescription: "Reference to a widget on another dashboard. Exactly one of `definition` or `reference` must be set.",
													},
													"width": schema.Int64Attribute{
														Optional: true,
														Computed: true,
														PlanModifiers: []planmodifier.Int64{
															int64planmodifier.UseNonNullStateForUnknown(),
														},
														DeprecationMessage:  "Widget appearance.width is ignored by the API and has no effect.",
														MarkdownDescription: "Deprecated: the widget appearance.width field is ignored by the API and has no effect.",
													},
												},
												Validators: []validator.Object{
													dashboardwidgets.ExactlyOneOfChildren("definition", "reference"),
													dashboardwidgets.HighlightedNotOnReference(),
												},
											},
											Validators: []validator.List{
												listvalidator.SizeAtLeast(1),
											},
											MarkdownDescription: "The list of widgets to display in the dashboard.",
										},
									},
								},
								Validators: []validator.List{
									listvalidator.SizeAtLeast(1),
								},
								Optional: true,
							},
							"options": schema.SingleNestedAttribute{
								Attributes: map[string]schema.Attribute{
									"name": schema.StringAttribute{
										Required: true,
									},
									"description": schema.StringAttribute{
										Optional: true,
									},
									"color": schema.StringAttribute{
										Optional: true,
										Validators: []validator.String{
											stringvalidator.OneOf(dashboardwidgets.SectionValidColors...),
										},
										MarkdownDescription: fmt.Sprintf("Section color, valid values: %v", dashboardwidgets.SectionValidColors),
									},
									"collapsed": schema.BoolAttribute{
										Optional: true,
									},
								}, Optional: true,
							},
						},
					},
					Optional: true,
				},
			},
			MarkdownDescription: "Layout configuration for the dashboard's visual elements.",
			Validators: []validator.Object{
				dashboardwidgets.ExactlyOneOfObject(
					path.MatchRelative().AtParent().AtName("content_json"),
				),
			},
		},
		"variables": schema.ListNestedAttribute{
			Optional:           true,
			DeprecationMessage: "Use `variables_v2` for new dashboard variables. This legacy attribute remains available during migration.",
			NestedObject: schema.NestedAttributeObject{
				Attributes: map[string]schema.Attribute{
					"name": schema.StringAttribute{
						Optional: true,
					},
					"definition": schema.SingleNestedAttribute{
						Required: true,
						Attributes: map[string]schema.Attribute{
							"constant_value": schema.StringAttribute{
								Optional: true,
								DeprecationMessage: "`constant_value` is deprecated and rejected by the Coralogix API. " +
									"Use a `multi_select` variable with a `constant_list` source and a single `selected_values` entry instead, " +
									"e.g. `multi_select = { source = { constant_list = [\"value\"] }, selected_values = [\"value\"], values_order_direction = \"asc\" }`.",
							},
							"multi_select": schema.SingleNestedAttribute{
								Attributes: map[string]schema.Attribute{
									"selected_values": schema.ListAttribute{
										ElementType: types.StringType,
										Optional:    true,
										Computed:    true,
										Default:     listdefault.StaticValue(types.ListValueMust(types.StringType, []attr.Value{})),
									},
									"values_order_direction": schema.StringAttribute{
										Required: true,
										Validators: []validator.String{
											stringvalidator.OneOf(dashboardwidgets.DashboardValidOrderDirections...),
										},
										MarkdownDescription: fmt.Sprintf("The order direction of the values. Can be one of `%s`.", strings.Join(dashboardwidgets.DashboardValidOrderDirections, "`, `")),
									},
									"selection_type": schema.StringAttribute{
										Optional: true,
										Validators: []validator.String{
											stringvalidator.OneOf(dashboardwidgets.DashboardValidMultiSelectSelectionTypes...),
										},
										MarkdownDescription: fmt.Sprintf("Selection mode of the variable. Can be one of `%s`. Omit to use the API default (multi-select with an implicit \"All\" option).", strings.Join(dashboardwidgets.DashboardValidMultiSelectSelectionTypes, "`, `")),
									},
									"source": schema.SingleNestedAttribute{
										Attributes: map[string]schema.Attribute{
											"logs_path": schema.StringAttribute{
												Optional: true,
											},
											"metric_label": schema.SingleNestedAttribute{
												Attributes: map[string]schema.Attribute{
													"metric_name": schema.StringAttribute{
														Optional: true,
													},
													"label": schema.StringAttribute{
														Required: true,
													},
												},
												Optional: true,
											},
											"constant_list": schema.ListAttribute{
												ElementType: types.StringType,
												Optional:    true,
											},
											"span_field": dashboardwidgets.SpansFieldSchema(),
											"query": schema.SingleNestedAttribute{
												Attributes: map[string]schema.Attribute{
													"query": schema.SingleNestedAttribute{
														Attributes: map[string]schema.Attribute{
															"logs": schema.SingleNestedAttribute{
																Attributes: map[string]schema.Attribute{
																	"field_name": schema.SingleNestedAttribute{
																		Optional: true,
																		Attributes: map[string]schema.Attribute{
																			"log_regex": schema.StringAttribute{
																				Required: true,
																			},
																		},
																	},
																	"field_value": schema.SingleNestedAttribute{
																		Optional: true,
																		Attributes: map[string]schema.Attribute{
																			"observation_field": schema.SingleNestedAttribute{
																				Attributes:          dashboardwidgets.ObservationFieldSchema(),
																				Required:            true,
																				MarkdownDescription: "Explicit field reference with scope. Use when the field name contains a literal dot (e.g. `log.level`) or exists in multiple scopes — the bare `field` is resolved by the backend via dot-split, which silently fails to match flat fields whose identifier contains dots.",
																			},
																		},
																	},
																},
																Optional: true,
																Validators: []validator.Object{
																	dashboardwidgets.ExactlyOneOfChildren("field_name", "field_value"),
																},
															},
															"metrics": schema.SingleNestedAttribute{
																Attributes: map[string]schema.Attribute{
																	"metric_name": schema.SingleNestedAttribute{
																		Optional: true,
																		Attributes: map[string]schema.Attribute{
																			"metric_regex": schema.StringAttribute{
																				Required: true,
																			},
																		},
																	},
																	"label_name": schema.SingleNestedAttribute{
																		Optional: true,
																		Attributes: map[string]schema.Attribute{
																			"metric_regex": schema.StringAttribute{
																				Required: true,
																			},
																		},
																	},
																	"label_value": schema.SingleNestedAttribute{
																		Attributes: map[string]schema.Attribute{
																			"metric_name": stringOrVariableSchema(),
																			"label_name":  stringOrVariableSchema(),
																			"label_filters": schema.ListNestedAttribute{
																				Optional: true,
																				PlanModifiers: []planmodifier.List{
																					NormalizeEmptyListToNull{},
																				},
																				NestedObject: schema.NestedAttributeObject{
																					Attributes: map[string]schema.Attribute{
																						"metric": stringOrVariableSchema(),
																						"label":  stringOrVariableSchema(),
																						"operator": schema.SingleNestedAttribute{
																							Optional: true,
																							Attributes: map[string]schema.Attribute{
																								"type": schema.StringAttribute{
																									Required: true,
																									Validators: []validator.String{
																										stringvalidator.OneOf("equals", "not_equals"),
																									},
																								},
																								"selected_values": schema.ListNestedAttribute{
																									Optional: true,
																									PlanModifiers: []planmodifier.List{
																										NormalizeEmptyListToNull{},
																									},
																									NestedObject: schema.NestedAttributeObject{
																										Attributes: stringOrVariableAttr(),
																										Validators: []validator.Object{
																											dashboardwidgets.ExactlyOneOfChildren("string_value", "variable_name"),
																										},
																									},
																								},
																							},
																						},
																					},
																				},
																			},
																		},
																		Optional: true,
																	},
																},
																Validators: []validator.Object{
																	dashboardwidgets.ExactlyOneOfChildren("metric_name", "label_name", "label_value"),
																},
																Optional: true,
															},
															"spans": schema.SingleNestedAttribute{
																Attributes: map[string]schema.Attribute{
																	"field_name": schema.SingleNestedAttribute{
																		Attributes: map[string]schema.Attribute{
																			"span_regex": schema.StringAttribute{
																				Required: true,
																			},
																		},
																		Optional: true,
																	},
																	"field_value": dashboardwidgets.SpansFieldSchema(),
																},
																Validators: []validator.Object{
																	dashboardwidgets.ExactlyOneOfChildren("field_name", "field_value"),
																},
																Optional: true,
															},
														},
														Validators: []validator.Object{
															dashboardwidgets.ExactlyOneOfChildren("logs", "metrics", "spans"),
														},
														Required: true,
													},
													"refresh_strategy": schema.StringAttribute{
														Optional: true,
														Computed: true,
														Default:  stringdefault.StaticString(utils.UNSPECIFIED),
														Validators: []validator.String{
															stringvalidator.OneOf(dashboardwidgets.DashboardValidRefreshStrategies...),
														},
													},
													"value_display_options": schema.SingleNestedAttribute{
														Attributes: map[string]schema.Attribute{
															"value_regex": schema.StringAttribute{
																Optional: true,
															},
															"label_regex": schema.StringAttribute{
																Optional: true,
															},
														},
														Optional: true,
													},
												},
												Optional: true,
											},
										},
										Validators: []validator.Object{
											dashboardwidgets.ExactlyOneOfChildren("logs_path", "metric_label", "constant_list", "span_field", "query"),
										},
										Optional: true,
									},
								},
								Optional: true,
							},
						},
						Validators: []validator.Object{
							dashboardwidgets.ExactlyOneOfChildren("constant_value", "multi_select"),
						},
					},
					"display_name": schema.StringAttribute{
						Required: true,
					},
				},
			},
			Validators: []validator.List{
				listvalidator.SizeAtLeast(1),
			},
			MarkdownDescription: "Deprecated: list of legacy variables. Use `variables_v2` for new dashboard variables.",
		},
		"variables_v2": VariablesV2Schema(),
		"filters": schema.ListNestedAttribute{
			Optional: true,
			NestedObject: schema.NestedAttributeObject{
				Attributes: map[string]schema.Attribute{
					"source": schema.SingleNestedAttribute{
						Attributes: dashboardwidgets.TopLevelFilterSourceSchema(),
						Validators: []validator.Object{
							dashboardwidgets.ExactlyOneOfChildren("logs", "metrics", "spans"),
						},
						Required: true,
					},
					"id": schema.StringAttribute{
						Optional: true,
						Computed: true,
						PlanModifiers: []planmodifier.String{
							// The API stores whatever id the filter was created with
							// and does not assign one, so keep what is there.
							stringplanmodifier.UseNonNullStateForUnknown(),
						},
						MarkdownDescription: "Identifier of the filter inside the dashboard. Generated when omitted.",
					},
					"display_name": schema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "Name shown on the filter chip in the Coralogix UI. The API stores it, and leaves it out when the filter has no name of its own.",
					},
					"scope": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"all_widgets": schema.SingleNestedAttribute{
								Optional:            true,
								Attributes:          map[string]schema.Attribute{},
								MarkdownDescription: "Apply this filter to every widget in the dashboard.",
							},
							"specific_widgets": schema.SingleNestedAttribute{
								Optional: true,
								Attributes: map[string]schema.Attribute{
									"widget_ids": schema.ListAttribute{
										ElementType:         types.StringType,
										Required:            true,
										MarkdownDescription: "UUIDs of the widgets this filter applies to.",
									},
								},
							},
						},
						Validators: []validator.Object{
							dashboardwidgets.ExactlyOneOfChildren("all_widgets", "specific_widgets"),
						},
						MarkdownDescription: "Restrict this filter to specific widgets. Omit to apply it to all widgets.",
					},
					"enabled": schema.BoolAttribute{
						Optional: true,
						Computed: true,
						Default:  booldefault.StaticBool(true),
					},
					"collapsed": schema.BoolAttribute{
						Optional: true,
						Computed: true,
						Default:  booldefault.StaticBool(false),
					},
				},
			},
			Validators: []validator.List{
				listvalidator.SizeAtLeast(1),
			},
			MarkdownDescription: "List of filters that can be applied to the dashboard's data.",
		},
		"time_frame": dashboardwidgets.TimeFrameSchema(),
		"folder": schema.SingleNestedAttribute{
			Attributes: map[string]schema.Attribute{
				"id": schema.StringAttribute{
					Optional: true,
					MarkdownDescription: "ID of the dashboards folder this dashboard belongs to. " +
						"When authoring a `coralogix_dashboard` resource, this is the lifecycle-safe " +
						"choice: reference a `coralogix_dashboards_folder` resource's `id` so the " +
						"folder is created and destroyed by Terraform alongside the dashboard.",
				},
				"path": schema.StringAttribute{
					Optional: true,
					MarkdownDescription: "Slash-separated folder path (e.g. `Team/Subteam`). When set " +
						"on a `coralogix_dashboard` resource and the path does not already exist, the " +
						"Coralogix dashboards service implicitly creates the missing folder hierarchy " +
						"as a server-side side-effect of placing the dashboard. **That auto-created " +
						"folder is not tracked in Terraform state and is not removed when the " +
						"dashboard is destroyed — it will be left behind as an orphan in the Coralogix " +
						"UI.** Use `folder.id` (referencing a `coralogix_dashboards_folder` resource) " +
						"for symmetric apply/destroy semantics.",
				},
			},
			Optional: true,
			Validators: []validator.Object{
				dashboardwidgets.ExactlyOneOfChildren("id", "path"),
			},
			MarkdownDescription: "The dashboards folder this dashboard belongs to. Exactly one of " +
				"`id` or `path` is set. When authoring a `coralogix_dashboard` resource, `id` (pointing " +
				"at a `coralogix_dashboards_folder` resource) is the recommended form; `path` is " +
				"accepted but can trigger implicit server-side folder creation that Terraform will " +
				"not clean up on destroy — see the `path` attribute description for details.",
		},
		"annotations": schema.ListNestedAttribute{
			Optional: true,
			NestedObject: schema.NestedAttributeObject{
				Attributes: map[string]schema.Attribute{
					"id": schema.StringAttribute{
						Optional: true,
						Computed: true,
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.UseNonNullStateForUnknown(),
						},
					},
					"description": schema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "A human-readable description of the annotation. The Coralogix UI shows it next to the annotation name.",
					},
					"color": schema.StringAttribute{
						Optional: true,
						Validators: []validator.String{
							stringvalidator.OneOf(dashboardwidgets.DashboardValidAnnotationColors...),
						},
						MarkdownDescription: fmt.Sprintf("The colour the Coralogix UI draws the annotation in. Valid values are: %s.", strings.Join(dashboardwidgets.DashboardValidAnnotationColors, ", ")),
					},
					"name": schema.StringAttribute{
						Required: true,
					},
					"enabled": schema.BoolAttribute{
						Optional: true,
						Computed: true,
						Default:  booldefault.StaticBool(true),
					},
					"source": schema.SingleNestedAttribute{
						Attributes: map[string]schema.Attribute{
							"metrics": schema.SingleNestedAttribute{
								Attributes: map[string]schema.Attribute{
									"promql_query": schema.StringAttribute{
										Required: true,
									},
									"strategy": schema.SingleNestedAttribute{
										Attributes: map[string]schema.Attribute{
											// The API models this as a oneof with a
											// single member, so a stored strategy can
											// select nothing. Requiring it made an
											// annotation the Coralogix UI creates
											// impossible to express.
											"start_time": schema.SingleNestedAttribute{
												Attributes:          map[string]schema.Attribute{},
												Optional:            true,
												MarkdownDescription: "Take the first data point and use its value as the annotation timestamp, instead of the point's own timestamp. Omit the block to leave the strategy unset, which is what the API stores when nothing is chosen.",
											},
										},
										Required: true,
									},
									"message_template": schema.StringAttribute{
										Optional: true,
									},
									"labels": schema.ListAttribute{
										ElementType: types.StringType,
										Optional:    true,
									},
								},
								Optional: true,
							},
							"logs": schema.SingleNestedAttribute{
								Attributes: logsAndSpansAttributes(),
								Optional:   true,
							},
							"spans": schema.SingleNestedAttribute{
								Attributes: logsAndSpansAttributes(),
								Optional:   true,
							},
							"manual":           manualAnnotationSourceAttribute(),
							"dataprime":        dataprimeAnnotationSourceAttribute(),
							"event_recurrence": eventRecurrenceAnnotationSourceAttribute(),
						},
						Required: true,
						Validators: []validator.Object{
							dashboardwidgets.ExactlyOneOfChildren("metrics", "logs", "spans", "manual", "dataprime", "event_recurrence"),
						},
					},
					"scope": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"all_widgets": schema.SingleNestedAttribute{
								Optional:            true,
								Attributes:          map[string]schema.Attribute{},
								MarkdownDescription: "Apply this annotation to every widget in the dashboard.",
							},
							"specific_widgets": schema.SingleNestedAttribute{
								Optional: true,
								Attributes: map[string]schema.Attribute{
									"widget_ids": schema.ListAttribute{
										ElementType:         types.StringType,
										Required:            true,
										MarkdownDescription: "UUIDs of the widgets this annotation applies to.",
									},
								},
							},
						},
						Validators: []validator.Object{
							dashboardwidgets.ExactlyOneOfChildren("all_widgets", "specific_widgets"),
						},
						MarkdownDescription: "Restrict this annotation to specific widgets. Omit to show on all widgets.",
					},
				},
			},
			Validators: []validator.List{
				listvalidator.SizeAtLeast(1),
			},
		},
		"auto_refresh": schema.SingleNestedAttribute{
			Attributes: map[string]schema.Attribute{
				"type": schema.StringAttribute{
					Optional: true,
					Computed: true,
					Default:  stringdefault.StaticString("off"),
					Validators: []validator.String{
						stringvalidator.OneOf("off", "one_minute", "two_minutes", "five_minutes", "fifteen_minutes"),
					},
				},
			},
			Optional: true,
			Computed: true,
			PlanModifiers: []planmodifier.Object{
				NullWhenContentJSONManaged{},
			},
		},
		"content_json": schema.StringAttribute{
			Optional: true,
			Validators: []validator.String{
				stringvalidator.ConflictsWith(
					path.MatchRelative().AtParent().AtName("id"),
					path.MatchRelative().AtParent().AtName("name"),
					path.MatchRelative().AtParent().AtName("description"),
					path.MatchRelative().AtParent().AtName("layout"),
					path.MatchRelative().AtParent().AtName("variables"),
					path.MatchRelative().AtParent().AtName("variables_v2"),
					path.MatchRelative().AtParent().AtName("filters"),
					path.MatchRelative().AtParent().AtName("time_frame"),
					path.MatchRelative().AtParent().AtName("annotations"),
				),
				ContentJsonValidator{},
			},
			Description: "an option to set the dashboard content from a json file.",
		},
		"access_policy": schema.StringAttribute{
			Optional:            true,
			Computed:            true,
			PlanModifiers:       []planmodifier.String{PreserveStateForEquivalentJSON{}, stringplanmodifier.UseStateForUnknown()},
			MarkdownDescription: "JSON-encoded access policy for this dashboard.",
		},
	}
}
