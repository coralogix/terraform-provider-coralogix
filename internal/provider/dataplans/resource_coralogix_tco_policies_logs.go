// Copyright 2024 Coralogix Ltd.
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
	"strings"
	"time"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	"regexp"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	tcoPolicys "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/policies_service"

	"github.com/hashicorp/terraform-plugin-framework-validators/float64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

var (
	_                              resource.ResourceWithConfigure      = &TCOPoliciesLogsResource{}
	_                              resource.ResourceWithImportState    = &TCOPoliciesLogsResource{}
	_                              resource.ResourceWithValidateConfig = &TCOPoliciesLogsResource{}
	tcoPoliciesPrioritySchemaToApi                                     = map[string]tcoPolicys.QuotaV1Priority{
		"block":  tcoPolicys.QUOTAV1PRIORITY_PRIORITY_TYPE_BLOCK,
		"high":   tcoPolicys.QUOTAV1PRIORITY_PRIORITY_TYPE_HIGH,
		"low":    tcoPolicys.QUOTAV1PRIORITY_PRIORITY_TYPE_LOW,
		"medium": tcoPolicys.QUOTAV1PRIORITY_PRIORITY_TYPE_MEDIUM,
	}
	tcoPoliciesPriorityApiToSchema = utils.ReverseMap(tcoPoliciesPrioritySchemaToApi)
	tcoPoliciesValidPriorities     = utils.GetKeys(tcoPoliciesPrioritySchemaToApi)
	tcoPoliciesRuleTypeSchemaToApi = map[string]tcoPolicys.RuleTypeId{
		"is":              tcoPolicys.RULETYPEID_RULE_TYPE_ID_IS,
		"is_not":          tcoPolicys.RULETYPEID_RULE_TYPE_ID_IS_NOT,
		"starts_with":     tcoPolicys.RULETYPEID_RULE_TYPE_ID_START_WITH,
		"includes":        tcoPolicys.RULETYPEID_RULE_TYPE_ID_INCLUDES,
		utils.UNSPECIFIED: tcoPolicys.RULETYPEID_RULE_TYPE_ID_UNSPECIFIED,
	}
	tcoPoliciesRuleTypeApiToSchema = utils.ReverseMap(tcoPoliciesRuleTypeSchemaToApi)
	tcoPoliciesValidRuleTypes      = utils.GetKeys(tcoPoliciesRuleTypeSchemaToApi)
	tcoPolicySeveritySchemaToApi   = map[string]tcoPolicys.QuotaV1Severity{
		"debug":    tcoPolicys.QUOTAV1SEVERITY_SEVERITY_DEBUG,
		"verbose":  tcoPolicys.QUOTAV1SEVERITY_SEVERITY_VERBOSE,
		"info":     tcoPolicys.QUOTAV1SEVERITY_SEVERITY_INFO,
		"warning":  tcoPolicys.QUOTAV1SEVERITY_SEVERITY_WARNING,
		"error":    tcoPolicys.QUOTAV1SEVERITY_SEVERITY_ERROR,
		"critical": tcoPolicys.QUOTAV1SEVERITY_SEVERITY_CRITICAL,
	}
	tcoPolicySeverityApiToSchema = utils.ReverseMap(tcoPolicySeveritySchemaToApi)
	validPolicySeverities        = utils.GetKeys(tcoPolicySeveritySchemaToApi)
	// overrideTCOPoliciesLogsURL     = tcoPolicys.TCOPoliciesAtomicOverwriteLogPoliciesRPC
	// getCompanyPoliciesURL          = tcoPolicys.TCOPoliciesGetCompanyPoliciesRPC
	LogSource = tcoPolicys.V1SOURCETYPE_SOURCE_TYPE_LOGS
)

func NewTCOPoliciesLogsResource() resource.Resource {
	return &TCOPoliciesLogsResource{}
}

type TCOPoliciesLogsResource struct {
	client *tcoPolicys.PoliciesServiceAPIService
}

