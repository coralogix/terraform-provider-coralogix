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

package dataplans

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	tcoPolicys "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/policies_service"
	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	"github.com/hashicorp/terraform-plugin-framework-validators/float64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/mapvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

var (
	_ resource.ResourceWithConfigure      = &QuotaRuleResource{}
	_ resource.ResourceWithValidateConfig = &QuotaRuleResource{}
	_ resource.ResourceWithImportState    = &QuotaRuleResource{}
)

const quotaRuleDefaultDataspace = "default"

var quotaRuleTargetDataspaceRegexp = regexp.MustCompile(`^[A-Za-z](?:[A-Za-z0-9_]|\.[A-Za-z0-9_])*$`)
var quotaRuleTagRuleKeyRegexp = regexp.MustCompile(`^tags\..+`)

func NewQuotaRuleResource() resource.Resource {
	return &QuotaRuleResource{}
}

type QuotaRuleResource struct {
	client *tcoPolicys.PoliciesServiceAPIService
}

type QuotaRuleModel struct {
	ID                         types.String `tfsdk:"id"`
	Name                       types.String `tfsdk:"name"`
	Description                types.String `tfsdk:"description"`
	Enabled                    types.Bool   `tfsdk:"enabled"`
	Priority                   types.String `tfsdk:"priority"`
	Order                      types.Int64  `tfsdk:"order"`
	ApplicationRule            types.Object `tfsdk:"application_rule"`
	SubsystemRule              types.Object `tfsdk:"subsystem_rule"`
	ArchiveRetentionID         types.String `tfsdk:"archive_retention_id"`
	LogRules                   types.Object `tfsdk:"log_rules"`
	SpanRules                  types.Object `tfsdk:"span_rules"`
	QuotaBasedPriorityOverride types.Object `tfsdk:"quota_based_priority_override"`
	Targets                    types.List   `tfsdk:"targets"`
}

type QuotaRuleLogRulesModel struct {
	Severities     types.Set    `tfsdk:"severities"`
	DpxlExpression types.String `tfsdk:"dpxl_expression"`
}

type QuotaRuleSpanRulesModel struct {
	ServiceRule types.Object `tfsdk:"service_rule"`
	ActionRule  types.Object `tfsdk:"action_rule"`
	TagRules    types.Map    `tfsdk:"tag_rules"`
}

type QuotaRuleTargetModel struct {
	Dataset                    types.String `tfsdk:"dataset"`
	Dataspace                  types.String `tfsdk:"dataspace"`
	Priority                   types.String `tfsdk:"priority"`
	ArchiveRetentionID         types.String `tfsdk:"archive_retention_id"`
	QuotaBasedPriorityOverride types.Object `tfsdk:"quota_based_priority_override"`
}

func (r *QuotaRuleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_quota_rule"
}

func (r *QuotaRuleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	r.client = clientSet.TCOPolicies()
}

func (r *QuotaRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *QuotaRuleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a single Coralogix quota policy. Provider credentials select the account and team context. Use exactly one of `log_rules` or `span_rules` to select the source type, and use `targets` with `dataset` and `dataspace` for target routing.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Backend quota policy ID. Import accepts this value.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				MarkdownDescription: "Quota policy name.",
			},
			"description": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(""),
				MarkdownDescription: "Quota policy description.",
			},
			"enabled": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
				MarkdownDescription: "Whether the quota policy is enabled.",
			},
			"priority": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Validators: []validator.String{
					stringvalidator.OneOf(tcoPoliciesValidPriorities...),
				},
				MarkdownDescription: fmt.Sprintf("Policy-level quota priority. Required when `targets` is not configured. Do not set when target-level priority or priority overrides are configured. Valid values are %q.", tcoPoliciesValidPriorities),
			},
			"order": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Backend evaluation order among quota policies.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"application_rule": ruleNestedAttribute("Application names matched by this quota policy."),
			"subsystem_rule":   ruleNestedAttribute("Subsystem names matched by this quota policy."),
			"archive_retention_id": schema.StringAttribute{
				Optional: true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				MarkdownDescription: "Archive retention ID applied by this quota policy.",
			},
			"log_rules": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"severities": schema.SetAttribute{
						Optional:    true,
						ElementType: types.StringType,
						Validators: []validator.Set{
							setvalidator.SizeAtLeast(1),
							setvalidator.ValueStringsAre(stringvalidator.OneOf(validPolicySeverities...)),
						},
						MarkdownDescription: fmt.Sprintf("Log severities matched by this quota policy. Valid values are %q. Mutually exclusive with `dpxl_expression`.", validPolicySeverities),
					},
					"dpxl_expression": schema.StringAttribute{
						Optional: true,
						Validators: []validator.String{
							stringvalidator.LengthAtLeast(1),
							stringvalidator.ConflictsWith(path.MatchRelative().AtParent().AtName("severities")),
						},
						MarkdownDescription: "DataPrime expression matched by this quota policy. Mutually exclusive with `severities`. The expression must include a version prefix, e.g. `<v1> $d.severity == 'INFO'`.",
					},
				},
				MarkdownDescription: "Log source-type matching rules. Exactly one of `log_rules` or `span_rules` must be set.",
			},
			"span_rules": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"service_rule": ruleNestedAttribute("Service names matched by this span quota policy."),
					"action_rule":  ruleNestedAttribute("Action names matched by this span quota policy."),
					"tag_rules": schema.MapNestedAttribute{
						Optional: true,
						Validators: []validator.Map{
							mapvalidator.KeysAre(stringvalidator.RegexMatches(quotaRuleTagRuleKeyRegexp, "tag names must have a 'tags.' prefix followed by a tag name")),
						},
						NestedObject: schema.NestedAttributeObject{
							Attributes: quotaRuleRuleSchemaAttributes(),
						},
						MarkdownDescription: "Span tag rules keyed by tag name, for example `tags.http.method`.",
					},
				},
				MarkdownDescription: "Span source-type matching rules. Exactly one of `log_rules` or `span_rules` must be set.",
			},
			"quota_based_priority_override": quotaBasedPriorityOverrideNestedAttribute("Dynamically reassign the quota policy priority based on daily quota consumption tiers."),
			"targets": schema.ListNestedAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.List{
					quotaRuleTargetsRemovedPlanModifier{},
					listplanmodifier.UseStateForUnknown(),
				},
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"dataset": schema.StringAttribute{
							Required: true,
							Validators: []validator.String{
								stringvalidator.LengthAtLeast(1),
							},
							MarkdownDescription: "Target dataset.",
						},
						"dataspace": schema.StringAttribute{
							Optional: true,
							Computed: true,
							Default:  stringdefault.StaticString(quotaRuleDefaultDataspace),
							Validators: []validator.String{
								stringvalidator.RegexMatches(quotaRuleTargetDataspaceRegexp, "dataspace must start with a letter and contain only letters, numbers, underscores, or dots between segments"),
							},
							MarkdownDescription: "Target dataspace.",
						},
						"priority": schema.StringAttribute{
							Optional: true,
							Validators: []validator.String{
								stringvalidator.OneOf(tcoPoliciesValidPriorities...),
							},
							MarkdownDescription: fmt.Sprintf("Target priority override. Valid values are %q.", tcoPoliciesValidPriorities),
						},
						"archive_retention_id": schema.StringAttribute{
							Optional: true,
							Validators: []validator.String{
								stringvalidator.LengthAtLeast(1),
							},
							MarkdownDescription: "Archive retention ID applied to this target.",
						},
						"quota_based_priority_override": quotaBasedPriorityOverrideNestedAttribute("Dynamically reassign this target priority based on daily quota consumption tiers."),
					},
				},
				MarkdownDescription: "Target routing destinations for this quota policy.",
			},
		},
	}
}

