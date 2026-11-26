// Copyright 2026 Coralogix Ltd.
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

// A config override template that holds a credential cannot be a write-only attribute of
// config_overrides[].fields. The Framework rejects a write-only attribute inside a set, and inside
// a Computed nested attribute, and config_overrides is both. The template goes in two maps next to
// config_overrides, keyed by "<entity_type>.<field_name>", as field_values_wo is keyed by field name.

import (
	"context"
	"fmt"
	"sort"
	"strings"

	connectors "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/connectors_service"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const (
	templatesWoAttr         = "config_overrides_templates_wo"
	templatesWoVersionsAttr = "config_overrides_templates_wo_versions"
)

// templateEntityTypes maps the Terraform entity types that a template key can name to the API values.
var templateEntityTypes = map[string]connectors.NotificationCenterEntityType{
	"alerts":             connectors.NOTIFICATIONCENTERENTITYTYPE_ALERTS,
	"cases":              connectors.NOTIFICATIONCENTERENTITYTYPE_CASES,
	"test_notifications": connectors.NOTIFICATIONCENTERENTITYTYPE_TEST_NOTIFICATIONS,
}

// splitTemplateKey splits "<entity_type>.<field_name>" at the first dot.
func splitTemplateKey(key string) (entityType, fieldName string, ok bool) {
	entityType, fieldName, ok = strings.Cut(key, ".")
	_, known := templateEntityTypes[entityType]
	return entityType, fieldName, ok && known && fieldName != ""
}

// mergeWriteOnlyTemplatesIntoRequest adds the templates of config_overrides_templates_wo to the
// overrides of the request. The value lives only in the configuration.
func mergeWriteOnlyTemplatesIntoRequest(ctx context.Context, config tfsdk.Config, body any) diag.Diagnostics {
	var secrets types.Map
	diags := config.GetAttribute(ctx, path.Root(templatesWoAttr), &secrets)
	if diags.HasError() || secrets.IsNull() || secrets.IsUnknown() {
		return diags
	}
	values := map[string]string{}
	diags.Append(secrets.ElementsAs(ctx, &values, false)...)
	if diags.HasError() {
		return diags
	}
	var connector *connectors.Connector
	switch req := body.(type) {
	case *connectors.CreateConnectorRequest:
		connector = &req.Connector
	case *connectors.ReplaceConnectorRequest:
		connector = &req.Connector
	default:
		return diags
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		entityType, fieldName, ok := splitTemplateKey(key)
		if !ok {
			diags.AddAttributeError(path.Root(templatesWoAttr), "Invalid write-only template key", invalidTemplateKeyDetail(key))
			continue
		}
		value := values[key]
		index := overrideIndex(connector.ConfigOverrides, templateEntityTypes[entityType])
		if index < 0 {
			diags.AddAttributeError(path.Root(templatesWoAttr), "Write-only template without an override",
				fmt.Sprintf("%q names the entity type %q, which is not in config_overrides.", key, entityType))
			continue
		}
		connector.ConfigOverrides[index].Fields = append(connector.ConfigOverrides[index].Fields,
			connectors.TemplatedConnectorConfigField{FieldName: fieldName, Template: &value})
	}
	return diags
}

func overrideIndex(overrides []connectors.EntityTypeConfigOverrides, entityType connectors.NotificationCenterEntityType) int {
	for i := range overrides {
		if overrides[i].EntityType == entityType {
			return i
		}
	}
	return -1
}

func invalidTemplateKeyDetail(key string) string {
	names := make([]string, 0, len(templateEntityTypes))
	for name := range templateEntityTypes {
		names = append(names, name)
	}
	sort.Strings(names)
	return fmt.Sprintf("The key %q has to be \"<entity_type>.<field_name>\", with an entity type of %s, and a field name.", key, strings.Join(names, ", "))
}

// restoreWriteOnlyTemplatesAfterRead keeps the write-only templates out of the state. The API returns
// every template on a read, so flatten puts them in config_overrides[].fields. The versions in the
// prior value, the plan after a write or the state before a read, say which keys are write-only.
// The state drops those fields and keeps the versions.
func restoreWriteOnlyTemplatesAfterRead(ctx context.Context, state *tfsdk.State, prior any) diag.Diagnostics {
	src, ok := prior.(priorAttributes)
	if state == nil || !ok {
		return nil
	}
	var versions types.Map
	diags := src.GetAttribute(ctx, path.Root(templatesWoVersionsAttr), &versions)
	if diags.HasError() {
		return diags
	}
	if versions.IsNull() || versions.IsUnknown() {
		return diags
	}
	var overrides types.List
	diags.Append(state.GetAttribute(ctx, path.Root("config_overrides"), &overrides)...)
	if diags.HasError() {
		return diags
	}
	kept, d := omitWriteOnlyTemplates(ctx, overrides, versions.Elements())
	diags.Append(d...)
	if diags.HasError() {
		return diags
	}
	diags.Append(state.SetAttribute(ctx, path.Root("config_overrides"), kept)...)
	diags.Append(state.SetAttribute(ctx, path.Root(templatesWoAttr), types.MapNull(types.StringType))...)
	diags.Append(state.SetAttribute(ctx, path.Root(templatesWoVersionsAttr), versions)...)
	return diags
}

// omitWriteOnlyTemplates removes the fields named by writeOnly from every override.
func omitWriteOnlyTemplates(ctx context.Context, overrides types.List, writeOnly map[string]attr.Value) (types.List, diag.Diagnostics) {
	if overrides.IsNull() || overrides.IsUnknown() || len(writeOnly) == 0 {
		return overrides, nil
	}
	var diags diag.Diagnostics
	items := make([]attr.Value, 0, len(overrides.Elements()))
	for _, element := range overrides.Elements() {
		override, ok := element.(types.Object)
		if !ok || override.IsNull() || override.IsUnknown() {
			items = append(items, element)
			continue
		}
		attrs := override.Attributes()
		entityType, _ := attrs["entity_type"].(types.String)
		fields, _ := attrs["fields"].(types.Set)
		if fields.IsNull() || fields.IsUnknown() {
			items = append(items, element)
			continue
		}
		kept := make([]attr.Value, 0, len(fields.Elements()))
		for _, field := range fields.Elements() {
			object, ok := field.(types.Object)
			if ok {
				name, _ := object.Attributes()["field_name"].(types.String)
				if _, secret := writeOnly[entityType.ValueString()+"."+name.ValueString()]; secret {
					continue
				}
			}
			kept = append(kept, field)
		}
		newFields, d := types.SetValue(fields.ElementType(ctx), kept)
		diags.Append(d...)
		next := make(map[string]attr.Value, len(attrs))
		for name, value := range attrs {
			next[name] = value
		}
		next["fields"] = newFields
		object, d := types.ObjectValue(override.AttributeTypes(ctx), next)
		diags.Append(d...)
		items = append(items, object)
	}
	if diags.HasError() {
		return overrides, diags
	}
	out, d := types.ListValue(overrides.ElementType(ctx), items)
	diags.Append(d...)
	return out, diags
}

// validateWriteOnlyTemplates checks the rules that no attribute validator expresses:
//   - a key is "<entity_type>.<field_name>"
//   - every key has a version, and every version has a key, and none is null
//   - the entity type of a key is in config_overrides
//   - a key is not also a field of config_overrides
func validateWriteOnlyTemplates(ctx context.Context, config tfsdk.Config) diag.Diagnostics {
	var secrets, versions types.Map
	var overrides types.List
	diags := config.GetAttribute(ctx, path.Root(templatesWoAttr), &secrets)
	diags.Append(config.GetAttribute(ctx, path.Root(templatesWoVersionsAttr), &versions)...)
	diags.Append(config.GetAttribute(ctx, path.Root("config_overrides"), &overrides)...)
	if diags.HasError() || secrets.IsUnknown() || versions.IsUnknown() {
		return diags
	}
	names, numbers := map[string]attr.Value{}, map[string]attr.Value{}
	if !secrets.IsNull() {
		names = secrets.Elements()
	}
	if !versions.IsNull() {
		numbers = versions.Elements()
	}
	var invalid, orphans, nullSecrets, missing, nullVersions []string
	for key := range numbers {
		if _, ok := names[key]; !ok {
			orphans = append(orphans, key)
		}
	}
	for key, secret := range names {
		if _, _, ok := splitTemplateKey(key); !ok {
			invalid = append(invalid, key)
		}
		if secret.IsNull() {
			nullSecrets = append(nullSecrets, key)
		}
		switch version, ok := numbers[key]; {
		case !ok:
			missing = append(missing, key)
		case version.IsNull():
			nullVersions = append(nullVersions, key)
		}
	}
	report := func(attribute, summary, detail string, keys []string) {
		if len(keys) == 0 {
			return
		}
		sort.Strings(keys)
		diags.AddAttributeError(path.Root(attribute), summary, fmt.Sprintf(detail, strings.Join(keys, ", ")))
	}
	report(templatesWoAttr, "Invalid write-only template key",
		"These keys are not \"<entity_type>.<field_name>\" with an entity type of alerts, cases, or test_notifications: %s.", invalid)
	report(templatesWoVersionsAttr, "Version without a write-only template",
		"These `"+templatesWoVersionsAttr+"` entries name a template that is not set in `"+templatesWoAttr+"`: %s.", orphans)
	report(templatesWoAttr, "Null write-only template",
		"These `"+templatesWoAttr+"` entries are null: %s.\n\nA write-only template needs a value to send. Drop the entry, or set a value.", nullSecrets)
	report(templatesWoVersionsAttr, "Missing write-only template version",
		"These `"+templatesWoAttr+"` entries have no matching `"+templatesWoVersionsAttr+"` entry: %s.\n\n"+
			"Terraform cannot notice that a write-only value changed, so a version has to sit next to each one. Increment it to send a rotated secret.", missing)
	report(templatesWoVersionsAttr, "Null write-only template version",
		"These `"+templatesWoVersionsAttr+"` entries are null: %s.\n\nA version has to be a number.", nullVersions)
	if len(names) > 0 {
		diags.Append(validateTemplateOverrides(names, overrides)...)
	}
	return diags
}

// validateTemplateOverrides checks the keys against the overrides of the configuration.
func validateTemplateOverrides(names map[string]attr.Value, overrides types.List) diag.Diagnostics {
	var diags diag.Diagnostics
	declared, plain, known := declaredTemplates(overrides)
	if !known {
		return diags
	}
	var undeclared, both []string
	for key := range names {
		entityType, _, ok := splitTemplateKey(key)
		switch {
		case !ok:
		case !declared[entityType]:
			undeclared = append(undeclared, key)
		case plain[key]:
			both = append(both, key)
		}
	}
	sort.Strings(undeclared)
	sort.Strings(both)
	if len(undeclared) > 0 {
		diags.AddAttributeError(path.Root(templatesWoAttr), "Write-only template without an override",
			fmt.Sprintf("The entity type of these keys is not in `config_overrides`: %s.\n\n"+
				"Declare it, for example `config_overrides = [{ entity_type = \"alerts\", fields = [] }]`.", strings.Join(undeclared, ", ")))
	}
	if len(both) > 0 {
		diags.AddAttributeError(path.Root(templatesWoAttr), "Template set both ways",
			fmt.Sprintf("These keys appear in `"+templatesWoAttr+"` and in `config_overrides[].fields`: %s.\n\n"+
				"A template is set in one of them. Keep the ordinary value in `fields`, or the secret in `"+templatesWoAttr+"`.", strings.Join(both, ", ")))
	}
	return diags
}

// declaredTemplates returns the entity types of the overrides, and the "<entity_type>.<field_name>"
// keys of their plain fields. known is false when a value is unknown, and nothing can be checked.
func declaredTemplates(overrides types.List) (declared, plain map[string]bool, known bool) {
	declared, plain = map[string]bool{}, map[string]bool{}
	if overrides.IsUnknown() {
		return declared, plain, false
	}
	for _, element := range overrides.Elements() {
		override, ok := element.(types.Object)
		if !ok || override.IsNull() || override.IsUnknown() {
			return declared, plain, false
		}
		entityType, _ := override.Attributes()["entity_type"].(types.String)
		if entityType.IsUnknown() {
			return declared, plain, false
		}
		declared[entityType.ValueString()] = true
		fields, _ := override.Attributes()["fields"].(types.Set)
		for _, field := range fields.Elements() {
			object, ok := field.(types.Object)
			if !ok || object.IsNull() || object.IsUnknown() {
				continue
			}
			name, _ := object.Attributes()["field_name"].(types.String)
			plain[entityType.ValueString()+"."+name.ValueString()] = true
		}
	}
	return declared, plain, true
}
