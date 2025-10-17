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

package notifications

import (
	"context"
	"fmt"
	"log"
	"strings"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/encoding/protojson"
)

var (
	notificationsEntityTypeSchemaToProto = map[string]cxsdk.NotificationsEntityType{
		"unspecified": cxsdk.NotificationsEntityTypeUnspecified,
		"alerts":      cxsdk.NotificationsEntityTypeAlerts,
	}
	notificationsEntityTypeProtoToSchema = utils.ReverseMap(notificationsEntityTypeSchemaToProto)
	validNotificationsEntityTypes        = utils.GetKeys(notificationsEntityTypeSchemaToProto)
)

func NewGlobalRouterResource() resource.Resource {
	return &GlobalRouterResource{}
}

type GlobalRouterResource struct {
	client *cxsdk.NotificationsClient
}

type GlobalRouterResourceModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Description  types.String `tfsdk:"description"`
	EntityType   types.String `tfsdk:"entity_type"`
	Rules        types.List   `tfsdk:"rules"`    // RoutingRuleModel
	FallBack     types.List   `tfsdk:"fallback"` // RoutingTargetModel
	EntityLabels types.Map    `tfsdk:"entity_labels"`
}

type RoutingRuleModel struct {
	Condition     types.String `tfsdk:"condition"`
	Targets       types.List   `tfsdk:"targets"` // RoutingTargetModel
	CustomDetails types.Map    `tfsdk:"custom_details"`
	Name          types.String `tfsdk:"name"`
}

type RoutingTargetModel struct {
	ConnectorId   types.String `tfsdk:"connector_id"`
	PresetId      types.String `tfsdk:"preset_id"`
	CustomDetails types.Map    `tfsdk:"custom_details"`
}

func (r *GlobalRouterResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_global_router"
}

func (r *GlobalRouterResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	r.client = clientSet.GetNotifications()
}

func (r *GlobalRouterResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version: 0,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Description:   "The ID of the GlobalRouter.",
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Name of the GlobalRouter.",
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "Description of the GlobalRouter.",
			},
			"entity_type": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Validators: []validator.String{
					stringvalidator.OneOf(validNotificationsEntityTypes...),
				},
				Description: "Type of the entity. Valid values are: " + strings.Join(validNotificationsEntityTypes, ", "),
			},
			"rules": schema.ListNestedAttribute{
				Optional:    true,
				Description: "Routing rules for the GlobalRouter.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"condition": schema.StringAttribute{
							Required: true,
						},
						"targets": schema.ListNestedAttribute{
							Optional:    true,
							Description: "Routing targets for the rule.",
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"connector_id": schema.StringAttribute{
										Required:    true,
										Description: "ID of the connector.",
									},
									"preset_id": schema.StringAttribute{
										Optional:    true,
										Description: "ID of the preset.",
									},
									"custom_details": schema.MapAttribute{
										Optional:    true,
										ElementType: types.StringType,
										Description: "Custom details for the target.",
									},
								},
							},
						},
						"custom_details": schema.MapAttribute{
							Optional:    true,
							ElementType: types.StringType,
							Description: "Custom details for the rule.",
						},
						"name": schema.StringAttribute{
							Required:    true,
							Description: "Name of the routing rule.",
						},
					},
				},
			},
			"fallback": schema.ListNestedAttribute{
				Optional:    true,
				Description: "Fallback routing targets.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"connector_id": schema.StringAttribute{
							Required:    true,
							Description: "ID of the connector.",
						},
						"preset_id": schema.StringAttribute{
							Optional:    true,
							Description: "ID of the preset.",
						},
						"custom_details": schema.MapAttribute{
							Optional:    true,
							ElementType: types.StringType,
							Description: "Custom details for the target.",
						},
					},
				},
			},
			"entity_labels": schema.MapAttribute{
				Optional:    true,
				ElementType: types.StringType,
			},
		},
		MarkdownDescription: "Coralogix GlobalRouter. **Note:** This resource is in alpha stage.",
	}
}

