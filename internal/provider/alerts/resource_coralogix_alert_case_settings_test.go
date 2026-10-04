// Copyright 2025 Coralogix Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package alerts

import (
	"context"
	"testing"

	alertschema "github.com/coralogix/terraform-provider-coralogix/internal/provider/alerts/alert_schema"
	alerttypes "github.com/coralogix/terraform-provider-coralogix/internal/provider/alerts/alert_types"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"

	alerts "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/alert_definitions_service"
)

var (
	caseEnrichmentQueryObjectType = types.ObjectType{AttrTypes: alertschema.CaseEnrichmentQueryAttr()}
	caseDestinationObjectType     = types.ObjectType{AttrTypes: alertschema.CaseDestinationAttr()}
)

func caseSettingsObject(mode string, queries, destinations []attr.Value) types.Object {
	return types.ObjectValueMust(alertschema.CaseSettingsAttr(), map[string]attr.Value{
		"auto_resolve_mode":  types.StringValue(mode),
		"enrichment_queries": types.ListValueMust(caseEnrichmentQueryObjectType, queries),
		"destinations":       types.ListValueMust(caseDestinationObjectType, destinations),
	})
}

func caseDestinationObject(connectorId, condition string, presetId types.String) attr.Value {
	return types.ObjectValueMust(alertschema.CaseDestinationAttr(), map[string]attr.Value{
		"connector_id": types.StringValue(connectorId),
		"condition":    types.StringValue(condition),
		"preset_id":    presetId,
	})
}

func TestExtractCaseSettings(t *testing.T) {
	ctx := context.Background()

	t.Run("null object is omitted", func(t *testing.T) {
		got, diags := extractCaseSettings(ctx, types.ObjectNull(alertschema.CaseSettingsAttr()))
		if diags.HasError() {
			t.Fatalf("extractCaseSettings returned diagnostics: %v", diags)
		}
		if got != nil {
			t.Fatalf("extractCaseSettings() = %v, want nil", got)
		}
	})

	t.Run("unknown object is omitted", func(t *testing.T) {
		got, diags := extractCaseSettings(ctx, types.ObjectUnknown(alertschema.CaseSettingsAttr()))
		if diags.HasError() {
			t.Fatalf("extractCaseSettings returned diagnostics: %v", diags)
		}
		if got != nil {
			t.Fatalf("extractCaseSettings() = %v, want nil", got)
		}
	})

}

// The API stores an object holding only defaults as absent, so a block with
// defaults must still send a concrete ENABLED mode.
func TestExtractCaseSettingsDefaults(t *testing.T) {
	ctx := context.Background()
	got, diags := extractCaseSettings(ctx, caseSettingsObject(alerttypes.CaseAutoResolveModeEnabled, []attr.Value{}, []attr.Value{}))
	if diags.HasError() {
		t.Fatalf("extractCaseSettings returned diagnostics: %v", diags)
	}
	if got.AutoResolveMode == nil || *got.AutoResolveMode != alerts.ALERTDEFCASEAUTORESOLVEMODE_ALERT_DEF_CASE_AUTO_RESOLVE_MODE_ENABLED {
		t.Errorf("AutoResolveMode = %v, want ENABLED", got.AutoResolveMode)
	}
	if got.EnrichmentQueries == nil || len(got.EnrichmentQueries) != 0 {
		t.Errorf("EnrichmentQueries = %v, want empty non-nil slice", got.EnrichmentQueries)
	}
	if got.Destinations == nil || len(got.Destinations) != 0 {
		t.Errorf("Destinations = %v, want empty non-nil slice", got.Destinations)
	}
}

