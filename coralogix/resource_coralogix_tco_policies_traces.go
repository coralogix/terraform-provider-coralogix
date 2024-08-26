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
	"regexp"
	"strings"
	"time"

	"terraform-provider-coralogix/coralogix/clientset"
	tcopolicies "terraform-provider-coralogix/coralogix/clientset/grpc/tco-policies"

	"github.com/hashicorp/terraform-plugin-framework-validators/mapvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/grpc/status"
)

var (
	_                            resource.ResourceWithConfigure      = &TCOPoliciesTracesResource{}
	_                            resource.ResourceWithValidateConfig = &TCOPoliciesTracesResource{}
	_                            resource.ResourceWithImportState    = &TCOPoliciesTracesResource{}
	tracesSource                                                     = tcopolicies.SourceType_SOURCE_TYPE_SPANS
	overrideTCOPoliciesTracesURL                                     = "com.coralogix.quota.v1.PoliciesService/AtomicOverwriteSpanPolicies"
)

func NewTCOPoliciesTracesResource() resource.Resource {
	return &TCOPoliciesTracesResource{}
}

type TCOPoliciesTracesResource struct {
	client *clientset.TCOPoliciesClient
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
						"applications": schema.SingleNestedAttribute{
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
							MarkdownDescription: "The subsystems to apply the policy on. Applies the policy on all the subsystems by default.",
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
								mapvalidator.KeysAre(stringvalidator.RegexMatches(regexp.MustCompile("tags.*"), "tag names must have 'tags.' prefix")),
							},
							MarkdownDescription: "The subsystems to apply the policy on. Applies the policy on all the subsystems by default.",
						},
					},
				},
				Required: true,
			},
		},
		MarkdownDescription: "Coralogix TCO-Policies-List. For more information - https://coralogix.com/docs/tco-optimizer-api",
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

	overwriteReq, diags := extractOverwriteTcoPoliciesTraces(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	log.Printf("[INFO] Overwriting tco-policies-traces list: %s", protojson.Format(overwriteReq))
	overwriteResp, err := r.client.OverwriteTCOTracesPolicies(ctx, overwriteReq)
	for err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if retryableStatusCode(status.Code(err)) {
			log.Printf("[INFO] Retrying to overwrite tco-policies-traces list: %s", protojson.Format(overwriteResp))
			overwriteResp, err = r.client.OverwriteTCOTracesPolicies(ctx, overwriteReq)
			continue
		}
		resp.Diagnostics.AddError(
			"Error overwriting tco-policies-traces",
			formatRpcErrors(err, overrideTCOPoliciesLogsURL, protojson.Format(overwriteReq)),
		)
		return
	}
	log.Printf("[INFO] Submitted tco-policies-traces list: %s", protojson.Format(overwriteResp))
	state, diags := flattenOverwriteTCOPoliciesTracesList(ctx, overwriteResp)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// Set state to fully populated data
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *TCOPoliciesTracesResource) Read(ctx context.Context, _ resource.ReadRequest, resp *resource.ReadResponse) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	getPoliciesReq := &tcopolicies.GetCompanyPoliciesRequest{SourceType: &tracesSource}
	log.Printf("[INFO] Reading tco-policies-traces")
	getPoliciesResp, err := r.client.GetTCOPolicies(ctx, getPoliciesReq)
	for err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if retryableStatusCode(status.Code(err)) {
			log.Print("[INFO] Retrying to read tco-policies-traces")
			getPoliciesResp, err = r.client.GetTCOPolicies(ctx, getPoliciesReq)
			continue
		}
		resp.Diagnostics.AddError(
			"Error reading tco-policies",
			formatRpcErrors(err, getCompanyPoliciesURL, protojson.Format(getPoliciesReq)),
		)
		return
	}
	log.Printf("[INFO] Received tco-policies-traces: %s", protojson.Format(getPoliciesResp))

	state, diags := flattenGetTCOTracesPoliciesList(ctx, getPoliciesResp)
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

	overwriteReq, diags := extractOverwriteTcoPoliciesTraces(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	log.Printf("[INFO] Overwriting tco-policies-traces list: %s", protojson.Format(overwriteReq))
	overwriteResp, err := r.client.OverwriteTCOTracesPolicies(ctx, overwriteReq)
	for err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if retryableStatusCode(status.Code(err)) {
			log.Printf("[INFO] Retrying to overwrite tco-policies-traces list: %s", protojson.Format(overwriteResp))
			overwriteResp, err = r.client.OverwriteTCOTracesPolicies(ctx, overwriteReq)
			continue
		}
		resp.Diagnostics.AddError(
			"Error overwriting tco-policies-traces",
			formatRpcErrors(err, overrideTCOPoliciesLogsURL, protojson.Format(overwriteReq)),
		)
		return
	}
	log.Printf("[INFO] Submitted tco-policies-traces list: %s", protojson.Format(overwriteResp))
	state, diags := flattenOverwriteTCOPoliciesTracesList(ctx, overwriteResp)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// Set state to fully populated data
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *TCOPoliciesTracesResource) Delete(ctx context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	overwriteReq := &tcopolicies.AtomicOverwriteSpanPoliciesRequest{}
	log.Printf("[INFO] Overwriting tco-policies-traces list: %s", protojson.Format(overwriteReq))
	overwriteResp, err := r.client.OverwriteTCOTracesPolicies(ctx, overwriteReq)
	for err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if retryableStatusCode(status.Code(err)) {
			log.Printf("[INFO] Retrying to overwrite tco-policies-traces list: %s", protojson.Format(overwriteResp))
			overwriteResp, err = r.client.OverwriteTCOTracesPolicies(ctx, overwriteReq)
			continue
		}
		resp.Diagnostics.AddError(
			"Error overwriting tco-policies-traces",
			formatRpcErrors(err, overrideTCOPoliciesLogsURL, protojson.Format(overwriteReq)),
		)
		return
	}
	log.Printf("[INFO] Submitted tco-policies-traces list: %s", protojson.Format(overwriteResp))
	state, diags := flattenOverwriteTCOPoliciesTracesList(ctx, overwriteResp)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// Set state to fully populated data
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func extractOverwriteTcoPoliciesTraces(ctx context.Context, plan *TCOPoliciesListModel) (*tcopolicies.AtomicOverwriteSpanPoliciesRequest, diag.Diagnostics) {
	var policies []*tcopolicies.CreateSpanPolicyRequest
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
		policies = append(policies, createPolicyRequest)
	}

	if diags.HasError() {
		return nil, diags
	}

	return &tcopolicies.AtomicOverwriteSpanPoliciesRequest{Policies: policies}, nil
}

