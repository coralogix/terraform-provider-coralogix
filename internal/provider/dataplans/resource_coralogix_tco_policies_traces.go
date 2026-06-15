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
	"strings"
	"time"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	tcoPolicys "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/policies_service"
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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

var (
	_            resource.ResourceWithConfigure      = &TCOPoliciesTracesResource{}
	_            resource.ResourceWithValidateConfig = &TCOPoliciesTracesResource{}
	_            resource.ResourceWithImportState    = &TCOPoliciesTracesResource{}
	TracesSource                                     = tcoPolicys.V1SOURCETYPE_SOURCE_TYPE_SPANS
)

func NewTCOPoliciesTracesResource() resource.Resource {
	return &TCOPoliciesTracesResource{}
}

type TCOPoliciesTracesResource struct {
	client *tcoPolicys.PoliciesServiceAPIService
}

func (r *TCOPoliciesTracesResource) ImportState(ctx context.Context, request resource.ImportStateRequest, response *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), request, response)
}

type TCOPolicyTracesModel struct {
	ID                 types.String `tfsdk:"id"`
	Name               types.String `tfsdk:"name"`
	Description        types.String `tfsdk:"description"`
	Enabled            types.Bool   `tfsdk:"enabled"`
	Order              types.Int64  `tfsdk:"order"`
	Priority           types.String `tfsdk:"priority"`
	Applications       types.Object `tfsdk:"applications"` //TCORuleModel
	Subsystems         types.Object `tfsdk:"subsystems"`   //TCORuleModel
	ArchiveRetentionID types.String `tfsdk:"archive_retention_id"`
	Services           types.Object `tfsdk:"services"` //TCORuleModel
	Actions            types.Object `tfsdk:"actions"`  //TCORuleModel
	Tags               types.Map    `tfsdk:"tags"`     //string -> TCORuleModel
	Targets            types.List   `tfsdk:"targets"`  // []TCOPolicyTraceTargetModel
}

func (r *TCOPoliciesTracesResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tco_policies_traces"
}

func (r *TCOPoliciesTracesResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *TCOPoliciesTracesResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
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
							MarkdownDescription: fmt.Sprintf("Legacy policy-level priority. Required when `targets` is not set. Can be one of %q.", tcoPoliciesValidPriorities),
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
						"actions": schema.SingleNestedAttribute{
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
							MarkdownDescription: "The actions to apply the policy on. Applies the policy on all the actions by default.",
						},
						"services": schema.SingleNestedAttribute{
							Optional: true,
							Attributes: map[string]schema.Attribute{
								"names": schema.SetAttribute{
									Required:    true,
									ElementType: types.StringType,
									Validators: []validator.Set{
										setvalidator.SizeAtLeast(1),
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
							MarkdownDescription: "The services to apply the policy on. Applies the policy on all the services by default.",
						},
						"tags": schema.MapNestedAttribute{
							Optional: true,
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"names": schema.SetAttribute{
										Required:    true,
										ElementType: types.StringType,
										Validators: []validator.Set{
											setvalidator.SizeAtLeast(1),
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
							},
							Validators: []validator.Map{
								mapvalidator.KeysAre(stringvalidator.RegexMatches(regexp.MustCompile("tags.*"), "tag names must have a 'tags.' prefix")),
							},
							MarkdownDescription: "The tags to apply the policy on. Applies the policy on all the tags by default.",
						},
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
										MarkdownDescription: "The dataset routed by this target.",
									},
									"dataspace": schema.StringAttribute{
										Optional: true,
										Computed: true,
										Default:  stringdefault.StaticString(tcoPolicyDefaultDataspace),
										Validators: []validator.String{
											stringvalidator.RegexMatches(tcoPolicyTargetDataspaceRegexp, "dataspace must start with a letter and contain only letters, numbers, underscores, or dots between segments"),
										},
										MarkdownDescription: "The dataspace routed by this target.",
									},
									"priority": schema.StringAttribute{
										Required: true,
										Validators: []validator.String{
											stringvalidator.OneOf(tcoPoliciesValidPriorities...),
										},
										MarkdownDescription: fmt.Sprintf("The target priority. Can be one of %q.", tcoPoliciesValidPriorities),
									},
									"archive_retention_id": schema.StringAttribute{
										Optional:    true,
										Description: "Allowing traces routed to this target to be tagged with a specific retention.",
										Validators: []validator.String{
											stringvalidator.LengthAtLeast(1),
										},
									},
								},
							},
							MarkdownDescription: "Target-level routing destinations for this policy. When set, legacy top-level priority and archive_retention_id must not be set.",
						},
					},
				},
				Required: true,
			},
		},
		MarkdownDescription: "Coralogix TCO-Policies-List. For more information - https://coralogix.com/docs/tco-optimizer-api.",
	}
}

