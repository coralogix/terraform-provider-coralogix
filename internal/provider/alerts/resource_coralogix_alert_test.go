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
