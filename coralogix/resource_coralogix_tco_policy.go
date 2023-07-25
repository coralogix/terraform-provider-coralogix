package coralogix

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/golang/protobuf/jsonpb"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"terraform-provider-coralogix/coralogix/clientset"
	tcopolicies "terraform-provider-coralogix/coralogix/clientset/grpc/tco-policies"
)

var (
	_                                resource.ResourceWithConfigure   = &ActionResource{}
	_                                resource.ResourceWithImportState = &ActionResource{}
	tcoPoliciesPrioritySchemaToProto                                  = map[string]tcopolicies.Priority{
		"Block":  tcopolicies.Priority_PRIORITY_TYPE_BLOCK,
		"High":   tcopolicies.Priority_PRIORITY_TYPE_HIGH,
		"Low":    tcopolicies.Priority_PRIORITY_TYPE_LOW,
		"Medium": tcopolicies.Priority_PRIORITY_TYPE_MEDIUM,
	}
	tcoPoliciesPriorityProtoToSchema = ReverseMap(tcoPoliciesPrioritySchemaToProto)
	tcoPoliciesValidPriorities       = GetKeys(tcoPoliciesPrioritySchemaToProto)
	tcoPoliciesRuleTypeSchemaToProto = map[string]tcopolicies.RuleTypeId{
		"Is":          tcopolicies.RuleTypeId_RULE_TYPE_ID_IS,
		"Is Not":      tcopolicies.RuleTypeId_RULE_TYPE_ID_IS_NOT,
		"Starts With": tcopolicies.RuleTypeId_RULE_TYPE_ID_START_WITH,
		"Includes":    tcopolicies.RuleTypeId_RULE_TYPE_ID_INCLUDES,
	}
	tcoPoliciesRuleTypeProtoToSchema = ReverseMap(tcoPoliciesRuleTypeSchemaToProto)
	tcoPoliciesValidRuleTypes        = GetKeys(tcoPoliciesRuleTypeSchemaToProto)
	tcoPolicySeveritySchemaToProto   = map[string]tcopolicies.Severity{
		"Debug":    tcopolicies.Severity_SEVERITY_DEBUG,
		"Verbose":  tcopolicies.Severity_SEVERITY_VERBOSE,
		"Info":     tcopolicies.Severity_SEVERITY_INFO,
		"Warning":  tcopolicies.Severity_SEVERITY_WARNING,
		"Error":    tcopolicies.Severity_SEVERITY_ERROR,
		"Critical": tcopolicies.Severity_SEVERITY_CRITICAL,
	}
	tcoPolicySeverityProtoToSchema   = ReverseMap(tcoPolicySeveritySchemaToProto)
	validPolicySeverities            = GetKeys(tcoPolicySeveritySchemaToProto)
	tcoPolicySourceTypeSchemaToProto = map[string]tcopolicies.SourceType{
		"Logs":   tcopolicies.SourceType_SOURCE_TYPE_LOGS,
		"Traces": tcopolicies.SourceType_SOURCE_TYPE_SPANS,
	}
)

func NewTCOPolicyResource() resource.Resource {
	return &TCOPolicyResource{}
}

type TCOPolicyResource struct {
	client *clientset.TCOPoliciesClient
}

func (t *TCOPolicyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tco_policy"
}

func (t *TCOPolicyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	t.client = clientSet.TCOPolicies()
}

func (t *TCOPolicyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version: 1,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
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
				Optional: true,
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
				Required: true,
				Validators: []validator.Int64{
					int64validator.AtLeast(1),
				},
				Description: "Determines the policy's order between the other policies. Currently, will be computed by creation order.",
			},
			"archive_retention_id": schema.StringAttribute{
				Optional:    true,
				Description: "Allowing logs with a specific retention to be tagged.",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"source_type": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.OneOf("Logs", "Traces"),
				},
			},
			"severities": schema.SetAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
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
						},
					},
					"rule_type": schema.StringAttribute{
						Optional: true,
						Computed: true,
						Default:  stringdefault.StaticString("Is"),
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
						Default:  stringdefault.StaticString("Is"),
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
						Default:  stringdefault.StaticString("Is"),
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
						Default:  stringdefault.StaticString("Is"),
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
							Default:  stringdefault.StaticString("Is"),
							Validators: []validator.String{
								stringvalidator.OneOf(tcoPoliciesValidRuleTypes...),
							},
						},
					},
				},
				MarkdownDescription: "The subsystems to apply the policy on. Applies the policy on all the subsystems by default.",
			},
		},
		MarkdownDescription: "Coralogix action. For more info please review - https://coralogix.com/docs/coralogix-action-extension/.",
	}
}

