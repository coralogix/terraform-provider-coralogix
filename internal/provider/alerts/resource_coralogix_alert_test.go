// Copyright 2024 Coralogix Ltd.
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
	"math/big"
	"testing"

	alertschema "github.com/coralogix/terraform-provider-coralogix/internal/provider/alerts/alert_schema"
	alerttypes "github.com/coralogix/terraform-provider-coralogix/internal/provider/alerts/alert_types"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"

	alerts "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/alert_definitions_service"
)

// TestFlattenTracingSimpleFilter_LatencyExact ensures the latency_threshold_ms
// returned by the API is preserved exactly through the flatten path. Prior to
// the fix, this used big.ParseFloat with prec=10 which silently rounded values
// outside the [1,1024] significand range — e.g. 50000 → 49984, breaking
// post-apply consistency on v2→v3 migrations.
func TestFlattenTracingSimpleFilter_LatencyExact(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		expected int64
	}{
		{"low value (exact at all precisions)", "1", 1},
		{"3-digit (exact at prec=10)", "100", 100},
		{"30000 — was rounded UP to 30016 by prec=10 ToNearestAway", "30000", 30000},
		{"50000 — was rounded DOWN to 49984 by prec=10", "50000", 50000},
		{"100000 — was rounded by prec=10", "100000", 100000},
		{"max int32-ish", "2147483647", 2147483647},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			filter := &alerts.TracingSimpleFilter{
				LatencyThresholdMs: &tc.input,
			}
			got, diags := flattenTracingSimpleFilter(context.Background(), filter)
			if diags.HasError() {
				t.Fatalf("flattenTracingSimpleFilter returned diagnostics: %v", diags)
			}

			var model alerttypes.TracingFilterModel
			if diags := got.As(context.Background(), &model, basetypes.ObjectAsOptions{}); diags.HasError() {
				t.Fatalf("As() returned diagnostics: %v", diags)
			}

			f := model.LatencyThresholdMs.ValueBigFloat()
			if f == nil {
				t.Fatalf("LatencyThresholdMs is nil")
			}
			i, acc := f.Int64()
			if acc != big.Exact {
				t.Fatalf("LatencyThresholdMs %v is not an exact int64 (accuracy=%v); precision was insufficient", f, acc)
			}
			if i != tc.expected {
				t.Fatalf("LatencyThresholdMs = %d, want %d (raw big.Float: %v)", i, tc.expected, f)
			}
		})
	}
}

func TestFlattenTracingSimpleFilter_InvalidLatency(t *testing.T) {
	bad := "not-a-number"
	filter := &alerts.TracingSimpleFilter{
		LatencyThresholdMs: &bad,
	}
	_, diags := flattenTracingSimpleFilter(context.Background(), filter)
	if !diags.HasError() {
		t.Fatalf("expected error for non-numeric latency, got nil diagnostics")
	}
	found := false
	for _, d := range diags.Errors() {
		if d.Summary() == "Invalid Latency Threshold Ms" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected diagnostic summary 'Invalid Latency Threshold Ms', got: %v", diags.Errors())
	}
}

func TestFlattenAlertLabels(t *testing.T) {
	ctx := context.Background()
	base := func(labels map[string]string) *alerts.AlertDefProperties {
		return &alerts.AlertDefProperties{
			LogsImmediate: &alerts.LogsImmediateType{},
			EntityLabels:  labels,
		}
	}

	t.Run("nil map is null", func(t *testing.T) {
		got, diags := flattenAlertLabels(ctx, base(nil))
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics: %v", diags)
		}
		if !got.IsNull() {
			t.Fatalf("got %v, want null", got)
		}
	})

	t.Run("empty map is null", func(t *testing.T) {
		got, diags := flattenAlertLabels(ctx, base(map[string]string{}))
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics: %v", diags)
		}
		if !got.IsNull() {
			t.Fatalf("got %v, want null", got)
		}
	})

	t.Run("empty map is rejected by schema", func(t *testing.T) {
		labelsAttr, ok := alertschema.V3().Attributes["labels"].(schema.MapAttribute)
		if !ok {
			t.Fatal("labels is not a MapAttribute")
		}
		empty, diags := types.MapValue(types.StringType, map[string]attr.Value{})
		if diags.HasError() {
			t.Fatalf("MapValue: %v", diags)
		}
		req := validator.MapRequest{ConfigValue: empty}
		resp := validator.MapResponse{}
		for _, v := range labelsAttr.Validators {
			v.ValidateMap(ctx, req, &resp)
		}
		if !resp.Diagnostics.HasError() {
			t.Fatal("expected labels = {} to be rejected")
		}
	})

	t.Run("populated map is kept", func(t *testing.T) {
		got, diags := flattenAlertLabels(ctx, base(map[string]string{"team": "payments"}))
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics: %v", diags)
		}
		if got.IsNull() {
			t.Fatal("got null, want populated map")
		}
		var m map[string]string
		if diags := got.ElementsAs(ctx, &m, false); diags.HasError() {
			t.Fatalf("ElementsAs: %v", diags)
		}
		if m["team"] != "payments" {
			t.Fatalf("got %#v, want team=payments", m)
		}
	})
}

