package coralogix

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"

	"terraform-provider-coralogix/coralogix/clientset"
	tcopolicies "terraform-provider-coralogix/coralogix/clientset/grpc/tco-policies"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/mapvalidator"
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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

var (
	_ resource.ResourceWithConfigure      = &TCOPolicyTracesResource{}
	_ resource.ResourceWithImportState    = &TCOPolicyTracesResource{}
	_ resource.ResourceWithValidateConfig = &TCOPolicyTracesResource{}
)

func NewTCOPolicyTracesResource() resource.Resource {
	return &TCOPolicyTracesResource{}
}

type TCOPolicyTracesResource struct {
	client *clientset.TCOPoliciesClient
}

type TCOPolicyTracesResourceModel struct {
	ID                 types.String `tfsdk:"id"`
	Name               types.String `tfsdk:"name"`
	Description        types.String `tfsdk:"description"`
	Enabled            types.Bool   `tfsdk:"enabled"`
	Order              types.Int64  `tfsdk:"order"`
	Priority           types.String `tfsdk:"priority"`
	Applications       types.Object `tfsdk:"applications"`
	Subsystems         types.Object `tfsdk:"subsystems"`
	ArchiveRetentionID types.String `tfsdk:"archive_retention_id"`
	Services           types.Object `tfsdk:"services"`
	Actions            types.Object `tfsdk:"actions"`
	Tags               types.Map    `tfsdk:"tags"`
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
				Validators: []validator.Map{
					mapvalidator.KeysAre(stringvalidator.RegexMatches(regexp.MustCompile("tags.*"), "tag names must have 'tags.' prefix")),
				},
				MarkdownDescription: "The subsystems to apply the policy on. Applies the policy on all the subsystems by default.",
			},
		},
		MarkdownDescription: "Coralogix TCO-Policy. For more information - https://coralogix.com/docs/tco-optimizer-api .",
	}
}

func (r *TCOPolicyTracesResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data TCOPolicyTracesResourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	validateTCORuleModelModel(data.Subsystems, "subsystems", resp)

	validateTCORuleModelModel(data.Applications, "applications", resp)

	validateTCORuleModelModel(data.Services, "services", resp)

	validateTCORuleModelModel(data.Actions, "actions", resp)

	var tagsMap map[string]types.Object
	diags := data.Tags.ElementsAs(ctx, &tagsMap, true)
	if diags != nil {
		resp.Diagnostics.Append(diags...)
	} else {
		for tagName, tagRule := range tagsMap {
			root := fmt.Sprintf("tags.%s", tagName)
			validateTCORuleModelModel(tagRule, root, resp)
		}
	}

}

func (r *TCOPolicyTracesResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)

}

func (r *TCOPolicyTracesResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan *TCOPolicyTracesResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	createPolicyRequest, diags := extractCreateTcoPolicyTraces(ctx, *plan)
	if diags.HasError() {
		resp.Diagnostics = diags
		return
	}
	policyStr := protojson.Format(createPolicyRequest)
	log.Printf("[INFO] Creating new tco-policy: %s", policyStr)
	createResp, err := r.client.CreateTCOPolicy(ctx, createPolicyRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error creating tco-policy",
			formatRpcErrors(err, createTCOPolicyURL, policyStr),
		)
		return
	}
	policy := createResp.GetPolicy()
	policyStr = protojson.Format(policy)
	log.Printf("[INFO] Submitted new tco-policy: %s", policyStr)
	plan.ID = types.StringValue(createResp.GetPolicy().GetId().GetValue())
	id := plan.ID.ValueString()
	order := int(plan.Order.ValueInt64())
	err, reqStr := updatePoliciesOrder(ctx, r.client, id, order, tcopolicies.SourceType_SOURCE_TYPE_SPANS)
	for err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if retryableStatusCode(status.Code(err)) {
			log.Print("[INFO] Retrying to reorder tco-policies")
			err, reqStr = updatePoliciesOrder(ctx, r.client, id, order, tcopolicies.SourceType_SOURCE_TYPE_SPANS)
			continue
		}
		resp.Diagnostics.AddError(
			"Error Reordering tco-policy",
			formatRpcErrors(err, updateTCOPoliciesOrderURL, reqStr),
		)
		return
	}
	policy.Order = wrapperspb.Int32(int32(plan.Order.ValueInt64()))
	plan, diags = flattenTCOPolicyTraces(ctx, policy)
	if diags.HasError() {
		resp.Diagnostics = diags
		return
	}
	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func (r *TCOPolicyTracesResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state *TCOPolicyTracesResourceModel
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
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if status.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("tco-policy %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			reqStr := protojson.Format(getPolicyReq)
			resp.Diagnostics.AddError(
				"Error reading tco-policy",
				formatRpcErrors(err, getTCOPolicyURL, reqStr),
			)
		}
		return
	}
	policy := getPolicyResp.GetPolicy()
	log.Printf("[INFO] Received tco-policy: %s", protojson.Format(policy))

	state, diags = flattenTCOPolicyTraces(ctx, policy)
	if diags.HasError() {
		resp.Diagnostics = diags
		return
	}
	//
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r TCOPolicyTracesResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var plan *TCOPolicyTracesResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	policyUpdateReq, diags := extractUpdateTCOPolicyTraces(ctx, *plan)
	if diags.HasError() {
		resp.Diagnostics = diags
		return
	}
	log.Printf("[INFO] Updating tco-policy: %s", protojson.Format(policyUpdateReq))
	policyUpdateResp, err := r.client.UpdateTCOPolicy(ctx, policyUpdateReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error updating tco-policy",
			formatRpcErrors(err, updateTCOPolicyURL, protojson.Format(policyUpdateReq)),
		)
		return
	}
	log.Printf("[INFO] Submitted updated tco-policy: %s", protojson.Format(policyUpdateResp))

	id := plan.ID.ValueString()
	order := int(plan.Order.ValueInt64())
	err, reqStr := updatePoliciesOrder(ctx, r.client, id, order, tcopolicies.SourceType_SOURCE_TYPE_SPANS)
	for err != nil {
		log.Printf("[ERROR] Received error: %s", err)
		if retryableStatusCode(status.Code(err)) {
			log.Print("[INFO] Retrying to reorder tco-policies")
			err, reqStr = updatePoliciesOrder(ctx, r.client, id, order, tcopolicies.SourceType_SOURCE_TYPE_SPANS)
			continue
		}
		resp.Diagnostics.AddError(
			"Error Reordering tco-policy",
			formatRpcErrors(err, updateTCOPoliciesOrderURL, reqStr),
		)
	}

	// Get refreshed tco-policy value from Coralogix
	getPolicyReq := &tcopolicies.GetPolicyRequest{Id: wrapperspb.String(id)}
	getPolicyResp, err := r.client.GetTCOPolicy(ctx, getPolicyReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if status.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("tco-policy %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError(
				"Error reading tco-policy",
				formatRpcErrors(err, getTCOPolicyURL, protojson.Format(getPolicyReq)),
			)
		}
		return
	}
	log.Printf("[INFO] Received tco-policy: %s", getPolicyResp)

	plan, diags = flattenTCOPolicyTraces(ctx, getPolicyResp.GetPolicy())
	if diags.HasError() {
		resp.Diagnostics = diags
		return
	}
	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func (r TCOPolicyTracesResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state TCOPolicyTracesResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	deleteReq := &tcopolicies.DeletePolicyRequest{Id: wrapperspb.String(id)}
	log.Printf("[INFO] Deleting tco-policy %s", id)
	if _, err := r.client.DeleteTCOPolicy(ctx, deleteReq); err != nil {
		resp.Diagnostics.AddError(
			fmt.Sprintf("Error Deleting tco-policy %s", id),
			formatRpcErrors(err, deleteTCOPolicyURL, protojson.Format(deleteReq)),
		)
		return
	}
	log.Printf("[INFO] tco-policy %s deleted", id)
}

