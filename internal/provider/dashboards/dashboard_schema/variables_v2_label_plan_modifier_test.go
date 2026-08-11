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

package dashboard_schema

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

func TestStaticValueLabelFromValue(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	s := schema.Schema{
		Attributes: map[string]schema.Attribute{
			"value": schema.StringAttribute{Required: true},
			"label": schema.StringAttribute{Optional: true, Computed: true},
		},
	}

	tests := []struct {
		name        string
		configLabel types.String
		planLabel   types.String
		stateLabel  types.String
		planValue   tftypes.Value
		want        types.String
	}{
		{
			name:        "omit_copies_value",
			configLabel: types.StringNull(),
			planLabel:   types.StringUnknown(),
			stateLabel:  types.StringNull(),
			planValue:   tftypes.NewValue(tftypes.String, "production"),
			want:        types.StringValue("production"),
		},
		{
			name:        "label_set_keeps_custom",
			configLabel: types.StringValue("Staging"),
			planLabel:   types.StringValue("Staging"),
			stateLabel:  types.StringValue("production"), // prior omit default
			planValue:   tftypes.NewValue(tftypes.String, "staging"),
			want:        types.StringValue("Staging"),
		},
		{
			name:        "omit_after_custom_recomputes_from_value",
			configLabel: types.StringNull(),
			planLabel:   types.StringUnknown(),
			stateLabel:  types.StringValue("Staging"), // prior custom must not stick
			planValue:   tftypes.NewValue(tftypes.String, "staging"),
			want:        types.StringValue("staging"),
		},
		{
			name:        "omit_value_change_follows_new_value",
			configLabel: types.StringNull(),
			planLabel:   types.StringUnknown(),
			stateLabel:  types.StringValue("production"), // old default
			planValue:   tftypes.NewValue(tftypes.String, "prod"),
			want:        types.StringValue("prod"),
		},
		{
			name:        "label_set_value_change_keeps_label",
			configLabel: types.StringValue("Display"),
			planLabel:   types.StringValue("Display"),
			stateLabel:  types.StringValue("Display"),
			planValue:   tftypes.NewValue(tftypes.String, "b"), // was "a"
			want:        types.StringValue("Display"),
		},
		{
			name:        "explicit_equal_to_value_kept",
			configLabel: types.StringValue("production"),
			planLabel:   types.StringValue("production"),
			stateLabel:  types.StringValue("production"),
			planValue:   tftypes.NewValue(tftypes.String, "production"),
			want:        types.StringValue("production"),
		},
		{
			name:        "omit_with_unknown_value_stays_unknown",
			configLabel: types.StringNull(),
			planLabel:   types.StringUnknown(),
			stateLabel:  types.StringNull(),
			planValue:   tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
			want:        types.StringUnknown(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			raw := tftypes.NewValue(s.Type().TerraformType(ctx), map[string]tftypes.Value{
				"value": tt.planValue,
				"label": labelTFTValue(tt.planLabel),
			})
			req := planmodifier.StringRequest{
				Path:        path.Root("label"),
				ConfigValue: tt.configLabel,
				PlanValue:   tt.planLabel,
				StateValue:  tt.stateLabel,
				Plan: tfsdk.Plan{
					Schema: s,
					Raw:    raw,
				},
			}
			resp := &planmodifier.StringResponse{PlanValue: tt.planLabel}
			StaticValueLabelFromValue{}.PlanModifyString(ctx, req, resp)
			if resp.Diagnostics.HasError() {
				t.Fatalf("diagnostics: %v", resp.Diagnostics)
			}
			if !resp.PlanValue.Equal(tt.want) {
				t.Fatalf("plan = %#v, want %#v", resp.PlanValue, tt.want)
			}
		})
	}
}

func labelTFTValue(v types.String) tftypes.Value {
	if v.IsUnknown() {
		return tftypes.NewValue(tftypes.String, tftypes.UnknownValue)
	}
	if v.IsNull() {
		return tftypes.NewValue(tftypes.String, nil)
	}
	return tftypes.NewValue(tftypes.String, v.ValueString())
}
