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

	dashboardservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_service"
	dashboardschema "github.com/coralogix/terraform-provider-coralogix/internal/provider/dashboards/dashboard_schema"
	dashboardwidgets "github.com/coralogix/terraform-provider-coralogix/internal/provider/dashboards/dashboard_widgets"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// TestDashboardWidgetComputedPlanModifiersUnknownForNewWidget reproduces the
// "Provider produced inconsistent result after apply" failure when adding a
// widget to an existing dashboard.
//
// On resource update, the framework plans omitted Optional+Computed nested
// fields as unknown. UseStateForUnknown then copies null prior state for a new
// list element into the plan. After apply, flatten writes a server id / width
// and Terraform rejects the null→known transition.
//
// The same failure applies to every generated id under a list that can grow on
// update: sections, rows, widgets, line-chart query definitions, data-table
// aggregations and annotations. All of them, plus widget width, use
// UseNonNullStateForUnknown so a new element keeps an unknown plan, while an
// existing element still preserves non-null state.
func TestDashboardWidgetComputedPlanModifiersUnknownForNewWidget(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	root := dashboardschema.V4()

	stringCases := []struct {
		name     string
		segments []string
	}{
		{
			name:     "section_id",
			segments: []string{"layout", "sections", "id"},
		},
		{
			name:     "row_id",
			segments: []string{"layout", "sections", "rows", "id"},
		},
		{
			name:     "widget_id",
			segments: []string{"layout", "sections", "rows", "widgets", "id"},
		},
		{
			name:     "annotation_id",
			segments: []string{"annotations", "id"},
		},
		{
			name: "line_chart_query_definition_id",
			segments: []string{
				"layout", "sections", "rows", "widgets", "definition",
				"line_chart", "query_definitions", "id",
			},
		},
		{
			name: "data_table_logs_aggregation_id",
			segments: []string{
				"layout", "sections", "rows", "widgets", "definition",
				"data_table", "query", "logs", "grouping", "aggregations", "id",
			},
		},
		{
			name: "data_table_spans_aggregation_id",
			segments: []string{
				"layout", "sections", "rows", "widgets", "definition",
				"data_table", "query", "spans", "grouping", "aggregations", "id",
			},
		},
	}

	for _, tc := range stringCases {
		t.Run(tc.name+"/new_stays_unknown", func(t *testing.T) {
			t.Parallel()

			attr := dashboardMustType[schema.StringAttribute](t,
				dashboardResolveAttribute(t, root.Attributes, tc.segments...),
				tc.name,
			)
			req := planmodifier.StringRequest{
				ConfigValue: types.StringNull(),
				PlanValue:   types.StringUnknown(),
				State:       dashboardUpdateState(),
				StateValue:  types.StringNull(),
			}
			resp := &planmodifier.StringResponse{PlanValue: types.StringUnknown()}
			for _, modifier := range attr.PlanModifiers {
				modifier.PlanModifyString(ctx, req, resp)
				req.PlanValue = resp.PlanValue
			}
			if !resp.PlanValue.IsUnknown() {
				t.Fatalf("new %s plan = %#v, want unknown so Terraform accepts a server-generated id after apply "+
					"(UseStateForUnknown would copy null state and then fail: was null, but now cty.StringVal(...))",
					tc.name, resp.PlanValue)
			}
		})

		t.Run(tc.name+"/existing_keeps_state", func(t *testing.T) {
			t.Parallel()

			attr := dashboardMustType[schema.StringAttribute](t,
				dashboardResolveAttribute(t, root.Attributes, tc.segments...),
				tc.name,
			)
			state := types.StringValue("ad2ca57f-d76a-4940-bd0a-b9bd081649fe")
			req := planmodifier.StringRequest{
				ConfigValue: types.StringNull(),
				PlanValue:   types.StringUnknown(),
				State:       dashboardUpdateState(),
				StateValue:  state,
			}
			resp := &planmodifier.StringResponse{PlanValue: types.StringUnknown()}
			for _, modifier := range attr.PlanModifiers {
				modifier.PlanModifyString(ctx, req, resp)
				req.PlanValue = resp.PlanValue
			}
			if !resp.PlanValue.Equal(state) {
				t.Fatalf("existing %s plan = %#v, want prior state %#v", tc.name, resp.PlanValue, state)
			}
		})
	}

	t.Run("width/new_widget_stays_unknown", func(t *testing.T) {
		t.Parallel()

		attr := dashboardMustType[schema.Int64Attribute](t,
			dashboardResolveAttribute(t, root.Attributes, "layout", "sections", "rows", "widgets", "width"),
			"widget width",
		)
		req := planmodifier.Int64Request{
			ConfigValue: types.Int64Null(),
			PlanValue:   types.Int64Unknown(),
			State:       dashboardUpdateState(),
			StateValue:  types.Int64Null(),
		}
		resp := &planmodifier.Int64Response{PlanValue: types.Int64Unknown()}
		for _, modifier := range attr.PlanModifiers {
			modifier.PlanModifyInt64(ctx, req, resp)
			req.PlanValue = resp.PlanValue
		}
		if !resp.PlanValue.IsUnknown() {
			t.Fatalf("new widget width plan = %#v, want unknown so Terraform accepts API width after apply "+
				"(UseStateForUnknown would copy null state and then fail: was null, but now cty.NumberIntVal(0))", resp.PlanValue)
		}
	})

	t.Run("width/existing_widget_keeps_state", func(t *testing.T) {
		t.Parallel()

		attr := dashboardMustType[schema.Int64Attribute](t,
			dashboardResolveAttribute(t, root.Attributes, "layout", "sections", "rows", "widgets", "width"),
			"widget width",
		)
		state := types.Int64Value(0)
		req := planmodifier.Int64Request{
			ConfigValue: types.Int64Null(),
			PlanValue:   types.Int64Unknown(),
			State:       dashboardUpdateState(),
			StateValue:  state,
		}
		resp := &planmodifier.Int64Response{PlanValue: types.Int64Unknown()}
		for _, modifier := range attr.PlanModifiers {
			modifier.PlanModifyInt64(ctx, req, resp)
			req.PlanValue = resp.PlanValue
		}
		if !resp.PlanValue.Equal(state) {
			t.Fatalf("existing widget width plan = %#v, want prior state %#v", resp.PlanValue, state)
		}
	})
}

