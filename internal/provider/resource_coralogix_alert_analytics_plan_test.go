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
	"encoding/json"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// Plan-time behavior — defaults, plan modifiers and unknown marking — never calls
// the Coralogix API, so the analytics arms' "no perpetual diff on the minimal
// config" claim can be asserted without credentials. The acceptance tests cover
// the same ground end-to-end, but only run on a tenant with the `alerts-dataprime`
// feature enabled, which makes this the coverage that always runs.

// alertNotificationGroupDefault is what the schema's static default puts in state
// for a config that omits notification_group. It is pre-existing behavior unrelated
// to the analytics arms, but a post-apply state fixture has to carry it or the
// plan comparison below trips over it instead of over the attribute under test.
const alertNotificationGroupDefault = `{
	"group_by_keys": null,
	"webhooks_settings": null,
	"destinations": null,
	"router": null
}`

func alertResourceObjectType(ctx context.Context, t *testing.T) tftypes.Object {
	t.Helper()
	server, err := testAccProtoV6ProviderFactories["coralogix"]()
	if err != nil {
		t.Fatalf("provider factory: %s", err)
	}
	schemaResp, err := server.GetProviderSchema(ctx, &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatalf("GetProviderSchema: %s", err)
	}
	alertSchema, ok := schemaResp.ResourceSchemas["coralogix_alert"]
	if !ok {
		t.Fatal("coralogix_alert is missing from the provider schema")
	}
	object, ok := alertSchema.ValueType().(tftypes.Object)
	if !ok {
		t.Fatalf("coralogix_alert schema is not an object, got %T", alertSchema.ValueType())
	}
	return object
}

// alertValueFromJSON fills every attribute the fixture omits with null, the way an
// unset attribute appears in a real plan request.
func alertValueFromJSON(t *testing.T, object tftypes.Object, raw string, overrides map[string]string) tftypes.Value {
	t.Helper()
	var attrs map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &attrs); err != nil {
		t.Fatalf("unmarshal fixture: %s", err)
	}
	for name, override := range overrides {
		attrs[name] = json.RawMessage(override)
	}
	for name := range object.AttributeTypes {
		if _, ok := attrs[name]; !ok {
			attrs[name] = json.RawMessage("null")
		}
	}
	complete, err := json.Marshal(attrs)
	if err != nil {
		t.Fatalf("marshal fixture: %s", err)
	}
	value, err := tftypes.ValueFromJSON(complete, object)
	if err != nil {
		t.Fatalf("ValueFromJSON: %s", err)
	}
	return value
}

// describeValueDiff names the top-level attributes that differ, so a failure
// points at an attribute instead of dumping the whole alert object.
func describeValueDiff(t *testing.T, want, got tftypes.Value) string {
	t.Helper()
	var wantAttrs, gotAttrs map[string]tftypes.Value
	if err := want.As(&wantAttrs); err != nil {
		return err.Error()
	}
	if err := got.As(&gotAttrs); err != nil {
		return err.Error()
	}
	diff := ""
	for name, wantValue := range wantAttrs {
		if gotValue := gotAttrs[name]; !wantValue.Equal(gotValue) {
			diff += "\n  " + name + ":\n    want = " + wantValue.String() + "\n    got  = " + gotValue.String()
		}
	}
	return diff
}

