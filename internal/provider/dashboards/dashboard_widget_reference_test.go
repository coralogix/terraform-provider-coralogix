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
	dashboardschema "github.com/coralogix/terraform-provider-coralogix/internal/provider/dashboards/dashboard_schema"
	dashboardwidgets "github.com/coralogix/terraform-provider-coralogix/internal/provider/dashboards/dashboard_widgets"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestDashboardWidgetDefinitionOrReferenceExactlyOneOf(t *testing.T) {
	ctx := context.Background()
	root := dashboardschema.V4()
	widgetsAttr := dashboardResolveAttribute(t, root.Attributes, "layout", "sections", "rows", "widgets")
	widgets, ok := widgetsAttr.(schema.ListNestedAttribute)
	if !ok {
		t.Fatal("widgets is not a list nested attribute")
	}

	widgetPath := path.Root("widgets").AtListIndex(0)
	validators := widgets.NestedObject.Validators

	t.Run("both_definition_and_reference_set", func(t *testing.T) {
		cfg := dashboardObjectConfig(ctx, t, widgets.NestedObject.Type(), "definition", "reference")
		diagnostics := dashboardValidateObject(t, ctx, cfg, widgetPath, validators)
		if len(diagnostics) != 1 {
			t.Fatalf("diagnostics = %d, want 1: %v", len(diagnostics), diagnostics)
		}
		dashboardRequireDetailNames(t, diagnostics[0].detail, "definition", "reference")
	})

	t.Run("neither_definition_nor_reference_set", func(t *testing.T) {
		cfg := dashboardObjectConfig(ctx, t, widgets.NestedObject.Type())
		diagnostics := dashboardValidateObject(t, ctx, cfg, widgetPath, validators)
		if len(diagnostics) != 1 {
			t.Fatalf("diagnostics = %d, want 1: %v", len(diagnostics), diagnostics)
		}
		if !strings.Contains(diagnostics[0].detail, "No attribute was configured") {
			t.Fatalf("unexpected diagnostic: %s", diagnostics[0].detail)
		}
	})

	t.Run("reference_only_is_valid", func(t *testing.T) {
		cfg := dashboardObjectConfig(ctx, t, widgets.NestedObject.Type(), "reference")
		diagnostics := dashboardValidateObject(t, ctx, cfg, widgetPath, validators)
		if len(diagnostics) != 0 {
			t.Fatalf("expected no diagnostics for reference-only widget, got: %v", diagnostics)
		}
	})
}

func TestExpandFlattenWidgetReferenceRoundTrip(t *testing.T) {
	ctx := context.Background()
	dashboardID := "abcdefghijklmnopqrstu"
	widgetID := "83aed974-510b-43be-bd19-c92daf56beff"
	localID := "11111111-2222-3333-4444-555555555555"
	title := "local-title"
	description := "local-description"

	expanded, diags := expandWidget(ctx, WidgetModel{
		ID:          types.StringValue(localID),
		Title:       types.StringValue(title),
		Description: types.StringValue(description),
		Width:       types.Int64Value(4),
		Reference: &WidgetReferenceModel{
			DashboardID: types.StringValue(dashboardID),
			WidgetID:    types.StringValue(widgetID),
		},
	})
	dashboardRequireNoDiags(t, diags, "expandWidget")
	dashboardRequireExpandedReference(t, expanded, dashboardID, widgetID, title, description)

	flattened, diags := flattenDashboardWidget(ctx, expanded)
	dashboardRequireNoDiags(t, diags, "flattenDashboardWidget")
	dashboardRequireFlattenedReference(t, flattened, localID, dashboardID, widgetID, title, description)
}

func dashboardRequireNoDiags(t *testing.T, diags interface{ HasError() bool }, label string) {
	t.Helper()
	if diags.HasError() {
		t.Fatalf("%s: %v", label, diags)
	}
}

