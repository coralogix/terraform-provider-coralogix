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
	seen := map[string]bool{}
	capped := map[string]bool{}
	var walk func(path string, attributes map[string]schema.Attribute)
	walk = func(path string, attributes map[string]schema.Attribute) {
		for name, attribute := range attributes {
			current := path + "." + name
			switch typed := attribute.(type) {
			case schema.ListNestedAttribute:
				checked++
				seen[current] = true
				capped[current] = listHasMaximum(ctx, typed.Validators)
				assertListCapped(ctx, t, current, typed.Validators)
				walk(current+"[*]", typed.NestedObject.Attributes)
			case schema.ListAttribute:
				checked++
				seen[current] = true
				capped[current] = listHasMaximum(ctx, typed.Validators)
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

	// A 72-entry path map rots quietly, so assert it describes the schema as it
	// is: every exempt path must still exist and must still be uncapped. An
	// entry that gains a cap, or a path that disappears, fails here rather than
	// silently waiving a list that no longer needs waiving.
	for path, reason := range dynamicListsDeclaredBySharedHelpers {
		switch {
		case !seen[path]:
			t.Errorf("%s is exempted (%s) but is not a list in the schema; remove the entry", path, reason)
		case capped[path]:
			t.Errorf("%s is exempted (%s) but now carries a maximum; remove the entry", path, reason)
		}
	}

	t.Logf("checked %d list attribute(s), %d shared-helper exception(s)", checked, len(dynamicListsDeclaredBySharedHelpers))
}

// Lists declared by a helper that the legacy widgets also use. Their protos
// document the same 1000-item cap, but those resources have shipped for years
// and the API does not enforce it, so capping them is a deliberate change with
// its own changelog warning rather than a side effect of this one.
//
// Keyed by exact path, not by attribute name: several dynamic-owned lists share
// a name with one of these - dynamic.visualization.table.columns against a
// legend's columns, for instance - and a name-based exemption would have let
// their caps be removed unnoticed.
const (
	observationFieldHelper = "ObservationFieldSchema, also used by the data table and hexagon widgets"
	legendHelper           = "LegendSchema, also used by the line chart and hexagon widgets"
	filtersHelper          = "LogsFiltersSchema and SpansFilterSchema, also used by the data table, line chart and hexagon widgets"
	filterOperatorHelper   = "FilterOperatorSchema, reached from the legacy widgets through LogsFiltersSchema"
)

var dynamicListsDeclaredBySharedHelpers = map[string]string{
	"dynamic.query_definitions[*].query.logs.aggregations[*].observation_field.keypath":                  observationFieldHelper,
	"dynamic.query_definitions[*].query.logs.filters":                                                    filtersHelper,
	"dynamic.query_definitions[*].query.logs.filters[*].observation_field.keypath":                       observationFieldHelper,
	"dynamic.query_definitions[*].query.logs.filters[*].operator.selected_values":                        filterOperatorHelper,
	"dynamic.query_definitions[*].query.logs.group_by[*].keypath":                                        observationFieldHelper,
	"dynamic.query_definitions[*].query.spans.aggregations[*].observation_field.keypath":                 observationFieldHelper,
	"dynamic.query_definitions[*].query.spans.filters":                                                   filtersHelper,
	"dynamic.query_definitions[*].query.spans.filters[*].operator.selected_values":                       filterOperatorHelper,
	"dynamic.visualization.gauge.category_fields[*].keypath":                                             observationFieldHelper,
	"dynamic.visualization.gauge.legend.columns":                                                         legendHelper,
	"dynamic.visualization.gauge.value_field.keypath":                                                    observationFieldHelper,
	"dynamic.visualization.gauge.value_fields[*].keypath":                                                observationFieldHelper,
	"dynamic.visualization.geomap.aggregation.avg.field.keypath":                                         observationFieldHelper,
	"dynamic.visualization.geomap.aggregation.max.field.keypath":                                         observationFieldHelper,
	"dynamic.visualization.geomap.aggregation.min.field.keypath":                                         observationFieldHelper,
	"dynamic.visualization.geomap.aggregation.sum.field.keypath":                                         observationFieldHelper,
	"dynamic.visualization.geomap.config.aws_region_config.aws_region_field.keypath":                     observationFieldHelper,
	"dynamic.visualization.geomap.config.coordinate_config.latitude_field.keypath":                       observationFieldHelper,
	"dynamic.visualization.geomap.config.coordinate_config.longitude_field.keypath":                      observationFieldHelper,
	"dynamic.visualization.geomap.tooltip.labels[*].keypath":                                             observationFieldHelper,
	"dynamic.visualization.heatmap.tooltip.labels[*].keypath":                                            observationFieldHelper,
	"dynamic.visualization.heatmap.value_field.keypath":                                                  observationFieldHelper,
	"dynamic.visualization.heatmap.x_axis_fields[*].keypath":                                             observationFieldHelper,
	"dynamic.visualization.heatmap.y_axis_fields[*].keypath":                                             observationFieldHelper,
	"dynamic.visualization.hexagon_bins.category_fields[*].keypath":                                      observationFieldHelper,
	"dynamic.visualization.hexagon_bins.legend.columns":                                                  legendHelper,
	"dynamic.visualization.hexagon_bins.value_field.keypath":                                             observationFieldHelper,
	"dynamic.visualization.horizontal_bars.category_fields[*].keypath":                                   observationFieldHelper,
	"dynamic.visualization.horizontal_bars.legend.columns":                                               legendHelper,
	"dynamic.visualization.horizontal_bars.sub_category_fields[*].keypath":                               observationFieldHelper,
	"dynamic.visualization.horizontal_bars.value_field.keypath":                                          observationFieldHelper,
	"dynamic.visualization.horizontal_bars_multi.category_fields[*].keypath":                             observationFieldHelper,
	"dynamic.visualization.horizontal_bars_multi.legend.columns":                                         legendHelper,
	"dynamic.visualization.horizontal_bars_multi.query_field_settings[*].value_field.keypath":            observationFieldHelper,
	"dynamic.visualization.pie_chart.category_fields[*].keypath":                                         observationFieldHelper,
	"dynamic.visualization.pie_chart.legend.columns":                                                     legendHelper,
	"dynamic.visualization.pie_chart.sub_category_fields[*].keypath":                                     observationFieldHelper,
	"dynamic.visualization.pie_chart.value_field.keypath":                                                observationFieldHelper,
	"dynamic.visualization.stat.category_fields[*].keypath":                                              observationFieldHelper,
	"dynamic.visualization.stat.legend.columns":                                                          legendHelper,
	"dynamic.visualization.stat.value_field.keypath":                                                     observationFieldHelper,
	"dynamic.visualization.stat.value_fields[*].keypath":                                                 observationFieldHelper,
	"dynamic.visualization.stat_card.category_fields[*].keypath":                                         observationFieldHelper,
	"dynamic.visualization.stat_card.label.observation_field.keypath":                                    observationFieldHelper,
	"dynamic.visualization.stat_card.label.template_variables[*].observation_field.keypath":              observationFieldHelper,
	"dynamic.visualization.stat_card.legend.columns":                                                     legendHelper,
	"dynamic.visualization.stat_card.primary_value.observation_field.keypath":                            observationFieldHelper,
	"dynamic.visualization.stat_card.primary_value.template_variables[*].observation_field.keypath":      observationFieldHelper,
	"dynamic.visualization.stat_card.title.observation_field.keypath":                                    observationFieldHelper,
	"dynamic.visualization.stat_card.title.template_variables[*].observation_field.keypath":              observationFieldHelper,
	"dynamic.visualization.stat_card.value_fields[*].keypath":                                            observationFieldHelper,
	"dynamic.visualization.table.columns[*].field.keypath":                                               observationFieldHelper,
	"dynamic.visualization.table.rules[*].rule_scope.field.keypath":                                      observationFieldHelper,
	"dynamic.visualization.time_series_bars.category_fields[*].keypath":                                  observationFieldHelper,
	"dynamic.visualization.time_series_bars.legend.columns":                                              legendHelper,
	"dynamic.visualization.time_series_bars.temporal_field.keypath":                                      observationFieldHelper,
	"dynamic.visualization.time_series_bars.value_fields[*].keypath":                                     observationFieldHelper,
	"dynamic.visualization.time_series_lines.category_fields[*].keypath":                                 observationFieldHelper,
	"dynamic.visualization.time_series_lines.legend.columns":                                             legendHelper,
	"dynamic.visualization.time_series_lines.temporal_field.keypath":                                     observationFieldHelper,
	"dynamic.visualization.time_series_lines.value_fields[*].keypath":                                    observationFieldHelper,
	"dynamic.visualization.time_series_lines_multi.legend.columns":                                       legendHelper,
	"dynamic.visualization.time_series_lines_multi.query_display_settings[*].category_fields[*].keypath": observationFieldHelper,
	"dynamic.visualization.time_series_lines_multi.query_display_settings[*].temporal_field.keypath":     observationFieldHelper,
	"dynamic.visualization.time_series_lines_multi.query_display_settings[*].value_fields[*].keypath":    observationFieldHelper,
	"dynamic.visualization.vertical_bars.category_fields[*].keypath":                                     observationFieldHelper,
	"dynamic.visualization.vertical_bars.legend.columns":                                                 legendHelper,
	"dynamic.visualization.vertical_bars.sub_category_fields[*].keypath":                                 observationFieldHelper,
	"dynamic.visualization.vertical_bars.value_field.keypath":                                            observationFieldHelper,
	"dynamic.visualization.vertical_bars_multi.category_fields[*].keypath":                               observationFieldHelper,
	"dynamic.visualization.vertical_bars_multi.legend.columns":                                           legendHelper,
	"dynamic.visualization.vertical_bars_multi.query_field_settings[*].value_field.keypath":              observationFieldHelper,
}

// Deliberately does not consult dynamicListsWhereEmptyRoundTrips: that map
// exempts a list from needing a *minimum*, because empty round-trips for it.
// The documented maximum is a separate question.
func assertListCapped[V interface{ Description(context.Context) string }](ctx context.Context, t *testing.T, path string, validators []V) {
	t.Helper()

	if _, ok := dynamicListsDeclaredBySharedHelpers[path]; ok {
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

func listHasMaximum[V interface{ Description(context.Context) string }](ctx context.Context, validators []V) bool {
	for _, validator := range validators {
		description := validator.Description(ctx)
		if strings.Contains(description, "at most") || strings.Contains(description, "between") {
			return true
		}
	}
	return false
}