func (r *TCOPoliciesTracesResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
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
		var tcoPolicy TCOPolicyTracesModel
		if dg := po.As(ctx, &tcoPolicy, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		validateTCORuleModelModel(tcoPolicy.Subsystems, "subsystems", resp)
		validateTCORuleModelModel(tcoPolicy.Applications, "applications", resp)
	}
}

func (r *TCOPoliciesTracesResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	var plan *TCOPoliciesListModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	rq, diags := extractOverwriteTcoPoliciesTraces(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	result, httpResponse, err := r.client.
		PoliciesServiceAtomicOverwriteSpanPolicies(ctx).
		AtomicOverwriteSpanPoliciesRequest(*rq).
		Execute()
	if err != nil {
		resp.Diagnostics.AddError("Error creating coralogix_tco_policies_traces",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Create", rq),
		)
		return
	}
	state, diags := flattenOverwriteTCOPoliciesTracesList(ctx, result)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *TCOPoliciesTracesResource) Read(ctx context.Context, _ resource.ReadRequest, resp *resource.ReadResponse) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	result, httpResponse, err := r.client.
		PoliciesServiceGetCompanyPolicies(ctx).
		SourceType(TracesSource).
		Execute()

	if err != nil {
		if httpResponse.StatusCode == http.StatusNotFound {
			resp.Diagnostics.AddWarning(
				"coralogix_tco_policies_traces is in state, but no longer exists in Coralogix backend",
				"coralogix_tco_policies_traces will be recreated when you apply",
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError("Error reading coralogix_tco_policies_traces",
				utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Read", nil),
			)
		}
		return
	}

	state, diags := flattenGetTCOTracesPoliciesList(ctx, result)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *TCOPoliciesTracesResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	var plan *TCOPoliciesListModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	rq, diags := extractOverwriteTcoPoliciesTraces(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	result, httpResponse, err := r.client.
		PoliciesServiceAtomicOverwriteSpanPolicies(ctx).
		AtomicOverwriteSpanPoliciesRequest(*rq).
		Execute()

	if err != nil {
		if httpResponse.StatusCode == http.StatusNotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("coralogix_tco_policies_traces %v is in state, but no longer exists in Coralogix backend", rq),
				fmt.Sprintf("%v will be recreated when you apply", rq),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError("Error replacing coralogix_tco_policies_traces", utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Replace", rq))
		}
		return
	}

	state, diags := flattenOverwriteTCOPoliciesTracesList(ctx, result)

	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *TCOPoliciesTracesResource) Delete(ctx context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	rq := r.client.
		PoliciesServiceAtomicOverwriteSpanPolicies(ctx).
		AtomicOverwriteSpanPoliciesRequest(*tcoPolicys.NewAtomicOverwriteSpanPoliciesRequestWithDefaults())
	_, httpResponse, err := rq.Execute()
	if err != nil {
		resp.Diagnostics.AddError("Error deleting coralogix_tco_policies_traces",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Delete", nil),
		)
		return
	}
}

func extractOverwriteTcoPoliciesTraces(ctx context.Context, plan *TCOPoliciesListModel) (*tcoPolicys.AtomicOverwriteSpanPoliciesRequest, diag.Diagnostics) {
	var policies []tcoPolicys.CreateSpanPolicyRequest
	var policiesObjects []types.Object
	diags := plan.Policies.ElementsAs(ctx, &policiesObjects, true)
	if diags.HasError() {
		return nil, diags
	}
	for _, po := range policiesObjects {
		var tcoPolicy TCOPolicyTracesModel
		if dg := po.As(ctx, &tcoPolicy, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		createPolicyRequest, dgs := extractTcoPolicyTraces(ctx, tcoPolicy)
		if dgs.HasError() {
			diags.Append(dgs...)
			continue
		}
		policies = append(policies, *createPolicyRequest)
	}

	if diags.HasError() {
		return nil, diags
	}

	return &tcoPolicys.AtomicOverwriteSpanPoliciesRequest{Policies: policies}, nil
}

func extractTcoPolicyTraces(ctx context.Context, plan TCOPolicyTracesModel) (*tcoPolicys.CreateSpanPolicyRequest, diag.Diagnostics) {

	applicationRule, diags := expandTCOPolicyRule(ctx, plan.Applications)
	if diags.HasError() {
		return nil, diags
	}
	subsystemRule, diags := expandTCOPolicyRule(ctx, plan.Subsystems)
	if diags.HasError() {
		return nil, diags
	}
	services, diags := expandTCOPolicyRule(ctx, plan.Services)
	if diags.HasError() {
		return nil, diags
	}
	actions, diags := expandTCOPolicyRule(ctx, plan.Actions)
	if diags.HasError() {
		return nil, diags
	}
	tagRules, diags := expandTagsRules(ctx, plan.Tags)
	if diags.HasError() {
		return nil, diags
	}
	enabled := !plan.Enabled.ValueBool()

	policy := tcoPolicys.CreateGenericPolicyRequest{
		Name:            plan.Name.ValueString(),
		Description:     plan.Description.ValueString(),
		ApplicationRule: applicationRule,
		SubsystemRule:   subsystemRule,
		Disabled:        &enabled,
	}
	targetsConfigured := !plan.Targets.IsNull() && !plan.Targets.IsUnknown()
	legacyPriorityConfigured := !plan.Priority.IsNull() && !plan.Priority.IsUnknown()
	if targetsConfigured {
		if legacyPriorityConfigured || !plan.ArchiveRetentionID.IsNull() {
			diags.AddError(
				"TCO trace policy cannot mix targets with top-level priority or archive_retention_id",
				"Move priority and archive retention settings into each target, or remove targets and use the legacy top-level fields.",
			)
			return nil, diags
		}
		targets, dgs := expandTCOTracePolicyTargets(ctx, plan.Targets)
		if dgs.HasError() {
			diags.Append(dgs...)
			return nil, diags
		}
		policy.Priority = tcoPolicys.QUOTAV1PRIORITY_PRIORITY_TYPE_UNSPECIFIED
		policy.Targets = targets
	} else {
		if !legacyPriorityConfigured {
			diags.AddError(
				"TCO trace policy must use either targets or legacy top-level priority",
				"Set targets for target-level routing, or set the legacy top-level priority field.",
			)
			return nil, diags
		}
		policy.Priority = tcoPoliciesPrioritySchemaToApi[plan.Priority.ValueString()]
		policy.ArchiveRetention = expandActiveRetention(plan.ArchiveRetentionID)
	}

	return &tcoPolicys.CreateSpanPolicyRequest{
		Policy: policy,
		SpanRules: tcoPolicys.SpanRules{
			ServiceRule: services,
			ActionRule:  actions,
			TagRules:    tagRules,
		},
	}, nil
}

func flattenOverwriteTCOPoliciesTracesList(ctx context.Context, overwriteResp *tcoPolicys.AtomicOverwriteSpanPoliciesResponse) (*TCOPoliciesListModel, diag.Diagnostics) {
	var policies []*TCOPolicyTracesModel
	var diags diag.Diagnostics
	for _, policy := range overwriteResp.GetCreateResponses() {
		tcoPolicy, dgs := flattenTCOTracesPolicy(ctx, &policy.Policy)
		if dgs.HasError() {
			diags.Append(dgs...)
			continue
		}
		policies = append(policies, tcoPolicy)
	}

	if diags.HasError() {
		return nil, diags
	}

	policiesList, diags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: policiesTracesAttr()}, policies)
	if diags.HasError() {
		return nil, diags
	}
	return &TCOPoliciesListModel{Policies: policiesList}, nil
}

func policiesTracesAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"id":                   types.StringType,
		"name":                 types.StringType,
		"description":          types.StringType,
		"enabled":              types.BoolType,
		"order":                types.Int64Type,
		"priority":             types.StringType,
		"applications":         types.ObjectType{AttrTypes: tcoPolicyRuleAttributes()},
		"subsystems":           types.ObjectType{AttrTypes: tcoPolicyRuleAttributes()},
		"archive_retention_id": types.StringType,
		"actions":              types.ObjectType{AttrTypes: tcoPolicyRuleAttributes()},
		"services":             types.ObjectType{AttrTypes: tcoPolicyRuleAttributes()},
		"tags":                 types.MapType{ElemType: types.ObjectType{AttrTypes: tcoPolicyRuleAttributes()}},
		"targets":              types.ListType{ElemType: types.ObjectType{AttrTypes: tcoPolicyTraceTargetAttributes()}},
	}
}

