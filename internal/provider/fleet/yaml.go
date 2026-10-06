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

package fleet

import (
	"context"
	"reflect"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"gopkg.in/yaml.v3"
)

// PreserveStateForEquivalentYAML keeps the previous state string when the
// configured YAML is semantically equal, so inline vs multiline lists do not plan.
type PreserveStateForEquivalentYAML struct{}

func (m PreserveStateForEquivalentYAML) Description(_ context.Context) string {
	return "Preserves the previous state value when the configured YAML is semantically equivalent."
}

func (m PreserveStateForEquivalentYAML) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m PreserveStateForEquivalentYAML) PlanModifyString(_ context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() || req.StateValue.IsNull() || req.StateValue.IsUnknown() {
		return
	}
	if yamlStringsEqual(req.ConfigValue.ValueString(), req.StateValue.ValueString()) {
		resp.PlanValue = req.StateValue
	}
}

// UseStateForUnknownWhenFamilyUnchanged keeps a computed family value (family
// and remote IDs, hashes, preset defaults and generated remotes) when nothing
// that mints a new family version changed. Terraform marks computed nils
// unknown whenever any config text differs, including inline vs multiline YAML
// lists, before the YAML and JSON plan modifiers run. Levels is how many
// parent steps lead from the attribute to the family object.
type UseStateForUnknownWhenFamilyUnchanged struct {
	Levels int
}

func (m UseStateForUnknownWhenFamilyUnchanged) Description(_ context.Context) string {
	return "Keeps the previous computed value when the configuration family is semantically unchanged."
}

func (m UseStateForUnknownWhenFamilyUnchanged) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m UseStateForUnknownWhenFamilyUnchanged) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if !req.PlanValue.IsUnknown() || req.StateValue.IsNull() || req.StateValue.IsUnknown() {
		return
	}
	if familyUnchangedAt(ctx, req.Plan, req.State, m.familyPath(req.Path)) {
		resp.PlanValue = req.StateValue
	}
}

func (m UseStateForUnknownWhenFamilyUnchanged) PlanModifyList(ctx context.Context, req planmodifier.ListRequest, resp *planmodifier.ListResponse) {
	if !req.PlanValue.IsUnknown() || req.StateValue.IsNull() || req.StateValue.IsUnknown() {
		return
	}
	if familyUnchangedAt(ctx, req.Plan, req.State, m.familyPath(req.Path)) {
		resp.PlanValue = req.StateValue
	}
}

func (m UseStateForUnknownWhenFamilyUnchanged) familyPath(p path.Path) path.Path {
	for i := 0; i < m.Levels; i++ {
		p = p.ParentPath()
	}
	return p
}

func familyUnchangedAt(ctx context.Context, plan tfsdk.Plan, state tfsdk.State, familyPath path.Path) bool {
	var planFamily, stateFamily *FleetConfigurationGroupFamilyModel
	if diags := plan.GetAttribute(ctx, familyPath, &planFamily); diags.HasError() {
		return false
	}
	if diags := state.GetAttribute(ctx, familyPath, &stateFamily); diags.HasError() {
		return false
	}
	return planFamily != nil && stateFamily != nil && familyConfigUnchanged(planFamily, stateFamily)
}

func echoYAML(configured, api string) types.String {
	if yamlStringsEqual(configured, api) && configured != "" {
		return types.StringValue(configured)
	}
	if api == "" {
		return types.StringNull()
	}
	return types.StringValue(api)
}

func yamlStringsEqual(a, b string) bool {
	if a == b {
		return true
	}
	var left, right any
	if err := yaml.Unmarshal([]byte(a), &left); err != nil {
		return false
	}
	if err := yaml.Unmarshal([]byte(b), &right); err != nil {
		return false
	}
	return reflect.DeepEqual(left, right)
}

func familyConfigUnchanged(plan, state *FleetConfigurationGroupFamilyModel) bool {
	if plan == nil || state == nil {
		return plan == state
	}
	if !plan.Active.Equal(state.Active) || !plan.Description.Equal(state.Description) {
		return false
	}
	return presetConfigUnchanged(plan.Preset, state.Preset) && rawConfigUnchanged(plan.Raw, state.Raw)
}

func presetConfigUnchanged(plan, state *FleetPresetFamilyModel) bool {
	if plan == nil || state == nil {
		return plan == state
	}
	if !plan.ChartName.Equal(state.ChartName) ||
		!plan.ChartVersion.Equal(state.ChartVersion) ||
		!plan.Metadata.Equal(state.Metadata) {
		return false
	}
	// Omitting integration_version plans unknown; the API keeps the prior value.
	if !plan.IntegrationVersion.IsUnknown() && !plan.IntegrationVersion.Equal(state.IntegrationVersion) {
		return false
	}
	if plan.ObservabilityFeatures.IsUnknown() || state.ObservabilityFeatures.IsUnknown() {
		return false
	}
	return jsonStringsEqual(plan.ObservabilityFeatures.ValueString(), state.ObservabilityFeatures.ValueString())
}

func rawConfigUnchanged(plan, state *FleetRawFamilyModel) bool {
	if plan == nil || state == nil {
		return plan == state
	}
	if !plan.Metadata.Equal(state.Metadata) {
		return false
	}
	// Omitting collector_version plans unknown; the API keeps the prior value.
	if !plan.CollectorVersion.IsUnknown() && !plan.CollectorVersion.Equal(state.CollectorVersion) {
		return false
	}
	if len(plan.RemoteConfigurations) != len(state.RemoteConfigurations) {
		return false
	}
	for i := range plan.RemoteConfigurations {
		planned := plan.RemoteConfigurations[i]
		prior := state.RemoteConfigurations[i]
		if !planned.Name.Equal(prior.Name) || !planned.AgentSelector.Equal(prior.AgentSelector) {
			return false
		}
		if planned.RawConfiguration.IsUnknown() || prior.RawConfiguration.IsUnknown() {
			return false
		}
		if !yamlStringsEqual(planned.RawConfiguration.ValueString(), prior.RawConfiguration.ValueString()) {
			return false
		}
	}
	return true
}
