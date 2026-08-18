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

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestPreserveStateForEquivalentJSON(t *testing.T) {
	t.Parallel()

	state := types.StringValue(`{"default":{"permissions":{"team-dashboards:Read":"grant","team-dashboards:Update":"grant"}},"rules":[{"id":"first"},{"id":"second"}],"version":"2025-01-01"}`)

	tests := []struct {
		name         string
		config       types.String
		wantPreserve bool
	}{
		{
			name:         "object key order is ignored",
			config:       types.StringValue(`{"version":"2025-01-01","rules":[{"id":"first"},{"id":"second"}],"default":{"permissions":{"team-dashboards:Update":"grant","team-dashboards:Read":"grant"}}}`),
			wantPreserve: true,
		},
		{
			name:         "array order is semantic",
			config:       types.StringValue(`{"default":{"permissions":{"team-dashboards:Read":"grant","team-dashboards:Update":"grant"}},"rules":[{"id":"second"},{"id":"first"}],"version":"2025-01-01"}`),
			wantPreserve: false,
		},
		{
			name:         "invalid json is not suppressed",
			config:       types.StringValue(`{"version":`),
			wantPreserve: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := planmodifier.StringRequest{
				ConfigValue: tt.config,
				PlanValue:   tt.config,
				StateValue:  state,
			}
			resp := &planmodifier.StringResponse{PlanValue: tt.config}

			PreserveStateForEquivalentJSON{}.PlanModifyString(context.Background(), req, resp)

			if tt.wantPreserve {
				if !resp.PlanValue.Equal(state) {
					t.Fatalf("expected PlanValue to equal state %v, got %v", state, resp.PlanValue)
				}
				return
			}

			if !resp.PlanValue.Equal(tt.config) {
				t.Fatalf("expected PlanValue to remain config %v, got %v", tt.config, resp.PlanValue)
			}
		})
	}
}

