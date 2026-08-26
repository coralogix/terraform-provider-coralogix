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
	"testing"

	dashboardservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_service"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestDataTableMetricsQueryEditorModeRoundTrip(t *testing.T) {
	ctx := t.Context()

	metrics := &QueryMetricsModel{
		PromqlQuery:     types.StringValue("up"),
		Filters:         types.ListNull(types.ObjectType{AttrTypes: MetricsFilterModelAttr()}),
		PromqlQueryType: types.StringValue("range"),
		EditorMode:      types.StringValue("builder"),
	}

	expanded, diags := expandDataTableMetricsQuery(ctx, metrics)
	if diags.HasError() {
		t.Fatalf("expandDataTableMetricsQuery() diagnostics = %v", diags)
	}
	if got := expanded.GetEditorMode(); got != dashboardservice.METRICSQUERYEDITORMODE_METRICS_QUERY_EDITOR_MODE_BUILDER {
		t.Fatalf("expanded EditorMode = %v, want BUILDER", got)
	}

	flattened, diags := flattenDataTableMetricsQuery(ctx, expanded)
	if diags.HasError() {
		t.Fatalf("flattenDataTableMetricsQuery() diagnostics = %v", diags)
	}
	if !flattened.Metrics.EditorMode.Equal(metrics.EditorMode) {
		t.Fatalf("round-tripped editor_mode = %v, want %v", flattened.Metrics.EditorMode, metrics.EditorMode)
	}
}
