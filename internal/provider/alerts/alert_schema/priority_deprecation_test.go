// Copyright 2025 Coralogix Ltd.
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

package alertschema

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var priorityValidatorTypeNames = []string{
	"logs_immediate", "logs_threshold", "logs_anomaly", "logs_ratio_threshold",
	"logs_new_value", "logs_unique_count", "logs_time_relative_threshold",
	"metric_threshold", "metric_anomaly", "tracing_immediate", "tracing_threshold",
	"flow", "slo_threshold",
}

func priorityValidatorConfig(ctx context.Context, setType string, prioritySet bool) tfsdk.Config {
	emptyObjAttr := schema.SingleNestedAttribute{Optional: true, Attributes: map[string]schema.Attribute{}}
	tdAttrs := map[string]schema.Attribute{}
	for _, n := range priorityValidatorTypeNames {
		tdAttrs[n] = emptyObjAttr
	}
	s := schema.Schema{
		Attributes: map[string]schema.Attribute{
			"priority":        schema.StringAttribute{Optional: true},
			"type_definition": schema.SingleNestedAttribute{Optional: true, Attributes: tdAttrs},
		},
	}

	emptyObjType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{}}
	tdTypes := map[string]tftypes.Type{}
	tdValues := map[string]tftypes.Value{}
	for _, n := range priorityValidatorTypeNames {
		tdTypes[n] = emptyObjType
		if n == setType {
			tdValues[n] = tftypes.NewValue(emptyObjType, map[string]tftypes.Value{})
		} else {
			tdValues[n] = tftypes.NewValue(emptyObjType, nil)
		}
	}
	tdObjType := tftypes.Object{AttributeTypes: tdTypes}

	tdValue := tftypes.NewValue(tdObjType, tdValues)
	if setType == "" {
		tdValue = tftypes.NewValue(tdObjType, nil)
	}

	priorityValue := tftypes.NewValue(tftypes.String, "P1")
	if !prioritySet {
		priorityValue = tftypes.NewValue(tftypes.String, nil)
	}

	raw := tftypes.NewValue(s.Type().TerraformType(ctx), map[string]tftypes.Value{
		"priority":        priorityValue,
		"type_definition": tdValue,
	})
	return tfsdk.Config{Schema: s, Raw: raw}
}

func TestPriorityDeprecationWarning(t *testing.T) {
	cases := []struct {
		name        string
		setType     string
		prioritySet bool
		wantWarn    bool
	}{
		{"metric_threshold warns", "metric_threshold", true, true},
		{"slo_threshold warns", "slo_threshold", true, true},
		{"logs_threshold warns", "logs_threshold", true, true},
		{"logs_ratio_threshold warns", "logs_ratio_threshold", true, true},
		{"logs_time_relative_threshold warns", "logs_time_relative_threshold", true, true},
		{"metric_anomaly does not warn", "metric_anomaly", true, false},
		{"logs_immediate does not warn", "logs_immediate", true, false},
		{"tracing_threshold does not warn", "tracing_threshold", true, false},
		{"flow does not warn", "flow", true, false},
		{"null type_definition does not warn", "", true, false},
		{"priority unset does not warn", "metric_threshold", false, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			cfg := priorityValidatorConfig(ctx, tc.setType, tc.prioritySet)
			priorityValue := types.StringValue("P1")
			if !tc.prioritySet {
				priorityValue = types.StringNull()
			}
			req := validator.StringRequest{
				Path:        path.Root("priority"),
				ConfigValue: priorityValue,
				Config:      cfg,
			}
			resp := &validator.StringResponse{}
			PriorityDeprecationWarning{}.ValidateString(ctx, req, resp)

			if resp.Diagnostics.HasError() {
				t.Fatalf("unexpected error diagnostics: %v", resp.Diagnostics.Errors())
			}
			warnings := resp.Diagnostics.Warnings()
			if tc.wantWarn && len(warnings) != 1 {
				t.Fatalf("expected 1 deprecation warning, got %d: %v", len(warnings), warnings)
			}
			if !tc.wantWarn && len(warnings) != 0 {
				t.Fatalf("expected no warning, got %d: %v", len(warnings), warnings)
			}
		})
	}
}
