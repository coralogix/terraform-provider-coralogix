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
	"strings"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	aievaluations "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/ai_evaluations_service"
	"github.com/hashicorp/terraform-plugin-framework-validators/float64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.ResourceWithConfigure        = &AIEvaluationResource{}
	_ resource.ResourceWithConfigValidators = &AIEvaluationResource{}
	_ resource.ResourceWithImportState      = &AIEvaluationResource{}

	aiEvaluationTargetSchemaToAPI = map[string]aievaluations.EvaluationTarget{
		"prompt":   aievaluations.EVALUATIONTARGET_PROMPT,
		"response": aievaluations.EVALUATIONTARGET_RESPONSE,
	}
	aiEvaluationTargetAPIToSchema = utils.ReverseMap(aiEvaluationTargetSchemaToAPI)
	aiEvaluationValidTargets      = utils.GetKeys(aiEvaluationTargetSchemaToAPI)

	aiEvaluationPIICategorySchemaToAPI = map[string]aievaluations.PiiCategory{
		"PHONE_NUMBER":  aievaluations.PIICATEGORY_PHONE_NUMBER,
		"EMAIL_ADDRESS": aievaluations.PIICATEGORY_EMAIL_ADDRESS,
		"CREDIT_CARD":   aievaluations.PIICATEGORY_CREDIT_CARD,
		"IBAN_CODE":     aievaluations.PIICATEGORY_IBAN_CODE,
		"US_SSN":        aievaluations.PIICATEGORY_US_SSN,
	}
	aiEvaluationPIICategoryAPIToSchema = utils.ReverseMap(aiEvaluationPIICategorySchemaToAPI)
	aiEvaluationValidPIICategories     = utils.GetKeys(aiEvaluationPIICategorySchemaToAPI)
)

func NewAIEvaluationResource() resource.Resource {
	return &AIEvaluationResource{}
}

type AIEvaluationResource struct {
	client *aievaluations.AIEvaluationsServiceAPIService
}

type AIEvaluationResourceModel struct {
	ID          types.String             `tfsdk:"id"`
	Application types.String             `tfsdk:"application"`
	Subsystem   types.String             `tfsdk:"subsystem"`
	Target      types.String             `tfsdk:"target"`
	Threshold   types.Float64            `tfsdk:"threshold"`
	IsEnabled   types.Bool               `tfsdk:"is_enabled"`
	Config      *AIEvaluationConfigModel `tfsdk:"config"`
}

type AIEvaluationConfigModel struct {
	AllowedTopics                 *AIEvaluationAllowedTopicsConfigModel                 `tfsdk:"allowed_topics"`
	Competition                   *AIEvaluationCompetitionConfigModel                   `tfsdk:"competition"`
	HallucinationCompleteness     *AIEvaluationHallucinationCompletenessConfigModel     `tfsdk:"hallucination_completeness"`
	HallucinationContextAdherence *AIEvaluationHallucinationContextAdherenceConfigModel `tfsdk:"hallucination_context_adherence"`
	HallucinationContextRelevance *AIEvaluationHallucinationContextRelevanceConfigModel `tfsdk:"hallucination_context_relevance"`
	HallucinationCorrectness      *AIEvaluationHallucinationCorrectnessConfigModel      `tfsdk:"hallucination_correctness"`
	HallucinationTaskAdherence    *AIEvaluationHallucinationTaskAdherenceConfigModel    `tfsdk:"hallucination_task_adherence"`
	LanguageMismatch              *AIEvaluationLanguageMismatchConfigModel              `tfsdk:"language_mismatch"`
	PII                           *AIEvaluationPIIConfigModel                           `tfsdk:"pii"`
	PromptInjection               *AIEvaluationPromptInjectionConfigModel               `tfsdk:"prompt_injection"`
	RestrictedTopics              *AIEvaluationRestrictedTopicsConfigModel              `tfsdk:"restricted_topics"`
	Sexism                        *AIEvaluationSexismConfigModel                        `tfsdk:"sexism"`
	SQLAllowedTables              *AIEvaluationSQLAllowedTablesConfigModel              `tfsdk:"sql_allowed_tables"`
	SQLHallucination              *AIEvaluationSQLHallucinationConfigModel              `tfsdk:"sql_hallucination"`
	SQLReadOnly                   *AIEvaluationSQLReadOnlyConfigModel                   `tfsdk:"sql_read_only"`
	SQLRestrictedTables           *AIEvaluationSQLRestrictedTablesConfigModel           `tfsdk:"sql_restricted_tables"`
	Toxicity                      *AIEvaluationToxicityConfigModel                      `tfsdk:"toxicity"`
}

type AIEvaluationAllowedTopicsConfigModel struct {
	Topics types.Set `tfsdk:"topics"`
}

