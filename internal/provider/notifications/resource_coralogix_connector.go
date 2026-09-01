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
	"net/http"
	"sort"
	"strings"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	connectors "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/connectors_service"

	"github.com/hashicorp/terraform-plugin-framework-validators/mapvalidator"
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
)

var (
	_                        resource.ResourceWithImportState = &ConnectorResource{}
	connectorTypeSchemaToApi                                  = map[string]connectors.NotificationCenterConnectorType{
		utils.UNSPECIFIED:     connectors.NOTIFICATIONCENTERCONNECTORTYPE_CONNECTOR_TYPE_UNSPECIFIED,
		"slack":               connectors.NOTIFICATIONCENTERCONNECTORTYPE_SLACK,
		"generic_https":       connectors.NOTIFICATIONCENTERCONNECTORTYPE_GENERIC_HTTPS,
		"pagerduty":           connectors.NOTIFICATIONCENTERCONNECTORTYPE_PAGERDUTY,
		"pagerduty_incidents": connectors.NOTIFICATIONCENTERCONNECTORTYPE_PAGERDUTY_INCIDENTS,
		"email":               connectors.NOTIFICATIONCENTERCONNECTORTYPE_EMAIL,
		"service_now":         connectors.NOTIFICATIONCENTERCONNECTORTYPE_SERVICE_NOW,
		"microsoft_teams":     connectors.NOTIFICATIONCENTERCONNECTORTYPE_MICROSOFT_TEAMS,
		"eventbridge":         connectors.NOTIFICATIONCENTERCONNECTORTYPE_EVENTBRIDGE,
		"incident_io":         connectors.NOTIFICATIONCENTERCONNECTORTYPE_INCIDENT_IO,
	}
	connectorTypeApiToSchema       = utils.ReverseMap(connectorTypeSchemaToApi)
	validConnectorTypesSchemaToApi = utils.GetKeys(connectorTypeSchemaToApi)
	connectorEntityTypeSchemaToApi = map[string]connectors.NotificationCenterEntityType{
		utils.UNSPECIFIED:    connectors.NOTIFICATIONCENTERENTITYTYPE_ENTITY_TYPE_UNSPECIFIED,
		"alerts":             connectors.NOTIFICATIONCENTERENTITYTYPE_ALERTS,
		"cases":              connectors.NOTIFICATIONCENTERENTITYTYPE_CASES,
		"test_notifications": connectors.NOTIFICATIONCENTERENTITYTYPE_TEST_NOTIFICATIONS,
	}
	connectorNotificationCenterEntityTypeApiToSchema       = utils.ReverseMap(connectorEntityTypeSchemaToApi)
	connectorValidNotificationCenterEntityTypesSchemaToApi = utils.GetKeys(connectorEntityTypeSchemaToApi)
)

func NewConnectorResource() resource.Resource {
	return &ConnectorResource{}
}