func (r *GlobalRouterResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *GlobalRouterResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan *GlobalRouterResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	router, diags := extractRouter(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	createGlobalRouterRequest := &cxsdk.CreateOrReplaceGlobalRouterRequest{
		Router: router,
	}

	log.Printf("[INFO] Creating new GlobalRouter: %s", protojson.Format(createGlobalRouterRequest))
	createResp, err := r.client.CreateOrReplaceGlobalRouter(ctx, createGlobalRouterRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err)
		resp.Diagnostics.AddError("Error creating GlobalRouter",
			//TODO: add the proper url
			utils.FormatRpcErrors(err, "", protojson.Format(createGlobalRouterRequest)),
		)
		return
	}
	log.Printf("[INFO] Submitted new GlobalRouter: %s", protojson.Format(createResp))
	plan, diags = flattenGlobalRouter(ctx, createResp.Router)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func (r *GlobalRouterResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state *GlobalRouterResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	//Get refreshed GlobalRouter value from Coralogix
	id := state.ID.ValueString()
	getGlobalRouterReq := &cxsdk.GetGlobalRouterRequest{Id: id}
	getGlobalRouterResp, err := r.client.GetGlobalRouter(ctx, getGlobalRouterReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if cxsdk.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("GlobalRouter %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%q will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError(
				"Error reading GlobalRouter",
				//TODO: add the proper url
				utils.FormatRpcErrors(err, "", protojson.Format(getGlobalRouterReq)),
			)
		}
		return
	}
	GlobalRouter := getGlobalRouterResp.GetRouter()
	log.Printf("[INFO] Received GlobalRouter: %s", protojson.Format(GlobalRouter))

	state, diags = flattenGlobalRouter(ctx, GlobalRouter)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	//
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r GlobalRouterResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var plan *GlobalRouterResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	router, diags := extractRouter(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	updateRouter := &cxsdk.CreateOrReplaceGlobalRouterRequest{
		Router: router,
	}
	log.Printf("[INFO] Updating GlobalRouter: %s", protojson.Format(updateRouter))
	updateRouterResp, err := r.client.CreateOrReplaceGlobalRouter(ctx, updateRouter)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if cxsdk.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("GlobalRouter %q is in state, but no longer exists in Coralogix backend", *router.Id),
				fmt.Sprintf("%s will be recreated when you apply", *router.Id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError(
				"Error reading GlobalRouter",
				//TODO: add the proper url
				utils.FormatRpcErrors(err, "", protojson.Format(updateRouterResp)),
			)
		}
		return
	}
	log.Printf("[INFO] Submitted updated GlobalRouter: %s", protojson.Format(updateRouterResp))

	plan, diags = flattenGlobalRouter(ctx, updateRouterResp.GetRouter())
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func (r GlobalRouterResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state GlobalRouterResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	deleteReq := &cxsdk.DeleteGlobalRouterRequest{Id: id}
	if _, err := r.client.DeleteGlobalRouter(ctx, deleteReq); err != nil {
		resp.Diagnostics.AddError(
			fmt.Sprintf("Error Deleting GlobalRouter %s", id),
			//TODO: add the proper url
			utils.FormatRpcErrors(err, "", protojson.Format(deleteReq)),
		)
		return
	}
	log.Printf("[INFO] GlobalRouter %s deleted", id)
}

func extractRouter(ctx context.Context, plan *GlobalRouterResourceModel) (*cxsdk.GlobalRouter, diag.Diagnostics) {
	rules, diags := extractGlobalRouterRules(ctx, plan.Rules)
	if diags.HasError() {
		return nil, diags
	}

	fallback, diags := extractRoutingTargets(ctx, plan.FallBack)
	if diags.HasError() {
		return nil, diags
	}

	entityLabels, diags := utils.ExtractStringMap(ctx, plan.EntityLabels)
	if diags.HasError() {
		return nil, diags
	}

	routerId := "router_default"
	return &cxsdk.GlobalRouter{
		Id:           &routerId,
		Name:         plan.Name.ValueString(),
		Description:  plan.Description.ValueString(),
		Rules:        rules,
		Fallback:     fallback,
		EntityLabels: entityLabels,
	}, nil
}

func extractGlobalRouterRules(ctx context.Context, rules types.List) ([]*cxsdk.RoutingRule, diag.Diagnostics) {
	var diags diag.Diagnostics
	var rulesObjects []types.Object
	rules.ElementsAs(ctx, &rulesObjects, true)
	extractedRules := make([]*cxsdk.RoutingRule, 0, len(rulesObjects))
	for _, rule := range rulesObjects {
		var ruleModel RoutingRuleModel
		if dg := rule.As(ctx, &ruleModel, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		extractedRule, dg := extractRoutingRule(ctx, ruleModel)
		if diags.HasError() {
			diags.Append(dg...)
			continue
		}
		extractedRules = append(extractedRules, extractedRule)
	}

	if diags.HasError() {
		return nil, diags
	}

	return extractedRules, diags
}

func extractRoutingRule(ctx context.Context, routingModel RoutingRuleModel) (*cxsdk.RoutingRule, diag.Diagnostics) {
	var diags diag.Diagnostics
	targets, dg := extractRoutingTargets(ctx, routingModel.Targets)
	if dg.HasError() {
		diags.Append(dg...)
		return nil, diags
	}

	customDetails, dg := utils.ExtractStringMap(ctx, routingModel.CustomDetails)
	if dg.HasError() {
		diags.Append(dg...)
		return nil, diags
	}

	return &cxsdk.RoutingRule{
		Name:          utils.TypeStringToStringPointer(routingModel.Name),
		Condition:     routingModel.Condition.ValueString(),
		Targets:       targets,
		CustomDetails: customDetails,
	}, nil
}

func extractRoutingTargets(ctx context.Context, targets types.List) ([]*cxsdk.RoutingTarget, diag.Diagnostics) {
	var diags diag.Diagnostics
	var targetsObjects []types.Object
	targets.ElementsAs(ctx, &targetsObjects, true)
	extractedTargets := make([]*cxsdk.RoutingTarget, 0, len(targetsObjects))
	for _, target := range targetsObjects {
		var targetModel RoutingTargetModel
		if dg := target.As(ctx, &targetModel, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		extractedTarget, dgs := extractRoutingTarget(ctx, targetModel)
		if dgs.HasError() {
			diags.Append(dgs...)
			continue
		}
		extractedTargets = append(extractedTargets, extractedTarget)
	}

	if diags.HasError() {
		return nil, diags
	}

	return extractedTargets, diags

}

func extractRoutingTarget(ctx context.Context, routingTargetModel RoutingTargetModel) (*cxsdk.RoutingTarget, diag.Diagnostics) {
	customDetails, diags := utils.ExtractStringMap(ctx, routingTargetModel.CustomDetails)
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.RoutingTarget{
		ConnectorId:   routingTargetModel.ConnectorId.ValueString(),
		PresetId:      utils.TypeStringToStringPointer(routingTargetModel.PresetId),
		CustomDetails: customDetails,
	}, nil
}

func flattenGlobalRouter(ctx context.Context, GlobalRouter *cxsdk.GlobalRouter) (*GlobalRouterResourceModel, diag.Diagnostics) {
	rules, diags := flattenGlobalRouterRules(ctx, GlobalRouter.GetRules())
	if diags.HasError() {
		return nil, diags
	}

	return &GlobalRouterResourceModel{
		ID:           types.StringValue(GlobalRouter.GetId()),
		Name:         types.StringValue(GlobalRouter.GetName()),
		Description:  types.StringValue(GlobalRouter.GetDescription()),
		Rules:        rules,
		FallBack:     types.ListNull(types.ObjectType{AttrTypes: routingTargetAttr()}),
		EntityLabels: types.MapNull(types.StringType),
	}, nil
}

func flattenGlobalRouterRules(ctx context.Context, rules []*cxsdk.RoutingRule) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	rulesList := make([]types.Object, 0, len(rules))
	for _, rule := range rules {
		ruleModel, dgs := flattenRoutingRule(ctx, rule)
		if dgs.HasError() {
			diags.Append(dgs...)
			continue
		}
		rulesList = append(rulesList, ruleModel)
	}

	if diags.HasError() {
		return types.List{}, diags
	}

	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: routingRuleAttr()}, rulesList)
}

