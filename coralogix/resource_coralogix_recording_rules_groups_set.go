package coralogix

import (
	"context"
	"fmt"
	"log"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gopkg.in/yaml.v3"
	"terraform-provider-coralogix/coralogix/clientset"
	rrgs "terraform-provider-coralogix/coralogix/clientset/grpc/recording-rules-groups-sets/v1"
)

var (
	_ resource.ResourceWithConfigure    = &RecordingRuleGroupSetResource{}
	_ resource.ResourceWithImportState  = &RecordingRuleGroupSetResource{}
	_ resource.ResourceWithUpgradeState = &RecordingRuleGroupSetResource{}
)

func NewRecordingRuleGroupSetResource() resource.Resource {
	return &RecordingRuleGroupSetResource{}
}

type RecordingRuleGroupSetResource struct {
	client *clientset.RecordingRulesGroupsSetsClient
}

func (r RecordingRuleGroupSetResource) UpgradeState(_ context.Context) map[int64]resource.StateUpgrader {
	schemaV0 := recordingRuleGroupSetV0()
	return map[int64]resource.StateUpgrader{
		0: {
			PriorSchema:   &schemaV0,
			StateUpgrader: upgradeRecordingRuleGroupSetStateV0ToV1,
		},
	}
}

func recordingRuleGroupSetV0() schema.Schema {
	return schema.Schema{
		Version: 0,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"yaml_content": schema.StringAttribute{
				Optional: true,
			},
			"group": schema.SetNestedAttribute{
				Optional:     true,
				Computed:     true,
				NestedObject: recordingRuleGroupSchemaV0(),
			},
			"name": schema.StringAttribute{
				Optional: true,
				Computed: true,
			},
		},
	}
}

func recordingRuleGroupSchemaV0() schema.NestedAttributeObject {
	return schema.NestedAttributeObject{
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required: true,
			},
			"interval": schema.Int64Attribute{
				Required: true,
			},
			"limit": schema.Int64Attribute{
				Optional: true,
			},
			"rule": schema.ListNestedAttribute{
				Required:     true,
				NestedObject: recordingRulesSchemaV0(),
			},
		},
	}
}

func recordingRulesSchemaV0() schema.NestedAttributeObject {
	return schema.NestedAttributeObject{
		Attributes: map[string]schema.Attribute{
			"record": schema.StringAttribute{
				Required: true,
			},
			"expr": schema.StringAttribute{
				Required: true,
			},
			"labels": schema.MapAttribute{
				ElementType: types.StringType,
				Optional:    true,
			},
		},
	}
}

func upgradeRecordingRuleGroupSetStateV0ToV1(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {

}

func (r RecordingRuleGroupSetResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r RecordingRuleGroupSetResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	r.client = clientSet.RecordingRuleGroupsSets()
}

func (r RecordingRuleGroupSetResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_recording_rules_groups_set"
}

func (r RecordingRuleGroupSetResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version: 1,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"yaml_content": schema.StringAttribute{
				Optional: true,
				Validators: []validator.String{
					stringvalidator.ExactlyOneOf(
						path.MatchRelative().AtParent().AtName("groups"),
						path.MatchRelative().AtParent().AtName("name"),
					),
					recordingRulesGroupYamlContentValidator{},
				},
			},
			"groups": schema.SetNestedAttribute{
				Optional:     true,
				Computed:     true,
				NestedObject: recordingRuleGroupSchema(),
			},
			"name": schema.StringAttribute{
				Optional: true,
				Computed: true,
			},
		},
	}
}

func recordingRuleGroupSchema() schema.NestedAttributeObject {
	return schema.NestedAttributeObject{
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The rule-group name. Have to be unique.",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"interval": schema.Int64Attribute{
				Required:    true,
				Description: "How often rules in the group are evaluated (in seconds).",
				Validators: []validator.Int64{
					int64validator.AtLeast(0),
				},
			},
			"limit": schema.Int64Attribute{
				Optional:    true,
				Description: "Limit the number of alerts an alerting rule and series a recording-rule can produce. 0 is no limit.",
			},
			"rules": schema.ListNestedAttribute{
				Required:     true,
				NestedObject: recordingRulesSchema(),
			},
		},
	}
}

