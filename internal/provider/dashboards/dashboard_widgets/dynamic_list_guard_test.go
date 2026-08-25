// Copyright 2025 Coralogix Ltd.
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
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// A list whose read direction returns null for zero elements cannot accept an
// explicit empty value: the plan passes and the apply then fails with
// "inconsistent result after apply ... was cty.ListValEmpty(...), but now null".
// So every such list needs a minimum-size validator.
//
// This is walked from the schema rather than grepped from source, because a list
// declared by a shared helper has no attribute-name prefix to match on and gets
// missed. That is how two of these shipped unguarded.
//
// The exceptions below are lists whose flatten returns a *known empty* list, so
// empty round-trips and a guard would reject a valid configuration.
var dynamicListsWhereEmptyRoundTrips = map[string]string{
	"dynamic.query_definitions[*].query.logs.filters[*].operator.selected_values":  "an empty selection means all values; flattened as a known empty list",
	"dynamic.query_definitions[*].query.spans.filters[*].operator.selected_values": "an empty selection means all values; flattened as a known empty list",
}

func TestDynamicWidgetListsRejectExplicitEmpty(t *testing.T) {
	ctx := context.Background()

	var checked int
	var walk func(path string, attributes map[string]schema.Attribute)
	walk = func(path string, attributes map[string]schema.Attribute) {
		for name, attribute := range attributes {
			current := path + "." + name

			switch typed := attribute.(type) {
			case schema.ListNestedAttribute:
				checked++
				assertListGuarded(ctx, t, current, typed.Validators)
				walk(current+"[*]", typed.NestedObject.Attributes)
			case schema.ListAttribute:
				checked++
				assertListGuarded(ctx, t, current, typed.Validators)
			case schema.SingleNestedAttribute:
				walk(current, typed.Attributes)
			case schema.SetNestedAttribute:
				walk(current+"[*]", typed.NestedObject.Attributes)
			}
		}
	}

	dynamic, ok := DynamicSchema().(schema.SingleNestedAttribute)
	if !ok {
		t.Fatalf("expected the dynamic widget schema to be a single nested attribute, got %T", DynamicSchema())
	}
	walk("dynamic", dynamic.Attributes)

	if checked == 0 {
		t.Fatal("no list attribute was checked; the schema or this walk changed shape")
	}
	t.Logf("checked %d list attribute(s), %d documented exception(s)", checked, len(dynamicListsWhereEmptyRoundTrips))
}

func assertListGuarded[V interface{ Description(context.Context) string }](ctx context.Context, t *testing.T, path string, validators []V) {
	t.Helper()

	for _, validator := range validators {
		if strings.Contains(validator.Description(ctx), "at least 1") {
			if reason, ok := dynamicListsWhereEmptyRoundTrips[path]; ok {
				t.Errorf("%s is listed as round-tripping empty (%s) but carries a minimum-size validator; remove one or the other", path, reason)
			}
			return
		}
	}

	if _, ok := dynamicListsWhereEmptyRoundTrips[path]; ok {
		return
	}

	t.Errorf("%s has no minimum-size validator: an explicit empty list would pass the plan and fail the apply. "+
		"Add listvalidator.SizeBetween(1, 1000), or record it in dynamicListsWhereEmptyRoundTrips with the reason its flatten returns a known empty list.", path)
}

// Every repeated field in the dashboard protos is documented as at most 1000
// items, so each list needs an upper bound as well as a lower one. Walked from
// the schema because a list declared by a shared helper carries no attribute
// name to grep for.
func TestDynamicWidgetListsAreCappedAtTheDocumentedMaximum(t *testing.T) {
	ctx := context.Background()

	var checked int
	var walk func(path string, attributes map[string]schema.Attribute)
	walk = func(path string, attributes map[string]schema.Attribute) {
		for name, attribute := range attributes {
			current := path + "." + name
			switch typed := attribute.(type) {
			case schema.ListNestedAttribute:
				checked++
				assertListCapped(ctx, t, current, typed.Validators)
				walk(current+"[*]", typed.NestedObject.Attributes)
			case schema.ListAttribute:
				checked++
				assertListCapped(ctx, t, current, typed.Validators)
			case schema.SingleNestedAttribute:
				walk(current, typed.Attributes)
			case schema.SetNestedAttribute:
				walk(current+"[*]", typed.NestedObject.Attributes)
			}
		}
	}
	walk("dynamic", DynamicSchema().(schema.SingleNestedAttribute).Attributes)

	if checked == 0 {
		t.Fatal("no list attribute was checked; the schema or this walk changed shape")
	}
	t.Logf("checked %d list attribute(s)", checked)
}

