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
	"math/big"
	"strings"
	"testing"

	dashboardschema "github.com/coralogix/terraform-provider-coralogix/internal/provider/dashboards/dashboard_schema"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// This file follows the pattern established by
// TestDashboardStructuredConfigurationRejectsUnsupportedAutoRefreshBranches
// in resource_coralogix_dashboard_test.go: pull the real attribute out of
// dashboard_schema.V4() and call its own validators directly, rather than
// hand-rolling a copy of the schema shape. Unlike that test, several oneof
// groups here nest object-typed children, so the helpers below build
// present-but-otherwise-empty tftypes.Value objects for a "set" branch:
// exactlyOneOfChildrenValidator only inspects whether a named child is
// null/unknown/set, never its grandchildren, so grandchildren are left null.

// dashboardResolveAttribute walks a chain of attribute names starting from
// attrs, stepping transparently through SingleNestedAttribute and
// ListNestedAttribute/SetNestedAttribute containers (a list/set level
// doesn't consume a path segment of its own), and returns the schema
// attribute found at the last segment without further unwrapping it, so
// callers can assert its exact kind.
func dashboardResolveAttribute(t *testing.T, attrs map[string]schema.Attribute, segments ...string) schema.Attribute {
	t.Helper()
	current := attrs
	var found schema.Attribute
	for i, name := range segments {
		attribute, ok := current[name]
		if !ok {
			t.Fatalf("no attribute %q at segment %d of %v", name, i, segments)
		}
		found = attribute
		if i == len(segments)-1 {
			break
		}
		switch a := attribute.(type) {
		case schema.SingleNestedAttribute:
			current = a.Attributes
		case schema.ListNestedAttribute:
			current = a.NestedObject.Attributes
		case schema.SetNestedAttribute:
			current = a.NestedObject.Attributes
		default:
			t.Fatalf("attribute %q at segment %d of %v is not a nested container (%T)", name, i, segments, attribute)
		}
	}
	return found
}

// dashboardSetValue builds a tftypes.Value representing "this branch of the
// oneof is set": for object types, present-but-not-null with every one of
// its own attributes null (exactlyOneOfChildrenValidator only inspects
// null/unknown-ness of the direct children it's given, never grandchildren,
// so grandchildren don't need to be recursively populated); for scalar leaf
// types (folder's id/path are plain strings, not nested objects), a
// concrete non-null value, since a scalar can't be "present but empty" the
// way an object can.
func dashboardSetValue(t tftypes.Type) tftypes.Value {
	if obj, ok := t.(tftypes.Object); ok {
		inner := make(map[string]tftypes.Value, len(obj.AttributeTypes))
		for name, innerType := range obj.AttributeTypes {
			inner[name] = tftypes.NewValue(innerType, nil)
		}
		return tftypes.NewValue(t, inner)
	}
	switch {
	case t.Is(tftypes.String):
		return tftypes.NewValue(t, "set")
	case t.Is(tftypes.Bool):
		return tftypes.NewValue(t, true)
	case t.Is(tftypes.Number):
		return tftypes.NewValue(t, big.NewFloat(0))
	default:
		return tftypes.NewValue(t, nil)
	}
}

// dashboardObjectConfig builds a types.Object valid for objectType, setting
// each name in setNames to a present-empty value (see
// dashboardSetValue) and leaving every other attribute null.
func dashboardObjectConfig(ctx context.Context, t *testing.T, objectType attr.Type, setNames ...string) types.Object {
	t.Helper()
	set := make(map[string]bool, len(setNames))
	for _, name := range setNames {
		set[name] = true
	}
	tfType, ok := objectType.TerraformType(ctx).(tftypes.Object)
	if !ok {
		t.Fatalf("attribute type %T is not an object", objectType)
	}
	rawAttrs := make(map[string]tftypes.Value, len(tfType.AttributeTypes))
	for name, childType := range tfType.AttributeTypes {
		if set[name] {
			rawAttrs[name] = dashboardSetValue(childType)
		} else {
			rawAttrs[name] = tftypes.NewValue(childType, nil)
		}
	}
	val, err := objectType.ValueFromTerraform(ctx, tftypes.NewValue(tfType, rawAttrs))
	if err != nil {
		t.Fatalf("build object config: %s", err)
	}
	obj, ok := val.(types.Object)
	if !ok {
		t.Fatalf("config value is %T, not types.Object", val)
	}
	return obj
}

