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
	"regexp"
	"time"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

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
	_ resource.ResourceWithConfigure      = &TCOPoliciesRumResource{}
	_ resource.ResourceWithValidateConfig = &TCOPoliciesRumResource{}
	_ resource.ResourceWithImportState    = &TCOPoliciesRumResource{}
	// RumSource selects RUM policies from the shared company-policies getter.
	RumSource = tcoPolicys.V1SOURCETYPE_SOURCE_TYPE_RUM
	// tcoRumTierPriorities are the priorities valid inside a usage tier. "block" is
	// excluded: the API rejects it as a tier value (a tier reassigns priority, so a
	// block tier would just drop data and has no effect).
	tcoRumTierPriorities = func() []string {
		out := make([]string, 0, len(tcoPoliciesValidPriorities))
		for _, p := range tcoPoliciesValidPriorities {
			if p != "block" {
				out = append(out, p)
			}
		}
		return out
	}()
)

func NewTCOPoliciesRumResource() resource.Resource {
	return &TCOPoliciesRumResource{}
}

type TCOPoliciesRumResource struct {
	client *tcoPolicys.PoliciesServiceAPIService
}

func (r *TCOPoliciesRumResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// TCOPolicyRumModel mirrors TCOPolicyLogsModel minus Targets — RUM policies have no
// dataset routing.
type TCOPolicyRumModel struct {
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
}

func (r *TCOPoliciesRumResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tco_policies_rum"
}

func (r *TCOPoliciesRumResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *TCOPoliciesRumResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
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
							Required: true,
							Validators: []validator.String{
								stringvalidator.OneOf(tcoPoliciesValidPriorities...),
							},
							MarkdownDescription: fmt.Sprintf("The policy priority. Can be one of %q. When `quota_based_priority_override` is set, this is also the fallback priority applied once all `usage_tiers` are exhausted — the equivalent of \"Route the remaining quota to\" in the UI — and must be more restrictive than the last tier's priority (most to least restrictive: `block`, `low`, `medium`, `high`).", tcoPoliciesValidPriorities),
						},
						"order": schema.Int64Attribute{
							Computed:            true,
							MarkdownDescription: "The policy's order between the other policies.",
						},
						"archive_retention_id": schema.StringAttribute{
							Optional:    true,
							Description: "Allowing RUM events with a specific retention to be tagged.",
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
							MarkdownDescription: fmt.Sprintf("The severities to apply the policy on. Valid severities are %q. Mutually exclusive with `dpxl_expression` — set exactly one.", validPolicySeverities),
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
											regexp.MustCompile("^[^A-Z]*$"), "must not contain uppercase letters; the backend lowercases application/subsystem names, so an uppercase value would drift")),
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
											regexp.MustCompile("^[^A-Z]*$"), "must not contain uppercase letters; the backend lowercases application/subsystem names, so an uppercase value would drift")),
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
							MarkdownDescription: "DataPrime expression to match RUM events for this policy. Mutually exclusive with `severities` — set exactly one. The expression must include a version prefix and reference the canonical `$d.*` schema (not `$d.cx_rum.*`), e.g. `<v1> $d.severity == 'Error'`.",
						},
						"quota_based_priority_override": schema.SingleNestedAttribute{
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
													stringvalidator.OneOf(tcoRumTierPriorities...),
												},
												MarkdownDescription: fmt.Sprintf("The priority to apply when this tier is active. Can be one of %q (`block` is not valid for a tier).", tcoRumTierPriorities),
											},
										},
									},
									MarkdownDescription: "Ordered list of quota-consumption tiers; the policy's priority is dynamically reassigned to the matching tier's `priority` once `daily_quota_percentage` is reached.",
								},
							},
							MarkdownDescription: "Dynamically reassign the policy's priority based on daily quota consumption tiers. Once all `usage_tiers` are exhausted, the policy's top-level `priority` is used as the fallback (\"Route the remaining quota to\" in the UI), which must be more restrictive than the last tier.",
						},
					},
				},
				Required: true,
			},
		},
		MarkdownDescription: "Coralogix RUM TCO-Policies-List. Behaves like `coralogix_tco_policies_logs` minus dataset routing (`targets`). For more information - https://coralogix.com/docs/tco-optimizer-api.",
	}
}

