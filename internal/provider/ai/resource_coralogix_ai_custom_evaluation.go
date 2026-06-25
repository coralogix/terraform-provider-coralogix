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
	"net/http"
	"regexp"
	"slices"
	"sort"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	aiapplications "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/ai_applications_service"
	aievaluations "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/ai_evaluations_service"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const (
	aiCustomEvaluationAcceptableScore    = "0"
	aiCustomEvaluationProhibitedScore    = "1"
	aiCustomEvaluationPolicyTypeQuality  = "quality"
	aiCustomEvaluationPolicyTypeSecurity = "security"
)

var (
	_ resource.ResourceWithConfigure   = &AICustomEvaluationResource{}
	_ resource.ResourceWithImportState = &AICustomEvaluationResource{}

	aiCustomEvaluationInstructionsPlaceholderRegexp = regexp.MustCompile(`\{(?:prompt|response|chat_history)\}`)
)

func NewAICustomEvaluationResource() resource.Resource {
	return &AICustomEvaluationResource{}
}

type AICustomEvaluationResource struct {
	aiApplicationsClient *aiapplications.AIApplicationsServiceAPIService
	aiEvaluationsClient  *aievaluations.AIEvaluationsServiceAPIService
}

type AICustomEvaluationResourceModel struct {
	ID                        types.String                     `tfsdk:"id"`
	Name                      types.String                     `tfsdk:"name"`
	PolicyType                types.String                     `tfsdk:"policy_type"`
	Description               types.String                     `tfsdk:"description"`
	Instructions              types.String                     `tfsdk:"instructions"`
	ShouldIncludeSystemPrompt types.Bool                       `tfsdk:"should_include_system_prompt"`
	Applications              types.Set                        `tfsdk:"applications"`
	ApplicationIDs            types.Set                        `tfsdk:"application_ids"`
	Criteria                  *AICustomEvaluationCriteriaModel `tfsdk:"criteria"`
}

type AICustomEvaluationApplicationModel struct {
	Application types.String `tfsdk:"application"`
	Subsystem   types.String `tfsdk:"subsystem"`
}

type AICustomEvaluationCriteriaModel struct {
	Acceptable *AICustomEvaluationCriterionModel `tfsdk:"acceptable"`
	Prohibited *AICustomEvaluationCriterionModel `tfsdk:"prohibited"`
}

type AICustomEvaluationCriterionModel struct {
	Flags    types.String `tfsdk:"flags"`
	Examples types.List   `tfsdk:"examples"`
}

type aiApplicationReference struct {
	ID        string
	Name      string
	Subsystem string
}

type aiApplicationSelectorKey struct {
	Application string
	Subsystem   string
}

func (r *AICustomEvaluationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ai_custom_evaluation"
}

func (r *AICustomEvaluationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	clientSet, ok := req.ProviderData.(*clientset.ClientSet)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *clientset.ClientSet, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.aiApplicationsClient = clientSet.AIApplications()
	r.aiEvaluationsClient = clientSet.AIEvaluations()
}