type ConnectorResource struct {
	client *connectors.ConnectorsServiceAPIService
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
	// Beside the field list rather than inside it: a write-only attribute is
	// not allowed anywhere under a set.
	FieldValuesWO         types.Map `tfsdk:"field_values_wo"`          // field name -> secret
	FieldValuesWOVersions types.Map `tfsdk:"field_values_wo_versions"` // field name -> int64
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

	r.client, _, _ = clientSet.GetNotifications()
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
					stringvalidator.OneOf(validConnectorTypesSchemaToApi...),
				},
				MarkdownDescription: fmt.Sprintf("Connector type. Valid values are: %s. `incident_io` is a preview type.", validConnectorTypesSchemaToApi),
			},
			"connector_config": schema.SingleNestedAttribute{
				Optional: true,
				Validators: []validator.Object{
					connectorWriteOnlyFieldsValidator{},
				},
				Attributes: map[string]schema.Attribute{
					"field_values_wo": schema.MapAttribute{
						ElementType: types.StringType,
						Optional:    true,
						WriteOnly:   true,
						Validators: []validator.Map{
							mapvalidator.AlsoRequires(path.MatchRelative().AtParent().AtName("field_values_wo_versions")),
						},
						MarkdownDescription: "Secret values for connector fields, keyed by field name, which Terraform sends to the API and never writes to state. " +
							"A field named here must not also appear in `fields`. Each entry needs a matching entry in `field_values_wo_versions`. Requires Terraform 1.11 or later.\n\n" +
							"Importing is the one exception. An import has neither configuration nor prior state, so nothing identifies which field is a secret, and the API returns every field's value: " +
							"the secret is written to state by the import itself. The following apply removes it again. Treat a secret that has been through an import as exposed, and rotate it.",
					},
					"field_values_wo_versions": schema.MapAttribute{
						ElementType: types.Int64Type,
						Optional:    true,
						MarkdownDescription: "Version of each `field_values_wo` entry, keyed by the same field name. Increment a value to send a rotated secret: " +
							"Terraform holds no copy of a write-only value, so it cannot notice that one changed. These versions are kept in state and are not secret.",
					},
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
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"entity_type": schema.StringAttribute{
							Required: true,
							Validators: []validator.String{
								stringvalidator.OneOf(connectorValidNotificationCenterEntityTypesSchemaToApi...),
							},
							Description: fmt.Sprintf("Entity type for the connector. Valid values are: %s", connectorValidNotificationCenterEntityTypesSchemaToApi),
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
		MarkdownDescription: "Coralogix Notification Center Connector. For more info please review - https://coralogix.com/docs/user-guides/notification-center/connectors/. **Note:** This resource is in Beta stage.",
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
	var config *ConnectorResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	secretFields, diags := secretFieldsFromConfig(ctx, config)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	writeOnlyFields := writeOnlyFieldNames(ctx, plan)

	connector, diags := extractConnector(ctx, plan, secretFields)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	rq := connectors.CreateConnectorRequest{
		Connector: connector,
	}
	result, httpResponse, err := r.client.
		ConnectorsServiceCreateConnector(ctx).
		CreateConnectorRequest(rq).
		Execute()

	if err != nil {
		resp.Diagnostics.AddError("Error creating coralogix_connector",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Create", rq),
		)
		return
	}

	source := plan
	plan, diags = flattenConnector(ctx, result.Connector, writeOnlyFields)
	if !diags.HasError() {
		diags.Append(carryWriteOnlyVersions(ctx, plan, source)...)
	}
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

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

	id := state.ID.ValueString()
	rq := r.client.ConnectorsServiceGetConnector(ctx, id)

	result, httpResponse, err := rq.Execute()
	if err != nil {
		if httpResponse.StatusCode == http.StatusNotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("coralogix_connector %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError("Error reading coralogix_connector",
				utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Read", nil),
			)
		}
		return
	}

	// Read has no configuration, so the field names come from prior state.
	priorState := state
	state, diags = flattenConnector(ctx, result.Connector, writeOnlyFieldNames(ctx, state))
	if !diags.HasError() {
		diags.Append(carryWriteOnlyVersions(ctx, state, priorState)...)
	}
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r ConnectorResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan *ConnectorResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := plan.ID.ValueString()
	var config *ConnectorResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	secretFields, diags := secretFieldsFromConfig(ctx, config)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	writeOnlyFields := writeOnlyFieldNames(ctx, plan)

	connector, diags := extractConnector(ctx, plan, secretFields)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	rq := connectors.ReplaceConnectorRequest{
		Connector: connector,
	}

	result, httpResponse, err := r.client.
		ConnectorsServiceReplaceConnector(ctx).
		ReplaceConnectorRequest(rq).
		Execute()

	if err != nil {
		if httpResponse.StatusCode == http.StatusNotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("coralogix_connector %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError("Error replacing coralogix_connector", utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Replace", rq))
		}
		return
	}

	source := plan
	plan, diags = flattenConnector(ctx, result.Connector, writeOnlyFields)
	if !diags.HasError() {
		diags.Append(carryWriteOnlyVersions(ctx, plan, source)...)
	}
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

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

	_, httpResponse, err := r.client.
		ConnectorsServiceDeleteConnector(ctx, id).
		Execute()

	if err != nil {
		resp.Diagnostics.AddError("Error deleting coralogix_connector",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Delete", id),
		)
		return
	}
}

