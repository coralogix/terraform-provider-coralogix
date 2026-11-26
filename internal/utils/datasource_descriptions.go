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

package utils

import (
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
)

// DescribeDataSourceAttribute sets the markdown description of the attribute at the path of names:
// a top-level attribute, or an attribute of nested single objects. The data source copies the
// description of the resource, which can be wrong for a data source, for example the text of a
// write-only attribute. It returns false when the schema has no such attribute or the attribute
// has a type that it does not handle.
func DescribeDataSourceAttribute(s datasourceschema.Schema, markdown string, names ...string) bool {
	return describeAttribute(s.Attributes, markdown, names)
}

func describeAttribute(attributes map[string]datasourceschema.Attribute, markdown string, names []string) bool {
	if len(names) == 0 {
		return false
	}
	current, ok := attributes[names[0]]
	if !ok {
		return false
	}
	if len(names) > 1 {
		nested, ok := current.(datasourceschema.SingleNestedAttribute)
		if !ok {
			return false
		}
		return describeAttribute(nested.Attributes, markdown, names[1:])
	}
	switch a := current.(type) {
	case datasourceschema.StringAttribute:
		a.MarkdownDescription = markdown
		attributes[names[0]] = a
	case datasourceschema.Int64Attribute:
		a.MarkdownDescription = markdown
		attributes[names[0]] = a
	case datasourceschema.MapAttribute:
		a.MarkdownDescription = markdown
		attributes[names[0]] = a
	default:
		return false
	}
	return true
}
