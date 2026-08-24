// Copyright 2025 Coralogix Ltd.
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

package dashboard_schema

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// Terraform CLI calls UpgradeResourceState even when the stored version matches
// the current schema version, and the framework then round-trips the raw state
// through the current schema type with these options. Decoding fixtures the
// same way means a widening that would break a real upgrade fails here instead
// of in a customer's plan.
func priorSchemaUnmarshalOpts() tfprotov6.UnmarshalOpts {
	return tfprotov6.UnmarshalOpts{
		ValueFromJSONOpts: tftypes.ValueFromJSONOpts{
			IgnoreUndefinedAttributes: true,
		},
	}
}

func decodeAgainstSchema(t *testing.T, resourceSchema schema.Schema, stateJSON string) {
	t.Helper()

	rawState := tfprotov6.RawState{JSON: []byte(stateJSON)}
	schemaType := resourceSchema.Type().TerraformType(context.Background())

	if _, err := rawState.UnmarshalWithOpts(schemaType, priorSchemaUnmarshalOpts()); err != nil {
		t.Fatalf("decoding stored state against schema failed: %s", err)
	}
}

// A dynamic stat widget as written before the visualization gained its
// remaining fields and before the widget gained `interpretation`.
const narrowDynamicStatStateJSON = `{
  "id": "dashboard-id",
  "name": "dynamic stat dashboard",
  "layout": {
    "sections": [{
      "id": "section-id",
      "rows": [{
        "id": "row-id",
        "appearance": {"height": 19},
        "widgets": [{
          "id": "widget-id",
          "title": "stat",
          "definition": {
            "dynamic": {
              "query_definitions": [{
                "id": "query-id",
                "name": "errors",
                "query": {
                  "logs": {
                    "lucene_query": "*",
                    "data_mode_type": "unspecified"
                  }
                }
              }],
              "time_frame": {"relative": {"duration": "seconds:900"}},
              "visualization": {
                "stat": {
                  "unit": "bytes",
                  "threshold_type": "absolute",
                  "thresholds": [{"from": 0, "color": "green", "label": null}],
                  "value_field": {"keypath": ["duration"], "scope": "metadata"},
                  "legend": {"is_visible": true, "columns": null, "group_by_query": false, "placement": "bottom"}
                }
              }
            }
          }
        }]
      }]
    }]
  }
}`

// Proves the dynamic widget's new attributes need no schema version bump:
// attributes absent from stored JSON decode to null.
func TestCurrentSchemaDecodesNarrowDynamicStatState(t *testing.T) {
	decodeAgainstSchema(t, V4(), narrowDynamicStatStateJSON)
}

// A dynamic stat card widget as written before the visualization gained
// category_fields, value_fields and the template-variable elements.
const narrowDynamicStatCardStateJSON = `{
  "id": "dashboard-id",
  "name": "dynamic stat card dashboard",
  "layout": {
    "sections": [{
      "id": "section-id",
      "rows": [{
        "id": "row-id",
        "appearance": {"height": 19},
        "widgets": [{
          "id": "widget-id",
          "title": "stat card",
          "definition": {
            "dynamic": {
              "query_definitions": [{
                "id": "query-id",
                "query": {"logs": {"lucene_query": "*", "data_mode_type": "unspecified"}}
              }],
              "visualization": {
                "stat_card": {
                  "unit": "milliseconds",
                  "legend_by": "unspecified",
                  "title": {"template_text": "p99"},
                  "primary_value": {"observation_field": {"keypath": ["duration"], "scope": "metadata"}},
                  "color_label_mapping": {
                    "color_by": "value",
                    "range": {
                      "threshold_type": "absolute",
                      "min_max": {"auto": true},
                      "thresholds": [{"from": 0, "color": "green", "label": null}]
                    }
                  }
                }
              }
            }
          }
        }]
      }]
    }]
  }
}`

func TestCurrentSchemaDecodesNarrowDynamicStatCardState(t *testing.T) {
	decodeAgainstSchema(t, V4(), narrowDynamicStatCardStateJSON)
}

// V1, V2 and V3 are wired as PriorSchema values in the resource's UpgradeState
// map, so widening a schema helper they share with the current version
// retroactively changes the type historical state is decoded against. Every

// A dynamic time-series widget as written before the visualization gained its
// remaining fields, and before query definitions could carry an explicit id.
const narrowDynamicTimeSeriesStateJSON = `{
  "id": "dashboard-id",
  "name": "dynamic time series dashboard",
  "layout": {
    "sections": [{
      "id": "section-id",
      "rows": [{
        "id": "row-id",
        "appearance": {"height": 19},
        "widgets": [{
          "id": "widget-id",
          "title": "lines",
          "definition": {
            "dynamic": {
              "query_definitions": [{
                "id": "query-id",
                "query": {"metrics": {"promql_query": "vector(1)", "promql_query_type": "unspecified"}}
              }],
              "visualization": {
                "time_series_lines": {
                  "unit": "custom",
                  "scale_type": "linear",
                  "stacked_line": "unspecified",
                  "x_axis_time_format": "unspecified",
                  "y_axis_min": 0,
                  "y_axis_max": 99.5,
                  "legend": {"is_visible": true, "columns": null, "group_by_query": false, "placement": "bottom"}
                }
              }
            }
          }
        }]
      }]
    }]
  }
}`

func TestCurrentSchemaDecodesNarrowDynamicTimeSeriesState(t *testing.T) {
	decodeAgainstSchema(t, V4(), narrowDynamicTimeSeriesStateJSON)
}