func TestExtractCustomEvaluationDelay(t *testing.T) {
	cases := []struct {
		name string
		in   types.Int32
		want *int32
	}{
		{
			name: "null is omitted",
			in:   types.Int32Null(),
			want: nil,
		},
		{
			name: "unknown is omitted",
			in:   types.Int32Unknown(),
			want: nil,
		},
		{
			name: "explicit zero is preserved",
			in:   types.Int32Value(0),
			want: int32Ptr(0),
		},
		{
			name: "explicit non-zero is preserved",
			in:   types.Int32Value(60),
			want: int32Ptr(60),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractCustomEvaluationDelay(tc.in)
			if tc.want == nil {
				if got != nil {
					t.Fatalf("extractCustomEvaluationDelay() = %v, want nil", *got)
				}
				return
			}

			if got == nil {
				t.Fatalf("extractCustomEvaluationDelay() = nil, want %v", *tc.want)
			}
			if *got != *tc.want {
				t.Fatalf("extractCustomEvaluationDelay() = %v, want %v", *got, *tc.want)
			}
		})
	}
}

func int32Ptr(v int32) *int32 {
	return &v
}

func TestExtractDataSources(t *testing.T) {
	ctx := context.Background()
	dataSourceObjectType := types.ObjectType{AttrTypes: alertschema.DataSourcesAttr()}

	t.Run("null list is omitted", func(t *testing.T) {
		got, diags := extractDataSources(ctx, types.ListNull(dataSourceObjectType))
		if diags.HasError() {
			t.Fatalf("extractDataSources returned diagnostics: %v", diags)
		}
		if got != nil {
			t.Fatalf("extractDataSources() = %v, want nil", got)
		}
	})

	t.Run("unknown list is omitted", func(t *testing.T) {
		got, diags := extractDataSources(ctx, types.ListUnknown(dataSourceObjectType))
		if diags.HasError() {
			t.Fatalf("extractDataSources returned diagnostics: %v", diags)
		}
		if got != nil {
			t.Fatalf("extractDataSources() = %v, want nil", got)
		}
	})

	t.Run("values are extracted", func(t *testing.T) {
		dataSources := types.ListValueMust(dataSourceObjectType, []attr.Value{
			types.ObjectValueMust(alertschema.DataSourcesAttr(), map[string]attr.Value{
				"data_space": types.StringValue("default"),
				"data_set":   types.StringValue("my-dataset"),
			}),
		})
		got, diags := extractDataSources(ctx, dataSources)
		if diags.HasError() {
			t.Fatalf("extractDataSources returned diagnostics: %v", diags)
		}
		if len(got) != 1 {
			t.Fatalf("extractDataSources() returned %d elements, want 1", len(got))
		}
		if got[0].DataSpace == nil || *got[0].DataSpace != "default" {
			t.Errorf("DataSpace = %v, want \"default\"", got[0].DataSpace)
		}
		if got[0].DataSet == nil || *got[0].DataSet != "my-dataset" {
			t.Errorf("DataSet = %v, want \"my-dataset\"", got[0].DataSet)
		}
	})
}

func TestFlattenDataSources(t *testing.T) {
	ctx := context.Background()

	t.Run("nil slice flattens to null list", func(t *testing.T) {
		got, diags := flattenDataSources(ctx, nil)
		if diags.HasError() {
			t.Fatalf("flattenDataSources returned diagnostics: %v", diags)
		}
		if !got.IsNull() {
			t.Fatalf("flattenDataSources(nil) = %v, want null list", got)
		}
	})

	t.Run("empty slice flattens to null list", func(t *testing.T) {
		got, diags := flattenDataSources(ctx, []alerts.AlertDefDataSource{})
		if diags.HasError() {
			t.Fatalf("flattenDataSources returned diagnostics: %v", diags)
		}
		if !got.IsNull() {
			t.Fatalf("flattenDataSources([]) = %v, want null list", got)
		}
	})

	t.Run("values round-trip", func(t *testing.T) {
		dataSpace, dataSet := "default", "my-dataset"
		got, diags := flattenDataSources(ctx, []alerts.AlertDefDataSource{
			{DataSpace: &dataSpace, DataSet: &dataSet},
		})
		if diags.HasError() {
			t.Fatalf("flattenDataSources returned diagnostics: %v", diags)
		}
		var models []alerttypes.DataSourceModel
		if diags := got.ElementsAs(ctx, &models, false); diags.HasError() {
			t.Fatalf("ElementsAs returned diagnostics: %v", diags)
		}
		if len(models) != 1 {
			t.Fatalf("flattened list has %d elements, want 1", len(models))
		}
		if models[0].DataSpace.ValueString() != "default" {
			t.Errorf("data_space = %q, want \"default\"", models[0].DataSpace.ValueString())
		}
		if models[0].DataSet.ValueString() != "my-dataset" {
			t.Errorf("data_set = %q, want \"my-dataset\"", models[0].DataSet.ValueString())
		}
	})
}

func destinationObject(retriggeringPeriodMinutes types.Int64) attr.Value {
	routingOverridesType := types.ObjectType{AttrTypes: alertschema.RoutingOverridesV3Attr()}
	return types.ObjectValueMust(alertschema.NotificationDestinationsV3Attr(), map[string]attr.Value{
		"connector_id":                types.StringValue("connector-id"),
		"preset_id":                   types.StringValue("preset-id"),
		"notify_on":                   types.StringValue("Triggered Only"),
		"triggered_routing_overrides": types.ObjectNull(routingOverridesType.AttrTypes),
		"resolved_routing_overrides":  types.ObjectNull(routingOverridesType.AttrTypes),
		"retriggering_period_minutes": retriggeringPeriodMinutes,
	})
}