func TestAutoRefreshNullWhenContentJSONManaged(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	autoRefreshTypes := map[string]attr.Type{"type": types.StringType}
	autoRefreshAttribute, ok := V4().Attributes["auto_refresh"].(schema.SingleNestedAttribute)
	if !ok {
		t.Fatal("auto_refresh is not a single nested attribute")
	}
	if len(autoRefreshAttribute.PlanModifiers) != 1 {
		t.Fatalf("auto_refresh plan modifiers = %d, want 1", len(autoRefreshAttribute.PlanModifiers))
	}
	modifier, ok := autoRefreshAttribute.PlanModifiers[0].(NullWhenContentJSONManaged)
	if !ok {
		t.Fatalf("auto_refresh plan modifier = %T, want NullWhenContentJSONManaged", autoRefreshAttribute.PlanModifiers[0])
	}
	testSchema := schema.Schema{Attributes: map[string]schema.Attribute{
		"auto_refresh": schema.SingleNestedAttribute{
			Attributes: map[string]schema.Attribute{
				"type": schema.StringAttribute{Optional: true},
			},
			Optional: true,
			Computed: true,
		},
		"content_json": schema.StringAttribute{Optional: true},
	}}
	configuredAutoRefresh := types.ObjectValueMust(autoRefreshTypes, map[string]attr.Value{
		"type": types.StringValue("off"),
	})
	structuredState := types.ObjectValueMust(autoRefreshTypes, map[string]attr.Value{
		"type": types.StringValue("five_minutes"),
	})
	contentJSON := tftypes.NewValue(tftypes.String, `{"name":"dashboard"}`)
	noContentJSON := tftypes.NewValue(tftypes.String, nil)
	priorState := tfsdk.State{Raw: tftypes.NewValue(
		tftypes.Object{AttributeTypes: map[string]tftypes.Type{}},
		map[string]tftypes.Value{},
	)}
	createState := tfsdk.State{Raw: tftypes.NewValue(testSchema.Type().TerraformType(ctx), nil)}

	tests := []struct {
		name        string
		contentJSON tftypes.Value
		configValue types.Object
		planValue   types.Object
		state       tfsdk.State
		stateValue  types.Object
		want        types.Object
	}{
		{
			name:        "content_json keeps the attribute null",
			contentJSON: contentJSON,
			configValue: types.ObjectNull(autoRefreshTypes),
			planValue:   types.ObjectUnknown(autoRefreshTypes),
			state:       priorState,
			stateValue:  types.ObjectNull(autoRefreshTypes),
			want:        types.ObjectNull(autoRefreshTypes),
		},
		{
			name:        "content_json nulls a value left over from a structured configuration",
			contentJSON: contentJSON,
			configValue: types.ObjectNull(autoRefreshTypes),
			planValue:   types.ObjectUnknown(autoRefreshTypes),
			state:       priorState,
			stateValue:  structuredState,
			want:        types.ObjectNull(autoRefreshTypes),
		},
		{
			name:        "structured dashboard keeps the unknown so the API decides",
			contentJSON: noContentJSON,
			configValue: types.ObjectNull(autoRefreshTypes),
			planValue:   types.ObjectUnknown(autoRefreshTypes),
			state:       priorState,
			stateValue:  structuredState,
			want:        types.ObjectUnknown(autoRefreshTypes),
		},
		{
			name:        "a configured auto_refresh is left alone",
			contentJSON: contentJSON,
			configValue: configuredAutoRefresh,
			planValue:   configuredAutoRefresh,
			state:       priorState,
			stateValue:  types.ObjectNull(autoRefreshTypes),
			want:        configuredAutoRefresh,
		},
		{
			name:        "create is left alone",
			contentJSON: contentJSON,
			configValue: types.ObjectNull(autoRefreshTypes),
			planValue:   types.ObjectUnknown(autoRefreshTypes),
			state:       createState,
			stateValue:  types.ObjectNull(autoRefreshTypes),
			want:        types.ObjectUnknown(autoRefreshTypes),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			raw := tftypes.NewValue(testSchema.Type().TerraformType(ctx), map[string]tftypes.Value{
				"auto_refresh": tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{"type": tftypes.String}}, nil),
				"content_json": tt.contentJSON,
			})
			req := planmodifier.ObjectRequest{
				Config:      tfsdk.Config{Raw: raw, Schema: testSchema},
				ConfigValue: tt.configValue,
				PlanValue:   tt.planValue,
				State:       tt.state,
				StateValue:  tt.stateValue,
			}
			resp := &planmodifier.ObjectResponse{PlanValue: req.PlanValue}

			modifier.PlanModifyObject(ctx, req, resp)

			if resp.Diagnostics.HasError() {
				t.Fatalf("diagnostics: %v", resp.Diagnostics)
			}
			if !resp.PlanValue.Equal(tt.want) {
				t.Fatalf("auto_refresh plan = %#v, want %#v", resp.PlanValue, tt.want)
			}
		})
	}
}

func TestContentJsonValidator(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		value     types.String
		wantError bool
	}{
		{
			name:      "valid OpenAPI dashboard",
			value:     types.StringValue(`{"name":"dashboard","layout":{"sections":[]}}`),
			wantError: false,
		},
		{
			name:      "valid protobuf field names with required nested alias",
			value:     types.StringValue(`{"name":"dashboard","layout":{"sections":[{"rows":[{"widgets":[{"definition":{"line_chart":{"query_definitions":[]}}}]}]}]}}`),
			wantError: false,
		},
		{
			name:      "valid lower-camel parent with required nested alias",
			value:     types.StringValue(`{"name":"dashboard","layout":{"sections":[{"rows":[{"widgets":[{"definition":{"lineChart":{"query_definitions":[]}}}]}]}]}}`),
			wantError: false,
		},
		{
			name:      "missing required OpenAPI field",
			value:     types.StringValue(`{"layout":{"sections":[]}}`),
			wantError: true,
		},
		{
			name:      "invalid json",
			value:     types.StringValue(`{"name":`),
			wantError: true,
		},
		{
			name:      "null is ignored",
			value:     types.StringNull(),
			wantError: false,
		},
		{
			name:      "unknown is ignored",
			value:     types.StringUnknown(),
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			resp := &validator.StringResponse{}
			ContentJsonValidator{}.ValidateString(context.Background(), validator.StringRequest{
				ConfigValue: tt.value,
			}, resp)

			if tt.wantError && !resp.Diagnostics.HasError() {
				t.Fatal("expected validator error")
			}
			if !tt.wantError && resp.Diagnostics.HasError() {
				t.Fatalf("expected no validator error, got %v", resp.Diagnostics)
			}
		})
	}
}