type AIEvaluationCompetitionConfigModel struct {
	Competitors types.Set `tfsdk:"competitors"`
}

type AIEvaluationHallucinationCompletenessConfigModel struct{}

type AIEvaluationHallucinationContextAdherenceConfigModel struct{}

type AIEvaluationHallucinationContextRelevanceConfigModel struct{}

type AIEvaluationHallucinationCorrectnessConfigModel struct{}

type AIEvaluationHallucinationTaskAdherenceConfigModel struct{}

type AIEvaluationLanguageMismatchConfigModel struct{}

type AIEvaluationRestrictedTopicsConfigModel struct {
	Topics types.Set `tfsdk:"topics"`
}

type AIEvaluationPIIConfigModel struct {
	Categories types.Set `tfsdk:"categories"`
}

type AIEvaluationPromptInjectionConfigModel struct {
	AdditionalContext types.String `tfsdk:"additional_context"`
}

type AIEvaluationSexismConfigModel struct{}

type AIEvaluationSQLAllowedTablesConfigModel struct {
	Tables types.Set `tfsdk:"tables"`
}

type AIEvaluationSQLHallucinationConfigModel struct{}

type AIEvaluationSQLReadOnlyConfigModel struct{}

type AIEvaluationSQLRestrictedTablesConfigModel struct {
	Tables types.Set `tfsdk:"tables"`
}

type AIEvaluationToxicityConfigModel struct{}

func (r *AIEvaluationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ai_evaluation"
}

func (r *AIEvaluationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	r.client = clientSet.AIEvaluations()
}

func (r *AIEvaluationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version: 0,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "AI evaluation ID.",
			},
			"application": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 256),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				MarkdownDescription: "Name of the AI application this evaluation belongs to.",
			},
			"subsystem": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 256),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				MarkdownDescription: "Subsystem within the application.",
			},
			"target": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.OneOf(aiEvaluationValidTargets...),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				MarkdownDescription: fmt.Sprintf("Target span content the evaluation runs against. Can be one of %q.", aiEvaluationValidTargets),
			},
			"threshold": schema.Float64Attribute{
				Required: true,
				Validators: []validator.Float64{
					float64validator.Between(0, 1),
				},
				MarkdownDescription: "Score threshold. Must be between 0.0 and 1.0 inclusive.",
			},
			"is_enabled": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
				MarkdownDescription: "Whether the evaluation is active.",
			},
			"config": schema.SingleNestedAttribute{
				Required: true,
				Attributes: map[string]schema.Attribute{
					"allowed_topics":                  aiEvaluationAllowedTopicsConfigAttribute(),
					"competition":                     aiEvaluationCompetitionConfigAttribute(),
					"hallucination_completeness":      aiEvaluationHallucinationCompletenessConfigAttribute(),
					"hallucination_context_adherence": aiEvaluationHallucinationContextAdherenceConfigAttribute(),
					"hallucination_context_relevance": aiEvaluationHallucinationContextRelevanceConfigAttribute(),
					"hallucination_correctness":       aiEvaluationHallucinationCorrectnessConfigAttribute(),
					"hallucination_task_adherence":    aiEvaluationHallucinationTaskAdherenceConfigAttribute(),
					"language_mismatch":               aiEvaluationLanguageMismatchConfigAttribute(),
					"pii":                             aiEvaluationPIIConfigAttribute(),
					"prompt_injection":                aiEvaluationPromptInjectionConfigAttribute(),
					"restricted_topics":               aiEvaluationRestrictedTopicsConfigAttribute(),
					"sexism":                          aiEvaluationSexismConfigAttribute(),
					"sql_allowed_tables":              aiEvaluationSQLAllowedTablesConfigAttribute(),
					"sql_hallucination":               aiEvaluationSQLHallucinationConfigAttribute(),
					"sql_read_only":                   aiEvaluationSQLReadOnlyConfigAttribute(),
					"sql_restricted_tables":           aiEvaluationSQLRestrictedTablesConfigAttribute(),
					"toxicity":                        aiEvaluationToxicityConfigAttribute(),
				},
				MarkdownDescription: "AI evaluation configuration.",
			},
		},
		MarkdownDescription: "Coralogix AI evaluation.",
	}
}

func (r *AIEvaluationResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.ExactlyOneOf(
			path.MatchRoot("config").AtName("allowed_topics"),
			path.MatchRoot("config").AtName("competition"),
			path.MatchRoot("config").AtName("hallucination_completeness"),
			path.MatchRoot("config").AtName("hallucination_context_adherence"),
			path.MatchRoot("config").AtName("hallucination_context_relevance"),
			path.MatchRoot("config").AtName("hallucination_correctness"),
			path.MatchRoot("config").AtName("hallucination_task_adherence"),
			path.MatchRoot("config").AtName("language_mismatch"),
			path.MatchRoot("config").AtName("pii"),
			path.MatchRoot("config").AtName("prompt_injection"),
			path.MatchRoot("config").AtName("restricted_topics"),
			path.MatchRoot("config").AtName("sexism"),
			path.MatchRoot("config").AtName("sql_allowed_tables"),
			path.MatchRoot("config").AtName("sql_hallucination"),
			path.MatchRoot("config").AtName("sql_read_only"),
			path.MatchRoot("config").AtName("sql_restricted_tables"),
			path.MatchRoot("config").AtName("toxicity"),
		),
	}
}

