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
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// These tests drive the muxed provider server directly, so they cover the plan-time behavior of
// coralogix_ai_evaluation without credentials or a live AI application to hang an evaluation on.

func TestAIEvaluationSQLLoadPlansNoChangesAgainstMatchingState(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	server, object := testAIEvaluationServerAndType(ctx, t)

	config := testAIEvaluationConfigValue(t, object, map[string]tftypes.Value{
		"sql_load": testAIEvaluationSQLLoadValue(object, 5, 3, true),
	})

	// `id` is the only Computed attribute, so Terraform's proposed new state keeps the prior value.
	prior := testAIEvaluationValue(object, "00000000-0000-0000-0000-000000000001", config)
	resp, err := server.PlanResourceChange(ctx, &tfprotov6.PlanResourceChangeRequest{
		TypeName:         "coralogix_ai_evaluation",
		PriorState:       testAIEvaluationDynamicValue(t, object, prior),
		Config:           testAIEvaluationDynamicValue(t, object, testAIEvaluationValue(object, nil, config)),
		ProposedNewState: testAIEvaluationDynamicValue(t, object, prior),
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, diagnostic := range resp.Diagnostics {
		t.Errorf("unexpected diagnostic: %s: %s", diagnostic.Summary, diagnostic.Detail)
	}
	if len(resp.RequiresReplace) != 0 {
		t.Errorf("unexpected RequiresReplace paths: %v", resp.RequiresReplace)
	}

	planned, err := resp.PlannedState.Unmarshal(object)
	if err != nil {
		t.Fatal(err)
	}
	// Every sql_load sub-attribute is Required, so the plan must match state exactly rather than
	// leaving anything unknown or diffing a materialized zero against a null.
	if !planned.Equal(prior) {
		t.Errorf("expected an empty plan, got:\nprior:   %s\nplanned: %s", prior, planned)
	}
}

func TestAIEvaluationSQLLoadRejectsASecondConfigArm(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	server, object := testAIEvaluationServerAndType(ctx, t)

	configType := object.AttributeTypes["config"].(tftypes.Object)
	config := testAIEvaluationConfigValue(t, object, map[string]tftypes.Value{
		"sql_load":          testAIEvaluationSQLLoadValue(object, 5, 3, true),
		"sql_hallucination": tftypes.NewValue(configType.AttributeTypes["sql_hallucination"], map[string]tftypes.Value{}),
	})

	resp, err := server.ValidateResourceConfig(ctx, &tfprotov6.ValidateResourceConfigRequest{
		TypeName: "coralogix_ai_evaluation",
		Config:   testAIEvaluationDynamicValue(t, object, testAIEvaluationValue(object, nil, config)),
	})
	if err != nil {
		t.Fatal(err)
	}

	// The API answers a two-armed config with a raw proto oneof error, so the ExactlyOneOf validator
	// has to reject it before the request is ever sent.
	if len(resp.Diagnostics) == 0 {
		t.Fatalf("expected sql_load alongside sql_hallucination to be rejected, got no diagnostics")
	}
}

func testAIEvaluationServerAndType(ctx context.Context, t *testing.T) (tfprotov6.ProviderServer, tftypes.Object) {
	t.Helper()

	server, err := testAccProtoV6ProviderFactories["coralogix"]()
	if err != nil {
		t.Fatal(err)
	}

	schemaResponse, err := server.GetProviderSchema(ctx, &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatal(err)
	}

	resourceSchema, ok := schemaResponse.ResourceSchemas["coralogix_ai_evaluation"]
	if !ok {
		t.Fatalf("coralogix_ai_evaluation is not registered on the provider")
	}

	return server, resourceSchema.ValueType().(tftypes.Object)
}

func testAIEvaluationSQLLoadValue(object tftypes.Object, joinLimit int64, cteLimit int64, allowRecursiveCte bool) tftypes.Value {
	sqlLoadType := object.AttributeTypes["config"].(tftypes.Object).AttributeTypes["sql_load"]

	return tftypes.NewValue(sqlLoadType, map[string]tftypes.Value{
		"join_limit":          tftypes.NewValue(tftypes.Number, joinLimit),
		"cte_limit":           tftypes.NewValue(tftypes.Number, cteLimit),
		"allow_recursive_cte": tftypes.NewValue(tftypes.Bool, allowRecursiveCte),
	})
}

// testAIEvaluationConfigValue builds a config object with every arm null except the given ones.
func testAIEvaluationConfigValue(t *testing.T, object tftypes.Object, arms map[string]tftypes.Value) tftypes.Value {
	t.Helper()

	configType := object.AttributeTypes["config"].(tftypes.Object)
	attributes := make(map[string]tftypes.Value, len(configType.AttributeTypes))
	for name, attributeType := range configType.AttributeTypes {
		attributes[name] = tftypes.NewValue(attributeType, nil)
	}
	for name, value := range arms {
		if _, ok := configType.AttributeTypes[name]; !ok {
			t.Fatalf("config has no %q attribute", name)
		}
		attributes[name] = value
	}

	return tftypes.NewValue(configType, attributes)
}

func testAIEvaluationValue(object tftypes.Object, id interface{}, config tftypes.Value) tftypes.Value {
	return tftypes.NewValue(object, map[string]tftypes.Value{
		"id":          tftypes.NewValue(tftypes.String, id),
		"application": tftypes.NewValue(tftypes.String, "example-ai-application"),
		"subsystem":   tftypes.NewValue(tftypes.String, "example-subsystem"),
		"target":      tftypes.NewValue(tftypes.String, "response"),
		"threshold":   tftypes.NewValue(tftypes.Number, 0.8),
		"is_enabled":  tftypes.NewValue(tftypes.Bool, true),
		"config":      config,
	})
}

func testAIEvaluationDynamicValue(t *testing.T, object tftypes.Object, value tftypes.Value) *tfprotov6.DynamicValue {
	t.Helper()

	encoded, err := tfprotov6.NewDynamicValue(object, value)
	if err != nil {
		t.Fatal(err)
	}

	return &encoded
}
