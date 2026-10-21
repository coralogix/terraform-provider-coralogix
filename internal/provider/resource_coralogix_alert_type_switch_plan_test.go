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

	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// Whether an omitted Optional+Computed block is planned unknown or null is decided
// entirely by the schema's plan modifier, which runs before any API call, so the two
// cases below pin that choice without a tenant. The acceptance tests cover the same
// ground end to end.

// alertNestedValue walks a path of attribute names so a failure names the attribute
// instead of dumping the whole alert object.
func alertNestedValue(t *testing.T, value tftypes.Value, path ...string) tftypes.Value {
	t.Helper()
	current := value
	for i, name := range path {
		var attrs map[string]tftypes.Value
		if err := current.As(&attrs); err != nil {
			t.Fatalf("%s is not a known object: %s", strings.Join(path[:i], "."), err)
		}
		next, ok := attrs[name]
		if !ok {
			t.Fatalf("%s has no attribute %q", strings.Join(path[:i], "."), name)
		}
		current = next
	}
	return current
}

// TestPlanTypeSwitchToThresholdPlansOmittedBlocksUnknown changes an alert in place from
// logs_ratio_threshold to a threshold type, leaving out the blocks the API fills in on its
// own. Their prior value is null because the previous type_definition had no such block,
// and the read flattens the API's answer to a non-null object, so a plan that carries that
// null over cannot match the applied value. The attributes have to stay unknown.
func TestPlanTypeSwitchToThresholdPlansOmittedBlocksUnknown(t *testing.T) {
	ctx := context.Background()
	object := alertResourceObjectType(ctx, t)

	prior := alertValueFromJSON(t, object, `{
		"id": "an-alert-id",
		"name": "ratio alert",
		"enabled": true,
		"priority": "P2",
		"phantom_mode": false,
		"deleted": false,
		"type_definition": {
			"logs_ratio_threshold": {
				"numerator": {"simple_filter": {"lucene_query": "message:\"request failed\""}},
				"numerator_alias": "failed",
				"denominator": {"simple_filter": {"lucene_query": "message:\"request handled\""}},
				"denominator_alias": "handled",
				"rules": [{
					"condition": {"condition_type": "MORE_THAN", "threshold": 0.1, "time_window": "1_HOUR"},
					"override": {"priority": "P2"}
				}]
			}
		}
	}`, map[string]string{"notification_group": alertNotificationGroupDefault})

	cases := []struct {
		name           string
		typeDefinition string
		arm            string
		unknown        []string
	}{
		{
			name: "metric_threshold",
			typeDefinition: `{
				"metric_threshold": {
					"metric_filter": {"promql": "sum(rate(requests_failed_total[1h]))"},
					"rules": [{
						"condition": {"condition_type": "MORE_THAN", "threshold": 0.1},
						"override": {"priority": "P2"}
					}]
				}
			}`,
			arm:     "metric_threshold",
			unknown: []string{"no_data_policy"},
		},
		{
			name: "logs_threshold",
			typeDefinition: `{
				"logs_threshold": {
					"rules": [{
						"condition": {"condition_type": "MORE_THAN", "threshold": 2, "time_window": "10_MINUTES"},
						"override": {"priority": "P2"}
					}]
				}
			}`,
			arm:     "logs_threshold",
			unknown: []string{"no_data_policy", "logs_filter"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			config := alertValueFromJSON(t, object, `{
				"id": "an-alert-id",
				"name": "ratio alert",
				"enabled": true,
				"priority": "P2",
				"phantom_mode": false
			}`, map[string]string{"type_definition": tc.typeDefinition})

			planned := planAlertResourceChange(ctx, t, object, prior, config)
			for _, attribute := range tc.unknown {
				value := alertNestedValue(t, planned, "type_definition", tc.arm, attribute)
				if value.IsKnown() {
					t.Errorf("%s was planned as %s, want unknown: a known plan value cannot match the object the read writes after apply", attribute, value.String())
				}
			}
		})
	}
}

// TestPlanOmittedBlocksKeepNonNullPriorValue is the other half of the contract, and the
// upgrade case: alerts created before this change hold the objects the read wrote in state
// even where the block was never configured. Those values have to keep coming back in the
// plan, or every such alert would plan a change after the modifier swap.
func TestPlanOmittedBlocksKeepNonNullPriorValue(t *testing.T) {
	ctx := context.Background()
	object := alertResourceObjectType(ctx, t)

	alert := `{
		"id": "an-alert-id",
		"name": "logs threshold alert",
		"enabled": true,
		"priority": "P2",
		"phantom_mode": false,
		"type_definition": {
			"logs_threshold": {
				"rules": [{
					"condition": {"condition_type": "MORE_THAN", "threshold": 2, "time_window": "10_MINUTES"},
					"override": {"priority": "P2"}
				}]
			}
		}
	}`

	config := alertValueFromJSON(t, object, alert, nil)
	// What a previous apply wrote: the read flattens an absent policy to an object of null
	// attributes and an absent filter to the filter the API reports, never to null.
	prior := alertValueFromJSON(t, object, alert, map[string]string{
		"deleted":            "false",
		"notification_group": alertNotificationGroupDefault,
		"type_definition": `{
			"logs_threshold": {
				"no_data_policy": {"auto_retire_seconds": null, "state": null},
				"logs_filter": {"simple_filter": {"lucene_query": "", "label_filters": null}},
				"rules": [{
					"condition": {"condition_type": "MORE_THAN", "threshold": 2, "time_window": "10_MINUTES"},
					"override": {"priority": "P2"}
				}]
			}
		}`,
	})

	planned := planAlertResourceChange(ctx, t, object, prior, config)
	for _, attribute := range []string{"no_data_policy", "logs_filter"} {
		want := alertNestedValue(t, prior, "type_definition", "logs_threshold", attribute)
		got := alertNestedValue(t, planned, "type_definition", "logs_threshold", attribute)
		if !got.Equal(want) {
			t.Errorf("%s was planned as %s, want the prior state value %s", attribute, got.String(), want.String())
		}
	}
}
