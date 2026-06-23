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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/float64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.ResourceWithConfigure        = &AIEvaluationResource{}
	_ resource.ResourceWithConfigValidators = &AIEvaluationResource{}
	_ resource.ResourceWithImportState      = &AIEvaluationResource{}

	aiEvaluationTargetSchemaToAPI = map[string]aievaluations.EvaluationTarget{
		"prompt":       aievaluations.EVALUATIONTARGET_PROMPT,
		"response":     aievaluations.EVALUATIONTARGET_RESPONSE,
		"conversation": aievaluations.EVALUATIONTARGET_CONVERSATION,
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
	PII      *AIEvaluationPIIConfigModel      `tfsdk:"pii"`
	Toxicity *AIEvaluationToxicityConfigModel `tfsdk:"toxicity"`
}

type AIEvaluationPIIConfigModel struct {
	Categories types.Set `tfsdk:"categories"`
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
					stringvalidator.LengthAtLeast(1),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				MarkdownDescription: "Name of the AI application this evaluation belongs to.",
			},
			"subsystem": schema.StringAttribute{
				Optional: true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
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
				PlanModifiers: []planmodifier.Float64{
					float64planmodifier.RequiresReplace(),
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
					"pii":      aiEvaluationPIIConfigAttribute(),
					"toxicity": aiEvaluationToxicityConfigAttribute(),
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
			path.MatchRoot("config").AtName("pii"),
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

func aiEvaluationPIIConfigAttribute() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"categories": schema.SetAttribute{
				ElementType: types.StringType,
				Required:    true,
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
					setvalidator.ValueStringsAre(stringvalidator.OneOf(aiEvaluationValidPIICategories...)),
				},
				MarkdownDescription: fmt.Sprintf("PII categories to detect. Can include %q.", aiEvaluationValidPIICategories),
			},
		},
		MarkdownDescription: "Configuration for PII evaluation.",
	}
}

func aiEvaluationToxicityConfigAttribute() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional:            true,
		Attributes:          map[string]schema.Attribute{},
		MarkdownDescription: "Configuration for Toxicity evaluation. This evaluation type has no fields.",
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
		Target:      target.Ptr(),
		Threshold:   aievaluations.PtrFloat64(plan.Threshold.ValueFloat64()),
		IsEnabled:   aievaluations.PtrBool(plan.IsEnabled.ValueBool()),
		Config:      config,
	}
	if !plan.Subsystem.IsNull() {
		rq.Subsystem = aievaluations.PtrString(plan.Subsystem.ValueString())
	}

	return rq, diags
}

func extractAIEvaluationConfig(ctx context.Context, model *AIEvaluationConfigModel) (*aievaluations.EvaluationConfig, diag.Diagnostics) {
	var diags diag.Diagnostics
	if model == nil {
		diags.AddError("Missing AI evaluation config", "`config` must be set.")
		return nil, diags
	}

	switch {
	case model.PII != nil && model.Toxicity != nil:
		diags.AddError("Invalid AI evaluation config", "Exactly one of `config.pii` or `config.toxicity` must be set.")
		return nil, diags
	case model.PII != nil:
		return extractAIEvaluationPIIConfig(ctx, *model.PII)
	case model.Toxicity != nil:
		return extractAIEvaluationToxicityConfig(), diags
	default:
		diags.AddError("Missing AI evaluation config", "Exactly one of `config.pii` or `config.toxicity` must be set.")
		return nil, diags
	}
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

	config := aievaluations.EvaluationConfigPiiAsEvaluationConfig(
		aievaluations.NewEvaluationConfigPii(aievaluations.PiiConfig{Categories: apiCategories}),
	)

	return &config, diags
}

func extractAIEvaluationToxicityConfig() *aievaluations.EvaluationConfig {
	config := aievaluations.EvaluationConfigToxicityAsEvaluationConfig(
		aievaluations.NewEvaluationConfigToxicity(map[string]interface{}{}),
	)

	return &config
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
	var diags diag.Diagnostics

	switch actualConfig := config.GetActualInstance().(type) {
	case *aievaluations.EvaluationConfigPii:
		categories, categoryDiags := flattenAIEvaluationPIICategories(ctx, actualConfig.GetPii())
		diags.Append(categoryDiags...)
		return AIEvaluationConfigModel{PII: &AIEvaluationPIIConfigModel{Categories: categories}}, diags
	case *aievaluations.EvaluationConfigToxicity:
		return AIEvaluationConfigModel{Toxicity: &AIEvaluationToxicityConfigModel{}}, diags
	default:
		diags.AddError("Unsupported AI evaluation config", "Only PII and Toxicity AI evaluation configs are currently supported by this resource.")
		return AIEvaluationConfigModel{}, diags
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