func (r *AIEvaluationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *AIEvaluationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan AIEvaluationResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	rq, diags := extractCreateAIEvaluation(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, httpResponse, err := r.client.
		AiEvaluationsServiceCreateAiEvaluation(ctx).
		AiEvaluationsServiceCreateAiEvaluationRequest(*rq).
		Execute()
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating coralogix_ai_evaluation",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Create", rq),
		)
		return
	}

	state, diags := flattenAIEvaluation(ctx, result.GetAiEvaluation())
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *AIEvaluationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state AIEvaluationResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	result, httpResponse, err := r.client.
		AiEvaluationsServiceGetAiEvaluation(ctx, id).
		Execute()
	if err != nil {
		apiErr := cxsdkOpenapi.NewAPIError(httpResponse, err)
		if cxsdkOpenapi.Code(apiErr) == http.StatusNotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("coralogix_ai_evaluation %v is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%v will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError("Error reading coralogix_ai_evaluation", utils.FormatOpenAPIErrors(apiErr, "Read", nil))
		return
	}

	state, diags = flattenAIEvaluation(ctx, result.GetAiEvaluation())
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *AIEvaluationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan AIEvaluationResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	config, diags := extractAIEvaluationConfig(ctx, plan.Config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	rq := &aievaluations.AiEvaluationsServiceUpdateAiEvaluationRequest{
		Config:    config,
		IsEnabled: aievaluations.PtrBool(plan.IsEnabled.ValueBool()),
		Threshold: aievaluations.PtrFloat64(plan.Threshold.ValueFloat64()),
	}

	result, httpResponse, err := r.client.
		AiEvaluationsServiceUpdateAiEvaluation(ctx, plan.ID.ValueString()).
		AiEvaluationsServiceUpdateAiEvaluationRequest(*rq).
		Execute()
	if err != nil {
		resp.Diagnostics.AddError(
			"Error updating coralogix_ai_evaluation",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Update", rq),
		)
		return
	}

	state, diags := flattenAIEvaluation(ctx, result.GetAiEvaluation())
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *AIEvaluationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state AIEvaluationResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	_, httpResponse, err := r.client.
		AiEvaluationsServiceDeleteAiEvaluation(ctx, id).
		Execute()
	if err != nil {
		apiErr := cxsdkOpenapi.NewAPIError(httpResponse, err)
		if cxsdkOpenapi.Code(apiErr) == http.StatusNotFound {
			return
		}

		resp.Diagnostics.AddError(
			"Error deleting coralogix_ai_evaluation",
			utils.FormatOpenAPIErrors(apiErr, "Delete", id),
		)
	}
}

func aiEvaluationCompetitionConfigAttribute() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"competitors": aiEvaluationStringSetAttribute("Competitor names to watch for."),
		},
		MarkdownDescription: "Configuration for Competition evaluation.",
	}
}

func aiEvaluationHallucinationCompletenessConfigAttribute() schema.SingleNestedAttribute {
	return aiEvaluationEmptyConfigAttribute("Configuration for Hallucination Completeness evaluation. This evaluation type has no fields.")
}

func aiEvaluationHallucinationContextAdherenceConfigAttribute() schema.SingleNestedAttribute {
	return aiEvaluationEmptyConfigAttribute("Configuration for Hallucination Context Adherence evaluation. This evaluation type has no fields.")
}

func aiEvaluationHallucinationContextRelevanceConfigAttribute() schema.SingleNestedAttribute {
	return aiEvaluationEmptyConfigAttribute("Configuration for Hallucination Context Relevance evaluation. This evaluation type has no fields.")
}

func aiEvaluationHallucinationCorrectnessConfigAttribute() schema.SingleNestedAttribute {
	return aiEvaluationEmptyConfigAttribute("Configuration for Hallucination Correctness evaluation. This evaluation type has no fields.")
}

func aiEvaluationHallucinationTaskAdherenceConfigAttribute() schema.SingleNestedAttribute {
	return aiEvaluationEmptyConfigAttribute("Configuration for Hallucination Task Adherence evaluation. This evaluation type has no fields.")
}

func aiEvaluationLanguageMismatchConfigAttribute() schema.SingleNestedAttribute {
	return aiEvaluationEmptyConfigAttribute("Configuration for Language Mismatch evaluation. This evaluation type has no fields.")
}