func (r *TCOPoliciesRumResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
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
		var tcoPolicy TCOPolicyRumModel
		if dg := po.As(ctx, &tcoPolicy, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		validateTCOPoliciesLogs(tcoPolicy.Subsystems, "subsystems", resp)
		validateTCOPoliciesLogs(tcoPolicy.Applications, "applications", resp)
	}
}

func (r *TCOPoliciesRumResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	var plan *TCOPoliciesListModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	rq, diags := extractOverwriteTcoPoliciesRum(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	result, httpResponse, err := r.client.
		PoliciesServiceAtomicOverwriteRumPolicies(ctx).
		AtomicOverwriteRumPoliciesRequest(*rq).
		Execute()
	if err != nil {
		resp.Diagnostics.AddError("Error creating coralogix_tco_policies_rum",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Create", rq),
		)
		return
	}
	state, diags := flattenOverwriteTCOPoliciesRumList(ctx, result)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *TCOPoliciesRumResource) Read(ctx context.Context, _ resource.ReadRequest, resp *resource.ReadResponse) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	result, httpResponse, err := r.client.PoliciesServiceGetCompanyPolicies(ctx).SourceType(RumSource).Execute()
	if err != nil {
		apiErr := cxsdkOpenapi.NewAPIError(httpResponse, err)
		if cxsdkOpenapi.Code(apiErr) == http.StatusNotFound {
			resp.Diagnostics.AddWarning(
				"coralogix_tco_policies_rum is in state, but no longer exists in Coralogix backend",
				"coralogix_tco_policies_rum will be recreated when you apply",
			)
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading coralogix_tco_policies_rum",
			utils.FormatOpenAPIErrors(apiErr, "Read", nil),
		)
		return
	}

	state, diags := flattenGetTCOPoliciesRumList(ctx, result)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *TCOPoliciesRumResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	var plan *TCOPoliciesListModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	rq, diags := extractOverwriteTcoPoliciesRum(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	result, httpResponse, err := r.client.
		PoliciesServiceAtomicOverwriteRumPolicies(ctx).
		AtomicOverwriteRumPoliciesRequest(*rq).
		Execute()

	if err != nil {
		apiErr := cxsdkOpenapi.NewAPIError(httpResponse, err)
		if cxsdkOpenapi.Code(apiErr) == http.StatusNotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("coralogix_tco_policies_rum %v is in state, but no longer exists in Coralogix backend", rq),
				fmt.Sprintf("%v will be recreated when you apply", rq),
			)
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error replacing coralogix_tco_policies_rum", utils.FormatOpenAPIErrors(apiErr, "Replace", rq))
		return
	}

	state, diags := flattenOverwriteTCOPoliciesRumList(ctx, result)

	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *TCOPoliciesRumResource) Delete(ctx context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	rq := r.client.
		PoliciesServiceAtomicOverwriteRumPolicies(ctx).
		AtomicOverwriteRumPoliciesRequest(*tcoPolicys.NewAtomicOverwriteRumPoliciesRequestWithDefaults())
	_, httpResponse, err := rq.Execute()
	if err != nil {
		resp.Diagnostics.AddError("Error deleting coralogix_tco_policies_rum",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Delete", nil),
		)
		return
	}
}