func (t *TCOPolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)

}

func (t *TCOPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	jsm := &jsonpb.Marshaler{}
	var plan TCOPolicyResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	createPolicyRequest := extractCreateTcoPolicy(ctx, plan)
	policyStr, _ := jsm.MarshalToString(createPolicyRequest)
	log.Printf("[INFO] Creating new tco-policy: %s", policyStr)
	createResp, err := t.client.CreateTCOPolicy(ctx, createPolicyRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		resp.Diagnostics.AddError(
			"Error creating tco-policy",
			"Could not create tco-policy, unexpected error: "+err.Error(),
		)
		return
	}
	policy := createResp.GetPolicy()
	policyStr, _ = jsm.MarshalToString(policy)
	log.Printf("[INFO] Submitted new tco-policy: %#v", policy)

	t.updatePoliciesOrder(ctx, plan)

	plan = flattenTCOPolicy(ctx, policy)

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func flattenTCOPolicy(ctx context.Context, policy *tcopolicies.Policy) TCOPolicyResourceModel {
	tcoPolicy := TCOPolicyResourceModel{
		ID:                 types.StringValue(policy.GetId().GetValue()),
		Name:               types.StringValue(policy.GetName().GetValue()),
		Description:        types.StringValue(policy.GetDescription().GetValue()),
		Enabled:            types.BoolValue(policy.GetEnabled().GetValue()),
		Order:              types.Int64Value(int64(policy.GetOrder().GetValue())),
		Priority:           types.StringValue(tcoPoliciesPriorityProtoToSchema[policy.GetPriority()]),
		Applications:       flattenTCOPolicyRule(policy.GetApplicationRule()),
		Subsystems:         flattenTCOPolicyRule(policy.GetSubsystemRule()),
		ArchiveRetentionID: flattenArchiveRetention(policy.GetArchiveRetention()),
	}

	tcoPolicy = flattenTcoPolicySourceType(ctx, policy, &tcoPolicy)

	return tcoPolicy
}

func flattenTcoPolicySourceType(ctx context.Context, policy *tcopolicies.Policy, policyModel *TCOPolicyResourceModel) TCOPolicyResourceModel {
	switch sourceType := policy.GetSourceTypeRules().(type) {
	case *tcopolicies.Policy_LogRules:
		logRules := sourceType.LogRules
		policyModel.Severities = flattenTCOPolicySeverities(logRules.GetSeverities())
	case *tcopolicies.Policy_SpanRules:
		spanRule := sourceType.SpanRules
		policyModel.Actions = flattenTCOPolicyRule(spanRule.GetActionRule())
		policyModel.Services = flattenTCOPolicyRule(spanRule.GetServiceRule())
		policyModel.Tags = flattenTCOPolicyTags(ctx, spanRule.GetTagRules())
	}

	return *policyModel
}

func flattenTCOPolicyTags(ctx context.Context, tags []*tcopolicies.TagRule) types.Map {
	if len(tags) == 0 {
		return types.MapNull(types.StringType)
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

	types.MapValueMust(types.ObjectType{AttrTypes: tcoRuleModelAttr()}, elements)

	return types.MapValueMust(types.StringType, elements)
}

func tcoRuleModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"rule_type": types.StringType,
		"names": types.SetType{
			ElemType: types.StringType,
		},
	}
}

func flattenTCOPolicyRule(rule *tcopolicies.Rule) *TCORuleModel {
	if rule == nil {
		return nil
	}

	ruleType := types.StringValue(tcoPoliciesRuleTypeProtoToSchema[rule.GetRuleTypeId()])

	names := strings.Split(rule.GetName().GetValue(), ",")
	namesSet := stringSliceToTypeStringSet(names)

	return &TCORuleModel{
		RuleType: ruleType,
		Names:    namesSet,
	}
}