func (r *TCOPoliciesLogsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

type TCOPoliciesListModel struct {
	ID       types.String `tfsdk:"id"`
	Policies types.List   `tfsdk:"policies"` // TCOPolicyLogsModel
}

type TCOPolicyLogsModel struct {
	ID                         types.String `tfsdk:"id"`
	Name                       types.String `tfsdk:"name"`
	Description                types.String `tfsdk:"description"`
	Enabled                    types.Bool   `tfsdk:"enabled"`
	Order                      types.Int64  `tfsdk:"order"`
	Priority                   types.String `tfsdk:"priority"`
	Applications               types.Object `tfsdk:"applications"`
	Subsystems                 types.Object `tfsdk:"subsystems"`
	Severities                 types.Set    `tfsdk:"severities"`
	ArchiveRetentionID         types.String `tfsdk:"archive_retention_id"`
	DpxlExpression             types.String `tfsdk:"dpxl_expression"`
	QuotaBasedPriorityOverride types.Object `tfsdk:"quota_based_priority_override"` // QuotaBasedPriorityOverrideModel
	Targets                    types.List   `tfsdk:"targets"`                       // []TCOTargetModel
}

type TCORuleModel struct {
	RuleType types.String `tfsdk:"rule_type"`
	Names    types.Set    `tfsdk:"names"`
}

type QuotaBasedPriorityOverrideModel struct {
	UsageTiers types.List `tfsdk:"usage_tiers"` // []UsageTierModel
}

type UsageTierModel struct {
	DailyQuotaPercentage types.Float64 `tfsdk:"daily_quota_percentage"`
	Priority             types.String  `tfsdk:"priority"`
}

type TCOTargetModel struct {
	Dataset                    types.String `tfsdk:"dataset"`
	Dataspace                  types.String `tfsdk:"dataspace"`
	Priority                   types.String `tfsdk:"priority"`
	ArchiveRetentionID         types.String `tfsdk:"archive_retention_id"`
	QuotaBasedPriorityOverride types.Object `tfsdk:"quota_based_priority_override"` // QuotaBasedPriorityOverrideModel
}

func (r *TCOPoliciesLogsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tco_policies_logs"
}

func (r *TCOPoliciesLogsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *TCOPoliciesLogsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "This field can be ignored",
			},
			"policies": schema.ListNestedAttribute{
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "tco-policy ID.",
						},
						"name": schema.StringAttribute{
							Required: true,
							Validators: []validator.String{
								stringvalidator.LengthAtLeast(1),
							},
							MarkdownDescription: "tco-policy name.",
						},
						"description": schema.StringAttribute{
							Optional:            true,
							Computed:            true,
							Default:             stringdefault.StaticString(""),
							MarkdownDescription: "The policy description",
						},
						"enabled": schema.BoolAttribute{
							Optional:            true,
							Computed:            true,
							Default:             booldefault.StaticBool(true),
							MarkdownDescription: "Determines weather the policy will be enabled. True by default.",
						},
						"priority": schema.StringAttribute{
							Optional: true,
							Validators: []validator.String{
								stringvalidator.OneOf(tcoPoliciesValidPriorities...),
							},
							MarkdownDescription: fmt.Sprintf("The policy priority. Can be one of %q. Required unless `targets` is set — with `targets`, the priority is configured per-target instead. For a quota-based policy (when `quota_based_priority_override` is set) this is also the fallback priority applied once all `usage_tiers` are exhausted — the equivalent of \"Route the remaining quota to\" in the UI — and must be more restrictive than the last tier's priority (most to least restrictive: `block`, `low`, `medium`, `high`).", tcoPoliciesValidPriorities),
						},
						"order": schema.Int64Attribute{
							Computed:            true,
							MarkdownDescription: "The policy's order between the other policies.",
						},
						"archive_retention_id": schema.StringAttribute{
							Optional:    true,
							Description: "Allowing logs with a specific retention to be tagged.",
							Validators: []validator.String{
								stringvalidator.LengthAtLeast(1),
							},
						},
						"severities": schema.SetAttribute{
							Optional:    true,
							Computed:    true,
							ElementType: types.StringType,
							Validators: []validator.Set{
								setvalidator.SizeAtLeast(1),
								setvalidator.ValueStringsAre(stringvalidator.OneOf(validPolicySeverities...)),
							},
							MarkdownDescription: fmt.Sprintf("The severities to apply the policy on. Valid severities are %q.", validPolicySeverities),
						},
						"applications": schema.SingleNestedAttribute{
							Optional: true,
							Attributes: map[string]schema.Attribute{
								"names": schema.SetAttribute{
									Required:    true,
									ElementType: types.StringType,
									Validators: []validator.Set{
										setvalidator.SizeAtLeast(1),
										setvalidator.ValueStringsAre(stringvalidator.RegexMatches(
											regexp.MustCompile("[^A-Z]+"), "Only lowercase letters are allowed")),
									},
								},
								"rule_type": schema.StringAttribute{
									Optional: true,
									Computed: true,
									Default:  stringdefault.StaticString("is"),
									Validators: []validator.String{
										stringvalidator.OneOf(tcoPoliciesValidRuleTypes...),
									},
									MarkdownDescription: fmt.Sprintf("The rule type. Can be one of %q.", tcoPoliciesValidRuleTypes),
								},
							},
							MarkdownDescription: "The applications to apply the policy on. Applies the policy on all the applications by default.",
						},
						"subsystems": schema.SingleNestedAttribute{
							Optional: true,
							Attributes: map[string]schema.Attribute{
								"names": schema.SetAttribute{
									Required:    true,
									ElementType: types.StringType,
									Validators: []validator.Set{
										setvalidator.SizeAtLeast(1),
										setvalidator.ValueStringsAre(stringvalidator.RegexMatches(
											regexp.MustCompile("[^A-Z]+"), "Only lowercase letters are allowed")),
									},
								},
								"rule_type": schema.StringAttribute{
									Optional: true,
									Computed: true,
									Default:  stringdefault.StaticString("is"),
									Validators: []validator.String{
										stringvalidator.OneOf(tcoPoliciesValidRuleTypes...),
									},
								},
							},
							MarkdownDescription: "The subsystems to apply the policy on. Applies the policy on all the subsystems by default.",
						},
						"dpxl_expression": schema.StringAttribute{
							Optional: true,
							Validators: []validator.String{
								stringvalidator.LengthAtLeast(1),
								stringvalidator.ConflictsWith(path.MatchRelative().AtParent().AtName("severities")),
							},
							MarkdownDescription: "DataPrime expression to match logs for this policy. Mutually exclusive with `severities` — set exactly one. The expression must include a version prefix, e.g. `<v1> $d.severity == 'INFO'`.",
						},
						"quota_based_priority_override": quotaBasedPriorityOverrideSchema(),
						"targets": schema.ListNestedAttribute{
							Optional: true,
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
										MarkdownDescription: "The dataset this target routes to (e.g. `logs`, `audit_logs`). Requires a log-based dataset to exist in the account when routing to anything other than `logs`.",
									},
									"dataspace": schema.StringAttribute{
										Optional:            true,
										Computed:            true,
										Default:             stringdefault.StaticString("default"),
										MarkdownDescription: "The dataspace of the target dataset. Defaults to `default`.",
									},
									"priority": schema.StringAttribute{
										Optional: true,
										Validators: []validator.String{
											stringvalidator.OneOf(tcoPoliciesValidPriorities...),
										},
										MarkdownDescription: fmt.Sprintf("The priority to apply for logs routed to this target. Can be one of %q. Set the priority per-target instead of at the policy level when using `targets`.", tcoPoliciesValidPriorities),
									},
									"archive_retention_id": schema.StringAttribute{
										Optional:    true,
										Description: "Allowing logs routed to this target with a specific retention to be tagged.",
										Validators: []validator.String{
											stringvalidator.LengthAtLeast(1),
										},
									},
									"quota_based_priority_override": quotaBasedPriorityOverrideSchema(),
								},
							},
							MarkdownDescription: "Route matching logs to one or more targets (a `dataspace`/`dataset` pair), each with its own `priority`, archive retention, and quota-based override. When set, the priority is configured per-target and the policy-level `priority`/`quota_based_priority_override` must not be set. Omit `targets` to keep the legacy single-target (`logs`) behavior driven by the policy-level `priority`.",
						},
					},
				},
				Required: true,
			},
		},
		MarkdownDescription: "Coralogix TCO-Policies-List. For more information - https://coralogix.com/docs/tco-optimizer-api.",
	}
}

