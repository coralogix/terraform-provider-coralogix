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

	quotaRules "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/quota_allocation_rule_set_service"
	"github.com/coralogix/terraform-provider-coralogix/internal/provider/generated/quotaallocationruleset"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

const quotaAllocationTypeUnspecified = "unspecified"

const quotaAllocationTypePercentage = "percentage"

func NewQuotaAllocationRuleSetResource() resource.Resource {
	return quotaallocationruleset.NewResource(quotaallocationruleset.Hooks{
		BeforeFlatten:  dropCxManagedQuotaAllocationRules,
		ValidateConfig: validateQuotaAllocationRulesConfig,
		BeforeWrite:    validateQuotaAllocationRulesBeforeWrite,
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

// validateQuotaAllocationRules checks what no key of the behavior-overrides file expresses: the
// percentage limit depends on allocation_type. The duplicate entity type and the float32 precision
// are generated checks.
func validateQuotaAllocationRules(rules []quotaRuleConfig) diag.Diagnostics {
	diags := diag.Diagnostics{}
	for _, rule := range rules {
		if rule.allocation == nil || rule.allocationTypeUnknown {
			continue
		}
		if rule.allocationType != "" && rule.allocationType != quotaAllocationTypePercentage && rule.allocationType != quotaAllocationTypeUnspecified {
			continue
		}
		if allocation, _ := rule.allocation.Float64(); allocation > 100 {
			diags.AddAttributeError(
				path.Root("rules"),
				"Invalid percentage quota allocation",
				fmt.Sprintf("The quota allocation rule for entity_type %q uses allocation_type %q, so allocation must be between 0 and 100.", rule.entityType, quotaAllocationTypePercentage),
			)
		}
	}
	return diags
}