// dashboardObjectConfigNull builds the null types.Object of objectType,
// representing a wholly-omitted optional block rather than one that's
// present-but-empty.
func dashboardObjectConfigNull(ctx context.Context, t *testing.T, objectType attr.Type) types.Object {
	t.Helper()
	tfType, ok := objectType.TerraformType(ctx).(tftypes.Object)
	if !ok {
		t.Fatalf("attribute type %T is not an object", objectType)
	}
	val, err := objectType.ValueFromTerraform(ctx, tftypes.NewValue(tfType, nil))
	if err != nil {
		t.Fatalf("build null object config: %s", err)
	}
	obj, ok := val.(types.Object)
	if !ok {
		t.Fatalf("config value is %T, not types.Object", val)
	}
	return obj
}

func dashboardValidateObject(ctx context.Context, cfg types.Object, at path.Path, validators []validator.Object) []diagDetail {
	req := validator.ObjectRequest{ConfigValue: cfg, Path: at}
	resp := &validator.ObjectResponse{}
	for _, v := range validators {
		v.ValidateObject(ctx, req, resp)
	}
	details := make([]diagDetail, len(resp.Diagnostics))
	for i, d := range resp.Diagnostics {
		details[i] = diagDetail{path: at, detail: d.Detail()}
	}
	return details
}

type diagDetail struct {
	path   path.Path
	detail string
}

// dashboardMustType asserts v has static type T, failing the test
// immediately with a message naming what was being asserted. It centralizes
// the assert-or-fail branch so call sites read as a single expression
// instead of each contributing its own branch to cyclomatic complexity.
func dashboardMustType[T any](t *testing.T, v any, what string) T {
	t.Helper()
	x, ok := v.(T)
	if !ok {
		t.Fatalf("%s is %T, not %T", what, v, *new(T))
	}
	return x
}

// dashboardChildValue returns a tftypes.Value for objType with exactly
// setName populated to value and every other attribute null. It's the
// one-level building block for composing multi-level oneof configs (e.g.
// source.manual.strategy.range) without repeating the same
// build-a-map/range/if-else shape at each level.
func dashboardChildValue(objType tftypes.Object, setName string, value tftypes.Value) tftypes.Value {
	raw := make(map[string]tftypes.Value, len(objType.AttributeTypes))
	for name, childType := range objType.AttributeTypes {
		if name == setName {
			raw[name] = value
		} else {
			raw[name] = tftypes.NewValue(childType, nil)
		}
	}
	return tftypes.NewValue(objType, raw)
}

// dashboardRequireDetailNames fails the test unless detail names every one
// of names (each wrapped in backticks, matching
// exactlyOneOfChildrenValidator's error format).
func dashboardRequireDetailNames(t *testing.T, detail string, names ...string) {
	t.Helper()
	for _, name := range names {
		if !strings.Contains(detail, "`"+name+"`") {
			t.Fatalf("diagnostic does not name %q: %s", name, detail)
		}
	}
}

// TestDashboardWidgetDefinitionExactlyOneOfAtFullSchemaLevel drives the
// widget-type 8-way oneof through the real "definition" attribute pulled out
// of dashboard_schema.V4(), confirming the diagnostic surfaces at the parent
// "definition" path (it moved there from a per-child path as part of the
// ExactlyOneOfChildren migration) and still names every configured branch.
func TestDashboardWidgetDefinitionExactlyOneOfAtFullSchemaLevel(t *testing.T) {
	ctx := context.Background()
	root := dashboardschema.V4()
	definitionAttr := dashboardResolveAttribute(t, root.Attributes, "layout", "sections", "rows", "widgets", "definition")
	definition, ok := definitionAttr.(schema.SingleNestedAttribute)
	if !ok {
		t.Fatal("widget definition is not a single nested attribute")
	}
	if !definition.Required {
		t.Fatal("widget definition is not Required; the zero-set case below would not be a distinct, reachable state")
	}

	definitionPath := path.Root("definition")

	t.Run("two_widget_types_set", func(t *testing.T) {
		cfg := dashboardObjectConfig(ctx, t, definition.GetType(), "gauge", "line_chart")
		diagnostics := dashboardValidateObject(ctx, cfg, definitionPath, definition.Validators)
		if len(diagnostics) != 1 {
			t.Fatalf("diagnostics = %d, want 1: %v", len(diagnostics), diagnostics)
		}
		if !diagnostics[0].path.Equal(definitionPath) {
			t.Fatalf("diagnostic path = %s, want %s", diagnostics[0].path, definitionPath)
		}
		for _, name := range []string{"gauge", "line_chart"} {
			if !strings.Contains(diagnostics[0].detail, "`"+name+"`") {
				t.Fatalf("diagnostic does not name %q: %s", name, diagnostics[0].detail)
			}
		}
	})

	t.Run("zero_widget_types_set", func(t *testing.T) {
		cfg := dashboardObjectConfig(ctx, t, definition.GetType())
		diagnostics := dashboardValidateObject(ctx, cfg, definitionPath, definition.Validators)
		if len(diagnostics) != 1 {
			t.Fatalf("diagnostics = %d, want 1: %v", len(diagnostics), diagnostics)
		}
		if !strings.Contains(diagnostics[0].detail, "No attribute was configured") {
			t.Fatalf("unexpected diagnostic: %s", diagnostics[0].detail)
		}
	})
}