func (r *TCOPoliciesLogsResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data TCOPoliciesListModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var policiesObjects []types.Object
	diags := data.Policies.ElementsAs(ctx, &policiesObjects, true)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
	}

	for _, po := range policiesObjects {
		var tcoPolicy TCOPolicyLogsModel
		if dg := po.As(ctx, &tcoPolicy, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		validateTCOPoliciesLogs(tcoPolicy.Subsystems, "subsystems", resp)
		validateTCOPoliciesLogs(tcoPolicy.Applications, "applications", resp)
		validateTCOTargets(ctx, tcoPolicy, resp)
	}
}

func validateTCOTargets(ctx context.Context, policy TCOPolicyLogsModel, resp *resource.ValidateConfigResponse) {
	topLevelPrioritySet := !policy.Priority.IsNull() && !policy.Priority.IsUnknown()
	topLevelOverrideSet := !utils.ObjIsNullOrUnknown(policy.QuotaBasedPriorityOverride)
	topLevelArchiveSet := !policy.ArchiveRetentionID.IsNull() && !policy.ArchiveRetentionID.IsUnknown()

	if policy.Targets.IsNull() || policy.Targets.IsUnknown() {
		if !topLevelPrioritySet {
			resp.Diagnostics.AddAttributeError(
				path.Root("policies"),
				"Missing Priority",
				"`priority` is required when `targets` is not set (including as the fallback priority when `quota_based_priority_override` is set).",
			)
		}
		return
	}

	var targetObjects []types.Object
	if diags := policy.Targets.ElementsAs(ctx, &targetObjects, true); diags.HasError() {
		return
	}

	anyTargetPriority := false
	seen := make(map[string]struct{}, len(targetObjects))
	for _, to := range targetObjects {
		var tm TCOTargetModel
		if dg := to.As(ctx, &tm, basetypes.ObjectAsOptions{}); dg.HasError() {
			continue
		}

		targetPrioritySet := !tm.Priority.IsNull() && !tm.Priority.IsUnknown()
		targetOverrideSet := !utils.ObjIsNullOrUnknown(tm.QuotaBasedPriorityOverride)
		if targetPrioritySet {
			anyTargetPriority = true
		}

		if targetOverrideSet && !targetPrioritySet {
			resp.Diagnostics.AddAttributeError(
				path.Root("policies"),
				"Missing Target Priority",
				"A target that sets `quota_based_priority_override` must also set `priority` (the fallback priority used once all usage tiers are exhausted).",
			)
		}

		if !tm.Dataset.IsNull() && !tm.Dataset.IsUnknown() {
			dataspace := "default"
			if !tm.Dataspace.IsNull() && !tm.Dataspace.IsUnknown() && tm.Dataspace.ValueString() != "" {
				dataspace = tm.Dataspace.ValueString()
			}
			key := dataspace + "\x00" + tm.Dataset.ValueString()
			if _, dup := seen[key]; dup {
				resp.Diagnostics.AddAttributeError(
					path.Root("policies"),
					"Duplicate Target",
					fmt.Sprintf("Targets must be unique by (dataspace, dataset); found a duplicate for dataspace %q dataset %q.", dataspace, tm.Dataset.ValueString()),
				)
			}
			seen[key] = struct{}{}
		}
	}

	if topLevelPrioritySet || topLevelOverrideSet || topLevelArchiveSet {
		resp.Diagnostics.AddAttributeError(
			path.Root("policies"),
			"Conflicting Target Configuration",
			"When `targets` is set, configure `priority`, `quota_based_priority_override`, and `archive_retention_id` per-target — they must not be set at the policy level.",
		)
	}

	if !anyTargetPriority {
		resp.Diagnostics.AddAttributeError(
			path.Root("policies"),
			"Missing Priority",
			"At least one target must set `priority` when `targets` is used.",
		)
	}
}

