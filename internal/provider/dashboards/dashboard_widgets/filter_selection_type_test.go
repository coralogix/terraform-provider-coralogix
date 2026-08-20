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
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestFilterSelectionTypeFromSelectedValues(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	s := schema.Schema{Attributes: map[string]schema.Attribute{
		"selection_type":  schema.StringAttribute{Optional: true, Computed: true},
		"selected_values": schema.ListAttribute{Optional: true, Computed: true, ElementType: types.StringType},
	}}

	tests := []struct {
		name            string
		configSelection types.String
		planSelection   types.String
		stateSelection  types.String
		selectedValues  tftypes.Value
		want            types.String
	}{
		{
			name:            "omitted_empty_overrides_prior_list",
			configSelection: types.StringNull(),
			planSelection:   types.StringValue(filterSelectionTypeList),
			stateSelection:  types.StringValue(filterSelectionTypeList),
			selectedValues:  filterSelectedValuesTerraformValue([]string{}),
			want:            types.StringValue(filterSelectionTypeAll),
		},
		{
			name:            "omitted_non_empty_overrides_prior_all",
			configSelection: types.StringNull(),
			planSelection:   types.StringValue(filterSelectionTypeAll),
			stateSelection:  types.StringValue(filterSelectionTypeAll),
			selectedValues:  filterSelectedValuesTerraformValue([]string{"api"}),
			want:            types.StringValue(filterSelectionTypeList),
		},
		{
			name:            "explicit_list_is_preserved",
			configSelection: types.StringValue(filterSelectionTypeList),
			planSelection:   types.StringValue(filterSelectionTypeList),
			stateSelection:  types.StringValue(filterSelectionTypeAll),
			selectedValues:  filterSelectedValuesTerraformValue([]string{}),
			want:            types.StringValue(filterSelectionTypeList),
		},
		{
			name:            "unknown_values_keep_selection_unknown",
			configSelection: types.StringNull(),
			planSelection:   types.StringUnknown(),
			stateSelection:  types.StringValue(filterSelectionTypeAll),
			selectedValues:  tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, tftypes.UnknownValue),
			want:            types.StringUnknown(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			raw := tftypes.NewValue(s.Type().TerraformType(ctx), map[string]tftypes.Value{
				"selection_type":  filterSelectionTypeTerraformValue(tt.planSelection),
				"selected_values": tt.selectedValues,
			})
			req := planmodifier.StringRequest{
				Path:        path.Root("selection_type"),
				ConfigValue: tt.configSelection,
				PlanValue:   tt.planSelection,
				StateValue:  tt.stateSelection,
				Plan:        tfsdk.Plan{Schema: s, Raw: raw},
			}
			resp := &planmodifier.StringResponse{PlanValue: tt.planSelection}

			filterSelectionTypeFromSelectedValues{}.PlanModifyString(ctx, req, resp)

			if resp.Diagnostics.HasError() {
				t.Fatalf("diagnostics: %v", resp.Diagnostics)
			}
			if !resp.PlanValue.Equal(tt.want) {
				t.Fatalf("plan = %#v, want %#v", resp.PlanValue, tt.want)
			}
		})
	}
}

func filterSelectionTypeTerraformValue(value types.String) tftypes.Value {
	if value.IsUnknown() {
		return tftypes.NewValue(tftypes.String, tftypes.UnknownValue)
	}
	if value.IsNull() {
		return tftypes.NewValue(tftypes.String, nil)
	}
	return tftypes.NewValue(tftypes.String, value.ValueString())
}

func filterSelectedValuesTerraformValue(values []string) tftypes.Value {
	elements := make([]tftypes.Value, 0, len(values))
	for _, value := range values {
		elements = append(elements, tftypes.NewValue(tftypes.String, value))
	}
	return tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, elements)
}
