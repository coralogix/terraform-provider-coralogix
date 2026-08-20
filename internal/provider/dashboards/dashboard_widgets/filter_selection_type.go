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

package dashboard_widgets

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const (
	filterSelectionTypeAll  = "all"
	filterSelectionTypeList = "list"
)

// filterSelectionTypeFromSelectedValues preserves the legacy meaning of an
// omitted selection_type. It derives the planned value from selected_values
// instead of retaining a computed value from prior state.
type filterSelectionTypeFromSelectedValues struct{}

func (m filterSelectionTypeFromSelectedValues) Description(_ context.Context) string {
	return "When selection_type is omitted, an empty selected_values list selects all values and a non-empty list selects those values."
}

func (m filterSelectionTypeFromSelectedValues) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m filterSelectionTypeFromSelectedValues) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.ConfigValue.IsUnknown() || !req.ConfigValue.IsNull() {
		return
	}

	var selectedValues types.List
	diags := req.Plan.GetAttribute(ctx, req.Path.ParentPath().AtName("selected_values"), &selectedValues)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	if selectedValues.IsUnknown() {
		resp.PlanValue = types.StringUnknown()
		return
	}
	if selectedValues.IsNull() || len(selectedValues.Elements()) == 0 {
		resp.PlanValue = types.StringValue(filterSelectionTypeAll)
		return
	}
	resp.PlanValue = types.StringValue(filterSelectionTypeList)
}