// Lists declared by a helper that the legacy widgets also use. Their protos
// document the same 1000-item cap, but those resources have shipped for years
// and the API does not enforce it, so capping them is a deliberate decision
// with its own changelog warning rather than a side effect of this change.
var dynamicListsDeclaredBySharedHelpers = map[string]string{
	"keypath":         "ObservationFieldSchema, also used by the data table and hexagon widgets",
	"columns":         "LegendSchema, also used by the line chart and hexagon widgets",
	"filters":         "LogsFiltersSchema and SpansFilterSchema, also used by the data table, line chart and hexagon widgets",
	"selected_values": "FilterOperatorSchema, reached from the legacy widgets through LogsFiltersSchema",
}

// Deliberately does not consult dynamicListsWhereEmptyRoundTrips: that map
// exempts a list from needing a *minimum*, because empty round-trips for it.
// The documented maximum is a separate question.
func assertListCapped[V interface{ Description(context.Context) string }](ctx context.Context, t *testing.T, path string, validators []V) {
	t.Helper()

	leaf := path[strings.LastIndex(path, ".")+1:]
	if _, ok := dynamicListsDeclaredBySharedHelpers[strings.TrimSuffix(leaf, "[*]")]; ok {
		return
	}

	for _, validator := range validators {
		description := validator.Description(ctx)
		if strings.Contains(description, "at most") || strings.Contains(description, "between") {
			return
		}
	}

	t.Errorf("%s has no maximum-size validator: the API documents every repeated field as at most 1000 items. "+
		"Use listvalidator.SizeBetween(1, 1000).", path)
}

// The dynamic widget documents custom_unit as 1-128 characters and a tooltip
// message_template as 1-4096, and the schema declares them through helpers so
// the limit lives in one place. This catches an inline declaration slipping
// back in without one, which is how they were missed originally.
func TestDynamicWidgetStringLimitsAreEnforced(t *testing.T) {
	ctx := context.Background()

	bounded := map[string]bool{"custom_unit": true, "message_template": true}

	var checked int
	var walk func(path string, attributes map[string]schema.Attribute)
	walk = func(path string, attributes map[string]schema.Attribute) {
		for name, attribute := range attributes {
			current := path + "." + name
			switch typed := attribute.(type) {
			case schema.StringAttribute:
				if !bounded[name] {
					continue
				}
				checked++
				if !hasLengthBound(ctx, typed.Validators) {
					t.Errorf("%s has no length validator: the API documents a maximum for it, "+
						"and the schema should use DynamicCustomUnitSchema or DynamicMessageTemplateSchema.", current)
				}
			case schema.SingleNestedAttribute:
				walk(current, typed.Attributes)
			case schema.ListNestedAttribute:
				walk(current+"[*]", typed.NestedObject.Attributes)
			case schema.SetNestedAttribute:
				walk(current+"[*]", typed.NestedObject.Attributes)
			}
		}
	}
	walk("dynamic", DynamicSchema().(schema.SingleNestedAttribute).Attributes)

	if checked == 0 {
		t.Fatal("no length-limited attribute was checked; the schema or this walk changed shape")
	}
	t.Logf("checked %d length-limited attribute(s)", checked)
}

func hasLengthBound(ctx context.Context, validators []validator.String) bool {
	for _, v := range validators {
		if strings.Contains(v.Description(ctx), "length") {
			return true
		}
	}
	return false
}
