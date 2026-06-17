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
	"testing"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

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

func TestFlattenDashboardOptionsColorPredefined(t *testing.T) {
	customWith := func(color *cxsdk.DashboardSectionColor) *cxsdk.DashboardSectionOptions {
		return &cxsdk.DashboardSectionOptions{
			Value: &cxsdk.DashboardSectionOptionsCustom{
				Custom: &cxsdk.CustomSectionOptions{
					Name:  wrapperspb.String("Status"),
					Color: color,
				},
			},
		}
	}
	predefined := func(c cxsdk.DashboardSectionColorPredefinedColor) *cxsdk.DashboardSectionColor {
		return &cxsdk.DashboardSectionColor{
			Value: &cxsdk.DashboardSectionColorPredefined{Predefined: c},
		}
	}

	cases := []struct {
		name string
		opts *cxsdk.DashboardSectionOptions
		want types.String
	}{
		{
			name: "color unset",
			opts: customWith(nil),
			want: types.StringNull(),
		},
		{
			name: "color predefined UNSPECIFIED round-trips to null",
			opts: customWith(predefined(cxsdk.DashboardSectionColorPredefinedColor(0))),
			want: types.StringNull(),
		},
		{
			name: "color predefined BLUE round-trips to lowercase token",
			opts: customWith(predefined(cxsdk.DashboardSectionColorPredefinedColor(3))),
			want: types.StringValue("blue"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, diags := flattenDashboardOptions(context.Background(), tc.opts)
			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %s", diags.Errors())
			}
			if got == nil {
				t.Fatalf("expected non-nil options model")
			}
			if !got.Color.Equal(tc.want) {
				t.Fatalf("Color mismatch: want %q, got %q", tc.want.String(), got.Color.String())
			}
		})
	}
}
