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
	dashboardschema "github.com/coralogix/terraform-provider-coralogix/internal/provider/dashboards/dashboard_schema"

	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	"github.com/hashicorp/terraform-plugin-framework-validators/int32validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
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

const priorityDeprecationMessage = "This field will be removed in the future in favor of the 'override' property where possible."

const noDataPolicyUnspecifiedDeprecationMessage = "`UNSPECIFIED` is a protobuf sentinel, not a UI option. " +
	"Omit `no_data_policy` entirely for the same legacy no-data handling, or set `state` to one of " +
	"`OK`, `ALERTING`, `KEEP_LAST`, or `NO_DATA`. `UNSPECIFIED` will be rejected in a future provider release."

type PriorityDeprecationWarning struct {
}

func (p PriorityDeprecationWarning) Description(ctx context.Context) string {
	return "Warns that priority is deprecated for alert types that support an override block."
}

func (p PriorityDeprecationWarning) MarkdownDescription(ctx context.Context) string {
	return "Warns that priority is deprecated for alert types that support an override block."
}

func (p PriorityDeprecationWarning) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	paths, diags := req.Config.PathMatches(ctx, path.MatchRoot("type_definition"))
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	var typeDefinition types.Object
	diags = req.Config.GetAttribute(ctx, paths[0], &typeDefinition)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	if typeDefinition.IsNull() || typeDefinition.IsUnknown() {
		return
	}

	var typeDefinitionModel alerttypes.AlertTypeDefinitionModel
	if diags = typeDefinition.As(ctx, &typeDefinitionModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	if !utils.ObjIsNullOrUnknown(typeDefinitionModel.LogsThreshold) ||
		!utils.ObjIsNullOrUnknown(typeDefinitionModel.LogsRatioThreshold) ||
		!utils.ObjIsNullOrUnknown(typeDefinitionModel.LogsTimeRelativeThreshold) ||
		!utils.ObjIsNullOrUnknown(typeDefinitionModel.MetricThreshold) ||
		!utils.ObjIsNullOrUnknown(typeDefinitionModel.SloThreshold) ||
		!utils.ObjIsNullOrUnknown(typeDefinitionModel.AnalyticsThreshold) {
		resp.Diagnostics.AddAttributeWarning(req.Path, "Deprecated Attribute", priorityDeprecationMessage)
	}
}

// NoDataPolicyUnspecifiedDeprecationWarning warns when config uses the protobuf UNSPECIFIED sentinel.
type NoDataPolicyUnspecifiedDeprecationWarning struct{}

func (n NoDataPolicyUnspecifiedDeprecationWarning) Description(ctx context.Context) string {
	return "Warns that no_data_policy.state = UNSPECIFIED is deprecated."
}

func (n NoDataPolicyUnspecifiedDeprecationWarning) MarkdownDescription(ctx context.Context) string {
	return "Warns that `no_data_policy.state = \"UNSPECIFIED\"` is deprecated."
}

func (n NoDataPolicyUnspecifiedDeprecationWarning) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	if req.ConfigValue.ValueString() == "UNSPECIFIED" {
		resp.Diagnostics.AddAttributeWarning(req.Path, "Deprecated Attribute", noDataPolicyUnspecifiedDeprecationMessage)
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

// preserveStateForEquivalentJSON keeps the prior state text when the planned
// policy is semantically identical JSON under a different formatting. The
// backend returns its own key order, while configurations are usually written
// with jsonencode; without this, a reordered-but-equal policy would plan a
// diff on every run. It reuses the dashboard package's plan modifier: the
// logic is identical, so there is exactly one implementation (hoisting it to
// internal/utils would export framework types from a utils package; a second
// copy here would drift). If a fourth consumer appears, hoist then.
var preserveStateForEquivalentJSON = dashboardschema.PreserveStateForEquivalentJSON{}

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
	} else if !utils.ObjIsNullOrUnknown(typeDefinition.SloThreshold) {
		typeDefinitionStr = "slo_threshold"
	} else if !utils.ObjIsNullOrUnknown(typeDefinition.TracingThreshold) {
		typeDefinitionStr = "tracing_threshold"
	} else if !utils.ObjIsNullOrUnknown(typeDefinition.Flow) {
		typeDefinitionStr = "flow"
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
	case "slo_threshold", "tracing_threshold", "flow":
		// Backend may return group_by (e.g. from SLO grouping) even when not set in config.
		// When state has no value yet, use unknown so Terraform accepts whatever the API returns
		// after apply (avoids "Provider produced inconsistent result"). When state already has a
		// value, use it so the next plan is empty (avoids test failure "plan was not empty").
		if request.ConfigValue.IsUnknown() || request.ConfigValue.IsNull() {
			if request.StateValue.IsNull() || request.StateValue.IsUnknown() {
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
		Validators: []validator.Int32{
			int32validator.AtLeast(0),
		},
		MarkdownDescription: "Delay evaluation of the rules by n milliseconds. When omitted, the provider does not send a custom evaluation delay.",
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
			objectplanmodifier.UseNonNullStateForUnknown(),
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
						Computed: true,
						Default:  stringdefault.StaticString(""),
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
		MarkdownDescription: "Manage triggering on undetected values. `trigger_undetected_values` requires at least one rule with a `LESS_THAN` condition type.",
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

// analyticsNoDataPolicyNotEmpty rejects a no_data_policy block that sets none of
// its (all-optional) children. Such a block would otherwise round-trip as null on
// read — extractNoDataPolicy collapses an all-null policy to nil and the API omits
// it — and fail apply with "inconsistent result after apply: object, but now null".
// Removing the block entirely is the supported way to leave the policy unset.
type analyticsNoDataPolicyNotEmpty struct{}

func (v analyticsNoDataPolicyNotEmpty) Description(ctx context.Context) string {
	return "no_data_policy must set at least one attribute; remove the block to leave the policy unset."
}

func (v analyticsNoDataPolicyNotEmpty) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v analyticsNoDataPolicyNotEmpty) ValidateObject(ctx context.Context, req validator.ObjectRequest, resp *validator.ObjectResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	var model alerttypes.NoDataPolicyModel
	if diags := req.ConfigValue.As(ctx, &model, basetypes.ObjectAsOptions{}); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	// An unknown child (variable-derived) leaves IsNull() false, so this only
	// fires when every child is explicitly absent.
	if model.State.IsNull() && model.AutoRetireSeconds.IsNull() {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Empty no_data_policy",
			"no_data_policy must set at least one of state or auto_retire_seconds; remove the block to leave the policy unset.",
		)
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

func UndetectedValuesManagementAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"trigger_undetected_values": types.BoolType,
		"auto_retire_timeframe":     types.StringType,
	}
}

func LabelFilterTypesAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"value":     types.StringType,
		"operation": types.StringType,
	}
}

func RetriggeringPeriodAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"minutes": types.Int64Type,
	}
}
