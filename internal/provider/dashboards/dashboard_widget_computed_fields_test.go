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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
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
// Widget id/width use UseNonNullStateForUnknown so a new widget keeps an
// unknown plan, while an existing widget still preserves non-null state.
func TestDashboardWidgetComputedPlanModifiersUnknownForNewWidget(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	root := dashboardschema.V4()

	t.Run("id/new_widget_stays_unknown", func(t *testing.T) {
		t.Parallel()

		attr := dashboardMustType[schema.StringAttribute](t,
			dashboardResolveAttribute(t, root.Attributes, "layout", "sections", "rows", "widgets", "id"),
			"widget id",
		)
		req := planmodifier.StringRequest{
			ConfigValue: types.StringNull(),
			PlanValue:   types.StringUnknown(),
			StateValue:  types.StringNull(),
		}
		resp := &planmodifier.StringResponse{PlanValue: types.StringUnknown()}
		for _, modifier := range attr.PlanModifiers {
			modifier.PlanModifyString(ctx, req, resp)
			req.PlanValue = resp.PlanValue
		}
		if !resp.PlanValue.IsUnknown() {
			t.Fatalf("new widget id plan = %#v, want unknown so Terraform accepts a server-generated id after apply "+
				"(UseStateForUnknown would copy null state and then fail: was null, but now cty.StringVal(...))", resp.PlanValue)
		}
	})

	t.Run("id/existing_widget_keeps_state", func(t *testing.T) {
		t.Parallel()

		attr := dashboardMustType[schema.StringAttribute](t,
			dashboardResolveAttribute(t, root.Attributes, "layout", "sections", "rows", "widgets", "id"),
			"widget id",
		)
		state := types.StringValue("ad2ca57f-d76a-4940-bd0a-b9bd081649fe")
		req := planmodifier.StringRequest{
			ConfigValue: types.StringNull(),
			PlanValue:   types.StringUnknown(),
			StateValue:  state,
		}
		resp := &planmodifier.StringResponse{PlanValue: types.StringUnknown()}
		for _, modifier := range attr.PlanModifiers {
			modifier.PlanModifyString(ctx, req, resp)
			req.PlanValue = resp.PlanValue
		}
		if !resp.PlanValue.Equal(state) {
			t.Fatalf("existing widget id plan = %#v, want prior state %#v", resp.PlanValue, state)
		}
	})

	t.Run("width/new_widget_stays_unknown", func(t *testing.T) {
		t.Parallel()

		attr := dashboardMustType[schema.Int64Attribute](t,
			dashboardResolveAttribute(t, root.Attributes, "layout", "sections", "rows", "widgets", "width"),
			"widget width",
		)
		req := planmodifier.Int64Request{
			ConfigValue: types.Int64Null(),
			PlanValue:   types.Int64Unknown(),
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
