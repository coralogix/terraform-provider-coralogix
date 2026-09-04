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