func (t *TCOPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state TCOPolicyResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	//Get refreshed tco-policy value from Coralogix
	id := state.ID.ValueString()
	log.Printf("[INFO] Reading tco-policy: %s", id)
	getPolicyResp, err := t.client.GetTCOPolicy(ctx, &tcopolicies.GetPolicyRequest{Id: wrapperspb.String(id)})
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		if status.Code(err) == codes.NotFound {
			state.ID = types.StringNull()
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("tco-policy %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error reading tco-policy",
				handleRpcErrorNewFramework(err, "tco-policy"),
			)
		}
		return
	}
	policy := getPolicyResp.GetPolicy()
	log.Printf("[INFO] Received tco-policy: %#v", policy)

	state = flattenTCOPolicy(ctx, policy)
	//
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (t TCOPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var plan TCOPolicyResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	policyUpdateReq := extractUpdateTCOPolicy(ctx, plan)
	log.Printf("[INFO] Updating tco-policy: %#v", policyUpdateReq)
	policyUpdateResp, err := t.client.UpdateTCOPolicy(ctx, policyUpdateReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		resp.Diagnostics.AddError(
			"Error updating tco-policy",
			"Could not update Action, unexpected error: "+err.Error(),
		)
		return
	}
	log.Printf("[INFO] Submitted updated tco-policy: %#v", policyUpdateResp)

	t.updatePoliciesOrder(ctx, plan)

	// Get refreshed Action value from Coralogix
	id := plan.ID.ValueString()
	getPolicyResp, err := t.client.GetTCOPolicy(ctx, &tcopolicies.GetPolicyRequest{Id: wrapperspb.String(id)})
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		if status.Code(err) == codes.NotFound {
			plan.ID = types.StringNull()
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("Action %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error reading Action",
				handleRpcErrorNewFramework(err, "Action"),
			)
		}
		return
	}
	log.Printf("[INFO] Received tco-policy: %#v", getPolicyResp)

	plan = flattenTCOPolicy(ctx, getPolicyResp.GetPolicy())

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func extractUpdateTCOPolicy(ctx context.Context, plan TCOPolicyResourceModel) *tcopolicies.UpdatePolicyRequest {
	name := typeStringToWrapperspbString(plan.Name)
	description := typeStringToWrapperspbString(plan.Description)
	priority := tcoPoliciesPrioritySchemaToProto[plan.Priority.ValueString()]
	applicationRule := expandTCOPolicyRule(ctx, plan.Applications)
	subsystemRule := expandTCOPolicyRule(ctx, plan.Subsystems)
	archiveRetention := expandActiveRetention(plan.ArchiveRetentionID)

	updateRequest := &tcopolicies.UpdatePolicyRequest{
		Name:             name,
		Description:      description,
		Priority:         priority,
		ApplicationRule:  applicationRule,
		SubsystemRule:    subsystemRule,
		ArchiveRetention: archiveRetention,
	}

	updateRequest = expandSourceTypeRulesUpdate(ctx, plan, updateRequest)
	return updateRequest
}

func (t TCOPolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ActionResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	log.Printf("[INFO] Deleting tco-policy %s\n", id)
	if _, err := t.client.DeleteTCOPolicy(ctx, &tcopolicies.DeletePolicyRequest{Id: wrapperspb.String(id)}); err != nil {
		resp.Diagnostics.AddError(
			fmt.Sprintf("Error Deleting tco-policy %s", state.ID.ValueString()),
			handleRpcErrorNewFramework(err, "tco-policy"),
		)
		return
	}
	log.Printf("[INFO] tco-policy %s deleted\n", id)
}

type TCOPolicyResourceModel struct {
	ID                 types.String  `tfsdk:"id"`
	Name               types.String  `tfsdk:"name"`
	Description        types.String  `tfsdk:"description"`
	Enabled            types.Bool    `tfsdk:"enabled"`
	Order              types.Int64   `tfsdk:"order"`
	Priority           types.String  `tfsdk:"priority"`
	Applications       *TCORuleModel `tfsdk:"applications"`
	Subsystems         *TCORuleModel `tfsdk:"subsystems"`
	ArchiveRetentionID types.String  `tfsdk:"archive_retention_id"`
	SourceType         types.String  `tfsdk:"source_type"`
	Severities         types.Set     `tfsdk:"severities"`
	Services           *TCORuleModel `tfsdk:"services"`
	Actions            *TCORuleModel `tfsdk:"actions"`
	Tags               types.Map     `tfsdk:"tags"`
}

type TCORuleModel struct {
	RuleType types.String `tfsdk:"rule_type"`
	Names    types.Set    `tfsdk:"names"`
}

func extractCreateTcoPolicy(ctx context.Context, plan TCOPolicyResourceModel) *tcopolicies.CreatePolicyRequest {
	name := typeStringToWrapperspbString(plan.Name)
	description := typeStringToWrapperspbString(plan.Description)
	priority := tcoPoliciesPrioritySchemaToProto[plan.Priority.ValueString()]
	applicationRule := expandTCOPolicyRule(ctx, plan.Applications)
	subsystemRule := expandTCOPolicyRule(ctx, plan.Subsystems)
	archiveRetention := expandActiveRetention(plan.ArchiveRetentionID)

	createRequest := &tcopolicies.CreatePolicyRequest{
		Name:             name,
		Description:      description,
		Priority:         priority,
		ApplicationRule:  applicationRule,
		SubsystemRule:    subsystemRule,
		ArchiveRetention: archiveRetention,
	}

	createRequest = expandSourceTypeRules(ctx, plan, createRequest)
	return createRequest
}