// dashboardUpdateState returns a plan-modifier request State that is non-null,
// which is what the framework passes on update. UseStateForUnknown returns
// early when the whole prior state is null (create), so without this the
// new-element subtests would pass with either modifier and prove nothing.
// Only State.Raw null-ness is read, so an empty object value is enough.
func dashboardUpdateState() tfsdk.State {
	return tfsdk.State{
		Raw: tftypes.NewValue(
			tftypes.Object{AttributeTypes: map[string]tftypes.Type{}},
			map[string]tftypes.Value{},
		),
	}
}

// TestFlattenDashboardWidgetWritesServerAssignedIdAndWidth documents the
// post-apply half of the add-widget inconsistency: the API assigns a widget
// id and returns appearance.width 0, and flatten writes both into state.
func TestFlattenDashboardWidgetWritesServerAssignedIdAndWidth(t *testing.T) {
	t.Parallel()

	widgetID := "ad2ca57f-d76a-4940-bd0a-b9bd081649fe"
	width := int32(0)
	title := "Single metrics gaug - 2"
	unit := dashboardservice.GAUGEUNIT_UNIT_NUMBER
	promql := "vector(1)"

	flattened, diags := flattenDashboardWidget(context.Background(), &dashboardservice.Widget{
		Id:    &dashboardservice.UUID{Value: &widgetID},
		Title: &title,
		Appearance: &dashboardservice.WidgetAppearance{
			Width: &width,
		},
		Definition: &dashboardservice.WidgetDefinition{
			Gauge: &dashboardservice.WidgetsGauge{
				Query: &dashboardservice.GaugeQuery{
					Metrics: &dashboardservice.GaugeMetricsQuery{
						PromqlQuery: &dashboardservice.PromQlQuery{Value: &promql},
					},
				},
				Unit: &unit,
			},
		},
	})
	if diags.HasError() {
		t.Fatalf("flattenDashboardWidget diagnostics = %v", diags)
	}
	if flattened.ID.ValueString() != widgetID {
		t.Fatalf("flattened id = %q, want server-assigned %q", flattened.ID.ValueString(), widgetID)
	}
	if flattened.Width.IsNull() || flattened.Width.ValueInt64() != 0 {
		t.Fatalf("flattened width = %#v, want 0 from API appearance.width", flattened.Width)
	}
}

