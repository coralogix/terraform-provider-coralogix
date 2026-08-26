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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coralogix/coralogix-management-sdk/go/openapi/dashboardjson"
	dashboardservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_service"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Whatever the API can store, the typed schema has to be able to express, or
// the dashboard cannot be managed as HCL at all. Two invariants make that
// checkable without knowing anything about a particular visualization:
//
//   - an attribute the schema marks Required must never read back null, and
//   - a one-of object must never read back present with no arm selected, since
//     ExactlyOneOfChildren would then reject the config it produced.
//
// Driving them over the repo's content_json fixtures means a shape somebody
// else adds from a real dashboard tests this schema without anyone remembering
// to. Both invariants were added after each caught a real defect in the table
// visualization; see TestDynamicWidgetTableReadsShapesTheSchemaMustExpress for
// the one-of arms that no fixture happened to cover.
func TestDynamicWidgetReadsFixturesIntoExpressibleHCL(t *testing.T) {
	ctx := context.Background()

	paths, err := filepath.Glob("../../testdata/dashboards/content_json_*.json")
	if err != nil {
		t.Fatalf("globbing fixtures: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("no content_json fixtures found; the guard would silently pass")
	}

	dynamicSchema, ok := DynamicSchema().(schema.SingleNestedAttribute)
	if !ok {
		t.Fatalf("expected the dynamic widget schema to be a single nested attribute, got %T", DynamicSchema())
	}

	checked, refused := 0, 0
	for _, path := range paths {
		name := strings.TrimSuffix(filepath.Base(path), ".json")

		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}

		dashboard := &dashboardservice.Dashboard{}
		if err := dashboardjson.Unmarshal(content, dashboard); err != nil {
			t.Fatalf("decoding %s: %v", name, err)
		}

		for _, widget := range dynamicWidgetsOf(dashboard) {
			flattened, diags := FlattenDynamic(ctx, widget)
			if diags.HasError() {
				// Refusing a shape this version cannot model is deliberate and
				// tested elsewhere. Any other diagnostic is a read regression,
				// and swallowing it here would let this guard pass while the
				// widget it was meant to check went unexamined.
				if !refusesOnlyUnsupportedWidgets(diags) {
					t.Errorf("%s: flattening the dynamic widget: %v", name, diags)
				}
				refused++
				continue
			}

			object, diags := types.ObjectValueFrom(ctx, dynamicModelAttr(), flattened.Dynamic)
			if diags.HasError() {
				t.Errorf("%s: normalizing the flattened widget: %v", name, diags)
				continue
			}

			checked++
			assertExpressible(t, name, dynamicSchema.Attributes, object)
		}
	}

	if checked == 0 {
		t.Fatal("no dynamic widget was checked; the fixtures or the traversal changed")
	}
	t.Logf("checked %d dynamic widget(s) across %d fixture(s), %d deliberately refused", checked, len(paths), refused)
}

// The two deliberate refusals - a deprecated top-level query, and a
// visualization variant this version does not model - both report this summary.
func refusesOnlyUnsupportedWidgets(diags diag.Diagnostics) bool {
	for _, d := range diags.Errors() {
		if d.Summary() != "Unsupported Dashboard Widget Definition" {
			return false
		}
	}

	return true
}

func dynamicWidgetsOf(dashboard *dashboardservice.Dashboard) []*dashboardservice.WidgetsDynamic {
	var found []*dashboardservice.WidgetsDynamic

	for _, section := range dashboard.Layout.Sections {
		for _, row := range section.Rows {
			for _, widget := range row.Widgets {
				if widget.Definition != nil && widget.Definition.Dynamic != nil {
					found = append(found, widget.Definition.Dynamic)
				}
			}
		}
	}

	return found
}

func assertExpressible(t *testing.T, fixture string, attributes map[string]schema.Attribute, object types.Object) {
	t.Helper()

	if object.IsNull() || object.IsUnknown() {
		return
	}

	values := object.Attributes()
	for name, attribute := range attributes {
		value, ok := values[name]
		if !ok {
			continue
		}

		if value.IsNull() && attribute.IsRequired() {
			t.Errorf("%s: %q is Required but reads back null, so no configuration can match it", fixture, name)
			continue
		}

		descend(t, fixture, name, attribute, value)
	}
}

func descend(t *testing.T, fixture, name string, attribute schema.Attribute, value attr.Value) {
	t.Helper()

	switch typed := attribute.(type) {
	case schema.SingleNestedAttribute:
		nested, ok := value.(types.Object)
		if !ok {
			return
		}
		assertNoEmptyOneOf(t, fixture, name, typed, nested)
		assertExpressible(t, fmt.Sprintf("%s > %s", fixture, name), typed.Attributes, nested)

	case schema.ListNestedAttribute:
		list, ok := value.(types.List)
		if !ok || list.IsNull() {
			return
		}
		for index, element := range list.Elements() {
			nested, ok := element.(types.Object)
			if !ok {
				continue
			}
			label := fmt.Sprintf("%s > %s[%d]", fixture, name, index)
			assertExpressible(t, label, typed.NestedObject.Attributes, nested)
		}
	}
}

// A present one-of object with nothing selected is config the validator on that
// same object refuses, so the read direction must have returned null instead.
func assertNoEmptyOneOf(t *testing.T, fixture, name string, attribute schema.SingleNestedAttribute, value types.Object) {
	t.Helper()

	if value.IsNull() || value.IsUnknown() {
		return
	}

	for _, candidate := range attribute.Validators {
		oneOf, ok := candidate.(ExactlyOneOfChildrenValidator)
		if !ok {
			continue
		}

		set := 0
		for _, child := range oneOf.ChildNames {
			if child, ok := value.Attributes()[child]; ok && !child.IsNull() {
				set++
			}
		}

		if set == 0 {
			t.Errorf("%s: %q reads back present with none of %v set, which its own exactly-one-of validator rejects",
				fixture, name, oneOf.ChildNames)
		}
	}
}