func expandSourceTypeRules(ctx context.Context, plan TCOPolicyResourceModel, req *tcopolicies.CreatePolicyRequest) *tcopolicies.CreatePolicyRequest {
	switch plan.SourceType.ValueString() {
	case "Logs":
		req.SourceTypeRules = expandLogsSourceType(plan)
	case "Spans":
		req.SourceTypeRules = expandSpansSourceType(ctx, plan)
	}

	return req
}

func expandLogsSourceType(plan TCOPolicyResourceModel) *tcopolicies.CreatePolicyRequest_LogRules {
	severities := expandTCOPolicySeverities(plan.Severities.Elements())

	return &tcopolicies.CreatePolicyRequest_LogRules{
		LogRules: &tcopolicies.LogRules{
			Severities: severities,
		},
	}
}

func expandSpansSourceType(ctx context.Context, plan TCOPolicyResourceModel) *tcopolicies.CreatePolicyRequest_SpanRules {
	serviceRule := expandTCOPolicyRule(ctx, plan.Services)
	actionRule := expandTCOPolicyRule(ctx, plan.Actions)
	tagRules := expandTagsRules(ctx, plan.Tags)

	return &tcopolicies.CreatePolicyRequest_SpanRules{
		SpanRules: &tcopolicies.SpanRules{
			ServiceRule: serviceRule,
			ActionRule:  actionRule,
			TagRules:    tagRules,
		},
	}
}

func expandTagsRules(ctx context.Context, tags types.Map) []*tcopolicies.TagRule {
	tagsMap := tags.Elements()
	result := make([]*tcopolicies.TagRule, 0, len(tagsMap))

	for tagName, tagElement := range tagsMap {
		tagValue, _ := tagElement.ToTerraformValue(ctx)
		var tag TCORuleModel
		tagValue.As(&tag)
		tagRule := expandTagRule(ctx, tagName, tag)
		result = append(result, tagRule)
	}

	return result
}

func expandTagRule(ctx context.Context, name string, tagRule TCORuleModel) *tcopolicies.TagRule {
	rule := expandTCOPolicyRule(ctx, &tagRule)
	return &tcopolicies.TagRule{
		TagName:    wrapperspb.String(name),
		RuleTypeId: rule.GetRuleTypeId(),
		TagValue:   rule.GetName(),
	}
}

func expandSourceTypeRulesUpdate(ctx context.Context, plan TCOPolicyResourceModel, req *tcopolicies.UpdatePolicyRequest) *tcopolicies.UpdatePolicyRequest {
	switch plan.SourceType.ValueString() {
	case "Logs":
		req.SourceTypeRules = expandLogsSourceTypeUpdate(plan)
	case "Spans":
		req.SourceTypeRules = expandSpansSourceTypeUpdate(ctx, plan)
	}

	return req
}

func expandLogsSourceTypeUpdate(plan TCOPolicyResourceModel) *tcopolicies.UpdatePolicyRequest_LogRules {
	severities := expandTCOPolicySeverities(plan.Severities.Elements())

	return &tcopolicies.UpdatePolicyRequest_LogRules{
		LogRules: &tcopolicies.LogRules{
			Severities: severities,
		},
	}
}

func expandSpansSourceTypeUpdate(ctx context.Context, plan TCOPolicyResourceModel) *tcopolicies.UpdatePolicyRequest_SpanRules {
	return &tcopolicies.UpdatePolicyRequest_SpanRules{}
}

func expandTCOPolicyRule(ctx context.Context, rule *TCORuleModel) *tcopolicies.Rule {
	if rule == nil {
		return nil
	}

	ruleType := tcoPoliciesRuleTypeSchemaToProto[rule.RuleType.ValueString()]

	names := typeStringSliceToStringSlice(ctx, rule.Names.Elements())
	nameStr := wrapperspb.String(strings.Join(names, ","))

	return &tcopolicies.Rule{
		RuleTypeId: ruleType,
		Name:       nameStr,
	}
}

//func resourceCoralogixTCOPolicy() *schema.Resource {
//	return &schema.Resource{
//		CreateContext: resourceCoralogixTCOPolicyCreate,
//		ReadContext:   resourceCoralogixTCOPolicyRead,
//		UpdateContext: resourceCoralogixTCOPolicyUpdate,
//		DeleteContext: resourceCoralogixTCOPolicyDelete,
//
//		Importer: &schema.ResourceImporter{
//			StateContext: schema.ImportStatePassthroughContext,
//		},
//
//		Timeouts: &schema.ResourceTimeout{
//			Create: schema.DefaultTimeout(60 * time.Second),
//			Read:   schema.DefaultTimeout(30 * time.Second),
//			Update: schema.DefaultTimeout(60 * time.Second),
//			Delete: schema.DefaultTimeout(30 * time.Second),
//		},
//
//		Schema: TCOPolicySchema(),
//
//		Description: "Coralogix TCO-Policy. For more information - https://coralogix.com/docs/tco-optimizer-api .",
//	}
//}

//func resourceCoralogixTCOPolicyCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
//	tcoPolicyReq, err := extractTCOPolicyRequest(d)
//	if err != nil {
//		return diag.FromErr(err)
//	}
//	req := tcopolicies.CreatePolicyRequest{
//		Name:,
//	}
//	log.Printf("[INFO] Creating new tco-policy: %#v", tcoPolicyReq)
//	tcoPolicyResp, err := meta.(*clientset.ClientSet).TCOPolicies().CreateTCOPolicy(ctx, tcoPolicyReq)
//	if err != nil {
//		log.Printf("[ERROR] Received error: %#v", err)
//		return handleRpcError(err, "tco-policy")
//	}
//
//	log.Printf("[INFO] Submitted new tco-policy: %#v", tcoPolicyResp)
//
//	var m map[string]interface{}
//	if err = json.Unmarshal([]byte(tcoPolicyResp), &m); err != nil {
//		return diag.FromErr(err)
//	}
//
//	d.SetId(m["id"].(string))
//
//	if err = updatePoliciesOrder(ctx, d, meta); err != nil {
//		return diag.FromErr(err)
//	}
//
//	return resourceCoralogixTCOPolicyRead(ctx, d, meta)
//}

//func updatePoliciesOrder(ctx context.Context, d *schema.ResourceData, meta interface{}) error {
//	tcoPoliciesResp, err := meta.(*clientset.ClientSet).TCOPolicies().GetTCOPolicies(ctx)
//	var policies []map[string]interface{}
//	if err = json.Unmarshal([]byte(tcoPoliciesResp), &policies); err != nil {
//		return err
//	}
//
//	policiesOrders := make([]string, len(policies))
//	currentIndex := -1
//	for i, policy := range policies {
//		id := policy["id"].(string)
//		policiesOrders[i] = id
//		if id == d.Id() {
//			currentIndex = i
//		}
//	}
//	desiredIndex := d.Get("order").(int) - 1
//	if desiredIndex >= len(policies) {
//		desiredIndex = len(policies) - 1
//	}
//	if currentIndex == desiredIndex {
//		return nil
//	}
//	policiesOrders[currentIndex], policiesOrders[desiredIndex] = policiesOrders[desiredIndex], policiesOrders[currentIndex]
//
//	reorderRequest, err := json.Marshal(policiesOrders)
//	if _, err = meta.(*clientset.ClientSet).TCOPolicies().ReorderTCOPolicies(ctx, string(reorderRequest)); err != nil {
//		return err
//	}
//
//	return nil
//}

func (t *TCOPolicyResource) updatePoliciesOrder(ctx context.Context, policy TCOPolicyResourceModel) error {
	sourceType := tcoPolicySourceTypeSchemaToProto[policy.SourceType.ValueString()]
	req := &tcopolicies.GetCompanyPoliciesRequest{
		EnabledOnly: wrapperspb.Bool(false),
		SourceType:  &sourceType,
	}
	log.Printf("[INFO] Get tco-policies request: %#v", req)

	tcoPoliciesResp, err := t.client.GetTCOPolicies(ctx, req)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return err
	}

	log.Printf("[INFO] Get tco-policies response: %#v", tcoPoliciesResp)

	policies := tcoPoliciesResp.GetPolicies()
	policiesIDsByOrder, currentPolicyIndex := getPoliciesIDsByOrderAndCurrentPolicyIndex(policies, policy)

	desiredPolicyIndex := getPolicyDesireIndex(policy, policies)

	if currentPolicyIndex == desiredPolicyIndex {
		return nil
	}

	policiesIDsByOrder[currentPolicyIndex].Order, policiesIDsByOrder[desiredPolicyIndex].Order = policiesIDsByOrder[desiredPolicyIndex].Order, policiesIDsByOrder[currentPolicyIndex].Order
	reorderReq := &tcopolicies.ReorderPoliciesRequest{
		Orders:     policiesIDsByOrder,
		SourceType: sourceType,
	}

	log.Printf("[INFO] Reorder tco-policies request: %#v", reorderReq)
	var resp *tcopolicies.ReorderPoliciesResponse
	if resp, err = t.client.ReorderTCOPolicies(ctx, reorderReq); err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return err
	}
	log.Printf("[INFO] Reorder tco-policies response: %#v", resp)

	return nil
}

