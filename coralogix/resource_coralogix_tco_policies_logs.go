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

package coralogix

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"terraform-provider-coralogix/coralogix/clientset"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"

	"regexp"

	"google.golang.org/protobuf/encoding/protojson"

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
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

var (
	_                                resource.ResourceWithConfigure   = &TCOPoliciesLogsResource{}
	_                                resource.ResourceWithImportState = &TCOPoliciesLogsResource{}
	tcoPoliciesPrioritySchemaToProto                                  = map[string]cxsdk.TCOPolicyPriority{
		"block":  cxsdk.TCOPolicyPriorityBlock,
		"high":   cxsdk.TCOPolicyPriorityHigh,
		"low":    cxsdk.TCOPolicyPriorityLow,
		"medium": cxsdk.TCOPolicyPriorityMedium,
	}
	tcoPoliciesPriorityProtoToSchema = ReverseMap(tcoPoliciesPrioritySchemaToProto)
	tcoPoliciesValidPriorities       = GetKeys(tcoPoliciesPrioritySchemaToProto)
	tcoPoliciesRuleTypeSchemaToProto = map[string]cxsdk.TCOPolicyRuleTypeID{
		"is":          cxsdk.TCOPolicyRuleTypeIDIs,
		"is_not":      cxsdk.TCOPolicyRuleTypeIDIsNot,
		"starts_with": cxsdk.TCOPolicyRuleTypeIDStartWith,
		"includes":    cxsdk.TCOPolicyRuleTypeIDIncludes,
		"unspecified": cxsdk.TCOPolicyRuleTypeIDUnspecified,
	}
	tcoPoliciesRuleTypeProtoToSchema = ReverseMap(tcoPoliciesRuleTypeSchemaToProto)
	tcoPoliciesValidRuleTypes        = GetKeys(tcoPoliciesRuleTypeSchemaToProto)
	tcoPolicySeveritySchemaToProto   = map[string]cxsdk.TCOPolicySeverity{
		"debug":    cxsdk.TCOPolicySeverityDebug,
		"verbose":  cxsdk.TCOPolicySeverityVerbose,
		"info":     cxsdk.TCOPolicySeverityInfo,
		"warning":  cxsdk.TCOPolicySeverityWarning,
		"error":    cxsdk.TCOPolicySeverityError,
		"critical": cxsdk.TCOPolicySeverityCritical,
	}
	tcoPolicySeverityProtoToSchema = ReverseMap(tcoPolicySeveritySchemaToProto)
	validPolicySeverities          = GetKeys(tcoPolicySeveritySchemaToProto)
	overrideTCOPoliciesLogsURL     = cxsdk.TCOPoliciesAtomicOverwriteLogPoliciesRPC
	getCompanyPoliciesURL          = cxsdk.TCOPoliciesGetCompanyPoliciesRPC
	logSource                      = cxsdk.TCOPolicySourceTypeLogs
)

func NewTCOPoliciesLogsResource() resource.Resource {
	return &TCOPoliciesLogsResource{}
}

type TCOPoliciesLogsResource struct {
	client *cxsdk.TCOPoliciesClient
}

func (r *TCOPoliciesLogsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

type TCOPoliciesListModel struct {
	ID       types.String `tfsdk:"id"`
	Policies types.List   `tfsdk:"policies"` // TCOPolicyLogsModel
}

type TCOPolicyLogsModel struct {
	ID                 types.String `tfsdk:"id"`
	Name               types.String `tfsdk:"name"`
	Description        types.String `tfsdk:"description"`
	Enabled            types.Bool   `tfsdk:"enabled"`
	Order              types.Int64  `tfsdk:"order"`
	Priority           types.String `tfsdk:"priority"`
	Applications       types.Object `tfsdk:"applications"`
	Subsystems         types.Object `tfsdk:"subsystems"`
	Severities         types.Set    `tfsdk:"severities"`
	ArchiveRetentionID types.String `tfsdk:"archive_retention_id"`
}

type TCORuleModel struct {
	RuleType types.String `tfsdk:"rule_type"`
	Names    types.Set    `tfsdk:"names"`
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
							Required: true,
							Validators: []validator.String{
								stringvalidator.OneOf(tcoPoliciesValidPriorities...),
							},
							MarkdownDescription: fmt.Sprintf("The policy priority. Can be one of %q.", tcoPoliciesValidPriorities),
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
							ElementType: types.StringType,
							Validators: []validator.Set{
								setvalidator.SizeAtLeast(1),
								setvalidator.ValueStringsAre(stringvalidator.OneOf(validPolicySeverities...)),
							},
							MarkdownDescription: fmt.Sprintf("The severities to apply the policy on. Can be few of %q.", validPolicySeverities),
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
	}
}

