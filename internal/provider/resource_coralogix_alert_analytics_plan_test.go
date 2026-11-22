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

// withAttribute returns value with the attribute at the path of names replaced.
func withAttribute(t *testing.T, value, replacement tftypes.Value, names ...string) tftypes.Value {
	t.Helper()
	target := tftypes.NewAttributePath()
	for _, name := range names {
		target = target.WithAttributeName(name)
	}
	out, err := tftypes.Transform(value, func(p *tftypes.AttributePath, v tftypes.Value) (tftypes.Value, error) {
		if p.Equal(target) {
			return replacement, nil
		}
		return v, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
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

func planAlertResourceChange(ctx context.Context, t *testing.T, object tftypes.Object, prior, config tftypes.Value) tftypes.Value {
	return planAlertResourceChangeProposing(ctx, t, object, prior, config, config)
}

// planAlertResourceChangeProposing plans with the proposed new state that Terraform core would send:
// the config, plus the prior value of each Computed attribute that the config leaves null.
func planAlertResourceChangeProposing(ctx context.Context, t *testing.T, object tftypes.Object, prior, config, proposed tftypes.Value) tftypes.Value {
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
	// planAlertResourceChange passes the config itself, which is right while every
	// Computed attribute the config leaves null is also null in the prior state.
	resp, err := server.PlanResourceChange(ctx, &tfprotov6.PlanResourceChangeRequest{
		TypeName:         "coralogix_alert",
		PriorState:       encode(prior),
		Config:           encode(config),
		ProposedNewState: encode(proposed),
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
			name: "analytics_result_count",
			config: `{
				"id": "an-alert-id",
				"name": "analytics result count alert",
				"enabled": true,
				"priority": "P3",
				"phantom_mode": false,
				"type_definition": {
					"analytics_result_count": {
						"dataprime_query": {"query": "source logs | count"},
						"no_data_policy": null,
						"timeframe_minutes": 10,
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
			prior := alertValueFromJSON(t, object, tc.config, map[string]string{
				"deleted":            "false",
				"notification_group": alertNotificationGroupDefault,
			})
			planned := planAlertResourceChange(ctx, t, object, prior, config)
			if tc.name == "analytics_result_count" {
				// The API reads back the operator of a result count alert that leaves it out. The
				// config matches the state, so Terraform proposes the prior state itself.
				prior = withAttribute(t, prior, tftypes.NewValue(tftypes.String, "MORE_THAN"), "type_definition", "analytics_result_count", "operator")
				planned = planAlertResourceChangeProposing(ctx, t, object, prior, config, prior)
			}
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
		"name": "analytics result count alert",
		"enabled": true,
		"priority": "P3",
		"phantom_mode": false,
		"deleted": false,
		"type_definition": {
			"analytics_result_count": {
				"dataprime_query": {"query": "source logs | count"},
				"no_data_policy": {"state": "ALERTING", "auto_retire_seconds": 3600},
				"timeframe_minutes": 10,
				"custom_evaluation_delay": 120000,
				"operator": "MORE_THAN"
			}
		}
	}`, map[string]string{"notification_group": alertNotificationGroupDefault})

	config := alertValueFromJSON(t, object, `{
		"id": "an-alert-id",
		"name": "analytics result count alert",
		"enabled": true,
		"priority": "P3",
		"phantom_mode": false,
		"type_definition": {
			"analytics_result_count": {
				"dataprime_query": {"query": "source logs | count"},
				"no_data_policy": {"state": "ALERTING", "auto_retire_seconds": 3600},
				"timeframe_minutes": 10,
				"custom_evaluation_delay": null
			}
		}
	}`, nil)

	want := alertValueFromJSON(t, object, `{
		"id": "an-alert-id",
		"name": "analytics result count alert",
		"enabled": true,
		"priority": "P3",
		"phantom_mode": false,
		"deleted": false,
		"type_definition": {
			"analytics_result_count": {
				"dataprime_query": {"query": "source logs | count"},
				"no_data_policy": {"state": "ALERTING", "auto_retire_seconds": 3600},
				"timeframe_minutes": 10,
				"custom_evaluation_delay": null
			}
		}
	}`, map[string]string{"notification_group": alertNotificationGroupDefault})

	// operator is computed: the API fills it, so an update that leaves it out plans it unknown.
	// operator is Computed: Terraform proposes its prior value, and the framework plans it unknown
	// because the update changes the alert.
	proposed := withAttribute(t, config, tftypes.NewValue(tftypes.String, "MORE_THAN"), "type_definition", "analytics_result_count", "operator")
	want = withAttribute(t, want, tftypes.NewValue(tftypes.String, tftypes.UnknownValue), "type_definition", "analytics_result_count", "operator")

	planned := planAlertResourceChangeProposing(ctx, t, object, prior, config, proposed)
	if !planned.Equal(want) {
		t.Fatalf("removing the optional attributes did not plan them back to null:%s", describeValueDiff(t, want, planned))
	}
}