func TestExtractDestinationsRetriggeringPeriodMinutes(t *testing.T) {
	ctx := context.Background()
	destinationObjectType := types.ObjectType{AttrTypes: alertschema.NotificationDestinationsV3Attr()}

	t.Run("null is omitted", func(t *testing.T) {
		destinations := types.ListValueMust(destinationObjectType, []attr.Value{destinationObject(types.Int64Null())})
		got, diags := extractDestinations(ctx, destinations)
		if diags.HasError() {
			t.Fatalf("extractDestinations returned diagnostics: %v", diags)
		}
		if len(got) != 1 {
			t.Fatalf("extractDestinations() returned %d destinations, want 1", len(got))
		}
		if got[0].RetriggeringPeriodMinutes != nil {
			t.Fatalf("RetriggeringPeriodMinutes = %v, want nil", *got[0].RetriggeringPeriodMinutes)
		}
	})

	t.Run("value is preserved", func(t *testing.T) {
		destinations := types.ListValueMust(destinationObjectType, []attr.Value{destinationObject(types.Int64Value(600))})
		got, diags := extractDestinations(ctx, destinations)
		if diags.HasError() {
			t.Fatalf("extractDestinations returned diagnostics: %v", diags)
		}
		if len(got) != 1 {
			t.Fatalf("extractDestinations() returned %d destinations, want 1", len(got))
		}
		if got[0].RetriggeringPeriodMinutes == nil || *got[0].RetriggeringPeriodMinutes != 600 {
			t.Fatalf("RetriggeringPeriodMinutes = %v, want 600", got[0].RetriggeringPeriodMinutes)
		}
	})
}

func TestFlattenNotificationDestinationsRetriggeringPeriodMinutes(t *testing.T) {
	ctx := context.Background()
	connectorId, presetId := "connector-id", "preset-id"
	retriggeringPeriodMinutes := int64(600)

	cases := []struct {
		name string
		in   *int64
		want types.Int64
	}{
		{"nil flattens to null", nil, types.Int64Null()},
		{"value flattens to value", &retriggeringPeriodMinutes, types.Int64Value(600)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, diags := flattenNotificationDestinations(ctx, []alerts.NotificationDestination{
				{
					ConnectorId:               &connectorId,
					PresetId:                  &presetId,
					RetriggeringPeriodMinutes: tc.in,
				},
			})
			if diags.HasError() {
				t.Fatalf("flattenNotificationDestinations returned diagnostics: %v", diags)
			}
			var models []alerttypes.NotificationDestinationModel
			if diags := got.ElementsAs(ctx, &models, false); diags.HasError() {
				t.Fatalf("ElementsAs returned diagnostics: %v", diags)
			}
			if len(models) != 1 {
				t.Fatalf("flattened list has %d elements, want 1", len(models))
			}
			if !models[0].RetriggeringPeriodMinutes.Equal(tc.want) {
				t.Fatalf("retriggering_period_minutes = %v, want %v", models[0].RetriggeringPeriodMinutes, tc.want)
			}
		})
	}
}

func TestPreserveDestinationRetriggeringNulls(t *testing.T) {
	ctx := context.Background()
	groupObject := func(retriggeringPeriodMinutes types.Int64) types.Object {
		return types.ObjectValueMust(alertschema.NotificationGroupV3Attr(), map[string]attr.Value{
			"group_by_keys":     types.ListNull(types.StringType),
			"webhooks_settings": types.SetNull(types.ObjectType{AttrTypes: alertschema.WebhooksSettingsAttr()}),
			"destinations": types.ListValueMust(
				types.ObjectType{AttrTypes: alertschema.NotificationDestinationsV3Attr()},
				[]attr.Value{destinationObject(retriggeringPeriodMinutes)},
			),
			"router": types.ObjectNull(alertschema.NotificationRouterAttr()),
		})
	}

	t.Run("unconfigured value is kept null despite backend echo", func(t *testing.T) {
		current := groupObject(types.Int64Null())
		flattened := groupObject(types.Int64Value(10))
		got, diags := preserveDestinationRetriggeringNulls(ctx, &current, flattened)
		if diags.HasError() {
			t.Fatalf("preserveDestinationRetriggeringNulls returned diagnostics: %v", diags)
		}
		var model alerttypes.NotificationGroupModel
		if diags := got.As(ctx, &model, basetypes.ObjectAsOptions{}); diags.HasError() {
			t.Fatalf("As() returned diagnostics: %v", diags)
		}
		var destinations []alerttypes.NotificationDestinationModel
		if diags := model.Destinations.ElementsAs(ctx, &destinations, false); diags.HasError() {
			t.Fatalf("ElementsAs returned diagnostics: %v", diags)
		}
		if !destinations[0].RetriggeringPeriodMinutes.IsNull() {
			t.Fatalf("retriggering_period_minutes = %v, want null", destinations[0].RetriggeringPeriodMinutes)
		}
	})

	t.Run("configured value is preserved", func(t *testing.T) {
		current := groupObject(types.Int64Value(600))
		flattened := groupObject(types.Int64Value(600))
		got, diags := preserveDestinationRetriggeringNulls(ctx, &current, flattened)
		if diags.HasError() {
			t.Fatalf("preserveDestinationRetriggeringNulls returned diagnostics: %v", diags)
		}
		var model alerttypes.NotificationGroupModel
		if diags := got.As(ctx, &model, basetypes.ObjectAsOptions{}); diags.HasError() {
			t.Fatalf("As() returned diagnostics: %v", diags)
		}
		var destinations []alerttypes.NotificationDestinationModel
		if diags := model.Destinations.ElementsAs(ctx, &destinations, false); diags.HasError() {
			t.Fatalf("ElementsAs returned diagnostics: %v", diags)
		}
		if destinations[0].RetriggeringPeriodMinutes.ValueInt64() != 600 {
			t.Fatalf("retriggering_period_minutes = %v, want 600", destinations[0].RetriggeringPeriodMinutes)
		}
	})

	t.Run("nil current passes flattened through", func(t *testing.T) {
		flattened := groupObject(types.Int64Value(10))
		got, diags := preserveDestinationRetriggeringNulls(ctx, nil, flattened)
		if diags.HasError() {
			t.Fatalf("preserveDestinationRetriggeringNulls returned diagnostics: %v", diags)
		}
		if !got.Equal(flattened) {
			t.Fatalf("expected flattened object to pass through unchanged")
		}
	})

	t.Run("state upgrade clears every echoed value", func(t *testing.T) {
		flattened := groupObject(types.Int64Value(10))
		got, diags := clearDestinationRetriggering(ctx, flattened)
		if diags.HasError() {
			t.Fatalf("clearDestinationRetriggering returned diagnostics: %v", diags)
		}
		var model alerttypes.NotificationGroupModel
		if diags := got.As(ctx, &model, basetypes.ObjectAsOptions{}); diags.HasError() {
			t.Fatalf("As() returned diagnostics: %v", diags)
		}
		var destinations []alerttypes.NotificationDestinationModel
		if diags := model.Destinations.ElementsAs(ctx, &destinations, false); diags.HasError() {
			t.Fatalf("ElementsAs returned diagnostics: %v", diags)
		}
		if !destinations[0].RetriggeringPeriodMinutes.IsNull() {
			t.Fatalf("retriggering_period_minutes = %v, want null", destinations[0].RetriggeringPeriodMinutes)
		}
	})
}