func expandTCOTracePolicyTargets(ctx context.Context, targets types.List) ([]tcoPolicys.V1Target, diag.Diagnostics) {
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
		var targetModel TCOPolicyTraceTargetModel
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
			dataspace := tcoPolicyDefaultDataspace
			target.Dataspace = &dataspace
		}
		if !targetModel.Priority.IsNull() && !targetModel.Priority.IsUnknown() {
			priority := tcoPoliciesPrioritySchemaToApi[targetModel.Priority.ValueString()]
			target.Priority = &priority
		}
		result = append(result, target)
	}
	if diags.HasError() {
		return nil, diags
	}
	return result, nil
}

func flattenTCOTracePolicyTargets(ctx context.Context, targets []tcoPolicys.V1Target) (types.List, diag.Diagnostics) {
	if len(targets) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: tcoPolicyTraceTargetAttributes()}), nil
	}

	targetModels := make([]TCOPolicyTraceTargetModel, 0, len(targets))
	for _, target := range targets {
		priority := types.StringNull()
		if target.Priority != nil {
			priority = types.StringValue(tcoPoliciesPriorityApiToSchema[*target.Priority])
		}
		dataspace := types.StringValue(tcoPolicyDefaultDataspace)
		if target.Dataspace != nil {
			dataspace = types.StringValue(*target.Dataspace)
		}
		targetModels = append(targetModels, TCOPolicyTraceTargetModel{
			Dataset:            types.StringPointerValue(target.Dataset),
			Dataspace:          dataspace,
			Priority:           priority,
			ArchiveRetentionID: flattenArchiveRetention(target.ArchiveRetention),
		})
	}
	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: tcoPolicyTraceTargetAttributes()}, targetModels)
}