func extractTcoPolicyTraces(ctx context.Context, plan TCOPolicyTracesModel) (*tcopolicies.CreateSpanPolicyRequest, diag.Diagnostics) {
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
	services, diags := expandTCOPolicyRule(ctx, plan.Services)
	if diags.HasError() {
		return nil, diags
	}
	actions, diags := expandTCOPolicyRule(ctx, plan.Actions)
	if diags.HasError() {
		return nil, diags
	}
	archiveRetention := expandActiveRetention(plan.ArchiveRetentionID)
	tagRules, diags := expandTagsRules(ctx, plan.Tags)
	if diags.HasError() {
		return nil, diags
	}

	return &tcopolicies.CreateSpanPolicyRequest{
		Policy: &tcopolicies.CreateGenericPolicyRequest{
			Name:             name,
			Description:      description,
			Priority:         priority,
			ApplicationRule:  applicationRule,
			SubsystemRule:    subsystemRule,
			ArchiveRetention: archiveRetention,
		},
		SpanRules: &tcopolicies.SpanRules{
			ServiceRule: services,
			ActionRule:  actions,
			TagRules:    tagRules,
		},
	}, nil
}

func flattenOverwriteTCOPoliciesTracesList(ctx context.Context, overwriteResp *tcopolicies.AtomicOverwriteSpanPoliciesResponse) (*TCOPoliciesListModel, diag.Diagnostics) {
	var policies []*TCOPolicyTracesModel
	var diags diag.Diagnostics
	for _, policy := range overwriteResp.GetCreateResponses() {
		tcoPolicy, dgs := flattenTCOTracesPolicy(ctx, policy.GetPolicy())
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
	}
}