func TestFlattenLogsRatioThresholdUndetectedValuesManagement(t *testing.T) {
	ctx := context.Background()
	trigger := true
	autoRetireTimeframe := alerts.V3AUTORETIRETIMEFRAME_AUTO_RETIRE_TIMEFRAME_HOUR_1

	got, diags := flattenLogsRatioThreshold(ctx, &alerts.LogsRatioThresholdType{
		UndetectedValuesManagement: &alerts.V3UndetectedValuesManagement{
			TriggerUndetectedValues: &trigger,
			AutoRetireTimeframe:     &autoRetireTimeframe,
		},
	})
	if diags.HasError() {
		t.Fatalf("flattenLogsRatioThreshold returned diagnostics: %v", diags)
	}

	var model alerttypes.LogsRatioThresholdModel
	if diags := got.As(ctx, &model, basetypes.ObjectAsOptions{}); diags.HasError() {
		t.Fatalf("As() returned diagnostics: %v", diags)
	}
	var undetectedValuesManagement alerttypes.UndetectedValuesManagementModel
	if diags := model.UndetectedValuesManagement.As(ctx, &undetectedValuesManagement, basetypes.ObjectAsOptions{}); diags.HasError() {
		t.Fatalf("As() returned diagnostics: %v", diags)
	}
	if !undetectedValuesManagement.TriggerUndetectedValues.ValueBool() {
		t.Errorf("trigger_undetected_values = false, want true")
	}
	if undetectedValuesManagement.AutoRetireTimeframe.ValueString() != alerttypes.AutoRetireTimeframeProtoToSchemaMap[alerts.V3AUTORETIRETIMEFRAME_AUTO_RETIRE_TIMEFRAME_HOUR_1] {
		t.Errorf("auto_retire_timeframe = %q, want %q", undetectedValuesManagement.AutoRetireTimeframe.ValueString(), alerttypes.AutoRetireTimeframeProtoToSchemaMap[alerts.V3AUTORETIRETIMEFRAME_AUTO_RETIRE_TIMEFRAME_HOUR_1])
	}
}

func analyticsAlertResourceModel() alerttypes.AlertResourceModel {
	return alerttypes.AlertResourceModel{
		Name:              types.StringValue("analytics alert"),
		Priority:          types.StringValue("P3"),
		GroupBy:           types.ListNull(types.StringType),
		IncidentsSettings: types.ObjectNull(alertschema.IncidentsSettingsAttr()),
		NotificationGroup: types.ObjectNull(alertschema.NotificationGroupV3Attr()),
		Labels:            types.MapNull(types.StringType),
		Schedule:          types.ObjectNull(alertschema.AlertScheduleAttr()),
	}
}

func dataprimeQueryObject(query string) types.Object {
	return types.ObjectValueMust(alertschema.DataprimeQueryAttr(), map[string]attr.Value{
		"query": types.StringValue(query),
	})
}

func analyticsThresholdRuleObject(threshold float64, priority string) attr.Value {
	return types.ObjectValueMust(alertschema.AnalyticsThresholdRuleAttr(), map[string]attr.Value{
		"condition": types.ObjectValueMust(alertschema.AnalyticsThresholdConditionAttr(), map[string]attr.Value{
			"threshold": types.Float64Value(threshold),
		}),
		"override": types.ObjectValueMust(alertschema.AlertOverrideAttr(), map[string]attr.Value{
			"priority": types.StringValue(priority),
		}),
	})
}

