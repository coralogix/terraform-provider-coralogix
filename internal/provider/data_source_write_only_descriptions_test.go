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

package provider

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// dataSourceAttributeText returns the description of the attribute at a path of names.
func dataSourceAttributeText(t *testing.T, schema *tfprotov6.Schema, names ...string) string {
	t.Helper()
	attrs := schema.Block.Attributes
	for i, name := range names {
		var found *tfprotov6.SchemaAttribute
		for _, a := range attrs {
			if a.Name == name {
				found = a
			}
		}
		if found == nil {
			t.Fatalf("the data source has no attribute %v", names[:i+1])
		}
		if i == len(names)-1 {
			return found.Description
		}
		if found.NestedType == nil {
			t.Fatalf("%v is not a nested attribute", names[:i+1])
		}
		attrs = found.NestedType.Attributes
	}
	return ""
}

// A data source returns the secret in the plain attribute, and returns null in the write-only one.
// The text that it copies from the resource says the opposite, so the data source replaces it.
func TestDataSourceDescribesWriteOnlyAttributesAsNull(t *testing.T) {
	server, err := testAccProtoV6ProviderFactories["coralogix"]()
	if err != nil {
		t.Fatal(err)
	}
	schemas, err := server.GetProviderSchema(context.Background(), &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatal(err)
	}
	webhook := schemas.DataSourceSchemas["coralogix_webhook"]
	cases := map[string][]string{}
	for _, arm := range []string{"custom", "slack", "microsoft_teams", "opsgenie", "demisto"} {
		cases["webhook "+arm+".url_wo"] = []string{arm, "url_wo"}
		cases["webhook "+arm+".url_wo_version"] = []string{arm, "url_wo_version"}
	}
	for name, path := range cases {
		if text := dataSourceAttributeText(t, webhook, path...); !strings.HasPrefix(text, "Always null in a data source.") {
			t.Errorf("%s: description = %q", name, text)
		}
	}
	for source, names := range map[string][]string{
		"coralogix_integration": {"parameters_wo", "parameters_wo_versions"},
		"coralogix_connector":   {"config_overrides_templates_wo", "config_overrides_templates_wo_versions"},
	} {
		for _, name := range names {
			if text := dataSourceAttributeText(t, schemas.DataSourceSchemas[source], name); !strings.HasPrefix(text, "Always null in a data source.") {
				t.Errorf("%s.%s: description = %q", source, name, text)
			}
		}
	}
}