func aiEvaluationRestrictedTopicsConfigAttribute() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"topics": aiEvaluationStringSetAttribute("Topics that should not appear."),
		},
		MarkdownDescription: "Configuration for Restricted Topics evaluation.",
	}
}

func aiEvaluationAllowedTopicsConfigAttribute() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"topics": aiEvaluationStringSetAttribute("Topics considered allowed."),
		},
		MarkdownDescription: "Configuration for Allowed Topics evaluation.",
	}
}

func aiEvaluationStringSetAttribute(markdownDescription string) schema.SetAttribute {
	return schema.SetAttribute{
		ElementType: types.StringType,
		Required:    true,
		Validators: []validator.Set{
			setvalidator.SizeAtLeast(1),
			setvalidator.SizeAtMost(1024),
			setvalidator.ValueStringsAre(stringvalidator.LengthBetween(1, 256)),
		},
		MarkdownDescription: markdownDescription,
	}
}

func aiEvaluationPIIConfigAttribute() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"categories": schema.SetAttribute{
				ElementType: types.StringType,
				Required:    true,
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
					setvalidator.SizeAtMost(1024),
					setvalidator.ValueStringsAre(stringvalidator.OneOf(aiEvaluationValidPIICategories...)),
				},
				MarkdownDescription: fmt.Sprintf("PII categories to detect. Can include %q.", aiEvaluationValidPIICategories),
			},
		},
		MarkdownDescription: "Configuration for PII evaluation.",
	}
}

func aiEvaluationPromptInjectionConfigAttribute() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"additional_context": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(""),
				Validators: []validator.String{
					stringvalidator.LengthBetween(0, 65536),
				},
				MarkdownDescription: "Additional context passed to the LLM evaluator.",
			},
		},
		MarkdownDescription: "Configuration for Prompt Injection evaluation.",
	}
}

func aiEvaluationSexismConfigAttribute() schema.SingleNestedAttribute {
	return aiEvaluationEmptyConfigAttribute("Configuration for Sexism evaluation. This evaluation type has no fields.")
}

func aiEvaluationSQLAllowedTablesConfigAttribute() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"tables": aiEvaluationStringSetAttribute("SQL table names that are allowed."),
		},
		MarkdownDescription: "Configuration for SQL Allowed Tables evaluation.",
	}
}

func aiEvaluationSQLHallucinationConfigAttribute() schema.SingleNestedAttribute {
	return aiEvaluationEmptyConfigAttribute("Configuration for SQL Hallucination evaluation. This evaluation type has no fields.")
}

func aiEvaluationSQLReadOnlyConfigAttribute() schema.SingleNestedAttribute {
	return aiEvaluationEmptyConfigAttribute("Configuration for SQL Read Only evaluation. This evaluation type has no fields.")
}

func aiEvaluationSQLRestrictedTablesConfigAttribute() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"tables": aiEvaluationStringSetAttribute("SQL table names that are not allowed."),
		},
		MarkdownDescription: "Configuration for SQL Restricted Tables evaluation.",
	}
}

func aiEvaluationToxicityConfigAttribute() schema.SingleNestedAttribute {
	return aiEvaluationEmptyConfigAttribute("Configuration for Toxicity evaluation. This evaluation type has no fields.")
}

func aiEvaluationEmptyConfigAttribute(markdownDescription string) schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional:            true,
		Attributes:          map[string]schema.Attribute{},
		MarkdownDescription: markdownDescription,
	}
}

func extractCreateAIEvaluation(ctx context.Context, plan AIEvaluationResourceModel) (*aievaluations.AiEvaluationsServiceCreateAiEvaluationRequest, diag.Diagnostics) {
	var diags diag.Diagnostics

	target, ok := aiEvaluationTargetSchemaToAPI[plan.Target.ValueString()]
	if !ok {
		diags.AddError(
			"Invalid AI evaluation target",
			fmt.Sprintf("Expected one of %q, got %q.", aiEvaluationValidTargets, plan.Target.ValueString()),
		)
		return nil, diags
	}

	config, configDiags := extractAIEvaluationConfig(ctx, plan.Config)
	diags.Append(configDiags...)
	if diags.HasError() {
		return nil, diags
	}

	rq := &aievaluations.AiEvaluationsServiceCreateAiEvaluationRequest{
		Application: aievaluations.PtrString(plan.Application.ValueString()),
		Subsystem:   aievaluations.PtrString(plan.Subsystem.ValueString()),
		Target:      target.Ptr(),
		Threshold:   aievaluations.PtrFloat64(plan.Threshold.ValueFloat64()),
		IsEnabled:   aievaluations.PtrBool(plan.IsEnabled.ValueBool()),
		Config:      config,
	}

	return rq, diags
}

