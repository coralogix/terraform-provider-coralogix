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
	"context"
	"fmt"
	"regexp"

	alerts "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/alert_definitions_service"
	alerttypes "github.com/coralogix/terraform-provider-coralogix/internal/provider/alerts/alert_types"

	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	"github.com/hashicorp/terraform-plugin-framework-validators/int32validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int32default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

type GroupByValidator struct {
}

func (g GroupByValidator) Description(ctx context.Context) string {
	return "Group by validator."
}

func (g GroupByValidator) MarkdownDescription(ctx context.Context) string {
	return "Group by validator."
}

func (g GroupByValidator) ValidateList(ctx context.Context, request validator.ListRequest, response *validator.ListResponse) {
	paths, diags := request.Config.PathMatches(ctx, path.MatchRoot("type_definition"))
	if diags.HasError() {
		response.Diagnostics.Append(diags...)
		return
	}
	var typeDefinition types.Object
	diags = request.Config.GetAttribute(ctx, paths[0], &typeDefinition)
	if diags.HasError() {
		response.Diagnostics.Append(diags...)
		return
	}

	if typeDefinition.IsNull() || typeDefinition.IsUnknown() {
		return
	}

	var typeDefinitionModel alerttypes.AlertTypeDefinitionModel
	if diags = typeDefinition.As(ctx, &typeDefinitionModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		response.Diagnostics.Append(diags...)
		return
	}

	if !utils.ObjIsNullOrUnknown(typeDefinitionModel.LogsImmediate) || !utils.ObjIsNullOrUnknown(typeDefinitionModel.LogsNewValue) || !utils.ObjIsNullOrUnknown(typeDefinitionModel.TracingImmediate) {
		if !(request.ConfigValue.IsNull() || request.ConfigValue.IsUnknown()) {
			response.Diagnostics.AddError("group_by", "Group by is not allowed for logs_immediate, logs_new_value, tracing_immediate alert types.")
		}
	}
}

type PriorityOverrideFallback struct {
}

func (c PriorityOverrideFallback) Description(ctx context.Context) string {
	return "Fall back to top level priority for overrides."
}

func (c PriorityOverrideFallback) MarkdownDescription(ctx context.Context) string {
	return "Fall back to top level priority for overrides."
}

func (c PriorityOverrideFallback) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	// if a priority override is provided, do nothing
	if !req.ConfigValue.IsNull() {
		return
	}

	var topLevelPriorityConfig types.String
	if diags := req.Config.GetAttribute(ctx, path.Root("priority"), &topLevelPriorityConfig); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// if the top level priority and the override priority are both null, set the plan value to "P5". If the top level priority is not null, use that value for the override priority
	if topLevelPriorityConfig.IsNull() {
		resp.PlanValue = types.StringValue("P5")
	} else {
		resp.PlanValue = topLevelPriorityConfig
	}
}

type ComputedForSomeAlerts struct {
}

func (c ComputedForSomeAlerts) Description(ctx context.Context) string {
	return "Computed for metric alerts."
}

func (c ComputedForSomeAlerts) MarkdownDescription(ctx context.Context) string {
	return "Computed for metric alerts."
}