func getPoliciesIDsByOrderAndCurrentPolicyIndex(policies []*tcopolicies.Policy, policy TCOPolicyResourceModel) ([]*tcopolicies.PolicyOrder, int) {
	policiesIDsByOrder := make([]*tcopolicies.PolicyOrder, len(policies))
	policyID := policy.ID.ValueString()
	currentPolicyIndex := -1
	for i, p := range policies {
		id := p.GetId().GetValue()
		policiesIDsByOrder[i] = &tcopolicies.PolicyOrder{
			Order: wrapperspb.Int32(int32(i + 1)),
			Id:    wrapperspb.String(id),
		}

		if id == policyID {
			currentPolicyIndex = i
		}
	}
	return policiesIDsByOrder, currentPolicyIndex
}

func getPolicyDesireIndex(policy TCOPolicyResourceModel, policies []*tcopolicies.Policy) int {
	desiredPolicyIndex := int(policy.Order.ValueInt64() - 1)
	if desiredPolicyIndex >= len(policies) {
		desiredPolicyIndex = len(policies) - 1
	}
	return desiredPolicyIndex
}

//func resourceCoralogixTCOPolicyRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
//	id := d.Id()
//	log.Printf("[INFO] Reading tco-policy %s", id)
//	tcoPolicyResp, err := meta.(*clientset.ClientSet).TCOPolicies().GetTCOPolicy(ctx, id)
//	if err != nil {
//		log.Printf("[ERROR] Received error: %#v", err)
//		if status.Code(err) == codes.NotFound {
//			d.SetId("")
//			return diag.Diagnostics{diag.Diagnostic{
//				Severity: diag.Warning,
//				Summary:  fmt.Sprintf("Tco-Policy %q is in state, but no longer exists in Coralogix backend", id),
//				Detail:   fmt.Sprintf("%s will be recreated when you apply", id),
//			}}
//		}
//	}
//
//	log.Printf("[INFO] Received tco-policy: %#v", tcoPolicyResp)
//
//	return setTCOPolicy(d, tcoPolicyResp)
//}
//
//func resourceCoralogixTCOPolicyUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
//	tcoPolicyReq, err := extractTCOPolicyRequest(d)
//	if err != nil {
//		return diag.FromErr(err)
//	}
//
//	id := d.Id()
//	log.Printf("[INFO] Updating tco-policy %s to %s", id, tcoPolicyReq)
//	tcoPolicyResp, err := meta.(*clientset.ClientSet).TCOPolicies().UpdateTCOPolicy(ctx, id, tcoPolicyReq)
//	if err != nil {
//		log.Printf("[ERROR] Received error: %#v", err)
//		return handleRpcError(err, "tco-policy")
//	}
//
//	log.Printf("[INFO] Submitted new tco-policy: %#v", tcoPolicyResp)
//
//	var m map[string]interface{}
//	if err = json.Unmarshal([]byte(tcoPolicyResp), &m); err != nil {
//		return diag.FromErr(err)
//	}
//
//	d.SetId(m["id"].(string))
//
//	if err = updatePoliciesOrder(ctx, d, meta); err != nil {
//		return diag.FromErr(err)
//	}
//
//	return resourceCoralogixTCOPolicyRead(ctx, d, meta)
//}

//func resourceCoralogixTCOPolicyDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
//	id := d.Id()
//
//	log.Printf("[INFO] Deleting tco-policy %s", id)
//	req :=
//	err := meta.(*clientset.ClientSet).TCOPolicies().DeleteTCOPolicy(ctx, id)
//	if err != nil {
//		log.Printf("[ERROR] Received error: %#v", err)
//		return handleRpcErrorWithID(err, "tco-policy", id)
//	}
//	log.Printf("[INFO] tco-policy %s deleted", id)
//
//	d.SetId("")
//	return nil
//}

func expandActiveRetention(archiveRetention types.String) *tcopolicies.ArchiveRetention {
	if archiveRetention.IsNull() {
		return nil
	}

	return &tcopolicies.ArchiveRetention{
		Id: wrapperspb.String(archiveRetention.String()),
	}
}

//func extractSourceTypeRules(plan TCOPolicyResourceModel) interface{} {
//
//}
//
//func expandTCOPolicyFilter(v interface{}) *tcoPolicyFilter {
//	l := v.([]interface{})
//	if len(l) == 0 {
//		return nil
//	}
//	m := l[0].(map[string]interface{})
//
//	filterType := expandTcoPolicyFilterType(m)
//	rule := expandTcoPolicyFilterRule(m)
//
//	return &tcoPolicyFilter{
//		Type: filterType,
//		Rule: rule,
//	}
//}

