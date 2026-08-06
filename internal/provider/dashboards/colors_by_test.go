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

	dashboardservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_service"
	dashboardwidgets "github.com/coralogix/terraform-provider-coralogix/internal/provider/dashboards/dashboard_widgets"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func colorsByPopulatedBranches(colorsBy *dashboardservice.ColorsBy) []string {
	branches := make([]string, 0, 1)
	if colorsBy.Stack != nil {
		branches = append(branches, "stack")
	}
	if colorsBy.GroupBy != nil {
		branches = append(branches, "groupBy")
	}
	if colorsBy.Aggregation != nil {
		branches = append(branches, "aggregation")
	}
	if colorsBy.Query != nil {
		branches = append(branches, "query")
	}
	if colorsBy.Category != nil {
		branches = append(branches, "category")
	}
	return branches
}

// Keyed by the schema value the colors_by validator accepts; the value is the REST oneof branch
// it must map to. Driving the test from DashboardValidColorsBy means widening the validator
// without teaching expandColorsBy the new value fails here rather than silently at apply.
var colorsByRESTBranches = map[string]string{
	"stack":       "stack",
	"group_by":    "groupBy",
	"aggregation": "aggregation",
	"query":       "query",
	"category":    "category",
}

func TestColorsByRoundTrip(t *testing.T) {
	if len(colorsByRESTBranches) != len(dashboardwidgets.DashboardValidColorsBy) {
		t.Fatalf("colorsByRESTBranches covers %d values, DashboardValidColorsBy advertises %d: %v",
			len(colorsByRESTBranches), len(dashboardwidgets.DashboardValidColorsBy), dashboardwidgets.DashboardValidColorsBy)
	}

	for _, schemaValue := range dashboardwidgets.DashboardValidColorsBy {
		restBranch, ok := colorsByRESTBranches[schemaValue]
		if !ok {
			t.Fatalf("schema accepts colors_by = %q but this test has no expected REST branch for it", schemaValue)
		}

		t.Run(schemaValue, func(t *testing.T) {
			expanded := expandColorsBy(types.StringValue(schemaValue))
			if expanded == nil {
				t.Fatalf("expandColorsBy(%q) = nil, want the %q branch", schemaValue, restBranch)
			}

			populated := colorsByPopulatedBranches(expanded)
			if len(populated) != 1 || populated[0] != restBranch {
				t.Fatalf("expandColorsBy(%q) populated branches = %v, want exactly [%q]", schemaValue, populated, restBranch)
			}

			got, dg := flattenBarChartColorsBy(expanded)
			if dg != nil {
				t.Fatalf("flattenBarChartColorsBy(%q) returned diagnostic %v", schemaValue, dg)
			}
			if got != types.StringValue(schemaValue) {
				t.Fatalf("flattenBarChartColorsBy round trip = %v, want %q", got, schemaValue)
			}
		})
	}
}

func TestExpandColorsByUnsetOrUnknownStringIsNil(t *testing.T) {
	for name, value := range map[string]types.String{
		"null":     types.StringNull(),
		"unknown":  types.StringUnknown(),
		"empty":    types.StringValue(""),
		"unmapped": types.StringValue("rainbow"),
	} {
		t.Run(name, func(t *testing.T) {
			if got := expandColorsBy(value); got != nil {
				t.Fatalf("expandColorsBy(%v) = %+v, want nil so no colorsBy is sent", value, got)
			}
		})
	}
}