func validateTCOPoliciesLogs(rule types.Object, root string, resp *resource.ValidateConfigResponse) {
	if utils.ObjIsNullOrUnknown(rule) {
		return
	}

	ruleModel := &TCORuleModel{}
	diags := rule.As(context.Background(), ruleModel, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	ruleType := ruleModel.RuleType.ValueString()
	nameLength := len(ruleModel.Names.Elements())
	if (ruleType == "starts_with" || ruleType == "includes") && nameLength > 1 {
		resp.Diagnostics.AddAttributeWarning(
			path.Root(root),
			"Conflicting Attributes Values Configuration",
			fmt.Sprintf("Currently, rule_type \"%s\" supports only one value, but \"names\" has %d elements. Remove all but one to remove this warning.", ruleType, nameLength),
		)
	}
}

func tcoPolicyRuleAttributes() map[string]attr.Type {
	return map[string]attr.Type{
		"rule_type": types.StringType,
		"names":     types.SetType{ElemType: types.StringType},
	}
}

func (r *TCOPoliciesLogsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	var plan *TCOPoliciesListModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	rq, diags := extractOverwriteTcoPoliciesLogs(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	result, httpResponse, err := r.client.
		PoliciesServiceAtomicOverwriteLogPolicies(ctx).
		AtomicOverwriteLogPoliciesRequest(*rq).
		Execute()
	if err != nil {
		resp.Diagnostics.AddError("Error creating coralogix_tco_policies_logs",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Create", rq),
		)
		return
	}
	state, diags := flattenOverwriteTCOPoliciesLogsList(ctx, result)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *TCOPoliciesLogsResource) Read(ctx context.Context, _ resource.ReadRequest, resp *resource.ReadResponse) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	result, httpResponse, err := r.client.PoliciesServiceGetCompanyPolicies(ctx).SourceType(LogSource).Execute()
	if err != nil {
		if httpResponse.StatusCode == http.StatusNotFound {
			resp.Diagnostics.AddWarning(
				"coralogix_tco_policies_logs is in state, but no longer exists in Coralogix backend",
				"coralogix_tco_policies_logs will be recreated when you apply",
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError("Error reading coralogix_tco_policies_logs",
				utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Read", nil),
			)
		}
		return
	}

	state, diags := flattenGetTCOPoliciesLogsList(ctx, result)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *TCOPoliciesLogsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	var plan *TCOPoliciesListModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	rq, diags := extractOverwriteTcoPoliciesLogs(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	result, httpResponse, err := r.client.
		PoliciesServiceAtomicOverwriteLogPolicies(ctx).
		AtomicOverwriteLogPoliciesRequest(*rq).
		Execute()

	if err != nil {
		if httpResponse.StatusCode == http.StatusNotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("coralogix_tco_policies_logs %v is in state, but no longer exists in Coralogix backend", rq),
				fmt.Sprintf("%v will be recreated when you apply", rq),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError("Error replacing coralogix_tco_policies_logs", utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Replace", rq))
		}
		return
	}

	state, diags := flattenOverwriteTCOPoliciesLogsList(ctx, result)

	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *TCOPoliciesLogsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	rq := r.client.
		PoliciesServiceAtomicOverwriteLogPolicies(ctx).
		AtomicOverwriteLogPoliciesRequest(*tcoPolicys.NewAtomicOverwriteLogPoliciesRequestWithDefaults())
	_, httpResponse, err := rq.Execute()
	if err != nil {
		resp.Diagnostics.AddError("Error deleting coralogix_tco_policies_logs",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Delete", nil),
		)
		return
	}
}

func flattenOverwriteTCOPoliciesLogsList(ctx context.Context, overwriteResp *tcoPolicys.AtomicOverwriteLogPoliciesResponse) (*TCOPoliciesListModel, diag.Diagnostics) {
	var policies []*TCOPolicyLogsModel
	var diags diag.Diagnostics
	for _, policy := range overwriteResp.GetCreateResponses() {
		tcoPolicy, dgs := flattenTCOLogsPolicy(ctx, policy.GetPolicy())
		if dgs.HasError() {
			diags.Append(dgs...)
			continue
		}
		policies = append(policies, tcoPolicy)
	}

	if diags.HasError() {
		return nil, diags
	}

	policiesList, diags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: policiesLogsAttr()}, policies)
	if diags.HasError() {
		return nil, diags
	}
	return &TCOPoliciesListModel{Policies: policiesList}, nil
}

func flattenGetTCOPoliciesLogsList(ctx context.Context, getResp *tcoPolicys.GetCompanyPoliciesResponse) (*TCOPoliciesListModel, diag.Diagnostics) {
	var policies []*TCOPolicyLogsModel
	var diags diag.Diagnostics
	for _, policy := range getResp.GetPolicies() {
		tcoPolicy, dgs := flattenTCOLogsPolicy(ctx, policy)
		if dgs.HasError() {
			diags.Append(dgs...)
			continue
		}
		policies = append(policies, tcoPolicy)
	}

	if diags.HasError() {
		return nil, diags
	}

	policiesList, diags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: policiesLogsAttr()}, policies)
	if diags.HasError() {
		return nil, diags
	}
	return &TCOPoliciesListModel{Policies: policiesList}, nil
}

