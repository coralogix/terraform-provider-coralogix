package coralogix

import (
	"context"
	"fmt"
	"log"
	"strings"

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
	_ resource.ResourceWithConfigure   = &TCOPolicyTracesResource{}
	_ resource.ResourceWithImportState = &TCOPolicyTracesResource{}
)

func NewTCOPolicyTracesResource() resource.Resource {
	return &TCOPolicyTracesResource{}
}

type TCOPolicyTracesResource struct {
	client *clientset.TCOPoliciesClient
}

type TCOPolicyTracesResourceModel struct {
	ID                 types.String  `tfsdk:"id"`
	Name               types.String  `tfsdk:"name"`
	Description        types.String  `tfsdk:"description"`
	Enabled            types.Bool    `tfsdk:"enabled"`
	Order              types.Int64   `tfsdk:"order"`
	Priority           types.String  `tfsdk:"priority"`
	Applications       *TCORuleModel `tfsdk:"applications"`
	Subsystems         *TCORuleModel `tfsdk:"subsystems"`
	ArchiveRetentionID types.String  `tfsdk:"archive_retention_id"`
	Services           *TCORuleModel `tfsdk:"services"`
	Actions            *TCORuleModel `tfsdk:"actions"`
	Tags               types.Map     `tfsdk:"tags"`
}

func (r *TCOPolicyTracesResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tco_policy_traces"
}

func (r *TCOPolicyTracesResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *TCOPolicyTracesResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
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
				MarkdownDescription: "The subsystems to apply the policy on. Applies the policy on all the subsystems by default.",
			},
		},
		MarkdownDescription: "Coralogix TCO-Policy. For more information - https://coralogix.com/docs/tco-optimizer-api .",
	}
}

func (r *TCOPolicyTracesResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)

}

func (r *TCOPolicyTracesResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan TCOPolicyTracesResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	createPolicyRequest := extractCreateTcoPolicyTraces(ctx, plan)
	policyStr, _ := jsm.MarshalToString(createPolicyRequest)
	log.Printf("[INFO] Creating new tco-policy: %s", policyStr)
	createResp, err := r.client.CreateTCOPolicy(ctx, createPolicyRequest)
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
	plan.ID = types.StringValue(createResp.GetPolicy().GetId().GetValue())
	updatePoliciesOrder(ctx, r.client, plan.ID.ValueString(), int(plan.Order.ValueInt64()), tcopolicies.SourceType_SOURCE_TYPE_SPANS)

	policy.Order = wrapperspb.Int32(int32(plan.Order.ValueInt64()))
	plan = flattenTCOPolicyTraces(ctx, policy)

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *TCOPolicyTracesResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state TCOPolicyTracesResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	//Get refreshed tco-policy value from Coralogix
	id := state.ID.ValueString()
	log.Printf("[INFO] Reading tco-policy: %s", id)
	getPolicyResp, err := r.client.GetTCOPolicy(ctx, &tcopolicies.GetPolicyRequest{Id: wrapperspb.String(id)})
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

	state = flattenTCOPolicyTraces(ctx, policy)
	//
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r TCOPolicyTracesResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var plan TCOPolicyTracesResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	policyUpdateReq := extractUpdateTCOPolicyTraces(ctx, plan)
	log.Printf("[INFO] Updating tco-policy: %#v", policyUpdateReq)
	policyUpdateResp, err := r.client.UpdateTCOPolicy(ctx, policyUpdateReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		resp.Diagnostics.AddError(
			"Error updating tco-policy",
			"Could not update tco-policy, unexpected error: "+err.Error(),
		)
		return
	}
	log.Printf("[INFO] Submitted updated tco-policy: %#v", policyUpdateResp)

	updatePoliciesOrder(ctx, r.client, plan.ID.ValueString(), int(plan.Order.ValueInt64()), tcopolicies.SourceType_SOURCE_TYPE_SPANS)

	// Get refreshed tco-policy value from Coralogix
	id := plan.ID.ValueString()
	getPolicyResp, err := r.client.GetTCOPolicy(ctx, &tcopolicies.GetPolicyRequest{Id: wrapperspb.String(id)})
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		if status.Code(err) == codes.NotFound {
			plan.ID = types.StringNull()
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
	log.Printf("[INFO] Received tco-policy: %#v", getPolicyResp)

	plan = flattenTCOPolicyTraces(ctx, getPolicyResp.GetPolicy())

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r TCOPolicyTracesResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state TCOPolicyResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	log.Printf("[INFO] Deleting tco-policy %s\n", id)
	if _, err := r.client.DeleteTCOPolicy(ctx, &tcopolicies.DeletePolicyRequest{Id: wrapperspb.String(id)}); err != nil {
		resp.Diagnostics.AddError(
			fmt.Sprintf("Error Deleting tco-policy %s", state.ID.ValueString()),
			handleRpcErrorNewFramework(err, "tco-policy"),
		)
		return
	}
	log.Printf("[INFO] tco-policy %s deleted\n", id)
}

func flattenTCOPolicyTraces(ctx context.Context, policy *tcopolicies.Policy) TCOPolicyTracesResourceModel {
	traceRules := policy.GetSourceTypeRules().(*tcopolicies.Policy_SpanRules).SpanRules
	tcoPolicy := TCOPolicyTracesResourceModel{
		ID:                 types.StringValue(policy.GetId().GetValue()),
		Name:               types.StringValue(policy.GetName().GetValue()),
		Description:        types.StringValue(policy.GetDescription().GetValue()),
		Enabled:            types.BoolValue(policy.GetEnabled().GetValue()),
		Order:              types.Int64Value(int64(policy.GetOrder().GetValue())),
		Priority:           types.StringValue(tcoPoliciesPriorityProtoToSchema[policy.GetPriority()]),
		Applications:       flattenTCOPolicyRule(policy.GetApplicationRule()),
		Subsystems:         flattenTCOPolicyRule(policy.GetSubsystemRule()),
		ArchiveRetentionID: flattenArchiveRetention(policy.GetArchiveRetention()),
		Services:           flattenTCOPolicyRule(traceRules.GetServiceRule()),
		Actions:            flattenTCOPolicyRule(traceRules.GetActionRule()),
		Tags:               flattenTCOPolicyTags(ctx, traceRules.GetTagRules()),
	}

	return tcoPolicy
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

	return types.MapValueMust(types.ObjectType{AttrTypes: tcoRuleModelAttr()}, elements)
}

func tcoRuleModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"rule_type": types.StringType,
		"names": types.SetType{
			ElemType: types.StringType,
		},
	}
}

