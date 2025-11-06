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
	"net/http"
	"strings"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	presets "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/presets_service"
	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_                              resource.ResourceWithConfigure   = &PresetResource{}
	_                              resource.ResourceWithImportState = &PresetResource{}
	presetConnectorTypeSchemaToApi                                  = map[string]presets.ConnectorType{
		"unspecified":   presets.CONNECTORTYPE_CONNECTOR_TYPE_UNSPECIFIED,
		"slack":         presets.CONNECTORTYPE_SLACK,
		"generic_https": presets.CONNECTORTYPE_GENERIC_HTTPS,
		"pagerduty":     presets.CONNECTORTYPE_PAGERDUTY,
		"service_now":   presets.CONNECTORTYPE_SERVICE_NOW,
	}
	presetConnectorTypeApiToSchema = utils.ReverseMap(presetConnectorTypeSchemaToApi)
	validConnectorTypes            = utils.GetKeys(presetConnectorTypeSchemaToApi)
	presetEntityTypeSchemaToApi    = map[string]presets.NotificationCenterEntityType{
		"unspecified":        presets.NOTIFICATIONCENTERENTITYTYPE_ENTITY_TYPE_UNSPECIFIED,
		"alerts":             presets.NOTIFICATIONCENTERENTITYTYPE_ALERTS,
		"cases":              presets.NOTIFICATIONCENTERENTITYTYPE_CASES,
		"test_notifications": presets.NOTIFICATIONCENTERENTITYTYPE_TEST_NOTIFICATIONS,
	}
	presetsNotificationCenterEntityTypeApiToSchema       = utils.ReverseMap(presetEntityTypeSchemaToApi)
	presetsValidNotificationCenterEntityTypesSchemaToApi = utils.GetKeys(presetEntityTypeSchemaToApi)
)

func NewPresetResource() resource.Resource {
	return &PresetResource{}
}

type PresetResource struct {
	client *presets.PresetsServiceAPIService
}

type PresetResourceModel struct {
	ID              types.String `tfsdk:"id"`
	EntityType      types.String `tfsdk:"entity_type"`
	ConnectorType   types.String `tfsdk:"connector_type"`
	ConfigOverrides types.List   `tfsdk:"config_overrides"` // PresetConfigOverrideModel
	Name            types.String `tfsdk:"name"`
	ParentId        types.String `tfsdk:"parent_id"`
	Description     types.String `tfsdk:"description"`
}

type PresetConfigOverrideModel struct {
	ConditionType types.Object `tfsdk:"condition_type"` //PresetConditionTypeModel
	PayloadType   types.String `tfsdk:"payload_type"`
	MessageConfig types.Object `tfsdk:"message_config"` // MessageConfigModel
}

type MessageConfigModel struct {
	Fields types.Set `tfsdk:"fields"` // MessageConfigFieldModel
}

type MessageConfigFieldModel struct {
	FieldName types.String `tfsdk:"field_name"`
	Template  types.String `tfsdk:"template"`
}

type PresetConditionTypeModel struct {
	MatchEntityType           types.Object `tfsdk:"match_entity_type"`              //MatchEntityTypeModel
	MatchEntityTypeAndSubType types.Object `tfsdk:"match_entity_type_and_sub_type"` //MatchEntityTypeAndSubTypeModel
}

type MatchEntityTypeModel struct {
}

type MatchEntityTypeAndSubTypeModel struct {
	EntitySubType types.String `tfsdk:"entity_sub_type"`
}

func (r *PresetResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_preset"
}

func (r *PresetResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	_, _, r.client = clientSet.GetNotifications()
}