func flattenTCOLogsPolicy(ctx context.Context, policy tcoPolicys.Policy) (*TCOPolicyLogsModel, diag.Diagnostics) {
	logsPolicy := policy

	logRules := logsPolicy.LogRules
	applications, diags := flattenTCOPolicyRule(ctx, logsPolicy.ApplicationRule)
	if diags.HasError() {
		return nil, diags
	}
	subsystems, diags := flattenTCOPolicyRule(ctx, logsPolicy.SubsystemRule)
	if diags.HasError() {
		return nil, diags
	}
	quotaBased, diags := flattenQuotaBasedPriorityOverride(ctx, logsPolicy.PriorityOverride)
	if diags.HasError() {
		return nil, diags
	}
	targets, diags := flattenTCOTargets(ctx, logsPolicy.Targets)
	if diags.HasError() {
		return nil, diags
	}

	return &TCOPolicyLogsModel{
		ID:                         types.StringValue(logsPolicy.GetId()),
		Name:                       types.StringValue(logsPolicy.GetName()),
		Description:                types.StringValue(logsPolicy.GetDescription()),
		Enabled:                    types.BoolValue(logsPolicy.GetEnabled()),
		Order:                      types.Int64Value(int64(logsPolicy.GetOrder())),
		Priority:                   flattenTCOPriority(&logsPolicy.Priority),
		Applications:               applications,
		Subsystems:                 subsystems,
		ArchiveRetentionID:         flattenArchiveRetention(logsPolicy.ArchiveRetention),
		Severities:                 flattenTCOPolicySeverities(logRules.GetSeverities()),
		DpxlExpression:             types.StringPointerValue(logRules.DpxlExpression),
		QuotaBasedPriorityOverride: quotaBased,
		Targets:                    targets,
	}, nil
}

func flattenTCOPriority(priority *tcoPolicys.QuotaV1Priority) types.String {
	if priority == nil {
		return types.StringNull()
	}
	if p, ok := tcoPoliciesPriorityApiToSchema[*priority]; ok && p != "" {
		return types.StringValue(p)
	}
	return types.StringNull()
}

