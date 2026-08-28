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

	"github.com/coralogix/terraform-provider-coralogix/internal/provider/dashboards/dashboard_schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// Four attributes must not declare a static default and must not preserve prior
// state: a gauge min and max, a data table column width, and a widget highlight.
// The first three are wrapper fields in the API, where an absent value is not
// zero, and the fourth is a plain bool. A static default invents a value the
// dashboard never had. State preservation is worse: prior state cannot tell an
// attribute the user removed from one never set, so keeping it would make the
// value impossible to clear by deleting the line.
func TestWrapperValuesHaveNoDefaultAndNoStatePreservation(t *testing.T) {
	t.Parallel()
	root := dashboard_schema.V4()
	widget := []string{"layout", "sections", "rows", "widgets", "definition"}

	for name, path := range map[string][]string{
		"gauge min":               append(append([]string{}, widget...), "gauge", "min"),
		"gauge max":               append(append([]string{}, widget...), "gauge", "max"),
		"data table column width": append(append([]string{}, widget...), "data_table", "columns", "width"),
		"widget highlighted":      {"layout", "sections", "rows", "widgets", "highlighted"},
	} {
		t.Run(name, func(t *testing.T) {
			attribute := dashboardResolveAttribute(t, root.Attributes, path...)
			if attribute == nil {
				t.Fatalf("%s is missing", name)
			}
			var hasDefault bool
			var optional, computed bool
			var modifiers int
			switch typed := attribute.(type) {
			case schema.Float64Attribute:
				hasDefault, optional, computed, modifiers = typed.Default != nil, typed.Optional, typed.Computed, len(typed.PlanModifiers)
			case schema.Int64Attribute:
				hasDefault, optional, computed, modifiers = typed.Default != nil, typed.Optional, typed.Computed, len(typed.PlanModifiers)
			case schema.BoolAttribute:
				hasDefault, optional, computed, modifiers = typed.Default != nil, typed.Optional, typed.Computed, len(typed.PlanModifiers)
			default:
				t.Fatalf("%s has unexpected kind %T", name, attribute)
			}
			if hasDefault {
				t.Errorf("%s declares a static default, which invents a value the API never had", name)
			}
			if !optional || !computed {
				t.Errorf("%s must stay optional and computed, got optional=%t computed=%t", name, optional, computed)
			}
			if modifiers != 0 {
				t.Errorf("%s has a plan modifier, so a value the user removed cannot be cleared", name)
			}
		})
	}
}
