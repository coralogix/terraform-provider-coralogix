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
	"testing"

	dashboardwidgets "github.com/coralogix/terraform-provider-coralogix/internal/provider/dashboards/dashboard_widgets"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestHorizontalBarChartLogsQueryObservationFieldsRoundTrip covers the two
// attributes the horizontal bar chart's logs query expand used to leave out.
// The schema had them and the read filled them in, so a configuration that set
// them failed the apply with "provider produced inconsistent result after
// apply", and the value was silently never sent.
func TestHorizontalBarChartLogsQueryObservationFieldsRoundTrip(t *testing.T) {
	ctx := t.Context()

	observationField := types.ObjectValueMust(dashboardwidgets.ObservationFieldsObject().AttrTypes, map[string]attr.Value{
		"keypath": types.ListValueMust(types.StringType, []attr.Value{types.StringValue("subsystemname")}),
		"scope":   types.StringValue("label"),
	})
	logs := types.ObjectValueMust(barChartLogsQueryAttr(), map[string]attr.Value{
		"lucene_query":             types.StringNull(),
		"aggregation":              types.ObjectNull(dashboardwidgets.AggregationModelAttr()),
		"filters":                  types.ListNull(types.ObjectType{AttrTypes: dashboardwidgets.LogsFilterModelAttr()}),
		"group_names":              types.ListNull(types.StringType),
		"group_names_fields":       types.ListValueMust(dashboardwidgets.ObservationFieldsObject(), []attr.Value{observationField}),
		"stacked_group_name":       types.StringNull(),
		"stacked_group_name_field": observationField,
		"time_frame":               types.ObjectNull(dashboardwidgets.TimeFrameModelAttr()),
	})

	expanded, diags := expandHorizontalBarChartLogsQuery(ctx, logs)
	if diags.HasError() {
		t.Fatalf("expandHorizontalBarChartLogsQuery() diagnostics = %v", diags)
	}
	if len(expanded.GetGroupNamesFields()) != 1 {
		t.Fatalf("group_names_fields did not reach the API: %+v", expanded.GroupNamesFields)
	}
	if expanded.StackedGroupNameField == nil {
		t.Fatal("stacked_group_name_field did not reach the API")
	}

	flattened, diags := flattenHorizontalBarChartQueryLogs(ctx, expanded)
	if diags.HasError() {
		t.Fatalf("flattenHorizontalBarChartQueryLogs() diagnostics = %v", diags)
	}
	readBack := flattened.Logs.Attributes()
	if readBack["group_names_fields"].IsNull() {
		t.Fatal("group_names_fields read back as null, so an apply would report an inconsistent result")
	}
	if readBack["stacked_group_name_field"].IsNull() {
		t.Fatal("stacked_group_name_field read back as null")
	}
}
