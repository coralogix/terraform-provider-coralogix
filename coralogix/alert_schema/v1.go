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

package alertschema

import (
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/boolvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func V1() schema.Schema {
	return schema.Schema{
		Version:             1,
		MarkdownDescription: "Coralogix Alert. For more info check - https://coralogix.com/docs/getting-started-with-coralogix-alerts/.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "Alert ID.",
			},
			"name": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				MarkdownDescription: "Alert name.",
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Alert description.",
			},
			"enabled": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
				MarkdownDescription: "Alert enabled status. True by default.",
			},
			"priority": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Validators: []validator.String{
					stringvalidator.OneOf(validAlertPriorities...),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				DeprecationMessage:  "This field will be removed in the future in favor of the 'override' property where possible.",
				MarkdownDescription: fmt.Sprintf("Alert priority. Valid values: %q.", validAlertPriorities),
			},
			"schedule": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"active_on": schema.SingleNestedAttribute{
						Required: true,
						Attributes: map[string]schema.Attribute{
							"days_of_week": schema.SetAttribute{
								Required:    true,
								ElementType: types.StringType,
								Validators: []validator.Set{
									setvalidator.ValueStringsAre(
										stringvalidator.OneOf(validDaysOfWeek...),
									),
								},
								MarkdownDescription: fmt.Sprintf("Days of the week. Valid values: %q.", validDaysOfWeek),
							},
							"start_time": schema.StringAttribute{
								Required: true,
								Validators: []validator.String{
									stringvalidator.RegexMatches(
										regexp.MustCompile(`^[0-9]{2}:[0-9]{2}$`),
										"Use 24h time formats like 15:04 with a leading zero",
									),
								},
							},
							"end_time": schema.StringAttribute{
								Required: true,
								Validators: []validator.String{
									stringvalidator.RegexMatches(
										regexp.MustCompile(`^[0-9]{2}:[0-9]{2}$`),
										"Use 24h time formats like 15:04 with a leading zero",
									),
								},
							},
							"utc_offset": schema.StringAttribute{
								Optional: true,
								Default:  stringdefault.StaticString(DEFAULT_TIMEZONE_OFFSET),
								Computed: true,
								Validators: []validator.String{
									stringvalidator.RegexMatches(
										regexp.MustCompile(`^[-+][0-9]{4}$`),
										"Time zone to interpret the start/end times in, using a UTC offset like -0700",
									),
								},
							},
						},
					},
				},
				MarkdownDescription: "Alert schedule. Will be activated all the time if not specified.",
			},
			// type is being inferred by the type_definition attribute
			"type_definition": schema.SingleNestedAttribute{
				Required:            true,
				MarkdownDescription: "Alert type definition. Exactly one of the following must be specified: logs_immediate, logs_threshold, logs_anomaly, logs_ratio_threshold, logs_new_value, logs_unique_count, logs_time_relative_threshold, metric_threshold, metric_anomaly, tracing_immediate, tracing_threshold, flow, slo_threshold.",
				Attributes: map[string]schema.Attribute{
					"logs_immediate": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"logs_filter":                 logsFilterSchema(),
							"notification_payload_filter": notificationPayloadFilterSchema(),
						},
						Validators: []validator.Object{
							objectvalidator.ExactlyOneOf(
								path.MatchRoot("type_definition").AtName("logs_threshold"),
								path.MatchRoot("type_definition").AtName("logs_anomaly"),
								path.MatchRoot("type_definition").AtName("logs_ratio_threshold"),
								path.MatchRoot("type_definition").AtName("logs_unique_count"),
								path.MatchRoot("type_definition").AtName("logs_new_value"),
								path.MatchRoot("type_definition").AtName("logs_time_relative_threshold"),
								path.MatchRoot("type_definition").AtName("metric_threshold"),
								path.MatchRoot("type_definition").AtName("metric_anomaly"),
								path.MatchRoot("type_definition").AtName("tracing_immediate"),
								path.MatchRoot("type_definition").AtName("tracing_threshold"),
								path.MatchRoot("type_definition").AtName("flow"),
								path.MatchRoot("type_definition").AtName("slo_threshold"),
							),
						},
					},
					"logs_threshold": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"rules": schema.SetNestedAttribute{
								Required:   true,
								Validators: []validator.Set{setvalidator.SizeAtLeast(1)},
								NestedObject: schema.NestedAttributeObject{
									Attributes: map[string]schema.Attribute{
										"condition": schema.SingleNestedAttribute{
											Required: true,
											Attributes: map[string]schema.Attribute{
												"threshold": schema.Float64Attribute{
													Required: true,
												},
												"time_window": logsTimeWindowSchema(validLogsTimeWindowValues),
												"condition_type": schema.StringAttribute{
													Required: true,
													Validators: []validator.String{
														stringvalidator.OneOf(logsThresholdConditionValues...),
													},
													MarkdownDescription: fmt.Sprintf("Condition to evaluate the threshold with. Valid values: %q.", logsThresholdConditionValues),
												},
											},
										},
										"override": overrideAlertSchema(),
									},
								},
							},
							"notification_payload_filter":  notificationPayloadFilterSchema(),
							"logs_filter":                  logsFilterSchema(),
							"undetected_values_management": undetectedValuesManagementSchema(),
							"custom_evaluation_delay":      evaluationDelaySchema(),
						},
					},
					"logs_anomaly": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"custom_evaluation_delay":     evaluationDelaySchema(),
							"logs_filter":                 logsFilterSchema(),
							"notification_payload_filter": notificationPayloadFilterSchema(),
							"rules": schema.SetNestedAttribute{
								Required:   true,
								Validators: []validator.Set{setvalidator.SizeAtLeast(1)},
								NestedObject: schema.NestedAttributeObject{
									Attributes: map[string]schema.Attribute{
										"condition": schema.SingleNestedAttribute{
											Required: true,
											Attributes: map[string]schema.Attribute{
												"time_window": logsTimeWindowSchema(validLogsTimeWindowValues),
												"minimum_threshold": schema.Float64Attribute{
													Required: true,
												},
												"condition_type": schema.StringAttribute{
													Computed: true,
													Default:  stringdefault.StaticString("MORE_THAN_USUAL"),
													PlanModifiers: []planmodifier.String{
														stringplanmodifier.UseStateForUnknown(),
													},
												},
											},
										},
									},
								},
							},
						},
					},
					"logs_ratio_threshold": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"rules": schema.SetNestedAttribute{
								Required:   true,
								Validators: []validator.Set{setvalidator.SizeAtLeast(1)},
								NestedObject: schema.NestedAttributeObject{
									Attributes: map[string]schema.Attribute{
										"condition": schema.SingleNestedAttribute{
											Required: true,
											Attributes: map[string]schema.Attribute{
												"threshold": schema.Float64Attribute{
													Required: true,
												},
												"time_window": logsTimeWindowSchema(validLogsRatioTimeWindowValues),
												"condition_type": schema.StringAttribute{
													Required: true,
													Validators: []validator.String{
														stringvalidator.OneOf(logsRatioConditionMapValues...),
													},
													MarkdownDescription: fmt.Sprintf("Condition to evaluate the threshold with. Valid values: %q.", logsRatioConditionMapValues),
												},
											},
										},
										"override": overrideAlertSchema(),
									},
								},
							},
							"numerator": logsFilterSchema(),
							"numerator_alias": schema.StringAttribute{
								Required: true,
							},
							"denominator": logsFilterSchema(),
							"denominator_alias": schema.StringAttribute{
								Required: true,
							},
							"notification_payload_filter": notificationPayloadFilterSchema(),
							"group_by_for":                logsRatioGroupByForSchema(),
							"custom_evaluation_delay":     evaluationDelaySchema(),
						},
					},
					"logs_new_value": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"rules": schema.SetNestedAttribute{
								Required:   true,
								Validators: []validator.Set{setvalidator.SizeAtLeast(1)},
								NestedObject: schema.NestedAttributeObject{
									Attributes: map[string]schema.Attribute{
										"condition": schema.SingleNestedAttribute{
											Required: true,
											Attributes: map[string]schema.Attribute{
												"keypath_to_track": schema.StringAttribute{Required: true},
												"time_window":      logsTimeWindowSchema(validLogsNewValueTimeWindowValues),
											},
										},
									},
								},
							},
							"logs_filter":                 logsFilterSchema(),
							"notification_payload_filter": notificationPayloadFilterSchema(),
						},
					},
					"logs_unique_count": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"logs_filter":                 logsFilterSchema(),
							"notification_payload_filter": notificationPayloadFilterSchema(),
							"rules": schema.SetNestedAttribute{
								Required:   true,
								Validators: []validator.Set{setvalidator.SizeAtLeast(1)},
								NestedObject: schema.NestedAttributeObject{
									Attributes: map[string]schema.Attribute{
										"condition": schema.SingleNestedAttribute{
											Required: true,
											Attributes: map[string]schema.Attribute{
												"time_window":      logsTimeWindowSchema(validLogsUniqueCountTimeWindowValues),
												"max_unique_count": schema.Int64Attribute{Required: true},
											},
										},
									},
								},
							},
							"max_unique_count_per_group_by_key": schema.Int64Attribute{
								Optional: true,
							},
							"unique_count_keypath": schema.StringAttribute{
								Required: true,
							},
						},
					},
					"logs_time_relative_threshold": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"custom_evaluation_delay":      evaluationDelaySchema(),
							"logs_filter":                  logsFilterSchema(),
							"notification_payload_filter":  notificationPayloadFilterSchema(),
							"undetected_values_management": undetectedValuesManagementSchema(),
							"rules": schema.SetNestedAttribute{
								Required:   true,
								Validators: []validator.Set{setvalidator.SizeAtLeast(1)},
								NestedObject: schema.NestedAttributeObject{
									Attributes: map[string]schema.Attribute{
										"condition": schema.SingleNestedAttribute{
											Required: true,
											Attributes: map[string]schema.Attribute{
												"condition_type": schema.StringAttribute{
													Required: true,
													Validators: []validator.String{
														stringvalidator.OneOf(logsTimeRelativeConditionValues...),
													},
													MarkdownDescription: fmt.Sprintf("Condition . Valid values: %q.", logsTimeRelativeConditionValues),
												},
												"threshold": schema.Float64Attribute{
													Required: true,
												},
												"compared_to": schema.StringAttribute{
													Required: true,
													Validators: []validator.String{
														stringvalidator.OneOf(validLogsTimeRelativeComparedTo...),
													},
													MarkdownDescription: fmt.Sprintf("Compared to a different time frame. Valid values: %q.", validLogsTimeRelativeComparedTo),
												},
											},
										},
										"override": overrideAlertSchema(),
									},
								},
							},
						},
					},
					// Metrics
					"metric_threshold": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"custom_evaluation_delay":      evaluationDelaySchema(),
							"metric_filter":                metricFilterSchema(),
							"undetected_values_management": undetectedValuesManagementSchema(),
							"rules": schema.SetNestedAttribute{
								Required:   true,
								Validators: []validator.Set{setvalidator.SizeAtLeast(1)},
								NestedObject: schema.NestedAttributeObject{
									Attributes: map[string]schema.Attribute{
										"condition": schema.SingleNestedAttribute{
											Required: true,
											Attributes: map[string]schema.Attribute{
												"threshold": schema.Float64Attribute{
													Required: true,
												},
												"for_over_pct": schema.Int64Attribute{
													Required:            true,
													MarkdownDescription: "Percentage of metrics over the threshold. 0 means 'for at least once', 100 means 'for at least'. ",
												},
												"of_the_last": metricTimeWindowSchema(),
												"condition_type": schema.StringAttribute{
													Required: true,
													Validators: []validator.String{
														stringvalidator.OneOf(metricsThresholdConditionValues...),
													},
													MarkdownDescription: fmt.Sprintf("Condition to evaluate the threshold with. Valid values: %q.", metricsThresholdConditionValues),
												},
											},
										},
										"override": overrideAlertSchema(),
									},
								},
							},
							"missing_values": schema.SingleNestedAttribute{
								Required: true,
								Attributes: map[string]schema.Attribute{
									"replace_with_zero": schema.BoolAttribute{
										Optional: true,
										Validators: []validator.Bool{
											boolvalidator.ExactlyOneOf(path.MatchRelative().AtParent().AtName("min_non_null_values_pct")),
										},
									},
									"min_non_null_values_pct": schema.Int64Attribute{
										Optional: true,
									},
								},
							},
						},
					},
					"metric_anomaly": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"custom_evaluation_delay": evaluationDelaySchema(),
							"metric_filter":           metricFilterSchema(),
							"rules": schema.SetNestedAttribute{
								Required:   true,
								Validators: []validator.Set{setvalidator.SizeAtLeast(1)},
								NestedObject: schema.NestedAttributeObject{
									Attributes: map[string]schema.Attribute{
										"condition": schema.SingleNestedAttribute{
											Required: true,
											Attributes: map[string]schema.Attribute{
												"min_non_null_values_pct": schema.Int64Attribute{
													Required: true,
												},
												"threshold": schema.Float64Attribute{
													Required: true,
												},
												"for_over_pct": schema.Int64Attribute{
													Required:            true,
													MarkdownDescription: "Percentage of metrics over the threshold. 0 means 'for at least once', 100 means 'for at least'. ",
												},
												"of_the_last": anomalyMetricTimeWindowSchema(),
												"condition_type": schema.StringAttribute{
													Required: true,
													Validators: []validator.String{
														stringvalidator.OneOf(metricAnomalyConditionValues...),
													},
													MarkdownDescription: fmt.Sprintf("Condition to evaluate the threshold with. Valid values: %q.", metricAnomalyConditionValues),
												},
											},
										},
									},
								},
							},
						},
					},
					// Tracing
					"tracing_immediate": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"tracing_filter":              tracingQuerySchema(),
							"notification_payload_filter": notificationPayloadFilterSchema(),
						},
					},
					"tracing_threshold": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"tracing_filter":              tracingQuerySchema(),
							"notification_payload_filter": notificationPayloadFilterSchema(),
							"rules": schema.SetNestedAttribute{
								Required:   true,
								Validators: []validator.Set{setvalidator.SizeAtLeast(1)},
								NestedObject: schema.NestedAttributeObject{
									Attributes: map[string]schema.Attribute{
										"condition": schema.SingleNestedAttribute{
											Required: true,
											Attributes: map[string]schema.Attribute{
												"span_amount": schema.Float64Attribute{
													Required: true,
												},
												"time_window": logsTimeWindowSchema(validTracingTimeWindow),
												"condition_type": schema.StringAttribute{
													Computed: true,
													Default:  stringdefault.StaticString("MORE_THAN"),
													PlanModifiers: []planmodifier.String{
														stringplanmodifier.UseStateForUnknown(),
													},
												},
											},
										},
									},
									// Condition type is missing since there is only a single type to be filled in
								},
							},
						},
					},
					// Flow
					"flow": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"stages": schema.ListNestedAttribute{
								Required: true,
								NestedObject: schema.NestedAttributeObject{
									Attributes: map[string]schema.Attribute{
										"flow_stages_groups": schema.ListNestedAttribute{
											Required: true,
											NestedObject: schema.NestedAttributeObject{
												Attributes: map[string]schema.Attribute{
													"alert_defs": schema.SetNestedAttribute{
														Required: true,
														NestedObject: schema.NestedAttributeObject{
															Attributes: map[string]schema.Attribute{
																"id": schema.StringAttribute{
																	Required: true,
																},
																"not": schema.BoolAttribute{
																	Optional: true,
																	Computed: true,
																	Default:  booldefault.StaticBool(false),
																},
															},
														},
													},
													"next_op": schema.StringAttribute{
														Required: true,
														Validators: []validator.String{
															stringvalidator.OneOf(validFlowStagesGroupNextOps...),
														},
														MarkdownDescription: fmt.Sprintf("Next operation. Valid values: %q.", validFlowStagesGroupNextOps),
													},
													"alerts_op": schema.StringAttribute{
														Required: true,
														Validators: []validator.String{
															stringvalidator.OneOf(validFlowStagesGroupAlertsOps...),
														},
														MarkdownDescription: fmt.Sprintf("Alerts operation. Valid values: %q.", validFlowStagesGroupAlertsOps),
													},
												},
											},
										},
										"timeframe_ms": schema.Int64Attribute{
											Optional: true,
											Computed: true,
											Default:  int64default.StaticInt64(0),
										},
										"timeframe_type": schema.StringAttribute{
											Required: true,
											Validators: []validator.String{
												stringvalidator.OneOf(validFlowStageTimeFrameTypes...),
											},
										},
									},
								},
							},
							"enforce_suppression": schema.BoolAttribute{
								Optional: true,
								Computed: true,
								Default:  booldefault.StaticBool(false),
							},
						},
					},
					"slo_threshold": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"slo_definition": schema.SingleNestedAttribute{
								Required: true,
								Attributes: map[string]schema.Attribute{
									"slo_id": schema.StringAttribute{
										Required:            true,
										MarkdownDescription: "The SLO ID.",
									},
								},
								MarkdownDescription: "Configuration for the referenced SLO.",
							},
							"error_budget": schema.SingleNestedAttribute{
								Optional: true,
								Attributes: map[string]schema.Attribute{
									"rules": sloThresholdRulesAttribute(),
								},
								Validators: []validator.Object{
									objectvalidator.ExactlyOneOf(path.MatchRelative().AtParent().AtName("burn_rate")),
								},
								MarkdownDescription: "Error budget threshold configuration.",
							},
							"burn_rate": schema.SingleNestedAttribute{
								Optional: true,
								Attributes: map[string]schema.Attribute{
									"rules": sloThresholdRulesAttribute(),
									"dual": schema.SingleNestedAttribute{
										Optional: true,
										Attributes: map[string]schema.Attribute{
											"time_duration": timeDurationAttribute(),
										},
										Validators: []validator.Object{
											objectvalidator.ExactlyOneOf(path.MatchRelative().AtParent().AtName("single")),
										},
									},
									"single": schema.SingleNestedAttribute{
										Optional: true,
										Attributes: map[string]schema.Attribute{
											"time_duration": timeDurationAttribute(),
										},
										Validators: []validator.Object{
											objectvalidator.ExactlyOneOf(path.MatchRelative().AtParent().AtName("dual")),
										},
									},
								},
								Validators: []validator.Object{
									objectvalidator.ExactlyOneOf(path.MatchRelative().AtParent().AtName("error_budget")),
								},
								MarkdownDescription: "Burn rate threshold configuration.",
							},
						},
						MarkdownDescription: "SLO threshold alert type definition.",
					},
				},
			},
			"phantom_mode": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
			},
			"deleted": schema.BoolAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"group_by": schema.ListAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.List{
					ComputedForSomeAlerts{},
				},
				Validators: []validator.List{
					//imidiate, new value, tracing-immidiate,
					GroupByValidator{},
				},
				ElementType:         types.StringType,
				MarkdownDescription: "Group by fields.",
			},
			"incidents_settings": schema.SingleNestedAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.UseStateForUnknown(),
				},
				Attributes: map[string]schema.Attribute{
					"notify_on": schema.StringAttribute{
						Required: true,
						Validators: []validator.String{
							stringvalidator.OneOf(validNotifyOn...),
						},
						MarkdownDescription: fmt.Sprintf("Notify on. Valid values: %q.", validNotifyOn),
					},
					"retriggering_period": schema.SingleNestedAttribute{
						Required: true,
						Attributes: map[string]schema.Attribute{
							"minutes": schema.Int64Attribute{
								Required: true,
							},
						},
					},
				},
			},
			"notification_group": schema.SingleNestedAttribute{
				Optional: true,
				Computed: true,
				Default: objectdefault.StaticValue(types.ObjectValueMust(notificationGroupAttr(), map[string]attr.Value{
					"group_by_keys": types.ListNull(types.StringType),
					"webhooks_settings": types.SetNull(types.ObjectType{AttrTypes: map[string]attr.Type{
						"retriggering_period": types.ObjectType{AttrTypes: map[string]attr.Type{
							"minutes": types.Int64Type,
						}},
						"notify_on":      types.StringType,
						"integration_id": types.StringType,
						"recipients":     types.SetType{ElemType: types.StringType},
					},
					}),
					"destinations": types.ListNull(types.ObjectType{AttrTypes: map[string]attr.Type{
						"connector_id": types.StringType,
						"preset_id":    types.StringType,
						"notify_on":    types.StringType,
						"triggered_routing_overrides": types.ObjectType{AttrTypes: map[string]attr.Type{
							"connector_overrides": types.ListType{ElemType: types.ObjectType{AttrTypes: map[string]attr.Type{
								"field_name": types.StringType,
								"template":   types.StringType,
							}}},
							"preset_overrides": types.ListType{ElemType: types.ObjectType{AttrTypes: map[string]attr.Type{
								"field_name": types.StringType,
								"template":   types.StringType,
							}}},
							"output_schema_id": types.StringType,
						}},
						"resolved_routing_overrides": types.ObjectType{AttrTypes: map[string]attr.Type{
							"connector_overrides": types.ListType{ElemType: types.ObjectType{AttrTypes: map[string]attr.Type{
								"field_name": types.StringType,
								"template":   types.StringType,
							}}},
							"preset_overrides": types.ListType{ElemType: types.ObjectType{AttrTypes: map[string]attr.Type{
								"field_name": types.StringType,
								"template":   types.StringType,
							}}},
							"output_schema_id": types.StringType,
						}},
					}}),
					"router": types.ObjectNull(map[string]attr.Type{
						"notify_on": types.StringType,
					}),
				})),
				Attributes: map[string]schema.Attribute{
					"group_by_keys": schema.ListAttribute{
						Optional:    true,
						ElementType: types.StringType,
					},
					"webhooks_settings": schema.SetNestedAttribute{
						Optional: true,
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"retriggering_period": schema.SingleNestedAttribute{
									Optional: true,
									Computed: true,
									Default: objectdefault.StaticValue(types.ObjectValueMust(retriggeringPeriodAttr(), map[string]attr.Value{
										"minutes": types.Int64Value(10),
									})),
									Attributes: map[string]schema.Attribute{
										"minutes": schema.Int64Attribute{
											Required: true,
										},
									},
									MarkdownDescription: "Retriggering period in minutes. 10 minutes by default.",
								},
								"notify_on": schema.StringAttribute{
									Optional: true,
									Computed: true,
									Default:  stringdefault.StaticString("Triggered Only"),
									Validators: []validator.String{
										stringvalidator.OneOf(validNotifyOn...),
									},
									MarkdownDescription: fmt.Sprintf("Notify on. Valid values: %q. Triggered Only by default.", validNotifyOn),
								},
								"integration_id": schema.StringAttribute{
									Optional: true,
									Validators: []validator.String{
										stringvalidator.ExactlyOneOf(path.MatchRelative().AtParent().AtName("recipients")),
									},
								},
								"recipients": schema.SetAttribute{
									Optional:    true,
									ElementType: types.StringType,
								},
							},
							PlanModifiers: []planmodifier.Object{
								objectplanmodifier.UseStateForUnknown(),
							},
						},
					},
					"destinations": schema.ListNestedAttribute{
						Optional: true,
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"connector_id": schema.StringAttribute{
									Required:   true,
									Validators: []validator.String{},
								},
								"preset_id": schema.StringAttribute{
									Required:   true,
									Validators: []validator.String{},
								},
								"notify_on": schema.StringAttribute{
									Optional: true,
									Computed: true,
									Default:  stringdefault.StaticString("Triggered Only"),
									Validators: []validator.String{
										stringvalidator.OneOf(validNotifyOn...),
									},
								},
								"triggered_routing_overrides": schema.SingleNestedAttribute{
									Optional: true,
									Attributes: map[string]schema.Attribute{
										"connector_overrides": schema.ListNestedAttribute{
											Optional: true,
											Computed: true,
											Default: listdefault.StaticValue(types.ListNull(types.ObjectType{AttrTypes: map[string]attr.Type{
												"field_name": types.StringType,
												"template":   types.StringType,
											}})),
											NestedObject: schema.NestedAttributeObject{
												Attributes: map[string]schema.Attribute{
													"field_name": schema.StringAttribute{
														Required: true,
													},
													"template": schema.StringAttribute{
														Required: true,
													},
												},
											},
										},
										"preset_overrides": schema.ListNestedAttribute{
											Optional: true,
											Computed: true,
											Default: listdefault.StaticValue(types.ListNull(types.ObjectType{AttrTypes: map[string]attr.Type{
												"field_name": types.StringType,
												"template":   types.StringType,
											}})),
											NestedObject: schema.NestedAttributeObject{
												Attributes: map[string]schema.Attribute{
													"field_name": schema.StringAttribute{
														Required: true,
													},
													"template": schema.StringAttribute{
														Required: true,
													},
												},
											},
										},
										"output_schema_id": schema.StringAttribute{
											Required: true,
										},
									},
								},
								"resolved_routing_overrides": schema.SingleNestedAttribute{
									Optional: true,
									Attributes: map[string]schema.Attribute{
										"connector_overrides": schema.ListNestedAttribute{
											Optional: true,
											Computed: true,
											Default: listdefault.StaticValue(types.ListNull(types.ObjectType{AttrTypes: map[string]attr.Type{
												"field_name": types.StringType,
												"template":   types.StringType,
											}})),
											NestedObject: schema.NestedAttributeObject{
												Attributes: map[string]schema.Attribute{
													"field_name": schema.StringAttribute{
														Required: true,
													},
													"template": schema.StringAttribute{
														Required: true,
													},
												},
											},
										},
										"preset_overrides": schema.ListNestedAttribute{
											Optional: true,
											Computed: true,
											Default: listdefault.StaticValue(types.ListNull(types.ObjectType{AttrTypes: map[string]attr.Type{
												"field_name": types.StringType,
												"template":   types.StringType,
											}})),
											NestedObject: schema.NestedAttributeObject{
												Attributes: map[string]schema.Attribute{
													"field_name": schema.StringAttribute{
														Required: true,
													},
													"template": schema.StringAttribute{
														Required: true,
													},
												},
											},
										},
										"output_schema_id": schema.StringAttribute{
											Required: true,
										},
									},
								},
							},
						},
					},
					"router": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"notify_on": schema.StringAttribute{
								Optional: true,
								Computed: true,
								Default:  stringdefault.StaticString("Triggered Only"),
								Validators: []validator.String{
									stringvalidator.OneOf(validNotifyOn...),
								},
							},
						},
					},
				},
			},
			"labels": schema.MapAttribute{
				Optional:    true,
				ElementType: types.StringType,
			},
		},
	}
}
