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

// The three wrapper fields that lost a static default must be plain optional: a
// gauge min and max, and a data table column width. The API leaves each out when
// it is unset, so plain optional round trips, removing the attribute hands the
// value back to the API, and no default invents a value on import. Computed is
// the part that must not come back: a computed attribute that is null in
// configuration plans as "known after apply" on every run, which is a plan that
// is never empty.
func TestWrapperValuesArePlainOptional(t *testing.T) {
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
			if !optional || computed {
				t.Errorf("%s must be plain optional, got optional=%t computed=%t", name, optional, computed)
			}
			if modifiers != 0 {
				t.Errorf("%s has a plan modifier, so a value the user removed cannot be cleared", name)
			}
		})
	}
}

// The widget highlight is the opposite case. The API returns a value for every
// widget, so plain optional would fail the apply with "was null, but now false".
// It has to be computed, and a computed attribute needs the prior value or every
// plan reports "known after apply". Writing false is how a user stops
// highlighting a widget.
func TestWidgetHighlightedKeepsItsPriorValue(t *testing.T) {
	t.Parallel()
	attribute := dashboardResolveAttribute(t, dashboard_schema.V4().Attributes,
		"layout", "sections", "rows", "widgets", "highlighted")
	highlighted, ok := attribute.(schema.BoolAttribute)
	if !ok {
		t.Fatalf("highlighted is %T, want a bool attribute", attribute)
	}
	if !highlighted.Optional || !highlighted.Computed {
		t.Errorf("highlighted must be optional and computed, got optional=%t computed=%t", highlighted.Optional, highlighted.Computed)
	}
	if highlighted.Default != nil {
		t.Error("highlighted must not declare a static default")
	}
	if len(highlighted.PlanModifiers) == 0 {
		t.Error("highlighted has no plan modifier, so every plan reports known after apply")
	}
}
