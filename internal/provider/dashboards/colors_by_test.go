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

func TestColorsByRoundTrip(t *testing.T) {
	for _, testCase := range []struct {
		schemaValue string
		restBranch  string
	}{
		{schemaValue: "stack", restBranch: "stack"},
		{schemaValue: "group_by", restBranch: "groupBy"},
		{schemaValue: "aggregation", restBranch: "aggregation"},
		{schemaValue: "query", restBranch: "query"},
		{schemaValue: "category", restBranch: "category"},
	} {
		t.Run(testCase.schemaValue, func(t *testing.T) {
			expanded := expandColorsBy(types.StringValue(testCase.schemaValue))
			if expanded == nil {
				t.Fatalf("expandColorsBy(%q) = nil, want the %q branch", testCase.schemaValue, testCase.restBranch)
			}

			populated := colorsByPopulatedBranches(expanded)
			if len(populated) != 1 || populated[0] != testCase.restBranch {
				t.Fatalf("expandColorsBy(%q) populated branches = %v, want exactly [%q]", testCase.schemaValue, populated, testCase.restBranch)
			}

			got, dg := flattenBarChartColorsBy(expanded)
			if dg != nil {
				t.Fatalf("flattenBarChartColorsBy(%q) returned diagnostic %v", testCase.schemaValue, dg)
			}
			if got != types.StringValue(testCase.schemaValue) {
				t.Fatalf("flattenBarChartColorsBy round trip = %v, want %q", got, testCase.schemaValue)
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