func extractAIEvaluationConfig(ctx context.Context, model *AIEvaluationConfigModel) (*aievaluations.EvaluationConfig, diag.Diagnostics) {
	var diags diag.Diagnostics
	if model == nil {
		diags.AddError("Missing AI evaluation config", "`config` must be set.")
		return nil, diags
	}

	if config, configDiags, ok := extractAIEvaluationValuedConfig(ctx, model); ok {
		return config, configDiags
	}
	if config, ok := extractAIEvaluationMarkerConfig(model); ok {
		return config, diags
	}

	diags.AddError("Missing AI evaluation config", "Exactly one AI evaluation config block must be set.")
	return nil, diags
}

func extractAIEvaluationValuedConfig(ctx context.Context, model *AIEvaluationConfigModel) (*aievaluations.EvaluationConfig, diag.Diagnostics, bool) {
	switch {
	case model.AllowedTopics != nil:
		config, diags := extractAIEvaluationAllowedTopicsConfig(ctx, *model.AllowedTopics)
		return config, diags, true
	case model.Competition != nil:
		config, diags := extractAIEvaluationCompetitionConfig(ctx, *model.Competition)
		return config, diags, true
	case model.PII != nil:
		config, diags := extractAIEvaluationPIIConfig(ctx, *model.PII)
		return config, diags, true
	case model.PromptInjection != nil:
		return extractAIEvaluationPromptInjectionConfig(*model.PromptInjection), nil, true
	case model.RestrictedTopics != nil:
		config, diags := extractAIEvaluationRestrictedTopicsConfig(ctx, *model.RestrictedTopics)
		return config, diags, true
	case model.SQLAllowedTables != nil:
		config, diags := extractAIEvaluationSQLAllowedTablesConfig(ctx, *model.SQLAllowedTables)
		return config, diags, true
	case model.SQLRestrictedTables != nil:
		config, diags := extractAIEvaluationSQLRestrictedTablesConfig(ctx, *model.SQLRestrictedTables)
		return config, diags, true
	default:
		return nil, nil, false
	}
}

func extractAIEvaluationMarkerConfig(model *AIEvaluationConfigModel) (*aievaluations.EvaluationConfig, bool) {
	switch {
	case model.HallucinationCompleteness != nil:
		return extractAIEvaluationHallucinationCompletenessConfig(), true
	case model.HallucinationContextAdherence != nil:
		return extractAIEvaluationHallucinationContextAdherenceConfig(), true
	case model.HallucinationContextRelevance != nil:
		return extractAIEvaluationHallucinationContextRelevanceConfig(), true
	case model.HallucinationCorrectness != nil:
		return extractAIEvaluationHallucinationCorrectnessConfig(), true
	case model.HallucinationTaskAdherence != nil:
		return extractAIEvaluationHallucinationTaskAdherenceConfig(), true
	case model.LanguageMismatch != nil:
		return extractAIEvaluationLanguageMismatchConfig(), true
	case model.Sexism != nil:
		return extractAIEvaluationSexismConfig(), true
	case model.SQLHallucination != nil:
		return extractAIEvaluationSQLHallucinationConfig(), true
	case model.SQLReadOnly != nil:
		return extractAIEvaluationSQLReadOnlyConfig(), true
	case model.Toxicity != nil:
		return extractAIEvaluationToxicityConfig(), true
	default:
		return nil, false
	}
}

func extractAIEvaluationAllowedTopicsConfig(ctx context.Context, model AIEvaluationAllowedTopicsConfigModel) (*aievaluations.EvaluationConfig, diag.Diagnostics) {
	var diags diag.Diagnostics
	var topics []string

	diags.Append(model.Topics.ElementsAs(ctx, &topics, false)...)
	if diags.HasError() {
		return nil, diags
	}

	return &aievaluations.EvaluationConfig{
		AllowedTopics: &aievaluations.AllowedTopicsConfig{Topics: topics},
	}, diags
}

func extractAIEvaluationCompetitionConfig(ctx context.Context, model AIEvaluationCompetitionConfigModel) (*aievaluations.EvaluationConfig, diag.Diagnostics) {
	var diags diag.Diagnostics
	var competitors []string

	diags.Append(model.Competitors.ElementsAs(ctx, &competitors, false)...)
	if diags.HasError() {
		return nil, diags
	}

	return &aievaluations.EvaluationConfig{
		Competition: &aievaluations.CompetitionConfig{Competitors: competitors},
	}, diags
}

func extractAIEvaluationHallucinationCompletenessConfig() *aievaluations.EvaluationConfig {
	return &aievaluations.EvaluationConfig{HallucinationCompleteness: map[string]interface{}{}}
}

func extractAIEvaluationHallucinationContextAdherenceConfig() *aievaluations.EvaluationConfig {
	return &aievaluations.EvaluationConfig{HallucinationContextAdherence: map[string]interface{}{}}
}