//func expandTcoPolicyFilterRule(m map[string]interface{}) interface{} {
//	if rules, ok := m["rules"]; ok && rules != nil {
//		rulesList := rules.(*schema.Set).List()
//		if len(rulesList) == 0 {
//			return m["rule"].(string)
//		} else {
//			return rulesList
//		}
//	}
//	return m["rule"].(string)
//}
//
//func expandTcoPolicyFilterType(m map[string]interface{}) string {
//	var filterType string
//	if is, ok := m["is"]; ok && is.(bool) {
//		filterType = "Is"
//	} else if isNot, ok := m["is_not"]; ok && isNot.(bool) {
//		filterType = "Is Not"
//	} else if starsWith, ok := m["starts_with"]; ok && starsWith.(bool) {
//		filterType = "Starts With"
//	} else {
//		filterType = "Includes"
//	}
//	return filterType
//}

func expandTCOPolicySeverities(severities []attr.Value) []tcopolicies.Severity {
	result := make([]tcopolicies.Severity, 0, len(severities))
	for _, severity := range severities {
		val, _ := severity.ToTerraformValue(context.Background())
		var str string
		val.As(&str)
		s := tcoPolicySeveritySchemaToProto[str]
		result = append(result, s)
	}
	return result
}

//func setTCOPolicy(d *schema.ResourceData, tcoPolicyResp string) diag.Diagnostics {
//	var m map[string]interface{}
//	if err := json.Unmarshal([]byte(tcoPolicyResp), &m); err != nil {
//		return diag.FromErr(err)
//	}
//
//	var diags diag.Diagnostics
//	if err := d.Set("name", m["name"].(string)); err != nil {
//		diags = append(diags, diag.FromErr(err)...)
//	}
//	if err := d.Set("enabled", m["enabled"].(bool)); err != nil {
//		diags = append(diags, diag.FromErr(err)...)
//	}
//	if err := d.Set("order", int(m["order"].(float64))); err != nil {
//		diags = append(diags, diag.FromErr(err)...)
//	}
//	if err := d.Set("priority", m["priority"].(string)); err != nil {
//		diags = append(diags, diag.FromErr(err)...)
//	}
//	if err := d.Set("severities", flattenTCOPolicySeverities(m["severities"])); err != nil {
//		diags = append(diags, diag.FromErr(err)...)
//	}
//	if err := d.Set("application_name", flattenTCOPolicyFilter(m["applicationName"])); err != nil {
//		diags = append(diags, diag.FromErr(err)...)
//	}
//	if err := d.Set("subsystem_name", flattenTCOPolicyFilter(m["subsystemName"])); err != nil {
//		diags = append(diags, diag.FromErr(err)...)
//	}
//	if err := d.Set("archive_retention_id", flattenArchiveRetention(m["archiveRetention"])); err != nil {
//
//	}
//	return diags
//}

func flattenArchiveRetention(archiveRetention *tcopolicies.ArchiveRetention) types.String {
	if archiveRetention == nil || archiveRetention.Id == nil {
		return types.StringNull()
	}

	return types.StringValue(archiveRetention.GetId().GetValue())
}

func flattenTCOPolicySeverities(severities []tcopolicies.Severity) types.Set {
	if len(severities) == 0 {
		return types.SetNull(types.StringType)
	}

	elements := make([]attr.Value, 0, len(severities))
	for _, severity := range severities {
		elements = append(elements, types.StringValue(tcoPolicySeverityProtoToSchema[severity]))
	}
	return types.SetValueMust(types.StringType, elements)
}

//func flattenTCOPolicyFilter(v interface{}) interface{} {
//	if v == nil {
//		return nil
//	}
//	filter := v.(map[string]interface{})
//
//	filterType := tcoPolicyResponseFilterTypeToTcoPolicySchemaFilterType[filter["type"].(string)]
//	flattenedFilter := map[string]interface{}{
//		filterType: true,
//	}
//
//	if rules, ok := filter["rule"].([]interface{}); ok {
//		flattenedFilter["rules"] = interfaceSliceToStringSlice(rules)
//	} else {
//		flattenedFilter["rule"] = filter["rule"].(string)
//	}
//
//	return []interface{}{flattenedFilter}
//}