func (r *AICustomEvaluationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version: 0,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "AI custom evaluation ID.",
			},
			"name": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 256),
				},
				MarkdownDescription: "Display name of the custom evaluation.",
			},
			"policy_type": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 256),
					stringvalidator.OneOf(
						aiCustomEvaluationPolicyTypeQuality,
						aiCustomEvaluationPolicyTypeSecurity,
					),
				},
				MarkdownDescription: "Policy type identifier. Can be one of `quality` or `security`.",
			},
			"description": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(""),
				Validators: []validator.String{
					stringvalidator.LengthBetween(0, 65536),
				},
				MarkdownDescription: "Human-readable description. Defaults to an empty string.",
			},
			"instructions": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 1048576),
					stringvalidator.RegexMatches(
						aiCustomEvaluationInstructionsPlaceholderRegexp,
						"instructions must contain at least one of {prompt}, {response}, or {chat_history}.",
					),
				},
				MarkdownDescription: "Instructions sent to the LLM evaluator. Must contain at least one of `{prompt}`, `{response}`, or `{chat_history}`.",
			},
			"should_include_system_prompt": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "Whether to include the system prompt in the LLM input. Defaults to `false`.",
			},
			"applications": schema.SetNestedAttribute{
				Optional: true,
				Computed: true,
				Default:  setdefault.StaticValue(aiCustomEvaluationApplicationsDefaultValue()),
				Validators: []validator.Set{
					setvalidator.SizeAtMost(1024),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"application": schema.StringAttribute{
							Required: true,
							Validators: []validator.String{
								stringvalidator.LengthBetween(1, 256),
							},
							MarkdownDescription: "AI application name.",
						},
						"subsystem": schema.StringAttribute{
							Required: true,
							Validators: []validator.String{
								stringvalidator.LengthBetween(1, 256),
							},
							MarkdownDescription: "AI application subsystem.",
						},
					},
				},
				MarkdownDescription: "AI applications to link this custom evaluation to, selected by application and subsystem. Defaults to no linked applications.",
			},
			"application_ids": schema.SetAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Resolved AI application IDs linked to this custom evaluation.",
			},
			"criteria": schema.SingleNestedAttribute{
				Optional: true,
				Computed: true,
				Default:  objectdefault.StaticValue(aiCustomEvaluationCriteriaDefaultValue()),
				Attributes: map[string]schema.Attribute{
					"acceptable": aiCustomEvaluationCriterionAttribute("Criteria and examples for acceptable responses."),
					"prohibited": aiCustomEvaluationCriterionAttribute("Criteria and examples for prohibited responses."),
				},
				MarkdownDescription: "Acceptable and prohibited criteria for this custom evaluation. Defaults to empty criteria.",
			},
		},
		MarkdownDescription: "Coralogix AI custom evaluation.",
	}
}

func (r *AICustomEvaluationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *AICustomEvaluationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan AICustomEvaluationResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	applications, diags := r.resolveApplicationSelectors(ctx, plan.Applications)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	rq, diags := extractCreateAICustomEvaluation(ctx, plan, applicationIDs(applications))
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, httpResponse, err := r.aiEvaluationsClient.
		AiEvaluationsServiceCreateCustomEvaluation(ctx).
		AiEvaluationsServiceCreateCustomEvaluationRequest(*rq).
		Execute()
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating coralogix_ai_custom_evaluation",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Create", rq),
		)
		return
	}

	state, diags := flattenAICustomEvaluation(ctx, result.GetItem(), applications, nil)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *AICustomEvaluationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state AICustomEvaluationResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	customEvaluation, found, err := r.getCustomEvaluationByID(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("Error reading coralogix_ai_custom_evaluation", err.Error())
		return
	}
	if !found {
		resp.Diagnostics.AddWarning(
			fmt.Sprintf("coralogix_ai_custom_evaluation %v is in state, but no longer exists in Coralogix backend", id),
			fmt.Sprintf("%v will be recreated when you apply", id),
		)
		resp.State.RemoveResource(ctx)
		return
	}

	applicationIDs := customEvaluation.GetApplicationIds()
	applications, ok := applicationReferencesFromState(ctx, state, applicationIDs)
	if !ok {
		var applicationDiags diag.Diagnostics
		applications, applicationDiags = r.resolveApplicationIDs(ctx, applicationIDs)
		resp.Diagnostics.Append(applicationDiags...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	statePtr, diags := flattenAICustomEvaluation(ctx, customEvaluation, applications, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, statePtr)...)
}

