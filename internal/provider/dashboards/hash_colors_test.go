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
	"strings"
	"testing"

	dashboardservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_service"
	dashboardschema "github.com/coralogix/terraform-provider-coralogix/internal/provider/dashboards/dashboard_schema"
	dashboardwidgets "github.com/coralogix/terraform-provider-coralogix/internal/provider/dashboards/dashboard_widgets"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// hashColorsCases drives every classic-widget round-trip below. A null value must reach the API
// as an absent field, because the provider models hash_colors as a plain Optional bool. Nothing
// may substitute false for "not set", or removing the attribute from HCL would show a diff.
var hashColorsCases = map[string]types.Bool{
	"true":  types.BoolValue(true),
	"false": types.BoolValue(false),
	"null":  types.BoolNull(),
}

func TestPieChartHashColorsRoundTrip(t *testing.T) {
	ctx := t.Context()

	for name, hashColors := range hashColorsCases {
		t.Run(name, func(t *testing.T) {
			expanded, diags := expandPieChart(ctx, &dashboardwidgets.PieChartModel{HashColors: hashColors})
			if diags.HasError() {
				t.Fatalf("expandPieChart() diagnostics = %v", diags)
			}
			assertExpandedHashColors(t, expanded.PieChart.HashColors, hashColors)

			flattened, diags := flattenPieChart(ctx, expanded.PieChart)
			if diags.HasError() {
				t.Fatalf("flattenPieChart() diagnostics = %v", diags)
			}
			assertFlattenedHashColors(t, flattened.PieChart.HashColors, hashColors)
		})
	}
}

func TestBarChartHashColorsRoundTrip(t *testing.T) {
	ctx := t.Context()

	for name, hashColors := range hashColorsCases {
		t.Run(name, func(t *testing.T) {
			model := &dashboardwidgets.BarChartModel{
				XAxis:      &dashboardwidgets.BarChartXAxisModel{Value: &dashboardwidgets.BarChartXAxisValueModel{}},
				HashColors: hashColors,
			}

			expanded, diags := expandBarChart(ctx, model)
			if diags.HasError() {
				t.Fatalf("expandBarChart() diagnostics = %v", diags)
			}
			assertExpandedHashColors(t, expanded.BarChart.HashColors, hashColors)

			flattened, diags := flattenBarChart(ctx, expanded.BarChart)
			if diags.HasError() {
				t.Fatalf("flattenBarChart() diagnostics = %v", diags)
			}
			assertFlattenedHashColors(t, flattened.BarChart.HashColors, hashColors)
		})
	}
}

func TestHorizontalBarChartHashColorsRoundTrip(t *testing.T) {
	ctx := t.Context()

	for name, hashColors := range hashColorsCases {
		t.Run(name, func(t *testing.T) {
			expanded, diags := expandHorizontalBarChart(ctx, &dashboardwidgets.HorizontalBarChartModel{HashColors: hashColors})
			if diags.HasError() {
				t.Fatalf("expandHorizontalBarChart() diagnostics = %v", diags)
			}
			assertExpandedHashColors(t, expanded.HorizontalBarChart.HashColors, hashColors)

			flattened, diags := flattenHorizontalBarChart(ctx, expanded.HorizontalBarChart)
			if diags.HasError() {
				t.Fatalf("flattenHorizontalBarChart() diagnostics = %v", diags)
			}
			assertFlattenedHashColors(t, flattened.HorizontalBarChart.HashColors, hashColors)
		})
	}
}

// A dashboard created before hash_colors existed comes back from the API without the field.
// Reading it must leave the attribute null, not false, or the first plan after an upgrade drifts.
func TestClassicWidgetHashColorsStaysNullWhenAPIOmitsIt(t *testing.T) {
	flattened, diags := flattenPieChart(t.Context(), &dashboardservice.WidgetsPieChart{})
	if diags.HasError() {
		t.Fatalf("flattenPieChart() diagnostics = %v", diags)
	}
	if !flattened.PieChart.HashColors.IsNull() {
		t.Fatalf("flattened hash_colors = %v, want null when the API omits the field", flattened.PieChart.HashColors)
	}
}

func assertExpandedHashColors(t *testing.T, got *bool, want types.Bool) {
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

func assertFlattenedHashColors(t *testing.T, got types.Bool, want types.Bool) {
	t.Helper()
	if !got.Equal(want) {
		t.Fatalf("round-tripped hash_colors = %v, want %v", got, want)
	}
}

// Schema v3 exists only to decode prior state, so its shape must stay frozen. v3 originally
// shared LineChartSchema() with v4, which meant adding hash_colors silently widened it too.
// v3 now uses the frozen LineChartSchemaV3(). This fails if anyone points it back at the
// shared helper, or adds hash_colors to the frozen copy.
func TestSchemaV3DoesNotGainHashColors(t *testing.T) {
	ctx := t.Context()

	v3 := dashboardschema.V3().Type().TerraformType(ctx).String()
	if count := strings.Count(v3, "hash_colors"); count != 0 {
		t.Fatalf("schema v3 declares hash_colors %d times, want 0 — v3 must stay frozen", count)
	}

	// Guards the other direction: the attribute really is wired on the current schema, so a
	// zero count above cannot be explained by the whole feature having been dropped.
	v4 := dashboardschema.V4().Type().TerraformType(ctx).String()
	if count := strings.Count(v4, "hash_colors"); count != 12 {
		t.Fatalf("schema v4 declares hash_colors %d times, want 12 (8 dynamic + 4 classic widgets)", count)
	}
}
