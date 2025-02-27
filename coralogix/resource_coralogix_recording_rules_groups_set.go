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

	"terraform-provider-coralogix/coralogix/clientset"
	"terraform-provider-coralogix/coralogix/utils"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
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
	"gopkg.in/yaml.v3"
)

var (
	_                  resource.ResourceWithConfigure    = &RecordingRuleGroupSetResource{}
	_                  resource.ResourceWithImportState  = &RecordingRuleGroupSetResource{}
	_                  resource.ResourceWithUpgradeState = &RecordingRuleGroupSetResource{}
	createRuleGroupURL                                   = "rule_manager.groups.RuleGroupSets/Create"
	getRuleGroupURL                                      = "rule_manager.groups.RuleGroupSets/Fetch"
	updateRuleGroupURL                                   = "rule_manager.groups.RuleGroupSets/Update"
	deleteRuleGroupURL                                   = "rule_manager.groups.RuleGroupSets/Delete"
)

func NewRecordingRuleGroupSetResource() resource.Resource {
	return &RecordingRuleGroupSetResource{}
}

type RecordingRuleGroupSetResource struct {
	client *cxsdk.RecordingRuleGroupSetsClient
}

func (r *RecordingRuleGroupSetResource) UpgradeState(_ context.Context) map[int64]resource.StateUpgrader {
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
			"rule": schema.SetNestedAttribute{
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
	type RecordingRuleGroupSetResourceModelV0 struct {
		ID          types.String `tfsdk:"id"`
		YamlContent types.String `tfsdk:"yaml_content"`
		Group       types.Set    `tfsdk:"group"` //RecordingRuleGroupModelV0
		Name        types.String `tfsdk:"name"`
	}

	var priorStateData RecordingRuleGroupSetResourceModelV0
	resp.Diagnostics.Append(req.State.Get(ctx, &priorStateData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	groups, diags := upgradeRecordingRulesGroupsV0(ctx, priorStateData.Group)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	upgradedStateData := RecordingRuleGroupSetResourceModel{
		ID:          priorStateData.ID,
		YamlContent: priorStateData.YamlContent,
		Name:        priorStateData.Name,
		Groups:      groups,
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, upgradedStateData)...)
}

func upgradeRecordingRulesGroupsV0(ctx context.Context, groups types.Set) (types.Set, diag.Diagnostics) {
	type RecordingRuleGroupModelV0 struct {
		Name     types.String `tfsdk:"name"`
		Interval types.Int64  `tfsdk:"interval"`
		Limit    types.Int64  `tfsdk:"limit"`
		Rule     types.Set    `tfsdk:"rule"` //RecordingRuleModel
	}

	var diags diag.Diagnostics
	var priorGroupsObjects []types.Object
	var upgradedGroups []RecordingRuleGroupModel
	groups.ElementsAs(ctx, &priorGroupsObjects, true)

	for _, groupObject := range priorGroupsObjects {
		var priorGroup RecordingRuleGroupModelV0
		if dg := groupObject.As(ctx, &priorGroup, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}

		rules, dg := upgradeRecordingRulesV0(ctx, priorGroup.Rule)
		if dg.HasError() {
			diags.Append(dg...)
			continue
		}

		upgradedGroup := RecordingRuleGroupModel{
			Name:     priorGroup.Name,
			Interval: priorGroup.Interval,
			Limit:    priorGroup.Limit,
			Rules:    rules,
		}

		upgradedGroups = append(upgradedGroups, upgradedGroup)
	}

	if diags.HasError() {
		return types.SetNull(types.ObjectType{AttrTypes: recordingRuleGroupAttributes()}), diags
	}

	return types.SetValueFrom(ctx, types.ObjectType{AttrTypes: recordingRuleGroupAttributes()}, upgradedGroups)
}

func upgradeRecordingRulesV0(ctx context.Context, rule types.Set) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	var priorRulesObjects []types.Object
	var upgradedRules []RecordingRuleModel
	rule.ElementsAs(ctx, &priorRulesObjects, true)

	for _, ruleObject := range priorRulesObjects {
		var priorRule RecordingRuleModel
		if dg := ruleObject.As(ctx, &priorRule, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}

		upgradedRule := RecordingRuleModel{
			Record: priorRule.Record,
			Expr:   priorRule.Expr,
			Labels: priorRule.Labels,
		}

		upgradedRules = append(upgradedRules, upgradedRule)
	}

	if diags.HasError() {
		return types.ListNull(types.ObjectType{AttrTypes: recordingRuleAttributes()}), diags
	}

	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: recordingRuleAttributes()}, upgradedRules)
}

func (r *RecordingRuleGroupSetResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *RecordingRuleGroupSetResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *RecordingRuleGroupSetResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_recording_rules_groups_set"
}

func (r *RecordingRuleGroupSetResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
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
					stringvalidator.ConflictsWith(
						path.MatchRelative().AtParent().AtName("groups"),
						path.MatchRelative().AtParent().AtName("name"),
					),
					recordingRulesGroupYamlContentValidator{},
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplaceIf(JSONStringsEqualPlanModifier, "", ""),
				},
			},
			"groups": schema.SetNestedAttribute{
				Optional:     true,
				Computed:     true,
				NestedObject: recordingRuleGroupSchema(),
				Validators: []validator.Set{
					setvalidator.ExactlyOneOf(
						path.MatchRelative().AtParent().AtName("yaml_content"),
					),
				},
				PlanModifiers: []planmodifier.Set{
					setplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Validators: []validator.String{
					stringvalidator.ExactlyOneOf(
						path.MatchRelative().AtParent().AtName("yaml_content")),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func recordingRuleGroupSchema() schema.NestedAttributeObject {
	return schema.NestedAttributeObject{
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The (unique) rule-group name.",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"interval": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "How often rules in the group are evaluated (in seconds). Default is 60 seconds.",
				Validators: []validator.Int64{
					int64validator.AtLeast(60),
				},
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"limit": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Limits the number of alerts an alerting rule and series a recording-rule can produce. 0 is no limit.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
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

func (r *RecordingRuleGroupSetResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan *RecordingRuleGroupSetResourceModel
	diags := req.Plan.Get(ctx, &plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	createRequest, diags := expandRecordingRulesGroupsSet(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	log.Printf("[INFO] Creating new recogring-rule-group-set: %s", protojson.Format(createRequest))

	createResp, err := r.client.Create(ctx, createRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error creating recording-rule-group-set",
			utils.FormatRpcErrors(err, createRuleGroupURL, protojson.Format(createRequest)),
		)
		return
	}
	id := createResp.GetId()
	log.Printf("[INFO] Submitted new recording-rule-group-set id: %s", id)
	plan.ID = types.StringValue(id)

	log.Printf("[INFO] Reading recording-rule-group-set id: %s", id)
	getResp, err := r.client.Get(ctx, &cxsdk.GetRuleGroupSetRequest{Id: id})
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if cxsdk.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("recording-rule-group-set %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError(
				"Error reading recording-rule-group-set",
				utils.FormatRpcErrors(err, getRuleGroupURL, protojson.Format(createRequest)),
			)
		}
		return
	}

	log.Printf("[INFO] Received recogring-rule-group-set: %s", protojson.Format(getResp))

	plan, diags = flattenRecordingRuleGroupSet(ctx, plan, getResp)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func flattenRecordingRuleGroupSet(ctx context.Context, plan *RecordingRuleGroupSetResourceModel, resp *cxsdk.GetRuleGroupSetResponse) (*RecordingRuleGroupSetResourceModel, diag.Diagnostics) {
	if yamlContent := plan.YamlContent.ValueString(); yamlContent != "" {
		groups, diags := flattenRecordingRuleGroups(ctx, resp.GetGroups())
		if diags.HasError() {
			return nil, diags
		}

		return &RecordingRuleGroupSetResourceModel{
			ID:          types.StringValue(resp.GetId()),
			YamlContent: types.StringValue(plan.YamlContent.ValueString()),
			Name:        types.StringValue(plan.Name.ValueString()),
			Groups:      groups,
		}, nil
	}

	groups, diags := flattenRecordingRuleGroups(ctx, resp.GetGroups())
	if diags.HasError() {
		return nil, diags
	}

	return &RecordingRuleGroupSetResourceModel{
		ID:          types.StringValue(resp.GetId()),
		Name:        types.StringValue(resp.GetName()),
		Groups:      groups,
		YamlContent: types.StringNull(),
	}, nil
}

func flattenRecordingRuleGroups(ctx context.Context, groups []*cxsdk.OutRuleGroup) (types.Set, diag.Diagnostics) {
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
		"rules": types.ListType{
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

func flattenRecordingRuleGroup(ctx context.Context, group *cxsdk.OutRuleGroup) (*RecordingRuleGroupModel, diag.Diagnostics) {
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

func flattenRecordingRules(ctx context.Context, rules []*cxsdk.OutRule) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	var rulesObjects []types.Object
	for _, rule := range rules {
		flattenedRule, flattenedDiags := flattenRecordingRule(ctx, rule)
		if flattenedDiags.HasError() {
			diags.Append(flattenedDiags...)
			continue
		}
		ruleObject, flattenedDiags := types.ObjectValueFrom(ctx, recordingRuleAttributes(), flattenedRule)
		if flattenedDiags.HasError() {
			diags.Append(flattenedDiags...)
			continue
		}
		rulesObjects = append(rulesObjects, ruleObject)
	}
	if diags.HasError() {
		return types.ListNull(types.ObjectType{AttrTypes: recordingRuleAttributes()}), diags
	}

	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: recordingRuleAttributes()}, rulesObjects)
}

func flattenRecordingRule(ctx context.Context, rule *cxsdk.OutRule) (*RecordingRuleModel, diag.Diagnostics) {
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

func (r *RecordingRuleGroupSetResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state *RecordingRuleGroupSetResourceModel
	diags := req.State.Get(ctx, &state)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	id := state.ID.ValueString()
	log.Printf("[INFO] Reading recording-rule-group-set id: %s", id)
	getReq := &cxsdk.GetRuleGroupSetRequest{Id: id}
	getResp, err := r.client.Get(ctx, getReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if cxsdk.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("recording-rule-group-set %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError(
				"Error reading recording-rule-group-set",
				utils.FormatRpcErrors(err, getRuleGroupURL, protojson.Format(getReq)),
			)
		}
		return
	}

	state, diags = flattenRecordingRuleGroupSet(ctx, state, getResp)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	// Set state to fully populated data
	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
}

func (r *RecordingRuleGroupSetResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan *RecordingRuleGroupSetResourceModel
	diags := req.Plan.Get(ctx, &plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	updateRequest, diags := expandUpdateRecordingRulesGroupsSet(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	log.Printf("[INFO] Updating recording-rule-group-set: %s", protojson.Format(updateRequest))

	_, err := r.client.Update(ctx, updateRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error updating recording-rule-group-set",
			utils.FormatRpcErrors(err, updateRuleGroupURL, protojson.Format(updateRequest)),
		)
		return
	}

	log.Printf("[INFO] Reading recording-rule-group-set id: %s", plan.ID.ValueString())
	getReq := &cxsdk.GetRuleGroupSetRequest{Id: plan.ID.ValueString()}
	getResp, err := r.client.Get(ctx, getReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if cxsdk.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("recording-rule-group-set %q is in state, but no longer exists in Coralogix backend", plan.ID.ValueString()),
				fmt.Sprintf("%s will be recreated when you apply", plan.ID.ValueString()),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError(
				"Error reading recording-rule-group-set",
				utils.FormatRpcErrors(err, getRuleGroupURL, protojson.Format(getReq)),
			)
		}
		return
	}
	log.Printf("[INFO] Received recogring-rule-group-set: %s", protojson.Format(getResp))

	plan, diags = flattenRecordingRuleGroupSet(ctx, plan, getResp)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func (r *RecordingRuleGroupSetResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state *RecordingRuleGroupSetResourceModel
	diags := req.State.Get(ctx, &state)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	id := state.ID.ValueString()
	log.Printf("[INFO] Deleting recording-rule-group-set id: %s", id)
	deleteReq := &cxsdk.DeleteRuleGroupSetRequest{Id: id}
	_, err := r.client.Delete(ctx, deleteReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if cxsdk.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("recording-rule-group-set %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError(
				"Error deleting recording-rule-group-set",
				utils.FormatRpcErrors(err, deleteRuleGroupURL, protojson.Format(deleteReq)),
			)
		}
		return
	}
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
	Rules    types.List   `tfsdk:"rules"` //RecordingRuleModel
}

type RecordingRuleModel struct {
	Record types.String `tfsdk:"record"`
	Expr   types.String `tfsdk:"expr"`
	Labels types.Map    `tfsdk:"labels"`
}

func expandRecordingRulesGroupsSet(ctx context.Context, plan *RecordingRuleGroupSetResourceModel) (*cxsdk.CreateRuleGroupSetRequest, diag.Diagnostics) {
	if yamlContent := plan.YamlContent.ValueString(); yamlContent != "" {
		return expandRecordingRulesGroupsSetFromYaml(yamlContent)
	}

	return expandRecordingRulesGroupSetExplicitly(ctx, plan)
}

func expandUpdateRecordingRulesGroupsSet(ctx context.Context, plan *RecordingRuleGroupSetResourceModel) (*cxsdk.UpdateRuleGroupSetRequest, diag.Diagnostics) {
	if yamlContent := plan.YamlContent.ValueString(); yamlContent != "" {
		rrg, diags := expandRecordingRulesGroupsSetFromYaml(yamlContent)
		if diags.HasError() {
			return nil, diags
		}

		return &cxsdk.UpdateRuleGroupSetRequest{
			Id:     plan.ID.ValueString(),
			Groups: rrg.Groups,
			// Name:   rrg.Name,
		}, nil
	}

	rrg, diags := expandRecordingRulesGroupSetExplicitly(ctx, plan)
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.UpdateRuleGroupSetRequest{
		Id:     plan.ID.ValueString(),
		Groups: rrg.Groups,
		// Name:   rrg.Name,
	}, nil
}

func expandRecordingRulesGroupsSetFromYaml(yamlContent string) (*cxsdk.CreateRuleGroupSetRequest, diag.Diagnostics) {
	var result cxsdk.CreateRuleGroupSetRequest
	if err := yaml.Unmarshal([]byte(yamlContent), &result); err != nil {
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error on unmarshal yaml_content", err.Error())}
	}
	return &result, nil
}

func expandRecordingRulesGroupSetExplicitly(ctx context.Context, plan *RecordingRuleGroupSetResourceModel) (*cxsdk.CreateRuleGroupSetRequest, diag.Diagnostics) {
	name := plan.Name.ValueString()
	groups, diags := expandRecordingRulesGroups(ctx, plan.Groups)
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.CreateRuleGroupSetRequest{
		Name:   &name,
		Groups: groups,
	}, nil
}

func expandRecordingRulesGroups(ctx context.Context, groups types.Set) ([]*cxsdk.InRuleGroup, diag.Diagnostics) {
	var diags diag.Diagnostics
	var groupsObjects []types.Object
	var expandedGroups []*cxsdk.InRuleGroup
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

func expandRecordingRuleGroup(ctx context.Context, group RecordingRuleGroupModel) (*cxsdk.InRuleGroup, diag.Diagnostics) {
	var interval *uint32
	if !(group.Interval.IsNull() || group.Interval.IsUnknown()) {
		interval = new(uint32)
		*interval = uint32(group.Interval.ValueInt64())
	} else {
		interval = new(uint32)
		*interval = 60
	}

	var limit *uint64
	if !group.Limit.IsNull() {
		limit = new(uint64)
		*limit = uint64(group.Limit.ValueInt64())
	}

	rules, diags := expandRecordingRules(ctx, group.Rules)
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.InRuleGroup{
		Name:     group.Name.ValueString(),
		Interval: interval,
		Limit:    limit,
		Rules:    rules,
	}, nil
}

func expandRecordingRules(ctx context.Context, rules types.List) ([]*cxsdk.InRule, diag.Diagnostics) {
	var diags diag.Diagnostics
	var rulesObjects []types.Object
	var expandedRules []*cxsdk.InRule
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

func expandRecordingRule(ctx context.Context, rule RecordingRuleModel) (*cxsdk.InRule, diag.Diagnostics) {
	labels, diags := utils.TypeMapToStringMap(ctx, rule.Labels)
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.InRule{
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
	if req.ConfigValue.IsNull() {
		return
	}

	var set cxsdk.CreateRuleGroupSetRequest
	if err := yaml.Unmarshal([]byte(req.ConfigValue.ValueString()), &set); err != nil {
		resp.Diagnostics.AddError("error on validating yaml_content", err.Error())
	}

	for i, group := range set.Groups {
		if group.Name == "" {
			resp.Diagnostics.AddError("error on validating yaml_content", fmt.Sprintf("groups[%d] name can not be empty", i))
		}
	}
}