func (r *AICustomEvaluationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan AICustomEvaluationResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state AICustomEvaluationResourceModel
	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	applications, diags := r.resolveApplicationsForUpdate(ctx, plan, state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	rq, diags := extractUpdateAICustomEvaluation(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, httpResponse, err := r.aiEvaluationsClient.
		AiEvaluationsServiceUpdateCustomEvaluation(ctx, plan.ID.ValueString()).
		AiEvaluationsServiceUpdateCustomEvaluationRequest(*rq).
		Execute()
	if err != nil {
		resp.Diagnostics.AddError(
			"Error updating coralogix_ai_custom_evaluation",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Update", rq),
		)
		return
	}

	err = r.reconcileApplicationLinks(ctx, plan.ID.ValueString(), applicationIDsFromSet(ctx, state.ApplicationIDs), applicationIDs(applications))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error updating coralogix_ai_custom_evaluation application links",
			fmt.Sprintf("%s. The custom evaluation fields may already have been updated; run Terraform again after resolving the link error.", err.Error()),
		)
		return
	}

	updated := result.GetItem()
	updated.ApplicationIds = applicationIDs(applications)
	statePtr, diags := flattenAICustomEvaluation(ctx, updated, applications, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, statePtr)...)
}

func (r *AICustomEvaluationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state AICustomEvaluationResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	_, httpResponse, err := r.aiEvaluationsClient.
		AiEvaluationsServiceDeleteCustomEvaluation(ctx, id).
		Execute()
	if err != nil {
		apiErr := cxsdkOpenapi.NewAPIError(httpResponse, err)
		if cxsdkOpenapi.Code(apiErr) == http.StatusNotFound {
			return
		}

		resp.Diagnostics.AddError(
			"Error deleting coralogix_ai_custom_evaluation",
			utils.FormatOpenAPIErrors(apiErr, "Delete", id),
		)
	}
}

func aiCustomEvaluationCriterionAttribute(markdownDescription string) schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Computed: true,
		Default:  objectdefault.StaticValue(aiCustomEvaluationCriterionDefaultValue()),
		Attributes: map[string]schema.Attribute{
			"flags": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(""),
				Validators: []validator.String{
					stringvalidator.LengthBetween(0, 65536),
				},
				MarkdownDescription: "Criterion flags.",
			},
			"examples": aiCustomEvaluationStringListAttribute(
				"Example conversations for this criterion.",
			),
		},
		MarkdownDescription: markdownDescription,
	}
}

func aiCustomEvaluationStringListAttribute(markdownDescription string) schema.ListAttribute {
	return schema.ListAttribute{
		Optional:    true,
		Computed:    true,
		ElementType: types.StringType,
		Default: listdefault.StaticValue(
			types.ListValueMust(types.StringType, []attr.Value{}),
		),
		Validators: []validator.List{
			listvalidator.SizeAtMost(100),
			listvalidator.ValueStringsAre(stringvalidator.LengthBetween(1, 65536)),
		},
		MarkdownDescription: markdownDescription,
	}
}

func aiCustomEvaluationCriteriaDefaultValue() types.Object {
	criterionType := types.ObjectType{AttrTypes: aiCustomEvaluationCriterionAttributeTypes()}

	return types.ObjectValueMust(
		map[string]attr.Type{
			"acceptable": criterionType,
			"prohibited": criterionType,
		},
		map[string]attr.Value{
			"acceptable": aiCustomEvaluationCriterionDefaultValue(),
			"prohibited": aiCustomEvaluationCriterionDefaultValue(),
		},
	)
}

func aiCustomEvaluationCriterionDefaultValue() types.Object {
	return types.ObjectValueMust(
		aiCustomEvaluationCriterionAttributeTypes(),
		map[string]attr.Value{
			"flags": types.StringValue(""),
			"examples": types.ListValueMust(
				types.StringType,
				[]attr.Value{},
			),
		},
	)
}

func aiCustomEvaluationCriterionAttributeTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"flags":    types.StringType,
		"examples": types.ListType{ElemType: types.StringType},
	}
}

func aiCustomEvaluationApplicationsDefaultValue() types.Set {
	return types.SetValueMust(
		types.ObjectType{AttrTypes: aiCustomEvaluationApplicationAttributeTypes()},
		[]attr.Value{},
	)
}

func aiCustomEvaluationApplicationAttributeTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"application": types.StringType,
		"subsystem":   types.StringType,
	}
}

