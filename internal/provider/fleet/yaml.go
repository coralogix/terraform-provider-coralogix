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

// UseStateForUnknownWhenYAMLUnchanged keeps computed family/remote IDs when
// the only configuration difference is semantically equal YAML. Terraform marks
// computed nils unknown whenever any config text differs, including inline vs
// multiline lists, before YAML plan modifiers run.
type UseStateForUnknownWhenYAMLUnchanged struct{}

func (m UseStateForUnknownWhenYAMLUnchanged) Description(_ context.Context) string {
	return "Keeps the previous computed value when remote configuration YAML is semantically unchanged."
}

func (m UseStateForUnknownWhenYAMLUnchanged) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m UseStateForUnknownWhenYAMLUnchanged) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if !req.PlanValue.IsUnknown() || req.StateValue.IsNull() || req.StateValue.IsUnknown() {
		return
	}
	if familyOrRemoteYAMLUnchanged(ctx, req) {
		resp.PlanValue = req.StateValue
	}
}

func familyOrRemoteYAMLUnchanged(ctx context.Context, req planmodifier.StringRequest) bool {
	parent := req.Path.ParentPath()
	var planYAML types.String
	if diags := req.Plan.GetAttribute(ctx, parent.AtName("raw_configuration"), &planYAML); !diags.HasError() && !planYAML.IsNull() {
		return yamlAttrUnchanged(ctx, req, parent.AtName("raw_configuration")) &&
			stringAttrEqual(ctx, req, parent.AtName("name")) &&
			mapAttrEqual(ctx, req, parent.AtName("agent_selector"))
	}

	if !boolAttrEqual(ctx, req, parent.AtName("active")) ||
		!stringAttrEqual(ctx, req, parent.AtName("collector_version")) ||
		!stringAttrEqual(ctx, req, parent.AtName("description")) ||
		!mapAttrEqual(ctx, req, parent.AtName("metadata")) {
		return false
	}

	remotesPath := parent.AtName("remote_configuration")
	var planRemotes, stateRemotes types.List
	if diags := req.Plan.GetAttribute(ctx, remotesPath, &planRemotes); diags.HasError() || planRemotes.IsNull() || planRemotes.IsUnknown() {
		return false
	}
	if diags := req.State.GetAttribute(ctx, remotesPath, &stateRemotes); diags.HasError() || stateRemotes.IsNull() || stateRemotes.IsUnknown() {
		return false
	}
	if len(planRemotes.Elements()) != len(stateRemotes.Elements()) {
		return false
	}
	for i := range planRemotes.Elements() {
		item := remotesPath.AtListIndex(i)
		if !yamlAttrUnchanged(ctx, req, item.AtName("raw_configuration")) ||
			!stringAttrEqual(ctx, req, item.AtName("name")) ||
			!mapAttrEqual(ctx, req, item.AtName("agent_selector")) {
			return false
		}
	}
	return true
}

func yamlAttrUnchanged(ctx context.Context, req planmodifier.StringRequest, attrPath path.Path) bool {
	var planYAML, stateYAML types.String
	if diags := req.Plan.GetAttribute(ctx, attrPath, &planYAML); diags.HasError() || planYAML.IsNull() || planYAML.IsUnknown() {
		return false
	}
	if diags := req.State.GetAttribute(ctx, attrPath, &stateYAML); diags.HasError() || stateYAML.IsNull() || stateYAML.IsUnknown() {
		return false
	}
	return yamlStringsEqual(planYAML.ValueString(), stateYAML.ValueString())
}

func stringAttrEqual(ctx context.Context, req planmodifier.StringRequest, attrPath path.Path) bool {
	var planVal, stateVal types.String
	if diags := req.Plan.GetAttribute(ctx, attrPath, &planVal); diags.HasError() {
		return false
	}
	if diags := req.State.GetAttribute(ctx, attrPath, &stateVal); diags.HasError() {
		return false
	}
	return planVal.Equal(stateVal)
}

func boolAttrEqual(ctx context.Context, req planmodifier.StringRequest, attrPath path.Path) bool {
	var planVal, stateVal types.Bool
	if diags := req.Plan.GetAttribute(ctx, attrPath, &planVal); diags.HasError() {
		return false
	}
	if diags := req.State.GetAttribute(ctx, attrPath, &stateVal); diags.HasError() {
		return false
	}
	return planVal.Equal(stateVal)
}

func mapAttrEqual(ctx context.Context, req planmodifier.StringRequest, attrPath path.Path) bool {
	var planVal, stateVal types.Map
	if diags := req.Plan.GetAttribute(ctx, attrPath, &planVal); diags.HasError() {
		return false
	}
	if diags := req.State.GetAttribute(ctx, attrPath, &stateVal); diags.HasError() {
		return false
	}
	return planVal.Equal(stateVal)
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