// TestExpandAnalyticsImmediate covers the probed round-trip values for the
// analytics_immediate arm, plus the minimal config in which every optional leaf
// must stay absent from the request.
func TestExpandAnalyticsImmediate(t *testing.T) {
	ctx := context.Background()

	t.Run("all probed values are sent", func(t *testing.T) {
		object := types.ObjectValueMust(alertschema.AnalyticsImmediateAttr(), map[string]attr.Value{
			"dataprime_query": dataprimeQueryObject("source logs | filter severity == 'error' | count"),
			"no_data_policy": types.ObjectValueMust(alertschema.NoDataPolicyAttr(), map[string]attr.Value{
				"auto_retire_seconds": types.Int64Value(3600),
				"state":               types.StringValue("ALERTING"),
			}),
			"use_rows_as_permutations": types.BoolValue(true),
			"timeframe_minutes":        types.Int32Value(45),
			"custom_evaluation_delay":  types.Int32Value(120000),
		})

		got, diags := expandAnalyticsImmediateTypeDefinition(ctx, &alerts.AlertDefProperties{}, object, analyticsAlertResourceModel())
		if diags.HasError() {
			t.Fatalf("expandAnalyticsImmediateTypeDefinition returned diagnostics: %v", diags)
		}
		if got.Type == nil || *got.Type != alerts.ALERTDEFTYPE_ALERT_DEF_TYPE_ANALYTICS_IMMEDIATE {
			t.Fatalf("Type = %v, want ANALYTICS_IMMEDIATE", got.Type)
		}
		immediate := got.AnalyticsImmediate
		if immediate == nil {
			t.Fatal("AnalyticsImmediate = nil, want value")
		}
		if immediate.DataprimeQuery == nil || immediate.DataprimeQuery.GetQuery() != "source logs | filter severity == 'error' | count" {
			t.Errorf("DataprimeQuery = %v, want the configured query", immediate.DataprimeQuery)
		}
		if immediate.UseRowsAsPermutations == nil || !*immediate.UseRowsAsPermutations {
			t.Errorf("UseRowsAsPermutations = %v, want true", immediate.UseRowsAsPermutations)
		}
		if immediate.TimeframeMinutes == nil || *immediate.TimeframeMinutes != 45 {
			t.Errorf("TimeframeMinutes = %v, want 45", immediate.TimeframeMinutes)
		}
		if immediate.EvaluationDelayMs == nil || *immediate.EvaluationDelayMs != 120000 {
			t.Errorf("EvaluationDelayMs = %v, want 120000", immediate.EvaluationDelayMs)
		}
		if immediate.NoDataPolicy == nil || immediate.NoDataPolicy.State == nil ||
			*immediate.NoDataPolicy.State != alerts.NODATAPOLICYSTATE_NO_DATA_POLICY_STATE_ALERTING {
			t.Errorf("NoDataPolicy.State = %v, want ALERTING", immediate.NoDataPolicy)
		}
		if immediate.NoDataPolicy.AutoRetireSeconds == nil || *immediate.NoDataPolicy.AutoRetireSeconds != 3600 {
			t.Errorf("NoDataPolicy.AutoRetireSeconds = %v, want 3600", immediate.NoDataPolicy.AutoRetireSeconds)
		}
	})

	t.Run("omitted optionals are not sent", func(t *testing.T) {
		object := types.ObjectValueMust(alertschema.AnalyticsImmediateAttr(), map[string]attr.Value{
			"dataprime_query":          dataprimeQueryObject("source logs | count"),
			"no_data_policy":           types.ObjectNull(alertschema.NoDataPolicyAttr()),
			"use_rows_as_permutations": types.BoolNull(),
			"timeframe_minutes":        types.Int32Null(),
			"custom_evaluation_delay":  types.Int32Null(),
		})

		got, diags := expandAnalyticsImmediateTypeDefinition(ctx, &alerts.AlertDefProperties{}, object, analyticsAlertResourceModel())
		if diags.HasError() {
			t.Fatalf("expandAnalyticsImmediateTypeDefinition returned diagnostics: %v", diags)
		}
		immediate := got.AnalyticsImmediate
		if immediate.NoDataPolicy != nil {
			t.Errorf("NoDataPolicy = %v, want nil", immediate.NoDataPolicy)
		}
		if immediate.UseRowsAsPermutations != nil {
			t.Errorf("UseRowsAsPermutations = %v, want nil", *immediate.UseRowsAsPermutations)
		}
		if immediate.TimeframeMinutes != nil {
			t.Errorf("TimeframeMinutes = %v, want nil", *immediate.TimeframeMinutes)
		}
		if immediate.EvaluationDelayMs != nil {
			t.Errorf("EvaluationDelayMs = %v, want nil", *immediate.EvaluationDelayMs)
		}
	})
}