func extractCreateAICustomEvaluation(ctx context.Context, plan AICustomEvaluationResourceModel, applicationIDs []string) (*aievaluations.AiEvaluationsServiceCreateCustomEvaluationRequest, diag.Diagnostics) {
	var diags diag.Diagnostics

	examples, safe, violates, criteriaDiags := extractAICustomEvaluationCriteria(ctx, plan.Criteria)
	diags.Append(criteriaDiags...)
	if diags.HasError() {
		return nil, diags
	}

	rq := &aievaluations.AiEvaluationsServiceCreateCustomEvaluationRequest{
		ApplicationIds:            applicationIDs,
		Description:               aievaluations.PtrString(plan.Description.ValueString()),
		Examples:                  examples,
		Instructions:              aievaluations.PtrString(plan.Instructions.ValueString()),
		Name:                      aievaluations.PtrString(plan.Name.ValueString()),
		PolicyType:                aievaluations.PtrString(plan.PolicyType.ValueString()),
		Safe:                      aievaluations.PtrString(safe),
		ShouldIncludeSystemPrompt: aievaluations.PtrBool(plan.ShouldIncludeSystemPrompt.ValueBool()),
		Violates:                  aievaluations.PtrString(violates),
	}

	return rq, diags
}

func extractUpdateAICustomEvaluation(ctx context.Context, plan AICustomEvaluationResourceModel) (*aievaluations.AiEvaluationsServiceUpdateCustomEvaluationRequest, diag.Diagnostics) {
	var diags diag.Diagnostics

	examples, safe, violates, criteriaDiags := extractAICustomEvaluationCriteria(ctx, plan.Criteria)
	diags.Append(criteriaDiags...)
	if diags.HasError() {
		return nil, diags
	}

	rq := &aievaluations.AiEvaluationsServiceUpdateCustomEvaluationRequest{
		Description:               aievaluations.PtrString(plan.Description.ValueString()),
		Examples:                  examples,
		Instructions:              aievaluations.PtrString(plan.Instructions.ValueString()),
		Name:                      aievaluations.PtrString(plan.Name.ValueString()),
		PolicyType:                aievaluations.PtrString(plan.PolicyType.ValueString()),
		Safe:                      aievaluations.PtrString(safe),
		ShouldIncludeSystemPrompt: aievaluations.PtrBool(plan.ShouldIncludeSystemPrompt.ValueBool()),
		Violates:                  aievaluations.PtrString(violates),
	}

	return rq, diags
}

