package coralogix

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"terraform-provider-coralogix/coralogix/clientset"
	tcopolicies "terraform-provider-coralogix/coralogix/clientset/grpc/tco-policies"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
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
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

var (
	_                                resource.ResourceWithConfigure    = &TCOPolicyResource{}
	_                                resource.ResourceWithImportState  = &TCOPolicyResource{}
	_                                resource.ResourceWithUpgradeState = &TCOPolicyResource{}
	tcoPoliciesPrioritySchemaToProto                                   = map[string]tcopolicies.Priority{
		"block":  tcopolicies.Priority_PRIORITY_TYPE_BLOCK,
		"high":   tcopolicies.Priority_PRIORITY_TYPE_HIGH,
		"low":    tcopolicies.Priority_PRIORITY_TYPE_LOW,
		"medium": tcopolicies.Priority_PRIORITY_TYPE_MEDIUM,
	}
	tcoPoliciesPriorityProtoToSchema = ReverseMap(tcoPoliciesPrioritySchemaToProto)
	tcoPoliciesValidPriorities       = GetKeys(tcoPoliciesPrioritySchemaToProto)
	tcoPoliciesRuleTypeSchemaToProto = map[string]tcopolicies.RuleTypeId{
		"is":          tcopolicies.RuleTypeId_RULE_TYPE_ID_IS,
		"is_not":      tcopolicies.RuleTypeId_RULE_TYPE_ID_IS_NOT,
		"starts_with": tcopolicies.RuleTypeId_RULE_TYPE_ID_START_WITH,
		"includes":    tcopolicies.RuleTypeId_RULE_TYPE_ID_INCLUDES,
	}
	tcoPoliciesRuleTypeProtoToSchema = ReverseMap(tcoPoliciesRuleTypeSchemaToProto)
	tcoPoliciesValidRuleTypes        = GetKeys(tcoPoliciesRuleTypeSchemaToProto)
	tcoPolicySeveritySchemaToProto   = map[string]tcopolicies.Severity{
		"debug":    tcopolicies.Severity_SEVERITY_DEBUG,
		"verbose":  tcopolicies.Severity_SEVERITY_VERBOSE,
		"info":     tcopolicies.Severity_SEVERITY_INFO,
		"warning":  tcopolicies.Severity_SEVERITY_WARNING,
		"error":    tcopolicies.Severity_SEVERITY_ERROR,
		"critical": tcopolicies.Severity_SEVERITY_CRITICAL,
	}
	tcoPolicySeverityProtoToSchema = ReverseMap(tcoPolicySeveritySchemaToProto)
	validPolicySeverities          = GetKeys(tcoPolicySeveritySchemaToProto)
)

func NewTCOPolicyResource() resource.Resource {
	return &TCOPolicyResource{}
}

type TCOPolicyResource struct {
	client *clientset.TCOPoliciesClient
}

type TCOPolicyResourceModel struct {
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

func (r *TCOPolicyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tco_policy_logs"
}

func (r *TCOPolicyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *TCOPolicyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
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
				Required: true,
				Validators: []validator.Int64{
					int64validator.AtLeast(1),
				},
				MarkdownDescription: "Determines the policy's order between the other policies.",
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
		MarkdownDescription: "Coralogix TCO-Policy. For more information - https://coralogix.com/docs/tco-optimizer-api .",
	}
}

func (r *TCOPolicyResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data TCOPolicyResourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	validateTCORuleModelModel(data.Subsystems, "subsystems", resp)

	validateTCORuleModelModel(data.Applications, "applications", resp)
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

func (r *TCOPolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)

}

func (r *TCOPolicyResource) UpgradeState(_ context.Context) map[int64]resource.StateUpgrader {
	schemaV0 := tcoPolicySchemaV0()
	return map[int64]resource.StateUpgrader{
		0: {
			PriorSchema:   &schemaV0,
			StateUpgrader: upgradeTcoPolicyStateV0ToV1,
		},
	}
}