func (r *QuotaRuleResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data QuotaRuleModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(validateQuotaRuleModel(ctx, data)...)
}

func (r *QuotaRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan QuotaRuleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	request, diags := expandQuotaRuleCreate(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	result, httpResponse, err := r.client.
		PoliciesServiceCreatePolicy(ctx).
		PoliciesServiceCreatePolicyRequest(request).
		Execute()
	if err != nil {
		resp.Diagnostics.AddError("Error creating coralogix_quota_rule",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Create", request),
		)
		return
	}

	state, diags := flattenCreateQuotaRuleResponse(ctx, result)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *QuotaRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state QuotaRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, httpResponse, err := r.client.PoliciesServiceGetPolicy(ctx, state.ID.ValueString()).Execute()
	if err != nil {
		if responseStatus(httpResponse) == http.StatusNotFound {
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError("Error reading coralogix_quota_rule",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Read", state.ID.ValueString()),
		)
		return
	}

	newState, diags := flattenGetQuotaRuleResponse(ctx, result)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, newState)...)
}

func (r *QuotaRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan QuotaRuleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var priorState QuotaRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &priorState)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var config QuotaRuleModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if plan.ID.IsNull() || plan.ID.IsUnknown() || plan.ID.ValueString() == "" {
		plan.ID = priorState.ID
	}
	if plan.Order.IsNull() || plan.Order.IsUnknown() {
		plan.Order = priorState.Order
	}
	normalizeQuotaRuleUpdatePlanFromConfig(&plan, config)
	if !plan.Targets.IsNull() && !plan.Targets.IsUnknown() {
		plan.Priority = types.StringNull()
	}

	request, diags := expandQuotaRuleUpdate(ctx, plan, priorState)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	result, httpResponse, err := r.client.
		PoliciesServiceUpdatePolicy(ctx).
		PoliciesServiceUpdatePolicyRequest(request).
		Execute()
	if err != nil {
		if responseStatus(httpResponse) == http.StatusNotFound {
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError("Error updating coralogix_quota_rule",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Update", request),
		)
		return
	}

	state, diags := flattenUpdateQuotaRuleResponse(ctx, result)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *QuotaRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state QuotaRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, httpResponse, err := r.client.PoliciesServiceDeletePolicy(ctx, state.ID.ValueString()).Execute()
	if err != nil {
		if responseStatus(httpResponse) == http.StatusNotFound {
			return
		}

		resp.Diagnostics.AddError("Error deleting coralogix_quota_rule",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Delete", state.ID.ValueString()),
		)
		return
	}
}

func ruleNestedAttribute(description string) schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional:            true,
		Attributes:          quotaRuleRuleSchemaAttributes(),
		MarkdownDescription: description,
	}
}

func quotaRuleRuleSchemaAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"names": schema.SetAttribute{
			Required:    true,
			ElementType: types.StringType,
			Validators: []validator.Set{
				setvalidator.SizeAtLeast(1),
				setvalidator.ValueStringsAre(stringvalidator.LengthAtLeast(1)),
			},
		},
		"rule_type": schema.StringAttribute{
			Optional: true,
			Computed: true,
			Default:  stringdefault.StaticString("is"),
			Validators: []validator.String{
				stringvalidator.OneOf(tcoPoliciesValidRuleTypes...),
			},
			MarkdownDescription: fmt.Sprintf("Rule type. Valid values are %q.", tcoPoliciesValidRuleTypes),
		},
	}
}