func (r *PresetResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version: 0,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
				MarkdownDescription: "The ID of the Preset. Can be set to a custom value, or left empty to auto-generate. Requires recreation in case of change.",
			},
			"name": schema.StringAttribute{
				Required: true,
			},
			"entity_type": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.OneOf(presetsValidNotificationCenterEntityTypesSchemaToApi...),
				},
				MarkdownDescription: fmt.Sprintf("The type of entity for the preset. Valid values are: %s", strings.Join(presetsValidNotificationCenterEntityTypesSchemaToApi, ", ")),
			},
			"description": schema.StringAttribute{
				Optional: true,
			},
			"connector_type": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.OneOf(validConnectorTypes...),
				},
				MarkdownDescription: fmt.Sprintf("The type of connector for the preset. Valid values are: %s", strings.Join(validConnectorTypes, ", ")),
			},
			"config_overrides": schema.ListNestedAttribute{
				Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"condition_type": schema.SingleNestedAttribute{
							Required: true,
							Attributes: map[string]schema.Attribute{
								"match_entity_type": schema.SingleNestedAttribute{
									Optional: true,
									Validators: []validator.Object{
										objectvalidator.ExactlyOneOf(
											path.MatchRelative().AtParent().AtName("match_entity_type_and_sub_type"),
										),
									},
									Attributes: map[string]schema.Attribute{},
								},
								"match_entity_type_and_sub_type": schema.SingleNestedAttribute{
									Optional: true,
									Attributes: map[string]schema.Attribute{
										"entity_sub_type": schema.StringAttribute{
											Required: true,
										},
									},
								},
							},
							MarkdownDescription: "Condition type for the preset. Must be either match_entity_type or match_entity_type_and_sub_type.",
						},
						"payload_type": schema.StringAttribute{
							Optional: true,
							Computed: true,
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
							},
						},
						"message_config": schema.SingleNestedAttribute{
							Required: true,
							Attributes: map[string]schema.Attribute{
								"fields": schema.SetNestedAttribute{
									Required: true,
									NestedObject: schema.NestedAttributeObject{
										Attributes: map[string]schema.Attribute{
											"field_name": schema.StringAttribute{
												Required: true,
											},
											"template": schema.StringAttribute{
												Required: true,
											},
										},
									},
								},
							},
						},
					},
				},
			},
			"parent_id": schema.StringAttribute{
				Required: true,
			},
		},
		MarkdownDescription: "Coralogix Preset. **NOTE:** This resource is in Beta stage.",
	}
}

