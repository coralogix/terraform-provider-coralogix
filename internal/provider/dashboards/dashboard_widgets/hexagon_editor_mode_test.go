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

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestHexagonMetricsQueryEditorModeRoundTrip covers the query editor a hexagon
// metrics query was written with. The Coralogix UI sets it to builder, so a
// request that never carries it turns an imported hexagon into a text query.
func TestHexagonMetricsQueryEditorModeRoundTrip(t *testing.T) {
	ctx := context.Background()

	model := &HexagonQueryMetricsModel{
		PromqlQuery:     types.StringValue("up"),
		Filters:         types.ListNull(types.ObjectType{AttrTypes: MetricsFilterModelAttr()}),
		PromqlQueryType: types.StringValue("instant"),
		Aggregation:     types.StringValue("sum"),
		EditorMode:      types.StringValue("builder"),
	}

	expanded, diags := expandHexagonMetricsQuery(ctx, model)
	if diags.HasError() {
		t.Fatalf("expandHexagonMetricsQuery() diagnostics = %v", diags)
	}
	if got := expanded.GetEditorMode(); got != "METRICS_QUERY_EDITOR_MODE_BUILDER" {
		t.Fatalf("editor_mode did not reach the API: %q", got)
	}

	flattened, diags := flattenHexagonMetricsQuery(ctx, expanded)
	if diags.HasError() {
		t.Fatalf("flattenHexagonMetricsQuery() diagnostics = %v", diags)
	}
	if got := flattened.Metrics.EditorMode; got.ValueString() != "builder" {
		t.Fatalf("round-tripped editor_mode = %v, want builder", got)
	}
}
