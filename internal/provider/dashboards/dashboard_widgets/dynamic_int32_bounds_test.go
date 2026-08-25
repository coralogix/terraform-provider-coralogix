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
	"math"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Every int64 attribute that reaches the API through expandInt32Pointer needs an
// upper bound, because that conversion is an unchecked cast: without one, a
// value above the int32 range silently wraps negative between plan and apply.
// A lower bound alone is not enough, which is how these regressed once.
// Attributes that reach the API as something other than an int32, so the
// conversion cannot truncate and any int64 is safe.
var dynamicInt64AttributesNotBackedByInt32 = map[string]string{
	"dynamic.visualization.time_series_lines.series_count_limit":                                 "sent as a string, so there is no int32 conversion",
	"dynamic.visualization.time_series_lines_multi.query_display_settings[*].series_count_limit": "sent as a string, so there is no int32 conversion",
}

func TestDynamicWidgetInt32AttributesAreBoundedAbove(t *testing.T) {
	ctx := context.Background()

	// Confirm the conversion really does wrap, so this guard is not theatre.
	if got := expandInt32Pointer(types.Int64Value(math.MaxInt32 + 1)); got == nil || *got >= 0 {
		t.Fatalf("expected the int32 conversion to wrap negative, got %v", got)
	}

	var checked int
	var walk func(path string, attributes map[string]schema.Attribute)
	walk = func(path string, attributes map[string]schema.Attribute) {
		for name, attribute := range attributes {
			current := path + "." + name
			switch typed := attribute.(type) {
			case schema.Int64Attribute:
				checked++
				assertBoundedAbove(ctx, t, current, typed)
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
		t.Fatal("no int64 attribute was checked; the schema or this walk changed shape")
	}
	t.Logf("checked %d int64 attribute(s)", checked)
}

func assertBoundedAbove(ctx context.Context, t *testing.T, path string, attribute schema.Int64Attribute) {
	t.Helper()

	if _, ok := dynamicInt64AttributesNotBackedByInt32[path]; ok {
		return
	}

	for _, validator := range attribute.Validators {
		// "between x and y" and "at most y" both bound above; "at least x" does not.
		description := validator.Description(ctx)
		if strings.Contains(description, "at most") || strings.Contains(description, "must be between") {
			return
		}
	}

	t.Errorf("%s has no upper bound: a value above the int32 range would wrap negative on the way to the API. "+
		"Use Between(min, math.MaxInt32) rather than AtLeast(min).", path)
}
