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

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// modifyAlertPlan plans what no plan modifier of one attribute can state: group_by, which some alert
// types compute, and the override priority of a rule, which falls back to the priority of the alert.
func modifyAlertPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	resp.Diagnostics.Append(planGroupBy(ctx, req, resp)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(planOverridePriority(ctx, req, resp)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(planWebhookRetriggering(req, resp)...)
}

// groupByArms are the arms of type_definition for which the API computes group_by.
var groupByArms = []string{"metric_threshold", "metric_anomaly", "logs_new_value", "slo_threshold", "tracing_threshold", "flow"}

// groupByInput is the attribute of the arm whose change makes the API compute group_by again.
var groupByInput = map[string][]string{
	"metric_threshold": {"type_definition", "metric_threshold", "metric_filter", "promql"},
	"metric_anomaly":   {"type_definition", "metric_anomaly", "metric_filter", "promql"},
	"logs_new_value":   {"type_definition", "logs_new_value", "rules"},
}

// planGroupBy plans group_by when the configuration leaves it out:
//   - metric_threshold, metric_anomaly (the promql) and logs_new_value (the rules): the API derives
//     group_by from them, so a change makes it unknown and no change keeps the state;
//   - slo_threshold, tracing_threshold, flow: the API may return a group_by that the configuration does not
//     have, so it is unknown until the state holds one;
//   - any other type: the planned value is the configured one, null.
func planGroupBy(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) diag.Diagnostics {
	groupByPath := path.Root("group_by")
	var config, state types.List
	diags := req.Config.GetAttribute(ctx, groupByPath, &config)
	diags.Append(req.State.GetAttribute(ctx, groupByPath, &state)...)
	if diags.HasError() || !(config.IsNull() || config.IsUnknown()) {
		return diags
	}
	var typeDefinition types.Object
	diags.Append(resp.Plan.GetAttribute(ctx, path.Root("type_definition"), &typeDefinition)...)
	if diags.HasError() {
		return diags
	}
	planned := config
	if arm := firstSetArm(typeDefinition, groupByArms); arm != "" {
		input, hasInput := groupByInput[arm]
		var changed bool
		if hasInput {
			changed = valueChanged(req.State.Raw, resp.Plan.Raw, input)
		} else {
			changed = state.IsNull() || state.IsUnknown()
		}
		planned = state
		if changed {
			planned = types.ListUnknown(types.StringType)
		}
	}
	diags.Append(resp.Plan.SetAttribute(ctx, groupByPath, planned)...)
	return diags
}

// firstSetArm returns the first of the arms that type_definition sets, and "" when it sets none.
func firstSetArm(typeDefinition types.Object, arms []string) string {
	for _, arm := range arms {
		if armIsSet(typeDefinition, arm) {
			return arm
		}
	}
	return ""
}

// valueChanged reports whether the plan holds another value than the state at the attribute names.
func valueChanged(state, plan tftypes.Value, names []string) bool {
	steps := make([]tftypes.AttributePathStep, len(names))
	for i, name := range names {
		steps[i] = tftypes.AttributeName(name)
	}
	planned, plannedOK := valueAtAttributes(plan, steps)
	current, currentOK := valueAtAttributes(state, steps)
	if plannedOK != currentOK {
		return true
	}
	return !planned.Equal(current)
}

// planOverridePriority plans the override priority of a rule that the configuration leaves out: the
// priority of the alert, or P5 when the alert sets none. The rules are a set or a list, and the
// plan, the configuration, and the state order a set apart, so it rebuilds each rule as a whole and
// takes the configured rule that the planned one matches.
func planOverridePriority(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) diag.Diagnostics {
	var topLevel types.String
	diags := req.Config.GetAttribute(ctx, path.Root("priority"), &topLevel)
	if diags.HasError() {
		return diags
	}
	fallback := types.StringValue("P5")
	if !topLevel.IsNull() {
		fallback = topLevel
	}
	if fallback.IsUnknown() {
		return diags
	}
	value, err := fallback.ToTerraformValue(ctx)
	if err != nil {
		diags.AddError("Invalid priority", err.Error())
		return diags
	}
	planned, err := tftypes.Transform(resp.Plan.Raw, func(p *tftypes.AttributePath, v tftypes.Value) (tftypes.Value, error) {
		if !isRuleItem(p) {
			return v, nil
		}
		return withFallbackPriority(req.Config.Raw, p, v, value), nil
	})
	if err != nil {
		diags.AddError("Unable to plan the override priority", err.Error())
		return diags
	}
	resp.Plan.Raw = planned
	return diags
}

// isRuleItem reports whether p is an item of a rules collection of type_definition.
func isRuleItem(p *tftypes.AttributePath) bool {
	steps := p.Steps()
	if len(steps) < 4 || steps[0] != tftypes.AttributePathStep(tftypes.AttributeName("type_definition")) {
		return false
	}
	switch steps[len(steps)-1].(type) {
	case tftypes.ElementKeyValue, tftypes.ElementKeyInt:
	default:
		return false
	}
	return steps[len(steps)-2] == tftypes.AttributePathStep(tftypes.AttributeName("rules"))
}

// withFallbackPriority returns the planned rule with override.priority set to fallback when the rule
// has an override whose priority is unknown in the plan and null in the configuration.
func withFallbackPriority(config tftypes.Value, p *tftypes.AttributePath, rule, fallback tftypes.Value) tftypes.Value {
	override, ok := overrideOf(rule)
	if !ok || override.priority.IsKnown() || !priorityIsUnconfigured(config, p, rule) {
		return rule
	}
	overrideAttrs := maps.Clone(override.object.attrs)
	overrideAttrs["priority"] = fallback
	ruleAttrs := maps.Clone(override.rule)
	ruleAttrs["override"] = tftypes.NewValue(override.object.typ, overrideAttrs)
	return tftypes.NewValue(rule.Type(), ruleAttrs)
}

type objectParts struct {
	typ   tftypes.Type
	attrs map[string]tftypes.Value
}

type overrideParts struct {
	rule     map[string]tftypes.Value
	object   objectParts
	priority tftypes.Value
}

// overrideOf returns the parts of the override object of a planned rule.
func overrideOf(rule tftypes.Value) (overrideParts, bool) {
	var ruleAttrs map[string]tftypes.Value
	if !rule.IsKnown() || rule.IsNull() || rule.As(&ruleAttrs) != nil {
		return overrideParts{}, false
	}
	override, found := ruleAttrs["override"]
	if !found || !override.IsKnown() || override.IsNull() {
		return overrideParts{}, false
	}
	var overrideAttrs map[string]tftypes.Value
	if override.As(&overrideAttrs) != nil {
		return overrideParts{}, false
	}
	priority, found := overrideAttrs["priority"]
	if !found {
		return overrideParts{}, false
	}
	return overrideParts{rule: ruleAttrs, object: objectParts{typ: override.Type(), attrs: overrideAttrs}, priority: priority}, true
}

// priorityIsUnconfigured reports whether the configuration leaves out the override priority of the
// configured rules that the planned rule matches. A matching rule that sets it, also with a value that
// is not known yet, wins over one that leaves it out.
func priorityIsUnconfigured(config tftypes.Value, p *tftypes.AttributePath, rule tftypes.Value) bool {
	steps := p.Steps()
	rules, ok := valueAtAttributes(config, steps[:len(steps)-1])
	if !ok || !rules.IsKnown() || rules.IsNull() {
		return false
	}
	var items []tftypes.Value
	if rules.As(&items) != nil {
		return false
	}
	if _, isList := rules.Type().(tftypes.List); isList {
		index, isIndex := steps[len(steps)-1].(tftypes.ElementKeyInt)
		if !isIndex || int(index) >= len(items) {
			return false
		}
		items = items[index : index+1]
	}
	unconfigured := false
	for _, item := range items {
		if !plannedFromConfig(item, rule) {
			continue
		}
		override, ok := overrideOf(item)
		if !ok {
			continue
		}
		if !override.priority.IsKnown() || !override.priority.IsNull() {
			return false
		}
		unconfigured = true
	}
	return unconfigured
}

// valueAtAttributes returns the value that the attribute steps lead to.
func valueAtAttributes(v tftypes.Value, steps []tftypes.AttributePathStep) (tftypes.Value, bool) {
	for _, step := range steps {
		name, ok := step.(tftypes.AttributeName)
		if !ok || !v.IsKnown() || v.IsNull() {
			return tftypes.Value{}, false
		}
		var attrs map[string]tftypes.Value
		if v.As(&attrs) != nil {
			return tftypes.Value{}, false
		}
		if v, ok = attrs[string(name)]; !ok {
			return tftypes.Value{}, false
		}
	}
	return v, true
}

// plannedFromConfig reports whether the planned value can come from the configured one: every value that
// the configuration sets is the planned one. A null or unknown configured value matches any plan, because
// a computed attribute fills it in.
func plannedFromConfig(config, plan tftypes.Value) bool {
	if !config.IsKnown() || config.IsNull() || !plan.IsKnown() {
		return true
	}
	if plan.IsNull() {
		return false
	}
	switch config.Type().(type) {
	case tftypes.Object, tftypes.Map:
		var c, p map[string]tftypes.Value
		if config.As(&c) != nil || plan.As(&p) != nil {
			return false
		}
		for k, cv := range c {
			pv, found := p[k]
			if !found || !plannedFromConfig(cv, pv) {
				return false
			}
		}
		return true
	case tftypes.List, tftypes.Tuple:
		var c, p []tftypes.Value
		if config.As(&c) != nil || plan.As(&p) != nil || len(c) != len(p) {
			return false
		}
		for i := range c {
			if !plannedFromConfig(c[i], p[i]) {
				return false
			}
		}
		return true
	case tftypes.Set:
		var c, p []tftypes.Value
		if config.As(&c) != nil || plan.As(&p) != nil || len(c) != len(p) {
			return false
		}
		for _, cv := range c {
			if !slices.ContainsFunc(p, func(pv tftypes.Value) bool { return plannedFromConfig(cv, pv) }) {
				return false
			}
		}
		return true
	}
	return config.Equal(plan)
}

// planWebhookRetriggering keeps the retriggering period of a webhook that the configuration leaves out.
// The released resource kept the period of the state item at the same position in the set. The plan and
// the state order a set apart, so this pairs a planned webhook with the state webhook that has the same
// integration or the same recipients.
func planWebhookRetriggering(req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) diag.Diagnostics {
	var diags diag.Diagnostics
	webhooks := []tftypes.AttributePathStep{tftypes.AttributeName("notification_group"), tftypes.AttributeName("webhooks_settings")}
	stateWebhooks, ok := valueAtAttributes(req.State.Raw, webhooks)
	var stateItems []tftypes.Value
	if !ok || !stateWebhooks.IsKnown() || stateWebhooks.IsNull() || stateWebhooks.As(&stateItems) != nil || len(stateItems) == 0 {
		return diags
	}
	planned, err := tftypes.Transform(resp.Plan.Raw, func(p *tftypes.AttributePath, v tftypes.Value) (tftypes.Value, error) {
		steps := p.Steps()
		if len(steps) != 3 || steps[0] != tftypes.AttributePathStep(webhooks[0]) || steps[1] != tftypes.AttributePathStep(webhooks[1]) {
			return v, nil
		}
		if _, isItem := steps[2].(tftypes.ElementKeyValue); !isItem {
			return v, nil
		}
		return withStateRetriggering(v, stateItems), nil
	})
	if err != nil {
		diags.AddError("Unable to plan the retriggering period", err.Error())
		return diags
	}
	resp.Plan.Raw = planned
	return diags
}

// withStateRetriggering returns the planned webhook with the retriggering period of the state webhook that
// it matches, when the plan leaves the period unknown and the state has one.
func withStateRetriggering(webhook tftypes.Value, stateItems []tftypes.Value) tftypes.Value {
	var attrs map[string]tftypes.Value
	if !webhook.IsKnown() || webhook.IsNull() || webhook.As(&attrs) != nil {
		return webhook
	}
	if period, found := attrs["retriggering_period"]; !found || period.IsKnown() {
		return webhook
	}
	for _, item := range stateItems {
		var stateAttrs map[string]tftypes.Value
		if !item.IsKnown() || item.IsNull() || item.As(&stateAttrs) != nil || !sameWebhook(attrs, stateAttrs) {
			continue
		}
		period := stateAttrs["retriggering_period"]
		if !period.IsKnown() || period.IsNull() {
			return webhook
		}
		kept := maps.Clone(attrs)
		kept["retriggering_period"] = period
		return tftypes.NewValue(webhook.Type(), kept)
	}
	return webhook
}

// sameWebhook reports whether two webhooks send to the same integration or the same recipients.
func sameWebhook(a, b map[string]tftypes.Value) bool {
	for _, key := range []string{"integration_id", "recipients"} {
		x, y := a[key], b[key]
		if x.IsKnown() && !x.IsNull() && y.IsKnown() && !y.IsNull() && x.Equal(y) {
			return true
		}
	}
	return false
}
