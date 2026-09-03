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

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// connectorModel builds the shape ValidateConfig sees: a connector whose field
// list carries the given name/value pairs.
func connectorModel(t *testing.T, id string, fields map[string]string) *ConnectorResourceModel {
	t.Helper()
	ctx := context.Background()

	values := make([]ConnectorConfigFieldModel, 0, len(fields))
	for name, value := range fields {
		values = append(values, ConnectorConfigFieldModel{
			FieldName: types.StringValue(name),
			Value:     types.StringValue(value),
		})
	}
	fieldSet, diags := types.SetValueFrom(ctx, types.ObjectType{AttrTypes: connectorConfigFieldAttrs()}, values)
	if diags.HasError() {
		t.Fatalf("build field set: %v", diags)
	}

	config, diags := types.ObjectValueFrom(ctx, connectorConfigAttr(), ConnectorConfigModel{
		ConnectorConfigFields: fieldSet,
		FieldValuesWO:         types.MapNull(types.StringType),
		FieldValuesWOVersions: types.MapNull(types.Int64Type),
	})
	if diags.HasError() {
		t.Fatalf("build connector config: %v", diags)
	}

	return &ConnectorResourceModel{
		ID:              types.StringValue(id),
		Name:            types.StringValue("a connector"),
		ConnectorConfig: config,
	}
}

func TestConnectorCredentialWarnings(t *testing.T) {
	ctx := context.Background()

	for name, tc := range map[string]struct {
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
		// Most connector fields are ordinary, so warning on all of them would
		// bury the ones that matter.
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
		t.Run(name, func(t *testing.T) {
			got := connectorCredentialWarnings(ctx, connectorModel(t, "an-id", tc.fields))
			if len(got) != tc.want {
				t.Errorf("got %d warnings, want %d: %v", len(got), tc.want, got)
			}
		})
	}
}

// A field supplied write-only is absent from the field list, so there is
// nothing to warn about.
func TestConnectorCredentialWarningsQuietForWriteOnly(t *testing.T) {
	ctx := context.Background()
	config := connectorModel(t, "an-id", map[string]string{"url": "https://example.com"})
	if got := connectorCredentialWarnings(ctx, config); len(got) > 0 {
		t.Errorf("expected no warnings, got %v", got)
	}
}

// Terraform's console renderer collapses diagnostics by summary alone, so two
// connectors with the same problem need different summaries or only one shows.
func TestConnectorWarningSummariesDifferPerConnector(t *testing.T) {
	ctx := context.Background()
	summaryFor := func(id string) string {
		warnings := connectorCredentialWarnings(ctx, connectorModel(t, id, map[string]string{"apiKey": "x"}))
		if len(warnings) != 1 {
			t.Fatalf("expected 1 warning for %q, got %d", id, len(warnings))
		}
		return warnings[0].Summary()
	}
	if first, second := summaryFor("alpha"), summaryFor("beta"); first == second {
		t.Errorf("both connectors produced the summary %q; only one would be shown", first)
	}
}

// The id is generated when not supplied, so it can be unknown at plan time.
func TestConnectorWarningSummaryWithUnknownID(t *testing.T) {
	config := connectorModel(t, "ignored", map[string]string{"apiKey": "x"})
	config.ID = types.StringUnknown()

	summary := connectorWarningSummary(config, "apiKey")
	if strings.Contains(summary, "%!") {
		t.Errorf("summary has a broken format verb: %q", summary)
	}
	// Falls back to the name, which is still better than nothing to go on.
	if !strings.Contains(summary, "a connector") {
		t.Errorf("expected the name in the summary, got %q", summary)
	}
}

// An unknown connector_config cannot be decoded, and validation has nothing to
// say about it either way.
func TestConnectorCredentialWarningsSkipUnknownConfig(t *testing.T) {
	ctx := context.Background()
	config := connectorModel(t, "an-id", map[string]string{"apiKey": "x"})
	config.ConnectorConfig = types.ObjectUnknown(connectorConfigAttr())

	if got := connectorCredentialWarnings(ctx, config); len(got) > 0 {
		t.Errorf("expected no warnings for an unknown config, got %v", got)
	}
}