func extractAIEvaluationHallucinationContextRelevanceConfig() *aievaluations.EvaluationConfig {
	return &aievaluations.EvaluationConfig{HallucinationContextRelevance: map[string]interface{}{}}
}

func extractAIEvaluationHallucinationCorrectnessConfig() *aievaluations.EvaluationConfig {
	return &aievaluations.EvaluationConfig{HallucinationCorrectness: map[string]interface{}{}}
}

func extractAIEvaluationHallucinationTaskAdherenceConfig() *aievaluations.EvaluationConfig {
	return &aievaluations.EvaluationConfig{HallucinationTaskAdherence: map[string]interface{}{}}
}

func extractAIEvaluationLanguageMismatchConfig() *aievaluations.EvaluationConfig {
	return &aievaluations.EvaluationConfig{LanguageMismatch: map[string]interface{}{}}
}

func extractAIEvaluationRestrictedTopicsConfig(ctx context.Context, model AIEvaluationRestrictedTopicsConfigModel) (*aievaluations.EvaluationConfig, diag.Diagnostics) {
	var diags diag.Diagnostics
	var topics []string

	diags.Append(model.Topics.ElementsAs(ctx, &topics, false)...)
	if diags.HasError() {
		return nil, diags
	}

	return &aievaluations.EvaluationConfig{
		RestrictedTopics: &aievaluations.RestrictedTopicsConfig{Topics: topics},
	}, diags
}

func extractAIEvaluationPIIConfig(ctx context.Context, model AIEvaluationPIIConfigModel) (*aievaluations.EvaluationConfig, diag.Diagnostics) {
	var diags diag.Diagnostics
	var categories []string

	diags.Append(model.Categories.ElementsAs(ctx, &categories, false)...)
	if diags.HasError() {
		return nil, diags
	}

	apiCategories := make([]aievaluations.PiiCategory, 0, len(categories))
	for _, category := range categories {
		apiCategory, ok := aiEvaluationPIICategorySchemaToAPI[category]
		if !ok {
			diags.AddError(
				"Invalid AI evaluation PII category",
				fmt.Sprintf("Expected one of %q, got %q.", aiEvaluationValidPIICategories, category),
			)
			continue
		}
		apiCategories = append(apiCategories, apiCategory)
	}
	if diags.HasError() {
		return nil, diags
	}

	return &aievaluations.EvaluationConfig{
		Pii: &aievaluations.PiiConfig{Categories: apiCategories},
	}, diags
}

func extractAIEvaluationPromptInjectionConfig(model AIEvaluationPromptInjectionConfigModel) *aievaluations.EvaluationConfig {
	promptInjectionConfig := aievaluations.PromptInjectionConfig{}
	if !model.AdditionalContext.IsNull() {
		promptInjectionConfig.AdditionalContext = aievaluations.PtrString(model.AdditionalContext.ValueString())
	}

	return &aievaluations.EvaluationConfig{PromptInjection: &promptInjectionConfig}
}

func extractAIEvaluationSexismConfig() *aievaluations.EvaluationConfig {
	return &aievaluations.EvaluationConfig{Sexism: map[string]interface{}{}}
}

func extractAIEvaluationSQLAllowedTablesConfig(ctx context.Context, model AIEvaluationSQLAllowedTablesConfigModel) (*aievaluations.EvaluationConfig, diag.Diagnostics) {
	var diags diag.Diagnostics
	var tables []string

	diags.Append(model.Tables.ElementsAs(ctx, &tables, false)...)
	if diags.HasError() {
		return nil, diags
	}

	return &aievaluations.EvaluationConfig{
		SqlAllowedTables: &aievaluations.SqlAllowedTablesConfig{Tables: tables},
	}, diags
}

func extractAIEvaluationSQLHallucinationConfig() *aievaluations.EvaluationConfig {
	return &aievaluations.EvaluationConfig{SqlHallucination: map[string]interface{}{}}
}

func extractAIEvaluationSQLReadOnlyConfig() *aievaluations.EvaluationConfig {
	return &aievaluations.EvaluationConfig{SqlReadOnly: map[string]interface{}{}}
}

func extractAIEvaluationSQLRestrictedTablesConfig(ctx context.Context, model AIEvaluationSQLRestrictedTablesConfigModel) (*aievaluations.EvaluationConfig, diag.Diagnostics) {
	var diags diag.Diagnostics
	var tables []string

	diags.Append(model.Tables.ElementsAs(ctx, &tables, false)...)
	if diags.HasError() {
		return nil, diags
	}

	return &aievaluations.EvaluationConfig{
		SqlRestrictedTables: &aievaluations.SqlRestrictedTablesConfig{Tables: tables},
	}, diags
}