func dashboardRequireExpandedReference(t *testing.T, expanded *dashboardservice.Widget, dashboardID, widgetID, title, description string) {
	t.Helper()
	if expanded.Definition != nil {
		t.Fatal("expected Definition to be nil for reference widgets")
	}
	if expanded.Title == nil || *expanded.Title != title {
		t.Fatalf("expected title %q, got %#v", title, expanded.Title)
	}
	if expanded.Description == nil || *expanded.Description != description {
		t.Fatalf("expected description %q, got %#v", description, expanded.Description)
	}
	if expanded.Reference == nil || expanded.Reference.GetDashboardId() != dashboardID {
		t.Fatalf("unexpected reference dashboard id: %#v", expanded.Reference)
	}
	if expanded.Reference.WidgetId == nil || expanded.Reference.WidgetId.Value == nil || *expanded.Reference.WidgetId.Value != widgetID {
		t.Fatalf("unexpected reference widget id: %#v", expanded.Reference.WidgetId)
	}
}

func dashboardRequireFlattenedReference(t *testing.T, flattened *WidgetModel, localID, dashboardID, widgetID, title, description string) {
	t.Helper()
	if flattened.Definition != nil {
		t.Fatal("flatten must not invent a definition from a reference")
	}
	if flattened.Title.ValueString() != title {
		t.Fatalf("title = %q, want %q", flattened.Title.ValueString(), title)
	}
	if flattened.Description.ValueString() != description {
		t.Fatalf("description = %q, want %q", flattened.Description.ValueString(), description)
	}
	if flattened.Reference == nil {
		t.Fatal("expected reference in flattened state")
	}
	if flattened.Reference.DashboardID.ValueString() != dashboardID {
		t.Fatalf("dashboard_id = %q, want %q", flattened.Reference.DashboardID.ValueString(), dashboardID)
	}
	if flattened.Reference.WidgetID.ValueString() != widgetID {
		t.Fatalf("widget_id = %q, want %q", flattened.Reference.WidgetID.ValueString(), widgetID)
	}
	if flattened.ID.ValueString() != localID {
		t.Fatalf("local id = %q, want %q", flattened.ID.ValueString(), localID)
	}
}

func TestFlattenWidgetReferenceOnly(t *testing.T) {
	ctx := context.Background()
	dashboardID := "abcdefghijklmnopqrstu"
	widgetID := "83aed974-510b-43be-bd19-c92daf56beff"
	localID := "11111111-2222-3333-4444-555555555555"

	ref := dashboardservice.NewWidgetReference()
	ref.SetDashboardId(dashboardID)
	ref.SetWidgetId(dashboardservice.UUID{Value: &widgetID})

	widget := &dashboardservice.Widget{
		Id:        &dashboardservice.UUID{Value: &localID},
		Reference: ref,
	}

	flattened, diags := flattenDashboardWidget(ctx, widget)
	if diags.HasError() {
		t.Fatalf("flattenDashboardWidget: %v", diags)
	}
	if flattened.Definition != nil {
		t.Fatal("expected definition nil when API returns only a reference")
	}
	if flattened.Reference == nil || flattened.Reference.DashboardID.ValueString() != dashboardID {
		t.Fatalf("expected reference preserve, got %#v", flattened.Reference)
	}
}

func TestExpandFlattenInlineDefinitionStillWorks(t *testing.T) {
	ctx := context.Background()
	markdownText := "hello"
	expanded, diags := expandWidget(ctx, WidgetModel{
		ID:    types.StringValue("11111111-2222-3333-4444-555555555555"),
		Title: types.StringNull(),
		Definition: &dashboardwidgets.WidgetDefinitionModel{
			Markdown: &dashboardwidgets.MarkdownModel{
				MarkdownText: types.StringValue(markdownText),
				TooltipText:  types.StringNull(),
			},
		},
		Width: types.Int64Value(0),
	})
	if diags.HasError() {
		t.Fatalf("expandWidget: %v", diags)
	}
	if expanded.Reference != nil {
		t.Fatal("expected Reference nil for inline definition")
	}
	if expanded.Definition == nil || expanded.Definition.Markdown == nil {
		t.Fatal("expected markdown definition")
	}

	flattened, diags := flattenDashboardWidget(ctx, expanded)
	if diags.HasError() {
		t.Fatalf("flattenDashboardWidget: %v", diags)
	}
	if flattened.Reference != nil {
		t.Fatal("expected Reference nil after flatten of inline widget")
	}
	if flattened.Definition == nil || flattened.Definition.Markdown == nil {
		t.Fatal("expected markdown definition after flatten")
	}
	if flattened.Definition.Markdown.MarkdownText.ValueString() != markdownText {
		t.Fatalf("markdown_text = %q, want %q", flattened.Definition.Markdown.MarkdownText.ValueString(), markdownText)
	}
}
