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

	"github.com/coralogix/terraform-provider-coralogix/internal/provider/dashboards/dashboard_widgets"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// A highlighted widget is marked for every user of the dashboard, so the value
// has to reach the API and read back. It used to be dropped in both directions.
func TestWidgetHighlightedRoundTrip(t *testing.T) {
	ctx := context.Background()

	for name, highlighted := range map[string]types.Bool{
		"true":  types.BoolValue(true),
		"false": types.BoolValue(false),
		"null":  types.BoolNull(),
	} {
		t.Run(name, func(t *testing.T) {
			model := &WidgetModel{
				ID:          types.StringValue("11111111-1111-1111-1111-111111111111"),
				Title:       types.StringValue("w"),
				Highlighted: highlighted,
				Definition: &dashboard_widgets.WidgetDefinitionModel{
					Markdown: &dashboard_widgets.MarkdownModel{
						MarkdownText: types.StringValue("x"),
					},
				},
			}

			expanded, diags := expandWidget(ctx, *model)
			if diags.HasError() {
				t.Fatalf("expandWidget() diagnostics = %v", diags)
			}
			if highlighted.IsNull() {
				if expanded.Highlighted != nil {
					t.Fatalf("a null highlighted must send nothing, sent %v", *expanded.Highlighted)
				}
			} else if expanded.Highlighted == nil || *expanded.Highlighted != highlighted.ValueBool() {
				t.Fatalf("highlighted did not reach the API: %v", expanded.Highlighted)
			}

			flattened, diags := flattenDashboardWidget(ctx, expanded)
			if diags.HasError() {
				t.Fatalf("flattenDashboardWidget() diagnostics = %v", diags)
			}
			if !flattened.Highlighted.Equal(highlighted) {
				t.Fatalf("round-tripped highlighted = %v, want %v", flattened.Highlighted, highlighted)
			}
		})
	}
}