func quotaBasedPriorityOverrideNestedAttribute(description string) schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional:            true,
		MarkdownDescription: description,
		Attributes: map[string]schema.Attribute{
			"usage_tiers": schema.ListNestedAttribute{
				Required: true,
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"daily_quota_percentage": schema.Float64Attribute{
							Required: true,
							Validators: []validator.Float64{
								float64validator.Between(0, 100),
							},
							MarkdownDescription: "Daily quota consumption percentage at which this tier becomes active.",
						},
						"priority": schema.StringAttribute{
							Required: true,
							Validators: []validator.String{
								stringvalidator.OneOf(tcoPoliciesValidPriorities...),
							},
							MarkdownDescription: fmt.Sprintf("Priority to apply when this tier is active. Valid values are %q.", tcoPoliciesValidPriorities),
						},
					},
				},
			},
		},
	}
}

func validateQuotaRuleModel(ctx context.Context, data QuotaRuleModel) diag.Diagnostics {
	var diags diag.Diagnostics

	hasLogRules := !data.LogRules.IsNull() && !data.LogRules.IsUnknown()
	hasSpanRules := !data.SpanRules.IsNull() && !data.SpanRules.IsUnknown()
	if hasLogRules == hasSpanRules {
		diags.AddAttributeError(
			path.Root("log_rules"),
			"Invalid quota rule source-type configuration",
			"Exactly one of log_rules or span_rules must be configured.",
		)
		return diags
	}

	if hasLogRules {
		var logRules QuotaRuleLogRulesModel
		diags.Append(data.LogRules.As(ctx, &logRules, basetypes.ObjectAsOptions{})...)
		if diags.HasError() {
			return diags
		}

		hasSeverities := !logRules.Severities.IsNull() && !logRules.Severities.IsUnknown()
		hasDpxlExpression := !logRules.DpxlExpression.IsNull() && !logRules.DpxlExpression.IsUnknown()
		if hasSeverities == hasDpxlExpression {
			diags.AddAttributeError(
				path.Root("log_rules"),
				"Invalid log quota rule matcher",
				"Exactly one of log_rules.severities or log_rules.dpxl_expression must be configured.",
			)
		}
	}

	targetsConfigured := !data.Targets.IsNull() && !data.Targets.IsUnknown()
	priorityConfigured := !data.Priority.IsNull() && !data.Priority.IsUnknown()
	if targetsConfigured && priorityConfigured {
		diags.AddAttributeError(
			path.Root("priority"),
			"Invalid quota rule priority configuration",
			"Do not set policy-level priority when targets are configured. Set priority inside each target instead.",
		)
	}
	if !targetsConfigured && !priorityConfigured {
		diags.AddAttributeError(
			path.Root("priority"),
			"Missing quota rule priority",
			"Set policy-level priority when targets are not configured.",
		)
	}

	return diags
}

func normalizeQuotaRuleUpdatePlanFromConfig(plan *QuotaRuleModel, config QuotaRuleModel) {
	targetsRemoved := config.Targets.IsNull()
	priorityConfigured := !config.Priority.IsNull() && !config.Priority.IsUnknown()
	if targetsRemoved && priorityConfigured {
		plan.Targets = types.ListNull(types.ObjectType{AttrTypes: quotaRuleTargetAttributes()})
	}
}

type quotaRuleTargetsRemovedPlanModifier struct{}

func (m quotaRuleTargetsRemovedPlanModifier) Description(context.Context) string {
	return "Allows target routing to be removed when policy-level priority is configured."
}

func (m quotaRuleTargetsRemovedPlanModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m quotaRuleTargetsRemovedPlanModifier) PlanModifyList(ctx context.Context, req planmodifier.ListRequest, resp *planmodifier.ListResponse) {
	if !req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	var priority types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("priority"), &priority)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if priority.IsNull() || priority.IsUnknown() {
		return
	}

	resp.PlanValue = types.ListNull(types.ObjectType{AttrTypes: quotaRuleTargetAttributes()})
}

func expandQuotaRuleCreate(ctx context.Context, plan QuotaRuleModel) (tcoPolicys.PoliciesServiceCreatePolicyRequest, diag.Diagnostics) {
	logRulesConfigured := !plan.LogRules.IsNull() && !plan.LogRules.IsUnknown()
	if logRulesConfigured {
		logRules, diags := expandQuotaRuleLogRules(ctx, plan.LogRules)
		if diags.HasError() {
			return tcoPolicys.PoliciesServiceCreatePolicyRequest{}, diags
		}

		request, diags := expandQuotaRuleCreateLog(ctx, plan, logRules)
		if diags.HasError() {
			return tcoPolicys.PoliciesServiceCreatePolicyRequest{}, diags
		}
		return tcoPolicys.CreatePolicyRequestLogRulesAsPoliciesServiceCreatePolicyRequest(request), nil
	}

	spanRules, diags := expandQuotaRuleSpanRules(ctx, plan.SpanRules)
	if diags.HasError() {
		return tcoPolicys.PoliciesServiceCreatePolicyRequest{}, diags
	}

	request, diags := expandQuotaRuleCreateSpan(ctx, plan, spanRules)
	if diags.HasError() {
		return tcoPolicys.PoliciesServiceCreatePolicyRequest{}, diags
	}
	return tcoPolicys.CreatePolicyRequestSpanRulesAsPoliciesServiceCreatePolicyRequest(request), nil
}