func TestExtractCaseSettingsValues(t *testing.T) {
	ctx := context.Background()
	obj := caseSettingsObject(alerttypes.CaseAutoResolveModeDisabled,
		[]attr.Value{types.ObjectValueMust(alertschema.CaseEnrichmentQueryAttr(), map[string]attr.Value{
			"query": types.StringValue("source logs | limit 1"),
			"type":  types.StringValue(alerttypes.CaseEnrichmentQueryTypeDataPrime),
		})},
		[]attr.Value{
			caseDestinationObject("connector-b", "true", types.StringNull()),
			caseDestinationObject("connector-a", "caseMetadata.notificationReason == 'caseResolved'", types.StringValue("preset-a")),
		})
	got, diags := extractCaseSettings(ctx, obj)
	if diags.HasError() {
		t.Fatalf("extractCaseSettings returned diagnostics: %v", diags)
	}
	if *got.AutoResolveMode != alerts.ALERTDEFCASEAUTORESOLVEMODE_ALERT_DEF_CASE_AUTO_RESOLVE_MODE_DISABLED {
		t.Errorf("AutoResolveMode = %v, want DISABLED", *got.AutoResolveMode)
	}
	if len(got.EnrichmentQueries) != 1 || got.EnrichmentQueries[0].Query != "source logs | limit 1" ||
		*got.EnrichmentQueries[0].Type != alerts.ALERTDEFCASEENRICHMENTQUERYTYPE_ALERT_DEF_CASE_ENRICHMENT_QUERY_TYPE_DATAPRIME {
		t.Errorf("EnrichmentQueries = %+v, want one DataPrime query", got.EnrichmentQueries)
	}
	if len(got.Destinations) != 2 {
		t.Fatalf("Destinations has %d elements, want 2", len(got.Destinations))
	}
	if got.Destinations[0].ConnectorId != "connector-b" || got.Destinations[0].PresetId != nil {
		t.Errorf("Destinations[0] = %+v, want connector-b without preset", got.Destinations[0])
	}
	if got.Destinations[1].ConnectorId != "connector-a" || got.Destinations[1].PresetId == nil || *got.Destinations[1].PresetId != "preset-a" {
		t.Errorf("Destinations[1] = %+v, want connector-a with preset-a", got.Destinations[1])
	}
}

func TestFlattenCaseSettings(t *testing.T) {
	ctx := context.Background()

	t.Run("nil flattens to null", func(t *testing.T) {
		got, diags := flattenCaseSettings(ctx, nil)
		if diags.HasError() {
			t.Fatalf("flattenCaseSettings returned diagnostics: %v", diags)
		}
		if !got.IsNull() {
			t.Fatalf("flattenCaseSettings(nil) = %v, want null", got)
		}
	})

	// An object created outside Terraform reads back UNSPECIFIED for an
	// omitted mode or query type, and empty lists for omitted lists.
	t.Run("unspecified enums and empty lists flatten to the schema defaults", func(t *testing.T) {
		got, diags := flattenCaseSettings(ctx, &alerts.AlertDefCaseSettings{
			AutoResolveMode: alerts.ALERTDEFCASEAUTORESOLVEMODE_ALERT_DEF_CASE_AUTO_RESOLVE_MODE_UNSPECIFIED.Ptr(),
			EnrichmentQueries: []alerts.AlertDefCaseEnrichmentQuery{{
				Query: "source logs | limit 1",
				Type:  alerts.ALERTDEFCASEENRICHMENTQUERYTYPE_ALERT_DEF_CASE_ENRICHMENT_QUERY_TYPE_UNSPECIFIED.Ptr(),
			}},
			Destinations: []alerts.AlertDefCaseDestination{},
		})
		if diags.HasError() {
			t.Fatalf("flattenCaseSettings returned diagnostics: %v", diags)
		}
		want := caseSettingsObject(alerttypes.CaseAutoResolveModeEnabled,
			[]attr.Value{types.ObjectValueMust(alertschema.CaseEnrichmentQueryAttr(), map[string]attr.Value{
				"query": types.StringValue("source logs | limit 1"),
				"type":  types.StringValue(alerttypes.CaseEnrichmentQueryTypeDataPrime),
			})},
			[]attr.Value{})
		if !got.Equal(want) {
			t.Fatalf("flattenCaseSettings() = %v, want %v", got, want)
		}
	})

	t.Run("nil enums and lists flatten to the schema defaults", func(t *testing.T) {
		got, diags := flattenCaseSettings(ctx, &alerts.AlertDefCaseSettings{})
		if diags.HasError() {
			t.Fatalf("flattenCaseSettings returned diagnostics: %v", diags)
		}
		want := caseSettingsObject(alerttypes.CaseAutoResolveModeEnabled, []attr.Value{}, []attr.Value{})
		if !got.Equal(want) {
			t.Fatalf("flattenCaseSettings() = %v, want %v", got, want)
		}
	})

	t.Run("disabled mode and destinations keep order and preset presence", func(t *testing.T) {
		preset := "preset-a"
		got, diags := flattenCaseSettings(ctx, &alerts.AlertDefCaseSettings{
			AutoResolveMode: alerts.ALERTDEFCASEAUTORESOLVEMODE_ALERT_DEF_CASE_AUTO_RESOLVE_MODE_DISABLED.Ptr(),
			Destinations: []alerts.AlertDefCaseDestination{
				{ConnectorId: "connector-b", Condition: "true"},
				{ConnectorId: "connector-a", Condition: "true", PresetId: &preset},
			},
		})
		if diags.HasError() {
			t.Fatalf("flattenCaseSettings returned diagnostics: %v", diags)
		}
		want := caseSettingsObject(alerttypes.CaseAutoResolveModeDisabled, []attr.Value{}, []attr.Value{
			caseDestinationObject("connector-b", "true", types.StringNull()),
			caseDestinationObject("connector-a", "true", types.StringValue("preset-a")),
		})
		if !got.Equal(want) {
			t.Fatalf("flattenCaseSettings() = %v, want %v", got, want)
		}
	})
}