func planAlertResourceChange(ctx context.Context, t *testing.T, object tftypes.Object, prior, config tftypes.Value, proposed ...tftypes.Value) tftypes.Value {
	t.Helper()
	server, err := testAccProtoV6ProviderFactories["coralogix"]()
	if err != nil {
		t.Fatalf("provider factory: %s", err)
	}
	encode := func(v tftypes.Value) *tfprotov6.DynamicValue {
		dv, err := tfprotov6.NewDynamicValue(object, v)
		if err != nil {
			t.Fatalf("NewDynamicValue: %s", err)
		}
		return &dv
	}
	// Terraform core's proposed new state takes the config value wherever the config
	// sets one and the prior value for a Computed attribute the config leaves null.
	// Most fixtures leave the Computed attributes null in the prior state too, so the
	// proposed new state is the config itself. A fixture whose prior state holds a
	// value for a Computed attribute that the config omits passes the proposed state.
	proposedNew := config
	if len(proposed) > 0 {
		proposedNew = proposed[0]
	}
	resp, err := server.PlanResourceChange(ctx, &tfprotov6.PlanResourceChangeRequest{
		TypeName:         "coralogix_alert",
		PriorState:       encode(prior),
		Config:           encode(config),
		ProposedNewState: encode(proposedNew),
	})
	if err != nil {
		t.Fatalf("PlanResourceChange: %s", err)
	}
	for _, d := range resp.Diagnostics {
		if d.Severity == tfprotov6.DiagnosticSeverityError {
			t.Fatalf("PlanResourceChange diagnostic: %s — %s", d.Summary, d.Detail)
		}
	}
	planned, err := resp.PlannedState.Unmarshal(object)
	if err != nil {
		t.Fatalf("unmarshal planned state: %s", err)
	}
	return planned
}

