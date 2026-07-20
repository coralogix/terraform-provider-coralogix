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

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
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
