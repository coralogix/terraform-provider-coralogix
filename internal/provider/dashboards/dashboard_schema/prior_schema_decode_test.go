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
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// V1, V2 and V3 are wired as PriorSchema values in the resource's UpgradeState
// map, so any widening of a schema helper they share with the current version
// retroactively changes the type that historical state is decoded against.
// These tests decode legacy-shaped state exactly the way
// fwserver.UpgradeResourceState does, so a change that would break a real
// upgrade fails here instead of in a customer's plan.
func priorSchemaUnmarshalOpts() tfprotov6.UnmarshalOpts {
	return tfprotov6.UnmarshalOpts{
		ValueFromJSONOpts: tftypes.ValueFromJSONOpts{
			IgnoreUndefinedAttributes: true,
		},
	}
}

func decodeAgainstPriorSchema(t *testing.T, priorSchema schema.Schema, stateJSON string) {
	t.Helper()

	rawState := tfprotov6.RawState{JSON: []byte(stateJSON)}
	priorSchemaType := priorSchema.Type().TerraformType(context.Background())

	if _, err := rawState.UnmarshalWithOpts(priorSchemaType, priorSchemaUnmarshalOpts()); err != nil {
		t.Fatalf("decoding legacy state against prior schema failed: %s", err)
	}
}

// A spans filter written before `observation_field` existed carries only
// `field` and `operator`.
const legacySpansFilterStateJSON = `{
  "id": "dashboard-id",
  "name": "legacy dashboard",
  "layout": {
    "sections": [{
      "id": "section-id",
      "rows": [{
        "id": "row-id",
        "appearance": {"height": 19},
        "widgets": [{
          "id": "widget-id",
          "title": "spans",
          "definition": {
            "data_table": {
              "query": {
                "spans": {
                  "lucene_query": "*",
                  "filters": [{
                    "field": {"type": "metadata", "value": "status"},
                    "operator": {"type": "equals", "selected_values": ["ok"]}
                  }]
                }
              }
            }
          }
        }]
      }]
    }]
  }
}`

func TestPriorSchemasDecodeLegacySpansFilterState(t *testing.T) {
	for name, priorSchema := range map[string]schema.Schema{
		"v1": V1(),
		"v2": V2(),
		"v3": V3(),
	} {
		t.Run(name, func(t *testing.T) {
			decodeAgainstPriorSchema(t, priorSchema, legacySpansFilterStateJSON)
		})
	}
}

// Guards the general case behind the spans-filter one: prior-version state was
// written against a narrower schema, so every prior schema must tolerate state
// that simply omits attributes added later.
func TestPriorSchemasDecodeStateMissingLaterAttributes(t *testing.T) {
	for name, priorSchema := range map[string]schema.Schema{
		"v1": V1(),
		"v2": V2(),
		"v3": V3(),
	} {
		t.Run(name, func(t *testing.T) {
			decodeAgainstPriorSchema(t, priorSchema, fmt.Sprintf(`{"id":%q,"name":"minimal"}`, name))
		})
	}
}