// TestFlattenAnalyticsImmediate asserts that a field the API never returns comes
// back null rather than as a zero value, which is what keeps plain Optional from
// producing a perpetual diff.
func TestFlattenAnalyticsImmediate(t *testing.T) {
	ctx := context.Background()
	query, delay, timeframe, permutations, retire := "source logs | count", int32(120000), int32(45), true, int32(3600)
	state := alerts.NODATAPOLICYSTATE_NO_DATA_POLICY_STATE_ALERTING

	t.Run("minimal read stays null", func(t *testing.T) {
		got, diags := flattenAnalyticsImmediate(ctx, &alerts.AnalyticsImmediateType{
			DataprimeQuery: &alerts.DataprimeAlertQuery{Query: &query},
		})
		if diags.HasError() {
			t.Fatalf("flattenAnalyticsImmediate returned diagnostics: %v", diags)
		}
		var model alerttypes.AnalyticsImmediateModel
		if diags := got.As(ctx, &model, basetypes.ObjectAsOptions{}); diags.HasError() {
			t.Fatalf("As() returned diagnostics: %v", diags)
		}
		if !model.NoDataPolicy.IsNull() {
			t.Errorf("no_data_policy = %v, want null", model.NoDataPolicy)
		}
		if !model.UseRowsAsPermutations.IsNull() {
			t.Errorf("use_rows_as_permutations = %v, want null", model.UseRowsAsPermutations)
		}
		if !model.TimeframeMinutes.IsNull() {
			t.Errorf("timeframe_minutes = %v, want null", model.TimeframeMinutes)
		}
		if !model.CustomEvaluationDelay.IsNull() {
			t.Errorf("custom_evaluation_delay = %v, want null", model.CustomEvaluationDelay)
		}
	})

	t.Run("populated read round-trips", func(t *testing.T) {
		got, diags := flattenAnalyticsImmediate(ctx, &alerts.AnalyticsImmediateType{
			DataprimeQuery:        &alerts.DataprimeAlertQuery{Query: &query},
			NoDataPolicy:          &alerts.NoDataPolicy{State: &state, AutoRetireSeconds: &retire},
			UseRowsAsPermutations: &permutations,
			TimeframeMinutes:      &timeframe,
			EvaluationDelayMs:     &delay,
		})
		if diags.HasError() {
			t.Fatalf("flattenAnalyticsImmediate returned diagnostics: %v", diags)
		}
		var model alerttypes.AnalyticsImmediateModel
		if diags := got.As(ctx, &model, basetypes.ObjectAsOptions{}); diags.HasError() {
			t.Fatalf("As() returned diagnostics: %v", diags)
		}
		var queryModel alerttypes.DataprimeQueryModel
		if diags := model.DataprimeQuery.As(ctx, &queryModel, basetypes.ObjectAsOptions{}); diags.HasError() {
			t.Fatalf("As() returned diagnostics: %v", diags)
		}
		if queryModel.Query.ValueString() != query {
			t.Errorf("query = %q, want %q", queryModel.Query.ValueString(), query)
		}
		if !model.UseRowsAsPermutations.ValueBool() {
			t.Errorf("use_rows_as_permutations = false, want true")
		}
		if model.TimeframeMinutes.ValueInt32() != 45 {
			t.Errorf("timeframe_minutes = %d, want 45", model.TimeframeMinutes.ValueInt32())
		}
		if model.CustomEvaluationDelay.ValueInt32() != 120000 {
			t.Errorf("custom_evaluation_delay = %d, want 120000", model.CustomEvaluationDelay.ValueInt32())
		}
		var policy alerttypes.NoDataPolicyModel
		if diags := model.NoDataPolicy.As(ctx, &policy, basetypes.ObjectAsOptions{}); diags.HasError() {
			t.Fatalf("As() returned diagnostics: %v", diags)
		}
		if policy.State.ValueString() != "ALERTING" || policy.AutoRetireSeconds.ValueInt64() != 3600 {
			t.Errorf("no_data_policy = %+v, want ALERTING/3600", policy)
		}
	})
}

func TestExpandAnalyticsThreshold(t *testing.T) {
	ctx := context.Background()
	object := types.ObjectValueMust(alertschema.AnalyticsThresholdAttr(), map[string]attr.Value{
		"dataprime_query": dataprimeQueryObject("source logs | count as error_count"),
		"rules": types.ListValueMust(
			types.ObjectType{AttrTypes: alertschema.AnalyticsThresholdRuleAttr()},
			[]attr.Value{
				analyticsThresholdRuleObject(10, "P4"),
				analyticsThresholdRuleObject(20, "P3"),
				analyticsThresholdRuleObject(30, "P2"),
			},
		),
		"operator":                 types.StringValue("LESS_THAN"),
		"target_column":            types.StringValue("error_count"),
		"no_data_policy":           types.ObjectNull(alertschema.NoDataPolicyAttr()),
		"use_rows_as_permutations": types.BoolValue(false),
		"timeframe_minutes":        types.Int32Value(30),
		"custom_evaluation_delay":  types.Int32Null(),
	})

	got, diags := expandAnalyticsThresholdTypeDefinition(ctx, &alerts.AlertDefProperties{}, object, analyticsAlertResourceModel())
	if diags.HasError() {
		t.Fatalf("expandAnalyticsThresholdTypeDefinition returned diagnostics: %v", diags)
	}
	if got.Type == nil || *got.Type != alerts.ALERTDEFTYPE_ALERT_DEF_TYPE_ANALYTICS_THRESHOLD {
		t.Fatalf("Type = %v, want ANALYTICS_THRESHOLD", got.Type)
	}
	threshold := got.AnalyticsThreshold
	if threshold == nil {
		t.Fatal("AnalyticsThreshold = nil, want value")
	}
	if threshold.Operator == nil || *threshold.Operator != alerts.ANALYTICSTHRESHOLDOPERATOR_ANALYTICS_THRESHOLD_OPERATOR_LESS_THAN {
		t.Errorf("Operator = %v, want LESS_THAN", threshold.Operator)
	}
	if threshold.TargetColumn == nil || *threshold.TargetColumn != "error_count" {
		t.Errorf("TargetColumn = %v, want error_count", threshold.TargetColumn)
	}
	if threshold.UseRowsAsPermutations == nil || *threshold.UseRowsAsPermutations {
		t.Errorf("UseRowsAsPermutations = %v, want false", threshold.UseRowsAsPermutations)
	}
	if threshold.TimeframeMinutes == nil || *threshold.TimeframeMinutes != 30 {
		t.Errorf("TimeframeMinutes = %v, want 30", threshold.TimeframeMinutes)
	}
	if threshold.EvaluationDelayMs != nil {
		t.Errorf("EvaluationDelayMs = %v, want nil", *threshold.EvaluationDelayMs)
	}
	// Configured order must survive the expand — analytics rules are positional.
	want := []float64{10, 20, 30}
	if len(threshold.Rules) != len(want) {
		t.Fatalf("Rules has %d elements, want %d", len(threshold.Rules), len(want))
	}
	for i, w := range want {
		if threshold.Rules[i].Condition == nil || *threshold.Rules[i].Condition.Threshold != w {
			t.Errorf("Rules[%d].Condition.Threshold = %v, want %v", i, threshold.Rules[i].Condition, w)
		}
		if threshold.Rules[i].Override == nil || threshold.Rules[i].Override.Priority == nil {
			t.Errorf("Rules[%d].Override = %v, want a priority", i, threshold.Rules[i].Override)
		}
	}
}