func extractAIEvaluationToxicityConfig() *aievaluations.EvaluationConfig {
	return &aievaluations.EvaluationConfig{Toxicity: map[string]interface{}{}}
}

func flattenAIEvaluation(ctx context.Context, evaluation aievaluations.AiEvaluation) (AIEvaluationResourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	config, configDiags := flattenAIEvaluationConfig(ctx, evaluation.GetConfig())
	diags.Append(configDiags...)
	if diags.HasError() {
		return AIEvaluationResourceModel{}, diags
	}

	return AIEvaluationResourceModel{
		ID:          types.StringPointerValue(evaluation.Id),
		Application: types.StringPointerValue(evaluation.Application),
		Subsystem:   flattenAIEvaluationOptionalString(evaluation.Subsystem),
		Target:      types.StringValue(flattenAIEvaluationTarget(evaluation.GetTarget())),
		Threshold:   types.Float64PointerValue(evaluation.Threshold),
		IsEnabled:   types.BoolPointerValue(evaluation.IsEnabled),
		Config:      &config,
	}, diags
}

func flattenAIEvaluationConfig(ctx context.Context, config aievaluations.EvaluationConfig) (AIEvaluationConfigModel, diag.Diagnostics) {
	if model, diags, ok := flattenAIEvaluationValuedConfig(ctx, config); ok {
		return model, diags
	}
	if model, ok := flattenAIEvaluationMarkerConfig(config); ok {
		return model, nil
	}

	var diags diag.Diagnostics
	diags.AddError("Unsupported AI evaluation config", "Only Allowed Topics, Competition, Hallucination Completeness, Hallucination Context Adherence, Hallucination Context Relevance, Hallucination Correctness, Hallucination Task Adherence, Language Mismatch, PII, Prompt Injection, Restricted Topics, Sexism, SQL Allowed Tables, SQL Hallucination, SQL Read Only, SQL Restricted Tables, and Toxicity AI evaluation configs are currently supported by this resource.")
	return AIEvaluationConfigModel{}, diags
}

func flattenAIEvaluationValuedConfig(ctx context.Context, config aievaluations.EvaluationConfig) (AIEvaluationConfigModel, diag.Diagnostics, bool) {
	var diags diag.Diagnostics
	switch {
	case config.AllowedTopics != nil:
		topics, topicDiags := flattenAIEvaluationAllowedTopics(ctx, *config.AllowedTopics)
		diags.Append(topicDiags...)
		return AIEvaluationConfigModel{AllowedTopics: &AIEvaluationAllowedTopicsConfigModel{Topics: topics}}, diags, true
	case config.Competition != nil:
		competitors, competitorDiags := flattenAIEvaluationCompetition(ctx, *config.Competition)
		diags.Append(competitorDiags...)
		return AIEvaluationConfigModel{Competition: &AIEvaluationCompetitionConfigModel{Competitors: competitors}}, diags, true
	case config.Pii != nil:
		categories, categoryDiags := flattenAIEvaluationPIICategories(ctx, *config.Pii)
		diags.Append(categoryDiags...)
		return AIEvaluationConfigModel{PII: &AIEvaluationPIIConfigModel{Categories: categories}}, diags, true
	case config.PromptInjection != nil:
		promptInjection := config.PromptInjection
		return AIEvaluationConfigModel{PromptInjection: &AIEvaluationPromptInjectionConfigModel{AdditionalContext: types.StringValue(promptInjection.GetAdditionalContext())}}, diags, true
	case config.RestrictedTopics != nil:
		topics, topicDiags := flattenAIEvaluationRestrictedTopics(ctx, *config.RestrictedTopics)
		diags.Append(topicDiags...)
		return AIEvaluationConfigModel{RestrictedTopics: &AIEvaluationRestrictedTopicsConfigModel{Topics: topics}}, diags, true
	case config.SqlAllowedTables != nil:
		tables, tableDiags := flattenAIEvaluationSQLAllowedTables(ctx, *config.SqlAllowedTables)
		diags.Append(tableDiags...)
		return AIEvaluationConfigModel{SQLAllowedTables: &AIEvaluationSQLAllowedTablesConfigModel{Tables: tables}}, diags, true
	case config.SqlRestrictedTables != nil:
		tables, tableDiags := flattenAIEvaluationSQLRestrictedTables(ctx, *config.SqlRestrictedTables)
		diags.Append(tableDiags...)
		return AIEvaluationConfigModel{SQLRestrictedTables: &AIEvaluationSQLRestrictedTablesConfigModel{Tables: tables}}, diags, true
	default:
		return AIEvaluationConfigModel{}, nil, false
	}
}

