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

	dashboardschema "github.com/coralogix/terraform-provider-coralogix/internal/provider/dashboards/dashboard_schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// The API owns the value of these enums when the configuration omits them, so a
// static default cannot be used. On update the framework plans an omitted nested
// Optional+Computed attribute as unknown regardless of prior state, which left
// the plan never empty. UseNonNullStateForUnknown keeps what the API chose.
//
// A list element added on update has null prior state at its index. Copying that
// null into the plan would fail the apply once flatten writes a value, so the
// modifier must keep the plan unknown in that case.
func TestApiOwnedEnumsKeepTheirStateValue(t *testing.T) {
	ctx := context.Background()
	root := dashboardschema.V4()

	for _, attribute := range []struct {
		name     string
		segments []string
	}{
		{name: "line_chart.x_axis_time_format", segments: []string{"layout", "sections", "rows", "widgets", "definition", "line_chart", "x_axis_time_format"}},
		{name: "line_chart.metrics.editor_mode", segments: []string{"layout", "sections", "rows", "widgets", "definition", "line_chart", "query_definitions", "query", "metrics", "editor_mode"}},
		{name: "line_chart.metrics.series_limit_type", segments: []string{"layout", "sections", "rows", "widgets", "definition", "line_chart", "query_definitions", "query", "metrics", "series_limit_type"}},
		{name: "bar_chart.bar_value_display", segments: []string{"layout", "sections", "rows", "widgets", "definition", "bar_chart", "bar_value_display"}},
		{name: "bar_chart.x_axis_time_format", segments: []string{"layout", "sections", "rows", "widgets", "definition", "bar_chart", "x_axis_time_format"}},
		{name: "bar_chart.metrics.aggregation", segments: []string{"layout", "sections", "rows", "widgets", "definition", "bar_chart", "query", "metrics", "aggregation"}},
		{name: "gauge.legend_by", segments: []string{"layout", "sections", "rows", "widgets", "definition", "gauge", "legend_by"}},
		{name: "gauge.metrics.promql_query_type", segments: []string{"layout", "sections", "rows", "widgets", "definition", "gauge", "query", "metrics", "promql_query_type"}},
		{name: "pie_chart.metrics.editor_mode", segments: []string{"layout", "sections", "rows", "widgets", "definition", "pie_chart", "query", "metrics", "editor_mode"}},
		{name: "data_table.metrics.editor_mode", segments: []string{"layout", "sections", "rows", "widgets", "definition", "data_table", "query", "metrics", "editor_mode"}},
	} {
		t.Run(attribute.name, func(t *testing.T) {
			resolved := dashboardMustType[schema.StringAttribute](t,
				dashboardResolveAttribute(t, root.Attributes, attribute.segments...), attribute.name)

			if resolved.Default != nil {
				t.Fatal("attribute has a static default; the API decides the value when the attribute is omitted")
			}
			if len(resolved.PlanModifiers) == 0 {
				t.Fatal("attribute has no plan modifier, so an omitted value plans as unknown on every update")
			}

			if got := planEnum(ctx, resolved, types.StringValue("auto")); !got.Equal(types.StringValue("auto")) {
				t.Fatalf("planned value with non-null state = %v, want the state value", got)
			}
			if got := planEnum(ctx, resolved, types.StringNull()); !got.IsUnknown() {
				t.Fatalf("planned value for a new list element = %v, want unknown", got)
			}
		})
	}
}

// planEnum runs the attribute's plan modifiers for an omitted configuration.
// The State must hold a non-null Raw, or UseNonNullStateForUnknown returns early
// and a "stays unknown" assertion passes whatever the modifier is.
func planEnum(ctx context.Context, attribute schema.StringAttribute, stateValue types.String) types.String {
	response := &planmodifier.StringResponse{PlanValue: types.StringUnknown()}
	for _, modifier := range attribute.PlanModifiers {
		modifier.PlanModifyString(ctx, planmodifier.StringRequest{
			Path:        path.Root("enum"),
			ConfigValue: types.StringNull(),
			PlanValue:   response.PlanValue,
			StateValue:  stateValue,
			State: tfsdk.State{Raw: tftypes.NewValue(
				tftypes.Object{AttributeTypes: map[string]tftypes.Type{}},
				map[string]tftypes.Value{},
			)},
		}, response)
	}
	return response.PlanValue
}
