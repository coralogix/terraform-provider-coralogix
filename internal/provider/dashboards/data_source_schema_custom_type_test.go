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

package dashboards

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// The data source derives its schema from the resource's, so a custom type the
// resource declares must survive that conversion. Flatten always produces the
// custom value, so if the data-source schema declares the plain type instead,
// writing state fails with a value-conversion diagnostic and the data source
// cannot return the dashboard at all.
//
// Custom float types are the case that bites: the converter copies CustomType
// for strings but historically dropped it for floats.
func TestDataSourceSchemaPreservesResourceCustomTypes(t *testing.T) {
	ctx := context.Background()

	var r DashboardResource
	var resourceResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &resourceResp)

	var d DashboardDataSource
	var dataSourceResp datasource.SchemaResponse
	d.Schema(ctx, datasource.SchemaRequest{}, &dataSourceResp)

	// layout.sections[*].rows[*].widgets[*].definition.dynamic.visualization
	//   .time_series_lines.y_axis_max
	path := []string{
		"layout", "sections", "rows", "widgets", "definition",
		"dynamic", "visualization", "time_series_lines", "y_axis_max",
	}

	resourceType := resourceAttrTypeAt(t, resourceResp.Schema.Attributes, path)
	dataSourceType := dataSourceAttrTypeAt(t, dataSourceResp.Schema.Attributes, path)

	if resourceType == nil || dataSourceType == nil {
		t.Fatalf("could not resolve %v in both schemas (resource=%v data source=%v)", path, resourceType, dataSourceType)
	}
	if !resourceType.Equal(dataSourceType) {
		t.Errorf("data-source attribute type for %v = %T (%v), want the resource's %T (%v).\n"+
			"The custom type was dropped converting the resource schema, so flattened values will not match the declared schema.",
			path, dataSourceType, dataSourceType, resourceType, resourceType)
	}
}

func resourceAttrTypeAt(t *testing.T, attrs map[string]resourceschema.Attribute, path []string) attr.Type {
	t.Helper()
	var current attr.Type
	for i, name := range path {
		a, ok := attrs[name]
		if !ok {
			return nil
		}
		current = a.GetType()
		if i == len(path)-1 {
			return current
		}
		switch typed := a.(type) {
		case resourceschema.ListNestedAttribute:
			attrs = typed.NestedObject.Attributes
		case resourceschema.SingleNestedAttribute:
			attrs = typed.Attributes
		default:
			return nil
		}
	}
	return current
}

func dataSourceAttrTypeAt(t *testing.T, attrs map[string]datasourceschema.Attribute, path []string) attr.Type {
	t.Helper()
	var current attr.Type
	for i, name := range path {
		a, ok := attrs[name]
		if !ok {
			return nil
		}
		current = a.GetType()
		if i == len(path)-1 {
			return current
		}
		switch typed := a.(type) {
		case datasourceschema.ListNestedAttribute:
			attrs = typed.NestedObject.Attributes
		case datasourceschema.SingleNestedAttribute:
			attrs = typed.Attributes
		default:
			return nil
		}
	}
	return current
}
