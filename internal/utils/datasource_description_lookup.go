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
	"sort"

	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
)

// These helpers read a data source schema. Use them with OverrideDatasourceDescriptions, which
// writes the descriptions.

// DatasourceMarkdownDescription returns the markdown description of the attribute at the path of
// names: a top-level attribute, or an attribute of nested single objects. It returns false when the
// schema has no such attribute.
func DatasourceMarkdownDescription(s datasourceschema.Schema, names ...string) (string, bool) {
	attributes := s.Attributes
	for i, name := range names {
		current, ok := attributes[name]
		if !ok {
			return "", false
		}
		if i == len(names)-1 {
			return current.GetMarkdownDescription(), true
		}
		nested, ok := current.(datasourceschema.SingleNestedAttribute)
		if !ok {
			return "", false
		}
		attributes = nested.Attributes
	}
	return "", false
}

// NestedAttributesWith returns the names of the top-level single nested attributes that have a
// child attribute named child, in name order.
func NestedAttributesWith(s datasourceschema.Schema, child string) []string {
	var names []string
	for name, attribute := range s.Attributes {
		if nested, ok := attribute.(datasourceschema.SingleNestedAttribute); ok {
			if _, has := nested.Attributes[child]; has {
				names = append(names, name)
			}
		}
	}
	sort.Strings(names)
	return names
}
