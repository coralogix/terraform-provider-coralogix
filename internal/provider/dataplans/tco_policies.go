// Copyright 2024 Coralogix Ltd.
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

	"github.com/coralogix/terraform-provider-coralogix/internal/provider/generated/tcopolicieslogs"
	"github.com/coralogix/terraform-provider-coralogix/internal/provider/generated/tcopoliciesrum"
	"github.com/coralogix/terraform-provider-coralogix/internal/provider/generated/tcopoliciestraces"

	tcoPolicys "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/policies_service"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// The source types that the generated resources read. The acceptance tests read the policies of
// each type with them.
var (
	LogSource    = tcoPolicys.V1SOURCETYPE_SOURCE_TYPE_LOGS
	TracesSource = tcoPolicys.V1SOURCETYPE_SOURCE_TYPE_SPANS
	RumSource    = tcoPolicys.V1SOURCETYPE_SOURCE_TYPE_RUM
)

// NewTCOPoliciesLogsResource returns coralogix_tco_policies_logs, which is generated from the API
// contract (internal/provider/generated/tcopolicieslogs).
func NewTCOPoliciesLogsResource() resource.Resource {
	return tcopolicieslogs.NewResource(tcoPoliciesLogsHooks)
}

func NewTCOPoliciesLogsDataSource() datasource.DataSource {
	return tcopolicieslogs.NewDataSource(tcoPoliciesLogsHooks)
}

// NewTCOPoliciesTracesResource returns coralogix_tco_policies_traces, which is generated from the
// API contract (internal/provider/generated/tcopoliciestraces).
func NewTCOPoliciesTracesResource() resource.Resource {
	return tcopoliciestraces.NewResource(tcopoliciestraces.Hooks{ValidateConfig: warnMultiValueRules})
}

func NewTCOPoliciesTracesDataSource() datasource.DataSource {
	return tcopoliciestraces.NewDataSource(tcopoliciestraces.Hooks{})
}

// NewTCOPoliciesRumResource returns coralogix_tco_policies_rum, which is generated from the API
// contract (internal/provider/generated/tcopoliciesrum).
func NewTCOPoliciesRumResource() resource.Resource {
	return tcopoliciesrum.NewResource(tcopoliciesrum.Hooks{ValidateConfig: warnMultiValueRules})
}

func NewTCOPoliciesRumDataSource() datasource.DataSource {
	return tcopoliciesrum.NewDataSource(tcopoliciesrum.Hooks{})
}

// tcoPoliciesLogsHooks are the hooks of the logs resource and data source. The API adds a default
// target to a policy that has none, and a policy holds a priority, or a quota-based override, on
// the policy or on its targets, never both. A policy with its own priority or override therefore
// has targets that the API made, which the released resource reads as null.
var tcoPoliciesLogsHooks = tcopolicieslogs.Hooks{
	ValidateConfig: warnMultiValueRules,
	BeforeFlatten: func(_ context.Context, v *tcoPolicys.GetCompanyPoliciesResponse) diag.Diagnostics {
		for i := range v.Policies {
			p := &v.Policies[i]
			if p.GetPriority() != tcoPolicys.QUOTAV1PRIORITY_PRIORITY_TYPE_UNSPECIFIED || p.PriorityOverride != nil {
				p.Targets = nil
			}
		}
		return nil
	},
}

// warnMultiValueRules warns about an applications or subsystems rule of type starts_with or
// includes with more than one name: the API matches only one value for those types.
func warnMultiValueRules(ctx context.Context, config tfsdk.Config) diag.Diagnostics {
	var diags diag.Diagnostics
	var policies types.List
	diags.Append(config.GetAttribute(ctx, path.Root("policies"), &policies)...)
	if diags.HasError() || policies.IsNull() || policies.IsUnknown() {
		return diags
	}
	for i, element := range policies.Elements() {
		policy, ok := element.(types.Object)
		if !ok || policy.IsNull() || policy.IsUnknown() {
			continue
		}
		for _, attribute := range []string{"applications", "subsystems"} {
			rule, ok := policy.Attributes()[attribute].(types.Object)
			if !ok || rule.IsNull() || rule.IsUnknown() {
				continue
			}
			ruleType, _ := rule.Attributes()["rule_type"].(types.String)
			names, _ := rule.Attributes()["names"].(types.Set)
			kind := ruleType.ValueString()
			if (kind == "starts_with" || kind == "includes") && len(names.Elements()) > 1 {
				diags.AddAttributeWarning(
					path.Root("policies").AtListIndex(i).AtName(attribute),
					"Conflicting Attributes Values Configuration",
					fmt.Sprintf("Currently, rule_type \"%s\" supports only one value, but \"names\" has %d elements. Remove all but one to remove this warning.", kind, len(names.Elements())),
				)
			}
		}
	}
	return diags
}