func flattenRoutingRule(ctx context.Context, rule *cxsdk.RoutingRule) (types.Object, diag.Diagnostics) {
	targets, diags := flattenRoutingTargets(ctx, rule.GetTargets())
	if diags.HasError() {
		return types.ObjectNull(routingRuleAttr()), diags
	}

	customDetails, diags := flattenCustomDetails(ctx, rule.GetCustomDetails())
	if diags.HasError() {
		return types.ObjectNull(routingRuleAttr()), diags
	}

	ruleModel := RoutingRuleModel{
		Condition:     types.StringValue(rule.GetCondition()),
		Name:          types.StringValue(rule.GetName()),
		Targets:       targets,
		CustomDetails: customDetails,
	}

	return types.ObjectValueFrom(ctx, routingRuleAttr(), ruleModel)
}

func flattenCustomDetails(ctx context.Context, details map[string]string) (types.Map, diag.Diagnostics) {
	var diags diag.Diagnostics
	if details == nil {
		return types.MapNull(types.StringType), diags
	}

	detailsMap := make(map[string]types.String)
	for k, v := range details {
		detailsMap[k] = types.StringValue(v)
	}

	return types.MapValueFrom(ctx, types.StringType, detailsMap)
}

func flattenRoutingTargets(ctx context.Context, targets []*cxsdk.RoutingTarget) (types.List, diag.Diagnostics) {
	if targets == nil {
		return types.ListNull(types.ObjectType{AttrTypes: routingTargetAttr()}), nil
	}

	var diags diag.Diagnostics
	targetsList := make([]types.Object, 0, len(targets))
	for _, target := range targets {
		targetModel, dgs := flattenRoutingTarget(ctx, target)
		if dgs.HasError() {
			diags.Append(dgs...)
			continue
		}
		targetsList = append(targetsList, targetModel)
	}

	if diags.HasError() {
		return types.ListNull(types.ObjectType{AttrTypes: routingTargetAttr()}), diags
	}

	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: routingTargetAttr()}, targetsList)
}