// TestDashboardGaugeLogsTimeFrameExactlyOneOf exercises TimeFrameSchema() at
// one real call site (gauge's query.logs.time_frame), the most reused
// oneof group in the tree. It also confirms time_frame = {} (present,
// neither branch set) is a distinct, reachable error from omitting
// time_frame entirely, which is valid since the whole block is Optional.
func TestDashboardGaugeLogsTimeFrameExactlyOneOf(t *testing.T) {
	ctx := context.Background()
	root := dashboardschema.V4()
	timeFrameAttr := dashboardResolveAttribute(t, root.Attributes,
		"layout", "sections", "rows", "widgets", "definition", "gauge", "query", "logs", "time_frame")
	timeFrame, ok := timeFrameAttr.(schema.SingleNestedAttribute)
	if !ok {
		t.Fatal("gauge query.logs.time_frame is not a single nested attribute")
	}
	if !timeFrame.Optional {
		t.Fatal("gauge query.logs.time_frame is not Optional; the omitted case below would not apply")
	}

	timeFramePath := path.Root("time_frame")

	t.Run("both_absolute_and_relative_set", func(t *testing.T) {
		cfg := dashboardObjectConfig(ctx, t, timeFrame.GetType(), "absolute", "relative")
		diagnostics := dashboardValidateObject(ctx, cfg, timeFramePath, timeFrame.Validators)
		if len(diagnostics) != 1 {
			t.Fatalf("diagnostics = %d, want 1: %v", len(diagnostics), diagnostics)
		}
		for _, name := range []string{"absolute", "relative"} {
			if !strings.Contains(diagnostics[0].detail, "`"+name+"`") {
				t.Fatalf("diagnostic does not name %q: %s", name, diagnostics[0].detail)
			}
		}
	})

	t.Run("present_but_empty_time_frame", func(t *testing.T) {
		cfg := dashboardObjectConfig(ctx, t, timeFrame.GetType())
		diagnostics := dashboardValidateObject(ctx, cfg, timeFramePath, timeFrame.Validators)
		if len(diagnostics) != 1 {
			t.Fatalf("diagnostics = %d, want 1: %v", len(diagnostics), diagnostics)
		}
		if !strings.Contains(diagnostics[0].detail, "No attribute was configured") {
			t.Fatalf("unexpected diagnostic: %s", diagnostics[0].detail)
		}
	})

	t.Run("omitted_time_frame_is_valid", func(t *testing.T) {
		cfg := dashboardObjectConfigNull(ctx, t, timeFrame.GetType())
		diagnostics := dashboardValidateObject(ctx, cfg, timeFramePath, timeFrame.Validators)
		if len(diagnostics) != 0 {
			t.Fatalf("expected no diagnostics when time_frame is omitted entirely, got: %v", diagnostics)
		}
	})
}