func extractAICustomEvaluationCriteria(ctx context.Context, model *AICustomEvaluationCriteriaModel) ([]aievaluations.CustomEvaluationExample, string, string, diag.Diagnostics) {
	var diags diag.Diagnostics

	acceptable := aiCustomEvaluationEmptyCriterionModel()
	prohibited := aiCustomEvaluationEmptyCriterionModel()
	if model != nil {
		if model.Acceptable != nil {
			acceptable = *model.Acceptable
		}
		if model.Prohibited != nil {
			prohibited = *model.Prohibited
		}
	}

	acceptableFlags, acceptableExamples, acceptableDiags := extractAICustomEvaluationCriterion(ctx, acceptable)
	diags.Append(acceptableDiags...)
	prohibitedFlags, prohibitedExamples, prohibitedDiags := extractAICustomEvaluationCriterion(ctx, prohibited)
	diags.Append(prohibitedDiags...)
	if diags.HasError() {
		return nil, "", "", diags
	}
	if len(acceptableExamples)+len(prohibitedExamples) > 100 {
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

func extractAICustomEvaluationCriterion(ctx context.Context, model AICustomEvaluationCriterionModel) (string, []string, diag.Diagnostics) {
	var diags diag.Diagnostics
	var examples []string

	if !model.Examples.IsNull() && !model.Examples.IsUnknown() {
		diags.Append(model.Examples.ElementsAs(ctx, &examples, false)...)
	}

	flags := ""
	if !model.Flags.IsNull() && !model.Flags.IsUnknown() {
		flags = model.Flags.ValueString()
	}

	return flags, examples, diags
}

func aiCustomEvaluationEmptyCriterionModel() AICustomEvaluationCriterionModel {
	return AICustomEvaluationCriterionModel{
		Flags: types.StringValue(""),
		Examples: types.ListValueMust(
			types.StringType,
			[]attr.Value{},
		),
	}
}

func flattenAICustomEvaluation(ctx context.Context, customEvaluation aievaluations.CustomEvaluation, applications []aiApplicationReference, priorState *AICustomEvaluationResourceModel) (*AICustomEvaluationResourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	config := customEvaluation.GetConfig()

	criteria, criteriaDiags := flattenAICustomEvaluationCriteria(ctx, config)
	diags.Append(criteriaDiags...)
	if diags.HasError() {
		return nil, diags
	}

	applicationIDsValue, applicationIDDiags := types.SetValueFrom(ctx, types.StringType, applicationIDs(applications))
	diags.Append(applicationIDDiags...)

	applicationsValue, applicationDiags := applicationSelectorsValueFromReferences(ctx, applications)
	diags.Append(applicationDiags...)
	if priorState != nil && setsEqual(ctx, priorState.ApplicationIDs, applicationIDsValue) && !priorState.Applications.IsNull() && !priorState.Applications.IsUnknown() {
		applicationsValue = priorState.Applications
	}
	if diags.HasError() {
		return nil, diags
	}

	state := &AICustomEvaluationResourceModel{
		ID:                        types.StringValue(customEvaluation.GetId()),
		Name:                      types.StringValue(customEvaluation.GetName()),
		PolicyType:                types.StringValue(config.GetPolicyType()),
		Description:               types.StringValue(customEvaluation.GetDescription()),
		Instructions:              types.StringValue(config.GetInstructions()),
		ShouldIncludeSystemPrompt: types.BoolValue(config.GetShouldIncludeSystemPrompt()),
		Applications:              applicationsValue,
		ApplicationIDs:            applicationIDsValue,
		Criteria:                  criteria,
	}

	return state, diags
}

func flattenAICustomEvaluationCriteria(ctx context.Context, config aievaluations.CustomEvaluationConfig) (*AICustomEvaluationCriteriaModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	acceptableExamples := make([]string, 0)
	prohibitedExamples := make([]string, 0)

	for _, example := range config.GetExamples() {
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
			return nil, diags
		}
	}

	acceptable, acceptableDiags := flattenAICustomEvaluationCriterion(ctx, config.GetSafe(), acceptableExamples)
	diags.Append(acceptableDiags...)
	prohibited, prohibitedDiags := flattenAICustomEvaluationCriterion(ctx, config.GetViolates(), prohibitedExamples)
	diags.Append(prohibitedDiags...)
	if diags.HasError() {
		return nil, diags
	}

	return &AICustomEvaluationCriteriaModel{
		Acceptable: acceptable,
		Prohibited: prohibited,
	}, diags
}

func flattenAICustomEvaluationCriterion(ctx context.Context, flags string, examples []string) (*AICustomEvaluationCriterionModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	examplesValue, exampleDiags := types.ListValueFrom(ctx, types.StringType, examples)
	diags.Append(exampleDiags...)
	if diags.HasError() {
		return nil, diags
	}

	return &AICustomEvaluationCriterionModel{
		Flags:    types.StringValue(flags),
		Examples: examplesValue,
	}, diags
}

func (r *AICustomEvaluationResource) getCustomEvaluationByID(ctx context.Context, id string) (aievaluations.CustomEvaluation, bool, error) {
	resp, httpResponse, err := r.aiEvaluationsClient.
		AiEvaluationsServiceGetCustomEvaluations(ctx).
		Execute()
	if err != nil {
		apiErr := cxsdkOpenapi.NewAPIError(httpResponse, err)
		if cxsdkOpenapi.Code(apiErr) == http.StatusNotFound {
			return aievaluations.CustomEvaluation{}, false, nil
		}

		return aievaluations.CustomEvaluation{}, false, fmt.Errorf(
			"%s",
			utils.FormatOpenAPIErrors(apiErr, "Read", nil),
		)
	}

	for _, customEvaluation := range resp.GetItems() {
		if customEvaluation.GetId() == id {
			return customEvaluation, true, nil
		}
	}

	return aievaluations.CustomEvaluation{}, false, nil
}

func (r *AICustomEvaluationResource) resolveApplicationsForUpdate(ctx context.Context, plan AICustomEvaluationResourceModel, state AICustomEvaluationResourceModel) ([]aiApplicationReference, diag.Diagnostics) {
	var diags diag.Diagnostics

	planApplications, planDiags := applicationSelectorsFromSet(ctx, plan.Applications)
	diags.Append(planDiags...)
	stateApplications, stateDiags := applicationSelectorsFromSet(ctx, state.Applications)
	diags.Append(stateDiags...)
	if diags.HasError() {
		return nil, diags
	}

	if applicationSelectorsEqual(planApplications, stateApplications) && !state.ApplicationIDs.IsNull() && !state.ApplicationIDs.IsUnknown() {
		return r.resolveApplicationIDs(ctx, applicationIDsFromSet(ctx, state.ApplicationIDs))
	}

	return r.resolveApplicationSelectors(ctx, plan.Applications)
}

func (r *AICustomEvaluationResource) resolveApplicationSelectors(ctx context.Context, applications types.Set) ([]aiApplicationReference, diag.Diagnostics) {
	var diags diag.Diagnostics
	selectors, selectorDiags := applicationSelectorsFromSet(ctx, applications)
	diags.Append(selectorDiags...)
	if diags.HasError() {
		return nil, diags
	}
	if len(selectors) == 0 {
		return []aiApplicationReference{}, diags
	}

	allApplications, err := r.listAIApplications(ctx)
	if err != nil {
		diags.AddError("Error resolving AI applications", err.Error())
		return nil, diags
	}

	applicationsBySelector := make(map[aiApplicationSelectorKey][]aiApplicationReference, len(allApplications))
	for _, application := range allApplications {
		if application.Name == "" || application.ID == "" {
			continue
		}
		key := aiApplicationSelectorKey{
			Application: application.Name,
			Subsystem:   application.Subsystem,
		}
		applicationsBySelector[key] = append(applicationsBySelector[key], application)
	}

	resolved := make([]aiApplicationReference, 0, len(selectors))
	for _, selector := range selectors {
		name := selector.Application.ValueString()
		subsystem := selector.Subsystem.ValueString()
		matches := applicationsBySelector[aiApplicationSelectorKey{
			Application: name,
			Subsystem:   subsystem,
		}]
		if len(matches) == 0 {
			diags.AddError("AI application not found", fmt.Sprintf("No AI application named %q with subsystem %q was found.", name, subsystem))
			continue
		}
		if len(matches) > 1 {
			diags.AddError("Ambiguous AI application selector", fmt.Sprintf("Found %d AI applications named %q with subsystem %q.", len(matches), name, subsystem))
			continue
		}
		resolved = append(resolved, matches[0])
	}
	if diags.HasError() {
		return nil, diags
	}

	sortApplications(resolved)
	return resolved, diags
}

func (r *AICustomEvaluationResource) resolveApplicationIDs(ctx context.Context, applicationIDs []string) ([]aiApplicationReference, diag.Diagnostics) {
	var diags diag.Diagnostics
	if len(applicationIDs) == 0 {
		return []aiApplicationReference{}, diags
	}

	allApplications, err := r.listAIApplications(ctx)
	if err != nil {
		diags.AddError("Error resolving AI applications", err.Error())
		return nil, diags
	}

	applicationsByID := make(map[string]aiApplicationReference, len(allApplications))
	for _, application := range allApplications {
		if application.ID == "" {
			continue
		}
		applicationsByID[application.ID] = application
	}

	resolved := make([]aiApplicationReference, 0, len(applicationIDs))
	for _, id := range applicationIDs {
		application, ok := applicationsByID[id]
		if !ok {
			diags.AddError("AI application not found", fmt.Sprintf("Linked AI application ID %q was not found.", id))
			continue
		}
		resolved = append(resolved, application)
	}
	if diags.HasError() {
		return nil, diags
	}

	sortApplications(resolved)
	return resolved, diags
}

func (r *AICustomEvaluationResource) listAIApplications(ctx context.Context) ([]aiApplicationReference, error) {
	const pageSize = int32(200)
	var applications []aiApplicationReference
	for pageOffset := int64(0); ; pageOffset++ {
		resp, httpResponse, err := r.aiApplicationsClient.
			AiApplicationsServiceListAiApplications(ctx).
			PageSize(pageSize).
			PageOffset(pageOffset).
			Execute()
		if err != nil {
			return nil, fmt.Errorf(
				"%s",
				utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "List", nil),
			)
		}

		page := resp.GetAiApplications()
		for _, application := range page {
			applications = append(applications, aiApplicationReference{
				ID:        application.GetId(),
				Name:      application.GetApplication(),
				Subsystem: application.GetSubsystem(),
			})
		}
		if len(page) < int(pageSize) {
			break
		}
	}

	return applications, nil
}