func validateTCOPoliciesLogs(rule types.Object, root string, resp *resource.ValidateConfigResponse) {
	if objIsNullOrUnknown(rule) {
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
			fmt.Sprintf("Currently, rule_type \"%s\" is supportred with only one value, but \"names\" includes %d elements.", ruleType, nameLength),
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

	overwriteReq, diags := extractOverwriteTcoPoliciesLogs(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	log.Printf("[INFO] Overwriting tco-policies-logs list: %s", protojson.Format(overwriteReq))
	overwriteResp, err := r.client.OverwriteTCOLogsPolicies(ctx, overwriteReq)
	for err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if retryableStatusCode(status.Code(err)) {
			log.Printf("[INFO] Retrying to overwrite tco-policies-logs list: %s", protojson.Format(overwriteResp))
			overwriteResp, err = r.client.OverwriteTCOLogsPolicies(ctx, overwriteReq)
			continue
		}
		resp.Diagnostics.AddError(
			"Error overwriting tco-policies-logs",
			formatRpcErrors(err, overrideTCOPoliciesLogsURL, protojson.Format(overwriteReq)),
		)
		return
	}
	log.Printf("[INFO] Submitted tco-policies-logs list: %s", protojson.Format(overwriteResp))
	state, diags := flattenOverwriteTCOPoliciesLogsList(ctx, overwriteResp)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// Set state to fully populated data
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *TCOPoliciesLogsResource) Read(ctx context.Context, _ resource.ReadRequest, resp *resource.ReadResponse) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	getPoliciesReq := &cxsdk.GetCompanyPoliciesRequest{SourceType: &logSource}
	log.Printf("[INFO] Reading tco-policies-logs")
	getPoliciesResp, err := r.client.List(ctx, getPoliciesReq)
	for err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if retryableStatusCode(status.Code(err)) {
			log.Print("[INFO] Retrying to read tco-policies-logs")
			getPoliciesResp, err = r.client.List(ctx, getPoliciesReq)
			continue
		}
		resp.Diagnostics.AddError(
			"Error reading tco-policies",
			formatRpcErrors(err, getCompanyPoliciesURL, protojson.Format(getPoliciesReq)),
		)
		return
	}
	log.Printf("[INFO] Received tco-policies-logs: %s", protojson.Format(getPoliciesResp))

	state, diags := flattenGetTCOPoliciesLogsList(ctx, getPoliciesResp)
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

	overwriteReq, diags := extractOverwriteTcoPoliciesLogs(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	log.Printf("[INFO] Overwriting tco-policies-logs list: %s", protojson.Format(overwriteReq))
	overwriteResp, err := r.client.OverwriteTCOLogsPolicies(ctx, overwriteReq)
	for err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if retryableStatusCode(status.Code(err)) {
			log.Printf("[INFO] Retrying to overwrite tco-policies-logs list: %s", protojson.Format(overwriteResp))
			overwriteResp, err = r.client.OverwriteTCOLogsPolicies(ctx, overwriteReq)
			continue
		}
		resp.Diagnostics.AddError(
			"Error overwriting tco-policies-logs",
			formatRpcErrors(err, overrideTCOPoliciesLogsURL, protojson.Format(overwriteReq)),
		)
		return
	}
	log.Printf("[INFO] Submitted tco-policies-logs list: %s", protojson.Format(overwriteResp))
	state, diags := flattenOverwriteTCOPoliciesLogsList(ctx, overwriteResp)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// Set state to fully populated data
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *TCOPoliciesLogsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	overwriteReq := &cxsdk.AtomicOverwriteLogPoliciesRequest{}
	log.Printf("[INFO] Overwriting tco-policies-logs list: %s", protojson.Format(overwriteReq))
	overwriteResp, err := r.client.OverwriteTCOLogsPolicies(ctx, overwriteReq)
	for err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if retryableStatusCode(status.Code(err)) {
			log.Printf("[INFO] Retrying to overwrite tco-policies-logs list: %s", protojson.Format(overwriteResp))
			overwriteResp, err = r.client.OverwriteTCOLogsPolicies(ctx, overwriteReq)
			continue
		}
		resp.Diagnostics.AddError(
			"Error overwriting tco-policies-logs",
			formatRpcErrors(err, overrideTCOPoliciesLogsURL, protojson.Format(overwriteReq)),
		)
		return
	}
	log.Printf("[INFO] Submitted tco-policies-logs list: %s", protojson.Format(overwriteResp))
	state, diags := flattenOverwriteTCOPoliciesLogsList(ctx, overwriteResp)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// Set state to fully populated data
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func flattenOverwriteTCOPoliciesLogsList(ctx context.Context, overwriteResp *cxsdk.AtomicOverwriteLogPoliciesResponse) (*TCOPoliciesListModel, diag.Diagnostics) {
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

func flattenGetTCOPoliciesLogsList(ctx context.Context, getResp *cxsdk.GetCompanyPoliciesResponse) (*TCOPoliciesListModel, diag.Diagnostics) {
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

func flattenTCOLogsPolicy(ctx context.Context, policy *cxsdk.TCOPolicy) (*TCOPolicyLogsModel, diag.Diagnostics) {
	logRules := policy.GetSourceTypeRules().(*cxsdk.TCOPolicyLogRules).LogRules
	applications, diags := flattenTCOPolicyRule(ctx, policy.GetApplicationRule())
	if diags.HasError() {
		return nil, diags
	}
	subsystems, diags := flattenTCOPolicyRule(ctx, policy.GetSubsystemRule())
	if diags.HasError() {
		return nil, diags
	}

	return &TCOPolicyLogsModel{
		ID:                 types.StringValue(policy.GetId().GetValue()),
		Name:               types.StringValue(policy.GetName().GetValue()),
		Description:        types.StringValue(policy.GetDescription().GetValue()),
		Enabled:            types.BoolValue(policy.GetEnabled().GetValue()),
		Order:              types.Int64Value(int64(policy.GetOrder().GetValue())),
		Priority:           types.StringValue(tcoPoliciesPriorityProtoToSchema[policy.GetPriority()]),
		Applications:       applications,
		Subsystems:         subsystems,
		ArchiveRetentionID: flattenArchiveRetention(policy.GetArchiveRetention()),
		Severities:         flattenTCOPolicySeverities(logRules.GetSeverities()),
	}, nil
}

func policiesLogsAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"id":                   types.StringType,
		"name":                 types.StringType,
		"description":          types.StringType,
		"enabled":              types.BoolType,
		"order":                types.Int64Type,
		"priority":             types.StringType,
		"applications":         types.ObjectType{AttrTypes: tcoPolicyRuleAttributes()},
		"subsystems":           types.ObjectType{AttrTypes: tcoPolicyRuleAttributes()},
		"severities":           types.SetType{ElemType: types.StringType},
		"archive_retention_id": types.StringType,
	}
}