//func TCOPolicySchema() map[string]*schema.Schema {
//	return map[string]*schema.Schema{
//		"name": {
//			Type:         schema.TypeString,
//			Required:     true,
//			ValidateFunc: validation.StringIsNotEmpty,
//			Description:  "The policy name. Have to be unique per policy.",
//		},
//		"enabled": {
//			Type:        schema.TypeBool,
//			Optional:    true,
//			Default:     true,
//			Description: "Determines weather the policy will be enabled. True by default.",
//		},
//		"priority": {
//			Type:         schema.TypeString,
//			Required:     true,
//			ValidateFunc: validation.StringInSlice(validPolicyPriorities, false),
//			Description:  fmt.Sprintf("The policy priority. Can be one of %q.", validPolicyPriorities),
//		},
//		"order": {
//			Type:         schema.TypeInt,
//			Required:     true,
//			ValidateFunc: validation.IntAtLeast(1),
//			Description:  "Determines the policy's order between the other policies. Currently, will be computed by creation order.",
//		},
//		"severities": {
//			Type:     schema.TypeSet,
//			Required: true,
//			Elem: &schema.Schema{
//				Type:         schema.TypeString,
//				ValidateFunc: validation.StringInSlice(validPolicySeverities, false),
//			},
//			Set:         schema.HashString,
//			MinItems:    1,
//			Description: fmt.Sprintf("The severities to apply the policy on. Can be few of %q.", validPolicySeverities),
//		},
//		"application_name": {
//			Type:        schema.TypeList,
//			MaxItems:    1,
//			Optional:    true,
//			Elem:        tcoPolicyFiltersSchema("application_name"),
//			Description: "The applications to apply the policy on. Applies the policy on all the applications by default.",
//		},
//		"subsystem_name": {
//			Type:        schema.TypeList,
//			MaxItems:    1,
//			Optional:    true,
//			Elem:        tcoPolicyFiltersSchema("subsystem_name"),
//			Description: "The subsystems to apply the policy on. Applies the policy on all the subsystems by default.",
//		},
//		"archive_retention_id": {
//			Type:         schema.TypeString,
//			Optional:     true,
//			Description:  "Allowing logs with a specific retention to be tagged.",
//			ValidateFunc: validation.StringIsNotEmpty,
//		},
//	}
//}

//func tcoPolicyFiltersSchema(filterName string) *schema.Resource {
//	filterTypesRoutes := filterTypesRoutes(filterName)
//	return &schema.Resource{
//		Schema: map[string]*schema.Schema{
//			"is": {
//				Type:         schema.TypeBool,
//				Optional:     true,
//				ExactlyOneOf: filterTypesRoutes,
//				RequiredWith: []string{fmt.Sprintf("%s.0.rules", filterName)},
//				Description:  "Determines the filter's type. One of is/is_not/starts_with/includes have to be set.",
//			},
//			"is_not": {
//				Type:         schema.TypeBool,
//				Optional:     true,
//				ExactlyOneOf: filterTypesRoutes,
//				RequiredWith: []string{fmt.Sprintf("%s.0.rules", filterName)},
//				Description:  "Determines the filter's type. One of is/is_not/starts_with/includes have to be set.",
//			},
//			"starts_with": {
//				Type:         schema.TypeBool,
//				Optional:     true,
//				ExactlyOneOf: filterTypesRoutes,
//				RequiredWith: []string{fmt.Sprintf("%s.0.rule", filterName)},
//				Description:  "Determines the filter's type. One of is/is_not/starts_with/includes have to be set.",
//			},
//			"includes": {
//				Type:         schema.TypeBool,
//				Optional:     true,
//				ExactlyOneOf: filterTypesRoutes,
//				RequiredWith: []string{fmt.Sprintf("%s.0.rule", filterName)},
//				Description:  "Determines the filter's type. One of is/is_not/starts_with/includes have to be set.",
//			},
//			"rules": {
//				Type:     schema.TypeSet,
//				Optional: true,
//				MinItems: 1,
//				Elem: &schema.Schema{
//					Type: schema.TypeString,
//					Set:  schema.HashString,
//				},
//				ExactlyOneOf: []string{fmt.Sprintf("%s.0.rule", filterName), fmt.Sprintf("%s.0.rules", filterName)},
//				Description:  "Set of rules to apply the filter on. In case of is=true/is_not=true replace to 'rules' (set of strings).",
//			},
//			"rule": {
//				Type:         schema.TypeString,
//				Optional:     true,
//				ExactlyOneOf: []string{fmt.Sprintf("%s.0.rule", filterName), fmt.Sprintf("%s.0.rules", filterName)},
//				Description:  "Single rule to apply the filter on. In case of start_with=true/includes=true replace to 'rule' (single string).",
//			},
//		},
//	}
//}

//func filterTypesRoutes(filterName string) []string {
//	return []string{
//		fmt.Sprintf("%s.0.is", filterName),
//		fmt.Sprintf("%s.0.is_not", filterName),
//		fmt.Sprintf("%s.0.starts_with", filterName),
//		fmt.Sprintf("%s.0.includes", filterName),
//	}
//}
