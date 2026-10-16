// Copyright 2026 Coralogix Ltd.
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

package dataplans

import (
	"context"
	"fmt"
	"math/big"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	quotaRules "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/quota_allocation_rule_set_service"
	"github.com/coralogix/terraform-provider-coralogix/internal/provider/generated/quotaallocationruleset"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

const quotaAllocationTypeUnspecified = "unspecified"

func NewQuotaAllocationRuleSetResource() resource.Resource {
	return quotaallocationruleset.NewResource(quotaallocationruleset.Hooks{
		BeforeFlatten:     dropCxManagedQuotaAllocationRules,
		ValidateConfig:    validateQuotaAllocationRulesConfig,
		BeforeWrite:       validateQuotaAllocationRulesBeforeWrite,
		ModifyPlan:        normalizeUnspecifiedQuotaAllocationTypes,
		CreateFallback:    adoptQuotaAllocationRuleSetOnConflict,
		IgnoreDeleteError: quotaAllocationRuleSetClearedAfterDeleteError,
	})
}

// dropCxManagedQuotaAllocationRules keeps the rules that Coralogix manages out of the state of the
// resource. The API returns them next to the rules of the customer and preserves them on every
// write. The data source shows them. A rule with no type, or an unspecified one, is a percentage
// rule: that is what the API stores for it, and what the configuration plans.
func dropCxManagedQuotaAllocationRules(_ context.Context, ruleSet *quotaRules.QuotaAllocationEntityTypeRuleSet) diag.Diagnostics {
	ruleSet.Rules = customerQuotaAllocationRules(ruleSet.Rules)
	for i := range ruleSet.Rules {
		if allocationType, ok := ruleSet.Rules[i].GetAllocationTypeOk(); !ok || *allocationType == quotaRules.QUOTAALLOCATIONTYPE_QUOTA_ALLOCATION_TYPE_UNSPECIFIED {
			ruleSet.Rules[i].SetAllocationType(quotaRules.QUOTAALLOCATIONTYPE_QUOTA_ALLOCATION_TYPE_PERCENTAGE)
		}
	}
	return nil
}

func customerQuotaAllocationRules(rules []quotaRules.QuotaAllocationEntityTypeRule) []quotaRules.QuotaAllocationEntityTypeRule {
	customer := make([]quotaRules.QuotaAllocationEntityTypeRule, 0, len(rules))
	for _, rule := range rules {
		if !rule.GetCxManaged() {
			customer = append(customer, rule)
		}
	}
	return customer
}

// quotaRuleConfig is a rule in the configuration. A nil allocation or an empty field is a value
// that is null or not known yet.
type quotaRuleConfig struct {
	entityType     string
	allocation     *big.Float
	allocationType string
	// allocationTypeUnknown: the value is not known yet, for example it comes from another resource.
	allocationTypeUnknown bool
}

func quotaRulesFromConfig(raw tftypes.Value) []quotaRuleConfig {
	var root map[string]tftypes.Value
	if raw.IsNull() || !raw.IsFullyKnown() && !raw.IsKnown() || raw.As(&root) != nil {
		return nil
	}
	var items []tftypes.Value
	if rules := root["rules"]; !rules.IsKnown() || rules.IsNull() || rules.As(&items) != nil {
		return nil
	}
	out := make([]quotaRuleConfig, 0, len(items))
	for _, item := range items {
		var fields map[string]tftypes.Value
		if !item.IsKnown() || item.IsNull() || item.As(&fields) != nil {
			continue
		}
		var rule quotaRuleConfig
		_ = fields["entity_type"].As(&rule.entityType)
		if value := fields["allocation"]; value.IsKnown() && !value.IsNull() {
			rule.allocation = new(big.Float)
			if value.As(rule.allocation) != nil {
				rule.allocation = nil
			}
		}
		if value := fields["allocation_type"]; !value.IsKnown() {
			rule.allocationType, rule.allocationTypeUnknown = "", true
		} else {
			_ = value.As(&rule.allocationType)
		}
		out = append(out, rule)
	}
	return out
}

// validateQuotaAllocationRulesBeforeWrite runs the checks again on the resolved configuration: a
// value that comes from another resource is unknown while Terraform validates the configuration,
// so the checks skip it, and the write is the first place that sees it.
func validateQuotaAllocationRulesBeforeWrite(_ context.Context, config tfsdk.Config, _ any) diag.Diagnostics {
	return validateQuotaAllocationRules(quotaRulesFromConfig(config.Raw))
}

func validateQuotaAllocationRulesConfig(_ context.Context, config tfsdk.Config) diag.Diagnostics {
	return validateQuotaAllocationRules(quotaRulesFromConfig(config.Raw))
}

// validateQuotaAllocationRules checks what no attribute validator can: a duplicate entity type, a
// percentage above 100, and an allocation that the API stores as another float32.
func validateQuotaAllocationRules(rules []quotaRuleConfig) diag.Diagnostics {
	diags := diag.Diagnostics{}
	seen := map[string]struct{}{}
	for _, rule := range rules {
		if rule.entityType == "" {
			continue
		}
		if _, ok := seen[rule.entityType]; ok {
			diags.AddAttributeError(
				path.Root("rules"),
				"Duplicate quota allocation rule entity_type",
				fmt.Sprintf("Only one quota allocation rule can be configured for entity_type %q.", rule.entityType),
			)
			continue
		}
		seen[rule.entityType] = struct{}{}

		if rule.allocation == nil {
			continue
		}
		allocation, _ := rule.allocation.Float64()
		if !rule.allocationTypeUnknown && (rule.allocationType == "" || rule.allocationType == quotaAllocationTypePercentage || rule.allocationType == quotaAllocationTypeUnspecified) && allocation > 100 {
			diags.AddAttributeError(
				path.Root("rules"),
				"Invalid percentage quota allocation",
				fmt.Sprintf("The quota allocation rule for entity_type %q uses allocation_type %q, so allocation must be between 0 and 100.", rule.entityType, quotaAllocationTypePercentage),
			)
		}
		normalized := float32ToSchemaFloat64Allocation(float32(allocation))
		if normalized != allocation {
			diags.AddAttributeError(
				path.Root("rules"),
				"Invalid quota allocation precision",
				fmt.Sprintf("The quota allocation rule for entity_type %q uses allocation %s, but the Coralogix API stores allocations as float32 and would return %s. Configure %s instead to avoid Terraform state drift.", rule.entityType, formatQuotaAllocation(allocation), formatQuotaAllocation(normalized), formatQuotaAllocation(normalized)),
			)
		}
	}
	return diags
}

func formatQuotaAllocation(value float64) string {
	return new(big.Float).SetFloat64(value).Text('f', -1)
}

// normalizeUnspecifiedQuotaAllocationTypes plans allocation_type "unspecified", an alias that the
// resource accepts, as "percentage", which is what the API stores for it.
func normalizeUnspecifiedQuotaAllocationTypes(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}
	var rules types.Set
	diags := resp.Plan.GetAttribute(ctx, path.Root("rules"), &rules)
	if diags.HasError() || rules.IsNull() || rules.IsUnknown() {
		resp.Diagnostics.Append(diags...)
		return
	}
	var items []quotaallocationruleset.QuotaAllocationEntityTypeRuleModel
	if diags := rules.ElementsAs(ctx, &items, false); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	changed := false
	for i := range items {
		if items[i].AllocationType.ValueString() == quotaAllocationTypeUnspecified {
			items[i].AllocationType = types.StringValue(quotaAllocationTypePercentage)
			changed = true
		}
	}
	if !changed {
		return
	}
	normalized, diags := types.SetValueFrom(ctx, rules.ElementType(ctx), items)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("rules"), normalized)...)
	// Terraform plans the computed id as unknown, because the configuration differs from the state
	// until the alias is normalized. When the normalized rules are the stored rules, nothing changes.
	if req.State.Raw.IsNull() {
		return
	}
	var stateRules types.Set
	if req.State.GetAttribute(ctx, path.Root("rules"), &stateRules).HasError() || !stateRules.Equal(normalized) {
		return
	}
	var id types.String
	if req.State.GetAttribute(ctx, path.Root("id"), &id).HasError() {
		return
	}
	resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("id"), id)...)
}