func TestCaseSettingsRoundTrip(t *testing.T) {
	ctx := context.Background()
	in := caseSettingsObject(alerttypes.CaseAutoResolveModeDisabled,
		[]attr.Value{types.ObjectValueMust(alertschema.CaseEnrichmentQueryAttr(), map[string]attr.Value{
			"query": types.StringValue("source logs | limit 1"),
			"type":  types.StringValue(alerttypes.CaseEnrichmentQueryTypeDataPrime),
		})},
		[]attr.Value{
			caseDestinationObject("connector-b", "true", types.StringNull()),
			caseDestinationObject("connector-a", "true", types.StringValue("preset-a")),
		})
	extracted, diags := extractCaseSettings(ctx, in)
	if diags.HasError() {
		t.Fatalf("extractCaseSettings returned diagnostics: %v", diags)
	}
	out, diags := flattenCaseSettings(ctx, extracted)
	if diags.HasError() {
		t.Fatalf("flattenCaseSettings returned diagnostics: %v", diags)
	}
	if !out.Equal(in) {
		t.Fatalf("round trip = %v, want %v", out, in)
	}
}

// case_settings is a V3-only attribute: prior schema versions must stay
// frozen so stored state from them still decodes.
func TestCaseSettingsSchemaVersions(t *testing.T) {
	if _, ok := alertschema.V1().Attributes["case_settings"]; ok {
		t.Error("V1 schema must not contain case_settings")
	}
	if _, ok := alertschema.V2().Attributes["case_settings"]; ok {
		t.Error("V2 schema must not contain case_settings")
	}
	attribute, ok := alertschema.V3().Attributes["case_settings"]
	if !ok {
		t.Fatal("V3 schema is missing case_settings")
	}
	want := types.ObjectType{AttrTypes: alertschema.CaseSettingsAttr()}
	if got := attribute.GetType(); !got.Equal(want) {
		t.Fatalf("case_settings type = %v, want %v", got, want)
	}
	if attribute.IsRequired() || attribute.IsComputed() {
		t.Error("case_settings must be plain Optional so removing the block clears it")
	}
	if diags := alertschema.V3().ValidateImplementation(context.Background()); diags.HasError() {
		t.Fatalf("V3 schema is invalid: %v", diags)
	}
}