func flattenTCOPolicyTraces(ctx context.Context, policy *tcopolicies.Policy) (*TCOPolicyTracesResourceModel, diag.Diagnostics) {
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

	return &TCOPolicyTracesResourceModel{
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

func tcoRuleModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"rule_type": types.StringType,
		"names": types.SetType{
			ElemType: types.StringType,
		},
	}
}

func extractUpdateTCOPolicyTraces(ctx context.Context, plan TCOPolicyTracesResourceModel) (*tcopolicies.UpdatePolicyRequest, diag.Diagnostics) {
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
	sourceTypeRules, diags := expandTracesSourceTypeUpdate(ctx, plan)
	if diags.HasError() {
		return nil, diags
	}
	return &tcopolicies.UpdatePolicyRequest{
		Id:               id,
		Name:             name,
		Description:      description,
		Priority:         priority,
		ApplicationRule:  applicationRule,
		SubsystemRule:    subsystemRule,
		ArchiveRetention: archiveRetention,
		SourceTypeRules:  sourceTypeRules,
	}, nil
}

func extractCreateTcoPolicyTraces(ctx context.Context, plan TCOPolicyTracesResourceModel) (*tcopolicies.CreatePolicyRequest, diag.Diagnostics) {
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
	sourceTypeRules, diags := expandTracesSourceType(ctx, plan)
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

func expandTracesSourceType(ctx context.Context, plan TCOPolicyTracesResourceModel) (*tcopolicies.CreatePolicyRequest_SpanRules, diag.Diagnostics) {
	serviceRule, diags := expandTCOPolicyRule(ctx, plan.Services)
	if diags.HasError() {
		return nil, diags
	}
	actionRule, diags := expandTCOPolicyRule(ctx, plan.Actions)
	if diags.HasError() {
		return nil, diags
	}
	tagRules, diags := expandTagsRules(ctx, plan.Tags)
	if diags.HasError() {
		return nil, diags
	}

	return &tcopolicies.CreatePolicyRequest_SpanRules{
		SpanRules: &tcopolicies.SpanRules{
			ServiceRule: serviceRule,
			ActionRule:  actionRule,
			TagRules:    tagRules,
		},
	}, nil
}

func expandTracesSourceTypeUpdate(ctx context.Context, plan TCOPolicyTracesResourceModel) (*tcopolicies.UpdatePolicyRequest_SpanRules, diag.Diagnostics) {
	serviceRule, diags := expandTCOPolicyRule(ctx, plan.Services)
	if diags.HasError() {
		return nil, diags
	}
	actionRule, diags := expandTCOPolicyRule(ctx, plan.Actions)
	if diags.HasError() {
		return nil, diags
	}
	tagRules, diags := expandTagsRules(ctx, plan.Tags)
	if diags.HasError() {
		return nil, diags
	}
	return &tcopolicies.UpdatePolicyRequest_SpanRules{
		SpanRules: &tcopolicies.SpanRules{
			ServiceRule: serviceRule,
			ActionRule:  actionRule,
			TagRules:    tagRules,
		},
	}, nil
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
