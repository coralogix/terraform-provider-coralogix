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

	aievaluations "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/ai_evaluations_service"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// criteria is not an API object. The flags of the acceptable criteria are the field safe, the flags
// of the prohibited criteria are the field violates, and the examples of both are one list in which
// each example has a score: 1 for an acceptable one, 0 for a prohibited one.
const (
	aiCustomEvaluationAcceptableScore = "1"
	aiCustomEvaluationProhibitedScore = "0"
	// aiCustomEvaluationMaxExamples is the limit of the API across both lists.
	aiCustomEvaluationMaxExamples = 100
)

func aiCustomEvaluationCriterionAttributeTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"flags":    types.StringType,
		"examples": types.ListType{ElemType: types.StringType},
	}
}

func aiCustomEvaluationCriteriaAttributeTypes() map[string]attr.Type {
	criterionType := types.ObjectType{AttrTypes: aiCustomEvaluationCriterionAttributeTypes()}
	return map[string]attr.Type{
		"acceptable": criterionType,
		"prohibited": criterionType,
	}
}

// extractAICustomEvaluationCriteria turns the criteria of a configuration into the examples, the
// safe text, and the violates text of a request. A criterion that is not set is empty.
func extractAICustomEvaluationCriteria(ctx context.Context, criteria types.Object) ([]aievaluations.CustomEvaluationExample, string, string, diag.Diagnostics) {
	var diags diag.Diagnostics

	acceptableFlags, acceptableExamples, acceptableDiags := extractAICustomEvaluationCriterion(ctx, criteria, "acceptable")
	diags.Append(acceptableDiags...)
	prohibitedFlags, prohibitedExamples, prohibitedDiags := extractAICustomEvaluationCriterion(ctx, criteria, "prohibited")
	diags.Append(prohibitedDiags...)
	if diags.HasError() {
		return nil, "", "", diags
	}
	if len(acceptableExamples)+len(prohibitedExamples) > aiCustomEvaluationMaxExamples {
		diags.AddError(
			"Too many AI custom evaluation examples",
			"Custom evaluation criteria can include at most 100 total examples across acceptable and prohibited criteria.",
		)
		return nil, "", "", diags
	}

	examples := make([]aievaluations.CustomEvaluationExample, 0, len(acceptableExamples)+len(prohibitedExamples))
	for _, example := range acceptableExamples {
		examples = append(examples, aievaluations.CustomEvaluationExample{
			Conversation: aievaluations.PtrString(example),
			Score:        aievaluations.PtrString(aiCustomEvaluationAcceptableScore),
		})
	}
	for _, example := range prohibitedExamples {
		examples = append(examples, aievaluations.CustomEvaluationExample{
			Conversation: aievaluations.PtrString(example),
			Score:        aievaluations.PtrString(aiCustomEvaluationProhibitedScore),
		})
	}

	return examples, acceptableFlags, prohibitedFlags, diags
}

func extractAICustomEvaluationCriterion(ctx context.Context, criteria types.Object, name string) (string, []string, diag.Diagnostics) {
	var diags diag.Diagnostics
	if criteria.IsNull() || criteria.IsUnknown() {
		return "", nil, diags
	}
	criterion, ok := criteria.Attributes()[name].(types.Object)
	if !ok || criterion.IsNull() || criterion.IsUnknown() {
		return "", nil, diags
	}

	flags := ""
	if value, ok := criterion.Attributes()["flags"].(types.String); ok && !value.IsNull() && !value.IsUnknown() {
		flags = value.ValueString()
	}
	var examples []string
	if value, ok := criterion.Attributes()["examples"].(types.List); ok && !value.IsNull() && !value.IsUnknown() {
		diags.Append(value.ElementsAs(ctx, &examples, false)...)
	}

	return flags, examples, diags
}

// flattenAICustomEvaluationCriteria shows the flags and the scored examples of the API as criteria.
func flattenAICustomEvaluationCriteria(ctx context.Context, config aievaluations.CustomEvaluationConfig) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	acceptableExamples := make([]string, 0)
	prohibitedExamples := make([]string, 0)

	for _, example := range config.GetExamples() {
		if len(example.AdditionalProperties) != 0 {
			// The resource shows the conversation and the score only. A field that it does not know
			// would be lost on the next update, which replaces the whole list.
			diags.AddError(
				"Unsupported AI custom evaluation example field",
				fmt.Sprintf("Custom evaluation examples can have the fields conversation and score. The API returned %d more.", len(example.AdditionalProperties)),
			)
			return types.ObjectNull(aiCustomEvaluationCriteriaAttributeTypes()), diags
		}
		switch example.GetScore() {
		case aiCustomEvaluationAcceptableScore:
			acceptableExamples = append(acceptableExamples, example.GetConversation())
		case aiCustomEvaluationProhibitedScore:
			prohibitedExamples = append(prohibitedExamples, example.GetConversation())
		default:
			diags.AddError(
				"Unsupported AI custom evaluation example score",
				fmt.Sprintf("Custom evaluation examples must have score %q or %q, got %q.", aiCustomEvaluationAcceptableScore, aiCustomEvaluationProhibitedScore, example.GetScore()),
			)
			return types.ObjectNull(aiCustomEvaluationCriteriaAttributeTypes()), diags
		}
	}

	acceptable, acceptableDiags := flattenAICustomEvaluationCriterion(ctx, config.GetSafe(), acceptableExamples)
	diags.Append(acceptableDiags...)
	prohibited, prohibitedDiags := flattenAICustomEvaluationCriterion(ctx, config.GetViolates(), prohibitedExamples)
	diags.Append(prohibitedDiags...)
	if diags.HasError() {
		return types.ObjectNull(aiCustomEvaluationCriteriaAttributeTypes()), diags
	}

	criteria, criteriaDiags := types.ObjectValue(aiCustomEvaluationCriteriaAttributeTypes(), map[string]attr.Value{
		"acceptable": acceptable,
		"prohibited": prohibited,
	})
	diags.Append(criteriaDiags...)
	return criteria, diags
}

func flattenAICustomEvaluationCriterion(ctx context.Context, flags string, examples []string) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	examplesValue, exampleDiags := types.ListValueFrom(ctx, types.StringType, examples)
	diags.Append(exampleDiags...)
	if diags.HasError() {
		return types.ObjectNull(aiCustomEvaluationCriterionAttributeTypes()), diags
	}

	criterion, criterionDiags := types.ObjectValue(aiCustomEvaluationCriterionAttributeTypes(), map[string]attr.Value{
		"flags":    types.StringValue(flags),
		"examples": examplesValue,
	})
	diags.Append(criterionDiags...)
	return criterion, diags
}