func flattenTCOTargets(ctx context.Context, targets []tcoPolicys.V1Target) (types.List, diag.Diagnostics) {
	targetType := types.ObjectType{AttrTypes: tcoTargetAttributes()}
	if len(targets) == 0 {
		return types.ListNull(targetType), nil
	}

	targetObjects := make([]TCOTargetModel, 0, len(targets))
	for _, t := range targets {
		override, diags := flattenQuotaBasedPriorityOverride(ctx, t.PriorityOverride)
		if diags.HasError() {
			return types.ListNull(targetType), diags
		}

		dataspace := "default"
		if t.Dataspace != nil && *t.Dataspace != "" {
			dataspace = *t.Dataspace
		}

		targetObjects = append(targetObjects, TCOTargetModel{
			Dataset:                    types.StringPointerValue(t.Dataset),
			Dataspace:                  types.StringValue(dataspace),
			Priority:                   flattenTCOPriority(t.Priority),
			ArchiveRetentionID:         flattenArchiveRetention(t.ArchiveRetention),
			QuotaBasedPriorityOverride: override,
		})
	}

	return types.ListValueFrom(ctx, targetType, targetObjects)
}

func flattenQuotaBasedPriorityOverride(ctx context.Context, po *tcoPolicys.PriorityOverride) (types.Object, diag.Diagnostics) {
	if po == nil || po.QuotaBased == nil {
		return types.ObjectNull(quotaBasedPriorityOverrideAttributes()), nil
	}

	tiers := po.QuotaBased.UsageTiers
	tierObjects := make([]UsageTierModel, 0, len(tiers))
	for _, t := range tiers {
		var priority string
		if t.Priority != nil {
			priority = tcoPoliciesPriorityApiToSchema[*t.Priority]
		}
		tierObjects = append(tierObjects, UsageTierModel{
			DailyQuotaPercentage: types.Float64PointerValue(t.DailyQuotaPercentage),
			Priority:             types.StringValue(priority),
		})
	}
	tiersList, diags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: usageTierAttributes()}, tierObjects)
	if diags.HasError() {
		return types.ObjectNull(quotaBasedPriorityOverrideAttributes()), diags
	}
	return types.ObjectValueFrom(ctx, quotaBasedPriorityOverrideAttributes(), &QuotaBasedPriorityOverrideModel{
		UsageTiers: tiersList,
	})
}

func policiesLogsAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"id":                            types.StringType,
		"name":                          types.StringType,
		"description":                   types.StringType,
		"enabled":                       types.BoolType,
		"order":                         types.Int64Type,
		"priority":                      types.StringType,
		"applications":                  types.ObjectType{AttrTypes: tcoPolicyRuleAttributes()},
		"subsystems":                    types.ObjectType{AttrTypes: tcoPolicyRuleAttributes()},
		"severities":                    types.SetType{ElemType: types.StringType},
		"archive_retention_id":          types.StringType,
		"dpxl_expression":               types.StringType,
		"quota_based_priority_override": types.ObjectType{AttrTypes: quotaBasedPriorityOverrideAttributes()},
		"targets":                       types.ListType{ElemType: types.ObjectType{AttrTypes: tcoTargetAttributes()}},
	}
}

func quotaBasedPriorityOverrideSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional: true,
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
							MarkdownDescription: "Daily quota consumption (in percent) at which this tier becomes active. Must be between 0 and 100.",
						},
						"priority": schema.StringAttribute{
							Required: true,
							Validators: []validator.String{
								stringvalidator.OneOf(tcoPoliciesValidPriorities...),
							},
							MarkdownDescription: fmt.Sprintf("The priority to apply when this tier is active. Can be one of %q.", tcoPoliciesValidPriorities),
						},
					},
				},
				MarkdownDescription: "Ordered list of quota-consumption tiers; the priority is dynamically reassigned to the matching tier's `priority` once `daily_quota_percentage` is reached.",
			},
		},
		MarkdownDescription: "Dynamically reassign the priority based on daily quota consumption tiers. Once all `usage_tiers` are exhausted, the applicable fallback priority is used (the policy-level `priority`, or this target's `priority` when using `targets`) — the \"Route the remaining quota to\" in the UI — which must be more restrictive than the last tier.",
	}
}

func tcoTargetAttributes() map[string]attr.Type {
	return map[string]attr.Type{
		"dataset":                       types.StringType,
		"dataspace":                     types.StringType,
		"priority":                      types.StringType,
		"archive_retention_id":          types.StringType,
		"quota_based_priority_override": types.ObjectType{AttrTypes: quotaBasedPriorityOverrideAttributes()},
	}
}

func quotaBasedPriorityOverrideAttributes() map[string]attr.Type {
	return map[string]attr.Type{
		"usage_tiers": types.ListType{ElemType: types.ObjectType{AttrTypes: usageTierAttributes()}},
	}
}