func expandQuotaRuleUpdate(ctx context.Context, plan QuotaRuleModel, priorState QuotaRuleModel) (tcoPolicys.PoliciesServiceUpdatePolicyRequest, diag.Diagnostics) {
	logRulesConfigured := !plan.LogRules.IsNull() && !plan.LogRules.IsUnknown()
	if logRulesConfigured {
		logRules, diags := expandQuotaRuleLogRules(ctx, plan.LogRules)
		if diags.HasError() {
			return tcoPolicys.PoliciesServiceUpdatePolicyRequest{}, diags
		}

		request, diags := expandQuotaRuleUpdateLog(ctx, plan, priorState, logRules)
		if diags.HasError() {
			return tcoPolicys.PoliciesServiceUpdatePolicyRequest{}, diags
		}
		return tcoPolicys.UpdatePolicyRequestLogRulesAsPoliciesServiceUpdatePolicyRequest(request), nil
	}

	spanRules, diags := expandQuotaRuleSpanRules(ctx, plan.SpanRules)
	if diags.HasError() {
		return tcoPolicys.PoliciesServiceUpdatePolicyRequest{}, diags
	}

	request, diags := expandQuotaRuleUpdateSpan(ctx, plan, priorState, spanRules)
	if diags.HasError() {
		return tcoPolicys.PoliciesServiceUpdatePolicyRequest{}, diags
	}
	return tcoPolicys.UpdatePolicyRequestSpanRulesAsPoliciesServiceUpdatePolicyRequest(request), nil
}

func expandQuotaRuleCreateLog(ctx context.Context, plan QuotaRuleModel, logRules tcoPolicys.LogRules) (*tcoPolicys.CreatePolicyRequestLogRules, diag.Diagnostics) {
	applicationRule, subsystemRule, archiveRetention, priorityOverride, targets, diags := expandQuotaRuleCommon(ctx, plan)
	if diags.HasError() {
		return nil, diags
	}
	priority, diags := expandQuotaRulePolicyPriority(plan)
	if diags.HasError() {
		return nil, diags
	}

	disabled := !plan.Enabled.ValueBool()
	request := tcoPolicys.NewCreatePolicyRequestLogRules(logRules, plan.Name.ValueString(), priority)
	request.Description = plan.Description.ValueStringPointer()
	request.Disabled = &disabled
	request.ApplicationRule = applicationRule
	request.SubsystemRule = subsystemRule
	request.ArchiveRetention = archiveRetention
	request.PriorityOverride = priorityOverride
	request.Targets = targets
	return request, nil
}

func expandQuotaRuleCreateSpan(ctx context.Context, plan QuotaRuleModel, spanRules tcoPolicys.SpanRules) (*tcoPolicys.CreatePolicyRequestSpanRules, diag.Diagnostics) {
	applicationRule, subsystemRule, archiveRetention, priorityOverride, targets, diags := expandQuotaRuleCommon(ctx, plan)
	if diags.HasError() {
		return nil, diags
	}
	priority, diags := expandQuotaRulePolicyPriority(plan)
	if diags.HasError() {
		return nil, diags
	}

	disabled := !plan.Enabled.ValueBool()
	request := tcoPolicys.NewCreatePolicyRequestSpanRules(plan.Name.ValueString(), priority, spanRules)
	request.Description = plan.Description.ValueStringPointer()
	request.Disabled = &disabled
	request.ApplicationRule = applicationRule
	request.SubsystemRule = subsystemRule
	request.ArchiveRetention = archiveRetention
	request.PriorityOverride = priorityOverride
	request.Targets = targets
	return request, nil
}

func expandQuotaRuleUpdateLog(ctx context.Context, plan QuotaRuleModel, priorState QuotaRuleModel, logRules tcoPolicys.LogRules) (*tcoPolicys.UpdatePolicyRequestLogRules, diag.Diagnostics) {
	applicationRule, subsystemRule, archiveRetention, priorityOverride, targets, diags := expandQuotaRuleCommon(ctx, plan)
	if diags.HasError() {
		return nil, diags
	}

	priority, diags := expandQuotaRulePolicyPriority(plan)
	if diags.HasError() {
		return nil, diags
	}
	request := tcoPolicys.NewUpdatePolicyRequestLogRules(plan.ID.ValueString(), logRules)
	request.Name = plan.Name.ValueStringPointer()
	request.Description = plan.Description.ValueStringPointer()
	request.Enabled = plan.Enabled.ValueBoolPointer()
	request.Priority = &priority
	request.ApplicationRule = quotaRuleUpdateRule(plan.ApplicationRule, priorState.ApplicationRule, applicationRule)
	request.SubsystemRule = quotaRuleUpdateRule(plan.SubsystemRule, priorState.SubsystemRule, subsystemRule)
	request.ArchiveRetention = quotaRuleUpdateArchiveRetention(plan.ArchiveRetentionID, priorState.ArchiveRetentionID, archiveRetention)
	request.PriorityOverride = quotaRuleUpdatePriorityOverride(plan, priorityOverride)
	request.Targets = quotaRuleUpdateTargets(plan, targets)
	return request, nil
}