func flattenGetTCOTracesPoliciesList(ctx context.Context, resp *tcoPolicys.GetCompanyPoliciesResponse) (TCOPoliciesListModel, diag.Diagnostics) {
	var policies []*TCOPolicyTracesModel
	var diags diag.Diagnostics
	for _, policy := range resp.GetPolicies() {
		tcoPolicy, dgs := flattenTCOTracesPolicy(ctx, &policy)
		if dgs.HasError() {
			diags.Append(dgs...)
			continue
		}
		policies = append(policies, tcoPolicy)
	}

	if diags.HasError() {
		return TCOPoliciesListModel{}, diags
	}

	policiesList, diags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: policiesTracesAttr()}, policies)
	if diags.HasError() {
		return TCOPoliciesListModel{}, diags
	}
	return TCOPoliciesListModel{Policies: policiesList}, nil

}

func flattenTCOTracesPolicy(ctx context.Context, policy *tcoPolicys.Policy) (*TCOPolicyTracesModel, diag.Diagnostics) {

	spanPolicy := policy.PolicySpanRules

	traceRules := spanPolicy.SpanRules
	applications, diags := flattenTCOPolicyRule(ctx, spanPolicy.ApplicationRule)
	if diags.HasError() {
		return nil, diags
	}
	subsystems, diags := flattenTCOPolicyRule(ctx, spanPolicy.SubsystemRule)
	if diags.HasError() {
		return nil, diags
	}
	services, diags := flattenTCOPolicyRule(ctx, traceRules.ServiceRule)
	if diags.HasError() {
		return nil, diags
	}
	actions, diags := flattenTCOPolicyRule(ctx, traceRules.ActionRule)
	if diags.HasError() {
		return nil, diags
	}
	targets, diags := flattenTCOTracePolicyTargets(ctx, spanPolicy.GetTargets())
	if diags.HasError() {
		return nil, diags
	}
	priority := types.StringValue(tcoPoliciesPriorityApiToSchema[spanPolicy.GetPriority()])
	archiveRetentionID := flattenArchiveRetention(spanPolicy.ArchiveRetention)
	if len(spanPolicy.GetTargets()) > 0 {
		priority = types.StringNull()
		archiveRetentionID = types.StringNull()
	}

	return &TCOPolicyTracesModel{
		ID:                 types.StringValue(spanPolicy.GetId()),
		Name:               types.StringValue(spanPolicy.GetName()),
		Description:        types.StringValue(spanPolicy.GetDescription()),
		Enabled:            types.BoolValue(spanPolicy.GetEnabled()),
		Order:              types.Int64Value(int64(spanPolicy.GetOrder())),
		Priority:           priority,
		Applications:       applications,
		Subsystems:         subsystems,
		ArchiveRetentionID: archiveRetentionID,
		Services:           services,
		Actions:            actions,
		Tags:               flattenTCOPolicyTags(ctx, traceRules.TagRules),
		Targets:            targets,
	}, nil
}

