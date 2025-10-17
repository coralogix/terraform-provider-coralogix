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
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/float64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func V4() schema.Schema {
	attributes := dashboardSchemaAttributesV4()

	return schema.Schema{
		Version:    4,
		Attributes: attributes,
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
								Computed: true,
								PlanModifiers: []planmodifier.String{
									stringplanmodifier.UseStateForUnknown(),
								},
							},
							"rows": schema.ListNestedAttribute{
								NestedObject: schema.NestedAttributeObject{
									Attributes: map[string]schema.Attribute{
										"id": schema.StringAttribute{
											Computed: true,
											PlanModifiers: []planmodifier.String{
												stringplanmodifier.UseStateForUnknown(),
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
														Computed: true,
														PlanModifiers: []planmodifier.String{
															stringplanmodifier.UseStateForUnknown(),
														},
													},
													"title": schema.StringAttribute{
														Optional:            true,
														MarkdownDescription: "Widget title. Required for all widgets except markdown.",
													},
													"description": schema.StringAttribute{
														Optional:            true,
														MarkdownDescription: "Widget description.",
													},
													"definition": schema.SingleNestedAttribute{
														Required: true,
														Attributes: map[string]schema.Attribute{
															"line_chart": dashboardwidgets.LineChartSchema(),
															"hexagon":    dashboardwidgets.HexagonSchema(),
															"data_table": dashboardwidgets.DataTableSchema(),
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
																				},
																				Validators: []validator.Object{
																					objectvalidator.ExactlyOneOf(
																						path.MatchRelative().AtParent().AtName("spans"),
																						path.MatchRelative().AtParent().AtName("metrics"),
																						path.MatchRelative().AtParent().AtName("data_prime"),
																					),
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
																						Default:             stringdefault.StaticString(dashboardwidgets.UNSPECIFIED),
																					},
																					"filters":    dashboardwidgets.MetricFiltersSchema(),
																					"time_frame": dashboardwidgets.TimeFrameSchema(),
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
																					"spans_aggregation": dashboardwidgets.SpansAggregationSchema(),
																					"filters":           dashboardwidgets.SpansFilterSchema(),
																					"time_frame":        dashboardwidgets.TimeFrameSchema(),
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
																			"data_prime": schema.SingleNestedAttribute{
																				Attributes: map[string]schema.Attribute{
																					"query": schema.StringAttribute{
																						Optional: true,
																					},
																					"filters": schema.ListNestedAttribute{
																						NestedObject: schema.NestedAttributeObject{
																							Attributes: dashboardwidgets.FiltersSourceSchema(),
																						},
																						Optional: true,
																					},
																					"time_frame": dashboardwidgets.TimeFrameSchema(),
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
																	"min": schema.Float64Attribute{
																		Optional: true,
																		Computed: true,
																		Default:  float64default.StaticFloat64(0),
																	},
																	"max": schema.Float64Attribute{
																		Optional: true,
																		Computed: true,
																		Default:  float64default.StaticFloat64(100),
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
																		Default:  stringdefault.StaticString(dashboardwidgets.UNSPECIFIED),
																		Validators: []validator.String{
																			stringvalidator.OneOf(dashboardwidgets.DashboardValidDataModeTypes...),
																		},
																		MarkdownDescription: fmt.Sprintf("The data mode type. Can be one of %q.", dashboardwidgets.DashboardValidDataModeTypes),
																	},
																	"threshold_by": schema.StringAttribute{
																		Optional: true,
																		Computed: true,
																		Default:  stringdefault.StaticString(dashboardwidgets.UNSPECIFIED),
																		Validators: []validator.String{
																			stringvalidator.OneOf(dashboardwidgets.DashboardValidGaugeThresholdBy...),
																		},
																		MarkdownDescription: fmt.Sprintf("The threshold by. Can be one of %q.", dashboardwidgets.DashboardValidGaugeThresholdBy),
																	},
																	"display_series_name": schema.BoolAttribute{
																		Optional: true,
																		Computed: true,
																		Default:  booldefault.StaticBool(true),
																	},
																	"decimal": schema.NumberAttribute{
																		Optional: true,
																	},
																},
																Validators: []validator.Object{
																	dashboardwidgets.SupportedWidgetsValidatorWithout("gauge"),
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
																					"aggregation":        dashboardwidgets.SpansAggregationSchema(),
																					"filters":            dashboardwidgets.SpansFilterSchema(),
																					"group_names":        dashboardwidgets.SpansFieldsSchema(),
																					"stacked_group_name": dashboardwidgets.SpansFieldSchema(),
																					"time_frame":         dashboardwidgets.TimeFrameSchema(),
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
																					"filters": dashboardwidgets.MetricFiltersSchema(),
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
																						Required: true,
																					},
																					"filters": schema.ListNestedAttribute{
																						NestedObject: schema.NestedAttributeObject{
																							Attributes: dashboardwidgets.FiltersSourceSchema(),
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
																				Default:  stringdefault.StaticString(dashboardwidgets.UNSPECIFIED),
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
																		Default:  stringdefault.StaticString(dashboardwidgets.UNSPECIFIED),
																	},
																	"color_scheme": schema.StringAttribute{
																		Optional: true,
																		Validators: []validator.String{
																			stringvalidator.OneOf(dashboardwidgets.DashboardValidColorSchemes...),
																		},
																		Description: fmt.Sprintf("The color scheme. Can be one of %s.", strings.Join(dashboardwidgets.DashboardValidColorSchemes, ", ")),
																	},
																	"data_mode_type": schema.StringAttribute{
																		Optional: true,
																		Computed: true,
																		Default:  stringdefault.StaticString(dashboardwidgets.UNSPECIFIED),
																		Validators: []validator.String{
																			stringvalidator.OneOf(dashboardwidgets.DashboardValidDataModeTypes...),
																		},
																	},
																},
																Validators: []validator.Object{
																	dashboardwidgets.SupportedWidgetsValidatorWithout("pie_chart"),
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
																					"filters": dashboardwidgets.MetricFiltersSchema(),
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
																					"aggregation":        dashboardwidgets.SpansAggregationSchema(),
																					"filters":            dashboardwidgets.SpansFilterSchema(),
																					"group_names":        dashboardwidgets.SpansFieldsSchema(),
																					"stacked_group_name": dashboardwidgets.SpansFieldSchema(),
																					"time_frame":         dashboardwidgets.TimeFrameSchema(),
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
																			"data_prime": schema.SingleNestedAttribute{
																				Attributes: map[string]schema.Attribute{
																					"query": schema.StringAttribute{
																						Required: true,
																					},
																					"filters": schema.ListNestedAttribute{
																						NestedObject: schema.NestedAttributeObject{
																							Attributes: dashboardwidgets.FiltersSourceSchema(),
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
																				Validators: []validator.Object{
																					objectvalidator.ExactlyOneOf(
																						path.MatchRelative().AtParent().AtName("logs"),
																						path.MatchRelative().AtParent().AtName("metrics"),
																						path.MatchRelative().AtParent().AtName("spans"),
																					),
																				},
																			},
																		},
																		Optional: true,
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
																		Default:  stringdefault.StaticString(dashboardwidgets.UNSPECIFIED),
																	},
																	"colors_by": schema.StringAttribute{
																		Optional: true,
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
																				Optional: true,
																				Validators: []validator.Object{
																					objectvalidator.ExactlyOneOf(
																						path.MatchRelative().AtParent().AtName("value"),
																					),
																				},
																			},
																			"value": schema.SingleNestedAttribute{
																				Attributes: map[string]schema.Attribute{},
																				Optional:   true,
																				Validators: []validator.Object{
																					objectvalidator.ExactlyOneOf(
																						path.MatchRelative().AtParent().AtName("time"),
																					),
																				},
																			},
																		},
																	},
																	"unit": schema.StringAttribute{
																		Optional: true,
																		Computed: true,
																		Default:  stringdefault.StaticString(dashboardwidgets.UNSPECIFIED),
																		Validators: []validator.String{
																			stringvalidator.OneOf(dashboardwidgets.DashboardValidUnits...),
																		},
																		MarkdownDescription: fmt.Sprintf("The unit of the chart. Can be one of %s.", strings.Join(dashboardwidgets.DashboardValidUnits, ", ")),
																	},
																	"sort_by": schema.StringAttribute{
																		Optional: true,
																		Computed: true,
																		Default:  stringdefault.StaticString(dashboardwidgets.UNSPECIFIED),
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
																	"data_mode_type": schema.StringAttribute{
																		Optional: true,
																		Computed: true,
																		Default:  stringdefault.StaticString(dashboardwidgets.UNSPECIFIED),
																		Validators: []validator.String{
																			stringvalidator.OneOf(dashboardwidgets.DashboardValidDataModeTypes...),
																		},
																	},
																},
																Validators: []validator.Object{
																	dashboardwidgets.SupportedWidgetsValidatorWithout("bar_chart"),
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
																					"time_frame": dashboardwidgets.TimeFrameSchema(),
																				},
																				Optional: true,
																			},
																			"spans": schema.SingleNestedAttribute{
																				Attributes: map[string]schema.Attribute{
																					"lucene_query": schema.StringAttribute{
																						Optional: true,
																					},
																					"aggregation":        dashboardwidgets.SpansAggregationSchema(),
																					"filters":            dashboardwidgets.SpansFilterSchema(),
																					"group_names":        dashboardwidgets.SpansFieldsSchema(),
																					"stacked_group_name": dashboardwidgets.SpansFieldSchema(),
																					"time_frame":         dashboardwidgets.TimeFrameSchema(),
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
																				Validators: []validator.Object{
																					objectvalidator.ExactlyOneOf(
																						path.MatchRelative().AtParent().AtName("logs"),
																						path.MatchRelative().AtParent().AtName("metrics"),
																						path.MatchRelative().AtParent().AtName("spans"),
																					),
																				},
																			},
																		},
																		Optional: true,
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
																		Default:  stringdefault.StaticString(dashboardwidgets.UNSPECIFIED),
																	},
																	"colors_by": schema.StringAttribute{
																		Optional: true,
																	},
																	"unit": schema.StringAttribute{
																		Optional: true,
																		Computed: true,
																		Default:  stringdefault.StaticString(dashboardwidgets.UNSPECIFIED),
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
																		Default:  stringdefault.StaticString(dashboardwidgets.UNSPECIFIED),
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
																	"data_mode_type": schema.StringAttribute{
																		Optional: true,
																		Computed: true,
																		Default:  stringdefault.StaticString(dashboardwidgets.UNSPECIFIED),
																		Validators: []validator.String{
																			stringvalidator.OneOf(dashboardwidgets.DashboardValidDataModeTypes...),
																		},
																	},
																},
																Validators: []validator.Object{
																	dashboardwidgets.SupportedWidgetsValidatorWithout("horizontal_bar_chart"),
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
																Validators: []validator.Object{
																	dashboardwidgets.SupportedWidgetsValidatorWithout("markdown"),
																	objectvalidator.ConflictsWith(
																		path.MatchRelative().AtParent().AtParent().AtName("title"),
																	),
																},
																Optional: true,
															},
														},
														MarkdownDescription: fmt.Sprintf("The widget definition. Can contain one of %v", dashboardwidgets.SupportedWidgetTypes),
													},
													"width": schema.Int64Attribute{
														Optional:            true,
														Computed:            true,
														Default:             int64default.StaticInt64(0),
														MarkdownDescription: "The width of the chart.",
													},
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
				objectvalidator.ExactlyOneOf(
					path.MatchRelative().AtParent().AtName("content_json"),
				),
			},
		},
		"variables": schema.ListNestedAttribute{
			Optional: true,
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
								Validators: []validator.String{
									stringvalidator.ExactlyOneOf(path.MatchRelative().AtParent().AtName("multi_select")),
								},
							},
							"multi_select": schema.SingleNestedAttribute{
								Attributes: map[string]schema.Attribute{
									"selected_values": schema.ListAttribute{
										ElementType: types.StringType,
										Optional:    true,
									},
									"values_order_direction": schema.StringAttribute{
										Required: true,
										Validators: []validator.String{
											stringvalidator.OneOf(dashboardwidgets.DashboardValidOrderDirections...),
										},
										MarkdownDescription: fmt.Sprintf("The order direction of the values. Can be one of `%s`.", strings.Join(dashboardwidgets.DashboardValidOrderDirections, "`, `")),
									},
									"source": schema.SingleNestedAttribute{
										Attributes: map[string]schema.Attribute{
											"logs_path": schema.StringAttribute{
												Optional: true,
												Validators: []validator.String{
													stringvalidator.ExactlyOneOf(
														path.MatchRelative().AtParent().AtName("metric_label"),
														path.MatchRelative().AtParent().AtName("constant_list"),
														path.MatchRelative().AtParent().AtName("span_field"),
														path.MatchRelative().AtParent().AtName("query"),
													),
												},
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
																		Validators: []validator.Object{
																			objectvalidator.ExactlyOneOf(path.MatchRelative().AtParent().AtName("field_value")),
																		},
																	},
																	"field_value": schema.SingleNestedAttribute{
																		Optional: true,
																		Attributes: map[string]schema.Attribute{
																			"observation_field": schema.SingleNestedAttribute{
																				Attributes: dashboardwidgets.ObservationFieldSchema(),
																				Required:   true,
																			},
																		},
																	},
																},
																Optional: true,
																Validators: []validator.Object{
																	objectvalidator.ExactlyOneOf(
																		path.MatchRelative().AtParent().AtName("spans"),
																		path.MatchRelative().AtParent().AtName("metrics"),
																	),
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
																		Validators: []validator.Object{
																			objectvalidator.ExactlyOneOf(
																				path.MatchRelative().AtParent().AtName("label_name"),
																				path.MatchRelative().AtParent().AtName("label_value"),
																			),
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
																									NestedObject: schema.NestedAttributeObject{
																										Attributes: stringOrVariableAttr(),
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
																		Validators: []validator.Object{
																			objectvalidator.ExactlyOneOf(path.MatchRelative().AtParent().AtName("field_value")),
																		},
																	},
																	"field_value": dashboardwidgets.SpansFieldSchema(),
																},
																Optional: true,
															},
														},
														Required: true,
													},
													"refresh_strategy": schema.StringAttribute{
														Optional: true,
														Computed: true,
														Default:  stringdefault.StaticString(dashboardwidgets.UNSPECIFIED),
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
										Optional: true,
									},
								},
								Optional: true,
							},
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
			MarkdownDescription: "List of variables that can be used within the dashboard for dynamic content.",
		},
		"filters": schema.ListNestedAttribute{
			Optional: true,
			NestedObject: schema.NestedAttributeObject{
				Attributes: map[string]schema.Attribute{
					"source": schema.SingleNestedAttribute{
						Attributes: dashboardwidgets.FiltersSourceSchema(),
						Required:   true,
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
					Computed: true,
					Validators: []validator.String{
						stringvalidator.ExactlyOneOf(
							path.MatchRelative().AtParent().AtName("path"),
						),
					},
				},
				"path": schema.StringAttribute{
					Optional: true,
					Computed: true,
					Validators: []validator.String{
						stringvalidator.ExactlyOneOf(
							path.MatchRelative().AtParent().AtName("id"),
						),
					},
				},
			},
			Optional: true,
		},
		"annotations": schema.ListNestedAttribute{
			Optional: true,
			NestedObject: schema.NestedAttributeObject{
				Attributes: map[string]schema.Attribute{
					"id": schema.StringAttribute{
						Optional: true,
						Computed: true,
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.UseStateForUnknown(),
						},
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
											"start_time": schema.SingleNestedAttribute{
												Attributes: map[string]schema.Attribute{},
												Required:   true,
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
								Validators: []validator.Object{
									objectvalidator.ExactlyOneOf(
										path.MatchRelative().AtParent().AtName("logs"),
										path.MatchRelative().AtParent().AtName("spans"),
									),
								},
							},
							"logs": schema.SingleNestedAttribute{
								Attributes: logsAndSpansAttributes(),
								Optional:   true,
								Validators: []validator.Object{
									objectvalidator.ExactlyOneOf(
										path.MatchRelative().AtParent().AtName("metrics"),
										path.MatchRelative().AtParent().AtName("spans"),
									),
								},
							},
							"spans": schema.SingleNestedAttribute{
								Attributes: logsAndSpansAttributes(),
								Optional:   true,
								Validators: []validator.Object{
									objectvalidator.ExactlyOneOf(
										path.MatchRelative().AtParent().AtName("metrics"),
										path.MatchRelative().AtParent().AtName("logs"),
									),
								},
							},
						},
						Required: true,
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
						stringvalidator.OneOf("off", "two_minutes", "five_minutes"),
					},
				},
			},
			Optional: true,
			Computed: true,
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
					path.MatchRelative().AtParent().AtName("filters"),
					path.MatchRelative().AtParent().AtName("time_frame"),
					path.MatchRelative().AtParent().AtName("annotations"),
				),
				ContentJsonValidator{},
			},
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.RequiresReplaceIf(utils.JSONStringsEqualPlanModifier, "", ""),
			},
			Description: "an option to set the dashboard content from a json file.",
		},
	}
}
