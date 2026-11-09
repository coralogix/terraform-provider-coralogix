// Copyright 2026 Coralogix Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package alerts

import (
	"context"
	"maps"
	"slices"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const priorityDeprecationMessage = "This field will be removed in the future in favor of the 'override' property where possible."

const noDataPolicyUnspecifiedDeprecationMessage = "`UNSPECIFIED` is a protobuf sentinel, not a UI option. " +
	"Omit `no_data_policy` entirely for the same legacy no-data handling, or set `state` to one of " +
	"`OK`, `ALERTING`, `KEEP_LAST`, or `NO_DATA`. `UNSPECIFIED` will be rejected in a future provider release."

// arms of type_definition that have an override block in each rule, so the top level priority is
// deprecated for them.
var armsWithOverride = []string{"logs_threshold", "logs_ratio_threshold", "logs_time_relative_threshold", "metric_threshold", "slo_threshold", "analytics_threshold"}

// arms of type_definition that take no group_by.
var armsWithoutGroupBy = []string{"logs_immediate", "logs_new_value", "tracing_immediate"}

// analytics arms of type_definition, whose no_data_policy must not be empty.
var analyticsArms = []string{"analytics_immediate", "analytics_threshold"}

// validateAlertConfig checks what no attribute validator of the generated schema states.
func validateAlertConfig(ctx context.Context, config tfsdk.Config) diag.Diagnostics {
	var diags diag.Diagnostics
	var typeDefinition types.Object
	diags.Append(config.GetAttribute(ctx, path.Root("type_definition"), &typeDefinition)...)
	if diags.HasError() {
		return diags
	}
	diags.Append(warnPriorityDeprecated(ctx, config, typeDefinition)...)
	diags.Append(checkGroupBy(ctx, config, typeDefinition)...)
	diags.Append(checkGroupByFor(ctx, config, typeDefinition)...)
	diags.Append(warnNoDataPolicyUnspecified(typeDefinition)...)
	diags.Append(checkAnalyticsNoDataPolicy(typeDefinition)...)
	diags.Append(checkAnalyticsRuleCount(ctx, config, typeDefinition)...)
	diags.Append(checkWebhookTargets(ctx, config)...)
	return diags
}

// armIsSet reports whether type_definition sets the arm. An arm with an unknown value does not count.
func armIsSet(typeDefinition types.Object, arm string) bool {
	if typeDefinition.IsNull() || typeDefinition.IsUnknown() {
		return false
	}
	value, ok := typeDefinition.Attributes()[arm].(types.Object)
	return ok && !value.IsNull() && !value.IsUnknown()
}

func anyArmIsSet(typeDefinition types.Object, arms []string) bool {
	for _, arm := range arms {
		if armIsSet(typeDefinition, arm) {
			return true
		}
	}
	return false
}

// warnPriorityDeprecated warns about the top level priority of an alert type that has a rule override.
func warnPriorityDeprecated(ctx context.Context, config tfsdk.Config, typeDefinition types.Object) diag.Diagnostics {
	var priority types.String
	diags := config.GetAttribute(ctx, path.Root("priority"), &priority)
	if diags.HasError() || priority.IsNull() || priority.IsUnknown() {
		return diags
	}
	if anyArmIsSet(typeDefinition, armsWithOverride) {
		diags.AddAttributeWarning(path.Root("priority"), "Deprecated Attribute", priorityDeprecationMessage)
	}
	return diags
}

// checkGroupBy rejects group_by for the alert types that cannot group.
func checkGroupBy(ctx context.Context, config tfsdk.Config, typeDefinition types.Object) diag.Diagnostics {
	var groupBy types.List
	diags := config.GetAttribute(ctx, path.Root("group_by"), &groupBy)
	if diags.HasError() || groupBy.IsNull() || groupBy.IsUnknown() {
		return diags
	}
	if anyArmIsSet(typeDefinition, armsWithoutGroupBy) {
		diags.AddError("group_by", "Group by is not allowed for logs_immediate, logs_new_value, tracing_immediate alert types.")
	}
	return diags
}

// checkGroupByFor requires group_by when logs_ratio_threshold sets group_by_for.
func checkGroupByFor(ctx context.Context, config tfsdk.Config, typeDefinition types.Object) diag.Diagnostics {
	if !armIsSet(typeDefinition, "logs_ratio_threshold") {
		return nil
	}
	groupByFor := path.Root("type_definition").AtName("logs_ratio_threshold").AtName("group_by_for")
	var value types.String
	diags := config.GetAttribute(ctx, groupByFor, &value)
	if diags.HasError() {
		return diags
	}
	request := validator.StringRequest{Config: config, ConfigValue: value, Path: groupByFor, PathExpression: groupByFor.Expression()}
	response := &validator.StringResponse{}
	stringvalidator.AlsoRequires(path.MatchRoot("group_by")).ValidateString(ctx, request, response)
	diags.Append(response.Diagnostics...)
	return diags
}

// noDataPolicyState is the configured state of the no_data_policy of an arm.
type noDataPolicyState struct {
	path  path.Path
	state types.String
}

// noDataPolicyStates returns the configured no_data_policy.state of every arm that has a no_data_policy.
func noDataPolicyStates(typeDefinition types.Object) []noDataPolicyState {
	if typeDefinition.IsNull() || typeDefinition.IsUnknown() {
		return nil
	}
	var out []noDataPolicyState
	attrs := typeDefinition.Attributes()
	for _, arm := range slices.Sorted(maps.Keys(attrs)) {
		armObject, ok := attrs[arm].(types.Object)
		if !ok || armObject.IsNull() || armObject.IsUnknown() {
			continue
		}
		policy, ok := armObject.Attributes()["no_data_policy"].(types.Object)
		if !ok || policy.IsNull() || policy.IsUnknown() {
			continue
		}
		if state, ok := policy.Attributes()["state"].(types.String); ok {
			statePath := path.Root("type_definition").AtName(arm).AtName("no_data_policy").AtName("state")
			out = append(out, noDataPolicyState{path: statePath, state: state})
		}
	}
	return out
}

// warnNoDataPolicyUnspecified warns about the deprecated UNSPECIFIED sentinel as a no-data state.
func warnNoDataPolicyUnspecified(typeDefinition types.Object) diag.Diagnostics {
	var diags diag.Diagnostics
	for _, policy := range noDataPolicyStates(typeDefinition) {
		if !policy.state.IsNull() && !policy.state.IsUnknown() && policy.state.ValueString() == "UNSPECIFIED" {
			diags.AddAttributeWarning(policy.path, "Deprecated Attribute", noDataPolicyUnspecifiedDeprecationMessage)
		}
	}
	return diags
}

// checkAnalyticsNoDataPolicy rejects a no_data_policy block of an analytics alert that sets none of its
// attributes. The block would read back as null and fail the apply. Removing the block leaves the policy
// unset. An unknown attribute is not null, so only a block whose attributes are all absent fails.
func checkAnalyticsNoDataPolicy(typeDefinition types.Object) diag.Diagnostics {
	var diags diag.Diagnostics
	for _, arm := range analyticsArms {
		if !armIsSet(typeDefinition, arm) {
			continue
		}
		policy, ok := typeDefinition.Attributes()[arm].(types.Object).Attributes()["no_data_policy"].(types.Object)
		if !ok || policy.IsNull() || policy.IsUnknown() || !allNull(policy.Attributes()) {
			continue
		}
		diags.AddAttributeError(
			path.Root("type_definition").AtName(arm).AtName("no_data_policy"),
			"Empty no_data_policy",
			"no_data_policy must set at least one of state or auto_retire_seconds; remove the block to leave the policy unset.",
		)
	}
	return diags
}

func allNull(attrs map[string]attr.Value) bool {
	for _, value := range attrs {
		if !value.IsNull() {
			return false
		}
	}
	return true
}

// maxAnalyticsRules is the number of rules that an analytics_threshold alert takes.
const maxAnalyticsRules = 5

// checkAnalyticsRuleCount rejects more rules than the API takes. The generated schema checks the minimum.
func checkAnalyticsRuleCount(ctx context.Context, config tfsdk.Config, typeDefinition types.Object) diag.Diagnostics {
	if !armIsSet(typeDefinition, "analytics_threshold") {
		return nil
	}
	rulesPath := path.Root("type_definition").AtName("analytics_threshold").AtName("rules")
	var rules types.List
	diags := config.GetAttribute(ctx, rulesPath, &rules)
	if diags.HasError() || rules.IsNull() || rules.IsUnknown() || len(rules.Elements()) <= maxAnalyticsRules {
		return diags
	}
	request := validator.ListRequest{Config: config, ConfigValue: rules, Path: rulesPath, PathExpression: rulesPath.Expression()}
	response := &validator.ListResponse{}
	listvalidator.SizeBetween(1, maxAnalyticsRules).ValidateList(ctx, request, response)
	diags.Append(response.Diagnostics...)
	return diags
}

// checkWebhookTargets requires each webhook to name an integration or recipients. The generated schema
// rejects both together, and this rejects neither.
func checkWebhookTargets(ctx context.Context, config tfsdk.Config) diag.Diagnostics {
	webhooksPath := path.Root("notification_group").AtName("webhooks_settings")
	var webhooks types.Set
	diags := config.GetAttribute(ctx, webhooksPath, &webhooks)
	if diags.HasError() || webhooks.IsNull() || webhooks.IsUnknown() {
		return diags
	}
	for _, element := range webhooks.Elements() {
		webhook, ok := element.(types.Object)
		if !ok || webhook.IsNull() || webhook.IsUnknown() {
			continue
		}
		integration, recipients := webhook.Attributes()["integration_id"], webhook.Attributes()["recipients"]
		if integration == nil || recipients == nil || !integration.IsNull() || !recipients.IsNull() {
			continue
		}
		diags.AddAttributeError(webhooksPath.AtSetValue(element).AtName("integration_id"), "Invalid Attribute Combination",
			"No attribute specified when one (and only one) of [integration_id, recipients] is required")
	}
	return diags
}
