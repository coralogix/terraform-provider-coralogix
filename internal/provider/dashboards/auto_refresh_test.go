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
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// The API models an auto refresh interval as one empty object per choice. All
// five have to round trip: one minute and fifteen minutes were missing, so a
// dashboard set to either could not be written or read.
func TestDashboardAutoRefreshEveryIntervalRoundTrip(t *testing.T) {
	ctx := context.Background()

	for _, interval := range []string{"off", "one_minute", "two_minutes", "five_minutes", "fifteen_minutes"} {
		t.Run(interval, func(t *testing.T) {
			refresh, diags := types.ObjectValueFrom(ctx, dashboardAutoRefreshModelAttr(),
				&DashboardAutoRefreshModel{Type: types.StringValue(interval)})
			if diags.HasError() {
				t.Fatalf("ObjectValueFrom() diagnostics = %v", diags)
			}

			expanded, diags := expandDashboardAutoRefresh(ctx, &dashboardservice.Dashboard{}, refresh)
			if diags.HasError() {
				t.Fatalf("expandDashboardAutoRefresh() diagnostics = %v", diags)
			}
			set := 0
			for _, branch := range []map[string]interface{}{
				expanded.Off, expanded.OneMinute, expanded.TwoMinutes,
				expanded.FiveMinutes, expanded.FifteenMinutes,
			} {
				if branch != nil {
					set++
				}
			}
			if set != 1 {
				t.Fatalf("want exactly one interval on the request, got %d", set)
			}

			flattened, diags := flattenDashboardAutoRefresh(ctx, expanded)
			if diags.HasError() {
				t.Fatalf("flattenDashboardAutoRefresh() diagnostics = %v", diags)
			}
			var readBack DashboardAutoRefreshModel
			if diags := flattened.As(ctx, &readBack, basetypes.ObjectAsOptions{}); diags.HasError() {
				t.Fatalf("As() diagnostics = %v", diags)
			}
			if readBack.Type.ValueString() != interval {
				t.Fatalf("round-tripped interval = %q, want %q", readBack.Type.ValueString(), interval)
			}
		})
	}
}