func (c ComputedForSomeAlerts) PlanModifyList(ctx context.Context, request planmodifier.ListRequest, response *planmodifier.ListResponse) {
	paths, diags := request.Plan.PathMatches(ctx, path.MatchRoot("type_definition"))
	if diags.HasError() {
		response.Diagnostics.Append(diags...)
		return
	}
	var typeDefinition alerttypes.AlertTypeDefinitionModel
	diags = request.Plan.GetAttribute(ctx, paths[0], &typeDefinition)
	if diags.HasError() {
		response.Diagnostics.Append(diags...)
		return
	}

	// special case for metric alerts
	var typeDefinitionStr string
	if !utils.ObjIsNullOrUnknown(typeDefinition.MetricThreshold) {
		typeDefinitionStr = "metric_threshold"
	} else if !utils.ObjIsNullOrUnknown(typeDefinition.MetricAnomaly) {
		typeDefinitionStr = "metric_anomaly"
	} else if !utils.ObjIsNullOrUnknown(typeDefinition.LogsNewValue) {
		typeDefinitionStr = "logs_new_value"
	}

	switch typeDefinitionStr {
	case "metric_threshold", "metric_anomaly":
		paths, diags = request.Plan.PathMatches(ctx, path.MatchRoot("type_definition").AtName(typeDefinitionStr).AtName("metric_filter").AtName("promql"))
		if diags.HasError() {
			response.Diagnostics.Append(diags...)
			return
		}

		var promqlPlan types.String
		diags = request.Plan.GetAttribute(ctx, paths[0], &promqlPlan)
		if diags.HasError() {
			response.Diagnostics.Append(diags...)
			return
		}

		var promqlState types.String
		diags = request.State.GetAttribute(ctx, paths[0], &promqlState)
		if diags.HasError() {
			response.Diagnostics.Append(diags...)
			return
		}

		if request.ConfigValue.IsUnknown() || request.ConfigValue.IsNull() {
			if !promqlState.Equal(promqlPlan) {
				response.PlanValue = types.ListUnknown(types.StringType)
			} else {
				response.PlanValue = request.StateValue
			}
			return
		}
	case "logs_new_value": // keypath_to_track values end up in the group_by attribute
		paths, diags = request.Plan.PathMatches(ctx, path.MatchRoot("type_definition").AtName(typeDefinitionStr).AtName("rules"))
		if diags.HasError() {
			response.Diagnostics.Append(diags...)
			return
		}

		var rulesPlan types.Set
		diags = request.Plan.GetAttribute(ctx, paths[0], &rulesPlan)
		if diags.HasError() {
			response.Diagnostics.Append(diags...)
			return
		}

		var rulesState types.Set
		diags = request.State.GetAttribute(ctx, paths[0], &rulesState)
		if diags.HasError() {
			response.Diagnostics.Append(diags...)
			return
		}

		if request.ConfigValue.IsUnknown() || request.ConfigValue.IsNull() {
			if !rulesState.Equal(rulesPlan) {
				response.PlanValue = types.ListUnknown(types.StringType)
			} else {
				response.PlanValue = request.StateValue
			}
			return
		}
	}
	response.PlanValue = request.ConfigValue
}

func evaluationDelaySchema() schema.Attribute {
	return schema.Int32Attribute{
		Optional: true,
		Computed: true,
		Default:  int32default.StaticInt32(0),
		Validators: []validator.Int32{
			int32validator.AtLeast(0),
		},
		MarkdownDescription: "Delay evaluation of the rules by n milliseconds. Defaults to 0.",
	}
}

func metricTimeWindowSchema() schema.StringAttribute {
	return schema.StringAttribute{
		Required: true,
		Validators: []validator.String{
			stringvalidator.Any(
				stringvalidator.OneOf(alerttypes.ValidMetricTimeWindowValues...),
				stringvalidator.RegexMatches(regexp.MustCompile(`^(0|(([0-9]+)y)?(([0-9]+)w)?(([0-9]+)d)?(([0-9]+)h)?(([0-9]+)m)?(([0-9]+)s)?(([0-9]+)ms)?)$`), ""),
			),
		},
		MarkdownDescription: fmt.Sprintf("Time window to evaluate the threshold with. Valid values: %q.\nOr having valid time duration - Supported units: y, w, d, h, m, s, ms.\nExamples: `30s`, `1m`, `1h20m15s`, `15d`", alerttypes.ValidMetricTimeWindowValues),
	}
}

func anomalyMetricTimeWindowSchema() schema.StringAttribute {
	return schema.StringAttribute{
		Required: true,
		Validators: []validator.String{
			stringvalidator.OneOf(alerttypes.ValidMetricTimeWindowValues...),
		},
		MarkdownDescription: fmt.Sprintf("Time window to evaluate the threshold with. Valid values: %q.", alerttypes.ValidMetricTimeWindowValues),
	}
}

func logsTimeWindowSchema(validLogsTimeWindowValues []string) schema.StringAttribute {
	return schema.StringAttribute{
		Required: true,
		Validators: []validator.String{
			stringvalidator.OneOf(validLogsTimeWindowValues...),
		},
		MarkdownDescription: fmt.Sprintf("Time window to evaluate the threshold with. Valid values: %q.", validLogsTimeWindowValues),
	}
}

func overrideAlertSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Required: true,
		Attributes: map[string]schema.Attribute{
			"priority": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Validators: []validator.String{
					stringvalidator.OneOf(alerttypes.ValidAlertPriorities...),
				},
				PlanModifiers: []planmodifier.String{
					PriorityOverrideFallback{},
				},
				MarkdownDescription: fmt.Sprintf("Alert priority. Valid values: %q.", alerttypes.ValidAlertPriorities),
			},
		},
	}
}

