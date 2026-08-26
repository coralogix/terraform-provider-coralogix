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

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// hashColorsCases is shared by every hash_colors round-trip test. A null value must reach the
// API as an absent field, because the provider models hash_colors as a plain Optional bool.
var hashColorsCases = map[string]types.Bool{
	"true":  types.BoolValue(true),
	"false": types.BoolValue(false),
	"null":  types.BoolNull(),
}

func TestLineChartQueryDefinitionHashColorsRoundTrip(t *testing.T) {
	ctx := context.Background()

	for name, hashColors := range hashColorsCases {
		t.Run(name, func(t *testing.T) {
			model := &LineChartQueryDefinitionModel{
				ID: types.StringValue("11111111-1111-1111-1111-111111111111"),
				Query: &LineChartQueryModel{
					Metrics: &LineChartQueryMetricsModel{
						PromqlQuery: types.StringValue("http_requests_total"),
						Filters:     types.ListNull(types.ObjectType{AttrTypes: MetricsFilterModelAttr()}),
					},
				},
				Resolution: types.ObjectNull(lineChartQueryResolutionModelAttr()),
				HashColors: hashColors,
			}

			expanded, diags := expandLineChartQueryDefinition(ctx, model)
			if diags.HasError() {
				t.Fatalf("expandLineChartQueryDefinition() diagnostics = %v", diags)
			}
			assertHashColorsPointer(t, expanded.HashColors, hashColors)

			flattened, diags := flattenLineChartQueryDefinition(ctx, expanded)
			if diags.HasError() {
				t.Fatalf("flattenLineChartQueryDefinition() diagnostics = %v", diags)
			}
			if !flattened.HashColors.Equal(hashColors) {
				t.Fatalf("round-tripped hash_colors = %v, want %v", flattened.HashColors, hashColors)
			}
		})
	}
}

// The attr-type map must list hash_colors, otherwise converting the model into the
// types.List element type panics with a struct/object mismatch.
func TestLineChartQueryDefinitionModelAttrCoversHashColors(t *testing.T) {
	ctx := context.Background()
	attrTypes := lineChartQueryDefinitionModelAttr()

	if got, ok := attrTypes["hash_colors"]; !ok || !got.Equal(types.BoolType) {
		t.Fatalf("lineChartQueryDefinitionModelAttr()[\"hash_colors\"] = %v, ok = %t, want types.BoolType", got, ok)
	}

	model := LineChartQueryDefinitionModel{
		ID:         types.StringValue("11111111-1111-1111-1111-111111111111"),
		Resolution: types.ObjectNull(lineChartQueryResolutionModelAttr()),
		HashColors: types.BoolValue(true),
	}
	object, diags := types.ObjectValueFrom(ctx, attrTypes, &model)
	if diags.HasError() {
		t.Fatalf("types.ObjectValueFrom() diagnostics = %v", diags)
	}
	if _, diags := types.ListValue(types.ObjectType{AttrTypes: attrTypes}, []attr.Value{object}); diags.HasError() {
		t.Fatalf("types.ListValue() diagnostics = %v", diags)
	}
}

func assertHashColorsPointer(t *testing.T, got *bool, want types.Bool) {
	t.Helper()
	if want.IsNull() {
		if got != nil {
			t.Fatalf("expanded HashColors = %t, want nil so the field is omitted from the request", *got)
		}
		return
	}
	if got == nil {
		t.Fatalf("expanded HashColors = nil, want %t", want.ValueBool())
	}
	if *got != want.ValueBool() {
		t.Fatalf("expanded HashColors = %t, want %t", *got, want.ValueBool())
	}
}