// TestFlattenDashboardSectionWritesServerAssignedSectionAndRowIds documents the
// post-apply half of the add-section / add-row inconsistency: expandSection and
// expandRow generate a UUID for a section or row that has no id in config, and
// flatten writes those ids back into state as known strings.
func TestFlattenDashboardSectionWritesServerAssignedSectionAndRowIds(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sectionID := "1f3d9f0a-6c1e-4a35-9a2f-1d4b1f0a7c11"
	rowID := "2a4e8b1c-7d2f-4b46-8b3e-2e5c2a1b8d22"
	height := int32(19)

	flattened, diags := flattenDashboardSection(ctx, &dashboardservice.Section{
		Id: &dashboardservice.UUID{Value: &sectionID},
		Rows: []dashboardservice.Row{
			{
				Id:         &dashboardservice.UUID{Value: &rowID},
				Appearance: &dashboardservice.RowAppearance{Height: &height},
			},
		},
	})
	if diags.HasError() {
		t.Fatalf("flattenDashboardSection diagnostics = %v", diags)
	}
	if flattened.ID.ValueString() != sectionID {
		t.Fatalf("flattened section id = %q, want server-assigned %q", flattened.ID.ValueString(), sectionID)
	}

	var rows []RowModel
	diags = flattened.Rows.ElementsAs(ctx, &rows, false)
	if diags.HasError() {
		t.Fatalf("ElementsAs rows: %v", diags)
	}
	if len(rows) != 1 {
		t.Fatalf("rows len = %d, want 1", len(rows))
	}
	if rows[0].ID.IsNull() || rows[0].ID.IsUnknown() || rows[0].ID.ValueString() != rowID {
		t.Fatalf("flattened row id = %#v, want known server-assigned %q", rows[0].ID, rowID)
	}
}

// TestFlattenLineChartWritesServerAssignedQueryDefinitionId documents the
// post-apply half for line charts: the API assigns query_definitions[].id and
// flatten writes it as a known string. Combined with UseNonNullStateForUnknown,
// a new line-chart widget can accept that id after apply.
func TestFlattenLineChartWritesServerAssignedQueryDefinitionId(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	widgetID := "ad2ca57f-d76a-4940-bd0a-b9bd081649fe"
	queryDefID := "b1e2c3d4-e5f6-7890-abcd-ef1234567890"
	title := "line chart"
	promql := "vector(1)"
	unit := dashboardservice.COMMONUNIT_UNIT_UNSPECIFIED
	scale := dashboardservice.SCALETYPE_SCALE_TYPE_UNSPECIFIED
	dataMode := dashboardservice.WIDGETSCOMMONDATAMODETYPE_DATA_MODE_TYPE_HIGH_UNSPECIFIED

	flattened, diags := flattenDashboardWidget(ctx, &dashboardservice.Widget{
		Id:    &dashboardservice.UUID{Value: &widgetID},
		Title: &title,
		Definition: &dashboardservice.WidgetDefinition{
			LineChart: &dashboardservice.LineChart{
				QueryDefinitions: []dashboardservice.LineChartQueryDefinition{
					{
						Id: queryDefID,
						Query: dashboardservice.LineChartQuery{
							Metrics: &dashboardservice.LineChartMetricsQuery{
								PromqlQuery: &dashboardservice.PromQlQuery{Value: &promql},
							},
						},
						Unit:         &unit,
						ScaleType:    &scale,
						DataModeType: &dataMode,
					},
				},
			},
		},
	})
	if diags.HasError() {
		t.Fatalf("flattenDashboardWidget diagnostics = %v", diags)
	}
	if flattened.Definition == nil || flattened.Definition.LineChart == nil {
		t.Fatal("flattened definition.line_chart is nil")
	}

	var definitions []dashboardwidgets.LineChartQueryDefinitionModel
	diags = flattened.Definition.LineChart.QueryDefinitions.ElementsAs(ctx, &definitions, false)
	if diags.HasError() {
		t.Fatalf("ElementsAs query_definitions: %v", diags)
	}
	if len(definitions) != 1 {
		t.Fatalf("query_definitions len = %d, want 1", len(definitions))
	}
	if definitions[0].ID.IsNull() || definitions[0].ID.IsUnknown() || definitions[0].ID.ValueString() != queryDefID {
		t.Fatalf("query_definitions[0].id = %#v, want known server-assigned %q", definitions[0].ID, queryDefID)
	}
}
