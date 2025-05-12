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
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"terraform-provider-coralogix/coralogix/clientset"
	"terraform-provider-coralogix/coralogix/utils"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/grpc/codes"
)

var (
	_                                resource.ResourceWithConfigure   = &PresetResource{}
	_                                resource.ResourceWithImportState = &PresetResource{}
	presetConnectorTypeSchemaToProto                                  = map[string]cxsdk.ConnectorType{
		"unspecified":   cxsdk.ConnectorTypeUnSpecified,
		"slack":         cxsdk.ConnectorTypeSlack,
		"generic_https": cxsdk.ConnectorTypeGenericHTTPS,
		"pagerduty":     cxsdk.ConnectorTypePagerDuty,
	}
	presetConnectorTypeProtoToSchema = utils.ReverseMap(presetConnectorTypeSchemaToProto)
	validConnectorTypes              = utils.GetKeys(presetConnectorTypeSchemaToProto)
)

func NewPresetResource() resource.Resource {
	return &PresetResource{}
}

type PresetResource struct {
	client *cxsdk.NotificationsClient
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

	r.client = clientSet.GetNotifications()
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
					stringvalidator.OneOf(validNotificationsEntityTypes...),
				},
				MarkdownDescription: fmt.Sprintf("The type of entity for the preset. Valid values are: %s", strings.Join(validNotificationsEntityTypes, ", ")),
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
		MarkdownDescription: "Coralogix Preset. **NOTE:** This resource is in alpha stage.",
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

	presetStr := protojson.Format(preset)
	log.Printf("[INFO] Creating new Preset: %s", presetStr)
	createResp, err := r.client.CreateCustomPreset(ctx, &cxsdk.CreateCustomPresetRequest{Preset: preset})
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err)
		resp.Diagnostics.AddError("Error creating Preset",
			//TODO: add the proper url
			utils.FormatRpcErrors(err, "", presetStr),
		)
		return
	}

	Preset := createResp.GetPreset()
	log.Printf("[INFO] Submitted new Preset: %s", protojson.Format(Preset))

	plan, diags = flattenPreset(ctx, Preset)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func (r *PresetResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state *PresetResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	//Get refreshed Preset value from Coralogix
	log.Printf("[INFO] Reading Preset: %s", id)
	getPresetReq := &cxsdk.GetPresetRequest{Id: id}
	getPresetResp, err := r.client.GetPreset(ctx, getPresetReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if cxsdk.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("Preset %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError(
				"Error reading Preset",
				//TODO: add the proper url
				utils.FormatRpcErrors(err, "", protojson.Format(getPresetReq)),
			)
		}
		return
	}
	Preset := getPresetResp.GetPreset()
	log.Printf("[INFO] Received Preset: %s", protojson.Format(Preset))

	state, diags = flattenPreset(ctx, Preset)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	//
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r PresetResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
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
	log.Printf("[INFO] Updating Preset: %s", protojson.Format(preset))
	PresetUpdateResp, err := r.client.ReplaceCustomPreset(ctx, &cxsdk.ReplaceCustomPresetRequest{Preset: preset})
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error updating Preset",
			//TODO: add the proper url
			utils.FormatRpcErrors(err, "", protojson.Format(preset)),
		)
		return
	}
	log.Printf("[INFO] Submitted updated Preset: %s", protojson.Format(PresetUpdateResp))

	plan, diags = flattenPreset(ctx, preset)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// Set state to fully populated data
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
	deleteReq := &cxsdk.DeleteCustomPresetRequest{Id: id}
	if _, err := r.client.DeleteCustomPreset(ctx, deleteReq); err != nil {
		resp.Diagnostics.AddError(
			fmt.Sprintf("Error Deleting Preset %s", id),
			//TODO: add the proper url
			utils.FormatRpcErrors(err, "", protojson.Format(deleteReq)),
		)
		return
	}
	log.Printf("[INFO] Preset %s deleted", id)
}

func extractPreset(ctx context.Context, plan *PresetResourceModel) (*cxsdk.Preset, diag.Diagnostics) {
	configOverrides, diags := extractPresetConfigOverrides(ctx, plan.ConfigOverrides)
	presetType := cxsdk.PresetTypeCustom
	if diags.HasError() {
		return nil, diags
	}
	return &cxsdk.Preset{
		Id:              utils.TypeStringToStringPointer(plan.ID),
		EntityType:      notificationCenterEntityTypeSchemaToProto[plan.EntityType.ValueString()],
		ConnectorType:   presetConnectorTypeSchemaToProto[plan.ConnectorType.ValueString()],
		Name:            plan.Name.ValueString(),
		ConfigOverrides: configOverrides,
		Description:     plan.Description.ValueString(),
		PresetType:      &presetType,
		ParentId:        utils.TypeStringToStringPointer(plan.ParentId),
	}, nil
}
func extractPresetConfigOverrides(ctx context.Context, overrides types.List) ([]*cxsdk.ConfigOverrides, diag.Diagnostics) {
	var diags diag.Diagnostics
	var overrideObjects []types.Object
	overrides.ElementsAs(ctx, &overrideObjects, true)
	extractedOverrides := make([]*cxsdk.ConfigOverrides, 0, len(overrideObjects))

	for _, o := range overrideObjects {
		extractedOverride, dg := extractOverride(ctx, o)
		if dg.HasError() {
			diags.Append(dg...)
			continue
		}
		extractedOverrides = append(extractedOverrides, extractedOverride)
	}

	if diags.HasError() {
		return nil, diags
	}

	return extractedOverrides, nil
}