// TestDashboardFolderExactlyOneOf covers the user-facing, documented
// "folder" attribute independently of the internal wiring test: both id and
// path set is a conflict, and folder = {} (present, neither set) is the
// distinct zero-set error.
func TestDashboardFolderExactlyOneOf(t *testing.T) {
	ctx := context.Background()
	root := dashboardschema.V4()
	folderAttr := dashboardResolveAttribute(t, root.Attributes, "folder")
	folder, ok := folderAttr.(schema.SingleNestedAttribute)
	if !ok {
		t.Fatal("folder is not a single nested attribute")
	}

	folderPath := path.Root("folder")

	t.Run("both_id_and_path_set", func(t *testing.T) {
		cfg := dashboardObjectConfig(ctx, t, folder.GetType(), "id", "path")
		diagnostics := dashboardValidateObject(ctx, cfg, folderPath, folder.Validators)
		if len(diagnostics) != 1 {
			t.Fatalf("diagnostics = %d, want 1: %v", len(diagnostics), diagnostics)
		}
		for _, name := range []string{"id", "path"} {
			if !strings.Contains(diagnostics[0].detail, "`"+name+"`") {
				t.Fatalf("diagnostic does not name %q: %s", name, diagnostics[0].detail)
			}
		}
	})

	t.Run("present_but_empty_folder", func(t *testing.T) {
		cfg := dashboardObjectConfig(ctx, t, folder.GetType())
		diagnostics := dashboardValidateObject(ctx, cfg, folderPath, folder.Validators)
		if len(diagnostics) != 1 {
			t.Fatalf("diagnostics = %d, want 1: %v", len(diagnostics), diagnostics)
		}
		if !strings.Contains(diagnostics[0].detail, "No attribute was configured") {
			t.Fatalf("unexpected diagnostic: %s", diagnostics[0].detail)
		}
	})
}

// TestDashboardAnnotationSourceAndManualStrategyExactlyOneOf covers the
// annotation "source" 4-way oneof (metrics/logs/spans/manual) and the nested
// manual "strategy" 2-way oneof (instant/range), and confirms they're
// independent: setting source.manual with only manual.strategy.range set is
// valid for both groups at once, neither validator interfering with the
// other.
func TestDashboardAnnotationSourceAndManualStrategyExactlyOneOf(t *testing.T) {
	ctx := context.Background()
	root := dashboardschema.V4()

	source := dashboardMustType[schema.SingleNestedAttribute](t,
		dashboardResolveAttribute(t, root.Attributes, "annotations", "source"), "annotation source")
	strategy := dashboardMustType[schema.SingleNestedAttribute](t,
		dashboardResolveAttribute(t, root.Attributes, "annotations", "source", "manual", "strategy"), "manual annotation strategy")

	sourcePath := path.Root("source")
	strategyPath := path.Root("source").AtName("manual").AtName("strategy")

	t.Run("source_zero_set", func(t *testing.T) {
		cfg := dashboardObjectConfig(ctx, t, source.GetType())
		diagnostics := dashboardValidateObject(ctx, cfg, sourcePath, source.Validators)
		if len(diagnostics) != 1 || !strings.Contains(diagnostics[0].detail, "No attribute was configured") {
			t.Fatalf("unexpected diagnostics for zero-set source: %v", diagnostics)
		}
	})

	t.Run("source_two_set", func(t *testing.T) {
		cfg := dashboardObjectConfig(ctx, t, source.GetType(), "logs", "manual")
		diagnostics := dashboardValidateObject(ctx, cfg, sourcePath, source.Validators)
		if len(diagnostics) != 1 {
			t.Fatalf("diagnostics = %d, want 1: %v", len(diagnostics), diagnostics)
		}
		dashboardRequireDetailNames(t, diagnostics[0].detail, "logs", "manual")
	})

	t.Run("strategy_zero_set", func(t *testing.T) {
		cfg := dashboardObjectConfig(ctx, t, strategy.GetType())
		diagnostics := dashboardValidateObject(ctx, cfg, strategyPath, strategy.Validators)
		if len(diagnostics) != 1 || !strings.Contains(diagnostics[0].detail, "No attribute was configured") {
			t.Fatalf("unexpected diagnostics for zero-set strategy: %v", diagnostics)
		}
	})

	t.Run("strategy_two_set", func(t *testing.T) {
		cfg := dashboardObjectConfig(ctx, t, strategy.GetType(), "instant", "range")
		diagnostics := dashboardValidateObject(ctx, cfg, strategyPath, strategy.Validators)
		if len(diagnostics) != 1 {
			t.Fatalf("diagnostics = %d, want 1: %v", len(diagnostics), diagnostics)
		}
		dashboardRequireDetailNames(t, diagnostics[0].detail, "instant", "range")
	})

	t.Run("manual_with_only_strategy_range_set_is_valid_at_both_levels", func(t *testing.T) {
		sourceType := dashboardMustType[tftypes.Object](t, source.GetType().TerraformType(ctx), "source type")
		manualType := dashboardMustType[tftypes.Object](t, sourceType.AttributeTypes["manual"], "manual type")
		strategyType := dashboardMustType[tftypes.Object](t, manualType.AttributeTypes["strategy"], "strategy type")

		strategyRaw := dashboardChildValue(strategyType, "range", dashboardSetValue(strategyType.AttributeTypes["range"]))
		manualRaw := dashboardChildValue(manualType, "strategy", strategyRaw)
		sourceRaw := dashboardChildValue(sourceType, "manual", manualRaw)

		sourceVal, err := source.GetType().ValueFromTerraform(ctx, sourceRaw)
		if err != nil {
			t.Fatalf("build source config: %s", err)
		}
		sourceCfg := dashboardMustType[types.Object](t, sourceVal, "source config value")

		if diagnostics := dashboardValidateObject(ctx, sourceCfg, sourcePath, source.Validators); len(diagnostics) != 0 {
			t.Fatalf("expected source group to accept manual-only, got: %v", diagnostics)
		}

		manualObj := dashboardMustType[types.Object](t, sourceCfg.Attributes()["manual"], "source config's manual attribute")
		strategyObj := dashboardMustType[types.Object](t, manualObj.Attributes()["strategy"], "manual config's strategy attribute")
		if diagnostics := dashboardValidateObject(ctx, strategyObj, strategyPath, strategy.Validators); len(diagnostics) != 0 {
			t.Fatalf("expected strategy group to accept range-only, got: %v", diagnostics)
		}
	})
}

