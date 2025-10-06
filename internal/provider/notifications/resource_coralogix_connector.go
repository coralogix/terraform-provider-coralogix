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

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
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
	"google.golang.org/protobuf/encoding/protojson"
)

var (
	_                          resource.ResourceWithImportState = &ConnectorResource{}
	connectorTypeSchemaToProto                                  = map[string]cxsdk.ConnectorType{
		"unspecified":   cxsdk.ConnectorTypeUnSpecified,
		"slack":         cxsdk.ConnectorTypeSlack,
		"generic_https": cxsdk.ConnectorTypeGenericHTTPS,
		"pagerduty":     cxsdk.ConnectorTypePagerDuty,
	}
	connectorTypeProtoToSchema                = utils.ReverseMap(connectorTypeSchemaToProto)
	validConnectorTypesSchemaToProto          = utils.GetKeys(connectorTypeSchemaToProto)
	notificationCenterEntityTypeSchemaToProto = map[string]cxsdk.NotificationsEntityType{
		"unspecified": cxsdk.NotificationsEntityTypeUnspecified,
		"alerts":      cxsdk.NotificationsEntityTypeAlerts,
	}
	notificationCenterEntityTypeProtoToSchema       = utils.ReverseMap(notificationCenterEntityTypeSchemaToProto)
	validNotificationCenterEntityTypesSchemaToProto = utils.GetKeys(notificationCenterEntityTypeSchemaToProto)
)

func NewConnectorResource() resource.Resource {
	return &ConnectorResource{}
}

type ConnectorResource struct {
	client *cxsdk.NotificationsClient
}