func flattenTCOPolicyRule(ctx context.Context, rule *cxsdk.TCOPolicyRule) (types.Object, diag.Diagnostics) {
	if rule == nil {
		return types.ObjectNull(tcoPolicyRuleAttributes()), nil
	}

	ruleType := types.StringValue(tcoPoliciesRuleTypeProtoToSchema[rule.GetRuleTypeId()])
	names := strings.Split(rule.GetName().GetValue(), ",")
	namesSet := stringSliceToTypeStringSet(names)
	tcoModel := &TCORuleModel{
		RuleType: ruleType,
		Names:    namesSet,
	}

	return types.ObjectValueFrom(ctx, tcoPolicyRuleAttributes(), tcoModel)
}

func extractOverwriteTcoPoliciesLogs(ctx context.Context, plan *TCOPoliciesListModel) (*cxsdk.AtomicOverwriteLogPoliciesRequest, diag.Diagnostics) {
	var policies []*cxsdk.CreateLogPolicyRequest
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
		createPolicyRequest, dgs := extractTcoPolicyLog(ctx, tcoPolicy)
		if dgs.HasError() {
			diags.Append(dgs...)
			continue
		}
		policies = append(policies, createPolicyRequest)
	}

	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.AtomicOverwriteLogPoliciesRequest{Policies: policies}, nil
}