func validateTCORuleModelModel(rule types.Object, root string, resp *resource.ValidateConfigResponse) {
	if rule.IsNull() || rule.IsUnknown() {
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

func flattenTCOPolicyTags(ctx context.Context, tags []tcoPolicys.TagRule) types.Map {
	if len(tags) == 0 {
		return types.MapNull(types.ObjectType{AttrTypes: tcoRuleModelAttr()})
	}

	elements := make(map[string]attr.Value)
	for _, tag := range tags {
		name := tag.GetTagName()

		ruleType := types.StringValue(tcoPoliciesRuleTypeApiToSchema[tag.RuleTypeId])

		values := strings.Split(tag.GetTagValue(), ",")
		valuesSet := utils.StringSliceToTypeStringSet(values)

		tagRule := TCORuleModel{RuleType: ruleType, Names: valuesSet}

		element, _ := types.ObjectValueFrom(ctx, tcoRuleModelAttr(), tagRule)
		elements[name] = element
	}

	return types.MapValueMust(types.ObjectType{AttrTypes: tcoRuleModelAttr()}, elements)
}

func expandTagsRules(ctx context.Context, tags types.Map) ([]tcoPolicys.TagRule, diag.Diagnostics) {
	var tagsMap map[string]types.Object
	d := tags.ElementsAs(ctx, &tagsMap, true)
	if d != nil {
		panic(d)
	}

	var diags diag.Diagnostics
	result := make([]tcoPolicys.TagRule, 0, len(tagsMap))
	for tagName, tagElement := range tagsMap {
		tagRule, e := expandTagRule(ctx, tagName, tagElement)
		if e.HasError() {
			diags.Append(e...)
			continue
		}
		result = append(result, *tagRule)
	}

	if diags.HasError() {
		return nil, diags
	}
	return result, nil
}

func tcoRuleModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"rule_type": types.StringType,
		"names": types.SetType{
			ElemType: types.StringType,
		},
	}
}

func expandTagRule(ctx context.Context, name string, tag types.Object) (*tcoPolicys.TagRule, diag.Diagnostics) {
	rule, diags := expandTCOPolicyRule(ctx, tag)
	if diags.HasError() {
		return nil, diags
	}
	return &tcoPolicys.TagRule{
		TagName:    name,
		RuleTypeId: rule.GetRuleTypeId(),
		TagValue:   rule.GetName(),
	}, nil
}
