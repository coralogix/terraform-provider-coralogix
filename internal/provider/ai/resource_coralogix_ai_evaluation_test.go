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

package ai

import (
	"context"
	"fmt"
	"strings"
	"testing"

	aievaluations "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/ai_evaluations_service"
	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	fwschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestExtractAIEvaluationSQLLoadConfigSendsEveryLimit(t *testing.T) {
	t.Parallel()

	config := extractAIEvaluationSQLLoadConfig(AIEvaluationSQLLoadConfigModel{
		JoinLimit:         types.Int64Value(5),
		CteLimit:          types.Int64Value(0),
		AllowRecursiveCte: types.BoolValue(false),
	})

	if config.SqlLoad == nil {
		t.Fatalf("expected sqlLoad config, got %+v", config)
	}

	// A request carrying `config` replaces the previous config wholesale and the sub-fields have no
	// field presence, so a zero-valued or false sub-field must still be sent rather than omitted.
	if config.SqlLoad.JoinLimit == nil || *config.SqlLoad.JoinLimit != "5" {
		t.Errorf("expected joinLimit \"5\", got %v", config.SqlLoad.JoinLimit)
	}
	if config.SqlLoad.CteLimit == nil || *config.SqlLoad.CteLimit != "0" {
		t.Errorf("expected cteLimit \"0\", got %v", config.SqlLoad.CteLimit)
	}
	if config.SqlLoad.AllowRecursiveCte == nil || *config.SqlLoad.AllowRecursiveCte {
		t.Errorf("expected allowRecursiveCte false, got %v", config.SqlLoad.AllowRecursiveCte)
	}
}

func TestExtractAIEvaluationSQLLoadConfigSerializesLimitsAsStrings(t *testing.T) {
	t.Parallel()

	config := extractAIEvaluationSQLLoadConfig(AIEvaluationSQLLoadConfigModel{
		JoinLimit:         types.Int64Value(5),
		CteLimit:          types.Int64Value(3),
		AllowRecursiveCte: types.BoolValue(true),
	})

	serialized, err := config.MarshalJSON()
	if err != nil {
		t.Fatalf("failed to marshal evaluation config: %s", err)
	}

	expected := `{"sqlLoad":{"allowRecursiveCte":true,"cteLimit":"3","joinLimit":"5"}}`
	if string(serialized) != expected {
		t.Errorf("expected %s, got %s", expected, serialized)
	}
}

func TestFlattenAIEvaluationSQLLoadMaterializesZeroLimits(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name      string
		joinLimit *string
		cteLimit  *string
	}{
		{name: "zero_strings", joinLimit: aievaluations.PtrString("0"), cteLimit: aievaluations.PtrString("0")},
		{name: "absent_fields", joinLimit: nil, cteLimit: nil},
		{name: "empty_strings", joinLimit: aievaluations.PtrString(""), cteLimit: aievaluations.PtrString("")},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			model, diags := flattenAIEvaluationSQLLoad(aievaluations.SqlLoadConfig{
				JoinLimit: testCase.joinLimit,
				CteLimit:  testCase.cteLimit,
			})
			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}

			if model.JoinLimit != types.Int64Value(0) {
				t.Errorf("expected join_limit 0, got %v", model.JoinLimit)
			}
			if model.CteLimit != types.Int64Value(0) {
				t.Errorf("expected cte_limit 0, got %v", model.CteLimit)
			}
			if model.AllowRecursiveCte != types.BoolValue(false) {
				t.Errorf("expected allow_recursive_cte false, got %v", model.AllowRecursiveCte)
			}
		})
	}
}

func TestFlattenAIEvaluationSQLLoadRoundTripsExtractedConfig(t *testing.T) {
	t.Parallel()

	original := AIEvaluationSQLLoadConfigModel{
		JoinLimit:         types.Int64Value(9),
		CteLimit:          types.Int64Value(3),
		AllowRecursiveCte: types.BoolValue(true),
	}

	config := extractAIEvaluationSQLLoadConfig(original)
	model, diags := flattenAIEvaluationSQLLoad(*config.SqlLoad)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if *model != original {
		t.Errorf("expected %+v, got %+v", original, *model)
	}
}

func TestFlattenAIEvaluationSQLLoadRejectsNonNumericLimit(t *testing.T) {
	t.Parallel()

	// A value the API could not have produced must surface as a diagnostic rather than as a null on
	// a required attribute, which would fail the apply with an inconsistent-result error instead.
	_, diags := flattenAIEvaluationSQLLoad(aievaluations.SqlLoadConfig{
		JoinLimit: aievaluations.PtrString("abc"),
		CteLimit:  aievaluations.PtrString("3"),
	})

	if !diags.HasError() {
		t.Fatalf("expected a diagnostic for a non-numeric join_limit, got none")
	}
	if summary := diags.Errors()[0].Summary(); !strings.Contains(summary, "join_limit") {
		t.Errorf("expected the diagnostic to name join_limit, got %q", summary)
	}
}

func TestFlattenAIEvaluationConfigSupportsSQLLoad(t *testing.T) {
	t.Parallel()

	model, diags := flattenAIEvaluationConfig(context.Background(), aievaluations.EvaluationConfig{
		SqlLoad: &aievaluations.SqlLoadConfig{
			JoinLimit:         aievaluations.PtrString("5"),
			CteLimit:          aievaluations.PtrString("3"),
			AllowRecursiveCte: aievaluations.PtrBool(true),
		},
	})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if model.SQLLoad == nil {
		t.Fatalf("expected sql_load in the flattened config, got %+v", model)
	}
	if model.SQLLoad.JoinLimit != types.Int64Value(5) {
		t.Errorf("expected join_limit 5, got %v", model.SQLLoad.JoinLimit)
	}
}

func TestAIEvaluationConfigValidatorCoversEveryConfigArm(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	schemaResponse := &fwresource.SchemaResponse{}
	NewAIEvaluationResource().(*AIEvaluationResource).Schema(ctx, fwresource.SchemaRequest{}, schemaResponse)

	configAttribute, ok := schemaResponse.Schema.Attributes["config"].(fwschema.SingleNestedAttribute)
	if !ok {
		t.Fatalf("expected config to be a SingleNestedAttribute, got %T", schemaResponse.Schema.Attributes["config"])
	}

	validators := (&AIEvaluationResource{}).ConfigValidators(ctx)
	if len(validators) != 1 {
		t.Fatalf("expected exactly one config validator, got %d", len(validators))
	}
	description := validators[0].Description(ctx)

	// Every arm of the config union — sql_load included — must be in the ExactlyOneOf list, so
	// Terraform rejects two arms at plan time instead of surfacing the API's raw oneof error.
	for name := range configAttribute.Attributes {
		if !strings.Contains(description, fmt.Sprintf("config.%s", name)) {
			t.Errorf("config.%s is missing from the ExactlyOneOf validator: %s", name, description)
		}
	}
}