// secretFieldsFromConfig reads the write-only secrets. They exist nowhere but
// the configuration, which is also why Read cannot obtain them.
func secretFieldsFromConfig(ctx context.Context, config *ConnectorResourceModel) (map[string]string, diag.Diagnostics) {
	if config == nil || config.ConnectorConfig.IsNull() || config.ConnectorConfig.IsUnknown() {
		return nil, nil
	}
	var model ConnectorConfigModel
	if dg := config.ConnectorConfig.As(ctx, &model, basetypes.ObjectAsOptions{}); dg.HasError() {
		return nil, dg
	}
	if model.FieldValuesWO.IsNull() || model.FieldValuesWO.IsUnknown() {
		return nil, nil
	}
	out := make(map[string]string)
	if dg := model.FieldValuesWO.ElementsAs(ctx, &out, false); dg.HasError() {
		return nil, dg
	}
	return out, nil
}

// writeOnlyFieldNames reads the managed field names from the versions map,
// which is the only record of them that reaches state.
func writeOnlyFieldNames(ctx context.Context, model *ConnectorResourceModel) map[string]struct{} {
	if model == nil || model.ConnectorConfig.IsNull() || model.ConnectorConfig.IsUnknown() {
		return nil
	}
	var config ConnectorConfigModel
	if dg := model.ConnectorConfig.As(ctx, &config, basetypes.ObjectAsOptions{}); dg.HasError() {
		return nil
	}
	if config.FieldValuesWOVersions.IsNull() || config.FieldValuesWOVersions.IsUnknown() {
		return nil
	}
	names := make(map[string]struct{}, len(config.FieldValuesWOVersions.Elements()))
	for name := range config.FieldValuesWOVersions.Elements() {
		names[name] = struct{}{}
	}
	return names
}

// carryWriteOnlyVersions restores the versions map, which the API does not
// store and the flatten therefore cannot recover.
func carryWriteOnlyVersions(ctx context.Context, flattened, source *ConnectorResourceModel) diag.Diagnostics {
	if flattened == nil || source == nil || flattened.ConnectorConfig.IsNull() {
		return nil
	}
	var from ConnectorConfigModel
	if source.ConnectorConfig.IsNull() || source.ConnectorConfig.IsUnknown() {
		return nil
	}
	if dg := source.ConnectorConfig.As(ctx, &from, basetypes.ObjectAsOptions{}); dg.HasError() {
		return dg
	}
	var into ConnectorConfigModel
	if dg := flattened.ConnectorConfig.As(ctx, &into, basetypes.ObjectAsOptions{}); dg.HasError() {
		return dg
	}
	into.FieldValuesWOVersions = from.FieldValuesWOVersions
	value, dg := types.ObjectValueFrom(ctx, connectorConfigAttr(), into)
	if dg.HasError() {
		return dg
	}
	flattened.ConnectorConfig = value
	return nil
}

// connectorWriteOnlyFieldsValidator exists because both mistakes it catches are
// otherwise silent: a duplicated name is sent twice and the API keeps one, and a
// secret with no version leaves nothing in state to omit it on read.
type connectorWriteOnlyFieldsValidator struct{}

func (v connectorWriteOnlyFieldsValidator) Description(_ context.Context) string {
	return "each field_values_wo entry needs a matching field_values_wo_versions entry, and must not also appear in fields"
}

func (v connectorWriteOnlyFieldsValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v connectorWriteOnlyFieldsValidator) ValidateObject(ctx context.Context, req validator.ObjectRequest, resp *validator.ObjectResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	var config ConnectorConfigModel
	if dg := req.ConfigValue.As(ctx, &config, basetypes.ObjectAsOptions{}); dg.HasError() {
		return
	}
	if config.FieldValuesWO.IsUnknown() || config.FieldValuesWOVersions.IsUnknown() {
		return
	}

	secrets := map[string]attr.Value{}
	if !config.FieldValuesWO.IsNull() {
		secrets = config.FieldValuesWO.Elements()
	}
	versions := map[string]attr.Value{}
	if !config.FieldValuesWOVersions.IsNull() {
		versions = config.FieldValuesWOVersions.Elements()
	}

	// A version naming a field that is not managed write-only makes the read
	// leave that field out, and the apply then fails on a value the
	// configuration still declares.
	orphans := make([]string, 0, len(versions))
	for name := range versions {
		if _, ok := secrets[name]; !ok {
			orphans = append(orphans, name)
		}
	}
	sort.Strings(orphans)
	if len(orphans) > 0 {
		resp.Diagnostics.AddAttributeError(req.Path.AtName("field_values_wo_versions"),
			"Version Without a Write-Only Field",
			fmt.Sprintf("These `field_values_wo_versions` entries name a field that is not set in `field_values_wo`: %s.\n\n"+
				"A version only means something for a field whose value is supplied write-only. Remove the version, or move that field's value into `field_values_wo`.",
				strings.Join(orphans, ", ")))
	}

	if len(secrets) == 0 {
		return
	}

	missing := make([]string, 0, len(secrets))
	for name := range secrets {
		if _, ok := versions[name]; !ok {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		resp.Diagnostics.AddAttributeError(req.Path.AtName("field_values_wo_versions"),
			"Missing Write-Only Field Version",
			fmt.Sprintf("These `field_values_wo` entries have no matching `field_values_wo_versions` entry: %s.\n\n"+
				"Every write-only secret needs a version. Terraform keeps no copy of the value, so the version is both how a rotation is signalled and the only record in state that the field is managed this way.",
				strings.Join(missing, ", ")))
	}

	if config.ConnectorConfigFields.IsNull() || config.ConnectorConfigFields.IsUnknown() {
		return
	}
	var fields []ConnectorConfigFieldModel
	if dg := config.ConnectorConfigFields.ElementsAs(ctx, &fields, true); dg.HasError() {
		return
	}
	both := make([]string, 0)
	for _, f := range fields {
		if f.FieldName.IsNull() || f.FieldName.IsUnknown() {
			continue
		}
		if _, ok := secrets[f.FieldName.ValueString()]; ok {
			both = append(both, f.FieldName.ValueString())
		}
	}
	sort.Strings(both)
	if len(both) > 0 {
		resp.Diagnostics.AddAttributeError(req.Path.AtName("field_values_wo"),
			"Duplicate Connector Field",
			fmt.Sprintf("These fields are set in both `fields` and `field_values_wo`: %s.\n\n"+
				"Give each field a value in one place only. A field whose value is secret belongs in `field_values_wo`, and should be left out of `fields`.",
				strings.Join(both, ", ")))
	}
}

// secretFields is passed in rather than read from the plan, which never carries
// a write-only value.
func extractConnector(ctx context.Context, plan *ConnectorResourceModel, secretFields map[string]string) (*connectors.Connector, diag.Diagnostics) {
	connectorConfigs, diags := extractConnectorConfig(ctx, plan.ConnectorConfig, secretFields)
	if diags.HasError() {
		return nil, diags
	}

	configOverrides, diags := extractConfigOverrides(ctx, plan.ConfigOverrides)
	if diags.HasError() {
		return nil, diags
	}
	ty := connectorTypeSchemaToApi[plan.Type.ValueString()]
	return &connectors.Connector{
		Id:              utils.TypeStringToStringPointer(plan.ID),
		Name:            plan.Name.ValueStringPointer(),
		Description:     plan.Description.ValueStringPointer(),
		Type:            &ty,
		ConnectorConfig: connectorConfigs,
		ConfigOverrides: configOverrides,
	}, nil
}

func extractConnectorConfig(ctx context.Context, connectorConfig types.Object, secretFields map[string]string) (*connectors.ConnectorConfig, diag.Diagnostics) {
	var connectorConfigModel ConnectorConfigModel
	diags := connectorConfig.As(ctx, &connectorConfigModel, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	extractedConnectorConfigFields, diags := extractConnectorConfigFields(ctx, connectorConfigModel.ConnectorConfigFields, secretFields)
	if diags.HasError() {
		return nil, diags
	}

	return &connectors.ConnectorConfig{
		Fields: extractedConnectorConfigFields,
	}, nil
}

func extractConnectorConfigFields(ctx context.Context, connectorConfigFields types.Set, secretFields map[string]string) ([]connectors.NotificationCenterConnectorConfigField, diag.Diagnostics) {
	var diags diag.Diagnostics
	var connectorConfigFieldsObjects []types.Object
	connectorConfigFields.ElementsAs(ctx, &connectorConfigFieldsObjects, true)
	extractedConnectorConfigFields := make([]connectors.NotificationCenterConnectorConfigField, 0, len(connectorConfigFieldsObjects))

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

	// Appended rather than merged by key: the schema already rejects a name
	// present in both places.
	secretNames := make([]string, 0, len(secretFields))
	for name := range secretFields {
		secretNames = append(secretNames, name)
	}
	sort.Strings(secretNames)
	for _, name := range secretNames {
		fieldName, value := name, secretFields[name]
		extractedConnectorConfigFields = append(extractedConnectorConfigFields, connectors.NotificationCenterConnectorConfigField{
			FieldName: &fieldName,
			Value:     &value,
		})
	}

	return extractedConnectorConfigFields, diags
}

func extractConnectorConfigField(connectorConfigField ConnectorConfigFieldModel) connectors.NotificationCenterConnectorConfigField {
	return connectors.NotificationCenterConnectorConfigField{
		FieldName: connectorConfigField.FieldName.ValueStringPointer(),
		Value:     connectorConfigField.Value.ValueStringPointer(),
	}
}

func extractConfigOverrides(ctx context.Context, overrides types.List) ([]connectors.EntityTypeConfigOverrides, diag.Diagnostics) {
	if overrides.IsNull() || overrides.IsUnknown() {
		return nil, nil
	}
	var diags diag.Diagnostics
	var connectorOverridesObjects []types.Object
	overrides.ElementsAs(ctx, &connectorOverridesObjects, true)
	extractedConnectorOverrides := make([]connectors.EntityTypeConfigOverrides, 0, len(connectorOverridesObjects))

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
		extractedConnectorOverrides = append(extractedConnectorOverrides, *extractedConnectorOverride)
	}

	if diags.HasError() {
		return nil, diags
	}

	return extractedConnectorOverrides, diags
}

func extractConnectorOverride(ctx context.Context, connectorOverrideModel ConfigOverrideModel) (*connectors.EntityTypeConfigOverrides, diag.Diagnostics) {
	templatedConnectorConfigFields, diags := extractTemplatedConnectorConfigFields(ctx, connectorOverrideModel.Fields)
	if diags.HasError() {
		return nil, diags
	}
	entityType := connectorEntityTypeSchemaToApi[connectorOverrideModel.EntityType.ValueString()]
	return &connectors.EntityTypeConfigOverrides{
		EntityType: &entityType,
		Fields:     templatedConnectorConfigFields,
	}, nil
}

func extractTemplatedConnectorConfigFields(ctx context.Context, connectorConfigFields types.Set) ([]connectors.TemplatedConnectorConfigField, diag.Diagnostics) {
	var diags diag.Diagnostics
	var connectorConfigFieldsObjects []types.Object
	connectorConfigFields.ElementsAs(ctx, &connectorConfigFieldsObjects, true)
	extractedConnectorConfigFields := make([]connectors.TemplatedConnectorConfigField, 0, len(connectorConfigFieldsObjects))

	for _, ccf := range connectorConfigFieldsObjects {
		var connectorConfigFieldModel TemplatedConnectorConfigFieldModel
		if dg := ccf.As(ctx, &connectorConfigFieldModel, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		extractedConnectorConfigField := extractTemplatedConnectorConfigField(connectorConfigFieldModel)
		extractedConnectorConfigFields = append(extractedConnectorConfigFields, *extractedConnectorConfigField)
	}

	if diags.HasError() {
		return nil, diags
	}

	return extractedConnectorConfigFields, diags
}

func extractTemplatedConnectorConfigField(model TemplatedConnectorConfigFieldModel) *connectors.TemplatedConnectorConfigField {
	return &connectors.TemplatedConnectorConfigField{
		FieldName: model.FieldName.ValueStringPointer(),
		Template:  model.Template.ValueStringPointer(),
	}
}

// Fields named in writeOnlyFields are left out. The API returns their values,
// and storing one would put the secret in state and diff on every later plan.
func flattenConnector(ctx context.Context, connector *connectors.Connector, writeOnlyFields map[string]struct{}) (*ConnectorResourceModel, diag.Diagnostics) {
	if connector.ConnectorConfig == nil {
		connector.ConnectorConfig = &connectors.ConnectorConfig{}
	}
	config, diags := flattenConnectorConfig(ctx, *connector.ConnectorConfig, writeOnlyFields)
	if diags.HasError() {
		return nil, diags
	}

	overrides, diags := flattenConnectorOverrides(ctx, connector.ConfigOverrides)
	if diags.HasError() {
		return nil, diags
	}

	return &ConnectorResourceModel{
		ID:              types.StringValue(connector.GetId()),
		Name:            types.StringValue(connector.GetName()),
		Description:     types.StringValue(connector.GetDescription()),
		Type:            types.StringValue(connectorTypeApiToSchema[connector.GetType()]),
		ConnectorConfig: config,
		ConfigOverrides: overrides,
	}, nil
}

func flattenConnectorOverrides(ctx context.Context, overrides []connectors.EntityTypeConfigOverrides) (types.List, diag.Diagnostics) {
	if overrides == nil {
		return types.ListNull(types.ObjectType{AttrTypes: connectorOverrideAttr()}), nil
	}
	var diags diag.Diagnostics
	flattenedOverrides := make([]types.Object, 0, len(overrides))
	for _, override := range overrides {
		flattenedOverride, dg := flattenConnectorOverride(ctx, &override)
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

func flattenConnectorOverride(ctx context.Context, override *connectors.EntityTypeConfigOverrides) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	overrideFields, dg := flattenTemplatedConnectorConfigFields(ctx, override.GetFields())
	if dg.HasError() {
		diags.Append(dg...)
		return types.ObjectNull(connectorOverrideAttr()), diags
	}

	connectorOverrideModel := ConfigOverrideModel{
		EntityType: types.StringValue(connectorNotificationCenterEntityTypeApiToSchema[override.GetEntityType()]),
		Fields:     overrideFields,
	}

	return types.ObjectValueFrom(ctx, connectorOverrideAttr(), connectorOverrideModel)
}

func flattenTemplatedConnectorConfigFields(ctx context.Context, fields []connectors.TemplatedConnectorConfigField) (types.Set, diag.Diagnostics) {
	var diags diag.Diagnostics
	flattenedFields := make([]types.Object, 0, len(fields))
	for _, field := range fields {
		flattenedField, dg := flattenTemplatedConnectorConfigField(ctx, &field)
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

func flattenTemplatedConnectorConfigField(ctx context.Context, field *connectors.TemplatedConnectorConfigField) (types.Object, diag.Diagnostics) {
	fieldModel := TemplatedConnectorConfigFieldModel{
		FieldName: types.StringValue(field.GetFieldName()),
		Template:  types.StringValue(field.GetTemplate()),
	}

	return types.ObjectValueFrom(ctx, templatedConnectorConfigFieldAttr(), fieldModel)
}

func flattenConnectorConfig(ctx context.Context, connectorConfig connectors.ConnectorConfig, writeOnlyFields map[string]struct{}) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	configFields, dg := flattenConnectorConfigFields(ctx, connectorConfig.Fields, writeOnlyFields)
	if dg.HasError() {
		diags.Append(dg...)
		return types.ObjectNull(connectorConfigAttr()), diags
	}

	connectorConfigModel := ConnectorConfigModel{
		ConnectorConfigFields: configFields,
		// Never written to state; the caller restores the versions map.
		FieldValuesWO:         types.MapNull(types.StringType),
		FieldValuesWOVersions: types.MapNull(types.Int64Type),
	}

	return types.ObjectValueFrom(ctx, connectorConfigAttr(), connectorConfigModel)
}

func flattenConnectorConfigFields(ctx context.Context, configFields []connectors.NotificationCenterConnectorConfigField, writeOnlyFields map[string]struct{}) (types.Set, diag.Diagnostics) {
	var diags diag.Diagnostics
	if configFields == nil {
		return types.SetNull(types.ObjectType{AttrTypes: connectorConfigFieldAttrs()}), diags
	}

	configFieldsList := make([]ConnectorConfigFieldModel, 0, len(configFields))
	for _, field := range configFields {
		if _, managed := writeOnlyFields[field.GetFieldName()]; managed {
			continue
		}
		fieldModel := ConnectorConfigFieldModel{
			FieldName: types.StringValue(field.GetFieldName()),
			Value:     types.StringValue(field.GetValue()),
		}
		configFieldsList = append(configFieldsList, fieldModel)
	}

	return types.SetValueFrom(ctx, types.ObjectType{AttrTypes: connectorConfigFieldAttrs()}, configFieldsList)
}

func connectorConfigAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"fields":                   types.SetType{ElemType: types.ObjectType{AttrTypes: connectorConfigFieldAttrs()}},
		"field_values_wo":          types.MapType{ElemType: types.StringType},
		"field_values_wo_versions": types.MapType{ElemType: types.Int64Type},
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
