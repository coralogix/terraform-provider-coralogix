// Copyright 2024 Coralogix Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an AS IS BASIS,
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
	"github.com/coralogix/terraform-provider-coralogix/internal/provider/generated/connector"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	connectors "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/connectors_service"

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

var _ resource.ResourceWithImportState = &ConnectorResource{}

func NewConnectorResource() resource.Resource {
	return &ConnectorResource{}
}

type ConnectorResource struct {
	client *connectors.ConnectorsServiceAPIService
}

// ConnectorResourceModel is the generated connector plus write-only secret fields
// that the generator cannot emit.
type ConnectorResourceModel struct {
	ID              types.String `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	Description     types.String `tfsdk:"description"`
	Type            types.String `tfsdk:"type"`
	ConnectorConfig types.Object `tfsdk:"connector_config"` // ConnectorConfigModel
	ConfigOverrides types.List   `tfsdk:"config_overrides"`
}

type ConnectorConfigModel struct {
	ConnectorConfigFields types.Set `tfsdk:"fields"`
	FieldValuesWO         types.Map `tfsdk:"field_values_wo"`
	FieldValuesWOVersions types.Map `tfsdk:"field_values_wo_versions"`
}

type ConnectorConfigFieldModel struct {
	FieldName types.String `tfsdk:"field_name"`
	Value     types.String `tfsdk:"value"`
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

	r.client = clientSet.Connectors()
}

func (r *ConnectorResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := connector.Schema()
	if idAttr, ok := s.Attributes["id"].(schema.StringAttribute); ok {
		idAttr.PlanModifiers = []planmodifier.String{
			stringplanmodifier.UseStateForUnknown(),
			stringplanmodifier.RequiresReplace(),
		}
		s.Attributes["id"] = idAttr
	}
	cfg, ok := s.Attributes["connector_config"].(schema.SingleNestedAttribute)
	if !ok {
		resp.Diagnostics.AddError("Unexpected connector schema", "connector_config is not a single nested attribute.")
		return
	}
	cfg.Validators = []validator.Object{connectorWriteOnlyFieldsValidator{}}
	cfg.Attributes["field_values_wo"] = schema.MapAttribute{
		ElementType: types.StringType,
		Optional:    true,
		WriteOnly:   true,
		MarkdownDescription: "Secret values for connector fields, keyed by field name, which Terraform sends to the API and never writes to state. " +
			"A field named here must not also appear in `fields`. Each entry needs a matching entry in `field_values_wo_versions`. Requires Terraform 1.11 or later.\n\n" +
			"Importing is the one exception. An import has neither configuration nor prior state, so nothing identifies which field is a secret, and the API returns every field's value: " +
			"the secret is written to state by the import itself. The following apply removes it again. Treat a secret that has been through an import as exposed, and rotate it. " +
			"Reading the same connector through `data.coralogix_connector` also returns the value, under `connector_config.fields`: a data source reads from the API and has no configuration telling it which value is managed write-only.",
	}
	cfg.Attributes["field_values_wo_versions"] = schema.MapAttribute{
		ElementType: types.Int64Type,
		Optional:    true,
		MarkdownDescription: "Version of each `field_values_wo` entry, keyed by the same field name. Increment a value to send a rotated secret: " +
			"Terraform holds no copy of a write-only value, so it cannot notice that one changed. These versions are kept in state and are not secret.",
	}
	s.Attributes["connector_config"] = cfg
	resp.Schema = s
}

func (r *ConnectorResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
	resp.Diagnostics.AddWarning(
		"An imported secret is written to state",
		"Importing reads the connector's current configuration from Coralogix, including every field value it carries, and writes it to state. "+
			"Treat an imported secret as exposed and rotate it.\n\n"+
			"To keep it out of state from then on, move the value from connector_config.fields into connector_config.field_values_wo, "+
			"give it an entry in connector_config.field_values_wo_versions, and apply. That apply removes the imported value from state. "+
			"Write-only attributes need Terraform 1.11 or later.",
	)
}

var connectorCredentialFieldNames = map[string]struct{}{
	"additionalheaders": {},
	"alertsourcetoken":  {},
	"apikey":            {},
	"authorization":     {},
	"headers":           {},
	"integrationkey":    {},
	"password":          {},
	"secret":            {},
	"servicekey":        {},
	"token":             {},
}

func connectorWarningSummary(config *ConnectorResourceModel, field string) string {
	subject := ""
	switch {
	case !config.ID.IsNull() && !config.ID.IsUnknown():
		subject = config.ID.ValueString()
	case !config.Name.IsNull() && !config.Name.IsUnknown():
		subject = config.Name.ValueString()
	}
	if subject == "" {
		return fmt.Sprintf("Connector field %q is stored in state", field)
	}
	return fmt.Sprintf("Connector field %q of %q is stored in state", field, subject)
}

func connectorCredentialWarnings(ctx context.Context, config *ConnectorResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics
	if config == nil || config.ConnectorConfig.IsNull() || config.ConnectorConfig.IsUnknown() {
		return diags
	}

	var connectorConfig ConnectorConfigModel
	if dg := config.ConnectorConfig.As(ctx, &connectorConfig, basetypes.ObjectAsOptions{}); dg.HasError() {
		return diags
	}
	fieldSet := connectorConfig.ConnectorConfigFields
	if fieldSet.IsNull() || fieldSet.IsUnknown() {
		return diags
	}

	var fields []ConnectorConfigFieldModel
	if dg := fieldSet.ElementsAs(ctx, &fields, false); dg.HasError() {
		return diags
	}

	names := make([]string, 0, len(fields))
	for _, field := range fields {
		if field.FieldName.IsNull() || field.FieldName.IsUnknown() {
			continue
		}
		name := field.FieldName.ValueString()
		if _, ok := connectorCredentialFieldNames[strings.ToLower(name)]; !ok {
			continue
		}
		if field.Value.IsNull() {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		diags.AddAttributeWarning(
			path.Root("connector_config").AtName("fields"),
			connectorWarningSummary(config, name),
			fmt.Sprintf("%s is set through fields, which appears to carry a secret. Terraform writes it to state. ", name)+
				"Move it to connector_config.field_values_wo with an entry in connector_config.field_values_wo_versions to send the value without storing it. "+
				"Write-only attributes need Terraform 1.11 or later. "+
				"Terraform shows one warning per distinct message, so check any other connector with the same name as well.",
		)
	}
	return diags
}

func (r *ConnectorResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config *ConnectorResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(connectorCredentialWarnings(ctx, config)...)
}

func (r *ConnectorResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan *ConnectorResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
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

	body, diags := expandConnectorRequest(ctx, plan, secretFields, true)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	createBody, ok := body.(*connectors.CreateConnectorRequest)
	if !ok {
		resp.Diagnostics.AddError("Error creating coralogix_connector", "internal: unexpected create body")
		return
	}
	result, httpResponse, err := r.client.ConnectorsServiceCreateConnector(ctx).CreateConnectorRequest(*createBody).Execute()
	if err != nil {
		resp.Diagnostics.AddError("Error creating coralogix_connector",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Create", *createBody))
		return
	}

	source := plan
	plan, diags = flattenConnector(ctx, result.Connector, writeOnlyFieldNames(ctx, source))
	if !diags.HasError() {
		diags.Append(carryWriteOnlyVersions(ctx, plan, source)...)
	}
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *ConnectorResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state *ConnectorResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	result, httpResponse, err := r.client.ConnectorsServiceGetConnector(ctx, id).Execute()
	if err != nil {
		if httpResponse != nil && httpResponse.StatusCode == http.StatusNotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("coralogix_connector %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError("Error reading coralogix_connector",
				utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Read", nil))
		}
		return
	}

	priorState := state
	state, diags := flattenConnector(ctx, result.Connector, writeOnlyFieldNames(ctx, state))
	if !diags.HasError() {
		diags.Append(carryWriteOnlyVersions(ctx, state, priorState)...)
	}
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r ConnectorResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan *ConnectorResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
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

	body, diags := expandConnectorRequest(ctx, plan, secretFields, false)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	replaceBody, ok := body.(*connectors.ReplaceConnectorRequest)
	if !ok {
		resp.Diagnostics.AddError("Error replacing coralogix_connector", "internal: unexpected replace body")
		return
	}

	result, httpResponse, err := r.client.ConnectorsServiceReplaceConnector(ctx).ReplaceConnectorRequest(*replaceBody).Execute()
	if err != nil {
		if httpResponse != nil && httpResponse.StatusCode == http.StatusNotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("coralogix_connector %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError("Error replacing coralogix_connector", utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Replace", *replaceBody))
		}
		return
	}

	source := plan
	plan, diags = flattenConnector(ctx, result.Connector, writeOnlyFieldNames(ctx, source))
	if !diags.HasError() {
		diags.Append(carryWriteOnlyVersions(ctx, plan, source)...)
	}
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r ConnectorResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ConnectorResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := state.ID.ValueString()
	_, httpResponse, err := r.client.ConnectorsServiceDeleteConnector(ctx, id).Execute()
	if err != nil {
		resp.Diagnostics.AddError("Error deleting coralogix_connector",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Delete", id))
	}
}

func expandConnectorRequest(ctx context.Context, plan *ConnectorResourceModel, secretFields map[string]string, create bool) (any, diag.Diagnostics) {
	gen, diags := toGeneratedModel(ctx, plan)
	if diags.HasError() {
		return nil, diags
	}
	if create {
		body, d := connector.ExpandCreate(ctx, gen)
		diags.Append(d...)
		if diags.HasError() {
			return nil, diags
		}
		if body != nil && body.Connector != nil {
			mergeSecretFields(body.Connector, secretFields)
		}
		return body, diags
	}
	body, d := connector.ExpandUpdate(ctx, gen)
	diags.Append(d...)
	if diags.HasError() {
		return nil, diags
	}
	if body != nil && body.Connector != nil {
		mergeSecretFields(body.Connector, secretFields)
	}
	return body, diags
}

func toGeneratedModel(ctx context.Context, plan *ConnectorResourceModel) (*connector.ConnectorModel, diag.Diagnostics) {
	out := &connector.ConnectorModel{
		Id:              plan.ID,
		Name:            plan.Name,
		Description:     plan.Description,
		Type:            plan.Type,
		ConfigOverrides: plan.ConfigOverrides,
	}
	if plan.ConnectorConfig.IsNull() || plan.ConnectorConfig.IsUnknown() {
		return out, nil
	}
	var cfg ConnectorConfigModel
	diags := plan.ConnectorConfig.As(ctx, &cfg, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}
	out.ConnectorConfig = &connector.ConnectorConfigModel{Fields: cfg.ConnectorConfigFields}
	return out, diags
}

func flattenConnector(ctx context.Context, api *connectors.Connector, writeOnlyFields map[string]struct{}) (*ConnectorResourceModel, diag.Diagnostics) {
	gen, diags := connector.Flatten(ctx, api)
	if diags.HasError() {
		return nil, diags
	}
	return fromGeneratedModel(ctx, gen, writeOnlyFields)
}

func fromGeneratedModel(ctx context.Context, gen *connector.ConnectorModel, writeOnlyFields map[string]struct{}) (*ConnectorResourceModel, diag.Diagnostics) {
	out := &ConnectorResourceModel{
		ID:              gen.Id,
		Name:            gen.Name,
		Description:     gen.Description,
		Type:            gen.Type,
		ConfigOverrides: gen.ConfigOverrides,
	}
	if gen.ConnectorConfig == nil {
		out.ConnectorConfig = types.ObjectNull(connectorConfigAttr())
		return out, nil
	}
	fields, diags := omitWriteOnlyFields(ctx, gen.ConnectorConfig.Fields, writeOnlyFields)
	if diags.HasError() {
		return nil, diags
	}
	value, dg := types.ObjectValueFrom(ctx, connectorConfigAttr(), ConnectorConfigModel{
		ConnectorConfigFields: fields,
		FieldValuesWO:         types.MapNull(types.StringType),
		FieldValuesWOVersions: types.MapNull(types.Int64Type),
	})
	diags.Append(dg...)
	if diags.HasError() {
		return nil, diags
	}
	out.ConnectorConfig = value
	return out, diags
}

func omitWriteOnlyFields(ctx context.Context, fields types.Set, writeOnlyFields map[string]struct{}) (types.Set, diag.Diagnostics) {
	if fields.IsNull() || fields.IsUnknown() || len(writeOnlyFields) == 0 {
		return fields, nil
	}
	var items []ConnectorConfigFieldModel
	diags := fields.ElementsAs(ctx, &items, false)
	if diags.HasError() {
		return fields, diags
	}
	kept := make([]ConnectorConfigFieldModel, 0, len(items))
	for _, item := range items {
		if _, skip := writeOnlyFields[item.FieldName.ValueString()]; skip {
			continue
		}
		kept = append(kept, item)
	}
	out, dg := types.SetValueFrom(ctx, types.ObjectType{AttrTypes: connectorConfigFieldAttrs()}, kept)
	diags.Append(dg...)
	return out, diags
}

func mergeSecretFields(api *connectors.Connector, secretFields map[string]string) {
	if api == nil || len(secretFields) == 0 {
		return
	}
	if api.ConnectorConfig == nil {
		api.ConnectorConfig = &connectors.ConnectorConfig{}
	}
	names := make([]string, 0, len(secretFields))
	for name := range secretFields {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fieldName, value := name, secretFields[name]
		api.ConnectorConfig.Fields = append(api.ConnectorConfig.Fields, connectors.NotificationCenterConnectorConfigField{
			FieldName: &fieldName,
			Value:     &value,
		})
	}
}

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

	nullSecrets := make([]string, 0, len(secrets))
	for name, secret := range secrets {
		if secret.IsNull() {
			nullSecrets = append(nullSecrets, name)
		}
	}
	sort.Strings(nullSecrets)
	if len(nullSecrets) > 0 {
		resp.Diagnostics.AddAttributeError(req.Path.AtName("field_values_wo"),
			"Null Write-Only Field Value",
			fmt.Sprintf("These `field_values_wo` entries are null: %s.\n\n"+
				"A write-only field has to carry a string. Drop the entry, or set a value.",
				strings.Join(nullSecrets, ", ")))
	}

	missing := make([]string, 0, len(secrets))
	nullVersions := make([]string, 0, len(secrets))
	for name := range secrets {
		version, ok := versions[name]
		if !ok {
			missing = append(missing, name)
			continue
		}
		if version.IsNull() {
			nullVersions = append(nullVersions, name)
		}
	}
	sort.Strings(missing)
	sort.Strings(nullVersions)
	if len(missing) > 0 {
		resp.Diagnostics.AddAttributeError(req.Path.AtName("field_values_wo_versions"),
			"Missing Write-Only Field Version",
			fmt.Sprintf("These `field_values_wo` entries have no matching `field_values_wo_versions` entry: %s.\n\n"+
				"Terraform cannot notice that a write-only value changed, so a version has to sit next to each one. Increment it to send a rotated secret.",
				strings.Join(missing, ", ")))
	}
	if len(nullVersions) > 0 {
		resp.Diagnostics.AddAttributeError(req.Path.AtName("field_values_wo_versions"),
			"Null Write-Only Field Version",
			fmt.Sprintf("These `field_values_wo_versions` entries are null: %s.\n\n"+
				"A version has to be an integer. Drop the entry together with the `field_values_wo` value, or set a number.",
				strings.Join(nullVersions, ", ")))
	}

	if config.ConnectorConfigFields.IsNull() || config.ConnectorConfigFields.IsUnknown() {
		return
	}
	var fields []ConnectorConfigFieldModel
	if dg := config.ConnectorConfigFields.ElementsAs(ctx, &fields, false); dg.HasError() {
		return
	}
	both := make([]string, 0)
	for _, field := range fields {
		if field.FieldName.IsNull() || field.FieldName.IsUnknown() {
			continue
		}
		name := field.FieldName.ValueString()
		if _, ok := secrets[name]; ok {
			both = append(both, name)
		}
	}
	sort.Strings(both)
	if len(both) > 0 {
		resp.Diagnostics.AddAttributeError(req.Path.AtName("fields"),
			"Field Set Both Ways",
			fmt.Sprintf("These names appear in both `fields` and `field_values_wo`: %s.\n\n"+
				"A field can be supplied in only one of those places. Keep the ordinary value in `fields`, or the secret in `field_values_wo`.",
				strings.Join(both, ", ")))
	}
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