func extractUpdateTCOPolicyTraces(ctx context.Context, plan TCOPolicyTracesResourceModel) *tcopolicies.UpdatePolicyRequest {
	id := typeStringToWrapperspbString(plan.ID)
	name := typeStringToWrapperspbString(plan.Name)
	description := typeStringToWrapperspbString(plan.Description)
	priority := tcoPoliciesPrioritySchemaToProto[plan.Priority.ValueString()]
	applicationRule := expandTCOPolicyRule(ctx, plan.Applications)
	subsystemRule := expandTCOPolicyRule(ctx, plan.Subsystems)
	archiveRetention := expandActiveRetention(plan.ArchiveRetentionID)

	updateRequest := &tcopolicies.UpdatePolicyRequest{
		Id:               id,
		Name:             name,
		Description:      description,
		Priority:         priority,
		ApplicationRule:  applicationRule,
		SubsystemRule:    subsystemRule,
		ArchiveRetention: archiveRetention,
		SourceTypeRules:  expandTracesSourceTypeUpdate(ctx, plan),
	}

	return updateRequest
}

func extractCreateTcoPolicyTraces(ctx context.Context, plan TCOPolicyTracesResourceModel) *tcopolicies.CreatePolicyRequest {
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
		SourceTypeRules:  expandTracesSourceType(ctx, plan),
	}

	return createRequest
}

func expandTracesSourceType(ctx context.Context, plan TCOPolicyTracesResourceModel) *tcopolicies.CreatePolicyRequest_SpanRules {
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

func expandTracesSourceTypeUpdate(ctx context.Context, plan TCOPolicyTracesResourceModel) *tcopolicies.UpdatePolicyRequest_SpanRules {
	serviceRule := expandTCOPolicyRule(ctx, plan.Services)
	actionRule := expandTCOPolicyRule(ctx, plan.Actions)
	tagRules := expandTagsRules(ctx, plan.Tags)

	return &tcopolicies.UpdatePolicyRequest_SpanRules{
		SpanRules: &tcopolicies.SpanRules{
			ServiceRule: serviceRule,
			ActionRule:  actionRule,
			TagRules:    tagRules,
		},
	}
}

func expandTagsRules(ctx context.Context, tags types.Map) []*tcopolicies.TagRule {
	var tagsMap map[string]TCORuleModel
	d := tags.ElementsAs(ctx, &tagsMap, true)
	if d != nil {
		panic(d)
	}

	result := make([]*tcopolicies.TagRule, 0, len(tagsMap))
	for tagName, tagElement := range tagsMap {
		tagRule := expandTagRule(ctx, tagName, &tagElement)
		result = append(result, tagRule)
	}

	return result
}

func expandTagRule(ctx context.Context, name string, tag *TCORuleModel) *tcopolicies.TagRule {
	rule := expandTCOPolicyRule(ctx, tag)
	return &tcopolicies.TagRule{
		TagName:    wrapperspb.String(name),
		RuleTypeId: rule.GetRuleTypeId(),
		TagValue:   rule.GetName(),
	}
}
