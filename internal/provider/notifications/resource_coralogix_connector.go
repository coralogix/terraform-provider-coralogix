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
	"sort"
	"strings"

	"github.com/coralogix/terraform-provider-coralogix/internal/provider/generated/connector"

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
)

var (
	_ resource.Resource                     = &ConnectorResource{}
	_ resource.ResourceWithImportState      = &ConnectorResource{}
	_ resource.ResourceWithValidateConfig   = &ConnectorResource{}
	_ resource.ResourceWithConfigure        = &ConnectorResource{}
	_ resource.ResourceWithConfigValidators = &ConnectorResource{}
)

func NewConnectorResource() resource.Resource {
	return &ConnectorResource{Resource: connector.NewResource(connector.Hooks{
		BeforeWrite: mergeWriteOnlyIntoRequest,
		AfterRead:   restoreWriteOnlyAfterRead,
	}).(*connector.Resource)}
}

// ConnectorResource is the generated connector plus write-only secret overlay.
type ConnectorResource struct {
	*connector.Resource
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

func connectorWarningSummary(id, name types.String, field string) string {
	subject := ""
	switch {
	case !id.IsNull() && !id.IsUnknown():
		subject = id.ValueString()
	case !name.IsNull() && !name.IsUnknown():
		subject = name.ValueString()
	}
	if subject == "" {
		return fmt.Sprintf("Connector field %q is stored in state", field)
	}
	return fmt.Sprintf("Connector field %q of %q is stored in state", field, subject)
}

func connectorCredentialWarnings(ctx context.Context, id, name types.String, fields types.Set) diag.Diagnostics {
	var diags diag.Diagnostics
	if fields.IsNull() || fields.IsUnknown() {
		return diags
	}

	names := make([]string, 0)
	for _, elem := range fields.Elements() {
		obj, ok := elem.(types.Object)
		if !ok || obj.IsNull() || obj.IsUnknown() {
			continue
		}
		attrs := obj.Attributes()
		fieldName, _ := attrs["field_name"].(types.String)
		value, _ := attrs["value"].(types.String)
		if fieldName.IsNull() || fieldName.IsUnknown() || value.IsNull() {
			continue
		}
		n := fieldName.ValueString()
		if _, ok := connectorCredentialFieldNames[strings.ToLower(n)]; !ok {
			continue
		}
		names = append(names, n)
	}
	sort.Strings(names)

	for _, field := range names {
		diags.AddAttributeWarning(
			path.Root("connector_config").AtName("fields"),
			connectorWarningSummary(id, name, field),
			fmt.Sprintf("%s is set through fields, which appears to carry a secret. Terraform writes it to state. ", field)+
				"Move it to connector_config.field_values_wo with an entry in connector_config.field_values_wo_versions to send the value without storing it. "+
				"Write-only attributes need Terraform 1.11 or later. "+
				"Terraform shows one warning per distinct message, so check any other connector with the same name as well.",
		)
	}
	return diags
}

func (r *ConnectorResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var cfg types.Object
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("connector_config"), &cfg)...)
	if resp.Diagnostics.HasError() || cfg.IsNull() || cfg.IsUnknown() {
		return
	}
	fields, _ := cfg.Attributes()["fields"].(types.Set)
	var id, name types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("id"), &id)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("name"), &name)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(connectorCredentialWarnings(ctx, id, name, fields)...)
}

func omitWriteOnlyFields(ctx context.Context, fields types.Set, writeOnlyFields map[string]struct{}) (types.Set, diag.Diagnostics) {
	if fields.IsNull() || fields.IsUnknown() || len(writeOnlyFields) == 0 {
		return fields, nil
	}
	kept := make([]attr.Value, 0, len(fields.Elements()))
	for _, elem := range fields.Elements() {
		obj, ok := elem.(types.Object)
		if !ok {
			kept = append(kept, elem)
			continue
		}
		name, _ := obj.Attributes()["field_name"].(types.String)
		if _, skip := writeOnlyFields[name.ValueString()]; skip {
			continue
		}
		kept = append(kept, elem)
	}
	out, diags := types.SetValue(fields.ElementType(ctx), kept)
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
			FieldName: fieldName,
			Value:     &value,
		})
	}
}

type connectorWriteOnlyFieldsValidator struct{}

func (v connectorWriteOnlyFieldsValidator) Description(_ context.Context) string {
	return "each field_values_wo entry needs a matching field_values_wo_versions entry, and must not also appear in fields"
}

func (v connectorWriteOnlyFieldsValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func objectMap(v attr.Value) types.Map {
	if m, ok := v.(types.Map); ok {
		return m
	}
	return types.MapNull(types.StringType)
}

func (v connectorWriteOnlyFieldsValidator) ValidateObject(ctx context.Context, req validator.ObjectRequest, resp *validator.ObjectResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	attrs := req.ConfigValue.Attributes()
	wo := objectMap(attrs["field_values_wo"])
	versionsMap, _ := attrs["field_values_wo_versions"].(types.Map)
	if wo.IsUnknown() || versionsMap.IsUnknown() {
		return
	}

	secrets := map[string]attr.Value{}
	if !wo.IsNull() {
		secrets = wo.Elements()
	}
	versions := map[string]attr.Value{}
	if !versionsMap.IsNull() && !versionsMap.IsUnknown() {
		versions = versionsMap.Elements()
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

	fields, _ := attrs["fields"].(types.Set)
	if fields.IsNull() || fields.IsUnknown() {
		return
	}
	both := make([]string, 0)
	for _, elem := range fields.Elements() {
		obj, ok := elem.(types.Object)
		if !ok || obj.IsNull() || obj.IsUnknown() {
			continue
		}
		name, _ := obj.Attributes()["field_name"].(types.String)
		if name.IsNull() || name.IsUnknown() {
			continue
		}
		n := name.ValueString()
		if _, ok := secrets[n]; ok {
			both = append(both, n)
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
