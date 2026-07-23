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

package dashboard_schema

import (
	"sort"
	"testing"

	dashboardwidgets "github.com/coralogix/terraform-provider-coralogix/internal/provider/dashboards/dashboard_widgets"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// dashboardOneOfContainer is an object-shaped schema node that groups named
// child attributes and can itself carry validator.Object validators: either
// a SingleNestedAttribute, or the NestedObject wrapped by a List/Set nested
// attribute.
type dashboardOneOfContainer struct {
	path       string
	attributes map[string]schema.Attribute
	validators []validator.Object
}

// dashboardObjectValidatorsOf returns the validator.Object list an attribute
// carries on itself, regardless of which nested-attribute kind it is. Only
// the kinds actually used in the V4 schema are handled; anything else
// carries no object validators for this walk's purposes.
func dashboardObjectValidatorsOf(attribute schema.Attribute) []validator.Object {
	switch a := attribute.(type) {
	case schema.SingleNestedAttribute:
		return a.Validators
	case schema.ListNestedAttribute:
		return a.NestedObject.Validators
	case schema.SetNestedAttribute:
		return a.NestedObject.Validators
	default:
		return nil
	}
}

// dashboardWalkOneOfContainers recursively collects every object-shaped
// container reachable from attributes, tagging each with a dot-joined path
// (list/set levels marked with "[]") for readable failure messages.
func dashboardWalkOneOfContainers(prefix string, attributes map[string]schema.Attribute, out *[]dashboardOneOfContainer) {
	for name, attribute := range attributes {
		p := prefix + "." + name
		switch a := attribute.(type) {
		case schema.SingleNestedAttribute:
			*out = append(*out, dashboardOneOfContainer{path: p, attributes: a.Attributes, validators: a.Validators})
			dashboardWalkOneOfContainers(p, a.Attributes, out)
		case schema.ListNestedAttribute:
			*out = append(*out, dashboardOneOfContainer{path: p + "[]", attributes: a.NestedObject.Attributes, validators: a.NestedObject.Validators})
			dashboardWalkOneOfContainers(p+"[]", a.NestedObject.Attributes, out)
		case schema.SetNestedAttribute:
			*out = append(*out, dashboardOneOfContainer{path: p + "[]", attributes: a.NestedObject.Attributes, validators: a.NestedObject.Validators})
			dashboardWalkOneOfContainers(p+"[]", a.NestedObject.Attributes, out)
		}
	}
}

func dashboardSortedKeys(m map[string]schema.Attribute) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func dashboardSortedCopy(s []string) []string {
	out := append([]string(nil), s...)
	sort.Strings(out)
	return out
}

func dashboardEqualStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// dashboardUnknownChildNames returns the entries of childNames that aren't
// keys of attributes. An ExactlyOneOfChildren-family validator's childNames
// don't have to cover every attribute in the container it's attached to —
// some oneof groups sit alongside an unrelated sibling that applies
// regardless of which branch of the group is chosen (e.g. a filter's
// `operator`, next to the mutually exclusive `field`/`observation_field`
// pair) — but every name it does list must resolve to a real attribute, or
// it's silently stale drift from a rename.
func dashboardUnknownChildNames(childNames []string, attributes map[string]schema.Attribute) []string {
	var unknown []string
	for _, name := range childNames {
		if _, ok := attributes[name]; !ok {
			unknown = append(unknown, name)
		}
	}
	return unknown
}

// TestV4WidgetOneOfGroupsAreMigratedToExactlyOneOfChildren walks every
// object-shaped node under the widget "definition" subtree of V4() (the
// per-widget hot path the ExactlyOneOfChildren perf migration targeted) and
// asserts two things generically, without hand-enumerating every group:
//
//  1. No child attribute anywhere in the subtree still carries an old-style,
//     child-attached ExactlyOneOfObject validator. That pattern is exactly
//     the shape of the leftover bugs found in Hexagon's and LineChart's
//     query oneofs: the parent-level ExactlyOneOfChildren was added
//     elsewhere in the file, but one group was missed.
//  2. Wherever a migrated ExactlyOneOfChildren-family validator IS attached
//     to a container, its childNames set is exactly the container's own
//     attribute keys — catching childNames silently drifting from the
//     schema's real attribute map (these are plain strings with no
//     compiler tie back to the map keys), and catching duplicate
//     attachment.
func TestV4WidgetOneOfGroupsAreMigratedToExactlyOneOfChildren(t *testing.T) {
	root := V4()
	layout, ok := root.Attributes["layout"].(schema.SingleNestedAttribute)
	if !ok {
		t.Fatal("layout is not a single nested attribute")
	}

	var containers []dashboardOneOfContainer
	dashboardWalkOneOfContainers("layout", layout.Attributes, &containers)
	if len(containers) == 0 {
		t.Fatal("walked zero containers under layout; the walker is likely broken")
	}

	migratedGroups := dashboardAssertOneOfGroupsMigrated(t, containers)

	// Sanity floor so a future refactor that silently strips every
	// validator from the walked subtree doesn't pass this test by having
	// nothing left to check. As of this test's authoring there are 15
	// migrated groups under layout: the 8-way widget "definition", one
	// 4-way query group per of {gauge, pie_chart, bar_chart,
	// horizontal_bar_chart, data_table, hexagon, line_chart}, bar_chart's
	// xaxis, and several TimeFrameSchema()/FiltersSourceSchema() instances.
	const minMigratedGroups = 15
	if migratedGroups < minMigratedGroups {
		t.Errorf("found %d migrated ExactlyOneOfChildren groups under layout, want at least %d", migratedGroups, minMigratedGroups)
	}
}

// TestV4VariablesOneOfGroupsAreMigratedToExactlyOneOfChildren mirrors
// TestV4WidgetOneOfGroupsAreMigratedToExactlyOneOfChildren for the
// dashboard's top-level "variables" list, a second per-instance subtree
// (variables aren't widgets, but a dashboard can define many of them, so
// their oneof groups pay the same per-instance config-tree-walk cost as the
// widget "definition" subtree if left in the old child-attached style).
func TestV4VariablesOneOfGroupsAreMigratedToExactlyOneOfChildren(t *testing.T) {
	root := V4()
	variables, ok := root.Attributes["variables"].(schema.ListNestedAttribute)
	if !ok {
		t.Fatal("variables is not a list nested attribute")
	}

	var containers []dashboardOneOfContainer
	dashboardWalkOneOfContainers("variables[]", variables.NestedObject.Attributes, &containers)
	if len(containers) == 0 {
		t.Fatal("walked zero containers under variables; the walker is likely broken")
	}

	migratedGroups := dashboardAssertOneOfGroupsMigrated(t, containers)

	// Sanity floor, mirroring the one in the widget test above. As of this
	// test's authoring there are 11 migrated groups under variables:
	// definition (constant_value/multi_select), multi_select.source (5-way),
	// the query logs/metrics/spans selector plus its own logs/metrics/spans
	// field-vs-value groups (4 groups), and 5 stringOrVariableSchema()
	// (string_value/variable_name) instances.
	const minMigratedGroups = 11
	if migratedGroups < minMigratedGroups {
		t.Errorf("found %d migrated ExactlyOneOfChildren groups under variables, want at least %d", migratedGroups, minMigratedGroups)
	}
}

// dashboardAssertOneOfGroupsMigrated asserts, for every container walked
// from a subtree root, that (1) no child attribute still carries an
// old-style child-attached ExactlyOneOfObject validator, and (2) wherever an
// ExactlyOneOfChildren-family validator is attached, its childNames resolve
// to real attribute keys of that same container. It returns the number of
// containers carrying a migrated validator, for callers to sanity-floor.
func dashboardAssertOneOfGroupsMigrated(t *testing.T, containers []dashboardOneOfContainer) int {
	t.Helper()
	migratedGroups := 0
	for _, container := range containers {
		for _, childName := range dashboardSortedKeys(container.attributes) {
			for _, v := range dashboardObjectValidatorsOf(container.attributes[childName]) {
				if fv, ok := v.(dashboardwidgets.FriendlyExactlyOneOfObjectValidator); ok {
					t.Errorf("%s.%s still carries an old-style child-attached ExactlyOneOfObject validator (siblings: %v); "+
						"migrate this oneof group to a single ExactlyOneOfChildren validator attached to %s",
						container.path, childName, fv.PathExpressions, container.path)
				}
			}
		}

		var matches [][]string
		for _, v := range container.validators {
			if ev, ok := v.(dashboardwidgets.ExactlyOneOfChildrenValidator); ok {
				matches = append(matches, ev.ChildNames)
			}
		}
		if len(matches) > 1 {
			t.Errorf("%s has %d ExactlyOneOfChildren-family validators attached, want at most 1", container.path, len(matches))
			continue
		}
		if len(matches) == 1 {
			migratedGroups++
			got := dashboardSortedCopy(matches[0])
			if unknown := dashboardUnknownChildNames(got, container.attributes); len(unknown) > 0 {
				t.Errorf("%s: ExactlyOneOfChildren childNames = %v reference attribute(s) %v not present in this container's attribute keys %v; "+
					"childNames is a plain string list with no compiler tie back to the schema map, so this is likely drift from a rename",
					container.path, got, unknown, dashboardSortedKeys(container.attributes))
			}
		}
	}
	return migratedGroups
}

// dashboardOneOfCase describes one known oneof parent attribute outside the
// widget "definition" subtree, reached by walking SingleNestedAttribute
// and ListNestedAttribute containers by name from V4()'s root attributes.
type dashboardOneOfCase struct {
	name string
	path []string
}

// TestV4TopLevelOneOfGroupsAreWiredToExactlyOneOfChildren checks the
// remaining known oneof parent attributes that sit outside the per-widget
// "layout" subtree covered by TestV4WidgetOneOfGroupsAreMigratedToExactlyOneOfChildren:
// the dashboard's folder reference and the annotation source groups.
func TestV4TopLevelOneOfGroupsAreWiredToExactlyOneOfChildren(t *testing.T) {
	root := V4()

	cases := []dashboardOneOfCase{
		{name: "folder (id/path)", path: []string{"folder"}},
		{name: "annotation source (metrics/logs/spans/manual)", path: []string{"annotations", "source"}},
		{name: "manual annotation strategy (instant/range)", path: []string{"annotations", "source", "manual", "strategy"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			container, ok := dashboardNavigateOneOfContainer(root.Attributes, tc.path)
			if !ok {
				t.Fatalf("could not navigate to %v", tc.path)
			}

			var matches [][]string
			for _, v := range container.validators {
				if ev, ok := v.(dashboardwidgets.ExactlyOneOfChildrenValidator); ok {
					matches = append(matches, ev.ChildNames)
				}
			}
			if len(matches) != 1 {
				t.Fatalf("%s has %d ExactlyOneOfChildren-family validators attached, want exactly 1", container.path, len(matches))
			}
			got := dashboardSortedCopy(matches[0])
			want := dashboardSortedKeys(container.attributes)
			if !dashboardEqualStrings(got, want) {
				t.Fatalf("%s: ExactlyOneOfChildren childNames = %v, want exactly the attribute keys %v", container.path, got, want)
			}
		})
	}
}

// dashboardNavigateOneOfContainer walks a chain of attribute names starting
// from root, stepping transparently through SingleNestedAttribute and
// ListNestedAttribute/SetNestedAttribute containers (list/set levels don't
// consume a path segment of their own — NestedObject.Attributes is used
// directly), and returns the container found at the last segment.
func dashboardNavigateOneOfContainer(root map[string]schema.Attribute, segments []string) (dashboardOneOfContainer, bool) {
	current := dashboardOneOfContainer{path: "root", attributes: root}
	for _, segment := range segments {
		attribute, ok := current.attributes[segment]
		if !ok {
			return dashboardOneOfContainer{}, false
		}
		path := current.path + "." + segment
		switch a := attribute.(type) {
		case schema.SingleNestedAttribute:
			current = dashboardOneOfContainer{path: path, attributes: a.Attributes, validators: a.Validators}
		case schema.ListNestedAttribute:
			current = dashboardOneOfContainer{path: path + "[]", attributes: a.NestedObject.Attributes, validators: a.NestedObject.Validators}
		case schema.SetNestedAttribute:
			current = dashboardOneOfContainer{path: path + "[]", attributes: a.NestedObject.Attributes, validators: a.NestedObject.Validators}
		default:
			return dashboardOneOfContainer{}, false
		}
	}
	return current, true
}