func timeDurationAttribute() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Required: true,
		Attributes: map[string]schema.Attribute{
			"duration": schema.Int64Attribute{
				Required: true,
			},
			"unit": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.OneOf(alerttypes.ValidDurationUnits...),
				},
			},
		},
	}
}

func sloThresholdRulesAttribute() schema.ListNestedAttribute {
	return schema.ListNestedAttribute{
		Required: true,
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"condition": schema.SingleNestedAttribute{
					Required: true,
					Attributes: map[string]schema.Attribute{
						"threshold": schema.Float64Attribute{
							Required: true,
						},
					},
				},
				"override": overrideAlertSchema(),
			},
		},
	}
}

func logsRatioGroupByForSchema() schema.StringAttribute {
	return schema.StringAttribute{
		Optional: true,
		Computed: true,
		Default:  stringdefault.StaticString("Both"),
		Validators: []validator.String{
			stringvalidator.OneOf(alerttypes.ValidLogsRatioGroupByFor...),
			stringvalidator.AlsoRequires(path.MatchRoot("group_by")),
		},
		MarkdownDescription: fmt.Sprintf("Group by for. Valid values: %q. 'Both' by default.", alerttypes.ValidLogsRatioGroupByFor),
	}
}

func tracingQuerySchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Required: true,
		Attributes: map[string]schema.Attribute{
			"latency_threshold_ms": schema.NumberAttribute{
				Required: true,
			},
			"tracing_label_filters": tracingLabelFiltersSchema(),
		},
	}
}

func tracingLabelFiltersSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Required: true,
		Attributes: map[string]schema.Attribute{
			"application_name": tracingFiltersTypeSchema(),
			"subsystem_name":   tracingFiltersTypeSchema(),
			"service_name":     tracingFiltersTypeSchema(),
			"operation_name":   tracingFiltersTypeSchema(),
			"span_fields":      tracingSpanFieldsFilterSchema(),
		},
	}
}

func tracingFiltersTypeSchema() schema.SetNestedAttribute {
	return schema.SetNestedAttribute{
		Optional: true,
		NestedObject: schema.NestedAttributeObject{
			Attributes: tracingFiltersTypeSchemaAttributes(),
		},
	}
}

func tracingFiltersTypeSchemaAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"values": schema.SetAttribute{
			Required:    true,
			ElementType: types.StringType,
		},
		"operation": schema.StringAttribute{
			Optional: true,
			Computed: true,
			Default:  stringdefault.StaticString("IS"),
			Validators: []validator.String{
				stringvalidator.OneOf(alerttypes.ValidTracingFilterOperations...),
			},
			MarkdownDescription: fmt.Sprintf("Operation. Valid values: %q. 'IS' by default.", alerttypes.ValidTracingFilterOperations),
		},
	}
}

func tracingSpanFieldsFilterSchema() schema.SetNestedAttribute {
	return schema.SetNestedAttribute{
		Optional: true,
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"key": schema.StringAttribute{
					Required: true,
				},
				"filter_type": schema.SingleNestedAttribute{
					Optional:   true,
					Attributes: tracingFiltersTypeSchemaAttributes(),
				},
			},
		},
	}
}

func metricFilterSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Required: true,
		Attributes: map[string]schema.Attribute{
			"promql": schema.StringAttribute{
				Required: true,
			},
		},
	}
}

func logsFilterSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Computed: true,
		PlanModifiers: []planmodifier.Object{
			objectplanmodifier.UseStateForUnknown(),
		},
		Attributes: map[string]schema.Attribute{
			"simple_filter": schema.SingleNestedAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.UseStateForUnknown(),
				},
				Attributes: map[string]schema.Attribute{
					"lucene_query": schema.StringAttribute{
						Optional: true,
					},
					"label_filters": schema.SingleNestedAttribute{
						Optional: true,
						Computed: true,
						Default: objectdefault.StaticValue(types.ObjectValueMust(LabelFiltersAttr(), map[string]attr.Value{
							"application_name": types.SetNull(types.ObjectType{AttrTypes: LabelFilterTypesAttr()}),
							"subsystem_name":   types.SetNull(types.ObjectType{AttrTypes: LabelFilterTypesAttr()}),
							"severities":       types.SetNull(types.StringType),
						})),
						Attributes: map[string]schema.Attribute{
							"application_name": logsAttributeFilterSchema(),
							"subsystem_name":   logsAttributeFilterSchema(),
							"severities": schema.SetAttribute{
								Optional:    true,
								ElementType: types.StringType,
								Validators: []validator.Set{
									setvalidator.ValueStringsAre(
										stringvalidator.OneOf(alerttypes.ValidLogSeverities...),
									),
								},
								MarkdownDescription: fmt.Sprintf("Severities. Valid values: %q.", alerttypes.ValidLogSeverities),
							},
						},
					},
				},
			},
		},
	}
}