// TestPlanAnalyticsMinimalConfigIsStable checks that a config with only the required
// fields set and every optional leaf unset re-plans with no diff against the state a
// previous apply wrote. A plain Optional leaf that flattened an absent value to a
// zero value, or a no_data_policy flattened to an object of null attributes, would
// fail here. It also pins that the analytics arms take the plain group_by path —
// planning group_by as null rather than unknown.
func TestPlanAnalyticsMinimalConfigIsStable(t *testing.T) {
	ctx := context.Background()
	object := alertResourceObjectType(ctx, t)

	cases := []struct {
		name   string
		config string
	}{
		{
			name: "analytics_immediate",
			config: `{
				"id": "an-alert-id",
				"name": "analytics immediate alert",
				"enabled": true,
				"priority": "P3",
				"phantom_mode": false,
				"type_definition": {
					"analytics_immediate": {
						"dataprime_query": {"query": "source logs | count"},
						"no_data_policy": null,
						"use_rows_as_permutations": null,
						"timeframe_minutes": 10,
						"evaluation_period_minutes": null,
						"custom_evaluation_delay": null
					}
				}
			}`,
		},
		{
			name: "analytics_threshold",
			config: `{
				"id": "an-alert-id",
				"name": "analytics threshold alert",
				"enabled": true,
				"priority": "P3",
				"phantom_mode": false,
				"type_definition": {
					"analytics_threshold": {
						"dataprime_query": {"query": "source logs | count as c"},
						"rules": [
							{"condition": {"threshold": 10}, "override": {"priority": "P4"}},
							{"condition": {"threshold": 20}, "override": {"priority": "P3"}}
						],
						"operator": "MORE_THAN",
						"target_column": "c",
						"no_data_policy": null,
						"use_rows_as_permutations": null,
						"timeframe_minutes": 10,
						"evaluation_period_minutes": null,
						"custom_evaluation_delay": null
					}
				}
			}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			config := alertValueFromJSON(t, object, tc.config, nil)
			// The prior state is the same config plus what a previous apply filled in.
			// The API fills in min(10, timeframe_minutes) and returns it on every read.
			priorConfig := strings.ReplaceAll(tc.config, `"evaluation_period_minutes": null`, `"evaluation_period_minutes": 10`)
			prior := alertValueFromJSON(t, object, priorConfig, map[string]string{
				"deleted":            "false",
				"notification_group": alertNotificationGroupDefault,
			})

			// The config differs from the prior state only at Computed attributes, for
			// which Terraform core proposes the prior value, so the proposed new state
			// is the prior state.
			planned := planAlertResourceChange(ctx, t, object, prior, config, prior)
			if !planned.Equal(prior) {
				t.Fatalf("plan is not empty for a config that matches state:%s", describeValueDiff(t, prior, planned))
			}
		})
	}
}

// TestPlanAnalyticsOptionalsCanBeCleared asserts the other half of the set/unset
// contract: removing an optional analytics attribute from the configuration plans
// it back to null rather than silently preserving the prior value, which is what
// Optional+Computed with UseStateForUnknown would do.
func TestPlanAnalyticsOptionalsCanBeCleared(t *testing.T) {
	ctx := context.Background()
	object := alertResourceObjectType(ctx, t)

	prior := alertValueFromJSON(t, object, `{
		"id": "an-alert-id",
		"name": "analytics immediate alert",
		"enabled": true,
		"priority": "P3",
		"phantom_mode": false,
		"deleted": false,
		"type_definition": {
			"analytics_immediate": {
				"dataprime_query": {"query": "source logs | count"},
				"no_data_policy": {"state": "ALERTING", "auto_retire_seconds": 3600},
				"use_rows_as_permutations": true,
				"timeframe_minutes": 10,
				"evaluation_period_minutes": 10,
				"custom_evaluation_delay": 120000
			}
		}
	}`, map[string]string{"notification_group": alertNotificationGroupDefault})

	config := alertValueFromJSON(t, object, `{
		"id": "an-alert-id",
		"name": "analytics immediate alert",
		"enabled": true,
		"priority": "P3",
		"phantom_mode": false,
		"type_definition": {
			"analytics_immediate": {
				"dataprime_query": {"query": "source logs | count"},
				"no_data_policy": {"state": "ALERTING", "auto_retire_seconds": 3600},
				"use_rows_as_permutations": null,
				"timeframe_minutes": 10,
				"evaluation_period_minutes": 10,
				"custom_evaluation_delay": null
			}
		}
	}`, nil)

	want := alertValueFromJSON(t, object, `{
		"id": "an-alert-id",
		"name": "analytics immediate alert",
		"enabled": true,
		"priority": "P3",
		"phantom_mode": false,
		"deleted": false,
		"type_definition": {
			"analytics_immediate": {
				"dataprime_query": {"query": "source logs | count"},
				"no_data_policy": {"state": "ALERTING", "auto_retire_seconds": 3600},
				"use_rows_as_permutations": null,
				"timeframe_minutes": 10,
				"evaluation_period_minutes": 10,
				"custom_evaluation_delay": null
			}
		}
	}`, map[string]string{"notification_group": alertNotificationGroupDefault})

	planned := planAlertResourceChange(ctx, t, object, prior, config)
	if !planned.Equal(want) {
		t.Fatalf("removing the optional attributes did not plan them back to null:%s", describeValueDiff(t, want, planned))
	}
}

// analyticsImmediateConfig builds an analytics_immediate alert with the given
// timeframe and evaluation period (JSON literals, "null" for unset) and name.
func analyticsImmediateConfig(name, timeframe, period string) string {
	return `{
		"id": "an-alert-id",
		"name": "` + name + `",
		"enabled": true,
		"priority": "P3",
		"phantom_mode": false,
		"type_definition": {
			"analytics_immediate": {
				"dataprime_query": {"query": "source logs | count"},
				"no_data_policy": null,
				"use_rows_as_permutations": null,
				"timeframe_minutes": ` + timeframe + `,
				"evaluation_period_minutes": ` + period + `,
				"custom_evaluation_delay": null
			}
		}
	}`
}

func plannedEvaluationPeriod(t *testing.T, planned tftypes.Value) tftypes.Value {
	t.Helper()
	value, _, err := tftypes.WalkAttributePath(planned, tftypes.NewAttributePath().
		WithAttributeName("type_definition").
		WithAttributeName("analytics_immediate").
		WithAttributeName("evaluation_period_minutes"))
	if err != nil {
		t.Fatalf("walk planned state: %s", err)
	}
	return value.(tftypes.Value)
}

// TestPlanAnalyticsEvaluationPeriodOnUpdate pins the plan of an evaluation period
// that the config omits while the alert changes. A live probe showed that the API
// keeps the stored value on a replace that omits it, and rejects the replace when
// that value exceeds the new timeframe_minutes. So the plan keeps the prior value,
// and falls back to the default min(10, timeframe_minutes) only when keeping it
// would be invalid. It is never unknown on an update, so the request always sends it.
func TestPlanAnalyticsEvaluationPeriodOnUpdate(t *testing.T) {
	ctx := context.Background()
	object := alertResourceObjectType(ctx, t)
	prior := alertValueFromJSON(t, object, analyticsImmediateConfig("an alert", "10", "5"), map[string]string{
		"deleted":            "false",
		"notification_group": alertNotificationGroupDefault,
	})

	cases := []struct {
		name   string
		config string
		want   int64
	}{
		{"omitted and another attribute changes", analyticsImmediateConfig("a renamed alert", "10", "null"), 5},
		{"omitted and the timeframe stays above it", analyticsImmediateConfig("an alert", "6", "null"), 5},
		{"omitted and the timeframe drops to it", analyticsImmediateConfig("an alert", "5", "null"), 5},
		{"omitted and the timeframe drops below it", analyticsImmediateConfig("an alert", "3", "null"), 3},
		{"configured value is kept", analyticsImmediateConfig("a renamed alert", "10", "7"), 7},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			config := alertValueFromJSON(t, object, tc.config, nil)
			planned := plannedEvaluationPeriod(t, planAlertResourceChange(ctx, t, object, prior, config))
			want := tftypes.NewValue(tftypes.Number, tc.want)
			if !planned.Equal(want) {
				t.Fatalf("evaluation_period_minutes = %s, want %d", planned, tc.want)
			}
		})
	}

	t.Run("create leaves it for the API to fill in", func(t *testing.T) {
		config := alertValueFromJSON(t, object, analyticsImmediateConfig("an alert", "10", "null"), nil)
		emptyPrior := tftypes.NewValue(object, nil)
		planned := plannedEvaluationPeriod(t, planAlertResourceChange(ctx, t, object, emptyPrior, config))
		if planned.IsKnown() {
			t.Fatalf("evaluation_period_minutes = %s, want unknown on create", planned)
		}
	})
}

// TestValidateAnalyticsEvaluationPeriod covers the range (1 to 15) and the
// cross-field rule against timeframe_minutes, both rejected by the API with
// INVALID_ARGUMENT, so that a bad value fails at plan time.
func TestValidateAnalyticsEvaluationPeriod(t *testing.T) {
	ctx := context.Background()
	object := alertResourceObjectType(ctx, t)
	server, err := testAccProtoV6ProviderFactories["coralogix"]()
	if err != nil {
		t.Fatalf("provider factory: %s", err)
	}

	cases := []struct {
		name      string
		timeframe string
		period    string
		wantError bool
	}{
		{"lowest value", "10", "1", false},
		{"highest value", "20", "15", false},
		{"equal to the timeframe", "5", "5", false},
		{"omitted", "5", "null", false},
		{"unknown timeframe is not checked", "null", "9", false},
		{"zero", "10", "0", true},
		{"above 15", "30", "16", true},
		{"above the timeframe", "5", "10", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			config := alertValueFromJSON(t, object, analyticsImmediateConfig("an alert", tc.timeframe, tc.period), nil)
			dv, err := tfprotov6.NewDynamicValue(object, config)
			if err != nil {
				t.Fatalf("NewDynamicValue: %s", err)
			}
			resp, err := server.ValidateResourceConfig(ctx, &tfprotov6.ValidateResourceConfigRequest{
				TypeName: "coralogix_alert",
				Config:   &dv,
			})
			if err != nil {
				t.Fatalf("ValidateResourceConfig: %s", err)
			}
			got := false
			for _, d := range resp.Diagnostics {
				if d.Severity == tfprotov6.DiagnosticSeverityError && (d.Attribute != nil && strings.Contains(d.Attribute.String(), "evaluation_period_minutes")) {
					got = true
				}
			}
			if got != tc.wantError {
				t.Fatalf("error on evaluation_period_minutes = %v, want %v; diagnostics: %v", got, tc.wantError, resp.Diagnostics)
			}
		})
	}
}
