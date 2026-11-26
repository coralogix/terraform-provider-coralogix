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
	"slices"
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

// webhookArmsOf returns the top-level nested attributes of a webhook schema that have a child
// attribute named child.
func webhookArmsOf(schema *tfprotov6.Schema, child string) []string {
	var arms []string
	for _, a := range schema.Block.Attributes {
		if a.NestedType == nil {
			continue
		}
		for _, c := range a.NestedType.Attributes {
			if c.Name == child {
				arms = append(arms, a.Name)
			}
		}
	}
	slices.Sort(arms)
	return arms
}

// A data source returns the secret in the plain attribute, and returns null in the write-only one.
// The text that it copies from the resource says the opposite, so the data source replaces it.
// The arms come from the schema, so a new arm with a url_wo is checked with no edit of the test.
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
	arms := webhookArmsOf(webhook, "url_wo")
	if len(arms) == 0 {
		t.Fatal("the webhook data source has no arm with url_wo")
	}
	for _, arm := range arms {
		for _, name := range []string{"url_wo", "url_wo_version"} {
			if text := dataSourceAttributeText(t, webhook, arm, name); !strings.HasPrefix(text, "Always null in a data source.") {
				t.Errorf("webhook %s.%s: description = %q", arm, name, text)
			}
		}
		// The advice to use url_wo is for the resource.
		if text := dataSourceAttributeText(t, webhook, arm, "url"); strings.Contains(text, "belongs in") || strings.Contains(text, "exactly one of") {
			t.Errorf("webhook %s.url: description = %q, want no advice to use url_wo", arm, text)
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

// A webhook arm with a url has url_wo too, unless it is listed here with the reason. A new arm with
// a url fails this test until someone gives it url_wo or lists it. The arms with url_wo must also be
// the arms that the resource tests run (webhookArms, and microsoft_teams, which has its own test).
func TestWebhookArmsWithURLAreCovered(t *testing.T) {
	server, err := testAccProtoV6ProviderFactories["coralogix"]()
	if err != nil {
		t.Fatal(err)
	}
	schemas, err := server.GetProviderSchema(context.Background(), &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatal(err)
	}
	webhook := schemas.ResourceSchemas["coralogix_webhook"]
	withoutWriteOnly := map[string]string{
		"jira":                     "the credential of jira is api_token, which has api_token_wo",
		"sendlog":                  "the url is computed, and the API returns it",
		"microsoft_teams_workflow": "not asked for yet",
	}
	for _, arm := range webhookArmsOf(webhook, "url") {
		if _, listed := withoutWriteOnly[arm]; !listed && !slices.Contains(webhookArmsOf(webhook, "url_wo"), arm) {
			t.Errorf("the webhook arm %q has a url and no url_wo. Add url_wo, or list the arm with the reason", arm)
		}
	}
	want := []string{"microsoft_teams"}
	for _, tc := range webhookArms {
		want = append(want, tc.arm)
	}
	slices.Sort(want)
	if got := webhookArmsOf(webhook, "url_wo"); !slices.Equal(got, want) {
		t.Errorf("arms with url_wo = %v, arms of the resource tests = %v. Add the new arm to webhookArms", got, want)
	}
}