func logsAttributeFilterSchema() schema.SetNestedAttribute {
	return schema.SetNestedAttribute{
		Optional: true,
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"value": schema.StringAttribute{
					Required: true,
				},
				"operation": schema.StringAttribute{
					Optional: true,
					Computed: true,
					Default:  stringdefault.StaticString("IS"),
					Validators: []validator.String{
						stringvalidator.OneOf(alerttypes.ValidLogFilterOperationType...),
					},
					MarkdownDescription: fmt.Sprintf("Operation. Valid values: %q.'IS' by default.", alerttypes.ValidLogFilterOperationType),
				},
			},
		},
	}
}

func notificationPayloadFilterSchema() schema.SetAttribute {
	return schema.SetAttribute{
		Optional:    true,
		ElementType: types.StringType,
	}
}

func undetectedValuesManagementSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Computed: true,
		PlanModifiers: []planmodifier.Object{
			objectplanmodifier.UseStateForUnknown(),
		},
		Attributes: map[string]schema.Attribute{
			"trigger_undetected_values": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
			},
			"auto_retire_timeframe": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(alerttypes.AutoRetireTimeframeProtoToSchemaMap[alerts.V3AUTORETIRETIMEFRAME_AUTO_RETIRE_TIMEFRAME_NEVER_OR_UNSPECIFIED]),
				Validators: []validator.String{
					stringvalidator.OneOf(alerttypes.ValidAutoRetireTimeframes...),
				},
				MarkdownDescription: fmt.Sprintf("Auto retire timeframe. Valid values: %q.", alerttypes.ValidAutoRetireTimeframes),
			},
		},
		Default: objectdefault.StaticValue(types.ObjectValueMust(UndetectedValuesManagementAttr(), map[string]attr.Value{
			"trigger_undetected_values": types.BoolValue(false),
			"auto_retire_timeframe":     types.StringValue(alerttypes.AutoRetireTimeframeProtoToSchemaMap[alerts.V3AUTORETIRETIMEFRAME_AUTO_RETIRE_TIMEFRAME_NEVER_OR_UNSPECIFIED]),
		})),
	}
}

func WebhooksSettingsAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"notify_on": types.StringType,
		"retriggering_period": types.ObjectType{
			AttrTypes: RetriggeringPeriodAttr(),
		},
		"integration_id": types.StringType,
		"recipients":     types.SetType{ElemType: types.StringType},
	}
}

func ConfigurationOverridesAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"field_name": types.StringType,
		"template":   types.StringType,
	}
}

func NotificationRouterAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"notify_on": types.StringType,
	}
}

func AlertTypeDefinitionAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"logs_immediate": types.ObjectType{
			AttrTypes: LogsImmediateAttr(),
		},
		"logs_threshold": types.ObjectType{
			AttrTypes: LogsThresholdAttr(),
		},
		"logs_anomaly": types.ObjectType{
			AttrTypes: LogsAnomalyAttr(),
		},
		"logs_ratio_threshold": types.ObjectType{
			AttrTypes: LogsRatioThresholdAttr(),
		},
		"logs_new_value": types.ObjectType{
			AttrTypes: LogsNewValueAttr(),
		},
		"logs_unique_count": types.ObjectType{
			AttrTypes: LogsUniqueCountAttr(),
		},
		"logs_time_relative_threshold": types.ObjectType{
			AttrTypes: LogsTimeRelativeAttr(),
		},
		"metric_threshold": types.ObjectType{
			AttrTypes: MetricThresholdAttr(),
		},
		"metric_anomaly": types.ObjectType{
			AttrTypes: MetricAnomalyAttr(),
		},
		"tracing_immediate": types.ObjectType{
			AttrTypes: TracingImmediateAttr(),
		},
		"tracing_threshold": types.ObjectType{
			AttrTypes: TracingThresholdAttr(),
		},
		"flow": types.ObjectType{
			AttrTypes: FlowAttr(),
		},
		"slo_threshold": types.ObjectType{
			AttrTypes: SloThresholdAttr(),
		},
	}
}

func LogsImmediateAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"logs_filter": types.ObjectType{
			AttrTypes: LogsFilterAttr(),
		},
		"notification_payload_filter": types.SetType{
			ElemType: types.StringType,
		},
	}
}

func LogsFilterAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"simple_filter": types.ObjectType{
			AttrTypes: LuceneFilterAttr(),
		},
	}
}

func LuceneFilterAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"lucene_query": types.StringType,
		"label_filters": types.ObjectType{
			AttrTypes: LabelFiltersAttr(),
		},
	}
}

func LabelFiltersAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"application_name": types.SetType{
			ElemType: types.ObjectType{
				AttrTypes: LabelFilterTypesAttr(),
			},
		},
		"subsystem_name": types.SetType{
			ElemType: types.ObjectType{
				AttrTypes: LabelFilterTypesAttr(),
			},
		},
		"severities": types.SetType{
			ElemType: types.StringType,
		},
	}
}

func LogsThresholdAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"logs_filter":                  types.ObjectType{AttrTypes: LogsFilterAttr()},
		"notification_payload_filter":  types.SetType{ElemType: types.StringType},
		"rules":                        types.SetType{ElemType: types.ObjectType{AttrTypes: LogsThresholdRulesAttr()}},
		"undetected_values_management": types.ObjectType{AttrTypes: UndetectedValuesManagementAttr()},
		"custom_evaluation_delay":      types.Int32Type,
	}
}

func LogsThresholdRulesAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"condition": types.ObjectType{AttrTypes: LogsThresholdConditionAttr()},
		"override":  types.ObjectType{AttrTypes: AlertOverrideAttr()},
	}
}

func LogsThresholdConditionAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"threshold":      types.Float64Type,
		"time_window":    types.StringType,
		"condition_type": types.StringType,
	}
}

func LogsAnomalyAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"logs_filter":                 types.ObjectType{AttrTypes: LogsFilterAttr()},
		"rules":                       types.SetType{ElemType: types.ObjectType{AttrTypes: LogsAnomalyRulesAttr()}},
		"notification_payload_filter": types.SetType{ElemType: types.StringType},
		"custom_evaluation_delay":     types.Int32Type,
		"percentage_of_deviation":     types.Float64Type,
	}
}

func LogsAnomalyRulesAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"condition": types.ObjectType{AttrTypes: LogsAnomalyConditionAttr()},
	}
}

func LogsAnomalyConditionAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"minimum_threshold": types.Float64Type,
		"time_window":       types.StringType,
		"condition_type":    types.StringType,
	}
}

func LogsRatioThresholdAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"numerator":         types.ObjectType{AttrTypes: LogsFilterAttr()},
		"numerator_alias":   types.StringType,
		"denominator":       types.ObjectType{AttrTypes: LogsFilterAttr()},
		"denominator_alias": types.StringType,
		"rules":             types.SetType{ElemType: types.ObjectType{AttrTypes: LogsRatioThresholdRulesAttr()}},
		"ignore_infinity":   types.BoolType,
		"notification_payload_filter": types.SetType{
			ElemType: types.StringType,
		},
		"group_by_for":            types.StringType,
		"custom_evaluation_delay": types.Int32Type,
	}
}

func LogsRatioThresholdRulesAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"condition": types.ObjectType{AttrTypes: LogsRatioThresholdRuleConditionAttr()},
		"override":  types.ObjectType{AttrTypes: AlertOverrideAttr()},
	}
}

func LogsRatioThresholdRuleConditionAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"threshold":      types.Float64Type,
		"time_window":    types.StringType,
		"condition_type": types.StringType,
	}
}

func AlertOverrideAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"priority": types.StringType,
	}
}

func LogsNewValueAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"logs_filter":                 types.ObjectType{AttrTypes: LogsFilterAttr()},
		"rules":                       types.SetType{ElemType: types.ObjectType{AttrTypes: LogsNewValueRulesAttr()}},
		"notification_payload_filter": types.SetType{ElemType: types.StringType},
	}
}

func LogsNewValueRulesAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"condition": types.ObjectType{AttrTypes: LogsNewValueConditionAttr()},
	}
}

func LogsNewValueConditionAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"time_window":      types.StringType,
		"keypath_to_track": types.StringType,
	}
}

func UndetectedValuesManagementAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"trigger_undetected_values": types.BoolType,
		"auto_retire_timeframe":     types.StringType,
	}
}

func LogsUniqueCountAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"logs_filter":                       types.ObjectType{AttrTypes: LogsFilterAttr()},
		"notification_payload_filter":       types.SetType{ElemType: types.StringType},
		"rules":                             types.SetType{ElemType: types.ObjectType{AttrTypes: LogsUniqueCountRulesAttr()}},
		"unique_count_keypath":              types.StringType,
		"max_unique_count_per_group_by_key": types.Int64Type,
	}
}

func LogsUniqueCountRulesAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"condition": types.ObjectType{AttrTypes: LogsUniqueCountConditionAttr()},
	}
}

func LogsUniqueCountConditionAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"max_unique_count": types.Int64Type,
		"time_window":      types.StringType,
	}
}

func AlertScheduleAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"active_on": types.ObjectType{
			AttrTypes: AlertScheduleActiveOnAttr(),
		},
	}
}

func AlertScheduleActiveOnAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"days_of_week": types.SetType{
			ElemType: types.StringType,
		},
		"start_time": types.StringType,
		"end_time":   types.StringType,
		"utc_offset": types.StringType,
	}
}

func LogsTimeRelativeAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"logs_filter":                  types.ObjectType{AttrTypes: LogsFilterAttr()},
		"notification_payload_filter":  types.SetType{ElemType: types.StringType},
		"undetected_values_management": types.ObjectType{AttrTypes: UndetectedValuesManagementAttr()},
		"rules":                        types.SetType{ElemType: types.ObjectType{AttrTypes: LogsTimeRelativeRulesAttr()}},
		"custom_evaluation_delay":      types.Int32Type,
		"ignore_infinity":              types.BoolType,
	}
}

func LogsTimeRelativeRulesAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"condition": types.ObjectType{
			AttrTypes: LogsTimeRelativeConditionAttr(),
		},
		"override": types.ObjectType{
			AttrTypes: AlertOverrideAttr(),
		},
	}
}

func LogsTimeRelativeConditionAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"threshold":      types.Float64Type,
		"compared_to":    types.StringType,
		"condition_type": types.StringType,
	}
}

func MetricThresholdAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"metric_filter":                types.ObjectType{AttrTypes: MetricFilterAttr()},
		"undetected_values_management": types.ObjectType{AttrTypes: UndetectedValuesManagementAttr()},
		"rules":                        types.SetType{ElemType: types.ObjectType{AttrTypes: MetricThresholdRulesAttr()}},
		"missing_values":               types.ObjectType{AttrTypes: MissingValuesAttr()},
		"custom_evaluation_delay":      types.Int32Type,
	}
}

func MissingValuesAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"replace_with_zero":       types.BoolType,
		"min_non_null_values_pct": types.Int64Type,
	}
}

func MetricThresholdRulesAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"condition": types.ObjectType{
			AttrTypes: MetricThresholdConditionAttr(),
		},
		"override": types.ObjectType{
			AttrTypes: AlertOverrideAttr(),
		},
	}
}

func MetricThresholdConditionAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"threshold":      types.Float64Type,
		"for_over_pct":   types.Int64Type,
		"of_the_last":    types.StringType,
		"condition_type": types.StringType,
	}
}

func MetricFilterAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"promql": types.StringType,
	}
}

func MetricAnomalyAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"metric_filter":           types.ObjectType{AttrTypes: MetricFilterAttr()},
		"rules":                   types.SetType{ElemType: types.ObjectType{AttrTypes: MetricAnomalyRulesAttr()}},
		"custom_evaluation_delay": types.Int32Type,
		"percentage_of_deviation": types.Float64Type,
	}
}

func MetricAnomalyRulesAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"condition": types.ObjectType{AttrTypes: MetricAnomalyConditionAttr()},
	}
}

func MetricAnomalyConditionAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"min_non_null_values_pct": types.Int64Type,
		"threshold":               types.Float64Type,
		"for_over_pct":            types.Int64Type,
		"of_the_last":             types.StringType,
		"condition_type":          types.StringType,
	}
}

func TracingImmediateAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"tracing_filter":              types.ObjectType{AttrTypes: TracingQueryAttr()},
		"notification_payload_filter": types.SetType{ElemType: types.StringType},
	}
}

func TracingThresholdAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"tracing_filter":              types.ObjectType{AttrTypes: TracingQueryAttr()},
		"notification_payload_filter": types.SetType{ElemType: types.StringType},
		"rules":                       types.SetType{ElemType: types.ObjectType{AttrTypes: TracingThresholdRulesAttr()}},
	}
}

func TracingThresholdRulesAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"condition": types.ObjectType{AttrTypes: TracingThresholdConditionAttr()},
	}
}

func TracingThresholdConditionAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"span_amount":    types.Float64Type,
		"time_window":    types.StringType,
		"condition_type": types.StringType,
	}
}

func FlowAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"stages": types.ListType{
			ElemType: types.ObjectType{
				AttrTypes: FlowStageAttr(),
			},
		},
		"enforce_suppression": types.BoolType,
	}
}

func FlowStageAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"flow_stages_groups": types.ListType{
			ElemType: types.ObjectType{
				AttrTypes: FlowStageGroupAttr(),
			},
		},
		"timeframe_ms":   types.Int64Type,
		"timeframe_type": types.StringType,
	}
}

func FlowStageGroupAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"alert_defs": types.SetType{
			ElemType: types.ObjectType{
				AttrTypes: AlertDefsAttr(),
			},
		},
		"next_op":   types.StringType,
		"alerts_op": types.StringType,
	}
}

func AlertDefsAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"id":  types.StringType,
		"not": types.BoolType,
	}
}

func SloThresholdAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"slo_definition": types.ObjectType{AttrTypes: SloDefinitionAttr()},
		"error_budget":   types.ObjectType{AttrTypes: SloErrorBudgetAttr()},
		"burn_rate":      types.ObjectType{AttrTypes: SloBurnRateAttr()},
	}
}

func SloDefinitionAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"slo_id": types.StringType,
	}
}

func SloErrorBudgetAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"rules": types.ListType{
			ElemType: types.ObjectType{AttrTypes: SloThresholdRuleAttr()},
		},
	}
}

func SloBurnRateAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"rules": types.ListType{
			ElemType: types.ObjectType{AttrTypes: SloThresholdRuleAttr()},
		},
		"dual":   types.ObjectType{AttrTypes: SloDurationWrapperAttr()},
		"single": types.ObjectType{AttrTypes: SloDurationWrapperAttr()},
	}
}

func SloDurationWrapperAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"time_duration": types.ObjectType{AttrTypes: SloDurationAttr()},
	}
}

func SloDurationAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"duration": types.Int64Type,
		"unit":     types.StringType,
	}
}

func SloThresholdRuleAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"condition": types.ObjectType{AttrTypes: SloThresholdConditionAttr()},
		"override":  types.ObjectType{AttrTypes: AlertOverrideAttr()},
	}
}

func SloThresholdConditionAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"threshold": types.Float64Type,
	}
}

func TracingQueryAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"latency_threshold_ms":  types.NumberType,
		"tracing_label_filters": types.ObjectType{AttrTypes: TracingLabelFiltersAttr()},
	}
}

func LabelFilterTypesAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"value":     types.StringType,
		"operation": types.StringType,
	}
}

func TracingLabelFiltersAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"application_name": types.SetType{ElemType: types.ObjectType{AttrTypes: TracingFiltersTypeAttr()}},
		"subsystem_name":   types.SetType{ElemType: types.ObjectType{AttrTypes: TracingFiltersTypeAttr()}},
		"service_name":     types.SetType{ElemType: types.ObjectType{AttrTypes: TracingFiltersTypeAttr()}},
		"operation_name":   types.SetType{ElemType: types.ObjectType{AttrTypes: TracingFiltersTypeAttr()}},
		"span_fields":      types.SetType{ElemType: types.ObjectType{AttrTypes: TracingSpanFieldsFilterAttr()}},
	}
}

func TracingFiltersTypeAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"operation": types.StringType,
		"values":    types.SetType{ElemType: types.StringType},
	}
}

func TracingSpanFieldsFilterAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"key":         types.StringType,
		"filter_type": types.ObjectType{AttrTypes: TracingFiltersTypeAttr()},
	}
}

func RetriggeringPeriodAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"minutes": types.Int64Type,
	}
}

func IncidentsSettingsAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"notify_on": types.StringType,
		"retriggering_period": types.ObjectType{
			AttrTypes: RetriggeringPeriodAttr(),
		},
	}
}