func expandQuotaRuleUpdateSpan(ctx context.Context, plan QuotaRuleModel, priorState QuotaRuleModel, spanRules tcoPolicys.SpanRules) (*tcoPolicys.UpdatePolicyRequestSpanRules, diag.Diagnostics) {
	applicationRule, subsystemRule, archiveRetention, priorityOverride, targets, diags := expandQuotaRuleCommon(ctx, plan)
	if diags.HasError() {
		return nil, diags
	}

	priority, diags := expandQuotaRulePolicyPriority(plan)
	if diags.HasError() {
		return nil, diags
	}
	request := tcoPolicys.NewUpdatePolicyRequestSpanRules(plan.ID.ValueString(), spanRules)
	request.Name = plan.Name.ValueStringPointer()
	request.Description = plan.Description.ValueStringPointer()
	request.Enabled = plan.Enabled.ValueBoolPointer()
	request.Priority = &priority
	request.ApplicationRule = quotaRuleUpdateRule(plan.ApplicationRule, priorState.ApplicationRule, applicationRule)
	request.SubsystemRule = quotaRuleUpdateRule(plan.SubsystemRule, priorState.SubsystemRule, subsystemRule)
	request.ArchiveRetention = quotaRuleUpdateArchiveRetention(plan.ArchiveRetentionID, priorState.ArchiveRetentionID, archiveRetention)
	request.PriorityOverride = quotaRuleUpdatePriorityOverride(plan, priorityOverride)
	request.Targets = quotaRuleUpdateTargets(plan, targets)
	return request, nil
}

func expandQuotaRuleCommon(ctx context.Context, plan QuotaRuleModel) (*tcoPolicys.QuotaV1Rule, *tcoPolicys.QuotaV1Rule, *tcoPolicys.ArchiveRetention, *tcoPolicys.PriorityOverride, []tcoPolicys.V1Target, diag.Diagnostics) {
	var diags diag.Diagnostics

	applicationRule, dgs := expandTCOPolicyRule(ctx, plan.ApplicationRule)
	if dgs.HasError() {
		diags.Append(dgs...)
	}
	subsystemRule, dgs := expandTCOPolicyRule(ctx, plan.SubsystemRule)
	if dgs.HasError() {
		diags.Append(dgs...)
	}
	priorityOverride, dgs := expandQuotaBasedPriorityOverride(ctx, plan.QuotaBasedPriorityOverride)
	if dgs.HasError() {
		diags.Append(dgs...)
	}
	targets, dgs := expandQuotaRuleTargets(ctx, plan.Targets)
	if dgs.HasError() {
		diags.Append(dgs...)
	}
	if diags.HasError() {
		return nil, nil, nil, nil, nil, diags
	}

	return applicationRule, subsystemRule, expandActiveRetention(plan.ArchiveRetentionID), priorityOverride, targets, diags
}

func quotaRuleUpdatePriorityOverride(plan QuotaRuleModel, priorityOverride *tcoPolicys.PriorityOverride) *tcoPolicys.PriorityOverride {
	targetsConfigured := !plan.Targets.IsNull() && !plan.Targets.IsUnknown()
	if !targetsConfigured && (plan.QuotaBasedPriorityOverride.IsNull() || priorityOverride == nil) {
		return tcoPolicys.NewPriorityOverride()
	}
	return priorityOverride
}

func quotaRuleUpdateRule(planRule types.Object, priorRule types.Object, rule *tcoPolicys.QuotaV1Rule) *tcoPolicys.QuotaV1Rule {
	if planRule.IsNull() && !priorRule.IsNull() && !priorRule.IsUnknown() {
		return tcoPolicys.NewQuotaV1Rule()
	}
	return rule
}

func quotaRuleUpdateArchiveRetention(planArchiveRetention types.String, priorArchiveRetention types.String, archiveRetention *tcoPolicys.ArchiveRetention) *tcoPolicys.ArchiveRetention {
	if planArchiveRetention.IsNull() && !priorArchiveRetention.IsNull() && !priorArchiveRetention.IsUnknown() {
		return tcoPolicys.NewArchiveRetention()
	}
	return archiveRetention
}

func quotaRuleUpdateTargets(plan QuotaRuleModel, targets []tcoPolicys.V1Target) []tcoPolicys.V1Target {
	if plan.Targets.IsNull() {
		return []tcoPolicys.V1Target{}
	}
	return targets
}

func expandQuotaRulePolicyPriority(plan QuotaRuleModel) (tcoPolicys.QuotaV1Priority, diag.Diagnostics) {
	var diags diag.Diagnostics
	targetsConfigured := !plan.Targets.IsNull() && !plan.Targets.IsUnknown()
	if targetsConfigured {
		return tcoPolicys.QUOTAV1PRIORITY_PRIORITY_TYPE_UNSPECIFIED, diags
	}
	if plan.Priority.IsNull() || plan.Priority.IsUnknown() {
		diags.AddAttributeError(
			path.Root("priority"),
			"Missing quota rule priority",
			"Set policy-level priority when targets are not configured.",
		)
		return tcoPolicys.QUOTAV1PRIORITY_PRIORITY_TYPE_UNSPECIFIED, diags
	}
	return tcoPoliciesPrioritySchemaToApi[plan.Priority.ValueString()], diags
}