func upgradeTcoPolicyStateV0ToV1(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
	type TCOPolicyResourceModelV0 struct {
		ID                 types.String `tfsdk:"id"`
		Name               types.String `tfsdk:"name"`
		Description        types.String `tfsdk:"description"`
		Enabled            types.Bool   `tfsdk:"enabled"`
		Order              types.Int64  `tfsdk:"order"`
		Priority           types.String `tfsdk:"priority"`
		ApplicationName    types.List   `tfsdk:"application_name"`
		SubsystemName      types.List   `tfsdk:"subsystem_name"`
		Severities         types.Set    `tfsdk:"severities"`
		ArchiveRetentionID types.String `tfsdk:"archive_retention_id"`
	}

	var priorStateData TCOPolicyResourceModelV0
	resp.Diagnostics.Append(req.State.Get(ctx, &priorStateData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	upgradedStateData := TCOPolicyResourceModel{
		ID:                 priorStateData.ID,
		Name:               priorStateData.Name,
		Description:        priorStateData.Description,
		Enabled:            priorStateData.Enabled,
		Order:              priorStateData.Order,
		Priority:           priorStateData.Priority,
		Applications:       upgradeTCOPolicyRuleV0ToV1(ctx, priorStateData.ApplicationName),
		Subsystems:         upgradeTCOPolicyRuleV0ToV1(ctx, priorStateData.SubsystemName),
		Severities:         priorStateData.Severities,
		ArchiveRetentionID: priorStateData.ArchiveRetentionID,
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, upgradedStateData)...)
}

func upgradeTCOPolicyRuleV0ToV1(ctx context.Context, tCOPolicyRule types.List) types.Object {
	var tCOPolicyRuleObjects []types.Object
	tCOPolicyRule.ElementsAs(ctx, &tCOPolicyRuleObjects, true)
	if len(tCOPolicyRuleObjects) == 0 {
		return types.ObjectNull(tcoPolicyRuleAttributes())
	}

	var tCORuleModelObjectV0 TCORuleModelV0
	tCOPolicyRuleObjects[0].As(ctx, &tCORuleModelObjectV0, basetypes.ObjectAsOptions{})

	tCORuleModelObjectV1 := &TCORuleModel{}
	if tCORuleModelObjectV0.Is.ValueBool() {
		tCORuleModelObjectV1.RuleType = types.StringValue("is")
	} else if tCORuleModelObjectV0.IsNot.ValueBool() {
		tCORuleModelObjectV1.RuleType = types.StringValue("is_not")
	} else if tCORuleModelObjectV0.Include.ValueBool() {
		tCORuleModelObjectV1.RuleType = types.StringValue("includes")
	} else if tCORuleModelObjectV0.StartsWith.ValueBool() {
		tCORuleModelObjectV1.RuleType = types.StringValue("starts_with")
	}

	if rule := tCORuleModelObjectV0.Rule.ValueString(); rule != "" {
		rules := []attr.Value{types.StringValue(rule)}
		tCORuleModelObjectV1.Names = types.SetValueMust(types.StringType, rules)
	} else {
		rules := tCORuleModelObjectV0.Rules.Elements()
		tCORuleModelObjectV1.Names = types.SetValueMust(types.StringType, rules)
	}

	obj, _ := types.ObjectValueFrom(ctx, tcoPolicyRuleAttributes(), tCORuleModelObjectV1)

	return obj
}

func tcoPolicyRuleAttributes() map[string]attr.Type {
	return map[string]attr.Type{
		"rule_type": types.StringType,
		"names":     types.SetType{ElemType: types.StringType},
	}
}

type TCORuleModelV0 struct {
	Is         types.Bool   `tfsdk:"is"`
	IsNot      types.Bool   `tfsdk:"is_not"`
	Include    types.Bool   `tfsdk:"include"`
	StartsWith types.Bool   `tfsdk:"starts_with"`
	Rule       types.String `tfsdk:"rule"`
	Rules      types.Set    `tfsdk:"rules"`
}

func tcoPolicySchemaV0() schema.Schema {
	return schema.Schema{
		Version: 0,
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
				Computed: true,
				Default:  stringdefault.StaticString(""),
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
			"severities": schema.SetAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
					setvalidator.ValueStringsAre(stringvalidator.OneOf(validPolicySeverities...)),
				},
				MarkdownDescription: fmt.Sprintf("The severities to apply the policy on. Can be few of %q.", validPolicySeverities),
			},
		},
		Blocks: map[string]schema.Block{
			"application_name": schema.ListNestedBlock{
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"starts_with": schema.BoolAttribute{},
						"is":          schema.BoolAttribute{},
						"is_not":      schema.BoolAttribute{},
						"includes":    schema.BoolAttribute{},
						"rule":        schema.StringAttribute{},
						"rules": schema.SetAttribute{
							ElementType: types.StringType,
						},
					},
				},
			},
			"subsystem_name": schema.ListNestedBlock{
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"starts_with": schema.BoolAttribute{},
						"is":          schema.BoolAttribute{},
						"is_not":      schema.BoolAttribute{},
						"includes":    schema.BoolAttribute{},
						"rule":        schema.StringAttribute{},
						"rules": schema.SetAttribute{
							ElementType: types.StringType,
						},
					},
				},
			},
		},
	}
}

