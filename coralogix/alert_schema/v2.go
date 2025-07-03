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
	alerttypes "terraform-provider-coralogix/coralogix/alert_types"

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

func V2() schema.Schema {
	return schema.Schema{
		Version:             2,
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
					stringvalidator.OneOf(alerttypes.ValidAlertPriorities...),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				DeprecationMessage:  "This field will be removed in the future in favor of the 'override' property where possible.",
				MarkdownDescription: fmt.Sprintf("Alert priority. Valid values: %q.", alerttypes.ValidAlertPriorities),
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
										stringvalidator.OneOf(alerttypes.ValidDaysOfWeek...),
									),
								},
								MarkdownDescription: fmt.Sprintf("Days of the week. Valid values: %q.", alerttypes.ValidDaysOfWeek),
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
							"logs_filter":                 LogsFilterSchema(),
							"notification_payload_filter": NotificationPayloadFilterSchema(),
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
												"time_window": LogsTimeWindowSchema(alerttypes.ValidLogsTimeWindowValues),
												"condition_type": schema.StringAttribute{
													Required: true,
													Validators: []validator.String{
														stringvalidator.OneOf(alerttypes.LogsThresholdConditionValues...),
													},
													MarkdownDescription: fmt.Sprintf("Condition to evaluate the threshold with. Valid values: %q.", alerttypes.LogsThresholdConditionValues),
												},
											},
										},
										"override": OverrideAlertSchema(),
									},
								},
							},
							"notification_payload_filter":  NotificationPayloadFilterSchema(),
							"logs_filter":                  LogsFilterSchema(),
							"undetected_values_management": UndetectedValuesManagementSchema(),
							"custom_evaluation_delay":      EvaluationDelaySchema(),
						},
					},
					"logs_anomaly": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"custom_evaluation_delay":     EvaluationDelaySchema(),
							"logs_filter":                 LogsFilterSchema(),
							"notification_payload_filter": NotificationPayloadFilterSchema(),
							"rules": schema.SetNestedAttribute{
								Required:   true,
								Validators: []validator.Set{setvalidator.SizeAtLeast(1)},
								NestedObject: schema.NestedAttributeObject{
									Attributes: map[string]schema.Attribute{
										"condition": schema.SingleNestedAttribute{
											Required: true,
											Attributes: map[string]schema.Attribute{
												"time_window": LogsTimeWindowSchema(alerttypes.ValidLogsTimeWindowValues),
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
												"time_window": LogsTimeWindowSchema(alerttypes.ValidLogsRatioTimeWindowValues),
												"condition_type": schema.StringAttribute{
													Required: true,
													Validators: []validator.String{
														stringvalidator.OneOf(alerttypes.LogsRatioConditionMapValues...),
													},
													MarkdownDescription: fmt.Sprintf("Condition to evaluate the threshold with. Valid values: %q.", alerttypes.LogsRatioConditionMapValues),
												},
											},
										},
										"override": OverrideAlertSchema(),
									},
								},
							},
							"numerator": LogsFilterSchema(),
							"numerator_alias": schema.StringAttribute{
								Required: true,
							},
							"denominator": LogsFilterSchema(),
							"denominator_alias": schema.StringAttribute{
								Required: true,
							},
							"notification_payload_filter": NotificationPayloadFilterSchema(),
							"group_by_for":                LogsRatioGroupByForSchema(),
							"custom_evaluation_delay":     EvaluationDelaySchema(),
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
												"time_window":      LogsTimeWindowSchema(alerttypes.ValidLogsNewValueTimeWindowValues),
											},
										},
									},
								},
							},
							"logs_filter":                 LogsFilterSchema(),
							"notification_payload_filter": NotificationPayloadFilterSchema(),
						},
					},
					"logs_unique_count": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"logs_filter":                 LogsFilterSchema(),
							"notification_payload_filter": NotificationPayloadFilterSchema(),
							"rules": schema.SetNestedAttribute{
								Required:   true,
								Validators: []validator.Set{setvalidator.SizeAtLeast(1)},
								NestedObject: schema.NestedAttributeObject{
									Attributes: map[string]schema.Attribute{
										"condition": schema.SingleNestedAttribute{
											Required: true,
											Attributes: map[string]schema.Attribute{
												"time_window":      LogsTimeWindowSchema(alerttypes.ValidLogsUniqueCountTimeWindowValues),
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
							"custom_evaluation_delay":      EvaluationDelaySchema(),
							"logs_filter":                  LogsFilterSchema(),
							"notification_payload_filter":  NotificationPayloadFilterSchema(),
							"undetected_values_management": UndetectedValuesManagementSchema(),
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
														stringvalidator.OneOf(alerttypes.LogsTimeRelativeConditionValues...),
													},
													MarkdownDescription: fmt.Sprintf("Condition . Valid values: %q.", alerttypes.LogsTimeRelativeConditionValues),
												},
												"threshold": schema.Float64Attribute{
													Required: true,
												},
												"compared_to": schema.StringAttribute{
													Required: true,
													Validators: []validator.String{
														stringvalidator.OneOf(alerttypes.ValidLogsTimeRelativeComparedTo...),
													},
													MarkdownDescription: fmt.Sprintf("Compared to a different time frame. Valid values: %q.", alerttypes.ValidLogsTimeRelativeComparedTo),
												},
											},
										},
										"override": OverrideAlertSchema(),
									},
								},
							},
						},
					},
					// Metrics
					"metric_threshold": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"custom_evaluation_delay":      EvaluationDelaySchema(),
							"metric_filter":                MetricFilterSchema(),
							"undetected_values_management": UndetectedValuesManagementSchema(),
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
												"of_the_last": MetricTimeWindowSchema(),
												"condition_type": schema.StringAttribute{
													Required: true,
													Validators: []validator.String{
														stringvalidator.OneOf(alerttypes.MetricsThresholdConditionValues...),
													},
													MarkdownDescription: fmt.Sprintf("Condition to evaluate the threshold with. Valid values: %q.", alerttypes.MetricsThresholdConditionValues),
												},
											},
										},
										"override": OverrideAlertSchema(),
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
							"custom_evaluation_delay": EvaluationDelaySchema(),
							"metric_filter":           MetricFilterSchema(),
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
												"of_the_last": AnomalyMetricTimeWindowSchema(),
												"condition_type": schema.StringAttribute{
													Required: true,
													Validators: []validator.String{
														stringvalidator.OneOf(alerttypes.MetricAnomalyConditionValues...),
													},
													MarkdownDescription: fmt.Sprintf("Condition to evaluate the threshold with. Valid values: %q.", alerttypes.MetricAnomalyConditionValues),
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
							"tracing_filter":              TracingQuerySchema(),
							"notification_payload_filter": NotificationPayloadFilterSchema(),
						},
					},
					"tracing_threshold": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"tracing_filter":              TracingQuerySchema(),
							"notification_payload_filter": NotificationPayloadFilterSchema(),
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
												"time_window": LogsTimeWindowSchema(alerttypes.ValidTracingTimeWindow),
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
															stringvalidator.OneOf(alerttypes.ValidFlowStagesGroupNextOps...),
														},
														MarkdownDescription: fmt.Sprintf("Next operation. Valid values: %q.", alerttypes.ValidFlowStagesGroupNextOps),
													},
													"alerts_op": schema.StringAttribute{
														Required: true,
														Validators: []validator.String{
															stringvalidator.OneOf(alerttypes.ValidFlowStagesGroupAlertsOps...),
														},
														MarkdownDescription: fmt.Sprintf("Alerts operation. Valid values: %q.", alerttypes.ValidFlowStagesGroupAlertsOps),
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
												stringvalidator.OneOf(alerttypes.ValidFlowStageTimeFrameTypes...),
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
									"rules": SloThresholdRulesAttribute(),
								},
								Validators: []validator.Object{
									objectvalidator.ExactlyOneOf(path.MatchRelative().AtParent().AtName("burn_rate")),
								},
								MarkdownDescription: "Error budget threshold configuration.",
							},
							"burn_rate": schema.SingleNestedAttribute{
								Optional: true,
								Attributes: map[string]schema.Attribute{
									"rules": SloThresholdRulesAttribute(),
									"dual": schema.SingleNestedAttribute{
										Optional: true,
										Attributes: map[string]schema.Attribute{
											"time_duration": TimeDurationAttribute(),
										},
										Validators: []validator.Object{
											objectvalidator.ExactlyOneOf(path.MatchRelative().AtParent().AtName("single")),
										},
									},
									"single": schema.SingleNestedAttribute{
										Optional: true,
										Attributes: map[string]schema.Attribute{
											"time_duration": TimeDurationAttribute(),
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
							stringvalidator.OneOf(alerttypes.ValidNotifyOn...),
						},
						MarkdownDescription: fmt.Sprintf("Notify on. Valid values: %q.", alerttypes.ValidNotifyOn),
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
				Default: objectdefault.StaticValue(types.ObjectValueMust(NotificationGroupAttr(), map[string]attr.Value{
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
							"payload_type": types.StringType,
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
							"payload_type": types.StringType,
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
									Default: objectdefault.StaticValue(types.ObjectValueMust(RetriggeringPeriodAttr(), map[string]attr.Value{
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
										stringvalidator.OneOf(alerttypes.ValidNotifyOn...),
									},
									MarkdownDescription: fmt.Sprintf("Notify on. Valid values: %q. Triggered Only by default.", alerttypes.ValidNotifyOn),
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
										stringvalidator.OneOf(alerttypes.ValidNotifyOn...),
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
										"payload_type": schema.StringAttribute{
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
										"payload_type": schema.StringAttribute{
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
									stringvalidator.OneOf(alerttypes.ValidNotifyOn...),
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