func extractTcoPolicyLog(ctx context.Context, plan TCOPolicyLogsModel) (*cxsdk.CreateLogPolicyRequest, diag.Diagnostics) {
	name := typeStringToWrapperspbString(plan.Name)
	description := typeStringToWrapperspbString(plan.Description)
	priority := tcoPoliciesPrioritySchemaToProto[plan.Priority.ValueString()]
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

	return &cxsdk.CreateLogPolicyRequest{
		Policy: &cxsdk.CreateGenericPolicyRequest{
			Name:             name,
			Description:      description,
			Priority:         priority,
			ApplicationRule:  applicationRule,
			SubsystemRule:    subsystemRule,
			ArchiveRetention: archiveRetention,
		},
		LogRules: &cxsdk.TCOLogRules{
			Severities: severities,
		},
	}, nil
}

func expandTCOPolicyRule(ctx context.Context, rule types.Object) (*cxsdk.TCOPolicyRule, diag.Diagnostics) {
	if rule.IsNull() || rule.IsUnknown() {
		return nil, nil
	}

	tcoRuleModel := &TCORuleModel{}
	diags := rule.As(ctx, tcoRuleModel, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	ruleType := tcoPoliciesRuleTypeSchemaToProto[tcoRuleModel.RuleType.ValueString()]
	names, diags := typeStringSliceToStringSlice(ctx, tcoRuleModel.Names.Elements())
	if diags.HasError() {
		return nil, diags
	}
	nameStr := wrapperspb.String(strings.Join(names, ","))

	return &cxsdk.TCOPolicyRule{
		RuleTypeId: ruleType,
		Name:       nameStr,
	}, nil
}

func expandActiveRetention(archiveRetention types.String) *cxsdk.ArchiveRetention {
	if archiveRetention.IsNull() {
		return nil
	}

	return &cxsdk.ArchiveRetention{
		Id: wrapperspb.String(archiveRetention.ValueString()),
	}
}

func expandTCOPolicySeverities(ctx context.Context, severities []attr.Value) ([]cxsdk.TCOPolicySeverity, diag.Diagnostics) {
	result := make([]cxsdk.TCOPolicySeverity, 0, len(severities))
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
		s := tcoPolicySeveritySchemaToProto[str]
		result = append(result, s)
	}
	return result, diags
}

func flattenArchiveRetention(archiveRetention *cxsdk.ArchiveRetention) types.String {
	if archiveRetention == nil || archiveRetention.Id == nil {
		return types.StringNull()
	}

	return types.StringValue(archiveRetention.GetId().GetValue())
}

func flattenTCOPolicySeverities(severities []cxsdk.TCOPolicySeverity) types.Set {
	if len(severities) == 0 {
		return types.SetNull(types.StringType)
	}

	elements := make([]attr.Value, 0, len(severities))
	for _, severity := range severities {
		elements = append(elements, types.StringValue(tcoPolicySeverityProtoToSchema[severity]))
	}
	return types.SetValueMust(types.StringType, elements)
}
