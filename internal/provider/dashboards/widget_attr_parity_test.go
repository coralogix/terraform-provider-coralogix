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
	"fmt"
	"sort"
	"strings"
	"testing"

	dashboardschema "github.com/coralogix/terraform-provider-coralogix/internal/provider/dashboards/dashboard_schema"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestWidgetSchemaMatchesWidgetModelAttr keeps the hand-written widget attr.Type
// map in step with the V4 widget schema. A schema attribute added without the
// matching attr.Type entry only fails at apply time, with a Value Conversion
// Error on "Path: layout"; this assertion turns that into a unit-test failure.
func TestWidgetSchemaMatchesWidgetModelAttr(t *testing.T) {
	widgets := widgetsAttributeFromV4(t)

	want := types.ObjectType{AttrTypes: widgetModelAttr()}
	got, ok := widgets.GetType().(types.ListType)
	if !ok {
		t.Fatalf("widgets attribute type = %T, want types.ListType", widgets.GetType())
	}

	if diff := attrTypeDiff("widget", got.ElemType, want); diff != "" {
		t.Fatalf("V4 widget schema and widgetModelAttr() disagree:\n%s", diff)
	}
}

// widgetsAttributeFromV4 walks the V4 schema down to
// layout.sections[].rows[].widgets and returns that attribute.
func widgetsAttributeFromV4(t *testing.T) schema.Attribute {
	t.Helper()

	attributes := dashboardschema.V4().Attributes
	layout, ok := attributes["layout"].(schema.SingleNestedAttribute)
	if !ok {
		t.Fatalf("layout attribute = %T, want schema.SingleNestedAttribute", attributes["layout"])
	}
	sections, ok := layout.Attributes["sections"].(schema.ListNestedAttribute)
	if !ok {
		t.Fatalf("layout.sections attribute = %T, want schema.ListNestedAttribute", layout.Attributes["sections"])
	}
	rows, ok := sections.NestedObject.Attributes["rows"].(schema.ListNestedAttribute)
	if !ok {
		t.Fatalf("layout.sections.rows attribute = %T, want schema.ListNestedAttribute", sections.NestedObject.Attributes["rows"])
	}
	widgets, ok := rows.NestedObject.Attributes["widgets"]
	if !ok {
		t.Fatal("layout.sections.rows.widgets attribute is missing")
	}
	return widgets
}

// attrTypeDiff reports the first differences between two attr.Type trees as a
// human-readable list of paths, so a failure names the missing attribute
// instead of dumping two whole type trees.
func attrTypeDiff(path string, got, want attr.Type) string {
	if got.Equal(want) {
		return ""
	}

	gotObject, gotOK := got.(types.ObjectType)
	wantObject, wantOK := want.(types.ObjectType)
	if gotOK && wantOK {
		return objectAttrTypeDiff(path, gotObject, wantObject)
	}

	gotList, gotOK := got.(types.ListType)
	wantList, wantOK := want.(types.ListType)
	if gotOK && wantOK {
		return attrTypeDiff(path+"[]", gotList.ElemType, wantList.ElemType)
	}

	gotSet, gotOK := got.(types.SetType)
	wantSet, wantOK := want.(types.SetType)
	if gotOK && wantOK {
		return attrTypeDiff(path+"[]", gotSet.ElemType, wantSet.ElemType)
	}

	return fmt.Sprintf("  %s: schema type %s, attr.Type map %s\n", path, got, want)
}

func objectAttrTypeDiff(path string, got, want types.ObjectType) string {
	var diffs []string
	for _, name := range sortedAttrTypeNames(got.AttrTypes, want.AttrTypes) {
		gotType, inSchema := got.AttrTypes[name]
		wantType, inMap := want.AttrTypes[name]
		switch {
		case !inMap:
			diffs = append(diffs, fmt.Sprintf("  %s.%s: in the schema, missing from the attr.Type map\n", path, name))
		case !inSchema:
			diffs = append(diffs, fmt.Sprintf("  %s.%s: in the attr.Type map, missing from the schema\n", path, name))
		default:
			diffs = append(diffs, attrTypeDiff(path+"."+name, gotType, wantType))
		}
	}
	return strings.Join(diffs, "")
}

func sortedAttrTypeNames(left, right map[string]attr.Type) []string {
	seen := make(map[string]struct{}, len(left)+len(right))
	for name := range left {
		seen[name] = struct{}{}
	}
	for name := range right {
		seen[name] = struct{}{}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