func flattenGetTCOTracesPoliciesList(ctx context.Context, resp *tcopolicies.GetCompanyPoliciesResponse) (TCOPoliciesListModel, diag.Diagnostics) {
	var policies []*TCOPolicyTracesModel
	var diags diag.Diagnostics
	for _, policy := range resp.GetPolicies() {
		tcoPolicy, dgs := flattenTCOTracesPolicy(ctx, policy)
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

func flattenTCOTracesPolicy(ctx context.Context, policy *tcopolicies.Policy) (*TCOPolicyTracesModel, diag.Diagnostics) {
	traceRules := policy.GetSourceTypeRules().(*tcopolicies.Policy_SpanRules).SpanRules
	applications, diags := flattenTCOPolicyRule(ctx, policy.GetApplicationRule())
	if diags.HasError() {
		return nil, diags
	}
	subsystems, diags := flattenTCOPolicyRule(ctx, policy.GetSubsystemRule())
	if diags.HasError() {
		return nil, diags
	}
	services, diags := flattenTCOPolicyRule(ctx, traceRules.GetServiceRule())
	if diags.HasError() {
		return nil, diags
	}
	actions, diags := flattenTCOPolicyRule(ctx, traceRules.GetActionRule())
	if diags.HasError() {
		return nil, diags
	}

	return &TCOPolicyTracesModel{
		ID:                 types.StringValue(policy.GetId().GetValue()),
		Name:               types.StringValue(policy.GetName().GetValue()),
		Description:        types.StringValue(policy.GetDescription().GetValue()),
		Enabled:            types.BoolValue(policy.GetEnabled().GetValue()),
		Order:              types.Int64Value(int64(policy.GetOrder().GetValue())),
		Priority:           types.StringValue(tcoPoliciesPriorityProtoToSchema[policy.GetPriority()]),
		Applications:       applications,
		Subsystems:         subsystems,
		ArchiveRetentionID: flattenArchiveRetention(policy.GetArchiveRetention()),
		Services:           services,
		Actions:            actions,
		Tags:               flattenTCOPolicyTags(ctx, traceRules.GetTagRules()),
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
			fmt.Sprintf("Currently, rule_type \"%s\" is supportred with only one value, but \"names\" includes %d elements.", ruleType, nameLength),
		)
	}
}

func flattenTCOPolicyTags(ctx context.Context, tags []*tcopolicies.TagRule) types.Map {
	if len(tags) == 0 {
		return types.MapNull(types.ObjectType{AttrTypes: tcoRuleModelAttr()})
	}

	elements := make(map[string]attr.Value)
	for _, tag := range tags {
		name := tag.GetTagName().GetValue()

		ruleType := types.StringValue(tcoPoliciesRuleTypeProtoToSchema[tag.GetRuleTypeId()])

		values := strings.Split(tag.GetTagValue().GetValue(), ",")
		valuesSet := stringSliceToTypeStringSet(values)

		tagRule := TCORuleModel{RuleType: ruleType, Names: valuesSet}

		element, _ := types.ObjectValueFrom(ctx, tcoRuleModelAttr(), tagRule)
		elements[name] = element
	}

	return types.MapValueMust(types.ObjectType{AttrTypes: tcoRuleModelAttr()}, elements)
}

func expandTagsRules(ctx context.Context, tags types.Map) ([]*tcopolicies.TagRule, diag.Diagnostics) {
	var tagsMap map[string]types.Object
	d := tags.ElementsAs(ctx, &tagsMap, true)
	if d != nil {
		panic(d)
	}

	var diags diag.Diagnostics
	result := make([]*tcopolicies.TagRule, 0, len(tagsMap))
	for tagName, tagElement := range tagsMap {
		tagRule, digs := expandTagRule(ctx, tagName, tagElement)
		if digs.HasError() {
			diags.Append(digs...)
			continue
		}
		result = append(result, tagRule)
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

func expandTagRule(ctx context.Context, name string, tag types.Object) (*tcopolicies.TagRule, diag.Diagnostics) {
	rule, diags := expandTCOPolicyRule(ctx, tag)
	if diags.HasError() {
		return nil, diags
	}
	return &tcopolicies.TagRule{
		TagName:    wrapperspb.String(name),
		RuleTypeId: rule.GetRuleTypeId(),
		TagValue:   rule.GetName(),
	}, nil
}