func recordingRulesSchema() schema.NestedAttributeObject {
	return schema.NestedAttributeObject{
		Attributes: map[string]schema.Attribute{
			"record": schema.StringAttribute{
				Required:    true,
				Description: "The name of the time series to output to. Must be a valid metric name.",
			},
			"expr": schema.StringAttribute{
				Required: true,
				Description: "The PromQL expression to evaluate. " +
					"Every evaluation cycle this is evaluated at the current time," +
					" and the result recorded as a new set of time series with the metric name as given by 'record'.",
			},
			"labels": schema.MapAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Description: "Labels to add or overwrite before storing the result.",
			},
		},
	}
}

func (r RecordingRuleGroupSetResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan *RecordingRuleGroupSetResourceModel
	diags := req.Plan.Get(ctx, &plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	createPolicyRequest, diags := expandRecordingRulesGroupsSet(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	rrgStr, _ := jsm.MarshalToString(createPolicyRequest)
	log.Printf("[INFO] Creating new recogring-rule-group-set: %s", rrgStr)
	createResp, err := r.client.CreateRecordingRuleGroupsSet(ctx, createPolicyRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		resp.Diagnostics.AddError(
			"Error creating recording-rule-group-set",
			"Could not create recording-rule-group-set, unexpected error: "+err.Error(),
		)
		return
	}
	id := createResp.GetId()
	log.Printf("[INFO] Submitted new recording-rule-group-set id: %#v", id)
	plan.ID = types.StringValue(id)

	log.Printf("[INFO] Reading recording-rule-group-set id: %#v", id)
	getResp, err := r.client.GetRecordingRuleGroupsSet(ctx, &rrgs.FetchRuleGroupSet{Id: id})
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		if status.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("recording-rule-group-set %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error reading recording-rule-group-set",
				handleRpcErrorNewFramework(err, "recording-rule-group-set"),
			)
		}
		return
	}

	plan, diags = flattenRecordingRuleGroupSet(ctx, getResp)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func flattenRecordingRuleGroupSet(ctx context.Context, resp *rrgs.OutRuleGroupSet) (*RecordingRuleGroupSetResourceModel, diag.Diagnostics) {
	groups, diags := flattenRecordingRuleGroups(ctx, resp.GetGroups())
	if diags.HasError() {
		return nil, diags
	}

	return &RecordingRuleGroupSetResourceModel{
		ID:     types.StringValue(resp.GetId()),
		Name:   types.StringValue(resp.GetName()),
		Groups: groups,
	}, nil
}

func flattenRecordingRuleGroups(ctx context.Context, groups []*rrgs.OutRuleGroup) (types.Set, diag.Diagnostics) {
	var diags diag.Diagnostics
	var groupsObjects []types.Object
	for _, group := range groups {
		flattenedGroup, flattenDiags := flattenRecordingRuleGroup(ctx, group)
		if flattenDiags.HasError() {
			diags.Append(flattenDiags...)
			continue
		}
		groupObject, flattenDiags := types.ObjectValueFrom(ctx, recordingRuleGroupAttributes(), flattenedGroup)
		if flattenDiags.HasError() {
			diags.Append(flattenDiags...)
			continue
		}
		groupsObjects = append(groupsObjects, groupObject)
	}
	if diags.HasError() {
		return types.SetNull(types.ObjectType{AttrTypes: recordingRuleGroupAttributes()}), diags
	}

	return types.SetValueFrom(ctx, types.ObjectType{AttrTypes: recordingRuleGroupAttributes()}, groupsObjects)
}

func recordingRuleGroupAttributes() map[string]attr.Type {
	return map[string]attr.Type{
		"name":     types.StringType,
		"interval": types.Int64Type,
		"limit":    types.Int64Type,
		"rules": types.SetType{
			ElemType: types.ObjectType{
				AttrTypes: recordingRuleAttributes(),
			},
		},
	}
}

func recordingRuleAttributes() map[string]attr.Type {
	return map[string]attr.Type{
		"record": types.StringType,
		"expr":   types.StringType,
		"labels": types.MapType{
			ElemType: types.StringType,
		},
	}
}

func flattenRecordingRuleGroup(ctx context.Context, group *rrgs.OutRuleGroup) (*RecordingRuleGroupModel, diag.Diagnostics) {
	rules, diags := flattenRecordingRules(ctx, group.GetRules())
	if diags.HasError() {
		return nil, diags
	}

	return &RecordingRuleGroupModel{
		Name:     types.StringValue(group.GetName()),
		Interval: types.Int64Value(int64(group.GetInterval())),
		Limit:    types.Int64Value(int64(group.GetLimit())),
		Rules:    rules,
	}, nil
}

func flattenRecordingRules(ctx context.Context, rules []*rrgs.OutRule) (types.Set, diag.Diagnostics) {
	var diags diag.Diagnostics
	var rulesObjects []types.Object
	for _, rule := range rules {
		flattenedRule, expandDiags := flattenRecordingRule(ctx, rule)
		if expandDiags.HasError() {
			diags.Append(expandDiags...)
			continue
		}
		ruleObject, expandDiags := types.ObjectValueFrom(ctx, recordingRuleAttributes(), flattenedRule)
		if expandDiags.HasError() {
			diags.Append(expandDiags...)
			continue
		}
		rulesObjects = append(rulesObjects, ruleObject)
	}
	if diags.HasError() {
		return types.SetNull(types.ObjectType{AttrTypes: recordingRuleAttributes()}), diags
	}

	return types.SetValueFrom(ctx, types.ObjectType{AttrTypes: recordingRuleAttributes()}, rulesObjects)
}

func flattenRecordingRule(ctx context.Context, rule *rrgs.OutRule) (*RecordingRuleModel, diag.Diagnostics) {
	labels, diags := types.MapValueFrom(ctx, types.StringType, rule.GetLabels())
	if diags.HasError() {
		return nil, diags
	}

	return &RecordingRuleModel{
		Record: types.StringValue(rule.GetRecord()),
		Expr:   types.StringValue(rule.GetExpr()),
		Labels: labels,
	}, nil
}

func (r RecordingRuleGroupSetResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	//TODO implement me
	panic("implement me")
}

func (r RecordingRuleGroupSetResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	//TODO implement me
	panic("implement me")
}

func (r RecordingRuleGroupSetResource) Delete(ctx context.Context, request resource.DeleteRequest, response *resource.DeleteResponse) {
	//TODO implement me
	panic("implement me")
}

type RecordingRuleGroupSetResourceModel struct {
	ID          types.String `tfsdk:"id"`
	YamlContent types.String `tfsdk:"yaml_content"`
	Groups      types.Set    `tfsdk:"groups"` //RecordingRuleGroupModel
	Name        types.String `tfsdk:"name"`
}

type RecordingRuleGroupModel struct {
	Name     types.String `tfsdk:"name"`
	Interval types.Int64  `tfsdk:"interval"`
	Limit    types.Int64  `tfsdk:"limit"`
	Rules    types.Set    `tfsdk:"rules"` //RecordingRuleModel
}

type RecordingRuleModel struct {
	Record types.String `tfsdk:"record"`
	Expr   types.String `tfsdk:"expr"`
	Labels types.Map    `tfsdk:"labels"`
}

func expandRecordingRulesGroupsSet(ctx context.Context, plan *RecordingRuleGroupSetResourceModel) (*rrgs.CreateRuleGroupSet, diag.Diagnostics) {
	if yamlContent := plan.YamlContent.ValueString(); yamlContent != "" {
		return expandRecordingRulesGroupsSetFromYaml(yamlContent)
	}

	return expandRecordingRulesGroupSetExplicitly(ctx, plan)
}

func expandRecordingRulesGroupsSetFromYaml(yamlContent string) (*rrgs.CreateRuleGroupSet, diag.Diagnostics) {
	var result rrgs.CreateRuleGroupSet
	if err := yaml.Unmarshal([]byte(yamlContent), &result); err != nil {
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("", "")}
	}
	return &result, nil
}

