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

package notifications

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func connectorFields(t *testing.T, fields map[string]string) types.Set {
	t.Helper()
	values := make([]attr.Value, 0, len(fields))
	for name, value := range fields {
		obj, diags := types.ObjectValue(connectorConfigFieldAttrs(), map[string]attr.Value{
			"field_name": types.StringValue(name),
			"value":      types.StringValue(value),
		})
		if diags.HasError() {
			t.Fatalf("build field: %v", diags)
		}
		values = append(values, obj)
	}
	fieldSet, diags := types.SetValue(types.ObjectType{AttrTypes: connectorConfigFieldAttrs()}, values)
	if diags.HasError() {
		t.Fatalf("build field set: %v", diags)
	}
	return fieldSet
}

func TestConnectorCredentialWarnings(t *testing.T) {
	ctx := context.Background()
	id := types.StringValue("an-id")
	name := types.StringValue("a connector")

	for caseName, tc := range map[string]struct {
		fields map[string]string
		want   int
	}{
		"a secret-looking field warns": {
			fields: map[string]string{"additionalHeaders": `{"Authorization":"x"}`},
			want:   1,
		},
		"casing does not matter": {
			fields: map[string]string{"APIKEY": "x"},
			want:   1,
		},
		"ordinary fields stay quiet": {
			fields: map[string]string{"url": "https://example.com", "method": "POST", "channel": "#alerts"},
		},
		"only the secret-looking one of several": {
			fields: map[string]string{"url": "https://example.com", "integrationKey": "x"},
			want:   1,
		},
		"two secrets warn twice": {
			fields: map[string]string{"apiKey": "x", "password": "y"},
			want:   2,
		},
	} {
		t.Run(caseName, func(t *testing.T) {
			got := connectorCredentialWarnings(ctx, id, name, connectorFields(t, tc.fields))
			if len(got) != tc.want {
				t.Errorf("got %d warnings, want %d: %v", len(got), tc.want, got)
			}
		})
	}
}

func TestConnectorCredentialWarningsQuietForWriteOnly(t *testing.T) {
	ctx := context.Background()
	got := connectorCredentialWarnings(ctx, types.StringValue("an-id"), types.StringValue("a connector"), connectorFields(t, map[string]string{"url": "https://example.com"}))
	if len(got) > 0 {
		t.Errorf("expected no warnings, got %v", got)
	}
}

func TestConnectorWarningSummariesDifferPerConnector(t *testing.T) {
	ctx := context.Background()
	summaryFor := func(id string) string {
		warnings := connectorCredentialWarnings(ctx, types.StringValue(id), types.StringValue("a connector"), connectorFields(t, map[string]string{"apiKey": "x"}))
		if len(warnings) != 1 {
			t.Fatalf("expected 1 warning for %q, got %d", id, len(warnings))
		}
		return warnings[0].Summary()
	}
	if first, second := summaryFor("alpha"), summaryFor("beta"); first == second {
		t.Errorf("both connectors produced the summary %q; only one would be shown", first)
	}
}

func TestConnectorWarningSummaryWithUnknownID(t *testing.T) {
	summary := connectorWarningSummary(types.StringUnknown(), types.StringValue("a connector"), "apiKey")
	if strings.Contains(summary, "%!") {
		t.Errorf("summary has a broken format verb: %q", summary)
	}
	if !strings.Contains(summary, "a connector") {
		t.Errorf("expected the name in the summary, got %q", summary)
	}
}

func TestConnectorCredentialWarningsSkipUnknownConfig(t *testing.T) {
	ctx := context.Background()
	got := connectorCredentialWarnings(ctx, types.StringValue("an-id"), types.StringValue("a connector"), types.SetUnknown(types.ObjectType{AttrTypes: connectorConfigFieldAttrs()}))
	if len(got) > 0 {
		t.Errorf("expected no warnings for an unknown config, got %v", got)
	}
}
