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

package dashboards

import (
	"context"
	"strings"
	"testing"

	dashboardservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_service"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func ptr[T any](v T) *T {
	return &v
}

func TestDashboardAccessPolicyForConfiguredRequest(t *testing.T) {
	policy := types.StringValue(`{"version":"2025-01-01"}`)

	tests := []struct {
		name   string
		config types.String
		plan   types.String
		want   *string
	}{
		{
			name:   "omitted config does not send preserved state",
			config: types.StringNull(),
			plan:   policy,
		},
		{
			name:   "configured policy sends planned value",
			config: policy,
			plan:   policy,
			want:   policy.ValueStringPointer(),
		},
		{
			name:   "configured empty value does not send",
			config: types.StringValue(""),
			plan:   types.StringValue(""),
		},
		{
			name:   "configured unknown sends planned value",
			config: types.StringUnknown(),
			plan:   policy,
			want:   policy.ValueStringPointer(),
		},
		{
			name:   "unknown plan does not send",
			config: types.StringUnknown(),
			plan:   types.StringUnknown(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dashboardAccessPolicyForConfiguredRequest(tt.config, tt.plan)
			if tt.want == nil {
				if got != nil {
					t.Fatalf("expected nil access policy, got %q", *got)
				}
				return
			}
			if got == nil || *got != *tt.want {
				t.Fatalf("expected access policy %q, got %v", *tt.want, got)
			}
		})
	}
}

func TestExpandDashboardVariableDefinition_ConstantValueDeprecated(t *testing.T) {
	def := &DashboardVariableDefinitionModel{
		ConstantValue: types.StringValue("production"),
	}

	_, diags := expandDashboardVariableDefinition(context.Background(), def)
	if !diags.HasError() {
		t.Fatalf("expected an error for the deprecated constant_value, got none")
	}

	msg := diags.Errors()[0].Summary() + " " + diags.Errors()[0].Detail()
	if !strings.Contains(msg, "constant_value") || !strings.Contains(msg, "multi_select") {
		t.Fatalf("expected the error to direct users from constant_value to multi_select, got: %s", msg)
	}
}

func TestFlattenDashboardVariableDefinition_LegacyConstantBecomesMultiSelect(t *testing.T) {
	def := &dashboardservice.VariableDefinition{
		Constant: &dashboardservice.Constant{Value: ptr("production")},
	}

	got, diags := flattenDashboardVariableDefinition(context.Background(), def)
	if diags.HasError() {
		t.Fatalf("unexpected error flattening legacy constant: %v", diags)
	}
	if got.MultiSelect == nil {
		t.Fatalf("expected legacy Constant to flatten into multi_select, got %+v", got)
	}
	if !got.ConstantValue.IsNull() {
		t.Fatalf("expected constant_value to be null after remap, got %q", got.ConstantValue.ValueString())
	}
}

func TestFlattenDashboardOptionsColor(t *testing.T) {
	tests := []struct {
		name       string
		color      *dashboardservice.SectionColor
		wantNull   bool
		wantString string
	}{
		{
			name:     "color absent (nil)",
			color:    nil,
			wantNull: true,
		},
		{
			name: "predefined unspecified (zero value) is null",
			color: &dashboardservice.SectionColor{
				Predefined: dashboardservice.SECTIONPREDEFINEDCOLOR_SECTION_PREDEFINED_COLOR_UNSPECIFIED.Ptr(),
			},
			wantNull: true,
		},
		{
			name:     "color wrapper present but value oneof unset is null",
			color:    &dashboardservice.SectionColor{},
			wantNull: true,
		},
		{
			name: "predefined cyan flattens to lowercase string",
			color: &dashboardservice.SectionColor{
				Predefined: dashboardservice.SECTIONPREDEFINEDCOLOR_SECTION_PREDEFINED_COLOR_CYAN.Ptr(),
			},
			wantString: "cyan",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := &dashboardservice.SectionOptions{
				Custom: &dashboardservice.CustomSectionOptions{
					Name:  ptr("section"),
					Color: tt.color,
				},
			}
			model, diags := flattenDashboardOptions(context.Background(), opts)
			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}
			if model == nil {
				t.Fatalf("expected non-nil model, got nil")
			}
			if tt.wantNull {
				if !model.Color.IsNull() {
					t.Fatalf("expected null color, got %q", model.Color.ValueString())
				}
				return
			}
			if model.Color.IsNull() {
				t.Fatalf("expected color %q, got null", tt.wantString)
			}
			if got := model.Color.ValueString(); got != tt.wantString {
				t.Fatalf("expected color %q, got %q", tt.wantString, got)
			}
		})
	}
}