func TestFlattenAnalyticsThreshold(t *testing.T) {
	ctx := context.Background()
	query, targetColumn := "source logs | count as error_count", "error_count"
	rule := func(threshold float64, priority alerts.AlertDefPriority) alerts.AnalyticsThresholdRule {
		return alerts.AnalyticsThresholdRule{
			Condition: &alerts.AnalyticsThresholdRuleCondition{Threshold: &threshold},
			Override:  &alerts.AlertDefOverride{Priority: &priority},
		}
	}

	t.Run("rules keep submission order and exact thresholds", func(t *testing.T) {
		got, diags := flattenAnalyticsThreshold(ctx, &alerts.AnalyticsThresholdType{
			DataprimeQuery: &alerts.DataprimeAlertQuery{Query: &query},
			TargetColumn:   &targetColumn,
			Operator:       alerts.ANALYTICSTHRESHOLDOPERATOR_ANALYTICS_THRESHOLD_OPERATOR_LESS_THAN.Ptr(),
			Rules: []alerts.AnalyticsThresholdRule{
				rule(10, alerts.ALERTDEFPRIORITY_ALERT_DEF_PRIORITY_P4),
				rule(20, alerts.ALERTDEFPRIORITY_ALERT_DEF_PRIORITY_P3),
				rule(42.5, alerts.ALERTDEFPRIORITY_ALERT_DEF_PRIORITY_P1),
			},
		})
		if diags.HasError() {
			t.Fatalf("flattenAnalyticsThreshold returned diagnostics: %v", diags)
		}
		var model alerttypes.AnalyticsThresholdModel
		if diags := got.As(ctx, &model, basetypes.ObjectAsOptions{}); diags.HasError() {
			t.Fatalf("As() returned diagnostics: %v", diags)
		}
		if model.Operator.ValueString() != "LESS_THAN" {
			t.Errorf("operator = %q, want LESS_THAN", model.Operator.ValueString())
		}
		var rules []alerttypes.AnalyticsThresholdRuleModel
		if diags := model.Rules.ElementsAs(ctx, &rules, false); diags.HasError() {
			t.Fatalf("ElementsAs returned diagnostics: %v", diags)
		}
		want := []float64{10, 20, 42.5}
		if len(rules) != len(want) {
			t.Fatalf("rules has %d elements, want %d", len(rules), len(want))
		}
		for i, w := range want {
			var condition alerttypes.AnalyticsThresholdConditionModel
			if diags := rules[i].Condition.As(ctx, &condition, basetypes.ObjectAsOptions{}); diags.HasError() {
				t.Fatalf("As() returned diagnostics: %v", diags)
			}
			if condition.Threshold.ValueFloat64() != w {
				t.Errorf("rules[%d].condition.threshold = %v, want %v", i, condition.Threshold.ValueFloat64(), w)
			}
		}
	})

	t.Run("absent operator flattens to MORE_THAN, not UNSPECIFIED", func(t *testing.T) {
		got, diags := flattenAnalyticsThreshold(ctx, &alerts.AnalyticsThresholdType{
			DataprimeQuery: &alerts.DataprimeAlertQuery{Query: &query},
			TargetColumn:   &targetColumn,
			Rules:          []alerts.AnalyticsThresholdRule{rule(1, alerts.ALERTDEFPRIORITY_ALERT_DEF_PRIORITY_P2)},
		})
		if diags.HasError() {
			t.Fatalf("flattenAnalyticsThreshold returned diagnostics: %v", diags)
		}
		var model alerttypes.AnalyticsThresholdModel
		if diags := got.As(ctx, &model, basetypes.ObjectAsOptions{}); diags.HasError() {
			t.Fatalf("As() returned diagnostics: %v", diags)
		}
		if model.Operator.ValueString() != "MORE_THAN" {
			t.Errorf("operator = %q, want MORE_THAN", model.Operator.ValueString())
		}
		if !model.NoDataPolicy.IsNull() {
			t.Errorf("no_data_policy = %v, want null", model.NoDataPolicy)
		}
	})
}

