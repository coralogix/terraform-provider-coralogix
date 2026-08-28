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

// Three attributes lost a static default, because each is a wrapper field in the
// API where an absent value is not zero: a gauge min and max, and a data table
// column width. Dropping the default alone would change an existing
// configuration, because the value the default put in state would be cleared on
// the next apply. Each one keeps a non-null prior state instead, so a
// configuration written against the old default keeps its value and plans clean.
func TestWrapperDefaultsAreReplacedByStatePreservation(t *testing.T) {
	t.Parallel()
	root := dashboard_schema.V4()
	widget := []string{"layout", "sections", "rows", "widgets", "definition"}

	for name, path := range map[string][]string{
		"gauge min":               append(append([]string{}, widget...), "gauge", "min"),
		"gauge max":               append(append([]string{}, widget...), "gauge", "max"),
		"data table column width": append(append([]string{}, widget...), "data_table", "columns", "width"),
	} {
		t.Run(name, func(t *testing.T) {
			attribute := dashboardResolveAttribute(t, root.Attributes, path...)
			if attribute == nil {
				t.Fatalf("%s is missing", name)
			}
			switch typed := attribute.(type) {
			case schema.Float64Attribute:
				if typed.Default != nil {
					t.Errorf("%s still declares a static default", name)
				}
				if !typed.Optional || !typed.Computed {
					t.Errorf("%s must stay optional and computed, got optional=%t computed=%t", name, typed.Optional, typed.Computed)
				}
				if len(typed.PlanModifiers) == 0 {
					t.Errorf("%s has no plan modifier, so a stored value is cleared on the next apply", name)
				}
			case schema.Int64Attribute:
				if typed.Default != nil {
					t.Errorf("%s still declares a static default", name)
				}
				if !typed.Optional || !typed.Computed {
					t.Errorf("%s must stay optional and computed, got optional=%t computed=%t", name, typed.Optional, typed.Computed)
				}
				if len(typed.PlanModifiers) == 0 {
					t.Errorf("%s has no plan modifier, so a stored value is cleared on the next apply", name)
				}
			default:
				t.Fatalf("%s has unexpected kind %T", name, attribute)
			}
		})
	}
}