func expandRecordingRulesGroupSetExplicitly(ctx context.Context, plan *RecordingRuleGroupSetResourceModel) (*rrgs.CreateRuleGroupSet, diag.Diagnostics) {
	name := plan.Name.ValueString()
	groups, diags := expandRecordingRulesGroups(ctx, plan.Groups)
	if diags.HasError() {
		return nil, diags
	}

	return &rrgs.CreateRuleGroupSet{
		Name:   &name,
		Groups: groups,
	}, nil
}

func expandRecordingRulesGroups(ctx context.Context, groups types.Set) ([]*rrgs.InRuleGroup, diag.Diagnostics) {
	var diags diag.Diagnostics
	var groupsObjects []types.Object
	var expandedGroups []*rrgs.InRuleGroup
	groups.ElementsAs(ctx, &groupsObjects, true)

	for _, groupObject := range groupsObjects {
		var group RecordingRuleGroupModel
		if dg := groupObject.As(ctx, &group, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expandedGroup, expandDiags := expandRecordingRuleGroup(ctx, group)
		if expandDiags.HasError() {
			diags.Append(expandDiags...)
			continue
		}
		expandedGroups = append(expandedGroups, expandedGroup)
	}

	return expandedGroups, diags
}

func expandRecordingRuleGroup(ctx context.Context, group RecordingRuleGroupModel) (*rrgs.InRuleGroup, diag.Diagnostics) {
	interval := uint32(group.Interval.ValueInt64())
	limit := uint64(group.Limit.ValueInt64())
	rules, diags := expandRecordingRules(ctx, group.Rules)
	if diags.HasError() {
		return nil, diags
	}

	return &rrgs.InRuleGroup{
		Name:     group.Name.ValueString(),
		Interval: &interval,
		Limit:    &limit,
		Rules:    rules,
	}, nil
}

func expandRecordingRules(ctx context.Context, rules types.Set) ([]*rrgs.InRule, diag.Diagnostics) {
	var diags diag.Diagnostics
	var rulesObjects []types.Object
	var expandedRules []*rrgs.InRule
	rules.ElementsAs(ctx, &rulesObjects, true)

	for _, ruleObject := range rulesObjects {
		var rule RecordingRuleModel
		if dg := ruleObject.As(ctx, &rule, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expandedRule, expandDiags := expandRecordingRule(ctx, rule)
		if expandDiags.HasError() {
			diags.Append(expandDiags...)
			continue
		}
		expandedRules = append(expandedRules, expandedRule)
	}

	return expandedRules, diags
}

func expandRecordingRule(ctx context.Context, rule RecordingRuleModel) (*rrgs.InRule, diag.Diagnostics) {
	labels, diags := typeMapToStringMap(ctx, rule.Labels)
	if diags.HasError() {
		return nil, diags
	}

	return &rrgs.InRule{
		Record: rule.Record.ValueString(),
		Expr:   rule.Expr.ValueString(),
		Labels: labels,
	}, nil
}

type recordingRulesGroupYamlContentValidator struct{}

func (v recordingRulesGroupYamlContentValidator) Description(ctx context.Context) string {
	return "validate yaml_content"
}

func (v recordingRulesGroupYamlContentValidator) MarkdownDescription(ctx context.Context) string {
	return "validate yaml_content"
}

func (v recordingRulesGroupYamlContentValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	var set rrgs.CreateRuleGroupSet
	if err := yaml.Unmarshal([]byte(req.ConfigValue.ValueString()), &set); err != nil {
		resp.Diagnostics.AddError("error on validating yaml_content", err.Error())
	}

	groups := set.Groups
	if len(groups) == 0 {
		resp.Diagnostics.AddError("error on validating yaml_content", "groups list can not be empty")
	}

	for i, group := range groups {
		if group.Name == "" {
			resp.Diagnostics.AddError("error on validating yaml_content", fmt.Sprintf("groups[%d] name can not be empty", i))
		}
		if group.Interval == nil {
			resp.Diagnostics.AddError("error on validating yaml_content", fmt.Sprintf("groups[%d] limit have to be set", i))
		}
	}
}