func (r *AICustomEvaluationResource) reconcileApplicationLinks(ctx context.Context, customEvaluationID string, currentApplicationIDs []string, desiredApplicationIDs []string) error {
	current := stringSet(currentApplicationIDs)
	desired := stringSet(desiredApplicationIDs)

	toLink := make([]string, 0)
	for id := range desired {
		if _, ok := current[id]; !ok {
			toLink = append(toLink, id)
		}
	}
	sort.Strings(toLink)

	toUnlink := make([]string, 0)
	for id := range current {
		if _, ok := desired[id]; !ok {
			toUnlink = append(toUnlink, id)
		}
	}
	sort.Strings(toUnlink)

	for _, id := range toLink {
		_, httpResponse, err := r.aiEvaluationsClient.
			AiEvaluationsServiceLinkCustomEvaluation(ctx, customEvaluationID, id).
			Execute()
		if err != nil {
			return fmt.Errorf("failed to link AI application %q: %s", id, utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Link", id))
		}
	}

	for _, id := range toUnlink {
		_, httpResponse, err := r.aiEvaluationsClient.
			AiEvaluationsServiceUnlinkCustomEvaluationFromApp(ctx, customEvaluationID, id).
			Execute()
		if err != nil {
			return fmt.Errorf("failed to unlink AI application %q: %s", id, utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Unlink", id))
		}
	}

	return nil
}

func applicationIDs(applications []aiApplicationReference) []string {
	ids := make([]string, 0, len(applications))
	for _, application := range applications {
		ids = append(ids, application.ID)
	}
	sort.Strings(ids)
	return ids
}

func sortApplications(applications []aiApplicationReference) {
	sort.Slice(applications, func(i, j int) bool {
		if applications[i].ID == applications[j].ID {
			if applications[i].Name == applications[j].Name {
				return applications[i].Subsystem < applications[j].Subsystem
			}
			return applications[i].Name < applications[j].Name
		}
		return applications[i].ID < applications[j].ID
	})
}

func applicationIDsFromSet(ctx context.Context, applicationIDs types.Set) []string {
	return stringSliceFromSet(ctx, applicationIDs)
}

// Application selectors are lookup inputs; resolved IDs are the stable link identity.
// Preserve configured selectors during refresh when the backend link IDs did not change.
func applicationReferencesFromState(ctx context.Context, state AICustomEvaluationResourceModel, applicationIDs []string) ([]aiApplicationReference, bool) {
	if state.ApplicationIDs.IsNull() || state.ApplicationIDs.IsUnknown() || state.Applications.IsNull() || state.Applications.IsUnknown() {
		return nil, false
	}

	ids := append([]string(nil), applicationIDs...)
	sort.Strings(ids)
	if !stringSlicesEqual(applicationIDsFromSet(ctx, state.ApplicationIDs), ids) {
		return nil, false
	}

	selectors, diags := applicationSelectorsFromSet(ctx, state.Applications)
	if diags.HasError() || len(selectors) != len(ids) {
		return nil, false
	}

	applications := make([]aiApplicationReference, 0, len(ids))
	for i, id := range ids {
		selector := selectors[i]
		applications = append(applications, aiApplicationReference{
			ID:        id,
			Name:      selector.Application.ValueString(),
			Subsystem: selector.Subsystem.ValueString(),
		})
	}

	return applications, true
}

func applicationSelectorsValueFromReferences(ctx context.Context, applications []aiApplicationReference) (types.Set, diag.Diagnostics) {
	selectors := make([]AICustomEvaluationApplicationModel, 0, len(applications))
	for _, application := range applications {
		selectors = append(selectors, AICustomEvaluationApplicationModel{
			Application: types.StringValue(application.Name),
			Subsystem:   types.StringValue(application.Subsystem),
		})
	}

	return types.SetValueFrom(ctx, types.ObjectType{AttrTypes: aiCustomEvaluationApplicationAttributeTypes()}, selectors)
}

func applicationSelectorsFromSet(ctx context.Context, set types.Set) ([]AICustomEvaluationApplicationModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	if set.IsNull() || set.IsUnknown() {
		return nil, diags
	}

	var selectors []AICustomEvaluationApplicationModel
	diags.Append(set.ElementsAs(ctx, &selectors, false)...)
	if diags.HasError() {
		return nil, diags
	}

	sort.Slice(selectors, func(i, j int) bool {
		return applicationSelectorLess(selectors[i], selectors[j])
	})
	return selectors, diags
}

func applicationSelectorsEqual(left []AICustomEvaluationApplicationModel, right []AICustomEvaluationApplicationModel) bool {
	if len(left) != len(right) {
		return false
	}

	leftCopy := slices.Clone(left)
	rightCopy := slices.Clone(right)
	sort.Slice(leftCopy, func(i, j int) bool {
		return applicationSelectorLess(leftCopy[i], leftCopy[j])
	})
	sort.Slice(rightCopy, func(i, j int) bool {
		return applicationSelectorLess(rightCopy[i], rightCopy[j])
	})

	for i := range leftCopy {
		if leftCopy[i].Application.ValueString() != rightCopy[i].Application.ValueString() ||
			leftCopy[i].Subsystem.ValueString() != rightCopy[i].Subsystem.ValueString() {
			return false
		}
	}
	return true
}

func applicationSelectorLess(left AICustomEvaluationApplicationModel, right AICustomEvaluationApplicationModel) bool {
	if left.Application.ValueString() == right.Application.ValueString() {
		return left.Subsystem.ValueString() < right.Subsystem.ValueString()
	}
	return left.Application.ValueString() < right.Application.ValueString()
}

func stringSliceFromSet(ctx context.Context, set types.Set) []string {
	if set.IsNull() || set.IsUnknown() {
		return nil
	}

	var values []string
	diags := set.ElementsAs(ctx, &values, false)
	if diags.HasError() {
		return nil
	}
	sort.Strings(values)
	return values
}

func setsEqual(ctx context.Context, left types.Set, right types.Set) bool {
	return stringSlicesEqual(stringSliceFromSet(ctx, left), stringSliceFromSet(ctx, right))
}

func stringSlicesEqual(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}

	leftCopy := slices.Clone(left)
	rightCopy := slices.Clone(right)
	sort.Strings(leftCopy)
	sort.Strings(rightCopy)
	for i := range leftCopy {
		if leftCopy[i] != rightCopy[i] {
			return false
		}
	}
	return true
}

func stringSet(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}