func extractOverride(ctx context.Context, overrideObject types.Object) (*cxsdk.ConfigOverrides, diag.Diagnostics) {
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

	return &cxsdk.ConfigOverrides{
		ConditionType: conditionType,
		PayloadType:   utils.TypeStringToStringPointer(override.PayloadType),
		MessageConfig: messageConfig,
	}, nil

}

func extractConditionType(ctx context.Context, conditionType types.Object) (*cxsdk.ConditionType, diag.Diagnostics) {
	var condition PresetConditionTypeModel
	if diags := conditionType.As(ctx, &condition, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	if matchEntityType := condition.MatchEntityType; !(matchEntityType.IsNull() || matchEntityType.IsUnknown()) {
		var matchEntityTypeModel MatchEntityTypeModel
		if diags := matchEntityType.As(ctx, &matchEntityTypeModel, basetypes.ObjectAsOptions{}); diags.HasError() {
			return nil, diags
		}
		return &cxsdk.ConditionType{
			Condition: &cxsdk.ConditionTypeMatchEntityType{
				MatchEntityType: &cxsdk.MatchEntityTypeCondition{},
			},
		}, nil
	} else if matchEntityTypeAndSubType := condition.MatchEntityTypeAndSubType; !(matchEntityTypeAndSubType.IsNull() || matchEntityTypeAndSubType.IsUnknown()) {
		var matchEntityTypeAndSubTypeModel MatchEntityTypeAndSubTypeModel
		if diags := matchEntityTypeAndSubType.As(ctx, &matchEntityTypeAndSubTypeModel, basetypes.ObjectAsOptions{}); diags.HasError() {
			return nil, diags
		}
		return &cxsdk.ConditionType{
			Condition: &cxsdk.ConditionTypeMatchEntityTypeAndSubType{
				MatchEntityTypeAndSubType: &cxsdk.MatchEntityTypeAndSubTypeCondition{
					EntitySubType: matchEntityTypeAndSubTypeModel.EntitySubType.ValueString(),
				},
			},
		}, nil
	}

	return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Invalid condition type", "Condition type must be either MatchEntityType or MatchEntityTypeAndSubType")}
}

func extractMessageConfig(ctx context.Context, config types.Object) (*cxsdk.MessageConfig, diag.Diagnostics) {
	var messageConfig MessageConfigModel
	if diags := config.As(ctx, &messageConfig, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	messageFields, diags := extractMessageConfigFields(ctx, messageConfig.Fields)
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.MessageConfig{
		Fields: messageFields,
	}, nil
}

func extractMessageConfigFields(ctx context.Context, configFields types.Set) ([]*cxsdk.MessageConfigField, diag.Diagnostics) {
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

func flattenPreset(ctx context.Context, preset *cxsdk.Preset) (*PresetResourceModel, diag.Diagnostics) {
	configOverrides, diags := flattenPresetConfigOverrides(ctx, preset.ConfigOverrides)
	if diags.HasError() {
		return nil, diags
	}

	return &PresetResourceModel{
		ID:              utils.StringPointerToTypeString(preset.Id),
		EntityType:      types.StringValue(notificationCenterEntityTypeProtoToSchema[preset.EntityType]),
		ConnectorType:   types.StringValue(presetConnectorTypeProtoToSchema[preset.ConnectorType]),
		ConfigOverrides: configOverrides,
		Name:            types.StringValue(preset.Name),
		ParentId:        utils.StringPointerToTypeString(preset.ParentId),
		Description:     types.StringValue(preset.Description),
	}, nil

}

func flattenPresetConfigOverrides(ctx context.Context, overrides []*cxsdk.ConfigOverrides) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	overridesList := make([]types.Object, 0, len(overrides))

	for _, override := range overrides {
		overrideObject, dg := flattenPresetOverride(ctx, override)
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

func flattenPresetOverride(ctx context.Context, override *cxsdk.ConfigOverrides) (types.Object, diag.Diagnostics) {
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

func flattenMessageConfig(ctx context.Context, config *cxsdk.MessageConfig) (types.Object, diag.Diagnostics) {
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

func flattenMessageConfigFields(ctx context.Context, configFields []*cxsdk.MessageConfigField) (types.Set, diag.Diagnostics) {
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

func flattenConditionType(ctx context.Context, condition *cxsdk.ConditionType) (types.Object, diag.Diagnostics) {
	if condition == nil {
		return types.ObjectNull(conditionTypeAttr()), nil
	}

	var presetCondition PresetConditionTypeModel
	if matchEntityType := condition.GetMatchEntityType(); matchEntityType != nil {
		matchEntityType, diags := types.ObjectValueFrom(ctx, matchEntityTypeTypeAttr(), MatchEntityTypeModel{})
		if diags.HasError() {
			return types.ObjectNull(conditionTypeAttr()), diags
		}
		presetCondition.MatchEntityType = matchEntityType
		presetCondition.MatchEntityTypeAndSubType = types.ObjectNull(matchEntityTypeAndSubTypeAttr())
	} else if matchEntityTypeAndSubType := condition.GetMatchEntityTypeAndSubType(); matchEntityTypeAndSubType != nil {
		matchEntityTypeAndSubType, diags := types.ObjectValueFrom(ctx, matchEntityTypeAndSubTypeAttr(), MatchEntityTypeAndSubTypeModel{
			EntitySubType: types.StringValue(matchEntityTypeAndSubType.EntitySubType),
		})
		if diags.HasError() {
			return types.ObjectNull(conditionTypeAttr()), diags
		}
		presetCondition.MatchEntityTypeAndSubType = matchEntityTypeAndSubType
		presetCondition.MatchEntityType = types.ObjectNull(matchEntityTypeTypeAttr())
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