func flattenAIEvaluationMarkerConfig(config aievaluations.EvaluationConfig) (AIEvaluationConfigModel, bool) {
	switch {
	case config.HallucinationCompleteness != nil:
		return AIEvaluationConfigModel{HallucinationCompleteness: &AIEvaluationHallucinationCompletenessConfigModel{}}, true
	case config.HallucinationContextAdherence != nil:
		return AIEvaluationConfigModel{HallucinationContextAdherence: &AIEvaluationHallucinationContextAdherenceConfigModel{}}, true
	case config.HallucinationContextRelevance != nil:
		return AIEvaluationConfigModel{HallucinationContextRelevance: &AIEvaluationHallucinationContextRelevanceConfigModel{}}, true
	case config.HallucinationCorrectness != nil:
		return AIEvaluationConfigModel{HallucinationCorrectness: &AIEvaluationHallucinationCorrectnessConfigModel{}}, true
	case config.HallucinationTaskAdherence != nil:
		return AIEvaluationConfigModel{HallucinationTaskAdherence: &AIEvaluationHallucinationTaskAdherenceConfigModel{}}, true
	case config.LanguageMismatch != nil:
		return AIEvaluationConfigModel{LanguageMismatch: &AIEvaluationLanguageMismatchConfigModel{}}, true
	case config.Sexism != nil:
		return AIEvaluationConfigModel{Sexism: &AIEvaluationSexismConfigModel{}}, true
	case config.SqlHallucination != nil:
		return AIEvaluationConfigModel{SQLHallucination: &AIEvaluationSQLHallucinationConfigModel{}}, true
	case config.SqlReadOnly != nil:
		return AIEvaluationConfigModel{SQLReadOnly: &AIEvaluationSQLReadOnlyConfigModel{}}, true
	case config.Toxicity != nil:
		return AIEvaluationConfigModel{Toxicity: &AIEvaluationToxicityConfigModel{}}, true
	default:
		return AIEvaluationConfigModel{}, false
	}
}

func flattenAIEvaluationTarget(target aievaluations.EvaluationTarget) string {
	if schemaTarget, ok := aiEvaluationTargetAPIToSchema[target]; ok {
		return schemaTarget
	}

	return strings.ToLower(string(target))
}

func flattenAIEvaluationOptionalString(value *string) types.String {
	if value == nil || *value == "" {
		return types.StringNull()
	}

	return types.StringValue(*value)
}

func flattenAIEvaluationAllowedTopics(ctx context.Context, allowedTopics aievaluations.AllowedTopicsConfig) (types.Set, diag.Diagnostics) {
	var diags diag.Diagnostics

	topicsSet, setDiags := types.SetValueFrom(ctx, types.StringType, allowedTopics.GetTopics())
	diags.Append(setDiags...)
	return topicsSet, diags
}

func flattenAIEvaluationCompetition(ctx context.Context, competition aievaluations.CompetitionConfig) (types.Set, diag.Diagnostics) {
	var diags diag.Diagnostics

	competitorsSet, setDiags := types.SetValueFrom(ctx, types.StringType, competition.GetCompetitors())
	diags.Append(setDiags...)
	return competitorsSet, diags
}

func flattenAIEvaluationRestrictedTopics(ctx context.Context, restrictedTopics aievaluations.RestrictedTopicsConfig) (types.Set, diag.Diagnostics) {
	var diags diag.Diagnostics

	topicsSet, setDiags := types.SetValueFrom(ctx, types.StringType, restrictedTopics.GetTopics())
	diags.Append(setDiags...)
	return topicsSet, diags
}

func flattenAIEvaluationSQLAllowedTables(ctx context.Context, sqlAllowedTables aievaluations.SqlAllowedTablesConfig) (types.Set, diag.Diagnostics) {
	var diags diag.Diagnostics

	tablesSet, setDiags := types.SetValueFrom(ctx, types.StringType, sqlAllowedTables.GetTables())
	diags.Append(setDiags...)
	return tablesSet, diags
}

func flattenAIEvaluationSQLRestrictedTables(ctx context.Context, sqlRestrictedTables aievaluations.SqlRestrictedTablesConfig) (types.Set, diag.Diagnostics) {
	var diags diag.Diagnostics

	tablesSet, setDiags := types.SetValueFrom(ctx, types.StringType, sqlRestrictedTables.GetTables())
	diags.Append(setDiags...)
	return tablesSet, diags
}

func flattenAIEvaluationPIICategories(ctx context.Context, pii aievaluations.PiiConfig) (types.Set, diag.Diagnostics) {
	var diags diag.Diagnostics

	categories := make([]string, 0, len(pii.GetCategories()))
	for _, category := range pii.GetCategories() {
		schemaCategory, ok := aiEvaluationPIICategoryAPIToSchema[category]
		if !ok {
			schemaCategory = string(category)
		}
		categories = append(categories, schemaCategory)
	}

	categorySet, setDiags := types.SetValueFrom(ctx, types.StringType, categories)
	diags.Append(setDiags...)
	return categorySet, diags
}