func (r *TCOPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	var plan *TCOPolicyResourceModel
	diags := req.Plan.Get(ctx, &plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	createPolicyRequest, diags := extractCreateTcoPolicy(ctx, *plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	policyStr := protojson.Format(createPolicyRequest)
	log.Printf("[INFO] Creating new tco-policy: %s", policyStr)
	createResp, err := r.client.CreateTCOPolicy(ctx, createPolicyRequest)
	for err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		if retryableStatusCode(status.Code(err)) {
			log.Printf("[INFO] Retrying to create tco-policy: %s", policyStr)
			createResp, err = r.client.CreateTCOPolicy(ctx, createPolicyRequest)
			continue
		}
		resp.Diagnostics.AddError(
			"Error creating tco-policy",
			handleRpcErrorNewFramework(err, "tco-policy", policyStr),
		)
		return
	}
	policy := createResp.GetPolicy()
	policyStr = protojson.Format(policy)
	log.Printf("[INFO] Submitted new tco-policy: %s", policyStr)
	plan.ID = types.StringValue(createResp.GetPolicy().GetId().GetValue())

	// Update order
	id := plan.ID.ValueString()
	order := int(plan.Order.ValueInt64())
	log.Print("[INFO] Reordering tco-policies")
	err, reqStr := updatePoliciesOrder(ctx, r.client, id, order, tcopolicies.SourceType_SOURCE_TYPE_LOGS)
	for err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		if retryableStatusCode(status.Code(err)) {
			log.Print("[INFO] Retrying to reorder tco-policies")
			err, reqStr = updatePoliciesOrder(ctx, r.client, id, order, tcopolicies.SourceType_SOURCE_TYPE_LOGS)
			continue
		}
		resp.Diagnostics.AddError(
			"Error Reordering tco-policy",
			handleRpcErrorNewFramework(err, "tco-policy", reqStr),
		)
		return
	}

	policy.Order = wrapperspb.Int32(int32(plan.Order.ValueInt64()))
	plan, diags = flattenTCOPolicy(ctx, policy)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func (r *TCOPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	var state *TCOPolicyResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	//Get refreshed tco-policy value from Coralogix
	id := state.ID.ValueString()
	log.Printf("[INFO] Reading tco-policy: %s", id)
	getPolicyReq := &tcopolicies.GetPolicyRequest{Id: wrapperspb.String(id)}
	getPolicyResp, err := r.client.GetTCOPolicy(ctx, getPolicyReq)
	for err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		if retryableStatusCode(status.Code(err)) {
			log.Printf("[INFO] Retrying to read tco-policy: %s", id)
			getPolicyResp, err = r.client.GetTCOPolicy(ctx, &tcopolicies.GetPolicyRequest{Id: wrapperspb.String(id)})
			continue
		}
		if status.Code(err) == codes.NotFound {
			state.ID = types.StringNull()
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("tco-policy %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
		} else {
			reqStr := protojson.Format(getPolicyReq)
			resp.Diagnostics.AddError(
				"Error reading tco-policy",
				handleRpcErrorNewFramework(err, "tco-policy", reqStr),
			)
		}
		return
	}
	policy := getPolicyResp.GetPolicy()
	log.Printf("[INFO] Received tco-policy: %#v", policy)

	state, diags = flattenTCOPolicy(ctx, policy)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	//
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *TCOPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	// Retrieve values from plan
	var plan *TCOPolicyResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	policyUpdateReq, diags := extractUpdateTCOPolicy(ctx, *plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	log.Printf("[INFO] Updating tco-policy: %#v", policyUpdateReq)
	policyUpdateResp, err := r.client.UpdateTCOPolicy(ctx, policyUpdateReq)
	for err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		if retryableStatusCode(status.Code(err)) {
			log.Printf("[INFO] Retrying to update tco-policy: %#v", policyUpdateReq)
			policyUpdateResp, err = r.client.UpdateTCOPolicy(ctx, policyUpdateReq)
			continue
		}
		resp.Diagnostics.AddError(
			"Error updating tco-policy",
			"Could not update tco-policy, unexpected error: "+err.Error(),
		)
		return
	}
	log.Printf("[INFO] Submitted updated tco-policy: %#v", policyUpdateResp)

	err, reqStr := updatePoliciesOrder(ctx, r.client, plan.ID.ValueString(), int(plan.Order.ValueInt64()), tcopolicies.SourceType_SOURCE_TYPE_LOGS)
	for err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		if retryableStatusCode(status.Code(err)) {
			log.Printf("[INFO] Retrying to reorder tco-policies")
			err, reqStr = updatePoliciesOrder(ctx, r.client, plan.ID.ValueString(), int(plan.Order.ValueInt64()), tcopolicies.SourceType_SOURCE_TYPE_LOGS)
			continue
		}
		resp.Diagnostics.AddError(
			"Error Reordering tco-policy",
			handleRpcErrorNewFramework(err, "tco-policy", reqStr),
		)
		return
	}
	// Get refreshed tco-policy value from Coralogix
	id := plan.ID.ValueString()
	getPolicyReq := &tcopolicies.GetPolicyRequest{Id: wrapperspb.String(id)}
	getPolicyResp, err := r.client.GetTCOPolicy(ctx, getPolicyReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		if status.Code(err) == codes.NotFound {
			plan.ID = types.StringNull()
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("tco-policy %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
		} else {
			reqStr = protojson.Format(getPolicyReq)
			resp.Diagnostics.AddError(
				"Error reading tco-policy",
				handleRpcErrorNewFramework(err, "tco-policy", reqStr),
			)
		}
		return
	}
	log.Printf("[INFO] Received tco-policy: %#v", getPolicyResp)

	plan, diags = flattenTCOPolicy(ctx, getPolicyResp.GetPolicy())
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func (r TCOPolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	var state TCOPolicyResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	log.Printf("[INFO] Deleting tco-policy %s\n", id)
	deleteReq := &tcopolicies.DeletePolicyRequest{Id: wrapperspb.String(id)}
	_, err := r.client.DeleteTCOPolicy(ctx, deleteReq)
	for err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		if retryableStatusCode(status.Code(err)) {
			log.Printf("[INFO] Retrying to delete tco-policy: %s", id)
			_, err = r.client.DeleteTCOPolicy(ctx, deleteReq)
			continue
		}
		reqStr := protojson.Format(deleteReq)
		resp.Diagnostics.AddError(
			fmt.Sprintf("Error Deleting tco-policy %s", id),
			handleRpcErrorNewFramework(err, "tco-policy", reqStr),
		)
		return
	}
	log.Printf("[INFO] tco-policy %s deleted\n", id)
}

func flattenTCOPolicy(ctx context.Context, policy *tcopolicies.Policy) (*TCOPolicyResourceModel, diag.Diagnostics) {
	logRules := policy.GetSourceTypeRules().(*tcopolicies.Policy_LogRules).LogRules
	applications, diags := flattenTCOPolicyRule(ctx, policy.GetApplicationRule())
	if diags.HasError() {
		return nil, diags
	}
	subsystems, diags := flattenTCOPolicyRule(ctx, policy.GetSubsystemRule())
	if diags.HasError() {
		return nil, diags
	}
	tcoPolicy := &TCOPolicyResourceModel{
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
	}

	return tcoPolicy, nil
}

func flattenTCOPolicyRule(ctx context.Context, rule *tcopolicies.Rule) (types.Object, diag.Diagnostics) {
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

func extractUpdateTCOPolicy(ctx context.Context, plan TCOPolicyResourceModel) (*tcopolicies.UpdatePolicyRequest, diag.Diagnostics) {
	id := typeStringToWrapperspbString(plan.ID)
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
	sourceTypeRules, diags := expandLogsSourceTypeUpdate(ctx, plan)
	if diags.HasError() {
		return nil, diags
	}

	updateRequest := &tcopolicies.UpdatePolicyRequest{
		Id:               id,
		Name:             name,
		Description:      description,
		Priority:         priority,
		ApplicationRule:  applicationRule,
		SubsystemRule:    subsystemRule,
		ArchiveRetention: archiveRetention,
		SourceTypeRules:  sourceTypeRules,
	}

	return updateRequest, nil
}

func extractCreateTcoPolicy(ctx context.Context, plan TCOPolicyResourceModel) (*tcopolicies.CreatePolicyRequest, diag.Diagnostics) {
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
	sourceTypeRules, diags := expandLogsSourceType(ctx, plan)
	if diags.HasError() {
		return nil, diags
	}

	return &tcopolicies.CreatePolicyRequest{
		Name:             name,
		Description:      description,
		Priority:         priority,
		ApplicationRule:  applicationRule,
		SubsystemRule:    subsystemRule,
		ArchiveRetention: archiveRetention,
		SourceTypeRules:  sourceTypeRules,
	}, nil
}

func expandLogsSourceType(ctx context.Context, plan TCOPolicyResourceModel) (*tcopolicies.CreatePolicyRequest_LogRules, diag.Diagnostics) {
	severities, diags := expandTCOPolicySeverities(ctx, plan.Severities.Elements())
	if diags.HasError() {
		return nil, diags
	}
	return &tcopolicies.CreatePolicyRequest_LogRules{
		LogRules: &tcopolicies.LogRules{
			Severities: severities,
		},
	}, nil
}

func expandLogsSourceTypeUpdate(ctx context.Context, plan TCOPolicyResourceModel) (*tcopolicies.UpdatePolicyRequest_LogRules, diag.Diagnostics) {
	severities, diags := expandTCOPolicySeverities(ctx, plan.Severities.Elements())
	if diags.HasError() {
		return nil, diags
	}
	return &tcopolicies.UpdatePolicyRequest_LogRules{
		LogRules: &tcopolicies.LogRules{
			Severities: severities,
		},
	}, nil
}

func expandTCOPolicyRule(ctx context.Context, rule types.Object) (*tcopolicies.Rule, diag.Diagnostics) {
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

	return &tcopolicies.Rule{
		RuleTypeId: ruleType,
		Name:       nameStr,
	}, nil
}

func updatePoliciesOrder(ctx context.Context, client *clientset.TCOPoliciesClient, policyID string, policyOrder int, sourceType tcopolicies.SourceType) (error, string) {
	getPoliciesReq := &tcopolicies.GetCompanyPoliciesRequest{
		EnabledOnly: wrapperspb.Bool(false),
		SourceType:  &sourceType,
	}
	getPoliciesReqStr := protojson.Format(getPoliciesReq)
	log.Printf("[INFO] Get tco-policies request: %s", getPoliciesReqStr)

	getPoliciesResp, err := client.GetTCOPolicies(ctx, getPoliciesReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return err, getPoliciesReqStr
	}

	getPoliciesRespStr := protojson.Format(getPoliciesResp)
	log.Printf("[INFO] Get tco-policies response: %#v", getPoliciesRespStr)

	policies := getPoliciesResp.GetPolicies()
	policiesIDsByOrder, currentPolicyIndex := getPoliciesIDsByOrderAndCurrentPolicyIndex(policies, policyID)

	desiredPolicyIndex := getPolicyDesireIndex(policyOrder, policies)

	if currentPolicyIndex == desiredPolicyIndex {
		return nil, ""
	}

	policiesIDsByOrder[currentPolicyIndex].Order, policiesIDsByOrder[desiredPolicyIndex].Order = policiesIDsByOrder[desiredPolicyIndex].Order, policiesIDsByOrder[currentPolicyIndex].Order
	reorderReq := &tcopolicies.ReorderPoliciesRequest{
		Orders:     policiesIDsByOrder,
		SourceType: sourceType,
	}
	reorderReqStr := protojson.Format(reorderReq)
	log.Printf("[INFO] Reorder tco-policies request: %s", reorderReqStr)

	reorderResp, err := client.ReorderTCOPolicies(ctx, reorderReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return err, reorderReqStr
	}
	reorderRespStr := protojson.Format(reorderResp)
	log.Printf("[INFO] Reorder tco-policies response: %s", reorderRespStr)

	return nil, ""
}

func getPoliciesIDsByOrderAndCurrentPolicyIndex(policies []*tcopolicies.Policy, policyID string) ([]*tcopolicies.PolicyOrder, int) {
	policiesIDsByOrder := make([]*tcopolicies.PolicyOrder, len(policies))
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

func getPolicyDesireIndex(order int, policies []*tcopolicies.Policy) int {
	desiredPolicyIndex := order - 1
	if desiredPolicyIndex >= len(policies) {
		desiredPolicyIndex = len(policies) - 1
	}
	return desiredPolicyIndex
}

func expandActiveRetention(archiveRetention types.String) *tcopolicies.ArchiveRetention {
	if archiveRetention.IsNull() {
		return nil
	}

	return &tcopolicies.ArchiveRetention{
		Id: wrapperspb.String(archiveRetention.ValueString()),
	}
}

func expandTCOPolicySeverities(ctx context.Context, severities []attr.Value) ([]tcopolicies.Severity, diag.Diagnostics) {
	result := make([]tcopolicies.Severity, 0, len(severities))
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