func usageTierAttributes() map[string]attr.Type {
	return map[string]attr.Type{
		"daily_quota_percentage": types.Float64Type,
		"priority":               types.StringType,
	}
}

func flattenTCOPolicyRule(ctx context.Context, rule *tcoPolicys.QuotaV1Rule) (types.Object, diag.Diagnostics) {
	if rule == nil {
		return types.ObjectNull(tcoPolicyRuleAttributes()), nil
	}

	ruleType := types.StringValue(tcoPoliciesRuleTypeApiToSchema[rule.GetRuleTypeId()])
	names := strings.Split(rule.GetName(), ",")
	namesSet := utils.StringSliceToTypeStringSet(names)
	tcoModel := &TCORuleModel{
		RuleType: ruleType,
		Names:    namesSet,
	}

	return types.ObjectValueFrom(ctx, tcoPolicyRuleAttributes(), tcoModel)
}

func extractOverwriteTcoPoliciesLogs(ctx context.Context, plan *TCOPoliciesListModel) (*tcoPolicys.AtomicOverwriteLogPoliciesRequest, diag.Diagnostics) {
	var policies []tcoPolicys.CreateLogPolicyRequest
	var policiesObjects []types.Object
	diags := plan.Policies.ElementsAs(ctx, &policiesObjects, true)
	if diags.HasError() {
		return nil, diags
	}
	for _, po := range policiesObjects {
		var tcoPolicy TCOPolicyLogsModel
		if dg := po.As(ctx, &tcoPolicy, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		policyRq, dgs := extractTcoPolicyLog(ctx, tcoPolicy)
		if dgs.HasError() {
			diags.Append(dgs...)
			continue
		}
		policies = append(policies, *policyRq)
	}

	if diags.HasError() {
		return nil, diags
	}

	return &tcoPolicys.AtomicOverwriteLogPoliciesRequest{Policies: policies}, nil
}

func extractTcoPolicyLog(ctx context.Context, plan TCOPolicyLogsModel) (*tcoPolicys.CreateLogPolicyRequest, diag.Diagnostics) {
	priority := tcoPoliciesPrioritySchemaToApi[plan.Priority.ValueString()]
	applicationRule, diags := expandTCOPolicyRule(ctx, plan.Applications)
	if diags.HasError() {
		return nil, diags
	}
	subsystemRule, diags := expandTCOPolicyRule(ctx, plan.Subsystems)
	if diags.HasError() {
		return nil, diags
	}
	archiveRetention := expandActiveRetention(plan.ArchiveRetentionID)
	severities, diags := expandTCOPolicySeverities(ctx, plan.Severities.Elements())
	if diags.HasError() {
		return nil, diags
	}
	priorityOverride, diags := expandQuotaBasedPriorityOverride(ctx, plan.QuotaBasedPriorityOverride)
	if diags.HasError() {
		return nil, diags
	}
	enabled := !plan.Enabled.ValueBool()

	targets, diags := expandTCOTargets(ctx, plan.Targets)
	if diags.HasError() {
		return nil, diags
	}

	logRules := tcoPolicys.LogRules{}
	if !plan.DpxlExpression.IsNull() && !plan.DpxlExpression.IsUnknown() {
		logRules.DpxlExpression = plan.DpxlExpression.ValueStringPointer()
	} else {
		logRules.Severities = severities
	}

	policy := tcoPolicys.CreateGenericPolicyRequest{
		Name:            plan.Name.ValueString(),
		Description:     plan.Description.ValueString(),
		ApplicationRule: applicationRule,
		SubsystemRule:   subsystemRule,
		Disabled:        &enabled,
	}

	if len(targets) > 0 {
		policy.Priority = tcoPolicys.QUOTAV1PRIORITY_PRIORITY_TYPE_UNSPECIFIED
		policy.Targets = targets
	} else {
		policy.Priority = priority
		policy.ArchiveRetention = archiveRetention
		policy.PriorityOverride = priorityOverride
	}

	return &tcoPolicys.CreateLogPolicyRequest{
		Policy:   policy,
		LogRules: logRules,
	}, nil
}

func expandTCOTargets(ctx context.Context, targets types.List) ([]tcoPolicys.V1Target, diag.Diagnostics) {
	if targets.IsNull() || targets.IsUnknown() {
		return nil, nil
	}

	var targetObjects []types.Object
	if diags := targets.ElementsAs(ctx, &targetObjects, true); diags.HasError() {
		return nil, diags
	}

	result := make([]tcoPolicys.V1Target, 0, len(targetObjects))
	for _, to := range targetObjects {
		var tm TCOTargetModel
		if diags := to.As(ctx, &tm, basetypes.ObjectAsOptions{}); diags.HasError() {
			return nil, diags
		}

		target := tcoPolicys.V1Target{}
		if !tm.Dataset.IsNull() && !tm.Dataset.IsUnknown() {
			target.Dataset = tm.Dataset.ValueStringPointer()
		}
		if !tm.Dataspace.IsNull() && !tm.Dataspace.IsUnknown() {
			target.Dataspace = tm.Dataspace.ValueStringPointer()
		}
		if !tm.Priority.IsNull() && !tm.Priority.IsUnknown() {
			p := tcoPoliciesPrioritySchemaToApi[tm.Priority.ValueString()]
			target.Priority = &p
		}
		target.ArchiveRetention = expandActiveRetention(tm.ArchiveRetentionID)

		override, diags := expandQuotaBasedPriorityOverride(ctx, tm.QuotaBasedPriorityOverride)
		if diags.HasError() {
			return nil, diags
		}
		target.PriorityOverride = override

		result = append(result, target)
	}

	return result, nil
}

func expandQuotaBasedPriorityOverride(ctx context.Context, override types.Object) (*tcoPolicys.PriorityOverride, diag.Diagnostics) {
	if override.IsNull() || override.IsUnknown() {
		return nil, nil
	}

	var model QuotaBasedPriorityOverrideModel
	if diags := override.As(ctx, &model, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	var tierObjects []types.Object
	if diags := model.UsageTiers.ElementsAs(ctx, &tierObjects, true); diags.HasError() {
		return nil, diags
	}

	tiers := make([]tcoPolicys.UsageTier, 0, len(tierObjects))
	for _, to := range tierObjects {
		var tm UsageTierModel
		if diags := to.As(ctx, &tm, basetypes.ObjectAsOptions{}); diags.HasError() {
			return nil, diags
		}
		tier := tcoPolicys.UsageTier{
			DailyQuotaPercentage: tm.DailyQuotaPercentage.ValueFloat64Pointer(),
		}
		if !tm.Priority.IsNull() && !tm.Priority.IsUnknown() {
			p := tcoPoliciesPrioritySchemaToApi[tm.Priority.ValueString()]
			tier.Priority = &p
		}
		tiers = append(tiers, tier)
	}

	return &tcoPolicys.PriorityOverride{
		QuotaBased: &tcoPolicys.QuotaBased{
			UsageTiers: tiers,
		},
	}, nil
}

func expandTCOPolicyRule(ctx context.Context, rule types.Object) (*tcoPolicys.QuotaV1Rule, diag.Diagnostics) {
	if rule.IsNull() || rule.IsUnknown() {
		return nil, nil
	}

	tcoRuleModel := &TCORuleModel{}
	diags := rule.As(ctx, tcoRuleModel, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	ruleType := tcoPoliciesRuleTypeSchemaToApi[tcoRuleModel.RuleType.ValueString()]
	names, diags := utils.TypeStringElementsToStringSlice(ctx, tcoRuleModel.Names.Elements())
	if diags.HasError() {
		return nil, diags
	}
	nameStr := strings.Join(names, ",")

	return &tcoPolicys.QuotaV1Rule{
		RuleTypeId: &ruleType,
		Name:       &nameStr,
	}, nil
}

func expandActiveRetention(archiveRetention types.String) *tcoPolicys.ArchiveRetention {
	if archiveRetention.IsNull() {
		return nil
	}

	return &tcoPolicys.ArchiveRetention{
		Id: archiveRetention.ValueStringPointer(),
	}
}

func expandTCOPolicySeverities(ctx context.Context, severities []attr.Value) ([]tcoPolicys.QuotaV1Severity, diag.Diagnostics) {
	result := make([]tcoPolicys.QuotaV1Severity, 0, len(severities))
	var diags diag.Diagnostics
	for _, severity := range severities {
		val, err := severity.ToTerraformValue(ctx)
		if err != nil {
			diags.AddError("Error expanding tco-policy severities", err.Error())
			continue
		}
		var str string
		if err = val.As(&str); err != nil {
			diags.AddError("Error expanding tco-policy severities", err.Error())
			continue
		}
		s := tcoPolicySeveritySchemaToApi[str]
		result = append(result, s)
	}
	return result, diags
}

func flattenArchiveRetention(archiveRetention *tcoPolicys.ArchiveRetention) types.String {
	if archiveRetention == nil || archiveRetention.Id == nil {
		return types.StringNull()
	}

	return types.StringValue(archiveRetention.GetId())
}

func flattenTCOPolicySeverities(severities []tcoPolicys.QuotaV1Severity) types.Set {
	if len(severities) == 0 {
		return types.SetNull(types.StringType)
	}

	elements := make([]attr.Value, 0, len(severities))
	for _, severity := range severities {
		elements = append(elements, types.StringValue(tcoPolicySeverityApiToSchema[severity]))
	}
	return types.SetValueMust(types.StringType, elements)
}
