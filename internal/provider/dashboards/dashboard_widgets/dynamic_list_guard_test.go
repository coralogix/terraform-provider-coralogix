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
		"Add listvalidator.SizeAtLeast(1), or record it in dynamicListsWhereEmptyRoundTrips with the reason its flatten returns a known empty list.", path)
}