// TestDashboardDataPrimeFiltersExactlyOneOfPerListElement covers the
// genuinely different shape of FiltersSourceSchema() inside a
// ListNestedAttribute's NestedObject (gauge's query.data_prime.filters):
// the validator runs once per list element, so a real per-element path
// (constructed the same way the framework itself would, via AtListIndex)
// must identify which index failed.
func TestDashboardDataPrimeFiltersExactlyOneOfPerListElement(t *testing.T) {
	ctx := context.Background()
	root := dashboardschema.V4()
	filtersAttr := dashboardResolveAttribute(t, root.Attributes,
		"layout", "sections", "rows", "widgets", "definition", "gauge", "query", "data_prime", "filters")
	filters, ok := filtersAttr.(schema.ListNestedAttribute)
	if !ok {
		t.Fatal("gauge query.data_prime.filters is not a list nested attribute")
	}
	validators := filters.NestedObject.Validators
	if len(validators) != 1 {
		t.Fatalf("gauge query.data_prime.filters element validators = %d, want 1", len(validators))
	}
	elementType := types.ObjectType{AttrTypes: attrTypesOf(t, filters.NestedObject.Attributes)}

	filtersPath := path.Root("filters")

	t.Run("zero_elements_is_a_no_op", func(t *testing.T) {
		// An empty list has no elements to walk, so the per-element
		// validator is never invoked and there is nothing to assert beyond
		// "this does not panic" — documented here as a no-op rather than
		// silently assumed.
	})

	t.Run("one_element_two_sources_set", func(t *testing.T) {
		cfg := dashboardObjectConfig(ctx, t, elementType, "logs", "metrics")
		elementPath := filtersPath.AtListIndex(0)
		diagnostics := dashboardValidateObject(ctx, cfg, elementPath, validators)
		if len(diagnostics) != 1 {
			t.Fatalf("diagnostics = %d, want 1: %v", len(diagnostics), diagnostics)
		}
		if !diagnostics[0].path.Equal(elementPath) {
			t.Fatalf("diagnostic path = %s, want %s", diagnostics[0].path, elementPath)
		}
		for _, name := range []string{"logs", "metrics"} {
			if !strings.Contains(diagnostics[0].detail, "`"+name+"`") {
				t.Fatalf("diagnostic does not name %q: %s", name, diagnostics[0].detail)
			}
		}
	})

	t.Run("one_element_zero_sources_set", func(t *testing.T) {
		cfg := dashboardObjectConfig(ctx, t, elementType)
		elementPath := filtersPath.AtListIndex(0)
		diagnostics := dashboardValidateObject(ctx, cfg, elementPath, validators)
		if len(diagnostics) != 1 {
			t.Fatalf("diagnostics = %d, want 1: %v", len(diagnostics), diagnostics)
		}
		if !diagnostics[0].path.Equal(elementPath) {
			t.Fatalf("diagnostic path = %s, want %s", diagnostics[0].path, elementPath)
		}
		if !strings.Contains(diagnostics[0].detail, "No attribute was configured") {
			t.Fatalf("unexpected diagnostic: %s", diagnostics[0].detail)
		}
	})
}

func attrTypesOf(t *testing.T, attrs map[string]schema.Attribute) map[string]attr.Type {
	t.Helper()
	out := make(map[string]attr.Type, len(attrs))
	for name, attribute := range attrs {
		out[name] = attribute.GetType()
	}
	return out
}