func extractOverwriteTcoPoliciesRum(ctx context.Context, plan *TCOPoliciesListModel) (*tcoPolicys.AtomicOverwriteRumPoliciesRequest, diag.Diagnostics) {
	var policies []tcoPolicys.CreateRumPolicyRequest
	var policiesObjects []types.Object
	diags := plan.Policies.ElementsAs(ctx, &policiesObjects, true)
	if diags.HasError() {
		return nil, diags
	}
	for _, po := range policiesObjects {
		var tcoPolicy TCOPolicyRumModel
		if dg := po.As(ctx, &tcoPolicy, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		policyRq, dgs := extractTcoPolicyRum(ctx, tcoPolicy)
		if dgs.HasError() {
			diags.Append(dgs...)
			continue
		}
		policies = append(policies, *policyRq)
	}

	if diags.HasError() {
		return nil, diags
	}

	return &tcoPolicys.AtomicOverwriteRumPoliciesRequest{Policies: policies}, nil
}

func extractTcoPolicyRum(ctx context.Context, plan TCOPolicyRumModel) (*tcoPolicys.CreateRumPolicyRequest, diag.Diagnostics) {
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

	rumRules := tcoPolicys.LogRules{}
	if !plan.DpxlExpression.IsNull() && !plan.DpxlExpression.IsUnknown() {
		rumRules.DpxlExpression = plan.DpxlExpression.ValueStringPointer()
	} else {
		rumRules.Severities = severities
	}

	return &tcoPolicys.CreateRumPolicyRequest{
		Policy: tcoPolicys.CreateGenericPolicyRequest{
			Name:             plan.Name.ValueString(),
			Description:      plan.Description.ValueString(),
			Priority:         priority,
			ApplicationRule:  applicationRule,
			SubsystemRule:    subsystemRule,
			ArchiveRetention: archiveRetention,
			Disabled:         &enabled,
			PriorityOverride: priorityOverride,
		},
		RumRules: rumRules,
	}, nil
}

func flattenOverwriteTCOPoliciesRumList(ctx context.Context, overwriteResp *tcoPolicys.AtomicOverwriteRumPoliciesResponse) (*TCOPoliciesListModel, diag.Diagnostics) {
	policies := make([]*TCOPolicyRumModel, 0)
	var diags diag.Diagnostics
	for _, policy := range overwriteResp.GetCreateResponses() {
		tcoPolicy, dgs := flattenTCORumPolicy(ctx, policy.GetPolicy())
		if dgs.HasError() {
			diags.Append(dgs...)
			continue
		}
		policies = append(policies, tcoPolicy)
	}

	if diags.HasError() {
		return nil, diags
	}

	policiesList, diags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: policiesRumAttr()}, policies)
	if diags.HasError() {
		return nil, diags
	}
	return &TCOPoliciesListModel{Policies: policiesList}, nil
}

func flattenGetTCOPoliciesRumList(ctx context.Context, getResp *tcoPolicys.GetCompanyPoliciesResponse) (*TCOPoliciesListModel, diag.Diagnostics) {
	policies := make([]*TCOPolicyRumModel, 0)
	var diags diag.Diagnostics
	for _, policy := range getResp.GetPolicies() {
		tcoPolicy, dgs := flattenTCORumPolicy(ctx, policy)
		if dgs.HasError() {
			diags.Append(dgs...)
			continue
		}
		policies = append(policies, tcoPolicy)
	}

	if diags.HasError() {
		return nil, diags
	}

	policiesList, diags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: policiesRumAttr()}, policies)
	if diags.HasError() {
		return nil, diags
	}
	return &TCOPoliciesListModel{Policies: policiesList}, nil
}

func flattenTCORumPolicy(ctx context.Context, policy tcoPolicys.Policy) (*TCOPolicyRumModel, diag.Diagnostics) {
	rumRules := policy.GetRumRules()
	applications, diags := flattenTCOPolicyRule(ctx, policy.ApplicationRule)
	if diags.HasError() {
		return nil, diags
	}
	subsystems, diags := flattenTCOPolicyRule(ctx, policy.SubsystemRule)
	if diags.HasError() {
		return nil, diags
	}
	quotaBased, diags := flattenQuotaBasedPriorityOverride(ctx, policy.PriorityOverride)
	if diags.HasError() {
		return nil, diags
	}
	return &TCOPolicyRumModel{
		ID:                         types.StringValue(policy.GetId()),
		Name:                       types.StringValue(policy.GetName()),
		Description:                types.StringValue(policy.GetDescription()),
		Enabled:                    types.BoolValue(policy.GetEnabled()),
		Order:                      types.Int64Value(int64(policy.GetOrder())),
		Priority:                   types.StringValue(tcoPoliciesPriorityApiToSchema[policy.GetPriority()]),
		Applications:               applications,
		Subsystems:                 subsystems,
		ArchiveRetentionID:         flattenArchiveRetention(policy.ArchiveRetention),
		Severities:                 flattenTCOPolicySeverities(rumRules.GetSeverities()),
		DpxlExpression:             types.StringPointerValue(rumRules.DpxlExpression),
		QuotaBasedPriorityOverride: quotaBased,
	}, nil
}

func policiesRumAttr() map[string]attr.Type {
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
	}
}