// adoptQuotaAllocationRuleSetOnConflict handles a create that the API refuses with 409 because the
// account already has a rule set. When it holds no rules of the customer, only rules that
// Coralogix manages, the provider replaces it: the API preserves the managed rules. Otherwise the
// user has to import the rule set.
func adoptQuotaAllocationRuleSetOnConflict(ctx context.Context, client *quotaRules.QuotaAllocationRuleSetServiceAPIService, body *quotaRules.CreateQuotaAllocationRuleSetRequest, createErr error) (*quotaRules.QuotaAllocationEntityTypeRuleSet, bool, diag.Diagnostics) {
	if cxsdk.Code(createErr) != 409 {
		return nil, false, nil
	}
	var diags diag.Diagnostics
	existing, httpResponse, err := client.QuotaAllocationRuleSetServiceGetQuotaAllocationRuleSet(ctx).Execute()
	if err != nil {
		diags.AddError("Unable to read the existing quota_allocation_rule_set after a create conflict", cxsdk.NewAPIError(httpResponse, err).Error())
		return nil, false, diags
	}
	if existing != nil && existing.RuleSet != nil && len(customerQuotaAllocationRules(existing.RuleSet.Rules)) > 0 {
		diags.AddError(
			"Quota allocation rule set already exists",
			"Coralogix already has a quota allocation rule set. Import it with `terraform import coralogix_quota_allocation_rule_set.<name> quota-allocation-rule-set` before managing it with Terraform.",
		)
		return nil, false, diags
	}
	// Send only the rules of the customer. The API preserves the managed rules on a replace.
	replace := quotaRules.ReplaceQuotaAllocationRuleSetRequest{RuleSet: body.RuleSet}
	if existing != nil && existing.RuleSet != nil && existing.RuleSet.GetId() != "" {
		replace.RuleSet.SetId(existing.RuleSet.GetId())
	}
	replaced, httpResponse, err := client.QuotaAllocationRuleSetServiceReplaceQuotaAllocationRuleSet(ctx).ReplaceQuotaAllocationRuleSetRequest(replace).Execute()
	if err != nil {
		diags.AddError("Unable to preserve the Coralogix-managed quota allocation rules during create", cxsdk.NewAPIError(httpResponse, err).Error())
		return nil, false, diags
	}
	if replaced == nil {
		return nil, true, nil
	}
	return replaced.RuleSet, true, nil
}

// quotaAllocationRuleSetClearedAfterDeleteError ignores an error of the delete call when the API
// cleared the rules of the customer anyway: it keeps the rules that Coralogix manages, and
// sometimes answers with an error after it cleared the rest.
func quotaAllocationRuleSetClearedAfterDeleteError(ctx context.Context, client *quotaRules.QuotaAllocationRuleSetServiceAPIService, _ error) bool {
	result, httpResponse, err := client.QuotaAllocationRuleSetServiceGetQuotaAllocationRuleSet(ctx).Execute()
	if err != nil {
		// A 404 of the verification read means that the rule set is gone, as for the old delete.
		return cxsdk.Code(cxsdk.NewAPIError(httpResponse, err)) == 404
	}
	return result == nil || result.RuleSet == nil || len(customerQuotaAllocationRules(result.RuleSet.Rules)) == 0
}
