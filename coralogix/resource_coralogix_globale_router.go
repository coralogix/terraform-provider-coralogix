package coralogix

import (
	"context"
	"fmt"
	"log"
	"strings"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/encoding/protojson"
	"terraform-provider-coralogix/coralogix/clientset"
	"terraform-provider-coralogix/coralogix/utils"
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

type ConfigOverridesModel struct {
	MessageConfigFields   types.List `tfsdk:"message_config_fields"`   // MessageConfigFieldModel
	ConnectorConfigFields types.List `tfsdk:"connector_config_fields"` // ConnectorOverrideFieldModel
}

type MessageConfigFieldModel struct {
	FieldName types.String `tfsdk:"field_name"`
	Template  types.String `tfsdk:"template"`
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
				Computed:    true,
				Description: "The ID of the GlobalRouter.",
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
				Required: true,
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
						"name": schema.StringAttribute{
							Required:    true,
							Description: "Name of the routing rule.",
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
									"config_overrides": schema.SingleNestedAttribute{
										Optional:    true,
										Description: "Configuration overrides for the target.",
										Attributes: map[string]schema.Attribute{
											"message_config_fields": schema.ListNestedAttribute{
												Optional: true,
												NestedObject: schema.NestedAttributeObject{
													Attributes: map[string]schema.Attribute{
														"field_name": schema.StringAttribute{
															Required:    true,
															Description: "Name of the field.",
														},
														"template": schema.StringAttribute{
															Optional:    true,
															Description: "Template for the field.",
														},
													},
												},
												Description: "Message configuration fields for the target.",
											},
											"connector_config_fields": schema.ListNestedAttribute{
												Optional: true,
												NestedObject: schema.NestedAttributeObject{
													Attributes: map[string]schema.Attribute{
														"field_name": schema.StringAttribute{
															Required:    true,
															Description: "Name of the field.",
														},
														"template": schema.StringAttribute{
															Optional:    true,
															Description: "Template for the field.",
														},
													},
												},
											},
										},
									},
								},
							},
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
						"config_overrides": schema.SingleNestedAttribute{
							Optional:    true,
							Description: "Configuration overrides for the target.",
							Attributes: map[string]schema.Attribute{
								"message_config_fields": schema.ListNestedAttribute{
									Optional: true,
									NestedObject: schema.NestedAttributeObject{
										Attributes: map[string]schema.Attribute{
											"field_name": schema.StringAttribute{
												Required:    true,
												Description: "Name of the field.",
											},
											"template": schema.StringAttribute{
												Optional:    true,
												Description: "Template for the field.",
											},
										},
									},
								},
								"connector_config_fields": schema.ListNestedAttribute{
									Optional: true,
									NestedObject: schema.NestedAttributeObject{
										Attributes: map[string]schema.Attribute{
											"field_name": schema.StringAttribute{
												Required:    true,
												Description: "Name of the field.",
											},
											"template": schema.StringAttribute{
												Optional:    true,
												Description: "Template for the field.",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			"entity_labels": schema.MapAttribute{
				Optional:    true,
				ElementType: types.StringType,
			},
		},
		MarkdownDescription: "Coralogix GlobalRouter. For more info please review - https://coralogix.com/docs/coralogix-GlobalRouter-extension/.",
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
	createGlobalRouterRequest := &cxsdk.CreateGlobalRouterRequest{
		Router: router,
	}

	log.Printf("[INFO] Creating new GlobalRouter: %s", protojson.Format(createGlobalRouterRequest))
	createResp, err := r.client.CreateGlobalRouter(ctx, createGlobalRouterRequest)
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
	updateRouter := &cxsdk.ReplaceGlobalRouterRequest{
		Router: router,
	}
	log.Printf("[INFO] Updating GlobalRouter: %s", protojson.Format(updateRouter))
	updateRouterResp, err := r.client.ReplaceGlobalRouter(ctx, updateRouter)
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

	return &cxsdk.GlobalRouter{
		Id:           utils.TypeStringToStringPointer(plan.ID),
		EntityType:   notificationsEntityTypeSchemaToProto[plan.EntityType.ValueString()],
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

func extractMessageConfigFields(ctx context.Context, configFields types.List) ([]*cxsdk.MessageConfigField, diag.Diagnostics) {
	var diags diag.Diagnostics
	if configFields.IsNull() || configFields.IsUnknown() {
		return nil, diags
	}

	var configFieldsObjects []types.Object
	configFields.ElementsAs(ctx, &configFieldsObjects, true)
	extractedConfigFields := make([]*cxsdk.MessageConfigField, 0, len(configFieldsObjects))
	for _, field := range configFieldsObjects {
		var fieldModel MessageConfigFieldModel
		if dg := field.As(ctx, &fieldModel, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		extractedConfigField := &cxsdk.MessageConfigField{
			FieldName: fieldModel.FieldName.ValueString(),
			Template:  fieldModel.Template.ValueString(),
		}
		extractedConfigFields = append(extractedConfigFields, extractedConfigField)
	}

	if diags.HasError() {
		return nil, diags
	}

	return extractedConfigFields, diags
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
		ID:          types.StringValue(GlobalRouter.GetId()),
		Name:        types.StringValue(GlobalRouter.GetName()),
		Description: types.StringValue(GlobalRouter.GetDescription()),
		EntityType:  types.StringValue(notificationsEntityTypeProtoToSchema[GlobalRouter.GetEntityType()]),
		Rules:       rules,
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

func flattenMessageConfigFields(ctx context.Context, configFields []*cxsdk.MessageConfigField) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	if configFields == nil {
		return types.ListNull(types.ObjectType{AttrTypes: configOverridesAttr()}), diags
	}

	configFieldsList := make([]MessageConfigFieldModel, 0, len(configFields))
	for _, field := range configFields {
		fieldModel := MessageConfigFieldModel{
			FieldName: types.StringValue(field.GetFieldName()),
			Template:  types.StringValue(field.GetTemplate()),
		}
		configFieldsList = append(configFieldsList, fieldModel)
	}

	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: configOverridesAttr()}, configFieldsList)
}

func flattenConnectorConfigFields(ctx context.Context, configFields []*cxsdk.ConnectorConfigField) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	if configFields == nil {
		return types.ListNull(types.ObjectType{AttrTypes: configOverridesAttr()}), diags
	}

	configFieldsList := make([]ConnectorOverrideFieldModel, 0, len(configFields))

	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: configOverridesAttr()}, configFieldsList)
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
		"connector_id":     types.StringType,
		"preset_id":        types.StringType,
		"custom_details":   types.MapType{ElemType: types.StringType},
		"config_overrides": types.ObjectType{AttrTypes: configOverridesAttr()},
	}
}

func configOverridesAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"message_config_fields":   types.ListType{ElemType: types.ObjectType{AttrTypes: messageConfigFieldAttr()}},
		"connector_config_fields": types.ListType{ElemType: types.ObjectType{AttrTypes: connectorConfigFieldAttr()}},
	}
}

func messageConfigFieldAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"field_name": types.StringType,
		"template":   types.StringType,
	}
}

func connectorConfigFieldAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"field_name": types.StringType,
		"template":   types.StringType,
	}
}