func (r *PresetResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *PresetResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan *PresetResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	preset, diags := extractPreset(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	rq := presets.CreateCustomPresetRequest{
		Preset: preset,
	}

	log.Printf("[INFO] Creating new resource: %s", utils.FormatJSON(rq))
	result, httpResponse, err := r.client.
		PresetsServiceCreateCustomPreset(ctx).
		CreateCustomPresetRequest(rq).
		Execute()

	if err != nil {
		resp.Diagnostics.AddError("Error creating resource",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Create", rq),
		)
		return
	}
	log.Printf("[INFO] Created new resource: %s", utils.FormatJSON(result))
	plan, diags = flattenPreset(ctx, result.Preset)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func (r *PresetResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var plan *PresetResourceModel
	diags := req.State.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := plan.ID.ValueString()

	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	rq := r.client.
		PresetsServiceGetPreset(ctx, id)

	log.Printf("[INFO] Reading resource: %s", utils.FormatJSON(rq))
	result, httpResponse, err := rq.
		Execute()

	if err != nil {
		if httpResponse.StatusCode == http.StatusNotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("Resource %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError("Error reading resource", utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Read", nil))
		}
		return
	}
	log.Printf("[INFO] Replaced new resource: %s", utils.FormatJSON(result))
	plan, diags = flattenPreset(ctx, result.Preset)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func (r PresetResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan *PresetResourceModel
	diags := req.State.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := plan.ID.ValueString()

	preset, diags := extractPreset(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	rq := presets.ReplaceCustomPresetRequest{
		Preset: preset,
	}

	log.Printf("[INFO] Replacing resource: %s", utils.FormatJSON(rq))
	result, httpResponse, err := r.client.
		PresetsServiceReplaceCustomPreset(ctx).
		ReplaceCustomPresetRequest(rq).
		Execute()

	if err != nil {
		if httpResponse.StatusCode == http.StatusNotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("Resource %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError("Error replacing resource", utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Replace", nil))
		}
		return
	}
	log.Printf("[INFO] Replaced resource: %s", utils.FormatJSON(result))
	plan, diags = flattenPreset(ctx, result.Preset)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func (r PresetResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state PresetResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	log.Printf("[INFO] Deleting Preset %s", id)

	if _, httpResponse, err := r.client.PresetsServiceDeleteCustomPreset(ctx, id).Execute(); err != nil {
		resp.Diagnostics.AddError("Error deleting resource", utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Delete", nil))
		return
	}
	log.Printf("[INFO] Preset %s deleted", id)
}

func extractPreset(ctx context.Context, plan *PresetResourceModel) (*presets.Preset, diag.Diagnostics) {
	configOverrides, diags := extractPresetConfigOverrides(ctx, plan.ConfigOverrides)
	presetType := presets.PRESETTYPE_CUSTOM
	connectorType := presetConnectorTypeSchemaToApi[plan.ConnectorType.ValueString()]
	entityType := presetEntityTypeSchemaToApi[plan.EntityType.ValueString()]
	if diags.HasError() {
		return nil, diags
	}
	return &presets.Preset{
		Id:              utils.TypeStringToStringPointer(plan.ID),
		EntityType:      entityType,
		ConnectorType:   &connectorType,
		Name:            plan.Name.ValueString(),
		ConfigOverrides: configOverrides,
		Description:     plan.Description.ValueStringPointer(),
		PresetType:      &presetType,
		ParentId:        utils.TypeStringToStringPointer(plan.ParentId),
	}, nil
}
func extractPresetConfigOverrides(ctx context.Context, overrides types.List) ([]presets.ConfigOverrides, diag.Diagnostics) {
	var diags diag.Diagnostics
	var overrideObjects []types.Object
	overrides.ElementsAs(ctx, &overrideObjects, true)
	extractedOverrides := make([]presets.ConfigOverrides, 0, len(overrideObjects))

	for _, o := range overrideObjects {
		extractedOverride, dg := extractOverride(ctx, o)
		if dg.HasError() {
			diags.Append(dg...)
			continue
		}
		extractedOverrides = append(extractedOverrides, *extractedOverride)
	}

	if diags.HasError() {
		return nil, diags
	}

	return extractedOverrides, nil
}

func extractOverride(ctx context.Context, overrideObject types.Object) (*presets.ConfigOverrides, diag.Diagnostics) {
	var override PresetConfigOverrideModel
	if diags := overrideObject.As(ctx, &override, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}
	conditionType, diags := extractConditionType(ctx, override.ConditionType)
	if diags.HasError() {
		return nil, diags
	}

	messageConfig, diags := extractMessageConfig(ctx, override.MessageConfig)
	if diags.HasError() {
		return nil, diags
	}

	return &presets.ConfigOverrides{
		ConditionType: conditionType,
		PayloadType:   utils.TypeStringToStringPointer(override.PayloadType),
		MessageConfig: messageConfig,
	}, nil

}

func extractConditionType(ctx context.Context, conditionType types.Object) (*presets.NotificationCenterConditionType, diag.Diagnostics) {
	var condition PresetConditionTypeModel
	if diags := conditionType.As(ctx, &condition, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	if matchEntityType := condition.MatchEntityType; !(matchEntityType.IsNull() || matchEntityType.IsUnknown()) {
		var matchEntityTypeModel MatchEntityTypeModel
		if diags := matchEntityType.As(ctx, &matchEntityTypeModel, basetypes.ObjectAsOptions{}); diags.HasError() {
			return nil, diags
		}
		return &presets.NotificationCenterConditionType{
			NotificationCenterConditionTypeMatchEntityType: &presets.NotificationCenterConditionTypeMatchEntityType{},
		}, nil
	} else if matchEntityTypeAndSubType := condition.MatchEntityTypeAndSubType; !(matchEntityTypeAndSubType.IsNull() || matchEntityTypeAndSubType.IsUnknown()) {
		var matchEntityTypeAndSubTypeModel MatchEntityTypeAndSubTypeModel
		if diags := matchEntityTypeAndSubType.As(ctx, &matchEntityTypeAndSubTypeModel, basetypes.ObjectAsOptions{}); diags.HasError() {
			return nil, diags
		}
		return &presets.NotificationCenterConditionType{
			NotificationCenterConditionTypeMatchEntityTypeAndSubType: &presets.NotificationCenterConditionTypeMatchEntityTypeAndSubType{
				MatchEntityTypeAndSubType: &presets.MatchEntityTypeAndSubTypeCondition{
					EntitySubType: matchEntityTypeAndSubTypeModel.EntitySubType.ValueStringPointer(),
				},
			},
		}, nil
	}

	return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Invalid condition type", "Condition type must be either MatchEntityType or MatchEntityTypeAndSubType")}
}

func extractMessageConfig(ctx context.Context, config types.Object) (*presets.MessageConfig, diag.Diagnostics) {
	var messageConfig MessageConfigModel
	if diags := config.As(ctx, &messageConfig, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	messageFields, diags := extractMessageConfigFields(ctx, messageConfig.Fields)
	if diags.HasError() {
		return nil, diags
	}

	return &presets.MessageConfig{
		Fields: messageFields,
	}, nil
}

func extractMessageConfigFields(ctx context.Context, configFields types.Set) ([]presets.NotificationCenterMessageConfigField, diag.Diagnostics) {
	var diags diag.Diagnostics
	if configFields.IsNull() || configFields.IsUnknown() {
		return nil, diags
	}

	var configFieldsObjects []types.Object
	configFields.ElementsAs(ctx, &configFieldsObjects, true)
	extractedConfigFields := make([]presets.NotificationCenterMessageConfigField, 0, len(configFieldsObjects))
	for _, field := range configFieldsObjects {
		var fieldModel MessageConfigFieldModel
		if dg := field.As(ctx, &fieldModel, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		extractedConfigField := presets.NotificationCenterMessageConfigField{
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

func flattenPreset(ctx context.Context, preset *presets.Preset) (*PresetResourceModel, diag.Diagnostics) {
	configOverrides, diags := flattenPresetConfigOverrides(ctx, preset.ConfigOverrides)
	if diags.HasError() {
		return nil, diags
	}
	connectorType, exists := presetConnectorTypeApiToSchema[*preset.ConnectorType]
	if !exists {
		connectorType = string(*preset.ConnectorType)
	}
	return &PresetResourceModel{
		ID:              utils.StringPointerToTypeString(preset.Id),
		EntityType:      types.StringValue(presetsNotificationCenterEntityTypeApiToSchema[preset.EntityType]),
		ConnectorType:   types.StringValue(connectorType),
		ConfigOverrides: configOverrides,
		Name:            types.StringValue(preset.Name),
		ParentId:        utils.StringPointerToTypeString(preset.ParentId),
		Description:     types.StringPointerValue(preset.Description),
	}, nil

}

func flattenPresetConfigOverrides(ctx context.Context, overrides []presets.ConfigOverrides) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	overridesList := make([]types.Object, 0, len(overrides))

	for _, override := range overrides {
		overrideObject, dg := flattenPresetOverride(ctx, &override)
		if dg.HasError() {
			diags.Append(dg...)
			continue
		}
		overridesList = append(overridesList, overrideObject)
	}

	if diags.HasError() {
		return types.ListNull(types.ObjectType{AttrTypes: presetConfigOverrideAttr()}), diags
	}

	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: presetConfigOverrideAttr()}, overridesList)
}

func flattenPresetOverride(ctx context.Context, override *presets.ConfigOverrides) (types.Object, diag.Diagnostics) {
	conditionType, diags := flattenConditionType(ctx, override.ConditionType)
	if diags.HasError() {
		return types.ObjectNull(presetConfigOverrideAttr()), diags
	}

	messageConfig, diags := flattenMessageConfig(ctx, override.MessageConfig)
	if diags.HasError() {
		return types.ObjectNull(presetConfigOverrideAttr()), diags
	}

	overrideObject := PresetConfigOverrideModel{
		ConditionType: conditionType,
		MessageConfig: messageConfig,
		PayloadType:   utils.StringPointerToTypeString(override.PayloadType),
	}

	return types.ObjectValueFrom(ctx, presetConfigOverrideAttr(), overrideObject)
}

func flattenMessageConfig(ctx context.Context, config *presets.MessageConfig) (types.Object, diag.Diagnostics) {
	if config == nil {
		return types.ObjectNull(messageConfigAttr()), nil
	}

	fields, diags := flattenMessageConfigFields(ctx, config.Fields)
	if diags.HasError() {
		return types.ObjectNull(messageConfigAttr()), diags
	}

	messageConfig := MessageConfigModel{
		Fields: fields,
	}

	return types.ObjectValueFrom(ctx, messageConfigAttr(), messageConfig)
}

func flattenMessageConfigFields(ctx context.Context, configFields []presets.NotificationCenterMessageConfigField) (types.Set, diag.Diagnostics) {
	var diags diag.Diagnostics
	if configFields == nil {
		return types.SetNull(types.ObjectType{AttrTypes: configOverridesAttr()}), diags
	}

	configFieldsList := make([]MessageConfigFieldModel, 0, len(configFields))
	for _, field := range configFields {
		fieldModel := MessageConfigFieldModel{
			FieldName: types.StringValue(field.GetFieldName()),
			Template:  types.StringValue(field.GetTemplate()),
		}
		configFieldsList = append(configFieldsList, fieldModel)
	}

	return types.SetValueFrom(ctx, types.ObjectType{AttrTypes: configOverridesAttr()}, configFieldsList)
}

func flattenConditionType(ctx context.Context, condition *presets.NotificationCenterConditionType) (types.Object, diag.Diagnostics) {
	if condition == nil {
		return types.ObjectNull(conditionTypeAttr()), nil
	}

	var presetCondition PresetConditionTypeModel

	presetCondition.MatchEntityTypeAndSubType = types.ObjectNull(matchEntityTypeAndSubTypeAttr())
	presetCondition.MatchEntityType = types.ObjectNull(matchEntityTypeTypeAttr())

	if matchEntityType := condition.NotificationCenterConditionTypeMatchEntityType; matchEntityType != nil {
		matchEntityType, diags := types.ObjectValueFrom(ctx, matchEntityTypeTypeAttr(), MatchEntityTypeModel{})
		if diags.HasError() {
			return types.ObjectNull(conditionTypeAttr()), diags
		}
		presetCondition.MatchEntityType = matchEntityType

	} else if matchEntityTypeAndSubType := condition.NotificationCenterConditionTypeMatchEntityTypeAndSubType; matchEntityTypeAndSubType != nil {
		matchEntityTypeAndSubType, diags := types.ObjectValueFrom(ctx, matchEntityTypeAndSubTypeAttr(), MatchEntityTypeAndSubTypeModel{
			EntitySubType: types.StringPointerValue(matchEntityTypeAndSubType.GetMatchEntityTypeAndSubType().EntitySubType),
		})
		if diags.HasError() {
			return types.ObjectNull(conditionTypeAttr()), diags
		}
		presetCondition.MatchEntityTypeAndSubType = matchEntityTypeAndSubType
	} else {
		return types.ObjectNull(conditionTypeAttr()), diag.Diagnostics{diag.NewErrorDiagnostic("Invalid condition type", "Condition type must be either MatchEntityType or MatchEntityTypeAndSubType")}
	}

	return types.ObjectValueFrom(ctx, conditionTypeAttr(), presetCondition)

}
func presetConfigOverrideAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"condition_type": types.ObjectType{AttrTypes: conditionTypeAttr()},
		"payload_type":   types.StringType,
		"message_config": types.ObjectType{AttrTypes: messageConfigAttr()},
	}
}

func messageConfigAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"fields": types.SetType{ElemType: types.ObjectType{AttrTypes: messageConfigFieldAttr()}},
	}
}

func conditionTypeAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"match_entity_type":              types.ObjectType{AttrTypes: matchEntityTypeTypeAttr()},
		"match_entity_type_and_sub_type": types.ObjectType{AttrTypes: matchEntityTypeAndSubTypeAttr()},
	}
}

func matchEntityTypeTypeAttr() map[string]attr.Type {
	return map[string]attr.Type{}
}
func matchEntityTypeAndSubTypeAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"entity_sub_type": types.StringType,
	}
}
