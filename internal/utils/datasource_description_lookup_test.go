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
	"testing"

	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
)

func lookupTestSchema() datasourceschema.Schema {
	return datasourceschema.Schema{Attributes: map[string]datasourceschema.Attribute{
		"plain": datasourceschema.StringAttribute{MarkdownDescription: "plain"},
		"arm": datasourceschema.SingleNestedAttribute{Attributes: map[string]datasourceschema.Attribute{
			"url":    datasourceschema.StringAttribute{MarkdownDescription: "URL. Use `url_wo` instead."},
			"url_wo": datasourceschema.StringAttribute{MarkdownDescription: "secret"},
		}},
		"other": datasourceschema.SingleNestedAttribute{Attributes: map[string]datasourceschema.Attribute{
			"url": datasourceschema.StringAttribute{MarkdownDescription: "URL."},
		}},
	}}
}

func TestDatasourceMarkdownDescription(t *testing.T) {
	s := lookupTestSchema()
	if got, ok := DatasourceMarkdownDescription(s, "arm", "url"); !ok || got != "URL. Use `url_wo` instead." {
		t.Fatalf("nested description = %q, %v", got, ok)
	}
	if got, ok := DatasourceMarkdownDescription(s, "plain"); !ok || got != "plain" {
		t.Fatalf("top-level description = %q, %v", got, ok)
	}
	for _, names := range [][]string{{"missing"}, {"arm", "missing"}, {"plain", "url"}, {}} {
		if _, ok := DatasourceMarkdownDescription(s, names...); ok {
			t.Errorf("%v was reported as found", names)
		}
	}
}

func TestNestedAttributesWith(t *testing.T) {
	got := NestedAttributesWith(lookupTestSchema(), "url_wo")
	if len(got) != 1 || got[0] != "arm" {
		t.Fatalf("NestedAttributesWith = %v, want [arm]", got)
	}
}