type ConnectorResourceModel struct {
	ID              types.String `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	Description     types.String `tfsdk:"description"`
	Type            types.String `tfsdk:"type"`
	ConnectorConfig types.Object `tfsdk:"connector_config"` // ConnectorConfigModel
	ConfigOverrides types.List   `tfsdk:"config_overrides"` // ConfigOverrideModel
}

type ConnectorConfigModel struct {
	ConnectorConfigFields types.Set `tfsdk:"fields"` // ConnectorConfigFieldModel
}

type ConnectorConfigFieldModel struct {
	FieldName types.String `tfsdk:"field_name"`
	Value     types.String `tfsdk:"value"`
}

type TemplatedConnectorConfigFieldModel struct {
	FieldName types.String `tfsdk:"field_name"`
	Template  types.String `tfsdk:"template"`
}

type ConfigOverrideModel struct {
	EntityType types.String `tfsdk:"entity_type"`
	Fields     types.Set    `tfsdk:"fields"` // ConnectorOverrideFieldModel
}

type ConnectorOverrideFieldModel struct {
	FieldName types.String `tfsdk:"field_name"`
	Template  types.String `tfsdk:"template"`
}

func (r *ConnectorResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_connector"
}

func (r *ConnectorResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ConnectorResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
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
				MarkdownDescription: "Connector ID. Can be set by the user or generated by Coralogix. Requires recreation in case of change.",
			},
			"name": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				MarkdownDescription: "Connector name.",
			},
			"description": schema.StringAttribute{
				Optional: true,
			},
			"type": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.OneOf(validConnectorTypesSchemaToProto...),
				},
				MarkdownDescription: fmt.Sprintf("Connector type. Valid values are: %s", validConnectorTypesSchemaToProto),
			},
			"connector_config": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"fields": schema.SetNestedAttribute{
						Required: true,
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"field_name": schema.StringAttribute{
									Required: true,
								},
								"value": schema.StringAttribute{
									Required: true,
								},
							},
						},
					},
				},
			},
			"config_overrides": schema.ListNestedAttribute{
				Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"entity_type": schema.StringAttribute{
							Required: true,
							Validators: []validator.String{
								stringvalidator.OneOf(validNotificationCenterEntityTypesSchemaToProto...),
							},
							Description: fmt.Sprintf("Entity type for the connector. Valid values are: %s", validNotificationCenterEntityTypesSchemaToProto),
						},
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
		MarkdownDescription: "Coralogix Connector. **Note:** This resource is in alpha stage.",
	}
}

func (r *ConnectorResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *ConnectorResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan *ConnectorResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	connector, diags := extractConnector(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	createConnectorRequest := &cxsdk.CreateConnectorRequest{
		Connector: connector,
	}

	connectorStr := protojson.Format(createConnectorRequest)
	log.Printf("[INFO] Creating new Connector: %s", connectorStr)
	createResp, err := r.client.CreateConnector(ctx, createConnectorRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err)
		resp.Diagnostics.AddError("Error creating Connector",
			//TODO: add the proper url
			utils.FormatRpcErrors(err, "", connectorStr),
		)
		return
	}
	connector = createResp.GetConnector()
	log.Printf("[INFO] Submitted new Connector: %s", protojson.Format(connector))

	plan, diags = flattenConnector(ctx, connector)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func (r *ConnectorResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state *ConnectorResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	//Get refreshed Connector value from Coralogix
	id := state.ID.ValueString()
	log.Printf("[INFO] Reading Connector: %s", id)
	getConnectorReq := &cxsdk.GetConnectorRequest{Id: id}
	getConnectorResp, err := r.client.GetConnector(ctx, getConnectorReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if cxsdk.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("Connector %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError(
				"Error reading Connector",
				utils.FormatRpcErrors(err, cxsdk.ConnectorsGetRPC, protojson.Format(getConnectorReq)))
		}
		return
	}
	log.Printf("[INFO] Received Connector: %s", protojson.Format(getConnectorResp))

	state, diags = flattenConnector(ctx, getConnectorResp.GetConnector())
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	//
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r ConnectorResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var plan *ConnectorResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	connector, diags := extractConnector(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	connectorUpdateReq := &cxsdk.ReplaceConnectorRequest{
		Connector: connector,
	}
	log.Printf("[INFO] Updating Connector: %s", protojson.Format(connectorUpdateReq))
	connectorUpdateResp, err := r.client.ReplaceConnector(ctx, connectorUpdateReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if cxsdk.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("Connector %q is in state, but no longer exists in Coralogix backend", *connectorUpdateReq.Connector.Id),
				fmt.Sprintf("%s will be recreated when you apply", *connectorUpdateReq.Connector.Id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError(
				"Error updating Connector",
				//TODO: add the proper url
				utils.FormatRpcErrors(err, "", protojson.Format(connectorUpdateResp)),
			)
		}
		return
	}
	log.Printf("[INFO] Submitted updated Connector: %s", protojson.Format(connectorUpdateReq))

	plan, diags = flattenConnector(ctx, connectorUpdateResp.GetConnector())
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func (r ConnectorResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ConnectorResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	log.Printf("[INFO] Deleting Connector %s", id)
	deleteReq := &cxsdk.DeleteConnectorRequest{Id: id}
	if _, err := r.client.DeleteConnector(ctx, deleteReq); err != nil {
		resp.Diagnostics.AddError(
			fmt.Sprintf("Error Deleting Connector %s", id),
			//TODO: add the proper url
			utils.FormatRpcErrors(err, "", protojson.Format(deleteReq)),
		)
		return
	}
	log.Printf("[INFO] Connector %s deleted", id)
}

func extractConnector(ctx context.Context, plan *ConnectorResourceModel) (*cxsdk.Connector, diag.Diagnostics) {
	connectorConfigs, diags := extractConnectorConfig(ctx, plan.ConnectorConfig)
	if diags.HasError() {
		return nil, diags
	}

	configOverrides, diags := extractConfigOverrides(ctx, plan.ConfigOverrides)
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.Connector{
		Id:              utils.TypeStringToStringPointer(plan.ID),
		Name:            plan.Name.ValueString(),
		Description:     plan.Description.ValueString(),
		Type:            connectorTypeSchemaToProto[plan.Type.ValueString()],
		ConnectorConfig: connectorConfigs,
		ConfigOverrides: configOverrides,
	}, nil
}

func extractConnectorConfig(ctx context.Context, connectorConfig types.Object) (*cxsdk.ConnectorConfig, diag.Diagnostics) {
	var connectorConfigModel ConnectorConfigModel
	diags := connectorConfig.As(ctx, &connectorConfigModel, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	extractedConnectorConfigFields, diags := extractConnectorConfigFields(ctx, connectorConfigModel.ConnectorConfigFields)
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.ConnectorConfig{
		Fields: extractedConnectorConfigFields,
	}, nil
}

func extractConnectorConfigFields(ctx context.Context, connectorConfigFields types.Set) ([]*cxsdk.ConnectorConfigField, diag.Diagnostics) {
	var diags diag.Diagnostics
	var connectorConfigFieldsObjects []types.Object
	connectorConfigFields.ElementsAs(ctx, &connectorConfigFieldsObjects, true)
	extractedConnectorConfigFields := make([]*cxsdk.ConnectorConfigField, 0, len(connectorConfigFieldsObjects))

	for _, ccf := range connectorConfigFieldsObjects {
		var connectorConfigFieldModel ConnectorConfigFieldModel
		if dg := ccf.As(ctx, &connectorConfigFieldModel, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		extractedConnectorConfigField := extractConnectorConfigField(connectorConfigFieldModel)
		extractedConnectorConfigFields = append(extractedConnectorConfigFields, extractedConnectorConfigField)
	}

	if diags.HasError() {
		return nil, diags
	}

	return extractedConnectorConfigFields, diags
}

func extractConnectorConfigField(connectorConfigField ConnectorConfigFieldModel) *cxsdk.ConnectorConfigField {
	return &cxsdk.ConnectorConfigField{
		FieldName: connectorConfigField.FieldName.ValueString(),
		Value:     connectorConfigField.Value.ValueString(),
	}
}

func extractConfigOverrides(ctx context.Context, overrides types.List) ([]*cxsdk.EntityTypeConfigOverrides, diag.Diagnostics) {
	var diags diag.Diagnostics
	var connectorOverridesObjects []types.Object
	overrides.ElementsAs(ctx, &connectorOverridesObjects, true)
	extractedConnectorOverrides := make([]*cxsdk.EntityTypeConfigOverrides, 0, len(connectorOverridesObjects))

	for _, co := range connectorOverridesObjects {
		var connectorOverrideModel ConfigOverrideModel
		if dg := co.As(ctx, &connectorOverrideModel, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		extractedConnectorOverride, dg := extractConnectorOverride(ctx, connectorOverrideModel)
		if diags.HasError() {
			diags.Append(dg...)
			continue
		}
		extractedConnectorOverrides = append(extractedConnectorOverrides, extractedConnectorOverride)
	}

	if diags.HasError() {
		return nil, diags
	}

	return extractedConnectorOverrides, diags
}

func extractConnectorOverride(ctx context.Context, connectorOverrideModel ConfigOverrideModel) (*cxsdk.EntityTypeConfigOverrides, diag.Diagnostics) {
	templatedConnectorConfigFields, diags := extractTemplatedConnectorConfigFields(ctx, connectorOverrideModel.Fields)
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.EntityTypeConfigOverrides{
		EntityType: notificationCenterEntityTypeSchemaToProto[connectorOverrideModel.EntityType.ValueString()],
		Fields:     templatedConnectorConfigFields,
	}, nil
}

func extractTemplatedConnectorConfigFields(ctx context.Context, connectorConfigFields types.Set) ([]*cxsdk.TemplatedConnectorConfigField, diag.Diagnostics) {
	var diags diag.Diagnostics
	var connectorConfigFieldsObjects []types.Object
	connectorConfigFields.ElementsAs(ctx, &connectorConfigFieldsObjects, true)
	extractedConnectorConfigFields := make([]*cxsdk.TemplatedConnectorConfigField, 0, len(connectorConfigFieldsObjects))

	for _, ccf := range connectorConfigFieldsObjects {
		var connectorConfigFieldModel TemplatedConnectorConfigFieldModel
		if dg := ccf.As(ctx, &connectorConfigFieldModel, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		extractedConnectorConfigField := extractTemplatedConnectorConfigField(connectorConfigFieldModel)
		extractedConnectorConfigFields = append(extractedConnectorConfigFields, extractedConnectorConfigField)
	}

	if diags.HasError() {
		return nil, diags
	}

	return extractedConnectorConfigFields, diags
}

func extractTemplatedConnectorConfigField(model TemplatedConnectorConfigFieldModel) *cxsdk.TemplatedConnectorConfigField {
	return &cxsdk.TemplatedConnectorConfigField{
		FieldName: model.FieldName.ValueString(),
		Template:  model.Template.ValueString(),
	}
}

func flattenConnector(ctx context.Context, connector *cxsdk.Connector) (*ConnectorResourceModel, diag.Diagnostics) {
	config, diags := flattenConnectorConfig(ctx, connector.GetConnectorConfig())
	if diags.HasError() {
		return nil, diags
	}

	overrides, diags := flattenConnectorOverrides(ctx, connector.GetConfigOverrides())
	if diags.HasError() {
		return nil, diags
	}

	return &ConnectorResourceModel{
		ID:              types.StringValue(connector.GetId()),
		Name:            types.StringValue(connector.GetName()),
		Description:     types.StringValue(connector.GetDescription()),
		Type:            types.StringValue(connectorTypeProtoToSchema[connector.GetType()]),
		ConnectorConfig: config,
		ConfigOverrides: overrides,
	}, nil
}

func flattenConnectorOverrides(ctx context.Context, overrides []*cxsdk.EntityTypeConfigOverrides) (types.List, diag.Diagnostics) {
	if overrides == nil {
		return types.ListNull(types.ObjectType{AttrTypes: connectorOverrideAttr()}), nil
	}
	var diags diag.Diagnostics
	flattenedOverrides := make([]types.Object, 0, len(overrides))
	for _, override := range overrides {
		flattenedOverride, dg := flattenConnectorOverride(ctx, override)
		if dg.HasError() {
			diags.Append(dg...)
			continue
		}
		flattenedOverrides = append(flattenedOverrides, flattenedOverride)
	}

	if diags.HasError() {
		return types.ListNull(types.ObjectType{AttrTypes: connectorOverrideAttr()}), diags
	}

	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: connectorOverrideAttr()}, flattenedOverrides)
}

func flattenConnectorOverride(ctx context.Context, override *cxsdk.EntityTypeConfigOverrides) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	overrideFields, dg := flattenTemplatedConnectorConfigFields(ctx, override.GetFields())
	if dg.HasError() {
		diags.Append(dg...)
		return types.ObjectNull(connectorOverrideAttr()), diags
	}

	connectorOverrideModel := ConfigOverrideModel{
		EntityType: types.StringValue(notificationCenterEntityTypeProtoToSchema[override.GetEntityType()]),
		Fields:     overrideFields,
	}

	return types.ObjectValueFrom(ctx, connectorOverrideAttr(), connectorOverrideModel)
}

func flattenTemplatedConnectorConfigFields(ctx context.Context, fields []*cxsdk.TemplatedConnectorConfigField) (types.Set, diag.Diagnostics) {
	var diags diag.Diagnostics
	flattenedFields := make([]types.Object, 0, len(fields))
	for _, field := range fields {
		flattenedField, dg := flattenTemplatedConnectorConfigField(ctx, field)
		if dg.HasError() {
			diags.Append(dg...)
			continue
		}
		flattenedFields = append(flattenedFields, flattenedField)
	}

	if diags.HasError() {
		return types.SetNull(types.ObjectType{AttrTypes: templatedConnectorConfigFieldAttr()}), diags
	}

	return types.SetValueFrom(ctx, types.ObjectType{AttrTypes: templatedConnectorConfigFieldAttr()}, flattenedFields)
}

func flattenTemplatedConnectorConfigField(ctx context.Context, field *cxsdk.TemplatedConnectorConfigField) (types.Object, diag.Diagnostics) {
	fieldModel := TemplatedConnectorConfigFieldModel{
		FieldName: types.StringValue(field.GetFieldName()),
		Template:  types.StringValue(field.GetTemplate()),
	}

	return types.ObjectValueFrom(ctx, templatedConnectorConfigFieldAttr(), fieldModel)
}

func flattenConnectorConfig(ctx context.Context, connectorConfig *cxsdk.ConnectorConfig) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	configFields, dg := flattenConnectorConfigFields(ctx, connectorConfig.GetFields())
	if dg.HasError() {
		diags.Append(dg...)
		return types.ObjectNull(connectorConfigAttr()), diags
	}

	connectorConfigModel := ConnectorConfigModel{ConnectorConfigFields: configFields}

	return types.ObjectValueFrom(ctx, connectorConfigAttr(), connectorConfigModel)
}

func connectorConfigAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"fields": types.SetType{ElemType: types.ObjectType{AttrTypes: connectorConfigFieldAttrs()}},
	}
}

func connectorConfigFieldAttrs() map[string]attr.Type {
	return map[string]attr.Type{
		"field_name": types.StringType,
		"value":      types.StringType,
	}
}

func connectorOverrideAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"entity_type": types.StringType,
		"fields":      types.SetType{ElemType: types.ObjectType{AttrTypes: templatedConnectorConfigFieldAttr()}},
	}
}

func templatedConnectorConfigFieldAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"field_name": types.StringType,
		"template":   types.StringType,
	}
}