func expandQuotaRuleLogRules(ctx context.Context, logRulesObject types.Object) (tcoPolicys.LogRules, diag.Diagnostics) {
	var model QuotaRuleLogRulesModel
	diags := logRulesObject.As(ctx, &model, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return tcoPolicys.LogRules{}, diags
	}

	logRules := tcoPolicys.LogRules{}
	if !model.DpxlExpression.IsNull() && !model.DpxlExpression.IsUnknown() {
		logRules.DpxlExpression = model.DpxlExpression.ValueStringPointer()
		return logRules, diags
	}

	if !model.Severities.IsNull() && !model.Severities.IsUnknown() {
		severities, dgs := expandTCOPolicySeverities(ctx, model.Severities.Elements())
		if dgs.HasError() {
			diags.Append(dgs...)
			return tcoPolicys.LogRules{}, diags
		}
		logRules.Severities = severities
	}
	return logRules, diags
}

func expandQuotaRuleSpanRules(ctx context.Context, spanRulesObject types.Object) (tcoPolicys.SpanRules, diag.Diagnostics) {
	var model QuotaRuleSpanRulesModel
	diags := spanRulesObject.As(ctx, &model, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return tcoPolicys.SpanRules{}, diags
	}

	serviceRule, dgs := expandTCOPolicyRule(ctx, model.ServiceRule)
	if dgs.HasError() {
		diags.Append(dgs...)
	}
	actionRule, dgs := expandTCOPolicyRule(ctx, model.ActionRule)
	if dgs.HasError() {
		diags.Append(dgs...)
	}
	tagRules, dgs := expandQuotaRuleTagRules(ctx, model.TagRules)
	if dgs.HasError() {
		diags.Append(dgs...)
	}
	if diags.HasError() {
		return tcoPolicys.SpanRules{}, diags
	}

	return tcoPolicys.SpanRules{
		ServiceRule: serviceRule,
		ActionRule:  actionRule,
		TagRules:    tagRules,
	}, nil
}

func expandQuotaRuleTagRules(ctx context.Context, tags types.Map) ([]tcoPolicys.TagRule, diag.Diagnostics) {
	if tags.IsNull() || tags.IsUnknown() {
		return nil, nil
	}

	var tagsMap map[string]types.Object
	diags := tags.ElementsAs(ctx, &tagsMap, true)
	if diags.HasError() {
		return nil, diags
	}

	result := make([]tcoPolicys.TagRule, 0, len(tagsMap))
	for tagName, tagElement := range tagsMap {
		tagRule, dgs := expandTagRule(ctx, tagName, tagElement)
		if dgs.HasError() {
			diags.Append(dgs...)
			continue
		}
		result = append(result, *tagRule)
	}
	if diags.HasError() {
		return nil, diags
	}
	return result, nil
}

func expandQuotaRuleTargets(ctx context.Context, targets types.List) ([]tcoPolicys.V1Target, diag.Diagnostics) {
	if targets.IsNull() || targets.IsUnknown() {
		return nil, nil
	}

	var targetObjects []types.Object
	diags := targets.ElementsAs(ctx, &targetObjects, true)
	if diags.HasError() {
		return nil, diags
	}

	result := make([]tcoPolicys.V1Target, 0, len(targetObjects))
	for _, targetObject := range targetObjects {
		var targetModel QuotaRuleTargetModel
		if dgs := targetObject.As(ctx, &targetModel, basetypes.ObjectAsOptions{}); dgs.HasError() {
			diags.Append(dgs...)
			continue
		}

		target := tcoPolicys.V1Target{
			Dataset:          targetModel.Dataset.ValueStringPointer(),
			Dataspace:        targetModel.Dataspace.ValueStringPointer(),
			ArchiveRetention: expandActiveRetention(targetModel.ArchiveRetentionID),
		}
		if targetModel.Dataspace.IsNull() || targetModel.Dataspace.IsUnknown() {
			dataspace := quotaRuleDefaultDataspace
			target.Dataspace = &dataspace
		}
		if !targetModel.Priority.IsNull() && !targetModel.Priority.IsUnknown() {
			priority := tcoPoliciesPrioritySchemaToApi[targetModel.Priority.ValueString()]
			target.Priority = &priority
		}
		priorityOverride, dgs := expandQuotaBasedPriorityOverride(ctx, targetModel.QuotaBasedPriorityOverride)
		if dgs.HasError() {
			diags.Append(dgs...)
			continue
		}
		target.PriorityOverride = priorityOverride
		result = append(result, target)
	}
	if diags.HasError() {
		return nil, diags
	}
	return result, nil
}

func flattenCreateQuotaRuleResponse(ctx context.Context, resp *tcoPolicys.CreatePolicyResponse) (*QuotaRuleModel, diag.Diagnostics) {
	if resp == nil {
		var diags diag.Diagnostics
		diags.AddError("Empty create quota rule response", "Coralogix returned an empty response when creating the quota rule.")
		return nil, diags
	}
	return flattenQuotaRulePolicy(ctx, &resp.Policy)
}

func flattenGetQuotaRuleResponse(ctx context.Context, resp *tcoPolicys.GetPolicyResponse) (*QuotaRuleModel, diag.Diagnostics) {
	if resp == nil || resp.Policy == nil {
		var diags diag.Diagnostics
		diags.AddError("Empty get quota rule response", "Coralogix returned an empty response when reading the quota rule.")
		return nil, diags
	}
	return flattenQuotaRulePolicy(ctx, resp.Policy)
}