func flattenRoutingTarget(ctx context.Context, target *cxsdk.RoutingTarget) (types.Object, diag.Diagnostics) {
	customDetails, diags := flattenCustomDetails(ctx, target.GetCustomDetails())
	if diags.HasError() {
		return types.ObjectNull(routingTargetAttr()), diags
	}

	targetModel := RoutingTargetModel{
		ConnectorId:   types.StringValue(target.ConnectorId),
		PresetId:      utils.StringPointerToTypeString(target.PresetId),
		CustomDetails: customDetails,
	}

	return types.ObjectValueFrom(ctx, routingTargetAttr(), targetModel)
}

func flattenConnectorConfigFields(ctx context.Context, configFields []*cxsdk.ConnectorConfigField) (types.Set, diag.Diagnostics) {
	var diags diag.Diagnostics
	if configFields == nil {
		return types.SetNull(types.ObjectType{AttrTypes: connectorConfigFieldAttrs()}), diags
	}

	configFieldsList := make([]ConnectorConfigFieldModel, 0, len(configFields))
	for _, field := range configFields {
		fieldModel := ConnectorConfigFieldModel{
			FieldName: types.StringValue(field.GetFieldName()),
			Value:     types.StringValue(field.GetValue()),
		}
		configFieldsList = append(configFieldsList, fieldModel)
	}

	return types.SetValueFrom(ctx, types.ObjectType{AttrTypes: connectorConfigFieldAttrs()}, configFieldsList)
}

func routingRuleAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"condition":      types.StringType,
		"name":           types.StringType,
		"targets":        types.ListType{ElemType: types.ObjectType{AttrTypes: routingTargetAttr()}},
		"custom_details": types.MapType{ElemType: types.StringType},
	}
}

func routingTargetAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"connector_id":   types.StringType,
		"preset_id":      types.StringType,
		"custom_details": types.MapType{ElemType: types.StringType},
	}
}

func messageConfigFieldAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"field_name": types.StringType,
		"template":   types.StringType,
	}
}

func configOverridesAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"field_name": types.StringType,
		"template":   types.StringType,
	}
}