// TestHasKnownAlertType guards the single arm list every envelope getter consults.
// A missing arm is silent: the alert would read back with a nil name and P5 priority.
func TestHasKnownAlertType(t *testing.T) {
	arms := map[string]func(*alerts.AlertDefProperties){
		"logs_immediate":       func(p *alerts.AlertDefProperties) { p.LogsImmediate = &alerts.LogsImmediateType{} },
		"logs_threshold":       func(p *alerts.AlertDefProperties) { p.LogsThreshold = &alerts.LogsThresholdType{} },
		"logs_anomaly":         func(p *alerts.AlertDefProperties) { p.LogsAnomaly = &alerts.LogsAnomalyType{} },
		"logs_ratio_threshold": func(p *alerts.AlertDefProperties) { p.LogsRatioThreshold = &alerts.LogsRatioThresholdType{} },
		"logs_new_value":       func(p *alerts.AlertDefProperties) { p.LogsNewValue = &alerts.LogsNewValueType{} },
		"logs_unique_count":    func(p *alerts.AlertDefProperties) { p.LogsUniqueCount = &alerts.LogsUniqueCountType{} },
		"logs_time_relative_threshold": func(p *alerts.AlertDefProperties) {
			p.LogsTimeRelativeThreshold = &alerts.LogsTimeRelativeThresholdType{}
		},
		"metric_threshold":    func(p *alerts.AlertDefProperties) { p.MetricThreshold = &alerts.MetricThresholdType{} },
		"metric_anomaly":      func(p *alerts.AlertDefProperties) { p.MetricAnomaly = &alerts.MetricAnomalyType{} },
		"tracing_immediate":   func(p *alerts.AlertDefProperties) { p.TracingImmediate = &alerts.TracingImmediateType{} },
		"tracing_threshold":   func(p *alerts.AlertDefProperties) { p.TracingThreshold = &alerts.TracingThresholdType{} },
		"flow":                func(p *alerts.AlertDefProperties) { p.Flow = &alerts.FlowType{} },
		"slo_threshold":       func(p *alerts.AlertDefProperties) { p.SloThreshold = &alerts.SloThresholdType{} },
		"analytics_immediate": func(p *alerts.AlertDefProperties) { p.AnalyticsImmediate = &alerts.AnalyticsImmediateType{} },
		"analytics_threshold": func(p *alerts.AlertDefProperties) { p.AnalyticsThreshold = &alerts.AnalyticsThresholdType{} },
	}

	if len(arms) != len(alertschema.AlertTypeDefinitionAttr()) {
		t.Fatalf("this test covers %d arms, but type_definition has %d", len(arms), len(alertschema.AlertTypeDefinitionAttr()))
	}

	name, priority := "an alert", alerts.ALERTDEFPRIORITY_ALERT_DEF_PRIORITY_P1
	for armName, set := range arms {
		t.Run(armName, func(t *testing.T) {
			properties := &alerts.AlertDefProperties{Name: &name, Priority: &priority}
			set(properties)
			if !hasKnownAlertType(properties) {
				t.Fatalf("hasKnownAlertType() = false for %s", armName)
			}
			if got := getAlertName(properties); got == nil || *got != name {
				t.Errorf("getAlertName() = %v, want %q", got, name)
			}
			if got := getAlertPriority(properties); got == nil || *got != priority {
				t.Errorf("getAlertPriority() = %v, want P1", got)
			}
			if _, diags := getActiveOn(*properties); diags.HasError() {
				t.Errorf("getActiveOn() returned diagnostics: %v", diags)
			}
		})
	}

	t.Run("unrecognized type", func(t *testing.T) {
		properties := &alerts.AlertDefProperties{Name: &name, Priority: &priority}
		if hasKnownAlertType(properties) {
			t.Fatal("hasKnownAlertType() = true for properties with no type definition")
		}
		if got := getAlertName(properties); got != nil {
			t.Errorf("getAlertName() = %v, want nil", *got)
		}
		if _, diags := getActiveOn(*properties); !diags.HasError() {
			t.Error("getActiveOn() returned no diagnostics for an unrecognized alert type")
		}
	})
}

func TestExtractUndetectedValuesManagementForRatio(t *testing.T) {
	ctx := context.Background()

	t.Run("null object is omitted", func(t *testing.T) {
		got, diags := extractUndetectedValuesManagement(ctx, types.ObjectNull(alertschema.UndetectedValuesManagementAttr()))
		if diags.HasError() {
			t.Fatalf("extractUndetectedValuesManagement returned diagnostics: %v", diags)
		}
		if got != nil {
			t.Fatalf("extractUndetectedValuesManagement() = %v, want nil", got)
		}
	})

	t.Run("values are extracted", func(t *testing.T) {
		object := types.ObjectValueMust(alertschema.UndetectedValuesManagementAttr(), map[string]attr.Value{
			"trigger_undetected_values": types.BoolValue(true),
			"auto_retire_timeframe":     types.StringValue(alerttypes.AutoRetireTimeframeProtoToSchemaMap[alerts.V3AUTORETIRETIMEFRAME_AUTO_RETIRE_TIMEFRAME_HOUR_1]),
		})
		got, diags := extractUndetectedValuesManagement(ctx, object)
		if diags.HasError() {
			t.Fatalf("extractUndetectedValuesManagement returned diagnostics: %v", diags)
		}
		if got == nil {
			t.Fatal("extractUndetectedValuesManagement() = nil, want value")
		}
		if got.TriggerUndetectedValues == nil || !*got.TriggerUndetectedValues {
			t.Errorf("TriggerUndetectedValues = %v, want true", got.TriggerUndetectedValues)
		}
		if got.AutoRetireTimeframe == nil || *got.AutoRetireTimeframe != alerts.V3AUTORETIRETIMEFRAME_AUTO_RETIRE_TIMEFRAME_HOUR_1 {
			t.Errorf("AutoRetireTimeframe = %v, want HOUR_1", got.AutoRetireTimeframe)
		}
	})
}