func flattenUpdateQuotaRuleResponse(ctx context.Context, resp *tcoPolicys.UpdatePolicyResponse) (*QuotaRuleModel, diag.Diagnostics) {
	if resp == nil {
		var diags diag.Diagnostics
		diags.AddError("Empty update quota rule response", "Coralogix returned an empty response when updating the quota rule.")
		return nil, diags
	}
	return flattenQuotaRulePolicy(ctx, &resp.Policy)
}

func flattenQuotaRulePolicy(ctx context.Context, policy *tcoPolicys.Policy) (*QuotaRuleModel, diag.Diagnostics) {
	if policy == nil {
		var diags diag.Diagnostics
		diags.AddError("Empty quota rule policy", "Coralogix returned an empty quota rule policy.")
		return nil, diags
	}

	switch {
	case policy.PolicyLogRules != nil:
		return flattenQuotaRuleLogPolicy(ctx, policy.PolicyLogRules)
	case policy.PolicySpanRules != nil:
		return flattenQuotaRuleSpanPolicy(ctx, policy.PolicySpanRules)
	default:
		var diags diag.Diagnostics
		diags.AddError("Unsupported quota rule source type", "Coralogix returned a quota rule without log_rules or span_rules.")
		return nil, diags
	}
}

func flattenQuotaRuleLogPolicy(ctx context.Context, policy *tcoPolicys.PolicyLogRules) (*QuotaRuleModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	applicationRule, dgs := flattenTCOPolicyRule(ctx, policy.ApplicationRule)
	if dgs.HasError() {
		diags.Append(dgs...)
	}
	subsystemRule, dgs := flattenTCOPolicyRule(ctx, policy.SubsystemRule)
	if dgs.HasError() {
		diags.Append(dgs...)
	}
	priorityOverride, dgs := flattenQuotaBasedPriorityOverride(ctx, policy.PriorityOverride)
	if dgs.HasError() {
		diags.Append(dgs...)
	}
	targets, dgs := flattenQuotaRuleTargetsForPolicy(ctx, policy.GetPriority(), policy.GetTargets())
	if dgs.HasError() {
		diags.Append(dgs...)
	}
	logRules, dgs := flattenQuotaRuleLogRules(ctx, policy.LogRules)
	if dgs.HasError() {
		diags.Append(dgs...)
	}
	if diags.HasError() {
		return nil, diags
	}

	return &QuotaRuleModel{
		ID:                         types.StringValue(policy.GetId()),
		Name:                       types.StringValue(policy.GetName()),
		Description:                types.StringValue(policy.GetDescription()),
		Enabled:                    types.BoolValue(policy.GetEnabled()),
		Priority:                   flattenQuotaRulePriority(policy.GetPriority()),
		Order:                      types.Int64Value(int64(policy.GetOrder())),
		ApplicationRule:            applicationRule,
		SubsystemRule:              subsystemRule,
		ArchiveRetentionID:         flattenArchiveRetention(policy.ArchiveRetention),
		LogRules:                   logRules,
		SpanRules:                  types.ObjectNull(quotaRuleSpanRulesAttributes()),
		QuotaBasedPriorityOverride: priorityOverride,
		Targets:                    targets,
	}, nil
}

func flattenQuotaRuleSpanPolicy(ctx context.Context, policy *tcoPolicys.PolicySpanRules) (*QuotaRuleModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	applicationRule, dgs := flattenTCOPolicyRule(ctx, policy.ApplicationRule)
	if dgs.HasError() {
		diags.Append(dgs...)
	}
	subsystemRule, dgs := flattenTCOPolicyRule(ctx, policy.SubsystemRule)
	if dgs.HasError() {
		diags.Append(dgs...)
	}
	priorityOverride, dgs := flattenQuotaBasedPriorityOverride(ctx, policy.PriorityOverride)
	if dgs.HasError() {
		diags.Append(dgs...)
	}
	targets, dgs := flattenQuotaRuleTargetsForPolicy(ctx, policy.GetPriority(), policy.GetTargets())
	if dgs.HasError() {
		diags.Append(dgs...)
	}
	spanRules, dgs := flattenQuotaRuleSpanRules(ctx, policy.SpanRules)
	if dgs.HasError() {
		diags.Append(dgs...)
	}
	if diags.HasError() {
		return nil, diags
	}

	return &QuotaRuleModel{
		ID:                         types.StringValue(policy.GetId()),
		Name:                       types.StringValue(policy.GetName()),
		Description:                types.StringValue(policy.GetDescription()),
		Enabled:                    types.BoolValue(policy.GetEnabled()),
		Priority:                   flattenQuotaRulePriority(policy.GetPriority()),
		Order:                      types.Int64Value(int64(policy.GetOrder())),
		ApplicationRule:            applicationRule,
		SubsystemRule:              subsystemRule,
		ArchiveRetentionID:         flattenArchiveRetention(policy.ArchiveRetention),
		LogRules:                   types.ObjectNull(quotaRuleLogRulesAttributes()),
		SpanRules:                  spanRules,
		QuotaBasedPriorityOverride: priorityOverride,
		Targets:                    targets,
	}, nil
}

func flattenQuotaRulePriority(priority tcoPolicys.QuotaV1Priority) types.String {
	if priority == tcoPolicys.QUOTAV1PRIORITY_PRIORITY_TYPE_UNSPECIFIED {
		return types.StringNull()
	}
	if value, ok := tcoPoliciesPriorityApiToSchema[priority]; ok {
		return types.StringValue(value)
	}
	value := strings.TrimPrefix(strings.ToLower(string(priority)), "priority_type_")
	if value == "" {
		return types.StringNull()
	}
	return types.StringValue(value)
}

func flattenQuotaRuleLogRules(ctx context.Context, logRules tcoPolicys.LogRules) (types.Object, diag.Diagnostics) {
	return types.ObjectValueFrom(ctx, quotaRuleLogRulesAttributes(), QuotaRuleLogRulesModel{
		Severities:     flattenTCOPolicySeverities(logRules.GetSeverities()),
		DpxlExpression: types.StringPointerValue(logRules.DpxlExpression),
	})
}

func flattenQuotaRuleTargetsForPolicy(ctx context.Context, policyPriority tcoPolicys.QuotaV1Priority, targets []tcoPolicys.V1Target) (types.List, diag.Diagnostics) {
	if policyPriority != tcoPolicys.QUOTAV1PRIORITY_PRIORITY_TYPE_UNSPECIFIED {
		return types.ListNull(types.ObjectType{AttrTypes: quotaRuleTargetAttributes()}), nil
	}

	return flattenQuotaRuleTargets(ctx, targets)
}

func flattenQuotaRuleSpanRules(ctx context.Context, spanRules tcoPolicys.SpanRules) (types.Object, diag.Diagnostics) {
	serviceRule, diags := flattenTCOPolicyRule(ctx, spanRules.ServiceRule)
	if diags.HasError() {
		return types.ObjectNull(quotaRuleSpanRulesAttributes()), diags
	}
	actionRule, diags := flattenTCOPolicyRule(ctx, spanRules.ActionRule)
	if diags.HasError() {
		return types.ObjectNull(quotaRuleSpanRulesAttributes()), diags
	}

	return types.ObjectValueFrom(ctx, quotaRuleSpanRulesAttributes(), QuotaRuleSpanRulesModel{
		ServiceRule: serviceRule,
		ActionRule:  actionRule,
		TagRules:    flattenTCOPolicyTags(ctx, spanRules.TagRules),
	})
}

func flattenQuotaRuleTargets(ctx context.Context, targets []tcoPolicys.V1Target) (types.List, diag.Diagnostics) {
	if len(targets) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: quotaRuleTargetAttributes()}), nil
	}

	var diags diag.Diagnostics
	targetModels := make([]QuotaRuleTargetModel, 0, len(targets))
	for _, target := range targets {
		priority := types.StringNull()
		if target.Priority != nil {
			priority = flattenQuotaRulePriority(*target.Priority)
		}
		dataspace := types.StringValue(quotaRuleDefaultDataspace)
		if target.Dataspace != nil {
			dataspace = types.StringValue(*target.Dataspace)
		}
		priorityOverride, dgs := flattenQuotaBasedPriorityOverride(ctx, target.PriorityOverride)
		if dgs.HasError() {
			diags.Append(dgs...)
			continue
		}
		targetModels = append(targetModels, QuotaRuleTargetModel{
			Dataset:                    types.StringPointerValue(target.Dataset),
			Dataspace:                  dataspace,
			Priority:                   priority,
			ArchiveRetentionID:         flattenArchiveRetention(target.ArchiveRetention),
			QuotaBasedPriorityOverride: priorityOverride,
		})
	}
	if diags.HasError() {
		return types.ListNull(types.ObjectType{AttrTypes: quotaRuleTargetAttributes()}), diags
	}
	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: quotaRuleTargetAttributes()}, targetModels)
}

func quotaRuleAttributes() map[string]attr.Type {
	return map[string]attr.Type{
		"id":                            types.StringType,
		"name":                          types.StringType,
		"description":                   types.StringType,
		"enabled":                       types.BoolType,
		"priority":                      types.StringType,
		"order":                         types.Int64Type,
		"application_rule":              types.ObjectType{AttrTypes: tcoPolicyRuleAttributes()},
		"subsystem_rule":                types.ObjectType{AttrTypes: tcoPolicyRuleAttributes()},
		"archive_retention_id":          types.StringType,
		"log_rules":                     types.ObjectType{AttrTypes: quotaRuleLogRulesAttributes()},
		"span_rules":                    types.ObjectType{AttrTypes: quotaRuleSpanRulesAttributes()},
		"quota_based_priority_override": types.ObjectType{AttrTypes: quotaBasedPriorityOverrideAttributes()},
		"targets":                       types.ListType{ElemType: types.ObjectType{AttrTypes: quotaRuleTargetAttributes()}},
	}
}

func quotaRuleLogRulesAttributes() map[string]attr.Type {
	return map[string]attr.Type{
		"severities":      types.SetType{ElemType: types.StringType},
		"dpxl_expression": types.StringType,
	}
}

func quotaRuleSpanRulesAttributes() map[string]attr.Type {
	return map[string]attr.Type{
		"service_rule": types.ObjectType{AttrTypes: tcoPolicyRuleAttributes()},
		"action_rule":  types.ObjectType{AttrTypes: tcoPolicyRuleAttributes()},
		"tag_rules":    types.MapType{ElemType: types.ObjectType{AttrTypes: tcoPolicyRuleAttributes()}},
	}
}

func quotaRuleTargetAttributes() map[string]attr.Type {
	return map[string]attr.Type{
		"dataset":                       types.StringType,
		"dataspace":                     types.StringType,
		"priority":                      types.StringType,
		"archive_retention_id":          types.StringType,
		"quota_based_priority_override": types.ObjectType{AttrTypes: quotaBasedPriorityOverrideAttributes()},
	}
}
